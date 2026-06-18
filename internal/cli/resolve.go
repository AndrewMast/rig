package cli

import (
	"fmt"

	"github.com/AndrewMast/rig/internal/registry"
	"github.com/AndrewMast/rig/internal/resolver"
)

// resolveTarget turns a token into a single navigation target, applying the
// interactive/scripted rules: unique hits go straight through; a bare group
// match is confirmed; ambiguity prompts a pick-list when interactive and errors
// (listing candidates) when scripted. When allowGroup is false, group targets
// are rejected (used by project-only launchers).
func (a *App) resolveTarget(reg *registry.Registry, token string, allowGroup bool) (resolver.Target, error) {
	res := resolver.Resolve(reg, token)

	if len(res.Targets) == 0 {
		return resolver.Target{}, fmt.Errorf("no project or group matches %q", token)
	}

	if res.Unique() {
		t := res.Targets[0]
		if t.Kind == resolver.KindGroup && !allowGroup {
			return resolver.Target{}, fmt.Errorf("%q resolves to a group, but a project is required", token)
		}
		if res.Confirm {
			ok, err := a.UI.Confirm(fmt.Sprintf("Target group %q?", t.Group.Name), true)
			if err != nil {
				return resolver.Target{}, err
			}
			if !ok {
				return resolver.Target{}, fmt.Errorf("cancelled")
			}
		}
		return t, nil
	}

	// Ambiguous: build a pick-list.
	labels := make([]string, 0, len(res.Targets))
	filtered := make([]resolver.Target, 0, len(res.Targets))
	for _, t := range res.Targets {
		if t.Kind == resolver.KindGroup && !allowGroup {
			continue
		}
		filtered = append(filtered, t)
		labels = append(labels, targetLabel(t))
	}
	if len(filtered) == 0 {
		return resolver.Target{}, fmt.Errorf("no project matches %q", token)
	}
	idx, err := a.UI.Select(fmt.Sprintf("Multiple matches for %q:", token), labels)
	if err != nil {
		return resolver.Target{}, err
	}
	return filtered[idx], nil
}

// confirm asks for confirmation unless assumeYes is set (the scriptable bypass
// for destructive/prompting commands). It centralizes the "flags are the
// scriptable path" rule so a --yes/--force flag works the same everywhere.
func (a *App) confirm(prompt string, def, assumeYes bool) (bool, error) {
	if assumeYes {
		return true, nil
	}
	return a.UI.Confirm(prompt, def)
}

func targetLabel(t resolver.Target) string {
	if t.Kind == resolver.KindGroup {
		return t.Group.Name + "/  (group)"
	}
	return t.Project.ID()
}
