package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedLegacySessions 添加纯聊天旧会话（0 条 agent_runs 行、显式时间戳），
// u1 名下三行（lg-0/lg-1/lg-2，keyset 翻页因此确定）、u2 一行、软删除一行。
func seedLegacySessions(t *testing.T, db *gorm.DB) {
	t.Helper()
	base := time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC)
	rows := []struct {
		id    string
		title string
		owner string
		at    time.Time
	}{
		{"lg-0", "旧聊天：会议纪要", "u1", base},
		{"lg-1", "旧聊天：周报素材", "u1", base.Add(time.Hour)},
		{"lg-2", "旧聊天：报销问题", "u1", base.Add(2 * time.Hour)},
		{"lg-3", "别人的旧聊天", "u2", base.Add(3 * time.Hour)},
		{"lg-del", "已删除的旧聊天", "u1", base.Add(4 * time.Hour)},
	}
	for _, row := range rows {
		require.NoError(t, db.Exec(
			`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type, created_at, updated_at)
			 VALUES (?, 1, ?, ?, 'builtin', ?, ?)`, row.id, row.title, row.owner, row.at, row.at,
		).Error)
	}
	require.NoError(t, db.Exec("UPDATE sessions SET deleted_at = ? WHERE id = 'lg-del'", base.Add(5*time.Hour)).Error)
}

func legacyTaskIDs(page WorkbenchLegacyPage) []string {
	ids := make([]string, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.TaskID)
	}
	return ids
}

// TestWorkbenchLegacyListProjectsChatOnlySessionsWithinOwnerScope：legacy 投影
// 只覆盖「从未有过 agent_runs 行」的会话（与 run 列表二分区）；只携带可证明事实；
// 他人/异租户/软删除不可见；序列化行键集锁定——不出现任何 Run/Grant/预算/审批/
// Agent Version 字段（AC1）。
func TestWorkbenchLegacyListProjectsChatOnlySessionsWithinOwnerScope(t *testing.T) {
	db := openRunTestDB(t)
	seedWorkbenchListFixtures(t, db)
	seedLegacySessions(t, db)
	base := time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)
	// s1/s2 有 run 行：必须留在执行列表、绝不进入 legacy 投影。
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-a1", session: "s1", status: "running", agent: "agent-x", target: "platform", at: base})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u1", runID: "r-a2", session: "s2", status: "succeeded", agent: "agent-y", target: "platform", at: base.Add(time.Second)})
	// 计划勘误（相对 plan-t44 Task 1 Step 1 的补充夹具）：seedWorkbenchListFixtures
	// 种下的 s3（u2）与 t1（tenant 2）本为 0-run 会话，按 legacy 二分语义会进入
	// u2/tenant 2 视图，与下方「u2 只见 lg-3；tenant 2 无 legacy 行」断言矛盾
	// （且 plan-t44 L823 自证「s3 属于 u2 且 0 run」）。补 run 行使其留在执行侧，
	// 逐字镜像 workbench_list_test.go:83-84 的 r-b1/r-c1 先例；全部断言保持原样。
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 1, owner: "u2", runID: "r-b1", session: "s3", status: "running", agent: "agent-x", target: "platform", at: base.Add(2 * time.Second)})
	insertWorkbenchRun(t, db, workbenchRunSeed{tenant: 2, owner: "v1", runID: "r-c1", session: "t1", status: "running", agent: "agent-x", target: "platform", at: base.Add(3 * time.Second)})

	store := NewWorkbenchLegacyListStore(db)
	ctx := context.Background()

	page, err := store.ListOwnedLegacyTasks(ctx, 1, "u1", WorkbenchLegacyFilter{})
	require.NoError(t, err)
	require.Equal(t, []string{"lg-2", "lg-1", "lg-0"}, legacyTaskIDs(page), "newest first; run-backed s1/s2 and soft-deleted lg-del excluded")

	item := page.Items[0]
	require.Equal(t, "legacy", item.Kind)
	require.Equal(t, "none", item.Attention, "no Run means no provable attention source")
	require.Equal(t, "旧聊天：报销问题", item.Title)
	require.Empty(t, item.ArchivedAt)
	require.NotEmpty(t, item.UpdatedAt)

	// AC1 键集锁定：legacy 行的序列化形状不得携带任何 Run 级/安全语义字段。
	keys, err := marshalLegacyKeys(page.Items[0])
	require.NoError(t, err)
	for _, key := range []string{"task_id", "title", "attention", "updated_at", "kind"} {
		require.Contains(t, keys, key)
	}
	for _, forbidden := range []string{
		"run_id", "run_status", "execution_status", "settlement_status",
		"grant", "budget", "approval", "agent_version", "revision", "wait_reason",
	} {
		require.NotContains(t, keys, forbidden, "legacy projection must not fabricate %q", forbidden)
	}

	// 他人同租户 / 异租户：不可见（u2 只见自己的 lg-3；tenant 2 无 legacy 行）。
	page, err = store.ListOwnedLegacyTasks(ctx, 1, "u2", WorkbenchLegacyFilter{})
	require.NoError(t, err)
	require.Equal(t, []string{"lg-3"}, legacyTaskIDs(page))
	page, err = store.ListOwnedLegacyTasks(ctx, 2, "v1", WorkbenchLegacyFilter{})
	require.NoError(t, err)
	require.Empty(t, legacyTaskIDs(page))

	// 身份非法：ErrNotFound，零查询语义。
	_, err = store.ListOwnedLegacyTasks(ctx, 0, "u1", WorkbenchLegacyFilter{})
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
	_, err = store.ListOwnedLegacyTasks(ctx, 1, " ", WorkbenchLegacyFilter{})
	require.ErrorIs(t, err, agentruntime.ErrNotFound)

	// 标题子串搜索（忽略大小写、全空白归一）。
	page, err = store.ListOwnedLegacyTasks(ctx, 1, "u1", WorkbenchLegacyFilter{Query: "周报"})
	require.NoError(t, err)
	require.Equal(t, []string{"lg-1"}, legacyTaskIDs(page))
}

