package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestResolvePrefersMostSpecificLayer(t *testing.T) {
	defaults := VM{Base: "ubuntu-lts", CPUs: 2, Memory: "4GiB", Disk: "20GiB"}
	user := VM{CPUs: 4, Memory: "8GiB"}
	cell := VM{Memory: "16GiB"}

	got := Resolve(cell, user, defaults)

	want := VM{
		Base:   "ubuntu-lts", // only the defaults set it
		CPUs:   4,            // user config wins over defaults
		Memory: "16GiB",      // the cell wins over both
		Disk:   "20GiB",
	}
	if got != want {
		t.Errorf("Resolve() = %+v, want %+v", got, want)
	}
}

func TestResolveTreatsZeroAsUnset(t *testing.T) {
	got := Resolve(VM{}, VM{}, Defaults())
	if got != Defaults() {
		t.Errorf("Resolve() with empty layers = %+v, want the defaults %+v", got, Defaults())
	}
}

func TestResolveProvisionReplacesRatherThanAppends(t *testing.T) {
	user := VM{Provision: "apt-get install -y user-tool"}
	cell := VM{Provision: "apt-get install -y cell-tool"}

	if got := Resolve(cell, user, VM{}).Provision; got != cell.Provision {
		t.Errorf("Provision = %q, want the cell's script verbatim", got)
	}
}

func TestValidateName(t *testing.T) {
	valid := []string{"a", "n1", "nvim-claude", "cell-2"}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{"", "-leading", "trailing-", "Upper", "has space", "under_score", "dots.here"}
	for _, name := range invalid {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) = nil, want an error", name)
		}
	}
}

func TestGitEnvSetsBothAuthorAndCommitter(t *testing.T) {
	got := Git{Name: "Ada Lovelace", Email: "ada@example.com"}.Env()
	want := []string{
		"GIT_AUTHOR_NAME=Ada Lovelace",
		"GIT_COMMITTER_NAME=Ada Lovelace",
		"GIT_AUTHOR_EMAIL=ada@example.com",
		"GIT_COMMITTER_EMAIL=ada@example.com",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Env() = %q, want %q", got, want)
	}
}

// An identity that is not configured must not be passed in at all: an empty
// GIT_AUTHOR_NAME is worse than none, since git cannot fall back past it.
func TestGitEnvOmitsWhatIsUnset(t *testing.T) {
	if got := (Git{}).Env(); len(got) != 0 {
		t.Errorf("Env() = %q, want nothing for an unconfigured identity", got)
	}
	if got := (Git{Email: "ada@example.com"}).Env(); len(got) != 2 {
		t.Errorf("Env() = %q, want only the email pair", got)
	}
}

func TestResolveGitFallsBackFieldByField(t *testing.T) {
	user := Git{Name: "Ada Lovelace", Email: "ada@example.com"}

	if got := ResolveGit(Git{}, user); got != user {
		t.Errorf("ResolveGit() = %+v, want the user-wide identity %+v", got, user)
	}

	got := ResolveGit(Git{Email: "work@example.com"}, user)
	want := Git{Name: "Ada Lovelace", Email: "work@example.com"}
	if got != want {
		t.Errorf("ResolveGit() = %+v, want %+v", got, want)
	}
}

