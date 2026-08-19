package update

import "testing"

func TestNewer(t *testing.T) {
	cases := []struct {
		have, want string
		newer      bool
	}{
		{"0.1.1", "0.1.2", true},
		{"0.1.1", "0.2.0", true},
		{"0.9.0", "0.10.0", true}, // ordered as numbers, not as text
		{"0.1.1", "0.1.1", false},
		{"0.1.2", "0.1.1", false},
		{"0.2", "0.2.0", false}, // a missing component is a zero
		{"0.2", "0.2.1", true},
		{"dev", "0.1.1", false},                      // a source build is never behind
		{"v0.1.1-10-g619fd0f-dirty", "0.1.1", false}, // nor is a build git describe named
		{"v0.1.1-10-g619fd0f", "0.2.0", false},
		{"0.1.1", "", false},
		{"0.1.1", "not-a-version", false},
	}

	for _, c := range cases {
		if got := Newer(c.have, c.want); got != c.newer {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.have, c.want, got, c.newer)
		}
	}
}

func TestIsRelease(t *testing.T) {
	cases := map[string]bool{
		"0.1.1":                    true,
		"v0.1.1":                   true,
		"1.0":                      true,
		"dev":                      false,
		"v0.1.1-10-g619fd0f-dirty": false,
		"":                         false,
	}

	for version, release := range cases {
		if got := IsRelease(version); got != release {
			t.Errorf("IsRelease(%q) = %v, want %v", version, got, release)
		}
	}
}

func TestManagedBy(t *testing.T) {
	cases := map[string]bool{
		"/opt/homebrew/Cellar/solitary/0.1.1/bin/solitary": true,
		"/opt/homebrew/Caskroom/solitary/0.1.1/solitary":   true,
		"/home/linuxbrew/.linuxbrew/Cellar/s/0.1/bin/s":    true,
		"/usr/local/bin/solitary":                          false,
		"/home/someone/.local/bin/solitary":                false,
		"/home/someone/src/CellarDoor/solitary":            false,
	}

	for path, managed := range cases {
		if got := managedBy(path) != ""; got != managed {
			t.Errorf("managedBy(%q) managed = %v, want %v", path, got, managed)
		}
	}
}
