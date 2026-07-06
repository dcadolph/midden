package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/llm"
)

// chatTopK caps the number of entries fed to the chat model as context.
var chatTopK int

// chatCmd answers a question using recalled entries as context.
var chatCmd = &cobra.Command{
	Use:   "chat [question...]",
	Short: "Answer a question using semantic recall plus a chat model.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runChat,
}

func init() {
	chatCmd.Flags().IntVarP(&chatTopK, "top", "k", 8, "Number of entries to include as context.")
	rootCmd.AddCommand(chatCmd)
}

// runChat embeds the question, recalls relevant entries, and asks the chat model
// to answer using only those entries as evidence.
func runChat(cmd *cobra.Command, args []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	question := strings.Join(args, " ")
	rc, err := embedRecallQuery(cmd, v, question, 60*time.Second)
	if err != nil {
		return err
	}
	chat, err := llm.ChatterFromEnv()
	if err != nil {
		return errors.Join(ErrLLM, fmt.Errorf("pick chatter: %w", err))
	}
	matches := rc.Index.Search(rc.Query, chatTopK)
	var contextBuf strings.Builder
	for _, m := range matches {
		contextBuf.WriteString("--- ")
		contextBuf.WriteString(m.Entry.Time.Format(layoutDateTime))
		if len(m.Entry.Tags) > 0 {
			contextBuf.WriteString(" [")
			contextBuf.WriteString(strings.Join(m.Entry.Tags, ", "))
			contextBuf.WriteString("]")
		}
		contextBuf.WriteString("\n")
		contextBuf.WriteString(m.Entry.Body)
		contextBuf.WriteString("\n\n")
	}
	system := "You are a personal journal assistant. Answer the user's question using only the journal entries supplied below. Quote the date of any entry you cite. If the entries do not contain the answer, say so plainly. Do not invent facts."
	user := "Question: " + question + "\n\nRelevant entries:\n" + contextBuf.String()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	reply, err := chat.Reply(ctx, system, []llm.Message{{Role: "user", Content: user}})
	if err != nil {
		return errors.Join(ErrLLM, fmt.Errorf("chat: %w", err))
	}
	fmt.Fprintln(cmd.OutOrStdout(), strings.TrimSpace(reply))
	fmt.Fprintf(cmd.ErrOrStderr(), "\n(answered with %s over %d entries via %s)\n", chat.Name(), len(matches), rc.Embedder.Name())
	return nil
}
