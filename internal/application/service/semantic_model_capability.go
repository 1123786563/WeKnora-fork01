package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/config"
	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrSemanticModelCapabilityInvalid       = errors.New("semantic_model_capability_invalid")
	ErrSemanticModelCapabilityScopeInvalid  = errors.New("semantic_model_capability_scope_invalid")
	ErrSemanticModelCapabilityPolicyInvalid = errors.New("semantic_model_capability_policy_invalid")
)

type SemanticScopeResolver interface {
	Resolve(context.Context, string) (SemanticScopeSnapshot, error)
}
type semanticModelBudgetStore interface {
	EnsureTaskBudget(context.Context, uint64, string, domain.Credits, time.Time) error
}
type semanticModelInvocationStore interface {
	EnsureRun(context.Context, types.SemanticModelCapability) error
}
type semanticModelCapabilityClaims struct {
	jwt.RegisteredClaims
	Version                                                                  int `json:"version"`
	ScopeRef, ScopeHash, KBID, ModelID, Funding, PriceVersion, RunID, CallID string
	OwnerTenantID, RequesterTenantID, PolicyVersion                          uint64
}
type SemanticModelCapabilityIssuerService struct {
	cfg         *config.SemanticServiceConfig
	scope       SemanticScopeResolver
	policy      *SemanticModelPolicyService
	pricing     SemanticModelPricingResolver
	budget      semanticModelBudgetStore
	invocations semanticModelInvocationStore
	now         func() time.Time
}

func NewSemanticModelCapabilityIssuer(cfg *config.SemanticServiceConfig, scope SemanticScopeResolver, policy *SemanticModelPolicyService, pricing SemanticModelPricingResolver, budget semanticModelBudgetStore, invocations semanticModelInvocationStore) *SemanticModelCapabilityIssuerService {
	return &SemanticModelCapabilityIssuerService{cfg: cfg, scope: scope, policy: policy, pricing: pricing, budget: budget, invocations: invocations, now: time.Now}
}

