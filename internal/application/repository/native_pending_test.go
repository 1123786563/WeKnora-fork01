package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/nativecontract"
	"github.com/stretchr/testify/require"
)

// Multiple real transactions contend for the same run/pending pair. Mixed
// lock orders previously leaked SQLite busy errors or PostgreSQL deadlocks.
func TestNativePendingConcurrentCancelAndReservedResolution(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			repo, scope, key := nativePendingFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			require.NoError(t, repo.Create(ctx, key.Run, nativePendingDetail(key)))
			fence := nativecontract.Fence{Run: key.Run, Owner: "worker", Epoch: 1}
			require.NoError(t, repo.db.Exec("UPDATE native_agent_runs SET lease_owner=?, lease_expires_at=?", fence.Owner, time.Now().Add(time.Hour)).Error)
			reservation := NewInMemoryNativeToolDispatchReservation(NativeToolDispatchBudget{Root: key.Run, Available: 10})
			reservation.SetLiveFence(fence)
			dispatch := nativeUsageReservationRequest(fence, key.Run, 7)
			var reserved atomic.Int32
			start := make(chan struct{})
			type result struct {
				resolution nativecontract.PendingResolution
				err        error
			}
			results := make(chan result, 12)
			for i := 0; i < cap(results); i++ {
				i := i
				go func() {
					<-start
					req := nativeResolveRequest()
					req.DecisionID = fmt.Sprintf("decision-%d", i)
					if i%2 == 0 {
						req.Action = nativecontract.DecisionTerminate
						resolution, err := repo.Resolve(ctx, scope, key, req)
						results <- result{resolution, err}
					} else {
						resolution, err := repo.ResolveReserved(ctx, scope, key, req, fence, func() error {
							reserved.Add(1)
							return reservation.ReserveAndConsume(ctx, dispatch)
						})
						results <- result{resolution, err}
					}
				}()
			}
			close(start)
			var winner nativecontract.PendingResolution
			success := 0
			for i := 0; i < cap(results); i++ {
				select {
				case got := <-results:
					if got.err == nil {
						success++
						winner = got.resolution
						continue
					}
					var failure *nativecontract.Failure
					require.ErrorAs(t, got.err, &failure, "transaction errors must be controlled, not deadlocks/busy failures")
					require.Contains(t, []nativecontract.ErrorCode{nativecontract.ErrConflict, nativecontract.ErrLeaseLost}, failure.Code)
				case <-ctx.Done():
					t.Fatal("cancel/resolve transactions did not finish")
				}
			}
			require.Equal(t, 1, success)
			detail, err := repo.Get(ctx, scope, key)
			require.NoError(t, err)
			require.Equal(t, "2", detail.Ref.Revision)
			require.Equal(t, "5", detail.RunRevision)
			require.Equal(t, winner.Detail.ResolvedDecisionID, detail.ResolvedDecisionID)
			remaining, _ := reservation.Remaining(key.Run)
			if winner.RunStatus == nativecontract.RunCancelled {
				require.Zero(t, reserved.Load())
				require.EqualValues(t, 10, remaining)
				_, err = repo.ResolveReserved(ctx, scope, key, nativeResolveRequest(), fence, func() error { t.Error("reservation after terminal cancellation"); return nil })
				require.Equal(t, nativecontract.ErrLeaseLost, failureCode(t, err))
			} else {
				require.Equal(t, nativecontract.RunQueued, winner.RunStatus)
				require.EqualValues(t, 1, reserved.Load())
				require.EqualValues(t, 3, remaining)
			}
		})
	}
}

// Reservation denial must not advance either durable revision. A retry with
// the exact original decision must still be able to win the pending CAS.
func TestNativePendingReservedResolutionFailureRemainsReusable(t *testing.T) {
	repo, scope, key := nativePendingFixture(t)
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, key.Run, nativePendingDetail(key)))
	fence := nativecontract.Fence{Run: key.Run, Owner: "worker", Epoch: 1}
	require.NoError(t, repo.db.Exec("UPDATE native_agent_runs SET lease_owner=?, lease_expires_at=?", fence.Owner, time.Now().Add(time.Hour)).Error)
	denied := errors.New("reservation denied")
	_, err := repo.ResolveReserved(ctx, scope, key, nativeResolveRequest(), fence, func() error { return denied })
	require.ErrorIs(t, err, denied)
	detail, err := repo.Get(ctx, scope, key)
	require.NoError(t, err)
	require.Equal(t, nativecontract.PendingOpen, detail.Status)
	require.Equal(t, "4", detail.RunRevision)
	resolved, err := repo.ResolveReserved(ctx, scope, key, nativeResolveRequest(), fence, func() error { return nil })
	require.NoError(t, err)
	require.Equal(t, "2", resolved.Detail.Ref.Revision)
	replay, err := repo.ResolveReserved(ctx, scope, key, nativeResolveRequest(), fence, func() error { return nil })
	require.NoError(t, err)
	require.Equal(t, resolved, replay)
	changed := nativeResolveRequest()
	changed.Reason = "different payload"
	_, err = repo.ResolveReserved(ctx, scope, key, changed, fence, func() error { t.Fatal("changed decision reserved"); return nil })
	require.Equal(t, nativecontract.ErrConflict, failureCode(t, err))
}

