package host

import "testing"

func TestHypervisorAnswers(t *testing.T) {
	v := Hypervisor()

	if v.Detail == "" {
		t.Error("Hypervisor() gave no detail, want it to state what it found either way")
	}
	// A host that cannot run a machine and can do nothing about it is a real
	// answer; an unavailable one with advice attached is the fixable case.
	if v.Available && v.Fix != "" {
		t.Errorf("Hypervisor() = available with a fix %q, want no advice when nothing is wrong", v.Fix)
	}
}
