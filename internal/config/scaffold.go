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

# What this cell may reach. Leave it out and the cell reaches whatever the host
# reaches. Set it and nothing leaves except to what is listed — the host and the
# local network included. A domain covers its subdomains; a site's other domains
# have to be listed themselves, as does the registry an image is pulled from.
#
# network:
#   resolvers:        # optional; defaults to 1.1.1.1 and 8.8.8.8.
#     - host          # use the machine's own resolver — corporate DNS, VPNs
#   allow:
#     - github.com
#     - api.anthropic.com
#     - registry.npmjs.org
#
# Set network.vpn as well and everything above leaves through that tunnel
# instead, giving the cell its own exit address. Point it at the .conf your VPN
# provider gives you, saved beside this file and left out of anything you
# publish: it holds a private key, which is yours rather than the cell's.
# Nothing else changes — the allow list is enforced the same way, except that
# with the tunnel down nothing leaves at all.
#
#   vpn: ./vpn.conf

# Who this cell commits as. Set it once in ~/.config/solitary/config.yaml and
# every cell uses it; set it here only to differ from that.
#
# git:
#   name: Ada Lovelace
#   email: ada@example.com

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

// Applied is what a cell's machine was last given.
//
// Definition covers everything the machine reads when it boots, which is what
// decides whether a stopped machine has to be handed a new definition.
// Provision is the vm.provision script alone, held separately because it is the
// one setting a machine cannot be handed: a script that has already run has
// already changed the disk, and a definition without it does not undo that. So
// what it is worth saying about the two differs, and telling them apart needs
// them recorded apart.
type Applied struct {
	Definition string
	Provision  string
}

// NewApplied summarises a rendering for the record.
func NewApplied(rendered, provision string) Applied {
	return Applied{Definition: Digest(rendered), Provision: Digest(provision)}
}

// Recorded reports whether anything is known about the machine at all. A cell
// that was never created has no record, and neither has one created before
// solitary kept one.
func (a Applied) Recorded() bool {
	return a.Definition != ""
}

// ProvisionChanged reports whether vm.provision differs from the script the
// machine was given. A record from a version that did not keep the script's
// digest says nothing about it, which is not the same as saying it matches.
func (a Applied) ProvisionChanged(provision string) bool {
	return a.Provision != "" && a.Provision != Digest(provision)
}

// WriteApplied records what a cell's machine was given.
func WriteApplied(name string, record Applied) error {
	path, err := AppliedFile(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	content := fmt.Sprintf("definition %s\nprovision %s\n", record.Definition, record.Provision)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// ReadApplied returns what a cell's machine was given, or the zero value when
// the cell has never been created.
//
// A record written before this file had fields is one bare digest of the
// definition, and is read as one: the machine it describes is still the machine
// it describes, and what is not in the file is simply not known.
func ReadApplied(name string) (Applied, error) {
	path, err := AppliedFile(name)
	if err != nil {
		return Applied{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Applied{}, nil
		}
		return Applied{}, fmt.Errorf("reading %s: %w", path, err)
	}

	var record Applied
	for _, line := range strings.Split(string(data), "\n") {
		field, value, named := strings.Cut(strings.TrimSpace(line), " ")
		switch {
		case field == "definition":
			record.Definition = strings.TrimSpace(value)
		case field == "provision":
			record.Provision = strings.TrimSpace(value)
		case !named && field != "":
			// The older format: the definition's digest and nothing else.
			record.Definition = field
		}
	}

	return record, nil
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
