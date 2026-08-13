// Package config defines the on-disk formats solitary reads.
//
// Two files matter, both under ~/.config/solitary:
//
//	config.yaml            user-wide defaults
//	cells/<name>/cell.yaml one cell; the directory name is the cell's name
//	cells/<name>/.env      that cell's secret values, host-side only
//
// Nothing here is implemented yet; the types fix the shape of the formats.
package config

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
	Base string `yaml:"base"`

	CPUs   int    `yaml:"cpus"`
	Memory string `yaml:"memory"`
	Disk   string `yaml:"disk"`

	// Provision is a shell script run once, as root, after the built-in
	// podman setup. A value here replaces the user-wide one rather than
	// appending to it.
	Provision string `yaml:"provision"`
}

// UserConfig is ~/.config/solitary/config.yaml: defaults applied to every cell
// that does not override them.
type UserConfig struct {
	VM VM `yaml:"vm"`
}

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
// block, then the user-wide config, then Defaults.
func Resolve(cell, user, defaults VM) VM {
	panic("not implemented")
}
