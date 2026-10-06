package repository

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CraftStore persists Craft workspace bindings and delegation journals in the
// migrated business database. Workspace creation is a unique-binding race:
// the database decides which concurrent creator wins. Every delegation write
// is fenced against the live agent run row in the same transaction, never
// against a fence snapshot cached inside the task.
type CraftStore struct {
	db   *gorm.DB
	runs *AgentRunStore
}

var _ craft.Store = (*CraftStore)(nil)

// NewCraftStore constructs the craft.Store implementation backed by the
// migrated business database.
func NewCraftStore(db *gorm.DB) craft.Store {
	return &CraftStore{db: db, runs: NewAgentRunStore(db)}
}

type craftWorkspaceRow struct {
	ID                string    `gorm:"column:id"`
	TenantID          uint64    `gorm:"column:tenant_id"`
	SessionID         string    `gorm:"column:session_id"`
	OwnerID           string    `gorm:"column:owner_id"`
	SandboxID         string    `gorm:"column:sandbox_id"`
	Generation        string    `gorm:"column:generation"`
	OpenCodeSessionID string    `gorm:"column:oc_session_id"`
	RuntimeDigest     string    `gorm:"column:runtime_digest"`
	Revision          int64     `gorm:"column:revision"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
}

func (craftWorkspaceRow) TableName() string { return "craft_workspaces" }

func (r craftWorkspaceRow) view() craft.Workspace {
	return craft.Workspace{
		ID: r.ID,
		Scope: craft.Scope{
			TenantID: r.TenantID, UserID: r.OwnerID, SessionID: r.SessionID,
		},
		SandboxID:         r.SandboxID,
		Generation:        r.Generation,
		OpenCodeSessionID: r.OpenCodeSessionID,
		RuntimeDigest:     r.RuntimeDigest,
		Revision:          r.Revision,
	}
}

type craftDelegationRow struct {
	ID              string    `gorm:"column:id"`
	TenantID        uint64    `gorm:"column:tenant_id"`
	RunID           string    `gorm:"column:run_id"`
	ToolCallID      string    `gorm:"column:tool_call_id"`
	WorkspaceID     string    `gorm:"column:workspace_id"`
	PromptMessageID string    `gorm:"column:prompt_message_id"`
	RequestHash     string    `gorm:"column:request_hash"`
	TaskJSON        string    `gorm:"column:task_json"`
	Status          string    `gorm:"column:status"`
	ResultJSON      *string   `gorm:"column:result_json"`
	Revision        int64     `gorm:"column:revision"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (craftDelegationRow) TableName() string { return "craft_delegations" }

func (r craftDelegationRow) task() (craft.Task, error) {
	var task craft.Task
	if err := json.Unmarshal([]byte(r.TaskJSON), &task); err != nil {
		return craft.Task{}, fmt.Errorf("craft: decode delegation %s: %w", r.ID, err)
	}
	return task, nil
}

// craftMapRuntimeError maps recovery-program store errors at the assembly
// boundary: a lost lease is a superseded-worker conflict for Craft callers.
func craftMapRuntimeError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, agentruntime.ErrLeaseLost):
		return fmt.Errorf("%w: run fence is no longer live: %v", craft.ErrConflict, err)
	case errors.Is(err, agentruntime.ErrNotFound):
		return fmt.Errorf("%w: run not found: %v", craft.ErrNotFound, err)
	default:
		return err
	}
}

// GetWorkspace returns the single workspace bound to the scope's session.
// Another tenant's or session's binding is invisible; another user's binding
// exists but is forbidden — shared-session read ACLs stay in the permission
// services, this port only answers the execution owner.
func (s *CraftStore) GetWorkspace(ctx context.Context, scope craft.Scope) (craft.Workspace, error) {
	var row craftWorkspaceRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND session_id = ?", scope.TenantID, scope.SessionID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.Workspace{}, craft.ErrNotFound
	}
	if err != nil {
		return craft.Workspace{}, err
	}
	if row.OwnerID != scope.UserID {
		return craft.Workspace{}, fmt.Errorf("%w: workspace owned by %s", craft.ErrForbidden, row.OwnerID)
	}
	return row.view(), nil
}

