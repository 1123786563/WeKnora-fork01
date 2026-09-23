package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Tencent/WeKnora/internal/handler/dto"
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
// unbounded or zero TTL.
const defaultPluginPreviewTTL = 15 * time.Minute

func pluginPreviewTTL() time.Duration {
	raw := strings.TrimSpace(os.Getenv("PLUGIN_PREVIEW_TTL"))
	if raw == "" {
		return defaultPluginPreviewTTL
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return defaultPluginPreviewTTL
	}
	return d
}

// pluginService implements the PluginService interface. The EndpointLister
// seam is injected by the composition root (internal/container's
// NewPluginMCPEndpointLister over the airesource MCPManager) so this package
// never depends on the MCP client stack directly.
type pluginService struct {
	pluginRepo interfaces.PluginRepository
	lister     plugins.EndpointLister
}

// NewPluginService creates a new plugin service.
func NewPluginService(
	pluginRepo interfaces.PluginRepository,
	lister plugins.EndpointLister,
) interfaces.PluginService {
	return &pluginService{
		pluginRepo: pluginRepo,
		lister:     lister,
	}
}

// PreviewFromManifest fetches and verifies the manifest, then persists a
// TTL-bound preview carrying the identity fingerprint of exactly what was
// verified. The SSRF gate runs BEFORE any network I/O and before any
// persistence — a rejected URL leaves zero rows behind.
func (s *pluginService) PreviewFromManifest(
	ctx context.Context,
	tenantID uint64,
	actorID, manifestURL string,
) (*dto.PluginPreviewResponse, error) {
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
		logger.GetLogger(ctx).Errorf("failed to persist plugin preview: %v", err)
		return nil, fmt.Errorf("%w: %v", ErrPreviewPersistFailed, err)
	}

	return &dto.PluginPreviewResponse{
		PreviewID:           preview.ID,
		PluginID:            preview.PluginID,
		Version:             preview.Version,
		Name:                preview.Name,
		Description:         result.Manifest.Description,
		TransportType:       preview.TransportType,
		EndpointURL:         preview.EndpointURL,
		Tools:               previewToolsDTO(result.Snapshot),
		IdentityFingerprint: preview.IdentityFingerprint,
		ExpiresAt:           preview.ExpiresAt,
	}, nil
}

func previewToolsDTO(snapshot []types.PluginToolSnapshot) []dto.PluginPreviewTool {
	tools := make([]dto.PluginPreviewTool, 0, len(snapshot))
	for _, tool := range snapshot {
		scopes := append([]string(nil), tool.Scopes...)
		tools = append(tools, dto.PluginPreviewTool{
			Name:                 tool.Name,
			Description:          tool.Description,
			ReadOnly:             tool.ReadOnly,
			RequiresPersonalAuth: tool.RequiresPersonalAuth,
			Scopes:               scopes,
		})
	}
	return tools
}
