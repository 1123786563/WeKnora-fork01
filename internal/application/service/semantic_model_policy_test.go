package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestSemanticModelPolicyPutRequiresResolvedCommercialPolicy(t *testing.T) {
	repo := &semanticPolicyMemoryRepository{}
	svc := NewSemanticModelPolicyService(repo, semanticPolicyKB{kb: &types.KnowledgeBase{ID: "kb-1", TenantID: 7}}, semanticPolicyModel{model: &types.Model{ID: "chat-1", TenantID: 7, Type: types.ModelTypeKnowledgeQA, Status: types.ModelStatusActive}}, nil)
	_, err := svc.Put(context.Background(), SemanticModelPolicyScope{TenantID: 7, KBID: "kb-1", ActorID: "owner"}, SemanticModelPolicyInput{ModelCallsEnabled: true, ModelID: "chat-1", MaxInputTokensPerCall: 10, MaxOutputTokensPerCall: 10, MaxCallsPerTask: 1, MaxInputTokensPerTask: 10, MaxOutputTokensPerTask: 10})
	require.ErrorIs(t, err, ErrSemanticModelPricingUnavailable)
	require.Empty(t, repo.policies)
}

func TestSemanticModelPolicyPutPersistsOnlyServerResolvedPricing(t *testing.T) {
	repo := &semanticPolicyMemoryRepository{}
	resolver := func(context.Context, uint64, *types.Model) (SemanticModelPricing, error) {
		return SemanticModelPricing{Funding: domain.FundingPlatform, PriceVersion: "pv-immutable", Rates: domain.PriceVersionRates{Version: "pv-immutable", Rates: map[string]domain.DimensionRate{domain.DimensionModel: {RateMicro: 1, Units: 1}}}, TaskUpperMicro: int64ptrService(99)}, nil
	}
	svc := NewSemanticModelPolicyService(repo, semanticPolicyKB{kb: &types.KnowledgeBase{ID: "kb-1", TenantID: 7}}, semanticPolicyModel{model: &types.Model{ID: "chat-1", TenantID: 7, Type: types.ModelTypeKnowledgeQA, Status: types.ModelStatusActive}}, resolver)
	got, err := svc.Put(context.Background(), SemanticModelPolicyScope{TenantID: 7, KBID: "kb-1", ActorID: "owner"}, SemanticModelPolicyInput{ModelCallsEnabled: true, ModelID: "chat-1", MaxInputTokensPerCall: 10, MaxOutputTokensPerCall: 20, MaxCallsPerTask: 2, MaxInputTokensPerTask: 20, MaxOutputTokensPerTask: 40})
	require.NoError(t, err)
	require.Equal(t, "pv-immutable", got.PriceVersion)
	require.Equal(t, domain.FundingPlatform, got.Funding)
	require.Equal(t, int64(30), got.PerCallUpperMicro)
	require.Equal(t, int64(99), *got.TaskUpperMicro)
}

func TestSemanticModelPolicyPutRejectsInvalidOwnerModelAndCaps(t *testing.T) {
	repo := &semanticPolicyMemoryRepository{}
	resolver := func(context.Context, uint64, *types.Model) (SemanticModelPricing, error) {
		return SemanticModelPricing{}, errors.New("unexpected")
	}
	svc := NewSemanticModelPolicyService(repo, semanticPolicyKB{kb: &types.KnowledgeBase{ID: "kb-1", TenantID: 7}}, semanticPolicyModel{model: &types.Model{ID: "chat-foreign", TenantID: 8, Type: types.ModelTypeKnowledgeQA, Status: types.ModelStatusActive}}, resolver)
	_, err := svc.Put(context.Background(), SemanticModelPolicyScope{TenantID: 7, KBID: "kb-1", ActorID: "owner"}, SemanticModelPolicyInput{ModelCallsEnabled: true, ModelID: "chat-foreign", MaxInputTokensPerCall: 1, MaxOutputTokensPerCall: 1, MaxCallsPerTask: 1, MaxInputTokensPerTask: 1, MaxOutputTokensPerTask: 1})
	require.Error(t, err)
	require.Empty(t, repo.policies)
}

