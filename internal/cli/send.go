package cli

import (
	"github.com/spf13/cobra"

	"github.com/dm-balakin/solitary/internal/cell"
)

func newSendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send <name> <file>...",
		Short: "Put files into a cell's inbox",
		Long: "Copies files from this host into a cell, where they appear in its inbox\n" +
			"at $HOME/inbox. This is how a cell is given something to work on — a\n" +
			"spec, a dump, a dataset — without mounting anything from the host: the\n" +
			"files are copied in, and the cell has no way back out to where they came\n" +
			"from.\n\n" +
			"Only regular files, one level: pack a directory into an archive and send\n" +
			"that instead.",
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cell.Send(args[0], args[1:], cmd.OutOrStdout())
		},
	}
	cmd.Flags().SetInterspersed(false)

	return cmd
}
