# datasource — legacy 遗留文件索引（横向包内归属本模块的文件）

与 `docs/architecture/moves/datasource.yaml` 的 `legacy_files` 逐条镜像（同路径、同 Pass B 任务）。
非测试 `.go` 文件；同目录 `_test.go` 随主题文件一并搬迁。
标 **已迁移** 的行是已物理落位 `internal/datasource/connector/moauth/...` 的迁移轨迹记录（manifest 行已按 Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP 随物理迁移 commit 删除，此镜像行保留至 ib2 收口）。

| 文件 | 导航标签 | Pass B 任务 |
|---|---|---|
| `internal/application/repository/datasource_repo.go` | Datasource repo (application/repository) | `B-datasource` — **已迁移**（B2-DS.2 → `datasource/repository/`） |
| `internal/application/repository/datasource_passb_compat.go` | Datasource host compat (application/repository) | `B-datasource`（B2-DS.2 新增过渡 shim，Ruling TRANSITION-SHIM-ROW-REGISTRATION，ib2 删） |
| `internal/application/service/datasource_service.go` | Datasource service (application/service) | `B-datasource` — **已迁移**（B2-DS.3 → `datasource/service/`） |
| `internal/application/service/datasource_passb_compat.go` | Datasource host compat (application/service) | `B-datasource`（B2-DS.3 新增过渡 shim，含 cleanup seam 接线，ib2 删） |
| `internal/handler/datasource.go` | Datasource (handler) | `B-datasource` — **已迁移**（B2-DS.4 → `datasource/handler/`） |
| `internal/handler/datasource_credentials.go` | Datasource credentials (handler) | `B-datasource` — **已迁移**（B2-DS.4 → `datasource/handler/`） |
| `internal/handler/datasource_passb_compat.go` | Datasource host compat (handler) | `B-datasource`（B2-DS.4 新增过渡 shim，ib2 删） |
