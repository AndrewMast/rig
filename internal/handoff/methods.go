package handoff

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Env carries the side-effecting capabilities a delivery method needs. Tests
// inject fakes; the CLI wires real implementations (pbcopy, open, sh -c).
type Env struct {
	Out       io.Writer
	FileDir   string                  // where the file method writes scripts
	Clipboard func(text string) error // copy text to the clipboard
	Open      func(url string) error  // open a URL in the browser
	Run       func(command string) error
}

// Method delivers a batch. Each method is small and self-contained so new ones
// drop in without touching the core.
type Method func(env Env, b Batch) error

// methods is the registry keyed by method name.
var methods = map[string]Method{
	"print":     deliverPrint,
	"clipboard": deliverClipboard,
	"file":      deliverFile,
	"gh":        deliverGH,
	"link":      deliverLink,
	"drop":      deliverDrop,
}

// Deliver dispatches a batch to the named method.
func Deliver(method string, env Env, b Batch) error {
	fn, ok := methods[method]
	if !ok {
		return fmt.Errorf("unknown handoff method %q", method)
	}
	if len(b.Mutations) == 0 {
		return nil
	}
	return fn(env, b)
}

// Methods returns the registered method names (for validation/help).
func Methods() []string {
	out := make([]string, 0, len(methods))
	for n := range methods {
		out = append(out, n)
	}
	return out
}

func script(b Batch) string {
	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n")
	sb.WriteString("set -e\n")
	for _, m := range b.Mutations {
		fmt.Fprintf(&sb, "# %s\n%s\n", m.Title, m.Command)
	}
	return sb.String()
}

func deliverPrint(env Env, b Batch) error {
	fmt.Fprintf(env.Out, "# GitHub mutations for %s — run where gh is authenticated:\n", b.Repo)
	for _, m := range b.Mutations {
		fmt.Fprintf(env.Out, "# %s\n%s\n", m.Title, m.Command)
	}
	return nil
}

func deliverClipboard(env Env, b Batch) error {
	if env.Clipboard == nil {
		return fmt.Errorf("clipboard not available")
	}
	if err := env.Clipboard(strings.Join(b.Commands(), "\n") + "\n"); err != nil {
		return err
	}
	fmt.Fprintf(env.Out, "copied %d GitHub mutation(s) for %s to the clipboard\n", len(b.Mutations), b.Repo)
	return nil
}

func deliverFile(env Env, b Batch) error {
	dir := env.FileDir
	if dir == "" {
		return fmt.Errorf("file method: no output directory configured")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create handoff dir: %w", err)
	}
	name := fmt.Sprintf("handoff-%s.sh", strings.ReplaceAll(b.Repo, "/", "-"))
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script(b)), 0o755); err != nil {
		return fmt.Errorf("write handoff script: %w", err)
	}
	fmt.Fprintf(env.Out, "wrote handoff script: %s\n", path)
	return nil
}

func deliverGH(env Env, b Batch) error {
	if env.Run == nil {
		return fmt.Errorf("command runner not available")
	}
	for _, m := range b.Mutations {
		fmt.Fprintf(env.Out, "running: %s\n", m.Title)
		if err := env.Run(m.Command); err != nil {
			return fmt.Errorf("%s: %w", m.Title, err)
		}
	}
	return nil
}

func deliverLink(env Env, b Batch) error {
	if env.Open == nil {
		return fmt.Errorf("opener not available")
	}
	for _, m := range b.Mutations {
		fmt.Fprintf(env.Out, "%s\n  open: %s\n", m.Title, m.WebURL)
		if err := env.Open(m.WebURL); err != nil {
			return err
		}
		if m.PublicKey != "" && env.Clipboard != nil {
			if err := env.Clipboard(m.PublicKey); err != nil {
				return err
			}
			fmt.Fprintln(env.Out, "  public key copied to clipboard — paste it, set the title, tick write if needed")
		}
	}
	return nil
}

func deliverDrop(env Env, b Batch) error {
	if env.Run == nil {
		return fmt.Errorf("command runner not available")
	}
	// Pipe the script into the shared `drop` vault CLI.
	cmd := fmt.Sprintf("printf %s | drop", shellQuote(script(b)))
	if err := env.Run(cmd); err != nil {
		return fmt.Errorf("drop: %w (is the `drop` CLI installed?)", err)
	}
	fmt.Fprintf(env.Out, "dropped %d GitHub mutation(s) for %s\n", len(b.Mutations), b.Repo)
	return nil
}
