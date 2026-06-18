// Package registry is rig's authoritative, off-volume manifest of groups,
// projects, and keys. It is the source of truth that survives golden-master
// wipes and doubles as a rebuild manifest.
//
// The in-memory Registry type and its queries/mutations are pure and testable;
// loading and atomic persistence live in store.go behind a flock.
package registry

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/AndrewMast/rig/internal/model"
)

// SchemaVersion is bumped only on incompatible manifest changes. There is no
// migration path (clean slate); it exists to fail loudly on a mismatch.
const SchemaVersion = 1

// Registry is the whole manifest held in memory.
type Registry struct {
	Version  int             `json:"version"`
	Groups   []model.Group   `json:"groups"`
	Projects []model.Project `json:"projects"`
	Keys     []model.Key     `json:"keys"`
}

// New returns an empty registry stamped with the current schema version.
func New() *Registry {
	return &Registry{Version: SchemaVersion}
}

var (
	// ErrNotFound is returned by lookups that require a unique hit.
	ErrNotFound = errors.New("not found")
	// ErrExists is returned when adding an entity that already exists.
	ErrExists = errors.New("already exists")
	// ErrAmbiguous is returned when a lookup matches more than one entity.
	ErrAmbiguous = errors.New("ambiguous")
)

func eqFold(a, b string) bool { return strings.EqualFold(a, b) }

// --- Group queries -------------------------------------------------------

// FindGroup returns the group with the given name (case-insensitive, since the
// folder name is the identity and macOS paths are case-insensitive).
func (r *Registry) FindGroup(name string) *model.Group {
	for i := range r.Groups {
		if eqFold(r.Groups[i].Name, name) {
			return &r.Groups[i]
		}
	}
	return nil
}

// AddGroup inserts a new group, refusing duplicates by name.
func (r *Registry) AddGroup(g model.Group) error {
	if r.FindGroup(g.Name) != nil {
		return fmt.Errorf("group %q: %w", g.Name, ErrExists)
	}
	r.Groups = append(r.Groups, g)
	return nil
}

// RemoveGroup deletes a group by name. It does not check membership; callers
// enforce the "empty only" rule.
func (r *Registry) RemoveGroup(name string) error {
	for i := range r.Groups {
		if eqFold(r.Groups[i].Name, name) {
			r.Groups = append(r.Groups[:i], r.Groups[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("group %q: %w", name, ErrNotFound)
}

// --- Project queries -----------------------------------------------------

// FindProject returns the project identified by the (group, name) pair.
func (r *Registry) FindProject(group, name string) *model.Project {
	for i := range r.Projects {
		if eqFold(r.Projects[i].Group, group) && eqFold(r.Projects[i].Name, name) {
			return &r.Projects[i]
		}
	}
	return nil
}

// ProjectsByName returns every project with the given name across all groups.
func (r *Registry) ProjectsByName(name string) []model.Project {
	var out []model.Project
	for _, p := range r.Projects {
		if eqFold(p.Name, name) {
			out = append(out, p)
		}
	}
	return out
}

// ProjectsInGroup returns every project belonging to a group.
func (r *Registry) ProjectsInGroup(group string) []model.Project {
	var out []model.Project
	for _, p := range r.Projects {
		if eqFold(p.Group, group) {
			out = append(out, p)
		}
	}
	return out
}

// AddProject inserts a project, refusing a duplicate (group, name).
func (r *Registry) AddProject(p model.Project) error {
	if r.FindProject(p.Group, p.Name) != nil {
		return fmt.Errorf("project %q: %w", p.ID(), ErrExists)
	}
	r.Projects = append(r.Projects, p)
	return nil
}

// RemoveProject deletes a project by (group, name).
func (r *Registry) RemoveProject(group, name string) error {
	for i := range r.Projects {
		if eqFold(r.Projects[i].Group, group) && eqFold(r.Projects[i].Name, name) {
			r.Projects = append(r.Projects[:i], r.Projects[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("project %q: %w", group+"/"+name, ErrNotFound)
}

// SuggestProjectName returns name, or name-2/-3/... if there is a collision in
// the target group.
func (r *Registry) SuggestProjectName(group, name string) string {
	if r.FindProject(group, name) == nil {
		return name
	}
	for n := 2; ; n++ {
		cand := fmt.Sprintf("%s-%d", name, n)
		if r.FindProject(group, cand) == nil {
			return cand
		}
	}
}

// --- Key queries ---------------------------------------------------------

// FindKey returns the key with the given short ID.
func (r *Registry) FindKey(id string) *model.Key {
	for i := range r.Keys {
		if eqFold(r.Keys[i].ID, id) {
			return &r.Keys[i]
		}
	}
	return nil
}

// KeysForRepo returns all keys bound to a repo, write keys first.
func (r *Registry) KeysForRepo(repo string) []model.Key {
	var out []model.Key
	for _, k := range r.Keys {
		if eqFold(k.Repo, repo) {
			out = append(out, k)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Write && !out[j].Write })
	return out
}

// AddKey inserts a key, refusing a duplicate ID.
func (r *Registry) AddKey(k model.Key) error {
	if r.FindKey(k.ID) != nil {
		return fmt.Errorf("key %q: %w", k.ID, ErrExists)
	}
	r.Keys = append(r.Keys, k)
	return nil
}

// RemoveKey deletes a key by ID.
func (r *Registry) RemoveKey(id string) error {
	for i := range r.Keys {
		if eqFold(r.Keys[i].ID, id) {
			r.Keys = append(r.Keys[:i], r.Keys[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("key %q: %w", id, ErrNotFound)
}

// ProjectsBoundToKey returns projects bound to a given key ID.
func (r *Registry) ProjectsBoundToKey(id string) []model.Project {
	var out []model.Project
	for _, p := range r.Projects {
		if eqFold(p.KeyID, id) {
			out = append(out, p)
		}
	}
	return out
}
