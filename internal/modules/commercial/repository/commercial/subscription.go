package commercial

import (
	"context"
	"errors"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/gorm"
)

var (
	// ErrSubscriptionNotFound reports a space with no purchased subscription
	// (base tier): there is nothing to change yet.
	ErrSubscriptionNotFound = errors.New("subscription_not_found")
	// ErrSubscriptionVersionConflict reports a lost race: the subscription
	// changed since the caller read it; the client must cut a new quote.
	ErrSubscriptionVersionConflict = errors.New("subscription_version_conflict")
	// ErrScheduledChangeExists reports that a scheduled switch is already
	// pending: a second arrangement must explicitly supersede it, never
	// silently overwrite it.
	ErrScheduledChangeExists = errors.New("scheduled_change_exists")
)

// Benefit job states. pending = claimed and awaiting issuance/confirmation,
// granted = issued (external batch recorded when the external issuer is
// authoritative), failed = issuance attempted and errored; the key never
// changes across state transitions.
const (
	JobStatePending = "pending"
	JobStateGranted = "granted"
	JobStateFailed  = "failed"
)

// Subscription is the durable mirror of one external commercial subscription.
// ID is the external subscription ID; FutureIntervalJSON carries the
// purchased future interval (e.g. an early renewal window) so ticks can only
// ever grant months that have actually elapsed. ProjectionPlanJSON and
// DowngradeReason record the expiry downgrade projection; nothing is ever
// deleted on expiry.
type Subscription struct {
	ID                 string    `gorm:"primaryKey;column:id"`
	TenantID           uint64    `gorm:"column:tenant_id;uniqueIndex;not null"`
	PlanKey            string    `gorm:"column:plan_key;not null"`
	PlanVersion        int64     `gorm:"column:plan_version;not null"`
	PlanSnapshotJSON   string    `gorm:"column:plan_snapshot_json;not null"`
	Anchor             time.Time `gorm:"column:anchor;not null"`
	PaidUntil          time.Time `gorm:"column:paid_until;not null"`
	FutureIntervalJSON string    `gorm:"column:future_interval_json;not null;default:'{}'"`
	// ScheduledChangeJSON carries ONE pending scheduled plan switch in the
	// domain ScheduledPlanChange shape. It deliberately does NOT reuse
	// future_interval_json: that column holds PURCHASED future intervals
	// (e.g. early renewals), which a downgrade must never overwrite.
	ScheduledChangeJSON string `gorm:"column:scheduled_change_json;not null;default:''"`
	Version             int64  `gorm:"column:version;not null;default:1"`
	ProjectionPlanJSON  string `gorm:"column:projection_plan_json;not null;default:''"`
	DowngradeReason     string `gorm:"column:downgrade_reason;not null;default:''"`
}

func (Subscription) TableName() string { return "commercial_subscriptions" }

// BenefitJob is one scheduled benefit grant. Key is globally unique - monthly
// grants use the MonthlyGrantKey namespace, while upgrade deltas use the
// order_line_id namespace; the two never collide. ExternalRef stores the
// external batch ID once an external issuance succeeds.
type BenefitJob struct {
	Key            string    `gorm:"primaryKey;column:key"`
	TenantID       uint64    `gorm:"column:tenant_id;not null"`
	SubscriptionID string    `gorm:"column:subscription_id;not null"`
	MonthStart     time.Time `gorm:"column:month_start;not null"`
	Credits        int64     `gorm:"column:credits;not null"`
	State          string    `gorm:"column:state;not null"`
	ExternalRef    string    `gorm:"column:external_ref;not null;default:''"`
	OrderLineID    string    `gorm:"column:order_line_id;not null;default:''"`
	CreatedAt      time.Time `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null"`
}

func (BenefitJob) TableName() string { return "commercial_benefit_jobs" }

// SubscriptionStore persists commercial subscriptions and benefit jobs.
// Created via the 000112_commercial_lifecycle (PostgreSQL) /
// 000032_commercial_lifecycle (SQLite) migration pair; tests may AutoMigrate
// the gorm models directly.
type SubscriptionStore struct{ db *gorm.DB }

