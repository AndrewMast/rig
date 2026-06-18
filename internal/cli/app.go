package cli

import (
	"context"

	"github.com/AndrewMast/rig/internal/clock"
	"github.com/AndrewMast/rig/internal/config"
	"github.com/AndrewMast/rig/internal/gh"
	"github.com/AndrewMast/rig/internal/git"
	"github.com/AndrewMast/rig/internal/handoff"
	"github.com/AndrewMast/rig/internal/keygen"
	"github.com/AndrewMast/rig/internal/registry"
	"github.com/AndrewMast/rig/internal/ui"
	"github.com/spf13/cobra"
)

// App is the shared context every command receives: resolved paths, loaded
// config, the registry store, and the IO seams (behind interfaces so tests
// swap in fakes).
type App struct {
	Paths  config.Paths
	Config *config.Config
	Store  *registry.Store

	Git    git.Git
	GH     gh.GH
	Keygen keygen.Keygen
	Clock  clock.Clock
	UI     *ui.UI

	// envOverride, when set, replaces the real handoff delivery capabilities
	// (clipboard/opener/runner) — used by tests.
	envOverride *handoff.Env
}

// newApp resolves paths and loads config. It is called once per invocation from
// the root command's PersistentPreRunE.
func newApp() (*App, error) {
	paths, err := config.ResolvePaths()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(paths)
	if err != nil {
		return nil, err
	}
	return &App{
		Paths:  paths,
		Config: cfg,
		Store:  registry.NewStore(paths.Registry),
		Git:    git.New(),
		GH:     gh.New(gh.LoadToken(cfg.TokenFile(paths))),
		Keygen: keygen.New(),
		Clock:  clock.Real{},
		UI:     ui.New(),
	}, nil
}

// Registry loads the manifest fresh from disk.
func (a *App) Registry() (*registry.Registry, error) {
	return a.Store.Load()
}

// SaveRegistry persists the manifest atomically.
func (a *App) SaveRegistry(r *registry.Registry) error {
	return a.Store.Save(r)
}

type appKey struct{}

// withApp stashes the app on the command context.
func withApp(ctx context.Context, a *App) context.Context {
	return context.WithValue(ctx, appKey{}, a)
}

// appFrom retrieves the app stashed by the root command.
func appFrom(cmd *cobra.Command) *App {
	return cmd.Context().Value(appKey{}).(*App)
}
