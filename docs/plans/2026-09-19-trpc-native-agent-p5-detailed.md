# tRPC Native Agent P5 — 版本化协议与全部客户端 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 使所有现有端使用同一可靠业务协议。

**Architecture:** 先固定 Go/TS wire、SSE、API client 和纯领域状态机，再分端适配。保留 UI 与原生交互，不将 SDK 内部事件发给客户端。

**Tech Stack:** Go 1.26.0；当前根 SDK 候选 v1.11.0（尚非获批完整产品组合）；SQLite/PostgreSQL 与 P1.0 确认的其他活跃方言；现有 React/TypeScript、Flutter/Expo 和 Go 客户端。

**Spec:** [已确认规格](../specs/2026-09-19-trpc-native-agent-migration-design.md)、[原总计划](2026-09-19-trpc-native-agent-migration.md)、[P0](2026-09-19-trpc-native-agent-p0.md)、[完整业务接口草案](trpc-native/interfaces.md)。

## Global Constraints

- “首版保留现有全部业务功能；验收后通过维护窗口一次切换。”
- “旧会话不恢复执行，也不隐式导入新上下文。”
- “归档不等于删除；未批准任何历史数据、附件或审计记录的清除。”
- “适配器只处理业务边界或原生组件缺失能力，每个适配器记录原因、覆盖测试和可删除条件。”
- “缺少凭据、服务或设备的项目标记 `blocked-env`，不得计为通过；必需项存在阻塞或失败时不得上线。”
- 不拆分独立 Agent 服务；不双执行真实副作用。身份与权限来自服务端；秘密仅保存引用。
- P0 NO-GO 仍有效。此文件是受前置门槛约束的详细实施方案，不是已选定数据库/SDK 组合的声明；门槛未关闭时只执行证据、契约和隔离探针任务。
- 每项代码任务按 RED → GREEN → 回归 → 需求/质量审查 → 修复复核 → 限定文件提交；不得将计划内代码示例当作已执行证据。
- 并行执行最多 4 Track；一 Track 一 branch/worktree，一 worktree 最多一 implementer；同一文件不得由两个 agent 修改。
- 详细依赖、共享文件队列、完整测试、证据格式见 [总索引与 DAG](2026-09-19-trpc-native-agent-p1-p9-detailed.md)。

## Review Focus

1. 大整数精度：在下面任务的 RED 场景和集成验收中分别验证。
2. 游标缺口：在下面任务的 RED 场景和集成验收中分别验证。
3. 旧 attempt 晚到：在下面任务的 RED 场景和集成验收中分别验证。
4. 空间切换旧流：在下面任务的 RED 场景和集成验收中分别验证。
5. 审批重复点击/旧客户端重试：在下面任务的 RED 场景和集成验收中分别验证。

---

## 文件职责和阅读顺序

本文件列出的新增 native 文件是目标设计，不声称已经存在。旧入口只作迁移参考，所有新增测试由本阶段实施时创建。精确的跨阶段业务类型来自 interfaces.md §3，P1.1 冻结为 `nativecontract`；使用前必须读取完整定义。代码块给出最小规则、SQL 或验收命令，不构成已经编译的产品实现。存储构造器/SDK 装配必须使用 P1.0 的版本与方法证据；未获证据时保持 blocked-design，不能自行猜测 API。

