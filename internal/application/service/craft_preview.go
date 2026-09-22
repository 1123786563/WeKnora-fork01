// Package service - Craft controlled preview (W02: 独立 origin 受控预览).
//
// CraftPreviewService serves one immutable artifact version's files on an
// isolated https origin through short, bearer-capability URLs:
//
//   - Issue: one authenticated main-origin request validates the caller's
//     scope against the version and mints a 256-bit one-time redemption
//     ticket. Only the ticket's digest is kept — never the ticket itself, and
//     neither tickets nor capabilities ever enter persistent events or logs.
//   - Redeem: the first request on the isolated origin consumes the ticket
//     and mints a distinct read-only capability for the same version. Both
//     grants expire after PreviewTicketTTL (5 minutes); refresh always goes
//     back through the main origin's authenticated issuance.
//   - Resolve: every single file read re-validates the capability, re-loads
//     the version in the capability's scope, and serves only files listed in
//     that version's manifest. There is no third cookie, no session, and no
//     way to address any other version or tenant through the same URL.
//
// The preview serves immutable static web artifacts only. Kinds whose
// deliverables need a backing application answer ErrUnsupported — the preview
// never proxies a model-supplied host, port or URL.
package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// maxCraftPreviewGrants bounds the in-memory grant tables. Live grants are
// ephemeral by design (5 minute TTL, lazy purge); the cap keeps a runaway
// issuer from growing process memory unbounded.
const maxCraftPreviewGrants = 1 << 16

// CraftPreviewConfig configures the controlled preview. AppOrigin and
// PreviewOrigin are bare origins; the preview must remain disabled until
// PreviewOrigin is configured (production) — a local HTTPS test origin is
// configured separately.
type CraftPreviewConfig struct {
	// AppOrigin is the main application origin, e.g. https://app.example.com.
	AppOrigin string
	// PreviewOrigin is the isolated preview origin, e.g.
	// https://preview.example.com. Empty disables the feature.
	PreviewOrigin string
	// TTL bounds both the redemption ticket and the read capability.
	// Defaults to craft.PreviewTicketTTL.
	TTL time.Duration
	// Now overrides the clock in tests.
	Now func() time.Time
}

