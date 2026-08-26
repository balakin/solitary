// Package doctor checks the things a cell needs from the host before a cell
// needs them.
//
// Every failure it reports is one solitary already has a story for, told either
// by a command that has gone too far to be helpful — limactl missing halfway
// through a create — or by nothing at all, in the troubleshooting docs. Asking
// up front is cheaper than either.
package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/balakin/solitary/internal/config"
	"github.com/balakin/solitary/internal/host"
	"github.com/balakin/solitary/internal/lima"
	"github.com/balakin/solitary/internal/update"
)

// Status is what a check concluded.
type Status string

const (
	// OK means the check found nothing to say.
	OK Status = "ok"

	// Warn means something will bite under conditions that do not hold yet.
	Warn Status = "warn"

	// Fail means a cell cannot work on this host until it is fixed.
	Fail Status = "fail"
)

// Check is one question asked of the host and its answer.
type Check struct {
	// Name is the short label the check is listed under.
	Name string

	Status Status

	// Detail states what was found, as one line.
	Detail string

	// Fix says what to do about it, and is empty for a check that passed or
	// for one nothing can be done about.
	Fix string
}

// Host runs every host-level check, in the order a cell needs them: without
// limactl and a hypervisor nothing else matters.
//
// It never returns an error. A check that cannot be answered says so as its
// own result — an inspection that stops at the first problem is the thing this
// command exists not to be.
func Host() []Check {
	return []Check{
		checkLimactl(),
		checkHypervisor(),
		checkMemory(),
		checkDisk(),
		checkMachines(),
		checkConfig(),
		checkProxy(),
	}
}

// Failed reports whether any check failed, which is what makes doctor's own
// exit status worth testing in a script.
func Failed(checks []Check) bool {
	for _, c := range checks {
		if c.Status == Fail {
			return true
		}
	}
	return false
}

func checkLimactl() Check {
	bin, err := lima.Binary()
	if err != nil {
		return Check{
			Name:   "limactl",
			Status: Fail,
			Detail: "not found on PATH",
			Fix: "Solitary drives limactl on the host and podman through it in the machine.\n" +
				"Install Lima " + lima.MinVersion + " or newer from https://lima-vm.io, or with\n" +
				"the tap, which installs it alongside solitary:\n" +
				"  brew install balakin/solitary/solitary",
		}
	}

	version, err := lima.Version()
	if err != nil {
		return Check{
			Name:   "limactl",
			Status: Warn,
			Detail: fmt.Sprintf("%s does not report a version: %v", bin, err),
		}
	}

	if update.Newer(version, lima.MinVersion) {
		return Check{
			Name:   "limactl",
			Status: Fail,
			Detail: fmt.Sprintf("%s is version %s, older than the %s solitary is written against", bin, version, lima.MinVersion),
			Fix:    "Upgrade Lima. Machines created by an older one keep working; new ones may not be created at all.",
		}
	}

	return Check{Name: "limactl", Status: OK, Detail: fmt.Sprintf("%s, version %s", bin, version)}
}

func checkHypervisor() Check {
	v := host.Hypervisor()

	switch {
	case v.Available:
		return Check{Name: "hypervisor", Status: OK, Detail: v.Detail}
	case v.Fix == "":
		// Nothing to advise means the question could not be answered, not
		// that the answer was no.
		return Check{Name: "hypervisor", Status: Warn, Detail: v.Detail}
	default:
		return Check{Name: "hypervisor", Status: Fail, Detail: v.Detail, Fix: v.Fix}
	}
}

// checkMemory compares what the defined cells ask for against what this host
// can back a guest with.
//
// up already refuses a single machine larger than the filesystem behind it, but
// it only ever sees one cell. Whether they all fit at once is a question only
// something looking at the whole host can ask, and the answer is a warning
// rather than an error: cells are meant to outnumber the ones running.
func checkMemory() Check {
	backing, err := host.MemoryBacking()
	if err != nil {
		return Check{Name: "memory", Status: Warn, Detail: err.Error()}
	}
	machines, unreadable := definedMachines()

	return memoryStatus(backing, machines, unreadable)
}

