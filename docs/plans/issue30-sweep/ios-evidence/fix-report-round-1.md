# iOS 修复报告 — 第 1 轮（fix round 1）

- 日期：2026-09-23
- 执行者：iOS 修复员（动态工作流子代理）
- 工作区：worktree `.worktrees/issue30-sweep`，分支 `codex/issue30-mobile-office`
- 输入：`docs/plans/issue30-sweep/ios-evidence/ios-test-report.md`（iOS 测试轮发现清单，5 项）
- 结论一览：**5 项发现全部处理完毕**。默认配置（仓库源码直接 `expo prebuild` → `pod install` → Release 构建 → iOS 27 模拟器安装启动）不再 SIGTRAP、首屏正常渲染；`weknora:///resources` deep link warm 与冷启动均能导航到 Resources 屏。回归证据：定向单测 64/64 通过（含本轮新增 5 个插件单测）、typecheck 通过、两次 Release 构建成功、三轮模拟器启动验证（进程存活 + 截图）。

## 0. 修复形态总览

apps/mobile 保持 **CNG（prebuild）流程**（`ios/` 目录从未入库，本轮起被 `.gitignore` 显式忽略），因此全部 iOS 侧修复收敛为一个 **本地 Expo config plugin** + 两个应用配置改动：

| 文件 | 改动 |
| --- | --- |
| `apps/mobile/plugins/ios-xcode27.js`（新增） | iOS 27 SDK 兼容插件：scene 生命周期改写、URL 转发、部署目标钳制、RCTNewArchEnabled 夺回 |
| `apps/mobile/app.json` | `newArchEnabled: true → false`；plugins 数组注册 `"./plugins/ios-xcode27"` |
| `apps/mobile/package.json` | devDependencies 显式加入 `babel-preset-expo: ~55.0.25`（+ `pnpm-lock.yaml` 同步） |
| `apps/mobile/src/plugins/ios-xcode27.test.ts`（新增） | 插件改写逻辑的定向单测（5 个用例） |
| `.gitignore` | 追加 `apps/mobile/ios/`（生成产物，防误提交） |

修复后从零生成流程（本轮实际执行）：`pnpm install` → `rm -rf ios && npx expo prebuild -p ios --no-install` → `cd ios && pod install` → `xcodebuild … Release`，全链无需任何手工补丁。

## 1. 发现逐项处置

### #1 critical — 默认配置启动即 SIGTRAP（UIScene 生命周期强制）✅ 已修复

- 根因（测试轮已定位）：iOS 27 SDK 强制 UIKit scene 生命周期，Expo SDK 55 的 `ExpoAppDelegate` 场景支持仍是 TODO（`expo` 包 `ios/AppDelegates/ExpoAppDelegate.swift:54` 一带，本轮复核确认无任何 scene 方法），模板 AppDelegate 为旧式 `UIWindow(frame:)`。
- 修复：插件 `withAppDelegate` 将模板 AppDelegate 改写为 scene 形态（锚定 SDK 55 模板原文做精确替换，模板漂移时抛错自曝）：
  - `didFinishLaunchingWithOptions` 只保存 launchOptions，RN 启动延迟到 `SceneDelegate.scene(_:willConnectTo:)`（`startReactNativeOnce(in:)`，幂等）；
  - `application(_:configurationForConnecting:)` 返回 `SceneDelegate` 配置；
  - `withInfoPlist` 注入 `UIApplicationSceneManifest`（`UIApplicationSupportsMultipleScenes=false`）。
- 回归证据：
  - 单测：`applySceneLifecycle` 改写断言（旧式 `UIWindow(frame:)` 消失、scene 配置/延迟启动/URL 转发齐全、幂等、模板漂移抛错）——`src/plugins/ios-xcode27.test.ts`，随套件通过（64/64）。
  - 模拟器：`xcrun simctl launch … com.weknora.mobile` → PID 29784 / 37336 / 40120 三轮启动进程全部存活（`launchctl list` 可见），日志 `app-launch-log.txt` 显示 WeKnora 以 `FBSceneManager sceneID:com.weknora.mobile-default` 完成 scene 设置，无 SIGTRAP、无 `UIScene life cycle is required` 运行时错误。

### #2 critical — Fabric（newArchEnabled:true）在 iOS 27.0 模拟器白屏 ✅ 已修复（以关闭新架构规避）

- 修复：
  - `app.json` `newArchEnabled: false`；
  - **关键补充**：修复中发现 RN 0.83 的 `NewArchitectureHelper.new_arch_enabled` **硬编码返回 true**（`react-native/scripts/cocoapods/new_architecture.rb:156-158`），`react_native_post_install` 会在每次 `pod install` 时把 `RCTNewArchEnabled=true` 强行写回 app Info.plist（`react_native_pods.rb:550`）——即 app.json 的 `newArchEnabled:false` 对 iOS 原本完全失效。插件因此在两处兜底：prebuild 阶段 `withInfoPlist` 写 `RCTNewArchEnabled=false`；Podfile 钳制块在 `react_native_post_install` 之后按 `Podfile.properties.json` 的 `expo.newArchEnabled` 重新写回该键。
