# OCR 增量修复批次 3（issue30-sweep）修复报告

- 执行者：OCR 修复员-B3（修复执行员，按计划实施，未派发子代理）
- 计划：`docs/plans/issue30-sweep/plans/ocr-fix-increment-b3.md`
- Worktree：`.worktrees/issue30-sweep`
- 执行日期：2026-09-24
- 结果：**15/15 任务全部完成并逐任务提交**；附录 B 批次验收 7 项全部通过（Go 包失败集合与预存在基线完全一致，未扩大）。

## 提交清单（按任务顺序）

| # | Commit | 任务 | 覆盖发现 |
|---|--------|------|----------|
| 1 | `096a5b1de` | contracts：agent-adoption 状态数组 `as const` | B3-F85（high，P0 门禁） |
| 2 | `5aa0c7cd1` | Go：task_grant 授权语义 | B3-F82/F84/F83（high）+ F74/F75 |
| 3 | `d10bf5bf9` | Go：workbench 只读面授权与能力位 | B3-F63/F76 |
| 4 | `9f273fe49` | Go：agent-adoption 治理链 | B3-F69/F86/F87 + F73 |
| 5 | `e21047cb2` | domain：submission 重发终态 | B3-F44（high）+ F62 |
| 6 | `6ec1b9faa` | mobile-core：task-office start/恢复错误语义 | B3-F78/F64/F79/F65（high）+ F68 |
| 7 | `d9b6ce54f` | apps/mobile：意图/草稿持久化防御 | B3-F51/F38/F42（high） |
| 8 | `2e2c328d8` | apps/mobile：new-task 控制器生命周期 | B3-F37/F41/F40/F59/F39（high）+ F46 |
| 9 | `c36fdbb90` | mobile-core：inbox/device 投影一致性 | B3-F1/F5 + F2/F6/F50 |
| 10 | `068c974c6` | api-client：错误翻译与 origin 提取 | B3-F43/F11 |
| 11 | `dc520d199` | mobile-core：material 缓存与凭据回验 | B3-F31/F58 + F32/F33/F34/F35/F55 |
| 12 | `34a2c1c8b` | api-client/app：material 末页与视图 | B3-F52/F54/F57 + F10/F18/F36 |
| 13 | `f66593b7a` | legacy 域：卡片隔离与历史通道 | B3-F21/F22/F23/F24 + F14/F66 |
| 14 | `b0d983f65` | composition/请求 ID/设备身份 | B3-F26/F28/F29/F27/F49 |
| 15 | `e5e9ec9aa` | task-start smoke 证据链 | B3-F45/F60 |

## 各任务 RED→GREEN 证据（实跑命令与输出）

以下命令均在 worktree 根实跑。每任务先写失败测试、实跑确认 RED、最小实现、实跑确认 GREEN、提交（严格 TDD，按计划步骤）。

### Task 1（B3-F85，P0 门禁）
- RED：`npx tsc --noEmit --strict --skipLibCheck --target ES2022 --lib ES2022,DOM --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions packages/contracts/src/marketplace/agent-adoption.ts`
  → `agent-adoption.ts(89,5): error TS2322: Type 'string' is not assignable to type '"draft" | "mapped" | "tested" | "published"'.`（exit 2）
- GREEN：同命令 → exit 0；`pnpm exec tsx --test packages/contracts/src/marketplace/agent-adoption.test.ts` → `# pass 3 / # fail 0`
- 门禁复核：`pnpm run typecheck:shared 2>&1 | grep -E "^packages/" | sed 's/(.*//' | sort | uniq -c` → `2 packages/views/src/chat/mermaid.ts`（agent-adoption 清零；mermaid ×2 为计划声明的范围外预存在）

