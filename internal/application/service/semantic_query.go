package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
)

var ErrSemanticQueryBackend = fmt.Errorf("semantic query backend error")

// FusionConfig fixes the fusion rule for the first version: RRF with
// k=60 as the experimental starting point; identity dedup key is
// document/revision/chunk - raw engine scores are never added.
type FusionConfig struct {
	RRFK float64
}

// SearchRequest addresses one knowledge base; the facade signs the
// scope INTERNALLY - callers never bring their own authorization.
type SearchRequest struct {
	TenantID uint64
	KBID     string
	Query    string
	TopK     int
}

// ReasonRequest selects a reasoning mode over the same facade.
type ReasonRequest struct {
	TenantID       uint64
	KBID           string
	Query          string
	Mode           string // "rules" | "model"
	RuleSetVersion string
}

// SemanticSearcher is the semantic-pipeline seam (gRPC client in
// production, controlled fake in tests). It receives the ISSUED scope
// plus the query and topK - the wire request is fully populated.
type SemanticSearcher interface {
	Search(ctx context.Context, issued types.SemanticAccessScope, query string, topK int) ([]types.SemanticEvidence, string, error)
}

// SemanticReasoner dispatches Reason requests through the Q03 modes.
type SemanticReasoner interface {
	Reason(ctx context.Context, issued types.SemanticAccessScope, req ReasonRequest) (*types.SemanticReasonResult, error)
}

// KBTenantResolver resolves the OWNER tenant for a KB (A01 carry-over:
// callers must never trust their own tenant id for scope issuance).
type KBTenantResolver func(ctx context.Context, tenantID uint64, kbID string) (uint64, error)

// VectorSearcher is the ordinary-retrieval seam (vector/fulltext).
type VectorSearcher interface {
	Search(ctx context.Context, query string) ([]types.SemanticEvidence, error)
}

// SemanticQueryService is the UNIFIED query facade (Q04): chat and the
// agent graph tool all enter here. It issues the access scope, fuses
// ordinary and semantic retrieval with RRF, and validates delivery
// before ANY content leaves: a permission change discards the whole
// answer - never just the references.
type SemanticQueryService struct {
	scopes   *SemanticScopeService
	sem      SemanticSearcher
	vector   VectorSearcher
	reasoner SemanticReasoner
	fusion   FusionConfig
	resolve  KBTenantResolver
}

