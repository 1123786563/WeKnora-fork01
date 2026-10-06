package service

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// This fixture applies the shipped dialect migrations to isolated tables. Only
// provider proof and scope lookup are faked; callback, pending CAS and the
// coordinator all use their real implementations.
func nativeOAuthAcceptanceFixture(t *testing.T, dialect string, provider nativecontract.PendingOAuthProvider) (*NativePendingDecisionService, nativecontract.Scope, nativecontract.PendingKey) {
	t.Helper()
	var dialector gorm.Dialector
	files := []string{"sqlite/000083_native_agent_schema.up.sql", "sqlite/000100_native_oauth_attempts.up.sql"}
	if dialect == "postgres" {
		dsn := os.Getenv("TRPC_TEST_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("TRPC_TEST_POSTGRES_DSN unset: PostgreSQL OAuth handoff acceptance NOT VERIFIED")
		}
		admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		schema := "oauth_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
		t.Cleanup(func() {
			require.NoError(t, admin.Exec("DROP SCHEMA "+schema+" CASCADE").Error)
			conn, err := admin.DB()
			require.NoError(t, err)
			require.NoError(t, conn.Close())
		})
		if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
			u, err := url.Parse(dsn)
			require.NoError(t, err)
			q := u.Query()
			q.Set("search_path", schema)
			u.RawQuery = q.Encode()
			dsn = u.String()
		} else {
			dsn += " search_path=" + schema
		}
		dialector = postgres.Open(dsn)
		files = []string{"versioned/000162_native_agent_schema.up.sql", "versioned/000179_native_oauth_attempts.up.sql"}
	} else {
		dialector = sqlite.Open(filepath.Join(t.TempDir(), "oauth.db") + "?_foreign_keys=on&_busy_timeout=5000")
	}
	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.Equal(t, dialect, db.Name())
	conn, err := db.DB()
	require.NoError(t, err)
	conn.SetMaxOpenConns(8)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	require.NoError(t, db.Exec("CREATE TABLE tenants (id BIGINT PRIMARY KEY)").Error)
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	for _, file := range files {
		migration, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "../../../migrations", file))
		require.NoError(t, err)
		require.NoError(t, db.Exec(string(migration)).Error)
	}
	for _, statement := range []string{
		"INSERT INTO tenants (id) VALUES (1)",
		"INSERT INTO native_agent_tenants (tenant_id) VALUES (1)",
		"INSERT INTO native_agent_sessions (tenant_id, owner_id, session_id) VALUES (1, 'owner', 'session')",
	} {
		require.NoError(t, db.Exec(statement).Error)
	}
	require.NoError(t, db.Exec("INSERT INTO native_agent_runs (tenant_id, run_id, session_id, owner_id, status, revision, lease_owner, lease_epoch, lease_expires_at) VALUES (1, 'run-1', 'session', 'owner', 'waiting_user', 4, 'worker', 1, ?)", time.Now().Add(time.Hour)).Error)
	key := nativecontract.PendingKey{Run: nativecontract.RunIdentity{TenantID: 1, SessionID: "session", RunID: "run-1"}, PendingID: "oauth-pending"}
	scope := nativecontract.Scope{TenantID: 1, SessionOwnerID: "owner", Principal: nativecontract.Principal{Type: "visitor", ID: "visitor-1"}}
	detail := nativePendingServiceDetail(key)
	detail.WaitKind = nativecontract.WaitOAuth
	detail.OAuth = &nativecontract.PendingOAuth{ServiceID: "svc-1", State: "required"}
	detail.ExpiresAt = detail.ExpiresAt.Truncate(time.Microsecond)
	detail.AllowedActions = append(detail.AllowedActions, nativecontract.DecisionTerminate)
	pending := repository.NewNativePendingDecisionRepository(db)
	require.NoError(t, pending.Create(context.Background(), key.Run, detail))
	return NewNativePendingDecisionServiceWithOAuthAuthority(pending, &nativePendingScopeResolverFake{result: scope}, NewNativePendingOAuthAuthority(repository.NewNativeOAuthRepository(db, nil), provider, []string{"https://app.example/callback"}, nil)), scope, key
}

func nativeOAuthAcceptanceStart(t *testing.T, s *NativePendingDecisionService, scope nativecontract.Scope, key nativecontract.PendingKey) nativecontract.PendingOAuthCallback {
	t.Helper()
	_, err := s.BeginOAuth(context.Background(), scope, key, nativecontract.OAuthStartRequest{RedirectURI: "https://app.example/callback"})
	require.NoError(t, err)
	return nativecontract.PendingOAuthCallback{AuthorizationAttempt: "attempt", State: "secret-state", Receipt: "verified-provider-proof"}
}

