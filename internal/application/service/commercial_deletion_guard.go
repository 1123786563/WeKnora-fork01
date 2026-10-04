package service

import (
	"context"
	"errors"
	"fmt"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"

	"gorm.io/gorm"
)

// CommercialDeletionGuard is the production TenantDeletionGuard (O02, #102
// / Lago 30): tombstone-first scheduling stop (the closure tombstone every
// new-commercial-work entry refuses on), tenant-scoped connection grant
// revocation, the pending-work listing that refuses deletion, and — once
// nothing is pending — the authority-side closure that must confirm before
// the workspace row goes away. Financial records are never deleted here;
// they stay under the retention policy.
type CommercialDeletionGuard struct {
	closures *commercialsvc.WorkspaceClosureService
	grants   *appconnectorrepo.SpaceConnectionGrantStore
	db       *gorm.DB
}

// NewCommercialDeletionGuard wires the guard. platform may be nil
// (blocked-env): scheduling still stops and deletion still refuses on
// pending work; the closure stays in state closing until a seam is wired
// (deletion then refuses on the unconfirmed closure — fail closed).
func NewCommercialDeletionGuard(db *gorm.DB, platform domain.CommercialPlatform) (*CommercialDeletionGuard, error) {
	if db == nil {
		return nil, errors.New("commercial deletion guard requires a database")
	}
	closures, err := commercialsvc.NewWorkspaceClosureService(db, platform)
	if err != nil {
		return nil, err
	}
	return &CommercialDeletionGuard{
		closures: closures,
		grants:   appconnectorrepo.NewSpaceConnectionGrantStore(db),
		db:       db,
	}, nil
}

// DisableNewScheduling lands the closure tombstone: from this instant new
// purchases, billing-account ensures and charge reservations are refused.
func (g *CommercialDeletionGuard) DisableNewScheduling(ctx context.Context, tenantID uint64) error {
	return g.closures.DisableNewScheduling(ctx, tenantID)
}

// RevokeConnections removes every space-connection grant of the tenant so
// no dispatch races the deletion.
func (g *CommercialDeletionGuard) RevokeConnections(ctx context.Context, tenantID uint64) error {
	return g.grants.RevokeTenantGrants(ctx, tenantID)
}

// PendingCommercialWork lists the unsettled commercial records that refuse
// deletion: unconfirmed settlements, pending-payment orders, and in-flight
// refunds — by their durable keys, the operator's lookup handles.
func (g *CommercialDeletionGuard) PendingCommercialWork(ctx context.Context, tenantID uint64) ([]string, error) {
	pending := make([]string, 0, 3)
	var keys []string
	if err := g.db.WithContext(ctx).Raw(`SELECT key FROM commercial_settlement_records
		WHERE tenant_id = ? AND state <> 'confirmed'`, tenantID).Scan(&keys).Error; err != nil {
		return nil, err
	}
	for _, k := range keys {
		pending = append(pending, "settlement:"+k)
	}
	if err := g.db.WithContext(ctx).Raw(`SELECT id FROM commercial_orders
		WHERE tenant_id = ? AND state = 'pending'`, tenantID).Scan(&keys).Error; err != nil {
		return nil, err
	}
	for _, k := range keys {
		pending = append(pending, "payment:"+k)
	}
	if err := g.db.WithContext(ctx).Raw(`SELECT id FROM commercial_refunds
		WHERE tenant_id = ? AND state IN ('requested','reviewing','pending','revocation_pending')`,
		tenantID).Scan(&keys).Error; err != nil {
		return nil, err
	}
	for _, k := range keys {
		pending = append(pending, "refund:"+k)
	}
	return pending, nil
}

// RetentionPolicyVersion answers the configured retention policy version.
// No version is configured today: records are preserved indefinitely
// (CheckDeletionReadiness never enables auto-delete without an explicit
// version) — the honest answer, not a fabricated one.
func (g *CommercialDeletionGuard) RetentionPolicyVersion() string { return "" }

// CloseCommercialWorkspace runs the closure disposal (terminate charging
// objects on the authority, de-identify the customer, cap the local paid
// term). A platform failure refuses the deletion — the workspace row stays
// until the closure is confirmed.
func (g *CommercialDeletionGuard) CloseCommercialWorkspace(ctx context.Context, tenantID uint64) error {
	if _, err := g.closures.CloseWorkspace(ctx, tenantID, "tenant-deletion", commercialsvc.WorkspaceClosureReason); err != nil {
		return fmt.Errorf("commercial closure for tenant %d: %w", tenantID, err)
	}
	return nil
}

var _ TenantDeletionGuard = (*CommercialDeletionGuard)(nil)
