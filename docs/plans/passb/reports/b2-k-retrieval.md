# b2-k-retrieval 节点实施报告（K2 Knowledge Retrieval，29 legacy 文件）

- **节点**：`b2-k-retrieval`（phase B2，role work；plan `docs/plans/passb/22-knowledge-retrieval.md`）
- **分支/worktree**：`codex/passb-b2-k-retrieval` @ `.worktrees/passb-b2-k-retrieval`
- **基线 SHA（双口径，K2.1 §4.1 登记）**：节点派发 BASE=`63d641c12`（K2.1 起点，= 派发时刻 integration 侧分支头）；`git merge-base origin/main HEAD`=`b1a3d6dd8`（b0 固定 base_sha，conventions §9 公式值；差异源于 integration 分支提交不在 origin/main，owned_files 核对按节点派发口径执行并两口径均留痕）
- **任务执行窗口**：2026-09-24（K2.1）～ 2026-09-25（K2.8）；任务级报告：`.superpowers/sdd/passb/b2-k-retrieval/K2.{1..8}-report.md`（主 checkout 会话区）
- **配套证据**：`docs/architecture/evidence/passb/b2-k-retrieval.md`（§1/§2 例外基线、§3 K2.7 清点、§4 四面差分、§5 垫片台账）；**Integration Brief**：`docs/architecture/passb/briefs/b2-k-retrieval.md`

## 0. 结论摘要

计划 29 文件中 **15 个物理落位**三个子包（repository 5 / app 7 / handler 3，含 26 文件 rename 72–100%、函数体逻辑零改动），**14 个按 Ruling 2026-09-25-DEFERRED-FILE-SPLIT 推迟**（manifest/matrix 行保留，Brief §5 载解除编排）。节点 DAG gates 四项全绿；§7.3 四面高风险差分 **118/118 用例 T0≡T1 零差异**；计数基线 633/23+23/58 三方一致不变；importExceptions 105→110（exc-0106..0110，独立 commit + ledger 同窗）。

## 1. 任务-提交映射（一任务一 commit，conventions §4）

| 任务 | 提交 | 内容 |
|---|---|---|
| K2.1 基线对齐+T0 | `f6bb35751`（merge） | b2-k0 合入（Ruling WAVE-DEP-BASELINE），零冲突；T0 台账采证 |
| K2.2 repository 层 | `34176d899` | 5 生产 + 5 测试 + escape_like_seam + repository compat + TEST-SHIM 垫片 + manifest 删 5 行 |
| K2.3 semantic 三件 | `e57c31c6b` + `4a0e9190e`（豁免） | 3 生产 + 4 测试留 2（偏差 2）+ guard 双轨 + service compat 初版；exc-0106/0107（105→107） |
| K2.4 KB 访问栈（收缩） | `626f1f865` + `c9c9148e9`（豁免） | 按 Ruling DEFERRED-FILE-SPLIT 收缩：access/slug_fuzzy/graph 3 生产 + 1 测试；exc-0108..0110（107→110） |
| K2.5 kb-activity（收缩） | `0014cabdb` | kb_activity.go 单文件 + 活动族 6 符号导出 + auditActor seam；kbshare/tag/tag_access 推迟 |
| K2.6 handler 层 | `255b41847` | 3 生产 + 3 测试 + handler compat；knowledgebase handler 维持推迟 |
| K2.7 例外收口 | `bc7aac9fa` | 全量清点核对（5/5 已登记零缺失零多余）+ exc-0089/90/91 处置上报；无新增数据行 |
| K2.8 差分+Brief+本报告 | （本提交） | evidence §4/§5、Brief、节点报告（含 T0 表转录） |

## 2. T0 特征化台账（K2.1 Step 3 采证转录；计划 K2.8 Files 行「补全 K2.1 T0 表」）

### 2.1 包级 T0（命令：`go test -count=1 ./internal/application/repository/ ./internal/application/service/ ./internal/handler/ ./internal/modules/knowledge/...`，退出码 0）

