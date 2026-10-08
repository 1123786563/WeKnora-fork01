# b2-k-retrieval — Pass B 差分与治理证据

> 节点：`b2-k-retrieval`（K2 Knowledge Retrieval，phase B2，plan `docs/plans/passb/22-knowledge-retrieval.md`）。
> 本文件随任务逐步落盘：K2.3 先落 import 例外计数基线变更登记（§1）；K2.8 补全 §7.3 四面高风险差分与 gates 台账。

## 1. import 例外计数基线变更登记（conventions §8 / Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY）

- **变更**：import 例外计数 105 → **107**（+2）。
- **时间/任务**：2026-09-25，K2.3（service 层语义文件物理迁移 `internal/application/service` → `internal/knowledge/retrieval/app/`）。
- **批准依据**：协调者裁定 Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY（conventions §10.2 原文：「搬迁显形横向耦合 → 迁移节点以独立 commit 在 check.go importExceptions 登记精确 file→package 豁免（数据行非逻辑），同窗更新 exception-ledger.yaml 对应行 + 机械计数修正 + §8 基线登记」）。
- **变更清单**（file→package，两侧逐字一致）：

| id | importer | imported | plan | remove_at |
|---|---|---|---|---|
| exc-0106 | `internal/knowledge/retrieval/app/semantic_model_capability.go` | `github.com/Tencent/WeKnora/internal/commercial` | 22-knowledge-retrieval | ib2 |
| exc-0107 | `internal/knowledge/retrieval/app/semantic_model_policy.go` | `github.com/Tencent/WeKnora/internal/commercial` | 22-knowledge-retrieval | ib2 |

- **根因**：两文件在宿主 `internal/application/service` 时即 import commercial 域类型（`domain.Credits`/`FundingBYOK`/`PriceVersionRates`/`ValidateFunding`/`UsageFact`/`ChargeForCall` 等，semantic_model_capability.go:13、semantic_model_policy.go:11 base 实测）；迁入模块树后该既有横向 import 显形为 module→module，触发 architectureguard forbidden-import（诊断输出在案：两条 `forbidden-import … internal/commercial`，guard 退出码 1）。
- **PassBTask 取值偏差说明**：计划 §5.6 模板写 `PassBTask=K2.7`，但 `tools/passbguard/check.go:83-93` `PassBTaskModule` 映射表仅含模块级 id（`B-knowledge` 等），`K2.7` 无映射将在 barrier `make check-passb-readiness` 触发 `exception-task-module` 诊断（check.go:240-247 判定逻辑直读）。本登记沿用 105 条既有条目的模块级 `B-knowledge`，并同步 ledger `plan: 22-knowledge-retrieval`（KnownModules 校验通过，check.go:61 映射 knowledge/ib2）。**此偏差已在 K2.3 报告登记，请协调者/评审确认，K2.7 执行时沿用同口径。**
- **登记位置**：`tools/architectureguard/check.go` importExceptions 尾部（数据行，非逻辑）；`docs/architecture/passb/exception-ledger.yaml` exc-0106/0107 + 头部计数注释 105→107。
- **提交**：独立 commit（与 K2.3 搬迁 commit 分离，满足 Ruling「独立 commit」与 conventions §4 提交隔离）。
- **回收**：remove_at=ib2；ib2 经门面/端口合法化或 ADR 修订后由 barrier 删除两侧行并回写计数。

## 2. import 例外计数基线变更登记——K2.4（conventions §8 / Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY）

- **变更**：import 例外计数 107 → **110**（+3）。
- **时间/任务**：2026-09-25，K2.4（按 Ruling 2026-09-25-DEFERRED-FILE-SPLIT 收缩范围：`internal/application/service` → `internal/knowledge/retrieval/app/` 迁移 knowledgebase_access.go、slug_fuzzy.go、graph.go 三文件后显形）。
- **批准依据**：同 §1（Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY；计划 §5.6 种子表已预录 graph.go 两条与 knowledgebase_access.go 一条）。
- **变更清单**（file→package，两侧逐字一致；§5.6 种子表行 4/5 精确命中，无多退少补）：

| id | importer | imported | plan | remove_at |
|---|---|---|---|---|
| exc-0108 | `internal/knowledge/retrieval/app/knowledgebase_access.go` | `github.com/Tencent/WeKnora/internal/policy/access` | 22-knowledge-retrieval | ib2 |
| exc-0109 | `internal/knowledge/retrieval/app/graph.go` | `github.com/Tencent/WeKnora/internal/airesource/models/chat` | 22-knowledge-retrieval | ib2 |
| exc-0110 | `internal/knowledge/retrieval/app/graph.go` | `github.com/Tencent/WeKnora/internal/airesource/models/utils` | 22-knowledge-retrieval | ib2 |

