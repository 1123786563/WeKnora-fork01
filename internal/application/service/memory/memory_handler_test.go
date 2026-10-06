package memory_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service/memory"
	"github.com/Tencent/WeKnora/internal/middleware"
	agentmemory "github.com/Tencent/WeKnora/internal/modules/agentruntime/memory"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Ruling 2026-09-28-TEST-PACKAGE-SPLIT（b3-r-memory / R1.3；先例 R1.2
// persistence_message_paging_test.go）：原 internal/handler/memory_consistency_test.go
// 随 memory_handler.go 迁入本目录后，同包测试 import internal/middleware 与
// R1.2 宿主 compat（repository→memory，container.go:321 消费）构成确定性
// import cycle（middleware→repository(kb_access.go)→memory）。本文件按裁定转
// package memory_test 外部测试包：用例名、子用例与断言逐字不变，仅构造器/
// 错误值改经导出符号（memory.NewMemoryHandler / memory.ErrSensitiveContent）
// 等价组装。remove_at: IB3 核验（conventions §10.8）。
type memoryFailureService struct{ interfaces.MemoryService }

func (memoryFailureService) ConfirmItem(context.Context, string) (*types.MemoryItem, error) {
	return nil, fmt.Errorf("confirm: %w", types.ErrMemoryConflict)
}

func (memoryFailureService) UpdateItem(context.Context, string, string, int) (*types.MemoryItem, error) {
	return nil, fmt.Errorf("edit: %w", memory.ErrSensitiveContent)
}

func TestMemoryConsistencyHTTPFailures(t *testing.T) {
	router := gin.New()
	router.Use(middleware.ErrorHandler())
	handler := agentmemory.NewMemoryHandler(memoryFailureService{})
	router.POST("/memory/items/:id/confirm", handler.ConfirmItem)
	router.PUT("/memory/items/:id", handler.UpdateItem)
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodPost, "/memory/items/stale/confirm", "", http.StatusConflict},
		{http.MethodPut, "/memory/items/item", `{"content":"sensitive"}`, http.StatusBadRequest},
	} {
		t.Run(tc.method, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			require.Equal(t, tc.status, recorder.Code, recorder.Body.String())
			require.Contains(t, recorder.Body.String(), `"success":false`)
		})
	}
}
