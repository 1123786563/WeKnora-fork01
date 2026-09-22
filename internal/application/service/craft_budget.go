package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"
	"github.com/Tencent/WeKnora/internal/modules/craft"

	"gorm.io/gorm"
)

// Craft budget admission: the adapter from the craft-internal craft.BudgetPort
// to the approved G4 commercial interfaces. Red line kept deliberately: this
// file owns NO money semantics — every Credits number (per-call hold,
// task limit) arrives from deployment configuration, the conversion between
// money and Credits stays with the commercial implementation, and the craft
// grant model carries call counts and deadlines only.
//
// G4 mapping (verified against the real interfaces, see task report):
//
//	Admit          -> BudgetStore.EnsureTaskBudget (registers the run's task
//	                  budget; funds are verified per reservation, not here)
//	AuthorizeCall  -> BudgetStore.Reserve + MarkReservationDispatched, the
//	                  same single-transaction CAS discipline
//	                  ExecutionGateService.Begin applies for every commercial
//	                  dispatch: atomic across workers and processes, one
//	                  winner for the last quota
//	Cancel/Revoke  -> durable grant flag only; holds of admitted calls are
//	                  NEVER zeroed here
//	Reconcile      -> commercialsvc.BudgetService.Reconcile per reservation:
//	                  unstarted holds release, unconfirmed outcomes retain
//	                  their protection
//	Extend         -> BudgetStore.ExtendTaskLimit (exactly-once per key)
//	Settlement     -> NOT here: O01's usage outbox feeds the G4 OpenMeter
//	                  worker; craft never prices or charges.

var (
	// ErrCraftBudgetDatabaseMissing rejects wiring without durable storage.
	ErrCraftBudgetDatabaseMissing = errors.New("craft_budget_database_missing")
	// ErrCraftBudgetPolicyInvalid rejects an admission policy without
	// explicitly configured limits — an unbounded default must never exist.
	ErrCraftBudgetPolicyInvalid = errors.New("craft_budget_policy_invalid")
	// ErrCraftGrantNotFound rejects an unknown grant identity.
	ErrCraftGrantNotFound = errors.New("craft_grant_not_found")
	// ErrCraftCallAlreadySettled rejects re-authorizing a logical call whose
	// reservation already settled: re-forwarding it would re-bill it.
	ErrCraftCallAlreadySettled = errors.New("craft_call_already_settled")
)

// craftCallKeyPrefix namespaces craft call reservations inside the commercial
// reservation key space so a craft hold is always recognizable as such.
const craftCallKeyPrefix = "craft-call/"

// CraftCallKey is the commercial reservation key of one logical craft call.
func CraftCallKey(callID string) string { return craftCallKeyPrefix + callID }

// CraftBudgetPolicy is the deployment-owned admission configuration. Every
// field is mandatory: the adapter refuses to invent defaults for money or
// call bounds, so a misconfigured deployment fails closed at construction
// instead of admitting unbounded runs.
type CraftBudgetPolicy struct {
	// GrantWindow bounds how long an admitted run may keep authorizing calls.
	GrantWindow time.Duration
	// MaxCalls caps the number of authorized logical calls of one run.
	MaxCalls int
	// CallUpper is the per-call commercial hold taken before every forward.
	CallUpper commercial.Credits
	// TaskLimit is the commercial task budget limit registered at admission.
	TaskLimit commercial.Credits
}

func (p CraftBudgetPolicy) validate() error {
	if p.GrantWindow <= 0 || p.MaxCalls <= 0 || p.CallUpper <= 0 || p.TaskLimit <= 0 {
		return fmt.Errorf("%w: grant window, max calls, call upper and task limit must all be positive", ErrCraftBudgetPolicyInvalid)
	}
	return nil
}

