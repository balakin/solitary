package cli

import "github.com/spf13/cobra"

func newUpCmd() *cobra.Command {
	var (
		as     string
		detach bool
	)

	cmd := &cobra.Command{
		Use:   "up <name|image-ref>",
		Short: "Start a cell and attach to it",
		Long: "Boots the cell's VM if it does not exist or is stopped, starts the\n" +
			"container, prompts for any declared secrets that are missing, then\n" +
			"opens a shell inside the container.\n\n" +
			"Given an image reference rather than a known cell name, a cell is\n" +
			"created from it using default settings. If a cell of the derived name\n" +
			"already exists with a different image, the command fails and asks for\n" +
			"--as.",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return errNotImplemented
		},
	}

	cmd.Flags().StringVar(&as, "as", "", "name to give a cell created from an image reference")
	cmd.Flags().BoolVar(&detach, "detach", false, "start the cell without attaching a shell")

	return cmd
}
