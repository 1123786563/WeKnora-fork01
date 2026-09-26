# Pass B B2 — Knowledge Mini-program 协调计划（K0 冻结 + K5 集成）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**节点与文件属主：** 本文件是两个 DAG 节点的共同 `plan_path`（`docs/plans/passb/execution-dag.json`）：
- `b2-k0`（K0 冻结，§5–§7、§8 Task K0.1–K0.3、§13 历史记录的属主；**已 done+approved**，head=`5bcb798621b856f7ff6497986c7ca79dba8c338e`）；
- `b2-k-integration`（K5 集成，§8 Task K5.1–K5.3 及本修订新增段落的属主；DAG `depends_on=[b2-k-process, b2-k-wikifaq]`，`task_ids=[K5.1,K5.2,K5.3]`，gates=`go build ./...` / `go test ./internal/modules/knowledge/...` / `make check-backend-architecture` / `make check-passb-readiness` / `make verify-module-moves`）。

**修订记录：** 2026-09-23 b2-k0 初版（K0.1–K0.3 + K5 概要任务）→ 2026-09-23/24 审校与根因分析轮 → 2026-09-26 b2-k-integration 修订轮（K1–K4 已交付真实 seam，§8 K5.1–K5.3 由概要展开为零上下文可执行详案；K0 属主段落原样保留）→ 2026-09-26 R1 审校修订轮（五条 findings 逐条实测处置——P-K5-7 readiness 门禁预裁定（基线 218 条诊断实测）、PASSB_BASE_SHA=ALIGN_SHA 裁定（210 文件陷阱）、P-K5-2 双路径判据（merge-tree 预演）、骨架示意签名差异处置、module.go 包注释按 passbguard facade 形态契约重写（module_test.go 原稿复核合法未改；均经干净副本模拟实测）；详见 §13 R1 条目）→ **2026-09-26 K5.1 任务级 OCR 根因分析 replan 轮（本版）：OCR 两跑不完整（限流 429）根因登记 + 重试选区裁定（净改动 4 文件）+ Dependencies「其余 5 组」计数勘误溯源（详见 §13 对应条目）。**

**Goal:** 冻结知识子程序（K0–K5）的共享类型归属与跨 plan 未导出符号联动裁定（K0，已完成）；使 K1/K2/K3 并行、K4 串行完成 84 个 legacy 文件的搬迁与差分（K1–K4，已完成/在收口）；最后 K5 完成**模块门面五操作实装、装配切换 Integration Brief、18 条别名删除、例外/shim 收口核对与差分汇总门禁**。

**Architecture:** B2 知识子程序为五节点串并混合（framework:115-132）：K0 SERIAL 冻结 → K1(9)|K2(29)|K3(18) PARALLEL → K4(28) SERIAL（K1+K2 之后）→ K5 SERIAL 汇聚。K5 是 K1–K4 全量汇聚（CORR-1：`depends_on` 由 `[b2-k-process]` 修正为 `[b2-k-process, b2-k-wikifaq]`，漏 wikifaq 会丢 18 文件、`wiki_fixer_scope.go` 与对应别名）。

**Tech Stack:** Go 1.26、Gin、GORM、dig、asynq、YAML v3、Testify、Git worktrees。

**Spec:** `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md`（§4.2 依赖方向、§4.3 模块装配、§5.2 Knowledge 所有权、§5.18 覆盖矩阵、§11 Pass B 串并图、§13 提交隔离与回滚、§14.3 高风险差分、§17.2 完成标准）。

---

## 1. 事实源指针（全部只读输入）

| 事实源 | 用途 |
|---|---|
| `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` | 行为边界、装配契约（§4.3 五操作「具体 Go 签名由实施计划根据现有 RBAC、router、worker 和 ResourceCleaner seam 确定」）、完成标准 |
| `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md` | 程序分解与全局约束（:25-33、:109-153、:117-124、:132 并行前提）；行号以集成分支副本为准 |
| `.superpowers/sdd/passb/conventions.md`（主 checkout，git-ignored） | 派发契约、门禁、升级、差分、计数基线、裁定族（§1–§10） |
| `docs/plans/passb/execution-dag.json`（集成 worktree） | b2-k0/b2-k-* 六节点 owned_files、gates、required_contracts、notes、`package_private_couplings` |
| `docs/architecture/moves/knowledge.yaml` | 84 legacy 文件、18 alias_obligations、integration_points（routes 11 / workers 18 / lifecycle_hooks 1） |
| `docs/architecture/passb/knowledge-ingest.md` / `knowledge-retrieval.md` / `knowledge-wikifaq.md` / `knowledge-process.md` | 四域 brief（集成 worktree 版含 B0.2/B0.3 消歧注记） |
| B0 冻结产物（`.worktrees/passb-int/docs/architecture/passb/`）：`ownership-matrix.yaml`（aliases 区 :2408 起，知识 18 行 `plan: 20-knowledge-program, delete_barrier: ib2`）、`contracts.yaml`（knowledge.facade:1613 / knowledge.routes:1750 / knowledge.workers:1893，六 capability-port `stability: frozen`）、`event-catalog.yaml`、`exception-ledger.yaml`、`b0-evidence.md` | 文件属主/落位/删除屏障、契约签名、事件 v1、例外 removal owner |
| **K1–K4 子计划**：`docs/plans/passb/21-knowledge-ingest.md`、`22-knowledge-retrieval.md`、`23-knowledge-wikifaq.md`、`24-knowledge-process.md` | 各域任务、耦合面裁定、推迟件登记（K5 汇总输入） |
| **K1–K4 Integration Brief**：`docs/architecture/passb/briefs/b2-k-ingest.md`、`b2-k-retrieval.md`、`b2-k-wikifaq.md`、`b2-k-process.md`（基线对齐后随 `codex/passb-b2-k-process` 分支可见） | 装配切换申请、shim/seam 清单、删除批——K5.1 Brief 的汇总事实源 |
| **K1–K4 证据/报告**：`docs/architecture/evidence/passb/b2-k-{ingest,retrieval,wikifaq,process}.md`、`docs/plans/passb/reports/b2-k-*.md` | 差分双跑记录（K5.3 复核对象） |
| 实测代码 | 本计划全部签名与调用点行号的出处。**K5 详案（§8 K5.1–K5.3）的实测基准 = 分支 `codex/passb-b2-k-process` HEAD（2026-09-26，含 K0–K3 合并 + K4 终态；K1/K2/K3/K0 均为其祖先，`git merge-base --is-ancestor` 四支实测通过）；行号以实施时点复核为准。R1 修订轮（同日）另将该分支 `git archive` 至 /tmp 干净副本，逐字落盘 §8 K5.1 两代码块实跑全部门禁（build/test/kbfreeze/双守卫/passbguard）——结论已写入 §8 K5.1 与 P-K5-7** |

注意：主 checkout 的 `docs/architecture/passb/knowledge-wikifaq.md`、`knowledge-process.md` 为 B0 前旧版；以集成 worktree 版为准。

---

## 2. 节点裁定与前置条件

**节点裁定（调度方原文，照录）：**

1. K1-K3 仅在 K0 分配共享 Chunk/KnowledgeBase/Tag/semantic 类型后方可并行（framework:132）。
2. 知识 84 legacy 文件 = K1 9 + K2 29 + K3 18 + K4 28（framework:120-121；与 `ownership-matrix.yaml` 逐行核对一致：21→9、22→29、23→18、24→28）。
3. CORR-1：`b2-k-integration.depends_on` 由 `[b2-k-process]` 改为 `[b2-k-process, b2-k-wikifaq]`——K5 是 K1-K4 全量汇聚，漏 wikifaq 会丢 18 文件、`wiki_fixer_scope.go` 与对应别名。
4. BLOCKED 历史链（2026-09-23 b0 → 2026-09-24 b2-k0 → 2026-09-25 b2-k-retrieval → 2026-09-25/26 b2-k-process×2）：各前置 done 后逐次恢复；本修订轮（2026-09-26）K5 计划撰写已被派发。

**K0 前置条件核对（K0 开工门禁，b2-k0 已执行完毕，原文保留）：**

- [x] **P1 b0 冻结产物在位**（2026-09-23 18:50 实测）。
- [x] **P2 b0 已登记未闭环修复工单**（ocr-r3-a2-f1、b0-ocr-r3-1；K1-K5 门禁不依赖其收口）。
- [x] **P3 ib1 已合并但为零实施**（`git show 8c45a8815 --stat` 实测；差异①据此登记）。
- [x] **P4 门禁工具可用**：`Makefile:250 verify-module-moves`、`:255 check-backend-architecture`、`:262 check-passb-readiness`（本修订轮于集成分支再复核，三目标在位）。
- [x] **P5 工作树干净**。

**K5 派发前置（2026-09-26 修订轮新增；实施者开工前逐条给出实证，任一不满足按 conventions §5 回 BLOCKED）：**

- [ ] **P-K5-1 前置节点终态**：DAG `b2-k-wikifaq` `status=done, review_status=approved`（head=`ed156cd856a20a89bfcb24c27c782757a731fe42`）；`b2-k-process` `status=done, review_status=approved` 且 `head_sha` 已回填。本计划撰写时点（集成 worktree DAG 快照）b2-k-process 仍为 `in_progress/review=pending`（其分支已含差分证据、Brief 与节点报告三件套，收口在途）——**实施者必须以开工时点 DAG 实测为准**。
- [ ] **P-K5-2 基线对齐 merge**（Ruling 2026-09-24-WAVE-DEP-BASELINE；R1 修订轮补双路径判据——Case A/B 互斥，实施者按开工时点实测拓扑走其一，K0/K1/K2/K3 已是 `codex/passb-b2-k-process` 祖先、无需逐支重并）。先判 `git merge-base --is-ancestor codex/passb-b2-k-process HEAD`：
  - **Case A（输出 NO——2026-09-26 实测现状：本分支自 b1a3d6dd8 谱系分出、k-process 非其祖先）**：`git merge --no-ff codex/passb-b2-k-process` **必然产生独立 merge commit**（"Already up to date" 不可能出现；若出现说明误判 Case，转 Case B 判据复核）。冲突面已预演（R1 `git merge-tree --write-tree --name-only HEAD codex/passb-b2-k-process` 实测）：**唯一冲突 = 本计划文件自身**（add/add：k-process 侧是 b2-k0 时代旧版，其末次改动 `90530cac2`）——解法 = `git checkout --ours docs/plans/passb/20-knowledge-program.md && git add docs/plans/passb/20-knowledge-program.md`（属主 b2-k-integration 的修订版为准）；`internal/modules/knowledge/module.go` 两侧同为 22 行骨架、无冲突（原「两侧注册全保留」指引在本拓扑无适用对象，保留为通用规则：module.go 若因他因冲突仍按两侧注册全保留处理）。
  - **Case B（输出 YES——重派/中断恢复，或协调者已把含 k-process 的内容并入本分支）**：merge 输出 "Already up to date" **不是前置失败**；判据改为指认既有引入点——`git log --merges --ancestry-path codex/passb-b2-k-process..HEAD --oneline` 最近一条即既有对齐 merge commit；无法指认 merge commit（squash/直推进入）→ 记录实际进入方式并按 conventions §5 上报。
  - 两 Case 共同收尾判据：`go build ./...` 绿、`go test -count=1 ./internal/modules/knowledge/kbfreeze/` 绿、`git status` 干净；涉冻结签名取舍的冲突停下升级。
  - **ALIGN_SHA 登记（K5 全部 diff 检查的基线，见 K5.3 Step 2 裁定）**：Case A = 新 merge commit 的 SHA；Case B = 指认的既有对齐 merge commit 的 SHA；`ALIGN_SHA` 及其来源写入报告。
