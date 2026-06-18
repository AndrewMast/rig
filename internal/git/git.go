// Package git is the git IO seam. Real operations shell out to the git binary;
// a fake drives commands in tests. Crucially, remote verification is always
// git-over-SSH here (ls-remote reads, dry-run push for write) — the GitHub
// metadata token never substitutes for it.
package git

import "context"

// NoPush is the sentinel push URL used to guard a read-only clone, matching
// `git remote set-url --push origin no_push`.
const NoPush = "no_push"

// Remote is one named remote's fetch and push URLs.
type Remote struct {
	Fetch string
	Push  string
}

// Status is a snapshot of a working tree relative to its upstream.
type Status struct {
	Branch     string
	HasCommits bool
	Dirty      bool   // uncommitted changes present
	Upstream   string // tracking ref, empty if none
	Ahead      int    // commits ahead of upstream
	Behind     int    // commits behind upstream
}

// Git is the set of git operations rig needs.
type Git interface {
	// Clone clones url into dest. When sshKey is non-empty, the clone uses that
	// deploy key and persists it as the new repo's local core.sshCommand.
	Clone(ctx context.Context, url, dest, sshKey string) error
	Init(ctx context.Context, dir string) error

	Remotes(ctx context.Context, dir string) (map[string]Remote, error)
	AddRemote(ctx context.Context, dir, name, url string) error
	RemoveRemote(ctx context.Context, dir, name string) error
	// SetRemoteURL sets both fetch and push URLs for a remote.
	SetRemoteURL(ctx context.Context, dir, name, url string) error
	// SetPushURL sets only the push URL (used for the no_push guard).
	SetPushURL(ctx context.Context, dir, name, url string) error
	// SetSSHCommand persists (or, with an empty key, clears) the repo's local
	// core.sshCommand so all of its git traffic uses the bound deploy key.
	SetSSHCommand(ctx context.Context, dir, sshKey string) error

	// LsRemote succeeds when the remote is reachable and readable over SSH —
	// proving the repo exists and the deploy key reads. It runs in dir (so a
	// repo's local core.sshCommand applies) and, when sshKey is set, also via
	// GIT_SSH_COMMAND for contexts with no checkout yet (dir == "").
	LsRemote(ctx context.Context, dir, url, sshKey string) error
	// PushDryRun succeeds when the remote grants write access.
	PushDryRun(ctx context.Context, dir, remote string) error

	Status(ctx context.Context, dir string) (Status, error)
}

// SSHCommand builds the GIT_SSH_COMMAND / core.sshCommand value that pins git to
// a single deploy key.
func SSHCommand(keyPath string) string {
	return "ssh -i " + keyPath + " -o IdentitiesOnly=yes"
}
