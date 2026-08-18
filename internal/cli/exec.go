package cli

import (
	"github.com/spf13/cobra"

	"github.com/balakin/solitary/internal/cell"
)

func newExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec <name> <command> [args...]",
		Short: "Run one command in a running cell",
		Long: "Runs a single command inside the cell's container and exits with its\n" +
			"status. Like shell, it never changes state: it fails if the cell is\n" +
			"stopped or absent.\n\n" +
			"The command is run directly, not through a shell, so pipes and\n" +
			"redirection belong to the host unless you ask for a shell yourself:\n\n" +
			"  solitary exec claude git status\n" +
			"  solitary exec claude bash -lc 'git log | head'\n\n" +
			"Flags after the command name are the command's own. Use -- before it\n" +
			"if it starts with a flag.",
		Args: cobra.MinimumNArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			return cell.Exec(args[0], args[1:])
		},
	}
	// Stop parsing flags at the first argument, so that solitary does not
	// take -l in 'exec <name> ls -l' for one of its own.
	cmd.Flags().SetInterspersed(false)

	return cmd
}
