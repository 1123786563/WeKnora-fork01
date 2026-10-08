package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	repository "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/craft"
	"gorm.io/gorm"
)

// DockerSendOperation is an opaque handle for one prepared, held Docker
// activity. Its identity is server-owned and cannot be constructed externally.
type DockerSendOperation struct {
	budget  *CraftBudgetService
	claims  *repository.CraftDockerSendClaimRepository
	journal CraftChargeStartJournalRow
	grantID string
}

// DockerSendPermission is a process-local, non-serializable single-use token.
// Possessing it proves the durable repository claim won; it does not perform I/O.
type DockerSendPermission struct {
	receipt repository.DockerExecReceipt
	used    atomic.Bool
}

// Consume returns the bound receipt once. Replays and copied references to the
// same token cannot obtain a second authorization from this process.
func (p *DockerSendPermission) Consume() (repository.DockerExecReceipt, bool) {
	if p == nil || !p.used.CompareAndSwap(false, true) {
		return repository.DockerExecReceipt{}, false
	}
	return p.receipt, true
}

// DockerSendClaim is either the sole permission owner or a replay observation.
type DockerSendClaim struct {
	Permission *DockerSendPermission
	Replay     bool
}

// DockerSendObservation is a read-only recovery projection. A claimed or
// unknown operation can be inspected without returning a start permission.
type DockerSendObservation struct {
	State   string
	Receipt *repository.DockerExecReceipt
	Claimed bool
	// ClaimedAt is the durable send claim moment; the daemon event replay
	// window starts no later than here. ExecEvent*/DurationSource carry the
	// persisted exec_start/exec_die pair once one was captured.
	ClaimedAt           *time.Time
	ExecEventStartedNS  *int64
	ExecEventFinishedNS *int64
	DurationSource      *string
}

// CraftDockerSendCoordinator sequences the durable intent/hold, immutable
// receipt binding and exclusive send claim. It deliberately has no provider
// dependency, so no transaction or coordinator callback can perform Docker I/O.
type CraftDockerSendCoordinator struct {
	budget       *CraftBudgetService
	claims       *repository.CraftDockerSendClaimRepository
	normalInputs *repository.CraftDockerNormalInputRepository
}

func NewCraftDockerSendCoordinator(budget *CraftBudgetService, claims *repository.CraftDockerSendClaimRepository) (*CraftDockerSendCoordinator, error) {
	if budget == nil || claims == nil {
		return nil, fmt.Errorf("%w: Docker send coordinator requires budget and claim stores", craft.ErrInvalidInput)
	}
	return &CraftDockerSendCoordinator{budget: budget, claims: claims, normalInputs: repository.NewCraftDockerNormalInputRepository(budget.db)}, nil
}

// DockerNormalSendOperation is the typed coordinator handle for the normal
// stdin/output protocol. It binds the immutable staged request to a complete
// receipt before exposing the same single-use send permission as the
// outputless coordinator.
type DockerNormalSendOperation struct {
	mu     sync.RWMutex
	send   *DockerSendOperation
	inputs *repository.CraftDockerNormalInputRepository
	stage  repository.CraftDockerStagedNormalInput
}

// StagedInput returns a defensive copy of the original request and its
// currently persisted full receipt. Callers must use this value for the
// provider's inert ExecCreate; it cannot be changed by mutating the returned
// slices or maps.
func (op *DockerNormalSendOperation) StagedInput() repository.CraftDockerStagedNormalInput {
	if op == nil {
		return repository.CraftDockerStagedNormalInput{}
	}
	op.mu.RLock()
	defer op.mu.RUnlock()
	return cloneCraftDockerStagedNormalInput(op.stage)
}

// PrepareNormal commits the existing S2 intent/hold, then stages the exact
// normal create request before returning it to the provider adapter. It does
// not perform provider I/O.
func (c *CraftDockerSendCoordinator) PrepareNormal(ctx context.Context, grantID, activityID string, binding CraftCallBinding, request repository.CraftDockerNormalInputRequest) (*DockerNormalSendOperation, repository.CraftDockerStagedNormalInput, error) {
	if c == nil || c.budget == nil || c.claims == nil || c.normalInputs == nil {
		return nil, repository.CraftDockerStagedNormalInput{}, craft.ErrInvalidInput
	}
	grant, err := c.budget.loadGrant(ctx, grantID)
	if err != nil {
		return nil, repository.CraftDockerStagedNormalInput{}, err
	}
	if request.TenantID != grant.TenantID || request.RunID != grant.RunID || request.ActivityKey != activityID {
		return nil, repository.CraftDockerStagedNormalInput{}, fmt.Errorf("%w: normal request does not match admitted activity", craft.ErrConflict)
	}
	send, err := c.Prepare(ctx, grantID, activityID, binding)
	if err != nil {
		return nil, repository.CraftDockerStagedNormalInput{}, err
	}
	stage, err := c.normalInputs.Stage(ctx, request)
	if err != nil {
		return nil, repository.CraftDockerStagedNormalInput{}, err
	}
	operation := &DockerNormalSendOperation{send: send, inputs: c.normalInputs, stage: stage}
	return operation, operation.StagedInput(), nil
}