### Task 2（B3-F82/F84/F83 + F74/F75）
- RED：`go test ./internal/application/service/ -run 'TestGrantTaskAccessSurfaces|TestResolveTaskAccessSurfaces|TestResolveTaskAccessDeactivates' -v`
  → `--- FAIL: TestGrantTaskAccessSurfacesInfrastructureErrorsInsteadOf404`、`--- FAIL: TestResolveTaskAccessSurfacesInfrastructureErrorsInsteadOf404`、`--- FAIL: TestResolveTaskAccessDeactivatesStaleGrants`（actual "viewer" ≠ expected "none"）
  `go test ./internal/handler/ -run TestExtendTaskBudgetGatesBySessionOwnerAuthority -v` → `expected: 200, actual: 403`（real-owner 被旧 owner 谓词拒）
  `go test ./internal/application/repository/ -run 'TestTaskGrantStoreTaskOwnerIDHandlesNull|TestTaskGrantStoreUpsertConflictPath' -v` → 两用例 FAIL（NULL 扫描错误 / created_at 差 24h）
- GREEN：`go test ./internal/handler/ -run 'TestExtendTaskBudget' -v` → 4 个 `--- PASS`（含既有 3 个——共享夹具按新权威补 sessions 表后保持语义）
  store 两用例 `--- PASS`；`go test ./internal/application/service/ -run 'TestGrantTask|TestResolveTaskAccess'` → `--- PASS` ×7 / `ok`
- 实现要点：`loadTaskForGrantManagement`/`ResolveTaskAccess` 仅对 `ErrSessionNotFound` 折叠 404、其余透传（task_grant.go）；grant 命中后经 active 成员校验（停用 → TaskAccessNone）；`taskRunOwner` 改 `agent_runs JOIN sessions` 取 `sessions.user_id`（ADR-0004 权威，commercial_task_budget.go）；`TaskOwnerID` 用 `sql.NullString`（F74）；`UpsertGrant` 冲突路径重读 DB 行（F75）。
- 注：仓库层两个新用例改用自包含 sqlite 夹具（AutoMigrate 式建表），因全量迁移轨道被预存在 000112 冲突破坏（见「预存在失败」节）。

### Task 3（B3-F63/F76）
- RED：`WithTerminalLog` 未定义编译失败（计划注预期）；terminal-log viewer 用例在补齐方法后断言红。
- GREEN：`go test ./internal/handler/session/ -run 'TestGetWorkbenchTerminalLog|TestListWorkbenchArtifacts|TestGetWorkbenchSnapshot|TestGetWorkbenchExecution' -v` → 16 个 `--- PASS`（含既有 owner-scoped fail-closed 用例）
- 实现要点：`GetWorkbenchTerminalLog` 改 `resolveReadableRun`（与 Snapshot/Execution 同规则）；`WorkbenchArtifactHandler` 增 `terminal TerminalLogReader` 字段 + `WithTerminalLog` 构造器，`terminal.available = h.terminal != nil`；装配点在 `internal/container/workbench.go`（read 侧 snapshots 与 501 判定同源）。既有用例 `TestListWorkbenchArtifactsDeclaresTerminalAvailability` 按新诚实语义改为断言「接线后 true」（未接线 false 由新用例钉住）。

### Task 4（B3-F69/F86/F87 + F73）
- RED：`go test ./internal/router/ -run TestAvailableAgentsAPIKeyFloorMatchesAgentList -v` → `Should be false` FAIL（旧 admin=full-access 地板无 read_agents 能力）；repo 用例 `ErrAgentAdoptionRemapStateConflict` undefined；handler 用例 `data["variant"]` 双层包裹 FAIL。
- GREEN：三组用例分别 `--- PASS`。
- 实现要点：available-agents 的 API-key 地板改 `apiKeyReadAgents(apiKeyManageAgents(apiKeyChat(apiKeyFullAccess())))`（镜像 GET /api/v1/agents 的 OR 栈——注意本仓库约定 RequireFullAccess=true 与能力叠加并存，测试断言与镜像端点用例同型）；`PublishVariant` 去双层信封；`ReplaceCapabilityMappings` 的 UPDATE 带 `AND state IN ('draft','mapped','tested')` 守卫，RowsAffected≠1 且行仍在 → 新哨兵 `repository.ErrAgentAdoptionRemapStateConflict`（handler 映射 409）；`decodeIDList` 解析失败记 `logger.Errorf`（F73）。

