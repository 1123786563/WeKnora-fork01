package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AgentRunStore owns durable tRPC admission, leases and graph checkpoints.
// Every mutation of an executing run must lock its row with a valid fence in
// the same transaction as the mutation. Checking a fence and then writing in
// a separate transaction would let a superseded worker commit stale state.
type AgentRunStore struct{ db *gorm.DB }

var _ agentruntime.RunStore = (*AgentRunStore)(nil)

// NewAgentRunStore constructs a store backed by the migrated business database.
func NewAgentRunStore(db *gorm.DB) *AgentRunStore { return &AgentRunStore{db: db} }

type agentRunRow struct {
	TenantID                                                              uint64
	RunID, SessionID, OwnerID, RequestID, AssistantMessageID, RequestHash string
	EngineType, Status, WaitReason                                        string
	Snapshot                                                              string
	GraphVersion, SDKVersion                                              string
	SchemaVersion                                                         int
	LeaseOwner                                                            string
	LeaseUntil                                                            *time.Time
	Epoch, Revision                                                       int64
	MaxRounds, MaxToolCalls                                               int
	TokenBudget                                                           int64
	Deadline, CreatedAt, UpdatedAt                                        time.Time
}

func (agentRunRow) TableName() string { return "agent_runs" }

func (r agentRunRow) view() agentruntime.Run {
	run := agentruntime.Run{
		Key:       agentruntime.RunKey{TenantID: r.TenantID, RunID: r.RunID},
		SessionID: r.SessionID, UserID: r.OwnerID, RequestID: r.RequestID,
		AssistantMessageID: r.AssistantMessageID, Status: r.Status,
		WaitReason: r.WaitReason, Owner: r.LeaseOwner, Revision: r.Revision,
		Epoch: r.Epoch, Deadline: r.Deadline, Snapshot: json.RawMessage(r.Snapshot),
	}
	if r.LeaseUntil != nil {
		run.LeaseUntil = *r.LeaseUntil
	}
	return run
}

func runScope(db *gorm.DB, key agentruntime.RunKey) *gorm.DB {
	return db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID)
}

// Get loads a run in the specified tenant.
func (s *AgentRunStore) Get(ctx context.Context, key agentruntime.RunKey) (agentruntime.Run, error) {
	var row agentRunRow
	err := runScope(s.db.WithContext(ctx), key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return agentruntime.Run{}, agentruntime.ErrNotFound
	}
	return row.view(), err
}

// Admit atomically reserves a session, creates both business messages and
// persists the immutable request snapshot. Request retries are scoped to the
// authenticated tenant and owner; session validation precedes idempotency reads.
func (s *AgentRunStore) Admit(ctx context.Context, in agentruntime.Admission) (agentruntime.Run, error) {
	if in.Key.TenantID == 0 || in.Key.RunID == "" || in.SessionID == "" || in.UserID == "" ||
		in.RequestID == "" || in.AssistantMessageID == "" || in.RequestHash == "" || in.Deadline.IsZero() ||
		!json.Valid(in.Snapshot) {
		return agentruntime.Run{}, agentruntime.ErrConflict
	}
	user, err := admissionMessage(in.UserMessage, "user", in)
	if err != nil {
		return agentruntime.Run{}, err
	}
	assistant, err := admissionMessage(in.AssistantMessage, "assistant", in)
	if err != nil {
		return agentruntime.Run{}, err
	}
	assistant.ID = in.AssistantMessageID
	if in.UserMessageID != "" {
		// Reuse the handler-persisted user row: exactly one user message per
		// request regardless of which side wrote it first.
		user.ID = in.UserMessageID
	}
	var result agentruntime.Run
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// The write locks this session before any reads. This also avoids a
		// deferred SQLite read transaction trying to upgrade to a write lock.
		lock := tx.Table("sessions").Where("tenant_id = ? AND id = ? AND user_id = ? AND deleted_at IS NULL",
			in.Key.TenantID, in.SessionID, in.UserID).
			UpdateColumn("active_agent_run_id", gorm.Expr("active_agent_run_id"))
		if lock.Error != nil {
			return lock.Error
		}
		if lock.RowsAffected != 1 {
			return agentruntime.ErrNotFound
		}
		var session struct {
			EngineType       string
			ActiveAgentRunID *string
		}
		if e := tx.Table("sessions").Where("tenant_id = ? AND id = ?", in.Key.TenantID, in.SessionID).
			Take(&session).Error; e != nil {
			return e
		}
		if session.EngineType != "trpc" {
			return agentruntime.ErrConflict
		}
		var existing agentRunRow
		e := tx.Where("tenant_id = ? AND owner_id = ? AND request_id = ?",
			in.Key.TenantID, in.UserID, in.RequestID).Take(&existing).Error
		if e == nil {
			if existing.RequestHash != in.RequestHash || existing.SessionID != in.SessionID {
				return agentruntime.ErrConflict
			}
			result = existing.view()
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		slot := tx.Table("sessions").Where("tenant_id = ? AND id = ? AND active_agent_run_id IS NULL",
			in.Key.TenantID, in.SessionID).UpdateColumn("active_agent_run_id", in.Key.RunID)
		if slot.Error != nil {
			return slot.Error
		}
		if slot.RowsAffected != 1 {
			return agentruntime.ErrRunActive
		}
		row := agentRunRow{
			TenantID: in.Key.TenantID, RunID: in.Key.RunID,
			SessionID: in.SessionID, OwnerID: in.UserID, RequestID: in.RequestID,
			AssistantMessageID: in.AssistantMessageID, RequestHash: in.RequestHash,
			EngineType: "trpc", Status: "queued", Snapshot: string(in.Snapshot),
			GraphVersion: "1", SchemaVersion: 1, Deadline: in.Deadline,
		}
		// A concurrent request may target a different session: the database
		// unique key is the final arbiter and the slot reservation rolls back.
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected != 1 {
			return agentruntime.ErrConflict
		}
		// BeforeCreate unconditionally generates an ID. It was already applied
		// during normalization; skip it here to preserve the admitted IDs.
		// The HTTP handler persists the assistant placeholder (and on retries
		// the user message) before admission runs, so both creates are
		// idempotent by id: an existing row is reused, never a conflict.
		// Finalization owns the assistant row content by id regardless.
		if e := tx.Session(&gorm.Session{SkipHooks: true}).
			Clauses(clause.OnConflict{DoNothing: true}).Create(&user).Error; e != nil {
			return e
		}
		if e := tx.Session(&gorm.Session{SkipHooks: true}).
			Clauses(clause.OnConflict{DoNothing: true}).Create(&assistant).Error; e != nil {
			return e
		}
		result = row.view()
		return nil
	})
	return result, err
}