| 任务 | 前置 | 文件职责 |
| --- | --- | --- |
| P5.1 冻结 Go/TS wire 与跨语言 fixture | P1.1, P1.5 | `packages/contracts/src/chat/native.ts`<br>`packages/contracts/test/native-agent.test.ts`<br>`internal/agent/nativecontract/wire_test.go`<br>`tests/native-agent/wire-v1.json`<br>`packages/contracts/package.json` |
| P5.2 可靠 HTTP/SSE 与命令入口 | P5.1, P3.4, P2.6 | `internal/handler/session/native_run.go`<br>`internal/handler/session/native_stream.go`<br>`internal/handler/session/native_run_test.go`<br>`internal/handler/session/native_stream_test.go`<br>`internal/router/routes_native_agent.go` |
| P5.3 API client 与纯领域 reducer | P5.1 | `packages/api-client/src/chat/native.ts`<br>`packages/api-client/src/chat/native.test.ts`<br>`packages/domain/src/chat/native.ts`<br>`packages/domain/src/chat/native.test.ts`<br>`packages/api-client/package.json`<br>`packages/domain/package.json` |
| P5.4 Web 交互完整适配 | P5.2, P5.3 | `apps/web/src/chat/native-controller.ts`<br>`apps/web/src/chat/native-controller.test.ts`<br>`apps/web/src/chat/ChatRoutePage.tsx`<br>`apps/web/src/chat/native-agent.test.tsx` |
| P5.5 Desktop 与 Embed 平台适配 | P5.4 | `apps/desktop/src/native-agent.test.ts`<br>`apps/embed/src/native-agent.ts`<br>`apps/embed/src/native-agent.test.ts`<br>`apps/embed/src/main.tsx` |
| P5.6 Flutter 与 Expo 移动端分别迁移 | P5.2, P5.3 | `apps/mobile/lib/core/services/native_agent_service.dart`<br>`apps/mobile/test/native_agent_service_test.dart`<br>`apps/mobile/lib/features/chat/providers/chat_providers.dart`<br>`apps/mobile-next/src/features/conversations/chat/nativeAgent.ts`<br>`apps/mobile-next/src/features/conversations/chat/nativeAgent.test.ts`<br>`apps/mobile-next/src/features/conversations/chat/ChatService.ts` |
| P5.7 Go client/CLI、小程序与 DSH 消费者 | P5.2, P5.3 | `client/native_agent.go`<br>`client/native_agent_test.go`<br>`cli/native_agent.go`<br>`apps/miniprogram/src/services/native-agent.ts`<br>`apps/miniprogram/tests/native-agent.test.mjs`<br>`packages/dsh-weknora/src/native-agent.ts`<br>`packages/dsh-weknora/test/native-agent.test.mjs` |

### Task P5.1: 冻结 Go/TS wire 与跨语言 fixture

**Depends on:** P1.1, P1.5。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `packages/contracts/src/chat/native.ts` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `packages/contracts/test/native-agent.test.ts` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/nativecontract/wire_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `tests/native-agent/wire-v1.json` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `packages/contracts/package.json` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 完整映射 interfaces.md §6；protocol=weknora.agent.v1，schema_version=1；tenant_id/seq/usage 大整数以字符串传输；事件 kinds/payload、pending 详情、archive 及所有命令错误均入 fixture。

- [ ] **Step 1：先写失败测试。** 超 JS 安全整数、未知 version、非法 seq、缺失 RunID、未知事件、OAuth detail 脱敏；Go 生成与 TS 校验同 fixture。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
pnpm exec tsx --test packages/contracts/test/native-agent.test.ts
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { parseSequence } from '../src/chat/native.ts';
test('preserves sequence above safe integer', () => {
  assert.equal(parseSequence('9007199254740993'), 9007199254740993n);
  assert.throws(() => parseSequence('-1'));
  assert.throws(() => parseSequence('1.1'));
});
export function parseSequence(value: string): bigint {
  if (!/^[1-9][0-9]*$/.test(value)) throw new Error('invalid_sequence');
  const n = BigInt(value);
  if (n > 9223372036854775807n) throw new Error('invalid_sequence');
  return n;
}
```

共享接口只由本任务修改；从完整 Go 类型逐字段定义 TS discriminated union，运行时校验不只 TS cast。审批/用量/错误不得简化成任意 JSON。fixture 包含成功、拒绝、waiting、attempt replacement、cursor expired、归档；package exports 明确发布 native 子路径，后续 Track 不抢改同一导出。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。Go/TS 双向 fixture 一致；root typecheck:shared 若未覆盖新文件，由总集成人补入口。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- packages/contracts/src/chat/native.ts packages/contracts/test/native-agent.test.ts internal/agent/nativecontract/wire_test.go tests/native-agent/wire-v1.json packages/contracts/package.json
git commit -m "feat: implement p5.1 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P5.2: 可靠 HTTP/SSE 与命令入口

**Depends on:** P5.1, P3.4, P2.6。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/handler/session/native_run.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/handler/session/native_stream.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/handler/session/native_run_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/handler/session/native_stream_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/router/routes_native_agent.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 新增 /api/v1/native-agent/sessions、/runs、/runs/:id/events、/cancel、/steer、/pending、/pending/:pendingID/resolve；认证中间件使用现有实现。Last-Event-ID=v1:<base64url(runID)>:<seq>。

- [ ] **Step 1：先写失败测试。** 持久化失败不发送成功；重复/超前/过期/其他 run cursor；重连时撤权；断开流后 Run 继续；旧协议返回 client_upgrade_required。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/handler/session ./internal/router -run 'NativeRun|NativeStream|NativeRoute' -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
// SSE 响应每条必须使用已提交的 seq，不能在 handler 自增。
// id: v1:cnVuLTE:7
// event: weknora.agent.v1
// data: {"protocol":"weknora.agent.v1","schema_version":1,"event_id":"event-7","tenant_id":"1","session_id":"session-1","run_id":"run-1","seq":"7","kind":"run.status","payload":{"status":"running"}}
// 缺口响应在写 SSE header 前返回 409 + code=cursor_expired + snapshot URL。
```

