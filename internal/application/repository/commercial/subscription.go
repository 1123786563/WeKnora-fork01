package commercial

import (
	"context"
	"time"

	"gorm.io/gorm"
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
	Version            int64     `gorm:"column:version;not null;default:1"`
	ProjectionPlanJSON string    `gorm:"column:projection_plan_json;not null;default:''"`
	DowngradeReason    string    `gorm:"column:downgrade_reason;not null;default:''"`
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
		 future_interval_json, version, projection_plan_json, downgrade_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			tenant_id = excluded.tenant_id,
			plan_key = excluded.plan_key,
			plan_version = excluded.plan_version,
			plan_snapshot_json = excluded.plan_snapshot_json,
			anchor = excluded.anchor,
			paid_until = excluded.paid_until,
			future_interval_json = excluded.future_interval_json,
			version = excluded.version,
			projection_plan_json = excluded.projection_plan_json,
			downgrade_reason = excluded.downgrade_reason`,
		sub.ID, sub.TenantID, sub.PlanKey, sub.PlanVersion, sub.PlanSnapshotJSON,
		sub.Anchor, sub.PaidUntil, sub.FutureIntervalJSON, sub.Version,
		sub.ProjectionPlanJSON, sub.DowngradeReason).Error
}

// ListSubscriptions returns every subscription row for a tick.
func (s *SubscriptionStore) ListSubscriptions(ctx context.Context) ([]Subscription, error) {
	var subs []Subscription
	err := s.db.WithContext(ctx).Order("id").Find(&subs).Error
	return subs, err
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
