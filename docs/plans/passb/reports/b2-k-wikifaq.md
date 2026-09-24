# 实施报告 — b2-k-wikifaq（23-knowledge-wikifaq，K3.1–K3.4 全部完成）

> 节点分支 `codex/passb-b2-k-wikifaq`。行号与命令输出基准：K3.4 收口 HEAD（见 §1 命令包）。
> 报告状态：K3.4 收口更新（evidence 终稿四行差分表 + hook 恢复手工差分双跑 + 门禁终跑；K3.1–K3.3 内容见 git 历史与 evidence §1–§2）。

## 0. 任务与提交对照

| 任务 | commit | 说明 |
|---|---|---|
| P2 基线对齐 | `1c9d812d0` | merge: passb b2-k-wikifaq ← b2-k0（head `5bcb798621`；Ruling 2026-09-24-WAVE-DEP-BASELINE） |
| K3.1 wiki 域 12 文件迁入 | `b981d13e3` | `refactor(knowledge): K3 wiki 域 12 文件迁入 internal/modules/knowledge/wiki` |
| K3.1 审阅修复 | `7ffaf6cc4` | NewSpan typed-nil 归一（W2 span 适配器 nil 语义穿透，OCR R1） |
| K3.2 faq 域 6 文件迁入 | `f6dfba041` | `refactor(knowledge): K3 faq 域 6 文件迁入 … 并保持冻结端口委托` |
| K3.2 例外登记 | `b9a82d2c6` | faq 2 条 import 例外（Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY） |
| K3.2 审阅修复 | `b15dec368` | deleteFAQChunkVectors 扣减块恢复 BASE 结构（review finding critical） |
| K3.3 Brief 与断链登记 | `1a4505abd` | `docs(passb): b2-k-wikifaq Integration Brief 与断链登记` |
| K3.4 差分证据与门禁收口 | （本 commit） | `docs(passb): b2-k-wikifaq 差分证据与门禁收口`（evidence §3 终稿 + 本报告追加） |

## 1. conventions §1.2 命令包（K3.4 终跑 = 最终 HEAD 工作树，2026-09-25；K3.3 期同套命令均绿，见 git 历史）

基线说明：DAG `b2-k-wikifaq.base_sha` 尚未由协调者回填（本地副本 `status=pending` 滞后，§9 协调者独占）；本报告以**节点工作起点 = 基线对齐 merge `1c9d812d0`**（其后首个节点自有 commit 为 `b981d13e3`，`git log --format='%h %p' -1 b981d13e3` 实测父即 `1c9d812d0`）。

| 命令（原文） | 退出码 | 关键输出 |
|---|---|---|
| `go build ./...` | 0 | 仅 `ld: warning: ignoring duplicate libraries: '-lc++'`（cmd/desktop、cmd/server 链接器警告，与迁移无关） |
| `go test -count=1 ./internal/modules/knowledge/...` | 0 | 18 个有测试包 `ok`、0 `FAIL`；含 `internal/modules/knowledge/faq`、`…/kbfreeze`（2 守卫）、`…/wiki` |
| `make check-backend-architecture` | 0 | `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`；`OK (0 violations)` |
| `make verify-module-moves` | 0 | `modulemove: OK (16 manifests verified)` |
| `git diff --stat 1c9d812d0..HEAD` | 0 | `57 files changed, 2326 insertions(+), 556 deletions(-)`（K3.4 commit 仅追加修改已列 2 产物文件——evidence 与本报告——路径集合不变） |
| `git diff --name-only 1c9d812d0..HEAD \| sort` | 0 | 57 路径（§2 逐条归类）；禁改文件 pattern（rbac_lookups/qa.go/knowledge.go/routes_knowledge/task.go/go.mod/internal/types/kbfreeze/module.go/治理三件套/recover_pending/reset_pending）grep 零命中（exit 1） |
| hook 恢复双跑 | 0/0 | `go test -count=1 -v ./internal/container/ -run 'TestRecoverPendingWikiTasks_RecreatesOneTriggerPerLaneAndKB\|TestResetPendingTasks_DurableWikiOpSurvivesLiteRestart\|TestResetPendingTasks_LiteWikiDoesNotHideOtherLostSubtasks'`——旧侧（`git worktree add --detach /tmp/k34-base-1c9d812d 1c9d812d0`）与新侧（HEAD）均 3/3 PASS，日志逐字节一致（removed 2 / recreated 3） |

计数奇偶（conventions §8）：633 路由（564+69）/ 23+23 worker / 58 hooks，guard 实测 == `docs/architecture/evidence/pass-a-acceptance.md` 台账（:22/:23/:24）== manifests 发现值（三方一致）。本节点零路由/worker/钩子增删。

K3.1/K3.2 期实跑命令与 T0/T1/T2 双跑输出：见 `docs/architecture/evidence/passb/b2-k-wikifaq.md` §0–§2（含 `go test ./internal/application/repository/` 全包 357s、`go vet`、`gofmt -l` 空等），此处不重复粘贴。

## 2. 变更清单 vs owned_files 逐条核对

