//go:build linux

package host

import (
	"errors"
	"io/fs"
	"os"
)

// kvmPath is the device Lima's default driver needs on Linux. Without it a
// machine falls back to emulation or refuses to start, and either way the
// failure arrives minutes into a create rather than up front.
const kvmPath = "/dev/kvm"

// hypervisor reports whether this host can run a machine, by opening the KVM
// device the way a hypervisor would.
//
// Presence alone is not the question. The device is owned by a group, and a
// user outside it gets a permission error that names nothing about groups — so
// the check that matches the real failure is whether this user can open it.
func hypervisor() Virtualization {
	f, err := os.OpenFile(kvmPath, os.O_RDWR, 0)
	if err == nil {
		f.Close()
		return Virtualization{Available: true, Detail: kvmPath + " is present and this user can open it"}
	}

	if errors.Is(err, fs.ErrNotExist) {
		return Virtualization{
			Detail: kvmPath + " does not exist, so this host cannot run a machine",
			Fix: "Hardware virtualization is off in the firmware, or this host is itself a guest\n" +
				"without nested virtualization. Neither can be fixed from here.",
		}
	}

	if errors.Is(err, fs.ErrPermission) {
		return Virtualization{
			Detail: kvmPath + " exists but this user cannot open it",
			Fix: "Add yourself to the group that owns it, then log in again:\n" +
				"  sudo usermod -aG kvm \"$USER\"",
		}
	}

	return Virtualization{Detail: "cannot open " + kvmPath + ": " + err.Error()}
}
