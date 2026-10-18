// Package safefile confines reads and publishes files without following targets.
package safefile

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Write atomically publishes into an opened directory. Existing symlinks are
// refused; a last-moment symlink replacement is replaced, never followed.
func Write(dir *os.Root, name string, source io.Reader, mode fs.FileMode, replace bool) error {
	if filepath.Base(name) != name {
		return fmt.Errorf("invalid file name %q", name)
	}
	if info, err := dir.Lstat(name); err == nil {
		if !replace {
			return fmt.Errorf("%s already exists", name)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", name)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	tmp := ".solitary-" + rand.Text()
	f, err := dir.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = dir.Remove(tmp) }()
	_, copyErr := io.Copy(f, source)
	if copyErr == nil {
		copyErr = f.Chmod(mode)
	}
	if copyErr == nil {
		copyErr = f.Sync()
	}
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if replace {
		return dir.Rename(tmp, name)
	}
	// Link fails atomically if any destination already exists, even after the
	// initial check. The staging file is on the same filesystem.
	return dir.Link(tmp, name)
}

func WriteFile(path string, data []byte, mode fs.FileMode) error {
	dir, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return Write(dir, filepath.Base(path), bytes.NewReader(data), mode, true)
}
