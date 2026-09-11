# tRPC 恢复实施进度

- 设计：[已批准规格](../specs/2026-09-10-dual-agent-trpc-recovery-design.md)
- 计划：[实施计划](2026-09-10-dual-agent-trpc-recovery.md)
- 当前阶段：生产链路已接通，SQLite 单进程 SIGKILL 矩阵通过；PostgreSQL 矩阵、双 worker 竞争、API 黑盒与前端配套未完成，功能保持默认关闭。
- 范围：两引擎分会话，复用现有能力，仅 tRPC 持久化恢复，未知结果等待用户。
- 规划基线：`e91f8af`。重新执行工作树：`codex/trpc-recovery-r2`（自 `d370254` 起步）。

状态：pending / in_progress / implemented / reviewed / verified。只有目标测试及必要实网/进程验收通过才能 verified。

| ID | 任务 | 状态 | 实现提交 | 验证 / 审查证据（2026-09-12 重新收集） |
|---|---|---|---|---|
| 01 | SDK 恢复实证 | verified | e44678d..3049317（上轮） | `go test ./internal/agent/trpc` 40 用例 PASS（含 -race）；SDK v1.10.0 固定，无绝对路径 replace |
| 02 | 共享装配与引擎类型 | verified | 2c5c3be（上轮） | 装配/引擎校验测试 PASS；全部会话创建入口缺省 builtin 经查证（嵌入/IM/技能安装走 DB 默认） |
| 03 | Run 与租约存储 | verified | b6c4f6c（上轮） | SQLite 全迁移测试 24 用例 PASS；PostgreSQL 因 `TRPC_TEST_POSTGRES_DSN` 未设显式 SKIPPED（未验收） |
| 04 | 模型与 checkpoint | verified | fd3f5b2..fd66784 + 9bad9e0 | 本轮修复：版本 0 种子检查点严格解码、分支 pending write 恢复物化；checkpoint 往返与 -race PASS |
| 05 | 工具日志与策略 | verified | a48c4ac..7908475（上轮） | runtime/repository 测试 PASS（含 -race）；外部计数=1、结果读回含 OutputFiles 由矩阵覆盖 |
| 06 | tRPC 图纵向链路 | verified | aa52f855..6e357dd + 6599817 | 图批次/恢复测试 PASS；本轮接通模型工具声明与流式，端到端经生产执行器测试与 SIGKILL 矩阵覆盖 |
| 07 | 受理与后台接管 | implemented（缺口见下） | f1f6682..a0e2384 + 6599817 | 生产图执行器已注册（`ExecuteDurableRun`）并注入 worker，启动门禁在 Start 校验；受理快照含完整可重建配置，请求哈希绑定内容；等待类错误映射 waiting_user。剩余：双 worker 竞争矩阵、worker 级瞬时退避记账 |
| 08 | 持久化等待与决策 | implemented（缺口见下） | 7211a3f..4b01b2f（上轮） | 决策 CAS/幂等/重试链路测试 PASS；矩阵验证未知结果 parked + 用户显式 retry 计数=2；OAuth 等待仍为内存 Gate（未接持久化） |
| 09 | Sandbox 恢复 | implemented（缺口见下） | 5e9469c..fca462f + 6599817 | 占位 hook 已替换为 provider 沙箱列表查询（`ObserveInstance`：绑定校验+List 探活）；alive 继续/missing 停靠。剩余：alive/lost/destroyed 三态 fixture 端到端断言、执行期 external_task_ref 写入 |
| 10 | 能力完整复用 | implemented（缺口见下） | ded7719+850e8a0 + 6599817 | 生产执行器经 `prepareAgentCapabilities` 复用全部装配（工具注册/MCP/Skills/提示词/记忆召回/VLM）；能力快照漂移拒绝生效。剩余：恢复期延迟 MCP 集合比对、多模态端到端 |
| 11 | 事件、投影、steering | implemented（缺口见下） | 7eb186e..54d061e + 6599817 | 事件生产调用点已接通（run_started/attempt_replaced/tool_dispatched/tool_result/run_failed/run_completed）；finalize 单事务含消息+终态+事件+槽位释放；矩阵断言 0 丢失事件。剩余：steering 走 RunInput（无生产调用点）、outbox、保留水位裁剪 |
| 12 | HTTP 与生命周期 | verified | 3c789fc..2c12fc3（上轮，复审 PASS） | handler/router/lifecycle 测试 PASS；跨租户 404、引擎不可变、取消/删除围栏经复审确认；`ValidateEngineUpdate` 缺直接单测（小缺口） |
| 13 | 客户端恢复交互 | partial | adbff8d（上轮） | reducer 3 用例 PASS；恢复卡已挂载。缺口：新会话引擎选择器未加、SSE 重连未消费 run 事件回放、vue-tsc 环境不可用未跑 type-check |
| 14 | 崩溃矩阵与启用门禁 | in_progress | 2de9216 + d86f069 + 9bad9e0 | 真实 provider 二进制 + SQLite 单进程矩阵 8/8 PASS（六个持久化边界 + 用户重试 + 幂等重投）；矩阵发现并修复两个真实缺陷（静默空恢复、版本 0 种子）。剩余：双 worker 竞争、PostgreSQL 矩阵、API 黑盒行、沙箱三态、预算/权限行 |

## 2026-09-12 重新执行记录（codex/trpc-recovery-r2，基线 d370254）

- 基线核验：四组并行独立核验（Tasks 01-05 / 08+12 / 11+13 / 09 集成面）全部完成，证据见任务行与验收文档；仓库全量构建 PASS；唯一已知无关失败 `TestSkillPythonVerifier`（技能 venv 环境相关，在干净 main 同样失败）。
- `6599817`：生产图执行器 + 受理快照 + 容器接线 + provider 恢复 hook + 事件生产调用点 + worker 等待映射。
- `d86f069`：真实 provider 二进制 + SIGKILL 矩阵；发现静默空恢复缺陷（SDK 在 pending branch write 时不规划恢复边界 → 恢复零执行即“成功”）。
- `9bad9e0`：仅物化 branch 写（全量清除破坏 checkpoint 往返契约，被 -race 运行的测试发现）。
- 门禁状态：功能默认关闭不变；SQLite 矩阵通过不等于发布门禁通过，剩余矩阵行见验收文档。

## 执行记录要求

每次任务追加开始/结束时间、实现者、固定 HEAD、失败测试原因、通过命令、审查问题与修复提交。保留历史记录，不用最终 PASS 覆盖中途失败。

SDK 版本固定 v1.10.0（根模块，无子模块）。PostgreSQL 验收需要 `TRPC_TEST_POSTGRES_DSN`，未设置的组合保持未验收。
