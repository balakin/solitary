package config

import "testing"

func TestResolvePrefersMostSpecificLayer(t *testing.T) {
	defaults := VM{Base: "ubuntu-lts", CPUs: 2, Memory: "4GiB", Disk: "20GiB"}
	user := VM{CPUs: 4, Memory: "8GiB"}
	cell := VM{Memory: "16GiB"}

	got := Resolve(cell, user, defaults)

	want := VM{
		Base:   "ubuntu-lts", // only the defaults set it
		CPUs:   4,            // user config wins over defaults
		Memory: "16GiB",      // the cell wins over both
		Disk:   "20GiB",
	}
	if got != want {
		t.Errorf("Resolve() = %+v, want %+v", got, want)
	}
}

func TestResolveTreatsZeroAsUnset(t *testing.T) {
	got := Resolve(VM{}, VM{}, Defaults())
	if got != Defaults() {
		t.Errorf("Resolve() with empty layers = %+v, want the defaults %+v", got, Defaults())
	}
}

func TestResolveProvisionReplacesRatherThanAppends(t *testing.T) {
	user := VM{Provision: "apt-get install -y user-tool"}
	cell := VM{Provision: "apt-get install -y cell-tool"}

	if got := Resolve(cell, user, VM{}).Provision; got != cell.Provision {
		t.Errorf("Provision = %q, want the cell's script verbatim", got)
	}
}

func TestValidateName(t *testing.T) {
	valid := []string{"a", "n1", "nvim-claude", "cell-2"}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{"", "-leading", "trailing-", "Upper", "has space", "under_score", "dots.here"}
	for _, name := range invalid {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) = nil, want an error", name)
		}
	}
}
