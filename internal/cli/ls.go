package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/dm-balakin/solitary/internal/cell"
)

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List cells and their state",
		Long: "Lists every cell definition alongside the state of its VM and the\n" +
			"container inside it: uninitialized, stopped, running, degraded (VM up\n" +
			"but container not running) or broken.",
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

			return w.Flush()
		},
	}
}
