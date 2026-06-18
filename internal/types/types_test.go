package types

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndMerge(t *testing.T) {
	typesDir := t.TempDir()
	flutter := filepath.Join(typesDir, "flutter")
	os.MkdirAll(flutter, 0o755)
	os.WriteFile(filepath.Join(flutter, "type.toml"), []byte(`
[hooks]
setup = "flutter pub get"
create = "flutter create ."
[commands]
test = "flutter test"
run = "flutter run"
`), 0o644)

	prof, err := LoadType(typesDir, "flutter")
	if err != nil {
		t.Fatalf("load type: %v", err)
	}
	if prof.Hooks["setup"] != "flutter pub get" || prof.Commands["test"] != "flutter test" {
		t.Fatalf("unexpected profile: %+v", prof)
	}

	// Project rig.toml overrides the type's test command, adds a hook.
	projDir := t.TempDir()
	os.WriteFile(filepath.Join(projDir, "rig.toml"), []byte(`
[hooks]
setup = "make bootstrap"
[commands]
test = "make test"
`), 0o644)
	overlay, err := LoadProjectFile(projDir)
	if err != nil {
		t.Fatalf("load project file: %v", err)
	}

	merged := Merge(prof, overlay)
	if merged.Hooks["setup"] != "make bootstrap" {
		t.Errorf("override failed: %q", merged.Hooks["setup"])
	}
	if merged.Hooks["create"] != "flutter create ." {
		t.Errorf("base hook lost: %q", merged.Hooks["create"])
	}
	if merged.Commands["test"] != "make test" || merged.Commands["run"] != "flutter run" {
		t.Errorf("command merge wrong: %+v", merged.Commands)
	}
}

func TestLoadMissingTypeErrors(t *testing.T) {
	if _, err := LoadType(t.TempDir(), "ghost"); err == nil {
		t.Error("expected missing type to error")
	}
}

func TestLoadProjectFileMissingIsEmpty(t *testing.T) {
	p, err := LoadProjectFile(t.TempDir())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(p.Hooks) != 0 || len(p.Commands) != 0 {
		t.Error("missing rig.toml should be empty profile")
	}
}
