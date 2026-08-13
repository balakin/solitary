package cli

import "github.com/spf13/cobra"

func newRmCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "rm <name>",
		Short: "Destroy a cell's VM",
		Long: "Stops and deletes the cell's VM, discarding everything written inside\n" +
			"it. The cell definition and its secrets file are left on the host, so\n" +
			"up rebuilds an equivalent cell from scratch.",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return errNotImplemented
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "skip the confirmation prompt")

	return cmd
}