// Removing the in-transaction pending liveness check would consume a callback
// whose proof started before cancellation and finished after its durable commit.
func TestNativeOAuthAcceptanceCancelDuringProof(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			provider := &nativeOAuthProviderFake{onVerify: func() { close(entered); <-release }}
			s, scope, key := nativeOAuthAcceptanceFixture(t, dialect, provider)
			callback := nativeOAuthAcceptanceStart(t, s, scope, key)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			defer close(release)
			finished := make(chan error, 1)
			go func() {
				result, err := s.VerifyOAuthCallback(ctx, scope, key, "https://app.example/callback", callback)
				if result != (nativecontract.PendingOAuthVerificationResult{}) {
					finished <- fmt.Errorf("cancelled callback returned a receipt: %+v", result)
					return
				}
				finished <- err
			}()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("callback never entered proof verification")
			}
			request := nativePendingServiceRequest()
			request.Action = nativecontract.DecisionTerminate
			resolution, err := s.Resolve(ctx, scope, key, request)
			require.NoError(t, err)
			require.Equal(t, nativecontract.RunCancelled, resolution.RunStatus)
			// Release without closing so cleanup also releases on assertion failure.
			release <- struct{}{}
			select {
			case err := <-finished:
				var failure *nativecontract.Failure
				require.ErrorAs(t, err, &failure)
				require.Equal(t, nativecontract.ErrConflict, failure.Code)
				require.Equal(t, nativecontract.EffectNotDispatched, failure.Effect)
			case <-ctx.Done():
				t.Fatal("callback did not finish after cancellation")
			}
			stored, err := s.oauth.store.Get(ctx, scope, key, "attempt")
			require.NoError(t, err)
			require.EqualValues(t, 1, stored.Revision)
			require.Nil(t, stored.ConsumedAt)
			detail, err := s.Get(ctx, scope, key)
			require.NoError(t, err)
			require.Equal(t, "2", detail.Ref.Revision)
			require.Equal(t, "5", detail.RunRevision)
			require.Equal(t, nativecontract.RunCancelled, detail.RunStatus)
			reservation := repository.NewInMemoryNativeToolDispatchReservation(repository.NativeToolDispatchBudget{Root: key.Run, Available: 10})
			dispatch := nativePendingDispatch(key, scope)
			reservation.SetLiveFence(dispatch.Fence)
			identity, err := NewNativePendingConsumptionCoordinator(s, reservation).ResolveAndReserve(ctx, scope, key, nativePendingServiceRequest(), dispatch)
			var failure *nativecontract.Failure
			require.ErrorAs(t, err, &failure)
			require.Equal(t, nativecontract.ErrLeaseLost, failure.Code)
			require.Equal(t, nativecontract.EffectNotDispatched, failure.Effect)
			require.Equal(t, NativeResolvedPendingIdentity{}, identity)
			remaining, found := reservation.Remaining(key.Run)
			require.True(t, found)
			// A hold atomically deducts its positive quote, so the full balance
			// proves cancellation never reached reservation consumption.
			require.EqualValues(t, 10, remaining)
			request.DecisionID = "second-terminal-decision"
			_, err = s.Resolve(ctx, scope, key, request)
			require.Error(t, err, "exactly one terminal decision may commit")
		})
	}
}

// A receipt is evidence only. Turning OAuth into an approval in the repository
// would make these tests reserve budget and return a dispatch identity.
func TestNativeOAuthAcceptanceReceiptNeverAuthorizesHandoff(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			for _, scenario := range []string{"verified", "same-receipt replay", "different-receipt replay", "expired", "state mismatch", "service mismatch"} {
				t.Run(scenario, func(t *testing.T) {
					provider := &nativeOAuthProviderFake{}
					s, scope, key := nativeOAuthAcceptanceFixture(t, dialect, provider)
					callback := nativeOAuthAcceptanceStart(t, s, scope, key)
					ctx := context.Background()
					wantRevision := int64(1)
					if strings.Contains(scenario, "replay") {
						_, err := s.VerifyOAuthCallback(ctx, scope, key, "https://app.example/callback", callback)
						require.NoError(t, err)
						wantRevision = 2
					}
					switch scenario {
					case "different-receipt replay":
						callback.Receipt += "-different"
					case "expired":
						s.oauth.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
					case "state mismatch":
						callback.State = "wrong-state"
					case "service mismatch":
						provider.mutate = func(r *nativecontract.PendingOAuthReceipt) { r.Binding.ServiceID = "other-service" }
					}
					result, err := s.VerifyOAuthCallback(ctx, scope, key, "https://app.example/callback", callback)
					if scenario == "verified" || scenario == "same-receipt replay" {
						require.NoError(t, err)
						wantRevision = 2
					} else {
						require.Error(t, err)
						require.Equal(t, nativecontract.PendingOAuthVerificationResult{}, result)
					}
					reservation := repository.NewInMemoryNativeToolDispatchReservation(repository.NativeToolDispatchBudget{Root: key.Run, Available: 10})
					dispatch := nativePendingDispatch(key, scope)
					reservation.SetLiveFence(dispatch.Fence)
					identity, err := NewNativePendingConsumptionCoordinator(s, reservation).ResolveAndReserve(ctx, scope, key, nativePendingServiceRequest(), dispatch)
					var failure *nativecontract.Failure
					require.ErrorAs(t, err, &failure)
					require.Equal(t, nativecontract.ErrStore, failure.Code)
					require.Equal(t, nativecontract.EffectNotDispatched, failure.Effect)
					require.Equal(t, NativeResolvedPendingIdentity{}, identity)
					remaining, _ := reservation.Remaining(key.Run)
					require.EqualValues(t, 10, remaining)
					detail, err := s.Get(ctx, scope, key)
					require.NoError(t, err)
					require.Equal(t, nativecontract.PendingOpen, detail.Status)
					require.Equal(t, nativecontract.RunWaiting, detail.RunStatus)
					require.Equal(t, "1", detail.Ref.Revision)
					require.Equal(t, "4", detail.RunRevision)
					require.Equal(t, "required", detail.OAuth.State)
					stored, err := s.oauth.store.Get(ctx, scope, key, "attempt")
					require.NoError(t, err)
					require.Equal(t, wantRevision, stored.Revision)
				})
			}
		})
	}
}
