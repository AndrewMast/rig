package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/AndrewMast/rig/internal/model"
	"github.com/AndrewMast/rig/internal/registry"
	"github.com/AndrewMast/rig/internal/types"
)

// resolvedProfile loads a project's effective profile: its type's hooks/commands
// with the project's rig.toml merged over.
func (a *App) resolvedProfile(g *model.Group, p *model.Project) (types.Profile, error) {
	base, err := types.LoadType(a.Paths.ConfigDir+"/types", p.Type)
	if err != nil {
		return types.Profile{}, err
	}
	overlay, err := types.LoadProjectFile(p.Path(*g))
	if err != nil {
		return types.Profile{}, err
	}
	return types.Merge(base, overlay), nil
}

// runHook runs the named hook (if defined) in the project directory with the
// rig environment exported. A missing hook is a no-op.
func (a *App) runHook(reg *registry.Registry, g *model.Group, p *model.Project, name string) error {
	prof, err := a.resolvedProfile(g, p)
	if err != nil {
		return err
	}
	cmdline := prof.Hooks[name]
	if cmdline == "" {
		return nil
	}
	return a.runInProject(reg, g, p, cmdline)
}

// runInProject executes a command line via sh in the project directory with the
// rig hook environment exported.
func (a *App) runInProject(reg *registry.Registry, g *model.Group, p *model.Project, cmdline string) error {
	c := exec.Command("sh", "-c", cmdline)
	c.Dir = p.Path(*g)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	c.Env = append(os.Environ(), hookEnv(reg, g, p)...)
	if err := c.Run(); err != nil {
		return fmt.Errorf("hook/command failed: %w", err)
	}
	return nil
}

// hookEnv builds the RIG_* environment exposed to hooks and type commands.
// RIG_KEY_ACCESS reflects the bound deploy key's access (read/write); it is
// empty for public and local-only projects.
func hookEnv(reg *registry.Registry, g *model.Group, p *model.Project) []string {
	return []string{
		"RIG_PROJECT_NAME=" + p.Name,
		"RIG_PROJECT_GROUP=" + p.Group,
		"RIG_PROJECT_BASE=" + g.Base,
		"RIG_PROJECT_PATH=" + p.Path(*g),
		"RIG_PROJECT_REPO=" + p.Repo,
		"RIG_KEY_ACCESS=" + keyAccess(reg, p),
	}
}

// keyAccess returns "write"/"read" from the bound deploy key, or "" when the
// project has no key (public or local-only). If the binding references a key no
// longer in the registry, it falls back to the push guard as a proxy.
func keyAccess(reg *registry.Registry, p *model.Project) string {
	if p.Strategy != model.StrategyDeployKey {
		return ""
	}
	if k := reg.FindKey(p.KeyID); k != nil {
		return k.Access()
	}
	if p.Guard {
		return "read"
	}
	return "write"
}
