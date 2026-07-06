package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/dateutil"
)

// pathCmd prints filesystem paths inside the vault.
var pathCmd = &cobra.Command{
	Use:   "path [date]",
	Short: "Print the vault root, or the day file path when a date is supplied.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPath,
}

func init() {
	rootCmd.AddCommand(pathCmd)
}

// runPath prints either the vault root or the resolved day file path.
// It opens the vault without unlocking so printing a path never prompts for a passphrase.
func runPath(cmd *cobra.Command, args []string) error {
	v, err := openVaultRaw()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), v.Dir)
		return nil
	}
	day, err := dateutil.Parse(args[0])
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), v.DayPath(day))
	return nil
}
