package vault

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// dayHeaderRe matches the level-one date header at the top of a day file.
var dayHeaderRe = regexp.MustCompile(`^#\s+(\d{4})-(\d{2})-(\d{2})\s*$`)

// entryHeaderRe matches an entry header line carrying a time and zero or more tags.
var entryHeaderRe = regexp.MustCompile(`^##\s+(\d{2}):(\d{2}):(\d{2})(\s+.*)?$`)

// tagRe matches a single hash tag in an entry header.
var tagRe = regexp.MustCompile(`#([A-Za-z0-9_\-]+)`)

// ReadDay parses every entry in the day file for the given local date.
// It returns an empty slice and a nil error when the file does not exist.
func (v *Vault) ReadDay(day time.Time) ([]Entry, error) {
	path := v.DayPath(day)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open day file: %w", err)
	}
	defer f.Close()

	dayDate := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	var entries []Entry
	var current *Entry
	var body strings.Builder

	finalize := func() {
		if current == nil {
			return
		}
		current.Body = strings.TrimRight(body.String(), "\n")
		entries = append(entries, *current)
		current = nil
		body.Reset()
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if dayHeaderRe.MatchString(line) {
			continue
		}
		if m := entryHeaderRe.FindStringSubmatch(line); m != nil {
			finalize()
			t, err := parseEntryTime(dayDate, m[1], m[2], m[3])
			if err != nil {
				return nil, fmt.Errorf("parse entry time on %s: %w", path, err)
			}
			tags := extractTags(m[4])
			current = &Entry{Time: t, Tags: tags}
			continue
		}
		if current == nil {
			continue
		}
		body.WriteString(line)
		body.WriteString("\n")
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read day file: %w", err)
	}
	finalize()
	return entries, nil
}

// ListDays returns the sorted list of local dates that have day files in the vault.
// The slice is ordered ascending.
func (v *Vault) ListDays() ([]time.Time, error) {
	root := v.Dir
	var days []time.Time
	years, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read vault directory: %w", err)
	}
	for _, y := range years {
		if !y.IsDir() || !isFourDigit(y.Name()) {
			continue
		}
		months, err := os.ReadDir(joinPath(root, y.Name()))
		if err != nil {
			return nil, fmt.Errorf("read year directory: %w", err)
		}
		for _, m := range months {
			if !m.IsDir() || !isTwoDigit(m.Name()) {
				continue
			}
			files, err := os.ReadDir(joinPath(root, y.Name(), m.Name()))
			if err != nil {
				return nil, fmt.Errorf("read month directory: %w", err)
			}
			for _, f := range files {
				if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
					continue
				}
				base := strings.TrimSuffix(f.Name(), ".md")
				if !isTwoDigit(base) {
					continue
				}
				t, err := time.ParseInLocation("2006-01-02", y.Name()+"-"+m.Name()+"-"+base, time.Local)
				if err != nil {
					continue
				}
				days = append(days, t)
			}
		}
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })
	return days, nil
}

// parseEntryTime composes a local timestamp from the day date and hour, minute, second strings.
func parseEntryTime(day time.Time, h, m, s string) (time.Time, error) {
	t, err := time.ParseInLocation(
		"2006-01-02 15:04:05",
		fmt.Sprintf("%04d-%02d-%02d %s:%s:%s", day.Year(), int(day.Month()), day.Day(), h, m, s),
		day.Location(),
	)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

// extractTags pulls hashtag tokens from the trailing portion of an entry header line.
func extractTags(rest string) []string {
	if rest == "" {
		return nil
	}
	matches := tagRe.FindAllStringSubmatch(rest, -1)
	if len(matches) == 0 {
		return nil
	}
	tags := make([]string, 0, len(matches))
	for _, m := range matches {
		tags = append(tags, m[1])
	}
	return tags
}

// isFourDigit reports whether s is exactly four decimal digits.
func isFourDigit(s string) bool {
	if len(s) != 4 {
		return false
	}
	return isDigits(s)
}

// isTwoDigit reports whether s is exactly two decimal digits.
func isTwoDigit(s string) bool {
	if len(s) != 2 {
		return false
	}
	return isDigits(s)
}

// isDigits reports whether s contains only decimal digits.
func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// joinPath wraps filepath.Join to keep the parser readable.
func joinPath(parts ...string) string {
	return filepath.Join(parts...)
}
