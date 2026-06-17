package cmd

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// undoCmd removes the most recent entry from the most recent day file.
var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Remove the most recent entry written to the vault.",
	RunE:  runUndo,
}

func init() {
	rootCmd.AddCommand(undoCmd)
}

// runUndo finds the newest day file with entries and trims the last entry block from it.
func runUndo(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	days, err := v.ListDays()
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("list days: %w", err))
	}
	for i := len(days) - 1; i >= 0; i-- {
		path := v.DayPath(days[i])
		data, err := v.ReadBytes(path)
		if err != nil {
			return errors.Join(ErrVault, fmt.Errorf("read %s: %w", path, err))
		}
		trimmed, removed := trimLastEntry(data)
		if !removed {
			continue
		}
		if err := v.WriteBytes(path, trimmed); err != nil {
			return errors.Join(ErrVault, fmt.Errorf("write %s: %w", path, err))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Removed last entry from %s\n", days[i].Format("2006-01-02"))
		return nil
	}
	return errors.Join(ErrNotFound, fmt.Errorf("no entries to remove"))
}

// trimLastEntry removes the last `## HH:MM:SS ...` block (including its body and trailing blank lines).
// It returns the trimmed bytes and a flag indicating whether anything was removed.
func trimLastEntry(data []byte) ([]byte, bool) {
	idx := bytes.LastIndex(data, []byte("\n## "))
	if idx < 0 {
		if bytes.HasPrefix(data, []byte("## ")) {
			return data[:0], true
		}
		return data, false
	}
	return bytes.TrimRight(data[:idx+1], "\n"), true
}
