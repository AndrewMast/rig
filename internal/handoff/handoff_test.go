package handoff

import (
	"bytes"
	"strings"
	"testing"
)

func TestDeployKeyAddCommand(t *testing.T) {
	m := DeployKeyAdd("AndrewMast/dripstone", "rig host abc", "ssh-ed25519 AAAA test", false)
	if !strings.Contains(m.Command, "repos/AndrewMast/dripstone/keys") {
		t.Errorf("command missing repo path: %s", m.Command)
	}
	if !strings.Contains(m.Command, "read_only=true") {
		t.Errorf("read-only key should set read_only=true: %s", m.Command)
	}
	if !strings.Contains(m.Command, "'ssh-ed25519 AAAA test'") {
		t.Errorf("public key should be quoted: %s", m.Command)
	}

	w := DeployKeyAdd("a/b", "t", "k", true)
	if !strings.Contains(w.Command, "read_only=false") {
		t.Errorf("write key should set read_only=false: %s", w.Command)
	}
}

func TestRepoCreateVisibility(t *testing.T) {
	if !strings.Contains(RepoCreate("a/b", true).Command, "--private") {
		t.Error("expected --private")
	}
	if !strings.Contains(RepoCreate("a/b", false).Command, "--public") {
		t.Error("expected --public")
	}
}

func TestShellQuoteEscapesQuotes(t *testing.T) {
	got := shellQuote("it's")
	if got != `'it'\''s'` {
		t.Errorf("shellQuote = %q", got)
	}
}

func TestDeliverGHRunsEachCommand(t *testing.T) {
	var ran []string
	env := Env{
		Out: &bytes.Buffer{},
		Run: func(c string) error { ran = append(ran, c); return nil },
	}
	b := Batch{Repo: "a/b"}
	b.Add(RepoCreate("a/b", true))
	b.Add(DeployKeyAdd("a/b", "t", "k", true))
	if err := Deliver("gh", env, b); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if len(ran) != 2 {
		t.Errorf("expected 2 commands run, got %v", ran)
	}
}

func TestDeliverClipboard(t *testing.T) {
	var copied string
	out := &bytes.Buffer{}
	env := Env{Out: out, Clipboard: func(s string) error { copied = s; return nil }}
	b := Batch{Repo: "a/b"}
	b.Add(RepoCreate("a/b", true))
	if err := Deliver("clipboard", env, b); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if !strings.Contains(copied, "gh repo create a/b") {
		t.Errorf("clipboard content: %q", copied)
	}
	if !strings.Contains(out.String(), "clipboard") {
		t.Errorf("missing confirmation: %q", out.String())
	}
}

func TestDeliverUnknownMethod(t *testing.T) {
	b := Batch{Repo: "a/b"}
	b.Add(RepoCreate("a/b", true))
	if err := Deliver("smoke-signal", Env{Out: &bytes.Buffer{}}, b); err == nil {
		t.Error("expected unknown method error")
	}
}

func TestDeliverEmptyBatchIsNoop(t *testing.T) {
	if err := Deliver("gh", Env{Out: &bytes.Buffer{}}, Batch{Repo: "a/b"}); err != nil {
		t.Errorf("empty batch should be a no-op, got %v", err)
	}
}
