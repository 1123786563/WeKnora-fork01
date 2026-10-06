package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	reporun "github.com/Tencent/WeKnora/internal/application/repository"
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

// CraftBudgetExtensionIntentRow is the server-owned action offered for the
// current paused Run. Its composite primary key lets a completed action be
// replaced atomically only after the same Run enters a later pause cycle.
type CraftBudgetExtensionIntentRow struct {
	TenantID     uint64    `gorm:"column:tenant_id;primaryKey;autoIncrement:false"`
	SessionID    string    `gorm:"column:session_id;primaryKey"`
	RunID        string    `gorm:"column:run_id;primaryKey"`
	Key          string    `gorm:"column:intent_key;not null"`
	ExtraCalls   int       `gorm:"column:extra_calls;not null"`
	ExtraCredits int64     `gorm:"column:extra_credits;not null"`
	Status       string    `gorm:"column:status;not null"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null"`
}

func (CraftBudgetExtensionIntentRow) TableName() string { return "craft_budget_extension_intents" }

// CraftChargeStartJournalRow is the durable authorization to begin one
// chargeable external activity. Its identity and reservation are created in
// the same committed transaction; unresolved rows must never be replayed.
type CraftChargeStartJournalRow struct {
	TenantID       uint64    `gorm:"column:tenant_id;primaryKey;autoIncrement:false"`
	RunID          string    `gorm:"column:run_id;primaryKey"`
	ActivityKey    string    `gorm:"column:activity_key;primaryKey"`
	GrantID        string    `gorm:"column:grant_id;not null"`
	CallID         string    `gorm:"column:call_id;not null"`
	ReservationKey string    `gorm:"column:reservation_key;not null;uniqueIndex:uq_craft_charge_start_reservation"`
	State          string    `gorm:"column:state;not null"`
	Protocol       *string   `gorm:"column:protocol"`
	RunRevision    int64     `gorm:"column:run_revision;not null"`
	CreatedAt      time.Time `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null"`
}

func (CraftChargeStartJournalRow) TableName() string { return "craft_charge_start_journal" }

type craftChargeStartPreparationStatus uint8

const (
	craftChargeStartNotPrepared craftChargeStartPreparationStatus = iota
	craftChargeStartPrepared
)

type craftChargeStartPreparation struct {
	status  craftChargeStartPreparationStatus
	journal CraftChargeStartJournalRow
	denial  error
}

// CraftCallBinding names the identity facets a logical call binds beyond the
// grant: the (possibly empty) delegation, the model and the server-resolved
// funding source. Any change of any facet is a different logical call with
// its own sequence — the O01 DeriveCallID contract.
type CraftCallBinding struct {
	DelegationID string
	ModelID      string
	Funding      string
}

// CraftChargeStartOutcome is the typed result of beginning an external
// provider action. Unknown outcomes retain the G4 hold and require
// reconciliation before replay.
type CraftChargeStartOutcome uint8

const craftDockerSendProtocol = "docker_coordinator"

const (
	CraftChargeStartDefinitelyNotStarted CraftChargeStartOutcome = iota + 1
	CraftChargeStartStarted
	CraftChargeStartUnknown
)

const (
	craftChargeStartTimeout        = 30 * time.Second
	craftChargeStartResolveTimeout = 5 * time.Second
)

// BeginBinding commits the one-shot G4 hold and journal intent, then returns
// an opaque attempt. Call Resolve only after the complete external response
// has been observed; leaving an attempt unresolved is fail-closed.
func (s *CraftBudgetService) BeginBinding(ctx context.Context, grantID, activityID string, b CraftCallBinding) (CraftChargeStartAttempt, error) {
	if err := b.validate(); err != nil {
		return nil, err
	}
	if activityID == "" || len(activityID) > 256 {
		return nil, craft.ErrInvalidInput
	}
	row, err := s.loadGrant(ctx, grantID)
	if err != nil {
		return nil, err
	}
	if err := s.fastRefuse(row); err != nil {
		return nil, err
	}
	preparation, err := s.prepareCraftChargeStart(ctx, row, activityID, b)
	if err != nil {
		return nil, err
	}
	if preparation.status != craftChargeStartPrepared {
		if preparation.denial != nil {
			return nil, preparation.denial
		}
		return nil, fmt.Errorf("%w: charge start preparation did not commit an intent and hold", craft.ErrConflict)
	}
	if err := ctx.Err(); err != nil {
		// The resolution write must not inherit the already-canceled caller
		// context (the journal would strand in 'intent' forever), nor run
		// unbounded on a wedged database: the same bounded detach the
		// attempt's Resolve path uses applies here.
		resolveCtx, resolveDone := context.WithTimeout(context.WithoutCancel(ctx), craftChargeStartResolveTimeout)
		defer resolveDone()
		if resolveErr := s.resolveCraftChargeStart(resolveCtx, preparation.journal, CraftChargeStartDefinitelyNotStarted); resolveErr != nil {
			return nil, errors.Join(err, resolveErr)
		}
		return nil, err
	}
	initiationCtx, cancel := context.WithTimeout(ctx, craftChargeStartTimeout)
	return &craftChargeStartAttempt{service: s, journal: preparation.journal, ctx: initiationCtx, cancel: cancel}, nil
}

