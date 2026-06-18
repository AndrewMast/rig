package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	toml "github.com/pelletier/go-toml/v2"
)

// HandoffMethod names a GitHub-mutation delivery method.
type HandoffMethod string

const (
	MethodClipboard HandoffMethod = "clipboard"
	MethodDrop      HandoffMethod = "drop"
	MethodLink      HandoffMethod = "link"
	MethodPrint     HandoffMethod = "print"
	MethodFile      HandoffMethod = "file"
	MethodGH        HandoffMethod = "gh"
)

// LauncherTarget is a launcher's resolution policy.
type LauncherTarget string

const (
	// TargetProject resolves to a project only.
	TargetProject LauncherTarget = "project"
	// TargetFolder is project-first but folder-capable.
	TargetFolder LauncherTarget = "folder"
)

// Launcher is a config-defined subcommand that runs a templated command against
// a resolved path.
type Launcher struct {
	Command string         `toml:"command"`
	Target  LauncherTarget `toml:"target,omitempty"`
}

// Config mirrors config.toml. Secrets never live here: the GitHub token is read
// from its own 0600 file referenced by GitHub.TokenFile.
type Config struct {
	DefaultBase string `toml:"default_base"`

	Handoff struct {
		Method        HandoffMethod `toml:"method"`
		AlwaysConfirm bool          `toml:"always_confirm"`
	} `toml:"handoff"`

	GitHub struct {
		TokenFile string `toml:"token_file"`
	} `toml:"github"`

	Guard struct {
		ExpectedUser string `toml:"expected_user,omitempty"`
		ExpectedHost string `toml:"expected_host,omitempty"`
	} `toml:"guard"`

	Launchers map[string]Launcher `toml:"launchers"`
}

// DefaultLaunchers are shipped out of the box. Project-only tools target the
// project; editors/finder are folder-capable.
func DefaultLaunchers() map[string]Launcher {
	return map[string]Launcher{
		"solo":      {Command: "solo open {path}", Target: TargetProject},
		"merge":     {Command: "smerge {path}", Target: TargetProject},
		"gitbutler": {Command: "open -a GitButler {path}", Target: TargetProject},
		"sublime":   {Command: "subl {path}", Target: TargetFolder},
		"finder":    {Command: "open {path}", Target: TargetFolder},
	}
}

// Default returns the built-in config used when no file exists, seeded from the
// resolved paths.
func Default(p Paths) *Config {
	c := &Config{DefaultBase: p.DefaultBase}
	c.Handoff.Method = MethodClipboard
	c.Handoff.AlwaysConfirm = false
	c.GitHub.TokenFile = p.Token
	c.Launchers = DefaultLaunchers()
	return c
}

// Load reads config.toml at p.Config, merging it over the built-in defaults. A
// missing file yields the defaults. Shipped launchers are always present unless
// the file explicitly redefines them.
func Load(p Paths) (*Config, error) {
	c := Default(p)
	data, err := os.ReadFile(p.Config)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	// Decode the file over a zero value so we can tell which keys were set,
	// then merge into the defaults.
	var file Config
	if err := toml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", p.Config, err)
	}
	if file.DefaultBase != "" {
		c.DefaultBase = file.DefaultBase
	}
	if file.Handoff.Method != "" {
		c.Handoff.Method = file.Handoff.Method
	}
	c.Handoff.AlwaysConfirm = file.Handoff.AlwaysConfirm
	if file.GitHub.TokenFile != "" {
		c.GitHub.TokenFile = file.GitHub.TokenFile
	}
	c.Guard = file.Guard
	for name, l := range file.Launchers {
		if l.Target == "" {
			l.Target = TargetProject
		}
		c.Launchers[name] = l
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// Save writes the config to p.Config atomically (temp + rename).
func Save(p Paths, c *Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p.Config), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(p.Config), ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Rename(tmpName, p.Config); err != nil {
		return fmt.Errorf("commit config: %w", err)
	}
	return nil
}

// Validate checks the config for well-formedness.
func (c *Config) Validate() error {
	if c.DefaultBase == "" {
		return fmt.Errorf("default_base must be set")
	}
	switch c.Handoff.Method {
	case MethodClipboard, MethodDrop, MethodLink, MethodPrint, MethodFile, MethodGH:
	default:
		return fmt.Errorf("invalid handoff.method %q (want clipboard|drop|link|print|file|gh)", c.Handoff.Method)
	}
	for name, l := range c.Launchers {
		if l.Command == "" {
			return fmt.Errorf("launcher %q: command must be set", name)
		}
		switch l.Target {
		case "", TargetProject, TargetFolder:
		default:
			return fmt.Errorf("launcher %q: invalid target %q (want project|folder)", name, l.Target)
		}
	}
	return nil
}

// TokenFile returns the absolute token-file path (config value, tilde-expanded,
// falling back to the default).
func (c *Config) TokenFile(p Paths) string {
	tf := c.GitHub.TokenFile
	if tf == "" {
		return p.Token
	}
	return expandTilde(tf)
}

// LauncherNames returns launcher names in sorted order.
func (c *Config) LauncherNames() []string {
	names := make([]string, 0, len(c.Launchers))
	for n := range c.Launchers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
