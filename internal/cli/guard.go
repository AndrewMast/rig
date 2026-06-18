package cli

import (
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/spf13/cobra"
)

// guardExemptRoots are top-level commands that always run regardless of the
// optional [guard], so you can never lock yourself out of inspecting or fixing
// configuration on the "wrong" machine.
var guardExemptRoots = map[string]bool{
	"config":     true, // inspect/unset the guard itself
	"help":       true,
	"completion": true,
	"shell-init": true,
	"self":       true, // version/update/uninstall
}

// checkGuard enforces the optional [guard] (expected_user/expected_host). It is
// off unless set, and skipped entirely in disposable dev mode (RIG_HOME). On a
// mismatch it warns; interactively it asks to continue, and when scripted it
// refuses — so a misconfigured host can't silently mutate state.
func (a *App) checkGuard(cmd *cobra.Command) error {
	if a.Paths.DevMode {
		return nil
	}
	eu, eh := a.Config.Guard.ExpectedUser, a.Config.Guard.ExpectedHost
	if eu == "" && eh == "" {
		return nil
	}
	if guardExemptRoots[topLevelName(cmd)] {
		return nil
	}

	var problems []string
	if eu != "" {
		if cur := currentUser(); !strings.EqualFold(cur, eu) {
			problems = append(problems, fmt.Sprintf("user is %q, expected %q", cur, eu))
		}
	}
	if eh != "" {
		if h, _ := os.Hostname(); !strings.EqualFold(h, eh) {
			problems = append(problems, fmt.Sprintf("host is %q, expected %q", h, eh))
		}
	}
	if len(problems) == 0 {
		return nil
	}

	fmt.Fprintf(os.Stderr, "rig: guard tripped — %s\n", strings.Join(problems, "; "))
	ok, err := a.UI.Confirm("Continue anyway?", false)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("refused by guard (set/clear [guard] in config to change this)")
	}
	return nil
}

// topLevelName returns the first-level command name under root (e.g. for
// "rig project origin add" it returns "project").
func topLevelName(cmd *cobra.Command) string {
	c := cmd
	for c.HasParent() && c.Parent().HasParent() {
		c = c.Parent()
	}
	return c.Name()
}

// currentUser returns the current login name, falling back to $USER.
func currentUser() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return os.Getenv("USER")
}
