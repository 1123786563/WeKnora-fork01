package craft

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// PreviewTicketTTL bounds both the redemption ticket and the read capability
// it exchanges into (W02: 独立 origin 受控预览). Five minutes covers a working
// preview round; anything longer turns a leaked URL into a standing grant.
// Refreshing an expired preview always goes back through the main origin's
// authenticated issuance.
const PreviewTicketTTL = 5 * time.Minute

// PreviewPathSegment is the URL path segment every controlled preview is
// served under: /p/<opaque-token>/<version-relative-file>. Nothing outside
// this prefix is a preview, so a root-absolute reference such as /style.css
// in generated HTML can never resolve into the version tree — it simply has
// no route.
const PreviewPathSegment = "p"

// PreviewTokenBytes is the entropy of one preview token: 256 random bits.
const PreviewTokenBytes = 32

// PreviewTicket is the answer of one authorized issuance. URL points at the
// isolated preview origin and carries the one-time redemption ticket in its
// path; ExpiresAt is the ticket deadline (the capability minted at redemption
// gets its own, equally short deadline). Only the ticket's digest is retained
// server-side; neither ticket nor capability ever enters persistent events or
// logs.
type PreviewTicket struct {
	URL       string
	ExpiresAt time.Time
	VersionID string
}

// PreviewOriginAllowed reports whether previewOrigin is a valid serving origin
// for the app at appOrigin. Both must be parseable bare origins (no userinfo,
// path, query or fragment), the preview origin must be https, and it must be a
// different origin from the app — the preview shares nothing with the main
// site's origin, so generated scripts can never read the main session.
func PreviewOriginAllowed(appOrigin, previewOrigin string) bool {
	a, e1 := url.Parse(appOrigin)
	p, e2 := url.Parse(previewOrigin)
	if e1 != nil || e2 != nil || a.Host == "" || p.Host == "" || p.User != nil {
		return false
	}
	if p.Scheme != "https" || p.Path != "" || p.RawQuery != "" || p.Fragment != "" {
		return false
	}
	return !strings.EqualFold(a.Scheme+"://"+a.Host, p.Scheme+"://"+p.Host)
}

// NewPreviewToken mints one 256-bit random opaque token together with the
// digest under which it is tracked. The token travels in the preview URL; the
// digest is the only thing the service keeps, so a database or log disclosure
// cannot resurrect live grants.
func NewPreviewToken() (token, digest string, err error) {
	raw := make([]byte, PreviewTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("craft: mint preview token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, PreviewTokenDigest(token), nil
}

// PreviewTokenDigest derives the tracking digest of one preview token.
func PreviewTokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// IsVersionID reports whether id has the persisted version id shape:
// VersionIDPrefix followed by exactly 64 lowercase hex characters.
func IsVersionID(id string) bool {
	if len(id) != len(VersionIDPrefix)+64 || !strings.HasPrefix(id, VersionIDPrefix) {
		return false
	}
	for _, c := range id[len(VersionIDPrefix):] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// PreviewableKind reports whether the controlled preview can serve files of a
// version kind. Every kind admitted so far ships immutable static files
// (HTML/CSS/JS for web; report.md/report.docx for document; preview.json and
// the recalculated workbook for spreadsheet; preview.json, the rendered PDF
// and the page images for slides), so all four are previewable: the preview
// serves those pinned bytes only. A kind whose deliverable needs a backing
// application is not previewable — the preview never proxies a model-supplied
// host, port or URL.
func PreviewableKind(kind string) bool {
	switch kind {
	case KindWeb, KindDocument, KindSpreadsheet, KindSlides:
		return true
	default:
		return false
	}
}

// ValidatePreviewRequestPath canonicalizes one file path requested under a
// preview capability. Beyond ValidateArtifactPath's canonical-path rules
// (no traversal, no absolute prefix, no backslash, no credentials) it rejects
// any remaining percent-escape: the router already decoded the request path
// once, so an encoded byte here means someone is trying to smuggle a second
// decoding past the validator.
func ValidatePreviewRequestPath(raw string) (string, error) {
	rel := strings.TrimPrefix(raw, "/")
	if strings.ContainsRune(rel, '%') {
		return "", fmt.Errorf("%w: preview path %q contains percent-escapes", ErrInvalidInput, raw)
	}
	if err := ValidateArtifactPath(rel); err != nil {
		return "", err
	}
	return rel, nil
}

// PreviewCheckFromEvidence renders the preview verification check exactly as
// BuildChecks does for the same evidence. The W02 update channel and W01's
// collector must produce byte-identical entries: Publish's identical-replay
// adoption compares checks field by field, and a re-collect of an
// already-verified version carries the recorded verdict through the evidence
// source, so any wording drift would turn an idempotent retry into a
// conflict. preview_test pins this equality against BuildChecks itself.
func PreviewCheckFromEvidence(ran, passed bool) Check {
	switch {
	case !ran:
		return Check{Name: CheckPreview, Status: CheckNotRun, Detail: "page preview not verified for this version"}
	case passed:
		return Check{Name: CheckPreview, Status: CheckPassed, Detail: "controlled preview served this version's files"}
	default:
		return Check{Name: CheckPreview, Status: CheckFailed, Detail: "controlled preview verification failed"}
	}
}

// PreviewCheckStore is the W02-owned update channel for one published
// version's preview check. A version's verification facts are written at
// publish time by W01's collector; when the controlled preview later verifies
// the real page, this channel rewrites ONLY the preview entry in place. It
// never re-Publishes: the same version identity published with different
// checks is a conflict by design, which is exactly why the update needs its
// own narrow path.
type PreviewCheckStore interface {
	// UpdatePreviewCheck replaces (or appends) the preview check of one
	// published version in the requesting scope and returns the updated
	// version. check.Name must be CheckPreview.
	UpdatePreviewCheck(ctx context.Context, scope Scope, versionID string, check Check) (Version, error)
}