// memoryStatus is the judgement checkMemory makes, separated from the host it
// asks about so it can be exercised against hosts this one is not.
func memoryStatus(backing host.Backing, machines []machine, unreadable int) Check {
	if !backing.Known {
		return Check{
			Name:   "memory",
			Status: OK,
			Detail: "guest memory is not backed by a file on this host, so there is no ceiling of that kind",
		}
	}

	var total uint64
	var oversized []string
	for _, m := range machines {
		want, err := host.ParseSize(m.memory)
		if err != nil {
			continue // a size that does not parse is the definition's problem
		}
		total += want
		if want > backing.Total {
			oversized = append(oversized, fmt.Sprintf("%s (%s)", m.name, host.FormatSize(want)))
		}
	}

	held := fmt.Sprintf("/dev/shm holds %s, %s of it free", host.FormatSize(backing.Total), host.FormatSize(backing.Free))

	if len(oversized) > 0 {
		return Check{
			Name:   "memory",
			Status: Fail,
			Detail: fmt.Sprintf("%s, and %s ask for more than all of it", held, strings.Join(oversized, ", ")),
			Fix: "A machine larger than that filesystem starts, reports itself running and then dies\n" +
				"with no error. Lower vm.memory to " + host.FormatSize(backing.Total) + " or less, or raise /dev/shm.",
		}
	}

	if total > backing.Total {
		return Check{
			Name:   "memory",
			Status: Warn,
			Detail: fmt.Sprintf("%s; the %s defined ask for %s between them", held, cells(len(machines)), host.FormatSize(total)),
			Fix:    "Each fits on its own, but they cannot all run at once. Stop one before starting another.",
		}
	}

	detail := held
	if len(machines) > 0 {
		detail = fmt.Sprintf("%s; the %s defined ask for %s between them", held, cells(len(machines)), host.FormatSize(total))
	}
	if unreadable > 0 {
		detail += fmt.Sprintf(" (%d could not be read)", unreadable)
	}

	return Check{Name: "memory", Status: OK, Detail: detail}
}

// Free space thresholds. They are not derived from what cells declare: Lima
// allocates a disk sparsely, so a machine occupies what it has written rather
// than its size, and summing the sizes would warn on every host forever. What
// matters is that there is room left to grow into.
const (
	diskWarn = 10 << 30
	diskFail = 2 << 30
)

func checkDisk() Check {
	home, err := lima.Home()
	if err != nil {
		return Check{Name: "disk", Status: Warn, Detail: err.Error()}
	}

	// A machine's disk grows inside Lima's home, which may not exist yet.
	// The filesystem is the same either way, so ask about the nearest
	// directory that does exist.
	free, err := host.DiskFree(nearestExisting(home))
	if err != nil {
		return Check{Name: "disk", Status: Warn, Detail: err.Error()}
	}

	return diskStatus(free, home)
}

// diskStatus is the judgement checkDisk makes, separated from the filesystem
// it asks about.
func diskStatus(free uint64, home string) Check {
	detail := fmt.Sprintf("%s free where Lima keeps machines (%s)", host.FormatSize(free), home)
	fix := "A guest whose disk cannot grow gets I/O errors rather than a message.\n" +
		"Free space here, or move Lima's machines with LIMA_HOME."

	switch {
	case free < diskFail:
		return Check{Name: "disk", Status: Fail, Detail: detail, Fix: fix}
	case free < diskWarn:
		return Check{Name: "disk", Status: Warn, Detail: detail, Fix: fix}
	default:
		return Check{Name: "disk", Status: OK, Detail: detail}
	}
}

