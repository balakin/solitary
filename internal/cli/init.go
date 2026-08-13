package cli

import "github.com/spf13/cobra"

func newInitCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Scaffold a new cell definition",
		Long: "Creates ~/.config/solitary/cells/<name>/cell.yaml.\n" +
			"Refuses to overwrite an existing cell unless --force is given.",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return errNotImplemented
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing cell definition")

	return cmd
}
