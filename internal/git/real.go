package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Exec is the real Git, shelling out to the git binary.
type Exec struct{}

// New returns the real git client.
func New() Exec { return Exec{} }

// run executes git in dir (empty dir = current) and returns trimmed stdout.
func (e Exec) run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (e Exec) Clone(ctx context.Context, url, dest string) error {
	_, err := e.run(ctx, "", "clone", url, dest)
	return err
}

func (e Exec) Init(ctx context.Context, dir string) error {
	_, err := e.run(ctx, "", "init", dir)
	return err
}

func (e Exec) Remotes(ctx context.Context, dir string) (map[string]Remote, error) {
	out, err := e.run(ctx, dir, "remote", "-v")
	if err != nil {
		return nil, err
	}
	remotes := map[string]Remote{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		name, url, kind := fields[0], fields[1], fields[2]
		r := remotes[name]
		switch kind {
		case "(fetch)":
			r.Fetch = url
		case "(push)":
			r.Push = url
		}
		remotes[name] = r
	}
	return remotes, nil
}

func (e Exec) AddRemote(ctx context.Context, dir, name, url string) error {
	_, err := e.run(ctx, dir, "remote", "add", name, url)
	return err
}

func (e Exec) RemoveRemote(ctx context.Context, dir, name string) error {
	_, err := e.run(ctx, dir, "remote", "remove", name)
	return err
}

func (e Exec) SetRemoteURL(ctx context.Context, dir, name, url string) error {
	_, err := e.run(ctx, dir, "remote", "set-url", name, url)
	return err
}

func (e Exec) SetPushURL(ctx context.Context, dir, name, url string) error {
	_, err := e.run(ctx, dir, "remote", "set-url", "--push", name, url)
	return err
}

func (e Exec) LsRemote(ctx context.Context, url string) error {
	_, err := e.run(ctx, "", "ls-remote", url)
	return err
}

func (e Exec) PushDryRun(ctx context.Context, dir, remote string) error {
	_, err := e.run(ctx, dir, "push", "--dry-run", remote)
	return err
}

func (e Exec) Status(ctx context.Context, dir string) (Status, error) {
	var s Status

	// branch --show-current works before the first commit (unlike rev-parse
	// HEAD) and returns empty on a detached HEAD.
	branch, err := e.run(ctx, dir, "branch", "--show-current")
	if err != nil {
		return s, err
	}
	s.Branch = branch

	// A repo with no commits has no resolvable HEAD object.
	if _, err := e.run(ctx, dir, "rev-parse", "--verify", "-q", "HEAD"); err == nil {
		s.HasCommits = true
	}

	porcelain, err := e.run(ctx, dir, "status", "--porcelain")
	if err != nil {
		return s, err
	}
	s.Dirty = porcelain != ""

	// Upstream is optional; absence is not an error.
	if up, err := e.run(ctx, dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); err == nil {
		s.Upstream = up
		if counts, err := e.run(ctx, dir, "rev-list", "--left-right", "--count", "@{u}...HEAD"); err == nil {
			fields := strings.Fields(counts)
			if len(fields) == 2 {
				s.Behind, _ = strconv.Atoi(fields[0])
				s.Ahead, _ = strconv.Atoi(fields[1])
			}
		}
	}
	return s, nil
}
