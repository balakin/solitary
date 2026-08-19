package host

import "fmt"

// DiskFree reports the space available on the filesystem holding path.
//
// A machine's disk is not this space minus its size: Lima allocates disks
// sparsely, so a 20GiB machine occupies what it has actually written and grows
// into whatever is left. That makes the sum of what cells declare meaningless
// and the free space itself the only number worth watching — a guest whose disk
// cannot grow gets I/O errors, not a message.
func DiskFree(path string) (uint64, error) {
	free, err := diskFree(path)
	if err != nil {
		return 0, fmt.Errorf("reading free space on %s: %w", path, err)
	}
	return free, nil
}