读前检查范围，循环读事件日志再等待通知；通知丢失通过日志追赶。游标严格解析并验证 run，一次 response 不混空间。写超时结束连接不触发 Cancel；仅显式命令取消。路由注册进入串行共享队列，release switch 才改变默认入口；缺失 native gate 时返回执行门关闭。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。httptest 真实 SSE 字节与断流重连顺序验证；不得只测试 encoder。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/handler/session/native_run.go internal/handler/session/native_stream.go internal/handler/session/native_run_test.go internal/handler/session/native_stream_test.go internal/router/routes_native_agent.go
git commit -m "feat: implement p5.2 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P5.3: API client 与纯领域 reducer

**Depends on:** P5.1。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `packages/api-client/src/chat/native.ts` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `packages/api-client/src/chat/native.test.ts` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `packages/domain/src/chat/native.ts` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `packages/domain/src/chat/native.test.ts` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `packages/api-client/package.json` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `packages/domain/package.json` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 新增 native API methods 与 reduceNativeEvent(state,event)；state 含 scope/run/cursor/activeAttempt/text/tool/pending/terminal，类型显式定义在 domain/native.ts。API 传输只走已有 transport。

- [ ] **Step 1：先写失败测试。** 重复 seq 忽略；跳号要求 resync；旧 attempt 晚到不污染新文本；切空间后旧响应丢弃；审批双击同 command ID；未知 wire version 提示升级。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
pnpm exec tsx --test packages/api-client/src/chat/native.test.ts packages/domain/src/chat/native.test.ts
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
export function shouldApplySequence(last: bigint, incoming: bigint): 'apply'|'duplicate'|'resync' {
  if (incoming <= last) return 'duplicate';
  return incoming === last + 1n ? 'apply' : 'resync';
}
// native.test.ts 使用 node:test 与 strict assert
assert.equal(shouldApplySequence(7n, 7n), 'duplicate');
assert.equal(shouldApplySequence(7n, 9n), 'resync');
assert.equal(shouldApplySequence(7n, 8n), 'apply');
```

独立 attempt 文本 buffer，replacement 事件使旧 attempt 失活；旧 attempt 事件仍前进日志 cursor，但不拼接内容。reducer 无 React/DOM，无副作用。API 重连保留已确认游标，409 重取权威 snapshot 后恢复；写请求仅使用服务端幂等规则重试。AbortController 生命周期绑定 scope，不能把用户登出后的凭据继续用于重连。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。与 P5.2 集成 SSE 验证替换/缺口/撤权；域测试通过后解锁端实现。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- packages/api-client/src/chat/native.ts packages/api-client/src/chat/native.test.ts packages/domain/src/chat/native.ts packages/domain/src/chat/native.test.ts packages/api-client/package.json packages/domain/package.json
git commit -m "feat: implement p5.3 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P5.4: Web 交互完整适配

**Depends on:** P5.2, P5.3。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `apps/web/src/chat/native-controller.ts` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `apps/web/src/chat/native-controller.test.ts` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `apps/web/src/chat/ChatRoutePage.tsx` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `apps/web/src/chat/native-agent.test.tsx` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** Web 通过 native API 与 domain reducer，保留 ChatRoutePage 当前布局、附件、模型/Agent 选择、引用、工具和审批。共享视图改动若有必要由本 Track 唯一占有并列出精确文件。

- [ ] **Step 1：先写失败测试。** 聊天完整状态；重复审批、OAuth 返回、工具结果未知；切空间/退出/卸载；attempt 替换不闪回；键盘、窄屏和长文案；归档入口只读。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
pnpm test:web
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
pnpm typecheck:web
pnpm test:web
pnpm build:web
# 浏览器验收：登录两个空间，发消息，工具审批，断网重连，取消，切空间；
# 对照后端 Run/Event/Usage，保存视频或截图与事件序列。
```

