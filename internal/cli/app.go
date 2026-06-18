package cli

import (
	"context"

	"github.com/AndrewMast/rig/internal/config"
	"github.com/AndrewMast/rig/internal/registry"
	"github.com/spf13/cobra"
)

// App is the shared context every command receives: resolved paths, loaded
// config, and the registry store. IO clients (git/gh/keygen/clock) are added as
// those seams come online.
type App struct {
	Paths  config.Paths
	Config *config.Config
	Store  *registry.Store
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
