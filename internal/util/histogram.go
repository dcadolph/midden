package util

import "sort"

// TagCount pairs a label with the number of entries carrying it.
type TagCount struct {
	// Tag is the label without a leading hash character.
	Tag string `json:"tag"`
	// Count is the number of entries carrying the label.
	Count int `json:"count"`
}

// SortedCounts renders a label histogram as a slice ordered by descending count
// then ascending label. A non-positive limit returns every label.
func SortedCounts(counts map[string]int, limit int) []TagCount {
	out := make([]TagCount, 0, len(counts))
	for tag, n := range counts {
		out = append(out, TagCount{Tag: tag, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Tag < out[j].Tag
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