func NewSemanticQueryService(scopes *SemanticScopeService, sem SemanticSearcher,
	vector VectorSearcher, fusion FusionConfig, opts ...QueryOption) *SemanticQueryService {
	if fusion.RRFK <= 0 {
		fusion.RRFK = 60
	}
	svc := &SemanticQueryService{scopes: scopes, sem: sem, vector: vector, fusion: fusion}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// QueryOption configures optional seams on the facade.
type QueryOption func(*SemanticQueryService)

// WithKBTenantResolver enforces owner-tenant resolution before issuance.
func WithKBTenantResolver(resolve KBTenantResolver) QueryOption {
	return func(s *SemanticQueryService) { s.resolve = resolve }
}

// WithSemanticReasoner wires the Q03 Reason dispatch.
func WithSemanticReasoner(reasoner SemanticReasoner) QueryOption {
	return func(s *SemanticQueryService) { s.reasoner = reasoner }
}

// Search runs authorized retrieval with RRF fusion. On semantic-backend
// failure it degrades EXPLICITLY: mode "retrieval.degraded" tells the
// caller this was ordinary retrieval, never disguised as reasoning.
func (s *SemanticQueryService) Search(ctx context.Context, subjectID string, req SearchRequest) (types.SemanticSearchResult, error) {
	scope, evidence, mode, err := s.runAuthorizedSearch(ctx, subjectID, req)
	if err != nil {
		return types.SemanticSearchResult{}, err
	}
	// Delivery re-check on the search path too: the epoch must still hold
	// when evidence leaves the facade (spec section 5 delivery re-check).
	if err := s.scopes.ValidateDelivery(ctx, scope); err != nil {
		return types.SemanticSearchResult{}, fmt.Errorf("%w: delivery validation failed: %v", ErrSemanticScopeChanged, err)
	}
	return types.SemanticSearchResult{Mode: mode, Evidence: evidence}, nil
}

// Reason applies the FINAL delivery check after result generation. The
// Q04 slice routes reasoning requests through the retrieval channel with
// an HONEST mode label ("retrieval.pending-reason") and Status
// "insufficient_evidence" - it NEVER fabricates a supported conclusion
// (the real Reason dispatch through the Q03 modes lands with the
// W-wiring; see the ledger).
func (s *SemanticQueryService) Reason(ctx context.Context, subjectID string, req ReasonRequest) (*types.SemanticReasonResult, error) {
	ownerTenant, err := s.resolveTenant(ctx, req.TenantID, req.KBID)
	if err != nil {
		return nil, err
	}
	scope, err := s.scopes.Issue(ctx, subjectID, types.SemanticScopeKey{
		TenantID: ownerTenant, KBID: req.KBID}, "reason")
	if err != nil {
		return nil, err
	}
	if s.reasoner != nil {
		// Real Q03 dispatch (rules/model) through the issued scope.
		result, err := s.reasoner.Reason(ctx, scope, req)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrSemanticQueryBackend, err)
		}
		if err := s.scopes.ValidateDelivery(ctx, scope); err != nil {
			retried, retryErr := s.reasonOnce(ctx, subjectID, req)
			if retryErr != nil {
				return nil, fmt.Errorf("%w: %v / %v", ErrSemanticScopeChanged, err, retryErr)
			}
			return retried, nil
		}
		return result, nil
	}
	evidence, mode, err := s.sem.Search(ctx, scope, req.Query, 0)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSemanticQueryBackend, err)
	}
	if mode == "" {
		mode = "graphrag"
	}
	result := &types.SemanticReasonResult{
		Mode:     "retrieval.pending-reason",
		Evidence: s.fuse(evidence, nil),
		Status:   "insufficient_evidence",
	}
	_ = mode
	// Delivery validation: the FINAL authorization gate. On epoch change
	// the WHOLE answer is discarded (never deliver then retract) and ONE
	// recompute with a FRESH scope is attempted; a second change returns
	// a retryable error - never an infinite loop.
	if err := s.scopes.ValidateDelivery(ctx, scope); err != nil {
		retried, retryErr := s.reasonOnce(ctx, subjectID, req)
		if retryErr != nil {
			return nil, fmt.Errorf("%w: delivery validation failed and one retry failed: %v / %v",
				ErrSemanticScopeChanged, err, retryErr)
		}
		return retried, nil
	}
	return result, nil
}

// reasonOnce is the single-retry path: fresh scope, fresh computation,
// and a FINAL delivery check whose failure is terminal (retryable error
// surfaced to the caller - the plan forbids infinite recompute).
func (s *SemanticQueryService) reasonOnce(ctx context.Context, subjectID string, req ReasonRequest) (*types.SemanticReasonResult, error) {
	ownerTenant, err := s.resolveTenant(ctx, req.TenantID, req.KBID)
	if err != nil {
		return nil, err
	}
	scope, err := s.scopes.Issue(ctx, subjectID, types.SemanticScopeKey{
		TenantID: ownerTenant, KBID: req.KBID}, "reason")
	if err != nil {
		return nil, err
	}
	if s.reasoner != nil {
		result, err := s.reasoner.Reason(ctx, scope, req)
		if err != nil {
			return nil, err
		}
		if err := s.scopes.ValidateDelivery(ctx, scope); err != nil {
			return nil, fmt.Errorf("%w: retry delivery validation failed: %v", ErrSemanticScopeChanged, err)
		}
		return result, nil
	}
	evidence, _, err := s.sem.Search(ctx, scope, req.Query, 0)
	if err != nil {
		return nil, err
	}
	result := &types.SemanticReasonResult{
		Mode:     "retrieval.pending-reason",
		Evidence: s.fuse(evidence, nil),
		Status:   "insufficient_evidence",
	}
	if err := s.scopes.ValidateDelivery(ctx, scope); err != nil {
		return nil, fmt.Errorf("%w: retry delivery validation failed: %v", ErrSemanticScopeChanged, err)
	}
	return result, nil
}

