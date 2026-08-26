package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/go-cmp/cmp"

	"github.com/dcadolph/midden/internal/vault"
)

// commitSpec describes one commit to write into a test repository.
type commitSpec struct {
	// Message is the full commit message.
	Message string
	// Name and Email identify the author.
	Name  string
	Email string
	// When is the author timestamp.
	When time.Time
}

// testRepo builds a repository holding the given commits, in order, and returns
// its path.
func testRepo(t *testing.T, commits ...commitSpec) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	for i, c := range commits {
		name := fmt.Sprintf("file%d.txt", i)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(c.Message), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if _, err := wt.Add(name); err != nil {
			t.Fatalf("Add %s: %v", name, err)
		}
		_, err := wt.Commit(c.Message, &git.CommitOptions{
			Author: &object.Signature{Name: c.Name, Email: c.Email, When: c.When},
		})
		if err != nil {
			t.Fatalf("Commit %q: %v", c.Message, err)
		}
	}
	return dir
}

// gitDay returns a local timestamp on the given March 2024 day.
func gitDay(day, hour int) time.Time {
	return time.Date(2024, time.March, day, hour, 0, 0, 0, time.Local)
}

func TestReadCommitsOrdersOldestFirst(t *testing.T) {
	t.Parallel()
	dir := testRepo(t,
		commitSpec{Message: "first", Name: "Ada", Email: "ada@example.com", When: gitDay(4, 9)},
		commitSpec{Message: "second", Name: "Ada", Email: "ada@example.com", When: gitDay(6, 11)},
		commitSpec{Message: "third", Name: "Ada", Email: "ada@example.com", When: gitDay(8, 15)},
	)
	got, err := readCommits(dir, gitFilter{})
	if err != nil {
		t.Fatalf("readCommits: %v", err)
	}
	subjects := make([]string, len(got))
	for i, c := range got {
		subjects[i] = firstLine(c.Body)
	}
	if diff := cmp.Diff([]string{"first", "second", "third"}, subjects); diff != "" {
		t.Errorf("order mismatch (-want +got):\n%s", diff)
	}
	// Author time is what puts a commit on the day the work was done, which is
	// the point of ingesting history at all.
	if !got[0].When.Equal(gitDay(4, 9)) {
		t.Errorf("want the author timestamp %s, got %s", gitDay(4, 9), got[0].When)
	}
}

func TestReadCommitsHonorsDateRange(t *testing.T) {
	t.Parallel()
	dir := testRepo(t,
		commitSpec{Message: "before", Name: "Ada", Email: "ada@example.com", When: gitDay(1, 9)},
		commitSpec{Message: "inside", Name: "Ada", Email: "ada@example.com", When: gitDay(10, 9)},
		commitSpec{Message: "after", Name: "Ada", Email: "ada@example.com", When: gitDay(20, 9)},
	)
	span, err := resolveDateRange("2024-03-05", "2024-03-15")
	if err != nil {
		t.Fatalf("resolveDateRange: %v", err)
	}
	got, err := readCommits(dir, gitFilter{Span: span})
	if err != nil {
		t.Fatalf("readCommits: %v", err)
	}
	if len(got) != 1 || firstLine(got[0].Body) != "inside" {
		t.Fatalf("want only the in-range commit, got %d", len(got))
	}
}

func TestReadCommitsFiltersByAuthor(t *testing.T) {
	t.Parallel()
	dir := testRepo(t,
		commitSpec{Message: "mine", Name: "Ada Lovelace", Email: "ada@example.com", When: gitDay(4, 9)},
		commitSpec{Message: "theirs", Name: "Grace Hopper", Email: "grace@example.com", When: gitDay(5, 9)},
	)
	got, err := readCommits(dir, gitFilter{Authors: []string{"ada@example.com"}})
	if err != nil {
		t.Fatalf("readCommits: %v", err)
	}
	if len(got) != 1 || firstLine(got[0].Body) != "mine" {
		t.Fatalf("want only the matching author's commit, got %d", len(got))
	}
}

func TestReadCommitsRejectsAnUnknownRevision(t *testing.T) {
	t.Parallel()
	dir := testRepo(t, commitSpec{Message: "only", Name: "Ada", Email: "ada@example.com", When: gitDay(4, 9)})
	if _, err := readCommits(dir, gitFilter{Rev: "no-such-branch"}); err == nil {
		t.Error("want an error for an unresolvable revision")
	}
}

func TestReadCommitsRejectsANonRepository(t *testing.T) {
	t.Parallel()
	if _, err := readCommits(t.TempDir(), gitFilter{}); err == nil {
		t.Error("want an error for a directory that is not a repository")
	}
}

