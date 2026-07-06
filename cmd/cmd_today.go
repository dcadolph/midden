package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

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

// runToday ensures today's day file exists and opens it in the editor.
func runToday(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	path, err := v.EnsureDayFile(time.Now())
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("ensure today: %w", err))
	}
	editor := chooseEditor()
	c := exec.Command(editor, path) //nolint:gosec // Editor comes from user config or environment.
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return errors.Join(ErrEditor, fmt.Errorf("editor %s exited: %w", filepath.Base(editor), err))
	}
	return nil
}
