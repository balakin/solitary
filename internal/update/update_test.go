package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// serve stands in for github: one release, one archive holding one binary, and
// the checksums file the archive is checked against.
func serve(t *testing.T, version, binary string, sum func(string) string) {
	t.Helper()

	archive := archiveOf(t, binary)
	name := fmt.Sprintf("solitary_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)

	mux := http.NewServeMux()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"tag_name": "v%s"}`, version)
	})
	mux.HandleFunc("/download/v"+version+"/"+name, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	})
	mux.HandleFunc("/download/v"+version+"/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		digest := sha256.Sum256(archive)
		fmt.Fprintf(w, "%s  %s\n", sum(hex.EncodeToString(digest[:])), name)
	})

	mux.HandleFunc("/download/v"+version+"/checksums.txt.sigstore.json", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("fixture-signature")) })
	server := httptest.NewTLSServer(mux)
	previousClient, previousVerifier := http.DefaultClient, releaseVerifier
	http.DefaultClient = server.Client()
	releaseVerifier = func(_ context.Context, _, signature []byte) error {
		if string(signature) != "fixture-signature" {
			return fmt.Errorf("missing fixture signature")
		}
		return nil
	}
	t.Cleanup(func() { http.DefaultClient, releaseVerifier = previousClient, previousVerifier })
	t.Cleanup(server.Close)

	latest, download := latestURL, downloadURL
	t.Cleanup(func() { latestURL, downloadURL = latest, download })
	latestURL, downloadURL = server.URL+"/latest", server.URL+"/download"
}

func archiveOf(t *testing.T, binary string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for _, name := range []string{"LICENSE", "solitary"} {
		body := "a license\n"
		if name == "solitary" {
			body = binary
		}
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func intact(sum string) string { return sum }

func TestLatest(t *testing.T) {
	serve(t, "0.2.0", "new binary", intact)

	got, err := Latest(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got != "0.2.0" {
		t.Errorf("Latest() = %q, want 0.2.0", got)
	}
}

func TestInstallReplacesTheBinary(t *testing.T) {
	serve(t, "0.2.0", "new binary", intact)

	path := filepath.Join(t.TempDir(), "solitary")
	if err := os.WriteFile(path, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := install(t.Context(), "0.2.0", path); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new binary" {
		t.Errorf("binary = %q, want %q", got, "new binary")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %v, want 755", info.Mode().Perm())
	}

	// Nothing of the download is left behind next to the binary.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("install left %d files behind, want just the binary", len(entries)-1)
	}
}

// A download that does not match its checksum is the case the whole exercise
// exists for: it must not reach the binary.
func TestInstallRejectsAChangedArchive(t *testing.T) {
	serve(t, "0.2.0", "tampered", func(string) string { return strings.Repeat("0", 64) })

	path := filepath.Join(t.TempDir(), "solitary")
	if err := os.WriteFile(path, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := install(t.Context(), "0.2.0", path)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("install error = %v, want a checksum mismatch", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old binary" {
		t.Errorf("binary = %q, want it untouched", got)
	}
}

func TestInstallReportsAMissingRelease(t *testing.T) {
	serve(t, "0.2.0", "new binary", intact)

	path := filepath.Join(t.TempDir(), "solitary")
	if err := os.WriteFile(path, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := install(t.Context(), "9.9.9", path); err == nil {
		t.Fatal("install of a version that does not exist succeeded")
	}
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// The archive is checked against a signed manifest, but a redirect to cleartext
// is refused anyway: nothing the updater fetches has a reason to leave HTTPS.
func TestDownloadRefusesARedirectToHTTP(t *testing.T) {
	previous := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) {
		status, header := http.StatusOK, make(http.Header)
		if r.URL.Scheme == "https" {
			status = http.StatusFound
			header.Set("Location", "http://release.example.invalid/archive")
		}
		return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader("archive")), Request: r}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = previous })

	if _, err := download(context.Background(), "https://release.example.invalid/archive"); err == nil {
		t.Error("download() followed a redirect from HTTPS to HTTP")
	}
}
