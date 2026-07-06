package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/vault"
)

// weeklyOffset selects which trailing 7-day window to summarize (0 = ending today).
var weeklyOffset int

// weeklyCmd prints a one-paragraph-per-day summary of the trailing 7-day window.
var weeklyCmd = &cobra.Command{
	Use:   "weekly",
	Short: "Print a 7-day digest of entries grouped by day.",
	RunE:  runWeekly,
}

func init() {
	weeklyCmd.Flags().IntVar(&weeklyOffset, "offset", 0, "Weeks back from today (0 ends today, 1 ends a week ago).")
	rootCmd.AddCommand(weeklyCmd)
}

// runWeekly walks the trailing seven days and emits a per-day summary.
func runWeekly(cmd *cobra.Command, _ []string) error {
	v, err := openVault()
	if err != nil {
		return err
	}
	end := dayStart(time.Now()).AddDate(0, 0, -7*weeklyOffset)
	start := end.AddDate(0, 0, -6)
	entries, err := v.ReadRange(start, end)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("read range: %w", err))
	}
	if len(entries) == 0 {
		return errors.Join(ErrNotFound, fmt.Errorf("no entries between %s and %s",
			start.Format("2006-01-02"), end.Format("2006-01-02")))
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "# Week of %s to %s\n\n", start.Format("2006-01-02"), end.Format("2006-01-02"))
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dayEntries := entriesOnDay(entries, d)
		if len(dayEntries) == 0 {
			fmt.Fprintf(w, "## %s (%s)\nno entries\n\n", d.Format("Mon 2006-01-02"), shortWeekday(d))
			continue
		}
		fmt.Fprintf(w, "## %s (%s) — %d entr%s\n", d.Format("Mon 2006-01-02"), shortWeekday(d), len(dayEntries), plural(len(dayEntries)))
		for _, e := range dayEntries {
			fmt.Fprintf(w, "- %s", e.Time.Format("15:04"))
			if len(e.Tags) > 0 {
				fmt.Fprintf(w, " [%s]", strings.Join(e.Tags, ", "))
			}
			fmt.Fprintf(w, ": %s\n", firstLine(e.Body))
		}
		fmt.Fprintln(w)
	}
	return nil
}

// entriesOnDay returns the subset of entries whose timestamp falls on the given day.
func entriesOnDay(entries []vault.Entry, day time.Time) []vault.Entry {
	day = dayStart(day)
	var out []vault.Entry
	for _, e := range entries {
		es := dayStart(e.Time)
		if es.Equal(day) {
			out = append(out, e)
		}
	}
	return out
}

// firstLine returns the first non-empty line of s.
func firstLine(s string) string {
	for line := range strings.SplitSeq(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

// dayStart truncates the timestamp to local midnight of its day.
func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// shortWeekday returns the three-letter weekday name.
func shortWeekday(t time.Time) string {
	return t.Format("Mon")
}

// plural returns "y" for one and "ies" otherwise so callers can write "entr"+plural(n).
func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
