package interfaces

import (
	"context"
	"time"

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

	// DeleteExpiredPreviews drops preview rows past their expiry, consumed
	// or not: a preview is a TTL-bound review artifact (expires_at is the TTL
	// already shown to the admin), not an audit record. The service
	// triggers this lazily and best-effort on the preview success path —
	// a cleanup failure must never block an admin's preview.
	DeleteExpiredPreviews(ctx context.Context, before time.Time) error

	// CreateInstallation persists one accepted installation row.
	CreateInstallation(ctx context.Context, inst *types.PluginInstallation) error

	// GetInstallation retrieves an installation by ID within a tenant.
	// Not found (including a foreign tenant's ID) returns (nil, nil) —
	// the MCPServiceRepository convention; existence is not leaked.
	GetInstallation(ctx context.Context, tenantID uint64, id string) (*types.PluginInstallation, error)

	// GetInstallationByTenantPlugin retrieves the (unique) installation of
	// pluginID within a tenant. Not found returns (nil, nil).
	GetInstallationByTenantPlugin(ctx context.Context, tenantID uint64, pluginID string) (*types.PluginInstallation, error)

	// GetByServiceID retrieves the installation bound to a materialized MCP
	// service ID within a tenant (the runtime snapshot guard's lookup).
	// Not found returns (nil, nil) — including manual services, which have
	// no installation row by construction.
	GetByServiceID(ctx context.Context, tenantID uint64, serviceID string) (*types.PluginInstallation, error)

	// ListInstallationsByTenant returns all installations of a tenant.
	ListInstallationsByTenant(ctx context.Context, tenantID uint64) ([]*types.PluginInstallation, error)

	// UpdateInstallationState flips state in place. A zero-row update
	// returns gorm.ErrRecordNotFound.
	UpdateInstallationState(ctx context.Context, tenantID uint64, id, state string) error

	// UpdateInstallationServiceID backfills the materialized service
	// binding after CreateMCPService succeeds.
	UpdateInstallationServiceID(ctx context.Context, tenantID uint64, id, serviceID string) error

	// DeleteInstallation HARD-deletes an installation row — the
	// compensation path of ConfirmInstallation: a failed materialization
	// must free the (tenant_id, plugin_id) unique slot so the admin can
	// retry. Installations are governance state, not audit history.
	DeleteInstallation(ctx context.Context, tenantID uint64, id string) error
}

// PluginService defines the plugin business logic. T02 landed the manifest
// preview; the install slice (T06/T07) adds confirm, state and discovery.
type PluginService interface {
	// PreviewFromManifest fetches and verifies a weknora.plugin/1 manifest,
	// persists the preview (identity fingerprint + TTL) and returns the
	// admin review payload. It never creates an installation. The result is
	// a types-layer struct — this interfaces package must not depend on the
	// handler layer (the handler maps it onto its DTO).
	PreviewFromManifest(ctx context.Context, tenantID uint64, actorID, manifestURL string) (*types.PluginPreviewResult, error)

	// ConfirmInstallation consumes one verified preview and installs the
	// pinned version for the tenant: re-verifies the remote (digest +
	// identity fingerprint), writes the installation row, materializes the
	// MCP service and per-tool approval rows (read tools enabled, WRITE
	// TOOLS DISABLED), and marks the preview consumed. Any failure after
	// the installation row exists is compensated (materialized service +
	// installation row removed) so (tenant, plugin) stays installable.
	ConfirmInstallation(ctx context.Context, tenantID uint64, actorID, previewID string) (*types.PluginInstallationResult, error)

	// SetInstallationState disables/enables an installation and syncs the
	// materialized MCP service's Enabled flag (state ∈ {active, disabled}).
	SetInstallationState(ctx context.Context, tenantID uint64, installationID, state string) (*types.PluginInstallationResult, error)

	// ListInstallations returns the member-facing summaries of every
	// installation in the tenant.
	ListInstallations(ctx context.Context, tenantID uint64) ([]*types.PluginInstallationSummary, error)

	// GetInstallation returns one installation's full view within the
	// tenant; a foreign tenant's ID is "not found".
	GetInstallation(ctx context.Context, tenantID uint64, installationID string) (*types.PluginInstallationResult, error)
}
