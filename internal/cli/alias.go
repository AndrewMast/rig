package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/AndrewMast/rig/internal/registry"
	"github.com/spf13/cobra"
)

// addAlias appends a unique alias to a list, returning the new list and whether
// it changed. Comparison is case-insensitive.
func addAlias(list []string, alias string) ([]string, bool) {
	for _, e := range list {
		if strings.EqualFold(e, alias) {
			return list, false
		}
	}
	return append(list, alias), true
}

// removeAlias drops an alias (case-insensitive) from a list.
func removeAlias(list []string, alias string) ([]string, bool) {
	for i, e := range list {
		if strings.EqualFold(e, alias) {
			return append(list[:i], list[i+1:]...), true
		}
	}
	return list, false
}

// ensureAliasFree errors if the alias is already used by any project or group.
func ensureAliasFree(reg *registry.Registry, alias string) error {
	for _, p := range reg.Projects {
		for _, al := range p.Aliases {
			if strings.EqualFold(al, alias) {
				return fmt.Errorf("alias %q already used by project %s", alias, p.ID())
			}
		}
	}
	for _, g := range reg.Groups {
		for _, al := range g.Aliases {
			if strings.EqualFold(al, alias) {
				return fmt.Errorf("alias %q already used by group %s", alias, g.Name)
			}
		}
	}
	return nil
}

func newGroupAliasCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alias <add|rm|list> <group> [alias]",
		Short: "Manage group aliases",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			action, groupName := args[0], args[1]
			g := reg.FindGroup(groupName)
			if g == nil {
				return fmt.Errorf("group %q not found", groupName)
			}
			switch action {
			case "list":
				for _, al := range g.Aliases {
					cmd.Println(al)
				}
				return nil
			case "add":
				if len(args) < 3 {
					return fmt.Errorf("usage: rig group alias add <group> <alias>")
				}
				if err := ensureAliasFree(reg, args[2]); err != nil {
					return err
				}
				updated, changed := addAlias(g.Aliases, args[2])
				if !changed {
					cmd.Println("alias already present")
					return nil
				}
				g.Aliases = updated
			case "rm":
				if len(args) < 3 {
					return fmt.Errorf("usage: rig group alias rm <group> <alias>")
				}
				updated, changed := removeAlias(g.Aliases, args[2])
				if !changed {
					return fmt.Errorf("alias %q not found on group %q", args[2], groupName)
				}
				g.Aliases = updated
			default:
				return fmt.Errorf("unknown action %q (want add|rm|list)", action)
			}
			return app.SaveRegistry(reg)
		},
	}
	return cmd
}

func newAliasCmd() *cobra.Command {
	var projOnly, groupOnly bool
	cmd := &cobra.Command{
		Use:   "alias",
		Short: "Show all aliases across projects and groups",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			type row struct{ alias, target, kind string }
			var rows []row
			if !groupOnly {
				for _, p := range reg.Projects {
					for _, al := range p.Aliases {
						rows = append(rows, row{al, p.ID(), "project"})
					}
				}
			}
			if !projOnly {
				for _, g := range reg.Groups {
					for _, al := range g.Aliases {
						rows = append(rows, row{al, g.Name, "group"})
					}
				}
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].alias < rows[j].alias })
			if len(rows) == 0 {
				cmd.Println("no aliases defined")
				return nil
			}
			for _, r := range rows {
				cmd.Printf("%-16s -> %-24s (%s)\n", r.alias, r.target, r.kind)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&projOnly, "project", false, "show only project aliases")
	cmd.Flags().BoolVar(&groupOnly, "group", false, "show only group aliases")
	return cmd
}
