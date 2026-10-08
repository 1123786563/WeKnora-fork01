package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"gorm.io/gorm"
)

// CraftChargeStartKey identifies one durable external-start operation.
type CraftChargeStartKey struct {
	TenantID    uint64
	RunID       string
	ActivityKey string
}

// DockerExecReceipt is the immutable identity returned by Docker ExecCreate.
type DockerExecReceipt struct {
	Provider    string
	ContainerID string
	ExecID      string
}

// CraftDockerSendClaimRepository persists an immutable Docker exec receipt and
// grants one process the durable permission to issue ExecStart.
type CraftDockerSendClaimRepository struct {
	db *gorm.DB
}

func NewCraftDockerSendClaimRepository(db *gorm.DB) *CraftDockerSendClaimRepository {
	return &CraftDockerSendClaimRepository{db: db}
}

type craftDockerSendClaimRow struct {
	Protocol      *string    `gorm:"column:protocol"`
	Provider      *string    `gorm:"column:provider"`
	ContainerID   *string    `gorm:"column:container_id"`
	ExecID        *string    `gorm:"column:exec_id"`
	SendClaimedAt *time.Time `gorm:"column:send_claimed_at"`
	State         string     `gorm:"column:state"`
	RunRevision   int64      `gorm:"column:run_revision"`
	// Exec event pair columns hold daemon-authored timing evidence for the one
	// claimed physical start; NULL means the pair was never captured.
	ExecEventStartedNS  *int64 `gorm:"column:exec_event_started_at_ns"`
	ExecEventFinishedNS *int64 `gorm:"column:exec_event_finished_at_ns"`
}

func (craftDockerSendClaimRow) TableName() string { return "craft_charge_start_journal" }

func (r *CraftDockerSendClaimRepository) BindDockerExecReceipt(ctx context.Context, key CraftChargeStartKey, expectedRunRevision int64, receipt DockerExecReceipt) error {
	return r.bindDockerExecReceipt(ctx, key, expectedRunRevision, receipt, false)
}

func (r *CraftDockerSendClaimRepository) bindDockerExecReceipt(ctx context.Context, key CraftChargeStartKey, expectedRunRevision int64, receipt DockerExecReceipt, allowNormalInput bool) error {
	if err := validateCraftDockerSendIdentity(key, receipt); err != nil {
		return err
	}
	var affected int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockDockerSendRunFence(tx, ctx, key, expectedRunRevision); err != nil {
			return err
		}
		if !allowNormalInput {
			exists, err := r.normalInputExists(tx, ctx, key)
			if err != nil {
				return err
			}
			if exists {
				return ErrCraftDockerNormalInputConflict
			}
		}
		result := tx.Exec(`UPDATE craft_charge_start_journal
			SET provider = ?, container_id = ?, exec_id = ?
			WHERE tenant_id = ? AND run_id = ? AND activity_key = ?
			  AND state = 'intent' AND protocol = 'docker_coordinator' AND run_revision = ?
			  AND provider IS NULL AND container_id IS NULL AND exec_id IS NULL AND send_claimed_at IS NULL
			  AND EXISTS (SELECT 1 FROM agent_runs r WHERE r.tenant_id = ? AND r.run_id = ?
			    AND r.revision = ? AND r.status IN ('queued', 'running', 'recovering'))`,
			receipt.Provider, receipt.ContainerID, receipt.ExecID,
			key.TenantID, key.RunID, key.ActivityKey, expectedRunRevision, key.TenantID, key.RunID, expectedRunRevision)
		if result.Error != nil {
			return result.Error
		}
		affected = result.RowsAffected
		return nil
	})
	if err != nil {
		return err
	}
	if affected == 1 {
		return nil
	}

	row, err := r.load(ctx, key)
	if err != nil {
		return err
	}
	if row.State == "intent" && row.Protocol != nil && *row.Protocol == "docker_coordinator" && row.RunRevision == expectedRunRevision && matchesCraftDockerReceipt(row, receipt) {
		return nil
	}
	return craftDockerSendConflict("receipt cannot be bound to this operation epoch")
}

