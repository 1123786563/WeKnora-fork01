package repository

import (
	"context"
	"encoding/json"
	"errors"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const agentSecurityRevocationWaitReason = "agent_security_revocation"

const (
	AgentSecurityRevocationRelease    = "release"
	AgentSecurityRevocationDependency = "dependency"
)

// AgentSecurityRevocationRef identifies the already committed revocation row
// whose canceled-run count must commit atomically with the run transitions.
type AgentSecurityRevocationRef struct {
	Kind string
	ID   string
}

// CancelRunsByAgents atomically cancels the active tenant runs whose frozen
// coordinator snapshot names one of agentIDs. Malformed or non-coordinator
// snapshots are deliberately skipped by this best-effort governance scan.
func (s *AgentRunStore) CancelRunsByAgents(ctx context.Context, tenantID uint64, agentIDs []string, reason string) (int64, error) {
	return s.cancelRunsByAgents(ctx, tenantID, agentIDs, reason, nil)
}

// CancelRunsByAgentsForRevocation writes the cancellation count to its
// revocation ledger in the same transaction as run/event/session changes.
func (s *AgentRunStore) CancelRunsByAgentsForRevocation(ctx context.Context, tenantID uint64, agentIDs []string, reason string, ref AgentSecurityRevocationRef) (int64, error) {
	if ref.ID == "" || (ref.Kind != AgentSecurityRevocationRelease && ref.Kind != AgentSecurityRevocationDependency) {
		return 0, errors.New("valid agent security revocation reference is required")
	}
	return s.cancelRunsByAgents(ctx, tenantID, agentIDs, reason, &ref)
}

func (s *AgentRunStore) cancelRunsByAgents(ctx context.Context, tenantID uint64, agentIDs []string, reason string, ref *AgentSecurityRevocationRef) (int64, error) {
	if tenantID == 0 {
		return 0, nil
	}
	wanted := make(map[string]struct{}, len(agentIDs))
	for _, id := range agentIDs {
		if id != "" {
			wanted[id] = struct{}{}
		}
	}
	if len(wanted) == 0 && ref == nil {
		return 0, nil
	}

	var canceled int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// SQLite starts deferred transactions. Reserve its single-writer slot
		// before the candidate read so a worker cannot promote a competing read
		// transaction after our scan and strand this batch with SQLITE_BUSY.
		if tx.Dialector.Name() == "sqlite" {
			if err := tx.Table("tenants").Where("id = ?", tenantID).
				UpdateColumn("id", gorm.Expr("id")).Error; err != nil {
				return err
			}
		}

		var candidates []agentRunRow
		if len(wanted) > 0 {
			if err := tx.Table("agent_runs").
				Where("tenant_id = ? AND status NOT IN ?", tenantID, []string{"succeeded", "failed", "canceled"}).
				Order("created_at ASC").Order("run_id ASC").Find(&candidates).Error; err != nil {
				return err
			}
		}

		for _, candidate := range candidates {
			var snapshot struct {
				AgentID string `json:"agent_id"`
			}
			if err := json.Unmarshal([]byte(candidate.Snapshot), &snapshot); err != nil {
				continue
			}
			if _, match := wanted[snapshot.AgentID]; !match {
				continue
			}

			key := agentruntime.RunKey{TenantID: tenantID, RunID: candidate.RunID}
			var run agentRunRow
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("tenant_id = ? AND run_id = ? AND status NOT IN ?", tenantID, candidate.RunID,
					[]string{"succeeded", "failed", "canceled"}).Take(&run).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			if err != nil {
				return err
			}

			updated := tx.Table("agent_runs").
				Where("tenant_id = ? AND run_id = ? AND status NOT IN ?", tenantID, run.RunID,
					[]string{"succeeded", "failed", "canceled"}).
				Updates(map[string]any{
					"status": "canceled",
					// The detailed reason lives in the append-only security ledger
					// and event payload. Keep this compact status column bounded.
					"wait_reason": agentSecurityRevocationWaitReason,
					"lease_owner": "",
					"lease_until": nil,
					"revision":    gorm.Expr("revision + 1"),
					"updated_at":  gorm.Expr("CURRENT_TIMESTAMP"),
				})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected == 0 {
				continue
			}

			payload, err := json.Marshal(map[string]string{"reason": reason})
			if err != nil {
				return err
			}
			if err := appendRunEventLocked(tx, agentruntime.Fence{RunKey: key}, "cancellation_requested", string(payload)); err != nil {
				return err
			}
			if err := tx.Table("sessions").
				Where("tenant_id = ? AND id = ? AND active_agent_run_id = ?", tenantID, run.SessionID, run.RunID).
				Update("active_agent_run_id", nil).Error; err != nil {
				return err
			}
			canceled++
		}
		if ref != nil {
			var update *gorm.DB
			switch ref.Kind {
			case AgentSecurityRevocationRelease:
				update = tx.Model(&types.AgentReleaseRevocationEntity{}).Where("tenant_id = ? AND id = ?", tenantID, ref.ID).UpdateColumn("canceled_run_count", canceled)
			case AgentSecurityRevocationDependency:
				update = tx.Model(&types.AgentDependencyRevocationEntity{}).Where("tenant_id = ? AND id = ?", tenantID, ref.ID).UpdateColumn("canceled_run_count", canceled)
			}
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return errors.New("agent security revocation row not found for canceled-run count")
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return canceled, nil
}
