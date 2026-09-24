# B3 iOS 复验报告（增量构建 + 启动 + B3 页面导航）

- 日期：2026-09-24
- worktree：`.worktrees/issue30-sweep`（分支 `codex/issue30-mobile-office`，HEAD `13a761d2a`）
- 被测工程：`apps/mobile/ios/WeKnora.xcworkspace`（Expo 55 / RN 0.83.10 / 新架构关闭，由首轮修复锚定）
- 模拟器：iPhone 18 Pro（UDID `0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC`，iOS 27.0）
- 证据目录：`docs/plans/issue30-sweep/ios-evidence/b3-recheck/`
- 环境预检（`ios_preflight`）：macOS / Xcode 27.0 (27A266a) / simctl / 11 台模拟器全部 OK；MCP ui backend 报 `idb unavailable`，但 `idb` CLI 实际在 PATH（`/opt/homebrew/bin/idb` + `idb_companion`），本轮直接用 `idb ui describe-all` 做 AX 校验，用 `xcrun simctl` 做安装/启动/openurl/截图。

## 1. Release 增量构建

命令（在 `apps/mobile/ios` 下，与首轮实测同参数以复用 `ios/build` 增量产物）：

```
xcodebuild -workspace WeKnora.xcworkspace -scheme WeKnora -configuration Release \
  -sdk iphonesimulator \
  -destination "platform=iOS Simulator,id=0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC" \
  -derivedDataPath build ARCHS=arm64 ONLY_ACTIVE_ARCH=YES build
```

- 结果：**BUILD SUCCEEDED**，耗时约 105 秒（增量；首轮干净构建参照 `ios-evidence/xcodebuild-release.log` 的同 derivedDataPath）
- 日志：`b3-recheck/xcodebuild-release.log`
- 过程说明：第一次未带 `ARCHS=arm64 ONLY_ACTIVE_ARCH=YES` 会触发 Pods 双架构重编译，已中止并以与首轮一致的参数重跑（真实增量）。
- 警告统计：无 `error:`；全部 `warning:` 来自第三方（React-Fabric 头文件 `-Wdocumentation-deprecated-sync`、react-native-gesture-handler `-Wunused-function`、expo-file-system nullability 等 Pods/node_modules 头），与本仓代码无关。
- 产物：`build/Build/Products/Release-iphonesimulator/WeKnora.app`（`main.jsbundle` 5.47MB，20:14 生成）

## 2. 安装与启动

```
xcrun simctl install 0A38DB71-… …/Release-iphonesimulator/WeKnora.app   # INSTALL OK
xcrun simctl launch 0A38DB71-… com.weknora.mobile                        # PID 57276
```

- 启动后 12 秒截首屏：`01-first-screen.png` — WeKnora 登录页（DeploymentLoginScreen）。
- AX 校验（`idb ui describe-all`）：`Sign in to WeKnora` / `Sign in` / `Continue with single sign-on` — 与首轮 `fix-round-1/02-clean-first-screen.png` 一致。

## 3. B3 核心页面（deep link 导航复验）

方法：warm deep link `xcrun simctl openurl <UDID> "weknora:///<route>"` → 5 秒后 `idb ui describe-all` 读 AX 标签 + `simctl io screenshot`。每条路由做了两轮（第一轮截图、第二轮落盘 AX 文件），文案完全一致。未授权态（无 deployment 凭据/后端）渲染各自的 gate 文案——与路由源码 `apps/mobile/src/app/*` 的未授权分支一致。

| 路由 | Issue | AX 关键文案 | 截图 / AX 文件 |
| --- | --- | --- | --- |
| `/inbox` | #38 收件箱读模型 | 「请先登录并激活空间，再查看行动通知。」 | `02-inbox.png` / `ax-inbox.txt` |
| `/new` | #36 统一 New 入口 | "Sign in to create a task." | `03-new.png` / `ax-new.txt` |
| `/tasks/legacy` | #44 Legacy Task 投影 | "Legacy tasks" / "Old chat sessions appear here with the same task identity. Only history-provable facts are shown." / Reload / 「请先登录并激活空间，再查看历史任务。」 | `04-tasks-legacy.png` / `ax-tasks-legacy.txt` |
| `/tasks/materials` | #46 Task Material Interface | 「任务材料」「刷新材料」「尚无材料索引」「返回任务」 | `05-tasks-materials.png` / `ax-tasks-materials.txt` |
| `/attention` | #41 行动通知 | 「请先登录并激活空间，再打开收件箱。」 | `06-attention.png` / `ax-attention.txt` |
| `/resources` | #59 Resource Shelf | "Sign in to browse tenant resources."（与首轮 round-1 证据一致） | `07-resources.png` / `ax-resources.txt` |
| `/tasks/detail` | #42/#44 任务详情 | 「无法读取该任务」「请先登录并激活空间，再打开任务详情。」「重试」——未授权+无 taskId 的错误分支不崩溃 | `10-tasks-detail.png` / `ax-tasks-detail.txt` |

冷启动 deep link（SceneDelegate `willConnectTo` 转发链路）：

```
simctl terminate → simctl openurl "weknora:///inbox" → 等 10s
```

- AX：「请先登录并激活空间，再查看行动通知。」— 冷启动直达 inbox ✅ → `08-coldlaunch-inbox.png`
- 正常重启（terminate → launch）回到登录屏，deep link 状态不粘连 ✅ → `09-normal-relaunch-login.png` / `.txt`

## 4. 启动日志

命令：`xcrun simctl spawn <UDID> log show --last 6m --predicate 'process == "WeKnora"' --style compact` → `app-launch-log.txt`（620 行，覆盖两次进程 57276 / 59000）。

- 无 SIGTRAP / 无 fatal / 无崩溃 / 无 Hermes / RCTFatal / JS 异常关键字。
- 8 行含 "error" 的行全部为 iOS 27 模拟器系统级 XPC 噪声（`com.apple.uiintelligencesupport.agent` 连接失败、`PointerUI` non-launching port、bootstrap look-up），对所有 app 通用，与 WeKnora 业务代码无关。

## 5. 无法验证项（与首轮相同的环境边界）

1. **授权态交互**（#32/#34/#35/#36/#38/#41/#42/#44/#46/#59 的登录后完整业务流）：无 deployment 凭据与后端服务，模拟器上只能验证未授权 gate 文案与路由可达性；这些 Issue 的业务验收依赖仓库内 Go 集成测试 / opt-in 真实 HTTP 测试（B3 已提交），不在本轮 iOS 模拟器复验范围。
2. **MCP iOS UI 后端**：`ios_ui_*` 工具因 MCP 进程 PATH 找不到 idb 而不可用；本轮以 `idb` CLI 直接调用替代（tap 未使用，导航全部经 deep link 完成）。

## 6. 结论

B3 集成后的移动端在 iOS 27 模拟器上：Release 增量构建通过、安装启动正常、首屏登录页正常渲染、全部 7 条 B3 相关路由（含冷启动 deep link）可达且各自渲染正确的未授权 gate 文案、无崩溃与 JS 错误。与首轮实测（`ios-evidence/ios-test-report.md` + `fix-report-round-1.md`）行为一致，未发现回归。
