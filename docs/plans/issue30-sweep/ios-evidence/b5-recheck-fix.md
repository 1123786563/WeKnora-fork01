# B5 iOS 复验问题修复报告（issue30-sweep）

- 修复员：B5iOS修复员（动态工作流子代理）
- 日期：2026-09-28 02:40–03:05
- Worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep`，分支 `codex/issue30-mobile-office`（起点 `eee4317af`）
- 输入：[b5-recheck.md](b5-recheck.md) §6 四项发现（1 important + 3 minor）
- 新证据目录：[b5-recheck/fix/](b5-recheck/fix/)；本报告随修复一并提交

## 1. 发现 → 修复对照

### 1.1 [important] 遗留 iOS 工程静默缺 B5 原生模块的增量构建陷阱 → 已固化为机器守卫

复验结论是「T39 脚本的 prebuild 步骤属正确防线，但缺一道显式校验」。本次把该防线补成**构建管线内的硬门**：

**新增（可测模块，布局沿用 mimosa 裁决 3：读取/逻辑在 src/ 直测，scripts/ 仅薄壳）：**

| 文件 | 内容 |
|---|---|
| `apps/mobile/src/ios-native-deps.ts`（新，76 行） | 纯函数：`parseAutolinkingResolution`（解析 `expo-modules-autolinking resolve --platform ios --json`）、`podsSectionOf`/`installedPodNames`（只认 Podfile.lock **PODS 节**两空格缩进条目——DEPENDENCIES/SPEC CHECKSUMS 的「声明过」不算「装上了」）、`nativePodGaps`（autolinking 解析出的每个原生 pod 必须在 PODS 节，缺口即失败） |
| `apps/mobile/src/ios-native-deps-cli.ts`（新，46 行） | CLI 主体：跑 autolinking resolve（node_modules 现状）→ 读 `ios/Podfile.lock` → 判定；缺口打印 `包名 -> pod` 清单并 exit 1，全绿打印 `IOS_NATIVE_DEPS_OK modules=21 pods=22`。无 argv 路径输入 |
| `apps/mobile/scripts/verify-ios-native-deps.ts`（新，6 行） | 薄壳（同 `emit-acceptance-record.ts` 模式） |

**接入**：`apps/mobile/scripts/ios-release-build.sh:47-53` 新增第 3.5 步，`pnpm exec tsx scripts/verify-ios-native-deps.ts` 夹在 `pod install` 与 `xcodebuild` 之间——把「20 分钟静默错包」挡在编译之前。

**三重实证**：

1. **定向单测**（`apps/mobile/src/ios-native-deps.test.ts`，新，7 用例全绿）：用 B5 陷阱现场 fixture（lock 无 ExpoAudio/ExpoNetwork）断言守卫精确点名 `expo-audio -> ExpoAudio`、`expo-network -> ExpoNetwork`；完整 lock fixture（行格式取自本仓库真实 `ios/Podfile.lock:33/:87/:82`）零缺口；DEPENDENCIES 节的 ExpoAudio「诱饵」不得骗过守卫；带引号 subspec 行不算基础 pod。
2. **现场演示**（真树、非破坏）：把 `ios/Podfile.lock` 临时替换为剔除 ExpoAudio/ExpoNetwork 的副本后运行 CLI → `stale Podfile.lock: 2 native pod(s) ... expo-audio -> ExpoAudio / expo-network -> ExpoNetwork`，exit 1；随后逐字节恢复（`diff -q` 通过，恢复后 `Podfile.lock:33/:87` 原样）。
3. **管线内实跑**：修复后全管线重放（§3），`00-release-build.log:131` 记录 `IOS_NATIVE_DEPS_OK modules=21 pods=22`。

### 1.2 [minor] 任务前提「已生成并提交 apps/mobile/ios 工程」与 git 现状不符 → 仓库侧事实源已改写

「任务书」本体是波级编排器的外部输入，仓库内不可编辑；本次把所有仓库侧事实源改为与现实一致：

- `apps/mobile/scripts/ios-release-build.sh:8-12` 头部新增 ⚠️ 段：ios/ 工程永不入库（`8d39806f4` 曾提交、`80fc3648a` 移除，`git ls-files apps/mobile/ios` 恒 0）、**iOS 构建唯一受支持入口是本脚本**、后续批次任务书不得再写「ios 工程已生成并提交」。
- `apps/mobile/src/scripts/ios-acceptance-scripts.test.ts` 新增契约断言：守卫必须夹在 pod install 与 xcodebuild 之间（防止未来改动把 3.5 步挪丢）。
- 本报告即给后续批次的更正记录。`docs/plans/issue30-sweep/plans/plan-t70.md:52` 此前已独立记录同一更正，无需改动。

### 1.3 [minor] watchman 初始 crawl 挤压构建预算 → 脚本预热 + 热.watch 实测收益

`apps/mobile/scripts/ios-release-build.sh:21-27` 新增第 0 步：best-effort `watchman watch-project "$ROOT"`（后台、失败不阻塞），让主仓库根（`.worktrees/` 23G/26 worktree）的初始 crawl 与 prebuild/pod install 并行，并把「验收机预热 watchman」写进脚本注释。

实测（§3）：修复后全管线重放总耗时 **2:03**（`00-release-build.log` 末行 `2:03.25 total`），全程 **0 条** "Waiting for Watchman"（复验时 33,415 行起连续等待 ~15 分钟）。归因如实说明：本次为热 watch（复验 02:26 crawl 完成后 watch 常驻）+ 预热步骤双因素叠加；冷机场景预热只能把 crawl 挪出计时并与前期步骤并行，crawl 本身仍需在验收机预热或配置 watchman 忽略项（后者会让 metro 在 worktree 内的文件查询失效，不能盲配，故未默认开启）。

### 1.4 [minor] 本机无 Simulator.app GUI → headless 属性写进验收脚本

`apps/mobile/scripts/ios-acceptance-run.sh:7-9` 头部注明：全程仅 simctl CLI（boot/install/launch/openurl/io），不依赖 Simulator.app GUI，本管线在此类机器（MCP `ios_boot_simulator` 报 "Unable to find application named 'Simulator'"）照常可用。本次模拟器复验（§3）继续以 simctl CLI 完成，功能等效。

## 2. 修复后定向测试（全部实际执行）

| 命令（cwd=apps/mobile） | 结果 |
|---|---|
| `pnpm exec tsx --test src/ios-native-deps.test.ts` | **7/7 pass**（新增定向单测） |
| `pnpm exec tsx --test src/scripts/ios-acceptance-scripts.test.ts` | **3/3 pass**（含新增 2 断言；期间暴露并修正一次回归：我初版把 xcodebuild 改成绝对路径，破坏既有 `-derivedDataPath build` 契约断言，已回退为原调用形态） |
| `pnpm run typecheck`（tsc --noEmit） | **通过**（无输出） |
| `pnpm test`（全量） | **289 tests / 275 pass / 0 fail / 14 skip**（skip 为既有 opt-in 集成 smoke） |
| `bash -n apps/mobile/scripts/ios-release-build.sh` 与 `ios-acceptance-run.sh` | **SYNTAX_OK** |

## 3. 模拟器重复构建启动验证（修复管线全流程实跑）

**命令**：`bash apps/mobile/scripts/ios-release-build.sh /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep`（即 T39 固化管线本体，含本次全部修复）。日志 [fix/00-release-build.log](b5-recheck/fix/00-release-build.log)。

- 第 1 次尝试在 `pod install` 因 glog 0.3.5 的 GitHub shallow clone 网络闪断失败（`Connection reset by peer`），如实留档 [fix/00-release-build-attempt1-glog-network-fail.log](b5-recheck/fix/00-release-build-attempt1-glog-network-fail.log)；glog 为 git 源 pod，重试即过（环境性，非工程缺陷）。
- 第 2 次：prebuild → pod install（106 pods complete）→ **`IOS_NATIVE_DEPS_OK modules=21 pods=22`（:131）** → `** BUILD SUCCEEDED **`（:7008）→ `RELEASE_APP=...WeKnora.app`（:7010），总耗时 **2:03.25**。

**产物核验**（与复验基线逐项一致）：

| 验收点 | 本次实测 | 复验基线 |
|---|---|---|
| `nm WeKnora \| grep -c ExpoAudio` | **1414** | 1414 |
| `nm WeKnora \| grep -c ExpoNetwork` | **83** | 83 |
| `NSMicrophoneUsageDescription` | "WeKnora uses the microphone for voice dictation and live voice sessions." | 同 |
| 链接标志 `-lExpoAudio/-lExpoNetwork` | 构建日志 4 处命中 | 同 |

**模拟器流程**（iPhone 18 Pro iOS 27.0，UDID 0A38DB71-…85FC，全程 simctl CLI）：干净卸载 → `simctl install`（INSTALL_OK）→ `simctl launch`（pid 27893）→ 11s 后首屏截图 → 四条 `weknora://` 深链探针各截图。进程存活（launchctl 列表含 com.weknora.mobile）。

