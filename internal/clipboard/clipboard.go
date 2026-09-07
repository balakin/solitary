// Package clipboard reads an image off this host's clipboard.
//
// Only this host's, and only on the way in. A cell has no clipboard of its own
// and no channel back to reach this one, so a paste is the host copying a file
// in — never something inside a cell asking for whatever is on the clipboard
// now, which at any moment might be a password rather than a screenshot.
package clipboard

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// PNG is the only type ever asked for. A clipboard image is offered in several
// and the tools will convert between them; one type keeps what lands in a cell
// predictable, and a screenshot is a PNG everywhere solitary runs.
const PNG = "image/png"

// ErrNoImage is what an empty clipboard, or one holding text, comes back as.
// It is not a failure to read the clipboard: the read worked and there was no
// image in it, which is worth saying differently.
var ErrNoImage = errors.New("no image on the clipboard")

// Image returns the clipboard's image, as PNG bytes.
func Image() ([]byte, error) {
	data, err := image()
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, ErrNoImage
	}

	return data, nil
}

// run reads one tool's idea of the clipboard. Every platform's reader has the
// same shape: a command that writes the image to stdout and exits non-zero
// when the clipboard holds anything else.
func run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	var out, problem bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &problem

	if err := cmd.Run(); err != nil {
		// These tools fail the same way for an empty clipboard as for one
		// holding text, and explain themselves only on stderr. Nothing
		// was written either way, so the distinction is not worth
		// parsing their prose for.
		if out.Len() == 0 {
			return nil, ErrNoImage
		}
		return nil, fmt.Errorf("reading the clipboard with %s: %w: %s",
			name, err, strings.TrimSpace(problem.String()))
	}

	return out.Bytes(), nil
}
