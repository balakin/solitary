package cell

import (
	"reflect"
	"testing"
)

// Both of apt's formats are read, since a machine carries Ubuntu's own sources
// in one and whatever a provision script added may be in the other.
func TestSourceHostsReadsBothFormats(t *testing.T) {
	sources := `Types: deb
URIs: http://archive.ubuntu.com/ubuntu
Suites: resolute resolute-updates
Components: main restricted universe multiverse

Types: deb
URIs: http://security.ubuntu.com/ubuntu http://Security.Ubuntu.com/ubuntu
# deb http://commented.example.com/ubuntu resolute main
deb [signed-by=/usr/share/keyrings/x.gpg arch=amd64] https://ppa.launchpadcontent.net/x/y/ubuntu resolute main
deb-src https://src.example.com/debian stable main
deb file:/srv/local ./
deb cdrom:[Ubuntu 26.04]/ resolute main
`
	want := []string{"archive.ubuntu.com", "ppa.launchpadcontent.net", "security.ubuntu.com", "src.example.com"}
	if got := sourceHosts(sources); !reflect.DeepEqual(got, want) {
		t.Errorf("sourceHosts() = %q, want %q", got, want)
	}
}
