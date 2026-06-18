package cli

import (
	"fmt"
	"sort"

	"github.com/AndrewMast/rig/internal/model"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var group string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List managed projects",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			projects := reg.Projects
			if group != "" {
				projects = reg.ProjectsInGroup(group)
			}
			if len(projects) == 0 {
				cmd.Println("no projects")
				return nil
			}
			sort.Slice(projects, func(i, j int) bool { return projects[i].ID() < projects[j].ID() })
			for _, p := range projects {
				cmd.Printf("%-28s %-10s %s\n", p.ID(), p.Strategy, describeRemote(reg, p))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&group, "group", "", "only projects in this group")
	return cmd
}

// describeRemote summarizes a project's hosting for the list view.
func describeRemote(reg interface {
	FindKey(string) *model.Key
}, p model.Project) string {
	switch p.Strategy {
	case model.StrategyLocal:
		return "local-only"
	case model.StrategyPublic:
		return p.Repo + " (public)"
	case model.StrategyDeployKey:
		access := "?"
		if k := reg.FindKey(p.KeyID); k != nil {
			access = k.Access()
		}
		s := fmt.Sprintf("%s (%s)", p.Repo, access)
		if p.Guard {
			s += " [push-guarded]"
		}
		return s
	default:
		return p.Repo
	}
}
