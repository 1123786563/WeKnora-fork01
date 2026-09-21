package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

var (
	ErrSemanticModelDenied   = errors.New("semantic_model_denied")
	ErrSemanticModelInFlight = errors.New("semantic_model_in_flight")
	ErrSemanticModelUnknown  = errors.New("semantic_model_unknown")
)

type SemanticModelResult struct {
	Text                 string  `json:"text"`
	PromptTokens         *int64  `json:"prompt_tokens,omitempty"`
	CompletionTokens     *int64  `json:"completion_tokens,omitempty"`
	ProviderRequestID    *string `json:"provider_request_id,omitempty"`
	ResolvedModelVersion *string `json:"resolved_model_version,omitempty"`
}
type semanticCapability struct {
	owner                                         uint64
	modelID, funding, priceVersion, runID, callID string
	upper                                         int64
	deadline                                      typesTime
}

// typesTime is kept local so the gateway's construction API does not expose
// any provider concern; it aliases time.Time below.
type typesTime = time.Time

type SemanticModelGateway struct {
	issuer      interfaces.SemanticModelCapabilityIssuer
	models      interfaces.ModelService
	scope       SemanticScopeResolver
	budget      *SemanticModelBudgetAdapter
	invocations interfaces.SemanticModelInvocationStore
}

func NewSemanticModelGateway(issuer interfaces.SemanticModelCapabilityIssuer, models interfaces.ModelService, scope SemanticScopeResolver, budget *SemanticModelBudgetAdapter, invocations interfaces.SemanticModelInvocationStore) *SemanticModelGateway {
	return &SemanticModelGateway{issuer: issuer, models: models, scope: scope, budget: budget, invocations: invocations}
}

func (g *SemanticModelGateway) Invoke(ctx context.Context, wire types.SemanticModelWireRequest) (SemanticModelResult, error) {
	if g == nil || g.issuer == nil || g.models == nil || g.scope == nil || g.budget == nil || g.invocations == nil {
		return SemanticModelResult{}, ErrSemanticModelDenied
	}
	c, err := g.issuer.Verify(ctx, wire.CapabilityToken)
	if err != nil {
		return SemanticModelResult{}, ErrSemanticModelDenied
	}
	if c.Deadline.IsZero() || !c.Deadline.After(time.Now()) {
		return SemanticModelResult{}, ErrSemanticModelDenied
	}
	ownerCtx := types.WithExecutionTenant(ctx, c.OwnerTenantID)
	ownerCtx, cancel := context.WithDeadline(ownerCtx, c.Deadline)
	defer cancel()
	snapshot, err := g.scope.Resolve(ownerCtx, c.ScopeRef)
	if err != nil || snapshot.Scope.TenantID != c.OwnerTenantID || snapshot.Scope.KBID != c.KBID || snapshot.ScopeHash != c.ScopeHash {
		return SemanticModelResult{}, ErrSemanticModelDenied
	}
	if err := validateSemanticWire(wire, c); err != nil {
		return SemanticModelResult{}, ErrSemanticModelDenied
	}
	hash, err := semanticWireHash(wire)
	if err != nil {
		return SemanticModelResult{}, ErrSemanticModelDenied
	}
	claim, err := g.invocations.Claim(ownerCtx, c, hash)
	if err != nil {
		return SemanticModelResult{}, err
	}
	switch claim.Disposition {
	case types.SemanticModelInvocationCompletedReplay:
		var out SemanticModelResult
		if json.Unmarshal(claim.Result.Result, &out) != nil {
			return SemanticModelResult{}, ErrSemanticModelDenied
		}
		return out, nil
	case types.SemanticModelInvocationInFlight:
		return SemanticModelResult{}, ErrSemanticModelInFlight
	case types.SemanticModelInvocationUnknown:
		return SemanticModelResult{}, ErrSemanticModelUnknown
	case types.SemanticModelInvocationClaimedNew:
	default:
		return SemanticModelResult{}, ErrSemanticModelDenied
	}
	cap := semanticCapability{owner: c.OwnerTenantID, modelID: c.ModelID, funding: c.Funding, priceVersion: c.PriceVersion, runID: c.RunID, callID: c.CallID, upper: c.PerCallUpperMicro, deadline: c.Deadline}
	reservation, err := g.budget.reserve(ownerCtx, cap)
	if err != nil {
		_ = g.invocations.FailBeforeDispatch(ownerCtx, c)
		return SemanticModelResult{}, semanticBudgetFailure(err)
	}
	model, err := g.models.GetChatModel(ownerCtx, c.ModelID)
	if err != nil {
		if releaseErr := g.budget.release(ownerCtx, cap, reservation); releaseErr == nil {
			_ = g.invocations.FailBeforeDispatch(ownerCtx, c)
		} else {
			_ = g.invocations.MarkUnknown(ownerCtx, c)
		}
		return SemanticModelResult{}, err
	}
	// The two durable markers immediately precede I/O. Any failure is
	// ambiguous enough to retain both quota and platform hold.
	if err := g.invocations.MarkDispatched(ownerCtx, c); err != nil {
		g.markUnknown(c)
		return SemanticModelResult{}, err
	}
	if err := g.budget.markDispatched(ownerCtx, cap, reservation); err != nil {
		g.markUnknown(c)
		return SemanticModelResult{}, err
	}
	resp, err := model.Chat(ownerCtx, semanticMessages(wire.Messages), &chat.ChatOptions{MaxCompletionTokens: int(wire.Parameters.MaxOutputTokens), Temperature: semanticTemperature(wire.Parameters.Temperature)})
	if err != nil || resp == nil || ownerCtx.Err() != nil {
		g.markUnknown(c)
		return SemanticModelResult{}, ErrSemanticModelUnknown
	}
	in, out, ok := semanticUsage(resp, c)
	if !ok {
		g.markUnknown(c)
		return SemanticModelResult{}, ErrSemanticModelUnknown
	}
	result := SemanticModelResult{Text: resp.Content, PromptTokens: &in, CompletionTokens: &out}
	if err := g.budget.finish(ownerCtx, cap, reservation, in, out); err != nil {
		g.markUnknown(c)
		return SemanticModelResult{}, err
	}
	stored, _ := json.Marshal(result)
	if err := g.invocations.Complete(ownerCtx, c, types.SemanticModelInvocationResult{Result: stored, InputTokens: in, OutputTokens: out}); err != nil {
		g.markUnknown(c)
		return SemanticModelResult{}, err
	}
	return result, nil
}

