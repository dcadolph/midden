// Package cmd wires the midden CLI subcommands.
package cmd

import (
	"fmt"
	"os"
)

// Execute runs the root command and returns the process exit code.
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if code, ok := errorCode(err); ok {
			return code
		}
		return ExitGeneric
	}
	return ExitOK
}
