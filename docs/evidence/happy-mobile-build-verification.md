# Happy 移动端构建验证报告（happy-mobile-build-verification）

日期：2026-09-11。执行者：构建验证会话（goal round 1–2）。
本报告独立于 H01–H04 任务报告，只覆盖本次「typecheck / 双平台 Expo export / iOS 与 Android 原生构建 / 模拟器运行 / 认证回归」的重新验证与修复。

## 基线与产出

| 项 | 值 |
|---|---|
| 验证基线 SHA | `9df830e`（main，Merge codex/happy-mobile） |
| 独立修复分支 | `codex/happy-mobile-verify`，worktree `.worktrees/happy-mobile-verify` |
| 修复提交 | `codex/happy-mobile-verify` 分支上基于 9df830e 的唯一提交（`git -C .worktrees/happy-mobile-verify log -1`；提交信息含完整修复清单） |
| 工具链 | pnpm 10.28.2（仓库声明）、Node v26.7.0、Xcode 26.6 (17F113)、CocoaPods 1.17.0、JDK 17.0.19 (Temurin)、Android SDK（`~/Library/Android/sdk`） |

主 worktree 中他人的未提交修改（`apps/web/src/platform/legacy-session.test.ts`、`docs/superpowers/plans/saas-billing-connectors-progress.md`、`packages/api-client/src/knowledge/faq.test.ts`）未被触碰；未执行 reset --hard / git clean；未推送、未合并。

## 结论总表

| # | 检查项 | 结果 | 关键证据 |
|---|---|---|---|
| 1 | mobile typecheck | **PASS** | exit 0（修复前 2 个 TS2307 失败） |
| 2 | iOS Expo export（JS bundle） | **PASS** | `apps/mobile/dist-ios/_expo/static/js/ios/index-e30e4bf16bfa64e1f100414bfb8ff044.hbc`（12MB）+ metadata.json |
| 3 | Android Expo export（JS bundle） | **PASS** | `apps/mobile/dist-android/_expo/static/js/android/index-76536206f19f85efbfc9cb3a1a6141ea.hbc`（12MB） |
| 4 | iOS 原生编译（Debug, 模拟器） | **PASS** | `xcodebuild … CODE_SIGNING_ALLOWED=NO build` exit 0；`ios/build/Build/Products/Debug-iphonesimulator/WeKnoraMobiledev.app`（242MB） |
| 5 | iOS 原生编译（Release, 模拟器, 内嵌 bundle） | **PASS** | exit 0；`Release-iphonesimulator/WeKnoraMobiledev.app`（237MB，含 `main.jsbundle`） |
| 6 | Android APK 构建（assembleDebug） | **PASS** | 见「Android 构建记录」；`android/app/build/outputs/apk/debug/app-debug.apk` |
| 7 | 模拟器安装启动（iOS） | **PASS** | iPhone 17 Pro（5EECD8BB…，Booted）安装并启动 Release 包（PID 存活、无崩溃报告）；Debug 包此前亦启动成功 |
| 8 | 模拟器安装启动（Android） | **PASS** | AVD `test36-small` headless 启动 → `adb install -r app-debug.apk` Success → `am start …MainActivity`，进程 PID 3179 存活 25s+，logcat 无 FATAL；截图 `docs/evidence/assets/android-emulator-launch.png` |
| 9 | 认证与回调回归测试 | **PASS** | node:test 16+3+1 通过；Vitest 2+5 通过（runner 分离） |

三层成功口径已区分：JS bundle 导出成功（#2/#3）≠ 原生编译成功（#4/#5/#6）≠ 设备运行成功（#7）。

## 环境与命令记录

- 安装：`pnpm install --frozen-lockfile`（基线，exit 0，19.5s）；修复后复验 frozen 安装 exit 0。
- typecheck：`pnpm --filter @weknora/mobile typecheck`。
- export（双平台分别输出目录，未共用 dist）：`pnpm --filter @weknora/mobile exec expo export --platform ios --output-dir dist-ios`；`… --platform android --output-dir dist-android`。
- iOS 原生：`pnpm exec expo prebuild --platform ios`（生成 ios/，含 Podfile）→ `cd ios && pod install` → `xcodebuild -workspace WeKnoraMobiledev.xcworkspace -scheme WeKnoraMobiledev -configuration Debug -sdk iphonesimulator -destination 'platform=iOS Simulator,name=iPhone 17 Pro' -derivedDataPath build CODE_SIGNING_ALLOWED=NO build`；Release 同参数 `-configuration Release`。
- Android 原生：`pnpm exec expo prebuild --platform android` → `JAVA_HOME=$(/usr/libexec/java_home -v 17) ANDROID_HOME=~/Library/Android/sdk ./gradlew assembleDebug --no-daemon`。
- 模拟器：`xcrun simctl install 5EECD8BB-4B4A-473C-85C3-7841329FDF3C <app>` + `xcrun simctl launch … com.weknora.mobile.dev`；截图见 `docs/evidence/assets/`。

