package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTypeNewListShow(t *testing.T) {
	ta := newTestApp(t, "")
	if _, err := ta.run(t, "type", "new", "flutter"); err != nil {
		t.Fatalf("type new: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ta.typesDir(), "flutter", "type.toml")); err != nil {
		t.Fatalf("type.toml not created: %v", err)
	}
	out, _ := ta.run(t, "type", "list")
	if !strings.Contains(out, "flutter") {
		t.Errorf("type list: %q", out)
	}
	// Duplicate refused.
	if _, err := ta.run(t, "type", "new", "flutter"); err == nil {
		t.Error("expected duplicate type to be refused")
	}
}

func TestTypeCommandRunsInProjectContext(t *testing.T) {
	ta := newTestApp(t, "")
	// Define a type whose `greet` command writes a marker file.
	dir := filepath.Join(ta.typesDir(), "demo")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "type.toml"), []byte(`
[commands]
greet = "echo hi > marker.txt"
`), 0o644)

	// Create a project of that type by writing it into the registry directly,
	// then run the type command targeting it by token.
	if _, err := ta.run(t, "create", "Acme/widget", "--type", "demo"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := ta.run(t, "greet", "Acme/widget"); err != nil {
		t.Fatalf("greet: %v", err)
	}
	g := ta.reg(t).FindGroup("Acme")
	p := ta.reg(t).FindProject("Acme", "widget")
	marker := filepath.Join(p.Path(*g), "marker.txt")
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("type command did not run in project dir: %v", err)
	}
}

func TestTokenStatusAbsent(t *testing.T) {
	ta := newTestApp(t, "")
	out, err := ta.run(t, "config", "token", "status")
	if err != nil {
		t.Fatalf("token status: %v", err)
	}
	if !strings.Contains(out, "absent") {
		t.Errorf("expected absent, got %q", out)
	}
}
