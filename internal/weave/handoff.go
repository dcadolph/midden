package weave

import (
	"sort"
	"time"
)

// Handoff pairs a thread that ended with one that began soon after. A person
// remembers the activities themselves but rarely the succession between them,
// and the succession is where the story is: a thing stopped, and something took
// its place a few weeks later.
type Handoff struct {
	// From is the thread that ended.
	From Thread
	// To is the thread that began after it.
	To Thread
	// GapDays is how long passed between the last of one and the first of the other.
	GapDays int
}

// HandoffOptions tune succession detection.
type HandoffOptions struct {
	// Window is the most days that may pass between an ending and a beginning
	// for the two to count as a succession.
	Window int
	// SameSubject reports whether two threads concern the same subject. A
	// handoff is only meaningful within one life, so a child dropping an
	// activity should not pair with a sibling starting one. Nil pairs everything.
	SameSubject func(a, b Thread) bool
}

// DefaultHandoffOptions returns succession settings suited to a personal record.
// Successions are held to a shared subject, because one person dropping an
// activity has nothing to do with another person starting one, and pairing them
// would manufacture a story the record does not contain.
func DefaultHandoffOptions() HandoffOptions {
	return HandoffOptions{Window: 120, SameSubject: SharesSubject}
}

// SharesSubject reports whether two threads name a common subject, judged by a
// word they both carry that is distinctive enough to be a person or activity.
func SharesSubject(a, b Thread) bool {
	aw, bw := wordSet(a.Key), wordSet(b.Key)
	for w := range aw {
		if bw[w] && len(w) > 2 {
			return true
		}
	}
	return false
}

// Handoffs finds successions among the given threads, ordered by the weight of
// the thread that ended, so the largest losses come first.
func Handoffs(threads []Thread, opts HandoffOptions) []Handoff {
	if opts.Window <= 0 {
		opts.Window = 120
	}
	var out []Handoff
	for _, from := range threads {
		if from.Status != Ended {
			continue
		}
		for _, to := range threads {
			if to.Key == from.Key || to.First.Before(from.Last) {
				continue
			}
			gap := daysBetween(from.Last, to.First)
			if gap > opts.Window {
				continue
			}
			if opts.SameSubject != nil && !opts.SameSubject(from, to) {
				continue
			}
			out = append(out, Handoff{From: from, To: to, GapDays: gap})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].From.Weight() != out[j].From.Weight() {
			return out[i].From.Weight() > out[j].From.Weight()
		}
		return out[i].GapDays < out[j].GapDays
	})
	return out
}

// Milestone is a dated turning point in the record.
type Milestone struct {
	// When the turning point happened.
	When time.Time
	// Kind is what happened, either "ended" or "began".
	Kind string
	// Thread is the thread that turned.
	Thread Thread
}

// Milestones returns the endings and beginnings among the given threads in
// chronological order, which is the order a life is told in.
func Milestones(threads []Thread) []Milestone {
	var out []Milestone
	for _, t := range threads {
		switch t.Status {
		case Ended:
			out = append(out, Milestone{When: t.Last, Kind: "ended", Thread: t})
		case Emerging:
			out = append(out, Milestone{When: t.First, Kind: "began", Thread: t})
		case Ongoing:
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].When.Equal(out[j].When) {
			return out[i].When.Before(out[j].When)
		}
		return out[i].Thread.Weight() > out[j].Thread.Weight()
	})
	return out
}
