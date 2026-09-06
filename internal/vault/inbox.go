package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// InboxDir is the vault subdirectory holding per-device inbox trees.
// It sits outside the YYYY/MM layout that ListDays walks, so inbox entries are
// invisible to canonical reads until they are folded in.
const InboxDir = "inbox"

// Inbox returns a vault rooted at this vault's inbox for the named device.
//
// An inbox holds the same YYYY/MM/DD.md layout as the vault itself and inherits
// the passphrase, so entries are encrypted at rest exactly as canonical entries
// are. Devices that do not own the canonical files write here instead, so two
// devices appending on the same day never write the same path and their day
// files never conflict when merged.
func (v *Vault) Inbox(device string) (*Vault, error) {
	if err := ValidDevice(device); err != nil {
		return nil, err
	}
	dir := filepath.Join(v.Dir, InboxDir, device)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create inbox directory: %w", err)
	}
	return &Vault{Dir: dir, Passphrase: v.Passphrase}, nil
}

// InboxDevices returns the sorted names of devices that have an inbox tree.
// A vault with no inbox returns an empty slice and no error.
func (v *Vault) InboxDevices() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(v.Dir, InboxDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read inbox directory: %w", err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() || ValidDevice(e.Name()) != nil {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out, nil
}

// ValidDevice reports whether name is usable as an inbox device directory.
// The name must be a single path segment so it cannot escape the inbox tree.
func ValidDevice(name string) error {
	if name == "" {
		return fmt.Errorf("device name is empty")
	}
	if name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("device name %q must be a single path segment", name)
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("device name %q must not start with a dot", name)
	}
	return nil
}

// RemoveIfEmpty deletes this inbox directory when it holds no day files,
// tolerating the lock file the vault itself creates. An inbox that still holds
// entries is left alone and is not an error.
func (v *Vault) RemoveIfEmpty() error {
	days, err := v.ListDays()
	if err != nil {
		return err
	}
	if len(days) > 0 {
		return nil
	}
	entries, err := os.ReadDir(v.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read inbox directory: %w", err)
	}
	for _, e := range entries {
		if e.Name() != lockFile {
			return nil
		}
	}
	if err := os.RemoveAll(v.Dir); err != nil {
		return fmt.Errorf("remove inbox directory: %w", err)
	}
	return nil
}

// DrainDay removes the day file for the given date and prunes the month and
// year directories once they are empty, leaving the vault root in place.
// It is called on an inbox after that day's entries reach the canonical vault.
func (v *Vault) DrainDay(day time.Time) error {
	path := v.DayPath(day)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	// Prune the month then the year; a non-empty directory stops the walk.
	for dir := filepath.Dir(path); dir != v.Dir; dir = filepath.Dir(dir) {
		if err := os.Remove(dir); err != nil {
			return nil
		}
	}
	return nil
}
