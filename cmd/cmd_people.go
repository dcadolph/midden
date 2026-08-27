package cmd

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/dcadolph/midden/internal/jsonutil"
	"github.com/dcadolph/midden/internal/weave"
)

// People options.
var (
	peopleSince   string
	peopleUntil   string
	peopleMin     int
	peopleLimit   int
	peopleExclude []string
)

// peopleCmd lists the people the record names.
var peopleCmd = &cobra.Command{
	Use:   "people",
	Short: "List the people your record mentions, by how often and how recently.",
	Long: "People counts every capitalized name in entry headlines and reports who recurs.\n\n" +
		"Names are the most important entities in a personal record and nothing else surfaces them " +
		"directly. Everything here is counted rather than inferred: a listed person is a person the " +
		"record literally names, and a long silence next to a name that once recurred is exactly the " +
		"kind of fact a person cannot see from inside their own life.",
	RunE: runPeople,
}

func init() {
	peopleCmd.Flags().StringVar(&peopleSince, "since", "", "Only consider entries on or after this date.")
	peopleCmd.Flags().StringVar(&peopleUntil, "until", "", "Only consider entries on or before this date.")
	peopleCmd.Flags().IntVar(&peopleMin, "min", 3, "Fewest mentions a name needs to be listed.")
	peopleCmd.Flags().IntVar(&peopleLimit, "top", 25, "Maximum people to show.")
	peopleCmd.Flags().StringSliceVar(&peopleExclude, "exclude", nil,
		"Names to drop from the list, for words the record capitalizes that are not people.")
	rootCmd.AddCommand(peopleCmd)
}

// runPeople extracts and prints the people index.
func runPeople(cmd *cobra.Command, _ []string) error {
	span, err := resolveDateRange(peopleSince, peopleUntil)
	if err != nil {
		return err
	}
	v, err := openVault()
	if err != nil {
		return err
	}
	entries, err := entriesInRange(v, span)
	if err != nil {
		return errors.Join(ErrVault, fmt.Errorf("scan vault: %w", err))
	}
	if len(entries) == 0 {
		return errors.Join(ErrNotFound, fmt.Errorf("no entries in range (%s)", span.Label()))
	}
	opts := weave.DefaultPeopleOptions(time.Now())
	opts.MinMentions = peopleMin
	opts.Exclude = peopleExclude
	people := weave.People(entries, opts)
	if len(people) == 0 {
		return errors.Join(ErrNotFound, errors.New("no recurring names found"))
	}
	if peopleLimit > 0 && len(people) > peopleLimit {
		people = people[:peopleLimit]
	}
	if jsonOutput {
		return jsonutil.Encode(cmd.OutOrStdout(), peopleToJSON(people), jsonPretty)
	}
	writePeople(cmd.OutOrStdout(), people)
	return nil
}

// writePeople renders the index, flagging names that have gone quiet relative
// to how long they were around.
func writePeople(w io.Writer, people []weave.Person) {
	for _, p := range people {
		span := daysBetweenLabel(p.First, p.Last)
		note := ""
		// A name that recurred for a long stretch and then vanished for over a
		// year is a faded relationship, which is worth saying out loud.
		if p.SilentDays > 365 && p.Last.Sub(p.First) > 180*24*time.Hour {
			note = fmt.Sprintf("   <- last seen %s", p.Last.Format(layoutDate))
		}
		fmt.Fprintf(w, "  %-16s %4dx over %-9s%s\n", p.Name, p.Mentions, span, note)
		for _, c := range p.Contexts {
			fmt.Fprintf(w, "      · %s\n", truncate(c, 64))
		}
	}
}

// daysBetweenLabel renders a span between two dates as years or months.
func daysBetweenLabel(a, b time.Time) string {
	days := int(b.Sub(a).Hours() / 24)
	return years(days)
}

// personJSON is the wire shape of one person.
type personJSON struct {
	// Name is the most common spelling.
	Name string `json:"name"`
	// Mentions is how many entries name them.
	Mentions int `json:"mentions"`
	// First and Last are the bounding mention dates.
	First string `json:"first"`
	Last  string `json:"last"`
	// SilentDays is how long since the last mention.
	SilentDays int `json:"silent_days"`
	// Contexts are representative headlines.
	Contexts []string `json:"contexts,omitempty"`
}

// peopleToJSON converts people to their wire shape.
func peopleToJSON(people []weave.Person) []personJSON {
	out := make([]personJSON, len(people))
	for i, p := range people {
		out[i] = personJSON{
			Name: p.Name, Mentions: p.Mentions,
			First: p.First.Format(layoutDate), Last: p.Last.Format(layoutDate),
			SilentDays: p.SilentDays, Contexts: p.Contexts,
		}
	}
	return out
}
