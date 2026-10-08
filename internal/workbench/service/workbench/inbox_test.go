package workbench

import (
	"context"
	"fmt"
	"testing"
	"time"

	contract "github.com/Tencent/WeKnora/internal/workbench"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// 收件箱读的物理列集与 overview 共用（agent_runs/sessions 投影行），防自造 schema 掩盖缺列。
type inboxRunRow struct {
	TenantID  uint64
	RunID     string
	SessionID string
	OwnerID   string
	Status    string
	UpdatedAt time.Time
}

func (inboxRunRow) TableName() string { return "agent_runs" }

type inboxSessionRow struct {
	TenantID   uint64
	ID         string
	Title      string
	ArchivedAt *time.Time
}

func (inboxSessionRow) TableName() string { return "sessions" }

func TestGormInteractionStoreListPendingScopesOwnerAndExcludesUndecidableRows(t *testing.T) {
	dsn := "file:" + t.TempDir() + "/inbox-pending.db?_foreign_keys=on&_busy_timeout=10000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&interactionRow{}, &inboxRunRow{}, &inboxSessionRow{}))
	now := time.Now().UTC()
	past := now.Add(-time.Hour)
	require.NoError(t, db.Create([]*inboxSessionRow{
		{TenantID: 7, ID: "s-live", Title: "live task"},
		{TenantID: 7, ID: "s-archived", Title: "archived task", ArchivedAt: &now},
	}).Error)
	require.NoError(t, db.Create([]*inboxRunRow{
		{TenantID: 7, RunID: "run-live", SessionID: "s-live", OwnerID: "u1", Status: "waiting_user", UpdatedAt: now},
		{TenantID: 7, RunID: "run-archived", SessionID: "s-archived", OwnerID: "u1", Status: "running", UpdatedAt: now},
		{TenantID: 7, RunID: "run-dangling", SessionID: "s-missing", OwnerID: "u1", Status: "running", UpdatedAt: now},
	}).Error)
	rows := []*interactionRow{
		// 应列出：本人、待处理、活任务、未过期（dangling run 的 LEFT JOIN 使 archived_at 为 NULL，与 overview 同语义，保持可见）
		{TenantID: 7, ID: "i-live", RunID: "run-live", OwnerID: "u1", Kind: "tool_approval", ArgsHash: "h1", Status: "pending", CreatedAt: now, UpdatedAt: now},
		{TenantID: 7, ID: "i-dangling", RunID: "run-dangling", OwnerID: "u1", Kind: "recovery", ArgsHash: "h2", Status: "pending", CreatedAt: now.Add(time.Minute), UpdatedAt: now},
		// 应排除：已归档任务
		{TenantID: 7, ID: "i-archived", RunID: "run-archived", OwnerID: "u1", Kind: "budget", ArgsHash: "h3", Status: "pending", CreatedAt: now, UpdatedAt: now},
		// 应排除：已过期（不可决定）
		{TenantID: 7, ID: "i-expired", RunID: "run-live", OwnerID: "u1", Kind: "tool_approval", ArgsHash: "h4", Status: "pending", ExpiresAt: &past, CreatedAt: now, UpdatedAt: now},
		// 应排除：已解决
		{TenantID: 7, ID: "i-resolved", RunID: "run-live", OwnerID: "u1", Kind: "tool_approval", ArgsHash: "h5", Status: "resolved", CreatedAt: now, UpdatedAt: now},
		// 应排除：他人（owner 隔离）
		{TenantID: 7, ID: "i-other-owner", RunID: "run-live", OwnerID: "u2", Kind: "tool_approval", ArgsHash: "h6", Status: "pending", CreatedAt: now, UpdatedAt: now},
	}
	for _, row := range rows {
		require.NoError(t, db.Create(row).Error)
	}
	store := NewGormInteractionStore(db)
	items, err := store.ListPending(context.Background(), 7, "u1", 50)
	require.NoError(t, err)
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	require.Equal(t, []string{"i-live", "i-dangling"}, ids, "created_at ASC 排序；归档任务/过期/已解决/他人行一律不进收件箱")
	require.Equal(t, "run-live", items[0].RunID)
	require.Equal(t, "h1", items[0].ArgsHash)
	require.NotEmpty(t, items[0].CreatedAt, "收件箱行携带 created_at 供展示与排序核对")

	// limit 钳制：非正数与超上限回退默认 50（不放大租户读）。
	_, err = store.ListPending(context.Background(), 7, "u1", 0)
	require.NoError(t, err)
	_, err = store.ListPending(context.Background(), 7, "u1", 9999)
	require.NoError(t, err)
}

func TestInteractionServiceListInboxScopesIdentityAndFailsClosed(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:w08_inbox_service?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	// ListPending JOIN agent_runs/sessions（F1）：迁移必须一并建表，否则 SQLite 报 no such table。
	require.NoError(t, db.AutoMigrate(&interactionRow{}, &inboxRunRow{}, &inboxSessionRow{}))
	require.NoError(t, db.Create(&interactionRow{TenantID: 7, ID: "i1", RunID: "r1", OwnerID: "web_user:u1", Kind: "tool_approval", ArgsHash: "h", Status: "pending"}).Error)
	svc := NewInteractionService(NewGormInteractionStore(db), nil, nil)
	items, err := svc.ListInbox(interactionContext(), 50)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "i1", items[0].ID)
	_, err = svc.ListInbox(context.Background(), 50)
	require.Error(t, err, "身份上下文缺失必须拒绝，不回退到全局读")

	// store 未实现 ListPending（如内存桩）→ fail closed，不静默返回空。
	stub := &interactionStoreStub{current: contract.InteractionDecision{ID: "i1", Kind: "budget", ArgsHash: "a"}}
	_, err = NewInteractionService(stub, nil, nil).ListInbox(interactionContext(), 50)
	require.ErrorIs(t, err, ErrCapabilityUnavailable)
}

