# Integration Brief — knowledge (IA3)

适用对象：Pass A 集成者（Integrator）。任务：把 `bm-passa-a9` 分支的搬迁成果合入集成线
（IA3 顺序固定 K→C→AR，knowledge 第一个合入）。
本文件只描述 knowledge 模块（manifest：`docs/architecture/moves/knowledge.yaml`），
不动其他模块领地。

## 1. 本次搬迁（A9 已完成，SHAs 见 evidence 文档）

| from（旧导入路径） | to（新导入路径） |
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

共 142 个文件（含全部 `_test.go`），move commit 为**纯 rename**（142/142 相似度 100%，
0 insertions/deletions）。注意 `internal/modules/knowledge/retriever` 目录根部即检索服务包
（原 `application/service/retriever`，包名 `retriever`，12 个文件），与 doris/ elasticsearch/
等 12 个引擎仓子目录同层——这正是 manifest `to:` 的精确落位（合法 Go：一目录一包、
子目录为独立包）。
review round 1 Critical 修正：A9 首轮曾误落位为 `retriever/service/`（偏离 manifest），
fix commit 已把 12 个文件上移到 `retriever/` 根并重写全部 import（见 evidence §8）。

## 2. 旧路径别名（alias_obligations，18 处）

每个旧路径留了一个**零逻辑**转发别名包（type alias + var 值转发；每文件头部注明
`Deleted by Pass B task B-knowledge`）。别名面 = **禁改文件实际引用的符号**（A8 精确相等
先例），外加 `channels/im` 留在别名上的 3 个符号（见第 3 节）：

| 旧路径（别名包目录） | 转发符号 | 消费方 |
|---|---|---|
| `internal/application/repository/retriever/doris` | `NewDorisRetrieveEngineRepository`（1） | container.go |
| `internal/application/repository/retriever/elasticsearch` | （0，义务占位） | — |
| `internal/application/repository/retriever/elasticsearch/v7` | `NewElasticsearchEngineRepository`（1） | container.go |
| `internal/application/repository/retriever/elasticsearch/v8` | `NewElasticsearchEngineRepository`（1） | container.go |
| `internal/application/repository/retriever/milvus` | `NewMilvusRetrieveEngineRepository`（1） | container.go |
| `internal/application/repository/retriever/neo4j` | `NewNeo4jRepository`（1） | container.go |
| `internal/application/repository/retriever/opensearch` | `AuditSink`（type）+ `NewOpenSearchClient`/`NewRepository`/`WithAuditSink`（4） | container.go |
| `internal/application/repository/retriever/postgres` | `NewPostgresRetrieveEngineRepository`（1） | container.go |
| `internal/application/repository/retriever/qdrant` | `NewQdrantRetrieveEngineRepository`（1） | container.go |
| `internal/application/repository/retriever/sqlite` | `NewSQLiteRetrieveEngineRepository`（1） | container.go |
| `internal/application/repository/retriever/tencentvectordb` | `NewTencentVectorDBRetrieveEngineRepository`（1） | container.go |
| `internal/application/repository/retriever/weaviate` | `NewWeaviateRetrieveEngineRepository`（1） | container.go |
| `internal/application/service/retriever` | `RetrieveEngineRegistry`（type）+ `NewKVHybridRetrieveEngine`/`NewRetrieveEngineRegistry`/`NewVectorStoreRepoOwnership`（4） | container.go |
| `internal/infrastructure/chunker` | （0，义务占位） | — |
| `internal/infrastructure/docparser` | `SimpleFormatReader`（type）+ `NewGRPCDocumentReader`/`NewHTTPDocumentReader`/`NewImageResolver`/`IsImageFormat`/`IsSimpleFormat`（6） | container.go + channels/im/service.go |
| `internal/infrastructure/docparser/anydoc` | （0，义务占位） | — |
| `internal/infrastructure/semantic` | （0，义务占位） | — |
| `internal/searchutil` | （0，义务占位） | — |

