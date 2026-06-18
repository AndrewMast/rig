package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AndrewMast/rig/internal/model"
)

// projectTokenForCwd returns the (Group/Name) token of the managed project that
// contains the current working directory, if any.
func projectTokenForCwd(app *App) (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	reg, err := app.Registry()
	if err != nil {
		return "", false
	}
	var best model.Project
	var bestPath string
	for _, p := range reg.Projects {
		g := reg.FindGroup(p.Group)
		if g == nil {
			continue
		}
		ppath := p.Path(*g)
		if pathWithin(cwd, ppath) {
			// Prefer the deepest (most specific) match.
			if len(ppath) > len(bestPath) {
				best, bestPath = p, ppath
			}
		}
	}
	if bestPath == "" {
		return "", false
	}
	return best.ID(), true
}

// pathWithin reports whether dir is base or lives beneath it. Comparison is
// case-insensitive: on case-insensitive filesystems (macOS, Windows) the cwd
// reflects whatever casing was typed at `cd`, which need not match the
// registry's canonical casing for the group/name.
func pathWithin(dir, base string) bool {
	dirLow, baseLow := strings.ToLower(dir), strings.ToLower(base)
	return dirLow == baseLow || strings.HasPrefix(dirLow, baseLow+string(filepath.Separator))
}

// firstArg returns args[0], or "" when args is empty.
func firstArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

// projectToken returns explicit when non-empty, otherwise the token of the
// project containing the current directory. It errors when neither is available,
// letting project subcommands accept the token as optional.
func (a *App) projectToken(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	tok, ok := projectTokenForCwd(a)
	if !ok {
		return "", fmt.Errorf("not inside a project; pass a group/name")
	}
	return tok, nil
}

// pickProjectToken prompts the user to choose among all projects and returns the
// chosen (Group/Name) token.
func (a *App) pickProjectToken(prompt string) (string, error) {
	reg, err := a.Registry()
	if err != nil {
		return "", err
	}
	if len(reg.Projects) == 0 {
		return "", fmt.Errorf("no projects registered yet")
	}
	ids := make([]string, 0, len(reg.Projects))
	for _, p := range reg.Projects {
		ids = append(ids, p.ID())
	}
	sort.Strings(ids)
	idx, err := a.UI.Select(prompt, ids)
	if err != nil {
		return "", err
	}
	return ids[idx], nil
}
