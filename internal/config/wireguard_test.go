package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// write puts a configuration where ReadTunnel can find it.
func write(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "vpn.conf")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("writing the configuration: %v", err)
	}

	return path
}

const conf = `[Interface]
PrivateKey = 4Hn0aQ+kCVvJQ0oSQVn6z2b0Jm5xkxJcqf2vJ0kWZ1A=
Address = 10.64.0.2/32

[Peer]
PublicKey = 7Hn0aQ+kCVvJQ0oSQVn6z2b0Jm5xkxJcqf2vJ0kWZ1A=
AllowedIPs = 0.0.0.0/0
Endpoint = de-01.example.net:51820
`

func TestReadTunnel(t *testing.T) {
	tunnel, err := ReadTunnel(write(t, conf))
	if err != nil {
		t.Fatalf("ReadTunnel() error = %v", err)
	}

	if tunnel.EndpointHost != "de-01.example.net" || tunnel.EndpointPort != "51820" {
		t.Errorf("endpoint = %s:%s, want de-01.example.net:51820", tunnel.EndpointHost, tunnel.EndpointPort)
	}
	if tunnel.EndpointIsAddress() {
		t.Error("a named peer was read as an address")
	}
	if tunnel.Digest == "" {
		t.Error("the configuration has no digest, so replacing it would go unnoticed")
	}
}

func TestReadTunnelReadsAnAddressedPeer(t *testing.T) {
	tunnel, err := ReadTunnel(write(t, "[Peer]\nEndpoint=198.51.100.7:51820\n"))
	if err != nil {
		t.Fatalf("ReadTunnel() error = %v", err)
	}

	if !tunnel.EndpointIsAddress() {
		t.Errorf("peer %q was not read as an address", tunnel.EndpointHost)
	}
}

// The digest is the only thing standing between a replaced configuration and a
// machine that keeps dialling the peer it was created with.
func TestReadTunnelDigestFollowsTheContents(t *testing.T) {
	first, err := ReadTunnel(write(t, conf))
	if err != nil {
		t.Fatalf("ReadTunnel() error = %v", err)
	}
	second, err := ReadTunnel(write(t, strings.Replace(conf, "de-01", "se-04", 1)))
	if err != nil {
		t.Fatalf("ReadTunnel() error = %v", err)
	}

	if first.Digest == second.Digest {
		t.Error("two different configurations have the same digest")
	}
}

func TestReadTunnelRefusesWhatCannotWork(t *testing.T) {
	cases := []struct {
		name     string
		contents string
		want     string
	}{
		{
			// wg-quick would write this into /etc/resolv.conf, which a
			// restricted cell has made immutable.
			name:     "a DNS line",
			contents: conf + "DNS = 10.64.0.1\n",
			want:     "network.resolvers",
		},
		{
			// There would be nothing to let through, so the tunnel
			// could never be made.
			name:     "no endpoint",
			contents: "[Interface]\nAddress = 10.64.0.2/32\n",
			want:     "no Endpoint",
		},
		{
			name:     "an endpoint without a port",
			contents: "[Peer]\nEndpoint = de-01.example.net\n",
			want:     "host:port",
		},
		{
			// The peer is allowed by the same firewall as
			// everything else, which holds IPv4 addresses only.
			name:     "an IPv6 endpoint",
			contents: "[Peer]\nEndpoint = [2001:db8::1]:51820\n",
			want:     "IPv6",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadTunnel(write(t, tc.contents))
			if err == nil {
				t.Fatal("ReadTunnel() accepted a configuration that cannot work")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

// The first thing someone hits after copying a cell definition someone else
// wrote: the credential that goes with it is theirs to provide.
func TestReadTunnelExplainsAMissingConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vpn.conf")
	_, err := ReadTunnel(path)
	if err == nil {
		t.Fatal("ReadTunnel() accepted a configuration that is not there")
	}

	for _, want := range []string{path, "VPN provider"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}
