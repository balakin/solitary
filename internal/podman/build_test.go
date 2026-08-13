package podman

import (
	"os"
	"path/filepath"
	"testing"
)

// writeContext lays out a build context and returns the Containerfile path.
func writeContext(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	return filepath.Join(root, "Containerfile")
}

func digestOf(t *testing.T, containerfile string) string {
	t.Helper()

	got, err := ContextDigest(containerfile)
	if err != nil {
		t.Fatalf("ContextDigest() error = %v", err)
	}

	return got
}

func TestContextDigestIsStable(t *testing.T) {
	files := map[string]string{
		"Containerfile":    "FROM ubuntu:24.04\n",
		"scripts/setup.sh": "echo hello\n",
	}

	first := digestOf(t, writeContext(t, files))
	second := digestOf(t, writeContext(t, files))

	if first != second {
		t.Errorf("identical contexts produced different digests:\n%s\n%s", first, second)
	}
}

func TestContextDigestChangesWithContents(t *testing.T) {
	before := digestOf(t, writeContext(t, map[string]string{
		"Containerfile": "FROM ubuntu:24.04\n",
	}))
	after := digestOf(t, writeContext(t, map[string]string{
		"Containerfile": "FROM debian:bookworm\n",
	}))

	if before == after {
		t.Error("editing the Containerfile did not change the digest")
	}
}

func TestContextDigestChangesWithSupportingFiles(t *testing.T) {
	base := map[string]string{
		"Containerfile":    "FROM ubuntu:24.04\nCOPY setup.sh /\n",
		"scripts/setup.sh": "echo hello\n",
	}
	before := digestOf(t, writeContext(t, base))

	changed := map[string]string{
		"Containerfile":    base["Containerfile"],
		"scripts/setup.sh": "echo goodbye\n",
	}
	after := digestOf(t, writeContext(t, changed))

	if before == after {
		t.Error("editing a file in the context did not change the digest")
	}
}

func TestContextDigestIgnoresSkippedDirectories(t *testing.T) {
	withoutGit := digestOf(t, writeContext(t, map[string]string{
		"Containerfile": "FROM ubuntu:24.04\n",
	}))
	withGit := digestOf(t, writeContext(t, map[string]string{
		"Containerfile":  "FROM ubuntu:24.04\n",
		".git/HEAD":      "ref: refs/heads/main\n",
		"node_modules/x": "irrelevant\n",
	}))

	if withoutGit != withGit {
		t.Error("digest changed because of files that are not part of a build")
	}
}

// A cell's directory is its build context and also holds its secrets. Those
// must never be part of a build: they would reach the machine's disk and be
// readable by the Containerfile.
func TestBuildContextExcludesCellFiles(t *testing.T) {
	containerfile := writeContext(t, map[string]string{
		"Containerfile": "FROM ubuntu:24.04\n",
		"cell.yaml":     "build: ./Containerfile\n",
		"lima.yaml":     "cpus: 2\n",
		".env":          "CLAUDE_API_KEY=\"sk-ant-secret\"\n",
		"setup.sh":      "echo hello\n",
	})
	root := filepath.Dir(containerfile)

	files, err := contextFiles(root)
	if err != nil {
		t.Fatalf("contextFiles() error = %v", err)
	}

	for _, rel := range files {
		if excluded[rel] {
			t.Errorf("build context includes %s, which must never reach a build", rel)
		}
	}

	staged, err := stage(root, files)
	if err != nil {
		t.Fatalf("stage() error = %v", err)
	}
	defer os.RemoveAll(staged)

	for _, name := range []string{".env", "cell.yaml", "lima.yaml"} {
		if _, err := os.Stat(filepath.Join(staged, name)); err == nil {
			t.Errorf("staged context contains %s", name)
		}
	}
	for _, name := range []string{"Containerfile", "setup.sh"} {
		if _, err := os.Stat(filepath.Join(staged, name)); err != nil {
			t.Errorf("staged context is missing %s: %v", name, err)
		}
	}
}

// Rotating a secret must not look like a change to the image.
func TestContextDigestIgnoresSecrets(t *testing.T) {
	before := digestOf(t, writeContext(t, map[string]string{
		"Containerfile": "FROM ubuntu:24.04\n",
		".env":          "TOKEN=\"one\"\n",
	}))
	after := digestOf(t, writeContext(t, map[string]string{
		"Containerfile": "FROM ubuntu:24.04\n",
		".env":          "TOKEN=\"two\"\n",
	}))

	if before != after {
		t.Error("changing a secret changed the build context digest")
	}
}

func TestContextDigestChangesWhenAFileMoves(t *testing.T) {
	before := digestOf(t, writeContext(t, map[string]string{
		"Containerfile": "FROM ubuntu:24.04\n",
		"a/setup.sh":    "echo hello\n",
	}))
	after := digestOf(t, writeContext(t, map[string]string{
		"Containerfile": "FROM ubuntu:24.04\n",
		"b/setup.sh":    "echo hello\n",
	}))

	if before == after {
		t.Error("moving a file did not change the digest")
	}
}