func NewSubscriptionStore(db *gorm.DB) *SubscriptionStore { return &SubscriptionStore{db: db} }

// SaveSubscription upserts the subscription row (renewal rewrites paid_until
// and the future interval in place).
func (s *SubscriptionStore) SaveSubscription(ctx context.Context, sub *Subscription) error {
	if sub.Version == 0 {
		sub.Version = 1
	}
	if sub.FutureIntervalJSON == "" {
		sub.FutureIntervalJSON = "{}"
	}
	return s.db.WithContext(ctx).Exec(`INSERT INTO commercial_subscriptions
		(id, tenant_id, plan_key, plan_version, plan_snapshot_json, anchor, paid_until,
		 future_interval_json, scheduled_change_json, version, projection_plan_json, downgrade_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			tenant_id = excluded.tenant_id,
			plan_key = excluded.plan_key,
			plan_version = excluded.plan_version,
			plan_snapshot_json = excluded.plan_snapshot_json,
			anchor = excluded.anchor,
			paid_until = excluded.paid_until,
			future_interval_json = excluded.future_interval_json,
			scheduled_change_json = excluded.scheduled_change_json,
			version = excluded.version,
			projection_plan_json = excluded.projection_plan_json,
			downgrade_reason = excluded.downgrade_reason`,
		sub.ID, sub.TenantID, sub.PlanKey, sub.PlanVersion, sub.PlanSnapshotJSON,
		sub.Anchor, sub.PaidUntil, sub.FutureIntervalJSON, sub.ScheduledChangeJSON, sub.Version,
		sub.ProjectionPlanJSON, sub.DowngradeReason).Error
}

// ListSubscriptions returns every subscription row for a tick.
func (s *SubscriptionStore) ListSubscriptions(ctx context.Context) ([]Subscription, error) {
	var subs []Subscription
	err := s.db.WithContext(ctx).Order("id").Find(&subs).Error
	return subs, err
}

// Current returns the tenant's subscription row. A space on the base tier
// (no row yet) reports ErrSubscriptionNotFound — the caller decides whether
// that is a first purchase instead of a plan change.
func (s *SubscriptionStore) Current(ctx context.Context, tenantID uint64) (Subscription, error) {
	var sub Subscription
	err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&sub).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Subscription{}, ErrSubscriptionNotFound
	}
	return sub, err
}

// LatestVersion returns the subscription version quotes are cut against:
// the newest version of the space's row, or 0 while it is on the base tier.
func (s *SubscriptionStore) LatestVersion(ctx context.Context, tenantID uint64) (int64, error) {
	var version int64
	err := s.db.WithContext(ctx).Raw(
		`SELECT version FROM commercial_subscriptions
		WHERE tenant_id = ? ORDER BY version DESC LIMIT 1`, tenantID).Scan(&version).Error
	return version, err
}

// SchedulePlanChange atomically consumes the quote (the marker records what
// consumed it) and records the scheduled switch in its OWN column under a
// version guard: a purchased future interval in future_interval_json is
// never touched, and a second pending arrangement is refused instead of
// silently overwriting the first. The guarded UPDATE bumps version only
// when the caller saw the current one, so a concurrent change loses
// cleanly and the client re-quotes.
func (s *SubscriptionStore) SchedulePlanChange(ctx context.Context, sub Subscription, change domain.ScheduledPlanChange, quoteID string, quoteSubscriptionVersion int64, marker string, now time.Time) error {
	blob, err := change.JSON()
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current Subscription
		if err := tx.Where("id = ?", sub.ID).First(&current).Error; err != nil {
			return err
		}
		if current.ScheduledChangeJSON != "" {
			return ErrScheduledChangeExists
		}
		if _, err := consumeQuoteTx(tx, quoteID, quoteSubscriptionVersion, marker, now); err != nil {
			return err
		}
		res := tx.Model(&Subscription{}).
			Where("id = ? AND version = ? AND scheduled_change_json = ''", sub.ID, sub.Version).
			Updates(map[string]interface{}{
				"scheduled_change_json": blob,
				"version":               sub.Version + 1,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrSubscriptionVersionConflict
		}
		return nil
	})
}

