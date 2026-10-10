package lima

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// gpuHostMem is how much of the machine's address space is set aside for
// mapping what the host's GPU allocates. It is a window rather than memory:
// nothing is taken from the host until a mapping is made, and none of it comes
// out of vm.memory or the /dev/shm that backs it. 4GiB holds the textures and
// buffers a rendered UI or a browser makes.
const gpuHostMem = "4G"

// GPUArgs are the qemu arguments that give a machine a virtual GPU rendered by
// the host's render node.
//
// virtio-gpu-gl with Venus and blobs is the device that carries both OpenGL
// (virgl) and Vulkan (Venus); blob resources are what Venus maps its memory
// with, and they need the guest's RAM to be shared memory — which Lima already
// arranges, since virtiofs needs the same thing.
//
// egl-headless is the display that gives qemu an OpenGL context without a
// window, and rendernode is where that context comes from. It replaces the
// -display none Lima would add: Lima keeps the first -display it sees.
func GPUArgs(render string) []string {
	return []string{
		"-device", "virtio-gpu-gl-pci,venus=on,blob=on,hostmem=" + gpuHostMem,
		"-display", "egl-headless,rendernode=" + render,
	}
}

// gpuEnv is the environment limactl start needs for a machine with a GPU, or
// nil for one without.
//
// Lima has no setting for a device of this kind, and no way to add qemu
// arguments of any kind — except the variable it reads the qemu binary from,
// which it splits like a shell command line and passes the rest of along. Lima
// calls that a debugging aid and says so in a warning at every start; it is
// also the one way in that does not mean patching Lima, so it is what this
// uses until Lima has a field for it.
//
// It reaches qemu because limactl start forks the process that runs qemu, and
// that process inherits this environment. It has to be given at every start:
// Lima keeps no record of it, and a machine started without it boots with no
// GPU at all.
func gpuEnv(render string) ([]string, error) {
	if render == "" {
		return nil, nil
	}
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("vm.gpu needs a Linux host: on %s Lima does not run machines with qemu", runtime.GOOS)
	}

	arch, ok := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
	if !ok {
		return nil, fmt.Errorf("vm.gpu is not supported on %s", runtime.GOARCH)
	}
	key := "QEMU_SYSTEM_" + strings.ToUpper(arch)

	// Someone pointing Lima at a qemu of their own keeps it, with these
	// arguments after whatever they gave.
	qemu := os.Getenv(key)
	if qemu == "" {
		qemu = "qemu-system-" + arch
	}

	// No quoting: render is checked against config.gpuPath when the cell is
	// read, so it holds nothing Lima's splitting would take apart.
	return []string{key + "=" + strings.Join(append([]string{qemu}, GPUArgs(render)...), " ")}, nil
}
