package service

import (
	"context"
	"encoding/json"
	"math"
	"strconv"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
)

// NativeResolvedPendingIdentity is the typed result of the decision CAS. It is
// evidence for storage/coordinator tests, never permission to invoke a tool.
type NativeResolvedPendingIdentity struct {
	Key             nativecontract.PendingKey
	DecisionID      string
	PendingRevision string
	CallID          string
	PlanVersion     int
	ArgsHash        string
}

type nativePendingReservedStore interface {
	ResolveReserved(context.Context, nativecontract.Scope, nativecontract.PendingKey, nativecontract.ResolvePendingRequest, nativecontract.Fence, func() error) (nativecontract.PendingResolution, error)
}

// NativePendingConsumptionCoordinator composes the P2.4 CAS with P2.5's exact
// reservation seam. It intentionally has no dispatch callback or production
// registration. In-memory reservations are not crash-durable; a future durable
// adapter must reconcile a hold left behind by a failed SQL commit.
type NativePendingConsumptionCoordinator struct {
	pending      *NativePendingDecisionService
	reservations nativecontract.ToolDispatchReservation
}

func NewNativePendingConsumptionCoordinator(pending *NativePendingDecisionService, reservations nativecontract.ToolDispatchReservation) *NativePendingConsumptionCoordinator {
	return &NativePendingConsumptionCoordinator{pending: pending, reservations: reservations}
}

func (c *NativePendingConsumptionCoordinator) ResolveAndReserve(ctx context.Context, scope nativecontract.Scope, key nativecontract.PendingKey, decision nativecontract.ResolvePendingRequest, dispatch nativecontract.ToolDispatchRequest) (NativeResolvedPendingIdentity, error) {
	var empty NativeResolvedPendingIdentity
	if c == nil || c.pending == nil || c.reservations == nil {
		return empty, nativePendingServiceFailure(nativecontract.ErrStore, "pending consumption authority is unavailable")
	}
	store, ok := c.pending.store.(nativePendingReservedStore)
	if !ok {
		return empty, nativePendingServiceFailure(nativecontract.ErrStore, "transactional pending consumption is unavailable")
	}
	detail, err := c.pending.Get(ctx, scope, key)
	if err != nil {
		return empty, err
	}
	if dispatch.DecisionReference != "" || dispatch.Fence.Run != key.Run || dispatch.Plan.Run != key.Run || dispatch.Attempt.Run != key.Run || dispatch.Plan.CallID != detail.CallID || dispatch.Attempt.LogicalCallID != detail.CallID || dispatch.Plan.Version != detail.PlanVersion || dispatch.Plan.ArgsHash != detail.ArgsHash || dispatch.Plan.Tool.Name != detail.Service.ToolName || dispatch.Plan.Tool.ServiceID != detail.Service.ServiceID || dispatch.Plan.Tool.InstallationID != detail.Service.InstallationID || dispatch.Plan.Tool.SchemaHash != detail.Service.SchemaHash || decision.Action != nativecontract.DecisionRetry {
		return empty, nativePendingServiceFailure(nativecontract.ErrConflict, "reservation is not bound to the pending decision")
	}
	// Canonical structured identity replaces the caller's free-form reference.
	// Only this boundary translates it into the older P2.5 reservation contract.
	identity, err := json.Marshal(key)
	if err != nil {
		return empty, err
	}
	dispatch.DecisionReference = string(identity)
	resolved, err := store.ResolveReserved(ctx, scope, key, decision, dispatch.Fence, func() error {
		authoritative, err := c.pending.recheckDetail(ctx, scope, detail, "resolve")
		if err != nil {
			return err
		}
		authoritative, err = c.pending.recheck(ctx, authoritative, dispatch.Plan.RequiredGrants)
		if err != nil {
			return err
		}
		dispatch.Scope = authoritative
		return c.reservations.ReserveAndConsume(ctx, dispatch)
	})
	if err != nil {
		return empty, err
	}
	requestRevision, revisionErr := strconv.ParseInt(decision.PendingRevision, 10, 64)
	expectedRevision := strconv.FormatInt(requestRevision+1, 10)
	receipt := resolved.Detail
	detailRevisionMatches := detail.Ref.Revision == decision.PendingRevision
	if detail.Status == nativecontract.PendingResolved {
		detailRevisionMatches = detail.Ref.Revision == expectedRevision
	}
	if receipt.Status != nativecontract.PendingResolved || receipt.ResolvedDecisionID != decision.DecisionID || !nativePendingDetailMatchesKey(receipt, key) ||
		receipt.CallID != detail.CallID || receipt.CallID != decision.CallID || receipt.PlanVersion != detail.PlanVersion || receipt.PlanVersion != decision.PlanVersion ||
		receipt.ArgsHash != detail.ArgsHash || receipt.ArgsHash != decision.ArgsHash || receipt.ResolvedAction != decision.Action ||
		revisionErr != nil || requestRevision < 1 || requestRevision == math.MaxInt64 || !detailRevisionMatches || receipt.Ref.Revision != expectedRevision {
		return empty, nativePendingServiceFailure(nativecontract.ErrStore, "pending consumption receipt is invalid")
	}
	return NativeResolvedPendingIdentity{Key: key, DecisionID: resolved.Detail.ResolvedDecisionID, PendingRevision: resolved.Detail.Ref.Revision, CallID: resolved.Detail.CallID, PlanVersion: resolved.Detail.PlanVersion, ArgsHash: resolved.Detail.ArgsHash}, nil
}

