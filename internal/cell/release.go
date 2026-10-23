package cell

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/balakin/solitary/internal/config"
)

// Support is where a machine's release stands against the fixes Ubuntu
// publishes for it.
type Support int

const (
	// SupportUnknown is a release solitary cannot name: a machine never seen
	// running, created from Lima's moving ubuntu-lts alias.
	SupportUnknown Support = iota
	// SupportCurrent is the release new machines are created from.
	SupportCurrent
	// SupportBehind is an older release with years of fixes left. Worth
	// knowing, not worth a warning.
	SupportBehind
	// SupportEnding is a release whose fixes stop within endingWithin.
	SupportEnding
	// SupportEnded is a release that gets no more fixes.
	SupportEnded
)

// endingWithin is how long before a release's end it starts being warned
// about: long enough to move a cell when it suits, rather than the week the
// fixes stop.
const endingWithin = 183 * 24 * time.Hour

// clock is what support is judged by, replaced in tests.
var clock = time.Now

// Release is a machine's Ubuntu release, how it stands, and when its fixes
// stop.
type Release struct {
	Version string
	Support Support
	Ends    time.Time
}

var releaseVersion = regexp.MustCompile(`^Ubuntu (\d\d\.\d\d)`)

// releaseOf names the release a machine runs, from what it last said about
// itself, or failing that from the base the record says it was created from.
// The guest comes first: a machine created from the ubuntu-lts alias runs
// whatever release Lima took that to mean, and only asking it says which.
func releaseOf(g *Guest, base string) string {
	if g != nil {
		if m := releaseVersion.FindStringSubmatch(g.Release); m != nil {
			return m[1]
		}
	}
	if version, ok := strings.CutPrefix(base, "ubuntu-"); ok && config.SupportEnds[version] != (time.Time{}) {
		return version
	}

	return ""
}

// supportOf says how a release stands at t.
func supportOf(version string, t time.Time) Release {
	ends, known := config.SupportEnds[version]
	r := Release{Version: version, Ends: ends}
	switch {
	case !known:
		r.Support = SupportUnknown
	case !t.Before(ends):
		r.Support = SupportEnded
	case ends.Sub(t) < endingWithin:
		r.Support = SupportEnding
	case "ubuntu-"+version == config.Base:
		r.Support = SupportCurrent
	default:
		r.Support = SupportBehind
	}

	return r
}

// machineRelease is how the release of a cell's machine stands now, from what
// this host keeps about it: nothing here asks the machine.
func machineRelease(name string) Release {
	record, _ := config.ReadApplied(name)
	guest, _ := readGuest(name)

	return supportOf(releaseOf(guest, record.Base), clock())
}

// newRelease is the release new machines get, as a person names it.
func newRelease() string {
	return strings.TrimPrefix(config.Base, "ubuntu-")
}

// Describe says how the release stands, for a report: empty for one solitary
// cannot name.
func (r Release) Describe() string {
	ends := r.Ends.Format("January 2006")
	switch r.Support {
	case SupportCurrent:
		return fmt.Sprintf("Ubuntu %s is supported until %s", r.Version, ends)
	case SupportBehind:
		return fmt.Sprintf("Ubuntu %s is supported until %s; new machines get %s", r.Version, ends, newRelease())
	case SupportEnding:
		return fmt.Sprintf("Ubuntu %s stops getting fixes in %s", r.Version, ends)
	case SupportEnded:
		return fmt.Sprintf("Ubuntu %s stopped getting fixes in %s", r.Version, ends)
	}
	return ""
}

// Warns reports whether the release is one to act on.
func (r Release) Warns() bool {
	return r.Support == SupportEnding || r.Support == SupportEnded
}

// warnRelease tells up about a machine whose release is ending or has ended.
// It never refuses: the cell still works, and moving it is a choice about
// when, which belongs to whoever runs it.
func warnRelease(name string, w io.Writer) {
	r := machineRelease(name)
	if !r.Warns() {
		return
	}

	fmt.Fprintf(w, "Warning: %s, the release the machine behind %q runs.\n", r.Describe(), name)
	fmt.Fprintf(w, "         Moving it to %s keeps its home:\n", newRelease())
	fmt.Fprintf(w, "           solitary up --new-machine %s\n", name)
}
