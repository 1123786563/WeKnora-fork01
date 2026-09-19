package interfaces

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

// UsageAggregate is one aggregated usage row: per UTC day and model, with the
// user dimension present on the all-users and export shapes (WindowStart and
// Model are empty on export rows).
type UsageAggregate struct {
	WindowStart      string `json:"window_start"`
	Model            string `json:"model"`
	UserID           string `json:"user_id,omitempty"`
	InputTokens      int64  `json:"input_tokens"`
	OutputTokens     int64  `json:"output_tokens"`
	CacheReadTokens  int64  `json:"cache_read_tokens"`
	CacheWriteTokens int64  `json:"cache_write_tokens"`
	CostMicrocredits int64  `json:"cost_microcredits"`
}

// UsageRecorderService writes one chat turn's terminal token usage into the
// caller's daily bucket. Empty identity (tenant 0, empty user or model) or a
// nil usage block is silently skipped — usage accounting must never break or
// block the chat path.
type UsageRecorderService interface {
	RecordChatTurn(ctx context.Context, tenantID uint64, userID, model string, usage *types.TokenUsage) error
}

// UsageRepository persists daily token/cost buckets and aggregates them.
// All aggregation windows are half-open: [from, to) over UTC days.
type UsageRepository interface {
	// AddUsage upserts one bucket row; a conflict on the dimension tuple
	// (tenant, user, day, model, flow) adds the deltas onto the stored
	// bucket — a legal accumulation, not a no-op.
	AddUsage(ctx context.Context, u *types.UserUsage) error
	// AggregateByUser returns one row per UTC day x model for one user.
	AggregateByUser(ctx context.Context, tenantID uint64, userID string, from, to time.Time) ([]UsageAggregate, error)
	// AggregateAllUsers returns one row per UTC day x model x user, paged.
	AggregateAllUsers(ctx context.Context, tenantID uint64, from, to time.Time, limit, offset int) ([]UsageAggregate, error)
	// ExportRows returns per-user totals over the whole window.
	ExportRows(ctx context.Context, tenantID uint64, from, to time.Time) ([]UsageAggregate, error)
}
