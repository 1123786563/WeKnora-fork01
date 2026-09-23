package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/handler/dto"
	"github.com/Tencent/WeKnora/internal/types"
)

// PluginRepository defines the data access for the plugin domain. T02 lands
// the preview slice; installation persistence (T06) extends this interface.
type PluginRepository interface {
	// CreatePreview persists one verified preview row.
	CreatePreview(ctx context.Context, p *types.PluginPreview) error

	// GetPreview retrieves a preview by ID within a tenant.
	// Not found returns (nil, nil) — the MCPServiceRepository convention.
	GetPreview(ctx context.Context, tenantID uint64, id string) (*types.PluginPreview, error)

	// MarkPreviewConsumed atomically flips consumed_at, exactly once and
	// only while unexpired: the UPDATE carries WHERE consumed_at IS NULL
	// AND expires_at > now in one statement, and a zero RowsAffected
	// yields gorm.ErrRecordNotFound — the caller reads that as "already
	// consumed, expired, or absent" (one rejection path, no check-then-act
	// window) and must reject the confirmation.
	MarkPreviewConsumed(ctx context.Context, tenantID uint64, id string) error
}

// PluginService defines the plugin business logic. T02 lands the manifest
// preview; confirm/discover/drift slices extend this interface later.
type PluginService interface {
	// PreviewFromManifest fetches and verifies a weknora.plugin/1 manifest,
	// persists the preview (identity fingerprint + TTL) and returns the
	// admin review payload. It never creates an installation.
	PreviewFromManifest(ctx context.Context, tenantID uint64, actorID, manifestURL string) (*dto.PluginPreviewResponse, error)
}
