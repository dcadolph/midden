package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/dateutil"
	"github.com/dcadolph/midden/internal/vault"
)

// importDate names the local date assigned to the imported entry.
var importDate string

// importTags attach to the imported entry.
var importTags []string

// importCmd appends a markdown file as one entry on the chosen date.
var importCmd = &cobra.Command{
	Use:   "import [file]",
	Short: "Append a markdown file as a single entry on the chosen date.",
	Args:  cobra.ExactArgs(1),
	RunE:  runImport,
}

func init() {
	importCmd.Flags().StringVarP(&importDate, "date", "d", "today", "Date to file the entry under (YYYY-MM-DD or relative).")
	importCmd.Flags().StringSliceVarP(&importTags, "tag", "t", nil, "Tag to attach to the imported entry.")
	rootCmd.AddCommand(importCmd)
}

// runImport reads the file body and appends it as an entry on the resolved date.
func runImport(cmd *cobra.Command, args []string) error {
	when, err := importTimestamp(importDate)
	if err != nil {
		return err
	}
	body, err := readImportBody(args[0])
	if err != nil {
		return err
	}
	v, err := openVault()
	if err != nil {
		return err
	}
	if err := v.Append(vault.Entry{Time: when, Tags: normalizeTags(importTags), Body: body}); err != nil {
		return errors.Join(ErrVault, fmt.Errorf("append imported entry: %w", err))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Imported %s into %s\n", args[0], when.Format("2006-01-02 15:04:05"))
	return nil
}

// readImportBody reads the file at path or stdin when path is "-".
func readImportBody(path string) (string, error) {
	if path == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return string(data), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file %s: %w", path, err)
	}
	return string(data), nil
}

// importTimestamp resolves the import date string to a local timestamp.
// Today and relative dates keep the current clock time so the entry sorts naturally;
// absolute dates use noon local so entries land mid-day without timezone surprises.
func importTimestamp(s string) (time.Time, error) {
	day, err := dateutil.Parse(s)
	if err != nil {
		return time.Time{}, err
	}
	if s == "today" || s == "" {
		return time.Now(), nil
	}
	return time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, day.Location()), nil
}
