package cli

import (
	"context"
	"fmt"

	"github.com/AndrewMast/rig/internal/handoff"
	"github.com/AndrewMast/rig/internal/model"
	"github.com/AndrewMast/rig/internal/registry"
	"github.com/spf13/cobra"
)

// loadProject resolves a token to the registry's stored project pointer.
func (a *App) loadProject(reg *registry.Registry, token string) (*model.Project, error) {
	t, err := a.resolveTarget(reg, token, false)
	if err != nil {
		return nil, err
	}
	p := reg.FindProject(t.Project.Group, t.Project.Name)
	if p == nil {
		return nil, fmt.Errorf("project %q not found", token)
	}
	return p, nil
}

func newProjectKeyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "key <group/name>",
		Short: "Pick, create, or re-bind the project's deploy key (+ guard)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			p, err := app.loadProject(reg, args[0])
			if err != nil {
				return err
			}
			if p.Repo == "" {
				return fmt.Errorf("%s has no origin; use `rig project origin add` first", p.ID())
			}

			existing := reg.KeysForRepo(p.Repo)
			labels := make([]string, 0, len(existing)+2)
			for _, k := range existing {
				labels = append(labels, fmt.Sprintf("%s %s (%s)", k.ID, k.Access(), k.State))
			}
			labels = append(labels, "create new READ key", "create new WRITE key")
			idx, err := app.UI.Select("Choose a key for "+p.ID()+":", labels)
			if err != nil {
				return err
			}

			var chosen *model.Key
			var mut *handoff.Mutation
			fresh := false
			switch {
			case idx < len(existing):
				chosen = reg.FindKey(existing[idx].ID)
			case idx == len(existing): // new read
				k, m, e := app.mintKey(reg, p.Repo, false, "")
				if e != nil {
					return e
				}
				chosen, mut, fresh = k, &m, true
			default: // new write
				k, m, e := app.mintKey(reg, p.Repo, true, "")
				if e != nil {
					return e
				}
				chosen, mut, fresh = k, &m, true
			}

			p.KeyID = chosen.ID
			p.Guard = !chosen.Write // guard tracks the key's access by default
			if err := app.SaveRegistry(reg); err != nil {
				return err
			}

			if fresh {
				batch := handoff.Batch{Repo: p.Repo}
				batch.Add(*mut)
				if err := app.deliver(cmd, batch); err != nil {
					return err
				}
			}
			// Re-point origin and apply the guard for an active checkout.
			g := reg.FindGroup(p.Group)
			path := p.Path(*g)
			url := app.remoteURLFor(reg, p)
			if err := app.Git.SetRemoteURL(context.Background(), path, "origin", url); err != nil {
				return err
			}
			if err := app.applyGuard(context.Background(), reg, p, path); err != nil {
				cmd.Printf("note: %v\n", err)
			}
			cmd.Printf("bound %s to key %s (%s); push guard %s\n", p.ID(), chosen.ID, chosen.Access(), onOff(p.Guard))
			return app.SaveRegistry(reg)
		},
	}
}

func newProjectOriginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "origin [add|remove] [group/name] [owner/repo]",
		Short: "Inspect or change a project's origin (host a local project, or unhost)",
		Args:  cobra.MaximumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			// Arg shapes:
			//   origin add|remove <g/n>      explicit action on a project
			//   origin add|remove <g/n> <owner/repo>   prefilled repo for add
			//   origin <owner/repo>          smart: set the cwd project's origin
			//   origin <g/n>                 inspect that project
			//   origin                       inspect the cwd project
			action, token, repoArg := "inspect", "", ""
			switch {
			case len(args) == 0:
				// inspect cwd
			case args[0] == "add" || args[0] == "remove":
				action = args[0]
				if len(args) < 2 {
					return fmt.Errorf("usage: rig project origin %s <group/name> [owner/repo]", args[0])
				}
				token = args[1]
				if len(args) >= 3 {
					repoArg = args[2]
				}
			default:
				// Smart reading: a bare owner/repo while inside a local project
				// means "set this project's origin".
				if _, _, ok := model.ParseRepo(args[0]); ok && reg.FindProject(splitOrEmpty(args[0])) == nil {
					if tok, inProj := projectTokenForCwd(app); inProj {
						action, token, repoArg = "add", tok, args[0]
						break
					}
				}
				token = args[0]
			}

			if token == "" {
				tok, ok := projectTokenForCwd(app)
				if !ok {
					return fmt.Errorf("not inside a project; pass a group/name")
				}
				token = tok
			}
			p, err := app.loadProject(reg, token)
			if err != nil {
				return err
			}
			switch action {
			case "inspect":
				cmd.Printf("%s\n  strategy: %s\n  repo: %s\n  upstream: %s\n",
					p.ID(), p.Strategy, orNone(p.Repo), orNone(p.Upstream))
				return nil
			case "remove":
				return app.originRemove(cmd, reg, p)
			default:
				return app.originAdd(cmd, reg, p, repoArg)
			}
		},
	}
	return cmd
}

// splitOrEmpty returns the (group, name) of a "g/n" token, or ("","") so a
// non-project owner/repo argument never matches an existing project.
func splitOrEmpty(token string) (string, string) {
	g, n, ok := splitSlash(token)
	if !ok {
		return "", ""
	}
	return g, n
}

// originRemove drops the origin remote and returns the project to local-only.
// The GitHub repo is left untouched.
func (a *App) originRemove(cmd *cobra.Command, reg *registry.Registry, p *model.Project) error {
	g := reg.FindGroup(p.Group)
	_ = a.Git.RemoveRemote(context.Background(), p.Path(*g), "origin")
	p.Strategy = model.StrategyLocal
	p.Repo = ""
	p.KeyID = ""
	p.Guard = false
	if err := a.SaveRegistry(reg); err != nil {
		return err
	}
	cmd.Printf("%s is now local-only (GitHub repo left alone)\n", p.ID())
	return nil
}