// TestWorkbenchLegacyListArchivedFilterCursorBindingAndCrossEndpointReplay：
// archived 过滤镜像 run 列表语义；keyset 分页稳定；伪造游标、过滤器失配游标、
// 以及 run 列表签发的游标重放到 legacy 端点一律 ErrWorkbenchCursor。
func TestWorkbenchLegacyListArchivedFilterCursorBindingAndCrossEndpointReplay(t *testing.T) {
	db := openRunTestDB(t)
	seedLegacySessions(t, db)
	archived := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	require.NoError(t, db.Exec("UPDATE sessions SET archived_at = ? WHERE id = 'lg-2'", archived).Error)

	store := NewWorkbenchLegacyListStore(db)
	ctx := context.Background()
	// openRunTestDB 自带的 s1/s2（u1、0 run）也是合法 legacy 行且时间戳为夹具时钟——
	// 以标题搜索「旧聊天」把断言域收敛到 lg-* 行，顺序因此确定。
	scope := WorkbenchLegacyFilter{Query: "旧聊天"}

	active, err := store.ListOwnedLegacyTasks(ctx, 1, "u1", scope)
	require.NoError(t, err)
	require.Equal(t, []string{"lg-1", "lg-0"}, legacyTaskIDs(active), "archived task leaves the default view")

	archivedOnly, err := store.ListOwnedLegacyTasks(ctx, 1, "u1", WorkbenchLegacyFilter{Query: "旧聊天", ArchivedOnly: true})
	require.NoError(t, err)
	require.Equal(t, []string{"lg-2"}, legacyTaskIDs(archivedOnly))
	require.NotEmpty(t, archivedOnly.Items[0].ArchivedAt)

	// keyset：limit 1 翻页不重不漏。
	first, err := store.ListOwnedLegacyTasks(ctx, 1, "u1", WorkbenchLegacyFilter{Query: "旧聊天", Limit: 1})
	require.NoError(t, err)
	require.Equal(t, []string{"lg-1"}, legacyTaskIDs(first))
	require.NotEmpty(t, first.NextCursor)
	second, err := store.ListOwnedLegacyTasks(ctx, 1, "u1", WorkbenchLegacyFilter{Query: "旧聊天", Limit: 1, Cursor: first.NextCursor})
	require.NoError(t, err)
	require.Equal(t, []string{"lg-0"}, legacyTaskIDs(second))

	// 过滤器失配：带搜索词视图签发的游标重放到无搜索词视图被拒。
	_, err = store.ListOwnedLegacyTasks(ctx, 1, "u1", WorkbenchLegacyFilter{Cursor: first.NextCursor})
	require.ErrorIs(t, err, ErrWorkbenchCursor)
	// 跨身份重放被拒。
	_, err = store.ListOwnedLegacyTasks(ctx, 1, "u2", WorkbenchLegacyFilter{Query: "旧聊天", Cursor: first.NextCursor})
	require.ErrorIs(t, err, ErrWorkbenchCursor)
	// 伪造 base64 被拒（'Zm9yZ2Vk' 解码为非法 JSON 'forged'）。
	_, err = store.ListOwnedLegacyTasks(ctx, 1, "u1", WorkbenchLegacyFilter{Query: "旧聊天", Cursor: "Zm9yZ2Vk"})
	require.ErrorIs(t, err, ErrWorkbenchCursor)
	// run 列表签发的游标重放到 legacy 端点被拒（kind 判别符缺失）。
	runCursor := encodeWorkbenchListCursor(workbenchListCursor{
		Version: 1, TenantID: 1, OwnerID: "u1",
		CreatedAt: "2026-09-21T09:00:00Z", RunID: "r-a1",
	})
	_, err = store.ListOwnedLegacyTasks(ctx, 1, "u1", WorkbenchLegacyFilter{Query: "旧聊天", Cursor: runCursor})
	require.ErrorIs(t, err, ErrWorkbenchCursor)
}

func marshalLegacyKeys(summary WorkbenchLegacyTaskSummary) ([]string, error) {
	raw, err := json.Marshal(summary)
	if err != nil {
		return nil, err
	}
	var projected map[string]any
	if err := json.Unmarshal(raw, &projected); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(projected))
	for key := range projected {
		keys = append(keys, key)
	}
	return keys, nil
}
