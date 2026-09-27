# b2-datasource 节点报告（Pass B / 26-datasource）

> 节点：`b2-datasource`（B2 Data Source：4 legacy 文件迁入模块 + kbActivity 24 调用点接线 + cleanup seam + 别名/例外收口 + Brief/报告/门禁收口）。计划：`docs/plans/passb/26-datasource.md`（任务 B2-DS.1 → B2-DS.8）。
> 分支：`codex/passb-b2-datasource`（worktree `.worktrees/passb-b2-datasource`）。
> **ALIGN_SHA 来源**：`486d46b424b95d49903e3e9aaf23f1461ce5d863` = P-2 Case A 基线对齐 merge commit（B2-DS.1 于 2026-09-27 执行 `git merge --no-ff codex/passb-b2-k-integration`（@461d8c4b2）产生，Ruling 2026-09-24-WAVE-DEP-BASELINE；证据 evidence §前置核验 P-2 节）——本节点全部 diff 检查的 `PASSB_BASE_SHA` 采用值（计划 §0.1 P-2 裁定，禁用 merge-base origin/main 缺省公式）。
> 各任务实施级报告（含逐任务命令台账）：主 checkout 会话区 `.superpowers/sdd/passb/b2-datasource/B2-DS.{1..8}-report.md`；节点级证据：`docs/architecture/evidence/passb/b2-datasource.md`（§前置核验/§特征化基线/§覆盖面核对/§差分/§计数基线/§别名）。

## 1. 执行命令台账（B2-DS.8 收口时点全量实跑，2026-09-27，worktree 根）

DAG gates 四条命令原样执行（conventions §2，不得以更快等价检查替代）：

| 命令（原文） | 退出码 | 关键输出摘要 |
|---|---|---|
| `go build ./...` | **0** | 仅 cmd/desktop、cmd/server `ld: warning: ignoring duplicate libraries: '-lc++'` 链接噪音（BASE 固有，P-6 基线快照同款） |
| `go test -count=1 ./internal/modules/datasource/...` | **0** | **15 包全 `ok`、0 FAIL/SKIP**：datasource 根 6.114s + connector/{confluence,dingtalk,feishu/core,feishu/drive,feishu/wiki,gitlab,ima,moauth,notion,rss,yuque} + handler 1.255s + repository 1.206s + service 1.617s |
| `make check-backend-architecture` | **0** | `literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`；`architectureguard: OK (0 violations)` |
| `make verify-module-moves` | **0** | `modulemove: OK (16 manifests verified)` |

conventions §1.2 报告包命令（`PASSB_BASE_SHA=486d46b424b95d49903e3e9aaf23f1461ce5d863`，B2-DS.8 commit 前树实测）：

| 命令（原文） | 退出码 | 关键输出摘要 |
|---|---|---|
| `git diff --stat "$PASSB_BASE_SHA"...HEAD` | 0 | `34 files changed, 1136 insertions(+), 218 deletions(-)`（B2-DS.8 commit 后 38 文件：+Brief/报告/README×2） |
| `git diff "$PASSB_BASE_SHA"...HEAD --name-only \| sort` | 0 | 34 文件（清单见 §2）；与 §3 写入所有权求差集 = **空**（§2 逐条核对，含 4 项已登记授权偏差） |

前序任务（B2-DS.1~DS.7）逐任务命令台账：evidence 各节（§前置核验 P-1~P-6、§差分 B2-DS.3/B2-DS.7、§计数基线 B2-DS.5、§别名 B2-DS.6）+ 会话区报告，此处不重复。

## 2. 变更清单 vs owned_files（计划 §3 写入所有权）逐条核对

`git diff "$ALIGN_SHA"...HEAD --name-only | sort`（34 文件 + B2-DS.8 新增 4）分组核对：

