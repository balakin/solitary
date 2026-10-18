package safefile

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type raceReader struct {
	io.Reader
	before func()
}

func (r *raceReader) Read(p []byte) (int, error) {
	if r.before != nil {
		r.before()
		r.before = nil
	}
	return r.Reader.Read(p)
}

func TestDestinationSwapCannotRedirectWrite(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run(map[bool]string{false: "exclusive", true: "replace"}[force], func(t *testing.T) {
			dir := t.TempDir()
			victim := filepath.Join(t.TempDir(), "victim")
			if err := os.WriteFile(victim, []byte("unchanged"), 0o600); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			reader := &raceReader{Reader: strings.NewReader("published"), before: func() {
				if err := os.Symlink(victim, filepath.Join(dir, "report")); err != nil {
					t.Fatal(err)
				}
			}}
			err = Write(root, "report", reader, 0o644, force)
			if force && err != nil {
				t.Fatal(err)
			}
			if !force && err == nil {
				t.Fatal("clobbered a raced destination without force")
			}
			got, err := os.ReadFile(victim)
			if err != nil || string(got) != "unchanged" {
				t.Fatalf("followed destination symlink: %q %v", got, err)
			}
		})
	}
}
