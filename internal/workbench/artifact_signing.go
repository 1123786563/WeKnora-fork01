package workbench

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ArtifactGrant is the authorization fact embedded in a short-lived artifact
// download link. The signature binds tenant, session, message and artifact
// index together with the expiry, so a leaked URL cannot be replayed against
// a different artifact, tenant, or after expiry. The mobile client treats a
// rejected grant as "re-authorize" and asks for a fresh signed URL through the
// authenticated endpoint — the download URL itself never carries credentials.
type ArtifactGrant struct {
	TenantID  uint64
	SessionID string
	MessageID string
	Index     int
	// ExpiresAt is unix seconds; enforced by VerifyArtifactGrantAt.
	ExpiresAt int64
}

// MaxArtifactGrantTTL caps how far in the future a grant may be issued. The
// signer refuses longer windows so a signed link stays short-lived even if a
// caller passes a large duration.
const MaxArtifactGrantTTL = 15 * time.Minute

// ErrArtifactSigningKeyMissing is returned when the deployment has not
// configured WEKNORA_ARTIFACT_SIGNING_KEY. The signing endpoints fail closed
// (501) instead of minting links with a default key.
var ErrArtifactSigningKeyMissing = errors.New("artifact signing key not configured")

// ArtifactSigningKeyEnvVar is the only source of the HMAC secret. It must hold
// at least 32 bytes of entropy; values are read from the environment at
// request time so key rotation does not require a restart.
const ArtifactSigningKeyEnvVar = "WEKNORA_ARTIFACT_SIGNING_KEY"

// ArtifactSigningKeyFromEnv loads the HMAC secret. Secrets are never embedded
// in source, examples, or tests.
func ArtifactSigningKeyFromEnv() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(ArtifactSigningKeyEnvVar))
	if raw == "" {
		return nil, ErrArtifactSigningKeyMissing
	}
	decoded, err := hex.DecodeString(raw)
	if err != nil || len(decoded) < 32 {
		return nil, fmt.Errorf("%s must be at least 32 bytes of hex entropy", ArtifactSigningKeyEnvVar)
	}
	return decoded, nil
}

// Canonical returns the exact byte string covered by the HMAC. Field order is
// fixed and every field is length-delimited by '|' with forbidden characters
// rejected up front, so two different grants can never canonicalize equally.
func (g ArtifactGrant) Canonical() (string, error) {
	if g.TenantID == 0 {
		return "", errors.New("artifact grant: tenant id required")
	}
	if g.SessionID == "" || g.MessageID == "" {
		return "", errors.New("artifact grant: session and message ids required")
	}
	if g.Index < 0 || g.ExpiresAt <= 0 {
		return "", errors.New("artifact grant: index and expiry required")
	}
	for _, s := range []string{g.SessionID, g.MessageID} {
		if strings.ContainsAny(s, "|") {
			return "", errors.New("artifact grant: id contains forbidden character")
		}
	}
	return strings.Join([]string{
		"wk-artifact-v1",
		strconv.FormatUint(g.TenantID, 10),
		g.SessionID,
		g.MessageID,
		strconv.Itoa(g.Index),
		strconv.FormatInt(g.ExpiresAt, 10),
	}, "|"), nil
}

// SignArtifactGrant returns the lowercase hex HMAC-SHA256 of the canonical
// grant string. The secret must come from ArtifactSigningKeyFromEnv.
func SignArtifactGrant(secret []byte, g ArtifactGrant) (string, error) {
	if len(secret) < 32 {
		return "", errors.New("artifact signing key too short")
	}
	canonical, err := g.Canonical()
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// VerifyArtifactGrantAt checks the signature in constant time and the expiry
// against the supplied clock. A grant that expired even one second before now
// is rejected; callers map that to a re-authorize signal for the client.
func VerifyArtifactGrantAt(secret []byte, g ArtifactGrant, signature string, now time.Time) error {
	if len(secret) < 32 {
		return errors.New("artifact signing key too short")
	}
	if g.ExpiresAt <= now.Unix() {
		return errors.New("artifact grant expired")
	}
	expected, err := SignArtifactGrant(secret, g)
	if err != nil {
		return err
	}
	// hmac.Equal is constant-time; hex signatures are fixed length.
	if !hmac.Equal([]byte(strings.ToLower(signature)), []byte(expected)) {
		return errors.New("artifact grant signature mismatch")
	}
	return nil
}
