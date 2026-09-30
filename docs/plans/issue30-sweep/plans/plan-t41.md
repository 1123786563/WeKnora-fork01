# T11：注册设备、行动通知与安全深链（Issue #41）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 移动端在当前 Deployment 完成可撤销的设备注册（两步 intent→register、token 接管、撤销幂等语义），行动通知只作为同步 hint 触发权威重投影（通知点击 = 安全深链解析 → 重新鉴权 → 打开权威 Task 详情 → 本地已读，绝不执行通知描述的业务操作），错误 deep link 全部拒绝。

**Architecture:** 后端已完备（当前 HEAD 亲眼核实）：`/api/v1/mobile/devices*` 两步注册/撤销/列表端点（`internal/handler/mobile_device.go`）、`/api/v1/workbench/inbox*` 收件箱读模型与已读端点（`internal/handler/session/workbench_inbox.go`）、推送 payload 只含 `Intent.Kind` 的 hint 语义（`internal/modules/workbench/service/workbench/notification_delivery.go:111`）、投递侧重复通知去重（`internal/application/repository/mobile_notification.go` + `TestNotificationOutboxDeduplicates`）——**本计划 Go 侧零改动**，只把后端既有定向测试纳入回归命令。客户端新增三个深模块/纯函数（`packages/mobile-core/src/inbox/deep-link.ts` 安全深链解析、`packages/mobile-core/src/device/device-registry.ts` 设备注册深模块、`packages/mobile-core/src/inbox/notification-inbox.ts` 行动通知深模块）、两个真实 wire Adapter（`packages/api-client/src/mobile/devices.ts`、`inbox.ts`）与 apps/mobile 组装（`/inbox` 路由、InboxScreen、深链安全导航入口、fail-closed 原生 push token 适配器）。设备注册与收件箱与 Task Office 同模式接线：`MobileRuntime.authorizedRequest`（token 不出 Runtime，#34 交付）+ `scopeLease()` 围栏，**不修改 `mobile-runtime.ts` / `ports.ts` 任何既有行**（并行批次冲突最小化）。服务端 `workbench_notifications.deep_link` 列当前无生产写入方（迁移 000190 默认 `''`），因此客户端把 deep_link 一律按不可信输入解析（白名单 scheme/host/path + 参数校验），这正是验收标准「错误 deep link 的验证」的落点。

**Tech Stack:** TypeScript（`packages/mobile-core`、`packages/api-client`、`apps/mobile` Expo RN）、node:test + tsx（TS 测试运行器，与 `task-office.test.ts` 一致；命令在 worktree 根执行）、Go 1.26（仅回归验证既有测试，无代码改动）。前置：已执行过 `pnpm install`（本计划作者已实跑：`npx tsx --test packages/mobile-core/src/task-office/task-office.test.ts` 绿、`pnpm --filter @weknora/mobile test` 72 项全绿 fail 0、`pnpm --filter @weknora/mobile typecheck` 干净、`go test ./internal/application/repository/ -run 'TestMobileDevice...|TestNotificationOutboxDeduplicates'` ok、`go test ./internal/handler/session/ -run 'TestMX021'` ok）。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-41.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（User Stories 63–67，Implementation Decisions、Testing Decisions 中 push/device 相关条目）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§4 Mobile Runtime——「注册设备与 App 前后台生命周期」「push route 绑定」、§5 Task Office——「notification read state」、§10 App Shell 禁令、§12 依赖分类——「APNs / FCM | true external | Push Port + mock Adapter」、§13 Interface 测试面）
- ADR：`docs/adr/0007-registered-devices-and-encrypted-cache.md`（设备身份可撤销，不取代用户身份）、`docs/adr/0006-mobile-transport-by-semantics.md`（APNs/FCM 推送仅视为重新同步的提示）、`docs/adr/0012-mobile-business-logic-lives-behind-deep-modules.md`
- 领域术语：`CONTEXT.md`（「注册设备（Registered Device）」：同一物理设备在不同部署实例中注册彼此隔离，设备身份不取代用户身份、不自动获得空间权限；「行动通知（Actionable Notification）」：通知只提示客户端重新同步，不承载权威任务状态）
- Parent：Issue #30；Blocked by：#32/#34/#35（均已合并，接口在当前 HEAD 亲眼核实）
- 前序批次产出（本计划 Consumes，全部在当前 HEAD 亲眼核实）：#34 的 `MobileRuntime.authorizedRequest(input: RuntimeAuthorizedRequest): Promise<unknown>`（`packages/mobile-core/src/runtime/mobile-runtime.ts:383`，授权面 refresh-once）与 `MobileRuntimePorts.authorizedTransport`；#32 的 `MobileRuntime.scopeLease(): ScopeLease | undefined`（`packages/mobile-core/src/runtime/types.ts:58`）与包内 `leaseActive`（`packages/mobile-core/src/runtime/scope-lease.ts:26`）；#35 的 `/tasks/detail` 文件路由（`apps/mobile/src/app/tasks/detail.tsx`，`TaskOffice.open({taskId, runId})` → 权威 snapshot 水合，即「点击后同步权威 Task」的既有实现）；`activeMobileRuntime()` / `taskOfficeFor` 组合根模式（`apps/mobile/src/composition.ts:109`）；opt-in 集成模式 `WEKNORA_MOBILE_TEST_*` + `disallowedDeploymentHost(hostname, variable)`（`apps/mobile/src/runtime-integration-smoke.ts:118`）；`ApiError` 形态（`packages/api-client/src/errors.ts:24-33`：`name === 'ApiError'`、`status?: number`）。

## Global Constraints

以下为批准 Spec / ADR / Issue 的项目级约束，逐字引用，所有任务隐含遵守：

- 「Push is a synchronization hint.」（mobile-ai-office-design.md · Implementation Decisions）
- 「Registered devices are separate per Deployment. Device identity assists push and key wrapping but never replaces account or Tenant authorization.」（同上）
- 「Cache identity includes Deployment, user, Tenant, Task and Run where applicable. Scope changes invalidate subscriptions and reject late responses.」（同上）
- 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」（同上）
- 「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」（同上 · Testing Decisions）
- 「True external dependencies such as APNs, FCM, WebRTC, system audio and system share use mock or scripted Adapters at the Port and real-device acceptance separately.」（同上）
- 「Remote-owned dependencies use in-memory scenario Adapters for Module tests and real HTTP/SSE contract tests for production Adapters.」（同上）
- User Story 63：「As a member, I want cached task content encrypted and scoped to Deployment, user and Tenant, so that another account or space cannot read it.」
- User Story 64：「As an administrator, I want device revocation to remove or cryptographically invalidate cached content, so that lost devices can be contained.」
- User Story 65：「As a member, I want push notifications only for required action, failure or unknown outcome, completion and important budget events, so that alerts remain useful.」
- User Story 66：「As a privacy-conscious member, I want push bodies to omit sensitive business content, so that lock-screen notifications do not leak data.」
- 「移动 AI Office 保留 WeKnora 现有用户登录与 Tenant 授权作为身份权威，同时登记可独立撤销的移动设备，用于推送、设备策略和本地密钥封装。获准离线内容按用户与 Tenant 分区并加密……登出、切换空间、设备撤销或权限失效后删除对应缓存或使其密钥不可用。」（ADR-0007）
- 「移动端使用 REST 提交命令和取得权威 Snapshot……并把 APNs/FCM 推送仅视为重新同步的提示。」（ADR-0006）
- Mobile Runtime 唯一拥有：「注册设备与 App 前后台生命周期」（mobile-module-seams.md §4.1）；「OIDC return state、credential refresh single-flight、capability negotiation、设备注册、迟到响应拒绝、缓存 scope 切换、push route 绑定和启动恢复全部隐藏在此 Module。」（§4.3）
- 「Interface 不暴露 token、query key、generation number 或 SecureStore key。Scope Lease 是不透明、可撤销的能力对象，子 Module 每次异步提交前检查其有效性。」（mobile-module-seams.md §4.2）
- 「禁止：Screen 直接导入 packages/contracts 或 packages/api-client；Screen 自己维护 request_id、cursor、revision、scope generation；每个 Screen 建独立 query cache 或 token refresh；从 packages/ui 或 packages/views 引入 DOM/Radix 实现；把 Adapter DTO 直接作为长期 UI state。」（mobile-module-seams.md §10）
- 「APNs / FCM | true external | Push Port + mock Adapter」（mobile-module-seams.md §12 依赖分类表）
- 「注册设备（Registered Device）」：「同一物理设备在不同部署实例中具有彼此隔离的注册，设备身份不取代用户身份，也不自动获得空间权限。」、「行动通知（Actionable Notification）」：「通知只提示客户端重新同步，不承载权威任务状态。」（CONTEXT.md）
- 安全约束（会话注入）：服务端 SQL 一律参数绑定（本计划 Go 侧零改动；既有 inbox 查询已为 `?` 绑定，`workbench_inbox.go:88/115/128/139/152/158`）；凭据只从环境变量读取，源码与测试不写入可用凭据字面量（集成证据的设备 token 占位值 `integration-placeholder-token` 不是可用凭据——无 APNs/FCM 真实效力，且支持 `WEKNORA_MOBILE_TEST_DEVICE_TOKEN` 覆盖）；真实 HTTP 集成证据沿用 `WEKNORA_MOBILE_TEST_*` opt-in 环境变量（无回退凭据）。
- 服务端请求 URL 约束：api-client Adapter 构造即强校验 origin（绝对 HTTPS、无 userinfo/path/query/fragment，本包 `requireDeploymentOrigin` 既有模式）；deviceId 经 `encodeURIComponent` 编码进路径（防路径注入）。
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计。

**Issue #41 验收标准原文（docs/plans/issue30-sweep/issues/issue-41.md）：**

1. 「推送只作同步 hint，不直接改变业务状态。」
2. 「设备撤销、错误 deep link 和重复通知均有验证。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

验收标准映射：AC1 客户端半边由 Task 3（`applyHint` 只重投影、`markRead` 不执行业务）+ Task 6（导航前重新鉴权）交付，服务端半边（推送 payload 只含 `Intent.Kind`）已由 `notification_delivery.go:111` 实现并纳入回归命令；AC2 由 Task 1（错误 deep link）、Task 2（设备撤销语义）+ 既有 Go 测试回归（`TestMobileDeviceRevocationIsOwnerScoped` 等 14 项 + `TestNotificationOutboxDeduplicates`）+ Task 3（重复通知 notificationId 去重可观测）覆盖；AC3 由 Task 7 的 opt-in 真实 HTTP 集成证据（真实 JSON transport + 真实 wire Adapter + 真实 Runtime + 真实服务端两步注册/接管/撤销/inbox/markRead）承载。

验收标准 3 的本地可验证性说明（blocked-env 声明）：
- **真机 APNs/FCM 推送送达**（真实设备收到推送、点击冷启动落位）需要真机与推送凭据，本环境不存在——列为 blocked-env 验收项，本地替代证据：(a) Task 7 opt-in 真实 HTTP 集成（覆盖注册/接管/撤销/inbox/markRead 的完整服务端 wire 路径，等价于推送服务端的全部前置状态）；(b) Task 2/3 Interface 级场景测试（真实模块编排 + in-memory scenario Adapter）；(c) Go 侧既有推送 hint/去重/围栏测试回归命令。本地无 `WEKNORA_MOBILE_TEST_*` 环境时 Task 7 的真实 HTTP 用例以 `t.skip` 跳过（**不得伪造通过**）。
- **真机构建前置（expo-notifications）**：`apps/mobile/package.json` 当前不含 `expo-notifications`（实测 `grep expo-notifications apps/mobile/package.json` 零命中），而 `push-token.ts` 惰性 `require('expo-notifications')`——Node 测试链不受影响（fail closed → `'no-token'`），但 Metro 构建期无法解析的 require 会让 `expo export`/EAS 真机构建报 `Unable to resolve module`。真机目标必须先执行 `cd apps/mobile && npx expo install expo-notifications`（Task 6 内置该步骤；执行环境无法联网时如实记录为环境前置，其余验证不受影响，不伪造）。
- `InboxService.RevokeDevicesForOwner`（登出撤销全部设备注册，`workbench_inbox.go:157`）**无 HTTP 路由挂载**（`routes_workbench.go` 未暴露）——客户端不依赖该入口；设备撤销统一走 `DELETE /api/v1/mobile/devices/:id`（已挂载，`routes_workbench.go:199`）。此为与调查结论一致的服务端现状，记录在案。

**与调查结论的差异记录（以代码现状为准）：**

1. 调查称「推送 hint 语义 `internal/handler/session/workbench_inbox.go:123-129`（MarkRead 仅置 read=true 不执行业务操作）」——属实，且补充亲眼核实：MarkRead 是单条 `Update("read", true)`（`workbench_inbox.go:127-129`），对不存在的 notification_id 更新 0 行仍返回 success（幂等），Task 3/7 利用该语义。
2. 调查未提及：`workbench_notifications.deep_link` 列（迁移 `migrations/versioned/000190_workbench_notifications.up.sql`，`VARCHAR(512) NOT NULL DEFAULT ''`）**当前无生产写入方**（`rg deep_link internal/` 仅 inbox 读投影命中）——服务端不生成 deep_link 值，客户端必须按不可信输入解析（Task 1 白名单设计以此为前提）。
3. 调查称后端 16 项测试「本组实跑通过」——本计划作者重新实跑确认：`go test ./internal/application/repository/ -run 'TestMobileDeviceRevocationIsOwnerScoped|TestMobileDeviceTenantAndEnvironmentIsolation|TestNotificationOutboxDeduplicates' -count=1` → `ok`；另实跑 `go test ./internal/handler/ -run 'TestMobileDevice' -count=1` → `ok`、`go test ./internal/handler/session/ -run 'TestMX021' -count=1` → `ok`。
4. 旧客户端树（`apps/mobile/sources/weknora/notifications/deep-link.ts`、`NotificationRouter.tsx`、`registration.ts`、`InboxScreen.tsx`）确认已随 commit 723de9179 整体删除：`rg 通知|deep.?link|device apps/mobile/src` 零命中，`packages/mobile-core` 无 device/inbox 目录。本计划全部为新增重建。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **伪造/恶意 deep link**：通知行（或外部注入的链接）携带 `javascript:`、`https://`、未知 host（`weknora://evil/detail`）、CRLF 注入、超长串时，若被直接导航会把用户带去任意页面或触发协议处理器。——Task 1 测试全量钉死（仅 `weknora://tasks/detail` + 非空 taskId/runId 通过，其余一律 `undefined`）+ Task 6 `openNotificationFromInbox` 的 `invalid-link` 分支不导航不 markRead。
2. **把通知正文/点击当作业务操作**：点击「需要你审批」通知若直接执行审批或恢复任务状态，等于把锁屏推送当授权。——Task 3 测试「markRead 只调用 remote.markRead、绝不触发 remote.inbox 或任何其他调用」+「applyHint 后视图 JSON 不含 hint.kind」；Task 6 测试「未授权面点击 → blocked-unauthorized，不导航、不 markRead」；服务端半边（MarkRead 单条 Update、推送 payload 只含 Kind）由回归命令守护。
3. **推送 token / 设备密文泄漏**：设备 token 是可寻址凭据，注册行里的 `token_ciphertext`/`token_hash`/`tenant_id`（`json:"-"`）若进入语义行或证据 JSON，会扩大泄漏面。——Task 4 测试断言 list 语义行与序列化输出均不含这些列；Task 7 测试断言证据 JSON 不匹配 `/password|email|deviceToken|Bearer/`。
4. **撤销后的设备复活与 intent 重放**：过期 registration-intent（409）若被无限重试会打转；撤销后再撤销、或 scope 撤销（切租户/登出）后注册迟到提交，会写出跨 scope 的注册。——Task 2 测试「二次 409 → DEVICE_CONFLICT 上抛（重试恰好一次）」「lease 撤销后 register/revoke/list 全拒 DEVICE_SCOPE_CHANGED」；服务端围栏（owner-scoped 撤销、epoch 围栏、重放拒绝）由 `TestMobileDevice*` 14 项回归命令守护。
5. **重复通知翻倍未读/重复渲染**：inbox 分页以 `created_at` 时间戳为 cursor，同刻多行在翻页边界可能重复出现，若不去重会重复渲染并让用户重复处理同一事项。——Task 3 测试「more() 合并分页按 notificationId 去重且 duplicateNotificationIds 可观测」（与 TaskOffice `duplicateRunIds` 同构）；服务端投递去重由 `TestNotificationOutboxDeduplicates` 回归守护。

## 任务结构与文件地图

