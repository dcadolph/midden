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
// Events carrying a UID are deduplicated against entries already in the vault.
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
	defer func() { _ = f.Close() }()
	events, skipped, err := ics.Parse(f)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("parse ics: %w", err))
	}
	if skipped > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"Warning: skipped %d event(s) with a missing or unparseable DTSTART.\n", skipped)
	}
	v, err := openVault()
	if err != nil {
		return err
	}
	tags := entryTags(ingestTag)
	count := 0
	dupes := 0
	recurring := 0
	for _, e := range events {
		if e.Start.Before(from) || e.Start.After(to) {
			continue
		}
		if e.Recurs {
			recurring++
		}
		if e.UID != "" {
			dup, err := dayHasUID(v, e.Start, e.UID)
			if err != nil {
				return errors.Join(ErrVault, fmt.Errorf("dedupe event %q: %w", e.Summary, err))
			}
			if dup {
				dupes++
				continue
			}
		}
		entry := vault.Entry{Time: e.Start, Tags: tags, Body: formatEvent(e)}
		if err := v.Append(entry); err != nil {
			return errors.Join(ErrVault, fmt.Errorf("append event %q: %w", e.Summary, err))
		}
		count++
	}
	if recurring > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"Warning: %d event(s) in range recur; recurrences are not expanded.\n", recurring)
	}
	if dupes > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "Skipped %d duplicate event(s) already in the vault.\n", dupes)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Ingested %d event(s) into the vault.\n", count)
	return nil
}

// dayHasUID reports whether any entry on the event's day already contains the UID.
func dayHasUID(v *vault.Vault, day time.Time, uid string) (bool, error) {
	entries, err := v.ReadDay(day)
	if err != nil {
		return false, fmt.Errorf("read day: %w", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Body, uid) {
			return true, nil
		}
	}
	return false, nil
}

// formatEvent renders the event body markdown for an ingested calendar entry.
// All-day events omit clock times, and a trailing ICS-UID line makes the entry
// discoverable for deduplication on later runs.
func formatEvent(e ics.Event) string {
	var b strings.Builder
	b.WriteString(e.Summary)
	switch {
	case e.AllDay:
		b.WriteString(" (all day)")
	case !e.End.IsZero():
		fmt.Fprintf(&b, " (%s to %s)", e.Start.Format("15:04"), e.End.Format("15:04"))
	}
	if e.Location != "" {
		fmt.Fprintf(&b, "\nLocation: %s", e.Location)
	}
	if e.Description != "" {
		fmt.Fprintf(&b, "\n\n%s", e.Description)
	}
	if e.UID != "" {
		fmt.Fprintf(&b, "\nICS-UID: %s", e.UID)
	}
	return b.String()
}
