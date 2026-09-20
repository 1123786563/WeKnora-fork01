package commercial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"

	"gorm.io/gorm"
)

// Base Plan seed constants (#80, Lago T08). The built-in (base, v1)
// definition is a FIRST-SLICE constant — externalizing the values is a
// later product ticket. IncludedCreditsMicro is cent-aligned so the seam's
// exact decimal-string conversion never rounds.
const (
	BasePlanSeedVersion              int64 = 1
	BasePlanSeedIncludedCreditsMicro int64 = 1_000_000 // 1 credit/month, cent-aligned
	BasePlanSeedActor                      = "system:base-plan-seed"
	BasePlanSeedName                       = "Base Plan"
)

// BasePlanSeedFeatures is the closed feature set of the free tier.
func BasePlanSeedFeatures() map[string]bool {
	return map[string]bool{
		"api_access":       true,
		"advanced_models":  false,
		"priority_support": false,
	}
}

// BasePlanSeedLimits is the resource-quota set of the free tier (members in
// persons, storage in GB, concurrent tasks in tasks — the enforcement
// units are decided in ApplyQuotas).
func BasePlanSeedLimits() map[string]int64 {
	return map[string]int64{
		"members":          5,
		"storage_gb":       10,
		"concurrent_tasks": 2,
	}
}

// PlanView is the closed product plan answer of the Billing API. Key and
// Version address the LOCAL plan vocabulary; no external plan code, no
// provider identity ever appears.
type PlanView struct {
	Key      string
	Version  int64
	State    string // active|pending (closed set)
	Features map[string]bool
	Limits   map[string]int64
}

// BatchView is one monthly credit batch in the Billing API answer; an
// expired batch reports zero (the registry expiry overlay).
type BatchView struct {
	Period       string
	BalanceMicro int64
	ExpiresAt    time.Time
}

// CreditsView is the included-credit answer: the expiry-overlaid balance
// and the per-batch breakdown.
type CreditsView struct {
	BalanceMicro int64
	Batches      []BatchView
}

// BenefitsStatus is the closed product answer served by the Billing API:
// the #78 account envelope, the plan view (nil while pending — never a
// fabricated plan), the credits view, and a closed reason token
// (""|unconfigured|unreachable|invalid_response|unsupported).
type BenefitsStatus struct {
	Account BillingAccountStatus
	Plan    *PlanView
	Credits *CreditsView
	Reason  string
}

// BenefitsService owns the T08 lazy chain, run on billing access (the #78
// documented trigger; no startup dependency, no background clock — credits
// become spendable only with #87/#88 admission, which can add the clock):
//
//	EnsureBenefits
//	  ├─ SeedBasePlan         (idempotent #79 publish of (base, v1); no-op once published)
//	  ├─ EnsureBillingAccount (#78 — the SAME customer ensure; linked row answers fast)
//	  ├─ ensure_subscription  (seam; idempotent by ExternalSubscriptionID)
//	  ├─ EnsureMonthlyCredits (registry-gated grant for the current UTC period)
//	  └─ RefreshProjection    (benefits snapshot → plan identity via publications → limits/features → counters)
//
// Failure posture (the #78 doctrine): every platform failure is a STATE
// (pending + closed reason), never a caller error; the chain is resumable
// at every step (linked account row / published seed / subscription
// identity / registry row are all idempotent checkpoints). Only DB errors
// return errors.
type BenefitsService struct {
	accounts *BillingAccountService
	plans    *PlanVersionService
	store    *repocommercial.BenefitsStore
	platform domain.CommercialPlatform // nil legal: projection stays pending, nothing fabricated
	now      func() time.Time          // injectable clock
	// seedMu makes concurrent first-access seeding exactly-once in-process:
	// without it, racing seeds would each allocate the next (base, N)
	// version and fork parallel publications of one product plan.
	seedMu sync.Mutex
	// grantMus serializes the monthly grant PER TENANT (the seedMu pattern;
	// single-process deployment model). The registry's unique (tenant,
	// period) row alone cannot stop two concurrent FIRST grants: both
	// callers would observe WalletRef == "" — the row exists but the grant
	// has not completed — and both would POST to an authority with NO
	// wallet idempotency (E3: a replayed grant POST doubles the balance).
	// The per-tenant mutex closes that window in-process; the row's
	// WalletRef is the completed marker the second caller then reads.
	grantMuGuard sync.Mutex
	grantMus     map[uint64]*sync.Mutex
}