- [ ] **P-K5-3 四份 Brief 与四份 evidence 在位**：对齐后 `ls docs/architecture/passb/briefs/b2-k-{ingest,retrieval,wikifaq,process}.md docs/architecture/evidence/passb/b2-k-{ingest,retrieval,wikifaq,process}.md` 全部存在（K5.1/K5.3 的汇总输入）。
- [ ] **P-K5-4 门面零实现复核**：`grep -cE "^func " internal/modules/knowledge/module.go` = 0（22 行注释骨架；K1–K4 四子计划禁改清单均列明未触碰，2026-09-26 于 k-process 分支实测 16 个模块门面全部零函数）。
- [ ] **P-K5-5 门禁工具可用**：`Makefile` 三目标（:250/:255/:262）在位；`go run ./tools/passbguard -root .` 可执行。
- [ ] **P-K5-6 工作树干净**：`codex/passb-b2-k-integration` worktree `git status` 干净。
- [ ] **P-K5-7 readiness 门禁基线快照与节点判据预裁定（R1 修订轮新增）**：DAG b2-k-integration gates 实读含 `make check-passb-readiness`（= `go run ./tools/passbguard -root .`，Makefile:262；B2 knowledge 波内 k0/k-ingest/k-retrieval/k-wikifaq/k-process 五节点 gates 均无此项，K5 是首个执行者），但该命令在实测基准上**必然非 0 退出**——基线对齐后立即执行并留档：

  ```bash
  go run ./tools/passbguard -root . >/dev/null 2>&1; echo "baseline_exit=$?"   # 退出码原样入报告
  go run ./tools/passbguard -root . 2>&1 | grep -v '^exit status' | sort > /tmp/k5-readiness-baseline.txt
  wc -l /tmp/k5-readiness-baseline.txt
  ```

  **预期退出码 1、218 条既有诊断**（R1 修订轮于 k-process HEAD 干净副本复跑实测：85 contract-consumer-unrecorded / 42 contract-consumer-file-missing / 33 legacy-undeclared / 33 legacy-nonexistent / 13 contract-characterization-missing / 10 legacy-missing / event-producer-missing、event-consumer-missing 各 1）。根因全部是 B2 各 wave 搬迁后 b0 冻结治理文件未随迁改写（contracts.yaml consumers/characterization 路径指旧路径、manifest legacy_files 行待推迟批收口等）——**修复面全部位于 K5 禁改清单**（contracts.yaml=barrier 回写、ownership/manifest 行=各属主行级删除权），K5 节点不可能使其归零，**归零属 ib2 契约区回写与推迟批收口**（其中 knowledge.\* 契约的 unrecorded 类基线已有 41 条、K1–K4 落位模块文件大量在列——同类形态先例）。若开工时点实测退出码 0，按绿处理（快照仍留档）。据此本节点对该 gate 的完成判据预裁定为：
  - **(i)** 命令原样执行、退出码与全量输出如实入报告（预期 1；非 0 不构成节点失败，如实记录不算伪造）；
  - **(ii)** 节点终态相对本快照的差集 = **恰好 3 条预登记新增**：`contract-consumer-unrecorded: knowledge.service / knowledge.knowledge-base-service / knowledge.tag-service: production consumer internal/modules/knowledge/module.go references KnowledgeService|KnowledgeBaseService|KnowledgeTagService but is not recorded`（门面 `Dependencies` 必然引用三个冻结端口名，而 consumers 登记在 contracts.yaml=K5 禁改；R1 修订轮已模拟实测该终态差集恰为此 3 行）；**此外任何新增诊断 = 节点失败**；
  - **(iii)** 消失项逐条归因于 K5 自身改动（K5.2 别名成对删行/注释修正不触及 passbguard 诊断面——基线无 alias 类诊断，预期消失集为空）；无法归因 = 按 conventions §5 上报；
  - **(iv)** 本预裁定写入节点报告并在 DAG `b2-k-integration` notes 登记上报，请协调者按 conventions §9 采纳或修正该节点 gates 释义（K5 不自行改 DAG）。

---

## 3. 程序结构与派发序

```text
K0 本文件（SERIAL，done+approved @5bcb7986）
  ↓ 分配表评审通过（§5/§6/§7）
K1 ingest(9) | K2 retrieval(29) | K3 wiki+faq(18)   [PARALLEL，各自 worktree/分支]
  ↓（K4 仅依赖 K1+K2；基线对齐时并入 K3）
K4 process/状态机(28) + 18 worker handler 实现       [SERIAL，done/approved 待收口登记]
  ↓
K5 knowledge 集成（门面五操作实装 + 装配 Brief + 18 别名删除 + 差分汇总门禁）[SERIAL]
  ↓
IB2（29-core-capability-integration，contracts.yaml knowledge 契约区状态回写在此）
```

- 子计划文件：`21-knowledge-ingest.md`、`22-knowledge-retrieval.md`、`23-knowledge-wikifaq.md`、`24-knowledge-process.md`（均已撰写并评审通过；义务清单见 §9）。
- K5 任务即本文件 §8 K5.1–K5.3；派发属主节点是 `b2-k-integration`。K5 **实施**的开工前置 = P-K5-1（DAG 终态 done+approved）+ P-K5-2（本节点分支基线对齐），均以开工时点实测为准；误派正确响应为 BLOCKED 上报（先例：2026-09-24 K5.1 于 b2-k0 误派，`docs/plans/passb/reports/b2-k0.md` §7/§8）。**时序互斥裁定（R1）**：集成分支的 K1→K2→K3→K4 合并（集成工程师执行，framework:28）与 K5 分支的 P-K5-2 基线对齐**互不等待、互不触碰**——K5 分支的对齐对象只有 `codex/passb-b2-k-process` 一个分支；集成分支合并先行或后行均不改变本分支与 k-process 的祖先关系（2026-09-26 实测 k-process 非本分支祖先），仅当协调者把含 k-process 的内容直接并入本分支才转入 P-K5-2 Case B。
- 集成分支合并顺序缺省 **K1 → K2 → K3 → K4**（一次一支、审后合并，framework:28）；K5 分支的基线对齐 merge 不等于集成分支合并（Ruling WAVE-DEP-BASELINE：性质=基线对齐，不碰集成分支、不豁免 barrier 职责）。

---

## 4. 写入所有权与禁改清单

**K0（属主 b2-k0，已完成）可写（历史记录，原文保留）：** 本文件；`internal/modules/knowledge/kbfreeze/**`；`docs/architecture/evidence/passb/b2-k0.md`、`docs/plans/passb/reports/b2-k0.md`。

**K5（属主 b2-k-integration，本修订轮冻结）可写：**

| 路径 | 依据 |
|---|---|
| `internal/modules/knowledge/module.go`（门面五操作实装 + 包注释重写） | DAG b2-k-integration owned_files「internal/modules/knowledge 装配面（RegisterRoutes/RegisterWorkers/Start/Stop 实装…）」；conventions §3「module.go 门面注释仅 b0 与该模块集成节点可写」 |
| `internal/modules/knowledge/module_test.go`（新建） | 同上（装配面测试），随产新文件 |
| `internal/modules/knowledge/README.md`（装配面说明段更新） | 同上（模块装配文档；K1–K4 未触碰，装配说明归集成节点） |
| `internal/modules/knowledge/docparser/anydoc/convert_linked_test.go`（**仅 :19 注释内旧路径字符串**；R1 修订轮 `grep -n` 于 k-process 与集成分支两副本复核均= :19，审校意见「:19→:20」不成立——行号漂移时按内容定位） | 别名删除的机械缺口就地补齐（conventions §10 共同原则；见 §8 K5.2 Step 2c） |
| `docs/architecture/moves/knowledge.yaml`（alias_obligations 18 行删除） | DAG owned_files「…+ 别名删除」；Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP 行级删除权 |
| `docs/architecture/passb/ownership-matrix.yaml` 的 aliases 区 18 行（`plan: 20-knowledge-program` 行删除；本分支副本——k-process 分支实测 18 行在位，集成 worktree `.worktrees/passb-int/…` 为同文件的集成分支视图，删除随本分支合并回流） | 同上（manifest 行与 matrix 行**同 commit 成对删除**，passbguard 双侧奇偶校验 check.go:167-206） |
| `docs/architecture/passb/briefs/b2-k-integration.md`（新建） | DAG b2-k-integration owned_files 第 2 项 |
| `docs/architecture/evidence/passb/b2-k-integration.md`、`docs/plans/passb/reports/b2-k-integration.md`（新建） | conventions §1.1；DAG evidence_paths |
| 基线对齐 merge commits（`codex/passb-b2-k-process` → 本分支） | Ruling WAVE-DEP-BASELINE |
| `docs/plans/passb/20-knowledge-program.md`（本文件，限 K5 属主段落的计划修订轮：审校修复/根因分析/replan 等 docs-only commit） | 本文件即 DAG 两节点共同 `plan_path`，b2-k-integration 为 §8 K5.1–K5.3 及修订新增段落属主（本文件头注 ：5-7）；分支在案先例：0805a084a（计划初版）、c30cb90ee（R1 审校修订）、37b081b7e（K5.1 OCR 根因分析 replan）；K5.3 Step 2 / §12 #3 差集检查据此放行本文件（**仅限 docs 修订 commit，不得夹带任何生产代码**） |

**K5 禁改（违者节点失败，conventions §1.2/§3）：**

- `internal/router/router.go`、`internal/router/routes_knowledge.go`、`internal/router/routes_infra.go`、`internal/router/files.go`、`internal/router/task.go`、`internal/router/sync_task.go`、`internal/container/**`（含 `recover_pending_wiki_tasks.go`）、`internal/bootstrap/**`——全部集成工程师独占/共享装配面（conventions §3；K5 仅**消费** `internal/bootstrap` 既有契约，不改其一字）；
- `internal/modules/knowledge/{kbfreeze,ingest,retrieval,wiki,faq,process,retriever,searchutil,semantic,chunker,docparser}/**` 中 K0–K4 属主产物（K5 只新增 `module.go`/`module_test.go`/README 段/上述单行注释）；
- `docs/architecture/passb/{contracts,event-catalog,exception-ledger}.yaml`（barrier 回写；K5.2 只读核对）；`ownership-matrix.yaml` 仅限 aliases 区 18 行删除（legacy_files 区零触碰）；
- migration 编号序列、`go.mod`、`go.sum`、生产 SQL、既有迁移文件、`cmd/desktop`、`docreader`、`client`（framework:33）；
- 宿主过渡物（K1–K4 的 shim/compat/垫片文件，见 §8 K5.2 Step 3 清单）——删除归 ib2，K5 只盘点登记。

---

## 5. K0 冻结一：共享类型分配表（Chunk / KnowledgeBase / Tag / semantic）

> **属主：b2-k0（done+approved）。K5 实施者只读；表内类型是 kbfreeze 守卫的冻结面——K5 新增的 `Dependencies`/`Module`/`HandlerSet` 等标识符不得与下表任一冻结名同名（`internal/modules/knowledge/kbfreeze/freeze_test.go` 机器强制）。**

**裁定 R0（类型单一事实源）**：K1-K4 期间，下表全部共享类型的唯一事实源是 `internal/types`（含 `internal/types/interfaces`）。任何子计划不得在 `internal/modules/knowledge/**` 新增同名/同形影子定义、不得复制类型、不得修改 `internal/types`；需要类型变更走 conventions §5 升级。`internal/types` 的最终拆分不在本程序范围。

| 类型/常量族 | 现定义位置（base 实测） | K1-K4 期间事实源 | 语义归属（最终 owner plan） | 主要跨域消费者 |
|---|---|---|---|---|
| `types.Chunk` / `ChunkType` / `ChunkStatus` / `ChunkFlags` / `ImageInfo` / `VideoInfo` | `internal/types/chunk.go:113` / `:13` / `:43` / `:54` / `:86` / `:102` | `internal/types` | K1（21-ingest） | K2/K3/K4、conversation、agentruntime tools、router/rbac |
| `types.KnowledgeBase` / `KnowledgeBaseConfig` / `AutoTagConfig` / `ChunkingConfig` / `StorageConfig` | `internal/types/knowledgebase.go:59` / `:152` / `:178` / `:244` / `:372` | `internal/types` | K2（22-retrieval） | K1/K4、datasource、airesource、commercial |
| `types.Knowledge` / `KnowledgeListFilter` / `ManualKnowledgePayload` / `KnowledgeSearchScope` / `KnowledgeCheckParams` | `internal/types/knowledge.go:125` / `:96` / `:268` / `:278` / `:485` | `internal/types` | K4（24-process） | K1/K2/K3、conversation、craft、insights |
| `types.KnowledgeTag` / `KnowledgeTagWithStats` / `TagReferenceCounts` / `KnowledgeTagRelation` | `internal/types/tag.go:12` / `:56` / `:63` / `:70` | `internal/types` | K2（22-retrieval） | K4、handler/tag |
| `types.TagScope` / `types.SearchResult` / `types.SearchParams` | `internal/types/search.go:19` / `:151` / `:230` | `internal/types` | K2（检索面）；消费只读 | conversation chat_pipeline、agentruntime tools、agentcatalog |
| `types.WikiPage` / `types.WikiFolder`（及 wiki_page.go 内 Revision/Issue/Stats/Graph 族） | `internal/types/wiki_page.go:191` / `:419` | `internal/types` | K3（23-wikifaq） | K4、agentruntime wiki tools |
| `types.SemanticModelWireRequest` / `SemanticModelMessage` / `SemanticModelParameters` / `SemanticModelCapability` / `SemanticModelIssuedCapability` / `SemanticModelInvocationResult` / `SemanticModelInvocationDisposition` / `SemanticModelInvocationClaim` 等 | `internal/types/semantic_model.go:7` 起 | `internal/types` | K2（22-retrieval）；语义引擎包 `internal/modules/knowledge/semantic` 不动 | commercial（预算）、airesource、handler/semantic_* |

**端口接口归属（B0 已冻结）**：`ChunkService`（`interfaces/chunk.go`）、`KnowledgeBaseService`（`…/knowledgebase.go`）、`KnowledgeService`（`…/knowledge.go`）、`KnowledgeTagService`（`…/tag.go`）、`WikiPageService`（`…/wiki_page.go`）、`RetrieveEngineService`（`…/retriever.go`）——六个 `contracts.yaml` capability-port（id 行号：1576/1627/1773/1843/1860/1726，`stability: frozen`）。实现体落位：ChunkService→K1、KnowledgeBaseService/KnowledgeTagService/RetrieveEngineService→K2、WikiPageService→K3、KnowledgeService→K4。接口签名变化 = 契约违约，须 ADR/Spec 修订。

**知识门面（`knowledge.facade`，contracts.yaml:1613，items：NewModule/RegisterRoutes/RegisterWorkers/Start/Stop）**：K0 不发明具体 Go 签名；**精确签名由 K5 在实装时按当时真实 seam 推导并在其 Integration Brief 中给出（本修订轮已在 §8 K5.1 兑现——推导记录与完整代码见该节，实测基准=K1–K4 分支终态）。**

---

## 6. K0 冻结二：跨 plan 未导出符号联动裁定

> **属主：b2-k0（done+approved）。K5 实施者只读；本节是 K5.1 Brief (d)/(e)（seam 装配接线表与宿主 shim 清单）的冻结依据。**

**背景（实证）**：DAG `package_private_couplings` 只登记跨模块 owner 对；本节是 K0 对知识域内部（K1↔K2↔K3↔K4）同宿主包未导出符号互耦的实测与裁定。

