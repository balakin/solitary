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
	"regexp"
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

	// Secrets are the environment variable names this cell is allowed to
	// see. Values come from the cell's .env file and are passed to the
	// container at run time. Names absent from this mapping are never
	// passed, even when present in .env.
	Secrets Secrets `yaml:"secrets"`

	// User is the user work happens as inside the cell: the one a shell
	// lands on, and the one the cell's home belongs to.
	//
	// Cells run as root inside the container, which is where the layering
	// puts the boundary — the machine is what isolates a cell, and root
	// inside a rootless container is an unprivileged user in the machine. So
	// this is only needed by an image that serves a login of its own: an
	// sshd, an editor server. Naming that user here is what makes the home
	// theirs; without it the home belongs to the container's root and
	// everything they do in it fails on a permission.
	User string `yaml:"user"`

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

// Secret is one environment variable a cell may receive.
type Secret struct {
	// Name is the mapping key the secret was declared under, so it is not a
	// field of its own — the two could otherwise disagree.
	Name string `yaml:"-"`

	// Required says the cell cannot start without a value. It defaults to
	// true: a name is declared because the cell needs it, and the optional
	// one is the exception that has to say so.
	Required bool `yaml:"required"`

	// Description says what the secret is for. It is shown when asking for a
	// value and when listing what a cell wants, and is never stored beside
	// the value itself.
	Description string `yaml:"description"`
}

// Secrets are the secrets a cell declares, in the order it declared them:
// asking for values follows that order, and so does every listing.
type Secrets []Secret

// userName is what a cell's user has to look like: a name or a numeric id, and
// nothing that would mean something else to a shell.
var userName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)

// envName is what a name has to look like to survive being passed to podman as
// an environment variable.
var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// UnmarshalYAML reads the mapping of name to options.
//
// A mapping rather than the list of names this used to be, so that a name can
// carry the two labels above; the list form is refused by name rather than left
// to the yaml package, since a definition written against the old shape is the
// one thing certain to arrive here.
func (s *Secrets) UnmarshalYAML(node *yaml.Node) error {
	if node.Tag == "!!null" {
		return nil
	}
	if node.Kind == yaml.SequenceNode {
		hint := "NAME"
		if len(node.Content) > 0 && node.Content[0].Kind == yaml.ScalarNode {
			hint = node.Content[0].Value
		}
		return fmt.Errorf("secrets: is a mapping of name to options now, not a list; write %q on its own line rather than %q", hint+":", "- "+hint)
	}
	if node.Kind != yaml.MappingNode {
		return errors.New("secrets: must be a mapping of name to options")
	}

	seen := map[string]bool{}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if key.Kind != yaml.ScalarNode {
			return errors.New("secrets: every key must be an environment variable name")
		}
		if !envName.MatchString(key.Value) {
			return fmt.Errorf("secrets: %q is not an environment variable name", key.Value)
		}
		if seen[key.Value] {
			return fmt.Errorf("secrets: %s is declared twice", key.Value)
		}
		seen[key.Value] = true

		// Required starts true so that a document saying nothing about it
		// gets the default, and only an explicit required: false turns it
		// off: yaml leaves fields the document does not mention alone.
		entry := Secret{Name: key.Value, Required: true}
		if value.Tag != "!!null" {
			if err := value.Decode(&entry); err != nil {
				return fmt.Errorf("secrets: %s: %w", key.Value, err)
			}
			entry.Name = key.Value
		}
		*s = append(*s, entry)
	}

	return nil
}

// Names returns every declared name, in declaration order.
func (s Secrets) Names() []string {
	names := make([]string, 0, len(s))
	for _, secret := range s {
		names = append(names, secret.Name)
	}
	return names
}

// RequiredNames returns the names a cell cannot start without.
func (s Secrets) RequiredNames() []string {
	var names []string
	for _, secret := range s {
		if secret.Required {
			names = append(names, secret.Name)
		}
	}
	return names
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
	// too. An entry that parses as an IPv4 address or CIDR block is used as
	// given; an IPv6 one is refused, since a cell's machine has no IPv6
	// route to reach it through.
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
		if entry == HostResolver {
			continue
		}
		if net.ParseIP(entry) == nil {
			return fmt.Errorf("network.resolvers: %q is neither an IP address nor %q", entry, HostResolver)
		}
		if ipv6(entry) {
			return fmt.Errorf("network.resolvers: %q is an IPv6 address, which a cell cannot reach: %s", entry, noIPv6)
		}
	}

	return nil
}

