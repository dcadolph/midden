package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/dateutil"
)

// flashbackCmd prints entries from past years on the same calendar date.
var flashbackCmd = &cobra.Command{
	Use:   "flashback [date]",
	Short: "Show entries from past years on the same calendar date (default today).",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runFlashback,
}

func init() {
	rootCmd.AddCommand(flashbackCmd)
}

// runFlashback executes the flashback subcommand.
func runFlashback(cmd *cobra.Command, args []string) error {
	target := time.Now()
	if len(args) == 1 {
		t, err := dateutil.Parse(args[0])
		if err != nil {
			return err
		}
		target = t
	}
	v, err := openVault()
	if err != nil {
		return err
	}
	entries, err := v.Flashback(target.Month(), target.Day())
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("flashback: %w", err))
	}
	if len(entries) == 0 {
		return errors.Join(ErrNotFound, fmt.Errorf("no entries on %s in any prior year", target.Format("01-02")))
	}
	return printEntries(cmd.OutOrStdout(), entries)
}