// checkConfig reads the user-wide config and probes the state directory.
//
// The state directory is where the record of what each machine was created
// from lives. Nothing fails outright when it cannot be written — but drift
// goes undetected, so a change to vm or network is silently never noticed.
func checkConfig() Check {
	root, err := config.Root()
	if err != nil {
		return Check{Name: "config", Status: Fail, Detail: err.Error()}
	}

	if _, err := config.LoadUserConfig(); err != nil {
		return Check{
			Name:   "config",
			Status: Fail,
			Detail: err.Error(),
			Fix:    "Every cell reads this file for its defaults, so nothing starts until it parses.",
		}
	}

	state, err := config.StateDir()
	if err != nil {
		return Check{Name: "config", Status: Fail, Detail: err.Error()}
	}

	if err := probeWritable(state); err != nil {
		return Check{
			Name:   "config",
			Status: Warn,
			Detail: fmt.Sprintf("%s is not writable: %v", state, err),
			Fix: "Solitary records here what each machine was created from. Without it, a change to\n" +
				"vm, ports or network is never reported as needing a restart.",
		}
	}

	names, err := config.ListCells()
	if err != nil {
		return Check{Name: "config", Status: Warn, Detail: err.Error()}
	}
	if len(names) == 0 {
		return Check{Name: "config", Status: OK, Detail: root + ", no cells defined yet"}
	}

	return Check{Name: "config", Status: OK, Detail: fmt.Sprintf("%s, %s defined", root, cells(len(names)))}
}

// proxyVars are the variables a shell sets to put a proxy in front of
// everything. Lima can propagate them into a machine; solitary turns that off,
// because a proxy configuration names internal hosts and often carries
// credentials, and neither belongs in a cell that did not ask for them.
var proxyVars = []string{
	"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY",
	"http_proxy", "https_proxy", "all_proxy", "no_proxy",
}

// checkProxy reports a host proxy the cells will not inherit.
//
// It matters because of how it fails: a cell on a network that only reaches the
// internet through a proxy cannot resolve or connect, which looks exactly like
// an incomplete allow list, and the docs send you off to fix the wrong thing.
func checkProxy() Check {
	var set []string
	for _, name := range proxyVars {
		if os.Getenv(name) != "" {
			set = append(set, name)
		}
	}

	if len(set) == 0 {
		return Check{Name: "proxy", Status: OK, Detail: "no host proxy configured"}
	}

	return Check{
		Name:   "proxy",
		Status: Warn,
		Detail: strings.Join(set, ", ") + " set on the host; cells do not inherit it",
		Fix: "That is deliberate: a proxy configuration names internal hosts and carries credentials.\n" +
			"A cell that needs one has to set it itself, through its image or its secrets, and reach\n" +
			"the proxy through network.allow. Without that, a cell behind this proxy reaches nothing\n" +
			"and looks like an incomplete allow list.",
	}
}

// checkMachines compares the machines on this host with the cells defined for
// them. The two drift apart in ways nothing else reports: a cell renamed by
// renaming its directory leaves its old machine behind, and a machine whose
// disk grew once cannot be given a smaller vm.disk afterwards.
func checkMachines() Check {
	instances, err := lima.List()
	if err != nil {
		return Check{Name: "machines", Status: Warn, Detail: err.Error()}
	}
	defined, _ := definedMachines()

	// Measured here rather than in the judgement below, so that what is
	// asked of this host stays in one place. A disk that cannot be measured
	// is left out: an unknown size is not a finding.
	sizes := make(map[string]uint64, len(instances))
	for _, inst := range instances {
		if size := lima.DiskSize(inst.Name); size > 0 {
			sizes[inst.Name] = size
		}
	}

	return machinesStatus(instances, defined, sizes)
}

