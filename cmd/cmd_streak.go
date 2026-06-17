package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// streakCmd prints the number of consecutive days written ending today.
var streakCmd = &cobra.Command{
	Use:   "streak",
	Short: "Print the number of consecutive days written ending today.",
	RunE:  runStreak,
}

func init() {
	rootCmd.AddCommand(streakCmd)
}

// runStreak executes the streak subcommand.
func runStreak(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	n, err := v.Streak(time.Now())
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("streak: %w", err))
	}
	fmt.Fprintln(cmd.OutOrStdout(), n)
	return nil
}
