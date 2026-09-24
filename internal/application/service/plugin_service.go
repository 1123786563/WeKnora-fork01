package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/utils"
)

var (
	// ErrManifestURLRejected marks a client-input manifest URL that failed
	// SSRF validation (scheme, localhost/loopback/private/reserved target).
	// It is a 4xx-class rejection: the admin pasted an address WeKnora will
	// never fetch.
	ErrManifestURLRejected = errors.New("plugin manifest URL rejected")

	// ErrPreviewPersistFailed marks a persistence failure on the preview
	// path — a server-side fault, 5xx-class.
	ErrPreviewPersistFailed = errors.New("failed to persist plugin preview")
)

// defaultPluginPreviewTTL bounds how long a preview stays confirmable.
// PLUGIN_PREVIEW_TTL (Go duration string, e.g. "15m") overrides it; an
// empty, unparsable or non-positive value falls back to this default, so a
// misconfigured environment degrades to the spec'd bound instead of an
// unbounded or zero TTL. An oversized-but-parsable value (e.g. "8760h" ≈ a
// year) is clamped to maxPluginPreviewTTL — that is the same misconfiguration
// in the other direction: previews would stay confirmable for months and
// DeleteExpiredPreviews would never reclaim them (跨任务转交 T01-R3-F1).
const defaultPluginPreviewTTL = 15 * time.Minute

// maxPluginPreviewTTL caps a valid PLUGIN_PREVIEW_TTL override. Previews are
// TTL-bound review artifacts; anything beyond a day defeats the bound.
const maxPluginPreviewTTL = 24 * time.Hour

// minPluginPreviewTTL floors a valid PLUGIN_PREVIEW_TTL override (OCR round-1
// F1): a TTL smaller than the CreatePreview round trip (e.g. "1ms") makes the
// just-written row fall to the same request's DeleteExpiredPreviews sweep
// while the response still returns the PreviewID — confirm would then hit
// zero rows forever. Symmetric with the cap: misconfiguration degrades to the
// spec'd bound in BOTH directions.
const minPluginPreviewTTL = time.Minute

// maxPluginURLRunes aligns with plugin_previews.manifest_url /
// endpoint_url varchar(512) — in runes, the unit PostgreSQL varchar counts.
// Inputs longer than the column must be rejected as client-input problems
// BEFORE any network I/O (manifest URL) or BEFORE persistence (endpoint):
// otherwise an admin's oversized paste, or a remote manifest declaring an
// oversized endpoint, would complete a full fetch-and-verify round trip and
// then fail at CreatePreview with a column overflow misreported as a 500.
const maxPluginURLRunes = 512

func validatePluginURLLength(where, rawURL string) error {
	if utf8.RuneCountInString(rawURL) > maxPluginURLRunes {
		return fmt.Errorf("%s exceeds %d characters", where, maxPluginURLRunes)
	}
	return nil
}

// validateManifestURLInput is the deterministic input pre-check a manifest
// URL must pass BEFORE the SSRF verdict and any fetch (OCR round-1 F7/F6):
//
//   - url.Parse failure → fixed message, never the *url.Error text: it embeds
//     the full URL verbatim, so a malformed URL that also carries userinfo
//     would leak the credentials into the admin-visible 400 body (review
//     measured exactly that) — bypassing fetcher.go's no-echo discipline
//     (OCR T01-R3-F2) which only guards the WELL-FORMED userinfo case.
//   - userinfo (u.User != nil) → silent rejection with a fixed message, same
//     hygiene as fetcher.go's step 1.
//   - non-http(s) scheme (including empty, e.g. a bare "example.com/x"):
//     ValidateURLForSSRF would auto-prepend https:// and pass, but the fetch
//     layer uses the ORIGINAL URL and fails deterministically ("unsupported
//     protocol scheme") — surfacing as a retryable 503 instead of the
//     documented @Failure 400. Reject up front.
func validateManifestURLInput(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%w: URL is malformed (no details echoed)", ErrManifestURLRejected)
	}
	if u.User != nil {
		return fmt.Errorf("%w: URL must not embed userinfo credentials", ErrManifestURLRejected)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: URL scheme must be http or https, got %q", ErrManifestURLRejected, u.Scheme)
	}
	return nil
}

