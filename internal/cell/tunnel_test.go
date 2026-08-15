package cell

import (
	"strings"
	"testing"
	"time"
)

// A peer line of `wg show <if> dump`: public key, preshared key, endpoint,
// allowed ips, last handshake, received, sent, keepalive.
func TestParsePeer(t *testing.T) {
	var state TunnelState
	parsePeer(&state, strings.Join([]string{
		"d3ldwEjnFcbLwD1o8uC5xC3DaSNek8DGeTpOb/h/IE4=",
		"(none)",
		"86.38.98.13:51820",
		"0.0.0.0/0",
		"1755256000",
		"188352",
		"15575",
		"off",
	}, "\t"))

	if state.Peer != "86.38.98.13" {
		t.Errorf("Peer = %q, want the address without its port", state.Peer)
	}
	if !state.Handshook {
		t.Error("a peer with a handshake time was read as never having handshaken")
	}
	if state.Received != 188352 || state.Sent != 15575 {
		t.Errorf("transfer = %d/%d, want 188352/15575", state.Received, state.Sent)
	}
}

// A tunnel that is up but has never handshaken looks, from inside the cell,
// exactly like a network that is down. The two must not read the same here.
func TestParsePeerWithoutAHandshake(t *testing.T) {
	var state TunnelState
	parsePeer(&state, strings.Join([]string{
		"d3ldwEjnFcbLwD1o8uC5xC3DaSNek8DGeTpOb/h/IE4=",
		"(none)", "(none)", "0.0.0.0/0", "0", "0", "0", "off",
	}, "\t"))

	if state.Handshook {
		t.Error("a peer that has never handshaken was read as having done so")
	}
	if state.Peer != "" {
		t.Errorf("Peer = %q for a peer with no endpoint yet", state.Peer)
	}

	state.Up = true
	if state.Healthy() {
		t.Error("a tunnel that has never handshaken is not healthy")
	}
}

func TestTunnelHealthy(t *testing.T) {
	cases := []struct {
		name  string
		state TunnelState
		want  bool
	}{
		{"up and talking", TunnelState{Up: true, Handshook: true, Since: 30 * time.Second}, true},
		{"up and silent", TunnelState{Up: true, Handshook: true, Since: time.Hour}, false},
		{"never handshaken", TunnelState{Up: true}, false},
		{"not up at all", TunnelState{}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.state.Healthy(); got != tc.want {
				t.Errorf("Healthy() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTransferred(t *testing.T) {
	got := TunnelState{Received: 188352, Sent: 15575}.Transferred()
	if !strings.Contains(got, "183.9 KiB") || !strings.Contains(got, "15.2 KiB") {
		t.Errorf("Transferred() = %q", got)
	}
	if got := (TunnelState{Received: 12}).Transferred(); !strings.Contains(got, "12 B") {
		t.Errorf("Transferred() = %q for a handful of bytes", got)
	}
}