## 修复明细（本次提交的代码变更）

1. **缺失的 H04 屏幕（typecheck 根因）**：合并进 main 的 `045e0cd` 提交了路由文件 `sources/app/(app)/auth-return.tsx`、`invitation.tsx`（re-export `@/weknora/auth/AuthReturnScreen` / `InvitationScreen`），但从未提交这两个屏幕实现，main 上 typecheck 报 TS2307。新增：
   - `apps/mobile/sources/weknora/auth/AuthReturnScreen.tsx`：消费 `@weknora/domain/mobile` 的 `consumeAuthReturn/validateAuthReturn`，一次性消费 state、拒绝外站 redirect、拒绝 URL 携带 token；验证成功也不从 URL 取凭证（api-client 的 OIDC exchange 冻结为不可用），引导回密码登录。
   - `apps/mobile/sources/weknora/auth/InvitationScreen.tsx`：公共 lookup + 登录态 accept-by-token，服务端失效/过期响应原样呈现。
   - `apps/mobile/sources/weknora/auth/auth-return-status.ts` + `.test.ts`（Vitest，5 用例：一次性消费、伪造 state、缺 state、URL 带 token、重复参数取首值）。
2. **reanimated/worklets 版本断裂（pod install 根因）**：`^4.2.3` 漂移解析到 reanimated 4.6.0，其 compatibility.json 要求 worklets 0.12.x，与已装 0.7.4 不兼容，RNReanimated podspec 校验失败。按 Happy 冻结基线（ac64b9b 的 lockfile 为 reanimated 4.2.3 + worklets 0.7.x）将 `apps/mobile/package.json` 钉为 `"react-native-reanimated": "4.2.3"`，未升级任何其他依赖。
3. **pnpm 10 拦截的原生 postinstall（skia/libsodium 根因）**：安装时 `@shopify/react-native-skia`（下载预编译 Skia）与 `@more-tech/react-native-libsodium`（解包 Clibsodium.xcframework）的 postinstall 被 pnpm 默认策略忽略，导致 pod install 报 “Skia prebuilt binaries not found”、iOS 编译报 `sodium.h not found`。在 `pnpm-workspace.yaml` 增加 `onlyBuiltDependencies` 白名单（可复现，不依赖手工步骤）。
4. **react-native-audio-api 头文件（iOS 编译根因）**：Xcode 26 的 libc++ 不再间接提供 `size_t`，0.8.4 的 `Constants.h` 缺 `<cstddef>`。仓库既有 `patches/fix-react-native-audio-api-size-t.cjs`（从未被任何脚本引用、且指向不存在的 packages/happy-app 路径）已记录同一意图；改用 pnpm 原生补丁机制落地：`patches/react-native-audio-api@0.8.4.patch`（加 include + `std::size_t` 限定），经 `pnpm patch/patch-commit` 登记，frozen 安装自动应用。

### 仅本地环境的修复（未提交，记录以便复现）

- `ios/Pods/ReactNativeCore-artifacts/reactnative-core-0.83.1-{debug,release}.tar.gz`：Maven Central（repo1）下载的 RN 预编译核心；本机网络对 ~90MB 文件多次 curl 18 截断，用 `--retry -C -` 循环完成。pod install 的内置下载失败时这是它本来要做的事。
- `android/build.gradle`（生成物）：本机未装 NDK 27.0.12077973（目录为空的坏安装，已删除；sdkmanager 重装也报 ZipFile 错误，cmdline-tools 过旧），已装 27.1.12297006，故在生成的 root build.gradle 追加 `ext.ndkVersion` + `subprojects { pluginManager.withPlugin(…com.android.library…) { ndkVersion = "27.1.12297006" } }` 覆盖 AGP 8.6 默认值。
- Android 打包 OOM：生成的 `gradle.properties` 仅 `-Xmx2048m`，打包该多 ABI 大 APK 时 zipflinger Java heap OOM；以 `-Dorg.gradle.jvmargs="-Xmx8g -XX:MaxMetaspaceSize=1g"` 重跑。