// validateAllow refuses an IPv6 entry. A domain is left alone: what it resolves
// to is the resolver's business, and it answers with A records only.
func (n Network) validateAllow() error {
	for _, entry := range n.Allow {
		if ipv6(entry) {
			return fmt.Errorf("network.allow: %q is an IPv6 address, which a cell cannot reach: %s", entry, noIPv6)
		}
	}

	return nil
}

// noIPv6 is why an IPv6 entry is refused rather than rendered, said the same
// way wherever one turns up.
const noIPv6 = "a cell's machine has no IPv6 route, and its firewall holds IPv4 addresses only"

// ipv6 reports whether an entry is an IPv6 address or an IPv6 CIDR block.
//
// Rendering one into the firewall is not a narrower policy but no policy: the
// sets it would go into are ipv4_addr, so nft refuses the whole ruleset and the
// machine comes up allowing everything. The resolver already filters AAAA
// answers for the same reason, so nothing a cell can reach is being kept from
// it here.
func ipv6(entry string) bool {
	ip, _, err := net.ParseCIDR(entry)
	if err != nil {
		ip = net.ParseIP(entry)
	}

	return ip != nil && ip.To4() == nil
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

// ResolveNetwork merges a cell's network block with the user-wide one, field by
// field, the way the vm block is merged.
//
// A cell's own allow list replaces the user-wide one rather than adding to it:
// what a cell may reach should be readable from one place. But that is a
// statement about the allow list alone. Taking the whole block whenever a cell
// listed nothing would mean a cell that sets only resolvers or only a tunnel
// silently loses them — and, worse, quietly gains whatever else the user-wide
// file happens to say, so that one cell's policy shows up in another.
func ResolveNetwork(cell, user Network) Network {
	if len(cell.Allow) == 0 {
		cell.Allow = user.Allow
	}
	if len(cell.Resolvers) == 0 {
		cell.Resolvers = user.Resolvers
	}
	if cell.VPN == "" {
		cell.VPN = user.VPN
	}

	return cell
}

// LoadCell reads a cell definition. The returned Cell has its vm, git and
// network blocks already merged with the user-wide config and the built-in
// defaults.
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

	dir, err := CellDir(name)
	if err != nil {
		return nil, err
	}

	return ParseCell(data, dir)
}

// ParseCell reads a definition that has already been read off disk, resolving
// the paths inside it against dir, the directory the file came from.
//
// It is separate from LoadCell so that a definition can be checked before it
// becomes a cell — a clone validates a staged copy this way, under the same
// rules, rather than a second set of them.
func ParseCell(data []byte, dir string) (*Cell, error) {
	return parseCell(data, dir, true)
}

// CheckCell parses a definition without reading the WireGuard configuration a
// network.vpn may name.
//
// A definition that is being shared rather than run is expected to arrive
// without that file: it holds a private key, so it belongs to whoever runs the
// cell and never to the cell. Everything else is checked exactly as it would be
// on the way up.
func CheckCell(data []byte, dir string) (*Cell, error) {
	return parseCell(data, dir, false)
}

func parseCell(data []byte, dir string, tunnel bool) (*Cell, error) {
	path := filepath.Join(dir, "cell.yaml")

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
		cell.BuildPath = filepath.Join(dir, cell.Build)
	}
	if cell.Command == "" {
		cell.Command = DefaultCommand
	}
	if cell.User != "" && !userName.MatchString(cell.User) {
		return nil, fmt.Errorf("%s: user: %q is not a user name or id", path, cell.User)
	}

	user, err := LoadUserConfig()
	if err != nil {
		return nil, err
	}
	cell.VM = Resolve(cell.VM, user.VM, Defaults())
	cell.Git = ResolveGit(cell.Git, user.Git)
	cell.Network = ResolveNetwork(cell.Network, user.Network)
	if err := cell.Network.validateResolvers(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := cell.Network.validateAllow(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if cell.Network.VPN != "" {
		cell.Network.VPNPath = filepath.Join(dir, cell.Network.VPN)
		if tunnel {
			cell.Network.Tunnel, err = ReadTunnel(cell.Network.VPNPath)
			if err != nil {
				return nil, err
			}
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