### 6.1 通用裁定

**R1（定义方三件套）**——跨 plan/跨 owner 消费的未导出符号，其定义文件属主搬迁该文件时，同一分支内完成：实现体 `git mv` 至 destination（随迁 `_test.go`）；落位包内添加导出薄包装（一行委托，不复制实现）；宿主仍有其他 owner 调用方时创建薄 shim 新文件（文件头注释 `// Pass B 过渡 shim：删除点 ib2`）。

**R2（调用方消费侧 seam）**——调用方属主搬迁自己文件时，对他 plan 未导出符号的调用点改为消费侧注入 seam（spec §4.2）。seam 的生产实现由 K5 装配按 Integration Brief 接到定义方导出包装；跨 owner 宿主调用点的最终改写由集成工程师在 ib2 执行。

**R3（同属主调用点直接改）**——定义与调用同属一个 K plan 时，随搬迁直接更新调用点。

### 6.2 联动符号全表（定义 file:line 与签名均为 base 实测）

**组 A——K3 定义、K1 消费：**

| 符号 | 定义 | 签名 | 调用点（属主） | 裁定 |
|---|---|---|---|---|
| `previewText` | `internal/application/service/wiki_ingest.go:1383`（K3） | `func previewText(s string, maxRunes int) string` | `extract.go:309`、`image_multimodal.go:280/:296`（K1）；K3 内部（R3） | K1 按 R2 建 seam；K3 按 R1 导出 `wiki.PreviewText` + 宿主 shim |

**组 B——K2 定义、K3/K4/datasource 消费（K2 Brief §2 已全量导出为 `retrieval/app` 包导出名）：**

| 符号 | 定义 | 签名 | 调用点（属主） | 裁定 |
|---|---|---|---|---|
| `recordKBActivity` | `service/kb_activity.go:95`（K2） | `func recordKBActivity(ctx context.Context, audit interfaces.AuditLogService, tenantID uint64, kbID string, action types.AuditAction, targetType string, targetID string, outcome types.AuditOutcome, details map[string]any)` | K3/K4/datasource（datasource_service.go 17 处等，K0 全量枚举） | K2 R1 导出 `app.RecordKBActivity` + 宿主 compat；调用点 ib2 直连改写 |
| `kbActivityTrigger` / `withKBActivityTask` / `kbActivityAppendSampleTitles` / `withKBActivitySuppressed` | `service/kb_activity.go:36/:27/:49/:87`（K2） | 见源码 | K3/K4/datasource | 同上（`app.KBActivityTrigger` 等） |
| `resolveDeadSlug` | `service/slug_fuzzy.go:91`（K2） | `func resolveDeadSlug(deadSlug string, displayText string, liveSlugs map[string]struct{}, titleToSlug map[string]string) (string, bool)` | K3：wiki_ingest.go:1670、wiki_page.go:1191 | `app.ResolveDeadSlug` |
| `resolveKBReadTenant` / `kbReadPermissions` | `service/knowledgebase_access.go:21/:14`（K2） | 见源码 | K3/K4/conversation | `app.ResolveKBReadTenant` / `app.KBReadPermissions` |

**组 C——K4 定义、跨 plan 消费：**

| 符号 | 定义 | 签名 | 调用点（属主） | 裁定 |
|---|---|---|---|---|
| `escapeLikeKeyword` | `repository/knowledge.go:22`（K4） | `func escapeLikeKeyword(keyword string) string` | identity/conversation/K2 | K4 落位 `process/repository.EscapeLikeKeyword`（K4.1 已交付）；ib2 直连改写 |
| `withKnowledgeCleanup` / `deleteReferencedKnowledge` | `service/knowledge_delete_plan.go:22/:36`（K4） | 见源码 | datasource/conversation/insights/airesource | 随 K4 推迟批，补迁窗导出 |
| `isValidFileType` | `service/knowledge_util.go:62`（K4） | `func isValidFileType(filename string) bool` | 仅 K4 内（同名异义无耦合，K0.1 复核轮消歧） | 直接随迁 |
| `RecordWikiContentActivity`（已导出） | `service/kb_activity.go:181`（K2） | 见源码 | K3 wiki 域 | `app.RecordWikiContentActivity`（wiki 包以导出别名引用同一实例） |

**组 D——K1 定义、跨 owner 消费（K1 Brief §6.2 R1-10 已交付 `ingest.SanitizeOCRText`/`ingest.BuildVLMCaptionPrompt`）：**

| 符号 | 定义 | 签名 | 调用点（属主） | 裁定 |
|---|---|---|---|---|
| `buildVLMCaptionPrompt` | `service/image_multimodal.go:53`（K1） | `func buildVLMCaptionPrompt(ctx context.Context, cfg types.VLMConfig) string` | conversation `temporary_document.go:560` | K1 导出 + 宿主 shim；conversation 调用点 ib2 改写 |
| `sanitizeOCRText` | `service/ocr_sanitizer.go:29`（K1） | `func sanitizeOCRText(raw string) string` | conversation `temporary_document.go:541` | 同上 |

**组 E——知识文件出向（定义属其他模块）：**

| 符号 | 定义（属主） | 调用点（属主） | 实测导出状态 | 裁定 |
|---|---|---|---|---|
| `knowledgeBaseModelUsageBindings` / `scopeKnowledgeBasesByModelID` | `repository/model_usage.go:8/:57`（12-commercial） | K4：repository/knowledgebase.go:242/:216/:235 | 未导出 | K4 R2 seam；导出义务 12-commercial/ib1 补课，ib2 接线（差异①） |
| `semanticBudgetFailure` | `service/semantic_model_budget.go:114`（12-commercial） | K2：semantic_model.go:102 | 未导出 | 同上 |
| `auditActor` / `auditActorRole` | identity（tenant_member.go:162 等） | K2：kb_activity.go:157/:160 | 未导出 | K2 R2 seam（`app/audit_actor_seam.go` 已落位）；K5/ib2 接 identity 导出 |
| `getParserEngineOverridesFromContext` | conversation 包级（attachment_processor.go:334） | 同名异义无耦合（K0.1 复核轮实测） | 未导出 | K4 直接随迁，无需 seam |

**裁定约束汇总**：R1/R2 使任一子分支独立 `go build ./...` 绿色；宿主 shim 全部登记删除点 **ib2**（与 `ownership-matrix.yaml` 全部 84 行 `delete_barrier: ib2` 一致）。

---

## 7. K0 冻结三：契约、路由、worker、hook、别名、例外清单指针

> **属主：b2-k0（done+approved）。本节是 K5 装配面（§8 K5.1）与删除面（§8 K5.2）的冻结清单。行号=base 实测；K5 实施时按当时分支复核（K3 已使 container.go 挂接行 :1053→:1058，见 K3 Brief (a) A9）。**

### 7.1 路由（11 项，`knowledge.routes`，contracts.yaml:1750；切换归 K5 Brief）

`RegisterChunkerDebugRoutes`（routes_knowledge.go:18）、`RegisterChunkRoutes`（:28）、`RegisterKnowledgeRoutes`（:67）、`RegisterFAQRoutes`（:143）、`RegisterKnowledgeBaseRoutes`（:185）、`RegisterSemanticModelPolicyRoutes`（:254）、`RegisterKnowledgeBaseActivityRoutes`（:266）、`RegisterKnowledgeTagRoutes`（:279）、`RegisterWikiPageRoutes`（:309）、`RegisterSemanticInternalRoutes`（routes_infra.go:14）、`serveKBScopedFiles`（files.go:325，router.go:332 调用）。

### 7.2 Worker（18 项，`knowledge.workers`，contracts.yaml:1893；注册行禁改，K5 交付模块侧 RegisterWorkers、切换申请入 Brief）

`TypeChunkExtract`、`TypeDataTableSummary`、`TypeDocumentProcess`、`TypeManualProcess`、`TypeFAQImport`、`TypeQuestionGeneration`、`TypeSummaryGeneration`、`TypeKBClone`、`TypeKnowledgeMove`、`TypeKnowledgeListDelete`、`TypeKnowledgeListReparse`、`TypeIndexDelete`、`TypeKBDelete`、`TypeImageMultimodal`、`TypeKnowledgePostProcess`、`TypeKnowledgeAutoTag`、`TypeWikiIngest`、`TypeWikiFinalize`（注册位点：`internal/router/task.go`（K5 详案实测 :267-:322 区段）与 `internal/router/sync_task.go`（`RegisterSyncHandlers` :143-:163 区段）；分发面=`AsynqTaskParams`/`SyncTaskParams` 的 knowledge 子集 9 字段，见 §8 K5.1 表）。

### 7.3 生命周期与事件

- Hook 1 项：`recoverPendingWikiTasks`（挂接 `must(container.Invoke(recoverPendingWikiTasks))`；func 定义 `internal/container/recover_pending_wiki_tasks.go:32`，签名 `func recoverPendingWikiTasks(db *gorm.DB, task interfaces.TaskEnqueuer)`；幂等性注释 :27-30「Duplicate triggers are harmless」）→ K5 门面 Start 的等价入口（§8 K5.1）。
- 事件 4 项 v1（event-catalog「Knowledge 家族」区）：`knowledge.processing.completed`、`knowledge.processing.failed`、`knowledge.index.completed`、`knowledge.deletion.completed`。K1-K4 搬迁未改 producer 语义、metadata 键与 ordering 语义（K5.3 复核项）。

### 7.4 AI Resource 消费（现状即合规，禁新增内部包 import）

knowledge 范围生产代码对 `internal/modules/airesource` 的合法消费（含例外台账 exc-0088..0091 与 K1-K4 新登记显形例外，全部 `remove_at: ib2`）——K5.2 逐条核对，不在本节点删除（删除前置=airesource 根门面暴露端口，见 K1 Brief §10）。

### 7.5 别名（18 条，`knowledge.yaml` alias_obligations；**删除属主=本计划 K5.2**，matrix 行 `plan: 20-knowledge-program, delete_barrier: ib2`）

`internal/application/repository/retriever/{doris,elasticsearch,elasticsearch/v7,elasticsearch/v8,milvus,neo4j,opensearch,postgres,qdrant,sqlite,tencentvectordb,weaviate}`、`internal/application/service/retriever`、`internal/infrastructure/chunker`、`internal/infrastructure/docparser`、`internal/infrastructure/docparser/anydoc`、`internal/infrastructure/semantic`、`internal/searchutil`（全部 `passb_task: B-knowledge`）。**2026-09-26 预检（k-process 分支实测）：18 个旧路径物理目录已全部不存在（Pass A 已搬）；全仓 import 零命中（唯一文本命中=`internal/modules/knowledge/docparser/anydoc/convert_linked_test.go:19` 的 build 指令注释，非 import）**——K5.2 按程序复检后成对删行。

### 7.6 例外（知识程序属主 30 条，`remove_at=ib2`；removal owner 见 plan 列）

| plan（removal owner 节点） | 条目 |
|---|---|
| 21-knowledge-ingest（b2-k-ingest） | exc-0088（B0 批）+ exc-0106..0111（K1.6 登记）= 7 条 |
| 22-knowledge-retrieval（b2-k-retrieval） | exc-0089/0090/0091（B0 批）+ exc-0112..0116 = 8 条 |
| 23-knowledge-wikifaq（b2-k-wikifaq） | exc-0117..0129 = 13 条 |
| 24-knowledge-process（b2-k-process） | exc-0130/0131 = 2 条 |

K1–K4 四节点均未删行（各 Brief 明示「本节点不删行」；删除前置=上游门面/端口合法化，K1 Brief §10、K2 Brief §7）。**K5.2 只核对与登记收口编排，不删行**（conventions §3：exception-ledger 仅各属主节点删除自己的行；行属主是 21/22/23/24 计划，非 20）。

消费方例外（非本程序属主）：conversation→`knowledge/searchutil` 9 对（exc-0070/0074/0075/0076/0077/0078/0083/0084/0085，owner 35-conversation-program，remove_at ib3）、agentruntime→`knowledge/searchutil` 等（owner 32/33，ib3）。K2 保持 `searchutil` 包路径与 API 稳定即可。

---

## 8. 任务（业务完整、可独立审阅；一任务一 commit，conventions §4）

> **任务属主标注：Task K0.1–K0.3 属主节点 `b2-k0`（已 done+approved @5bcb7986，以下为历史记录原文）；Task K5.1–K5.3 属主节点 `b2-k-integration`（本修订轮展开为可执行详案）。K1/K2/K3/K4 任务见子计划 21/22/23/24（各自属主节点）。**

### Task K0.1 — 冻结分配表与联动裁定表（本文件 §5–§7）【属主：b2-k0；已完成】

- [x] 以 base `8c45a8815` 工作树复核 §5 表内每个 file:line 与 §6 表内每个签名/调用点（K0.1 复核轮 2026-09-23 执行完毕；唯一修订为组 E `getParserEngineOverridesFromContext` 同名伪影消歧，记录见 §13）。
- [x] 复核 §5 分配表与 `ownership-matrix.yaml` 的 84 行零漂移：`python3 -c` 读 yaml 统计 plan 计数 = `{21:9, 22:29, 23:18, 24:28}` total 84。
- [x] 评审通过后 §5/§6/§7 即为 K1-K5 的约束事实源。
- **产出文件**：本文件。**验收**：评审 approved；84 行计数核对记录写入报告。

### Task K0.2 — kbfreeze 守卫测试包（R0 与六 port 签名的机器强制）【属主：b2-k0；已完成】