| 包 | 结果 |
|---|---|
| `internal/application/repository` | ok 285.210s |
| `internal/application/service` | ok 220.151s |
| `internal/handler` | ok 3.907s |
| `internal/modules/knowledge`（含 kbfreeze/retriever×N/searchutil/semantic/chunker/docparser…） | 全 ok；4 包 `[no test files]`（knowledge 根、retriever/{elasticsearch,neo4j,postgres}） |

无 FAIL、无 SKIP、无 panic/race；原始输出 `.superpowers/sdd/passb/b2-k-retrieval/K2.1-t0-test-output.txt`。

### 2.2 用例级 T0（差分锚定基础）

| 包 | 命令 | 退出码 | RUN | 顶层 PASS | 子测试 PASS | SKIP |
|---|---|---|---|---|---|---|
| repository | `go test -count=1 -v ./internal/application/repository/` | 0 | 709 | 503 | 190 | 1（TestAgentRunPostgres，Postgres 环境依赖） |
| service+handler | `go test -count=1 -v ./internal/application/service/ ./internal/handler/` | 0 | 3580 | 2508 | 1057 | 2（TestCraftRecoveryProcessFailureMatrix、TestCreateStore_DifferentEndpointSameIndex_Allowed，环境依赖） |

FAIL 总数 0。原始输出：`K2.1-t0-verbose-{repository,service-handler}.txt`。

## 3. §7.3 四面高风险差分（K2.8 Step 1；详见 evidence §4）

- **方法**：T0（§2.2 verbose 基线，宿主原实现）vs T1（2026-09-25 现态，15 文件已落位）；同一 awk 两遍提取 `name/verdict/子用例计数` → `sort -u` → `diff`，**diff 为空 = 逐用例等价**。
- **结果**：面 1（TypeIndexDelete tag 侧）14 用例、面 2（HybridSearch/融合/FAQ/分组）61 用例（7 测试文件全量，超计划「7+5+4 命中」锚定口径）、面 3（KB 活动审计流）11 用例、面 4（KB 读权限/租户解析）32 用例——**合计 118 顶层用例 + 105 子用例计数，T0 与 T1 完全一致，零差异**；差分失败数 0。
- **T1 命令**（5 条，原文+退出码见 evidence §4.2）：宿主 service 包 2 条（96.827s / 3.438s，exit 0）、落位 handler 包（2.230s，exit 0）、落位 repository 包（6.338s，exit 0）、补跑 1 条（2.461s，exit 0）。
- **面 2/面 1 部分推迟件口径**（如实）：7 个 search 生产文件与 tag.go（service）为推迟件，T1 跑宿主零改动原实现（等价性=代码零改动+结果一致双保险）；ib2 补迁时以本 T1 为新 T0 复跑（evidence §4.4）。

## 4. 节点 gates（K2.8 Step 2 逐条执行；DAG gates 原样命令，无替代）

| # | 命令（原文） | 退出码 | 关键输出摘录 |
|---|---|---|---|
| 1 | `go build ./...` | 0 | 仅预存 `ld: warning: ignoring duplicate libraries: '-lc++'`（cmd/desktop、cmd/server，基线固有） |
| 2 | `go test ./internal/modules/knowledge/... -count=1` | 0 | 19 包 ok（含 kbfreeze 2.339s、retrieval/app 2.264s、app/handler 2.836s、app/repository 54.424s）+ 4 包 no-test-files；FAIL 0 |
| 3 | `make check-backend-architecture` | 0 | `literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`；`architectureguard: OK (0 violations)` |
| 4 | `make verify-module-moves` | 0 | `modulemove: OK (16 manifests verified)` |

计数基线 633/23+23/58 三方一致不因本节点变化（conventions §8）。原始输出：`K2.8-gate{1-build,2-modules,3-arch,4-moves}.txt`。

## 5. 变更清单 vs owned_files（K2.8 Step 3；conventions §1.2）

`git diff --stat 63d641c12...HEAD`（K2.8 提交前）：43 文件，+1633/−188。`git diff --name-only 63d641c12...HEAD | sort` 分类核对：