// The user-wide network block is a set of defaults, not a policy that replaces
// a cell's own. A cell that sets one field of it keeps every other field it
// set, and a cell that sets none inherits the defaults.
func TestResolveNetworkFallsBackFieldByField(t *testing.T) {
	user := Network{
		Allow:     []string{"github.com"},
		Resolvers: []string{"9.9.9.9"},
		VPN:       "./user.conf",
	}

	cases := []struct {
		name string
		cell Network
		want Network
	}{
		{
			name: "a cell that says nothing takes the defaults",
			cell: Network{},
			want: user,
		},
		{
			name: "its own allow list replaces the user-wide one",
			cell: Network{Allow: []string{"gitlab.com"}},
			want: Network{Allow: []string{"gitlab.com"}, Resolvers: []string{"9.9.9.9"}, VPN: "./user.conf"},
		},
		{
			// The bug this exists for: a cell that names a resolver and no
			// allow list used to lose the resolver and take the whole
			// user-wide block, so one cell's policy turned up in another.
			name: "resolvers survive an empty allow list",
			cell: Network{Resolvers: []string{"1.1.1.1"}},
			want: Network{Allow: []string{"github.com"}, Resolvers: []string{"1.1.1.1"}, VPN: "./user.conf"},
		},
		{
			name: "and so does a tunnel",
			cell: Network{VPN: "./vpn.conf"},
			want: Network{Allow: []string{"github.com"}, Resolvers: []string{"9.9.9.9"}, VPN: "./vpn.conf"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveNetwork(tc.cell, user); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ResolveNetwork() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestNetworkSplitsDomainsFromAddresses(t *testing.T) {
	n := Network{Allow: []string{"github.com", "10.1.2.0/24", "api.anthropic.com", "192.0.2.7"}}

	if got, want := n.Domains(), []string{"github.com", "api.anthropic.com"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Domains() = %q, want %q", got, want)
	}
	if got, want := n.Addresses(), []string{"10.1.2.0/24", "192.0.2.7"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Addresses() = %q, want %q", got, want)
	}
}

// An empty allow list means the cell's network is left alone. Anything else
// would silently cut off every cell that predates this setting.
func TestNetworkIsUnrestrictedUntilSomethingIsAllowed(t *testing.T) {
	if (Network{}).Restricted() {
		t.Error("a cell with no allow list is restricted")
	}
	if !(Network{Allow: []string{"github.com"}}).Restricted() {
		t.Error("a cell with an allow list is not restricted")
	}
}

func TestNetworkResolvers(t *testing.T) {
	if got := (Network{}).ResolverAddresses(); !reflect.DeepEqual(got, DefaultResolvers()) {
		t.Errorf("ResolverAddresses() = %q, want the defaults", got)
	}

	n := Network{Resolvers: []string{HostResolver, "10.0.0.53"}}
	if got, want := n.ResolverAddresses(), []string{"10.0.0.53"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ResolverAddresses() = %q, want %q", got, want)
	}
	if !n.UsesHostResolver() {
		t.Error("UsesHostResolver() = false for a cell that asks for it")
	}
	if (Network{Resolvers: []string{"10.0.0.53"}}).UsesHostResolver() {
		t.Error("UsesHostResolver() = true for a cell that named only addresses")
	}
}

// The host's resolver is reached over the interface the tunnel replaced, so a
// cell paired this way hides where its traffic comes from while still telling
// the network it is on every name it looks up.
func TestHostResolverOutsideTunnel(t *testing.T) {
	tunnel := &Tunnel{EndpointHost: "de-01.example.net", EndpointPort: "51820"}

	cases := []struct {
		name    string
		network Network
		want    bool
	}{
		{"host resolver behind a tunnel", Network{Tunnel: tunnel, Resolvers: []string{HostResolver}}, true},
		{"named alongside an address", Network{Tunnel: tunnel, Resolvers: []string{HostResolver, "1.1.1.1"}}, true},
		{"addresses only", Network{Tunnel: tunnel, Resolvers: []string{"1.1.1.1"}}, false},
		{"the default resolvers", Network{Tunnel: tunnel}, false},
		{"no tunnel to be outside of", Network{Resolvers: []string{HostResolver}}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.network.HostResolverOutsideTunnel(); got != tc.want {
				t.Errorf("HostResolverOutsideTunnel() = %v, want %v", got, tc.want)
			}
		})
	}
}

// A typo here would render a machine whose resolver forwards nowhere, which
// looks like every name being unreachable.
func TestNetworkRejectsAResolverThatIsNeither(t *testing.T) {
	if err := (Network{Resolvers: []string{"dns.corp.example"}}).validateResolvers(); err == nil {
		t.Error("a hostname was accepted as a resolver")
	}
	if err := (Network{Resolvers: []string{HostResolver, "10.0.0.53"}}).validateResolvers(); err != nil {
		t.Errorf("validateResolvers() error = %v", err)
	}
}

// An IPv6 entry rendered into the firewall is not a narrower policy but no
// policy: the sets are ipv4_addr, so nft refuses the whole ruleset and the
// machine comes up allowing everything. Both lists are checked before it can.
func TestNetworkRefusesIPv6(t *testing.T) {
	cases := []struct {
		name    string
		network Network
		err     func(Network) error
	}{
		{"an address in the allow list", Network{Allow: []string{"2606:4700:4700::1111"}}, Network.validateAllow},
		{"a CIDR block in the allow list", Network{Allow: []string{"2606:4700::/32"}}, Network.validateAllow},
		{"a resolver", Network{Resolvers: []string{"2606:4700:4700::1111"}}, Network.validateResolvers},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.err(tc.network)
			if err == nil {
				t.Fatal("an IPv6 entry was accepted")
			}
			if !strings.Contains(err.Error(), "IPv6") {
				t.Errorf("error %q does not say what was wrong", err)
			}
		})
	}
}

// The refusal is of IPv6 and nothing else: a domain is left to the resolver,
// which answers with A records only, and IPv4 is what the firewall holds.
func TestNetworkAllowsWhatACellCanReach(t *testing.T) {
	network := Network{
		Allow:     []string{"github.com", "198.51.100.7", "10.1.2.0/24"},
		Resolvers: []string{HostResolver, "10.0.0.53"},
	}
	if err := network.validateAllow(); err != nil {
		t.Errorf("validateAllow() error = %v", err)
	}
	if err := network.validateResolvers(); err != nil {
		t.Errorf("validateResolvers() error = %v", err)
	}
}
