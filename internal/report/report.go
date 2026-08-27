// Package report renders a single-file HTML summary of the vault.
package report

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"math"
	"strings"
	"time"

	"github.com/dcadolph/midden/internal/util"
	"github.com/dcadolph/midden/internal/vault"
	"github.com/dcadolph/midden/internal/weave"
)

//go:embed templates/*.html
var templatesFS embed.FS

// reportTemplate is the report page template, parsed once at package load.
var reportTemplate = template.Must(template.ParseFS(templatesFS, "templates/report.html"))

// CalendarCell is one square in the heatmap.
type CalendarCell struct {
	// Date is the YYYY-MM-DD label shown in the cell tooltip.
	Date string
	// Count is the number of entries on this date.
	Count int
	// Class is the CSS class applied to color the cell.
	Class string
}

// TagBar is a single row in the tag histogram with the bar already sized.
type TagBar struct {
	// Tag is the tag label.
	Tag string
	// Count is the entry count.
	Count int
	// Width is the percentage width of the bar from zero to one hundred.
	Width int
}

// DayRow is a single row in the days table.
type DayRow struct {
	// Date is the YYYY-MM-DD date.
	Date string
	// Entries is the entry count on the date.
	Entries int
	// Words is the total whitespace-separated word count on the date.
	Words int
}

// EraRow is one chapter of the record with its bar pre-sized.
type EraRow struct {
	// Span is the era's month range.
	Span string
	// Months is the era's length.
	Months int
	// Entries is the entry count inside it.
	Entries int
	// PerMonth is the average monthly volume, pre-formatted.
	PerMonth string
	// Width is the bar width percentage, scaled by log volume.
	Width int
}

// ThreadRow is one recurring commitment.
type ThreadRow struct {
	// Label is the thread's display title.
	Label string
	// Count is how many occurrences it holds.
	Count int
	// Span describes how long it ran.
	Span string
	// Detail carries the status-specific tail: when it ended, its cadence, or
	// how long it has been quiet.
	Detail string
}

// SilenceRow is one stretch where the record went quiet.
type SilenceRow struct {
	// Span is the silence's month range.
	Span string
	// Months is its length.
	Months int
	// Scope names the source that fell silent, or the whole record.
	Scope string
	// Detail carries the before and after rates.
	Detail string
}

// PersonRow is one person the record names.
type PersonRow struct {
	// Name is the most common spelling.
	Name string
	// Mentions is how many entries name them.
	Mentions int
	// Span describes how long they have been in the record.
	Span string
	// LastSeen is set only for names that recurred and then faded.
	LastSeen string
}

// HandoffRow is one succession between threads.
type HandoffRow struct {
	// From and To are the thread labels.
	From string
	To   string
	// Detail carries the dates and the gap.
	Detail string
}

// Data is the template view model for the report.
type Data struct {
	// Title appears in the document title.
	Title string
	// Stats is the precomputed vault summary.
	Stats vault.Stats
	// Streak is the consecutive-days streak ending today.
	Streak int
	// Calendar holds one CalendarCell per day in the heatmap window.
	Calendar []CalendarCell
	// Tags holds the top tag bars.
	Tags []TagBar
	// Days holds one row per day with entries on it.
	Days []DayRow
	// Eras holds the record's chapters.
	Eras []EraRow
	// Ended, Dormant, and Ongoing hold the weave threads by status.
	Ended   []ThreadRow
	Dormant []ThreadRow
	Ongoing []ThreadRow
	// Silences holds the stretches where the record went quiet.
	Silences []SilenceRow
	// People holds the names the record keeps mentioning.
	People []PersonRow
	// Handoffs holds successions between threads.
	Handoffs []HandoffRow
}

// Render writes the HTML report to w using the supplied template data.
func Render(w io.Writer, data Data) error {
	if err := reportTemplate.Execute(w, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}
	return nil
}

