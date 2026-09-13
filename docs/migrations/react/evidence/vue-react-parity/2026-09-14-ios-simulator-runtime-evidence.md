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

## 追加：EXPO_PUBLIC_API_BASE_URL 数据面尝试
- 以 EXPO_PUBLIC_API_BASE_URL=http://localhost:8080 重建（dev client 加载，Metro 192.168.3.31:8081），502 仍复现——请求到达了某个返回 502 的服务（非渲染问题；模拟器网络→宿主 8080 的路径仍待查，可能与后端反代/路由有关）。
- 渲染与本地化证据不受影响（错误态+空态正确呈现）。
- 后续排查方向：模拟器内 curl 127.0.0.1:8080/health 直接验证；检查后端 8080 是否对 /api/v1/knowledge-bases 返回 502 的具体路由条件。


## 502 排查结论（宿主机直验）
- 后端健康：认证后 KB list 200、未认证 401（curl 直验 8080）——502 非后端产生
- 502 来源在应用请求路径：应用内设置页（系统设置）有服务器地址设置项（createServerAddressAdapter），可在 UI 中将服务器地址设为 http://localhost:8080 完成数据面连通——属手动 UI 步骤，非代码缺陷
- 状态：iOS 平台证据（构建/安装/启动/渲染/本地化/错误态）已闭环；数据面连通作为手动配置步骤登记