- **PassBTask 口径**：沿用 §1 所述偏差（`B-knowledge` 模块级 id，非计划模板的 `K2.7`；passbguard PassBTaskModule 映射约束，K2.3 已登记待协调者确认，K2.7 沿用同口径）。
- **登记位置/提交/回收**：同 §1 模式（check.go 数据行 + ledger exc-0108..0110 + 头部计数 107→110；独立 commit；remove_at=ib2）。

## 3. K2.7 全量清点与例外处置（2026-09-25）

### 3.1 Step 1 全量清点（与 §5.6 种子表核对，多退少补）

- **命令勘误**：计划 K2.7 Step 1 原样命令 `grep -rn "internal/modules/" internal/knowledge/retrieval/ --include="*.go" | grep -v "modules/knowledge"` 恒为空输出——`-rn` 的每条匹配行行首即文件路径 `internal/knowledge/retrieval/...`，本身含 `modules/knowledge` 子串，`grep -v` 将全部行滤除。按计划意图（清点 import 路径指向其他模块的行）改过滤位置于引号内 import 路径，实际执行命令：
  `grep -rn 'WeKnora/internal/modules/' internal/knowledge/retrieval/ --include="*.go" | grep -v '"github.com/Tencent/WeKnora/internal/knowledge' | sort`
- **清点结果（2026-09-25，分支 255b41847 后）**：生产文件 5 对，与 check.go importExceptions 已登记 5 条（exc-0106..0110，K2.3/K2.4 两窗独立 commit 4a0e9190e/c9c9148e9）**一一对应，零缺失、零多余**：

| importer（落位文件） | imported | 登记行 |
|---|---|---|
| `internal/knowledge/retrieval/app/graph.go:16` | `internal/airesource/models/chat` | exc-0109 |
| `internal/knowledge/retrieval/app/graph.go:17` | `internal/airesource/models/utils` | exc-0110 |
| `internal/knowledge/retrieval/app/knowledgebase_access.go:7` | `internal/policy/access` | exc-0108 |
| `internal/knowledge/retrieval/app/semantic_model_capability.go:13` | `internal/commercial` | exc-0106 |
| `internal/knowledge/retrieval/app/semantic_model_policy.go:10` | `internal/commercial` | exc-0107 |

- **测试文件不计**：`app/semantic_model_capability_test.go:10`、`app/semantic_model_policy_test.go:9` 亦 import commercial，但 architectureguard 判定面跳过 `_test.go`（check.go:1224/:1284 实读），无需豁免。
- **种子表「多退」**（推迟件未迁移，行保留宿主、豁免待 ib2 迁移时按 Ruling 登记；宿主在 horizontalDirs 内不触发 forbidden-import）：`service/knowledgebase.go:15/:17/:18`（datasource、policy/access、policy/storageallowlist）、`service/kbshare.go:10`（policy/access）、`service/knowledgebase_search.go:9`（airesource/models/embedding）。另：种子表 `tag_access.go` 行经实测 grep **无任何跨模块 import**（种子表「其余执行时 grep」预期内），多列即退。

### 3.2 Step 2 登记核对结论（计数终值收口）

- check.go/exception-ledger.yaml **无新增数据行**：K2.7 清点时点全部 5 对已在 K2.3（105→107）与 K2.4（107→110）窗口按 Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY 独立 commit 登记（见本文件 §1/§2）。
- **计数三方核对（conventions §8 口径）**：check.go importExceptions 属主条目 5（`PassBTask: B-knowledge`）== ledger `plan: 22-knowledge-retrieval` 且 `remove_at: ib2` 行 5（exc-0106..0110）== evidence §1+§2 登记增量合计 5（105→110）。ledger 头部计数注释已为 **110**（B0.2 生成 105 + 本节点 5）。
- **PassBTask 口径偏差**（§1 已登记，此处收口）：计划 §5.6 模板 `PassBTask=K2.7` 与 passbguard `PassBTaskModule` 映射表（tools/passbguard/check.go:83-93，仅模块级 id）不兼容，沿用 `B-knowledge`，待协调者/评审确认（K2.8 Brief 复述）。

### 3.3 Step 3 exc-0089/0090/0091 处置结论（conventions §5 上报，本节点不改不删）

