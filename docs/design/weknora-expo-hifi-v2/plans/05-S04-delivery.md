# S04 · 质量与发布验收 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: 使用superpowers:subagent-driven-development或executing-plans逐任务推进，独立审查与证据后方可解锁依赖。

**Goal:** 交付质量与发布验收的可审查工程增量。

**Architecture:** 沿WeKnora既有权限、执行与商业边界增量接线，移动端通过纯TS包和NativeHost消费产品能力，不以原型模拟代替真实服务。

**Tech Stack:** 现有Go/Gin/GORM、Expo/React Native、TypeScript/pnpm；先校准锁文件，不默认升级全部依赖。

**Spec:** [详细设计](../docs/01-detailed-design.md)、[页面规格](../docs/02-screen-specifications.md)、[设计系统](../docs/03-design-system.md)、[API分类](../docs/04-api-contracts.md)、[总计划](00-implementation-master.md)。

## Global Constraints

继续现有apps/mobile与WeKnora产品控制面；不新增权威mobile_tasks、第二套审批或钱包。保留原W/T/H依赖和验收状态。字段/路由/代码路径必须对照当前checkout；本计划新模块为建议增量，不承诺已经存在。一次用户意图一个持久request_id；未知启动先对账。取消、执行停止、费用结算分别表达。工具、预算与连接授权分别处理。静态、单元、数据库、原生和真实服务证据不混记；blocked-env不能记为通过。仅修改已取得锁的文件；不得擅自push、merge、发布或触发Provider真实写入。


## 本册执行规则

下方测试代码是**未来工程验收规范**，不是本次已经运行的项目测试。每任务的probe文件由该任务建立，调用实际模块/挂载组件/数据库/设备或解析真实命令输出，返回observed数据；严禁简单返回expected常量让测试通过。缺环境throw明确blocked-env并保留日志，不用SKIP当通过。初始测试模块缺失不是有效行为RED；建立最小接缝后必须看到真实断言失败，再修复实现。已有正确行为直接回归，不故意破坏制造RED。

## MX-032 · 服务端能力清单与部署开关

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S04 / pending（尚未执行仓库任务） |
| 角色 / Track | Go 后端 / server |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/032` |
| 直接依赖 | MX-013、MX-019、MX-021 |
| 条件依赖 | resources: MX-022,MX-024,MX-025；connectors: MX-023；remote: MX-026,MX-027；dictation: MX-028；voice: MX-029 |
| 原计划对应 | W：W34；UX：UX10 |
| 关闭缺口 | G11 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-032.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `internal/application/service/workbench/capabilities.go`
- `deploy/mobile-workbench/profile-manifest.json`
- `docs/evidence/mobile-v2/profile-gates.md`
- `tests/mobile-v2/mx-032.test.ts`
- `tests/mobile-v2/probes/mx-032.ts`


### Interfaces

Consumes：已验收 provider/driver/权限/商业 subgate。  
Produces：profile → capability state/reason；最低协议窗口。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string }
export interface Observation { remoteCapability: string; reason: string }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-032.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-032.ts';

test('profile-with-missing-evidence', async () => {
  const observed = await runProbe({
  "fixture": "remote-without-cancel-evidence"
});
  assert.deepEqual(observed, {
  "remoteCapability": "unavailable",
  "reason": "missing_cancel_evidence"
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-032.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] capability 取部署、服务健康、用户权限与验收证据交集。
- [ ] core/oidc/resources/connectors/remote/personal_node/dictation/voice/full_happy 分开。
- [ ] 能力撤销后刷新并强制服务端拒绝新准入。
- [ ] 配置只隐藏未发布能力，不将按钮置灰当完成功能。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-032.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 任一缺证据能力不能显示 supported。
- [ ] 旧任务仍可停止/清理/结算，不因关新准入而丢失。
- [ ] 旧客户端兼容性以版本协商测试证明。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-032.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：隔离 PostgreSQL + SQLite。

## MX-033 · 恢复、并发与安全故障注入

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S04 / pending（尚未执行仓库任务） |
| 角色 / Track | QA / Release / quality |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/033` |
| 直接依赖 | MX-006、MX-012、MX-018、MX-019、MX-032 |
| 条件依赖 | resources: MX-022,MX-024,MX-025；connectors: MX-023；remote: MX-026,MX-027；dictation: MX-028；voice: MX-029 |
| 原计划对应 | W：W33, W35；UX：UX10 |
| 关闭缺口 | G01, G04, G06, G09 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-033.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `tests/mobile-v2/recovery.test.ts`
- `tests/mobile-v2/security.test.ts`
- `docs/evidence/mobile-v2/fault-matrix.md`
- `tests/mobile-v2/mx-033.test.ts`
- `tests/mobile-v2/probes/mx-033.ts`


