package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/jsonutil"
	"github.com/dcadolph/midden/internal/vault"
)

// exportFormat selects the output shape for the export subcommand.
var exportFormat string

// exportCmd dumps every entry in the vault in the requested format.
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Dump every entry as JSON, JSON Lines, or concatenated markdown.",
	RunE:  runExport,
}

func init() {
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "json", "Output format: json, jsonl, or md.")
	rootCmd.AddCommand(exportCmd)
}

// runExport executes the export subcommand.
// The global --json flag implies --format json unless --format was set explicitly.
func runExport(cmd *cobra.Command, _ []string) error {
	if jsonOutput && !cmd.Flags().Changed("format") {
		exportFormat = "json"
	}
	switch exportFormat {
	case "json", "jsonl", "md":
	default:
		return fmt.Errorf("unsupported format %q: expected json, jsonl, or md", exportFormat)
	}
	v, err := openVault()
	if err != nil {
		return err
	}
	days, err := v.ListDays()
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("list days: %w", err))
	}
	w := cmd.OutOrStdout()
	switch exportFormat {
	case "jsonl":
		return exportAsJSONL(w, v, days)
	case "md":
		return exportAsMarkdown(w, v, days)
	}
	return exportAsJSON(w, v, days)
}

// exportAsJSON writes one JSON array containing every entry.
func exportAsJSON(w io.Writer, v *vault.Vault, days []time.Time) error {
	var all []entryJSON
	for _, d := range days {
		entries, err := v.ReadDay(d)
		if err != nil {
			return errors.Join(ErrVault, fmt.Errorf("read day %s: %w", d.Format(layoutDate), err))
		}
		all = append(all, entriesToJSON(entries)...)
	}
	return jsonutil.Encode(w, all, jsonPretty)
}

// exportAsJSONL writes one JSON object per line.
func exportAsJSONL(w io.Writer, v *vault.Vault, days []time.Time) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, d := range days {
		entries, err := v.ReadDay(d)
		if err != nil {
			return errors.Join(ErrVault, fmt.Errorf("read day %s: %w", d.Format(layoutDate), err))
		}
		for _, j := range entriesToJSON(entries) {
			if err := enc.Encode(j); err != nil {
				return fmt.Errorf("encode entry: %w", err)
			}
		}
	}
	return nil
}

// exportAsMarkdown concatenates every day file in date order.
func exportAsMarkdown(w io.Writer, v *vault.Vault, days []time.Time) error {
	for _, d := range days {
		path := v.DayPath(d)
		data, err := v.ReadBytes(path)
		if err != nil {
			return errors.Join(ErrVault, fmt.Errorf("read day %s: %w", d.Format(layoutDate), err))
		}
		if _, err := w.Write(data); err != nil {
			return fmt.Errorf("write markdown: %w", err)
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return fmt.Errorf("write separator: %w", err)
		}
	}
	return nil
}
