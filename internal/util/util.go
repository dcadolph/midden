// Package util holds small shared helpers used across midden packages.
package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExpandHome rewrites a leading "~" or "~/" to the user home directory.
// Named-user forms like "~alice" are rejected rather than silently misresolved.
// A path that does not start with a tilde is returned unchanged.
func ExpandHome(p string) (string, error) {
	if p == "" || p[0] != '~' {
		return p, nil
	}
	if len(p) > 1 && p[1] != '/' && p[1] != filepath.Separator {
		return "", fmt.Errorf("cannot expand user-specific home path %q", p)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, p[1:]), nil
}

// Absolute returns the cleaned absolute path for p with leading tilde expanded.
func Absolute(p string) (string, error) {
	expanded, err := ExpandHome(p)
	if err != nil {
		return "", err
	}
	return filepath.Abs(expanded)
}

// NormalizeTags trims whitespace and drops empty entries and leading hash characters.
// Tags retain their original case; comparisons use case-insensitive helpers.
func NormalizeTags(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, t := range in {
		t = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(t), "#"))
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return out
}

// TruncateRunes returns s shortened to at most max runes, appending an ellipsis when truncated.
// A max of zero or less returns the input unchanged.
func TruncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

// ContainsFold reports whether the lower-cased haystack contains the lower-cased needle.
func ContainsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
