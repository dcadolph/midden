package interview

import (
	"strings"
	"testing"
	"time"

	"github.com/dcadolph/midden/internal/weave"
)

// observed is a fixed observation date so generation never depends on the clock.
var observed = time.Date(2026, time.August, 26, 12, 0, 0, 0, time.Local)

// day returns a local date.
func day(year int, month time.Month, d int) time.Time {
	return time.Date(year, month, d, 12, 0, 0, 0, time.Local)
}

// endedThread builds a heavy thread that has gone quiet.
func endedThread(key, label string) weave.Thread {
	return weave.Thread{
		Key: key, Label: label, Count: 216,
		First: day(2023, time.January, 10), Last: day(2025, time.August, 7),
		MedianGap: 4, SilentDays: 384, SpanDays: 940, Status: weave.Ended,
	}
}

func TestGenerateAsksAboutEndings(t *testing.T) {
	t.Parallel()
	got := Generate([]weave.Thread{endedThread("arts martial william", "William- Martial arts")},
		nil, nil, nil, DefaultOptions(observed))
	if len(got) != 1 {
		t.Fatalf("want one question, got %d", len(got))
	}
	q := got[0]
	if q.Kind != KindEnded {
		t.Errorf("want an ending question, got %s", q.Kind)
	}
	// The evidence has to travel with the question, or the person is being asked
	// to remember the very thing they cannot.
	for _, want := range []string{"216", "2025-08-07"} {
		if !strings.Contains(q.Context, want) {
			t.Errorf("want %q in the evidence, got %q", want, q.Context)
		}
	}
}

func TestGenerateSkipsOngoingThreads(t *testing.T) {
	t.Parallel()
	// A grocery run recurring for years is not a gap, and asking about it
	// teaches the person to ignore the prompt.
	chore := weave.Thread{
		Key: "curbside heb", Label: "H-E-B curbside", Count: 239,
		First: day(2022, time.October, 1), Last: observed,
		MedianGap: 4, SpanDays: 1400, Status: weave.Ongoing,
	}
	if got := Generate([]weave.Thread{chore}, nil, nil, nil, DefaultOptions(observed)); len(got) != 0 {
		t.Errorf("want nothing asked about a live thread, got %q", got[0].Prompt)
	}
}

func TestGenerateSkipsLightThreads(t *testing.T) {
	t.Parallel()
	light := weave.Thread{
		Key: "dentist", Label: "Dentist", Count: 3,
		First: day(2024, time.March, 1), Last: day(2024, time.May, 1),
		MedianGap: 30, SilentDays: 800, SpanDays: 61, Status: weave.Ended,
	}
	if got := Generate([]weave.Thread{light}, nil, nil, nil, DefaultOptions(observed)); len(got) != 0 {
		t.Errorf("want incidental patterns ignored, got %q", got[0].Prompt)
	}
}

func TestGenerateNeverRepeatsAnAnsweredQuestion(t *testing.T) {
	t.Parallel()
	th := endedThread("arts martial william", "William- Martial arts")
	first := Generate([]weave.Thread{th}, nil, nil, nil, DefaultOptions(observed))
	if len(first) != 1 {
		t.Fatalf("want one question, got %d", len(first))
	}
	answered := map[string]bool{first[0].ID: true}
	// Being asked again about something already answered is the fastest way to
	// make the prompt worthless.
	if again := Generate([]weave.Thread{th}, nil, nil, answered, DefaultOptions(observed)); len(again) != 0 {
		t.Errorf("want an answered question retired, got %q", again[0].Prompt)
	}
}

func TestQuestionIDIsStableAcrossRuns(t *testing.T) {
	t.Parallel()
	th := endedThread("arts martial william", "William- Martial arts")
	a := Generate([]weave.Thread{th}, nil, nil, nil, DefaultOptions(observed))[0].ID
	b := Generate([]weave.Thread{th}, nil, nil, nil, DefaultOptions(observed))[0].ID
	if a != b {
		t.Errorf("want a stable id so answers keep matching, got %q then %q", a, b)
	}
	other := endedThread("hip hannah hop", "Hannah- hip hop")
	if c := Generate([]weave.Thread{other}, nil, nil, nil, DefaultOptions(observed))[0].ID; c == a {
		t.Error("want different gaps to carry different ids")
	}
}

