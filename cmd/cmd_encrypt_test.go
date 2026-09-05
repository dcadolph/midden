package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/dcadolph/midden/index"
	"github.com/dcadolph/midden/internal/vault"
)

func TestSealablePaths(t *testing.T) {
	t.Parallel()
	tests := []struct {
		Name      string
		Days      []string
		WriteIdx  bool
		WantIndex bool
		WantDays  int
	}{{ // Test 0: An empty vault has nothing to seal.
		Name: "empty",
	}, { // Test 1: Day files are returned when no index exists.
		Name: "days only", Days: []string{"2026/09/01.md", "2026/09/02.md"}, WantDays: 2,
	}, { // Test 2: The recall index is sealed alongside the day files.
		Name: "days and index", Days: []string{"2026/09/01.md"}, WriteIdx: true,
		WantDays: 1, WantIndex: true,
	}, { // Test 3: An index with no day files is still sealed.
		Name: "index only", WriteIdx: true, WantIndex: true,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d %s", testNum, test.Name), func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			for _, rel := range test.Days {
				path := filepath.Join(dir, rel)
				if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.WriteFile(path, []byte("# day\n"), 0o600); err != nil {
					t.Fatalf("write day: %v", err)
				}
			}
			idxPath := filepath.Join(dir, index.Filename)
			if test.WriteIdx {
				if err := os.WriteFile(idxPath, []byte("{}"), 0o600); err != nil {
					t.Fatalf("write index: %v", err)
				}
			}
			v, err := vault.Open(dir)
			if err != nil {
				t.Fatalf("open vault: %v", err)
			}
			got, err := sealablePaths(v)
			if err != nil {
				t.Fatalf("sealablePaths: %v", err)
			}
			if hasIndex := slices.Contains(got, idxPath); hasIndex != test.WantIndex {
				t.Errorf("index included = %t, want %t (paths %v)", hasIndex, test.WantIndex, got)
			}
			days := len(got)
			if test.WantIndex {
				days--
			}
			if days != test.WantDays {
				t.Errorf("day file count = %d, want %d (paths %v)", days, test.WantDays, got)
			}
		})
	}
}

func TestSealablePathsCoversIndexAgainstLeak(t *testing.T) {
	t.Parallel()
	// The index stores entry bodies verbatim, so encryption must cover it or an
	// encrypted vault would publish the journal in the file beside it.
	dir := t.TempDir()
	secret := []byte(`{"entries":[{"body":"a private entry"}]}`)
	if err := os.WriteFile(filepath.Join(dir, index.Filename), secret, 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}
	v, err := vault.Open(dir)
	if err != nil {
		t.Fatalf("open vault: %v", err)
	}
	paths, err := sealablePaths(v)
	if err != nil {
		t.Fatalf("sealablePaths: %v", err)
	}
	sealed := v.WithPassphrase("test-passphrase")
	for _, p := range paths {
		data, err := os.ReadFile(p) //nolint:gosec // Paths come from the temp vault.
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		if err := sealed.WriteBytes(p, data); err != nil {
			t.Fatalf("seal %s: %v", p, err)
		}
	}
	onDisk, err := os.ReadFile(filepath.Join(dir, index.Filename)) //nolint:gosec // Temp vault path.
	if err != nil {
		t.Fatalf("read sealed index: %v", err)
	}
	if len(onDisk) == 0 {
		t.Fatal("sealed index is empty")
	}
	if string(onDisk) == string(secret) {
		t.Error("index was left in plaintext beside an encrypted vault")
	}
	back, err := sealed.ReadBytes(filepath.Join(dir, index.Filename))
	if err != nil {
		t.Fatalf("read back index: %v", err)
	}
	if string(back) != string(secret) {
		t.Errorf("index round trip = %q, want %q", back, secret)
	}
}
