// Package cli wires rig's command tree. Commands here are thin presentation:
// they parse flags, prompt for missing values, and delegate to the pure core
// and the IO interfaces.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is overwritten at build time via -ldflags. The git tag is the source
// of truth; this is only a fallback for `go run`/dev builds.
var version = "dev"

func newRootCmd(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:           "rig",
		Short:         "Stand up, authenticate, navigate, and manage local coding projects",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.SetVersionTemplate("rig {{.Version}}\n")

	// Built-in command groups. These always win over launchers.
	root.AddCommand(newSelfCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newPathCmd())
	root.AddCommand(newCdCmd())
	root.AddCommand(newShellInitCmd())

	// Config-defined launchers register last and may not shadow a built-in.
	registerLaunchers(root, app)

	return root
}

// Execute builds the app and runs the root command, returning a process exit
// code.
func Execute() int {
	app, err := newApp()
	if err != nil {
		fmt.Fprintln(os.Stderr, "rig: "+err.Error())
		return 1
	}
	root := newRootCmd(app)
	ctx := withApp(context.Background(), app)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "rig: "+err.Error())
		return 1
	}
	return 0
}
