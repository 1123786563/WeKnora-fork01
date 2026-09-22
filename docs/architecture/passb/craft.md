# Pass B Brief — B-craft（craft 模块收尾：别名删除 + 横向包遗留文件拆分）

Manifest：`docs/architecture/moves/craft.yaml`（模块 craft）。本 brief 覆盖 B-craft 的
全部义务：别名删除、container 切换、30 个 legacy 文件的域拆分。文件明细与
`internal/modules/craft/legacy/README.md` 逐条镜像。

## Scope（legacy_files，30 文件）

repository（6）：`internal/application/repository/` 下 craft_preview_check.go、
craft_scheduled.go、craft_snapshot.go、craft_usage.go、craft_version.go、craft_workspace.go

service（18）：`internal/application/service/` 下 craft_artifacts.go、craft_budget.go、
craft_control.go、craft_decision_delivery.go、craft_delegate.go、craft_inputs.go、
craft_interaction_store.go、craft_knowledge.go、craft_knowledge_tool.go、craft_lifecycle.go、
craft_preview.go、craft_recovery.go、craft_scheduled.go、craft_session.go、craft_snapshot.go、
craft_usage.go、craft_usage_view.go、craft_workspace.go

handler（1）：`internal/handler/craft_model_gateway.go`

handler/session（5）：`internal/handler/session/` 下 craft.go、craft_interaction.go、
craft_preview.go、craft_scheduled.go、craft_usage.go

（同目录 `_test.go` 随主题文件一并搬移，不单列。）

## 边界目标

工作区/会话/运行/Interaction/Snapshot/Artifact Version+Preview/用量/Scheduled Task 全域
归入 `internal/modules/craft`（§5.7/§5.18）。已搬入的域核心（contracts、request、
version、preview、document/spreadsheet/slides 渲染、budget、decision、lifecycle、recovery、
interaction、knowledge、skill、usage、snapshot、scope、input、release）与 Pass B 拆入的
service/repository/handler 层衔接为完整模块栈。建议结构 `internal/modules/craft/{app,repo,http}`
或按 A9-A11 先例分域子包；**craft 生命周期、Artifact 语义、schedule 语义是外部契约，
拆分不改行为**。

## 删除义务

1. **别名目录** `internal/craft/`（alias.go，7 符号：Store、VersionStore、PreviewCheckStore、
   Workspace、Executor、KindWeb、KnownKind）——前置条件：IA4 已完成
   `internal/container/container.go:42` 的 import 切换到
   `github.com/Tencent/WeKnora/internal/modules/craft`，此时旧路径零 importer，整目录删除。
2. 上述 30 个 legacy 文件（含各自 `_test.go`）从横向包删除，迁入模块；
3. A13 guard 移交的 4 条预存耦合中归 craft 侧的两条改走模块门面或在本任务删除例外：
   - `internal/modules/agentruntime/agent/opencode/{executor,normalizer}.go`、
     `internal/modules/agentruntime/agent/tools/craft_delegate.go` → `internal/modules/craft`
     （agentruntime→craft 方向；与 B-agentruntime 协调，或经 craft 模块根门面收敛）；
   - `internal/modules/craft/contracts.go` → `internal/modules/agentruntime/agent/runtime`
     （craft→agentruntime 方向，Pass B 引入窄端口或登记删除）。

## 集成点迁移（随文件拆分一并处理）

- 路由：`internal/handler/session/{craft,craft_interaction,craft_scheduled,craft_preview}.go`
  的 RegisterCraftXxx 委托（共 12+4+7+3 条 + 1 休眠挂载）与
  `internal/handler/craft_model_gateway.go` 顶层 pre-auth 网关（router.go:262-265 挂载、
  routes_chat.go:103/123/135 三分组挂载点保持挂载语句不动，委托目标改指模块内部）；
- 生命周期：container 侧 9 个挂点（container.go:665/668/680/684/1038 +
  internal/container/craft_interaction.go:81/92、craft_lifecycle.go:101/116/136）随
  B-container/bootstrap 收敛任务处理，本任务只保证模块侧提供等价构造函数；
- 用量视图：craft_usage_view 依赖 commercial 计量门面，保持只读消费。
