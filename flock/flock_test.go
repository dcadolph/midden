package flock

import (
	"path/filepath"
	"testing"
	"time"
)

// TestAcquireContention takes the lock, contends from a second goroutine,
// releases, and confirms the contender then acquires.
func TestAcquireContention(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "vault.lock")
	first, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	type result struct {
		lock *Lock
		err  error
	}
	done := make(chan result, 1)
	go func() {
		l, err := Acquire(path)
		done <- result{lock: l, err: err}
	}()
	select {
	case <-done:
		t.Fatal("second Acquire returned while the lock was held")
	case <-time.After(100 * time.Millisecond):
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("second Acquire: %v", r.err)
		}
		if err := r.lock.Close(); err != nil {
			t.Errorf("Close second lock: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("second Acquire did not return after the lock was released")
	}
}

// TestCloseTwiceIsNoOp releases the lock twice and expects both calls to succeed.
func TestCloseTwiceIsNoOp(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "vault.lock")
	l, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}
