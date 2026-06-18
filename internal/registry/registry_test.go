package registry

import (
	"path/filepath"
	"testing"

	"github.com/AndrewMast/rig/internal/model"
)

func TestProjectIdentityIsGroupNamePair(t *testing.T) {
	r := New()
	mustAdd(t, r.AddProject(model.Project{Group: "Acme", Name: "app"}))
	mustAdd(t, r.AddProject(model.Project{Group: "Other", Name: "app"})) // same name, different group: allowed

	if err := r.AddProject(model.Project{Group: "Acme", Name: "app"}); err == nil {
		t.Fatal("expected duplicate (group,name) to be refused")
	}
	if got := r.ProjectsByName("app"); len(got) != 2 {
		t.Fatalf("ProjectsByName = %d, want 2", len(got))
	}
	if r.FindProject("acme", "APP") == nil {
		t.Error("lookup should be case-insensitive")
	}
}

func TestSuggestProjectName(t *testing.T) {
	r := New()
	mustAdd(t, r.AddProject(model.Project{Group: "G", Name: "app"}))
	if got := r.SuggestProjectName("G", "app"); got != "app-2" {
		t.Errorf("suggest = %q, want app-2", got)
	}
	mustAdd(t, r.AddProject(model.Project{Group: "G", Name: "app-2"}))
	if got := r.SuggestProjectName("G", "app"); got != "app-3" {
		t.Errorf("suggest = %q, want app-3", got)
	}
	if got := r.SuggestProjectName("G", "fresh"); got != "fresh" {
		t.Errorf("suggest = %q, want fresh", got)
	}
}

func TestKeysForRepoWriteFirst(t *testing.T) {
	r := New()
	mustAdd(t, r.AddKey(model.Key{ID: "r1", Repo: "a/b", Write: false}))
	mustAdd(t, r.AddKey(model.Key{ID: "w1", Repo: "a/b", Write: true}))
	mustAdd(t, r.AddKey(model.Key{ID: "x1", Repo: "c/d", Write: false}))

	got := r.KeysForRepo("a/b")
	if len(got) != 2 {
		t.Fatalf("KeysForRepo = %d, want 2", len(got))
	}
	if !got[0].Write {
		t.Error("write key should sort first")
	}
}

func TestProjectsBoundToKey(t *testing.T) {
	r := New()
	mustAdd(t, r.AddProject(model.Project{Group: "G", Name: "p1", KeyID: "k1"}))
	mustAdd(t, r.AddProject(model.Project{Group: "G", Name: "p2", KeyID: "k1"}))
	mustAdd(t, r.AddProject(model.Project{Group: "G", Name: "p3", KeyID: "k2"}))
	if got := r.ProjectsBoundToKey("k1"); len(got) != 2 {
		t.Errorf("bound = %d, want 2", len(got))
	}
}

func TestStoreRoundTripAndAtomicity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "registry.json")
	s := NewStore(path)

	r := New()
	mustAdd(t, r.AddGroup(model.Group{Name: "Acme", Base: "/base"}))
	mustAdd(t, r.AddProject(model.Project{Group: "Acme", Name: "app", Strategy: model.StrategyLocal}))
	if err := s.Save(r); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := s.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Version != SchemaVersion {
		t.Errorf("version = %d, want %d", got.Version, SchemaVersion)
	}
	if got.FindGroup("Acme") == nil || got.FindProject("Acme", "app") == nil {
		t.Error("round-trip lost data")
	}

	// No leftover temp files in the manifest dir.
	entries, _ := filepath.Glob(filepath.Join(dir, "sub", ".registry-*.tmp"))
	if len(entries) != 0 {
		t.Errorf("leftover temp files: %v", entries)
	}
}

func TestLoadMissingIsEmpty(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "nope.json"))
	r, err := s.Load()
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if len(r.Projects) != 0 || r.Version != SchemaVersion {
		t.Error("missing file should yield fresh empty registry")
	}
}

func mustAdd(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("add: %v", err)
	}
}
