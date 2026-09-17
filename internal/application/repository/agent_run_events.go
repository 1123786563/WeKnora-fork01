package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type agentRunEventRow struct {
	TenantID                      uint64
	RunID                         string
	Seq                           int64
	AttemptID, EventType, Payload string
}

func (agentRunEventRow) TableName() string { return "agent_run_events" }

// AppendEvent allocates the next sequence while holding the run's fenced row.
func (s *AgentRunStore) AppendEvent(ctx context.Context, fence agentruntime.Fence, evt agentruntime.RunEvent) (agentruntime.RunEvent, error) {
	if evt.Type == "" || len(evt.Type) > 128 || len(evt.Payload) == 0 || !json.Valid(evt.Payload) {
		return agentruntime.RunEvent{}, agentruntime.ErrConflict
	}
	var out agentruntime.RunEvent
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.lockToolRun(tx, fence); err != nil {
			return err
		}
		var last agentRunEventRow
		e := tx.Where("tenant_id=? AND run_id=?", fence.TenantID, fence.RunID).Order("seq DESC").Take(&last).Error
		if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		seq := last.Seq + 1
		row := agentRunEventRow{TenantID: fence.TenantID, RunID: fence.RunID, Seq: seq, AttemptID: evt.AttemptID, EventType: evt.Type, Payload: string(evt.Payload)}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		out = agentruntime.RunEvent{Seq: seq, AttemptID: evt.AttemptID, Type: evt.Type, Payload: append(json.RawMessage(nil), evt.Payload...)}
		return nil
	})
	return out, err
}

func (s *AgentRunStore) ReadEvents(ctx context.Context, key agentruntime.RunKey, after int64, limit int) ([]agentruntime.RunEvent, error) {
	if limit <= 0 {
		return nil, agentruntime.ErrConflict
	}
	if after < 0 {
		return nil, agentruntime.ErrCursorExpired
	}
	var total int64
	if err := s.db.WithContext(ctx).Table("agent_runs").Where("tenant_id=? AND run_id=?", key.TenantID, key.RunID).Count(&total).Error; err != nil {
		return nil, err
	}
	if total == 0 {
		return nil, agentruntime.ErrNotFound
	}
	var first agentRunEventRow
	if err := s.db.WithContext(ctx).Where("tenant_id=? AND run_id=?", key.TenantID, key.RunID).Order("seq ASC").Take(&first).Error; err == nil && first.Seq > after+1 {
		return nil, agentruntime.ErrCursorExpired
	}
	var rows []agentRunEventRow
	q := s.db.WithContext(ctx).Where("tenant_id=? AND run_id=? AND seq>?", key.TenantID, key.RunID, after).Order("seq ASC").Limit(limit).Find(&rows)
	if q.Error != nil {
		return nil, q.Error
	}
	out := make([]agentruntime.RunEvent, 0, len(rows))
	for _, r := range rows {
		out = append(out, agentruntime.RunEvent{Seq: r.Seq, AttemptID: r.AttemptID, Type: r.EventType, Payload: json.RawMessage(r.Payload)})
	}
	return out, nil
}

// LastEventSeq returns the highest retained sequence for a run, or 0 when
// no event exists. Finalize-time retention trimming uses it as the anchor.
func (s *AgentRunStore) LastEventSeq(ctx context.Context, key agentruntime.RunKey) (int64, error) {
	var last agentRunEventRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).
		Order("seq DESC").Take(&last).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return last.Seq, nil
}

// TrimEventsBefore deletes retained events with seq below the watermark for
// one run; the run row and events at or above the watermark are untouched.
// Trimming is what makes the replay cursor's explicit reload error reachable
// in production.
func (s *AgentRunStore) TrimEventsBefore(
	ctx context.Context, key agentruntime.RunKey, watermark int64,
) (int64, error) {
	if watermark < 0 {
		return 0, agentruntime.ErrConflict
	}
	result := s.db.WithContext(ctx).
		Where("tenant_id = ? AND run_id = ? AND seq < ?", key.TenantID, key.RunID, watermark).
		Delete(&agentRunEventRow{})
	return result.RowsAffected, result.Error
}

