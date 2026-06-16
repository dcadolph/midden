package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestOpenAndDayPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	v, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if v.Dir != dir {
		t.Errorf("vault dir mismatch: want %s, got %s", dir, v.Dir)
	}

	day := time.Date(2026, 6, 16, 9, 14, 23, 0, time.Local)
	wantPath := filepath.Join(dir, "2026", "06", "16.md")
	if got := v.DayPath(day); got != wantPath {
		t.Errorf("day path mismatch: want %s, got %s", wantPath, got)
	}
}

func TestEnsureDayFileWritesHeader(t *testing.T) {
	t.Parallel()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	day := time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local)
	path, err := v.EnsureDayFile(day)
	if err != nil {
		t.Fatalf("EnsureDayFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if diff := cmp.Diff("# 2026-06-16\n\n", string(data)); diff != "" {
		t.Errorf("header mismatch (-want +got):\n%s", diff)
	}
}

func TestEnsureDayFileIdempotent(t *testing.T) {
	t.Parallel()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	day := time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local)
	path, err := v.EnsureDayFile(day)
	if err != nil {
		t.Fatalf("first EnsureDayFile: %v", err)
	}
	if err := os.WriteFile(path, []byte("# 2026-06-16\n\nexisting body\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if _, err := v.EnsureDayFile(day); err != nil {
		t.Fatalf("second EnsureDayFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "existing body") {
		t.Errorf("EnsureDayFile clobbered existing content: %q", data)
	}
}

func TestAppendCreatesDayFileAndAppends(t *testing.T) {
	t.Parallel()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	stamp := time.Date(2026, 6, 16, 9, 14, 23, 0, time.Local)
	if err := v.Append(Entry{Time: stamp, Tags: []string{"first"}, Body: "Body one."}); err != nil {
		t.Fatalf("first Append: %v", err)
	}
	if err := v.Append(Entry{Time: stamp.Add(time.Hour), Body: "Body two."}); err != nil {
		t.Fatalf("second Append: %v", err)
	}
	data, err := os.ReadFile(v.DayPath(stamp))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "# 2026-06-16\n\n## 09:14:23 #first\nBody one.\n\n## 10:14:23\nBody two.\n\n"
	if diff := cmp.Diff(want, string(data)); diff != "" {
		t.Errorf("file mismatch (-want +got):\n%s", diff)
	}
}

func TestAppendRejectsEmptyBody(t *testing.T) {
	t.Parallel()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	err = v.Append(Entry{Time: time.Now(), Body: "   \n\n"})
	if err == nil {
		t.Fatal("Append with empty body returned no error")
	}
}

func TestResolveDirEnvOverride(t *testing.T) {
	tests := []struct {
		WantResult string
		Override   string
		Env        string
	}{{ // Test 0: Override beats env beats default.
		Override:   "/tmp/override",
		Env:        "/tmp/env",
		WantResult: "/tmp/override",
	}, { // Test 1: Env wins when override is empty.
		Override:   "",
		Env:        "/tmp/env-only",
		WantResult: "/tmp/env-only",
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Setenv(EnvHome, test.Env)
			got, err := resolveDir(test.Override)
			if err != nil {
				t.Fatalf("resolveDir: %v", err)
			}
			if diff := cmp.Diff(test.WantResult, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
