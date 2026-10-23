//go:build linux || darwin

package cell

import (
	"errors"
	"os"
	"syscall"
)

// tryLock takes an exclusive flock on f without waiting for it. flock rather
// than a file that exists or not: the kernel releases it when the process
// ends, so a command that was killed never leaves a cell locked behind it.
func tryLock(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return errLocked
	}
	return err
}
