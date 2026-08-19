package cli

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/balakin/solitary/internal/doctor"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check that this host can run cells",
		Long: "Checks the host a cell needs before a cell needs it: limactl and its\n" +
			"version, whether this machine can run a hypervisor at all, whether the\n" +
			"cells defined here fit in memory and on disk, and whether the config\n" +
			"tree reads and records.\n\n" +
			"It changes nothing. Every check that is not ok names what to do about\n" +
			"it, and doctor exits non-zero when one failed outright.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			checks := doctor.Host()

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for _, c := range checks {
				fmt.Fprintf(w, "%s\t%s\t%s\n", c.Name, c.Status, c.Detail)
				for _, line := range fixLines(c.Fix) {
					fmt.Fprintf(w, "\t\t%s\n", line)
				}
			}
			if err := w.Flush(); err != nil {
				return err
			}

			if doctor.Failed(checks) {
				return errors.New("this host cannot run cells until the failed checks above are fixed")
			}

			return nil
		},
	}
}

// fixLines splits a fix into the lines the table indents under its check.
func fixLines(fix string) []string {
	if fix == "" {
		return nil
	}
	return strings.Split(fix, "\n")
}
