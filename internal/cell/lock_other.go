//go:build !linux && !darwin

package cell

import "os"

// tryLock does nothing where there is no flock: solitary is not released
// for those systems, and they only have to build.
func tryLock(*os.File) error { return nil }
