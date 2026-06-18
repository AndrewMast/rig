package cli

import (
	"testing"

	"github.com/AndrewMast/rig/internal/model"
	"github.com/AndrewMast/rig/internal/registry"
)

func TestKeyAccessFromBoundKey(t *testing.T) {
	reg := registry.New()
	reg.AddKey(model.Key{ID: "w1", Repo: "a/b", Write: true})
	reg.AddKey(model.Key{ID: "r1", Repo: "a/b", Write: false})

	write := model.Project{Strategy: model.StrategyDeployKey, KeyID: "w1", Repo: "a/b"}
	if got := keyAccess(reg, &write); got != "write" {
		t.Errorf("write key access = %q, want write", got)
	}
	read := model.Project{Strategy: model.StrategyDeployKey, KeyID: "r1", Repo: "a/b"}
	if got := keyAccess(reg, &read); got != "read" {
		t.Errorf("read key access = %q, want read", got)
	}

	// A write key with the guard on still reports the key's true access.
	guardedWrite := model.Project{Strategy: model.StrategyDeployKey, KeyID: "w1", Guard: true}
	if got := keyAccess(reg, &guardedWrite); got != "write" {
		t.Errorf("guarded write key access = %q, want write (key, not guard)", got)
	}

	// Public and local-only projects have no key access.
	if got := keyAccess(reg, &model.Project{Strategy: model.StrategyPublic}); got != "" {
		t.Errorf("public access = %q, want empty", got)
	}
	if got := keyAccess(reg, &model.Project{Strategy: model.StrategyLocal}); got != "" {
		t.Errorf("local access = %q, want empty", got)
	}

	// Dangling binding falls back to the guard proxy.
	dangling := model.Project{Strategy: model.StrategyDeployKey, KeyID: "gone", Guard: true}
	if got := keyAccess(reg, &dangling); got != "read" {
		t.Errorf("dangling+guard access = %q, want read", got)
	}
}
