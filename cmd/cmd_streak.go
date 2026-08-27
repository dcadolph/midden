package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/dcadolph/midden/internal/vault"
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
	// Only days the person actually wrote something count. Imported calendar
	// events would otherwise report a streak for appointments merely attended.
	n, err := v.Streak(time.Now(), vault.Entry.Authored)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("streak: %w", err))
	}
	fmt.Fprintln(cmd.OutOrStdout(), n)
	return nil
}
