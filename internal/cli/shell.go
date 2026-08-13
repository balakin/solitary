package cli

import "github.com/spf13/cobra"

func newShellCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell <name>",
		Short: "Open a shell in a running cell",
		Long: "Attaches to a cell that is already running. Unlike up, this command\n" +
			"never changes state: it fails if the cell is stopped or absent.",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return errNotImplemented
		},
	}
}