**文件**：`internal/modules/knowledge/kbfreeze/freeze_test.go`（package `kbfreeze`，仅测试文件）。**测试 1** `TestKnowledgeSharedTypesHaveNoShadowDefinitions`（30 冻结名 + 豁免表唯一条目 `chunker/splitter.go → Chunk`）；**测试 2** `TestFrozenCapabilityPortInterfacesUnchanged`（六端口方法集断言）。

```bash
go test ./internal/modules/knowledge/kbfreeze/ -count=1 -v   # 预期：2 PASS，exit 0
go build ./...                                              # 预期：exit 0
make check-backend-architecture                             # 预期：exit 0
make verify-module-moves                                    # 预期：exit 0
```

- **验收**：两条测试 PASS；守卫包未 import `internal/application/*`、`internal/handler/*`；diff 仅含 kbfreeze。commit：`test(passb): b2-k0 知识共享类型与端口冻结守卫`。

### Task K0.3 — 前置差异上报、证据与报告【属主：b2-k0；已完成】

- [x] 差异①（commercial 三符号未导出）、差异②（b0 两张未闭环修复工单）、差异③（F2 status 机制未落盘——K5 的「门面 current 化」与 ib2 状态回写在字段补落盘前无载体；**补落盘前 K5 仅交付门面实装与装配 Brief，不声称契约状态变更**，本修订轮沿用该裁定）。
- [x] 执行证据命令包（base=8c45a8815）；写 evidence 与报告；回填报据。

### Task K5.1 — knowledge 门面五操作实装 + 装配切换 Integration Brief【属主：b2-k-integration】

**业务目标**：把 22 行零实现骨架 `internal/modules/knowledge/module.go` 实装为可编译、可测试的模块门面（contracts.yaml knowledge.facade:1613 的五操作），并产出交给 ib2 集成工程师的装配切换 Brief。**本任务不触碰任何 router/container/bootstrap 文件**（§4 禁改清单）；门面是纯新增装配面，切换由集成工程师按 Brief 执行。

**输入**：K1–K4 落位包的真实导出面（下表，均于 `codex/passb-b2-k-process` HEAD 实测）；`internal/bootstrap` 既有契约（R1 修订轮实读修正行号：`WorkerRegistry` workers.go:12、`WorkerSink.RegisterTaskHandler(taskType string, handler any)` :21-22、`NewWorkerRegistry(mode string, sink WorkerSink) *WorkerRegistry` :28、`(*WorkerRegistry).Register(taskType string, handler any) error` :39——重复登记返回 `already registered: …` 错误、`(*WorkerRegistry).TaskTypes() []string` :57、`VerifyWorkerParity(redis, lite *WorkerRegistry) error` :70；包注释明示「IA 障碍任务才接入 router/container」，当前零生产 importer，**K5 是首个生产消费方**；import 方向 模块→bootstrap 合法：无环、不在 architectureguard `horizontalDirs`（check.go:1055-1059）、非模块间禁互导面（moduleImportBase 前缀检查）——R1 模拟实测 `make check-backend-architecture` exit 0 复证）。

**签名推导记录（K0 §5「不发明」纪律的兑现；每条对应真实 seam）**：

| 五操作 | 真实 seam（file:line，k-process HEAD 实测） | 推导结论 |
|---|---|---|
| `NewModule(deps Dependencies) (*Module, error)` | `router/task.go:37-63 AsynqTaskParams`、`sync_task.go:115-131 SyncTaskParams` 的 knowledge 子集（9 分发面字段，类型全部=interfaces 冻结端口）+ container.go 的 handler dig Provide 面 | `Dependencies` 聚合 9 worker 分发面 + 7 路由 handler 供给 + 1 生命周期入口；构造仍在 dig 容器（conventions §3），门面只收已构造实例（窄端口注入，spec §4.3） |
| `RegisterWorkers(redis, lite *bootstrap.WorkerRegistry) error` | task.go 的 18 行 `mux.HandleFunc(types.TypeX, params.Y.Handle/ProcessX)` 与 sync_task.go 的 18 行 `params.Executor.RegisterHandler(...)` 逐条同构（两栈第二参同形 `func(context.Context, *asynq.Task) error`） | 18 类型按稳定序登记进双栈 registry（`WorkerSink.RegisterTaskHandler` 以 any 承载）+ `bootstrap.VerifyWorkerParity` 收尾——即 DAG produced_artifacts「18 workers Redis/Lite 双栈装配」。ib2 生产适配两枚 WorkerSink（Redis=`mux.HandleFunc`、Lite=`SyncTaskExecutor.RegisterHandler`，同形方法值），入 Brief (b) |
| `RegisterRoutes() (HandlerSet, error)` | 11 组注册函数（§7.1）路由体逐条耦合 `internal/router` 包私有 `rbacGuards`（`g.Viewer()`/`g.KBAccessRead("id")`/`g.OwnedKBOrAdmin()` 等闭包族）——**B2 无平台端口可消费（spec §4.3「具体 Go 签名由实施计划根据现有 RBAC…seam 确定」：RBAC seam 现状=router 包私有）**；architectureguard 路由扫描面=`internal/router`+`internal/handler`（discovery.go:65-66），模块内路由代码不入 633 计数 | 实装为「供给面交付」：校验并返回 7 个已落位 handler 的 `HandlerSet`；ib2 按 Brief (a) 把路由块 handler 形参切到模块供给（RBAC 语义与路由计数 633 零变化，conventions §8）；路由体迁入模块属 IA/后续计划。**禁止在 module.go 复刻 method+path 路由表（双写，framework:30）** |
| `Start(ctx context.Context) error` | `container.go:1058 must(container.Invoke(recoverPendingWikiTasks))`（func at recover_pending_wiki_tasks.go:32，包私有，K5 禁改该文件） | 调用注入的 `PendingWikiRecovery func(ctx context.Context)`（生产值=container 侧现函数的等价闭包，ib2 注入并撤原挂接行——单一注册点，spec §4.3「同一钩子只注册一次」；过渡期双重触发无害，恢复函数幂等 :27-30）。nil 时 no-op（部分装配/测试场景） |
| `Stop(ctx context.Context) error` | 知识域当前无模块自持后台 goroutine（housekeeping 调度属 K4 推迟件，经 `KnowledgeHousekeeping` 窄端口由 42-system-policy 消费方调度，24 计划 §5.3 表「Housekeeping 调度」行；**不得出现第二套清扫实现**） | 预留对称面，返回 nil |

**骨架示意签名差异处置（R1 修订轮登记）**：现骨架（module.go 22 行 0 函数，k-process HEAD 实读）示意 `RegisterRoutes(r RouteRegistrar)`（:14）/ `RegisterWorkers(mux WorkerRegistrar)`（:16）；本任务实装为 `RegisterRoutes() (HandlerSet, error)` / `RegisterWorkers(redis, lite *bootstrap.WorkerRegistry) error`。合法性依据：contracts.yaml knowledge.facade（:1613 区段）`signature: façade` 为**占位值非 Go 签名**，`items` 仅冻结五操作名单（NewModule/RegisterRoutes/RegisterWorkers/Start/Stop）——**签名本身未被冻结**；K0 §5 裁定「精确签名由 K5 实装时按当时真实 seam 推导」（上表即兑现：`RouteRegistrar`/`WorkerRegistrar` 在 B2 无真实 seam 对应，实装消费 `bootstrap.WorkerRegistry` 既有契约）。module.go 属 §4 K5 可写，整文件重写时占位示意随旧注释消亡；Brief (f) 照录实装签名。**但 passbguard 冻结 module.go 注释的「形态」而非签名**（check.go:312-325：五操作须以 `//\t` 缩进显式声明——facadeOpRE `^//\t(?:\(m \*Module\) )?(NewModule|RegisterRoutes|RegisterWorkers|Start|Stop)\(`；且须含三个计数句式 `当前 (\d+) 项入口`、`当前 (\d+) 项，见 integration_points\.workers`、`当前 (\d+) 项生命周期挂点`，数值须等于 manifest integration_points 冻结计数 11/18/1）——下方完整代码的包注释已按该契约撰写（R1 模拟实测：不满足将新增 8 条 contract-facade-shape-drift/contract-facade-count-drift 诊断）。

**写入文件与完整内容：**

1. **`internal/modules/knowledge/module.go`（重写）**——完整代码（编译目标；import 的包均真实存在）：

