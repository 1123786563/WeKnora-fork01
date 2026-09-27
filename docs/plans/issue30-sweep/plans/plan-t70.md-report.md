# Task 7 实施报告：平台一致性扫描 + Android 七工作流验收矩阵

**Status: DONE** | Commit: `6b09d1a3b`（初版）→ `322c57d01`（修复轮 1）| 日期：2026-09-27

## 任务定位

Issue #70 实施计划（`docs/plans/issue30-sweep/plans/plan-t70.md`）7 个任务中的第 7 个（收口任务）：新建 `apps/mobile/src/android-acceptance.ts`（七工作流结构化验收矩阵数据）与 `apps/mobile/src/android-parity.test.ts`（双守卫：平台 API 源级扫描 + 矩阵完整性 existsSync 强制）。初版完成后经审查修复轮 1（见文末）补强 ESM 盲区守卫。

## 实现内容

### 交付物 1：`apps/mobile/src/android-acceptance.ts`（新建，+130 行，commit `6b09d1a3b`）

计划 Task 7 Step 3 代码逐字落地，导出：

- `ANDROID_WORKFLOWS`：七项核心工作流常量元组（`system-back` / `background-limits` / `notifications` / `file-uri` / `keystore` / `microphone` / `weak-network`，与 Issue #70「验证系统返回、后台限制、通知、文件 URI、Keystore、麦克风和弱网恢复」一一对应）；
- `AndroidWorkflow`、`AndroidWorkflowAcceptance`（`workflow` / `domainInterface` / `adapterSeam` / `localEvidence[]` / `residual: { kind: 'blocked-env'; reason; unblock }`）；
- `ANDROID_ACCEPTANCE_MATRIX`：七行矩阵——#71 发布证据矩阵直接消费本表（计划 Produces 原文）。

### 交付物 2：`apps/mobile/src/android-parity.test.ts`（新建，+67 行 → 修复轮后 +39/-1，commit `322c57d01`）

计划 Task 7 Step 1 代码逐字落地，三个测试：

1. **`platform APIs stay inside src/adapters`**——递归扫描 `apps/mobile/src` 全部非测试 `.ts/.tsx`，react-native 平台 API 接线只允许在 `src/adapters/`；sanity 断言 `adapterHits >= 1`（防扫描器失效静默通过）。
2. **`screens and routes never import wire clients or shared contracts`**——`screens/`、`app/` 禁止 `@weknora/api-client|contracts` 直连。
3. **`the Android acceptance matrix covers exactly the seven core workflows with real evidence paths`**——七工作流精确覆盖、seam existsSync、localEvidence 全部 existsSync、`residual.kind` 恒 `'blocked-env'` 且 reason/unblock 非空。

### 前置证据核对（写码前先做）

实现前对矩阵引用的全部 21 个文件（7 个 `adapterSeam` + 17 条去重后 `localEvidence`）逐一 `[ -f ]` 核对：21/21 全部 OK——含 Task 6 刚入库的 `apps/mobile/src/adapters/app-state.test.ts`。

## TDD 证据（初版）

### RED

命令：`pnpm exec tsx --test apps/mobile/src/android-parity.test.ts`

```
# Error: Cannot find module './android-acceptance.ts'
#   code: 'MODULE_NOT_FOUND',
1..1
# tests 1
# pass 0
# fail 1
EXIT=1
```

### GREEN

同命令 →

```
ok 1 - platform APIs stay inside src/adapters (验收标准 2：平台差异仅在 Adapter)
ok 2 - screens and routes never import wire clients or shared contracts (module-seams §10)
ok 3 - the Android acceptance matrix covers exactly the seven core workflows with real evidence paths (验收标准 3)
# tests 3
# pass 3
# fail 0
EXIT=0
```

## 全量回归（Task 7 Step 5，计划指定命令逐字执行，初版）

1. `pnpm --filter @weknora/mobile test` → `243 tests / 232 pass / 0 fail / 11 skip`，TEST_EXIT=0；
2. `pnpm --filter @weknora/mobile typecheck` → exit 0；
3. `pnpm exec tsx --test packages/mobile-core/src/voice/dictation.test.ts` → `18 pass / 0 fail`。

## 计划级验证命令（plan-t70.md:1282 整链，初版）

