//go:build darwin

package host

import "syscall"

// hvSupport is the sysctl macOS answers with whether the CPU can back a
// hypervisor at all. Virtualization.framework, which Lima uses by default,
// needs it; a Mac too old for it has no other driver to fall back to.
const hvSupport = "kern.hv_support"

// hypervisor reports whether this host can run a machine.
func hypervisor() Virtualization {
	supported, err := syscall.SysctlUint32(hvSupport)
	if err != nil {
		return Virtualization{Detail: "cannot read " + hvSupport + ": " + err.Error()}
	}

	if supported == 0 {
		return Virtualization{
			Detail: hvSupport + " is 0, so this host cannot run a machine",
			Fix:    "This Mac cannot back a hypervisor. Nothing can be changed from here.",
		}
	}

	return Virtualization{Available: true, Detail: hvSupport + " is 1"}
}
