package cmd

import "github.com/spf13/cobra"

// vaultDir is the global override for the vault directory.
// Empty means the default resolver picks it.
var vaultDir string

// rootCmd is the parent for all midden subcommands.
var rootCmd = &cobra.Command{
	Use:           "midden",
	Short:         "Personal markdown journal.",
	Long:          "Midden keeps a personal journal as plain markdown files, one per day, that any tool can read and grep can search.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&vaultDir, "vault", "", "Override the vault directory (default $MIDDEN_HOME or ~/midden).")
}
