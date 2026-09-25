package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// pluginRepository implements the PluginRepository interface.
// Every query is parameter-bound — no string-assembled SQL.
type pluginRepository struct {
	db *gorm.DB
}

// ErrInstallationDuplicateKey marks a (tenant_id, plugin_id) unique-index
// conflict on CreateInstallation (OCR round-1 R12 F15): two concurrent
// confirms of the same plugin both pass the service's duplicate check, and
// the loser must read this as "already installed" (409), never as a raw
// driver error mapped to a blanket 500.
var ErrInstallationDuplicateKey = errors.New("plugin installation already exists for this workspace")

// NewPluginRepository creates a new plugin repository.
func NewPluginRepository(db *gorm.DB) interfaces.PluginRepository {
	return &pluginRepository{db: db}
}

// CreatePreview persists one verified preview row.
func (r *pluginRepository) CreatePreview(ctx context.Context, p *types.PluginPreview) error {
	return r.db.WithContext(ctx).Create(p).Error
}

// GetPreview retrieves a preview by ID within a tenant; not found is
// (nil, nil) per the repository convention.
func (r *pluginRepository) GetPreview(ctx context.Context, tenantID uint64, id string) (*types.PluginPreview, error) {
	var preview types.PluginPreview
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&preview).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &preview, nil
}

// MarkPreviewConsumed flips consumed_at exactly once, with the TTL verdict
// INSIDE the same atomic UPDATE: the WHERE clause carries both
// consumed_at IS NULL and expires_at > now. A second call, a call against
// an absent ID, or a call after the preview crossed its expiry boundary all
// affect zero rows, which surfaces as gorm.ErrRecordNotFound — the
// confirmation path reads one rejection for "already consumed / expired /
// absent" and there is no check-then-act window between an Expired() read
// and this update.
func (r *pluginRepository) MarkPreviewConsumed(ctx context.Context, tenantID uint64, id string) error {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&types.PluginPreview{}).
		Where("tenant_id = ? AND id = ? AND consumed_at IS NULL AND expires_at > ?", tenantID, id, now).
		Update("consumed_at", now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteExpiredPreviews drops preview rows past their expiry, consumed or
// not: a preview is a TTL-bound review artifact (expires_at is the TTL
// already shown to the admin), not an audit record — the table must not
// grow without bound (整分支 OCR 一轮 F1). Parameter-bound via gorm, the
// same shape as DeleteExpiredGrants; the service triggers it lazily and
// best-effort on the preview success path.
func (r *pluginRepository) DeleteExpiredPreviews(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).
		Where("expires_at <= ?", before).
		Delete(&types.PluginPreview{}).Error
}

// CreateInstallation persists one accepted installation row. A unique-index
// conflict on (tenant_id, plugin_id) surfaces as ErrInstallationDuplicateKey
// (OCR round-1 R12 F15) — PostgreSQL 23505, gorm's translated sentinel, and
// SQLite's "UNIQUE constraint failed" text are all recognized.
func (r *pluginRepository) CreateInstallation(ctx context.Context, inst *types.PluginInstallation) error {
	err := r.db.WithContext(ctx).Create(inst).Error
	if err != nil && isInstallationDuplicateKey(err) {
		return ErrInstallationDuplicateKey
	}
	return err
}

// isInstallationDuplicateKey recognizes the (tenant_id, plugin_id) unique
// conflict across the supported drivers.
func isInstallationDuplicateKey(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) || errors.Is(err, ErrInstallationDuplicateKey) {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// GetInstallation retrieves an installation by ID within a tenant; not
// found (including a foreign tenant's ID) is (nil, nil) — existence is not
// leaked across tenants.
func (r *pluginRepository) GetInstallation(ctx context.Context, tenantID uint64, id string) (*types.PluginInstallation, error) {
	var inst types.PluginInstallation
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&inst).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &inst, nil
}

// GetInstallationByTenantPlugin retrieves the unique installation of one
// plugin within a tenant; not found is (nil, nil).
func (r *pluginRepository) GetInstallationByTenantPlugin(ctx context.Context, tenantID uint64, pluginID string) (*types.PluginInstallation, error) {
	var inst types.PluginInstallation
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND plugin_id = ?", tenantID, pluginID).
		First(&inst).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &inst, nil
}

// GetByServiceID retrieves the installation bound to a materialized service
// ID within a tenant; not found (including manual services) is (nil, nil).
// The runtime snapshot guard resolves accepted capability through this.
func (r *pluginRepository) GetByServiceID(ctx context.Context, tenantID uint64, serviceID string) (*types.PluginInstallation, error) {
	if serviceID == "" {
		return nil, nil
	}
	var inst types.PluginInstallation
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND service_id = ?", tenantID, serviceID).
		First(&inst).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &inst, nil
}

// ListInstallationsByTenant returns all installations of a tenant.
func (r *pluginRepository) ListInstallationsByTenant(ctx context.Context, tenantID uint64) ([]*types.PluginInstallation, error) {
	var installations []*types.PluginInstallation
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at ASC").
		Find(&installations).Error
	return installations, err
}