// markUnknown never inherits a cancelled request context: reconciliation is
// an independent, bounded durability attempt after outbound I/O may have run.
func (g *SemanticModelGateway) markUnknown(c types.SemanticModelCapability) {
	cleanup, cancel := context.WithTimeout(types.WithExecutionTenant(context.Background(), c.OwnerTenantID), 5*time.Second)
	defer cancel()
	_ = g.invocations.MarkUnknown(cleanup, c)
}

func validateSemanticWire(w types.SemanticModelWireRequest, c types.SemanticModelCapability) error {
	if len(w.Messages) == 0 || w.Parameters.MaxOutputTokens <= 0 || w.Parameters.MaxOutputTokens > c.MaxOutputTokensPerCall {
		return errors.New("bounds")
	}
	var n int64
	for _, m := range w.Messages {
		if strings.TrimSpace(m.Role) == "" || strings.TrimSpace(m.Content) == "" {
			return errors.New("message")
		}
		n += int64(len(m.Content)+3) / 4
	}
	if n <= 0 || n > c.MaxInputTokensPerCall {
		return errors.New("input")
	}
	return nil
}
func semanticWireHash(w types.SemanticModelWireRequest) (string, error) {
	b, e := json.Marshal(struct {
		Messages   []types.SemanticModelMessage  `json:"messages"`
		Parameters types.SemanticModelParameters `json:"parameters"`
	}{w.Messages, w.Parameters})
	if e != nil {
		return "", e
	}
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:]), nil
}
func semanticMessages(in []types.SemanticModelMessage) []chat.Message {
	out := make([]chat.Message, len(in))
	for i, m := range in {
		out[i] = chat.Message{Role: m.Role, Content: m.Content}
	}
	return out
}
func semanticTemperature(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
func semanticUsage(resp *types.ChatResponse, c types.SemanticModelCapability) (int64, int64, bool) {
	in, out, total := int64(resp.Usage.PromptTokens), int64(resp.Usage.CompletionTokens), int64(resp.Usage.TotalTokens)
	if in <= 0 || out <= 0 || total <= 0 || in > c.MaxInputTokensPerCall || out > c.MaxOutputTokensPerCall || in > math.MaxInt64-out {
		return in, out, false
	}
	return in, out, total == in+out
}
