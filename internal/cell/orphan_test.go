package cell

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/balakin/solitary/internal/config"
	"github.com/balakin/solitary/internal/lima"
)

// A machine whose cell is gone is an orphan; one that still has a definition,
// and one Lima manages for something else, are not. A machine carrying the
// parameter says so, and one predating it is still listed.
func TestOrphansAmong(t *testing.T) {
	got := orphansAmong([]lima.Instance{
		{Name: "solitary-gone"},
		{Name: "solitary-kept"},
		{Name: "docker"},
		marked("solitary-also-gone", "also-gone"),
	}, []string{"kept"})

	want := []Orphan{{Name: "also-gone", Marked: true}, {Name: "gone"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("orphansAmong() = %v, want %v", got, want)
	}
}

// marked builds an instance carrying the parameter solitary renders into every
// definition, the way limactl reports one back.
func marked(instance, cell string) lima.Instance {
	inst := lima.Instance{Name: instance}
	inst.Config.Param = map[string]string{lima.ParamCell: cell}

	return inst
}

// The prefix is the only mark a machine carries, so a machine named without it
// is not ours to sweep even when no cell claims it.
func TestOrphansAmongIgnoresForeignMachines(t *testing.T) {
	if got := orphansAmong([]lima.Instance{{Name: "default"}}, nil); got != nil {
		t.Fatalf("orphansAmong() = %v, want none", got)
	}
}

// Describe answers for a name with no definition rather than failing: the row
// it is asked about was listed on purpose.
func TestDescribeOrphan(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))

	detail, err := Describe("vanished")
	if err != nil {
		t.Fatalf("Describe() error = %v", err)
	}
	if !detail.Orphaned {
		t.Error("Describe() Orphaned = false, want true")
	}
	if detail.Name != "vanished" {
		t.Errorf("Describe() Name = %q, want %q", detail.Name, "vanished")
	}
}

// An invalid name is still refused, so nothing built from one reaches the
// filesystem.
func TestDescribeRejectsInvalidName(t *testing.T) {
	if _, err := Describe("../escape"); err == nil {
		t.Fatal("Describe() error = nil, want one")
	}
}

// A definition is what makes a cell, so a directory without one is not counted.
func TestHasCell(t *testing.T) {
	writeCell(t, definition, "")

	for name, want := range map[string]bool{"probe": true, "absent": false} {
		got, err := config.HasCell(name)
		if err != nil {
			t.Fatalf("HasCell(%q) error = %v", name, err)
		}
		if got != want {
			t.Errorf("HasCell(%q) = %v, want %v", name, got, want)
		}
	}
}
