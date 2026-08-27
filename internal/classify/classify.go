// Package classify proposes identity merges the arithmetic cannot make on its
// own, using a model under strict rules.
//
// Word overlap groups "sleepover with Kayla" and "sleepover @ Kayla's", but it
// cannot know that Will and William are one person or that two differently
// named activities are the same commitment. A model can, and a model also
// invents things, so the boundary here is absolute: arithmetic generates the
// candidate pairs, the model answers only yes or no about a pair it is shown,
// the answer is an ephemeral proposal carrying its evidence, and nothing
// changes anywhere until a person accepts. An accepted verdict is recorded as
// an ordinary vault entry the person approved, which the arithmetic then reads
// like any other fact. The model never writes.
package classify

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dcadolph/midden/internal/weave"
	"github.com/dcadolph/midden/llm"
)

// Kind is what a candidate pair is about.
type Kind string

// The pair kinds.
const (
	// KindThread pairs two recurring threads that may be one activity.
	KindThread Kind = "thread"
	// KindPerson pairs two names that may be one person.
	KindPerson Kind = "person"
)

// Candidate is a pair the arithmetic flagged as possibly identical.
type Candidate struct {
	// Kind is what the pair is about.
	Kind Kind
	// A and B are display labels for the two sides.
	A, B string
	// KeyA and KeyB identify the sides stably, for markers.
	KeyA, KeyB string
	// Evidence is what the arithmetic knows about each side, shown to the
	// person and to the model alike.
	Evidence string
}

// MarkerKey renders the stable identity of the pair, order-independent.
func (c Candidate) MarkerKey() string {
	a, b := c.KeyA, c.KeyB
	if b < a {
		a, b = b, a
	}
	return string(c.Kind) + "|" + a + "|" + b
}

// Verdict is the model's answer about one candidate.
type Verdict struct {
	// Candidate is the pair judged.
	Candidate Candidate
	// Same reports the model's answer, or nil when the model gave none that
	// parsed. No answer is never treated as an answer.
	Same *bool
}

