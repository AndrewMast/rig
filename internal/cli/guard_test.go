package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/AndrewMast/rig/internal/config"
	"github.com/AndrewMast/rig/internal/ui"
	"github.com/spf13/cobra"
)

// guardApp builds a non-dev App with a guard configured to never match the
// current user, plus a scripted UI.
func guardApp(stdin string) *App {
	cfg := &config.Config{}
	cfg.Guard.ExpectedUser = "definitely-not-the-current-user-xyz"
	return &App{
		Paths:  config.Paths{DevMode: false},
		Config: cfg,
		UI:     ui.NewWith(strings.NewReader(stdin), &bytes.Buffer{}),
	}
}

// cmdUnder builds root -> group -> child so topLevelName resolves correctly.
func cmdUnder(group, child string) *cobra.Command {
	root := &cobra.Command{Use: "rig"}
	g := &cobra.Command{Use: group}
	c := &cobra.Command{Use: child}
	g.AddCommand(c)
	root.AddCommand(g)
	return c
}

func TestGuardRefusesWhenDeclined(t *testing.T) {
	app := guardApp("n\n")
	if err := app.checkGuard(cmdUnder("project", "delete")); err == nil {
		t.Fatal("expected guard to refuse when user declines")
	}
}

func TestGuardContinuesWhenConfirmed(t *testing.T) {
	app := guardApp("y\n")
	if err := app.checkGuard(cmdUnder("project", "delete")); err != nil {
		t.Errorf("expected continue on confirm, got %v", err)
	}
}

func TestGuardExemptsConfig(t *testing.T) {
	app := guardApp("n\n") // would refuse, but config is exempt
	if err := app.checkGuard(cmdUnder("config", "set")); err != nil {
		t.Errorf("config should be exempt from the guard, got %v", err)
	}
}

func TestGuardSkippedInDevMode(t *testing.T) {
	app := guardApp("n\n")
	app.Paths.DevMode = true
	if err := app.checkGuard(cmdUnder("project", "delete")); err != nil {
		t.Errorf("dev mode should skip the guard, got %v", err)
	}
}

func TestGuardOffWhenUnset(t *testing.T) {
	app := guardApp("n\n")
	app.Config.Guard.ExpectedUser = ""
	app.Config.Guard.ExpectedHost = ""
	if err := app.checkGuard(cmdUnder("project", "delete")); err != nil {
		t.Errorf("unset guard should be a no-op, got %v", err)
	}
}
