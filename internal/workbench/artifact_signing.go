package workbench

import (
	"context"
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

// VersionArtifactGrant is a short-lived authority for one immutable artifact
// version. Unlike ArtifactGrant (the legacy Task/message link contract), it
// binds the authenticated owner, resource and exact version. It is deliberately
// a separate wire contract so existing Task download URLs remain valid.
type VersionArtifactGrant struct {
	TenantID   uint64
	OwnerID    string
	ResourceID string
	VersionID  string
	Digest     string
	ExpiresAt  int64 // unix nanoseconds
}

func (g VersionArtifactGrant) Canonical() (string, error) {
	if g.TenantID == 0 || strings.TrimSpace(g.OwnerID) == "" || strings.TrimSpace(g.ResourceID) == "" || strings.TrimSpace(g.VersionID) == "" || strings.TrimSpace(g.Digest) == "" || g.ExpiresAt <= 0 {
		return "", errors.New("version artifact grant: tenant, owner, resource, version, digest and expiry required")
	}
	digest, err := hex.DecodeString(g.Digest)
	if err != nil || len(digest) != sha256.Size {
		return "", errors.New("version artifact grant: digest must be SHA-256 hex")
	}
	parts := []string{"wk-version-artifact-v2", strconv.FormatUint(g.TenantID, 10), g.OwnerID, g.ResourceID, g.VersionID, strings.ToLower(g.Digest), strconv.FormatInt(g.ExpiresAt, 10)}
	for _, part := range parts[2:6] {
		if strings.ContainsAny(part, "|\r\n") || strings.TrimSpace(part) != part {
			return "", errors.New("version artifact grant: invalid field")
		}
	}
	return strings.Join(parts, "|"), nil
}

// NewVersionArtifactGrant constructs a server-issued grant with a bounded TTL.
// Callers must take all identity and version fields from authenticated context
// and the authoritative artifact catalog.
func NewVersionArtifactGrant(tenantID uint64, ownerID, resourceID, versionID, digest string, now time.Time, ttl time.Duration) (VersionArtifactGrant, error) {
	if ttl <= 0 || ttl < time.Second {
		return VersionArtifactGrant{}, errors.New("artifact grant TTL must be at least one second")
	}
	if ttl > MaxArtifactGrantTTL {
		ttl = MaxArtifactGrantTTL
	}
	g := VersionArtifactGrant{TenantID: tenantID, OwnerID: ownerID, ResourceID: resourceID, VersionID: versionID, Digest: strings.ToLower(digest), ExpiresAt: now.Add(ttl).UnixNano()}
	_, err := g.Canonical()
	return g, err
}

// SignVersionArtifactGrant signs an exact immutable version grant.
func SignVersionArtifactGrant(secret []byte, grant VersionArtifactGrant) (string, error) {
	if len(secret) < 32 {
		return "", errors.New("artifact signing key too short")
	}
	canonical, err := grant.Canonical()
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// VerifyVersionArtifactGrantAt checks signature and expiry. Download callers
// must also invoke VersionArtifactGrantAuthorizer.AuthorizeVersionGrant on
// every request to recheck ownership, existence, digest and revocation.
func VerifyVersionArtifactGrantAt(secret []byte, grant VersionArtifactGrant, signature string, now time.Time) error {
	if grant.ExpiresAt <= now.UnixNano() {
		return errors.New("artifact grant expired")
	}
	expected, err := SignVersionArtifactGrant(secret, grant)
	if err != nil {
		return err
	}
	if !hmac.Equal([]byte(strings.ToLower(signature)), []byte(expected)) {
		return errors.New("artifact grant signature mismatch")
	}
	return nil
}

// VersionArtifactGrantAuthorizer is implemented by the owner of the artifact
// resource catalog. It must deny deleted/revoked versions and require an exact
// match of tenant, owner, resource, version and digest.
type VersionArtifactGrantAuthorizer interface {
	AuthorizeVersionGrant(ctx context.Context, grant VersionArtifactGrant) error
}

// VersionArtifactGrantAuthority combines cryptographic verification with the
// required live owner/resource check. Call Authorize for every download.
type VersionArtifactGrantAuthority struct {
	Secret     []byte
	Authorizer VersionArtifactGrantAuthorizer
}

func (a VersionArtifactGrantAuthority) Issue(ctx context.Context, tenantID uint64, ownerID, resourceID, versionID, digest string, now time.Time, ttl time.Duration) (VersionArtifactGrant, string, error) {
	grant, err := NewVersionArtifactGrant(tenantID, ownerID, resourceID, versionID, digest, now, ttl)
	if err != nil {
		return VersionArtifactGrant{}, "", err
	}
	if a.Authorizer == nil {
		return VersionArtifactGrant{}, "", errors.New("artifact grant authorizer unavailable")
	}
	if err := a.Authorizer.AuthorizeVersionGrant(ctx, grant); err != nil {
		return VersionArtifactGrant{}, "", err
	}
	sig, err := SignVersionArtifactGrant(a.Secret, grant)
	if err != nil {
		return VersionArtifactGrant{}, "", err
	}
	return grant, sig, nil
}

func (a VersionArtifactGrantAuthority) Authorize(ctx context.Context, grant VersionArtifactGrant, signature string, now time.Time) error {
	if err := VerifyVersionArtifactGrantAt(a.Secret, grant, signature, now); err != nil {
		return err
	}
	if a.Authorizer == nil {
		return errors.New("artifact grant authorizer unavailable")
	}
	return a.Authorizer.AuthorizeVersionGrant(ctx, grant)
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

// Canonical returns a versioned, unambiguous representation of the grant.
func (g ArtifactVersionGrant) Canonical() (string, error) {
	if g.TenantID == 0 || g.OwnerID == "" || g.RunID == "" || g.SessionID == "" || g.VersionID == "" || g.ExpiresAt <= 0 {
		return "", errors.New("artifact version grant: tenant, owner, run, session, version and expiry required")
	}
	for _, value := range []string{g.OwnerID, g.RunID, g.SessionID, g.VersionID} {
		if strings.ContainsAny(value, "|") {
			return "", errors.New("artifact version grant: field contains forbidden character")
		}
	}
	return strings.Join([]string{
		"wk-artifact-version-v1",
		strconv.FormatUint(g.TenantID, 10), g.OwnerID, g.RunID, g.SessionID, g.VersionID,
		strconv.FormatInt(g.ExpiresAt, 10),
	}, "|"), nil
}

// ---- issue-140 世代契约（run/session 签名下载链接），与 VersionArtifactGrant 并存 ----

// ArtifactVersionGrant binds a download capability to one immutable artifact
// version and the run owner who received it. The owner is rechecked against
// the durable run record when the link is used.
type ArtifactVersionGrant struct {
	TenantID  uint64
	OwnerID   string
	RunID     string
	SessionID string
	VersionID string
	ExpiresAt int64
}

// SignArtifactVersionGrant returns the lowercase hex HMAC-SHA256 signature.
func SignArtifactVersionGrant(secret []byte, grant ArtifactVersionGrant) (string, error) {
	if len(secret) < 32 {
		return "", errors.New("artifact signing key too short")
	}
	canonical, err := grant.Canonical()
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// VerifyArtifactVersionGrantAt checks signature and expiry in constant time.
func VerifyArtifactVersionGrantAt(secret []byte, grant ArtifactVersionGrant, signature string, now time.Time) error {
	if grant.ExpiresAt <= now.Unix() {
		return errors.New("artifact grant expired")
	}
	expected, err := SignArtifactVersionGrant(secret, grant)
	if err != nil {
		return err
	}
	if !hmac.Equal([]byte(strings.ToLower(signature)), []byte(expected)) {
		return errors.New("artifact grant signature mismatch")
	}
	return nil
}