// ThreadPairs returns thread pairs worth asking about: enough word overlap to
// be suspicious but not enough for the arithmetic to have merged them, or a
// same-subject succession. Ordered by combined weight, capped.
func ThreadPairs(threads []weave.Thread, limit int) []Candidate {
	type scored struct {
		c Candidate
		w int
	}
	var out []scored
	for i := 0; i < len(threads); i++ {
		for j := i + 1; j < len(threads); j++ {
			a, b := threads[i], threads[j]
			// A shared first name alone is not a resemblance: every one of a
			// child's activities shares their name, and pairing martial arts
			// with a birthday would bury the real questions. Worth asking means
			// at least a third of the combined words are shared, or one thread
			// began soon after the other ended within one subject, which is the
			// shape a renamed continuation has.
			jac := jaccardKeys(a.Key, b.Key)
			succession := weave.SharesSubject(a, b) && adjacent(a, b, 180)
			if jac < 1.0/3 && !succession {
				continue
			}
			out = append(out, scored{
				c: Candidate{
					Kind: KindThread,
					A:    a.Label, B: b.Label,
					KeyA: a.Key, KeyB: b.Key,
					Evidence: threadEvidence(a) + "\n" + threadEvidence(b),
				},
				w: a.Weight() + b.Weight(),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].w != out[j].w {
			return out[i].w > out[j].w
		}
		return out[i].c.MarkerKey() < out[j].c.MarkerKey()
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	final := make([]Candidate, len(out))
	for i, s := range out {
		final[i] = s.c
	}
	return final
}

// PersonPairs returns name pairs that may be one person: one name a prefix of
// the other, or a single letter apart. Ordered by combined mentions, capped.
func PersonPairs(people []weave.Person, limit int) []Candidate {
	type scored struct {
		c Candidate
		w int
	}
	var out []scored
	for i := 0; i < len(people); i++ {
		for j := i + 1; j < len(people); j++ {
			a, b := people[i], people[j]
			la, lb := strings.ToLower(a.Name), strings.ToLower(b.Name)
			if !namesResemble(la, lb) {
				continue
			}
			out = append(out, scored{
				c: Candidate{
					Kind: KindPerson,
					A:    a.Name, B: b.Name,
					KeyA: la, KeyB: lb,
					Evidence: personEvidenceLine(a) + "\n" + personEvidenceLine(b),
				},
				w: a.Mentions + b.Mentions,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].w != out[j].w {
			return out[i].w > out[j].w
		}
		return out[i].c.MarkerKey() < out[j].c.MarkerKey()
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	final := make([]Candidate, len(out))
	for i, s := range out {
		final[i] = s.c
	}
	return final
}

// classifySystem constrains the model to a bare yes-or-no. The schema is the
// whole contract: anything that does not parse is treated as no answer.
const classifySystem = "You judge whether two items from one person's private records refer to the same " +
	"underlying thing. Answer ONLY with JSON of the form {\"same\": true} or {\"same\": false}. " +
	"No prose, no explanation, no markdown."

// Judge asks the model about each candidate in turn. A model failure or an
// unparseable reply yields a nil verdict for that pair rather than a guess,
// and never stops the remaining pairs from being judged.
func Judge(ctx context.Context, chat llm.Chatter, candidates []Candidate, perCall time.Duration) []Verdict {
	out := make([]Verdict, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, Verdict{Candidate: c, Same: judgeOne(ctx, chat, c, perCall)})
	}
	return out
}

// judgeOne runs a single strict-schema call.
func judgeOne(ctx context.Context, chat llm.Chatter, c Candidate, perCall time.Duration) *bool {
	callCtx, cancel := context.WithTimeout(ctx, perCall)
	defer cancel()
	kind := "recurring activities"
	if c.Kind == KindPerson {
		kind = "people"
	}
	user := fmt.Sprintf("Are these two %s the same?\n\nItem A: %s\nItem B: %s\n\nEvidence:\n%s",
		kind, c.A, c.B, c.Evidence)
	reply, err := chat.Reply(callCtx, classifySystem, []llm.Message{{Role: "user", Content: user}})
	if err != nil {
		return nil
	}
	return parseVerdict(reply)
}

// parseVerdict extracts the schema answer from a reply, tolerating wrapping
// prose but nothing structurally loose: the first balanced JSON object must
// carry a boolean "same" and may carry nothing else worth trusting.
func parseVerdict(reply string) *bool {
	start := strings.IndexByte(reply, '{')
	if start < 0 {
		return nil
	}
	depth := 0
	end := -1
	for i := start; i < len(reply); i++ {
		switch reply[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
			}
		}
		if end > 0 {
			break
		}
	}
	if end < 0 {
		return nil
	}
	var parsed struct {
		Same *bool `json:"same"`
	}
	if err := json.Unmarshal([]byte(reply[start:end]), &parsed); err != nil {
		return nil
	}
	return parsed.Same
}

// threadEvidence renders what the arithmetic knows about a thread.
func threadEvidence(t weave.Thread) string {
	return fmt.Sprintf("- %q: %d occurrences, %s to %s, status %s, spellings %s",
		t.Label, t.Count, t.First.Format("2006-01-02"), t.Last.Format("2006-01-02"),
		t.Status, strings.Join(t.Variants, " / "))
}

// personEvidenceLine renders what the arithmetic knows about a person.
func personEvidenceLine(p weave.Person) string {
	return fmt.Sprintf("- %q: %d mentions, %s to %s, seen in: %s",
		p.Name, p.Mentions, p.First.Format("2006-01-02"), p.Last.Format("2006-01-02"),
		strings.Join(p.Contexts, " / "))
}

// namesResemble reports whether two lowercase names could be one person:
// prefix of one another, or one edit apart.
func namesResemble(a, b string) bool {
	if len(a) < 3 || len(b) < 3 {
		return false
	}
	if strings.HasPrefix(a, b) || strings.HasPrefix(b, a) {
		return true
	}
	return oneEditApart(a, b)
}

// oneEditApart reports whether two strings differ by a single insertion,
// deletion, or substitution.
func oneEditApart(a, b string) bool {
	if len(a) > len(b) {
		a, b = b, a
	}
	if len(b)-len(a) > 1 {
		return false
	}
	edits, i, j := 0, 0, 0
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			i++
			j++
			continue
		}
		edits++
		if edits > 1 {
			return false
		}
		if len(a) == len(b) {
			i++
		}
		j++
	}
	if j < len(b) || i < len(a) {
		edits++
	}
	return edits == 1
}

// jaccardKeys returns word-set overlap between two normalized keys.
func jaccardKeys(a, b string) float64 {
	aw := strings.Fields(a)
	bw := strings.Fields(b)
	if len(aw) == 0 || len(bw) == 0 {
		return 0
	}
	set := map[string]bool{}
	for _, w := range aw {
		set[w] = true
	}
	inter := 0
	seen := map[string]bool{}
	distinctB := map[string]bool{}
	for _, w := range bw {
		distinctB[w] = true
		if set[w] && !seen[w] {
			inter++
			seen[w] = true
		}
	}
	union := len(set) + len(distinctB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// adjacent reports whether one thread begins within the window after the other
// ends, in either direction.
func adjacent(a, b weave.Thread, windowDays int) bool {
	gap1 := b.First.Sub(a.Last).Hours() / 24
	gap2 := a.First.Sub(b.Last).Hours() / 24
	return (gap1 >= 0 && gap1 <= float64(windowDays)) || (gap2 >= 0 && gap2 <= float64(windowDays))
}
