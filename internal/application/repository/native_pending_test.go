package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/stretchr/testify/require"
)

func nativePendingFixture(t *testing.T) (*NativePendingDecisionRepository, nativecontract.Scope, nativecontract.PendingKey) {
	t.Helper()
	db := openRunTestDB(t)
	require.NoError(t, db.Exec("INSERT INTO native_agent_tenants (tenant_id) VALUES (?)", 1).Error)
	require.NoError(t, db.Exec("INSERT INTO native_agent_sessions (tenant_id, owner_id, session_id) VALUES (?, ?, ?)", 1, "owner", "session").Error)
	require.NoError(t, db.Exec(`INSERT INTO native_agent_runs
		(tenant_id, run_id, owner_id, session_id, status, revision, lease_epoch)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, 1, "run-1", "owner", "session", "waiting_user", 4, 1).Error)
	key := nativecontract.PendingKey{Run: nativecontract.RunIdentity{TenantID: 1, SessionID: "session", RunID: "run-1"}, PendingID: "pending-1"}
	return NewNativePendingDecisionRepository(db), nativecontract.Scope{TenantID: 1, SessionOwnerID: "owner"}, key
}

func nativePendingDetail(key nativecontract.PendingKey) nativecontract.PendingDecisionDetail {
	return nativecontract.PendingDecisionDetail{
		Version: 1, Ref: nativecontract.PendingReference{PendingID: key.PendingID, Revision: "1"},
		SessionID: key.Run.SessionID, RunID: key.Run.RunID, CallID: "call-1", WaitKind: nativecontract.WaitApproval,
		Status: nativecontract.PendingOpen, RunStatus: nativecontract.RunWaiting, RunRevision: "4", PlanVersion: 2, ArgsHash: "args-v1",
		Service:              nativecontract.PendingServiceIdentity{Kind: "connector", ServiceID: "svc-1", ServiceName: "Calendar", ResourceRef: "calendar-1", ToolName: "create", RegisteredToolName: "connector.create", SchemaHash: "schema-v1"},
		OperationDescription: "Create a calendar event", RedactedArgs: json.RawMessage(`{"title":"Planning"}`), RedactedPaths: []string{"/attendees"}, RedactionVersion: "v1",
		ExpiresAt: time.Now().Add(time.Hour).UTC(), AllowedActions: []nativecontract.DecisionAction{nativecontract.DecisionRetry, nativecontract.DecisionTerminate},
	}
}

func nativeResolveRequest() nativecontract.ResolvePendingRequest {
	return nativecontract.ResolvePendingRequest{DecisionID: "decision-1", CallID: "call-1", ExpectedRevision: "4", PendingRevision: "1", PlanVersion: 2, ArgsHash: "args-v1", ResourceRef: "calendar-1", Action: nativecontract.DecisionRetry, Reason: "approved"}
}

func TestNativePendingDecisionRepositoryScopesDetailsAndConsumesCASOnce(t *testing.T) {
	repo, scope, key := nativePendingFixture(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, key.Run, nativePendingDetail(key)))

	got, err := repo.Get(ctx, scope, key)
	require.NoError(t, err)
	require.Equal(t, json.RawMessage(`{"title":"Planning"}`), got.RedactedArgs)
	require.Empty(t, got.OAuth)
	_, err = repo.Get(ctx, nativecontract.Scope{TenantID: 2, SessionOwnerID: "owner"}, key)
	require.Equal(t, nativecontract.ErrForbidden, failureCode(t, err))

	for _, changed := range []nativecontract.ResolvePendingRequest{
		func() nativecontract.ResolvePendingRequest {
			r := nativeResolveRequest()
			r.ExpectedRevision = "3"
			return r
		}(),
		func() nativecontract.ResolvePendingRequest {
			r := nativeResolveRequest()
			r.PendingRevision = "0"
			return r
		}(),
		func() nativecontract.ResolvePendingRequest { r := nativeResolveRequest(); r.PlanVersion = 3; return r }(),
		func() nativecontract.ResolvePendingRequest {
			r := nativeResolveRequest()
			r.ArgsHash = "changed"
			return r
		}(),
		func() nativecontract.ResolvePendingRequest {
			r := nativeResolveRequest()
			r.CallID = "other-call"
			return r
		}(),
		func() nativecontract.ResolvePendingRequest {
			r := nativeResolveRequest()
			r.ResourceRef = "other-resource"
			return r
		}(),
	} {
		_, err = repo.Resolve(ctx, scope, key, changed)
		require.Equal(t, nativecontract.ErrConflict, failureCode(t, err))
	}

	resolved, err := repo.Resolve(ctx, scope, key, nativeResolveRequest())
	require.NoError(t, err)
	require.Equal(t, nativecontract.PendingResolved, resolved.Detail.Status)
	require.Equal(t, nativecontract.RunQueued, resolved.RunStatus)
	require.Equal(t, "queued", resolved.ResumeState)
	require.Equal(t, "5", resolved.RunRevision)

	replayed, err := repo.Resolve(ctx, scope, key, nativeResolveRequest())
	require.NoError(t, err)
	require.Equal(t, resolved, replayed)
	changed := nativeResolveRequest()
	changed.Reason = "a different payload"
	_, err = repo.Resolve(ctx, scope, key, changed)
	require.Equal(t, nativecontract.ErrConflict, failureCode(t, err))
}

func TestNativePendingDecisionRepositoryRejectsExpiredAndCrossRunDecisionReplay(t *testing.T) {
	repo, scope, key := nativePendingFixture(t)
	ctx := context.Background()
	expired := nativePendingDetail(key)
	expired.ExpiresAt = time.Now().Add(-time.Minute)
	require.NoError(t, repo.Create(ctx, key.Run, expired))
	_, err := repo.Resolve(ctx, scope, key, nativeResolveRequest())
	require.Equal(t, nativecontract.ErrConflict, failureCode(t, err))

	page, err := repo.List(ctx, scope, key.Run, "", 100)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, nativecontract.PendingExpired, page.Items[0].Status)
}
