package cmd

import (
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

func init() {
	recentCmd.Flags().IntVarP(&recentCount, "count", "n", 10, "Number of entries to show.")
	rootCmd.AddCommand(recentCmd)
}

// runRecent executes the recent subcommand.
func runRecent(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	entries, err := v.Recent(recentCount)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("read recent: %w", err))
	}
	return printEntries(cmd.OutOrStdout(), entries)
}
