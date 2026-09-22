# Pass A Move Manifests（任务 F1）

本目录是 Pass A 模块归位的**唯一搬迁事实源**：16 份 `docs/architecture/moves/<module>.yaml`
逐一声明每个业务模块拥有的包搬迁（move_packages）、旧路径别名义务（alias_obligations）、
横向包内遗留文件（legacy_files）、允许写入的文件（owned_files）、验收命令（test_commands）、
集成点（integration_points）与禁改共享文件（forbidden_shared_files）。
Pass A 的每个模块 worker 只做 manifest 内声明的动作；scope 选择权不在 worker。
边界权威：`docs/architecture/backend-modules.yaml`（F0）+ 
`docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` §5.18。

## Manifest schema（LOCKED，任务 F2 以 KnownFields(true) 严格加载；加字段/改名需协调变更）

```yaml
module: <id>                      # 16 个业务模块 id 之一
description: <one line>           # 模块一句话职责（沿自 F0）
move_packages:                    # 本模块整体独占的 Go 包
  - from: internal/foo            # 现导入路径（== 目录）
    to: internal/modules/<id>/foo # 目标路径（规则见下）
alias_obligations:                # Pass A 在每个旧路径留下的无逻辑转发别名包
  - old_import_path: internal/foo # == move_packages[].from（一一对应）
    passb_task: B-<id>            # 删除该别名的 Pass B 任务
legacy_files:                     # 横向（多所有者）包内归本模块的遗留文件（非测试 .go）
  - path: internal/application/service/foo.go
    reason: trapped in horizontal ... package
    navigation_label: Foo service (application/service)
    passb_task: B-<id>
owned_files:                      # 本模块 Pass A worker 可创建/修改的全部文件
  move_sources: [...]             # == 全部 move_packages[].from 树
  move_targets: [...]             # == 全部 move_packages[].to 树 + 旧路径上的别名包文件
  importers:                      # 迁移前 grep 捕获的各搬迁包直接导入方（只做 import 路径修复，禁止结构改动）
  module_files:                   # internal/modules/<id>/{README.md,module.go,legacy/README.md}
test_commands:                    # 搬迁后证明本模块的精确命令
integration_points:
  routes: [...]                   # F0 归属本模块的路由注册入口（含 handler 侧委托）
  workers: [...]                  # asynq 任务类型（Redis/Lite 同集合）
  lifecycle_hooks: [...]          # container.Invoke 生命周期挂点
forbidden_shared_files:           # router/container/全局 worker 注册/go.mod/go.sum/migrations
```

## 确定性规则（rulings，worker 不得自行裁量）

1. **目标路径**：默认 `to = internal/modules/<module>/<原包叶子名>`；当包叶子名等于模块 id
   且该包是顶层包（`internal/<id>`）时，`to = internal/modules/<module>`（模块根）；
   非顶层的同名叶子（如 `internal/application/service/appconnector`）保留父级层：
   `internal/modules/appconnector/service/appconnector`。子包随最长已拥有祖先包整体搬移，
   保持相对结构（如 `internal/agent/approval` → `internal/modules/agentruntime/agent/approval`）。
   若无已拥有祖先包、但某祖先目录本身不是 Go 包且其下全部仓库包都属于本模块，
   则以该目录为搬移树根保留结构（`internal/application/repository/retriever/*` →
   `internal/modules/knowledge/retriever/*`；`internal/models/*` → `internal/modules/airesource/models/*`）。
2. **别名**：每个 move_package 在旧路径留一个无逻辑转发别名包（Pass A 建议用 type alias +
   var/func 转发；不承载逻辑），与 move_packages 一一对应，Pass B `B-<module>` 删除。
3. **legacy_files 粒度**：仅列非测试 `.go` 文件；同目录 `_test.go` 随其主题文件一并搬迁，不单列。
   横向 host 包内的共享文件（`internal/handler/session` 的 handler/helpers/types）按 F0
   plurality-owner 归属 conversation，Pass B 再细分。