// NewBenefitsService builds the service and bootstraps its schema (portable
// EnsureSchema — safe next to migrations 000180/000101). A nil platform is
// legal (blocked-env: the chain fails closed as pending/unconfigured).
func NewBenefitsService(db *gorm.DB, accounts *BillingAccountService, plans *PlanVersionService, platform domain.CommercialPlatform) (*BenefitsService, error) {
	if db == nil {
		return nil, errors.New("benefits service requires a database")
	}
	store := repocommercial.NewBenefitsStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		return nil, err
	}
	return &BenefitsService{
		accounts: accounts,
		plans:    plans,
		store:    store,
		platform: platform,
		now:      func() time.Time { return time.Now().UTC() },
	}, nil
}

// SetNow injects the clock (deterministic period/expiry tests).
func (s *BenefitsService) SetNow(fn func() time.Time) {
	if fn == nil {
		return
	}
	s.now = fn
}

// SeedBasePlan idempotently publishes the built-in free tier through the
// #79 flow: a no-op once (base, v1) carries a publication (an
// operator-published variant definition is respected — publish-once
// immutability); otherwise the (base,1) draft is created (or the existing
// draft reused) and published with actor system:base-plan-seed. Boot NEVER
// blocks on this — it runs inside the lazy ensure chain.
func (s *BenefitsService) SeedBasePlan(ctx context.Context) (bool, error) {
	if s == nil || s.plans == nil {
		return false, nil // nothing to seed without the plan service (pending posture)
	}
	s.seedMu.Lock()
	defer s.seedMu.Unlock()
	if _, err := s.plans.GetPublication(ctx, domain.BasePlanKey, BasePlanSeedVersion); err == nil {
		return false, nil // published already — respect it, never re-publish
	} else if !errors.Is(err, repocommercial.ErrPublicationNotFound) {
		return false, err
	}
	view, err := s.plans.GetVersion(ctx, domain.BasePlanKey, BasePlanSeedVersion)
	if err != nil {
		if !errors.Is(err, repocommercial.ErrPlanNotFound) {
			return false, err
		}
		// Fresh key: allocate (base, v1) as a draft (CreateDraft allocates
		// max+1 = 1 on a fresh key).
		view, err = s.plans.CreateDraft(ctx, BasePlanSeedActor, DraftInput{
			PlanKey:              domain.BasePlanKey,
			Name:                 BasePlanSeedName,
			AmountFen:            0,
			IncludedCreditsMicro: BasePlanSeedIncludedCreditsMicro,
			Features:             BasePlanSeedFeatures(),
			Limits:               BasePlanSeedLimits(),
			Currency:             domain.CurrencyCNY,
		})
		if err != nil {
			return false, err
		}
	}
	if _, err := s.plans.Publish(ctx, BasePlanSeedActor, "base_plan_seed", domain.BasePlanKey, view.Version); err != nil {
		return false, err
	}
	return true, nil
}

// isPlatformFailure reports whether err is one of the seam's closed
// sentinels — a STATE upstream — as opposed to a local DB failure, which
// must return as an error.
func isPlatformFailure(err error) bool {
	return errors.Is(err, domain.ErrPlatformUnsupported) ||
		errors.Is(err, domain.ErrPlatformUnconfigured) ||
		errors.Is(err, domain.ErrPlatformUnreachable) ||
		errors.Is(err, domain.ErrPlatformInvalidResponse)
}

