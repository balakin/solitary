package config

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"strings"
)

// VPNInterface is the interface a cell's tunnel comes up on, and VPNConfigFile
// is where the machine keeps the configuration it comes up from. wg-quick takes
// the interface name from the file name, so the two are one decision.
const (
	VPNInterface  = "vpn0"
	VPNConfigFile = "/etc/wireguard/" + VPNInterface + ".conf"
)

// Tunnel is what solitary has to know about a WireGuard configuration. It is
// deliberately not the configuration itself: the file holds a private key, so
// only these three things — none of them secret — leave the host.
type Tunnel struct {
	// EndpointHost and EndpointPort are the peer this tunnel dials. The
	// firewall has to let them through outside the tunnel, since they are
	// where the tunnel is made.
	EndpointHost string
	EndpointPort string

	// Digest identifies the configuration without disclosing it, so that
	// replacing the file counts as a change to the cell.
	Digest string
}

// EndpointIsAddress reports whether the peer was given as an address rather
// than a name. The two are allowed through differently: an address goes
// straight into the firewall, while a name has to be resolved first — and by
// the cell's own resolver, which is the only thing that may add to the set.
func (t Tunnel) EndpointIsAddress() bool {
	return net.ParseIP(t.EndpointHost) != nil
}

// ReadTunnel reads a wg-quick configuration.
//
// It refuses configurations that cannot work in a cell rather than letting the
// machine come up with a tunnel that never connects, since a tunnel that is
// down is — by design — a cell that reaches nothing.
func ReadTunnel(path string) (*Tunnel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("no WireGuard configuration at %s\n"+
				"This cell sends everything through a tunnel, and the configuration for it is\n"+
				"yours rather than the cell's. Save the .conf file your VPN provider gives you\n"+
				"to that path, unchanged, and run this again", path)
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	sum := sha256.Sum256(data)
	tunnel := Tunnel{Digest: hex.EncodeToString(sum[:])}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		key, value, ok := setting(scanner.Text())
		if !ok {
			continue
		}

		switch key {
		case "dns":
			// wg-quick would write this into /etc/resolv.conf, which a
			// restricted cell has made immutable — so bringing the
			// tunnel up would fail. There is one place a cell's DNS is
			// decided, and this is not it.
			return nil, fmt.Errorf("%s: remove the DNS line\n"+
				"A cell resolves through its own resolver. To forward to the one this\n"+
				"tunnel provides, name it under network.resolvers instead", path)
		case "endpoint":
			host, port, err := net.SplitHostPort(value)
			if err != nil {
				return nil, fmt.Errorf("%s: Endpoint %q is not host:port", path, value)
			}
			if ipv6(host) {
				// The peer is the one address that has to be
				// reachable before the tunnel is, and it is
				// allowed by the same firewall as everything
				// else. Most providers publish both, so this
				// is usually a line to change rather than a
				// provider to leave.
				return nil, fmt.Errorf("%s: Endpoint %q is an IPv6 address, which a cell cannot reach: %s\n"+
					"Use the peer's IPv4 endpoint instead", path, host, noIPv6)
			}
			tunnel.EndpointHost, tunnel.EndpointPort = host, port
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	if tunnel.EndpointHost == "" {
		return nil, fmt.Errorf("%s: no Endpoint\n"+
			"The peer this tunnel dials has to be reachable before there is a tunnel, so\n"+
			"solitary needs it named here", path)
	}

	return &tunnel, nil
}

// setting splits one line of a wg-quick configuration. Section headers,
// comments and blank lines are not settings.
func setting(line string) (key, value string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
		return "", "", false
	}

	key, value, ok = strings.Cut(line, "=")
	if !ok {
		return "", "", false
	}

	return strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(value), true
}
