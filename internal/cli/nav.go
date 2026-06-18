package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path [token]",
		Short: "Print the absolute path a token resolves to",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			token, err := navToken(app, args)
			if err != nil {
				return err
			}
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			t, err := app.resolveTarget(reg, token, true)
			if err != nil {
				return err
			}
			cmd.Println(t.Path)
			return nil
		},
	}
}

func newCdCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cd [token]",
		Short: "Resolve a token for the shell wrapper to cd into",
		Long: "Prints the resolved path. A directory change must happen in the parent " +
			"shell, so install the shell integration (rig shell-init) for `rig cd` to " +
			"actually change directory.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			token, err := navToken(app, args)
			if err != nil {
				return err
			}
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			t, err := app.resolveTarget(reg, token, true)
			if err != nil {
				return err
			}
			cmd.Println(t.Path)
			if app.UI.Interactive {
				fmt.Fprintln(os.Stderr, "rig: not changing dir (no shell integration). Run `rig shell-init` to enable `rig cd`.")
			}
			return nil
		},
	}
}

// navToken returns the explicit token, or resolves the cwd / picker when none
// was given.
func navToken(app *App, args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	// Zero-arg: prefer the project containing the cwd, else prompt a picker.
	if tok, ok := projectTokenForCwd(app); ok {
		return tok, nil
	}
	return app.pickProjectToken("Go to project:")
}
