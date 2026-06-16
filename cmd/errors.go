package cmd

import "errors"

// Sentinel errors used to map failures to exit codes.
var (
	// ErrVault wraps vault read or write failures.
	ErrVault = errors.New("vault operation failed")
	// ErrNotFound wraps lookups that returned no results.
	ErrNotFound = errors.New("not found")
	// ErrEditor wraps editor invocation failures.
	ErrEditor = errors.New("editor failed")
)

// errorCode maps a known error to its process exit code.
func errorCode(err error) (int, bool) {
	switch {
	case errors.Is(err, ErrVault):
		return ExitVault, true
	case errors.Is(err, ErrNotFound):
		return ExitNotFound, true
	case errors.Is(err, ErrEditor):
		return ExitEditor, true
	}
	return 0, false
}
