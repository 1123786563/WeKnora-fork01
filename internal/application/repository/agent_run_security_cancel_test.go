package repository

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// seedSecurityCancelFixture reuses openRunTestDB's tenant 1 / u1 / s1 / s2,
// then adds tenant 2 and four runs for scope, status and agent matching.
func seedSecurityCancelFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (2, 't2', 'test')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES ('u2','u2','u2@example.test','x',2)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type, active_agent_run_id) VALUES
		('s3', 2, 'task-3', 'u2', 'trpc', 'sec-r4')`).Error)
	require.NoError(t, db.Exec(`UPDATE sessions SET active_agent_run_id = 'sec-r1' WHERE tenant_id = 1 AND id = 's1'`).Error)
	blocked := `{"session_id":"s1","agent_id":"local-agent-blocked","request_id":"req-1","text":"hi"}`
	clean := `{"session_id":"s2","agent_id":"local-agent-clean","request_id":"req-2","text":"hi"}`
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, engine_type, status, snapshot, deadline) VALUES
		(1, 'sec-r1', 's1', 'u1', 'req-1', 'm1', 'h1', 'trpc', 'running',  ?, datetime('now','+1 hour')),
		(1, 'sec-r2', 's1', 'u1', 'req-2', 'm2', 'h2', 'trpc', 'succeeded', ?, datetime('now','+1 hour')),
		(1, 'sec-r3', 's2', 'u1', 'req-3', 'm3', 'h3', 'trpc', 'running',  ?, datetime('now','+1 hour')),
		(2, 'sec-r4', 's3', 'u2', 'req-4', 'm4', 'h4', 'trpc', 'running',  ?, datetime('now','+1 hour')),
		(1, 'sec-r5', 's2', 'u1', 'req-5', 'm5', 'h5', 'trpc', 'running', '{"version":1}', datetime('now','+1 hour'))`,
		blocked, blocked, clean, blocked).Error)
	require.NoError(t, db.Exec(`UPDATE agent_runs SET lease_owner = 'worker-1', lease_until = datetime('now','+1 minute'), revision = 3 WHERE tenant_id = 1 AND run_id = 'sec-r1'`).Error)
}

