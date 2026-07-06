// Package flock wraps advisory POSIX file locks for safe concurrent appends.
package flock

import (
	"fmt"
	"os"
	"syscall"
)

// Lock holds an advisory exclusive lock on a file descriptor.
// It is acquired with Acquire and released by Close.
type Lock struct {
	f *os.File
}

// Acquire opens the lock file at path and takes an exclusive advisory lock.
// The lock file is created with mode 0o600 if it does not exist.
// Acquire blocks until the lock is granted or the underlying syscall fails.
func Acquire(path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // Lock path derives from the vault directory.
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return &Lock{f: f}, nil
}

// Close releases the lock and closes the underlying file descriptor.
// Subsequent calls are no-ops.
func (l *Lock) Close() error {
	if l == nil || l.f == nil {
		return nil
	}
	defer func() { l.f = nil }()
	if err := syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN); err != nil {
		_ = l.f.Close()
		return fmt.Errorf("unlock: %w", err)
	}
	if err := l.f.Close(); err != nil {
		return fmt.Errorf("close lock file: %w", err)
	}
	return nil
}
