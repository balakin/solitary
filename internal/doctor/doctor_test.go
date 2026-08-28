package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/balakin/solitary/internal/host"
	"github.com/balakin/solitary/internal/lima"
)

// isolate points the config and state trees at temporary locations, so a test
// never reads or writes the real ones.
func isolate(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))

	return root
}

// writeCell writes a minimal definition for a cell asking for memory.
func writeCell(t *testing.T, name, memory string) {
	t.Helper()

	dir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "solitary", "cells", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	body := "image: example.com/tools:latest\n"
	if memory != "" {
		body += "vm:\n  memory: " + memory + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "cell.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const gib = 1 << 30

func TestMemoryStatus(t *testing.T) {
	backing := host.Backing{Total: 8 * gib, Free: 6 * gib, Known: true}

	tests := []struct {
		name     string
		machines []machine
		want     Status
		detail   string
	}{
		{
			name:     "fits",
			machines: []machine{{name: "a", memory: "4GiB"}},
			want:     OK,
		},
		{
			// Cells are meant to outnumber the ones running, so this is
			// a warning and not a failure.
			name:     "together they do not fit",
			machines: []machine{{name: "a", memory: "6GiB"}, {name: "b", memory: "6GiB"}},
			want:     Warn,
		},
		{
			// A machine larger than the filesystem behind it cannot run
			// at all, however few of them there are.
			name:     "one is larger than the host can back",
			machines: []machine{{name: "a", memory: "16GiB"}},
			want:     Fail,
			detail:   "a (16.0GiB)",
		},
		{
			name:     "no cells defined",
			machines: nil,
			want:     OK,
		},
		{
			// The definition's problem, not the host's: it is reported
			// by the check that reads definitions.
			name:     "a size that does not parse is ignored",
			machines: []machine{{name: "a", memory: "plenty"}},
			want:     OK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := memoryStatus(backing, tt.machines, 0)
			if got.Status != tt.want {
				t.Errorf("memoryStatus() = %q (%s), want %q", got.Status, got.Detail, tt.want)
			}
			if tt.detail != "" && !strings.Contains(got.Detail, tt.detail) {
				t.Errorf("memoryStatus() detail = %q, want it to name %q", got.Detail, tt.detail)
			}
		})
	}
}

func TestMemoryStatusUnknownBacking(t *testing.T) {
	// macOS does not back guest memory with a file, so there is no ceiling
	// of this kind and a cell asking for more than the host has is not this
	// check's business.
	got := memoryStatus(host.Backing{}, []machine{{name: "a", memory: "64GiB"}}, 0)
	if got.Status != OK {
		t.Errorf("memoryStatus() = %q (%s), want %q where the backing is unknown", got.Status, got.Detail, OK)
	}
}

func TestMemoryStatusReportsUnreadable(t *testing.T) {
	got := memoryStatus(host.Backing{Total: 8 * gib, Free: 8 * gib, Known: true}, nil, 2)
	if !strings.Contains(got.Detail, "2 could not be read") {
		t.Errorf("memoryStatus() detail = %q, want it to say two definitions could not be read", got.Detail)
	}
}

func TestDiskStatus(t *testing.T) {
	tests := []struct {
		free uint64
		want Status
	}{
		{free: 40 * gib, want: OK},
		{free: diskWarn - 1, want: Warn},
		{free: diskFail - 1, want: Fail},
	}

	for _, tt := range tests {
		got := diskStatus(tt.free, "/home/someone/.lima")
		if got.Status != tt.want {
			t.Errorf("diskStatus(%s) = %q, want %q", host.FormatSize(tt.free), got.Status, tt.want)
		}
		if !strings.Contains(got.Detail, "/home/someone/.lima") {
			t.Errorf("diskStatus() detail = %q, want it to name where machines are kept", got.Detail)
		}
	}
}

func TestCheckProxy(t *testing.T) {
	for _, name := range proxyVars {
		t.Setenv(name, "")
	}

	if got := checkProxy(); got.Status != OK {
		t.Errorf("checkProxy() = %q (%s), want %q with no proxy set", got.Status, got.Detail, OK)
	}

	t.Setenv("HTTPS_PROXY", "http://proxy.internal:3128")

	got := checkProxy()
	if got.Status != Warn {
		t.Errorf("checkProxy() = %q, want %q: a cell does not inherit the host's proxy", got.Status, Warn)
	}
	if !strings.Contains(got.Detail, "HTTPS_PROXY") {
		t.Errorf("checkProxy() detail = %q, want it to name the variable that is set", got.Detail)
	}
}

func TestCheckConfig(t *testing.T) {
	isolate(t)

	// Nothing on disk at all is the state a fresh install is in.
	if got := checkConfig(); got.Status != OK {
		t.Errorf("checkConfig() = %q (%s), want %q before anything is defined", got.Status, got.Detail, OK)
	}

	writeCell(t, "claude", "4GiB")

	got := checkConfig()
	if got.Status != OK {
		t.Errorf("checkConfig() = %q (%s), want %q", got.Status, got.Detail, OK)
	}
	if !strings.Contains(got.Detail, "1 cell") {
		t.Errorf("checkConfig() detail = %q, want it to count the cell", got.Detail)
	}
}

func TestCheckConfigUnparseableUserConfig(t *testing.T) {
	isolate(t)

	root := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "solitary")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte("vm: [not a mapping\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Every cell reads this file for its defaults, so nothing starts until
	// it parses.
	if got := checkConfig(); got.Status != Fail {
		t.Errorf("checkConfig() = %q (%s), want %q", got.Status, got.Detail, Fail)
	}
}

func TestDefinedMachines(t *testing.T) {
	isolate(t)

	writeCell(t, "beta", "8GiB")
	writeCell(t, "alpha", "")

	machines, unreadable := definedMachines()
	if unreadable != 0 {
		t.Errorf("definedMachines() unreadable = %d, want 0", unreadable)
	}
	if len(machines) != 2 {
		t.Fatalf("definedMachines() = %+v, want two cells", machines)
	}
	if machines[0].name != "alpha" || machines[1].name != "beta" {
		t.Errorf("definedMachines() = %+v, want them in alphabetical order", machines)
	}
	// A cell that names no memory still has one: the user-wide config, then
	// the built-in default, are merged in when it is loaded.
	if machines[0].memory == "" {
		t.Error("definedMachines() left memory empty for a cell that names none, want the resolved default")
	}
}

func TestDefinedMachinesCountsUnreadable(t *testing.T) {
	isolate(t)

	writeCell(t, "good", "4GiB")

	dir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "solitary", "cells", "bad")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Neither an image nor a build: a definition that cannot be used.
	if err := os.WriteFile(filepath.Join(dir, "cell.yaml"), []byte("secrets:\n  TOKEN:\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	machines, unreadable := definedMachines()
	if len(machines) != 1 || unreadable != 1 {
		t.Errorf("definedMachines() = (%+v, %d), want one readable and one not", machines, unreadable)
	}
}

func TestNearestExisting(t *testing.T) {
	root := t.TempDir()

	if got := nearestExisting(root); got != root {
		t.Errorf("nearestExisting(%q) = %q, want the directory itself", root, got)
	}

	missing := filepath.Join(root, "lima", "machines", "one")
	if got := nearestExisting(missing); got != root {
		t.Errorf("nearestExisting(%q) = %q, want %q", missing, got, root)
	}
}

func TestProbeWritable(t *testing.T) {
	root := t.TempDir()

	// A directory that does not exist yet is answered for by the filesystem
	// it would be created on.
	if err := probeWritable(filepath.Join(root, "state", "solitary")); err != nil {
		t.Errorf("probeWritable() error = %v, want a writable temporary directory to pass", err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("probeWritable() left %v behind, want nothing: doctor changes nothing", entries)
	}

	readonly := filepath.Join(root, "readonly")
	if err := os.Mkdir(readonly, 0o500); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: permissions do not stop a write")
	}
	if err := probeWritable(readonly); err == nil {
		t.Error("probeWritable() error = nil for a directory that cannot be written to")
	}
}

func TestFailed(t *testing.T) {
	if Failed([]Check{{Status: OK}, {Status: Warn}}) {
		t.Error("Failed() = true with only a warning: a warning is not a broken host")
	}
	if !Failed([]Check{{Status: OK}, {Status: Fail}}) {
		t.Error("Failed() = false with a failed check")
	}
}

func TestHostAnswersEveryCheck(t *testing.T) {
	isolate(t)

	checks := Host()
	if len(checks) == 0 {
		t.Fatal("Host() returned nothing")
	}

	for _, c := range checks {
		if c.Name == "" {
			t.Errorf("check %+v has no name", c)
		}
		// A check that cannot be answered has to say so rather than pass
		// silently: doctor exists not to stop at the first problem.
		if c.Detail == "" {
			t.Errorf("check %q has no detail", c.Name)
		}
		switch c.Status {
		case OK, Warn, Fail:
		default:
			t.Errorf("check %q has status %q", c.Name, c.Status)
		}
	}
}

func TestMachinesStatus(t *testing.T) {
	tests := []struct {
		name      string
		instances []lima.Instance
		defined   []machine
		sizes     map[string]uint64
		want      Status
		detail    string
	}{
		{
			name:      "every machine matches its cell",
			instances: []lima.Instance{{Name: "solitary-a"}},
			defined:   []machine{{name: "a", disk: "20GiB"}},
			sizes:     map[string]uint64{"solitary-a": 20 * gib},
			want:      OK,
		},
		{
			// Lima grows a disk and refuses to shrink one, so this is
			// the cell that will not start until it is said out loud.
			name:      "a cell asks for less than its machine has",
			instances: []lima.Instance{{Name: "solitary-a"}},
			defined:   []machine{{name: "a", disk: "20GiB"}},
			sizes:     map[string]uint64{"solitary-a": 50 * gib},
			want:      Warn,
			detail:    "a asks for 20.0GiB, its machine has 50.0GiB",
		},
		{
			// A larger disk is fine: that one Lima applies at the next
			// start.
			name:      "a cell asks for more than its machine has",
			instances: []lima.Instance{{Name: "solitary-a"}},
			defined:   []machine{{name: "a", disk: "80GiB"}},
			sizes:     map[string]uint64{"solitary-a": 50 * gib},
			want:      OK,
		},
		{
			// Renaming a cell renames its directory, and the machine
			// the old name was backed by stays where it was.
			name:      "a machine with no cell",
			instances: []lima.Instance{{Name: "solitary-old"}},
			want:      Warn,
			detail:    "solitary-old",
		},
		{
			// Lima manages machines for other tools on the same host,
			// and those are not this tool's to comment on.
			name:      "a machine that is not a cell's",
			instances: []lima.Instance{{Name: "default"}},
			want:      OK,
		},
		{
			// Neither finding hides the other.
			name:      "both at once",
			instances: []lima.Instance{{Name: "solitary-a"}, {Name: "solitary-old"}},
			defined:   []machine{{name: "a", disk: "20GiB"}},
			sizes:     map[string]uint64{"solitary-a": 50 * gib},
			want:      Warn,
			detail:    "solitary-old",
		},
		{
			// A disk that could not be measured is not a finding.
			name:      "an unmeasurable disk",
			instances: []lima.Instance{{Name: "solitary-a"}},
			defined:   []machine{{name: "a", disk: "20GiB"}},
			want:      OK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := machinesStatus(tt.instances, tt.defined, tt.sizes)
			if got.Status != tt.want {
				t.Errorf("machinesStatus() = %q (%s), want %q", got.Status, got.Detail, tt.want)
			}
			if tt.detail != "" && !strings.Contains(got.Detail, tt.detail) {
				t.Errorf("machinesStatus() detail = %q, want it to name %q", got.Detail, tt.detail)
			}
			if got.Status != OK && got.Fix == "" {
				t.Error("machinesStatus() left a finding with nothing to do about it")
			}
		})
	}
}

// Both findings at once have to carry both fixes, since each names a different
// command.
func TestMachinesStatusFixesBoth(t *testing.T) {
	got := machinesStatus(
		[]lima.Instance{{Name: "solitary-a"}, {Name: "solitary-old"}},
		[]machine{{name: "a", disk: "20GiB"}},
		map[string]uint64{"solitary-a": 50 * gib},
	)

	for _, want := range []string{"solitary rm", "limactl delete"} {
		if !strings.Contains(got.Fix, want) {
			t.Errorf("machinesStatus() fix = %q, want it to name %q", got.Fix, want)
		}
	}
}
