package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/dateutil"
)

// onCmd prints every entry from a single day.
var onCmd = &cobra.Command{
	Use:   "on [date]",
	Short: "Show every entry on a date (YYYY-MM-DD, today, yesterday, weekday, N-units-ago).",
	Args:  cobra.ExactArgs(1),
	RunE:  runOn,
}

func init() {
	rootCmd.AddCommand(onCmd)
}

// runOn executes the on subcommand.
func runOn(cmd *cobra.Command, args []string) error {
	day, err := dateutil.Parse(args[0])
	if err != nil {
		return err
	}
	v, err := openVault()
	if err != nil {
		return err
	}
	entries, err := v.ReadDay(day)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("read day: %w", err))
	}
	if len(entries) == 0 && !jsonOutput {
		return errors.Join(ErrNotFound, fmt.Errorf("no entries on %s", day.Format(layoutDate)))
	}
	return printEntries(cmd.OutOrStdout(), entries)
}
