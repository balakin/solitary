package cli

import (
	"github.com/spf13/cobra"

	"github.com/dm-balakin/solitary/internal/cell"
)

func newDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down <name>",
		Short: "Stop a cell",
		Long:  "Stops the cell's VM. The disk and the host-side secrets file are kept.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cell.Down(args[0], cmd.ErrOrStderr())
		},
	}
}
