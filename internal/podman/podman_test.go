package podman

import (
	"reflect"
	"strings"
	"testing"
)

// A command run from a script must reach the container exactly as given: no
// shell, no terminal bootstrap, nothing that could reinterpret its arguments.
func TestExecArgsWithoutATerminalRunTheCommandDirectly(t *testing.T) {
	args := execArgs(false, []string{"git", "commit", "-m", "two words"})

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

	args := execArgs(true, []string{"nvim", "a file.txt"})
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
