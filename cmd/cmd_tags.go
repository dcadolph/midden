package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/jsonutil"
)

// tagsLimit caps the size of the tag histogram printed by tagsCmd.
var tagsLimit int

// tagsCmd prints the tag histogram across the vault.
var tagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "List every tag with its entry count.",
	RunE:  runTags,
}

func init() {
	tagsCmd.Flags().IntVarP(&tagsLimit, "limit", "n", 0, "Limit the number of tags shown; zero means all.")
	rootCmd.AddCommand(tagsCmd)
}

// runTags executes the tags subcommand.
func runTags(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	counts, err := v.TagCounts(tagsLimit)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("read tags: %w", err))
	}
	if jsonOutput {
		return jsonutil.Encode(cmd.OutOrStdout(), counts, jsonPretty)
	}
	color := isTerminal(cmd.OutOrStdout())
	for _, c := range counts {
		if color {
			fmt.Fprintf(cmd.OutOrStdout(), "%s%5d%s  %s%s%s\n", colorBold, c.Count, colorReset, colorYellow, c.Tag, colorReset)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "%5d  %s\n", c.Count, c.Tag)
		}
	}
	return nil
}
