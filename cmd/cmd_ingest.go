package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/ics"
	"github.com/dcadolph/midden/internal/vault"
)

// uidPrefix marks the line that records which calendar occurrence an entry came
// from, so repeated ingests recognize what is already in the vault.
const uidPrefix = "ICS-UID: "

// ingestProgressEvery is how many appended events pass between progress lines.
const ingestProgressEvery = 250

// ingestCmd groups passive-ingestion subcommands.
var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Pull external data into the vault as entries.",
}

// ingestICSCmd ingests a local .ics calendar export.
var ingestICSCmd = &cobra.Command{
	Use:   "ics [file]",
	Short: "Append calendar events from an .ics file as entries.",
	Long: "Ingest reads a calendar export and appends one entry per event occurrence.\n\n" +
		"The whole file is ingested by default, because importing years of calendar history is the " +
		"point of the command; narrow it with --from and --to when you want part of it. Recurring " +
		"series are expanded into the occurrences they actually produced, so a weekly meeting " +
		"contributes every week it happened rather than only its first. Occurrences already in the " +
		"vault are skipped, so ingesting the same export twice is safe.",
	Args: cobra.ExactArgs(1),
	RunE: runIngestICS,
}

// ingestFrom and ingestTo narrow the date range pulled out of the calendar.
// Empty means the range is derived from the file itself.
var (
	ingestFrom string
	ingestTo   string
	ingestTag  []string
)

func init() {
	ingestICSCmd.Flags().StringVar(&ingestFrom, "from", "",
		"Start of the date range to ingest (default: the earliest event in the file).")
	ingestICSCmd.Flags().StringVar(&ingestTo, "to", "",
		"End of the date range to ingest (default: the later of the last event in the file and today).")
	ingestICSCmd.Flags().StringSliceVarP(&ingestTag, "tag", "t", []string{"calendar"}, "Tags to attach to every ingested event.")
	ingestCmd.AddCommand(ingestICSCmd)
	rootCmd.AddCommand(ingestCmd)
}

// runIngestICS parses the .ics file, expands recurring series into occurrences,
// and appends the ones the vault does not already hold.
func runIngestICS(cmd *cobra.Command, args []string) error {
	events, skipped, err := parseICSFile(args[0])
	if err != nil {
		return err
	}
	if skipped > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"Warning: skipped %d event(s) with a missing or unparseable DTSTART.\n", skipped)
	}
	span, err := ingestWindow(events, ingestFrom, ingestTo)
	if err != nil {
		return err
	}
	occurrences, report := ics.Expand(events, span.From, span.To)
	reportExpansion(cmd, report)

	v, err := openVault()
	if err != nil {
		return err
	}
	seen, err := existingOccurrences(v, span)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("scan vault for existing events: %w", err))
	}
	tags := entryTags(ingestTag)
	entries := make([]vault.Entry, 0, len(occurrences))
	dupes := 0
	for _, e := range occurrences {
		body := formatEvent(e)
		if seen[occurrenceKey(e.UID, firstLine(body), e.Start)] {
			dupes++
			continue
		}
		entries = append(entries, vault.Entry{Time: e.Start, Tags: tags, Body: body})
		if len(entries)%ingestProgressEvery == 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "Prepared %d/%d event(s)\n", len(entries), len(occurrences))
		}
	}
	if err := v.AppendAll(entries); err != nil {
		return errors.Join(ErrVault, fmt.Errorf("append events: %w", err))
	}
	if dupes > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "Skipped %d event(s) already in the vault.\n", dupes)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Ingested %d event(s) into the vault (%s).\n", len(entries), span.Label())
	return nil
}

