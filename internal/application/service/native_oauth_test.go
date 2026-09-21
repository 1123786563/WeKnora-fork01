package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/nativecontract"
	"github.com/stretchr/testify/require"
)

type nativeOAuthProviderFake struct {
	verifyErr error
	mutate    func(*nativecontract.PendingOAuthReceipt)
	onVerify  func()
}

func (f *nativeOAuthProviderFake) Prepare(_ context.Context, b nativecontract.PendingOAuthBinding) (nativecontract.OAuthStartResult, string, error) {
	return nativecontract.OAuthStartResult{AuthorizationURL: "https://provider.example/authorize", AuthorizationAttempt: "attempt", ExpiresAt: b.ExpiresAt}, "secret-state", nil
}
func (f *nativeOAuthProviderFake) Verify(_ context.Context, a nativecontract.PendingOAuthAttempt, proof string) (nativecontract.PendingOAuthReceipt, error) {
	if f.onVerify != nil {
		f.onVerify()
	}
	if f.verifyErr != nil {
		return nativecontract.PendingOAuthReceipt{}, f.verifyErr
	}
	if !strings.HasPrefix(proof, "verified-provider-proof") {
		return nativecontract.PendingOAuthReceipt{}, errors.New("secret provider error")
	}
	r := nativecontract.PendingOAuthReceipt{Binding: a.Binding, AuthorizationAttempt: a.AuthorizationAttempt}
	if f.mutate != nil {
		f.mutate(&r)
	}
	return r, nil
}

func nativeOAuthServiceFixture(t *testing.T, provider nativecontract.PendingOAuthProvider) (*NativePendingDecisionService, nativecontract.Scope, nativecontract.PendingKey) {
	t.Helper()
	db, pending, coordinator, _, scope, key := nativePendingCoordinatorDB(t)
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	schema, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "../../../migrations/sqlite/000100_native_oauth_attempts.up.sql"))
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(schema)).Error)
	s := coordinator.pending
	resolver := s.scopes.(*nativePendingScopeResolverFake)
	key.PendingID = "oauth-pending"
	scope.Principal = nativecontract.Principal{Type: "visitor", ID: "visitor-1"}
	resolver.result = scope
	d := nativePendingServiceDetail(key)
	d.WaitKind = nativecontract.WaitOAuth
	d.OAuth = &nativecontract.PendingOAuth{ServiceID: "svc-1", State: "required"}
	d.RedactedArgs = []byte(`{"password":"[redacted]"}`)
	d.ExpiresAt = d.ExpiresAt.UTC().Truncate(time.Microsecond)
	require.NoError(t, pending.Create(context.Background(), key.Run, d))
	s = NewNativePendingDecisionServiceWithOAuthAuthority(pending, resolver, NewNativePendingOAuthAuthority(repository.NewNativeOAuthRepository(db, nil), provider, []string{"https://app.example/callback"}, time.Now))
	return s, scope, key
}

func TestNativeOAuthAuthorityVerifiesProofWithoutResolvingOrExposingSecrets(t *testing.T) {
	s, scope, key := nativeOAuthServiceFixture(t, &nativeOAuthProviderFake{})
	ctx := context.Background()
	start, err := s.BeginOAuth(ctx, scope, key, nativecontract.OAuthStartRequest{RedirectURI: "https://app.example/callback"})
	require.NoError(t, err)
	require.Equal(t, "attempt", start.AuthorizationAttempt)
	receipt, err := s.VerifyOAuthCallback(ctx, scope, key, "https://app.example/callback", nativecontract.PendingOAuthCallback{AuthorizationAttempt: "attempt", State: "secret-state", Receipt: "verified-provider-proof"})
	require.NoError(t, err)
	require.Equal(t, "svc-1", receipt.ServiceID)
	require.Equal(t, "2", receipt.Revision)
	detail, err := s.Get(ctx, scope, key)
	require.NoError(t, err)
	require.Equal(t, nativecontract.PendingOpen, detail.Status)
	require.Equal(t, "required", detail.OAuth.State, "callback proof is not a live token or tool approval")
	require.JSONEq(t, `{"password":"[redacted]"}`, string(detail.RedactedArgs))
	encoded, err := json.Marshal(detail)
	require.NoError(t, err)
	for _, secret := range []string{"secret-state", "verified-provider-proof", "authorization_attempt"} {
		require.NotContains(t, string(encoded), secret)
	}
	replay, err := s.VerifyOAuthCallback(ctx, scope, key, "https://app.example/callback", nativecontract.PendingOAuthCallback{AuthorizationAttempt: "attempt", State: "secret-state", Receipt: "verified-provider-proof"})
	require.NoError(t, err)
	require.Equal(t, receipt, replay)
	encoded, err = json.Marshal(receipt)
	require.NoError(t, err)
	for _, secret := range []string{"secret-state", "verified-provider-proof", "password", "principal", "redirect", "hash"} {
		require.NotContains(t, strings.ToLower(string(encoded)), secret)
	}
}

