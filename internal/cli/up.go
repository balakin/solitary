package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/balakin/solitary/internal/cell"
)

func newUpCmd() *cobra.Command {
	var detach, rebuild bool

	cmd := &cobra.Command{
		Use:   "up <name>",
		Short: "Start a cell and attach to it",
		Long: "Boots the cell's VM if it does not exist or is stopped, starts the\n" +
			"container, prompts for any declared secrets that are missing, then\n" +
			"opens a shell inside the container.\n\n" +
			"The cell has to be defined already: 'init' scaffolds one and 'clone'\n" +
			"takes one from a repository.\n\n" +
			"--rebuild is how the tools in a cell are updated. The container is\n" +
			"started over from its image at every boot, so an update made inside it\n" +
			"does not last; this builds the image again without a cache, or pulls it\n" +
			"again, and replaces the container. The cell's home is kept.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if err := cell.Up(name, rebuild, cmd.ErrOrStderr()); err != nil {
				return err
			}

			if detach {
				fmt.Fprintf(cmd.ErrOrStderr(), "Cell %q is running. Enter it with: solitary shell %s\n", name, name)
				return nil
			}

			return cell.Shell(name)
		},
	}

	cmd.Flags().BoolVar(&detach, "detach", false, "start the cell without attaching a shell")
	cmd.Flags().BoolVar(&rebuild, "rebuild", false, "build or pull the image afresh and replace the container")

	return cmd
}