| # | 任务 | 主要交付 | 共享文件改动 |
|---|---|---|---|
| 1 | mobile-core：安全深链纯函数 | `parseNotificationDeepLink`（错误 deep link 全拒） | `packages/mobile-core/src/index.ts` 追加导出 |
| 2 | mobile-core：Device Registration 深模块 | `createDeviceRegistry`（两步注册、409 单次重取、token 接管、scope 围栏）+ scenario Adapter | 同上 |
| 3 | mobile-core：行动通知 Inbox 深模块 | `createNotificationInbox`（hint 只重投影、去重、markRead 不执行业务）+ scenario Adapter | 同上 |
| 4 | api-client：设备注册 wire Adapter | `createMobileDeviceRemote`（intent/register/revoke/list 真实 wire） | `packages/api-client/package.json` exports 一行 |
| 5 | api-client：收件箱 wire Adapter | `createMobileInboxRemote`（inbox/markRead 真实 wire） | 同上一行 |
| 6 | apps/mobile：组装 | `/inbox` 路由、InboxScreen、深链安全导航、fail-closed push token、HomeScreen 入口 | `composition.ts` 追加、`HomeScreen.tsx` 一行、`app-smoke.test.tsx` 追加测试 |
| 7 | 端到端集成证据与计划级验证 | `DeviceInboxIntegrationEvidence` + opt-in 真实 HTTP 用例 + 回归命令 | 无 |

---

### Task 1: mobile-core 安全深链解析纯函数

**Files:**
- Create: `packages/mobile-core/src/inbox/deep-link.ts`
- Test: `packages/mobile-core/src/inbox/deep-link.test.ts`
- Modify: `packages/mobile-core/src/index.ts`（文件末尾追加导出）

**Interfaces:**
- Consumes: 无（纯函数，零依赖）。
- Produces: `interface TaskDetailDeepLinkTarget { kind: 'task-detail'; taskId: string; runId: string }`；`type DeepLinkTarget = TaskDetailDeepLinkTarget`；`parseNotificationDeepLink(value: string): DeepLinkTarget | undefined`——Task 3 的 `resolveTarget` 与 Task 6 的 `openNotificationFromInbox` 依赖此签名。合法格式唯一：`weknora://tasks/detail?taskId=<非空>&runId=<非空>`（与应用内既有路由 `/tasks/detail?taskId=..&runId=..`（`apps/mobile/src/app/tasks/detail.tsx`）参数名一致；`weknora://oidc` 是 OIDC 回调专用（`composition.ts:26`），不在通知深链白名单内）。

- [ ] **Step 1: Write the failing test**

创建 `packages/mobile-core/src/inbox/deep-link.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { parseNotificationDeepLink } from './deep-link.ts';

const VALID = 'weknora://tasks/detail?taskId=t-1&runId=r-1';

test('accepts the one canonical task-detail deep link and decodes ids', () => {
  assert.deepEqual(parseNotificationDeepLink(VALID), { kind: 'task-detail', taskId: 't-1', runId: 'r-1' });
  // scheme 大小写由 URL 归一（WHATWG URL 把 protocol 小写化）
  assert.deepEqual(parseNotificationDeepLink('WEKNORA://tasks/detail?taskId=t-1&runId=r-1'), { kind: 'task-detail', taskId: 't-1', runId: 'r-1' });
  // 百分号编码的 id 解码后保留原值（含空格与斜杠）
  assert.deepEqual(parseNotificationDeepLink('weknora://tasks/detail?taskId=a%20b&runId=r%2F1'), { kind: 'task-detail', taskId: 'a b', runId: 'r/1' });
});

test('rejects every malformed, foreign, or injected deep link', () => {
  const rejected = [
    '', '   ', 'not-a-url',
    VALID.replace('weknora:', 'https:'),          // https 伪装
    VALID.replace('weknora:', 'javascript:'),      // 协议处理器注入
    VALID.replace('weknora:', 'WEKNORA://tasks'),  // 双重前缀：path 变 //tasks/detail，不再是 /detail
    'weknora://evil/detail?taskId=t-1&runId=r-1',  // 未知 host
    'weknora://oidc?code=x&state=y',               // 非通知路由 host
    'weknora:///detail?taskId=t-1&runId=r-1',       // 空 host
    'weknora://tasks/other?taskId=t-1&runId=r-1',   // 未知 path
    'weknora://tasks/detail',                       // 缺全部参数
    'weknora://tasks/detail?taskId=t-1',            // 缺 runId
    'weknora://tasks/detail?taskId=%20&runId=r-1',  // 空白 taskId
    'weknora://tasks/detail?taskId=t-1&runId=',     // 空 runId
    `${VALID}#fragment`,                            // fragment 注入
    'weknora://tasks/detail?taskId=a%0Db&runId=r-1', // CRLF 注入（解码后含 \r）
    `weknora://tasks/detail?taskId=${'x'.repeat(129)}&runId=r-1`, // 超长 id
    `weknora://tasks/detail?taskId=t-1&runId=r-1&${'p=v&'.repeat(200)}x=1`, // 总长超 512（对齐 deep_link 列宽）
  ];
  for (const value of rejected) {
    assert.equal(parseNotificationDeepLink(value), undefined, `must reject: ${value.slice(0, 60)}`);
  }
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx tsx --test packages/mobile-core/src/inbox/deep-link.test.ts`
Expected: FAIL —— `Cannot find module .../deep-link.ts`（ERR_MODULE_NOT_FOUND），两个用例均失败。

- [ ] **Step 3: Write minimal implementation**

创建 `packages/mobile-core/src/inbox/deep-link.ts`：

```ts
/**
 * 行动通知安全深链解析（#41，AC2「错误 deep link 的验证」）。
 *
 * 服务端 workbench_notifications.deep_link 列当前无生产写入方（迁移 000190 默认 ''），
 * 因此该值一律按不可信输入处理：白名单 scheme + host + path，参数逐项校验，
 * 其余全部拒绝（返回 undefined，调用方不得导航）。合法格式唯一：
 *   weknora://tasks/detail?taskId=<非空 ≤128>&runId=<非空 ≤128>
 * （参数名与 app 内文件路由 /tasks/detail 一致；weknora://oidc 属 OIDC 回调，不在通知白名单。）
 */

export interface TaskDetailDeepLinkTarget {
  kind: 'task-detail';
  taskId: string;
  runId: string;
}

export type DeepLinkTarget = TaskDetailDeepLinkTarget;

const DEEP_LINK_SCHEME = 'weknora:';
const DEEP_LINK_HOSTS: ReadonlySet<string> = new Set(['tasks']);
const DEEP_LINK_PATH = '/detail';
const ID_MAX_LENGTH = 128;
/** 对齐 workbench_notifications.deep_link 列宽（VARCHAR(512)）。 */
const DEEP_LINK_MAX_LENGTH = 512;
/** 解码后的 id 不得含控制字符（CRLF/头注入）。 */
const CONTROL_CHARACTERS = /[\u0000-\u001f\u007f]/;

export function parseNotificationDeepLink(value: string): DeepLinkTarget | undefined {
  if (typeof value !== 'string' || value.trim() === '') return undefined;
  if (value.length > DEEP_LINK_MAX_LENGTH) return undefined;
  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    return undefined;
  }
  if (parsed.protocol !== DEEP_LINK_SCHEME) return undefined;
  if (!DEEP_LINK_HOSTS.has(parsed.host)) return undefined;
  if (parsed.pathname !== DEEP_LINK_PATH) return undefined;
  if (parsed.username !== '' || parsed.password !== '') return undefined;
  if (parsed.hash !== '') return undefined;
  const taskId = (parsed.searchParams.get('taskId') ?? '').trim();
  const runId = (parsed.searchParams.get('runId') ?? '').trim();
  if (taskId === '' || runId === '') return undefined;
  if (taskId.length > ID_MAX_LENGTH || runId.length > ID_MAX_LENGTH) return undefined;
  if (CONTROL_CHARACTERS.test(taskId) || CONTROL_CHARACTERS.test(runId)) return undefined;
  return { kind: 'task-detail', taskId, runId };
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npx tsx --test packages/mobile-core/src/inbox/deep-link.test.ts`
Expected: PASS（`# pass 2`，`# fail 0`）。

- [ ] **Step 5: 公共导出与提交**

在 `packages/mobile-core/src/index.ts` 末尾追加：

```ts
export { parseNotificationDeepLink } from './inbox/deep-link.ts';
export type { DeepLinkTarget, TaskDetailDeepLinkTarget } from './inbox/deep-link.ts';
```

Run: `npx tsx --test packages/mobile-core/src/inbox/deep-link.test.ts && pnpm --filter @weknora/mobile typecheck`
Expected: PASS + typecheck 干净（`tsc --noEmit` 无输出）。

```bash
git add packages/mobile-core/src/inbox/deep-link.ts packages/mobile-core/src/inbox/deep-link.test.ts packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core/inbox): 安全深链解析纯函数——错误 deep link 全拒（#41）"
```

---

### Task 2: mobile-core Device Registration 深模块

**Files:**
- Create: `packages/mobile-core/src/device/device-registry.ts`
- Create: `packages/mobile-core/src/device/in-memory-device-remote.ts`
- Test: `packages/mobile-core/src/device/device-registry.test.ts`
- Modify: `packages/mobile-core/src/index.ts`（末尾追加导出）

**Interfaces:**
- Consumes: `leaseActive(lease: ScopeLease | undefined): boolean`（`packages/mobile-core/src/runtime/scope-lease.ts:26`，包内 import）；`ScopeLease`（`packages/mobile-core/src/runtime/types.ts:17`）；ApiError 结构性形态（`packages/api-client/src/errors.ts:24-33`：`name === 'ApiError'` + `status?: number`——module 以结构判断，不 import api-client，保持依赖方向）。服务端 wire 契约（Task 4 的 `DeviceRemote` 实现）：两步注册 `POST /api/v1/mobile/devices/:id/registration-intent` → `PUT /api/v1/mobile/devices/:id`（`internal/handler/mobile_device.go:129-241`，intent epoch = current+1、5 分钟过期）；撤销 `DELETE /api/v1/mobile/devices/:id[?revision=N]` → 204，未注册/已撤销 404，revision 不匹配 409（`mobile_device.go:243-278`）；token 上限 4096、platform ∈ {ios, android}（`mobile_device.go:195`）。
- Produces（Task 6/7 依赖的精确签名）:
  - `type DevicePlatform = 'ios' | 'android'`
  - `interface DeviceRegistrationRecord { deviceId: string; platform: string; environment: string; revision: number; scopeGeneration: number; revokedAt?: string; lastSeenAt?: string }`
  - `interface DeviceRemote { issueIntent(deviceId: string): Promise<{ registrationIntent: string; scopeGeneration: number }>; register(input: { deviceId: string; token: string; platform: string; registrationIntent: string }): Promise<DeviceRegistrationRecord>; revoke(input: { deviceId: string; revision?: number }): Promise<void>; list(): Promise<DeviceRegistrationRecord[]> }`
  - `interface DevicePorts { remote: DeviceRemote; lease(): ScopeLease | undefined }`
  - `class DeviceError extends Error`（`code: DeviceErrorCode`，`DeviceErrorCode = 'DEVICE_SCOPE_CHANGED' | 'DEVICE_INVALID_INPUT' | 'DEVICE_CONFLICT' | 'DEVICE_NOT_FOUND' | 'DEVICE_BACKEND'`）
  - `createDeviceRegistry(ports: DevicePorts): DeviceRegistry`，`DeviceRegistry = { register(input: { deviceId: string; token: string; platform: DevicePlatform }): Promise<DeviceRegistrationRecord>; revoke(input: { deviceId: string; revision?: number }): Promise<void>; list(): Promise<DeviceRegistrationRecord[]> }`
  - `createScenarioDeviceRemote(): ScenarioDeviceRemote`（`{ remote: DeviceRemote; snapshot(): { active: DeviceRegistrationRecord[]; intentsIssued: number }; conflictNextRegisters(count: number): void }`）

- [ ] **Step 1: Write the failing test**

创建 `packages/mobile-core/src/device/device-registry.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { createDeviceRegistry, DeviceError } from './device-registry.ts';
import { createScenarioDeviceRemote } from './in-memory-device-remote.ts';

function leased() {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: '7' });
  return { revocable, lease: revocable.asScopeLease() };
}

function registryWith(leaseRef: { lease?: ScopeLease }) {
  const scenario = createScenarioDeviceRemote();
  const registry = createDeviceRegistry({ remote: scenario.remote, lease: () => leaseRef.lease });
  return { scenario, registry };
}

test('register performs the two-step intent then bind and returns the server record', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { scenario, registry } = registryWith(leaseRef);

  const record = await registry.register({ deviceId: 'device-1', token: 'push-token-a', platform: 'ios' });

  assert.equal(record.deviceId, 'device-1');
  assert.equal(record.platform, 'ios');
  assert.equal(record.revision, 1);
  assert.equal(scenario.snapshot().intentsIssued, 1, 'exactly one registration intent must be issued');
});

test('a stale-intent conflict (409) retries with one fresh intent and then succeeds', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { scenario, registry } = registryWith(leaseRef);
  scenario.conflictNextRegisters(1); // 第一次 register 409（模拟并发/过期 intent）

  const record = await registry.register({ deviceId: 'device-1', token: 'push-token-a', platform: 'android' });

  assert.equal(record.revision, 1);
  assert.equal(scenario.snapshot().intentsIssued, 2, 'exactly one re-issued intent after the conflict');
});

test('a second consecutive conflict surfaces DEVICE_CONFLICT instead of retrying forever', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { scenario, registry } = registryWith(leaseRef);
  scenario.conflictNextRegisters(2);

  await assert.rejects(
    registry.register({ deviceId: 'device-1', token: 'push-token-a', platform: 'ios' }),
    (error: unknown) => error instanceof DeviceError && error.code === 'DEVICE_CONFLICT',
  );
  assert.equal(scenario.snapshot().intentsIssued, 2, 'the retry bound is exactly one extra intent');
});

test('re-registering the same device takes over the token (upsert, revision bumps)', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { scenario, registry } = registryWith(leaseRef);

  await registry.register({ deviceId: 'device-1', token: 'push-token-a', platform: 'ios' });
  const second = await registry.register({ deviceId: 'device-1', token: 'push-token-b', platform: 'ios' });

  assert.ok(second.revision > 1, 'token takeover must bump the durable revision');
  assert.deepEqual(scenario.snapshot().active.map((row) => row.deviceId), ['device-1'], 'one active binding, never two');
});

test('concurrent registers are serialized so interleaved epochs cannot corrupt the takeover', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { scenario, registry } = registryWith(leaseRef);

  await Promise.all([
    registry.register({ deviceId: 'device-1', token: 'push-token-a', platform: 'ios' }),
    registry.register({ deviceId: 'device-1', token: 'push-token-b', platform: 'ios' }),
  ]);

  assert.equal(scenario.snapshot().active.length, 1, 'serial mutation leaves exactly one active binding');
  assert.equal(scenario.snapshot().active[0]!.revision, 2);
});

test('revoking an unknown device surfaces DEVICE_NOT_FOUND; revoking a registered device removes it', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { scenario, registry } = registryWith(leaseRef);

  await assert.rejects(
    registry.revoke({ deviceId: 'ghost' }),
    (error: unknown) => error instanceof DeviceError && error.code === 'DEVICE_NOT_FOUND',
  );
  await registry.register({ deviceId: 'device-1', token: 'push-token-a', platform: 'ios' });
  await registry.revoke({ deviceId: 'device-1' });
  assert.deepEqual(scenario.snapshot().active, []);
});

test('invalid input never reaches the remote', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { scenario, registry } = registryWith(leaseRef);

  const invalid = [
    { deviceId: '', token: 't', platform: 'ios' as const },
    { deviceId: '   ', token: 't', platform: 'ios' as const },
    { deviceId: 'd', token: '', platform: 'ios' as const },
    { deviceId: 'd', token: 't', platform: 'webos' as unknown as 'ios' },
    { deviceId: 'x'.repeat(129), token: 't', platform: 'ios' as const },
    { deviceId: 'd', token: 'x'.repeat(4097), platform: 'ios' as const },
  ];
  for (const input of invalid) {
    await assert.rejects(
      registry.register(input),
      (error: unknown) => error instanceof DeviceError && error.code === 'DEVICE_INVALID_INPUT',
    );
  }
  assert.equal(scenario.snapshot().intentsIssued, 0, 'validation happens before any wire call');
});

test('a revoked scope lease rejects register, revoke and list with DEVICE_SCOPE_CHANGED', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = { lease };
  const { registry } = registryWith(leaseRef);

  revocable.revoke(); // 切租户/换部署/登出后 Runtime 撤销 lease（#32 revoke 咽喉）

  for (const attempt of [
    () => registry.register({ deviceId: 'd', token: 't', platform: 'ios' }),
    () => registry.revoke({ deviceId: 'd' }),
    () => registry.list(),
  ]) {
    await assert.rejects(attempt(), (error: unknown) => error instanceof DeviceError && error.code === 'DEVICE_SCOPE_CHANGED');
  }
});

test('a late register completing after lease revocation is rejected, never resolved as success', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = { lease };
  const { scenario, registry } = registryWith(leaseRef);
  const slowRemote = {
    remote: {
      ...scenario.remote,
      issueIntent: async (deviceId: string) => {
        const intent = await scenario.remote.issueIntent(deviceId);
        revocable.revoke(); // intent 返回后、bind 提交前 scope 变化
        return intent;
      },
    },
  };
  const fenced = createDeviceRegistry({ remote: slowRemote.remote, lease: () => leaseRef.lease });

  await assert.rejects(
    fenced.register({ deviceId: 'd', token: 't', platform: 'ios' }),
    (error: unknown) => error instanceof DeviceError && error.code === 'DEVICE_SCOPE_CHANGED',
  );
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx tsx --test packages/mobile-core/src/device/device-registry.test.ts`
Expected: FAIL —— `Cannot find module .../device-registry.ts`（ERR_MODULE_NOT_FOUND）。