// PutWorkspace writes the workspace with compare-and-swap revision semantics.
// expected == 0 is a first creation that stores revision 1: the unique
// (tenant_id, session_id) binding is claimed by exactly one of any number of
// racing creators, every loser gets ErrConflict. expected >= 1 updates only
// the row still carrying that revision, so zero never reaches the CAS path.
func (s *CraftStore) PutWorkspace(ctx context.Context, in craft.Workspace, expected int64) (craft.Workspace, error) {
	if in.Scope.TenantID == 0 || in.Scope.UserID == "" || in.Scope.SessionID == "" || expected < 0 {
		return craft.Workspace{}, fmt.Errorf("%w: incomplete workspace scope", craft.ErrInvalidInput)
	}
	var out craft.Workspace
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if expected == 0 {
			row := craftWorkspaceRow{
				TenantID: in.Scope.TenantID, SessionID: in.Scope.SessionID, OwnerID: in.Scope.UserID,
				SandboxID: in.SandboxID, Generation: in.Generation,
				OpenCodeSessionID: in.OpenCodeSessionID, RuntimeDigest: in.RuntimeDigest,
				Revision: 1,
			}
			if in.ID == "" {
				row.ID = uuid.NewString()
			} else {
				row.ID = in.ID
			}
			created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
			if created.Error != nil {
				return created.Error
			}
			if created.RowsAffected != 1 {
				return fmt.Errorf("%w: session %d/%s already has a workspace binding",
					craft.ErrConflict, in.Scope.TenantID, in.Scope.SessionID)
			}
			head := craftDraftHeadRow{
				WorkspaceID: row.ID, TenantID: row.TenantID, Revision: 0, State: string(craft.DraftHeadEmpty),
			}
			if err := tx.Create(&head).Error; err != nil {
				return err
			}
			origin := craftDraftOriginRow{
				WorkspaceID: row.ID, TenantID: row.TenantID, OriginRevision: 0, OriginState: string(craft.DraftHeadEmpty),
			}
			if err := tx.Create(&origin).Error; err != nil {
				return err
			}
			out = row.view()
			return nil
		}
		updated := tx.Model(&craftWorkspaceRow{}).
			Where("id = ? AND tenant_id = ? AND session_id = ? AND revision = ?",
				in.ID, in.Scope.TenantID, in.Scope.SessionID, expected).
			Updates(map[string]any{
				"owner_id":       in.Scope.UserID,
				"sandbox_id":     in.SandboxID,
				"generation":     in.Generation,
				"oc_session_id":  in.OpenCodeSessionID,
				"runtime_digest": in.RuntimeDigest,
				"revision":       gorm.Expr("revision + 1"),
				"updated_at":     gorm.Expr("CURRENT_TIMESTAMP"),
			})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			var exists int64
			count := tx.Model(&craftWorkspaceRow{}).
				Where("id = ? AND tenant_id = ? AND session_id = ?", in.ID, in.Scope.TenantID, in.Scope.SessionID).
				Count(&exists)
			if count.Error != nil {
				return count.Error
			}
			if exists == 0 {
				return fmt.Errorf("%w: workspace %s", craft.ErrNotFound, in.ID)
			}
			return fmt.Errorf("%w: workspace %s revision changed", craft.ErrConflict, in.ID)
		}
		var row craftWorkspaceRow
		if e := tx.Where("id = ? AND tenant_id = ?", in.ID, in.Scope.TenantID).Take(&row).Error; e != nil {
			return e
		}
		out = row.view()
		return nil
	})
	return out, err
}

func validateCraftTask(task craft.Task) error {
	if task.Scope.TenantID == 0 || task.Scope.UserID == "" || task.Scope.SessionID == "" ||
		task.ToolCallID == "" || task.WorkspaceID == "" || task.Prompt == "" || task.RequestHash == "" ||
		task.Fence.TenantID == 0 || task.Fence.RunID == "" || task.Fence.Owner == "" || task.Fence.Epoch <= 0 ||
		task.Fence.TenantID != task.Scope.TenantID {
		return fmt.Errorf("%w: incomplete delegation request", craft.ErrInvalidInput)
	}
	if task.SnapshotDigestVersion != craftSnapshotDigestVersion || len(task.SnapshotDigest) != 64 ||
		task.Fence.SnapshotDigestVersion != task.SnapshotDigestVersion || task.Fence.SnapshotDigest != task.SnapshotDigest {
		return fmt.Errorf("%w: delegation requires a fence-matched admitted snapshot identity", craft.ErrInvalidInput)
	}
	digestBytes, err := hex.DecodeString(task.SnapshotDigest)
	if err != nil || hex.EncodeToString(digestBytes) != task.SnapshotDigest {
		return fmt.Errorf("%w: malformed admitted snapshot identity", craft.ErrInvalidInput)
	}
	return nil
}