`git diff --name-only 1c9d812d0..HEAD | sort` 基线为 55 文件；K3.3 追加 3 项变更（legacy/README.md 补删、Brief、本报告）后共 57 文件；K3.4 仅修改其中已列 2 产物文件（evidence、本报告），**路径集合终态仍为 57**，逐条归类：

| 类别（计划 §3.2 授权条目） | 文件 | 核对 |
|---|---|---|
| 18 legacy 文件 + 16 随迁测试（§3.1 表） | wiki 12 + faq 6（git mv rename 识别，evidence §1.5/§2.7）+ 14 wiki 测试 + 2 faq 测试 → `internal/modules/knowledge/wiki\|faq/**` | ✓ 全部命中 §3.1 |
| 新包落位新增 | `wiki/seams.go`+`seams_test.go`（W0）、`faq/service.go`+`service_test.go`（F0，含 K3.2 Step 2 TDD 锚定测试） | ✓ §6.1 |
| 宿主过渡兼容（§6.2 + 实施增量） | W1 `wiki_k3_compat.go`、W2 `wiki_k3_ctor_compat.go`、H1 `wiki_page_k3_compat.go`、H2 `faq_k3_compat.go`、S1 `wiki_fixer_scope_compat.go`、D1 `knowledge_faq_k3_delegate.go`；**增量 W3** `repository/wiki_k3_repo_compat.go`（escapeLikePattern/5 哨兵物理留驻裁定，文件头登记） | ✓ §3.2「本节点产出的宿主过渡兼容新文件」；7 行均已同 commit 登记 knowledge.yaml（:343-369） |
| 伴生/垫片测试（本节点新建 `_test.go`） | `wiki_k3_compat_test.go`（TestRepairContentLinks 宿主真实 seam 双跑）、`wiki_k3_span_adapter_test.go`、`wiki_k3_test_support_shim_test.go`、`knowledge_faq_k3_test_support_shim_test.go` | ✓ Ruling 2026-09-24-TEST-SUPPORT-SHIM（垫片文件头注 Ruling ID + remove_at ib2）；compat 伴生测试属本节点自有新文件 |
| 治理/例外（Ruling 授权） | `tools/architectureguard/check.go`（importExceptions 数据行 13 条：wiki 11 + faq 2，:107-198）、`docs/architecture/passb/exception-ledger.yaml`（exc-0106..0118，RemoveAt=ib2，计数 105→118 机械修正） | ✓ Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY |
| 装配提前落地（裁定授权） | `internal/container/container.go`：**仅** 1 provider 行（:316 `knowledgeWiki.NewWikiPageRepository`）+ 1 import 行（:93）+ 4 行注释 | ✓ Ruling 2026-09-25-CYCLE-FORCED-COMPOSITION；Brief (a) A1 同款切换排期；提交/报告/Brief 三处留痕 |
| 镜像文档机械同步 | `internal/modules/knowledge/legacy/README.md`：K3.1 删 wiki 12 行（b981d13e3）；**K3.3 补删 faq 6 行**（K3.2 机械遗漏发现后就地补齐，conventions §10 共同原则） | ✓ 属 `internal/modules/knowledge/**` 写权面；与 manifest 终态对齐 |
| manifest | `docs/architecture/moves/knowledge.yaml`：18 legacy 行删（与物理迁移同 commit，Ruling LEGACY-ROW-OWNERSHIP）+ 7 过渡行增 | ✓ |
| 节点产物 | `docs/architecture/evidence/passb/b2-k-wikifaq.md`、`docs/plans/passb/reports/b2-k-wikifaq.md`（本文件）、`docs/architecture/passb/briefs/b2-k-wikifaq.md`（K3.3 新增） | ✓ §1.1 |

**差集结论**：与 §3.2 求差集为空——超出计划 §6.2 字面 6 文件清单的每一项（W3、4 个测试文件、check.go、exception-ledger、container.go、legacy/README）均有计划条款或裁定族授权（出处见上表各行）。禁改面零触碰：`git diff --name-only` 不含 `rbac_lookups.go`、`qa.go`、`knowledge.go`、`routes_knowledge.go`、`router/task.go`/`sync_task.go`、`go.mod`/`go.sum`、`internal/types/**`、`kbfreeze/**`、`module.go`、ownership-matrix/contracts/event-catalog（验收 #5/#6/#9）。

## 3. 18 文件零残留核验（验收 #3）

对 §3.1 表 18 条 legacy 路径逐条 `git ls-files --error-unmatch` → **全部无 RESIDUE（零残留，2026-09-25 K3.3 实跑）**。注意：验收 #3 的字面 glob 命令（`wiki_*.go`/`knowledge_faq*.go`）会额外命中 7 个过渡文件名（`wiki_k3_compat.go`、`knowledge_faq_k3_delegate.go` 等——本节点产出的兼容层，非 legacy 残留），故以精确路径枚举为准；destination 两侧 = wiki 12+2 新增（seams.go/seams_test.go）、faq 6+2 新增（service.go/service_test.go）。

## 4. K3.3 交付物

