// Package vault handles all read and write operations against the markdown journal directory.
package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// EnvHome is the environment variable that overrides the default vault directory.
const EnvHome = "MIDDEN_HOME"

// DefaultDir is the path used when EnvHome is unset and no override is given.
// It is resolved relative to the calling user's home directory at call time.
const DefaultDir = "midden"

// Vault is a handle to a journal directory on disk.
type Vault struct {
	// Dir is the absolute path to the root of the journal directory.
	Dir string
}

// Open returns a Vault rooted at the given directory.
// An empty override defers to the environment variable and then to DefaultDir under the user home.
// The directory and any required parents are created if missing.
func Open(override string) (*Vault, error) {
	dir, err := resolveDir(override)
	if err != nil {
		return nil, fmt.Errorf("resolve vault directory: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create vault directory: %w", err)
	}
	return &Vault{Dir: dir}, nil
}

// DayPath returns the absolute path to the markdown file for the given local date.
// The file is not guaranteed to exist.
func (v *Vault) DayPath(day time.Time) string {
	return filepath.Join(
		v.Dir,
		fmt.Sprintf("%04d", day.Year()),
		fmt.Sprintf("%02d", int(day.Month())),
		fmt.Sprintf("%02d.md", day.Day()),
	)
}

// EnsureDayFile creates the day file for the given local date if it does not exist.
// The file is initialized with a single-line date header.
// It returns the absolute path to the file.
func (v *Vault) EnsureDayFile(day time.Time) (string, error) {
	path := v.DayPath(day)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat day file: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create day directory: %w", err)
	}
	header := fmt.Sprintf("# %s\n\n", day.Format("2006-01-02"))
	if err := os.WriteFile(path, []byte(header), 0o644); err != nil {
		return "", fmt.Errorf("write day header: %w", err)
	}
	return path, nil
}

// resolveDir picks the vault directory using override then env then default.
func resolveDir(override string) (string, error) {
	if override != "" {
		return absolute(override)
	}
	if env := os.Getenv(EnvHome); env != "" {
		return absolute(env)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home: %w", err)
	}
	return filepath.Join(home, DefaultDir), nil
}

// absolute expands the path and converts it to an absolute path.
func absolute(p string) (string, error) {
	if len(p) > 0 && p[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("user home: %w", err)
		}
		p = filepath.Join(home, p[1:])
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("absolute path: %w", err)
	}
	return abs, nil
}
