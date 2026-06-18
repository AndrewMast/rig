package cli

import "github.com/spf13/cobra"

// newSelfCmd groups rig's self-management commands. update/uninstall land here
// later; version is wired now so the binary reports its build stamp.
func newSelfCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "self",
		Short: "Manage the rig binary itself (update, uninstall, version)",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the rig version",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			c.Printf("rig %s\n", version)
			return nil
		},
	})
	return cmd
}