```go
// Package knowledge 是 WeKnora 后端 knowledge 模块（Pass B B2 集成态门面）。
//
// 职责（spec §5.18 / F0）：知识库、文档、Chunk、Tag、FAQ、Wiki、知识图谱、
// 检索与语义知识索引。
//
// 门面五操作（contracts.yaml knowledge.facade:1613 冻结五操作名单）由
// b2-k-integration（20 计划 K5.1）按真实 seam 实装，声明如下（passbguard
// facade 形态契约 check.go:312-325：五操作须以 //\t 缩进显式声明、计数
// 句式须与 manifest integration_points 冻结值一致）：
//
//	NewModule(deps Dependencies) (*Module, error)
//	    构造模块实例；16 个必填装配依赖（9 worker 分发面 + 7 路由 handler 供给）
//	    缺失时返回列出全部缺失字段名的错误。
//	(m *Module) RegisterRoutes() (HandlerSet, error)
//	    交付路由块模块侧 handler 供给（当前 11 项入口，见 manifest integration_points.routes）。
//	(m *Module) RegisterWorkers(redis, lite *bootstrap.WorkerRegistry) error
//	    把任务处理器登记进 Redis/Lite 双栈注册表（当前 18 项，见 integration_points.workers）。
//	(m *Module) Start(ctx context.Context) error
//	    启动生命周期挂点（当前 1 项生命周期挂点，见 manifest integration_points.lifecycle_hooks）。
//	(m *Module) Stop(ctx context.Context) error
//	    优雅停止；当前无模块自持后台 goroutine，预留对称面。
//
// worker 面 = router/task.go 与 router/sync_task.go 的 knowledge 子集（18 类型
// 双栈同构）；生命周期面 = container.go:1058 挂接的 recoverPendingWikiTasks
// 等价入口；路由供给面 = 11 组注册函数的模块侧 handler 供给（路由体与
// rbacGuards guard 语义留驻 internal/router，ib2 按 Brief 切换形参——RBAC
// 语义与路由计数 633 零变化，conventions §8）。
package knowledge

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"

	"github.com/Tencent/WeKnora/internal/bootstrap"
	"github.com/Tencent/WeKnora/internal/modules/knowledge/faq"
	"github.com/Tencent/WeKnora/internal/modules/knowledge/ingest"
	kbhandler "github.com/Tencent/WeKnora/internal/modules/knowledge/retrieval/app/handler"
	"github.com/Tencent/WeKnora/internal/modules/knowledge/wiki"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Dependencies 是 knowledge 模块的装配依赖（窄端口注入，spec §4.3）。
// 字段类型与 router.AsynqTaskParams / router.SyncTaskParams 的 knowledge 子集
// 逐一对应：18 个 worker 的全部分发面经 interfaces 冻结端口注入，构造仍在
// dig 容器（conventions §3 集成工程师独占），门面只收已构造实例。
type Dependencies struct {
	// 18 workers 的分发面（9 个；task.go:37-63 / sync_task.go:115-131 knowledge 子集）。
	KnowledgeService     interfaces.KnowledgeService
	KnowledgeBaseService interfaces.KnowledgeBaseService
	TagService           interfaces.KnowledgeTagService
	ChunkExtractor       interfaces.TaskHandler // dig name "chunkExtractor"
	DataTableSummary     interfaces.TaskHandler // dig name "dataTableSummary"
	ImageMultimodal      interfaces.TaskHandler // dig name "imageMultimodal"
	KnowledgePostProcess interfaces.TaskHandler // dig name "knowledgePostProcess"
	KnowledgeAutoTag     interfaces.TaskHandler // dig name "knowledgeAutoTag"
	WikiIngest           interfaces.TaskHandler // dig name "wikiIngest"

	// 路由块 handler 供给面（7 个；§7.1 全表 11 项中已落位模块包的组，其余
	// 4 项为宿主推迟件：3 组 handler（RegisterKnowledgeRoutes/
	// RegisterKnowledgeBaseRoutes/RegisterKnowledgeBaseActivityRoutes）+
	// serveKBScopedFiles 文件服务面（非 handler 供给，无字段）；ib2/补迁窗后
	// 增补字段——装配面扩展，非契约变更）。
	Chunk               *ingest.ChunkHandler                     // RegisterChunkRoutes（routes_knowledge.go:28）
	ChunkerDebug        gin.HandlerFunc                          // RegisterChunkerDebugRoutes（:18）；生产值 ingest.PreviewChunking（chunker_debug.go:123）
	WikiPage            *wiki.WikiPageHandler                    // RegisterWikiPageRoutes（:309）
	FAQ                 *faq.FAQHandler                          // RegisterFAQRoutes（:143）
	Tag                 *kbhandler.TagHandler                    // RegisterKnowledgeTagRoutes（:279）
	SemanticModelPolicy *kbhandler.SemanticModelPolicyHandler    // RegisterSemanticModelPolicyRoutes（:254）
	SemanticInternal    *kbhandler.SemanticInternalHandler       // RegisterSemanticInternalRoutes（routes_infra.go:14）

	// 生命周期挂点（1 个）：container.go:1058 挂接的 recoverPendingWikiTasks
	// 等价入口（恢复语义冻结面见 K3 Brief (a) A9——fail-closed 清扫、
	// asynq.TaskID("wiki-finalize-"+scope)、MaxRetry 10、Timeout 60/30min、
	// 幂等）。生产值由集成工程师按 Brief 注入；nil 时 Start 为 no-op。
	PendingWikiRecovery func(ctx context.Context)
}

// Module 是 knowledge 模块装配门面实例。
type Module struct {
	deps Dependencies
}

// NewModule 构造模块实例；16 个必填装配依赖（9 分发面 + 7 handler 供给）缺失时
// 返回列出全部缺失字段名的错误（装配错误在注册前暴露，快速失败）。
func NewModule(deps Dependencies) (*Module, error) {
	var missing []string
	if deps.KnowledgeService == nil {
		missing = append(missing, "KnowledgeService")
	}
	if deps.KnowledgeBaseService == nil {
		missing = append(missing, "KnowledgeBaseService")
	}
	if deps.TagService == nil {
		missing = append(missing, "TagService")
	}
	if deps.ChunkExtractor == nil {
		missing = append(missing, "ChunkExtractor")
	}
	if deps.DataTableSummary == nil {
		missing = append(missing, "DataTableSummary")
	}
	if deps.ImageMultimodal == nil {
		missing = append(missing, "ImageMultimodal")
	}
	if deps.KnowledgePostProcess == nil {
		missing = append(missing, "KnowledgePostProcess")
	}
	if deps.KnowledgeAutoTag == nil {
		missing = append(missing, "KnowledgeAutoTag")
	}
	if deps.WikiIngest == nil {
		missing = append(missing, "WikiIngest")
	}
	if deps.Chunk == nil {
		missing = append(missing, "Chunk")
	}
	if deps.ChunkerDebug == nil {
		missing = append(missing, "ChunkerDebug")
	}
	if deps.WikiPage == nil {
		missing = append(missing, "WikiPage")
	}
	if deps.FAQ == nil {
		missing = append(missing, "FAQ")
	}
	if deps.Tag == nil {
		missing = append(missing, "Tag")
	}
	if deps.SemanticModelPolicy == nil {
		missing = append(missing, "SemanticModelPolicy")
	}
	if deps.SemanticInternal == nil {
		missing = append(missing, "SemanticInternal")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("knowledge module: missing assembly dependencies: %s", strings.Join(missing, ", "))
	}
	return &Module{deps: deps}, nil
}

// workerHandlers 返回 18 个任务类型的处理器映射——与 router/task.go、
// router/sync_task.go 的 knowledge 子集逐条同构（同类型同方法）。
func (m *Module) workerHandlers() map[string]func(context.Context, *asynq.Task) error {
	return map[string]func(context.Context, *asynq.Task) error{
		types.TypeChunkExtract:         m.deps.ChunkExtractor.Handle,
		types.TypeDataTableSummary:     m.deps.DataTableSummary.Handle,
		types.TypeDocumentProcess:      m.deps.KnowledgeService.ProcessDocument,
		types.TypeManualProcess:        m.deps.KnowledgeService.ProcessManualUpdate,
		types.TypeFAQImport:            m.deps.KnowledgeService.ProcessFAQImport,
		types.TypeQuestionGeneration:   m.deps.KnowledgeService.ProcessQuestionGeneration,
		types.TypeSummaryGeneration:    m.deps.KnowledgeService.ProcessSummaryGeneration,
		types.TypeKBClone:              m.deps.KnowledgeService.ProcessKBClone,
		types.TypeKnowledgeMove:        m.deps.KnowledgeService.ProcessKnowledgeMove,
		types.TypeKnowledgeListDelete:  m.deps.KnowledgeService.ProcessKnowledgeListDelete,
		types.TypeKnowledgeListReparse: m.deps.KnowledgeService.ProcessKnowledgeListReparse,
		types.TypeIndexDelete:          m.deps.TagService.ProcessIndexDelete,
		types.TypeKBDelete:             m.deps.KnowledgeBaseService.ProcessKBDelete,
		types.TypeImageMultimodal:      m.deps.ImageMultimodal.Handle,
		types.TypeKnowledgePostProcess: m.deps.KnowledgePostProcess.Handle,
		types.TypeKnowledgeAutoTag:     m.deps.KnowledgeAutoTag.Handle,
		types.TypeWikiIngest:           m.deps.WikiIngest.Handle,
		types.TypeWikiFinalize:         m.deps.WikiIngest.Handle,
	}
}

// RegisterWorkers 把 18 个任务处理器登记到 Redis 与 Lite 双栈注册表并校验
// 双栈集合一致（bootstrap.VerifyWorkerParity）。本模块是 internal/bootstrap
// 契约的首个生产消费方（该包零内部 import、方向合法，见 20 计划 §8 K5.1
// 输入段）。ib2 生产适配（Brief (b)）：WorkerSink 两枚——Redis 栈包
// *asynq.ServeMux（委托 mux.HandleFunc）、Lite 栈包 *router.SyncTaskExecutor
// （委托 RegisterHandler；二者第二参同形 func(context.Context, *asynq.Task) error）。
func (m *Module) RegisterWorkers(redis, lite *bootstrap.WorkerRegistry) error {
	handlers := m.workerHandlers()
	names := make([]string, 0, len(handlers))
	for tt := range handlers {
		names = append(names, tt)
	}
	sort.Strings(names)
	for _, tt := range names {
		h := handlers[tt]
		if err := redis.Register(tt, h); err != nil {
			return fmt.Errorf("knowledge module: register worker %q (redis): %w", tt, err)
		}
		if err := lite.Register(tt, h); err != nil {
			return fmt.Errorf("knowledge module: register worker %q (lite): %w", tt, err)
		}
	}
	return bootstrap.VerifyWorkerParity(redis, lite)
}

// HandlerSet 是 knowledge 11 组路由注册（§7.1）的模块侧 handler 供给
// （已落位 7 组；字段与 Dependencies 对应段一一对应）。
type HandlerSet struct {
	Chunk               *ingest.ChunkHandler
	ChunkerDebug        gin.HandlerFunc
	WikiPage            *wiki.WikiPageHandler
	FAQ                 *faq.FAQHandler
	Tag                 *kbhandler.TagHandler
	SemanticModelPolicy *kbhandler.SemanticModelPolicyHandler
	SemanticInternal    *kbhandler.SemanticInternalHandler
}

// RegisterRoutes 交付 knowledge 路由块的模块侧 handler 供给。
// B2 实装口径：11 组注册函数留驻 internal/router（路由体耦合包私有
// rbacGuards 平台面）；ib2 按 Brief (a) 把路由块 handler 形参切到本供给
// （RBAC 语义与路由计数 633 零变化）。禁止在本包复刻路由表（双写）。
func (m *Module) RegisterRoutes() (HandlerSet, error) {
	return HandlerSet{
		Chunk:               m.deps.Chunk,
		ChunkerDebug:        m.deps.ChunkerDebug,
		WikiPage:            m.deps.WikiPage,
		FAQ:                 m.deps.FAQ,
		Tag:                 m.deps.Tag,
		SemanticModelPolicy: m.deps.SemanticModelPolicy,
		SemanticInternal:    m.deps.SemanticInternal,
	}, nil
}

// Start 启动模块生命周期挂点。当前唯一挂点=PendingWikiRecovery
// （container.go:1058 的模块侧等价入口）；nil 时 no-op。
// ib2 切换（Brief (c)）：原挂接行改经本门面（单一注册点，spec §4.3）；
// 过渡期双重触发无害（恢复函数幂等，recover_pending_wiki_tasks.go:27-30）。
func (m *Module) Start(ctx context.Context) error {
	if m.deps.PendingWikiRecovery != nil {
		m.deps.PendingWikiRecovery(ctx)
	}
	return nil
}

// Stop 优雅停止。知识域当前无模块自持后台 goroutine（housekeeping 调度经
// KnowledgeHousekeeping 窄端口由 42-system-policy 消费方调度，24 计划 §5.3；
// 不得出现第二套清扫实现）；预留对称面。
func (m *Module) Stop(ctx context.Context) error {
	return nil
}
```

（上例为可直接落盘的编译目标；逐字段 if 保持零依赖可读性，实施时可收敛为表驱动，语义不变——字段名与错误文案保持，测试断言以字段名子串匹配。**包注释中 facade 形态契约句式（`//\t` 五操作声明 + 三计数句 11/18/1）任何情况下不得改动或删除——passbguard 强制，R1 模拟实测改动即产生 8 条 drift 诊断。**）

2. **`internal/modules/knowledge/module_test.go`（新建）**——五组测试：

```go
package knowledge

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"

	"github.com/Tencent/WeKnora/internal/bootstrap"
	"github.com/Tencent/WeKnora/internal/modules/knowledge/faq"
	"github.com/Tencent/WeKnora/internal/modules/knowledge/ingest"
	kbhandler "github.com/Tencent/WeKnora/internal/modules/knowledge/retrieval/app/handler"
	"github.com/Tencent/WeKnora/internal/modules/knowledge/wiki"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// stubTaskHandler 满足 interfaces.TaskHandler（task_handler.go:10，单方法）。
type stubTaskHandler struct{}

func (stubTaskHandler) Handle(ctx context.Context, t *asynq.Task) error { return nil }

// stubXXXService 以「嵌入 nil 接口」满足巨型冻结端口：本包测试只做非空校验、
// 方法值创建与登记（bootstrap.WorkerRegistry.Register 以 any 承载 handler、
// 不调用），永不调用服务方法。语义依据：Go 方法值创建不解引用嵌入的 nil
// 接口（2026-09-26 已以独立 go test 实证，见 20 计划 §13 自检记录）。
type stubKnowledgeService struct{ interfaces.KnowledgeService }
type stubKnowledgeBaseService struct{ interfaces.KnowledgeBaseService }
type stubKnowledgeTagService struct{ interfaces.KnowledgeTagService }

// knowledgeWorkerTypes 是 18 个 knowledge 任务类型的期望清单
// （knowledge.yaml integration_points.workers / contracts.yaml:1893）。
var knowledgeWorkerTypes = []string{
	types.TypeChunkExtract, types.TypeDataTableSummary, types.TypeDocumentProcess,
	types.TypeManualProcess, types.TypeFAQImport, types.TypeQuestionGeneration,
	types.TypeSummaryGeneration, types.TypeKBClone, types.TypeKnowledgeMove,
	types.TypeKnowledgeListDelete, types.TypeKnowledgeListReparse, types.TypeIndexDelete,
	types.TypeKBDelete, types.TypeImageMultimodal, types.TypeKnowledgePostProcess,
	types.TypeKnowledgeAutoTag, types.TypeWikiIngest, types.TypeWikiFinalize,
}

func fullDependencies(recovery func(ctx context.Context)) Dependencies {
	return Dependencies{
		KnowledgeService:     stubKnowledgeService{},
		KnowledgeBaseService: stubKnowledgeBaseService{},
		TagService:           stubKnowledgeTagService{},
		ChunkExtractor:       stubTaskHandler{},
		DataTableSummary:     stubTaskHandler{},
		ImageMultimodal:      stubTaskHandler{},
		KnowledgePostProcess: stubTaskHandler{},
		KnowledgeAutoTag:     stubTaskHandler{},
		WikiIngest:           stubTaskHandler{},
		Chunk:                &ingest.ChunkHandler{},
		ChunkerDebug:         func(c *gin.Context) {},
		WikiPage:             &wiki.WikiPageHandler{},
		FAQ:                  &faq.FAQHandler{},
		Tag:                  &kbhandler.TagHandler{},
		SemanticModelPolicy:  &kbhandler.SemanticModelPolicyHandler{},
		SemanticInternal:     &kbhandler.SemanticInternalHandler{},
		PendingWikiRecovery:  recovery,
	}
}

func TestNewModuleRejectsMissingAssemblyDependencies(t *testing.T) {
	deps := fullDependencies(nil)
	deps.KnowledgeService = nil
	deps.Chunk = nil
	_, err := NewModule(deps)
	if err == nil ||
		!strings.Contains(err.Error(), "KnowledgeService") ||
		!strings.Contains(err.Error(), "Chunk") {
		t.Fatalf("want missing-deps error naming KnowledgeService and Chunk, got %v", err)
	}
	if _, err := NewModule(fullDependencies(nil)); err != nil {
		t.Fatalf("full dependencies must construct, got %v", err)
	}
}

func TestRegisterWorkersDualStackParity(t *testing.T) {
	m, err := NewModule(fullDependencies(nil))
	if err != nil {
		t.Fatal(err)
	}
	redis := bootstrap.NewWorkerRegistry("redis", nil)
	lite := bootstrap.NewWorkerRegistry("lite", nil)
	if err := m.RegisterWorkers(redis, lite); err != nil {
		t.Fatalf("RegisterWorkers: %v", err)
	}
	if err := bootstrap.VerifyWorkerParity(redis, lite); err != nil {
		t.Fatalf("parity: %v", err)
	}
	want := append([]string(nil), knowledgeWorkerTypes...)
	sort.Strings(want)
	if got := redis.TaskTypes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("registered task types drift: got %d %v, want %d", len(got), got, len(want))
	}
	// 同一模块重复登记必须被 bootstrap 拒绝（already-registered 前缀契约）。
	if err := m.RegisterWorkers(redis, lite); err == nil ||
		!strings.HasPrefix(err.Error(), "knowledge module: register worker") {
		t.Fatalf("re-registration must be rejected, got %v", err)
	}
}

func TestRegisterRoutesReturnsHandlerSupply(t *testing.T) {
	m, err := NewModule(fullDependencies(nil))
	if err != nil {
		t.Fatal(err)
	}
	set, err := m.RegisterRoutes()
	if err != nil {
		t.Fatalf("RegisterRoutes: %v", err)
	}
	if set.Chunk == nil || set.WikiPage == nil || set.FAQ == nil || set.Tag == nil ||
		set.SemanticModelPolicy == nil || set.SemanticInternal == nil || set.ChunkerDebug == nil {
		t.Fatal("handler supply must be fully populated")
	}
}

func TestStartInvokesPendingWikiRecovery(t *testing.T) {
	called := false
	m, _ := NewModule(fullDependencies(func(ctx context.Context) { called = true }))
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !called {
		t.Fatal("Start must invoke PendingWikiRecovery when wired")
	}
	mNil, _ := NewModule(fullDependencies(nil))
	if err := mNil.Start(context.Background()); err != nil {
		t.Fatalf("Start with nil recovery must be a no-op, got %v", err)
	}
}

func TestStopReturnsNil(t *testing.T) {
	m, _ := NewModule(fullDependencies(nil))
	if err := m.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
```