// parseICSFile reads and parses the calendar export at the given path.
func parseICSFile(path string) ([]ics.Event, int, error) {
	f, err := os.Open(path) //nolint:gosec // The calendar path is the command argument the user typed.
	if err != nil {
		return nil, 0, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	events, skipped, err := ics.Parse(f)
	if err != nil {
		return nil, 0, errors.Join(ErrVault, fmt.Errorf("parse ics: %w", err))
	}
	return events, skipped, nil
}

// ingestWindow resolves the range to ingest. An unset bound is derived from the
// file so the default is the whole export rather than a single day. The far end
// also has to be concrete because a recurring rule with no UNTIL would otherwise
// have nothing to stop it, so it reaches at least to the end of today.
func ingestWindow(events []ics.Event, since, until string) (dateRange, error) {
	span, err := resolveDateRange(since, until)
	if err != nil {
		return dateRange{}, err
	}
	first, last := eventBounds(events)
	if span.From.IsZero() && !first.IsZero() {
		span.From = dayStart(first)
	}
	if span.To.IsZero() {
		end := time.Now()
		if last.After(end) {
			end = last
		}
		span.To = dayStart(end).AddDate(0, 0, 1).Add(-time.Nanosecond)
	}
	return span, nil
}

// eventBounds returns the earliest and latest explicit start time in the file.
func eventBounds(events []ics.Event) (time.Time, time.Time) {
	var first, last time.Time
	for _, e := range events {
		if first.IsZero() || e.Start.Before(first) {
			first = e.Start
		}
		if e.Start.After(last) {
			last = e.Start
		}
	}
	return first, last
}

// reportExpansion tells the user what the expansion could not do faithfully, so
// a partial calendar is never presented as a complete one.
func reportExpansion(cmd *cobra.Command, r ics.ExpandReport) {
	w := cmd.ErrOrStderr()
	if r.Unexpanded > 0 {
		fmt.Fprintf(w, "Warning: %d recurring series use a rule midden does not expand; "+
			"only their first occurrence was ingested.\n", r.Unexpanded)
	}
	if r.Truncated > 0 {
		fmt.Fprintf(w, "Warning: %d recurring series were cut short during expansion and may be missing occurrences.\n",
			r.Truncated)
	}
	if r.Occurrences > 0 {
		fmt.Fprintf(w, "Expanded recurring series into %d occurrence(s); %d excluded, %d replaced by overrides.\n",
			r.Occurrences, r.Excluded, r.Overridden)
	}
}

// existingOccurrences collects the calendar occurrences already in the vault
// across the window. The whole range is read once here rather than a day at a
// time per event, because a backfill asks about far more events than there are
// days to hold them.
func existingOccurrences(v *vault.Vault, span dateRange) (map[string]bool, error) {
	days, err := v.ListDays()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, d := range days {
		if !span.From.IsZero() && d.Before(dayStart(span.From)) {
			continue
		}
		if !span.To.IsZero() && d.After(span.To) {
			continue
		}
		entries, err := v.ReadDay(d)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			seen[occurrenceKey(bodyUID(e.Body), firstLine(e.Body), e.Time)] = true
		}
	}
	return seen, nil
}

// occurrenceKey identifies one calendar occurrence. The start time is part of
// the key because every occurrence of a series shares the series UID, so the
// UID alone cannot tell two of them apart. Events with no UID fall back to their
// rendered first line, which both sides of the comparison derive the same way.
func occurrenceKey(uid, headline string, start time.Time) string {
	id := uid
	if id == "" {
		id = "line:" + headline
	}
	return id + "@" + start.Format("2006-01-02T15:04:05")
}

// bodyUID returns the calendar UID recorded in an entry body, or empty when the
// entry did not come from a calendar ingest.
func bodyUID(body string) string {
	for line := range strings.SplitSeq(body, "\n") {
		if after, ok := strings.CutPrefix(strings.TrimSpace(line), uidPrefix); ok {
			return after
		}
	}
	return ""
}

// formatEvent renders the event body markdown for an ingested calendar entry.
// All-day events omit clock times, and a trailing UID line makes the entry
// recognizable on later runs.
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
		fmt.Fprintf(&b, "\n%s%s", uidPrefix, e.UID)
	}
	return b.String()
}
