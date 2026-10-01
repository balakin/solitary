package lima

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// limactl reports the whole definition back with each machine, and the
// parameter solitary wrote into it comes with it. This is one line of real
// limactl list --json output, trimmed to the fields that are read.
func TestInstanceCell(t *testing.T) {
	const line = `{"name":"solitary-probe","status":"Running","dir":"/home/u/.lima/solitary-probe",` +
		`"config":{"param":{"internal_netplanOptional":"true","solitary_cell":"probe"}}}`

	var inst Instance
	if err := json.Unmarshal([]byte(line), &inst); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}

	name, ok := inst.Cell()
	if !ok || name != "probe" {
		t.Errorf("Cell() = %q, %v, want %q, true", name, ok, "probe")
	}
}

// A machine created before solitary wrote the parameter says nothing about
// which cell it belongs to, which has to be an answer rather than an empty
// name that reads like one.
func TestInstanceCellOfAnUnmarkedMachine(t *testing.T) {
	var inst Instance
	if err := json.Unmarshal([]byte(`{"name":"solitary-old","config":{"param":{}}}`), &inst); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}

	if name, ok := inst.Cell(); ok {
		t.Errorf("Cell() = %q, true, want no name", name)
	}
}

// A machine that stops answering is what execTimeout is for, and the child
// limactl leaves behind is what used to defeat it: ssh outlives the kill and
// keeps the output pipe open, so Wait never returns and `solitary ls` and the
// dashboard hang on "Reading cells…" instead of reporting the machine.
//
// The fake limactl here is that shape — a background child holding the same
// stdout, and a foreground one that never exits.
func TestExecGivesUpOnAChildThatOutlivesTheDeadline(t *testing.T) {
	fakeLimactl(t, "#!/bin/sh\nsleep 60 &\nsleep 60\n")
	shortenDeadlines(t)

	done := make(chan error, 1)
	go func() {
		_, err := Exec("solitary-probe", "true")
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, ErrUnreachable) {
			t.Fatalf("Exec err = %v, want %v", err, ErrUnreachable)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Exec did not return: the deadline did not reach the whole process tree")
	}
}

// shortenDeadlines makes a test's fake limactl run out of time in milliseconds
// rather than the seconds a real machine is given.
func shortenDeadlines(t *testing.T) {
	t.Helper()
	previousExec, previousProbe := execTimeout, probeTimeout
	execTimeout, probeTimeout = 200*time.Millisecond, 200*time.Millisecond
	t.Cleanup(func() { execTimeout, probeTimeout = previousExec, previousProbe })
}

// fakeLimactl puts a limactl on PATH that runs script.
func fakeLimactl(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "limactl"), []byte(script), 0o755); err != nil {
		t.Fatalf("writing the fake limactl: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// A machine fresh from boot can keep its disk busy long enough for podman to
// run past the deadline while the machine itself answers at once. That is a
// cell to wait for, not one to restart, so it must not come back as
// unreachable.
//
// The fake limactl here answers `true` and hangs on everything else.
func TestExecTellsASlowCommandFromAHungMachine(t *testing.T) {
	fakeLimactl(t, "#!/bin/sh\nfor last; do :; done\n[ \"$last\" = true ] && exit 0\nsleep 60\n")
	shortenDeadlines(t)

	_, err := Exec("solitary-probe", "podman", "container", "inspect", "solitary")
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("Exec err = %v, want %v", err, ErrBusy)
	}
	if errors.Is(err, ErrUnreachable) {
		t.Fatalf("Exec err = %v, which also claims the machine is unreachable", err)
	}
}

// Reachable is the probe itself, so a machine that does not answer it is
// unreachable after one deadline — not after a second probe on top.
func TestReachableOnAHungMachine(t *testing.T) {
	marks := filepath.Join(t.TempDir(), "calls")
	fakeLimactl(t, "#!/bin/sh\necho >> "+marks+"\nsleep 60\n")
	shortenDeadlines(t)

	done := make(chan bool, 1)
	go func() { done <- Reachable("solitary-probe") }()

	select {
	case ok := <-done:
		if ok {
			t.Fatal("Reachable = true on a machine that never answers")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Reachable did not return")
	}

	calls, err := os.ReadFile(marks)
	if err != nil {
		t.Fatalf("reading the calls: %v", err)
	}
	if n := len(calls); n != 1 {
		t.Errorf("limactl ran %d times, want 1", n)
	}
}
