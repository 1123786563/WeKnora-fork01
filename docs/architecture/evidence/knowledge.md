# Evidence — Task A9 [PARALLEL MOVE]: Move Knowledge Packages

- Worker/分支：`bm-passa-a9`（worktree `.worktrees/bm-passa-a9`），基点 `918f90000`（IA2 后主线 HEAD）。
- Manifest（scope 权威）：`docs/architecture/moves/knowledge.yaml`；schema：`docs/architecture/moves/README.md`；基线：`docs/architecture/backend-baseline.md`（routes 633）。
- 冻结契约：`docs/architecture/frozen-entrypoints-batch-a2.md` —— A9 仅消费 execution/airesource/agentcatalog/policy 的 §1 路径与 §2 符号；未新增任何跨模块内部包 import（4 条 guard 诊断均为搬迁暴露的**预存**耦合，见 §6）。

## 1. Commits

| SHA | 主题 | 内容 |
|---|---|---|
| `1cc9a7bd3` | refactor(knowledge): move packages to internal/modules/knowledge | 纯 rename：142 文件全部 R100（`git diff --cached --name-status` 142×R100，0 insertions/deletions，`git diff --summary` 逐条 `rename … (100%)`） |
| `7fd56e4ac` | refactor(knowledge): repair imports and add pass-a aliases | 68 个非禁改 importer 的 import 行修复 + 18 个零逻辑别名包 + 1 处测试相对路径深度修复（86 files, +203/−85） |
| （待本 commit） | docs(knowledge): add integration brief, passb briefs and evidence | integration/knowledge.md、passb/knowledge-{ingest,process,wikifaq,retrieval}.md、evidence/knowledge.md |

## 2. Pre-move gate（基线记录）

- `go run ./tools/modulemove verify --module knowledge` → `modulemove: OK (knowledge)`
- manifest test_commands 旧路径等价基线（搬迁前原样执行）：
  - `go test ./internal/application/service/retriever/... ./internal/application/repository/retriever/... -count=1` → 13 包全 ok（service/retriever 1.6s；doris/es v7/es v8/milvus/opensearch/qdrant/sqlite/tencentvectordb/weaviate ok；elasticsearch、neo4j、postgres 无测试文件）
  - `go test ./internal/infrastructure/chunker/... ./internal/infrastructure/docparser/... ./internal/infrastructure/semantic/... ./internal/searchutil/... -count=1` → 5 包全 ok
  - `go test -tags anydoc -count=1 ./internal/infrastructure/docparser/...` → **build failed（本机环境限制）**：cgo 链接需要 `third_party/anydoc-go/lib/darwin_arm64/libanydoc_go`，该原生库未构建且本机无 cargo/rustc（`scripts/build-anydoc-lib.sh` 明示需要 Rust）。该命令仅 CI（anydoc.yml）具备条件；**搬迁前后失败原因逐字一致**（同缺同一库，非本次改造引入）。
- `go build ./...` → OK（仅既有的 `-lc++` duplicate libraries 链接告警）。
- 共享服务：postgres :5432 / redis :6379 运行中（未启停任何容器）。

## 3. Post-move 验证