// CraftBudgetGrantRow is the durable admission verdict of one run
// (migration 000124_craft_budget / 000044 sqlite). No money columns: funds
// and holds live exclusively in the commercial tables.
type CraftBudgetGrantRow struct {
	TenantID  uint64    `gorm:"column:tenant_id;primaryKey;autoIncrement:false"`
	RunID     string    `gorm:"column:run_id;primaryKey"`
	GrantID   string    `gorm:"column:grant_id;not null;uniqueIndex:uq_craft_budget_grant_id"`
	Deadline  time.Time `gorm:"column:deadline;not null"`
	MaxCalls  int       `gorm:"column:max_calls;not null"`
	Allowed   bool      `gorm:"column:allowed;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

func (CraftBudgetGrantRow) TableName() string { return "craft_budget_grants" }

// CraftBudgetCallRow is the durable authorization ledger of ONE logical call.
// CallKey is simultaneously the commercial reservation key, so exactly one row
// here pairs with exactly one commercial hold. The (tenant, run, delegation,
// model, funding, call_seq) uniqueness keeps the per-binding call sequence
// strictly monotonic across restarts — the gateway derives call identities
// from this table and never restarts them at zero after recovery.
type CraftBudgetCallRow struct {
	TenantID     uint64    `gorm:"column:tenant_id;primaryKey;autoIncrement:false"`
	CallKey      string    `gorm:"column:call_key;primaryKey"`
	GrantID      string    `gorm:"column:grant_id;not null;index:idx_craft_budget_calls_grant,priority:2"`
	RunID        string    `gorm:"column:run_id;not null;default:''"`
	DelegationID string    `gorm:"column:delegation_id;not null;default:''"`
	ModelID      string    `gorm:"column:model_id;not null"`
	Funding      string    `gorm:"column:funding;not null"`
	CallSeq      int64     `gorm:"column:call_seq;not null"`
	CallID       string    `gorm:"column:call_id;not null;uniqueIndex:uq_craft_budget_calls_call_id"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
}

func (CraftBudgetCallRow) TableName() string { return "craft_budget_calls" }

// CraftCallBinding names the identity facets a logical call binds beyond the
// grant: the (possibly empty) delegation, the model and the server-resolved
// funding source. Any change of any facet is a different logical call with
// its own sequence — the O01 DeriveCallID contract.
type CraftCallBinding struct {
	DelegationID string
	ModelID      string
	Funding      string
}

func (b CraftCallBinding) validate() error {
	if b.ModelID == "" {
		return fmt.Errorf("%w: call binding without model", craft.ErrInvalidInput)
	}
	if err := commercial.ValidateFunding(b.Funding); err != nil {
		return fmt.Errorf("%w: call binding funding %q is not server-recognized", craft.ErrInvalidInput, b.Funding)
	}
	return nil
}

// craftCallSeqAttempts bounds the re-read rounds of the per-binding sequence
// allocation. Cross-process safety comes from the unique sequence index, not
// from a process mutex.
const craftCallSeqAttempts = 16

// CraftBudgetService implements craft.BudgetPort on the real G4 interfaces.
// It is the ONLY assembly-approved budget port of the craft model gateway;
// test fakes must never be wired into a charging deployment.
type CraftBudgetService struct {
	db        *gorm.DB
	budget    *repocommercial.BudgetStore
	reconcile *commercialsvc.BudgetService
	policy    CraftBudgetPolicy
	now       func() time.Time
}

// NewCraftBudgetService validates its wiring and policy, then assembles over
// the G4 budget store and the U04 reconcile service. A nil entitlement check
// follows the commercial service's own wiring convention ("allow") — the
// entitlement hook is the commercial assembly's responsibility and its absence
// is a disclosed gap there, not a fabricated gate here.
func NewCraftBudgetService(db *gorm.DB, store *repocommercial.BudgetStore, policy CraftBudgetPolicy) (*CraftBudgetService, error) {
	if db == nil {
		return nil, ErrCraftBudgetDatabaseMissing
	}
	if err := policy.validate(); err != nil {
		return nil, err
	}
	if store == nil {
		store = repocommercial.NewBudgetStore(db)
	}
	reconcile, err := commercialsvc.NewBudgetService(db, store, nil)
	if err != nil {
		return nil, err
	}
	return &CraftBudgetService{
		db:        db,
		budget:    store,
		reconcile: reconcile,
		policy:    policy,
		now:       func() time.Time { return time.Now().UTC() },
	}, nil
}

