package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AndrewMast/rig/internal/handoff"
	"github.com/AndrewMast/rig/internal/keygen"
	"github.com/AndrewMast/rig/internal/model"
	"github.com/AndrewMast/rig/internal/registry"
	"github.com/spf13/cobra"
)

// newKeyID returns a short unique hex id not already present in the registry.
func newKeyID(reg *registry.Registry) (string, error) {
	for attempts := 0; attempts < 100; attempts++ {
		b := make([]byte, 3)
		if _, err := rand.Read(b); err != nil {
			return "", fmt.Errorf("generate key id: %w", err)
		}
		id := hex.EncodeToString(b)
		if reg.FindKey(id) == nil {
			return id, nil
		}
	}
	return "", fmt.Errorf("could not allocate a unique key id")
}

// keyTitle is the GitHub-side deploy-key title: device + id, for admin
// legibility. The local Label is deliberately NOT included.
func (a *App) keyTitle(k model.Key) string {
	return fmt.Sprintf("rig:%s:%s", a.deviceName(), k.ID)
}

// deviceName is the configured github.device, or the short, lowercased hostname
// when unset.
func (a *App) deviceName() string {
	if d := a.Config.GitHub.Device; d != "" {
		return d
	}
	return defaultDevice()
}

// defaultDevice is the first DNS label of the hostname, lowercased (e.g.
// "Andrews-MacBook-Pro.local" -> "andrews-macbook-pro").
func defaultDevice() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "rig"
	}
	return strings.ToLower(strings.SplitN(host, ".", 2)[0])
}

// repoSSHURL is the standard SSH origin URL for a repo. The specific deploy key
// is selected per-repo via the local git config (core.sshCommand), not the URL.
func repoSSHURL(repo string) string {
	return fmt.Sprintf("git@github.com:%s.git", repo)
}

// keyPath is the absolute private-key path for a key.
func (a *App) keyPath(k model.Key) string {
	return filepath.Join(a.Paths.SSHDir, k.KeyFile())
}

// mintKey allocates a new deploy key: it generates the SSH material and Host
// alias, records the key (pending) in the registry, and returns the matching
// deploy-key-add mutation for handoff. The registry is not saved here.
func (a *App) mintKey(reg *registry.Registry, repo string, write bool, label string) (*model.Key, handoff.Mutation, error) {
	id, err := newKeyID(reg)
	if err != nil {
		return nil, handoff.Mutation{}, err
	}
	k := model.Key{
		ID:    id,
		Repo:  repo,
		Write: write,
		Slug:  model.SlugForRepo(repo),
		State: model.StatePending,
		Label: label,
	}
	mat, err := a.Keygen.Generate(keygen.Request{
		SSHDir:  a.Paths.SSHDir,
		KeyFile: k.KeyFile(),
		Comment: a.keyTitle(k),
	})
	if err != nil {
		return nil, handoff.Mutation{}, err
	}
	if err := reg.AddKey(k); err != nil {
		return nil, handoff.Mutation{}, err
	}
	mut := handoff.DeployKeyAdd(repo, a.keyTitle(k), mat.PublicKey, write)
	return reg.FindKey(id), mut, nil
}

func newKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "Manage SSH deploy keys (many per repo; read/write independent)",
	}
	cmd.AddCommand(newKeyCreateCmd(), newKeyListCmd(), newKeyVerifyCmd(), newKeyDeleteCmd())
	return cmd
}

// newKeyVerifyCmd probes a deploy key over SSH and promotes it from pending to
// active. Verification is a property of the (key, repo) pair — it needs no
// checkout and works whether or not the key is bound to a project — so it lives
// here rather than on `project finish`, which short-circuits once a project is
// already active.
func newKeyVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify [id | owner/repo]",
		Short: "Probe a deploy key over SSH and promote it from pending to active",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			keys, err := app.keysToVerify(reg, args)
			if err != nil {
				return err
			}
			var failed int
			for _, k := range keys {
				if err := app.verifyKey(cmd, k); err != nil {
					failed++
					cmd.Printf("key %s (%s) for %s: still pending — %v\n", k.ID, k.Access(), k.Repo, err)
				}
			}
			if err := app.SaveRegistry(reg); err != nil {
				return err
			}
			if failed > 0 {
				return fmt.Errorf("%d of %d key(s) failed verification", failed, len(keys))
			}
			return nil
		},
	}
}

