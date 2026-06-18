// Package resolver turns a user token into a navigation target. It is pure: it
// ranks candidates and reports whether a hit is definitive, ambiguous, or needs
// confirmation, but it never prompts. The CLI layer decides how to resolve
// ambiguity (interactive pick-list vs. scripted error).
package resolver

import (
	"sort"
	"strings"

	"github.com/AndrewMast/rig/internal/model"
	"github.com/AndrewMast/rig/internal/registry"
)

// Kind distinguishes a project target from a group target.
type Kind int

const (
	KindProject Kind = iota
	KindGroup
)

// Target is one resolved destination with its absolute path.
type Target struct {
	Kind    Kind
	Project model.Project // valid when Kind == KindProject
	Group   model.Group   // the group (for KindGroup, or the project's group)
	Path    string
}

// Stage records which resolution rule produced the result.
type Stage int

const (
	StageNone Stage = iota
	StageAlias
	StageFullID
	StageProjectName
	StageGroupName
	StageFuzzy
)

// Resolution is the outcome of resolving a token.
type Resolution struct {
	Stage   Stage
	Targets []Target // 1 = unique; >1 = ambiguous, ranked best-first; 0 = no match
	// Confirm is set when the single target is a bare group-name match (rule 4):
	// definitive enough to offer, but confirm before navigating.
	Confirm bool
}

// Unique reports whether exactly one target matched.
func (r Resolution) Unique() bool { return len(r.Targets) == 1 }

// Resolve applies the projects-first resolution rules to a token.
func Resolve(reg *registry.Registry, token string) Resolution {
	token = strings.TrimSpace(token)

	// Rule 1: exact alias (project or group).
	if res, ok := byAlias(reg, token); ok {
		return res
	}

	// Rule 2: exact Group/name.
	if owner, name, ok := splitID(token); ok {
		if p := reg.FindProject(owner, name); p != nil {
			return Resolution{Stage: StageFullID, Targets: []Target{projectTarget(reg, *p)}}
		}
	}

	// Rule 3: exact project name across groups.
	if byName := reg.ProjectsByName(token); len(byName) > 0 {
		targets := make([]Target, 0, len(byName))
		for _, p := range byName {
			targets = append(targets, projectTarget(reg, p))
		}
		return Resolution{Stage: StageProjectName, Targets: targets}
	}

	// Rule 4: exact group name (only when no project matched) — confirm.
	if g := reg.FindGroup(token); g != nil {
		return Resolution{
			Stage:   StageGroupName,
			Targets: []Target{{Kind: KindGroup, Group: *g, Path: g.Path()}},
			Confirm: true,
		}
	}

	// Rule 5: fuzzy, projects-first, ranked.
	return fuzzy(reg, token)
}

func splitID(token string) (group, name string, ok bool) {
	parts := strings.SplitN(token, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func projectTarget(reg *registry.Registry, p model.Project) Target {
	t := Target{Kind: KindProject, Project: p}
	if g := reg.FindGroup(p.Group); g != nil {
		t.Group = *g
		t.Path = p.Path(*g)
	}
	return t
}

// byAlias matches an exact alias on any project or group.
func byAlias(reg *registry.Registry, token string) (Resolution, bool) {
	var targets []Target
	for _, p := range reg.Projects {
		if containsFold(p.Aliases, token) {
			targets = append(targets, projectTarget(reg, p))
		}
	}
	// A project alias is a definitive project hit.
	if len(targets) > 0 {
		return Resolution{Stage: StageAlias, Targets: targets}, true
	}
	for _, g := range reg.Groups {
		if containsFold(g.Aliases, token) {
			// Group aliases are definitive group targets.
			return Resolution{Stage: StageAlias, Targets: []Target{{Kind: KindGroup, Group: g, Path: g.Path()}}}, true
		}
	}
	return Resolution{}, false
}

func containsFold(list []string, s string) bool {
	for _, e := range list {
		if strings.EqualFold(e, s) {
			return true
		}
	}
	return false
}

// fuzzy ranks projects (and groups, lower priority) by subsequence match.
func fuzzy(reg *registry.Registry, token string) Resolution {
	type scored struct {
		t     Target
		score int
	}
	var hits []scored
	for _, p := range reg.Projects {
		// Score against both the bare name and the Group/name id; take the best.
		s := subseqScore(token, p.Name)
		if sid := subseqScore(token, p.ID()); sid > s {
			s = sid
		}
		if s > 0 {
			hits = append(hits, scored{projectTarget(reg, p), s + projectBoost})
		}
	}
	for _, g := range reg.Groups {
		if s := subseqScore(token, g.Name); s > 0 {
			hits = append(hits, scored{Target{Kind: KindGroup, Group: g, Path: g.Path()}, s})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })

	targets := make([]Target, 0, len(hits))
	for _, h := range hits {
		targets = append(targets, h.t)
	}
	return Resolution{Stage: StageFuzzy, Targets: targets}
}

// projectBoost keeps projects ranked above groups at equal subsequence quality.
const projectBoost = 1000