// Build prepares Data for Render by walking the vault.
// The heatmap covers the full year ending on the supplied reference date.
func Build(title string, v *vault.Vault, now time.Time, topTags int) (Data, error) {
	stats, err := v.ComputeStats(topTags, now)
	if err != nil {
		return Data{}, fmt.Errorf("compute stats: %w", err)
	}
	streak, err := v.Streak(now, vault.Entry.Authored)
	if err != nil {
		return Data{}, fmt.Errorf("compute streak: %w", err)
	}
	days, err := v.ListDays()
	if err != nil {
		return Data{}, fmt.Errorf("list days: %w", err)
	}
	counts := map[string]dayMetric{}
	var all []vault.Entry
	for _, d := range days {
		entries, err := v.ReadDay(d)
		if err != nil {
			return Data{}, fmt.Errorf("read day %s: %w", d.Format("2006-01-02"), err)
		}
		key := d.Format("2006-01-02")
		m := counts[key]
		for _, e := range entries {
			m.Entries++
			m.Words += len(strings.Fields(e.Body))
		}
		counts[key] = m
		all = append(all, entries...)
	}
	threads := weave.Threads(all, weave.DefaultOptions(now))
	ended, dormant, ongoing := splitThreads(threads, now)
	return Data{
		Title:    title,
		Stats:    stats,
		Streak:   streak,
		Calendar: buildCalendar(counts, now),
		Tags:     buildTagBars(stats.TopTags),
		Days:     buildDayRows(days, counts),
		Eras:     buildEraRows(weave.Eras(all, weave.DefaultEraOptions(now))),
		Ended:    ended,
		Dormant:  dormant,
		Ongoing:  ongoing,
		Silences: buildSilenceRows(weave.GapsBySource(all, []string{"calendar", "git"}, weave.DefaultGapOptions(now))),
		People:   buildPersonRows(weave.People(all, weave.DefaultPeopleOptions(now))),
		Handoffs: buildHandoffRows(weave.Handoffs(threads, weave.DefaultHandoffOptions())),
	}, nil
}

// rowCap bounds every weave-derived list so the report stays a summary.
const rowCap = 15

// monthLabel renders a month for display.
const monthLabel = "2006-01"

// buildEraRows converts eras, sizing each bar by log volume so the quiet
// chapters stay visible next to the loud ones.
func buildEraRows(eras []weave.Era) []EraRow {
	if len(eras) < 2 {
		return nil
	}
	maxLog := 0.0
	for _, e := range eras {
		if l := logVolume(e.PerMonth); l > maxLog {
			maxLog = l
		}
	}
	out := make([]EraRow, len(eras))
	for i, e := range eras {
		width := 4
		if maxLog > 0 {
			width = int(logVolume(e.PerMonth) / maxLog * 100)
			if width < 4 {
				width = 4
			}
		}
		out[i] = EraRow{
			Span:     e.From.Format(monthLabel) + " to " + e.To.Format(monthLabel),
			Months:   e.Months,
			Entries:  e.Entries,
			PerMonth: fmt.Sprintf("%.0f", e.PerMonth),
			Width:    width,
		}
	}
	return out
}

// logVolume compresses a monthly rate for bar sizing.
func logVolume(perMonth float64) float64 {
	if perMonth < 0 {
		return 0
	}
	return math.Log1p(perMonth)
}

// splitThreads converts threads into display rows by status.
func splitThreads(threads []weave.Thread, now time.Time) (ended, dormant, ongoing []ThreadRow) {
	for _, t := range threads {
		row := ThreadRow{
			Label: t.Label,
			Count: t.Count,
			Span:  spanLabel(t.SpanDays),
		}
		switch t.Status {
		case weave.Ended:
			if len(ended) >= rowCap {
				continue
			}
			row.Detail = "Last on " + t.Last.Format("2006-01-02") + ", " + silenceLabel(t.SilentDays) + " ago."
			ended = append(ended, row)
		case weave.Dormant:
			if len(dormant) >= rowCap {
				continue
			}
			row.Detail = "Quiet " + silenceLabel(t.SilentDays) + ", but it has returned from gaps this long before."
			dormant = append(dormant, row)
		case weave.Ongoing:
			if len(ongoing) >= rowCap {
				continue
			}
			row.Detail = fmt.Sprintf("Roughly every %d days, still going.", t.MedianGap)
			ongoing = append(ongoing, row)
		case weave.Emerging:
			if len(ongoing) >= rowCap {
				continue
			}
			row.Detail = "New since " + t.First.Format("2006-01-02") + "."
			ongoing = append(ongoing, row)
		}
	}
	return ended, dormant, ongoing
}

// buildSilenceRows converts gaps for display.
func buildSilenceRows(gaps []weave.Gap) []SilenceRow {
	out := make([]SilenceRow, 0, len(gaps))
	for _, g := range gaps {
		if len(out) >= rowCap {
			break
		}
		scope := "Whole record"
		if g.Source != "" {
			scope = strings.ToUpper(g.Source[:1]) + g.Source[1:] + " only"
		}
		out = append(out, SilenceRow{
			Span:   g.From.Format(monthLabel) + " to " + g.To.Format(monthLabel),
			Months: g.Months,
			Scope:  scope,
			Detail: fmt.Sprintf("About %.0f entries a month before, %.0f after.", g.Before, g.After),
		})
	}
	return out
}

