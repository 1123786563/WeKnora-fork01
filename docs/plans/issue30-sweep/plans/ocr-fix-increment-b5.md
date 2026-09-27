# OCR 增量修复批次 5（issue30-sweep）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 OCR 第五批增量发现（B5 报告，39 项主发现 + 34 项 lowWorth 顺手项）的全部 39 项主发现，按根因聚合为 23 个任务（同根因合并、同文件串行），lowWorth 34 项按文件/域就近折叠为所在任务的顺手项，不单独开流。

**Architecture:** 按**根因**聚合成 23 个任务，横跨 Go 侧（迁移装载、workbench research store/handler 契约、appconnector publish 家族、Confluence/飞书/GitLab 适配器契约）与 TS 侧（voice-room 状态机与音频清理、research 草稿与错误映射、iOS 验收门与证据 fail-closed）。两条大流文件集互不重叠，可并行；流内同文件任务串行（行号以各任务 Step 前实读为准）。所有修复维持既有架构约束：诚实回执（不冒充、不静默）、fail-closed 验收门语义、golang-migrate 单调版本序列、mobile-core 不依赖 RN/DOM。

**Tech Stack:** Go（`internal/`，标准 go test）；TypeScript（`packages/mobile-core`、`packages/api-client`、`apps/mobile`，node:test + tsx）。测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行。本计划作者已实跑下列基线（2026-09-27，当前 HEAD 2d811d6e4）：

- `go build ./...` → **exit 0（绿）**。
- `go test ./internal/modules/appconnector/ ./internal/modules/codedelivery/` → **ok / ok（双包全绿）**。
- `go test ./internal/handler/` → **FAIL：16 个用例失败，16/16 个错误均为 `duplicate migration file: 000119_space_connection_grants.down.sql`**——即 F53 的直接受害者（作者实跑确认错误文本）。
- `go test ./internal/application/service/` → **FAIL**（失败含 `TestAgentForkLineageMigrationProvidesColumnsAndLicenseTable`、`TestAgentAdoptionService*`、`TestRegisterLicenseValidationAndList` 等，作者抽查两个用例的错误文本均为同一 duplicate migration 根因）。
- `pnpm exec tsx --test packages/mobile-core/src/voice-room/voice-room.test.ts packages/mobile-core/src/research/task-research.test.ts apps/mobile/src/research-view.test.ts apps/mobile/src/voice-room-integration-smoke.test.ts apps/mobile/src/ios-core-workflow-integration-smoke.test.ts apps/mobile/src/ios-release-evidence.test.ts` → **36 tests / 34 pass / 0 fail / 2 skipped**。

**Spec:**
- 发现来源：B5 增量发现清单（本计划的 ask 材料；编号 B5-F*，全部标注「已验证」；本计划作者按当前 HEAD 对 39 项主发现的关键锚点逐一复核——行号以复核为准，差异见「差异记录」）
- 报告原文：`docs/plans/issue30-sweep/ocr/ocr-increment-b5.md`（78 findings / 101 items）
- 领域术语与不变量：`CONTEXT.md`（:339「原始音频默认在实时处理后删除」为 F26 的违反对象）
- 前序批次：`docs/plans/issue30-sweep/plans/ocr-fix-increment-b4.md`（沿用其纪律与验收口径：lowWorth 就近折叠、失败集合不扩大）
- Parent：Issue #30（issue30-sweep）

## Global Constraints

以下为批准 Spec / ADR / 报告隐含的项目级约束，所有任务隐含遵守：

- **迁移版本号单调且不双占**：golang-migrate file source 对重复版本号 Initialize 即报错（`internal/database/migration.go:115/:120/:439` 以 `file://migrations/{versioned,sqlite}` 装载；e2e 测试直接 `migrate.NewWithDatabaseInstance("file://...migrations/sqlite", ...)`）。任何新迁移必须取实际空闲号——**sqlite 树顺延目标是 000123，不是报告建议的 000120**（000120-000122 已被 agent_upgrade_proposals/task_research/agent_fork_lineage 占用，作者 ls 实核）。
- **诚实回执/不冒充**：所有验收证据与错误映射不得编造来源或成功（F8/F10/F19/F23/F39 同族）；「服务端确定性 4xx」不得译为「暂时不可用请重试」（F47）。
- **fail-closed 门不被架空**：验收门的检查必须真实触发（F15 的 test -s 守卫、F38 的 disposition 枚举校验、F8 的 source 显式声明）。
- **值传递 store 不得回显未落库状态**：GORM 时间戳只落被 Create 的指针目标（F74/F75）。
- **mobile-core 不依赖 RN/DOM/具体传输**（B4 约束沿用）：T11-T15 对 mobile-core 的修改不引入平台依赖。
- **上下文.md:339 隐私不变量**：原始音频默认在实时处理后删除（F26）。
- **Mimosa 安全约束**（会话注入）：服务端发请求仅 http/https、外部输入一律参数绑定、凭据只从环境变量读取——本批次 Go 修复不得引入字符串拼接 SQL（GORM 参数绑定既有惯例维持）。
- 严格 RED→GREEN→REFACTOR：每个任务先写失败测试、实跑确认失败、最小实现、通过、提交；实现与已批准 Spec 冲突时升级处理。
- 失败集合不扩大：预存在失败（Go 侧因 F53 双占而红的 handler/service 用例在 Task 1 修复后应转绿；TS 侧 2 skipped 维持）不得因本批次新增。

## Review Focus

报告隐含但任务测试需钉住、最可能咬到真实用户的五类失效模式（每行后标注 owning 任务）：

1. **迁移装载失败使全部部署与测试停摆**——任何版本号双占都让两轨迁移流无法到达 head；新迁移必须取实核空闲号且重跑 e2e 全绿。——Task 1 测试（16 个受害用例转绿即最强验收）。
2. **已批准的外部写入计划被错误终态化**——跨家族 action 送错端点会被消费审批后 settle failed，受害管线幂等闭环永久破坏；修复后必须拒绝而非销毁。——Task 5 测试（Confluence/飞书双家族交叉拒收）。
3. **静默数据损坏**——HTTP 响应被 LimitReader 截断后以不完整字节返回（GitLab >8MiB / Confluence >1MB）、GitLab 基线锚定错误把默认分支增量生成回卷动作，MR 合并即真实丢代码。——Task 7/8 测试（截断检测必须报错而非返回；分支不存在时不得生成回卷）。
4. **设备磁盘隐私残留与跨进程数据覆盖**——每轮语音转写留存一个音频文件（违反 CONTEXT.md:339）；research 草稿冷启动后同 id 静默覆盖上一会话批注。——Task 11/14 测试（转写后清理端口被调用；两次进程的 draftId 必不相同）。
5. **验收门假阴性**——崩溃筛查 grep 模式不全、空证据文件放行、evidence source 被默认值兜底、disposition 拼错被当 blocked-env 通过：崩溃/未验收状态可被静默放行。——Task 17/18 测试（SIGABRT 命中即失败；空 push-process-alive.txt 即失败；缺 source 即 gap；非法 disposition 即 gap）。

---

## 任务结构与文件地图

| # | 任务 | 根因分组（发现编号） | 主要文件 | 优先级 |
|---|---|---|---|---|
| 1 | 迁移版本号双占修复 | B5-F53；顺手 F56 | `migrations/versioned/`、`migrations/sqlite/`、`internal/modules/appconnector/repository/appconnector/space_grant.go` | **critical** |
| 2 | workbench research：store 值传递零值时间戳 | B5-F74 + B5-F75（同根因） | `internal/application/repository/task_research.go`、`internal/handler/session/workbench_research.go` | **high** |
| 3 | workbench research：store 错误分类 | B5-F63 + B5-F76（同根因）；顺手 F58/F68 | `internal/handler/session/workbench_research.go` | **high** |
| 4 | workbench research：输入长度边界 | B5-F64；顺手 F67/F65/F66 | `internal/handler/session/workbench_research.go` | medium |
| 5 | publish 家族归属校验与哨兵贯通 | B5-F42 + B5-F62 + B5-F78（同根因：家族谓词缺失/跨包哨兵断链）；顺手 F57/F69 | `internal/handler/app_connector_{confluence,feishu,notion}_publish.go`、`internal/modules/appconnector/publish/provider.go` | **high** |
| 6 | Confluence 请求/对账契约 | B5-F43 + B5-F71 | `internal/modules/appconnector/confluence_create.go`、`confluence_update.go` | medium |
| 7 | HTTP 响应截断检测（跨模块同根因） | B5-F72 + B5-F44 | `internal/modules/codedelivery/gitlab_client.go`、`internal/modules/appconnector/confluence_create.go` | **high** |
| 8 | GitLab EnsureBranch 锚定与错误语义 | B5-F52 + B5-F73 | `internal/modules/codedelivery/gitlab_client.go` | **high** |
| 9 | 飞书 docx 请求契约 | B5-F60 + B5-F61 | `internal/modules/appconnector/feishu_docx.go` | medium |
| 10 | 飞书多批次续传路径 | B5-F77 | `internal/modules/appconnector/feishu_docx.go`、`internal/modules/appconnector/service/appconnector/action.go` | **high** |
| 11 | voice-room：音频清理端口 | B5-F26；顺手 F35 | `packages/mobile-core/src/voice-room/voice-room.ts`、`apps/mobile/src/composition.ts` | **high** |
| 12 | voice-room：状态机守卫 | B5-F27 + B5-F28（同根因：世代/守卫）；顺手 F51 | `packages/mobile-core/src/voice-room/voice-room.ts` | medium |
| 13 | voice-room 冒烟顺序 | B5-F50；顺手 F32/F36 | `apps/mobile/src/voice-room-integration-smoke.ts` | **high**(test) |
| 14 | research：draftId 进程间唯一 | B5-F45 | `packages/mobile-core/src/research/task-research.ts` | **high** |
| 15 | research：错误映射纪律 | B5-F46 + B5-F47（同根因）；顺手 F17/F18/F23/F25 | `packages/mobile-core/src/research/task-research.ts`、`packages/api-client/src/mobile/research.ts`、`apps/mobile/src/composition.ts` | medium |
| 16 | research：UI 如实呈现 | B5-F19；顺手 F48/F49/F20/F3 | `apps/mobile/src/research-view.ts`、`apps/mobile/src/app/tasks/research.tsx`、`apps/mobile/src/screens/MaterialsScreen.tsx` | medium |
| 17 | iOS 验收脚本门收紧 | B5-F1 + B5-F13 + B5-F14 + B5-F15（同文件四项）；顺手 F2/F16 | `apps/mobile/scripts/ios-acceptance-run.sh`、`ios-release-build.sh` | medium |
| 18 | iOS release evidence fail-closed | B5-F8 + B5-F38（同根因）；顺手 F9/F11/F12/F41 | `apps/mobile/src/ios-release-evidence.ts`、`ios-release-evidence-cli.ts` | medium |
| 19 | iOS 冒烟证据保真 | B5-F6 + B5-F7（同文件）；顺手 F10/F34/F39/F40 | `apps/mobile/src/ios-core-workflow-integration-smoke.ts` | medium |
| 20 | agent 治理数据准确性 | B5-F37；顺手 F5 | `internal/application/service/agent_fork_lineage.go`、`internal/application/repository/agent_marketplace_lineage.go` | medium |
| 21 | FormActionPlan 输入边界 | B5-F54；顺手 F55/F70 | `internal/handler/app_connector_action_plan.go`、`internal/modules/appconnector/plan/plan.go` | medium |
| 22 | voice 深链 runId 与句柄生命周期 | B5-F29；顺手 F31/F21 | `apps/mobile/src/app/tasks/voice.tsx` | medium |
| 23 | api-client 503 翻译 | B5-F30 | `packages/api-client/src/mobile/voice-sessions.ts` | medium |

**发现覆盖对照**（ask 材料 39 项主发现逐项对账）：

