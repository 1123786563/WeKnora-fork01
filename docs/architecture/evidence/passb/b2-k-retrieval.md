# b2-k-retrieval — Pass B 差分与治理证据

> 节点：`b2-k-retrieval`（K2 Knowledge Retrieval，phase B2，plan `docs/plans/passb/22-knowledge-retrieval.md`）。
> 本文件随任务逐步落盘：K2.3 先落 import 例外计数基线变更登记（§1）；K2.8 补全 §7.3 四面高风险差分与 gates 台账。

## 1. import 例外计数基线变更登记（conventions §8 / Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY）

- **变更**：import 例外计数 105 → **107**（+2）。
- **时间/任务**：2026-09-25，K2.3（service 层语义文件物理迁移 `internal/application/service` → `internal/modules/knowledge/retrieval/app/`）。
- **批准依据**：协调者裁定 Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY（conventions §10.2 原文：「搬迁显形横向耦合 → 迁移节点以独立 commit 在 check.go importExceptions 登记精确 file→package 豁免（数据行非逻辑），同窗更新 exception-ledger.yaml 对应行 + 机械计数修正 + §8 基线登记」）。
- **变更清单**（file→package，两侧逐字一致）：

| id | importer | imported | plan | remove_at |
|---|---|---|---|---|
| exc-0106 | `internal/modules/knowledge/retrieval/app/semantic_model_capability.go` | `github.com/Tencent/WeKnora/internal/modules/commercial` | 22-knowledge-retrieval | ib2 |
| exc-0107 | `internal/modules/knowledge/retrieval/app/semantic_model_policy.go` | `github.com/Tencent/WeKnora/internal/modules/commercial` | 22-knowledge-retrieval | ib2 |

- **根因**：两文件在宿主 `internal/application/service` 时即 import commercial 域类型（`domain.Credits`/`FundingBYOK`/`PriceVersionRates`/`ValidateFunding`/`UsageFact`/`ChargeForCall` 等，semantic_model_capability.go:13、semantic_model_policy.go:11 base 实测）；迁入模块树后该既有横向 import 显形为 module→module，触发 architectureguard forbidden-import（诊断输出在案：两条 `forbidden-import … internal/modules/commercial`，guard 退出码 1）。
- **PassBTask 取值偏差说明**：计划 §5.6 模板写 `PassBTask=K2.7`，但 `tools/passbguard/check.go:83-93` `PassBTaskModule` 映射表仅含模块级 id（`B-knowledge` 等），`K2.7` 无映射将在 barrier `make check-passb-readiness` 触发 `exception-task-module` 诊断（check.go:240-247 判定逻辑直读）。本登记沿用 105 条既有条目的模块级 `B-knowledge`，并同步 ledger `plan: 22-knowledge-retrieval`（KnownModules 校验通过，check.go:61 映射 knowledge/ib2）。**此偏差已在 K2.3 报告登记，请协调者/评审确认，K2.7 执行时沿用同口径。**
- **登记位置**：`tools/architectureguard/check.go` importExceptions 尾部（数据行，非逻辑）；`docs/architecture/passb/exception-ledger.yaml` exc-0106/0107 + 头部计数注释 105→107。
- **提交**：独立 commit（与 K2.3 搬迁 commit 分离，满足 Ruling「独立 commit」与 conventions §4 提交隔离）。
- **回收**：remove_at=ib2；ib2 经门面/端口合法化或 ADR 修订后由 barrier 删除两侧行并回写计数。

## 2. import 例外计数基线变更登记——K2.4（conventions §8 / Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY）

- **变更**：import 例外计数 107 → **110**（+3）。
- **时间/任务**：2026-09-25，K2.4（按 Ruling 2026-09-25-DEFERRED-FILE-SPLIT 收缩范围：`internal/application/service` → `internal/modules/knowledge/retrieval/app/` 迁移 knowledgebase_access.go、slug_fuzzy.go、graph.go 三文件后显形）。
- **批准依据**：同 §1（Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY；计划 §5.6 种子表已预录 graph.go 两条与 knowledgebase_access.go 一条）。
- **变更清单**（file→package，两侧逐字一致；§5.6 种子表行 4/5 精确命中，无多退少补）：

