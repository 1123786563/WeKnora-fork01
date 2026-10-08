package repository

// Pass B 过渡 shim（37-insights）：analytics.go 已物理迁移至
// internal/insights/analytics/analytics_repository.go。本文件为
// 留守宿主消费方（container.go:217 dig Provide）提供转发构造器，调用点
// 零改动。删除点：ib3 集成屏障直连后（Brief 指令）。
// Ruling 2026-09-25-TRANSITION-SHIM-ROW-REGISTRATION：manifest/matrix 成对
// 补行，ib3 同 commit 随文件删行。

import (
	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/insights/analytics"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// NewAnalyticsRepository forwards to the module constructor (signature
// identical to the pre-migration host declaration).
func NewAnalyticsRepository(db *gorm.DB) interfaces.AnalyticsRepository {
	return analytics.NewAnalyticsRepository(db)
}
