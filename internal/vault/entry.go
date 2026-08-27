package vault

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dcadolph/midden/flock"
)

// Entry is a single journal record within a day file.
type Entry struct {
	// Time is the local timestamp at which the entry was appended.
	Time time.Time
	// Tags are the entry tags without the leading hash character.
	Tags []string
	// Body is the free-form entry text. Trailing whitespace is trimmed on serialization.
	Body string
}

// Append writes the entry to the appropriate day file in the vault.
// The day file is created with a header if it does not yet exist.
// Entries are appended in arrival order under an advisory lock so concurrent
// writers do not interleave bytes; entries are never re-sorted by Append.
// Encrypted vaults are read-decrypted-appended-encrypted in one pass to keep
// the file sealed at rest.
func (v *Vault) Append(entry Entry) error {
	if strings.TrimSpace(entry.Body) == "" {
		return fmt.Errorf("entry body is empty")
	}
	lock, err := flock.Acquire(v.LockPath())
	if err != nil {
		return fmt.Errorf("acquire vault lock: %w", err)
	}
	defer func() { _ = lock.Close() }()
	path, err := v.EnsureDayFile(entry.Time)
	if err != nil {
		return err
	}
	if v.Passphrase == "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // Day path derives from the vault directory.
		if err != nil {
			return fmt.Errorf("open day file: %w", err)
		}
		defer func() { _ = f.Close() }()
		if _, err := f.WriteString(entry.Serialize()); err != nil {
			return fmt.Errorf("append entry: %w", err)
		}
		return nil
	}
	existing, err := v.readDayBytes(path)
	if err != nil {
		return err
	}
	existing = append(existing, []byte(entry.Serialize())...)
	if err := v.writeDayBytes(path, existing); err != nil {
		return err
	}
	return nil
}

// AppendAll writes every entry to its day file, grouping by day so each file is
// touched once rather than once per entry. This matters for bulk ingestion: on
// an encrypted vault Append decrypts and re-encrypts the whole day file per
// call, so appending a backfill entry at a time costs one full crypt cycle per
// event. Entries keep their given order within each day, every body is
// validated before anything is written, and the batch runs under a single
// advisory lock.
func (v *Vault) AppendAll(entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}
	for i, e := range entries {
		if strings.TrimSpace(e.Body) == "" {
			return fmt.Errorf("entry %d body is empty", i)
		}
	}
	byDay := map[string][]Entry{}
	var order []string
	for _, e := range entries {
		key := e.Time.Format("2006-01-02")
		if _, ok := byDay[key]; !ok {
			order = append(order, key)
		}
		byDay[key] = append(byDay[key], e)
	}
	sort.Strings(order)
	lock, err := flock.Acquire(v.LockPath())
	if err != nil {
		return fmt.Errorf("acquire vault lock: %w", err)
	}
	defer func() { _ = lock.Close() }()
	for _, key := range order {
		if err := v.appendDay(byDay[key]); err != nil {
			return fmt.Errorf("append %s: %w", key, err)
		}
	}
	return nil
}

// appendDay writes a run of same-day entries to their day file. The caller
// holds the vault lock.
func (v *Vault) appendDay(entries []Entry) error {
	path, err := v.EnsureDayFile(entries[0].Time)
	if err != nil {
		return err
	}
	var block strings.Builder
	for _, e := range entries {
		block.WriteString(e.Serialize())
	}
	if v.Passphrase == "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // Day path derives from the vault directory.
		if err != nil {
			return fmt.Errorf("open day file: %w", err)
		}
		defer func() { _ = f.Close() }()
		if _, err := f.WriteString(block.String()); err != nil {
			return fmt.Errorf("append entries: %w", err)
		}
		return nil
	}
	existing, err := v.readDayBytes(path)
	if err != nil {
		return err
	}
	return v.writeDayBytes(path, append(existing, []byte(block.String())...))
}

// ImportMarkers are the body lines that mark an entry as produced by an
// importer or by tool bookkeeping rather than written by the person. They live
// here so every command judging "did the person write this" shares one
// definition. Identity verdicts belong here even though a person approved
// them: approving a merge is operating the tool, not writing about a life, and
// the authored count must never move except by writing.
var ImportMarkers = []string{"ICS-UID: ", "GIT-COMMIT: ", SameMarker, DiffMarker}

// Identity markers record an accepted or rejected merge verdict. They are
// bookkeeping: configuration the person approved, not a record of their life.
const (
	// SameMarker records that two keys are one thing.
	SameMarker = "MIDDEN-SAME: "
	// DiffMarker records that two keys are not.
	DiffMarker = "MIDDEN-DIFF: "
)

// Bookkeeping reports whether the entry is tool bookkeeping rather than a
// record of anything that happened. Bookkeeping steers the analysis, so the
// analysis must never read it as data: an entry saying two names are the same
// person would otherwise count as a fresh mention of both.
func (e Entry) Bookkeeping() bool {
	for line := range strings.SplitSeq(e.Body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, SameMarker) || strings.HasPrefix(trimmed, DiffMarker) {
			return true
		}
	}
	return false
}

// Authored reports whether the entry was written by the person rather than
// imported. The distinction is load-bearing: a backfilled vault holds thousands
// of imported entries, and any feature that means to measure the person's own
// writing must not count them.
func (e Entry) Authored() bool {
	for line := range strings.SplitSeq(e.Body, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, m := range ImportMarkers {
			if strings.HasPrefix(trimmed, m) {
				return false
			}
		}
	}
	return true
}

// Serialize renders the entry as the markdown block written to a day file.
// The block begins with a level-two header carrying the time and tags
// and ends with a trailing blank line so successive entries stay separated.
// Body lines that would parse as day or entry headers are prefixed with one
// space so serialize-parse round trips never invent or drop entries.
func (e Entry) Serialize() string {
	var b strings.Builder
	b.WriteString("## ")
	b.WriteString(e.Time.Format("15:04:05"))
	for _, t := range e.Tags {
		b.WriteString(" #")
		b.WriteString(t)
	}
	b.WriteString("\n")
	for i, line := range strings.Split(strings.TrimRight(e.Body, " \t\n"), "\n") {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(escapeBodyLine(line))
	}
	b.WriteString("\n\n")
	return b.String()
}

// escapeBodyLine prefixes a space when the line would otherwise be misread as
// a day or entry header during parsing.
func escapeBodyLine(line string) string {
	if dayHeaderRe.MatchString(line) || entryHeaderRe.MatchString(line) {
		return " " + line
	}
	return line
}

// HasTag reports whether the entry carries the given tag, compared case-insensitively.
func (e Entry) HasTag(tag string) bool {
	tag = strings.ToLower(strings.TrimPrefix(tag, "#"))
	for _, t := range e.Tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}
