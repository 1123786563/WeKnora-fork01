package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/craft"
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
	Provider      *string    `gorm:"column:provider"`
	ContainerID   *string    `gorm:"column:container_id"`
	ExecID        *string    `gorm:"column:exec_id"`
	SendClaimedAt *time.Time `gorm:"column:send_claimed_at"`
	State         string     `gorm:"column:state"`
	RunRevision   int64      `gorm:"column:run_revision"`
}

func (craftDockerSendClaimRow) TableName() string { return "craft_charge_start_journal" }

func (r *CraftDockerSendClaimRepository) BindDockerExecReceipt(ctx context.Context, key CraftChargeStartKey, expectedRunRevision int64, receipt DockerExecReceipt) error {
	if err := validateCraftDockerSendIdentity(key, receipt); err != nil {
		return err
	}
	result := r.db.WithContext(ctx).Exec(`UPDATE craft_charge_start_journal
		SET provider = ?, container_id = ?, exec_id = ?
		WHERE tenant_id = ? AND run_id = ? AND activity_key = ?
		  AND state = 'intent' AND run_revision = ?
		  AND provider IS NULL AND container_id IS NULL AND exec_id IS NULL AND send_claimed_at IS NULL`,
		receipt.Provider, receipt.ContainerID, receipt.ExecID,
		key.TenantID, key.RunID, key.ActivityKey, expectedRunRevision)
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
	if row.State == "intent" && row.RunRevision == expectedRunRevision && matchesCraftDockerReceipt(row, receipt) {
		return nil
	}
	return craftDockerSendConflict("receipt cannot be bound to this operation epoch")
}

// ClaimDockerExecSend atomically consumes the sole send authorization. A
// false/nil result means an exact matching receipt was already claimed, which
// is durable replay state and never permission to send again.
func (r *CraftDockerSendClaimRepository) ClaimDockerExecSend(ctx context.Context, key CraftChargeStartKey, expectedRunRevision int64, receipt DockerExecReceipt) (bool, error) {
	if err := validateCraftDockerSendIdentity(key, receipt); err != nil {
		return false, err
	}
	result := r.db.WithContext(ctx).Exec(`UPDATE craft_charge_start_journal
		SET send_claimed_at = CURRENT_TIMESTAMP
		WHERE tenant_id = ? AND run_id = ? AND activity_key = ?
		  AND state = 'intent' AND run_revision = ?
		  AND provider = ? AND container_id = ? AND exec_id = ?
		  AND send_claimed_at IS NULL`,
		key.TenantID, key.RunID, key.ActivityKey, expectedRunRevision,
		receipt.Provider, receipt.ContainerID, receipt.ExecID)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 1 {
		return true, nil
	}

	row, err := r.load(ctx, key)
	if err != nil {
		return false, err
	}
	if row.State == "intent" && row.RunRevision == expectedRunRevision && matchesCraftDockerReceipt(row, receipt) && row.SendClaimedAt != nil {
		return false, nil
	}
	return false, craftDockerSendConflict("send claim did not match an unclaimed operation epoch")
}

func (r *CraftDockerSendClaimRepository) load(ctx context.Context, key CraftChargeStartKey) (craftDockerSendClaimRow, error) {
	var row craftDockerSendClaimRow
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND run_id = ? AND activity_key = ?", key.TenantID, key.RunID, key.ActivityKey).Take(&row).Error
	if err == gorm.ErrRecordNotFound {
		return craftDockerSendClaimRow{}, craftDockerSendConflict("operation not found")
	}
	return row, err
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
