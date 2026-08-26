package vault

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// batchEntry builds an entry on the given day at the given hour.
func batchEntry(day, hour int, body string) Entry {
	return Entry{Time: time.Date(2024, time.March, day, hour, 0, 0, 0, time.Local), Body: body}
}

// readBodies returns the entry bodies stored on the given day.
func readBodies(t *testing.T, v *Vault, day time.Time) []string {
	t.Helper()
	entries, err := v.ReadDay(day)
	if err != nil {
		t.Fatalf("ReadDay: %v", err)
	}
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Body
	}
	return out
}

func TestAppendAllGroupsEntriesByDay(t *testing.T) {
	t.Parallel()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	entries := []Entry{
		batchEntry(4, 9, "monday first"),
		batchEntry(5, 8, "tuesday"),
		batchEntry(4, 17, "monday second"),
	}
	if err := v.AppendAll(entries); err != nil {
		t.Fatalf("AppendAll: %v", err)
	}
	monday := time.Date(2024, time.March, 4, 0, 0, 0, 0, time.Local)
	if diff := cmp.Diff([]string{"monday first", "monday second"}, readBodies(t, v, monday)); diff != "" {
		t.Errorf("monday mismatch (-want +got):\n%s", diff)
	}
	tuesday := time.Date(2024, time.March, 5, 0, 0, 0, 0, time.Local)
	if diff := cmp.Diff([]string{"tuesday"}, readBodies(t, v, tuesday)); diff != "" {
		t.Errorf("tuesday mismatch (-want +got):\n%s", diff)
	}
}

func TestAppendAllMatchesRepeatedAppend(t *testing.T) {
	t.Parallel()
	entries := []Entry{
		batchEntry(4, 9, "one"),
		batchEntry(4, 10, "two"),
		batchEntry(6, 11, "three"),
	}
	batched, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := batched.AppendAll(entries); err != nil {
		t.Fatalf("AppendAll: %v", err)
	}
	single, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for _, e := range entries {
		if err := single.Append(e); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	for _, day := range []int{4, 6} {
		d := time.Date(2024, time.March, day, 0, 0, 0, 0, time.Local)
		if diff := cmp.Diff(readBodies(t, single, d), readBodies(t, batched, d)); diff != "" {
			t.Errorf("day %d differs from repeated Append (-single +batched):\n%s", day, diff)
		}
	}
}

func TestAppendAllOnEncryptedVault(t *testing.T) {
	t.Parallel()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := v.SetEncrypted(); err != nil {
		t.Fatalf("SetEncrypted: %v", err)
	}
	v = v.WithPassphrase("correct horse battery staple")
	if err := v.AppendAll([]Entry{batchEntry(4, 9, "sealed one"), batchEntry(4, 10, "sealed two")}); err != nil {
		t.Fatalf("AppendAll: %v", err)
	}
	day := time.Date(2024, time.March, 4, 0, 0, 0, 0, time.Local)
	if diff := cmp.Diff([]string{"sealed one", "sealed two"}, readBodies(t, v, day)); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestAppendAllAppendsToAnExistingDay(t *testing.T) {
	t.Parallel()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := v.Append(batchEntry(4, 8, "already here")); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := v.AppendAll([]Entry{batchEntry(4, 9, "added")}); err != nil {
		t.Fatalf("AppendAll: %v", err)
	}
	day := time.Date(2024, time.March, 4, 0, 0, 0, 0, time.Local)
	if diff := cmp.Diff([]string{"already here", "added"}, readBodies(t, v, day)); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestAppendAllRejectsEmptyBodyBeforeWriting(t *testing.T) {
	t.Parallel()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	err = v.AppendAll([]Entry{batchEntry(4, 9, "good"), batchEntry(5, 9, "   ")})
	if err == nil {
		t.Fatal("want an error for an empty body")
	}
	// Validation runs before any write so a bad batch never lands half-applied.
	day := time.Date(2024, time.March, 4, 0, 0, 0, 0, time.Local)
	if got := readBodies(t, v, day); len(got) != 0 {
		t.Errorf("want nothing written, got %v", got)
	}
}

func TestAppendAllEmptyBatchIsANoOp(t *testing.T) {
	t.Parallel()
	v, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := v.AppendAll(nil); err != nil {
		t.Errorf("AppendAll(nil): %v", err)
	}
}
