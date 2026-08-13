// Package config defines the on-disk formats solitary reads.
//
// Two files matter, both under ~/.config/solitary:
//
//	config.yaml            user-wide defaults
//	cells/<name>/cell.yaml one cell; the directory name is the cell's name
//	cells/<name>/.env      that cell's secret values, host-side only
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// Cell is a single cell definition, read from cells/<name>/cell.yaml.
//
// The cell's name is the name of the directory holding the file, never a field
// inside it, so the two can never disagree.
type Cell struct {
	// Image is the container image holding the toolset, e.g.
	// ghcr.io/you/nvim-claude:latest. Required.
	Image string `yaml:"image"`

	// Secrets lists the environment variable names this cell is allowed to
	// see. Values come from the cell's .env file and are passed to the
	// container at run time. Names absent from this list are never passed,
	// even when present in .env.
	Secrets []string `yaml:"secrets"`

	// Command is the shell command the container runs. It must not exit: the
	// container lives as long as this process does, and shells opened with
	// up or shell are separate from it. Defaults to DefaultCommand.
	Command string `yaml:"command"`

	// Ports restricts which guest ports reach the host. When empty, Lima's
	// default forwarding applies and every port the container listens on is
	// reachable on host localhost. When set, only these are forwarded.
	Ports []int `yaml:"ports"`

	// VM overrides the machine the container runs in.
	VM VM `yaml:"vm"`
}

// VM describes the Lima machine backing a cell. Every field is optional: values
// fall back to the user-wide config, then to the defaults built into the binary.
type VM struct {
	// Base names the Lima image template, e.g. ubuntu-lts.
	Base string `yaml:"base,omitempty"`

	CPUs   int    `yaml:"cpus,omitempty"`
	Memory string `yaml:"memory,omitempty"`
	Disk   string `yaml:"disk,omitempty"`

	// Provision is a shell script run once, as root, after the built-in
	// podman setup. A value here replaces the user-wide one rather than
	// appending to it.
	Provision string `yaml:"provision,omitempty"`
}

// UserConfig is ~/.config/solitary/config.yaml: defaults applied to every cell
// that does not override them.
type UserConfig struct {
	VM VM `yaml:"vm"`
}

// DefaultCommand keeps a container alive without assuming anything about the
// image. Cells that run a server instead set command: in cell.yaml.
const DefaultCommand = "sleep infinity"

// Defaults returns the settings compiled into the binary, used when neither the
// cell nor the user-wide config specifies a value.
func Defaults() VM {
	return VM{
		Base:   "ubuntu-lts",
		CPUs:   2,
		Memory: "4GiB",
		Disk:   "20GiB",
	}
}

// Resolve merges the three layers, most specific first: the cell's own vm
// block, then the user-wide config, then defaults.
func Resolve(cell, user, defaults VM) VM {
	str := func(layers ...string) string {
		for _, v := range layers {
			if v != "" {
				return v
			}
		}
		return ""
	}
	num := func(layers ...int) int {
		for _, v := range layers {
			if v != 0 {
				return v
			}
		}
		return 0
	}

	return VM{
		Base:      str(cell.Base, user.Base, defaults.Base),
		CPUs:      num(cell.CPUs, user.CPUs, defaults.CPUs),
		Memory:    str(cell.Memory, user.Memory, defaults.Memory),
		Disk:      str(cell.Disk, user.Disk, defaults.Disk),
		Provision: str(cell.Provision, user.Provision, defaults.Provision),
	}
}

// LoadCell reads a cell definition. The returned Cell has its vm block already
// merged with the user-wide config and the built-in defaults.
func LoadCell(name string) (*Cell, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}

	path, err := CellFile(name)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("no cell named %q: run 'solitary init %s' first", name, name)
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var cell Cell
	if err := yaml.Unmarshal(data, &cell); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if cell.Image == "" {
		return nil, fmt.Errorf("%s: image is required", path)
	}
	if cell.Command == "" {
		cell.Command = DefaultCommand
	}

	user, err := LoadUserConfig()
	if err != nil {
		return nil, err
	}
	cell.VM = Resolve(cell.VM, user.VM, Defaults())

	return &cell, nil
}

// LoadUserConfig reads the user-wide defaults. A missing file is not an error.
func LoadUserConfig() (*UserConfig, error) {
	path, err := UserConfigFile()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &UserConfig{}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var cfg UserConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &cfg, nil
}

// ListCells returns the names of every cell that has a definition on disk, in
// alphabetical order. A cell exists as soon as it is defined, whether or not a
// machine was ever created for it.
func ListCells() ([]string, error) {
	dir, err := CellsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path, err := CellFile(e.Name())
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(path); err != nil {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	return names, nil
}