### Task 5（B3-F44 + F62）
- RED：`pnpm exec tsx --test packages/domain/src/mobile/submission.test.ts` → `not ok 10`（422 落 awaiting_reconciliation）、`not ok 12`（双查 lookup）。
- GREEN：`# pass 12 / # fail 0`；office 回归 `packages/mobile-core/src/task-office/task-office-start.test.ts` → 12/12。
- 实现要点：`isDeterministicRefusal`（status 400-499 鸭子探测）→ rejected 终态；无形态失败复查 lookup（rejected → rejected，其余保持对账态）；resume 非 unknown 分支直接用 lookup 结果（消除双查）；**resume 对 rejected entry 早退返回终态（零网络）**——新语义（rejected 是终态，同 ID 重入只读回）；4 个既有用例断言按新语义修正（详见 commit message：失败 dispatch 后多一次复查 lookup；无形态失败且 lookup=rejected 时直接落终态）。

### Task 6（B3-F78/F64/F79/F65 + F68）
- RED：4 个新用例 `not ok 13-16`（重入抛 CONFLICT / remove 失败伪装失败 / 无 SUPERSEDED / 整批 reject）。
- GREEN：`# pass 16 / # fail 0`；同包回归 `pnpm exec tsx --test "packages/mobile-core/src/task-office/*.test.ts"` → `# tests 76 / # pass 76 / # fail 0`。
- 实现要点：record 缺失时先查 `submissionStore`（进程内权威）——sameScope 且 bound → `receiptOf(settled, false)` 零网络回执；同 scope 非 bound → CONFLICT（sessionId 只能从 intentLog 重建，D5 绝不换 session）；**为区分「同意图幂等重放」与「换意图复用已 bound 的 ID」，office 内维护 `replayGoalKeys`（save/加载记录时写入、remove 不清除）——换 goal 重放仍 CONFLICT（保住既有 zero-network conflict 用例）**；remove 改 best-effort（catch 吞）；bound 后执行与 followUp 相同的五项失效动作；reconcilePending 逐记录 try/catch 隔离；toStartInput 调用处补 F68 契约注释。

### Task 7（B3-F51/F38/F42）
- RED：4 个新用例 `not ok 7/8/9/13`。
- GREEN：`pnpm exec tsx --test apps/mobile/src/adapters/intent-log.test.ts apps/mobile/src/new-task-drafts.test.ts` → `# pass 13 / # fail 0`；office→adapter 链路回归（new-task-view + task-office-start）→ 25/25。
- 实现要点：save/remove 经模块内 promise chain 串行互斥；单条序列化后仍超 1536B → `TaskOfficeError('TASK_OFFICE_INVALID_INPUT')` 显式拒绝（不截断、不落部分状态）；`load()` 字段级防御（text/agentId/budgetUpper/attachments/knowledgeIds 逐项降级）。
- **语义变更声明**：既有用例「a single oversized record is kept alone」被 B3-F38 明确推翻（计划 Interfaces 一节），已改写为「拒绝 + 可行动错误码」并在 commit message 注明。NewTaskScreen 的 maxLength 注释同步更新为「双保险」论证（Task 8）。

### Task 8（B3-F37/F41/F40/F59/F39 + F46）
- RED：5 个新用例 `not ok 10-14`（requestId 未回填 / rejected 驱动重试 / 草稿被清 / 输入被覆盖 / 推荐不刷新）。
- GREEN：`pnpm exec tsx --test apps/mobile/src/new-task-view.test.ts` → `# pass 14 / # fail 0`；回归（new-task-drafts + app-smoke）→ 53/53。
- 实现要点：`initialized` 回填 `intentRequestId = unresolved.requestId`；submit 的 rejected 分支不设 inFlight/intentRequestId、发布 `SUBMISSION_REJECTED_COPY`；`draftDirty` 保护 loading 窗口输入；`ports.agents().catch(() => [])` 降级；`project()` 以合并后 agents 求推荐；NewTaskScreen：`editable={!state.loading}` + 按钮禁用、预算本地中间态 `budgetText`（'' 与 0 分离）、GOAL_TEXT_MAX_LENGTH 注释更新。

