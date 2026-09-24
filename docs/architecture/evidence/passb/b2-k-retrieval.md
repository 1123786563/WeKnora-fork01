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

## 3. 高风险差分（§7.3，K2.8 落盘）

（待 K2.8 Step 1 执行后补全：TypeIndexDelete tag 侧、HybridSearch/融合/FAQ 混排/分组、KB 活动审计流、KB 读权限/租户解析四面双跑。）