**截图证据**（[b5-recheck/fix/](b5-recheck/fix/)，视觉模型逐字复核 + 与复验基线像素 diff）：

| 截图 | 路由 | 实测内容 |
|---|---|---|
| 01-first-screen.png | 冷启动首屏 | DeploymentLoginScreen："Sign in to WeKnora"、weknora.example.com、Email/Password |
| 02-voice-room.png | `weknora://tasks/voice?taskId=recheck-demo` | 语音房结构性空态：「当前部署或设备不支持语音房（需要麦克风与授权通道）。」+ 顶部常驻「语音确认的文字以『调整指令』进入任务时间线；高风险审批只能在行动收件箱完成，语音无法代替审批。」+「开始说话/离开语音房」 |
| 03-task-detail.png | `weknora://tasks/detail?...` | 「无法读取该任务 / 请先登录并激活空间，再打开任务详情。」+ 重试（B2-F13 分支） |
| 04-inbox.png | `weknora://inbox` | 「请先登录并激活空间，再查看行动通知。」（inbox.tsx:92 原文一致） |
| 05-tasks.png | `weknora://tasks` | 「请先登录并激活空间，再查看任务列表。」（composition.ts:298 原文一致，且 app-smoke.test.tsx:562 已断言该文案） |

**像素级佐证**（PIL ImageChops）：五张截图与复验基线（04/06/07/08/09）逐一 diff，差异 bbox 全部**仅落在状态栏时钟区（约 171-319 × 77-117）**——修复管线渲染结果与复验逐像素一致。