// sameDelegationRequest compares the caller-controlled request identity. The
// live owner and epoch may change during recovery, but the Run key and its
// immutable admission digest remain part of the delegation identity.
func sameDelegationRequest(a, b craft.Task) bool {
	if a.ToolCallID != b.ToolCallID || a.WorkspaceID != b.WorkspaceID || a.Prompt != b.Prompt ||
		a.RequestHash != b.RequestHash || !craft.SameScope(a.Scope, b.Scope) ||
		a.Fence.TenantID != b.Fence.TenantID || a.Fence.RunID != b.Fence.RunID ||
		a.Fence.SnapshotDigestVersion != b.Fence.SnapshotDigestVersion || a.Fence.SnapshotDigest != b.Fence.SnapshotDigest ||
		a.SnapshotDigestVersion != b.SnapshotDigestVersion || a.SnapshotDigest != b.SnapshotDigest ||
		len(a.Inputs) != len(b.Inputs) || len(a.SkillDigests) != len(b.SkillDigests) ||
		!a.Deadline.Equal(b.Deadline) {
		return false
	}
	for i := range a.Inputs {
		if a.Inputs[i] != b.Inputs[i] {
			return false
		}
	}
	for i := range a.SkillDigests {
		if a.SkillDigests[i] != b.SkillDigests[i] {
			return false
		}
	}
	return true
}

func sameCraftResult(a, b craft.Result) bool {
	if a.TaskID != b.TaskID || a.Status != b.Status || a.Summary != b.Summary ||
		len(a.Files) != len(b.Files) || len(a.Checks) != len(b.Checks) {
		return false
	}
	for i := range a.Files {
		if a.Files[i] != b.Files[i] {
			return false
		}
	}
	for i := range a.Checks {
		if a.Checks[i] != b.Checks[i] {
			return false
		}
	}
	return true
}

