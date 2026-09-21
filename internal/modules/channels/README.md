# channels 模块（Pass A 骨架）

IM、Webhook、Embed Channel、外部身份/会话映射、入站标准化、出站投递（§5.11/§5.18）

- **职责**：IM 适配器、Channel、Embed、Webhook、外部身份与会话映射、入站标准化与出站投递。
- **非职责**：统一会话行为归 Conversation；身份主数据归 Identity。
- **所有权依据**：`docs/architecture/backend-modules.yaml`（F0）；spec §5.18 覆盖矩阵。
- **Pass B 任务**：`B-channels`（拆分横向包遗留文件、删除别名、收敛门面）。

## 搬迁的包（move_packages → 目标）

| 现路径 | 目标路径 |
|---|---|
| `internal/im` | `internal/modules/channels/im` |
| `internal/im/dingtalk` | `internal/modules/channels/im/dingtalk` |
| `internal/im/feishu` | `internal/modules/channels/im/feishu` |
| `internal/im/mattermost` | `internal/modules/channels/im/mattermost` |
| `internal/im/qqbot` | `internal/modules/channels/im/qqbot` |
| `internal/im/slack` | `internal/modules/channels/im/slack` |
| `internal/im/telegram` | `internal/modules/channels/im/telegram` |
| `internal/im/wechat` | `internal/modules/channels/im/wechat` |
| `internal/im/wecom` | `internal/modules/channels/im/wecom` |
| `internal/im/yunzhijia` | `internal/modules/channels/im/yunzhijia` |

## 横向包遗留文件（legacy_files）

共 7 个文件（明细与 Pass B 任务见 [legacy/README.md](legacy/README.md)）：

- `internal/application/repository`：1
- `internal/application/service`：3
- `internal/handler`：3

## 集成点（F0）

- **routes**：4 项 — 见 manifest `integration_points.routes`
- **workers**：0 项 — 见 manifest `integration_points.workers`
- **lifecycle hooks**：1 项 — 见 manifest `integration_points.lifecycle_hooks`

## 验收命令

- `go test ./internal/modules/channels/... -count=1`

导入方（import-path 修复对象，11 个）与禁改共享文件见同目录 manifest：
`docs/architecture/moves/channels.yaml`。