func TestCancelRunsByAgentsCancelsOnlyMatchingActiveRuns(t *testing.T) {
	db := openRunTestDB(t)
	seedSecurityCancelFixture(t, db)
	runs := NewAgentRunStore(db)
	ctx := context.Background()

	reason := "agent security revocation: CVE-2026-0001"
	canceled, err := runs.CancelRunsByAgents(ctx, 1, []string{"local-agent-blocked"}, reason)
	require.NoError(t, err)
	require.EqualValues(t, 1, canceled)

	blocked, err := runs.Get(ctx, agentruntime.RunKey{TenantID: 1, RunID: "sec-r1"})
	require.NoError(t, err)
	require.Equal(t, "canceled", blocked.Status)
	require.Equal(t, "agent_security_revocation", blocked.WaitReason)
	require.Empty(t, blocked.Owner)
	require.True(t, blocked.LeaseUntil.IsZero())
	require.EqualValues(t, 4, blocked.Revision)

	var events int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_run_events WHERE tenant_id = 1 AND run_id = 'sec-r1' AND event_type = 'cancellation_requested'`).Scan(&events).Error)
	require.EqualValues(t, 1, events, "在途处置必须留下 cancellation_requested 时间线事实（#37 语义）")

	var slot *string
	require.NoError(t, db.Raw(`SELECT active_agent_run_id FROM sessions WHERE tenant_id = 1 AND id = 's1'`).Scan(&slot).Error)
	require.Nil(t, slot, "会话活动 Run 槽必须释放")

	terminal, err := runs.Get(ctx, agentruntime.RunKey{TenantID: 1, RunID: "sec-r2"})
	require.NoError(t, err)
	require.Equal(t, "succeeded", terminal.Status, "终态 Run 不被在途处置触碰")
	other, err := runs.Get(ctx, agentruntime.RunKey{TenantID: 1, RunID: "sec-r3"})
	require.NoError(t, err)
	require.Equal(t, "running", other.Status, "agent 不匹配的 Run 不受影响")
	foreign, err := runs.Get(ctx, agentruntime.RunKey{TenantID: 2, RunID: "sec-r4"})
	require.NoError(t, err)
	require.Equal(t, "running", foreign.Status, "跨租户 Run 不受影响")
	malformed, err := runs.Get(ctx, agentruntime.RunKey{TenantID: 1, RunID: "sec-r5"})
	require.NoError(t, err)
	require.Equal(t, "running", malformed.Status, "无法解析的非 coordinator snapshot 按计划 fail-quiet 跳过")

	noAgents, err := runs.CancelRunsByAgents(ctx, 1, nil, "empty")
	require.NoError(t, err)
	require.Zero(t, noAgents)
	noTenant, err := runs.CancelRunsByAgents(ctx, 0, []string{"local-agent-blocked"}, "empty")
	require.NoError(t, err)
	require.Zero(t, noTenant)
	again, err := runs.CancelRunsByAgents(ctx, 1, []string{"local-agent-blocked"}, "again")
	require.NoError(t, err)
	require.EqualValues(t, 0, again, "处置幂等：已终态的行不再计入")
}

func TestCancelRunsByAgentsForRevocationRollsBackRunAndCountTogether(t *testing.T) {
	db := openRunTestDB(t)
	seedSecurityCancelFixture(t, db)
	revocation := &types.AgentReleaseRevocationEntity{TenantID: 1, ListingID: "listing", ReleaseID: "release", Reason: "reason", RevokedBy: "admin"}
	require.NoError(t, db.Create(revocation).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER fail_revocation_count BEFORE UPDATE OF canceled_run_count ON agent_release_revocations BEGIN SELECT RAISE(ABORT, 'count unavailable'); END`).Error)

	count, err := NewAgentRunStore(db).CancelRunsByAgentsForRevocation(context.Background(), 1,
		[]string{"local-agent-blocked"}, "security revocation", AgentSecurityRevocationRef{Kind: AgentSecurityRevocationRelease, ID: revocation.ID})
	require.Error(t, err)
	require.Zero(t, count)
	var status string
	require.NoError(t, db.Raw(`SELECT status FROM agent_runs WHERE tenant_id = 1 AND run_id = 'sec-r1'`).Scan(&status).Error)
	require.Equal(t, "running", status, "count update failure rolls back the run transition")
	var events int64
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id = ? AND run_id = ? AND event_type = ?", 1, "sec-r1", "cancellation_requested").Count(&events).Error)
	require.Zero(t, events)
	var stored types.AgentReleaseRevocationEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", 1, revocation.ID).Take(&stored).Error)
	require.Zero(t, stored.CanceledRunCount)
	var activeRunID string
	require.NoError(t, db.Raw(`SELECT active_agent_run_id FROM sessions WHERE tenant_id = 1 AND id = 's1'`).Scan(&activeRunID).Error)
	require.Equal(t, "sec-r1", activeRunID)
}

