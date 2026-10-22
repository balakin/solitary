package cell

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/balakin/solitary/internal/config"
)

// writeCell lays out a cell definition and its secrets in a temporary home.
func writeCell(t *testing.T, definition, env string) {
	t.Helper()

	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	// Lookup shells out to limactl, which must not run in a test.
	t.Setenv("PATH", t.TempDir())

	dir, err := config.CellDir("probe")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cell.yaml"), []byte(definition), 0o600); err != nil {
		t.Fatal(err)
	}
	if env != "" {
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

const definition = `image: docker.io/library/ubuntu:24.04
secrets:
  SET_ONE:
  MISSING_ONE:
  OPTIONAL_ONE:
    required: false
    description: only needed to push
ports:
  - 8080
vm:
  cpus: 4
  memory: 8GiB
`

func TestDescribeReportsWhichSecretsAreSet(t *testing.T) {
	writeCell(t, definition, "SET_ONE=\"a value\"\n")

	detail, err := Describe("probe")
	if err != nil {
		t.Fatalf("Describe() error = %v", err)
	}

	if detail.Name != "probe" || detail.Image != "docker.io/library/ubuntu:24.04" {
		t.Errorf("Describe() = %+v, want the cell's own definition", detail)
	}
	if detail.VM.CPUs != 4 || detail.VM.Memory != "8GiB" {
		t.Errorf("machine = %+v, want the cell's overrides", detail.VM)
	}
	if len(detail.Ports) != 1 || detail.Ports[0] != 8080 {
		t.Errorf("ports = %v, want [8080]", detail.Ports)
	}

	want := []SecretState{
		{Name: "SET_ONE", Set: true, Required: true},
		{Name: "MISSING_ONE", Set: false, Required: true},
		{Name: "OPTIONAL_ONE", Set: false, Required: false, Description: "only needed to push"},
	}
	if !reflect.DeepEqual(detail.Secrets, want) {
		t.Errorf("secrets = %+v, want %+v", detail.Secrets, want)
	}
}

// Describe feeds a screen. A value that reached it could be shown, logged or
// screen-shared, so it must not be carried at all.
func TestDescribeNeverCarriesASecretValue(t *testing.T) {
	writeCell(t, definition, "SET_ONE=\"sk-ant-the-actual-secret\"\n")

	detail, err := Describe("probe")
	if err != nil {
		t.Fatalf("Describe() error = %v", err)
	}

	if strings.Contains(dump(t, detail), "sk-ant-the-actual-secret") {
		t.Error("Describe() carries a secret's value")
	}
}

// dump renders every field of a detail, so the check above cannot be fooled by
// a value hiding somewhere other than the secrets list.
func dump(t *testing.T, detail Detail) string {
	t.Helper()

	var b strings.Builder
	b.WriteString(detail.Name + detail.Image)
	b.WriteString(detail.VM.Base + detail.VM.Memory + detail.VM.Disk + detail.VM.Provision)
	for _, s := range detail.Secrets {
		b.WriteString(s.Name)
	}

	return b.String()
}

// A cell that declares no secrets has none to report, rather than an entry
// saying so.
func TestDescribeWithoutSecrets(t *testing.T) {
	writeCell(t, "image: docker.io/library/ubuntu:24.04\n", "")

	detail, err := Describe("probe")
	if err != nil {
		t.Fatalf("Describe() error = %v", err)
	}
	if len(detail.Secrets) != 0 {
		t.Errorf("secrets = %+v, want none", detail.Secrets)
	}
}

// The base a machine was made from is told the way up will record it: as the
// record has it, or — for a record from before it was kept — the old default
// or the vm.base the cell names. A cell with no record has no machine to name.
func TestDescribeSaysWhatTheMachineWasCreatedFrom(t *testing.T) {
	for name, tt := range map[string]struct {
		definition string
		record     *config.Applied
		want       string
	}{
		"no machine":      {definition: "image: ubuntu\n", want: ""},
		"recorded":        {definition: "image: ubuntu\n", record: &config.Applied{Definition: "d", Base: config.Base}, want: config.Base},
		"an older record": {definition: "image: ubuntu\n", record: &config.Applied{Definition: "d"}, want: config.LegacyBase},
		"a written base":  {definition: "image: ubuntu\nvm:\n  base: ubuntu-24.04\n", record: &config.Applied{Definition: "d"}, want: "ubuntu-24.04"},
	} {
		t.Run(name, func(t *testing.T) {
			writeCell(t, tt.definition, "")
			if tt.record != nil {
				if err := config.WriteApplied("probe", *tt.record); err != nil {
					t.Fatal(err)
				}
			}

			detail, err := Describe("probe")
			if err != nil {
				t.Fatal(err)
			}
			if detail.CreatedFrom != tt.want {
				t.Errorf("CreatedFrom = %q, want %q", detail.CreatedFrom, tt.want)
			}
		})
	}
}
