package cell

import "testing"

// A move is only worth suggesting to a machine on another release: one created
// from the ubuntu-lts alias that Lima resolved to the release solitary names
// now has nothing to move to.
func TestRunsBase(t *testing.T) {
	for _, tc := range []struct {
		guest *Guest
		want  bool
	}{
		{&Guest{Release: "Ubuntu 26.04.1 LTS"}, true},
		{&Guest{Release: "Ubuntu 24.04.5 LTS"}, false},
		{&Guest{}, false},
		{nil, false},
	} {
		if got := runsBase(tc.guest, "ubuntu-26.04"); got != tc.want {
			t.Errorf("runsBase(%+v, ubuntu-26.04) = %v, want %v", tc.guest, got, tc.want)
		}
	}
}
