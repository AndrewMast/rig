package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGroupNewAndList(t *testing.T) {
	ta := newTestApp(t, "")
	base := filepath.Join(ta.Paths.Home, "projects")
	if _, err := ta.run(t, "group", "new", "Acme", "--base", base); err != nil {
		t.Fatalf("group new: %v", err)
	}
	if g := ta.reg(t).FindGroup("Acme"); g == nil {
		t.Fatal("group not registered")
	}
	if fi, err := os.Stat(filepath.Join(base, "Acme")); err != nil || !fi.IsDir() {
		t.Fatal("group folder not created")
	}
	out, _ := ta.run(t, "group", "list")
	if !strings.Contains(out, "Acme") {
		t.Errorf("group list missing Acme: %q", out)
	}
}

func TestCreateLocalProject(t *testing.T) {
	ta := newTestApp(t, "")
	out, err := ta.run(t, "create", "Acme/widget")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !strings.Contains(out, "local-only") {
		t.Errorf("unexpected output: %q", out)
	}
	p := ta.reg(t).FindProject("Acme", "widget")
	if p == nil {
		t.Fatal("project not registered")
	}
	if p.Strategy != "local" || p.State != "active" {
		t.Errorf("project = %+v", p)
	}
	// git init should have run against the derived path.
	wantPath := filepath.Join(ta.Paths.Home, "projects", "Acme", "widget")
	foundInit := false
	for _, c := range ta.gitFake.Calls {
		if c == "init "+wantPath {
			foundInit = true
		}
	}
	if !foundInit {
		t.Errorf("git init not called for %s; calls=%v", wantPath, ta.gitFake.Calls)
	}

	list, _ := ta.run(t, "list")
	if !strings.Contains(list, "Acme/widget") || !strings.Contains(list, "local-only") {
		t.Errorf("list output: %q", list)
	}
}

func TestCreateSuggestsNameOnCollision(t *testing.T) {
	ta := newTestApp(t, "")
	if _, err := ta.run(t, "create", "Acme/widget"); err != nil {
		t.Fatal(err)
	}
	if _, err := ta.run(t, "create", "Acme/widget"); err != nil {
		t.Fatal(err)
	}
	if ta.reg(t).FindProject("Acme", "widget-2") == nil {
		t.Error("expected collision to create widget-2")
	}
}

func TestStatusLocalProject(t *testing.T) {
	ta := newTestApp(t, "")
	if _, err := ta.run(t, "create", "Acme/widget"); err != nil {
		t.Fatal(err)
	}
	out, err := ta.run(t, "status", "Acme/widget")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(out, "Acme/widget") || !strings.Contains(out, "local-only") {
		t.Errorf("status output: %q", out)
	}
}

func TestAdoptDerivesIdentityFromPath(t *testing.T) {
	ta := newTestApp(t, "") // empty stdin: confirm defaults to yes
	base := filepath.Join(ta.Paths.Home, "projects")
	folder := filepath.Join(base, "MyGroup", "myproj")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ta.run(t, "adopt", folder); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	p := ta.reg(t).FindProject("MyGroup", "myproj")
	if p == nil {
		t.Fatal("adopted project not registered")
	}
	g := ta.reg(t).FindGroup("MyGroup")
	if g == nil || g.Base != base {
		t.Errorf("group base wrong: %+v", g)
	}
}

func TestGroupAliasRoundTrip(t *testing.T) {
	ta := newTestApp(t, "")
	base := filepath.Join(ta.Paths.Home, "projects")
	if _, err := ta.run(t, "group", "new", "Acme", "--base", base); err != nil {
		t.Fatal(err)
	}
	if _, err := ta.run(t, "group", "alias", "add", "Acme", "ac"); err != nil {
		t.Fatalf("alias add: %v", err)
	}
	g := ta.reg(t).FindGroup("Acme")
	if len(g.Aliases) != 1 || g.Aliases[0] != "ac" {
		t.Errorf("aliases = %v", g.Aliases)
	}
	out, _ := ta.run(t, "alias")
	if !strings.Contains(out, "ac") || !strings.Contains(out, "Acme") {
		t.Errorf("unified alias view: %q", out)
	}
}
