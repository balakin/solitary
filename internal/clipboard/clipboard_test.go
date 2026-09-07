package clipboard

import (
	"errors"
	"testing"
)

// A host without a clipboard tool, or with an empty one, is the ordinary case
// in CI: both are answers rather than failures, and neither should return
// bytes. What cannot be asserted is the image itself — that needs someone to
// have copied one.
func TestImage(t *testing.T) {
	data, err := Image()
	switch {
	case err == nil && len(data) == 0:
		t.Error("Image() returned no error and no image")
	case errors.Is(err, ErrNoImage):
		t.Skip("nothing on this host's clipboard")
	case err != nil:
		t.Skipf("no clipboard on this host: %v", err)
	}
}