### Task 9（B3-F1/F5 + F2/F6/F50）
- RED：5 个新用例 `not ok 10/11/12/23/24`。
- GREEN：`pnpm exec tsx --test packages/mobile-core/src/device/*.test.ts packages/mobile-core/src/inbox/*.test.ts` → `# pass 26 / # fail 0`。
- 实现要点：inbox `markedRead` 集合 + merge 强制 `read:true`（在途旧快照不回退已读投影）；空串游标归一 undefined；device revoke 409 → DEVICE_CONFLICT；`assertSoundRecord`（revision/scopeGeneration 非正安全整数 → DEVICE_BACKEND）；attemptRegister 成功返回前复检 lease。
- 注：`tsx --test 目录/` 形态会尝试加载不存在的 index.ts（与 HEAD 相同的工具行为，非本任务引入）；等价覆盖以显式 glob/逐文件运行呈现。

### Task 10（B3-F43/F11）
- RED：`deployment-origin.test.ts` ERR_MODULE_NOT_FOUND（模块不存在）；decide 用例 `not ok 14`（400 被翻译为 superseded）。
- GREEN：`pnpm exec tsx --test packages/api-client/src/mobile/*.test.ts` → `# tests 78 / # pass 74 / # fail 0 / # skipped 4`（opt-in integration）。
- 实现要点：新建 `deployment-origin.ts`（最严格合并版，含 inbox 版 hostname 检查）；7 处私有拷贝（devices/inbox/legacy-tasks/materials/resources/runtime/task-office）全部替换为共享 import；decide 只对 409 翻译 SUPERSEDED，400 透传原始 ApiError。既有用例「400 → INTERACTION_SUPERSEDED」断言按 F43 新语义改为「400 → HTTP_400 透传」。

### Task 11（B3-F31/F58 + F32-35/F55）
- RED：4 个新用例 `not ok 4/7/22/23`。
- GREEN：`pnpm exec tsx --test packages/mobile-core/src/material/*.test.ts apps/mobile/src/materials-view.test.ts` → `# tests 35 / # pass 35 / # fail 0`；`pnpm run typecheck:mobile` → PASS。
- 实现要点：`MAX_BLOB_ENTRIES = 6` LRU（命中重排、超限逐出最旧）；`mintGrant` 回验 `grant.artifact.id/version`（错位 → 新错误码 `MATERIAL_GRANT_MISMATCH`）；fetchBytes 复检实际字节 `PREVIEW_MAX_BYTES`；分享先查端口再铸造；decodeUtf8 回退改码点循环（正确 UTF-8）；evidence detail 截断 2000 字符（含省略号）；`MATERIAL_ERROR_COPY` 键类型收紧 `Record<MaterialErrorCode, string>` + 补 MISMATCH 文案。

### Task 12（B3-F52/F54/F57 + F10/F18/F36）
- RED：2 个新用例 `not ok 5/8`（末页返回数字 5 / stale 覆盖）。
- GREEN：四文件联跑 → `# pass 30 / # fail 0`；`pnpm run typecheck:mobile` → PASS。
- 实现要点：terminalLog 适配器末页转译（`next <= after → 省略键`）；`MaterialRemoteTerminalPage.nextCursor?` 与 `MaterialBackendTerminalPage.nextCursor?` 合同对齐；materials-view `run()` generation 令牌 + dispose 作废；native share Android 分支（url 并入 message）；fetchBlob 30s AbortController；resources-view refresh 重置 error。
- 差异：F18 的「notice 字段」在当前 resources-view.ts 不存在（状态仅 page/loading/error）；已实现可验证的「refresh 重置 error」半边，notice 半边无对应物。

### Task 13（B3-F21-F24 + F14/F66）
- RED：4 个新用例 `not ok 51/52/56/60`。
- GREEN：四文件联跑 → `# tests 141 / # pass 141 / # fail 0`；`pnpm run typecheck:mobile` → PASS。
- 实现要点：`questions: Record<string, string>` 按卡片隔离；无授权面显式空态（'请先登录并激活空间…'，loading 复位）；submitFollowUp 刷新失败 → `followUpState: 'idle'` + 「已发送，但刷新列表失败：…」；`LEGACY_ERROR_COPY`（含 TASK_OFFICE_SUPERSEDED 映射）；`history(taskId, {limit, before})` 透传（api-client + office + scenario backend 三层签名扩展）；`legacyHistory` 走 `settle(epoch, 'legacy', …)`；openHistory 请求 limit 100。

