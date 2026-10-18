package update

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	sigverify "github.com/sigstore/sigstore-go/pkg/verify"
)

const (
	signingIdentity = "https://github.com/balakin/solitary/.github/workflows/release.yml@refs/heads/main"
	signingIssuer   = "https://token.actions.githubusercontent.com"
)

func secureClient() *http.Client {
	client := *http.DefaultClient
	previous := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return fmt.Errorf("refusing redirect to non-HTTPS URL")
		}
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		if previous != nil {
			return previous(req, via)
		}
		return nil
	}
	if client.Timeout == 0 {
		client.Timeout = 30 * time.Second
	}
	return &client
}

// verifyRelease binds the signed checksum manifest to the release workflow,
// Sigstore trust roots and transparency log. Archive names include the version
// so an older signed manifest cannot satisfy a request for a different release.
func verifyRelease(ctx context.Context, manifest, signature []byte) error {
	var signed bundle.Bundle
	if err := json.Unmarshal(signature, &signed); err != nil {
		return fmt.Errorf("invalid signature bundle: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	opts := tuf.DefaultOptions().WithContext(ctx)
	opts.DisableLocalCache = true
	trusted, err := root.FetchTrustedRootWithOptions(opts)
	if err != nil {
		return fmt.Errorf("loading Sigstore trust roots: %w", err)
	}
	digest := sha256.Sum256(manifest)
	return verifySignedDigest(trusted, &signed, signingIdentity, "sha256", digest[:])
}

func verifySignedDigest(trusted root.TrustedMaterial, signed *bundle.Bundle, expectedIdentity, algorithm string, digest []byte) error {
	identity, err := sigverify.NewShortCertificateIdentity(signingIssuer, "", expectedIdentity, "")
	if err != nil {
		return err
	}
	verifier, err := sigverify.NewVerifier(trusted, sigverify.WithSignedCertificateTimestamps(1), sigverify.WithTransparencyLog(1), sigverify.WithObserverTimestamps(1))
	if err != nil {
		return err
	}
	_, err = verifier.Verify(signed, sigverify.NewPolicy(sigverify.WithArtifactDigest(algorithm, digest), sigverify.WithCertificateIdentity(identity)))
	if err != nil {
		return fmt.Errorf("release signature verification failed: %w", err)
	}
	return nil
}