| 发现 | 任务 | 处置 |
|---|---|---|
| F53（critical） | Task 1 | 修复（sqlite→000123、versioned→000199，实核空闲号） |
| F26（high） | Task 11 | 修复 |
| F42（high） | Task 5 | 修复（与 F62/F78 合并） |
| F45（high） | Task 14 | 修复 |
| F50（high） | Task 13 | 修复 |
| F52（high） | Task 8 | 修复 |
| F72（high） | Task 7 | 修复（与 F44 跨模块合并） |
| F74（high） | Task 2 | 修复（与 F75 同根因合并） |
| F75（high） | Task 2 | 修复 |
| F77（high） | Task 10 | 修复 |
| F1/F13/F14/F15 | Task 17 | 修复（同文件合并） |
| F6/F7 | Task 19 | 修复（同文件合并） |
| F8/F38 | Task 18 | 修复（同根因合并） |
| F19 | Task 16 | 修复 |
| F20 | Task 16 | 顺手项 |
| F27/F28 | Task 12 | 修复（同根因合并） |
| F29 | Task 22 | 修复 |
| F30 | Task 23 | 修复 |
| F37 | Task 20 | 修复 |
| F43/F71 | Task 6 | 修复（同适配器家族合并） |
| F44 | Task 7 | 修复 |
| F46/F47 | Task 15 | 修复（同根因合并） |
| F54 | Task 21 | 修复 |
| F60/F61 | Task 9 | 修复（同文件合并） |
| F62 | Task 5 | 修复 |
| F63/F76 | Task 3 | 修复（同根因合并） |
| F64 | Task 4 | 修复 |
| F71 | Task 6 | 修复 |
| F73 | Task 8 | 修复 |
| F78 | Task 5 | 修复 |

**lowWorth 34 项就近折叠对照**：

| lowWorth | 折叠到 | 一句话处置 |
|---|---|---|
| F56（space_grant Save upsert 覆写 created_at/gr） | Task 1 | 同文件 space_grant.go：upsert 改列限定 |
| F58（AuthorizeResearchSource DB 故障误标授权拒绝） | Task 3 | 同文件：错误分类一并修 |
| F68（writeResearchError 未映射 ResolveTaskAccess 类别） | Task 3 | 同文件：NotFound→404 / BadRequest→400 / 其余→500 |
| F67（ListResearch/ListAnnotations 无分页） | Task 4 | 同根因输入/输出边界：补 limit/offset |
| F65（空 body 400 顺序在 404/409 后） | Task 4 | 校验顺序前移 |
| F66（researchDelegateInput.AgentID 绑定未读） | Task 4 | 移除死字段或落库（按契约注释定夺，见任务内说明） |
| F57（发布服务引用飞书专属哨兵） | Task 5 | 同文件 provider.go：哨兵中立项 |
| F69（unsupported_provider %v 包装断链） | Task 5 | 错误包装改 %w 保 errors.Is |
| F5（UpsertLicense created_at 虚报/created_by 覆写） | Task 20 | 同域 agent 治理：DoUpdates 收窄 + 回读 |
| F55（planByID DB 故障→404） | Task 21 | 同文件：错误分类 |
| F70（恢复性重批覆写 approved_by/approved_at） | Task 21 | plan.go:281 附近：冻结首批批人 |
| F17/F18（draftsPort put/list/remove 错误形态不一） | Task 15 | 同根因：统一 ResearchError |
| F25（annotateFailure 404 折叠） | Task 15 | api-client 同文件：404 区分 |
| F23（delegation-rejected-by-scope-fence 前缀失真） | Task 15 | 证据前缀按真实失败原因 |
| F48（三元 else 不可达） | Task 16 | receipt.outcome 契约对齐 |
| F49（'no-materials' 从未赋值） | Task 16 | 联合类型收敛 |
| F3（MaterialsScreen Text onPress 无障碍） | Task 16 | 同 research 入口域：改 Button |
| F35（maxSeconds 600 硬编码） | Task 11 | 同文件：引用 mobile-core 常量 |
| F51（phase==='ended' 无离开出口） | Task 12 | 同文件：ended 态保留离开按钮 |
| F32（探测会话未 end()） | Task 13 | 同文件：探测后 end |
| F36（注释「如实上抛」与 return 不符） | Task 13 | 同文件：注释如实化 |
| F2（babel-preset-expo ln -s 无 -f） | Task 17 | 同 iOS 脚本域：ln -sfn |
| F16（scripts/ TS 文件不在 typecheck） | Task 17 | 纳入 typecheck 覆盖 |
| F9/F11/F12/F41（CLI 健壮性四项） | Task 18 | 同文件域：JSON 防崩/根推导/未知 subject/顶层形状 |
| F10（外层 catch 污染 signIn） | Task 19 | 同文件：仅未成功才置 failed |
| F21（runId 缺参与未登录共用文案） | Task 22 | 同文件：缺 runId 文案点名真实原因 |
| F31（effect catch 句柄泄漏） | Task 22 | 同文件：构造抛错路径补 close |
| F34/F39/F40（冒烟部署校验复制/证据面） | Task 19 | 冒烟家族域：共享 helper 收敛 + 证据面补断言 |

**执行顺序**：Task 1 必须最先（Go 测试基线因 F53 双占而红，T1 即解锁）。此后 Go 流（2→3→4 同文件串行；5；6→7 同文件串行；7→8 同文件串行；9→10 同文件串行；20、21 独立）与 TS 流（11→12 同文件串行；13；14；15；16；17；18；19；22；23）文件集互不重叠可并行。线性执行按编号顺序即可。

## 差异记录（报告/ask 材料 vs 代码现状，以代码现状为准）

1. **ask F53 的装载点行号有偏**：`migration.go:140/379` 实际为 `internal/database/migration.go:115`（versioned）与 `:439`（ResetMigrations 类路径），sqlite 分支在 `:120`。核心事实（file source 双版本号装载即败）不变，且作者实跑 `go test ./internal/handler/` 拿到 16/16 用例失败、错误文本 `duplicate migration file: 000119_space_connection_grants.down.sql`——F53 的「两轨迁移流均无法到达 head」在本 worktree 有直接测试证据。
2. **ask F77 的 `action.go:68 ocDispatchClientDeadline = 30 * time.Second` 不存在**：grep 全 appconnector 包无该标识符；实际 30s 超时是 `internal/modules/appconnector/publish/dispatcher.go:331` 与 `publish/confluence_bridge.go:302` 的 `http.Client{Timeout: 30 * time.Second}`。F77 的核心证据链成立：Execute 状态门在 `internal/modules/appconnector/service/appconnector/action.go:329`（`if row.State != appconn.ActionAuthorized`，ask 写的「action.go:329-330」行号吻合但包路径是 service/appconnector 子包），`dispatcher.Dispatch` 全仓库仅 `action.go:446` 一处。
3. **F72 之外 gitlab_client.go:96 还有同款 `io.LimitReader(resp.Body, 4<<20)`**（callJSON 类路径）：Task 7 将 :96 与 :128 一并加截断检测，同根因同修。
4. **F66 的 AgentID**：`workbench_research.go:168` `researchDelegateInput.AgentID` 绑定后 handler 未读取。归为 Task 4 顺手项；实现时若领域上确需委派目标 agent，升级为显式需求而非静默落库（本计划按「移除死绑定」书写，执行者发现反证则停下升级）。
5. **F53 报告原文建议 sqlite 顺延至 000120 有误**（ask 已指出），作者实核：sqlite 000120-000122 已被 agent_upgrade_proposals/task_research/agent_fork_lineage 占用，**000123 空闲**；versioned 侧 000199 空闲。Task 1 按实核空闲号书写。
6. **TS 基线 2 skipped** 为既有跳过（voice-room-integration-smoke 的真机/部署分支），本批次不改动其跳过语义。

---

### Task 1: 迁移版本号双占修复（B5-F53 critical；顺手 B5-F56）

**Files:**
- Rename: `migrations/versioned/000198_space_connection_grants.{up,down}.sql` → `000199_space_connection_grants.{up,down}.sql`
- Rename: `migrations/sqlite/000119_space_connection_grants.{up,down}.sql` → `000123_space_connection_grants.{up,down}.sql`
- Modify: `internal/modules/appconnector/repository/appconnector/space_grant.go`（顺手 F56：GrantSpaceConnection 的 gorm `Save` upsert 覆写 created_at/created_by）

**Interfaces:**
- Consumes: golang-migrate file source 装载（`internal/database/migration.go:115/:120/:439`；e2e 测试 `migrate.NewWithDatabaseInstance("file://...migrations/sqlite", ...)`）。
- Produces: 两轨迁移树版本号无双占；`go test ./internal/handler/` 与 `./internal/application/service/` 从 duplicate-migration 失败恢复可跑。F56：`GrantSpaceConnection` 重复授权不再覆写 `created_at`/`created_by`（用 `clause.OnConflict{DoUpdates: name/scope/updated_at 类列}` 或 `Select` 限定列，维持既有列语义）。

**根因与修复说明：** versioned 树 000198 被 app_action_plans 与 space_connection_grants 双占（000199 空闲），sqlite 树 000119 双占（000120-000122 已占、000123 空闲）——golang-migrate file source 对重复版本号 Initialize 即错，两轨迁移流均无法到达 head，直接打挂 handler 包 16 个用例与 service 包一簇用例（作者实跑基线为证）。修复即两个文件重命名取空闲号（内容不变，space_connection_grants 尚未发布到任何环境——issue30-sweep 未合流，无已部署库需要 down 迁移）。顺手 F56：同迁移所属的 store `space_grant.go` GrantSpaceConnection 用 `Save` 全量覆写，重复授权会把 created_at/created_by 抹成最新操作者——与 Task 20 F5 同型的审计字段的保护，本任务一并改为列限定 upsert。

- [ ] **Step 1: 写失败测试（Go 侧以「现有红测试」为 RED——先实跑确认当前失败形态）**

Run: `go test ./internal/handler/ -run TestAppActionPlansTablesExistAfterMigrations 2>&1 | grep -c "duplicate migration file"`
Expected: `1`（RED 已天然存在：duplicate migration file: 000119_space_connection_grants.down.sql）。

再为 F56 在 space_grant 既有测试文件（同目录 `space_grant_test.go`，若无则新建）追加：

```go
func TestGrantSpaceConnectionKeepsFirstCreatorOnRegrant(t *testing.T) {
	// arrange: sqlite 内存库跑 migrations/sqlite 全量迁移；插入一条 first 创建的 grant 行
	// act: 以不同 operator 再次 GrantSpaceConnection（同 tenant/space/key）
	// assert: row.CreatedAt 等于首插时间（非本次 now）；row.CreatedBy 等于首插操作者；Updated* 允许前进
}
```

- [ ] **Step 2: 实跑确认 F56 测试失败**

Run: `go test ./internal/modules/appconnector/repository/appconnector/ -run TestGrantSpaceConnectionKeepsFirstCreatorOnRegrant`
Expected: FAIL（当前 Save 覆写 created_at/created_by）。

- [ ] **Step 3: 最小实现**

```bash
git mv migrations/versioned/000198_space_connection_grants.up.sql migrations/versioned/000199_space_connection_grants.up.sql
git mv migrations/versioned/000198_space_connection_grants.down.sql migrations/versioned/000199_space_connection_grants.down.sql
git mv migrations/sqlite/000119_space_connection_grants.up.sql migrations/sqlite/000123_space_connection_grants.up.sql
git mv migrations/sqlite/000119_space_connection_grants.down.sql migrations/sqlite/000123_space_connection_grants.down.sql
```

`space_grant.go` GrantSpaceConnection：`Save` 改为 `clause.OnConflict{Columns: 冲突键, DoUpdates: AssignmentColumns([]string{"name", "updated_at" /* 既有可变列 */})}` 式列限定（列集合以实体注释/表结构实读为准，执行者确认后落定；created_at/created_by 不在 DoUpdates）。

- [ ] **Step 4: 实跑确认通过 + 全量回归（本任务的最强验收：受害用例转绿）**

