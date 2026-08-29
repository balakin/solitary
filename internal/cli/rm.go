package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/balakin/solitary/internal/cell"
)

func newRmCmd() *cobra.Command {
	var (
		force   bool
		orphans bool
	)

	cmd := &cobra.Command{
		Use:   "rm [name]",
		Short: "Destroy a cell's VM",
		Long: "Stops and deletes the cell's VM, discarding everything written inside\n" +
			"it. The cell definition and its secrets file are left on the host, so\n" +
			"up rebuilds an equivalent cell from scratch.\n\n" +
			"With --orphans and no name, destroys every VM whose cell definition is\n" +
			"gone — the ones ls shows as orphaned. Deleting a cell directory leaves\n" +
			"its VM behind, still holding the disk it was given.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if orphans {
				if len(args) > 0 {
					return fmt.Errorf("--orphans removes every orphaned machine, so it takes no name")
				}
				return removeOrphans(cmd, force)
			}
			if len(args) == 0 {
				return fmt.Errorf("rm needs the name of a cell, or --orphans to sweep the machines no cell claims")
			}

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
	cmd.Flags().BoolVar(&orphans, "orphans", false, "destroy every machine whose cell definition is gone")

	return cmd
}

// removeOrphans destroys the machines no definition claims. They are named in
// the prompt rather than counted: nothing else on the host says what they are,
// so the confirmation is the last chance to recognise one that should be kept.
func removeOrphans(cmd *cobra.Command, force bool) error {
	names, err := cell.Orphans()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "No orphaned machines.")
		return nil
	}

	if !force {
		ok, err := confirm(cmd, fmt.Sprintf(
			"Destroy %s with no cell (%s)? Everything inside is lost. [y/N] ",
			machineCount(len(names)), strings.Join(names, ", ")))
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(cmd.ErrOrStderr(), "Cancelled.")
			return nil
		}
	}

	for _, name := range names {
		if err := cell.Remove(name, cmd.ErrOrStderr()); err != nil {
			return err
		}
	}

	return nil
}

func machineCount(n int) string {
	if n == 1 {
		return "1 machine"
	}
	return fmt.Sprintf("%d machines", n)
}
