package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// tagCmd prints every entry tagged with the given label.
var tagCmd = &cobra.Command{
	Use:   "tag [name]",
	Short: "Show every entry tagged with the given label.",
	Args:  cobra.ExactArgs(1),
	RunE:  runTag,
}

func init() {
	rootCmd.AddCommand(tagCmd)
}

// runTag executes the tag subcommand.
func runTag(cmd *cobra.Command, args []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	entries, err := v.WithTag(args[0])
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("read tag: %w", err))
	}
	if len(entries) == 0 {
		return errors.Join(ErrNotFound, fmt.Errorf("no entries with tag %q", args[0]))
	}
	printEntries(cmd.OutOrStdout(), entries)
	return nil
}