func TestGenerateOrdersHeaviestFirst(t *testing.T) {
	t.Parallel()
	heavy := endedThread("arts martial william", "William- Martial arts")
	lighter := weave.Thread{
		Key: "acro hannah", Label: "Hannah- Acro", Count: 42,
		First: day(2023, time.March, 1), Last: day(2024, time.April, 12),
		MedianGap: 7, SilentDays: 866, SpanDays: 408, Status: weave.Ended,
	}
	got := Generate([]weave.Thread{lighter, heavy}, nil, nil, nil, DefaultOptions(observed))
	if len(got) != 2 {
		t.Fatalf("want two questions, got %d", len(got))
	}
	if got[0].Prompt != heavy.Label+" stopped. What happened?" {
		t.Errorf("want the bigger loss asked first, got %q", got[0].Prompt)
	}
}

func TestCrossingQuestionCarriesBothSides(t *testing.T) {
	t.Parallel()
	c := weave.Overlap{
		Day:       day(2026, time.July, 30),
		Counts:    map[string]int{"calendar": 1, "git": 113},
		Headlines: map[string]string{"calendar": "Dad- off work/ vacation", "git": "Trim projects"},
	}
	got := Generate(nil, []weave.Overlap{c}, nil, nil, DefaultOptions(observed))
	if len(got) != 1 {
		t.Fatalf("want one crossing question, got %d", len(got))
	}
	// A crossing is only meaningful if both sides are shown; either alone is
	// unremarkable.
	for _, want := range []string{"off work", "Trim projects"} {
		if !strings.Contains(got[0].Context, want) {
			t.Errorf("want %q in the evidence, got %q", want, got[0].Context)
		}
	}
}

func TestGenerateEmptyRecord(t *testing.T) {
	t.Parallel()
	if got := Generate(nil, nil, nil, nil, DefaultOptions(observed)); len(got) != 0 {
		t.Errorf("want nothing asked of an empty record, got %d", len(got))
	}
}

func TestGenerateSkipsShortLivedThreads(t *testing.T) {
	t.Parallel()
	// A school-year reminder repeated for one term. It carries enough weight to
	// pass the volume bar, but it was logistics, never a commitment, so asking
	// why it stopped mistakes a to-do list for a life.
	reminder := weave.Thread{
		Key: "book class folder red return shirt wear william", Label: "William- wear red class shirt",
		Count: 16, First: day(2024, time.January, 26), Last: day(2024, time.May, 10),
		MedianGap: 7, SilentDays: 838, SpanDays: 105, Status: weave.Ended,
	}
	if got := Generate([]weave.Thread{reminder}, nil, nil, nil, DefaultOptions(observed)); len(got) != 0 {
		t.Errorf("want a short reminder run ignored, got %q", got[0].Prompt)
	}
}

func TestGenerateSkipsDormantThreads(t *testing.T) {
	t.Parallel()
	seasonal := weave.Thread{
		Key: "haylie show spring", Label: "Haylie- spring show",
		Count: 6, First: day(2023, time.April, 25), Last: day(2026, time.April, 25),
		MedianGap: 1, MaxGap: 760, SilentDays: 123, SpanDays: 1096, Status: weave.Dormant,
	}
	// Asking a person to explain the end of something that has not ended asserts
	// a false fact about their life.
	if got := Generate([]weave.Thread{seasonal}, nil, nil, nil, DefaultOptions(observed)); len(got) != 0 {
		t.Errorf("want a dormant thread left alone, got %q", got[0].Prompt)
	}
}

func TestGapQuestionOutranksEveryThread(t *testing.T) {
	t.Parallel()
	heavy := endedThread("arts martial william", "William- Martial arts")
	gap := weave.Gap{
		From: day(2017, time.July, 1), To: day(2022, time.July, 31),
		Months: 61, Entries: 6, Before: 4, After: 98,
	}
	got := Generate([]weave.Thread{heavy}, nil, []weave.Gap{gap}, nil, DefaultOptions(observed))
	if len(got) != 2 {
		t.Fatalf("want both questions, got %d", len(got))
	}
	// Years the record cannot account for is a larger hole than any one
	// commitment ending, and the two are not measured on comparable scales.
	if got[0].Kind != KindGap {
		t.Errorf("want the silence asked first, got %s", got[0].Kind)
	}
}
