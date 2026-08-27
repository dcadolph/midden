// Package weave finds the shape of a record over time: what recurs, when it
// started, when it stopped, and what took its place.
//
// The detection here is arithmetic rather than inference. A person can recall
// what they did but cannot perceive absence, because nothing marks the last
// time something happened. Counting occurrences and measuring silence against a
// thread's own cadence surfaces those endings exactly, with no model in the
// path and nothing to hallucinate.
package weave

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dcadolph/midden/internal/vault"
)

// Status is where a thread stands at the observation date.
type Status string

// The statuses a thread can hold.
const (
	// Ongoing means the thread is still within its usual cadence.
	Ongoing Status = "ongoing"
	// Dormant means the thread is quiet but has already come back from a gap
	// this long before, so the silence is its rhythm rather than its end.
	Dormant Status = "dormant"
	// Ended means the thread has been silent far longer than its cadence.
	Ended Status = "ended"
	// Emerging means the thread began recently and is still establishing.
	Emerging Status = "emerging"
)

// Thread is a recurring pattern of entries sharing a normalized title.
type Thread struct {
	// Key is the normalized title threads are grouped by.
	Key string
	// Label is a representative original title, for display.
	Label string
	// Count is how many entries belong to the thread.
	Count int
	// First and Last are the earliest and latest occurrence.
	First time.Time
	Last  time.Time
	// MedianGap is the typical number of days between occurrences.
	MedianGap int
	// MaxGap is the longest the thread has ever gone quiet and come back. A
	// yearly show and a weekly class both look silent in July; only this tells
	// them apart.
	MaxGap int
	// GapSpread is the coefficient of variation of the thread's gaps: zero for a
	// metronome, above one for an erratic pattern. Regularity is what earns a
	// quick verdict; an irregular thread has to be silent far longer before its
	// silence means anything.
	GapSpread float64
	// SilentDays is how long the thread has gone quiet at the observation date.
	SilentDays int
	// SpanDays is how long the thread ran from first to last occurrence.
	SpanDays int
	// Status is where the thread stands.
	Status Status
	// Variants are the distinct headlines that were folded into this thread.
	// A record whose whole worth is being true has to be auditable: a reader
	// must be able to see what was grouped together before believing a claim
	// built on the grouping.
	Variants []string
}

// Weight ranks how much of a life a thread represents, combining how often it
// happened with how long it ran. A weekly commitment held for two years outranks
// a burst of activity over one month, which is the order a person would tell
// them in.
func (t Thread) Weight() int {
	return t.Count * (t.SpanDays + 1)
}

// Options tune thread detection.
type Options struct {
	// Now is the observation date. Occurrences after it are ignored, so a
	// calendar holding future events does not report them as current activity.
	Now time.Time
	// MinCount is the fewest occurrences a pattern needs to count as a thread.
	MinCount int
	// SilenceFactor multiplies a thread's own cadence to decide when silence
	// means it ended. Measuring against the thread's own rhythm is what lets a
	// daily habit and a yearly tradition be judged on the same scale.
	SilenceFactor int
	// MinSilenceDays floors the silence test, so a thread that happened twice in
	// two days is not declared over by the third day.
	MinSilenceDays int
	// EmergingDays is how recently a thread must have started to count as new.
	EmergingDays int
	// DormantTolerance scales a thread's longest previous gap when deciding
	// whether its current silence is seasonal rather than final.
	DormantTolerance float64
	// MergeSimilarity is how much two word sets must overlap to be treated as
	// one thread, from zero to one. Word-set equality alone still splits a
	// commitment recorded with an extra word attached, and each fragment then
	// appears to end whenever the wording drifts.
	MergeSimilarity float64
}

// DefaultOptions returns detection settings suited to a personal record.
func DefaultOptions(now time.Time) Options {
	return Options{
		Now:              now,
		MinCount:         5,
		SilenceFactor:    4,
		MinSilenceDays:   90,
		EmergingDays:     180,
		MergeSimilarity:  0.6,
		DormantTolerance: 1.3,
	}
}

