package codedelivery

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openDeliveryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "deliveries.db") + "?_foreign_keys=on&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&DeliveryRow{}, &ApprovalProbeRow{}))
	return db
}

// ApprovalProbeRow 只为测试注入 app_action_approvals 形状的行（生产表由
// appconnector 迁移创建；LatestApproverForAction 按表名读）。
type ApprovalProbeRow struct {
	ArgsDigest string    `gorm:"primaryKey;column:args_digest"`
	ActionID   string    `gorm:"column:action_id"`
	Actor      string    `gorm:"column:actor"`
	Expiry     time.Time `gorm:"column:expiry"`
	Remaining  int64     `gorm:"column:remaining"`
}

func (ApprovalProbeRow) TableName() string { return "app_action_approvals" }

func TestDeliveryStoreLifecycleAndCAS(t *testing.T) {
	db := openDeliveryTestDB(t)
	store := NewDeliveryStore(db)
	ctx := context.Background()

	row := DeliveryRow{
		ID: "dlv-1", TenantID: 7, TaskID: "s-1", RunID: "r-1", OwnerID: "u1",
		ActionID: "act-1", ConnectionID: "conn-1", Repo: "octocat/hello",
		BaselineSHA: "b0000000000000000000000000000000000000000",
		Branch:      "weknora/task/s-1", State: "prepared",
	}
	require.NoError(t, store.CreateDelivery(ctx, row))

	got, err := store.GetDelivery(ctx, 7, "dlv-1")
	require.NoError(t, err)
	require.Equal(t, "prepared", got.State)

	// 跨租户读 = 未找到（不泄漏存在性）。
	_, err = store.GetDelivery(ctx, 8, "dlv-1")
	require.ErrorIs(t, err, ErrDeliveryNotFound)

	// CAS：prepared→dispatched 成功；prepared→delivered（非法源态）失败。
	require.NoError(t, store.TransitionState(ctx, 7, "dlv-1", []string{"prepared"}, "dispatched", ""))
	require.ErrorIs(t, store.TransitionState(ctx, 7, "dlv-1", []string{"prepared"}, "delivered", ""),
		ErrDeliveryStateConflict)

	// 回执：仅非零字段落账。
	require.NoError(t, store.RecordReceipts(ctx, 7, "dlv-1", ReceiptUpdate{
		CommitSHA: "c1", PRNumber: 3, PRURL: "https://github.com/octocat/hello/pull/3", RemoteLogin: "octocat",
	}))
	got, err = store.LatestForRun(ctx, 7, "r-1")
	require.NoError(t, err)
	require.Equal(t, "c1", got.CommitSHA)
	require.EqualValues(t, 3, got.PRNumber)
	require.Equal(t, "octocat", got.RemoteLogin)

	// 审批者 join：按 expiry 最新一条。
	require.NoError(t, db.Create(&ApprovalProbeRow{ArgsDigest: "d1", ActionID: "act-1", Actor: "u1", Expiry: time.Now().Add(time.Hour), Remaining: 1}).Error)
	require.NoError(t, db.Create(&ApprovalProbeRow{ArgsDigest: "d2", ActionID: "act-1", Actor: "approver@7", Expiry: time.Now().Add(2 * time.Hour), Remaining: 1}).Error)
	approver, ok, err := store.LatestApproverForAction(ctx, "act-1")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "approver@7", approver)

	_, ok, err = store.LatestApproverForAction(ctx, "act-none")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestTransitionStateWithReceiptsIsAtomic(t *testing.T) {
	db := openDeliveryTestDB(t)
	store := NewDeliveryStore(db)
	ctx := context.Background()
	require.NoError(t, store.CreateDelivery(ctx, DeliveryRow{ID: "atomic", TenantID: 7, TaskID: "s", RunID: "r", OwnerID: "u", ActionID: "a", State: "dispatched"}))
	update := ReceiptUpdate{CommitSHA: "sha-confirmed"}
	require.ErrorIs(t, store.TransitionStateWithReceipts(ctx, 7, "atomic", []string{"unknown"}, "pushed", "", update), ErrDeliveryStateConflict)
	row, err := store.GetDelivery(ctx, 7, "atomic")
	require.NoError(t, err)
	require.Equal(t, "dispatched", row.State)
	require.Empty(t, row.CommitSHA, "failed CAS must not persist the associated receipt")
	require.NoError(t, store.TransitionStateWithReceipts(ctx, 7, "atomic", []string{"dispatched"}, "pushed", "", update))
	row, err = store.GetDelivery(ctx, 7, "atomic")
	require.NoError(t, err)
	require.Equal(t, "pushed", row.State)
	require.Equal(t, "sha-confirmed", row.CommitSHA)
}