| 组 | 文件 | 计划依据（§3/§1） | 判定 |
|---|---|---|---|
| legacy 迁移 4 | `internal/modules/datasource/repository/datasource_repo.go`、`service/datasource_service.go`、`handler/datasource.go`、`handler/datasource_credentials.go` | manifest legacy_files 4 条（B2-DS.2/3/4 git mv，rename 识别） | ✓ |
| 随迁测试 16 | repository 2（`datasource_repo_test.go`/`datasource_repo_synclog_test.go`）+ service 10（cancel_enqueue/credential_refresh/credential_refresh_trigger/reindex/result_cap/service_test/stream/sweep_wiring/sync_cancel/sync_heartbeat）+ handler 4（test/documents_count/reindex/credentials） | framework:29 随迁义务（计划 §1 随迁 17 文件中的 16——delete_sqlite 见偏差组） | ✓ |
| 留守重写 1 | `internal/application/service/datasource_purge_test.go` | §3 树图「保留 + 重写构造点（B2-DS.3）」 | ✓ |
| 宿主过渡 shim 3 | `internal/application/repository/datasource_passb_compat.go`、`internal/application/service/datasource_passb_compat.go`、`internal/handler/datasource_passb_compat.go` | §1 节点产出（Ruling TRANSITION-SHIM-ROW-REGISTRATION 成对补行） | ✓ |
| 测试装置垫片 3 | 宿主 `internal/application/service/datasource_shim_test.go`（K 属主留守测试消费随迁符号）+ 模块 `service/datasource_kbdelete_shim_test.go`（随迁文件共享 K 属主 fixture）+ 模块 `handler/datasource_handler_shim_test.go`（随迁文件消费宿主 handler 共享 helper `errorCapture`） | Ruling 2026-09-24-TEST-SUPPORT-SHIM（唯一 `_test.go` 垫片、台账追踪、remove_at=ib2）——B2-DS.3/B2-DS.4 报告与 commit 登记 | ✓（授权偏差，见 §3.2） |
| 治理文件 5 | `docs/architecture/moves/datasource.yaml`（legacy 4 删+shim 3 增+alias 12 删+move 三区核销）、`docs/architecture/passb/ownership-matrix.yaml`（镜像成对）、`docs/architecture/passb/exception-ledger.yaml`（+3 行 exc-0132..0134）、`tools/architectureguard/check.go`（importExceptions 数据行 +3）、`docs/architecture/evidence/passb/b2-datasource.md` | §1 节点产出 + Ruling LEGACY-ROW-OWNERSHIP/IMPORT-EXCEPTION-REGISTRY/TRANSITION-SHIM-ROW-REGISTRATION | ✓ |
| **偏差授权 1** | `internal/application/service/datasource_delete_sqlite_test.go`（留守宿主+构造点重写） | **不在 §3 字面清单**——B2-DS.3 实测偏差（fixture 依赖 `knowledgeBaseService` K4 白盒不可迁），按 Ruling 2026-09-25-DEFERRED-FILE-SPLIT 收缩+登记（§3.2；差分经宿主补跑 3/3 PASS 锚定） | ✓（登记在案） |
| **偏差授权 2** | `docs/plans/passb/26-datasource.md` | R3 修订 §3 补登（限 docs-only 计划修订轮 commit；81f53c33a） | ✓（登记在案） |
| 本任务产出 4 | `docs/architecture/passb/briefs/b2-datasource.md`（新增）、`docs/plans/passb/reports/b2-datasource.md`（本文件）、`internal/modules/datasource/README.md`、`internal/modules/datasource/legacy/README.md`（迁移状态回填） | §1 节点产出（Brief/报告；README 回填为 §1 列名产出，前序任务未覆盖、B2-DS.8 补齐，见 §3.9）；README 属 `internal/modules/datasource/**` | ✓ |

**差集结论**：34+4 文件全部落入 owned_files 字面集或已登记授权偏差集，**严格差集为空**。

## 3. 事实偏差登记（conventions §5 如实登记，均非阻塞）

