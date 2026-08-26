package cell

import (
	"strings"
	"testing"
)

const gib = 1 << 30

// The refusal exists because Lima's own message names neither the cell nor the
// setting, so everything needed to act on it has to be in this one.
func TestDiskRefusalNamesBothSizesAndBothWaysOut(t *testing.T) {
	err := diskRefusal("vscode", "/home/ada/.config/solitary/cells/vscode/cell.yaml", 20*gib, 50*gib)
	if err == nil {
		t.Fatal("diskRefusal() = nil for a disk that would have to shrink")
	}

	for _, want := range []string{
		"vscode",      // which cell
		"20.0GiB",     // what it asks for
		"50.0GiB",     // what it has
		"vm.disk",     // the setting to change
		"cell.yaml",   // where that setting lives
		"solitary rm", // the other way out
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("diskRefusal() = %q, want it to name %q", err, want)
		}
	}
}

// The suggested size is one to type into cell.yaml, so it is a whole unit, and
// it never rounds below the disk it has to clear.
func TestWholeGiBRoundsUp(t *testing.T) {
	tests := []struct {
		bytes uint64
		want  string
	}{
		{50 * gib, "50GiB"},
		{50*gib + 1, "51GiB"},
		{gib / 2, "1GiB"},
	}

	for _, tt := range tests {
		if got := wholeGiB(tt.bytes); got != tt.want {
			t.Errorf("wholeGiB(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}
