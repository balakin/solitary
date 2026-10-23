package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/balakin/solitary/internal/cell"
)

func newUpCmd() *cobra.Command {
	var detach, rebuild, newMachine, force bool

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
			"again, and replaces the container. The cell's home is kept.\n\n" +
			"--new-machine replaces the machine itself with a new one, created from\n" +
			"the base solitary uses now and patched, and carries the cell's home\n" +
			"across through the host. Nothing else on the old machine's disk comes\n" +
			"with it: what vm.provision or anyone installed there is gone, and the\n" +
			"image is built or pulled again. The old machine is only destroyed once\n" +
			"its home has been saved and read back in full; a move that stops\n" +
			"halfway keeps the home and finishes at the next up.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if newMachine {
				if rebuild {
					return fmt.Errorf("--new-machine builds or pulls the image afresh already; --rebuild adds nothing to it")
				}
				if !force {
					ok, err := confirm(cmd, fmt.Sprintf("Replace the machine behind %q? Its home is carried across; everything else\n"+
						"on its disk is not, and everything running in it stops. [y/N] ", name))
					if err != nil {
						return err
					}
					if !ok {
						fmt.Fprintln(cmd.ErrOrStderr(), "Cancelled.")
						return nil
					}
				}
				if err := cell.NewMachine(name, cmd.ErrOrStderr()); err != nil {
					return err
				}
			} else if err := cell.Up(name, rebuild, cmd.ErrOrStderr()); err != nil {
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
	cmd.Flags().BoolVar(&newMachine, "new-machine", false, "replace the machine with a new one, carrying the home across")
	cmd.Flags().BoolVar(&force, "force", false, "with --new-machine, skip the confirmation prompt")

	return cmd
}