### Interfaces

Consumes：当前集成 HEAD；真实受控环境。  
Produces：故障矩阵证据与各 profile 独立结论。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string; fault: string }
export interface Observation { providerWriteCount: number; settlementCount: number }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-033.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-033.ts';

test('crash-recovery-budget', async () => {
  const observed = await runProbe({
  "fixture": "side-effect-succeeded",
  "fault": "crash-before-local-settlement"
});
  assert.deepEqual(observed, {
  "providerWriteCount": 1,
  "settlementCount": 1
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-033.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 注入 ACK 丢失、事件乱序、游标过期、DB写失败与进程重启。
- [ ] 并发批准/撤权/换空间/迟到回调验证写入前边界。
- [ ] 删除墓碑保留清理与迟到用量，不移除必要结算记录。
- [ ] remote/voice 对应环境缺失保留 blocked，不用 mock 标通过。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-033.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 至少逐项覆盖本交付状态表的所有负向行为。
- [ ] 无重复外部执行、重复账务与越权可见数据。
- [ ] 日志不含 Prompt、token、完整文件内容。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-033.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

## MX-034 · 双平台产品链 E2E

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S04 / pending（尚未执行仓库任务） |
| 角色 / Track | QA / Release / quality |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/034` |
| 直接依赖 | MX-015、MX-017、MX-018、MX-019、MX-021、MX-030、MX-031、MX-033 |
| 条件依赖 | resources: MX-022,MX-024,MX-025；connectors: MX-023；remote: MX-026,MX-027；dictation: MX-028；voice: MX-029 |
| 原计划对应 | W：W12, W16, W36；UX：UX10 |
| 关闭缺口 | G10 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-034.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `tests/mobile-v2/maestro/core.yaml`
- `tests/mobile-v2/maestro/scope.yaml`
- `docs/evidence/mobile-v2/native-e2e.md`
- `tests/mobile-v2/mx-034.test.ts`
- `tests/mobile-v2/probes/mx-034.ts`


### Interfaces

Consumes：iOS/Android 开发构建 + 当前后端。  
Produces：真实登录→任务→审批→成果→重启恢复证据。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string }
export interface Observation { verifiedPlatforms: string[]; duplicateRunCount: number }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-034.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-034.ts';

test('native-core-e2e', async () => {
  const observed = await runProbe({
  "fixture": "controlled-platform-agent-ios-android"
});
  assert.deepEqual(observed, {
  "verifiedPlatforms": [
    "ios",
    "android"
  ],
  "duplicateRunCount": 0
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-034.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 真实产品账户与平台 Agent 走完核心链。
- [ ] 冷启动通知/前后台/杀进程/权限拒绝分别记录。
- [ ] 记录 build、OS、后端 SHA、数据库与服务来源。
- [ ] 浏览器原型截图不替代 Native E2E。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-034.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 两个原生平台同一核心场景通过。
- [ ] 没有 silent skip/no tests found。
- [ ] 截图/视频/日志可关联单一 run_id 与构建。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-034.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

## MX-035 · 视觉、无障碍与性能验收

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S04 / pending（尚未执行仓库任务） |
| 角色 / Track | QA / Release / quality |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/035` |
| 直接依赖 | MX-008、MX-017、MX-034 |
| 条件依赖 | resources: MX-022,MX-024,MX-025；connectors: MX-023；remote: MX-026,MX-027；dictation: MX-028；voice: MX-029 |
| 原计划对应 | W：W36；UX：UX10 |
| 关闭缺口 | 页面/产品交付要求 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-035.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `tests/mobile-v2/visual-baseline.json`
- `docs/evidence/mobile-v2/accessibility-performance.md`
- `tests/mobile-v2/mx-035.test.ts`
- `tests/mobile-v2/probes/mx-035.ts`


### Interfaces

Consumes：高保真 screens/tokens + 实际原生实现。  
Produces：逐页视觉差异与性能基线报告。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string }
export interface Observation { unreachablePrimaryActions: string[]; criticalA11yFindings: string[] }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-035.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-035.ts';

test('visual-a11y-matrix', async () => {
  const observed = await runProbe({
  "fixture": "all-pages-light-dark-200-percent-native"
});
  assert.deepEqual(observed, {
  "unreachablePrimaryActions": [],
  "criticalA11yFindings": []
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-035.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 比对18页 light/dark、320–430宽、平板与横屏。
- [ ] 读屏/动态字体/键盘/返回/焦点/减少动画实测。
- [ ] 1000条消息与长文本测输入和滚动；记录设备/数据/耗时。
- [ ] 以明确差异清单修正，不以缩小字体消除溢出。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-035.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 所有主要文字层级、颜色语义、触控与组件态符合约束。
- [ ] 用户查看历史时流式输出不抢滚动。
- [ ] 性能指标按实测记录，不将目标值写成通过。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-035.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

## MX-036 · 发布门禁、回退与交接

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S04 / pending（尚未执行仓库任务） |
| 角色 / Track | QA / Release / quality |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/036` |
| 直接依赖 | MX-032、MX-033、MX-034、MX-035 |
| 条件依赖 | resources: MX-022,MX-024,MX-025；connectors: MX-023；remote: MX-026,MX-027；dictation: MX-028；voice: MX-029 |
| 原计划对应 | W：W33, W34, W35, W36, W37；UX：UX10 |
| 关闭缺口 | G11 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-036.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `docs/evidence/mobile-v2/release-report.md`
- `docs/evidence/mobile-v2/acceptance-index.json`
- `deploy/mobile-workbench/mobile-release.md`
- `tests/mobile-v2/mx-036.test.ts`
- `tests/mobile-v2/probes/mx-036.ts`


### Interfaces

Consumes：各任务 integrated SHA；profile 子门禁；原 W/T/H 依赖闭包。  
Produces：仅已验收 profile 发布结论与回退演练。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string }
export interface Observation { core: string; remote: string; globalAllPassed: boolean }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-036.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-036.ts';

test('release-blocked-subgate', async () => {
  const observed = await runProbe({
  "fixture": "core-pass-remote-blocked-env"
});
  assert.deepEqual(observed, {
  "core": "releasable",
  "remote": "blocked-env",
  "globalAllPassed": false
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-036.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 读取本轮启用的所有直接/条件/继承前置，并校验 hash 与证据。
- [ ] 未完成能力明确禁用；支付/公开个人节点/E2EE 单独授权。
- [ ] 新版本先关新准入并保留旧运行清理/结算；原生与OTA兼容窗口验证。
- [ ] 逐路径提交/PR/发布单独授权；交接复现命令、风险与责任人。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-036.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 没有以 core 通过覆盖 remote/resources/voice。
- [ ] native 依赖变更不作为纯JS更新下发。
- [ ] 旧运行/事件/账务能在回退后继续对账。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-036.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

外部门槛：["原 W 索引按激活 profile 递归闭包重新计算", "full_happy：W32 与 H24–H33 对应真实交互证据；本 MX 不替代其实现", "personal_node：W23 独立验收", "oidc：W08 真实 IdP 回跳验收"]。

