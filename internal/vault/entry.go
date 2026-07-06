package vault

import (
	"fmt"
	"os"
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
