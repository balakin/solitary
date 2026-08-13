package lima

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dm-balakin/solitary/internal/config"
)

var update = os.Getenv("UPDATE_GOLDEN") != ""

func TestRenderGolden(t *testing.T) {
	cases := []struct {
		name   string
		vm     config.VM
		ports  []int
		golden string
	}{
		{
			name:   "defaults",
			vm:     config.Defaults(),
			golden: "defaults.yaml",
		},
		{
			name:   "provision and ports",
			vm:     config.VM{Base: "ubuntu-lts", CPUs: 4, Memory: "8GiB", Disk: "40GiB", Provision: "apt-get update\napt-get install -y build-essential"},
			ports:  []int{8080, 3000},
			golden: "provision-ports.yaml",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Render(tc.vm, tc.ports)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			path := filepath.Join("testdata", tc.golden)
			if update {
				if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
					t.Fatalf("writing golden file: %v", err)
				}
			}

			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading golden file (run with UPDATE_GOLDEN=1 to create it): %v", err)
			}
			if got != string(want) {
				t.Errorf("Render() does not match %s:\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
			}
		})
	}
}

func TestRenderOmitsPortForwardsWhenNoPortsDeclared(t *testing.T) {
	got, err := Render(config.Defaults(), nil)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	// With no ports declared, Lima's default forwarding must be left alone.
	if strings.Contains(got, "portForwards") {
		t.Error("Render() emitted portForwards for a cell that declares no ports")
	}
}

func TestRenderAlwaysDisablesMounts(t *testing.T) {
	got, err := Render(config.Defaults(), nil)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	// A mount would hand an agent a path back to files the host executes.
	if !strings.Contains(got, "mounts: []") {
		t.Error("Render() did not disable mounts")
	}
}
