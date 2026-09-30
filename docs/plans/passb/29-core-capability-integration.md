# 29 — IB2 核心能力集成 barrier（K 序 + 25a→25b→25c 序 + datasource/appconnector）

> **角色**：集成屏障（barrier），由 Pass B 总集成工程师独占执行。
> **上游框架**：`docs/plans/2026-09-23-backend-modularization-pass-b-framework.md` §IB2（framework:148-153）。
> **写盘依据**：本计划文件在屏障开工时不存在，按任务指示从框架对应屏障节展开写盘；分支 SHA 与依赖关系为集成工程师 2026-09-27 本会话 `git merge-base --is-ancestor` / `git rev-list --count` 实测。

## 1. 目标与范围

将 B2 面四条已收口（done）子分支按依赖序逐支并入 `codex/passb-integration`，完成模块装配切换（router/container/全局注册由集成工程师独占），验证 Knowledge 18 workers Redis/Lite 双栈、catalog version/install 不变式、datasource/appconnector sync/action 行为，并删除且仅删除 IB2 属主的 import 例外。

**不在范围**：B3+ 节点派发、非 IB2 属主例外清理、main 分支操作、远程推送。

## 2. 集成序与分支基线（本会话实测）

| 序 | 分支 | head（实测） | 内容 | 依赖序依据 |
|---|---|---|---|---|
| 1 | `codex/passb-b2-k-integration` | `461d8c4b2` | K5 集成；**已包含 k0/k-ingest/k-retrieval/k-wikifaq/k-process 全部祖先**（本会话 `merge-base --is-ancestor` 逐一核验） | K 序：K1–K4 已汇聚于 K5 分支 |
| 2 | `codex/passb-b2-ac-market` | `8e0ce1a67` | 25c；**已包含 25a（ac-definition `938087598`）与 25b（ac-skills `5ed64d324`）** | 25a→25b→25c：25c 消费 25a/25b 的 release/install 门面（framework:139） |
| 3 | `codex/passb-b2-datasource` | `4ebe14cf5` | 26-datasource；B2-DS.1 已做 P-2 基线对齐 merge k-integration@461d8c4b2（Ruling WAVE-DEP-BASELINE，commit 486d46b42） | 依赖 b2-k-integration（DAG CORR-2：datasource→knowledge 5 符号 24 调用点） |
| 4 | `codex/passb-b2-appconnector` | `8e80bb3c6`（DAG 登记 head `6e8c84860`，其后有 2 个文档提交，以分支实际 HEAD 为准合并） | 27-appconnector | B1 已冻结其外部能力依赖（framework:146） |

开工基线：integration HEAD `326d548cb`（管家台账遗留落盘后）。开工前 DAG（工作树版）四入边节点均 done；`b2-k-integration.review_status=changes_requested` 系台账 7548 行留痕的登记纪律（修复 R1 已闭合、复审 0/0、节点级 OCR 0/0 均在案），节点以「门禁+OCR」口径 done，台账 8396 行确认 barrier 派发合法——非实质审查未通过。

## 3. 每支合并的动作

1. `git merge --no-ff` 单支合入；冲突按**上游冻结契约（B0 contracts/exception ledger）+ 子计划裁定**解决并逐条记录。
2. **装配切换核验**（集成工程师独占）：router/container/bootstrap 全局注册、go.mod/go.sum、migration 编号。已知提前落地项：k-integration 与 datasource 对 `internal/container/container.go` 的同一份改动（K3.1 Ruling 2026-09-25-CYCLE-FORCED-COMPOSITION：`repository.NewWikiPageRepository` → `knowledgeWiki.NewWikiPageRepository` provider 切换，注释自证"为 IB2 排期的同款切换提前落地，IB2 转核验项"）——IB2 核验采纳。
3. 合并后运行**该模块与直接消费者测试**：
   - k-integration：`go test ./internal/modules/knowledge/...` + 直接消费者（届时已在 integration 的依赖 knowledge 符号的包）；
   - ac-market：`go test ./internal/modules/agentcatalog/...`（或分支实际模块路径）+ 消费者；
   - datasource：`go test ./internal/modules/datasource/...` + 消费者；
   - appconnector：`go test ./internal/modules/appconnector/...` + 消费者。
4. `go build ./...` 保持可编译。

## 4. 终局门禁（全部合并后）

- `make -C .worktrees/passb-int check-backend-architecture`
- `make -C .worktrees/passb-int verify-module-moves`
- （屏障 DAG gates 另含 `go test ./internal/... -count=1 -timeout=25m`、`make check-passb-readiness`、changed-range lint——本屏障执行时按任务点名两项为硬门禁，其余尽力执行并如实报告）

## 5. 例外台账（exception-ledger.yaml）

- 开工时 integration HEAD（B0 版）`remove_at: ib2` 共 8 条：exc-0058..0061（appconnector→commercial ×4）、exc-0088（knowledge docparser→airesource）、exc-0089..0091（knowledge retriever→airesource ×3）。
- 合并后以子分支演进版 ledger 为基，**只删 IB2 属主例外**，且仅当对应 import 已实际消除（guard 通过为准）；未消解的 IB2 属主例外如实登记移交，不推给 b5 之外的属主。
- 撞号处理：exc-id 跨并行分支独立续号，集成侧如撞号按 (from,to) 边键重排（B0 台账头注）。

## 6. 产出与台账

- `docs/architecture/passb/briefs/ib2.md`（若入边分支已提供则核验）
- `docs/architecture/evidence/passb/ib2.md`（合并记录、冲突裁定、测试与门禁证据）
- `docs/architecture/passb/execution-ledger.md` 追加 IB2 屏障条目 + `docs/plans/passb/execution-dag.json` ib2 节点回填（head_sha），台账落盘提交。

## 7. 纪律

- 一次只合并一个分支；不合并到 main；不推送远程；不触碰其他任务 worktree/分支。
- 指令矛盾或合并将破坏冻结契约时，升级询问而非硬合并。
- 高风险行为（worker 双栈、catalog 不变式、sync/action）以子分支差分证据 + 集成侧重跑为准。