1. **§0.2.2 K4 推迟致 seam 方案（计划指定登记项）**：DAG notes 预期「依赖 b2-k-integration 同时覆盖 K2 与 K4」；实测 K4 按 Ruling DEFERRED-FILE-SPLIT 把 `knowledge_delete_plan.go` 列入推迟批（24 计划 §3.3 批 2-12），对齐后 `withKnowledgeCleanup` 仍未导出、留守宿主（P-3 判据实测）。本节点对 5 符号采用分裂处置：4 个 K2 已导出符号（`RecordKBActivity`×17/`WithKBActivityTask`×2/`KBActivityTrigger`×1/`WithKBActivitySuppressed`×3）直连改写；1 个 K4 推迟符号走消费侧 seam（模块字段+`SetKnowledgeCleanup` :119-121、调用点 :898-899）+ 宿主 compat 构造包装器接线（与迁移前同目标函数，行为零变化）。seam 对「补迁窗已导出」与「未导出」两种时点均行为等价；终局编排见 Brief (b)。
2. **B2-DS.3 测试随迁分裂**：计划「service 侧 11 文件 76 用例随迁」，实测 kbDelete* helper 族（K 属主 `knowledgebase_delete_datasource_test.go`/`knowledgebase_pr3_test.go`）被 5 文件类型级嵌入且 cancel_enqueue↔sync_cancel fixture 互引——实际随迁 10 文件 73 用例 + 模块包 `kbDeleteDSRepo` 垫片（Ruling 3）+ delete_sqlite 3 用例留守（K4 白盒不可迁）+ purge 9 留守（计划既定）。奇偶：73+3+9=85 与 T1 基线 service 85 一致；差分双跑零偏差（evidence §差分 B2-DS.7）。
3. **行号漂移**：计划 R2 时点（afe266402，P-2 对齐前）container.go 行号 :309/:310/:633/:760/:857/:2368 对齐后实测漂移为 :310/:311/:638/:765/:862/:2373（+1~+6，K 谱系合入所致）；purge_test bindingRows 断言 :354-:355→:349-:350 区段（evidence §前置核验「对齐后行号复核」+ §差分）。Brief/本报告均按 HEAD 实测行号引用。
4. **P-1 review_status 偏差**：计划 §0.1 P-1 期望 `b2-k-integration` review_status=approved，调度事实源实测 `changes_requested`（status=done、head_sha=461d8c4b2 已回填；调度方已将本节点置 in_progress=恢复派发行动裁定）。按「以开工时点 DAG 实测为准」口径继续开工，evidence §前置核验登记。
5. **B2-DS.1 任务级 OCR 两跑不完整（R3 replan）**：429 限流使两跑不完整；重试选区裁定=限定 B2-DS.1 净改动 1 文件（evidence），排除 P-2 对齐带入的 161 个 K 谱系文件（K5.1 `379c0613d` 同型）。两跑 19 条 findings 全落 K 谱系文件（本节点 owned_files 之外），两轨处置：K 属主债务移交；其中治理 YAML 两条 high 经实测属实（check-passb-readiness 当前树 exit 1，K 属主继承债务）移交协调者（81f53c33a）。
6. **passbguard `PassBTask "B2-DS.5"` 映射缺失（IB2 债务）**：`tools/passbguard/check.go:83-93` PassBTaskModule 仅含 B0 建制 9 个模块级 id；B2-DS.5 数据行按计划 T5 Step 1 原文使用 `PassBTask: "B2-DS.5"` 触发 `exception-task-module` 诊断 +1（去重 1 条）。本节点无权改 passbguard（guard 逻辑禁改、映射数据行 owner=barrier）；Brief (c) 移交 IB2。当前树 `go test ./tools/passbguard/...` 的 `TestRealRepoExceptionLedgerPlansMatchGuardTasks` 因之 FAIL（BASE 预存实证：90b93f321 树同 FAIL，B2-DS.6 净改动仅 manifest/matrix 数据行与该断言无交集）。
7. **check-passb-readiness exit 1（K 谱系继承债务，非本节点 gate）**：B2-DS.5 实测 passbguard 诊断 259→248（−12 治愈：12 条 contract-consumer-module-import 全指向 datasource_service.go，被 3 例外吸收；+1 见上条；净 −11）。B2-DS.1 已实测属 K 谱系属主债务并移交协调者（§3.5）。
8. **P-2 执行事故（已完整还原）**：P-2 首次 merge 因 cwd 判断失误在主 checkout main 分支执行（错误 commit 871fd0d66，未推送）；已按「备份用户未提交修改→`git reset --hard ae30162c5`→恢复用户修改」完整还原（主 checkout status 与会话起始快照一致），错误 commit 成悬空对象；worktree 全程未受影响。后续命令全部改用 `git -C`/绝对路径。详见 evidence §P-2 执行事故登记。
9. **README 回填时点**：计划 §1 将 `internal/modules/datasource/README.md`/`legacy/README.md` 迁移状态回填列为节点产出，但 T2-T7 各任务步骤未显式覆盖、前序任务未执行；B2-DS.8 补齐（K 节点同型格式：legacy/README 迁移轨迹行标 **已迁移**、compat 行补登、主 README 计数与状态刷新）。

## 4. 高风险差分结论（指针）

**115 用例零偏差**（conventions §6 四要素齐备）：T1 基线（ALIGN_SHA 树 85+10+20 全 PASS）vs T7 终态（HEAD=52ae889a2 树 73+9+3+10+20 全 PASS），程序化 diff 五组全空、0 FAIL/0 SKIP。高风险两面专门登记：knowledge 删除/purge 级联 9 用例（绑定清理断言同时证明 seam 接线与迁移前直引等价）、Worker 状态机/取消/重试/幂等 21 用例（cancel_enqueue 6+sync_cancel 7+heartbeat 2+stream 6）、审计活动流（17 直连改写点行为面）。详见 evidence §差分 B2-DS.7（legacy 删除前差分已通过：T1 在前、T7 在全部删除后、T3/T4 包级复跑为每删除 commit 即时门）。

## 5. 计数奇偶三方一致（conventions §8 / F5 口径）

