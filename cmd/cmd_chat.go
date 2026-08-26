package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/index"
	"github.com/dcadolph/midden/llm"
)

// chatTopK caps the number of entries fed to the chat model as context.
var chatTopK int

// chatSince and chatUntil bound the entries the answer may draw on.
var (
	chatSince string
	chatUntil string
)

// chatSweep answers from every entry in scope rather than the closest matches.
var chatSweep bool

// Timeouts for the chat call. Current models reason before answering, so even a
// single reply can take minutes over a large record; a sweep may summarize a
// decade in chunks and is budgeted against that rather than against one reply.
const (
	chatTimeout      = 5 * time.Minute
	chatSweepTimeout = 30 * time.Minute
)

// digestTopTags caps the tag histogram supplied with every answer.
const digestTopTags = 25

// chatSystem frames what the model is reading. The two halves of the context
// carry different authority: the summary counts every entry in scope, while the
// entries are the quotable text and may be a sample, so the model is told not
// to read absence from the sample as absence from the record.
const chatSystem = "You are reading a personal journal and calendar archive. " +
	"Answer only from the record supplied below. " +
	"The corpus summary is a complete count over every indexed entry in scope: use it for questions " +
	"about the shape of the record as a whole, such as which periods, people, places, or themes recur. " +
	"The entries after it are the specific text you may quote, and they may be a sample rather than the " +
	"whole record, so never conclude that something did not happen merely because it is absent from them. " +
	"Quote the date of any entry you cite. Say plainly when the record does not answer the question. " +
	"Do not invent facts."

// chatCmd answers a question using recalled entries as context.
var chatCmd = &cobra.Command{
	Use:   "chat [question...]",
	Short: "Answer a question using semantic recall plus a chat model.",
	Long: "Chat answers a question from the indexed vault.\n\n" +
		"By default it retrieves the entries closest to the question. Questions about a period of time " +
		"are answered better by scoping the range with --since and --until, which sweeps every entry in " +
		"that range instead of ranking them, and questions about the record as a whole are answered " +
		"better with --sweep. Every answer also receives a summary counted over the whole vault in scope.",
	Args: cobra.MinimumNArgs(1),
	RunE: runChat,
}

func init() {
	chatCmd.Flags().IntVarP(&chatTopK, "top", "k", 8, "Number of entries to include as context.")
	chatCmd.Flags().StringVar(&chatSince, "since", "", "Only consider entries on or after this date.")
	chatCmd.Flags().StringVar(&chatUntil, "until", "", "Only consider entries on or before this date.")
	chatCmd.Flags().BoolVar(&chatSweep, "sweep", false,
		"Answer from every entry in scope instead of the closest matches, summarizing in chunks when the range is large.")
	rootCmd.AddCommand(chatCmd)
}

// runChat embeds the question, gathers the entries in scope, and asks the chat
// model to answer using only those entries and the corpus summary as evidence.
func runChat(cmd *cobra.Command, args []string) error {
	span, err := resolveDateRange(chatSince, chatUntil)
	if err != nil {
		return err
	}
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

	// A bounded range is a question about a period, which a ranked head cannot
	// answer, so scoping the range implies sweeping it.
	sweep := chatSweep || span.Bounded()
	timeout := chatTimeout
	if sweep {
		timeout = chatSweepTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var (
		body  string
		count int
	)
	if sweep {
		entries := rc.Index.InRange(span.From, span.To)
		count = len(entries)
		body, err = sweepContext(ctx, cmd, chat, question, entries)
		if err != nil {
			return errors.Join(ErrLLM, err)
		}
	} else {
		matches := rc.Index.SearchRange(rc.Query, chatTopK, span.From, span.To)
		count = len(matches)
		body = renderEntries(matchedEntries(matches))
	}

	digest := rc.Index.Digest(span.From, span.To, digestTopTags)
	if digest.Entries == 0 {
		return errors.Join(ErrNotFound, fmt.Errorf("no indexed entries in range (%s)", span.Label()))
	}
	user := "Question: " + question + "\n\n" + renderDigest(digest, span.Label()) + "\nRecord:\n" + body
	reply, err := chat.Reply(ctx, chatSystem, []llm.Message{{Role: "user", Content: user}})
	if err != nil {
		return errors.Join(ErrLLM, fmt.Errorf("chat: %w", err))
	}
	fmt.Fprintln(cmd.OutOrStdout(), strings.TrimSpace(reply))
	fmt.Fprintf(cmd.ErrOrStderr(), "\n(answered with %s over %d of %d entries in %s via %s)\n",
		chat.Name(), count, digest.Entries, span.Label(), rc.Embedder.Name())
	return nil
}

// matchedEntries strips the similarity scores from ranked matches.
func matchedEntries(matches []index.Match) []index.Entry {
	out := make([]index.Entry, len(matches))
	for i, m := range matches {
		out[i] = m.Entry
	}
	return out
}