- [ ] **Step 3: Write minimal implementation**

创建 `packages/mobile-core/src/device/in-memory-device-remote.ts`：

```ts
import type { DeviceRemote, DeviceRegistrationRecord } from './device-registry.ts';

/** ApiError 结构性形态（packages/api-client/src/errors.ts:24-33）：name + status。 */
export function wireError(status: number, message: string): Error {
  return Object.assign(new Error(message), { name: 'ApiError', status });
}

export interface ScenarioDeviceSnapshot {
  active: DeviceRegistrationRecord[];
  intentsIssued: number;
}

export interface ScenarioDeviceRemote {
  remote: DeviceRemote;
  snapshot(): ScenarioDeviceSnapshot;
  /** 模拟过期/并发 intent：接下来 count 次 register 调用一律 409。 */
  conflictNextRegisters(count: number): void;
}

/**
 * in-memory scenario Adapter（module-seams §12：remote-but-owned 依赖用 in-memory
 * scenario Adapter 做 Module 测试）。复刻服务端两步注册的关键语义：intent 携带
 * current+1 epoch、bind 校验 intent、token 接管 bump revision（internal/handler/mobile_device.go:74-101/178-241）。
 */
export function createScenarioDeviceRemote(): ScenarioDeviceRemote {
  const rows = new Map<string, DeviceRegistrationRecord>();
  let epoch = 0;
  let conflicts = 0;
  let intents = 0;
  return {
    remote: {
      async issueIntent(deviceId) {
        intents += 1;
        epoch += 1;
        return { registrationIntent: `intent:${deviceId}:${epoch}`, scopeGeneration: epoch };
      },
      async register(input) {
        if (conflicts > 0) {
          conflicts -= 1;
          throw wireError(409, 'registration intent is stale');
        }
        if (input.registrationIntent !== `intent:${input.deviceId}:${epoch}`) {
          throw wireError(409, 'registration intent is stale');
        }
        const existing = rows.get(input.deviceId);
        const record: DeviceRegistrationRecord = {
          deviceId: input.deviceId,
          platform: input.platform,
          environment: 'development',
          revision: (existing?.revision ?? 0) + 1,
          scopeGeneration: epoch,
        };
        rows.set(input.deviceId, record);
        return { ...record };
      },
      async revoke(input) {
        const row = rows.get(input.deviceId);
        if (row === undefined) throw wireError(404, 'mobile device not found');
        if (input.revision !== undefined && input.revision !== row.revision) {
          throw wireError(409, 'mobile device revision conflict');
        }
        rows.delete(input.deviceId);
      },
      async list() {
        return [...rows.values()].map((row) => ({ ...row }));
      },
    },
    snapshot() {
      return { active: [...rows.values()].map((row) => ({ ...row })), intentsIssued: intents };
    },
    conflictNextRegisters(count) {
      conflicts = count;
    },
  };
}
```

创建 `packages/mobile-core/src/device/device-registry.ts`：

```ts
import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';

/**
 * Device Registration 深模块（#41，spec §4「注册设备」+ CONTEXT.md「注册设备」）：
 * - 两步注册：先取服务端签名的 registration intent（epoch=current+1，5 分钟过期），
 *   再以 intent + token 完成绑定——客户端不能自造未来 epoch；
 * - 409（intent 过期/并发）：恰好重取一次新 intent 重试，二次冲突显式 DEVICE_CONFLICT；
 * - token 接管：同 deviceId 重复 register 直接绑定新 token（服务端 upsert，revision 递增）；
 * - 单飞：同一 registry 上的注册/撤销串行提交，并发接管不交错 epoch；
 * - scope 围栏：每次提交前与两步之间检查 lease（切租户/换部署/登出即拒，迟到结果不落地）。
 * token 经 authorizedRequest 通道传输，从不进入返回记录、视图或错误文本。
 */

export type DevicePlatform = 'ios' | 'android';

export interface DeviceRegistrationRecord {
  deviceId: string;
  platform: string;
  environment: string;
  revision: number;
  scopeGeneration: number;
  revokedAt?: string;
  lastSeenAt?: string;
}

export interface DeviceRemote {
  issueIntent(deviceId: string): Promise<{ registrationIntent: string; scopeGeneration: number }>;
  register(input: { deviceId: string; token: string; platform: string; registrationIntent: string }): Promise<DeviceRegistrationRecord>;
  revoke(input: { deviceId: string; revision?: number }): Promise<void>;
  list(): Promise<DeviceRegistrationRecord[]>;
}

export interface DevicePorts {
  remote: DeviceRemote;
  lease(): ScopeLease | undefined;
}

export type DeviceErrorCode =
  | 'DEVICE_SCOPE_CHANGED'
  | 'DEVICE_INVALID_INPUT'
  | 'DEVICE_CONFLICT'
  | 'DEVICE_NOT_FOUND'
  | 'DEVICE_BACKEND';

export class DeviceError extends Error {
  constructor(readonly code: DeviceErrorCode, options?: { cause?: unknown }) {
    super(code, options);
    this.name = 'DeviceError';
  }
}

export interface DeviceRegistry {
  register(input: { deviceId: string; token: string; platform: DevicePlatform }): Promise<DeviceRegistrationRecord>;
  revoke(input: { deviceId: string; revision?: number }): Promise<void>;
  list(): Promise<DeviceRegistrationRecord[]>;
}

const deviceIdMaxLength = 128;
/** 对齐服务端 token 上限（internal/handler/mobile_device.go:195）。 */
const tokenMaxLength = 4096;

function wireStatus(error: unknown): number | undefined {
  if (typeof error !== 'object' || error === null) return undefined;
  if ((error as { name?: unknown }).name !== 'ApiError') return undefined;
  const status = (error as { status?: unknown }).status;
  return typeof status === 'number' ? status : undefined;
}

export function createDeviceRegistry(ports: DevicePorts): DeviceRegistry {
  let queue: Promise<unknown> = Promise.resolve();
  const serialized = <T>(action: () => Promise<T>): Promise<T> => {
    const next = queue.then(action, action);
    queue = next.then(() => {}, () => {});
    return next;
  };
  const requireLease = (): ScopeLease => {
    const lease = ports.lease();
    if (lease === undefined || !leaseActive(lease)) throw new DeviceError('DEVICE_SCOPE_CHANGED');
    return lease;
  };
  const attemptRegister = async (
    lease: ScopeLease,
    deviceId: string,
    token: string,
    platform: string,
  ): Promise<DeviceRegistrationRecord> => {
    const intent = await ports.remote.issueIntent(deviceId);
    if (!leaseActive(lease)) throw new DeviceError('DEVICE_SCOPE_CHANGED');
    return ports.remote.register({ deviceId, token, platform, registrationIntent: intent.registrationIntent });
  };
  return {
    register(input) {
      const deviceId = typeof input.deviceId === 'string' ? input.deviceId.trim() : '';
      const token = typeof input.token === 'string' ? input.token.trim() : '';
      const platform = input.platform;
      if (deviceId === '' || deviceId.length > deviceIdMaxLength) return Promise.reject(new DeviceError('DEVICE_INVALID_INPUT'));
      if (token === '' || token.length > tokenMaxLength) return Promise.reject(new DeviceError('DEVICE_INVALID_INPUT'));
      if (platform !== 'ios' && platform !== 'android') return Promise.reject(new DeviceError('DEVICE_INVALID_INPUT'));
      return serialized(async () => {
        const lease = requireLease();
        try {
          return await attemptRegister(lease, deviceId, token, platform);
        } catch (error) {
          if (error instanceof DeviceError) throw error;
          if (wireStatus(error) !== 409) throw new DeviceError('DEVICE_BACKEND', { cause: error });
          if (!leaseActive(lease)) throw new DeviceError('DEVICE_SCOPE_CHANGED');
          try {
            // intent 过期/并发（409）：恰好重取一次新 intent 再试（Review Focus #4：重试有界）
            return await attemptRegister(lease, deviceId, token, platform);
          } catch (retryError) {
            if (retryError instanceof DeviceError) throw retryError;
            if (wireStatus(retryError) === 409) throw new DeviceError('DEVICE_CONFLICT', { cause: retryError });
            throw new DeviceError('DEVICE_BACKEND', { cause: retryError });
          }
        }
      });
    },
    revoke(input) {
      const deviceId = typeof input.deviceId === 'string' ? input.deviceId.trim() : '';
      if (deviceId === '' || deviceId.length > deviceIdMaxLength) return Promise.reject(new DeviceError('DEVICE_INVALID_INPUT'));
      if (input.revision !== undefined && (!Number.isSafeInteger(input.revision) || input.revision <= 0)) {
        return Promise.reject(new DeviceError('DEVICE_INVALID_INPUT'));
      }
      return serialized(async (): Promise<void> => {
        requireLease();
        try {
          await ports.remote.revoke({ deviceId, ...(input.revision === undefined ? {} : { revision: input.revision }) });
        } catch (error) {
          if (error instanceof DeviceError) throw error;
          if (wireStatus(error) === 404) throw new DeviceError('DEVICE_NOT_FOUND', { cause: error });
          throw new DeviceError('DEVICE_BACKEND', { cause: error });
        }
      });
    },
    async list() {
      requireLease();
      try {
        return await ports.remote.list();
      } catch (error) {
        if (error instanceof DeviceError) throw error;
        throw new DeviceError('DEVICE_BACKEND', { cause: error });
      }
    },
  };
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npx tsx --test packages/mobile-core/src/device/device-registry.test.ts`
Expected: PASS（`# pass 9`，`# fail 0`）。

- [ ] **Step 5: 公共导出与提交**

在 `packages/mobile-core/src/index.ts` 末尾追加：

```ts
export { createDeviceRegistry, DeviceError } from './device/device-registry.ts';
export type { DeviceErrorCode, DevicePlatform, DevicePorts, DeviceRegistrationRecord, DeviceRegistry, DeviceRemote } from './device/device-registry.ts';
export { createScenarioDeviceRemote } from './device/in-memory-device-remote.ts';
export type { ScenarioDeviceRemote, ScenarioDeviceSnapshot } from './device/in-memory-device-remote.ts';
```

Run: `npx tsx --test packages/mobile-core/src/device/device-registry.test.ts && pnpm --filter @weknora/mobile typecheck`
Expected: PASS + typecheck 干净。

```bash
git add packages/mobile-core/src/device/device-registry.ts packages/mobile-core/src/device/device-registry.test.ts packages/mobile-core/src/device/in-memory-device-remote.ts packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core/device): Device Registration 深模块——两步注册、409 单次重取、token 接管与 scope 围栏（#41）"
```

---

### Task 3: mobile-core 行动通知 Inbox 深模块

**Files:**
- Create: `packages/mobile-core/src/inbox/notification-inbox.ts`
- Create: `packages/mobile-core/src/inbox/in-memory-inbox-remote.ts`
- Test: `packages/mobile-core/src/inbox/notification-inbox.test.ts`
- Modify: `packages/mobile-core/src/index.ts`（末尾追加导出）

**Interfaces:**
- Consumes: `parseNotificationDeepLink(value: string): DeepLinkTarget | undefined`（Task 1）；`leaseActive` / `ScopeLease`（同 Task 2）。服务端 wire 契约（Task 5 实现）：`GET /api/v1/workbench/inbox?cursor=` → `{items, unread_count, next_cursor}`（`internal/handler/session/workbench_inbox.go:84-119`，页限 50、`created_at <` 时间戳 cursor）；`POST /api/v1/workbench/inbox/read` body `{notification_id}` → success（`workbench_inbox.go:123-130` 单条 `Update("read", true)`，**不执行任何业务操作**，不存在 id 更新 0 行仍 success——幂等）。
- Produces（Task 6/7 依赖的精确签名）:
  - `interface InboxItem { notificationId: string; kind: string; title: string; body: string; createdAt: string; read: boolean; deepLink?: string }`
  - `interface InboxView { items: InboxItem[]; unreadCount: number; nextCursor?: string; duplicateNotificationIds: string[] }`
  - `interface InboxBackendItem`（与 `InboxItem` 字段逐字一致）/ `interface InboxBackendPage { items: InboxBackendItem[]; unreadCount: number; nextCursor?: string }`
  - `interface InboxRemote { inbox(cursor?: string): Promise<InboxBackendPage>; markRead(notificationId: string): Promise<void> }`
  - `interface NotificationInboxPorts { remote: InboxRemote; lease(): ScopeLease | undefined }`
  - `class InboxError extends Error`（`code: InboxErrorCode`，`InboxErrorCode = 'INBOX_SCOPE_CHANGED' | 'INBOX_SUPERSEDED' | 'INBOX_NO_ACTIVE_QUERY' | 'INBOX_INVALID_INPUT' | 'INBOX_BACKEND'`）
  - `createNotificationInbox(ports: NotificationInboxPorts): NotificationInbox`，其中
    `interface NotificationInbox { page(): Promise<InboxView>; more(): Promise<InboxView>; markRead(notificationId: string): Promise<void>; applyHint(hint: { kind?: string }): Promise<InboxView>; resolveTarget(item: InboxItem): DeepLinkTarget | undefined; subscribe(listener: (view: InboxView) => void): () => void }`
  - `createScenarioInboxRemote(pages: InboxScriptPage[]): ScenarioInboxRemote`（`{ remote: InboxRemote; calls(): string[]; setPages(pages: InboxScriptPage[]): void }`）

- [ ] **Step 1: Write the failing test**

