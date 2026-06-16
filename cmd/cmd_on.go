package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// onCmd prints every entry from a single day.
var onCmd = &cobra.Command{
	Use:   "on [date]",
	Short: "Show every entry on a date (YYYY-MM-DD, today, yesterday).",
	Args:  cobra.ExactArgs(1),
	RunE:  runOn,
}

func init() {
	rootCmd.AddCommand(onCmd)
}

// runOn executes the on subcommand.
func runOn(cmd *cobra.Command, args []string) error {
	day, err := parseDayArg(args[0])
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
	if len(entries) == 0 {
		return errors.Join(ErrNotFound, fmt.Errorf("no entries on %s", day.Format("2006-01-02")))
	}
	printEntries(cmd.OutOrStdout(), entries)
	return nil
}

// parseDayArg parses a date string accepted by the on and between subcommands.
// It accepts the literal "today" and "yesterday" and the canonical YYYY-MM-DD form.
func parseDayArg(s string) (time.Time, error) {
	now := time.Now()
	switch s {
	case "today":
		return now, nil
	case "yesterday":
		return now.AddDate(0, 0, -1), nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q: expected YYYY-MM-DD, today, or yesterday", s)
	}
	return t, nil
}
