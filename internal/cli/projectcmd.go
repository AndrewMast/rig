package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/AndrewMast/rig/internal/model"
	"github.com/AndrewMast/rig/internal/registry"
	"github.com/AndrewMast/rig/internal/types"
	"github.com/spf13/cobra"
)

// registerProjectCommands adds type/project [commands] as subcommands. The set
// of names is the union of all defined types' commands plus the cwd project's
// rig.toml, so `rig <cmd> [token]` works from anywhere; execution resolves the
// target project and runs its own merged command. Precedence is built-in >
// launcher > type/project command, so any name already taken is skipped.
func registerProjectCommands(root *cobra.Command, app *App) {
	taken := map[string]bool{}
	for _, c := range root.Commands() {
		taken[c.Name()] = true
	}

	names := map[string]bool{}
	// All types' command names.
	if entries, err := os.ReadDir(app.typesDir()); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if prof, err := types.LoadType(app.typesDir(), e.Name()); err == nil {
				for n := range prof.Commands {
					names[n] = true
				}
			}
		}
	}
	// The cwd project's rig.toml commands (if any).
	if reg, err := app.Registry(); err == nil {
		if tok, ok := projectTokenForCwd(app); ok {
			if p, err := app.loadProjectQuiet(reg, tok); err == nil && p != nil {
				if g := reg.FindGroup(p.Group); g != nil {
					if prof, err := app.resolvedProfile(g, p); err == nil {
						for n := range prof.Commands {
							names[n] = true
						}
					}
				}
			}
		}
	}

	for _, name := range sortedStringSet(names) {
		if taken[name] {
			continue
		}
		root.AddCommand(newTypeRunCmd(name))
	}
}

func sortedStringSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func newTypeRunCmd(name string) *cobra.Command {
	return &cobra.Command{
		Use:                name + " [token]",
		Short:              "Project command: " + name,
		Args:               cobra.MaximumNArgs(1),
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			token := ""
			if len(args) == 1 {
				token = args[0]
			} else if t, ok := projectTokenForCwd(app); ok {
				token = t
			} else {
				return fmt.Errorf("not inside a project; pass a token")
			}
			p, err := app.loadProject(reg, token)
			if err != nil {
				return err
			}
			g := reg.FindGroup(p.Group)
			prof, err := app.resolvedProfile(g, p)
			if err != nil {
				return err
			}
			cmdline := prof.Commands[name]
			if cmdline == "" {
				return fmt.Errorf("no command %q defined for %s", name, p.ID())
			}
			return app.runInProject(reg, g, p, cmdline)
		},
	}
}

// loadProjectQuiet resolves a full "Group/Name" token to the stored project
// without prompting (used during command registration).
func (a *App) loadProjectQuiet(reg *registry.Registry, token string) (*model.Project, error) {
	g, n, ok := splitSlash(token)
	if !ok {
		return nil, fmt.Errorf("expected Group/Name, got %q", token)
	}
	p := reg.FindProject(g, n)
	if p == nil {
		return nil, fmt.Errorf("project %q not found", token)
	}
	return p, nil
}
