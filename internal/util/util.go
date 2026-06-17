// Package util holds small shared helpers used across midden packages.
package util

import (
	"os"
	"path/filepath"
	"strings"
)

// ExpandHome rewrites a leading tilde to the user home directory.
// A path that does not start with a tilde is returned unchanged.
func ExpandHome(p string) (string, error) {
	if p == "" || p[0] != '~' {
		return p, nil
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
