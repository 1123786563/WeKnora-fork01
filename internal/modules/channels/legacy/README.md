# channels — legacy 遗留文件索引（横向包内归属本模块的文件）

与 `docs/architecture/moves/channels.yaml` 的 `legacy_files` 逐条镜像（同路径、同 Pass B 任务）。
非测试 `.go` 文件；同目录 `_test.go` 随主题文件一并搬迁。

| 文件 | 导航标签 | Pass B 任务 |
|---|---|---|
| `internal/application/repository/embed_channel.go` | Embed channel (application/repository) | `B-channels` |
| `internal/application/service/embed_channel.go` | Embed channel (application/service) | `B-channels` |
| `internal/application/service/embed_session.go` | Embed session (application/service) | `B-channels` |
| `internal/application/service/embed_webhook.go` | Embed webhook (application/service) | `B-channels` |
| `internal/handler/embed_channel.go` | Embed channel (handler) | `B-channels` |
| `internal/handler/im.go` | Im (handler) | `B-channels` |
| `internal/handler/wechat_qrcode.go` | Wechat qrcode (handler) | `B-channels` |
