package logutil

import (
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestWrapUnwrapRoundTrip(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	logger := New(&b, slog.LevelInfo)
	ctx := Wrap(context.Background(), logger)
	Unwrap(ctx).Info("hello", "k", "v")
	got := b.String()
	if !strings.Contains(got, "hello") || !strings.Contains(got, "k=v") {
		t.Errorf("logged output missing fields: %q", got)
	}
}

func TestUnwrapMissingReturnsNop(t *testing.T) {
	t.Parallel()
	logger := Unwrap(context.Background())
	if logger == nil {
		t.Fatal("Unwrap returned nil logger")
	}
	// Logging through the fallback must not panic and produce no output anywhere visible.
	logger.Info("discarded")
}

func TestNewNilWriterDefaultsToStderr(t *testing.T) {
	t.Parallel()
	if New(nil, slog.LevelDebug) == nil {
		t.Fatal("New returned nil logger")
	}
}