func admissionMessage(raw json.RawMessage, role string, in agentruntime.Admission) (types.Message, error) {
	var message types.Message
	if len(raw) == 0 || json.Unmarshal(raw, &message) != nil || message.Role != role {
		return message, fmt.Errorf("%w: invalid %s message", agentruntime.ErrConflict, role)
	}
	// Identity and lifecycle fields come only from admission, never from JSON.
	if err := message.BeforeCreate(nil); err != nil {
		return message, err
	}
	message.SessionID, message.RequestID = in.SessionID, in.RequestID
	message.CreatedAt, message.UpdatedAt = time.Time{}, time.Time{}
	message.DeletedAt = gorm.DeletedAt{}
	message.IsCompleted = role == "user"
	return message, nil
}

// SQLite stores dates as text; julianday accepts both driver timestamps and
// SQLite's UTC strftime format, without relying on lexical timezone ordering.
func (s *AgentRunStore) nowSQL() string {
	if s.db.Name() == "sqlite" {
		return "julianday('now')"
	}
	return "clock_timestamp()"
}

func (s *AgentRunStore) leaseColumnSQL() string {
	if s.db.Name() == "sqlite" {
		return "julianday(lease_until)"
	}
	return "lease_until"
}

func (s *AgentRunStore) leaseExpiry(ttl time.Duration) clause.Expr {
	if s.db.Name() == "sqlite" {
		return gorm.Expr("strftime('%Y-%m-%d %H:%M:%f', 'now', ?)", fmt.Sprintf("+%.3f seconds", ttl.Seconds()))
	}
	return gorm.Expr("clock_timestamp() + (? * interval '1 second')", ttl.Seconds())
}

func (s *AgentRunStore) claimableSQL() string {
	return "(status = 'queued' OR (status IN ('running', 'recovering') AND (lease_until IS NULL OR " +
		s.leaseColumnSQL() + " <= " + s.nowSQL() + ")))"
}

// Claim takes a queued or expired run using database time and increments its epoch.
func (s *AgentRunStore) Claim(
	ctx context.Context, key agentruntime.RunKey, owner string, ttl time.Duration,
) (agentruntime.Fence, error) {
	if owner == "" || ttl < time.Millisecond {
		return agentruntime.Fence{}, agentruntime.ErrConflict
	}
	var fence agentruntime.Fence
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		claimed := runScope(tx, key).Where(s.claimableSQL()).Updates(map[string]any{
			"lease_owner": owner, "lease_until": s.leaseExpiry(ttl), "epoch": gorm.Expr("epoch + 1"),
			"revision": gorm.Expr("revision + 1"), "status": "running", "updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
		if claimed.Error != nil {
			return claimed.Error
		}
		if claimed.RowsAffected != 1 {
			return agentruntime.ErrLeaseLost
		}
		var row agentRunRow
		if e := runScope(tx, key).Take(&row).Error; e != nil {
			return e
		}
		fence = agentruntime.Fence{RunKey: key, Owner: owner, Epoch: row.Epoch}
		return nil
	})
	return fence, err
}

