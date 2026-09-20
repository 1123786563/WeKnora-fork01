package service

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// This catches a verifier that trusts a signed token after its live A01 scope
// has been revoked, rather than re-resolving it before mutable quota work.
func TestSemanticModelCapabilityVerifyRejectsTamperingAndRevokedScope(t *testing.T) {
	resolver := &semanticCapabilityScopeResolver{snapshot: SemanticScopeSnapshot{Scope: types.SemanticScopeKey{TenantID: 7, KBID: "kb-1"}, SubjectID: "requester", RequesterTenantID: 9, ScopeHash: "scope-hash", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), Audience: "semantic"}}
	issuer := NewSemanticModelCapabilityIssuer(semanticCapabilityConfig("semantic"), resolver, semanticCapabilityPolicy(), semanticCapabilityPricing, nil, nil)
	issued, err := issuer.Issue(context.Background(), "scope-ref", "key-one")
	require.NoError(t, err)
	_, err = issuer.Verify(context.Background(), issued.Token+"x")
	require.ErrorIs(t, err, ErrSemanticModelCapabilityInvalid)

	resolver.err = ErrSemanticScopeChanged
	_, err = issuer.Verify(context.Background(), issued.Token)
	require.ErrorIs(t, err, ErrSemanticModelCapabilityScopeInvalid)
}

type semanticCapabilityScopeResolver struct {
	snapshot SemanticScopeSnapshot
	err      error
}

func (r *semanticCapabilityScopeResolver) Resolve(context.Context, string) (SemanticScopeSnapshot, error) {
	return r.snapshot, r.err
}
func semanticCapabilityConfig(audience string) *config.SemanticServiceConfig {
	return &config.SemanticServiceConfig{Enabled: true, Audience: audience, ServiceToken: "service-token-which-is-long-enough-000", ScopeSigningKey: "scope-signing-key-which-is-long-enough", ModelSigningKey: "model-signing-key-which-is-long-enough"}
}

type semanticCapabilityRepo struct {
	p repository.SemanticModelPolicy
}

func (r semanticCapabilityRepo) Get(context.Context, uint64, string) (*repository.SemanticModelPolicy, error) {
	p := r.p
	return &p, nil
}
func (r semanticCapabilityRepo) Put(context.Context, repository.SemanticModelPolicy) (*repository.SemanticModelPolicy, error) {
	return nil, nil
}

type semanticCapabilityKB struct{}

func (semanticCapabilityKB) GetKnowledgeBaseByIDOnly(context.Context, string) (*types.KnowledgeBase, error) {
	return &types.KnowledgeBase{ID: "kb-1", TenantID: 7}, nil
}
func semanticCapabilityPolicy() *SemanticModelPolicyService {
	return NewSemanticModelPolicyService(semanticCapabilityRepo{p: repository.SemanticModelPolicy{TenantID: 7, KBID: "kb-1", ModelCallsEnabled: true, ModelID: "model-1", Funding: domain.FundingBYOK, PriceVersion: "pv1", PolicyVersion: 3, MaxInputTokensPerCall: 10, MaxOutputTokensPerCall: 20, MaxCallsPerTask: 1, MaxInputTokensPerTask: 10, MaxOutputTokensPerTask: 20, UpdatedBy: "owner"}}, semanticCapabilityKB{}, nil, nil)
}
func semanticCapabilityPricing(context.Context, uint64, *types.Model) (SemanticModelPricing, error) {
	return SemanticModelPricing{Funding: domain.FundingBYOK, PriceVersion: "pv1", Rates: domain.PriceVersionRates{Version: "pv1"}}, nil
}
