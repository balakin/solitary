package lima

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// gpuHostMem is how much of the machine's address space is set aside for
// mapping what the host's GPU allocates. It is a window rather than memory:
// nothing is taken from the host until a mapping is made, and none of it comes
// out of vm.memory or the /dev/shm that backs it. 4GiB holds the textures and
// buffers a rendered UI or a browser makes.
const gpuHostMem = "4G"

// GPUQEMULaunch is the internal entry point Lima uses to start QEMU for a GPU VM.
const GPUQEMULaunch = "__qemu-gpu"

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

// qemuCommand is the variable Lima reads the qemu binary from on this host,
// and what is in it: the user's own value when there is one, or the binary
// Lima would otherwise look up on PATH.
func qemuCommand() (key, command string, err error) {
	if runtime.GOOS != "linux" {
		return "", "", fmt.Errorf("a GPU needs a Linux host: on %s Lima does not run machines with qemu", runtime.GOOS)
	}
	arch, ok := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
	if !ok {
		return "", "", fmt.Errorf("a GPU is not supported on %s", runtime.GOARCH)
	}

	key = "QEMU_SYSTEM_" + strings.ToUpper(arch)
	// Someone pointing Lima at a qemu of their own keeps it, with the GPU's
	// arguments after whatever they gave.
	command = os.Getenv(key)
	if command == "" {
		command = "qemu-system-" + arch
	}
	return key, command, nil
}

// GPUSupport reports why this host's qemu cannot give a machine a GPU, or nil
// when it can.
//
// Venus and egl-headless are build options of QEMU rather than anything a
// machine definition can ask for. An older QEMU also lacks the KVM guest PAT
// option Venus needs on Intel hosts. Ask before starting so a cell that wanted
// a GPU can start without one instead of dying in Lima's log.
func GPUSupport() error {
	_, command, err := qemuCommand()
	if err != nil {
		return err
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return fmt.Errorf("QEMU_SYSTEM_X86_64 does not name a QEMU binary")
	}
	bin := fields[0]
	version, err := exec.Command(bin, "-device", "virtio-gpu-gl-pci,venus=on", "-version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("checking %s version for GPU support: %w", bin, err)
	}
	major, err := qemuMajorVersion(string(version))
	if err != nil {
		return fmt.Errorf("checking %s version for GPU support: %w", bin, err)
	}
	if major < 11 {
		return fmt.Errorf("%s is QEMU %d; a GPU needs QEMU 11 or newer", bin, major)
	}

	device, err := exec.Command(bin, "-device", "virtio-gpu-gl-pci,help").CombinedOutput()
	if err != nil || !strings.Contains(string(device), "venus=") {
		return fmt.Errorf("%s has no virtio-gpu-gl-pci with Venus; it needs a qemu built against a virglrenderer that has it", bin)
	}
	display, err := exec.Command(bin, "-display", "help").CombinedOutput()
	if err != nil || !strings.Contains(string(display), "egl-headless") {
		return fmt.Errorf("%s has no egl-headless display; it needs a qemu built with OpenGL", bin)
	}

	return nil
}

func qemuMajorVersion(output string) (int, error) {
	const prefix = "QEMU emulator version "
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		version := strings.Fields(strings.TrimPrefix(line, prefix))
		if len(version) == 0 {
			break
		}
		major, err := strconv.Atoi(strings.SplitN(version[0], ".", 2)[0])
		if err == nil {
			return major, nil
		}
		break
	}
	return 0, fmt.Errorf("could not read QEMU version from %q", strings.TrimSpace(output))
}

// ExecGPUQEMU runs the selected qemu after moving Lima's accelerator setting
// to -accel, where QEMU exposes honor-guest-pat. It replaces this process so
// Lima still manages and observes qemu as its direct child.
func ExecGPUQEMU(command []string) error {
	if len(command) == 0 {
		return fmt.Errorf("missing QEMU command for GPU")
	}
	bin, err := exec.LookPath(command[0])
	if err != nil {
		return fmt.Errorf("finding %s for GPU: %w", command[0], err)
	}
	args := append([]string{command[0]}, gpuQEMUArgs(command[1:])...)
	if err := syscall.Exec(bin, args, os.Environ()); err != nil {
		return fmt.Errorf("starting %s for GPU: %w", command[0], err)
	}
	return nil
}