### Task 14（B3-F26/F28/F29/F27/F49）
- RED：3 个用例 `not ok 1/4/57`（并发 deviceId 3 次写盘 / 无 expo-crypto / 源级缓存键断言）。
- GREEN：`pnpm exec tsx --test request-id device-identity app-smoke attention-inbox-view inbox` → `# tests 73 / # pass 73 / # fail 0`；`pnpm run typecheck:mobile` → PASS；`pnpm install --filter @weknora/mobile` 安装 `expo-crypto@55.0.19` 并更新 lockfile。
- 实现要点：四个缓存（taskOffices/deviceRegistries/notificationInboxes/taskMaterials）统一 `deploymentScopeKey(origin, tenantId)` 为键 + `cachePut`（超 8 清最旧）；`registeredFor.add` 移到成功回调后 + `registrationAttempts` 会话内每 origin 最多 2 次；`openNotificationFromInbox` 参数 `Pick<InboxItem, 'notificationId' | 'deepLink'>`、删 `as InboxItem`（mobile-core `resolveTarget` 参数类型同步放宽——它只读 deepLink）；request-id 惰性 `require('expo-crypto').randomUUID`；device-identity in-flight 去重。
- 差异：**F48（deviceId 空串死守卫）在当前 composition.ts 不存在**（registerActiveDeviceIfPossible 仅有 `deviceId === undefined` 的活检查，无空串死分支）——无对应物，未做删除（如实记录）。

### Task 15（B3-F45/F60）
- RED：`not ok 3`（结构断言：无 finally、重入裸 await）。
- GREEN：`pnpm exec tsx --test apps/mobile/src/task-start-integration-smoke.test.ts apps/mobile/src/app-smoke.test.tsx` → `# tests 56 / # pass 55 / # fail 0 / # skipped 1`（live 用例 opt-in skip，语义正常）。
- 实现要点：`runTaskStartIntegration` total 化（主流程 try/catch/finally + `runtime.dispose()`）；第二次同 ID 重入独立 try/catch（失败 → `repeatSubmitSameRequest = 'failed'` + errorReason）。测试的结构断言按「重入在 try 内且 catch 记 failed」收紧（最初计划版正则会把合法的 try 内形态误判红）。

## 附录 B 批次验收（全部实跑）

1. **TS 全量**：`pnpm exec tsx --test "packages/domain/src/mobile/*.test.ts" "packages/mobile-core/src/**/*.test.ts" "packages/api-client/src/mobile/*.test.ts" "apps/mobile/src/**/*.test.ts*"` → `# tests 518 / # pass 509 / # fail 0 / # skipped 9`（skip 均为 opt-in integration，未伪造通过）。
2. **Go 四包**：`go test ./internal/application/service/ ./internal/application/repository/ ./internal/router/ ./internal/handler/` → handler `ok`；service/repository/router FAIL——失败计数 **135/276/4**，与 `git stash` 后干净 HEAD 基线（135/276/4）**完全一致**（见「预存在失败」节）。
3. **session 包**：`go test ./internal/handler/session/ -run 'TestGetWorkbench|TestListWorkbench' -v` → **20 个 `--- PASS` / `ok`**；全包 `--- FAIL` 计数 27 = 干净 HEAD 基线 27（全部为迁移冲突族，含计划点名的 `TestWorkbenchStartHTTPIntegrationAndIdentityIsolation`）。**失败集合未扩大。**
4. **窄版 tsc + 门禁收窄**：窄版命令 exit 0；`pnpm run typecheck:shared` 失败集合 = `packages/views/src/chat/mermaid.ts` ×2（agent-adoption 清零，较基线**收窄**；mermaid 为范围外预存在）。
5. **typecheck:mobile**：`pnpm run typecheck:mobile` → **PASS**。
6. **origin 私有拷贝**：行首 `function requireDeploymentOrigin` 计数 **0**（全目录含 export 共 1 处 = 共享导出本身；计划附录 B 该项 grep 的意图是私有拷贝清零，共享版按其 Step 3 说明以 `export function` 存在）。
7. **6 项 high 用户可见症状抽查**（逐条对应测试证据）：
   - 跨重启重试同 ID（F37/F78：重入零 createSession 零 POST）→ Task 6 用例 `replaying a bound intent after its log record was removed…`（`posts.length === 1 && sessions.length === 1`）+ Task 8 用例 `a recovered unresolved intent is retried with the same request id`。
   - 超大合法 goal 显式报错（F38）→ Task 7 用例（TASK_OFFICE_INVALID_INPUT，零落盘）。
   - 离线首开 New 屏草稿不丢（F59）→ Task 8 用例 `agents() failing offline must not lose the stored draft`。
   - bound 后 remove 失败不伪装失败（F64）→ Task 6 用例 `a failing intentLog remove after a bound start…`（phase=bound 正常返回）。
   - 重放不毒化 requestId（F78）→ 同 Task 6 第一条用例（zero-network bound 回执）。
   - typecheck 门禁内 agent-adoption 清零（F85）→ 验收第 4 项。

