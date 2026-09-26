# T33 iOS 抽查（T02 模式，集成 HEAD d6b3e1bb5，2026-09-26）

## 拓扑
Lite 127.0.0.1:57828（同 Web/微信轮同一服务器）+ TLS 终结代理 127.0.0.1:57830→57828（自签 CA 仅注入模拟器钥匙串：`xcrun simctl keychain 0A38… add-root-cert ca.pem`）。

## 构建与启动（真实记录）
1. `npx expo prebuild -p ios --no-install` + `pod install`（CocoaPods 1.17.0）→ WeKnora.xcworkspace
2. `EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN=https://127.0.0.1:57830 RCT_NO_LAUNCH_PACKAGER=1 xcodebuild -workspace WeKnora.xcworkspace -scheme WeKnora -configuration Release -sdk iphonesimulator -destination 'id=0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC' -derivedDataPath build CODE_SIGNING_ALLOWED=NO build` → **\*\* BUILD SUCCEEDED \*\*（exit 0）**
3. `grep -c '127.0.0.1:57830' WeKnora.app/main.jsbundle` = **1**（origin 构建期内联）
4. ad-hoc 重签（codesign -f -s -）+ `simctl install` + `simctl launch` → **应用真实启动至自身登录门**（AX 实测 8 元素：Sign in to WeKnora / Use WeKnora Cloud / origin 预填 https://127.0.0.1:57830 / Email / Password / Sign in / Continue with single sign-on；截图 ios/01-launch.png）

## GUI 登录（idb 键盘注入）
- 逐字符注入 a33@t.io（短邮箱绕 T02 E4 键盘截断）+ 12 字符密码 → tap Sign in
- **服务端证据（SQLite auth_tokens 表，非推测）**：user f3fb7958-9f70-4844-9964-91d06373577e（=a33@t.io）于 **22:52:53 与 22:54:17 两次签发 access+refresh token**（auth_tokens 4 行）⇒ 真实 POST /api/v1/auth/login 经 TLS 代理到达服务器并 **200**
- **App UI 终态：Update required 面**（AX 实测：'Update required / https://127.0.0.1:57830 cannot open this version of WeKnora. / Update the app or contact the deployment administrator. / Sign out'；截图 ios/02-after-signin.png）——**D-iOS-1 在集成 HEAD 完整复现**：登录服务端成功后，App 内授权链路异常回退 fail-closed 面，登录后续请求（/auth/me 等）不发。与 T02/T02补缺 报告一致（修复依赖可用签名环境，本机 0 valid identities）。

## HTTP 边界链（同服务器同 fixture，见 ios-http-probes.txt）
- 属主读取受保护 Task（career 申请创建的 workbench run）：列表 200 含 ef1125b7/waiting_user；run snapshot 200
- 跨租户：B（t33b@t33.io）读同 run snapshot → 404；B 列表 200 空数组；匿名 → 401

## Career 链 App 内可达性（如实）
不可达：①D-iOS-1 登录后授权面未建立；②Expo 端 Career UI 六票（T05/T23/T25/T27/T29/T31）随 T01 blocked 从未实现。Career 链闭环由同服务器 Web（完整七步）与微信（同源档案/申请/材料页）覆盖。

## 环境
Xcode 27.0 (27A266a)；iPhone 18 Pro 模拟器 UDID 0A38DB71-…（iOS 27.0 Booted）；idb 0.4.x + idb_companion；ad-hoc 签名空 entitlements（本机无 Apple 开发证书，T02 同边界）。
