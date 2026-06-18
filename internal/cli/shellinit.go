package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newShellInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "shell-init [zsh|bash]",
		Short:     "Emit shell integration (the rig() wrapper + completions)",
		Long:      "Prints shell code to eval, e.g. eval \"$(rig shell-init zsh)\". The wrapper lets `rig cd` change the parent shell's directory and loads completions.",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"zsh", "bash"},
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := "zsh"
			if len(args) == 1 {
				shell = args[0]
			}
			switch shell {
			case "zsh":
				cmd.Print(shellInitZsh)
			case "bash":
				cmd.Print(shellInitBash)
			default:
				return fmt.Errorf("unsupported shell %q (want zsh or bash)", shell)
			}
			return nil
		},
	}
	return cmd
}

const shellInitZsh = `# rig shell integration (zsh)
rig() {
  if [ "$1" = "cd" ]; then
    shift
    local __rig_dir
    __rig_dir="$(command rig path "$@")" || return $?
    builtin cd "$__rig_dir"
  else
    command rig "$@"
  fi
}
if command -v compdef >/dev/null 2>&1; then
  source <(command rig completion zsh) 2>/dev/null
fi
`

const shellInitBash = `# rig shell integration (bash)
rig() {
  if [ "$1" = "cd" ]; then
    shift
    local __rig_dir
    __rig_dir="$(command rig path "$@")" || return $?
    builtin cd "$__rig_dir"
  else
    command rig "$@"
  fi
}
if command -v complete >/dev/null 2>&1; then
  source <(command rig completion bash) 2>/dev/null
fi
`
