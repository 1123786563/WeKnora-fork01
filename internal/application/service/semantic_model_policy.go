package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/repository"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

var (
	ErrSemanticModelPolicyDisabled     = errors.New("semantic_model_policy_disabled")
	ErrSemanticModelPolicyInvalid      = errors.New("semantic_model_policy_invalid")
	ErrSemanticModelPricingUnavailable = errors.New("semantic_model_pricing_unavailable")
)

type SemanticModelPolicyScope struct {
	TenantID      uint64
	KBID, ActorID string
}
type SemanticModelPolicyInput struct {
	ModelCallsEnabled      bool   `json:"model_calls_enabled"`
	ModelID                string `json:"model_id"`
	MaxInputTokensPerCall  int64  `json:"max_input_tokens_per_call"`
	MaxOutputTokensPerCall int64  `json:"max_output_tokens_per_call"`
	MaxCallsPerTask        int64  `json:"max_calls_per_task"`
	MaxInputTokensPerTask  int64  `json:"max_input_tokens_per_task"`
	MaxOutputTokensPerTask int64  `json:"max_output_tokens_per_task"`
}
type SemanticModelPricing struct {
	Funding, PriceVersion string
	Rates                 domain.PriceVersionRates
	TaskUpperMicro        *int64
}
type SemanticModelPolicyRepository interface {
	Get(context.Context, uint64, string) (*repository.SemanticModelPolicy, error)
	Put(context.Context, repository.SemanticModelPolicy) (*repository.SemanticModelPolicy, error)
}
type semanticPolicyKBLookup interface {
	GetKnowledgeBaseByIDOnly(context.Context, string) (*types.KnowledgeBase, error)
}
type semanticPolicyModelLookup interface {
	GetModelByID(context.Context, string) (*types.Model, error)
}
type SemanticModelPricingResolver func(context.Context, uint64, *types.Model) (SemanticModelPricing, error)

type SemanticModelPolicyService struct {
	repo    SemanticModelPolicyRepository
	kb      semanticPolicyKBLookup
	model   semanticPolicyModelLookup
	pricing SemanticModelPricingResolver
}

func NewSemanticModelPolicyService(repo SemanticModelPolicyRepository, kb semanticPolicyKBLookup, model semanticPolicyModelLookup, pricing SemanticModelPricingResolver) *SemanticModelPolicyService {
	return &SemanticModelPolicyService{repo: repo, kb: kb, model: model, pricing: pricing}
}

// NewUnavailableSemanticModelPolicyService wires the production-safe default.
// Until pricing administration supplies a trusted immutable resolver, every
// attempted policy write is denied before it can be persisted.
func NewUnavailableSemanticModelPolicyService(repo *repository.SemanticModelPolicyRepository, kb interfaces.KnowledgeBaseService, model interfaces.ModelService) *SemanticModelPolicyService {
	return NewSemanticModelPolicyService(repo, kb, model, nil)
}

func (s *SemanticModelPolicyService) Get(ctx context.Context, scope SemanticModelPolicyScope) (*repository.SemanticModelPolicy, error) {
	if err := s.validateScope(ctx, scope); err != nil {
		return nil, err
	}
	p, err := s.repo.Get(ctx, scope.TenantID, scope.KBID)
	if err != nil {
		return nil, err
	}
	if !p.ModelCallsEnabled {
		return nil, ErrSemanticModelPolicyDisabled
	}
	return p, nil
}

