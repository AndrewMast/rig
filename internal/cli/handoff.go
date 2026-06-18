package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AndrewMast/rig/internal/handoff"
	"github.com/spf13/cobra"
)

// handoffEnv wires the real side-effecting capabilities for handoff delivery.
// Tests may set envOverride to inject fakes for the clipboard, opener, and
// command runner.
func (a *App) handoffEnv(cmd *cobra.Command) handoff.Env {
	if a.envOverride != nil {
		e := *a.envOverride
		if e.Out == nil {
			e.Out = cmd.OutOrStdout()
		}
		return e
	}
	return handoff.Env{
		Out:       cmd.OutOrStdout(),
		FileDir:   filepath.Join(a.Paths.ConfigDir, "handoff"),
		Clipboard: pbcopy,
		Open:      openURL,
		Run:       runShell,
	}
}

// chooseMethod returns the handoff method to use, prompting when the config
// requests confirmation on every run.
func (a *App) chooseMethod(cmd *cobra.Command) (string, error) {
	method := string(a.Config.Handoff.Method)
	if !a.Config.Handoff.AlwaysConfirm {
		return method, nil
	}
	opts := handoff.Methods()
	// Put the configured method first as the default.
	sortDefaultFirst(opts, method)
	idx, err := a.UI.Select("Handoff method:", opts)
	if err != nil {
		return "", err
	}
	return opts[idx], nil
}

// deliver runs a batch through the chosen method.
func (a *App) deliver(cmd *cobra.Command, b handoff.Batch) error {
	method, err := a.chooseMethod(cmd)
	if err != nil {
		return err
	}
	return handoff.Deliver(method, a.handoffEnv(cmd), b)
}

func sortDefaultFirst(opts []string, def string) {
	for i, o := range opts {
		if o == def {
			opts[0], opts[i] = opts[i], opts[0]
			return
		}
	}
}

func pbcopy(text string) error {
	c := exec.Command("pbcopy")
	c.Stdin = strings.NewReader(text)
	if err := c.Run(); err != nil {
		return fmt.Errorf("pbcopy: %w", err)
	}
	return nil
}

func openURL(url string) error {
	if err := exec.Command("open", url).Run(); err != nil {
		return fmt.Errorf("open %s: %w", url, err)
	}
	return nil
}

func runShell(command string) error {
	c := exec.Command("sh", "-c", command)
	if out, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