// ResumeBoundNormal recovers the only unclaimed full receipt after a crash.
// The caller must provide the original request; Stage's exact replay check
// proves recovery has not changed command, environment, stdin, timeout or
// output policy. A claimed receipt remains observation-only.
func (c *CraftDockerSendCoordinator) ResumeBoundNormal(ctx context.Context, grantID, activityID string, request repository.CraftDockerNormalInputRequest) (*DockerNormalSendOperation, repository.CraftDockerStagedNormalInput, error) {
	if c == nil || c.normalInputs == nil {
		return nil, repository.CraftDockerStagedNormalInput{}, craft.ErrInvalidInput
	}
	send, receipt, err := c.ResumeBound(ctx, grantID, activityID)
	if err != nil {
		return nil, repository.CraftDockerStagedNormalInput{}, err
	}
	if request.TenantID != send.journal.TenantID || request.RunID != send.journal.RunID || request.ActivityKey != activityID {
		return nil, repository.CraftDockerStagedNormalInput{}, fmt.Errorf("%w: normal recovery request does not match admitted activity", craft.ErrConflict)
	}
	stage, err := c.normalInputs.Stage(ctx, request)
	if err != nil {
		return nil, repository.CraftDockerStagedNormalInput{}, err
	}
	fullReceipt := repository.CraftDockerNormalReceipt{
		DockerExecReceipt: receipt,
		StdinEnabled:      stage.StdinEnabled,
		StdinByteCount:    stage.StdinByteCount,
		StdinSHA256:       stage.StdinSHA256,
		TimeoutMillis:     stage.Request.TimeoutMillis,
	}
	if stage.Receipt == nil {
		// BindDockerNormalExecReceipt first persists the S2 journal IDs, then
		// persists their full input identity. If the process stopped between
		// those commits, the immutable stage and journal IDs are sufficient to
		// finish the exact same bind; no provider call or new receipt is created.
		key := repository.CraftChargeStartKey{TenantID: send.journal.TenantID, RunID: send.journal.RunID, ActivityKey: send.journal.ActivityKey}
		if err := c.claims.BindDockerNormalExecReceipt(ctx, key, send.journal.RunRevision, fullReceipt); err != nil {
			return nil, repository.CraftDockerStagedNormalInput{}, err
		}
		stage, err = c.normalInputs.Read(ctx, key)
		if err != nil {
			return nil, repository.CraftDockerStagedNormalInput{}, err
		}
	}
	if stage.Receipt == nil || *stage.Receipt != fullReceipt {
		return nil, repository.CraftDockerStagedNormalInput{}, fmt.Errorf("%w: full normal receipt does not match bound Docker receipt", craft.ErrConflict)
	}
	operation := &DockerNormalSendOperation{send: send, inputs: c.normalInputs, stage: stage}
	return operation, operation.StagedInput(), nil
}

// Bind persists the complete provider receipt against both the S2 journal and
// the immutable staged request. Run validation and all database operations
// finish before a caller may ask for Claim; this method performs no I/O.
func (op *DockerNormalSendOperation) Bind(ctx context.Context, receipt repository.CraftDockerNormalReceipt) error {
	if op == nil || op.send == nil || op.inputs == nil || op.send.budget == nil || op.send.claims == nil {
		return craft.ErrInvalidInput
	}
	grant, err := op.send.budget.loadGrant(ctx, op.send.grantID)
	if err != nil {
		return err
	}
	if err := op.send.validateCurrentRun(ctx, grant); err != nil {
		return err
	}
	key := repository.CraftChargeStartKey{TenantID: op.send.journal.TenantID, RunID: op.send.journal.RunID, ActivityKey: op.send.journal.ActivityKey}
	if err := op.send.claims.BindDockerNormalExecReceipt(ctx, key, op.send.journal.RunRevision, receipt); err != nil {
		return err
	}
	stage, err := op.inputs.Read(ctx, key)
	if err != nil {
		return err
	}
	if stage.Receipt == nil || *stage.Receipt != receipt {
		return fmt.Errorf("%w: bound normal receipt did not persist exactly", craft.ErrConflict)
	}
	op.mu.Lock()
	op.stage = stage
	op.mu.Unlock()
	return nil
}

