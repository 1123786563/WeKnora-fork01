package codedelivery

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// created_at 并列（sqlite DATETIME 秒级精度下极易发生）时 LatestForRun 必须
// 给出确定且跨调用稳定的次序（最终修复轮发现 4）：created_at DESC 之上以
// id DESC 决出全序。此前仅按 created_at DESC 排序，并列行次序未定，同一
// run 并发多次 prepare 的极端场景读面可能取错行。
func TestLatestForRunDeterministicOnCreatedAtTie(t *testing.T) {
	dsn := "file:" + t.TempDir() + "/tie.db?_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&DeliveryRow{}))

	// 强制两行同一时刻：无论存储精度如何都构成真正的并列。
	at := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	first := DeliveryRow{ID: "dlv_a", TenantID: 7, TaskID: "s-1", RunID: "run-1", OwnerID: "u1", State: StatePrepared, CreatedAt: at, UpdatedAt: at}
	second := DeliveryRow{ID: "dlv_b", TenantID: 7, TaskID: "s-1", RunID: "run-1", OwnerID: "u1", State: StatePrepared, CreatedAt: at, UpdatedAt: at}
	require.NoError(t, db.Create(&first).Error)
	require.NoError(t, db.Create(&second).Error)

	s := NewDeliveryStore(db)
	for i := 0; i < 5; i++ {
		row, err := s.LatestForRun(context.Background(), 7, "run-1")
		require.NoError(t, err)
		require.Equal(t, "dlv_b", row.ID, "并列 created_at 必须由 id DESC 稳定决出")
	}
}
