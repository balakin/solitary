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
	"net"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Cell is a single cell definition, read from cells/<name>/cell.yaml.
//
// The cell's name is the name of the directory holding the file, never a field
// inside it, so the two can never disagree.
type Cell struct {
	// Image is the container image holding the toolset, e.g.
	// ghcr.io/you/nvim-claude:latest. Exactly one of image or build is
	// required.
	Image string `yaml:"image"`

	// Build is a Containerfile to build the toolset from, as a path relative
	// to the cell's directory. Its directory is the build context, which is
	// copied into the machine and built there — the host never runs a build.
	Build string `yaml:"build"`

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

	// Network restricts what the cell can reach.
	Network Network `yaml:"network"`

	// Git is the identity commits made in this cell are attributed to.
	// Usually set once in the user-wide config rather than per cell.
	Git Git `yaml:"git"`

	// BuildPath is Build resolved against the cell's directory. It is filled
	// in by LoadCell rather than read from the file.
	BuildPath string `yaml:"-"`
}

// Tag is the image reference a built cell produces. Podman qualifies locally
// built images with localhost/.
func Tag(name string) string {
	return "localhost/solitary-" + name + ":latest"
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

// Network says what a cell is allowed to reach.
//
// An empty Allow leaves the cell's network alone: it reaches whatever the host
// reaches. Setting it turns the cell default-deny — nothing leaves except to
// what is listed, the host and the local network included.
type Network struct {
	// Resolvers are the DNS servers the cell's own resolver forwards to.
	//
	// Empty means the public resolvers in DefaultResolvers. The entry
	// "host" means the resolver the machine is given by its network, which
	// is the host's — the answer for a network whose names only its own
	// resolver knows, or where reaching a public one is blocked outright.
	// It is the one hole in VM→host isolation, and a narrow one: the
	// cell's resolver alone, on port 53.
	Resolvers []string `yaml:"resolvers,omitempty"`

	// VPN is a wg-quick configuration file, as a path relative to the cell's
	// directory. Setting it sends everything the cell reaches through that
	// tunnel: the machine brings the interface up, and the container inherits
	// it, since it runs on the machine's own network.
	//
	// The file is not part of the cell definition. It holds a private key, so
	// it is read at run time and placed into the machine directly — a cell
	// definition is meant to be readable, copied, and shared, and the
	// credential that goes with it is whoever runs it.
	VPN string `yaml:"vpn,omitempty"`

	// VPNPath is VPN resolved against the cell's directory, and Tunnel is what
	// was read out of it. Both are filled in by LoadCell rather than read from
	// the file.
	VPNPath string  `yaml:"-"`
	Tunnel  *Tunnel `yaml:"-"`

	// Allow lists domains and addresses the cell may open connections to.
	// A domain covers its subdomains, so "github.com" reaches
	// "api.github.com"; it does not cover a different domain the site
	// happens to use, so "objects.githubusercontent.com" has to be listed
	// too. An entry that parses as an IP address or CIDR block is used as
	// given.
	Allow []string `yaml:"allow,omitempty"`
}

// HostResolver is the entry that means "whatever this machine is told to use".
const HostResolver = "host"

// DefaultResolvers are used when a cell names none. They are public on purpose:
// the host is otherwise unreachable from a restricted cell, and a resolver on
// the host's network would see every name the cell looks up.
func DefaultResolvers() []string {
	return []string{"1.1.1.1", "8.8.8.8"}
}

// ResolverAddresses are the fixed addresses to forward to. It is empty when the
// cell asks only for the host's resolver, which is not known until the machine
// boots.
func (n Network) ResolverAddresses() []string {
	if len(n.Resolvers) == 0 {
		return DefaultResolvers()
	}

	var addresses []string
	for _, entry := range n.Resolvers {
		if entry != HostResolver {
			addresses = append(addresses, entry)
		}
	}

	return addresses
}

// UsesHostResolver reports whether the machine has to discover a resolver at
// boot rather than being given one here.
func (n Network) UsesHostResolver() bool {
	for _, entry := range n.Resolvers {
		if entry == HostResolver {
			return true
		}
	}

	return false
}

// HostResolverOutsideTunnel reports a combination that undoes much of what a
// tunnel is for.
//
// The host's resolver is discovered as the address the machine's own network
// gives it, which is link-local to that network — so queries to it keep taking
// the interface the tunnel replaced, by design, rather than going through it.
// The cell's traffic leaves from somewhere else, but every name it looks up is
// still read by whoever runs that resolver. It also reopens the one hole in
// VM→host isolation.
//
// It is not refused: a network whose names only its own resolver knows is a
// real thing to be on, and pairing it with a tunnel for everything else is a
// legitimate — if narrow — choice. So this is said out loud instead.
func (n Network) HostResolverOutsideTunnel() bool {
	return n.Tunnel != nil && n.UsesHostResolver()
}

// validateResolvers refuses an entry that is neither an address nor the host
// keyword, rather than rendering a machine whose resolver silently does
// nothing.
func (n Network) validateResolvers() error {
	for _, entry := range n.Resolvers {
		if entry == HostResolver || net.ParseIP(entry) != nil {
			continue
		}

		return fmt.Errorf("network.resolvers: %q is neither an IP address nor %q", entry, HostResolver)
	}

	return nil
}

// Restricted reports whether this network is default-deny.
func (n Network) Restricted() bool {
	return len(n.Allow) > 0
}

// Domains and Addresses split the allow list by what each entry is, because the
// two are enforced differently: an address goes straight into the firewall,
// while a domain is resolved by the cell's own resolver, which records what it
// resolves to. That is what keeps a rule working when a site changes its
// addresses.
func (n Network) Domains() []string {
	var domains []string
	for _, entry := range n.Allow {
		if !isAddress(entry) {
			domains = append(domains, entry)
		}
	}

	return domains
}

func (n Network) Addresses() []string {
	var addresses []string
	for _, entry := range n.Allow {
		if isAddress(entry) {
			addresses = append(addresses, entry)
		}
	}

	return addresses
}

func isAddress(entry string) bool {
	if _, _, err := net.ParseCIDR(entry); err == nil {
		return true
	}

	return net.ParseIP(entry) != nil
}

// Git is who a cell commits as.
//
// A cell has no state of its own to carry a git identity: nothing is mounted
// from the host, and anything configured by hand inside a cell is lost when it
// is rebuilt. So solitary passes the identity in as environment variables,
// which git reads ahead of any config file.
type Git struct {
	Name  string `yaml:"name,omitempty"`
	Email string `yaml:"email,omitempty"`
}

// Env renders the identity as environment variables.
//
// git keeps the author of a change separate from whoever committed it, and has
// no single setting for both, so one name here becomes both names. Whatever is
// unset is left out, so that git falls back to its own rules rather than being
// handed an empty identity.
func (g Git) Env() []string {
	var env []string
	if g.Name != "" {
		env = append(env, "GIT_AUTHOR_NAME="+g.Name, "GIT_COMMITTER_NAME="+g.Name)
	}
	if g.Email != "" {
		env = append(env, "GIT_AUTHOR_EMAIL="+g.Email, "GIT_COMMITTER_EMAIL="+g.Email)
	}
	return env
}

// UserConfig is ~/.config/solitary/config.yaml: defaults applied to every cell
// that does not override them.
type UserConfig struct {
	VM      VM      `yaml:"vm"`
	Git     Git     `yaml:"git"`
	Network Network `yaml:"network"`
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

// ResolveGit merges a cell's identity with the user-wide one, field by field,
// so a cell can change the email it commits with and keep the name.
func ResolveGit(cell, user Git) Git {
	if cell.Name == "" {
		cell.Name = user.Name
	}
	if cell.Email == "" {
		cell.Email = user.Email
	}
	return cell
}

// LoadCell reads a cell definition. The returned Cell has its vm and git blocks
// already merged with the user-wide config and the built-in defaults.
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
	switch {
	case cell.Image == "" && cell.Build == "":
		return nil, fmt.Errorf("%s: set either image or build", path)
	case cell.Image != "" && cell.Build != "":
		return nil, fmt.Errorf("%s: set image or build, not both", path)
	}
	if cell.Build != "" {
		dir, err := CellDir(name)
		if err != nil {
			return nil, err
		}
		cell.BuildPath = filepath.Join(dir, cell.Build)
	}
	if cell.Command == "" {
		cell.Command = DefaultCommand
	}

	user, err := LoadUserConfig()
	if err != nil {
		return nil, err
	}
	cell.VM = Resolve(cell.VM, user.VM, Defaults())
	cell.Git = ResolveGit(cell.Git, user.Git)
	// A cell's own allow list replaces the user-wide one rather than adding
	// to it: what a cell may reach should be readable from one place.
	if len(cell.Network.Allow) == 0 {
		cell.Network = user.Network
	}
	if err := cell.Network.validateResolvers(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if cell.Network.VPN != "" {
		dir, err := CellDir(name)
		if err != nil {
			return nil, err
		}
		cell.Network.VPNPath = filepath.Join(dir, cell.Network.VPN)
		cell.Network.Tunnel, err = ReadTunnel(cell.Network.VPNPath)
		if err != nil {
			return nil, err
		}
	}

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
