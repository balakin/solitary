package cell

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/balakin/solitary/internal/config"
)

// errLocked is what tryLock reports for a lock someone else holds.
var errLocked = errors.New("locked")

// lockCell keeps every other solitary command off a cell until unlock is
// called: whatever changes a cell's machine — up, patch, a move, down, rm —
// assumes nothing else is changing it at the same time, and a move emptying
// a home while another command starts a container over it loses the home.
//
// It refuses rather than waits. Someone who started a second command did not
// know the first was running, and a command that sits silent for the minutes a
// build or a move takes looks hung.
func lockCell(name string) (unlock func(), err error) {
	// The name becomes part of a path.
	if err := config.ValidateName(name); err != nil {
		return nil, err
	}
	path, err := config.LockFile(name)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}

	if err := tryLock(f); err != nil {
		f.Close()
		if !errors.Is(err, errLocked) {
			return nil, fmt.Errorf("locking %s: %w", path, err)
		}
		holder := ""
		if data, err := os.ReadFile(path); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
				holder = fmt.Sprintf(" (process %d)", pid)
			}
		}
		return nil, fmt.Errorf("cell %q is busy with another solitary command%s; try again once it has finished", name, holder)
	}

	// Who holds it, for the message above. Only informative: the lock is
	// the kernel's, and it goes with the process however that process ends.
	if err := f.Truncate(0); err == nil {
		_, _ = f.WriteAt([]byte(strconv.Itoa(os.Getpid())+"\n"), 0)
	}

	return func() {
		_ = f.Truncate(0)
		f.Close()
	}, nil
}
