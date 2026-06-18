// Package types models reusable project type profiles and per-project rig.toml
// overlays. A profile is just hooks + commands; parsing and merging are pure and
// testable. Running hooks/commands is the CLI's job.
package types

import (
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// Profile is a type's (or project's) hooks and extra commands.
type Profile struct {
	Hooks    map[string]string `toml:"hooks"`
	Commands map[string]string `toml:"commands"`
}

// Hook names recognized by rig.
const (
	HookPreflight = "preflight"
	HookSetup     = "setup"
	HookCreate    = "create"
)

// TypeFile returns the path to a type's type.toml.
func TypeFile(typesDir, name string) string {
	return filepath.Join(typesDir, name, "type.toml")
}

// LoadType reads a type profile. A missing type is an error; a type with no
// type.toml yields an empty profile.
func LoadType(typesDir, name string) (Profile, error) {
	if name == "" {
		return Profile{}, nil
	}
	dir := filepath.Join(typesDir, name)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return Profile{}, fmt.Errorf("type %q not found", name)
	}
	return parseFile(TypeFile(typesDir, name))
}

// LoadProjectFile reads a project's rig.toml from its root. A missing file
// yields an empty profile (not an error).
func LoadProjectFile(projectDir string) (Profile, error) {
	return parseFile(filepath.Join(projectDir, "rig.toml"))
}

func parseFile(path string) (Profile, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Profile{}, nil
	}
	if err != nil {
		return Profile{}, fmt.Errorf("read %s: %w", path, err)
	}
	var p Profile
	if err := toml.Unmarshal(data, &p); err != nil {
		return Profile{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return p, nil
}

// Merge overlays over onto base (over wins), returning a new profile.
func Merge(base, over Profile) Profile {
	out := Profile{Hooks: map[string]string{}, Commands: map[string]string{}}
	for k, v := range base.Hooks {
		out.Hooks[k] = v
	}
	for k, v := range over.Hooks {
		out.Hooks[k] = v
	}
	for k, v := range base.Commands {
		out.Commands[k] = v
	}
	for k, v := range over.Commands {
		out.Commands[k] = v
	}
	return out
}
