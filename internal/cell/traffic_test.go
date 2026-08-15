package cell

import "testing"

// The lines below are verbatim from a cell's journal, because the whole value
// of this parser is that it reads what the machine actually writes.
func TestParseTraffic(t *testing.T) {
	cases := []struct {
		name string
		line string
		want Traffic
	}{
		{
			name: "refused connection",
			line: "2026-08-15T11:57:48+02:00 lima-solitary-claude kernel: solitary-deny IN= OUT=eth0 SRC=192.168.5.15 DST=192.168.5.3 LEN=64 TOS=0x00 PREC=0x00 TTL=64 ID=12353 DF PROTO=TCP SPT=56890 DPT=53 WINDOW=64240 RES=0x00 SYN URGP=0",
			want: Traffic{At: "11:57:48", Kind: TrafficDenied, Detail: "192.168.5.3:53"},
		},
		{
			name: "name asked about",
			line: "2026-08-15T11:57:48+02:00 lima-solitary-claude dnsmasq[2696]: query[A] api.github.com from 127.0.0.1",
			want: Traffic{At: "11:57:48", Kind: TrafficQuery, Detail: "api.github.com"},
		},
		{
			name: "name answered",
			line: "2026-08-15T11:57:48+02:00 lima-solitary-claude dnsmasq[2696]: reply api.github.com is 140.82.121.6",
			want: Traffic{At: "11:57:48", Kind: TrafficResolved, Detail: "api.github.com → 140.82.121.6"},
		},
		{
			name: "answered from cache",
			line: "2026-08-15T11:57:48+02:00 lima-solitary-claude dnsmasq[2696]: cached 3.ntp.ubuntu.com is 91.189.91.112",
			want: Traffic{At: "11:57:48", Kind: TrafficResolved, Detail: "3.ntp.ubuntu.com → 91.189.91.112"},
		},
		{
			name: "name not on the allow list",
			line: "2026-08-15T11:57:48+02:00 lima-solitary-claude dnsmasq[2696]: config example.com is NXDOMAIN",
			want: Traffic{At: "11:57:48", Kind: TrafficRefused, Detail: "example.com"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseTraffic(tc.line)
			if !ok {
				t.Fatalf("ParseTraffic() did not recognise the line")
			}
			if got != tc.want {
				t.Errorf("ParseTraffic() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// Most of a journal says nothing about the network, and the AAAA answers are an
// artifact of asking for addresses the machine cannot use.
func TestParseTrafficIgnoresTheRest(t *testing.T) {
	for _, line := range []string{
		"2026-08-15T11:57:48+02:00 lima-solitary-claude systemd[1]: Started dnsmasq.service.",
		"2026-08-15T11:57:48+02:00 lima-solitary-claude dnsmasq[2696]: config 4.ntp.ubuntu.com is NODATA-IPv6",
		"",
	} {
		if got, ok := ParseTraffic(line); ok {
			t.Errorf("ParseTraffic(%q) = %+v, want it ignored", line, got)
		}
	}
}
