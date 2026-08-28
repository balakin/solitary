package cli

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/balakin/solitary/internal/clone"
	"github.com/balakin/solitary/internal/config"
)

func newCloneCmd() *cobra.Command {
	var (
		as    string
		force bool
		yes   bool
		list  bool
	)

	cmd := &cobra.Command{
		Use:   "clone <source>",
		Short: "Install a cell definition from a repository",
		Long: "Takes one cell out of a git repository and installs it as a cell of your\n" +
			"own. A repository can be a single cell — a cell.yaml at its root — or a\n" +
			"catalogue of them, one directory each; name the one you want and the rest\n" +
			"stay where they are.\n\n" +
			"    solitary clone owner/repo              a repository that is one cell\n" +
			"    solitary clone owner/repo/claude       one cell out of a catalogue\n" +
			"    solitary clone owner/repo              a catalogue, unnamed: lists it\n" +
			"    solitary clone https://host/repo.git#claude\n" +
			"    solitary clone ../my-cells#claude\n\n" +
			"What the definition asks for is shown before anything is written, because\n" +
			"a cell names the secrets it wants and running it hands them to someone\n" +
			"else's image. Nothing starts either way: 'up' does that, afterwards.\n\n" +
			"A .env or a vpn.conf found in the repository is refused rather than\n" +
			"copied — credentials belong to whoever runs a cell, never to the cell.\n" +
			"The copy has no link back to where it came from: cloning the same source\n" +
			"again with --force replaces the definition and keeps your .env beside it.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source, err := clone.Parse(args[0])
			if err != nil {
				return err
			}

			out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()

			// The name a cell would take is known from the source
			// alone, so a collision is refused here rather than
			// after a repository has been fetched and a question
			// asked that could not have been honoured.
			name := source.DefaultName()
			if as != "" {
				name = as
			}
			if !list {
				if err := config.ValidateName(name); err != nil {
					return fmt.Errorf("%w\nname it yourself with --as", err)
				}
				if !force {
					if err := clone.Vacant(name); err != nil {
						return err
					}
				}
			}

			staged, err := clone.Stage(source, errOut)
			if err != nil {
				var catalogue *clone.Catalogue
				if errors.As(err, &catalogue) {
					return showCatalogue(out, catalogue)
				}
				return err
			}
			defer staged.Discard()

			for _, refused := range staged.Refused {
				fmt.Fprintf(errOut, "Left behind — %s\n", refused)
			}

			if list {
				describeStaged(out, staged, name)
				return nil
			}

			describeStaged(out, staged, name)

			if !yes {
				ok, err := confirm(cmd, "Install it? [y/N] ")
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(errOut, "Cancelled.")
					return nil
				}
			}

			written, err := clone.Install(staged, name, force)
			if err != nil {
				return err
			}

			dir, err := config.CellDir(name)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "Wrote %s into %s\n", strings.Join(written, ", "), dir)
			fmt.Fprintf(out, "Next: solitary up %s\n", name)

			return nil
		},
	}

	cmd.Flags().StringVar(&as, "as", "", "install under this name instead of the one in the repository")
	cmd.Flags().BoolVar(&force, "force", false, "replace an existing definition, keeping its .env")
	cmd.Flags().BoolVar(&yes, "yes", false, "install without asking")
	cmd.Flags().BoolVar(&list, "list", false, "show what the definition asks for without installing it")

	return cmd
}

// showCatalogue reports a repository that holds several cells with none named.
// It is not an error worth a non-zero exit: the answer is on the screen, one
// command away.
func showCatalogue(out io.Writer, catalogue *clone.Catalogue) error {
	if len(catalogue.Cells) == 0 {
		return errors.New(catalogue.Error())
	}

	fmt.Fprintf(out, "%s holds %d cells:\n", catalogue.Source.Display, len(catalogue.Cells))
	for _, cell := range catalogue.Cells {
		fmt.Fprintf(out, "  %s\n", cell)
	}
	fmt.Fprintf(out, "Take one with: solitary clone %s#%s\n", catalogue.Source.Display, catalogue.Cells[0])

	return nil
}

// describeStaged shows what a definition asks for, in the shape ls and the
// dashboard show a cell that already exists.
func describeStaged(out io.Writer, staged *clone.Staged, name string) {
	c := staged.Cell

	image := c.Image
	if c.Build != "" {
		image = "build:" + c.Build
	}

	fmt.Fprintf(out, "\nCell %q from %s\n", name, staged.Source.Display)
	fmt.Fprintf(out, "  image    %s\n", image)
	fmt.Fprintf(out, "  machine  %d cpus · %s · %s\n", c.VM.CPUs, c.VM.Memory, c.VM.Disk)

	if len(c.Secrets) > 0 {
		// Which of them the cell can do without is part of what is being
		// consented to here, so an optional one says so.
		declared := make([]string, 0, len(c.Secrets))
		for _, secret := range c.Secrets {
			if secret.Required {
				declared = append(declared, secret.Name)
			} else {
				declared = append(declared, secret.Name+" (optional)")
			}
		}
		fmt.Fprintf(out, "  secrets  %s\n", strings.Join(declared, ", "))
		fmt.Fprintf(out, "           set them with: solitary secrets %s\n", name)
	}

	if len(c.Ports) > 0 {
		ports := make([]string, 0, len(c.Ports))
		for _, port := range c.Ports {
			ports = append(ports, strconv.Itoa(port))
		}
		fmt.Fprintf(out, "  ports    %s\n", strings.Join(ports, ", "))
	} else {
		fmt.Fprintln(out, "  ports    all reach host localhost")
	}

	if c.Network.Restricted() {
		fmt.Fprintf(out, "  network  %d allowed\n", len(c.Network.Allow))
		for _, entry := range c.Network.Allow {
			fmt.Fprintf(out, "           %s\n", entry)
		}
	} else {
		fmt.Fprintln(out, "  network  unrestricted")
	}

	if c.Network.VPN != "" {
		fmt.Fprintf(out, "  vpn      %s — supply your own, it is not in the repository\n", c.Network.VPN)
	}

	fmt.Fprintln(out)
}
