// Package host inspects the machine solitary runs on, to catch settings that
// cannot work here before they are applied.
package host

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// shmPath is where Lima backs a guest's memory on Linux. The guest's entire RAM
// is a file here, so a machine asking for more than this filesystem holds dies
// the moment it touches enough pages — KVM returns EFAULT and the VM is gone,
// while the process stays alive and Lima still reports it as running.
const shmPath = "/dev/shm"

// Backing is how much memory the host can back a guest with. Known is false
// where the question does not apply, such as macOS, where the hypervisor does
// not back guest memory with a file.
type Backing struct {
	Total uint64
	Free  uint64
	Known bool
}

// MemoryBacking reports the host's capacity for backing guest memory.
func MemoryBacking() (Backing, error) {
	total, free, ok, err := shmCapacity()
	if err != nil {
		return Backing{}, fmt.Errorf("reading %s: %w", shmPath, err)
	}
	return Backing{Total: total, Free: free, Known: ok}, nil
}

var sizeRE = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)\s*([A-Za-z]*)$`)

var units = map[string]uint64{
	"":    1,
	"b":   1,
	"k":   1 << 10,
	"kib": 1 << 10,
	"kb":  1000,
	"m":   1 << 20,
	"mib": 1 << 20,
	"mb":  1000 * 1000,
	"g":   1 << 30,
	"gib": 1 << 30,
	"gb":  1000 * 1000 * 1000,
	"t":   1 << 40,
	"tib": 1 << 40,
	"tb":  1000 * 1000 * 1000 * 1000,
}

// ParseSize reads a size as Lima writes them, e.g. "4GiB", "512MiB", "2G".
func ParseSize(s string) (uint64, error) {
	m := sizeRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, fmt.Errorf("cannot read %q as a size", s)
	}

	value, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, fmt.Errorf("cannot read %q as a size: %w", s, err)
	}

	unit, ok := units[strings.ToLower(m[2])]
	if !ok {
		return 0, fmt.Errorf("unknown unit %q in %q", m[2], s)
	}

	return uint64(value * float64(unit)), nil
}

// FormatSize renders a byte count the way sizes are written in cell.yaml.
func FormatSize(b uint64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1fGiB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.0fMiB", float64(b)/float64(1<<20))
	default:
		return fmt.Sprintf("%d bytes", b)
	}
}

// VerifyMemory checks a machine's memory against what the host can back.
//
// A machine larger than the backing filesystem cannot run at all, so that is an
// error rather than a warning: creating it would produce a VM that boots, is
// reported as running, and then dies without a message anyone sees.
func VerifyMemory(memory string, b Backing) (warning string, err error) {
	if !b.Known || memory == "" {
		return "", nil
	}

	want, err := ParseSize(memory)
	if err != nil {
		return "", err
	}

	if want > b.Total {
		return "", fmt.Errorf(
			"this machine asks for %s of memory, but %s on this host holds %s, and that is where the guest's memory lives\n"+
				"         A machine larger than that starts, reports itself running and then dies with no error\n"+
				"         Lower vm.memory to %s or less, or raise %s",
			FormatSize(want), shmPath, FormatSize(b.Total), FormatSize(b.Total), shmPath)
	}

	if want > b.Free {
		return fmt.Sprintf(
			"this machine asks for %s of memory and only %s of %s is free right now.\n"+
				"         Stop another cell if it fails to start.",
			FormatSize(want), FormatSize(b.Free), shmPath), nil
	}

	return "", nil
}
