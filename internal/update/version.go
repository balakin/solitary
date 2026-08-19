package update

import (
	"strconv"
	"strings"
)

// IsRelease reports whether version names a published release.
//
// Releases carry nothing but dotted numbers: release-please writes the version
// and GoReleaser bakes it in verbatim. Anything else is a build from source —
// `make build` bakes in what git describe says, which is the last tag plus the
// commits and the dirty tree on top of it. Such a build is ahead of the release
// it names, not behind it, so it is never compared and never replaced.
func IsRelease(version string) bool {
	_, ok := parse(version)
	return ok
}

// Newer reports whether want is a later release than have. A version that is
// not a release, on either side, is never newer.
func Newer(have, want string) bool {
	hi, ok := parse(have)
	if !ok {
		return false
	}
	wi, ok := parse(want)
	if !ok {
		return false
	}

	for i := range max(len(hi), len(wi)) {
		h, w := at(hi, i), at(wi, i)
		if h != w {
			return w > h
		}
	}

	return false
}

// parse splits a version into its numeric parts, reporting whether it is a
// release version at all.
func parse(version string) ([]int, bool) {
	var parts []int

	for field := range strings.SplitSeq(strings.TrimPrefix(version, "v"), ".") {
		n, err := strconv.Atoi(field)
		if err != nil || n < 0 {
			return nil, false
		}
		parts = append(parts, n)
	}
	if len(parts) == 0 {
		return nil, false
	}

	return parts, true
}

// at treats a missing component as zero, so that 0.2 and 0.2.0 are one version.
func at(parts []int, i int) int {
	if i >= len(parts) {
		return 0
	}
	return parts[i]
}
