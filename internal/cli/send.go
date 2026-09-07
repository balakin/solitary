package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/balakin/solitary/internal/cell"
)

func newSendCmd() *cobra.Command {
	var fromClipboard bool

	cmd := &cobra.Command{
		Use:   "send <name> <file>...",
		Short: "Put files into a cell's inbox",
		Long: "Copies files from this host into a cell, where they appear in its inbox\n" +
			"at $HOME/inbox. This is how a cell is given something to work on — a\n" +
			"spec, a dump, a dataset — without mounting anything from the host: the\n" +
			"files are copied in, and the cell has no way back out to where they came\n" +
			"from.\n\n" +
			"Only regular files, one level: pack a directory into an archive and send\n" +
			"that instead.\n\n" +
			"--clipboard sends the image on this host's clipboard instead of a file,\n" +
			"which is how a screenshot reaches a cell: a terminal carries characters,\n" +
			"so an image cannot be pasted into a shell in one. It arrives in the inbox\n" +
			"as clipboard-<timestamp>.png.",
		Args: func(cmd *cobra.Command, args []string) error {
			if fromClipboard {
				if len(args) != 1 {
					return errors.New("--clipboard sends the clipboard on its own; name the cell and no files")
				}
				return nil
			}

			return cobra.MinimumNArgs(2)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if fromClipboard {
				return cell.SendClipboard(args[0], cmd.OutOrStdout())
			}

			return cell.Send(args[0], args[1:], cmd.OutOrStdout())
		},
	}
	// Interspersed, unlike exec: the arguments here are host paths rather
	// than a command with flags of its own, and a path whose file name
	// could be read as a flag is one Send refuses anyway. Being able to
	// write --clipboard after the cell name, where a flag is expected, is
	// worth more than parsing a name that would not be accepted.
	cmd.Flags().BoolVar(&fromClipboard, "clipboard", false, "Send the image on this host's clipboard")

	return cmd
}