- **阻断证据（base 实读）**：
  1. `internal/types/interfaces/retriever.go:105/:112/:119`——冻结契约 `RetrieveEngineService` 的 `Index`/`BatchIndex`/`EstimateStorageSize` 形参直接使用 `embedder embedding.Embedder`（retriever.go:6 import `internal/airesource/models/embedding`）；contracts.yaml:1726-1731 `knowledge.retrieve-engine-service` `stability: frozen`，framework:26 签名变化须 ADR/Spec 修订。删除 exc-0089（composite.go:13）/exc-0090（keywords_vector_hybrid_indexer.go:13）需先消除 embedding 类型出参/形参依赖=改冻结契约签名，本节点无权。
  2. `keywords_vector_hybrid_indexer.go:106/:121` 的 `utils.ChunkSlice` 调用与 :13 embedding import 同文件共存——exc-0091 无法独立于 exc-0090 所在文件的同一 import 面先行删除。
- **处置**：本节点不改 `internal/types/interfaces/retriever.go`、`internal/knowledge/retriever/{composite,keywords_vector_hybrid_indexer}.go` 两个 Pass A 文件、不删 exc-0089/0090/0091 三行；`remove_at: ib2` 维持不变。
- **裁定建议（两分支，ib2 采纳其一）**：① 三行随 ib2 与 airesource 门面任务（embedding/utils 经模块根公共门面导出）收口后由 barrier 删除；② 走 ADR 修订 `RetrieveEngineService` 签名（去 `embedding.Embedder` 形参依赖）后由 barrier 删除。K0 §9「删除」义务按此执行到机制边界并留痕，不作现场改判（conventions §7.3）。

### 3.4 Step 4 GREEN 采证（K2.7 收口时点）

- `make check-backend-architecture` → `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16` / `architectureguard: OK (0 violations)`，退出码 0（633/23+23/58 计数基线不变，conventions §8）。
- `go build ./...` → 退出码 0。
- （完整命令原文与输出见 K2.7 任务报告 `.superpowers/sdd/passb/b2-k-retrieval/K2.7-report.md`；节点 gates 四项在 K2.8 Step 2 统一执行。）

## 4. 高风险差分（§7.3 四面，K2.8 Step 1 执行，2026-09-25）

### 4.1 双跑方法与 T0/T1 定义（spec §14.3 同输入双跑）

- **T0（旧实现特征化基线）**：K2.1 Step 3 用例级台账（2026-09-24 采集，时点=基线对齐 merge f6bb35751 后、K2.2 首个搬迁前，全部生产文件仍在宿主原位）——包级 `ok`（repository 285.210s / service 220.151s / handler 3.907s / knowledge 模块树全 ok）+ 全量 verbose 逐用例输出（`go test -count=1 -v ./internal/application/repository/`、`go test -count=1 -v ./internal/application/service/ ./internal/handler/`，两命令退出码 0，RUN 709+3580、FAIL 0）。原始文件：`.superpowers/sdd/passb/b2-k-retrieval/K2.1-t0-{test-output,verbose-repository,verbose-service-handler}.txt`（主 checkout git-ignored 会话区）。
- **T1（搬迁后同用例）**：2026-09-25，分支 HEAD=bc7aac9fa（K2.7 后；K2.8 采证时点），15 个生产文件已物理落位 `internal/knowledge/retrieval/app{,/repository,/handler}`、宿主 compat 3 文件在册。
- **比对口径**：两侧 verbose 输出经同一 awk 两遍提取（`name / verdict / 子用例计数`，子用例行晚于父行故 END 汇出）→ `sort -u` → `diff`。**判据：diff 为空 = 逐用例 verdict 与子用例计数完全一致**。

### 4.2 T1 命令台账（原文 + 退出码）

| # | 命令 | 退出码 | 关键输出 |
|---|---|---|---|
| 1 | `go test -count=1 -v -run '<95 用例联合 pattern（§7.3 四面锚定函数名 alternation）>' ./internal/application/service/` | 0 | `ok ...internal/application/service 96.827s`；顶层 PASS 92、FAIL/SKIP 0（宿主面：面 1 write_access、面 3 全部、面 4 全部 + 面 2 的 38 用例） |
| 2 | `go test -count=1 -v -run 'TestTagDeletionNeverIgnoresInvalidExclusions' ./internal/knowledge/retrieval/app/handler/` | 0 | `--- PASS: TestTagDeletionNeverIgnoresInvalidExclusions (0.00s)`（6 子用例）；`ok ... 2.230s` |
| 3 | `go test -count=1 -v -run 'TestSemanticScopeEpoch' ./internal/knowledge/retrieval/app/repository/` | 0 | 3 用例全 PASS（Fanout/AtomicFailures/UserUnion）；`ok ... 6.338s` |
| 4 | `go test -count=1 -v -run 'TestResolveKBReadTenantPreservesServiceBoundary' ./internal/application/service/` | 0 | `--- PASS`；`ok ... 2.461s`（补跑：#1 pattern 漏列此名，单独补齐后并入比对） |
| 5 | `go test -count=1 -v -run '<61 函数 pattern：由面 2 的 7 个测试文件 grep "^func Test" 生成全量 alternation>' ./internal/application/service/` | 0 | 61/61 顶层 PASS、FAIL/SKIP 0；`ok ... 3.438s`（面 2 全文件覆盖补强，超出计划「7+5+4 命中」锚定口径） |