18 个别名合计 26 个符号。其中 4 个零符号别名是 manifest ruling 2（别名与 move_packages
一一对应）的义务产物（A3 moauth 先例）。

## 3. 禁改共享文件中仍指向旧路径的精确行（file:line 清单 + 切换指令）

**`internal/container/container.go`（禁改）— 13 行 import（本分支 HEAD 精确行号）：**

| 行 | import（带 qualifier） |
|---|---|
| :40 | `dorisRepo "github.com/Tencent/WeKnora/internal/application/repository/retriever/doris"` |
| :41 | `elasticsearchRepoV7 "github.com/Tencent/WeKnora/internal/application/repository/retriever/elasticsearch/v7"` |
| :42 | `elasticsearchRepoV8 "github.com/Tencent/WeKnora/internal/application/repository/retriever/elasticsearch/v8"` |
| :43 | `milvusRepo "github.com/Tencent/WeKnora/internal/application/repository/retriever/milvus"` |
| :44 | `neo4jRepo "github.com/Tencent/WeKnora/internal/application/repository/retriever/neo4j"` |
| :45 | `openSearchRepo "github.com/Tencent/WeKnora/internal/application/repository/retriever/opensearch"` |
| :46 | `postgresRepo "github.com/Tencent/WeKnora/internal/application/repository/retriever/postgres"` |
| :47 | `qdrantRepo "github.com/Tencent/WeKnora/internal/application/repository/retriever/qdrant"` |
| :48 | `sqliteRetrieverRepo "github.com/Tencent/WeKnora/internal/application/repository/retriever/sqlite"` |
| :49 | `tencentVectorDBRepo "github.com/Tencent/WeKnora/internal/application/repository/retriever/tencentvectordb"` |
| :50 | `weaviateRepo "github.com/Tencent/WeKnora/internal/application/repository/retriever/weaviate"` |
| :55 | `"github.com/Tencent/WeKnora/internal/application/service/retriever"`（无 qualifier，包名 `retriever`） |
| :64 | `"github.com/Tencent/WeKnora/internal/infrastructure/docparser"`（无 qualifier） |

**给集成者的操作（knowledge 部分）：**
1. 把上述 13 行的路径前缀 `internal/application/repository/retriever/` →
   `internal/modules/knowledge/retriever/`、`internal/application/service/retriever` →
   `internal/modules/knowledge/retriever`、`internal/infrastructure/docparser` →
   `internal/modules/knowledge/docparser`。全部 qualifier 与 container.go 其余行**零改动**
   （已验证所有引用均为构造器/类型只读消费，**无可变导出 var 赋值**，无需 A6
   LocalImageResolver 式翻转）；
2. `go build ./...` 通过后，删除第 2 节所列 18 个别名目录（连同 alias.go）——
   除下述 `channels/im` 一处外，它们此刻全部无其他 importer；
3. 复跑第 9 节验收命令。

**`internal/router/router.go`、`internal/router/task.go`、`internal/router/sync_task.go`、
`go.mod`、`go.sum`、`migrations/`（均为禁改）：零处引用**（已 grep 验证；router 对本模块
仅经 `internal/handler` 与 `internal/application/service` 两个横向包间接消费，两包 Pass A
不整体搬移）。18 个 worker 注册（第 5 节）全部经 `params.*` 句柄与 `types.Type*` 常量，
不直接 import 本模块任何包。

**`internal/modules/channels/im/service.go:28`（非禁改，但属 channels 领地，A9 未触碰）：**
`"github.com/Tencent/WeKnora/internal/infrastructure/docparser"` 经别名编译
（消费 `IsImageFormat`/`IsSimpleFormat`/`SimpleFormatReader`，别名面已覆盖）。
A6 先例：他模块 owned 文件留给 Integrator 统一翻转。**IA3 处置**：切到
`internal/modules/knowledge/docparser` 时会在 guard 产生 `channels/im → modules/knowledge`
跨模块内部包诊断（见第 8 节第 5 条），需按 appconnector 先例登记精确路径例外
（临时，Pass B 删）或改走 knowledge 模块门面（Pass B）。

