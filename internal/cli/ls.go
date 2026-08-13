package cli

import "github.com/spf13/cobra"

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List cells and their state",
		Long: "Lists every cell definition alongside the state of its VM and the\n" +
			"container inside it: uninitialized, stopped, running, degraded (VM up\n" +
			"but container not running) or broken.",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errNotImplemented
		},
	}
}