func (s *CraftBudgetService) resolveCraftChargeStart(ctx context.Context, journal CraftChargeStartJournalRow, outcome CraftChargeStartOutcome) error {
	state := map[CraftChargeStartOutcome]string{
		CraftChargeStartStarted: "started", CraftChargeStartUnknown: "unknown",
		CraftChargeStartDefinitelyNotStarted: "definitely_unstarted",
	}[outcome]
	if state == "" {
		return craft.ErrInvalidInput
	}
	result := s.db.WithContext(ctx).Model(&CraftChargeStartJournalRow{}).
		Where("tenant_id = ? AND run_id = ? AND activity_key = ? AND state = ? AND protocol IS NULL AND provider IS NULL AND container_id IS NULL AND exec_id IS NULL AND send_claimed_at IS NULL", journal.TenantID, journal.RunID, journal.ActivityKey, "intent").
		Updates(map[string]any{"state": state, "updated_at": s.now()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("%w: charge start intent outcome changed concurrently", craft.ErrConflict)
	}
	return nil
}

// CraftChargeStartAttempt is an opaque, durable one-shot charge start. Its
// initiation context bounds only the wait for response headers; the owner
// resolves the journal after the complete external response has been observed.
type CraftChargeStartAttempt interface {
	InitiationContext() context.Context
	CancelInitiation()
	Resolve(context.Context, CraftChargeStartOutcome) error
}

type craftChargeStartAttempt struct {
	service *CraftBudgetService
	journal CraftChargeStartJournalRow
	ctx     context.Context
	cancel  context.CancelFunc
}

func (a *craftChargeStartAttempt) InitiationContext() context.Context { return a.ctx }
func (a *craftChargeStartAttempt) CancelInitiation()                  { a.cancel() }

func (a *craftChargeStartAttempt) Resolve(ctx context.Context, outcome CraftChargeStartOutcome) error {
	if ctx == nil {
		ctx = context.Background()
	}
	resolveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), craftChargeStartResolveTimeout)
	defer cancel()
	return a.service.resolveCraftChargeStart(resolveCtx, a.journal, outcome)
}

// StartBinding is the durable Run-fenced path for chargeable model/sandbox
// starts. The callback must cover only bounded transport initiation, never the
// full provider operation. Existing gateway/sandbox paths must be migrated to
// this method before F3 can be considered closed.
func (s *CraftBudgetService) StartBinding(ctx context.Context, grantID, activityID string, b CraftCallBinding,
	start func(context.Context) (CraftChargeStartOutcome, error),
) (CraftChargeStartOutcome, error) {
	if err := b.validate(); err != nil {
		return 0, err
	}
	if activityID == "" || len(activityID) > 256 || start == nil {
		return 0, craft.ErrInvalidInput
	}
	row, err := s.loadGrant(ctx, grantID)
	if err != nil {
		return 0, err
	}
	if err := s.fastRefuse(row); err != nil {
		return 0, err
	}
	preparation, err := s.prepareCraftChargeStart(ctx, row, activityID, b)
	if err != nil {
		return 0, err
	}
	if preparation.status != craftChargeStartPrepared {
		if preparation.denial != nil {
			return CraftChargeStartDefinitelyNotStarted, preparation.denial
		}
		return CraftChargeStartUnknown, fmt.Errorf("%w: charge start preparation did not commit an intent and hold", craft.ErrConflict)
	}
	journal := preparation.journal
	if err := ctx.Err(); err != nil {
		resolveCtx, resolveDone := context.WithTimeout(context.WithoutCancel(ctx), craftChargeStartResolveTimeout)
		defer resolveDone()
		if resolveErr := s.resolveCraftChargeStart(resolveCtx, journal, CraftChargeStartDefinitelyNotStarted); resolveErr != nil {
			return CraftChargeStartUnknown, resolveErr
		}
		return CraftChargeStartDefinitelyNotStarted, err
	}
	// The authorization and G4 hold are committed before calling external
	// code. A bounded context limits cooperative transports; a callback that
	// ignores cancellation cannot keep a SQL transaction or Run lock open.
	boundedCtx, cancel := context.WithTimeout(ctx, craftChargeStartTimeout)
	defer cancel()
	outcome, startErr := start(boundedCtx)
	if boundedCtx.Err() != nil && outcome != CraftChargeStartDefinitelyNotStarted {
		outcome = CraftChargeStartUnknown
		if startErr == nil {
			startErr = boundedCtx.Err()
		}
	}
	if outcome != CraftChargeStartStarted && outcome != CraftChargeStartDefinitelyNotStarted && outcome != CraftChargeStartUnknown {
		outcome = CraftChargeStartUnknown
	}
	if startErr != nil && outcome != CraftChargeStartDefinitelyNotStarted {
		outcome = CraftChargeStartUnknown
	}
	// The journal resolution must survive a caller context that was canceled
	// while the external callback ran (client disconnect, stop, shutdown):
	// resolve with a bounded detached context, mirroring the attempt's own
	// Resolve discipline. Without this the journal strands in 'intent', the
	// activity replay is refused, and the lease recovery scan excludes the
	// Run until someone reconciles by hand.
	resolveCtx, resolveDone := context.WithTimeout(context.WithoutCancel(ctx), craftChargeStartResolveTimeout)
	defer resolveDone()
	if resolveErr := s.resolveCraftChargeStart(resolveCtx, journal, outcome); resolveErr != nil {
		// Join like BeginBinding: the caller must see BOTH the external
		// callback failure and the persistence failure.
		return outcome, errors.Join(startErr, resolveErr)
	}
	return outcome, startErr
}

