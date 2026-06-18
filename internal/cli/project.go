package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AndrewMast/rig/internal/git"
	"github.com/AndrewMast/rig/internal/model"
	"github.com/AndrewMast/rig/internal/registry"
	"github.com/spf13/cobra"
)

// materialize clones the project if needed, verifies access over SSH (the only
// trusted verification), applies the push guard, and flips the project (and its
// key) to active. It operates on the registry's stored copies and mirrors the
// result back into p.
func (a *App) materialize(cmd *cobra.Command, reg *registry.Registry, p *model.Project) error {
	sp := reg.FindProject(p.Group, p.Name)
	if sp == nil {
		sp = p
	}
	g := reg.FindGroup(sp.Group)
	if g == nil {
		return fmt.Errorf("group %q missing from registry", sp.Group)
	}
	ctx := context.Background()
	path := sp.Path(*g)
	url := a.remoteURLFor(reg, sp)
	sshKey := a.projectKeyPath(reg, sp) // "" for public/local

	// Clone unless the checkout already exists.
	if _, err := os.Stat(filepath.Join(path, ".git")); os.IsNotExist(err) {
		if url == "" {
			return fmt.Errorf("no remote URL for %s", sp.ID())
		}
		if err := a.Git.Clone(ctx, url, path, sshKey); err != nil {
			return err
		}
	} else if sshKey != "" {
		// Existing checkout (e.g. a hosted local project): pin the deploy key
		// into the repo's local config.
		if err := a.Git.SetSSHCommand(ctx, path, sshKey); err != nil {
			return err
		}
	}

	// Verify read access over SSH/HTTPS (repo exists + key reads).
	if url != "" {
		if err := a.Git.LsRemote(ctx, path, url, sshKey); err != nil {
			return fmt.Errorf("verification failed (repo unreachable / key not added yet): %w", err)
		}
	}

	// Apply the push guard and, for write keys, verify write access.
	if err := a.applyGuard(ctx, reg, sp, path); err != nil {
		return err
	}

	sp.State = model.StateActive
	if k := reg.FindKey(sp.KeyID); k != nil {
		k.State = model.StateActive
	}
	*p = *sp
	cmd.Printf("finished %s at %s\n", sp.ID(), path)
	return nil
}

// pendingBoundKey reports whether the project's bound deploy key is still
// pending verification.
func (a *App) pendingBoundKey(reg *registry.Registry, p *model.Project) bool {
	if p.Strategy != model.StrategyDeployKey {
		return false
	}
	k := reg.FindKey(p.KeyID)
	return k != nil && k.State == model.StatePending
}

// projectKeyPath returns the private-key path for a deploy-key project, or ""
// when the project has no bound key (public or local-only).
func (a *App) projectKeyPath(reg *registry.Registry, p *model.Project) string {
	if p.Strategy != model.StrategyDeployKey {
		return ""
	}
	if k := reg.FindKey(p.KeyID); k != nil {
		return a.keyPath(*k)
	}
	return ""
}

// applyGuard sets or lifts the per-project push guard to match p.Guard, and for
// an unguarded write key verifies push access via a dry run.
func (a *App) applyGuard(ctx context.Context, reg *registry.Registry, p *model.Project, path string) error {
	if p.Strategy != model.StrategyDeployKey {
		return nil
	}
	url := a.remoteURLFor(reg, p)
	if p.Guard {
		return a.Git.SetPushURL(ctx, path, "origin", git.NoPush)
	}
	// Lift any guard by restoring the push URL, then prove write works.
	if err := a.Git.SetRemoteURL(ctx, path, "origin", url); err != nil {
		return err
	}
	if err := a.Git.PushDryRun(ctx, path, "origin"); err != nil {
		return fmt.Errorf("write verification failed: %w", err)
	}
	return nil
}

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Operate on an existing project",
	}
	cmd.AddCommand(
		newProjectFinishCmd(),
		newProjectDeleteCmd(),
		newProjectGuardCmd(),
		newProjectAliasCmd(),
		newProjectKeyCmd(),
		newProjectOriginCmd(),
		newProjectUpstreamCmd(),
	)
	return cmd
}

func newProjectFinishCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "finish [group/name]",
		Short: "Verify and complete a pending project (clone + git-over-SSH checks)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			token, err := app.projectToken(firstArg(args))
			if err != nil {
				return err
			}
			t, err := app.resolveTarget(reg, token, false)
			if err != nil {
				return err
			}
			p := reg.FindProject(t.Project.Group, t.Project.Name)
			if p == nil {
				return fmt.Errorf("project %q not found", token)
			}
			// A project can be active while a freshly (re)bound key is still
			// pending — e.g. after `rig project key` mints a key the handoff
			// hasn't verified yet. Re-run verification in that case rather than
			// short-circuiting, so finish promotes the stranded key.
			if p.State == model.StateActive && !app.pendingBoundKey(reg, p) {
				cmd.Printf("%s is already active\n", p.ID())
				return nil
			}
			if err := app.materialize(cmd, reg, p); err != nil {
				return err
			}
			return app.SaveRegistry(reg)
		},
	}
}

func newProjectDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <group/name>",
		Short: "Delete a project (refuses on uncommitted/unpushed work unless --force)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			t, err := app.resolveTarget(reg, args[0], false)
			if err != nil {
				return err
			}
			p := reg.FindProject(t.Project.Group, t.Project.Name)
			if p == nil {
				return fmt.Errorf("project %q not found", args[0])
			}
			g := reg.FindGroup(p.Group)
			path := p.Path(*g)

			if !force {
				if st, err := app.Git.Status(context.Background(), path); err == nil {
					if st.Dirty {
						return fmt.Errorf("%s has uncommitted changes; use --force", p.ID())
					}
					if st.Ahead > 0 {
						return fmt.Errorf("%s has %d unpushed commit(s); use --force", p.ID(), st.Ahead)
					}
				}
			}
			ok, err := app.confirm(fmt.Sprintf("Delete %s and its folder %s?", p.ID(), path), false, force)
			if err != nil || !ok {
				return err
			}
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("remove folder: %w", err)
			}
			if err := reg.RemoveProject(p.Group, p.Name); err != nil {
				return err
			}
			if err := app.SaveRegistry(reg); err != nil {
				return err
			}
			cmd.Printf("deleted %s\n", p.ID())
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "delete even with uncommitted/unpushed work")
	return cmd
}

func newProjectGuardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "guard [group/name] <on|off>",
		Short: "Set or lift the per-project push guard",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			// The mode is always the last arg; an optional leading arg is the
			// project token (otherwise the cwd project).
			explicit, mode := "", args[len(args)-1]
			if len(args) == 2 {
				explicit = args[0]
			}
			var guard bool
			switch mode {
			case "on":
				guard = true
			case "off":
				guard = false
			default:
				return fmt.Errorf("expected on|off, got %q", mode)
			}
			token, err := app.projectToken(explicit)
			if err != nil {
				return err
			}
			t, err := app.resolveTarget(reg, token, false)
			if err != nil {
				return err
			}
			p := reg.FindProject(t.Project.Group, t.Project.Name)
			if p == nil {
				return fmt.Errorf("project %q not found", token)
			}
			g := reg.FindGroup(p.Group)
			p.Guard = guard
			if p.State == model.StateActive {
				if err := app.applyGuard(context.Background(), reg, p, p.Path(*g)); err != nil {
					return err
				}
			}
			if err := app.SaveRegistry(reg); err != nil {
				return err
			}
			cmd.Printf("push guard %s for %s\n", mode, p.ID())
			return nil
		},
	}
}

func newProjectAliasCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "alias <add|rm|list> [group/name] [alias]",
		Short: "Manage project aliases",
		Args:  cobra.RangeArgs(1, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			// The trailing arg of add/rm is always the alias, so the optional
			// middle arg (if any) is the project token; otherwise use the cwd
			// project. list takes only an optional token.
			action := args[0]
			explicit, alias := "", ""
			switch action {
			case "list":
				if len(args) >= 2 {
					explicit = args[1]
				}
			case "add", "rm":
				switch len(args) {
				case 2:
					alias = args[1]
				case 3:
					explicit, alias = args[1], args[2]
				default:
					return fmt.Errorf("usage: rig project alias %s [group/name] <alias>", action)
				}
			default:
				return fmt.Errorf("unknown action %q (want add|rm|list)", action)
			}
			token, err := app.projectToken(explicit)
			if err != nil {
				return err
			}
			t, err := app.resolveTarget(reg, token, false)
			if err != nil {
				return err
			}
			p := reg.FindProject(t.Project.Group, t.Project.Name)
			if p == nil {
				return fmt.Errorf("project %q not found", token)
			}
			switch action {
			case "list":
				for _, al := range p.Aliases {
					cmd.Println(al)
				}
				return nil
			case "add":
				if err := ensureAliasFree(reg, alias); err != nil {
					return err
				}
				updated, changed := addAlias(p.Aliases, alias)
				if !changed {
					cmd.Println("alias already present")
					return nil
				}
				p.Aliases = updated
			case "rm":
				updated, changed := removeAlias(p.Aliases, alias)
				if !changed {
					return fmt.Errorf("alias %q not found on %s", alias, p.ID())
				}
				p.Aliases = updated
			}
			return app.SaveRegistry(reg)
		},
	}
}