创建 `packages/mobile-core/src/inbox/notification-inbox.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { createNotificationInbox, InboxError } from './notification-inbox.ts';
import type { InboxBackendPage, InboxRemote } from './notification-inbox.ts';
import { createScenarioInboxRemote } from './in-memory-inbox-remote.ts';

function leased() {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: '7' });
  return { revocable, lease: revocable.asScopeLease() };
}

function inboxWith(leaseRef: { lease?: ScopeLease }, pages: Parameters<typeof createScenarioInboxRemote>[0]) {
  const scenario = createScenarioInboxRemote(pages);
  const inbox = createNotificationInbox({ remote: scenario.remote, lease: () => leaseRef.lease });
  return { scenario, inbox };
}

test('page projects the first page with the unread count', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { inbox } = inboxWith(leaseRef, [
    { items: [{ notificationId: 'n-1', kind: 'attention', title: '需要你处理', read: false }], unreadCount: 1 },
  ]);

  const view = await inbox.page();

  assert.deepEqual(view.items, [{ notificationId: 'n-1', kind: 'attention', title: '需要你处理', body: '', createdAt: '2026-09-24T00:00:00Z', read: false }]);
  assert.equal(view.unreadCount, 1);
  assert.equal(view.nextCursor, undefined);
});

test('more merges pages, dedupes repeated notification ids observably (AC2 重复通知)', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { inbox } = inboxWith(leaseRef, [
    { items: [{ notificationId: 'n-1', read: false }, { notificationId: 'n-2', read: true }], nextCursor: 'c-1' },
    { items: [{ notificationId: 'n-2', read: true }, { notificationId: 'n-3', read: false }] }, // n-2 翻页边界重复
  ]);

  const first = await inbox.page();
  assert.equal(first.nextCursor, 'c-1');
  const second = await inbox.more();

  assert.deepEqual(second.items.map((item) => item.notificationId), ['n-1', 'n-2', 'n-3'], '重复行不重复渲染');
  assert.deepEqual(second.duplicateNotificationIds, ['n-2'], '重复通知可观测');
});

test('more without an active query or an exhausted cursor rejects INBOX_NO_ACTIVE_QUERY', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { inbox } = inboxWith(leaseRef, [{ items: [] }]);

  await assert.rejects(inbox.more(), (error: unknown) => error instanceof InboxError && error.code === 'INBOX_NO_ACTIVE_QUERY');
  await inbox.page();
  await assert.rejects(inbox.more(), (error: unknown) => error instanceof InboxError && error.code === 'INBOX_NO_ACTIVE_QUERY');
});

test('markRead is idempotent, and never performs any action besides the markRead wire call (AC1)', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { scenario, inbox } = inboxWith(leaseRef, [
    { items: [{ notificationId: 'n-1', kind: 'attention', read: false }, { notificationId: 'n-2', read: false }], unreadCount: 2 },
  ]);
  await inbox.page();
  const inboxCallsBefore = scenario.calls().length;

  await inbox.markRead('n-1');
  await inbox.markRead('n-1'); // 幂等：重复 markRead 仍只多一次同 id 的 wire 调用，本地未读不再递减（下一用例断言 1→0 只发生一次）

  // 行为断言：markRead 全程只新增 remote.markRead 调用，绝不触发 remote.inbox 重取或其他业务调用
  assert.deepEqual(scenario.calls().slice(inboxCallsBefore), ['markRead:n-1', 'markRead:n-1']);
  await assert.rejects(
    inbox.markRead('   '),
    (error: unknown) => error instanceof InboxError && error.code === 'INBOX_INVALID_INPUT',
  );
});

test('markRead publishes the decremented unread view to subscribers', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { inbox } = inboxWith(leaseRef, [
    { items: [{ notificationId: 'n-1', kind: 'attention', read: false }], unreadCount: 1 },
  ]);
  const seen: number[] = [];
  const unsubscribe = inbox.subscribe((view) => { seen.push(view.unreadCount); });
  await inbox.page();
  await inbox.markRead('n-1');
  unsubscribe();
  assert.deepEqual(seen, [1, 0], '订阅者看到未读 1 → 0');
});

test('applyHint only re-projects: hint.kind never enters the view and no business call happens (AC1)', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { scenario, inbox } = inboxWith(leaseRef, [
    { items: [{ notificationId: 'n-1', kind: 'attention', read: false }], unreadCount: 1 },
  ]);
  await inbox.page();
  scenario.setPages([{ items: [{ notificationId: 'n-1', kind: 'attention', read: true }], unreadCount: 0 }]);

  const view = await inbox.applyHint({ kind: 'run.completed' });

  assert.equal(view.items[0]!.read, true, 'hint 后投影来自权威重取');
  assert.equal(view.unreadCount, 0);
  assert.equal(JSON.stringify(view).includes('run.completed'), false, 'hint 语义不进入任何状态或视图');
  assert.deepEqual(scenario.calls(), ['inbox', 'inbox'], 'hint 只触发一次 inbox 重取，零其他调用');
});

test('resolveTarget wires the safe deep-link parser: valid passes, malformed is undefined', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { inbox } = inboxWith(leaseRef, [
    {
      items: [
        { notificationId: 'n-1', deepLink: 'weknora://tasks/detail?taskId=t-1&runId=r-1' },
        { notificationId: 'n-2', deepLink: 'https://evil.example/tasks/detail?taskId=t&runId=r' },
        { notificationId: 'n-3' },
      ],
    },
  ]);
  const view = await inbox.page();

  assert.deepEqual(inbox.resolveTarget(view.items[0]!), { kind: 'task-detail', taskId: 't-1', runId: 'r-1' });
  assert.equal(inbox.resolveTarget(view.items[1]!), undefined);
  assert.equal(inbox.resolveTarget(view.items[2]!), undefined);
});

test('a revoked scope lease rejects page and markRead with INBOX_SCOPE_CHANGED', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = { lease };
  const { inbox } = inboxWith(leaseRef, [{ items: [] }]);

  revocable.revoke();
  // 注：more() 不在此断言——本用例从未 page()，无活跃查询时 more() 先按 INBOX_NO_ACTIVE_QUERY
  // 拒绝（与 TaskOffice moreTasks 同语义，见上一用例）；lease 围栏对 more 的拦截由其内部
  // fetchPage 的 requireLease/settle 承担，已被 page/markRead 路径覆盖同一代码。
  await assert.rejects(inbox.page(), (error: unknown) => error instanceof InboxError && error.code === 'INBOX_SCOPE_CHANGED');
  await assert.rejects(inbox.markRead('n-1'), (error: unknown) => error instanceof InboxError && error.code === 'INBOX_SCOPE_CHANGED');
});

test('a late page result superseded by a newer page is rejected, never published', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  let releaseFirst: (() => void) | undefined;
  const hungRemote: InboxRemote = {
    inbox: async () => {
      if (releaseFirst === undefined) {
        await new Promise<void>((resolve) => { releaseFirst = resolve; });
        return { items: [], unreadCount: 0 };
      }
      return { items: [{ notificationId: 'fresh', read: false }], unreadCount: 1 };
    },
    markRead: async () => {},
  };
  const inbox = createNotificationInbox({ remote: hungRemote, lease: () => leaseRef.lease });

  const first = inbox.page();
  const second = inbox.page(); // 新查询发起，旧的被取代
  releaseFirst!();
  await second;
  await assert.rejects(first, (error: unknown) => error instanceof InboxError && error.code === 'INBOX_SUPERSEDED');
});

test('remote failures surface as INBOX_BACKEND without leaking raw errors', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const failing: InboxRemote = {
    inbox: async () => { throw Object.assign(new Error('SECRET-BEARER-VALUE'), { name: 'ApiError', status: 500 }); },
    markRead: async () => {},
  };
  const inbox = createNotificationInbox({ remote: failing, lease: () => leaseRef.lease });

  await assert.rejects(
    inbox.page(),
    (error: unknown) => error instanceof InboxError && error.code === 'INBOX_BACKEND' && !String((error as Error).message).includes('SECRET'),
  );
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx tsx --test packages/mobile-core/src/inbox/notification-inbox.test.ts`
Expected: FAIL —— `Cannot find module .../notification-inbox.ts`（ERR_MODULE_NOT_FOUND）。

- [ ] **Step 3: Write minimal implementation**

创建 `packages/mobile-core/src/inbox/in-memory-inbox-remote.ts`：

```ts
import type { InboxBackendPage, InboxRemote } from './notification-inbox.ts';

export interface InboxScriptPage {
  items: Array<{
    notificationId: string;
    kind?: string;
    title?: string;
    body?: string;
    createdAt?: string;
    read?: boolean;
    deepLink?: string;
  }>;
  unreadCount?: number;
  nextCursor?: string;
}

export interface ScenarioInboxRemote {
  remote: InboxRemote;
  /** 观测调用序列（「markRead 不触发 inbox 重取」等行为断言用）。 */
  calls(): string[];
  setPages(pages: InboxScriptPage[]): void;
}

/** in-memory scenario Adapter：每次 inbox() 消费一页，用尽后重复最后一页。 */
export function createScenarioInboxRemote(pages: InboxScriptPage[]): ScenarioInboxRemote {
  let queue = [...pages];
  const log: string[] = [];
  return {
    remote: {
      async inbox(cursor) {
        log.push(`inbox${cursor === undefined ? '' : `:${cursor}`}`);
        if (queue.length === 0) throw new Error('INBOX_SCRIPT_EXHAUSTED');
        const page = queue.length > 1 ? queue.shift()! : queue[0]!;
        const backendPage: InboxBackendPage = {
          items: page.items.map((item) => ({
            notificationId: item.notificationId,
            kind: item.kind ?? 'attention',
            title: item.title ?? '',
            body: item.body ?? '',
            createdAt: item.createdAt ?? '2026-09-24T00:00:00Z',
            read: item.read ?? false,
            ...(item.deepLink === undefined ? {} : { deepLink: item.deepLink }),
          })),
          unreadCount: page.unreadCount ?? page.items.filter((item) => item.read !== true).length,
          ...(page.nextCursor === undefined || page.nextCursor === '' ? {} : { nextCursor: page.nextCursor }),
        };
        return backendPage;
      },
      async markRead(notificationId) {
        log.push(`markRead:${notificationId}`);
      },
    },
    calls() {
      return [...log];
    },
    setPages(next) {
      queue = [...next];
    },
  };
}
```

创建 `packages/mobile-core/src/inbox/notification-inbox.ts`：

```ts
import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { parseNotificationDeepLink } from './deep-link.ts';
import type { DeepLinkTarget } from './deep-link.ts';

/**
 * 行动通知 Inbox 深模块（#41，CONTEXT.md「行动通知」：通知只提示客户端重新同步，
 * 不承载权威任务状态）：
 * - page()/more()：模块拥有 cursor 与查询身份，屏不维护分页状态（module-seams §10）；
 * - 重复通知：跨页重复的 notificationId 不再渲染，经 duplicateNotificationIds 观测
 *   （分页以 created_at 时间戳为 cursor，翻页边界同刻多行可能重复——TaskOffice duplicateRunIds 同构）；
 * - markRead：只置已读（本地投影 + 服务端幂等 Update("read", true)），绝不执行通知
 *   描述的任何业务操作（workbench_inbox.go:123-130 注释为证）；
 * - applyHint：推送 hint 只是触发器——等价一次权威重投影，hint.kind 不进入任何状态；
 * - resolveTarget：通知行 deep_link 一律经安全解析（deep-link.ts 白名单），错误链接返回 undefined；
 * - 迟到拒绝：scope lease 撤销或更新的查询使旧结果按 SUPERSEDED/SCOPE_CHANGED 拒绝。
 */

export interface InboxBackendItem {
  notificationId: string;
  kind: string;
  title: string;
  body: string;
  createdAt: string;
  read: boolean;
  deepLink?: string;
}

export interface InboxBackendPage {
  items: InboxBackendItem[];
  unreadCount: number;
  nextCursor?: string;
}

export interface InboxItem {
  notificationId: string;
  kind: string;
  title: string;
  body: string;
  createdAt: string;
  read: boolean;
  deepLink?: string;
}

export interface InboxView {
  items: InboxItem[];
  unreadCount: number;
  nextCursor?: string;
  duplicateNotificationIds: string[];
}

export interface InboxRemote {
  inbox(cursor?: string): Promise<InboxBackendPage>;
  markRead(notificationId: string): Promise<void>;
}

export interface NotificationInboxPorts {
  remote: InboxRemote;
  lease(): ScopeLease | undefined;
}

export type InboxErrorCode =
  | 'INBOX_SCOPE_CHANGED'
  | 'INBOX_SUPERSEDED'
  | 'INBOX_NO_ACTIVE_QUERY'
  | 'INBOX_INVALID_INPUT'
  | 'INBOX_BACKEND';

export class InboxError extends Error {
  constructor(readonly code: InboxErrorCode, options?: { cause?: unknown }) {
    super(code, options);
    this.name = 'InboxError';
  }
}

export interface NotificationInbox {
  page(): Promise<InboxView>;
  more(): Promise<InboxView>;
  markRead(notificationId: string): Promise<void>;
  /** 推送同步 hint：只触发一次权威重投影，不携带/写入任何业务状态（AC1 客户端半边）。 */
  applyHint(hint: { kind?: string }): Promise<InboxView>;
  resolveTarget(item: InboxItem): DeepLinkTarget | undefined;
  subscribe(listener: (view: InboxView) => void): () => void;
}

export function createNotificationInbox(ports: NotificationInboxPorts): NotificationInbox {
  const listeners = new Set<(view: InboxView) => void>();
  let sequence = 0;
  let view: InboxView | undefined;
  let seen = new Set<string>();
  let duplicates: string[] = [];
  let cursor: string | undefined;
  const requireLease = (): ScopeLease => {
    const lease = ports.lease();
    if (lease === undefined || !leaseActive(lease)) throw new InboxError('INBOX_SCOPE_CHANGED');
    return lease;
  };
  const callRemote = async <T>(action: () => Promise<T>): Promise<T> => {
    try {
      return await action();
    } catch (error) {
      if (error instanceof InboxError) throw error;
      throw new InboxError('INBOX_BACKEND', { cause: error });
    }
  };
  const publish = (next: InboxView): InboxView => {
    view = next;
    for (const listener of listeners) listener(next);
    return next;
  };
  const settle = (ticket: number, lease: ScopeLease): void => {
    if (ticket !== sequence) throw new InboxError('INBOX_SUPERSEDED');
    if (!leaseActive(lease)) throw new InboxError('INBOX_SCOPE_CHANGED');
  };
  const merge = (page: InboxBackendPage, reset: boolean): InboxView => {
    if (reset) {
      seen = new Set();
      duplicates = [];
    }
    const items: InboxItem[] = reset ? [] : [...(view?.items ?? [])];
    for (const item of page.items) {
      if (seen.has(item.notificationId)) {
        duplicates.push(item.notificationId);
        continue;
      }
      seen.add(item.notificationId);
      items.push({ ...item });
    }
    cursor = page.nextCursor;
    return publish({
      items,
      unreadCount: page.unreadCount,
      ...(page.nextCursor === undefined ? {} : { nextCursor: page.nextCursor }),
      duplicateNotificationIds: [...duplicates],
    });
  };
  const fetchPage = async (reset: boolean): Promise<InboxView> => {
    const lease = requireLease();
    const ticket = ++sequence;
    const page = await callRemote(() => ports.remote.inbox(reset ? undefined : cursor));
    settle(ticket, lease);
    return merge(page, reset);
  };
  return {
    page: () => fetchPage(true),
    more: () => {
      if (view === undefined || cursor === undefined) return Promise.reject(new InboxError('INBOX_NO_ACTIVE_QUERY'));
      return fetchPage(false);
    },
    async markRead(notificationId: string): Promise<void> {
      const trimmed = typeof notificationId === 'string' ? notificationId.trim() : '';
      if (trimmed === '') throw new InboxError('INBOX_INVALID_INPUT');
      const lease = requireLease();
      await callRemote(() => ports.remote.markRead(trimmed));
      if (!leaseActive(lease)) throw new InboxError('INBOX_SCOPE_CHANGED');
      if (view !== undefined) {
        const wasUnread = view.items.some((item) => item.notificationId === trimmed && !item.read);
        const items = view.items.map((item) => (item.notificationId === trimmed && !item.read ? { ...item, read: true } : item));
        publish({
          items,
          unreadCount: wasUnread ? Math.max(0, view.unreadCount - 1) : view.unreadCount,
          ...(view.nextCursor === undefined ? {} : { nextCursor: view.nextCursor }),
          duplicateNotificationIds: view.duplicateNotificationIds,
        });
      }
    },
    applyHint(hint: { kind?: string }): Promise<InboxView> {
      void hint; // hint 只是触发器：kind 不进入任何状态或视图（AC1）
      return fetchPage(true);
    },
    resolveTarget(item: InboxItem): DeepLinkTarget | undefined {
      return parseNotificationDeepLink(item.deepLink ?? '');
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npx tsx --test packages/mobile-core/src/inbox/notification-inbox.test.ts`
Expected: PASS（`# pass 10`，`# fail 0`）。

- [ ] **Step 5: 公共导出与提交**

在 `packages/mobile-core/src/index.ts` 末尾追加：

```ts
export { createNotificationInbox, InboxError } from './inbox/notification-inbox.ts';
export type {
  InboxBackendItem, InboxBackendPage, InboxErrorCode, InboxItem, InboxRemote, InboxView,
  NotificationInbox, NotificationInboxPorts,
} from './inbox/notification-inbox.ts';
export { createScenarioInboxRemote } from './inbox/in-memory-inbox-remote.ts';
export type { InboxScriptPage, ScenarioInboxRemote } from './inbox/in-memory-inbox-remote.ts';
```

Run: `npx tsx --test packages/mobile-core/src/inbox/notification-inbox.test.ts packages/mobile-core/src/inbox/deep-link.test.ts && pnpm --filter @weknora/mobile typecheck`
Expected: PASS + typecheck 干净。

```bash
git add packages/mobile-core/src/inbox/notification-inbox.ts packages/mobile-core/src/inbox/notification-inbox.test.ts packages/mobile-core/src/inbox/in-memory-inbox-remote.ts packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core/inbox): 行动通知 Inbox 深模块——hint 只重投影、重复通知去重、markRead 不执行业务（#41）"
```

---

### Task 4: api-client 设备注册 wire Adapter

**Files:**
- Create: `packages/api-client/src/mobile/devices.ts`
- Test: `packages/api-client/src/mobile/devices.test.ts`
- Modify: `packages/api-client/package.json`（`exports` 追加 `"./mobile/devices": "./src/mobile/devices.ts"`）

