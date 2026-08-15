package cell

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dm-balakin/solitary/internal/config"
	"github.com/dm-balakin/solitary/internal/lima"
)

// ErrNoTunnel is returned for a cell that does not send its traffic through
// one.
var ErrNoTunnel = errors.New("cell has no tunnel")

// TunnelState is what a cell's tunnel is doing right now, as opposed to what
// its definition asks for. The two differ in the case worth seeing: a tunnel
// that is configured and not up, which is a cell that reaches nothing.
type TunnelState struct {
	// Endpoint is the peer as configured, and Peer is the address it is
	// actually talking to — a provider that balances a hostname across
	// servers moves this between handshakes.
	Endpoint string
	Peer     string

	// Up reports whether the interface exists at all.
	Up bool

	// Since is how long ago the last handshake was. A tunnel with no
	// handshake is up but not carrying anything, which looks the same from
	// inside the cell as a network that is simply down.
	Since     time.Duration
	Handshook bool

	Received int64
	Sent     int64
}

// Healthy reports a tunnel that is up and has been talking to its peer
// recently. WireGuard rehandshakes every two minutes while there is traffic,
// so anything much older than that means nothing is going through.
func (t TunnelState) Healthy() bool {
	return t.Up && t.Handshook && t.Since < 5*time.Minute
}

// TunnelStatus reads the state of a cell's tunnel from its machine.
//
// This shells into the machine, so it is not free: call it for the cell being
// looked at, not for every cell in a list.
func TunnelStatus(name string) (TunnelState, error) {
	c, err := config.LoadCell(name)
	if err != nil {
		return TunnelState{}, err
	}
	if c.Network.Tunnel == nil {
		return TunnelState{}, ErrNoTunnel
	}

	state := TunnelState{
		Endpoint: c.Network.Tunnel.EndpointHost + ":" + c.Network.Tunnel.EndpointPort,
	}

	// The first line of a dump is the interface, and it begins with the
	// private key. It is dropped in the machine rather than here: a key has
	// no reason to cross into this process, let alone onto a screen.
	out, err := lima.Exec(config.Instance(name), "sh", "-c",
		"sudo wg show "+config.VPNInterface+" dump 2>/dev/null | tail -n +2")
	if err != nil {
		return state, fmt.Errorf("reading the tunnel in %q: %w", name, err)
	}

	// An interface that is not there says nothing, and the pipeline still
	// succeeds — so no output is the answer "down", not a failure.
	peer := strings.TrimSpace(string(out))
	if peer == "" {
		return state, nil
	}
	state.Up = true
	parsePeer(&state, peer)

	return state, nil
}

// parsePeer reads one peer line of `wg show <if> dump`: public key, preshared
// key, endpoint, allowed ips, last handshake as a unix time, bytes received,
// bytes sent, keepalive.
func parsePeer(state *TunnelState, line string) {
	fields := strings.Split(strings.SplitN(line, "\n", 2)[0], "\t")
	if len(fields) < 8 {
		return
	}

	if fields[2] != "(none)" {
		state.Peer, _, _ = strings.Cut(fields[2], ":")
	}
	if at, err := strconv.ParseInt(fields[4], 10, 64); err == nil && at > 0 {
		state.Handshook = true
		state.Since = time.Since(time.Unix(at, 0)).Truncate(time.Second)
	}
	state.Received, _ = strconv.ParseInt(fields[5], 10, 64)
	state.Sent, _ = strconv.ParseInt(fields[6], 10, 64)
}

// Transferred renders what has gone through the tunnel.
func (t TunnelState) Transferred() string {
	return "↓ " + Size(t.Received) + "  ↑ " + Size(t.Sent)
}
