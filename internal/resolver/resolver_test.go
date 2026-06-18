package resolver

import (
	"testing"

	"github.com/AndrewMast/rig/internal/model"
	"github.com/AndrewMast/rig/internal/registry"
)

func build(t *testing.T) *registry.Registry {
	t.Helper()
	r := registry.New()
	must(t, r.AddGroup(model.Group{Name: "Dripstone", Base: "/Volumes/Development"}))
	must(t, r.AddGroup(model.Group{Name: "Other", Base: "/Volumes/Development"}))
	must(t, r.AddProject(model.Project{Group: "Dripstone", Name: "dripstone"}))
	must(t, r.AddProject(model.Project{Group: "Dripstone", Name: "flutter-app", Aliases: []string{"fa"}}))
	must(t, r.AddProject(model.Project{Group: "Other", Name: "flutter-app"})) // same name, two groups
	return r
}

func TestProjectNameBeatsGroupName(t *testing.T) {
	// Spec example: group Dripstone + project dripstone, no aliases → the project.
	res := Resolve(build(t), "dripstone")
	if res.Stage != StageProjectName || !res.Unique() {
		t.Fatalf("stage=%v unique=%v", res.Stage, res.Unique())
	}
	if res.Targets[0].Kind != KindProject || res.Targets[0].Project.Name != "dripstone" {
		t.Errorf("resolved to %+v", res.Targets[0])
	}
	if res.Targets[0].Path != "/Volumes/Development/Dripstone/dripstone" {
		t.Errorf("path = %q", res.Targets[0].Path)
	}
}

func TestExactFullID(t *testing.T) {
	res := Resolve(build(t), "Other/flutter-app")
	if res.Stage != StageFullID || !res.Unique() {
		t.Fatalf("stage=%v unique=%v", res.Stage, res.Unique())
	}
	if res.Targets[0].Project.Group != "Other" {
		t.Errorf("group = %q", res.Targets[0].Project.Group)
	}
}

func TestAmbiguousProjectName(t *testing.T) {
	res := Resolve(build(t), "flutter-app")
	if res.Stage != StageProjectName {
		t.Fatalf("stage = %v", res.Stage)
	}
	if len(res.Targets) != 2 {
		t.Fatalf("targets = %d, want 2", len(res.Targets))
	}
}

func TestAlias(t *testing.T) {
	res := Resolve(build(t), "fa")
	if res.Stage != StageAlias || !res.Unique() {
		t.Fatalf("stage=%v unique=%v", res.Stage, res.Unique())
	}
	if res.Targets[0].Project.Name != "flutter-app" || res.Targets[0].Project.Group != "Dripstone" {
		t.Errorf("resolved to %+v", res.Targets[0].Project)
	}
}

func TestBareGroupNeedsConfirm(t *testing.T) {
	res := Resolve(build(t), "Other") // no project named Other
	if res.Stage != StageGroupName || !res.Confirm || !res.Unique() {
		t.Fatalf("stage=%v confirm=%v unique=%v", res.Stage, res.Confirm, res.Unique())
	}
	if res.Targets[0].Kind != KindGroup {
		t.Error("expected group target")
	}
}

func TestFuzzyRanksPrefixFirst(t *testing.T) {
	r := registry.New()
	must(t, r.AddGroup(model.Group{Name: "G", Base: "/b"}))
	must(t, r.AddProject(model.Project{Group: "G", Name: "flutter-app"}))
	must(t, r.AddProject(model.Project{Group: "G", Name: "my-flutter-thing"}))
	res := Resolve(r, "flut")
	if res.Stage != StageFuzzy {
		t.Fatalf("stage = %v", res.Stage)
	}
	if res.Targets[0].Project.Name != "flutter-app" {
		t.Errorf("prefix match should rank first, got %q", res.Targets[0].Project.Name)
	}
}

func TestNoMatch(t *testing.T) {
	res := Resolve(build(t), "zzzqqq-nonexistent")
	if len(res.Targets) != 0 {
		t.Errorf("expected no targets, got %d", len(res.Targets))
	}
}

func TestSubseqScoreOrdering(t *testing.T) {
	prefix := subseqScore("fl", "flutter")
	contig := subseqScore("ut", "flutter")
	scattered := subseqScore("fr", "flutter")
	if !(prefix > contig && contig > scattered && scattered > 0) {
		t.Errorf("ordering wrong: prefix=%d contig=%d scattered=%d", prefix, contig, scattered)
	}
	if subseqScore("zx", "flutter") != 0 {
		t.Error("non-subsequence should score 0")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
