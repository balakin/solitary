package update

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sigstore/sigstore-go/pkg/testing/data"
)

// Upstream's real public-good Sigstore fixture exercises certificate, log,
// signature, identity and artifact binding entirely offline.
func TestSignatureBindsArtifactAndWorkflow(t *testing.T) {
	trusted := data.TrustedRoot(t, "public-good.json")
	signed := data.Bundle(t, "sigstore.js@2.0.0-provenance.sigstore.json")
	digest, err := hex.DecodeString("46d4e2f74c4877316640000a6fdf8a8b59f1e0847667973e9859f774dd31b8f1e0937813b777fb66a2ac67d50540fe34640966eee9fc2ccca387082b4c85cd3c")
	if err != nil {
		t.Fatal(err)
	}
	identity := "https://github.com/sigstore/sigstore-js/.github/workflows/release.yml@refs/heads/main"
	if err := verifySignedDigest(trusted, signed, identity, "sha512", digest); err != nil {
		t.Fatal(err)
	}
	if err := verifySignedDigest(trusted, signed, signingIdentity, "sha512", digest); err == nil {
		t.Fatal("accepted a different project's release identity")
	}
	digest[0] ^= 1
	if err := verifySignedDigest(trusted, signed, identity, "sha512", digest); err == nil {
		t.Fatal("accepted a changed artifact")
	}
}

func TestSignatureRejectsMissingBundle(t *testing.T) {
	for _, body := range [][]byte{nil, []byte(`{}`), []byte(`{"messageSignature":{}}`)} {
		if err := verifyRelease(t.Context(), []byte("manifest"), body); err == nil {
			t.Fatal("accepted an unsigned manifest")
		}
	}
}

func TestInstallKeepsBinaryWhenSignatureFails(t *testing.T) {
	serve(t, "0.2.0", "new binary", intact)
	releaseVerifier = func(context.Context, []byte, []byte) error { return errors.New("wrong workflow identity") }
	path := filepath.Join(t.TempDir(), "solitary")
	if err := os.WriteFile(path, []byte("old binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := install(t.Context(), "0.2.0", path); err == nil {
		t.Fatal("installed an unauthenticated archive")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "old binary" {
		t.Fatalf("old binary was changed: %q, %v", got, err)
	}
}
