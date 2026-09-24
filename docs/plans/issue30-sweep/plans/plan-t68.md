# T38: Taro Adapter 复用深 Module 与维护模式门槛（Issue #68）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Taro 小程序（apps/miniprogram）通过 `@weknora/mobile-core` 的同一 Task Office / Resource Shelf / Task Material Interface 跑关键 scenario（列表/详情/发起/审批/资源/材料），MobileRuntime 成为小程序唯一会话编排器，删除旧原生 `miniprogram/` 编排树，并以可执行门槛（平台纯度测试 + 编排树唯一性测试 + 深模块依赖断言）固化「进入维护模式」的判定。

**Architecture:** 复用 B1–B3 已交付的深模块与 remote Adapter 零改动：小程序组合根把 `createMobileRuntime` 接到 Taro 平台 Adapter（storage 凭据仓、weapp 授权 REST/SSE 通道、blob 抓取），再以 `createTaskOfficeRemote` / `createMobileResourceRemote` / `createMobileMaterialRemote`（api-client，全部复用既有 ClientRequest 通道）装配 TaskOffice/ResourceShelf/TaskMaterial——与 `apps/mobile/src/composition.ts` 同一模式、不同平台 Adapter。小程序内手写的 AuthCoordinator/ScopeGuard 编排与 workbench controller（startTask/listTasks/watchExecution/pendingIntent/rejectInteraction）被整体替换（replace-dont-layer，module-seams §14）；旧原生 `miniprogram/` 根树删除。lease 只能由 `createMobileRuntime` 铸造（`RuntimeScopeLease` 包内私有，`packages/mobile-core/src/runtime/scope-lease.ts:17-25`），因此深模块复用必然要求 MobileRuntime 拥有会话——这是单一凭据所有权的架构必然，不是可选项。

**Tech Stack:** TypeScript（`packages/mobile-core`、`packages/api-client`、`apps/miniprogram` Taro 4.2.1 + React 18.3.1）、node:test（小程序 `--experimental-transform-types --test`；mobile-core `tsx --test`）。所有命令在 worktree 根 `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep` 执行；`pnpm install` 已就绪（根/各包 node_modules 均在，见「差异记录」）。本计划作者已实跑以下基线：`pnpm exec tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts` 0 fail；`cd apps/miniprogram && node --experimental-strip-types --test tests/*.test.mjs` 41 pass / 0 fail；`node --experimental-transform-types -e "import('../../packages/mobile-core/src/index.ts')…"` IMPORT_OK（`--experimental-strip-types` 下 STRIP_FAIL：mobile-core 错误类使用 constructor 参数属性，必须 transform-types——Task 2 处理）；`cd apps/miniprogram && npm run typecheck` 存在 13 个**先在**错误（全部 `src/features/account/pages.tsx`，`CommercialSummary` 无 `stale/paid_until` 字段——账户域历史问题，非本计划引入，见差异记录 D4，本计划以「错误集合不得增长」为门）。

**Spec:** `docs/plans/issue30-sweep/issues/issue-68.md`（验收标准原文）；`docs/specs/2026-09-20-mobile-module-seams.md`（§2.3 拓扑、§13 测试策略、§14 迁移顺序、§15 Deletion test）；`docs/specs/2026-09-20-mobile-ai-office-design.md`（Testing Decisions，151-163 行）。

## Global Constraints

以下约束逐字引用批准需求与 spec，每个任务隐含遵守：

1. 「业务 Module 不依赖 Expo 或 WeChat。」（Issue #68 验收标准 1 / 任务清单 1）
2. 「新旧编排器不长期并存，迁移采用 replace-dont-layer。」（Issue #68 验收标准 2 / 任务清单 2）
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」（Issue #68 验收标准 3 / 任务清单 3）
4. 「迁移采用 replace-don't-layer：当 Screen 全部经过新 Interface 且 scenario tests 覆盖不变量后，删除旧公开 controller，不保留新旧两套编排器。」（module-seams §14）
5. 「结论：采用此方案。它让业务变化集中，并允许 Expo Screen、Taro 小程序和测试通过同一 Interface 使用行为。」（module-seams §2.3——六个深 Module：Mobile Runtime / Task Office / Resource Shelf / Task Material / Voice Room / Scoped Vault；App Shell 只是 composition root 与 presentation Adapter）
6. 「5. 用 Taro Adapter 跑同一 Task Office scenario，证明 Interface 不依赖 Expo；」（module-seams §14 迁移顺序第 5 步）
7. 「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」「Remote-owned dependencies use in-memory scenario Adapters for Module tests and real HTTP/SSE contract tests for production Adapters.」「True external dependencies such as APNs, FCM, WebRTC, system audio and system share use mock or scripted Adapters at the Port and real-device acceptance separately.」（ai-office-design Testing Decisions）
8. 「现有小函数测试在对应 Interface-level tests 建立后删除或降为少量纯 policy tests。不要在新 Interface 测试外再叠一层 Screen 对同一内部行为的白盒测试。」（module-seams §13）
9. Mimosa 安全约束（本仓库会话级约束，与移动端网络面直接相关）：「服务端请求 URL 时：仅允许 http/https；发请求前校验 host，并拒绝 localhost、环回、私有和保留地址。」「数据库查询时：所有外部输入必须使用参数绑定。」「配置凭据时：只从环境变量或密钥服务读取；源码、示例和测试都不要写入可用的凭据字面量。」
10. 凭据纪律（module-seams 深模块不变量 + B1 #32 既有设计）：token 不出 MobileRuntime；平台直连通道（上传/下载）只能「用时现读」Runtime 落盘的凭据，永不缓存、永不再造第二条刷新路径。
11. 前置批次接口逐字复用，不修改 `packages/mobile-core` / `packages/api-client` 任何既有源文件（唯一例外：新增 `packages/mobile-core/src/platform-purity.test.ts` 一个测试文件，属纯增量，见并行说明）。

## Review Focus

spec 是愿景文档：它说软件必须做什么，不列举它将遇到的一切输入；spec 对某类输入沉默不等于允许该输入弄坏程序。以下五类最可能咬到真实用户的输入/失效模式，各配一个归属于对应任务的测试（每条都已在任务步骤中落为真实断言）：

1. **刷新轮换后的双编排器残留**：若旧 AuthCoordinator/workbench controller 未删净，两条刷新路径会互相作废对方的 refresh token（服务端轮换后旧 token 失效），表现为「聊着聊着被登出」。→ Task 6 `orchestrator.test.mjs` 断言旧类已删 + Task 5 删除后 typecheck 全绿；Task 3 assembly 测试断言唯一凭据键只有 Runtime 一个写者。
2. **迟到 scope 结果回填**：切租户/登出后旧响应把已撤销 scope 的数据写进新投影。→ Task 4 office-assembly「tenant switch 使旧 TaskOffice SCOPE_CHANGED fail closed」+ Task 3 assembly「a response landing after a scope change is discarded」。
3. **401 重试语义变化误伤非幂等写**：authorizedRequest 对 401 刷新后重放（含 POST）——401 是「未认证即拒绝」，服务端未执行请求，重放安全；真正的危险是**歧义失败**（网络中断/超时）被重放造成重复提交。→ Task 3 assembly「NETWORK_ERROR on POST is never retried」断言 posts===1。
4. **平台直连通道读到过期 token**：Runtime 轮换后上传/下载若缓存旧 token 会 401。→ Task 2 platform-adapters「rotation write is immediately visible to just-in-time readers」+ 预检（currentBearerToken 先走一次授权 GET 触发 refresh-once）。
5. **第二棵小程序编排树复活**：有人重新引入旧原生树或第二个 miniprogram 包，维护模式门槛静默失效。→ Task 6 `orchestrator.test.mjs` 三断言（旧树不存在 / workspace 恰一个 miniprogram 条目 / 深模块依赖在位）。

---

## 与调查结论的差异记录（以代码现状为准）

- **D1（node_modules 已安装）**：调查摘要称「worktree 未安装 JS 依赖（根目录无 node_modules）」。当前 worktree 根、`apps/miniprogram`、`packages/mobile-core` 均有 node_modules，且基线测试实跑通过（见 Tech Stack）。本计划所有命令可直接执行。
- **D2（深模块已存在）**：调查称「mobile-core 仅含 Mobile Runtime 一个 Module」。当前 HEAD 已含 task-office/shelf/material/vault/device/inbox 全部深模块（`packages/mobile-core/src` 59 个 .ts）。缺口收窄为：**apps/miniprogram 尚未依赖 mobile-core、关键 scenario 未走深模块**（`apps/miniprogram/package.json` dependencies 无 `@weknora/mobile-core`）。
- **D3（旧树的测试早已缺失）**：`miniprogram/package.json` 的 `test` 脚本指向 `../tests/miniprogram/*.test.js`，而 `tests/miniprogram/` 目录**已不存在**（`ls tests/` 仅 mobile-v2/native-agent）——旧树测试已先行失效，删除无测试损失。CI（`.github/workflows/frontend.yml:125-148`）只引用 workspace 包 `@weknora/miniprogram`（即 apps/miniprogram），删旧树不需改 CI。
- **D4（typecheck 基线非绿）**：`apps/miniprogram` 现存 13 个 tsc 错误，全部在 `src/features/account/pages.tsx`（使用 `CommercialSummary` 上不存在的 `stale/paid_until` 等字段；contracts 注释明示这些字段「served by no endpoint」）。属账户域历史问题、非本 Issue 范围（#68 是 Task/Resource/Material 复用）；本计划不静默修复也不静默忽略，以「错误集合不得增长」为门（Task 8 步骤含精确过滤命令）。
- **D5（strip-types 不可用）**：mobile-core 错误类使用 constructor 参数属性（如 `packages/mobile-core/src/task-office/task-office-errors.ts:24`），`node --experimental-strip-types` 报「TypeScript parameter property is not supported in strip-only mode」。已实跑验证 `--experimental-transform-types` 可正常加载 mobile-core（Node v22.22.3）。Task 2 切换测试运行器 flag。

## Consumes（前置批次产出接口，全部在当前 HEAD 亲眼核实）

- **#32**：`createMobileRuntime(ports: MobileRuntimePorts): MobileRuntime`（`packages/mobile-core/src/runtime/mobile-runtime.ts:282` 起）；`MobileRuntimePorts`（`ports.ts:85-110`：`credentialStore`/`remoteFor(deployment): RuntimeRemote`/`clientVersion`/`authorizedTransport?: (deploymentOrigin) => AuthorizedTransport`/`authorizedStream?: (deploymentOrigin) => AuthorizedStreamTransport | undefined`/`resourceShelf?: { remoteFor(origin): ResourceRemote }`）；`RuntimeRemote` 八方法（`ports.ts:28-36`）；`MobileRuntime.authorizedRequest(input: RuntimeAuthorizedRequest): Promise<unknown>`（`mobile-runtime.ts:383-388`，401 refresh-once 重放语义）；`authorizedEventStream`（`:389-412`，pre-stream 401 刷新一次、迟到 scope 拒绝、无流通道 `RUNTIME_STREAM_UNAVAILABLE` fail closed）；`activateTenant(tenantId: string)`；`StoredCredential = { token: string; refreshToken: string }`。
- **#33**：`createMobileResourceRemote(options: { origin: string; request: Request }): MobileResourceRemote`（`packages/api-client/src/mobile/resources.ts:45`，GET `/api/v1/agents`、`/api/v1/knowledge-bases`、`/api/v1/apps/connections`）；`ResourceShelfHandle`（`packages/mobile-core/src/shelf/types.ts:37-42`：browse/selection/subscribe/close）；`MobileRuntime.resourceShelf(): ResourceShelfHandle | undefined`。
- **#34**：`createTaskOfficeRemote(options: { origin: string; request: Request; stream?: (input: ClientRequest, onChunk) => Promise<void> })`（`packages/api-client/src/mobile/task-office.ts:50-241`：overview/list/createSession/start/lookup/archive/restore/detail/stream/inbox/decide）；`TaskOffice` 全方法（`packages/mobile-core/src/task-office/task-office.ts:180-194`）；`MobileRuntime.authorizedRequest`（见上）。
- **#35**：`TaskOffice.open(input: { taskId: string; runId: string }): TaskHandle`（hydrate/view/updates/resync/close，`packages/mobile-core/src/task-office/task-detail.ts:72-78`）；`TaskDetailView`（lifecycle/runStatus/attention/connection/interruption/timeline/duplicateSeqs，`:54-71`）；`MobileRuntime.authorizedEventStream`；api-client remote 的 409→`TASK_STREAM_CURSOR_EXPIRED` 映射（`task-office.ts:185`）。
- **#36**：`TaskOffice.start(goal: TaskOfficeGoal, options?: { requestId?: string }): Promise<TaskStartReceipt>`（Start POST 前耐久落盘意图、同 ID 受控重入）；`TaskOfficePorts.intentLog?: SubmissionIntentLog` / `newRequestId?: () => string`（`task-office.ts:160-170`）；`reconcilePending(): Promise<TaskStartReceipt[]>`（`:192`）；`createSession`（POST `/api/v1/sessions`）/`start`（POST `/api/v1/workbench/executions`）/`lookup`（GET `/requests/:id`）。
- **#38**：`TaskOffice.inbox(): Promise<InboxView>` 与 `decide(input: AttentionDecisionInput): Promise<AttentionDecisionReceipt>`（recorded|delivery-unknown|superseded|gone）；`INTERACTION_ACTIONS` 冻结矩阵（`packages/mobile-core/src/task-office/attention-inbox.ts:20-24`：tool_approval=[approve,reject]、budget=[extend]、recovery=[retry,provide_result,terminate]）；inbox wire = GET `/api/v1/workbench/interactions?limit=`、decide wire = POST `/api/v1/workbench/executions/interactions/:id/decisions`（`packages/api-client/src/mobile/interactions.ts:71-82`）。
- **#46**：`createTaskMaterial(ports: TaskMaterialPorts): TaskMaterial`；`TaskMaterialPorts = { remote: MaterialBackendPort; blob: BlobFetchPort; share?: SharePort }`（`packages/mobile-core/src/material/ports.ts:59-63`）；`TaskMaterialHandle`（index/open/act/subscribe/close，`types.ts:72-78`）；`createMobileMaterialRemote(options: { origin: string; request: Request })`（`packages/api-client/src/mobile/materials.ts`，GET `…/artifacts`、POST `…/artifacts/:index/signed-url`、GET `…/terminal-log`、GET `…/snapshot`）；`MaterialIndex`/`MaterialView`/`PREVIEW_MAX_BYTES`。
- **既有平台层**（apps/miniprogram，本计划改造对象）：`createWeappTransport(network)`（`platform/transport.ts:44-134`，send/sendBinary/sendMultipartFile/stream）；`storage`/`clearPrivateCache`（`platform/storage.ts`）；`ScopeGuard`（`core/scope.ts`）；taro 测试替身 `tests/helpers/taro-stub.mjs`（`registerHooks` 重定向 `@tarojs/taro`）。

## Produces（本计划对外产出，供 #71 等后续消费）

1. `apps/miniprogram` 依赖 `@weknora/mobile-core: workspace:*`；`services/runtime.ts` 导出 `runtime: MobileRuntime`（唯一会话编排器）与 `auth: TaroSessionFacade`（`bootstrap/login/switchTenant/logout/snapshot/subscribe/credential/scope/abortSubscriptions`）。
2. `services/mobile-office.ts`：`activeTaskOffice(): TaskOffice | undefined`、`openActiveMaterial(): TaskMaterialHandle | undefined`、`activeResourceShelf(): ResourceShelfHandle | undefined`、`resolveTaskForRun(runId: string): Promise<{ taskId: string; runId: string }>`。
3. `platform/credential-store.ts`：`credentialKeyOf(origin)`、`readStoredCredential(store, origin): StoredCredential | undefined`、`adoptLegacyCredentials(store, origin): void`、`createTaroCredentialStore(store, origin): CredentialStore`。
4. `platform/authorized-channels.ts`：`createAuthorizedRequestChannel(request): AuthorizedTransport`、`createAuthorizedStreamChannel(network, origin): AuthorizedStreamTransport`、`createTaroBlobFetch(network): BlobFetchPort`。
5. `platform/intent-log.ts`：`createTaroIntentLog(store): SubmissionIntentLog`。
6. 门槛测试：`packages/mobile-core/src/platform-purity.test.ts`（AC1）、`apps/miniprogram/tests/orchestrator.test.mjs`（AC2/维护模式门槛）、`apps/miniprogram/tests/office-assembly.test.mjs`（AC3 Interface 级场景）。
7. opt-in 真实集成证据入口：`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD`（+可选 `WEKNORA_MOBILE_TEST_START_TASK=1` 探针），`apps/miniprogram/tests/integration/miniprogram-office-integration.test.mjs`。
8. 旧原生 `miniprogram/` 根树删除；`website-docs/05-clients/04-miniprogram.md` 重写为 Taro 版事实源。

## File Structure（任务分解的文件地图）

新增（本计划独有，并行安全）：

| 文件 | 职责 |
|---|---|
| `packages/mobile-core/src/platform-purity.test.ts` | AC1 纯度门槛（依赖+导入扫描） |
| `apps/miniprogram/src/platform/credential-store.ts` | Taro storage 凭据仓（Runtime 唯一写者；legacy 形态收养） |
| `apps/miniprogram/src/platform/authorized-channels.ts` | 授权 REST/SSE 通道 + 免凭据 blob 抓取 Adapter |
| `apps/miniprogram/src/platform/intent-log.ts` | 耐久意图日志（SubmissionIntentLog over storage） |
| `apps/miniprogram/src/services/session.ts` | SessionView 门面（Runtime snapshot→UI 会话视图 + ScopeGuard 桥） |
| `apps/miniprogram/src/services/mobile-office.ts` | 深模块组合根（TaskOffice/TaskMaterial 记忆化工厂） |
| `apps/miniprogram/src/services/office-views.ts` | 纯呈现映射（状态标签/回执文案/收件箱过滤） |
| `apps/miniprogram/tests/platform-adapters.test.mjs` | 平台 Adapter 单测 |
| `apps/miniprogram/tests/office-assembly.test.mjs` | 关键 scenario（真实源码+taro-stub 后端，最高稳定 Interface 级） |
| `apps/miniprogram/tests/office-views.test.mjs` | 纯映射单测 |
| `apps/miniprogram/tests/orchestrator.test.mjs` | replace-dont-layer / 维护模式门槛 |
| `apps/miniprogram/tests/helpers/node-taro.mjs` | fetch 底座 Taro 替身（真实网络集成证据用） |
| `apps/miniprogram/tests/integration/miniprogram-office-integration.test.mjs` | opt-in 真实集成证据 |

修改（精确位置，全部在 apps/miniprogram 与两处文档，无其他包共享文件）：

| 文件 | 修改 |
|---|---|
| `apps/miniprogram/package.json` | dependencies 加 `@weknora/mobile-core`；test 脚本换 `--experimental-transform-types` 并纳入 integration 目录（Task 7 时） |
| `apps/miniprogram/src/platform/transport.ts:83-132` | `stream`（函数 :83 起，注释 :79）提升为导出的 `streamWeapp(network, …)`（授权 SSE 通道复用同一流实现；`return {send:…}` 委托行在 :133） |
| `apps/miniprogram/src/services/runtime.ts` | 全文重写为 MobileRuntime 组合根（导出面保持：auth/client/executions/stream/chatStream/apiOrigin/logout/stopSubscriptions，新增 runtime/network 导出） |
| `apps/miniprogram/src/core/auth.ts` | 删 `AuthCoordinator` 类与 `AuthPort`；保留 `SessionView`/`normalizeApiOrigin`/`isBearer`/`anonymous`（components/ui.tsx 与文件通道仍消费） |
| `apps/miniprogram/src/core/intent.ts` | 删 `PendingIntent`；保留 `ValueStore`/`requestId`（storage 与 newRequestId 端口消费） |
| `apps/miniprogram/src/features/execution/pages.tsx` | TasksPage/ExecutionPage/ApprovalPage/ArtifactPage 四页迁移到 TaskOffice/TaskMaterial |
| `apps/miniprogram/src/features/home/pages.tsx` | AgentsPage 改 shelf browse；AgentPage 提交改 `office.start`（HomePage 不动） |
| `apps/miniprogram/tests/assembly.test.mjs` | 语义更新（401 刷新重放/唯一凭据键/bootstrap 恢复/切租户） |
| `apps/miniprogram/tests/core.test.mjs` | 删 execution/PendingIntent 白盒用例；加 normalizeApiOrigin 纯函数用例 |
| `website-docs/05-clients/04-miniprogram.md` | 重写为 Taro 应用文档（旧原生实现删除记录） |