func TestNativeOAuthAuthorityRejectsProofFailureAndLeavesAttemptReusable(t *testing.T) {
	for name, provider := range map[string]*nativeOAuthProviderFake{
		"provider failure": {verifyErr: errors.New("secret callback proof")},
		"wrong tenant":     {mutate: func(r *nativecontract.PendingOAuthReceipt) { r.Binding.Key.Run.TenantID++ }},
		"wrong principal":  {mutate: func(r *nativecontract.PendingOAuthReceipt) { r.Binding.Principal.ID = "other" }},
		"wrong service":    {mutate: func(r *nativecontract.PendingOAuthReceipt) { r.Binding.ServiceID = "other" }},
		"wrong redirect":   {mutate: func(r *nativecontract.PendingOAuthReceipt) { r.Binding.RedirectURI = "https://evil.example" }},
		"wrong attempt":    {mutate: func(r *nativecontract.PendingOAuthReceipt) { r.AuthorizationAttempt = "other" }},
		"wrong expiry": {mutate: func(r *nativecontract.PendingOAuthReceipt) {
			r.Binding.ExpiresAt = r.Binding.ExpiresAt.Add(time.Second)
		}},
	} {
		t.Run(name, func(t *testing.T) {
			s, scope, key := nativeOAuthServiceFixture(t, provider)
			ctx := context.Background()
			_, err := s.BeginOAuth(ctx, scope, key, nativecontract.OAuthStartRequest{RedirectURI: "https://app.example/callback"})
			require.NoError(t, err)
			callback := nativecontract.PendingOAuthCallback{AuthorizationAttempt: "attempt", State: "secret-state", Receipt: "verified-provider-proof"}
			got, err := s.VerifyOAuthCallback(ctx, scope, key, "https://app.example/callback", callback)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret callback proof")
			require.Equal(t, nativecontract.PendingOAuthVerificationResult{}, got)
			stored, err := s.oauth.store.Get(ctx, scope, key, "attempt")
			require.NoError(t, err)
			require.EqualValues(t, 1, stored.Revision)
			require.Nil(t, stored.ConsumedAt)
			provider.verifyErr, provider.mutate = nil, nil
			_, err = s.VerifyOAuthCallback(ctx, scope, key, "https://app.example/callback", callback)
			require.NoError(t, err, "failed verification must leave the attempt available for a valid proof")
		})
	}
}

func TestNativeOAuthAuthorityRejectsUnboundStart(t *testing.T) {
	for name, mutate := range map[string]func(*NativePendingDecisionService, *nativecontract.Scope, *nativecontract.OAuthStartRequest){
		"redirect": func(_ *NativePendingDecisionService, _ *nativecontract.Scope, r *nativecontract.OAuthStartRequest) {
			r.RedirectURI += "/unlisted"
		},
		"no provider": func(s *NativePendingDecisionService, _ *nativecontract.Scope, _ *nativecontract.OAuthStartRequest) {
			s.oauth.provider = nil
		},
		"no principal": func(s *NativePendingDecisionService, scope *nativecontract.Scope, _ *nativecontract.OAuthStartRequest) {
			scope.Principal = nativecontract.Principal{}
			s.scopes.(*nativePendingScopeResolverFake).result = *scope
		},
		"service mismatch": func(s *NativePendingDecisionService, _ *nativecontract.Scope, _ *nativecontract.OAuthStartRequest) {
			f := s.store.(nativePendingStoreFake)
			f.detail.OAuth.ServiceID = "other"
			s.store = f
		},
		"expired": func(s *NativePendingDecisionService, _ *nativecontract.Scope, _ *nativecontract.OAuthStartRequest) {
			f := s.store.(nativePendingStoreFake)
			f.detail.ExpiresAt = time.Now().Add(-time.Minute)
			s.store = f
		},
		"closed pending": func(s *NativePendingDecisionService, _ *nativecontract.Scope, _ *nativecontract.OAuthStartRequest) {
			f := s.store.(nativePendingStoreFake)
			f.detail.Status = nativecontract.PendingCancelled
			s.store = f
		},
	} {
		t.Run(name, func(t *testing.T) {
			s, scope, key := nativeOAuthServiceFixture(t, &nativeOAuthProviderFake{})
			detail, err := s.Get(context.Background(), scope, key)
			require.NoError(t, err)
			s.store = nativePendingStoreFake{detail: detail}
			r := nativecontract.OAuthStartRequest{RedirectURI: "https://app.example/callback"}
			mutate(s, &scope, &r)
			result, err := s.BeginOAuth(context.Background(), scope, key, r)
			require.Error(t, err)
			require.Equal(t, nativecontract.OAuthStartResult{}, result)
		})
	}
}

