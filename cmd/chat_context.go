package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/index"
	"github.com/dcadolph/midden/llm"
)

// Budgets for a swept range, measured in characters of entry bodies. A range
// small enough to fit is sent verbatim; a larger one is summarized in
// chronological chunks so the answer still covers the whole span rather than a
// prefix of it.
const (
	// chatSweepBudget is the most entry text sent to the model in one prompt.
	chatSweepBudget = 400000
	// chatChunkBudget is the most entry text summarized in a single chunk.
	chatChunkBudget = 60000
	// chatChunkWorkers bounds how many chunk summaries run at once.
	chatChunkWorkers = 4
)

// chunkSystem frames the map step of a swept range. The summaries are read only
// by the reduce step, so they are told to keep the specifics an answer needs
// rather than to read well on their own.
const chunkSystem = "You are compressing part of a personal journal and calendar archive so a later question " +
	"can be answered from it. Preserve concrete specifics: dates, people, places, projects, events, " +
	"recurring commitments, and anything that marks a change. Drop routine filler. " +
	"Do not speculate beyond the entries and do not add a preamble."

// sweepContext renders every entry in the range as chat context. Ranges within
// the budget are rendered verbatim; larger ranges are summarized in
// chronological chunks first, which keeps coverage of the whole span at the
// cost of detail rather than truncating the span at full detail.
func sweepContext(
	ctx context.Context,
	cmd *cobra.Command,
	chat llm.Chatter,
	question string,
	entries []index.Entry,
) (string, error) {
	if len(entries) == 0 {
		return "", nil
	}
	if entriesSize(entries) <= chatSweepBudget {
		return renderEntries(entries), nil
	}
	chunks := chunkEntries(entries, chatChunkBudget)
	fmt.Fprintf(cmd.ErrOrStderr(),
		"Range holds %d entries, too many to read at once; summarizing in %d chunks.\n",
		len(entries), len(chunks))
	summaries, err := summarizeChunks(ctx, chat, question, chunks)
	if err != nil {
		return "", err
	}
	return strings.Join(summaries, "\n\n"), nil
}

// summarizeChunks compresses each chunk concurrently and returns the summaries
// in chronological order. Any chunk failing fails the sweep, because a silently
// dropped chunk would leave a hole in a range the answer claims to cover.
func summarizeChunks(ctx context.Context, chat llm.Chatter, question string, chunks [][]index.Entry) ([]string, error) {
	out := make([]string, len(chunks))
	errs := make([]error, len(chunks))
	sem := make(chan struct{}, chatChunkWorkers)
	var wg sync.WaitGroup
	for i, chunk := range chunks {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return nil, fmt.Errorf("summarize range: %w", ctx.Err())
		}
		wg.Add(1)
		go func(i int, chunk []index.Entry) {
			defer wg.Done()
			defer func() { <-sem }()
			label := chunkLabel(chunk)
			user := "Later question: " + question + "\n\nEntries:\n" + renderEntries(chunk)
			reply, err := chat.Reply(ctx, chunkSystem, []llm.Message{{Role: "user", Content: user}})
			if err != nil {
				errs[i] = fmt.Errorf("summarize %s: %w", label, err)
				return
			}
			out[i] = "--- " + label + "\n" + strings.TrimSpace(reply)
		}(i, chunk)
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return out, nil
}

// chunkEntries splits chronologically ordered entries into runs whose bodies
// stay within the budget. An entry larger than the budget on its own becomes a
// chunk of one rather than being dropped or split mid-body.
func chunkEntries(entries []index.Entry, budget int) [][]index.Entry {
	var (
		out   [][]index.Entry
		chunk []index.Entry
		size  int
	)
	for _, e := range entries {
		n := len(e.Body)
		if len(chunk) > 0 && size+n > budget {
			out = append(out, chunk)
			chunk, size = nil, 0
		}
		chunk = append(chunk, e)
		size += n
	}
	if len(chunk) > 0 {
		out = append(out, chunk)
	}
	return out
}

// chunkLabel renders the date span a chunk covers.
func chunkLabel(chunk []index.Entry) string {
	if len(chunk) == 0 {
		return "empty range"
	}
	first := chunk[0].Time.Format(layoutDate)
	last := chunk[len(chunk)-1].Time.Format(layoutDate)
	if first == last {
		return first
	}
	return first + " to " + last
}

// entriesSize returns the total body length across the entries.
func entriesSize(entries []index.Entry) int {
	total := 0
	for _, e := range entries {
		total += len(e.Body)
	}
	return total
}

// renderEntries renders entries as the dated blocks the chat model reads.
func renderEntries(entries []index.Entry) string {
	var b strings.Builder
	for _, e := range entries {
		b.WriteString("--- ")
		b.WriteString(e.Time.Format(layoutDateTime))
		if len(e.Tags) > 0 {
			b.WriteString(" [")
			b.WriteString(strings.Join(e.Tags, ", "))
			b.WriteString("]")
		}
		b.WriteString("\n")
		b.WriteString(e.Body)
		b.WriteString("\n\n")
	}
	return b.String()
}

// renderDigest renders the corpus summary supplied with every answer. The
// counts come from every indexed entry in scope, which is what lets the model
// answer about the record as a whole rather than about the entries it was sent.
func renderDigest(d index.Digest, label string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Corpus summary, counted over every indexed entry in scope (%s)\n", label)
	if d.Entries == 0 {
		b.WriteString("Entries: 0\n")
		return b.String()
	}
	fmt.Fprintf(&b, "Entries: %d\n", d.Entries)
	fmt.Fprintf(&b, "Span: %s to %s\n", d.First.Format(layoutDate), d.Last.Format(layoutDate))
	if len(d.TopTags) > 0 {
		parts := make([]string, len(d.TopTags))
		for i, t := range d.TopTags {
			parts[i] = fmt.Sprintf("%s (%d)", t.Tag, t.Count)
		}
		fmt.Fprintf(&b, "Tags by entry count: %s\n", strings.Join(parts, ", "))
	}
	if len(d.Months) > 0 {
		parts := make([]string, len(d.Months))
		for i, m := range d.Months {
			parts[i] = fmt.Sprintf("%s (%d)", m.Month, m.Count)
		}
		fmt.Fprintf(&b, "Entries per month: %s\n", strings.Join(parts, ", "))
	}
	return b.String()
}