// EnsureBenefits runs the lazy chain for the tenant on billing access. The
// answer is the closed BenefitsStatus; platform failures are states, only
// DB errors return errors. tenantID comes exclusively from the caller's
// authenticated scope.
func (s *BenefitsService) EnsureBenefits(ctx context.Context, tenantID uint64, displayName, actor string) (BenefitsStatus, error) {
	if s == nil {
		return BenefitsStatus{
			Account: BillingAccountStatus{State: BillingAccountPending, Reason: "unconfigured"},
		}, nil
	}
	if tenantID == 0 {
		return BenefitsStatus{}, errors.New("benefits ensure requires an authenticated tenant scope")
	}
	pending := func(account BillingAccountStatus, reason string) BenefitsStatus {
		return BenefitsStatus{Account: account, Reason: reason}
	}

	// 1. Seed the Base Plan (idempotent; platform failures surface as the
	// closed reason, drafts persist for resume).
	if s.platform == nil || s.accounts == nil || s.plans == nil {
		// Fail closed honestly: no seam wired means no authority answer.
		account := BillingAccountStatus{State: BillingAccountPending, Reason: "unconfigured"}
		if s.accounts != nil {
			if got, err := s.accounts.EnsureBillingAccount(ctx, tenantID, displayName, actor); err == nil {
				account = got
			}
		}
		return pending(account, "unconfigured"), nil
	}
	if _, err := s.SeedBasePlan(ctx); err != nil {
		if isPlatformFailure(err) {
			return pending(BillingAccountStatus{State: BillingAccountPending}, platformReason(err)), nil
		}
		return BenefitsStatus{}, err
	}

	// 2. The SAME #78 customer ensure (a linked row answers fast, no submit).
	account, err := s.accounts.EnsureBillingAccount(ctx, tenantID, displayName, actor)
	if err != nil {
		return BenefitsStatus{}, err // DB error only
	}
	if account.State != BillingAccountLinked {
		return pending(account, account.Reason), nil
	}

	// 3. The subscription: idempotent by ExternalSubscriptionID.
	publication, err := s.plans.GetPublication(ctx, domain.BasePlanKey, BasePlanSeedVersion)
	if err != nil {
		return BenefitsStatus{}, err
	}
	if _, err := s.platform.SubmitCommand(ctx, domain.Command{
		Kind:   domain.CommandKindEnsureSubscription,
		Key:    domain.EnsureSubscriptionCommandKey(domain.ExternalSubscriptionID(tenantID)),
		Actor:  actor,
		Reason: BillingAccountEnsureReason,
		Payload: domain.EnsureSubscriptionPayload{
			TenantID:               tenantID,
			ExternalCustomerID:     domain.ExternalCustomerID(tenantID),
			ExternalSubscriptionID: domain.ExternalSubscriptionID(tenantID),
			PlanCode:               publication.PlanCode,
		},
	}); err != nil {
		if isPlatformFailure(err) {
			return pending(account, platformReason(err)), nil
		}
		return BenefitsStatus{}, err
	}

	// 4. Monthly included credits: registry-gated grant for the current UTC
	// period (coordinator-owned issuance — the T03 verdict).
	if _, err := s.EnsureMonthlyCredits(ctx, tenantID); err != nil {
		if isPlatformFailure(err) {
			return pending(account, platformReason(err)), nil
		}
		return BenefitsStatus{}, err
	}

	// 5. Projection refresh: benefits snapshot → local plan identity →
	// limits/features → quota counters.
	row, batches, reason, err := s.refreshAndCollect(ctx, tenantID)
	if err != nil {
		if isPlatformFailure(err) {
			return pending(account, platformReason(err)), nil
		}
		return BenefitsStatus{}, err
	}
	status := BenefitsStatus{Account: account, Reason: reason}
	if row.PlanKey != "" {
		features, _ := decodeJSONBoolMap(row.FeaturesJSON)
		limits, _ := decodeJSONIntMap(row.LimitsJSON)
		status.Plan = &PlanView{
			Key:      row.PlanKey,
			Version:  row.PlanVersion,
			State:    row.SubscriptionState,
			Features: features,
			Limits:   limits,
		}
	}
	if len(batches) > 0 {
		status.Credits = &CreditsView{Batches: batches}
		for _, b := range batches {
			status.Credits.BalanceMicro += b.BalanceMicro
		}
	}
	return status, nil
}