3. **`docs/architecture/passb/briefs/b2-k-integration.md`（新建）**——汇总 K1–K4 四 Brief + 本门面，至少含：
   - **(a) 11 路由注册切换表**（§7.1 全表，宿主→`HandlerSet` 供给逐一列出；`routes_knowledge.go`/`routes_infra.go`/`files.go` 形参类型切换申请，RBAC/633 零变化声明；K1 Brief §9 ②③、K3 Brief (a) A7/A8 并发项合并）；
   - **(b) 18 worker 双栈装配切换申请**：`WorkerSink` 两枚适配器（Redis=`mux.HandleFunc` 委托、Lite=`SyncTaskExecutor.RegisterHandler` 委托）+ 集成工程师把 task.go/sync_task.go 的 knowledge 18 行切到 `Module.RegisterWorkers(redis, lite)` 的切换序（注册行本体集成工程师独占，conventions §3）；
   - **(c) `recoverPendingWikiTasks` 切模块 Start 的等价性说明**（container.go:1058 行切换申请；恢复语义冻结面照录 K3 Brief (a) A9；单一注册点原则；过渡期幂等无害）；
   - **(d) seam 装配接线表**（K1 Brief §7 全表 + K3 Brief (d) faq/wiki Seams 表合并；seam → 定义方导出符号 → 落位包；K4 Brief (g) 已就绪符号与随推迟批符号分列）；
   - **(e) 宿主薄 shim/compat 删除批清单**（K1 4 文件 + K2 3+1 + K3 7+垫片 + K4 2；逐一「删除前置=残留 importer 清零」；Ruling TRANSITION-SHIM-ROW-REGISTRATION 成对删行口径）；
   - **(f) 五操作精确签名**（本任务已实装的 `NewModule`/`RegisterWorkers`/`RegisterRoutes`/`Start`/`Stop` 签名照录 + Dependencies 字段表）；
   - **(g) K5.2 的 18 别名删除结论与例外 30 行收口编排**（ib2 删除批：airesource/policy 门面前置、container.go:36-38 三行旧 import 改写申请——K0 §7.5/K1 Brief §12 义务）；
   - **(h) 推迟件汇总裁定请求**（K2 14 件 + K4 17+环境阻断件 + repository/kbshare.go 计划空位——协调者在 ib2 前裁定补迁窗口归属；K4 Brief (f)、K2 Brief §5 汇总）。
   - Brief 头注：`container.go:316` 已提前落地项（K3 Brief (a) A1，Ruling CYCLE-FORCED-COMPOSITION）IB2 转核验清单。

**TDD 步骤与命令（worktree 根执行；RED→GREEN→REFACTOR，conventions §1.4）：**

```bash
# Step 0 前置核验（§2 P-K5-1..P-K5-7 全过；P-K5-2 已按 Case A/B 完成并登记 ALIGN_SHA）
git status                                    # 干净
git log --oneline -3                          # Case A：含 P-K5-2 新 merge commit；Case B：含既有对齐 merge commit（判据以 P-K5-2 ancestor 检查为准，本行不构成独立判据）

# Step 1 RED：先写 module_test.go（完整内容见上），module.go 仍为骨架
go test ./internal/modules/knowledge/ -count=1
# 预期：编译失败（undefined: NewModule / Dependencies / Module / HandlerSet 等）——RED 成立，退出码非 0

# Step 2 GREEN：重写 module.go（完整内容见上，含 "strings" import）
go test ./internal/modules/knowledge/ -count=1 -v
# 预期：TestNewModuleRejectsMissingAssemblyDependencies / TestRegisterWorkersDualStackParity /
#       TestRegisterRoutesReturnsHandlerSupply / TestStartInvokesPendingWikiRecovery /
#       TestStopReturnsNil 全部 PASS，退出码 0

# Step 3 邻接回归（K0 守卫 + 全仓编译 + 三门禁；R1 修订轮已按本节内容于干净副本全量模拟实测）
go test -count=1 ./internal/modules/knowledge/kbfreeze/   # 预期：2 PASS（门面新增标识符无影子冲突）
go build ./...                                             # 预期：退出码 0
make check-backend-architecture                            # 预期：退出码 0（total=633：literal+apiKeyRoute+handle、redis=23 lite=23、hooks=58、modules=16 不变）
make verify-module-moves                                   # 预期：退出码 0（16 manifests verified）
make check-passb-readiness                                 # 原样执行；预期退出码 1（基线 218 条既有诊断，P-K5-7 预裁定），退出码与输出如实入报告
go run ./tools/passbguard -root . 2>&1 | grep -v '^exit status' | sort | diff - /tmp/k5-readiness-baseline.txt
# 预期：diff 恰为 P-K5-7 (ii) 预登记的 3 行（module.go 已于 Step 2 落盘，三个冻结端口名引用
# 显形为 contract-consumer-unrecorded）；此外任何新增/消失=节点失败（消失项须可归因）
```

- **Step 4**：写 Brief（上述 (a)–(h)），README.md 追加「装配门面（K5.1）」段（五操作签名与 ib2 切换指针）。
- **产出 commit（2 笔，一任务内门面与 Brief 分离——docs 与 feat 不混提交，conventions §4）**：
  1. `feat(knowledge): passb b2-k-integration 门面五操作实装（K5.1）`（module.go + module_test.go + README 段）；
  2. `docs(passb): b2-k-integration 装配 Brief`（briefs/b2-k-integration.md）。
- **验收**：Step 1-3 命令输出（含退出码）摘录入报告；Brief (a)–(h) 齐备；`git diff "$ALIGN_SHA"...HEAD --name-only` 中不出现任何 `internal/router/**`、`internal/container/**`、`internal/bootstrap/**` 文件。

### Task K5.2 — 18 别名删除与例外/shim 收口核对记录【属主：b2-k-integration】

**业务目标**：把 §7.5 的 18 条 alias 义务收口删除（manifest+matrix 同 commit 成对删行），并产出例外 30 行与全部宿主过渡 shim 的收口核对记录（ib2 删除批的输入）。

**Step 1 — 逐条零 importer 复检（18 条）**：

```bash
python3 - <<'EOF'
import yaml
d = yaml.safe_load(open('docs/architecture/moves/knowledge.yaml'))
for a in d['alias_obligations']:
    print(a['old_import_path'])
EOF
# 对输出的每个 old_import_path 执行：
for p in internal/application/repository/retriever/doris internal/application/service/retriever internal/infrastructure/chunker internal/searchutil; do
  echo "== $p =="
  grep -rln "\"$p\"" --include="*.go" . | grep -v _test.go || echo "(zero non-test importer)"
done   # 实际执行时对全部 18 条逐一跑；2026-09-26 预检全部预期 (zero non-test importer)
ls internal/application/repository/retriever internal/infrastructure/chunker internal/searchutil 2>&1
# 预期：No such file or directory（旧路径物理目录不存在，Pass A 已搬）
```

**Step 2 — 成对删行（单一 commit）**：
- (a) `docs/architecture/moves/knowledge.yaml`：删除 alias_obligations 的 18 行（`internal/application/repository/retriever/*` 12 + `internal/application/service/retriever` 1 + `internal/infrastructure/{chunker,docparser,docparser/anydoc,semantic}` 4 + `internal/searchutil` 1）；
- (b) `docs/architecture/passb/ownership-matrix.yaml`（本分支副本）aliases 区：删除对应 18 行（`plan: 20-knowledge-program, delete_barrier: ib2`；k-process 分支实测 18 行在位）；
- (c) 机械缺口就地补齐（conventions §10）：`internal/modules/knowledge/docparser/anydoc/convert_linked_test.go:19` build 指令注释内 `internal/infrastructure/docparser` → `internal/modules/knowledge/docparser`（唯一文本残留，非 import；R1 `grep -n` 复核仍为 :19，漂移时按内容定位）；
- (d) 删行后复跑 `go run ./tools/passbguard -root .`——**预期退出码仍为 1**（218 条基线诊断不在本节点修复面，P-K5-7）；alias 双侧奇偶校验以 manifest↔matrix 行集对照为键、无 18 的字面计数断言（2026-09-26 于 check.go:167-206 实读确认；实测基线无 alias 类诊断，成对删行后奇偶保持）；以快照比对验收：`go run ./tools/passbguard -root . 2>&1 | grep -v '^exit status' | sort | diff - /tmp/k5-readiness-baseline.txt` 差异仍应恰为 P-K5-7 (ii) 的 3 条预登记行；出现其他差异=按 conventions §5 上报（若为纯计数断言显形则按 §8 机械修正 + 台账登记，独立 commit）。

**Step 3 — 例外 30 行与 shim 盘点（只读核对，产出记录）**：
- 例外：`grep -B4 "plan: 2[1-4]-knowledge" docs/architecture/passb/exception-ledger.yaml | grep -c "id: exc-"` = 30（exc-0088、0089-0091、0106-0111、0112-0116、0117-0129、0130-0131；§7.6 表）。逐行确认 `remove_at: ib2` 与 tools/architectureguard/check.go importExceptions 双侧一致（`go run ./tools/architectureguard` 零例外诊断）；登记「K5 不删行、ib2 删除批前置=airesource/policy 门面端口或 ADR 修订」（K1 Brief §10、K2 Brief §7 处置建议照录）；
- shim/compat 全清单（K5 盘点、ib2 删除）：K1 四件（`internal/application/repository/chunk_ingest_shim.go`、`internal/application/service/chunk_ingest_shim.go`、`internal/application/service/span_trace_seam_adapter.go`、`internal/handler/chunk_ingest_shim.go`）+ K2 四件（`kbretrieval_passb_compat.go`×3 + `kbretrieval_passb_compat_test.go` 垫片）+ K3 七件（W1 `wiki_k3_compat.go`、W2 `wiki_k3_ctor_compat.go`、W3 `wiki_k3_repo_compat.go`、H1 `wiki_page_k3_compat.go`、H2 `faq_k3_compat.go`、S1 `internal/handler/session/wiki_fixer_scope_compat.go`、D1 `knowledge_faq_k3_delegate.go`）+ K4 二件（`kbprocess_passb_compat.go` service/handler 各一）；每件登记「真源落位包 + 导出符号 + 残留 importer 计数（`grep -rln` 输出）」，残留应仅剩 ib2 改写项与推迟件；
- `container.go:36-38` 三行旧路径 import 改写申请：并入 K5.1 Brief (g)（不在本任务重复）。

**Step 4 — 证据落盘**：上述命令原文+退出码+18 条逐条结论写入 `docs/architecture/evidence/passb/b2-k-integration.md` §别名与 §例外/shim 章节。

- **产出 commit（2 笔）**：① `refactor(passb): b2-k-integration 删除 18 条知识别名义务行（manifest+matrix 同 commit）`（含 convert_linked_test.go 注释修正）；② `docs(passb): b2-k-integration 别名与 shim 收口核对记录`（evidence 章节）。
- **验收**：18 条逐条有结论（删除/残留+属主——预期全删）；passbguard 删行后绿；diff 仅含 §4 K5 可写清单文件；例外 30 行零改动（`git diff -- docs/architecture/passb/exception-ledger.yaml` 为空）。

### Task K5.3 — 差分汇总、节点门禁与收口证据【属主：b2-k-integration】

**Step 1 — K1-K4 差分证据复核**：`docs/architecture/evidence/passb/b2-k-{ingest,retrieval,wikifaq,process}.md` 四文件的差分章节齐备性核对（用例清单、双跑输出、比对结论、命令与退出码——conventions §6 四要素）；高风险四面（§10 表：分块索引写入+`knowledge.index.completed` / 索引清理+检索融合 / Wiki-FAQ 摄取与恢复 / 删除级联与重试幂等）逐一登记「已覆盖/随推迟批顺延（K4 Brief §8.3 口径）」；**缺项即节点不完成并上报（conventions §5）**。

**Step 2 — 节点 DAG gates 全量执行（不得以更快等价检查替代，conventions §2）**：