var _ craft.BudgetPort = (*CraftBudgetService)(nil)

func newCraftGrantID() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "grant_" + hex.EncodeToString(buf), nil
}

// loadGrant resolves a grant identity to its durable row.
func (s *CraftBudgetService) loadGrant(ctx context.Context, grantID string) (CraftBudgetGrantRow, error) {
	if grantID == "" {
		return CraftBudgetGrantRow{}, fmt.Errorf("%w: empty grant id", craft.ErrInvalidInput)
	}
	var row CraftBudgetGrantRow
	err := s.db.WithContext(ctx).Where("grant_id = ?", grantID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CraftBudgetGrantRow{}, fmt.Errorf("%w: grant %s", ErrCraftGrantNotFound, grantID)
	}
	if err != nil {
		return CraftBudgetGrantRow{}, err
	}
	return row, nil
}

// snapshot projects the durable row (plus the live call count) to the port
// grant shape.
func (s *CraftBudgetService) snapshot(ctx context.Context, row CraftBudgetGrantRow) craft.BudgetGrant {
	var used int64
	_ = s.db.WithContext(ctx).Model(&CraftBudgetCallRow{}).
		Where("tenant_id = ? AND grant_id = ?", row.TenantID, row.GrantID).Count(&used).Error
	return craft.BudgetGrant{
		ID:        row.GrantID,
		Deadline:  row.Deadline,
		MaxCalls:  row.MaxCalls,
		UsedCalls: int(used),
		Allowed:   row.Allowed,
	}
}

// Admit implements craft.BudgetPort: it durably admits one run of one scope
// and registers the run's commercial task budget (G4 EnsureTaskBudget).
// Admission is idempotent per (tenant, run): the first admission fixes the
// grant (deadline, call cap) and the task budget; later admissions return the
// same grant unchanged — a revoked grant stays revoked and an expired grant
// never regains validity. Funds themselves are NOT verified here: G4's
// contract verifies the funded account atomically on every reservation, which
// is the strict guarantee (a healthy admission with an unfunded account
// therefore blocks on the FIRST call, not at admission).
func (s *CraftBudgetService) Admit(ctx context.Context, scope craft.Scope, runID string) (craft.BudgetGrant, error) {
	if scope.TenantID == 0 || runID == "" {
		return craft.BudgetGrant{}, fmt.Errorf("%w: admission needs a tenant scope and a run id", craft.ErrInvalidInput)
	}
	var row CraftBudgetGrantRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND run_id = ?", scope.TenantID, runID).First(&row).Error
	now := s.now()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		grantID, gerr := newCraftGrantID()
		if gerr != nil {
			return craft.BudgetGrant{}, gerr
		}
		row = CraftBudgetGrantRow{
			TenantID:  scope.TenantID,
			RunID:     runID,
			GrantID:   grantID,
			Deadline:  now.Add(s.policy.GrantWindow).UTC(),
			MaxCalls:  s.policy.MaxCalls,
			Allowed:   true,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
			return craft.BudgetGrant{}, err
		}
	} else if err != nil {
		return craft.BudgetGrant{}, err
	}
	// Register (idempotently) the commercial task budget of this run. A
	// pre-existing commercial row is authoritative and never mutated here.
	if err := s.budget.EnsureTaskBudget(ctx, row.TenantID, row.RunID, s.policy.TaskLimit, row.Deadline); err != nil {
		return s.snapshot(ctx, row), err
	}
	return s.snapshot(ctx, row), nil
}