func (s *CraftBudgetService) prepareCraftChargeStart(ctx context.Context, row CraftBudgetGrantRow, activityID string, b CraftCallBinding) (craftChargeStartPreparation, error) {
	return s.prepareCraftChargeStartWithProtocol(ctx, row, activityID, b, nil)
}

func (s *CraftBudgetService) prepareCraftDockerChargeStart(ctx context.Context, row CraftBudgetGrantRow, activityID string, b CraftCallBinding) (craftChargeStartPreparation, error) {
	protocol := craftDockerSendProtocol
	return s.prepareCraftChargeStartWithProtocol(ctx, row, activityID, b, &protocol)
}

func (s *CraftBudgetService) prepareCraftChargeStartWithProtocol(ctx context.Context, row CraftBudgetGrantRow, activityID string, b CraftCallBinding, protocol *string) (craftChargeStartPreparation, error) {
	callKey := CraftCallKey("activity/" + activityID)
	// Bounded whole-transaction retry on the per-binding sequence slot:
	// two concurrent charge starts with identical facets can read the same
	// MAX(call_seq) and race the INSERT (a failed statement aborts the
	// transaction on PostgreSQL, so the retry must restart the transaction,
	// not continue inside it). The unique index remains the final
	// arbiter — same discipline as AuthorizeBinding's sequence loop.
	for attempt := 0; attempt < craftCallSeqAttempts; attempt++ {
		preparation := craftChargeStartPreparation{}
		err := s.prepareCraftChargeStartTx(ctx, row, activityID, b, protocol, callKey, &preparation)
		if err != nil && isUniqueViolation(err) {
			continue
		}
		if err != nil {
			return craftChargeStartPreparation{}, err
		}
		return preparation, nil
	}
	return craftChargeStartPreparation{}, fmt.Errorf("%w: charge start call sequence contention on grant %s", craft.ErrConflict, row.GrantID)
}

func (s *CraftBudgetService) prepareCraftChargeStartTx(ctx context.Context, row CraftBudgetGrantRow, activityID string, b CraftCallBinding, protocol *string, callKey string, preparation *craftChargeStartPreparation) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		run, err := lockCraftRun(tx, ctx, row)
		if err != nil {
			return err
		}
		if err := chargeableCraftRun(run); err != nil {
			return err
		}
		var mapping repocommercial.TaskBudgetRow
		if err := tx.Where("tenant_id = ? AND run_id = ?", row.TenantID, row.RunID).Take(&mapping).Error; err != nil {
			return err
		}
		if run.SessionID == "" || mapping.RootRunID != run.SessionID {
			return craft.ErrForbidden
		}
		var previous CraftChargeStartJournalRow
		previousErr := tx.Where("tenant_id = ? AND run_id = ? AND activity_key = ?", row.TenantID, row.RunID, activityID).Take(&previous).Error
		if previousErr == nil && previous.State != "definitely_unstarted" {
			return fmt.Errorf("%w: activity start already attempted; reconcile before retry", craft.ErrConflict)
		}
		if previousErr == nil {
			// A DEFINITELY-not-started attempt is a clean slate: nothing was
			// physically sent, so the activity may restart instead of
			// deadlocking every retry on a phantom conflict. The same-key
			// ledger call row AND its reservation must be cleared in the
			// SAME transaction, or the re-insert collides on the primary
			// key and the "restart" never actually works.
			if err := tx.Where("tenant_id = ? AND run_id = ? AND activity_key = ?", row.TenantID, row.RunID, activityID).
				Delete(&CraftChargeStartJournalRow{}).Error; err != nil {
				return err
			}
			if err := tx.Where("tenant_id = ? AND call_key = ?", row.TenantID, callKey).
				Delete(&CraftBudgetCallRow{}).Error; err != nil {
				return err
			}
			// Clear the dispatched reservation row in the SAME transaction:
			// ReserveInTx only replays idempotently in the held state, so a
			// leftover dispatched row blocks the restart's re-reserve.
			// ReservationRow's key column is `key` (reservation_key is the
			// BudgetLotAllocationRow column) — the wrong column made this
			// DELETE fail on every restart, so the "clean restart" path
			// never actually worked. The reservation's LOT ALLOCATION row
			// shares the reservation_key and must go with it, or the
			// restart's re-reserve collides on the allocation primary key
			// (and the freed lot would double-count capacity).
			if err := tx.Where("tenant_id = ? AND `key` = ?", row.TenantID, callKey).
				Delete(&repocommercial.ReservationRow{}).Error; err != nil {
				return err
			}
			if err := tx.Where("tenant_id = ? AND reservation_key = ?", row.TenantID, callKey).
				Delete(&repocommercial.BudgetLotAllocationRow{}).Error; err != nil {
				return err
			}
		} else if !errors.Is(previousErr, gorm.ErrRecordNotFound) {
			return previousErr
		}
		var used int64
		if err := tx.Model(&CraftBudgetCallRow{}).Where("tenant_id = ? AND grant_id = ? AND call_seq >= 0", row.TenantID, row.GrantID).Count(&used).Error; err != nil {
			return err
		}
		if used >= int64(row.MaxCalls) {
			if err := s.pauseCraftRunInTx(ctx, tx, row); err != nil {
				return err
			}
			preparation.denial = craft.ErrGrantExhausted
			return nil
		}
		var seq int64
		if err := tx.Model(&CraftBudgetCallRow{}).Where("tenant_id = ? AND run_id = ? AND delegation_id = ? AND model_id = ? AND funding = ?", row.TenantID, row.RunID, b.DelegationID, b.ModelID, b.Funding).Select("COALESCE(MAX(call_seq),0)").Scan(&seq).Error; err != nil {
			return err
		}
		seq++
		callID := "activity/" + activityID
		call := CraftBudgetCallRow{TenantID: row.TenantID, CallKey: callKey, GrantID: row.GrantID, RunID: row.RunID,
			DelegationID: b.DelegationID, ModelID: b.ModelID, Funding: b.Funding, CallSeq: seq, CallID: callID, CreatedAt: s.now()}
		if err := tx.Create(&call).Error; err != nil {
			return err
		}
		reservation, err := s.budget.ReserveInTx(ctx, tx, commercial.BudgetRequest{TenantID: row.TenantID, RunID: row.RunID, Key: callKey, Upper: s.policy.CallUpper, Deadline: row.Deadline})
		if err != nil {
			if mapped := s.mapReserveDenial(err); mapped != nil {
				if e := tx.Where("tenant_id = ? AND call_key = ?", row.TenantID, callKey).Delete(&CraftBudgetCallRow{}).Error; e != nil {
					return e
				}
				if e := s.pauseCraftRunInTx(ctx, tx, row); e != nil {
					return e
				}
				preparation.denial = mapped
				return nil
			}
			return err
		}
		_ = reservation
		if err := s.budget.MarkReservationDispatchedInTx(ctx, tx, row.TenantID, callKey); err != nil {
			return err
		}
		now := s.now()
		journal := CraftChargeStartJournalRow{TenantID: row.TenantID, RunID: row.RunID, ActivityKey: activityID,
			GrantID: row.GrantID, CallID: callID, ReservationKey: callKey, State: "intent", Protocol: protocol,
			RunRevision: run.Revision, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&journal).Error; err != nil {
			return err
		}
		preparation.journal = journal
		preparation.status = craftChargeStartPrepared
		return nil
	})
}