// PrepareTask persists the delegation request before the sub-execution is
// dispatched. The real run row is locked with the live fence and the session
// ownership is re-checked in the same transaction; the tool call must exist
// and still be active. Repeating the identical call returns the stored task
// with the original prompt message id; the same call with different arguments
// is a conflict.
func (s *CraftStore) PrepareTask(ctx context.Context, in craft.Task) (craft.Task, error) {
	if err := validateCraftTask(in); err != nil {
		return craft.Task{}, err
	}
	var out craft.Task
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := s.runs.lockToolRun(tx, in.Fence); e != nil {
			return craftMapRuntimeError(e)
		}
		var run agentRunRow
		if e := runScope(tx, in.Fence.RunKey).Take(&run).Error; e != nil {
			return e
		}
		if run.SessionID != in.Scope.SessionID || run.OwnerID != in.Scope.UserID {
			return fmt.Errorf("%w: delegation scope does not match run %s", craft.ErrForbidden, in.Fence.RunID)
		}
		if !validCraftAdmittedSnapshot(run) ||
			in.SnapshotDigestVersion != run.SnapshotDigestVersion || in.SnapshotDigest != run.SnapshotDigest ||
			in.Fence.SnapshotDigestVersion != run.SnapshotDigestVersion || in.Fence.SnapshotDigest != run.SnapshotDigest {
			return fmt.Errorf("%w: delegation snapshot identity does not match the admitted Run", craft.ErrConflict)
		}
		var ws craftWorkspaceRow
		e := tx.Where("id = ? AND tenant_id = ?", in.WorkspaceID, in.Fence.TenantID).Take(&ws).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: workspace %s", craft.ErrNotFound, in.WorkspaceID)
		}
		if e != nil {
			return e
		}
		if ws.SessionID != run.SessionID || ws.OwnerID != run.OwnerID {
			return fmt.Errorf("%w: workspace %s bound to another session", craft.ErrForbidden, ws.ID)
		}
		var call agentToolCallRow
		e = toolCallScope(tx, in.Fence.RunKey, in.ToolCallID).Take(&call).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: tool call %s", craft.ErrNotFound, in.ToolCallID)
		}
		if e != nil {
			return e
		}
		switch call.Status {
		case agentruntime.ToolStatusPlanned, agentruntime.ToolStatusDispatching:
		default:
			return fmt.Errorf("%w: tool call %s is %s", craft.ErrConflict, in.ToolCallID, call.Status)
		}

		// Preserve idempotent reuse/observation for a delegation that was
		// already prepared before Stop. Only a fresh row is subject to the
		// StopIntent fence below.
		var existing craftDelegationRow
		existingErr := tx.Where("tenant_id = ? AND run_id = ? AND tool_call_id = ?",
			in.Fence.TenantID, in.Fence.RunID, in.ToolCallID).Take(&existing).Error
		if existingErr == nil {
			stored, err := existing.task()
			if err != nil {
				return err
			}
			if !sameDelegationRequest(stored, in) {
				return fmt.Errorf("%w: tool call %s already prepared with a different request",
					craft.ErrConflict, in.ToolCallID)
			}
			out = stored
			return nil
		}
		if !errors.Is(existingErr, gorm.ErrRecordNotFound) {
			return existingErr
		}

		_, intentErr := getStopIntent(tx, in.Fence.TenantID, in.Scope.SessionID, in.Fence.RunID)
		if intentErr == nil {
			return fmt.Errorf("%w: Run %s has a durable stop intent", craft.ErrConflict, in.Fence.RunID)
		}
		if !errors.Is(intentErr, craft.ErrNotFound) {
			return intentErr // fail closed: only typed absence permits fresh preparation
		}

		if in.ID == "" {
			in.ID = uuid.NewString()
		}
		if in.PromptMessageID == "" {
			in.PromptMessageID = uuid.NewString()
		}
		encoded, err := json.Marshal(in)
		if err != nil {
			return err
		}
		row := craftDelegationRow{
			ID: in.ID, TenantID: in.Fence.TenantID, RunID: in.Fence.RunID, ToolCallID: in.ToolCallID,
			WorkspaceID: in.WorkspaceID, PromptMessageID: in.PromptMessageID, RequestHash: in.RequestHash,
			TaskJSON: string(encoded), Status: "prepared",
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 1 {
			out = in
			return nil
		}
		// Lost a race for the (tenant, run, tool call) identity: only the
		// identical request may adopt the stored delegation.
		existing = craftDelegationRow{}
		e = tx.Where("tenant_id = ? AND run_id = ? AND tool_call_id = ?",
			in.Fence.TenantID, in.Fence.RunID, in.ToolCallID).Take(&existing).Error
		if e != nil {
			return e
		}
		stored, err := existing.task()
		if err != nil {
			return err
		}
		if !sameDelegationRequest(stored, in) {
			return fmt.Errorf("%w: tool call %s already prepared with a different request",
				craft.ErrConflict, in.ToolCallID)
		}
		out = stored
		return nil
	})
	return out, err
}

// authorizeCraftDelegation answers whether the scope may read one delegation
// row: cross-tenant or cross-session rows do not exist for the caller,
// another user's binding is visible but forbidden.
func authorizeCraftDelegation(db *gorm.DB, row craftDelegationRow, scope craft.Scope) error {
	if row.TenantID != scope.TenantID {
		return craft.ErrNotFound
	}
	var ws craftWorkspaceRow
	err := db.Where("id = ?", row.WorkspaceID).Take(&ws).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.ErrNotFound
	}
	if err != nil {
		return err
	}
	if ws.SessionID != scope.SessionID {
		return craft.ErrNotFound
	}
	if ws.OwnerID != scope.UserID {
		return fmt.Errorf("%w: workspace owned by %s", craft.ErrForbidden, ws.OwnerID)
	}
	return nil
}

// GetTask returns the durable delegation request in the requesting scope.
func (s *CraftStore) GetTask(ctx context.Context, scope craft.Scope, taskID string) (craft.Task, error) {
	var row craftDelegationRow
	err := s.db.WithContext(ctx).Where("id = ?", taskID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.Task{}, craft.ErrNotFound
	}
	if err != nil {
		return craft.Task{}, err
	}
	if e := authorizeCraftDelegation(s.db.WithContext(ctx), row, scope); e != nil {
		return craft.Task{}, e
	}
	return row.task()
}

