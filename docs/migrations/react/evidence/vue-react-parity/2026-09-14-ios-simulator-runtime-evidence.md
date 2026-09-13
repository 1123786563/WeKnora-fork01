# 2026-09-14 iOS 模拟器运行时证据（N031 iOS 平台解锁）

## 构建与环境
- 前置: CocoaPods 1.17.0、apps/mobile/ios 工程已生成（Podfile.lock）、iPhone 17 Pro 模拟器（iOS 26.5）
- 构建: xcrun simctl boot "iPhone 17 Pro" + npx expo run:ios（CocoaPods 安装 → pods 编译 → Xcode 构建 → Metro bundle 1365 模块 7.6s）
- 全程日志: /tmp/ios-build.log

## 运行时验证（截图 ios-sim-app-launch.png）
- 应用安装并启动于模拟器：路由 knowledge/index 正常渲染
- 中文本地化完整：知识库标题、导航（新对话/共享空间/系统设置/退出登录）、筛选 chips（全部/我创建的/收藏/最近访问）、空态文案（暂无知识库）
- 数据面：API 请求返回 502（模拟器内 API base 与宿主 8080 后端的连通差异）——错误态渲染正确（错误行+空态共存），数据连通需后续把 API base 指向宿主后端

## 结论
- iOS 平台证据从 blocked-env 解锁为「构建+安装+启动+渲染+本地化」已验收；数据连通（502）作为已知限制登记
- Android 维持 blocked-env（SDK/emulator 缺失；恢复: 安装 Android Studio + sdkmanager "platform-tools" "emulator" + avdmanager create）

## 命令
xcrun simctl boot "iPhone 17 Pro"
npx expo run:ios --device "iPhone 17 Pro"
xcrun simctl io "iPhone 17 Pro" screenshot <path>