// buildPersonRows converts people for display, flagging faded names.
func buildPersonRows(people []weave.Person) []PersonRow {
	out := make([]PersonRow, 0, len(people))
	for _, p := range people {
		if len(out) >= rowCap {
			break
		}
		row := PersonRow{
			Name:     p.Name,
			Mentions: p.Mentions,
			Span:     spanLabel(int(p.Last.Sub(p.First).Hours() / 24)),
		}
		if p.SilentDays > 365 && p.Last.Sub(p.First) > 180*24*time.Hour {
			row.LastSeen = p.Last.Format("2006-01-02")
		}
		out = append(out, row)
	}
	return out
}

// buildHandoffRows converts successions for display.
func buildHandoffRows(handoffs []weave.Handoff) []HandoffRow {
	out := make([]HandoffRow, 0, len(handoffs))
	seen := map[string]bool{}
	for _, h := range handoffs {
		if len(out) >= rowCap {
			break
		}
		// One succession per ended thread: the earliest successor tells the story.
		if seen[h.From.Key] {
			continue
		}
		seen[h.From.Key] = true
		out = append(out, HandoffRow{
			From: h.From.Label,
			To:   h.To.Label,
			Detail: fmt.Sprintf("Ended %s after %dx; began %d days later.",
				h.From.Last.Format("2006-01-02"), h.From.Count, h.GapDays),
		})
	}
	return out
}

// spanLabel renders a day count as years or months for display.
func spanLabel(days int) string {
	switch {
	case days >= 365:
		return fmt.Sprintf("%.1f years", float64(days)/365)
	case days >= 60:
		return fmt.Sprintf("%d months", days/30)
	}
	return fmt.Sprintf("%d days", days)
}

// silenceLabel renders a silence length for display.
func silenceLabel(days int) string {
	if days >= 60 {
		return fmt.Sprintf("%d months", days/30)
	}
	return fmt.Sprintf("%d days", days)
}

// dayMetric pairs entry count and word count for a single day.
type dayMetric struct {
	// Entries is the entry count on the day.
	Entries int
	// Words is the whitespace-separated word count on the day.
	Words int
}

// buildCalendar emits a GitHub-style heatmap covering the year ending at now.
// The window starts on the Sunday on or before one year ago and ends on the
// Saturday of the week containing now, so the cell count is a multiple of
// seven and cells flow column-major: one week per column, one weekday per row.
func buildCalendar(counts map[string]dayMetric, now time.Time) []CalendarCell {
	start := startOfWeek(now.AddDate(-1, 0, 0))
	end := startOfWeek(now).AddDate(0, 0, 6)
	var cells []CalendarCell
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		m := counts[key]
		cells = append(cells, CalendarCell{
			Date:  key,
			Count: m.Entries,
			Class: cellClass(m.Entries),
		})
	}
	return cells
}

// cellClass maps an entry count to a CSS class shading.
func cellClass(count int) string {
	switch {
	case count == 0:
		return ""
	case count == 1:
		return "l1"
	case count <= 3:
		return "l2"
	default:
		return "l3"
	}
}

// buildDayRows orders the day rows newest first and skips days with no entries.
func buildDayRows(days []time.Time, counts map[string]dayMetric) []DayRow {
	rows := make([]DayRow, 0, len(days))
	for i := len(days) - 1; i >= 0; i-- {
		key := days[i].Format("2006-01-02")
		m := counts[key]
		if m.Entries == 0 {
			continue
		}
		rows = append(rows, DayRow{Date: key, Entries: m.Entries, Words: m.Words})
	}
	return rows
}

// buildTagBars scales tag counts to percent widths against the top tag.
// The top tag gets width 100 and every other tag gets at least width 4.
func buildTagBars(tags []util.TagCount) []TagBar {
	if len(tags) == 0 {
		return nil
	}
	top := tags[0].Count
	out := make([]TagBar, len(tags))
	for i, t := range tags {
		w := 0
		if top > 0 {
			w = max(int(float64(t.Count)/float64(top)*100), 4)
		}
		out[i] = TagBar{Tag: t.Tag, Count: t.Count, Width: w}
	}
	return out
}

// startOfWeek returns midnight on the Sunday at or before t.
func startOfWeek(t time.Time) time.Time {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return day.AddDate(0, 0, -int(day.Weekday()))
}
