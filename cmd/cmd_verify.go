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
			fmt.Fprintf(cmd.OutOrStdout(), "FAIL %s: %v\n", d.Format("2006-01-02"), err)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "ok   %s: %d entries\n", d.Format("2006-01-02"), len(entries))
	}
	if failures > 0 {
		return errors.Join(ErrVault, fmt.Errorf("%d file(s) failed to parse", failures))
	}
	return nil
}