// tenantGrantMu returns the per-tenant grant serialization mutex (lazily
// created; bounded by the active tenant count).
func (s *BenefitsService) tenantGrantMu(tenantID uint64) *sync.Mutex {
	s.grantMuGuard.Lock()
	defer s.grantMuGuard.Unlock()
	if s.grantMus == nil {
		s.grantMus = map[uint64]*sync.Mutex{}
	}
	mu, ok := s.grantMus[tenantID]
	if !ok {
		mu = &sync.Mutex{}
		s.grantMus[tenantID] = mu
	}
	return mu
}

// EnsureMonthlyCredits issues the current period's included credits through
// the three-layer idempotency: the registry's unique (tenant, period) row
// is the gate; the seam command's deterministic Key
// (grant_included_credits:<ext>:<period>) plus the adapter's
// read-before-create make every replay an identity resolve, never a blind
// second grant (E3). The grant path is serialized PER TENANT: a batch row
// with WalletRef == "" means an earlier grant has not completed yet, and a
// concurrent first-ensure racing through that window would issue a second
// POST to an authority with no wallet idempotency (E3) — exactly one
// caller proceeds (the seedMu in-process pattern; single-process
// deployment model). A batch row whose grant never completed (refused or
// lost response) re-submits safely on the next ensure — the adapter finds
// the existing wallet by deterministic name and replays.
func (s *BenefitsService) EnsureMonthlyCredits(ctx context.Context, tenantID uint64) (repocommercial.CreditBatchRow, error) {
	if s.platform == nil {
		return repocommercial.CreditBatchRow{}, domain.ErrPlatformUnconfigured
	}
	mu := s.tenantGrantMu(tenantID)
	mu.Lock()
	defer mu.Unlock()
	now := s.now().UTC()
	period := domain.MonthlyPeriod(now)
	end, err := domain.PeriodEnd(period)
	if err != nil {
		return repocommercial.CreditBatchRow{}, err
	}
	extCustomer := domain.ExternalCustomerID(tenantID)
	row, created, err := s.store.EnsureBatch(ctx, repocommercial.CreditBatchRow{
		TenantID:     tenantID,
		Period:       period,
		CommandKey:   domain.GrantCreditsCommandKey(extCustomer, period),
		GrantedMicro: BasePlanSeedIncludedCreditsMicro,
		ExpiresAt:    end,
		State:        repocommercial.CreditBatchStateGranted,
		CreatedAt:    now,
	})
	if err != nil {
		return row, err
	}
	if !created && row.WalletRef != "" {
		return row, nil // a completed batch replays from the registry alone
	}
	receipt, err := s.platform.SubmitCommand(ctx, domain.Command{
		Kind:   domain.CommandKindGrantIncludedCredits,
		Key:    domain.GrantCreditsCommandKey(extCustomer, period),
		Actor:  BasePlanSeedActor,
		Reason: "monthly_included_credits",
		Payload: domain.GrantIncludedCreditsPayload{
			TenantID:           tenantID,
			ExternalCustomerID: extCustomer,
			Period:             period,
			CreditsMicro:       BasePlanSeedIncludedCreditsMicro,
			ExpiresAt:          end,
		},
	})
	if err != nil {
		// Indeterminate or refused: the registry row stays as the resumable
		// checkpoint; the adapter's read-before-create makes the next submit
		// an identity resolve, never a doubled grant.
		return row, err
	}
	if err := s.store.MarkBatchWallet(ctx, row.ID, receipt.ExternalID); err != nil {
		return row, err
	}
	return s.store.GetBatch(ctx, tenantID, period)
}