// UpdateInstallationState flips state in place and refreshes updated_at;
// a zero-row update surfaces as gorm.ErrRecordNotFound.
func (r *pluginRepository) UpdateInstallationState(ctx context.Context, tenantID uint64, id, state string) error {
	result := r.db.WithContext(ctx).
		Model(&types.PluginInstallation{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]interface{}{"state": state, "updated_at": time.Now()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateInstallationServiceID backfills the materialized service binding.
func (r *pluginRepository) UpdateInstallationServiceID(ctx context.Context, tenantID uint64, id, serviceID string) error {
	result := r.db.WithContext(ctx).
		Model(&types.PluginInstallation{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]interface{}{"service_id": serviceID, "updated_at": time.Now()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateInstallationAccepted persists an accepted upgrade in place (T16):
// accepted_version, endpoint_url, tools_snapshot, tools_digest and the drift
// reset (drift_state='none', drift_detail=NULL — accepting IS the drift
// healing write). It is also the compensation write-back of the upgrade
// flow: the service re-invokes it with the memory-held OLD values when a
// later step (materialized service sync, policy increments) fails, which is
// why every field is a parameter rather than a partial patch. Consumed via
// the service's installationUpgradeWriter capability seam (plugin_install_
// service.go) — the T06-era PluginRepository contract predates the upgrade
// slice and is extended additively here rather than reshaped. A zero-row
// update surfaces as gorm.ErrRecordNotFound; all values are parameter-bound.
func (r *pluginRepository) UpdateInstallationAccepted(
	ctx context.Context, tenantID uint64, id, acceptedVersion, endpointURL string,
	toolsSnapshot types.PluginPreviewTools, toolsDigest string,
) error {
	result := r.db.WithContext(ctx).
		Model(&types.PluginInstallation{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(map[string]interface{}{
			"accepted_version": acceptedVersion,
			"endpoint_url":     endpointURL,
			"tools_snapshot":   toolsSnapshot,
			"tools_digest":     toolsDigest,
			"drift_state":      types.PluginDriftNone,
			"drift_detail":     nil,
			"updated_at":       time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteInstallationToolPolicies removes the per-tool policy rows an upgrade
// accept CREATED in that call (T16-OCR1-F2 compensation): when the incremental
// policy loop fails mid-way, the rows already landed for tools of the
// candidate snapshot must go — the compensation restores the OLD snapshot,
// and a residual row would (a) sit as a governance record for a tool that is
// not in the restored snapshot and (b) short-circuit the install-time rule
// (a NEW tool lands Enabled=ReadOnly) on a later accept of a candidate that
// reclassifies the same name as a write tool, landing it exposed. The plugin
// domain owns its derived policy rows (the HardDeleteServiceCascade
// discipline); MCPToolApproval carries no soft-delete column, so this is a
// hard, parameter-bound delete. Consumed via the service's
// installationUpgradeWriter capability seam.
func (r *pluginRepository) DeleteInstallationToolPolicies(
	ctx context.Context, tenantID uint64, serviceID string, toolNames []string,
) error {
	if serviceID == "" || len(toolNames) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND service_id = ? AND tool_name IN ?", tenantID, serviceID, toolNames).
		Delete(&types.MCPToolApproval{}).Error
}

// DeleteInstallation HARD-deletes (Unscoped) — this is the compensation
// path of ConfirmInstallation: a failed materialization must free the
// (tenant_id, plugin_id) unique slot. Installations carry no soft-delete
// column; the row either is the tenant's accepted baseline or does not
// exist.
func (r *pluginRepository) DeleteInstallation(ctx context.Context, tenantID uint64, id string) error {
	return r.db.WithContext(ctx).
		Unscoped().
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&types.PluginInstallation{}).Error
}

// HardDeleteServiceCascade HARD-deletes a plugin-materialized MCP service
// row together with EVERY service_id-keyed derived row (OCR round-1 R12
// F21, T06-OCR1-F5): mcp_tool_approvals, mcp_oauth_tokens (member personal
// tokens, AES-256-GCM at rest) and mcp_oauth_clients on every driver, plus
// mcp_metadata on PostgreSQL (dialect-gated — the table has no SQLite
// twin). Without the full sweep, uninstalling after members authorized
// would strand sensitive credential rows on a dead service forever. The
// mcp_tool_approvals FK only cascades on a hard DELETE, the shared
// MCPServiceRepository.Delete is a soft delete, and production SQLite DSNs
// never enable foreign_keys — these explicit deletes are the only cleanup
// there is. The plugin domain owns its derived rows, so the cascade lives
// here rather than on the shared MCP service repository.
func (r *pluginRepository) HardDeleteServiceCascade(ctx context.Context, tenantID uint64, serviceID string) error {
	if serviceID == "" {
		return nil
	}
	// Derived rows FIRST (T06-OCR1-F5): every service_id-keyed table must be
	// swept before the service row goes — member OAuth tokens (AES-256-GCM
	// at rest) and dynamic-client registrations would otherwise strand on a
	// dead service forever. Production SQLite DSNs never enable
	// foreign_keys, so these explicit deletes are the only cleanup there is.
	// The whole sweep runs in ONE transaction (T06-OCR2-F1): 4~5
	// independent DELETEs must land all-or-nothing — a mid-sweep failure
	// with partial deletes stranded an un-replayable half-clean state (and
	// the compensation path would then drop the installation row, leaving
	// an orphan service row with no self-heal anchor). This is a
	// single-repo-method transaction, not the cross-service WithTx the plan
	// ruled out; db.Transaction here is the repository-layer convention.
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		deleteDerived := func(model any) error {
			return tx.Where("tenant_id = ? AND service_id = ?", tenantID, serviceID).
				Delete(model).Error
		}
		if err := deleteDerived(&types.MCPToolApproval{}); err != nil {
			return err
		}
		if err := deleteDerived(&types.MCPOAuthToken{}); err != nil {
			return err
		}
		if err := deleteDerived(&types.MCPOAuthClient{}); err != nil {
			return err
		}
		if r.db.Dialector.Name() == "postgres" {
			// mcp_metadata is a PostgreSQL-only table — the SQLite migration
			// stream never creates it, so the delete is dialect-gated.
			if err := deleteDerived(&types.MCPMetadata{}); err != nil {
				return err
			}
		}
		return tx.Unscoped().
			Where("tenant_id = ? AND id = ?", tenantID, serviceID).
			Delete(&types.MCPService{}).Error
	})
}