func TestGitFilterAccepts(t *testing.T) {
	t.Parallel()
	span, err := resolveDateRange("2024-03-05", "2024-03-15")
	if err != nil {
		t.Fatalf("resolveDateRange: %v", err)
	}
	merge := []plumbing.Hash{plumbing.NewHash("a"), plumbing.NewHash("b")}
	tests := []struct {
		Filter  gitFilter
		Parents []plumbing.Hash
		Name    string
		Email   string
		When    time.Time
		Want    bool
	}{{ // Test 0: A plain commit with no filter is accepted.
		When: gitDay(10, 9), Want: true,
	}, { // Test 1: Merge commits are noise in a personal history and are skipped by default.
		Parents: merge, When: gitDay(10, 9), Want: false,
	}, { // Test 2: Merge commits are kept when asked for.
		Filter: gitFilter{Merges: true}, Parents: merge, When: gitDay(10, 9), Want: true,
	}, { // Test 3: A commit before the range is rejected.
		Filter: gitFilter{Span: span}, When: gitDay(1, 9), Want: false,
	}, { // Test 4: A commit after the range is rejected.
		Filter: gitFilter{Span: span}, When: gitDay(20, 9), Want: false,
	}, { // Test 5: A commit inside the range is accepted.
		Filter: gitFilter{Span: span}, When: gitDay(10, 9), Want: true,
	}, { // Test 6: The author filter matches an email substring.
		Filter: gitFilter{Authors: []string{"ada@"}},
		Name:   "Ada Lovelace", Email: "ada@example.com", When: gitDay(10, 9), Want: true,
	}, { // Test 7: The author filter matches a name regardless of case.
		Filter: gitFilter{Authors: []string{"lovelace"}},
		Name:   "Ada Lovelace", Email: "ada@example.com", When: gitDay(10, 9), Want: true,
	}, { // Test 8: A commit by someone else is rejected.
		Filter: gitFilter{Authors: []string{"ada@"}},
		Name:   "Grace Hopper", Email: "grace@example.com", When: gitDay(10, 9), Want: false,
	}, { // Test 9: Any one of several authors matching is enough.
		Filter: gitFilter{Authors: []string{"ada@", "grace@"}},
		Name:   "Grace Hopper", Email: "grace@example.com", When: gitDay(10, 9), Want: true,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			c := &object.Commit{
				ParentHashes: test.Parents,
				Author:       object.Signature{Name: test.Name, Email: test.Email, When: test.When},
			}
			if diff := cmp.Diff(test.Want, test.Filter.accepts(c, test.When)); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFormatCommitRoundTripsThroughBodyMarker(t *testing.T) {
	t.Parallel()
	dir := testRepo(t, commitSpec{
		Message: "Add retry logic\n\nThe provider rate limits under load.",
		Name:    "Ada", Email: "ada@example.com", When: gitDay(4, 9),
	})
	got, err := readCommits(dir, gitFilter{})
	if err != nil {
		t.Fatalf("readCommits: %v", err)
	}
	body := got[0].Body
	if firstLine(body) != "Add retry logic" {
		t.Errorf("want the subject to lead the entry, got %q", firstLine(body))
	}
	for _, want := range []string{"Repo: " + filepath.Base(dir), "Author: Ada", "The provider rate limits under load."} {
		if !strings.Contains(body, want) {
			t.Errorf("entry missing %q:\n%s", want, body)
		}
	}
	// The hash a run writes and the hash a later run reads back have to agree, or
	// every re-ingest duplicates the whole history.
	if diff := cmp.Diff(got[0].SHA, bodyMarker(body, commitPrefix)); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestBodyMarkerIgnoresProse(t *testing.T) {
	t.Parallel()
	// A marker only counts when it starts a line, so a journal entry that talks
	// about the format is never mistaken for an ingested record.
	body := "Wrote about how GIT-COMMIT: lines work in the vault today."
	if got := bodyMarker(body, commitPrefix); got != "" {
		t.Errorf("want no marker from prose, got %q", got)
	}
}

func TestExistingCommitsFindsIngestedHashes(t *testing.T) {
	t.Parallel()
	v, _ := reindexVault(t)
	sha := "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b"
	err := v.AppendAll([]vault.Entry{
		{Time: gitDay(4, 9), Body: "Add retry logic\nRepo: midden\n" + commitPrefix + sha},
		{Time: gitDay(5, 9), Body: "A handwritten entry with no marker."},
	})
	if err != nil {
		t.Fatalf("AppendAll: %v", err)
	}
	seen, err := existingCommits(v, dateRange{})
	if err != nil {
		t.Fatalf("existingCommits: %v", err)
	}
	if !seen[sha] {
		t.Errorf("want the ingested commit recognized, got %v", seen)
	}
	if len(seen) != 1 {
		t.Errorf("want only marker-bearing entries counted, got %d", len(seen))
	}
}
