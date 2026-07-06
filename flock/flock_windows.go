//go:build windows

package flock

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// Lock holds an exclusive lock on a file handle.
// It is acquired with Acquire and released by Close.
type Lock struct {
	f *os.File
}

// Acquire opens the lock file at path and takes an exclusive lock.
// The lock file is created with mode 0o600 if it does not exist.
// A non-blocking attempt runs first; when another process holds the lock a
// single notice is printed to stderr before falling back to a blocking wait,
// so users know why the command has paused.
func Acquire(path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // Lock path derives from the vault directory.
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	err = lockHandle(f, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY)
	if err == nil {
		return &Lock{f: f}, nil
	}
	if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		_ = f.Close()
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	fmt.Fprintln(os.Stderr, "waiting for vault lock (another midden command is running)")
	if err := lockHandle(f, windows.LOCKFILE_EXCLUSIVE_LOCK); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return &Lock{f: f}, nil
}

// Close releases the lock and closes the underlying file handle.
// Subsequent calls are no-ops.
func (l *Lock) Close() error {
	if l == nil || l.f == nil {
		return nil
	}
	defer func() { l.f = nil }()
	if err := windows.UnlockFileEx(windows.Handle(l.f.Fd()), 0, 1, 0, new(windows.Overlapped)); err != nil {
		_ = l.f.Close()
		return fmt.Errorf("unlock: %w", err)
	}
	if err := l.f.Close(); err != nil {
		return fmt.Errorf("close lock file: %w", err)
	}
	return nil
}

// lockHandle locks the first byte of the file with the given LockFileEx flags.
func lockHandle(f *os.File, flags uint32) error {
	return windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, 1, 0, new(windows.Overlapped))
}
