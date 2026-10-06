package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
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

type PendingAgentSecurityCancellation struct {
	TenantID     uint64
	RevocationID string
	Kind         string
}

func (s *AgentRunStore) ListPendingRunCancellations(ctx context.Context, limit int) ([]PendingAgentSecurityCancellation, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	out := make([]PendingAgentSecurityCancellation, 0)
	var releases []types.AgentReleaseRevocationEntity
	if err := s.db.WithContext(ctx).Where("run_cancellation_state = ?", "pending").Order("created_at ASC").Limit(limit).Find(&releases).Error; err != nil {
		return nil, err
	}
	for _, r := range releases {
		out = append(out, PendingAgentSecurityCancellation{TenantID: r.TenantID, RevocationID: r.ID, Kind: AgentSecurityRevocationRelease})
	}
	remaining := limit - len(out)
	if remaining > 0 {
		var deps []types.AgentDependencyRevocationEntity
		if err := s.db.WithContext(ctx).Where("run_cancellation_state = ?", "pending").Order("created_at ASC").Limit(remaining).Find(&deps).Error; err != nil {
			return nil, err
		}
		for _, r := range deps {
			out = append(out, PendingAgentSecurityCancellation{TenantID: r.TenantID, RevocationID: r.ID, Kind: AgentSecurityRevocationDependency})
		}
	}
	return out, nil
}

