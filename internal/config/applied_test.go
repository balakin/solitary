package config

import (
	"os"
	"path/filepath"
	"testing"
)

// isolate points both the cell directory and the state directory at temporary
// locations, so a test never reads or writes the real ones.
func isolate(t *testing.T) {
	t.Helper()

	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
}

func TestAppliedRecordRoundTrip(t *testing.T) {
	isolate(t)

	if got, err := ReadApplied("cell"); err != nil || got != "" {
		t.Fatalf("ReadApplied() = (%q, %v), want empty for a cell never created", got, err)
	}

	if err := WriteApplied("cell", "rendered definition"); err != nil {
		t.Fatalf("WriteApplied() error = %v", err)
	}

	got, err := ReadApplied("cell")
	if err != nil {
		t.Fatalf("ReadApplied() error = %v", err)
	}
	if want := Digest("rendered definition"); got != want {
		t.Errorf("ReadApplied() = %q, want %q", got, want)
	}
}

// The record is solitary's own bookkeeping and must not appear among the files
// a person edits.
func TestAppliedRecordIsNotInTheCellDirectory(t *testing.T) {
	isolate(t)

	if err := WriteApplied("cell", "rendered definition"); err != nil {
		t.Fatalf("WriteApplied() error = %v", err)
	}

	dir, err := CellDir("cell")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, e := range entries {
		t.Errorf("cell directory contains generated file %q", e.Name())
	}

	state, err := AppliedFile("cell")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(state); err != nil {
		t.Errorf("record was not written to the state directory: %v", err)
	}
}

func TestRemoveApplied(t *testing.T) {
	isolate(t)

	if err := WriteApplied("cell", "rendered"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveApplied("cell"); err != nil {
		t.Fatalf("RemoveApplied() error = %v", err)
	}
	if got, err := ReadApplied("cell"); err != nil || got != "" {
		t.Errorf("ReadApplied() = (%q, %v), want empty after removal", got, err)
	}

	// Removing twice is not an error: rm runs on cells that were never created.
	if err := RemoveApplied("cell"); err != nil {
		t.Errorf("RemoveApplied() on a missing record error = %v", err)
	}
}
