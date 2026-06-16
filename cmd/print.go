package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/dcadolph/midden/internal/vault"
)

// printEntries renders the entries to the writer in a human-readable form.
// Each entry begins with its full local timestamp and optional tag list,
// followed by the body and a trailing blank line.
func printEntries(w io.Writer, entries []vault.Entry) {
	for _, e := range entries {
		header := e.Time.Format("2006-01-02 15:04:05")
		if len(e.Tags) > 0 {
			header += "  [" + strings.Join(e.Tags, ", ") + "]"
		}
		fmt.Fprintln(w, header)
		fmt.Fprintln(w, e.Body)
		fmt.Fprintln(w)
	}
}
