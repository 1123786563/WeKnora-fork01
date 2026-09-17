# S03 · 资源与受控扩展 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: 使用superpowers:subagent-driven-development或executing-plans逐任务推进，独立审查与证据后方可解锁依赖。

**Goal:** 交付资源与受控扩展的可审查工程增量。

**Architecture:** 沿WeKnora既有权限、执行与商业边界增量接线，移动端通过纯TS包和NativeHost消费产品能力，不以原型模拟代替真实服务。

**Tech Stack:** 现有Go/Gin/GORM、Expo/React Native、TypeScript/pnpm；先校准锁文件，不默认升级全部依赖。

**Spec:** [详细设计](../docs/01-detailed-design.md)、[页面规格](../docs/02-screen-specifications.md)、[设计系统](../docs/03-design-system.md)、[API分类](../docs/04-api-contracts.md)、[总计划](00-implementation-master.md)。

## Global Constraints

继续现有apps/mobile与WeKnora产品控制面；不新增权威mobile_tasks、第二套审批或钱包。保留原W/T/H依赖和验收状态。字段/路由/代码路径必须对照当前checkout；本计划新模块为建议增量，不承诺已经存在。一次用户意图一个持久request_id；未知启动先对账。取消、执行停止、费用结算分别表达。工具、预算与连接授权分别处理。静态、单元、数据库、原生和真实服务证据不混记；blocked-env不能记为通过。仅修改已取得锁的文件；不得擅自push、merge、发布或触发Provider真实写入。


## 本册执行规则

下方测试代码是**未来工程验收规范**，不是本次已经运行的项目测试。每任务的probe文件由该任务建立，调用实际模块/挂载组件/数据库/设备或解析真实命令输出，返回observed数据；严禁简单返回expected常量让测试通过。缺环境throw明确blocked-env并保留日志，不用SKIP当通过。初始测试模块缺失不是有效行为RED；建立最小接缝后必须看到真实断言失败，再修复实现。已有正确行为直接回归，不故意破坏制造RED。

## MX-022 · 资源与知识消费页面

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S03 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/022` |
| 直接依赖 | MX-008、MX-011 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W25, W28；UX：UX06 |
| 关闭缺口 | 页面/产品交付要求 |
| 验收profile | resources |
| 证据路径 | `docs/evidence/mobile-v2/MX-022.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/screens/ResourcesScreen.tsx`
- `apps/mobile/sources/weknora/screens/KnowledgeScreen.tsx`
- `packages/domain/src/mobile/resource-presentation.ts`
- `tests/mobile-v2/mx-022.test.ts`
- `tests/mobile-v2/probes/mx-022.ts`


### Interfaces

Consumes：现有资源列表/知识/文档 API。  
Produces：M11/M12 与知识引用选择模型。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string; fault: string }
export interface Observation { visibleSensitiveFields: string[]; canAskWithKnowledge: boolean }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-022.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-022.ts';

test('knowledge-access-revoked', async () => {
  const observed = await runProbe({
  "fixture": "cached-knowledge-title",
  "fault": "server-403"
});
  assert.deepEqual(observed, {
  "visibleSensitiveFields": [],
  "canAskWithKnowledge": false
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-022.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 资源分 Agent/知识/连接/成果，不复制全后台菜单。
- [ ] 文档列表展示扫描/索引状态；每次预览重新授权。
- [ ] 用此知识提问仅选引用，不复制文档进入新 tenant。
- [ ] 收藏归属 scope，不与产品权限混淆。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-022.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 知识未就绪给出明确状态。
- [ ] 资源搜索只查当前允许的范围。
- [ ] 知识可选，空知识库仍可创建通用任务。

- [ ] 保留resources子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-022.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

## MX-023 · 连接详情与授权生命周期

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S03 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/023` |
| 直接依赖 | MX-019、MX-022 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W24；UX：UX06 |
| 关闭缺口 | G11 |
| 验收profile | connectors |
| 证据路径 | `docs/evidence/mobile-v2/MX-023.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/screens/ConnectionScreen.tsx`
- `apps/mobile/sources/weknora/resources/connection-controller.ts`
- `tests/mobile-v2/mx-023.test.ts`
- `tests/mobile-v2/probes/mx-023.ts`


### Interfaces

Consumes：产品 Connection/Installation/Action 权限；OC T13/T18 子门禁。  
Produces：M13 connection state / allowlisted actions / revoke。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string; fault: string }
export interface Observation { providerWriteCount: number; rawSecretFields: string[] }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-023.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-023.ts';