- 回归证据：
  - `pod install` 后 `ios/WeKnora/Info.plist` 保持 `RCTNewArchEnabled=false`（本轮 grep 复核）；
  - 模拟器：首屏 DeploymentLoginScreen 完整渲染（`01-first-screen.png`、`02-clean-first-screen.png`、`05-rebuild-reinstall-first-screen.png`；idb AX 快照含 "Sign in to WeKnora"/"Sign in"/"Continue with single sign-on"），JS 侧 `Running "main"`（见第 2 节日志），无白屏。
- 备注（残留风险）：Fabric 白屏的根因在 RN 0.83.10 / iOS 27.0 runtime 组合，本仓库以关闭新架构规避；后续升级 Expo/RN 后可重开 `newArchEnabled` 复测。另注意 RN 编译期 `-DRCT_NEW_ARCH_ENABLED=1` 标志仍作用于从源码构建的 pods（`new_architecture.rb:46-67`），与运行时开关独立——与测试轮 07/08 证据一致（同配置可正常渲染）。

### #3 important — pnpm 严格布局下 Metro 找不到 babel-preset-expo ✅ 已修复

- 修复：`apps/mobile/package.json` devDependencies 显式声明 `babel-preset-expo: ~55.0.25` 并 `pnpm install`（lock 新增 36 行，仅此包）。删除了测试轮手工 symlink 后由 pnpm 正式重建：`apps/mobile/node_modules/babel-preset-expo -> .pnpm/babel-preset-expo@55.0.25…`。
- 回归证据：两次全新 Release 构建的 `Bundle React Native code and images` 阶段均成功产出 `main.jsbundle`（`ios/build/…/WeKnora.app/main.jsbundle`，3.4MB；构建日志 `xcodebuild-release-round1.log` 中 `Cannot find module` 计数为 0）。

### #4 minor — Xcode 27 与 pods deployment target 不兼容 ✅ 已修复

- 修复（替代测试轮的手工 Podfile/pbxproj 补丁，改为可再生的插件产物）：
  - `withPodfileProperties` 写 `ios.deploymentTarget: "16.0"`（Podfile `platform :ios, podfile_properties['ios.deploymentTarget']` 生效）；
  - Podfile 钳制块把所有 pods 的 `IPHONEOS_DEPLOYMENT_TARGET < 16.0` 抬到 16.0（覆盖 SDWebImage 9.0、expo-router 15.1）；
  - `withXcodeProject` 把 pbxproj 内 4 处 `IPHONEOS_DEPLOYMENT_TARGET` 15.1 → 16.0。
- 回归证据：单测 `applyPodfileClamp` / `raiseDeploymentTargets`（含"高于钳值不降、注释项跳过"断言）；`pod install` 后 `Pods/Pods.xcodeproj` 224 个构建配置全部为 `= 16.0`（`grep -o | sort | uniq -c` 仅剩 16.0 一档）；两次 Release 构建成功（此前该组合在构建 1/2/3 轮连续失败，见测试报告）。

### #5 minor — deep link weknora:///resources 未导航 ✅ 已修复并端到端证实

- 复核（本轮新证据）：测试轮 09 号截图上的确认弹窗（「在"WeKnora"中打开？」）从未被点击（idb 缺失），URL 很可能从未送达 app——"未导航"是环境阻塞放大出的结论；同时上一轮 scene 补丁确实缺少冷启动 URL 转发。
- 修复：SceneDelegate 完整 URL 转发——
  - warm：`scene(_:openURLContexts:)` → `AppDelegate.application(_:open:options:)` → Expo 订阅者（expo-linking `LinkingAppDelegateSubscriber` 置 `ExpoLinkingRegistry.initialURL` 并广播 `onURLReceivedNotification`，源码 `expo-linking/ios/LinkingAppDelegateSubscriber.swift:6-10`）+ `RCTLinkingManager`；
  - 冷启动：`scene(_:willConnectTo:)` 内转发 `connectionOptions.urlContexts`（RN 0.83 的 `getInitialURL` 只读 app launchOptions（`RCTLinkingManager.mm`），scene 下冷启动 URL 只在 connectionOptions 里，不转发则 `expo-router` 的 `Linking.getLinkingURL()` 拿不到初始路由）；
  - Universal Links：`scene(_:continue:)` 同样转发。
