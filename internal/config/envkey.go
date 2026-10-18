package config

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// EnvKey is a host-only HMAC key. Labels cannot expose a reusable hash for
// guessing low-entropy secrets without access to this private state file.
func EnvKey() ([]byte, error) {
	dir, err := StateDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "env-key")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			f.Close()
			return nil, err
		}
		_, err = f.Write(key)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
	} else if !os.IsExist(err) {
		return nil, err
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	info, err := r.Lstat("env-key")
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%s must be a private regular file (0600)", path)
	}
	f, err = r.Open("env-key")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	key, err := io.ReadAll(io.LimitReader(f, 33))
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid environment key in %s", path)
	}
	return key, nil
}
