package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AndrewMast/rig/internal/config"
	"github.com/AndrewMast/rig/internal/gh"
	"github.com/spf13/cobra"
)

func newConfigTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage the optional read-only GitHub metadata token",
	}
	cmd.AddCommand(newTokenSetCmd(), newTokenRemoveCmd(), newTokenStatusCmd())
	return cmd
}

func newTokenSetCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Store the metadata token (hidden prompt; verified via API probe)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app := appFrom(cmd)
			tok, err := app.UI.Secret("GitHub metadata token")
			if err != nil {
				return err
			}
			if tok == "" {
				return fmt.Errorf("no token entered")
			}

			// Relocate + wire token_file when --file is given.
			if file != "" {
				app.Config.GitHub.TokenFile = file
				if err := config.Save(app.Paths, app.Config); err != nil {
					return err
				}
			}
			path := app.Config.TokenFile(app.Paths)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte(tok+"\n"), 0o600); err != nil {
				return fmt.Errorf("write token: %w", err)
			}
			// Verify via API probe.
			if err := gh.New(tok).Verify(context.Background()); err != nil {
				cmd.Printf("token saved to %s, but verification failed: %v\n", path, err)
				return nil
			}
			cmd.Printf("token saved to %s and verified\n", path)
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "store the token at this path and wire token_file")
	return cmd
}

func newTokenRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove",
		Short: "Delete the stored token file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app := appFrom(cmd)
			path := app.Config.TokenFile(app.Paths)
			if err := os.Remove(path); err != nil {
				if os.IsNotExist(err) {
					cmd.Println("no token file to remove")
					return nil
				}
				return err
			}
			cmd.Printf("removed %s\n", path)
			return nil
		},
	}
}

func newTokenStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Report token presence and validity (never prints the token)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app := appFrom(cmd)
			tok := gh.LoadToken(app.Config.TokenFile(app.Paths))
			if tok == "" {
				cmd.Println("token: absent")
				return nil
			}
			source := app.Config.TokenFile(app.Paths)
			if os.Getenv("RIG_GH_TOKEN") != "" {
				source = "RIG_GH_TOKEN (env)"
			}
			if err := gh.New(tok).Verify(context.Background()); err != nil {
				cmd.Printf("token: present (%s) but invalid: %v\n", source, err)
				return nil
			}
			cmd.Printf("token: present (%s) and valid\n", source)
			return nil
		},
	}
}