// machinesStatus is the judgement checkMachines makes, separated from the host
// it asks about so it can be exercised against hosts this one is not.
func machinesStatus(instances []lima.Instance, defined []machine, sizes map[string]uint64) Check {
	cellOf := make(map[string]machine, len(defined))
	for _, m := range defined {
		cellOf[config.Instance(m.name)] = m
	}

	var orphans, shrunk []string
	for _, inst := range instances {
		m, ok := cellOf[inst.Name]
		if !ok {
			if strings.HasPrefix(inst.Name, config.InstancePrefix) {
				orphans = append(orphans, inst.Name)
			}
			continue
		}

		actual, ok := sizes[inst.Name]
		if !ok {
			continue
		}
		want, err := host.ParseSize(m.disk)
		if err == nil && want < actual {
			shrunk = append(shrunk, fmt.Sprintf("%s asks for %s, its machine has %s",
				m.name, host.FormatSize(want), host.FormatSize(actual)))
		}
	}

	// Both are reported when both are there: a disk that cannot shrink is
	// about one cell, a machine with no cell is about another, and hiding
	// either behind the other is how one of them goes unnoticed for months.
	var detail, fix []string
	if len(shrunk) > 0 {
		detail = append(detail, strings.Join(shrunk, "; ")+"; a disk cannot be shrunk")
		fix = append(fix,
			"Raise vm.disk to what the machine already has, or discard the machine and",
			"everything on it with 'solitary rm <name>'.")
	}
	if len(orphans) > 0 {
		detail = append(detail, fmt.Sprintf("%s with no cell: %s", machineCount(len(orphans)), strings.Join(orphans, ", ")))
		fix = append(fix,
			"A cell is named by its directory, so renaming one leaves its machine behind,",
			"holding the disk it was given. Discard one with 'limactl delete <name>'.")
	}
	if len(detail) == 0 {
		return Check{
			Name:   "machines",
			Status: OK,
			Detail: fmt.Sprintf("%s, each matching the cell that defines it", machineCount(len(instances))),
		}
	}

	// A warning rather than a failure: the host is fine, and it is one cell
	// that will not start. up says which, and says it in the only place the
	// question comes up.
	return Check{
		Name:   "machines",
		Status: Warn,
		Detail: strings.Join(detail, "; "),
		Fix:    strings.Join(fix, "\n"),
	}
}

// machineCount pluralises a count, as cells does for the checks above.
func machineCount(n int) string {
	if n == 1 {
		return "1 machine"
	}

	return fmt.Sprintf("%d machines", n)
}

// machine is what the host-level checks need out of a cell definition.
type machine struct {
	name   string
	memory string
	disk   string
}

// definedMachines reads every cell definition, reporting how many could not be
// read rather than failing: a definition that does not parse is a question
// about that cell, and this is the half of doctor that asks about the host.
func definedMachines() (machines []machine, unreadable int) {
	names, err := config.ListCells()
	if err != nil {
		return nil, 0
	}
	sort.Strings(names)

	for _, name := range names {
		c, err := config.LoadCell(name)
		if err != nil {
			unreadable++
			continue
		}
		machines = append(machines, machine{name: name, memory: c.VM.Memory, disk: c.VM.Disk})
	}

	return machines, unreadable
}

// nearestExisting walks up from path to the first directory that exists, so a
// filesystem question can be asked about a directory not created yet.
func nearestExisting(path string) string {
	for {
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return path
		}
		path = parent
	}
}

// probeWritable reports whether a directory can be written to, by writing to
// it. Permissions alone do not answer the question — a read-only mount and an
// exhausted filesystem both pass that test and fail the real one.
func probeWritable(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		dir = nearestExisting(dir)
	}

	f, err := os.CreateTemp(dir, ".solitary-doctor-*")
	if err != nil {
		return err
	}
	name := f.Name()
	f.Close()

	return os.Remove(name)
}

// cells pluralises a count, which reads badly enough in these messages to be
// worth the three lines.
func cells(n int) string {
	if n == 1 {
		return "1 cell"
	}
	return fmt.Sprintf("%d cells", n)
}
