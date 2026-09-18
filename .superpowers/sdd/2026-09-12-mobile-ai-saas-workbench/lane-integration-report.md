# Lane 集成报告 — 五条 mobile lane → codex/react-vue-parity-align

- 集成执行时间：2026-09-17（本会话补上届协调者未完成的集成义务）
- 集成起点：13cc1ab5（docs(sdd): resolve stash-pop conflict in W ledger）
- 最终 HEAD：**44710c6e**
- 集成顺序（严格串行 DAG）：W13 → W14 → W15 → W16 → W24
- 迁移号分配总表（sqlite 起点 max=000069，versioned 起点 max=000144）：

| Lane | sqlite | versioned | 裁决 |
| --- | --- | --- | --- |
| W13 mobile_devices | 000058（保留） | 000136→**000145** | sqlite 000058-000060 为 W33 集成时刻意留空的预留号；versioned 000136 与 execution_cleanup 撞号 |
| W14 mobile_notifications | 000059（保留） | 000137→**000146** | 同上；W14 带入的 versioned 000136_mobile_devices 与已 renumber 的 000145 内容一致，删除副本 |
| W15 delivery | 000060（保留） | 000138→**000147** | |
| W15 provider_state | 000061→**000070** | 000139→**000148** | sqlite 000061 与 000061_execution_cleanup 撞号 |
| W24 usage_binding | 000058→**000071** | 000136→**000149** | 与 000058_mobile_devices / 000136_execution_cleanup 撞号 |

---

## Lane 1 — W13（设备注册与账号切换撤销）

- **Merge commit**: `71294f7b`（7 commits 并入）
- **冲突（4 文件，全部语义并集解决）**：
  - `internal/container/workbench.go` — imports 并集（config + handler）
  - `internal/router/router.go` — MobileVoiceHandler(W30) + MobileDeviceHandler(W13) 并存；RegisterMobileVoiceRoutes + RegisterMobileDeviceRoutes 并存
  - `internal/router/routes_workbench.go` — 两个路由注册函数并存
  - `internal/database/migration_sqlite_versioned_schema_test.go` — 表清单并集（execution_cleanup* + mobile_devices）；`expectedSQLiteMigrationVersion` 66→**69**。注：HEAD 常量 66 是 W30 voice 集成时遗留的过时值（实际 sqlite max 已 69），本次按「常量=真实最大迁移号」语义一并修正
- **迁移 renumber**: versioned 000136_mobile_devices → 000145_mobile_devices（内容含 uq/idx 索引，与 sqlite 000058 对应）
- **Focused 测试**（来自 task-W13-final-review.md）:
  - `go test ./internal/application/repository -run 'TestMobileDevice'` — **PASS**
  - `go test ./internal/handler -run 'TestMobileDevice'` — **PASS**
  - `go test ./internal/database -run 'TestSQLiteMigrations*'` — FAIL（000055 族 dirty，**merge 前基线同样失败**，既有）

## Lane 2 — W14（事务事件到通知 Outbox）

- **Merge commit**: `b6a701e9`（17 commits 并入；含测试适配 amend）
- **冲突（8 文件）**：
  - `internal/container/container.go` — W26 ArtifactVersionStore + W14 Notification* providers 并集
  - `internal/container/workbench.go` / `internal/router/router.go` — voice 侧内容保留（W14 基底早于 W30，未见 voice）
  - `internal/application/repository/mobile_device_test.go` — **全取 HEAD**：验证 W14 相对其基底未演进此文件（W14 版= W13 验收版），HEAD=W13 lane 终版（race-fix：磁盘库 + Deterministic 测试）
  - `apps/mobile/.../session.tsx` / `session.test.tsx` / `registration.test.tsx` — **全取 HEAD**：逐行 diff 验证 HEAD 为 W14 语义的超集（W14 的 pendingStore/flushPendingRevocations/nativeDeviceId 等均已包含在 W13 终版中）
  - `migration_sqlite_versioned_schema_test.go` — 表清单并集（+intents/checkpoints）、注释合并、常量 69
- **迁移 renumber**: versioned 000137_mobile_notifications → 000146；删除 W14 带入的重复 000136_mobile_devices（与 000145 逐字节一致）
- **测试适配（amend 进 merge commit）**: W14 的 `mobile_notification_test.go` 内嵌 minimal `mobile_devices` 表 workaround（分支上无完整 W13 迁移时的兼容层）与集成后完整 000058 表冲突（`NOT NULL constraint failed: platform`）——4 处 INSERT 补全 platform/token_ciphertext/token_hash 列（该形态与同文件后期测试 151/195/233 行一致）
- **Focused 测试**:
  - `go test ./internal/application/service/workbench -run 'TestNotificationDelivery'` — **PASS**
  - `go test ./internal/application/repository -run 'TestMobileDevice|TestNotification'` — 修复后 **PASS**

