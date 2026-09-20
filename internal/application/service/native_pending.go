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
	authoritative, err := s.recheck(ctx, scope)
	if err != nil {
		return nativecontract.PendingDecisionPage{}, err
	}
	if authoritative.TenantID != run.TenantID {
		return nativecontract.PendingDecisionPage{}, nativePendingServiceFailure(nativecontract.ErrForbidden, "run is outside the current scope")
	}
	return s.store.List(ctx, authoritative, run, cursor, limit)
}

func (s *NativePendingDecisionService) Get(ctx context.Context, scope nativecontract.Scope, key nativecontract.PendingKey) (nativecontract.PendingDecisionDetail, error) {
	authoritative, err := s.recheck(ctx, scope)
	if err != nil {
		return nativecontract.PendingDecisionDetail{}, err
	}
	if authoritative.TenantID != key.Run.TenantID {
		return nativecontract.PendingDecisionDetail{}, nativePendingServiceFailure(nativecontract.ErrForbidden, "pending decision is outside the current scope")
	}
	return s.store.Get(ctx, authoritative, key)
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
	authoritative, err := s.recheck(ctx, scope)
	if err != nil {
		return nativecontract.PendingResolution{}, err
	}
	if authoritative.TenantID != key.Run.TenantID {
		return nativecontract.PendingResolution{}, nativePendingServiceFailure(nativecontract.ErrForbidden, "pending decision is outside the current scope")
	}
	// Recheck is deliberately before the CAS: revocation, expiry, or a policy
	// source failure does not leave a window in which a stale request can decide.
	return s.store.Resolve(ctx, authoritative, key, req)
}

func (s *NativePendingDecisionService) recheck(ctx context.Context, scope nativecontract.Scope) (nativecontract.Scope, error) {
	if s == nil || s.store == nil || s.scopes == nil || scope.TenantID == 0 || scope.SessionOwnerID == "" {
		return nativecontract.Scope{}, nativePendingServiceFailure(nativecontract.ErrForbidden, "pending decision authority is unavailable")
	}
	authoritative, err := s.scopes.Recheck(ctx, scope, scope.Grants)
	if err != nil || authoritative.TenantID != scope.TenantID || authoritative.SessionOwnerID != scope.SessionOwnerID {
		return nativecontract.Scope{}, nativePendingServiceFailure(nativecontract.ErrForbidden, "pending decision authority was revoked")
	}
	return authoritative, nil
}

func nativePendingServiceFailure(code nativecontract.ErrorCode, message string) error {
	return &nativecontract.Failure{Code: code, Message: message, Effect: nativecontract.EffectNotDispatched}
}

var _ nativecontract.PendingDecisionService = (*NativePendingDecisionService)(nil)
