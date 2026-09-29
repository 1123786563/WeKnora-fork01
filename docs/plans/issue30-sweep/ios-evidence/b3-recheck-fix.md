# B3 iOS 复验问题修复报告（第二轮）

- 日期：2026-09-24
- worktree：`.worktrees/issue30-sweep`（分支 `codex/issue30-mobile-office`，基线 HEAD `98a7f98d4`）
- 输入：B3 复验第二轮报告（`ios-evidence/b3-recheck.md` §5）登记的 2 项 minor 问题
- 本轮证据目录：`ios-evidence/b3-recheck/fix/`（新证据文件后缀/前缀 `r2fix`、编号 05–07，与上轮同目录文件不冲突）
- 结论速览：问题 1 已修复并在模拟器重复验证（warm 深链 + 冷启动直达 `/tasks` 均出现 gate 文案，其余路由回归抽验不变）；问题 2 为环境/上游问题（无代码修复项），现状如实复现并记录。定向测试新增 1 用例，B3 全套 519 用例 0 fail；Release 构建 0 error。

## 问题 1：`/tasks` 根路由未授权态无 gate 文案（minor）

### 根因（复验报告已定位，本轮核实）

- 路由 `apps/mobile/src/app/tasks.tsx:5-12` 无条件委托 `MobileTasks`；
- `apps/mobile/src/composition.ts`（修复前 :242）在 `surface ≠ 'authorized' || !deployment || !identity?.userId` 时 `return null`——页面无任何内容，只剩布局层空白/spinner；
- 对照其余全部路由（`app/inbox.tsx:89-95`、`app/new.tsx:70-79`、`app/attention.tsx:12-16`、`app/tasks/detail.tsx:25-27`、`app/tasks/materials.tsx:15-17`、`screens/LegacyTasksScreen.tsx:27`）未授权分支均有「请先登录并激活空间，再…」口径的 gate 文案。

### 修复（最小改动，1 文件 + 定向测试）

落点选择：gate 放进 `MobileTasks` 的原 `return null` 分支（而非路由层直查快照）。理由：

1. **响应性**：`MobileTasks` 已通过 `useSyncExternalStore` 订阅 Runtime 快照（composition.ts:241）——页面驻留时登出/面切换会即时从列表切到 gate；路由层直查快照（`/inbox` 的既有模式）只在挂载时求值一次，覆盖不了「驻留中登出仍空白」的路径。
2. **单一事实源**：授权判定逻辑不动（同一分支条件），gate 挂在同一分支上，不引入第二处判定。
3. 路由文件 `tasks.tsx` 零变更。

变更内容（`apps/mobile/src/composition.ts`）：

- 新增 `import { Text, View } from 'react-native'`（:2）；
- `MobileTasks` 未授权分支由 `return null` 改为：

```ts
if (snapshot.surface !== 'authorized' || !snapshot.deployment || !snapshot.identity?.userId) {
  return createElement(View, null, createElement(Text, null, '请先登录并激活空间，再查看任务列表。'));
}
```

文案与既有 gate 家族同口径（「请先登录并激活空间，再查看行动通知。/…任务材料。/…历史任务。」）。导入方核查：`composition.ts` 的全部导入方为路由/屏幕文件（Expo 运行时或 app-smoke 桩环境，桩已提供 `View`/`Text`，`app-smoke.test.tsx:27`），新增 `react-native` 导入对 Node 测试环境安全。

### 定向单测（新增 1 用例）

`apps/mobile/src/app-smoke.test.tsx` 新增 `the /tasks root shell renders a sign-in gate instead of a blank page when unauthorized (B3 recheck)`：

- 断言 `app/tasks.tsx` 默认导出为函数；
- 渲染 `MobileTasks`（桩环境 Runtime 初始面为 `deployment-login`，`packages/mobile-core/src/runtime/mobile-runtime.ts:108`，即 `/tasks` 未授权态的真实产物）：
  - 文案含「请先登录并激活空间，再查看任务列表。」；
  - `TasksScreen` 不可达（未授权不得渲染列表）。

授权分支代码未变更，其行为由既有用例承载（`TasksScreen drives search, filters…`、`RuntimeSurface` 系列等）。

## 问题 2：MCP `ios_preflight` 仍报 ui backend `idb unavailable`（minor，环境/上游）

- 本轮现状复现（2026-09-24 22:28 实测）：`ios_preflight` 全项 OK，唯 `ui backend: backend=auto; idb unavailable`——与复验报告描述一致。
- 无代码修复项：双层根因在上轮 fix 报告（git 历史 `98a7f98d4` 版本本文件）已定位并落地可达边界——层 A（GUI 调度进程 PATH）已做 `launchctl setenv PATH` 宿主修复，生效需重启 ZCode（会终止工作流，未做）；层 B（fb-idb 1.6.1 不支持 `idb --version` 导致插件探测 exit 2）在 ZCode 应用包内，属上游，不改应用包。
- 覆盖不受影响：本轮全部 AX 校验继续经 idb CLI 直调 `/opt/homebrew/bin/idb ui describe-all --udid 0A38DB71-…`（可用，见下文证据）。

