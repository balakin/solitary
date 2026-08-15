package lima

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dm-balakin/solitary/internal/config"
)

var update = os.Getenv("UPDATE_GOLDEN") != ""

func TestRenderGolden(t *testing.T) {
	cases := []struct {
		name    string
		vm      config.VM
		ports   []int
		network config.Network
		golden  string
	}{
		{
			name:   "defaults",
			vm:     config.Defaults(),
			golden: "defaults.yaml",
		},
		{
			name:   "provision and ports",
			vm:     config.VM{Base: "ubuntu-lts", CPUs: 4, Memory: "8GiB", Disk: "40GiB", Provision: "apt-get update\napt-get install -y build-essential"},
			ports:  []int{8080, 3000},
			golden: "provision-ports.yaml",
		},
		{
			name:    "restricted network",
			vm:      config.Defaults(),
			network: config.Network{Allow: []string{"github.com", "api.anthropic.com", "10.1.2.0/24"}},
			golden:  "network-allow.yaml",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Render(tc.vm, tc.ports, tc.network)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			path := filepath.Join("testdata", tc.golden)
			if update {
				if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
					t.Fatalf("writing golden file: %v", err)
				}
			}

			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading golden file (run with UPDATE_GOLDEN=1 to create it): %v", err)
			}
			if got != string(want) {
				t.Errorf("Render() does not match %s:\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
			}
		})
	}
}

func TestRenderOmitsPortForwardsWhenNoPortsDeclared(t *testing.T) {
	got, err := Render(config.Defaults(), nil, config.Network{})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	// With no ports declared, Lima's default forwarding must be left alone.
	if strings.Contains(got, "portForwards") {
		t.Error("Render() emitted portForwards for a cell that declares no ports")
	}
}

func TestRenderAlwaysDisablesMounts(t *testing.T) {
	got, err := Render(config.Defaults(), nil, config.Network{})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	// A mount would hand an agent a path back to files the host executes.
	if !strings.Contains(got, "mounts: []") {
		t.Error("Render() did not disable mounts")
	}
}

// A rule with no guestIP matches only a listener on 127.0.0.1, and Lima's own
// fallback then forwards anything bound to 0.0.0.0 — which is what a dev server
// binds. An allow list that covers only one of the two lets through exactly the
// ports it was written to keep out.
func TestRenderCoversBothAddressesAServerCanBindTo(t *testing.T) {
	got, err := Render(config.Defaults(), []int{3000}, config.Network{})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	for _, want := range []string{
		// allowed, bound to either address
		"guestIP: \"0.0.0.0\"\n    guestPort: 3000",
		"- guestPort: 3000\n    hostPort: 3000",
		// everything else denied, bound to either address
		"guestIP: \"0.0.0.0\"\n    guestPortRange: [1, 65535]\n    proto: any\n    ignore: true",
		"- guestPortRange: [1, 65535]\n    proto: any\n    ignore: true",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered ports are missing a rule:\n%s\n--- rendered ---\n%s", want, got)
		}
	}

	// Nothing may widen where a forwarded port is reachable from.
	if strings.Contains(got, "hostIP:") {
		t.Error("Render() set hostIP; forwarded ports must reach this host's localhost only")
	}
}

// The firewall is the whole point of the setting, so these are the properties
// that make it one rather than a suggestion.
func TestRenderRestrictedNetwork(t *testing.T) {
	got, err := Render(config.Defaults(), nil, config.Network{
		Allow: []string{"github.com", "10.1.2.0/24"},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	for _, want := range []struct{ rule, why string }{
		{"policy drop", "anything not allowed must be denied by default"},
		{"for domain in github.com", "an allowed domain must be configured"},
		{`echo "server=/$domain/$resolver"`, "an allowed domain must resolve"},
		{`echo "nftset=/$domain/inet#solitary#allowed4"`, "resolving it must be what opens the firewall for it"},
		{`resolvers="1.1.1.1 8.8.8.8"`, "a cell that names no resolvers gets the default ones"},
		{"local=/#/", "an unlisted name must not resolve at all"},
		{"elements = { 10.1.2.0/24 }", "a configured address must be allowed directly"},
		{"ip daddr 127.0.0.1 udp dport 53 accept", "the cell may only ask its own resolver"},
		{"meta skuid dnsmasq ip daddr @upstream", "only the resolver may reach the resolvers it forwards to"},
		{"ct state established,related accept", "replies to what the host started must survive"},
	} {
		if !strings.Contains(got, want.rule) {
			t.Errorf("missing %q: %s\n--- rendered ---\n%s", want.rule, want.why, got)
		}
	}
}

// A cell that allows nothing must carry no firewall at all, rather than an
// empty one that denies everything.
func TestRenderLeavesAnUnrestrictedNetworkAlone(t *testing.T) {
	got, err := Render(config.Defaults(), nil, config.Network{})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	for _, unwanted := range []string{"nftables", "dnsmasq", "policy drop"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("a cell with no allow list carries %q", unwanted)
		}
	}
}

// A network whose names only its own resolver knows — a corporate one, a split
// horizon, a proxy that intercepts DNS — cannot be served by a public resolver.
func TestRenderHostResolver(t *testing.T) {
	got, err := Render(config.Defaults(), nil, config.Network{
		Allow:     []string{"github.com"},
		Resolvers: []string{config.HostResolver, "10.0.0.53"},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	for _, want := range []struct{ rule, why string }{
		{`resolvers="10.0.0.53"`, "an address given here is used as it is"},
		{"/run/systemd/netif/leases", "the machine's own resolver is discovered from its lease"},
		{`resolvers="$resolvers $host_dns"`, "and forwarded to alongside the configured ones"},
		{`nft add element inet solitary upstream`, "a discovered resolver has to be allowed once it is known"},
		{"elements = { 10.0.0.53 }", "a configured one is allowed from the start"},
	} {
		if !strings.Contains(got, want.rule) {
			t.Errorf("missing %q: %s", want.rule, want.why)
		}
	}

	// Discovery must fail loudly: a machine that cannot find its resolver
	// would otherwise come up resolving nothing at all.
	if !strings.Contains(got, "could not find the resolver this machine was given") {
		t.Error("a machine that cannot discover its resolver does not say so")
	}
}