// keysToVerify resolves the verify target to one or more registry keys. With no
// argument it picks the cwd project's bound key; an owner/repo verifies every
// key for that repo; anything else is treated as a key id. The returned keys
// point into the registry so verifyKey can promote their state in place.
func (a *App) keysToVerify(reg *registry.Registry, args []string) ([]*model.Key, error) {
	if len(args) == 0 {
		tok, ok := projectTokenForCwd(a)
		if !ok {
			return nil, fmt.Errorf("not inside a project; pass a key id or owner/repo")
		}
		p, err := a.loadProject(reg, tok)
		if err != nil {
			return nil, err
		}
		if p.KeyID == "" {
			return nil, fmt.Errorf("%s has no deploy key bound", p.ID())
		}
		k := reg.FindKey(p.KeyID)
		if k == nil {
			return nil, fmt.Errorf("bound key %q not found", p.KeyID)
		}
		return []*model.Key{k}, nil
	}
	arg := args[0]
	if _, _, ok := model.ParseRepo(arg); ok {
		var out []*model.Key
		for _, k := range reg.KeysForRepo(arg) {
			if kp := reg.FindKey(k.ID); kp != nil {
				out = append(out, kp)
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("no keys registered for %s", arg)
		}
		return out, nil
	}
	k := reg.FindKey(arg)
	if k == nil {
		return nil, fmt.Errorf("key %q not found", arg)
	}
	return []*model.Key{k}, nil
}

// verifyKey probes the key's read access over SSH and, on success, marks it
// active. It is idempotent. On failure the state is left untouched so a transient
// network error never demotes a working key; the caller reports the failure.
func (a *App) verifyKey(cmd *cobra.Command, k *model.Key) error {
	if err := a.Git.LsRemote(context.Background(), "", repoSSHURL(k.Repo), a.keyPath(*k)); err != nil {
		return err
	}
	k.State = model.StateActive
	cmd.Printf("key %s (%s) verified for %s — active\n", k.ID, k.Access(), k.Repo)
	return nil
}

func newKeyCreateCmd() *cobra.Command {
	var write bool
	var label string
	cmd := &cobra.Command{
		Use:   "create <owner/repo>",
		Short: "Mint a new deploy key for a repo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			if _, _, ok := model.ParseRepo(args[0]); !ok {
				return fmt.Errorf("expected owner/repo, got %q", args[0])
			}
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			k, mut, err := app.mintKey(reg, args[0], write, label)
			if err != nil {
				return err
			}
			if err := app.SaveRegistry(reg); err != nil {
				return err
			}
			batch := handoff.Batch{Repo: args[0]}
			batch.Add(mut)
			if err := app.deliver(cmd, batch); err != nil {
				return err
			}
			// Best-effort verification: the key reads once GitHub has it.
			if app.Git.LsRemote(context.Background(), "", repoSSHURL(k.Repo), app.keyPath(*k)) == nil {
				k.State = model.StateActive
				_ = app.SaveRegistry(reg)
				cmd.Printf("key %s (%s) verified for %s\n", k.ID, k.Access(), k.Repo)
			} else {
				cmd.Printf("key %s (%s) created for %s — pending until the deploy key is added\n", k.ID, k.Access(), k.Repo)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&write, "write", false, "mint a write key (default: read)")
	cmd.Flags().StringVar(&label, "label", "", "local-only label for the picker")
	return cmd
}

func newKeyListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List deploy keys grouped by repo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			if len(reg.Keys) == 0 {
				cmd.Println("no keys")
				return nil
			}
			repos := map[string][]model.Key{}
			var order []string
			for _, k := range reg.Keys {
				if _, ok := repos[k.Repo]; !ok {
					order = append(order, k.Repo)
				}
				repos[k.Repo] = append(repos[k.Repo], k)
			}
			sort.Strings(order)
			for _, repo := range order {
				cmd.Printf("%s\n", repo)
				for _, k := range repos[repo] {
					bound := reg.ProjectsBoundToKey(k.ID)
					line := fmt.Sprintf("  %s  %-5s  %s", k.ID, k.Access(), k.State)
					if k.Label != "" {
						line += fmt.Sprintf("  label=%q", k.Label)
					}
					if len(bound) > 0 {
						names := make([]string, 0, len(bound))
						for _, p := range bound {
							names = append(names, p.ID())
						}
						line += fmt.Sprintf("  bound=%v", names)
					}
					cmd.Println(line)
				}
			}
			return nil
		},
	}
}

func newKeyDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [id]",
		Short: "Delete a deploy key (blocked while bound to a project)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			reg, err := app.Registry()
			if err != nil {
				return err
			}
			id := ""
			if len(args) == 1 {
				id = args[0]
			}
			if id == "" {
				id, err = app.pickKey(reg)
				if err != nil {
					return err
				}
			}
			k := reg.FindKey(id)
			if k == nil {
				return fmt.Errorf("key %q not found", id)
			}
			if bound := reg.ProjectsBoundToKey(k.ID); len(bound) > 0 {
				return fmt.Errorf("key %s is bound to %d project(s); rebind them first", k.ID, len(bound))
			}

			// Remove SSH artifacts and queue the GitHub-side removal.
			if err := app.Keygen.Remove(keygen.Request{
				SSHDir:  app.Paths.SSHDir,
				KeyFile: k.KeyFile(),
			}); err != nil {
				return err
			}
			batch := handoff.Batch{Repo: k.Repo}
			batch.Add(handoff.DeployKeyRemove(k.Repo, app.keyTitle(*k)))
			if err := app.deliver(cmd, batch); err != nil {
				return err
			}
			if err := reg.RemoveKey(k.ID); err != nil {
				return err
			}
			if err := app.SaveRegistry(reg); err != nil {
				return err
			}
			cmd.Printf("deleted key %s\n", k.ID)
			return nil
		},
	}
}

// pickKey prompts the user to choose a key.
func (a *App) pickKey(reg *registry.Registry) (string, error) {
	if len(reg.Keys) == 0 {
		return "", fmt.Errorf("no keys to delete")
	}
	labels := make([]string, len(reg.Keys))
	for i, k := range reg.Keys {
		labels[i] = fmt.Sprintf("%s  %s (%s)", k.ID, k.Repo, k.Access())
	}
	idx, err := a.UI.Select("Delete which key?", labels)
	if err != nil {
		return "", err
	}
	return reg.Keys[idx].ID, nil
}
