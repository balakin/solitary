package update

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallerRequiresAuthenticatedManifest(t *testing.T) {
	for _, reject := range []bool{false, true} {
		t.Run(fmt.Sprint("reject=", reject), func(t *testing.T) {
			dir := t.TempDir()
			archive := archiveOf(t, "#!/bin/sh\necho 'solitary version fixture'\n")
			name := fmt.Sprintf("solitary_1.2.3_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
			sum := sha256.Sum256(archive)
			for file, body := range map[string][]byte{
				name:                          archive,
				"checksums.txt":               []byte(fmt.Sprintf("%x  %s\n", sum, name)),
				"checksums.txt.sigstore.json": []byte("transport fixture; crypto tested separately"),
				"curl": []byte(`#!/bin/sh
printf '%s\n' "$@" >> "$FAKE_CURL_LOG"
while [ "$#" -gt 0 ]; do
 case "$1" in -o) shift; output=$1 ;; https://*) url=$1 ;; esac
 shift
done
cat "$FAKE_RELEASE/${url##*/}" > "$output"
`),
				"cosign": []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$FAKE_SIGN_LOG\"\nexit \"$FAKE_SIGN_EXIT\"\n"),
			} {
				if err := os.WriteFile(filepath.Join(dir, file), body, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			installDir := filepath.Join(dir, "install")
			if err := os.Mkdir(installDir, 0o700); err != nil {
				t.Fatal(err)
			}
			binary := filepath.Join(installDir, "solitary")
			if err := os.WriteFile(binary, []byte("old binary"), 0o700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("sh", filepath.Join("..", "..", "website", "public", "install.sh"))
			exit := "0"
			if reject {
				exit = "1"
			}
			cmd.Env = append(os.Environ(), "PATH="+dir+":/usr/bin:/bin", "SOLITARY_INSTALL_DIR="+installDir, "SOLITARY_VERSION=v1.2.3", "FAKE_RELEASE="+dir, "FAKE_CURL_LOG="+filepath.Join(dir, "curl.log"), "FAKE_SIGN_LOG="+filepath.Join(dir, "sign.log"), "FAKE_SIGN_EXIT="+exit)
			out, err := cmd.CombinedOutput()
			if reject && err == nil {
				t.Fatal("installer accepted a rejected signature")
			}
			if !reject && err != nil {
				t.Fatalf("installer failed: %v\n%s", err, out)
			}
			content, err := os.ReadFile(binary)
			if err != nil {
				t.Fatal(err)
			}
			if reject && string(content) != "old binary" {
				t.Fatal("installer changed binary before signature verification")
			}
			if !reject && !strings.Contains(string(content), "fixture") {
				t.Fatal("installer did not publish authenticated archive")
			}
			args, err := os.ReadFile(filepath.Join(dir, "sign.log"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(args), signingIdentity) || !strings.Contains(string(args), signingIssuer) {
				t.Fatal("installer omitted the pinned workflow identity")
			}
			args, err = os.ReadFile(filepath.Join(dir, "curl.log"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(args), "--proto-redir\n=https") {
				t.Fatal("installer permits protocol downgrade")
			}
		})
	}
}