1. **Integration Brief** `docs/architecture/passb/briefs/b2-k-wikifaq.md`：覆盖计划 Task K3.3 要求的 (a) 装配切换表（A1–A11，container.go:316/436/437/438/725/864、routes_knowledge.go:143/:309、recover_pending_wiki_tasks.go:75、router/task.go:124、qa.go:205 逐行）；(b) 宿主调用点改写全清单（W1 16 行 + D1 faq 面 + W3 面，全部 file:line 实测）；(c) 7 兼容文件删除批 + rbac_lookups.go:105/:183 收口说明；(d) faq/wiki seam 生产接线表（seam→宿主符号→K2 `retrieval/app`/K4 `process` 落位目标）；(e) 就地重命名清单（17 行，含 `EnqueueWikiRetractWithError` 计划外后缀差异登记）；另加删除义务 2 结论与 6 项 ib2 登记。
2. **删除义务 2 核对**（Brief 专节）：`grep -rn 'IsImageFormat\|IsSimpleFormat\|SimpleFormatReader' internal/modules/knowledge/wiki/ internal/modules/knowledge/faq/` → 退出码 1（零命中），无回收动作。
3. **legacy/README.md faq 6 行补删**（本 commit）：K3.2 遗漏的镜像行机械缺口。

## 5. 计划偏差与实测修订登记（上报协调者随 K3.4 回填 DAG notes）

1. **§5.1 组 A 实测扩面**：W1 实际转发面比计划 §6.2 W1 行多 8 个符号/常量族（WikiRetractPayload、WikiPendingOp、WikiDeletedTombstoneKey、wikiDeletedTTL、wikiTaskType/wikiTaskScope/WikiOpIngest/WikiOpRetract）；`enqueueWikiRetract` 导出名为 `EnqueueWikiRetractWithError`（非计划的 `EnqueueWikiRetract`）。
2. **§5.2 组 B 实测缩减**：`hash`（调用点为同名局部变量/注释）、`contains`（wiki_ingest.go:2169 假阳性）零真实调用，未建 seam（evidence §1.6-5、§2.4-3）。
3. **§6.2 清单外新增 W3**（repository/wiki_k3_repo_compat.go）：escapeLikePattern 拆出物理留驻 + 5 哨兵留驻裁定（import 环根因，文件头登记）。
4. **§4 表补录**：接口 FAQ 面第 15 方法 `UpdateLastFAQImportResultDisplayStatus`（编译器枚举，K3.2 Step 4 预定机制）。
5. **计划 §10.4 例外二选一**：wiki 侧选 seam 注入路径（`IsKnowledgeBaseNotFound`），未新增 wiki→repository import 例外；faq 侧 knowledge_faq_import.go 2 条例外按 Ruling 登记（exc-0117/0118）。
6. **接口 FAQ 面 15 方法清单**：见 evidence §2.6(a)。

## 6. 未完成项（如实）

1. **K3.4 已执行完毕**（本 commit）：evidence §3 终稿（四行差分终表 + hook 恢复手工差分双跑 + T0/T1/T2 对照 + 门禁终跑）落库；门禁四项 + §1.2 命令包在最终 HEAD 工作树实跑全绿（§1 表）；计数奇偶三方一致复核（§1 尾段）。**向协调者回报（本节点不改 DAG，conventions §9）**：建议 DAG `b2-k-wikifaq` 置 `review`、`task_ids=[K3.1,K3.2,K3.3,K3.4]`；随报 §5 六条计划偏差/实测修订（组 A 扩面 8 符号/常量族 + `EnqueueWikiRetractWithError` 命名差异、组 B `hash`/`contains` 实测零调用未建 seam、W3 增量、接口 FAQ 面第 15 方法补录、§10.4 二选一实际选择、faq 2 条 import 例外 exc-0117/0118）供协调者回填 DAG notes。
2. **DAG 字段**：`base_sha`/`status`/`task_ids` 未回填——协调者独占（conventions §9）。
3. **ib2 残差**：7 兼容文件、13 条 import 例外、3+1 测试垫片/伴生、双副本 2 处、container.go:316 转核验——全部登记于 Brief「其它 ib2 登记」节，删除点 ib2（最迟 B5）。
4. 本节点不改 DAG/治理三件套（ownership-matrix 18 行删除归 ib2 delete_barrier 收口）。

## 7. 高风险差分证据指针（conventions §6）

`docs/architecture/evidence/passb/b2-k-wikifaq.md` 终稿：**§3.1 四行差分终表**（① TypeWikiIngest/TypeWikiFinalize 状态机 132/132、② TypeFAQImport 8+15、③ recoverPendingWikiTasks hook 恢复双跑逐字节一致、④ Wiki 页面/文件夹操作）；§3.2 hook 恢复逐项等价论证（recover 文件零改动 + W1 真类型别名 + 结构体逐字节一致）；§3.3 T0 基线 vs T1/T2 终态对照；§3.4 门禁终跑与计数奇偶；细节展开 §1.2/§1.3/§1.4/§2.3/§2.5/§2.8。
