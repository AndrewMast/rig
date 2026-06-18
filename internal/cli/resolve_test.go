package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/AndrewMast/rig/internal/model"
	"github.com/AndrewMast/rig/internal/registry"
	"github.com/AndrewMast/rig/internal/resolver"
	"github.com/AndrewMast/rig/internal/ui"
)

func testReg(t *testing.T) *registry.Registry {
	t.Helper()
	r := registry.New()
	r.AddGroup(model.Group{Name: "Dripstone", Base: "/Volumes/Development"})
	r.AddGroup(model.Group{Name: "Other", Base: "/Volumes/Development"})
	r.AddProject(model.Project{Group: "Dripstone", Name: "app"})
	r.AddProject(model.Project{Group: "Other", Name: "app"})
	return r
}

func TestResolveTargetUnique(t *testing.T) {
	app := &App{UI: ui.NewWith(strings.NewReader(""), &bytes.Buffer{})}
	tgt, err := app.resolveTarget(testReg(t), "Dripstone/app", false)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if tgt.Kind != resolver.KindProject || tgt.Project.Group != "Dripstone" {
		t.Errorf("got %+v", tgt)
	}
}

func TestResolveTargetAmbiguousPrompts(t *testing.T) {
	// Two projects named "app"; choosing option 2 should pick the second.
	in := strings.NewReader("2\n")
	out := &bytes.Buffer{}
	app := &App{UI: ui.NewWith(in, out)}
	tgt, err := app.resolveTarget(testReg(t), "app", false)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !strings.Contains(out.String(), "Multiple matches") {
		t.Errorf("expected pick-list, got %q", out.String())
	}
	if tgt.Project.ID() != "Other/app" {
		t.Errorf("picked %q, want Other/app", tgt.Project.ID())
	}
}

func TestResolveTargetGroupRejectedWhenProjectRequired(t *testing.T) {
	app := &App{UI: ui.NewWith(strings.NewReader("y\n"), &bytes.Buffer{})}
	_, err := app.resolveTarget(testReg(t), "Dripstone", false) // bare group, project required
	if err == nil {
		t.Fatal("expected group target to be rejected")
	}
}

func TestResolveTargetNoMatch(t *testing.T) {
	app := &App{UI: ui.NewWith(strings.NewReader(""), &bytes.Buffer{})}
	if _, err := app.resolveTarget(testReg(t), "nope", false); err == nil {
		t.Fatal("expected no-match error")
	}
}
