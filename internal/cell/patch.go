package cell

import (
	_ "embed"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"

	"github.com/balakin/solitary/internal/config"
	"github.com/balakin/solitary/internal/lima"
)

//go:embed patch.sh
var patchScript string

// Patch brings a cell's machine up to date: every update its release has,
// security fixes included, installed now rather than whenever
// unattended-upgrades gets to them. The container is not touched — its system
// is the image's, and up --rebuild is what updates that.
//
// A stopped cell is patched with its machine alone, and stopped again: its
// container never starts, and the next start is the reboot whatever was
// installed may need. A running cell keeps running unless the patch needs a
// reboot and reboot says yes, since a reboot ends everything in the container.
func Patch(name string, reboot func() (bool, error), progress io.Writer) (err error) {
	unlock, err := lockCell(name)
	if err != nil {
		return err
	}
	defer unlock()

	c, err := config.LoadCell(name)
	if err != nil {
		return err
	}

	instance := config.Instance(name)
	inst, err := lima.Lookup(instance)
	if err != nil {
		return err
	}
	if inst == nil {
		return fmt.Errorf("cell %q has no machine to patch; 'solitary up %s' creates one already up to date", name, name)
	}
	running := inst.Status == lima.StatusRunning

	if !running {
		fmt.Fprintf(progress, "Cell %q is stopped; starting its machine alone to patch it.\n", name)
	}
	if _, err := ensureMachine(name, c, progress); err != nil {
		return err
	}
	if !running {
		// Put back as it was found, failed patch or not: a cell someone left
		// stopped is not one to leave running behind their back.
		defer func() {
			fmt.Fprintf(progress, "Stopping cell %q again...\n", name)
			if stopErr := lima.Stop(instance); stopErr != nil && err == nil {
				err = stopErr
			}
		}()
	}

	if err := reachArchive(name, instance, c.Network); err != nil {
		return err
	}

	fmt.Fprintf(progress, "Patching the machine behind %q...\n", name)
	if err := upgrade(instance, progress); err != nil {
		return err
	}

	guest, err := askGuest(instance)
	if err != nil {
		return err
	}
	switch {
	case !guest.RebootRequired:
	case !running:
		// It is about to be stopped, and its next start is the reboot.
		guest.RebootRequired = false
		fmt.Fprintln(progress, "What needs a reboot takes effect the next time the cell starts.")
	default:
		ok, err := reboot()
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintf(progress, "A reboot finishes it, and stops everything in the container: stop and start\n"+
				"the cell, or run 'solitary patch %s --reboot'.\n", name)
			break
		}
		fmt.Fprintf(progress, "Restarting cell %q to finish; its container starts again with it.\n", name)
		if err := lima.Stop(instance); err != nil {
			return err
		}
		if err := up(name, false, progress); err != nil {
			return err
		}
		if guest, err = askGuest(instance); err != nil {
			return err
		}
	}
	if err := writeGuest(name, guest); err != nil {
		return err
	}

	summary := strings.TrimSpace(guest.Release + " · kernel " + guest.Kernel)
	if pending := guest.Pending(); pending != "" {
		summary += " · " + pending
	}
	fmt.Fprintf(progress, "Cell %q is patched: %s.\n", name, summary)

	return nil
}

// upgrade runs the patch script in a running machine, showing what apt says as
// it says it.
func upgrade(instance string, progress io.Writer) error {
	cmd, err := lima.Command(instance, "sudo", "sh", "-c", patchScript)
	if err != nil {
		return err
	}
	cmd.Stdout, cmd.Stderr = progress, progress
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("patching the machine: %w", err)
	}

	return nil
}

// reachArchive makes sure the machine's apt can reach every host it fetches
// from before it is asked to. Ubuntu's archive is always open to it; a source
// a cell added — a PPA its provision script set up, say — is reachable only if
// network.allow says so, and is named here rather than left to fail halfway
// through an upgrade. The allow list is never widened to make it work: what a
// cell reaches is the cell's own statement.
func reachArchive(name, instance string, network config.Network) error {
	if !network.Restricted() {
		return nil
	}

	// A machine that booted from a definition older than the archive's
	// opening has a firewall that refuses apt like everything else, whatever
	// the definition says now.
	if _, err := lima.Exec(instance, "sudo", "nft", "list", "set", "inet", "solitary", "archive4"); err != nil {
		return fmt.Errorf("cell %q started before solitary opened Ubuntu's archive to its machine;\n"+
			"stop and start it ('solitary down %s', then 'solitary up %s') and patch again", name, name, name)
	}

	out, err := lima.Exec(instance, "sh", "-c",
		"cat /etc/apt/sources.list /etc/apt/sources.list.d/*.list /etc/apt/sources.list.d/*.sources 2>/dev/null; true")
	if err != nil {
		return fmt.Errorf("reading the machine's apt sources: %w", err)
	}
	var missing []string
	for _, host := range sourceHosts(string(out)) {
		if !config.ArchiveCovers(host) && !network.Covers(host) {
			missing = append(missing, host)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("cell %q fetches packages from %s, which network.allow does not list;\n"+
			"add them to its cell.yaml — the machine picks the change up when it next starts — and patch again",
			name, strings.Join(missing, ", "))
	}

	return nil
}

// sourceHosts lists the hosts apt sources name, in either of apt's formats:
// one-line "deb [options] uri suite" entries and deb822 "URIs:" fields. Only
// the network ones count; a file: or cdrom: source fetches from nowhere.
func sourceHosts(sources string) []string {
	seen := map[string]bool{}
	for _, line := range strings.Split(sources, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		var uris []string
		switch fields[0] {
		case "URIs:":
			uris = fields[1:]
		case "deb", "deb-src":
			rest := strings.Join(fields[1:], " ")
			if strings.HasPrefix(rest, "[") {
				if _, after, ok := strings.Cut(rest, "]"); ok {
					rest = after
				}
			}
			if f := strings.Fields(rest); len(f) > 0 {
				uris = f[:1]
			}
		}

		for _, uri := range uris {
			u, err := url.Parse(uri)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
				continue
			}
			seen[strings.ToLower(u.Hostname())] = true
		}
	}

	hosts := make([]string, 0, len(seen))
	for host := range seen {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)

	return hosts
}
