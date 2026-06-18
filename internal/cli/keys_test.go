package cli

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/AndrewMast/rig/internal/handoff"
	"github.com/AndrewMast/rig/internal/model"
)

func TestOriginAddHostsLocalProjectGHDirect(t *testing.T) {
	// create local, then host it. stdin: owner, repo name (default), confirm.
	ta := newTestApp(t, "AndrewMast\n\n\n")
	ta.Config.Handoff.Method = "gh"
	var ran []string
	ta.envOverride = &handoff.Env{Run: func(c string) error { ran = append(ran, c); return nil }}

	if _, err := ta.run(t, "create", "Acme/widget"); err != nil {
		t.Fatal(err)
	}
	if _, err := ta.run(t, "project", "origin", "add", "Acme/widget"); err != nil {
		t.Fatalf("origin add: %v", err)
	}
	p := ta.reg(t).FindProject("Acme", "widget")
	if p.Strategy != model.StrategyDeployKey || p.Repo != "AndrewMast/widget" {
		t.Fatalf("project = %+v", p)
	}
	if p.State != model.StateActive {
		t.Errorf("gh-direct host should be active, got %s", p.State)
	}
	// Should have run a repo-create and a deploy-key-add.
	joined := strings.Join(ran, "\n")
	if !strings.Contains(joined, "repo create") || !strings.Contains(joined, "/keys") {
		t.Errorf("expected repo-create + deploy-key-add, got %v", ran)
	}
}

func TestOriginRemoveUnhosts(t *testing.T) {
	ta := newTestApp(t, "Acme\n\n")
	reg := ta.reg(t)
	reg.AddKey(model.Key{ID: "abc123", Repo: "AndrewMast/widget", Write: true, Slug: "andrewmast-widget", State: model.StateActive})
	ta.SaveRegistry(reg)
	if _, err := ta.run(t, "clone", "AndrewMast/widget"); err != nil {
		t.Fatal(err)
	}
	if _, err := ta.run(t, "project", "origin", "remove", "Acme/widget"); err != nil {
		t.Fatalf("origin remove: %v", err)
	}
	p := ta.reg(t).FindProject("Acme", "widget")
	if p.Strategy != model.StrategyLocal || p.Repo != "" || p.KeyID != "" {
		t.Errorf("project not unhosted: %+v", p)
	}
}

func TestKeyDeleteBlockedWhileBound(t *testing.T) {
	ta := newTestApp(t, "Acme\n\n")
	reg := ta.reg(t)
	reg.AddKey(model.Key{ID: "abc123", Repo: "AndrewMast/widget", Write: true, Slug: "andrewmast-widget", State: model.StateActive})
	ta.SaveRegistry(reg)
	if _, err := ta.run(t, "clone", "AndrewMast/widget"); err != nil {
		t.Fatal(err)
	}
	_, err := ta.run(t, "key", "delete", "abc123")
	if err == nil || !strings.Contains(err.Error(), "bound") {
		t.Errorf("expected bound-key delete to be blocked, got %v", err)
	}
}

func TestKeyCreateAndList(t *testing.T) {
	ta := newTestApp(t, "")
	if _, err := ta.run(t, "key", "create", "AndrewMast/widget", "--write", "--label", "laptop"); err != nil {
		t.Fatalf("key create: %v", err)
	}
	keys := ta.reg(t).KeysForRepo("AndrewMast/widget")
	if len(keys) != 1 || !keys[0].Write || keys[0].Label != "laptop" {
		t.Fatalf("keys = %+v", keys)
	}
	out, _ := ta.run(t, "key", "list")
	if !strings.Contains(out, "AndrewMast/widget") || !strings.Contains(out, "write") {
		t.Errorf("key list: %q", out)
	}
}

func TestKeyVerifyPromotesPending(t *testing.T) {
	ta := newTestApp(t, "")
	reg := ta.reg(t)
	reg.AddKey(model.Key{ID: "abc123", Repo: "AndrewMast/widget", Slug: "andrewmast-widget", State: model.StatePending})
	ta.SaveRegistry(reg)

	out, err := ta.run(t, "key", "verify", "abc123")
	if err != nil {
		t.Fatalf("key verify: %v", err)
	}
	if !strings.Contains(out, "active") {
		t.Errorf("output = %q, want it to mention active", out)
	}
	if got := ta.reg(t).FindKey("abc123").State; got != model.StateActive {
		t.Errorf("state = %s, want active", got)
	}
}

