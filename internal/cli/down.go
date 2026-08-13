package cli

import "github.com/spf13/cobra"

func newDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down <name>",
		Short: "Stop a cell",
		Long:  "Stops the cell's VM. The disk and the host-side secrets file are kept.",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return errNotImplemented
		},
	}
}