**启动日志**（[fix/06-launch-log-full.txt](b5-recheck/fix/06-launch-log-full.txt)）：B4 同款崩溃门 `grep -cE 'fatal|uncaught|NSException|SIGTRAP|SIGSEGV'` → **FATAL_LOG_HITS=0**。error 级条目仅 UIKitCore `statusBarStyle` 弃用与 QuartzCore handler 噪音共 29 行（[fix/06-launch-errors-deprecation-noise.log](b5-recheck/fix/06-launch-errors-deprecation-noise.log)，全部系统噪音，无应用级错误）。

## 4. 顺带更正一处复验证据误读（非缺陷，不改代码）

复验报告 §3.1 对 09-tasks.png 的转述「请开始新的任务或从行动通知进入。」与其自身像素不符：该图与本次 05-tasks.png 除时钟区外逐像素一致，且真实文案为「请先登录并激活空间，再查看任务列表。」（composition.ts:298，编译产物 main.jsbundle 内同串，app-smoke.test.tsx:562 有断言）。另复验的 08-inbox-crop.png 与 09-tasks-crop.png MD5 相同（387ba72c…，同一文件被复制两份），09 的放大证据实际是 inbox 的裁剪。行为本身正确，仅证据转述有误，在此如实更正。

## 5. 结论

四项发现全部处置：important 陷阱固化为 pod install 后、xcodebuild 前的机器硬门（单测 + 现场演示 + 管线内实跑三重实证）；三项 minor 分别以脚本头部事实源改写、watchman 预热、headless 说明落档。修复后全管线（含守卫）2:03 构建成功，产物与复验基线逐项一致（1414/83 符号、麦克风文案、链接标志），模拟器冷装启动 + 5 页面渲染逐像素复现基线、崩溃门 0 命中。全量 mobile 测试 289/275 pass/0 fail、typecheck 通过。
