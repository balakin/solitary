package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// scaffold is written by 'solitary init'. It is a commented file rather than
// marshalled YAML so that a new cell explains its own options.
const scaffold = `# The container image holding this cell's tools.
image: docker.io/library/ubuntu:24.04

# Or build the image instead, from a Containerfile beside this file. Its
# directory is the build context, copied into the machine and built there, so
# nothing in it ever runs on the host. Set either image or build, not both.
#
# build: ./Containerfile

# Environment variables this cell is allowed to see. Values are read from the
# .env file next to this one and passed into the container at run time. Names
# not listed here are never passed, even if .env defines them.
#
# secrets:
#   - CLAUDE_API_KEY
#   - GITHUB_TOKEN

# Guest ports reachable from the host. Leave this out and every port the cell
# listens on reaches host localhost; set it and only these do.
#
# ports:
#   - 8080

# Overrides for the machine this cell runs in. Anything omitted falls back to
# ~/.config/solitary/config.yaml, then to the defaults built into solitary
# (ubuntu-lts, 2 CPUs, 4GiB memory, 20GiB disk).
#
# vm:
#   cpus: 4
#   memory: 8GiB
#   provision: |
#     apt-get install -y build-essential
`

// InitCell writes a new cell definition. It refuses to overwrite an existing
// one unless force is set, since by then the file is likely hand-edited.
func InitCell(name string, force bool) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}

	dir, err := CellDir(name)
	if err != nil {
		return "", err
	}
	path, err := CellFile(name)
	if err != nil {
		return "", err
	}

	if !force {
		if _, err := os.Stat(path); err == nil {
			return "", fmt.Errorf("cell %q already exists at %s (use --force to overwrite)", name, path)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("checking %s: %w", path, err)
		}
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating %s: %w", dir, err)
	}
	if err := os.WriteFile(path, []byte(scaffold), 0o600); err != nil {
		return "", fmt.Errorf("writing %s: %w", path, err)
	}

	return path, nil
}

// Digest summarises a rendered Lima definition. Only equality matters, so a
// digest is recorded rather than the definition itself: there is nothing in it
// for anyone to edit, and the definition it describes is regenerated on demand.
func Digest(rendered string) string {
	sum := sha256.Sum256([]byte(rendered))
	return hex.EncodeToString(sum[:])
}

// WriteApplied records what a cell's machine was created from.
func WriteApplied(name, rendered string) error {
	path, err := AppliedFile(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(Digest(rendered)+"\n"), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// ReadApplied returns the digest a cell's machine was created from, or an empty
// string when the cell has never been created.
func ReadApplied(name string) (string, error) {
	path, err := AppliedFile(name)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("reading %s: %w", path, err)
	}

	return strings.TrimSpace(string(data)), nil
}

// RemoveApplied forgets what a cell's machine was created from.
func RemoveApplied(name string) error {
	path, err := AppliedFile(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("removing %s: %w", path, err)
	}

	return nil
}
