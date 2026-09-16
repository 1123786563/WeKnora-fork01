# r102 iOS Release 内嵌业务启动验收

## 环境与命令

- 模拟器：iPhone 17 Pro，iOS 26.5，UDID `5EECD8BB-4B4A-473C-85C3-7841329FDF3C`
- Release 工程：`apps/mobile/ios/WeKnora.xcworkspace`
- 构建：`xcodebuild -workspace ios/WeKnora.xcworkspace -scheme WeKnora -sdk iphonesimulator -configuration Release -derivedDataPath /tmp/weknora-react-multiclient-release CODE_SIGNING_ALLOWED=NO build`
- 安装并启动：`xcrun simctl install`、`xcrun simctl launch ... com.weknora.mobile`

## 结果

- Release 原生构建成功：`** BUILD SUCCEEDED **`。
- App 安装并启动成功，未依赖 Metro development server。
- 首屏进入 WeKnora 业务登录页，显示品牌标题、`Sign in to your workspace`、Email、Password、Sign in、SSO、Create account、Join with invitation、Change server 等入口。
- 截图：`/tmp/weknora-react-multiclient-ios-release.png`（1206×2622）。

## 证据边界

该证据关闭 iOS Release 内嵌 JS 的业务启动层，证明原生包不是只停留在 Expo Development Build。尚未输入账号或执行登录，因此认证后的知识库、聊天和移动端权限交互仍需后续验收。
