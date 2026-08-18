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

	if got, err := ReadApplied("cell"); err != nil || got.Recorded() {
		t.Fatalf("ReadApplied() = (%+v, %v), want nothing recorded for a cell never created", got, err)
	}

	want := NewApplied("rendered definition", "apt-get install -y ripgrep")
	if err := WriteApplied("cell", want); err != nil {
		t.Fatalf("WriteApplied() error = %v", err)
	}

	got, err := ReadApplied("cell")
	if err != nil {
		t.Fatalf("ReadApplied() error = %v", err)
	}
	if got != want {
		t.Errorf("ReadApplied() = %+v, want %+v", got, want)
	}
	if !got.Recorded() {
		t.Error("Recorded() = false for a record that was written")
	}
	if got.ProvisionChanged("apt-get install -y ripgrep") {
		t.Error("ProvisionChanged() = true for the script that was recorded")
	}
	if !got.ProvisionChanged("apt-get install -y ripgrep fd-find") {
		t.Error("ProvisionChanged() = false for a script that was edited")
	}
}

// A record written before the file had fields is one bare digest of the
// definition. The machine it describes is still the machine it describes, so it
// is read rather than discarded — and what it does not say about vm.provision
// is treated as not known, not as unchanged.
func TestAppliedRecordReadsTheOlderFormat(t *testing.T) {
	isolate(t)

	path, err := AppliedFile("cell")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(Digest("rendered definition")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ReadApplied("cell")
	if err != nil {
		t.Fatalf("ReadApplied() error = %v", err)
	}
	if got.Definition != Digest("rendered definition") {
		t.Errorf("Definition = %q, want the digest the file holds", got.Definition)
	}
	if !got.Recorded() {
		t.Error("Recorded() = false for a machine the older format describes")
	}
	if got.ProvisionChanged("anything at all") {
		t.Error("ProvisionChanged() = true for a record that says nothing about the script")
	}
}

// A cell with no vm.provision has nothing to record for it, and the field it
// leaves empty must not be read back as part of another one.
func TestAppliedRecordRoundTripsAnEmptyField(t *testing.T) {
	isolate(t)

	want := Applied{Definition: Digest("rendered definition")}
	if err := WriteApplied("cell", want); err != nil {
		t.Fatal(err)
	}

	got, err := ReadApplied("cell")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("ReadApplied() = %+v, want %+v", got, want)
	}
}

// The record is solitary's own bookkeeping and must not appear among the files
// a person edits.
func TestAppliedRecordIsNotInTheCellDirectory(t *testing.T) {
	isolate(t)

	if err := WriteApplied("cell", NewApplied("rendered definition", "")); err != nil {
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

	if err := WriteApplied("cell", NewApplied("rendered", "")); err != nil {
		t.Fatal(err)
	}
	if err := RemoveApplied("cell"); err != nil {
		t.Fatalf("RemoveApplied() error = %v", err)
	}
	if got, err := ReadApplied("cell"); err != nil || got.Recorded() {
		t.Errorf("ReadApplied() = (%+v, %v), want nothing recorded after removal", got, err)
	}

	// Removing twice is not an error: rm runs on cells that were never created.
	if err := RemoveApplied("cell"); err != nil {
		t.Errorf("RemoveApplied() on a missing record error = %v", err)
	}
}
