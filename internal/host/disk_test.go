package host

import "testing"

func TestDiskFree(t *testing.T) {
	free, err := DiskFree(t.TempDir())
	if err != nil {
		t.Fatalf("DiskFree() error = %v", err)
	}
	if free == 0 {
		t.Error("DiskFree() = 0 for a directory a test just wrote to")
	}

	if _, err := DiskFree("/no/such/path/for/solitary"); err == nil {
		t.Error("DiskFree() error = nil for a path that does not exist")
	}
}
