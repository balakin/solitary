package cell

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The outbox is written inside the cell and read out here, so a name in it is
// input from the least trusted place there is. This is the rule that stops one
// being used as a path.
func TestValidName(t *testing.T) {
	cases := []struct {
		name string
		ok   bool
	}{
		{"report.pdf", true},
		{".hidden", true},
		{"a name with spaces.txt", true},
		{"", false},
		{".", false},
		{"..", false},
		{"../escape", false},
		{"/etc/passwd", false},
		{"sub/dir", false},
		{`windows\path`, false},
		{"-rf", false},
		{"two\nlines", false},
		{"report\x1b[2J.txt", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validName(tc.name)
			if tc.ok && err != nil {
				t.Errorf("validName(%q) = %v, want it accepted", tc.name, err)
			}
			if !tc.ok && err == nil {
				t.Errorf("validName(%q) accepted a name that is not one", tc.name)
			}
		})
	}
}

func TestSelection(t *testing.T) {
	published := []Artifact{
		{Name: "report.pdf", Size: 10},
		{Name: "-rf", Size: 20, Problem: `"-rf" starts with a dash`},
		{Name: "dist.tar", Size: 30},
	}

	t.Run("everything fetchable by default", func(t *testing.T) {
		wanted, refused, err := selection(published, nil)
		if err != nil {
			t.Fatalf("selection() error = %v", err)
		}
		if len(wanted) != 2 || wanted[0].Name != "report.pdf" || wanted[1].Name != "dist.tar" {
			t.Errorf("wanted = %v", wanted)
		}
		// One badly named file must not hold up the rest, and must not
		// disappear either.
		if len(refused) != 1 || refused[0].Name != "-rf" {
			t.Errorf("refused = %v", refused)
		}
	})

	t.Run("by name", func(t *testing.T) {
		wanted, _, err := selection(published, []string{"dist.tar"})
		if err != nil {
			t.Fatalf("selection() error = %v", err)
		}
		if len(wanted) != 1 || wanted[0].Name != "dist.tar" {
			t.Errorf("wanted = %v", wanted)
		}
	})

	t.Run("a name that is not there", func(t *testing.T) {
		_, _, err := selection(published, []string{"nope"})
		if err == nil {
			t.Fatal("selection() quietly fetched less than was asked for")
		}
		if !strings.Contains(err.Error(), "--list") {
			t.Errorf("error %q does not say how to find out what is there", err)
		}
	})

	t.Run("asking for one that cannot come out", func(t *testing.T) {
		wanted, refused, err := selection(published, []string{"-rf"})
		if err != nil {
			t.Fatalf("selection() error = %v", err)
		}
		if len(wanted) != 0 || len(refused) != 1 {
			t.Errorf("wanted = %v, refused = %v", wanted, refused)
		}
	})
}

