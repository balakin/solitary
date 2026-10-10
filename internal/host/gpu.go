package host

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RenderNode is one of the host's GPUs, as a machine would render with it.
type RenderNode struct {
	// Path is the node to open: its /dev/dri/by-path name where the host has
	// one, since renderD128 and renderD129 are numbered in the order the
	// drivers loaded and can swap between boots.
	Path string

	// Driver is the kernel driver behind it, which is what the choice
	// between several is made on.
	Driver string
}

// driverRank orders the drivers a machine could render with, best first.
//
// A discrete GPU before an integrated one: on a laptop the integrated one is
// driving the desktop already, and the discrete one is the one sitting idle.
// nouveau after the vendor's own drivers because what it renders with is
// slower and less complete. A driver not named here — a virtual one, a new
// one — is still used when it is all there is.
var driverRank = map[string]int{
	"nvidia":  0,
	"amdgpu":  0,
	"radeon":  1,
	"nouveau": 2,
	"i915":    3,
	"xe":      3,
}

// unknownRank is where a driver missing from driverRank sorts.
const unknownRank = 4

// RenderNodes lists the host's render nodes, best to render with first.
func RenderNodes() ([]RenderNode, error) {
	return renderNodesIn("/sys/class/drm", "/dev/dri")
}

// renderNodesIn is RenderNodes against a sysfs class directory and a /dev/dri
// of the caller's choosing, which is what makes it testable.
func renderNodesIn(class, dri string) ([]RenderNode, error) {
	entries, err := os.ReadDir(class)
	if err != nil {
		if os.IsNotExist(err) {
			// Not Linux, or no DRM at all: no GPU, rather than a failure.
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", class, err)
	}

	// by-path names, keyed by the node each one points at.
	stable := map[string]string{}
	if links, err := os.ReadDir(filepath.Join(dri, "by-path")); err == nil {
		for _, link := range links {
			if !strings.HasSuffix(link.Name(), "-render") {
				continue
			}
			path := filepath.Join(dri, "by-path", link.Name())
			if target, err := os.Readlink(path); err == nil {
				stable[filepath.Base(target)] = path
			}
		}
	}

	var nodes []RenderNode
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "renderD") {
			continue
		}
		driver := ""
		if target, err := os.Readlink(filepath.Join(class, name, "device", "driver")); err == nil {
			driver = filepath.Base(target)
		}
		path := filepath.Join(dri, name)
		if s, ok := stable[name]; ok {
			path = s
		}
		nodes = append(nodes, RenderNode{Path: path, Driver: driver})
	}

	sort.SliceStable(nodes, func(i, j int) bool {
		ri, rj := rank(nodes[i].Driver), rank(nodes[j].Driver)
		if ri != rj {
			return ri < rj
		}
		return nodes[i].Path < nodes[j].Path
	})

	return nodes, nil
}

func rank(driver string) int {
	if r, ok := driverRank[driver]; ok {
		return r
	}
	return unknownRank
}

// OpenRenderNode reports whether this user can open a render node, which is
// all qemu needs of it. A node the user cannot open fails the machine's start
// with nothing said anywhere but Lima's log.
func OpenRenderNode(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	return f.Close()
}

// PickRenderNode chooses the render node a machine renders with: the one
// named, when one is, or else the best of the host's that this user can open.
//
// A named node is used as given or not at all. A host that names a GPU meant
// that one, and quietly rendering on another would hide that the name is
// wrong.
func PickRenderNode(named string) (RenderNode, error) {
	if named != "" {
		if err := OpenRenderNode(named); err != nil {
			return RenderNode{}, fmt.Errorf("the gpu in config.yaml cannot be opened: %w", err)
		}
		return RenderNode{Path: named}, nil
	}

	nodes, err := RenderNodes()
	if err != nil {
		return RenderNode{}, err
	}
	for _, node := range nodes {
		if OpenRenderNode(node.Path) == nil {
			return node, nil
		}
	}
	if len(nodes) == 0 {
		return RenderNode{}, fmt.Errorf("this host has no render node")
	}
	return RenderNode{}, fmt.Errorf("this host has %d render nodes and this user can open none of them", len(nodes))
}