- 回归证据（模拟器，安装 idb 后具备 tap/AX 能力）：
  - warm：app 运行于登录屏时 `xcrun simctl openurl 0A38… "weknora:///resources"` → AX 快照变为 "WeKnora" + **"Sign in to browse tenant resources."**（未授权 ResourcesRoute 的固定文案，源码 `src/app/resources.tsx` 生命周期分支），截图 `03-deeplink-resources.png` + `03-deeplink-resources-axlabels.txt`；
  - 冷启动：`simctl terminate` 后 `openurl "weknora:///resources"` → 8 秒后 AX 快照同样为 "Sign in to browse tenant resources."，截图 `04-coldlaunch-deeplink-resources.png`（证明 willConnectTo 冷启动转发链路工作）；
  - 正常启动仍回登录屏（root 路由，deep link 状态不粘连）。

## 2. 定向测试与验证命令（全部本轮实际执行）

| 命令 | 结果 |
| --- | --- |
| `pnpm install`（仓库根） | ✅ 4.2s，`babel-preset-expo@55.0.25` 正式链接 |
| `pnpm --filter @weknora/mobile test` | ✅ 64/64 通过（含新增 `src/plugins/ios-xcode27.test.ts` 5 例） |
| `pnpm --filter @weknora/mobile typecheck` | ✅ exit 0（原版 tsconfig include 已还原并复验通过） |
| `rm -rf ios && npx expo prebuild -p ios --no-install`（apps/mobile） | ✅ 生成物含全部修复（AppDelegate scene 化 / Info.plist 双键 / Podfile 钳制 / pbxproj 16.0 / properties 双键） |
| `cd ios && pod install` | ✅ 103 pods；Info.plist 键保住；pods 224 配置全 16.0 |
| `xcodebuild -workspace WeKnora.xcworkspace -scheme WeKnora -configuration Release -sdk iphonesimulator -destination 'platform=iOS Simulator,id=0A38DB71-…' -derivedDataPath build ARCHS=arm64 ONLY_ACTIVE_ARCH=YES build` | ✅ BUILD SUCCEEDED（执行 2 次：干净构建 + 增量重建，日志 `xcodebuild-release-round1.log`） |
| `simctl install/launch/openurl/terminate` + `idb ui tap/describe-all`（iPhone 18 Pro, iOS 27.0） | ✅ 三轮启动进程存活；warm/cold deep link 均导航成功 |
| `simctl spawn … log show`（启动日志） | ✅ 无 SIGTRAP/UIScene 运行时错误；scene 设置正常（`app-launch-log.txt`） |

环境说明：为获得 tap/AX 自动化，本轮安装了 `idb-companion`（brew，本机已有 1.6.1）+ `fb-idb`（pip 1.6.1）；这是宿主工具链改动，与仓库无关。测试轮发现并本轮沿用的客观限制：Debug+Metro 联调仍受宿主全局代理（127.0.0.1:17890）拦截 localhost 影响，未改动用户网络环境。

## 3. 证据清单（docs/plans/issue30-sweep/ios-evidence/fix-round-1/）

- `01-first-screen.png` — 修复后首次启动：登录屏已渲染（画面上另有测试轮遗留的 deep link 确认弹窗，随后已处置）
- `02-clean-first-screen.png` — 处置弹窗后重启的干净首屏（DeploymentLoginScreen）
- `03-deeplink-resources.png` / `03-deeplink-resources-axlabels.txt` — warm deep link `weknora:///resources` → "Sign in to browse tenant resources."
- `04-coldlaunch-deeplink-resources.png` / `04-coldlaunch-deeplink-resources-axlabels.txt` — 冷启动（terminate 后经 URL 启动）直达 Resources 屏（独立重拍，与 03 字节不同）
- `05-rebuild-reinstall-first-screen.png` — 增量重建 + 重装 + 启动，首屏仍正常
- `app-launch-log.txt` — 本轮启动关键日志（无崩溃；scene 生命周期正常）
- `xcodebuild-release-round1.log` — 首次干净 Release 构建日志（裁剪版：头部环境 + 全部 error/warning + 结尾 BUILD SUCCEEDED）

## 4. 未验证项 / 残留

1. **授权面交互（#32/#34/#35/#66）** — 与测试轮相同：无 deployment 凭据与后端，未涉及；Resources 屏仅验证到未授权提示文案。
2. **Fabric 根因** — 未深入定位 RN 0.83 Fabric 在 iOS 27.0 runtime 白屏的上游缺陷（超本仓范围），以 `newArchEnabled:false` 规避；升级 SDK 后建议复测再翻转。
3. **Expo 模板锚定** — 插件锚定 SDK 55 模板原文，未来升级 Expo 若模板变化，prebuild 会显式报错（单测覆盖该行为），届时需同步更新锚点。