// originAdd hosts a local project or attaches a new writable origin onto an
// existing (e.g. read-only) clone. When it attaches a new repo over an existing
// origin, the old source is demoted to `upstream` by default. It decides
// create-vs-attach, binds a write key, wires the remotes, and runs the
// handoff + verify loop. repoArg, when non-empty, prefills the target repo.
func (a *App) originAdd(cmd *cobra.Command, reg *registry.Registry, p *model.Project, repoArg string) error {
	ctx := context.Background()
	// Capture the current origin before we change it (for demote-to-upstream).
	oldRepo := p.Repo
	oldURL := a.remoteURLFor(reg, p)

	// Determine the target repo.
	owner, repoName := "", ""
	if r := repoArg; r != "" {
		o, n, ok := model.ParseRepo(r)
		if !ok {
			return fmt.Errorf("expected owner/repo, got %q", r)
		}
		owner, repoName = o, n
	} else {
		defOwner := ""
		if a.GH.Available() {
			if login, err := a.GH.Login(ctx); err == nil {
				defOwner = login
			}
		}
		o, err := a.UI.Input("Owner", defOwner)
		if err != nil {
			return err
		}
		n, err := a.UI.Input("Repo name", p.Name)
		if err != nil {
			return err
		}
		owner, repoName = o, n
	}
	repo := owner + "/" + repoName

	if repo == oldRepo {
		return fmt.Errorf("%s already has origin %s", p.ID(), oldRepo)
	}

	// New vs attach: the token probes existence; always confirm.
	exists := false
	if a.GH.Available() {
		if ex, err := a.GH.RepoExists(ctx, owner, repoName); err == nil {
			exists = ex
		}
	}
	createNew := !exists
	prompt := fmt.Sprintf("Create new repo %s?", repo)
	if exists {
		prompt = fmt.Sprintf("Repo %s exists — attach to it?", repo)
	}
	ok, err := a.UI.Confirm(prompt, true)
	if err != nil || !ok {
		return err
	}

	// Demote an existing source (e.g. a read-only clone) to upstream so the
	// PR-back relationship is preserved.
	demote := false
	if oldRepo != "" {
		demote, err = a.UI.Confirm(fmt.Sprintf("Demote current source %s to upstream?", oldRepo), true)
		if err != nil {
			return err
		}
	}

	key, mut, fresh, err := a.keyForClone(reg, repo, true) // hosting your own repo → write key
	if err != nil {
		return err
	}
	p.Repo = repo
	p.Strategy = model.StrategyDeployKey
	p.KeyID = key.ID
	p.Guard = false
	p.State = model.StatePending
	if demote {
		p.Upstream = oldRepo
	}
	if err := a.SaveRegistry(reg); err != nil {
		return err
	}

	batch := handoff.Batch{Repo: repo}
	if createNew {
		batch.Add(handoff.RepoCreate(repo, true))
	}
	if fresh && mut != nil {
		batch.Add(*mut)
	}
	method, err := a.chooseMethod(cmd)
	if err != nil {
		return err
	}
	if err := handoff.Deliver(method, a.handoffEnv(cmd), batch); err != nil {
		return err
	}

	// Move the old source onto `upstream` before re-pointing origin.
	g := reg.FindGroup(p.Group)
	path := p.Path(*g)
	if demote && oldURL != "" {
		if err := a.Git.AddRemote(ctx, path, "upstream", oldURL); err != nil {
			_ = a.Git.SetRemoteURL(ctx, path, "upstream", oldURL)
		}
		cmd.Printf("demoted %s to upstream\n", oldRepo)
	}

	// Wire the (new) origin remote on the existing checkout.
	url := a.remoteURLFor(reg, p)
	if err := a.Git.AddRemote(ctx, path, "origin", url); err != nil {
		// Already present? fall back to set-url.
		_ = a.Git.SetRemoteURL(ctx, path, "origin", url)
	}

	if method == "gh" {
		key.State = model.StateActive
		if err := a.Git.LsRemote(ctx, url); err != nil {
			return fmt.Errorf("verification failed: %w", err)
		}
		p.State = model.StateActive
		cmd.Printf("hosted %s at %s\n", p.ID(), repo)
		return a.SaveRegistry(reg)
	}
	cmd.Printf("%s pending — run `rig project finish %s` once the repo/key exist\n", p.ID(), p.ID())
	return nil
}

func newProjectUpstreamCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upstream [add|remove]",
		Short: "Inspect or change a project's upstream (fork-like) remote",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			action, token := "inspect", args[0]
			if args[0] == "add" || args[0] == "remove" {
				if len(args) != 2 {
					return fmt.Errorf("usage: rig project upstream %s <group/name>", args[0])
				}
				action, token = args[0], args[1]
			}
			p, err := app.loadProject(reg, token)
			if err != nil {
				return err
			}
			g := reg.FindGroup(p.Group)
			path := p.Path(*g)
			ctx := context.Background()

			switch action {
			case "inspect":
				cmd.Printf("%s upstream: %s\n", p.ID(), orNone(p.Upstream))
				return nil
			case "add":
				up, err := app.UI.Input("Upstream owner/repo", "")
				if err != nil {
					return err
				}
				if _, _, ok := model.ParseRepo(up); !ok {
					return fmt.Errorf("expected owner/repo, got %q", up)
				}
				url := "https://github.com/" + up + ".git"
				if err := app.Git.AddRemote(ctx, path, "upstream", url); err != nil {
					_ = app.Git.SetRemoteURL(ctx, path, "upstream", url)
				}
				p.Upstream = up
				cmd.Printf("added upstream %s to %s\n", up, p.ID())
			case "remove":
				_ = app.Git.RemoveRemote(ctx, path, "upstream")
				p.Upstream = ""
				cmd.Printf("removed upstream from %s\n", p.ID())
			}
			return app.SaveRegistry(reg)
		},
	}
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
