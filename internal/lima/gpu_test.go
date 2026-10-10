package lima

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestGPUEnvIsEmptyWithoutAGPU(t *testing.T) {
	env, err := gpuEnv("")
	if err != nil {
		t.Fatalf("gpuEnv(\"\") error = %v", err)
	}
	if env != nil {
		t.Errorf("gpuEnv(\"\") = %q, want nothing", env)
	}
}

// The arguments ride on the variable Lima reads the qemu binary from, so the
// binary has to come first and the arguments after it.
func TestGPUEnvNamesQemuAndItsArguments(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("written against a Linux x86_64 host")
	}
	t.Setenv("QEMU_SYSTEM_X86_64", "")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	env, err := gpuEnv("/dev/dri/by-path/pci-0000:01:00.0-render")
	if err != nil {
		t.Fatalf("gpuEnv() error = %v", err)
	}
	if len(env) != 1 {
		t.Fatalf("gpuEnv() = %q, want one QEMU_SYSTEM_X86_64 entry", env)
	}
	fields := strings.Fields(env[0])
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "QEMU_SYSTEM_X86_64=") {
		t.Fatalf("gpuEnv() = %q, want an executable first", env)
	}
	wrapper := strings.TrimPrefix(fields[0], "QEMU_SYSTEM_X86_64=")
	info, err := os.Stat(wrapper)
	if err != nil || info.Mode()&0o100 == 0 {
		t.Fatalf("GPU wrapper %q is not executable: %v", wrapper, err)
	}
	script, err := os.ReadFile(wrapper)
	if err != nil || !strings.Contains(string(script), "'__qemu-gpu' 'qemu-system-x86_64' \"$@\"") {
		t.Fatalf("GPU wrapper %q does not forward QEMU: %v, %q", wrapper, err, script)
	}
	want := "QEMU_SYSTEM_X86_64=" + wrapper +
		" -device virtio-gpu-gl-pci,venus=on,blob=on,hostmem=4G" +
		" -display egl-headless,rendernode=/dev/dri/by-path/pci-0000:01:00.0-render"
	if env[0] != want {
		t.Errorf("gpuEnv() = %q, want [%q]", env, want)
	}
}

// A qemu someone already pointed Lima at is the one that runs, with the GPU
// added to it rather than in place of it.
func TestGPUEnvKeepsAQemuAlreadyChosen(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("written against a Linux x86_64 host")
	}
	t.Setenv("QEMU_SYSTEM_X86_64", "/opt/qemu/bin/qemu-system-x86_64 -smp sockets=1")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	env, err := gpuEnv("/dev/dri/renderD128")
	if err != nil {
		t.Fatalf("gpuEnv() error = %v", err)
	}
	if len(env) != 1 {
		t.Fatalf("gpuEnv() = %q, want one entry", env)
	}
	wrapper := strings.TrimPrefix(strings.Fields(env[0])[0], "QEMU_SYSTEM_X86_64=")
	script, err := os.ReadFile(wrapper)
	if err != nil || !strings.Contains(string(script), "'/opt/qemu/bin/qemu-system-x86_64' '-smp' 'sockets=1'") {
		t.Errorf("gpuEnv() = %q, want the existing qemu first", env)
	}
}

func TestQEMUMajorVersion(t *testing.T) {
	for _, tc := range []struct {
		output string
		want   int
	}{
		{"QEMU emulator version 11.0.5\nCopyright ...", 11},
		{"QEMU emulator version 11.1.0-rc2\n", 11},
		{"QEMU emulator version 10.2.2 (qemu-10.2.2)\n", 10},
		{"unrecognized output", 0},
	} {
		got, err := qemuMajorVersion(tc.output)
		if tc.want == 0 {
			if err == nil {
				t.Errorf("qemuMajorVersion(%q) = %d, want an error", tc.output, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("qemuMajorVersion(%q) = %d, %v; want %d", tc.output, got, err, tc.want)
		}
	}
}

func TestGPUSupportRequiresQEMU11(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("QEMU GPU mode is tested on Linux x86_64")
	}
	bin := filepath.Join(t.TempDir(), "qemu-system-x86_64")
	script := "#!/bin/sh\nprintf 'QEMU emulator version 10.2.2\\n'\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("QEMU_SYSTEM_X86_64", bin)
	if err := GPUSupport(); err == nil || !strings.Contains(err.Error(), "QEMU 11 or newer") {
		t.Errorf("GPUSupport() = %v, want the QEMU 11 requirement", err)
	}
}

func TestGPUSupportAcceptsQEMU11WithVenus(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("QEMU GPU mode is tested on Linux x86_64")
	}
	bin := filepath.Join(t.TempDir(), "qemu-system-x86_64")
	script := "#!/bin/sh\n" +
		"if [ \"$3\" = -version ]; then printf 'QEMU emulator version 11.0.5\\n'; " +
		"elif [ \"$1\" = -device ]; then printf 'venus=<bool>\\n'; " +
		"else printf 'egl-headless\\n'; fi\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("QEMU_SYSTEM_X86_64", bin)
	if err := GPUSupport(); err != nil {
		t.Errorf("GPUSupport() = %v, want QEMU 11 with Venus to pass", err)
	}
}

func TestGPUQEMUArgsMovesLimasAccelerator(t *testing.T) {
	args := []string{"-device", "virtio-gpu-gl-pci,venus=on", "-machine", "q35,accel=kvm,usb=off", "-smp", "4"}
	want := []string{"-accel", "kvm,honor-guest-pat=on", "-device", "virtio-gpu-gl-pci,venus=on", "-machine", "q35,usb=off", "-smp", "4"}
	if got := gpuQEMUArgs(args); !reflect.DeepEqual(got, want) {
		t.Errorf("gpuQEMUArgs() = %q, want %q", got, want)
	}
}

func TestGPUQEMUArgsPreservesLimasProbe(t *testing.T) {
	args := []string{"-M", "none", "-accel", "help"}
	if got := gpuQEMUArgs(args); !reflect.DeepEqual(got, args) {
		t.Errorf("gpuQEMUArgs() = %q, want Lima's probe unchanged", got)
	}
}
