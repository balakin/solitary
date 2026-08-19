//go:build linux || darwin

package host

import "syscall"

// diskFree reports the space available to this user on the filesystem holding
// path.
func diskFree(path string) (free uint64, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}

	return uint64(st.Bsize) * st.Bavail, nil
}