// NativePendingDecisionStore separates the request authorization facade from
// the SQLite/PostgreSQL CAS authority. The store itself repeats tenant/session
// ownership checks so a future internal caller cannot bypass this facade.
type NativePendingDecisionStore interface {
	List(context.Context, nativecontract.Scope, nativecontract.RunIdentity, string, int) (nativecontract.PendingDecisionPage, error)
	Get(context.Context, nativecontract.Scope, nativecontract.PendingKey) (nativecontract.PendingDecisionDetail, error)
	Resolve(context.Context, nativecontract.Scope, nativecontract.PendingKey, nativecontract.ResolvePendingRequest) (nativecontract.PendingResolution, error)
}

// NativePendingDecisionService authenticates each read and one-time decision
// against current server authority. It intentionally has no OAuth callback
// completion path: only a future provider-bound callback verifier may make a
// pending OAuth request authorized.
type NativePendingDecisionService struct {
	store  NativePendingDecisionStore
	scopes nativecontract.ScopeResolver
}

func NewNativePendingDecisionService(store NativePendingDecisionStore, scopes nativecontract.ScopeResolver) *NativePendingDecisionService {
	return &NativePendingDecisionService{store: store, scopes: scopes}
}

func (s *NativePendingDecisionService) List(ctx context.Context, scope nativecontract.Scope, run nativecontract.RunIdentity, cursor string, limit int) (nativecontract.PendingDecisionPage, error) {
	authoritative, err := s.recheck(ctx, scope, nil)
	if err != nil {
		return nativecontract.PendingDecisionPage{}, err
	}
	if authoritative.TenantID != run.TenantID {
		return nativecontract.PendingDecisionPage{}, nativePendingServiceFailure(nativecontract.ErrNotFound, "pending run was not found")
	}
	page, err := s.store.List(ctx, authoritative, run, cursor, limit)
	if err != nil {
		return nativecontract.PendingDecisionPage{}, err
	}
	for _, detail := range page.Items {
		if !nativePendingDetailMatchesRun(detail, run) {
			return nativecontract.PendingDecisionPage{}, nativePendingServiceFailure(nativecontract.ErrNotFound, "pending run was not found")
		}
		if _, err := s.recheckDetail(ctx, authoritative, detail, "view"); err != nil {
			return nativecontract.PendingDecisionPage{}, err
		}
	}
	return page, nil
}

func (s *NativePendingDecisionService) Get(ctx context.Context, scope nativecontract.Scope, key nativecontract.PendingKey) (nativecontract.PendingDecisionDetail, error) {
	authoritative, err := s.recheck(ctx, scope, nil)
	if err != nil {
		return nativecontract.PendingDecisionDetail{}, err
	}
	if authoritative.TenantID != key.Run.TenantID {
		return nativecontract.PendingDecisionDetail{}, nativePendingServiceFailure(nativecontract.ErrNotFound, "pending decision was not found")
	}
	detail, err := s.store.Get(ctx, authoritative, key)
	if err != nil {
		return nativecontract.PendingDecisionDetail{}, err
	}
	if !nativePendingDetailMatchesKey(detail, key) {
		return nativecontract.PendingDecisionDetail{}, nativePendingServiceFailure(nativecontract.ErrNotFound, "pending decision was not found")
	}
	if _, err := s.recheckDetail(ctx, authoritative, detail, "view"); err != nil {
		return nativecontract.PendingDecisionDetail{}, err
	}
	return detail, nil
}

