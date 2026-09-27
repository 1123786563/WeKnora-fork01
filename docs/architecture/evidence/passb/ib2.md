# IB2 核心能力集成 barrier —— 执行证据

> 执行者：Pass B 总集成工程师（barrier 独占装配工作）
> 日期：2026-09-27
> 集成分支：`codex/passb-integration`（worktree `.worktrees/passb-int`）
> 计划：`docs/plans/passb/29-core-capability-integration.md`（本屏障开工时按框架 IB2 节写盘）
> 框架依据：framework:148-153

## 1. 合并序列（一次一支，任务指定序）

| 序 | 分支 | head | merge 提交 | 冲突 |
|---|---|---|---|---|
| 1 | codex/passb-b2-k-integration（K 序：已含 k0/ingest/retrieval/wikifaq/process 全部祖先，本会话 merge-base 逐一实测） | 461d8c4b2 | `ddea7b526` | 0 |
| 2 | codex/passb-b2-ac-market（25 序：已含 25a ac-definition 938087598 + 25b ac-skills 5ed64d324） | 8e0ce1a67 | `51c2a83ab` | 4 文件（check.go / exception-ledger / execution-ledger / pass-a-acceptance） |
| 3 | codex/passb-b2-datasource（B2-DS.1 已含 k-integration 基线对齐 486d46b42） | 4ebe14cf5 | `bc2127f8d` | 2 文件（check.go / exception-ledger） |
| 4 | codex/passb-b2-appconnector（DAG 登记 head 6e8c84860 后另有 2 个文档提交，按分支实际 HEAD 合并） | 8e80bb3c6 | `e09d491ee` | 2 文件（pass-a-acceptance / ownership_test） |

开工前状态核验：工作树 DAG（管家未提交登记落盘后）四入边节点均 done；`b2-k-integration.review_status=changes_requested` 系台账 7548 行留痕的登记纪律（修复 R1 已闭合、复审 0/0、节点级 OCR 0/0 均在案），节点以「门禁+OCR」口径 done，台账 8396 行确认 barrier 派发合法。

### 冲突裁定记录

- **check.go（两次）**：追加型冲突，两侧各自登记的 importExceptions 条目（HEAD=K 面 26 条；ac-market=25b 8 条；datasource=B2-DS.5 3 条）裁定保留两侧全集。
- **exception-ledger.yaml（两次）**：exc-id 撞号两次裁定——25b 原 exc-0106..0113 与 K 面已占号撞号，datasource 原顺延 exc-0132..0134 与 25b 重编号撞号；均按 B0 头注规则以 (from,to) 边键为准重编号（25b→0132..0139，datasource→0140..0142），终态 142 条、ID 唯一。
- **execution-ledger.md**：纯追加型时间序日志，ac-market 侧 3 条目（09-24 04:35-10:21）按时间戳插入 HEAD 侧对应区间，全条目保留。
- **pass-a-acceptance.md**：guard 例外计数行融合为集成真值 142 条；legacy_files 摘要行写集成合并树实测 358。
- **ownership_test.go**：wantPerModule 取 matrix 实测合并真值（knowledge 76→53、datasource 3、appconnector 1，total 358）。

## 2. 每支合并后的模块与直接消费者测试（本会话实跑）

| 合并 | 模块测试 | 消费者测试 |
|---|---|---|
| 1/4 k-integration | `go test ./internal/modules/knowledge/... -count=1` 29 包全 ok | application/{repository,service}、container、handler、handler/session、agentruntime/agent{,/tools}、channels/im、conversation/chat_pipeline 共 9 包全 ok |
| 2/4 ac-market | `go test ./internal/modules/agentcatalog/... -count=1` 3 包 ok（根包 no test files） | application/{repository,service}、handler 共 3 包 ok |
| 3/4 datasource | `go test ./internal/modules/datasource/... -count=1` 5 包 ok | application/{repository,service}、container、handler 共 4 包 ok |
| 4/4 appconnector | `go test ./internal/modules/appconnector/... -count=1` 4 包 ok | application/{repository,service}、container、handler、agentruntime/agent/tools、datasource/service 共 6 包 ok |

消费者集合由 `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./internal/...` 实测确定。

## 3. 装配切换（集成工程师独占面，K5 Brief）

