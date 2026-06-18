package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/AndrewMast/rig/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and edit rig configuration",
	}
	cmd.AddCommand(
		newConfigShowCmd(),
		newConfigGetCmd(),
		newConfigSetCmd(),
		newConfigEditCmd(),
		newConfigTokenCmd(),
	)
	return cmd
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the effective config (token redacted)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app := appFrom(cmd)
			cmd.Printf("# %s\n", app.Paths.Config)
			cmd.Printf("default_base = %q\n\n", app.Config.DefaultBase)
			cmd.Printf("[handoff]\nmethod = %q\nalways_confirm = %v\n\n",
				app.Config.Handoff.Method, app.Config.Handoff.AlwaysConfirm)
			cmd.Printf("[github]\ntoken_file = %q  # %s\n\n",
				app.Config.GitHub.TokenFile, tokenStatusWord(app))
			if u := app.Config.Guard.ExpectedUser; u != "" {
				cmd.Printf("[guard]\nexpected_user = %q\nexpected_host = %q\n\n",
					u, app.Config.Guard.ExpectedHost)
			}
			for _, name := range app.Config.LauncherNames() {
				l := app.Config.Launchers[name]
				target := l.Target
				if target == "" {
					target = config.TargetProject
				}
				cmd.Printf("[launchers.%s]\ncommand = %q\ntarget = %q\n", name, l.Command, target)
			}
			return nil
		},
	}
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Print a single config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			val, err := app.Config.Get(args[0])
			if err != nil {
				return err
			}
			cmd.Println(val)
			return nil
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Validate and write a single config key",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			if err := app.Config.Set(args[0], args[1]); err != nil {
				return err
			}
			if err := config.Save(app.Paths, app.Config); err != nil {
				return err
			}
			cmd.Printf("set %s = %s\n", args[0], args[1])
			return nil
		},
	}
}

func newConfigEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open config.toml in $EDITOR, re-validated on save",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app := appFrom(cmd)
			// Ensure the file exists so the editor opens something sensible.
			if _, err := os.Stat(app.Paths.Config); os.IsNotExist(err) {
				if err := config.Save(app.Paths, app.Config); err != nil {
					return err
				}
			}
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			ed := exec.Command(editor, app.Paths.Config)
			ed.Stdin, ed.Stdout, ed.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := ed.Run(); err != nil {
				return fmt.Errorf("editor: %w", err)
			}
			// Re-validate by loading.
			if _, err := config.Load(app.Paths); err != nil {
				return fmt.Errorf("config invalid after edit: %w", err)
			}
			cmd.Println("config ok")
			return nil
		},
	}
}

// tokenStatusWord reports token presence without revealing it.
func tokenStatusWord(app *App) string {
	if _, err := os.Stat(app.Config.TokenFile(app.Paths)); err == nil {
		return "present"
	}
	if os.Getenv("RIG_GH_TOKEN") != "" {
		return "present (env)"
	}
	return "absent"
}
