package cli

import "github.com/spf13/cobra"

func newSecretsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "secrets <name>",
		Short: "Set the secrets a cell is allowed to see",
		Long: "Prompts for each name listed under secrets: in the cell definition and\n" +
			"writes the values to the cell's host-side .env file. Values already set\n" +
			"are kept unless a new one is typed. If the cell is running, prints the\n" +
			"command needed to restart it so the change takes effect.",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return errNotImplemented
		},
	}
}
