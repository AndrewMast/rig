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
		Args:          cobra.ArbitraryArgs,
		// Enforce the optional [guard] before any command runs.
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return appFrom(cmd).checkGuard(cmd)
		},
		// Lowest precedence in the command ladder (built-in > launcher >
		// type/project command > fuzzy nav): a bare token that matches no
		// command is resolved as a navigation target.
		RunE: runFuzzyNav,
	}
	root.SetVersionTemplate("rig {{.Version}}\n")
	// cobra's cmd.Print* family defaults to stderr; route it to stdout so
	// scriptable output (rig path, shell-init, list, …) is capturable via
	// $(...). Errors are written explicitly to os.Stderr below.
	root.SetOut(os.Stdout)

	// Built-in command groups. These always win over launchers.
	root.AddCommand(newSelfCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newPathCmd())
	root.AddCommand(newCdCmd())
	root.AddCommand(newShellInitCmd())
	root.AddCommand(newGroupCmd())
	root.AddCommand(newAliasCmd())
	root.AddCommand(newCreateCmd())
	root.AddCommand(newAdoptCmd())
	root.AddCommand(newCloneCmd())
	root.AddCommand(newListCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newProjectCmd())
	root.AddCommand(newKeyCmd())
	root.AddCommand(newTypeCmd())

	// Config-defined launchers register next and may not shadow a built-in.
	registerLaunchers(root, app)
	// The cwd project's type/rig.toml commands register last (lowest priority).
	registerProjectCommands(root, app)

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
