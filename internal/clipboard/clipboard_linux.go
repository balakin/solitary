package clipboard

import (
	"fmt"
	"os"
	"os/exec"
)

// image reads the clipboard of whichever display server this session runs.
//
// The session picks the tool rather than both being tried in turn: wl-paste
// under X11 and xclip under Wayland do not fail because the clipboard holds no
// image, they fail because there is no server of their kind to ask — and
// reporting that as an empty clipboard would send someone looking in the wrong
// place.
func image() ([]byte, error) {
	tool, args := "xclip", []string{"-selection", "clipboard", "-t", PNG, "-o"}
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		tool, args = "wl-paste", []string{"--no-newline", "--type", PNG}
	}

	if _, err := exec.LookPath(tool); err != nil {
		return nil, fmt.Errorf("%s reads the clipboard on this session and is not installed", tool)
	}

	return run(tool, args...)
}
