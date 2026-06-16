// Package main is the entry point for the midden CLI.
package main

import (
	"os"

	"github.com/dcadolph/midden/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