// SaveResult records the terminal delegation outcome. The fence is validated
// against the live agent_runs row (owner, epoch, lease, status) together with
// the session, workspace and tool call in one transaction — never against the
// fence cached inside the task. Replaying the identical result is idempotent;
// a different result for a finished delegation is rejected.
func (s *CraftStore) SaveResult(ctx context.Context, fence agentruntime.Fence, result craft.Result) error {
	switch result.Status {
	case "succeeded", "failed", "canceled", "unknown":
	default:
		return fmt.Errorf("%w: result status %q", craft.ErrInvalidInput, result.Status)
	}
	if result.TaskID == "" {
		return fmt.Errorf("%w: missing task id", craft.ErrInvalidInput)
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := s.runs.lockToolRun(tx, fence); e != nil {
			return craftMapRuntimeError(e)
		}
		var row craftDelegationRow
		err := tx.Where("id = ?", result.TaskID).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: delegation %s", craft.ErrNotFound, result.TaskID)
		}
		if err != nil {
			return err
		}
		if row.TenantID != fence.TenantID || row.RunID != fence.RunID {
			return fmt.Errorf("%w: delegation %s belongs to another run", craft.ErrForbidden, row.ID)
		}
		var run agentRunRow
		if e := runScope(tx, fence.RunKey).Take(&run).Error; e != nil {
			return e
		}
		var calls int64
		e := tx.Model(&agentToolCallRow{}).
			Where("tenant_id = ? AND run_id = ? AND call_id = ?", fence.TenantID, fence.RunID, row.ToolCallID).
			Count(&calls).Error
		if e != nil {
			return e
		}
		if calls != 1 {
			return fmt.Errorf("%w: tool call %s", craft.ErrNotFound, row.ToolCallID)
		}
		var ws craftWorkspaceRow
		e = tx.Where("id = ?", row.WorkspaceID).Take(&ws).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return craft.ErrNotFound
		}
		if e != nil {
			return e
		}
		if ws.SessionID != run.SessionID || ws.OwnerID != run.OwnerID {
			return fmt.Errorf("%w: workspace %s no longer binds the run session", craft.ErrForbidden, ws.ID)
		}
		if row.ResultJSON != nil {
			var stored craft.Result
			if e := json.Unmarshal([]byte(*row.ResultJSON), &stored); e != nil {
				return fmt.Errorf("craft: decode result %s: %w", row.ID, e)
			}
			if sameCraftResult(stored, result) {
				return nil
			}
			return fmt.Errorf("%w: delegation %s already has a different result", craft.ErrConflict, row.ID)
		}
		encoded, e := json.Marshal(result)
		if e != nil {
			return e
		}
		updated := tx.Model(&craftDelegationRow{}).
			Where("id = ? AND revision = ?", row.ID, row.Revision).
			Updates(map[string]any{
				"result_json": string(encoded),
				"status":      result.Status,
				"revision":    gorm.Expr("revision + 1"),
				"updated_at":  gorm.Expr("CURRENT_TIMESTAMP"),
			})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return fmt.Errorf("%w: delegation %s changed concurrently", craft.ErrConflict, row.ID)
		}
		return nil
	})
}

