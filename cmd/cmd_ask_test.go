package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/dcadolph/midden/internal/vault"
)

// askVault returns a vault holding a weekly class that ran for months and then
// stopped, which is the shape ask is built to notice.
func askVault(t *testing.T) *vault.Vault {
	t.Helper()
	v, err := vault.Open(t.TempDir())
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	start := time.Now().AddDate(-2, 0, 0)
	entries := make([]vault.Entry, 0, 40)
	for i := range 40 {
		entries = append(entries, vault.Entry{
			Time: start.AddDate(0, 0, 7*i),
			Tags: []string{"calendar"},
			Body: "William- martial arts",
		})
	}
	if err := v.AppendAll(entries); err != nil {
		t.Fatalf("AppendAll: %v", err)
	}
	return v
}

// entriesOf reads every entry in the vault.
func entriesOf(t *testing.T, v *vault.Vault) []vault.Entry {
	t.Helper()
	out, err := entriesInRange(v, dateRange{})
	if err != nil {
		t.Fatalf("entriesInRange: %v", err)
	}
	return out
}

func TestPendingQuestionsFindsAnEnding(t *testing.T) {
	t.Parallel()
	got := pendingQuestions(entriesOf(t, askVault(t)))
	if len(got) == 0 {
		t.Fatal("want a question about the class that stopped")
	}
	if !strings.Contains(got[0].Prompt, "martial arts") {
		t.Errorf("want the ended thread asked about, got %q", got[0].Prompt)
	}
}

func TestAnsweredQuestionsAreRetired(t *testing.T) {
	t.Parallel()
	v := askVault(t)
	first := pendingQuestions(entriesOf(t, v))
	if len(first) == 0 {
		t.Fatal("want a question to answer")
	}
	// An answer entry carries the marker, which is what retires the question.
	err := v.Append(vault.Entry{
		Time: time.Now(),
		Tags: []string{"answer"},
		Body: "He switched to baseball.\n\n" + askedPrefixLabel + first[0].Prompt + "\n" + askedPrefix + first[0].ID,
	})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	for _, q := range pendingQuestions(entriesOf(t, v)) {
		if q.ID == first[0].ID {
			t.Error("want an answered question never asked again")
		}
	}
}

func TestDismissedQuestionsAreAlsoRetired(t *testing.T) {
	t.Parallel()
	v := askVault(t)
	first := pendingQuestions(entriesOf(t, v))
	if len(first) == 0 {
		t.Fatal("want a question to dismiss")
	}
	// A queue that keeps returning a rejected question trains the person to stop
	// reading it, so a dismissal has to stick exactly like an answer.
	err := v.Append(vault.Entry{
		Time: time.Now(),
		Tags: []string{"skipped"},
		Body: "Not worth recording.\n" + askedPrefixLabel + first[0].Prompt + "\n" + askedPrefix + first[0].ID,
	})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	for _, q := range pendingQuestions(entriesOf(t, v)) {
		if q.ID == first[0].ID {
			t.Error("want a dismissed question never asked again")
		}
	}
}

func TestAnswersDoNotBecomeQuestionsThemselves(t *testing.T) {
	t.Parallel()
	v := askVault(t)
	q := pendingQuestions(entriesOf(t, v))
	// Enough answers to form a thread on their own if they were not excluded.
	for i := range 8 {
		err := v.Append(vault.Entry{
			Time: time.Now().AddDate(0, 0, -i*7),
			Tags: []string{"answer"},
			Body: "Some answer text.\n\n" + askedPrefixLabel + q[0].Prompt + "\n" + askedPrefix + "fake-" + string(rune('a'+i)),
		})
		if err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	// The interviewer asking about its own past questions would be a loop that
	// never closes.
	for _, got := range pendingQuestions(entriesOf(t, v)) {
		if strings.Contains(got.Prompt, "Some answer text") || strings.Contains(got.Prompt, "In answer to") {
			t.Errorf("want answers excluded from question generation, got %q", got.Prompt)
		}
	}
}

func TestBodyMarkerReadsTheAskedID(t *testing.T) {
	t.Parallel()
	body := "He switched to baseball.\n\n" + askedPrefixLabel + "William- Martial arts stopped. What happened?\n" + askedPrefix + "ended-191c9382"
	if diff := cmp.Diff("ended-191c9382", bodyMarker(body, askedPrefix)); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
	// The headline is the person's own words, which is what every read command
	// shows and what weave groups on.
	if diff := cmp.Diff("He switched to baseball.", firstLine(body)); diff != "" {
		t.Errorf("headline mismatch (-want +got):\n%s", diff)
	}
}

func TestReadAnswerAcceptsSeveralLines(t *testing.T) {
	t.Parallel()
	got, err := readAnswer(strings.NewReader("first line\nsecond line\n\nignored after the blank\n"))
	if err != nil {
		t.Fatalf("readAnswer: %v", err)
	}
	if diff := cmp.Diff("first line\nsecond line", got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestReadAnswerHandlesEmptyInput(t *testing.T) {
	t.Parallel()
	got, err := readAnswer(strings.NewReader(""))
	if err != nil {
		t.Fatalf("readAnswer: %v", err)
	}
	if got != "" {
		t.Errorf("want an empty answer, got %q", got)
	}
}
