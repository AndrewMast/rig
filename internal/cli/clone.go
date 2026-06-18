package cli

import (
	"context"
	"fmt"

	"github.com/AndrewMast/rig/internal/handoff"
	"github.com/AndrewMast/rig/internal/model"
	"github.com/AndrewMast/rig/internal/registry"
	"github.com/spf13/cobra"
)

func newCloneCmd() *cobra.Command {
	var read, public bool
	var typ string
	cmd := &cobra.Command{
		Use:   "clone <owner/repo>",
		Short: "Clone an existing repo (write key by default; --read or --public)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			owner, repoName, ok := model.ParseRepo(args[0])
			if !ok {
				return fmt.Errorf("expected owner/repo, got %q", args[0])
			}
			if read && public {
				return fmt.Errorf("--read and --public are mutually exclusive")
			}
			repo := owner + "/" + repoName

			reg, err := app.Registry()
			if err != nil {
				return err
			}

			// Group + name prompts (pre-fill group from cwd, name from repo).
			group := ""
			if cg, ok := groupForCwd(app, reg); ok {
				group = cg
			}
			group, err = app.UI.Input("Group", group)
			if err != nil {
				return err
			}
			if group == "" {
				return fmt.Errorf("group is required")
			}
			name, err := app.UI.Input("Project name", repoName)
			if err != nil {
				return err
			}
			name = reg.SuggestProjectName(group, name)

			g, err := ensureGroup(app, reg, group, "")
			if err != nil {
				return err
			}

			p := model.Project{Group: g.Name, Name: name, Type: typ, Repo: repo}

			// Detect-and-offer: when a key-based strategy is about to provision a
			// deploy key for a repo that's actually public, offer the keyless
			// HTTPS path instead. Best-effort — a failed/anonymous check just
			// proceeds with the key. Skipped when --public was already chosen.
			if !public {
				if isPub, err := app.GH.RepoPublic(context.Background(), owner, repoName); err == nil && isPub {
					ok, err := app.UI.Confirm(
						fmt.Sprintf("%s is public — clone over HTTPS without a deploy key?", repo), true)
					if err != nil {
						return err
					}
					public = ok
				}
			}

			switch {
			case public:
				p.Strategy = model.StrategyPublic
			default:
				p.Strategy = model.StrategyDeployKey
				p.Guard = read // read clones are push-guarded
			}

			// Public: no key, clone immediately.
			if public {
				p.State = model.StatePending
				if err := reg.AddProject(p); err != nil {
					return err
				}
				if err := app.materialize(cmd, reg, &p); err != nil {
					return err
				}
				return app.SaveRegistry(reg)
			}

			// Deploy-key: reuse or mint a key.
			key, mut, fresh, err := app.keyForClone(reg, repo, !read)
			if err != nil {
				return err
			}
			p.KeyID = key.ID
			p.State = model.StatePending
			if err := reg.AddProject(p); err != nil {
				return err
			}
			if err := app.SaveRegistry(reg); err != nil {
				return err
			}

			// A reused, already-active key lets us clone right now.
			if !fresh && key.State == model.StateActive {
				if err := app.materialize(cmd, reg, &p); err != nil {
					return err
				}
				return app.SaveRegistry(reg)
			}

			// Fresh key: deliver the deploy-key-add handoff, then either
			// auto-finish (gh-direct) or wait for `rig project finish`.
			batch := handoff.Batch{Repo: repo}
			if mut != nil {
				batch.Add(*mut)
			}
			method, err := app.chooseMethod(cmd)
			if err != nil {
				return err
			}
			if err := handoff.Deliver(method, app.handoffEnv(cmd), batch); err != nil {
				return err
			}
			if method == "gh" {
				key.State = model.StateActive
				if err := app.materialize(cmd, reg, &p); err != nil {
					return err
				}
				return app.SaveRegistry(reg)
			}
			cmd.Printf("project %s is pending — run `rig project finish %s` once the deploy key is added\n", p.ID(), p.ID())
			return nil
		},
	}
	cmd.Flags().BoolVar(&read, "read", false, "read-only clone (read deploy key, push-guarded)")
	cmd.Flags().BoolVar(&public, "public", false, "public repo over HTTPS, no key")
	cmd.Flags().StringVar(&typ, "type", "", "project type profile")
	return cmd
}

// keyForClone reuses or mints a key for a clone. Write clones reuse an existing
// write key or mint one; read clones reuse ANY existing key or mint a read key.
// fresh reports whether a new key was minted (its add must be handed off).
func (a *App) keyForClone(reg *registry.Registry, repo string, write bool) (key *model.Key, mut *handoff.Mutation, fresh bool, err error) {
	existing := reg.KeysForRepo(repo)
	if write {
		for _, k := range existing {
			if k.Write {
				return reg.FindKey(k.ID), nil, false, nil
			}
		}
	} else if len(existing) > 0 {
		// Any key (write keys read too) satisfies a read clone.
		return reg.FindKey(existing[0].ID), nil, false, nil
	}
	k, m, err := a.mintKey(reg, repo, write, "")
	if err != nil {
		return nil, nil, false, err
	}
	return k, &m, true, nil
}

// remoteURLFor returns the origin URL for a project's strategy.
func (a *App) remoteURLFor(reg *registry.Registry, p *model.Project) string {
	switch p.Strategy {
	case model.StrategyPublic:
		return "https://github.com/" + p.Repo + ".git"
	case model.StrategyDeployKey:
		if k := reg.FindKey(p.KeyID); k != nil {
			return keyRemoteURL(*k)
		}
	}
	return ""
}
