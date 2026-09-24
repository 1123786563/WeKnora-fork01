# B3 iOS 复验（第二轮·增量）

- 日期：2026-09-24 22:11–22:17 CST
- worktree：`.worktrees/issue30-sweep`（分支 `codex/issue30-mobile-office`，HEAD `8deac8915`；自上轮构建 `aa51eeeaf` 后仅有 2 个 docs 提交，`apps/mobile`/`packages/**` 源码零变更）
- 本轮证据目录：`ios-evidence/b3-recheck/r2/`（上一轮证据保留在 `b3-recheck/` 根与 `b3-recheck/fix/`，上轮报告正文见 git 历史 `01c9ccf5c` 版本与本文件同路径）
- 环境：Xcode 27.0 (27A266a)、iOS 27.0 runtime、iPhone 18 Pro 模拟器 UDID `0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC`（复用已 booted 实例）；`ios_preflight` 全项 OK，唯 ui backend 仍报 `idb unavailable`（已知上游问题，见下）
- 结论速览：Release 增量构建 30 秒 BUILD SUCCEEDED（0 error）；安装/启动正常；8 条 B3 路由 + 冷启动深链 + 正常重启 + 2 条错误深链全部与上轮基线一致或符合安全预期；启动日志 0 fatal/crash。**无回归。**

## 1. 增量构建（任务指定命令，同上轮 derivedDataPath）

```
cd apps/mobile/ios && xcodebuild \
  -workspace WeKnora.xcworkspace -scheme WeKnora \
  -configuration Release -sdk iphonesimulator \
  -destination "platform=iOS Simulator,id=0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC" \
  -derivedDataPath build ARCHS=arm64 ONLY_ACTIVE_ARCH=YES build
```

| 项 | 结果 |
| --- | --- |
| 结果 | **BUILD SUCCEEDED**，exit 0，耗时 29.96s（63.5s user / 241% cpu，远低于 25 分钟停止线） |
| error | 0（`grep -c 'error:'`） |
| warning | 55（与上轮两轮完全同数，React-Fabric/gesture-handler/expo 三方头文件类，非 B3 代码） |
| 产物 | `build/Build/Products/Release-iphonesimulator/WeKnora.app/main.jsbundle` 3,672,281 字节（22:12 生成，与 fix 轮逐字节同大小） |
| 日志 | `r2/xcodebuild-release.log` |

## 2. 安装 / 启动 / 日志

| 步骤 | 命令 | 结果 |
| --- | --- | --- |
| 安装 | `xcrun simctl install <UDID> <WeKnora.app>` | OK |
| 日志流 | `simctl spawn <UDID> log stream --predicate 'process == "WeKnora" …'` 后台采集全程 | `r2/app-launch-log.txt` 954 行 |
| 启动 | `xcrun simctl launch <UDID> com.weknora.mobile` | PID 48083 |
| 启动日志体检 | `grep -ciE "fatal\|sigtrap\|crash\|rctfatal"` | **0**；全部 error 行为 iOS 27 `com.apple.uiintelligencesupport`/`BoardServices:PointerUI` 系统 XPC 噪声（对所有 app 通用，与上轮结论一致），WeKnora 自身无 error |

## 3. B3 路由矩阵（warm deep link：`simctl openurl "weknora:///<route>"` → 等 5–10s → `idb ui describe-all` AX + `simctl io screenshot`）

未授权态（模拟器无 deployment 凭据/后端，与上轮同口径）——每页 AX 文案与上轮基线逐条一致：

| 路由 | 覆盖 Issue | 本轮 AX（gate/骨架文案） | 与上轮 | 证据（r2/） |
| --- | --- | --- | --- | --- |
| `/`（首屏冷启动） | — | Sign in to WeKnora / Sign in / Continue with single sign-on | 一致 | `01-first-screen.png` |
| `/inbox` | #38 行动通知收件箱 | 「请先登录并激活空间，再查看行动通知。」 | 一致 | `02-inbox.png` + `ax-02-inbox.txt` |
| `/new` | #36 统一 New 入口 | "Sign in to create a task." | 一致 | `03-new.png` + `ax-03-new.txt` |
| `/tasks/legacy` | #44 Legacy Task 投影 | Legacy tasks / "Old chat sessions appear here with the same task identity. Only history-provable facts are shown." / Reload / 「请先登录并激活空间，再查看历史任务。」 | 一致 | `04-tasks-legacy.png` + `ax-04-tasks-legacy.txt` |
| `/tasks/materials` | #46 Task Material | 返回任务 / 任务材料 / 刷新材料 / 尚无材料索引 / 「请先登录并激活空间，再查看任务材料。」 | 一致 | `05-tasks-materials.png` + `ax-05-tasks-materials.txt` |
| `/attention` | #38/#42 决定入口 | 「请先登录并激活空间，再打开收件箱。」 | 一致 | `06-attention.png` + `ax-06-attention.txt` |
| `/resources` | #59 agents 投影 | "Sign in to browse tenant resources." | 一致 | `07-resources.png` + `ax-07-resources.txt` |
| `/tasks/detail` | #44/#46 详情 | 无法读取该任务 / 「请先登录并激活空间，再打开任务详情。」/ 重试 | 一致 | `10-tasks-detail.png` + `ax-10-tasks-detail.txt` |