Run: `go test ./internal/modules/appconnector/repository/appconnector/ -run TestGrantSpaceConnectionKeepsFirstCreatorOnRegrant`
Expected: PASS。
Run: `go test ./internal/handler/`
Expected: **PASS（基线 16 个 duplicate-migration 失败全部转绿）**。
Run: `go test ./internal/application/service/`
Expected: PASS（或仅剩与迁移无关的预存在失败——若有，逐一确认非本批次引入并记录）。

- [ ] **Step 5: Commit**

```bash
git add migrations/ internal/modules/appconnector/repository/appconnector/
git commit -m "fix(migrations): dedupe 000198/000119 double-booking to 000199/000123 (B5-F53); keep first creator on regrant (B5-F56)"
```

---

### Task 2: workbench research：store 值传递零值时间戳（B5-F74 + B5-F75）

**Files:**
- Modify: `internal/application/repository/task_research.go:32`（CreateDelegation 值参→指针）
- Modify: `internal/application/repository/task_research.go:110`（CreateAnnotation 值参→指针）
- Modify: `internal/handler/session/workbench_research.go:214`（&delegation）、`:340`（&annotation）
- Test: `internal/handler/session/workbench_research_test.go`（若无对应用例区则追加）

**Interfaces:**
- Consumes: `TaskResearchStore.CreateDelegation(ctx, d types.TaskResearchDelegation) error` 与 `TaskAnnotationStore.CreateAnnotation(ctx, a types.TaskArtifactAnnotation) error`（现值传递，GORM `Create(&d)` 只落局部拷贝）；handler 侧 `delegationViewOf`（:130，`d.CreatedAt.UTC().Format(time.RFC3339)`）与 `annotationViewOf`（:153）。
- Produces: `CreateDelegation(ctx context.Context, d *types.TaskResearchDelegation) error`、`CreateAnnotation(ctx context.Context, a *types.TaskArtifactAnnotation) error`——GORM 自动时间戳回写调用方实体；201 响应 `created_at` 为真实落库时刻。调用方仅 workbench_research.go 两处（grep 确认后执行者再核）。

**根因与修复说明：** handler 构造 delegation/annotation 后**值**传给 store，`s.db.Create(&d)` 的时间戳写在 store 的局部拷贝上；`:218`/`:344` 的 201 回显用 handler 本地零值实体 → `created_at` 恒 `0001-01-01T00:00:00Z`。改指针签名是最小修复（GORM 语义：Create 回写传入指针目标）。

- [ ] **Step 1: 写失败测试**

```go
func TestDelegateResearchReturnsPersistedCreatedAt(t *testing.T) {
	// arrange: sqlite 内存库 + 全量迁移 + 认证用户 + owned run（复用同文件既有夹具）
	// act: POST /api/v1/workbench/executions/:run/research/delegate（合法 objective+sources）
	// assert: resp 201, data.delegation.created_at 解析后距今 < 5s（绝非 "0001-01-01T00:00:00Z"）
}
func TestAnnotateMaterialReturnsPersistedCreatedAt(t *testing.T) { /* 同型：POST .../annotations */ }
```

- [ ] **Step 2: 实跑确认失败**

Run: `go test ./internal/handler/ -run 'TestDelegateResearchReturnsPersistedCreatedAt|TestAnnotateMaterialReturnsPersistedCreatedAt'`
Expected: FAIL ×2——created_at 为零值。

- [ ] **Step 3: 最小实现**

签名改指针（上 Interfaces），`s.db.WithContext(ctx).Create(d)`（去 &）；handler `:214`/`:340` 传 `&delegation`/`&annotation`。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/handler/ -run 'Research|Annotate'`
Expected: PASS（含既有 research 用例——语义仅回显字段变化）。

- [ ] **Step 5: Commit**

```bash
git add internal/application/repository/task_research.go internal/handler/session/workbench_research.go internal/handler/session/workbench_research_test.go
git commit -m "fix(research): return persisted created_at from delegate/annotate 201 responses (B5-F74, B5-F75)"
```

---

### Task 3: workbench research：store 错误分类（B5-F63 + B5-F76；顺手 B5-F58/F68）

**Files:**
- Modify: `internal/handler/session/workbench_research.go:106`（resolveReadable：GetOwnedRun 错误分类）
- Modify: `internal/handler/session/workbench_research.go:259`（CompleteResearch：GetDelegation 错误分类）
- Modify: `internal/handler/session/workbench_research.go:203`（顺手 F58：AuthorizeResearchSource 错误分类）
- Modify: workbench_research.go 内 `writeResearchError`（顺手 F68：映射 ResolveTaskAccess 的 NotFound/BadRequest）

**Interfaces:**
- Consumes: 既有契约先例 `workbench_read.go:206-234` `resolveReadableRun`——区分 `ErrNotFound`（→404）与其余错误（→500）；store 契约 `GetDelegation` 仅 `gorm.ErrRecordNotFound` → `types.ErrTaskResearchNotFound`（task_research.go:38-44），其余为基础设施错误。
- Produces: `resolveReadable`/`CompleteResearch`/`AuthorizeResearchSource` 三处：`errors.Is(err, <NotFound 哨兵>)` → 404 兜底；其余 `err != nil` → 500（`writeResearchError` 或等价 helper，含底层错误日志）。F68：`writeResearchError` 对 `ResolveTaskAccess` 的 NotFound→404 / BadRequest→400 / 其余→500。

**根因与修复说明：** 三处读路径把**任意** store 错误（含 DB 故障）当 miss：resolveReadable 落入 granted 兜底再统一 404 `run_not_found`（:106-114）；CompleteResearch 一律 404 `research_not_found`（:259-262）；F58 的 AuthorizeResearchSource 把 DB 故障误标为授权拒绝。DB 故障期间三个读端点全报 404，掩盖基础设施故障，与 `workbench_read.go` 既有契约（NotFound→404、存储故障→500）相悖。

- [ ] **Step 1: 写失败测试**

```go
func TestResearchReadsReturn500OnStoreFailure(t *testing.T) {
	// arrange: 注入 GetOwnedRun/GetDelegation 返回 fmt.Errorf("db down")（非 NotFound 哨兵）
	// act+assert: GET research 列表 / POST complete /（F58）delegate 触发 AuthorizeResearchSource 失败
	//   → 三处均 500 且不出现 run_not_found/research_not_found 的 404 语义
}
```

- [ ] **Step 2: 实跑确认失败**

Run: `go test ./internal/handler/ -run TestResearchReadsReturn500OnStoreFailure`
Expected: FAIL——当前统一 404。

- [ ] **Step 3: 最小实现**

三处按 Interfaces 的 Produces 改写（模式照抄 workbench_read.go 的分类先例；F58 的 AuthorizeResearchSource 需 store 层先提供可区分的错误——若当前实现只返回 bool/error 单态，为其补 `ErrSourceNotFound` 哨兵或让 DB 错误原样上抛，执行者按现有 store 签名择最小改动）。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/handler/ -run 'Research|Annotate|Workbench'`
Expected: PASS（含 Task 2 新增用例）。

- [ ] **Step 5: Commit**

```bash
git add internal/handler/session/workbench_research.go internal/handler/session/workbench_research_test.go
git commit -m "fix(research): classify store failures as 500, not 404, across research reads (B5-F63, B5-F76; F58/F68)"
```

---

### Task 4: workbench research：输入长度边界（B5-F64；顺手 B5-F67/F65/F66）

**Files:**
- Modify: `internal/handler/session/workbench_research.go:58-60`（常量区补 maxResearchSummaryRunes / maxAnnotationBodyRunes）
- Modify: `internal/handler/session/workbench_research.go`（AnnotateMaterial body 校验、CompleteResearch summary 校验、校验顺序、List 分页）
- Modify: `internal/handler/session/workbench_research.go:168`（F66：AgentID 死绑定移除）

**Interfaces:**
- Consumes: 既有上限先例 `maxResearchSources = 8`、`maxResearchObjectiveRunes = 2000`（:58-59）；批量上限先例 `tenant.go` MaxItems≤2000。
- Produces: `maxResearchSummaryRunes = 2000`、`maxAnnotationBodyRunes = 8000`（与 objective 对称的量级；DDL 为 TEXT，上限防滥用与响应放大）；F67：`ListResearch`/`ListAnnotations` 支持 `?limit=&cursor=`（缺省 limit 50、上限 200，cursor 为 id 游标，响应附 `next_cursor`）；F65：空 body 的 400 校验置于 404（material 匹配）与 409（版本冲突）之前；F66：`researchDelegateInput` 移除 `AgentID` 字段（差异记录 4）。

**根因与修复说明：** objective 限 2000 runes 而 summary 与 annotation body 完全无上限（DDL TEXT），认证用户单请求可写入任意大小文本且列表全量回显——存储滥用与响应放大向量；F67 的 append-only 无分页全量返回使响应线性膨胀。输入格式错误（空 body）排在业务状态错误（404/409）之后违反校验序惯例。

- [ ] **Step 1: 写失败测试**

```go
func TestResearchInputBounds(t *testing.T) {
	// 1) annotation body 超上限（8001 runes）→ 400（且先于 404/409 判定：用不存在的 material id + 超长 body 断言 400 而非 404）
	// 2) complete summary 超上限 → 400
	// 3) 预插 3 条 annotation，GET ?limit=2 → 返回 2 条 + next_cursor；再按 cursor 取余 1 条
	// 4) F66：POST delegate 请求体携带 agent_id 字段——行为不报错但契约中已无该字段（绑定结构体删除后多余键被忽略，响应不含 agent_id）
}
```

- [ ] **Step 2: 实跑确认失败**

Run: `go test ./internal/handler/ -run TestResearchInputBounds`
Expected: FAIL——超长 body/summary 被接受；无分页；顺序错。

- [ ] **Step 3: 最小实现**

按 Interfaces Produces 落地（分页用 id > cursor 的键集分页，参数绑定走 GORM 参数化，杜绝拼接）。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/handler/ -run 'Research|Annotate'`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/handler/session/workbench_research.go internal/handler/session/workbench_research_test.go
git commit -m "fix(research): bound summary/annotation length, paginate lists, fix validation order, drop dead AgentID (B5-F64; F65/F66/F67)"
```

---

### Task 5: publish 家族归属校验与哨兵贯通（B5-F42 + B5-F62 + B5-F78；顺手 B5-F57/F69）

**Files:**
- Modify: `internal/handler/app_connector_confluence_publish.go:60`（confluenceActionByID 补家族谓词）
- Modify: `internal/handler/app_connector_feishu_publish.go:55`（feishuActionByID 补家族谓词）
- Modify: `internal/modules/appconnector/publish/provider.go:69`（F78：BlocksOf 哨兵同包化；F57：服务体不引飞书专属哨兵）
- Modify: publish dispatcher/handler 错误包装处（F69：%v → %w）

**Interfaces:**
- Consumes: 三管线共享同一 ActionStore 各建专属 ActionService（`internal/container/{confluence,feishu,notion}_publish.go:16/:16/:24` 注释为证）；`service/appconnector/action.go:329` Execute 状态门、`:446` Dispatch；handler `failPublish` 只匹配 publish 包哨兵。
- Produces: `confluenceActionByID`/`feishuActionByID` 在 tenant+id 之上增加 `row.AppID == "confluence"` / `== "feishu"` 谓词（字段名以 ActionRow 实体实读为准），跨家族 id 返回 not-found 语义（404），绝不进入 ClaimDispatch/settle；F78：`FeishuProfile.BlocksOf` 返回 publish 包自身的哨兵（或 handler 匹配处改按 `errors.Is` 可达的错误族——两方案择一，标准是「空内容/超限 400/413 映射活过来、不再落 500」）；F69：Dispatch 面 `%v` 包装改 `%w`。

