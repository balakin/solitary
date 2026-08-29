package podman

import (
	"reflect"
	"strings"
	"testing"
)

// A command run from a script must reach the container exactly as given: no
// shell, no terminal bootstrap, nothing that could reinterpret its arguments.
func TestExecArgsWithoutATerminalRunTheCommandDirectly(t *testing.T) {
	args := execArgs(false, "", []string{"git", "commit", "-m", "two words"})

	for _, unwanted := range []string{"--tty", "/bin/sh"} {
		for _, got := range args {
			if got == unwanted {
				t.Errorf("scripted exec passes %s: %q", unwanted, args)
			}
		}
	}
	if got := args[len(args)-4:]; !reflect.DeepEqual(got, []string{"git", "commit", "-m", "two words"}) {
		t.Errorf("command reached podman as %q", got)
	}
}

// An interactive session gets a terminal and the bootstrap, and the command
// still arrives unchanged after it.
func TestExecArgsWithATerminalCarryTheTerminal(t *testing.T) {
	t.Setenv("TERM", "xterm-ghostty")
	t.Setenv("COLORTERM", "truecolor")

	args := execArgs(true, "", []string{"nvim", "a file.txt"})
	joined := strings.Join(args, " ")

	if !strings.Contains(joined, "--tty") {
		t.Error("interactive exec did not ask for a terminal")
	}
	if !strings.Contains(joined, "TERM=xterm-ghostty") || !strings.Contains(joined, "COLORTERM=truecolor") {
		t.Errorf("terminal was not passed in: %q", args)
	}
	if got := args[len(args)-2:]; !reflect.DeepEqual(got, []string{"nvim", "a file.txt"}) {
		t.Errorf("command reached podman as %q", got)
	}
	// $0 for the bootstrap, so that "$@" is the command and nothing else.
	if got := args[len(args)-3]; got != "solitary" {
		t.Errorf("argument before the command = %q, want a name for $0", got)
	}
}

// A cell that names a user is attached to as that user, so that what a session
// leaves in the home belongs to whoever works there.
func TestExecArgsRunAsTheCellsUser(t *testing.T) {
	args := execArgs(false, "cell", []string{"id"})

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--user cell") {
		t.Errorf("session did not run as the cell's user: %q", args)
	}

	// The ordinary cell has no user of its own and is attached to as the
	// image's, which podman is left to work out.
	if got := strings.Join(execArgs(false, "", []string{"id"}), " "); strings.Contains(got, "--user") {
		t.Errorf("a cell without a user passed one anyway: %q", got)
	}
}

// isRoot decides whether a cell needs a mapping at all, so it has to know
// every way an image has of saying root.
func TestIsRoot(t *testing.T) {
	for _, user := range []string{"", "root", "0", "root:root", "0:0"} {
		if !isRoot(user) {
			t.Errorf("isRoot(%q) = false, want true", user)
		}
	}
	for _, user := range []string{"cell", "1000", "cell:cell", "1000:1000"} {
		if isRoot(user) {
			t.Errorf("isRoot(%q) = true, want false", user)
		}
	}
}

// A cell that names no device is run exactly as it was before devices
// existed: nothing is passed through, and the label says so.
func TestRunArgsWithoutDevicesPassNone(t *testing.T) {
	args := runArgs(RunOptions{Image: "alpine", Command: "sleep infinity"}, nil)

	for _, got := range args {
		if got == "--device" {
			t.Errorf("a cell with no devices was given one: %q", args)
		}
	}
	if !strings.Contains(strings.Join(args, " "), deviceLabel+"= ") {
		t.Errorf("the device label was not written empty: %q", args)
	}
}

// Devices reach podman in the order the definition wrote them, and the label
// records the same list — that is what tells the next up whether the running
// container still matches the file.
func TestRunArgsPassDevicesAndRecordThem(t *testing.T) {
	devices := []string{"/dev/kvm", "/dev/net/tun"}
	args := runArgs(RunOptions{Image: "alpine", Command: "sleep infinity", Devices: devices}, nil)

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--device /dev/kvm --device /dev/net/tun") {
		t.Errorf("devices did not reach podman in order: %q", args)
	}
	if !strings.Contains(joined, deviceLabel+"=/dev/kvm /dev/net/tun") {
		t.Errorf("devices were not recorded on the container: %q", args)
	}
	if got := DeviceList(devices); got != "/dev/kvm /dev/net/tun" {
		t.Errorf("DeviceList() = %q", got)
	}
}
