package git

import (
	"context"
	"fmt"
)

// Fake is a scriptable in-memory Git for tests. It records each call and serves
// canned remotes, statuses, and verification errors.
type Fake struct {
	Calls []string // ordered "op arg arg" log

	RemotesByDir    map[string]map[string]Remote
	StatusByDir     map[string]Status
	SSHCommandByDir map[string]string // dir -> persisted core.sshCommand

	// LsRemoteErr keyed by URL; PushErr keyed by dir. Absent = success.
	LsRemoteErr map[string]error
	PushErr     map[string]error
}

// NewFake returns an initialized fake.
func NewFake() *Fake {
	return &Fake{
		RemotesByDir: map[string]map[string]Remote{},
		StatusByDir:  map[string]Status{},
		LsRemoteErr:  map[string]error{},
		PushErr:      map[string]error{},
	}
}

func (f *Fake) log(format string, args ...any) {
	f.Calls = append(f.Calls, fmt.Sprintf(format, args...))
}

func (f *Fake) remotes(dir string) map[string]Remote {
	if f.RemotesByDir[dir] == nil {
		f.RemotesByDir[dir] = map[string]Remote{}
	}
	return f.RemotesByDir[dir]
}

// SSHCommandByDir records the persisted core.sshCommand per repo dir.
func (f *Fake) Clone(_ context.Context, url, dest, sshKey string) error {
	f.log("clone %s %s", url, dest)
	if sshKey != "" {
		f.setSSH(dest, sshKey)
	}
	return nil
}

func (f *Fake) SetSSHCommand(_ context.Context, dir, sshKey string) error {
	f.log("set-sshcommand %s %s", dir, sshKey)
	f.setSSH(dir, sshKey)
	return nil
}

func (f *Fake) setSSH(dir, sshKey string) {
	if f.SSHCommandByDir == nil {
		f.SSHCommandByDir = map[string]string{}
	}
	f.SSHCommandByDir[dir] = sshKey
}

func (f *Fake) Init(_ context.Context, dir string) error {
	f.log("init %s", dir)
	return nil
}

func (f *Fake) Remotes(_ context.Context, dir string) (map[string]Remote, error) {
	f.log("remotes %s", dir)
	out := map[string]Remote{}
	for k, v := range f.remotes(dir) {
		out[k] = v
	}
	return out, nil
}

func (f *Fake) AddRemote(_ context.Context, dir, name, url string) error {
	f.log("add-remote %s %s %s", dir, name, url)
	f.remotes(dir)[name] = Remote{Fetch: url, Push: url}
	return nil
}

func (f *Fake) RemoveRemote(_ context.Context, dir, name string) error {
	f.log("remove-remote %s %s", dir, name)
	delete(f.remotes(dir), name)
	return nil
}

func (f *Fake) SetRemoteURL(_ context.Context, dir, name, url string) error {
	f.log("set-url %s %s %s", dir, name, url)
	f.remotes(dir)[name] = Remote{Fetch: url, Push: url}
	return nil
}

func (f *Fake) SetPushURL(_ context.Context, dir, name, url string) error {
	f.log("set-push-url %s %s %s", dir, name, url)
	r := f.remotes(dir)[name]
	r.Push = url
	f.remotes(dir)[name] = r
	return nil
}

func (f *Fake) LsRemote(_ context.Context, dir, url, sshKey string) error {
	f.log("ls-remote %s", url)
	return f.LsRemoteErr[url]
}

func (f *Fake) PushDryRun(_ context.Context, dir, remote string) error {
	f.log("push-dry-run %s %s", dir, remote)
	return f.PushErr[dir]
}

func (f *Fake) Status(_ context.Context, dir string) (Status, error) {
	f.log("status %s", dir)
	return f.StatusByDir[dir], nil
}
