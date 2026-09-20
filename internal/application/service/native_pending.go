package service

import (
	"context"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
)

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
