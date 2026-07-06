package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/jsonutil"
)

// statsTopTags is the maximum number of tags included in the human-readable stats summary.
var statsTopTags int

// statsCmd prints a vault summary.
var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show a summary of the vault: days, entries, words, top tags.",
	RunE:  runStats,
}

func init() {
	statsCmd.Flags().IntVar(&statsTopTags, "top-tags", 10, "Number of top tags to include in the summary.")
	rootCmd.AddCommand(statsCmd)
}

// runStats executes the stats subcommand.
func runStats(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	s, err := v.ComputeStats(statsTopTags)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("compute stats: %w", err))
	}
	if jsonOutput {
		return jsonutil.Encode(cmd.OutOrStdout(), s, jsonPretty)
	}
	w := cmd.OutOrStdout()
	color := isTerminal(w)
	heading := func(label string) {
		if color {
			fmt.Fprintf(w, "%s%s%s\n", colorBold, label, colorReset)
		} else {
			fmt.Fprintln(w, label)
		}
	}
	heading("Counts")
	fmt.Fprintf(w, "  Days:    %d\n", s.Days)
	fmt.Fprintf(w, "  Entries: %d\n", s.Entries)
	fmt.Fprintf(w, "  Words:   %d\n", s.Words)
	fmt.Fprintf(w, "  Tags:    %d\n", s.Tags)
	if !s.FirstEntry.IsZero() {
		heading("Span")
		fmt.Fprintf(w, "  First: %s\n", s.FirstEntry.Format(layoutDateTime))
		fmt.Fprintf(w, "  Last:  %s\n", s.LastEntry.Format(layoutDateTime))
	}
	if len(s.TopTags) > 0 {
		heading("Top tags")
		for _, c := range s.TopTags {
			fmt.Fprintf(w, "  %5d  %s\n", c.Count, c.Tag)
		}
	}
	return nil
}
