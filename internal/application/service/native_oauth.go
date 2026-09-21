package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/nativecontract"
)

// NativeOAuthAttemptStore is the durable authority port. Read metadata excludes
// secret state/proof; Consume accepts the hash of an already verified receipt.
type NativeOAuthAttemptStore interface {
	Create(context.Context, nativecontract.Scope, repository.NativeOAuthAttempt) error
	Get(context.Context, nativecontract.Scope, nativecontract.PendingKey, string) (repository.NativeOAuthMetadata, error)
	Consume(context.Context, nativecontract.Scope, repository.NativeOAuthAttempt, string) (repository.NativeOAuthMetadata, error)
}

// NativePendingOAuthAuthority is a non-production seam for deterministic
// provider fakes. No real adapter, token storage/query, callback route, or
// execution wiring exists. Receipt verification never authorizes Resolve.
type NativePendingOAuthAuthority struct {
	store     NativeOAuthAttemptStore
	provider  nativecontract.PendingOAuthProvider
	redirects map[string]bool
	now       func() time.Time
}

func NewNativePendingOAuthAuthority(store NativeOAuthAttemptStore, provider nativecontract.PendingOAuthProvider, redirects []string, now func() time.Time) *NativePendingOAuthAuthority {
	if now == nil {
		now = time.Now
	}
	a := &NativePendingOAuthAuthority{store: store, provider: provider, redirects: make(map[string]bool), now: now}
	for _, redirect := range redirects {
		u, err := url.Parse(redirect)
		if err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Fragment == "" {
			a.redirects[redirect] = true
		}
	}
	return a
}

// NewNativePendingDecisionServiceWithOAuthAuthority is explicit test composition;
// the ordinary constructor retains the durable fail-closed default.
func NewNativePendingDecisionServiceWithOAuthAuthority(store NativePendingDecisionStore, scopes nativecontract.ScopeResolver, authority *NativePendingOAuthAuthority) *NativePendingDecisionService {
	s := NewNativePendingDecisionService(store, scopes)
	s.oauth = authority
	return s
}

func (s *NativePendingDecisionService) oauthDetail(ctx context.Context, scope nativecontract.Scope, key nativecontract.PendingKey) (nativecontract.PendingDecisionDetail, nativecontract.Scope, error) {
	detail, err := s.Get(ctx, scope, key)
	if err != nil {
		return nativecontract.PendingDecisionDetail{}, nativecontract.Scope{}, err
	}
	current, err := s.recheckDetail(ctx, scope, detail, "resolve")
	if err != nil {
		return nativecontract.PendingDecisionDetail{}, nativecontract.Scope{}, err
	}
	// OAuth identity must never be silently substituted by the scope resolver.
	if current.Principal != scope.Principal {
		return nativecontract.PendingDecisionDetail{}, nativecontract.Scope{}, nativePendingServiceFailure(nativecontract.ErrForbidden, "OAuth principal changed")
	}
	return detail, current, nil
}

func (a *NativePendingOAuthAuthority) binding(scope nativecontract.Scope, key nativecontract.PendingKey, d nativecontract.PendingDecisionDetail, redirect string) (nativecontract.PendingOAuthBinding, error) {
	if a == nil || a.store == nil || a.provider == nil || a.now == nil {
		return nativecontract.PendingOAuthBinding{}, nativePendingServiceFailure(nativecontract.ErrStore, "OAuth callback authority is unavailable")
	}
	if scope.TenantID != key.Run.TenantID || scope.Principal.Type == "" || scope.Principal.ID == "" || scope.SessionOwnerID == "" || !nativePendingDetailMatchesKey(d, key) || d.WaitKind != nativecontract.WaitOAuth || d.Status != nativecontract.PendingOpen || d.RunStatus != nativecontract.RunWaiting || d.Ref.Revision == "" || d.OAuth == nil || d.Service.ServiceID == "" || d.OAuth.ServiceID != d.Service.ServiceID || !d.ExpiresAt.After(a.now()) || !a.redirects[redirect] {
		return nativecontract.PendingOAuthBinding{}, nativePendingServiceFailure(nativecontract.ErrInvalid, "OAuth pending binding is invalid or expired")
	}
	return nativecontract.PendingOAuthBinding{Key: key, Principal: scope.Principal, SessionOwnerID: scope.SessionOwnerID, ServiceID: d.Service.ServiceID, InstallationID: d.Service.InstallationID, PendingRevision: d.Ref.Revision, RedirectURI: redirect, ExpiresAt: d.ExpiresAt.UTC().Truncate(time.Microsecond)}, nil
}

func (a *NativePendingOAuthAuthority) begin(ctx context.Context, scope nativecontract.Scope, key nativecontract.PendingKey, detail nativecontract.PendingDecisionDetail, req nativecontract.OAuthStartRequest) (nativecontract.OAuthStartResult, error) {
	b, err := a.binding(scope, key, detail, req.RedirectURI)
	if err != nil {
		return nativecontract.OAuthStartResult{}, err
	}
	result, state, err := a.provider.Prepare(ctx, b)
	if err != nil {
		return nativecontract.OAuthStartResult{}, nativePendingServiceFailure(nativecontract.ErrStore, "OAuth preparation failed")
	}
	u, parseErr := url.Parse(result.AuthorizationURL)
	if parseErr != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || !result.ExpiresAt.Equal(b.ExpiresAt) || result.AuthorizationAttempt == "" || state == "" {
		return nativecontract.OAuthStartResult{}, nativePendingServiceFailure(nativecontract.ErrStore, "OAuth preparation receipt is invalid")
	}
	attempt, err := nativeOAuthDurableAttempt(b, result.AuthorizationAttempt, state)
	if err != nil {
		return nativecontract.OAuthStartResult{}, err
	}
	if err := a.store.Create(ctx, scope, attempt); err != nil {
		return nativecontract.OAuthStartResult{}, err
	}
	return result, nil
}

