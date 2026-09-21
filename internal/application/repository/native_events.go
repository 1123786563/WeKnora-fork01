package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/nativecontract"
)

func (c *NativeCommitCoordinator) Read(ctx context.Context, scope nativecontract.Scope, run nativecontract.RunIdentity, cursor string, limit int) (nativecontract.EventPage, error) {
	if scope.TenantID == 0 || scope.TenantID != run.TenantID || run.RunID == "" {
		return nativecontract.EventPage{}, typedFailure(nativecontract.ErrNotFound, "run is outside the event scope")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}
	var min int64
	if err := c.db.WithContext(ctx).Table("native_agent_events").Select("COALESCE(MIN(sequence), 0)").Where("tenant_id=? AND run_id=?", run.TenantID, run.RunID).Scan(&min).Error; err != nil {
		return nativecontract.EventPage{}, err
	}
	after := int64(0)
	if cursor != "" {
		parsed, err := strconv.ParseInt(cursor, 10, 64)
		if err != nil || parsed < 0 {
			return nativecontract.EventPage{}, typedFailure(nativecontract.ErrCursor, "event cursor is invalid")
		}
		after = parsed
		if min > 0 && after < min {
			return nativecontract.EventPage{}, typedFailure(nativecontract.ErrCursor, "event cursor has expired")
		}
	}
	var rows []struct {
		Sequence int64
		Payload  []byte
	}
	if err := c.db.WithContext(ctx).Table("native_agent_events").Select("sequence, payload").Where("tenant_id=? AND run_id=? AND sequence > ?", run.TenantID, run.RunID, after).Order("sequence ASC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nativecontract.EventPage{}, err
	}
	page := nativecontract.EventPage{MinSequence: min}
	for i, row := range rows {
		if i == limit {
			page.NextCursor = fmt.Sprint(rows[i-1].Sequence)
			break
		}
		var event nativecontract.BusinessEvent
		if err := json.Unmarshal(row.Payload, &event); err != nil {
			return nativecontract.EventPage{}, typedFailure(nativecontract.ErrStore, "stored business event is corrupt")
		}
		if event.Sequence != fmt.Sprint(row.Sequence) {
			return nativecontract.EventPage{}, typedFailure(nativecontract.ErrStore, "stored business event sequence is corrupt")
		}
		page.Events = append(page.Events, event)
	}
	if len(rows) <= limit && len(page.Events) > 0 {
		page.NextCursor = fmt.Sprint(page.Events[len(page.Events)-1].Sequence)
	}
	return page, nil
}
