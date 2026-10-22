package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/balakin/solitary/internal/cell"
)

func newInspectCmd() *cobra.Command {
	var asJSON, packages bool

	cmd := &cobra.Command{
		Use:   "inspect [name]",
		Short: "Report what a cell runs on outside its container",
		Long: "Reports a cell one layer at a time: the host it runs on (solitary, Lima,\n" +
			"QEMU, the kernel), the machine (its system and kernel, pending updates,\n" +
			"whether it waits on a reboot, its disk and the packages its isolation is\n" +
			"made of) and the container inside it. With no name, the host alone —\n" +
			"what a bug report needs first.\n\n" +
			"It changes nothing and starts nothing. A stopped machine is reported as\n" +
			"it was last seen, with the date. The container's environment, where a\n" +
			"cell's secrets are, is never read.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var report cell.Report
			if len(args) == 0 {
				report.Host = cell.InspectHost(version)
			} else {
				var err error
				if report, err = cell.Inspect(args[0], version, packages); err != nil {
					return err
				}
			}

			out := cmd.OutOrStdout()
			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			writeReport(out, report, time.Now())

			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the report as JSON")
	cmd.Flags().BoolVar(&packages, "packages", false, "list every package in a running machine and its container")

	return cmd
}

// writeReport prints a report for someone to read. now is when it is read,
// which is what "as of" is measured against.
func writeReport(out io.Writer, r cell.Report, now time.Time) {
	for i, row := range r.Rows(now) {
		switch {
		case row.Heading != "" && i > 0:
			fmt.Fprintln(out)
			fallthrough
		case row.Heading != "":
			if row.Value == "" {
				fmt.Fprintln(out, row.Heading)
			} else {
				fmt.Fprintf(out, "%-10s %s\n", row.Heading, row.Value)
			}
		default:
			fmt.Fprintf(out, "  %-9s %s\n", row.Label, row.Value)
		}
	}
}
