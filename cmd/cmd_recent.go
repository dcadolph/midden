package cmd

import (
	"time"

	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// recentCount is the number of most recent entries to print.
var recentCount int

// recentCmd prints the most recent N entries across all day files.
var recentCmd = &cobra.Command{
	Use:   "recent",
	Short: "Show the most recent entries.",
	RunE:  runRecent,
}

// recentFuture includes entries dated after now, which an imported calendar holds.
var recentFuture bool

func init() {
	recentCmd.Flags().IntVarP(&recentCount, "count", "n", 10, "Number of entries to show.")
	recentCmd.Flags().BoolVar(&recentFuture, "future", false,
		"Include entries dated after now, such as calendar appointments that have not happened yet.")
	rootCmd.AddCommand(recentCmd)
}

// runRecent executes the recent subcommand.
func runRecent(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	cutoff := time.Time{}
	if !recentFuture {
		cutoff = time.Now()
	}
	entries, err := v.RecentBefore(recentCount, cutoff)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("read recent: %w", err))
	}
	if len(entries) == 0 && !jsonOutput {
		return errors.Join(ErrNotFound, fmt.Errorf("no entries yet"))
	}
	return printEntries(cmd.OutOrStdout(), entries)
}