// GetResult returns the saved outcome of one delegation in the requesting
// scope. A delegation without a result yet answers ErrNotFound.
func (s *CraftStore) GetResult(ctx context.Context, scope craft.Scope, taskID string) (craft.Result, error) {
	var row craftDelegationRow
	err := s.db.WithContext(ctx).Where("id = ?", taskID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.Result{}, craft.ErrNotFound
	}
	if err != nil {
		return craft.Result{}, err
	}
	if e := authorizeCraftDelegation(s.db.WithContext(ctx), row, scope); e != nil {
		return craft.Result{}, e
	}
	if row.ResultJSON == nil {
		return craft.Result{}, fmt.Errorf("%w: delegation %s has no result yet", craft.ErrNotFound, row.ID)
	}
	var result craft.Result
	if e := json.Unmarshal([]byte(*row.ResultJSON), &result); e != nil {
		return craft.Result{}, fmt.Errorf("craft: decode result %s: %w", row.ID, e)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// T16 (#134): the durable Workspace writer lease.
// ---------------------------------------------------------------------------

// craftWriterLeaseRow is the one-row-per-Workspace writer lease. The
// workspace_id primary key IS the compare-and-swap identity: two concurrent
// writers race a single insert/update on it and the database decides.
type craftWriterLeaseRow struct {
	WorkspaceID string    `gorm:"column:workspace_id;primaryKey"`
	TenantID    uint64    `gorm:"column:tenant_id"`
	SessionID   string    `gorm:"column:session_id"`
	RunID       string    `gorm:"column:run_id"`
	Revision    int64     `gorm:"column:revision"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (craftWriterLeaseRow) TableName() string { return "craft_workspace_writer_leases" }

func (r craftWriterLeaseRow) view() *craft.WriterLease {
	return &craft.WriterLease{
		WorkspaceID: r.WorkspaceID, TaskID: r.SessionID, RunID: r.RunID, Revision: r.Revision,
	}
}

func craftWriterAcquired(row craftWriterLeaseRow) craft.WriterAcquisition {
	return craft.WriterAcquisition{
		Outcome: craft.WriterAcquireOutcome{WorkspaceID: row.WorkspaceID, Status: craft.WriterAcquired},
		Lease:   row.view(),
	}
}

func craftWriterConflict(row craftWriterLeaseRow) craft.WriterAcquisition {
	return craft.WriterAcquisition{
		Outcome: craft.WriterAcquireOutcome{WorkspaceID: row.WorkspaceID, Status: craft.WriterConflict},
		Holder:  row.view(),
	}
}

// observeCraftWriterRun reads the authoritative facts of one Run inside the
// caller's transaction: the durable status and the counts of writers that
// may still write through the workspace. A missing run row is an UNOBSERVED
// (unknown) outcome — reported as facts, never as an error, because
// "unknown" is a decision input, not a failure.
func observeCraftWriterRun(tx *gorm.DB, tenantID uint64, runID string) (craft.WriterRunFacts, error) {
	var run agentRunRow
	err := runScope(tx, agentruntime.RunKey{TenantID: tenantID, RunID: runID}).Take(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.WriterRunFacts{Observed: false}, nil
	}
	if err != nil {
		return craft.WriterRunFacts{}, err
	}
	facts := craft.WriterRunFacts{Observed: true, Status: run.Status}
	if err := tx.Model(&agentToolCallRow{}).
		Where("tenant_id = ? AND run_id = ? AND status NOT IN ('succeeded','failed')", tenantID, runID).
		Count(&facts.PendingToolWriters).Error; err != nil {
		return craft.WriterRunFacts{}, err
	}
	if err := tx.Model(&craftDelegationRow{}).
		Where("tenant_id = ? AND run_id = ? AND status NOT IN ('succeeded','failed','canceled')", tenantID, runID).
		Count(&facts.PendingDelegations).Error; err != nil {
		return craft.WriterRunFacts{}, err
	}
	return facts, nil
}

// lockCraftWriterWorkspace locks and authorizes the leased workspace row in
// the caller's transaction: cross-tenant or cross-session workspaces do not
// exist for the caller, another user's binding is visible but forbidden.
func lockCraftWriterWorkspace(tx *gorm.DB, scope craft.Scope, workspaceID string) (craftWorkspaceRow, error) {
	var ws craftWorkspaceRow
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND tenant_id = ?", workspaceID, scope.TenantID).Take(&ws).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craftWorkspaceRow{}, fmt.Errorf("%w: workspace %s", craft.ErrNotFound, workspaceID)
	}
	if err != nil {
		return craftWorkspaceRow{}, err
	}
	if ws.SessionID != scope.SessionID {
		// Cross-session workspaces are invisible on this surface, matching
		// GetWriterLease/GetWorkspace: the exported store seam must not leak
		// the binding session's existence.
		return craftWorkspaceRow{}, fmt.Errorf("%w: workspace %s", craft.ErrNotFound, workspaceID)
	}
	if ws.OwnerID != scope.UserID {
		return craftWorkspaceRow{}, fmt.Errorf("%w: workspace owned by %s", craft.ErrForbidden, ws.OwnerID)
	}
	return ws, nil
}

// craftWriterLeaseFenceRevision reads the draft-head revision a new writer
// must be fenced against. A workspace without a trustworthy head refuses
// admission fail-closed.
func craftWriterLeaseFenceRevision(tx *gorm.DB, scope craft.Scope, workspaceID string) (int64, error) {
	var head craftDraftHeadRow
	err := tx.Where("workspace_id = ? AND tenant_id = ?", workspaceID, scope.TenantID).Take(&head).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, craft.ErrDraftHeadUnresolved
	}
	if err != nil {
		return 0, err
	}
	return head.Revision, nil
}

// AcquireWriterLease admits at most one writing Run per Workspace through
// transactional compare-and-swap on the unique lease row. The winner's lease
// binds the Task (session), the Workspace, the Run and the draft-head
// revision at acquisition; a same-Run retry is idempotent; a loser receives
// a STABLE conflict naming the holder; and a previous holder is superseded
// only by the authoritative-recovery verdict re-verified from the durable
// run state inside this transaction.
func (s *CraftStore) AcquireWriterLease(ctx context.Context, scope craft.Scope, workspaceID, runID string) (craft.WriterAcquisition, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" || workspaceID == "" || runID == "" {
		return craft.WriterAcquisition{}, fmt.Errorf("%w: incomplete writer lease request", craft.ErrInvalidInput)
	}
	var out craft.WriterAcquisition
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockCraftWriterWorkspace(tx, scope, workspaceID); err != nil {
			return err
		}
		revision, err := craftWriterLeaseFenceRevision(tx, scope, workspaceID)
		if err != nil {
			return err
		}
		readLease := func() (craftWriterLeaseRow, error) {
			var row craftWriterLeaseRow
			e := tx.Where("workspace_id = ? AND tenant_id = ?", workspaceID, scope.TenantID).Take(&row).Error
			if e != nil {
				return craftWriterLeaseRow{}, e
			}
			if row.SessionID != scope.SessionID {
				return craftWriterLeaseRow{}, fmt.Errorf(
					"%w: lease of workspace %s names session %s, not %s", craft.ErrInvalidInput, workspaceID, row.SessionID, scope.SessionID)
			}
			return row, nil
		}
		// takeOver resolves the request against the CURRENT holder row:
		// same-Run is the idempotent winner, an authoritative-recovery
		// verdict supersedes via CAS, and anything else stays a stable
		// conflict naming the holder.
		var takeOver func(current craftWriterLeaseRow) error
		// insertLease claims the empty workspace through the guarded
		// insert; losing the unique-row race defers to the durable
		// winner's takeOver.
		var insertLease func() error
		takeOver = func(current craftWriterLeaseRow) error {
			if current.RunID == runID {
				out = craftWriterAcquired(current)
				return nil
			}
			facts, ferr := observeCraftWriterRun(tx, scope.TenantID, current.RunID)
			if ferr != nil {
				return ferr
			}
			if !craft.WriterLeaseTakeover(current.view(), facts) {
				out = craftWriterConflict(current)
				return nil
			}
			updated := tx.Model(&craftWriterLeaseRow{}).
				Where("workspace_id = ? AND tenant_id = ? AND run_id = ? AND revision = ?",
					workspaceID, scope.TenantID, current.RunID, current.Revision).
				Updates(map[string]any{
					"session_id": scope.SessionID, "run_id": runID, "revision": revision,
					"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
				})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				after, aerr := readLease()
				if errors.Is(aerr, gorm.ErrRecordNotFound) {
					// The holder row vanished under the CAS (a concurrent
					// release won the race — release and takeover share the
					// same eligibility rule, so they genuinely contend): the
					// fence is gone, and this run claims the now-empty
					// workspace through the insert path instead of bubbling
					// a raw not-found that StartRun would misreport as
					// "unknown, fence retained".
					return insertLease()
				}
				if aerr != nil {
					return aerr
				}
				return takeOver(after)
			}
			won, werr := readLease()
			if werr != nil {
				return werr
			}
			out = craftWriterAcquired(won)
			return nil
		}
		insertLease = func() error {
			row := craftWriterLeaseRow{
				WorkspaceID: workspaceID, TenantID: scope.TenantID,
				SessionID: scope.SessionID, RunID: runID, Revision: revision,
			}
			created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
			if created.Error != nil {
				return created.Error
			}
			if created.RowsAffected == 1 {
				out = craftWriterAcquired(row)
				return nil
			}
			// Lost the unique-row race: the durable winner is the holder.
			winner, werr := readLease()
			if werr != nil {
				return werr
			}
			return takeOver(winner)
		}

		var leaseRow craftWriterLeaseRow
		err = tx.Where("workspace_id = ? AND tenant_id = ?", workspaceID, scope.TenantID).Take(&leaseRow).Error
		switch {
		case err == nil:
			if leaseRow.SessionID != scope.SessionID {
				return fmt.Errorf(
					"%w: lease of workspace %s names session %s, not %s", craft.ErrInvalidInput, workspaceID, leaseRow.SessionID, scope.SessionID)
			}
			return takeOver(leaseRow)
		case errors.Is(err, gorm.ErrRecordNotFound):
			return insertLease()
		default:
			return err
		}
	})
	if err != nil {
		return craft.WriterAcquisition{}, err
	}
	if err := out.Outcome.Validate(); err != nil {
		return craft.WriterAcquisition{}, err
	}
	return out, nil
}

// ReleaseWriterLease releases the Workspace writer lease ONLY after a
// verified outcome: the named basis must not be unknown, the releasing Run
// must be the recorded holder, and the holder's durable run state is
// re-verified as terminal with zero unfinished writers inside the releasing
// transaction. Every refusal leaves the fence intact.
func (s *CraftStore) ReleaseWriterLease(ctx context.Context, scope craft.Scope, workspaceID, runID, basis string) error {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" || workspaceID == "" || runID == "" {
		return fmt.Errorf("%w: incomplete writer lease release", craft.ErrInvalidInput)
	}
	switch basis {
	case craft.WriterReleaseVerifiedCompletion, craft.WriterReleaseConfirmedStop, craft.WriterReleaseAuthoritativeRecovery:
	case craft.WriterReleaseUnknown:
		return fmt.Errorf("%w: unknown outcome retains the writer fence of run %s", craft.ErrBusy, runID)
	default:
		return fmt.Errorf("%w: unknown release basis %q", craft.ErrInvalidInput, basis)
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the workspace BEFORE the lease row: this carries the
		// workspace ACL (session binding + OwnerID) on EVERY path — a
		// same-session non-owner must never release another writer's
		// fence — and serializes Release with AcquireWriterLease under the
		// same workspace→lease lock order (AB-BA avoidance). It also makes
		// the lease-row delete below strictly ordered against any
		// concurrent Acquire's read/CAS of that row.
		if _, err := lockCraftWriterWorkspace(tx, scope, workspaceID); err != nil {
			return err
		}
		var leaseRow craftWriterLeaseRow
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("workspace_id = ? AND tenant_id = ?", workspaceID, scope.TenantID).Take(&leaseRow).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: workspace %s holds no writer lease", craft.ErrNotFound, workspaceID)
		}
		if err != nil {
			return err
		}
		if leaseRow.SessionID != scope.SessionID || leaseRow.RunID != runID {
			return fmt.Errorf("%w: the lease of workspace %s is held by run %s, not run %s",
				craft.ErrForbidden, workspaceID, leaseRow.RunID, runID)
		}
		facts, err := observeCraftWriterRun(tx, scope.TenantID, runID)
		if err != nil {
			return err
		}
		if !craft.WriterLeaseReleasable(facts) {
			return fmt.Errorf(
				"%w: run %s is not a verified terminal writer (status %q, pending tools %d, pending delegations %d); the fence stays",
				craft.ErrBusy, runID, facts.Status, facts.PendingToolWriters, facts.PendingDelegations)
		}
		deleted := tx.Where("workspace_id = ? AND tenant_id = ? AND run_id = ?", workspaceID, scope.TenantID, runID).
			Delete(&craftWriterLeaseRow{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return fmt.Errorf("%w: lease of run %s changed concurrently", craft.ErrConflict, runID)
		}
		return nil
	})
}

// GetWriterLease projects the durable lease row without ever taking it —
// the read path of status surfaces; a workspace without a lease answers
// ErrNotFound.
func (s *CraftStore) GetWriterLease(ctx context.Context, scope craft.Scope, workspaceID string) (*craft.WriterLease, error) {
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" || workspaceID == "" {
		return nil, fmt.Errorf("%w: incomplete writer lease read", craft.ErrInvalidInput)
	}
	// Keep the ACL shape of GetWorkspace on EVERY answer — the hit path
	// too, not just the empty one: the workspace row decides cross-session
	// invisibility (NotFound) and non-owner refusal (Forbidden) before any
	// lease fact (Task/Run/revision) is projected. This method is an
	// exported seam, so it must not rely on callers filtering first.
	var ws craftWorkspaceRow
	we := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", workspaceID, scope.TenantID).Take(&ws).Error
	if errors.Is(we, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w: workspace %s", craft.ErrNotFound, workspaceID)
	}
	if we != nil {
		return nil, we
	}
	if ws.SessionID != scope.SessionID {
		return nil, fmt.Errorf("%w: workspace %s", craft.ErrNotFound, workspaceID)
	}
	if ws.OwnerID != scope.UserID {
		return nil, fmt.Errorf("%w: workspace owned by %s", craft.ErrForbidden, ws.OwnerID)
	}
	var leaseRow craftWriterLeaseRow
	err := s.db.WithContext(ctx).Where("workspace_id = ? AND tenant_id = ?", workspaceID, scope.TenantID).Take(&leaseRow).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w: workspace %s holds no writer lease", craft.ErrNotFound, workspaceID)
	}
	if err != nil {
		return nil, err
	}
	return leaseRow.view(), nil
}
