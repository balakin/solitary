package cell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/balakin/solitary/internal/config"
	"github.com/balakin/solitary/internal/lima"
)

// attentionFixture is a cell whose machine was given exactly what the cell
// describes, with the record to say so, and the machine's directory holding the
// definition Lima stored.
func attentionFixture(t *testing.T, status string) (*config.Cell, *lima.Instance, string) {
	t.Helper()
	isolate(t)

	c := &config.Cell{VM: config.Defaults()}
	rendered, err := lima.Render("probe", c.VM, nil, config.Network{})
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	stored := filepath.Join(dir, "lima.yaml")
	if err := os.WriteFile(stored, []byte("stored definition\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	record := config.NewApplied(rendered, "")
	record.Stored = config.Digest("stored definition\n")
	if err := config.WriteApplied("probe", record); err != nil {
		t.Fatal(err)
	}

	return c, &lima.Instance{Name: "solitary-probe", Status: status, Dir: dir}, stored
}

func whats(found []Attention) string {
	var all []string
	for _, a := range found {
		all = append(all, a.What)
	}
	return strings.Join(all, "; ")
}

// A machine that holds what solitary gave it and runs what the cell describes
// needs nothing, running or not.
func TestAttentionIsEmptyForAMachineThatMatches(t *testing.T) {
	for _, status := range []string{lima.StatusRunning, lima.StatusStopped} {
		c, inst, _ := attentionFixture(t, status)
		if found := attention("probe", c, inst); len(found) != 0 {
			t.Errorf("%s: attention() = %q, want nothing", status, whats(found))
		}
	}
}

// An edit to the stored definition is reported either way, and the fix says
// what puts it back for the state the machine is in.
func TestAttentionReportsAnEditedDefinition(t *testing.T) {
	for status, fix := range map[string]string{
		lima.StatusStopped: "up puts back",
		lima.StatusRunning: "stop and start",
	} {
		c, inst, stored := attentionFixture(t, status)
		if err := os.WriteFile(stored, []byte("stored definition\nssh:\n  forwardAgent: true\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		found := attention("probe", c, inst)
		if len(found) != 1 || !strings.Contains(found[0].What, "edited outside solitary") || !strings.Contains(found[0].Fix, fix) {
			t.Errorf("%s: attention() = %+v, want the edit and %q", status, found, fix)
		}
	}
}

// Settings that changed matter only to a running machine, which holds the old
// ones until it restarts. A stopped one takes them at the next up unasked.
func TestAttentionReportsChangedSettingsOnlyWhileRunning(t *testing.T) {
	for status, want := range map[string]bool{lima.StatusRunning: true, lima.StatusStopped: false} {
		c, inst, _ := attentionFixture(t, status)
		c.VM.CPUs++

		found := attention("probe", c, inst)
		if got := strings.Contains(whats(found), "settings that changed"); got != want {
			t.Errorf("%s: attention() = %q, want changed settings reported: %v", status, whats(found), want)
		}
	}
}

// A changed provision script is reported whatever state the machine is in:
// nothing short of rebuilding it undoes what the old script did.
func TestAttentionReportsAChangedProvisionScript(t *testing.T) {
	c, inst, _ := attentionFixture(t, lima.StatusStopped)
	record, err := config.ReadApplied("probe")
	if err != nil {
		t.Fatal(err)
	}
	record.Provision = config.Digest("apt-get install -y ripgrep")
	if err := config.WriteApplied("probe", record); err != nil {
		t.Fatal(err)
	}
	c.VM.Provision = "apt-get install -y ripgrep fd-find"

	found := attention("probe", c, inst)
	if len(found) != 1 || !strings.Contains(found[0].What, "vm.provision") || !strings.Contains(found[0].Fix, "--new-machine") {
		t.Errorf("attention() = %+v, want the provision script and how to apply it", found)
	}
}

// An ignored vm.base is about the definition, so it is reported for a cell
// with no machine as well; a machine with no record has nothing to compare and
// reports nothing else.
func TestAttentionReportsAnIgnoredBaseWithoutAMachine(t *testing.T) {
	isolate(t)

	c := &config.Cell{VM: config.Defaults(), IgnoredBase: "ubuntu-lts"}
	found := attention("probe", c, nil)
	if len(found) != 1 || !strings.Contains(found[0].What, "vm.base (ubuntu-lts)") {
		t.Errorf("attention() = %+v, want the ignored base alone", found)
	}

	found = attention("probe", c, &lima.Instance{Name: "solitary-probe", Status: lima.StatusRunning, Dir: t.TempDir()})
	if len(found) != 1 {
		t.Errorf("attention() = %+v for a machine with no record, want the ignored base alone", found)
	}
}

// Security fixes a machine was last seen without are something to act on; a
// reboot it waits on is only while it runs, since starting a stopped one is
// that reboot.
func TestAttentionNamesMissingSecurityFixes(t *testing.T) {
	security, updates := 3, 10
	for _, tc := range []struct {
		status string
		guest  Guest
		want   string
	}{
		{lima.StatusRunning, Guest{Updates: &updates, Security: &security}, "machine: 3 security updates pending"},
		{lima.StatusStopped, Guest{Updates: &updates, Security: &security, RebootRequired: true}, "machine: 3 security updates pending"},
		{lima.StatusRunning, Guest{RebootRequired: true}, "machine: installed fixes wait on a reboot"},
		{lima.StatusStopped, Guest{RebootRequired: true}, ""},
		{lima.StatusRunning, Guest{Updates: &updates}, ""},
	} {
		c, inst, _ := attentionFixture(t, tc.status)
		if err := writeGuest("probe", tc.guest); err != nil {
			t.Fatal(err)
		}
		if got := whats(attention("probe", c, inst)); got != tc.want {
			t.Errorf("%s, %+v: attention() = %q, want %q", tc.status, tc.guest, got, tc.want)
		}
	}
}
