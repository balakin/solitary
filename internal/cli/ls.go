package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/balakin/solitary/internal/cell"
)

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List cells and their state",
		Long: "Lists every cell definition alongside the state of its VM and the\n" +
			"container inside it: uninitialized, stopped, running, degraded (VM up\n" +
			"but container not running) or broken. Below the table, anything about a\n" +
			"cell that will not right itself, with what to do about it.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cells, err := cell.List()
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if len(cells) == 0 {
				fmt.Fprintln(out, "No cells yet. Create one with: solitary init <name>")
				return nil
			}

			w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NAME\tSTATUS\tIMAGE")
			for _, c := range cells {
				fmt.Fprintf(w, "%s\t%s\t%s\n", c.Name, c.Status, c.Image)
			}
			if err := w.Flush(); err != nil {
				return err
			}

			// Below the table rather than in it, so that the columns stay
			// what they were for anything that reads them.
			first := true
			for _, c := range cells {
				for _, a := range c.Attention {
					if first {
						fmt.Fprintln(out)
						first = false
					}
					fmt.Fprintf(out, "! %s: %s\n  %s\n", c.Name, a.What, a.Fix)
				}
			}

			return nil
		},
	}
}
