package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/config"
	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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

func TestSemanticModelCapabilityVerifyRequiresExpiryAndAudience(t *testing.T) {
	resolver := &semanticCapabilityScopeResolver{snapshot: SemanticScopeSnapshot{Scope: types.SemanticScopeKey{TenantID: 7, KBID: "kb-1"}, SubjectID: "requester", RequesterTenantID: 9, ScopeHash: "scope-hash", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), Audience: "semantic"}}
	issuer := NewSemanticModelCapabilityIssuer(semanticCapabilityConfig("semantic"), resolver, semanticCapabilityPolicy(), semanticCapabilityPricing, nil, nil)
	for name, claims := range map[string]semanticModelCapabilityClaims{
		"expired":        {RegisteredClaims: jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"semantic"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute))}, Version: 1, ScopeRef: "scope-ref", ScopeHash: "scope-hash", KBID: "kb-1", ModelID: "model-1", Funding: domain.FundingBYOK, PriceVersion: "pv1", RunID: "semantic-run:" + hash("scope-ref"), CallID: "semantic-call:test", OwnerTenantID: 7, PolicyVersion: 3},
		"missing expiry": {RegisteredClaims: jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"semantic"}}, Version: 1, ScopeRef: "scope-ref", ScopeHash: "scope-hash", KBID: "kb-1", ModelID: "model-1", Funding: domain.FundingBYOK, PriceVersion: "pv1", RunID: "semantic-run:" + hash("scope-ref"), CallID: "semantic-call:test", OwnerTenantID: 7, PolicyVersion: 3},
		"wrong audience": {RegisteredClaims: jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"other"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}, Version: 1, ScopeRef: "scope-ref", ScopeHash: "scope-hash", KBID: "kb-1", ModelID: "model-1", Funding: domain.FundingBYOK, PriceVersion: "pv1", RunID: "semantic-run:" + hash("scope-ref"), CallID: "semantic-call:test", OwnerTenantID: 7, PolicyVersion: 3},
	} {
		t.Run(name, func(t *testing.T) {
			inv := &recordingInvocationStore{}
			budget := &recordingBudgetStore{}
			issuer = NewSemanticModelCapabilityIssuer(semanticCapabilityConfig("semantic"), resolver, semanticCapabilityPolicy(), semanticCapabilityPricing, budget, inv)
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(issuer.cfg.ModelSigningKey))
			require.NoError(t, err)
			_, err = issuer.Verify(context.Background(), token)
			require.ErrorIs(t, err, ErrSemanticModelCapabilityInvalid)
			require.Zero(t, inv.calls)
			require.Zero(t, budget.calls)
		})
	}
}