**Interfaces:**
- Consumes: `ClientRequest`（`packages/api-client/src/client.ts`——`{ method: string; path: string; headers?: Record<string, string>; body?: unknown }` 通道，与 `createTaskOfficeRemote` 同一 `Request = (input: ClientRequest) => Promise<unknown>` 模式）；成功信封 `{"success": true, "data": ...}`（`mobile_device.go:159/237/397`）。服务端 wire 精确形状（当前 HEAD 亲眼核实）：
  - intent 响应 data：`{"registration_intent": string, "scope_generation": number}`（`mobile_device.go:159`）
  - register 响应 data：`{"device_id","environment","platform","scope_generation","revision"}`（`mobile_device.go:237-240`）
  - list 响应 data：`DeviceRegistration` JSON 数组，列 `device_id/owner_id/environment/platform/revision/scope_generation/revoked_at?/last_seen_at?`；敏感列 `TenantID/SpaceID/TokenCiphertext/TokenHash` 均为 `json:"-"` 不上 wire（`internal/application/repository/mobile_device.go:29-44`——struct 定义在 repository 层，handler 侧 `internal/handler/mobile_device.go` 只消费）
  - revoke：204 No Content；404 未注册/已撤销；409 revision 冲突（`mobile_device.go:400-416`）
- Produces: `createMobileDeviceRemote(options: { origin: string; request: Request }): MobileDeviceRemote`，`MobileDeviceRemote` 方法与 mobile-core `DeviceRemote`（Task 2）**结构逐字一致**（`issueIntent(deviceId: string): Promise<{ registrationIntent: string; scopeGeneration: number }>` / `register(input: { deviceId; token; platform; registrationIntent }): Promise<MobileDeviceRegistration>` / `revoke(input: { deviceId; revision? }): Promise<void>` / `list(): Promise<MobileDeviceRegistration[]>`；`MobileDeviceRegistration` 字段 = `DeviceRegistrationRecord`）。结构可赋值由 Task 6 组合根的 `pnpm --filter @weknora/mobile typecheck` 实证（#35 `RemoteTaskDetail` 同模式）。

- [ ] **Step 1: Write the failing test**

创建 `packages/api-client/src/mobile/devices.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileDeviceRemote } from './devices.ts';
import type { ClientRequest } from '../client.ts';

const ORIGIN = 'https://weknora.example.test';

function recorder(responder: (input: ClientRequest) => unknown): { seen: ClientRequest[]; request: (input: ClientRequest) => Promise<unknown> } {
  const seen: ClientRequest[] = [];
  return {
    seen,
    request: async (input: ClientRequest): Promise<unknown> => {
      seen.push(input);
      return responder(input);
    },
  };
}

/** 真实 POST /api/v1/mobile/devices/:id/registration-intent 响应（internal/handler/mobile_device.go:159）。 */
const INTENT_WIRE = { success: true, data: { registration_intent: 'aW50ZW50.WQ', scope_generation: 2 } };
/** 真实 PUT /api/v1/mobile/devices/:id 响应（internal/handler/mobile_device.go:237-240）。 */
const REGISTER_WIRE = { success: true, data: { device_id: 'device-1', environment: 'production', platform: 'ios', scope_generation: 2, revision: 1 } };
/** 真实 GET /api/v1/mobile/devices 响应行（internal/application/repository/mobile_device.go:29-44：TokenCiphertext/TokenHash/TenantID/SpaceID 均 json:"-"，不上 wire）。 */
const LIST_WIRE = {
  success: true,
  data: [
    { device_id: 'device-1', owner_id: 'user-1', environment: 'production', platform: 'ios', revision: 3, scope_generation: 5, revoked_at: null, last_seen_at: '2026-09-24T00:00:00Z' },
    { device_id: 'device-2', owner_id: 'user-1', environment: 'production', platform: 'android', revision: 1, scope_generation: 1 },
  ],
};

test('issueIntent posts the registration-intent endpoint and unwraps the envelope', async () => {
  const spy = recorder(() => INTENT_WIRE);
  const remote = createMobileDeviceRemote({ origin: ORIGIN, request: spy.request });

  const intent = await remote.issueIntent('device-1');

  assert.deepEqual(intent, { registrationIntent: 'aW50ZW50.WQ', scopeGeneration: 2 });
  assert.equal(spy.seen[0]!.method, 'POST');
  assert.equal(spy.seen[0]!.path, '/api/v1/mobile/devices/device-1/registration-intent');
});

test('register puts the sealed wire body and maps the returned record', async () => {
  const spy = recorder(() => REGISTER_WIRE);
  const remote = createMobileDeviceRemote({ origin: ORIGIN, request: spy.request });

  const record = await remote.register({ deviceId: 'device-1', token: 'push-token', platform: 'ios', registrationIntent: 'aW50ZW50.WQ' });

  assert.deepEqual(record, { deviceId: 'device-1', platform: 'ios', environment: 'production', revision: 1, scopeGeneration: 2 });
  assert.equal(spy.seen[0]!.method, 'PUT');
  assert.equal(spy.seen[0]!.path, '/api/v1/mobile/devices/device-1');
  assert.deepEqual(spy.seen[0]!.body, { token: 'push-token', platform: 'ios', registration_intent: 'aW50ZW50.WQ' });
});

test('revoke deletes with an optional revision query and treats 204 as success', async () => {
  const spy = recorder(() => undefined);
  const remote = createMobileDeviceRemote({ origin: ORIGIN, request: spy.request });

  await remote.revoke({ deviceId: 'device-1' });
  await remote.revoke({ deviceId: 'device-1', revision: 3 });

  assert.equal(spy.seen[0]!.method, 'DELETE');
  assert.equal(spy.seen[0]!.path, '/api/v1/mobile/devices/device-1');
  assert.equal(spy.seen[1]!.path, '/api/v1/mobile/devices/device-1?revision=3');
});

test('device ids are URL-encoded into the path (injection-safe) and revision must be a positive integer', async () => {
  const spy = recorder(() => INTENT_WIRE);
  const remote = createMobileDeviceRemote({ origin: ORIGIN, request: spy.request });

  await remote.issueIntent('a/b c?x=1');
  assert.equal(spy.seen[0]!.path, '/api/v1/mobile/devices/a%2Fb%20c%3Fx%3D1/registration-intent');

  await assert.rejects(remote.revoke({ deviceId: 'd', revision: 0 }), /positive integer/);
  await assert.rejects(remote.revoke({ deviceId: 'd', revision: 1.5 }), /positive integer/);
});

test('list maps rows and never surfaces token ciphertext columns (Review Focus #3)', async () => {
  const spy = recorder(() => LIST_WIRE);
  const remote = createMobileDeviceRemote({ origin: ORIGIN, request: spy.request });

  const rows = await remote.list();

  assert.deepEqual(rows, [
    { deviceId: 'device-1', platform: 'ios', environment: 'production', revision: 3, scopeGeneration: 5, lastSeenAt: '2026-09-24T00:00:00Z' },
    { deviceId: 'device-2', platform: 'android', environment: 'production', revision: 1, scopeGeneration: 1 },
  ]);
  const serialized = JSON.stringify(rows);
  for (const forbidden of ['token_ciphertext', 'token_hash', 'tenant_id', 'owner_id']) {
    assert.equal(serialized.includes(forbidden), false, `semantic rows must not carry ${forbidden}`);
  }
});

test('malformed envelopes and origins fail fast', async () => {
  const failing = recorder(() => ({ success: false }));
  const remote = createMobileDeviceRemote({ origin: ORIGIN, request: failing.request });
  await assert.rejects(remote.issueIntent('device-1'), /success/);
  await assert.rejects(remote.list(), /success/);

  assert.throws(() => createMobileDeviceRemote({ origin: 'http://weknora.example.test', request: failing.request }), /HTTPS/);
  assert.throws(() => createMobileDeviceRemote({ origin: 'https://weknora.example.test/path', request: failing.request }), /path/);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx tsx --test packages/api-client/src/mobile/devices.test.ts`
Expected: FAIL —— `Cannot find module .../devices.ts`（ERR_MODULE_NOT_FOUND）。

- [ ] **Step 3: Write minimal implementation**

创建 `packages/api-client/src/mobile/devices.ts`：

```ts
import type { ClientRequest } from '../client.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface MobileDeviceRemoteOptions {
  /**
   * 部署 Origin。构造即强校验（与 createMobileRuntimeRemote / createTaskOfficeRemote 同一
   * requireDeploymentOrigin 模式）：绝对 HTTPS、无 userinfo、无 path/query/fragment。
   */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输。 */
  request: Request;
}

/** 与 mobile-core DeviceRegistrationRecord 结构逐字一致（结构可赋值由 apps/mobile typecheck 证明）。 */
export interface MobileDeviceRegistration {
  deviceId: string;
  platform: string;
  environment: string;
  revision: number;
  scopeGeneration: number;
  revokedAt?: string;
  lastSeenAt?: string;
}

export interface MobileDeviceRemote {
  /** POST /api/v1/mobile/devices/:id/registration-intent（internal/handler/mobile_device.go:129）。 */
  issueIntent(deviceId: string): Promise<{ registrationIntent: string; scopeGeneration: number }>;
  /** PUT /api/v1/mobile/devices/:id（mobile_device.go:178）——两步注册第二步。 */
  register(input: { deviceId: string; token: string; platform: string; registrationIntent: string }): Promise<MobileDeviceRegistration>;
  /** DELETE /api/v1/mobile/devices/:id[?revision=N]（mobile_device.go:243）——204。 */
  revoke(input: { deviceId: string; revision?: number }): Promise<void>;
  /** GET /api/v1/mobile/devices（mobile_device.go:386）——敏感列 json:"-" 不上 wire。 */
  list(): Promise<MobileDeviceRegistration[]>;
}

function requireDeploymentOrigin(origin: string): void {
  let parsed: URL;
  if (typeof origin !== 'string' || origin.trim() === '') throw new Error('deployment origin is required');
  try {
    parsed = new URL(origin);
  } catch {
    throw new Error(`deployment origin must be an absolute URL: ${origin}`);
  }
  if (parsed.protocol !== 'https:') throw new Error('deployment origin must use HTTPS');
  if (parsed.username !== '' || parsed.password !== '') throw new Error('deployment origin must not embed user info');
  if (parsed.hostname === '') throw new Error('deployment origin must include a host');
  if (parsed.pathname !== '/') throw new Error('deployment origin must not include a path');
  if (parsed.search !== '' || parsed.hash !== '') throw new Error('deployment origin must not include a query or fragment');
}

function requireDeviceId(deviceId: string): string {
  const trimmed = typeof deviceId === 'string' ? deviceId.trim() : '';
  if (trimmed === '') throw new Error('device id is required');
  return trimmed;
}

function unwrap(value: unknown, label: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error(`${label} response must be a success envelope`);
  const envelope = value as { success?: unknown; data?: unknown };
  if (envelope.success !== true || !Object.prototype.hasOwnProperty.call(envelope, 'data')) {
    throw new Error(`${label} response.success must be true with data`);
  }
  if (typeof envelope.data !== 'object' || envelope.data === null || Array.isArray(envelope.data)) {
    throw new Error(`${label} data must be an object`);
  }
  return envelope.data as Record<string, unknown>;
}

function optionalTimestamp(data: Record<string, unknown>, key: string): string | undefined {
  const value = data[key];
  return typeof value === 'string' && value.trim() !== '' ? value : undefined;
}

export function createMobileDeviceRemote(options: MobileDeviceRemoteOptions): MobileDeviceRemote {
  requireDeploymentOrigin(options.origin);
  const request = options.request;
  const devicePath = (deviceId: string): string => `/api/v1/mobile/devices/${encodeURIComponent(requireDeviceId(deviceId))}`;
  return {
    async issueIntent(deviceId: string): Promise<{ registrationIntent: string; scopeGeneration: number }> {
      const data = unwrap(await request({ method: 'POST', path: `${devicePath(deviceId)}/registration-intent` }), 'device intent');
      if (typeof data.registration_intent !== 'string' || data.registration_intent.trim() === '') {
        throw new Error('device intent registration_intent is required');
      }
      if (typeof data.scope_generation !== 'number' || !Number.isSafeInteger(data.scope_generation) || data.scope_generation < 0) {
        throw new Error('device intent scope_generation must be an integer');
      }
      return { registrationIntent: data.registration_intent, scopeGeneration: data.scope_generation };
    },
    async register(input: { deviceId: string; token: string; platform: string; registrationIntent: string }): Promise<MobileDeviceRegistration> {
      if (typeof input.token !== 'string' || input.token.trim() === '') throw new Error('device token is required');
      if (typeof input.platform !== 'string' || input.platform.trim() === '') throw new Error('device platform is required');
      if (typeof input.registrationIntent !== 'string' || input.registrationIntent.trim() === '') throw new Error('device registration_intent is required');
      const data = unwrap(await request({
        method: 'PUT',
        path: devicePath(input.deviceId),
        body: { token: input.token, platform: input.platform, registration_intent: input.registrationIntent },
      }), 'device register');
      const registration: MobileDeviceRegistration = {
        deviceId: requireDeviceId(String(data.device_id ?? '')),
        platform: String(data.platform ?? ''),
        environment: String(data.environment ?? ''),
        revision: Number(data.revision ?? 0),
        scopeGeneration: Number(data.scope_generation ?? 0),
        ...(optionalTimestamp(data, 'revoked_at') === undefined ? {} : { revokedAt: optionalTimestamp(data, 'revoked_at')! }),
        ...(optionalTimestamp(data, 'last_seen_at') === undefined ? {} : { lastSeenAt: optionalTimestamp(data, 'last_seen_at')! }),
      };
      if (registration.deviceId === '') throw new Error('device register device_id is required');
      return registration;
    },
    async revoke(input: { deviceId: string; revision?: number }): Promise<void> {
      if (input.revision !== undefined && (!Number.isSafeInteger(input.revision) || input.revision <= 0)) {
        throw new Error('device revision must be a positive integer');
      }
      const query = input.revision === undefined ? '' : `?revision=${input.revision}`;
      await request({ method: 'DELETE', path: `${devicePath(input.deviceId)}${query}` }); // 204：无 body 可解
    },
    async list(): Promise<MobileDeviceRegistration[]> {
      const response = await request({ method: 'GET', path: '/api/v1/mobile/devices' });
      if (typeof response !== 'object' || response === null || Array.isArray(response)) {
        throw new Error('device list response must be a success envelope');
      }
      const envelope = response as { success?: unknown; data?: unknown };
      if (envelope.success !== true || !Array.isArray(envelope.data)) {
        throw new Error('device list response.success must be true with a data array');
      }
      const rows = envelope.data as Array<Record<string, unknown>>;
      // wire 行的敏感列（TokenCiphertext/TokenHash/TenantID/SpaceID）在服务端就是 json:"-"（internal/application/repository/mobile_device.go:29-44），
      // 适配器只提取白名单字段，结构性杜绝 token 材料进入语义行（Review Focus #3）。
      return rows.map((row) => ({
        deviceId: typeof row.device_id === 'string' ? row.device_id : '',
        platform: typeof row.platform === 'string' ? row.platform : '',
        environment: typeof row.environment === 'string' ? row.environment : '',
        revision: typeof row.revision === 'number' ? row.revision : 0,
        scopeGeneration: typeof row.scope_generation === 'number' ? row.scope_generation : 0,
        ...(optionalTimestamp(row, 'revoked_at') === undefined ? {} : { revokedAt: optionalTimestamp(row, 'revoked_at')! }),
        ...(optionalTimestamp(row, 'last_seen_at') === undefined ? {} : { lastSeenAt: optionalTimestamp(row, 'last_seen_at')! }),
      }));
    },
  };
}

在 `packages/api-client/package.json` 的 `exports` 中，`"./mobile/runtime"` 行之前追加：

```json
    "./mobile/devices": "./src/mobile/devices.ts",
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npx tsx --test packages/api-client/src/mobile/devices.test.ts`
Expected: PASS（`# pass 6`，`# fail 0`）。

- [ ] **Step 5: Commit**

```bash
git add packages/api-client/src/mobile/devices.ts packages/api-client/src/mobile/devices.test.ts packages/api-client/package.json
git commit -m "feat(api-client/mobile): 设备注册 wire Adapter——intent/register/revoke/list（#41）"
```

---

### Task 5: api-client 收件箱 wire Adapter

**Files:**
- Create: `packages/api-client/src/mobile/inbox.ts`
- Test: `packages/api-client/src/mobile/inbox.test.ts`
- Modify: `packages/api-client/package.json`（`exports` 追加 `"./mobile/inbox": "./src/mobile/inbox.ts"`）

