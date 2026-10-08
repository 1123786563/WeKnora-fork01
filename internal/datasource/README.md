# datasource 模块（Pass A 骨架 + Pass B B2 搬迁完成）

外部知识源、连接器注册、同步、取消、重试、进度、凭据元数据、调度（§5.9/§5.18）

- **职责**：外部知识源连接、连接器注册、同步调度、取消/重试、进度与凭据元数据。
- **非职责**：文档与索引状态归 Knowledge；同步结果经 Knowledge 摄取端口进入知识域。
- **所有权依据**：`docs/architecture/backend-modules.yaml`（F0）；spec §5.18 覆盖矩阵。
- **Pass B 任务**：`B-datasource`（拆分横向包遗留文件、删除别名、收敛门面）——B2-DS.1~DS.8 已完成（2026-09-27）；门面实装（module.go 五操作）归 IB2（见 `docs/architecture/passb/briefs/b2-datasource.md` (d)）。

## 搬迁的包（move_packages → 目标）

以下 12 项已于 Pass A 集成期物理完成（`internal/datasource` 目录已不存在）；B2-DS.6 按空义务核销（Ruling LEGACY-ROW-OWNERSHIP「可早删不可晚删」+ K5.2 先例）删除 manifest `move_packages`/`alias_obligations` 双侧义务行，manifest 现值为 `[]`。下表保留为迁移轨迹记录：

| 现路径 | 目标路径 |
|---|---|
| `internal/datasource` | `internal/datasource/connector/moauth` |
| `internal/datasource/connector/confluence` | `internal/datasource/connector/moauth/connector/confluence` |
| `internal/datasource/connector/dingtalk` | `internal/datasource/connector/moauth/connector/dingtalk` |
| `internal/datasource/connector/feishu/core` | `internal/datasource/connector/moauth/connector/feishu/core` |
| `internal/datasource/connector/feishu/drive` | `internal/datasource/connector/moauth/connector/feishu/drive` |
| `internal/datasource/connector/feishu/wiki` | `internal/datasource/connector/moauth/connector/feishu/wiki` |
| `internal/datasource/connector/gitlab` | `internal/datasource/connector/moauth/connector/gitlab` |
| `internal/datasource/connector/ima` | `internal/datasource/connector/moauth/connector/ima` |
| `internal/datasource/connector/moauth` | `internal/datasource/connector/moauth` |
| `internal/datasource/connector/notion` | `internal/datasource/connector/moauth/connector/notion` |
| `internal/datasource/connector/rss` | `internal/datasource/connector/moauth/connector/rss` |
| `internal/datasource/connector/yuque` | `internal/datasource/connector/moauth/connector/yuque` |

## 横向包遗留文件（legacy_files）

原 4 个 legacy 文件已随 B2-DS.2/DS.3/DS.4 物理迁入本模块（`datasource/repository`、`datasource/service`、`datasource/handler`），manifest 行同 commit 删除；现 manifest `legacy_files` 仅余 **3 个 Pass B 过渡 shim**（宿主 compat，ib2 装配直连切换后随文件删行，明细与迁移轨迹见 [legacy/README.md](legacy/README.md)）：

- `internal/application/repository`：1（compat）
- `internal/application/service`：1（compat，含 cleanup seam 接线）
- `internal/handler`：1（compat）

## 集成点（F0）

- **routes**：1 项 — 见 manifest `integration_points.routes`
- **workers**：2 项 — 见 manifest `integration_points.workers`
- **lifecycle hooks**：2 项 — 见 manifest `integration_points.lifecycle_hooks`

## 验收命令

- `go test ./internal/datasource/connector/moauth/... -count=1`

导入方（importers，现为 3 个真实宿主——compat shim 所在：`internal/application/service`、`internal/container`、`internal/handler`）与禁改共享文件见同目录 manifest：
`docs/architecture/moves/datasource.yaml`。
