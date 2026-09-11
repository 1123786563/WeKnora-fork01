# T21 iOS 原生上传完成与受保护下载证据 — 2026-09-12

## 范围与环境

- 当前 React 多端 worktree 的 `com.weknora.mobile` iPhone 17 Pro / iOS 26.5 模拟器。
- 隔离 Lite Go 后端：`127.0.0.1:18086`；SQLite 与本地文件目录均位于
  `/tmp/weknora-react-t21-ios-20260912`。
- 隔离 OpenAI-compatible embedding stub：`127.0.0.1:19997/v1`，只返回固定四维向量。
- 临时 owner 账号、viewer 账号和知识库均为本轮创建，未使用生产数据或用户现有凭证。
- 上传 fixture：`ios-t21-upload.txt`，144 bytes，SHA-256
  `a8bb6d2375da317ed42fac48884c76d2da07f578b551eef5bb544e869dde0419`。

## 原生上传与处理

1. iOS App 的 `Files` 页面显示 `0 files`，点击 `Upload` 后真实打开系统
   Files 文档选择器。
2. 通过 `xcrun simctl openurl` 将隔离 TXT fixture 放入模拟器 Files，使用系统选择器选中
   `ios-native-upload.txt`，返回 App 后页面显示 `1 files`、文件名和 `processing`。
3. Lite 后端创建文档 ID
   `ca8e7093-471f-47ca-add4-85df48e5aa6b`。绑定 embedding 模型后，
   `GET /api/v1/knowledge-bases/7eaa83cc-b004-45cc-b62f-243c9ad52e5e/knowledge`
   返回该文档 `parse_status=completed`、`pending_subtasks_count=0`、无错误消息；
   `summary_status=failed` 仅表示未配置可选 summary model。
4. 从知识库列表重新进入该页面后，原生列表实际显示
   `ios-native-upload.txt, completed`。详情页实际显示 `completed`、`Type: txt`、
   `Size: 144` 和上述服务端文档 ID。
5. `POST /v1/embeddings` 被本地 stub 记录并返回 HTTP 200；直接调用
   `POST /api/v1/knowledge-bases/{kb_id}/hybrid-search`，查询
   `completed upload fixture` 返回 1 条结果，结果正文与上传 fixture 一致。

## 受保护下载与原生分享

- owner 使用 Bearer 调用
  `GET /api/v1/knowledge/{document_id}/download`，返回 HTTP 200、
  `Content-Type: application/octet-stream`、
  `Content-Disposition: attachment; filename=ios-native-upload.txt`。
- 下载文件为 144 bytes，SHA-256 与原始 fixture 完全一致，证明受保护下载不是仅返回
  元数据或空文件。
- 原生详情页点击 `Download and share` 后真实打开 iOS Share Sheet；可访问性树和屏幕显示
  `ios-native-upload`、`文本文件 · 144 字节`，并提供 Copy、保存到“文件”和添加标签等系统动作。
  截图保存在 `/tmp/weknora-react-t21-ios-20260912/share-sheet.png`，SHA-256
  `effeac7ff56a15645546b37d4fbe0edcd830bb977fdfc8c271e0c9017afded71`。
- 另一个隔离 viewer 账号访问同一 download endpoint 返回 HTTP 403，响应为
  `Permission denied to access this knowledge base`，没有泄露文件字节。

## 结论与仍缺证据

本次通过：iOS 原生 Files 选择、multipart 上传、processing→completed 状态回读、原生详情、
真实 embedding 处理、混合检索结果、受保护下载字节/文件名校验、原生分享及跨账号 403。

T21 仍为 `review`：Android 尚未用同一 completed fixture 重跑，超大文件/取消/413、真实设备、
生产存储和 Web/移动同一账号同一数据的完整对照矩阵仍未完成。
