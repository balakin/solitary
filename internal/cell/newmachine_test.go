package cell

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeArchive(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := tar.NewWriter(&buf)
	for _, name := range names {
		body := bytes.Repeat([]byte(name), 4096)
		if err := w.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: name, Mode: 0o600, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// The old machine is destroyed on the strength of this count, so a stream cut
// off anywhere — inside a file, between two, or inside the end tar writes —
// must fail rather than count what arrived.
func TestCountEntriesRefusesACutOffArchive(t *testing.T) {
	data := writeArchive(t, "./a", "./b", "./c/d")
	path := filepath.Join(t.TempDir(), "home.tar")

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if n, err := countEntries(path); err != nil || n != 3 {
		t.Fatalf("countEntries(whole) = %d, %v, want 3", n, err)
	}

	// Each file is a 512-byte header and its body, a multiple of 512 here,
	// so the first file ends where the second header begins.
	between := 512 + len(bytes.Repeat([]byte("./a"), 4096))
	for _, cut := range []int{len(data) / 2, between, len(data) - 512} {
		if err := os.WriteFile(path, data[:cut], 0o600); err != nil {
			t.Fatal(err)
		}
		if n, err := countEntries(path); err == nil {
			t.Errorf("countEntries(cut at %d of %d) = %d, want an error", cut, len(data), n)
		}
	}
}

// Every up finishes a move, not only the command that started it: a waiting
// home is found by name, beside the cell's other state.
func TestHomeArchivesLiveInTheStateDirectory(t *testing.T) {
	isolate(t)
	partial, saved, pending, err := homeArchives("probe")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(pending) != "probe.home.tar" || partial != pending+".partial" || saved != pending+".saved" {
		t.Errorf("homeArchives() = %q, %q, %q", partial, saved, pending)
	}
	if exists(pending) {
		t.Error("a cell that never moved has a home waiting")
	}
}
