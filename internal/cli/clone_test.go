package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/balakin/solitary/internal/clone"
	"github.com/balakin/solitary/internal/config"
)

// What a cell runs as root in its VM, and as what, is the part of a definition
// a clone is most likely to be taken over by, so the review has to show all of
// it rather than a summary.
func TestDescribeStagedShowsWhatTheCellRuns(t *testing.T) {
	c := &config.Cell{
		Image:   "ubuntu:24.04",
		Command: "sleep infinity",
		User:    "cell",
		VM:      config.VM{Base: config.Base, CPUs: 2, Memory: "1GiB", Disk: "10GiB", Provision: "echo provisioned"},
		Network: config.Network{Allow: []string{"example.com"}, Resolvers: []string{"host"}},
		// Written by the author, and not what the machine will be made of.
		IgnoredBase: "ubuntu-lts",
	}

	var out bytes.Buffer
	describeStaged(&out, &clone.Staged{Cell: c, Source: clone.Source{Display: "fixture"}}, "probe")
	for _, want := range []string{"echo provisioned", "sleep infinity", "user", "resolvers", "host", "ubuntu-lts (ignored"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("describeStaged() omits %q:\n%s", want, out.String())
		}
	}
}

// A description comes from whoever wrote the definition, so it must not be able
// to repaint the screen it is being reviewed on.
func TestDescribeStagedEscapesTerminalControls(t *testing.T) {
	var out bytes.Buffer
	describeStaged(&out, &clone.Staged{
		Cell:   &config.Cell{Image: "ubuntu:24.04", Description: "review\x1b[2J\x1b[Hforged"},
		Source: clone.Source{Display: "fixture"},
	}, "probe")
	if strings.Contains(out.String(), "\x1b") {
		t.Errorf("describeStaged() wrote a raw escape sequence: %q", out.String())
	}
}