// ApplyDueScheduledChange applies a scheduled switch whose effective time
// has passed: the subscription's plan fields switch to the scheduled
// target (snapshot resolved from the immutable published catalog row),
// the arrangement is cleared and the version advances — all in ONE
// transaction guarded on the exact stored JSON, so concurrent ticks
// apply it exactly once and a lost race is a clean no-op.
func (s *SubscriptionStore) ApplyDueScheduledChange(ctx context.Context, sub Subscription, now time.Time) (bool, error) {
	change, ok := domain.ParseScheduledPlanChange(sub.ScheduledChangeJSON)
	if !ok || !change.Due(now) {
		return false, nil
	}
	res := s.db.WithContext(ctx).Model(&Subscription{}).
		Where("id = ? AND version = ? AND scheduled_change_json = ?", sub.ID, sub.Version, sub.ScheduledChangeJSON).
		Updates(map[string]interface{}{
			"plan_key":              change.PlanKey,
			"plan_version":          change.PlanVersion,
			"plan_snapshot_json":    change.PlanSnapshotJSON,
			"scheduled_change_json": "",
			"version":               sub.Version + 1,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// ClaimBenefitJob inserts the job guarded by its unique key
// (INSERT ... ON CONFLICT DO NOTHING). It returns claimed=true for exactly
// one caller regardless of how many workers race on the same period; every
// other caller gets claimed=false and must leave the existing row alone.
func (s *SubscriptionStore) ClaimBenefitJob(ctx context.Context, job BenefitJob) (bool, error) {
	now := time.Now().UTC()
	res := s.db.WithContext(ctx).Exec(`INSERT INTO commercial_benefit_jobs
		(key, tenant_id, subscription_id, month_start, credits, state, external_ref, order_line_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (key) DO NOTHING`,
		job.Key, job.TenantID, job.SubscriptionID, job.MonthStart, job.Credits,
		job.State, job.ExternalRef, job.OrderLineID, now, now)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// GetBenefitJob loads one job by key.
func (s *SubscriptionStore) GetBenefitJob(ctx context.Context, key string) (BenefitJob, error) {
	var job BenefitJob
	err := s.db.WithContext(ctx).Where("key = ?", key).First(&job).Error
	return job, err
}

// ListBenefitJobs returns all jobs ordered by key.
func (s *SubscriptionStore) ListBenefitJobs(ctx context.Context) ([]BenefitJob, error) {
	var jobs []BenefitJob
	err := s.db.WithContext(ctx).Order("key").Find(&jobs).Error
	return jobs, err
}

// CompleteBenefitJob advances a claimed job's state and records the external
// batch reference on success. The key is never rewritten, and an existing
// external_ref is never blanked by a retry.
func (s *SubscriptionStore) CompleteBenefitJob(ctx context.Context, key, state, externalRef string) error {
	return s.db.WithContext(ctx).Exec(
		"UPDATE commercial_benefit_jobs SET state = ?, external_ref = CASE WHEN ? <> '' THEN ? ELSE external_ref END, updated_at = ? WHERE key = ?",
		state, externalRef, externalRef, time.Now().UTC(), key).Error
}

// RecordDowngrade stores the expiry projection (base tier) and over-limit
// reason. The conditional guard makes concurrent workers idempotent and the
// update purely additive: no member, job, or top-up row is ever removed.
func (s *SubscriptionStore) RecordDowngrade(ctx context.Context, id, projectionJSON, reason string) error {
	return s.db.WithContext(ctx).Exec(
		"UPDATE commercial_subscriptions SET projection_plan_json = ?, downgrade_reason = ? WHERE id = ? AND downgrade_reason = ''",
		projectionJSON, reason, id).Error
}
