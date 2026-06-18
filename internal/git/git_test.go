package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRealRemotesAndPushGuard(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	dir := t.TempDir()
	g := New()
	if err := g.Init(ctx, dir); err != nil {
		t.Fatalf("init: %v", err)
	}

	if err := g.AddRemote(ctx, dir, "origin", "git@github.com:a/b.git"); err != nil {
		t.Fatalf("add remote: %v", err)
	}
	if err := g.SetPushURL(ctx, dir, "origin", NoPush); err != nil {
		t.Fatalf("set push url: %v", err)
	}
	remotes, err := g.Remotes(ctx, dir)
	if err != nil {
		t.Fatalf("remotes: %v", err)
	}
	if remotes["origin"].Fetch != "git@github.com:a/b.git" {
		t.Errorf("fetch url = %q", remotes["origin"].Fetch)
	}
	if remotes["origin"].Push != NoPush {
		t.Errorf("push url = %q, want %s", remotes["origin"].Push, NoPush)
	}
}

func TestRealStatus(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	dir := t.TempDir()
	g := New()
	mustRun(t, dir, "git", "init")
	mustRun(t, dir, "git", "config", "user.email", "t@example.com")
	mustRun(t, dir, "git", "config", "user.name", "t")

	st, err := g.Status(ctx, dir)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.HasCommits {
		t.Error("fresh repo should have no commits")
	}

	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, _ = g.Status(ctx, dir)
	if !st.Dirty {
		t.Error("untracked file should make repo dirty")
	}

	mustRun(t, dir, "git", "add", ".")
	mustRun(t, dir, "git", "commit", "-m", "init")
	st, _ = g.Status(ctx, dir)
	if !st.HasCommits || st.Dirty {
		t.Errorf("after commit: hasCommits=%v dirty=%v", st.HasCommits, st.Dirty)
	}
}

func TestFakeRecordsCalls(t *testing.T) {
	ctx := context.Background()
	f := NewFake()
	f.LsRemoteErr["git@github.com:a/b.git"] = nil
	_ = f.AddRemote(ctx, "/p", "origin", "git@github.com:a/b.git")
	_ = f.SetPushURL(ctx, "/p", "origin", NoPush)
	r, _ := f.Remotes(ctx, "/p")
	if r["origin"].Push != NoPush {
		t.Errorf("push = %q", r["origin"].Push)
	}
	if len(f.Calls) != 3 {
		t.Errorf("calls = %v", f.Calls)
	}
}

func mustRun(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v: %s", name, args, err, out)
	}
}
