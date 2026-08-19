package host

// Virtualization is what this host can tell us about its ability to run a
// machine at all. Available false with an empty Fix means the question could
// not be answered here, not that the answer was no.
type Virtualization struct {
	Available bool

	// Detail states what was found, as one line.
	Detail string

	// Fix says what to do about it, or is empty when nothing can be done.
	Fix string
}

// Hypervisor reports whether a machine can run on this host.
//
// Lima fails this late and obscurely — a create that runs for a while and then
// dies, or a machine that starts under emulation and is merely slow — so it is
// worth asking before anything else.
func Hypervisor() Virtualization {
	return hypervisor()
}
