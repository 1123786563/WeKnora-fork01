# insights — legacy 遗留文件索引（横向包内归属本模块的文件）

与 `docs/architecture/moves/insights.yaml` 的 `legacy_files` 逐条镜像（同路径、同 Pass B 任务）。
非测试 `.go` 文件；同目录 `_test.go` 随主题文件一并搬迁。
标 **已迁移** 的行是已物理落位 `internal/insights/...` 的迁移轨迹记录（manifest 行已按 Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP 随物理迁移 commit 删除，此镜像行保留至 ib3 收口）。

| 文件 | 导航标签 | Pass B 任务 |
|---|---|---|
| `internal/application/repository/analytics.go` | Analytics (application/repository) | `B-insights` — **已迁移**（B3-IN.3 → `insights/analytics/`） |
| `internal/application/repository/insights_passb_compat.go` | Insights host compat (application/repository) | `B-insights`（B3-IN.3 新增过渡 shim，Ruling TRANSITION-SHIM-ROW-REGISTRATION，ib3 删） |
| `internal/application/service/dataset.go` | Dataset (application/service) | `B-insights` — **已迁移**（B3-IN.4 → `insights/evaluation/`） |
| `internal/application/service/evaluation.go` | Evaluation (application/service) | `B-insights` — **已迁移**（B3-IN.4 → `insights/evaluation/`） |
| `internal/application/service/metric_hook.go` | Metric hook (application/service) | `B-insights` — **已迁移**（B3-IN.4 → `insights/evaluation/`） |
| `internal/application/service/insights_passb_compat.go` | Insights host compat (application/service) | `B-insights`（B3-IN.4 新增过渡 shim，含 cleanup seam 接线，Ruling TRANSITION-SHIM-ROW-REGISTRATION，ib3 删） |
| `internal/handler/analytics.go` | Analytics (handler) | `B-insights` — **已迁移**（B3-IN.3 → `insights/analytics/`） |
| `internal/handler/evaluation.go` | Evaluation (handler) | `B-insights` — **已迁移**（B3-IN.4 → `insights/evaluation/`） |
| `internal/handler/insights_passb_compat.go` | Insights host compat (handler) | `B-insights`（B3-IN.3 新增过渡 shim，B3-IN.4 同 commit 追加 evaluation 面，Ruling TRANSITION-SHIM-ROW-REGISTRATION，ib3 删） |
