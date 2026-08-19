package lima

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

// MinVersion is the oldest Lima solitary is written against, as the
// installation docs state it.
const MinVersion = "2.0"

// Binary is the limactl solitary would run, or ErrNotInstalled.
func Binary() (string, error) {
	return limactl()
}

// versionRE pulls the version out of `limactl --version`, which prints a line
// of the form "limactl version 2.0.1" and may print more besides.
var versionRE = regexp.MustCompile(`\b([0-9]+(?:\.[0-9]+)+)\b`)

// Version reports the version of the limactl on PATH, without a leading v.
func Version() (string, error) {
	bin, err := limactl()
	if err != nil {
		return "", err
	}

	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("running limactl --version: %w", err)
	}

	return parseVersion(out)
}

// parseVersion pulls the version out of what limactl --version printed.
func parseVersion(out []byte) (string, error) {
	m := versionRE.FindSubmatch(out)
	if m == nil {
		return "", fmt.Errorf("cannot read a version out of %q", out)
	}

	return string(m[1]), nil
}

// Home is the directory Lima keeps its machines in, which is where a cell's
// disk grows. LIMA_HOME moves it; otherwise it is ~/.lima.
func Home() (string, error) {
	if dir := os.Getenv("LIMA_HOME"); dir != "" {
		return dir, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home directory: %w", err)
	}

	return filepath.Join(home, ".lima"), nil
}
