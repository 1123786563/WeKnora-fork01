package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/stretchr/testify/require"
)

type nativePendingScopeResolverFake struct {
	mu     sync.Mutex
	result nativecontract.Scope
	err    error
	errAt  int
	grants [][]nativecontract.ResourceGrant
}

func (f *nativePendingScopeResolverFake) Resolve(context.Context) (nativecontract.Scope, error) {
	return f.result, f.err
}
func (f *nativePendingScopeResolverFake) Recheck(_ context.Context, _ nativecontract.Scope, grants []nativecontract.ResourceGrant) (nativecontract.Scope, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.grants = append(f.grants, grants)
	if f.err != nil && (f.errAt == 0 || len(f.grants) == f.errAt) {
		return nativecontract.Scope{}, f.err
	}
	return f.result, nil
}

type nativePendingStoreFake struct {
	detail nativecontract.PendingDecisionDetail
}

func (s nativePendingStoreFake) List(_ context.Context, _ nativecontract.Scope, _ nativecontract.RunIdentity, _ string, _ int) (nativecontract.PendingDecisionPage, error) {
	return nativecontract.PendingDecisionPage{Items: []nativecontract.PendingDecisionDetail{s.detail}}, nil
}
func (s nativePendingStoreFake) Get(_ context.Context, _ nativecontract.Scope, _ nativecontract.PendingKey) (nativecontract.PendingDecisionDetail, error) {
	return s.detail, nil
}
func (s nativePendingStoreFake) Resolve(_ context.Context, _ nativecontract.Scope, _ nativecontract.PendingKey, _ nativecontract.ResolvePendingRequest) (nativecontract.PendingResolution, error) {
	return nativecontract.PendingResolution{Detail: s.detail}, nil
}

func nativePendingServiceFixture(t *testing.T, recheckErr error) (*NativePendingDecisionService, *nativePendingScopeResolverFake, nativecontract.Scope, nativecontract.PendingKey) {
	t.Helper()
	key := nativecontract.PendingKey{Run: nativecontract.RunIdentity{TenantID: 1, SessionID: "session", RunID: "run-1"}, PendingID: "pending-1"}
	detail := nativePendingServiceDetail(key)
	scope := nativecontract.Scope{TenantID: 1, SessionOwnerID: "owner", Grants: []nativecontract.ResourceGrant{{ResourceType: "connector", ResourceID: "calendar-1", Action: "write"}}}
	resolver := &nativePendingScopeResolverFake{result: scope, err: recheckErr, errAt: 2}
	return NewNativePendingDecisionService(nativePendingStoreFake{detail: detail}, resolver), resolver, scope, key
}

func nativePendingServiceDetail(key nativecontract.PendingKey) nativecontract.PendingDecisionDetail {
	return nativecontract.PendingDecisionDetail{Version: 1, Ref: nativecontract.PendingReference{PendingID: key.PendingID, Revision: "1"}, SessionID: key.Run.SessionID, RunID: key.Run.RunID, CallID: "call-1", WaitKind: nativecontract.WaitApproval, Status: nativecontract.PendingOpen, RunStatus: nativecontract.RunWaiting, RunRevision: "4", PlanVersion: 2, ArgsHash: "args-v1", Service: nativecontract.PendingServiceIdentity{Kind: "connector", ServiceID: "svc-1", InstallationID: "install-1", ResourceRef: "calendar-1", ToolName: "create", SchemaHash: "schema-v1"}, OperationDescription: "create", RedactedArgs: []byte(`{}`), ExpiresAt: nativePendingFuture(), AllowedActions: []nativecontract.DecisionAction{nativecontract.DecisionRetry}}
}

func nativePendingFuture() time.Time { return time.Now().Add(time.Hour).UTC() }

func nativePendingServiceRequest() nativecontract.ResolvePendingRequest {
	return nativecontract.ResolvePendingRequest{DecisionID: "decision-1", CallID: "call-1", ExpectedRevision: "4", PendingRevision: "1", PlanVersion: 2, ArgsHash: "args-v1", ResourceRef: "calendar-1", Action: nativecontract.DecisionRetry, Reason: "approved"}
}

func TestNativePendingDecisionServiceFailsClosedWhenScopeWasRevoked(t *testing.T) {
	svc, _, scope, key := nativePendingServiceFixture(t, errors.New("grant revoked"))
	_, err := svc.Resolve(context.Background(), scope, key, nativePendingServiceRequest())
	var failure *nativecontract.Failure
	require.True(t, errors.As(err, &failure))
	require.Equal(t, nativecontract.ErrForbidden, failure.Code)
}

