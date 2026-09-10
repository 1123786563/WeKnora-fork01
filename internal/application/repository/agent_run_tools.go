package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"gorm.io/gorm"
)

var _ agentruntime.ToolJournal = (*AgentRunStore)(nil)
var _ agentruntime.ToolResultReader = (*AgentRunStore)(nil)

type agentToolCallRow struct {
	TenantID                                   uint64
	RunID, CallID                              string
	CallSeq                                    int64
	ToolName, ToolIdentity, ArgsHash, Args     string
	RecoveryPolicy, IdempotencyKey             string
	IdempotencyExpiresAt                       *time.Time
	Status, UnknownReason, OutputFiles, Source string
	Result                                     *string
	CreatedAt, UpdatedAt                       time.Time
	PlanVersion, ApprovedPlanVersion           int64
	PlanHistory                                string
}

func (agentToolCallRow) TableName() string { return "agent_tool_calls" }

type agentToolAttemptRow struct {
	TenantID                 uint64
	RunID, CallID            string
	Attempt                  int
	Epoch                    int64
	Status, ErrorMessage     string
	DispatchedAt, FinishedAt *time.Time
	CreatedAt                time.Time
}

func (agentToolAttemptRow) TableName() string { return "agent_tool_attempts" }

func toolCallScope(tx *gorm.DB, key agentruntime.RunKey, callID string) *gorm.DB {
	return tx.Where("tenant_id = ? AND run_id = ? AND call_id = ?", key.TenantID, key.RunID, callID)
}

// LoadToolResult returns the committed result for one logical call under the fence scope.
func (s *AgentRunStore) LoadToolResult(ctx context.Context, fence agentruntime.Fence, callID string) (agentruntime.StoredToolResult, error) {
	if fence.TenantID == 0 || fence.RunID == "" || fence.Owner == "" || fence.Epoch <= 0 {
		return agentruntime.StoredToolResult{}, agentruntime.ErrLeaseLost
	}
	run, err := s.Get(ctx, fence.RunKey)
	if err != nil || run.Owner != fence.Owner || run.Epoch != fence.Epoch {
		return agentruntime.StoredToolResult{}, agentruntime.ErrLeaseLost
	}
	var row agentToolCallRow
	if err := toolCallScope(s.db.WithContext(ctx), fence.RunKey, callID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return agentruntime.StoredToolResult{}, agentruntime.ErrNotFound
		}
		return agentruntime.StoredToolResult{}, err
	}
	record, err := row.view()
	if err != nil {
		return agentruntime.StoredToolResult{}, err
	}
	if record.Result == nil {
		return agentruntime.StoredToolResult{}, agentruntime.ErrNotFound
	}
	return *record.Result, nil
}

func (row agentToolCallRow) view() (agentruntime.ToolRecord, error) {
	r := agentruntime.ToolRecord{
		Plan: agentruntime.ToolPlan{
			Version: row.PlanVersion,
			CallID:  row.CallID, Name: row.ToolName, Identity: row.ToolIdentity,
			ArgsHash: row.ArgsHash, Args: json.RawMessage(row.Args), RecoveryPolicy: row.RecoveryPolicy,
			IdempotencyKey: row.IdempotencyKey,
		},
		CallSeq: row.CallSeq, Status: row.Status, UnknownReason: row.UnknownReason,
	}
	if row.IdempotencyExpiresAt != nil {
		r.Plan.IdempotencyExpiresAt = *row.IdempotencyExpiresAt
	}
	if row.Result != nil {
		result := agentruntime.StoredToolResult{Source: row.Source}
		if err := json.Unmarshal([]byte(*row.Result), &result.Result); err != nil {
			return r, err
		}
		if err := json.Unmarshal([]byte(row.OutputFiles), &result.OutputFiles); err != nil {
			return r, err
		}
		result.Result.OutputFiles = append([]string(nil), result.OutputFiles...)
		r.Result = &result
	}
	return r, nil
}