## 4. 路由集成点（routes，manifest 原文）

路由注册文件均在禁改的 `internal/router/` 内，Pass A 零触碰；handler 侧委托经
`internal/handler` 横向包（legacy_files，Pass B 拆分）：

- RegisterChunkerDebugRoutes — internal/router/routes_knowledge.go:18
- RegisterChunkRoutes — internal/router/routes_knowledge.go:28
- RegisterKnowledgeRoutes — internal/router/routes_knowledge.go:67
- RegisterKnowledgeBaseRoutes — internal/router/routes_knowledge.go:185
- RegisterFAQRoutes — internal/router/routes_knowledge.go:143
- RegisterSemanticModelPolicyRoutes — internal/router/routes_knowledge.go:254
- RegisterKnowledgeBaseActivityRoutes — internal/router/routes_knowledge.go:266
- RegisterKnowledgeTagRoutes — internal/router/routes_knowledge.go:279
- RegisterWikiPageRoutes — internal/router/routes_knowledge.go:309
- RegisterSemanticInternalRoutes — internal/router/routes_infra.go:14
- serveKBScopedFiles — internal/router/files.go（router.go:332 调用）

基线路由总数 633 不变（guard 复核 literal=564 + apiKeyRoute=69）。删除/索引语义
（TypeIndexDelete、TypeKBDelete、级联清理）是外部契约，Pass A 零行为变化。

## 5. Workers（18 项，Redis + Lite 同集合双注册）

knowledge 的 asynq 任务类型在禁改文件中注册（**A9 零触碰**；下列行号为 HEAD 快照）：

- **Redis mux**（`internal/router/task.go`）：TypeChunkExtract :266、TypeDataTableSummary :267、
  TypeDocumentProcess :270、TypeManualProcess :274、TypeFAQImport :277、
  TypeQuestionGeneration :280、TypeSummaryGeneration :283、TypeKBClone :286、
  TypeKnowledgeMove :289、TypeKnowledgeListDelete :292、TypeKnowledgeListReparse :298、
  TypeIndexDelete :301、TypeKBDelete :304、TypeImageMultimodal :307、
  TypeKnowledgePostProcess :310、TypeKnowledgeAutoTag :311、TypeWikiIngest :319、
  TypeWikiFinalize :320
- **Lite/sync executor**（`internal/router/sync_task.go`）：同一 18 项注册于 :143-:163
  （:146 为 conversation 的 TypeSummarizeMemory，不在本模块）。
  死信重点集合 task.go:378-380（TypeDocumentProcess/TypeKnowledgePostProcess/TypeManualProcess）。

处理器句柄全部来自 `internal/application/service`（横向包，legacy_files，Pass B 拆分），
guard 基线 Redis=23 / Lite=23 不变。

## 6. container.Invoke 生命周期挂点

- `recoverPendingWikiTasks` — `internal/container/container.go:1052`（Invoke）；
  函数体在 `internal/container/recover_pending_wiki_tasks.go:32`（**非禁改**文件；
  A9 已验证其不 import 本模块任何包，无需修理）。guard 基线 hooks=58 不变。

## 7. 配置键（搬迁包直接读取的 env）

