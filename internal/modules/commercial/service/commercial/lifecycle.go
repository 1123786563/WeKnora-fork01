// Package commercial implements commercial billing lifecycle services on top
// of the domain types in internal/commercial and the commercial repository.
package commercial

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"
)

// IssuerKind selects the single authoritative benefit issuer, per the V03
// monthly-benefit contract: exactly one of the external provider or the local
// job queue issues a given monthly grant, never both.
type IssuerKind string

const (
	// IssuerLocal makes the claimed local job the authoritative grant; a
	// local worker processes pending jobs. This is the default.
	IssuerLocal IssuerKind = "local"
	// IssuerExternal makes the external provider the authoritative issuer;
	// the local row only records the claim and, on success, the batch ID.
	IssuerExternal IssuerKind = "external"
)

var ErrExternalIssuerMissing = errors.New("external_issuer_missing")

// Issuer is the explicit V03 issuer configuration.
type Issuer struct {
	Kind IssuerKind
	// GrantMonth issues one month's benefit through the external provider
	// and returns its batch reference. Required when Kind is IssuerExternal.
	GrantMonth func(ctx context.Context, sub domain.Subscription, monthStart time.Time) (externalRef string, err error)
}

// LifecycleService schedules idempotent monthly benefit grants and the expiry
// downgrade projection.
type LifecycleService struct {
	store  *repocommercial.SubscriptionStore
	issuer Issuer
}

// NewLifecycleService validates the issuer configuration. An empty kind
// defaults to the local issuer.
func NewLifecycleService(store *repocommercial.SubscriptionStore, issuer Issuer) (*LifecycleService, error) {
	if issuer.Kind == "" {
		issuer.Kind = IssuerLocal
	}
	if issuer.Kind == IssuerExternal && issuer.GrantMonth == nil {
		return nil, ErrExternalIssuerMissing
	}
	return &LifecycleService{store: store, issuer: issuer}, nil
}

// Tick brings every subscription up to date at now:
//
//   - Only months with month_start <= now < paid_until are scheduled; annual
//     terms are granted month by month and early renewals that extend
//     paid_until never pre-grant future months.
//   - Each month is claimed by its unique MonthlyGrantKey, so the same period
//     replayed by many workers yields exactly one job.
//   - With the external issuer, the batch ID is saved on success and the key
//     is never changed on unknown or failed states.
//   - An expired subscription gets the base-tier projection and over-limit
//     reason recorded; data, members, and unexpired top-ups are never deleted.
func (s *LifecycleService) Tick(ctx context.Context, now time.Time) error {
	rows, err := s.store.ListSubscriptions(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		// A scheduled plan switch whose effective time (the paid_until it
		// was cut against) has passed is applied FIRST — exactly once under
		// the stored-JSON guard — so the rest of the tick projects the
		// switched plan, not the stale one (design 6.2: 降级在已付费覆盖
		// 区间结束后切换).
		applied, err := s.applyDueScheduledChange(ctx, row, now)
		if err != nil {
			return err
		}
		if applied {
			fresh, err := s.store.Current(ctx, row.TenantID)
			if err != nil {
				return err
			}
			row = fresh
		}
		sub, err := toDomainSubscription(row)
		if err != nil {
			return err
		}
		projection := sub.ProjectionAt(now)
		if projection.Downgraded {
			if row.DowngradeReason != projection.Reason || row.ProjectionPlanJSON == "" {
				if err := s.recordExpiry(ctx, row.ID, projection); err != nil {
					return err
				}
			}
			continue
		}
		if err := s.issueDueMonths(ctx, sub, now); err != nil {
			return err
		}
	}
	return nil
}

// applyDueScheduledChange applies a due scheduled plan switch through the
// store's exactly-once guard. The arrangement carries its own immutable
// plan snapshot, so applying never re-resolves the catalog at effective
// time; a malformed or absent record is a clean no-op.
func (s *LifecycleService) applyDueScheduledChange(ctx context.Context, row repocommercial.Subscription, now time.Time) (bool, error) {
	change, ok := domain.ParseScheduledPlanChange(row.ScheduledChangeJSON)
	if !ok || !change.Due(now) {
		return false, nil
	}
	return s.store.ApplyDueScheduledChange(ctx, row, now)
}

func (s *LifecycleService) issueDueMonths(ctx context.Context, sub domain.Subscription, now time.Time) error {
	for _, monthStart := range sub.DueMonths(now) {
		key := domain.MonthlyGrantKey(sub.ID, monthStart)
		claimed, err := s.store.ClaimBenefitJob(ctx, repocommercial.BenefitJob{
			Key:            key,
			TenantID:       sub.TenantID,
			SubscriptionID: sub.ID,
			MonthStart:     monthStart,
			Credits:        int64(sub.Plan.Monthly),
			State:          repocommercial.JobStatePending,
		})
		if err != nil {
			return err
		}
		if !claimed {
			// Another worker already owns this exact period; never re-key.
			continue
		}
		if s.issuer.Kind != IssuerExternal {
			// Local issuer: the claimed pending job is the authoritative
			// grant record for a local worker to fulfill.
			continue
		}
		externalRef, err := s.issuer.GrantMonth(ctx, sub, monthStart)
		if err != nil {
			if cerr := s.store.CompleteBenefitJob(ctx, key, repocommercial.JobStateFailed, ""); cerr != nil {
				return cerr
			}
			return err
		}
		if err := s.store.CompleteBenefitJob(ctx, key, repocommercial.JobStateGranted, externalRef); err != nil {
			return err
		}
	}
	return nil
}

func (s *LifecycleService) recordExpiry(ctx context.Context, subscriptionID string, projection domain.Projection) error {
	blob, err := json.Marshal(projection.Plan)
	if err != nil {
		return err
	}
	return s.store.RecordDowngrade(ctx, subscriptionID, string(blob), projection.Reason)
}

func toDomainSubscription(row repocommercial.Subscription) (domain.Subscription, error) {
	var plan domain.PlanVersion
	if err := json.Unmarshal([]byte(row.PlanSnapshotJSON), &plan); err != nil {
		return domain.Subscription{}, err
	}
	return domain.Subscription{
		ID:        row.ID,
		TenantID:  row.TenantID,
		Plan:      plan,
		Anchor:    row.Anchor,
		PaidUntil: row.PaidUntil,
		Version:   row.Version,
	}, nil
}
