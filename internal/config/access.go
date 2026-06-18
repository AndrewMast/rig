package config

import (
	"fmt"
	"strconv"
	"strings"
)

// Get returns the string form of a single dotted config key (e.g.
// "handoff.method", "launchers.solo.command"). It is the read half of the
// scriptable get/set pair.
func (c *Config) Get(key string) (string, error) {
	switch key {
	case "default_base":
		return c.DefaultBase, nil
	case "handoff.method":
		return string(c.Handoff.Method), nil
	case "handoff.always_confirm":
		return strconv.FormatBool(c.Handoff.AlwaysConfirm), nil
	case "github.token_file":
		return c.GitHub.TokenFile, nil
	case "guard.expected_user":
		return c.Guard.ExpectedUser, nil
	case "guard.expected_host":
		return c.Guard.ExpectedHost, nil
	}
	if name, field, ok := launcherKey(key); ok {
		l, exists := c.Launchers[name]
		if !exists {
			return "", fmt.Errorf("launcher %q not defined", name)
		}
		switch field {
		case "command":
			return l.Command, nil
		case "target":
			return string(l.Target), nil
		}
	}
	return "", fmt.Errorf("unknown config key %q", key)
}

// Set assigns a single dotted config key from its string form and re-validates.
func (c *Config) Set(key, val string) error {
	switch key {
	case "default_base":
		c.DefaultBase = val
	case "handoff.method":
		c.Handoff.Method = HandoffMethod(val)
	case "handoff.always_confirm":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("handoff.always_confirm: want true/false, got %q", val)
		}
		c.Handoff.AlwaysConfirm = b
	case "github.token_file":
		c.GitHub.TokenFile = val
	case "guard.expected_user":
		c.Guard.ExpectedUser = val
	case "guard.expected_host":
		c.Guard.ExpectedHost = val
	default:
		name, field, ok := launcherKey(key)
		if !ok {
			return fmt.Errorf("unknown config key %q", key)
		}
		if c.Launchers == nil {
			c.Launchers = map[string]Launcher{}
		}
		l := c.Launchers[name]
		switch field {
		case "command":
			l.Command = val
		case "target":
			l.Target = LauncherTarget(val)
		default:
			return fmt.Errorf("unknown launcher field %q", field)
		}
		c.Launchers[name] = l
	}
	return c.Validate()
}

// launcherKey parses "launchers.<name>.<field>".
func launcherKey(key string) (name, field string, ok bool) {
	const prefix = "launchers."
	if !strings.HasPrefix(key, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(key, prefix)
	dot := strings.LastIndex(rest, ".")
	if dot < 1 {
		return "", "", false
	}
	return rest[:dot], rest[dot+1:], true
}
