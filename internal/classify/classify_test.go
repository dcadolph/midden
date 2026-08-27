package classify

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/dcadolph/midden/internal/weave"
	"github.com/dcadolph/midden/llm"
)

// day returns a local date.
func day(year int, month time.Month, d int) time.Time {
	return time.Date(year, month, d, 12, 0, 0, 0, time.Local)
}

// thread builds a minimal thread for candidate tests.
func thread(key, label string, count int, first, last time.Time, status weave.Status) weave.Thread {
	return weave.Thread{
		Key: key, Label: label, Count: count,
		First: first, Last: last,
		SpanDays: int(last.Sub(first).Hours() / 24), Status: status,
	}
}

func TestThreadPairsProposeBlockedMergesAndSuccessions(t *testing.T) {
	t.Parallel()
	threads := []weave.Thread{
		// The canonical case: the arithmetic refused to merge these because the
		// longer title adds a subject, and only a judged decision may join them.
		thread("arts martial", "Martial arts", 50, day(2023, 1, 18), day(2024, 1, 17), weave.Ended),
		thread("arts martial william", "William- Martial arts", 166, day(2024, 1, 24), day(2025, 8, 7), weave.Ended),
		// A same-subject succession.
		thread("baseball practice william", "William- baseball practice", 21, day(2026, 2, 2), day(2026, 8, 24), weave.Ongoing),
		// Unrelated: shares only the subject word with the others and is not
		// adjacent to either, so it must not be paired.
		thread("birthday william", "William's birthday", 5, day(2022, 12, 27), day(2021, 12, 27), weave.Ongoing),
	}
	got := ThreadPairs(threads, 10)
	keys := map[string]bool{}
	for _, c := range got {
		keys[c.MarkerKey()] = true
	}
	if !keys["thread|arts martial|arts martial william"] {
		t.Error("want the subject-blocked merge proposed for judgment")
	}
	if !keys["thread|arts martial william|baseball practice william"] {
		t.Error("want the succession proposed for judgment")
	}
	for k := range keys {
		if k == "thread|arts martial william|birthday william" {
			t.Error("want the name-only overlap left alone")
		}
	}
}

func TestPersonPairs(t *testing.T) {
	t.Parallel()
	people := []weave.Person{
		{Name: "William", Mentions: 391, First: day(2017, 6, 30), Last: day(2026, 8, 24)},
		{Name: "Will", Mentions: 18, First: day(2022, 9, 27), Last: day(2026, 8, 20)},
		{Name: "Kayla", Mentions: 35, First: day(2023, 5, 12), Last: day(2026, 8, 15)},
	}
	got := PersonPairs(people, 10)
	if len(got) != 1 {
		t.Fatalf("want only the resembling pair, got %d", len(got))
	}
	if diff := cmp.Diff("person|will|william", got[0].MarkerKey()); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestNamesResemble(t *testing.T) {
	t.Parallel()
	tests := []struct {
		A, B string
		Want bool
	}{
		{"will", "william", true},  // prefix
		{"ashlie", "ash", true},    // prefix
		{"jon", "john", true},      // one insertion
		{"kayla", "kayden", false}, // shared prefix but two edits
		{"sara", "sarah", true},    // one insertion
		{"emma", "mimi", false},    // unrelated
		{"al", "albert", false},    // too short to trust
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%s vs %s", test.A, test.B), func(t *testing.T) {
			t.Parallel()
			if got := namesResemble(test.A, test.B); got != test.Want {
				t.Errorf("want %v, got %v", test.Want, got)
			}
		})
	}
}

func TestParseVerdict(t *testing.T) {
	t.Parallel()
	yes, no := true, false
	tests := []struct {
		Name  string
		Reply string
		Want  *bool
	}{
		{"clean yes", `{"same": true}`, &yes},
		{"clean no", `{"same": false}`, &no},
		{"wrapped in prose", "Sure! Here is my answer: {\"same\": true} Hope that helps.", &yes},
		{"markdown fenced", "```json\n{\"same\": false}\n```", &no},
		{"missing field", `{"verdict": "yes"}`, nil},
		{"no json at all", "They are the same thing.", nil},
		{"unbalanced", `{"same": true`, nil},
		{"empty", "", nil},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			got := parseVerdict(test.Reply)
			switch {
			case test.Want == nil && got != nil:
				t.Errorf("want no verdict, got %v", *got)
			case test.Want != nil && got == nil:
				t.Errorf("want %v, got no verdict", *test.Want)
			case test.Want != nil && got != nil && *got != *test.Want:
				t.Errorf("want %v, got %v", *test.Want, *got)
			}
		})
	}
}

// scriptedChatter replies from a fixed list, then errors.
type scriptedChatter struct {
	// replies are returned in order.
	replies []string
	// calls counts invocations.
	calls int
}

// Name identifies the mock provider.
func (s *scriptedChatter) Name() string { return "mock:script" }

// Reply returns the next scripted reply.
func (s *scriptedChatter) Reply(_ context.Context, _ string, _ []llm.Message) (string, error) {
	if s.calls >= len(s.replies) {
		return "", errors.New("provider exploded")
	}
	r := s.replies[s.calls]
	s.calls++
	return r, nil
}

func TestJudgeSurvivesFailuresWithoutGuessing(t *testing.T) {
	t.Parallel()
	candidates := []Candidate{
		{Kind: KindPerson, A: "Will", B: "William", KeyA: "will", KeyB: "william"},
		{Kind: KindPerson, A: "Ash", B: "Ashlie", KeyA: "ash", KeyB: "ashlie"},
		{Kind: KindPerson, A: "Sara", B: "Sarah", KeyA: "sara", KeyB: "sarah"},
	}
	chat := &scriptedChatter{replies: []string{`{"same": true}`, "gibberish with no answer"}}
	got := Judge(context.Background(), chat, candidates, time.Minute)
	if len(got) != 3 {
		t.Fatalf("want every candidate judged, got %d", len(got))
	}
	if got[0].Same == nil || !*got[0].Same {
		t.Error("want the first verdict to be same")
	}
	// An unparseable reply and a provider failure are both no answer, never a
	// guess in either direction, and neither stops the rest.
	if got[1].Same != nil {
		t.Error("want gibberish to yield no verdict")
	}
	if got[2].Same != nil {
		t.Error("want a provider failure to yield no verdict")
	}
}