func (s *SemanticModelCapabilityIssuerService) Issue(ctx context.Context, scopeRef, invocationKey string) (types.SemanticModelIssuedCapability, error) {
	if !s.ready() || strings.TrimSpace(scopeRef) == "" || strings.TrimSpace(invocationKey) == "" {
		return types.SemanticModelIssuedCapability{}, ErrSemanticModelCapabilityInvalid
	}
	snapshot, err := s.scope.Resolve(ctx, scopeRef)
	if err != nil {
		return types.SemanticModelIssuedCapability{}, ErrSemanticModelCapabilityScopeInvalid
	}
	c, err := s.bind(ctx, snapshot, scopeRef, invocationKey)
	if err != nil {
		return types.SemanticModelIssuedCapability{}, err
	}
	if c.MaxInputTokensPerCall > math.MaxInt64/4 {
		return types.SemanticModelIssuedCapability{}, ErrSemanticModelCapabilityPolicyInvalid
	}
	if s.invocations != nil {
		if err := s.invocations.EnsureRun(ctx, c); err != nil {
			return types.SemanticModelIssuedCapability{}, err
		}
	}
	if c.Funding == domain.FundingPlatform {
		if s.budget == nil || c.TaskUpperMicro == nil {
			return types.SemanticModelIssuedCapability{}, ErrSemanticModelCapabilityPolicyInvalid
		}
		if err := s.budget.EnsureTaskBudget(ctx, c.OwnerTenantID, c.RunID, domain.Credits(*c.TaskUpperMicro), c.Deadline); err != nil {
			return types.SemanticModelIssuedCapability{}, err
		}
	}
	claims := semanticModelCapabilityClaims{RegisteredClaims: jwt.RegisteredClaims{Audience: jwt.ClaimStrings{s.cfg.Audience}, ExpiresAt: jwt.NewNumericDate(c.ExpiresAt), IssuedAt: jwt.NewNumericDate(s.now().UTC())}, Version: 1, ScopeRef: scopeRef, ScopeHash: c.ScopeHash, KBID: c.KBID, ModelID: c.ModelID, Funding: c.Funding, PriceVersion: c.PriceVersion, RunID: c.RunID, CallID: c.CallID, OwnerTenantID: c.OwnerTenantID, RequesterTenantID: c.RequesterTenantID, PolicyVersion: c.PolicyVersion}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.ModelSigningKey))
	if err != nil {
		return types.SemanticModelIssuedCapability{}, ErrSemanticModelCapabilityInvalid
	}
	return types.SemanticModelIssuedCapability{Token: token, MaxInputBytes: c.MaxInputTokensPerCall * 4, MaxOutputTokens: c.MaxOutputTokensPerCall, ExpiresAt: c.ExpiresAt}, nil
}
func (s *SemanticModelCapabilityIssuerService) Verify(ctx context.Context, token string) (types.SemanticModelCapability, error) {
	if !s.ready() {
		return types.SemanticModelCapability{}, ErrSemanticModelCapabilityInvalid
	}
	var claims semanticModelCapabilityClaims
	parsed, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, ErrSemanticModelCapabilityInvalid
		}
		return []byte(s.cfg.ModelSigningKey), nil
	}, jwt.WithAudience(s.cfg.Audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !parsed.Valid || claims.Version != 1 {
		return types.SemanticModelCapability{}, ErrSemanticModelCapabilityInvalid
	}
	snapshot, err := s.scope.Resolve(ctx, claims.ScopeRef)
	if err != nil || snapshot.ScopeHash != claims.ScopeHash || snapshot.Scope.TenantID != claims.OwnerTenantID || snapshot.Scope.KBID != claims.KBID {
		return types.SemanticModelCapability{}, ErrSemanticModelCapabilityScopeInvalid
	}
	c, err := s.bind(ctx, snapshot, claims.ScopeRef, "")
	if err != nil {
		return types.SemanticModelCapability{}, err
	}
	if c.ModelID != claims.ModelID || c.Funding != claims.Funding || c.PriceVersion != claims.PriceVersion || c.RunID != claims.RunID || c.CallID != "" || c.PolicyVersion != claims.PolicyVersion {
		return types.SemanticModelCapability{}, ErrSemanticModelCapabilityPolicyInvalid
	}
	c.CallID = claims.CallID
	return c, nil
}
func (s *SemanticModelCapabilityIssuerService) bind(ctx context.Context, snapshot SemanticScopeSnapshot, scopeRef, invocationKey string) (types.SemanticModelCapability, error) {
	if s.policy == nil || snapshot.Scope.TenantID == 0 || snapshot.Scope.KBID == "" || snapshot.ScopeHash == "" {
		return types.SemanticModelCapability{}, ErrSemanticModelCapabilityScopeInvalid
	}
	p, err := s.policy.Get(types.WithExecutionTenant(ctx, snapshot.Scope.TenantID), SemanticModelPolicyScope{TenantID: snapshot.Scope.TenantID, KBID: snapshot.Scope.KBID, ActorID: "semantic-service"})
	if err != nil {
		return types.SemanticModelCapability{}, ErrSemanticModelCapabilityPolicyInvalid
	}
	if s.pricing == nil {
		return types.SemanticModelCapability{}, ErrSemanticModelCapabilityPolicyInvalid
	}
	pricing, err := s.pricing(types.WithExecutionTenant(ctx, snapshot.Scope.TenantID), snapshot.Scope.TenantID, &types.Model{ID: p.ModelID, TenantID: snapshot.Scope.TenantID})
	if err != nil || pricing.PriceVersion != p.PriceVersion || pricing.Funding != p.Funding || pricing.Rates.Version != p.PriceVersion {
		return types.SemanticModelCapability{}, ErrSemanticModelCapabilityPolicyInvalid
	}
	expiry, err := time.Parse(time.RFC3339, snapshot.ExpiresAt)
	if err != nil || !expiry.After(s.now()) {
		return types.SemanticModelCapability{}, ErrSemanticModelCapabilityScopeInvalid
	}
	expiry = expiry.UTC()
	deadline := expiry
	run := "semantic-run:" + hash(scopeRef)
	call := ""
	if invocationKey != "" {
		call = "semantic-call:" + hash(scopeRef+"\x00"+invocationKey)
	}
	return types.SemanticModelCapability{OwnerTenantID: snapshot.Scope.TenantID, RequesterTenantID: snapshot.RequesterTenantID, KBID: snapshot.Scope.KBID, ScopeRef: scopeRef, ScopeHash: snapshot.ScopeHash, SubjectID: snapshot.SubjectID, Purpose: string(snapshot.Purpose), Audience: snapshot.Audience, PolicyVersion: p.PolicyVersion, ModelID: p.ModelID, Funding: p.Funding, PriceVersion: p.PriceVersion, RunID: run, CallID: call, MaxInputTokensPerCall: p.MaxInputTokensPerCall, MaxOutputTokensPerCall: p.MaxOutputTokensPerCall, MaxCallsPerTask: p.MaxCallsPerTask, MaxInputTokensPerTask: p.MaxInputTokensPerTask, MaxOutputTokensPerTask: p.MaxOutputTokensPerTask, PerCallUpperMicro: p.PerCallUpperMicro, TaskUpperMicro: p.TaskUpperMicro, ExpiresAt: expiry, Deadline: deadline}, nil
}
func (s *SemanticModelCapabilityIssuerService) ready() bool {
	return s != nil && s.cfg != nil && s.cfg.Enabled && s.cfg.HasValidModelSigningKey() && s.scope != nil && s.policy != nil
}
func hash(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