// VerifyOAuthCallback verifies proof and rechecks authority BEFORE durable CAS.
// Failed proof leaves revision 1 untouched. A popup message is not evidence;
// even a verified receipt does not query tokens, resolve pending, or dispatch.
func (s *NativePendingDecisionService) VerifyOAuthCallback(ctx context.Context, scope nativecontract.Scope, key nativecontract.PendingKey, redirect string, callback nativecontract.PendingOAuthCallback) (nativecontract.PendingOAuthVerificationResult, error) {
	var empty nativecontract.PendingOAuthVerificationResult
	detail, current, err := s.oauthDetail(ctx, scope, key)
	if err != nil {
		return empty, err
	}
	a := s.oauth
	if a == nil || a.store == nil || a.provider == nil || a.now == nil {
		return empty, nativePendingServiceFailure(nativecontract.ErrStore, "OAuth callback authority is unavailable")
	}
	if !a.redirects[redirect] || callback.AuthorizationAttempt == "" || callback.State == "" || callback.Receipt == "" {
		return empty, nativePendingServiceFailure(nativecontract.ErrInvalid, "OAuth callback is incomplete or redirect is not allowed")
	}
	stored, err := a.store.Get(ctx, current, key, callback.AuthorizationAttempt)
	if err != nil {
		return empty, err
	}
	b := nativeOAuthProviderBinding(stored.Binding)
	if b.Key != key || b.Principal != current.Principal || b.SessionOwnerID != current.SessionOwnerID || b.RedirectURI != redirect || b.ServiceID != detail.Service.ServiceID || b.InstallationID != detail.Service.InstallationID || detail.OAuth == nil || detail.OAuth.ServiceID != b.ServiceID || stored.AttemptID != callback.AuthorizationAttempt {
		return empty, nativePendingServiceFailure(nativecontract.ErrConflict, "OAuth callback binding does not match")
	}
	// A committed receipt remains replayable after expiry. First consumption
	// still requires live pending state and is rechecked in the SQL transaction.
	if stored.Revision != 2 && (stored.Revision != 1 || detail.Status != nativecontract.PendingOpen || detail.WaitKind != nativecontract.WaitOAuth || detail.RunStatus != nativecontract.RunWaiting || detail.Ref.Revision != b.PendingRevision || !b.ExpiresAt.After(a.now())) {
		return empty, nativePendingServiceFailure(nativecontract.ErrConflict, "OAuth pending is stale or expired")
	}
	attempt := nativecontract.PendingOAuthAttempt{Binding: b, AuthorizationAttempt: callback.AuthorizationAttempt, State: callback.State}
	receipt, err := a.provider.Verify(ctx, attempt, callback.Receipt)
	if err != nil || receipt.Binding != b || receipt.AuthorizationAttempt != attempt.AuthorizationAttempt {
		return empty, nativePendingServiceFailure(nativecontract.ErrForbidden, "OAuth callback proof is invalid")
	}
	// A verifier may take time; reject revocation before consuming the attempt.
	latest, err := s.recheckDetail(ctx, current, detail, "resolve")
	if err != nil {
		return empty, err
	}
	if latest.Principal != current.Principal {
		return empty, nativePendingServiceFailure(nativecontract.ErrForbidden, "OAuth principal changed")
	}
	durable, err := nativeOAuthDurableAttempt(b, callback.AuthorizationAttempt, callback.State)
	if err != nil {
		return empty, err
	}
	committed, err := a.store.Consume(ctx, latest, durable, nativeOAuthSecretHash(callback.Receipt))
	if err != nil {
		return empty, err
	}
	if committed.Binding != stored.Binding || committed.AttemptID != stored.AttemptID || committed.Revision != 2 || committed.ConsumedAt == nil {
		return empty, nativePendingServiceFailure(nativecontract.ErrStore, "OAuth consumption receipt is invalid")
	}
	return nativecontract.PendingOAuthVerificationResult{PendingID: key.PendingID, AuthorizationAttempt: committed.AttemptID, ServiceID: b.ServiceID, Revision: "2", VerifiedAt: committed.ConsumedAt.UTC()}, nil
}

func nativeOAuthSecretHash(secret string) string {
	hash := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(hash[:])
}

func nativeOAuthDurableAttempt(b nativecontract.PendingOAuthBinding, attemptID, state string) (repository.NativeOAuthAttempt, error) {
	revision, err := strconv.ParseInt(b.PendingRevision, 10, 64)
	if err != nil || revision < 1 {
		return repository.NativeOAuthAttempt{}, nativePendingServiceFailure(nativecontract.ErrInvalid, "OAuth pending revision is invalid")
	}
	return repository.NativeOAuthAttempt{Binding: repository.NativeOAuthBinding{Key: b.Key, Principal: b.Principal, SessionOwnerID: b.SessionOwnerID, ServiceID: b.ServiceID, InstallationID: b.InstallationID, RedirectURI: b.RedirectURI, PendingRevision: revision, ExpiresAt: b.ExpiresAt}, AttemptID: attemptID, StateHash: nativeOAuthSecretHash(state)}, nil
}

func nativeOAuthProviderBinding(b repository.NativeOAuthBinding) nativecontract.PendingOAuthBinding {
	return nativecontract.PendingOAuthBinding{Key: b.Key, Principal: b.Principal, SessionOwnerID: b.SessionOwnerID, ServiceID: b.ServiceID, InstallationID: b.InstallationID, RedirectURI: b.RedirectURI, PendingRevision: strconv.FormatInt(b.PendingRevision, 10), ExpiresAt: b.ExpiresAt}
}