**根因与修复说明：** 同租户下 Notion/飞书/Confluence 三家族共享 ActionStore，而 confluence/feishu 的 `*ActionByID` 只按 tenant_id+id 解析任意 action 行。受害链（ask 已验证）：Execute 状态门通过 → ClaimDispatch 消费审批（authorized→dispatched）→ `adapterFor` 因 AppID 不匹配返回 ErrDispatchNotStarted → settle 终态 Failed——**其它管线已批准的外部写入计划被永久销毁且从未触达其真实 provider**，幂等闭环破坏。F78 同族：`provider.go:69` `BlocksOf: appconn.FeishuTextBlocks` 直接透传 appconnector 包哨兵，与 handler 匹配的 publish 包哨兵是不同实例，`errors.Is` 不成立——空 artifact/超限的 400/413 映射成死分支被上报 500。

- [ ] **Step 1: 写失败测试**

```go
func TestPublishEndpointsRejectCrossFamilyActionIDs(t *testing.T) {
	// arrange: sqlite 迁移库；创建一条 notion 家族 authorized action
	// act: POST /api/v1/app-connectors/confluence/actions/:id/publish（同 id）
	// assert: 404（not-found 语义）；且 notion action 行状态仍为 authorized（审批未被消费、未 settle failed）
	// 同型断言 feishu 端点对 confluence action；GET 回执端点同型拒收
}
func TestFeishuBlocksErrorMapsToClientError(t *testing.T) {
	// arrange: feishu 发布 action 的 artifact 为空 → Dispatch 返回 BlocksOf 哨兵错误
	// assert: handler 响应 400（而非 500）；F69 同断言 unsupported_provider → 400
}
```

- [ ] **Step 2: 实跑确认失败**

Run: `go test ./internal/handler/ -run 'CrossFamily|BlocksErrorMaps'`
Expected: FAIL ×2——跨家族被消费审批后 settle failed；空内容 500。

- [ ] **Step 3: 最小实现**

按 Interfaces Produces 落地（家族谓词注意：notion 端点同型检查若同样缺失则一并补上——执行者 grep `notionActionByID` 确认，同根因同修）。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/handler/ -run 'Publish'`
Expected: PASS（含既有 notion/confluence/feishu e2e 全家——Task 1 后它们已恢复可跑）。
Run: `go test ./internal/modules/appconnector/...`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/handler/ internal/modules/appconnector/publish/
git commit -m "fix(publish): enforce family ownership on action resolution and unify sentinel errors (B5-F42, F62, F78; F57/F69)"
```

---

### Task 6: Confluence 请求/对账契约（B5-F43 + B5-F71）

**Files:**
- Modify: `internal/modules/appconnector/confluence_create.go:88-89`（wire 键名 snake_case → camelCase）
- Modify: `internal/modules/appconnector/confluence_update.go:287`（Query 对账改语义级判定）

**Interfaces:**
- Consumes: Atlassian Confluence Cloud REST v2 `POST /api/v2/pages` 官方契约为 camelCase（spaceId/parentId）；同文件响应解析 `confluencePageWire` 已按 camelCase 解码；Notion 家族语义级判定先例 `notionBlocksContained`。
- Produces: create wire 为 `SpaceID json:"spaceId"` / `ParentID json:"parentId,omitempty"`；Query 成功判定改为语义级——`html.UnescapeString`（或规范化比较器）对 storage 做实体/空白归一后比较，标题 trim 比较；归一仍不等才 `confluence_query_unverifiable`。

**根因与修复说明：** F43：请求体键名 `space_id`/`parent_id` 与官方 camelCase 契约不一致，生产 Cloud 路径首次调用即 400 且判 definitive failed——T20 Cloud 版本实际不可用（fake server 按同构 wire 解码所以测试无感）。F71：Query 以字节精确相等（`cur.BodyStorage != snap.Storage`）判定，而 ConfluenceStorageBody 用 Go `html.EscapeString` 生成 `&#39;`/`&#34;` 实体、服务端序列化器通常回写裸字符——正文含引号时已落地的更新被 park unknown 后 Query 永远 unverifiable，该页面从此不能再经本管线更新。

- [ ] **Step 1: 写失败测试**

```go
func TestConfluenceCreateWireUsesCamelCaseKeys(t *testing.T) {
	// arrange: fake Confluence 服务按官方契约仅认 "spaceId"/"parentId" 键（snake_case 键返回 400）
	// act: execute create
	// assert: 成功；且捕获的请求体 JSON 键为 spaceId/parentId
}
func TestConfluenceQueryAcceptsReserializedStorage(t *testing.T) {
	// arrange: 页面 storage 含引号；fake 服务回读时把 &#39;/&#34; 还原为裸字符（真实服务行为）
	// act+assert: Query 收敛 succeeded（不再 confluence_query_unverifiable）；仍存在真实差异时保持 unverifiable（负例）
}
```

- [ ] **Step 2: 实跑确认失败**

Run: `go test ./internal/modules/appconnector/ -run 'ConfluenceCreateWire|ConfluenceQueryReserialized'`
Expected: FAIL ×2。

- [ ] **Step 3: 最小实现**

按 Interfaces Produces 落地（归一比较器放 confluence_update.go 内私有函数；fake 测试的 wire 校验与生产同构，防「fake 原样回显掩盖差异」再次发生）。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/modules/appconnector/`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/modules/appconnector/confluence_create.go internal/modules/appconnector/confluence_update.go
git commit -m "fix(confluence): camelCase create wire keys and semantic query reconciliation (B5-F43, B5-F71)"
```

---

### Task 7: HTTP 响应截断检测（B5-F72 + B5-F44，跨模块同根因）

**Files:**
- Modify: `internal/modules/codedelivery/gitlab_client.go:96`（4MiB 同款）、`:128`（8MiB callRaw）
- Modify: `internal/modules/appconnector/confluence_create.go:162`（1MB do/callRaw 同型）

**Interfaces:**
- Consumes: 三处均为 `io.ReadAll(io.LimitReader(resp.Body, N))`——`ReadAll` 在超限时成功返回不完整字节，无任何检测即 `*out = raw`。
- Produces: 共享截断检测模式（各包私有 helper 即可，不跨模块引公共库）：读 `N+1` 字节，`len(raw) > N` 即返回 `fmt.Errorf("...response exceeds %d-byte cap: %s %s", N, method, path)` 类**错误**（绝不返回截断字节）。GitLab 侧错误包 `ErrCodeTransport` 家族；Confluence 侧错误使 action 落 unknown 并进入 Query（与既有 unknown 语义一致）。

**根因与修复说明：** 同根因跨模块：上限本意是防失控响应，但静默截断把「防 DoS」变成「静默数据损坏」——GitLab >8MiB 文件以截断内容被提交进 MR（Blob 取回路径无拦截；GitHub 适配器按 blob sha 引用无此问题）；Confluence create/update 回复与 Query 对账读回显整页 storage（EscapeString 放大后可达 ~6MB），超限时已成功的发布永久停 unknown 且对账读截断永远无法收敛。

- [ ] **Step 1: 写失败测试**

```go
// gitlab_client_test.go
func TestGitLabRawRejectsBodyOverCap(t *testing.T) {
	// arrange: httptest 服务返回 9MiB 正文，HTTP 200
	// act: callRaw
	// assert: 返回错误（含 exceeds 字样），绝不返回 9MiB 截断后的 8MiB 字节
}
// confluence_create_test.go（或同包既有 fake 测试文件）
func TestConfluenceDoRejectsBodyOverCap(t *testing.T) {
	// arrange: fake 服务返回 1MB+1 字节，HTTP 200
	// act+assert: 返回错误而非截断 JSON；execute 路径落 ActionUnknown 而非携带半截 output 成功
}
```

- [ ] **Step 2: 实跑确认失败**

Run: `go test ./internal/modules/codedelivery/ -run TestGitLabRawRejectsBodyOverCap` 与 `go test ./internal/modules/appconnector/ -run TestConfluenceDoRejectsBodyOverCap`
Expected: FAIL ×2——当前返回截断字节、无错误。

- [ ] **Step 3: 最小实现**

```go
raw, err := io.ReadAll(io.LimitReader(resp.Body, cap+1))
if err != nil { /* 既有 read 错误分支 */ }
if len(raw) > cap { return nil, fmt.Errorf("%w: %s %s: response exceeds %d-byte cap", ErrCodeTransport, method, path, cap) }
```

