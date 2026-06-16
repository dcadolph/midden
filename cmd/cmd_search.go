package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// searchCmd prints every entry whose body or tags contain the query substring.
var searchCmd = &cobra.Command{
	Use:   "search [query...]",
	Short: "Find entries whose body or tags contain the query (case-insensitive).",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSearch,
}

func init() {
	rootCmd.AddCommand(searchCmd)
}

// runSearch executes the search subcommand.
func runSearch(cmd *cobra.Command, args []string) error {
	q := strings.Join(args, " ")
	v, err := openVault()
	if err != nil {
		return err
	}
	entries, err := v.Search(q)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("search: %w", err))
	}
	if len(entries) == 0 {
		return errors.Join(ErrNotFound, fmt.Errorf("no entries match %q", q))
	}
	printEntries(cmd.OutOrStdout(), entries)
	return nil
}
