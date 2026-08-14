//go:build !linux

package host

// shmCapacity reports that the question does not apply. On macOS the guest's
// memory is not backed by a file on a sized filesystem, so there is no ceiling
// of this kind to check against.
func shmCapacity() (total, free uint64, ok bool, err error) {
	return 0, 0, false, nil
}