（三处同型；常量就地命名 `maxXxxBodyBytes`。）

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/modules/codedelivery/ ./internal/modules/appconnector/`
Expected: PASS（两包基线全绿维持）。

- [ ] **Step 5: Commit**

```bash
git add internal/modules/codedelivery/gitlab_client.go internal/modules/appconnector/confluence_create.go
git commit -m "fix(http): detect cap-exceeding responses instead of silently truncating (B5-F72, B5-F44)"
```

---

### Task 8: GitLab EnsureBranch 锚定与错误语义（B5-F52 + B5-F73）

**Files:**
- Modify: `internal/modules/codedelivery/gitlab_client.go:288-356`（EnsureBranch）

**Interfaces:**
- Consumes: GitHub 适配器对照先例（分支直接创建于 parent=BaselineSHA，无回卷问题）；`ErrDispatchNotStarted`、`ErrBaselineTooLarge`、`ErrInvalidMaterial` 哨兵；`MaxDeliveryFiles`。
- Produces: (a) F52——分支不存在时 `startRef` 不再取默认分支 tip（base），改取**基线锚点**（BaselineSHA；无基线时基线即默认分支 tip 的既有首迭代语义不变），`current` 从基线树起算，使 intended−current 的差集只含本交付真删的文件；(b) F73——空 actions 且 !exists 返回 `ErrDispatchNotStarted`（可重入），超上限返回独立的确定性 `ErrBaselineTooLarge` 语义（settle 落 failed 并向用户暴露上限原因，而非滞留 unknown）；上限分母改为**非 delete 动作数**（delete 不产生 delivery 文件）。

**根因与修复说明：** F52：分支不存在时以默认分支 tip 为 startRef 计算 current，而 intended=基线树+entries——基线锚定后合并进默认分支的增量（别人后续提交）被生成为回卷 update/delete 动作，MR 被合并即真实丢失默认分支增量（GitHub 路径无此问题，两平台安全行为不一致）。F73：EnsureBranch 的本地拒绝（空 actions :351 / 超上限 :354）既非 ErrDispatchNotStarted 也非远端事实错误，dispatcher 原样上抛后 settle 落 unknown 无自愈出口；且上限按含 delete 的总 action 数计，正常 3 轮迭代即确定性触发，ResolveUnknown 会因同分支上一轮开放 draft MR 误收敛为 delivered 污染审计链。

- [ ] **Step 1: 写失败测试**

```go
func TestEnsureBranchDoesNotRevertPostBaselineDefaultBranchCommits(t *testing.T) {
	// arrange: fake GitLab：默认分支 tip 上有基线锚定后新提交（新增文件 X、修改文件 Y）
	// act: EnsureBranch（分支不存在）+ actions 计算注入本交付 entries
	// assert: 生成的动作不含对 X 的 delete、不含把 Y 回卷到基线旧内容的 update（GitHub 同场景对照断言一致性）
}
func TestEnsureBranchLocalRejectionsSettleDeterministically(t *testing.T) {
	// arrange1: 空 actions 且分支不存在 → assert 返回 ErrDispatchNotStarted（可重入，非 unknown 滞留）
	// arrange2: 非 delete 动作数超 2*MaxDeliveryFiles（delete 不计入）→ assert ErrBaselineTooLarge；
	//   恰好因 delete 累积导致总数超限但非 delete 数未超 → 不再报上限
}
```

- [ ] **Step 2: 实跑确认失败**

Run: `go test ./internal/modules/codedelivery/ -run 'EnsureBranch'`
Expected: FAIL ×2。

- [ ] **Step 3: 最小实现**

按 Interfaces Produces 落地（startRef 取 BaslineSHA 的字段名以 DeliveryMaterial 实体实读为准；若基线锚点缺失于该路径，按「首迭代=基线即 tip」既有语义退化为 base 并注释说明）。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/modules/codedelivery/`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/modules/codedelivery/gitlab_client.go
git commit -m "fix(gitlab): anchor EnsureBranch at the baseline and settle local rejections deterministically (B5-F52, B5-F73)"
```

---

### Task 9: 飞书 docx 请求契约（B5-F60 + B5-F61）

**Files:**
- Modify: `internal/modules/appconnector/feishu_docx.go:539`（create 请求体补 title）
- Modify: `internal/modules/appconnector/feishu_docx.go:741`（appendChildren index 去掉 -1）
- Modify: `internal/modules/appconnector/publish/provider.go:64-65`（与 feishu_docx.go 头注释矛盾的注释修正）

**Interfaces:**
- Consumes: 飞书 SDK `CreateDocumentBlockChildrenReqBody.Index *int`（omitempty；官方语义 0=最前、缺省=追加末尾、负值未定义）；`CreateDocumentReqBody` 的可选 `Title` 字段（SDK model.go:7897）；`snap.Title` 已在快照契约 `{parent_folder, title, blocks}` 中。
- Produces: create 请求体 `{"folder_token": ..., "title": snap.Title}`；appendChildren 请求体**省略 index 键**（`map[string]any{"children": blocks}`）表达追加末尾；provider.go 注释改为如实描述（create 契约含可选 title、本管线外发 title）。

**根因与修复说明：** F60：`"index": -1` 不在 SDK/FE-PUB-01 契约内（0=最前、缺省=末尾、负值未定义）——服务端校验拒绝 -1 时所有多块发布在第一步确定性 4xx（create 副作用已发生）；fake server 解码后忽略该值，测试无法捕获；缺省即无风险表达追加。F61：create 请求体只带 folder_token，审批绑定的 title 从不外发（docx 标题不从 text 块派生），所有发布文档均为「无标题文档」，审批 Title 沦为死台账；provider.go:64-65「契约无 title 参数」的注释与 feishu_docx.go 头部承认 SDK 声明可选 title 直接矛盾。

- [ ] **Step 1: 写失败测试**

```go
func TestFeishuCreateSendsTitleAndAppendOmitsIndex(t *testing.T) {
	// arrange: fake 飞书服务严格校验：create 必须携带非空 title（缺省则创建无标题文档并记录）；
	//   children 请求体若含 "index" 键且值为负 → 400
	// act: execute create（多块）
	// assert: 成功；捕获的 create 体含 title == snap.Title；children 体无 index 键
}
```

- [ ] **Step 2: 实跑确认失败**

Run: `go test ./internal/modules/appconnector/ -run TestFeishuCreateSendsTitle`
Expected: FAIL——create 体无 title；children 体含 index:-1。

- [ ] **Step 3: 最小实现**

按 Interfaces Produces 落地（JSON map 省略键即 omitempty 语义）。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/modules/appconnector/`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/modules/appconnector/feishu_docx.go internal/modules/appconnector/publish/provider.go
git commit -m "fix(feishu): send the approved title on create and omit undefined append index (B5-F60, B5-F61)"
```

---

### Task 10: 飞书多批次续传路径（B5-F77）

**Files:**
- Modify: `internal/modules/appconnector/feishu_docx.go:526-580`（executeCreate 多批次循环，续传入口）
- Modify: `internal/modules/appconnector/service/appconnector/action.go:329` 附近（Execute 状态门接受续传态）或 Query 侧（二选一，见 Step 3）

**Interfaces:**
- Consumes: `progress.DocumentID`/`progress.BlocksDone`（feishu_docx.go:535-536 已读进度字段——续传数据已存在）；Execute 状态门 `if row.State != appconn.ActionAuthorized`（action.go:329）；30s `http.Client.Timeout`（dispatcher.go:331）；progress 持久化通道（执行者实读 progress 的存取点）。
- Produces: 多批次中途失败（ActionFailed/ActionUnknown）后存在生产可达的收敛路径。最小方案：**executeCreate 中途 ActionUnknown/失败但已取得 docID 时，把 docID+blocksDone 写入 progress 并让 Query 侧对携带 progress 的 action 直接续跑 appendChildren 剩余批次再收敛 terminal**；或等效地放宽 Execute 门接受「dispatched+progress 存在」的续传。方案择一，验收标准唯一：中途失败不永久卡 unknown、审批额度不被白白消费。

**根因与修复说明：** executeCreate 多批次 appendChildren 循环中途失败落 ActionFailed/ActionUnknown；Execute 仅接受 authorized（unknown 无法重入续传）、Query 要求完整块序列——进度续传在生产管线不可达；30s deadline 下较大文档（多批次 × 网络延迟）易落入永久 unknown，审批额度被消费。progress 字段已存在（:535-536 在读），缺的是失败后能再进入的通道。

- [ ] **Step 1: 写失败测试**

```go
func TestFeishuMultiBatchFailureResumesAndConverges(t *testing.T) {
	// arrange: fake 飞书：第 2 批 appendChildren 返回 504（后恢复）；共 3 批
	// act: execute → 中途 unknown；随后 Query（或重入 Execute，按 Step 3 选定方案）
	// assert: 续传只补发剩余批次（第 1 批不重发——捕获请求计数断言）；最终收敛 succeeded；文档块数完整
}
```

- [ ] **Step 2: 实跑确认失败**

Run: `go test ./internal/modules/appconnector/ -run TestFeishuMultiBatchFailureResumes`
Expected: FAIL——当前中途 unknown 后 Query 永远 unverifiable、Execute 拒绝重入。

- [ ] **Step 3: 最小实现**

按 Interfaces Produces 择一落地（倾向 Query 侧续传：不放宽 Execute 的 authorized 门——审批语义不变；执行者若发现 progress 持久化通道不支持写入，则按实际通道调整并在 commit message 注明）。**涉及状态机放宽的部分若与 ADR 冲突，升级处理而非静默重设计**（AGENTS.md 冲突规则）。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/modules/appconnector/...`
Expected: PASS（含 Task 5/9 新增）。

- [ ] **Step 5: Commit**

```bash
git add internal/modules/appconnector/
git commit -m "fix(feishu): make multi-batch publish resumable after mid-flight failure (B5-F77)"
```

---

### Task 11: voice-room：音频清理端口（B5-F26；顺手 B5-F35）

**Files:**
- Modify: `packages/mobile-core/src/voice-room/voice-room.ts:146` 附近（VoiceRoomPorts 补 audioCleanup）、`:171-173`（discardAudio 接清理）、`:299`（转写成功路径）
- Modify: `apps/mobile/src/composition.ts:392-411`（voiceRoomFor 接线 audioCleanup；顺手 F35：maxSeconds 引用 mobile-core 常量）

**Interfaces:**
- Consumes: dictation 的同型端口先例 `packages/mobile-core/src/voice/dictation.ts:78` `audioCleanup?: (audio: DictationAudio) => Promise<void>` 与 `:129-130` 的 fire-and-forget 调用（「尽最大努力：清理失败不外泄、不阻塞主流程」）；组合根既有 `createNativeAudioFileCleanupIfAvailable`（composition.ts 已 import 但 voiceRoomFor 未用）；`DictationCapturePort` 产出的音频 uri 形态。
- Produces: `VoiceRoomPorts.audioCleanup?: (audio: { uri: string }) => Promise<void>`；`discardAudio` 在现有 `disposition.onDiscarded` 观察回调之外调用 `ports.audioCleanup?.(audio)`（尽最大努力）；composition 的 voiceRoomFor 传入与 dictation 同源的清理适配。F35：组合根硬编码 600 改引用 mobile-core 导出的 `VOICE_ROOM_MAX_SECONDS`（若未导出则在 voice-room.ts 导出）。

**根因与修复说明：** 真机上每轮转写的 m4a 临时文件没有任何客户端删除路径：`discardAudio`（:171-173）仅调 `disposition.onDiscarded` 观察回调，VoiceRoomPorts（:134-146）无 audioCleanup 端口，组合根复用 nativeDictationCapture 但未传清理能力——违反 CONTEXT.md:339「原始音频默认在实时处理后删除」。dictation 模块已有完整同型端口（:78/:129-130），voice-room 照抄该 seam 即可。

- [ ] **Step 1: 写失败测试（voice-room.test.ts 追加）**

```ts
test('a transcribed turn cleans up its raw audio file via the audioCleanup port (B5-F26)', async () => {
  // arrange: in-memory 夹具 + capture 产出 { uri: 'file:///tmp/turn.m4a' }；
  //   audioCleanup: async (audio) => cleaned.push(audio.uri)
  // act: 走完一轮 transcribe 成功路径（state → review）
  // assert: cleaned 深等 ['file:///tmp/turn.m4a']；清理端口抛错时转写流程仍成功（尽最大努力，负例）
});
test('leaving during listening also cleans up the raw audio (B5-F26)', async () => {
  // act: 转写中 leave() → discardAudio('cancelled') 路径
  // assert: cleaned 含该 uri
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/voice-room/voice-room.test.ts`
Expected: FAIL ×2——cleaned 为空。

- [ ] **Step 3: 最小实现**

```ts
// VoiceRoomPorts 追加：
/** 原始音频清理（CONTEXT.md:339：实时处理后删除）；缺省不清理，尽最大努力不阻塞主流程。 */
audioCleanup?: (audio: { uri: string }) => Promise<void>;
// discardAudio 内（onDiscarded 之后）：
if (audio?.uri !== undefined && ports.audioCleanup !== undefined) {
  void ports.audioCleanup({ uri: audio.uri }).catch(() => undefined);
}
```

composition.ts voiceRoomFor 的 createVoiceRoom 入参追加 `audioCleanup`（与 dictation 装配 :364-371 同一适配对象复用）。

- [ ] **Step 4: 实跑确认通过 + 回归 + typecheck**

