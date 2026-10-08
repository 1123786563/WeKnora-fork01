# knowledge 模块（Pass A 骨架 + Pass B B2 集成态门面）

知识库、文档、Chunk、Tag、FAQ、Wiki、graph、检索、语义索引（§5.2/§5.18）

- **职责**：知识库、文档、Chunk、Tag、FAQ、Wiki、知识图谱、检索与语义知识索引。
- **非职责**：向量驱动配置归 AI Resource；摄取同步状态归 Data Source；docparser 引擎进程归 docreader（仓库内包归属 Knowledge）。
- **所有权依据**：`docs/architecture/backend-modules.yaml`（F0）；spec §5.18 覆盖矩阵。
- **Pass B 任务**：`B-knowledge`（拆分横向包遗留文件、删除别名、收敛门面）。

## 搬迁的包（move_packages → 目标）

| 现路径 | 目标路径 |
|---|---|
| `internal/application/repository/retriever/doris` | `internal/knowledge/retriever/doris` |
| `internal/application/repository/retriever/elasticsearch` | `internal/knowledge/retriever/elasticsearch` |
| `internal/application/repository/retriever/elasticsearch/v7` | `internal/knowledge/retriever/elasticsearch/v7` |
| `internal/application/repository/retriever/elasticsearch/v8` | `internal/knowledge/retriever/elasticsearch/v8` |
| `internal/application/repository/retriever/milvus` | `internal/knowledge/retriever/milvus` |
| `internal/application/repository/retriever/neo4j` | `internal/knowledge/retriever/neo4j` |
| `internal/application/repository/retriever/opensearch` | `internal/knowledge/retriever/opensearch` |
| `internal/application/repository/retriever/postgres` | `internal/knowledge/retriever/postgres` |
| `internal/application/repository/retriever/qdrant` | `internal/knowledge/retriever/qdrant` |
| `internal/application/repository/retriever/sqlite` | `internal/knowledge/retriever/sqlite` |
| `internal/application/repository/retriever/tencentvectordb` | `internal/knowledge/retriever/tencentvectordb` |
| `internal/application/repository/retriever/weaviate` | `internal/knowledge/retriever/weaviate` |
| `internal/application/service/retriever` | `internal/knowledge/retriever` |
| `internal/infrastructure/chunker` | `internal/knowledge/chunker` |
| `internal/infrastructure/docparser` | `internal/knowledge/docparser` |
| `internal/infrastructure/docparser/anydoc` | `internal/knowledge/docparser/anydoc` |
| `internal/infrastructure/semantic` | `internal/knowledge/semantic` |
| `internal/searchutil` | `internal/knowledge/searchutil` |

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

## 装配门面（K5.1，b2-k-integration）

`module.go`（`package knowledge`）已实装 contracts.yaml `knowledge.facade` 五操作（签名按真实 seam 推导，20 计划 §8 K5.1）：

```go
func NewModule(deps Dependencies) (*Module, error)
func (m *Module) RegisterRoutes() (HandlerSet, error)
func (m *Module) RegisterWorkers(redis, lite *bootstrap.WorkerRegistry) error
func (m *Module) Start(ctx context.Context) error
func (m *Module) Stop(ctx context.Context) error
```

- **Dependencies**：16 必填装配依赖（9 worker 分发面 = `router.AsynqTaskParams`/`SyncTaskParams` knowledge 子集 + 7 路由 handler 供给）+ 1 可选 `PendingWikiRecovery`；构造仍在 dig 容器（集成工程师独占），门面只收已构造实例（窄端口注入，spec §4.3）。缺失必填字段返回列出全部缺失名的错误。
- **RegisterWorkers**：18 任务类型按稳定序登记进 Redis/Lite 双栈 `bootstrap.WorkerRegistry` 并以 `bootstrap.VerifyWorkerParity` 校验双栈一致；与 `router/task.go`、`router/sync_task.go` 现行注册行逐条同构。
- **RegisterRoutes**：交付 7 组已落位 handler 的 `HandlerSet`（路由体与 rbacGuards 留驻 `internal/router`，本包不复刻路由表——双写禁令）；其余 4 项为宿主推迟件（3 组 handler：RegisterKnowledgeRoutes/RegisterKnowledgeBaseRoutes/RegisterKnowledgeBaseActivityRoutes + serveKBScopedFiles 文件服务面），ib2/补迁窗后补迁增补字段。
- **Start/Stop**：唯一生命周期挂点 = `recoverPendingWikiTasks` 等价入口（nil 时 no-op；幂等）；Stop 预留对称面。

**ib2 切换指针**：装配切换申请（11 路由形参切换 / 18 worker 双栈装配 / hook 切 Start / seam 接线 / shim 删除批 / 推迟件裁定）见 `docs/architecture/passb/briefs/b2-k-integration.md` (a)–(h)。门面当前为 M4 预备态（纯新增装配面、零生产引用），ib2 切换前无消费者。

## 验收命令

- `go test ./internal/knowledge/... -count=1`
- `go test -tags anydoc -count=1 ./internal/knowledge/docparser/...  # mirrors .github/workflows/anydoc.yml`

导入方（import-path 修复对象，12 个）与禁改共享文件见同目录 manifest：
`docs/architecture/moves/knowledge.yaml`。
