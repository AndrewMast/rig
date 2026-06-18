// Package config resolves rig's on-disk locations and loads/saves config.toml.
//
// In normal operation paths are XDG-based under ~/.config/rig with SSH keys in
// ~/.ssh. Setting RIG_HOME relocates everything — registry, token, ssh dir, and
// project base — under one throwaway root for disposable dev mode, where the
// optional safety guards are skipped.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Paths is the resolved set of locations rig reads and writes.
type Paths struct {
	// Home is RIG_HOME when set, otherwise the XDG config dir for rig.
	Home string
	// ConfigDir holds config.toml, the registry, and the default token file.
	ConfigDir string
	// Config is the config.toml path.
	Config string
	// Registry is the JSON manifest path (off-volume in normal use).
	Registry string
	// Token is the default token file path (config may override it).
	Token string
	// SSHDir is where deploy keys live.
	SSHDir string
	// DefaultBase is the fallback project base when config sets none.
	DefaultBase string
	// DevMode is true when RIG_HOME is set: a self-contained, guard-free root.
	DevMode bool
}

// ResolvePaths computes rig's locations from the environment.
func ResolvePaths() (Paths, error) {
	if home := os.Getenv("RIG_HOME"); home != "" {
		home, err := filepath.Abs(home)
		if err != nil {
			return Paths{}, fmt.Errorf("resolve RIG_HOME: %w", err)
		}
		return Paths{
			Home:        home,
			ConfigDir:   home,
			Config:      filepath.Join(home, "config.toml"),
			Registry:    filepath.Join(home, "registry.json"),
			Token:       filepath.Join(home, "token"),
			SSHDir:      filepath.Join(home, "ssh"),
			DefaultBase: filepath.Join(home, "projects"),
			DevMode:     true,
		}, nil
	}

	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return Paths{}, fmt.Errorf("resolve home dir: %w", err)
		}
		configHome = filepath.Join(h, ".config")
	}
	dir := filepath.Join(configHome, "rig")
	sshHome, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve home dir: %w", err)
	}
	return Paths{
		Home:        dir,
		ConfigDir:   dir,
		Config:      filepath.Join(dir, "config.toml"),
		Registry:    filepath.Join(dir, "registry.json"),
		Token:       filepath.Join(dir, "token"),
		SSHDir:      filepath.Join(sshHome, ".ssh"),
		DefaultBase: "/Volumes/Development",
		DevMode:     false,
	}, nil
}

// expandTilde turns a leading ~ into the user's home directory.
func expandTilde(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, strings.TrimPrefix(strings.TrimPrefix(p, "~"), "/"))
		}
	}
	return p
}
