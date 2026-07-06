package cmd

import (
	"github.com/spf13/cobra"
)

// todayCmd opens today's day file in the user's editor.
var todayCmd = &cobra.Command{
	Use:   "today",
	Short: "Open today's day file in the editor.",
	RunE:  runToday,
}

func init() {
	rootCmd.AddCommand(todayCmd)
}

// runToday opens today's day file through the edit flow, which handles
// encrypted vaults by decrypting to a temp file and re-encrypting on save.
func runToday(cmd *cobra.Command, _ []string) error {
	return runEdit(cmd, nil)
}