- `go test ./internal/modules/knowledge/... -count=1` → 15 包全 ok：chunker、docparser、docparser/anydoc、retriever/{doris,elasticsearch/v7,elasticsearch/v8,milvus,opensearch,qdrant,service,sqlite,tencentvectordb,weaviate}、searchutil、semantic（elasticsearch 父包与 neo4j、postgres 无测试文件）。
- 直连消费方测试（全部 -count=1）：
  - `internal/container` ok（3.2s，含 retrieve_registry_wiring_test）
  - `internal/application/service` ok（100.6s）+ `chat_pipeline` ok + file/memory/metric/workbench ok
  - `internal/handler` ok + `internal/handler/session` ok（19.1s）
  - `internal/agent`、`internal/agent/tools` 及 agent/** 全 ok（含已知 flaky `agent/opencode` 15.3s 本次通过、`recoverytest` 26.8s）
  - `internal/modules/channels/im/...` 9 包全 ok
- `go build ./...` → OK；`go vet`（knowledge/container/handler/agent/channels + 旧路径别名包）→ 零输出。
- `go run ./tools/modulemove verify --module knowledge` → OK（搬迁后 from 消失 + to 存在语义）。
- `go run ./tools/architectureguard` → `literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16`，与 F0 基线一致；另有 §6 的 4 条 forbidden-import 新诊断（预存耦合暴露，未修、未加例外、未改工具）。

## 4. Rename / 零逻辑证据

- Move commit：142 文件 `R100`；repair commit 触及已搬迁包内文件的**全部内容变化仅 10 行**（9 行 import 路径 + 1 行测试相对路径深度），逐 hunk 复核（`git show 7fd56e4ac -- internal/modules/knowledge/`）：
  - `docparser/image_resolver_test.go`：`filepath.Join("..","..","..","testdata",…)` → 4 级 `..`（包深度 3→4，任务允许的 test-relative-path 修复，在此披露）。
  - chunker/splitter.go、docparser/{anydoc_reader,engines}.go 等：`internal/infrastructure/docparser|chunker` → 新模块路径（包内互引）。
  - retriever/elasticsearch/{v7,v8}/repository.go：父包 import → 新路径。
- 别名包 18 个，合计 26 符号（type alias + var 值转发，零函数体），每文件头部 `Deleted by Pass B task B-knowledge`。别名面 = 禁改文件（container.go）实际引用 ∪ channels/im 留用 3 符号；4 个零符号别名系 ruling 2 一一对应义务。**已验证无可变导出 var 被禁改文件赋值**（未触发 A6 LocalImageResolver 式翻转豁免）。

## 5. 禁改文件零触碰声明

`internal/container/container.go`、`internal/router/{router,task,sync_task}.go`、`go.mod`、`go.sum`、`migrations/` 在本分支**零 diff**（曾因 sed 排除模式失误短暂改动 container.go/im/service.go，发现后立即 `git checkout --` 还原并复核，最终提交不含）。`internal/modules/channels/im/service.go:28` 保留旧路径别名导入（A6 先例：他模块 owned 文件不触碰），移交 IA3。

## 6. 移交 IA3 的 guard 发现（未修、未加例外、未改工具）

1. `internal/modules/knowledge/docparser/weknoracloud_http_reader.go` → `modules/airesource/models/utils`
2. `internal/modules/knowledge/retriever/service/composite.go` → `modules/airesource/models/embedding`
3. `internal/modules/knowledge/retriever/service/keywords_vector_hybrid_indexer.go` → `modules/airesource/models/embedding`
4. `internal/modules/knowledge/retriever/service/keywords_vector_hybrid_indexer.go` → `modules/airesource/models/utils`

基点提交中同文件同内容即 import airesource 模块路径（当时文件在非模块路径，guard 不可见）——预存耦合被搬迁暴露。另：IA3 翻转 `channels/im/service.go:28` 时将新增第 5 条 `channels/im → modules/knowledge` 诊断，随翻转登记。

## 7. 假设与偏差记录

- 任务 brief 文件缺失：`task-A9-brief.md` 未由脚本生成，worker 按计划原文自行提取生成（extract-brief.sh A9 → 7 行 checklist），执行以任务指令 + manifest 为准。
- anydoc 基线失败为本机环境限制（无 Rust 工具链、原生库未构建），非代码问题；判据是搬迁前后失败输出一致且非 anydoc 模式 docparser 测试全绿。风险窗口由 CI anydoc.yml 覆盖。
- `internal/application/service/retriever` 落位为 `internal/modules/knowledge/retriever/service`（manifest `to:` 逐字；目录名 service、包名仍为 `retriever`，与 manifest 确定性规则一致）。
- 4 条 guard 诊断涉及符号面不变：搬迁零逻辑，error/retry/删除/索引语义未动（测试全绿 + guard 业务基线一致佐证）。
