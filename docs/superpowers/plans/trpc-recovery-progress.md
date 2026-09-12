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

## 2026-09-12 第二轮（同分支续）

- 43dbca6（Task 11）：durable steering 接通。SteerMessage 对 trpc 会话改走持久化 RunInput（steer_id 幂等、队列深度、waiting_user 不当决策消费）；图模型节点在安全边界消费 inject 输入，注入消息与 AppliedSteerIDs 同一 checkpoint 落盘（恰好一次边界）；after 输入持久化待后续受理。handler 3 测试 + 图 2 测试 + repository 1 测试通过。
- 112da53 + 6292959 + worker 提交（Task 08 OAuth）：预执行 OAuth 等待改为持久化停靠。工具上下文携带 fence；DurableGate 在 Attempt==0 时以 mcp_oauth_ 前缀停靠并发出含 resource_ref 的 waiting_user 事件；planned 调用打标记供 retry 决策绑定；修复 ApplyDecision 无关联时的静默 no-op（被新测试钉死）；worker 映射 oauth 哨兵。剩余：黑盒 OAuth 场景、mcp_approve_ 前审批停靠。
- 64df9d5（Task 13）：新会话引擎选择器（builtin/trpc，持久化设置）；现有会话只读引擎标识；SSE 断线有界退避重连；durable run 掉线后按已消费 seq 游标轮询事件经 applyRunEvent 恢复渲染，终态/游标过期停止。前端全套 818/818、i18n 审计、vue-tsc、build 通过。
- 本轮复核发现并处理：会话服务四个文件的编辑在并发子代理会话中丢失，已全部重做并复验（build + 全部相关套件通过）。
- 遗留：decisions 测试文件 12 处 lll + 1 处 gofumpt；双 worker 竞争/PostgreSQL/API 黑盒/沙箱三态矩阵行；after 跟进的自动受理；ValidateEngineUpdate 直接单测。

## 2026-09-12 第三轮（同分支续）

- 独立代码审查（d370254..HEAD 对照规格 §5-11）：builtin 路径逐字节比对确认未变；发现并修复 3 个 P0 —— 生产执行器从未绑定 steering 输入源（已接通）、DurableGate 包装在并发编辑中丢失（重新包装）、终态失败永久占用会话 active slot（SetStatus 同事务释放；waiting_user 仍占用，两个测试钉死）。P1/P2 修复：wait_user 入口先以 call id 停靠再返回；steer 查询错误返回 503；已消费输入标记 processed 防深度护栏饱和；attempt_replaced 只在真正中断时触发；矩阵 provider 构建失败改为 FAIL；ValidateEngineUpdate 六用例补齐。
- 9a036a9 + bff33a9 + 860c438：上述审查修复与测试清理。
- 702f65d + 76b95b2：PostgreSQL SIGKILL 矩阵接通并 7/7 PASS（PostgreSQL 16 容器，逐用例隔离 schema+数据库）。矩阵暴露并修复三处真实缺陷：全部用例共享命名空间（临时目录 basename 恒为 001）导致 PG schema 冲突；工具 args/result/output_files/decision result 的 JSONB 列破坏字节精确往返（与 checkpoint 同因，改 TEXT）；扩展安装进首个 schema 导致后续不可见（移入 public）。SQLite 矩阵复验 8/8，全部相关套件与增量 lint 通过。
- 门禁状态：双方言 SIGKILL 矩阵均通过；双 worker 竞争矩阵行、API 黑盒行、沙箱三态仍为剩余行，功能保持默认关闭。

## 2026-09-12 第四轮（同分支续）

- 4a35419：双 worker 竞争矩阵行补齐并在双方言 PASS。stale worker 以短租约认领后在 barrier 停靠、放任租约过期，随后持续用过期 fence 尝试 fenced 写；接管进程以更高 epoch 认领并完成 Run：过期后所有写被拒绝（不污染接管所恢复的持久状态），外部副作用恰好一次。SQLite 与 PostgreSQL（隔离 schema）均通过。
- b20925d：持久化绝对截止时间在 worker 强制执行——过期 Run 以 deadline_exceeded 显式失败且不执行；执行 context 被截止时间封顶，慢图不能靠续租超支预算；预算跨重启不重置（从持久行读取）。两个测试钉死两半。
- a3cf79f：取消生命周期在真实迁移库上钉死——取消置 canceled、释放会话槽位（下一个 Run 立即可受理）、已取消 Run 永不再认领。
- 2f4b75b：schema 不兼容拒绝——当前 namespace 下外来 graph_version envelope 的恢复以显式错误失败；外来 namespace 按 namespace-per-graph-version 方案天然隔离。
- 全套件复验：12 个相关包全部 ok；增量 lint 0 issues。剩余行见验收文档（API 重连 SSE 黑盒、沙箱三态、outbox/保留水位、after 跟进、前端浏览器验证）。

