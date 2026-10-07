package handler

// Pass B 过渡 shim（37-insights）：analytics.go 已物理迁移至
// internal/modules/insights/analytics/analytics_handler.go（模块留驻）。
// order 60 handler 批：evaluation_handler.go 已迁回本包并改名 evaluation.go
// （真身 EvaluationHandler/NewEvaluationHandler 同名同签名），Evaluation 侧
// 别名随批删除；本文件继续为 analytics 留守宿主消费方提供 type 别名与
// 转发声明，调用点零改动：
//   - container.go:746 dig Provide（NewAnalyticsHandler）；
//   - router.go:74/:379 具体类型参数 *handler.AnalyticsHandler（真 type
//     别名，方法集随型，routes_analytics.go:15 零改动）；
//   - usage.go:89/:114/:138 parseAnalyticsRange、:107/:128 analyticsRows
//     （commercial 属主留守文件，conventions §1.3 禁直改——转发声明覆盖）。
// 删除点：ib3 集成屏障直连后（Brief 指令；usage.go 调用点直改模块导出
// analytics.ParseAnalyticsRange / analytics.Rows 后删本文件）。
// Ruling 2026-09-25-TRANSITION-SHIM-ROW-REGISTRATION：manifest/matrix 成对
// 补行，ib3 同 commit 随文件删行。

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/modules/insights/analytics"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// AnalyticsHandler is the module type re-exported as a host alias so the
// router's concrete-type parameter keeps compiling unchanged.
type AnalyticsHandler = analytics.AnalyticsHandler

// NewAnalyticsHandler forwards to the module constructor (signature
// identical to the pre-migration host declaration).
func NewAnalyticsHandler(repo interfaces.AnalyticsRepository) *AnalyticsHandler {
	return analytics.NewAnalyticsHandler(repo)
}

// parseAnalyticsRange forwards to analytics.ParseAnalyticsRange for the
// remaining same-package commercial consumers (usage.go).
func parseAnalyticsRange(c *gin.Context) (time.Time, time.Time, bool) {
	return analytics.ParseAnalyticsRange(c)
}

// analyticsRows forwards to analytics.Rows for the remaining same-package
// commercial consumers (usage.go).
func analyticsRows[T any](rows []T) []T {
	return analytics.Rows(rows)
}
