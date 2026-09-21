package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/models/asr"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/models/embedding"
	"github.com/Tencent/WeKnora/internal/models/rerank"
	"github.com/Tencent/WeKnora/internal/models/vlm"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// This catches an authorization regression that resolves owner credentials
// before the signed capability and fresh A01 scope have been accepted.
func TestSemanticModelDeniedBeforeModelResolution(t *testing.T) {
	models := &semanticGatewayModels{}
	gateway := NewSemanticModelGateway(semanticGatewayIssuer{err: errors.New("bad capability")}, models, semanticGatewayScope{}, NewSemanticModelBudgetAdapter(nil, nil, nil), semanticGatewayInvocations{})
	_, err := gateway.Invoke(context.Background(), types.SemanticModelWireRequest{CapabilityToken: "bad"})
	require.ErrorIs(t, err, ErrSemanticModelDenied)
	require.Zero(t, models.chatCalls)
}

// This catches a replay path that takes a fresh quota/budget hold or reaches
// provider credentials after an identical completed invocation already exists.
func TestSemanticModelCompletedReplaySkipsModelResolution(t *testing.T) {
	result := []byte(`{"text":"saved"}`)
	models := &semanticGatewayModels{}
	gateway := NewSemanticModelGateway(semanticGatewayIssuer{cap: semanticGatewayCapability()}, models, semanticGatewayScope{}, NewSemanticModelBudgetAdapter(nil, nil, nil), semanticGatewayInvocations{claim: types.SemanticModelInvocationClaim{Disposition: types.SemanticModelInvocationCompletedReplay, Result: types.SemanticModelInvocationResult{Result: result}}})
	got, err := gateway.Invoke(context.Background(), semanticGatewayWire())
	require.NoError(t, err)
	require.Equal(t, "saved", got.Text)
	require.Zero(t, models.chatCalls)
}

func semanticGatewayCapability() types.SemanticModelCapability {
	return types.SemanticModelCapability{OwnerTenantID: 7, KBID: "kb", ScopeRef: "scope", ScopeHash: "hash", ModelID: "model", Funding: "byok", PriceVersion: "pv", RunID: "run", CallID: "call", MaxInputTokensPerCall: 20, MaxOutputTokensPerCall: 20, PerCallUpperMicro: 10, Deadline: time.Now().Add(time.Hour)}
}
func semanticGatewayWire() types.SemanticModelWireRequest {
	return types.SemanticModelWireRequest{CapabilityToken: "ok", Messages: []types.SemanticModelMessage{{Role: "user", Content: "hello"}}, Parameters: types.SemanticModelParameters{MaxOutputTokens: 5}}
}

type semanticGatewayIssuer struct {
	cap types.SemanticModelCapability
	err error
}

func (s semanticGatewayIssuer) Issue(context.Context, string, string) (types.SemanticModelIssuedCapability, error) {
	return types.SemanticModelIssuedCapability{}, errors.New("unused")
}
func (s semanticGatewayIssuer) Verify(context.Context, string) (types.SemanticModelCapability, error) {
	return s.cap, s.err
}

type semanticGatewayScope struct{}

func (semanticGatewayScope) Resolve(context.Context, string) (SemanticScopeSnapshot, error) {
	return SemanticScopeSnapshot{Scope: types.SemanticScopeKey{TenantID: 7, KBID: "kb"}, ScopeHash: "hash"}, nil
}

type semanticGatewayInvocations struct {
	claim types.SemanticModelInvocationClaim
}

func (s semanticGatewayInvocations) EnsureRun(context.Context, types.SemanticModelCapability) error {
	return nil
}
func (s semanticGatewayInvocations) Claim(context.Context, types.SemanticModelCapability, string) (types.SemanticModelInvocationClaim, error) {
	return s.claim, nil
}
func (semanticGatewayInvocations) MarkDispatched(context.Context, types.SemanticModelCapability) error {
	return nil
}
func (semanticGatewayInvocations) Complete(context.Context, types.SemanticModelCapability, types.SemanticModelInvocationResult) error {
	return nil
}
func (semanticGatewayInvocations) FailBeforeDispatch(context.Context, types.SemanticModelCapability) error {
	return nil
}
func (semanticGatewayInvocations) MarkUnknown(context.Context, types.SemanticModelCapability) error {
	return nil
}

type semanticGatewayModels struct{ chatCalls int }

func (*semanticGatewayModels) CreateModel(context.Context, *types.Model) error { return nil }
func (*semanticGatewayModels) GetModelByID(context.Context, string) (*types.Model, error) {
	return nil, nil
}
func (*semanticGatewayModels) ListModels(context.Context) ([]*types.Model, error) { return nil, nil }
func (*semanticGatewayModels) UpdateModel(context.Context, *types.Model) error    { return nil }
func (*semanticGatewayModels) DeleteModel(context.Context, string) error          { return nil }
func (*semanticGatewayModels) UpdateModelCredentials(context.Context, string, *string, *string) (*types.Model, error) {
	return nil, nil
}
func (*semanticGatewayModels) ClearModelCredential(context.Context, string, string) error { return nil }
func (*semanticGatewayModels) GetEmbeddingModel(context.Context, string) (embedding.Embedder, error) {
	return nil, nil
}
func (*semanticGatewayModels) GetEmbeddingModelForTenant(context.Context, string, uint64) (embedding.Embedder, error) {
	return nil, nil
}
func (*semanticGatewayModels) GetRerankModel(context.Context, string) (rerank.Reranker, error) {
	return nil, nil
}
func (s *semanticGatewayModels) GetChatModel(context.Context, string) (chat.Chat, error) {
	s.chatCalls++
	return nil, nil
}
func (*semanticGatewayModels) GetVLMModel(context.Context, string) (vlm.VLM, error) { return nil, nil }
func (*semanticGatewayModels) GetASRModel(context.Context, string) (asr.ASR, error) { return nil, nil }
