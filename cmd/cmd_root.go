package cmd

import "github.com/spf13/cobra"

// vaultDir is the global override for the vault directory.
// Empty means the default resolver picks it.
var vaultDir string

// jsonOutput requests JSON-formatted stdout from any subcommand that supports it.
var jsonOutput bool

// jsonPretty indents JSON output when set.
var jsonPretty bool

// noColor disables ANSI escape sequences in terminal output.
var noColor bool

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
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Emit structured JSON to stdout instead of human-readable text.")
	rootCmd.PersistentFlags().BoolVar(&jsonPretty, "pretty", false, "Indent JSON output (only meaningful with --json).")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable ANSI color escape sequences.")
	rootCmd.PersistentFlags().StringVar(&passphraseFlag, "passphrase", "", "Vault passphrase for encrypted vaults; prefer $MIDDEN_PASSPHRASE or interactive prompt.")
}
