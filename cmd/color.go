package cmd

import (
	"io"
	"os"

	"golang.org/x/term"
)

// ANSI color escape sequences used when output is going to an interactive terminal.
const (
	colorReset  = "\x1b[0m"
	colorDim    = "\x1b[2m"
	colorBold   = "\x1b[1m"
	colorCyan   = "\x1b[36m"
	colorYellow = "\x1b[33m"
	colorGreen  = "\x1b[32m"
)

// isTerminal reports whether the writer is an interactive terminal and color output is desired.
// It also honors the NO_COLOR convention and the global --no-color flag.
func isTerminal(w io.Writer) bool {
	if noColor {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
