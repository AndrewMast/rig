// Package cli wires rig's command tree. Commands here are thin presentation:
// they parse flags, prompt for missing values, and delegate to the pure core
// and the IO interfaces.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is overwritten at build time via -ldflags. The git tag is the source
// of truth; this is only a fallback for `go run`/dev builds.
var version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "rig",
		Short:         "Stand up, authenticate, navigate, and manage local coding projects",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.SetVersionTemplate("rig {{.Version}}\n")

	// Command groups are registered here as they come online.
	root.AddCommand(newSelfCmd())

	return root
}

// Execute runs the root command and returns a process exit code.
func Execute() int {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "rig: "+err.Error())
		return 1
	}
	return 0
}