// lockToolRun serializes journal writes with lease takeover and cancellation.
// Session ownership and the reserved run slot are checked in the same
// transaction; detailed tool authorization still belongs to ToolRegistry.
func (s *AgentRunStore) lockToolRun(tx *gorm.DB, fence agentruntime.Fence) error {
	if fence.TenantID == 0 || fence.RunID == "" || fence.Owner == "" || fence.Epoch <= 0 {
		return agentruntime.ErrLeaseLost
	}
	locked := s.fenced(tx, fence).Updates(map[string]any{
		"revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
	})
	if locked.Error != nil {
		return locked.Error
	}
	if locked.RowsAffected != 1 {
		return agentruntime.ErrLeaseLost
	}
	var run agentRunRow
	if err := runScope(tx, fence.RunKey).Take(&run).Error; err != nil {
		return err
	}
	// Updating the session locks permission-changing writes as well as checking
	// them, on both PostgreSQL and SQLite.
	session := tx.Table("sessions").Where(
		"tenant_id = ? AND id = ? AND user_id = ? AND active_agent_run_id = ? "+
			"AND engine_type = 'trpc' AND deleted_at IS NULL",
		fence.TenantID, run.SessionID, run.OwnerID, fence.RunID).
		UpdateColumn("active_agent_run_id", gorm.Expr("active_agent_run_id"))
	if session.Error != nil {
		return session.Error
	}
	if session.RowsAffected != 1 {
		return agentruntime.ErrLeaseLost
	}
	return nil
}

func normalizeToolPlan(plan agentruntime.ToolPlan) (agentruntime.ToolPlan, error) {
	if plan.Version == 0 {
		plan.Version = 1
	}
	if plan.Version < 1 {
		return plan, agentruntime.ErrConflict
	}
	if plan.CallID == "" || len(plan.CallID) > 255 || plan.Name == "" || len(plan.Name) > 255 ||
		plan.Identity == "" || len(plan.Identity) > 512 || plan.ArgsHash == "" || len(plan.ArgsHash) > 64 ||
		len(plan.IdempotencyKey) > 255 || !json.Valid(plan.Args) {
		return plan, agentruntime.ErrConflict
	}
	if plan.RecoveryPolicy == "" {
		plan.RecoveryPolicy = agentruntime.ToolRecoveryWaitUser
	}
	switch plan.RecoveryPolicy {
	case agentruntime.ToolRecoveryWaitUser, agentruntime.ToolRecoveryQueryable,
		agentruntime.ToolRecoveryIdempotent, agentruntime.ToolRecoveryReadOnly:
	default:
		return plan, agentruntime.ErrConflict
	}
	if !plan.IdempotencyExpiresAt.IsZero() {
		plan.IdempotencyExpiresAt = plan.IdempotencyExpiresAt.UTC().Truncate(time.Microsecond)
	}
	return plan, nil
}

func sameToolPlan(a, b agentruntime.ToolPlan) bool {
	// JSONB may change whitespace/key ordering, so compare JSON values without
	// float64 conversion (large integer arguments must retain their identity).
	var aa, bb any
	da, db := json.NewDecoder(bytes.NewReader(a.Args)), json.NewDecoder(bytes.NewReader(b.Args))
	da.UseNumber()
	db.UseNumber()
	if da.Decode(&aa) != nil || db.Decode(&bb) != nil || !reflect.DeepEqual(aa, bb) {
		return false
	}
	return a.Version == b.Version && a.CallID == b.CallID && a.Name == b.Name &&
		a.Identity == b.Identity && a.ArgsHash == b.ArgsHash &&
		a.IdempotencyKey == b.IdempotencyKey && a.RecoveryPolicy == b.RecoveryPolicy &&
		a.IdempotencyExpiresAt.Equal(b.IdempotencyExpiresAt)
}

