# iOS 模拟器验证报告 — issue30-sweep apps/mobile

- 日期：2026-09-23
- 执行者：iOS 测试员（动态工作流子代理）
- 被测对象：worktree `.worktrees/issue30-sweep` 的 `apps/mobile`（Expo ~55.0.0 / React Native 0.83.10 / expo-router ~55.0.18 / React 19.2.0，`package.json`）
- 结论一览：**Release 构建成功、安装成功；默认配置启动即 SIGTRAP 崩溃（iOS 27 SDK 强制 UIScene 生命周期）；本地 scene 补丁 + 关闭新架构后启动不崩溃且首屏（DeploymentLoginScreen）渲染成功**。授权面核心交互（#32–#35、#66）因无可用 deployment 凭据/服务器，未验证。

---

## 1. 环境与 Preflight

- 工具：`mcp__plugin_ios-simulator_ios-simulator__ios_preflight`，全部通过。
  - macOS darwin、Xcode 27.0 (Build 27A266a)、simctl 可用、11 台 iOS 27.0 模拟器。
  - UI 自动化后端：`idb unavailable` → tap/swipe/type 自动化不可用，本轮以构建/安装/启动/截图/日志验证。
- 模拟器：复用已 booted 的 **iPhone 18 Pro，UDID `0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC`，iOS 27.0 runtime**。
- 依赖状态：pnpm workspace（`pnpm@10.28.2`），`apps/mobile/node_modules/@weknora/{api-client,domain,mobile-core}` symlink 正常；CocoaPods 1.17.0。

## 2. 构建（preflight → prebuild → pod install → xcodebuild Release）

| 步骤 | 命令 | 结果 |
| --- | --- | --- |
| prebuild | `cd apps/mobile && npx expo prebuild -p ios --no-install` | 成功，生成 `ios/WeKnora.xcodeproj`（scheme `WeKnora`，bundle id `com.weknora.mobile`） |
| pods | `cd ios && pod install` | 成功（26s，103 pods）；二次 60s |
| 构建 1 | `xcodebuild -workspace ios/WeKnora.xcworkspace -scheme WeKnora -configuration Release -sdk iphonesimulator -destination 'platform=iOS Simulator,id=0A38…85FC' -derivedDataPath ios/build build` | **失败**：`SDWebImage-SDWebImage` deployment target 9.0 < Xcode 27 支持范围 15.0–27.0 |
| 修复 A | Podfile post_install 钳制 pods `IPHONEOS_DEPLOYMENT_TARGET ≥ 15.1`，重跑 pod install | — |
| 构建 2 | 同上 | **失败**：`expo-router/ios/LinkPreview/LinkPreviewNativeActionView.swift:141` `'subtitle' is only available in iOS 16.0 or newer`（pod target 15.1） |
| 修复 B | 钳制下限提高到 16.0 + `ARCHS=arm64 ONLY_ACTIVE_ARCH=YES`，重跑 pod install | — |
| 构建 3 | 同上 | **失败**：主 app target 15.1 无法 import 模块 `Expo`（16.0） |
| 修复 C | `WeKnora.xcodeproj/project.pbxproj` 4 处 `IPHONEOS_DEPLOYMENT_TARGET` 15.1 → 16.0 | — |
| 构建 4 | 同上 | **失败**：Metro `Cannot find module 'babel-preset-expo'`（`Bundle React Native code and images` 阶段，`xcodebuild-release.log:7976`） |
| 修复 D | `apps/mobile/node_modules/babel-preset-expo` symlink 到根 `.pnpm/babel-preset-expo@55.0.25…/node_modules/babel-preset-expo`（仅本地，未入 git） | — |
| 构建 5 | 同上 | ✅ **BUILD SUCCEEDED**（arm64 Release，`ios/build/Build/Products/Release-iphonesimulator/WeKnora.app` 含 `main.jsbundle`） |

完整日志：`xcodebuild-release.log`（最终成功轮）、`xcodebuild-debug.log`（诊断用 Debug 构建，同样 BUILD SUCCEEDED）。

## 3. 运行

### 3.1 默认配置：启动即崩溃（SIGTRAP）

- `xcrun simctl install … WeKnora.app` 成功；`xcrun simctl launch … com.weknora.mobile` 返回 PID 22255。
- 约 1s 后进程退出：SpringBoard 日志 `Process exited … status: signal(2) code: SIGTRAP(5)`（`app-launch-log.txt`）。
- 根因（app 自身日志，`app-launch-log.txt` 尾部）：
  > `(UIKitCore) … _UIApplicationEvaluateRuntimeIssueForNoSceneLifecycleAdoption … : Application failed to launch: UIScene life cycle is required for apps built with this SDK.`
- 即：**Xcode 27 / iOS 27 SDK 强制 UIKit scene 生命周期；prebuild 生成的 AppDelegate（`ExpoAppDelegate` 子类，`UIWindow(frame:)` 旧式）不满足**。Expo SDK 55 的 `ExpoAppDelegate.swift` 场景支持仍是 `// TODO: - Configuring and Discarding Scenes`（expo 包 `ios/AppDelegates/ExpoAppDelegate.swift:54`）。
- 截图 `01-first-screen.png`：崩溃后回到 Springboard（WeKnora 图标第三行）。

### 3.2 本地 scene 补丁：不再崩溃但白屏

