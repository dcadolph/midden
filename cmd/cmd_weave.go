package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/jsonutil"
	"github.com/dcadolph/midden/internal/vault"
	"github.com/dcadolph/midden/internal/weave"
)

// Weave options.
var (
	weaveSince   string
	weaveUntil   string
	weaveMin     int
	weaveLimit   int
	weaveSources []string
	weaveTags    []string
	weaveExplain bool
)

// weaveCmd surfaces the shape of the record over time.
var weaveCmd = &cobra.Command{
	Use:   "weave",
	Short: "Show what recurs in the record, when it started, and when it stopped.",
	Long: "Weave finds the threads running through the vault.\n\n" +
		"Search answers what you already know to ask about. Weave answers what you cannot ask, " +
		"because a person can recall what they did but cannot perceive absence: nothing marks the " +
		"last time something happened. Every figure here is counted rather than inferred, so there " +
		"is no model in the path and nothing to invent.",
	RunE: runWeave,
}

func init() {
	weaveCmd.Flags().StringVar(&weaveSince, "since", "", "Only consider entries on or after this date.")
	weaveCmd.Flags().StringVar(&weaveUntil, "until", "", "Only consider entries on or before this date.")
	weaveCmd.Flags().IntVar(&weaveMin, "min", 5, "Fewest occurrences a pattern needs to count as a thread.")
	weaveCmd.Flags().IntVar(&weaveLimit, "top", 12, "Maximum threads to show per section.")
	weaveCmd.Flags().BoolVar(&weaveExplain, "explain", false,
		"Show the distinct headlines folded into each thread, so a grouping can be checked before it is believed.")
	weaveCmd.Flags().StringSliceVar(&weaveTags, "tag", nil,
		"Only weave entries carrying one of these tags. Commit history repeats boilerplate subjects across repositories, so restricting to a life source such as calendar keeps those out of the threads.")
	weaveCmd.Flags().StringSliceVar(&weaveSources, "source", []string{"calendar", "git"},
		"Source tags to compare when looking for days where parts of your life meet.")
	rootCmd.AddCommand(weaveCmd)
}

// runWeave reads the vault, detects threads, and reports what ended, what began,
// and where separate parts of the record meet.
func runWeave(cmd *cobra.Command, _ []string) error {
	span, err := resolveDateRange(weaveSince, weaveUntil)
	if err != nil {
		return err
	}
	v, err := openVault()
	if err != nil {
		return err
	}
	entries, err := entriesInRange(v, span)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("scan vault: %w", err))
	}
	if len(entries) == 0 {
		return errors.Join(ErrNotFound, fmt.Errorf("no entries in range (%s)", span.Label()))
	}

	now := time.Now()
	threadInput := entries
	if len(weaveTags) > 0 {
		threadInput = filterByTag(entries, weaveTags)
		if len(threadInput) == 0 {
			return errors.Join(ErrNotFound, fmt.Errorf("no entries carrying %s", strings.Join(weaveTags, ", ")))
		}
	}
	opts := weave.DefaultOptions(now)
	opts.MinCount = weaveMin
	threads := weave.Threads(threadInput, opts)
	handoffs := weave.Handoffs(threads, weave.DefaultHandoffOptions())
	overlaps := weave.Overlaps(entries, weaveSources, now)
	gaps := weave.GapsBySource(entries, weaveSources, weave.DefaultGapOptions(now))
	eras := weave.Eras(entries, weave.DefaultEraOptions(now))

	if jsonOutput {
		return jsonutil.Encode(cmd.OutOrStdout(), weaveJSON{
			Threads:  threadsToJSON(threads),
			Handoffs: handoffsToJSON(handoffs),
		}, jsonPretty)
	}
	writeWeave(cmd.OutOrStdout(), threads, handoffs, overlaps, gaps, eras, weaveLimit)
	return nil
}