## Lane 3 — W15（供应商投递、回执与重试）

- **Merge commit**: `061a442a`（27 commits 并入；自动合并无文本冲突）
- **迁移 renumber（golang-migrate 重复版本号，git 不报冲突但运行时会拒绝）**:
  - sqlite 000061_mobile_notification_provider_state → **000070**（与 000061_execution_cleanup 撞号）
  - versioned 000138_mobile_notification_delivery → **000147**、000139_mobile_notification_provider_state → **000148**（与 execution_cleanup_artifacts/artifact_intents 撞号）
  - versioned 000136/000137 的 renumber 由 W14 merge 的历史正确传递（merge-base 已前移至 W14 tip，git 三方正确保留删除/改名）
- **文档归位**: W15 的 docs commit（9818f339）把 10 个 `task-W15-*.md` 放在仓库根目录，git mv 至 `.superpowers/sdd/2026-09-12-mobile-ai-saas-workbench/`（与其他 lane 一致）
- **schema 测试维护**（W15 lane 未维护该测试，常量停留在 57）: 表清单 +mobile_notification_provider_state、注释 +delivery/provider_state、常量 70
- **Focused 测试**（来自 task-W15-*.md）:
  - `go test -race ./internal/notification -count=1` — **PASS**
  - `go test ./internal/application/service/workbench -run 'Test(Push|NotificationWorker|NotificationDelivery|NotificationRetry|NotificationPermanent|NotificationInvalid|NotificationProvider)'` — **PASS**（该域在 W15 分支上被 lane 既有编译损坏 `d.dispatch.ReconcileUnknown` undefined 阻塞、从未运行；集成侧编译通过后全部通过）
  - repository 通知域回归 — **PASS**

## Lane 4 — W16（安全深链与待处理卡）

- **Merge commit**: `0d9baebd`（2 commits 并入，纯前端）
- **冲突（3 文件）**：
  - `apps/mobile/sources/app/(app)/index.tsx` — W16 加的 import 与 auto-merge 保留的重复，删冲突块（保留一份）
  - `apps/mobile/sources/app/_layout.tsx` — 语义合并：保留 HEAD 的 native execution storage gate（W34）+ W16 的 NotificationRouter 包裹
  - `apps/mobile/sources/weknora/workbench/WorkbenchScreen.tsx`（add/add）— **裁决保留双方行为**：HEAD 完整分组执行列表（W11 已验收形态，09aba1ba）+ 在屏幕容器顶部挂 W16 的 `<PendingNotificationCard />`（W16 核心交付物）
- **迁移**: 无
- **Focused 测试**:
  - `tsx --test sources/weknora/notifications/deep-link.test.ts` — **PASS**（2 pass / 0 fail）
  - `vitest run NotificationRouter.test.tsx` — 3 failed / 1 passed：**lane 既有**（在 W16 分支原始 worktree 以相同依赖跑出完全相同失败；W16 审查时环境 blocked 从未执行过）。记录为遗留，非合并引入

## Lane 5 — W24（受控工具、可信用量与预算树接线）

- **Merge commit**: `05725468`（7 commits 并入）
- **冲突（5 文件）**：
  - `internal/container/agent_runtime.go` — 仅注释差异（BASE breakage fix 注释），保留 HEAD 注释
  - `internal/application/service/workbench/remote_dispatch.go` — 取 W24 侧：`dispatchRemote` 命名 + 新增 `ReconcileLateUsage`（HEAD 的 `dispatchFenced` 仅命名差异，无外部引用）
  - `internal/application/service/workbench/admission.go` —
    - `StartInput` 并集：保留 W11 的 `SpaceID`（深链语义，不入 request hash）+ W24 的 `Binding *TrustedAdmissionBinding`
    - snapshot 单一化：采用 W24 的含 binding 字段版本（HEAD 版为重复声明会编译失败），并入 `"space_id": in.SpaceID`
  - `internal/container/workbench.go` — **NewWorkbenchAdmissionCoordinator 融合**：W24 的 `NewAdmissionCoordinatorWithBinding(NewDurableTaskBudget, NewDatabaseAdmissionBindingResolver)` + W34 的 `SetAdmissionGate(NewWorkbenchCapabilityGate(cfg))`；签名合并为 `(cfg, db, runs, targets)`（dig 容器已 provide 全部依赖）
  - `internal/application/service/workbench/admission_test.go` — imports 并集（execution+types+require）；测试并集（W11 space-snapshot 测试 + W24 binding/BYOK 测试）
