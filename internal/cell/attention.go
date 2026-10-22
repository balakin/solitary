package cell

import (
	"fmt"
	"io"

	"github.com/balakin/solitary/internal/config"
	"github.com/balakin/solitary/internal/lima"
)

// Attention is something about a cell that will not right itself: what is
// wrong, and what to do about it.
type Attention struct {
	What string
	Fix  string
}

// attention works out what about a cell needs someone to act on it, from what
// this host holds alone: the definition, the record of what its machine was
// given, and the definition Lima stores for that machine. Nothing here asks a
// machine anything, so a listing can afford it for every cell at every refresh.
//
// It names only what someone has to do. A stopped machine whose settings
// changed is not here: the next up applies them without being asked.
func attention(name string, c *config.Cell, inst *lima.Instance) []Attention {
	var found []Attention
	if c.IgnoredBase != "" {
		found = append(found, Attention{
			What: fmt.Sprintf("vm.base (%s) is set but no longer read", c.IgnoredBase),
			Fix:  "remove it from cell.yaml or config.yaml",
		})
	}
	if inst == nil {
		return found
	}

	record, err := config.ReadApplied(name)
	if err != nil || !record.Recorded() {
		return found
	}
	running := inst.Status == lima.StatusRunning

	if stored, err := inst.StoredDefinition(); err == nil && record.Edited(stored) {
		fix := "up puts back what the cell describes"
		if running {
			fix = "stop and start the cell; up puts back what it describes"
		}
		found = append(found, Attention{What: "machine definition edited outside solitary", Fix: fix})
	}
	if running && settingsChanged(name, c, record) {
		found = append(found, Attention{
			What: "running with settings that changed since it started",
			Fix:  "stop and start the cell to apply them",
		})
	}
	if record.ProvisionChanged(c.VM.Provision) {
		found = append(found, Attention{
			What: "vm.provision changed since the machine was provisioned",
			Fix:  "rm then up rebuilds it; the disk and home go with it",
		})
	}

	return found
}

// settingsChanged reports whether the definition the cell describes now is not
// the one its machine was last given — rendered the way up would render it,
// GPU included, so that a cell up would leave alone is not reported.
func settingsChanged(name string, c *config.Cell, record config.Applied) bool {
	vm := c.VM
	vm.GPU = chooseGPU(name, c, io.Discard)

	rendered, err := lima.Render(name, vm, c.Ports, c.Network)
	if err != nil {
		return false
	}

	return record.Definition != config.Digest(rendered)
}