```bash
# PASSB_BASE_SHA 采用值裁定（Ruling 2026-09-24-WAVE-DEP-BASELINE 推论；R1 修订轮实测依据）：
#   merge-base origin/main HEAD 在本节点分支恒为 b1a3d6dd8（2026-09-26 实测——K0–K4 产物
#   经 P-K5-2 对齐 merge 进入本分支，不改变与 origin/main 的分叉点），直接套用 conventions
#   §1.2 缺省公式会使 diff 含 K0–K4 全部 210 个文件（实测：
#   git diff b1a3d6dd8...codex/passb-b2-k-process --name-only | wc -l = 210），
#   与 §4 K5 可写清单求差必非空——按 §12 #3 判据自设失败。
#   缺省采用值 = ALIGN_SHA（P-K5-2 对齐 merge commit；对齐产物视同本节点基线）。
#   协调者显式给定值仅当其为 ALIGN_SHA 或其后代（git merge-base --is-ancestor
#   "$ALIGN_SHA" "$GIVEN" 为真）时采用；否则记录差异并改用 ALIGN_SHA（报告注明原因）。
PASSB_BASE_SHA="$ALIGN_SHA"    # 记录采用值与来源（P-K5-2 登记；禁用 merge-base origin/main 公式，理由见上）
go build ./...                                        # 预期：退出码 0
go test -count=1 ./internal/modules/knowledge/...     # 预期：全 PASS（含 kbfreeze 2 + module 5 + K1-K4 随迁测试）
make check-backend-architecture                       # 预期：退出码 0；total=633（literal+apiKeyRoute+handle）、redis=23 lite=23、hooks=58、modules=16
make check-passb-readiness                            # 原样执行；预期退出码 1（P-K5-7 预裁定：基线 218 条既有诊断不在本节点修复面），退出码与全量输出如实入报告
make verify-module-moves                              # 预期：退出码 0（16 manifests verified）
go run ./tools/passbguard -root . 2>&1 | grep -v '^exit status' | sort | diff - /tmp/k5-readiness-baseline.txt
# 预期：diff 恰为 P-K5-7 (ii) 预登记的 3 条 module.go consumer-unrecorded 行；
# 其余任何差异=节点失败；消失项逐条归因于 K5 自身改动（预期消失集为空）
```

