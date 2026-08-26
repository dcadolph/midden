package cmd

import (
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/vault"
)

// recallTopK caps the number of entries returned.
var recallTopK int

// recallSince and recallUntil bound the entries the search may return.
var (
	recallSince string
	recallUntil string
)

// recallCmd performs a semantic search over the indexed entries.
var recallCmd = &cobra.Command{
	Use:   "recall [query...]",
	Short: "Semantic search over indexed entries (run midden reindex first).",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runRecall,
}

func init() {
	recallCmd.Flags().IntVarP(&recallTopK, "top", "k", 5, "Number of entries to return.")
	recallCmd.Flags().StringVar(&recallSince, "since", "", "Only consider entries on or after this date.")
	recallCmd.Flags().StringVar(&recallUntil, "until", "", "Only consider entries on or before this date.")
	rootCmd.AddCommand(recallCmd)
}

// runRecall embeds the query, searches the index within the requested date
// range, and prints the top entries.
func runRecall(cmd *cobra.Command, args []string) error {
	span, err := resolveDateRange(recallSince, recallUntil)
	if err != nil {
		return err
	}
	v, err := openVault()
	if err != nil {
		return err
	}
	rc, err := embedRecallQuery(cmd, v, strings.Join(args, " "), 60*time.Second)
	if err != nil {
		return err
	}
	matches := rc.Index.SearchRange(rc.Query, recallTopK, span.From, span.To)
	out := make([]vault.Entry, len(matches))
	for i, m := range matches {
		out[i] = vault.Entry{Time: m.Entry.Time, Tags: m.Entry.Tags, Body: m.Entry.Body}
	}
	return printEntries(cmd.OutOrStdout(), out)
}