func (c CraftPreviewConfig) withDefaults() CraftPreviewConfig {
	if c.TTL <= 0 {
		c.TTL = craft.PreviewTicketTTL
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return c
}

// craftPreviewGrant is one live grant (ticket or capability) tracked by the
// digest of its opaque token. The token itself is never stored.
type craftPreviewGrant struct {
	scope     craft.Scope
	versionID string
	expiresAt time.Time
}

// CraftPreviewService implements the W02 controlled preview.
type CraftPreviewService struct {
	versions craft.VersionStore
	files    interfaces.FileService
	checks   craft.PreviewCheckStore
	config   CraftPreviewConfig

	mu      sync.Mutex
	tickets map[string]craftPreviewGrant // digest → grant
	caps    map[string]craftPreviewGrant // digest → grant
}

// NewCraftPreviewService assembles the preview service. versions and files
// are required; checks may be nil (verdict recording is then unavailable and
// the evidence source reports not_run, but serving still works). Configuring
// a preview origin that is not a distinct https origin from the app origin is
// an assembly bug and panics here, mirroring the artifact service.
func NewCraftPreviewService(
	versions craft.VersionStore,
	files interfaces.FileService,
	checks craft.PreviewCheckStore,
	config CraftPreviewConfig,
) *CraftPreviewService {
	if versions == nil || files == nil {
		panic("craft: NewCraftPreviewService requires a version store and file service")
	}
	config = config.withDefaults()
	if config.AppOrigin != "" && config.PreviewOrigin != "" &&
		!craft.PreviewOriginAllowed(config.AppOrigin, config.PreviewOrigin) {
		panic(fmt.Sprintf("craft: preview origin %q must be a distinct https origin from app origin %q",
			config.PreviewOrigin, config.AppOrigin))
	}
	return &CraftPreviewService{
		versions: versions,
		files:    files,
		checks:   checks,
		config:   config,
		tickets:  map[string]craftPreviewGrant{},
		caps:     map[string]craftPreviewGrant{},
	}
}

// Enabled reports whether the preview origin is configured. Unconfigured
// deployments keep the feature off: issuance answers ErrUnsupported and the
// isolated-origin endpoint 404s.
func (s *CraftPreviewService) Enabled() bool {
	return s != nil && strings.TrimSpace(s.config.PreviewOrigin) != ""
}

// Issue mints one redemption ticket for a published version after the full
// version ACL ran in the requesting scope. The ticket URL points at the
// isolated preview origin. Issuing is not verification: the preview check
// only turns passed when real page verification records its verdict (W06),
// never here.
func (s *CraftPreviewService) Issue(ctx context.Context, scope craft.Scope, versionID string) (craft.PreviewTicket, error) {
	if !s.Enabled() {
		return craft.PreviewTicket{}, fmt.Errorf("%w: preview origin is not configured", craft.ErrUnsupported)
	}
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return craft.PreviewTicket{}, fmt.Errorf("%w: preview issuance requires a complete scope", craft.ErrInvalidInput)
	}
	if !craft.IsVersionID(versionID) {
		return craft.PreviewTicket{}, fmt.Errorf("%w: version id %q", craft.ErrInvalidInput, versionID)
	}
	version, err := s.versions.Get(ctx, scope, versionID)
	if err != nil {
		return craft.PreviewTicket{}, err
	}
	if !craft.PreviewableKind(version.Kind) {
		return craft.PreviewTicket{}, fmt.Errorf("%w: 不支持此预览类型: kind %q 的产物需要后端应用，受控预览仅服务不可变静态网页产物，不代理任意端口或 URL",
			craft.ErrUnsupported, version.Kind)
	}
	entry, ok := craft.EntryPath(version.Kind)
	if !ok {
		return craft.PreviewTicket{}, fmt.Errorf("%w: kind %q has no entry file", craft.ErrUnsupported, version.Kind)
	}

	token, digest, err := craft.NewPreviewToken()
	if err != nil {
		return craft.PreviewTicket{}, err
	}
	now := s.config.Now()
	expiresAt := now.Add(s.config.TTL)
	if err := s.putTicket(digest, craftPreviewGrant{scope: scope, versionID: version.ID, expiresAt: expiresAt}); err != nil {
		return craft.PreviewTicket{}, err
	}
	base := strings.TrimSuffix(s.config.PreviewOrigin, "/")
	url := base + "/" + craft.PreviewPathSegment + "/" + token + "/" + entry
	// No events, no persistent logs: the ticket never appears anywhere but
	// the authorized response body.
	return craft.PreviewTicket{URL: url, ExpiresAt: expiresAt, VersionID: version.ID}, nil
}

// PreviewOpen is one answered request on the isolated origin: either a
// redirect that exchanges a live ticket into its fresh capability, or the
// resolved file with an open reader.
type PreviewOpen struct {
	// Redirect is the capability URL to follow (ticket redemption). The
	// redirected path keeps the requested file so relative resources inherit
	// the capability path.
	Redirect string
	// File is the resolved manifest entry (set when Redirect is empty).
	File craft.File
	// Reader streams the file's pinned bytes (set when Redirect is empty).
	Reader io.ReadCloser
}

// Open answers one /p/<token>/<file> request: a still-live ticket is redeemed
// exactly once — the second hit on the same ticket no longer exists — and
// answers a redirect to the same path under the fresh capability; a
// capability serves the requested file of its bound version.
func (s *CraftPreviewService) Open(ctx context.Context, token, requestPath string) (PreviewOpen, error) {
	if !s.Enabled() {
		return PreviewOpen{}, fmt.Errorf("%w: preview origin is not configured", craft.ErrUnsupported)
	}
	rel, err := craft.ValidatePreviewRequestPath(requestPath)
	if err != nil {
		return PreviewOpen{}, err
	}
	digest := craft.PreviewTokenDigest(token)

	// Ticket redemption first: unconsumed, unexpired tickets only.
	s.mu.Lock()
	grant, ok := s.tickets[digest]
	if ok {
		delete(s.tickets, digest) // one time, always: consumed is indistinguishable from unknown
	}
	s.mu.Unlock()
	if ok {
		if s.config.Now().After(grant.expiresAt) {
			return PreviewOpen{}, fmt.Errorf("%w: preview ticket expired, re-authorize on the main origin", craft.ErrForbidden)
		}
		capToken, capDigest, err := craft.NewPreviewToken()
		if err != nil {
			return PreviewOpen{}, err
		}
		expiresAt := s.config.Now().Add(s.config.TTL)
		if err := s.putCapability(capDigest, craftPreviewGrant{scope: grant.scope, versionID: grant.versionID, expiresAt: expiresAt}); err != nil {
			return PreviewOpen{}, err
		}
		return PreviewOpen{Redirect: "/" + craft.PreviewPathSegment + "/" + capToken + "/" + rel}, nil
	}

	file, _, grant, err := s.lookup(ctx, digest, rel)
	if err != nil {
		return PreviewOpen{}, err
	}
	reader, err := s.openReader(ctx, grant, file)
	if err != nil {
		return PreviewOpen{}, err
	}
	return PreviewOpen{File: file, Reader: reader}, nil
}