删除：

| 路径 | 理由 |
|---|---|
| `apps/miniprogram/src/services/workbench.ts` | 手写 Task controller，被 TaskOffice 整体替换（replace-dont-layer） |
| `apps/miniprogram/src/core/execution.ts` | 手写快照/SSE 投影，被 TaskDetailView 替换 |
| `apps/miniprogram/tests/auth.test.mjs` | AuthCoordinator 白盒测试，语义已由 mobile-core runtime 测试 + 新 assembly 覆盖（module-seams §13） |
| `miniprogram/`（30 个 git 跟踪文件） | 旧原生编排树（AC2） |

并行批次说明：本计划与同批其余计划并行实施、独立 worktree 后合并。除 `packages/mobile-core/src/platform-purity.test.ts`（纯新增文件，不改任何既有行）外，全部改动收敛在 `apps/miniprogram/**`、`website-docs/05-clients/04-miniprogram.md` 与 `miniprogram/` 删除——这些路径没有任何其他批次计划触碰（#39/#40/#43/#45/#52/#56 目标为 apps/mobile、packages 与 Go 域）。Task 5（页面迁移+删 controller）与 Task 3（runtime 重写）之间的顺序保证中间态可编译：workbench.ts 在 Task 3 后仍工作于新 client 之上，Task 5 才删除它。

---

### Task 1: mobile-core 平台纯度门槛（AC1 可执行门槛）

**Files:**
- Create: `packages/mobile-core/src/platform-purity.test.ts`

**Interfaces:**
- Consumes: 无（纯文件扫描）。
- Produces: 可执行断言——`@weknora/mobile-core` 的 dependencies/devDependencies 不含 `expo*`/`@tarojs/*`/`react-native*`/`weixin*`/`wechat*`/`wx-*`，`src/**/*.ts` 源码无这些平台的 import/require。这是 Issue #68 验收标准 1「业务 Module 不依赖 Expo 或 WeChat」的回归门槛（当前为真，测试把它钉住）。

- [ ] **Step 1: 写门槛测试（当前应为 PASS 的回归钉）**

创建 `packages/mobile-core/src/platform-purity.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const srcRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const packageRoot = path.resolve(srcRoot, '..');

/**
 * Issue #68 AC1「业务 Module 不依赖 Expo 或 WeChat」的可执行门槛。
 * 业务 Module = mobile-core 六个深 Module（module-seams §3）。平台（Expo/RN/微信/Taro）
 * 只允许出现在 App Shell 的 Adapter 里；一旦有人把平台依赖引入本包，本测试立即失败。
 */
const FORBIDDEN_DEPENDENCIES = [/^expo($|\/)/, /^expo-/, /^@tarojs\//, /^react-native($|\/)/, /^weixin/, /^wechat/, /^wx-/];
const FORBIDDEN_IMPORTS = [/^expo($|\/)/, /^expo-/, /^@tarojs\//, /^react-native($|\/)/, /^weixin/, /^wx\//];

function tsFiles(dir: string): string[] {
  const out: string[] = [];
  for (const name of readdirSync(dir)) {
    const full = path.join(dir, name);
    if (statSync(full).isDirectory()) out.push(...tsFiles(full));
    else if (/\.ts$/.test(name)) out.push(full);
  }
  return out;
}

test('mobile-core declares no Expo/WeChat/Taro/React-Native dependency (issue #68 AC1 gate)', () => {
  const pkg = JSON.parse(readFileSync(path.join(packageRoot, 'package.json'), 'utf8')) as {
    dependencies?: Record<string, string>;
    devDependencies?: Record<string, string>;
  };
  const declared = [...Object.keys(pkg.dependencies ?? {}), ...Object.keys(pkg.devDependencies ?? {})];
  const offenders = declared.filter((name) => FORBIDDEN_DEPENDENCIES.some((pattern) => pattern.test(name)));
  assert.deepEqual(offenders, [], `业务 Module 不得声明平台依赖: ${offenders.join(', ')}`);
});

test('mobile-core source imports no platform module (issue #68 AC1 gate)', () => {
  const offenders: string[] = [];
  for (const file of tsFiles(srcRoot)) {
    const source = readFileSync(file, 'utf8');
    const specifiers = [
      ...source.matchAll(/from\s+['"]([^'"]+)['"]/g),
      ...source.matchAll(/import\s*\(\s*['"]([^'"]+)['"]\s*\)/g),
      ...source.matchAll(/require\s*\(\s*['"]([^'"]+)['"]\s*\)/g),
    ].map((match) => match[1]);
    for (const specifier of specifiers) {
      if (FORBIDDEN_IMPORTS.some((pattern) => pattern.test(specifier))) {
        offenders.push(`${path.relative(packageRoot, file)} -> ${specifier}`);
      }
    }
  }
  assert.deepEqual(offenders, [], `业务 Module 源码不得导入平台模块: ${offenders.join('; ')}`);
});
```

- [ ] **Step 2: 运行确认通过（这是门槛钉，先证明绿）**

Run: `pnpm exec tsx --test packages/mobile-core/src/platform-purity.test.ts`
Expected: PASS（2 个用例绿——当前 mobile-core 的 dependencies 仅 `@weknora/domain`，devDependencies 另有 `@weknora/api-client` 与 `@types/node`（均非平台依赖，门槛不禁）；源码无平台导入）。

- [ ] **Step 3: 证明门槛真的能失败（RED 探针后还原）**

```bash
printf "import 'react-native';\n" >> packages/mobile-core/src/runtime/scope-lease.ts
pnpm exec tsx --test packages/mobile-core/src/platform-purity.test.ts
```

Expected: FAIL，错误信息含 `scope-lease.ts -> react-native`（测试只读源码文本，不做模块解析，探针不会引起加载错误）。

```bash
git checkout -- packages/mobile-core/src/runtime/scope-lease.ts
pnpm exec tsx --test packages/mobile-core/src/platform-purity.test.ts
```

Expected: PASS。

- [ ] **Step 4: Commit**

```bash
git add packages/mobile-core/src/platform-purity.test.ts
git commit -m "test(mobile-core): platform purity gate for issue #68 AC1 (no Expo/WeChat deps or imports)"
```

---

### Task 2: 依赖接线 + Taro 平台 Adapter（凭据仓 / 授权通道 / blob 抓取 / 意图日志）

**Files:**
- Modify: `apps/miniprogram/package.json`
- Modify: `apps/miniprogram/src/platform/transport.ts:83-132`
- Create: `apps/miniprogram/src/platform/credential-store.ts`
- Create: `apps/miniprogram/src/platform/authorized-channels.ts`
- Create: `apps/miniprogram/src/platform/intent-log.ts`
- Test: `apps/miniprogram/tests/platform-adapters.test.mjs`

**Interfaces:**
- Consumes: `CredentialStore`/`StoredCredential`/`AuthorizedTransport`/`AuthorizedStreamTransport`/`BlobFetchPort`/`SubmissionIntentLog`/`SubmissionIntentRecord`（`@weknora/mobile-core` 根导出，`packages/mobile-core/src/index.ts`）；`ApiError`（`@weknora/api-client`）；`createWeappTransport`/`WeappNetwork`/`TransportFailure`/`HttpRequest`（既有平台层）。
- Produces（Task 3/4 消费）:
  - `credentialKeyOf(origin: string): string`
  - `readStoredCredential(store: ValueStore, origin: string): StoredCredential | undefined`
  - `adoptLegacyCredentials(store: ValueStore, origin: string): void`
  - `createTaroCredentialStore(store: ValueStore, origin: string): CredentialStore`
  - `createAuthorizedRequestChannel(request: (input: ClientRequest) => Promise<unknown>): AuthorizedTransport`
  - `createAuthorizedStreamChannel(network: WeappNetwork, origin: string): AuthorizedStreamTransport`
  - `createTaroBlobFetch(network: WeappNetwork): BlobFetchPort`
  - `createTaroIntentLog(store: ValueStore): SubmissionIntentLog`
  - `streamWeapp(network: WeappNetwork, request: HttpRequest, onText: (text: string) => void, onMetadata?: (meta: StreamMetadata) => void): Promise<HttpResult>`（transport.ts 新导出）

- [ ] **Step 1: 写失败测试**

创建 `apps/miniprogram/tests/platform-adapters.test.mjs`：

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import { registerHooks } from 'node:module';
import { pathToFileURL } from 'node:url';

const stubURL = pathToFileURL(new URL('./helpers/taro-stub.mjs', import.meta.url).pathname).href;
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === '@tarojs/taro') return { url: stubURL, shortCircuit: true };
    return nextResolve(specifier, context);
  },
});
globalThis.__API_ORIGIN__ = 'https://api.example.test';
const { stub } = await import('./helpers/taro-stub.mjs');
const transport = await import('../src/platform/transport.ts');
const credentialStore = await import('../src/platform/credential-store.ts');
const channels = await import('../src/platform/authorized-channels.ts');
const intentLog = await import('../src/platform/intent-log.ts');

const memoryStore = () => {
  const map = new Map();
  return { read: k => map.get(k), write: (k, v) => map.set(k, v), remove: k => map.delete(k), keys: () => [...map.keys()] };
};

test('credential store: runtime write/read/clear roundtrip on the canonical key', async () => {
  const store = memoryStore();
  const cs = credentialStore.createTaroCredentialStore(store, 'https://api.example.test');
  assert.equal(await cs.read('https://api.example.test'), undefined, 'empty store reads undefined');
  await cs.write('https://api.example.test', { token: 't1', refreshToken: 'r1' });
  assert.deepEqual(await cs.read('https://api.example.test'), { token: 't1', refreshToken: 'r1' });
  await cs.clear('https://api.example.test');
  assert.equal(await cs.read('https://api.example.test'), undefined);
  assert.equal(store.keys().includes('wk:auth:https://api.example.test'), false, 'clear removes the key');
});

test('credential store: just-in-time readers see a rotation immediately (single writer is the runtime)', async () => {
  const store = memoryStore();
  const cs = credentialStore.createTaroCredentialStore(store, 'https://api.example.test');
  await cs.write('https://api.example.test', { token: 't1', refreshToken: 'r1' });
  assert.equal(credentialStore.readStoredCredential(store, 'https://api.example.test').token, 't1');
  // Runtime 轮换（refresh 后 persistCredential 写回）
  await cs.write('https://api.example.test', { token: 't2', refreshToken: 'r2' });
  assert.equal(credentialStore.readStoredCredential(store, 'https://api.example.test').token, 't2');
});

test('credential store: legacy bearer shape and host-case variants are adopted exactly once (D7 semantics)', () => {
  const store = memoryStore();
  store.write('wk:auth:https://API.example.test', { kind: 'bearer', accessToken: 'legacy', refreshToken: 'legacy-r' });
  store.write('wk:auth:https://api.example.test ', { kind: 'garbage' });
  store.write('wk:auth:https://other.example.test', { kind: 'bearer', accessToken: 'other', refreshToken: 'other-r' });
  credentialStore.adoptLegacyCredentials(store, 'https://api.example.test');
  assert.deepEqual(credentialStore.readStoredCredential(store, 'https://api.example.test'), { token: 'legacy', refreshToken: 'legacy-r' });
  assert.equal(store.keys().includes('wk:auth:https://API.example.test'), false, 'case variant removed after adoption');
  assert.deepEqual(credentialStore.readStoredCredential(store, 'https://other.example.test'), { token: 'other', refreshToken: 'other-r' }, 'other origin untouched');
});

test('authorized stream channel: pre-stream non-2xx rejects with a real ApiError shape (runtime 401 retry depends on it)', async () => {
  const network = {
    request(options) { return stub.dispatch('request', options); },
    uploadFile(options) { return stub.dispatch('uploadFile', options); },
  };
  stub.reset();
  stub.use(call => {
    if (call.kind !== 'request') { call.options.fail({ errMsg: 'unexpected' }); return; }
    if (call.options.enableChunked) { stub.emitHeaders(call, {}, 401); return; }
    call.options.fail({ errMsg: 'unexpected route' });
  });
  const stream = channels.createAuthorizedStreamChannel(network, 'https://api.example.test');
  await assert.rejects(
    stream({ method: 'GET', path: '/api/v1/workbench/executions/run-1/events' }, 'tok', () => {}),
    error => error.name === 'ApiError' && error.status === 401 && error.code === 'HTTP_401',
  );
});

test('authorized stream channel: chunks are forwarded as decoded text', async () => {
  const network = { request: options => stub.dispatch('request', options), uploadFile: options => stub.dispatch('uploadFile', options) };
  stub.reset();
  stub.use(call => {
    stub.emitHeaders(call, { 'Content-Type': 'text/event-stream; charset=utf-8' }, 200);
    stub.emitChunk(call, new TextEncoder().encode('data: x\n\n').buffer);
    stub.succeed(call, { statusCode: 200, header: { 'Content-Type': 'text/event-stream' }, data: '' });
  });
  const stream = channels.createAuthorizedStreamChannel(network, 'https://api.example.test');
  let text = '';
  await stream({ method: 'GET', path: '/api/v1/workbench/executions/run-1/events' }, 'tok', chunk => { text += chunk; });
  assert.equal(text, 'data: x\n\n');
  assert.equal(stub.lastCall('request').options.header.authorization, 'Bearer tok');
  assert.equal(stub.lastCall('request').options.header.accept, 'text/event-stream');
});

test('blob fetch: refuses non-http(s) schemes and fetches arraybuffer bytes over the network seam', async () => {
  const network = { request: options => stub.dispatch('request', options), uploadFile: options => stub.dispatch('uploadFile', options) };
  const blob = channels.createTaroBlobFetch(network);
  await assert.rejects(blob.fetch('ftp://api.example.test/file.bin'), /BLOB_FETCH/);
  stub.reset();
  stub.use(call => {
    assert.equal(call.options.responseType, 'arraybuffer');
    stub.succeed(call, { statusCode: 200, header: { 'content-type': 'text/plain' }, data: new TextEncoder().encode('hello').buffer });
  });
  const result = await blob.fetch('https://cdn.example.test/file.txt');
  assert.equal(new TextDecoder().decode(result.bytes), 'hello');
  assert.equal(result.mime, 'text/plain');
});

test('intent log: save/load/listScope/remove roundtrip; corrupt storage reads as empty', async () => {
  const store = memoryStore();
  const log = intentLog.createTaroIntentLog(store);
  const scope = { origin: 'https://api.example.test', tenantID: '1', userID: 'u1' };
  const record = { requestId: 'req-1', sessionId: 's-1', goal: { text: '写周报', agentId: 'a1', budgetUpper: 100 }, scope, persistedAt: '2026-09-24T00:00:00Z' };
  await log.save(record);
  assert.deepEqual(await log.load('req-1'), record);
  assert.equal((await log.listScope(scope)).length, 1);
  assert.equal((await log.listScope({ ...scope, tenantID: '2' })).length, 0, 'scope mismatch is filtered out');
  await log.save({ ...record, requestId: 'req-1', sessionId: 's-2' });
  assert.equal((await log.listScope(scope)).length, 1, 'same requestId overwrites, never duplicates');
  await log.remove('req-1');
  assert.equal(await log.load('req-1'), undefined);
  store.write('wk:mini:intents.v1', 'not-json-object');
  assert.deepEqual(await log.listScope(scope), [], 'corrupt storage reads as an empty table');
});
```

注意：`stub.dispatch` 若 taro-stub 未导出，改用其等价驱动面——先读 `tests/helpers/taro-stub.mjs` 末尾的 `// ---- 测试驱动面 ----` 段确认导出名（当前导出 `reset/state/use/succeed/emitHeaders/emitChunk/lastCall`；`dispatch` 是内部函数）。若 `dispatch` 未导出，本步骤同时给 taro-stub.mjs 追加一行 `export { dispatch };`（该文件是测试替身，属本计划管辖）。

- [ ] **Step 2: 运行确认失败**

Run: `cd apps/miniprogram && node --experimental-transform-types --test tests/platform-adapters.test.mjs`
Expected: FAIL——`Cannot find package '@weknora/mobile-core'`（credential-store/authorized-channels/intent-log 尚未创建、依赖未接线）。

- [ ] **Step 3: 接线依赖与运行器，提取 streamWeapp**

3a. `apps/miniprogram/package.json` dependencies 增加（保持字母序）：

```json
    "@weknora/domain": "workspace:*",
    "@weknora/mobile-core": "workspace:*",
```

test 脚本改为（strip-types 无法加载 mobile-core，见差异记录 D5）：

```json
    "test": "node --experimental-transform-types --test tests/*.test.mjs",
```

然后执行 `pnpm install`（worktree 根；把 workspace 依赖写入 lockfile 并链接 node_modules）。

3b. `apps/miniprogram/src/platform/transport.ts`：把 `createWeappTransport` 内部的 `function stream(...)` 整体提升为模块级导出 `streamWeapp`，闭包变量 `native` 换成参数 `network`；`createWeappTransport` 返回值改为委托。具体：

把（transport.ts:94 起，注释 `Distinct from HttpTransport.sendStream:` 之下）：

```ts
  function stream(request:HttpRequest,onText:(text:string)=>void,onMetadata?:(meta:StreamMetadata)=>void):Promise<HttpResult> {
```

及其整个函数体（到与之配对的 `}`，即 `return {send:…}` 之前）剪切为模块级：

```ts
/** 授权 SSE 与既有会话流共用同一实现：metadata/分块/超时/中止语义只有一份。 */
export function streamWeapp(network:WeappNetwork,request:HttpRequest,onText:(text:string)=>void,onMetadata?:(meta:StreamMetadata)=>void):Promise<HttpResult> {
  // ……原函数体，`native.request(...)` 改为 `network.request(...)`，其余逐字保留……
}
```

并在 `createWeappTransport` 返回对象中替换为委托：

```ts
  return {send:(r:HttpRequest)=>perform(r),sendBinary:(r:HttpRequest)=>perform(r,true),sendMultipartFile,stream:(r,onText,onMetadata?)=>streamWeapp(native,r,onText,onMetadata)};
```

3c. 创建 `apps/miniprogram/src/platform/credential-store.ts`：

```ts
import type { CredentialStore, StoredCredential } from '@weknora/mobile-core';
import type { ValueStore } from '../core/intent.ts';
import { normalizeApiOrigin } from '../core/auth.ts';

export const CREDENTIAL_KEY_PREFIX = 'wk:auth:';
export function credentialKeyOf(origin: string): string { return `${CREDENTIAL_KEY_PREFIX}${origin}`; }

/**
 * 用时现读（上传/下载等平台直连通道专用）：只有 Runtime 写这个键（登录/轮换/清除），
 * 读取方永远拿到最新轮换后的凭据——本函数是「token 不出 Runtime」在小程序平台通道上的
 * 忠实落地：不缓存、不刷新、不解析过期。
 * 兼容收养旧 AuthCoordinator 形态 {kind:'bearer',accessToken,refreshToken}。
 */
export function readStoredCredential(store: ValueStore, origin: string): StoredCredential | undefined {
  const value = store.read(credentialKeyOf(origin));
  if (value === null || typeof value !== 'object') return undefined;
  const record = value as Record<string, unknown>;
  const token = typeof record.token === 'string' && record.token !== ''
    ? record.token
    : typeof record.accessToken === 'string' && record.accessToken !== '' ? record.accessToken : undefined;
  const refreshToken = typeof record.refreshToken === 'string' && record.refreshToken !== '' ? record.refreshToken : undefined;
  if (token === undefined || refreshToken === undefined) return undefined;
  return { token, refreshToken };
}

/** 一次性收养旧登录态（D7 语义迁移）：同 origin 的大小写变体收养首个有效 bearer 后清除；垃圾变体直接清除；其他 origin 不动。 */
export function adoptLegacyCredentials(store: ValueStore, origin: string): void {
  const canonical = credentialKeyOf(origin);
  for (const key of store.keys?.() ?? []) {
    if (!key.startsWith(CREDENTIAL_KEY_PREFIX) || key === canonical) continue;
    if (normalizeApiOrigin(key.slice(CREDENTIAL_KEY_PREFIX.length)) !== normalizeApiOrigin(origin)) continue;
    const adopted = readStoredCredential(store, key.slice(CREDENTIAL_KEY_PREFIX.length));
    if (adopted && readStoredCredential(store, origin) === undefined) {
      store.write(canonical, { token: adopted.token, refreshToken: adopted.refreshToken });
    }
    store.remove(key);
  }
}

export function createTaroCredentialStore(store: ValueStore, origin: string): CredentialStore {
  adoptLegacyCredentials(store, origin);
  return {
    async read(deployment) { return readStoredCredential(store, deployment); },
    async write(deployment, credential) { store.write(credentialKeyOf(deployment), { token: credential.token, refreshToken: credential.refreshToken }); },
    async clear(deployment) { store.remove(credentialKeyOf(deployment)); },
  };
}
```

