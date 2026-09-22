package commercial

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var (
	// ErrProjectionNotFound: no benefits projection recorded for the tenant
	// (never ensured, or the chain stopped at a pending state).
	ErrProjectionNotFound = errors.New("projection_not_found")
	// ErrBatchNotFound: no credit batch registered for (tenant, period).
	ErrBatchNotFound = errors.New("credit_batch_not_found")
	// ErrBatchWalletConflict: a batch already carries a DIFFERENT wallet ref
	// — wallet identity must never be silently rewritten (E3 recovery
	// depends on the name/metadata identity staying stable).
	ErrBatchWalletConflict = errors.New("credit_batch_wallet_conflict")
	// ErrInvalidBenefitsRow: structural guard for both row types.
	ErrInvalidBenefitsRow = errors.New("invalid_benefits_row")
)

// Closed advisory states of a credit batch. Expiry itself is ALWAYS decided
// by ExpiresAt against now (the pre-dispatch registry read); State is
// bookkeeping for operators and reconciliation.
const (
	CreditBatchStateGranted = "granted"
	CreditBatchStateExpired = "expired"
)

// BenefitsRow is the rebuildable per-tenant billing projection: the plan the
// space is on, its closed entitlement/quota snapshot, and the freshness
// time. External identities (plan_code, external_subscription_id) are
// seam-internal columns — the Billing API maps through
// commercial_plan_publications.
type BenefitsRow struct {
	TenantID               uint64    `gorm:"primaryKey;column:tenant_id"`
	ExternalCustomerID     string    `gorm:"column:external_customer_id;not null"`
	ExternalSubscriptionID string    `gorm:"column:external_subscription_id;not null"`
	SubscriptionState      string    `gorm:"column:subscription_state;not null"` // active|pending
	PlanCode               string    `gorm:"column:plan_code;not null;default ''"`
	PlanKey                string    `gorm:"column:plan_key;not null;default ''"` // resolved via publications at write time
	PlanVersion            int64     `gorm:"column:plan_version;not null;default 0"`
	FeaturesJSON           string    `gorm:"column:features_json;not null;default '{}'"`
	LimitsJSON             string    `gorm:"column:limits_json;not null;default '{}'"`
	CreditsBalanceMicro    int64     `gorm:"column:credits_balance_micro;not null;default 0"`
	ProjectedAt            time.Time `gorm:"column:projected_at;not null"`
}

func (BenefitsRow) TableName() string { return "commercial_tenant_benefits" }

// CreditBatchRow is the coordinator's monthly batch registry (T03 verdict
// obligations: pre-dispatch expiry rejection, grant idempotency, wallet-cap
// accounting). One row per (tenant, period) — the unique constraint IS the
// first idempotency layer. WalletRef is the seam-internal wallet identity
// — the DETERMINISTIC WeKnora wallet name (the E3 recovery identity, since
// receipts stay frozen and never carry provider lago_ids); the registry
// also budgets the ≤6-wallet cap (monthly cadence occupies ≤ 2 transient
// slots — current month + the just-expired one awaiting the authority's
// hourly termination tick).
type CreditBatchRow struct {
	ID          int64     `gorm:"primaryKey;autoIncrement"`
	TenantID    uint64    `gorm:"column:tenant_id;uniqueIndex:uq_credit_batch_tenant_period"`
	Period      string    `gorm:"column:period;uniqueIndex:uq_credit_batch_tenant_period"`
	CommandKey  string    `gorm:"column:command_key;not null"` // grant_included_credits:<ext>:<period>
	WalletRef   string    `gorm:"column:wallet_ref;not null;default ''"` // seam-internal deterministic wallet name (E3 recovery identity); empty until the grant completes
	GrantedMicro int64    `gorm:"column:granted_micro;not null"`
	ExpiresAt   time.Time `gorm:"column:expires_at;not null"`
	State       string    `gorm:"column:state;not null"` // granted|expired (advisory; expiry is decided by ExpiresAt)
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
}

func (CreditBatchRow) TableName() string { return "commercial_credit_batches" }

// BenefitsStore owns the benefits projection and the credit batch registry.
// Every query is keyed by tenant — there is no path through which one space
// reads another's rows — and no external call is ever made inside the store.
type BenefitsStore struct{ db *gorm.DB }

func NewBenefitsStore(db *gorm.DB) *BenefitsStore { return &BenefitsStore{db: db} }