// ClaimDockerExecSend atomically consumes the sole send authorization. A
// false/nil result means an exact matching receipt was already claimed, which
// is durable replay state and never permission to send again.
func (r *CraftDockerSendClaimRepository) ClaimDockerExecSend(ctx context.Context, key CraftChargeStartKey, expectedRunRevision int64, receipt DockerExecReceipt) (bool, error) {
	return r.claimDockerExecSend(ctx, key, expectedRunRevision, receipt, false)
}

func (r *CraftDockerSendClaimRepository) claimDockerExecSend(ctx context.Context, key CraftChargeStartKey, expectedRunRevision int64, receipt DockerExecReceipt, allowNormalInput bool) (bool, error) {
	if err := validateCraftDockerSendIdentity(key, receipt); err != nil {
		return false, err
	}
	var affected int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockDockerSendRunFence(tx, ctx, key, expectedRunRevision); err != nil {
			return err
		}
		if !allowNormalInput {
			exists, err := r.normalInputExists(tx, ctx, key)
			if err != nil {
				return err
			}
			if exists {
				return ErrCraftDockerNormalInputConflict
			}
		}
		result := tx.Exec(`UPDATE craft_charge_start_journal
			SET send_claimed_at = CURRENT_TIMESTAMP
			WHERE tenant_id = ? AND run_id = ? AND activity_key = ?
			  AND state = 'intent' AND protocol = 'docker_coordinator' AND run_revision = ?
			  AND provider = ? AND container_id = ? AND exec_id = ?
			  AND send_claimed_at IS NULL
			  AND EXISTS (SELECT 1 FROM agent_runs r WHERE r.tenant_id = ? AND r.run_id = ?
			    AND r.revision = ? AND r.status IN ('queued', 'running', 'recovering'))`,
			key.TenantID, key.RunID, key.ActivityKey, expectedRunRevision,
			receipt.Provider, receipt.ContainerID, receipt.ExecID, key.TenantID, key.RunID, expectedRunRevision)
		if result.Error != nil {
			return result.Error
		}
		affected = result.RowsAffected
		return nil
	})
	if err != nil {
		return false, err
	}
	if affected == 1 {
		return true, nil
	}

	row, err := r.load(ctx, key)
	if err != nil {
		return false, err
	}
	if row.State == "intent" && row.Protocol != nil && *row.Protocol == "docker_coordinator" && row.RunRevision == expectedRunRevision && matchesCraftDockerReceipt(row, receipt) && row.SendClaimedAt != nil {
		return false, nil
	}
	return false, craftDockerSendConflict("send claim did not match an unclaimed operation epoch")
}

// RecordDockerExecEventPair persists the daemon-authored exec_start/exec_die
// timing pair for one already-claimed send. Identical evidence replays
// idempotently; an unclaimed operation, another receipt, or a different pair
// conflicts, and a non-monotonic pair is refused outright.
func (r *CraftDockerSendClaimRepository) RecordDockerExecEventPair(ctx context.Context, key CraftChargeStartKey, receipt DockerExecReceipt, startedNS, finishedNS int64) error {
	if err := validateCraftDockerSendIdentity(key, receipt); err != nil {
		return err
	}
	if startedNS <= 0 || finishedNS <= startedNS {
		return craftDockerSendConflict("exec event pair must be positive and monotonic")
	}
	result := r.db.WithContext(ctx).Exec(`UPDATE craft_charge_start_journal
		SET exec_event_started_at_ns = ?, exec_event_finished_at_ns = ?, duration_source = 'docker_exec_events'
		WHERE tenant_id = ? AND run_id = ? AND activity_key = ?
		  AND protocol = 'docker_coordinator' AND provider = ? AND container_id = ? AND exec_id = ?
		  AND send_claimed_at IS NOT NULL
		  AND exec_event_started_at_ns IS NULL AND exec_event_finished_at_ns IS NULL AND duration_source IS NULL`,
		startedNS, finishedNS, key.TenantID, key.RunID, key.ActivityKey, receipt.Provider, receipt.ContainerID, receipt.ExecID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}
	row, err := r.load(ctx, key)
	if err != nil {
		return err
	}
	if row.SendClaimedAt != nil && row.Protocol != nil && *row.Protocol == "docker_coordinator" && matchesCraftDockerReceipt(row, receipt) &&
		row.ExecEventStartedNS != nil && *row.ExecEventStartedNS == startedNS && row.ExecEventFinishedNS != nil && *row.ExecEventFinishedNS == finishedNS {
		return nil
	}
	return craftDockerSendConflict("exec event pair cannot attach to this operation epoch")
}