// ReconcileRunCancellation retries one durable obligation. The tenant guard
// serializes replicas; the ledger row is rechecked after acquiring it.
func (s *AgentRunStore) ReconcileRunCancellation(ctx context.Context, tenantID uint64, revocationID string) (int64, error) {
	var canceled int64
	err := withTenantSecurityGuard(ctx, s.db, tenantID, func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "sqlite" {
			if err := tx.Table("tenants").Where("id = ?", tenantID).UpdateColumn("id", gorm.Expr("id")).Error; err != nil {
				return err
			}
		}
		ref := AgentSecurityRevocationRef{}
		var release types.AgentReleaseRevocationEntity
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, revocationID).Take(&release).Error
		if err == nil {
			ref = AgentSecurityRevocationRef{Kind: AgentSecurityRevocationRelease, ID: release.ID}
			if release.RunCancellationState != "pending" {
				canceled = release.CanceledRunCount
				return nil
			}
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			var dep types.AgentDependencyRevocationEntity
			err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, revocationID).Take(&dep).Error
			if err != nil {
				return err
			}
			ref = AgentSecurityRevocationRef{Kind: AgentSecurityRevocationDependency, ID: dep.ID}
			if dep.RunCancellationState != "pending" {
				canceled = dep.CanceledRunCount
				return nil
			}
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		releaseIDs := make(map[string]struct{})
		reason := "agent security revocation"
		cumulative := int64(0)
		if ref.Kind == AgentSecurityRevocationRelease {
			releaseIDs[release.ReleaseID] = struct{}{}
			reason = "agent security revocation: " + release.Reason
			cumulative = release.CanceledRunCount
		} else {
			var dep types.AgentDependencyRevocationEntity
			if err := tx.Where("tenant_id = ? AND id = ?", tenantID, revocationID).Take(&dep).Error; err != nil {
				return err
			}
			reason = "agent security revocation: " + dep.Reason
			cumulative = dep.CanceledRunCount
			var local []types.AgentReleaseEntity
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ?", tenantID).Order("id ASC").Find(&local).Error; err != nil {
				return err
			}
			for _, r := range local {
				if dependencyLockContains(r.DependencyLockJSON, &dep) {
					releaseIDs[r.ID] = struct{}{}
				}
			}
			var introduced []types.TenantIntroducedReleaseEntity
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ?", tenantID).Order("id ASC").Find(&introduced).Error; err != nil {
				return err
			}
			for _, r := range introduced {
				if dependencyLockContains(r.DependencyLockJSON, &dep) {
					releaseIDs[r.ID] = struct{}{}
				}
			}
		}
		var variants []types.AgentAdoptionVariantEntity
		if len(releaseIDs) > 0 {
			ids := make([]string, 0, len(releaseIDs))
			for id := range releaseIDs {
				ids = append(ids, id)
			}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND release_id IN ?", tenantID, ids).Order("id ASC").Find(&variants).Error; err != nil {
				return err
			}
		}
		pins := make(map[AgentSecurityRunPin]struct{})
		for _, v := range variants {
			if v.LocalAgentID != "" && v.LocalAgentVersionID != "" && v.ReleaseID != "" {
				pins[AgentSecurityRunPin{AgentID: v.LocalAgentID, LocalAgentVersionID: v.LocalAgentVersionID, ReleaseID: v.ReleaseID}] = struct{}{}
			}
		}
		if len(pins) > 0 {
			var clauses []string
			args := []any{tenantID, []string{"succeeded", "failed", "canceled"}}
			for p := range pins {
				clauses = append(clauses, "(security_agent_id = ? AND security_local_agent_version_id = ? AND security_release_id = ?)")
				args = append(args, p.AgentID, p.LocalAgentVersionID, p.ReleaseID)
			}
			var candidates []agentRunRow
			if err := tx.Where("tenant_id = ? AND status NOT IN ? AND ("+strings.Join(clauses, " OR ")+")", args...).Order("created_at ASC").Order("run_id ASC").Find(&candidates).Error; err != nil {
				return err
			}
			for _, candidate := range candidates {
				var run agentRunRow
				err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND run_id = ? AND status NOT IN ?", tenantID, candidate.RunID, []string{"succeeded", "failed", "canceled"}).Take(&run).Error
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				if err != nil {
					return err
				}
				res := tx.Table("agent_runs").Where("tenant_id = ? AND run_id = ? AND status NOT IN ?", tenantID, run.RunID, []string{"succeeded", "failed", "canceled"}).Updates(map[string]any{"status": "canceled", "wait_reason": agentSecurityRevocationWaitReason, "lease_owner": "", "lease_until": nil, "revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP")})
				if res.Error != nil {
					return res.Error
				}
				if res.RowsAffected == 0 {
					continue
				}
				payload, err := json.Marshal(map[string]string{"reason": reason})
				if err != nil {
					return err
				}
				key := agentruntime.RunKey{TenantID: tenantID, RunID: run.RunID}
				if err := appendRunEventLocked(tx, agentruntime.Fence{RunKey: key}, "cancellation_requested", string(payload)); err != nil {
					return err
				}
				if err := tx.Table("sessions").Where("tenant_id = ? AND id = ? AND active_agent_run_id = ?", tenantID, run.SessionID, run.RunID).Update("active_agent_run_id", nil).Error; err != nil {
					return err
				}
				canceled++
			}
		}
		var update *gorm.DB
		if ref.Kind == AgentSecurityRevocationRelease {
			update = tx.Model(&types.AgentReleaseRevocationEntity{}).Where("tenant_id = ? AND id = ? AND run_cancellation_state = 'pending'", tenantID, ref.ID).Updates(map[string]any{"canceled_run_count": gorm.Expr("canceled_run_count + ?", canceled), "run_cancellation_state": "complete"})
		} else {
			update = tx.Model(&types.AgentDependencyRevocationEntity{}).Where("tenant_id = ? AND id = ? AND run_cancellation_state = 'pending'", tenantID, ref.ID).Updates(map[string]any{"canceled_run_count": gorm.Expr("canceled_run_count + ?", canceled), "run_cancellation_state": "complete"})
		}
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return errors.New("pending agent security cancellation changed concurrently")
		}
		canceled += cumulative
		return nil
	})
	return canceled, err
}

// CancelRunsByAgents atomically cancels the active tenant runs whose frozen
// coordinator snapshot names one of agentIDs. Malformed or non-coordinator
// snapshots are deliberately skipped by this best-effort governance scan.
func (s *AgentRunStore) CancelRunsByAgents(ctx context.Context, tenantID uint64, agentIDs []string, reason string) (int64, error) {
	return s.cancelRunsByAgents(ctx, tenantID, agentIDs, nil, reason, nil)
}