// BeginOAuth refuses to manufacture either an authorization URL or a completed
// callback. A provider-bound implementation must validate state, principal,
// service installation, and callback receipt before it can replace this seam.
func (s *NativePendingDecisionService) BeginOAuth(ctx context.Context, scope nativecontract.Scope, key nativecontract.PendingKey, _ nativecontract.OAuthStartRequest) (nativecontract.OAuthStartResult, error) {
	if _, err := s.Get(ctx, scope, key); err != nil {
		return nativecontract.OAuthStartResult{}, err
	}
	return nativecontract.OAuthStartResult{}, nativePendingServiceFailure(nativecontract.ErrStore, "OAuth callback authority is unavailable")
}

func (s *NativePendingDecisionService) Resolve(ctx context.Context, scope nativecontract.Scope, key nativecontract.PendingKey, req nativecontract.ResolvePendingRequest) (nativecontract.PendingResolution, error) {
	authoritative, err := s.recheck(ctx, scope, nil)
	if err != nil {
		return nativecontract.PendingResolution{}, err
	}
	if authoritative.TenantID != key.Run.TenantID {
		return nativecontract.PendingResolution{}, nativePendingServiceFailure(nativecontract.ErrNotFound, "pending decision was not found")
	}
	detail, err := s.store.Get(ctx, authoritative, key)
	if err != nil {
		return nativecontract.PendingResolution{}, err
	}
	if !nativePendingDetailMatchesKey(detail, key) {
		return nativecontract.PendingResolution{}, nativePendingServiceFailure(nativecontract.ErrNotFound, "pending decision was not found")
	}
	authoritative, err = s.recheckDetail(ctx, authoritative, detail, "resolve")
	if err != nil {
		return nativecontract.PendingResolution{}, err
	}
	// Recheck is deliberately before the CAS: revocation, expiry, or a policy
	// source failure does not leave a window in which a stale request can decide.
	return s.store.Resolve(ctx, authoritative, key, req)
}

func (s *NativePendingDecisionService) recheck(ctx context.Context, scope nativecontract.Scope, grants []nativecontract.ResourceGrant) (nativecontract.Scope, error) {
	if s == nil || s.store == nil || s.scopes == nil || scope.TenantID == 0 || scope.SessionOwnerID == "" {
		return nativecontract.Scope{}, nativePendingServiceFailure(nativecontract.ErrForbidden, "pending decision authority is unavailable")
	}
	authoritative, err := s.scopes.Recheck(ctx, scope, grants)
	if err != nil || authoritative.TenantID != scope.TenantID || authoritative.SessionOwnerID != scope.SessionOwnerID {
		return nativecontract.Scope{}, nativePendingServiceFailure(nativecontract.ErrForbidden, "pending decision authority was revoked")
	}
	return authoritative, nil
}

func (s *NativePendingDecisionService) recheckDetail(ctx context.Context, scope nativecontract.Scope, detail nativecontract.PendingDecisionDetail, action string) (nativecontract.Scope, error) {
	return s.recheck(ctx, scope, nativePendingDetailGrants(detail, action))
}

// Pending details are authorization bindings, not display-only metadata. Each
// non-empty identity component becomes an explicit server-side grant request;
// the ScopeResolver owns the meaning and current availability of those grants.
func nativePendingDetailGrants(detail nativecontract.PendingDecisionDetail, action string) []nativecontract.ResourceGrant {
	service := detail.Service
	grants := make([]nativecontract.ResourceGrant, 0, 4)
	if service.ResourceRef != "" {
		grants = append(grants, nativecontract.ResourceGrant{ResourceType: "native_pending_resource", ResourceID: service.ResourceRef, Action: action})
	}
	if service.ServiceID != "" {
		grants = append(grants, nativecontract.ResourceGrant{ResourceType: "native_pending_service", ResourceID: service.ServiceID, Action: action})
	}
	if service.InstallationID != "" {
		grants = append(grants, nativecontract.ResourceGrant{ResourceType: "native_pending_installation", ResourceID: service.InstallationID, Action: action})
	}
	if service.ToolName != "" {
		grants = append(grants, nativecontract.ResourceGrant{ResourceType: "native_pending_tool", ResourceID: service.ToolName, Action: action})
	}
	return grants
}

func nativePendingDetailMatchesRun(detail nativecontract.PendingDecisionDetail, run nativecontract.RunIdentity) bool {
	return detail.RunID == run.RunID && detail.SessionID == run.SessionID
}

func nativePendingDetailMatchesKey(detail nativecontract.PendingDecisionDetail, key nativecontract.PendingKey) bool {
	return detail.Ref.PendingID == key.PendingID && nativePendingDetailMatchesRun(detail, key.Run)
}

func nativePendingServiceFailure(code nativecontract.ErrorCode, message string) error {
	return &nativecontract.Failure{Code: code, Message: message, Effect: nativecontract.EffectNotDispatched}
}

var _ nativecontract.PendingDecisionService = (*NativePendingDecisionService)(nil)
