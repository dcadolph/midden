package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// betweenCmd prints every entry whose date is within an inclusive range.
var betweenCmd = &cobra.Command{
	Use:   "between [from] [to]",
	Short: "Show every entry between two dates inclusive (YYYY-MM-DD, today, yesterday).",
	Args:  cobra.ExactArgs(2),
	RunE:  runBetween,
}

func init() {
	rootCmd.AddCommand(betweenCmd)
}

// runBetween executes the between subcommand.
func runBetween(cmd *cobra.Command, args []string) error {
	from, err := parseDayArg(args[0])
	if err != nil {
		return err
	}
	to, err := parseDayArg(args[1])
	if err != nil {
		return err
	}
	v, err := openVault()
	if err != nil {
		return err
	}
	entries, err := v.ReadRange(from, to)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("read range: %w", err))
	}
	if len(entries) == 0 {
		return errors.Join(ErrNotFound, fmt.Errorf("no entries in range"))
	}
	printEntries(cmd.OutOrStdout(), entries)
	return nil
}
