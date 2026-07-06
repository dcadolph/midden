// Package config loads optional midden configuration from disk.
//
// Configuration is YAML and lives at $MIDDEN_CONFIG when set,
// otherwise at $XDG_CONFIG_HOME/midden/config.yaml or
// ~/.config/midden/config.yaml. The absence of a config file is
// not an error; defaults apply silently.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/dcadolph/midden/internal/util"
)

// EnvConfig is the environment variable that overrides the configuration file path.
const EnvConfig = "MIDDEN_CONFIG"

// Config holds every persisted user preference.
type Config struct {
	// DefaultTags are attached to every new entry in addition to per-command tags.
	DefaultTags []string `yaml:"default_tags,omitempty"`
	// Editor is the editor command used by `midden today` and editor-mode `midden add`.
	// When empty, midden falls back to $MIDDEN_EDITOR, $VISUAL, $EDITOR, then vi.
	Editor string `yaml:"editor,omitempty"`
	// Keychain enables OS keychain storage of the vault passphrase.
	Keychain bool `yaml:"keychain,omitempty"`
	// Vault overrides the vault directory.
	Vault string `yaml:"vault,omitempty"`
}

// Load returns the parsed Config from disk.
// A missing file resolves to a zero Config and a nil error.
func Load() (Config, error) {
	path, err := ResolvePath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path) //nolint:gosec // Config path resolved from the environment.
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	return c, nil
}

// ResolvePath returns the absolute path that Load will read.
// It does not check whether the file exists.
func ResolvePath() (string, error) {
	if env := os.Getenv(EnvConfig); env != "" {
		path, err := util.Absolute(env)
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", EnvConfig, err)
		}
		return path, nil
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("user home: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "midden", "config.yaml"), nil
}
