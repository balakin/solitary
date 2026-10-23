package cell

import (
	"strings"
	"testing"
	"time"

	"github.com/balakin/solitary/internal/config"
	"github.com/balakin/solitary/internal/lima"
)

// The release new machines get has to have an end, or a machine created
// today could never be told its support is ending.
func TestTheBaseHasASupportEnd(t *testing.T) {
	if _, ok := config.SupportEnds[newRelease()]; !ok {
		t.Errorf("config.SupportEnds has no end for %s, the release config.Base names", newRelease())
	}
}

func TestSupportOf(t *testing.T) {
	ends := config.SupportEnds["24.04"]
	for _, tc := range []struct {
		version string
		at      time.Time
		want    Support
	}{
		{"24.04", ends.AddDate(-2, 0, 0), SupportBehind},
		{"24.04", ends.Add(-endingWithin + time.Hour), SupportEnding},
		{"24.04", ends.Add(-time.Hour), SupportEnding},
		{"24.04", ends, SupportEnded},
		{newRelease(), config.SupportEnds[newRelease()].AddDate(-3, 0, 0), SupportCurrent},
		// The current release ends like any other.
		{newRelease(), config.SupportEnds[newRelease()].AddDate(0, -1, 0), SupportEnding},
		{"25.10", ends, SupportUnknown},
		{"", ends, SupportUnknown},
	} {
		if got := supportOf(tc.version, tc.at).Support; got != tc.want {
			t.Errorf("supportOf(%q, %s) = %d, want %d", tc.version, tc.at.Format(time.DateOnly), got, tc.want)
		}
	}
}

// What the machine said comes before what it was created from: the ubuntu-lts
// alias names no release, and a recorded base can be older than the machine.
func TestReleaseOfPrefersTheMachine(t *testing.T) {
	for _, tc := range []struct {
		guest *Guest
		base  string
		want  string
	}{
		{&Guest{Release: "Ubuntu 26.04.1 LTS"}, "ubuntu-24.04", "26.04"},
		{nil, "ubuntu-24.04", "24.04"},
		{&Guest{}, "ubuntu-24.04", "24.04"},
		{nil, "ubuntu-lts", ""},
		{nil, "ubuntu-99.04", ""},
		{nil, "", ""},
	} {
		if got := releaseOf(tc.guest, tc.base); got != tc.want {
			t.Errorf("releaseOf(%+v, %q) = %q, want %q", tc.guest, tc.base, got, tc.want)
		}
	}
}

// A machine on an ending release is marked, and up says so, pointing at the
// move; one with years left is not.
func TestAnEndingReleaseIsWarnedAbout(t *testing.T) {
	ends := config.SupportEnds["24.04"]
	defer func(old func() time.Time) { clock = old }(clock)

	c, inst, _ := attentionFixture(t, lima.StatusStopped)
	if err := writeGuest("probe", Guest{Release: "Ubuntu 24.04.5 LTS"}); err != nil {
		t.Fatal(err)
	}

	clock = func() time.Time { return ends.AddDate(-2, 0, 0) }
	if got := whats(attention("probe", c, inst)); got != "" {
		t.Errorf("two years out: attention() = %q, want nothing", got)
	}
	var out strings.Builder
	if warnRelease("probe", &out); out.Len() != 0 {
		t.Errorf("two years out: warnRelease() = %q, want nothing", out.String())
	}

	clock = func() time.Time { return ends.AddDate(0, -2, 0) }
	if got, want := whats(attention("probe", c, inst)), "machine: Ubuntu 24.04 stops getting fixes in May 2029"; got != want {
		t.Errorf("two months out: attention() = %q, want %q", got, want)
	}
	warnRelease("probe", &out)
	if !strings.Contains(out.String(), "solitary up --new-machine probe") {
		t.Errorf("warnRelease() = %q, want it to name the move", out.String())
	}
}
