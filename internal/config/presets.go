package config

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// A preset is an allow list entry that stands for a list of names solitary
// carries, for a service that is reached through more names than anyone should
// have to type — and more than its own domain covers.
//
// GitHub is why they exist. A job's logs and artifacts are served by a redirect
// from api.github.com to one of a hundred and forty Azure storage accounts,
// numbered, and which one a given job landed in is GitHub's business. Allowing
// blob.core.windows.net instead would open every storage account on Azure, and
// a pattern such as productionresultssa*.blob.core.windows.net is no narrower
// than that: storage account names are global and first come, first served, so
// a pattern also matches the accounts GitHub never registered and anyone can.
// What is left is listing each account GitHub owns, by name.
//
// The lists are compiled in rather than fetched from GitHub's meta API at boot,
// so that what a cell may reach is fixed by the binary it runs under, and two
// boots of the same cell never differ because a server answered differently.
// They move with a release instead: TestPresetsMatchGitHubMeta compares them
// with what GitHub publishes, when asked to.
var presets = map[string]func() []string{
	// Where a job's logs and artifacts are stored, which `gh run view --log`
	// and an artifact download are redirected to. GitHub's meta API publishes
	// them as domains.actions_inbound.full_domains.
	"github:actions-results": func() []string {
		return numbered("productionresultssa%d.blob.core.windows.net", [][2]int{{0, 19}, {30, 49}, {100, 199}})
	},

	// GitHub Packages: the registries, and where they store what they serve.
	// The npm, Maven, NuGet and RubyGems registries answer on pkg.github.com
	// and redirect downloads to a storage account each, from the meta API's
	// domains.packages. ghcr.io serves image layers from
	// pkg-containers.githubusercontent.com, which the meta API does not list.
	"github:packages": func() []string {
		return []string{
			"pkg.github.com",
			"ghcr.io",
			"pkg-containers.githubusercontent.com",
			"npmregistryv2prod.blob.core.windows.net",
			"mavenregistryv2prod.blob.core.windows.net",
			"nugetregistryv2prod.blob.core.windows.net",
			"rubygemsregistryv2prod.blob.core.windows.net",
		}
	},
}

// numbered expands a name with a number in it over inclusive ranges.
func numbered(format string, ranges [][2]int) []string {
	var names []string
	for _, r := range ranges {
		for i := r[0]; i <= r[1]; i++ {
			names = append(names, fmt.Sprintf(format, i))
		}
	}

	return names
}

// isPreset reports whether an entry is meant as a preset, known or not. A colon
// is never in a DNS name, and an IPv6 address is checked for first.
func isPreset(entry string) bool {
	return strings.Contains(entry, ":") && !ipv6(entry)
}

// Preset returns the names a preset stands for, and whether it is one.
func Preset(entry string) ([]string, bool) {
	expand, ok := presets[entry]
	if !ok {
		return nil, false
	}

	return expand(), true
}

// Presets are the names of every preset, sorted, for an error that has to say
// which ones exist.
func Presets() []string {
	names := make([]string, 0, len(presets))
	for name := range presets {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

// Entries is the allow list with every preset replaced by the names it stands
// for: what is enforced, where Allow is what was written. A name listed twice —
// by hand and by a preset — is kept once.
func (n Network) Entries() []string {
	var entries []string
	for _, entry := range n.Allow {
		names, ok := Preset(entry)
		if !ok {
			names = []string{entry}
		}
		for _, name := range names {
			if !slices.Contains(entries, name) {
				entries = append(entries, name)
			}
		}
	}

	return entries
}
