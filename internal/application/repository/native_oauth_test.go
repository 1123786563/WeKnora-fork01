package repository

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/stretchr/testify/require"
)

func durableOAuthFixture(t *testing.T) (*NativeOAuthRepository, nativecontract.Scope, NativeOAuthAttempt, *time.Time) {
	t.Helper()
	pending, scope, key := nativePendingFixture(t)
	scope.Principal = nativecontract.Principal{Type: "visitor", ID: "visitor-1"}
	now := time.Now().UTC().Truncate(time.Microsecond)
	detail := nativePendingDetail(key)
	detail.WaitKind = nativecontract.WaitOAuth
	detail.ExpiresAt = now.Add(time.Hour)
	detail.OAuth = &nativecontract.PendingOAuth{ServiceID: detail.Service.ServiceID, State: "required"}
	require.NoError(t, pending.Create(context.Background(), key.Run, detail))
	attempt := NativeOAuthAttempt{Binding: NativeOAuthBinding{Key: key, Principal: scope.Principal, SessionOwnerID: scope.SessionOwnerID,
		ServiceID: "svc-1", PendingRevision: 1, RedirectURI: "https://app.example/callback", ExpiresAt: now.Add(5 * time.Minute)},
		AttemptID: "attempt-1", StateHash: strings.Repeat("a", 64)}
	return NewNativeOAuthRepository(pending.db, func() time.Time { return now }), scope, attempt, &now
}

func TestNativeOAuthDurableIdentityAndReceiptReplay(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			r, scope, attempt, _ := durableOAuthFixture(t)
			ctx := context.Background()
			require.NoError(t, r.Create(ctx, scope, attempt))
			require.NoError(t, r.Create(ctx, scope, attempt), "identical creation is restart-safe")
			reopened := NewNativeOAuthRepository(reopenRunDB(t, r.db), r.now)
			got, err := reopened.Get(ctx, scope, attempt.Binding.Key, attempt.AttemptID)
			require.NoError(t, err)
			require.Equal(t, attempt.Binding, got.Binding)
			require.EqualValues(t, 1, got.Revision)
			require.Nil(t, got.ConsumedAt)
			payload, err := json.Marshal(got)
			require.NoError(t, err)
			require.NotContains(t, string(payload), attempt.StateHash)
			require.NotContains(t, string(payload), "StateHash")
			receiptHash := strings.Repeat("b", 64)
			first, err := reopened.Consume(ctx, scope, attempt, receiptHash)
			require.NoError(t, err)
			require.EqualValues(t, 2, first.Revision)
			require.NotNil(t, first.ConsumedAt)
			replay, err := r.Consume(ctx, scope, attempt, receiptHash)
			require.NoError(t, err)
			require.Equal(t, first, replay)
			_, err = r.Consume(ctx, scope, attempt, strings.Repeat("c", 64))
			require.Equal(t, nativecontract.ErrConflict, failureCode(t, err))
			require.Error(t, r.Create(ctx, scope, attempt), "creation must not resurrect a consumed attempt")
			detail, err := NewNativePendingDecisionRepository(r.db).Get(ctx, scope, attempt.Binding.Key)
			require.NoError(t, err)
			require.Equal(t, nativecontract.PendingOpen, detail.Status)
			require.Equal(t, "required", detail.OAuth.State)
			require.Equal(t, "1", detail.Ref.Revision)
		})
	}
}

func TestNativeOAuthDurableCommittedReceiptReplaysAfterExpiry(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			r, scope, attempt, now := durableOAuthFixture(t)
			ctx := context.Background()
			require.NoError(t, r.Create(ctx, scope, attempt))
			unconsumed := attempt
			unconsumed.AttemptID, unconsumed.StateHash = "attempt-2", strings.Repeat("d", 64)
			require.NoError(t, r.Create(ctx, scope, unconsumed))
			receiptHash := strings.Repeat("b", 64)
			first, err := r.Consume(ctx, scope, attempt, receiptHash)
			require.NoError(t, err)
			// Advance beyond both attempt and pending expiry. A committed receipt
			// is a historical read, while first consumption still requires liveness.
			*now = attempt.Binding.ExpiresAt.Add(time.Hour)
			reopened := NewNativeOAuthRepository(reopenRunDB(t, r.db), r.now)
			replay, err := reopened.Consume(ctx, scope, attempt, receiptHash)
			require.NoError(t, err)
			require.Equal(t, first, replay)
			_, err = reopened.Consume(ctx, scope, attempt, strings.Repeat("c", 64))
			require.Equal(t, nativecontract.ErrConflict, failureCode(t, err))
			_, err = reopened.Consume(ctx, scope, unconsumed, receiptHash)
			require.Equal(t, nativecontract.ErrConflict, failureCode(t, err))
			got, err := reopened.Get(ctx, scope, unconsumed.Binding.Key, unconsumed.AttemptID)
			require.NoError(t, err)
			require.EqualValues(t, 1, got.Revision)
			require.Nil(t, got.ConsumedAt)
		})
	}
}