// PauseRunForBudget orders a durable budget pause against StartBinding using
// the same agent_runs write lock.
func (s *CraftBudgetService) PauseRunForBudget(ctx context.Context, grantID string) error {
	row, err := s.loadGrant(ctx, grantID)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockCraftRun(tx, ctx, row); err != nil {
			return err
		}
		return s.pauseCraftRunInTx(ctx, tx, row)
	})
}

func (s *CraftBudgetService) pauseCraftRunInTx(ctx context.Context, tx *gorm.DB, grant CraftBudgetGrantRow) error {
	return s.runs.PauseForCraftBudgetInTx(ctx, tx,
		agentruntime.RunKey{TenantID: grant.TenantID, RunID: grant.RunID}, craftBudgetWaitReason)
}

type lockedCraftRun struct {
	SessionID, Status, WaitReason string
	Revision                      int64
}

func lockCraftRun(tx *gorm.DB, ctx context.Context, grant CraftBudgetGrantRow) (lockedCraftRun, error) {
	// A no-op UPDATE acquires the row write lock on PostgreSQL and SQLite's
	// database write reservation before reading the state used by the fence.
	lock := tx.WithContext(ctx).Table("agent_runs").Where("tenant_id = ? AND run_id = ?", grant.TenantID, grant.RunID).
		UpdateColumn("revision", gorm.Expr("revision"))
	if lock.Error != nil {
		return lockedCraftRun{}, lock.Error
	}
	if lock.RowsAffected != 1 {
		return lockedCraftRun{}, craft.ErrNotFound
	}
	var run lockedCraftRun
	err := tx.WithContext(ctx).Table("agent_runs").Select("session_id, status, wait_reason, revision").
		Where("tenant_id = ? AND run_id = ?", grant.TenantID, grant.RunID).Take(&run).Error
	return run, err
}

