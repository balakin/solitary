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
	"syscall"

	"github.com/balakin/solitary/internal/config"

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
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return contextFilesRoot(r, "")
}

func contextFilesRoot(root *os.Root, private string) ([]string, error) {
	var files []string
	err := fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != "." && (excluded[d.Name()] || config.SensitiveName(d.Name()) || path == private) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() && !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func openContext(containerfile string, boundary []string) (*os.Root, string, error) {
	contextDir, err := filepath.Abs(filepath.Dir(containerfile))
	if err != nil {
		return nil, "", err
	}
	rootDir := contextDir
	if len(boundary) > 0 && boundary[0] != "" {
		rootDir, err = filepath.Abs(boundary[0])
		if err != nil {
			return nil, "", err
		}
	}
	anchor, err := os.OpenRoot(rootDir)
	if err != nil {
		return nil, "", err
	}
	defer anchor.Close()
	rel, err := filepath.Rel(rootDir, contextDir)
	if err != nil {
		return nil, "", err
	}
	root, err := anchor.OpenRoot(rel)
	if err != nil {
		return nil, "", fmt.Errorf("build context must remain inside the cell directory: %w", err)
	}
	file, err := openContextFile(root, filepath.Base(containerfile))
	if err != nil {
		root.Close()
		return nil, "", fmt.Errorf("opening Containerfile: %w", err)
	}
	file.Close()
	private := ""
	if len(boundary) > 1 && boundary[1] != "" {
		privatePath, absErr := filepath.Abs(boundary[1])
		if absErr != nil {
			root.Close()
			return nil, "", absErr
		}
		private, err = filepath.Rel(contextDir, privatePath)
		if err != nil {
			root.Close()
			return nil, "", err
		}
		private = filepath.ToSlash(private)
	}
	return root, private, nil
}

func openContextFile(root *os.Root, name string) (*os.File, error) {
	f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("%s is not a regular build file", name)
	}
	return f, nil
}

// ContextDigest summarises a build context: the paths it contains and their
// contents. Two contexts with the same digest produce the same image, so a
// build can be skipped when it has not moved.
func ContextDigest(containerfile string, boundary ...string) (string, error) {
	root, private, err := openContext(containerfile, boundary)
	if err != nil {
		return "", err
	}
	defer root.Close()
	files, err := contextFilesRoot(root, private)
	if err != nil {
		return "", err
	}

	sum := sha256.New()
	for _, rel := range files {
		fmt.Fprintf(sum, "%s\x00", rel)

		f, err := openContextFile(root, rel)
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
	r, err := os.OpenRoot(root)
	if err != nil {
		return "", err
	}
	defer r.Close()
	return stageRoot(r, files)
}

func stageRoot(root *os.Root, files []string) (string, error) {
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
		f, err := openContextFile(root, rel)
		if err != nil {
			os.RemoveAll(dir)
			return "", err
		}
		data, err := io.ReadAll(f)
		f.Close()
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
//
// fresh builds as though nothing had been built before: no cached layer, and
// the base image pulled again. A Containerfile is rarely a pure function of its
// context — an unpinned package, a `latest` anything — and a cached layer is
// the build remembering what that resolved to the first time.
func Build(instance, containerfile, tag, digest string, fresh bool, boundary ...string) error {
	root, private, err := openContext(containerfile, boundary)
	if err != nil {
		return err
	}
	defer root.Close()
	files, err := contextFilesRoot(root, private)
	if err != nil {
		return err
	}
	staged, err := stageRoot(root, files)
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

	args := []string{
		"podman", "build",
		"--tag", tag,
		"--label", buildLabel + "=" + digest,
		"--file", buildDir + "/" + filepath.Base(containerfile),
	}
	if fresh {
		args = append(args, "--no-cache", "--pull=always")
	}
	buildErr := lima.Attach(instance, append(args, buildDir)...)

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