- 补丁（仅本地生成目录 `apps/mobile/ios/WeKnora/`，未提交）：`AppDelegate.swift` 增加 `application(_:configurationForConnecting:options:)` 返回 `SceneDelegate`，RN 启动延迟到 `scene(_:willConnectTo:)` 中 `factory.startReactNative(withModuleName:"main", in: window, …)`；`Info.plist` 增加 `UIApplicationSceneManifest`（`UIApplicationSupportsMultipleScenes=false`）。
- 结果：进程存活，JS 执行（unified log `[com.facebook.react.log:javascript] Running "main"`，Expo 模块全部注册、`ExpoSecureStore` keychain 读取发生），**但屏幕空白**（`02-after-scene-patch.png`、`03-after-18s.png`、`04-light-mode.png`，强制浅色外观排除暗色模式）。
- Debug 构建 + Metro 诊断受阻：app 对 `http://localhost:8081/status` 的连接被宿主系统级代理劫持（`scutil --proxy`：HTTP/HTTPS 代理 `127.0.0.1:17890`，`ExcludeSimpleHostnames: 0`；默认路由 utun4 VPN；各 networksetup 服务的代理开关均为关，代理来自 VPN/TUN 客户端，未改动用户环境）。Metro 端零请求。
- deep link `weknora:///` 触发系统确认弹窗；`weknora:///resources` 未产生可见导航（见 4）。

### 3.3 关闭新架构：首屏渲染成功

- 实验：`Info.plist` `RCTNewArchEnabled` `true → false`（app.json 为 `newArchEnabled: true`），增量重建 Release，重装启动。
- 结果：✅ **首屏正常渲染 DeploymentLoginScreen**：
  - `07-old-arch.png`：含此前 openurl 残留的系统弹窗；
  - `08-clean-first-screen.png`（重启后干净首屏）："Sign in to WeKnora" 标题、origin 输入框（placeholder `https://weknora.example.com`）、Email、Password 输入框、"Sign in"、"Continue with single sign-on" 按钮。
- 由此确证 3.2 的白屏根因：**Fabric（新架构）在 iOS 27.0 模拟器 runtime 上不渲染内容**（JS 无错误运行、`Running "main"`，但 surface 无可见输出）；老架构渲染管线正常。
- 启动日志（`final-run-keylog.txt`）：最终两次启动（PID 65123、66393）均 `Registering module 'ExpoSecureStore'` + `Running "main"`，无 fatal/abort。

## 4. 交互路径验证结果

| 目标 | 结果 |
| --- | --- |
| 构建 / 安装 / 启动 / 首屏渲染 | ✅（需上述环境适配；证据 `08-clean-first-screen.png`） |
| #32 Tenant 切换 + Scoped Vault 隔离 | ❌ 未验证：授权面需要真实 deployment（官方云或自托管）账号凭据与后端；本轮无可用凭据（也不允许写入凭据字面量），runtime 停留在 `deployment-login` surface |
| #33 Resources 页 | ⚠️ 部分：路由文件与 `ResourcesRoute`（未授权时渲染 "Sign in to browse tenant resources."）存在（`src/app/resources.tsx`），但 deep link `weknora:///resources` 在 scene 补丁环境下未触发导航（`09-resources-route.png` 与首屏相同），无法走到该屏；tap 自动化因 idb 缺失不可用 |
| #34 首页三段聚合视图 | ❌ 未验证（同 #32，需授权面） |
| #35 Task 详情/恢复 | ❌ 未验证（同上） |
| #66 多 Deployment 原子切换 | ❌ 未验证（同上）；未授权面可确认 `DeploymentLoginScreen` 的表单与 `validatedDeploymentOrigin`（仅 https、无凭据/path）客户端防线在源码层存在（`src/screens/DeploymentLoginScreen.tsx:16-24`） |

## 5. 截图与日志证据

- `01-first-screen.png` 默认配置启动崩溃后 Springboard
- `02-after-scene-patch.png` `03-after-18s.png` `04-light-mode.png` scene 补丁后白屏（含浅色强制）
- `05-after-openurl.png` openurl 后（白屏依旧）
- `06-debug-build.png` Debug 构建白屏（Metro 不可达）
- `07-old-arch.png` 老架构首屏（含 openurl 系统弹窗）
- `08-clean-first-screen.png` **干净首屏（最终通过证据）**
- `09-resources-route.png` resources deep link 未导航
- `app-launch-log.txt` SIGTRAP 轮关键日志（含 UIKit scene 强制失败的原始行）
- `final-run-keylog.txt` 最终成功轮关键日志
- `xcodebuild-release.log` / `xcodebuild-debug.log` 构建日志
- `metro.log` Metro 启动与零请求记录

## 6. 本轮对被测代码 / 仓库的改动

- `git` 提交仅包含本目录证据与报告。
- 以下本地改动**留在工作区未提交**（均为 prebuild 生成目录或 node_modules，属验证环境适配）：
  - `apps/mobile/ios/`（prebuild 生成 + Podfile deployment-target 钳制 + pbxproj 16.0 + scene 补丁 AppDelegate/Info.plist + RCTNewArchEnabled=false）
  - `apps/mobile/node_modules/babel-preset-expo` symlink

## 7. 无法验证项汇总

1. #32/#34/#35/#66 的授权面交互 — 无 deployment 凭据与后端（opt-in 真实 HTTP 集成证据本应来自带凭据的环境）。
2. UI tap 级自动化 — idb 后端缺失（技能标注 P0 之外能力）。
3. Debug + Metro 的 JS 级诊断 — 宿主 VPN/TUN 全局代理拦截模拟器 localhost 流量；未改动用户网络环境。
4. deep link 路由导航 — scene 补丁环境下 `weknora:///resources` 未导航（可能与本地补丁的 linking 转发相关，未能在无 Metro 条件下定位）。
