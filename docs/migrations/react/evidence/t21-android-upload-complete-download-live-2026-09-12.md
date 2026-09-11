# T21 Android 原生上传完成与受保护下载证据 — 2026-09-12

## 范围与环境

- 当前 React 多端 worktree 的 `com.weknora.mobile` release APK，SHA-256：
  `274c9fcaa9720de2fac6fc7648a61f06cd301b008ffad8552df4ce78c386399a`。
- AVD：`test36-small`，Android API 36，serial `emulator-5554`。
- 隔离 Lite Go 后端：`127.0.0.1:8080`，使用 `go run -tags sqlite_fts5
  ./cmd/server`；SQLite 数据库为
  `/tmp/weknora-react-t21-android-20260912.db`，本地文件根目录为
  `/tmp/weknora-react-t21-android-20260912/files`，通过
  `adb reverse tcp:8080 tcp:8080` 提供给模拟器。
- 隔离 OpenAI-compatible embedding stub：`127.0.0.1:19996/v1`，返回固定
  四维向量；服务端只在本地临时进程中使用，并将 `127.0.0.1` 显式加入
  `SSRF_WHITELIST_EXTRA`。
- 临时 owner 账号、模型、知识库和文件均为本轮创建，未使用生产数据或用户现有凭证。
- Fixture：`android-t21-complete.txt`，144 bytes，SHA-256
  `a8bb6d2375da317ed42fac48884c76d2da07f578b551eef5bb544e869dde0419`。

## 原生上传与处理

1. Release APK 清除旧会话后通过真实 Android 登录表单登录隔离 owner，并在知识库列表打开
   `React T21 Android Bound KB`。
2. Fixture 通过 `adb push` 放入模拟器 `Downloads`，点击原生 `Upload` 后打开真实
   Android `DocumentsUI`。选择 `android-t21-complete.txt` 返回 WeKnora，列表显示
   `1 files` 和 `android-t21-complete.txt, completed`。
3. 绑定 embedding 模型
   `19b3333e-389b-428a-aa53-d7d93f9a6529` 的知识库 ID 为
   `aed9d246-85de-4541-8998-d590badec063`。认证列表返回 HTTP 200、1 条记录、
   `parse_status=completed`、`pending_subtasks_count=0`；可选 summary 未配置，故
   `summary_status=failed`，不将其误报为摘要成功。
4. 详情页实际显示 `completed`、`Type: txt`、`Size: 144` 和文档 ID
   `1b23b0d6-566d-4202-9078-fd2050949315`。本地 embedding stub 记录了 HTTP 200
   的 embedding 请求。
5. 认证 hybrid-search 请求使用 `query_text=completed upload fixture` 返回 HTTP 200 和
   1 条结果，结果正文包含上传 fixture 的完整文本；SQLite 日志同时记录 vector 与
   keyword 两路各匹配 1 条并完成 RRF fusion。

## 受保护下载与原生分享

- owner 调用
  `GET /api/v1/knowledge/1b23b0d6-566d-4202-9078-fd2050949315/download` 返回 HTTP 200、
  `Content-Type: application/octet-stream`、`Content-Disposition: attachment;
  filename=android-t21-complete.txt`、`Content-Length: 144`。
- 下载文件为 144 bytes，SHA-256 为
  `a8bb6d2375da317ed42fac48884c76d2da07f578b551eef5bb544e869dde0419`，与原始 fixture
  完全一致，证明下载返回的是受保护文件字节而非仅元数据。
- 原生详情页点击 `Download and share` 后真实打开 Android Sharesheet；可访问性树显示
  `Sharing 1 file` 和 `android-t21-complete.txt`。截图位于
  `/tmp/weknora-react-t21-android-20260912/share-sheet.png`，SHA-256：
  `cc1c5a8129d38c91f9dd593e88bd89e54d1319115c08dabcbdfd16a35b9bef12`。
- 详情页 UIAutomator dump 包含 `completed`、文档 ID、`Type: txt`、`Size: 144` 和
  `Download and share`；dump SHA-256：
  `74cfa6e99b48bd6fd93eb6268d396566b10112d047ba2525bd69af1393958795`。

## 结论与仍缺证据

本次通过：Android 原生 DocumentsUI 选择、multipart 上传、processing→completed 状态
回读、embedding 处理、混合检索结果、受保护下载字节/文件名校验、原生 Sharesheet 以及
Android completed-fixture 与 iOS completed-fixture 的同字节对照。

T21 仍为 `review`：取消上传、超大文件、413 负例、Android 跨账号 403、真实设备、
生产存储和 Web/移动同一账号同一数据的完整对照矩阵仍未完成。
