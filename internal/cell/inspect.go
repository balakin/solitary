package cell

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/balakin/solitary/internal/config"
	"github.com/balakin/solitary/internal/lima"
	"github.com/balakin/solitary/internal/podman"
)

// inspectScript is what inspect runs in a cell's machine. Embedded, like the
// artifact tool, so that it asks the same questions of every machine.
//
//go:embed inspect.sh
var inspectScript string

// Report is what inspect says about a cell, one layer at a time: the host it
// runs on, the machine, and the container inside it. A layer the cell does not
// have yet is absent rather than empty.
//
// Nothing in it holds the container's environment, which is where a cell's
// secrets are: a report is made to be pasted into an issue.
type Report struct {
	Host      Host       `json:"host"`
	Cell      string     `json:"cell,omitempty"`
	Machine   *Machine   `json:"machine,omitempty"`
	Container *Container `json:"container,omitempty"`
}

// Host is what every cell on this host runs on.
type Host struct {
	Solitary string `json:"solitary"`
	Lima     string `json:"lima,omitempty"`
	QEMU     string `json:"qemu,omitempty"`
	QEMUPath string `json:"qemuPath,omitempty"`
	Kernel   string `json:"kernel,omitempty"`
	// Nested is the KVM module's nested setting, Y or N: whether a guest
	// can run guests of its own.
	Nested string `json:"nestedVirtualization,omitempty"`
	// Unknown says which of the above could not be read, and why.
	Unknown []string `json:"unknown,omitempty"`
}

// Machine is the cell's machine as solitary and Lima know it, and what was
// last seen inside it.
type Machine struct {
	Status string `json:"status"`
	// CreatedFrom is the image template the machine was created from, as
	// the applied record has it; NewFrom is what a new one would be.
	CreatedFrom string `json:"createdFrom,omitempty"`
	NewFrom     string `json:"newMachinesFrom"`
	Guest       *Guest `json:"guest,omitempty"`
	// Unreachable says why a running machine could not be asked, when it
	// could not; Guest is then what was last seen.
	Unreachable string    `json:"unreachable,omitempty"`
	Packages    []Package `json:"packages,omitempty"`
}

// Guest is what a machine said about itself, and when.
type Guest struct {
	AsOf           time.Time         `json:"asOf"`
	Live           bool              `json:"live"`
	Release        string            `json:"release,omitempty"`
	Kernel         string            `json:"kernel,omitempty"`
	RebootRequired bool              `json:"rebootRequired"`
	Updates        *int              `json:"updates,omitempty"`
	Security       *int              `json:"securityUpdates,omitempty"`
	ListsRefreshed time.Time         `json:"listsRefreshed,omitzero"`
	Unattended     string            `json:"unattendedUpgrades,omitempty"`
	DiskUsed       int64             `json:"diskUsed,omitempty"`
	DiskSize       int64             `json:"diskSize,omitempty"`
	Packages       map[string]string `json:"packages,omitempty"`
}

// Pending says how far behind the machine's packages are, and whether a kernel
// or library it already installed is waiting on a reboot; empty when the
// machine said neither.
//
// It is shown under the system line, as part of it, so it names its own unit:
// beside a container image, a bare "65 pending" reads as being about the image.
func (g *Guest) Pending() string {
	var parts []string
	if g.Updates != nil {
		part := fmt.Sprintf("%d updates pending", *g.Updates)
		if *g.Updates == 1 {
			part = "1 update pending"
		}
		if g.Security != nil && *g.Security > 0 {
			part += fmt.Sprintf(", %d of them security", *g.Security)
		}
		parts = append(parts, part)
	}
	if g.RebootRequired {
		parts = append(parts, "reboot required")
	}

	return strings.Join(parts, " · ")
}

// Behind reports whether the machine is missing security fixes it could have:
// pending security updates, or installed ones a reboot has not applied yet.
func (g *Guest) Behind() bool {
	return g.RebootRequired || (g.Security != nil && *g.Security > 0)
}

