# S00 · 基线与契约校准 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: 使用superpowers:subagent-driven-development或executing-plans逐任务推进，独立审查与证据后方可解锁依赖。

**Goal:** 交付基线与契约校准的可审查工程增量。

**Architecture:** 沿WeKnora既有权限、执行与商业边界增量接线，移动端通过纯TS包和NativeHost消费产品能力，不以原型模拟代替真实服务。

**Tech Stack:** 现有Go/Gin/GORM、Expo/React Native、TypeScript/pnpm；先校准锁文件，不默认升级全部依赖。

**Spec:** [详细设计](../docs/01-detailed-design.md)、[页面规格](../docs/02-screen-specifications.md)、[设计系统](../docs/03-design-system.md)、[API分类](../docs/04-api-contracts.md)、[总计划](00-implementation-master.md)。

## Global Constraints

继续现有apps/mobile与WeKnora产品控制面；不新增权威mobile_tasks、第二套审批或钱包。保留原W/T/H依赖和验收状态。字段/路由/代码路径必须对照当前checkout；本计划新模块为建议增量，不承诺已经存在。一次用户意图一个持久request_id；未知启动先对账。取消、执行停止、费用结算分别表达。工具、预算与连接授权分别处理。静态、单元、数据库、原生和真实服务证据不混记；blocked-env不能记为通过。仅修改已取得锁的文件；不得擅自push、merge、发布或触发Provider真实写入。


## 本册执行规则

下方测试代码是**未来工程验收规范**，不是本次已经运行的项目测试。每任务的probe文件由该任务建立，调用实际模块/挂载组件/数据库/设备或解析真实命令输出，返回observed数据；严禁简单返回expected常量让测试通过。缺环境throw明确blocked-env并保留日志，不用SKIP当通过。初始测试模块缺失不是有效行为RED；建立最小接缝后必须看到真实断言失败，再修复实现。已有正确行为直接回归，不故意破坏制造RED。

## MX-001 · 当前代码、接口与台账对账

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S00 / pending（尚未执行仓库任务） |
| 角色 / Track | 架构 / 协调 / shared |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/001` |
| 直接依赖 | 无；需可读当前checkout |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：按MX-001对账；UX：UX00 |
| 关闭缺口 | G11, G12 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-001.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `docs/evidence/mobile-v2/current-state-matrix.md`
- `docs/evidence/mobile-v2/file-ownership.json`
- `tests/mobile-v2/mx-001.test.ts`
- `tests/mobile-v2/probes/mx-001.ts`


### Interfaces

Consumes：本交付 references 与当前 checkout HEAD。  
Produces：EvidenceMatrix；逐路径 Create/Modify 裁决；原 W/T 子门禁映射。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string }
export interface Observation { unownedGaps: string[]; duplicateOwners: string[] }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-001.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-001.ts';

test('inventory-coverage', async () => {
  const observed = await runProbe({
  "fixture": "current-checkout-and-G01-G12"
});
  assert.deepEqual(observed, {
  "unownedGaps": [],
  "duplicateOwners": []
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-001.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 记录 HEAD、锁文件哈希、git status 与所有作用路径的 AGENTS；不清理用户改动。
- [ ] 按 G01–G12 对比当前实现，已有行为先复验，不制造退化。
- [ ] 检索全量 router/DI 和原 W 索引，标注 A/B/C 接口与前置证据。
- [ ] 预登记共享文件、迁移两序列和 profile 负责人。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-001.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 每个 G 缺口都有具体代码位置、负责人和下一测试。
- [ ] Create/Modify 不与实际文件状态矛盾。
- [ ] 禁止将原 pending 或 passed 自动复制成本轮事实。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-001.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

## MX-002 · Expo 与 React 原生依赖基线

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S00 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/002` |
| 直接依赖 | MX-001 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W36；UX：UX00 |
| 关闭缺口 | G05 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-002.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/package.json`
- `package.json`
- `pnpm-lock.yaml`
- `docs/evidence/mobile-v2/native-baseline.md`
- `tests/mobile-v2/mx-002.test.ts`
- `tests/mobile-v2/probes/mx-002.ts`


### Interfaces

Consumes：当前锁文件；SDK55 官方矩阵。  
Produces：可复现的 SDK/RN/React/renderer/native modules 版本组合。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string }
export interface Observation { rendererMatchesReact: boolean; nativeBuildPlatforms: string[] }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-002.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-002.ts';

test('native-version-compat', async () => {
  const observed = await runProbe({
  "fixture": "locked-expo55-baseline"
});
  assert.deepEqual(observed, {
  "rendererMatchesReact": true,
  "nativeBuildPlatforms": [
    "ios",
    "android"
  ]
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-002.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 读取实际依赖解析而非只看 manifest。
- [ ] 在独立依赖分支对齐 React、renderer、Expo 模块及包管理器。
- [ ] 逐一复验 libsodium/音视频/MMKV/Unistyles/Reanimated/键盘模块。
- [ ] 对 android/ios 归属和 prebuild 删除行为确认后再构建；升级新 SDK 独立决策。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-002.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] doctor 的未解决项逐条列明。
- [ ] iOS/Android 同一 lockfile 构建并启动关键模块。
- [ ] 没有通过关闭新架构或随意移除功能掩盖不兼容。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-002.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

## MX-003 · 跨语言契约与能力规则冻结

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S00 / pending（尚未执行仓库任务） |
| 角色 / Track | 共享 TS / Go / shared |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/003` |
| 直接依赖 | MX-001 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W01, W06；UX：UX00 |
| 关闭缺口 | G03, G08 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-003.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `packages/contracts/src/mobile/execution.ts`
- `packages/contracts/src/mobile/interactions.ts`
- `packages/contracts/src/mobile/read-models.ts`
- `internal/workbench/contracts.go`
- `internal/workbench/interaction.go`
- `tests/mobile-v2/mx-003.test.ts`
- `tests/mobile-v2/probes/mx-003.ts`


