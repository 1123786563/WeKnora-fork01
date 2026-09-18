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

## D004 · CFT 证据/台账曾被主线清理误删，T027/T031 证据从未入库——恢复与重生成策略

- 日期：2026-09-18 · 决策人：CFT 总控（恢复轮）
- 事实：合并提交 b08a67a6（craft/cft-execution → main）之后，清理提交 a35b0389 误删 `docs/craft/**` 与 `docs/testing/craft/**` 共 84 个文件（36 项证据、执行台账、发布门禁与 O05 钻井记录）。已由 cbb60257 从 b08a67a6 以 plumbing（临时索引，零触碰工作区）恢复并 fast-forward 并回 main。
- 另发现：`CFT-S04-T027/`、`CFT-S05-T031/` 两个证据目录在**任何分支上都不存在**（EVIDENCE.md 未及提交即随 worktree 清理丢失），index/task-status 的指针此前是悬空的。
- 决定：不凭记忆伪造原证据；在 main 上真实重跑同一名义测试重生成这两份证据（见各自 EVIDENCE.md），并在重跑中如实记录暴露的回归（D005）。台账其余内容按恢复原样保留，不改写历史结论。

## D005 · migrations/sqlite 三 lane 撞号 000058——保留最早、重编号其余

- 日期：2026-09-18 · 决策人：CFT 总控（恢复轮）
- 事实：mobile（000058_mobile_devices）、paseo（000058_paseo_control）、w24/execution（000058_execution_registrations）三条并行 lane 各自占用 000058；golang-migrate 加载 `migrations/sqlite` 即失败（duplicate migration file），阻断全部依赖迁移 harness 的 Go 测试（含 CFT-S05-T031 首轮重跑 4/4 FAIL），全新库部署/测试同样不可建。
- 决定：仅重编号、零内容改动——paseo_control→000077、execution_registrations→000078；保留 mobile_devices 的 000058（三者最早提交，且 000059/000060 同链）。已核实 000059-000076 无对被重命名表的依赖。
- 影响：长期存在的开发库若按旧号应用过 paseo/registrations 之一，重编号后会被视为新迁移而撞已有表，需重建该开发库；测试临时库与全新库不受影响。此修复同时解除非 Craft 测试面的同类阻断。
