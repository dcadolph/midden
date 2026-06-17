package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLoadMissingFileReturnsZero(t *testing.T) {
	t.Setenv(EnvConfig, filepath.Join(t.TempDir(), "missing.yaml"))
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if diff := cmp.Diff(Config{}, got); diff != "" {
		t.Errorf("expected zero config, got diff:\n%s", diff)
	}
}

func TestLoadParsesEveryField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := "default_tags:\n  - work\n  - life\neditor: nvim\nkeychain: true\nvault: /tmp/vault\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	t.Setenv(EnvConfig, path)
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := Config{
		DefaultTags: []string{"work", "life"},
		Editor:      "nvim",
		Keychain:    true,
		Vault:       "/tmp/vault",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}
