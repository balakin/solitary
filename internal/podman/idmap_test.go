package podman

import (
	"strings"
	"testing"
)

// The table podman reports on an ordinary machine: the container's root is the
// machine's user, and everything else is borrowed from /etc/subuid.
var rootless = idMappings{
	UIDs: []idRange{{ContainerID: 0, HostID: 1000, Size: 1}, {ContainerID: 1, HostID: 100000, Size: 65536}},
	GIDs: []idRange{{ContainerID: 0, HostID: 1000, Size: 1}, {ContainerID: 1, HostID: 100000, Size: 65536}},
}

func TestMachineID(t *testing.T) {
	for id, want := range map[int]int{0: 1000, 1: 100000, 1001: 101000, 65536: 165535} {
		if got, ok := machineID(rootless.UIDs, id); !ok || got != want {
			t.Errorf("machineID(%d) = %d, %v, want %d", id, got, ok, want)
		}
	}
	if _, ok := machineID(rootless.UIDs, 65537); ok {
		t.Error("an id past the subordinate range was mapped")
	}
}

// The view swaps two ids and nothing else: the machine's user appears as the
// cell's user, and what the container's root writes lands where keep-id put
// it. This is the rule the machine was tested with, written out.
func TestViewRuleSwapsOwnerAndRoot(t *testing.T) {
	got, err := viewRule(rootless, 1001, 1001)
	if err != nil {
		t.Fatal(err)
	}

	want := "u:1000:101000:1 u:100000:1000:1 g:1000:101000:1 g:100000:1000:1"
	if got != want {
		t.Errorf("viewRule() = %q, want %q", got, want)
	}
}

// The user and the group are separate ids and are mapped separately: an image
// may well give its user a group with another number.
func TestViewRuleMapsGroupOnItsOwn(t *testing.T) {
	got, err := viewRule(rootless, 1001, 2000)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "u:1000:101000:1") || !strings.Contains(got, "g:1000:101999:1") {
		t.Errorf("viewRule() = %q, user and group not mapped to their own ids", got)
	}
}

// A user podman has no id for cannot be given its home, and that is the
// image's to fix rather than something to work around.
func TestViewRuleRefusesAnUnmappableUser(t *testing.T) {
	if _, err := viewRule(rootless, 70000, 70000); err == nil || !strings.Contains(err.Error(), "70000") {
		t.Errorf("viewRule() error = %v, want one naming the id", err)
	}
}

// A machine without subordinate ids leaves nothing to put the cell's user on.
func TestViewRuleNeedsSubordinateIDs(t *testing.T) {
	bare := idMappings{
		UIDs: []idRange{{ContainerID: 0, HostID: 1000, Size: 1}},
		GIDs: []idRange{{ContainerID: 0, HostID: 1000, Size: 1}},
	}
	if _, err := viewRule(bare, 1001, 1001); err == nil {
		t.Error("viewRule() mapped a user on a machine with no subordinate ids")
	}
}

// A cell's user is one of the subordinate ids on the machine, and a device is
// handed to that id rather than to the machine's user, which inside the
// container is root.
func TestDeviceOwnerIsTheUsersMachineID(t *testing.T) {
	got, err := deviceOwner(rootless, 1001)
	if err != nil {
		t.Fatal(err)
	}
	if got != 101000 {
		t.Errorf("deviceOwner(1001) = %d, want 101000", got)
	}
}

func TestDeviceOwnerRefusesAnUnmappableUser(t *testing.T) {
	if _, err := deviceOwner(rootless, 70000); err == nil || !strings.Contains(err.Error(), "70000") {
		t.Errorf("deviceOwner() error = %v, want one naming the id", err)
	}
}
