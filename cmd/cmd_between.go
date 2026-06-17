package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/dateutil"
)

// betweenCmd prints every entry whose date is within an inclusive range.
var betweenCmd = &cobra.Command{
	Use:   "between [from] [to]",
	Short: "Show every entry between two dates inclusive (YYYY-MM-DD, today, yesterday, weekday, N-units-ago).",
	Args:  cobra.ExactArgs(2),
	RunE:  runBetween,
}

func init() {
	rootCmd.AddCommand(betweenCmd)
}

// runBetween executes the between subcommand.
func runBetween(cmd *cobra.Command, args []string) error {
	from, err := dateutil.Parse(args[0])
	if err != nil {
		return err
	}
	to, err := dateutil.Parse(args[1])
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
	return printEntries(cmd.OutOrStdout(), entries)
}
