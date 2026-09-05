package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestValidDevice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In      string
		WantErr bool
	}{{ // Test 0: A plain name is valid.
		In: "iphone", WantErr: false,
	}, { // Test 1: Hyphens and digits are fine.
		In: "iphone-14-pro", WantErr: false,
	}, { // Test 2: An empty name is rejected.
		In: "", WantErr: true,
	}, { // Test 3: A forward slash would escape the inbox tree.
		In: "a/b", WantErr: true,
	}, { // Test 4: A backslash is rejected too.
		In: `a\b`, WantErr: true,
	}, { // Test 5: Parent traversal is rejected.
		In: "..", WantErr: true,
	}, { // Test 6: The current directory is rejected.
		In: ".", WantErr: true,
	}, { // Test 7: A dotfile name is rejected so it cannot shadow vault markers.
		In: ".midden.encrypted", WantErr: true,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			err := ValidDevice(test.In)
			if (err != nil) != test.WantErr {
				t.Errorf("ValidDevice(%q) error = %v, want error %t", test.In, err, test.WantErr)
			}
		})
	}
}

func TestInboxIsInvisibleToCanonicalReads(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	v := &Vault{Dir: dir}
	day := time.Date(2026, 9, 5, 9, 0, 0, 0, time.Local)
	if err := v.Append(Entry{Time: day, Body: "Written on the desktop."}); err != nil {
		t.Fatalf("append canonical: %v", err)
	}
	box, err := v.Inbox("iphone")
	if err != nil {
		t.Fatalf("inbox: %v", err)
	}
	if err := box.Append(Entry{Time: day.Add(time.Hour), Body: "Captured on the phone."}); err != nil {
		t.Fatalf("append inbox: %v", err)
	}

	// The canonical vault must not see inbox entries until they are folded.
	canonical, err := v.ReadDay(day)
	if err != nil {
		t.Fatalf("read canonical: %v", err)
	}
	if len(canonical) != 1 || canonical[0].Body != "Written on the desktop." {
		t.Errorf("canonical day = %+v, want only the desktop entry", canonical)
	}
	days, err := v.ListDays()
	if err != nil {
		t.Fatalf("list days: %v", err)
	}
	if len(days) != 1 {
		t.Errorf("canonical ListDays returned %d days, want 1", len(days))
	}

	// The two devices wrote different files, which is what keeps them mergeable.
	if v.DayPath(day) == box.DayPath(day) {
		t.Fatal("inbox and canonical resolved to the same day file")
	}
	pending, err := box.ReadDay(day)
	if err != nil {
		t.Fatalf("read inbox: %v", err)
	}
	if len(pending) != 1 || pending[0].Body != "Captured on the phone." {
		t.Errorf("inbox day = %+v, want only the phone entry", pending)
	}
}

func TestInboxDevices(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	v := &Vault{Dir: dir}
	if got, err := v.InboxDevices(); err != nil || len(got) != 0 {
		t.Fatalf("InboxDevices on empty vault = %v, %v; want empty", got, err)
	}
	day := time.Date(2026, 9, 5, 9, 0, 0, 0, time.Local)
	for _, device := range []string{"watch", "iphone"} {
		box, err := v.Inbox(device)
		if err != nil {
			t.Fatalf("inbox %s: %v", device, err)
		}
		if err := box.Append(Entry{Time: day, Body: "x"}); err != nil {
			t.Fatalf("append %s: %v", device, err)
		}
	}
	// A stray file in the inbox root must not be mistaken for a device.
	if err := os.WriteFile(filepath.Join(dir, InboxDir, "README"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write stray file: %v", err)
	}
	got, err := v.InboxDevices()
	if err != nil {
		t.Fatalf("InboxDevices: %v", err)
	}
	if diff := cmp.Diff([]string{"iphone", "watch"}, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestInboxEncryptionFollowsTheVault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	v := (&Vault{Dir: dir}).WithPassphrase("test-passphrase")
	box, err := v.Inbox("iphone")
	if err != nil {
		t.Fatalf("inbox: %v", err)
	}
	day := time.Date(2026, 9, 5, 9, 0, 0, 0, time.Local)
	secret := "a private capture"
	if err := box.Append(Entry{Time: day, Body: secret}); err != nil {
		t.Fatalf("append: %v", err)
	}
	raw, err := os.ReadFile(box.DayPath(day)) //nolint:gosec // Path from the temp vault.
	if err != nil {
		t.Fatalf("read raw: %v", err)
	}
	if !isEncryptedBytes(raw) {
		t.Error("inbox day file is not encrypted although the vault carries a passphrase")
	}
	entries, err := box.ReadDay(day)
	if err != nil {
		t.Fatalf("read inbox: %v", err)
	}
	if len(entries) != 1 || entries[0].Body != secret {
		t.Errorf("inbox round trip = %+v, want the original entry", entries)
	}
}

func TestDrainDayPrunesEmptyDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	v := &Vault{Dir: dir}
	box, err := v.Inbox("iphone")
	if err != nil {
		t.Fatalf("inbox: %v", err)
	}
	sameMonth := time.Date(2026, 9, 5, 9, 0, 0, 0, time.Local)
	otherDay := time.Date(2026, 9, 6, 9, 0, 0, 0, time.Local)
	for _, d := range []time.Time{sameMonth, otherDay} {
		if err := box.Append(Entry{Time: d, Body: "x"}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	if err := box.DrainDay(sameMonth); err != nil {
		t.Fatalf("drain: %v", err)
	}
	if _, err := os.Stat(box.DayPath(sameMonth)); !os.IsNotExist(err) {
		t.Error("drained day file still exists")
	}
	// The month still holds another day, so it must survive.
	if _, err := os.Stat(filepath.Dir(box.DayPath(sameMonth))); err != nil {
		t.Errorf("month directory was pruned while still holding a day: %v", err)
	}
	if err := box.DrainDay(otherDay); err != nil {
		t.Fatalf("drain second: %v", err)
	}
	if _, err := os.Stat(filepath.Join(box.Dir, "2026")); !os.IsNotExist(err) {
		t.Error("empty year directory was not pruned")
	}
	if _, err := os.Stat(box.Dir); err != nil {
		t.Errorf("inbox root should survive draining: %v", err)
	}
}