// Claim consumes the durable full-receipt CAS and returns a process-local
// single-use permission only to the winner. Replays are observation-only.
func (op *DockerNormalSendOperation) Claim(ctx context.Context, receipt repository.CraftDockerNormalReceipt) (DockerSendClaim, error) {
	if op == nil || op.send == nil || op.inputs == nil || op.send.budget == nil || op.send.claims == nil {
		return DockerSendClaim{}, craft.ErrInvalidInput
	}
	grant, err := op.send.budget.loadGrant(ctx, op.send.grantID)
	if err != nil {
		return DockerSendClaim{}, err
	}
	if err := op.send.validateCurrentRun(ctx, grant); err != nil {
		return DockerSendClaim{}, err
	}
	key := repository.CraftChargeStartKey{TenantID: op.send.journal.TenantID, RunID: op.send.journal.RunID, ActivityKey: op.send.journal.ActivityKey}
	claimed, err := op.send.claims.ClaimDockerNormalExecSend(ctx, key, op.send.journal.RunRevision, receipt)
	if err != nil {
		return DockerSendClaim{}, err
	}
	if !claimed {
		return DockerSendClaim{Replay: true}, nil
	}
	return DockerSendClaim{Permission: &DockerSendPermission{receipt: receipt.DockerExecReceipt}}, nil
}

func cloneCraftDockerStagedNormalInput(in repository.CraftDockerStagedNormalInput) repository.CraftDockerStagedNormalInput {
	out := in
	out.Request.Command = append([]string(nil), in.Request.Command...)
	if in.Request.Stdin != nil {
		out.Request.Stdin = append([]byte{}, in.Request.Stdin...)
	}
	if in.Request.Environment != nil {
		out.Request.Environment = make(map[string]string, len(in.Request.Environment))
		for key, value := range in.Request.Environment {
			out.Request.Environment[key] = value
		}
	}
	if in.Receipt != nil {
		receipt := *in.Receipt
		out.Receipt = &receipt
	}
	return out
}

