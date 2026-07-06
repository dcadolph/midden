package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// lastCount is the number of most-recent entries lastCmd prints.
var lastCount int

// lastCmd prints the most recent entry, or the n most recent when -n is passed.
var lastCmd = &cobra.Command{
	Use:   "last",
	Short: "Show the most recent entry (or n with --count).",
	RunE:  runLast,
}

func init() {
	lastCmd.Flags().IntVarP(&lastCount, "count", "n", 1, "Number of recent entries to show.")
	rootCmd.AddCommand(lastCmd)
}

// runLast executes the last subcommand.
func runLast(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	entries, err := v.Recent(lastCount)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("read recent: %w", err))
	}
	if len(entries) == 0 && !jsonOutput {
		return errors.Join(ErrNotFound, fmt.Errorf("no entries yet"))
	}
	return printEntries(cmd.OutOrStdout(), entries)
}
