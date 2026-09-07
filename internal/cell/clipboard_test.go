package cell

import (
	"testing"
	"time"
)

// The name is generated here and joined onto a path inside the machine, so it
// has to pass the same check a name chosen inside a cell does.
func TestClipboardName(t *testing.T) {
	name := clipboardName(time.Date(2026, 9, 7, 14, 3, 9, 0, time.UTC))
	if name != "clipboard-20260907-140309.png" {
		t.Errorf("clipboardName() = %q", name)
	}
	if err := validName(name); err != nil {
		t.Errorf("validName(%q) = %v", name, err)
	}
}