// AskGuest asks a cell's running machine what inspect asks it, and keeps the
// answer for when the machine is stopped.
func AskGuest(name string) (Guest, error) {
	if err := config.ValidateName(name); err != nil {
		return Guest{}, err
	}
	guest, err := askGuest(config.Instance(name))
	if err != nil {
		return Guest{}, err
	}
	if err := writeGuest(name, guest); err != nil {
		return Guest{}, err
	}

	return guest, nil
}

// Container is the cell's container, when it has one.
type Container struct {
	Running  bool      `json:"running"`
	Image    string    `json:"image,omitempty"`
	User     string    `json:"user,omitempty"`
	Devices  string    `json:"devices,omitempty"`
	Release  string    `json:"release,omitempty"`
	Packages []Package `json:"packages,omitempty"`
}

// Package is one installed package and its version.
type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InspectHost reports the host alone: what a bug report needs first, and all
// there is to say without a cell. version is solitary's own.
func InspectHost(version string) Host {
	host := Host{Solitary: version}
	unknown := func(what string, err error) {
		host.Unknown = append(host.Unknown, fmt.Sprintf("%s: %v", what, err))
	}

	if v, err := lima.Version(); err != nil {
		unknown("lima", err)
	} else {
		host.Lima = v
	}

	if out, err := exec.Command("uname", "-sr").Output(); err != nil {
		unknown("kernel", err)
	} else {
		host.Kernel = strings.TrimSpace(string(out))
	}

	// On macOS Lima runs machines with Apple's own framework, so there is
	// no qemu and no KVM to report.
	if runtime.GOOS == "linux" {
		if bin, v, err := lima.QEMU(); err != nil {
			unknown("qemu", err)
		} else {
			host.QEMU, host.QEMUPath = v, bin
		}
		for _, module := range []string{"kvm_intel", "kvm_amd"} {
			if data, err := os.ReadFile(filepath.Join("/sys/module", module, "parameters", "nested")); err == nil {
				host.Nested = strings.TrimSpace(string(data))
				break
			}
		}
	}

	return host
}

// Inspect reports a cell on every layer it has. A machine that is not running
// is not started for it: what was last seen inside is reported, with the date
// it was seen. packages adds every installed package, machine and container,
// which only a running cell can answer.
func Inspect(name, version string, packages bool) (Report, error) {
	if err := config.ValidateName(name); err != nil {
		return Report{}, err
	}
	report := Report{Host: InspectHost(version), Cell: name}

	instance := config.Instance(name)
	inst, err := lima.Lookup(instance)
	if err != nil {
		return Report{}, err
	}
	if inst == nil {
		// A cell not created yet is still a cell; a name that is neither
		// a definition nor a machine is a mistake to say so about.
		switch defined, err := config.HasCell(name); {
		case err != nil:
			return Report{}, err
		case !defined:
			return Report{}, fmt.Errorf("no cell named %q and no machine for one", name)
		}
		return report, nil
	}

	record, err := config.ReadApplied(name)
	if err != nil {
		return Report{}, err
	}
	machine := &Machine{Status: strings.ToLower(inst.Status), CreatedFrom: record.Base, NewFrom: config.Base}
	report.Machine = machine

	cached, err := readGuest(name)
	if err != nil {
		return Report{}, err
	}
	machine.Guest = cached

	if inst.Status != lima.StatusRunning {
		return report, nil
	}

	// A machine that does not answer is still reported: what was last seen
	// stands, dated, with the reason it is not current beside it.
	if guest, err := askGuest(instance); err != nil {
		machine.Unreachable = err.Error()
	} else if err := report.addLive(name, instance, guest, packages); err != nil {
		return Report{}, err
	}

	return report, nil
}

// addLive adds what a running machine answered, and the container inside it,
// keeping the answer for when the machine is stopped.
func (r *Report) addLive(name, instance string, guest Guest, packages bool) error {
	r.Machine.Guest = &guest
	if err := writeGuest(name, guest); err != nil {
		return err
	}

	if packages {
		list, err := installed(instance)
		if err != nil {
			return err
		}
		r.Machine.Packages = list
	}

	container, err := inspectContainer(instance, packages)
	if err != nil {
		return err
	}
	r.Container = container

	return nil
}