// 过期 pending 行没有清理路径且必为最旧（created_at ASC 窗口头）。若过期
// 谓词只在 Go 侧 skip，SQL LIMIT 先截断窗口，过期行累积满窗口后活待办被
// 静默饿死——过期过滤必须下推到 SQL 谓词（参数绑定）。
func TestGormInteractionStoreListPendingDoesNotStarveWhenExpiredRowsFillWindow(t *testing.T) {
	dsn := "file:" + t.TempDir() + "/inbox-starve.db?_foreign_keys=on&_busy_timeout=10000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&interactionRow{}, &inboxRunRow{}, &inboxSessionRow{}))
	now := time.Now().UTC()
	past := now.Add(-time.Hour)
	for i := 0; i < 3; i++ {
		require.NoError(t, db.Create(&interactionRow{
			TenantID: 7, ID: fmt.Sprintf("i-expired-%d", i), RunID: "run-live", OwnerID: "u1",
			Kind: "tool_approval", ArgsHash: "h-old", Status: "pending", ExpiresAt: &past,
			CreatedAt: now.Add(time.Duration(i) * time.Minute), UpdatedAt: now,
		}).Error)
	}
	require.NoError(t, db.Create(&interactionRow{
		TenantID: 7, ID: "i-alive", RunID: "run-live", OwnerID: "u1",
		Kind: "tool_approval", ArgsHash: "h-alive", Status: "pending",
		CreatedAt: now.Add(time.Hour), UpdatedAt: now,
	}).Error)
	store := NewGormInteractionStore(db)
	items, err := store.ListPending(context.Background(), 7, "u1", 3)
	require.NoError(t, err)
	require.Len(t, items, 1, "窗口被过期行填满时活待办仍必须可见：过期过滤先于 LIMIT 生效")
	require.Equal(t, "i-alive", items[0].ID)
}