**Interfaces:**
- Consumes: `ClientRequest` 通道（同 Task 4）；成功信封（`workbench_inbox.go:207/232`）。服务端 wire 精确形状：
  - inbox data：`{"items": [{"notification_id","kind","title","body","created_at","read","deep_link"(omitempty)}], "unread_count": number, "next_cursor": string}`（`workbench_inbox.go:53-67`，`next_cursor` 空串表示无下一页，`workbench_inbox.go:96-100`）
  - markRead：POST body `{"notification_id"}` → `{"success":true,"data":{"notification_id","read":true}}`（`workbench_inbox.go:210-233`）
- Produces: `createMobileInboxRemote(options: { origin: string; request: Request }): MobileInboxRemote`，`MobileInboxRemote = { inbox(cursor?: string): Promise<RemoteInboxPage>; markRead(notificationId: string): Promise<void> }`；`RemoteInboxItem`/`RemoteInboxPage` 与 mobile-core `InboxBackendItem`/`InboxBackendPage`（Task 3）结构逐字一致（`notificationId/kind/title/body/createdAt/read/deepLink?` 与 `items/unreadCount/nextCursor?`）。

- [ ] **Step 1: Write the failing test**

创建 `packages/api-client/src/mobile/inbox.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileInboxRemote } from './inbox.ts';
import type { ClientRequest } from '../client.ts';

const ORIGIN = 'https://weknora.example.test';

function recorder(responder: (input: ClientRequest) => unknown): { seen: ClientRequest[]; request: (input: ClientRequest) => Promise<unknown> } {
  const seen: ClientRequest[] = [];
  return {
    seen,
    request: async (input: ClientRequest): Promise<unknown> => {
      seen.push(input);
      return responder(input);
    },
  };
}

/** 真实 GET /api/v1/workbench/inbox 响应（internal/handler/session/workbench_inbox.go:53-119）。 */
const INBOX_WIRE = {
  success: true,
  data: {
    items: [
      { notification_id: 'n-1', kind: 'attention', title: '需要你处理', body: '', created_at: '2026-09-24T01:00:00Z', read: false, deep_link: 'weknora://tasks/detail?taskId=t-1&runId=r-1' },
      { notification_id: 'n-2', kind: 'budget', title: '预算事件', body: '', created_at: '2026-09-24T02:00:00Z', read: true, deep_link: '' },
    ],
    unread_count: 1,
    next_cursor: '2026-09-24T01:00:00Z',
  },
};
const INBOX_WIRE_END = { success: true, data: { items: [], unread_count: 0, next_cursor: '' } };
const MARK_READ_WIRE = { success: true, data: { notification_id: 'n-1', read: true } };

test('inbox maps wire rows, omits empty deep_link, and normalizes an empty next_cursor', async () => {
  let call = 0;
  const spy = recorder(() => (call += 1) === 1 ? INBOX_WIRE : INBOX_WIRE_END);
  const remote = createMobileInboxRemote({ origin: ORIGIN, request: spy.request });

  const first = await remote.inbox();
  assert.deepEqual(first, {
    items: [
      { notificationId: 'n-1', kind: 'attention', title: '需要你处理', body: '', createdAt: '2026-09-24T01:00:00Z', read: false, deepLink: 'weknora://tasks/detail?taskId=t-1&runId=r-1' },
      { notificationId: 'n-2', kind: 'budget', title: '预算事件', body: '', createdAt: '2026-09-24T02:00:00Z', read: true },
    ],
    unreadCount: 1,
    nextCursor: '2026-09-24T01:00:00Z',
  });
  assert.equal(spy.seen[0]!.method, 'GET');
  assert.equal(spy.seen[0]!.path, '/api/v1/workbench/inbox');

  const last = await remote.inbox('2026-09-24T01:00:00Z');
  assert.equal(last.nextCursor, undefined, '空 next_cursor 归一为无下一页');
  assert.equal(spy.seen[1]!.path, '/api/v1/workbench/inbox?cursor=2026-09-24T01%3A00%3A00Z', 'cursor 必须编码进 query');
});

test('markRead posts the notification_id body and expects the read receipt envelope', async () => {
  const spy = recorder(() => MARK_READ_WIRE);
  const remote = createMobileInboxRemote({ origin: ORIGIN, request: spy.request });

  await remote.markRead('n-1');

  assert.equal(spy.seen[0]!.method, 'POST');
  assert.equal(spy.seen[0]!.path, '/api/v1/workbench/inbox/read');
  assert.deepEqual(spy.seen[0]!.body, { notification_id: 'n-1' });
  await assert.rejects(remote.markRead('  '), /notification id/);
});

test('malformed envelopes and origins fail fast', async () => {
  const failing = recorder(() => ({ data: {} }));
  const remote = createMobileInboxRemote({ origin: ORIGIN, request: failing.request });
  await assert.rejects(remote.inbox(), /success/);

  assert.throws(() => createMobileInboxRemote({ origin: 'https://weknora.example.test/api/v1', request: failing.request }), /path/);
  assert.throws(() => createMobileInboxRemote({ origin: 'https://user:pass@weknora.example.test', request: failing.request }), /user info/);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx tsx --test packages/api-client/src/mobile/inbox.test.ts`
Expected: FAIL —— `Cannot find module .../inbox.ts`（ERR_MODULE_NOT_FOUND）。

- [ ] **Step 3: Write minimal implementation**

创建 `packages/api-client/src/mobile/inbox.ts`：

```ts
import type { ClientRequest } from '../client.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface MobileInboxRemoteOptions {
  /** 部署 Origin：构造即强校验（同 createMobileDeviceRemote 模式）。 */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输。 */
  request: Request;
}

/** 与 mobile-core InboxBackendItem 结构逐字一致（结构可赋值由 apps/mobile typecheck 证明）。 */
export interface RemoteInboxItem {
  notificationId: string;
  kind: string;
  title: string;
  body: string;
  createdAt: string;
  read: boolean;
  deepLink?: string;
}

/** 与 mobile-core InboxBackendPage 结构逐字一致。 */
export interface RemoteInboxPage {
  items: RemoteInboxItem[];
  unreadCount: number;
  nextCursor?: string;
}

export interface MobileInboxRemote {
  /** GET /api/v1/workbench/inbox?cursor=（internal/handler/session/workbench_inbox.go:193）。 */
  inbox(cursor?: string): Promise<RemoteInboxPage>;
  /** POST /api/v1/workbench/inbox/read（workbench_inbox.go:214）——只置已读，服务端不执行任何业务操作。 */
  markRead(notificationId: string): Promise<void>;
}

function requireDeploymentOrigin(origin: string): void {
  let parsed: URL;
  if (typeof origin !== 'string' || origin.trim() === '') throw new Error('deployment origin is required');
  try {
    parsed = new URL(origin);
  } catch {
    throw new Error(`deployment origin must be an absolute URL: ${origin}`);
  }
  if (parsed.protocol !== 'https:') throw new Error('deployment origin must use HTTPS');
  if (parsed.username !== '' || parsed.password !== '') throw new Error('deployment origin must not embed user info');
  if (parsed.hostname === '') throw new Error('deployment origin must include a host');
  if (parsed.pathname !== '/') throw new Error('deployment origin must not include a path');
  if (parsed.search !== '' || parsed.hash !== '') throw new Error('deployment origin must not include a query or fragment');
}

function requireString(value: unknown, label: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${label} is required`);
  return value;
}

export function createMobileInboxRemote(options: MobileInboxRemoteOptions): MobileInboxRemote {
  requireDeploymentOrigin(options.origin);
  const request = options.request;
  const mapItem = (row: Record<string, unknown>): RemoteInboxItem => ({
    notificationId: requireString(row.notification_id, 'inbox notification_id'),
    kind: requireString(row.kind, 'inbox kind'),
    title: typeof row.title === 'string' ? row.title : '',
    body: typeof row.body === 'string' ? row.body : '',
    createdAt: requireString(row.created_at, 'inbox created_at'),
    read: row.read === true,
    ...(typeof row.deep_link === 'string' && row.deep_link.trim() !== '' ? { deepLink: row.deep_link } : {}),
  });
  return {
    async inbox(cursor?: string): Promise<RemoteInboxPage> {
      const path = typeof cursor === 'string' && cursor.trim() !== ''
        ? `/api/v1/workbench/inbox?cursor=${encodeURIComponent(cursor.trim())}`
        : '/api/v1/workbench/inbox';
      const response = await request({ method: 'GET', path });
      if (typeof response !== 'object' || response === null || Array.isArray(response)) {
        throw new Error('inbox response must be a success envelope');
      }
      const envelope = response as { success?: unknown; data?: unknown };
      if (envelope.success !== true || !Object.prototype.hasOwnProperty.call(envelope, 'data')) {
        throw new Error('inbox response.success must be true with data');
      }
      const data = envelope.data as { items?: unknown; unread_count?: unknown; next_cursor?: unknown };
      if (!Array.isArray(data.items)) throw new Error('inbox data.items must be an array');
      if (typeof data.unread_count !== 'number' || !Number.isSafeInteger(data.unread_count) || data.unread_count < 0) {
        throw new Error('inbox data.unread_count must be a non-negative integer');
      }
      return {
        items: (data.items as Array<Record<string, unknown>>).map(mapItem),
        unreadCount: data.unread_count,
        ...(typeof data.next_cursor === 'string' && data.next_cursor !== '' ? { nextCursor: data.next_cursor } : {}),
      };
    },
    async markRead(notificationId: string): Promise<void> {
      const trimmed = typeof notificationId === 'string' ? notificationId.trim() : '';
      requireString(trimmed, 'notification id');
      const response = await request({ method: 'POST', path: '/api/v1/workbench/inbox/read', body: { notification_id: trimmed } });
      if (typeof response !== 'object' || response === null || Array.isArray(response)) {
        throw new Error('inbox markRead response must be a success envelope');
      }
      if ((response as { success?: unknown }).success !== true) {
        throw new Error('inbox markRead response.success must be true');
      }
    },
  };
}
```

在 `packages/api-client/package.json` 的 `exports` 中，Task 4 追加的 `"./mobile/devices"` 行之后追加：

```json
    "./mobile/inbox": "./src/mobile/inbox.ts",
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npx tsx --test packages/api-client/src/mobile/inbox.test.ts`
Expected: PASS（`# pass 3`，`# fail 0`）。

- [ ] **Step 5: Commit**

```bash
git add packages/api-client/src/mobile/inbox.ts packages/api-client/src/mobile/inbox.test.ts packages/api-client/package.json
git commit -m "feat(api-client/mobile): 行动通知 inbox wire Adapter（#41）"
```

---

### Task 6: apps/mobile 组装——/inbox 路由、深链安全导航与设备注册 fail-closed 入口

**Files:**
- Create: `apps/mobile/src/screens/InboxScreen.tsx`
- Create: `apps/mobile/src/app/inbox.tsx`
- Create: `apps/mobile/src/adapters/push-token.ts`
- Create: `apps/mobile/src/adapters/device-identity.ts`
- Modify: `apps/mobile/src/composition.ts`（文件末尾追加工厂与安全导航入口；`MobileApp` 内追加一次性设备注册 effect——均为追加，不改既有行）
- Modify: `apps/mobile/src/screens/HomeScreen.tsx:48`（"Open Resources" 按钮行（实测 48 行；45 行是 "Sign out"、46 行是 "View all tasks"）之后追加一行 "Open Inbox" 入口）
- Modify: `apps/mobile/package.json`（真机推送前置：dependencies 新增 `expo-notifications`——见下方「真机构建前置」）
- Test: `apps/mobile/src/app-smoke.test.tsx`（文件末尾追加 4 个测试）

**真机构建前置（blocked-env 边界，不阻塞 Node 测试链）：** `push-token.ts` 惰性 `require('expo-notifications')` 在 Node 测试环境与 app-smoke 下 fail closed（依赖缺失 → `'no-token'`，全部本地验证不依赖该包）。但 **Metro/Expo 打包在构建期解析模块图**：依赖表中没有 `expo-notifications` 时 `expo export` / EAS 真机构建会报 `Unable to resolve module`，try/catch 只救运行时救不了构建。因此真机目标必须先安装：

```bash
cd apps/mobile && npx expo install expo-notifications
```

（Expo 官方版本协商命令，自动匹配已装的 Expo SDK ~55.0.0；该步骤需要执行环境可访问 npm registry。）执行环境无法联网安装时：如实记录为环境前置（真机设备注册入口因此不可用、推送验收维持 blocked-env），**不得**为通过验证而伪造安装或跳过记录；本计划其余全部验证（Node 测试链、Go 回归、opt-in HTTP 集成）不受影响。

**Interfaces:**
- Consumes: 本计划 Task 1–5 全部产出；`MobileRuntime.authorizedRequest` / `scopeLease()` / `snapshot()`（#34/#32）；`activeMobileRuntime()`（`composition.ts:182`）；`router.push`（expo-router，app-smoke stub 已提供 `router: { replace() {}, push() {} }`）；`expo-secure-store` 的 `SecureStorePort`（`apps/mobile/src/adapters/secure-store.ts`——`getItemAsync/setItemAsync/deleteItemAsync`，与 vault/deployment Registry Adapter 同一 seam）；`Platform.OS`（react-native）。
- Produces（Task 7 与后续批次依赖）:
  - `notificationInboxFor(activeRuntime: MobileRuntime, origin: string): NotificationInbox`（composition 导出，按 origin 记忆化）
  - `deviceRegistryFor` 为模块内私有；对外 `registerActiveDeviceIfPossible(activeRuntime: Pick<MobileRuntime, 'snapshot' | 'authorizedRequest' | 'scopeLease'>, tokenSource?: { token(): Promise<string | undefined> }, identity?: { deviceId(): Promise<string | undefined> }): Promise<'registered' | 'no-token' | 'no-device-id' | 'unauthorized' | 'failed'>`
  - `openNotificationFromInbox(inbox: Pick<NotificationInbox, 'resolveTarget' | 'markRead'>, snapshot: Pick<RuntimeSnapshot, 'surface'>, item: Pick<InboxItem, 'notificationId'>, push: (path: string, params?: Record<string, string>) => void): Promise<'navigated' | 'blocked-unauthorized' | 'invalid-link'>`——**通知点击的唯一安全入口**：深链解析 → 重新鉴权检查（`surface === 'authorized'`，fail closed）→ 导航 `/tasks/detail`（权威 Task 同步由 #35 的 `TaskOffice.open` + snapshot 水合承担）→ 本地 markRead。
  - `createNativePushTokenIfAvailable(): { token(): Promise<string | undefined> }`（require `expo-notifications` 失败/无方法/取值失败 → undefined，fail closed——真机推送属 blocked-env）
  - `createNativeDeviceIdentity(): { deviceId(): Promise<string | undefined> }`（SecureStore 持久随机设备 id，key `weknora.device-id.v1`；解析失败 → undefined，fail closed）
  - `nativeDevicePlatform(): 'ios' | 'android'`（惰性 require `react-native` 的 `Platform.OS`，解析失败回落 `android`）

- [ ] **Step 1: Write the failing test**

在 `apps/mobile/src/app-smoke.test.tsx` 文件**末尾**追加（文件已有的 `test`/`assert`/`render`/`descendants`/`hooks` helper 直接复用，无新增顶层 import）：