// EnsureToolPlan persists the immutable logical call before dispatch. Repeated
// calls can only observe exactly the same identity, arguments and safety facts.
func (s *AgentRunStore) EnsureToolPlan(
	ctx context.Context, fence agentruntime.Fence, plan agentruntime.ToolPlan,
) (agentruntime.ToolRecord, error) {
	plan, err := normalizeToolPlan(plan)
	if err != nil {
		return agentruntime.ToolRecord{}, err
	}
	var record agentruntime.ToolRecord
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := s.lockToolRun(tx, fence); e != nil {
			return e
		}
		var row agentToolCallRow
		e := toolCallScope(tx, fence.RunKey, plan.CallID).Take(&row).Error
		if e == nil {
			record, e = row.view()
			if e != nil {
				return e
			}
			if !sameToolPlan(record.Plan, plan) {
				var history []agentruntime.ToolPlan
				if err := json.Unmarshal([]byte(row.PlanHistory), &history); err != nil {
					return err
				}
				for _, previous := range history {
					if sameToolPlan(previous, plan) {
						return nil
					}
				}
				return agentruntime.ErrConflict
			}
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		var seq int64
		if e := tx.Model(&agentToolCallRow{}).Where("tenant_id = ? AND run_id = ?", fence.TenantID, fence.RunID).
			Select("COALESCE(MAX(call_seq), 0)").Scan(&seq).Error; e != nil {
			return e
		}
		row = agentToolCallRow{
			PlanVersion: plan.Version, PlanHistory: "[]",
			TenantID: fence.TenantID, RunID: fence.RunID, CallID: plan.CallID,
			CallSeq: seq + 1, ToolName: plan.Name, ToolIdentity: plan.Identity, ArgsHash: plan.ArgsHash,
			Args: string(plan.Args), RecoveryPolicy: plan.RecoveryPolicy, IdempotencyKey: plan.IdempotencyKey,
			Status: agentruntime.ToolStatusPlanned, OutputFiles: "[]",
		}
		if !plan.IdempotencyExpiresAt.IsZero() {
			row.IdempotencyExpiresAt = &plan.IdempotencyExpiresAt
		}
		if e := tx.Create(&row).Error; e != nil {
			return e
		}
		record, e = row.view()
		return e
	})
	return record, err
}

// BeginToolAttempt appends one attempt. A dispatch in this epoch is still live,
// even for a read-only tool, and cannot be concurrently repeated.
func (s *AgentRunStore) BeginToolAttempt(
	ctx context.Context, fence agentruntime.Fence, callID string, expectedVersion ...int64,
) (agentruntime.ToolAttempt, error) {
	var attempt agentruntime.ToolAttempt
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := s.lockToolRun(tx, fence); e != nil {
			return e
		}
		deadline := "deadline"
		if s.db.Name() == "sqlite" {
			deadline = "julianday(deadline)"
		}
		var live int64
		if e := runScope(tx, fence.RunKey).Where(deadline + " > " + s.nowSQL()).Count(&live).Error; e != nil {
			return e
		}
		if live != 1 {
			return agentruntime.ErrLeaseLost
		}
		var row agentToolCallRow
		if e := toolCallScope(tx, fence.RunKey, callID).Take(&row).Error; e != nil {
			return e
		}
		record, e := row.view()
		if e != nil {
			return e
		}
		if len(expectedVersion) > 1 || (len(expectedVersion) == 1 && expectedVersion[0] != record.Plan.Version) {
			return agentruntime.ErrConflict
		}
		var previous agentToolAttemptRow
		e = toolCallScope(tx, fence.RunKey, callID).Order("attempt DESC").Take(&previous).Error
		if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if row.Status == agentruntime.ToolStatusDispatching && previous.Epoch == fence.Epoch {
			return agentruntime.ErrConflict
		}
		switch agentruntime.RecoveryAction(record.RecoveryFacts(time.Now())) {
		case "execute", "retry":
		case "wait_user":
			return agentruntime.ErrToolWaitUser
		case "query":
			return agentruntime.ErrToolRecoveryQuery
		default:
			return agentruntime.ErrConflict
		}
		now := time.Now().UTC()
		next := agentToolAttemptRow{
			TenantID: fence.TenantID, RunID: fence.RunID, CallID: callID,
			Attempt: previous.Attempt + 1, Epoch: fence.Epoch, Status: agentruntime.ToolStatusDispatching,
			DispatchedAt: &now,
		}
		if e := tx.Create(&next).Error; e != nil {
			return e
		}
		update := toolCallScope(tx.Model(&agentToolCallRow{}), fence.RunKey, callID).
			Updates(map[string]any{"status": agentruntime.ToolStatusDispatching, "unknown_reason": ""})
		if update.Error != nil {
			return update.Error
		}
		attempt = agentruntime.ToolAttempt{
			RunKey: fence.RunKey, Owner: fence.Owner,
			CallID: callID, Number: next.Attempt, Epoch: fence.Epoch,
		}
		return nil
	})
	return attempt, err
}