3d. 创建 `apps/miniprogram/src/platform/authorized-channels.ts`：

```ts
import { ApiError } from '@weknora/api-client';
import type { ClientRequest, HttpRequest } from '@weknora/api-client';
import type { AuthorizedStreamTransport, AuthorizedTransport, BlobFetchPort } from '@weknora/mobile-core';
import { TransportFailure, streamWeapp, type WeappNetwork } from './transport.ts';

/**
 * 授权 REST 通道（与 apps/mobile composition.ts:103-107 同形态）：复用既有 ClientRequest
 * 通道，叠加 Bearer；不新建传输、不持有 token——token 由 Runtime 调用时注入。
 */
export function createAuthorizedRequestChannel(request: (input: ClientRequest) => Promise<unknown>): AuthorizedTransport {
  return (input, accessToken) =>
    request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
}

/**
 * 授权 SSE 通道（AuthorizedStreamTransport 契约）：pre-stream 非 2xx 以真 ApiError 拒绝——
 * Runtime 的 401 刷新重试（unauthorizedStatus 按 name==='ApiError'&&status===401 识别，
 * mobile-runtime.ts:86-90）与 api-client 远端 409→TASK_STREAM_CURSOR_EXPIRED 映射都依赖该形态。
 */
export function createAuthorizedStreamChannel(network: WeappNetwork, origin: string): AuthorizedStreamTransport {
  return (input, accessToken, onChunk) => streamAuthorized(network, origin, input, accessToken, onChunk);
}

async function streamAuthorized(
  network: WeappNetwork,
  origin: string,
  input: { method: string; path: string; headers?: Record<string, string>; body?: unknown; signal?: AbortSignal },
  accessToken: string,
  onChunk: (chunk: string) => void,
): Promise<void> {
  const request: HttpRequest = {
    method: input.method,
    url: `${origin}${input.path}`,
    headers: { ...(input.headers ?? {}), authorization: `Bearer ${accessToken}`, accept: 'text/event-stream' },
    ...(input.body === undefined ? {} : { body: input.body }),
    ...(input.signal === undefined ? {} : { signal: input.signal }),
  };
  try {
    await streamWeapp(network, request, onChunk);
  } catch (error) {
    if (error instanceof TransportFailure && typeof error.status === 'number') {
      throw new ApiError({ status: error.status, code: `HTTP_${error.status}`, message: `authorized stream failed with HTTP ${error.status}` });
    }
    throw error;
  }
}

/** 免凭据字节抓取（Task Material 签名链接的兑现通道）：仅接受 http/https（module-seams §7.3 / Mimosa URL 约束）。 */
export function createTaroBlobFetch(network: WeappNetwork): BlobFetchPort {
  return {
    async fetch(url) {
      let parsed: URL;
      try { parsed = new URL(url); } catch { throw new Error('BLOB_FETCH_INVALID_URL'); }
      if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') throw new Error('BLOB_FETCH_UNSUPPORTED_SCHEME');
      const fetched = await new Promise<{ buffer: ArrayBuffer; mime: string }>((resolve, reject) => {
        network.request({
          url, method: 'GET', header: {}, timeout: 60_000, responseType: 'arraybuffer', dataType: 'text',
          success: result => {
            if (result.statusCode !== 200) {
              reject(Object.assign(new Error(`BLOB_FETCH_HTTP_${result.statusCode}`), { status: result.statusCode }));
              return;
            }
            if (!(result.data instanceof ArrayBuffer)) { reject(new Error('BLOB_FETCH_NOT_BINARY')); return; }
            const rawHeader = (result.header ?? {}) as Record<string, unknown>;
            const mime = typeof rawHeader['content-type'] === 'string' && rawHeader['content-type'] !== ''
              ? rawHeader['content-type'] : 'application/octet-stream';
            resolve({ buffer: result.data, mime });
          },
          fail: () => reject(new Error('BLOB_FETCH_NETWORK')),
        });
      });
      return { bytes: new Uint8Array(fetched.buffer), mime: fetched.mime };
    },
  };
}
```

3e. 创建 `apps/miniprogram/src/platform/intent-log.ts`：

```ts
import type { SubmissionIntentLog, SubmissionIntentRecord } from '@weknora/mobile-core';
import type { ValueStore } from '../core/intent.ts';

const KEY = 'wk:mini:intents.v1';

/**
 * 耐久意图日志（#36 SubmissionIntentLog 的 storage 实现）：TaskOffice 在 Start POST 前
 * await save；重启后 load/listScope 用原 session 与原 goal 重建 digest 一致的 Start 输入。
 * 损坏 JSON 读空表——恢复语义宁可少恢复，不可恢复出脏意图。
 */
export function createTaroIntentLog(store: ValueStore): SubmissionIntentLog {
  const readAll = (): SubmissionIntentRecord[] => {
    const value = store.read(KEY);
    if (!Array.isArray(value)) return [];
    return value.filter((item): item is SubmissionIntentRecord =>
      typeof item === 'object' && item !== null && typeof (item as { requestId?: unknown }).requestId === 'string');
  };
  const writeAll = (records: SubmissionIntentRecord[]): void => { store.write(KEY, records); };
  return {
    async save(record) {
      writeAll([...readAll().filter(item => item.requestId !== record.requestId), record]);
    },
    async load(requestId) { return readAll().find(item => item.requestId === requestId); },
    async listScope(scope) {
      return readAll().filter(item => {
        const s = item.scope;
        return s !== undefined && s.origin === scope.origin && s.tenantID === scope.tenantID && s.userID === scope.userID;
      });
    },
    async remove(requestId) { writeAll(readAll().filter(item => item.requestId !== requestId)); },
  };
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd apps/miniprogram && node --experimental-transform-types --test tests/platform-adapters.test.mjs`
Expected: PASS（7 个用例绿）。

再跑既有套件确认运行器切换无回归：

Run: `cd apps/miniprogram && node --experimental-transform-types --test tests/*.test.mjs`
Expected: 41+7 pass / 0 fail。

- [ ] **Step 5: Commit**

```bash
git add apps/miniprogram/package.json pnpm-lock.yaml apps/miniprogram/src/platform/transport.ts apps/miniprogram/src/platform/credential-store.ts apps/miniprogram/src/platform/authorized-channels.ts apps/miniprogram/src/platform/intent-log.ts apps/miniprogram/tests/platform-adapters.test.mjs tests/helpers/taro-stub.mjs
git commit -m "feat(miniprogram): taro platform adapters for mobile-core (credential store, authorized channels, blob fetch, intent log)"
```

（`tests/helpers/taro-stub.mjs` 仅当 Step 1 追加了 `export { dispatch };` 时才在 add 列表中。）

---

### Task 3: MobileRuntime 成为唯一会话编排器（services/runtime.ts 重写 + session 门面）

**Files:**
- Create: `apps/miniprogram/src/services/session.ts`
- Modify: `apps/miniprogram/src/services/runtime.ts`（全文重写）
- Modify: `apps/miniprogram/src/core/auth.ts`（删 `AuthCoordinator`/`AuthPort`，保留类型与纯函数）
- Test（重写）: `apps/miniprogram/tests/assembly.test.mjs`

**Interfaces:**
- Consumes: Task 2 全部 Produces；`createMobileRuntime`/`MobileRuntime`/`RuntimeSnapshot`/`StoredCredential`（mobile-core）；`createMobileRuntimeRemote`（`@weknora/api-client/mobile/runtime`）；`CLIENT_PROTOCOL_VERSION`（`@weknora/domain/mobile`，值为 3）；`ScopeGuard`/`scopeKey`（`core/scope.ts`）；`SessionView`/`normalizeApiOrigin`（`core/auth.ts`）。
- Produces（Task 4/5 与既有页面消费）:
  - `services/runtime.ts` 导出面（**与旧导出同名同签名**，features/与 components/ui.tsx/app.tsx/platform/files.ts 零改动）：`auth: TaroSessionFacade`、`client`（授权 JSON 通道上的 WeKnoraClient）、`executions`、`stream(input, onText)`、`chatStream(options, onEvent)`、`apiOrigin`、`logout()`、`stopSubscriptions()`；新增 `runtime: MobileRuntime`、`network: WeappNetwork`、`currentBearerToken(): Promise<string>`（上传/下载直连通道预检+现读）。
  - `TaroSessionFacade = { bootstrap(): Promise<void>; login(email: string, password: string): Promise<void>; switchTenant(id: number): Promise<void>; logout(): Promise<void>; snapshot(): SessionView; subscribe(listener: () => void): () => void; credential(): Credential; readonly scope: ScopeGuard; abortSubscriptions(): void }`

- [ ] **Step 1: 重写 assembly 测试（RED——新语义下旧实现必然失败）**

`apps/miniprogram/tests/assembly.test.mjs` 全文替换为：

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import { registerHooks } from 'node:module';
import { pathToFileURL } from 'node:url';

// 平台边界替换：真实 @tarojs/taro 在 Node 下因 webpack DefinePlugin 常量无法求值，
// 用契约级替身承载 request/uploadFile/storage；其余全部为待提交真实源码。
const stubURL = pathToFileURL(new URL('./helpers/taro-stub.mjs', import.meta.url).pathname).href;
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === '@tarojs/taro') return { url: stubURL, shortCircuit: true };
    return nextResolve(specifier, context);
  },
});
globalThis.__API_ORIGIN__ = 'https://api.example.test';
const { stub } = await import('./helpers/taro-stub.mjs');
const runtimeModule = await import('../src/services/runtime.ts');

const ORIGIN = 'https://api.example.test';
// activateTenant 的身份来自切换后的 me()（mobile-runtime.ts:416-429：switch-tenant → persist → authenticate），
// 因此 me() 必须是有状态替身：activeTenant 随 switch-tenant 路由翻转。
let activeTenant = 1;
const me = () => ({ success: true, data: { user: { id: 'u1', username: 'Lin' }, tenant: { id: activeTenant, name: activeTenant === 1 ? 'Space' : 'Space 2' }, memberships: [
  { tenant_id: 1, tenant_name: 'Space', role: 'owner' },
  { tenant_id: 2, tenant_name: 'Space 2', role: '成员' },
] } });
const capabilities = () => ({ success: true, data: { protocol_minimum: 1, protocol_maximum: 5 } });
const settle = ms => new Promise(resolve => setTimeout(resolve, ms ?? 10));
async function until(predicate, ms = 1500) { const end = Date.now() + ms; while (Date.now() < end) { if (predicate()) return true; await settle(5); } return predicate(); }

/** 安装按 method+pathname 路由的假后端；route 返回 undefined 时挂起（等测试手动响应）。 */
function backend(routes) {
  stub.use(call => {
    const method = call.options.method, path = new URL(call.options.url).pathname;
    const exact = routes[`${method} ${path}`];
    let fn = exact;
    if (fn === undefined) {
      // 以 '/' 结尾的键按前缀匹配，承载 :run_id / :request_id 路径参数。
      const prefix = Object.keys(routes).filter(k => k.endsWith('/') && `${method} ${path}`.startsWith(k)).sort((a, b) => b.length - a.length)[0];
      if (prefix) fn = routes[prefix];
    }
    if (fn === undefined) { call.options.fail({ errMsg: `no backend route for ${method} ${path}` }); return; }
    fn(call);
  });
}
/** 登录态前置路由：login → me → capabilities（Runtime authenticate 的真实三步）。 */
function authRoutes(extra = {}) {
  return {
    'POST /api/v1/auth/login': call => stub.succeed(call, { data: { success: true, data: { token: 't1', refresh_token: 'r1' } } }),
    'GET /api/v1/auth/me': call => stub.succeed(call, { data: me() }),
    'GET /api/v1/system/capabilities': call => stub.succeed(call, { data: capabilities() }),
    ...extra,
  };
}
async function freshLogin(extraRoutes = {}) {
  // 注意：stub.reset() 会连 storage 一起清空，而 MobileRuntime 每次授权请求都从凭据仓现读
  // （sendWithCredential → credentialStore.read）——登录后绝不能再 reset，只按需重装路由 handler。
  stub.reset();
  backend(authRoutes(extraRoutes));
  await runtimeModule.auth.login('u@example.test', 'pw');
}
const authorization = call => call.options.header.Authorization ?? call.options.header.authorization;
const authKeys = () => [...stub.state.storage.keys()].filter(k => k.startsWith('wk:auth:'));

test('assembly: login through the real MobileRuntime stores exactly one credential and stamps scope', async () => {
  await freshLogin();
  assert.equal(runtimeModule.auth.snapshot().phase, 'ready');
  assert.equal(runtimeModule.auth.snapshot().tenantId, '1');
  assert.equal(JSON.stringify(runtimeModule.auth.snapshot()).includes('t1'), false, 'tokens never escape into observable UI state');
  assert.equal(authKeys().length, 1, 'exactly one credential key (single writer: the runtime)');
  const stored = stub.state.storage.get(authKeys()[0]);
  assert.ok(stored && (stored.token === 't1' || stored.accessToken === 't1'), 'runtime persists the credential in the store');
  assert.ok(runtimeModule.runtime.scopeLease(), 'authorization mints a scope lease for deep modules');
  assert.ok(runtimeModule.runtime.resourceShelf(), 'resource shelf opens with the lease');
  // 身份富集是 authorized 后的一次异步 /auth/me：轮询等待而非 sleep。
  assert.ok(await until(() => runtimeModule.auth.snapshot().userName === 'Lin'), 'identity enrichment reads /auth/me through the authorized channel');
});

test('assembly: concurrent 401s share one refresh and replay with the rotated token (all methods)', async () => {
  let refreshCount = 0, replayAuth = '';
  await freshLogin({
    'GET /api/v1/execution-targets': call => {
      if (authorization(call) === 'Bearer t1') { stub.succeed(call, { statusCode: 401, data: { success: false } }); return; }
      replayAuth = authorization(call);
      stub.succeed(call, { data: { success: true, data: [{ id: 'platform', kind: 'platform', state: 'active' }] } });
    },
    'POST /api/v1/auth/refresh': call => { refreshCount++; stub.succeed(call, { data: { success: true, access_token: 't2', refresh_token: 'r2' } }); },
  });
  const [a, b] = await Promise.all([
    runtimeModule.client.request({ method: 'GET', path: '/api/v1/execution-targets' }),
    runtimeModule.client.request({ method: 'GET', path: '/api/v1/execution-targets' }),
  ]);
  assert.equal(refreshCount, 1, 'concurrent 401s must share one refresh (runtime single-flight)');
  assert.equal(replayAuth, 'Bearer t2');
  assert.equal(a.success, true); assert.equal(b.success, true);
  assert.equal(runtimeModule.auth.credential().accessToken, 't2', 'just-in-time readers see the rotation immediately');
});

test('assembly: a definitive 401 on a POST is refreshed and replayed; an ambiguous network failure never replays', async () => {
  // 401 = 服务端未认证即拒绝，请求未被执行，重放安全（Runtime 语义，与本计划差异记录一致）。
  let posts = 0, refreshes = 0, lastAuth = '';
  await freshLogin({
    'POST /api/v1/workbench/executions': call => {
      posts++;
      if (authorization(call) === 'Bearer t1') { stub.succeed(call, { statusCode: 401, data: { success: false } }); return; }
      lastAuth = authorization(call);
      stub.succeed(call, { statusCode: 202, data: { success: true, data: { run_id: 'run-1', request_id: 'req-1', status: 'admitted' } } });
    },
    'POST /api/v1/auth/refresh': call => { refreshes++; stub.succeed(call, { data: { success: true, access_token: 't2', refresh_token: 'r2' } }); },
  });
  const ack = await runtimeModule.client.request({ method: 'POST', path: '/api/v1/workbench/executions', body: {} });
  assert.equal(ack.run_id, 'run-1');
  assert.equal(posts, 2, 'the 401 POST is replayed exactly once after refresh');
  assert.equal(refreshes, 1);
  assert.equal(lastAuth, 'Bearer t2');
  // 歧义失败（网络中断）：绝不重放——重复提交的风险面在这里，不在 401。
  // 注意只重装路由 handler（backend 直接覆盖），不 reset——凭据仓里的 t2 必须仍在。
  let lost = 0;
  backend(authRoutes({
    'POST /api/v1/workbench/executions': call => { lost++; call.options.fail({ errMsg: 'request lost' }); },
  }));
  await assert.rejects(runtimeModule.client.request({ method: 'POST', path: '/api/v1/workbench/executions', body: {} }));
  assert.equal(lost, 1, 'NETWORK_ERROR on a POST must never be retried');
});

test('assembly: 403 is surfaced as permission denial, never treated as 401 refresh', async () => {
  let refreshes = 0;
  await freshLogin({
    'GET /api/v1/execution-targets': call => stub.succeed(call, { statusCode: 403, data: { success: false } }),
    'POST /api/v1/auth/refresh': call => { refreshes++; stub.succeed(call, {}); },
  });
  await assert.rejects(runtimeModule.client.request({ method: 'GET', path: '/api/v1/execution-targets' }), error => error.status === 403);
  assert.equal(refreshes, 0);
});

test('assembly: a response landing after sign-out is discarded (RUNTIME_SCOPE_CHANGED)', async () => {
  await freshLogin({
    'GET /api/v1/execution-targets': () => {/* 挂起：等登出后再响应 */},
  });
  const pending = runtimeModule.client.request({ method: 'GET', path: '/api/v1/execution-targets' });
  await runtimeModule.runtime.signOut();
  const call = stub.lastCall('request');
  stub.succeed(call, { data: { success: true, data: [{ id: 'x', kind: 'platform', state: 'active' }] } });
  await assert.rejects(pending, error => /SCOPE_CHANGED|cancelled/i.test(`${error.message} ${error.code ?? ''}`));
  assert.equal(runtimeModule.auth.snapshot().phase, 'anonymous');
});

test('assembly: chatStream assembles SSE frames end-to-end through the authorized Taro stream channel', async () => {
  await freshLogin({
    'POST /api/v1/knowledge-chat/s-1': call => {
      assert.equal(call.options.header.authorization, 'Bearer t1', 'the runtime injects the bearer into the stream channel');
      stub.emitHeaders(call, { 'Content-Type': 'text/event-stream; charset=utf-8' });
      // 中文+emoji 逐字节拆分跨 chunk，验证真实 Utf8Decoder 与 SSE 解析装配。
      const wire = new TextEncoder().encode('data: {"response_type":"answer","content":"你好😀"}\r\n\r\n');
      for (const byte of wire) stub.emitChunk(call, Uint8Array.of(byte).buffer);
      stub.succeed(call, { statusCode: 200, header: { 'Content-Type': 'text/event-stream' }, data: '' });
    },
  });
  const events = [];
  await runtimeModule.chatStream({ sessionId: 's-1', body: { query: '问' } }, event => events.push(event));
  assert.equal(events.length, 1);
  assert.equal(events[0].response_type, 'answer');
  assert.equal(events[0].content, '你好😀');
});

test('assembly: tenant switch rotates the scope and aborts in-flight subscriptions', async () => {
  await freshLogin({
    'POST /api/v1/auth/switch-tenant': call => {
      activeTenant = 2; // 切换后 me() 返回新租户（activateTenant 以 me() 为身份权威）
      stub.succeed(call, { data: { success: true, data: { token: 't3', refresh_token: 'r3', tenant: { id: 2, name: 'Space 2' }, memberships: [] } } });
    },
  });
  const before = runtimeModule.auth.scope.capture();
  const controller = runtimeModule.auth.scope.controller();
  await runtimeModule.auth.switchTenant(2);
  assert.equal(controller.signal.aborted, true, 'old subscriptions are aborted on switch');
  assert.equal(runtimeModule.auth.scope.isCurrent(before), false);
  assert.equal(runtimeModule.auth.snapshot().tenantId, '2');
  assert.equal(runtimeModule.auth.snapshot().tenantName, 'Space 2', 'tenantName derives synchronously from the memberships-backed tenant list');
});

