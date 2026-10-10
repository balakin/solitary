package lima

import (
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

	env, err := gpuEnv("/dev/dri/by-path/pci-0000:01:00.0-render")
	if err != nil {
		t.Fatalf("gpuEnv() error = %v", err)
	}
	want := "QEMU_SYSTEM_X86_64=qemu-system-x86_64" +
		" -device virtio-gpu-gl-pci,venus=on,blob=on,hostmem=4G" +
		" -display egl-headless,rendernode=/dev/dri/by-path/pci-0000:01:00.0-render"
	if len(env) != 1 || env[0] != want {
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

	env, err := gpuEnv("/dev/dri/renderD128")
	if err != nil {
		t.Fatalf("gpuEnv() error = %v", err)
	}
	if len(env) != 1 || !strings.HasPrefix(env[0], "QEMU_SYSTEM_X86_64=/opt/qemu/bin/qemu-system-x86_64 -smp sockets=1 -device ") {
		t.Errorf("gpuEnv() = %q, want the existing qemu first", env)
	}
}
