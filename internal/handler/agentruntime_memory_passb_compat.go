package handler

import (
	agentmemory "github.com/Tencent/WeKnora/internal/modules/agentruntime/memory"
)

// Pass B 宿主兼容层（b3-r-memory / R1.3）：internal/handler/memory.go 已物理迁移至
// internal/modules/agentruntime/memory/memory_handler.go。
// 留守宿主消费方：internal/router/router.go:122、internal/router/routes_memory.go:17、
// internal/container/container.go:867。删除点：ib3 直连后同 commit 删除。
type MemoryHandler = agentmemory.MemoryHandler

var NewMemoryHandler = agentmemory.NewMemoryHandler