func TestSize(t *testing.T) {
	for _, tc := range []struct {
		n    int64
		want string
	}{
		{12, "12 B"},
		{188352, "183.9 KiB"},
		{5 * 1024 * 1024, "5.0 MiB"},
	} {
		if got := Size(tc.n); got != tc.want {
			t.Errorf("Size(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// fakeMachine stands in for limactl with one running machine, solitary-probe,
// whose guest filesystem is a directory on the host: shell runs the command it
// is given there, and copy copies. FAKE_SWAP_SOURCE and FAKE_SWAP_TARGET make
// the guest replace a file with a symlink mid-transfer, the race a hostile cell
// would run. It returns the guest user's home and the cell's home inside it.
func fakeMachine(t *testing.T) (string, string) {
	t.Helper()

	root := t.TempDir()
	vmHome := filepath.Join(root, "vm-user")
	home := filepath.Join(vmHome, "cell")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/bash
set -eu
case "$1" in
list) printf '%s\n' '{"name":"solitary-probe","status":"Running"}' ;;
shell)
  shift 3
  if [ "$1" = sh ] && [ "${3:-}" = 'echo $HOME' ]; then
    printf '%s\n' "$FAKE_VMHOME"
  elif [ "$1" = python3 ]; then
    if [ "$4" = read ]; then
      if [ -n "${FAKE_SWAP_SOURCE:-}" ]; then
        rm -f -- "$5/$6/$7"
        ln -s -- "$FAKE_SWAP_SOURCE" "$5/$6/$7"
      fi
      if [ -n "${FAKE_SWAP_TARGET:-}" ]; then
        ln -s -- "$FAKE_SWAP_TARGET" "$FAKE_SWAP_DEST"
      fi
    fi
    "$@"
  elif [ "$1" = podman ]; then
    exit 0
  else
    "$@"
  fi ;;
copy)
  src=$2; dst=$3
  case "$src" in solitary-probe:*) src=${src#*:};; esac
  case "$dst" in solitary-probe:*) dst=${dst#*:};; esac
  if [ -n "${FAKE_SWAP_SOURCE:-}" ]; then
    rm -f -- "$src"
    ln -s -- "$FAKE_SWAP_SOURCE" "$src"
  fi
  if [ -n "${FAKE_SWAP_TARGET:-}" ]; then
    ln -s -- "$FAKE_SWAP_TARGET" "$dst"
  fi
  cp -- "$src" "$dst" ;;
*) exit 2 ;;
esac
`
	if err := os.WriteFile(filepath.Join(root, "limactl"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_VMHOME", vmHome)

	return vmHome, home
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

// unchanged fails the test if the file at path no longer reads "unchanged":
// something wrote through a symlink the cell planted.
func unchanged(t *testing.T, path string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "unchanged" {
		t.Errorf("%s was overwritten through a symlink the cell controls", path)
	}
}

// Every path in the guest is the cell's to rearrange, so a write the host makes
// there must not follow a symlink the cell put in its way.
func TestInstallArtifactsDoesNotFollowAGuestSymlink(t *testing.T) {
	vmHome, home := fakeMachine(t)
	target := filepath.Join(vmHome, "outside")
	writeFile(t, target, "unchanged")
	if err := os.MkdirAll(filepath.Join(home, toolDir), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(home, toolDir, toolName)); err != nil {
		t.Fatal(err)
	}

	_ = installArtifacts("solitary-probe")
	unchanged(t, target)
}

func TestSendDoesNotFollowAGuestSymlink(t *testing.T) {
	vmHome, home := fakeMachine(t)
	target := filepath.Join(vmHome, "outside")
	writeFile(t, target, "unchanged")
	if err := os.MkdirAll(filepath.Join(home, inboxDir), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(home, inboxDir, "input")); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "input")
	writeFile(t, source, "sent")

	_ = Send("probe", []string{source}, io.Discard)
	unchanged(t, target)
}

// A file listed in the outbox can be swapped for a symlink between the listing
// and the read; fetch has to read what was published, not what it points at.
func TestFetchDoesNotFollowAReplacedSource(t *testing.T) {
	vmHome, home := fakeMachine(t)
	writeFile(t, filepath.Join(home, outboxDir, "report"), "published")
	private := filepath.Join(vmHome, "private")
	writeFile(t, private, "private")
	t.Setenv("FAKE_SWAP_SOURCE", private)

	into := t.TempDir()
	_ = Fetch("probe", nil, into, false, io.Discard)
	if data, _ := os.ReadFile(filepath.Join(into, "report")); string(data) == "private" {
		t.Error("Fetch() followed a symlink swapped in after the listing")
	}
}

// A symlink that appears in the destination directory during a fetch must not
// turn the write into a write somewhere else on the host.
func TestFetchDoesNotWriteThroughARacedDestination(t *testing.T) {
	_, home := fakeMachine(t)
	writeFile(t, filepath.Join(home, outboxDir, "report"), "published")
	target := filepath.Join(t.TempDir(), "outside")
	writeFile(t, target, "unchanged")
	into := t.TempDir()
	t.Setenv("FAKE_SWAP_TARGET", target)
	t.Setenv("FAKE_SWAP_DEST", filepath.Join(into, "report"))

	_ = Fetch("probe", nil, into, false, io.Discard)
	unchanged(t, target)
}
