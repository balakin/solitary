package podman

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/balakin/solitary/internal/lima"
)

// buildLabel records a digest of the build context an image was built from, so
// that a changed Containerfile is noticed without rebuilding to find out.
const buildLabel = "solitary.build"

// buildDir is where a build context is put inside the machine.
const buildDir = "/tmp/solitary-build"

// excluded names never form part of a build context.
//
// The first group is solitary's own files. A cell's directory holds its
// secrets, and those must not reach the machine's disk or be readable by a
// Containerfile. The rest are large and irrelevant to a build.
var excluded = map[string]bool{
	".env":         true,
	"cell.yaml":    true,
	".git":         true,
	"node_modules": true,
}

// contextFiles lists the files that make up a build context, relative to its
// root, in a stable order. It is the single definition of what a build sees,
// used both to digest a context and to copy it.
func contextFiles(root string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if excluded[d.Name()] {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading build context %s: %w", root, err)
	}
	sort.Strings(files)

	return files, nil
}

// ContextDigest summarises a build context: the paths it contains and their
// contents. Two contexts with the same digest produce the same image, so a
// build can be skipped when it has not moved.
func ContextDigest(containerfile string) (string, error) {
	root := filepath.Dir(containerfile)

	files, err := contextFiles(root)
	if err != nil {
		return "", err
	}

	sum := sha256.New()
	for _, rel := range files {
		fmt.Fprintf(sum, "%s\x00", rel)

		f, err := os.Open(filepath.Join(root, rel))
		if err != nil {
			return "", fmt.Errorf("reading %s: %w", rel, err)
		}
		_, err = io.Copy(sum, f)
		f.Close()
		if err != nil {
			return "", fmt.Errorf("reading %s: %w", rel, err)
		}
	}

	return hex.EncodeToString(sum.Sum(nil)), nil
}

// stage copies a build context into a temporary directory containing only the
// files a build is allowed to see. Copying the cell's directory as-is would put
// its secrets on the machine's disk.
//
// The caller removes the returned directory.
func stage(root string, files []string) (string, error) {
	dir, err := os.MkdirTemp("", "solitary-build-")
	if err != nil {
		return "", fmt.Errorf("creating a staging directory: %w", err)
	}

	for _, rel := range files {
		dst := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			os.RemoveAll(dir)
			return "", fmt.Errorf("staging %s: %w", rel, err)
		}
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			os.RemoveAll(dir)
			return "", fmt.Errorf("staging %s: %w", rel, err)
		}
		if err := os.WriteFile(dst, data, 0o600); err != nil {
			os.RemoveAll(dir)
			return "", fmt.Errorf("staging %s: %w", rel, err)
		}
	}

	return dir, nil
}

// BuiltDigest returns the context digest an image was built from, or an empty
// string when the image does not exist or was not built by solitary.
func BuiltDigest(instance, tag string) (string, error) {
	out, err := lima.Exec(instance,
		"podman", "image", "inspect", tag,
		"--format", "{{index .Config.Labels \""+buildLabel+"\"}}",
	)
	if err != nil {
		if isCommandFailure(err) {
			return "", nil
		}
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

// Build copies a build context into the machine and builds it there. The host
// never runs the build, so a Containerfile cannot execute anything outside the
// cell it is for.
func Build(instance, containerfile, tag, digest string) error {
	root := filepath.Dir(containerfile)

	files, err := contextFiles(root)
	if err != nil {
		return err
	}
	staged, err := stage(root, files)
	if err != nil {
		return err
	}
	defer os.RemoveAll(staged)

	// Start from nothing, so a file deleted on the host does not linger in
	// the machine and end up in the image — and so that the copy lands where
	// this expects it to.
	//
	// The directory is deliberately not created first. lima copies with scp,
	// which puts a directory *inside* a destination that already exists and
	// *at* one that does not, and the two are only the same thing on scp
	// versions new enough to have stopped doing the first. OpenSSH 9.x, which
	// is what Debian 12 and Ubuntu 24.04 ship, does the first: the context
	// lands in a subdirectory named after the staging directory, and the
	// build fails on a Containerfile that is not where it was put. Copying to
	// a path that does not exist yet means the same thing everywhere.
	if _, err := lima.Exec(instance, "rm", "-rf", buildDir); err != nil {
		return fmt.Errorf("clearing the build directory: %w", err)
	}
	if err := lima.Copy(staged, instance, buildDir); err != nil {
		return fmt.Errorf("copying the build context into the machine: %w", err)
	}

	buildErr := lima.Attach(instance,
		"podman", "build",
		"--tag", tag,
		"--label", buildLabel+"="+digest,
		"--file", buildDir+"/"+filepath.Base(containerfile),
		buildDir,
	)

	// Clear the context whether or not the build worked, rather than leaving
	// a copy of it in the machine.
	if _, err := lima.Exec(instance, "rm", "-rf", buildDir); err != nil && buildErr == nil {
		return fmt.Errorf("clearing the build directory: %w", err)
	}

	if buildErr != nil {
		return fmt.Errorf("building %s: %w", tag, buildErr)
	}

	return nil
}
