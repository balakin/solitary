package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dm-balakin/solitary/internal/cell"
)

func newRmCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "rm <name>",
		Short: "Destroy a cell's VM",
		Long: "Stops and deletes the cell's VM, discarding everything written inside\n" +
			"it. The cell definition and its secrets file are left on the host, so\n" +
			"up rebuilds an equivalent cell from scratch.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if !force {
				ok, err := confirm(cmd, fmt.Sprintf("Destroy the machine behind %q? Everything inside it is lost. [y/N] ", name))
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(cmd.ErrOrStderr(), "Cancelled.")
					return nil
				}
			}

			return cell.Remove(name, cmd.ErrOrStderr())
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "skip the confirmation prompt")

	return cmd
}
