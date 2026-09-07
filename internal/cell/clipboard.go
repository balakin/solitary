package cell

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/balakin/solitary/internal/clipboard"
)

// SendClipboard puts the image on this host's clipboard into a cell's inbox.
//
// This is a paste in the only shape a cell can take one. A terminal carries
// characters, so an image cannot be pasted into a shell in a cell however the
// clipboard is shared; and a cell reading the host's clipboard for itself would
// be the path back to the host that a cell exists not to have. So the host
// reads its own clipboard, when asked, and the image arrives as a file like
// every other thing sent in.
func SendClipboard(name string, progress io.Writer) error {
	image, err := clipboard.Image()
	if err != nil {
		return err
	}

	// Staged on the host and removed again: Send copies files by path, and
	// the name the inbox ends up with is this file's.
	dir, err := os.MkdirTemp("", "solitary-clipboard")
	if err != nil {
		return fmt.Errorf("staging the clipboard image: %w", err)
	}
	defer os.RemoveAll(dir)

	// 0644 rather than something tighter: the mode travels with the file,
	// and a cell whose container does not run as root has to be able to
	// read what was sent to it. The staging directory is private, so this
	// is not what the image is exposed at on the host.
	path := filepath.Join(dir, clipboardName(time.Now()))
	if err := os.WriteFile(path, image, 0o644); err != nil {
		return fmt.Errorf("staging the clipboard image: %w", err)
	}

	return Send(name, []string{path}, progress)
}

// clipboardName is what a pasted image is called in the inbox.
//
// A timestamp rather than one fixed name: pasting twice leaves two files rather
// than quietly replacing the one an agent is in the middle of reading, and the
// newest is the one that sorts last.
func clipboardName(at time.Time) string {
	return "clipboard-" + at.Format("20060102-150405") + ".png"
}
