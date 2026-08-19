//go:build !linux && !darwin

package host

import "errors"

// diskFree reports that the question cannot be answered here.
func diskFree(string) (uint64, error) {
	return 0, errors.New("no free space check for this platform")
}
