package config

import (
	"path/filepath"
	"testing"
)

func devPaths(t *testing.T) Paths {
	t.Helper()
	t.Setenv("RIG_HOME", t.TempDir())
	p, err := ResolvePaths()
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	if !p.DevMode {
		t.Fatal("RIG_HOME should enable dev mode")
	}
	return p
}

func TestLoadMissingReturnsDefaults(t *testing.T) {
	p := devPaths(t)
	c, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.Handoff.Method != MethodClipboard {
		t.Errorf("method = %q, want clipboard", c.Handoff.Method)
	}
	if _, ok := c.Launchers["solo"]; !ok {
		t.Error("default launchers should be present")
	}
	if c.Launchers["finder"].Target != TargetFolder {
		t.Error("finder should target folder")
	}
}

func TestSaveLoadRoundTripAndOverride(t *testing.T) {
	p := devPaths(t)
	c := Default(p)
	if err := c.Set("handoff.method", "gh"); err != nil {
		t.Fatalf("set: %v", err)
	}
	c.Launchers["custom"] = Launcher{Command: "echo {path}", Target: TargetFolder}
	if err := Save(p, c); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Handoff.Method != MethodGH {
		t.Errorf("method = %q, want gh", got.Handoff.Method)
	}
	if got.Launchers["custom"].Command != "echo {path}" {
		t.Error("custom launcher lost")
	}
	if _, ok := got.Launchers["solo"]; !ok {
		t.Error("default launchers should survive merge")
	}
}

func TestValidateRejectsBadMethodAndTarget(t *testing.T) {
	p := devPaths(t)
	c := Default(p)
	if err := c.Set("handoff.method", "carrier-pigeon"); err == nil {
		t.Error("expected invalid method to be rejected")
	}
	c = Default(p)
	c.Launchers["x"] = Launcher{Command: "y {path}", Target: "nowhere"}
	if err := c.Validate(); err == nil {
		t.Error("expected invalid target to be rejected")
	}
}

func TestGetSetDottedKeys(t *testing.T) {
	p := devPaths(t)
	c := Default(p)
	if err := c.Set("launchers.solo.target", "folder"); err != nil {
		t.Fatalf("set launcher target: %v", err)
	}
	if v, _ := c.Get("launchers.solo.target"); v != "folder" {
		t.Errorf("get = %q, want folder", v)
	}
	if err := c.Set("handoff.always_confirm", "notabool"); err == nil {
		t.Error("expected bad bool to be rejected")
	}
	if _, err := c.Get("nonsense.key"); err == nil {
		t.Error("expected unknown key error")
	}
}

func TestTokenFileExpandsTilde(t *testing.T) {
	p := devPaths(t)
	c := Default(p)
	c.GitHub.TokenFile = "~/secrets/tok"
	got := c.TokenFile(p)
	if filepath.IsAbs(got) == false || got == "~/secrets/tok" {
		t.Errorf("token file not expanded: %q", got)
	}
}
