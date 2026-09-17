# 当前状态矩阵 · MX-001 对账基线

基线：main `6a70c35a19c58511b8372b5a68b5147ab577ad14`（2026-09-18 复核）。
来源：三路只读代码审计（Go 后端 / 移动 TS / 台账与测试门禁），全部结论附当前 checkout 的代码位置。
分类：**A** = 当前源码确认存在并接线；**B** = 本轮设计新增提案（服务端接通前不得发布）；**C** = 原计划要求、当前仍待核验。

## G01–G12 逐项矩阵

| G | 当前事实（当前事实→代码位置） | 已有测试 | 接线情况 | 本轮修复 | MX 负责人 | 缺失证据 |
|---|---|---|---|---|---|---|
| G01 SSE data 形状 | Go 只写 `data: compact(event.Payload)`，`id: seq`、`event: type`（internal/handler/session/workbench_read.go:310-322，调用链 flushWorkbenchEvents :251-260）；envelope 字段仅在 snapshot 返回。TS parser 要求完整 envelope 且 `id===seq`（apps/mobile/sources/weknora/platform/stream-transport.ts:19-93；packages/contracts/src/mobile/execution.ts:124-139）。**两端不互通**。无版本协商（仅 Last-Event-ID 头） | TS: stream-transport.test.ts；Go: W03 handler 测试。各用各的 fixture，从未互通 | 未接通（真实串联必失败） | envelope 进业务帧、控制帧不占业务 seq、版本协商 v2 | MX-004 | Go 实际输出字节直接喂真实 TS parser 的跨语言测试 |
| G02 审批命令 | Go 已有 `GET …/:run_id/interactions` + `POST …/interactions/:id/decisions`（approve/reject，body=decision_id+args_hash+expected_revision；internal/handler/session/workbench_commands.go:57-73；internal/workbench/interaction.go:34-46）。TS `ConversationCommands` 只有 cancel/steer/refreshPending（view-model.ts:44-48）；ConversationScreen 调用不存在的 `approve?.()/reject?.()`（ConversationScreen.tsx:36,39）；refreshPending 把 interactionID 当 request_id 查 lookup（view-model.ts:112） | view-model.test.ts；Go interaction CAS 测试（service/workbench/interaction.go:120-180） | Go 端 A；TS SDK 与页面缺失 | 类型化 decide adapter + 拆分刷新语义 | MX-005（SDK/命令）+ MX-019（页面） | 挂载页面交互测试；真实 backend 允许/拒绝/409/过期/撤权 |
| G03 capability/revision 硬编码 | VM 硬编码 `canCancel:true, canSteer:true`、`expectedRevision=0`（view-model.ts:108-111）；contracts 三态 `Capability{state,reason}` 与 `ExecutionDTO.capabilities/revision` 已存在（packages/contracts/src/mobile/execution.ts:3-10,22,84-94；internal/workbench/contracts.go:24-40）但 VM 全部丢弃。另：`createProductConversationViewModel` 全仓无调用方（死代码） | contracts parse 测试 | 契约 A；消费 C | 映射三态+reason；无 revision 禁用命令 | MX-003（冻结规则）+ MX-017/018/019（消费） | capability 不可用路径的页面证据 |
| G04 提交身份 | `createRequestID` 是**悬空导入**（ConversationScreen.tsx:5，view-model.ts 无此导出）；第二路径用 `sessionId:text` 拼 request_id（SessionView.tsx:443-445）；send controller 纯内存单飞（view-model.ts:159-179）；无持久化/无重启恢复。Go 侧 LookupRequest 状态机完备：pending/dispatching/admitted/rejected/unknown（internal/application/service/workbench/admission.go:253-266；repository/workbench_request.go:100-114） | Go admission/handler 测试；api-client executions.test.ts | Go A；移动端 C（发送链路实际不可用） | 网络前持久化 request_id+输入摘要；未知先 lookup 不换 ID | MX-006 | 杀进程/ACK 丢失/重启恢复的真实测试 |
| G05 版本组合 | expo ~55.0.8、react 19.3.0、RN 0.83.1、reanimated 4.2.3、worklets ^0.7.1、nitro 0.35.2、unistyles ~3.1.1（apps/mobile/package.json）；根 pnpm 10.28.2 vs mobile packageManager 10.11.0；renderer 19.0.0 与 react 19.3.0 组合待官方矩阵核对 | 无 | 未验证 | 按官方 SDK55 矩阵对齐锁文件 | MX-002 | doctor、双平台构建、关键 native 模块运行证据 |
| G06 SQLite 适配 | `ExpoSQLiteDatabase` 是自注入接口（withTransactionAsync: Promise<T>，execution-storage.ts:35-40）；expo-sqlite **从未被 import**，无真实原生适配器实例化；cipher XChaCha20-Poly1305 注入式（SecureStore key） | execution-storage.test.ts（内存 driver） | 存储层 C（真机不可用） | 真实 expo-sqlite 适配器 + 原生事务边界 | MX-002（依赖）+ MX-012（适配） | 真机读回、并发写、磁盘满、database locked |
| G07 Happy 依赖 | SessionView 大量 import Happy hooks（SessionView.tsx:31-32,37），渲染被 isDataReady/session 门控（:427-436），消息源 useSessionMessages（:712）；路由 (app)/session/[id].tsx 直渲染 SessionView；ConversationScreen 未挂载任何路由 | agentGoalActionHandler.spec.ts | 产品会话硬依赖 Happy 全局态 | 产品渲染器与 legacy 分离 | MX-009（路由容器）+ MX-017（渲染） | 无 Happy 登录时产品链独立运行证据 |
| G08 三态合并 | VM 将 status 坍缩自 execution_status，丢 run_status/settlement_status/capabilities（view-model.ts:123-134）；Go DTO 三态齐全（workbench/contracts.go:29-40）。**新发现**：Go GormCancelPort 允许 `recovering` 而 validRunStatus 枚举不含（service/workbench/interaction.go:235 vs workbench/contracts.go:74-81）——服务端枚举不一致 | contracts parse 测试 | 契约 A；消费 C | 三态分别保留+最后同步时间 | MX-003（冻结）+ MX-018（详情页） | 未知状态不显示失败/完成的页面证据；Go 枚举对齐 |
| G09 source-event 授权 | 挂在普通 Viewer+API-key 分组（routes_workbench.go:17-22）；handler 先 ingest 后比对 run 归属（workbench_read.go:73-97，事后 404）；resolveBinding 无租户谓词全局查（repository/execution_observation.go:79-93） | 无针对性测试 | C（P1，remote profile 前必须修） | 服务身份+写入前 tenant/run/binding 校验 | MX-027 | 未断言可利用，但移动设计不得当一般客户端可写入口 |
| G10 测试门禁 | test:shared 不含 packages/domain/src/mobile/*.test.ts（2 个测试文件存在但不被跑）；typecheck:shared 不含 domain/mobile；apps/mobile `"test":"vitest"` 但**无 vitest 配置**，且 7 个测试文件是 node:test 风格与 vitest 不兼容 | 存在 16 个移动侧测试文件（见审计清单） | 门禁 C | tests/mobile-v2 门禁 + 逐任务记录发现数 | MX-002（package 脚本单写） | 修复后各 runner 发现数记录 |
| G11 台账分裂 | docs/superpowers/plans 的 W 索引**从未更新**（W01-W37 全 pending）；真实执行在 .superpowers/sdd/2026-09-12-mobile-ai-saas-workbench/progress.md（W01-W07/W09/W17-W21 有终审通过记录，W08/W10 最后处于修复循环，W13/W25/W34 code-ready，W36/W37 未派发）；open-connector T01-T18 全 passed 但 T18 provider_write/billing blocked-env；main 已含集成代码 | — | 台账互相矛盾 | 本目录为 MX 唯一事实源；不重置 W 历史 | MX-001（本次）+ MX-032/MX-036（能力门禁） | — |
| G12 聚合接口 | workbench 路由**无**列表 GET、首页聚合 GET、资源聚合 GET（routes_workbench.go 全表：Start/Lookup/Get/Snapshot/SSE/source-events/interactions list+decide/commands/execution-targets/workspace）；request lookup + snapshot 已有。Web 的 `GET /api/v1/sessions` 属 chat 域，非 workbench 执行列表 | 无 | A（lookup/snapshot）；B（聚合） | 薄聚合读模型 + 分页列表 | MX-013 | 聚合接口契约→服务→路由→DI→SDK→页面全链接线证据 |

## 接口 A/B/C 分类

**A（当前源码确认）**：`POST /api/v1/workbench/executions`；`GET …/executions/requests/:request_id`（200-with-unknown）；`GET …/executions/:run_id`；`GET …/executions/:run_id/snapshot`；`GET …/executions/:run_id/events`（SSE，payload-only 帧）；`POST …/executions/:run_id/source-events`；`GET …/executions/:run_id/interactions`；`POST …/executions/interactions/:id/decisions`；`POST …/executions/:run_id/commands`（cancel/steer）；`GET|POST /execution-targets`、`GET|DELETE /execution-targets/:id`；`GET /execution-workspaces/:id`；`POST /auth/mobile/exchange`；chat 域 `GET /api/v1/sessions` 等（Web 已消费）。

**B（本轮新增提案，服务端接通前不得发布）**：`GET /workbench/overview` 首页聚合（进行中执行/待审批/通知/用量摘要）→ MX-013；`GET /workbench/executions` 分页列表 → MX-013/014；通知注册+收件箱 → MX-021；服务端能力清单 → MX-032。

**C（原计划要求待核验）**：会话准备接口（Start 前附件/知识版本化扩展）→ MX-025；用量只读视图 → MX-031（先核对 billing 域现有端点再决定复用或新增）。

## DI 与迁移基线

- workbench DI：6 个 dig 类型 Provider（internal/container/workbench.go:15-56 + container.go:189-194）；admission 的 publish 为 no-op（依赖 worker 扫描）、budget 固定 NoopTaskBudget——MX-013/032 扩展时按此现状接线。
- 迁移头：PostgreSQL `migrations/versioned` 最大 **000135**（workbench_requests）；SQLite `migrations/sqlite` 最大 **000057**（execution_targets）。下一可用：PG 000136 / SQLite 000058。
