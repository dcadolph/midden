package weave

import (
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dcadolph/midden/internal/vault"
)

// Person is someone the record keeps mentioning. Names are the most important
// entities in a personal record, and they exist only inside entry headlines
// until something counts them. Everything here is arithmetic over
// capitalization and possessives; nothing is inferred, so a person listed is a
// person the record literally names.
type Person struct {
	// Name is the person's name as most commonly written.
	Name string
	// Mentions is how many entries name them.
	Mentions int
	// First and Last are the earliest and latest mention.
	First time.Time
	Last  time.Time
	// SilentDays is how long since the last mention at the observation date.
	SilentDays int
	// Contexts are the most frequent headlines they appear in, most common
	// first, capped for display.
	Contexts []string
}

// PeopleOptions tune extraction.
type PeopleOptions struct {
	// Now is the observation date; later entries are ignored.
	Now time.Time
	// MinMentions is the fewest mentions a name needs to be listed.
	MinMentions int
	// MaxContexts caps the example headlines kept per person.
	MaxContexts int
	// Exclude drops names the caller knows are not people, matched
	// case-insensitively. Weekday, month, and common calendar words are always
	// excluded.
	Exclude []string
}

// DefaultPeopleOptions returns extraction settings suited to a personal record.
func DefaultPeopleOptions(now time.Time) PeopleOptions {
	return PeopleOptions{Now: now, MinMentions: 3, MaxContexts: 3}
}

// notNames are capitalized words that are never people: calendar vocabulary,
// weekdays, months, and title-case noise that survives tokenization.
var notNames = map[string]bool{
	"monday": true, "tuesday": true, "wednesday": true, "thursday": true,
	"friday": true, "saturday": true, "sunday": true,
	"january": true, "february": true, "march": true, "april": true, "may": true,
	"june": true, "july": true, "august": true, "september": true,
	"october": true, "november": true, "december": true,
	"mom": true, "dad": true, "mama": true, "papa": true, "hubby": true,
	"school": true, "no": true, "the": true, "a": true, "an": true, "and": true,
	"appointment": true, "appt": true, "birthday": true, "party": true,
	"doctor": true, "dr": true, "dentist": true,
	"practice": true, "game": true, "class": true, "dance": true, "piano": true,
	"early": true, "release": true, "day": true, "last": true, "first": true,
	"invitation": true, "free": true, "call": true, "get": true, "go": true,
	"pick": true, "drop": true, "bring": true, "renew": true, "finish": true,
	"repo": true, "author": true, "location": true, "must": true, "do": true,
	"new": true, "all": true, "spring": true, "summer": true, "fall": true,
	"winter": true, "christmas": true, "halloween": true, "thanksgiving": true,
	"easter": true, "holiday": true, "vacation": true, "week": true,
}

// nameToken matches a capitalized word, optionally possessive, inside a
// headline. Unicode letters are allowed so accented names survive.
var nameToken = regexp.MustCompile(`\b\p{Lu}\p{Ll}+(?:['\x{2019}]s)?\b`)

// possessiveSuffix strips a trailing possessive from a matched token.
var possessiveSuffix = regexp.MustCompile(`['\x{2019}]s$`)

// personEvidence matches the grammar that only people attract: a possessive
// ("William's birthday", "Jax's grooming") or a companion preposition
// ("sleepover with Kayla", "w/ Sara", "@ Kayla"). Capitalization alone cannot
// tell a person from a commit verb, because commit subjects capitalize their
// first word and calendar titles capitalize freely; but nobody ever writes
// "Add's" or "dinner with Curbside". Blocklists rot; grammar does not.
var personEvidence = regexp.MustCompile(
	`(\p{Lu}\p{Ll}+)['\x{2019}]s\b|(?:\bwith |\bw/ ?|@ )(\p{Lu}\p{Ll}+)`)

// People extracts the people the record names, ordered by mentions then
// recency. Only entry headlines are scanned, because bodies of imported
// entries carry boilerplate (repository names, locations) that would flood
// the counts with non-people.
func People(entries []vault.Entry, opts PeopleOptions) []Person {
	if opts.MinMentions < 1 {
		opts.MinMentions = 1
	}
	excluded := map[string]bool{}
	for _, e := range opts.Exclude {
		excluded[strings.ToLower(e)] = true
	}

	type acc struct {
		mentions  int
		evidence  int
		first     time.Time
		last      time.Time
		spellings map[string]int
		contexts  map[string]int
	}
	byKey := map[string]*acc{}

	for _, e := range entries {
		if !opts.Now.IsZero() && e.Time.After(opts.Now) {
			continue
		}
		head := headline(e.Body)

		// First: which tokens in this headline carry person-grammar.
		evident := map[string]bool{}
		for _, m := range personEvidence.FindAllStringSubmatch(head, -1) {
			for _, g := range m[1:] {
				if g != "" {
					evident[strings.ToLower(g)] = true
				}
			}
		}

		seen := map[string]bool{}
		for _, raw := range nameToken.FindAllString(head, -1) {
			name := possessiveSuffix.ReplaceAllString(raw, "")
			lower := strings.ToLower(name)
			if notNames[lower] || excluded[lower] || len(name) < 3 || seen[lower] {
				continue
			}
			seen[lower] = true
			a := byKey[lower]
			if a == nil {
				a = &acc{spellings: map[string]int{}, contexts: map[string]int{}}
				byKey[lower] = a
			}
			a.mentions++
			if evident[lower] {
				a.evidence++
			}
			a.spellings[name]++
			a.contexts[head]++
			if a.first.IsZero() || e.Time.Before(a.first) {
				a.first = e.Time
			}
			if e.Time.After(a.last) {
				a.last = e.Time
			}
		}
	}

	out := make([]Person, 0, len(byKey))
	for _, a := range byKey {
		if a.mentions < opts.MinMentions {
			continue
		}
		// No person-grammar anywhere across every mention means the token is a
		// verb, a place, or title-case noise, no matter how often it recurs.
		if a.evidence == 0 {
			continue
		}
		p := Person{
			Name:     topKey(a.spellings),
			Mentions: a.mentions,
			First:    a.first,
			Last:     a.last,
			Contexts: topKeys(a.contexts, opts.MaxContexts),
		}
		if !opts.Now.IsZero() {
			p.SilentDays = daysBetween(a.last, opts.Now)
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Mentions != out[j].Mentions {
			return out[i].Mentions > out[j].Mentions
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// topKey returns the most frequent key in the map.
func topKey(m map[string]int) string {
	best, bestN := "", -1
	for k, n := range m {
		if n > bestN || (n == bestN && k < best) {
			best, bestN = k, n
		}
	}
	return best
}

// topKeys returns the n most frequent keys, most frequent first.
func topKeys(m map[string]int, n int) []string {
	type kv struct {
		k string
		n int
	}
	all := make([]kv, 0, len(m))
	for k, v := range m {
		all = append(all, kv{k, v})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].n != all[j].n {
			return all[i].n > all[j].n
		}
		return all[i].k < all[j].k
	})
	if n > 0 && len(all) > n {
		all = all[:n]
	}
	out := make([]string, len(all))
	for i, e := range all {
		out[i] = e.k
	}
	return out
}