// Prepare commits a fresh commercial hold and intent before returning. An
// exact pre-receipt replay may reuse the existing inert-create epoch; once a
// receipt is bound or a send is claimed, callers must observe/reconcile it.
func (c *CraftDockerSendCoordinator) Prepare(ctx context.Context, grantID, activityID string, binding CraftCallBinding) (*DockerSendOperation, error) {
	if c == nil || c.budget == nil || c.claims == nil || strings.TrimSpace(activityID) == "" || len(activityID) > 256 {
		return nil, craft.ErrInvalidInput
	}
	if err := binding.validate(); err != nil {
		return nil, err
	}
	grant, err := c.budget.loadGrant(ctx, grantID)
	if err != nil {
		return nil, err
	}
	if err := c.budget.fastRefuse(grant); err != nil {
		return nil, err
	}

	var journal CraftChargeStartJournalRow
	err = c.budget.db.WithContext(ctx).Where("tenant_id = ? AND run_id = ? AND activity_key = ?", grant.TenantID, grant.RunID, activityID).Take(&journal).Error
	if err == nil {
		if journal.GrantID != grantID || journal.State != "intent" || journal.Protocol == nil || *journal.Protocol != craftDockerSendProtocol {
			return nil, fmt.Errorf("%w: Docker operation is not a reusable prepared intent", craft.ErrConflict)
		}
		var call CraftBudgetCallRow
		if err := c.budget.db.WithContext(ctx).Where("tenant_id = ? AND call_key = ? AND grant_id = ? AND run_id = ?", grant.TenantID, journal.ReservationKey, grantID, grant.RunID).Take(&call).Error; err != nil {
			return nil, err
		}
		if call.ModelID != binding.ModelID || call.DelegationID != binding.DelegationID || call.Funding != binding.Funding {
			return nil, fmt.Errorf("%w: Docker activity binding changed", craft.ErrConflict)
		}
		var receipt struct{ ExecID *string }
		if err := c.budget.db.WithContext(ctx).Table("craft_charge_start_journal").Select("exec_id").Where("tenant_id = ? AND run_id = ? AND activity_key = ?", grant.TenantID, grant.RunID, activityID).Take(&receipt).Error; err != nil {
			return nil, err
		}
		if receipt.ExecID != nil {
			return nil, fmt.Errorf("%w: receipt-bound Docker operation must resume through ResumeBound", craft.ErrConflict)
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	} else {
		prep, prepErr := c.budget.prepareCraftDockerChargeStart(ctx, grant, activityID, binding)
		if prepErr != nil {
			return nil, prepErr
		}
		if prep.status != craftChargeStartPrepared {
			if prep.denial != nil {
				return nil, prep.denial
			}
			return nil, fmt.Errorf("%w: Docker intent was not prepared", craft.ErrConflict)
		}
		journal = prep.journal
	}
	if err := c.validateCurrentRun(ctx, grant, journal); err != nil {
		return nil, err
	}
	return &DockerSendOperation{budget: c.budget, claims: c.claims, journal: journal, grantID: grantID}, nil
}

// ResumeBound recovers a crash after exact receipt persistence but before the
// durable claim. Once claimed, a receipt is observation-only and this method
// returns conflict. The returned receipt is the one persisted by S1.
func (c *CraftDockerSendCoordinator) ResumeBound(ctx context.Context, grantID, activityID string) (*DockerSendOperation, repository.DockerExecReceipt, error) {
	if c == nil || c.budget == nil || c.claims == nil || strings.TrimSpace(grantID) == "" || strings.TrimSpace(activityID) == "" {
		return nil, repository.DockerExecReceipt{}, craft.ErrInvalidInput
	}
	grant, err := c.budget.loadGrant(ctx, grantID)
	if err != nil {
		return nil, repository.DockerExecReceipt{}, err
	}
	var row struct {
		CraftChargeStartJournalRow
		Provider      *string
		ContainerID   *string
		ExecID        *string
		SendClaimedAt *time.Time
	}
	err = c.budget.db.WithContext(ctx).Table("craft_charge_start_journal").Select("craft_charge_start_journal.*, provider, container_id, exec_id, send_claimed_at").
		Where("tenant_id = ? AND run_id = ? AND activity_key = ?", grant.TenantID, grant.RunID, activityID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, repository.DockerExecReceipt{}, craft.ErrNotFound
	}
	if err != nil {
		return nil, repository.DockerExecReceipt{}, err
	}
	if row.GrantID != grantID || row.State != "intent" || row.Protocol == nil || *row.Protocol != craftDockerSendProtocol || row.Provider == nil || row.ContainerID == nil || row.ExecID == nil || row.SendClaimedAt != nil {
		return nil, repository.DockerExecReceipt{}, fmt.Errorf("%w: Docker operation is not an unclaimed bound receipt", craft.ErrConflict)
	}
	if err := c.budget.validateDockerSendRun(ctx, grant, row.CraftChargeStartJournalRow); err != nil {
		return nil, repository.DockerExecReceipt{}, err
	}
	receipt := repository.DockerExecReceipt{Provider: *row.Provider, ContainerID: *row.ContainerID, ExecID: *row.ExecID}
	return &DockerSendOperation{budget: c.budget, claims: c.claims, journal: row.CraftChargeStartJournalRow, grantID: grantID}, receipt, nil
}

// Observe reloads durable recovery state for a grant-owned activity. It never
// prepares a hold, binds a receipt, or grants permission to send.
func (c *CraftDockerSendCoordinator) Observe(ctx context.Context, grantID, activityID string) (DockerSendObservation, error) {
	if c == nil || c.budget == nil || strings.TrimSpace(grantID) == "" || strings.TrimSpace(activityID) == "" {
		return DockerSendObservation{}, craft.ErrInvalidInput
	}
	grant, err := c.budget.loadGrant(ctx, grantID)
	if err != nil {
		return DockerSendObservation{}, err
	}
	var row struct {
		GrantID             string
		State               string
		Protocol            *string
		Provider            *string
		ContainerID         *string
		ExecID              *string
		SendClaimedAt       *time.Time
		ExecEventStartedNS  *int64 `gorm:"column:exec_event_started_at_ns"`
		ExecEventFinishedNS *int64 `gorm:"column:exec_event_finished_at_ns"`
		DurationSource      *string
	}
	err = c.budget.db.WithContext(ctx).Table("craft_charge_start_journal").
		Select("grant_id, state, protocol, provider, container_id, exec_id, send_claimed_at, exec_event_started_at_ns, exec_event_finished_at_ns, duration_source").
		Where("tenant_id = ? AND run_id = ? AND activity_key = ?", grant.TenantID, grant.RunID, activityID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DockerSendObservation{}, craft.ErrNotFound
	}
	if err != nil {
		return DockerSendObservation{}, err
	}
	if row.GrantID != grantID {
		return DockerSendObservation{}, craft.ErrForbidden
	}
	if row.Protocol == nil || *row.Protocol != craftDockerSendProtocol {
		return DockerSendObservation{}, fmt.Errorf("%w: operation belongs to another start protocol", craft.ErrConflict)
	}
	observation := DockerSendObservation{State: row.State, Claimed: row.SendClaimedAt != nil,
		ClaimedAt: row.SendClaimedAt, ExecEventStartedNS: row.ExecEventStartedNS, ExecEventFinishedNS: row.ExecEventFinishedNS, DurationSource: row.DurationSource}
	if row.Provider != nil && row.ContainerID != nil && row.ExecID != nil {
		observation.Receipt = &repository.DockerExecReceipt{Provider: *row.Provider, ContainerID: *row.ContainerID, ExecID: *row.ExecID}
	}
	return observation, nil
}

// RecordExecEventPair persists daemon-authored exec timing evidence for one
// grant-owned activity. It performs no provider I/O and grants no permission.
func (c *CraftDockerSendCoordinator) RecordExecEventPair(ctx context.Context, grantID, activityID string, receipt repository.DockerExecReceipt, startedNS, finishedNS int64) error {
	if c == nil || c.budget == nil || c.claims == nil || strings.TrimSpace(grantID) == "" || strings.TrimSpace(activityID) == "" {
		return craft.ErrInvalidInput
	}
	grant, err := c.budget.loadGrant(ctx, grantID)
	if err != nil {
		return err
	}
	var row struct{ GrantID string }
	err = c.budget.db.WithContext(ctx).Table("craft_charge_start_journal").Select("grant_id").
		Where("tenant_id = ? AND run_id = ? AND activity_key = ?", grant.TenantID, grant.RunID, activityID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.ErrNotFound
	}
	if err != nil {
		return err
	}
	if row.GrantID != grantID {
		return craft.ErrForbidden
	}
	return c.claims.RecordDockerExecEventPair(ctx, repository.CraftChargeStartKey{TenantID: grant.TenantID, RunID: grant.RunID, ActivityKey: activityID}, receipt, startedNS, finishedNS)
}

// Bind persists the exact receipt returned by an inert Docker exec create.
func (op *DockerSendOperation) Bind(ctx context.Context, receipt repository.DockerExecReceipt) error {
	if op == nil || op.budget == nil || op.claims == nil {
		return craft.ErrInvalidInput
	}
	grant, err := op.budget.loadGrant(ctx, op.grantID)
	if err != nil {
		return err
	}
	if err := op.validateCurrentRun(ctx, grant); err != nil {
		return err
	}
	key := repository.CraftChargeStartKey{TenantID: op.journal.TenantID, RunID: op.journal.RunID, ActivityKey: op.journal.ActivityKey}
	return op.claims.BindDockerExecReceipt(ctx, key, op.journal.RunRevision, receipt)
}

// Claim consumes the durable one-owner CAS. A replay can inspect that a claim
// was already consumed, but receives no permission to start again.
func (op *DockerSendOperation) Claim(ctx context.Context, receipt repository.DockerExecReceipt) (DockerSendClaim, error) {
	if op == nil || op.budget == nil || op.claims == nil {
		return DockerSendClaim{}, craft.ErrInvalidInput
	}
	grant, err := op.budget.loadGrant(ctx, op.grantID)
	if err != nil {
		return DockerSendClaim{}, err
	}
	if err := op.validateCurrentRun(ctx, grant); err != nil {
		return DockerSendClaim{}, err
	}
	key := repository.CraftChargeStartKey{TenantID: op.journal.TenantID, RunID: op.journal.RunID, ActivityKey: op.journal.ActivityKey}
	claimed, err := op.claims.ClaimDockerExecSend(ctx, key, op.journal.RunRevision, receipt)
	if err != nil {
		return DockerSendClaim{}, err
	}
	if !claimed {
		return DockerSendClaim{Replay: true}, nil
	}
	return DockerSendClaim{Permission: &DockerSendPermission{receipt: receipt}}, nil
}

func (op *DockerSendOperation) validateCurrentRun(ctx context.Context, grant CraftBudgetGrantRow) error {
	return op.budget.validateDockerSendRun(ctx, grant, op.journal)
}

func (c *CraftDockerSendCoordinator) validateCurrentRun(ctx context.Context, grant CraftBudgetGrantRow, journal CraftChargeStartJournalRow) error {
	return c.budget.validateDockerSendRun(ctx, grant, journal)
}
