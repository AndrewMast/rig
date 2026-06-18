package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AndrewMast/rig/internal/model"
	"github.com/AndrewMast/rig/internal/registry"
	"github.com/spf13/cobra"
)

func newGroupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Manage groups (named wrappers that own a base path)",
	}
	cmd.AddCommand(
		newGroupNewCmd(),
		newGroupListCmd(),
		newGroupRenameCmd(),
		newGroupMoveCmd(),
		newGroupDeleteCmd(),
		newGroupAliasCmd(),
	)
	return cmd
}

func newGroupNewCmd() *cobra.Command {
	var base string
	cmd := &cobra.Command{
		Use:   "new <Name>",
		Short: "Create an empty group and its folder",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			if name == "" {
				name, err = app.UI.Input("Group name", "")
				if err != nil {
					return err
				}
			}
			if base == "" {
				base, err = app.UI.Input("Base path", app.Config.DefaultBase)
				if err != nil {
					return err
				}
			}
			g, err := ensureGroup(app, reg, name, base)
			if err != nil {
				return err
			}
			if err := app.SaveRegistry(reg); err != nil {
				return err
			}
			cmd.Printf("created group %s at %s\n", g.Name, g.Path())
			return nil
		},
	}
	cmd.Flags().StringVar(&base, "base", "", "base path for the group (default: config default_base)")
	return cmd
}

func newGroupListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List groups",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			if len(reg.Groups) == 0 {
				cmd.Println("no groups yet")
				return nil
			}
			for _, g := range reg.Groups {
				n := len(reg.ProjectsInGroup(g.Name))
				line := fmt.Sprintf("%-20s %s  (%d project", g.Name, g.Base, n)
				if n != 1 {
					line += "s"
				}
				line += ")"
				if len(g.Aliases) > 0 {
					line += fmt.Sprintf("  aliases: %v", g.Aliases)
				}
				cmd.Println(line)
			}
			return nil
		},
	}
}

func newGroupRenameCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Rename a group and move its folder (relocates member projects)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			g := reg.FindGroup(args[0])
			if g == nil {
				return fmt.Errorf("group %q not found", args[0])
			}
			if existing := reg.FindGroup(args[1]); existing != nil {
				return fmt.Errorf("group %q already exists", args[1])
			}
			oldPath := g.Path()
			newPath := filepath.Join(g.Base, args[1])
			members := reg.ProjectsInGroup(g.Name)

			cmd.Printf("rename %s -> %s\n  %s -> %s\n", g.Name, args[1], oldPath, newPath)
			warnAffected(cmd, members)
			ok, err := app.confirm("Proceed?", false, yes)
			if err != nil || !ok {
				return err
			}
			if err := moveDir(oldPath, newPath); err != nil {
				return err
			}
			// Update group name and re-point members.
			newName := args[1]
			for i := range reg.Projects {
				if strings.EqualFold(reg.Projects[i].Group, g.Name) {
					reg.Projects[i].Group = newName
				}
			}
			reg.FindGroup(g.Name).Name = newName
			if err := app.SaveRegistry(reg); err != nil {
				return err
			}
			cmd.Println("renamed")
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func newGroupMoveCmd() *cobra.Command {
	var base string
	var yes bool
	cmd := &cobra.Command{
		Use:   "move <name> --base <newbase>",
		Short: "Move a group to a new base path (relocates member projects)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			if base == "" {
				return fmt.Errorf("--base is required")
			}
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			g := reg.FindGroup(args[0])
			if g == nil {
				return fmt.Errorf("group %q not found", args[0])
			}
			oldPath := g.Path()
			newPath := filepath.Join(base, g.Name)
			members := reg.ProjectsInGroup(g.Name)

			cmd.Printf("move %s\n  %s -> %s\n", g.Name, oldPath, newPath)
			warnAffected(cmd, members)
			ok, err := app.confirm("Proceed?", false, yes)
			if err != nil || !ok {
				return err
			}
			if err := moveDir(oldPath, newPath); err != nil {
				return err
			}
			reg.FindGroup(g.Name).Base = base
			if err := app.SaveRegistry(reg); err != nil {
				return err
			}
			cmd.Println("moved")
			return nil
		},
	}
	cmd.Flags().StringVar(&base, "base", "", "new base path")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func newGroupDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an empty group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			g := reg.FindGroup(args[0])
			if g == nil {
				return fmt.Errorf("group %q not found", args[0])
			}
			members := reg.ProjectsInGroup(g.Name)
			if len(members) > 0 {
				cmd.Println("group is not empty; remove these projects first:")
				warnAffected(cmd, members)
				return fmt.Errorf("refusing to delete non-empty group %q", g.Name)
			}
			path := g.Path()
			if err := reg.RemoveGroup(g.Name); err != nil {
				return err
			}
			if err := app.SaveRegistry(reg); err != nil {
				return err
			}
			// Remove the folder only if it is empty.
			if entries, err := os.ReadDir(path); err == nil && len(entries) == 0 {
				_ = os.Remove(path)
			}
			cmd.Printf("deleted group %s\n", g.Name)
			return nil
		},
	}
}

// ensureGroup returns the named group, auto-vivifying it (after confirm) with
// the given base if it does not exist. The base is ignored when the group
// already exists.
func ensureGroup(app *App, reg *registry.Registry, name, base string) (*model.Group, error) {
	if g := reg.FindGroup(name); g != nil {
		return g, nil
	}
	if base == "" {
		base = app.Config.DefaultBase
	}
	g := model.Group{Name: name, Base: base}
	if err := os.MkdirAll(g.Path(), 0o755); err != nil {
		return nil, fmt.Errorf("create group folder: %w", err)
	}
	if err := reg.AddGroup(g); err != nil {
		return nil, err
	}
	return reg.FindGroup(name), nil
}

func warnAffected(cmd *cobra.Command, members []model.Project) {
	if len(members) == 0 {
		return
	}
	cmd.Printf("affected projects (%d):\n", len(members))
	for _, p := range members {
		cmd.Printf("  - %s\n", p.ID())
	}
}

// moveDir relocates a directory, creating the destination's parent. A missing
// source is tolerated (the registry may be ahead of the filesystem).
func moveDir(oldPath, newPath string) error {
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		return fmt.Errorf("create destination parent: %w", err)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("move %s -> %s: %w", oldPath, newPath, err)
	}
	return nil
}
