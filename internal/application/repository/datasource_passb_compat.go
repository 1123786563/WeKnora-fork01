package repository

// Pass B 过渡 shim（26-datasource）：datasource_repo.go 已物理迁移至
// internal/modules/datasource/repository。本文件为留守宿主消费方
// （container.go:309-310 dig Provide、datasource_purge_test.go）提供
// type/var 别名，调用点零改动。删除点：ib2 集成屏障直连后（Brief 指令）。

import (
	"github.com/Tencent/WeKnora/internal/modules/datasource/repository"
)

type DataSourceRepository = repository.DataSourceRepository
type SyncLogRepository = repository.SyncLogRepository
type AppDataSourceBindingRow = repository.AppDataSourceBindingRow

var (
	NewDataSourceRepository = repository.NewDataSourceRepository
	NewSyncLogRepository    = repository.NewSyncLogRepository
)
