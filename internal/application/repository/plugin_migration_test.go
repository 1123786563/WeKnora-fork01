package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPluginPreviewMigrationsIndexExpiresAt（跨任务转交 T01-R2-F1）：
// DeleteExpiredPreviews 以 `expires_at <= ?` 做条件删除并在每次预览成功
// 路径同步触发——expires_at 上没有索引时该 DELETE 是全表扫描且行锁跨全部
// 租户；TTL 可配置得很长或清理长期失败时表随流量线性增长，每次管理员
// 预览都要付出 O(表大小) 扫描，与「table must not grow without bound」
// 依赖的正是这条路径。两份建表迁移（本分支新增未发布，直接补行）都必须
// 带 expires_at 索引；本测试静态钉住 DDL 防回归误删。
func TestPluginPreviewMigrationsIndexExpiresAt(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "..", "..", "migrations", "versioned", "000189_plugin_previews.up.sql"),
		filepath.Join("..", "..", "..", "migrations", "sqlite", "000110_plugin_previews.up.sql"),
	} {
		raw, err := os.ReadFile(path)
		require.NoError(t, err, "migration file must exist: %s", path)
		sql := strings.ToLower(string(raw))
		require.Contains(t, sql, "create index idx_plugin_previews_expires_at",
			"%s must index expires_at for the lazy cleanup DELETE", path)
	}
}