test('assembly: bootstrap restores a stored credential without a fresh login', async () => {
  stub.reset();
  stub.state.storage.set('wk:auth:https://api.example.test', { token: 't1', refreshToken: 'r1' });
  backend(authRoutes());
  await runtimeModule.auth.bootstrap();
  assert.equal(runtimeModule.auth.snapshot().phase, 'ready');
  assert.equal(runtimeModule.auth.snapshot().userId, 'u1');
});

test('assembly: bootstrap with a dead network keeps an honest error, not a silent anonymous', async () => {
  stub.reset();
  stub.state.storage.set('wk:auth:https://api.example.test', { token: 't1', refreshToken: 'r1' });
  // me() 以网络失败拒绝（不能挂起——boot 会永远等不到响应）；boot 吞错回 deployment-login，
  // 凭据未被清除 ⇒ 门面给出可重试 error 而非静默登出。
  backend({ 'GET /api/v1/auth/me': call => call.options.fail({ errMsg: 'network down' }) });
  await runtimeModule.auth.bootstrap();
  assert.equal(runtimeModule.auth.snapshot().phase, 'error', 'stored credential + unreachable server = retryable error, not a silent logout');
});

test('assembly: logout revokes remotely best-effort, clears credentials and the private cache', async () => {
  await freshLogin({
    'POST /api/v1/auth/logout': call => call.options.fail({ errMsg: 'offline' }),
  });
  stub.state.storage.set('wk:recent-runs:x', ['run-1']);
  await runtimeModule.logout();
  assert.equal(runtimeModule.auth.snapshot().phase, 'anonymous');
  assert.equal(runtimeModule.auth.credential().kind, 'anonymous');
  assert.equal([...stub.state.storage.keys()].filter(k => k.startsWith('wk:')).length, 0, 'private cache and credentials removed even when remote revocation fails');
});
```

注意：以上测试**复用模块级单例**（`services/runtime.ts` 是单例组合根），用例之间存在登录态顺序依赖——与旧 assembly 的做法一致（旧文件同样依赖单例与 `freshLogin`）。执行顺序即声明顺序（node:test 默认串行）。

Run: `cd apps/miniprogram && node --experimental-transform-types --test tests/assembly.test.mjs`
Expected: FAIL——`runtimeModule.auth.login is not a function`/`runtime` 未导出等（services/runtime.ts 尚为旧实现）。

- [ ] **Step 2: 精简 core/auth.ts（删除 AuthCoordinator，保留类型与纯函数）**

`apps/miniprogram/src/core/auth.ts` 全文替换为：

```ts
import type { AuthMe, AuthSession } from '@weknora/api-client';

/** 会话的呈现层视图（components/ui.tsx useSession 消费；凭据永不进入本视图）。 */
export interface SessionView {
  phase:'anonymous'|'loading'|'ready'|'switching'|'error';
  userId:string|null; userName:string; tenantId:string|null; tenantName:string;
  memberships:readonly unknown[]; error?:string;
}

export const anonymous=():SessionView=>({phase:'anonymous',userId:null,userName:'',tenantId:null,tenantName:'',memberships:[]});

/** URL hosts are case-insensitive, so storage keys must not embed raw host case:
 *  builds pointed at https://WeKnora-App.example and https://weknora-app.example
 *  talk to the same server and must share one session identity. */