// writeWeave renders the human-readable report.
func writeWeave(
	w io.Writer,
	threads []weave.Thread,
	handoffs []weave.Handoff,
	overlaps []weave.Overlap,
	gaps []weave.Gap,
	eras []weave.Era,
	limit int,
) {
	byStatus := func(s weave.Status) []weave.Thread {
		var out []weave.Thread
		for _, t := range threads {
			if t.Status == s {
				out = append(out, t)
			}
		}
		return out
	}

	if len(eras) > 1 {
		section(w, "Eras", "the chapters of the record, found by where its volume shifts")
		for _, e := range eras {
			fmt.Fprintf(w, "  %s to %s  %3d months  %6d entries  ~%.0f/month\n",
				e.From.Format("2006-01"), e.To.Format("2006-01"), e.Months, e.Entries, e.PerMonth)
		}
	}

	if len(gaps) > 0 {
		section(w, "Silences", "stretches where the record itself went quiet")
		for _, g := range head(gaps, limit) {
			scope := "whole record"
			if g.Source != "" {
				scope = g.Source + " only"
			}
			fmt.Fprintf(w, "  %s to %s  %d months, %d entries  [%s]  (about %.0f/month before, %.0f after)\n",
				g.From.Format("2006-01"), g.To.Format("2006-01"), g.Months, g.Entries, scope, g.Before, g.After)
		}
	}

	ended := byStatus(weave.Ended)
	section(w, "Ended", "things that stopped without anything marking the last one")
	for _, t := range head(ended, limit) {
		fmt.Fprintf(w, "  %-44s %4dx over %4s   last %s, %s ago\n",
			truncate(t.Label, 44), t.Count, years(t.SpanDays), t.Last.Format(layoutDate), months(t.SilentDays))
	}
	if len(ended) == 0 {
		fmt.Fprintln(w, "  nothing has gone quiet")
	}

	dormant := byStatus(weave.Dormant)
	if len(dormant) > 0 {
		section(w, "Between seasons", "quiet, but they have come back from a gap this long before")
		for _, t := range head(dormant, limit) {
			fmt.Fprintf(w, "  %-44s %4dx, last %s, quiet %s (longest gap before: %s)\n",
				truncate(t.Label, 44), t.Count, t.Last.Format(layoutDate),
				months(t.SilentDays), months(t.MaxGap))
			writeVariants(w, t)
		}
	}

	emerging := byStatus(weave.Emerging)
	section(w, "Started", "threads that began recently")
	for _, t := range head(emerging, limit) {
		fmt.Fprintf(w, "  %-44s %4dx since %s\n", truncate(t.Label, 44), t.Count, t.First.Format(layoutDate))
	}
	if len(emerging) == 0 {
		fmt.Fprintln(w, "  nothing new")
	}

	ongoing := byStatus(weave.Ongoing)
	section(w, "Ongoing", "the steady weight of the record")
	for _, t := range head(ongoing, limit) {
		fmt.Fprintf(w, "  %-44s %4dx every ~%d days\n", truncate(t.Label, 44), t.Count, t.MedianGap)
		writeVariants(w, t)
	}
	if len(ongoing) == 0 {
		fmt.Fprintln(w, "  nothing recurring")
	}

	if len(handoffs) > 0 {
		section(w, "Handoffs", "one thread ended and another began soon after")
		for _, h := range head(handoffs, limit) {
			fmt.Fprintf(w, "  %s\n", truncate(h.From.Label, 60))
			fmt.Fprintf(w, "    ended %s after %dx, then %s began %d days later\n",
				h.From.Last.Format(layoutDate), h.From.Count, truncate(h.To.Label, 40), h.GapDays)
		}
	}

	if len(overlaps) > 0 {
		section(w, "Crossings", "days where separate parts of the record meet")
		for _, o := range head(overlaps, limit) {
			parts := make([]string, 0, len(o.Counts))
			for src, n := range o.Counts {
				parts = append(parts, fmt.Sprintf("%d %s", n, src))
			}
			fmt.Fprintf(w, "  %s  (%s)\n", o.Day.Format(layoutDate), strings.Join(parts, ", "))
			for _, src := range weaveSources {
				if h := o.Headlines[src]; h != "" {
					fmt.Fprintf(w, "    %-9s %s\n", src+":", truncate(h, 62))
				}
			}
		}
	}
}

