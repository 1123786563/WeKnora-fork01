# T21 Android 原生上传与取消边界证据 — 2026-09-12

## 环境

- 当前 React worktree 的 `com.weknora.mobile` release APK。
- `test36-small` Android API 36 emulator，serial `emulator-5554`。
- 隔离 Lite 后端运行于 `127.0.0.1:18087`；Android 通过 `10.0.2.2:8080` 访问受控本地 relay。
- 临时 owner 账号和知识库 `T21 Negative Upload KB`；未使用生产数据。

## 原生 multipart 上传

1. 真实 Android DocumentsUI 选择 `android-t21-complete.txt`（144 bytes）后，当前 native
   transport 通过 Expo FileSystem `createUploadTask` 发出 multipart 请求；服务端返回 HTTP 200，
   列表回读了一个 144-byte TXT 文档并显示 `processing`。
2. 这条链路不再把 Android `content://` URI 转成 JS Blob；picker 将文件复制到可寻址的 cache
   `file://` 源，shared client 只传递平台拥有的 `{uri,name,type,size}` 引用。

## 取消与大文件负例

- 真实 DocumentsUI 选择 10 MB TXT fixture 后，页面显示 `Cancel`；点击后页面返回 `Upload`，
  没有向用户显示网络异常。该证据只确认客户端的 AbortSignal/UI 状态边界。
- 受控 relay 记录了请求体可能已进入 Android/relay 的 socket 队列；即使客户端随后取消，
  服务端仍可能收到已在途字节。因此本轮不把客户端取消误报成服务端回滚或“服务端一定未
  收到请求”，也不宣称完整的 server-side cancellation proof。
- 真实 DocumentsUI 选择 1.10 MB fixture 后，页面显示 `文件大小不能超过1MB`。直接 Lite
  handler 返回 HTTP 400（JSON message 同上），知识库列表保持 0 files；本地 relay/生产
  ingress 的 HTTP 413 尚未观察到，不能用 400 代替 413。

## 结论

本轮补齐了 Android 当前 release APK 的原生 multipart 上传路径、取消按钮的客户端边界和
超大文件错误展示证据。T21 仍为 `review`：服务端取消语义、独立生产 ingress 的 413、真实
硬件设备、生产存储以及完整 Web/移动同账号对照矩阵仍未完成。
