package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/index"
	"github.com/dcadolph/midden/llm"
)

// mockChatter is a Chatter whose reply is produced by a configurable function.
type mockChatter struct {
	// ReplyFunc produces the reply for one call, receiving the call ordinal.
	ReplyFunc func(call int, system string, history []llm.Message) (string, error)
	// calls counts how many times Reply was invoked.
	calls atomic.Int64
}

// Name identifies the mock provider.
func (m *mockChatter) Name() string { return "mock:chatter" }

// Reply delegates to ReplyFunc, recording the call ordinal.
func (m *mockChatter) Reply(_ context.Context, system string, history []llm.Message) (string, error) {
	return m.ReplyFunc(int(m.calls.Add(1)-1), system, history)
}

// sweepCmd returns a command with output discarded, for driving sweepContext.
func sweepCmd() *cobra.Command {
	c := &cobra.Command{}
	c.SetOut(new(strings.Builder))
	c.SetErr(new(strings.Builder))
	return c
}

func TestSweepContextRendersSmallRangeVerbatim(t *testing.T) {
	t.Parallel()
	chat := &mockChatter{ReplyFunc: func(int, string, []llm.Message) (string, error) {
		return "", errors.New("summarizer must not run for a range within budget")
	}}
	entries := []index.Entry{chunkEntry(0, 10), chunkEntry(1, 10)}
	got, err := sweepContext(context.Background(), sweepCmd(), chat, "what happened", entries)
	if err != nil {
		t.Fatalf("sweepContext: %v", err)
	}
	if chat.calls.Load() != 0 {
		t.Errorf("want no summarizer calls, got %d", chat.calls.Load())
	}
	if !strings.Contains(got, "2024-01-01") || !strings.Contains(got, "2024-01-02") {
		t.Errorf("want both entries rendered, got:\n%s", got)
	}
}

func TestSweepContextSummarizesOversizeRangeInOrder(t *testing.T) {
	t.Parallel()
	// Three entries, each two thirds of the chunk budget, force three chunks
	// and push the total past the sweep budget.
	size := chatChunkBudget * 2 / 3
	entries := []index.Entry{}
	for i := range chatSweepBudget/size + 2 {
		entries = append(entries, chunkEntry(i, size))
	}
	chat := &mockChatter{ReplyFunc: func(_ int, _ string, history []llm.Message) (string, error) {
		// Echo the first date in the chunk so ordering is verifiable.
		line := strings.SplitN(history[0].Content, "--- ", 2)[1]
		return "summary of " + strings.SplitN(line, " ", 2)[0], nil
	}}
	got, err := sweepContext(context.Background(), sweepCmd(), chat, "what happened", entries)
	if err != nil {
		t.Fatalf("sweepContext: %v", err)
	}
	if chat.calls.Load() < 2 {
		t.Fatalf("want the range summarized in several chunks, got %d calls", chat.calls.Load())
	}
	// Every chunk must appear, and the summaries must stay chronological even
	// though they were produced concurrently.
	var last string
	for _, line := range strings.Split(got, "\n") {
		if !strings.HasPrefix(line, "--- ") {
			continue
		}
		date := strings.SplitN(strings.TrimPrefix(line, "--- "), " ", 2)[0]
		if last != "" && date < last {
			t.Errorf("summaries out of order: %s came after %s", date, last)
		}
		last = date
	}
	if last == "" {
		t.Errorf("want labeled chunk summaries, got:\n%s", got)
	}
}

func TestSweepContextFailsWhenAChunkFails(t *testing.T) {
	t.Parallel()
	size := chatChunkBudget * 2 / 3
	entries := []index.Entry{}
	for i := range chatSweepBudget/size + 2 {
		entries = append(entries, chunkEntry(i, size))
	}
	chat := &mockChatter{ReplyFunc: func(call int, _ string, _ []llm.Message) (string, error) {
		if call == 1 {
			return "", errors.New("provider exploded")
		}
		return "fine", nil
	}}
	// A dropped chunk would leave a silent hole in a range the answer claims to
	// cover, so a single chunk failure must fail the sweep.
	_, err := sweepContext(context.Background(), sweepCmd(), chat, "what happened", entries)
	if err == nil {
		t.Fatal("want an error when a chunk summary fails")
	}
	if !strings.Contains(err.Error(), "provider exploded") {
		t.Errorf("want the provider failure surfaced, got %v", err)
	}
}

func TestSweepContextEmptyRange(t *testing.T) {
	t.Parallel()
	chat := &mockChatter{ReplyFunc: func(int, string, []llm.Message) (string, error) {
		return "", errors.New("must not run")
	}}
	got, err := sweepContext(context.Background(), sweepCmd(), chat, "what happened", nil)
	if err != nil {
		t.Fatalf("sweepContext: %v", err)
	}
	if got != "" {
		t.Errorf("want empty context, got %q", got)
	}
}

func TestSweepContextRespectsCancellation(t *testing.T) {
	t.Parallel()
	size := chatChunkBudget * 2 / 3
	entries := []index.Entry{}
	for i := range chatSweepBudget/size + 2 {
		entries = append(entries, chunkEntry(i, size))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	chat := &mockChatter{ReplyFunc: func(int, string, []llm.Message) (string, error) {
		return "", fmt.Errorf("canceled")
	}}
	if _, err := sweepContext(ctx, sweepCmd(), chat, "what happened", entries); err == nil {
		t.Fatal("want an error once the context is canceled")
	}
}
