# r100 iOS 原生编译与启动验收

## 环境与命令

- 模拟器：iPhone 17 Pro，iOS 26.5，UDID `5EECD8BB-4B4A-473C-85C3-7841329FDF3C`
- 工程：`apps/mobile/ios/WeKnora.xcworkspace`
- 构建：`xcodebuild -workspace ios/WeKnora.xcworkspace -scheme WeKnora -sdk iphonesimulator -configuration Debug -derivedDataPath /tmp/weknora-react-multiclient-derived CODE_SIGNING_ALLOWED=NO build`
- 安装并启动：`xcrun simctl install .../WeKnora.app`、`xcrun simctl launch ... com.weknora.mobile`

## 结果

- Xcode 原生编译成功：`** BUILD SUCCEEDED **`。
- Bundle ID `com.weknora.mobile` 安装成功。
- `simctl launch` 返回进程号 `91615`，应用可启动。
- 启动后显示 Expo Development Build 首页；页面提示 `No development servers found`，只有手动输入 URL 和历史服务器入口。
- 启动截图：`/tmp/weknora-react-multiclient-ios-launch.png`（1206×2622）。

## 证据边界

本项证明 iOS 原生工程可编译、安装和启动，不证明已进入 WeKnora 业务页面。由于当前模拟器没有可连接的 Expo development server，认证后的知识库/聊天交互仍需后续启动 Metro/Expo 服务后验收。
