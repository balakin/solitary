package lima

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{name: "as limactl prints it", out: "limactl version 2.0.1\n", want: "2.0.1"},
		{name: "with a build line after it", out: "limactl version 2.1.4\ngit commit: abc1234\n", want: "2.1.4"},
		{name: "two components", out: "limactl version 2.0\n", want: "2.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVersion([]byte(tt.out))
			if err != nil {
				t.Fatalf("parseVersion(%q) error = %v", tt.out, err)
			}
			if got != tt.want {
				t.Errorf("parseVersion(%q) = %q, want %q", tt.out, got, tt.want)
			}
		})
	}

	if _, err := parseVersion([]byte("limactl: command not found\n")); err == nil {
		t.Error("parseVersion() error = nil for output carrying no version")
	}
}

func TestHome(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("LIMA_HOME", "")

	got, err := Home()
	if err != nil {
		t.Fatalf("Home() error = %v", err)
	}
	if want := filepath.Join(root, ".lima"); got != want {
		t.Errorf("Home() = %q, want %q", got, want)
	}

	moved := filepath.Join(root, "elsewhere")
	t.Setenv("LIMA_HOME", moved)

	if got, err := Home(); err != nil || got != moved {
		t.Errorf("Home() = (%q, %v), want %q: LIMA_HOME moves it", got, err, moved)
	}
}

func TestVersionAgainstRealLimactl(t *testing.T) {
	if _, err := Binary(); err != nil {
		t.Skip("limactl not installed")
	}

	got, err := Version()
	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}
	if got == "" {
		t.Error("Version() = \"\", want the installed version")
	}
}

func TestBinaryNotInstalled(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	if _, err := Binary(); !errors.Is(err, ErrNotInstalled) {
		t.Errorf("Binary() error = %v, want ErrNotInstalled with nothing on PATH", err)
	}
}

func TestMinVersionIsWhatTheDocsSay(t *testing.T) {
	// The installation page states the floor; a check that disagrees with it
	// would fail installs the docs call supported.
	data, err := os.ReadFile(filepath.Join("..", "..", "website", "content", "docs", "installation.mdx"))
	if err != nil {
		t.Skip("installation docs not available")
	}
	if !strings.Contains(string(data), "Lima](https://lima-vm.io) "+MinVersion) {
		t.Errorf("installation.mdx does not state Lima %s as the minimum", MinVersion)
	}
}