func TestSemanticModelCapabilityInvalidAuthorityDoesNotMutate(t *testing.T) {
	valid := SemanticScopeSnapshot{Scope: types.SemanticScopeKey{TenantID: 7, KBID: "kb-1"}, SubjectID: "requester", RequesterTenantID: 9, ScopeHash: "scope-hash", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), Audience: "semantic"}
	for name, setup := range map[string]func(*semanticCapabilityScopeResolver, *SemanticModelPolicyService, *SemanticModelCapabilityIssuerService){
		"revoked scope": func(r *semanticCapabilityScopeResolver, _ *SemanticModelPolicyService, _ *SemanticModelCapabilityIssuerService) {
			r.err = ErrSemanticScopeChanged
		},
		"missing owner KB": func(_ *semanticCapabilityScopeResolver, p *SemanticModelPolicyService, _ *SemanticModelCapabilityIssuerService) {
			p.kb = semanticCapabilityKB{err: errors.New("missing")}
		},
		"deleted owner KB": func(_ *semanticCapabilityScopeResolver, p *SemanticModelPolicyService, _ *SemanticModelCapabilityIssuerService) {
			p.kb = semanticCapabilityKB{kb: &types.KnowledgeBase{ID: "kb-1", TenantID: 7, DeletedAt: gorm.DeletedAt{Valid: true}}}
		},
		"disabled policy": func(_ *semanticCapabilityScopeResolver, p *SemanticModelPolicyService, _ *SemanticModelCapabilityIssuerService) {
			p.repo = semanticCapabilityRepo{p: semanticPolicy(false, 3)}
		},
		"unknown rate": func(_ *semanticCapabilityScopeResolver, _ *SemanticModelPolicyService, i *SemanticModelCapabilityIssuerService) {
			i.pricing = func(context.Context, uint64, *types.Model) (SemanticModelPricing, error) {
				return SemanticModelPricing{}, errors.New("unknown rate")
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := &semanticCapabilityScopeResolver{snapshot: valid}
			inv := &recordingInvocationStore{}
			budget := &recordingBudgetStore{}
			issuer := NewSemanticModelCapabilityIssuer(semanticCapabilityConfig("semantic"), r, semanticCapabilityPolicy(), semanticCapabilityPricing, budget, inv)
			setup(r, issuer.policy, issuer)
			_, err := issuer.Issue(context.Background(), "scope-ref", "key")
			require.Error(t, err)
			require.Zero(t, inv.calls)
			require.Zero(t, budget.calls)
		})
	}
	for name, claims := range map[string]semanticModelCapabilityClaims{
		"tampered":               {RegisteredClaims: jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"semantic"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)), IssuedAt: jwt.NewNumericDate(time.Now())}, Version: 1, ScopeRef: "scope-ref", ScopeHash: "scope-hash", KBID: "kb-1", ModelID: "model-1", Funding: domain.FundingBYOK, PriceVersion: "pv1", RunID: "semantic-run:" + hash("scope-ref"), CallID: "semantic-call:test", OwnerTenantID: 7, PolicyVersion: 3},
		"scope binding mismatch": {RegisteredClaims: jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"semantic"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)), IssuedAt: jwt.NewNumericDate(time.Now())}, Version: 1, ScopeRef: "scope-ref", ScopeHash: "other-hash", KBID: "kb-1", ModelID: "model-1", Funding: domain.FundingBYOK, PriceVersion: "pv1", RunID: "semantic-run:" + hash("scope-ref"), CallID: "semantic-call:test", OwnerTenantID: 7, PolicyVersion: 3},
		"scope ref mismatch":     {RegisteredClaims: jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"semantic"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)), IssuedAt: jwt.NewNumericDate(time.Now())}, Version: 1, ScopeRef: "other-ref", ScopeHash: "scope-hash", KBID: "kb-1", ModelID: "model-1", Funding: domain.FundingBYOK, PriceVersion: "pv1", RunID: "semantic-run:" + hash("other-ref"), CallID: "semantic-call:test", OwnerTenantID: 7, PolicyVersion: 3},
		"stale policy":           {RegisteredClaims: jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"semantic"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)), IssuedAt: jwt.NewNumericDate(time.Now())}, Version: 1, ScopeRef: "scope-ref", ScopeHash: "scope-hash", KBID: "kb-1", ModelID: "model-1", Funding: domain.FundingBYOK, PriceVersion: "pv1", RunID: "semantic-run:" + hash("scope-ref"), CallID: "semantic-call:test", OwnerTenantID: 7, PolicyVersion: 2},
	} {
		t.Run(name, func(t *testing.T) {
			r := &semanticCapabilityScopeResolver{snapshot: valid}
			if name == "scope ref mismatch" {
				r.expectedRef = "scope-ref"
			}
			inv := &recordingInvocationStore{}
			budget := &recordingBudgetStore{}
			issuer := NewSemanticModelCapabilityIssuer(semanticCapabilityConfig("semantic"), r, semanticCapabilityPolicy(), semanticCapabilityPricing, budget, inv)
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(issuer.cfg.ModelSigningKey))
			require.NoError(t, err)
			if name == "tampered" {
				token += "x"
			}
			_, err = issuer.Verify(context.Background(), token)
			require.Error(t, err)
			require.Zero(t, inv.calls)
			require.Zero(t, budget.calls)
		})
	}
}

