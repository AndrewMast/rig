// Package model holds rig's pure domain types: groups, projects, and keys.
//
// These types carry no IO. Paths are derived, identity rules live here, and
// everything is trivially testable. Persistence is the registry's job; behavior
// that needs the filesystem, git, or GitHub lives behind the IO interfaces.
package model

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Strategy describes how a project talks to its remote.
type Strategy string

const (
	// StrategyLocal is a project with no remote at all (local-only git).
	StrategyLocal Strategy = "local"
	// StrategyDeployKey uses an SSH deploy key (read or write).
	StrategyDeployKey Strategy = "deploy-key"
	// StrategyPublic uses plain HTTPS against a genuinely public repo, no key.
	StrategyPublic Strategy = "public"
)

// State tracks where a project is in the handoff/verify lifecycle.
type State string

const (
	// StatePending means the GitHub mutations have been emitted but not yet
	// verified by git-over-SSH; `rig project finish` completes it.
	StatePending State = "pending"
	// StateActive means the project is fully wired and verified.
	StateActive State = "active"
)

// Group is a first-class named wrapper around a set of projects. The group name
// is the folder name (capitalization preserved); the group owns the base path,
// and every project's path is derived beneath it.
type Group struct {
	Name    string   `json:"name"`
	Base    string   `json:"base"`
	Aliases []string `json:"aliases,omitempty"`
}

// Path is the absolute group folder: <base>/<name>.
func (g Group) Path() string {
	return filepath.Join(g.Base, g.Name)
}

// Project is a managed local git checkout. Identity is the (Group, Name) pair;
// the path is always derived and never stored independently.
type Project struct {
	Group    string   `json:"group"`
	Name     string   `json:"name"`
	Type     string   `json:"type,omitempty"`
	Strategy Strategy `json:"strategy"`
	State    State    `json:"state"`

	// Repo is the origin "owner/repo" when hosted; empty for local-only.
	Repo string `json:"repo,omitempty"`
	// Upstream is an optional "owner/repo" PR-back source (fork-like).
	Upstream string `json:"upstream,omitempty"`
	// KeyID binds this project to a deploy key in the registry (empty when
	// local-only or public).
	KeyID string `json:"key_id,omitempty"`
	// Guard records the per-project push guard (the no_push sentinel). It
	// defaults to tracking the key's access but is independently settable.
	Guard bool `json:"guard,omitempty"`

	Aliases []string `json:"aliases,omitempty"`
}

// ID is the canonical "Group/Name" identity string.
func (p Project) ID() string {
	return p.Group + "/" + p.Name
}

// Path derives the absolute checkout path: <group.base>/<group.name>/<name>.
func (p Project) Path(g Group) string {
	return filepath.Join(g.Path(), p.Name)
}

// Hosted reports whether the project has a GitHub origin.
func (p Project) Hosted() bool {
	return p.Repo != "" && p.Strategy != StrategyLocal
}

// Key is a per-repo SSH deploy key. Many keys may exist per repo; read and write
// are independent key objects (not a toggled flag). The private key lives in
// ~/.ssh and never appears here.
type Key struct {
	ID    string `json:"id"`
	Repo  string `json:"repo"`
	Write bool   `json:"write"`
	Slug  string `json:"slug"`
	State State  `json:"state"`
	Label string `json:"label,omitempty"`
}

// Access renders the key's access level as a word.
func (k Key) Access() string {
	if k.Write {
		return "write"
	}
	return "read"
}

// KeyFile is the SSH private-key filename, derived from slug + ID so artifacts
// for different keys on the same repo never collide.
func (k Key) KeyFile() string {
	return fmt.Sprintf("project_%s_%s_deploy", k.Slug, k.ID)
}

// HostAlias is the ssh config Host alias used by this key's origin URL.
func (k Key) HostAlias() string {
	return fmt.Sprintf("github.com-%s-%s", k.Slug, k.ID)
}

// SlugForRepo turns "owner/repo" into a filesystem/host-safe slug.
func SlugForRepo(repo string) string {
	s := strings.ToLower(repo)
	repl := func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}
	s = strings.Map(repl, s)
	// Collapse runs of '-' and trim.
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

// ParseRepo splits "owner/repo" into its parts, tolerating a trailing ".git"
// and surrounding whitespace.
func ParseRepo(s string) (owner, repo string, ok bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, ".git")
	parts := strings.Split(s, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
