package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
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
	return nil
}

// sameDelegationRequest compares the caller-controlled request identity: the
// fence contributes only its run key because a recovered worker replays the
// same logical call under a new owner and epoch.
func sameDelegationRequest(a, b craft.Task) bool {
	if a.ToolCallID != b.ToolCallID || a.WorkspaceID != b.WorkspaceID || a.Prompt != b.Prompt ||
		a.RequestHash != b.RequestHash || !craft.SameScope(a.Scope, b.Scope) ||
		a.Fence.TenantID != b.Fence.TenantID || a.Fence.RunID != b.Fence.RunID ||
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
		var existing craftDelegationRow
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