页面仅绑定新 controller，不直接解析 SDK event；为 waiting 类型呈现已有交互与明确原因，禁止把未知结果显示为成功。取消按钮与连接状态分离，审批提交中去重。沿用 i18n 和 UI 包；不附带重构路由/主题/框架。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。真实后端浏览器证据包含发送→工具→审批→完成、断流、撤权、归档；mock UI tests 不替代。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- apps/web/src/chat/native-controller.ts apps/web/src/chat/native-controller.test.ts apps/web/src/chat/ChatRoutePage.tsx apps/web/src/chat/native-agent.test.tsx
git commit -m "feat: implement p5.4 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P5.5: Desktop 与 Embed 平台适配

**Depends on:** P5.4。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `apps/desktop/src/native-agent.test.ts` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `apps/embed/src/native-agent.ts` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `apps/embed/src/native-agent.test.ts` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `apps/embed/src/main.tsx` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** Desktop 复用 Web renderer，不复制 ChatRoutePage；Embed 消费 api-client/domain，宿主通信保留当前 origin/principal 限制。

- [ ] **Step 1：先写失败测试。** Wails 运行时流读取/取消、外链/OAuth 返回；Embed 两 visitor 同会话字符串不串读；非法宿主 origin 拒绝；宿主销毁解绑订阅。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
pnpm test:desktop && pnpm test:embed
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
pnpm typecheck:desktop
pnpm test:desktop
pnpm build:desktop-renderer
pnpm typecheck:embed
pnpm test:embed
pnpm build:embed
```

仅修平台 transport/生命周期差异；Desktop Web renderer 有修改由 P5.4 负责。Embed native-agent.ts 管作用域和宿主消息，不能从 parent postMessage 任意信任 tenant/principal；后端认证判定权限。记录 Wails 原生运行与真实嵌入页操作。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。renderer build 不算 Wails 验收；Embed SDK 与实际页面均跑审批、重连、取消。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- apps/desktop/src/native-agent.test.ts apps/embed/src/native-agent.ts apps/embed/src/native-agent.test.ts apps/embed/src/main.tsx
git commit -m "feat: implement p5.5 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P5.6: Flutter 与 Expo 移动端分别迁移

**Depends on:** P5.2, P5.3。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `apps/mobile/lib/core/services/native_agent_service.dart` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `apps/mobile/test/native_agent_service_test.dart` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `apps/mobile/lib/features/chat/providers/chat_providers.dart` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `apps/mobile-next/src/features/conversations/chat/nativeAgent.ts` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `apps/mobile-next/src/features/conversations/chat/nativeAgent.test.ts` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `apps/mobile-next/src/features/conversations/chat/ChatService.ts` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** Flutter 使用 wire fixture 独立 Dart DTO/状态机；Expo 使用框架无关共享契约或同 fixture，保持独立依赖安装方式。此任务执行时拆为 P5.6F/P5.6E 两 Track，文件集合天然不重叠。

- [ ] **Step 1：先写失败测试。** 后台→前台游标追赶、弱网重复事件、OAuth deep link、通知打开 waiting 详情、取消与附件；Dart int/JS bigint wire 一致；切账户丢弃旧流。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
git diff --check
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
# cwd=apps/mobile
flutter analyze
flutter test test/native_agent_service_test.dart
flutter test
# cwd=apps/mobile-next
npm run typecheck
npm test -- --runInBand
npm run check:isolation
```

