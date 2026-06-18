package cli

import (
	"strings"
	"testing"

	"github.com/AndrewMast/rig/internal/handoff"
	"github.com/AndrewMast/rig/internal/model"
)

func TestOriginAddDemotesReadOnlySourceToUpstream(t *testing.T) {
	// A read-only clone gains a new writable origin; the source is demoted.
	ta := newTestApp(t, "Acme\n\n")
	if _, err := ta.run(t, "clone", "--read", "AndrewMast/widget"); err != nil {
		t.Fatal(err)
	}

	ta.Config.Handoff.Method = "gh"
	var ran []string
	ta.envOverride = &handoff.Env{Run: func(c string) error { ran = append(ran, c); return nil }}

	// origin add <g/n> <owner/repo>: prefilled repo, so prompts are just the
	// create-confirm and demote-confirm (both default yes via empty stdin).
	out, err := ta.run(t, "project", "origin", "add", "Acme/widget", "AndrewMast/widget-fork")
	if err != nil {
		t.Fatalf("origin add: %v", err)
	}
	if !strings.Contains(out, "demoted AndrewMast/widget to upstream") {
		t.Errorf("expected demote message, got: %q", out)
	}

	p := ta.reg(t).FindProject("Acme", "widget")
	if p.Repo != "AndrewMast/widget-fork" {
		t.Errorf("new origin = %q, want AndrewMast/widget-fork", p.Repo)
	}
	if p.Upstream != "AndrewMast/widget" {
		t.Errorf("upstream = %q, want AndrewMast/widget (demoted source)", p.Upstream)
	}
	if p.Guard {
		t.Error("a writable origin should not be push-guarded")
	}
	if p.Strategy != model.StrategyDeployKey {
		t.Errorf("strategy = %q", p.Strategy)
	}

	// The upstream git remote should point at the old source.
	g := ta.reg(t).FindGroup("Acme")
	remotes := ta.gitFake.RemotesByDir[p.Path(*g)]
	if _, ok := remotes["upstream"]; !ok {
		t.Errorf("upstream remote not wired; remotes=%v", remotes)
	}
}

func TestOriginAddRejectsSameRepo(t *testing.T) {
	ta := newTestApp(t, "Acme\n\n")
	reg := ta.reg(t)
	reg.AddKey(model.Key{ID: "abc123", Repo: "AndrewMast/widget", Write: true, Slug: "andrewmast-widget", State: model.StateActive})
	ta.SaveRegistry(reg)
	if _, err := ta.run(t, "clone", "AndrewMast/widget"); err != nil {
		t.Fatal(err)
	}
	// Adding the same repo it already has should be rejected.
	_, err := ta.run(t, "project", "origin", "add", "Acme/widget", "AndrewMast/widget")
	if err == nil || !strings.Contains(err.Error(), "already has origin") {
		t.Errorf("expected same-repo rejection, got %v", err)
	}
}

func TestFuzzyNavFallbackPrintsPath(t *testing.T) {
	ta := newTestApp(t, "")
	if _, err := ta.run(t, "create", "Acme/widget"); err != nil {
		t.Fatal(err)
	}
	out, err := ta.run(t, "widget") // not a command → fuzzy nav
	if err != nil {
		t.Fatalf("fuzzy nav: %v", err)
	}
	g := ta.reg(t).FindGroup("Acme")
	p := ta.reg(t).FindProject("Acme", "widget")
	if !strings.Contains(out, p.Path(*g)) {
		t.Errorf("expected resolved path in output, got %q", out)
	}
}

func TestFuzzyNavUnknownErrors(t *testing.T) {
	ta := newTestApp(t, "")
	_, err := ta.run(t, "zzz-nonexistent")
	if err == nil || !strings.Contains(err.Error(), "not a command") {
		t.Errorf("expected unknown-token error, got %v", err)
	}
}