`make check-backend-architecture`：633（564+69）/23+23/58/16 与 `pass-a-acceptance.md` 台账、发现值三方一致；migrations `find|wc -l`=537。B2-DS.1 至 B2-DS.8 全程计数零漂移；本节点零路由/worker/hook/migration 增删（`git diff "$ALIGN_SHA"...HEAD --name-only | grep -E "internal/router/|migrations/"`=0）。例外行基线 131→134（B2-DS.5 +3，remove_at=ib2，双侧一致五诊断 0）；worker/route/hook 奇偶 2/1/2 与 manifest integration_points 一致（Brief (f)）。详见 evidence §计数基线。

## 6. 节点 commit 台账（ALIGN_SHA..HEAD）

| commit | 任务 | 摘要 |
|---|---|---|
| `020c05168` | B2-DS.1 | 前置核验（P-1~P-6）+ 特征化基线（115 用例）+ 覆盖面核对 |
| `81f53c33a` | R3 修订 | B2-DS.1 OCR 根因分析 replan（docs-only 计划修订轮） |
| `b6d705dbd` | B2-DS.2 | repository 迁入模块 + 宿主 compat + 成对行级收口 |
| `e6c3baa61` | B2-DS.3 | service 迁入模块（24 调用点改写 + cleanup seam）+ compat + purge_test 重写 + Ruling 3 垫片 ×2 |
| `75e94da99` | B2-DS.4 | handler 2 文件迁入模块 + 宿主 compat + handler 垫片 |
| `90b93f321` | B2-DS.5 | 3 条 importExceptions 数据行 + ledger 3 行 + 计数基线登记 |
| `52ae889a2` | B2-DS.6 | 12 条别名义务行成对删除（manifest+matrix 同 commit） |
| `a43505d1b` | B2-DS.7 | 差分复跑比对（115 零偏差）+ evidence 定稿 |
| （本 commit） | B2-DS.8 | Brief (a)-(f) + 本报告 + README 回填 + 节点门禁收口 |

## 7. 未完成项 / 遗留（如实；全部为 ib2/K4 补迁窗编排项，Brief 对应节登记）

1. 3 个宿主 compat 文件 + manifest/matrix 3 shim 行——ib2 装配直连切换批删除（Brief (a)；删除前置=留守测试消费者同窗处置）。
2. cleanup seam 终局（模块字段+setter+compat 接线的拆除）——K4 补迁窗导出 `withKnowledgeCleanup` 后直连改写，或 IB2 先切换时 container 侧等价闭包接线（Brief (b) 三时点分支）。
3. 3 条例外行（exc-0132..0134）——ib2 随 knowledge/appconnector/policy 门面合法化或 ADR 修订收口（Brief (c)）。
4. passbguard `PassBTaskModule` 增 `"B2-DS.5": "datasource"` 数据行（或裁定改用模块级 id）——IB2 债务（Brief (c)；§3.6）。
5. 留守测试 3 文件终局：`datasource_purge_test.go`（9）+ `datasource_delete_sqlite_test.go`（3）随 K4 补迁窗迁入模块重建（K4 Brief (c)/(f) 义务「datasource_purge_test.go→26-datasource/ib2」，Brief (e) 复述）；Ruling 3 垫片 3 件 ib2 先到先删最迟 B5。
6. 门面五操作实装（module.go）——IB2 时点按 K5.1 同法实装（Brief (d)；本节点零触碰）。
7. check-passb-readiness 全绿——K 谱系属主债务（B2-DS.1/B2-DS.5 移交协调者，§3.5/§3.7），非本节点 gate。
8. manifest integration_points 与 contracts.yaml datasource.lifecycle 的 container.go 行号引用陈旧（:636/:637/:2366/:2373 → 实测 :641/:642/:2372/:2379）——barrier 回写时机械修正（Brief (d) 登记修正节；contracts.yaml 本节点禁改）。

## 8. 节点结论

- 四 DAG gates 绿（§1，命令+退出码原样登记）；
- §1.2 差集核对：严格差集为空（§2，34+4 文件逐条对照）；
- 独立验收标准 §9 十条逐项满足（1 四 gates/2 差集/3 物理落位+rename/4 24 调用点对账+purge 9 PASS/5 115 零偏差/6 manifest-matrix 奇偶/7 例外双侧一致+guard 绿/8 宿主零业务残留/9 Brief (a)-(f) 齐备且 (b) 与 K4 Brief (c)/(f) 义务对账/10 禁改清单零触碰——check.go 仅数据行）；
- **建议 DAG `b2-datasource` 置 `review`**（conventions §9：回填归协调者）；审查者产出 `docs/plans/passb/reviews/b2-datasource.md`。