| 类别 | 文件数 | §4 可写依据 |
|---|---|---|
| 物理搬迁生产 15（repository 5 / app 7 / handler 3） | 15 | §3.1（29 文件清单子集） |
| 随迁测试 11 | 11 | §7.1 表 |
| 落位包新文件（escape_like_seam / semantic_scope_guard / audit_actor_seam） | 3 | §4「3 个落位新包及包内新文件（本地 seam 文件）」 |
| 宿主 compat 3（repository/service/handler kbretrieval_passb_compat.go） | 3 | §4 明列 |
| manifest/镜像/例外：moves/knowledge.yaml、legacy/README.md、exception-ledger.yaml、tools/architectureguard/check.go | 4 | §4 明列（Ruling LEGACY-ROW-OWNERSHIP / IMPORT-EXCEPTION-REGISTRY 行级权限） |
| evidence/passb/b2-k-retrieval.md | 1 | §4 节点自身产出 |
| `semantic_scope_test.go`（2 行 R1 字段同步，K2.3 §4.4） | 1 | §7.1 归 K2.3（留守处置见 K2.3 偏差 2；§5.1 R1 机制） |
| `kbretrieval_passb_compat_test.go`（测试垫片） | 1 | **Ruling 2026-09-24-TEST-SUPPORT-SHIM**（conventions §10.3 授权；evidence §5 台账在册，remove_at=ib2） |
| 基线对齐 merge 带入（b2-k0 已评审产物） | 4 | Ruling WAVE-DEP-BASELINE（20-knowledge-program.md、reports/b2-k0.md、evidence/b2-k0.md、kbfreeze/freeze_test.go），非本节点写入 |

- **K2.8 本提交新增 3 文件**（evidence 追记、reports/b2-k-retrieval.md、briefs/b2-k-retrieval.md）：§4「本节点自身产出」。
- **结论**：43+3 文件全部落于 §4 可写清单或已登记 Ruling 授权面；**禁改清单零触碰**（router/routes_*/files、container、bootstrap、task.go/sync_task.go、migration、go.mod/go.sum、生产 SQL、contracts/ownership-matrix/event-catalog、module.go、他 owner 宿主文件——diff --name-only 不含任何禁改路径，`git diff 63d641c12...HEAD -- internal/router/ internal/container/ internal/bootstrap/` 为空）。
- **推迟件原位零改动实证**：14 个推迟文件 + rbac_lookups.go 的 `git diff 63d641c12...HEAD` 输出为空（验收标准 7）。
- **conventions 公式口径**（如实登记）：`git diff --stat b1a3d6dd8...HEAD` = 102 文件/+21063——含 b1a3d6dd8→63d641c12 间 integration 分支全部提交（b1 四计划、b0 五轮修复等），非本节点产物；本节点真实面以上表 43+3 为准。

## 6. 推迟件（14 个；Ruling 2026-09-25-DEFERRED-FILE-SPLIT）

详见 Brief §5（逐文件根因/解除编排）与 K2.4 报告 §3、K2.5 报告 §5：

1. `service/semantic_model.go`（plan §3.3 原推迟；双向互耦 semantic_model_budget.go）
2. `handler/knowledgebase.go`（plan §3.3 原推迟；rbac_lookups.go 跨包方法）
3-10. `service/knowledgebase.go`、`knowledgebase_search.go`、`_fanout`、`_faq`、`_fusion`、`_results`、`_shared`、`_storegroup`（K2.4 收缩；**勘误：K2.4 报告 §3 表列 7，实际 8 文件，`_shared` 漏列**）
11. `service/kbshare.go`（K2.5 收缩；identity 哨兵所有权待裁）
12. `service/tag.go`、13. `service/tag_access.go`（K2.5 收缩；白盒测试拆分待编）
14. `repository/kbshare.go`（**计划空位**：§3.1 列入但任务节未点名——上报协调者裁定归属，建议随 11 同窗）

推迟件 manifest/matrix 行均保留（未迁移不删行）；B5 396 口径不减免。

