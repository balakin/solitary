package host

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// fakeDRM lays out a sysfs class directory and a /dev/dri with one render node
// per driver given, numbered from renderD128 in that order, and a by-path link
// for each.
func fakeDRM(t *testing.T, drivers ...string) (class, dri string) {
	t.Helper()
	root := t.TempDir()
	class = filepath.Join(root, "class")
	dri = filepath.Join(root, "dri")
	if err := os.MkdirAll(filepath.Join(dri, "by-path"), 0o755); err != nil {
		t.Fatal(err)
	}

	for i, driver := range drivers {
		node := fmt.Sprintf("renderD%d", 128+i)
		device := filepath.Join(class, node, "device")
		if err := os.MkdirAll(device, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("../../../bus/pci/drivers/"+driver, filepath.Join(device, "driver")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		link := filepath.Join(dri, "by-path", fmt.Sprintf("pci-0000:%02d:00.0-render", i))
		if err := os.Symlink("../"+node, link); err != nil {
			t.Fatal(err)
		}
	}
	// A card node, which is not a render node and must not be listed.
	if err := os.MkdirAll(filepath.Join(class, "card0", "device"), 0o755); err != nil {
		t.Fatal(err)
	}

	return class, dri
}

// A laptop: the integrated GPU loaded first and drives the desktop, and the
// discrete one is what a machine should render with.
func TestRenderNodesPutADiscreteGPUFirst(t *testing.T) {
	class, dri := fakeDRM(t, "i915", "nvidia")

	nodes, err := renderNodesIn(class, dri)
	if err != nil {
		t.Fatalf("renderNodesIn() error = %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("renderNodesIn() = %v, want two nodes", nodes)
	}
	if nodes[0].Driver != "nvidia" || nodes[1].Driver != "i915" {
		t.Errorf("renderNodesIn() order = %s, %s; want nvidia, i915", nodes[0].Driver, nodes[1].Driver)
	}
	if want := filepath.Join(dri, "by-path", "pci-0000:01:00.0-render"); nodes[0].Path != want {
		t.Errorf("renderNodesIn()[0].Path = %q, want the by-path name %q", nodes[0].Path, want)
	}
}

// A driver nobody ranked is still a GPU, and the only one there is is the one
// to use.
func TestRenderNodesKeepAnUnknownDriver(t *testing.T) {
	class, dri := fakeDRM(t, "somethingnew")

	nodes, err := renderNodesIn(class, dri)
	if err != nil {
		t.Fatalf("renderNodesIn() error = %v", err)
	}
	if len(nodes) != 1 || nodes[0].Driver != "somethingnew" {
		t.Errorf("renderNodesIn() = %v, want the one node", nodes)
	}
}

// No DRM at all — macOS, or a host without a GPU — is an empty answer, not an
// error.
func TestRenderNodesWithoutDRM(t *testing.T) {
	nodes, err := renderNodesIn(filepath.Join(t.TempDir(), "missing"), t.TempDir())
	if err != nil {
		t.Fatalf("renderNodesIn() error = %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("renderNodesIn() = %v, want none", nodes)
	}
}