两端保留原生交互，不导入 DOM UI。Flutter 在 service/provider 交界接入，Expo 在 ChatService/nativeAgent 交界接入；原生 secure storage 与 token 更新使用既有实现。先用共享 fixture 校验 reducer，再真实 API，再 iOS/Android 原生编译和设备启动。root pnpm 的 mobile 命令不能代替 Flutter。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。两个移动端分别记录类型/单元/编译/设备，不用 export 代替设备；设备缺失 blocked-env。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- apps/mobile/lib/core/services/native_agent_service.dart apps/mobile/test/native_agent_service_test.dart apps/mobile/lib/features/chat/providers/chat_providers.dart apps/mobile-next/src/features/conversations/chat/nativeAgent.ts apps/mobile-next/src/features/conversations/chat/nativeAgent.test.ts apps/mobile-next/src/features/conversations/chat/ChatService.ts
git commit -m "feat: implement p5.6 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P5.7: Go client/CLI、小程序与 DSH 消费者

**Depends on:** P5.2, P5.3。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `client/native_agent.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `client/native_agent_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `cli/native_agent.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `apps/miniprogram/src/services/native-agent.ts` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `apps/miniprogram/tests/native-agent.test.mjs` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `packages/dsh-weknora/src/native-agent.ts` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `packages/dsh-weknora/test/native-agent.test.mjs` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 共享 wire v1；三个子 Track P5.7G/P5.7M/P5.7D 各占自己的路径。CLI 复用 Go client，DSH 复用其现有鉴权/配置；小程序继续平台 transport。

- [ ] **Step 1：先写失败测试。** CLI Ctrl-C/显式 cancel 区别、SSE 重连；小程序前后台/网络切换；DSH 工具问答与审批错误不吞掉；旧客户端收到升级提示。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
git diff --check
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
# cwd=client（独立 Go module）
GOWORK=off go test ./... -count=1
# cwd=cli（独立 Go module）
GOWORK=off go test ./... -count=1
# cwd=仓库根
pnpm --filter @weknora/miniprogram test
pnpm --filter @weknora/miniprogram typecheck
pnpm --filter @weknora/miniprogram build:weapp
# cwd=packages/dsh-weknora
node --test test/native-agent.test.mjs
```

先增加新的 native protocol client，再把既有 Agent 使用入口切换到新 client；非 Agent 知识 API 保留。小程序页面对新增服务的接线文件由 M 子 Track 列入 manifest 后独占；DSH package exports/构建由 D 独占。P0 消费者清单发现其他端必须追加任务，不能视为已涵盖。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。CLI 实际二进制、小程序开发者工具/设备、DSH 实际 harness 各跑正常和错误链；不自动发布 npm 包。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- client/native_agent.go client/native_agent_test.go cli/native_agent.go apps/miniprogram/src/services/native-agent.ts apps/miniprogram/tests/native-agent.test.mjs packages/dsh-weknora/src/native-agent.ts packages/dsh-weknora/test/native-agent.test.mjs
git commit -m "feat: implement p5.7 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

## 阶段完成门槛

- 所有任务完成 scoped tests 和独立需求/质量审查；阻塞项原样记录。
- 合入集成分支后跑本阶段与上游消费方回归，不能把 worktree 单独 PASS 当作集成 PASS。
- 功能清单每条有新入口、具体测试与证据；没有删减原功能来换取完成。
- 当前文件是计划交付，所有实施任务初始 pending/blocked-design；没有声称本轮执行了这些测试或实现。