// AuthorizeBinding allocates the NEXT logical call identity of one binding
// (durable per-binding sequence, O01 DeriveCallID) and authorizes it in one
// step: this is what the model gateway calls before every real forward.
// The identity is server-derived only — callers never supply a call id.
func (s *CraftBudgetService) AuthorizeBinding(ctx context.Context, grantID string, b CraftCallBinding) (string, error) {
	if err := b.validate(); err != nil {
		return "", err
	}
	row, err := s.loadGrant(ctx, grantID)
	if err != nil {
		return "", err
	}
	if ferr := s.fastRefuse(row); ferr != nil {
		return "", ferr
	}
	for attempt := 0; attempt < craftCallSeqAttempts; attempt++ {
		var seq int64
		if err := s.db.WithContext(ctx).Model(&CraftBudgetCallRow{}).
			Where("tenant_id = ? AND run_id = ? AND delegation_id = ? AND model_id = ? AND funding = ?",
				row.TenantID, row.RunID, b.DelegationID, b.ModelID, b.Funding).
			Select("COALESCE(MAX(call_seq),0)").Scan(&seq).Error; err != nil {
			return "", err
		}
		seq++
		callID := craft.DeriveCallID(row.TenantID, row.RunID, b.DelegationID, b.ModelID, b.Funding, seq)
		call := CraftBudgetCallRow{
			TenantID:     row.TenantID,
			CallKey:      CraftCallKey(callID),
			GrantID:      row.GrantID,
			RunID:        row.RunID,
			DelegationID: b.DelegationID,
			ModelID:      b.ModelID,
			Funding:      b.Funding,
			CallSeq:      seq,
			CallID:       callID,
			CreatedAt:    s.now(),
		}
		if err := s.db.WithContext(ctx).Create(&call).Error; err != nil {
			// A concurrent authorization took this sequence slot: re-read the
			// fresh maximum and retry with the next one.
			continue
		}
		if err := s.enforceCallCap(ctx, row, call); err != nil {
			return "", err
		}
		if err := s.authorizeReservedCall(ctx, row, call, true); err != nil {
			return "", err
		}
		return callID, nil
	}
	return "", fmt.Errorf("%w: call sequence contention on grant %s", craft.ErrConflict, grantID)
}

// AuthorizeCall implements craft.BudgetPort for a KNOWN logical call id (the
// retry/recovery path). It is fully idempotent per callID: whatever step of
// the original authorization crashed — ledger row, reservation or dispatch —
// the retry completes it instead of admitting a second time.
func (s *CraftBudgetService) AuthorizeCall(ctx context.Context, grantID, callID string) error {
	if callID == "" {
		return fmt.Errorf("%w: empty call id", craft.ErrInvalidInput)
	}
	row, err := s.loadGrant(ctx, grantID)
	if err != nil {
		return err
	}
	if ferr := s.fastRefuse(row); ferr != nil {
		return ferr
	}
	callKey := CraftCallKey(callID)
	var call CraftBudgetCallRow
	err = s.db.WithContext(ctx).Where("tenant_id = ? AND call_key = ?", row.TenantID, callKey).First(&call).Error
	fresh := false
	now := s.now()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// First authorization of this call id: an identity the gateway derived
		// itself (O01 DeriveCallID) — reconstructed from the known facets only
		// when it matches this grant's run, never trusted from the caller.
		fresh = true
		call = CraftBudgetCallRow{
			TenantID: row.TenantID, CallKey: callKey, GrantID: row.GrantID, RunID: row.RunID,
			CallID: callID, CallSeq: 0, CreatedAt: now,
		}
		if err := s.db.WithContext(ctx).Create(&call).Error; err != nil {
			return err
		}
		if err := s.enforceCallCap(ctx, row, call); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if call.GrantID != row.GrantID {
		return fmt.Errorf("%w: call %s belongs to another grant", craft.ErrForbidden, callID)
	}
	return s.authorizeReservedCall(ctx, row, call, fresh)
}

