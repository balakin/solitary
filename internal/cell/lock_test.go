package cell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A second command on a cell refuses, says who has it, and can go ahead once
// the first lets go; another cell is not held up by it.
func TestLockCellRefusesASecondHolder(t *testing.T) {
	isolate(t)

	unlock, err := lockCell("probe")
	if err != nil {
		t.Fatal(err)
	}
	_, err = lockCell("probe")
	if err == nil {
		t.Fatal("a second lockCell on a held cell succeeded")
	}
	if want := fmt.Sprintf("busy with another solitary command (process %d)", os.Getpid()); !strings.Contains(err.Error(), want) {
		t.Errorf("lockCell() error = %q, want it to contain %q", err, want)
	}

	other, err := lockCell("other")
	if err != nil {
		t.Fatalf("locking another cell: %v", err)
	}
	other()

	unlock()
	again, err := lockCell("probe")
	if err != nil {
		t.Fatalf("lockCell() after unlock: %v", err)
	}
	again()
}

func TestLockCellRefusesAPath(t *testing.T) {
	isolate(t)
	if _, err := lockCell("../escape"); err == nil {
		t.Error("lockCell() took a name that is a path")
	}
}

// A home left as saved is the home once its machine is gone, and stale while
// the machine is there.
func TestClaimSavedHomeOnlyWithoutAMachine(t *testing.T) {
	if _, err := exec.LookPath("limactl"); err != nil {
		t.Skip("limactl not installed")
	}
	isolate(t)

	_, saved, pending, err := homeArchives("probe")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(saved), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(saved, []byte("home"), 0o600); err != nil {
		t.Fatal(err)
	}

	// No machine called solitary-probe exists on any host running this.
	if err := claimSavedHome("probe", &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	if exists(saved) || !exists(pending) {
		t.Errorf("after claimSavedHome: saved %v, pending %v; want the saved home waiting", exists(saved), exists(pending))
	}
}