| id | importer | imported | plan | remove_at |
|---|---|---|---|---|
| exc-0108 | `internal/modules/knowledge/retrieval/app/knowledgebase_access.go` | `github.com/Tencent/WeKnora/internal/modules/policy/access` | 22-knowledge-retrieval | ib2 |
| exc-0109 | `internal/modules/knowledge/retrieval/app/graph.go` | `github.com/Tencent/WeKnora/internal/modules/airesource/models/chat` | 22-knowledge-retrieval | ib2 |
| exc-0110 | `internal/modules/knowledge/retrieval/app/graph.go` | `github.com/Tencent/WeKnora/internal/modules/airesource/models/utils` | 22-knowledge-retrieval | ib2 |

- **PassBTask 口径**：沿用 §1 所述偏差（`B-knowledge` 模块级 id，非计划模板的 `K2.7`；passbguard PassBTaskModule 映射约束，K2.3 已登记待协调者确认，K2.7 沿用同口径）。
- **登记位置/提交/回收**：同 §1 模式（check.go 数据行 + ledger exc-0108..0110 + 头部计数 107→110；独立 commit；remove_at=ib2）。

## 3. K2.7 全量清点与例外处置（2026-09-25）

### 3.1 Step 1 全量清点（与 §5.6 种子表核对，多退少补）

- **命令勘误**：计划 K2.7 Step 1 原样命令 `grep -rn "internal/modules/" internal/modules/knowledge/retrieval/ --include="*.go" | grep -v "modules/knowledge"` 恒为空输出——`-rn` 的每条匹配行行首即文件路径 `internal/modules/knowledge/retrieval/...`，本身含 `modules/knowledge` 子串，`grep -v` 将全部行滤除。按计划意图（清点 import 路径指向其他模块的行）改过滤位置于引号内 import 路径，实际执行命令：
  `grep -rn 'WeKnora/internal/modules/' internal/modules/knowledge/retrieval/ --include="*.go" | grep -v '"github.com/Tencent/WeKnora/internal/modules/knowledge' | sort`
- **清点结果（2026-09-25，分支 255b41847 后）**：生产文件 5 对，与 check.go importExceptions 已登记 5 条（exc-0106..0110，K2.3/K2.4 两窗独立 commit 4a0e9190e/c9c9148e9）**一一对应，零缺失、零多余**：

| importer（落位文件） | imported | 登记行 |
|---|---|---|
| `internal/modules/knowledge/retrieval/app/graph.go:16` | `internal/modules/airesource/models/chat` | exc-0109 |
| `internal/modules/knowledge/retrieval/app/graph.go:17` | `internal/modules/airesource/models/utils` | exc-0110 |
| `internal/modules/knowledge/retrieval/app/knowledgebase_access.go:7` | `internal/modules/policy/access` | exc-0108 |
| `internal/modules/knowledge/retrieval/app/semantic_model_capability.go:13` | `internal/modules/commercial` | exc-0106 |
| `internal/modules/knowledge/retrieval/app/semantic_model_policy.go:10` | `internal/modules/commercial` | exc-0107 |

- **测试文件不计**：`app/semantic_model_capability_test.go:10`、`app/semantic_model_policy_test.go:9` 亦 import commercial，但 architectureguard 判定面跳过 `_test.go`（check.go:1224/:1284 实读），无需豁免。
- **种子表「多退」**（推迟件未迁移，行保留宿主、豁免待 ib2 迁移时按 Ruling 登记；宿主在 horizontalDirs 内不触发 forbidden-import）：`service/knowledgebase.go:15/:17/:18`（datasource、policy/access、policy/storageallowlist）、`service/kbshare.go:10`（policy/access）、`service/knowledgebase_search.go:9`（airesource/models/embedding）。另：种子表 `tag_access.go` 行经实测 grep **无任何跨模块 import**（种子表「其余执行时 grep」预期内），多列即退。