func chargeableCraftRun(run lockedCraftRun) error {
	if run.Status == "waiting_user" && run.WaitReason == craftBudgetWaitReason {
		return craft.ErrBudgetDenied
	}
	if run.Status != "queued" && run.Status != "running" && run.Status != "recovering" {
		return craft.ErrConflict
	}
	return nil
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
	runs      *reporun.AgentRunStore
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
		runs:      reporun.NewAgentRunStore(db),
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

const (
	craftBudgetExtensionExtraCalls    = 10
	craftBudgetExtensionIntentPending = "pending"
	craftBudgetExtensionIntentDone    = "completed"
)

func newCraftBudgetExtensionIntentKey() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "budget-extension-" + hex.EncodeToString(buf), nil
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
		Where("tenant_id = ? AND grant_id = ? AND call_seq >= 0", row.TenantID, row.GrantID).Count(&used).Error
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
		return craft.BudgetGrant{}, fmt.Errorf("%w: admission needs a tenant and run id", craft.ErrInvalidInput)
	}
	// The gateway's credential request carries only a Run ID. Resolve its
	// Task/Session from the durable, tenant-scoped Run instead of accepting a
	// client-supplied Session ID or creating a budget keyed to the Run.
	var run struct{ SessionID string }
	runErr := s.db.WithContext(ctx).Table("agent_runs").Select("session_id").
		Where("tenant_id = ? AND run_id = ?", scope.TenantID, runID).Take(&run).Error
	if runErr == nil {
		if scope.SessionID != "" && scope.SessionID != run.SessionID {
			return craft.BudgetGrant{}, craft.ErrForbidden
		}
		scope.SessionID = run.SessionID
	} else if errors.Is(runErr, gorm.ErrRecordNotFound) {
		return craft.BudgetGrant{}, craft.ErrNotFound
	} else {
		return craft.BudgetGrant{}, runErr
	}
	if scope.SessionID == "" || scope.SessionID == runID {
		return craft.BudgetGrant{}, fmt.Errorf("%w: durable Task identity unavailable", craft.ErrInvalidInput)
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
	// The Task is the Session. Its commercial owner row is registered once;
	// every Run is a zero-limit child mapping to that same Task Budget.
	if err := s.budget.EnsureTaskBudget(ctx, row.TenantID, scope.SessionID, s.policy.TaskLimit, row.Deadline); err != nil {
		return s.snapshot(ctx, row), err
	}
	// A newly admitted durable Run is an authorized continuation of the same
	// Task. Renew only the root deadline; G4 preserves limit and counters.
	if err := s.budget.RenewTaskBudgetDeadline(ctx, row.TenantID, scope.SessionID, row.Deadline); err != nil {
		return s.snapshot(ctx, row), err
	}
	if err := s.budget.AttachChildRun(ctx, row.TenantID, row.RunID, scope.SessionID); err != nil {
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
	if err := s.requireRunChargeable(ctx, row); err != nil {
		return "", err
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
			return "", s.pauseOnDenial(ctx, row, err)
		}
		if err := s.authorizeReservedCall(ctx, row, call, true); err != nil {
			return "", s.pauseOnDenial(ctx, row, err)
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
	if err := s.requireRunChargeable(ctx, row); err != nil {
		return err
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
			return s.pauseOnDenial(ctx, row, err)
		}
	} else if err != nil {
		return err
	} else if call.CallSeq < 0 {
		return fmt.Errorf("%w: call id is reserved for an extension marker", craft.ErrConflict)
	} else if call.GrantID != row.GrantID {
		return fmt.Errorf("%w: call %s belongs to another grant", craft.ErrForbidden, callID)
	}
	return s.pauseOnDenial(ctx, row, s.authorizeReservedCall(ctx, row, call, fresh))
}

// AuthorizeSandbox reserves a server-configured activity upper before one
// sandbox action. The event identity must come from the durable lifecycle
// event, so retrying its admission cannot create a second Task hold. The
// sandbox executor must call this before the external action, not after it.
//
// Sandbox activities get their own ledger facet: the empty-facet namespace
// AuthorizeCall's generic fresh branch uses would collide on the
// (tenant, run, ”, ”, ”, 0) unique tuple at the second distinct activity.
// The facet plus a per-run monotonic sequence (re-read under the unique
// index's final arbitration, like AuthorizeBinding) keeps every distinct
// activity insertable.
func (s *CraftBudgetService) AuthorizeSandbox(ctx context.Context, grantID, activityID string) error {
	if activityID == "" || len(activityID) > 256 {
		return fmt.Errorf("%w: invalid sandbox activity identity", craft.ErrInvalidInput)
	}
	row, err := s.loadGrant(ctx, grantID)
	if err != nil {
		return err
	}
	if ferr := s.fastRefuse(row); ferr != nil {
		return ferr
	}
	if err := s.requireRunChargeable(ctx, row); err != nil {
		return err
	}
	callID := "sandbox/" + activityID
	callKey := CraftCallKey(callID)
	var call CraftBudgetCallRow
	err = s.db.WithContext(ctx).Where("tenant_id = ? AND call_key = ?", row.TenantID, callKey).First(&call).Error
	fresh := false
	if errors.Is(err, gorm.ErrRecordNotFound) {
		fresh = true
		allocated := false
		for attempt := 0; attempt < craftCallSeqAttempts && !allocated; attempt++ {
			var seq int64
			if err := s.db.WithContext(ctx).Model(&CraftBudgetCallRow{}).
				Where("tenant_id = ? AND run_id = ? AND delegation_id = '' AND model_id = ? AND funding = ''",
					row.TenantID, row.RunID, craftSandboxCallModelID).
				Select("COALESCE(MAX(call_seq),0)").Scan(&seq).Error; err != nil {
				return err
			}
			seq++
			call = CraftBudgetCallRow{
				TenantID: row.TenantID, CallKey: callKey, GrantID: row.GrantID, RunID: row.RunID,
				DelegationID: "", ModelID: craftSandboxCallModelID, Funding: "",
				CallSeq: seq, CallID: callID, CreatedAt: s.now(),
			}
			if err := s.db.WithContext(ctx).Create(&call).Error; err != nil {
				// A concurrent sandbox authorization took this sequence slot;
				// re-read the fresh maximum and retry with the next one.
				continue
			}
			allocated = true
		}
		if !allocated {
			return fmt.Errorf("%w: sandbox call sequence contention on grant %s", craft.ErrConflict, grantID)
		}
	} else if err != nil {
		return err
	} else if call.CallSeq < 0 {
		return fmt.Errorf("%w: call id is reserved for an extension marker", craft.ErrConflict)
	} else if call.GrantID != row.GrantID {
		return fmt.Errorf("%w: call %s belongs to another grant", craft.ErrForbidden, callID)
	}
	if err := s.enforceCallCap(ctx, row, call); err != nil {
		return s.pauseOnDenial(ctx, row, err)
	}
	return s.pauseOnDenial(ctx, row, s.authorizeReservedCall(ctx, row, call, fresh))
}

// craftSandboxCallModelID is the sandbox activity facet of the call ledger:
// distinct from every model/delegation binding facet and from the extension
// marker namespace, so sandbox sequences never collide with either.
const craftSandboxCallModelID = "__craft_sandbox__"

// F08 T-3: the fixed identity facets of the server-owned offline web build
// activity. See docs/plans/2026-10-04-craft-f08-binding-proposal.md for the
// rationale and the pending budget-owner sign-off; dispatchers must consume
// CraftWebBuildCallBinding, never inline these values or accept caller input.
const (
	CraftWebBuildCallDelegationID = "web-build"
	CraftWebBuildCallModelID      = "__craft_web_build__"
)

// CraftWebBuildCallBinding is the fail-closed binding for the fixed offline
// web build activity: a named delegation facet, the non-model sentinel model
// facet (the build exec consumes no model tokens) and platform funding per
// PlatformAdmissionPolicy. Overrides require budget-owner sign-off.
func CraftWebBuildCallBinding() CraftCallBinding {
	return CraftCallBinding{
		DelegationID: CraftWebBuildCallDelegationID,
		ModelID:      CraftWebBuildCallModelID,
		Funding:      commercial.FundingPlatform,
	}
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
		Where("tenant_id = ? AND grant_id = ? AND call_seq >= 0", row.TenantID, row.GrantID).Count(&used).Error; err != nil {
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
	if err := s.budget.ExtendTaskLimit(ctx, row.TenantID, row.RunID, key, extraCredits); err != nil {
		if mapped := s.mapReserveDenial(err); mapped != nil {
			return mapped
		}
		return err
	}
	var applied repocommercial.TaskBudgetExtensionRow
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND run_id = ? AND key = ?", row.TenantID, row.RunID, key).Take(&applied).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// ExtendTaskLimit just applied (or idempotently replayed) this key,
			// so a missing extension row means the durable extension evidence
			// vanished — surface the domain NotFound instead of a bare gorm error.
			return craft.ErrNotFound
		}
		return err
	}
	if commercial.Credits(applied.ExtraMicro) != extraCredits {
		return fmt.Errorf("%w: extension key reused with different credits", craft.ErrConflict)
	}
	return s.applyCraftCallExtension(ctx, row, key, extraCalls)
}