func gpuQEMUArgs(args []string) []string {
	// Lima probes the binary named by QEMU_SYSTEM_X86_64 without passing the
	// extra arguments from that variable. Keep those probes as ordinary QEMU.
	venus := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "virtio-gpu-gl-pci,") && strings.Contains(arg, "venus=on") {
			venus = true
			break
		}
	}
	if !venus {
		return args
	}

	out := []string{"-accel", "kvm,honor-guest-pat=on"}
	for i, arg := range args {
		if i > 0 && args[i-1] == "-machine" {
			parts := strings.Split(arg, ",")
			kept := parts[:0]
			for _, part := range parts {
				if !strings.HasPrefix(part, "accel=") {
					kept = append(kept, part)
				}
			}
			arg = strings.Join(kept, ",")
		}
		out = append(out, arg)
	}
	return out
}

// gpuQEMUWrapper gives Lima a real executable to probe. Lima calls only the
// first word of QEMU_SYSTEM_X86_64 with -M none -accel help, dropping all the
// words after it. A direct "solitary __qemu-gpu ..." command therefore fails
// its probe before the VM can start. The wrapper preserves the selected QEMU
// command for both probes and the eventual VM launch.
func gpuQEMUWrapper(command string) (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("finding solitary for GPU launch: %w", err)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("finding cache for GPU launcher: %w", err)
	}
	dir := filepath.Join(cache, "solitary", "qemu")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating GPU launcher directory: %w", err)
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "", fmt.Errorf("QEMU command is empty")
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	parts := []string{quote(self), quote(GPUQEMULaunch)}
	for _, field := range fields {
		parts = append(parts, quote(field))
	}
	script := "#!/bin/sh\nexec " + strings.Join(parts, " ") + " \"$@\"\n"
	sum := sha256.Sum256([]byte(self + "\x00" + command))
	path := filepath.Join(dir, fmt.Sprintf("qemu-gpu-%x", sum[:8]))
	if strings.ContainsAny(path, " \t\n") {
		return "", fmt.Errorf("GPU launcher path %q has whitespace Lima cannot pass to QEMU", path)
	}
	temp, err := os.CreateTemp(dir, ".qemu-gpu-*")
	if err != nil {
		return "", fmt.Errorf("creating GPU launcher: %w", err)
	}
	defer os.Remove(temp.Name())
	if _, err := temp.WriteString(script); err != nil {
		temp.Close()
		return "", fmt.Errorf("writing GPU launcher: %w", err)
	}
	if err := temp.Chmod(0o700); err != nil {
		temp.Close()
		return "", fmt.Errorf("making GPU launcher executable: %w", err)
	}
	if err := temp.Close(); err != nil {
		return "", fmt.Errorf("closing GPU launcher: %w", err)
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return "", fmt.Errorf("installing GPU launcher: %w", err)
	}
	return path, nil
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
// GPU at all. On x86-64, solitary is the executable Lima starts so it can move
// Lima's accelerator setting to -accel and enable guest PAT before execing QEMU.
func gpuEnv(render string) ([]string, error) {
	if render == "" {
		return nil, nil
	}
	key, command, err := qemuCommand()
	if err != nil {
		return nil, err
	}
	if runtime.GOARCH == "amd64" {
		command, err = gpuQEMUWrapper(command)
		if err != nil {
			return nil, err
		}
	}

	// No quoting: render is checked against config.ValidGPU before it gets
	// here, so it holds nothing Lima's splitting would take apart.
	return []string{key + "=" + strings.Join(append([]string{command}, GPUArgs(render)...), " ")}, nil
}