func TestSemanticModelCapabilityIssueRejectsOverflowingInputBytes(t *testing.T) {
	resolver := &semanticCapabilityScopeResolver{snapshot: SemanticScopeSnapshot{Scope: types.SemanticScopeKey{TenantID: 7, KBID: "kb-1"}, ScopeHash: "scope-hash", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), Audience: "semantic"}}
	policy := semanticCapabilityPolicy()
	policy.repo = semanticCapabilityRepo{p: repository.SemanticModelPolicy{TenantID: 7, KBID: "kb-1", ModelCallsEnabled: true, ModelID: "model-1", Funding: domain.FundingBYOK, PriceVersion: "pv1", PolicyVersion: 3, MaxInputTokensPerCall: int64(^uint64(0)>>1)/4 + 1, MaxOutputTokensPerCall: 1, MaxCallsPerTask: 1, MaxInputTokensPerTask: int64(^uint64(0)>>1)/4 + 1, MaxOutputTokensPerTask: 1, UpdatedBy: "owner"}}
	issuer := NewSemanticModelCapabilityIssuer(semanticCapabilityConfig("semantic"), resolver, policy, semanticCapabilityPricing, nil, nil)
	_, err := issuer.Issue(context.Background(), "scope-ref", "key")
	require.ErrorIs(t, err, ErrSemanticModelCapabilityPolicyInvalid)
	policy.repo = semanticCapabilityRepo{p: repository.SemanticModelPolicy{TenantID: 7, KBID: "kb-1", ModelCallsEnabled: true, ModelID: "model-1", Funding: domain.FundingBYOK, PriceVersion: "pv1", PolicyVersion: 3, MaxInputTokensPerCall: int64(^uint64(0)>>1) / 4, MaxOutputTokensPerCall: 1, MaxCallsPerTask: 1, MaxInputTokensPerTask: int64(^uint64(0)>>1) / 4, MaxOutputTokensPerTask: 1, UpdatedBy: "owner"}}
	issued, err := issuer.Issue(context.Background(), "scope-ref", "key-boundary")
	require.NoError(t, err)
	require.Equal(t, int64(^uint64(0)>>1)-3, issued.MaxInputBytes)
}

type semanticCapabilityScopeResolver struct {
	snapshot    SemanticScopeSnapshot
	err         error
	expectedRef string
}

func (r *semanticCapabilityScopeResolver) Resolve(_ context.Context, ref string) (SemanticScopeSnapshot, error) {
	if r.expectedRef != "" && ref != r.expectedRef {
		return SemanticScopeSnapshot{}, ErrSemanticScopeInvalid
	}
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

type semanticCapabilityKB struct {
	err error
	kb  *types.KnowledgeBase
}

func (f semanticCapabilityKB) GetKnowledgeBaseByIDOnly(context.Context, string) (*types.KnowledgeBase, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.kb != nil {
		return f.kb, nil
	}
	return &types.KnowledgeBase{ID: "kb-1", TenantID: 7}, nil
}
func semanticCapabilityPolicy() *SemanticModelPolicyService {
	return NewSemanticModelPolicyService(semanticCapabilityRepo{p: semanticPolicy(true, 3)}, semanticCapabilityKB{}, nil, nil)
}
func semanticPolicy(enabled bool, version uint64) repository.SemanticModelPolicy {
	return repository.SemanticModelPolicy{TenantID: 7, KBID: "kb-1", ModelCallsEnabled: enabled, ModelID: "model-1", Funding: domain.FundingBYOK, PriceVersion: "pv1", PolicyVersion: version, MaxInputTokensPerCall: 10, MaxOutputTokensPerCall: 20, MaxCallsPerTask: 1, MaxInputTokensPerTask: 10, MaxOutputTokensPerTask: 20, UpdatedBy: "owner"}
}
func semanticCapabilityPricing(context.Context, uint64, *types.Model) (SemanticModelPricing, error) {
	return SemanticModelPricing{Funding: domain.FundingBYOK, PriceVersion: "pv1", Rates: domain.PriceVersionRates{Version: "pv1"}}, nil
}

type recordingInvocationStore struct{ calls int }

func (r *recordingInvocationStore) EnsureRun(context.Context, types.SemanticModelCapability) error {
	r.calls++
	return nil
}

type recordingBudgetStore struct{ calls int }

func (r *recordingBudgetStore) EnsureTaskBudget(context.Context, uint64, string, domain.Credits, time.Time) error {
	r.calls++
	return nil
}