| 键 | 位置 |
|---|---|
| `MAX_FILE_SIZE_MB` | internal/modules/knowledge/docparser/grpc_parser.go:23 |
| `IMAGE_HOST_KEEP_URL` | internal/modules/knowledge/docparser/image_resolver.go:255 |
| `DORIS_COMPAT_MODE` | internal/modules/knowledge/retriever/doris/compat.go:14 |
| `DORIS_TABLE_PREFIX` | internal/modules/knowledge/retriever/doris/repository.go:18 |
| `MILVUS_COLLECTION` / `MILVUS_METRIC_TYPE` | internal/modules/knowledge/retriever/milvus/repository.go:24-25 |
| `QDRANT_COLLECTION` | internal/modules/knowledge/retriever/qdrant/repository.go:18 |
| `WEAVIATE_COLLECTION` | internal/modules/knowledge/retriever/weaviate/repository.go:23 |
| `TENCENT_VECTORDB_DATABASE` / `_COLLECTION` / `_REPLICA_NUMBER` | internal/modules/knowledge/retriever/tencentvectordb/structs.go:11-13 |
| `OPENSEARCH_INDEX` | internal/modules/knowledge/retriever/opensearch/repository.go:117（经 types.ResolveIndexName） |
| `ELASTICSEARCH_INDEX` | internal/modules/knowledge/retriever/elasticsearch/{v7/repository.go:44,v8/repository.go:39} |

chunker / semantic / searchutil / retriever 根包 零自有 env 键（其余配置经
`*config.Config` 注入）。

## 8. Guard 新增发现（A9 Pass A 不修，交 IA3 处置）

architectureguard 在本分支报 **4 条新 forbidden-import** 诊断（基线其他指标不变：
routes 633 / redis 23 / lite 23 / hooks 58 / modules 16）。全部是**先于本次改造**的耦合被
搬迁暴露（这些文件在旧路径时代就已 import `internal/modules/airesource/models/*`，
A6 搬迁 airesource 时它们还在非模块路径，guard 不可见）：

1. `internal/modules/knowledge/docparser/weknoracloud_http_reader.go` → `modules/airesource/models/utils`
2. `internal/modules/knowledge/retriever/composite.go` → `modules/airesource/models/embedding`
3. `internal/modules/knowledge/retriever/keywords_vector_hybrid_indexer.go` → `modules/airesource/models/embedding`
4. `internal/modules/knowledge/retriever/keywords_vector_hybrid_indexer.go` → `modules/airesource/models/utils`

**处置建议（IA3）**：参照 batch-a1/a2 先例登记精确路径例外（B-knowledge 删），或
Pass B 改走 airesource 模块门面；不得为过 guard 回退本搬迁。另：IA3 翻转
`channels/im/service.go:28`（第 3 节）会新增第 5 条 `channels/im → modules/knowledge`
诊断，随该翻转一并登记。

## 9. 验收命令（集成后必跑）

- `go build ./...`
- `go test ./internal/modules/knowledge/... -count=1`（manifest test_commands）
- `go test -tags anydoc -count=1 ./internal/modules/knowledge/docparser/...`
  （镜像 .github/workflows/anydoc.yml；需先 `./scripts/build-anydoc-lib.sh` 构建原生库）
- `go test ./internal/container/... ./internal/application/service/... ./internal/handler/... ./internal/agent/... ./internal/modules/channels/im/... -count=1`（直接消费方回归）
- `go run ./tools/modulemove verify --module knowledge`
- `go run ./tools/architectureguard`（routes 633 / redis 23+lite 23 / hooks 58 不得变化；
  第 8 节 4 条诊断在集成处置前允许存在，其余必须 0）

## 10. Pass B 拆分简报（B-knowledge 四子任务）

横向包内 84 个 knowledge 遗留文件已按域拆为四份 Pass B brief（scope=各 legacy_files
子集）：`docs/architecture/passb/knowledge-ingest.md`（摄取 9 文件）、
`knowledge-process.md`（处理流水线 28 文件）、`knowledge-wikifaq.md`（Wiki+FAQ 18 文件）、
`knowledge-retrieval.md`（检索/KB/标签/语义模型 29 文件）。四份并集 == manifest
legacy_files 全集（84/84，脚本校验，两两无交）。18 个别名目录的删除义务
归 B-knowledge 主任务，随四子任务落地逐步回收。
