package repository

import (
	"context"
	"encoding/json"
	"errors"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CancelRunsByAgents atomically cancels the active tenant runs whose frozen
// coordinator snapshot names one of agentIDs. Malformed or non-coordinator
// snapshots are deliberately skipped by this best-effort governance scan.
func (s *AgentRunStore) CancelRunsByAgents(ctx context.Context, tenantID uint64, agentIDs []string, reason string) (int64, error) {
	if tenantID == 0 || len(agentIDs) == 0 {
		return 0, nil
	}
	wanted := make(map[string]struct{}, len(agentIDs))
	for _, id := range agentIDs {
		if id != "" {
			wanted[id] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return 0, nil
	}

	var canceled int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var candidates []agentRunRow
		if err := tx.Table("agent_runs").
			Where("tenant_id = ? AND status NOT IN ?", tenantID, []string{"succeeded", "failed", "canceled"}).
			Order("created_at ASC").Order("run_id ASC").Find(&candidates).Error; err != nil {
			return err
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
					"status":      "canceled",
					"wait_reason": reason,
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
		return nil
	})
	if err != nil {
		return 0, err
	}
	return canceled, nil
}