// applyCraftCallExtension records the extension key and increments the logical
// call cap in one transaction. Negative CallSeq values are durable extension
// markers in the existing call table and are excluded from call counts. G4
// commits first, so a replay can repair this transaction after process loss.
func (s *CraftBudgetService) applyCraftCallExtension(ctx context.Context, grant CraftBudgetGrantRow, key string, extraCalls int) error {
	callID := fmt.Sprintf("extension/%d/%s/%s", grant.TenantID, grant.RunID, key)
	callKey := CraftCallKey(callID)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var applied CraftBudgetCallRow
		err := tx.Where("tenant_id = ? AND call_key = ?", grant.TenantID, callKey).Take(&applied).Error
		if err == nil {
			if applied.GrantID != grant.GrantID || applied.CallSeq != -int64(extraCalls) {
				return fmt.Errorf("%w: extension key reused with different call cap", craft.ErrConflict)
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// The marker's model facet embeds the extension key: the unique
		// index tuple otherwise collapses every same-sized extension of one
		// run into the first marker, and a second "+N calls" request with a
		// different key could then never commit.
		marker := CraftBudgetCallRow{
			TenantID: grant.TenantID, CallKey: callKey, GrantID: grant.GrantID,
			RunID: grant.RunID, ModelID: fmt.Sprintf("__craft_budget_extension__/%s", key),
			Funding: commercial.FundingPlatform, CallSeq: -int64(extraCalls), CallID: callID,
			CreatedAt: s.now(),
		}
		if err := tx.Create(&marker).Error; err != nil {
			return err
		}
		result := tx.Model(&CraftBudgetGrantRow{}).
			Where("tenant_id = ? AND grant_id = ?", grant.TenantID, grant.GrantID).
			Updates(map[string]any{"max_calls": gorm.Expr("max_calls + ?", extraCalls), "updated_at": s.now()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return craft.ErrNotFound
		}
		return nil
	})
	if err == nil {
		return nil
	}
	// Concurrent same-key callers may race after the initial read. Resolve a
	// unique-index winner after rollback; do not apply the cap a second time.
	var applied CraftBudgetCallRow
	readErr := s.db.WithContext(ctx).Where("tenant_id = ? AND call_key = ?", grant.TenantID, callKey).Take(&applied).Error
	if readErr == nil && applied.GrantID == grant.GrantID && applied.CallSeq == -int64(extraCalls) {
		return nil
	}
	return err
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

const craftBudgetWaitReason = "budget_exhausted"

// requireRunChargeable prevents a separately extended grant from bypassing a
// paused Run. A durable Run and its Session-rooted G4 mapping are mandatory
// because they provide the pause and recovery authority.
func (s *CraftBudgetService) requireRunChargeable(ctx context.Context, grant CraftBudgetGrantRow) error {
	var run struct{ SessionID, Status, WaitReason string }
	err := s.db.WithContext(ctx).Table("agent_runs").Select("session_id, status, wait_reason").
		Where("tenant_id = ? AND run_id = ?", grant.TenantID, grant.RunID).Take(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.ErrNotFound
	}
	if err != nil {
		return err
	}
	var mapping repocommercial.TaskBudgetRow
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND run_id = ?", grant.TenantID, grant.RunID).Take(&mapping).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return craft.ErrNotFound
		}
		return err
	}
	if run.SessionID == "" || mapping.RootRunID != run.SessionID {
		return craft.ErrForbidden
	}
	if run.Status == "waiting_user" && run.WaitReason == craftBudgetWaitReason {
		return craft.ErrBudgetDenied
	}
	if run.Status != "queued" && run.Status != "running" && run.Status != "recovering" {
		return fmt.Errorf("%w: run is not chargeable", craft.ErrConflict)
	}
	return nil
}

