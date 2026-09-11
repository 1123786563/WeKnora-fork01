# T21 iOS 原生文件选择器尝试 — 2026-09-12

## 范围

- 使用当前工作树构建并安装的 iPhone 17 Pro / iOS 26.5 Release 原生宿主。
- 后端为独立 Lite Go 进程：`127.0.0.1:18084`，临时 SQLite 数据库和临时本地文件目录。
- 使用临时测试账号和临时知识库；未使用用户现有数据、凭证或生产服务。

## 观察

1. 通过原生知识库列表登录后打开临时知识库，页面显示 `Files` 和 `Upload`。
2. 点击 `Upload` 后真实打开 iOS 系统 Files 文档选择器。首次进入“我的 iPhone”为空；使用
   `xcrun simctl openurl booted file:///tmp/ios-native-upload.txt` 将隔离的 26 字节 TXT fixture
   暴露给 Files 后，选择器显示该文件。
3. 选择 `ios-native-upload.txt` 后返回 WeKnora，列表显示 `1 files`、文件名和 `processing`。
   详情页显示服务端文档 ID `12cf045e-3da2-426f-bfe8-08b6668a30f0`、`Type: txt` 和 `Size: 26`。
4. 详情页的 `Download and share` 真实打开 iOS 分享面板，面板显示 `ios-native-upload`、`26 字节`，
   并提供复制、保存到文件和添加标签等系统动作；因为 Lite 隔离环境没有文档解析器，按钮仍显示
   `Preparing...`，未将下载完成误判为成功。

截图：`/tmp/weknora-ios-upload-picker.png`，SHA-256
`d08bafa8bf82bcda5849963089aeabc3a39c8756c20c0b549e13a9ae397f111a`。

## 结论

这次运行证明了 iOS 原生 `Upload` 控件、系统文档选择器、multipart 上传回写、processing 状态、详情
展示和系统分享面板接线可达。解析器未连接导致状态停留在 `processing`，且下载准备未完成，因此不
宣称 iOS 处理完成或受保护下载完成；后续仍需验证 completed 内容、下载字节/文件名和真实设备行为。

T21 仍为 `review`；Android 与本次 iOS 的 picker/upload/share 证据均为隔离 Lite，不能替代生产存储、
解析器和真实设备验收。