// reserveCall holds the commercial budget of one call WITHOUT dispatching it.
// It exists for callers that must separate "reserved" from "dispatched"
// (recovery tests, lease hand-off); production forwards use AuthorizeBinding.
func (s *CraftBudgetService) reserveCall(ctx context.Context, grantID, callID string) error {
	row, err := s.loadGrant(ctx, grantID)
	if err != nil {
		return err
	}
	if ferr := s.fastRefuse(row); ferr != nil {
		return ferr
	}
	var call CraftBudgetCallRow
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND call_key = ?",
		row.TenantID, CraftCallKey(callID)).First(&call).Error; err != nil {
		return err
	}
	_, err = s.budget.Reserve(ctx, commercial.BudgetRequest{
		TenantID: row.TenantID,
		RunID:    row.RunID,
		Key:      call.CallKey,
		Upper:    s.policy.CallUpper,
		Deadline: row.Deadline,
	})
	return err
}

// authorizeReservedCall performs the commercial reservation + dispatch of one
// ledger-recorded call and interprets every G4 outcome against the port
// semantics: idempotent replay for the same key, exactly one winner for the
// last quota, denial mapping and never a partial commit. fresh marks a call
// row this invocation just inserted: a commercial denial compensates it back
// out so a later top-up can retry the same logical call cleanly.
func (s *CraftBudgetService) authorizeReservedCall(ctx context.Context, row CraftBudgetGrantRow, call CraftBudgetCallRow, fresh bool) error {
	req := commercial.BudgetRequest{
		TenantID: row.TenantID,
		RunID:    row.RunID,
		Key:      call.CallKey,
		Upper:    s.policy.CallUpper,
		Deadline: row.Deadline,
	}
	_, err := s.budget.Reserve(ctx, req)
	if err != nil {
		if errors.Is(err, repocommercial.ErrReservationKeyConflict) {
			return s.resolveReplayedReservation(ctx, row, call)
		}
		if mapped := s.mapReserveDenial(err); mapped != nil {
			if fresh {
				if derr := s.compensateCall(ctx, row, call); derr != nil {
					return derr
				}
			}
			return mapped
		}
		return err
	}
	if err := s.budget.MarkReservationDispatched(ctx, row.TenantID, call.CallKey); err != nil {
		if errors.Is(err, repocommercial.ErrReservationNotHeld) {
			// A concurrent authorization of the SAME call id won the dispatch.
			return s.resolveReplayedReservation(ctx, row, call)
		}
		return err
	}
	return nil
}

// resolveReplayedReservation decides the idempotent outcome of a call whose
// reservation already exists: content-equal active holds are already
// authorized; a settled call must never re-forward; a released hold means the
// authorization was withdrawn and stays withdrawn.
func (s *CraftBudgetService) resolveReplayedReservation(ctx context.Context, row CraftBudgetGrantRow, call CraftBudgetCallRow) error {
	var res repocommercial.ReservationRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND key = ?", row.TenantID, call.CallKey).First(&res).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%w: reservation of call %s vanished", craft.ErrConflict, call.CallID)
	}
	if err != nil {
		return err
	}
	sameContent := res.RunID == row.RunID && commercial.Credits(res.UpperMicro) == s.policy.CallUpper
	if !sameContent {
		return fmt.Errorf("%w: reservation %s content mismatch", craft.ErrConflict, res.Key)
	}
	switch res.State {
	case commercial.ReservationStateHeld:
		// Reserved but the dispatch step crashed: complete it now.
		if err := s.budget.MarkReservationDispatched(ctx, row.TenantID, call.CallKey); err != nil {
			if errors.Is(err, repocommercial.ErrReservationNotHeld) {
				return s.resolveReplayedReservation(ctx, row, call)
			}
			return err
		}
		return nil
	case commercial.ReservationStateDispatched, commercial.ReservationStateSettling:
		return nil // already authorized and dispatched: idempotent success
	case commercial.ReservationStateSettled:
		return fmt.Errorf("%w: call %s already settled", ErrCraftCallAlreadySettled, call.CallID)
	default: // released or unrecognized: the authorization is withdrawn
		return fmt.Errorf("%w: call %s hold was withdrawn", craft.ErrGrantRevoked, call.CallID)
	}
}