**Step 3 — 计数奇偶三方一致复核（conventions §8）**：`passbguard`/`architectureguard` 实测值 == `docs/architecture/evidence/pass-a-acceptance.md` 台账记录值 == `manifests/`（moves/*.yaml）发现值；633 路由 / 23+23 worker / 58 hook / 537 migration。18 knowledge worker 在 Redis/Lite 双栈各登记一次（K5.1 parity 测试 + architectureguard redis/lite 计数双证）。任何偏差走 §8 基线变更（台账+登记+独立 commit），禁止静默改断言。

**Step 4 — 证据与报告收口**：写 `docs/architecture/evidence/passb/b2-k-integration.md`（差分汇总、门禁命令台账、计数复核、K5.2 删除记录指针）与 `docs/plans/passb/reports/b2-k-integration.md`（执行命令台账、变更清单 vs owned_files 逐条核对、未完成项如实列出）；执行 conventions §1.2 报告包命令：

```bash
git diff --stat "$PASSB_BASE_SHA"...HEAD
git diff "$PASSB_BASE_SHA"...HEAD --name-only | sort    # 与 §4 K5 可写清单求差集，差集非空即失败
```

- **产出 commit**：`docs(passb): b2-k-integration 差分与门禁证据`。
- **验收**：五 gates 全绿（命令+退出码在报告）；差集为空；差分四要素与顺延登记齐备；DAG `b2-k-integration` 建议置 `review`（回填归协调者，conventions §9）。

---

## 9. K1-K4 子计划派发义务（子计划撰写者必须内嵌以下冻结约束）

> **属主：b2-k0 段落（历史记录；四子计划均已撰写并评审通过，义务已在 21/22/23/24 内嵌）。**

| 子计划 | 文件数 | 必须内嵌 |
|---|---:|---|
| `21-knowledge-ingest.md`（b2-k-ingest） | 9 | destination `internal/modules/knowledge/ingest`；组 D 两个符号 R1；组 A R2 seam；随迁测试逐文件判定；差分：TypeChunkExtract + `knowledge.index.completed`；例外 exc-0088 登记与 ib2 删除批 |
| `22-knowledge-retrieval.md`（b2-k-retrieval） | 29 | destination `internal/modules/knowledge/retrieval/app`；组 B 全部 R1/R2/R3；`escapeLikeKeyword` K2 侧 seam；`datasource_service_test.go` 不随迁（差分锚点）；差分：TypeIndexDelete + HybridSearch/融合/FAQ 混排；exc-0089/0090/0091 |
| `23-knowledge-wikifaq.md`（b2-k-wikifaq） | 18 | destination `internal/modules/knowledge/wiki`（12）与 `…/faq`（6）；组 B R2 seam；`wiki_fixer_scope.go` 去方法化裁定（B0.3 Step 3）；`recoverPendingWikiTasks` 行为不变；差分：TypeWikiIngest/TypeWikiFinalize/TypeFAQImport + hook 恢复等价 |
| `24-knowledge-process.md`（b2-k-process） | 28 | destination `internal/modules/knowledge/process`；组 C 全部 R1 与组 E seam；`knowledge_move_wiki_test.go` 随 K4；18 worker handler 实现交付（注册行禁改）；`knowledge_housekeeping.go` 独占清扫规则、System 经 `KnowledgeHousekeeping` 窄端口调度；差分：删除级联 + ProcessDocument 重试/幂等 + `knowledge.deletion.completed` |

共性义务：随迁全部 `_test.go`（framework:29）；白盒测试以定义文件属主为随迁归属；禁止双写/双注册/复制实现（framework:30）；每个搬迁 commit 按 M2/M3 分离（spec §13）；节点 gates = `go build ./...` + `go test ./internal/modules/knowledge/... -count=1` + `make check-backend-architecture` + `make verify-module-moves`。

---

## 10. 测试与高风险差分要求

**复用现有（contracts.yaml characterization_tests 登记；分两类处置）：**
- **随属主文件迁移（白盒/同属主）**：`embed_channel_chunk_test.go`、`knowledge_auto_tag_test.go`、`knowledge_post_process_wiki_enqueue_test.go`、`knowledge_replace_test.go`、`knowledge_write_access_test.go`、`handler/knowledge_mutation_admission_test.go`、`handler/rbac_lookups_test.go`、`knowledge_move_wiki_test.go`（随 K4）、`wiki_folder_prune_finalize_test.go`、`wiki_ingest_dedup_test.go`、`wiki_page_revision_test.go`（K3）、`handler/tag_delete_test.go`（K2）。
- **跨 plan 差分锚点（文件不迁移，双跑时引用）**：`agentruntime tools/scope_authorization_test.go` 与 wiki tools 测试 6 件、`datasource_service_test.go`。

**K5 新写**：`internal/modules/knowledge/module_test.go` 五组测试（§8 K5.1：缺失依赖拒绝、双栈 parity + 18 类型精确集 + 重复登记拒绝、handler 供给完整、Start 恢复挂点、Stop 对称）。**K5 复用**：kbfreeze 2 测试（邻接回归）。

**高风险差分（framework:40 knowledge deletion/indexing；spec §14.3 同输入双跑比对；证据入各节点 evidence 差分章节，K5.3 汇总复核）：**

| 面 | owner 节点 | 锚定用例（现状基线） |
|---|---|---|
| 分块索引写入与 `knowledge.index.completed` | K1 | `CreateChunks` 事件 producer 语义；`ChunkStatusIndexed` 可见性 |
| 索引清理 tag 侧与检索融合排序 | K2 | `TypeIndexDelete` 双栈行为；`HybridSearch` fanout/fusion/FAQ 混排 |
| Wiki/FAQ 摄取与恢复 | K3 | `TypeWikiIngest`/`TypeWikiFinalize`/`TypeFAQImport` 幂等；`recoverPendingWikiTasks` 重启恢复 |
| 删除级联与重试幂等 | K4 | `TypeKnowledgeListDelete`/`TypeIndexDelete`/`TypeKBDelete` 删除计划先行序；`knowledge.deletion.completed` 恰一次；asynq 重试语义零变化 |

**K5 门面装配面的行为等价声明（本修订轮新增）**：`RegisterWorkers` 的 18 类型→处理器映射与 task.go/sync_task.go 现行注册行**逐条同构**（§8 K5.1 推导表）；`Start` 与 container.go:1058 等价（幂等兜底）；路由与 worker 计数不变（K5.3 门禁 633/23+23/58 三方一致复核）。差分失败只修新实现，不得改期望值迎合（spec §14.3）。

---

## 11. 集成与回滚边界

- **集成边界**：K1-K4 分支在 K5 汇聚，一次一支、审后合并（framework:28），缺省序 K1→K2→K3→K4；K5 分支自身的基线对齐 merge（Ruling WAVE-DEP-BASELINE）不等于集成分支合并；装配切换（router/container/task/sync_task/bootstrap）仅集成工程师按 K5.1 Brief 串行执行；contracts.yaml knowledge 区状态回写仅 ib2（F2 status 字段未落盘——补落盘前 K5 不声称契约状态变更，K0.3 差异③裁定沿用）；宿主 shim 与跨 owner 宿主调用点改写仅 ib2。
- **回滚边界（spec §13）**：
  - K5.1 门面 commit（M4 预备态，纯新增装配面、零生产引用）：回滚 = `git revert` 对应 commit，无装配影响（ib2 切换前门面无消费者）；Brief/README 为 docs commit，独立回滚；
  - K5.2 别名删行 commit：回滚 = `git revert`（manifest 与 matrix 行随同 commit 恢复，passbguard 双侧奇偶随 revert 保持）；convert_linked_test.go 注释随同恢复；
  - K5.3 证据/报告为 docs commit，回滚无代码影响；
  - 本程序无 schema 变更、无 migration，回滚不需要数据修复；shim/例外删除（ib2）在任何残留 importer 未清零前禁止执行。
- **升级边界**：门禁不可能通过、或需改 §5/§6 冻结表、或 Brief 切换申请涉及冻结契约签名变化时，按 conventions §5 上报协调者（报告 + DAG notes），禁止现场改判所有权或扩大例外。「上游门面尚未落地」类阻塞（airesource/policy 门面之于例外删除）直接引用 Brief 前置登记，不自行实现上游门面。

---

## 12. 独立验收标准

**K0 节点（属主 b2-k0，已验收 approved）：**
1. DAG b2-k0 gates 三项全绿，命令原文+退出码记录于 `docs/plans/passb/reports/b2-k0.md`；
2. `git diff --name-only 8c45a8815...HEAD | sort` 与 §4 K0 可写清单差集为空；
3. §5 分配表与 `ownership-matrix.yaml` 84 行零漂移（计数 9/29/18/28）；
4. `go test -count=1 ./internal/modules/knowledge/kbfreeze/` PASS（2 测试）；
5. 治理文件零改动；差异①②③如实登记；计划评审 approved。

**K5 节点（属主 b2-k-integration，本修订轮细化）：**
1. §2 P-K5-1..P-K5-7 前置逐条实证在报告（任一不满足即 BLOCKED，不产出占位物）；
2. 五 gates 全部原样执行且命令原文+退出码在报告：`go build ./...`、`go test -count=1 ./internal/modules/knowledge/...`、`make check-backend-architecture`、`make verify-module-moves` 四项退出码必须为 0；`make check-passb-readiness` 按 P-K5-7 预裁定判据（预期退出码 1、终态快照差集恰为 3 条预登记行、无其他新增、消失项可归因）；
3. `git diff "$PASSB_BASE_SHA"...HEAD --name-only | sort`（$PASSB_BASE_SHA 采用值按 K5.3 Step 2 裁定，缺省=ALIGN_SHA；禁用 merge-base origin/main 公式）与 §4 K5 可写清单差集为空——**特别地：零 `internal/router/**`、`internal/container/**`、`internal/bootstrap/**`、`go.mod`、`go.sum` 文件出现**；
4. `module_test.go` 五测试 PASS + kbfreeze 2 测试 PASS（邻接回归）；
5. Brief (a)–(h) 齐备并评审 approved（reviewer 独立出 `docs/plans/passb/reviews/b2-k-integration.md`）；
6. 18 别名逐条结论在 evidence（预期全删：零 importer + 物理目录不存在双证）；passbguard 删行后快照差集仍恰为 P-K5-7 (ii) 的 3 条预登记行（奇偶保持，无其他新增/消失）；例外 30 行零改动；
7. K1-K4 差分证据四要素齐备性核对结论在 evidence（含顺延登记）；
8. 计数三方一致（633/23+23/58/537）复核输出在 evidence。

**程序级（IB2 前提）：** 84 legacy 文件全部搬迁且宿主删除完成至 matrix `delete_barrier: ib2` 口径（推迟件按 Ruling DEFERRED-FILE-SPLIT 登记、B5 的 396 口径不减免）；18 别名状态有逐条结论（K5.2 交付）；30 条属主例外删除或登记 ib2；§6 全部联动符号经 R1/R2 收敛且有差分证据；`go test ./internal/... -count=1` 于 ib2 全绿。

---

## 13. 计划自检记录（撰写时执行；含 2026-09-23 审校修复轮与 2026-09-26 K5 修订轮）

- **Spec 覆盖**：ask 十要素 → §1（Spec 指针）、§2（前置）、§4（文件与写权）、§5-§7（接口签名与冻结指针）、§10（行为兼容与差分）、§8（步骤/命令/预期）、§11（集成与回滚）、§7.5-7.6（删除义务）、§12（验收）——全覆盖。
- **无占位符**：K0 段落全部符号含 file:line 与真实签名（撰写与修复轮逐条 grep/读源码实证）；**K5 详案的全部签名、注册行、构造器、bootstrap 契约、例外 id、别名清单均于 `codex/passb-b2-k-process` HEAD（2026-09-26）逐条实读源码核对**（module.go 完整代码可直接编译目标；module_test.go 完整给出）。无 TBD。
- **类型一致性**：§5 表类型名/位置与 `internal/types/*.go` 实测一致；六 port 方法名清单转录自 contracts.yaml 冻结签名并核对接口源码；K5.1 `Dependencies` 字段类型 = `AsynqTaskParams`/`SyncTaskParams` knowledge 子集的 interfaces 类型逐一对应（task.go:40-49 实读）；`workerHandlers` 18 映射的方法名逐一核对 `interfaces/knowledge.go:206-223`、`tag.go:29-30`、`knowledgebase.go:132-138`、`task_handler.go:10`；`bootstrap.WorkerRegistry.Register/TaskTypes/VerifyWorkerParity` 签名实读 workers.go:50-95。
- **跨任务接口一致性**：§6 联动表 ↔ DAG b2-k-* notes 义务逐一对应；§7.6 与 exception-ledger 30 行一致（grep -B4 计数实测=30）；§7.1/§7.2 与 contracts.yaml knowledge.routes:1750/workers:1893 一致；K5.1 Brief (a)–(h) ↔ K1 Brief §6-§12 / K2 Brief §3-§8 / K3 Brief (a)–(e) / K4 Brief (a)–(i) 的删除批与切换项逐节对应；K5.2 shim 清单 ↔ 四 Brief 的 compat 文件清单一致。
- **测试语义实证（K5 修订轮新增）**：module_test.go 依赖的 Go 语义——「对非 nil 接口（动态类型嵌入 nil 接口）创建方法值不会 panic、仅调用时 panic」——已于 2026-09-26 以独立 `go test`（本仓 `internal/modules/knowledge` 包内临时用例，跑后删除）实证 PASS；bootstrap.Register 以 `any` 承载 handler 不调用，测试链路安全。
- **与上游冲突如实登记**：差异①（commercial 三符号未导出）、差异②（b0 两修复工单）、差异③（F2 status 字段未落盘）维持 K0.3 登记；K5 详案对差异③的处置=「补落盘前仅交付门面实装与 Brief，不声称契约状态变更」。b2-k-process 撰写时点仍 in_progress（P-K5-1 强制开工时点复核）。
- **审校修复轮（2026-09-23）已处置**（K0 属主，原文保留）：① chunker/splitter.go:26 Chunk 豁免；② knowledge_move_wiki_test.go 随迁归属 K3→K4；③ datasource_service_test.go 改差分锚点；④ §5 chunk.go 行号四处纠正；⑤ isValidFileType 撤销误派 shim；⑥ 组 B 调用点全量枚举；⑦ container.go 旧 import 坐标 :40-64→:36-38；⑧ framework 行号以集成副本为准；⑨ recoverPendingWikiTasks 挂接行 :1052→:1053（K3 后实测 :1058，§7.3 已注）；⑩ F2 status 未落盘登记差异③。
- **K0.1 复核轮（2026-09-23，实施者独立复核）**：§5/§6/§7 全部锚点逐项 grep 实测零漂移；84 行计数双副本一致；唯一计划修订=组 E `getParserEngineOverridesFromContext` 同名异义消歧 + ppc 修订工单上报（本程序不改 DAG）。
- **K5.1 误派根因分析轮（2026-09-24，根因分析员）**：K5.x 属主节点前置未满足时被派发两次均回 BLOCKED；本轮修订 §3 派发序声明与 §2 K5 前置；DAG 状态/notes 回填仍归协调者（conventions §9）。
- **K5 修订轮（2026-09-26，b2-k-integration 计划撰写员）**：① 实测基准声明（k-process HEAD 四祖先实测）；② §4 K5 可写/禁改清单落盘（module.go 写权=conventions §3「该模块集成节点」条 + DAG owned_files）；③ §8 K5.1–K5.3 由 K0 概要展开为可执行详案（门面五操作完整代码 + 五测试 + Brief 八节 + 18 别名成对删行 + 门禁/计数收口）；④ §7.5 预检结论（18 旧路径物理不存在 + import 零命中）；⑤ §7.6 例外清单更新为 30 行实数（K1-K4 显形例外并入）；⑥ RegisterRoutes 供给面口径的推导依据落盘（rbacGuards 包私有 + guard 扫描面 discovery.go:65-66 不含模块 + 禁双写）；⑦ 别名删除的行级权限依据（Ruling LEGACY-ROW-OWNERSHIP + matrix 行 plan: 20-knowledge-program + passbguard 奇偶校验实读 check.go:167-206）；⑧ 任务属主标注全文件贯穿（K0/K5 双节点同载文件的派发歧义防护）。
- **R1 审校修订轮（2026-09-26，b2-k-integration 计划撰写员；审校五条 findings 逐条实测处置）**：
  - **finding 1（critical）成立并扩展**：k-process HEAD `git archive` 至 /tmp 干净副本实跑 `go run ./tools/passbguard -root .` = **exit 1、218 条诊断**（85 contract-consumer-unrecorded / 42 contract-consumer-file-missing / 33 legacy-undeclared / 33 legacy-nonexistent / 13 contract-characterization-missing / 10 legacy-missing / event-producer-missing、event-consumer-missing 各 1，`grep -oE '^[a-z-]+:' | sort | uniq -c` 实测）；execution-dag.json 实读 b2-k-integration gates 确含 `make check-passb-readiness`（B2 knowledge 波五前置节点均无此 gate）→ 处置 = **P-K5-7 预裁定**（修复面全在 K5 禁改文件、归零属 ib2；节点判据=原样执行+如实记录+快照差集恰为 3 条预登记新增）。
  - **finding 2（important）成立**：`git merge-base origin/main codex/passb-b2-k-process` 与 `git merge-base origin/main codex/passb-b2-k-integration` 均 = b1a3d6dd825e3263e12b1daac2a52dab80ac5813；`git diff b1a3d6dd8...codex/passb-b2-k-process --name-only | wc -l` = **210** → 处置 = K5.3 Step 2 / §12 #3 / K5.1 验收改用 **ALIGN_SHA**（WAVE-DEP-BASELINE 推论：对齐产物视同本节点基线；协调者给定值须为 ALIGN_SHA 或其后代，否则记录差异改用 ALIGN_SHA）。
  - **finding 3（minor）成立**：`git merge-base --is-ancestor codex/passb-b2-k-process codex/passb-b2-k-integration` = **NO**（Case A 现状，merge 必产生独立 commit）；`git merge-tree --write-tree --name-only HEAD codex/passb-b2-k-process` 预演 = **仅 1 处冲突**（本计划文件 add/add；k-process 侧末次改动 `90530cac2`）→ 处置 = P-K5-2 Case A/B 互斥判据（Case B "Already up to date" 非失败，以 `--ancestry-path` 指认 merge commit）+ §3 时序互斥裁定（集成分支合并不触碰 K5 分支）。
  - **finding 4（minor）成立**：module.go 骨架 22 行 0 函数实读（:14 `RegisterRoutes(r RouteRegistrar)` / :16 `RegisterWorkers(mux WorkerRegistrar)`）；contracts.yaml knowledge.facade（:1613 区段）`signature: façade` 为占位值、items 仅冻结五操作名单 → 处置 = §8 K5.1「骨架示意签名差异处置」段（签名未冻结、K0 §5 授权 K5 推导、module.go 整文件重写）。
  - **finding 5（minor）复核不成立**：`git show codex/passb-b2-k-process:internal/modules/knowledge/docparser/anydoc/convert_linked_test.go | grep -n docparser` = **:19**（集成分支副本同），计划 :19 正确、审校「:19→:20」不采纳；§4 行加注复核结论。
  - **模拟实测（计划代码块逐字落盘至干净副本全量实跑）**：**真实缺陷一处已修**——module.go 原包注释 0 处满足 passbguard facade 形态契约（check.go:312-325 facadeOpRE + 三计数句式，`git show HEAD` 原版 grep 实证），按原注释跑 passbguard 新增 **8 条 contract-facade-{shape,count}-drift**；§8 K5.1 代码块包注释已按契约重写（11/18/1），修正后实测新增恰 3 条预登记 consumer-unrecorded、消失 0 条。**一处撤回**——首轮模拟曾报 module_test.go drift 断言 printf「4 动词 3 实参」vet 失败，经 `grep -n` 对照已提交计划原文为 3 动词 3 实参（合法），该失败系本轮模拟转录笔误而非计划缺陷，计划测试代码块保持原样；以计划原版测试行复跑 `go test ./internal/modules/knowledge/ -count=1 -v` = **exit 0、5 测试全 PASS**。其余实测：`go build ./...` exit 0；kbfreeze exit 0（2 测试）；`make check-backend-architecture` exit 0（**total=633：literal=564+apiKeyRoute=69**、redis=23 lite=23、hooks=58、modules=16）；`make verify-module-moves` exit 0（16 manifests verified）；基线 218 条中 knowledge.\* unrecorded 类已有 41 条（K1–K4 模块文件在列——3 条新增属同类形态）。
  - **bootstrap 契约行号修正**：WorkerRegistry workers.go:12 / WorkerSink.RegisterTaskHandler :21-22 / NewWorkerRegistry :28 / Register :39（already-registered 错误语义=测试重复登记拒绝断言依据）/ TaskTypes :57 / VerifyWorkerParity :70（原稿 :50-52/:74-95 不准）。
  - **P-K5-5 复核**：Makefile 三目标实读 :250（verify-module-moves）/ :255（check-backend-architecture）/ :262（check-passb-readiness），计划行号正确。
- **K5.1 任务级 OCR 根因分析轮（2026-09-26，根因分析员；两跑不完整后 replan，本条为重试指引的事实源）**：
  - **失败形态（实测留档）**：`ocr-r1.txt`（14:58）"Review partially complete: 13 finding(s); **22 of 80** selected item(s) failed"；`ocr-r1-a2.txt`（15:21）"Review partially complete: 10 finding(s); **40 of 80** selected item(s) failed"——第 2 跑劣于第 1 跑。两跑重试报告均显示几乎所有批次 LLM 请求遭 HTTP 429 限流（第 1 跑 42/181 请求受影响、3 个永久失败；第 2 跑 21/167 受影响、12 个永久失败，多批次 6 连 429 后放弃）。
  - **根因裁定**：①**外部诱因**=OCR LLM 供给方持续限流（同日本仓先例：b2-k-process 节点级 OCR 03:20/03:35 两跑不完整→03:39 BLOCKED→11:15 复跑完整通过并 0/0 裁定——限流窗口是时间性的，非永久阻断）；②**放大器（可处置部分）**=审区选区 80 项取自 157 文件审查包 `K5.1-review-pkg.md`（区间 BASE c30cb90ee→HEAD a623cef55，本会话 `git diff --name-only | wc -l`=157），其中 **K5.1 净改动仅 4 文件**（ALIGN_SHA b9c09f524..HEAD：module.go/module_test.go/README.md/briefs/b2-k-integration.md），其余 153 文件全部是 P-K5-2 基线对齐 merge 带入的 K1–K4 终态产物——四前置节点均已 done+approved 且各自拥有完整 OCR/审查链，ocr-context.md §2 明示「勿计入 K5.1 findings」；海量非本任务文件把 LLM 请求数推高至必然长时间暴露于限流窗。
  - **本轮修订①（缺陷溯源修复）**：§8 K5.1 代码块 Dependencies「其余 5 组 handler」计数错误勘误（7+5=12 与 §7.1 冻结 11 项矛盾；正确口径=其余 **4 项**：3 组宿主 handler + serveKBScopedFiles 文件服务面，与 Brief (a)「7 即时切换 + 3 随推迟件 + 1 零动作」分解一致）。该错误系 ocr-r1-a2 唯一落在 K5.1 净面上的 finding（documentation·medium，module.go:64-65）经本会话 grep 实证并溯源至本计划代码块 ：371（实施者逐字转录所致）。**module.go:64-65 同句勘误归 OCR 修复轮执行（本轮不实施代码）**；修正安全性依据：passbguard facade 形态契约（check.go:312-326 facadeOpRE/三计数句正则）只扫 module.go 包注释，此为 struct 字段注释、不匹配任何冻结正则，修正零 drift 风险。
  - **本轮修订②（重试指引，约束后续 OCR 轮）**：任务级 OCR 重试选区**限定 K5.1 净改动 4 文件**（b9c09f524..a623cef55），排除对齐产物 153 文件（已由各属主节点 OCR 链覆盖，重复审查无信息增益且是本次限流失败的放大器）；重试须避开限流窗——第 2 跑（15:21）劣于第 1 跑（14:58）证明背靠背重试无效，建议间隔 ≥2 小时或观察供给方恢复后再试；选区缩至 4 文件后请求数降至个位数，配合 ocr 内建重试可大幅提高完整输出概率。两跑已提取的 K1–K4 文件 findings（ingest/wiki/process 等）全部不在 K5.1 义务面（属各属主节点/ib2），不得据此判 K5.1 失败或扩大 K5.1 写权去修。
  - **本轮修订③（§4 可写清单补登）**：新增「本文件（限 K5 属主段落的计划修订轮 docs-only commit）」行——本轮 replan commit（37b081b7e）与既有先例（0805a084a/c30cb90ee，两者位于 ALIGN_SHA 之前故未触发）不同、落在 ALIGN_SHA..HEAD 区间内，若不补登则 K5.3 Step 2 / §12 #3 差集检查会因治理性计划修订而自设失败；补登依据=DAG `plan_path` 属主与头注 :5-7（本文件即本节点计划文档）。
  - **判定**：replan（非 blocked）——K5.1 实施本体已完成且五 gates 输出在案（K5.1-report §4/§7：净 diff 恰 4 文件、与 §4 可写清单差集为空）；失败面在审查步骤，其中可处置部分（选区缩窄）本轮已裁定、唯一净面 finding 已溯源并指定修复归属，不可控部分（供给方限流）有时间性解除先例。
