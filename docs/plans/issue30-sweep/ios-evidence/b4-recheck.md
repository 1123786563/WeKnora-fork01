# iOS 模拟器复验报告 — B4 集成增量复验（issue30-sweep）

- 日期：2026-09-25 15:06–15:14（本地）
- 执行者：iOS 复验员-B4（动态工作流子代理）
- Worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep`
- 结论：**构建-安装-启动-截图全流程通过**；B4 涉及移动端路由在 Release 构建中全部可达、渲染健康、未授权 fail-closed 正确；无 JS 异常、无崩溃。

## 1. 环境

`ios_preflight` 全绿：

- macOS + 完整 Xcode 27.0 (27A266a)，`xcode-select` → `/Applications/Xcode.app/Contents/Developer`
- `simctl` 可用；11 台 iOS 27.0 模拟器可用
- UI 自动化后端：`idb unavailable`（build/run/screenshot 可用，tap/swipe/type 不可用）

工程已存在（无需 prebuild）：`apps/mobile/ios/WeKnora.xcworkspace`、scheme `WeKnora`、bundle id `com.weknora.mobile`（`xcodebuild -list -workspace` 与 `project.pbxproj` 确认）。

模拟器：复用已 booted 的 **iPhone 18 Pro**（UDID `0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC`，iOS 27.0）。

## 2. 增量构建

命令（在 `apps/mobile/ios/` 下执行，derivedDataPath 复用既有 `build/` 命中增量缓存）：

```
xcodebuild -workspace WeKnora.xcworkspace -scheme WeKnora \
  -sdk iphonesimulator -configuration Release \
  -derivedDataPath build \
  -destination 'id=0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC' build
```

结果：

- `** BUILD SUCCEEDED **`，exit 0，**耗时 774 秒（≈12.9 分钟）**，在 25 分钟时限内
- error：**0**；warning：4796（抽样核对均为第三方 Pods 源码告警，非 B4 应用代码引入；未逐条归因，见发现 F2）
- 产物：`build/Build/Products/Release-iphonesimulator/WeKnora.app`，`file` 确认 universal（arm64 + x86_64）
- 完整日志：`b4-recheck/build.log`

## 3. 安装 / 启动 / 日志

```
xcrun simctl install booted .../Release-iphonesimulator/WeKnora.app   # INSTALL_OK
xcrun simctl launch booted com.weknora.mobile                          # PID 85161
```

- 启动成功，进程持续存活（`launchctl list` 可见 `UIKitApplication:com.weknora.mobile`，PID 85161）
- 启动日志错误筛查（`log show --predicate 'process == "WeKnora"'`，两轮：3 分钟增量 + 8 分钟全量）：
  - `launch-errors-full.log`（未处理 JS 异常/崩溃/fatal/TypeError 等关键词）：**0 行命中**
  - `launch-errors.log` 仅命中无害系统 XPC 噪音（`UIIntelligenceSupport`、`PointerUI`、`bootstrap look-up`，均为模拟器系统服务，与 B4 无关）

## 4. 截图证据（均存于 `ios-evidence/b4-recheck/`）

首屏为登录页；B4 核心路由经 `simctl openurl`（expo-router scheme `weknora://`，`Info.plist:28-32` 注册）逐页导航截图。未登录态下所有授权门控页面呈现一致的 fail-closed 空态，无红屏、无崩溃：

| 文件 | 页面 | 实测呈现 | B4 关联 |
|---|---|---|---|
| `01-home-first-screen.png` / `01b-home-retry.png` | `/` 首屏 | 首帧 JS bundle 加载白屏（约 8–10 秒），随后渲染 "Sign in to WeKnora" 登录页（部署源 + Email/Password + SSO） | 入口 |
| `02-tasks-list.png` | `/tasks` | 「请先登录并激活空间，再查看任务列表。」 | 任务中枢 |
| `03-task-detail-37-intervention.png` | `/tasks/detail?taskId=..&runId=..` | 「无法读取该任务 / 请先登录并激活空间，再打开任务详情。/ 重试」 | **#37** 干预承载页 |
| `04-task-budget-39.png` | `/tasks/budget?taskId=..` | 「任务预算」页 + 预算独立性声明 + 「请先登录并激活空间，再查看任务预算。」+ 暂无数据 + 追加按钮禁用 | **#39** 预算 |
| `05-ask-45-knowledge-qa.png` | `/ask` | 「Sign in to ask a knowledge question.」 | **#45** 知识问答 |
| `06-new-56-dictation.png` | `/new` | 「Sign in to create a task.」 | **#56** 录音转写入口 |
| `07-resources-60.png` | `/resources` | 「Sign in to browse tenant resources.」 | **#60** Available Agent/资源架 |

## 5. 发现

| # | 级别 | 发现 | 位置 |
|---|---|---|---|
| F1 | minor | 首帧白屏约 8–10 秒（Release JS bundle 加载完成前无 splash 内容），随后正常渲染登录页；属 RN Release 冷启动表现，无错误日志伴随 | 截图 `01` vs `01b` 时间戳 |
| F2 | minor | Release 构建告警 4796 条；抽样均为 Pods 第三方源码（非 B4 diff 引入），未逐条归因，不阻断构建 | `build.log` |
| F3 | info | 未登录态下各授权门控页面 fail-closed 空态呈现一致、文案明确，符合安全语义 | 截图 02–07 |

## 6. 无法验证项（如实声明）

以下需要**登录态 + 真实部署后端**，本次模拟器复验不可达，未验证：

- **#37** 干预三态（停止请求/停止确认/结果未知）实际呈现、冲突写阻止、revision 携带与 Run 回执绑定 —— 需授权会话与运行中 Run；
- **#39** 预算四读数、waiting_user/budget_exhausted 持久暂停与扩额恢复 —— 需授权会话与真实 sqlite 后端；
- **#40** 离线缓存/离线草稿/联网确认提交 —— 需授权会话与 Vault 快照；
- **#45** 证据信封三分类、实时撤权重校验 —— 需授权会话与真实 KB 后端；
- **#56** 录音→转写→确认提交流程 —— 需授权会话 + 麦克风交互；
- **#43/#48/#52/#60/#67** 对应管理端/发布/推送页面交互 —— 同上；
- 上述行为的移动端逻辑已由仓库内集成 smoke 测试覆盖（如 `task-intervention-integration-smoke.ts`、`task-budget-integration-smoke.ts`、`knowledge-qa-integration-smoke.ts`、`voice-dictation-integration-smoke.ts` 等），不在本次模拟器复验范围内，本次未运行。

不可达原因：`idb` UI 自动化后端不可用（preflight 实测），无法在登录表单点击/输入凭据；亦无可用的本地部署测试凭据。未以任何替代方式伪造上述交互证据。

## 7. 证据清单

```
docs/plans/issue30-sweep/ios-evidence/b4-recheck/
├── build.log                      # xcodebuild 完整日志（BUILD SUCCEEDED）
├── launch-errors.log              # 启动 3 分钟错误过滤（仅系统 XPC 噪音）
├── launch-errors-full.log         # 启动 8 分钟全量错误过滤（0 行）
├── 01-home-first-screen.png       # 首帧（JS 加载期）
├── 01b-home-retry.png             # 首屏登录页
├── 02-tasks-list.png
├── 03-task-detail-37-intervention.png
├── 04-task-budget-39.png
├── 05-ask-45-knowledge-qa.png
├── 06-new-56-dictation.png
└── 07-resources-60.png
```