// EnsureSchema creates both tables with portable DDL (the planversion.go
// precedent), so tests and dev deployments are safe without running
// migrations 000180/000101.
func (s *BenefitsStore) EnsureSchema(ctx context.Context) error {
	if err := s.db.WithContext(ctx).Exec(`CREATE TABLE IF NOT EXISTS commercial_tenant_benefits (
		tenant_id                INTEGER   NOT NULL PRIMARY KEY,
		external_customer_id     TEXT      NOT NULL,
		external_subscription_id TEXT      NOT NULL,
		subscription_state       TEXT      NOT NULL,
		plan_code                TEXT      NOT NULL DEFAULT '',
		plan_key                 TEXT      NOT NULL DEFAULT '',
		plan_version             INTEGER   NOT NULL DEFAULT 0,
		features_json            TEXT      NOT NULL DEFAULT '{}',
		limits_json              TEXT      NOT NULL DEFAULT '{}',
		credits_balance_micro    INTEGER   NOT NULL DEFAULT 0,
		projected_at             DATETIME  NOT NULL
	)`).Error; err != nil {
		return err
	}
	// AUTOINCREMENT 是 SQLite 方言，PostgreSQL 下是语法错误（42601）导致启动 panic；
	// 非 sqlite 方言按规范迁移（migrations/versioned/000183）的 BIGSERIAL 建自增主键。
	if s.db.Dialector != nil && s.db.Dialector.Name() != "sqlite" {
		return s.db.WithContext(ctx).Exec(`CREATE TABLE IF NOT EXISTS commercial_credit_batches (
			id            BIGSERIAL   NOT NULL PRIMARY KEY,
			tenant_id     INTEGER  NOT NULL,
			period        TEXT     NOT NULL,
			command_key   TEXT     NOT NULL,
			wallet_ref    TEXT     NOT NULL DEFAULT '',
			granted_micro INTEGER  NOT NULL,
			expires_at    DATETIME NOT NULL,
			state         TEXT     NOT NULL,
			created_at    DATETIME NOT NULL,
			CONSTRAINT uq_credit_batch_tenant_period UNIQUE (tenant_id, period)
		)`).Error
	}
	return s.db.WithContext(ctx).Exec(`CREATE TABLE IF NOT EXISTS commercial_credit_batches (
		id            INTEGER  NOT NULL PRIMARY KEY AUTOINCREMENT,
		tenant_id     INTEGER  NOT NULL,
		period        TEXT     NOT NULL,
		command_key   TEXT     NOT NULL,
		wallet_ref    TEXT     NOT NULL DEFAULT '',
		granted_micro INTEGER  NOT NULL,
		expires_at    DATETIME NOT NULL,
		state         TEXT     NOT NULL,
		created_at    DATETIME NOT NULL,
		CONSTRAINT uq_credit_batch_tenant_period UNIQUE (tenant_id, period)
	)`).Error
}

// UpsertProjection writes the per-tenant projection (one row per tenant —
// the primary key decides under concurrency).
func (s *BenefitsStore) UpsertProjection(ctx context.Context, row BenefitsRow) error {
	if row.TenantID == 0 || row.ExternalCustomerID == "" || row.ExternalSubscriptionID == "" ||
		row.SubscriptionState == "" || row.ProjectedAt.IsZero() {
		return ErrInvalidBenefitsRow
	}
	return s.db.WithContext(ctx).Save(&row).Error
}

// GetProjection reads the tenant's projection row.
func (s *BenefitsStore) GetProjection(ctx context.Context, tenantID uint64) (BenefitsRow, error) {
	var row BenefitsRow
	err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return BenefitsRow{}, ErrProjectionNotFound
	}
	return row, err
}

// EnsureBatch is the concurrency-safe single-grant gate: inserts (tenant,
// period) once; on the unique race it returns the EXISTING row so the caller
// replays instead of re-granting. The DATABASE constraint decides under
// concurrency — insert then read-back, never check-then-insert. The guard is
// the addressing minimum (tenant, period, expiry); CommandKey/GrantedMicro/
// State are caller data the real grant path always fills.
func (s *BenefitsStore) EnsureBatch(ctx context.Context, row CreditBatchRow) (CreditBatchRow, bool, error) {
	if row.TenantID == 0 || row.Period == "" || row.ExpiresAt.IsZero() {
		return CreditBatchRow{}, false, ErrInvalidBenefitsRow
	}
	err := s.db.WithContext(ctx).Create(&row).Error
	if err == nil {
		return row, true, nil
	}
	if !isUniqueViolation(err) {
		return CreditBatchRow{}, false, err
	}
	existing, getErr := s.GetBatch(ctx, row.TenantID, row.Period)
	if getErr != nil {
		return CreditBatchRow{}, false, err
	}
	return existing, false, nil
}

// GetBatch reads one (tenant, period) registry row.
func (s *BenefitsStore) GetBatch(ctx context.Context, tenantID uint64, period string) (CreditBatchRow, error) {
	var row CreditBatchRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND period = ?", tenantID, period).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CreditBatchRow{}, ErrBatchNotFound
	}
	return row, err
}

// MarkBatchWallet records the seam-internal wallet identity on the batch.
// Setting the SAME ref again is an idempotent no-op; a DIFFERENT ref on a
// non-empty column is the typed conflict — wallet identity never silently
// rewrites.
func (s *BenefitsStore) MarkBatchWallet(ctx context.Context, id int64, walletRef string) error {
	if id == 0 || walletRef == "" {
		return ErrInvalidBenefitsRow
	}
	res := s.db.WithContext(ctx).Model(&CreditBatchRow{}).
		Where("id = ? AND (wallet_ref = '' OR wallet_ref = ?)", id, walletRef).
		Updates(map[string]any{"wallet_ref": walletRef, "state": CreditBatchStateGranted})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var current CreditBatchRow
		if err := s.db.WithContext(ctx).Where("id = ?", id).First(&current).Error; err != nil {
			return err
		}
		if current.WalletRef != walletRef {
			return ErrBatchWalletConflict
		}
		return nil // same ref, idempotent no-op through a different path
	}
	return nil
}