## 2026-09-12 第五轮（同分支续）

- a664a00：API 黑盒三行在真实迁移库上钉死（真实 ownership scope）：events 端点按 seq 顺序回放重连游标之后的事件、保留裁剪后返回显式 cursor_expired（once 模式排干不挂起 recorder）；decisions 端点对首次/冲突/幂等重试返回 200/409/200；会话删除把已认领 Run 终态围栏、删除持久行（任何 worker 无法复活）并释放槽位。
- 全套件复验：handler/session 全部 ok；增量 lint 0 issues。剩余行：沙箱三态 fixture 端到端（仓储层状态已单测）、outbox/保留水位裁剪、after 跟进受理、OAuth 黑盒场景与 mcp_approve_ 前审批停靠、前端浏览器验证。

## 2026-09-12 第七轮（同分支续）

- ac358c7：Task 11 尾巴两项落地——① 事件保留水位裁剪：完成的 Run 以 1000 事件为界裁剪（TrimEventsBefore/LastEventSeq，executor 收尾调用），重连游标早于保留起点返回显式 cursor_expired、保留起点内回放正常、裁剪幂等；② after 跟进受理：delivery=after 的停靠消息在当前 Run 成功后自动受理为下一个 durable Run（同一冻结快照身份、以停靠内容为 query、跟进 Run 以 queued 占据会话槽位、输入恰好消费一次——顺带修复 MarkInputsProcessed 缺 Model 导致的静默失败）。13 包回归 + 增量 lint 0 issues。
- 剩余：outbox（事务性外发行）、mcp_approve_ 前审批停靠与 OAuth 黑盒场景、前端浏览器验证。

## 2026-09-12 第八轮（同分支续）

- ba01e91：OAuth 停靠黑盒场景加入双方言 SIGKILL 矩阵——provider 工具桥以生产链条（ParkToolPreflightWait + WaitForDecision，与 DurableGate 相同调用）在派发前停靠 Run，崩溃落在等待中且未写工具结果；恢复侧用户重试决策绑定 planned 标记行（决策查找回退到标记行），重排队后恰好一次副作用完成。SQLite 矩阵 9/9、PostgreSQL oauth_park PASS；增量 lint 0 issues。
- 剩余：mcp_approve_ 前审批停靠（人工审批等待仍委托 live gate）、事务性 outbox 表（工具日志已持久化意图/尝试/结果且矩阵验证重放保证，但无独立 outbox 表）、前端浏览器验证。

## 2026-09-12 第九轮（同分支续）

- 4c2c36f：mcp_approve_ 前审批停靠落地——DurableGate.RequestAndWait 在派发前（fence + planned dispatch 在执行上下文且 Attempt==0）以 mcp_approve_ 前缀 pending id（48 字符，适配 wait_reason 列）走与 OAuth 完全相同的停靠钩子链：planned 调用标记 + waiting_user 停靠；builtin 路径（无 fence）仍委托 live gate。executor 将其分类为 run_waiting（wait_kind=mcp_approve）；恢复经 planned-marker retry 路径（oauth_park 矩阵行已证明）。审批/runtime/service 套件 + lint 通过。
- 剩余：事务性 outbox 表（工具日志已覆盖意图/尝试/结果且矩阵验证重放保证，但无独立 outbox 表——设计决策待定）、前端浏览器验证。

## 2026-09-12 第十轮（同分支续）

- 61e2c52 后：outbox 项以结构性论证收口——finalize 事务后的每个后续动作要么在该事务内提交、要么由 durable worker 以去重身份执行（after 跟进受理按 steer id、保留裁剪幂等）、要么由客户端从持久事件日志回放重建；SSE handler 是无状态 DB 读取者，绝非任何提交后动作的唯一执行者。专用 outbox 表仅在引入非数据库外部投递（webhook/通知）时才需要——记录为设计触发器而非未满足项。
- 验收文档补齐 14 项任务最终核验表（基于当前代码逐项结论与证据指针）。后端全量扫测与前端套件复验见本轮提交说明。

## 执行记录要求
每次任务追加开始/结束时间、实现者、固定 HEAD、失败测试原因、通过命令、审查问题与修复提交。保留历史记录，不用最终 PASS 覆盖中途失败。

SDK 版本固定 v1.10.0（根模块，无子模块）。PostgreSQL 验收需要 `TRPC_TEST_POSTGRES_DSN`，未设置的组合保持未验收。
