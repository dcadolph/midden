package cmd

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/dcadolph/midden/internal/config"
)

// loadedConfig caches the parsed config so multiple subcommands share one read.
var loadedConfig struct {
	once sync.Once
	cfg  config.Config
	err  error
}

// userConfig returns the parsed midden configuration, loading from disk on first call.
// Errors short-circuit subsequent reads so callers see a consistent failure surface.
func userConfig() (config.Config, error) {
	loadedConfig.once.Do(func() {
		loadedConfig.cfg, loadedConfig.err = config.Load()
		if loadedConfig.err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", loadedConfig.err)
		}
	})
	return loadedConfig.cfg, loadedConfig.err
}

// resolveVaultDir returns the vault directory using flag, then config, then default.
func resolveVaultDir() string {
	if vaultDir != "" {
		return vaultDir
	}
	cfg, _ := userConfig()
	return cfg.Vault
}

// resolveDefaultTags returns the tag list to seed every new entry with from the config file.
func resolveDefaultTags() []string {
	cfg, _ := userConfig()
	return append([]string(nil), cfg.DefaultTags...)
}

// mergeTags returns the normalized union of defaults and flags, dropping
// case-insensitive duplicates while preserving first-seen order and case.
func mergeTags(defaults, flags []string) []string {
	merged := normalizeTags(append(append([]string(nil), defaults...), flags...))
	var out []string
	for _, t := range merged {
		if !slices.ContainsFunc(out, func(kept string) bool { return strings.EqualFold(kept, t) }) {
			out = append(out, t)
		}
	}
	return out
}

// entryTags merges the config default tags with the per-command tags for a new entry.
func entryTags(flagTags []string) []string {
	return mergeTags(resolveDefaultTags(), flagTags)
}

// resolveEditorOverride returns the config-supplied editor command or an empty string.
func resolveEditorOverride() string {
	cfg, _ := userConfig()
	return cfg.Editor
}

// keychainEnabled reports whether the configuration opted into keychain passphrase storage.
func keychainEnabled() bool {
	cfg, _ := userConfig()
	return cfg.Keychain
}