### 3.2 Step 2 登记核对结论（计数终值收口）

- check.go/exception-ledger.yaml **无新增数据行**：K2.7 清点时点全部 5 对已在 K2.3（105→107）与 K2.4（107→110）窗口按 Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY 独立 commit 登记（见本文件 §1/§2）。
- **计数三方核对（conventions §8 口径）**：check.go importExceptions 属主条目 5（`PassBTask: B-knowledge`）== ledger `plan: 22-knowledge-retrieval` 且 `remove_at: ib2` 行 5（exc-0106..0110）== evidence §1+§2 登记增量合计 5（105→110）。ledger 头部计数注释已为 **110**（B0.2 生成 105 + 本节点 5）。
- **PassBTask 口径偏差**（§1 已登记，此处收口）：计划 §5.6 模板 `PassBTask=K2.7` 与 passbguard `PassBTaskModule` 映射表（tools/passbguard/check.go:83-93，仅模块级 id）不兼容，沿用 `B-knowledge`，待协调者/评审确认（K2.8 Brief 复述）。

### 3.3 Step 3 exc-0089/0090/0091 处置结论（conventions §5 上报，本节点不改不删）

- **阻断证据（base 实读）**：
  1. `internal/types/interfaces/retriever.go:105/:112/:119`——冻结契约 `RetrieveEngineService` 的 `Index`/`BatchIndex`/`EstimateStorageSize` 形参直接使用 `embedder embedding.Embedder`（retriever.go:6 import `internal/modules/airesource/models/embedding`）；contracts.yaml:1726-1731 `knowledge.retrieve-engine-service` `stability: frozen`，framework:26 签名变化须 ADR/Spec 修订。删除 exc-0089（composite.go:13）/exc-0090（keywords_vector_hybrid_indexer.go:13）需先消除 embedding 类型出参/形参依赖=改冻结契约签名，本节点无权。
  2. `keywords_vector_hybrid_indexer.go:106/:121` 的 `utils.ChunkSlice` 调用与 :13 embedding import 同文件共存——exc-0091 无法独立于 exc-0090 所在文件的同一 import 面先行删除。
- **处置**：本节点不改 `internal/types/interfaces/retriever.go`、`internal/modules/knowledge/retriever/{composite,keywords_vector_hybrid_indexer}.go` 两个 Pass A 文件、不删 exc-0089/0090/0091 三行；`remove_at: ib2` 维持不变。
- **裁定建议（两分支，ib2 采纳其一）**：① 三行随 ib2 与 airesource 门面任务（embedding/utils 经模块根公共门面导出）收口后由 barrier 删除；② 走 ADR 修订 `RetrieveEngineService` 签名（去 `embedding.Embedder` 形参依赖）后由 barrier 删除。K0 §9「删除」义务按此执行到机制边界并留痕，不作现场改判（conventions §7.3）。

### 3.4 Step 4 GREEN 采证（K2.7 收口时点）

- `make check-backend-architecture` → `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16` / `architectureguard: OK (0 violations)`，退出码 0（633/23+23/58 计数基线不变，conventions §8）。
- `go build ./...` → 退出码 0。
- （完整命令原文与输出见 K2.7 任务报告 `.superpowers/sdd/passb/b2-k-retrieval/K2.7-report.md`；节点 gates 四项在 K2.8 Step 2 统一执行。）

## 4. 高风险差分（§7.3，K2.8 落盘）

（待 K2.8 Step 1 执行后补全：TypeIndexDelete tag 侧、HybridSearch/融合/FAQ 混排/分组、KB 活动审计流、KB 读权限/租户解析四面双跑。）
