package cli

import (
	"context"
	"sort"

	"github.com/AndrewMast/rig/internal/git"
	"github.com/AndrewMast/rig/internal/model"
	"github.com/AndrewMast/rig/internal/registry"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [token]",
		Short: "Report per-project state, key access, push guard, and git status",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}

			var projects []model.Project
			if len(args) == 1 {
				t, err := app.resolveTarget(reg, args[0], false)
				if err != nil {
					return err
				}
				projects = []model.Project{t.Project}
			} else if tok, ok := projectTokenForCwd(app); ok {
				t, err := app.resolveTarget(reg, tok, false)
				if err != nil {
					return err
				}
				projects = []model.Project{t.Project}
			} else {
				projects = append(projects, reg.Projects...)
				sort.Slice(projects, func(i, j int) bool { return projects[i].ID() < projects[j].ID() })
			}

			for _, p := range projects {
				app.printStatus(cmd, reg, p)
			}
			return nil
		},
	}
}

func (a *App) printStatus(cmd *cobra.Command, reg *registry.Registry, p model.Project) {
	cmd.Printf("%s\n", p.ID())
	cmd.Printf("  state:    %s\n", p.State)
	cmd.Printf("  remote:   %s\n", describeRemote(reg, p))
	if p.Strategy == model.StrategyDeployKey {
		if k := reg.FindKey(p.KeyID); k != nil && k.State == model.StatePending {
			cmd.Printf("  key:      %s %s pending (run `rig key verify`)\n", k.ID, k.Access())
		}
	}

	g := reg.FindGroup(p.Group)
	if g == nil {
		cmd.Printf("  (group %q missing from registry)\n", p.Group)
		return
	}
	st, err := a.Git.Status(context.Background(), p.Path(*g))
	if err != nil {
		cmd.Printf("  git:      unavailable (%v)\n", err)
		return
	}
	dirty := "clean"
	if st.Dirty {
		dirty = "dirty"
	}
	cmd.Printf("  branch:   %s (%s)\n", branchOr(st), dirty)
	if st.Upstream != "" {
		cmd.Printf("  tracking: %s (ahead %d, behind %d)\n", st.Upstream, st.Ahead, st.Behind)
	} else if p.Strategy == model.StrategyLocal {
		cmd.Printf("  tracking: none (local-only)\n")
	}
}

func branchOr(st git.Status) string {
	if st.Branch == "" {
		return "(detached)"
	}
	return st.Branch
}