## Android 构建记录（逐次）

1. 首次 assembleDebug：配置期失败——NDK 27.0.12077973 无 source.properties（坏安装）→ 删除坏目录。
2. 第二次：AGP 试图自动安装该 NDK 失败（InstallFailedException，sdkmanager ZipFile 错误）+ `:expo` 配置报 SoftwareComponent release not found（同为 NDK 解析失败的连锁）→ 增加 subprojects ndkVersion 覆盖。
3. 第三次：编译推进到 564 任务，剩少量 jar 下载遇 TLS “Remote host terminated the handshake”（与 Maven tarball 同源的本机网络抖动）→ 直接重试。
4. 第四次：编译全部通过（1676 任务），`:app:packageDebug` zipflinger `OutOfMemoryError: Java heap space` → 提升堆至 8g 重跑（见结论表）。
5. 第五次（8g 堆）：BUILD SUCCESSFUL，产物 `app/build/outputs/apk/debug/app-debug.apk`。

## 认证与回调回归（runner 分离）

| 套件 | runner | 结果 |
|---|---|---|
| `packages/api-client/src/auth.test.ts`、`auth/{login,oidc,invitations}.test.ts`、`packages/domain/src/mobile/auth-return.test.ts` | `tsx --test`（node:test） | 16/16 通过（含并发 401 单飞刷新、登录 wire 字段/错误处理、OIDC start/exchange 冻结、外站 redirect/token-in-URL 拒绝、state 单次使用） |
| `apps/mobile/…/auth/credentials.test.ts` | `tsx --test`（node:test；在 Vitest 下会误报 no suite） | 3/3 通过（凭证隔离、重启恢复、明文不落盘） |
| `apps/mobile/…/platform/origin-storage.test.ts` | `tsx --test` | 1/1 通过（Origin 持久化与清除） |
| `apps/mobile/…/platform/host.test.ts`、`…/auth/auth-return-status.test.ts` | `vitest run` | 2/2 + 5/5 通过 |
| 全部 node:test 用 Vitest 跑（反面验证） | — | credentials 报 “No test suite found”，证明 runner 必须区分 |

未恢复 Happy 上游服务默认连接；所有运行验证基于产品 host 与本地隔离 worktree。

## 模拟器/设备运行验证

- iOS（PASS）：iPhone 17 Pro（iOS 26.5，Booted）安装 Debug 包并启动（进程存活 20s+、无 WeKnora 崩溃报告）；随后安装 Release 包（内嵌 main.jsbundle，无需 Metro）启动成功，截图 `docs/evidence/assets/ios-simulator-release-launch.png`。未启动任何 Metro 进程，未占用其他项目的 8081 端口。
- 交互级验收（服务器入口、键盘、返回导航、主题）在本会话工具（无视觉通道）下未逐项执行，Release 包已启动且稳定，留待 H02 原生验收步骤以人工/视觉方式完成记录——此项不冒充已验收。
- Android（PASS）：AVD `test36-small`（本机 `emulator -list-avds` 列出 test36/test36-small）以 `-no-window -no-audio -no-boot-anim -gpu swiftshader_indirect` headless 启动，`adb wait-for-device` + `sys.boot_completed=1` 后安装启动成功。

## 阻塞与未执行项

- 键盘/返回导航/主题的逐屏原生验收：属于 H02 验收范围，本次构建会话不做功能验收宣称（两端 App 均已安装启动且进程稳定，作为后续人工验收的载体）。
- 本机网络对大文件（Maven Central tarball、个别 Maven jar）多次 TLS 截断：已用重试/断点续传绕过并记录。

## 提交记录

本文件与下列修复同批提交于 `codex/happy-mobile-verify` 分支：

- `apps/mobile/package.json`、`pnpm-lock.yaml`（reanimated 4.2.3 钉版）
- `pnpm-workspace.yaml`（onlyBuiltDependencies + patchedDependencies）
- `patches/react-native-audio-api@0.8.4.patch`
- `apps/mobile/sources/weknora/auth/{AuthReturnScreen,InvitationScreen,auth-return-status,auth-return-status.test}.ts{x}`
- `docs/evidence/happy-mobile-build-verification.md` + `docs/evidence/assets/ios-simulator-*.png`

不包含：node_modules、ios//android//dist-* 构建产物、Pods、敏感配置。
