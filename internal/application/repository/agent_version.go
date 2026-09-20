package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/types"
)

// ErrAgentVersionInvalid reports a freeze candidate that failed validation
// before any durable write.
var ErrAgentVersionInvalid = errors.New("invalid agent version")

// agentVersionFreezeAttempts bounds the retry loop around concurrent
// freezes: two transactions can read the same MAX(version_number) before
// either insert commits; the unique index rejects the loser, which retries
// and re-reads the winner's number.
const agentVersionFreezeAttempts = 3

// AgentVersionRepository persists the Agent domain's immutable tenant agent
// versions. The surface is append/read by construction — there is no update
// or delete method, so no code path can rewrite or remove a frozen snapshot.
type AgentVersionRepository interface {
	// Freeze appends the next immutable version of one (tenant, agent).
	// The caller supplies TenantID, AgentID, Snapshot, SourceSHA256,
	// FrozenBy and CreatedAt; the repository allocates the new immutable ID
	// and the next VersionNumber inside one transaction and returns the
	// stored row. Concurrent freezes of the same agent serialize through
	// the (tenant_id, agent_id, version_number) unique index: a collided
	// insert retries with a fresh MAX read.
	Freeze(ctx context.Context, e *types.AgentVersionEntity) (*types.AgentVersionEntity, error)
	// GetByTenantAndID returns one frozen version addressed by its immutable
	// id, or nil when the id is unknown or belongs to another tenant —
	// cross-tenant reads fail closed to not-found, never data.
	GetByTenantAndID(ctx context.Context, tenantID uint64, versionID string) (*types.AgentVersionEntity, error)
	// ListByTenantAndAgent returns every frozen version of one agent inside
	// one tenant, ascending by version number.
	ListByTenantAndAgent(ctx context.Context, tenantID uint64, agentID string) ([]types.AgentVersionEntity, error)
}

type agentVersionRepository struct{ db *gorm.DB }

// NewAgentVersionRepository returns a GORM-backed implementation.
func NewAgentVersionRepository(db *gorm.DB) AgentVersionRepository {
	return &agentVersionRepository{db: db}
}

// Freeze validates the candidate, then allocates ID + VersionNumber and
// inserts inside a single transaction. The MAX read and the insert share the
// transaction; the unique index — not the MAX read — is the concurrency
// guard, so a collision under concurrency surfaces as a unique violation
// and retries rather than ever persisting a duplicate number.
func (r *agentVersionRepository) Freeze(ctx context.Context, e *types.AgentVersionEntity) (*types.AgentVersionEntity, error) {
	if e == nil || e.TenantID == 0 || strings.TrimSpace(e.AgentID) == "" || e.Snapshot == "" || e.SourceSHA256 == "" {
		return nil, fmt.Errorf("%w: tenant, agent, snapshot and source digest are required", ErrAgentVersionInvalid)
	}

	var lastErr error
	for attempt := 0; attempt < agentVersionFreezeAttempts; attempt++ {
		var stored *types.AgentVersionEntity
		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			// Copy the candidate: freeze never mutates the caller's entity.
			row := *e
			row.AgentID = strings.TrimSpace(row.AgentID)
			var maxNumber int
			if err := tx.Model(&types.AgentVersionEntity{}).
				Where("tenant_id = ? AND agent_id = ?", row.TenantID, row.AgentID).
				Select("COALESCE(MAX(version_number), 0)").
				Scan(&maxNumber).Error; err != nil {
				return err
			}
			row.ID = uuid.NewString()
			row.VersionNumber = maxNumber + 1
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			stored = &row
			return nil
		})
		if err == nil {
			return stored, nil
		}
		lastErr = err
		if !isUniqueViolation(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("freeze the agent version after %d attempts: %w", agentVersionFreezeAttempts, lastErr)
}

func (r *agentVersionRepository) GetByTenantAndID(
	ctx context.Context, tenantID uint64, versionID string,
) (*types.AgentVersionEntity, error) {
	var row types.AgentVersionEntity
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, strings.TrimSpace(versionID)).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *agentVersionRepository) ListByTenantAndAgent(
	ctx context.Context, tenantID uint64, agentID string,
) ([]types.AgentVersionEntity, error) {
	var rows []types.AgentVersionEntity
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND agent_id = ?", tenantID, strings.TrimSpace(agentID)).
		Order("version_number ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