// ReviseToolPlan creates a new version only from gate-approved arguments before
// any external attempt. Prior versions are immutable audit/recovery references;
// only the latest version may dispatch, with a new parameter hash and key.
func (s *AgentRunStore) ReviseToolPlan(
	ctx context.Context, fence agentruntime.Fence, callID string, expectedVersion int64, args json.RawMessage,
) (agentruntime.ToolPlan, error) {
	var object map[string]any
	decoder := json.NewDecoder(bytes.NewReader(args))
	decoder.UseNumber()
	if !json.Valid(args) || decoder.Decode(&object) != nil || object == nil {
		return agentruntime.ToolPlan{}, agentruntime.ErrConflict
	}
	canonical, err := json.Marshal(object)
	if err != nil {
		return agentruntime.ToolPlan{}, err
	}
	var revised agentruntime.ToolPlan
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.lockToolRun(tx, fence); err != nil {
			return err
		}
		var row agentToolCallRow
		if err := toolCallScope(tx, fence.RunKey, callID).Take(&row).Error; err != nil {
			return err
		}
		record, err := row.view()
		if err != nil {
			return err
		}
		if record.Plan.Version != expectedVersion || record.Status != agentruntime.ToolStatusPlanned {
			return agentruntime.ErrConflict
		}
		var attempts int64
		count := toolCallScope(tx.Model(&agentToolAttemptRow{}), fence.RunKey, callID).Count(&attempts)
		if count.Error != nil {
			return count.Error
		}
		if attempts != 0 {
			return agentruntime.ErrConflict
		}
		revised = record.Plan
		revised.Args = canonical
		if sameToolPlan(revised, record.Plan) {
			return nil
		}
		var history []agentruntime.ToolPlan
		if err := json.Unmarshal([]byte(row.PlanHistory), &history); err != nil {
			return err
		}
		history = append(history, record.Plan)
		historyJSON, err := json.Marshal(history)
		if err != nil {
			return err
		}
		revised.Version++
		revised.ArgsHash = fmt.Sprintf("%x", sha256.Sum256(canonical))
		keyMaterial := fmt.Sprintf("%d:%s:%s:%d:%s:%s", fence.TenantID, fence.RunID, callID,
			revised.Version, record.Plan.IdempotencyKey, revised.ArgsHash)
		revised.IdempotencyKey = fmt.Sprintf("%x", sha256.Sum256([]byte(keyMaterial)))
		// Replace the approval binding, never carry a previous version's
		// approval forward to new arguments. This call is the new gate decision.
		return toolCallScope(tx.Model(&agentToolCallRow{}), fence.RunKey, callID).Updates(map[string]any{
			"args": string(canonical), "args_hash": revised.ArgsHash, "idempotency_key": revised.IdempotencyKey,
			"plan_version": revised.Version, "approved_plan_version": revised.Version,
			"plan_history": string(historyJSON),
		}).Error
	})
	return revised, err
}

func validToolAttempt(fence agentruntime.Fence, attempt agentruntime.ToolAttempt) bool {
	return attempt.RunKey == fence.RunKey && attempt.Owner == fence.Owner && attempt.Epoch == fence.Epoch &&
		attempt.CallID != "" && attempt.Number > 0
}