// mapReserveDenial classifies a G4 reservation denial into the port's
// sentinel vocabulary; nil means "not a known denial, propagate as-is".
func (s *CraftBudgetService) mapReserveDenial(err error) error {
	switch {
	case errors.Is(err, repocommercial.ErrInsufficientBudget),
		errors.Is(err, repocommercial.ErrBudgetAccountMissing),
		errors.Is(err, repocommercial.ErrTaskBudgetExhausted),
		errors.Is(err, repocommercial.ErrBudgetVerificationExpired),
		errors.Is(err, repocommercial.ErrBudgetLotsInsufficient):
		return fmt.Errorf("%w: %v", craft.ErrBudgetDenied, err)
	case errors.Is(err, repocommercial.ErrTaskBudgetExpired),
		errors.Is(err, repocommercial.ErrInvalidBudgetRequest):
		return fmt.Errorf("%w: %v", craft.ErrGrantExpired, err)
	}
	return nil
}

// fastRefuse applies the quick grant liveness fence (revocation and deadline)
// before any commercial round-trip. Strict concurrent control remains in the
// reservation transaction — this only fails closed early.
func (s *CraftBudgetService) fastRefuse(row CraftBudgetGrantRow) error {
	if !row.Allowed {
		return fmt.Errorf("%w: grant %s was cancelled", craft.ErrGrantRevoked, row.GrantID)
	}
	if !s.now().Before(row.Deadline) {
		return fmt.Errorf("%w: grant %s deadline %s passed", craft.ErrGrantExpired, row.GrantID, row.Deadline)
	}
	return nil
}

// enforceCallCap guards the craft call-count cap after a fresh ledger insert.
// Two racing calls for the LAST slot both fail closed here; the strict
// concurrent bound is the commercial reservation itself.
func (s *CraftBudgetService) enforceCallCap(ctx context.Context, row CraftBudgetGrantRow, call CraftBudgetCallRow) error {
	var used int64
	if err := s.db.WithContext(ctx).Model(&CraftBudgetCallRow{}).
		Where("tenant_id = ? AND grant_id = ?", row.TenantID, row.GrantID).Count(&used).Error; err != nil {
		return err
	}
	if int(used) <= row.MaxCalls {
		return nil
	}
	if err := s.compensateCall(ctx, row, call); err != nil {
		return err
	}
	return fmt.Errorf("%w: grant %s capped at %d calls", craft.ErrGrantExhausted, row.GrantID, row.MaxCalls)
}

// compensateCall removes a just-inserted call row whose commercial reservation
// was denied, so the grant's call count never includes a call that never held
// budget and a later top-up can retry cleanly.
func (s *CraftBudgetService) compensateCall(ctx context.Context, row CraftBudgetGrantRow, call CraftBudgetCallRow) error {
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND call_key = ?",
		row.TenantID, call.CallKey).Delete(&CraftBudgetCallRow{}).Error; err != nil {
		return err
	}
	return nil
}

// RevokeGrant durably cancels a grant: NEW calls are refused from this point
// on. It never touches reservations — already-admitted calls keep their
// protection and settle inside their deadline; their unstarted leftovers are
// released by Reconcile once provably unstarted.
func (s *CraftBudgetService) RevokeGrant(ctx context.Context, grantID string) error {
	row, err := s.loadGrant(ctx, grantID)
	if err != nil {
		return err
	}
	if !row.Allowed {
		return nil // already revoked: idempotent
	}
	return s.db.WithContext(ctx).Model(&CraftBudgetGrantRow{}).
		Where("grant_id = ?", grantID).
		Updates(map[string]any{"allowed": false, "updated_at": s.now()}).Error
}

