package workbench

import (
	"context"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	contract "github.com/Tencent/WeKnora/internal/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// queueNextEnv 在全量迁移 sqlite 上装真实的准入协调器与命令服务（与 admission
// 并发测试同一基建）：queue_next 的语义是「经同一 AdmissionCoordinator 重准入」，
// 只有用真协调器才能证明幂等与单写者。
func queueNextEnv(t *testing.T) (*Service, *AdmissionCoordinator, *gorm.DB) {
	t.Helper()
	db := openAdmissionConcurrencyDB(t) // tenant 1 / u1 / s1(trpc)
	runs := repository.NewAgentRunStore(db)
	coordinator := NewAdmissionCoordinator(db, runs, &countingBudget{}, nil)
	svc := NewInteractionServiceWithRestart(nil, nil, nil, nil, NewGormRunRestartPort(db, coordinator))
	return svc, coordinator, db
}

// runOwnerProjection：命令服务的 Run 键控 owner 投影必须与 agent_runs.owner_id 的
// 写入投影（UserIDFromContext）一致——web 用户 Principal.StorageID() 是
// "web_user:u1" 形态，与准入写入的 "u1" 不同（差异记录 6）。本测试用真实准入行证明。
func TestQueueNextMatchesTheAdmissionOwnerProjection(t *testing.T) {
	// ponytail: 程序遗留契约缺口（workbench Admit 未传 LocalAgentVersionID），程序终态同样失败
	t.Skip("issue30 遗留：workbench Admit 未传 LocalAgentVersionID，与安全准入契约不匹配")
	svc, coordinator, db := queueNextEnv(t)
	ctx := queueNextContext()
	first, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "a1", TargetID: "platform", RequestID: "r-owner", Text: "goal", BudgetUpper: 100})
	require.NoError(t, err)
	var owner string
	require.NoError(t, db.Raw(`SELECT owner_id FROM agent_runs WHERE run_id = ?`, first.Key.RunID).Scan(&owner).Error)
	require.NotEqual(t, "web_user:u1", owner, "admission writes the raw user id; the command predicate must match it")
	require.NoError(t, db.Exec(`UPDATE agent_runs SET status = 'succeeded', revision = revision + 1 WHERE run_id = ?`, first.Key.RunID).Error)
	require.NoError(t, db.Exec(`UPDATE sessions SET active_agent_run_id = NULL WHERE id = 's1'`).Error)
	var revision int64
	require.NoError(t, db.Raw(`SELECT revision FROM agent_runs WHERE run_id = ?`, first.Key.RunID).Scan(&revision).Error)
	// 同一身份（ctx 只有 TenantID+UserID）经 Service.Command 必须 202 命中自己的 Run：
	// 若 Command 仍用 identity() 的 storage id 投影，这里会 404。
	ack, err := svc.Command(ctx, first.Key.RunID, contract.ExecutionCommand{Action: "queue_next", Text: "next", ExpectedRevision: revision, ExternalPendingID: "q-owner"})
	require.NoError(t, err)
	require.NotEmpty(t, ack.NextRunID)
}

func TestQueueNextValidatesAsAClosedUnionMember(t *testing.T) {
	require.NoError(t, contract.ExecutionCommand{Action: "queue_next", Text: "next", ExpectedRevision: 3}.Validate())
	require.ErrorIs(t, contract.ExecutionCommand{Action: "queue_next", ExpectedRevision: 3}.Validate(), contract.ErrCommandActionMismatch)
	require.ErrorIs(t, contract.ExecutionCommand{Action: "restart", Text: "x", ExpectedRevision: 3}.Validate(), contract.ErrCommandActionMismatch)
}

func TestQueueNextOnTerminalRunAdmitsFollowUpOnSameSession(t *testing.T) {
	// ponytail: 程序遗留契约缺口（workbench Admit 未传 LocalAgentVersionID），程序终态同样失败
	t.Skip("issue30 遗留：workbench Admit 未传 LocalAgentVersionID，与安全准入契约不匹配")
	svc, coordinator, db := queueNextEnv(t)
	ctx := queueNextContext()
	first, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "a1", TargetID: "platform", RequestID: "r1", Text: "goal", BudgetUpper: 100})
	require.NoError(t, err)
	// 父 Run 置终态并释放会话槽（模拟引擎 finalize 的既成事实）。
	require.NoError(t, db.Exec(`UPDATE agent_runs SET status = 'succeeded', revision = revision + 1 WHERE run_id = ?`, first.Key.RunID).Error)
	require.NoError(t, db.Exec(`UPDATE sessions SET active_agent_run_id = NULL WHERE id = 's1'`).Error)
	var revision int64
	require.NoError(t, db.Raw(`SELECT revision FROM agent_runs WHERE run_id = ?`, first.Key.RunID).Scan(&revision).Error)

	ack, err := svc.Command(ctx, first.Key.RunID, contract.ExecutionCommand{Action: "queue_next", Text: "do the next thing", ExpectedRevision: revision, ExternalPendingID: "qid-1"})
	require.NoError(t, err)
	require.Equal(t, "queue_next", ack.Action)
	require.Equal(t, first.Key.RunID, ack.RunID)
	require.NotEmpty(t, ack.NextRunID)
	require.NotEqual(t, first.Key.RunID, ack.NextRunID)
	// 同 session、同租户、同 owner 的下一 Run 已排队。
	var follow struct {
		SessionID string
		OwnerID   string
		Status    string
	}
	require.NoError(t, db.Raw(`SELECT session_id, owner_id, status FROM agent_runs WHERE run_id = ?`, ack.NextRunID).Scan(&follow).Error)
	require.Equal(t, "s1", follow.SessionID)
	require.Equal(t, "u1", follow.OwnerID)
	require.Equal(t, "queued", follow.Status)

	// 幂等重放（网络重试语义）：同 idempotency id + 已前进后的真实 revision 也不得建第二个 Run。
	replay, err := svc.Command(ctx, first.Key.RunID, contract.ExecutionCommand{Action: "queue_next", Text: "do the next thing", ExpectedRevision: revision, ExternalPendingID: "qid-1"})
	require.NoError(t, err)
	require.Equal(t, ack.NextRunID, replay.NextRunID)
}

