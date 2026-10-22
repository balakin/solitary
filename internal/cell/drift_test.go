package cell

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/balakin/solitary/internal/config"
)

// isolate points the state directory at a temporary one, so a test never reads
// or writes the record of a real cell.
func isolate(t *testing.T) {
	t.Helper()

	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
}

// cellWith is a loaded cell carrying one provision script.
func cellWith(provision string) *config.Cell {
	return &config.Cell{VM: config.VM{Provision: provision}}
}

// A machine whose definition matches what the cell says is not something to
// warn about, and neither is one nobody recorded: the warning names a change,
// and there is no change to name.
func TestWarnDriftIsSilentWithoutAKnownChange(t *testing.T) {
	isolate(t)

	c := cellWith("apt-get install -y ripgrep")

	var out bytes.Buffer
	warnDrift("probe", c, "definition", "", &out)
	if out.Len() != 0 {
		t.Errorf("warnDrift() with no record wrote %q", out.String())
	}

	if err := config.WriteApplied("probe", config.NewApplied("definition", c.VM.Provision)); err != nil {
		t.Fatal(err)
	}
	warnDrift("probe", c, "definition", "", &out)
	if out.Len() != 0 {
		t.Errorf("warnDrift() on an unchanged definition wrote %q", out.String())
	}

	// Every setting the definition carries is read at boot, so the warning
	// must not name only the vm block.
	warnDrift("probe", c, "definition with an allow list", "", &out)
	if got := out.String(); !strings.Contains(got, "network") || !strings.Contains(got, "solitary down probe") {
		t.Errorf("warnDrift() = %q, want the settings that changed and how to apply them", got)
	}
}

// vm.provision is the one setting restarting a machine cannot apply, since what
// the old script did is already on the disk. So it is said separately, and it
// says the only thing that does undo it.
func TestWarnDriftReportsAChangedProvisionScript(t *testing.T) {
	isolate(t)

	if err := config.WriteApplied("probe", config.NewApplied("definition", "apt-get install -y ripgrep")); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	warnDrift("probe", cellWith("apt-get install -y ripgrep fd-find"), "definition", "", &out)

	got := out.String()
	for _, want := range []string{"vm.provision", "nothing undoes it", "solitary rm probe && solitary up probe"} {
		if !strings.Contains(got, want) {
			t.Errorf("warnDrift() = %q, want it to contain %q", got, want)
		}
	}
	// The definition itself did not change, so the restart advice would be
	// wrong here: stopping and starting applies nothing.
	if strings.Contains(got, "solitary down probe") {
		t.Errorf("warnDrift() = %q, want no restart advice for a change a restart cannot apply", got)
	}
}

// A record from a version that did not keep the script's digest says nothing
// about the script. Warning on it would mean warning on every cell that
// predates the record, about a change nobody made.
func TestWarnDriftIsSilentOnAProvisionScriptItCannotCompare(t *testing.T) {
	isolate(t)

	if err := config.WriteApplied("probe", config.Applied{Definition: config.Digest("definition")}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	warnDrift("probe", cellWith("apt-get install -y ripgrep"), "definition", "", &out)
	if strings.Contains(out.String(), "vm.provision") {
		t.Errorf("warnDrift() = %q, want nothing said about a script that was never recorded", out.String())
	}
}

// A running machine cannot have its definition replaced, so an edit to it is
// said, along with the restart that lets up put it back.
func TestWarnDriftReportsAnEditedDefinition(t *testing.T) {
	isolate(t)

	c := cellWith("")
	record := config.NewApplied("definition", "")
	record.Stored = config.Digest("stored")
	if err := config.WriteApplied("probe", record); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	warnDrift("probe", c, "definition", "stored", &out)
	if out.Len() != 0 {
		t.Errorf("warnDrift() on the definition solitary stored wrote %q", out.String())
	}

	warnDrift("probe", c, "definition", "stored\nssh:\n  forwardAgent: true\n", &out)
	got := out.String()
	for _, want := range []string{"edited outside solitary", "solitary down probe && solitary up probe"} {
		if !strings.Contains(got, want) {
			t.Errorf("warnDrift() = %q, want it to contain %q", got, want)
		}
	}
}

// A machine whose definition matches what the cell says, and whose stored
// definition is the one solitary left there, is not rewritten.
//
// "definition" is not a machine definition and there is no machine named
// solitary-probe, so anything that reached Lima would fail in these tests. A
// test that expects an error is asserting that the definition was applied.
func TestApplyDriftLeavesAMatchingMachineAlone(t *testing.T) {
	isolate(t)

	c := cellWith("apt-get install -y ripgrep")
	record := config.NewApplied("definition", c.VM.Provision)
	record.Stored = config.Digest("stored")
	if err := config.WriteApplied("probe", record); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := applyDrift("probe", "solitary-probe", c, "definition", "stored", &out); err != nil {
		t.Fatalf("applyDrift() error = %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("applyDrift() wrote %q for a machine that matches", out.String())
	}
}

// An edit to the stored definition is put back even when the cell itself did
// not change — that is the case the record exists for.
func TestApplyDriftRestoresAnEditedDefinition(t *testing.T) {
	isolate(t)

	c := cellWith("")
	record := config.NewApplied("definition", "")
	record.Stored = config.Digest("stored")
	if err := config.WriteApplied("probe", record); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := applyDrift("probe", "solitary-probe", c, "definition", "stored\nssh:\n  forwardAgent: true\n", &out)
	if err == nil {
		t.Fatal("applyDrift() = nil, want the edited definition replaced")
	}
	if !strings.Contains(out.String(), "edited outside solitary; restoring it") {
		t.Errorf("applyDrift() = %q, want the edit named", out.String())
	}
	if strings.Contains(out.String(), "settings for") {
		t.Errorf("applyDrift() = %q, want no claim that the cell's settings changed", out.String())
	}
}

// A record from before the stored definition was kept cannot vouch for what
// the machine holds, so the machine is written once — quietly, since nothing
// is known to have changed.
func TestApplyDriftRewritesAMachineItCannotVerify(t *testing.T) {
	isolate(t)

	c := cellWith("apt-get install -y ripgrep")
	if err := config.WriteApplied("probe", config.NewApplied("definition", c.VM.Provision)); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := applyDrift("probe", "solitary-probe", c, "definition", "stored", &out); err == nil {
		t.Fatal("applyDrift() = nil, want a record without the stored definition to rewrite the machine")
	}
	if out.Len() != 0 {
		t.Errorf("applyDrift() wrote %q for a machine with no known change", out.String())
	}
}
