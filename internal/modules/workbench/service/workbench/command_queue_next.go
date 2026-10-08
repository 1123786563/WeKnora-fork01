package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	agentruntime "github.com/Tencent/WeKnora/internal/agentruntime"
	workbench "github.com/Tencent/WeKnora/internal/modules/workbench"
	"gorm.io/gorm"
)

// RunRestartPort admits the next run of a task with an explicitly queued
// instruction (T07 queue-next / stop-restart). It is a port so deployments
// without an admission coordinator fail closed instead of improvising a
// second admission path.
type RunRestartPort interface {
	Restart(ctx context.Context, tenantID uint64, ownerID, runID, text, requestID string, expectedRevision int64) (string, error)
}

// GormRunRestartPort re-admits through the same AdmissionCoordinator that
// created the parent run: same session, agent, target, workspace and budget
// ceiling are read from the parent's immutable admission snapshot; only the
// text is the queued instruction. The engine-side after-input drain
// (admitAfterFollowUps) does not apply to workbench-admitted runs — their
// snapshot is the admission map, not a graph DurableRunSnapshot — so the
// command surface owns the follow-up admission itself.
type GormRunRestartPort struct {
	db        *gorm.DB
	admission *AdmissionCoordinator
}

func NewGormRunRestartPort(db *gorm.DB, admission *AdmissionCoordinator) *GormRunRestartPort {
	return &GormRunRestartPort{db: db, admission: admission}
}

type restartRunRow struct {
	SessionID, OwnerID, TargetID, Status string
	Revision                             int64
	Snapshot                             string
}

func (restartRunRow) TableName() string { return "agent_runs" }

func (p *GormRunRestartPort) Restart(ctx context.Context, tenantID uint64, ownerID, runID, text, requestID string, expectedRevision int64) (string, error) {
	if p == nil || p.db == nil || p.admission == nil {
		return "", ErrCapabilityUnavailable
	}
	if strings.TrimSpace(text) == "" || strings.TrimSpace(requestID) == "" {
		return "", workbench.ErrCommandActionMismatch
	}
	var row restartRunRow
	err := p.db.WithContext(ctx).
		Where("tenant_id = ? AND run_id = ?", tenantID, strings.TrimSpace(runID)).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", agentruntime.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	// A foreign run id is indistinguishable from a missing one: uniform 404.
	if row.OwnerID != strings.TrimSpace(ownerID) {
		return "", agentruntime.ErrNotFound
	}
	// Single writer (spec: one task permits at most one write run): while the
	// parent is active the next run cannot be admitted — the honest answer is
	// a conflict, never a silent queue.
	if row.Status != "succeeded" && row.Status != "failed" && row.Status != "canceled" {
		return "", agentruntime.ErrConflict
	}
	// The revision read-check is the "real revision" contract: a stale view
	// must never admit a restart on top of facts it has not observed. The
	// parent row itself is not mutated here — the follow-up admission is the
	// write, fenced separately by the session slot and the request id.
	if row.Revision != expectedRevision {
		return "", agentruntime.ErrConflict
	}
	var parent struct {
		AgentID      string `json:"agent_id"`
		TargetID     string `json:"target_id"`
		WorkspaceRef string `json:"workspace_ref"`
		SpaceID      string `json:"space_id"`
		BudgetUpper  int64  `json:"budget_upper"`
	}
	if err := json.Unmarshal([]byte(row.Snapshot), &parent); err != nil {
		// Not a coordinator-admitted run: fail closed instead of guessing.
		return "", agentruntime.ErrConflict
	}
	// A graph/tRPC DurableRunSnapshot (no agent_id key) unmarshals cleanly into
	// the struct above with every field zero-valued — json cannot distinguish
	// "absent" from "empty", so success here proves nothing. AdmissionCoordinator
	// .Start does not validate AgentID, and Restart inherits SessionID/owner from
	// the row, so without this guard a terminal chat run of the same owner would
	// admit a garbage follow-up run with an empty agent_id into a real session
	// slot. Unknown schema fails closed (mobile-module-seams §5.3).
	if strings.TrimSpace(parent.AgentID) == "" {
		return "", agentruntime.ErrConflict
	}
	run, err := p.admission.Start(ctx, StartInput{
		SessionID: row.SessionID, AgentID: parent.AgentID, TargetID: parent.TargetID,
		WorkspaceRef: parent.WorkspaceRef, SpaceID: parent.SpaceID,
		RequestID: strings.TrimSpace(requestID), Text: strings.TrimSpace(text), BudgetUpper: parent.BudgetUpper,
	})
	if err != nil {
		return "", err
	}
	return run.Key.RunID, nil
}