// Resolve validates one capability against one version-relative path and
// returns the manifest entry — the brief's serving contract. It performs the
// same per-read validation as Open without opening bytes.
func (s *CraftPreviewService) Resolve(ctx context.Context, token, path string) (craft.File, error) {
	if !s.Enabled() {
		return craft.File{}, fmt.Errorf("%w: preview origin is not configured", craft.ErrUnsupported)
	}
	rel, err := craft.ValidatePreviewRequestPath(path)
	if err != nil {
		return craft.File{}, err
	}
	file, _, _, err := s.lookup(ctx, craft.PreviewTokenDigest(token), rel)
	return file, err
}

// lookup resolves a capability digest and a canonical file path to the
// version's manifest entry. Every read re-validates: the grant must be live,
// the version must still exist in the grant's scope, and the file must be
// part of that version's manifest — an entry that only exists in a NEWER
// version of the workspace stays a 404.
func (s *CraftPreviewService) lookup(ctx context.Context, digest, rel string) (craft.File, craft.Scope, craftPreviewGrant, error) {
	s.mu.Lock()
	grant, ok := s.caps[digest]
	if ok && s.config.Now().After(grant.expiresAt) {
		delete(s.caps, digest)
		ok = false
	}
	s.mu.Unlock()
	if !ok {
		return craft.File{}, craft.Scope{}, craftPreviewGrant{}, fmt.Errorf("%w: no preview capability", craft.ErrNotFound)
	}
	version, err := s.versions.Get(ctx, grant.scope, grant.versionID)
	if err != nil {
		return craft.File{}, craft.Scope{}, craftPreviewGrant{}, err
	}
	for _, f := range version.Files {
		if f.Path == rel {
			return f, grant.scope, grant, nil
		}
	}
	return craft.File{}, craft.Scope{}, craftPreviewGrant{},
		fmt.Errorf("%w: %q is not part of version %s", craft.ErrNotFound, rel, grant.versionID)
}

// openReader streams one resolved file through the tenant's file service,
// reading as the file's owning tenant. The read is additionally bounded by
// the manifest's recorded size so a swapped blob cannot stream past the
// pinned entry.
func (s *CraftPreviewService) openReader(ctx context.Context, grant craftPreviewGrant, file craft.File) (io.ReadCloser, error) {
	readCtx := types.WithExecutionTenant(ctx, grant.scope.TenantID)
	reader, err := s.files.GetFile(readCtx, file.Ref)
	if err != nil {
		return nil, fmt.Errorf("craft: open preview object: %w", err)
	}
	return newBoundedReadCloser(reader, file.Bytes), nil
}

// putTicket stores one ticket grant, purging expired entries first.
func (s *CraftPreviewService) putTicket(digest string, grant craftPreviewGrant) error {
	return s.putGrant(s.tickets, digest, grant)
}

// putCapability stores one capability grant, purging expired entries first.
func (s *CraftPreviewService) putCapability(digest string, grant craftPreviewGrant) error {
	return s.putGrant(s.caps, digest, grant)
}

func (s *CraftPreviewService) putGrant(table map[string]craftPreviewGrant, digest string, grant craftPreviewGrant) error {
	now := s.config.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for d, g := range table {
		if now.After(g.expiresAt) {
			delete(table, d)
		}
	}
	if len(table) >= maxCraftPreviewGrants {
		return fmt.Errorf("%w: too many live preview grants", craft.ErrBusy)
	}
	table[digest] = grant
	return nil
}