// RefreshProjection reads the authority's benefits snapshot, resolves the
// plan identity through commercial_plan_publications, writes the projection
// row and applies the quota limits onto the counters.
func (s *BenefitsService) RefreshProjection(ctx context.Context, tenantID uint64) (repocommercial.BenefitsRow, error) {
	row, _, _, err := s.refreshAndCollect(ctx, tenantID)
	return row, err
}

// refreshAndCollect is RefreshProjection with the expiry-overlaid credit
// batches and the closed reason token for an unresolvable plan code.
func (s *BenefitsService) refreshAndCollect(ctx context.Context, tenantID uint64) (repocommercial.BenefitsRow, []BatchView, string, error) {
	if s.platform == nil {
		return repocommercial.BenefitsRow{}, nil, "unconfigured", domain.ErrPlatformUnconfigured
	}
	snap, err := s.platform.ReadSnapshot(ctx, domain.SnapshotQuery{
		Kind: domain.SnapshotKindBenefits, TenantID: tenantID,
	})
	if err != nil {
		return repocommercial.BenefitsRow{}, nil, platformReason(err), err
	}
	b := snap.Benefits
	now := s.now().UTC()
	row := repocommercial.BenefitsRow{
		TenantID:               tenantID,
		ExternalCustomerID:     domain.ExternalCustomerID(tenantID),
		ExternalSubscriptionID: domain.ExternalSubscriptionID(tenantID),
		SubscriptionState:      b.SubscriptionState,
		PlanCode:               b.PlanCode,
		CreditsBalanceMicro:    b.BalanceMicro, // RAW authority balance; the overlay lives in the view
		ProjectedAt:            now,
	}
	reason := ""
	if b.PlanCode != "" {
		pub, err := s.plans.FindPublicationByCode(ctx, b.PlanCode)
		switch {
		case err == nil:
			row.PlanKey, row.PlanVersion = pub.PlanKey, pub.Version
		case errors.Is(err, repocommercial.ErrPublicationNotFound):
			// An authority plan we never published: closed pending reason;
			// the projection keeps its last-known plan.
			reason = "invalid_response"
		default:
			return repocommercial.BenefitsRow{}, nil, "", err
		}
	}
	if existing, err := s.store.GetProjection(ctx, tenantID); err == nil && row.PlanKey == "" {
		row.PlanKey, row.PlanVersion = existing.PlanKey, existing.PlanVersion
	} else if !errors.Is(err, repocommercial.ErrProjectionNotFound) && err != nil {
		return repocommercial.BenefitsRow{}, nil, "", err
	}

	// Features and limits come from the LOCAL definition (the authority's
	// entitlement surface may lag or differ — the publication's definition
	// is the product truth); the snapshot's feature map wins when present.
	if row.PlanKey != "" {
		definition, defErr := s.definitionOf(ctx, row.PlanKey, row.PlanVersion)
		switch {
		case defErr == nil:
			row.LimitsJSON = encodeJSONIntMap(definition.Limits)
			if len(b.Features) > 0 {
				row.FeaturesJSON = encodeJSONBoolMap(b.Features)
			} else {
				row.FeaturesJSON = encodeJSONBoolMap(definition.Features)
			}
		default:
			if len(b.Features) > 0 {
				row.FeaturesJSON = encodeJSONBoolMap(b.Features)
			}
		}
	}
	if row.FeaturesJSON == "" {
		row.FeaturesJSON = "{}"
	}
	if row.LimitsJSON == "" {
		row.LimitsJSON = "{}"
	}
	if err := s.store.UpsertProjection(ctx, row); err != nil {
		return repocommercial.BenefitsRow{}, nil, "", err
	}
	var limits map[string]int64
	if row.PlanKey != "" {
		if definition, defErr := s.definitionOf(ctx, row.PlanKey, row.PlanVersion); defErr == nil {
			limits = definition.Limits
		}
	}
	if err := s.ApplyQuotas(ctx, tenantID, limits); err != nil {
		// ADVISORY by design: quota application is a fail-open surface — a
		// missing occupancy table or a transient read failure must never
		// turn a billing read into an error (and cannot lock a space out:
		// no limits were written, every reserve passes). The next refresh
		// re-applies.
		_ = err
	}

	// The expiry overlay: registry truth decides spendability. An expired
	// batch reports ZERO even while the authority still lists the wallet
	// active (the lazy-termination window — E1).
	all, err := s.store.ListBatches(ctx, tenantID)
	if err != nil {
		return repocommercial.BenefitsRow{}, nil, "", err
	}
	balances := make(map[string]int64, len(b.Batches))
	for _, batch := range b.Batches {
		balances[batch.Period] = batch.BalanceMicro
	}
	batches := make([]BatchView, 0, len(all))
	for _, a := range all {
		balance := int64(0)
		if a.ExpiresAt.After(now) {
			balance = balances[a.Period] // absent from the snapshot (terminated) = 0
		}
		batches = append(batches, BatchView{Period: a.Period, BalanceMicro: balance, ExpiresAt: a.ExpiresAt})
	}
	return row, batches, reason, nil
}