```
# tests 40 / # pass 40 / # fail 0 / # skipped 0      ← 六文件定向验证
# tests 67 / # pass 67 / # fail 0 / # skipped 0      ← app-smoke 全量
# tests 243 / # pass 232 / # fail 0 / # skipped 11   ← apps/mobile 全量
> tsc --noEmit                                       ← typecheck
PLAN_LEVEL_EXIT=0
```

## 提交（初版）

`6b09d1a3b` — test(mobile): 平台一致性源级扫描 + Android 七工作流验收矩阵（blocked-env 如实编码，#70）——2 files / 197 insertions(+)。

## 自检发现（初版）

- 两个文件与计划 Task 7 Step 1/Step 3 逐字一致；守卫 sanity 真实命中；矩阵 residual 恒 blocked-env；
- 未触碰 Go/迁移/voiceRoomFor/miniprogram；未派发子代理；未推送远端；工作区终态干净（除报告文件 untracked）。

---

# 修复轮 1 报告：平台 API 守卫补 ESM 行为符号扫描

**审查发现（important）**：计划原守卫正则 `/require\(['"]react-native['"]\)/` 只匹配 CJS require 形态，ESM 行为性导入（`import { Platform } from 'react-native'` 等）完全绕过扫描——全局约束「平台 API（Platform/BackHandler/AppState 等）只允许在 adapters/（Task 7 源级扫描强制）」对最惯用写法形同虚设。现状无违规（未来漂移盲区），代码与计划逐字一致属计划自带缺陷。

**修复路线**：采纳「加 ESM 行为符号扫描」——落实计划 Global Constraints 的本意（约束对象是平台 API 符号，require 只是先例形态之一）。计划文档 `plan-t70.md` 不属本任务授权文件，未回写；本修复落在授权文件 `android-parity.test.ts` 内。

## 修复实现（commit `322c57d01`，1 file / +39/-1）

`apps/mobile/src/android-parity.test.ts`：

1. 模块级常量 `PLATFORM_SYMBOLS = ['Platform', 'BackHandler', 'AppState', 'PermissionsAndroid', 'NativeModules', 'NativeEventEmitter', 'Linking']`（计划点名符号 + 明确无争议的命令式平台 API；UI 组件不在此列，screens 合法）；
2. 原 require 正则提取为 `REACT_NATIVE_REQUIRE_RE`（行为不变）；新增 `PLATFORM_SYMBOL_IMPORT_RE = import\s*\{[^}]*\b(?:…符号…)\b[^}]*\}\s*from\s*['"]react-native['"]`（`[^}]*` 含换行覆盖多行 import；`import type {` 天然不被命中——type-only 导入不引入运行时平台行为，保守不抓）；
3. 主测试改为双形态命中：`REACT_NATIVE_REQUIRE_RE.test(source) || PLATFORM_SYMBOL_IMPORT_RE.test(source)`；
4. 新增第 4 个守卫自证测试 `the platform-symbol scanner actually fires on ESM behavioral imports`：5 条 mustFire 样本（Platform/BackHandler/AppState+View 混合/多行 PermissionsAndroid/Linking+NativeModules+NativeEventEmitter）+ 4 条 mustNotFire 样本（UI 组件两种、type-only、非 react-native 来源）——把「扫描器对 ESM 形态有效且不误报」钉进回归。

### 现状实查（修复前先做，证明零误报边界）

- `grep -rEn "from 'react-native'|from \"react-native\""` → 20 个文件（screens 14 + app 路由 5 + composition.ts 1），`sort -u` 符号清单仅 7 种 UI 组件组合（Button/Image/ScrollView/Text/View/TextInput/Switch）——零平台 API 符号，与审查「现状无违规」一致；
- 无 `import * as … from 'react-native'`、无多行 `import {`…`} from 'react-native'` 形态（grep 实查 exit 1）。

## TDD 证据（修复轮，重建环境重放）

### RED

命令：`pnpm exec tsx --test apps/mobile/src/android-parity.test.ts`（`PLATFORM_SYMBOL_IMPORT_RE` 初值与旧守卫等效——忠实复刻当前守卫能力，失败归因于真实盲区）