```ts
test('the inbox route exists behind a default export', async () => {
  const inboxRoute = await import('./app/inbox.tsx');
  assert.equal(typeof inboxRoute.default, 'function', 'src/app/inbox.tsx must default-export the inbox route');
});

test('notification navigation re-authorizes, parses the safe deep link, and never performs business actions', async () => {
  const { openNotificationFromInbox } = await import('./composition.ts');

  const calls: string[] = [];
  const pushes: Array<{ path: string; params?: Record<string, string> }> = [];
  const push = (path: string, params?: Record<string, string>): void => { pushes.push({ path, params }); };
  const navigableInbox = {
    resolveTarget: () => ({ kind: 'task-detail' as const, taskId: 't-1', runId: 'r-1' }),
    markRead: async (id: string) => { calls.push(`markRead:${id}`); },
  };
  const invalidInbox = {
    resolveTarget: () => undefined,
    markRead: navigableInbox.markRead,
  };
  const authorized = { surface: 'authorized' as const };
  const unauthorized = { surface: 'deployment-login' as const };

  assert.equal(await openNotificationFromInbox(navigableInbox, authorized, { notificationId: 'n-1' }, push), 'navigated');
  assert.deepEqual(pushes, [{ path: '/tasks/detail', params: { taskId: 't-1', runId: 'r-1' } }]);
  assert.deepEqual(calls, ['markRead:n-1']);

  pushes.length = 0; calls.length = 0;
  assert.equal(await openNotificationFromInbox(navigableInbox, unauthorized, { notificationId: 'n-1' }, push), 'blocked-unauthorized');
  assert.deepEqual(pushes, [], 'an unauthorized surface must not navigate');
  assert.deepEqual(calls, [], 'an unauthorized surface must not mark read');

  assert.equal(await openNotificationFromInbox(invalidInbox, authorized, { notificationId: 'n-2' }, push), 'invalid-link');
  assert.deepEqual(pushes, [], 'a malformed deep link must not navigate');
  assert.deepEqual(calls, [], 'a malformed deep link must not mark read');
});

test('device registration is fail-closed without a native push token or device identity', async () => {
  const { registerActiveDeviceIfPossible } = await import('./composition.ts');
  const authorizedRuntime = {
    snapshot: () => ({ surface: 'authorized' as const, deployment: { origin: 'https://weknora.example.test', label: 'Test' } }),
    authorizedRequest: async () => { throw new Error('must not reach the wire without a token'); },
    scopeLease: () => undefined,
  };
  const unauthorizedRuntime = { snapshot: () => ({ surface: 'deployment-login' as const }) };

  assert.equal(await registerActiveDeviceIfPossible(authorizedRuntime as never, { token: async () => undefined }, { deviceId: async () => 'device-1' }), 'no-token');
  assert.equal(await registerActiveDeviceIfPossible(authorizedRuntime as never, { token: async () => 'tok' }, { deviceId: async () => undefined }), 'no-device-id');
  assert.equal(await registerActiveDeviceIfPossible(unauthorizedRuntime as never, { token: async () => 'tok' }, { deviceId: async () => 'device-1' }), 'unauthorized');
});

test('the home surface keeps a reachable inbox entry point and the inbox screen renders projections', async () => {
  const { HomeScreen } = await import('./screens/HomeScreen.tsx');
  hooks().__reset();
  const home = render(HomeScreen, {
    deploymentLabel: 'Test', tenants: [{ id: '7' }], activeTenantId: '7',
    onActivateTenant: () => {}, onSignOut: async () => {},
    taskOffice: { home: async () => ({ needsMe: [], running: [], recentlyCompleted: [], unreadNotifications: 0, asOf: '2026-09-24T00:00:00Z' }) },
  });
  assert.notEqual(
    descendants(home).find(({ type, props }) => type === 'Button' && props.title === 'Open Inbox'),
    undefined,
    'HomeScreen must keep an Open Inbox entry point',
  );

  const { InboxScreen } = await import('./screens/InboxScreen.tsx');
  hooks().__reset();
  const opened: string[] = [];
  const screen = render(InboxScreen, {
    view: {
      items: [
        { notificationId: 'n-1', kind: 'attention', title: '需要你处理', body: '', createdAt: '2026-09-24T01:00:00Z', read: false, deepLink: 'weknora://tasks/detail?taskId=t-1&runId=r-1' },
        { notificationId: 'n-2', kind: 'budget', title: '预算事件', body: '', createdAt: '2026-09-24T02:00:00Z', read: true },
      ],
      unreadCount: 1,
      duplicateNotificationIds: [],
    },
    loading: false,
    onRefresh: () => {}, onLoadMore: () => {},
    onOpenNotification: (item: { notificationId: string }) => { opened.push(item.notificationId); },
  });
  const texts = descendants(screen).filter(({ type }) => type === 'Text').map(({ props }) => String(props.children));
  assert.ok(texts.some((text) => text.includes('未读 1')), 'unread count is visible');
  const row = descendants(screen).find(({ type, props }) => type === 'Button' && String(props.title).includes('需要你处理'));
  (row!.props.onPress as () => void)();
  assert.deepEqual(opened, ['n-1'], 'tapping a row hands the item to the composition-owned navigation seam');
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL —— 新增 4 个用例中 `import('./app/inbox.tsx')`、`InboxScreen`、`openNotificationFromInbox`、`registerActiveDeviceIfPossible` 均 `Cannot find module`；既有 72 项仍绿。

- [ ] **Step 3: Write minimal implementation**

创建 `apps/mobile/src/adapters/push-token.ts`：

```ts
/**
 * 原生推送 token 适配器（module-seams §12：APNs/FCM 属 true external——Push Port +
 * scripted Adapter，真机验收单独进行）。Node 测试环境与无推送凭据的设备上
 * require('expo-notifications') 失败或取不到 token：返回 undefined，fail closed
 * （不注册设备，不阻塞登录/授权主流程）。
 */
export interface NativePushTokenSource {
  token(): Promise<string | undefined>;
}

export function createNativePushTokenIfAvailable(): NativePushTokenSource {
  return {
    async token(): Promise<string | undefined> {
      try {
        const notifications = require('expo-notifications') as {
          getDevicePushTokenAsync?: () => Promise<{ data?: unknown }>;
        };
        if (typeof notifications.getDevicePushTokenAsync !== 'function') return undefined;
        const push = await notifications.getDevicePushTokenAsync();
        return typeof push?.data === 'string' && push.data.trim() !== '' ? push.data : undefined;
      } catch {
        return undefined;
      }
    },
  };
}
```

创建 `apps/mobile/src/adapters/device-identity.ts`：

```ts
import type { SecureStorePort } from './secure-store.ts';

const DEVICE_ID_KEY = 'weknora.device-id.v1';

/** 稳定设备身份：SecureStore 持久的随机 id（首次生成后复用）。设备身份按 Deployment 隔离由服务端 (tenant, owner, device) 唯一键承担（ADR-0007）。 */
export interface NativeDeviceIdentity {
  deviceId(): Promise<string | undefined>;
}