// CancelRunsByAgentsForRevocation writes the cancellation count to its
// revocation ledger in the same transaction as run/event/session changes.
func (s *AgentRunStore) CancelRunsByAgentsForRevocation(ctx context.Context, tenantID uint64, agentIDs []string, reason string, ref AgentSecurityRevocationRef) (int64, error) {
	if ref.ID == "" || (ref.Kind != AgentSecurityRevocationRelease && ref.Kind != AgentSecurityRevocationDependency) {
		return 0, errors.New("valid agent security revocation reference is required")
	}
	return s.cancelRunsByAgents(ctx, tenantID, agentIDs, nil, reason, &ref)
}

type AgentSecurityRunPin struct{ AgentID, LocalAgentVersionID, ReleaseID string }

// CancelRunsBySecurityPinsForRevocation cancels only Runs whose immutable
// sidecars match an exact published Variant identity.
func (s *AgentRunStore) CancelRunsBySecurityPinsForRevocation(ctx context.Context, tenantID uint64, pins []AgentSecurityRunPin, reason string, ref AgentSecurityRevocationRef) (int64, error) {
	if ref.ID == "" || (ref.Kind != AgentSecurityRevocationRelease && ref.Kind != AgentSecurityRevocationDependency) {
		return 0, errors.New("valid agent security revocation reference is required")
	}
	return s.cancelRunsByAgents(ctx, tenantID, nil, pins, reason, &ref)
}

func (s *AgentRunStore) cancelRunsByAgents(ctx context.Context, tenantID uint64, agentIDs []string, pins []AgentSecurityRunPin, reason string, ref *AgentSecurityRevocationRef) (int64, error) {
	if tenantID == 0 {
		return 0, nil
	}
	wanted := make(map[string]struct{}, len(agentIDs))
	for _, id := range agentIDs {
		if id != "" {
			wanted[id] = struct{}{}
		}
	}
	pinSet := make(map[AgentSecurityRunPin]struct{}, len(pins))
	for _, pin := range pins {
		if pin.AgentID != "" && pin.LocalAgentVersionID != "" && pin.ReleaseID != "" {
			pinSet[pin] = struct{}{}
		}
	}
	if len(wanted) == 0 && len(pinSet) == 0 && ref == nil {
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
		if len(wanted) > 0 || len(pinSet) > 0 {
			query := tx.Table("agent_runs").Where("tenant_id = ? AND status NOT IN ?", tenantID, []string{"succeeded", "failed", "canceled"})
			if len(pinSet) > 0 {
				var clauses []string
				args := []any{}
				for pin := range pinSet {
					clauses = append(clauses, "(security_agent_id = ? AND security_local_agent_version_id = ? AND security_release_id = ?)")
					args = append(args, pin.AgentID, pin.LocalAgentVersionID, pin.ReleaseID)
				}
				query = query.Where("("+strings.Join(clauses, " OR ")+")", args...)
			}
			if err := query.
				Order("created_at ASC").Order("run_id ASC").Find(&candidates).Error; err != nil {
				return err
			}
		}

		for _, candidate := range candidates {
			legacyMatch := false
			if len(wanted) > 0 {
				var snapshot struct {
					AgentID string `json:"agent_id"`
				}
				if err := json.Unmarshal([]byte(candidate.Snapshot), &snapshot); err != nil {
					continue
				}
				_, legacyMatch = wanted[snapshot.AgentID]
			}
			_, exactMatch := pinSet[AgentSecurityRunPin{AgentID: candidate.SecurityAgentID, LocalAgentVersionID: candidate.SecurityLocalAgentVersionID, ReleaseID: candidate.SecurityReleaseID}]
			if !legacyMatch && !exactMatch {
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
				update = tx.Model(&types.AgentReleaseRevocationEntity{}).Where("tenant_id = ? AND id = ?", tenantID, ref.ID).Updates(map[string]any{"canceled_run_count": canceled, "run_cancellation_state": "complete"})
			case AgentSecurityRevocationDependency:
				update = tx.Model(&types.AgentDependencyRevocationEntity{}).Where("tenant_id = ? AND id = ?", tenantID, ref.ID).Updates(map[string]any{"canceled_run_count": canceled, "run_cancellation_state": "complete"})
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
