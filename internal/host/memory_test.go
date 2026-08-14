package host

import (
	"strings"
	"testing"
)

func TestParseSize(t *testing.T) {
	cases := map[string]uint64{
		"4GiB":   4 << 30,
		"8GiB":   8 << 30,
		"512MiB": 512 << 20,
		"2G":     2 << 30,
		"1.5GiB": 1536 << 20,
		"1024":   1024,
		"20 GiB": 20 << 30,
	}

	for in, want := range cases {
		got, err := ParseSize(in)
		if err != nil {
			t.Errorf("ParseSize(%q) error = %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseSize(%q) = %d, want %d", in, got, want)
		}
	}

	for _, in := range []string{"", "lots", "4 potatoes", "GiB"} {
		if _, err := ParseSize(in); err == nil {
			t.Errorf("ParseSize(%q) = nil error, want one", in)
		}
	}
}

// A machine larger than its backing store dies without reporting anything, so
// this has to be refused rather than warned about.
func TestVerifyMemoryRefusesWhatCannotBeBacked(t *testing.T) {
	backing := Backing{Total: 7900 << 20, Free: 7900 << 20, Known: true} // 7.7GiB

	_, err := VerifyMemory("8GiB", backing)
	if err == nil {
		t.Fatal("VerifyMemory() = nil error for a machine larger than its backing store")
	}
	for _, want := range []string{"8.0GiB", "/dev/shm", "vm.memory"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q:\n%v", want, err)
		}
	}
}

func TestVerifyMemoryWarnsWhenBackingIsTight(t *testing.T) {
	// Fits the filesystem, but most of it is in use by something else.
	backing := Backing{Total: 8 << 30, Free: 2 << 30, Known: true}

	warning, err := VerifyMemory("4GiB", backing)
	if err != nil {
		t.Fatalf("VerifyMemory() error = %v, want a warning only", err)
	}
	if warning == "" {
		t.Error("VerifyMemory() gave no warning for a machine that does not fit in free space")
	}
}

func TestVerifyMemoryAcceptsWhatFits(t *testing.T) {
	backing := Backing{Total: 8 << 30, Free: 7 << 30, Known: true}

	warning, err := VerifyMemory("4GiB", backing)
	if err != nil {
		t.Fatalf("VerifyMemory() error = %v", err)
	}
	if warning != "" {
		t.Errorf("VerifyMemory() warned about a machine that fits: %s", warning)
	}
}

// On hosts where guest memory is not backed by a sized filesystem there is
// nothing to check, and nothing should be refused.
func TestVerifyMemoryIgnoresUnknownBacking(t *testing.T) {
	warning, err := VerifyMemory("64GiB", Backing{Known: false})
	if err != nil || warning != "" {
		t.Errorf("VerifyMemory() = (%q, %v), want no opinion when backing is unknown", warning, err)
	}
}

func TestFormatSize(t *testing.T) {
	cases := map[uint64]string{
		4 << 30:   "4.0GiB",
		512 << 20: "512MiB",
	}
	for in, want := range cases {
		if got := FormatSize(in); got != want {
			t.Errorf("FormatSize(%d) = %q, want %q", in, got, want)
		}
	}
}

// The real host must be measurable where it matters, or the check silently
// never fires.
func TestMemoryBackingReadsThisHost(t *testing.T) {
	b, err := MemoryBacking()
	if err != nil {
		t.Fatalf("MemoryBacking() error = %v", err)
	}
	if b.Known && b.Total == 0 {
		t.Error("MemoryBacking() reported a known backing store of zero size")
	}
	t.Logf("known=%v total=%s free=%s", b.Known, FormatSize(b.Total), FormatSize(b.Free))
}
