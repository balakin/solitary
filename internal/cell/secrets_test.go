package cell

import (
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/balakin/solitary/internal/config"
	"github.com/balakin/solitary/internal/secrets"
)

const labelled = `image: docker.io/library/ubuntu:24.04
secrets:
  NEEDED:
  SPARE:
    required: false
    description: only needed to push
`

// resolve is what Up does with a cell's secrets, without the machine behind it.
// Tests never have a terminal, so this is always the unattended path.
func resolve(t *testing.T, definition, env string) ([]string, error) {
	t.Helper()

	writeCell(t, definition, env)
	if secrets.CanPrompt() {
		t.Skip("a terminal is attached; this covers the unattended path")
	}

	c, err := config.LoadCell("probe")
	if err != nil {
		t.Fatalf("LoadCell() error = %v", err)
	}

	return resolveSecrets("probe", c, io.Discard)
}

// An optional secret is a secret the cell was told it might not get, so nothing
// about it should stop a start.
func TestUnsetOptionalSecretDoesNotBlockAStart(t *testing.T) {
	env, err := resolve(t, labelled, "NEEDED=\"a value\"\n")
	if err != nil {
		t.Fatalf("resolveSecrets() error = %v, want a start", err)
	}

	if want := []string{"NEEDED=a value"}; !reflect.DeepEqual(env, want) {
		t.Errorf("env = %v, want %v", env, want)
	}
}

func TestUnsetRequiredSecretBlocksAStart(t *testing.T) {
	_, err := resolve(t, labelled, "SPARE=\"a value\"\n")
	if err == nil {
		t.Fatal("resolveSecrets() = nil, want an error naming the required secret")
	}
	if !strings.Contains(err.Error(), "NEEDED") {
		t.Errorf("error = %v, want it to name NEEDED", err)
	}
	if strings.Contains(err.Error(), "SPARE") {
		t.Errorf("error = %v, want it to leave the optional secret out", err)
	}
}
