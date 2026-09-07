//go:build !linux && !darwin

package clipboard

import "errors"

// image reports that the question cannot be answered here.
func image() ([]byte, error) {
	return nil, errors.New("no clipboard reader for this platform")
}