// RecordPreviewVerdict records the externally observed preview verdict for
// one version through the W02 update channel — the only way a published
// version's preview check moves off not_run. A verdict of passed without a
// run is rejected: facts that never happened are never recorded. Issuing a
// ticket does NOT call this; the verdict belongs to real page verification
// (W06 loads the page; only then does the check pass).
func (s *CraftPreviewService) RecordPreviewVerdict(ctx context.Context, scope craft.Scope, versionID string, ran, passed bool) error {
	if s == nil || s.checks == nil {
		return fmt.Errorf("%w: preview check update channel is not assembled", craft.ErrUnsupported)
	}
	if !ran && passed {
		return fmt.Errorf("%w: a preview verdict cannot pass without running", craft.ErrInvalidInput)
	}
	version, err := s.versions.Get(ctx, scope, versionID)
	if err != nil {
		return err
	}
	if !craft.PreviewableKind(version.Kind) {
		return fmt.Errorf("%w: kind %q has no controlled preview", craft.ErrUnsupported, version.Kind)
	}
	check := craft.PreviewCheckFromEvidence(ran, passed)
	for _, existing := range version.Checks {
		if existing == check {
			return nil // idempotent replay
		}
	}
	_, err = s.checks.UpdatePreviewCheck(ctx, scope, versionID, check)
	if err != nil {
		logger.Warnf(ctx, "[CraftPreview] preview check update failed for version %s: %v", versionID, err)
	}
	return err
}

// EvidenceSource adapts the recorded preview verdicts into W01's
// ArtifactEvidenceSource injection point. Truthfulness rules: a verdict is
// carried into a collection round only when the workspace has exactly one
// published version for that run — a re-collection retry of the same run
// re-derives the same version identity, so the recorded verdict is a fact
// about exactly the bytes being re-published. Multiple versions under one
// run mean content changed mid-run; the source then reports nothing and the
// fresh version's preview check stays not_run until it is really verified.
func (s *CraftPreviewService) EvidenceSource() ArtifactEvidenceSource {
	return func(ctx context.Context, task craft.Task) craft.ArtifactEvidence {
		versions, err := s.versions.List(ctx, task.Scope)
		if err != nil {
			return craft.ArtifactEvidence{}
		}
		var match *craft.Version
		for i := range versions {
			v := versions[i]
			if v.WorkspaceID != task.WorkspaceID || v.RunID != task.Fence.RunID {
				continue
			}
			if match != nil {
				return craft.ArtifactEvidence{} // ambiguous: content changed within the run
			}
			match = &versions[i]
		}
		if match == nil {
			return craft.ArtifactEvidence{}
		}
		evidence := craft.ArtifactEvidence{}
		for _, c := range match.Checks {
			if c.Name != craft.CheckPreview {
				continue
			}
			switch c.Status {
			case craft.CheckPassed:
				evidence.PreviewRan, evidence.PreviewPassed = true, true
			case craft.CheckFailed:
				evidence.PreviewRan = true
			}
		}
		return evidence
	}
}

// PreviewCSP renders the Content-Security-Policy served on every preview
// response. Resources come from the controlled preview origin only, network
// and worker registration are denied (connect-src/worker-src none), forms
// are inert, and only the main app origin may embed the preview.
func (s *CraftPreviewService) PreviewCSP() string {
	ancestors := "'none'"
	if s != nil && strings.TrimSpace(s.config.AppOrigin) != "" {
		ancestors = strings.TrimSpace(s.config.AppOrigin)
	}
	return "default-src 'none'; " +
		"script-src 'self' 'unsafe-inline'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob:; " +
		"font-src 'self'; " +
		"media-src 'self'; " +
		"connect-src 'none'; " +
		"object-src 'none'; " +
		"base-uri 'none'; " +
		"form-action 'none'; " +
		"frame-ancestors " + ancestors + "; " +
		"worker-src 'none'; " +
		"child-src 'none'"
}

// boundedReadCloser caps one stream at n bytes; an overrun is a hard error
// because the pinned manifest entry promised this exact size.
type boundedReadCloser struct {
	inner     io.ReadCloser
	remaining int64
}

func newBoundedReadCloser(inner io.ReadCloser, n int64) *boundedReadCloser {
	if n < 0 {
		n = 0
	}
	return &boundedReadCloser{inner: inner, remaining: n}
}

func (b *boundedReadCloser) Read(p []byte) (int, error) {
	if b.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.inner.Read(p)
	b.remaining -= int64(n)
	if err == nil && b.remaining <= 0 {
		// Emit EOF together with the final bytes; a longer underlying stream
		// is masked, never served.
		return n, io.EOF
	}
	return n, err
}

func (b *boundedReadCloser) Close() error { return b.inner.Close() }
