package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/AndrewMast/rig/internal/config"
	"github.com/spf13/cobra"
)

// registerLaunchers adds one subcommand per config-defined launcher. A launcher
// may never shadow a built-in command: collisions are warned about and skipped.
func registerLaunchers(root *cobra.Command, app *App) {
	builtins := map[string]bool{}
	for _, c := range root.Commands() {
		builtins[c.Name()] = true
	}
	for _, name := range app.Config.LauncherNames() {
		if builtins[name] {
			fmt.Fprintf(os.Stderr, "rig: launcher %q ignored (shadows a built-in command)\n", name)
			continue
		}
		root.AddCommand(newLauncherCmd(name, app.Config.Launchers[name]))
	}
}

func newLauncherCmd(name string, l config.Launcher) *cobra.Command {
	return &cobra.Command{
		Use:   name + " [token]",
		Short: fmt.Sprintf("Run the %q launcher (%s)", name, l.Command),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			allowGroup := l.Target == config.TargetFolder

			token, err := launcherToken(app, args)
			if err != nil {
				return err
			}
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			t, err := app.resolveTarget(reg, token, allowGroup)
			if err != nil {
				return err
			}
			return runLauncher(l, t.Path)
		},
	}
}

// launcherToken mirrors navToken: explicit arg, else cwd project, else picker.
func launcherToken(app *App, args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	if tok, ok := projectTokenForCwd(app); ok {
		return tok, nil
	}
	return app.pickProjectToken("Choose a project:")
}

// runLauncher expands {path} in the command template and runs it via the shell.
func runLauncher(l config.Launcher, path string) error {
	line := strings.ReplaceAll(l.Command, "{path}", shellQuote(path))
	c := exec.Command("sh", "-c", line)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("launcher failed: %w", err)
	}
	return nil
}

// shellQuote single-quotes a string for safe interpolation into an sh command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