// pauseOnDenial persists the budget decision before returning the refusal to
// the forwarder. No already dispatched activity is rolled back here.
func (s *CraftBudgetService) pauseOnDenial(ctx context.Context, grant CraftBudgetGrantRow, denial error) error {
	if denial == nil || !(errors.Is(denial, craft.ErrBudgetDenied) || errors.Is(denial, craft.ErrGrantExhausted)) {
		return denial
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockCraftRun(tx, ctx, grant); err != nil {
			return err
		}
		return s.pauseCraftRunInTx(ctx, tx, grant)
	})
	if err != nil {
		return err
	}
	return denial
}

// BudgetPause projects the durable pause fact and one stable server-owned
// extension action. Limit and Used are logical call counts; the action's
// ExtraCredits is the configured extension quantum, never an account balance.
func (s *CraftBudgetService) BudgetPause(ctx context.Context, scope craft.Scope, runID string) (craft.BudgetPause, error) {
	if scope.TenantID == 0 || scope.SessionID == "" || scope.UserID == "" || runID == "" {
		return craft.BudgetPause{}, craft.ErrInvalidInput
	}
	var grant CraftBudgetGrantRow
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND run_id = ?", scope.TenantID, runID).Take(&grant).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return craft.BudgetPause{}, craft.ErrNotFound
		}
		return craft.BudgetPause{}, err
	}
	var intent CraftBudgetExtensionIntentRow
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize GET-created intents with POST resume and any later pause on
		// this exact Run. The same lock primitive works across PostgreSQL and
		// SQLite deployments.
		run, err := lockCraftRun(tx, ctx, grant)
		if err != nil {
			return err
		}
		if run.SessionID != scope.SessionID || run.Status != "waiting_user" || run.WaitReason != craftBudgetWaitReason {
			return craft.ErrNotFound
		}
		err = tx.WithContext(ctx).Where("tenant_id = ? AND session_id = ? AND run_id = ?", scope.TenantID, scope.SessionID, runID).
			Take(&intent).Error
		if err == nil && intent.Status == craftBudgetExtensionIntentPending {
			return nil
		}
		intentMissing := errors.Is(err, gorm.ErrRecordNotFound)
		if err != nil && !intentMissing {
			return err
		}
		key, err := newCraftBudgetExtensionIntentKey()
		if err != nil {
			return err
		}
		now := s.now()
		action := CraftBudgetExtensionIntentRow{
			TenantID: scope.TenantID, SessionID: scope.SessionID, RunID: runID,
			Key: key, ExtraCalls: craftBudgetExtensionExtraCalls,
			ExtraCredits: int64(s.policy.TaskLimit), Status: craftBudgetExtensionIntentPending,
			CreatedAt: now, UpdatedAt: now,
		}
		if intentMissing {
			if err := tx.WithContext(ctx).Create(&action).Error; err != nil {
				return err
			}
			intent = action
			return nil
		}
		if intent.Status != craftBudgetExtensionIntentDone {
			return fmt.Errorf("%w: invalid budget extension intent state", craft.ErrConflict)
		}
		result := tx.WithContext(ctx).Model(&CraftBudgetExtensionIntentRow{}).
			Where("tenant_id = ? AND session_id = ? AND run_id = ? AND status = ?", scope.TenantID, scope.SessionID, runID, craftBudgetExtensionIntentDone).
			Updates(map[string]any{"intent_key": action.Key, "extra_calls": action.ExtraCalls,
				"extra_credits": action.ExtraCredits, "status": action.Status,
				"created_at": action.CreatedAt, "updated_at": action.UpdatedAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return craft.ErrConflict
		}
		intent = action
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.BudgetPause{}, craft.ErrNotFound
	}
	if err != nil {
		return craft.BudgetPause{}, err
	}
	grantView := s.snapshot(ctx, grant)
	return craft.BudgetPause{
		RunID: runID, Reason: "exhausted", Limit: int64(grantView.MaxCalls), Used: int64(grantView.UsedCalls),
		ExtensionAction: &craft.BudgetExtensionAction{Key: intent.Key, ExtraCalls: intent.ExtraCalls, ExtraCredits: intent.ExtraCredits},
	}, nil
}

