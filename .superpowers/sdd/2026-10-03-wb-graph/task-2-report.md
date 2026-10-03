# Task 2 实施报告：准入冻结 graph 执行身份（WB-GRAPH）

repo: main @ 1f4eb3aa9 之上。设计依据：task-1-report.md（缝选择/合并形态/短码/测试清单）。

## RED（先写测试）

| # | 测试 | 位置 | RED 形态 |
|---|---|---|---|
| 1 | TestAdmissionFreezesServerResolvedGraphCore | workbench/admission_graph_test.go | build failed：`SetAdmissionGraphFreezer undefined` |
| 2 | TestAdmissionGraphResolutionFailureRejectsWithoutRun | 同上 | build failed：`ErrGraphResolutionFailed undefined` |
| 3 | TestAdmissionWithoutFreezerKeepsLegacySnapshot | 同上 | 同上（编译失败，真 RED） |
| 4 | TestParseDurableRunSnapshotAcceptsWorkbenchGraphCoreSnapshot | service/agent_run_snapshot_test.go | **先行通过**：D8 已把全部键类型化，严格读端无需改动——此测试为组合形状的契约护栏（锁定 D8+graph-core 共存语义） |
| 5 | TestExecuteDurableRunWorkbenchAdmissionWithGraphCore | service/agent_run_graph_test.go | **先行通过**：组合快照 ModelID 非空 → D8 分支自然放行，run_started/run_completed、模型输出>0、Craft 门零触发——证明"不改 executor"前提成立 |

RED 输出（workbench）：
```
admission_graph_test.go:31:14: coordinator.SetAdmissionGraphFreezer undefined (type *AdmissionCoordinator has no field or method SetAdmissionGraphFreezer)
admission_graph_test.go:63:161: undefined: ErrGraphResolutionFailed
FAIL [build failed]
```

## GREEN（实现）

- `workbench/admission.go`：`ErrGraphResolutionFailed` 哨兵；`AdmissionGraphFreezer` 接口 + `SetAdmissionGraphFreezer`（照 SetPublishedAgentVersionResolver 模式，nil 安全）；`admitPending` 在 binding 完整性检查后、快照 marshal 前调用 freezer——失败同口径 settle→rejected（无 Run 行、预留未创建）；成功则 graph 核心键并入同一 `map[string]any` 一次 marshal。freezer 错误未带哨兵时由 admitPending 兜底包一次 `%w`（防双重包装）。
- `service/admission_graph_freeze.go`（新）：`sessionService.freezeAdmissionGraphCore` 复用 AgentQA 同一解析链（resolveRetrievalTenantID → buildAgentConfig → resolveChatModelID → GetModelByID 填 MaxContextTokens → agentRequiresRerankModel → BuildDurableRunSnapshot）；`graphResolutionFailure` 按消息映射短码（model_unresolved/model_unavailable/rerank_unresolved/config_unbuildable，未识别默认 config_unbuildable，fail-closed）。
- `service/session.go`：新增 `customAgents interfaces.CustomAgentService` 字段+构造参数（builtin+自有 agent 统一经 customAgentService.GetAgentByID 解析）；NewSessionService 构造时 `RegisterAdmissionGraphCoreFreezer(svc.freezeAdmissionGraphCore)`（照 RegisterGraphExecutor 懒注册先例）。
- **executor 零改动**：D8 分支（agent_run_graph.go:562）凭 ModelID 非空自然放行，测试 5 实证。

### 与设计的一处偏差（记录）

设计原案：freezer 实现直接满足 workbench 接口（service→workbench import）。实测**测试环**：workbench 包测试已导入 appservice（admission_test.go 等），service→workbench 会构成 `import cycle not allowed in test`。改用无环结构：service 侧定义纯参数函数类型 `AdmissionGraphCoreFreezer(ctx, tenantID, actor, sessionID, agentID, text)` + 懒注册；workbench 接口/哨兵/seam 形状不变；容器内 `admissionGraphFreezerFunc` 适配。语义与设计一致。

## 容器装配点

- `internal/container/workbench.go`：`wireWorkbenchAdmissionGraphFreezer(_ interfaces.SessionService, admission *workbenchservice.AdmissionCoordinator)`——拉 SessionService 触发构造→注册表就绪→`SetAdmissionGraphFreezer(admissionGraphFreezerFunc(core))`；registry 为 nil 时不装（legacy 行为，nil 安全）。
- `internal/container/container.go:1239`：`must(container.Invoke(wireWorkbenchAdmissionGraphFreezer))`（紧随 wireTaskDeletionGuard）。
- `internal/router/craft_b5_joined_test.go`：NewSessionService 位置参数补一个 nil。

## 全量门

| 门 | 结果 |
|---|---|
| 新选择器（两包 5 测试） | `ok workbench 2.699s` / `ok service 1.831s` |
| workbench 包全量 | `ok 31.115s`（全绿） |
| service 包全量 | 32 FAIL —— 与干净 worktree 基线（HEAD@1f4eb3aa9）**按名称逐项完全一致**（diff 仅耗时数字），零新增失败 |
| router / container 包（构造器波及面） | `ok 35.892s` / `ok 43.999s` |
| `go build ./...` | exit 0（仅既有 ld duplicate-libraries warning） |
| `git diff --check` | 干净 |

## Concerns

1. **短码映射靠消息匹配**（`chat model is not configured` 等前缀）：解析链错误是共享代码，未加哨兵以免扰动 AgentQA 语义；映射 fail-closed（未识别→config_unbuildable 仍显式拒绝）。若上游文案改动需同步映射。
2. **v1 只解析 caller-tenant agent**（builtin/自有）：跨租户共享/adopted agent 仍走原准入道（`ponytail:` 注释已标 ceiling）。
3. builtin agent 无 model_id 的部署将得到 `model_unresolved` 准入拒绝（显式、可修配置）——符合 fail-closed 契约，但需在 Task 4 活体验证确认默认部署 model 配置齐全。
4. registry 全局单例：多 NewSessionService 构造时后写覆盖前者（仅测试场景出现，生产单例）。

## 提交

`feat(workbench): 准入冻结 graph 执行身份，打通 workbench→durable executor (WB-GRAPH)`（含 9 文件 + 本报告；不 push）
