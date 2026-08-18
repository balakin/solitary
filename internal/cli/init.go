package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/balakin/solitary/internal/config"
)

func newInitCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Scaffold a new cell definition",
		Long: "Creates ~/.config/solitary/cells/<name>/cell.yaml.\n" +
			"Refuses to overwrite an existing cell unless --force is given.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			path, err := config.InitCell(name, force)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Created %s\n", path)
			fmt.Fprintf(out, "Edit it, then run: solitary up %s\n", name)

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing cell definition")

	return cmd
}