func TestNativeOAuthAuthorityWrongCallbackLeavesDurableAttemptOpen(t *testing.T) {
	for name, mutate := range map[string]func(*nativecontract.PendingOAuthCallback, *string){
		"state":         func(c *nativecontract.PendingOAuthCallback, _ *string) { c.State = "other-state" },
		"attempt":       func(c *nativecontract.PendingOAuthCallback, _ *string) { c.AuthorizationAttempt = "other-attempt" },
		"empty proof":   func(c *nativecontract.PendingOAuthCallback, _ *string) { c.Receipt = "" },
		"invalid proof": func(c *nativecontract.PendingOAuthCallback, _ *string) { c.Receipt = "invalid-proof" },
		"redirect":      func(_ *nativecontract.PendingOAuthCallback, r *string) { *r = "https://other.example/callback" },
	} {
		t.Run(name, func(t *testing.T) {
			s, scope, key := nativeOAuthServiceFixture(t, &nativeOAuthProviderFake{})
			ctx := context.Background()
			_, err := s.BeginOAuth(ctx, scope, key, nativecontract.OAuthStartRequest{RedirectURI: "https://app.example/callback"})
			require.NoError(t, err)
			callback := nativecontract.PendingOAuthCallback{AuthorizationAttempt: "attempt", State: "secret-state", Receipt: "verified-provider-proof"}
			redirect := "https://app.example/callback"
			mutate(&callback, &redirect)
			_, err = s.VerifyOAuthCallback(ctx, scope, key, redirect, callback)
			require.Error(t, err)
			stored, err := s.oauth.store.Get(ctx, scope, key, "attempt")
			require.NoError(t, err)
			require.EqualValues(t, 1, stored.Revision)
			require.Nil(t, stored.ConsumedAt)
		})
	}
}

func TestNativeOAuthAuthorityConcurrentProofsConsumeExactlyOnce(t *testing.T) {
	s, scope, key := nativeOAuthServiceFixture(t, &nativeOAuthProviderFake{})
	ctx := context.Background()
	_, err := s.BeginOAuth(ctx, scope, key, nativecontract.OAuthStartRequest{RedirectURI: "https://app.example/callback"})
	require.NoError(t, err)
	type outcome struct {
		proof  string
		result nativecontract.PendingOAuthVerificationResult
		err    error
	}
	start := make(chan struct{})
	results := make(chan outcome, 12)
	for i := 0; i < cap(results); i++ {
		i := i
		go func() {
			<-start
			proof := fmt.Sprintf("verified-provider-proof-%d", i)
			result, err := s.VerifyOAuthCallback(ctx, scope, key, "https://app.example/callback", nativecontract.PendingOAuthCallback{AuthorizationAttempt: "attempt", State: "secret-state", Receipt: proof})
			results <- outcome{proof, result, err}
		}()
	}
	close(start)
	var winner outcome
	for i := 0; i < cap(results); i++ {
		result := <-results
		if result.err == nil {
			require.Empty(t, winner.proof)
			winner = result
		} else {
			var failure *nativecontract.Failure
			require.ErrorAs(t, result.err, &failure)
			require.Equal(t, nativecontract.ErrConflict, failure.Code)
		}
	}
	require.NotEmpty(t, winner.proof)
	replay, err := s.VerifyOAuthCallback(ctx, scope, key, "https://app.example/callback", nativecontract.PendingOAuthCallback{AuthorizationAttempt: "attempt", State: "secret-state", Receipt: winner.proof})
	require.NoError(t, err)
	require.Equal(t, winner.result, replay)
	stored, err := s.oauth.store.Get(ctx, scope, key, "attempt")
	require.NoError(t, err)
	require.EqualValues(t, 2, stored.Revision)
}

func TestNativeOAuthAuthorityRechecksRevocationBeforeConsumption(t *testing.T) {
	provider := &nativeOAuthProviderFake{}
	s, scope, key := nativeOAuthServiceFixture(t, provider)
	ctx := context.Background()
	_, err := s.BeginOAuth(ctx, scope, key, nativecontract.OAuthStartRequest{RedirectURI: "https://app.example/callback"})
	require.NoError(t, err)
	provider.onVerify = func() {
		resolver := s.scopes.(*nativePendingScopeResolverFake)
		resolver.err = errors.New("revoked")
		resolver.errAt = 0
	}
	_, err = s.VerifyOAuthCallback(ctx, scope, key, "https://app.example/callback", nativecontract.PendingOAuthCallback{AuthorizationAttempt: "attempt", State: "secret-state", Receipt: "verified-provider-proof"})
	require.Error(t, err)
	stored, err := s.oauth.store.Get(ctx, scope, key, "attempt")
	require.NoError(t, err)
	require.EqualValues(t, 1, stored.Revision)
	require.Nil(t, stored.ConsumedAt)
}
