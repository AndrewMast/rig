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
	Clone(ctx context.Context, url, dest string) error
	Init(ctx context.Context, dir string) error

	Remotes(ctx context.Context, dir string) (map[string]Remote, error)
	AddRemote(ctx context.Context, dir, name, url string) error
	RemoveRemote(ctx context.Context, dir, name string) error
	// SetRemoteURL sets both fetch and push URLs for a remote.
	SetRemoteURL(ctx context.Context, dir, name, url string) error
	// SetPushURL sets only the push URL (used for the no_push guard).
	SetPushURL(ctx context.Context, dir, name, url string) error

	// LsRemote succeeds when the remote is reachable and readable over SSH —
	// proving the repo exists and the deploy key reads.
	LsRemote(ctx context.Context, url string) error
	// PushDryRun succeeds when the remote grants write access.
	PushDryRun(ctx context.Context, dir, remote string) error

	Status(ctx context.Context, dir string) (Status, error)
}
