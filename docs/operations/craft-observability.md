# Craft 用量展示与运行诊断（O04）

面向空间管理员与有权成员的用量核对与故障定位手册。配套实现：

- HTTP：`GET /api/v1/sessions/:session_id/craft/usage`（`internal/handler/session/craft_usage.go`）
- 组合服务：`internal/application/service/craft_usage_view.go`（用量账本 O01 + 生命周期 O03 + 版本检查 W02）
- 前端：`packages/views/src/craft/usage.tsx`（挂载点 `CraftUsageMount`）+ `packages/domain/src/craft/usage.ts`
- 指标：`internal/metrics/craft.go`

## 用量端点与数据保证

响应携带：`as_of`（账本读取时刻，RFC3339）、`usage`（`known_calls`/`unknown_calls`/`input_tokens`/`output_tokens`/`cached_tokens`/`funding`）、`byok_model_borne_by_space`、`runs`（含 `failure_reason`）、`calls`（主/子物理调用逐条）、`sandbox_residency`（驻留/存储）、`checks`（当前版本检查）。

不做什么：

- **不造定价**：响应不含任何金额字段（amounts 仅由商业视图提供）。断言见 `TestCraftUsageHTTPShapeHidesMoneyAndSecrets`。
- **不把未知折叠成 0**：unknown 物理调用单列（`unknown_calls` + 调用行 status=unknown），不进 token 求和。
- **不含凭据/知识正文**：视图字段结构上不可能携带供应商密钥或材料内容。
- **BYOK 如实标注**：`byok_model_borne_by_space=true` 时前端明示"模型由空间自有凭据承担"，绝不显示为免费。
- **`cached_tokens` 是输入的子集**（provider 契约），展示时注明不重复计。

访问矩阵（与 W03 工作台 GET 同一读 ACL；服务端执行）：

| 调用者 | 结果 |
| --- | --- |
| 会话所有者 | 200 |
| 同租户管理员 | 200 |
| 同租户普通成员（非所有者、非共享） | 404（不可见） |
| 跨租户/跨空间 | 404（不存在） |
| 未注册装配（fail-closed） | 路由不存在（404，无 503 垫片） |

普通成员按现有权限只能看到自己可访问会话的用量；空间级聚合成本属商业视图，不在本端点。

## 指标（低基数）

`internal/metrics`，快照 API：`metrics.CraftMetricsSnapshot()`（Prom 风格命名，供运维/测试读取）。

| 指标 | 类型 | 标签（封闭词表） | 记录点 |
| --- | --- | --- | --- |
| `craft_delegations_total` | counter | `status`∈{succeeded,failed,canceled} | delegation settle（`craft_delegate.go`） |
| `craft_reconcile_total` | counter | `outcome`∈{corrected,matched,mismatched} | 晚到修正 / OC 汇总核对（`craft_usage.go`） |
| `craft_pending_decisions` | gauge | 无 | 每轮 lifecycle sweep 从 `craft_interactions` 刷新 |
| `craft_preview_failures_total` | counter | 无 | 预览签发 HTTP 缝（4xx/5xx，`craft.go` 挂载处） |
| `craft_usage_unknown_total` | counter | 无 | 用量账本写入 unknown 事实（`repository/craft_usage.go`） |
| `craft_workspace_restore_seconds` | histogram（固定桶） | `le` | 快照恢复耗时（`craft_snapshot.go`） |

**基数纪律**：标签值全部来自封闭词表，词表外的值折叠为 `other`（`TestCraftMetricsLowCardinalityVocabulary`）。**run/tenant/session/tool_call/delegation/prompt ID 永不进入 metrics label**——它们属于受控日志与 trace 属性；测试断言任何 series 键不含这些身份。

## 日志契约

delegation 派发与结算各一行结构化日志（`[CraftDelegation] dispatch|settled ...`），一行串起：

`tenant=… session=… run=… tool_call=… delegation=… prompt=<RequestHash>`

`prompt=` 记录的是请求内容哈希（promptID），**不是 prompt 正文**。默认日志永不包含：token 计数、材料/知识正文、完整模型请求/响应。沙箱删除与 tombstone 失败均有 `[CraftLifecycle]` 告警行（含 session/delegation 标识）。

## 排障三路径

从用户 session 出发：`GET /sessions/:sid/craft/usage` 先看 `runs`（主 Run 状态与 failure_reason）与 `calls`（子执行逐条、unknown 单列）。

1. **"正在停止"**（会话删除后资源未清）：查 `craft_lifecycle_states`：`state='deleting'` 行的 `reason` 写明保留原因（未明远端任务/待决交互）；`GuardDispatch`/`GuardRestore` 在此期间拒绝新派发与恢复（ErrBusy）。sweep 每轮重驱（默认开，`CRAFT_LIFECYCLE_SWEEP_DISABLED=true` 可关）；删除入口的 tombstone 失败不阻断删除，由 sweep 发现兜底。
2. **"不明结果"**（子执行结果未知）：`calls` 中 status=unknown 的行 + `craft_lifecycle_states.reason` 中的 risk 记录点名未证实的 delegation id；晚到用量以 corrected revision 落账，刷新用量面板（as_of 前移）即可核对；对账轨迹在 `craft_usage_facts` 的 revision 链（`Revisions` 读接口）。
3. **"预览失败"**：`checks` 中 `preview` 检查的 status/detail（not_run/failed/passed）；签发失败计入 `craft_preview_failures_total`；预览源站（隔离 https origin）未配置时签发端点直接不可用（fail-closed），先查 `WEKNORA_CRAFT_PREVIEW_ORIGIN`。

## 真实运行统计（实测，不外推）

> 本节只记录实测样本量与测得值，不写成本承诺。P50/P95 随样本更新。

见 `task-O04-report.md` 的"真实运行统计"一节（W06 全栈 mock 验收 + 用量端点实测）。
