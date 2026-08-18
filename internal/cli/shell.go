package cli

import (
	"github.com/spf13/cobra"

	"github.com/balakin/solitary/internal/cell"
)

func newShellCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell <name>",
		Short: "Open a shell in a running cell",
		Long: "Attaches to a cell that is already running. Unlike up, this command\n" +
			"never changes state: it fails if the cell is stopped or absent.",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return cell.Shell(args[0])
		},
	}
}
