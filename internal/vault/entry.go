package vault

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dcadolph/midden/internal/flock"
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
	defer lock.Close()
	path, err := v.EnsureDayFile(entry.Time)
	if err != nil {
		return err
	}
	if v.Passphrase == "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("open day file: %w", err)
		}
		defer f.Close()
		if _, err := f.WriteString(entry.Serialize()); err != nil {
			return fmt.Errorf("append entry: %w", err)
		}
		return nil
	}
	existing, err := v.readDayBytes(path)
	if err != nil {
		return err
	}
	updated := append(existing, []byte(entry.Serialize())...)
	if err := v.writeDayBytes(path, updated); err != nil {
		return err
	}
	return nil
}

// Serialize renders the entry as the markdown block written to a day file.
// The block begins with a level-two header carrying the time and tags
// and ends with a trailing blank line so successive entries stay separated.
func (e Entry) Serialize() string {
	var b strings.Builder
	b.WriteString("## ")
	b.WriteString(e.Time.Format("15:04:05"))
	for _, t := range e.Tags {
		b.WriteString(" #")
		b.WriteString(t)
	}
	b.WriteString("\n")
	b.WriteString(strings.TrimRight(e.Body, " \t\n"))
	b.WriteString("\n\n")
	return b.String()
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
