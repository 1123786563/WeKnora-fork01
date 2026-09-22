package service

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/commercial/usage"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// UsageRecorder writes one chat turn's terminal token usage into the caller's
// user_usage daily bucket (SP12). It is the single ingestion point for the
// chat flow; craft traffic records through its own ledger. The service is
// deliberately fail-soft at the edges: empty identity or a nil usage block is
// silently skipped, and pricing comes from the injected ModelRates so an
// unrated deployment records tokens with zero cost rather than blocking chat.
type UsageRecorder struct {
	repo  interfaces.UsageRepository
	rates usage.ModelRates
	// now is the clock used to derive the UTC bucket day. Injectable for
	// deterministic tests; production uses time.Now.
	now func() time.Time
}

// NewUsageRecorderService assembles the recorder over the bucket repository
// and the model rate table (nil/empty rates mean "record without pricing").
func NewUsageRecorderService(repo interfaces.UsageRepository, rates usage.ModelRates) *UsageRecorder {
	if rates == nil {
		rates = usage.ModelRates{}
	}
	return &UsageRecorder{repo: repo, rates: rates, now: time.Now}
}

var _ interfaces.UsageRecorderService = (*UsageRecorder)(nil)

// RecordChatTurn accumulates one finished chat turn into the (tenant, user,
// UTC day, model, chat) bucket. A zero tenant, an empty user or model, or a
// nil usage block returns nil without touching the repository — incomplete
// attribution is skipped, not guessed. The legacy CachedTokens alias backs up
// CacheReadTokens when a provider only fills the old field. Any repository
// error is returned to the caller, which logs it and moves on: accounting
// must never fail the chat path.
func (s *UsageRecorder) RecordChatTurn(ctx context.Context, tenantID uint64, userID, model string, usage *types.TokenUsage) error {
	if s == nil || s.repo == nil {
		return nil
	}
	if tenantID == 0 || userID == "" || model == "" || usage == nil {
		return nil
	}
	cacheRead := usage.CacheReadTokens
	if cacheRead == 0 && usage.CachedTokens > 0 {
		cacheRead = usage.CachedTokens
	}
	day := s.now().UTC()
	return s.repo.AddUsage(ctx, &types.UserUsage{
		TenantID:         tenantID,
		UserID:           userID,
		WindowStart:      time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC),
		Model:            model,
		Flow:             types.UsageFlowChat,
		InputTokens:      int64(usage.PromptTokens),
		OutputTokens:     int64(usage.CompletionTokens),
		CacheReadTokens:  int64(cacheRead),
		CacheWriteTokens: int64(usage.CacheWriteTokens),
		CostMicrocredits: s.rates.CostMicro(model, int64(usage.PromptTokens), int64(usage.CompletionTokens)),
	})
}