func TestKeyVerifyByRepoPromotesEveryKey(t *testing.T) {
	ta := newTestApp(t, "")
	reg := ta.reg(t)
	reg.AddKey(model.Key{ID: "read01", Repo: "AndrewMast/widget", Slug: "andrewmast-widget", State: model.StatePending})
	reg.AddKey(model.Key{ID: "write1", Repo: "AndrewMast/widget", Write: true, Slug: "andrewmast-widget", State: model.StatePending})
	ta.SaveRegistry(reg)

	if _, err := ta.run(t, "key", "verify", "AndrewMast/widget"); err != nil {
		t.Fatalf("key verify: %v", err)
	}
	for _, id := range []string{"read01", "write1"} {
		if got := ta.reg(t).FindKey(id).State; got != model.StateActive {
			t.Errorf("key %s state = %s, want active", id, got)
		}
	}
}

func TestKeyVerifyFailureLeavesPending(t *testing.T) {
	ta := newTestApp(t, "")
	reg := ta.reg(t)
	reg.AddKey(model.Key{ID: "abc123", Repo: "AndrewMast/widget", Slug: "andrewmast-widget", State: model.StatePending})
	ta.SaveRegistry(reg)
	// The remote rejects the key (not added to GitHub yet).
	ta.gitFake.LsRemoteErr["git@github.com:AndrewMast/widget.git"] = fmt.Errorf("permission denied")

	_, err := ta.run(t, "key", "verify", "abc123")
	if err == nil {
		t.Fatal("expected verification to fail")
	}
	if got := ta.reg(t).FindKey("abc123").State; got != model.StatePending {
		t.Errorf("state = %s, want pending (failure must not promote)", got)
	}
}

func TestKeyTitleUsesConfiguredDevice(t *testing.T) {
	ta := newTestApp(t, "")
	ta.Config.GitHub.Device = "andrew-laptop"
	got := ta.keyTitle(model.Key{ID: "a1b2c3", Repo: "AndrewMast/gadget"})
	if want := "rig:andrew-laptop:a1b2c3"; got != want {
		t.Errorf("keyTitle = %q, want %q", got, want)
	}
}

func TestDefaultDeviceIsShortLowercase(t *testing.T) {
	d := defaultDevice()
	if d == "" || strings.Contains(d, ".") || d != strings.ToLower(d) {
		t.Errorf("defaultDevice = %q, want short lowercased host (no domain)", d)
	}
}

func TestProjectKeyDefaultsToCwd(t *testing.T) {
	// stdin: clone group name + confirm, then an empty line selecting the first
	// (existing) key in `project key`'s picker.
	ta := newTestApp(t, "Acme\n\n\n")
	reg := ta.reg(t)
	reg.AddKey(model.Key{ID: "abc123", Repo: "AndrewMast/widget", Write: true, Slug: "andrewmast-widget", State: model.StateActive})
	ta.SaveRegistry(reg)
	if _, err := ta.run(t, "clone", "AndrewMast/widget"); err != nil {
		t.Fatal(err)
	}

	reg = ta.reg(t)
	p := reg.FindProject("Acme", "widget")
	dir := p.Path(*reg.FindGroup("Acme"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	// No group/name argument: it should resolve the project from the cwd.
	if _, err := ta.run(t, "project", "key"); err != nil {
		t.Fatalf("project key (cwd): %v", err)
	}
	if got := ta.reg(t).FindProject("Acme", "widget").KeyID; got != "abc123" {
		t.Errorf("bound key = %q, want abc123", got)
	}
}

func TestProjectKeyNoArgsOutsideProjectErrors(t *testing.T) {
	ta := newTestApp(t, "")
	_, err := ta.run(t, "project", "key")
	if err == nil || !strings.Contains(err.Error(), "not inside a project") {
		t.Errorf("expected not-inside-a-project error, got %v", err)
	}
}

func TestProjectGuardToggle(t *testing.T) {
	ta := newTestApp(t, "Acme\n\n")
	reg := ta.reg(t)
	reg.AddKey(model.Key{ID: "abc123", Repo: "AndrewMast/widget", Write: true, Slug: "andrewmast-widget", State: model.StateActive})
	ta.SaveRegistry(reg)
	if _, err := ta.run(t, "clone", "AndrewMast/widget"); err != nil {
		t.Fatal(err)
	}
	if _, err := ta.run(t, "project", "guard", "Acme/widget", "on"); err != nil {
		t.Fatalf("guard on: %v", err)
	}
	if !ta.reg(t).FindProject("Acme", "widget").Guard {
		t.Error("guard should be on")
	}
	if _, err := ta.run(t, "project", "guard", "Acme/widget", "off"); err != nil {
		t.Fatalf("guard off: %v", err)
	}
	if ta.reg(t).FindProject("Acme", "widget").Guard {
		t.Error("guard should be off")
	}
}
