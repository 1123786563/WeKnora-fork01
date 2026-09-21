# knowledge 模块（Pass A 骨架）

知识库、文档、Chunk、Tag、FAQ、Wiki、graph、检索、语义索引（§5.2/§5.18）

- **职责**：知识库、文档、Chunk、Tag、FAQ、Wiki、知识图谱、检索与语义知识索引。
- **非职责**：向量驱动配置归 AI Resource；摄取同步状态归 Data Source；docparser 引擎进程归 docreader（仓库内包归属 Knowledge）。
- **所有权依据**：`docs/architecture/backend-modules.yaml`（F0）；spec §5.18 覆盖矩阵。
- **Pass B 任务**：`B-knowledge`（拆分横向包遗留文件、删除别名、收敛门面）。

## 搬迁的包（move_packages → 目标）

| 现路径 | 目标路径 |
|---|---|
| `internal/application/repository/retriever/doris` | `internal/modules/knowledge/retriever/doris` |
| `internal/application/repository/retriever/elasticsearch` | `internal/modules/knowledge/retriever/elasticsearch` |
| `internal/application/repository/retriever/elasticsearch/v7` | `internal/modules/knowledge/retriever/elasticsearch/v7` |
| `internal/application/repository/retriever/elasticsearch/v8` | `internal/modules/knowledge/retriever/elasticsearch/v8` |
| `internal/application/repository/retriever/milvus` | `internal/modules/knowledge/retriever/milvus` |
| `internal/application/repository/retriever/neo4j` | `internal/modules/knowledge/retriever/neo4j` |
| `internal/application/repository/retriever/opensearch` | `internal/modules/knowledge/retriever/opensearch` |
| `internal/application/repository/retriever/postgres` | `internal/modules/knowledge/retriever/postgres` |
| `internal/application/repository/retriever/qdrant` | `internal/modules/knowledge/retriever/qdrant` |
| `internal/application/repository/retriever/sqlite` | `internal/modules/knowledge/retriever/sqlite` |
| `internal/application/repository/retriever/tencentvectordb` | `internal/modules/knowledge/retriever/tencentvectordb` |
| `internal/application/repository/retriever/weaviate` | `internal/modules/knowledge/retriever/weaviate` |
| `internal/application/service/retriever` | `internal/modules/knowledge/retriever` |
| `internal/infrastructure/chunker` | `internal/modules/knowledge/chunker` |
| `internal/infrastructure/docparser` | `internal/modules/knowledge/docparser` |
| `internal/infrastructure/docparser/anydoc` | `internal/modules/knowledge/docparser/anydoc` |
| `internal/infrastructure/semantic` | `internal/modules/knowledge/semantic` |
| `internal/searchutil` | `internal/modules/knowledge/searchutil` |

## 横向包遗留文件（legacy_files）

共 84 个文件（明细与 Pass B 任务见 [legacy/README.md](legacy/README.md)）：

- `internal/application/repository`：13
- `internal/application/service`：58
- `internal/handler`：12
- `internal/handler/session`：1

## 集成点（F0）

- **routes**：11 项 — 见 manifest `integration_points.routes`
- **workers**：18 项 — 见 manifest `integration_points.workers`
- **lifecycle hooks**：1 项 — 见 manifest `integration_points.lifecycle_hooks`

## 验收命令

- `go test ./internal/modules/knowledge/... -count=1`
- `go test -tags anydoc -count=1 ./internal/modules/knowledge/docparser/...  # mirrors .github/workflows/anydoc.yml`

导入方（import-path 修复对象，12 个）与禁改共享文件见同目录 manifest：
`docs/architecture/moves/knowledge.yaml`。
