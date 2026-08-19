//go:build !linux && !darwin

package host

// hypervisor reports that the question cannot be answered here. Solitary ships
// for linux and darwin; anywhere else, let the check say so rather than claim
// a machine will start.
func hypervisor() Virtualization {
	return Virtualization{Detail: "no hypervisor check for this platform"}
}
