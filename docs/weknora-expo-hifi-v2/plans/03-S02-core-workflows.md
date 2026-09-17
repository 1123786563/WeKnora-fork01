# S02 · 核心页面与任务闭环 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: 使用superpowers:subagent-driven-development或executing-plans逐任务推进，独立审查与证据后方可解锁依赖。

**Goal:** 交付核心页面与任务闭环的可审查工程增量。

**Architecture:** 沿WeKnora既有权限、执行与商业边界增量接线，移动端通过纯TS包和NativeHost消费产品能力，不以原型模拟代替真实服务。

**Tech Stack:** 现有Go/Gin/GORM、Expo/React Native、TypeScript/pnpm；先校准锁文件，不默认升级全部依赖。

**Spec:** [详细设计](../docs/01-detailed-design.md)、[页面规格](../docs/02-screen-specifications.md)、[设计系统](../docs/03-design-system.md)、[API分类](../docs/04-api-contracts.md)、[总计划](00-implementation-master.md)。

## Global Constraints

继续现有apps/mobile与WeKnora产品控制面；不新增权威mobile_tasks、第二套审批或钱包。保留原W/T/H依赖和验收状态。字段/路由/代码路径必须对照当前checkout；本计划新模块为建议增量，不承诺已经存在。一次用户意图一个持久request_id；未知启动先对账。取消、执行停止、费用结算分别表达。工具、预算与连接授权分别处理。静态、单元、数据库、原生和真实服务证据不混记；blocked-env不能记为通过。仅修改已取得锁的文件；不得擅自push、merge、发布或触发Provider真实写入。


## 本册执行规则

下方测试代码是**未来工程验收规范**，不是本次已经运行的项目测试。每任务的probe文件由该任务建立，调用实际模块/挂载组件/数据库/设备或解析真实命令输出，返回observed数据；严禁简单返回expected常量让测试通过。缺环境throw明确blocked-env并保留日志，不用SKIP当通过。初始测试模块缺失不是有效行为RED；建立最小接缝后必须看到真实断言失败，再修复实现。已有正确行为直接回归，不故意破坏制造RED。