## 定向测试

- 新用例所在套件：`pnpm exec tsx --test "apps/mobile/src/app-smoke.test.tsx"` → `# tests 54 / # pass 54 / # fail 0`，新用例以 `ok 19` 通过。
- B3 全套口径（与 B3/复验报告同命令）：`pnpm exec tsx --test "packages/domain/src/mobile/*.test.ts" "packages/mobile-core/src/**/*.test.ts" "packages/api-client/src/mobile/*.test.ts" "apps/mobile/src/**/*.test.ts*"` → `# tests 519 / # pass 510 / # fail 0 / # skipped 9`（上轮基线 518/509/0/9，+1 即本轮新用例；skip 均为 opt-in integration 用例）。
- `pnpm run typecheck:mobile` → exit 0 PASS。

## 模拟器重复构建 + 启动验证（新证据 → `fix/`，编号 05–07 / 后缀 r2fix）

构建命令与复验轮相同（增量，同 `-derivedDataPath build`），日志 `fix/xcodebuild-release-r2fix.log`：

```
cd apps/mobile/ios && xcodebuild -workspace WeKnora.xcworkspace -scheme WeKnora \
  -configuration Release -sdk iphonesimulator \
  -destination "platform=iOS Simulator,id=0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC" \
  -derivedDataPath build ARCHS=arm64 ONLY_ACTIVE_ARCH=YES build
```

| 步骤 | 结果 | 证据 |
| --- | --- | --- |
| Release 增量构建 | **BUILD SUCCEEDED**，exit 0；`error:` 0、`warning:` 55（与上两轮同数，三方头文件类） | `fix/xcodebuild-release-r2fix.log` |
| 产物 | `main.jsbundle` 3,672,437 字节（上轮 3,672,281，+156 即 gate 变更） | — |
| 安装/启动 | `simctl install` OK；`simctl launch` → PID 88909 | — |
| **`/tasks` warm 深链**（修复目标场景） | `openurl "weknora:///tasks"` → 等 6s → AX：`WeKnora` + **「请先登录并激活空间，再查看任务列表。」**（复验轮同场景 AX 仅有 `WeKnora`，空白页） | `fix/05-tasks-root-gate.png` + `fix/ax-05-tasks-root-gate.txt` |
| **`/tasks` 冷启动直达** | terminate → `openurl "weknora:///tasks"` → 等 10s → AX 同上 gate 文案 | `fix/06-coldlaunch-tasks-gate.png` + `fix/ax-06-coldlaunch-tasks-gate.txt` |
| 回归抽验 `/inbox` | 「请先登录并激活空间，再查看行动通知。」与基线一致 | （本轮命令输出，未另存文件） |
| 回归抽验正常重启 | terminate → launch → 登录页 `Sign in to WeKnora / Sign in / Continue with single sign-on` | `fix/07-normal-relaunch-login.png` + `fix/ax-07-normal-relaunch-login.txt` |
| 启动日志体检 | 1,358 行，`fatal/sigtrap/crash/RCTFatal` 关键字 **0**；error 行全部为 iOS 27 `com.apple.uiintelligencesupport` XPC 系统噪声（与上两轮结论一致） | `fix/app-launch-log-r2fix.txt` |

截图均为 1206×2622 有效 PNG（`sips` 校验）；`05`/`06` 目视确认 gate 文案渲染于页面顶部。

## 未验证 / 遗留项（如实登记）

1. **授权态（登录后）行为**：模拟器无 deployment 凭据/可达后端，与上两轮同为未授权 gate 验证；授权态语义由 TS 测试（519/510/0/9）与 opt-in 集成证据承载，不在模拟器可达范围。
2. `ios_ui_*` MCP 工具在当前会话仍报 unavailable（问题 2，见上）；UI 校验全部经 idb CLI + 截图完成。
3. `MobileTasks` 授权分支在单测中不可直接驱动（runtime 单例无注入缝）——该分支代码本轮零变更，未新增测试，属既有覆盖（TasksScreen/RuntimeSurface 用例）。

## 结论

问题 1 以最小改动（`composition.ts` 单文件 + 1 定向用例）闭合：`/tasks` 未授权态从空白页变为与其余路由同口径的登录 gate，warm 深链与冷启动直达两个场景均在模拟器重复验证，回归抽验（`/inbox`、正常重启）与启动日志体检无异常；B3 全套测试 519 用例 0 fail、typecheck 通过。问题 2 无代码修复项，现状复现与可达边界已如实记录（上轮已定位根因并落地宿主侧修复，剩余动作需重启 ZCode 与上游修复，超出本轮）。
