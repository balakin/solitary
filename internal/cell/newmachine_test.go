package cell

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func writeArchive(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	z := gzip.NewWriter(&buf)
	w := tar.NewWriter(z)
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
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// The old machine is destroyed on the strength of this count, so a stream cut
// off anywhere — inside a file, or past the last one but before gzip's
// checksum — must fail rather than count what arrived.
func TestCountEntriesRefusesACutOffArchive(t *testing.T) {
	data := writeArchive(t, "./a", "./b", "./c/d")
	path := filepath.Join(t.TempDir(), "home.tar.gz")

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if n, err := countEntries(path); err != nil || n != 3 {
		t.Fatalf("countEntries(whole) = %d, %v, want 3", n, err)
	}

	for _, cut := range []int{len(data) / 2, len(data) - 4} {
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
	if filepath.Base(pending) != "probe.home.tar.gz" || partial != pending+".partial" || saved != pending+".saved" {
		t.Errorf("homeArchives() = %q, %q, %q", partial, saved, pending)
	}
	if exists(pending) {
		t.Error("a cell that never moved has a home waiting")
	}
}
