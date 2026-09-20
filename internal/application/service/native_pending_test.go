package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/stretchr/testify/require"
)

type nativePendingScopeResolverFake struct {
	result nativecontract.Scope
	err    error
}

func (f nativePendingScopeResolverFake) Resolve(context.Context) (nativecontract.Scope, error) {
	return f.result, f.err
}
func (f nativePendingScopeResolverFake) Recheck(context.Context, nativecontract.Scope, []nativecontract.ResourceGrant) (nativecontract.Scope, error) {
	return f.result, f.err
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

func nativePendingServiceFixture(t *testing.T, recheckErr error) (*NativePendingDecisionService, nativecontract.Scope, nativecontract.PendingKey) {
	t.Helper()
	key := nativecontract.PendingKey{Run: nativecontract.RunIdentity{TenantID: 1, SessionID: "session", RunID: "run-1"}, PendingID: "pending-1"}
	detail := nativePendingServiceDetail(key)
	scope := nativecontract.Scope{TenantID: 1, SessionOwnerID: "owner", Grants: []nativecontract.ResourceGrant{{ResourceType: "connector", ResourceID: "calendar-1", Action: "write"}}}
	return NewNativePendingDecisionService(nativePendingStoreFake{detail: detail}, nativePendingScopeResolverFake{result: scope, err: recheckErr}), scope, key
}

func nativePendingServiceDetail(key nativecontract.PendingKey) nativecontract.PendingDecisionDetail {
	return nativecontract.PendingDecisionDetail{Version: 1, Ref: nativecontract.PendingReference{PendingID: key.PendingID, Revision: "1"}, SessionID: key.Run.SessionID, RunID: key.Run.RunID, CallID: "call-1", WaitKind: nativecontract.WaitApproval, Status: nativecontract.PendingOpen, RunStatus: nativecontract.RunWaiting, RunRevision: "4", PlanVersion: 2, ArgsHash: "args-v1", Service: nativecontract.PendingServiceIdentity{Kind: "connector", ResourceRef: "calendar-1", ToolName: "create", SchemaHash: "schema-v1"}, OperationDescription: "create", RedactedArgs: []byte(`{}`), ExpiresAt: nativePendingFuture(), AllowedActions: []nativecontract.DecisionAction{nativecontract.DecisionRetry}}
}

func nativePendingFuture() time.Time { return time.Now().Add(time.Hour).UTC() }

func nativePendingServiceRequest() nativecontract.ResolvePendingRequest {
	return nativecontract.ResolvePendingRequest{DecisionID: "decision-1", CallID: "call-1", ExpectedRevision: "4", PendingRevision: "1", PlanVersion: 2, ArgsHash: "args-v1", ResourceRef: "calendar-1", Action: nativecontract.DecisionRetry, Reason: "approved"}
}

func TestNativePendingDecisionServiceFailsClosedWhenScopeWasRevoked(t *testing.T) {
	svc, scope, key := nativePendingServiceFixture(t, errors.New("grant revoked"))
	_, err := svc.Resolve(context.Background(), scope, key, nativePendingServiceRequest())
	var failure *nativecontract.Failure
	require.True(t, errors.As(err, &failure))
	require.Equal(t, nativecontract.ErrForbidden, failure.Code)
}

func TestNativePendingDecisionServiceDoesNotForgeOAuthCompletion(t *testing.T) {
	svc, scope, key := nativePendingServiceFixture(t, nil)
	_, err := svc.BeginOAuth(context.Background(), scope, key, nativecontract.OAuthStartRequest{RedirectURI: "https://example.test/callback"})
	var failure *nativecontract.Failure
	require.True(t, errors.As(err, &failure))
	require.Equal(t, nativecontract.ErrStore, failure.Code)
}