## MX-013 · 工作台聚合读模型与首页

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S02 / pending（尚未执行仓库任务） |
| 角色 / Track | 共享 TS / Go / shared |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/013` |
| 直接依赖 | MX-008、MX-011 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W03, W11；UX：UX02 |
| 关闭缺口 | G12 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-013.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `internal/handler/session/workbench_overview.go`
- `internal/application/service/workbench/overview.go`
- `packages/api-client/src/mobile/overview.ts`
- `apps/mobile/sources/weknora/screens/HomeScreen.tsx`
- `tests/mobile-v2/mx-013.test.ts`
- `tests/mobile-v2/probes/mx-013.ts`


### Interfaces

Consumes：Scope 与当前资源授权；提议 OverviewDTO。  
Produces：M03 + ownership-scoped 聚合响应。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string }
export interface Observation { visibleRunIds: string[]; perSessionHTTPRequests: number }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-013.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-013.ts';

test('overview-owner-scope', async () => {
  const observed = await runProbe({
  "fixture": "two-users-one-tenant"
});
  assert.deepEqual(observed, {
  "visibleRunIds": [
    "owned-run"
  ],
  "perSessionHTTPRequests": 0
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-013.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 优先复用现有查询，无等价接口再加 B 类 overview。
- [ ] 服务端授权后计算计数、进行中、待处理与成果；附 as_of。
- [ ] 上限限制及批量读取，避免前端逐会话请求。
- [ ] UI 实现首页层次，分页与错误/空态、上次同步可见。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-013.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 计数与列表过滤一致，无跨成员私密会话泄漏。
- [ ] 并发状态改变后 as_of/快照边界可解释。
- [ ] 空空间不显示另一空间演示/缓存内容。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-013.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

## MX-014 · 会话列表、搜索与分页

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S02 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/014` |
| 直接依赖 | MX-013 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W10, W11；UX：UX02 |
| 关闭缺口 | 页面/产品交付要求 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-014.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/screens/SessionListScreen.tsx`
- `packages/domain/src/mobile/session-list.ts`
- `tests/mobile-v2/mx-014.test.ts`
- `tests/mobile-v2/probes/mx-014.ts`


### Interfaces

Consumes：授权会话分页接口；stable cursor。  
Produces：M04 搜索/筛选/分页列表。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string; fault: string }
export interface Observation { duplicateIds: string[]; appliedFilter: string }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-014.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-014.ts';

test('pagination-filter-scope', async () => {
  const observed = await runProbe({
  "fixture": "equal-updated-at-two-pages",
  "fault": "filter-change-late-response"
});
  assert.deepEqual(observed, {
  "duplicateIds": [],
  "appliedFilter": "waiting_user"
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-014.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 搜索去抖仅控制读请求；每次捕获 scope generation。
- [ ] 以 updated_at+id 稳定分页；筛选条件纳入 query key。
- [ ] 虚拟列表使用 stable id，滚动与加载状态分离。
- [ ] 切空间清分页 cursor；空搜索不反复查询全库。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-014.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 跨页无重复，更新时间相同仍能稳定游标。
- [ ] 搜索无结果与空空间文案不同。
- [ ] 旧搜索结果到达时不覆盖新词或新空间。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-014.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

## MX-015 · 新建任务、资源编排与提交

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S02 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/015` |
| 直接依赖 | MX-006、MX-012、MX-016 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W04, W06, W10；UX：UX03 |
| 关闭缺口 | G04 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-015.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/screens/NewTaskScreen.tsx`
- `apps/mobile/sources/weknora/workbench/task-form.ts`
- `tests/mobile-v2/mx-015.test.ts`
- `tests/mobile-v2/probes/mx-015.ts`


### Interfaces

Consumes：AgentOption；已准备的 session/resources；SubmissionController。  
Produces：M05 + 持久 request_id + ACK→Run 导航。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string }
export interface Observation { startCount: number; draft: string }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-015.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-015.ts';

test('submit-attachment-not-ready', async () => {
  const observed = await runProbe({
  "fixture": "valid-draft-scanning-attachment"
});
  assert.deepEqual(observed, {
  "startCount": 0,
  "draft": "保留这段文本"
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-015.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 字段绑定草稿；校验文本、预算安全整数与资源权限。
- [ ] 按真实 API 准备 session/资源配置，不把附件字段擅加 StartInput。
- [ ] 先保存原请求再提交；unknown 导向原请求核实页。
- [ ] 取消输入保留可恢复草稿，离线不自动发送。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-015.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 输入空白/预算小数/负数/溢出不发请求。
- [ ] 附件未扫描完成不能交给 Agent。
- [ ] 双击或 ACK 丢失恢复无重复任务。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-015.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

## MX-016 · Agent 目录与可用能力选择

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S02 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/016` |
| 直接依赖 | MX-008、MX-011 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W11；UX：UX03 |
| 关闭缺口 | 页面/产品交付要求 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-016.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/screens/AgentPickerScreen.tsx`
- `packages/domain/src/mobile/agent-options.ts`
- `tests/mobile-v2/mx-016.test.ts`
- `tests/mobile-v2/probes/mx-016.ts`


### Interfaces

Consumes：服务端可见 Agent 和能力三态。  
Produces：M06 Agent 选择与目标约束。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string }
export interface Observation { selectedAgent: string; unavailableReason: string }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-016.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-016.ts';

test('unavailable-agent', async () => {
  const observed = await runProbe({
  "fixture": "coding-driver-unavailable"
});
  assert.deepEqual(observed, {
  "selectedAgent": "general",
  "unavailableReason": "driver_unavailable"
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-016.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 显示名称/能力摘要而非敏感 Prompt 配置。
- [ ] 按类型与关键词筛选；选中后返回原表单。
- [ ] 不可用能力显示原因，不允许隐藏后默认 fallback。
- [ ] 编码 Agent 依赖实际已授权 target，不以名字启用 shell。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-016.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 无权 Agent 不出现在列表。
- [ ] 缺能力目标给出可解释原因且不能提交。
- [ ] 切换 Agent 不丢任务文字。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-016.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

## MX-017 · 产品对话渲染与富消息

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S02 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/017` |
| 直接依赖 | MX-004、MX-009、MX-012 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W09, W10, W12；UX：UX04 |
| 关闭缺口 | G07, G08 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-017.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/conversations/ConversationScreen.tsx`
- `apps/mobile/sources/weknora/conversations/view-model.ts`
- `apps/mobile/sources/weknora/conversations/ProductMessageList.tsx`
- `apps/mobile/sources/weknora/conversations/MessageBlock.tsx`
- `tests/mobile-v2/mx-017.test.ts`
- `tests/mobile-v2/probes/mx-017.ts`


### Interfaces

Consumes：ExecutionSnapshot/Event；消息投影；NativeHost。  
Produces：M07 + ProductConversationVM；原生引用/工具/成果卡。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string }
export interface Observation { renderedMessageCount: number; text: string; happySyncReads: number }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-017.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-017.ts';

test('duplicate-message-event', async () => {
  const observed = await runProbe({
  "fixture": "same-message-seq42-twice"
});
  assert.deepEqual(observed, {
  "renderedMessageCount": 1,
  "text": "你好",
  "happySyncReads": 0
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-017.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 拆出产品 renderer，不再依赖 Happy useSession/useIsDataReady。
- [ ] messages、pendingInteractions、三状态与能力同源投影。
- [ ] 稳定 message ID，分段 append 更新，用户上滑不抢滚动。
- [ ] 输入端只有有类型 submit/steer/附件/语音事件，不调用任意 RPC。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-017.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 无 Happy 登录/sync 数据仍可发起与展示产品会话。
- [ ] 重复事件不重复文本；tool/result 不被拼入普通 assistant 字符串。
- [ ] 空态、长消息、键盘、撤权和恢复均能渲染。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-017.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

## MX-018 · 执行详情与保守取消展示

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S02 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/018` |
| 直接依赖 | MX-006、MX-017 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W03, W12；UX：UX04 |
| 关闭缺口 | G08 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-018.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/screens/ExecutionScreen.tsx`
- `packages/domain/src/mobile/execution-presentation.ts`
- `tests/mobile-v2/mx-018.test.ts`
- `tests/mobile-v2/probes/mx-018.ts`


### Interfaces

Consumes：ExecutionDTO 三状态；capabilities；revision。  
Produces：M08 状态投影与请求/取消处理。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string }
export interface Observation { executionStatus: string; label: string; refundRequested: boolean }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-018.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-018.ts';

test('cancel-not-stop', async () => {
  const observed = await runProbe({
  "fixture": "cancel-ack-no-process-exit"
});
  assert.deepEqual(observed, {
  "executionStatus": "stop_pending",
  "label": "停止待确认",
  "refundRequested": false
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-018.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 保留 run/execution/settlement 三个原值与观察时间。
- [ ] unknown 专用 lookup；无 revision 不发送 command。
- [ ] 取消前确认；ACK 后显示 stop_pending，等待真实确认。
- [ ] 终态结果可查看但结算仍可 pending；禁止显示自动退款。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-018.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 取消按钮不立即变为停止已确认。
- [ ] unknown 页不会创建新请求 ID。
- [ ] unknown 枚举可显示原始状态但禁控制命令。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-018.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

## MX-019 · 工具审批详情与二次确认

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S02 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/019` |
| 直接依赖 | MX-005、MX-011、MX-017 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W05, W06；UX：UX05 |
| 关闭缺口 | G02 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-019.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/screens/InteractionScreen.tsx`
- `apps/mobile/sources/weknora/interactions/ToolApproval.tsx`
- `apps/mobile/sources/weknora/interactions/controller.ts`
- `tests/mobile-v2/mx-019.test.ts`
- `tests/mobile-v2/probes/mx-019.ts`


### Interfaces

Consumes：Current Interaction；typed decide adapter。  
Produces：M09 + 冻结目标/内容/revision 确认 Sheet。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string; fault: string }
export interface Observation { decisionCount: number; nextAction: string }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-019.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-019.ts';

test('approval-frozen-revision', async () => {
  const observed = await runProbe({
  "fixture": "open-revision4-current5",
  "fault": "stale-confirm"
});
  assert.deepEqual(observed, {
  "decisionCount": 0,
  "nextAction": "refresh"
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-019.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 显示连接账号、精确目标、内容摘要、风险、有效期与版本。
- [ ] 打开确认前刷新当前详情，Sheet 保存 revision/digest/scope generation。
- [ ] 确认提交后等服务端状态，客户端不宣告外部写入已成功。
- [ ] 409/过期/撤权关闭旧确认并提示重新核对。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-019.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 确认前内容改变不接受旧批准。
- [ ] 并发批准/拒绝没有第二次外部写入。
- [ ] 返回/Escape/Android Back 不提交决定。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-019.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

## MX-020 · 预算、问题与连接授权交互

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S02 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/020` |
| 直接依赖 | MX-019 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W05；UX：UX05 |
| 关闭缺口 | 页面/产品交付要求 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-020.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/interactions/BudgetApproval.tsx`
- `apps/mobile/sources/weknora/interactions/QuestionForm.tsx`
- `apps/mobile/sources/weknora/interactions/ConnectionConsent.tsx`
- `tests/mobile-v2/mx-020.test.ts`
- `tests/mobile-v2/probes/mx-020.ts`


### Interfaces

Consumes：Interaction discriminated union；授权角色。  
Produces：三类专用交互页面/Sheet，不混用批准布尔值。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string }
export interface Observation { toolBudgetFieldCount: number; questionAnswer: string; connectionState: string }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-020.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-020.ts';

test('typed-interaction-fields', async () => {
  const observed = await runProbe({
  "fixture": "budget-question-connection-tool"
});
  assert.deepEqual(observed, {
  "toolBudgetFieldCount": 0,
  "questionAnswer": "用户编辑后的回答",
  "connectionState": "authorizing"
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-020.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 预算显示旧上限、新申请与差额，校验 budget_authorize 权限。
- [ ] 问题回复校验服务端 options/schema 与长度；提交最新编辑。
- [ ] 连接授权使用受信系统浏览器，并查询产品连接状态。
- [ ] 未知交互类型只读展示；不 fall back 至批准任意操作。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-020.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 工具批准不能增加预算，预算批准不能授权连接。
- [ ] 问题内容更新后发送最新值。
- [ ] 授权回跳状态不合法不激活连接。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-020.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

## MX-021 · 通知注册、收件箱与安全深链

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S02 / pending（尚未执行仓库任务） |
| 角色 / Track | 共享 TS / Go / shared |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/021` |
| 直接依赖 | MX-013、MX-019 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W13, W14, W15, W16；UX：UX07 |
| 关闭缺口 | 页面/产品交付要求 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-021.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/notifications/registration.ts`
- `apps/mobile/sources/weknora/notifications/deep-link.ts`
- `apps/mobile/sources/weknora/screens/InboxScreen.tsx`
- `internal/handler/session/workbench_inbox.go`
- `tests/mobile-v2/mx-021.test.ts`
- `tests/mobile-v2/probes/mx-021.ts`


### Interfaces

Consumes：事件 outbox；设备注册；Scope/AuthState。  
Produces：M10 + InboxDTO + 冷启动导航意图。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string; fault: string }
export interface Observation { deliveredToB: number; approvalCount: number }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-021.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-021.ts';

test('notification-cross-account', async () => {
  const observed = await runProbe({
  "fixture": "device-A-logout-B-login",
  "fault": "late-A-intent"
});
  assert.deepEqual(observed, {
  "deliveredToB": 0,
  "approvalCount": 0
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-021.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 设备 environment/user/session/revision 隔离与撤销。
- [ ] 事件提交与通知意图同事务，回执失败重试不制造新任务。
- [ ] 深链只携资源标识；先认证、空间确认、重查权限。
- [ ] 拒绝通知权限不影响任务；通知只提示不携审批正文。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-021.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] A账号退出后B不收到A的敏感内容。
- [ ] 跨空间通知点击先确认，不直接执行决定。
- [ ] 冷启动/删除资源/撤权/重复推送均正确。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-021.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