export function normalizeApiOrigin(origin:string):string{
  const trimmed=origin.replace(/\/+$/,'');
  const match=trimmed.match(/^(https:\/\/)([^/?#]+?)([\/?#].*)?$/i);
  return match?match[1]+match[2].toLowerCase()+(match[3]??''):trimmed;
}

export function isBearer(value:unknown):value is {kind:'bearer';accessToken:string;refreshToken?:string}{
  if(value===null||typeof value!=='object')return false;
  const v=value as Record<string,unknown>;
  return v.kind==='bearer'&&typeof v.accessToken==='string'&&v.accessToken.length>0&&
    (v.refreshToken===undefined||typeof v.refreshToken==='string');
}

/** 仅供类型参考的 wire 形态（session 门面富集时使用；不再有本地 AuthPort 编排）。 */
export type MeWire = AuthMe;
export type SessionWire = AuthSession;
```

- [ ] **Step 3: 创建 services/session.ts（门面）**

`apps/miniprogram/src/services/session.ts`：

```ts
import type { Credential } from '@weknora/api-client';
import type { ClientRequest } from '@weknora/api-client';
import type { MobileRuntime, RuntimeSnapshot } from '@weknora/mobile-core';
import type { SessionView } from '../core/auth.ts';
import { anonymous } from '../core/auth.ts';
import { ScopeGuard, scopeKey, type ScopeIdentity } from '../core/scope.ts';
import { readStoredCredential } from '../platform/credential-store.ts';
import type { ValueStore } from '../core/intent.ts';

export interface TaroSessionDeps {
  runtime: MobileRuntime;
  origin: string;
  storage: ValueStore;
}

export interface TaroSessionFacade {
  bootstrap(): Promise<void>;
  login(email: string, password: string): Promise<void>;
  switchTenant(id: number): Promise<void>;
  logout(): Promise<void>;
  snapshot(): SessionView;
  subscribe(listener: () => void): () => void;
  credential(): Credential;
  readonly scope: ScopeGuard;
  abortSubscriptions(): void;
}

interface MeEnrichment { userId: string; userName: string; memberships: unknown[] }

/**
 * SessionView 门面：MobileRuntime 是唯一会话编排器（登录/刷新/切租户/撤销），本门面只做
 * 「RuntimeSnapshot → 呈现视图」的投影与 UI 侧迟到拒绝（ScopeGuard 桥）。userName/成员
 * 角色不在 RuntimeSnapshot 里（presentation-safe），由授权通道 GET /auth/me 富集一次。
 */
export function createTaroSessionFacade(deps: TaroSessionDeps): TaroSessionFacade {
  const { runtime, origin, storage } = deps;
  const listeners = new Set<() => void>();
  const guard = new ScopeGuard({ origin, userId: null, tenantId: null });
  let enrichment: MeEnrichment | undefined;
  let enrichFlight: Promise<void> = Promise.resolve();
  let transition: 'idle' | 'loading' | 'switching' = 'idle';
  let view: SessionView = anonymous();

  const identityOf = (snapshot: RuntimeSnapshot): ScopeIdentity => ({
    origin,
    userId: snapshot.identity?.userId ?? null,
    tenantId: snapshot.identity?.activeTenantId ?? null,
  });

  const publish = (): void => { for (const fn of listeners) fn(); };

  const fallbackMemberships = (snapshot: RuntimeSnapshot): unknown[] =>
    (snapshot.identity?.tenants ?? []).map(tenant => ({ tenant_id: tenant.id, tenant_name: tenant.name ?? `工作空间 ${tenant.id}`, role: '成员' }));

  const project = (snapshot: RuntimeSnapshot): SessionView => {
    if (snapshot.surface === 'authorized' && snapshot.identity) {
      const identity = snapshot.identity;
      const tenantName = identity.tenants?.find(tenant => tenant.id === identity.activeTenantId)?.name ?? '';
      return {
        phase: 'ready',
        userId: identity.userId,
        userName: enrichment?.userId === identity.userId ? enrichment.userName : '',
        tenantId: identity.activeTenantId ?? null,
        tenantName,
        memberships: enrichment?.userId === identity.userId ? enrichment.memberships : fallbackMemberships(snapshot),
      };
    }
    if (snapshot.surface === 'read-only') return { ...anonymous(), phase: 'error', error: '服务端当前为只读模式，请稍后重试或到 Web 工作台操作' };
    if (snapshot.surface === 'upgrade-required') return { ...anonymous(), phase: 'error', error: '服务端协议版本不兼容，请更新小程序' };
    return anonymous();
  };

  /** 身份富集：authorized 后经授权通道读一次 /auth/me；迟到（登出/换身份）即丢弃。 */
  const enrichIdentity = (userId: string): void => {
    if (enrichment?.userId === userId) return;
    enrichFlight = enrichFlight.then(async () => {
      try {
        const raw = await runtime.authorizedRequest({ method: 'GET', path: '/api/v1/auth/me' });
        const snapshot = runtime.snapshot();
        if (snapshot.surface !== 'authorized' || snapshot.identity?.userId !== userId) return;
        const record = (typeof raw === 'object' && raw !== null ? raw : {}) as Record<string, unknown>;
        const data = (typeof record.data === 'object' && record.data !== null ? record.data : {}) as Record<string, unknown>;
        const user = (typeof data.user === 'object' && data.user !== null ? data.user : {}) as Record<string, unknown>;
        enrichment = {
          userId,
          userName: String(user.username ?? user.name ?? user.email ?? '用户'),
          memberships: Array.isArray(data.memberships) ? data.memberships : [],
        };
        applySnapshot(runtime.snapshot());
      } catch { /* 展示名缺失不致命：视图以空名渲染，绝不伪造 */ }
    });
  };

  let lastSnapshot: RuntimeSnapshot = runtime.snapshot();

  const applySnapshot = (snapshot: RuntimeSnapshot): void => {
    const next = identityOf(snapshot);
    if (scopeKey(next) !== scopeKey(identityOf(lastSnapshot))) guard.switchTo(next);
    lastSnapshot = snapshot;
    view = transition === 'idle' ? project(snapshot) : { ...project(snapshot), phase: transition };
    if (snapshot.surface === 'authorized' && snapshot.identity) enrichIdentity(snapshot.identity.userId);
    publish();
  };

  runtime.subscribe(applySnapshot);

  const facade: TaroSessionFacade = {
    bootstrap: async () => {
      const hadCredential = readStoredCredential(storage, origin) !== undefined;
      transition = 'loading'; view = { ...project(lastSnapshot), phase: 'loading' }; publish();
      await runtime.boot({ origin });
      transition = 'idle';
      applySnapshot(runtime.snapshot());
      // 存有凭据但 boot 后仍是匿名且凭据未被清除 => 网络不可达，给可重试错误而非静默登出。
      if (hadCredential && view.phase === 'anonymous' && readStoredCredential(storage, origin) !== undefined) {
        view = { ...view, phase: 'error', error: '无法验证登录状态，请检查网络后重试' };
        publish();
      }
    },
    login: async (email, password) => {
      if (!email.trim() || !password) throw new Error('请输入邮箱和密码');
      transition = 'loading'; guard.invalidate(); view = { ...view, phase: 'loading' }; publish();
      try {
        await runtime.signIn({ deployment: { origin }, email: email.trim(), password });
      } finally {
        transition = 'idle';
        applySnapshot(runtime.snapshot());
      }
    },
    switchTenant: async (id) => {
      if (view.phase !== 'ready' || !Number.isSafeInteger(id) || id <= 0) throw new Error('工作空间不可切换');
      transition = 'switching'; guard.invalidate(); view = { ...view, phase: 'switching' }; publish();
      try {
        await runtime.activateTenant(String(id));
      } finally {
        transition = 'idle';
        applySnapshot(runtime.snapshot());
      }
    },
    logout: async () => { await runtime.signOut(); },
    snapshot: () => view,
    subscribe: (listener) => { listeners.add(listener); return () => listeners.delete(listener); },
    credential: () => {
      const stored = readStoredCredential(storage, origin);
      return stored ? { kind: 'bearer', accessToken: stored.token, refreshToken: stored.refreshToken } : { kind: 'anonymous' };
    },
    scope: guard,
    abortSubscriptions: () => guard.abortAll(),
  };
  return facade;
}
```

注意：`enrichIdentity` 对 `applySnapshot` 的引用是运行期晚绑定（const 声明顺序在前者定义之后无碍——调用发生在工厂函数体执行完毕之后）。

- [ ] **Step 4: 重写 services/runtime.ts**

`apps/miniprogram/src/services/runtime.ts` 全文替换为：

```ts
import '../platform/polyfills.ts';
import Taro from '@tarojs/taro';
import { createWeKnoraClient, createExecutionsApi, createServerSentEventParser, parseChatEvent, buildChatStreamRequest } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileResourceRemote } from '@weknora/api-client/mobile/resources';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createMobileRuntime } from '@weknora/mobile-core';
import type { MobileRuntime } from '@weknora/mobile-core';
import type { ChatStreamEvent } from '@weknora/contracts';
import type { ClientRequest, HttpRequest, HttpResult, NativeMultipartFileRequest } from '@weknora/api-client';
import { normalizeApiOrigin } from '../core/auth.ts';
import { storage, clearPrivateCache } from '../platform/storage.ts';
import { createWeappTransport, type WeappNetwork } from '../platform/transport.ts';
import { createAuthorizedRequestChannel, createAuthorizedStreamChannel } from '../platform/authorized-channels.ts';
import { createTaroCredentialStore, readStoredCredential } from '../platform/credential-store.ts';
import { createTaroSessionFacade, type TaroSessionFacade } from './session.ts';

// baseURL、可信来源校验、auth 存储 key 必须使用同一个 host 大小写归一化后的 origin，
// 否则同一 host 的不同大小写构建之间会话孤立，旧变体 key 的 token 会残留本机。
const origin = normalizeApiOrigin(__API_ORIGIN__);
const network: WeappNetwork = {
  request: options => Taro.request(options as Parameters<typeof Taro.request>[0]),
  uploadFile: options => Taro.uploadFile(options as Parameters<typeof Taro.uploadFile>[0]),
};
const native = createWeappTransport(network);

/** 未鉴权 ClientRequest 通道（Runtime remoteFor 与授权 REST 通道共用；Bearer 由调用方叠加）。 */
const plainRequests = new Map<string, (input: ClientRequest) => Promise<unknown>>();
function plainRequestFor(origin: string): (input: ClientRequest) => Promise<unknown> {
  let request = plainRequests.get(origin);
  if (!request) {
    request = createWeKnoraClient({ baseURL: origin, transport: { send: (r: HttpRequest) => native.send(r) } }).request;
    plainRequests.set(origin, request);
  }
  return request;
}

/**
 * MobileRuntime 是本小程序唯一会话编排器（issue #68 AC2 replace-dont-layer）：
 * 凭据只有 Runtime 一个写者；深模块（Task Office / Resource Shelf / Task Material）
 * 的 lease 与授权通道全部由它铸造。平台 Adapter 见 platform/*。
 */
export const runtime: MobileRuntime = createMobileRuntime({
  credentialStore: createTaroCredentialStore(storage, origin),
  clientVersion: CLIENT_PROTOCOL_VERSION,
  remoteFor(deployment) { return createMobileRuntimeRemote({ origin: deployment, request: plainRequestFor(deployment) }); },
  authorizedTransport(deployment) { return createAuthorizedRequestChannel(plainRequestFor(deployment)); },
  authorizedStream(deployment) { return createAuthorizedStreamChannel(network, deployment); },
  resourceShelf: { remoteFor(deployment) { return createMobileResourceRemote({ origin: deployment, request: plainRequestFor(deployment) }); } },
});

export const auth: TaroSessionFacade = createTaroSessionFacade({ runtime, origin, storage });

/**
 * 平台直连通道（multipart 上传/二进制下载）取 token 的唯一入口：
 * 先走一次授权 GET（触发 Runtime 的 refresh-once 并把轮换落盘），再「用时现读」。
 * 永不缓存 token、永不再造第二条刷新路径。
 */
export async function currentBearerToken(): Promise<string> {
  await runtime.authorizedRequest({ method: 'GET', path: '/api/v1/auth/me' });
  const stored = readStoredCredential(storage, origin);
  if (!stored) throw new Error('AUTH_REQUIRED');
  return stored.token;
}

/** 授权 JSON 通道上的应用客户端：send 走 authorizedRequest（401 刷新重放由 Runtime 负责；
 *  非 2xx 已以 ApiError 抛出，因此外壳状态为名义 200——真实语义在 Runtime 内）。 */
const authorizedTransport = {
  async send(r: HttpRequest): Promise<HttpResult> {
    const path = r.url.startsWith(origin) ? r.url.slice(origin.length) : r.url;
    const body = await runtime.authorizedRequest({ method: r.method, path, headers: r.headers, body: r.body, signal: r.signal });
    return { status: 200, headers: {}, body };
  },
  async sendBinary(r: HttpRequest): Promise<HttpResult> {
    const token = await currentBearerToken();
    return native.sendBinary({ ...r, headers: { ...r.headers, authorization: `Bearer ${token}` } });
  },
  async sendMultipartFile(r: NativeMultipartFileRequest): Promise<HttpResult> {
    const token = await currentBearerToken();
    return native.sendMultipartFile({ ...r, headers: { ...r.headers, authorization: `Bearer ${token}` } } as NativeMultipartFileRequest);
  },
};

export const client = createWeKnoraClient({ baseURL: origin, transport: authorizedTransport });
export const executions = createExecutionsApi(client.request);

export async function stream(input: HttpRequest, onText: (text: string) => void): Promise<void> {
  const path = input.url.startsWith(origin) ? input.url.slice(origin.length) : input.url;
  await runtime.authorizedEventStream({ method: input.method, path, headers: input.headers, body: input.body, signal: input.signal }, onText);
}

export async function chatStream(options: Parameters<typeof buildChatStreamRequest>[0], onEvent: (event: ChatStreamEvent) => void): Promise<void> {
  const req = buildChatStreamRequest(options), parser = createServerSentEventParser(frame => onEvent(parseChatEvent(frame)));
  await stream({ ...req, url: origin + req.path, headers: req.headers ?? {} }, text => parser.push(text));
  parser.finish();
}

export const apiOrigin = origin;

export async function logout(): Promise<void> {
  // 远端吊销尽力而为（Runtime signOut 只清本地）；网络失败不阻塞本地登出。
  try { await runtime.authorizedRequest({ method: 'POST', path: '/api/v1/auth/logout' }); } catch { /* silent: local logout always proceeds */ }
  await runtime.signOut();
  clearPrivateCache();
}

export function stopSubscriptions(): void { auth.abortSubscriptions(); }
```

（若 tsc 对 `native.sendBinary`/`native.sendMultipartFile` 的类型收窄有意见，保持与旧文件 `services/runtime.ts:46-47` 相同的写法：`sendBinary:r=>scoped(r,native.sendBinary)` 的等价直传即可。）

- [ ] **Step 5: 删除 tests/auth.test.mjs，修剪 tests/core.test.mjs**

```bash
git rm apps/miniprogram/tests/auth.test.mjs
```

`apps/miniprogram/tests/core.test.mjs`：删除 import 中的 `const execution = await import('../src/core/execution.ts');` 与 `const intent = await import('../src/core/intent.ts');`（保留 format/scope/utf8/routes），删除所有引用 `execution.`/`intent.PendingIntent` 的用例体，并在文件末尾追加从 auth.test.mjs 迁来的纯函数用例：

```js
const authPure = await import(`../src/core/auth.ts`);
test('normalizeApiOrigin lowercases the host and drops trailing slashes only', () => {
  assert.equal(authPure.normalizeApiOrigin('https://API.example.test/'), 'https://api.example.test');
  assert.equal(authPure.normalizeApiOrigin('https://api.example.test'), 'https://api.example.test');
  assert.equal(authPure.normalizeApiOrigin('https://api.example.test/x').endsWith('/x'), true, 'path is preserved');
});
test('isBearer accepts only a well-formed bearer record', () => {
  assert.equal(authPure.isBearer({ kind: 'bearer', accessToken: 'a' }), true);
  assert.equal(authPure.isBearer({ kind: 'bearer', accessToken: '' }), false);
  assert.equal(authPure.isBearer({ token: 'a' }), false);
});
```

- [ ] **Step 6: 运行确认通过**

Run: `cd apps/miniprogram && node --experimental-transform-types --test tests/assembly.test.mjs tests/core.test.mjs tests/transport.test.mjs tests/platform-adapters.test.mjs`
Expected: PASS（assembly 10 用例 + core 修剪后用例 + transport 既有 + platform-adapters 7 用例；`tests/office-assembly.test.mjs` 此时还不存在）。

此时**旧 workbench.ts 仍可编译工作**（其消费的 client/executions/auth.scope 导出未变）——中间态可编译是 Task 5 删除它的前提。跑一次全量确认：

Run: `cd apps/miniprogram && node --experimental-transform-types --test tests/*.test.mjs && npx tsc --noEmit 2>&1 | grep 'error TS' | grep -v 'features/account/pages.tsx' || true`
Expected: 测试全绿；tsc 过滤后无输出（account 页 13 个先在错误除外）。

- [ ] **Step 7: Commit**

```bash
git add apps/miniprogram/src/services/runtime.ts apps/miniprogram/src/services/session.ts apps/miniprogram/src/core/auth.ts apps/miniprogram/tests/assembly.test.mjs apps/miniprogram/tests/core.test.mjs
git commit -m "feat(miniprogram): MobileRuntime becomes the single session orchestrator (replace-dont-layer)"
```

---

### Task 4: 深模块组合根 mobile-office + 关键 scenario（AC3 Interface 级证据）

**Files:**
- Create: `apps/miniprogram/src/services/mobile-office.ts`
- Create: `apps/miniprogram/src/services/office-views.ts`
- Test: `apps/miniprogram/tests/office-assembly.test.mjs`
- Test: `apps/miniprogram/tests/office-views.test.mjs`

**Interfaces:**
- Consumes: `runtime`/`executions`（Task 3 导出）；`createTaskOffice`/`createTaskMaterial`/`TaskOffice`/`TaskMaterial`/`TaskMaterialHandle`/`ResourceShelfHandle`（mobile-core）；`createTaskOfficeRemote`/`createMobileMaterialRemote`（api-client）；`createTaroBlobFetch`/`createTaroIntentLog`（Task 2）；`requestId`（core/intent.ts）。
- Produces（Task 5 页面消费）:
  - `activeTaskOffice(): TaskOffice | undefined`
  - `openActiveMaterial(): TaskMaterialHandle | undefined`
  - `activeResourceShelf(): ResourceShelfHandle | undefined`
  - `resolveTaskForRun(runId: string): Promise<{ taskId: string; runId: string }>`
  - `requireTaskOffice(): TaskOffice`（未授权时抛错，页面 loader 用）
  - office-views：`runStatusLabels: Record<string, string>`、`runStatusBadgeTone(runStatus): 'success'|'warning'|'info'|'neutral'`、`connectionLabel(connection: TaskConnectionState): string`、`interruptionNotice(reason: TaskInterruptionReason): string`、`inboxVisibleItems(items: InboxItem[], runId?: string): InboxItem[]`、`decisionReceiptText(receipt: AttentionDecisionReceipt): string`、`materialEntryRow(entry: MaterialEntry): { title: string; subtitle: string }`

- [ ] **Step 1: 写失败测试（office-views 纯映射）**

创建 `apps/miniprogram/tests/office-views.test.mjs`：

```js
import test from 'node:test';
import assert from 'node:assert/strict';
const views = await import('../src/services/office-views.ts');

test('run status labels cover the six states and fall back to the raw value', () => {
  for (const status of ['queued', 'running', 'waiting_user', 'reconciling', 'succeeded', 'failed', 'canceled']) {
    assert.equal(typeof views.runStatusLabels[status], 'string');
  }
  assert.equal(views.runStatusLabels.weird ?? 'weird', 'weird');
});
test('badge tone maps attention-carrying states to warning, success only for succeeded', () => {
  assert.equal(views.runStatusBadgeTone('waiting_user'), 'warning');
  assert.equal(views.runStatusBadgeTone('succeeded'), 'success');
  assert.equal(views.runStatusBadgeTone('running'), 'info');
  assert.equal(views.runStatusBadgeTone('whatever'), 'neutral');
});
test('inbox filtering: optional run filter keeps cross-run inbox as the source of truth', () => {
  const items = [
    { interactionId: 'i1', runId: 'run-1', kind: 'tool_approval', argsHash: 'h', expectedRevision: 2, createdAt: '2026-09-24T00:00:00Z' },
    { interactionId: 'i2', runId: 'run-2', kind: 'budget', argsHash: 'h', expectedRevision: 1, createdAt: '2026-09-24T00:00:01Z' },
  ];
  assert.equal(views.inboxVisibleItems(items).length, 2);
  assert.equal(views.inboxVisibleItems(items, 'run-1').length, 1);
  assert.equal(views.inboxVisibleItems(items, 'run-1')[0].interactionId, 'i1');
});
test('decision receipt copy is honest for all four states', () => {
  assert.match(views.decisionReceiptText({ status: 'recorded', record: { interactionId: 'i1', runId: 'r', kind: 'tool_approval', decisionId: 'd1', action: 'reject', argsHash: 'h', expectedRevision: 2 } }), /已记录/);
  assert.match(views.decisionReceiptText({ status: 'delivery-unknown', interactionId: 'i1', decisionId: 'd1' }), /不确定/);
  assert.match(views.decisionReceiptText({ status: 'superseded', interactionId: 'i1' }), /已被取代/);
  assert.match(views.decisionReceiptText({ status: 'gone', interactionId: 'i1' }), /已失效/);
});
test('interruption notices name the reason without inventing recovery promises', () => {
  assert.match(views.interruptionNotice('gap'), /缺口|重新同步/);
  assert.match(views.interruptionNotice('cursor-expired'), /游标/);
  assert.match(views.interruptionNotice('stream-error'), /连接/);
});
```

Run: `cd apps/miniprogram && node --experimental-transform-types --test tests/office-views.test.mjs`
Expected: FAIL（模块不存在）。

- [ ] **Step 2: 写 office-views 最小实现**

创建 `apps/miniprogram/src/services/office-views.ts`：

```ts
import type { AttentionDecisionReceipt, InboxItem } from '@weknora/mobile-core';
import type { TaskConnectionState, TaskInterruptionReason } from '@weknora/mobile-core';
import type { MaterialEntry } from '@weknora/mobile-core';
import { formatBytes, formatTime } from '../core/format.ts';

export const runStatusLabels: Record<string, string> = {
  queued: '排队中', running: '运行中', waiting_user: '待你确认', reconciling: '核对状态中',
  succeeded: '已完成', failed: '失败', canceled: '已取消',
};

export function runStatusBadgeTone(runStatus: string): 'success' | 'warning' | 'info' | 'neutral' {
  if (runStatus === 'succeeded') return 'success';
  if (runStatus === 'waiting_user' || runStatus === 'failed') return 'warning';
  if (runStatus === 'running' || runStatus === 'reconciling' || runStatus === 'queued') return 'info';
  return 'neutral';
}

export function connectionLabel(connection: TaskConnectionState): string {
  return { syncing: '同步中', live: '已连接', interrupted: '连接中断，可恢复', drained: '已同步' }[connection] ?? connection;
}

export function interruptionNotice(reason: TaskInterruptionReason): string {
  const copy: Record<TaskInterruptionReason, string> = {
    'gap': '事件出现缺口，正在自动重新同步；不会重放已完成的工作。',
    'cursor-expired': '服务端游标已过期裁剪，正在从快照重新同步。',
    'stream-error': '连接中断；页面保留已同步状态，可手动重新同步。',
    'stream-ended-nonterminal': '连接在非终态结束，正在核对最新状态。',
    'persist-failed': '本机缓存写入失败；已提交的服务端状态不受影响。',
    'stream-unavailable': '当前部署未提供流式通道；只能整段刷新快照。',
  };
  return copy[reason];
}

/** 收件箱过滤：源是跨 run 的 inbox()；按 run 进入时仅做视图过滤，不改权威列表。 */
export function inboxVisibleItems(items: InboxItem[], runId?: string): InboxItem[] {
  return runId ? items.filter(item => item.runId === runId) : items;
}

/** 四态回执如实文案：recorded 只表示决定已记录，绝不解释为外部派发完成（#38 AC2）。 */
export function decisionReceiptText(receipt: AttentionDecisionReceipt): string {
  switch (receipt.status) {
    case 'recorded': return '决定已记录。外部动作是否已派发完成以执行状态为准。';
    case 'delivery-unknown': return '结果不确定：服务端未确认该决定是否送达，请稍后重查，不要盲目重复提交。';
    case 'superseded': return '该事项已被新的决定取代。';
    case 'gone': return '该事项已失效（不存在或已过期）。';
  }
}

export function materialEntryRow(entry: MaterialEntry): { title: string; subtitle: string } {
  const kindLabel = entry.kind === 'test-report' ? '测试报告' : entry.kind === 'diff' ? 'Diff' : '产物';
  return { title: entry.name || `${kindLabel} #${entry.index}`, subtitle: `${kindLabel} · ${entry.mime} · ${formatBytes(entry.size)} · 版本 ${entry.version.slice(0, 8)}` };
}
```

Run: `cd apps/miniprogram && node --experimental-transform-types --test tests/office-views.test.mjs`
Expected: PASS。

- [ ] **Step 3: 写失败测试（office-assembly：关键 scenario 走真实源码 + taro-stub 后端）**

创建 `apps/miniprogram/tests/office-assembly.test.mjs`：

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import { registerHooks } from 'node:module';
import { pathToFileURL } from 'node:url';

const stubURL = pathToFileURL(new URL('./helpers/taro-stub.mjs', import.meta.url).pathname).href;
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === '@tarojs/taro') return { url: stubURL, shortCircuit: true };
    return nextResolve(specifier, context);
  },
});
globalThis.__API_ORIGIN__ = 'https://api.example.test';
const { stub } = await import('./helpers/taro-stub.mjs');
const runtimeModule = await import('../src/services/runtime.ts');
const office = await import('../src/services/mobile-office.ts');

// activateTenant 以切换后的 me() 为身份权威（mobile-runtime.ts:426-429）：activeTenant 随路由翻转。
let activeTenant = 1;
const me = () => ({ success: true, data: { user: { id: 'u1', username: 'Lin' }, tenant: { id: activeTenant, name: activeTenant === 1 ? 'Space' : 'Space 2' }, memberships: [] } });
const capabilities = () => ({ success: true, data: { protocol_minimum: 1, protocol_maximum: 5 } });
const runRow = (runId, sessionId, status, attention = 'none') => ({ run_id: runId, session_id: sessionId, title: `任务 ${runId}`, status, run_status: status, attention, updated_at: '2026-09-24T00:00:00Z' });
const overviewRow = runRow;
const execDto = (seq = 5) => ({ schema_version: 1, run_id: 'run-1', session_id: 's-1', revision: 3, driver: 'platform', run_status: 'running', execution_status: 'running', settlement_status: 'pending', seq, capabilities: {} });
const execEvent = seq => ({ schema_version: 1, run_id: 'run-1', attempt_id: 'a-1', seq, type: 'progress', occurred_at: '2026-09-18T00:00:00Z', payload: { summary: 'working' } });

function backend(routes) {
  stub.use(call => {
    const method = call.options.method, path = decodeURIComponent(new URL(call.options.url).pathname);
    let fn = routes[`${method} ${path}`];
    if (fn === undefined) {
      const prefix = Object.keys(routes).filter(k => k.endsWith('/') && `${method} ${path}`.startsWith(k)).sort((a, b) => b.length - a.length)[0];
      if (prefix) fn = routes[prefix];
    }
    if (fn === undefined && call.options.enableChunked) {
      // SSE 路由未命中时挂起等待测试驱动
      return;
    }
    if (fn === undefined) { call.options.fail({ errMsg: `no backend route for ${method} ${path}` }); return; }
    fn(call);
  });
}
const authRoutes = extra => ({
  'POST /api/v1/auth/login': call => stub.succeed(call, { data: { success: true, data: { token: 't1', refresh_token: 'r1' } } }),
  'GET /api/v1/auth/me': call => stub.succeed(call, { data: me() }),
  'GET /api/v1/system/capabilities': call => stub.succeed(call, { data: capabilities() }),
  ...extra,
});
async function freshLogin(extra = {}) { stub.reset(); backend(authRoutes(extra)); await runtimeModule.auth.login('u@example.test', 'pw'); }
const authorization = call => call.options.header.authorization ?? call.options.header.Authorization;
const settle = ms => new Promise(resolve => setTimeout(resolve, ms ?? 30));
async function until(predicate, ms = 1500) { const end = Date.now() + ms; while (Date.now() < end) { if (predicate()) return true; await settle(10); } return predicate(); }

const overviewRoutes = () => ({
  'GET /api/v1/workbench/overview': call => stub.succeed(call, { data: { success: true, data: {
    pending_interactions: [{ id: 'i-1', kind: 'tool_approval', created_at: '2026-09-24T00:00:00Z' }],
    in_progress: [overviewRow('run-1', 's-1', 'running')],
    recently_completed: [overviewRow('run-9', 's-9', 'succeeded')],
    counts: { unread_notifications: 3 }, as_of: '2026-09-24T01:00:00Z',
  } } }),
});

test('scenario: home() aggregates three sections through TaskOffice (GET /workbench/overview)', async () => {
  await freshLogin(overviewRoutes());
  const o = office.requireTaskOffice();
  const home = await o.home();
  assert.equal(home.needsMe.length, 1);
  assert.equal(home.running[0].runId, 'run-1');
  assert.equal(home.recentlyCompleted[0].runId, 'run-9');
  assert.equal(home.unreadNotifications, 3);
});

test('scenario: tasks()/moreTasks() own the cursor and suppress duplicate runIds across pages', async () => {
  let page = 0;
  await freshLogin({
    'GET /api/v1/workbench/executions': call => {
      page += 1;
      const items = page === 1 ? [runRow('run-1', 's-1', 'running'), runRow('run-2', 's-2', 'waiting_user', 'required')] : [runRow('run-2', 's-2', 'waiting_user', 'required'), runRow('run-3', 's-3', 'succeeded')];
      stub.succeed(call, { data: { success: true, data: { items, next_cursor: page === 1 ? 'c2' : undefined } } });
    },
  });
  const o = office.requireTaskOffice();
  const first = await o.tasks({});
  assert.equal(first.items.length, 2);
  assert.equal(first.nextCursor, 'c2');
  const second = await o.moreTasks();
  assert.deepEqual(second.duplicateRunIds, ['run-2'], 'duplicate run across pages is observable, never rendered twice');
  assert.equal(second.items.filter(item => item.runId === 'run-2').length, 0);
  assert.equal(second.items.length, 1);
});

test('scenario: start() persists intent before dispatch and resubmits the SAME request_id after an unknown lookup', async () => {
  const starts = [], lookups = [];
  let failFirst = true;
  await freshLogin({
    'POST /api/v1/sessions': call => stub.succeed(call, { data: { success: true, data: { id: 's-new' } } }),
    'POST /api/v1/workbench/executions': call => {
      starts.push(call.options.data);
      if (failFirst) { failFirst = false; call.options.fail({ errMsg: 'request lost' }); return; }
      stub.succeed(call, { statusCode: 202, data: { success: true, data: { run_id: 'run-9', request_id: call.options.data.request_id, status: 'admitted' } } });
    },
    'GET /api/v1/workbench/executions/requests/': call => {
      lookups.push(new URL(call.options.url).pathname.split('/').pop());
      stub.succeed(call, { data: { success: true, data: { state: 'unknown' } } });
    },
  });
  const o = office.requireTaskOffice();
  const goal = { text: '整理本周工作，生成一份周报', agentId: 'agent-1', budgetUpper: 200 };
  // 网络错误可能经 domain coordinator 原样透传（message='NETWORK_ERROR'）或被包为
  // TASK_OFFICE_BACKEND（cause 链里是 NETWORK_ERROR）——两种都证明「歧义失败如实上抛」。
  await assert.rejects(o.start(goal), error => /NETWORK_ERROR/i.test(`${error.message} ${error.cause?.message ?? ''} ${error.code ?? ''}`));
  assert.equal(starts.length, 1, 'no blind resubmission while the outcome is ambiguous');
  const receipt = await o.start(goal);
  assert.equal(receipt.dispatched, true);
  assert.equal(receipt.runId, 'run-9');
  assert.equal(starts.length, 2, 'unknown lookup authorizes resubmission');
  assert.equal(starts[1].request_id, starts[0].request_id, 'the SAME intent keeps the SAME request_id (durable identity)');
  assert.equal(starts[1].session_id, 's-new', 'reentry reuses the persisted session, never creates a second one');
});

test('scenario: open() hydrates from the snapshot and appends SSE events past the watermark', async () => {
  let eventStreams = 0;
  await freshLogin({
    'GET /api/v1/workbench/executions/run-1/snapshot': call => stub.succeed(call, { data: { success: true, data: { execution: execDto(5), task: { task_id: 's-1', title: '任务 run-1', attention: 'none' }, watermark: 5, incomplete: false, confirmed_watermark: 5, events: [execEvent(5)] } } }),
    'GET /api/v1/workbench/executions/run-1/events': call => {
      eventStreams += 1;
      assert.equal(call.options.header['Last-Event-ID'], '5', 'resume must continue from the snapshot watermark');
      stub.emitHeaders(call, { 'Content-Type': 'text/event-stream' });
      stub.emitChunk(call, new TextEncoder().encode(`data: ${JSON.stringify(execEvent(6))}\n\n`).buffer);
      stub.succeed(call, { statusCode: 200, header: { 'Content-Type': 'text/event-stream' }, data: '' });
    },
  });
  const handle = office.requireTaskOffice().open({ taskId: 's-1', runId: 'run-1' });
  const updates = [];
  const off = handle.updates(view => updates.push(view));
  const initial = await handle.hydrate();
  assert.equal(initial.taskId, 's-1');
  assert.equal(initial.cursor, 5);
  assert.ok(await until(() => updates.some(view => view.cursor === 6)), 'streamed event appends past the watermark');
  const latest = updates.at(-1);
  assert.equal(latest.timeline.at(-1).seq, 6);
  off();
  handle.close('test-done');
  await settle();
  assert.equal(eventStreams, 1, 'no reconnect after close');
});

test('scenario: inbox() reads the cross-run pending list and decide() distinguishes recorded from superseded', async () => {
  const decisions = [];
  await freshLogin({
    'GET /api/v1/workbench/interactions': call => {
      const limit = new URL(call.options.url).searchParams.get('limit');
      assert.ok(Number(limit) >= 1, 'inbox carries an explicit limit');
      stub.succeed(call, { data: { success: true, data: [
        { id: 'i-1', run_id: 'run-1', kind: 'tool_approval', args_hash: 'h1', expected_revision: 2, created_at: '2026-09-24T00:00:00Z' },
      ] } });
    },
    'POST /api/v1/workbench/executions/interactions/i-1/decisions': call => {
      decisions.push(call.options.data);
      if (decisions.length === 1) stub.succeed(call, { data: { success: true, data: { id: 'i-1', run_id: 'run-1', kind: 'tool_approval', decision_id: call.options.data.decision_id, action: 'reject', args_hash: 'h1', expected_revision: 2 } } });
      else stub.succeed(call, { statusCode: 409, data: { success: false, code: 'interaction_superseded' } });
    },
  });
  const o = office.requireTaskOffice();
  const inbox = await o.inbox();
  assert.equal(inbox.items.length, 1);
  assert.equal(inbox.items[0].runId, 'run-1');
  const item = inbox.items[0];
  const recorded = await o.decide({ item, action: 'reject' });
  assert.equal(recorded.status, 'recorded');
  assert.equal(recorded.record.action, 'reject');
  assert.ok(typeof decisions[0].decision_id === 'string' && decisions[0].decision_id !== '', 'decision_id is a non-empty durable identity minted by the module');
  const superseded = await o.decide({ item, action: 'reject' });
  assert.equal(superseded.status, 'superseded');
});

test('scenario: shelf browse projects the three classes with verdicts; a 403 marks the class forbidden and revokes', async () => {
  await freshLogin({
    'GET /api/v1/agents': call => stub.succeed(call, { data: { success: true, data: [{ id: 'agent-1', name: 'Helper', description: '帮手', is_builtin: true }], disabled_own_agent_ids: [] } }),
    'GET /api/v1/knowledge-bases': call => stub.succeed(call, { data: { success: true, data: [{ id: 7, name: '团队知识', knowledge_count: 3, updated_at: '2026-09-24T00:00:00Z' }] } }),
    'GET /api/v1/apps/connections': call => stub.succeed(call, { data: { success: true, data: [{ id: 'conn-1', kind: 'github', state: 'active' }] } }),
  });
  const shelf = office.activeResourceShelf();
  const page = await shelf.browse();
  assert.equal(page.agents.length, 1);
  assert.equal(page.agents[0].name, 'Helper');
  assert.equal(page.knowledge.length, 1);
  assert.equal(page.connections.length, 1);
  assert.equal(page.classVerdicts.agent.state, 'supported');
  const verdict = shelf.selection({ agentId: 'agent-1' });
  assert.deepEqual(verdict, { allowed: true, selection: { kind: 'agent', agentId: 'agent-1' } });
  // 403 → 类级 forbidden + authorization-revoked 事件（#33 语义）。
  // 只重装路由 handler（backend 覆盖式），不 reset——shelf 用内存 activeCredential，但保持 storage 完好是通用纪律。
  const events = [];
  shelf.subscribe(event => events.push(event));
  backend(authRoutes({
    'GET /api/v1/agents': call => stub.succeed(call, { statusCode: 403, data: { success: false } }),
  }));
  const forbidden = await shelf.browse();
  assert.equal(forbidden.classVerdicts.agent.state, 'forbidden');
  assert.equal(forbidden.agents.length, 0, 'failed class clears the projection, never backfills stale rows');
  assert.ok(events.some(event => event.type === 'authorization-revoked' && event.resourceClass === 'agent'));
});

test('scenario: material index + preview + terminal flow through TaskMaterial', async () => {
  await freshLogin({
    'GET /api/v1/workbench/executions/run-1/artifacts': call => stub.succeed(call, { data: { success: true, data: { version: '0123456789abcdef', artifacts: [{ index: 0, file_name: 'report.txt', file_type: 'text/plain', size: 5 }], terminal: { available: true } } } }),
    'POST /api/v1/workbench/executions/run-1/artifacts/0/signed-url': call => stub.succeed(call, { data: { success: true, data: { url: 'https://api.example.test/download/report.txt', expires_at: '2026-09-24T01:00:00Z' } } }),
    'GET /download/report.txt': call => stub.succeed(call, { statusCode: 200, header: { 'content-type': 'text/plain' }, data: new TextEncoder().encode('hello').buffer }),
    'GET /api/v1/workbench/executions/run-1/terminal-log': call => stub.succeed(call, { data: { success: true, data: { lines: [{ seq: 1, occurred_at: '2026-09-24T00:00:00Z', stream: 'stdout', text: 'build ok' }], next_cursor: 1 } } }),
  });
  const material = office.openActiveMaterial();
  const index = await material.index({ runId: 'run-1' });
  assert.equal(index.materials.length, 1);
  assert.equal(index.terminal.available, true);
  const entry = index.materials[0];
  assert.equal(entry.kind, 'artifact');
  const view = await material.open({ kind: entry.kind, runId: 'run-1', materialId: entry.materialId });
  assert.equal(view.kind, 'artifact');
  assert.equal(view.preview.state, 'supported', 'small text/plain preview is supported');
  assert.equal(view.text, 'hello');
  const terminal = await material.open({ kind: 'terminal', runId: 'run-1' });
  assert.equal(terminal.kind, 'terminal');
  assert.equal(terminal.lines[0].text, 'build ok');
  material.close('scenario-done');
});

test('scenario: tenant switch revokes the old office lease — late reads fail closed (TASK_OFFICE_SCOPE_CHANGED)', async () => {
  await freshLogin({
    ...overviewRoutes(),
    'POST /api/v1/auth/switch-tenant': call => { activeTenant = 2; stub.succeed(call, { data: { success: true, data: { token: 't3', refresh_token: 'r3', tenant: { id: 2, name: 'Space 2' }, memberships: [] } } }); },
  });
  const staleOffice = office.requireTaskOffice();
  await runtimeModule.auth.switchTenant(2);
  await assert.rejects(staleOffice.home(), error => error.code === 'TASK_OFFICE_SCOPE_CHANGED');
  const nextOffice = office.requireTaskOffice();
  const home = await nextOffice.home();
  assert.equal(home.running[0].runId, 'run-1', 'a fresh office for the new scope reads normally');
});

test('scenario: resolveTaskForRun derives taskId from the authoritative run row (ADR-0004 task-is-session)', async () => {
  await freshLogin({
    'GET /api/v1/workbench/executions/run-1': call => stub.succeed(call, { data: { success: true, data: execDto(5) } }),
  });
  const resolved = await office.resolveTaskForRun('run-1');
  assert.deepEqual(resolved, { taskId: 's-1', runId: 'run-1' });
});
```

Run: `cd apps/miniprogram && node --experimental-transform-types --test tests/office-assembly.test.mjs`
Expected: FAIL——`Cannot find module '../src/services/mobile-office.ts'`。

- [ ] **Step 4: 写 mobile-office.ts 最小实现**

创建 `apps/miniprogram/src/services/mobile-office.ts`：

```ts
import { createTaskOffice, createTaskMaterial } from '@weknora/mobile-core';
import type { ResourceShelfHandle, TaskMaterial, TaskMaterialHandle, TaskOffice } from '@weknora/mobile-core';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createMobileMaterialRemote } from '@weknora/api-client/mobile/materials';
import { createTaroBlobFetch } from '../platform/authorized-channels.ts';
import { createTaroIntentLog } from '../platform/intent-log.ts';
import { storage } from '../platform/storage.ts';
import { requestId } from '../core/intent.ts';
import { runtime, executions, network } from './runtime.ts';

/**
 * 深模块组合根（issue #68 What-to-build）：小程序的 Task/Resource/Material 关键 scenario
 * 全部经由 @weknora/mobile-core 的深模块 Interface——与 apps/mobile/src/composition.ts 同一
 * 装配、不同平台 Adapter。lease/授权通道/token 纪律全部由 MobileRuntime 铸造。
 */

/** 记忆化键 = origin::tenant（与 apps/mobile deploymentScopeKey 同语义）：切租户/换部署不复用含旧 scope 状态的实例。 */
const MAX_CACHE_ENTRIES = 8;
const cachePut = <T>(cache: Map<string, T>, key: string, make: () => T): T => {
  const existing = cache.get(key);
  if (existing !== undefined) return existing;
  if (cache.size >= MAX_CACHE_ENTRIES) cache.delete(cache.keys().next().value!);
  const created = make();
  cache.set(key, created);
  return created;
};

function deploymentScope(): { origin: string; tenantId: string } | undefined {
  const snapshot = runtime.snapshot();
  if (snapshot.surface !== 'authorized') return undefined;
  const origin = snapshot.deployment?.origin;
  const tenantId = snapshot.identity?.activeTenantId;
  if (!origin || !tenantId) return undefined;
  return { origin, tenantId };
}

const intentLog = createTaroIntentLog(storage);

const offices = new Map<string, TaskOffice>();

export function activeTaskOffice(): TaskOffice | undefined {
  const scope = deploymentScope();
  if (!scope) return undefined;
  return cachePut(offices, `${scope.origin}::${scope.tenantId}`, () => {
    const remote = createTaskOfficeRemote({
      origin: scope.origin,
      request: input => runtime.authorizedRequest(input),
      stream: (input, onChunk) => runtime.authorizedEventStream(input, onChunk),
    });
    return createTaskOffice({
      backend: remote,
      detail: remote,
      interactions: remote,
      lease: () => runtime.scopeLease(),
      // 耐久意图日志（storage）：Start POST 前落盘，重启后 reconcilePending 恢复。
      intentLog,
      // 平台无 crypto.randomUUID 时必须显式注入（#36 契约）。
      newRequestId: requestId,
      // TaskProjectionStore 持久化选型（SQLite/SecureStore）是 B2-F23 未决项——此处显式
      // 采用 in-memory（缺省），使「未注入持久化」成为组合根的显式决策而非静默回退。
    });
  });
}

export function requireTaskOffice(): TaskOffice {
  const office = activeTaskOffice();
  if (!office) throw new Error('任务面板尚未就绪（未授权或缺少活跃空间）');
  return office;
}

const materials = new Map<string, TaskMaterial>();

export function openActiveMaterial(): TaskMaterialHandle | undefined {
  const scope = deploymentScope();
  const lease = runtime.scopeLease();
  if (!scope || !lease) return undefined;
  const material = cachePut(materials, scope.origin, () => createTaskMaterial({
    remote: createMobileMaterialRemote({ origin: scope.origin, request: input => runtime.authorizedRequest(input) }),
    blob: createTaroBlobFetch(network),
    // share 缺省 fail closed（MATERIAL_SHARE_UNAVAILABLE）：微信分享面板属真机验收（spec Testing Decisions）。
  }));
  return material.open({ lease });
}

export function activeResourceShelf(): ResourceShelfHandle | undefined {
  return runtime.resourceShelf();
}

/** 按 runId 恢复任务详情的权威途径：run 行的 session_id 就是 taskId（ADR-0004）。 */
export async function resolveTaskForRun(runId: string): Promise<{ taskId: string; runId: string }> {
  const execution = await executions.get(runId);
  const sessionId = (execution as { session_id?: unknown }).session_id;
  if (sessionId === undefined || sessionId === null) throw new Error('该执行缺少会话身份，无法打开任务详情');
  return { taskId: String(sessionId), runId };
}
```

Run: `cd apps/miniprogram && node --experimental-transform-types --test tests/office-assembly.test.mjs tests/office-views.test.mjs`
Expected: PASS（10 + 5 用例绿）。若个别断言失败，按「测试是对的、实现没对齐 Interface」排查（不要改测试迁就实现；除非断言与前置批次的已合并语义冲突——那要回到差异记录）。

- [ ] **Step 5: 全量回归**

Run: `cd apps/miniprogram && node --experimental-transform-types --test tests/*.test.mjs && npx tsc --noEmit 2>&1 | grep 'error TS' | grep -v 'features/account/pages.tsx' || true`
Expected: 全部测试绿；tsc 过滤后无输出。

- [ ] **Step 6: Commit**

```bash
git add apps/miniprogram/src/services/mobile-office.ts apps/miniprogram/src/services/office-views.ts apps/miniprogram/tests/office-assembly.test.mjs apps/miniprogram/tests/office-views.test.mjs
git commit -m "feat(miniprogram): key scenarios run through the shared Task Office / Resource Shelf / Task Material interfaces"
```

---

### Task 5: 页面迁移到深模块 + 删除手写 controller（replace-dont-layer 收口）

**Files:**
- Modify: `apps/miniprogram/src/features/execution/pages.tsx`（TasksPage/ExecutionPage/ApprovalPage/ArtifactPage 全部重写）
- Modify: `apps/miniprogram/src/features/home/pages.tsx`（AgentsPage/AgentPage 重写；HomePage 逐字保留）
- Delete: `apps/miniprogram/src/services/workbench.ts`、`apps/miniprogram/src/core/execution.ts`
- Modify: `apps/miniprogram/src/core/intent.ts`（删 `PendingIntent`，保留 `ValueStore`/`requestId`）
- Test: `apps/miniprogram/tests/core.test.mjs`（已在 Task 3 修剪；本任务确认无 execution/intent 残留引用）

**Interfaces:**
- Consumes: Task 4 全部 Produces（`requireTaskOffice`/`openActiveMaterial`/`activeResourceShelf`/`resolveTaskForRun`/office-views）；`executions.command`（steer/cancel——`act(TaskIntent)` 属 #37，未交付，命令面继续走授权 API client，属已声明边界）；`timelineKindLabel`（mobile-core 导出）。
- Produces: 页面不再引用 `services/workbench.ts` 的任何符号——该文件可安全删除（本任务的删除步骤即 replace-dont-layer 的「删旧公开 controller」动作）。

- [ ] **Step 1: 先删旧 controller（RED——引用断开后 typecheck 失败，驱动页面迁移）**

```bash
git rm apps/miniprogram/src/services/workbench.ts apps/miniprogram/src/core/execution.ts
```

`apps/miniprogram/src/core/intent.ts` 全文替换为：

```ts
export interface ValueStore { read(key: string): unknown; write(key: string, value: unknown): void; remove(key: string): void; keys?(): string[] }
let sequence=0;
/** Unique intent correlation, NOT a password, credential, signature or auth token. */
export function requestId(): string { sequence+=1; return `mini-${Date.now().toString(36)}-${sequence.toString(36)}-${Math.random().toString(36).slice(2,14)}`; }
```

Run: `cd apps/miniprogram && npx tsc --noEmit 2>&1 | grep 'error TS' | grep -v 'features/account/pages.tsx' | head -20`
Expected: FAIL——`features/execution/pages.tsx` 与 `features/home/pages.tsx` 报「Cannot find module '../services/workbench.ts'」「Cannot find module '../../core/execution.ts'」（这就是本任务的 RED）。

- [ ] **Step 2: 重写 features/execution/pages.tsx**

`apps/miniprogram/src/features/execution/pages.tsx` 全文替换为：

```tsx
import { useEffect, useRef, useState, useCallback } from 'react';
import { View, Text } from '@tarojs/components';
import { useDidHide, useDidShow } from '@tarojs/taro';
import { timelineKindLabel } from '@weknora/mobile-core';
import type { AttentionDecisionReceipt, TaskDetailView, TaskListPage, TaskStartReceipt } from '@weknora/mobile-core';
import { Screen, Card, Action, Field, Notice, Section, ListRow, Badge, Empty, useData, DataBoundary, useAction, confirmAction } from '../../components/ui.tsx';
import { auth, executions } from '../../services/runtime.ts';
import { requireTaskOffice, openActiveMaterial, resolveTaskForRun } from '../../services/mobile-office.ts';
import { runStatusLabels, runStatusBadgeTone, connectionLabel, interruptionNotice, inboxVisibleItems, decisionReceiptText, materialEntryRow } from '../../services/office-views.ts';
import { navigate, routeParam } from '../../platform/navigation.ts';
import { errorMessage } from '../../core/errors.ts';
import { formatTime } from '../../core/format.ts';

const openTask=(taskId:string,runId:string)=>navigate('execution',{id:runId,task:taskId});

export function TasksPage(){
 const [filter,setFilter]=useState<''|'running'|'waiting_user'|'succeeded'>('');const [pages,setPages]=useState<TaskListPage[]>([]);const [lookupId,setLookupId]=useState('');const [receipt,setReceipt]=useState<string>();const action=useAction();
 const home=useData('tasks-home',()=>requireTaskOffice().home());
 const query=useData(`tasks:${filter}`,async()=>{const office=requireTaskOffice();const first=await office.tasks(filter?{status:filter}:{});setPages([first]);return first});
 const more=useAction();
 const pending=useData('tasks-pending',()=>requireTaskOffice().reconcilePending());
 return <Screen title='任务' tab><View className='wk-between'><Text className='wk-display'>工作正在向前。</Text><Action secondary onClick={()=>void navigate('agents')}>新任务</Action></View>
 <DataBoundary state={home}>{h=><>{h.needsMe.length>0&&<Card tone='warning'><Text className='wk-h3'>{h.needsMe.length} 项待你确认</Text><Action secondary onClick={()=>void navigate('approval')}>打开确认收件箱</Action></Card>}
 <Section title='正在运行'/>{h.running.map(item=><Card key={item.runId}><ListRow title={item.title} subtitle={formatTime(item.updatedAt)} icon='tasks' onClick={()=>void openTask(item.taskId,item.runId)} suffix={<Badge tone={runStatusBadgeTone(item.runStatus)}>{runStatusLabels[item.runStatus]??item.runStatus}</Badge>}/></Card>)}
 <Section title='最近完成'/>{h.recentlyCompleted.map(item=><Card key={item.runId}><ListRow title={item.title} subtitle={formatTime(item.updatedAt)} icon='tasks' onClick={()=>void openTask(item.taskId,item.runId)} suffix={<Badge tone='neutral'>{runStatusLabels[item.runStatus]??item.runStatus}</Badge>}/></Card>)}</>}</DataBoundary>
 <View className='wk-filters'>{([['','全部'],['running','运行中'],['waiting_user','待确认'],['succeeded','已完成']] as const).map(([id,name])=><Action key={id} secondary={filter!==id} onClick={()=>{setFilter(id);setPages([])}}>{name}</Action>)}</View>
 <DataBoundary state={query}>{first=><>{[...pages.flatMap(page=>page.items)].length?pages.flatMap((page,pi)=><View key={pi}>{page.items.map(item=><Card key={item.runId}><ListRow title={item.title} subtitle={formatTime(item.updatedAt)} icon='tasks' onClick={()=>void openTask(item.taskId,item.runId)} suffix={<Badge tone={runStatusBadgeTone(item.runStatus)}>{runStatusLabels[item.runStatus]??item.runStatus}</Badge>}/></Card>)}</View>):first.items.map(item=><Card key={item.runId}><ListRow title={item.title} subtitle={formatTime(item.updatedAt)} icon='tasks' onClick={()=>void openTask(item.taskId,item.runId)} suffix={<Badge tone={runStatusBadgeTone(item.runStatus)}>{runStatusLabels[item.runStatus]??item.runStatus}</Badge>}/></Card>)}
 {(pages.at(-1)?.nextCursor??first.nextCursor)&&<Action secondary loading={more.busy} onClick={()=>void more.run(async()=>{const page=await requireTaskOffice().moreTasks();setPages(p=>[...p,page])})}>加载更多</Action>}
 {!pages.length&&!first.items.length&&<Empty title='当前筛选下暂无任务'/>}</>}</DataBoundary>
 <DataBoundary state={pending}>{receipts=>receipts.filter(item=>!item.runId).map(item=><Card key={item.requestId} tone='warning'><Text className='wk-h3'>有一项提交仍待核对</Text><Text className='wk-muted'>{item.requestId}</Text><Action secondary loading={action.busy} onClick={()=>void action.run(async()=>{const after=await requireTaskOffice().reconcilePending();const admitted=after.find(r=>r.requestId===item.requestId&&r.runId);if(admitted){const resolved=await resolveTaskForRun(admitted.runId!);await openTask(resolved.taskId,admitted.runId!)}query.reload()})}>查询原请求</Action></Card>)}</DataBoundary>
 <Card><Field label='通过运行 ID 恢复' value={lookupId} onChange={setLookupId}/><Action secondary disabled={!lookupId.trim()} loading={action.busy} onClick={()=>void action.run(async()=>{const resolved=await resolveTaskForRun(lookupId.trim());await openTask(resolved.taskId,resolved.runId)})}>读取任务状态</Action><Text className='wk-muted wk-small'>服务端仍会验证任务归属，不会因知道 ID 而获得访问权限。</Text></Card>
 {receipt&&<Notice>{receipt}</Notice>}{action.error&&<Notice tone='danger'>{action.error}</Notice>}{more.error&&<Notice tone='danger'>{more.error}</Notice>}</Screen>;
}

export function ExecutionPage(){
 const runId=routeParam('id'),taskIdParam=routeParam('task');const [view,setView]=useState<TaskDetailView>();const [error,setError]=useState<string>();const [instruction,setInstruction]=useState('');const action=useAction();const handle=useRef<ReturnType<ReturnType<typeof requireTaskOffice>['open']>>();
 const stop=useCallback(()=>{handle.current?.close('page-hidden');handle.current=undefined},[]);
 const reload=useCallback(async()=>{stop();setError(undefined);
  try{
   const office=requireTaskOffice();
   const taskId=taskIdParam||(await resolveTaskForRun(runId)).taskId;
   const opened=office.open({taskId,runId});handle.current=opened;
   const off=opened.updates(next=>setView(next));
   const initial=await opened.hydrate();setView(initial);
   return()=>off();
  }catch(e){setError(errorMessage(e));return undefined}
 },[runId,taskIdParam]);
 useEffect(()=>{const off=reload().then(cleanup=>cleanup).catch(()=>undefined);return()=>{void off.then(cleanup=>cleanup&&cleanup());stop()}},[reload]);useDidShow(()=>{void reload()});useDidHide(stop);
 return <Screen title='执行详情'>{view?<><Card tone='mint'><View className='wk-between'><Badge tone={runStatusBadgeTone(view.runStatus)}>{runStatusLabels[view.runStatus]??view.runStatus}</Badge><Text className='wk-muted wk-small'>{connectionLabel(view.connection)}</Text></View><Text className='wk-display'>每一步，都有迹可循。</Text><Text className='wk-mono'>{runId}</Text><View className='wk-meta'><Text>任务：{view.title||view.taskId}</Text><Text>生命周期：{view.lifecycle}</Text><Text>执行：{view.executionStatus}</Text><Text>结算：{view.settlementStatus}</Text><Text>已确认事件：#{view.cursor}</Text></View></Card>
 {view.attention==='required'&&<Card tone='warning'><Text className='wk-h2'>任务需要你的确认</Text><Text className='wk-muted'>核对动作与影响后，再决定如何继续。</Text><Action onClick={()=>void navigate('approval',{run:runId})}>查看待处理事项</Action></Card>}
 {view.interruption&&<Notice tone='warning'>{interruptionNotice(view.interruption.reason)}</Notice>}
 <Section title='执行时间线'/><View className='wk-timeline'>{view.timeline.map(entry=><View className='wk-step' key={entry.seq}><Text className='wk-h3'>{timelineKindLabel(entry.kind)}</Text><Text className='wk-muted wk-small'>{formatTime(entry.occurredAt)} · #{entry.seq}</Text></View>)}</View>
 <Card><Field label='追加指令' value={instruction} onChange={setInstruction} multiline placeholder='补充要求，不会重建任务'/><Action loading={action.busy} disabled={!instruction.trim()||view.runStatus==='succeeded'||view.runStatus==='failed'||view.runStatus==='canceled'} onClick={()=>void action.run(async()=>{await executions.command(runId,{action:'steer',text:instruction.trim(),expected_revision:view.revision});setInstruction('');const cleanup=await reload();cleanup&&cleanup()})}>提交追加指令</Action><Text className='wk-muted wk-small'>追加/取消命令暂经授权 API 直发；统一的 Task 意图通道属后续 Issue。</Text></Card>
 <View className='wk-grid'><Action secondary onClick={()=>void navigate('artifact',{session:view.taskId,run:runId})}>查看材料</Action><Action danger disabled={view.runStatus==='succeeded'||view.runStatus==='failed'||view.runStatus==='canceled'} loading={action.busy} onClick={()=>void action.run(async()=>{if(!await confirmAction('取消这个任务？','这会向服务端发送取消命令，已发生的消耗不会因此自动退还。'))return;await executions.command(runId,{action:'cancel',expected_revision:view.revision});const cleanup=await reload();cleanup&&cleanup()})}>取消任务</Action></View></>:<Empty title='正在读取服务端快照' body='不会为了恢复页面重新启动任务。'/>{error&&<Notice tone='warning'>{error}</Notice>}}{action.error&&<Notice tone='danger'>{action.error}</Notice>}<Action secondary onClick={()=>void reload()}>重新同步快照</Action></Screen>;
}

export function ApprovalPage(){
 const runId=routeParam('run');const query=useData(`inbox:${runId}`,async()=>{const view=await requireTaskOffice().inbox();return inboxVisibleItems(view.items,runId||undefined)});const [receipt,setReceipt]=useState<string>();const action=useAction();
 const decide=async(item:{interactionId:string;runId:string;kind:'tool_approval'|'budget'|'recovery';argsHash:string;expectedRevision:number;createdAt:string},receiptFor:(r:AttentionDecisionReceipt)=>string)=>{const result=await requireTaskOffice().decide({item,action:'reject'});setReceipt(receiptFor(result));query.reload()};
 return <Screen title='审批确认'><Text className='wk-display'>先看清影响，{ '\n' }再作出决定。</Text><Notice tone='warning'>当前接口尚未提供动作名称、目标资源和风险摘要，不能只凭参数哈希批准外部写操作。请在 Web 核验完整动作；此版本仅支持安全拒绝。</Notice>
 <DataBoundary state={query}>{items=><>{items.map(item=><Card key={item.interactionId} tone='warning'><View className='wk-between'><Text className='wk-h3'>{item.kind==='tool_approval'?'工具操作确认':item.kind==='budget'?'预算扩展':'执行恢复'}</Text><Badge tone='warning'>待处理</Badge></View><Text className='wk-mono'>事项：{item.interactionId}</Text><Text className='wk-muted wk-small'>运行：{item.runId} · 版本：{item.expectedRevision} · 决策与参数哈希绑定</Text>{item.kind==='tool_approval'?<Action secondary loading={action.busy} onClick={()=>void action.run(()=>decide(item,decisionReceiptText))}>拒绝本次操作</Action>:<Text className='wk-muted wk-small'>此类事项需要扩展/恢复参数上下文，请到 Web 工作台处理。</Text>}</Card>)}{!items.length&&<Empty title='当前没有待处理事项'/>}</>}</DataBoundary>
 {receipt&&<Notice>{receipt}</Notice>}{action.error&&<Notice tone='danger'>{action.error}</Notice>}<Action secondary onClick={()=>void navigate('tasks')}>返回任务</Action></Screen>;
}

export function ArtifactPage(){
 const run=routeParam('run');const [content,setContent]=useState<string>();const action=useAction();const [receipt,setReceipt]=useState<string>();
 const query=useData(`materials:${run}`,async()=>{const material=openActiveMaterial();if(!material)throw new Error('材料面板尚未就绪');return material.index({runId:run})},!!run);
 const terminal=useData(`terminal:${run}`,async()=>{const material=openActiveMaterial();if(!material)throw new Error('材料面板尚未就绪');return material.open({kind:'terminal',runId:run})},!!run);
 return <Screen title='任务材料'><Text className='wk-display'>把完成的工作，{ '\n' }留在手边。</Text>
 <DataBoundary state={query}>{index=><>{index.materials.map(entry=>{const row=materialEntryRow(entry);return <Card key={entry.materialId}><ListRow title={row.title} subtitle={row.subtitle} onClick={()=>void action.run(async()=>{const material=openActiveMaterial();const view=await material!.open({kind:entry.kind,runId:run,materialId:entry.materialId});if(view.kind==='artifact'||view.kind==='test-report'||view.kind==='diff')setContent(view.text??JSON.stringify({preview:view.preview}))})}/><Action secondary loading={action.busy} onClick={()=>void action.run(async()=>{const material=openActiveMaterial();const grant=await material!.act({kind:'download',runId:run,materialId:entry.materialId});if(grant.kind==='grant')setReceipt(`签名链接（短时效）：${grant.url}`)})}>复制下载链接</Action></Card>})}{!index.materials.length&&<Empty title='暂时没有材料' body='任务可能仍在进行，或者没有生成可下载产物。'/>}</>}</DataBoundary>
 <DataBoundary state={terminal}>{view=>view.kind==='terminal'?<Card><Text className='wk-h3'>终端输出（只读）</Text>{view.lines.slice(0,80).map(line=><Text key={line.seq} className='wk-mono wk-small'>{line.stream==='stderr'?'[stderr] ':''}{line.text}</Text>)}{view.nextCursor!==undefined&&<Text className='wk-muted wk-small'>已截断显示前 80 行；完整输出请到 Web 工作台。</Text>}</Card>:null}</DataBoundary>
 {content&&<Card><Text className='wk-h3'>预览</Text><Text className='wk-prose' selectable>{content}</Text></Card>}
 {receipt&&<Notice>链接为短时效签名授权，请复制到浏览器打开；小程序不直接执行下载内容。HTML、脚本不在小程序中执行。</Notice>}
 {action.error&&<Notice tone='danger'>{action.error}</Notice>}
 {run&&<Action secondary onClick={()=>void navigate('execution',{id:run})}>返回任务</Action>}</Screen>;
}
```

说明（写进实现时的注释亦可）：`ApprovalPage.decide` 的参数类型来自 mobile-core `InboxItem` 的结构；页面把「仅 tool_approval 可拒绝」保持为现状 UX 立场（wire 未提供动作详情，诚实拒绝），`decide()` 的四态回执经 `decisionReceiptText` 呈现。

- [ ] **Step 3: 重写 features/home/pages.tsx 的 AgentsPage 与 AgentPage（HomePage 保留原文）**

`apps/miniprogram/src/features/home/pages.tsx` 中：

3a. 顶部 import 区改为：

```tsx
import { useState } from 'react';
import { View, Text, Textarea, Button } from '@tarojs/components';
import { Screen, Card, Action, Field, Notice, Section, ListRow, Badge, Empty, useData, DataBoundary, useAction, useSession } from '../../components/ui.tsx';
import { client, auth } from '../../services/runtime.ts';
import { requireTaskOffice, resolveTaskForRun } from '../../services/mobile-office.ts';
import { navigate, routeParam } from '../../platform/navigation.ts';
import { rows, identifier, text, record } from '../../services/views.ts';
```

（删除 `startTask, pendingIntent, rememberRun` 的 import；`executions` 若 HomePage 未用则从 import 中删除——HomePage 当前只用 client/auth。）

3b. `AgentsPage` 整体替换为（shelf browse：Resource Interface 的页面场景）：

```tsx
export function AgentsPage(){
 const [search,setSearch]=useState('');
 const query=useData('shelf-agents',async()=>{const shelf=activeResourceShelf();if(!shelf)throw new Error('资源面板尚未就绪');return shelf.browse()});
 return <Screen title='发现 Agent'><Text className='wk-display'>为工作，找个好帮手。</Text><Field label='搜索 Agent' value={search} onChange={setSearch} placeholder='名称或用途'/><DataBoundary state={query}>{page=>{const verdict=page.classVerdicts.agent;const items=page.agents.filter(a=>`${a.name} ${a.summary}`.toLowerCase().includes(search.toLowerCase()));return <>{verdict.state==='forbidden'&&<Notice tone='warning'>{verdict.reason||'Agent 列表当前不可访问（权限已被撤销）。'}</Notice>}<View className='wk-grid'>{items.map(a=><Card key={a.id} tone={a.capability.state==='supported'?'mint':'white'}><ListRow title={a.name} subtitle={a.summary||'已配置的空间助手'} icon='spark' onClick={()=>void navigate('agent',{id:a.id})}/><Badge tone={a.capability.state==='supported'?'success':'warning'}>{a.capability.state==='supported'?'可访问':'当前不可用'}</Badge></Card>)}</View>{!items.length&&<Empty title='没有匹配的 Agent' body='换个关键词再试一次。'/>}<Card><Text className='wk-muted wk-small'>当前空间：{page.knowledge.length} 个知识库 · {page.connections.length} 个连接（{page.classVerdicts.knowledge.state}/{page.classVerdicts.connection.state}）</Text></Card></>}}</DataBoundary></Screen>;
}
```

（import 区需补 `activeResourceShelf`：`import { requireTaskOffice, resolveTaskForRun, activeResourceShelf } from '../../services/mobile-office.ts';`）

3c. `AgentPage` 的 `submit` 替换为 office.start（页面其余骨架保留）：

```tsx
 const submit=()=>action.run(async()=>{
  if(!/^\d+$/.test(budget)||!Number.isSafeInteger(Number(budget)))throw new Error('预算须为可表示的非负整数');
  const receipt=await requireTaskOffice().start({text:prompt.trim(),agentId:id,budgetUpper:Number(budget)});
  if(receipt.runId){const resolved=await resolveTaskForRun(receipt.runId);await navigate('execution',{id:receipt.runId,task:resolved.taskId});return}
  throw new Error(receipt.phase==='rejected'?'该目标被拒绝：请调整描述或预算后重试':'已提交，等待服务端确认；请到任务中心查询原请求');
 });
```

并在页面 pending 分支（原有 `previous` 逻辑）替换为：

```tsx
 const pendingList=await requireTaskOffice().reconcilePending();
 const unbound=pendingList.find(item=>!item.runId);
 if(unbound){const after=await requireTaskOffice().reconcilePending();const admitted=after.find(item=>item.requestId===unbound.requestId&&item.runId);if(admitted){const resolved=await resolveTaskForRun(admitted.runId!);await navigate('execution',{id:admitted.runId!,task:resolved.taskId});return}throw new Error('原请求尚未确定，请在任务中心继续查询')}
```

（即：原 `startTask` 内嵌的重入/对账逻辑整体交给 TaskOffice——这正是本 Issue 的复用主张。）

- [ ] **Step 4: 运行与类型门**

Run: `cd apps/miniprogram && node --experimental-transform-types --test tests/*.test.mjs`
Expected: PASS（页面不直接进 node 测试；逻辑都在 Task 4 已测的组合根与 office-views 里）。

Run: `cd apps/miniprogram && npx tsc --noEmit 2>&1 | grep 'error TS' | grep -v 'features/account/pages.tsx' || true`
Expected: 无输出（新增页面代码零类型错误；account 页 13 个先在错误仍在但不增长）。

- [ ] **Step 5: 源级断言：页面不再引用旧 controller（防回归）**

在 `apps/miniprogram/tests/office-assembly.test.mjs` 末尾追加：

```js
import { readFileSync } from 'node:fs';
test('replace-dont-layer: execution/home pages reference the deep modules, never the deleted workbench controller', () => {
  const executionPages = readFileSync(new URL('../src/features/execution/pages.tsx', import.meta.url), 'utf8');
  const homePages = readFileSync(new URL('../src/features/home/pages.tsx', import.meta.url), 'utf8');
  for (const source of [executionPages, homePages]) {
    assert.equal(source.includes('services/workbench'), false);
    assert.equal(source.includes('core/execution'), false);
  }
  assert.ok(executionPages.includes('requireTaskOffice'));
  assert.ok(executionPages.includes('openActiveMaterial'));
  assert.ok(homePages.includes('requireTaskOffice') || homePages.includes('activeResourceShelf'));
});
```

Run: `cd apps/miniprogram && node --experimental-transform-types --test tests/office-assembly.test.mjs`
Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add apps/miniprogram/src/features/execution/pages.tsx apps/miniprogram/src/features/home/pages.tsx apps/miniprogram/src/core/intent.ts apps/miniprogram/tests/office-assembly.test.mjs
git commit -m "feat(miniprogram): tasks/execution/approval/artifact/agent screens run on Task Office & Task Material; delete hand-rolled controllers"
```

---

### Task 6: 删除旧原生编排树 + 维护模式门槛 + 文档重写

**Files:**
- Delete: `miniprogram/`（30 个 git 跟踪文件，`git rm -r miniprogram`）
- Create: `apps/miniprogram/tests/orchestrator.test.mjs`
- Modify: `website-docs/05-clients/04-miniprogram.md`（全文重写）

**Interfaces:**
- Consumes: 无代码接口；消费 git 状态与 workspace 配置文件。
- Produces: 「进入维护模式」的可验证门槛（Issue #68 What-to-build 后半句）：门槛 = 旧树不存在 + workspace 恰一个 miniprogram 编排条目 + 新编排器依赖深模块。门槛绿 ⇔ 旧原生实现正式被替换并进入维护（删除）状态，且任何复活尝试在 CI 失败。

- [ ] **Step 1: 写门槛测试（先 RED——旧树还在）**

创建 `apps/miniprogram/tests/orchestrator.test.mjs`：

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

// apps/miniprogram/tests → 上三级 = 仓库根。
const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

test('maintenance-mode gate: the legacy native miniprogram orchestrator tree is gone (replace-dont-layer)', () => {
  assert.equal(existsSync(path.join(repoRoot, 'miniprogram', 'app.json')), false, '旧原生编排树 miniprogram/ 必须整体删除，不得与新编排器长期并存');
  assert.equal(existsSync(path.join(repoRoot, 'miniprogram')), false, '目录本身也应消失（不留空壳）');
});

test('maintenance-mode gate: exactly one miniprogram orchestrator in the pnpm workspace', () => {
  const yaml = readFileSync(path.join(repoRoot, 'pnpm-workspace.yaml'), 'utf8');
  const entries = [...yaml.matchAll(/^\s*-\s+(\S+)\s*$/gm)].map(match => match[1]);
  const miniprogramEntries = entries.filter(entry => entry.includes('miniprogram'));
  assert.deepEqual(miniprogramEntries, ['apps/miniprogram']);
});

test('maintenance-mode gate: the surviving orchestrator consumes the deep modules', () => {
  const pkg = JSON.parse(readFileSync(new URL('../package.json', import.meta.url), 'utf8'));
  assert.ok(pkg.dependencies['@weknora/mobile-core'], 'apps/miniprogram 必须依赖 @weknora/mobile-core（深模块复用门槛）');
  assert.equal(pkg.dependencies['@weknora/mobile-core'], 'workspace:*');
});
```

Run: `cd apps/miniprogram && node --experimental-transform-types --test tests/orchestrator.test.mjs`
Expected: FAIL——第 1 个用例红（`miniprogram/app.json` 仍存在）。后两个用例应已绿（Task 2 已接线依赖）。

- [ ] **Step 2: 执行 replace-dont-layer（删除旧树）**

```bash
git rm -r miniprogram
```

（30 个跟踪文件：app.js/app.json/app.wxss/sitemap.json/project.config.json/project.private.config.json.example/package.json/README.md + pages/×3 四件套 + utils/×4 + assets/。旧树的测试目录 `tests/miniprogram/` 已不存在——差异记录 D3——无测试损失。CI `.github/workflows/frontend.yml` 只引用 workspace 包，无需改动；`Makefile`/`docker*`/`scripts` 无引用，已核实。）

Run: `cd apps/miniprogram && node --experimental-transform-types --test tests/orchestrator.test.mjs`
Expected: PASS（3 个用例绿——RED→GREEN 完成）。

- [ ] **Step 3: 重写 website-docs/05-clients/04-miniprogram.md**

全文替换为：

```markdown
# 微信小程序客户端（Taro）

微信小程序客户端源码位于 `apps/miniprogram`，基于 Taro 4.2.1 + React 18，覆盖 home/chat/auth/knowledge/execution/account 六个域。它与 Expo 原生 App（`apps/mobile`）共享同一套业务深 Module：

- `@weknora/mobile-core`：Mobile Runtime（会话编排）、Task Office（任务列表/详情/发起/审批收件箱）、Resource Shelf（Agent/知识库/连接）、Task Material（产物/预览/终端）；
- `@weknora/api-client`：`mobile/*` remote Adapter（复用既有 ClientRequest 通道，不新建 HTTP client）；
- `@weknora/contracts` / `@weknora/domain`：wire 契约与纯领域策略。

小程序只提供平台 Adapter：`src/platform/transport.ts`（wx.request/uploadFile 的 HTTP/SSE 传输）、`src/platform/credential-store.ts`（本地存储凭据仓——只有 MobileRuntime 一个写者）、`src/platform/authorized-channels.ts`（授权 REST/SSE 通道与免凭据 blob 抓取）、`src/platform/intent-log.ts`（耐久任务意图）。组合根在 `src/services/runtime.ts`（MobileRuntime 装配）与 `src/services/mobile-office.ts`（深模块记忆化工厂）。

## 迁移记录（replace-dont-layer）

仓库根原有的原生微信小程序 `miniprogram/`（无框架、API Key 直连、仅知识库问答）已于本迁移中**整体删除**：Taro 编排器经同一 Task/Resource/Material Interface 覆盖其能力后，旧树按「replace-don't-layer」退出（见 `docs/specs/2026-09-20-mobile-module-seams.md` §14）。删除的门槛由 `apps/miniprogram/tests/orchestrator.test.mjs` 长期守卫：旧树不得复活、workspace 恰一个 miniprogram 条目、存续编排器必须依赖 `@weknora/mobile-core`。旧的 `tests/miniprogram/*.test.js` 白盒测试随旧树一并退出，其行为由 mobile-core Interface 级测试与 `apps/miniprogram/tests/office-assembly.test.mjs` 场景测试承接。

## 认证与连接

- 后端地址来自构建期注入的 `__API_ORIGIN__`（见 `config/index.ts` 与 `.env.example`）；host 大小写归一化后作为唯一会话身份键。
- 登录走 MobileRuntime（邮箱+密码，Bearer + refresh 单飞轮换）；凭据只存于本机 storage 的 `wk:auth:<origin>` 键，**只有 Runtime 一个写者**，UI 与快照视图永不包含 token。
- 切换工作空间 = `MobileRuntime.activateTenant`（服务端重新签发并复核身份）；登出撤销本地 scope 与私有缓存，远端吊销尽力而为。

## 本地开发与测试

```bash
pnpm install
pnpm --filter @weknora/miniprogram run dev:weapp    # 微信开发者工具导入 dist/
pnpm --filter @weknora/miniprogram run test         # node --experimental-transform-types --test tests/*.test.mjs
pnpm --filter @weknora/miniprogram run typecheck
pnpm --filter @weknora/miniprogram run tokens:check
```

测试在 Node 内以契约级 Taro 替身（`tests/helpers/taro-stub.mjs`）装配真实源码：`tests/assembly.test.mjs` 验证 Runtime 编排（登录恢复/401 单飞刷新/迟到丢弃/SSE 装配），`tests/office-assembly.test.mjs` 在最高稳定 Interface 上跑关键 scenario（home 聚合/列表分页/耐久发起/详情快照+SSE/审批四态/资源三态/材料预览）。真实后端集成证据为 opt-in：设置 `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD`（可选 `WEKNORA_MOBILE_TEST_START_TASK=1`）后运行 `tests/integration/miniprogram-office-integration.test.mjs`。

## 边界与诚实声明

- steer/cancel 命令暂经授权 API 直发（统一 Task 意图通道属后续 Issue）；审批页在 wire 提供动作详情前仅支持安全拒绝。
- 小程序不执行 HTML/脚本/终端输入；材料下载以短时效签名链接提供。
- 支付通道未接入；订单与权益页如实展示服务端状态。
```

- [ ] **Step 4: 全量回归**

Run: `cd apps/miniprogram && node --experimental-transform-types --test tests/*.test.mjs`
Expected: PASS（含 orchestrator 3 用例）。

Run: `ls miniprogram 2>&1; git status --short | head -40`
Expected: `miniprogram` 不存在；git status 显示旧树 30 个删除 + 新增/修改文件。

- [ ] **Step 5: Commit**

```bash
git add miniprogram apps/miniprogram/tests/orchestrator.test.mjs website-docs/05-clients/04-miniprogram.md
git commit -m "chore(miniprogram): remove the legacy native orchestrator tree (replace-dont-layer) and add the maintenance-mode gate"
```

---

### Task 7: opt-in 真实集成证据（blocked-env 声明 + 本地替代证据）

**Files:**
- Create: `apps/miniprogram/tests/helpers/node-taro.mjs`
- Create: `apps/miniprogram/tests/integration/miniprogram-office-integration.test.mjs`
- Modify: `apps/miniprogram/package.json`（test 脚本纳入 `tests/integration/*.test.mjs`）

**Interfaces:**
- Consumes: Task 3/4 的真实组合根（`services/runtime.ts`、`services/mobile-office.ts`）；Task 2 的通道契约。
- Produces: 真实集成证据入口——`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD`（三者同进同出）+ 可选 `WEKNORA_MOBILE_TEST_START_TASK=1`（真实发起探针，含清理）。缺环境时用例 `t.skip`（**不得伪造通过**——AC3 的字面要求）。

**blocked-env 声明（验收标准 3 的本地可验证性边界）**：真实端到端（真部署 + 真凭据 + 真实授权通道 + 深模块编排 + 真服务端 overview/executions/interactions/resources/artifacts）需要一个真实 WeKnora Deployment（HTTPS 公网 origin）与一个测试账号，本 worktree 无此环境；本地证据层级为：office-assembly（真实源码 + 契约级平台替身，最高稳定 Interface 级）+ 本测试的 opt-in 真实模式（具备环境时自动产出端到端证据）。微信开发者工具内的真机运行属真机验收（spec Testing Decisions：real-device acceptance separately），本地不伪造。

- [ ] **Step 1: 写 fetch 底座的 Taro 替身**

创建 `apps/miniprogram/tests/helpers/node-taro.mjs`：

```js
// 真实网络版 Taro 契约替身：与 taro-stub.mjs 相同的模块契约（request/uploadFile/storage），
// 但 request 底座是 Node fetch——组合根以上（Runtime/深模块/remote Adapter/传输层）100% 真实源码，
// 只有最外层平台调用被替换为等价的 fetch 语义。SSE（enableChunked）不支持：真实集成证据不覆盖流式面。
const storage = new Map();
async function request(options) {
  const controller = new AbortController();
  const stop = () => controller.abort();
  options.signal?.addEventListener?.('abort', stop, { once: true });
  try {
    const hasBody = options.data !== undefined && options.method !== 'GET';
    const response = await fetch(options.url, {
      method: options.method,
      headers: options.header,
      ...(hasBody ? { body: typeof options.data === 'string' ? options.data : JSON.stringify(options.data) } : {}),
      ...(options.signal ? { signal: controller.signal } : {}),
    });
    const text = await response.text();
    options.success({ statusCode: response.status, header: Object.fromEntries(response.headers), data: text });
  } catch (error) {
    options.fail({ errMsg: `node-taro: ${error?.message ?? 'network error'}` });
  } finally {
    options.signal?.removeEventListener?.('abort', stop);
  }
  return { abort() { controller.abort(); }, onHeadersReceived() {}, offHeadersReceived() {}, onChunkReceived() {}, offChunkReceived() {} };
}
export const Taro = {
  request,
  uploadFile(options) { options.fail({ errMsg: 'node-taro: uploadFile not supported in integration mode' }); return { abort() {} }; },
  getStorageSync(key) { return storage.has(key) ? storage.get(key) : ''; },
  setStorageSync(key, value) { storage.set(key, value); },
  removeStorageSync(key) { storage.delete(key); },
  getStorageInfoSync() { return { keys: [...storage.keys()] }; },
};
export const nodeStorage = storage;
```

- [ ] **Step 2: 写集成测试（缺环境 skip；有环境时真跑）**

创建 `apps/miniprogram/tests/integration/miniprogram-office-integration.test.mjs`：

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import { registerHooks } from 'node:module';
import { pathToFileURL } from 'node:url';

const DEPLOYMENT_URL = process.env.WEKNORA_MOBILE_TEST_DEPLOYMENT_URL;
const EMAIL = process.env.WEKNORA_MOBILE_TEST_EMAIL;
const PASSWORD = process.env.WEKNORA_MOBILE_TEST_PASSWORD;
const START_PROBE = process.env.WEKNORA_MOBILE_TEST_START_TASK === '1';
/** 三变量同进同出：任一缺失即视为未配置（与前置批次 opt-in 语义一致）。 */
const enabled = Boolean(DEPLOYMENT_URL) && Boolean(EMAIL) && Boolean(PASSWORD);

/** Mimosa/仓库安全约束：真实请求前校验——仅 https、拒绝 localhost/环回/私网/链路本地/保留地址。 */
function assertAllowedDeploymentOrigin(origin) {
  let parsed;
  try { parsed = new URL(origin); } catch { throw new Error(`invalid deployment origin: ${origin}`); }
  if (parsed.protocol !== 'https:') throw new Error('integration deployment must be HTTPS');
  if (parsed.pathname !== '/' || parsed.search || parsed.hash) throw new Error('deployment origin must carry no path/query/fragment');
  const host = parsed.hostname.replace(/^\[|\]$/g, '');
  const v4 = host.match(/^(\d+)\.(\d+)\.(\d+)\.(\d+)$/);
  if (host === 'localhost' || host.endsWith('.localhost') || host.endsWith('.local') || host === '0.0.0.0') throw new Error('loopback/local host is not allowed');
  if (v4) {
    const [a, b] = [Number(v4[1]), Number(v4[2])];
    if (a === 127 || a === 10 || a === 0 || a >= 224) throw new Error('loopback/private/reserved IPv4 is not allowed');
    if (a === 172 && b >= 16 && b <= 31) throw new Error('private IPv4 is not allowed');
    if (a === 192 && b === 168) throw new Error('private IPv4 is not allowed');
    if (a === 169 && b === 254) throw new Error('link-local IPv4 is not allowed');
    if (a === 100 && b >= 64 && b <= 127) throw new Error('CGNAT IPv4 is not allowed');
  }
  const lower = host.toLowerCase();
  if (lower === '::1' || lower.startsWith('fc') || lower.startsWith('fd') || lower.startsWith('fe80')) throw new Error('IPv6 loopback/ULA/link-local is not allowed');
  if (/^::ffff:\d+\.\d+\.\d+\.\d+$/.test(lower)) throw new Error('IPv4-mapped IPv6 must be rejected by its IPv4 rules');
}

const nodeTaroURL = pathToFileURL(new URL('../helpers/node-taro.mjs', import.meta.url).pathname).href;
if (enabled) {
  assertAllowedDeploymentOrigin(DEPLOYMENT_URL);
  registerHooks({
    resolve(specifier, context, nextResolve) {
      if (specifier === '@tarojs/taro') return { url: nodeTaroURL, shortCircuit: true };
      return nextResolve(specifier, context);
    },
  });
}
globalThis.__API_ORIGIN__ = DEPLOYMENT_URL ?? 'https://api.example.test';

test('miniprogram office integration (opt-in, real deployment)', { skip: enabled ? false : 'set WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD (and optionally WEKNORA_MOBILE_TEST_START_TASK=1) to run the real evidence' }, async () => {
  const runtimeModule = await import('../../src/services/runtime.ts');
  const office = await import('../../src/services/mobile-office.ts');
  const evidence = [];
  const say = line => { evidence.push(line); console.log(`[evidence] ${line}`); };

  await runtimeModule.auth.login(EMAIL, PASSWORD);
  assert.equal(runtimeModule.auth.snapshot().phase, 'ready');
  say(`login: authorized as ${runtimeModule.auth.snapshot().userId} in tenant ${runtimeModule.auth.snapshot().tenantId}`);
  assert.ok(runtimeModule.runtime.scopeLease(), 'authorization minted a lease');

  const home = await office.requireTaskOffice().home();
  assert.equal(typeof home.asOf, 'string');
  say(`home: needsMe=${home.needsMe.length} running=${home.running.length} recentlyCompleted=${home.recentlyCompleted.length} unread=${home.unreadNotifications}`);

  const tasks = await office.requireTaskOffice().tasks({});
  assert.ok(Array.isArray(tasks.items));
  say(`tasks: items=${tasks.items.length} nextCursor=${tasks.nextCursor ?? '-'}`);

  const shelf = office.activeResourceShelf();
  if (shelf) {
    const page = await shelf.browse();
    say(`shelf: agents=${page.agents.length} knowledge=${page.knowledge.length} connections=${page.connections.length} agentVerdict=${page.classVerdicts.agent.state}`);
  } else {
    say('shelf: unavailable (no authorized shelf)');
  }

  const inbox = await office.requireTaskOffice().inbox();
  say(`inbox: pending=${inbox.items.length}`);

  let probeRunId;
  if (START_PROBE) {
    const receipt = await office.requireTaskOffice().start({ text: `issue68 integration probe ${new Date().toISOString()}`, agentId: 'builtin-quick-answer', budgetUpper: 1 });
    say(`start: requestId=${receipt.requestId} phase=${receipt.phase} runId=${receipt.runId ?? '-'}`);
    assert.ok(receipt.dispatched || receipt.phase === 'pending');
    probeRunId = receipt.runId;
  }

  const materialTarget = probeRunId ?? tasks.items[0]?.runId;
  if (materialTarget) {
    const material = office.openActiveMaterial();
    if (material) {
      const index = await material.index({ runId: materialTarget });
      say(`material: run=${materialTarget} entries=${index.materials.length} terminal=${index.terminal.available}`);
      material.close('evidence-done');
    }
  } else {
    say('material: skipped (no run available and probe disabled)');
  }

  if (probeRunId) {
    const resolved = await office.resolveTaskForRun(probeRunId);
    await office.requireTaskOffice().archive(resolved.taskId);
    say(`cleanup: archived probe task ${resolved.taskId}`);
  }
  await runtimeModule.logout();
  say('logout: credentials cleared');
  assert.ok(evidence.length >= 5, 'evidence log must be substantive, not fabricated');
});
```

`apps/miniprogram/package.json` 的 test 脚本更新为：

```json
    "test": "node --experimental-transform-types --test tests/*.test.mjs tests/integration/*.test.mjs",
```

- [ ] **Step 3: 运行（skip 路径即本地默认）**

Run: `cd apps/miniprogram && node --experimental-transform-types --test tests/integration/miniprogram-office-integration.test.mjs`
Expected: 1 skipped（skip reason 打印环境变量名）。**不得**在无凭据环境下出现 pass——伪造即违背 AC3。

Run: `cd apps/miniprogram && node --experimental-transform-types --test tests/*.test.mjs tests/integration/*.test.mjs`
Expected: 全部 pass + 1 skipped。

- [ ] **Step 4: Commit**

```bash
git add apps/miniprogram/tests/helpers/node-taro.mjs apps/miniprogram/tests/integration/miniprogram-office-integration.test.mjs apps/miniprogram/package.json
git commit -m "test(miniprogram): opt-in real-deployment integration evidence for the office scenarios (honest skip without credentials)"
```

---

### Task 8: 终验（计划级验证 + 自我审查落档）

**Files:**
- 无新文件（验证任务；证据输出到终端/CI 日志）。

- [ ] **Step 1: 计划级验证命令（worktree 根执行）**

```bash
pnpm exec tsx --test packages/mobile-core/src/platform-purity.test.ts && cd apps/miniprogram && node --experimental-transform-types --test tests/*.test.mjs tests/integration/*.test.mjs && if npx tsc --noEmit 2>&1 | grep 'error TS' | grep -v 'features/account/pages.tsx'; then echo 'UNEXPECTED NEW TYPE ERRORS'; exit 1; fi
```

Expected: mobile-core 纯度门 2 pass；apps/miniprogram 全部用例 pass（含 orchestrator 3 + office-assembly 11 + assembly 10 + platform-adapters 7 + office-views 5 + integration 1 skipped…以实际文件为准，0 fail）；tsc 无 account 页之外的新错误。

- [ ] **Step 2: 验收标准逐条核对（执行者在交付说明中逐字回答）**

1. AC1「业务 Module 不依赖 Expo 或 WeChat。」→ 纯度门槛绿 + office-assembly 在 Taro 平台替身下跑通同一 Task Office scenario（module-seams §14 第 5 步「证明 Interface 不依赖 Expo」）。
2. AC2「新旧编排器不长期并存，迁移采用 replace-dont-layer。」→ `miniprogram/` 树删除 + orchestrator 门槛 3 断言绿 + app 内 AuthCoordinator/workbench controller 删除（Task 5 源级断言）。
3. AC3「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」→ office-assembly（Interface 级场景）+ 集成测试缺环境时显式 skip 并打印证据层级声明；有环境时产出 `[evidence]` 日志。
4. 交付说明必须如实记录：D4 的 13 个先在 typecheck 错误仍在（未增长）；集成证据在本环境为 skipped（blocked-env）；真机微信运行未验证（真机验收另列）。

- [ ] **Step 3: Commit（如有遗留零散修正）**

```bash
git status --short
git add -A apps/miniprogram packages/mobile-core/src/platform-purity.test.ts website-docs
git commit -m "chore(issue30-sweep): final verification pass for issue #68 (t38 taro deep-module reuse & maintenance gate)"
```

---

## 自我审查（writing-plans Self-Review，四项检查）

**1. Spec coverage（验收标准 → 任务映射）**

- AC1「业务 Module 不依赖 Expo 或 WeChat」→ Task 1（依赖+导入双扫描门槛，含 RED 探针证明可失败）；「Taro Adapter 跑同一 Task Office scenario 证明 Interface 不依赖 Expo」（module-seams §14.5）→ Task 4 office-assembly 全部用例（真实 mobile-core + api-client remote + Taro 传输）。
- AC2「新旧编排器不长期并存，迁移采用 replace-dont-layer」→ 三层：仓库级旧原生树删除+门槛（Task 6）；app 级 AuthCoordinator/workbench controller 删除+源级断言（Task 3/5）；「删除旧公开 controller」对应的白盒测试删除（auth.test.mjs，module-seams §13 授权）。
- AC3「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据」→ Task 4（Interface 级场景：home/列表分页去重/耐久发起同 requestId/详情快照+SSE/审批四态/资源三态+403/材料预览+终端/切租户 fail closed）+ Task 7（opt-in 真实证据 + blocked-env 显式 skip，绝不伪造）。静态门槛（Task 1/6）只作 AC1/AC2 的守卫，未冒充 AC3 证据——计划正文已明示分层。
- What-to-build 前半句「小程序通过同一 Task/Resource/Material Interface 跑关键 scenario」→ Task 4（组合根）+ Task 5（页面消费）；后半句「形成进入维护模式的可验证门槛」→ Task 6 门槛测试三断言 + 文档迁移记录。
- 无遗漏的 spec 段：module-seams §14 迁移顺序 1-4/6-8 已由前置批次交付；本计划只落第 5 步与第 8 步的「旧 shallow exports 删除」（工作台 controller + 旧树）。

**2. Placeholder scan**：全文检索 TBD/TODO/「稍后实现/适当处理」——代码块内零占位符，唯一命中是 JSX 的 `placeholder` 组件属性（合法 UI 用法）。所有测试与实现代码完整给出；页面代码中的 JSX 为完整可编译代码（依赖既有 components/ui.tsx 组件面，已在 Task 5 列明）。

**3. Type consistency（跨任务签名一致性）**

- `TaroSessionFacade` 的方法集在 Task 3 定义（bootstrap/login/switchTenant/logout/snapshot/subscribe/credential/scope/abortSubscriptions）与 services/runtime.ts 的 `auth` 导出、assembly 测试调用（auth.login/switchTenant/bootstrap/logout/scope.capture/scope.controller/scope.isCurrent/credential/snapshot）一致；`components/ui.tsx:9` 消费的 `auth.subscribe/auth.snapshot` 与 `Screen` 的 `auth.bootstrap()`（error 分支）一致。
- `activeTaskOffice()/requireTaskOffice()/openActiveMaterial()/activeResourceShelf()/resolveTaskForRun(runId)` 在 Task 4 定义，Task 5 页面与 office-assembly/集成测试调用同名同参。
- `createTaroCredentialStore(store, origin)`/`readStoredCredential(store, origin)`/`adoptLegacyCredentials(store, origin)`（Task 2 定义）与 Task 3 runtime.ts、session.ts 的调用一致；存储键统一 `wk:auth:<origin>`（assembly/集成测试断言同一键）。
- `streamWeapp(network, request, onText, onMetadata?)`（Task 2 transport 提取）与 authorized-channels.ts 的调用一致。
- `createAuthorizedStreamChannel(network, origin)` 返回 `AuthorizedStreamTransport`，与 `MobileRuntimePorts.authorizedStream?: (deploymentOrigin: string) => AuthorizedStreamTransport | undefined`（ports.ts:106）逐字对位。
- office-views 导出（runStatusLabels/runStatusBadgeTone/connectionLabel/interruptionNotice/inboxVisibleItems/decisionReceiptText/materialEntryRow）与 Task 5 页面 import、office-views.test.mjs 断言一致。
- wire 形状与前置批次逐字核对：overview 行 `{run_id, session_id, title?, run_status, attention?, updated_at}`、列表行 `{run_id, session_id, title?, status, …}`、interactions 行 `{id, run_id, kind, args_hash, expected_revision, created_at}`、artifacts `{version, artifacts:[{index,file_name,file_type,size}], terminal:{available}}`、capabilities `{protocol_minimum, protocol_maximum}`（CLIENT_PROTOCOL_VERSION=3）——均来自本会话对 `packages/api-client/src/mobile/*` 与 `packages/contracts` 的实读。

**4. Review Focus 落实**

1. 双编排器残留 → Task 6 门槛断言旧树/唯一条目 + Task 5 源级断言页面不引用 workbench + Task 3 assembly「exactly one credential key（single writer: the runtime）」。
2. 迟到 scope 回填 → Task 4「tenant switch revokes the old office lease — TASK_OFFICE_SCOPE_CHANGED」+ Task 3「a response landing after sign-out is discarded」。
3. 401 重放误伤 → Task 3「a definitive 401 on a POST is refreshed and replayed; an ambiguous network failure never replays」（NETWORK_ERROR 时 posts===1）。
4. 平台直连通道过期 token → Task 2「just-in-time readers see a rotation immediately」+ `currentBearerToken()` 预检（Task 3 实现，assembly 第 2 用例断言 `credential().accessToken==='t2'`）。
5. 第二编排树复活 → Task 6 门槛三断言（CI 长期运行 `pnpm --filter @weknora/miniprogram run test`）。

四项检查全部执行完毕，未发现未覆盖的验收标准。编写过程中已自查并修正三处与 MobileRuntime 实际语义不符的初稿：activateTenant 的身份权威是切换后的 `me()`（测试替身须有状态）、`stub.reset()` 会清空凭据仓（登录后只重装路由 handler）、身份富集是异步（断言用轮询而非 sleep）。独立计划审查后已修复六处：Task 2 两处 `emitHeaders` 误用第二/三参次序（实签名为 `emitHeaders(call, header, statusCode)`，`tests/helpers/taro-stub.mjs:73`；已改为 `emitHeaders(call, {}, 401)` 与 `emitHeaders(call, { 'Content-Type': … }, 200)`，并用真实 transport 实跑验证 401 头事件产出 `TransportFailure:HTTP_401:401`——`npx tsx /tmp/lt-check2.mjs`）；自审占位段描述陈旧（`result0HeaderOf` 已不存在，改写为如实描述）；`createTaraSessionFacadeSafe` 拼写（内联为直接调用 `createTaroSessionFacade`）；inbox 用例恒真断言（改为 decision_id 非空字符串断言）；四处行号漂移（transport.ts stream :83-132/注释 :79、unauthorizedStatus mobile-runtime.ts:86-90、ResourceShelfHandle shelf/types.ts:37-42、TaskHandle task-detail.ts:72-78 与 TaskDetailView :54-71）；Task 1 Step 2 措辞（补 devDependencies 说明）。剩余的实现期注意点（taro-stub 是否导出 `dispatch`，见 Task 2 Step 1 附注）已写明处置办法。
```

以上为计划全文。
