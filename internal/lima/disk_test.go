package lima

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// qcow2Header builds the first 32 bytes of an image presenting size bytes to
// its guest, which is all qcow2Size reads.
func qcow2Header(size uint64) []byte {
	header := make([]byte, 32)
	copy(header, qcow2Magic)
	header[7] = 3 // version
	binary.BigEndian.PutUint64(header[24:32], size)

	return header
}

func TestQcow2SizeReadsTheHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disk")
	const want = 50 << 30
	if err := os.WriteFile(path, qcow2Header(want), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := qcow2Size(path)
	if err != nil {
		t.Fatalf("qcow2Size() error = %v", err)
	}
	if got != want {
		t.Errorf("qcow2Size() = %d, want %d", got, want)
	}
}

// A size that cannot be read is not an error: the caller skips its check
// rather than refusing to start a machine over a file it does not recognise.
func TestQcow2SizeIsZeroForWhatItCannotRead(t *testing.T) {
	dir := t.TempDir()

	raw := filepath.Join(dir, "raw")
	if err := os.WriteFile(raw, make([]byte, 64), 0o600); err != nil {
		t.Fatal(err)
	}
	short := filepath.Join(dir, "short")
	if err := os.WriteFile(short, qcow2Header(1 << 30)[:8], 0o600); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{raw, short, filepath.Join(dir, "absent")} {
		size, err := qcow2Size(path)
		if err != nil {
			t.Errorf("qcow2Size(%s) error = %v", filepath.Base(path), err)
		}
		if size != 0 {
			t.Errorf("qcow2Size(%s) = %d, want 0", filepath.Base(path), size)
		}
	}
}