## 4. 深链专项（#41）

| 场景 | 步骤 | 结果 | 证据 |
| --- | --- | --- | --- |
| 冷启动直达 inbox | terminate → `openurl "weknora:///inbox"` → 等 10s | 冷启动后直达 inbox gate「请先登录并激活空间，再查看行动通知。」（深链解析→重鉴权 gate 生效，未执行任何业务操作） | `08-coldlaunch-inbox.png` + `ax-coldlaunch-inbox.txt` |
| 正常重启 | terminate → `simctl launch` → 等 8s | 回到登录页（Sign in to WeKnora…），无未授权内容残留 | `09-normal-relaunch-login.png` + `ax-normal-relaunch.txt` |
| 错误深链 ① | `openurl "weknora:///unknown-page"` | Expo Router「Unmatched Route / Page could not be found / Go back」安全降级，不执行业务 | `11-bad-deeplink.png` |
| 错误深链 ② | `openurl "weknora:///tasks/detail?taskId=../../etc"`（路径穿越尝试） | 不解析、不崩溃，落到详情 gate「无法读取该任务 / 请先登录并激活空间…」拒绝执行 | `11-bad-deeplink.png` 采样前帧 |

## 5. 本轮发现

1. **（minor，观察非回归）`/tasks` 根路由未授权态无 gate 文案，仅布局层 spinner**：本轮误将上轮的 `/tasks/legacy` 写成 `/tasks`，两次采样（等 5s / 10s）AX 均只有 "WeKnora"，截图为空白页 + 居中 spinner。根因是源码现状：`apps/mobile/src/app/tasks.tsx:5-12` 未授权时直接渲染 `MobileTasks`，而 `apps/mobile/src/composition.ts:242` 在 surface≠authorized 时 `return null`，页面无内容；其余所有路由未授权分支都有明确 gate 文案。上轮未覆盖 `/tasks` 根路由，故非本轮回归；建议后续给 `/tasks` 补 gate 文案或跳转登录。证据：`r2/ax-tasks-root-null-observation.txt`（`/tasks` 两次采样）。
2. **（已知问题复现，非新发现）MCP `ios_preflight` 仍报 ui backend `idb unavailable`**：上轮 fix 报告已定位双层根因（GUI 调度进程 PATH 不含 `/opt/homebrew/bin` + fb-idb 1.6.1 不支持 `idb --version` 导致插件探测 exit 2），修复层 A 需重启 ZCode（会终止工作流，未做）、层 B 在 ZCode 应用包内属上游。本轮全程改用 `idb ui describe-all --udid …` CLI 直调（可用），不影响复验覆盖。

## 6. 无法验证项（如实登记）

- **授权态（登录后）的 B3 行为**：模拟器无 deployment 凭据/可达后端，本轮与上轮同为未授权 gate 验证；#36/#38/#41/#42/#44/#46/#59 的授权态语义由 B3 报告的 TS 测试（518/509/0/9）与 opt-in 真实 HTTP 集成证据承载，不在模拟器可达范围。
- **`ios_ui_*` MCP 工具**：idb 后端探测缺陷（见发现 2），未使用；UI 校验全部经 idb CLI + 截图完成。
- **多设备/真机**：无真机（blocked-env，与 #38 上轮口径一致）。

## 7. 截图清单（全部 1206×2622 有效 PNG，`sips` 校验）

`r2/01-first-screen.png`、`02-inbox.png`、`03-new.png`、`04-tasks-legacy.png`、`05-tasks-materials.png`、`06-attention.png`、`07-resources.png`、`08-coldlaunch-inbox.png`、`09-normal-relaunch-login.png`、`10-tasks-detail.png`、`11-bad-deeplink.png`
