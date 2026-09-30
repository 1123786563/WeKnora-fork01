package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	repository "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/craft"
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
}

// CraftDockerSendCoordinator sequences the durable intent/hold, immutable
// receipt binding and exclusive send claim. It deliberately has no provider
// dependency, so no transaction or coordinator callback can perform Docker I/O.
type CraftDockerSendCoordinator struct {
	budget *CraftBudgetService
	claims *repository.CraftDockerSendClaimRepository
}

func NewCraftDockerSendCoordinator(budget *CraftBudgetService, claims *repository.CraftDockerSendClaimRepository) (*CraftDockerSendCoordinator, error) {
	if budget == nil || claims == nil {
		return nil, fmt.Errorf("%w: Docker send coordinator requires budget and claim stores", craft.ErrInvalidInput)
	}
	return &CraftDockerSendCoordinator{budget: budget, claims: claims}, nil
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
		if journal.GrantID != grantID || journal.State != "intent" {
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
		prep, prepErr := c.budget.prepareCraftChargeStart(ctx, grant, activityID, binding)
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
	if row.GrantID != grantID || row.State != "intent" || row.Provider == nil || row.ContainerID == nil || row.ExecID == nil || row.SendClaimedAt != nil {
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
		GrantID       string
		State         string
		Provider      *string
		ContainerID   *string
		ExecID        *string
		SendClaimedAt *time.Time
	}
	err = c.budget.db.WithContext(ctx).Table("craft_charge_start_journal").Select("grant_id, state, provider, container_id, exec_id, send_claimed_at").
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
	observation := DockerSendObservation{State: row.State, Claimed: row.SendClaimedAt != nil}
	if row.Provider != nil && row.ContainerID != nil && row.ExecID != nil {
		observation.Receipt = &repository.DockerExecReceipt{Provider: *row.Provider, ContainerID: *row.ContainerID, ExecID: *row.ExecID}
	}
	return observation, nil
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
