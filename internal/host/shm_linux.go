//go:build linux

package host

import "syscall"

// shmCapacity reports the size and free space of the filesystem backing guest
// memory.
func shmCapacity() (total, free uint64, ok bool, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(shmPath, &st); err != nil {
		return 0, 0, false, err
	}

	block := uint64(st.Bsize)

	return block * st.Blocks, block * st.Bavail, true, nil
}
