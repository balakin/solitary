// Package update replaces the running binary with a newer release, and tells
// the user when one exists.
//
// It only ever manages an install that came from the release archives — by
// hand, or through the install script the site serves. A binary a package
// manager put there belongs to that package manager, and is left alone.
package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	repo = "balakin/solitary"

	// Releases is where a version that cannot be installed automatically is
	// found by hand.
	Releases = "https://github.com/" + repo + "/releases"

	// archives are small; this caps a download that stalls rather than fails.
	maxArchive = 64 << 20
)

// Endpoints are variables rather than constants so tests can serve them
// locally: everything below is otherwise one round trip to github.com.
var (
	latestURL   = "https://api.github.com/repos/" + repo + "/releases/latest"
	downloadURL = "https://github.com/" + repo + "/releases/download"
)

// ErrManaged is returned when the running binary belongs to a package manager,
// which has to be the one to replace it.
var ErrManaged = errors.New("this install is managed by a package manager")

// Latest is the version of the newest published release, without the leading v
// its tag carries.
func Latest(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestURL, nil)
	if err != nil {
		return "", fmt.Errorf("preparing release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("asking github for the latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("asking github for the latest release: %s", resp.Status)
	}

	var release struct {
		Tag string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&release); err != nil {
		return "", fmt.Errorf("reading the latest release: %w", err)
	}
	if release.Tag == "" {
		return "", errors.New("reading the latest release: no tag in the answer")
	}

	return strings.TrimPrefix(release.Tag, "v"), nil
}

// Install downloads the release archive for version and replaces the running
// binary with the one inside it.
func Install(ctx context.Context, version string) error {
	path, err := executable()
	if err != nil {
		return err
	}
	if manager := managedBy(path); manager != "" {
		return fmt.Errorf("%w (%s): upgrade it with: %s", ErrManaged, path, manager)
	}

	return install(ctx, version, path)
}

// install is Install once the binary to replace is known, which is what tests
// vary: the one Install picks is the test binary itself.
func install(ctx context.Context, version, path string) error {
	dir := filepath.Dir(path)
	// Writability is checked by writing, since that is the only answer that
	// counts, and the staged file has to live here anyway: a rename is only
	// atomic within one filesystem, and replacing the binary while it runs
	// is exactly what a rename can do and a copy cannot.
	staged, err := os.CreateTemp(dir, ".solitary-update-*")
	if err != nil {
		return fmt.Errorf("staging the download in %s: %w (install it there as a user who can write to it, or reinstall elsewhere: %s)", dir, err, Releases)
	}
	defer os.Remove(staged.Name()) // a no-op once the rename below succeeded

	name := fmt.Sprintf("solitary_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	base := downloadURL + "/v" + version

	archive, err := download(ctx, base+"/"+name)
	if err != nil {
		staged.Close()
		return err
	}

	sums, err := download(ctx, base+"/checksums.txt")
	if err != nil {
		staged.Close()
		return err
	}
	if err := verify(archive, sums, name); err != nil {
		staged.Close()
		return err
	}

	if err := extract(archive, staged); err != nil {
		staged.Close()
		return err
	}
	if err := staged.Close(); err != nil {
		return fmt.Errorf("writing the download: %w", err)
	}
	if err := os.Chmod(staged.Name(), 0o755); err != nil {
		return fmt.Errorf("making the download executable: %w", err)
	}
	if err := os.Rename(staged.Name(), path); err != nil {
		return fmt.Errorf("replacing %s: %w", path, err)
	}

	return nil
}

// executable resolves the running binary through any symlinks, so that what is
// replaced is the file itself rather than a link to it.
func executable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locating the running binary: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", path, err)
	}
	return resolved, nil
}

// managedBy names the command that owns an install, or "" when nothing does.
// Homebrew is the only one that ships solitary. Its binaries are reached through
// a symlink, which executable has already resolved: into a Cellar for the
// formula this tap publishes, or a Caskroom for anything installed as a cask.
func managedBy(path string) string {
	if strings.Contains(path, string(filepath.Separator)+"Cellar"+string(filepath.Separator)) ||
		strings.Contains(path, string(filepath.Separator)+"Caskroom"+string(filepath.Separator)) {
		return "brew upgrade solitary"
	}
	return ""
}

func download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("preparing the download: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading %s: %s", url, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxArchive))
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", url, err)
	}

	return body, nil
}

// verify checks the archive against the published checksum. It is the only
// thing standing between a release download and whatever answered the request.
func verify(archive, sums []byte, name string) error {
	sum := sha256.Sum256(archive)
	got := hex.EncodeToString(sum[:])

	for line := range strings.Lines(string(sums)) {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[1] != name {
			continue
		}
		if fields[0] != got {
			return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", name, fields[0], got)
		}
		return nil
	}

	return fmt.Errorf("checksums.txt names no %s", name)
}

// extract writes the solitary binary out of the release archive, which holds
// nothing else that matters.
func extract(archive []byte, w io.Writer) error {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return fmt.Errorf("reading the archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return errors.New("the archive holds no solitary binary")
		}
		if err != nil {
			return fmt.Errorf("reading the archive: %w", err)
		}
		if header.Name != "solitary" {
			continue
		}
		if _, err := io.Copy(w, io.LimitReader(tr, maxArchive)); err != nil {
			return fmt.Errorf("writing the download: %w", err)
		}
		return nil
	}
}
