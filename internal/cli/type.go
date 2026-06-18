package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/AndrewMast/rig/internal/types"
	"github.com/spf13/cobra"
)

func (a *App) typesDir() string { return filepath.Join(a.Paths.ConfigDir, "types") }

func newTypeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "type",
		Short: "Manage reusable project types (hooks + commands)",
	}
	cmd.AddCommand(newTypeListCmd(), newTypeShowCmd(), newTypeNewCmd(), newTypeDeleteCmd())
	return cmd
}

func newTypeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List defined types",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app := appFrom(cmd)
			entries, err := os.ReadDir(app.typesDir())
			if os.IsNotExist(err) {
				cmd.Println("no types defined")
				return nil
			}
			if err != nil {
				return err
			}
			var names []string
			for _, e := range entries {
				if e.IsDir() {
					names = append(names, e.Name())
				}
			}
			sort.Strings(names)
			if len(names) == 0 {
				cmd.Println("no types defined")
				return nil
			}
			for _, n := range names {
				cmd.Println(n)
			}
			return nil
		},
	}
}

func newTypeShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <type>",
		Short: "Print a type's hooks and commands",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			prof, err := types.LoadType(app.typesDir(), args[0])
			if err != nil {
				return err
			}
			cmd.Printf("type %s\n", args[0])
			cmd.Println("  hooks:")
			for _, k := range sortedKeys(prof.Hooks) {
				cmd.Printf("    %-9s %s\n", k, prof.Hooks[k])
			}
			cmd.Println("  commands:")
			for _, k := range sortedKeys(prof.Commands) {
				cmd.Printf("    %-9s %s\n", k, prof.Commands[k])
			}
			return nil
		},
	}
}

func newTypeNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <type>",
		Short: "Scaffold a new type with a template type.toml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			dir := filepath.Join(app.typesDir(), args[0])
			if _, err := os.Stat(dir); err == nil {
				return fmt.Errorf("type %q already exists", args[0])
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(types.TypeFile(app.typesDir(), args[0]), []byte(typeTemplate), 0o644); err != nil {
				return err
			}
			cmd.Printf("created type %s at %s\n", args[0], types.TypeFile(app.typesDir(), args[0]))
			return nil
		},
	}
}

func newTypeDeleteCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <type>",
		Short: "Delete a type",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app := appFrom(cmd)
			dir := filepath.Join(app.typesDir(), args[0])
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				return fmt.Errorf("type %q not found", args[0])
			}
			ok, err := app.confirm(fmt.Sprintf("Delete type %q?", args[0]), false, yes)
			if err != nil || !ok {
				return err
			}
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
			cmd.Printf("deleted type %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

const typeTemplate = `[hooks]
# preflight = "check-tools"      # run before clone/adopt; abort early on failure
# setup     = "make bootstrap"   # post-clone/adopt bootstrap
# create    = "make scaffold"    # scaffold a brand-new project

[commands]
# test = "make test"
# run  = "make run"
`
