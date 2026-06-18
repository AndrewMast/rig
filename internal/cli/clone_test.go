package cli

import (
	"strings"
	"testing"

	"github.com/AndrewMast/rig/internal/git"
	"github.com/AndrewMast/rig/internal/handoff"
	"github.com/AndrewMast/rig/internal/model"
)

func TestClonePublic(t *testing.T) {
	// stdin: group, then project name (accept default repo name).
	ta := newTestApp(t, "Acme\n\n")
	out, err := ta.run(t, "clone", "--public", "AndrewMast/widget")
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	p := ta.reg(t).FindProject("Acme", "widget")
	if p == nil || p.Strategy != model.StrategyPublic {
		t.Fatalf("project = %+v", p)
	}
	if p.State != model.StateActive {
		t.Errorf("public clone should be active, got %s", p.State)
	}
	// Cloned over HTTPS, no key minted.
	if len(ta.keyFake.Generated) != 0 {
		t.Errorf("public clone should mint no keys, got %d", len(ta.keyFake.Generated))
	}
	if !hasCall(ta.gitFake, "clone https://github.com/AndrewMast/widget.git") {
		t.Errorf("expected https clone; calls=%v", ta.gitFake.Calls)
	}
	_ = out
}

func TestCloneWriteFreshKeyPendingThenFinish(t *testing.T) {
	ta := newTestApp(t, "Acme\n\n")
	out, err := ta.run(t, "clone", "AndrewMast/widget")
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	if !strings.Contains(out, "pending") {
		t.Errorf("fresh-key clone should be pending: %q", out)
	}
	p := ta.reg(t).FindProject("Acme", "widget")
	if p.Strategy != model.StrategyDeployKey || p.State != model.StatePending || p.KeyID == "" {
		t.Fatalf("project = %+v", p)
	}
	if len(ta.keyFake.Generated) != 1 {
		t.Fatalf("expected one minted key, got %d", len(ta.keyFake.Generated))
	}

	// Finish verifies over SSH (fake returns success) and activates.
	if _, err := ta.run(t, "project", "finish", "Acme/widget"); err != nil {
		t.Fatalf("finish: %v", err)
	}
	p = ta.reg(t).FindProject("Acme", "widget")
	if p.State != model.StateActive {
		t.Errorf("after finish state = %s, want active", p.State)
	}
	k := ta.reg(t).FindKey(p.KeyID)
	if k.State != model.StateActive {
		t.Errorf("key should be active after finish, got %s", k.State)
	}
}

func TestCloneReadIsPushGuarded(t *testing.T) {
	ta := newTestApp(t, "Acme\n\n")
	if _, err := ta.run(t, "clone", "--read", "AndrewMast/widget"); err != nil {
		t.Fatalf("clone: %v", err)
	}
	p := ta.reg(t).FindProject("Acme", "widget")
	if !p.Guard {
		t.Error("read clone should be push-guarded")
	}
	k := ta.reg(t).FindKey(p.KeyID)
	if k.Write {
		t.Error("read clone should mint a read key")
	}
	// Finish should set the no_push sentinel on origin's push URL.
	if _, err := ta.run(t, "project", "finish", "Acme/widget"); err != nil {
		t.Fatalf("finish: %v", err)
	}
	g := ta.reg(t).FindGroup("Acme")
	path := p.Path(*g)
	if ta.gitFake.RemotesByDir[path]["origin"].Push != git.NoPush {
		t.Errorf("push URL = %q, want no_push", ta.gitFake.RemotesByDir[path]["origin"].Push)
	}
}