func TestCancelRunsBySecurityPinsMatchesOnlyExactVariantIdentity(t *testing.T) {
	db := openRunTestDB(t)
	seedSecurityCancelFixture(t, db)
	for _, row := range []struct{ id, agent, version, release string }{
		{"pin-hit", "agent-a", "version-a", "release-a"}, {"pin-version-other", "agent-a", "version-b", "release-a"}, {"pin-release-other", "agent-a", "version-a", "release-b"},
	} {
		require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id,run_id,session_id,owner_id,request_id,assistant_message_id,request_hash,engine_type,status,snapshot,deadline,security_agent_id,security_local_agent_version_id,security_release_id,security_pin_source) VALUES (1,?,'s2','u1',?,?,'hash','trpc','running',?,datetime('now','+1 hour'),?,?,?,'admission')`, row.id, row.id, "msg-"+row.id, `{"agent_id":"`+row.agent+`"}`, row.agent, row.version, row.release).Error)
	}
	revocation := &types.AgentReleaseRevocationEntity{ID: "rev-pin-test", TenantID: 1, ReleaseID: "release-a", Reason: "reason", InFlightDisposition: "cancel", RunCancellationState: "pending"}
	require.NoError(t, db.Create(revocation).Error)
	count, err := NewAgentRunStore(db).CancelRunsBySecurityPinsForRevocation(context.Background(), 1, []AgentSecurityRunPin{{AgentID: "agent-a", LocalAgentVersionID: "version-a", ReleaseID: "release-a"}}, "security", AgentSecurityRevocationRef{Kind: AgentSecurityRevocationRelease, ID: revocation.ID})
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	for _, row := range []struct{ id, want string }{{"pin-hit", "canceled"}, {"pin-version-other", "running"}, {"pin-release-other", "running"}} {
		var status string
		require.NoError(t, db.Raw(`SELECT status FROM agent_runs WHERE tenant_id=1 AND run_id=?`, row.id).Scan(&status).Error)
		require.Equal(t, row.want, status)
	}
	var stored types.AgentReleaseRevocationEntity
	require.NoError(t, db.Where("tenant_id=? AND id=?", 1, revocation.ID).Take(&stored).Error)
	require.EqualValues(t, 1, stored.CanceledRunCount)
	require.Equal(t, "complete", stored.RunCancellationState)
}

func TestRunCancellationReconciliationRetriesPendingExactPinsOnce(t *testing.T) {
	db := openRunTestDB(t)
	seedSecurityCancelFixture(t, db)
	require.NoError(t, db.Exec(`PRAGMA foreign_keys=OFF`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_adoption_variants(id,tenant_id,adoption_id,release_id,name,state,local_agent_id,local_agent_version_id,published_at,created_at,updated_at) VALUES ('variant-pin',1,'adoption-unused','release-reconcile','Pinned','retired','agent-reconcile','version-reconcile',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, db.Exec(`PRAGMA foreign_keys=ON`).Error)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id,run_id,session_id,owner_id,request_id,assistant_message_id,request_hash,engine_type,status,snapshot,deadline,security_agent_id,security_local_agent_version_id,security_release_id,security_pin_source) VALUES (1,'run-reconcile','s1','u1','req-reconcile','msg-reconcile','h','trpc','running','{"agent_id":"agent-reconcile"}',datetime('now','+1 hour'),'agent-reconcile','version-reconcile','release-reconcile','legacy_backfill')`).Error)
	revocation := types.AgentReleaseRevocationEntity{ID: "rev-reconcile", TenantID: 1, ReleaseID: "release-reconcile", Reason: "revoke", InFlightDisposition: "cancel", RunCancellationState: "pending"}
	require.NoError(t, db.Create(&revocation).Error)
	runs := NewAgentRunStore(db)
	pending, err := runs.ListPendingRunCancellations(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	type reconciliationResult struct {
		count int64
		err   error
	}
	results := make(chan reconciliationResult, 2)
	var replicas sync.WaitGroup
	for i := 0; i < 2; i++ {
		replicas.Add(1)
		go func() {
			defer replicas.Done()
			n, e := runs.ReconcileRunCancellation(context.Background(), 1, revocation.ID)
			results <- reconciliationResult{count: n, err: e}
		}()
	}
	replicas.Wait()
	close(results)
	for result := range results {
		require.NoError(t, result.err)
		require.EqualValues(t, 1, result.count, "replicas observe one cumulative completed obligation")
	}
	count, err := runs.ReconcileRunCancellation(context.Background(), 1, revocation.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	count, err = runs.ReconcileRunCancellation(context.Background(), 1, revocation.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, count, "a completed retry returns the cumulative count without canceling twice")
	var stored types.AgentReleaseRevocationEntity
	require.NoError(t, db.Where("tenant_id=? AND id=?", 1, revocation.ID).Take(&stored).Error)
	require.EqualValues(t, 1, stored.CanceledRunCount)
	require.Equal(t, "complete", stored.RunCancellationState)
}

func TestCancelRunsByAgentsKeepsLongReasonInEventAndBoundsWaitReason(t *testing.T) {
	db := openRunTestDB(t)
	seedSecurityCancelFixture(t, db)
	reason := strings.Repeat("revocation incident detail ", 8)

	canceled, err := NewAgentRunStore(db).CancelRunsByAgents(context.Background(), 1,
		[]string{"local-agent-blocked"}, reason)
	require.NoError(t, err)
	require.EqualValues(t, 1, canceled)
	run, err := NewAgentRunStore(db).Get(context.Background(), agentruntime.RunKey{TenantID: 1, RunID: "sec-r1"})
	require.NoError(t, err)
	require.Equal(t, "agent_security_revocation", run.WaitReason)
	require.LessOrEqual(t, len(run.WaitReason), 64, "wait_reason fits PostgreSQL VARCHAR(64)")

	var payload string
	require.NoError(t, db.Table("agent_run_events").Select("payload").
		Where("tenant_id = ? AND run_id = ? AND event_type = ?", 1, "sec-r1", "cancellation_requested").Scan(&payload).Error)
	var event map[string]string
	require.NoError(t, json.Unmarshal([]byte(payload), &event))
	require.Equal(t, reason, event["reason"], "事件保留完整撤回原因")
}

func TestCancelRunsByAgentsReservesSQLiteWriterBeforeCandidateScan(t *testing.T) {
	db := openRunTestDB(t)
	seedSecurityCancelFixture(t, db)
	ctx := context.Background()

	// Pin a second connection for a worker transaction and keep its initial
	// read open. The cancellation callback below gives the worker exactly the
	// interleaving that previously caused a deferred read-to-write upgrade.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	workerConn, err := sqlDB.Conn(ctx)
	require.NoError(t, err)
	workerDB, err := gorm.Open(sqlite.New(sqlite.Config{Conn: workerConn}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	workerTx := workerDB.WithContext(ctx).Begin()
	require.NoError(t, workerTx.Error)
	defer func() {
		_ = workerTx.Rollback().Error
		_ = workerConn.Close()
	}()
	var observedRevision int64
	require.NoError(t, workerTx.Table("agent_runs").Select("revision").
		Where("tenant_id = ? AND run_id = ?", 1, "sec-r1").Scan(&observedRevision).Error)

	candidateScanned := make(chan struct{})
	continueCancellation := make(chan struct{})
	var releaseOnce sync.Once
	releaseScan := func() { releaseOnce.Do(func() { close(continueCancellation) }) }
	defer releaseScan()
	var scanObserved bool
	callbackName := "test:wait_for_security_cancel_candidate_scan"
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if scanObserved || tx.Statement == nil {
			return
		}
		if _, ok := tx.Statement.Dest.(*[]agentRunRow); !ok {
			return
		}
		scanObserved = true
		close(candidateScanned)
		<-continueCancellation
	}))
	defer db.Callback().Query().Remove(callbackName)

	type cancelResult struct {
		count int64
		err   error
	}
	canceled := make(chan cancelResult, 1)
	go func() {
		count, cancelErr := NewAgentRunStore(db).CancelRunsByAgents(ctx, 1,
			[]string{"local-agent-blocked"}, strings.Repeat("security reason ", 10))
		canceled <- cancelResult{count: count, err: cancelErr}
	}()

	select {
	case <-candidateScanned:
	case <-time.After(15 * time.Second):
		releaseScan()
		t.Fatal("cancellation did not reach candidate-scan barrier")
	}

	workerWrite := make(chan error, 1)
	go func() {
		workerWrite <- workerTx.Table("agent_runs").
			Where("tenant_id = ? AND run_id = ? AND revision = ?", 1, "sec-r1", observedRevision).
			UpdateColumn("revision", gorm.Expr("revision + 1")).Error
	}()

	var workerErr error
	select {
	case workerErr = <-workerWrite:
	case <-time.After(15 * time.Second):
		releaseScan()
		t.Fatal("worker update did not resolve at the SQLite busy-timeout boundary")
	}
	if workerErr != nil {
		// Release the worker's read transaction before cancellation commits.
		require.NoError(t, workerTx.Rollback().Error)
	}
	releaseScan()

	var result cancelResult
	select {
	case result = <-canceled:
	case <-time.After(15 * time.Second):
		t.Fatal("security cancellation did not finish after releasing the barrier")
	}
	if workerErr == nil {
		_ = workerTx.Rollback().Error
	}
	require.Error(t, workerErr, "SQLite writer reservation prevents the stale worker write")
	require.NoError(t, result.err)
	require.EqualValues(t, 1, result.count)

	run, err := NewAgentRunStore(db).Get(ctx, agentruntime.RunKey{TenantID: 1, RunID: "sec-r1"})
	require.NoError(t, err)
	require.Equal(t, "canceled", run.Status)
	require.EqualValues(t, observedRevision+1, run.Revision, "仅撤回转换递增 revision，worker 写入未提交")
	var events int64
	require.NoError(t, db.Table("agent_run_events").Where("tenant_id = ? AND run_id = ? AND event_type = ?", 1, "sec-r1", "cancellation_requested").Count(&events).Error)
	require.EqualValues(t, 1, events, "Run 与单个取消事件作为一个事务提交")
}
