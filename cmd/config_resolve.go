package cmd

import (
	"fmt"
	"os"
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
