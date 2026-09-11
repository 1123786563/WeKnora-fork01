### Task 7: A07: 飞书和Notion同步迁移、游标与暂停

> **协调者编号修正（deviation note）**：原计划迁移编号 000119/000039 已被 A03（app_actions）占用。本任务改用下一空闲编号：PostgreSQL `000120_app_datasource_bindings`、SQLite `000040_app_datasource_bindings`。逻辑约束不变。

**Files:**
- Create: `internal/appconnector/sync.go`、`internal/appconnector/sync_test.go`
- Modify: `internal/application/service/datasource_service.go`、`internal/datasource/scheduler.go`
- Modify: `internal/application/repository/datasource_repo.go`
- Create: `internal/application/service/appconnector/sync_test.go`
- Create: `migrations/versioned/000120_app_datasource_bindings.up.sql`、`migrations/versioned/000120_app_datasource_bindings.down.sql`
- Create: `migrations/sqlite/000040_app_datasource_bindings.up.sql`、`migrations/sqlite/000040_app_datasource_bindings.down.sql`

**Interfaces:**
- Consumes: A01/A02空间连接；U05ExecutionGate；现有 datasource.Connector / StreamingConnector / StreamHandler.Checkpoint。
- Produces: `CanAdvanceCheckpoint(contentCommitted bool,currentFence,workerFence int64) bool`；`SyncBinding{TenantID uint64; InstallationID,ConnectionID,DataSourceID string; AuthVersion int64}`；`ResolveSyncBinding(context.Context,uint64,string)(SyncBinding,error)`。

**行为与边界：** 飞书和Notion各自全量/增量/断点恢复；重复运行不重复建条目；源失败不误删；套餐到期停新同步并允许已持久化结算；执行写入与调度使用空间连接。

**验收映射：** SYNC-01 至 SYNC-05；B24；AC-16

- [ ] **Step 1: 在 `internal/appconnector/sync_test.go` 写入以下失败断言。**

```go
package appconnector
import "testing"
func TestSyncCheckpointWaitsForDurableContentAndFence(t *testing.T) {
    if CanAdvanceCheckpoint(false,2,2) { t.Fatal("lost uncommitted content") }
    if CanAdvanceCheckpoint(true,2,1) { t.Fatal("stale worker advanced cursor") }
    if !CanAdvanceCheckpoint(true,2,2) { t.Fatal("valid checkpoint refused") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/appconnector ./internal/application/service ./internal/datasource -run "Test(Sync|DataSource)" -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func CanAdvanceCheckpoint(contentCommitted bool,currentFence,workerFence int64) bool {
 return contentCommitted && currentFence==workerFence
}
```

保持现有 Connector 协议不变，服务层在调度前解析空间连接与当前预算，实例游标绑定版本和租约fence。内容与索引任务持久化交接后才能推进 checkpoint；同源ID/版本使用现有知识映射去重。暂停原因分别为budget/plan/permission/quota/provider_limit，恢复沿用游标。

数据源迁移映射放 A01 的关联表 `app_datasource_bindings(tenant_id,datasource_id,installation_id,connection_id,auth_version)`；该表迁移应由本任务独立新增 PG000120/SQLite000040 up/down，不能改已应用A01迁移。旧凭据无法证明个人/空间归属标记requires_reauthorization。其他数据源继续旧路径但不能绕过已启用空间的预算；源端404/权限拒绝/删除分开，默认保留本地副本。

- [ ] **Step 4: 加入本任务"行为与边界"列明的反例，并执行同一定向命令。**

```sh
go test ./internal/appconnector ./internal/application/service ./internal/datasource -run "Test(Sync|DataSource)" -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/appconnector/sync.go internal/appconnector/sync_test.go internal/application/service/datasource_service.go internal/datasource/scheduler.go internal/application/repository/datasource_repo.go internal/application/service/appconnector/sync_test.go migrations/versioned/000120_app_datasource_bindings.up.sql migrations/versioned/000120_app_datasource_bindings.down.sql migrations/sqlite/000040_app_datasource_bindings.up.sql migrations/sqlite/000040_app_datasource_bindings.down.sql docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: migrate team sync to scoped connector execution"
```