// Threads groups entries into recurring threads and classifies each one.
// The result is ordered by weight, heaviest first.
func Threads(entries []vault.Entry, opts Options) []Thread {
	if opts.MinCount < 2 {
		opts.MinCount = 2
	}
	if opts.SilenceFactor < 1 {
		opts.SilenceFactor = 1
	}
	grouped := map[string][]time.Time{}
	labels := map[string]string{}
	variants := map[string]map[string]bool{}
	for _, e := range entries {
		if !opts.Now.IsZero() && e.Time.After(opts.Now) {
			continue
		}
		key := NormalizeTitle(e.Body)
		if key == "" {
			continue
		}
		grouped[key] = append(grouped[key], e.Time)
		if _, ok := labels[key]; !ok {
			labels[key] = headline(e.Body)
		}
		if variants[key] == nil {
			variants[key] = map[string]bool{}
		}
		variants[key][headline(e.Body)] = true
	}
	grouped, labels, variants = mergeSimilar(grouped, labels, variants, opts.MergeSimilarity)
	out := make([]Thread, 0, len(grouped))
	for key, times := range grouped {
		if len(times) < opts.MinCount {
			continue
		}
		t := buildThread(key, labels[key], times, opts)
		t.Variants = sortedSet(variants[key])
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Weight() != out[j].Weight() {
			return out[i].Weight() > out[j].Weight()
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// buildThread assembles one thread from its occurrence times.
func buildThread(key, label string, times []time.Time, opts Options) Thread {
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	first, last := times[0], times[len(times)-1]
	t := Thread{
		Key:       key,
		Label:     label,
		Count:     len(times),
		First:     first,
		Last:      last,
		MedianGap: medianGapDays(times),
		MaxGap:    maxGapDays(times),
		GapSpread: gapSpread(times),
		SpanDays:  daysBetween(first, last),
	}
	if !opts.Now.IsZero() {
		t.SilentDays = daysBetween(last, opts.Now)
	}
	t.Status = classify(t, opts)
	return t
}

// classify decides where a thread stands. Silence is judged against the
// thread's own cadence rather than a fixed window, so a weekly class and an
// annual tradition are both measured fairly, and the cadence bar stretches with
// the thread's own irregularity: a metronomic weekly class earns a verdict at
// four missed beats, while an erratic social thread with the same median gap
// has always had long pauses, so the same silence proves nothing about it.
func classify(t Thread, opts Options) Status {
	spread := t.GapSpread
	if spread > 1.5 {
		spread = 1.5
	}
	limit := int(float64(t.MedianGap*opts.SilenceFactor) * (1 + spread))
	if limit < opts.MinSilenceDays {
		limit = opts.MinSilenceDays
	}
	if t.SilentDays > limit {
		// A thread that has already returned from a gap this long is between
		// seasons, not over. Calling that an ending would ask a person to
		// explain the end of something that has not ended, which is worse than
		// not asking: it asserts a false fact about their life.
		if t.MaxGap > 0 && t.SilentDays <= int(float64(t.MaxGap)*opts.DormantTolerance) {
			return Dormant
		}
		return Ended
	}
	if opts.EmergingDays > 0 && !opts.Now.IsZero() && daysBetween(t.First, opts.Now) <= opts.EmergingDays {
		return Emerging
	}
	return Ongoing
}

// medianGapDays returns the typical number of days between sorted occurrences,
// or zero when there are fewer than two.
func medianGapDays(times []time.Time) int {
	if len(times) < 2 {
		return 0
	}
	gaps := make([]int, 0, len(times)-1)
	for i := 1; i < len(times); i++ {
		gaps = append(gaps, daysBetween(times[i-1], times[i]))
	}
	sort.Ints(gaps)
	mid := len(gaps) / 2
	if len(gaps)%2 == 1 {
		return gaps[mid]
	}
	return (gaps[mid-1] + gaps[mid]) / 2
}

// gapSpread returns the coefficient of variation of the spans between
// consecutive occurrences, or zero when there are fewer than three.
func gapSpread(times []time.Time) float64 {
	if len(times) < 3 {
		return 0
	}
	gaps := make([]float64, 0, len(times)-1)
	mean := 0.0
	for i := 1; i < len(times); i++ {
		g := float64(daysBetween(times[i-1], times[i]))
		gaps = append(gaps, g)
		mean += g
	}
	mean /= float64(len(gaps))
	if mean <= 0 {
		return 0
	}
	varsum := 0.0
	for _, g := range gaps {
		d := g - mean
		varsum += d * d
	}
	return math.Sqrt(varsum/float64(len(gaps))) / mean
}

// maxGapDays returns the longest span between consecutive occurrences.
func maxGapDays(times []time.Time) int {
	longest := 0
	for i := 1; i < len(times); i++ {
		if g := daysBetween(times[i-1], times[i]); g > longest {
			longest = g
		}
	}
	return longest
}

// daysBetween returns whole days from a to b, never negative.
func daysBetween(a, b time.Time) int {
	d := int(b.Sub(a).Hours() / 24)
	if d < 0 {
		return 0
	}
	return d
}

// Title normalization. Calendar entries carry the same commitment written many
// ways across years, so grouping has to survive case, emoji, punctuation, and
// separator drift without merging genuinely different activities.
var (
	// parenSuffix strips the time or all-day marker the ingest appends.
	parenSuffix = regexp.MustCompile(`\s*\((?:all day|\d{2}:\d{2} to \d{2}:\d{2})\)\s*$`)
	// nonTitle drops anything that is not a letter, digit, space, or hyphen.
	nonTitle = regexp.MustCompile(`[^\p{L}\p{N}\s-]+`)
	// bareNumber drops standalone numbers such as a field or grade number.
	bareNumber = regexp.MustCompile(`\b\d+\b`)
	// spaces collapses runs of whitespace.
	spaces = regexp.MustCompile(`\s+`)
	// separator normalizes the dash or colon between a name and an activity.
	separator = regexp.MustCompile(`\s*[-:]\s*`)
)

// filler holds the words that carry no identity in a calendar title. People
// write the same commitment differently every time, so these are dropped before
// grouping.
var filler = map[string]bool{
	"a": true, "an": true, "and": true, "at": true, "for": true, "from": true,
	"in": true, "my": true, "of": true, "on": true, "our": true, "the": true,
	"to": true, "w": true, "with": true, "his": true, "her": true, "their": true,
	"s": true, "is": true, "be": true, "am": true, "pm": true,
}

// possessive strips a trailing possessive so a place named for a person matches
// the person.
var possessive = regexp.MustCompile(`(\p{L})['\x{2019}]s\b`)

// NormalizeTitle reduces an entry body to the key its thread is grouped by.
//
// The key is a sorted set of meaningful words rather than the title itself,
// because a handwritten calendar records one commitment under many spellings.
// "sleepover with Kayla", "sleepover @ Kayla's", and "sleepover w/ Kayla" are
// the same standing arrangement, and grouping on the exact string splits them
// into separate threads that each appear to end the moment the wording changes.
// Collapsing to a word set makes the phrasing irrelevant and the subject decisive.
func NormalizeTitle(body string) string {
	t := strings.ToLower(headline(body))
	t = parenSuffix.ReplaceAllString(t, "")
	t = possessive.ReplaceAllString(t, "$1")
	t = nonTitle.ReplaceAllString(t, " ")
	t = separator.ReplaceAllString(t, " ")
	t = bareNumber.ReplaceAllString(t, " ")
	t = spaces.ReplaceAllString(t, " ")

	seen := map[string]bool{}
	var words []string
	for _, w := range strings.Fields(t) {
		w = strings.Trim(w, "-")
		if len(w) < 2 || filler[w] || seen[w] {
			continue
		}
		seen[w] = true
		words = append(words, w)
	}
	if len(words) == 0 {
		return ""
	}
	sort.Strings(words)
	return strings.Join(words, " ")
}

// headline returns the first non-empty line of an entry body.
func headline(body string) string {
	for line := range strings.SplitSeq(body, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			return parenSuffix.ReplaceAllString(s, "")
		}
	}
	return ""
}

// mergeSimilar folds together groups whose word sets overlap enough to be the
// same thread. Occurrences are merged into the largest group, which also keeps
// the most representative label.
func mergeSimilar(
	grouped map[string][]time.Time,
	labels map[string]string,
	variants map[string]map[string]bool,
	threshold float64,
) (map[string][]time.Time, map[string]string, map[string]map[string]bool) {
	if threshold <= 0 || threshold > 1 {
		return grouped, labels, variants
	}
	keys := make([]string, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	// Largest first, so smaller variants fold into the dominant spelling.
	sort.Slice(keys, func(i, j int) bool {
		if len(grouped[keys[i]]) != len(grouped[keys[j]]) {
			return len(grouped[keys[i]]) > len(grouped[keys[j]])
		}
		return keys[i] < keys[j]
	})
	sets := make([]map[string]bool, len(keys))
	for i, k := range keys {
		sets[i] = wordSet(k)
	}
	outTimes := map[string][]time.Time{}
	outLabels := map[string]string{}
	outVariants := map[string]map[string]bool{}
	canonical := make([]int, 0, len(keys))
	for i, k := range keys {
		target := -1
		for _, c := range canonical {
			if jaccard(sets[i], sets[c]) >= threshold && !addsSubject(sets[i], sets[c]) {
				target = c
				break
			}
		}
		if target < 0 {
			canonical = append(canonical, i)
			outTimes[k] = append(outTimes[k], grouped[k]...)
			outLabels[k] = labels[k]
			outVariants[k] = copySet(variants[k])
			continue
		}
		ck := keys[target]
		outTimes[ck] = append(outTimes[ck], grouped[k]...)
		if outVariants[ck] == nil {
			outVariants[ck] = map[string]bool{}
		}
		for v := range variants[k] {
			outVariants[ck][v] = true
		}
	}
	return outTimes, outLabels, outVariants
}

// copySet duplicates a set so later merges cannot mutate the original.
func copySet(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k := range in {
		out[k] = true
	}
	return out
}

// sortedSet renders a set as a stable ordered slice.
func sortedSet(in map[string]bool) []string {
	out := make([]string, 0, len(in))
	for k := range in {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// addsSubject reports whether merging two word sets would absorb a title that
// names nobody into one that names someone.
//
// "Doctor appt" and "Hannah doctor appt" overlap heavily, but the second says
// whose appointment it was and the first does not. Folding them together treats
// two people's appointments as one thread, which then reports a gap spanning the
// distance between two unrelated lives. A short title has no room for a spare
// word, so a word added to one is a subject rather than noise; a longer title
// can absorb one without changing what it is about.
func addsSubject(a, b map[string]bool) bool {
	shorter, longer := a, b
	if len(b) < len(a) {
		shorter, longer = b, a
	}
	if len(shorter) >= minTokensForNoise {
		return false
	}
	for w := range shorter {
		if !longer[w] {
			// Not a subset: the two differ in both directions, so neither is a
			// bare version of the other.
			return false
		}
	}
	return len(longer) > len(shorter)
}

// minTokensForNoise is how many words a title needs before an extra one can be
// treated as incidental rather than as the subject.
const minTokensForNoise = 3

// wordSet splits a normalized key back into its words.
func wordSet(key string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(key) {
		out[w] = true
	}
	return out
}

// jaccard returns the overlap of two word sets as intersection over union.
func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for w := range a {
		if b[w] {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}
