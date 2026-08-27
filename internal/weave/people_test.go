package weave

import (
	"testing"
	"time"

	"github.com/dcadolph/midden/internal/vault"
)

// named builds an entry with the given headline on a date offset in weeks.
func named(week int, head string) vault.Entry {
	return vault.Entry{
		Time: time.Date(2024, time.January, 6, 12, 0, 0, 0, time.Local).AddDate(0, 0, 7*week),
		Body: head,
	}
}

func TestPeopleRequireGrammarEvidence(t *testing.T) {
	t.Parallel()
	var entries []vault.Entry
	// A real person: possessives and companion prepositions across mentions.
	entries = append(entries,
		named(0, "Hannah- sleepover with Kayla"),
		named(2, "Kayla's birthday party"),
		named(4, "Hannah- sleepover w/ Kayla"),
	)
	// Commit-style verbs: capitalized every time, only ever sentence-initial,
	// never possessive, never a companion.
	for i := range 20 {
		entries = append(entries, named(i, "Add retry logic to the fetcher"))
		entries = append(entries, named(i, "Fix the flaky test"))
	}
	// Title-case noise that recurs heavily.
	for i := range 15 {
		entries = append(entries, named(i, "H-E-B Curbside"))
	}
	got := People(entries, DefaultPeopleOptions(observed))
	if len(got) != 1 {
		t.Fatalf("want only the real person, got %d: %+v", len(got), got)
	}
	if got[0].Name != "Kayla" {
		t.Errorf("want Kayla, got %q", got[0].Name)
	}
	if got[0].Mentions != 3 {
		t.Errorf("want 3 mentions, got %d", got[0].Mentions)
	}
}

func TestPeopleMergePossessiveAndBareSpellings(t *testing.T) {
	t.Parallel()
	entries := []vault.Entry{
		named(0, "Jax's grooming!!"),
		named(10, "Jax's grooming!!"),
		named(20, "Drop Jax at the vet"),
	}
	got := People(entries, DefaultPeopleOptions(observed))
	if len(got) != 1 || got[0].Mentions != 3 {
		t.Fatalf("want one person with 3 mentions, got %+v", got)
	}
	if got[0].Name != "Jax" {
		t.Errorf("want the possessive stripped from the display name, got %q", got[0].Name)
	}
}

func TestPeopleHonorMinMentionsAndExclude(t *testing.T) {
	t.Parallel()
	entries := []vault.Entry{
		named(0, "Lunch with Sam"),
		named(1, "Lunch with Sam"),
		named(2, "Coffee with Austin"),
		named(3, "Trip with Austin"),
		named(4, "Dinner with Austin"),
	}
	opts := DefaultPeopleOptions(observed)
	opts.MinMentions = 3
	opts.Exclude = []string{"austin"} // the city, not a person, in this record
	got := People(entries, opts)
	if len(got) != 0 {
		t.Errorf("want Sam below the floor and Austin excluded, got %+v", got)
	}
}

func TestPeopleCountEachEntryOnce(t *testing.T) {
	t.Parallel()
	// The same name twice in one headline is one mention, not two.
	entries := []vault.Entry{
		named(0, "Kayla and Kayla's cake with Kayla"),
		named(1, "Sleepover with Kayla"),
		named(2, "Kayla's party"),
	}
	got := People(entries, DefaultPeopleOptions(observed))
	if len(got) != 1 || got[0].Mentions != 3 {
		t.Fatalf("want 3 mentions across 3 entries, got %+v", got)
	}
}

func TestPeopleIgnoreFutureEntries(t *testing.T) {
	t.Parallel()
	entries := []vault.Entry{
		named(0, "Dinner with Sara"),
		named(1, "Sara's recital"),
		named(2, "Sara's recital"),
		{Time: observed.AddDate(1, 0, 0), Body: "Sara's wedding"},
	}
	got := People(entries, DefaultPeopleOptions(observed))
	if len(got) != 1 {
		t.Fatalf("want one person, got %+v", got)
	}
	if got[0].Last.After(observed) {
		t.Errorf("want the last mention capped at the observation date, got %s", got[0].Last)
	}
	if got[0].Mentions != 3 {
		t.Errorf("want the scheduled mention uncounted, got %d", got[0].Mentions)
	}
}
