package mobile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
)

// newBareRemote creates an empty bare repository and returns its path.
func newBareRemote(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "remote.git")
	if _, err := git.PlainInit(dir, true); err != nil {
		t.Fatalf("init bare remote: %v", err)
	}
	return dir
}

// syncOrFail runs a sync and fails the test on error, returning the summary.
func syncOrFail(t *testing.T, v *Vault, remote string) string {
	t.Helper()
	summary, err := v.Sync(remote, "", "main")
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	return summary
}

func TestSyncCarriesCapturesBetweenDevices(t *testing.T) {
	t.Parallel()
	remote := newBareRemote(t)

	phoneDir := t.TempDir()
	phone, err := OpenDevice(phoneDir, "", "iphone")
	if err != nil {
		t.Fatalf("open phone: %v", err)
	}
	if err := phone.AppendAt("2026-09-05T09:00:00", "voice", "Captured on the phone."); err != nil {
		t.Fatalf("phone append: %v", err)
	}
	syncOrFail(t, phone, remote)

	// A second device starting from nothing must receive the first one's work.
	deskDir := t.TempDir()
	desk, err := Open(deskDir, "")
	if err != nil {
		t.Fatalf("open desktop: %v", err)
	}
	syncOrFail(t, desk, remote)
	landed := filepath.Join(deskDir, "inbox", "iphone", "2026", "09", "05.md")
	data, err := os.ReadFile(landed) //nolint:gosec // Path from the temp vault.
	if err != nil {
		t.Fatalf("phone capture did not reach the desktop: %v", err)
	}
	if !strings.Contains(string(data), "Captured on the phone.") {
		t.Errorf("synced file = %q, want the phone capture", data)
	}
}

func TestSyncReconcilesDivergedHistories(t *testing.T) {
	t.Parallel()
	remote := newBareRemote(t)

	deskDir := t.TempDir()
	desk, err := Open(deskDir, "")
	if err != nil {
		t.Fatalf("open desktop: %v", err)
	}
	if err := desk.AppendAt("2026-09-05T08:00:00", "", "Desktop entry."); err != nil {
		t.Fatalf("desk append: %v", err)
	}
	syncOrFail(t, desk, remote)

	phoneDir := t.TempDir()
	phone, err := OpenDevice(phoneDir, "", "iphone")
	if err != nil {
		t.Fatalf("open phone: %v", err)
	}
	syncOrFail(t, phone, remote)

	// Both sides now write before either syncs again, so history diverges.
	if err := desk.AppendAt("2026-09-05T10:00:00", "", "Second desktop entry."); err != nil {
		t.Fatalf("desk append: %v", err)
	}
	syncOrFail(t, desk, remote)
	if err := phone.AppendAt("2026-09-05T11:00:00", "voice", "Phone entry while offline."); err != nil {
		t.Fatalf("phone append: %v", err)
	}
	summary := syncOrFail(t, phone, remote)
	if !strings.Contains(summary, "replayed") {
		t.Errorf("summary = %q, want a replay onto the remote", summary)
	}

	// The phone must keep its own capture and gain the desktop's.
	day, err := phone.DayJSON("2026-09-05")
	if err != nil {
		t.Fatalf("phone DayJSON: %v", err)
	}
	for _, want := range []string{"Desktop entry.", "Second desktop entry.", "Phone entry while offline."} {
		if !strings.Contains(day, want) {
			t.Errorf("phone day missing %q; got %s", want, day)
		}
	}

	// The desktop picks all of it up on its next sync.
	syncOrFail(t, desk, remote)
	deskDay, err := desk.DayJSON("2026-09-05")
	if err != nil {
		t.Fatalf("desk DayJSON: %v", err)
	}
	if !strings.Contains(deskDay, "Second desktop entry.") {
		t.Errorf("desktop lost its own entry: %s", deskDay)
	}
	phoneFile := filepath.Join(deskDir, "inbox", "iphone", "2026", "09", "05.md")
	data, err := os.ReadFile(phoneFile) //nolint:gosec // Path from the temp vault.
	if err != nil {
		t.Fatalf("phone capture did not reach the desktop: %v", err)
	}
	if !strings.Contains(string(data), "Phone entry while offline.") {
		t.Errorf("desktop inbox = %q, want the phone capture", data)
	}
}

func TestSyncDoesNotResurrectFoldedCaptures(t *testing.T) {
	t.Parallel()
	remote := newBareRemote(t)

	phoneDir := t.TempDir()
	phone, err := OpenDevice(phoneDir, "", "iphone")
	if err != nil {
		t.Fatalf("open phone: %v", err)
	}
	if err := phone.AppendAt("2026-09-05T09:00:00", "", "Captured then folded."); err != nil {
		t.Fatalf("phone append: %v", err)
	}
	syncOrFail(t, phone, remote)

	// The desktop folds the capture away, which deletes the inbox file.
	deskDir := t.TempDir()
	desk, err := Open(deskDir, "")
	if err != nil {
		t.Fatalf("open desktop: %v", err)
	}
	syncOrFail(t, desk, remote)
	inboxFile := filepath.Join(deskDir, "inbox", "iphone", "2026", "09", "05.md")
	if err := os.Remove(inboxFile); err != nil {
		t.Fatalf("simulate fold: %v", err)
	}
	if err := desk.AppendAt("2026-09-05T09:00:00", "", "Captured then folded."); err != nil {
		t.Fatalf("desk append folded entry: %v", err)
	}
	syncOrFail(t, desk, remote)

	// The phone still has the file locally and now diverges. The replay must
	// not restore what the desktop deliberately removed.
	if err := phone.AppendAt("2026-09-06T09:00:00", "", "A later capture."); err != nil {
		t.Fatalf("phone append: %v", err)
	}
	syncOrFail(t, phone, remote)

	if _, err := os.Stat(filepath.Join(phoneDir, "inbox", "iphone", "2026", "09", "05.md")); !os.IsNotExist(err) {
		t.Error("replay resurrected an inbox file the desktop had folded away")
	}
	day, err := phone.DayJSON("2026-09-05")
	if err != nil {
		t.Fatalf("DayJSON: %v", err)
	}
	if got := strings.Count(day, "Captured then folded."); got != 1 {
		t.Errorf("entry appears %d times after sync, want exactly 1: %s", got, day)
	}
}
