package cell

import (
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/balakin/solitary/internal/config"
)

// What the script prints on a 26.04 machine, trimmed: every key it can.
const guestOutput = `release=Ubuntu 26.04.1 LTS
kernel=7.0.0-34-generic
reboot=yes
updates=23;7
lists=1792000000
unattended=enabled
disk=3328000000 21474836480
package.podman=5.7.0+ds2-1
package.ca-certificates=20260816ubuntu0.26.04.1
something.new=a key a newer script prints
`

func TestParseGuest(t *testing.T) {
	g := parseGuest(guestOutput)

	if g.Release != "Ubuntu 26.04.1 LTS" || g.Kernel != "7.0.0-34-generic" || !g.RebootRequired {
		t.Errorf("parseGuest() = %+v, want release, kernel and the pending reboot", g)
	}
	if g.Updates == nil || *g.Updates != 23 || g.Security == nil || *g.Security != 7 {
		t.Errorf("updates, security = %v, %v; want 23 and 7", g.Updates, g.Security)
	}
	if !g.ListsRefreshed.Equal(time.Unix(1792000000, 0)) || g.Unattended != "enabled" {
		t.Errorf("lists, unattended = %v, %q", g.ListsRefreshed, g.Unattended)
	}
	if g.DiskUsed != 3328000000 || g.DiskSize != 21474836480 {
		t.Errorf("disk = %d of %d", g.DiskUsed, g.DiskSize)
	}
	if g.Packages["podman"] != "5.7.0+ds2-1" || len(g.Packages) != 2 {
		t.Errorf("packages = %v, want the two named and nothing from an unknown key", g.Packages)
	}
}

// A machine that cannot count its updates — no apt-check, an odd answer —
// reports nothing about them rather than zero, which would read as current.
func TestParseGuestLeavesUnknownCountsUnknown(t *testing.T) {
	for _, out := range []string{"release=Ubuntu\n", "updates=\n", "updates=E: apt is broken\n"} {
		if g := parseGuest(out); g.Updates != nil || g.Security != nil {
			t.Errorf("parseGuest(%q) counted %v, %v updates", out, g.Updates, g.Security)
		}
	}
}

// What was seen in a running machine is kept for when it stops, and read back
// as what it is: not live. rm forgets it with the machine.
func TestGuestIsKeptUntilTheMachineIsRemoved(t *testing.T) {
	isolate(t)

	if got, err := readGuest("probe"); err != nil || got != nil {
		t.Fatalf("readGuest() = %v, %v before anything was seen", got, err)
	}

	seen := parseGuest(guestOutput)
	seen.AsOf, seen.Live = time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC), true
	if err := writeGuest("probe", seen); err != nil {
		t.Fatal(err)
	}
	got, err := readGuest("probe")
	if err != nil || got == nil {
		t.Fatalf("readGuest() = %v, %v", got, err)
	}
	if got.Live || !got.AsOf.Equal(seen.AsOf) || got.Release != seen.Release || *got.Security != 7 {
		t.Errorf("readGuest() = %+v, want what was written, no longer live", got)
	}

	if err := config.RemoveApplied("probe"); err != nil {
		t.Fatal(err)
	}
	if got, _ := readGuest("probe"); got != nil {
		t.Errorf("readGuest() = %+v after the machine was removed", got)
	}
}

// A report is made to be pasted into an issue, and the container's
// environment is where a cell's secrets are. Nothing in the report may be able
// to hold it — checked on the types, so a field added later is caught too.
func TestReportHasNowhereToPutTheEnvironment(t *testing.T) {
	seen := map[reflect.Type]bool{}
	var walk func(t reflect.Type, path string)
	walk = func(typ reflect.Type, path string) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Map {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || seen[typ] {
			return
		}
		seen[typ] = true
		for i := range typ.NumField() {
			field := typ.Field(i)
			name := strings.ToLower(field.Name)
			for _, word := range []string{"env", "secret", "token", "key"} {
				if strings.Contains(name, word) {
					t.Errorf("%s.%s could carry a cell's environment", path, field.Name)
				}
			}
			walk(field.Type, path+"."+field.Name)
		}
	}
	walk(reflect.TypeOf(Report{}), "Report")
}

// The script runs in a machine solitary does not otherwise test it on, so it
// is at least held to parsing.
func TestInspectScriptParses(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	file := t.TempDir() + "/inspect.sh"
	if err := os.WriteFile(file, []byte(inspectScript), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(sh, "-n", file).CombinedOutput(); err != nil {
		t.Errorf("sh -n inspect.sh: %v\n%s", err, out)
	}
}