```
ok 1 - platform APIs stay inside src/adapters (验收标准 2：平台差异仅在 Adapter)
not ok 2 - the platform-symbol scanner actually fires on ESM behavioral imports (修复轮 1：CJS-only 盲区)
    守卫必须命中 ESM 平台符号导入（否则最惯用写法绕过扫描）：import { Platform } from 'react-native';
ok 3 - screens and routes never import wire clients or shared contracts (module-seams §10)
ok 4 - the Android acceptance matrix covers exactly the seven core workflows with real evidence paths (验收标准 3)
# tests 4
# pass 3
# fail 1
EXIT=1
```

失败断言 `import { Platform } from 'react-native';  false !== true` 即审查盲区本体的直接复现。

### GREEN

`PLATFORM_SYMBOL_IMPORT_RE` 替换为 ESM 符号正则后，同命令 →

```
ok 1 - platform APIs stay inside src/adapters (验收标准 2：平台差异仅在 Adapter)
ok 2 - the platform-symbol scanner actually fires on ESM behavioral imports (修复轮 1：CJS-only 盲区)
ok 3 - screens and routes never import wire clients or shared contracts (module-seams §10)
ok 4 - the Android acceptance matrix covers exactly the seven core workflows with real evidence paths (验收标准 3)
# tests 4
# pass 4
# fail 0
EXIT=0
```

主测试（测试 1）在全 src 双形态扫描下保持绿——扩大后的守卫对现状零误报。

## 回归（修复轮，重建环境实跑）

- `pnpm --filter @weknora/mobile test` → `244 tests / 233 pass / 0 fail / 11 skip`，TEST_EXIT=0（初版 243/232 + 守卫自证测试 1 个）；
- `pnpm --filter @weknora/mobile typecheck` → exit 0。
- （mobile-core dictation 未动，未重跑；初版证据见上文。）

## 提交（修复轮）

`322c57d01` — fix(mobile): 平台 API 守卫补 ESM 行为符号扫描——消除 CJS-only 盲区（#70 修复轮 1）——显式路径 `git add apps/mobile/src/android-parity.test.ts`，1 file / +39/-1。

## 环境事件记录（如实，影响本修复轮的执行路径）

1. **worktree 被清理**：初版提交 `6b09d1a3b` 后，任务 worktree `.worktrees/issue30-sweep-t70` 在修复轮执行中途被外部移除（主控已将 t70 合并进集成分支 `codex/issue30-mobile-office` @ `2d811d6e4` 并清理现场），本会话的 Bash 工具因 cwd 消失报 `spawn /bin/zsh ENOENT`、文件读取报不存在；修复轮首次提交尝试被 Mimosa hook 以 `internal/types/vectorstore_test.go` 既有硬编码凭据（历史提交 `5cf093706` 引入，非本任务文件、非本任务改动）强制拦截，随后未提交的修复改动随 worktree 删除而丢失。
2. **自愈**：`git worktree add .worktrees/issue30-sweep-t70 codex/issue30-t70` 重建任务 worktree（分支 HEAD `6b09d1a3b` 未受影响，无他人改动被触碰）→ `pnpm install --prefer-offline` 恢复依赖 → 逐字重放修复三步编辑 → RED/GREEN/全量回归在重建环境全部重跑（上文输出即重建环境的真实输出）→ 提交 `322c57d01`（本次 Mimosa 为 noBufs 兼容放行，未重现拦截）。
3. **遗留事实**：`internal/types/vectorstore_test.go` 的硬编码凭据发现属仓库既有内容且不在本任务授权文件内，本任务未修复也未绕过 hook——按边界规则留主控处置；本报告不对项目整体安全性做任何宣称。

## 自检发现（修复轮）

- 修复后守卫对两类形态（CJS 整包 require + ESM 平台符号导入）均有效，且有 mustFire/mustNotFire 自证测试钉住扫描器本身——Review Focus 4（防扫描器失效静默通过）从「只防 CJS 失效」升级为「双形态都防」；
- 符号表以计划点名三符号为核、按「实查零误报」边界扩展，未纳入 UI 命令式符号（StatusBar/Keyboard 等）避免过度设计；
- 分支 `codex/issue30-t70` 领先集成分支一个提交（`322c57d01`），待主控再次合并。

## 疑虑 / Concerns

- 无功能性疑虑。提请主控知悉：①t70 分支在集成 merge（`2d811d6e4`）之后新增修复提交 `322c57d01`，需再合并一次；②`internal/types/vectorstore_test.go` 既有凭据发现曾触发 Mimosa 强制拦截（本轮放行属扫描波动），需要专门任务处置。