// ActiveBatches returns the tenant's batches whose ExpiresAt is strictly
// after now — the pre-dispatch registry read. A batch at expires_at == now
// is ALREADY unavailable: the lazy-termination window (~65 min, E1) must
// never leak spendable-looking credits.
func (s *BenefitsStore) ActiveBatches(ctx context.Context, tenantID uint64, now time.Time) ([]CreditBatchRow, error) {
	var rows []CreditBatchRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND expires_at > ?", tenantID, now).
		Order("expires_at").Find(&rows).Error
	return rows, err
}

// ListBatches returns ALL of the tenant's registry rows (the full batch
// breakdown surface — expired batches list with zero available balance via
// the caller's expiry overlay).
func (s *BenefitsStore) ListBatches(ctx context.Context, tenantID uint64) ([]CreditBatchRow, error) {
	var rows []CreditBatchRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).Order("expires_at").Find(&rows).Error
	return rows, err
}

// SetBatchExpired flips the advisory state of one batch.
func (s *BenefitsStore) SetBatchExpired(ctx context.Context, id int64) error {
	res := s.db.WithContext(ctx).Model(&CreditBatchRow{}).
		Where("id = ?", id).Update("state", CreditBatchStateExpired)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrBatchNotFound
	}
	return nil
}

// ---- resource counters + occupancy (the quota enforcement surface) ----

var (
	// ErrCounterLimitReached: the conditional counter UPDATE refused a
	// growth delta (hard limit or zero floor) — the ErrQuotaExceeded-class
	// sentinel behind the ResourceQuotaGuard.
	ErrCounterLimitReached = errors.New("counter_limit_reached")
)

// ReserveCounterDelta atomically applies one delta to the tenant's resource
// counter with the hard-limit CAS: the INSERT-then-conditional-UPDATE pair
// runs in ONE transaction, so two concurrent adds at the boundary admit
// exactly the allowed number. delta <= 0 always passes (超限状态 cleanup:
// removal/decrement must never be blocked); a refused delta changes
// nothing. This is the ONE place the counter CAS SQL lives
// (CommercialHandler.ReserveResource delegates here).
func (s *BenefitsStore) ReserveCounterDelta(ctx context.Context, tenantID uint64, dimension string, delta int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`INSERT INTO commercial_resource_counters (tenant_id, resource, used, hard_limit)
			VALUES (?, ?, 0, NULL) ON CONFLICT (tenant_id, resource) DO NOTHING`, tenantID, dimension).Error; err != nil {
			return err
		}
		res := tx.Exec(`UPDATE commercial_resource_counters SET used = used + ?
			WHERE tenant_id = ? AND resource = ? AND used + ? >= 0
			AND (hard_limit IS NULL OR used + ? <= hard_limit)`,
			delta, tenantID, dimension, delta, delta)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrCounterLimitReached
		}
		return nil
	})
}

// SyncCounter writes one counter row's observed occupancy and its hard
// limit in one statement (NULL limit = unlimited — the fail-open degraded
// posture for unprojected dimensions). Called ONLY by the projection
// refresh, which re-syncs occupancy BEFORE applying limits (ordering is
// binding: 超限状态 must reflect real overage, never counter drift).
func (s *BenefitsStore) SyncCounter(ctx context.Context, tenantID uint64, dimension string, used int64, hardLimit *int64) error {
	return s.db.WithContext(ctx).Exec(`INSERT INTO commercial_resource_counters (tenant_id, resource, used, hard_limit)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (tenant_id, resource) DO UPDATE SET used = excluded.used, hard_limit = excluded.hard_limit`,
		tenantID, dimension, used, hardLimit).Error
}

// ObserveOccupancy reads the tenant's REAL occupancy: the active member
// count and the tenants.storage_used bytes (missing rows count as zero).
// This is the re-sync source of truth the quota limits apply against.
func (s *BenefitsStore) ObserveOccupancy(ctx context.Context, tenantID uint64) (members int64, storageBytes int64, err error) {
	if err = s.db.WithContext(ctx).Raw(
		`SELECT COUNT(*) FROM tenant_members
		WHERE tenant_id = ? AND status = 'active' AND deleted_at IS NULL`, tenantID).
		Scan(&members).Error; err != nil {
		return 0, 0, err
	}
	if err = s.db.WithContext(ctx).Raw(
		`SELECT COALESCE(storage_used, 0) FROM tenants WHERE id = ?`, tenantID).
		Scan(&storageBytes).Error; err != nil {
		return 0, 0, err
	}
	return members, storageBytes, nil
}