### Interfaces

Consumes：现有 ExecutionDTO/StartInput；本交付提议契约。  
Produces：完整 ExecutionEvent；三态 Capability；typed Interaction；契约版本清单。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string }
export interface Observation { unknownType: string; canCancel: boolean }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-003.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-003.ts';

test('unknown-capability', async () => {
  const observed = await runProbe({
  "fixture": "unknown-event-and-unavailable-cancel"
});
  assert.deepEqual(observed, {
  "unknownType": "future.event",
  "canCancel": false
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-003.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 逐字段标注已存在/提议/兼容转换；不擅自增加 start 字段。
- [ ] 冻结未知状态展示策略与命令拒绝规则。
- [ ] 创建同一 Go/TS JSON fixture；安全整数上界与 UTC 时间验证。
- [ ] A 类路由 body 保持兼容，B 类接口在服务端接通前不发布。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-003.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 未知事件 type 可保留但未知控制状态禁止写入。
- [ ] seq 非整数、溢出、缺 run_id、数组 payload 均拒绝。
- [ ] 预算/工具/问题/连接交互不能互换 payload。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-003.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

## MX-004 · SSE 完整事件与控制帧接通

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S00 / pending（尚未执行仓库任务） |
| 角色 / Track | 共享 TS / Go / shared |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/004` |
| 直接依赖 | MX-003 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W03, W09；UX：UX04 |
| 关闭缺口 | G01 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-004.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `internal/handler/session/workbench_read.go`
- `apps/mobile/sources/weknora/platform/stream-transport.ts`
- `packages/domain/src/mobile/execution-frames.ts`
- `tests/mobile-v2/mx-004.test.ts`
- `tests/mobile-v2/probes/mx-004.ts`


### Interfaces

Consumes：ExecutionEvent；版本协商规则。  
Produces：Go writer → native parser 一致字节合同。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string; fault: string }
export interface Observation { seqs: number[]; controlCount: number; cursor: number }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-004.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-004.ts';

test('go-bytes-to-native-parser', async () => {
  const observed = await runProbe({
  "fixture": "Go-emitted-UTF8-CRLF-control-seq42",
  "fault": "split-every-byte"
});
  assert.deepEqual(observed, {
  "seqs": [
    42
  ],
  "controlCount": 1,
  "cursor": 42
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-004.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 明确产品流 version=2 协商；保留旧调用兼容测试。
- [ ] 将完整 envelope 写入业务 frame，control/error 不占业务 seq。
- [ ] UTF-8/CRLF 增量解码并拒绝 id/type 冲突。
- [ ] 终态 drain 到一致 watermark；cursor_expired 触发快照路径。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-004.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 中文逐字节、拆 CRLF、心跳、多 data 行、EOF 都能解析。
- [ ] Go 实际输出直接喂 TS parser，不使用各自不同 fixture。
- [ ] 控制帧不推进 cursor，超过 256 条仍无丢失。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-004.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

## MX-005 · 类型化交互与乐观并发命令

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S00 / pending（尚未执行仓库任务） |
| 角色 / Track | Go 后端 / server |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/005` |
| 直接依赖 | MX-003 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W05；UX：UX05 |
| 关闭缺口 | G02, G03 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-005.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `internal/handler/session/workbench_commands.go`
- `internal/application/service/workbench/interaction.go`
- `packages/api-client/src/mobile/interactions.ts`
- `tests/mobile-v2/mx-005.test.ts`
- `tests/mobile-v2/probes/mx-005.ts`


### Interfaces

Consumes：InteractionDecisionDraft；现有动作审批与授权服务。  
Produces：list/refresh/decide adapter；revision + digest + requestID 原子检查。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string; fault: string }
export interface Observation { acceptedCount: number; conflictCount: number }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-005.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-005.ts';

test('concurrent-decisions', async () => {
  const observed = await runProbe({
  "fixture": "same-interaction-revision4",
  "fault": "two-concurrent-approve"
});
  assert.deepEqual(observed, {
  "acceptedCount": 1,
  "conflictCount": 1
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-005.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 读取真实 interaction DTO 并建立显式适配，禁止 any/optional-call 掩盖缺失。
- [ ] 绑定 actor、tenant、冻结内容摘要、权限版本、到期时间。
- [ ] 同意/拒绝 CAS；相同决定幂等，旧版本返回 409。
- [ ] 工具审批不叠加第二次商业准入；预算审批单独检查角色。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-005.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 两个设备并发决定只接受一次合法变更。
- [ ] 撤权/到期/修改目标后旧批准不可执行。
- [ ] 403 不刷新 token；unknown 不能伪装 failed 后重发。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-005.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：隔离 PostgreSQL + SQLite。

## MX-006 · 持久提交身份与请求对账

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S00 / pending（尚未执行仓库任务） |
| 角色 / Track | 共享 TS / Go / shared |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/006` |
| 直接依赖 | MX-003 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W04, W06；UX：UX03 |
| 关闭缺口 | G04 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-006.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `packages/domain/src/mobile/submission.ts`
- `packages/api-client/src/mobile/executions.ts`
- `internal/application/service/workbench/admission.go`
- `tests/mobile-v2/mx-006.test.ts`
- `tests/mobile-v2/probes/mx-006.ts`


### Interfaces

Consumes：StartExecutionInput；RequestLookup；SubmissionStore port。  
Produces：startOnce / reconcileRequest 状态机。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string; fault: string }
export interface Observation { startCount: number; lookupRequestId: string; runId: string }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-006.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-006.ts';

test('lost-ack-restart', async () => {
  const observed = await runProbe({
  "fixture": "same-request-and-input",
  "fault": "ack-drop-process-restart"
});
  assert.deepEqual(observed, {
  "startCount": 1,
  "lookupRequestId": "request-original",
  "runId": "run-original"
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-006.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 构造规范化输入摘要；网络前持久保存 request_id 与 scope。
- [ ] ACK 成功绑定原 run_id；超时置 awaiting_reconciliation。
- [ ] lookup 区分 pending/dispatching/admitted/rejected/unknown。
- [ ] 明确 rejected-before-dispatch 才开放受控重试；输入变更须新意图确认。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-006.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 双击、ACK 丢失、杀进程后只找到同一 Run。
- [ ] 相同 requestID 不同正文返回冲突。
- [ ] unknown 不触发重新 POST 或新预算预占。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-006.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

