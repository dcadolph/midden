package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/dateutil"
	"github.com/dcadolph/midden/ics"
	"github.com/dcadolph/midden/internal/vault"
)

// ingestCmd groups passive-ingestion subcommands.
var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Pull external data into the vault as entries.",
}

// ingestICSCmd ingests a local .ics calendar export.
var ingestICSCmd = &cobra.Command{
	Use:   "ics [file]",
	Short: "Append calendar events from an .ics file as entries.",
	Args:  cobra.ExactArgs(1),
	RunE:  runIngestICS,
}

// ingestFrom and ingestTo bound the date range pulled out of the calendar.
var (
	ingestFrom string
	ingestTo   string
	ingestTag  []string
)

func init() {
	ingestICSCmd.Flags().StringVar(&ingestFrom, "from", "today", "Start of the date range to ingest.")
	ingestICSCmd.Flags().StringVar(&ingestTo, "to", "today", "End of the date range to ingest.")
	ingestICSCmd.Flags().StringSliceVarP(&ingestTag, "tag", "t", []string{"calendar"}, "Tags to attach to every ingested event.")
	ingestCmd.AddCommand(ingestICSCmd)
	rootCmd.AddCommand(ingestCmd)
}

// runIngestICS parses the .ics file and appends an entry per event inside the chosen range.
func runIngestICS(cmd *cobra.Command, args []string) error {
	from, err := dateutil.Parse(ingestFrom)
	if err != nil {
		return err
	}
	to, err := dateutil.Parse(ingestTo)
	if err != nil {
		return err
	}
	to = to.AddDate(0, 0, 1).Add(-time.Nanosecond)
	f, err := os.Open(args[0])
	if err != nil {
		return fmt.Errorf("open %s: %w", args[0], err)
	}
	defer f.Close()
	events, err := ics.Parse(f)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("parse ics: %w", err))
	}
	v, err := openVault()
	if err != nil {
		return err
	}
	tags := normalizeTags(ingestTag)
	count := 0
	for _, e := range events {
		if e.Start.Before(from) || e.Start.After(to) {
			continue
		}
		entry := vault.Entry{Time: e.Start, Tags: tags, Body: formatEvent(e)}
		if err := v.Append(entry); err != nil {
			return errors.Join(ErrVault, fmt.Errorf("append event %q: %w", e.Summary, err))
		}
		count++
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Ingested %d event(s) into the vault.\n", count)
	return nil
}

// formatEvent renders the event body markdown for an ingested calendar entry.
func formatEvent(e ics.Event) string {
	var b strings.Builder
	b.WriteString(e.Summary)
	if !e.End.IsZero() {
		fmt.Fprintf(&b, " (%s to %s)", e.Start.Format("15:04"), e.End.Format("15:04"))
	}
	if e.Location != "" {
		fmt.Fprintf(&b, "\nLocation: %s", e.Location)
	}
	if e.Description != "" {
		fmt.Fprintf(&b, "\n\n%s", e.Description)
	}
	return b.String()
}
