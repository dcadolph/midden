package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// verifyCmd walks every day file and reports parse failures.
var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Check that every day file in the vault parses cleanly.",
	RunE:  runVerify,
}

func init() {
	rootCmd.AddCommand(verifyCmd)
}

// runVerify executes the verify subcommand.
// FAIL diagnostics go to stderr so the per-day summary on stdout stays clean.
// Parse failures return a plain error because nothing was written, so the
// process exits with the generic failure code rather than the vault code.
func runVerify(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	days, err := v.ListDays()
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("list days: %w", err))
	}
	var failures int
	for _, d := range days {
		entries, err := v.ReadDay(d)
		if err != nil {
			failures++
			fmt.Fprintf(cmd.ErrOrStderr(), "FAIL %s: %v\n", d.Format(layoutDate), err)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "ok   %s: %d entries\n", d.Format(layoutDate), len(entries))
	}
	if failures > 0 {
		return fmt.Errorf("%d file(s) failed to parse", failures)
	}
	return nil
}
