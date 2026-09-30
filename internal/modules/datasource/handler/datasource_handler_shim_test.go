package handler

// Pass B 测试装置垫片（Ruling 2026-09-24-TEST-SUPPORT-SHIM，26-datasource B2-DS.4）：
// 随迁的 datasource_test.go / datasource_credentials_test.go 在迁移前消费宿主
// internal/handler 包共享测试 helper errorCapture（定义于
// auth_register_invite_only_test.go:36，auth 属主留守文件，禁改）。本垫片为
// 模块 handler 包唯一副本，实现逐字照录宿主定义（含注释）；不得改写宿主文件、
// 不得内联改写测试。remove_at: ib2（共享测试装置归属裁定后收口，台账
// docs/architecture/evidence/passb/b2-datasource.md §测试垫片追踪）。

import (
	"net/http"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/gin-gonic/gin"
)

// errorCapture mirrors gin's default ErrorHandler behaviour for tests:
// when a handler calls c.Error(), we surface it as an HTTP response so the
// recorder reflects the real client-visible status. The production
// middleware does the same thing in middleware/error_handler.go.
func errorCapture() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}
		err := c.Errors.Last().Err
		if appErr, ok := err.(*apperrors.AppError); ok {
			c.JSON(appErr.HTTPCode, gin.H{"error": appErr.Message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
