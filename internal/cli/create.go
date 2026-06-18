package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AndrewMast/rig/internal/model"
	"github.com/AndrewMast/rig/internal/registry"
	"github.com/spf13/cobra"
)

func newCreateCmd() *cobra.Command {
	var typ string
	cmd := &cobra.Command{
		Use:   "create [Group/name | name]",
		Short: "Scaffold a new local-only project (host it later)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			arg := ""
			if len(args) == 1 {
				arg = args[0]
			}
			group, name, err := app.parseProjectArg(reg, arg, "")
			if err != nil {
				return err
			}
			name = reg.SuggestProjectName(group, name)

			g, err := ensureGroup(app, reg, group, "")
			if err != nil {
				return err
			}
			p := model.Project{
				Group:    g.Name,
				Name:     name,
				Type:     typ,
				Strategy: model.StrategyLocal,
				State:    model.StateActive,
			}
			path := p.Path(*g)
			if err := os.MkdirAll(path, 0o755); err != nil {
				return fmt.Errorf("create project folder: %w", err)
			}
			if err := app.Git.Init(context.Background(), path); err != nil {
				return err
			}
			if err := app.runHook(reg, g, &p, "create"); err != nil {
				return err
			}
			if err := reg.AddProject(p); err != nil {
				return err
			}
			if err := app.SaveRegistry(reg); err != nil {
				return err
			}
			cmd.Printf("created %s (local-only) at %s\n", p.ID(), path)
			return nil
		},
	}
	cmd.Flags().StringVar(&typ, "type", "", "project type profile")
	return cmd
}

// parseProjectArg resolves a "Group/name" or "name" argument (or empty) into a
// group and project name, inferring the group from the cwd and confirming with
// the user. defGroup, when non-empty, pre-fills the group prompt.
func (a *App) parseProjectArg(reg *registry.Registry, arg, defGroup string) (group, name string, err error) {
	if g, n, ok := splitSlash(arg); ok {
		return g, n, nil
	}

	name = arg
	if name == "" {
		name, err = a.UI.Input("Project name", "")
		if err != nil {
			return "", "", err
		}
	}
	if name == "" {
		return "", "", fmt.Errorf("project name is required")
	}

	group = defGroup
	if group == "" {
		if cg, ok := groupForCwd(a, reg); ok {
			group = cg
		}
	}
	group, err = a.UI.Input("Group", group)
	if err != nil {
		return "", "", err
	}
	if group == "" {
		return "", "", fmt.Errorf("group is required")
	}
	return group, name, nil
}

func splitSlash(s string) (a, b string, ok bool) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// groupForCwd returns the group whose folder contains the cwd, if any.
func groupForCwd(app *App, reg *registry.Registry) (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for _, g := range reg.Groups {
		gp := g.Path()
		if cwd == gp || strings.HasPrefix(cwd, gp+string(filepath.Separator)) {
			return g.Name, true
		}
	}
	return "", false
}