- **commercial execution.go 复核**: W24 相对其基底未改动该文件（`git diff 3a9bd435..codex/mobile-w24 -- internal/commercial/execution.go` 为空）；W30 的 ServiceVoice/audio_seconds/image_inputs 白名单完整保留，无实际交叠
- **迁移 renumber**: sqlite 000058_execution_target_usage_binding → **000071**、versioned 000136 → **000149**
- **schema 测试维护**: execution_targets 列清单 +usage_binding_json、常量 71
- **Focused 测试**（来自 task-W24-final3-review.md）:
  - `go test ./internal/application/repository -run 'TestExecutionTargetStorePersistsServerUsageBinding|TestPersistUsageBindingOverridesUntrustedSnapshotFields'` — **PASS**
  - `go test ./internal/application/service/workbench -run 'Test(ServerAdmissionBindingResolverRejectsRequestScopedBinding|ProductionAdmissionPersistsPlatformBYOKParentBinding|RemoteDispatcher|RemoteUsage)'` — **PASS**
  - `go test -race ... -run 'TestRemoteUsage|TestRemoteDispatcher|TestServerAdmissionBinding|TestProductionAdmission|TestAdmissionSnapshot'` — **PASS**
  - `go test ./internal/execution` — **PASS**

---

## 集成修复 commit（merge 序列之后，2 个）

1. `a6adf7a1` — **fix(integration): restore voice routes and workbenchservice import lost in W14/W15 auto-merge**
   - `RegisterMobileVoiceRoutes` 在 W14 冲突解决时被误删（本报告执行过程中的自查发现并恢复，函数体从 71294f7b 版本恢复）
   - `workbenchservice` import 在 W15 merge 的 auto-merge 中静默丢失（W15 merge commit 061a442a 原始形态编译不过，因当时仅跑测试包未暴露；container 包 build 抓出后补回）
2. `44710c6e` — **test(notifications): satisfy agent_run_events FK in paging fixture**
   - `TestEventRunKeysPageContinuesPastFirstPage`（W14 引入）在 W14/W15 分支上从未运行过（被 lane 既有编译损坏阻塞），fixture 直接插 agent_run_events 无父行，违反 000014 迁移的 FK；修复为每个 run 先插最小 agent_runs 父行（含 deadline NOT NULL）

## 全量验证

| 检查 | 结果 |
| --- | --- |
| `go build ./internal/... ./cmd/...` | **PASS**（仅 ld 重复库 warning，无害） |
| `go vet`（container/router/repository/handler/workbench/database/execution/notification） | **PASS**（0 输出） |
| `go test ./internal/application/repository` | 2 失败 = 基线 2 失败（TestCraftVersionsMigrationDownDropsVersionTables、TestExecutionDispatchSQLiteMigrationHead——既有） |
| `go test ./internal/handler` | 1 失败 = 基线 1（TestOIDCMobileStartCallbackExchangeIsOneTime——已知 OIDC 基线） |
| `go test ./internal/handler/session` | 19 失败 = 基线 19（逐名 diff 完全一致，000055 族既有） |
| `go test ./internal/application/service/workbench ./internal/notification ./internal/execution` | **全部 PASS** |

基线对比方法：临时 worktree 检出 13cc1ab5 跑同命令，失败清单排序逐行 diff。

## 前端测试遗留（记录）

- `session.test.tsx`（7 failed）与 `NotificationRouter.test.tsx`（3 failed）在集成分支与对应 lane 分支（w13/w16 worktree 原始检出 + 共享 node_modules）结果完全一致——均为 lane 既有状态（lane 审查时 tsx/vitest 环境被 blocked，从未真正执行）。React 组件测试需 DOM 环境，本 worktree 的 vitest 配置不含 jsdom。非合并引入。

## 外部提交记录（如实报告）

- `397ce76f feat(parity): R453 feishu prereq port, shared-glob repair (+144 gated tests), integrity audit` 出现在 W14 与 W15 两个 merge commit 之间：为外部 parity 进程在集成窗口（19:29:06）自行提交进本分支（仅改 docs/migrations/*.md + package.json + SDD 文档，作者 wuyj）。本集成未主动纳入外部文件；该 commit 无 Go 代码影响，W15 起所有 build/vet/测试均在含它的历史上通过，故保留未动。
- 工作树自始至终干净，未发生 stash push/pop 需求。

## 阻塞点

**无。** 五条 lane 全部集成完成，无未解决语义冲突。

## 最终历史（13cc1ab5 之后）

```
44710c6e test(notifications): satisfy agent_run_events FK in paging fixture
a6adf7a1 fix(integration): restore voice routes and workbenchservice import lost in W14/W15 auto-merge
05725468 Merge branch 'codex/mobile-w24'
0d9baebd Merge branch 'codex/mobile-w16'
061a442a Merge branch 'codex/mobile-w15'
397ce76f feat(parity): R453 feishu prereq port（外部进程提交，见上）
b6a701e9 Merge branch 'codex/mobile-w14'
71294f7b Merge branch 'codex/mobile-w13'
13cc1ab5 （集成起点）
```