// authorizeBudgetActor is deliberately checked against current server rows,
// not the caller's role claim. The Task owner or an active tenant billing
// admin/owner may extend; ordinary collaborators never can.
func (s *CraftBudgetService) authorizeBudgetActor(ctx context.Context, scope craft.Scope) error {
	var session struct{ UserID string }
	err := s.db.WithContext(ctx).Table("sessions").Select("user_id").
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", scope.TenantID, scope.SessionID).Take(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.ErrNotFound
	}
	if err != nil {
		return err
	}
	if session.UserID == scope.UserID {
		return nil
	}
	var member struct{ Role, Status string }
	err = s.db.WithContext(ctx).Table("tenant_members").Select("role, status").
		Where("tenant_id = ? AND user_id = ? AND deleted_at IS NULL", scope.TenantID, scope.UserID).
		Order("id ASC").Take(&member).Error
	if err == nil && member.Status == "active" && (member.Role == "admin" || member.Role == "owner") {
		return nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return craft.ErrForbidden
}

// MayExtendBudget projects the CURRENT extension authority (Task owner or
// active tenant billing admin/owner) without touching any grant or Run row.
// The T20 budget-pause HTTP surface uses it to project can_extend
// server-side; the authoritative check still reruns inside ExtendAndResume
// before any extension lands.
func (s *CraftBudgetService) MayExtendBudget(ctx context.Context, scope craft.Scope) error {
	return s.authorizeBudgetActor(ctx, scope)
}

// ExtendAndResume raises a paused Run's grant and requests recovery only
// after every dispatched effect is reconciled. Unknown outcomes keep the Run
// parked, preventing a resumed worker from replaying an uncertain call.
func (s *CraftBudgetService) ExtendAndResume(ctx context.Context, scope craft.Scope, runID, key string, extraCalls int, extraCredits commercial.Credits) error {
	if err := s.authorizeBudgetActor(ctx, scope); err != nil {
		return err
	}
	var grant CraftBudgetGrantRow
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND run_id = ?", scope.TenantID, runID).Take(&grant).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return craft.ErrNotFound
		}
		return err
	}
	pause, err := s.BudgetPause(ctx, scope, runID)
	if err != nil {
		return err
	}
	action := pause.ExtensionAction
	if action == nil || action.Key != key || action.ExtraCalls != extraCalls || action.ExtraCredits != int64(extraCredits) {
		return fmt.Errorf("%w: extension request does not match the pending server action", craft.ErrConflict)
	}
	if err := s.Extend(ctx, grant.GrantID, key, extraCalls, extraCredits); err != nil {
		return err
	}
	if err := s.Reconcile(ctx, grant.GrantID); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		run, err := lockCraftRun(tx, ctx, grant)
		if err != nil {
			return err
		}
		if run.SessionID != scope.SessionID || run.Status != "waiting_user" || run.WaitReason != craftBudgetWaitReason {
			return craft.ErrConflict
		}
		var intent CraftBudgetExtensionIntentRow
		if err := tx.WithContext(ctx).Where("tenant_id = ? AND session_id = ? AND run_id = ? AND intent_key = ? AND status = ?",
			scope.TenantID, scope.SessionID, runID, key, craftBudgetExtensionIntentPending).Take(&intent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return craft.ErrConflict
			}
			return err
		}
		if intent.ExtraCalls != extraCalls || intent.ExtraCredits != int64(extraCredits) {
			return craft.ErrConflict
		}
		result := tx.Table("agent_runs").Where(
			"tenant_id = ? AND session_id = ? AND run_id = ? AND status = ? AND wait_reason = ?",
			scope.TenantID, scope.SessionID, runID, "waiting_user", craftBudgetWaitReason).
			Updates(map[string]any{"status": "recovering", "wait_reason": "", "revision": gorm.Expr("revision + 1"), "updated_at": s.now()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return craft.ErrConflict
		}
		completed := tx.WithContext(ctx).Model(&CraftBudgetExtensionIntentRow{}).
			Where("tenant_id = ? AND session_id = ? AND run_id = ? AND intent_key = ? AND status = ?",
				scope.TenantID, scope.SessionID, runID, key, craftBudgetExtensionIntentPending).
			Updates(map[string]any{"status": craftBudgetExtensionIntentDone, "updated_at": s.now()})
		if completed.Error != nil {
			return completed.Error
		}
		if completed.RowsAffected != 1 {
			return craft.ErrConflict
		}
		return nil
	})
}

// validateDockerSendRun applies the server-owned Run fence to each receipt
// transition. The journal's original revision is immutable; a changed Run
// epoch can never authorize a provider send.
func (s *CraftBudgetService) validateDockerSendRun(ctx context.Context, grant CraftBudgetGrantRow, journal CraftChargeStartJournalRow) error {
	if grant.TenantID != journal.TenantID || grant.RunID != journal.RunID || grant.GrantID != journal.GrantID || journal.State != "intent" {
		return fmt.Errorf("%w: Docker operation does not match its prepared grant epoch", craft.ErrConflict)
	}
	var run lockedCraftRun
	err := s.db.WithContext(ctx).Table("agent_runs").Select("session_id, status, wait_reason, revision").
		Where("tenant_id = ? AND run_id = ?", journal.TenantID, journal.RunID).Take(&run).Error
	if err != nil {
		return err
	}
	if run.Revision != journal.RunRevision {
		return fmt.Errorf("%w: Docker operation Run revision changed", craft.ErrConflict)
	}
	return chargeableCraftRun(run)
}