## 7. 未完成项 / 遗留（conventions §1.2 如实列出）

1. **14 推迟件未迁移**（§6）——解除依赖 K3/K4/identity/ib2 窗口；Brief §5 已载编排。验收标准 2/3 的「27 文件落位、27 行删除」按实际为 **15/15**，为 Ruling DEFERRED-FILE-SPLIT 授权偏差（K2.4/K2.5 任务报告 + 本报告留痕），非静默缩水。
2. **验收标准 4 的 12 符号导出义务**：已交付 11（含表外 `AuditScopeKnowledgeBase`）+ R1 字段 2（`Knowledge`/`Shares`）；`ApplyTenantRoleCap` 随 kbshare.go 推迟顺延（Brief §2）。
3. **exc-0089/0090/0091 未删**（冻结契约阻断，K2.7 Step 3 证据 + 两分支裁定建议已上报，Brief §7）。
4. **PassBTask 口径偏差待追认**：计划 §5.6 模板 `K2.7` vs passbguard 模块级映射，5 条豁免沿用 `B-knowledge`（K2.3 起登记，evidence §1）。
5. **passbguard 中间态**（barrier 门禁非节点门禁）：matrix 行/contracts 回写滞后类诊断与 BASE 同类（K2.4 §4 #7、K2.6 §4.2）；contracts.yaml tag-service 路径回写请求已入 Brief §8。
6. **测试垫片 1 件**（evidence §5，remove_at=ib2）；`semantic_scope_test.go`/`semantic_scope_mutation_test.go` 留宿主归属待 ib2 裁定（K2.3 偏差 2，Brief §4 identity 行）。
7. `NewGraphBuilder`（graph.go）全仓零消费——K5 门面设计时裁定（Brief §8.4）。

## 8. 公共可观察行为核验（plan §6 逐条）

1. HTTP 面：633 路由/方法/RBAC 门零变化（gate 3 实测；router 目录 diff 为空）；5 个 Register* 函数经 compat 别名解析同一实现。
2. 检索/融合/排序：面 2 61 用例 T0≡T1（推迟件零改动 + 全文件覆盖）；contracts frozen 面未触签名。
3. 审计面：面 3 11 用例逐用例一致；`ScopeType=knowledge_base` 经 `AuditScopeKnowledgeBase` 别名保持。
4. 错误文案/哨兵：compat 委托逐字保持（`ErrSemanticScope*`/`ErrSemanticModel*` var 别名在册；`ErrInvalidTenantID` 宿主原生）。
5. 装配面：dig Provider/Decorator 零变化（container.go diff 空；`SetSemanticScopeInvalidator` 断言面经 guard 双轨不变）；18 worker 注册行零改动。
6. 事件面：Knowledge 家族 4 事件 producer 不在本节点文件，零改动。

## 9. 自查（对照验收标准 1–8；标准 9 属评审）

| # | 标准 | 结论 |
|---|---|---|
| 1 | gates 四项全绿+命令留痕 | ✅ §4 |
| 2 | 27+2 文件落位 R100、函数体零改动 | **部分**：15/29 落位（14 推迟，Ruling 授权）；rename 72–100%（<100% 为 R1 改名机制必然，各任务报告逐块 diff 核验函数体零改动） |
| 3 | manifest 27 删/3 compat/2 推迟 | **实际**：15 删/3 compat/14 推迟保留（同上授权偏差）；`make verify-module-moves` OK |
| 4 | §5.1 12 符号导出+compat 在册 | 11 交付 + 1 顺延（§7.2）；compat 委托逐一在册 |
| 5 | §7.3 四面双跑一致 | ✅ 118/118 零差异（evidence §4） |
| 6 | 计数基线/例外一一对应 | ✅ 633/23+23/58 不变；exc-0106..0110 五条两侧一致、remove_at=ib2、基线登记 evidence §1/§2 |
| 7 | 推迟件原位零改动+编排入 Brief | ✅ §5 零改动实证 + Brief §5 |
| 8 | 禁改清单零触碰 | ✅ §5 |
