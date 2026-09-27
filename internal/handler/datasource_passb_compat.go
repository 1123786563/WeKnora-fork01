package handler

// Pass B 过渡 shim（26-datasource）：datasource.go / datasource_credentials.go
// 已物理迁移至 internal/modules/datasource/handler。本文件为 routes_infra.go:301-306
// （形参 :303-304）、router.go:118-119（RouterParams 字段）/:408（消费行）、
// router_api_key_capabilities_test.go:353 与 container.go:760/:857 dig Provide
// 提供 type/var 别名，router 侧零改动。
// 删除点：ib2 集成屏障直连后（Brief 指令）。

import (
	"github.com/Tencent/WeKnora/internal/modules/datasource/handler"
)

type DataSourceHandler = handler.DataSourceHandler
type DataSourceCredentialsHandler = handler.DataSourceCredentialsHandler

var (
	NewDataSourceHandler            = handler.NewDataSourceHandler
	NewDataSourceCredentialsHandler = handler.NewDataSourceCredentialsHandler
)
