// Package logutil provides a small context-based logger built on the standard library slog package.
package logutil

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// loggerKey is the context key under which a Logger is stored.
type loggerKey struct{}

// New returns a slog Logger that writes to the given writer at the given level.
// When w is nil, the logger writes to stderr.
func New(w io.Writer, level slog.Level) *slog.Logger {
	if w == nil {
		w = os.Stderr
	}
	h := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	return slog.New(h)
}

// Wrap stores logger in ctx so downstream callers can recover it with Unwrap.
func Wrap(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// Unwrap recovers the logger from ctx, falling back to a discard logger when none is stored.
func Unwrap(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
