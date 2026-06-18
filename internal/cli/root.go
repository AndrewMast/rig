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

	// Build the shared app context once, before any command runs.
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		cmd.SetContext(withApp(cmd.Context(), app))
		return nil
	}

	// Command groups are registered here as they come online.
	root.AddCommand(newSelfCmd())
	root.AddCommand(newConfigCmd())

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
