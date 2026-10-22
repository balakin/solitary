package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/balakin/solitary/internal/cell"
	"github.com/balakin/solitary/internal/host"
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
	row := func(label, value string) {
		if value != "" {
			fmt.Fprintf(out, "  %-9s %s\n", label, value)
		}
	}

	fmt.Fprintln(out, "host")
	row("solitary", r.Host.Solitary)
	row("lima", r.Host.Lima)
	if r.Host.QEMU != "" {
		row("qemu", fmt.Sprintf("%s (%s)", r.Host.QEMU, r.Host.QEMUPath))
	}
	row("kernel", r.Host.Kernel)
	row("nested", r.Host.Nested)
	for _, unknown := range r.Host.Unknown {
		row("unknown", unknown)
	}

	if r.Cell == "" {
		return
	}
	fmt.Fprintln(out)
	m := r.Machine
	if m == nil {
		fmt.Fprintf(out, "machine    none: 'solitary up %s' creates it\n", r.Cell)
		return
	}
	fmt.Fprintf(out, "machine    %s\n", m.Status)
	created := m.CreatedFrom
	if created == "" {
		created = "not recorded yet"
	}
	if m.CreatedFrom != m.NewFrom {
		created += " (new machines: " + m.NewFrom + ")"
	}
	row("created", created)

	if g := m.Guest; g != nil {
		row("system", strings.TrimSpace(g.Release+" · kernel "+g.Kernel))
		row("updates", updates(g))
		row("lists", lists(g, now))
		if g.DiskSize > 0 {
			row("disk", fmt.Sprintf("%s of %s", host.FormatSize(uint64(g.DiskUsed)), host.FormatSize(uint64(g.DiskSize))))
		}
		for i, line := range packageLines(g.Packages, 66) {
			if i == 0 {
				row("packages", line)
			} else {
				row("", line)
			}
		}
		if !g.Live {
			row("", fmt.Sprintf("as last seen, %s", age(now.Sub(g.AsOf))))
		}
	} else {
		row("system", "not seen yet: start the cell and inspect it again")
	}
	row("asking", m.Unreachable)

	if c := r.Container; c != nil {
		fmt.Fprintln(out)
		state := "stopped"
		if c.Running {
			state = "running"
		}
		fmt.Fprintf(out, "container  %s\n", state)
		row("image", c.Image)
		row("system", c.Release)
		row("user", c.User)
		row("devices", c.Devices)
	}

	for _, list := range []struct {
		title    string
		packages []cell.Package
	}{{"machine packages", m.Packages}, {"container packages", containerPackages(r.Container)}} {
		if len(list.packages) == 0 {
			continue
		}
		fmt.Fprintf(out, "\n%s\n", list.title)
		for _, p := range list.packages {
			fmt.Fprintf(out, "  %s %s\n", p.Name, p.Version)
		}
	}
}

func containerPackages(c *cell.Container) []cell.Package {
	if c == nil {
		return nil
	}
	return c.Packages
}

// updates says how far behind the machine's packages are, and whether a
// kernel or library it already installed is waiting on a reboot.
func updates(g *cell.Guest) string {
	var parts []string
	if g.Updates != nil {
		part := fmt.Sprintf("%d pending", *g.Updates)
		if g.Security != nil {
			part += fmt.Sprintf(", %d security", *g.Security)
		}
		parts = append(parts, part)
	}
	if g.RebootRequired {
		parts = append(parts, "reboot required")
	}

	return strings.Join(parts, " · ")
}

// lists says when the package lists the counts are measured against were last
// refreshed: a cell whose firewall keeps the archive out never refreshes them,
// and then "0 pending" means nothing.
func lists(g *cell.Guest, now time.Time) string {
	var parts []string
	if !g.ListsRefreshed.IsZero() {
		parts = append(parts, "refreshed "+age(now.Sub(g.ListsRefreshed)))
	}
	if g.Unattended != "" {
		parts = append(parts, "unattended-upgrades "+g.Unattended)
	}

	return strings.Join(parts, " · ")
}

// packageLines lists packages by name, as many to a line as fit in width: a
// version is long enough that one line of them runs off any terminal.
func packageLines(packages map[string]string, width int) []string {
	names := make([]string, 0, len(packages))
	for name := range packages {
		names = append(names, name)
	}
	sort.Strings(names)

	var lines []string
	line := ""
	for _, name := range names {
		part := name + " " + packages[name]
		switch {
		case line == "":
			line = part
		case len(line)+len(" · ")+len(part) > width:
			lines = append(lines, line)
			line = part
		default:
			line += " · " + part
		}
	}
	if line != "" {
		lines = append(lines, line)
	}

	return lines
}

// age says how long ago something was, as a person would.
func age(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return plural(int(d.Minutes()), "minute") + " ago"
	case d < 48*time.Hour:
		return plural(int(d.Hours()), "hour") + " ago"
	default:
		return plural(int(d.Hours()/24), "day") + " ago"
	}
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}