// Finalize atomically completes the assistant message, run, completion event, and slot.
func (s *AgentRunStore) Finalize(ctx context.Context, fence agentruntime.Fence, answer json.RawMessage) error {
	if len(answer) == 0 || !json.Valid(answer) {
		return agentruntime.ErrConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.lockToolRun(tx, fence); err != nil {
			var terminal agentRunRow
			if e := runScope(tx, fence.RunKey).Where("status = ?", "succeeded").Take(&terminal).Error; e == nil {
				return nil
			}
			return agentruntime.ErrLeaseLost
		}
		var run agentRunRow
		if err := runScope(tx, fence.RunKey).Take(&run).Error; err != nil {
			return err
		}
		var obj struct {
			Content string `json:"content"`
			Answer  string `json:"answer"`
		}
		if err := json.Unmarshal(answer, &obj); err != nil {
			return err
		}
		if obj.Content == "" {
			obj.Content = obj.Answer
		}
		if obj.Content == "" {
			obj.Content = string(answer)
		}
		// Idempotent finalize: a terminal run is already committed.
		if run.Status == "succeeded" {
			return nil
		}
		if run.AssistantMessageID != "" {
			if err := tx.Table("messages").Where("id=? AND session_id=?", run.AssistantMessageID, run.SessionID).Updates(map[string]any{"content": obj.Content, "is_completed": true}).Error; err != nil {
				return err
			}
		}
		if err := s.fenced(tx, fence).Updates(map[string]any{"status": "succeeded", "wait_reason": "", "lease_owner": "", "lease_until": nil, "revision": gorm.Expr("revision+1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP")}).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Table("agent_run_events").Where("tenant_id=? AND run_id=? AND event_type='run_completed'", fence.TenantID, fence.RunID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			payload := string(answer)
			if err := appendRunEventLocked(tx, fence, "run_completed", payload); err != nil {
				return err
			}
		}
		return tx.Table("sessions").Where("tenant_id=? AND id=? AND active_agent_run_id=?", fence.TenantID, run.SessionID, fence.RunID).Update("active_agent_run_id", nil).Error
	})
}

// appendRunEventLocked serializes event sequence allocation behind the run
// row lock. All callers invoke it inside the same transaction that fenced or
// updated the run, so cancellation and completion cannot allocate one seq.
func appendRunEventLocked(tx *gorm.DB, f agentruntime.Fence, eventType, payload string) error {
	if err := tx.Table("agent_runs").Where("tenant_id=? AND run_id=?", f.TenantID, f.RunID).UpdateColumn("revision", gorm.Expr("revision")).Error; err != nil {
		return err
	}
	var r agentRunEventRow
	if err := tx.Where("tenant_id=? AND run_id=?", f.TenantID, f.RunID).Order("seq DESC").Take(&r).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Create(&agentRunEventRow{TenantID: f.TenantID, RunID: f.RunID, Seq: r.Seq + 1, EventType: eventType, Payload: payload}).Error
}

// DefaultEventRetention is the default agent-run event retention window:
// 30 days, overridable per pass through EventRetentionOptions.Retention.
const DefaultEventRetention = 30 * 24 * time.Hour

// eventRetentionProtectedSQL lists the event families that time-based
// retention must NEVER delete, because each follows its own retention or
// evidence policy instead of the global one:
//
//   - usage.*      — commercial usage records (billing/settlement evidence)
//   - approval.*   — live approval lane evidence
//   - run_completed — the finalize idempotency receipt: Finalize deduplicates
//     on its existence, so deleting it would let a late finalize retry
//     append a duplicate completion event
//   - retention.trimmed — the durable snapshot rows this pass writes; they
//     are what makes a trimmed prefix auditable and must outlive the trim
const eventRetentionProtectedSQL = "(event_type LIKE 'usage.%' OR event_type LIKE 'approval.%' OR event_type = 'run_completed' OR event_type = 'retention.trimmed')"

var eventRetentionTerminalStatuses = []string{"succeeded", "failed", "canceled"}

// EventRetentionOptions parameterizes one ApplyEventRetention pass.
type EventRetentionOptions struct {
	// Retention is the retention window. Zero selects DefaultEventRetention
	// (30 days); negative is rejected with ErrConflict — a negative window
	// would mean trim-everything, which is a configuration error.
	Retention time.Duration
	// Now allows tests to pin the pass clock; zero means time.Now().
	Now time.Time
	// Limit bounds how many runs one pass may trim (default 128) so a sweep
	// stays inside one worker tick.
	Limit int
}

// EventRetentionReport summarizes one retention pass for logs/metrics.
type EventRetentionReport struct {
	RunsConsidered        int
	RunsTrimmed           int
	RunsSkippedTombstoned int
	SnapshotsWritten      int
	EventsTrimmed         int64
}

