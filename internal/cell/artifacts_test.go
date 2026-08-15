package cell

import (
	"strings"
	"testing"
)

// The outbox is written inside the cell and read out here, so a name in it is
// input from the least trusted place there is. This is the rule that stops one
// being used as a path.
func TestValidName(t *testing.T) {
	cases := []struct {
		name string
		ok   bool
	}{
		{"report.pdf", true},
		{".hidden", true},
		{"a name with spaces.txt", true},
		{"", false},
		{".", false},
		{"..", false},
		{"../escape", false},
		{"/etc/passwd", false},
		{"sub/dir", false},
		{`windows\path`, false},
		{"-rf", false},
		{"two\nlines", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validName(tc.name)
			if tc.ok && err != nil {
				t.Errorf("validName(%q) = %v, want it accepted", tc.name, err)
			}
			if !tc.ok && err == nil {
				t.Errorf("validName(%q) accepted a name that is not one", tc.name)
			}
		})
	}
}

func TestSelection(t *testing.T) {
	published := []Artifact{
		{Name: "report.pdf", Size: 10},
		{Name: "-rf", Size: 20, Problem: `"-rf" starts with a dash`},
		{Name: "dist.tar", Size: 30},
	}

	t.Run("everything fetchable by default", func(t *testing.T) {
		wanted, refused, err := selection(published, nil)
		if err != nil {
			t.Fatalf("selection() error = %v", err)
		}
		if len(wanted) != 2 || wanted[0].Name != "report.pdf" || wanted[1].Name != "dist.tar" {
			t.Errorf("wanted = %v", wanted)
		}
		// One badly named file must not hold up the rest, and must not
		// disappear either.
		if len(refused) != 1 || refused[0].Name != "-rf" {
			t.Errorf("refused = %v", refused)
		}
	})

	t.Run("by name", func(t *testing.T) {
		wanted, _, err := selection(published, []string{"dist.tar"})
		if err != nil {
			t.Fatalf("selection() error = %v", err)
		}
		if len(wanted) != 1 || wanted[0].Name != "dist.tar" {
			t.Errorf("wanted = %v", wanted)
		}
	})

	t.Run("a name that is not there", func(t *testing.T) {
		_, _, err := selection(published, []string{"nope"})
		if err == nil {
			t.Fatal("selection() quietly fetched less than was asked for")
		}
		if !strings.Contains(err.Error(), "--list") {
			t.Errorf("error %q does not say how to find out what is there", err)
		}
	})

	t.Run("asking for one that cannot come out", func(t *testing.T) {
		wanted, refused, err := selection(published, []string{"-rf"})
		if err != nil {
			t.Fatalf("selection() error = %v", err)
		}
		if len(wanted) != 0 || len(refused) != 1 {
			t.Errorf("wanted = %v, refused = %v", wanted, refused)
		}
	})
}

func TestSize(t *testing.T) {
	for _, tc := range []struct {
		n    int64
		want string
	}{
		{12, "12 B"},
		{188352, "183.9 KiB"},
		{5 * 1024 * 1024, "5.0 MiB"},
	} {
		if got := Size(tc.n); got != tc.want {
			t.Errorf("Size(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}
