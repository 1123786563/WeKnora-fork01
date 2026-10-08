package repository

import (
	"context"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/workbench"
)

// TerminalLogEventType is the product event type that carries one chunk of
// read-only terminal output. Producers (the Paseo bridge) persist it through
// the authenticated source-event callback; observationType already admits the
// "tool." prefix, so ingestion needs no change.
const TerminalLogEventType = "tool.terminal"

const maxTerminalLogPageSize = 256

// ReadRunTerminalEvents pages a run's read-only terminal log: the sparse
// subsequence of TerminalLogEventType events ordered by product seq. Unlike
// ReadRunEvents, gaps between terminal chunks are expected (other event
// types interleave), so contiguity is NOT enforced and there is no
// cursor-expiry error — `after` simply resumes from the last delivered seq.
// The read is fully parameter-bound; tenant and run always come from the
// caller's ownership predicate, never from the request path alone.
func (s *AgentRunSnapshotRepository) ReadRunTerminalEvents(ctx context.Context, key agentruntime.RunKey, after int64, limit int) ([]workbench.ExecutionEvent, error) {
	if s == nil || s.db == nil || key.TenantID == 0 || key.RunID == "" || after < 0 {
		return nil, agentruntime.ErrNotFound
	}
	if limit <= 0 || limit > maxTerminalLogPageSize {
		limit = maxTerminalLogPageSize
	}
	var rows []snapshotEventRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND run_id = ? AND event_type = ? AND seq > ?", key.TenantID, key.RunID, TerminalLogEventType, after).
		Order("seq ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	events := make([]workbench.ExecutionEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, executionEventFor(row.RunID, row.AttemptID, row.Seq, row.EventType, row.Payload, row.CreatedAt))
	}
	return events, nil
}