// ApplyEventRetention performs one W35 time-based event retention pass.
//
// Eligible for trimming: events older than the retention cutoff of runs that
// already reached a terminal status AND are not protected below. Everything
// else is fail-closed:
//
//   - runs still in a non-terminal (active) status are never touched;
//   - runs whose session carries a live (non-purged) W33 deletion tombstone
//     are skipped entirely — their rows belong to the W33 evidence-fenced
//     purge path (ClaimCleanup/RunCleanupPurgeOnce), and deleting usage or
//     replay evidence out from under that fence could deadlock settlement;
//   - the protected families in eventRetentionProtectedSQL are never deleted
//     by this global pass.
//
// For every trimmed run the pass first appends a "retention.trimmed" summary
// event (trimmed count, first/last seq, per-type histogram, cutoff) and then
// deletes the stale prefix — snapshot and delete commit in the SAME
// transaction, so a crash can never leave events deleted without their
// snapshot. The summary survives future passes, replacing the deleted prefix
// as the durable audit record.
func (s *AgentRunStore) ApplyEventRetention(ctx context.Context, opts EventRetentionOptions) (EventRetentionReport, error) {
	retention := opts.Retention
	if retention == 0 {
		retention = DefaultEventRetention
	}
	if retention < 0 {
		return EventRetentionReport{}, agentruntime.ErrConflict
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 128
	}
	cutoff := now.Add(-retention).UTC()

	// Candidates must actually have trimmable work, otherwise fully
	// protected runs would permanently occupy the head of the ordering and
	// starve later runs under the pass limit.
	var candidates []struct {
		TenantID   uint64
		RunID      string
		Tombstoned bool
	}
	err := s.db.WithContext(ctx).Raw(`
		SELECT r.tenant_id, r.run_id,
		       EXISTS(SELECT 1 FROM execution_cleanup c
		              WHERE c.tenant_id = r.tenant_id AND c.session_id = r.session_id
		                AND c.state <> 'purged') AS tombstoned
		FROM agent_runs r
		WHERE r.status IN ? AND r.updated_at < ?
		  AND EXISTS (SELECT 1 FROM agent_run_events e
		              WHERE e.tenant_id = r.tenant_id AND e.run_id = r.run_id
		                AND e.created_at < ? AND NOT `+eventRetentionProtectedSQL+`)
		ORDER BY r.updated_at, r.tenant_id, r.run_id
		LIMIT ?`, eventRetentionTerminalStatuses, cutoff, cutoff, limit).Scan(&candidates).Error
	if err != nil {
		return EventRetentionReport{}, err
	}
	report := EventRetentionReport{RunsConsidered: len(candidates)}
	for _, candidate := range candidates {
		if candidate.Tombstoned {
			report.RunsSkippedTombstoned++
			continue
		}
		trimmed, snapshotted, err := s.trimRunEventsBeforeCutoff(ctx, candidate.TenantID, candidate.RunID, cutoff)
		if err != nil {
			return report, err
		}
		if snapshotted {
			report.SnapshotsWritten++
		}
		if trimmed > 0 {
			report.RunsTrimmed++
			report.EventsTrimmed += trimmed
		}
	}
	return report, nil
}

// trimRunEventsBeforeCutoff snapshots-then-trims one run inside a single
// transaction. The conditional no-op update on the run row takes the row lock
// and rechecks eligibility; if another writer finalized or reclaimed the run
// in between, the run is skipped (0, false, nil), not failed.
func (s *AgentRunStore) trimRunEventsBeforeCutoff(ctx context.Context, tenantID uint64, runID string, cutoff time.Time) (int64, bool, error) {
	var trimmed int64
	var snapshotted bool
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		recheck := tx.Table("agent_runs").Where(
			"tenant_id = ? AND run_id = ? AND status IN ? AND updated_at < ?",
			tenantID, runID, eventRetentionTerminalStatuses, cutoff,
		).UpdateColumn("revision", gorm.Expr("revision"))
		if recheck.Error != nil {
			return recheck.Error
		}
		if recheck.RowsAffected != 1 {
			return nil
		}
		var stats []struct {
			Seq       int64
			EventType string
		}
		if err := tx.Table("agent_run_events").Select("seq, event_type").
			Where("tenant_id = ? AND run_id = ? AND created_at < ? AND NOT "+eventRetentionProtectedSQL,
				tenantID, runID, cutoff).
			Order("seq").Scan(&stats).Error; err != nil {
			return err
		}
		if len(stats) == 0 {
			return nil
		}
		typeCounts := make(map[string]int, len(stats))
		for _, evt := range stats {
			typeCounts[evt.EventType]++
		}
		payload, err := json.Marshal(map[string]any{
			"cutoff":        cutoff.Format(time.RFC3339),
			"trimmed_count": len(stats),
			"first_seq":     stats[0].Seq,
			"last_seq":      stats[len(stats)-1].Seq,
			"type_counts":   typeCounts,
		})
		if err != nil {
			return err
		}
		// Snapshot first, trim second — one transaction, one outcome.
		if err := appendRunEventLocked(tx, agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: tenantID, RunID: runID}}, "retention.trimmed", string(payload)); err != nil {
			return err
		}
		deleted := tx.Table("agent_run_events").Where(
			"tenant_id = ? AND run_id = ? AND created_at < ? AND NOT "+eventRetentionProtectedSQL,
			tenantID, runID, cutoff).Delete(&agentRunEventRow{})
		if deleted.Error != nil {
			return deleted.Error
		}
		trimmed = deleted.RowsAffected
		snapshotted = true
		return nil
	})
	return trimmed, snapshotted, err
}

// NotificationEventKind is the single event-to-notification policy. Streaming
// token events intentionally return false; only durable user-actionable or
// terminal outcomes can create a mobile notification intent.
func NotificationEventKind(eventType string) (string, bool) {
	switch eventType {
	case "run_completed", "execution.succeeded", "completed", "succeeded":
		return "completed", true
	case "run_failed", "execution.failed", "failed", "error":
		return "failed", true
	case "interaction_requested", "waiting_user", "approval_requested":
		return "interaction_requested", true
	case "budget_exhausted", "budget_exceeded":
		return "budget_exhausted", true
	default:
		return "", false
	}
}
