package cell

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/balakin/solitary/internal/config"
	"github.com/balakin/solitary/internal/podman"
)

// A container records the command it was started with, as it records its image
// and environment, so a definition whose command changed gets a new container
// rather than one still running the old process. The fake limactl reports a
// running container that matches in everything but the command, and leaves a
// marker if anything replaces it.
func TestEnsureContainerReplacesOneRunningAnotherCommand(t *testing.T) {
	isolate(t)

	root := t.TempDir()
	marker := filepath.Join(root, "replaced")
	key, err := config.EnvKey()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_REPLACED", marker)
	t.Setenv("FAKE_VMHOME", root)
	t.Setenv("FAKE_ENV_DIGEST", podman.EnvDigest(nil, key))
	t.Setenv("FAKE_COMMAND_DIGEST", podman.CommandDigest("sleep infinity"))
	script := `#!/bin/bash
set -eu
shift 3
if [ "$1" = python3 ]; then cat >/dev/null; : > "$FAKE_REPLACED"; exit 0; fi
if [ "$1 $2" = 'sh -c' ]; then printf '%s\n' "$FAKE_VMHOME"; exit 0; fi
case "$1 $2 $3" in
"podman image exists") exit 0;;
"podman container inspect") printf 'running\tubuntu:24.04\t%s\t\t\t1GiB\t%s\n' "$FAKE_ENV_DIGEST" "$FAKE_COMMAND_DIGEST";;
"podman run "*) : > "$FAKE_REPLACED";;
*) exit 0;;
esac
`
	if err := os.WriteFile(filepath.Join(root, "limactl"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))

	c := &config.Cell{Image: "ubuntu:24.04", Command: "echo changed; sleep infinity", VM: config.VM{Memory: "1GiB"}}
	if err := ensureContainer("probe", "solitary-probe", c, nil, false, io.Discard); err != nil {
		t.Fatalf("ensureContainer() error = %v", err)
	}
	if _, err := os.Stat(marker); os.IsNotExist(err) {
		t.Error("ensureContainer() kept a container running the previous command")
	}
}
