// Package gh is the GitHub metadata IO seam: optional, read-only discovery used
// to decide create-vs-attach and to auto-detect owners. It NEVER mutates and
// NEVER verifies key liveness — that is always git-over-SSH. Repo mutations are
// the handoff package's job.
package gh

import (
	"context"
	"os"
	"strings"
)

// GH is read-only GitHub metadata access.
type GH interface {
	// Available reports whether a usable token is configured. When false,
	// callers must fall back to asking for owner/repo directly.
	Available() bool
	// Verify probes the API to confirm the token works.
	Verify(ctx context.Context) error
	// Login returns the authenticated user's login (owner auto-detection).
	Login(ctx context.Context) (string, error)
	// RepoExists reports whether owner/repo exists and is visible.
	RepoExists(ctx context.Context, owner, repo string) (bool, error)
}

// LoadToken returns the metadata token: RIG_GH_TOKEN wins, else the trimmed
// contents of the token file. Returns "" when neither is present.
func LoadToken(file string) string {
	if env := strings.TrimSpace(os.Getenv("RIG_GH_TOKEN")); env != "" {
		return env
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
