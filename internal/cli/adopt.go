package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AndrewMast/rig/internal/model"
	"github.com/spf13/cobra"
)

func newAdoptCmd() *cobra.Command {
	var typ string
	cmd := &cobra.Command{
		Use:   "adopt [path]",
		Short: "Adopt an existing folder as a project (defaults to cwd)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}

			target := "."
			if len(args) == 1 {
				target = args[0]
			}
			abs, err := filepath.Abs(target)
			if err != nil {
				return err
			}
			if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
				return fmt.Errorf("not a directory: %s", abs)
			}

			name := filepath.Base(abs)
			parent := filepath.Dir(abs)
			groupName := filepath.Base(parent)
			base := filepath.Dir(parent)

			// Adopt conflict: group exists with a different base.
			if g := reg.FindGroup(groupName); g != nil && g.Base != base {
				groupName, base, err = app.resolveAdoptConflict(g.Name, g.Base, base, name)
				if err != nil {
					return err
				}
			}

			cmd.Printf("adopt %s\n  group=%s base=%s name=%s\n", abs, groupName, base, name)
			ok, err := app.UI.Confirm("Proceed?", true)
			if err != nil || !ok {
				return err
			}

			if reg.FindProject(groupName, name) != nil {
				return fmt.Errorf("project %s/%s already registered", groupName, name)
			}
			g, err := ensureGroup(app, reg, groupName, base)
			if err != nil {
				return err
			}
			// If the chosen group's base puts the derived path elsewhere (the
			// "move under existing group" conflict choice), relocate the folder.
			desired := filepath.Join(g.Path(), name)
			if desired != abs {
				if err := moveDir(abs, desired); err != nil {
					return err
				}
				abs = desired
			}
			// Initialize git if the folder is not already a repo.
			if _, err := os.Stat(filepath.Join(abs, ".git")); os.IsNotExist(err) {
				if err := app.Git.Init(context.Background(), abs); err != nil {
					return err
				}
			}
			p := model.Project{
				Group:    g.Name,
				Name:     name,
				Type:     typ,
				Strategy: model.StrategyLocal,
				State:    model.StateActive,
			}
			if err := app.runHook(g, &p, "setup"); err != nil {
				return err
			}
			if err := reg.AddProject(p); err != nil {
				return err
			}
			if err := app.SaveRegistry(reg); err != nil {
				return err
			}
			cmd.Printf("adopted %s\n", p.ID())
			return nil
		},
	}
	cmd.Flags().StringVar(&typ, "type", "", "project type profile")
	return cmd
}

// resolveAdoptConflict handles a group that already exists with a different
// base. Default (and recommended) is to pick/create a different group rather
// than silently moving the folder.
func (a *App) resolveAdoptConflict(existingGroup, existingBase, derivedBase, name string) (group, base string, err error) {
	labels := []string{
		fmt.Sprintf("Pick a different group (keep folder at this base %s)", derivedBase),
		fmt.Sprintf("Move folder under existing group %q base (%s)", existingGroup, existingBase),
	}
	idx, err := a.UI.Select(
		fmt.Sprintf("Group %q already exists at a different base.", existingGroup), labels)
	if err != nil {
		return "", "", err
	}
	if idx == 1 {
		// Move folder under the existing group's base (handled by caller via
		// ensureGroup using existingBase; the actual relocation happens because
		// the derived path then differs — but we keep the folder in place by
		// using the existing group and its base, requiring the folder to move).
		return existingGroup, existingBase, nil
	}
	newName, err := a.UI.Input("New group name", name+"-group")
	if err != nil {
		return "", "", err
	}
	return newName, derivedBase, nil
}