func TestSemanticModelPolicyPutRejectsNonChatModel(t *testing.T) {
	repo := &semanticPolicyMemoryRepository{}
	resolver := func(context.Context, uint64, *types.Model) (SemanticModelPricing, error) {
		return SemanticModelPricing{}, errors.New("must not price non-chat model")
	}
	svc := NewSemanticModelPolicyService(repo, semanticPolicyKB{kb: &types.KnowledgeBase{ID: "kb-1", TenantID: 7}}, semanticPolicyModel{model: &types.Model{ID: "embedding-1", TenantID: 7, Type: types.ModelTypeEmbedding, Status: types.ModelStatusActive}}, resolver)
	_, err := svc.Put(context.Background(), SemanticModelPolicyScope{TenantID: 7, KBID: "kb-1", ActorID: "owner"}, SemanticModelPolicyInput{ModelCallsEnabled: true, ModelID: "embedding-1", MaxInputTokensPerCall: 1, MaxOutputTokensPerCall: 1, MaxCallsPerTask: 1, MaxInputTokensPerTask: 1, MaxOutputTokensPerTask: 1})
	require.ErrorIs(t, err, ErrSemanticModelPolicyInvalid)
	require.Empty(t, repo.policies)
}

func TestSemanticModelPolicyPutCanDisableWhenPricingIsUnavailable(t *testing.T) {
	repo := &semanticPolicyMemoryRepository{policies: []repository.SemanticModelPolicy{{TenantID: 7, KBID: "kb-1", ModelCallsEnabled: true, ModelID: "chat-1", Funding: domain.FundingPlatform, PriceVersion: "pv-1", MaxInputTokensPerCall: 10, MaxOutputTokensPerCall: 10, MaxCallsPerTask: 1, MaxInputTokensPerTask: 10, MaxOutputTokensPerTask: 10, PerCallUpperMicro: 20, UpdatedBy: "owner"}}}
	svc := NewSemanticModelPolicyService(repo, semanticPolicyKB{kb: &types.KnowledgeBase{ID: "kb-1", TenantID: 7}}, nil, nil)
	got, err := svc.Put(context.Background(), SemanticModelPolicyScope{TenantID: 7, KBID: "kb-1", ActorID: "owner"}, SemanticModelPolicyInput{ModelCallsEnabled: false})
	require.NoError(t, err)
	require.False(t, got.ModelCallsEnabled)
	require.Equal(t, uint64(2), got.PolicyVersion)
}

func int64ptrService(v int64) *int64 { return &v }

type semanticPolicyMemoryRepository struct {
	policies []repository.SemanticModelPolicy
}

func (r *semanticPolicyMemoryRepository) Get(context.Context, uint64, string) (*repository.SemanticModelPolicy, error) {
	if len(r.policies) == 0 {
		return nil, repository.ErrSemanticModelPolicyNotFound
	}
	p := r.policies[len(r.policies)-1]
	return &p, nil
}
func (r *semanticPolicyMemoryRepository) Put(_ context.Context, p repository.SemanticModelPolicy) (*repository.SemanticModelPolicy, error) {
	p.PolicyVersion = uint64(len(r.policies) + 1)
	r.policies = append(r.policies, p)
	return &p, nil
}

type semanticPolicyKB struct{ kb *types.KnowledgeBase }

func (f semanticPolicyKB) GetKnowledgeBaseByIDOnly(context.Context, string) (*types.KnowledgeBase, error) {
	return f.kb, nil
}

type semanticPolicyModel struct{ model *types.Model }

func (f semanticPolicyModel) GetModelByID(context.Context, string) (*types.Model, error) {
	return f.model, nil
}