## 预存在失败（非本批次引入；验收口径「失败集合不扩大」已核对）

- **migrations/sqlite 000112 序号冲突**（`000112_task_grants.*` 与 `000112_agent_adoption_variants.*` 并存，merge d58675a67 引入）：任何走全量迁移轨道的 Go 测试夹具在打开迁移源时即报 `duplicate migration file: 000112_task_grants.down.sql`。实测失败计数（本批次后 vs `git stash` 干净 HEAD）：service **135 = 135**、repository **276 = 276**、router **4 = 4**、session **27 = 27**、handler 0。计划 Global Constraints 明示此为已知预存在问题，序号重编应升级为独立决策，不在本批次顺手修改。
- `packages/views/src/chat/mermaid.ts` ×2 类型错误：范围外预存在（typecheck:shared），保持。

## 计划外发现与处置（如实记录）

1. **Task 6 的 goalKey 记忆表**：计划 Produces 未覆盖「同 ID 换 goal 的重放」场景——纯 store-bound 回执会静默丢弃用户的新 goal 并返回旧 run 回执，且直接击穿既有用例「the same request id with a different input conflicts with zero network」。处置：office 内 `replayGoalKeys`（内存、remove 不清除）区分「同意图幂等重放」（回执）与「换意图复用 bound ID」（CONFLICT）；跨重启（无记忆）按计划 Review Focus 1 返回 bound 回执。
2. **Task 10 附录 B 第 6 项 grep 口径**：`grep -rn "function requireDeploymentOrigin" | wc -l` 返回 1（共享导出自身匹配），私有拷贝以行首锚定 grep 计数为 0。已在验收节注明。
3. **F48 无对应物**：composition.ts 现行代码无 deviceId 空串死守卫（详见 Task 14 节）。
4. **F18 的 notice 字段无对应物**：resources-view 现行状态无 notice；实现了 error 重置半边。
5. **F53/F47/F70/F71 等延期项**：按计划附录 A 延期，未动。
6. **Mimosa hook 提示**：commit 前扫描多次返回 scanner_enobufs（按兼容策略继续，不宣称项目安全）；request-id.ts 的 Math.random 兜底被标记弱随机——该兜底为计划明示保留的可用性权衡（expo-crypto CSPRNG 为主路径，mobile-core 的 fail-closed 纪律会让 throw 阻断全部提交），已在代码注释中记录。

## 结论

B3 计划的 6 项 high 全部修复（F85/F82/F84/F83/F78/F64/F79/F65/F51/F38/F42/F37/F41/F40/F59/F39/F44/F62 所在任务均 GREEN）；38 项 medium 中可精确落地的部分 + 附录 A 映射的全部折叠项已完成。批次验收 7 项通过；全部预存在失败集合与基线一致（未扩大）。
