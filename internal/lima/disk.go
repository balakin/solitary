package lima

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// qcow2Magic opens every qcow2 image, and nothing else.
const qcow2Magic = "QFI\xfb"

// DiskSize reports how large the disk a machine already has is, in bytes.
//
// Lima grows a machine's disk to what its definition asks for when it starts,
// and refuses to shrink it, so this is the size a new vm.disk has to be
// measured against. The definition the machine stores is not that size: it is
// what was asked for last, which is exactly the thing that may be wrong.
//
// A machine with no disk yet, one in a format this does not know, and one Lima
// cannot be asked about all report zero. Every caller does the same thing with
// an unknown size — nothing — so there is no error to tell apart: a size that
// cannot be read is not a finding, and Lima still has the last word at start.
func DiskSize(name string) uint64 {
	dir, err := dirOf(name)
	if err != nil {
		return 0
	}

	size, err := qcow2Size(filepath.Join(dir, "disk"))
	if err != nil {
		return 0
	}

	return size
}

// qcow2Size reads the size an image presents to its guest out of the image's
// own header, rather than shelling out to qemu-img, which a host running Lima
// need not have.
func qcow2Size(path string) (uint64, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", path, err)
	}
	defer file.Close()

	// The qcow2 header opens with the magic and a version, and holds the
	// size of the disk it presents at offset 24, big endian. Anything too
	// short to hold that, or not carrying the magic, is not an image this
	// can answer for.
	var header [32]byte
	if _, err := io.ReadFull(file, header[:]); err != nil {
		return 0, nil
	}
	if string(header[:4]) != qcow2Magic {
		return 0, nil
	}

	return binary.BigEndian.Uint64(header[24:32]), nil
}
