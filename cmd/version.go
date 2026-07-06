package cmd

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// Version is the midden release version. Goreleaser injects it at build time;
// builds from go install fall back to the module version in build info.
var Version = ""

// versionString resolves the version from the ldflags value, then module
// build info, then a dev placeholder.
func versionString() string {
	if Version != "" {
		return Version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return strings.TrimPrefix(bi.Main.Version, "v")
	}
	return "dev"
}

// versionCmd prints the build version.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the midden version.",
	Run: func(cmd *cobra.Command, _ []string) {
		fmt.Fprintln(cmd.OutOrStdout(), versionString())
	},
}

func init() {
	rootCmd.Version = versionString()
	rootCmd.AddCommand(versionCmd)
}