func TestNativePendingDecisionServiceDoesNotForgeOAuthCompletion(t *testing.T) {
	svc, _, scope, key := nativePendingServiceFixture(t, nil)
	_, err := svc.BeginOAuth(context.Background(), scope, key, nativecontract.OAuthStartRequest{RedirectURI: "https://example.test/callback"})
	var failure *nativecontract.Failure
	require.True(t, errors.As(err, &failure))
	require.Equal(t, nativecontract.ErrStore, failure.Code)
}

func TestNativePendingDecisionServiceRechecksDetailGrantsForReads(t *testing.T) {
	for _, operation := range []struct {
		name string
		call func(*NativePendingDecisionService, nativecontract.Scope, nativecontract.PendingKey) error
	}{
		{"list", func(s *NativePendingDecisionService, scope nativecontract.Scope, key nativecontract.PendingKey) error {
			_, err := s.List(context.Background(), scope, key.Run, "", 10)
			return err
		}},
		{"get", func(s *NativePendingDecisionService, scope nativecontract.Scope, key nativecontract.PendingKey) error {
			_, err := s.Get(context.Background(), scope, key)
			return err
		}},
		{"resolve", func(s *NativePendingDecisionService, scope nativecontract.Scope, key nativecontract.PendingKey) error {
			_, err := s.Resolve(context.Background(), scope, key, nativePendingServiceRequest())
			return err
		}},
	} {
		t.Run(operation.name, func(t *testing.T) {
			svc, resolver, scope, key := nativePendingServiceFixture(t, errors.New("resource revoked"))
			var failure *nativecontract.Failure
			require.True(t, errors.As(operation.call(svc, scope, key), &failure))
			require.Equal(t, nativecontract.ErrForbidden, failure.Code)
			require.Len(t, resolver.grants, 2)
			require.Equal(t, []nativecontract.ResourceGrant{
				{ResourceType: "native_pending_resource", ResourceID: "calendar-1", Action: map[string]string{"list": "view", "get": "view", "resolve": "resolve"}[operation.name]},
				{ResourceType: "native_pending_service", ResourceID: "svc-1", Action: map[string]string{"list": "view", "get": "view", "resolve": "resolve"}[operation.name]},
				{ResourceType: "native_pending_installation", ResourceID: "install-1", Action: map[string]string{"list": "view", "get": "view", "resolve": "resolve"}[operation.name]},
				{ResourceType: "native_pending_tool", ResourceID: "create", Action: map[string]string{"list": "view", "get": "view", "resolve": "resolve"}[operation.name]},
			}, resolver.grants[1], "detail identity must be rechecked, not just the caller scope")
		})
	}
}

func TestNativePendingDecisionServiceHidesForeignTenantAndSession(t *testing.T) {
	for _, operation := range []struct {
		name string
		call func(*NativePendingDecisionService, nativecontract.Scope, nativecontract.PendingKey) error
	}{
		{"list", func(s *NativePendingDecisionService, scope nativecontract.Scope, key nativecontract.PendingKey) error {
			_, err := s.List(context.Background(), scope, key.Run, "", 10)
			return err
		}},
		{"get", func(s *NativePendingDecisionService, scope nativecontract.Scope, key nativecontract.PendingKey) error {
			_, err := s.Get(context.Background(), scope, key)
			return err
		}},
		{"resolve", func(s *NativePendingDecisionService, scope nativecontract.Scope, key nativecontract.PendingKey) error {
			_, err := s.Resolve(context.Background(), scope, key, nativePendingServiceRequest())
			return err
		}},
	} {
		for _, foreign := range []struct {
			name  string
			scope nativecontract.Scope
			key   func(nativecontract.PendingKey) nativecontract.PendingKey
		}{
			{"tenant", nativecontract.Scope{TenantID: 1, SessionOwnerID: "owner"}, func(key nativecontract.PendingKey) nativecontract.PendingKey { key.Run.TenantID = 2; return key }},
			{"session", nativecontract.Scope{TenantID: 1, SessionOwnerID: "owner"}, func(key nativecontract.PendingKey) nativecontract.PendingKey {
				key.Run.SessionID = "other-session"
				return key
			}},
		} {
			t.Run(operation.name+"/"+foreign.name, func(t *testing.T) {
				svc, _, _, key := nativePendingServiceFixture(t, nil)
				var failure *nativecontract.Failure
				require.True(t, errors.As(operation.call(svc, foreign.scope, foreign.key(key)), &failure))
				require.Equal(t, nativecontract.ErrNotFound, failure.Code)
			})
		}
	}
}
