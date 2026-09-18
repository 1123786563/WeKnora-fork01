# CFT 决策记录（DECISIONS）

## D001 · assistant-ui 采用本轮设计要求（T006 安装锁版），旧 G2 决策标记被替代

- 日期：2026-09-18 · 决策人：CFT 总控
- 背景：`packages/views/src/craft/workbench.tsx:3-20` 记录旧计划 G2 决策——不安装 @assistant-ui/react，用纯 React 19 + useSyncExternalStore 实现"同 ExternalStoreRuntime 语义"的消息日志。
- 本轮设计（ADR-C02 + T006 + 成功标准"真正使用 assistant-ui"）明确要求锁定实际兼容的 SDK 版本并真实挂载 Provider/Thread/Message/Composer。
- 决定：**按本轮设计执行**——T006 安装并锁版 @assistant-ui/react，用 ExternalStoreRuntime 适配现有 controller + createCraftMessageLog 投影；W05 的纯 React 投影作为数据层保留（正是 ExternalStoreRuntime 需要的外部 store），UI 层换为 assistant-ui primitives。旧 G2 决策注明"被 CFT-S00-T006 替代"，保留历史追溯，不删除 W05 投影逻辑。
- 约束：不重建通信协议；不由浏览器决定主 Run 终态；StrictMode/卸载/重复订阅要有测试。

## D002 · e2e harness 修复定位为装配/harness 层，不动业务代码

- 日期：2026-09-18 · 决策人：CFT 总控
- T001 复验发现 craft run 永远 queued。根因一：`AgentRunWorker` 是 craft admission 的唯一消费者，`agent.recovery.enabled` 默认 false（opt-in）导致不消费。根因二：`config/builtin_models.yaml` 的 `builtin-llm-mock`（局域网地址）抢占 is_default，`craftChatModelID` 命中它后 SSRF 拒绝。
- 决定：修复全部落在 `apps/web/e2e/craft-stack.sh`（测试 harness）：显式启用 recovery worker env；seed 时将 builtin-llm-mock 降为非默认。不改 config 默认值、不改 craftChatModelID 遍历逻辑（那是产品行为，如需调整属于独立任务并与 owner 确认）。
- 影响：生产部署若要用 craft，需要在部署配置显式 `agent.recovery.enabled=true`（这一点已由 config 注释与 ValidateAgentRuntimeConfig 表达）；T033 灰度任务需把该前提写进发布清单。

## D003 · `expectedWorkspaceRevision` 差异移交 T002 裁决

- 目标 DraftIntent 携带 `expectedWorkspaceRevision`（提交时工作区 CAS），现有 `craftRunRequestDTO` / api-client submit 均无该字段；服务端 CAS 目前存在于 interaction decide（ExpectedRevision）与 restore。
- T002 冻结合同时三选一：后端补字段（推荐，语义已在设计中明确）；或前端合同降级记录为后续演进；或复用既有 live-run guard 视为等价保护。不允许各任务自行猜测。