func pluginPreviewTTL() time.Duration {
	raw := strings.TrimSpace(os.Getenv("PLUGIN_PREVIEW_TTL"))
	if raw == "" {
		return defaultPluginPreviewTTL
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return defaultPluginPreviewTTL
	}
	if d < minPluginPreviewTTL {
		return minPluginPreviewTTL
	}
	if d > maxPluginPreviewTTL {
		return maxPluginPreviewTTL
	}
	return d
}

// pluginService implements the PluginService interface. The EndpointLister
// seam is injected by the composition root (internal/container's
// NewPluginMCPEndpointLister over the airesource MCPManager) so this package
// never depends on the MCP client stack directly. The install slice (T06)
// additionally holds the product-layer seams it materializes into:
// MCPServiceService (validated service creation), MCPServiceRepository
// (state sync + compensation deletes) and MCPToolApprovalService (per-tool
// policies) — reusing the existing MCP product layer per spec line 55
// instead of forking a parallel one.
type pluginService struct {
	pluginRepo          interfaces.PluginRepository
	mcpServiceService   interfaces.MCPServiceService
	mcpServiceRepo      interfaces.MCPServiceRepository
	toolApprovalService interfaces.MCPToolApprovalService
	lister              plugins.EndpointLister
	// clientCloser (T06-OCR1-F7/F13) closes the manager's cached MCP
	// clients for a service after the plugin domain hard-deletes or flips
	// its materialized row — injected by the composition root, nil-safe.
	clientCloser MCPClientCloser
}

// NewPluginService creates a new plugin service.
func NewPluginService(
	pluginRepo interfaces.PluginRepository,
	mcpServiceService interfaces.MCPServiceService,
	mcpServiceRepo interfaces.MCPServiceRepository,
	toolApprovalService interfaces.MCPToolApprovalService,
	lister plugins.EndpointLister,
	clientCloser MCPClientCloser,
) interfaces.PluginService {
	return &pluginService{
		pluginRepo:          pluginRepo,
		mcpServiceService:   mcpServiceService,
		mcpServiceRepo:      mcpServiceRepo,
		toolApprovalService: toolApprovalService,
		lister:              lister,
		clientCloser:        clientCloser,
	}
}

