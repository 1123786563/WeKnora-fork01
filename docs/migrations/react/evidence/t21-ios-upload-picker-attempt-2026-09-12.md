# T21 iOS 原生文件选择器尝试 — 2026-09-12

## 范围

- 使用当前工作树构建并安装的 iPhone 17 Pro / iOS 26.5 Release 原生宿主。
- 后端为独立 Lite Go 进程：`127.0.0.1:18084`，临时 SQLite 数据库和临时本地文件目录。
- 使用临时测试账号和临时知识库；未使用用户现有数据、凭证或生产服务。

## 观察

1. 通过原生知识库列表登录后打开临时知识库，页面显示 `Files` 和 `Upload`。
2. 点击 `Upload` 后真实打开 iOS 系统 Files 文档选择器。
3. 进入系统选择器的“我的 iPhone”位置后显示为空，没有可选文档；关闭选择器后回到 Files 页面，未产生上传请求或文件行。

截图：`/tmp/weknora-ios-upload-picker.png`，SHA-256
`d08bafa8bf82bcda5849963089aeabc3a39c8756c20c0b549e13a9ae397f111a`。

## 结论

这次运行证明了 iOS 原生 `Upload` 控件和系统文档选择器的接线可达，但没有文件可供选择，因此不宣称 iOS 上传、处理、下载或分享通过。需要后续提供可被 Simulator Files provider 暴露的 fixture（或在真实设备上选择文件）后，继续验证 multipart 上传、处理状态、受保护下载和分享。

T21 仍为 `review`；Android 已有独立 picker/upload/share 证据，不能替代本次 iOS 文件选择证据。
