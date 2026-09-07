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
	dir := t.TempDir()
	script := "#!/bin/sh\nsleep 60 &\nsleep 60\n"
	if err := os.WriteFile(filepath.Join(dir, "limactl"), []byte(script), 0o755); err != nil {
		t.Fatalf("writing the fake limactl: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	previous := execTimeout
	execTimeout = 200 * time.Millisecond
	t.Cleanup(func() { execTimeout = previous })

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
