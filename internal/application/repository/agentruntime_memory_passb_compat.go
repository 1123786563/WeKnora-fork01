package repository

import (
	agentmemory "github.com/Tencent/WeKnora/internal/modules/agentruntime/memory"
)

// Pass B 宿主兼容层（b3-r-memory / R1.2）：memory 仓储 4 文件已物理迁移至
// internal/modules/agentruntime/memory（ownership-matrix plan=31 行）。
// 留守宿主消费方：internal/container/container.go:321。
// 删除点：ib3 集成屏障按 Integration Brief 直连模块构造器后，随 manifest/matrix
// compat 行同 commit 删除（Ruling 2026-09-25-TRANSITION-SHIM-ROW-REGISTRATION）。
var NewMemoryRepository = agentmemory.NewMemoryRepository