export function createSecureDeviceIdentity(store: SecureStorePort): NativeDeviceIdentity {
  return {
    async deviceId(): Promise<string | undefined> {
      try {
        const existing = await store.getItemAsync(DEVICE_ID_KEY);
        if (typeof existing === 'string' && existing.trim() !== '') return existing.trim();
        const generated = globalThis.crypto?.randomUUID
          ? globalThis.crypto.randomUUID()
          : `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
        await store.setItemAsync(DEVICE_ID_KEY, generated);
        return generated;
      } catch {
        return undefined; // fail closed：无安全存储即无设备身份，不注册
      }
    },
  };
}

export function createNativeDeviceIdentity(): NativeDeviceIdentity {
  try {
    const secure = require('expo-secure-store') as SecureStorePort;
    return createSecureDeviceIdentity(secure);
  } catch {
    return { deviceId: async () => undefined };
  }
}

/** 原生平台（服务端仅接受 ios/android 枚举，internal/handler/mobile_device.go:195）。
 * 惰性 require：Node 测试 stub 环境解析失败时保守回落 android。不得在模块顶层具名
 * import react-native 的 Platform（app-smoke stub 未导出它）。 */
export function nativeDevicePlatform(): 'ios' | 'android' {
  try {
    const reactNative = require('react-native') as { Platform?: { OS?: string } };
    return reactNative.Platform?.OS === 'ios' ? 'ios' : 'android';
  } catch {
    return 'android';
  }
}
```

创建 `apps/mobile/src/screens/InboxScreen.tsx`：

```tsx
import { Button, ScrollView, Text, View } from 'react-native';
import type { InboxItem, InboxView } from '@weknora/mobile-core';

export interface InboxScreenProps {
  view?: InboxView;
  loading: boolean;
  error?: string;
  notice?: string;
  onRefresh(): void;
  onLoadMore?(): void;
  onOpenNotification(item: InboxItem): void;
}

/**
 * 行动通知收件箱屏（#41）：只消费 Inbox 投影与回调（module-seams §10——Screen 不导入
 * api-client、不维护 cursor/未读状态）。深链安全裁决在 composition 的
 * openNotificationFromInbox（解析 + 重新鉴权 + 导航），屏内不解析深链。
 */
export function InboxScreen({ view, loading, error, notice, onRefresh, onLoadMore, onOpenNotification }: InboxScreenProps) {
  if (view === undefined && error !== undefined) {
    return (
      <View>
        <Text>无法读取行动通知</Text>
        <Text>{error}</Text>
        <Button title="重试" onPress={onRefresh} />
      </View>
    );
  }
  return (
    <ScrollView>
      <Text>{`未读 ${view?.unreadCount ?? 0}`}</Text>
      {notice !== undefined && <Text>{notice}</Text>}
      {loading && <Text>正在同步…</Text>}
      {(view?.items ?? []).map((item) => (
        <Button
          key={item.notificationId}
          title={`${item.kind} · ${item.title === '' ? item.notificationId : item.title}${item.read ? '' : ' · 未读'}`}
          onPress={() => { onOpenNotification(item); }}
        />
      ))}
      {view !== undefined && view.items.length === 0 && !loading && <Text>暂无行动通知</Text>}
      {view?.nextCursor !== undefined && onLoadMore !== undefined
        ? <Button title="Load more" onPress={onLoadMore} />
        : null}
      <Button title="刷新" onPress={onRefresh} />
    </ScrollView>
  );
}
```

创建 `apps/mobile/src/app/inbox.tsx`：

```tsx
import { useEffect, useState } from 'react';
import { Text, View } from 'react-native';
import { router } from 'expo-router';
import type { InboxItem, InboxView, NotificationInbox } from '@weknora/mobile-core';
import { activeMobileRuntime, notificationInboxFor, openNotificationFromInbox } from '../composition.ts';
import { InboxScreen } from '../screens/InboxScreen.tsx';

interface InboxRouteState {
  view?: InboxView;
  loading: boolean;
  error?: string;
  notice?: string;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

/** /inbox 挂载生命周期宿主：subscribe 持续接收投影，page() 首次加载与手动刷新共用（与 /resources 同一模式）。 */
export function InboxRouteLifecycle({ inbox }: { inbox: NotificationInbox }) {
  const activeRuntime = activeMobileRuntime();
  const [state, setState] = useState<InboxRouteState>({ loading: true });
  useEffect(() => {
    let alive = true;
    const unsubscribe = inbox.subscribe((view) => {
      if (alive) setState({ view, loading: false, error: undefined });
    });
    void inbox.page().then(
      () => undefined,
      (failure) => { if (alive) setState((current) => ({ ...current, loading: false, error: errorMessage(failure) })); },
    );
    return () => {
      alive = false;
      unsubscribe();
    };
  }, [inbox]);
  const refresh = (): void => {
    setState((current) => ({ ...current, loading: true, error: undefined }));
    void inbox.page().then(
      () => undefined,
      (failure) => { setState((current) => ({ ...current, loading: false, error: errorMessage(failure) })); },
    );
  };
  const loadMore = state.view?.nextCursor !== undefined
    ? (): void => {
        void inbox.more().then(
          () => undefined,
          (failure) => { setState((current) => ({ ...current, error: errorMessage(failure) })); },
        );
      }
    : undefined;
  const openNotification = (item: InboxItem): void => {
    void (async () => {
      const outcome = await openNotificationFromInbox(inbox, activeRuntime.snapshot(), item, (path, params) => {
        router.push({ pathname: path, params });
      });
      setState((current) => ({
        ...current,
        notice: outcome === 'invalid-link'
          ? '该通知的链接无法安全打开。'
          : outcome === 'blocked-unauthorized'
            ? '请先登录并激活空间，再打开该任务。'
            : undefined,
      }));
    })();
  };
  return (
    <InboxScreen
      view={state.view}
      loading={state.loading}
      error={state.error}
      notice={state.notice}
      onRefresh={refresh}
      {...(loadMore === undefined ? {} : { onLoadMore: loadMore })}
      onOpenNotification={openNotification}
    />
  );
}

/** Expo Router 文件路由：/inbox。只消费行动通知 Inbox Interface；未授权面给出登录引导。 */
export default function InboxRoute() {
  const activeRuntime = activeMobileRuntime();
  const snapshot = activeRuntime.snapshot();
  const origin = snapshot.deployment?.origin;
  if (snapshot.surface !== 'authorized' || origin === undefined) {
    return (
      <View>
        <Text>请先登录并激活空间，再查看行动通知。</Text>
      </View>
    );
  }
  return <InboxRouteLifecycle inbox={notificationInboxFor(activeRuntime, origin)} />;
}
```

在 `apps/mobile/src/composition.ts`：
（a）顶部 import 区（`createNativeSecureDeploymentRegistry` import 之后）追加：

```ts
import { createDeviceRegistry, createNotificationInbox, type DeviceRegistry, type InboxItem, type NotificationInbox } from '@weknora/mobile-core';
import { createMobileDeviceRemote } from '@weknora/api-client/mobile/devices';
import { createMobileInboxRemote } from '@weknora/api-client/mobile/inbox';
import { createNativePushTokenIfAvailable } from './adapters/push-token.ts';
import { createNativeDeviceIdentity, nativeDevicePlatform } from './adapters/device-identity.ts';
```

（注意：**不得**在 composition 顶层 `import { Platform } from 'react-native'`——app-smoke 的 react-native CJS stub 只导出 View/Text/TextInput/Button/ScrollView，Node ESM 具名导入缺失导出会直接抛错并炸掉既有 72 项测试。平台判定经 `nativeDevicePlatform()` 惰性 require。）

（b）`taskOfficeFor` 函数（composition.ts:109-124）之后追加：

```ts
const deviceRegistries = new Map<string, DeviceRegistry>();

/** Device Registry 按 deployment origin 记忆化（同 taskOfficeFor 模式）：授权读通道与
 * scope lease 全部经 Runtime——设备注册的 token 不出 Runtime，注册身份按 Deployment 隔离（ADR-0007）。 */
function deviceRegistryFor(activeRuntime: Pick<MobileRuntime, 'authorizedRequest' | 'scopeLease'>, origin: string): DeviceRegistry {
  let registry = deviceRegistries.get(origin);
  if (!registry) {
    registry = createDeviceRegistry({
      remote: createMobileDeviceRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input) }),
      lease: () => activeRuntime.scopeLease(),
    });
    deviceRegistries.set(origin, registry);
  }
  return registry;
}

const notificationInboxes = new Map<string, NotificationInbox>();

/** 行动通知 Inbox 按 deployment origin 记忆化（同 taskOfficeFor 模式）。 */
export function notificationInboxFor(activeRuntime: MobileRuntime, origin: string): NotificationInbox {
  let inbox = notificationInboxes.get(origin);
  if (!inbox) {
    inbox = createNotificationInbox({
      remote: createMobileInboxRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input) }),
      lease: () => activeRuntime.scopeLease(),
    });
    notificationInboxes.set(origin, inbox);
  }
  return inbox;
}

/** 通知点击的唯一安全入口（#41 AC1/AC2）：安全深链解析 → 重新鉴权检查（fail closed）→
 * 导航 /tasks/detail（权威 Task 同步由 #35 的 TaskOffice.open + snapshot 水合承担）→
 * 本地 markRead（已读是投影，不是业务操作）。绝不执行通知描述的业务操作。 */
export async function openNotificationFromInbox(
  inbox: Pick<NotificationInbox, 'resolveTarget' | 'markRead'>,
  snapshot: Pick<RuntimeSnapshot, 'surface'>,
  item: Pick<InboxItem, 'notificationId'>,
  push: (path: string, params?: Record<string, string>) => void,
): Promise<'navigated' | 'blocked-unauthorized' | 'invalid-link'> {
  const target = inbox.resolveTarget(item as InboxItem);
  if (target === undefined) return 'invalid-link';
  if (snapshot.surface !== 'authorized') return 'blocked-unauthorized';
  push('/tasks/detail', { taskId: target.taskId, runId: target.runId });
  await inbox.markRead(item.notificationId);
  return 'navigated';
}

/** 设备注册入口（spec §4「注册设备与 App 前后台生命周期」）：无原生 push token 或无安全
 * 设备身份时 fail closed 跳过；注册失败不阻塞授权主流程（best effort）。 */
export async function registerActiveDeviceIfPossible(
  activeRuntime: Pick<MobileRuntime, 'snapshot' | 'authorizedRequest' | 'scopeLease'>,
  tokenSource: { token(): Promise<string | undefined> } = createNativePushTokenIfAvailable(),
  identity: { deviceId(): Promise<string | undefined> } = createNativeDeviceIdentity(),
): Promise<'registered' | 'no-token' | 'no-device-id' | 'unauthorized' | 'failed'> {
  const snapshot = activeRuntime.snapshot();
  const origin = snapshot.deployment?.origin;
  if (snapshot.surface !== 'authorized' || origin === undefined) return 'unauthorized';
  const token = await tokenSource.token();
  if (token === undefined) return 'no-token';
  const deviceId = await identity.deviceId();
  if (deviceId === undefined) return 'no-device-id';
  try {
    await deviceRegistryFor(activeRuntime, origin).register({
      deviceId,
      token,
      platform: nativeDevicePlatform(),
    });
    return 'registered';
  } catch {
    return 'failed';
  }
}
```

（c）`MobileApp` 组件（composition.ts:209-229）内，在 `bootRuntimeOnce` effect 之后追加一次性设备注册 effect（`useRef` 状态紧跟既有 `const booted = useRef(false);` 声明之后新增 `const registeredFor = useRef(new Set<string>());`）：

```ts
  // T11（#41）：进入授权面后 best-effort 注册设备（每个 origin 一次；无 push token 时 fail closed 静默跳过）
  useEffect(() => {
    const unsubscribe = activeRuntime.subscribe((next) => {
      if (next.surface !== 'authorized' || !next.deployment) return;
      if (registeredFor.current.has(next.deployment.origin)) return;
      registeredFor.current.add(next.deployment.origin);
      void registerActiveDeviceIfPossible(activeRuntime).catch(() => undefined);
    });
    return unsubscribe;
  }, [activeRuntime]);
```

在 `apps/mobile/src/screens/HomeScreen.tsx` 的 "Open Resources" 按钮（实测 HomeScreen.tsx:48）之后追加一行：

```tsx
      {/* T11（#41）：行动通知收件箱入口。 */}
      <Button title="Open Inbox" onPress={() => router.push('/inbox')} />
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: PASS（既有 72 项 + 新增 4 项 = `# pass 76`，`# fail 0`；typecheck 干净——`MobileDeviceRegistration`/`RemoteInboxPage` 对 mobile-core 端口的结构可赋值在此实证）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/screens/InboxScreen.tsx apps/mobile/src/app/inbox.tsx apps/mobile/src/adapters/push-token.ts apps/mobile/src/adapters/device-identity.ts apps/mobile/src/composition.ts apps/mobile/src/screens/HomeScreen.tsx apps/mobile/src/app-smoke.test.tsx
git commit -m "feat(mobile): /inbox 收件箱屏、通知深链安全导航与设备注册 fail-closed 入口（#41）"
```

---

### Task 7: 端到端集成证据（opt-in 真实 HTTP）与计划级验证

**Files:**
- Create: `apps/mobile/src/device-inbox-integration-smoke.ts`
- Test: `apps/mobile/src/device-inbox-integration-smoke.test.ts`

**Interfaces:**
- Consumes: `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD` opt-in 语义与 `disallowedDeploymentHost(hostname, variable)`（`apps/mobile/src/runtime-integration-smoke.ts:118`）；`createMobileRuntime` / `createInMemoryCredentialStore` / `createMobileRuntimeRemote`（#32 已核实）；本计划 Task 2–5 全部产出；`CLIENT_PROTOCOL_VERSION`（`@weknora/domain/mobile`）。
- Produces: `DeviceInboxIntegrationConfig`、`DeviceInboxIntegrationEvidence`、`deviceInboxIntegrationConfig(env: Record<string, string | undefined>): DeviceInboxIntegrationConfig`、`runDeviceInboxIntegration(config: Extract<DeviceInboxIntegrationConfig, { enabled: true }>): Promise<DeviceInboxIntegrationEvidence>`、`emitDeviceInboxIntegrationEvidence(evidence, emit)`。证据只含：deployment origin、各步骤枚举结果、未读计数、观察到的 deep_link 计数、命令时间戳——**不含任何凭据/token/bearer 字段**。

- [ ] **Step 1: Write the failing test**

创建 `apps/mobile/src/device-inbox-integration-smoke.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  deviceInboxIntegrationConfig,
  emitDeviceInboxIntegrationEvidence,
  runDeviceInboxIntegration,
} from './device-inbox-integration-smoke.ts';

/**
 * Opt-in 真实 HTTP 检查（#41 AC3——最高稳定 Interface：真实 JSON transport + 真实 wire
 * Adapter + 真实 Mobile Runtime + 真实服务端两步注册/接管/撤销/inbox/markRead）。无回退凭据：
 *
 * WEKNORA_MOBILE_TEST_DEPLOYMENT_URL=https://deployment.example \
 * WEKNORA_MOBILE_TEST_EMAIL=mobile-test@example.test \
 * WEKNORA_MOBILE_TEST_PASSWORD=short-lived-secret \
 * pnpm --filter @weknora/mobile test
 */
test('real HTTP device registration, takeover, revocation and inbox read complete through the wire', async (t) => {
  const config = deviceInboxIntegrationConfig(process.env);
  if (!config.enabled) {
    if (config.disposition === 'skip') t.skip(`DEVICE_INBOX_HTTP_SKIPPED: ${config.reason}`);
    else assert.fail(`DEVICE_INBOX_HTTP_INVALID: ${config.reason}`);
    return;
  }

  const evidence = await runDeviceInboxIntegration(config);
  emitDeviceInboxIntegrationEvidence(evidence, (record) => t.diagnostic(record));

  assert.equal(evidence.deviceRegister, 'registered', 'two-step intent→register must bind the device');
  assert.equal(evidence.deviceTokenTakeover, 'rotated', 're-registering must take over the token');
  assert.equal(evidence.deviceRevoke, 'revoked', 'revocation must succeed');
  assert.equal(evidence.deviceRevokeAgain, 'not-found', 'revoking a revoked device must surface DEVICE_NOT_FOUND');
  assert.equal(evidence.inboxRead, 'read', 'the inbox read model must be readable');
  assert.equal(evidence.markReadIdempotent, 'ok', 'markRead on an unknown id must stay idempotent success (no business action)');
});

test('device-inbox config marks missing credentials skippable and malformed hosts invalid', () => {
  assert.deepEqual(
    deviceInboxIntegrationConfig({}),
    { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' },
  );
  const withCredentials = {
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://deployment.example',
    WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: 'short-lived-secret',
  };
  for (const url of ['https://127.0.0.1', 'https://10.0.0.2', 'https://localhost', 'https://deployment.example/api/v1']) {
    const config = deviceInboxIntegrationConfig({ ...withCredentials, WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: url });
    assert.equal(config.enabled, false, `${url} must not enable a real HTTP run`);
    assert.equal(config.disposition, 'invalid', `${url} is invalid, not skippable`);
  }
  const fallback = deviceInboxIntegrationConfig(withCredentials);
  assert.equal(fallback.enabled, true);
  if (fallback.enabled) assert.equal(fallback.deviceToken, 'integration-placeholder-token', 'placeholder token is not a real push credential');
});

test('emitted evidence carries no credential fields (Review Focus #3)', () => {
  const emitted: string[] = [];
  emitDeviceInboxIntegrationEvidence({
    deploymentOrigin: 'https://deployment.example',
    deviceRegister: 'registered',
    deviceTokenTakeover: 'rotated',
    deviceRevoke: 'revoked',
    deviceRevokeAgain: 'not-found',
    inboxRead: 'read',
    inboxUnreadCount: 0,
    markReadIdempotent: 'ok',
    deepLinksObserved: 0,
    commandTimestamp: '2026-09-24T00:00:00.000Z',
  }, (record) => emitted.push(record));
  assert.equal(emitted.length, 1);
  // 匹配的是凭据值与形态（占位 token、账号、密码样例、Bearer 头），而不是泛型字段名词——
  // 证据的合法字段名 deviceTokenTakeover 本身含 'deviceToken' 子串，正则不得误伤自身契约。
  assert.doesNotMatch(emitted[0]!, /short-lived-secret|mobile-test@|password|Bearer\s|placeholder-token/i, 'evidence must stay credential-free');
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx tsx --test apps/mobile/src/device-inbox-integration-smoke.test.ts`
Expected: FAIL —— `Cannot find module .../device-inbox-integration-smoke.ts`（ERR_MODULE_NOT_FOUND）。

- [ ] **Step 3: Write minimal implementation**

创建 `apps/mobile/src/device-inbox-integration-smoke.ts`：

```ts
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileDeviceRemote } from '@weknora/api-client/mobile/devices';
import { createMobileInboxRemote } from '@weknora/api-client/mobile/inbox';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import {
  createDeviceRegistry, createInMemoryCredentialStore, createMobileRuntime, createNotificationInbox,
  DeviceError,
} from '@weknora/mobile-core';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';

export type DeviceInboxIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string; deviceToken: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface DeviceInboxIntegrationEvidence {
  deploymentOrigin: string;
  deviceRegister: 'registered' | 'failed';
  deviceTokenTakeover: 'rotated' | 'failed';
  deviceRevoke: 'revoked' | 'failed';
  deviceRevokeAgain: 'not-found' | 'unexpected';
  inboxRead: 'read' | 'failed';
  inboxUnreadCount?: number;
  markReadIdempotent: 'ok' | 'failed';
  deepLinksObserved: number;
  commandTimestamp: string;
}

/** 与 T04/T05 冒烟相同的 opt-in 语义（自包含，不跨计划 import 配置解析）。 */
export function deviceInboxIntegrationConfig(env: Record<string, string | undefined>): DeviceInboxIntegrationConfig {
  const deploymentOrigin = env.WEKNORA_MOBILE_TEST_DEPLOYMENT_URL?.trim();
  const email = env.WEKNORA_MOBILE_TEST_EMAIL?.trim();
  const password = env.WEKNORA_MOBILE_TEST_PASSWORD;
  if (!deploymentOrigin || !email || !password) {
    return { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' };
  }
  let parsed: URL;
  try {
    parsed = new URL(deploymentOrigin);
  } catch {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL is not an absolute URL' };
  }
  if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
  }
  const hostRejection = disallowedDeploymentHost(parsed.hostname, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL');
  if (hostRejection) return { enabled: false, disposition: 'invalid', reason: hostRejection };
  // 占位 token 不是可用凭据（无 APNs/FCM 真实效力）；可用真实 token 经变量覆盖。
  const deviceToken = env.WEKNORA_MOBILE_TEST_DEVICE_TOKEN?.trim() || 'integration-placeholder-token';
  return { enabled: true, deploymentOrigin: parsed.origin, email, password, deviceToken };
}

/**
 * 真实 JSON transport + 具体 Remote Adapter + Mobile Runtime + 服务端两步注册/接管/撤销/
 * inbox 读/markRead（#41 AC3）。证据契约不含任何凭据字段；任何步骤异常只记枚举失败，从不 reject。
 */
export async function runDeviceInboxIntegration(config: Extract<DeviceInboxIntegrationConfig, { enabled: true }>): Promise<DeviceInboxIntegrationEvidence> {
  const evidence: DeviceInboxIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    deviceRegister: 'failed',
    deviceTokenTakeover: 'failed',
    deviceRevoke: 'failed',
    deviceRevokeAgain: 'unexpected',
    inboxRead: 'failed',
    markReadIdempotent: 'failed',
    deepLinksObserved: 0,
    commandTimestamp: new Date().toISOString(),
  };
  const fetcher: FetchLike = (input, init) => fetch(input, init as RequestInit);
  const runtime = createMobileRuntime({
    credentialStore: createInMemoryCredentialStore(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    remoteFor(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return createMobileRuntimeRemote({ origin, request: client.request });
    },
    // 授权通道（对照 composition.ts:62-65 既有写法）：devices/inbox remote 全走
    // runtime.authorizedRequest——无此 port 时 authorizedRequest 直接抛
    // 'RUNTIME_UNAUTHORIZED'（mobile-runtime.ts:383-388，name='Error' 非 ApiError），
    // 证据会全部退化为 failed，因此本冒烟必须显式装配。
    authorizedTransport(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return (input, accessToken) => client.request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
    },
  });
  const throughRuntime = (input: { method: string; path: string; headers?: Record<string, string>; body?: unknown }): Promise<unknown> =>
    runtime.authorizedRequest(input);
  const deviceId = 'integration-smoke-device';
  const registry = createDeviceRegistry({
    remote: createMobileDeviceRemote({ origin: config.deploymentOrigin, request: throughRuntime }),
    lease: () => runtime.scopeLease(),
  });
  const inbox = createNotificationInbox({
    remote: createMobileInboxRemote({ origin: config.deploymentOrigin, request: throughRuntime }),
    lease: () => runtime.scopeLease(),
  });

  const snapshot = await runtime.signIn({
    deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' },
    email: config.email,
    password: config.password,
  });
  if (snapshot.surface !== 'authorized') return evidence; // 未授权：证据保持 failed 枚举，如实记录

  try {
    await registry.register({ deviceId, token: `${config.deviceToken}#1`, platform: 'ios' });
    evidence.deviceRegister = 'registered';
  } catch {
    return evidence;
  }
  try {
    await registry.register({ deviceId, token: `${config.deviceToken}#2`, platform: 'ios' }); // token 接管
    evidence.deviceTokenTakeover = 'rotated';
  } catch {
    return evidence;
  }
  try {
    await registry.revoke({ deviceId });
    evidence.deviceRevoke = 'revoked';
  } catch {
    return evidence;
  }
  try {
    await registry.revoke({ deviceId }); // 撤销后再撤销：必须 404 → DEVICE_NOT_FOUND
    evidence.deviceRevokeAgain = 'unexpected';
  } catch (error) {
    evidence.deviceRevokeAgain = error instanceof DeviceError && error.code === 'DEVICE_NOT_FOUND' ? 'not-found' : 'unexpected';
  }
  try {
    const view = await inbox.page();
    evidence.inboxRead = 'read';
    evidence.inboxUnreadCount = view.unreadCount;
    evidence.deepLinksObserved = view.items.filter((item) => item.deepLink !== undefined && item.deepLink !== '').length;
  } catch {
    return evidence;
  }
  try {
    // 服务端对不存在 id 幂等 success 且不执行任何业务操作（workbench_inbox.go:123-130）
    await inbox.markRead('integration-smoke-nonexistent');
    evidence.markReadIdempotent = 'ok';
  } catch {
    /* 保持 failed：如实记录该部署的语义差异 */
  }
  return evidence;
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitDeviceInboxIntegrationEvidence(evidence: DeviceInboxIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
```

- [ ] **Step 4: Run test to verify it passes（含计划级回归验证）**

Run: `pnpm --filter @weknora/mobile test`
Expected: PASS（无环境变量时真实 HTTP 用例 `t.skip`，`# skipped` ≥ 1；其余两项 config/证据契约用例绿）。

计划级全量定向验证（在 worktree 根执行，覆盖本计划全部新增测试 + 既有相关回归）：

```bash
npx tsx --test packages/mobile-core/src/inbox/deep-link.test.ts packages/mobile-core/src/device/device-registry.test.ts packages/mobile-core/src/inbox/notification-inbox.test.ts packages/api-client/src/mobile/devices.test.ts packages/api-client/src/mobile/inbox.test.ts && pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck && go test ./internal/application/repository/ -run 'TestMobileDevice|TestNotificationOutboxDeduplicates' -count=1 && go test ./internal/handler/session/ -run 'TestMX021' -count=1 && go test ./internal/handler/ -run 'TestMobileDeviceHandler' -count=1
```

Expected: 全部 PASS/ok——`TestMobileDevice*`（14 项，撤销 owner-scoped/租户环境隔离/token 接管/重放/生命周期围栏）+ `TestNotificationOutboxDeduplicates`（投递侧重复通知去重）+ `TestMX021*`（inbox 跨账户隔离）+ `TestMobileDeviceHandler*`（handler 级鉴权与 epoch）构成服务端半边的回归证据。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/device-inbox-integration-smoke.ts apps/mobile/src/device-inbox-integration-smoke.test.ts
git commit -m "test(mobile): 设备注册与行动通知端到端集成证据（opt-in 真实 HTTP）（#41）"
```

---

## 附：验收标准 → 证据映射（收尾自查表）

| 验收标准 | 客户端证据（本计划新增） | 服务端证据（既有，回归命令守护） |
|---|---|---|
| AC1 推送只作同步 hint，不直接改变业务状态 | Task 3：`applyHint` 只重投影、hint.kind 不入视图；`markRead` 只置已读且调用集可观测为零业务调用；Task 6：`openNotificationFromInbox` 导航前重新鉴权、点击不执行业务操作 | `notification_delivery.go:111`（PushPayload Title/Body 仅 Intent.Kind）；`workbench_inbox.go:123-130`（MarkRead 单条 Update） |
| AC2 设备撤销、错误 deep link 和重复通知均有验证 | Task 1：18 类畸形 deep link 全拒；Task 2：撤销 404→DEVICE_NOT_FOUND、scope 围栏、409 有界重试；Task 3：notificationId 去重可观测 | 14 项 `TestMobileDevice*`（撤销 owner-scoped、epoch 围栏、重放拒绝）；`TestNotificationOutboxDeduplicates` |
| AC3 端到端最高稳定 Interface | Task 7：真实 JSON transport + 真实 wire Adapter + 真实 Runtime + 服务端两步注册/接管/撤销/inbox/markRead（opt-in，无凭据 t.skip，不伪造） | 同左（同一真实服务端路径） |
| blocked-env：真机 APNs/FCM 送达与冷启动落位 | 本地替代：Task 2/3 Interface 级场景 + Task 7 opt-in HTTP + Go 回归；`createNativePushTokenIfAvailable` 在无凭据环境 fail closed | 真机验收门槛单独执行（spec Testing Decisions："real-device acceptance separately"） |
