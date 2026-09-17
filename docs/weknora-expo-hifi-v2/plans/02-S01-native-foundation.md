# S01 · 原生基础与设计系统 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: 使用superpowers:subagent-driven-development或executing-plans逐任务推进，独立审查与证据后方可解锁依赖。

**Goal:** 交付原生基础与设计系统的可审查工程增量。

**Architecture:** 沿WeKnora既有权限、执行与商业边界增量接线，移动端通过纯TS包和NativeHost消费产品能力，不以原型模拟代替真实服务。

**Tech Stack:** 现有Go/Gin/GORM、Expo/React Native、TypeScript/pnpm；先校准锁文件，不默认升级全部依赖。

**Spec:** [详细设计](../docs/01-detailed-design.md)、[页面规格](../docs/02-screen-specifications.md)、[设计系统](../docs/03-design-system.md)、[API分类](../docs/04-api-contracts.md)、[总计划](00-implementation-master.md)。

## Global Constraints

继续现有apps/mobile与WeKnora产品控制面；不新增权威mobile_tasks、第二套审批或钱包。保留原W/T/H依赖和验收状态。字段/路由/代码路径必须对照当前checkout；本计划新模块为建议增量，不承诺已经存在。一次用户意图一个持久request_id；未知启动先对账。取消、执行停止、费用结算分别表达。工具、预算与连接授权分别处理。静态、单元、数据库、原生和真实服务证据不混记；blocked-env不能记为通过。仅修改已取得锁的文件；不得擅自push、merge、发布或触发Provider真实写入。


## 本册执行规则

下方测试代码是**未来工程验收规范**，不是本次已经运行的项目测试。每任务的probe文件由该任务建立，调用实际模块/挂载组件/数据库/设备或解析真实命令输出，返回observed数据；严禁简单返回expected常量让测试通过。缺环境throw明确blocked-env并保留日志，不用SKIP当通过。初始测试模块缺失不是有效行为RED；建立最小接缝后必须看到真实断言失败，再修复实现。已有正确行为直接回归，不故意破坏制造RED。