func (r *CraftDockerSendClaimRepository) normalInputExists(db *gorm.DB, ctx context.Context, key CraftChargeStartKey) (bool, error) {
	var count int64
	err := db.WithContext(ctx).Table("craft_docker_normal_inputs").Where("tenant_id = ? AND run_id = ? AND activity_key = ?", key.TenantID, key.RunID, key.ActivityKey).Count(&count).Error
	if err != nil {
		return false, dockerOutputDBError(err)
	}
	return count != 0, nil
}

// lockDockerSendRunFence serializes receipt/claim CAS with Run lifecycle writes.
// The no-op UPDATE acquires the database's row/write lock before checking the
// epoch and executable status; the journal transition commits in that same tx.
func lockDockerSendRunFence(tx *gorm.DB, ctx context.Context, key CraftChargeStartKey, expectedRevision int64) error {
	locked := tx.WithContext(ctx).Table("agent_runs").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).
		UpdateColumn("revision", gorm.Expr("revision"))
	if locked.Error != nil {
		return locked.Error
	}
	if locked.RowsAffected != 1 {
		return craftDockerSendConflict("current Run fence is missing")
	}
	var run struct {
		Revision int64
		Status   string
	}
	if err := tx.WithContext(ctx).Table("agent_runs").Select("revision, status").
		Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Take(&run).Error; err != nil {
		return err
	}
	if run.Revision != expectedRevision || (run.Status != "queued" && run.Status != "running" && run.Status != "recovering") {
		return craftDockerSendConflict("current Run revision or status is not executable")
	}
	return nil
}

func (r *CraftDockerSendClaimRepository) load(ctx context.Context, key CraftChargeStartKey) (craftDockerSendClaimRow, error) {
	var row craftDockerSendClaimRow
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND run_id = ? AND activity_key = ?", key.TenantID, key.RunID, key.ActivityKey).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craftDockerSendClaimRow{}, craftDockerSendConflict("operation not found")
	}
	return row, dockerNormalInputDBError(err)
}

func validateCraftDockerSendIdentity(key CraftChargeStartKey, receipt DockerExecReceipt) error {
	if key.TenantID == 0 || strings.TrimSpace(key.RunID) == "" || strings.TrimSpace(key.ActivityKey) == "" ||
		receipt.Provider != "docker" || strings.TrimSpace(receipt.ContainerID) == "" || strings.TrimSpace(receipt.ExecID) == "" {
		return craftDockerSendConflict("invalid Docker send claim identity")
	}
	return nil
}

func matchesCraftDockerReceipt(row craftDockerSendClaimRow, receipt DockerExecReceipt) bool {
	return row.Provider != nil && row.ContainerID != nil && row.ExecID != nil &&
		*row.Provider == receipt.Provider && *row.ContainerID == receipt.ContainerID && *row.ExecID == receipt.ExecID
}

func craftDockerSendConflict(reason string) error {
	return fmt.Errorf("%w: docker send claim: %s", craft.ErrConflict, reason)
}