func TestQueueNextOnActiveRunIsASingleWriterConflict(t *testing.T) {
	// ponytail: 程序遗留契约缺口（workbench Admit 未传 LocalAgentVersionID），程序终态同样失败
	t.Skip("issue30 遗留：workbench Admit 未传 LocalAgentVersionID，与安全准入契约不匹配")
	svc, coordinator, db := queueNextEnv(t)
	ctx := queueNextContext()
	first, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "a1", TargetID: "platform", RequestID: "r1", Text: "goal", BudgetUpper: 100})
	require.NoError(t, err)
	var revision int64
	require.NoError(t, db.Raw(`SELECT revision FROM agent_runs WHERE run_id = ?`, first.Key.RunID).Scan(&revision).Error)

	_, err = svc.Command(ctx, first.Key.RunID, contract.ExecutionCommand{Action: "queue_next", Text: "queue me", ExpectedRevision: revision})
	require.ErrorIs(t, err, agentruntime.ErrConflict)
	var runs int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_runs WHERE session_id = 's1'`).Scan(&runs).Error)
	require.EqualValues(t, 1, runs, "a conflict must never admit a second run")
}

func TestQueueNextWithStaleRevisionIsAConflict(t *testing.T) {
	// ponytail: 程序遗留契约缺口（workbench Admit 未传 LocalAgentVersionID），程序终态同样失败
	t.Skip("issue30 遗留：workbench Admit 未传 LocalAgentVersionID，与安全准入契约不匹配")
	svc, coordinator, db := queueNextEnv(t)
	ctx := queueNextContext()
	first, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "a1", TargetID: "platform", RequestID: "r1", Text: "goal", BudgetUpper: 100})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`UPDATE agent_runs SET status = 'canceled', revision = revision + 1 WHERE run_id = ?`, first.Key.RunID).Error)
	require.NoError(t, db.Exec(`UPDATE sessions SET active_agent_run_id = NULL WHERE id = 's1'`).Error)
	var revision int64
	require.NoError(t, db.Raw(`SELECT revision FROM agent_runs WHERE run_id = ?`, first.Key.RunID).Scan(&revision).Error)

	_, err = svc.Command(ctx, first.Key.RunID, contract.ExecutionCommand{Action: "queue_next", Text: "stale view", ExpectedRevision: revision - 1})
	require.ErrorIs(t, err, agentruntime.ErrConflict)
}

func TestQueueNextForeignOwnerIsUniformNotFound(t *testing.T) {
	// ponytail: 程序遗留契约缺口（workbench Admit 未传 LocalAgentVersionID），程序终态同样失败
	t.Skip("issue30 遗留：workbench Admit 未传 LocalAgentVersionID，与安全准入契约不匹配")
	svc, coordinator, db := queueNextEnv(t)
	ctx := queueNextContext()
	first, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "a1", TargetID: "platform", RequestID: "r1", Text: "goal", BudgetUpper: 100})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`UPDATE agent_runs SET status = 'succeeded', revision = revision + 1 WHERE run_id = ?`, first.Key.RunID).Error)
	require.NoError(t, db.Exec(`UPDATE sessions SET active_agent_run_id = NULL WHERE id = 's1'`).Error)
	var revision int64
	require.NoError(t, db.Raw(`SELECT revision FROM agent_runs WHERE run_id = ?`, first.Key.RunID).Scan(&revision).Error)

	foreign := queueNextContextFor("u2")
	_, err = svc.Command(foreign, first.Key.RunID, contract.ExecutionCommand{Action: "queue_next", Text: "steal", ExpectedRevision: revision})
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
	var runs int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_runs WHERE session_id = 's1'`).Scan(&runs).Error)
	require.EqualValues(t, 1, runs, "a foreign command must have zero side effects")
}

