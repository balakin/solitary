package cell

import (
	"fmt"
	"strings"

	"github.com/balakin/solitary/internal/config"
	"github.com/balakin/solitary/internal/lima"
	"github.com/balakin/solitary/internal/podman"
)

// VenusDevices lists the Vulkan devices Venus exposes inside a running cell.
// It is a live answer: the host's render nodes alone do not say which Vulkan
// drivers the cell's image actually has available.
func VenusDevices(name string) ([]string, error) {
	out, err := lima.Exec(config.Instance(name), "podman", "exec", podman.Container, "vulkaninfo", "--summary")
	if err != nil {
		return nil, fmt.Errorf("reading Venus devices in %s: %w", name, err)
	}
	return parseVenusDevices(string(out)), nil
}

func parseVenusDevices(output string) []string {
	const prefix = "Virtio-GPU Venus ("
	var devices []string
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || strings.TrimSpace(key) != "deviceName" {
			continue
		}
		name := strings.TrimSpace(value)
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ")") {
			devices = append(devices, strings.TrimSuffix(strings.TrimPrefix(name, prefix), ")"))
		}
	}
	return devices
}