## MX-007 · 语义设计令牌与原生主题

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S01 / pending（尚未执行仓库任务） |
| 角色 / Track | 设计系统 / TS / shared |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/007` |
| 直接依赖 | MX-001 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：按MX-001对账；UX：UX01 |
| 关闭缺口 | 页面/产品交付要求 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-007.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `packages/design-tokens/src/mobile/tokens.json`
- `packages/design-tokens/src/mobile/native-tokens.ts`
- `packages/design-tokens/src/mobile/tokens.css`
- `apps/mobile/sources/weknora/ui/theme.ts`
- `tests/mobile-v2/mx-007.test.ts`
- `tests/mobile-v2/probes/mx-007.ts`


### Interfaces

Consumes：本交付 tokens/tokens.json。  
Produces：light/dark 原生主题与 CSS 输出；token 测试。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string }
export interface Observation { missingThemeKeys: string[]; webNativeColorDiff: string[] }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-007.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-007.ts';

test('token-theme-parity', async () => {
  const observed = await runProbe({
  "fixture": "tokens.json"
});
  assert.deepEqual(observed, {
  "missingThemeKeys": [],
  "webNativeColorDiff": []
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-007.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 将本交付语义令牌作为版本化源，先比对现有 tokens 目录。
- [ ] 原生数值 dp、字体缩放、圆角、层级与动效独立映射。
- [ ] 组件仅消费语义色，不依赖硬编码 hex 或 Web className。
- [ ] 新增 token 必须含用途、主题值和前景背景对比用例。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-007.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 浅/深色全部必需 key 同构。
- [ ] 关键文本/状态对比率 ≥4.5，不只检查主色。
- [ ] 生成结果无漂移，支持 reduced motion 与动态字体。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-007.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

## MX-008 · 原生基础组件与状态组件

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S01 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/008` |
| 直接依赖 | MX-007 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：按MX-001对账；UX：UX01 |
| 关闭缺口 | 页面/产品交付要求 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-008.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/ui/Button.tsx`
- `apps/mobile/sources/weknora/ui/Card.tsx`
- `apps/mobile/sources/weknora/ui/StatusBadge.tsx`
- `apps/mobile/sources/weknora/ui/StateView.tsx`
- `apps/mobile/sources/weknora/ui/Sheet.tsx`
- `apps/mobile/sources/weknora/ui/Field.tsx`
- `tests/mobile-v2/mx-008.test.ts`
- `tests/mobile-v2/probes/mx-008.ts`


### Interfaces

Consumes：native tokens；Capability。  
Produces：无业务网络副作用的 primitives。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string; fault: string }
export interface Observation { decisionCount: number; focusTarget: string }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-008.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-008.ts';

test('sheet-focus-cancel', async () => {
  const observed = await runProbe({
  "fixture": "mounted-native-sheet",
  "fault": "dismiss-before-decision"
});
  assert.deepEqual(observed, {
  "decisionCount": 0,
  "focusTarget": "trigger"
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-008.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 实现 primary/secondary/danger/disabled/loading/focus 按钮变体。
- [ ] 卡片/列表/空态/错误/离线/加载组件通过 props 驱动。
- [ ] Sheet 处理 focus、返回、键盘和大字体；禁止嵌套点击区域。
- [ ] 状态文字与颜色同时存在，动态文案提供读屏标签。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-008.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 紧凑触控区44、主触控区48，文本放大不裁切。
- [ ] busy 按钮不重复触发；危险操作不只靠颜色。
- [ ] 浅/深主题、VoiceOver/TalkBack 有可判读标签。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-008.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

## MX-009 · 四 Tab 与产品路由容器

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S01 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/009` |
| 直接依赖 | MX-003、MX-008 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W10, W11；UX：UX01 |
| 关闭缺口 | G07 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-009.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/app/(app)/_layout.tsx`
- `apps/mobile/sources/app/_layout.tsx`
- `apps/mobile/sources/weknora/navigation/routes.ts`
- `apps/mobile/sources/weknora/navigation/ProductShell.tsx`
- `tests/mobile-v2/mx-009.test.ts`
- `tests/mobile-v2/probes/mx-009.ts`


### Interfaces

Consumes：产品 route model；基础组件。  
Produces：四 Tab + 详情 Stack + modal 路由映射。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string }
export interface Observation { visibleTabs: string[]; happyAuthRequests: number }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-009.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-009.ts';

test('product-route-no-happy', async () => {
  const observed = await runProbe({
  "fixture": "product-scope-without-happy-sync"
});
  assert.deepEqual(observed, {
  "visibleTabs": [
    "工作台",
    "会话",
    "资源",
    "我的"
  ],
  "happyAuthRequests": 0
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-009.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 确认现有 Expo Router 文件，保留根和旧路由兼容入口。
- [ ] 产品 screen 与 legacy Happy screen 显式拆边界。
- [ ] Tab 只承载工作台/会话/资源/我的；详情保留返回栈。
- [ ] 恢复深链前不得绕过 ProductAuthProvider/ScopeProvider。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-009.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] M01–M18 均有唯一 screen ID 与导航路径。
- [ ] 详情页返回不会回到另一空间的旧页。
- [ ] 无需 Happy daemon/sync 才能渲染产品壳。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-009.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

## MX-010 · 产品登录、SSO 与冷启动身份

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S01 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/010` |
| 直接依赖 | MX-002、MX-003、MX-009 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W07, W08；UX：UX00 |
| 关闭缺口 | 页面/产品交付要求 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-010.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/auth/session.tsx`
- `apps/mobile/sources/weknora/auth/AuthReturnScreen.tsx`
- `apps/mobile/sources/weknora/auth/bootstrap.ts`
- `apps/mobile/sources/weknora/screens/LoginScreen.tsx`
- `tests/mobile-v2/mx-010.test.ts`
- `tests/mobile-v2/probes/mx-010.ts`


### Interfaces

Consumes：CredentialAdapter；OIDC SDK；bootstrap port。  
Produces：AuthState 与 user/membership/scope 完整初始化。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string }
export interface Observation { userId: string; tenantId: string; credentialInOrdinaryStorage: boolean }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-010.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-010.ts';

test('cold-start-scope', async () => {
  const observed = await runProbe({
  "fixture": "valid-credential-missing-scope"
});
  assert.deepEqual(observed, {
  "userId": "u1",
  "tenantId": "t1",
  "credentialInOrdinaryStorage": false
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-010.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] SecureStore 读凭证后执行服务端验证并加载成员关系。
- [ ] 401 single-flight 刷新；刷新回写检查 generation；403 不刷新。
- [ ] 系统浏览器 SSO 使用 PKCE、一次性 code、state、redirect 绑定。
- [ ] 未知 origin 不自动切换；logout 不回退初始 origin。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-010.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 刷新中退出后旧 token 不回写。
- [ ] 重启/SSO 成功后 user/tenant 不是空值。
- [ ] 真实 IdP 缺环境记录 blocked-env，密码登录证据不覆盖 SSO。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-010.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

## MX-011 · 空间切换、缓存与迟到响应隔离

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S01 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/011` |
| 直接依赖 | MX-010 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W07, W11；UX：UX02 |
| 关闭缺口 | 页面/产品交付要求 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-011.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/platform/product-session.ts`
- `apps/mobile/sources/weknora/screens/SpacePickerScreen.tsx`
- `packages/domain/src/mobile/query-scope.ts`
- `tests/mobile-v2/mx-011.test.ts`
- `tests/mobile-v2/probes/mx-011.ts`


### Interfaces

Consumes：AuthState；成员授权关系。  
Produces：generation-aware QueryKey/teardown 和切换控制器。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string; fault: string }
export interface Observation { visibleTenant: string; oldResponseApplied: boolean; canceledServerRuns: number }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-011.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-011.ts';

test('late-response-after-switch', async () => {
  const observed = await runProbe({
  "fixture": "tenant-A-to-B",
  "fault": "A-response-arrives-last"
});
  assert.deepEqual(observed, {
  "visibleTenant": "B",
  "oldResponseApplied": false,
  "canceledServerRuns": 0
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-011.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 阻止新 mutation 后递增 generation；abort 请求与 stream。
- [ ] 清除旧界面敏感内容，重建 QueryClient scope。
- [ ] 按 origin/user/tenant 读取对应草稿与缓存，不共享全局 key。
- [ ] 保留服务端 Run；关闭旧语音/远程订阅；设备绑定独立撤销。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-011.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 团队A→个人B没有旧消息、标题或账单闪现。
- [ ] 旧请求/刷新/上传完成后不能污染新空间。
- [ ] 返回A可恢复A草稿；未授权A则清理并拒绝恢复。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-011.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

## MX-012 · 原生持久事件、快照与恢复

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S01 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/012` |
| 直接依赖 | MX-002、MX-004、MX-011 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W09, W12；UX：UX04 |
| 关闭缺口 | G06, G10 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-012.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/platform/execution-storage.ts`
- `apps/mobile/sources/weknora/platform/native-host.ts`
- `apps/mobile/sources/weknora/platform/recovery.ts`
- `packages/domain/src/mobile/execution-cache.ts`
- `tests/mobile-v2/mx-012.test.ts`
- `tests/mobile-v2/probes/mx-012.ts`


### Interfaces

Consumes：StreamFrame；ScopeKey；SubmissionStore。  
Produces：SQLite 实际 driver + 加密 payload + 恢复控制器。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string; fault: string }
export interface Observation { cursor: number; persistedSeqs: number[] }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-012.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-012.ts';

test('disk-full-transaction', async () => {
  const observed = await runProbe({
  "fixture": "persisted-seq41",
  "fault": "disk-full-on-seq42"
});
  assert.deepEqual(observed, {
  "cursor": 41,
  "persistedSeqs": [
    41
  ]
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-012.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 真实 expo/fetch 与 Expo SQLite 端口注入；不使用 Web 测试代替。
- [ ] 写队列与 exclusive txn 内使用 txn 对象；显式捕获 void wrapper 返回结果。
- [ ] 事件/投影/cursor 同事务；AEAD 密钥初始化单飞、AAD 绑定 scope/run/seq。
- [ ] 过期 cursor 原子替换 snapshot/watermark 后重放；恢复先认证再对账。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-012.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 磁盘满/事务 rollback 不推进 cursor。
- [ ] 并发首次密钥、AAD篡改、密钥丢失、database locked 均可判定。
- [ ] 杀 App 重开原生数据库后消息/草稿/原请求状态正确恢复。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-012.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