// Reconcile implements craft.BudgetPort: it resolves every reservation of the
// grant through the U04 reconcile rules. Confirmed-unstarted holds release;
// dispatched or unconfirmed reservations RETAIN their holds and the grant
// reports craft.ErrReconcilePending — spend protection is never released on a
// guess. A missing reservation row (authorization denied and compensated, or
// released and settled already) simply resolves.
func (s *CraftBudgetService) Reconcile(ctx context.Context, grantID string) error {
	row, err := s.loadGrant(ctx, grantID)
	if err != nil {
		return err
	}
	var calls []CraftBudgetCallRow
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND grant_id = ?", row.TenantID, row.GrantID).
		Order("created_at").Find(&calls).Error; err != nil {
		return err
	}
	pending := false
	for _, call := range calls {
		var res repocommercial.ReservationRow
		err := s.db.WithContext(ctx).
			Where("tenant_id = ? AND key = ?", row.TenantID, call.CallKey).First(&res).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue // never reserved (denied and compensated) — nothing to resolve
		}
		if err != nil {
			return err
		}
		switch res.State {
		case commercial.ReservationStateReleased, commercial.ReservationStateSettled:
			continue // terminal already
		}
		rerr := s.reconcile.Reconcile(ctx, call.CallKey)
		switch {
		case rerr == nil:
			continue // released as provably unstarted
		case errors.Is(rerr, commercialsvc.ErrReservationUnknown):
			continue
		case errors.Is(rerr, commercialsvc.ErrReconcileNeedsConfirmation):
			pending = true // outcome unproven: keep the hold
		default:
			return rerr
		}
	}
	if pending {
		return fmt.Errorf("%w: grant %s keeps unconfirmed reservations", craft.ErrReconcilePending, grantID)
	}
	return nil
}

// Extend raises both fences of a grant: the commercial task limit (G4
// ExtendTaskLimit, exactly-once per idempotency key) and the craft call cap.
// The money authority raises first — a raised cap without funds would only
// produce denials, while raised funds without cap stay safely capped.
func (s *CraftBudgetService) Extend(ctx context.Context, grantID, key string, extraCalls int, extraCredits commercial.Credits) error {
	if key == "" || extraCalls <= 0 || extraCredits <= 0 {
		return fmt.Errorf("%w: extension needs a key, positive calls and positive credits", craft.ErrInvalidInput)
	}
	row, err := s.loadGrant(ctx, grantID)
	if err != nil {
		return err
	}
	if ferr := s.fastRefuse(row); ferr != nil {
		return ferr
	}
	// Whole-operation idempotency per key: when the commercial extension key
	// already applied, neither fence moves again. ExtendTaskLimit alone cannot
	// distinguish "applied now" from "idempotent replay", so the recorded key
	// is checked explicitly before raising the craft cap.
	var applied repocommercial.TaskBudgetExtensionRow
	err = s.db.WithContext(ctx).
		Where("tenant_id = ? AND run_id = ? AND key = ?", row.TenantID, row.RunID, key).
		First(&applied).Error
	if err == nil {
		return nil // this key already raised both fences exactly once
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err := s.budget.ExtendTaskLimit(ctx, row.TenantID, row.RunID, key, extraCredits); err != nil {
		if mapped := s.mapReserveDenial(err); mapped != nil {
			return mapped
		}
		return err
	}
	return s.db.WithContext(ctx).Model(&CraftBudgetGrantRow{}).
		Where("grant_id = ? AND allowed = ?", grantID, true).
		Updates(map[string]any{
			"max_calls":  gorm.Expr("max_calls + ?", extraCalls),
			"updated_at": s.now(),
		}).Error
}

// GrantSnapshot returns the live port projection of one grant (identity,
// deadline, caps, call count, allowance) — the recovery and reissue path of
// the gateway reads this instead of caching grant state.
func (s *CraftBudgetService) GrantSnapshot(ctx context.Context, grantID string) (craft.BudgetGrant, error) {
	row, err := s.loadGrant(ctx, grantID)
	if err != nil {
		return craft.BudgetGrant{}, err
	}
	return s.snapshot(ctx, row), nil
}