// resolveTenant applies the owner-tenant resolver. FAIL-CLOSED: with no
// resolver configured the facade refuses to issue any scope - the
// caller's tenant id is a LOOKUP key, never the issuance tenant (A01
// carry-over; production must wire resolveKBReadTenant via the option).
func (s *SemanticQueryService) resolveTenant(ctx context.Context, tenantID uint64, kbID string) (uint64, error) {
	if s.resolve == nil {
		return 0, fmt.Errorf("semantic query: no KB tenant resolver configured (fail closed)")
	}
	return s.resolve(ctx, tenantID, kbID)
}

// runAuthorizedSearch issues the scope, runs both engines and fuses; a
// semantic failure degrades with an explicit mode.
func (s *SemanticQueryService) runAuthorizedSearch(ctx context.Context, subjectID string, req SearchRequest) (types.SemanticAccessScope, []types.FusedEvidence, string, error) {
	ownerTenant, err := s.resolveTenant(ctx, req.TenantID, req.KBID)
	if err != nil {
		return types.SemanticAccessScope{}, nil, "", err
	}
	scope, err := s.scopes.Issue(ctx, subjectID, types.SemanticScopeKey{
		TenantID: ownerTenant, KBID: req.KBID}, "search")
	if err != nil {
		return scope, nil, "", err
	}
	semEvidence, mode, semErr := s.sem.Search(ctx, scope, req.Query, req.TopK)
	vecEvidence, vecErr := s.vector.Search(ctx, req.Query)
	if semErr != nil && vecErr != nil {
		return scope, nil, "", fmt.Errorf("%w: semantic=%v vector=%v", ErrSemanticQueryBackend, semErr, vecErr)
	}
	if semErr != nil {
		// Degrade explicitly: ordinary retrieval only, mode declares it.
		return scope, s.fuse(vecEvidence, nil), "retrieval.degraded", nil
	}
	// Report the backend's ACTUAL mode (never hardcode it).
	if mode == "" {
		mode = "graphrag"
	}
	return scope, s.fuse(semEvidence, vecEvidence), mode, nil
}

// fuse applies RRF over rank positions; identity deduplicates across
// engines by document/revision/chunk. Raw scores are never combined.
func (s *SemanticQueryService) fuse(lists ...[]types.SemanticEvidence) []types.FusedEvidence {
	scores := map[string]float64{}
	evidence := map[string]types.SemanticEvidence{}
	for _, list := range lists {
		for rank, ev := range list {
			identity := strings.Join([]string{ev.DocumentID,
				fmt.Sprintf("%d", ev.Revision), ev.ChunkID}, "/")
			scores[identity] += 1.0 / (s.fusion.RRFK + float64(rank+1))
			evidence[identity] = ev
		}
	}
	out := make([]types.FusedEvidence, 0, len(scores))
	for identity, score := range scores {
		out = append(out, types.FusedEvidence{SemanticEvidence: evidence[identity], FusedScore: score, Identity: identity})
	}
	// Stable descending order by fused score, then identity.
	sortFused(out)
	return out
}

func sortFused(list []types.FusedEvidence) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0; j-- {
			if list[j].FusedScore > list[j-1].FusedScore ||
				(list[j].FusedScore == list[j-1].FusedScore && list[j].Identity < list[j-1].Identity) {
				list[j], list[j-1] = list[j-1], list[j]
			} else {
				break
			}
		}
	}
}