// PreviewFromManifest fetches and verifies the manifest, then persists a
// TTL-bound preview carrying the identity fingerprint of exactly what was
// verified. The SSRF gate runs BEFORE any network I/O and before any
// persistence — a rejected URL leaves zero rows behind. The result is a
// types-layer struct; the HTTP handler maps it onto its DTO (整分支 OCR
// 一轮 F2: interfaces must not depend on the handler layer).
func (s *pluginService) PreviewFromManifest(
	ctx context.Context,
	tenantID uint64,
	actorID, manifestURL string,
) (*types.PluginPreviewResult, error) {
	// Length first, before ANY network I/O: an oversized URL is a client
	// input problem and must not cost a fetch round trip (nor surface later
	// as a column-overflow 500).
	if err := validatePluginURLLength("plugin manifest URL", manifestURL); err != nil {
		return nil, err
	}
	// Deterministic input pre-check (OCR round-1 F7/F6): malformed URLs,
	// userinfo credentials and schemeless hosts are 4xx input errors — the
	// SSRF layer auto-normalizes scheme (masking the last case as a fetch
	// fault) and its parse-failure text would echo credentials.
	if err := validateManifestURLInput(manifestURL); err != nil {
		return nil, err
	}
	// Classify the SSRF rejection here (FetchAndVerify re-validates the same
	// URL — defense in depth — but its plain error carries no identity the
	// handler could map onto a status code).
	if err := utils.ValidateURLForSSRF(manifestURL); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrManifestURLRejected, err)
	}
	result, err := plugins.FetchAndVerify(ctx, manifestURL, s.lister)
	if err != nil {
		// Fetch/verify failures (invalid manifest, digest mismatch, OAuth-
		// protected endpoint, unreachable endpoint) are deterministic
		// rejections of the admin's input; pass them through — the handler
		// maps the OAuth sentinel and everything else to 4xx.
		return nil, err
	}
	// The remote manifest declares the endpoint; ValidateManifest (T01)
	// checks scheme/host but not length. Bound it here, before persistence,
	// so an oversized declaration is a 4xx input rejection rather than an
	// endpoint_url column overflow reported as 500.
	if err := validatePluginURLLength("plugin transport endpoint", result.Manifest.Transport.Endpoint); err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(pluginPreviewTTL())
	preview := &types.PluginPreview{
		ID:                  uuid.New().String(),
		TenantID:            tenantID,
		ManifestURL:         manifestURL,
		PluginID:            result.Manifest.PluginID,
		Version:             result.Manifest.Version,
		Name:                result.Manifest.Name,
		TransportType:       result.Manifest.Transport.Type,
		EndpointURL:         result.Manifest.Transport.Endpoint,
		ToolsSnapshot:       types.PluginPreviewTools(result.Snapshot),
		ToolsDigest:         result.ToolsDigest,
		IdentityFingerprint: result.IdentityFingerprint,
		CreatedBy:           actorID,
		ExpiresAt:           expiresAt,
	}
	if err := s.pluginRepo.CreatePreview(ctx, preview); err != nil {
		// 整分支终评 r3-001：底层 DB 错误细节只进服务端日志；返回给调用方
		// （进而经 handler NewInternalServerError 原样进入 500 响应体）的
		// 文本只含哨兵语义——表名/约束名/驱动内部信息不得泄漏给客户端。
		logger.GetLogger(ctx).Errorf("failed to persist plugin preview: %v", err)
		return nil, ErrPreviewPersistFailed
	}
	// Opportunistic cleanup keeps preview rows — TTL-bound review artifacts,
	// consumed or not — from accumulating without bound (整分支 OCR 一轮 F1);
	// failure is non-fatal to the admin's preview (resource.go:244 pattern).
	// The row just written expires at now+TTL, safely past this cutoff.
	_ = s.pluginRepo.DeleteExpiredPreviews(ctx, time.Now())

	return &types.PluginPreviewResult{
		PreviewID:           preview.ID,
		PluginID:            preview.PluginID,
		Version:             preview.Version,
		Name:                preview.Name,
		Description:         result.Manifest.Description,
		TransportType:       preview.TransportType,
		EndpointURL:         preview.EndpointURL,
		Tools:               previewToolsReview(result.Snapshot),
		IdentityFingerprint: preview.IdentityFingerprint,
		ExpiresAt:           preview.ExpiresAt,
	}, nil
}

// previewToolsReview defensively copies the verified snapshot into the
// types-layer review rows (scopes are copied, never aliased). make+copy
// keeps undeclared scopes as [] instead of nil so the field serializes with
// one shape end to end (跨任务转交 T01-R1-F1).
func previewToolsReview(snapshot []types.PluginToolSnapshot) []types.PluginPreviewToolReview {
	tools := make([]types.PluginPreviewToolReview, 0, len(snapshot))
	for _, tool := range snapshot {
		scopes := make([]string, len(tool.Scopes))
		copy(scopes, tool.Scopes)
		tools = append(tools, types.PluginPreviewToolReview{
			Name:                 tool.Name,
			Description:          tool.Description,
			ReadOnly:             tool.ReadOnly,
			RequiresPersonalAuth: tool.RequiresPersonalAuth,
			Scopes:               scopes,
		})
	}
	return tools
}