func (s *AgentRunStore) fenced(tx *gorm.DB, fence agentruntime.Fence) *gorm.DB {
	return runScope(tx, fence.RunKey).Where(
		"lease_owner = ? AND epoch = ? AND status IN ('running', 'recovering') AND "+
			s.leaseColumnSQL()+" > "+s.nowSQL(), fence.Owner, fence.Epoch)
}

// Renew extends only a still-valid lease owned by the supplied fence.
func (s *AgentRunStore) Renew(ctx context.Context, fence agentruntime.Fence, ttl time.Duration) error {
	if ttl < time.Millisecond {
		return agentruntime.ErrConflict
	}
	result := s.fenced(s.db.WithContext(ctx), fence).Updates(map[string]any{
		"lease_until": s.leaseExpiry(ttl), "updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return agentruntime.ErrLeaseLost
	}
	return nil
}

// Scan lists claimable work, including expired runs needing recovery.
func (s *AgentRunStore) Scan(ctx context.Context, limit int) ([]agentruntime.RunKey, error) {
	if limit <= 0 {
		return nil, agentruntime.ErrConflict
	}
	var keys []agentruntime.RunKey
	err := s.db.WithContext(ctx).Table("agent_runs").Select("tenant_id, run_id").Where(s.claimableSQL()).
		Order("created_at ASC, tenant_id ASC, run_id ASC").Limit(limit).Scan(&keys).Error
	return keys, err
}

type agentCheckpointRow struct {
	TenantID                                 uint64
	RunID, Namespace, CheckpointID, ParentID string
	Seq                                      int64
	State, PendingWrites                     string
	CreatedAt, UpdatedAt                     time.Time
}

func (agentCheckpointRow) TableName() string { return "agent_run_checkpoints" }

// SaveCheckpoint saves graph state and pending writes under a transactional fence.
func (s *AgentRunStore) SaveCheckpoint(
	ctx context.Context, fence agentruntime.Fence, cp agentruntime.CheckpointRecord,
) error {
	if cp.Namespace == "" || cp.ID == "" || cp.Seq < 0 || !json.Valid(cp.State) || !json.Valid(cp.PendingWrites) {
		return agentruntime.ErrConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		locked := s.fenced(tx, fence).Updates(map[string]any{
			"revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
		if locked.Error != nil {
			return locked.Error
		}
		if locked.RowsAffected != 1 {
			return agentruntime.ErrLeaseLost
		}
		row := agentCheckpointRow{
			TenantID: fence.TenantID, RunID: fence.RunID,
			Namespace: cp.Namespace, CheckpointID: cp.ID, ParentID: cp.ParentID, Seq: cp.Seq,
			State: string(cp.State), PendingWrites: string(cp.PendingWrites),
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"}, {Name: "run_id"}, {Name: "namespace"}, {Name: "checkpoint_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"parent_id", "seq", "state", "pending_writes", "updated_at"}),
		}).Create(&row).Error
	})
}

// SetStatus durably records execution outcome under the current fence.
// A terminal failure releases the session's active-run slot in the same
// transaction: only non-terminal runs (including waiting_user) may hold it,
// otherwise one failed run would wedge the session's future admissions.
func (s *AgentRunStore) SetStatus(ctx context.Context, fence agentruntime.Fence, status, reason string) error {
	if status != "succeeded" && status != "failed" && status != "waiting_user" {
		return agentruntime.ErrConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := s.fenced(tx, fence).Updates(map[string]any{"status": status, "wait_reason": reason, "lease_owner": "", "lease_until": nil, "revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return agentruntime.ErrLeaseLost
		}
		if status == "failed" {
			var row agentRunRow
			if err := runScope(tx, fence.RunKey).Take(&row).Error; err != nil {
				return err
			}
			return tx.Table("sessions").Where("tenant_id=? AND id=? AND active_agent_run_id=?",
				fence.TenantID, row.SessionID, fence.RunID).Update("active_agent_run_id", nil).Error
		}
		return nil
	})
}

// LoadCheckpoint returns the latest committed graph snapshot for one tenant/run.
func (s *AgentRunStore) LoadCheckpoint(
	ctx context.Context, key agentruntime.RunKey,
) (agentruntime.CheckpointRecord, error) {
	var row agentCheckpointRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).
		Order("seq DESC, namespace ASC, checkpoint_id DESC").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return agentruntime.CheckpointRecord{}, agentruntime.ErrNotFound
	}
	return agentruntime.CheckpointRecord{
		Namespace: row.Namespace, ID: row.CheckpointID,
		ParentID: row.ParentID, Seq: row.Seq,
		State: json.RawMessage(row.State), PendingWrites: json.RawMessage(row.PendingWrites),
	}, err
}
