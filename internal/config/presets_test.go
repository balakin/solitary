package config

import (
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestEntriesExpandPresets(t *testing.T) {
	n := Network{Allow: []string{"github.com", "github:actions-results", "10.1.2.0/24", "productionresultssa0.blob.core.windows.net"}}
	entries := n.Entries()

	if len(entries) != 2+140 {
		t.Fatalf("Entries() has %d names, want github.com, the address and 140 storage accounts", len(entries))
	}
	if entries[0] != "github.com" || entries[1] != "productionresultssa0.blob.core.windows.net" {
		t.Errorf("Entries() = %q..., want the list in the order it was written", entries[:2])
	}
	if slices.Contains(entries, "github:actions-results") {
		t.Error("Entries() kept the preset's own name")
	}
	if got := n.Addresses(); !slices.Equal(got, []string{"10.1.2.0/24"}) {
		t.Errorf("Addresses() = %q, want only the address", got)
	}
}

func TestPresetsAreEnforced(t *testing.T) {
	n := Network{Allow: []string{"github:actions-results", "github:packages"}}
	for host, want := range map[string]bool{
		"productionresultssa15.blob.core.windows.net":  true,
		"productionresultssa199.blob.core.windows.net": true,
		"productionresultssa25.blob.core.windows.net":  false,
		"npmregistryv2prod.blob.core.windows.net":      true,
		"npm.pkg.github.com":                           true,
		"ghcr.io":                                      true,
		"example.blob.core.windows.net":                false,
		"blob.core.windows.net":                        false,
		"github.com":                                   false,
	} {
		if got := n.Covers(host); got != want {
			t.Errorf("Covers(%q) = %v, want %v", host, got, want)
		}
	}
}

// Every name a preset stands for is rendered into the machine's resolver, so it
// is held to the same rule as a name written by hand.
func TestPresetsHoldOnlyDomains(t *testing.T) {
	for _, name := range Presets() {
		names, _ := Preset(name)
		if len(names) == 0 {
			t.Errorf("preset %s stands for nothing", name)
		}
		for _, entry := range names {
			if !ValidDomain(entry) || strings.Contains(entry, "*") {
				t.Errorf("preset %s holds %q, which is not a DNS name", name, entry)
			}
		}
	}
}

func TestValidateAllowRefusesUnknownPresets(t *testing.T) {
	if err := (Network{Allow: []string{"github:actions-results", "github:packages"}}).validateAllow(); err != nil {
		t.Errorf("validateAllow() refused a known preset: %v", err)
	}

	err := (Network{Allow: []string{"github:actions"}}).validateAllow()
	if err == nil {
		t.Fatal("validateAllow() accepted a preset solitary does not know")
	}
	if !strings.Contains(err.Error(), "github:actions-results") {
		t.Errorf("error = %q, want it to name the presets that exist", err)
	}

	// Still an IPv6 address, and refused as one rather than as a preset.
	err = (Network{Allow: []string{"2606:4700::1111"}}).validateAllow()
	if err == nil || !strings.Contains(err.Error(), "IPv6") {
		t.Errorf("validateAllow() of an IPv6 address = %v, want the IPv6 refusal", err)
	}
}

// TestPresetsMatchGitHubMeta compares the GitHub presets with what GitHub
// publishes. It asks the network, so it runs only when asked to:
//
//	SOLITARY_CHECK_GITHUB_META=1 go test ./internal/config/ -run GitHubMeta
func TestPresetsMatchGitHubMeta(t *testing.T) {
	if os.Getenv("SOLITARY_CHECK_GITHUB_META") == "" {
		t.Skip("set SOLITARY_CHECK_GITHUB_META=1 to compare the presets with api.github.com/meta")
	}

	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get("https://api.github.com/meta")
	if err != nil {
		t.Fatalf("fetching GitHub's meta API: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("fetching GitHub's meta API: %s", resp.Status)
	}
	var meta struct {
		Domains struct {
			Packages       []string `json:"packages"`
			ActionsInbound struct {
				FullDomains []string `json:"full_domains"`
			} `json:"actions_inbound"`
		} `json:"domains"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		t.Fatalf("reading GitHub's meta API: %v", err)
	}

	results := regexp.MustCompile(`^productionresultssa\d+\.blob\.core\.windows\.net$`)
	var published []string
	for _, name := range meta.Domains.ActionsInbound.FullDomains {
		if results.MatchString(name) {
			published = append(published, name)
		}
	}
	compare(t, "github:actions-results", published, func(string) bool { return true })

	var storage []string
	for _, name := range meta.Domains.Packages {
		if !strings.Contains(name, "*") && strings.HasSuffix(name, ".blob.core.windows.net") {
			storage = append(storage, name)
		}
	}
	compare(t, "github:packages", storage, func(name string) bool { return strings.HasSuffix(name, ".blob.core.windows.net") })
}

// compare reports the names GitHub publishes that a preset lacks, and the ones
// of its own, among those in scope, that GitHub no longer publishes.
func compare(t *testing.T, preset string, published []string, scope func(string) bool) {
	t.Helper()
	carried, _ := Preset(preset)
	for _, name := range published {
		if !slices.Contains(carried, name) {
			t.Errorf("%s lacks %s, which GitHub publishes", preset, name)
		}
	}
	for _, name := range carried {
		if scope(name) && !slices.Contains(published, name) {
			t.Errorf("%s holds %s, which GitHub no longer publishes", preset, name)
		}
	}
}
