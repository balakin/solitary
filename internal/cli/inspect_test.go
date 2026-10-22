package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/balakin/solitary/internal/cell"
)

func TestWriteReport(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	updates, security := 23, 7
	report := cell.Report{
		Host: cell.Host{Solitary: "0.17.0", Lima: "2.2.1", QEMU: "11.0.5", QEMUPath: "/usr/bin/qemu-system-x86_64", Kernel: "Linux 7.2.11", Nested: "Y"},
		Cell: "probe",
		Machine: &cell.Machine{
			Status:      "stopped",
			CreatedFrom: "ubuntu-lts",
			NewFrom:     "ubuntu-26.04",
			Guest: &cell.Guest{
				AsOf:           now.Add(-72 * time.Hour),
				Release:        "Ubuntu 24.04.5 LTS",
				Kernel:         "6.8.0-142-generic",
				RebootRequired: true,
				Updates:        &updates,
				Security:       &security,
				ListsRefreshed: now.Add(-74 * time.Hour),
				Unattended:     "enabled",
				DiskUsed:       3 << 30,
				DiskSize:       20 << 30,
				Packages:       map[string]string{"podman": "4.9.3", "ca-certificates": "20240203"},
			},
		},
	}

	var out bytes.Buffer
	writeReport(&out, report, now)
	got := out.String()

	for _, want := range []string{
		"qemu      11.0.5 (/usr/bin/qemu-system-x86_64)",
		"machine    stopped",
		"created   ubuntu-lts (new machines: ubuntu-26.04)",
		"system    Ubuntu 24.04.5 LTS · kernel 6.8.0-142-generic",
		"updates   23 pending, 7 security · reboot required",
		"lists     refreshed 3 days ago · unattended-upgrades enabled",
		"ca-certificates 20240203 · podman 4.9.3",
		// A stopped machine is described as it was, and says so.
		"as last seen, 3 days ago",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("writeReport() omits %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "container") {
		t.Errorf("writeReport() describes a container the report does not have:\n%s", got)
	}
}

// A live reading is not hedged, and a running container gets its section.
func TestWriteReportOfARunningCell(t *testing.T) {
	now := time.Now()
	report := cell.Report{
		Cell:      "probe",
		Machine:   &cell.Machine{Status: "running", CreatedFrom: "ubuntu-26.04", NewFrom: "ubuntu-26.04", Guest: &cell.Guest{AsOf: now, Live: true, Release: "Ubuntu 26.04.1 LTS"}},
		Container: &cell.Container{Running: true, Image: "docker.io/library/ubuntu:24.04", Release: "Ubuntu 24.04.5 LTS", Devices: "/dev/kvm"},
	}

	var out bytes.Buffer
	writeReport(&out, report, now)
	got := out.String()

	if strings.Contains(got, "as last seen") || strings.Contains(got, "new machines") {
		t.Errorf("a live reading on the current base is hedged:\n%s", got)
	}
	for _, want := range []string{"container  running", "image     docker.io/library/ubuntu:24.04", "devices   /dev/kvm"} {
		if !strings.Contains(got, want) {
			t.Errorf("writeReport() omits %q:\n%s", want, got)
		}
	}
}