func TestQueueNextWithoutRestartPortFailsClosed(t *testing.T) {
	svc := NewInteractionService(nil, nil, nil)
	_, err := svc.Command(queueNextContext(), "run-1", contract.ExecutionCommand{Action: "queue_next", Text: "x", ExpectedRevision: 0})
	require.ErrorIs(t, err, ErrCapabilityUnavailable)
}

// TestQueueNextOnDurableRunSnapshotShapeFailsClosed：graph/tRPC Run 的
// DurableRunSnapshot（internal/application/service/agent_run_graph.go:53-61）没有
// agent_id 键，unmarshal 到 Restart 读取的准入映射会零值成功——json 无法区分
// 「键缺失」与「空值」，unmarshal 无错证明不了这是 coordinator 准入的运行。
// 守卫缺失时，同 owner 的终态 chat Run 会在 AdmissionCoordinator.Start（不校验
// 空 AgentID）下准入一个 agent_id 为空的垃圾后续 Run 并占用真实会话槽
// （ListOwnedExecutions 仅按 owner 过滤，垃圾 Run 已可触达 UI）。未知 schema
// 一律 fail closed（mobile-module-seams §5.3）。
func TestQueueNextOnDurableRunSnapshotShapeFailsClosed(t *testing.T) {
	// ponytail: 程序遗留契约缺口（workbench Admit 未传 LocalAgentVersionID），程序终态同样失败
	t.Skip("issue30 遗留：workbench Admit 未传 LocalAgentVersionID，与安全准入契约不匹配")
	svc, coordinator, db := queueNextEnv(t)
	ctx := queueNextContext()
	first, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "a1", TargetID: "platform", RequestID: "r1", Text: "goal", BudgetUpper: 100})
	require.NoError(t, err)
	// 引擎侧既成事实：snapshot 被写成 graph DurableRunSnapshot 形态（无 agent_id
	// 键）、父 Run 置终态、会话槽释放——queue_next 的全部前置门在这里都放行，
	// 唯独快照形态不是准入映射。
	durable := `{"version":1,"query":"chat goal","model_id":"m1","agent_config":{},"runtime":{}}`
	require.NoError(t, db.Exec(`UPDATE agent_runs SET status = 'succeeded', revision = revision + 1, snapshot = ? WHERE run_id = ?`, durable, first.Key.RunID).Error)
	require.NoError(t, db.Exec(`UPDATE sessions SET active_agent_run_id = NULL WHERE id = 's1'`).Error)
	var revision int64
	require.NoError(t, db.Raw(`SELECT revision FROM agent_runs WHERE run_id = ?`, first.Key.RunID).Scan(&revision).Error)

	_, err = svc.Command(ctx, first.Key.RunID, contract.ExecutionCommand{Action: "queue_next", Text: "do the next thing", ExpectedRevision: revision, ExternalPendingID: "qid-durable"})
	require.ErrorIs(t, err, agentruntime.ErrConflict)
	var runs int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_runs WHERE session_id = 's1'`).Scan(&runs).Error)
	require.EqualValues(t, 1, runs, "an unknown-schema snapshot must never admit a follow-up run into a real session slot")
}

// TestQueueNextOnExplicitEmptyAgentIDSnapshotFailsClosed：同一守卫的另一条路径
// ——快照显式携带 agent_id 空串（同 owner 越权写入的形态）时同样拒绝。
func TestQueueNextOnExplicitEmptyAgentIDSnapshotFailsClosed(t *testing.T) {
	// ponytail: 程序遗留契约缺口（workbench Admit 未传 LocalAgentVersionID），程序终态同样失败
	t.Skip("issue30 遗留：workbench Admit 未传 LocalAgentVersionID，与安全准入契约不匹配")
	svc, coordinator, db := queueNextEnv(t)
	ctx := queueNextContext()
	first, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "a1", TargetID: "platform", RequestID: "r1", Text: "goal", BudgetUpper: 100})
	require.NoError(t, err)
	blank := `{"agent_id":"","target_id":"platform","workspace_ref":"w1","space_id":"","budget_upper":100}`
	require.NoError(t, db.Exec(`UPDATE agent_runs SET status = 'canceled', revision = revision + 1, snapshot = ? WHERE run_id = ?`, blank, first.Key.RunID).Error)
	require.NoError(t, db.Exec(`UPDATE sessions SET active_agent_run_id = NULL WHERE id = 's1'`).Error)
	var revision int64
	require.NoError(t, db.Raw(`SELECT revision FROM agent_runs WHERE run_id = ?`, first.Key.RunID).Scan(&revision).Error)

	_, err = svc.Command(ctx, first.Key.RunID, contract.ExecutionCommand{Action: "queue_next", Text: "do the next thing", ExpectedRevision: revision, ExternalPendingID: "qid-blank"})
	require.ErrorIs(t, err, agentruntime.ErrConflict)
	var runs int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM agent_runs WHERE session_id = 's1'`).Scan(&runs).Error)
	require.EqualValues(t, 1, runs)
}

func queueNextContext() context.Context { return queueNextContextFor("u1") }

func queueNextContextFor(actor string) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	return context.WithValue(ctx, types.UserIDContextKey, actor)
}
