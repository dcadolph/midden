package vault

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestEncryptedAppendRoundTrips(t *testing.T) {
	t.Parallel()
	plain, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	enc := plain.WithPassphrase("correct horse battery staple")

	entries := []Entry{
		{Time: time.Date(2026, 6, 16, 9, 14, 23, 0, time.Local), Tags: []string{"project"}, Body: "first"},
		{Time: time.Date(2026, 6, 16, 14, 32, 1, 0, time.Local), Body: "second"},
	}
	for _, e := range entries {
		if err := enc.Append(e); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	raw, err := os.ReadFile(enc.DayPath(entries[0].Time))
	if err != nil {
		t.Fatalf("read raw file: %v", err)
	}
	if !isEncryptedBytes(raw) {
		t.Fatalf("day file is not encrypted at rest: %q", raw[:64])
	}

	got, err := enc.ReadDay(entries[0].Time)
	if err != nil {
		t.Fatalf("ReadDay: %v", err)
	}
	if diff := cmp.Diff(entries, got, cmp.Comparer(equalTimes)); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestReadEncryptedWithoutPassphraseFails(t *testing.T) {
	t.Parallel()
	plain, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	enc := plain.WithPassphrase("strong-pass")
	if err := enc.Append(Entry{Time: time.Now(), Body: "secret"}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	if _, err := plain.ReadDay(time.Now()); err == nil || !strings.Contains(err.Error(), "encrypted") {
		t.Errorf("ReadDay without passphrase should fail with encrypted error, got %v", err)
	}
}

func TestEncryptedAppendPreservesPriorEntries(t *testing.T) {
	t.Parallel()
	plain, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	enc := plain.WithPassphrase("pass1")
	day := time.Date(2026, 6, 16, 0, 0, 0, 0, time.Local)
	for i := 0; i < 5; i++ {
		if err := enc.Append(Entry{Time: day.Add(time.Duration(i) * time.Hour), Body: "entry"}); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	got, err := enc.ReadDay(day)
	if err != nil {
		t.Fatalf("ReadDay: %v", err)
	}
	if len(got) != 5 {
		t.Errorf("want 5 entries preserved, got %d", len(got))
	}
}

func TestSetAndClearEncryptedMarker(t *testing.T) {
	t.Parallel()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if v.IsEncrypted() {
		t.Fatal("fresh vault reports encrypted")
	}
	if err := v.SetEncrypted(); err != nil {
		t.Fatalf("SetEncrypted: %v", err)
	}
	if !v.IsEncrypted() {
		t.Fatal("vault does not report encrypted after SetEncrypted")
	}
	if err := v.ClearEncrypted(); err != nil {
		t.Fatalf("ClearEncrypted: %v", err)
	}
	if v.IsEncrypted() {
		t.Fatal("vault still reports encrypted after ClearEncrypted")
	}
}