T1 verbose 原始输出：`.superpowers/sdd/passb/b2-k-retrieval/K2.8-t1-{service-verbose,module-handler,module-repository,extra,face2-full}.txt`。

### 4.3 逐面比对结论（锚定用例清单 → 双跑 → 等价判据）

| 面（§7.3 行） | 锚定用例（顶层函数数） | T1 实现路径 | T0 | T1 | 比对 |
|---|---|---|---|---|---|
| 1. `TypeIndexDelete` tag 侧索引清理 | `knowledge_write_access_test.go` 10 + `tag_delete_test.go` 1（6 子用例）+ `semantic_scope_epoch_test.go` 3 = **14** | write_access 留宿主零改动（tag.go 推迟）；tag_delete 在落位 handler 包直跑；epoch 在落位 repository 包直跑 | 14 PASS | 14 PASS | **逐用例一致**（子用例计数一致） |
| 2. HybridSearch/融合/FAQ 混排/分组 | fanout 35 + fusion 5 + matchcount 2 + budget 3 + pr3 7 + dimension 2 + task_cancel 7 = **61**（7 文件全量） | 7 生产文件均 K2.4 推迟留宿主**零改动**（`git diff` 空，节点报告 §5），用例跑宿主原实现 | 61 PASS（子用例 21） | 61 PASS（子用例 21） | **逐用例一致**；等价性由零改动 + 同结果双保险 |
| 3. KB 活动审计流 | `kb_activity_test.go` 7 + `knowledge_shared_access_test.go` 4 = **11** | kb_activity.go 已落位 `app`，宿主测试经 compat 5 个一行委托 + 常量别名**直达新实现**；shared_access 主体 knowledge.go（K4）留宿主 | 11 PASS | 11 PASS | **逐用例一致**（K2.5 窗口 11/11 首跑复核通过） |
| 4. KB 读权限/租户解析 | `knowledge_caller_scope_test.go` 3 + `knowledge_shared_storage_failure_test.go` 1 + `semantic_scope_test.go` 15 + `semantic_scope_mutation_test.go` 13 = **32** | knowledgebase_access.go 已落位（测试经 compat 委托）；semantic_scope.go 已落位（测试经 type 别名 + R1 字段导出 `Knowledge`/`Shares` 白盒直达新实现） | 32 PASS | 32 PASS | **逐用例一致** |

**总比对**：118 顶层用例（含 105 子用例计数），`diff` 为空——T0 与 T1 verdict、子用例计数**完全一致，零差异**（差分失败数为 0，无「只修新实现」事项触发）。提取/比对中间产物：`K2.8-{t0,t1}-{extracted,face2-extracted,final}.txt`、锚定名单 `K2.8-anchor-names.txt`、`K2.8-face2-names.txt`。

### 4.4 面 2 推迟件的差分口径说明（如实）

§7.3 面 2 的 7 个生产文件（knowledgebase.go、knowledgebase_search*.go ×6）按 Ruling 2026-09-25-DEFERRED-FILE-SPLIT 推迟（K2.4 报告 §3），本窗口 T1 跑的是**宿主零改动原实现**（等价性=代码零改动+结果一致双保险）；ib2/后续补迁任务执行同一 §4.1 方法（T0=本节 T1 输出）再跑搬迁后差分。面 1 的 tag.go（service）同理（write_access 10 用例锚定推迟件行为）。已搬迁文件的差分（kb_activity、knowledgebase_access、semantic_scope、tag handler、epoch repo）**已闭环**。

## 5. 临时测试装置垫片（B5 清理范围）台账（Ruling 2026-09-24-TEST-SUPPORT-SHIM）

| 垫片文件 | 服务的留守测试（属主） | 最小符号 | 登记窗 | remove_at |
|---|---|---|---|---|
| `internal/application/repository/kbretrieval_passb_compat_test.go` | `internal/application/repository/knowledge_tag_test.go`（24-knowledge-process 属主，:219 构造/:241 调用随迁未导出类型 `knowledgeTagRepository`） | `type knowledgeTagRepository struct{ db *gorm.DB }` + `BatchCountReferences` 方法委托 `kbretrieval.NewKnowledgeTagRepository(r.db)`（不复制实现） | K2.2（commit 34176d899） | **ib2**（先到先删，最迟 B5） |

ib2 处置：knowledge_tag_test.go 随 K4 迁移或直连改写后，本垫片与 manifest 登记行同 commit 删除；B5 复核清零。