func TestNativePendingReservedResolutionRejectsStaleFenceAndCancellation(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		repo, scope, key := nativePendingFixture(t)
		ctx := context.Background()
		require.NoError(t, repo.Create(ctx, key.Run, nativePendingDetail(key)))
		fence := nativecontract.Fence{Run: key.Run, Owner: "old-worker", Epoch: 1}
		if cancel {
			req := nativeResolveRequest()
			req.Action = nativecontract.DecisionTerminate
			_, err := repo.Resolve(ctx, scope, key, req)
			require.NoError(t, err)
		}
		_, err := repo.ResolveReserved(ctx, scope, key, nativeResolveRequest(), fence, func() error { t.Fatal("invalid decision reserved"); return nil })
		require.Error(t, err)
		detail, err := repo.Get(ctx, scope, key)
		require.NoError(t, err)
		if cancel {
			require.Equal(t, nativecontract.RunCancelled, detail.RunStatus)
		} else {
			require.Equal(t, nativecontract.PendingOpen, detail.Status)
		}
	}
}

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
	require.Equal(t, nativecontract.ErrNotFound, failureCode(t, err))
	_, err = repo.Get(ctx, nativecontract.Scope{TenantID: 1, SessionOwnerID: "another-owner"}, key)
	require.Equal(t, nativecontract.ErrNotFound, failureCode(t, err))

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

func TestNativePendingDecisionRepositoryHoldsOAuthAndUnknownEffectsWithoutProviderAuthority(t *testing.T) {
	for _, item := range []struct {
		name, externalState string
		wait                nativecontract.WaitKind
	}{
		{"oauth", "", nativecontract.WaitOAuth},
		{"unknown", "", nativecontract.WaitUnknown},
		{"external-action", "pending", nativecontract.WaitApproval},
	} {
		t.Run(item.name, func(t *testing.T) {
			repo, scope, key := nativePendingFixture(t)
			detail := nativePendingDetail(key)
			detail.WaitKind = item.wait
			detail.AllowedActions = []nativecontract.DecisionAction{nativecontract.DecisionRetry, nativecontract.DecisionTerminate}
			if item.wait == nativecontract.WaitOAuth {
				detail.OAuth = &nativecontract.PendingOAuth{ServiceID: "svc-1", State: "required"}
			}
			if item.externalState != "" {
				detail.ExternalActionID, detail.ExternalActionState = "action-1", item.externalState
			}
			require.NoError(t, repo.Create(context.Background(), key.Run, detail))
			_, err := repo.Resolve(context.Background(), scope, key, nativeResolveRequest())
			if item.wait == nativecontract.WaitOAuth || item.externalState != "" {
				require.Equal(t, nativecontract.ErrStore, failureCode(t, err))
			} else {
				require.Equal(t, nativecontract.ErrUnknownEffect, failureCode(t, err))
			}
			got, err := repo.Get(context.Background(), scope, key)
			require.NoError(t, err)
			require.Equal(t, nativecontract.PendingOpen, got.Status)
			require.Equal(t, nativecontract.RunWaiting, got.RunStatus)
		})
	}
}

func TestNativePendingDecisionRepositoryReplayReturnsSavedResolutionAfterRestart(t *testing.T) {
	repo, scope, key := nativePendingFixture(t)
	require.NoError(t, repo.Create(context.Background(), key.Run, nativePendingDetail(key)))
	first, err := repo.Resolve(context.Background(), scope, key, nativeResolveRequest())
	require.NoError(t, err)
	require.NoError(t, repo.db.Exec("UPDATE native_agent_runs SET status=?, revision=? WHERE tenant_id=? AND run_id=?", "failed", 99, 1, "run-1").Error)
	restarted := NewNativePendingDecisionRepository(repo.db)
	replayed, err := restarted.Resolve(context.Background(), scope, key, nativeResolveRequest())
	require.NoError(t, err)
	require.Equal(t, first, replayed)
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