// writeVariants lists the headlines folded into a thread when explaining. A
// claim that something ended rests entirely on what was grouped together, so
// the grouping has to be inspectable.
func writeVariants(w io.Writer, t weave.Thread) {
	if !weaveExplain || len(t.Variants) < 2 {
		return
	}
	for _, v := range t.Variants {
		fmt.Fprintf(w, "        · %s\n", truncate(v, 68))
	}
}

// section writes a titled block header.
func section(w io.Writer, title, blurb string) {
	fmt.Fprintf(w, "\n%s — %s\n\n", title, blurb)
}

// head returns at most n items from the slice.
func head[T any](items []T, n int) []T {
	if n > 0 && len(items) > n {
		return items[:n]
	}
	return items
}

// years renders a day count as an approximate number of years.
func years(days int) string {
	return fmt.Sprintf("%.1fy", float64(days)/365)
}

// months renders a day count as an approximate number of months.
func months(days int) string {
	if days < 60 {
		return fmt.Sprintf("%d days", days)
	}
	return fmt.Sprintf("%d months", days/30)
}

// truncate shortens a label to fit a column.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// filterByTag keeps only entries carrying one of the given tags.
func filterByTag(entries []vault.Entry, tags []string) []vault.Entry {
	want := make(map[string]bool, len(tags))
	for _, t := range tags {
		want[strings.ToLower(strings.TrimPrefix(t, "#"))] = true
	}
	var out []vault.Entry
	for _, e := range entries {
		for _, t := range e.Tags {
			if want[strings.ToLower(t)] {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

// entriesInRange reads every entry inside the window.
func entriesInRange(v *vault.Vault, span dateRange) ([]vault.Entry, error) {
	var out []vault.Entry
	err := forEachEntryInRange(v, span, func(e vault.Entry) { out = append(out, e) })
	return out, err
}

// weaveJSON is the wire shape for structured weave output.
type weaveJSON struct {
	// Threads are the recurring patterns found in the record.
	Threads []threadJSON `json:"threads"`
	// Handoffs are the successions between threads.
	Handoffs []handoffJSON `json:"handoffs,omitempty"`
}

// threadJSON is the wire shape of one thread.
type threadJSON struct {
	// Label is the representative title.
	Label string `json:"label"`
	// Status is where the thread stands.
	Status string `json:"status"`
	// Count is how many occurrences it holds.
	Count int `json:"count"`
	// First and Last are the bounding dates.
	First string `json:"first"`
	Last  string `json:"last"`
	// MedianGapDays is the typical spacing between occurrences.
	MedianGapDays int `json:"median_gap_days"`
	// SilentDays is how long it has been quiet.
	SilentDays int `json:"silent_days"`
}

// handoffJSON is the wire shape of one succession.
type handoffJSON struct {
	// From and To are the labels of the ended and begun threads.
	From string `json:"from"`
	To   string `json:"to"`
	// GapDays is how long passed between them.
	GapDays int `json:"gap_days"`
}

// threadsToJSON converts threads to their wire shape.
func threadsToJSON(threads []weave.Thread) []threadJSON {
	out := make([]threadJSON, len(threads))
	for i, t := range threads {
		out[i] = threadJSON{
			Label: t.Label, Status: string(t.Status), Count: t.Count,
			First: t.First.Format(layoutDate), Last: t.Last.Format(layoutDate),
			MedianGapDays: t.MedianGap, SilentDays: t.SilentDays,
		}
	}
	return out
}

// handoffsToJSON converts successions to their wire shape.
func handoffsToJSON(handoffs []weave.Handoff) []handoffJSON {
	out := make([]handoffJSON, len(handoffs))
	for i, h := range handoffs {
		out[i] = handoffJSON{From: h.From.Label, To: h.To.Label, GapDays: h.GapDays}
	}
	return out
}
