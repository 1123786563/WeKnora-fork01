package commercial

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// ErrBillingAccountConflict is the typed domain error for billing-account
// mapping conflicts: one space trying to take an external customer identity
// another space (or an earlier mapping of the same space) already holds.
var ErrBillingAccountConflict = errors.New("billing_account_conflict")

// Closed projection-local states of commercial_billing_accounts.
const (
	// BillingAccountStatePending: an ensure was attempted but no receipt or
	// authority snapshot has confirmed the customer yet (includes every
	// indeterminate platform outcome — timeout/server error — where nothing
	// is rolled back and the next ensure recovers by identity).
	BillingAccountStatePending = "pending"
	// BillingAccountStateLinked: a receipt or authority snapshot confirmed
	// the customer under the deterministic identity.
	BillingAccountStateLinked = "linked"
)

// BillingAccount is the REBUILDABLE Billing Projection row: the immutable
// mapping from one tenant to exactly one authority-side customer identity
// (ADR-0012; Lago billing migration spec, "Local persistence and state
// projection"). external_customer_id is derived purely from the tenant id
// (commercial.ExternalCustomerID) — rename and owner transfer cannot move
// it. The authority is the truth; this row is a cache and may be rebuilt by
// re-ensuring. ProviderCustomerRef is seam-internal and never crosses the
// Billing API.
type BillingAccount struct {
	TenantID            uint64     `gorm:"primaryKey;column:tenant_id"`
	ExternalCustomerID  string     `gorm:"column:external_customer_id;uniqueIndex;not null"`
	ProviderCustomerRef string     `gorm:"column:provider_customer_ref;not null;default:''"`
	State               string     `gorm:"column:state;not null;default:'pending'"`
	EnsuredAt           *time.Time `gorm:"column:ensured_at"`
	CreatedAt           time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt           time.Time  `gorm:"column:updated_at;not null"`
}

func (BillingAccount) TableName() string { return "commercial_billing_accounts" }

// BillingAccountStore owns the commercial_billing_accounts table. It
// follows the AccountStore.Bind precedent: the database constraints decide
// the single row under concurrency (INSERT ... ON CONFLICT DO NOTHING then
// re-read), conflict normalization is typed, and NO external call is ever
// made inside a transaction.
type BillingAccountStore struct{ db *gorm.DB }

func NewBillingAccountStore(db *gorm.DB) *BillingAccountStore { return &BillingAccountStore{db: db} }

// EnsurePending idempotently establishes the space's pending projection
// row: the first call inserts, every later call re-reads the existing row.
// Concurrent ensures of the same tenant converge on exactly one row (the
// tenant primary key decides); an external identity held by ANOTHER tenant
// is the typed ErrBillingAccountConflict.
func (s *BillingAccountStore) EnsurePending(ctx context.Context, a BillingAccount) (BillingAccount, error) {
	if a.TenantID == 0 || a.ExternalCustomerID == "" {
		return BillingAccount{}, ErrBillingAccountConflict
	}
	res := s.db.WithContext(ctx).Exec(
		`INSERT INTO commercial_billing_accounts (tenant_id, external_customer_id, state, created_at, updated_at)
		 VALUES (?, ?, 'pending', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 ON CONFLICT (tenant_id) DO NOTHING`,
		a.TenantID, a.ExternalCustomerID,
	)
	if res.Error != nil {
		// The external-id UNIQUE key fired across tenants: normalize to the
		// typed domain error instead of leaking the dialect error.
		var owner BillingAccount
		if lookupErr := s.db.WithContext(ctx).Where("external_customer_id = ?", a.ExternalCustomerID).
			First(&owner).Error; lookupErr == nil && owner.TenantID != a.TenantID {
			return BillingAccount{}, ErrBillingAccountConflict
		}
		return BillingAccount{}, res.Error
	}
	var got BillingAccount
	if err := s.db.WithContext(ctx).Where("tenant_id = ?", a.TenantID).First(&got).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// The insert no-op'd yet this tenant holds no row: the identity
			// must be owned elsewhere — surface the typed conflict.
			return BillingAccount{}, ErrBillingAccountConflict
		}
		return BillingAccount{}, err
	}
	if got.ExternalCustomerID != a.ExternalCustomerID {
		// Identity is immutable per tenant; a different derived identity for
		// the same space is a conflict, never a silent remap.
		return BillingAccount{}, ErrBillingAccountConflict
	}
	return got, nil
}

// MarkLinked confirms the customer: pending→linked on first confirmation,
// linked→linked as an idempotent refresh (a later recovery re-confirming
// the same identity must never fail).
func (s *BillingAccountStore) MarkLinked(ctx context.Context, tenantID uint64, ref string, at time.Time) (BillingAccount, error) {
	if tenantID == 0 {
		return BillingAccount{}, ErrBillingAccountConflict
	}
	res := s.db.WithContext(ctx).Exec(
		`UPDATE commercial_billing_accounts
		 SET state = 'linked', provider_customer_ref = ?, ensured_at = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE tenant_id = ?`,
		ref, at, tenantID,
	)
	if res.Error != nil {
		return BillingAccount{}, res.Error
	}
	return s.Get(ctx, tenantID)
}

// Get reads the tenant's row; every query is WHERE tenant_id = ? — there is
// no path through which one space reads another's mapping.
func (s *BillingAccountStore) Get(ctx context.Context, tenantID uint64) (BillingAccount, error) {
	var a BillingAccount
	err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&a).Error
	return a, err
}
