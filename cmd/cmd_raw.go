package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/dateutil"
)

// rawCmd dumps the markdown content of a day file to stdout.
var rawCmd = &cobra.Command{
	Use:   "raw [date]",
	Short: "Print the raw markdown of a day file (default today).",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runRaw,
}

func init() {
	rootCmd.AddCommand(rawCmd)
}

// runRaw prints the markdown bytes for the requested day.
func runRaw(cmd *cobra.Command, args []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	day, err := chooseRawDay(args)
	if err != nil {
		return err
	}
	path := v.DayPath(day)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.Join(ErrNotFound, fmt.Errorf("no day file at %s", path))
		}
		return errors.Join(ErrVault, fmt.Errorf("open %s: %w", path, err))
	}
	defer f.Close()
	if _, err := io.Copy(cmd.OutOrStdout(), f); err != nil {
		return errors.Join(ErrVault, fmt.Errorf("read %s: %w", path, err))
	}
	return nil
}

// chooseRawDay parses the optional positional date or returns today's date.
func chooseRawDay(args []string) (time.Time, error) {
	if len(args) == 1 {
		return dateutil.Parse(args[0])
	}
	return time.Now(), nil
}