4. **test_commands** 首条一律 `go test ./internal/modules/<id>/... -count=1`；knowledge/agentruntime
   附加 CI（anydoc.yml / agent-recovery.yml）既有包级命令的后搬迁路径等价形式。
   已知不稳定用例（F0 §2.4）：`agent/opencode` 挂起、`TestAgentRunDecisionConcurrentOnlyOneRevision`
   并发偶败 —— 归 agentruntime，重试前先查 F0 基线。
5. **forbidden_shared_files**（16 份 manifest 完全一致）：
   `internal/router/router.go`、`internal/router/task.go`、`internal/router/sync_task.go`、
   `internal/container/container.go`、`go.mod`、`go.sum`、`migrations/`。

## 覆盖恒等式（机械可查）

**所有 manifest 之并（move_packages ∪ legacy_files）+ 下述 platform/bootstrap 残留 = F0 清单；**
**manifest 两两交集为空。** F1 校验输出：move_packages=99（唯一）、legacy_files=396（唯一）、
交集空、F0 的 102 个业务模块包全覆盖（99 搬迁 + 3 个横向 host 包贡献 legacy_files）。

## Platform / Bootstrap 残留（不在任何模块 manifest 中）

### platform 包（20）

- `internal/common`
- `internal/common/redislock`
- `internal/database`
- `internal/errors`
- `internal/event`
- `internal/filetransport`
- `internal/logger`
- `internal/metrics`
- `internal/middleware`
- `internal/middleware/asynqdl`
- `internal/stream`
- `internal/textconv`
- `internal/tracing/langfuse`
- `internal/types`
- `internal/types/interfaces`
- `internal/utils`
- `internal/infrastructure/web_fetch`
- `internal/application/service/file`
- `internal/handler/dto`
- `internal/handler`

其中 `internal/handler` 是 F0 判定的 platform-owned 横向 host 包：其非测试文件已按域作为
legacy_files 分派进各模块 manifest；以下文件归 platform 本体（Pass B 处理）：

- `internal/handler/list_pagination.go` — 通用分页绑定助手（HTTP adapter 层）
- `internal/handler/upload_limit.go` — 上传体积上限助手（HTTP adapter 层）

### bootstrap 包（6）

- `cmd/server`
- `internal/container`
- `internal/router`
- `internal/config`
- `internal/runtime`
- `internal/assets`

### 横向 host 包（Pass B 逐文件拆分，包本体 Pass A 不整体搬移）

| 包 | F0 plurality owner | 非测试文件 | 分派情况 |
|---|---|---|---|
| internal/application/service | knowledge | 182 | 13 个模块 manifest 的 legacy_files |
| internal/application/repository | agentruntime | 95 | 14 个模块 manifest 的 legacy_files + platform `task_queue.go` |
| internal/handler | platform | 86 | 16 个模块 manifest 的 legacy_files + platform 2 文件 |
| internal/handler/session | conversation | 36 | 6 个模块 manifest 的 legacy_files |

### platform / bootstrap 的集成点残留（摘自 F0）

- 路由（9）：`GET /health`（router.go:175）、`GET /swagger/*any`（router.go:182）、静态前端
  （static.go）、资源授权/预签名文件路由（files.go）、workbench artifacts download HMAC grant
  挂载点（router.go:233-235，域归 workbench、机制归 platform）。
- Worker：6 个 asynq server 池拓扑（task.go:173-229,330-343）与 RunAsynqServer/RegisterSyncHandlers
  启动（task.go:231 / sync_task.go:142）。任务处理器归属见各模块 `workers`。
- Hooks：registerLangfuseCleanup（container.go:166）、registerPoolCleanup（container.go:169）、
  RunAsynqServer/RegisterSyncHandlers Invoke（container.go:1044/1046）。
- Migrations：`migrations/mysql/00-init-db.sql`、`000000_init` ×2、paradedb ×2、
  task_queue_and_dead_letters ×2（manifests 不含 migrations 字段；`migrations/` 全目录禁改）。