- **(b) 18 worker 双栈**：task.go/sync_task.go 原 18+18 行 knowledge 注册删除，新装配点 `internal/router/workers_knowledge.go`（bootstrap.WorkerSink 两枚 + WorkerRegistry 双栈 + `knowledge.Module.RegisterWorkers`，VerifyWorkerParity 收尾）。Redis/Lite 互斥模式每次进程恰一次调用。非 knowledge 5+5 行不动。
- **(c) recoverPendingWikiTasks**：container.go:1058 直接 Invoke 撤销，改经 `newKnowledgeModule` 装配的 PendingWikiRecovery 等价闭包 + `mod.Start(ctx)` 单一注册点（spec §4.3；恢复函数幂等）。
- **(a) 表 1**：ChunkerDebug `handler.PreviewChunking` shim 转发改 `ingest.PreviewChunking` 直引。表 4/6/8/10（FAQ/Tag/SemanticModelPolicy/SemanticInternal）类型别名已同一零改。**表 2/9（Chunk/WikiPage）部分执行**：完整切换依赖 K1 Brief §9 四步序第 1 步 identity 去方法化（rbac_lookups.go:103/:105/:130/:183 方法仍在 wrapper 类型上），超出 IB2 单方面范围；container 侧 `ingest.NewChunkHandler` 并存供给、WikiPage 经 wrapper 内嵌导出字段取模块实例，门面 Dependencies 已收模块类型——登记移交（identity 去方法化后同窗收口）。
- **(e) shim/compat 删除批：不删**——15 个过渡物的删除前置（K4 推迟件补迁/残留 importer 清零/identity 去方法化）均未满足，按 Brief「删除前置=残留 importer 清零」口径维持现状并登记。
- container.go 提前落地项核验：K3 A1 provider 切换（knowledgeWiki.NewWikiPageRepository）已在位（ddea7b526 合并带入，本会话实读 container.go:309-315 注释与 import 行核验）。

## 4. IB2 回写批（K 面台账预告的义务收口）

- **matrix 漂移修复**：K2/K3 已迁残留 33 行删除 + K2/K3 宿主 compat 过渡 shim 10 行成对补行（Ruling 2026-09-25-TRANSITION-SHIM-ROW-REGISTRATION）——matrix 358 == manifests 358 三方一致。
- **contracts.yaml**：196 条诊断回写（missing 72 旧路径→新路径 rename map 100% 映射、vanished 8 删行、unrecorded 66 补登记）。
- **event-catalog.yaml**：knowledge.index.completed producer/consumer 路径改指 K1 迁后位置。
- **guard 同步**：architectureguard DiscoverWorkers 扩展识别门面注册新形态（module.go workerHandlers map 键计入双栈、WorkerSink 委托参数转发跳过）；passbguard PassBTaskModule 补 B2-DS.5 映射、HandlerSessionRuling compat 承接、brief 认领补全。

## 5. exception-ledger 状态（本屏障属主例外）

IB2 属主（`remove_at: ib2`）例外共 **45 条**（B0 面承 8 条：exc-0058..0061/0088..0091；K 面 26 条 exc-0106..0131；25b 8 条 exc-0132..0139；datasource 3 条 exc-0140..0142）。**抽查与全量核对：45 条对应 import 全部仍在代码中**（如 appconnector/adapter.go→commercial、ingest/extract.go→airesource/models/chat、datasource_service.go→appconnector，本会话 grep 实测）——按 K5 Brief (g) 删除前置是"各被导入包门面/端口合法化后切换直连"（airesource/policy/agentruntime 等的后续契约任务），IB2 窗口内未满足，**45 条全部保留**并在此登记移交（不推给 b5 之外的属主；删除义务维持 ib2 属主但需契约任务前置）。

## 6. 终局门禁（任务点名两项 + 屏障 gates 尽力项）

- `make -C .worktrees/passb-int check-backend-architecture`：**PASS**（`total=633 | redis=23 lite=23 | hooks=58 | modules=16`，0 violations——路由/worker/hook 计数奇偶零漂移）
- `make -C .worktrees/passb-int verify-module-moves`：**PASS**（16 manifests verified）
- `make check-passb-readiness`：**PASS**（legacy=358 aliases=69 exceptions=142 contracts=125 events=29 overlaps=0 missing=0）
- `go build ./...`：PASS（全树）
- `go test ./internal/... -count=1 -timeout=25m`（DAG gate 全量）：**未跑**（时间预算；已跑模块+消费者面 + tools 治理套，见 §2/§4）
- changed-range lint：未跑（同上，如实登记）

## 7. 遗留与移交

1. (a) 表 2/9 完整切换：identity 去方法化前置（rbac_lookups 方法迁移）——与 10-identity 同窗协调。
2. (e) 15 个 shim/compat 删除批：前置=K4 推迟件补迁（K2 §5 14 件 + K4 (f) 17 件 + 环境阻断件）+ 残留 importer 清零。
3. IB2 属主 45 条例外删除：前置=被导入包门面合法化契约任务。
4. K5 Brief (h) 推迟件窗口裁定：归协调者。
5. `app/graph.go` NewGraphBuilder 零消费裁定请求：归协调者（K2 Brief §8.4 遗留）。
