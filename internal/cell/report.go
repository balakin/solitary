package cell

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/balakin/solitary/internal/host"
)

// Row is one line of a report as a person reads it. inspect prints the rows
// and the dashboard styles them, so the two say the same things in the same
// order and cannot drift apart.
type Row struct {
	// Heading starts a section — host, machine, container — and Value, when
	// there is one, is that section's state.
	Heading string
	Label   string
	Value   string
	// Warn marks a value that names something the machine could fix and
	// has not.
	Warn bool
}

// Rows lays a report out for reading at now, which is what a reading's age is
// measured against.
func (r Report) Rows(now time.Time) []Row {
	var rows []Row
	add := func(label, value string) {
		if value != "" {
			rows = append(rows, Row{Label: label, Value: value})
		}
	}

	rows = append(rows, Row{Heading: "host"})
	add("solitary", r.Host.Solitary)
	add("lima", r.Host.Lima)
	if r.Host.QEMU != "" {
		add("qemu", fmt.Sprintf("%s (%s)", r.Host.QEMU, r.Host.QEMUPath))
	}
	add("kernel", r.Host.Kernel)
	add("nested", r.Host.Nested)
	for _, unknown := range r.Host.Unknown {
		add("unknown", unknown)
	}

	if r.Cell == "" {
		return rows
	}
	m := r.Machine
	if m == nil {
		return append(rows, Row{Heading: "machine", Value: fmt.Sprintf("none: 'solitary up %s' creates it", r.Cell)})
	}
	rows = append(rows, Row{Heading: "machine", Value: m.Status})
	created := m.CreatedFrom
	if created == "" {
		created = "not recorded yet"
	}
	if m.CreatedFrom != m.NewFrom {
		created += " (new machines: " + m.NewFrom + ")"
	}
	add("created", created)

	if g := m.Guest; g != nil {
		add("system", strings.TrimSpace(g.Release+" · kernel "+g.Kernel))
		// Under the system line rather than labelled on its own: these are
		// the system's packages, and a row of its own reads as anyone's.
		if pending := g.Pending(); pending != "" {
			rows = append(rows, Row{Value: pending, Warn: g.Behind()})
		}
		add("lists", lists(g, now))
		if g.DiskSize > 0 {
			add("disk", fmt.Sprintf("%s of %s", host.FormatSize(uint64(g.DiskUsed)), host.FormatSize(uint64(g.DiskSize))))
		}
		for i, line := range packageLines(g.Packages, 66) {
			if i == 0 {
				add("packages", line)
			} else {
				add("", line)
			}
		}
		if !g.Live {
			add("", "as last seen, "+age(now.Sub(g.AsOf)))
		}
	} else {
		add("system", "not seen yet: start the cell and inspect it again")
	}
	add("asking", m.Unreachable)

	if c := r.Container; c != nil {
		state := "stopped"
		if c.Running {
			state = "running"
		}
		rows = append(rows, Row{Heading: "container", Value: state})
		add("image", c.Image)
		add("system", c.Release)
		add("user", c.User)
		add("devices", c.Devices)
	}

	var containerPackages []Package
	if r.Container != nil {
		containerPackages = r.Container.Packages
	}
	for _, list := range []struct {
		title    string
		packages []Package
	}{{"machine packages", m.Packages}, {"container packages", containerPackages}} {
		if len(list.packages) == 0 {
			continue
		}
		rows = append(rows, Row{Heading: list.title})
		for _, p := range list.packages {
			add("", p.Name+" "+p.Version)
		}
	}

	return rows
}

// lists says when the package lists the counts are measured against were last
// refreshed: a cell whose firewall keeps the archive out never refreshes them,
// and then "0 pending" means nothing.
func lists(g *Guest, now time.Time) string {
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
