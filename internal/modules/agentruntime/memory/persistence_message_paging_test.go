package memory_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/memory"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Ruling 2026-09-28-TEST-PACKAGE-SPLIT（协调者裁定，b3-r-memory / R1.2）：
// 本用例原位于 persistence_consistency_test.go（package memory，in-package），
// 因宿主 compat agentruntime_memory_passb_compat.go 建立 repository→memory
// 生产依赖后，in-package 测试再 import 宿主 repository 构成 test import cycle。
// 处置=拆至本外部测试包（package memory_test），用例名与全部断言逐字不变；
// 组装由原 &Service{repo: NewMemoryRepository(db), tenantRepo: stub} +
// s.messageRepo = repository.NewMessageRepository(db) 改为经导出构造器
// NewMemoryService 等价注入（字段逐一对应，语义等价论证见节点报告 R1.2）。
// IB3 转核验项：外部测试包用例绿色 + 特征化等价性核验。

// pagingTenantRepo 是本文件本地 tenant 配置 stub，与
// persistence/stubs_test.go 的 stubTenantRepo 同形（嵌入接口 + configs 表，
// 仅实现本用例触达的 GetTenantByID），预置 tenant 1 的启用配置。
type pagingTenantRepo struct {
	interfaces.TenantRepository
	configs map[uint64]*types.MemoryConfig
}

func (s *pagingTenantRepo) GetTenantByID(_ context.Context, id uint64) (*types.Tenant, error) {
	return &types.Tenant{ID: id, MemoryConfig: s.configs[id]}, nil
}

func TestMemoryConsistencyRealMessagePaging(t *testing.T) {
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())),
		&gorm.Config{Logger: logger.Discard},
	)
	require.NoError(t, err)
	// 与 newMemoryHarness 相同的连接约束：共享缓存 SQLite 拒绝并发写，
	// 固定单连接让驱动串行化（原注释语义保留）。
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&types.MemorySubject{}, &types.MemoryItem{}, &types.MemoryTombstone{},
		&types.MemoryTopicStat{}, &types.MemoryDocAffinity{},
		&types.MemoryItemEmbedding{}, &types.MemoryExtractionSession{}))

	tr := &pagingTenantRepo{configs: map[uint64]*types.MemoryConfig{1: {
		Enabled: true, WriteMode: types.MemoryWriteAuto, EmbeddingModelID: "embed-1",
	}}}
	msgRepo := repository.NewMessageRepository(db)
	_ = memory.NewMemoryService(memory.NewMemoryRepository(db), tr, msgRepo, nil, nil, nil)

	tenantID := uint64(1)
	ctx := context.WithValue(t.Context(), types.TenantIDContextKey, tenantID)
	ctx = types.WithPrincipal(ctx, types.Principal{Type: types.PrincipalWebUser, ID: "alice"})
	// repository 的 message loader 均经 attachArtifacts 读 message_artifacts
	// （见 message_artifact.go 头注），同包其余 message 用例 DDL 均带
	// MessageArtifactRecord，此处补齐同款。
	require.NoError(t, db.AutoMigrate(&types.Message{}, &types.MessageArtifactRecord{}))
	at := time.Now().UTC().Truncate(time.Second)
	for i := 0; i < 85; i++ {
		require.NoError(t, db.Exec("INSERT INTO messages (id, session_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)", fmt.Sprintf("m%03d", i), "s", "user", "hello", at).Error)
	}
	require.NoError(t, db.Exec("INSERT INTO messages (id, session_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)", "other", "unrelated", "user", "private", at).Error)
	require.NoError(t, db.Create(&types.Message{
		ID: "deleted", SessionID: "s", Role: "user", CreatedAt: at, DeletedAt: gorm.DeletedAt{Time: at, Valid: true},
	}).Error)
	var cursor types.MemoryMessageCursor
	seen := map[string]bool{}
	for {
		rows, err := msgRepo.ListMessagesBySessionAfterCursor(ctx, "s", cursor, 40)
		require.NoError(t, err)
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			require.False(t, seen[row.ID])
			require.Equal(t, "s", row.SessionID)
			seen[row.ID] = true
			cursor = types.MemoryMessageCursor{At: row.CreatedAt, ID: row.ID}
		}
	}
	require.Len(t, seen, 85)
}