// finishToolAttempt is the sole completion transition. It never replaces an
// earlier attempt and rejects stale attempts even within the same epoch.
func (s *AgentRunStore) finishToolAttempt(
	ctx context.Context, fence agentruntime.Fence, attempt agentruntime.ToolAttempt,
	status, reason string, fields map[string]any,
) error {
	if !validToolAttempt(fence, attempt) {
		return agentruntime.ErrConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := s.lockToolRun(tx, fence); e != nil {
			return e
		}
		var latest agentToolAttemptRow
		if e := toolCallScope(tx, fence.RunKey, attempt.CallID).Order("attempt DESC").Take(&latest).Error; e != nil {
			return e
		}
		if latest.Attempt != attempt.Number || latest.Epoch != attempt.Epoch ||
			latest.Status != agentruntime.ToolStatusDispatching {
			return agentruntime.ErrConflict
		}
		fields["status"], fields["unknown_reason"] = status, reason
		updated := toolCallScope(tx.Model(&agentToolCallRow{}), fence.RunKey, attempt.CallID).
			Where("status = ?", agentruntime.ToolStatusDispatching).Updates(fields)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return agentruntime.ErrConflict
		}
		return toolCallScope(tx.Model(&agentToolAttemptRow{}), fence.RunKey, attempt.CallID).
			Where("attempt = ? AND epoch = ?", attempt.Number, attempt.Epoch).
			Updates(map[string]any{"status": status, "error_message": reason, "finished_at": time.Now().UTC()}).Error
	})
}

// CommitToolResult stores files separately because ToolResult excludes them
// from JSON. Images remain part of the normal result envelope.
func (s *AgentRunStore) CommitToolResult(
	ctx context.Context, fence agentruntime.Fence, attempt agentruntime.ToolAttempt,
	result agentruntime.StoredToolResult,
) error {
	fields, err := storedToolResultFields(result)
	if err != nil {
		return err
	}
	status := agentruntime.ToolStatusFailed
	if result.Result.Success {
		status = agentruntime.ToolStatusSucceeded
	}
	return s.finishToolAttempt(ctx, fence, attempt, status, "", fields)
}

func storedToolResultFields(result agentruntime.StoredToolResult) (map[string]any, error) {
	if result.Source == "" {
		result.Source = "tool"
	}
	files := result.OutputFiles
	if files == nil {
		files = result.Result.OutputFiles
	}
	if files == nil {
		files = []string{}
	}
	encodedFiles, err := json.Marshal(files)
	if err != nil {
		return nil, err
	}
	encodedResult, err := json.Marshal(result.Result)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"result": string(encodedResult), "output_files": string(encodedFiles), "source": result.Source,
	}, nil
}

// CommitToolRejection records a definitive preflight rejection without claiming
// an external dispatch. Pending approval/OAuth errors never call this method.
func (s *AgentRunStore) CommitToolRejection(
	ctx context.Context, fence agentruntime.Fence, callID string, result agentruntime.StoredToolResult,
) error {
	if result.Result.Success || callID == "" {
		return agentruntime.ErrConflict
	}
	fields, err := storedToolResultFields(result)
	if err != nil {
		return err
	}
	fields["status"] = agentruntime.ToolStatusFailed
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.lockToolRun(tx, fence); err != nil {
			return err
		}
		var attempts int64
		count := toolCallScope(tx.Model(&agentToolAttemptRow{}), fence.RunKey, callID).Count(&attempts)
		if err := count.Error; err != nil {
			return err
		}
		if attempts != 0 {
			return agentruntime.ErrConflict
		}
		updated := toolCallScope(tx.Model(&agentToolCallRow{}), fence.RunKey, callID).
			Where("status = ?", agentruntime.ToolStatusPlanned).Updates(fields)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return agentruntime.ErrConflict
		}
		return nil
	})
}

// MarkToolUnknown preserves uncertainty after any dispatched error. It must
// not turn timeouts/cancellation into permission to repeat a side effect.
func (s *AgentRunStore) MarkToolUnknown(
	ctx context.Context, fence agentruntime.Fence, attempt agentruntime.ToolAttempt, reason string,
) error {
	return s.finishToolAttempt(ctx, fence, attempt, agentruntime.ToolStatusUnknown, reason, map[string]any{})
}