// askGuest runs the inspect script in a running machine.
func askGuest(instance string) (Guest, error) {
	out, err := lima.Exec(instance, "sh", "-c", inspectScript)
	if err != nil {
		return Guest{}, fmt.Errorf("asking the machine: %w", err)
	}

	guest := parseGuest(string(out))
	guest.AsOf, guest.Live = time.Now().UTC(), true

	return guest, nil
}

// parseGuest reads what the inspect script printed. A line it does not know is
// skipped, so a script that learns a new question does not break an older
// reading of it.
func parseGuest(out string) Guest {
	var guest Guest
	for _, line := range strings.Split(out, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)

		switch key {
		case "release":
			guest.Release = value
		case "kernel":
			guest.Kernel = value
		case "reboot":
			guest.RebootRequired = value == "yes"
		case "updates":
			all, security, ok := strings.Cut(value, ";")
			if !ok {
				continue
			}
			if n, err := strconv.Atoi(all); err == nil {
				guest.Updates = &n
			}
			if n, err := strconv.Atoi(security); err == nil {
				guest.Security = &n
			}
		case "lists":
			if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
				guest.ListsRefreshed = time.Unix(seconds, 0).UTC()
			}
		case "unattended":
			guest.Unattended = value
		case "disk":
			if used, size, ok := strings.Cut(strings.Join(strings.Fields(value), " "), " "); ok {
				guest.DiskUsed, _ = strconv.ParseInt(used, 10, 64)
				guest.DiskSize, _ = strconv.ParseInt(size, 10, 64)
			}
		default:
			if name, ok := strings.CutPrefix(key, "package."); ok && value != "" {
				if guest.Packages == nil {
					guest.Packages = make(map[string]string)
				}
				guest.Packages[name] = value
			}
		}
	}

	return guest
}

// installed lists every package the machine has.
func installed(instance string, prefix ...string) ([]Package, error) {
	args := append(append([]string{}, prefix...), "dpkg-query", "-W", "-f=${Package}\t${Version}\n")
	out, err := lima.Exec(instance, args...)
	if err != nil {
		return nil, fmt.Errorf("listing packages: %w", err)
	}

	var list []Package
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if name, version, ok := strings.Cut(line, "\t"); ok {
			list = append(list, Package{Name: name, Version: version})
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })

	return list, nil
}

// inspectContainer reports the cell's container from its labels, and asks a
// running one which release its image is. It never reads the container's
// environment.
func inspectContainer(instance string, packages bool) (*Container, error) {
	state, err := podman.Inspect(instance)
	if err != nil {
		return nil, err
	}
	if !state.Exists {
		return nil, nil
	}

	container := &Container{Running: state.Running, Image: state.Image, User: state.User, Devices: state.Devices}
	if !state.Running {
		return container, nil
	}

	// An image need not have /etc/os-release, or dpkg; either missing is
	// not something to report as a failure.
	inContainer := []string{"podman", "exec", podman.Container}
	if out, err := lima.Exec(instance, append(inContainer, "sh", "-c", `. /etc/os-release && echo "$PRETTY_NAME"`)...); err == nil {
		container.Release = strings.TrimSpace(string(out))
	}
	if packages {
		container.Packages, _ = installed(instance, inContainer...)
	}

	return container, nil
}

// readGuest returns what was last seen inside a cell's machine, or nil when
// nothing has been.
func readGuest(name string) (*Guest, error) {
	path, err := config.GuestFile(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	// A file this cannot read is one to replace at the next look, not a
	// reason to refuse to report everything else.
	guest, ok := decodeGuest(data)
	if !ok {
		return nil, nil
	}
	guest.Live = false

	return &guest, nil
}

// decodeGuest reads a kept answer, reporting whether the file was one.
func decodeGuest(data []byte) (Guest, bool) {
	var guest Guest
	decoded := json.Unmarshal(data, &guest) == nil

	return guest, decoded
}

// writeGuest keeps what was seen inside a cell's machine for when it is not
// running.
func writeGuest(name string, guest Guest) error {
	path, err := config.GuestFile(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(guest, "", "  ")
	if err != nil {
		return fmt.Errorf("recording what the machine said: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}