func TestNativeOAuthDurableRejectsImmutableBindingChanges(t *testing.T) {
	mutations := map[string]func(*NativeOAuthAttempt){
		"tenant":              func(a *NativeOAuthAttempt) { a.Binding.Key.Run.TenantID++ },
		"principal":           func(a *NativeOAuthAttempt) { a.Binding.Principal.ID = "other" },
		"principal namespace": func(a *NativeOAuthAttempt) { a.Binding.Principal.Type = "user" },
		"owner":               func(a *NativeOAuthAttempt) { a.Binding.SessionOwnerID = "other" },
		"session":             func(a *NativeOAuthAttempt) { a.Binding.Key.Run.SessionID = "other" },
		"run":                 func(a *NativeOAuthAttempt) { a.Binding.Key.Run.RunID = "other" },
		"pending":             func(a *NativeOAuthAttempt) { a.Binding.Key.PendingID = "other" },
		"service":             func(a *NativeOAuthAttempt) { a.Binding.ServiceID = "other" },
		"installation":        func(a *NativeOAuthAttempt) { a.Binding.InstallationID = "other" },
		"redirect":            func(a *NativeOAuthAttempt) { a.Binding.RedirectURI += "/other" },
		"expiry":              func(a *NativeOAuthAttempt) { a.Binding.ExpiresAt = a.Binding.ExpiresAt.Add(time.Second) },
		"expiry precision":    func(a *NativeOAuthAttempt) { a.Binding.ExpiresAt = a.Binding.ExpiresAt.Add(time.Nanosecond) },
		"revision":            func(a *NativeOAuthAttempt) { a.Binding.PendingRevision++ },
		"state":               func(a *NativeOAuthAttempt) { a.StateHash = strings.Repeat("d", 64) },
		"attempt":             func(a *NativeOAuthAttempt) { a.AttemptID = "other" },
	}
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			r, scope, a, _ := durableOAuthFixture(t)
			ctx := context.Background()
			require.NoError(t, r.Create(ctx, scope, a))
			for name, mutate := range mutations {
				t.Run(name, func(t *testing.T) {
					changed := a
					mutate(&changed)
					_, err := r.Consume(ctx, scope, changed, strings.Repeat("b", 64))
					require.Error(t, err)
					require.Error(t, r.Create(ctx, scope, changed))
					stillOpen, err := r.Get(ctx, scope, a.Binding.Key, a.AttemptID)
					require.NoError(t, err)
					require.EqualValues(t, 1, stillOpen.Revision)
					require.Nil(t, stillOpen.ConsumedAt)
				})
			}
			for _, changed := range []nativecontract.Scope{
				{TenantID: 2, SessionOwnerID: scope.SessionOwnerID, Principal: scope.Principal},
				{TenantID: 1, SessionOwnerID: "other", Principal: scope.Principal},
				{TenantID: 1, SessionOwnerID: scope.SessionOwnerID, Principal: nativecontract.Principal{Type: "visitor", ID: "other"}},
			} {
				_, err := r.Get(ctx, changed, a.Binding.Key, a.AttemptID)
				require.Equal(t, nativecontract.ErrNotFound, failureCode(t, err))
			}
			_, err := r.Consume(ctx, scope, a, strings.Repeat("b", 64))
			require.NoError(t, err)
		})
	}
}

func TestNativeOAuthDurableExpiryAndUnverifiedProof(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			r, scope, a, now := durableOAuthFixture(t)
			ctx := context.Background()
			require.NoError(t, r.Create(ctx, scope, a))
			for _, invalid := range []string{"", "raw-provider-proof", strings.Repeat("z", 64)} {
				_, err := r.Consume(ctx, scope, a, invalid)
				require.Equal(t, nativecontract.ErrInvalid, failureCode(t, err))
			}
			*now = a.Binding.ExpiresAt
			_, err := r.Consume(ctx, scope, a, strings.Repeat("b", 64))
			require.Equal(t, nativecontract.ErrConflict, failureCode(t, err))
			got, err := r.Get(ctx, scope, a.Binding.Key, a.AttemptID)
			require.NoError(t, err)
			require.Nil(t, got.ConsumedAt)
			require.Error(t, r.Create(ctx, scope, a))
		})
	}
}

func TestNativeOAuthDurableCompetingReceipts(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			r, scope, a, _ := durableOAuthFixture(t)
			ctx := context.Background()
			require.NoError(t, r.Create(ctx, scope, a))
			other := NewNativeOAuthRepository(reopenRunDB(t, r.db), r.now)
			start := make(chan struct{})
			type outcome struct {
				hash string
				err  error
			}
			results := make(chan outcome, 12)
			for i := 0; i < cap(results); i++ {
				i := i
				go func() {
					<-start
					hash := strings.Repeat(string("abcdef012345"[i]), 64)
					repo := r
					if i%2 == 1 {
						repo = other
					}
					_, err := repo.Consume(ctx, scope, a, hash)
					results <- outcome{hash, err}
				}()
			}
			close(start)
			var winner string
			for i := 0; i < cap(results); i++ {
				result := <-results
				if result.err == nil {
					require.Empty(t, winner)
					winner = result.hash
				} else {
					require.Equal(t, nativecontract.ErrConflict, failureCode(t, result.err))
				}
			}
			require.NotEmpty(t, winner)
			replay, err := other.Consume(ctx, scope, a, winner)
			require.NoError(t, err)
			require.EqualValues(t, 2, replay.Revision)
		})
	}
}