test('connection-revoke-auth-version', async () => {
  const observed = await runProbe({
  "fixture": "auth-version3",
  "fault": "revoke-then-dispatch"
});
  assert.deepEqual(observed, {
  "providerWriteCount": 0,
  "rawSecretFields": []
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-023.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 明确个人连接和空间连接，不暴露 Provider 凭据。
- [ ] 授权入口使用产品 API；系统浏览器回跳后查询。
- [ ] 撤销带当前版本并使正在显示的权限失效。
- [ ] 真实外写开关仅由对应门禁控制，不因 T18 行 passed 自动开启。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-023.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 撤销后动作不可调用，不能空 grant 当拒绝全部。
- [ ] 连接授权不代表所有成员可读取密钥。
- [ ] 无 live-write/billing 证据只读或禁写。

- [ ] 保留connectors子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-023.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

外部门槛：["T13 产品 API 当前接线验证", "T18 provider_write/billing 子证据通过才可启用写操作"]。

## MX-024 · 不可变成果预览、下载与分享

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S03 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/024` |
| 直接依赖 | MX-017、MX-022 |
| 条件依赖 | remote: MX-027 |
| 原计划对应 | W：W25, W27, W28；UX：UX06 |
| 关闭缺口 | 页面/产品交付要求 |
| 验收profile | resources |
| 证据路径 | `docs/evidence/mobile-v2/MX-024.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/screens/ArtifactScreen.tsx`
- `apps/mobile/sources/weknora/resources/artifact-viewer.ts`
- `apps/mobile/sources/weknora/resources/share.ts`
- `tests/mobile-v2/mx-024.test.ts`
- `tests/mobile-v2/probes/mx-024.ts`


### Interfaces

Consumes：ArtifactRef{id,version,mime,size,run_id}；PreviewPort。  
Produces：M14 原生预览与受控系统分享。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string; fault: string }
export interface Observation { shareCount: number; persistentSignedUrls: string[] }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-024.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-024.ts';

test('artifact-link-expired', async () => {
  const observed = await runProbe({
  "fixture": "artifact-version1",
  "fault": "expired-url-then-permission-revoked"
});
  assert.deepEqual(observed, {
  "shareCount": 0,
  "persistentSignedUrls": []
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-024.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 下载前重新授权；签名 URL 不持久保存。
- [ ] 文本/图片/表格安全原生渲染；动态 HTML 优先静态化。
- [ ] 不可变 artifact/version 关联来源，工作目录文件先导入。
- [ ] 系统分享确认外流范围；缓存文件按策略清理。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-024.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 到期链接重新鉴权，不直接永久链接分享。
- [ ] 不可信 HTML 不读取产品 token/桥接或任意本地文件。
- [ ] 远程产物仅 remote profile 验证后显示。

- [ ] 保留resources子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-024.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

## MX-025 · 原生附件与上传校验

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S03 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/025` |
| 直接依赖 | MX-012、MX-015、MX-022 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W25；UX：UX06 |
| 关闭缺口 | 页面/产品交付要求 |
| 验收profile | resources |
| 证据路径 | `docs/evidence/mobile-v2/MX-025.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/resources/upload.ts`
- `apps/mobile/sources/weknora/platform/native-file.ts`
- `packages/api-client/src/knowledge/documents.ts`
- `tests/mobile-v2/mx-025.test.ts`
- `tests/mobile-v2/probes/mx-025.ts`


### Interfaces

Consumes：nativeFile + multipartFields；现有 session attachment API。  
Produces：UploadState 与正式 Expo 文件适配。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string; fault: string }
export interface Observation { lateResultApplied: boolean; draftPreserved: boolean }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-025.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-025.ts';

test('native-file-switch', async () => {
  const observed = await runProbe({
  "fixture": "Android-content-uri",
  "fault": "tenant-switch-during-upload"
});
  assert.deepEqual(observed, {
  "lateResultApplied": false,
  "draftPreserved": true
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-025.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] content:// 和 file:// 在 native port 转流/multipart，不传路径当文件。
- [ ] 客户端大小提示与服务端真实 MIME/摘要/扫描各司其职。
- [ ] abort/作用域切换/超时保留草稿并清理孤儿。
- [ ] 默认会话临时使用；入知识库另行明确操作。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-025.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] iOS照片/Android content URI 真文件链路通过。
- [ ] 伪 MIME/超限/零字节/撤权失败关闭。
- [ ] 扫描前 Agent 不能读取；取消不丢文本。

- [ ] 保留resources子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-025.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

## MX-026 · 执行目标选择与能力解释

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S03 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/026` |
| 直接依赖 | MX-016、MX-018 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W17, W18, W19；UX：UX09 |
| 关闭缺口 | 页面/产品交付要求 |
| 验收profile | remote |
| 证据路径 | `docs/evidence/mobile-v2/MX-026.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/screens/TargetPickerScreen.tsx`
- `packages/domain/src/mobile/target-options.ts`
- `tests/mobile-v2/mx-026.test.ts`
- `tests/mobile-v2/probes/mx-026.ts`


### Interfaces

Consumes：authorized target/workspace refs + capabilities。  
Produces：M15 目标选择 VM。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string }
export interface Observation { selectedTarget: string; admittedRemoteCount: number }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-026.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-026.ts';

test('target-not-observable', async () => {
  const observed = await runProbe({
  "fixture": "remote-create-true-observe-false"
});
  assert.deepEqual(observed, {
  "selectedTarget": "platform",
  "admittedRemoteCount": 0
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-026.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 按平台/托管/个人分类；只展示允许目标。
- [ ] 观察新鲜度与可用能力分开显示。
- [ ] 选择 target 不改变 tenant；workspace_ref 不等于空间。
- [ ] 未绑定节点给出原因，不接受任意 daemon URL。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-026.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 无权限 target 无敏感路径泄露。
- [ ] 离线节点不宣告任务失败或自动迁移。
- [ ] 未通过 probe 的目标不宣告 supported。

- [ ] 保留remote子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-026.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

## MX-027 · Paseo 服务身份与远程治理接线

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S03 / pending（尚未执行仓库任务） |
| 角色 / Track | Go 后端 / server |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/027` |
| 直接依赖 | MX-003、MX-006、MX-026 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W17, W18, W19, W20, W21, W22, W23, W24, W26；UX：UX09 |
| 关闭缺口 | G09 |
| 验收profile | remote |
| 证据路径 | `docs/evidence/mobile-v2/MX-027.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `internal/execution/mobile_driver_policy.go`
- `internal/handler/session/workbench_read.go`
- `internal/router/routes_workbench.go`
- `services/paseo-adapter/src/mobile-capabilities.ts`
- `tests/mobile-v2/mx-027.test.ts`
- `tests/mobile-v2/probes/mx-027.ts`


### Interfaces

Consumes：固定上游 SDK；服务身份；运行绑定/租约/fence。  
Produces：已验收的 target 能力集合与来源事件安全入口。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string; fault: string }
export interface Observation { persistedEvents: number; responseStatus: number }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-027.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-027.ts';

test('source-event-before-write', async () => {
  const observed = await runProbe({
  "fixture": "valid-run-wrong-binding",
  "fault": "cross-tenant-source-event"
});
  assert.deepEqual(observed, {
  "persistedEvents": 0,
  "responseStatus": 404
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-027.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 来源事件持久化前验证 service identity/tenant/run/binding/fence。
- [ ] 停止、启动回执不确定、租约过期都保留不确定状态并对账。
- [ ] 子 Run 复用预算树；不能复制余额或旁路 ActionService。
- [ ] 个人节点 W23 另验；产物导入不可变版本。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-027.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 普通移动 token 不能写 source-events。
- [ ] 错 binding/run 在任何写入前拒绝。
- [ ] 启动回执丢失不重复创建外部进程；迟到用量正确处理。

- [ ] 保留remote子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-027.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：隔离 PostgreSQL + SQLite。

## MX-028 · 确认式听写与会话草稿

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S03 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/028` |
| 直接依赖 | MX-017、MX-025 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W29；UX：UX08 |
| 关闭缺口 | 页面/产品交付要求 |
| 验收profile | dictation |
| 证据路径 | `docs/evidence/mobile-v2/MX-028.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/screens/VoiceInputScreen.tsx`
- `apps/mobile/sources/weknora/voice/dictation.ts`
- `tests/mobile-v2/mx-028.test.ts`
- `tests/mobile-v2/probes/mx-028.ts`


### Interfaces

Consumes：AudioPort/TranscriptionPort；scoped draft。  
Produces：M16 record → transcribe → edit → confirm。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string; fault: string }
export interface Observation { draft: string; submittedMessages: number }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-028.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-028.ts';

test('edited-transcript', async () => {
  const observed = await runProbe({
  "fixture": "initial-transcript",
  "fault": "edit-before-confirm"
});
  assert.deepEqual(observed, {
  "draft": "用户最终编辑的文本",
  "submittedMessages": 0
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-028.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 权限只在首次录音前申请；拒绝可继续文字任务。
- [ ] 转写结果进入可编辑 draft，不自动提交给 Agent。
- [ ] 用户确认后读取当前文本并沿普通提交/steer。
- [ ] 切空间/退出/取消音频关闭媒体但不取消已有 Run。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-028.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 修改转写后使用最新文字。
- [ ] 拒绝麦克风/打断/后台切换都有说明。
- [ ] 结束录音不等于取消任务或批准操作。

- [ ] 保留dictation子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-028.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

## MX-029 · 实时语音与独立中断语义

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S03 / pending（尚未执行仓库任务） |
| 角色 / Track | 共享 TS / Go / shared |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/029` |
| 直接依赖 | MX-006、MX-021、MX-028 |
| 条件依赖 | remote: MX-027 |
| 原计划对应 | W：W30, W31；UX：UX08 |
| 关闭缺口 | 页面/产品交付要求 |
| 验收profile | voice |
| 证据路径 | `docs/evidence/mobile-v2/MX-029.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/voice/realtime.ts`
- `apps/mobile/sources/weknora/voice/VoiceSessionPanel.tsx`
- `internal/application/service/workbench/voice_session.go`
- `tests/mobile-v2/mx-029.test.ts`
- `tests/mobile-v2/probes/mx-029.ts`


### Interfaces

Consumes：短期媒体令牌；预算授权；Run/voice session mapping。  
Produces：interrupt / endVoice / cancelTask 三个独立动作。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string; fault: string }
export interface Observation { mediaOutputStopped: boolean; runCancelCount: number }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-029.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-029.ts';

test('voice-interrupt-not-cancel', async () => {
  const observed = await runProbe({
  "fixture": "active-voice-active-run",
  "fault": "interrupt-output"
});
  assert.deepEqual(observed, {
  "mediaOutputStopped": true,
  "runCancelCount": 0
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-029.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 通过产品 API 授权与限额，不下发长期服务密钥。
- [ ] 媒体会话与产品 Run 绑定；账务沿原商业入口。
- [ ] 明确暂停播报/结束语音/取消任务的不同状态转移。
- [ ] 后台与来电打断处理；不可语音绕过危险写审批。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-029.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 三种停止操作互不误触发。
- [ ] 语音结束后迟到用量仍能结算。
- [ ] 真实媒体/推送环境缺失不得开放能力。

- [ ] 保留voice子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-029.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

## MX-030 · 账户偏好、外观与退出

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S03 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/030` |
| 直接依赖 | MX-009、MX-011 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：W07, W13；UX：UX01 |
| 关闭缺口 | 页面/产品交付要求 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-030.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/screens/ProfileScreen.tsx`
- `apps/mobile/sources/weknora/preferences/store.ts`
- `tests/mobile-v2/mx-030.test.ts`
- `tests/mobile-v2/probes/mx-030.ts`


### Interfaces

Consumes：AuthState/Scope；token theme；设备撤销。  
Produces：M17 偏好页与完整退出流程。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string; fault: string }
export interface Observation { credential: null; activeStreams: number; oldTokenRewritten: boolean }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-030.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-030.ts';

test('logout-device-generation', async () => {
  const observed = await runProbe({
  "fixture": "active-credential-and-device",
  "fault": "late-token-refresh"
});
  assert.deepEqual(observed, {
  "credential": null,
  "activeStreams": 0,
  "oldTokenRewritten": false
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-030.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 主题支持 system/light/dark；原型提供 light/dark 对照。
- [ ] 通知偏好与系统权限分开，关闭偏好不停止任务。
- [ ] 退出撤销设备、关闭流/语音、清凭证和指定缓存。
- [ ] 偏好不含 Provider secrets；不清别的账户数据。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-030.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 退出中刷新不会恢复旧账户。
- [ ] 浅深主题、字体、页面状态保持一致。
- [ ] 网络离线退出仍本地安全清理并记录最小撤销意图。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-030.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

## MX-031 · 空间用量与只读商业视图

| 调度项 | 值 |
|---|---|
| 阶段 / 状态 | S03 / pending（尚未执行仓库任务） |
| 角色 / Track | Expo 客户端 / native |
| 仓库 / 模块键 | 1123786563/WeKnora-fork01 / `mobile-v2/031` |
| 直接依赖 | MX-013、MX-030 |
| 条件依赖 | 无新增MX条件边 |
| 原计划对应 | W：按MX-001对账；UX：UX06 |
| 关闭缺口 | 页面/产品交付要求 |
| 验收profile | core |
| 证据路径 | `docs/evidence/mobile-v2/MX-031.md` |

### Files与文件锁

以下每条是完整写锁key；Create/Modify在MX-001逐路径判定。同一文件一次只允许一个写入者。

- `apps/mobile/sources/weknora/screens/UsageScreen.tsx`
- `apps/mobile/sources/weknora/commercial/usage-presenter.ts`
- `tests/mobile-v2/mx-031.test.ts`
- `tests/mobile-v2/probes/mx-031.ts`


### Interfaces

Consumes：现有商业域授权读取端口。  
Produces：M18 可用/预占/已结算/待结算分离。  
测试适配器签名（只属于本任务的测试支持层）：

```ts
export interface ProbeInput { fixture: string }
export interface Observation { reserved: number; pending: number; settled: number; paymentRequests: number }
export function runProbe(input: ProbeInput): Promise<Observation>;
```

`runProbe`必须从上述实际入口观测结果，不得成为第二套业务实现；Go任务可由probe调用精确Go测试/fixture导出，原生任务由probe读取已绑定build版本的真实设备结果。不能使用HTML示例state充当原生结果。

### 步骤1 · 固定测试与行为RED

- [ ] 读取关联规格、当前实现及父任务accepted SHA，记录实际基线与现有证据。
- [ ] 在`tests/mobile-v2/mx-031.test.ts`写入以下断言，按本任务的probe签名接实际边界：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { runProbe } from './probes/mx-031.ts';

test('usage-finality', async () => {
  const observed = await runProbe({
  "fixture": "reserved120-pending38-settled1260"
});
  assert.deepEqual(observed, {
  "reserved": 120,
  "pending": 38,
  "settled": 1260,
  "paymentRequests": 0
});
});
```

- [ ] 运行 `pnpm exec tsx --test tests/mobile-v2/mx-031.test.ts`；记录失败的具体断言、输入fixture与原始输出。依赖未安装/环境缺失不得充当行为RED。

### 步骤2 · 实现与真实接线

- [ ] 只适配选定 OpenMeter 模型与商业域，不建新钱包。
- [ ] 显示时间与账单账户作用域，不混用其他空间余额。
- [ ] BYOK 模型费和平台服务费分开说明，不假定全任务免费。
- [ ] 普通 Admin 与 billing admin 分权；购买/退款不纳入本轮。

- [ ] 对照Files核对实际路由、DI、SDK、Host、Screen入口；出现额外写入文件先扩大锁与计划，不“顺手”改共享文件。

### 步骤3 · GREEN与额外验收

- [ ] 重跑 `pnpm exec tsx --test tests/mobile-v2/mx-031.test.ts`，确认测试发现数、退出码与是否存在skip；在改动后的集成SHA再次验证。
- [ ] 没有商业权限不显示金额与敏感明细。
- [ ] 进行中消耗明确非最终数字。
- [ ] 记录来源与 as_of；无来源不展示假精确值。

- [ ] 保留core子门禁结果；只要激活外部/原生能力就补对应真实环境证据。纯controller测试不代替挂载页面，编译不代替设备，fixture不代替Provider。

### 步骤4 · 审查、提交与解锁

- [ ] 独立reviewer分别给出规格与质量结论，覆盖并发/权限/恢复和本任务完整接线；失败退回修复并复验。
- [ ] 在隔离任务分支逐路径暂存上述实际改动；`git diff --cached --check`后提交。禁止`git add .`、擅自push/merge。
- [ ] 把BASE/HEAD/integrated SHA、实际命令、环境/退出码、测试数、证据路径和剩余风险写入`docs/evidence/mobile-v2/MX-031.md`。仅有全部激活前置与子门禁通过才accepted。

### 并行条件

直接和激活条件依赖已accepted；接口版本冻结；Files写集合与其他活跃任务无交集；原生设备、数据库schema、端口、节点/测试目录互相隔离。否则排队，不能仅凭不同worktree并行。

环境门槛：iOS/Android 开发构建。