Run: `pnpm exec tsx --test packages/mobile-core/src/voice-room/voice-room.test.ts apps/mobile/src/voice-room-integration-smoke.test.ts`
Expected: PASS。
Run: `pnpm --filter @weknora/mobile typecheck`
Expected: exit 0。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/voice-room/voice-room.ts packages/mobile-core/src/voice-room/voice-room.test.ts apps/mobile/src/composition.ts
git commit -m "fix(voice-room): delete raw turn audio after processing via an audioCleanup port (B5-F26; F35)"
```

---

### Task 12: voice-room：状态机守卫（B5-F27 + B5-F28；顺手 B5-F51）

**Files:**
- Modify: `packages/mobile-core/src/voice-room/voice-room.ts:176-181`（collapseOnScopeLoss 区分代次过期）
- Modify: `packages/mobile-core/src/voice-room/voice-room.ts:343-348`（resume 守卫补全 + 不清空 pendingTurnId）
- Modify: `packages/mobile-core/src/voice-room/voice-room.ts:344` 附近（顺手 F51 归属 UI 侧——见说明）

**Interfaces:**
- Consumes: `live(attemptGeneration)`（:169 `attemptGeneration === generation && leaseActive(...)`）；`collapseOnScopeLoss`（:176-181 现把两因合并 publish ended/scope-revoked）；resume 守卫（:344 `if (leaving || state.phase === 'connecting') return`）与 `:348 amend({ pendingTurnId: undefined })`；confirmTranscript/discardTurn 按 `pendingTurnId` 匹配（:316/:335）。
- Produces: `collapseOnScopeLoss` 对「代次过期但 lease 仍活」返回 false 且**静默丢弃**（迟到结果本就属于旧代次，:289 注释「迟到结果整代丢弃」语义），仅 lease 真失效才 publish scope-revoked；resume 守卫扩为 `if (leaving || state.phase !== 'ready') return`（connecting/transcribing/review 均不重开，模块自身守卫不依赖 Screen 按钮门控）且**不再无条件清空 pendingTurnId**（review 轮的确认/放弃通道保活）。F51（apps/mobile 语音房屏幕 ended 态无操作按钮）：在对应 Screen 渲染层保留「离开/返回」出口（apps/mobile 侧文件以实读定位，属 UI 呈现不碰状态机）。

**根因与修复说明：** F27：resume（:345）与 leave（:362）都 `generation += 1`，旧代次的迟到结果进入 `collapseOnScopeLoss` 时 `live()` 为假——与「scope 真被撤销」合并为同一处理，伪报 scope-revoked 并锁死 ended 态，state 与内部 sessionId 失一致（openSession catch :209 在 live 为假时直接 return，phase 永久卡 connecting）。F28：resume 守卫仅拦 leaving/connecting——ready 阶段调用会因 `:348` 清空 pendingTurnId 产生无法确认/放弃的孤儿轮次；transcribing 期调用触发 F27 污染。

- [ ] **Step 1: 写失败测试（voice-room.test.ts 追加）**

```ts
test('a stale-generation late result is dropped silently, not reported as scope-revoked (B5-F27)', async () => {
  // arrange: 走到 transcribing；resume()（generation+1）后让旧代次转写结果迟到 settle
  // assert: state.phase 不是 'ended'；notice.reason 不是 'scope-revoked'；sessionId 未被清
});
test('resume() outside ready is a guarded no-op and keeps the pending turn confirmable (B5-F28)', async () => {
  // arrange: 走到 review（pendingTurnId 存在）
  // act: resume()
  // assert: state 不变（非 connecting 重开）；随后 confirmTranscript() 仍成功（pendingTurnId 未被清空）
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/voice-room/voice-room.test.ts`
Expected: FAIL ×2。

- [ ] **Step 3: 最小实现**

按 Interfaces Produces 落地（守卫收紧后注意 Task 13 冒烟顺序的联动——resume 合法窗口 = ready）。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test packages/mobile-core/src/voice-room/voice-room.test.ts apps/mobile/src/voice-room-integration-smoke.test.ts apps/mobile/src/app-smoke.test.tsx`
Expected: PASS（既有用例若依赖旧松动语义按新语义修正断言并注明）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/voice-room/ apps/mobile/src/
git commit -m "fix(voice-room): separate stale-generation drops from real scope revocation and guard resume (B5-F27, B5-F28; F51)"
```

---

### Task 13: voice-room 冒烟顺序（B5-F50；顺手 B5-F32/F36）

**Files:**
- Modify: `apps/mobile/src/voice-room-integration-smoke.ts:236-242`（顺序 leave→resume→leave 调整）

**Interfaces:**
- Consumes: Task 12 后的合法 resume 窗口（ready）；leave() 的 `leaving=true` 永不复位语义（:359-362，维持不动——leave 是终态出口）。
- Produces: 冒烟序列改为 `resume → leave`（或 `resume(no-op 断言) → leave`）：resume 在 review/ready 窗口内行使，evidence.resume 如实记录成功；终末 leave 只做一次。顺手 F32：同文件裸探测分支 `sessionRemote.open` 成功后补 `end()`；F36：注释与 `return 'unavailable'` 实现如实对齐。

**根因与修复说明：** 现顺序（:236 先 leave、:240 再 resume、:242 二次 leave）下，leave 置 leaving=true 永不复位、resume 守卫使其 no-op、state 停 ended、sessionId 已清——:241 的 resume 证据恒判 'failed'，一次完全成功的 live 运行仍产出 resume:'failed' 误导性证据；:242 第二个 leave 同为 no-op。

- [ ] **Step 1: 写失败断言改造（同文件契约测试 voice-room-integration-smoke.test.ts 追加/调整）**

```ts
// 契约钉子：冒烟脚本源码断言（读源或导出序列常量）——
// resume 步骤必须出现在首个 leave 之前；evidence.resume 由真实 resume 结果填充而非守卫 no-op。
```

（若该冒烟只有运行时证据无源断言，则改用导出 `SMOKE_SEQUENCE` 常量断言顺序。）

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/voice-room-integration-smoke.test.ts`
Expected: FAIL——现顺序 resume 在 leave 后。

- [ ] **Step 3: 最小实现**

按 Produces 调整序列与两处顺手注释/探测补 end。

- [ ] **Step 4: 实跑确认通过**

Run: `pnpm exec tsx --test apps/mobile/src/voice-room-integration-smoke.test.ts`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/voice-room-integration-smoke.ts apps/mobile/src/voice-room-integration-smoke.test.ts
git commit -m "test(voice-room): exercise resume before the terminal leave in the smoke order (B5-F50; F32/F36)"
```

---

### Task 14: research：draftId 进程间唯一（B5-F45）

**Files:**
- Modify: `packages/mobile-core/src/research/task-research.ts:52-54`（nextDraftId 弃用进程计数器）
- Test: `packages/mobile-core/src/research/task-research.test.ts`

**Interfaces:**
- Consumes: `DRAFT_ID_PATTERN`（:33 `[A-Za-z0-9._-]{1,64}`，可容纳熵后缀）；`TaskResearchPorts` 既有 `newRequestId` 类 id 工厂先例（voice-room 的 `createNativeRequestId` 注入模式）；scoped-vault `put` 按 id 覆盖写（无唯一性校验）。
- Produces: `nextDraftId` 改为注入的幂等 id 工厂（`ports.newDraftId?: () => string`，缺省用模块内 crypto 级随机——`crypto.randomUUID()` 截断或 `Math.random`+时间戳熵，保 DRAFT_ID_PATTERN 合法）；不再依赖进程内计数器。两个进程实例的 draftId 必然互异。

**根因与修复说明：** `research-ann-${(++draftSeq)}` 的 draftSeq 在 createTaskResearch 闭包内、每次进程启动从 0 重置，而草稿持久化在跨进程存活的 Scoped Vault——会话 A 离线草稿未同步→杀进程→会话 B 冷启动 draftSeq 复位→`put` 按 id 静默覆盖，违背「不静默丢批注」承诺。

- [ ] **Step 1: 写失败测试**

```ts
test('draft ids differ across process restarts (B5-F45)', () => {
  // act: const a = createTaskResearch(ports); const idA = a.annotate(...).draftId;
  //      const b = createTaskResearch(ports); const idB = b.annotate(...).draftId;  // 模拟冷启动新实例
  // assert: idA !== idB（当前同为 research-ann-1 → 红）
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/research/task-research.test.ts`
Expected: FAIL——两实例 id 相同。

- [ ] **Step 3: 最小实现**

按 Produces 落地（组合根可继续不传 newDraftId——缺省实现已进程间唯一）。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test packages/mobile-core/src/research/task-research.test.ts`
Expected: PASS（既有 draft 用例的显式 id 断言若有 `research-ann-1` 字面量，按新形态修正并注明）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/research/task-research.ts packages/mobile-core/src/research/task-research.test.ts
git commit -m "fix(research): make draft ids unique across process restarts (B5-F45)"
```

---

### Task 15: research：错误映射纪律（B5-F46 + B5-F47；顺手 B5-F17/F18/F23/F25）

**Files:**
- Modify: `packages/mobile-core/src/research/task-research.ts:112`（annotate 离线分支 drafts.put 包装）
- Modify: `packages/api-client/src/mobile/research.ts:114-117`（delegate 区分确定性 400）与 annotate 分支（F25：404 区分）
- Modify: `apps/mobile/src/composition.ts`（F17/F18：draftsPort put/list/remove 错误形态统一）

**Interfaces:**
- Consumes: `ResearchError` 模块错误族与 `messageOf` 翻译（research-view 只翻译 ResearchError/OfflineGateError）；服务端确定性 400 错误码 `research_invalid_request`、`research_source_out_of_task_grant`（workbench_research.go:185/:190/:204 实存）；组合根 drafts 适配器现抛普通 `Error('RESEARCH_DRAFT_UNAVAILABLE')`。
- Produces: drafts.put 失败包装为 `coded('RESEARCH_DRAFT_UNAVAILABLE', cause)` 形态的 ResearchError（用户得到「当前无法安全保存离线批注草稿…」设计文案）；api-client delegate 对 4xx 响应体中的服务端错误码透传（`research_source_out_of_task_grant` 等→独立 coded 码，文案「来源不在本任务授权范围内」/「请求无效」），仅网络/5xx 折叠 RESEARCH_BACKEND；F25：annotate 的 404 `research_not_found` 独立映射；F17/F18：draftsPort 的 list/remove 打开失败也抛同族 ResearchError 而非静默 []/no-op（put 的 fail-closed 对齐）；F23：委派失败证据前缀按真实失败类别（网络/越权/无效）不再一律 `delegation-rejected-by-scope-fence`。

**根因与修复说明：** 同根因——research 域的错误在 Port 边界丢失类别：F46 组合根普通 Error 上抛致用户看到裸内部码（同分支 requireDrafts/DRAFT_ID_PATTERN 都抛 ResearchError，唯独 Port 调用绕过归一化）；F47 服务端两类确定性 400 被统一译为「服务端暂时不可用，请稍后重试」——「来源越权」是用户手输 KB id 的预期常见路径，重试必然再失败。

- [ ] **Step 1: 写失败测试**

```ts
// task-research.test.ts
test('a failing drafts port surfaces as ResearchError, not a raw internal code (B5-F46)', async () => {
  // arrange: drafts.put rejects new Error('RESEARCH_DRAFT_UNAVAILABLE')
  // act+assert: annotate 离线分支 rejects ResearchError（coded 码 RESEARCH_DRAFT_UNAVAILABLE），非裸 Error
});
// research 契约测试（api-client 包或 apps 侧既有契约文件）
test('delegate maps deterministic 400s distinctly from backend failures (B5-F47)', async () => {
  // arrange: request 拒绝 400 {error: 'research_source_out_of_task_grant'} / 400 research_invalid_request / 网络 Error
  // assert: 三者错误码互异且前两者文案不含「稍后重试」；网络错误仍 RESEARCH_BACKEND
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/research/task-research.test.ts` + research 契约测试文件
Expected: FAIL ×2。

- [ ] **Step 3: 最小实现**

按 Produces 落地（F23 前缀修正落在产出 evidence 的分支处——执行者按 ask 描述「任何委派失败都打 delegation-rejected-by-scope-fence 前缀」grep 定位实际文件）。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test packages/mobile-core/src/research/task-research.test.ts apps/mobile/src/research-view.test.ts`
Expected: PASS。
Run: `pnpm --filter @weknora/mobile typecheck`
Expected: exit 0。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/research/ packages/api-client/src/mobile/research.ts apps/mobile/src/composition.ts
git commit -m "fix(research): preserve error classes across the drafts/delegate ports (B5-F46, B5-F47; F17/F18/F23/F25)"
```

---

### Task 16: research：UI 如实呈现（B5-F19；顺手 B5-F48/F49/F20/F3）

**Files:**
- Modify: `apps/mobile/src/research-view.ts:108-110`（flushDrafts 统计 failed）
- Modify: `apps/mobile/src/app/tasks/research.tsx:25`（F20：materials 句柄 close）
- Modify: `apps/mobile/src/screens/MaterialsScreen.tsx:30`（F3：Text onPress → Button）
- Modify: research 域 receipt/list 类型处（F48/F49，见说明）

**Interfaces:**
- Consumes: `flushAnnotationDrafts` 产出 `{ outcome: 'conflict' | 'failed' }`（task-research.ts:181——网络性失败草稿保留）；句柄配对纪律先例（/tasks/materials 宿主 effect 内 open、dispose 时 close）；同屏 Button 先例（MaterialsScreen 返回/刷新/下载/分享）。
- Produces: notice 三分支——全成「批注同步完成。」、有 conflict「…N 条因版本更新需重读后重提。」、有 failed「批注同步完成 N/M 条，K 条失败已保留草稿，可稍后重试。」；F20：materials 回调的句柄在用毕 index 后 close（与宿主 dispose 配对模式一致）；F3：改 `<Button title="研究与批注" onPress={onOpenResearch} testID="materials-open-research" />`；F48：不可达三元 else 移除（receipt.outcome 恒 'accepted'，冲突走抛错——契约收敛为两态）；F49：listed 联合类型删 `'no-materials'` 死成员（实际写 `annotated='skipped'`——按 ask 描述 grep 实位）。

**根因与修复说明：** flushDrafts 只统计 conflict 不统计 failed——离线误触同步全部失败时用户仍见「批注同步完成。」，误导性成功文案违背「如实呈现」。其余三项为同域呈现/契约一致性顺手修。

- [ ] **Step 1: 写失败测试（research-view.test.ts 追加）**

```ts
test('flush notice reports failed drafts honestly (B5-F19)', async () => {
  // arrange: flushAnnotationDrafts resolve [{outcome:'failed'},{outcome:'failed'}]
  // act: 触发同步
  // assert: notice 含「失败」与「保留草稿」字样，且不含纯成功文案「批注同步完成。」
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/research-view.test.ts`
Expected: FAIL——现 notice 恒「批注同步完成。」。

- [ ] **Step 3: 最小实现**

按 Produces 落地（F48/F49 为类型收敛，typecheck 钉死；F3 无运行时测试，靠 app-smoke 既有渲染红线不回归）。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test apps/mobile/src/research-view.test.ts apps/mobile/src/app-smoke.test.tsx`
Expected: PASS。
Run: `pnpm --filter @weknora/mobile typecheck`
Expected: exit 0（F48/F49 类型收敛在此爆残留引用——grep 后应为零）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/research-view.ts apps/mobile/src/app/tasks/research.tsx apps/mobile/src/screens/MaterialsScreen.ts
git commit -m "fix(research-ui): report failed draft flushes honestly and tidy handle/type contracts (B5-F19; F48/F49/F20/F3)"
```

---

### Task 17: iOS 验收脚本门收紧（B5-F1 + B5-F13 + B5-F14 + B5-F15；顺手 B5-F2/F16）

**Files:**
- Modify: `apps/mobile/scripts/ios-acceptance-run.sh:23-25`（F13 录屏 trap）、`:61`（F15 test -s 守卫）、`:71-72`（F14 log show 窗口与失败退出 + F1 grep 模式补全）
- Modify: `apps/mobile/scripts/ios-release-build.sh:23-27`（F2 ln -sfn）
- Modify: `apps/mobile` 的 typecheck/tsconfig 覆盖（F16，见 Step 3）

**Interfaces:**
- Consumes: 脚本既有 `set -euo pipefail`（:7）；`t39-record.json` 消费 push-process-alive.txt 作为 fail-closed 证据（含 PID）。
- Produces: F1——grep 模式补 `SIGABRT|SIGBUS|SIGILL|EXC_BAD_ACCESS|Terminating app`；F14——记录脚本起始时间并以 `--start "$T0"` 限定窗口，`log show` 失败（空/仅错误文本）时显式 `echo "log capture failed" >&2; exit 1` 而非 `|| true` 放行；F15——`grep "$BUNDLE" > push-process-alive.txt || true` 后补 `test -s "$OUT/push-process-alive.txt"` 守卫（空文件即失败）；F13——`REC=""` + `trap cleanup_recorder EXIT`（kill -INT + wait），正常 kill 后置空；F2——`ln -sfn "$preset" ...`；F16——scripts 下 TS 文件纳入 typecheck（tsconfig include 或 package.json script 追加，按仓库现有机制择一）。

**根因与修复说明：** 同文件四项共根因——验收门的「失败可被静默放行」：崩溃筛查模式不全（iOS 未捕获 NSException 最终以 abort() 落地 SIGABRT）、log show 失败被吞后空日志 grep 0 命中照常 ACCEPTANCE_PROBES_OK、`--last 5m` 相对窗口吸入前一轮遗留崩溃、push-process-alive.txt 为空无守卫、录屏孤儿进程双写证据文件。F2/F16 同 iOS 脚本域顺手收紧。

- [ ] **Step 1: 写失败验证（shell 断言用 bash -n + 定向场景，主钉子为脚本文本契约）**

```bash
# 契约钉子（可作 CI grep 或 plans 验收步）：
grep -q 'SIGABRT' apps/mobile/scripts/ios-acceptance-run.sh        # F1
grep -q 'trap cleanup_recorder EXIT' apps/mobile/scripts/ios-acceptance-run.sh  # F13
grep -q -- '--start' apps/mobile/scripts/ios-acceptance-run.sh      # F14
grep -q 'test -s .*push-process-alive' apps/mobile/scripts/ios-acceptance-run.sh # F15
```

外加行为验证：临时构造含 `SIGABRT` 的假 app-launch-log.txt 跑第 5 步片段 → 必须 FAILED（现模式 0 命中放行）。

- [ ] **Step 2: 实跑确认失败**

Run: 上述四条 grep（对未修改脚本）
Expected: 全部无命中（RED）。

- [ ] **Step 3: 最小实现**

按 Produces 落地（`--start` 时间取脚本头部 `T0=$(date '+%Y-%m-%d %H:%M:%S')`；F16 的覆盖机制先看 `apps/mobile/package.json` 的 typecheck script 实态再择一）。

- [ ] **Step 4: 实跑确认通过**

Run: 四条 grep 全命中；`bash -n apps/mobile/scripts/ios-acceptance-run.sh && bash -n apps/mobile/scripts/ios-release-build.sh`
Expected: 语法通过、契约全绿。
Run: `pnpm --filter @weknora/mobile typecheck`
Expected: exit 0（F16 后 scripts TS 文件被覆盖且无新增错误）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/scripts/ apps/mobile/package.json apps/mobile/tsconfig.json
git commit -m "fix(ios-acceptance): close vacuous-pass holes in the crash gate and recorder lifecycle (B5-F1, F13, F14, F15; F2/F16)"
```

---

### Task 18: iOS release evidence fail-closed（B5-F8 + B5-F38；顺手 B5-F9/F11/F12/F41）

**Files:**
- Modify: `apps/mobile/src/ios-release-evidence.ts:47`（F8 去 source 默认值）、`:84-88`（F38 disposition 枚举校验）、`:59` 附近（F12 未知 subject 报 gap）
- Modify: `apps/mobile/src/ios-release-evidence-cli.ts:36/:45`（F11 仓库根推导、F9 JSON.parse 防崩、F41 顶层形状校验）
- Test: `apps/mobile/src/ios-release-evidence.test.ts`

**Interfaces:**
- Consumes: 模块「不冒充」契约（AC3）与相邻 reason 分支「不兜底、让 gaps 点名」的注释原则；`REQUIRED_SUBJECTS`；CLI 从手写 JSON `as AcceptanceOutcomes` 读入。
- Produces: F8——evidenced 条目缺 source 时**不再默认** 'installed-package'，`iosReleaseEvidenceGaps` 的 evidenced 分支补 `source === undefined → gaps.push('... must declare source ...')`；F38——gaps 循环前对 disposition 做枚举白名单校验（`'evidenced'|'not-run'|其余既定值` 之外的值 → gap `unknown disposition`，绝不落入按 reason 检查的兜底分支）；F12——未知 subject 显式报 gap；F9——CLI JSON.parse/readFileSync try/catch 打印 `invalid outcomes file (<path>): <message>` 后 exit 1；F11——仓库根从模块位置推导（`realpathSync(fileURLToPath(new URL('../../../', import.meta.url)))`——src/ 相对仓库根深度固定，执行者实核层级数）；F41——顶层非法形状（null/数组）给友好错误而非裸 TypeError，且路径校验文案与实际允许语义（仓库根内绝对/相对路径）一致。

**根因与修复说明：** 同根因——evidence 门的 fail-closed 被默认值与宽松 else-if 架空：缺 source 静默升级为最强 'installed-package'（低保证据被放行、gaps 的 source 检查永不触发）；手写 JSON 拼错的 disposition（如 'skipped'）落入最后 else-if 被当 blocked-env，带 reason 即零 gap 通过——与「任何 skipped 不计为通过」直接冲突。

- [ ] **Step 1: 写失败测试（ios-release-evidence.test.ts 追加）**

```ts
test('an evidenced entry without source is a gap, never defaulted (B5-F8)', () => {
  // assert: gaps 含 'must declare source'；规范化结果不出现 source: 'installed-package'
});
test('an unknown disposition value is a gap, never treated as blocked-env (B5-F38)', () => {
  // arrange: { disposition: 'skipped', reason: 'x' } → assert gaps 非空且点名 unknown disposition
});
// F9/F11/F41：CLI 用例（若 CLI 无测试文件则本任务新建 ios-release-evidence-cli.test.ts）
test('CLI exits 1 with a friendly message on invalid JSON / shape (B5-F9, F5-F41)', () => { /* ... */ });
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/ios-release-evidence.test.ts`
Expected: FAIL ×2+（现默认值放行、'skipped' 零 gap 通过）。

- [ ] **Step 3: 最小实现**

按 Produces 落地。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test apps/mobile/src/ios-release-evidence.test.ts`
Expected: PASS（含既有用例——合法 outcomes 的零 gap 行为不变）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/ios-release-evidence.ts apps/mobile/src/ios-release-evidence-cli.ts apps/mobile/src/ios-release-evidence.test.ts
git commit -m "fix(ios-evidence): fail closed on missing source, unknown dispositions, and malformed outcomes (B5-F8, F38; F9/F11/F12/F41)"
```

---

### Task 19: iOS 冒烟证据保真（B5-F6 + B5-F7；顺手 B5-F10/F34/F39/F40）

**Files:**
- Modify: `apps/mobile/src/ios-core-workflow-integration-smoke.ts:107`（F7 精确匹配）、`:110-112`（F6 注释如实化）、`:256-258`（F10 条件置 failed）
- Modify: 冒烟家族部署校验处（F34/F40 共享 helper 收敛）与 tenantSwitch 证据处（F39）——执行者按 ask 描述 grep 定位实际文件

**Interfaces:**
- Consumes: Start 的权威路径契约（task-office.ts:166 注释、task-office.test.ts:192 `input.path === '/api/v1/workbench/executions'` 精确匹配先例）；同前缀端点实存（executions.ts:306 commands、materials.ts:81 signed-url、interactions.ts:71 decisions）；契约测试 `startPosts === 0` 断言（smoke.test.ts:155）。
- Produces: F7——`input.path === '/api/v1/workbench/executions'` 精确相等；F6——注释改为「请求在发出前拦断（authorized 未被调用、服务端必然未收到首枚 Start POST）；服务端已受理窗口由 lookup-admitted 回归与服务端 admission 幂等测试覆盖」；F10——`if (evidence.signIn !== 'authorized') evidence.signIn = 'failed'`；F34/F40——部署 Origin/URL/凭据校验抽共享 helper（三份拷贝收敛为一份，落在冒烟家族共用模块）；F39——tenantSwitch 证据面补活跃租户真实切换断言（surface 之外多断言一个可观察切换结果）。

**根因与修复说明：** F7 的子串匹配会误命中同前缀 POST 端点污染 `weakNetworkStartRequests` 验收计数、甚至拦错请求；F6 的注释宣称「请求已发出、服务端可能已受理」与实现（authorized 前直接 reject）矛盾，夸大弱网证据保真度；F10 外层 catch 无条件覆写 signIn 把已成功登录面污染为 failed。

- [ ] **Step 1: 写失败测试（ios-core-workflow-integration-smoke.test.ts 追加/改造）**

```ts
test('the weak-network interceptor only counts the exact Start path (B5-F7)', () => {
  // arrange: 走一遍注入路径后让 wiring 额外发一个 POST /api/v1/workbench/executions/run%2F1/commands
  // assert: weakNetworkStartRequests 计数不含 commands 请求；commands 请求不被拦断（正常 resolve）
});
```

（F6/F10 为注释与赋值修正，由既有 :155 `startPosts === 0` 断言与 signIn 断言钉住不回归。）

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/ios-core-workflow-integration-smoke.test.ts`
Expected: FAIL——commands 请求被计入/拦断。

- [ ] **Step 3: 最小实现**

按 Produces 落地（F34/F39/F40 为同域顺手收敛，改动大时允许只做收敛不新增行为，断言以「行为不回归 + 拷贝数减少」为准）。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test apps/mobile/src/ios-core-workflow-integration-smoke.test.ts`
Expected: PASS（:155 契约不回归）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/
git commit -m "fix(ios-smoke): exact Start path matching and honest weak-network copy (B5-F6, F7; F10/F34/F39/F40)"
```

---

### Task 20: agent 治理数据准确性（B5-F37；顺手 B5-F5）

**Files:**
- Modify: `internal/application/service/agent_fork_lineage.go:115`（基线投影补 AgentMode 默认化）
- Modify: `internal/application/repository/agent_marketplace_lineage.go:81-88`（F5 upsert 审计字段）
- Test: 对应既有测试文件（agent_fork_lineage / marketplace lineage 测试）

**Interfaces:**
- Consumes: `custom_agent.go:102-104` 的默认化先例（`if agent.Config.AgentMode == "" { agent.Config.AgentMode = types.AgentModeQuickAnswer }`）；UpsertLicense 的 `clause.OnConflict{DoUpdates: AssignmentColumns([name, allows_redistribution, created_by, updated_at])}`。
- Produces: `derivationBaselinePayload` 在 buildLocalAgent 后、PortablePayloadOf 前补同款默认化（空 AgentMode → AgentModeQuickAnswer），基线投影与采用方快照口径一致；F5——DoUpdates 移除 `created_by`，conflict 路径后回读行（`First` 按 id）返回真实 created_at/created_by。

**根因与修复说明：** F37：源 Release payload.AgentMode 为空时基线投影为 ""、采用方快照为 "quick-answer"——forkVerdict 必判不等，纯 Mapping 重提交（未改动 portable core）被误标 IsFork=true。F5：upsert 更新路径无条件 `license.CreatedAt = now` 而库行保留原值（GORM 不回读非主键列），重注册响应 created_at 虚报且 created_by 被「最后修改人 + 原始创建时间」混搭，损害审计溯源。

- [ ] **Step 1: 写失败测试**

```go
func TestForkBaselineDefaultsEmptyAgentMode(t *testing.T) {
	// arrange: derivation.Release payload AgentMode 为空字符串
	// act: derivationBaselinePayload
	// assert: 投影 AgentMode == quick-answer（与采用方快照相等 → 纯 Mapping 重提交不判 Fork）
}
func TestUpsertLicenseKeepsOriginalCreatorAndCreatedAt(t *testing.T) {
	// arrange: 首注册 by A；act: 重注册 by B
	// assert: 返回值与库行 created_by==A、created_at==首注册时刻；name/updated_at 前进
}
```

- [ ] **Step 2: 实跑确认失败**

Run: `go test ./internal/application/... -run 'ForkBaseline|UpsertLicenseKeeps'`
Expected: FAIL ×2。

- [ ] **Step 3: 最小实现**

按 Produces 落地。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/application/...`
Expected: PASS（Task 1 后基线已绿，维持）。

- [ ] **Step 5: Commit**

```bash
git add internal/application/
git commit -m "fix(agent-lineage): default empty AgentMode in fork baselines and keep license audit fields (B5-F37; F5)"
```

---

### Task 21: FormActionPlan 输入边界（B5-F54；顺手 B5-F55/F70）

**Files:**
- Modify: `internal/handler/app_connector_action_plan.go:88`（items 上限）、`:70`（F55 planByID 错误分类）
- Modify: `internal/modules/appconnector/plan/plan.go:281` 附近（F70 恢复性重批冻结批人）
- Test: action plan 既有测试文件

**Interfaces:**
- Consumes: 批量上限先例（`memoryExportMaxItems`、tenant.go MaxItems≤2000）；`FormPlan` 逐项串行外部预读 + 多次 DB 写的成本事实。
- Produces: `maxActionPlanItems = 200`（:88 校验 `len(input.Items) > maxActionPlanItems` → 400 `INVALID_REQUEST`「plan items exceed cap」）；F55——planByID 对 store 非 NotFound 错误 → 500（与 Task 3 同模式）；F70——恢复性重批路径不再覆写 approved_by/approved_at（首批批人身份与时间戳冻结；重批仅推进状态）。

**根因与修复说明：** F54：单次请求可提交数千 items 形成无界外部调用与超长请求（DoS 面），中途失败留下同等数量孤儿行。F55/F70 为同文件/同域审计准确性顺手修。

- [ ] **Step 1: 写失败测试**

```go
func TestFormActionPlanRejectsOversizeItems(t *testing.T) {
	// arrange: 201 个 items → assert 400 INVALID_REQUEST（现被接受 → 红）
}
func TestPlanByIDReturns500OnStoreFailure(t *testing.T) { /* 注入 DB 错误 → 500 非 404 */ }
func TestReApprovalFreezesFirstApprover(t *testing.T) { /* 重批后 approved_by/approved_at 保持首批 */ }
```

- [ ] **Step 2: 实跑确认失败**

Run: `go test ./internal/handler/ -run 'FormActionPlanRejects|PlanByIDReturns500|ReApprovalFreezes'`（plan.go 用例在 appconnector 包则相应分包跑）
Expected: FAIL。

- [ ] **Step 3: 最小实现**

按 Produces 落地。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/handler/ ./internal/modules/appconnector/...`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/handler/app_connector_action_plan.go internal/modules/appconnector/plan/plan.go
git commit -m "fix(action-plan): cap form items, classify store failures, freeze first approver (B5-F54; F55/F70)"
```

---

### Task 22: voice 深链 runId 与句柄生命周期（B5-F29；顺手 B5-F31/F21）

**Files:**
- Modify: `apps/mobile/src/app/tasks/voice.tsx:27`（runId 冒充移除）、effect catch 路径（F31 句柄清理）
- Modify: runId 缺参文案处（F21——与未登录文案区分）

**Interfaces:**
- Consumes: `task-office.ts:501-506`（detail(runId) 拉流、act(steer) 提交到该 runId）；对照先例 detail.tsx 缺省取空串**显式失败不冒充**；effect cleanup 注册模式。
- Produces: `office.open({ taskId, runId: runId ?? '' })`（缺 runId 显式失败，错误文案点名「缺少执行 ID（runId），请从任务详情进入」——与未登录文案区分，F21）；`room.join` 维持 `runId === undefined ? {} : { runId }` 既有形态；F31——effect 内构造抛错时已开句柄在 catch 路径 close（cleanup 函数提前注册或 catch 内显式释放）。

**根因与修复说明：** `runId: runId ?? taskId` 以 taskId 冒充 runId：外部深链缺 runId 时 steer 指令被提交到 runId=taskId 的错误执行流或触发服务端校验失败——与 detail.tsx「缺省取空串显式失败不冒充」的既有先例相悖。

- [ ] **Step 1: 写失败测试（app-smoke 或 voice 屏既有测试追加）**

```tsx
test('voice screen without runId fails explicitly instead of impersonating (B5-F29)', () => {
  // arrange: 渲染 voice 屏、路由参数无 runId
  // assert: office.open 收到 runId: ''（非 taskId）；呈现「缺少执行 ID」类文案（F21 断言文案与未登录文案互异）
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx`
Expected: FAIL——现传 taskId。

- [ ] **Step 3: 最小实现**

按 Produces 落地。

- [ ] **Step 4: 实跑确认通过 + 回归 + typecheck**

Run: `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx`
Expected: PASS。
Run: `pnpm --filter @weknora/mobile typecheck`
Expected: exit 0。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/app/tasks/voice.tsx apps/mobile/src/app-smoke.test.tsx
git commit -m "fix(voice): fail explicitly on missing runId instead of impersonating taskId (B5-F29; F31/F21)"
```

---

### Task 23: api-client 503 翻译（B5-F30）

**Files:**
- Modify: `packages/api-client/src/mobile/voice-sessions.ts:68-70`（503 分支收敛）
- Test: api-client 既有 voice-sessions 契约测试（无则新建）

**Interfaces:**
- Consumes: 既有码族 `VOICE_OPEN_UNKNOWN` / `VOICE_CHARGING_UNCONFIGURED` / `VOICE_PROVIDER_UNAVAILABLE`（502 用）。
- Produces: 503 分支——`text.includes('voice_open_unknown')` → VOICE_OPEN_UNKNOWN；**其余 503 一律 VOICE_PROVIDER_UNAVAILABLE**（服务端/网关侧暂不可用的既有中性码），VOICE_CHARGING_UNCONFIGURED 改由服务端在 503 响应体中携带明确标志（如错误码 `voice_charging_unconfigured` 子串）时才译出——未知方向不再兜底为部署配置结论。

**根因与修复说明：** 现实现「除一个子串外的一切 503」默认译 VOICE_CHARGING_UNCONFIGURED——未来网关层 503/限流都会被误译为部署配置结论，污染冒烟证据 charging-unconfigured 分支的诚实性。需先 grep 服务端 503 响应体实标（voice open 的 charging 未配置错误实际回什么文本/码），据此定子串；若服务端尚无明确标志，本任务包含服务端小改（在 503 体中加错误码）或经升级确认文案。

- [ ] **Step 1: 写失败测试**

```ts
test('a generic 503 maps to provider unavailable, not charging-unconfigured (B5-F30)', () => {
  // arrange: request 拒绝 503 无 voice_open_unknown 子串、无 charging 标志（如网关限流页）
  // assert: coded 码为 VOICE_PROVIDER_UNAVAILABLE
  // 补：503 体含服务端 charging 标志 → VOICE_CHARGING_UNCONFIGURED（正例）
});
```

- [ ] **Step 2: 实跑确认失败**

Run: 对应契约测试文件
Expected: FAIL——现译 CHARGING_UNCONFIGURED。

- [ ] **Step 3: 最小实现**

按 Produces 落地（charging 标志子串以服务端实际响应实读为准，差异记入 commit message）。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: 契约测试 + `pnpm exec tsx --test packages/mobile-core/src/voice-room/voice-room.test.ts apps/mobile/src/voice-room-integration-smoke.test.ts`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add packages/api-client/src/mobile/voice-sessions.ts
git commit -m "fix(api-client): stop defaulting unknown 503s to charging-unconfigured (B5-F30)"
```

---

## 附录 A：批次验收（全部任务完成后）

**Go 侧：**
- Run: `go build ./...` → Expected: exit 0。
- Run: `go test ./internal/handler/ ./internal/application/... ./internal/modules/appconnector/... ./internal/modules/codedelivery/`
  Expected: **全绿**（基线中 handler 16 个 duplicate-migration 失败在 Task 1 后转绿并全程维持；service 包抽查失败同根因消失）。
- 迁移树终态核验：`ls migrations/versioned/ | grep -E '^000(198|199)'` 恰为 `000198_app_action_plans` + `000199_space_connection_grants` 四文件；`ls migrations/sqlite/ | grep -E '^000(119|123)'` 恰为 `000119_app_action_plans` + `000123_space_connection_grants` 四文件。

**TS 侧：**
- Run: `pnpm exec tsx --test packages/mobile-core/src/voice-room/voice-room.test.ts packages/mobile-core/src/research/task-research.test.ts apps/mobile/src/research-view.test.ts apps/mobile/src/voice-room-integration-smoke.test.ts apps/mobile/src/ios-core-workflow-integration-smoke.test.ts apps/mobile/src/ios-release-evidence.test.ts`
  Expected: 基线 34 pass / 0 fail / 2 skipped 之上，新增用例全绿、0 fail。
- Run: `pnpm --filter @weknora/mobile typecheck` → Expected: exit 0。
- Run: `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx` → Expected: PASS（B4 基线 66 用例 + 本批次新增）。
- Run: `bash -n apps/mobile/scripts/ios-acceptance-run.sh && bash -n apps/mobile/scripts/ios-release-build.sh` → Expected: 语法通过。

**失败集合不扩大原则：** 任何预存在失败（TS 2 skipped；Go 侧经 Task 1 消除的迁移失败）不得因本批次新增；若既有用例因新语义而红（如 voice-room 守卫收紧、resume 序列），按新语义修正断言并在 commit message 注明。
