package cmd

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dcadolph/midden/internal/jsonutil"
	"github.com/dcadolph/midden/internal/vault"
)

// Timestamp layouts shared by every subcommand's output.
const (
	// layoutDate renders a date as YYYY-MM-DD.
	layoutDate = "2006-01-02"
	// layoutDateTime renders a full local timestamp.
	layoutDateTime = "2006-01-02 15:04:05"
)

// printEntries renders the entries to the writer either as JSON or as a human-readable form.
// Each text entry begins with its full local timestamp and optional tag list followed by the body.
func printEntries(w io.Writer, entries []vault.Entry) error {
	if jsonOutput {
		return jsonutil.Encode(w, entriesToJSON(entries), jsonPretty)
	}
	color := isTerminal(w)
	for _, e := range entries {
		writeEntryText(w, e, color)
	}
	return nil
}

// writeEntryText renders one entry as paragraphs to the writer.
// When color is true the timestamp and tag list are colorized with ANSI escapes.
func writeEntryText(w io.Writer, e vault.Entry, color bool) {
	ts := e.Time.Format(layoutDateTime)
	if color {
		fmt.Fprint(w, colorCyan, ts, colorReset)
	} else {
		fmt.Fprint(w, ts)
	}
	if len(e.Tags) > 0 {
		tagList := "  [" + strings.Join(e.Tags, ", ") + "]"
		if color {
			fmt.Fprint(w, colorYellow, tagList, colorReset)
		} else {
			fmt.Fprint(w, tagList)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, e.Body)
	fmt.Fprintln(w)
}

// entriesToJSON converts entries to a JSON-friendly shape with explicit fields.
func entriesToJSON(entries []vault.Entry) []entryJSON {
	out := make([]entryJSON, len(entries))
	for i, e := range entries {
		out[i] = entryJSON{
			Time: e.Time.Format(time.RFC3339),
			Tags: e.Tags,
			Body: e.Body,
		}
	}
	return out
}

// entryJSON is the wire shape used for JSON output of an entry.
type entryJSON struct {
	// Time is the RFC 3339 local timestamp of the entry.
	Time string `json:"time"`
	// Tags are the entry tags without leading hash characters.
	Tags []string `json:"tags,omitempty"`
	// Body is the entry body.
	Body string `json:"body"`
}
