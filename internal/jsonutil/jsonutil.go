// Package jsonutil centralizes JSON encoding for midden subcommands.
package jsonutil

import (
	"encoding/json"
	"fmt"
	"io"
)

// Encode writes v to w as JSON.
// Pretty controls indented output; compact JSON ends with a single trailing newline.
func Encode(w io.Writer, v any, pretty bool) error {
	enc := json.NewEncoder(w)
	if pretty {
		enc.SetIndent("", "  ")
	}
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}
