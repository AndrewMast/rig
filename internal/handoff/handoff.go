// Package handoff models the GitHub mutations rig produces (repo create, deploy
// key add/remove) and delivers them by a pluggable method (clipboard, drop,
// link, print, file, or run directly via gh). Mutation construction is pure and
// testable; delivery side effects go through an injected Env so tests can fake
// the clipboard, opener, and command runner.
package handoff

import (
	"fmt"
	"strings"
)

// Mutation is one GitHub change rig wants applied, expressed as a runnable gh
// command plus metadata for non-command delivery methods (link).
type Mutation struct {
	Kind      string // repo-create | deploy-key-add | deploy-key-remove
	Title     string // human-readable summary
	Command   string // gh command line that performs the mutation
	WebURL    string // page to open for the manual (link) method
	PublicKey string // public key to copy for the manual (link) method
}

// Batch is the set of mutations for one repo.
type Batch struct {
	Repo      string
	Mutations []Mutation
}

// Add appends a mutation.
func (b *Batch) Add(m Mutation) { b.Mutations = append(b.Mutations, m) }

// Commands returns the gh command lines, in order.
func (b Batch) Commands() []string {
	out := make([]string, 0, len(b.Mutations))
	for _, m := range b.Mutations {
		if m.Command != "" {
			out = append(out, m.Command)
		}
	}
	return out
}

// RepoCreate builds a repo-creation mutation.
func RepoCreate(repo string, private bool) Mutation {
	vis := "--public"
	if private {
		vis = "--private"
	}
	return Mutation{
		Kind:    "repo-create",
		Title:   fmt.Sprintf("create repo %s", repo),
		Command: fmt.Sprintf("gh repo create %s %s", repo, vis),
		WebURL:  "https://github.com/new",
	}
}

// DeployKeyAdd builds a mutation that adds a deploy key, inlining the public key
// via gh api. read_only is the inverse of write.
func DeployKeyAdd(repo, title, publicKey string, write bool) Mutation {
	readOnly := "true"
	if write {
		readOnly = "false"
	}
	cmd := fmt.Sprintf(
		"gh api -X POST repos/%s/keys -f title=%s -f key=%s -F read_only=%s",
		repo, shellQuote(title), shellQuote(publicKey), readOnly,
	)
	return Mutation{
		Kind:      "deploy-key-add",
		Title:     fmt.Sprintf("add deploy key %q (%s) to %s", title, accessWord(write), repo),
		Command:   cmd,
		WebURL:    fmt.Sprintf("https://github.com/%s/settings/keys/new", repo),
		PublicKey: publicKey,
	}
}

// DeployKeyRemove builds a mutation that deletes a deploy key resolved by title.
func DeployKeyRemove(repo, title string) Mutation {
	cmd := fmt.Sprintf(
		"gh api repos/%s/keys --jq %s | xargs -I{} gh api -X DELETE repos/%s/keys/{}",
		repo, shellQuote(fmt.Sprintf(".[] | select(.title==%q) | .id", title)), repo,
	)
	return Mutation{
		Kind:    "deploy-key-remove",
		Title:   fmt.Sprintf("remove deploy key %q from %s", title, repo),
		Command: cmd,
		WebURL:  fmt.Sprintf("https://github.com/%s/settings/keys", repo),
	}
}

func accessWord(write bool) string {
	if write {
		return "write"
	}
	return "read"
}

// shellQuote single-quotes a string for safe embedding in a shell command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