// definitionOf decodes the plan version's stored definition (the product
// truth behind the projection).
func (s *BenefitsService) definitionOf(ctx context.Context, planKey string, version int64) (domain.PlanVersion, error) {
	view, err := s.plans.GetVersion(ctx, planKey, version)
	if err != nil {
		return domain.PlanVersion{}, err
	}
	return decodeDefinition(view.DefinitionJSON)
}

// ApplyQuotas re-syncs the counters' used from OBSERVED occupancy (member
// count, tenants.storage_used) and THEN applies the projected hard limits.
// Ordering is binding: occupancy first, limits second — 超限状态 must
// reflect real overage, never counter drift (limits cannot permanently
// over-block a drifted counter). Units: members = persons; storage_gb
// counter tracks BYTES with hard_limit = G × 2^30; concurrent_tasks =
// tasks (exposed only — admission enforcement lands with #87/#88, an
// explicit documented non-goal here).
func (s *BenefitsService) ApplyQuotas(ctx context.Context, tenantID uint64, limits map[string]int64) error {
	members, storageBytes, err := s.store.ObserveOccupancy(ctx, tenantID)
	if err != nil {
		return err
	}
	limitPtr := func(key string) *int64 {
		if v, ok := limits[key]; ok {
			return &v
		}
		return nil // absent dimension = unlimited (fail-open degraded posture)
	}
	if err := s.store.SyncCounter(ctx, tenantID, "members", members, limitPtr("members")); err != nil {
		return err
	}
	storageLimit := limitPtr("storage_gb")
	if storageLimit != nil {
		bytes := *storageLimit * (1 << 30)
		storageLimit = &bytes
	}
	if err := s.store.SyncCounter(ctx, tenantID, "storage_gb", storageBytes, storageLimit); err != nil {
		return err
	}
	// No task admission exists yet (#87/#88): the observed occupancy of
	// concurrent tasks is zero by construction.
	return s.store.SyncCounter(ctx, tenantID, "concurrent_tasks", 0, limitPtr("concurrent_tasks"))
}

// ---- JSON helpers (closed, used by the projection row and the API view) ----

func encodeJSONBoolMap(m map[string]bool) string {
	if len(m) == 0 {
		return "{}"
	}
	blob, _ := json.Marshal(m)
	return string(blob)
}

func encodeJSONIntMap(m map[string]int64) string {
	if len(m) == 0 {
		return "{}"
	}
	blob, _ := json.Marshal(m)
	return string(blob)
}

func decodeJSONBoolMap(s string) (map[string]bool, error) {
	var m map[string]bool
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, fmt.Errorf("invalid features json: %w", err)
	}
	return m, nil
}

func decodeJSONIntMap(s string) (map[string]int64, error) {
	var m map[string]int64
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, fmt.Errorf("invalid limits json: %w", err)
	}
	return m, nil
}
