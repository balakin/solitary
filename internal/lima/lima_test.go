package lima

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/balakin/solitary/internal/config"
)

var update = os.Getenv("UPDATE_GOLDEN") != ""

// goldenCell is the cell name every rendering in these tests is for. Fixed
// rather than taken from the case, so the goldens differ only in the setting
// each case is about.
const goldenCell = "probe"

// The machine carries the cell's name, and the script reads it back from the
// parameter rather than having it written in: the parameter is what makes the
// machine nameable once its definition is gone, and Lima refuses one that
// nothing reads.
func TestRenderNamesTheCell(t *testing.T) {
	got, err := Render("probe", config.Defaults(), nil, config.Network{})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(got, ParamCell+`: "probe"`) {
		t.Errorf("Render() does not record the cell's name:\n%s", got)
	}
	if !strings.Contains(got, "$PARAM_"+ParamCell) {
		t.Errorf("Render() does not read the parameter back in the guest:\n%s", got)
	}
	if !strings.Contains(got, GuestCellFile) {
		t.Errorf("Render() does not write the name into the guest:\n%s", got)
	}
}

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
		{
			name: "tunnelled network",
			vm:   config.Defaults(),
			network: config.Network{
				Allow:  []string{"github.com"},
				VPN:    "./vpn.conf",
				Tunnel: &config.Tunnel{EndpointHost: "de-01.example.net", EndpointPort: "51820", Digest: "8f1e"},
			},
			golden: "network-vpn.yaml",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Render(goldenCell, tc.vm, tc.ports, tc.network)
			if err != nil {
				t.Fatalf("Render(goldenCell) error = %v", err)
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
				t.Errorf("Render(goldenCell) does not match %s:\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
			}
		})
	}
}

func TestRenderOmitsPortForwardsWhenNoPortsDeclared(t *testing.T) {
	got, err := Render(goldenCell, config.Defaults(), nil, config.Network{})
	if err != nil {
		t.Fatalf("Render(goldenCell) error = %v", err)
	}
	// With no ports declared, Lima's default forwarding must be left alone.
	if strings.Contains(got, "portForwards") {
		t.Error("Render(goldenCell) emitted portForwards for a cell that declares no ports")
	}
}

func TestRenderAlwaysDisablesMounts(t *testing.T) {
	got, err := Render(goldenCell, config.Defaults(), nil, config.Network{})
	if err != nil {
		t.Fatalf("Render(goldenCell) error = %v", err)
	}
	// A mount would hand an agent a path back to files the host executes.
	if !strings.Contains(got, "mounts: []") {
		t.Error("Render(goldenCell) did not disable mounts")
	}
}

// A rule with no guestIP matches only a listener on 127.0.0.1, and Lima's own
// fallback then forwards anything bound to 0.0.0.0 — which is what a dev server
// binds. An allow list that covers only one of the two lets through exactly the
// ports it was written to keep out.
func TestRenderCoversBothAddressesAServerCanBindTo(t *testing.T) {
	got, err := Render(goldenCell, config.Defaults(), []int{3000}, config.Network{})
	if err != nil {
		t.Fatalf("Render(goldenCell) error = %v", err)
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
		t.Error("Render(goldenCell) set hostIP; forwarded ports must reach this host's localhost only")
	}
}