func (s *SemanticModelPolicyService) Put(ctx context.Context, scope SemanticModelPolicyScope, input SemanticModelPolicyInput) (*repository.SemanticModelPolicy, error) {
	if err := s.validateScope(ctx, scope); err != nil {
		return nil, err
	}
	// Disabling is a revocation, not a priced model admission. Preserve the
	// immutable configuration already accepted by Go and advance its audit
	// version even if pricing administration is currently unavailable.
	if !input.ModelCallsEnabled {
		existing, err := s.repo.Get(ctx, scope.TenantID, scope.KBID)
		if err != nil {
			return nil, err
		}
		existing.ModelCallsEnabled = false
		existing.UpdatedBy = scope.ActorID
		return s.repo.Put(ctx, *existing)
	}
	if !validSemanticPolicyInput(input) {
		return nil, ErrSemanticModelPolicyInvalid
	}
	if s.model == nil || s.pricing == nil {
		return nil, ErrSemanticModelPricingUnavailable
	}
	// Resolution is source-owner scoped. WithExecutionTenant preserves the
	// authenticated Caller, so pricing/model ownership cannot drift while the
	// requester remains available for audit.
	ownerCtx := types.WithExecutionTenant(ctx, scope.TenantID)
	model, err := s.model.GetModelByID(ownerCtx, input.ModelID)
	if err != nil || model == nil || model.TenantID != scope.TenantID || model.Type != types.ModelTypeKnowledgeQA || model.Status != types.ModelStatusActive {
		return nil, ErrSemanticModelPolicyInvalid
	}
	pricing, err := s.pricing(ownerCtx, scope.TenantID, model)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSemanticModelPricingUnavailable, err)
	}
	if err := domain.ValidateFunding(pricing.Funding); err != nil || pricing.PriceVersion == "" || pricing.Rates.Version != pricing.PriceVersion {
		return nil, ErrSemanticModelPricingUnavailable
	}
	upper, err := pricing.Rates.ChargeForCall(domain.UsageFact{TenantID: scope.TenantID, CallID: "semantic-policy-envelope", AttemptID: "policy", Funding: pricing.Funding, Service: "semantic_model", PriceVersion: pricing.PriceVersion, Revision: 1, Dimensions: map[string]int64{domain.DimensionModel: input.MaxInputTokensPerCall + input.MaxOutputTokensPerCall}, Status: domain.UsageStatusFinal})
	if err != nil || upper < 0 {
		return nil, ErrSemanticModelPricingUnavailable
	}
	if pricing.Funding == domain.FundingPlatform && (pricing.TaskUpperMicro == nil || *pricing.TaskUpperMicro <= 0) {
		return nil, ErrSemanticModelPolicyInvalid
	}
	if pricing.TaskUpperMicro != nil && *pricing.TaskUpperMicro <= 0 {
		return nil, ErrSemanticModelPolicyInvalid
	}
	return s.repo.Put(ctx, repository.SemanticModelPolicy{TenantID: scope.TenantID, KBID: scope.KBID, ModelCallsEnabled: input.ModelCallsEnabled, ModelID: model.ID, Funding: pricing.Funding, PriceVersion: pricing.PriceVersion, MaxInputTokensPerCall: input.MaxInputTokensPerCall, MaxOutputTokensPerCall: input.MaxOutputTokensPerCall, MaxCallsPerTask: input.MaxCallsPerTask, MaxInputTokensPerTask: input.MaxInputTokensPerTask, MaxOutputTokensPerTask: input.MaxOutputTokensPerTask, PerCallUpperMicro: int64(upper), TaskUpperMicro: pricing.TaskUpperMicro, UpdatedBy: scope.ActorID})
}
func (s *SemanticModelPolicyService) validateScope(ctx context.Context, scope SemanticModelPolicyScope) error {
	if s == nil || s.repo == nil || s.kb == nil || scope.TenantID == 0 || strings.TrimSpace(scope.KBID) == "" || strings.TrimSpace(scope.ActorID) == "" {
		return ErrSemanticModelPolicyInvalid
	}
	kb, err := s.kb.GetKnowledgeBaseByIDOnly(ctx, scope.KBID)
	if err != nil || kb == nil || kb.TenantID != scope.TenantID || kb.DeletedAt.Valid {
		return ErrSemanticModelPolicyInvalid
	}
	return nil
}
func validSemanticPolicyInput(i SemanticModelPolicyInput) bool {
	if strings.TrimSpace(i.ModelID) == "" || i.MaxInputTokensPerCall <= 0 || i.MaxOutputTokensPerCall <= 0 || i.MaxCallsPerTask <= 0 || i.MaxInputTokensPerTask <= 0 || i.MaxOutputTokensPerTask <= 0 {
		return false
	}
	if i.MaxInputTokensPerCall > i.MaxInputTokensPerTask || i.MaxOutputTokensPerCall > i.MaxOutputTokensPerTask {
		return false
	}
	_, a := safeAdd(i.MaxInputTokensPerCall, i.MaxOutputTokensPerCall)
	_, b := safeAdd(i.MaxInputTokensPerTask, i.MaxOutputTokensPerTask)
	return !a && !b
}
func safeAdd(a, b int64) (int64, bool) {
	if b > 0 && a > math.MaxInt64-b {
		return 0, true
	}
	return a + b, false
}
