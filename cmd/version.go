package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is the current midden release tag.
// It is updated by hand on each release.
const Version = "0.1.0"

// versionCmd prints the build version.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the midden version.",
	Run: func(cmd *cobra.Command, _ []string) {
		fmt.Fprintln(cmd.OutOrStdout(), Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
