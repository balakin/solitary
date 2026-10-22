package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/balakin/solitary/internal/cell"
	"github.com/balakin/solitary/internal/secrets"
)

func newPatchCmd() *cobra.Command {
	var reboot bool

	cmd := &cobra.Command{
		Use:   "patch <name>",
		Short: "Install every update the cell's machine is missing",
		Long: "Upgrades the system of the cell's machine — the Ubuntu its container runs\n" +
			"on — with everything its release has, security fixes included, rather\n" +
			"than waiting for unattended-upgrades. The container's own system comes\n" +
			"from its image; up --rebuild is what updates that.\n\n" +
			"A stopped cell is patched with its machine alone and stopped again, so\n" +
			"its container never starts. A running cell keeps running; when the patch\n" +
			"needs a reboot, which ends everything in the container, it asks first,\n" +
			"and --reboot answers yes. The home is kept either way.\n\n" +
			"A cell with network.allow reaches Ubuntu's archive for this whatever the\n" +
			"list says, from apt alone. Any other source its machine fetches from has\n" +
			"to be listed, and patch names the ones that are not.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			return cell.Patch(name, func() (bool, error) {
				if reboot {
					return true, nil
				}
				if !secrets.CanPrompt() {
					return false, nil
				}
				return confirm(cmd, fmt.Sprintf("Restart %q to finish? Everything running in its container stops; its home is kept. [y/N] ", name))
			}, cmd.ErrOrStderr())
		},
	}

	cmd.Flags().BoolVar(&reboot, "reboot", false, "restart a running cell without asking when the patch needs it")

	return cmd
}
