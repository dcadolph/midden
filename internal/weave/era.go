package weave

import (
	"math"
	"sort"
	"time"

	"github.com/dcadolph/midden/internal/vault"
)

// Era is a stretch of the record with a consistent level of activity. Silences
// find where a record went quiet; eras find the larger structure those
// silences sit inside: the sparse early years, the dense family years, the
// work explosion. A person lives one day at a time and cannot see these
// boundaries from inside them.
type Era struct {
	// From is the first month of the era.
	From time.Time
	// To is the last day of the era's final month.
	To time.Time
	// Months is the era's length.
	Months int
	// Entries is how many entries fall inside it.
	Entries int
	// PerMonth is the era's average monthly volume.
	PerMonth float64
}

// EraOptions tune segmentation.
type EraOptions struct {
	// Now is the observation date; later months are ignored.
	Now time.Time
	// MinMonths is the shortest era worth reporting.
	MinMonths int
	// MinGain is the fraction of a segment's variance a split must remove to be
	// accepted. Splitting is greedy and recursive, so this is the brake that
	// stops the record shattering into micro-eras.
	MinGain float64
	// MaxEras caps how many eras the record may be cut into.
	MaxEras int
}

// DefaultEraOptions returns segmentation settings suited to a personal record.
func DefaultEraOptions(now time.Time) EraOptions {
	return EraOptions{Now: now, MinMonths: 6, MinGain: 0.15, MaxEras: 8}
}

// Eras segments the record into stretches of consistent activity, in
// chronological order.
//
// The method is binary segmentation over log-scaled monthly counts: find the
// split that removes the most variance, keep it if it removes enough, recurse
// into both halves. Log scaling matters because a personal record's volume
// spans orders of magnitude, and on a linear scale the difference between 400
// and 100 entries a month would dwarf the difference between 4 and 0, which is
// the difference that actually separates a life's chapters.
func Eras(entries []vault.Entry, opts EraOptions) []Era {
	if opts.MinMonths < 2 {
		opts.MinMonths = 2
	}
	if opts.MaxEras < 1 {
		opts.MaxEras = 1
	}
	counts, first, last := monthCounts(entries, opts.Now)
	if len(counts) == 0 || !first.Before(last) {
		return nil
	}
	series := monthSeries(first, last)
	if len(series) < 2*opts.MinMonths {
		return nil
	}
	x := make([]float64, len(series))
	for i, m := range series {
		x[i] = math.Log1p(float64(counts[key(m)]))
	}

	bounds := []int{0, len(x)}
	for len(bounds)-1 < opts.MaxEras {
		bestGain, bestAt := opts.MinGain, -1
		for s := 0; s < len(bounds)-1; s++ {
			lo, hi := bounds[s], bounds[s+1]
			gain, at := bestSplit(x[lo:hi], opts.MinMonths)
			if at >= 0 && gain >= bestGain {
				bestGain, bestAt = gain, lo+at
			}
		}
		if bestAt < 0 {
			break
		}
		bounds = append(bounds, bestAt)
		sort.Ints(bounds)
	}

	out := make([]Era, 0, len(bounds)-1)
	for s := 0; s < len(bounds)-1; s++ {
		lo, hi := bounds[s], bounds[s+1]
		total := 0
		for i := lo; i < hi; i++ {
			total += counts[key(series[i])]
		}
		out = append(out, Era{
			From:     series[lo],
			To:       series[hi-1].AddDate(0, 1, -1),
			Months:   hi - lo,
			Entries:  total,
			PerMonth: float64(total) / float64(hi-lo),
		})
	}
	return out
}

// bestSplit returns the largest acceptable variance reduction from splitting
// the segment, and where, or -1 when no split clears the bar. The gain is
// measured as a fraction of the segment's own variance, so a flat segment is
// never split just because it is long.
func bestSplit(x []float64, minLen int) (float64, int) {
	n := len(x)
	if n < 2*minLen {
		return 0, -1
	}
	total := sse(x)
	if total <= 1e-9 {
		return 0, -1
	}
	bestGain, bestAt := 0.0, -1
	for at := minLen; at <= n-minLen; at++ {
		gain := (total - sse(x[:at]) - sse(x[at:])) / total
		if gain > bestGain {
			bestGain, bestAt = gain, at
		}
	}
	return bestGain, bestAt
}

// sse returns the sum of squared deviations from the mean.
func sse(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}
	mean := 0.0
	for _, v := range x {
		mean += v
	}
	mean /= float64(len(x))
	out := 0.0
	for _, v := range x {
		d := v - mean
		out += d * d
	}
	return out
}
