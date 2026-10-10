package cell

import (
	"reflect"
	"testing"
)

func TestParseVenusDevices(t *testing.T) {
	output := `GPU0:
	deviceName = Virtio-GPU Venus (NVIDIA GeForce RTX 4050 Laptop GPU)
GPU1:
	deviceName = Virtio-GPU Venus (Intel(R) Graphics (RPL-P))
GPU2:
	deviceName = Virtio-GPU Venus (llvmpipe (LLVM 22.1.8, 256 bits))
GPU3:
	deviceName = llvmpipe (LLVM 20.1.2, 256 bits)
`
	want := []string{
		"NVIDIA GeForce RTX 4050 Laptop GPU",
		"Intel(R) Graphics (RPL-P)",
		"llvmpipe (LLVM 22.1.8, 256 bits)",
	}
	if got := parseVenusDevices(output); !reflect.DeepEqual(got, want) {
		t.Errorf("parseVenusDevices() = %q, want %q", got, want)
	}
}