func TestCloneWriteReusesExistingWriteKey(t *testing.T) {
	ta := newTestApp(t, "Acme\n\n")
	// Pre-seed an active write key for the repo.
	reg := ta.reg(t)
	reg.AddKey(model.Key{ID: "abc123", Repo: "AndrewMast/widget", Write: true, Slug: "andrewmast-widget", State: model.StateActive})
	if err := ta.SaveRegistry(reg); err != nil {
		t.Fatal(err)
	}
	out, err := ta.run(t, "clone", "AndrewMast/widget")
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	p := ta.reg(t).FindProject("Acme", "widget")
	if p.KeyID != "abc123" {
		t.Errorf("should reuse existing write key, got %q", p.KeyID)
	}
	if len(ta.keyFake.Generated) != 0 {
		t.Errorf("should not mint a new key, minted %d", len(ta.keyFake.Generated))
	}
	// Reused active key → clone immediately, active (not pending).
	if p.State != model.StateActive {
		t.Errorf("state = %s, want active", p.State)
	}
	_ = out
}

func TestCloneGHDirectAutoFinishes(t *testing.T) {
	ta := newTestApp(t, "Acme\n\n")
	ta.Config.Handoff.Method = "gh"
	var ran []string
	ta.envOverride = &handoff.Env{Run: func(c string) error { ran = append(ran, c); return nil }}

	if _, err := ta.run(t, "clone", "AndrewMast/widget"); err != nil {
		t.Fatalf("clone: %v", err)
	}
	if len(ran) == 0 {
		t.Error("gh-direct should run the deploy-key mutation")
	}
	p := ta.reg(t).FindProject("Acme", "widget")
	if p.State != model.StateActive {
		t.Errorf("gh-direct clone should auto-finish to active, got %s", p.State)
	}
}

func TestCloneOffersPublicAndSkipsKey(t *testing.T) {
	// stdin: group, name (default), then accept the "clone over HTTPS?" offer.
	ta := newTestApp(t, "Acme\n\ny\n")
	ta.ghFake.PublicRepo["AndrewMast/widget"] = true

	if _, err := ta.run(t, "clone", "AndrewMast/widget"); err != nil {
		t.Fatalf("clone: %v", err)
	}
	p := ta.reg(t).FindProject("Acme", "widget")
	if p.Strategy != model.StrategyPublic {
		t.Errorf("accepting the offer should switch to public, got %s", p.Strategy)
	}
	if len(ta.keyFake.Generated) != 0 {
		t.Errorf("public clone should mint no key, minted %d", len(ta.keyFake.Generated))
	}
	if !hasCall(ta.gitFake, "clone https://github.com/AndrewMast/widget.git") {
		t.Errorf("expected HTTPS clone; calls=%v", ta.gitFake.Calls)
	}
}

func TestCloneReadDeclinePublicKeepsDeployKey(t *testing.T) {
	// Public repo, but the user declines the offer and keeps the read key.
	ta := newTestApp(t, "Acme\n\nn\n")
	ta.ghFake.PublicRepo["AndrewMast/widget"] = true

	if _, err := ta.run(t, "clone", "--read", "AndrewMast/widget"); err != nil {
		t.Fatalf("clone: %v", err)
	}
	p := ta.reg(t).FindProject("Acme", "widget")
	if p.Strategy != model.StrategyDeployKey {
		t.Errorf("declining should keep deploy-key, got %s", p.Strategy)
	}
	if len(ta.keyFake.Generated) != 1 {
		t.Errorf("expected a read key to be minted, got %d", len(ta.keyFake.Generated))
	}
	if !p.Guard {
		t.Error("read clone should remain push-guarded")
	}
}

func TestClonePrivateRepoNoOffer(t *testing.T) {
	// Not public → no offer, mints a key as usual (no extra stdin needed).
	ta := newTestApp(t, "Acme\n\n")
	if _, err := ta.run(t, "clone", "AndrewMast/widget"); err != nil {
		t.Fatalf("clone: %v", err)
	}
	if len(ta.keyFake.Generated) != 1 {
		t.Errorf("private clone should mint a key, got %d", len(ta.keyFake.Generated))
	}
}

func hasCall(f *git.Fake, prefix string) bool {
	for _, c := range f.Calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}