// The firewall is the whole point of the setting, so these are the properties
// that make it one rather than a suggestion.
func TestRenderRestrictedNetwork(t *testing.T) {
	got, err := Render(goldenCell, config.Defaults(), nil, config.Network{
		Allow: []string{"github.com", "10.1.2.0/24"},
	})
	if err != nil {
		t.Fatalf("Render(goldenCell) error = %v", err)
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
	got, err := Render(goldenCell, config.Defaults(), nil, config.Network{})
	if err != nil {
		t.Fatalf("Render(goldenCell) error = %v", err)
	}

	for _, unwanted := range []struct{ rule, why string }{
		{"policy drop", "nothing may be denied by default"},
		{"/etc/dnsmasq.d/solitary.conf <<", "no resolver configuration may be written"},
		{"/etc/nftables.conf <<", "no firewall configuration may be written"},
		{"chattr +i", "resolv.conf may not be frozen"},
	} {
		if strings.Contains(got, unwanted.rule) {
			t.Errorf("a cell with no allow list carries %q: %s", unwanted.rule, unwanted.why)
		}
	}
}

// A machine keeps its disk when the cell that describes it changes, so removing
// a setting has to reach the guest as surely as adding one did. What the
// restriction wrote, an unrestricted definition takes back out.
func TestRenderReleasesAMachineThatWasRestricted(t *testing.T) {
	got, err := Render(goldenCell, config.Defaults(), nil, config.Network{})
	if err != nil {
		t.Fatalf("Render(goldenCell) error = %v", err)
	}

	for _, want := range []struct{ rule, why string }{
		{"nft delete table inet solitary", "the firewall the allow list installed must go"},
		{"rm -f /etc/dnsmasq.d/solitary.conf", "and the resolver that filled it"},
		{"chattr -i /etc/resolv.conf", "resolv.conf was made immutable and cannot be replaced until it is not"},
		{"systemctl enable --now systemd-resolved", "the machine must resolve again through what the image ships"},
		{"if [ ! -e /etc/dnsmasq.d/solitary.conf ] && ! nft list table inet solitary", "a machine that was never restricted must do none of this"},
	} {
		if !strings.Contains(got, want.rule) {
			t.Errorf("missing %q: %s\n--- rendered ---\n%s", want.rule, want.why, got)
		}
	}
}

// The same for a tunnel: a configuration nothing describes any more must not
// keep coming up at boot, and its private key must not stay on the disk.
func TestRenderReleasesAMachineThatWasTunnelled(t *testing.T) {
	got, err := Render(goldenCell, config.Defaults(), nil, config.Network{Allow: []string{"github.com"}})
	if err != nil {
		t.Fatalf("Render(goldenCell) error = %v", err)
	}

	for _, want := range []struct{ rule, why string }{
		{"systemctl disable --now wg-quick@vpn0", "an interface the cell no longer asks for must not come up"},
		{"rm -f /etc/wireguard/vpn0.conf", "and the key it came up from must not be left behind"},
	} {
		if !strings.Contains(got, want.rule) {
			t.Errorf("missing %q: %s\n--- rendered ---\n%s", want.rule, want.why, got)
		}
	}
}

// A tunnel is only worth having if it cannot be gone around, so these are the
// properties that decide whether a cell leaks when it drops.
func TestRenderTunnel(t *testing.T) {
	named, err := Render(goldenCell, config.Defaults(), nil, config.Network{
		Allow:  []string{"github.com"},
		VPN:    "./vpn.conf",
		Tunnel: &config.Tunnel{EndpointHost: "de-01.example.net", EndpointPort: "51820", Digest: "8f1e"},
	})
	if err != nil {
		t.Fatalf("Render(goldenCell) error = %v", err)
	}

	for _, want := range []struct{ rule, why string }{
		{`oifname "vpn0" ip daddr @allowed4 accept`, "an allowed name must be reachable through the tunnel only"},
		{`oifname "vpn0" ip daddr @static4 accept`, "and so must an allowed address"},
		{"ip daddr @vpn4 udp dport 51820 accept", "the peer must be reachable without the tunnel, since it is what makes it"},
		{"nftset=/de-01.example.net/inet#solitary#vpn4", "a peer named rather than addressed has to resolve into its own set"},
		{"delete table inet solitary", "reloading must not flush the table wg-quick installs"},
		{"wireguard-tools", "the machine needs the tools to bring the tunnel up"},
		{"After=dnsmasq.service nftables.service", "the tunnel comes up after what resolves and permits its peer"},
		{"8f1e", "replacing the configuration must count as a change to the cell"},
	} {
		if !strings.Contains(named, want.rule) {
			t.Errorf("missing %q: %s\n--- rendered ---\n%s", want.rule, want.why, named)
		}
	}

	// The plain allow rules are what a tunnelled cell must not carry: they
	// would let the same traffic out of the interface the tunnel replaced.
	for _, unwanted := range []string{"\n          ip daddr @allowed4 accept", "\n          ip daddr @static4 accept"} {
		if strings.Contains(named, unwanted) {
			t.Errorf("a tunnelled cell can reach %q outside its tunnel", strings.TrimSpace(unwanted))
		}
	}

	addressed, err := Render(goldenCell, config.Defaults(), nil, config.Network{
		Allow:  []string{"github.com"},
		VPN:    "./vpn.conf",
		Tunnel: &config.Tunnel{EndpointHost: "198.51.100.7", EndpointPort: "51820", Digest: "8f1e"},
	})
	if err != nil {
		t.Fatalf("Render(goldenCell) error = %v", err)
	}
	if !strings.Contains(addressed, "set vpn4 { type ipv4_addr;\n          elements = { 198.51.100.7 }") {
		t.Errorf("a peer given as an address is not allowed from the start:\n%s", addressed)
	}
	if strings.Contains(addressed, "nftset=/198.51.100.7/") {
		t.Error("a peer given as an address must not be handed to the resolver")
	}
}

// The configuration holds a private key, and a machine definition is kept on
// disk, handed to the guest, and meant to be shared.
func TestRenderNeverCarriesTheTunnelConfiguration(t *testing.T) {
	got, err := Render(goldenCell, config.Defaults(), nil, config.Network{
		Allow:  []string{"github.com"},
		VPN:    "./vpn.conf",
		Tunnel: &config.Tunnel{EndpointHost: "de-01.example.net", EndpointPort: "51820", Digest: "8f1e"},
	})
	if err != nil {
		t.Fatalf("Render(goldenCell) error = %v", err)
	}

	for _, unwanted := range []string{"PrivateKey", "[Interface]", "[Peer]"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("the rendered definition contains %q", unwanted)
		}
	}
}

// A network whose names only its own resolver knows — a corporate one, a split
// horizon, a proxy that intercepts DNS — cannot be served by a public resolver.
func TestRenderHostResolver(t *testing.T) {
	got, err := Render(goldenCell, config.Defaults(), nil, config.Network{
		Allow:     []string{"github.com"},
		Resolvers: []string{config.HostResolver, "10.0.0.53"},
	})
	if err != nil {
		t.Fatalf("Render(goldenCell) error = %v", err)
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
