package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The examples in examples/ are published for solitary clone to install, and
// clone parses a definition with CheckCell before it becomes a cell. So an
// example that drifts from the schema is a broken clone rather than a broken
// build, and nothing else in the repository would notice.
func TestExamplesParse(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("..", "..", "examples", "*", "cell.yaml"))
	if err != nil {
		t.Fatalf("looking for examples: %v", err)
	}
	if len(dirs) == 0 {
		t.Fatal("no examples found: examples/*/cell.yaml")
	}

	for _, path := range dirs {
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}

			cell, err := CheckCell(data, filepath.Dir(path))
			if err != nil {
				t.Fatalf("%s does not parse: %v", path, err)
			}

			// Every example builds its own image, so that what it
			// installs can be read beside the definition.
			if cell.Build == "" {
				t.Fatalf("%s: expected a build, got image %q", path, cell.Image)
			}
			if _, err := os.Stat(cell.BuildPath); err != nil {
				t.Errorf("%s names a Containerfile that is not there: %v", path, err)
			}
		})
	}
}

// A cell's name is its directory name, so an example directory that is not a
// usable name cannot be cloned without --as.
func TestExampleNamesAreUsable(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("..", "..", "examples", "*", "cell.yaml"))
	if err != nil {
		t.Fatalf("looking for examples: %v", err)
	}

	for _, path := range dirs {
		name := filepath.Base(filepath.Dir(path))
		if err := ValidateName(name); err != nil {
			t.Errorf("examples/%s: %v", name, err)
		}
	}
}
