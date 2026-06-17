// Package report renders a single-file HTML summary of the vault.
package report

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"strings"
	"time"

	"github.com/dcadolph/midden/internal/vault"
)

//go:embed templates/*.html
var templates embed.FS

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
	// CalCols is the column count used by the heatmap grid (always 7).
	CalCols int
	// Tags holds the top tag bars.
	Tags []TagBar
	// Days holds one row per day with entries on it.
	Days []DayRow
}

// Render writes the HTML report to w using the supplied template data.
func Render(w io.Writer, data Data) error {
	t, err := template.ParseFS(templates, "templates/report.html")
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	if err := t.Execute(w, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}
	return nil
}

// Build prepares Data for Render by walking the vault.
// The heatmap covers the full year ending on the supplied reference date.
func Build(title string, v *vault.Vault, now time.Time, topTags int) (Data, error) {
	stats, err := v.ComputeStats(topTags)
	if err != nil {
		return Data{}, err
	}
	streak, err := v.Streak(now)
	if err != nil {
		return Data{}, err
	}
	days, err := v.ListDays()
	if err != nil {
		return Data{}, err
	}
	counts := map[string]dayMetric{}
	for _, d := range days {
		entries, err := v.ReadDay(d)
		if err != nil {
			return Data{}, err
		}
		key := d.Format("2006-01-02")
		m := counts[key]
		for _, e := range entries {
			m.Entries++
			m.Words += len(strings.Fields(e.Body))
		}
		counts[key] = m
	}
	cal := buildCalendar(counts, now)
	rows := buildDayRows(days, counts)
	return Data{
		Title:    title,
		Stats:    stats,
		Streak:   streak,
		Calendar: cal,
		CalCols:  7,
		Tags:     buildTagBars(stats.TopTags),
		Days:     rows,
	}, nil
}

// dayMetric pairs entry count and word count for a single day.
type dayMetric struct {
	Entries int
	Words   int
}

// buildCalendar emits a year-back heatmap ending at the start of the week containing now.
// Cells are ordered week-first then day-of-week so the CSS grid renders columns of seven.
func buildCalendar(counts map[string]dayMetric, now time.Time) []CalendarCell {
	end := startOfWeek(now)
	start := end.AddDate(-1, 0, 0)
	var cells []CalendarCell
	for d := start; !d.After(end.AddDate(0, 0, 6)); d = d.AddDate(0, 0, 1) {
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

// buildDayRows orders the day rows newest first.
func buildDayRows(days []time.Time, counts map[string]dayMetric) []DayRow {
	rows := make([]DayRow, 0, len(days))
	for i := len(days) - 1; i >= 0; i-- {
		d := days[i]
		key := d.Format("2006-01-02")
		m := counts[key]
		if m.Entries == 0 {
			continue
		}
		rows = append(rows, DayRow{Date: key, Entries: m.Entries, Words: m.Words})
	}
	return rows
}

// buildTagBars scales tag counts to percent widths against the top tag.
func buildTagBars(tags []vault.TagCount) []TagBar {
	if len(tags) == 0 {
		return nil
	}
	top := tags[0].Count
	out := make([]TagBar, len(tags))
	for i, t := range tags {
		w := 0
		if top > 0 {
			w = int(float64(t.Count) / float64(top) * 100)
			if w < 4 {
				w = 4
			}
		}
		out[i] = TagBar{Tag: t.Tag, Count: t.Count, Width: w}
	}
	return out
}

// startOfWeek returns the Sunday at or before t.
func startOfWeek(t time.Time) time.Time {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return day.AddDate(0, 0, -int(day.Weekday()))
}
