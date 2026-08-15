package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/dm-balakin/solitary/internal/cell"
)

func newFetchCmd() *cobra.Command {
	var (
		into  string
		force bool
		list  bool
	)

	cmd := &cobra.Command{
		Use:   "fetch <name> [file...]",
		Short: "Collect what a cell has published",
		Long: "Copies files out of a cell's outbox, which is where anything inside it\n" +
			"leaves something to hand over: 'artifact <file>' in the cell puts a copy\n" +
			"there, and this brings it here. With no file names, everything is taken.\n\n" +
			"Copies rather than moves — the cell keeps its copy, so fetching twice is\n" +
			"not a mistake. Files land in the current directory unless --into names\n" +
			"another, are never written over something already there without --force,\n" +
			"and never arrive executable.\n\n" +
			"The cell's machine has to be running; its container does not, so what a\n" +
			"cell published can still be collected after whatever produced it died.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if list {
				return listArtifacts(cmd, args[0])
			}

			return cell.Fetch(args[0], args[1:], into, force, out)
		},
	}

	cmd.Flags().StringVar(&into, "into", "", "directory to fetch into (default: the current one)")
	cmd.Flags().BoolVar(&force, "force", false, "replace files already there")
	cmd.Flags().BoolVar(&list, "list", false, "show what the cell has published without copying it")

	return cmd
}

func listArtifacts(cmd *cobra.Command, name string) error {
	artifacts, err := cell.Artifacts(name)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if len(artifacts) == 0 {
		fmt.Fprintf(out, "Cell %q has published nothing.\n"+
			"Inside the cell, 'artifact <file>' puts a copy where this can find it.\n", name)
		return nil
	}

	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SIZE\tNAME")
	for _, artifact := range artifacts {
		if !artifact.OK() {
			// Named rather than hidden: something in the cell meant to
			// hand this over, and it is not going to come out.
			fmt.Fprintf(w, "-\t(cannot be fetched) %s\n", artifact.Problem)
			continue
		}
		fmt.Fprintf(w, "%s\t%s\n", cell.Size(artifact.Size), artifact.Name)
	}

	return w.Flush()
}
