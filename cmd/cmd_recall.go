package cmd

import (
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/vault"
)

// recallTopK caps the number of entries returned.
var recallTopK int

// recallCmd performs a semantic search over the indexed entries.
var recallCmd = &cobra.Command{
	Use:   "recall [query...]",
	Short: "Semantic search over indexed entries (run midden reindex first).",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runRecall,
}

func init() {
	recallCmd.Flags().IntVarP(&recallTopK, "top", "k", 5, "Number of entries to return.")
	rootCmd.AddCommand(recallCmd)
}

// runRecall embeds the query, searches the index, and prints the top entries.
func runRecall(cmd *cobra.Command, args []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	rc, err := embedRecallQuery(cmd, v, strings.Join(args, " "), 60*time.Second)
	if err != nil {
		return err
	}
	matches := rc.Index.Search(rc.Query, recallTopK)
	out := make([]vault.Entry, len(matches))
	for i, m := range matches {
		out[i] = vault.Entry{Time: m.Entry.Time, Tags: m.Entry.Tags, Body: m.Entry.Body}
	}
	return printEntries(cmd.OutOrStdout(), out)
}
