package chat

import (
	"context"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

// logUsage emits the standard "[LLM Usage]" line shared by every Chat
// implementation. It is a no-op when usage is nil so callers can pass through
// optional usage blocks without guarding at each call site.
func logUsage(ctx context.Context, model string, u *types.TokenUsage) {
	if u == nil {
		return
	}
	purpose, prefixFingerprint := types.LLMCallMetadataFromContext(ctx)
	logger.Infof(ctx,
		"[LLM Usage] model=%s, purpose=%s, prompt_prefix=%s, prompt_tokens=%d, completion_tokens=%d, "+
			"total_tokens=%d, cached_tokens=%d, cache_read_tokens=%d, cache_write_tokens=%d, "+
			"cache_miss_tokens=%d, cache_hit_rate=%.1f%%, cache_reported=%t, cache_status=%s%s",
		model, purpose, prefixFingerprint, u.PromptTokens, u.CompletionTokens, u.TotalTokens,
		u.CachedTokens, u.CacheReadTokens, u.CacheWriteTokens, u.CacheMissTokens,
		u.PromptCacheHitRate(), u.CacheReported, u.CacheStatus, usageAttribution(ctx))
}

// usageAttribution renders the ", session_id=…, principal=…" suffix that
// attributes a usage line to the session and terminal principal that
// triggered the call. Calls that run outside a session or without a resolved
// principal (document parsing, title generation, background jobs) render an
// empty suffix, keeping their lines byte-identical to before.
func usageAttribution(ctx context.Context) string {
	var b strings.Builder
	if sessionID, ok := types.SessionIDFromContext(ctx); ok && sessionID != "" {
		b.WriteString(", session_id=")
		b.WriteString(sessionID)
	}
	if principal, ok := types.PrincipalFromContext(ctx); ok {
		b.WriteString(", principal=")
		b.WriteString(principal.Type)
		b.WriteString(":")
		b.WriteString(principal.ID)
	}
	return b.String()
}

// UsageFactInput carries the identity of one physical billable call, per the
// Craft run/delegation/call linkage: a child run's calls carry both their own
// RunID/CallID/AttemptID and the DelegationID that spawned them, so a parent
// rollup can be marked display_only while each child settles its own facts.
// Funding is ALWAYS resolved server-side before dispatch; the adapter never
// invents a funding value.
type UsageFactInput struct {
	TenantID     uint64
	RunID        string
	DelegationID string
	CallID       string
	AttemptID    string
	Funding      string
	Service      string
	PriceVersion string
	// ParentAggregate marks a fact that only rolls up child calls: it is
	// adapted as display_only and can never settle (no double count).
	ParentAggregate bool
}

// UsageFactFromTokenUsage adapts a provider-reported TokenUsage into a
// commercial UsageFact. It adapts raw usage events for settlement WITHOUT
// replacing the display TokenUsage the engine already accumulates: display
// semantics stay on types.TokenUsage; this mapping is additive.
func UsageFactFromTokenUsage(u types.TokenUsage, in UsageFactInput, now time.Time) commercial.UsageFact {
	status := commercial.UsageStatusFinal
	if in.ParentAggregate {
		status = commercial.UsageStatusDisplayOnly
	}
	return commercial.UsageFact{
		TenantID:     in.TenantID,
		RunID:        in.RunID,
		DelegationID: in.DelegationID,
		CallID:       in.CallID,
		AttemptID:    in.AttemptID,
		Funding:      in.Funding,
		Service:      in.Service,
		PriceVersion: in.PriceVersion,
		Revision:     1,
		OccurredAt:   now,
		Dimensions: map[string]int64{
			commercial.DimensionModel:  int64(u.PromptTokens + u.CompletionTokens),
			commercial.DimensionOutput: int64(u.CompletionTokens),
		},
		Status: status,
	}
}
