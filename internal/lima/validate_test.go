package lima

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/balakin/solitary/internal/config"
)

// TestRenderedDefinitionsPassLimactlValidate checks the rendered YAML against
// the schema of the Lima actually installed, which a golden file cannot do.
// It is skipped where limactl is unavailable.
func TestRenderedDefinitionsPassLimactlValidate(t *testing.T) {
	limactl, err := exec.LookPath("limactl")
	if err != nil {
		t.Skip("limactl not installed")
	}

	cases := map[string]struct {
		vm      config.VM
		ports   []int
		network config.Network
	}{
		"defaults":  {vm: config.Defaults()},
		"withPorts": {vm: config.Defaults(), ports: []int{8080, 3000}},
		"provision": {vm: config.VM{Base: "ubuntu-lts", CPUs: 4, Memory: "8GiB", Disk: "40GiB", Provision: "echo hello\necho world"}},
		"restricted": {vm: config.Defaults(), network: config.Network{
			Allow: []string{"github.com", "api.anthropic.com", "10.1.2.0/24"},
		}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rendered, err := Render(goldenCell, tc.vm, tc.ports, tc.network)
			if err != nil {
				t.Fatalf("Render(goldenCell) error = %v", err)
			}

			path := filepath.Join(t.TempDir(), "cell.yaml")
			if err := os.WriteFile(path, []byte(rendered), 0o600); err != nil {
				t.Fatalf("writing definition: %v", err)
			}

			if out, err := exec.Command(limactl, "validate", path).CombinedOutput(); err != nil {
				t.Errorf("limactl validate rejected the rendered definition: %v\n%s\n--- definition ---\n%s", err, out, rendered)
			}
		})
	}
}

// Lima merges default.yaml and override.yaml into every machine it creates, so
// either one can add the host mounts a cell is defined without. Render refuses
// to work under a LIMA_HOME that has them rather than trust what they say.
func TestRenderRefusesGlobalLimaSettings(t *testing.T) {
	for _, name := range []string{"default.yaml", "override.yaml"} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("LIMA_HOME", home)
			if err := os.MkdirAll(filepath.Join(home, "_config"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, "_config", name), []byte("mounts:\n  - location: /tmp\n    writable: true\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Render(goldenCell, config.Defaults(), nil, config.Network{}); err == nil {
				t.Errorf("Render() accepted a LIMA_HOME with %s", name)
			}
		})
	}
}
