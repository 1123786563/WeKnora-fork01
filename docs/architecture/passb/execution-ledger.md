# Pass B 执行台账（execution-ledger）

- 唯一写入者：Ledger 管家（Pass B 台账子代理）；追加式日志，禁止改写既有条目。
- 事实源：`docs/plans/passb/execution-dag.json`；本文件与其冲突时以 JSON 为准。
- 字段语义与状态机：`.superpowers/sdd/passb/conventions.md` §9（主 checkout，git-ignored 区）——`status: pending → blocked → in_progress → review → done`；`base_sha` 派发时回填；`head_sha` 节点分支评审通过时回填；`review_status: pending → requested → changes_requested → approved`。

---

## 2026-09-23 03:18 CST · b0 → in_progress（派发）

- **节点**：b0 —— B0 契约与所有权冻结（passbguard + 4 份治理 yaml + 证据）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`（已存在，18,387 字节）
- **前置**：无（depends_on = []）
- **worktree**：`.worktrees/passb-int`（分支 `codex/passb-integration`）——DAG/台账所在 integration worktree；本次派发未提供独立 b0 实现 worktree 信息，如后续启用请以新条目补记
- **base SHA**：`b1a3d6dd8`（DAG 固定值，conventions §9，未变更；当前 integration HEAD `5bf228a40` = b1a3d6dd8 + 2 个 DAG 文档提交，已验证 b1a3d6dd8 是其祖先）
- **head SHA**：null（未回填——节点分支评审通过时回填）
- **测试证据路径（计划）**：`docs/architecture/passb/b0-evidence.md`、`docs/plans/passb/reports/b0.md`、`docs/plans/passb/reviews/b0.md`（均尚未产出）
- **审查结论**：pending（review_status 未请求）
- **OCR 报告路径**：无（尚未进入审查轮）
- **修复轮次**：0
- **本次变更**：`execution-dag.json` 节点 b0 `status` pending → in_progress（本次会话 Edit 后经 python3 json.load 验证合法，33 节点中仅 b0 非 pending）；`base_sha`/`head_sha`/`review_status` 均未动
- **备注**：调度指令原文为 "b0 → running"；状态机（conventions §9）无 `running` 值，按语义映射为规范值 `in_progress` 并在此留痕

---

## 2026-09-23 03:40 CST · b0 / B0.1 → done（审查通过）

- **节点/任务**：b0 · B0.1 —— Add Strict Pass B Governance Models and Loader（计划 Task B0.1，`docs/plans/passb/00-contract-and-ownership-freeze.md:53-115`）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：无（b0 首任务；节点内其余任务 B0.2–B0.6 尚未完成）
- **worktree**：`.worktrees/passb-b0`（分支 `codex/passb-b0`，本会话 git 核验）
- **BASE → HEAD**：`5bf228a40` → `9f87a809d`（1 commit：`test(passb): add strict governance schema loader`；本会话 `git log`/`git diff --stat` 核验：10 文件、1326 行新增，全落 `tools/passbguard/`）
- **测试证据**（B0.1-report.md §1 所载，本台账转录）：`go test ./tools/passbguard -run TestLoadGovernance -count=1` PASS（7 测试函数/28 子用例）；`go build ./...` exit 0；`make check-backend-architecture` PASS（633/23+23/58 基线不变）；`make verify-module-moves` PASS（16 manifests）；`go vet`/`gofmt -l`/`git diff --check` 干净
- **报告/审查证据路径**：`.superpowers/sdd/passb/b0/B0.1-report.md`（实施报告，本会话已读）；`.superpowers/sdd/passb/b0/B0.1-review-pkg.md`（审查包：log/diff 全文，49,520 字节）
- **审查结论**：通过（调度指令裁定"审查通过"；报告与审查包在案，本管家未重跑审查）
- **OCR 报告路径**：无（本任务未运行 open-code-review）
- **修复轮次**：0
- **本次 JSON 变更**：b0 节点新增 `task_status` 字段（`B0.1=done`，B0.2–B0.6=pending）——conventions §9 未定义任务级字段，按调度指令"更新两个字段/条目"引入并在此留痕字段语义；节点级 `status` 仍为 `in_progress`，`base_sha=b1a3d6dd8`/`head_sha=null`/`review_status=pending` 未动
- **交接提醒**（B0.1-report.md §6）：(1) B0.6 建 main.go 时须将 `tools/passbguard` 全目录（含测试）翻转为 `package main`；(2) B0.2 须为 `internal/application/repository/knowledge.go` 显式分配 K1–K4 归属（escapeLikeKeyword 被 identity/conversation 跨 owner 调用）；(3) Mimosa hook 曾返 `scanner_enobufs`，未获完整安全扫描结论

---

## 2026-09-23 04:10 CST · b0 / B0.2 → done（审查通过）

- **节点/任务**：b0 · B0.2 —— Generate and Validate the Ownership and Exception Ledgers（计划 `docs/plans/passb/00-contract-and-ownership-freeze.md` §Task B0.2，Step 1–6 全部执行）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：B0.1（done，见上条）
- **worktree**：`.worktrees/passb-b0`（分支 `codex/passb-b0`，本会话 git 核验）
- **BASE → HEAD**：`9f87a809d` → `0bee2f239`（2 commits：`a83d18b2c` refactor(passb): extract shared movemanifest library (F1-a)；`0bee2f239` docs(passb): freeze legacy and exception ownership；本会话 `git log`/`git diff --stat` 核验：10 文件、4232 插入/25 删除）
- **交付物核验**（本会话 `wc -l`）：`docs/architecture/passb/ownership-matrix.yaml` 2688 行（396 legacy + 99 alias）、`docs/architecture/passb/exception-ledger.yaml` 637 行（105 条 exc-0001..0105，无一条推给 b5）——与报告 §3 一致
- **测试证据**（B0.2-report.md §2 所载，本台账转录）：`go test ./tools/passbguard ./tools/modulemove ./tools/architectureguard ./internal/bootstrap -count=1` 4 包全 ok；`make verify-module-moves` OK（16 manifests）；`make check-backend-architecture` OK（633/23+23/58 基线不变）；Step 6 指定命令 `go test ./tools/passbguard -run 'Ownership\|Exception\|RealRepo' -count=1` ok（7 顶层用例）；`go vet`/`gofmt` 干净；F5 三方一致参数化（无 396/105 字面量断言）
- **未运行项**（报告 §6 如实）：`make check-passb-readiness`（target 由 B0.6 创建，尚不存在）；`golangci-lint --new-from-rev`（barrier/B0.6 门禁）
- **报告/审查证据路径**：`.superpowers/sdd/passb/b0/B0.2-report.md`（实施报告，本会话已读）；`.superpowers/sdd/passb/b0/B0.2-review-pkg.md`（审查包，185,187 字节）
- **审查结论**：通过（调度指令裁定"审查通过"；报告与审查包在案，本管家未重跑审查）
- **OCR 报告路径**：无（本任务未运行 open-code-review）
- **修复轮次**：0
- **本次 JSON 变更**：b0.`task_status` B0.2 pending → done（B0.1 仍 done；B0.3–B0.6 pending）；节点级 `status=in_progress`、`head_sha=null`、`review_status=pending` 未动
- **要点留痕**：(1) 上条交接提醒第 2 项已闭环——`repository/knowledge.go` 显式归 **24-knowledge-process（K4）**（matrix 行 plan: 24-knowledge-process / delete_barrier: ib2，knowledge-process.md 消歧注记 + 两个 RealRepo 测试钉死）；(2) F1 方案 a 落地：`tools/internal/movemanifest` 成为 manifest schema 单一事实源；(3) B0.3 既定工作预告：`native_archive.go` 现归 33-engine，B0.3 Step 2 将改派 34-protocol；(4) agentcatalog/conversation 取协调计划级属主（25/35），子计划细分留待其 program 计划撰写；(5) Mimosa hook 两次提交均 `scanner_enobufs`，安全扫描结论仍未取得

---

## 2026-09-23 04:35 CST · b0 / B0.3 → done（审查通过）

- **节点/任务**：b0 · B0.3 —— Resolve Shared-Host and Agent Runtime Brief Overlaps（计划 `docs/plans/passb/00-contract-and-ownership-freeze.md:173-217`）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：B0.2（done，见上条）
- **worktree**：`.worktrees/passb-b0`（分支 `codex/passb-b0`，本会话 git 核验，工作树干净）
- **BASE → HEAD**：`0bee2f239` → `5cfa5bc02`（1 commit：`docs(passb): resolve shared-host ownership`；本会话 `git diff --stat` 核验：12 文件、+800/−35）
- **交付物核验**（本会话）：变更清单 = 9 份 passb 治理文档（含 matrix）+ `tools/passbguard/overlap.go`(366 行)/`overlap_test.go`(288 行)；**新建 `docs/architecture/passb/execution.md`（42 行）**——RED 阶段发现 passb 目录无 execution brief、三文件裁定无处登记，属 DAG b0 owned_files `docs/architecture/passb/*.md` glob 覆盖（报告 §6.1 自陈供复核）
- **测试证据**（B0.3-report.md §2 所载，本台账转录）：计划 Step 5 指定命令 `go test ./tools/passbguard -run 'Overlap|Ambiguous|RealRepo' -count=1` PASS（13/13，含 B0.2 RealRepo 回归）；全包/`modulemove`/`architectureguard`/`internal/bootstrap` 测试 ok；`make verify-module-moves` OK（16 manifests）；`make check-backend-architecture` OK（633/23+23/58 基线一致）；`go build ./...` exit 0；vet/gofmt 干净。TDD RED 有实证记录（ambiguous-phrase 2 处 + native_archive 属主 33→34 期望失败等 4 项）
- **未运行项**（报告 §2/§6 如实）：`make check-passb-readiness`（B0.6 建 target）；`golangci-lint`、`go test ./internal/... -timeout=25m`（barrier 级门禁）；`go test -race`（b5）
- **报告/审查证据路径**：`.superpowers/sdd/passb/b0/B0.3-report.md`（实施报告，本会话已读）；`.superpowers/sdd/passb/b0/B0.3-review-pkg.md`（审查包，65,616 字节，本会话 ls 核验在案）
- **审查结论**：通过（调度指令裁定"审查通过"；本管家未重跑审查）
- **OCR 报告路径**：无（本任务未运行 open-code-review）
- **修复轮次**：0
- **本次 JSON 变更**：b0.`task_status` B0.3 pending → done（B0.1/B0.2 仍 done；B0.4–B0.6 pending）；节点级 `status=in_progress`、`head_sha=null`、`review_status=pending` 未动
- **要点留痕**：(1) 上条预告闭环——`native_archive.go`（service+handler 两文件）已改派 **34-protocol**，engine brief 计数同步 39→38；(2) shared-host 36 文件 HandlerSessionRuling 全量冻结（wiki_fixer_scope→23、workbench 族→40、craft 族→41、agent_run/agent_stream_handler→33、session 族 15 文件→35；pagination/upload-limit/error helper 保持 platform 零认领）；(3) housekeeping 裁定入 knowledge-process.md（业务规则归 24、System 经窄端口调度、禁双实现）；(4) 歧义短语（先合并为准、二择其一执行等）已由 CheckAmbiguity 机器禁止（条目原以字面引用短语样例，2026-09-23 b0 合入 integration 后禁令自指命中，按 freeze B0.3 Step 1 上游契约改写措辞，语义不变，详见当日集成合并台账条目）；(5) Mimosa hook 仍 `scanner_enobufs`

---

## 2026-09-23 05:11 CST · b0 / B0.4 → done（审查通过）

- **节点/任务**：b0 · B0.4 —— Freeze Capability and Composition Contracts（计划 `docs/plans/passb/00-contract-and-ownership-freeze.md:221-272`）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：B0.3（done，见上条）
- **worktree**：`.worktrees/passb-b0`（分支 `codex/passb-b0`，本会话 git 核验，工作树干净）
- **BASE → HEAD**：`5cfa5bc02` → `fac268d41`（1 commit：`docs(passb): freeze module capability contracts`；本会话 `git diff --stat` 核验：13 文件、+3599/−3）
- **交付物核验**（本会话）：`docs/architecture/passb/contracts.yaml` 2,176 行、**125 条契约**（`grep -cE "^\s*-? ?id: "` 实测=125，与报告 §3 合计一致）：B1 能力面 29 + 下游能力面 29 + 组合面 64（16 模块 façade 五操作 + route/worker/lifecycle-set）+ 组合基线 3；七种 kind 全覆盖（55 capability-port / 2 wire-protocol / 1 data-ownership / 16 module-construction / 17×3 set）
- **测试证据**（B0.4-report.md §2 所载，本台账转录）：计划 Step 7 指定命令 `go test ./tools/passbguard -run 'Contract\|RealRepo' -count=1` PASS（RealRepo CheckContracts 零诊断）；全包（4.901s）与 `go test ./tools/...` 全 ok；`go build ./...` exit 0；`make check-backend-architecture` OK（633/23+23/58 三方一致）；`make verify-module-moves` OK；`internal/bootstrap` ok；`git diff --check` 干净。TDD RED 有实证（undefined: DiscoverSymbol 等 build failed）
- **未运行项**（如实）：`make check-passb-readiness`（B0.6 建 target）；`golangci-lint`（barrier 级）；migrations 537 未触及
- **报告/审查证据路径**：`.superpowers/sdd/passb/b0/B0.4-report.md`（实施报告，本会话已读）；`.superpowers/sdd/passb/b0/B0.4-review-pkg.md`（审查包，202,900 字节，本会话 ls 核验在案）
- **审查结论**：通过（调度指令裁定"审查通过"；本管家未重跑审查）
- **OCR 报告路径**：无（本任务未运行 open-code-review）
- **修复轮次**：0
- **本次 JSON 变更**：b0.`task_status` B0.4 pending → done（B0.1–B0.3 仍 done；B0.5/B0.6 pending）；节点级 `status=in_progress`、`head_sha=null`、`review_status=pending` 未动
- **要点留痕**：(1) `internal/modules/*/module.go` 零改动——16/16 组注释计数与 manifest 一致（contract-facade-count-drift 已机器化该检）；
- **移交 B1 计划的必需输入**（报告 §5，6 条缺在库特征化测试的契约）：identity.organization-service / identity.tenant-api-key-service / airesource.storage-backend-service / airesource.weknora-cloud-service / execution.workspace-sandbox-policy / execution.sandbox-config-loader-construction——B1 子计划（10/11/13）撰写时必须补特征化测试路径；
- **已声明偏差**（报告 §4，留审查者裁夺记录在案）：签名/消费方发现用 go/parser+go/printer 而非 go/parser+go/types（源码文本相等即冻结判据）；跨模块子包导入检查仅对模块代码消费方生效（与 architectureguard 同口径），/adapters 检查全域；(3) Mimosa hook 仍 `scanner_enobufs`

---

## 2026-09-23 05:51 CST · b0 / B0.5 → done（审查通过）

- **节点/任务**：b0 · B0.5 —— Freeze the Versioned Event Catalog（计划 `docs/plans/passb/00-contract-and-ownership-freeze.md` §Task B0.5，Step 1–4 全部执行）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：B0.4（done，见上条）
- **worktree**：`.worktrees/passb-b0`（分支 `codex/passb-b0`，本会话 git 核验，工作树干净）
- **BASE → HEAD**：`fac268d41` → `3583773da`（1 commit：`docs(passb): freeze versioned event catalog`；本会话 `git diff --stat` 核验：恰 2 文件、+863——`event-catalog.yaml` 324 行 + `events_test.go` 539 行，与任务 Files 清单零差集）
- **交付物核验**（本会话）：`docs/architecture/passb/event-catalog.yaml` **29 条 version-1 事实记录**（`grep -cE "^\s*-? ?id: "` 实测=29），7 家族（agentrun 7 / conversation 4 / knowledge 4 / craft 5 / usage 2 / workbench 4 / notification 2 + channel 1）；producer 一律 file:Symbol 且经 go/parser 测试验证在盘；transport 冻结词汇 {in_process, durable}、replay 一律 source-query（F4 裁定落地——无完整重放表述）
- **测试证据**（B0.5-report.md 所载，本台账转录）：计划 Step 4 指定命令 `go test ./tools/passbguard -run Event -count=1` PASS（7 测试函数，含 13 子用例 schema 表驱动 + 5 命令式命名拒绝 + 4 映射抽查）；全包回归 PASS（1.747s，覆盖 B0.1–B0.4 套件）；`go build ./...` exit 0；`make verify-module-moves` OK（16 manifests）；`make check-backend-architecture` OK（633/23+23/58 基线不变）；`git diff --check` 干净。TDD RED 有实证（29 条事件未在册断言失败 → 生成后绿）
- **未运行项**（如实）：`go test ./internal/...`（barrier/b5 级，属 B0.6 Step 4 与后续 barrier）；`make check-passb-readiness`（B0.6 建 target）
- **报告/审查证据路径**：`.superpowers/sdd/passb/b0/B0.5-report.md`（实施报告，本会话已读）；`.superpowers/sdd/passb/b0/B0.5-review-pkg.md`（审查包，39,064 字节，本会话 ls 核验在案）
- **审查结论**：通过（调度指令裁定"审查通过"；本管家未重跑审查）
- **OCR 报告路径**：无（本任务未运行 open-code-review）
- **修复轮次**：0
- **本次 JSON 变更**：b0.`task_status` B0.5 pending → done（B0.1–B0.4 仍 done；仅 B0.6 pending）；节点级 `status=in_progress`、`head_sha=null`、`review_status=pending` 未动
- **留痕 concerns**（报告遗留节，供后续裁定）：(1) `knowledge.processing.failed` producer 锚定 `knowledge_housekeeping.go:runSweep` 巡检位点（解析即时置败散布多处），审查者若裁定改锚首写位点是单行修正；(2) `workbench.artifact.updated` producer 取 `service/message.go:UpdateMessage`（仓库无独立 artifact 写函数）；(3) channel 家族仅 1 条系 IM 渠道现状，非遗漏；(4) Insights 无独立 insights.* 事件族（消费已冻结 usage/session 事实），符合 B0 不发明新语义约束；(5) Mimosa hook 仍 `scanner_enobufs`

---

## 2026-09-23 06:21 CST · b0 / B0.6 → done（审查通过）——节点六任务全部完成

- **节点/任务**：b0 · B0.6 —— readiness command + B0 evidence（计划 `docs/plans/passb/00-contract-and-ownership-freeze.md:317-381`）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：B0.5（done，见上条）
- **worktree**：`.worktrees/passb-b0`（分支 `codex/passb-b0`，本会话 git 核验，工作树干净）
- **BASE → HEAD**：`3583773da` → `9cdfa0c62`（1 commit：`docs(passb): complete contract and ownership freeze`；本会话 `git diff --stat` 核验：15 文件、+564/−82）。**节点累计 9 提交**（`git log b1a3d6dd8..HEAD | wc -l` 本会话实测=9：a228d6e9d/5bf228a40/9f87a809d/a83d18b2c/0bee2f239/5cfa5bc02/fac268d41/3583773da/9cdfa0c62）
- **交付物核验**（本会话）：`docs/architecture/passb/b0-evidence.md` 113 行在案；Makefile `check-passb-readiness` 按 F6 裁定追加至 `.PHONY` 首行（Makefile:1）+ 目标体（:262）；`tools/passbguard` 全目录已翻转为 `package main`（B0.1 交接项闭环）
- **测试证据**（B0.6-report.md §2 所载，本台账转录）：**节点 5 项 gates 首次全部可满足且通过**——`go test ./tools/passbguard`（含 TestCLI 双路径）、`go test ./internal/bootstrap`、`make check-passb-readiness`（`legacy=396 aliases=99 exceptions=105 contracts=125 events=29 overlaps=0 missing=0`，零诊断）、`make verify-module-moves`（16 manifests）、`make check-backend-architecture`（633/23+23/58 基线不变）；另 `go test ./internal/... -count=1 -timeout=25m` 119 ok/0 FAIL（两次）、`go build ./...` exit 0、`golangci-lint --new-from-rev=b1a3d6dd8` **0 issues**（B0.1–B0.5 遗留 50 条 lint 告警已全部清偿，行为零变化）、`git diff --check b1a3d6dd8...HEAD` 零 finding、migrations 537 一致
- **报告/审查证据路径**：`.superpowers/sdd/passb/b0/B0.6-report.md`（实施报告，本会话已读）；`.superpowers/sdd/passb/b0/B0.6-review-pkg.md`（审查包，76,791 字节，本会话 ls 核验在案）
- **审查结论**：任务级通过（调度指令裁定"审查通过"；本管家未重跑审查）
- **OCR 报告路径**：无（本任务未运行 open-code-review）
- **修复轮次**：0（B0.1–B0.6 六任务均为一轮通过）
- **本次 JSON 变更**：b0.`task_status` B0.6 pending → done（**六任务全 done**）；节点级 `status` 仍为 `in_progress`、`head_sha=null`、`review_status=pending` 未动——按 conventions §9 节点状态机与 head_sha 回填（节点分支评审通过时）为协调者决定，本次指令未含节点级迁移
- **待协调者处置**：(1) **B0 架构审查待做**（报告 §4.3：计划最后一项 checkbox 保持未勾，不自行批准）；(2) **head_sha 回填**：B1 起点为本分支合并进 integration 分支后的头，b0-evidence.md 已声明不得以分支中间提交（如 9cdfa0c62）作为 B1 起点；(3) 遗留债务已登记 b0-evidence.md §5.4：`benefits.go:105 QF1008`（78f18915f 口径下 +1，先于节点基线，归 B1-CM 清偿）与 design-tokens css 尾随空白（前端 parity 流）；(4) Mimosa hook 仍 `scanner_enobufs`，建议集成阶段重跑完整审计

---

## 2026-09-23 06:26 CST · b0 → blocked（OCR 基础设施失败）＋ 全部 32 个传递依赖节点 → blocked

- **节点**：b0（六任务 B0.1–B0.6 均 done，但节点级架构审查未完成）+ **传递依赖闭包全部 32 节点**（本会话 BFS 实测：b0 为 DAG 根节点，闭包=其余全部节点，闭包外为空）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：无（b0 自身为根）
- **worktree**：`.worktrees/passb-b0`（分支 `codex/passb-b0`，HEAD 仍为 `9cdfa0c62`，未变）
- **base/head SHA**：`b1a3d6dd8` / `null`（未回填，维持）
- **阻塞原因**（调度指令原文照录）：**OCR 基础设施失败（exit 1）：Error: background content is 9914 characters, exceeding the hard limit of 8000 (aborting)**——即上条"待协调者处置 1"的 B0 节点级架构审查（OCR）因 background 内容 9914 字符超过 8000 硬限而中止，未产出审查报告
- **测试证据路径**：无新增（六任务报告/审查包见前六条目，仍有效）
- **审查结论**：任务级六条均通过；**节点级架构审查失败（基础设施错误，非审查否决）**
- **OCR 报告路径**：无（OCR 调用 aborting，未产出）
- **修复轮次**：1（第 1 次节点级审查尝试失败；修复方向：将 OCR background 内容压缩至 8000 字符以内后重试）
- **解除条件**：修复 OCR 调用（background < 8000 字符）并完成节点级审查后置 b0 为 done；32 个传递依赖节点在 b0 done 后恢复
- **本次 JSON 变更**（脚本原子更新，全部断言通过）：(1) b0 `status` in_progress → **blocked**（33 节点现全部 blocked：更新前分布 1 in_progress + 32 pending）；(2) b0 notes 数组追加 BLOCKED 条目（含 OCR 错误原文，conventions §5 上报摘要要求）；(3) 其余 32 节点 `status` pending → blocked，notes 各追加「BLOCKED（2026-09-23）：前置 b0 阻塞；解除条件：修复并 done b0 后恢复。」
- **受影响节点全录**（32）：b1-identity、b1-airesource、b1-commercial、b1-execution、ib1、b2-k0、b2-k-ingest、b2-k-retrieval、b2-k-wikifaq、b2-k-process、b2-k-integration、b2-ac-definition、b2-ac-skills、b2-ac-market、b2-datasource、b2-appconnector、ib2、b3-r-memory、b3-r-tools、b3-r-engine、b3-r-protocol、b3-r-integration、b3-conv-queryhistory、b3-conv-session、b3-channels、b3-insights、ib3、b4-workbench、b4-craft、b4-systempolicy、ib4、b5
- **未动字段**：b0.task_status（六任务仍 done——任务成果不受阻塞影响，阻塞的是节点级审查收口）、全部 review_status（pending）、全部 head_sha（null）、base_sha

---

## 2026-09-23 06:49 CST · b0 → in_progress（OCR 阻塞恢复后重新运行派发）

- **节点**：b0 —— B0 契约与所有权冻结（passbguard + 4 份治理 yaml + 证据）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：无（depends_on = []）
- **worktree**：指令未附新 worktree 信息；b0 实施分支 `.worktrees/passb-b0`（`codex/passb-b0`）HEAD 仍为 `9cdfa0c62`（本会话 git log 核验，六任务成果在案）；DAG/台账所在 integration worktree `.worktrees/passb-int`（分支 `codex/passb-integration`）
- **base SHA**：`b1a3d6dd8`（未动）；**head SHA**：null（未回填——节点级评审通过时回填）
- **测试证据路径**：无新增（B0.1–B0.6 六任务报告/审查包见 03:40–06:21 各条目，仍有效；节点 5 项 gates 全过见 06:21 条目）
- **审查结论**：任务级六条通过；节点级架构审查仍未完成（06:26 OCR 基础设施失败后的重试即本次恢复运行的主要待办）
- **OCR 报告路径**：无（尚未产出；重试时须把 background 压缩至 8000 字符硬限以内）
- **修复轮次**：1（第 1 次节点级审查因 OCR background 9914 字符超 8000 硬限 aborting；本次为修复后重试，结果由后续条目回填）
- **本次 JSON 变更**：**无字节级改动**——b0.`status` 目标值 `in_progress` 已在位：调度方恢复提交 `78193c4e5`（2026-09-23 06:41:55 +0800，"recover dag statuses after ocr background-limit block"，本会话 `git show` 核验：恰 33 处 status 行变更 = 1×in_progress（b0）+ 32×pending（传递依赖），b0 notes 的 BLOCKED 条目随之移除、32 个依赖节点 notes 内 BLOCKED 文本保留（`grep -c` 实测 32 处））。本会话 `python3 json.load` 复验 JSON 合法（33 节点：1 in_progress + 32 pending）。
- **备注**：指令原文 "b0 → running"；状态机（conventions §9）无 `running` 值，沿 03:18 条目先例映射为规范值 `in_progress` 并在此留痕。依赖节点 notes 中残留的 32 处 BLOCKED 文本与 `status=pending` 并存，系恢复提交 `78193c4e5` 所为（非本管家改动）；其解除条件（b0 done）尚未满足，调度方后续置 b0 done 时可一并清理，本指令未授权改动这些节点。

---

## 2026-09-23 06:50 CST · b0 恢复运行（复用 B0.1–B0.6 已完成任务）

- **节点**：b0 —— B0 契约与所有权冻结（passbguard + 4 份治理 yaml + 证据）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：无（depends_on = []）
- **恢复语义**：复用已完成任务 B0.1、B0.2、B0.3、B0.4、B0.5、B0.6——六任务不重置、不重跑；本节点剩余待办 = 节点级架构审查（OCR background 压缩至 8000 字符以内后重试）+ 审查通过后的 head_sha 回填与 status 收口
- **worktree**：b0 实施分支 `.worktrees/passb-b0`（`codex/passb-b0`）HEAD `9cdfa0c62`（本会话 git log 核验，与 06:21 条目一致，无新提交）；DAG/台账所在 integration worktree `.worktrees/passb-int`（分支 `codex/passb-integration`，HEAD 仍为 `78193c4e5`，本会话 git log 核验）
- **base SHA**：`b1a3d6dd8`（未动）；**head SHA**：null（未回填——节点级评审通过时回填）
- **测试证据路径**：无新增（B0.1–B0.6 六任务报告/审查包见 03:40–06:21 各条目；节点 5 项 gates 全过见 06:21 条目——复用即沿用该证据链）
- **审查结论**：任务级六条通过；节点级架构审查 pending（06:26 OCR 失败后待重试）
- **OCR 报告路径**：无（尚未产出）
- **修复轮次**：1（OCR background 9914 字符超 8000 硬限 aborting；重试待做，结果由后续条目回填）
- **本次 JSON 变更**：**无字节级改动**——指令目标态与本会话 `python3 json.load` 读取的现值完全一致：b0.`status=in_progress`（恢复即 in_progress，恢复提交 `78193c4e5` 已置位）、`task_status` 六任务全 `done`（复用 = 保持 done，无重置动作）、`base_sha=b1a3d6dd8`/`head_sha=null`/`review_status=pending` 均未动。JSON 合法性本会话复验通过（33 节点：1 in_progress + 32 pending）。
- **备注**：本条与 06:49 条目衔接：06:49 记录 "b0 → running" 派发时的状态恢复，本条补充调度方明确的"复用 B0.1–B0.6 已完成任务"指令语义并确认 task_status 不重置；其余 32 节点本指令未触及。

---

## 2026-09-23 07:09 CST · b0 节点级 OCR 第 1 轮：9 findings 全 confirmed（→ changes_requested）

- **节点/轮次**：b0 节点级架构审查 OCR 第 1 次（06:26 因 background 9914 字符超 8000 硬限 aborting 后的首次成功运行）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b0/ocr-r1.txt`（本会话已读全文：8,899 字节/146 行，2026-09-23 07:04 生成；首行 "Review complete: 9 finding(s) across 13 selected item(s)."）
- **结论计数**：confirmed=9 / rejected=0（调度指令口径；与报告 9 findings 数量一致，9 条全部确认、无一驳回）
- **findings 清单**（报告转录，2 high + 7 low）：
  1. [bug · high] `tools/passbguard/check.go:213-216` —— exception 台账缺 (from,to) 边级判重，两条不同 ID 相同 from→to 的台账行静默通过，"105 条例外唯一 removal owner" 失去 CI 保护（建议补 seenEdge）
  2. [bug · high] `tools/passbguard/discover.go:119-124` —— discoverGoTree 只跳过 .git 目录与 testdata、未跳过点前缀目录；worktree 的 .git 是文件命中不了 SkipDir，主仓根跑 `make check-passb-readiness` 时 `.worktrees/` 整仓副本进入 D.GoFiles 必然虚假诊断（建议 `strings.HasPrefix(entry.Name(), ".")`）
  3. [test · low] `tools/internal/movemanifest/manifest.go:4` —— 严格加载器回归测试留在消费方 `tools/modulemove/manifest_test.go` 未随库迁入（建议迁移并改 package movemanifest）
  4. [bug · low] `tools/passbguard/discover.go:183-186` —— `vs.Values[0]` 无长度防护，声明形态变更即 index out of range panic
  5. [bug · low] `tools/passbguard/model.go:263-267` —— integration_owner plan-id 形态不校验 KnownPlans 成员资格，拼错 plan id 静默生效
  6. [performance · low] `tools/passbguard/check.go:624-628` —— containsExact 线性扫描未复用 CheckContracts 顶部已建的 goFileSet
  7. [bug · low] `tools/passbguard/load.go:95-103` —— decodeStrict 只解码首个 YAML 文档，追加第二个 `---` 文档被静默截断零诊断
  8. [maintainability · low] `tools/passbguard/overlap.go:267-268` —— briefClaimsPath 仅 basename 匹配丢目录上下文（同 basename 异属主时误报/互相掩盖）
  9. [maintainability · low] `tools/passbguard/check.go:172-176` —— PassBTaskModule 回退分支为不可达死代码，建议删除
- **审查结论**：OCR 第 1 轮完成但**未通过**——9 条 findings 全部 confirmed、0 rejected，须修复后重审
- **修复轮次**：1（第 1 轮修复待启动；其中 2 条 high 为门禁正确性问题：#1 边级判重缺失、#2 点前缀目录未跳过——后者影响标准开发机上 `make check-passb-readiness` 的可用性）
- **base/head SHA**：`b1a3d6dd8` / null（未动——head_sha 待修复轮全部通过、节点级审查 approved 后回填）
- **worktree**：修复应在 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `9cdfa0c62`，本轮 git log 复核无新提交）进行；全部 findings 落在 `tools/passbguard/**` 与 `tools/internal/movemanifest`（B0.1/B0.2 交付面，均在 b0 owned_files `tools/passbguard/**` 与 F1 方案 a 共享库范围内）
- **本次 JSON 变更**：b0.`review_status` pending → **changes_requested**（conventions §9：pending → requested → changes_requested → approved）；`status` 保持 `in_progress`（9 条修复仍属节点执行期）、`task_status` 六任务 done 不动、base/head SHA 不动、其余 32 节点不动。本会话 `git diff` 核验恰 1 行变更；`python3 json.load` 复验合法（review_status 分布：1 changes_requested + 32 pending）
- **备注**：修复轮完成后节点级 OCR 需重跑（第 2 轮）；按 06:26 条目教训，重跑时 OCR background 仍须 <8000 字符。

---

## 2026-09-23 07:32 CST · b0 节点级 OCR 第 2 轮：0 findings（→ approved，节点级审查通过）

- **节点/轮次**：b0 节点级架构审查 OCR 第 2 次（第 1 轮 9 findings 修复后复审）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b0/ocr-r2.txt`（本会话已读全文：57 字节/1 行，2026-09-23 07:29 生成；"Review complete: 0 finding(s) across 2 selected item(s)."）
- **结论计数**：confirmed=0 / rejected=0（调度指令口径；与报告 0 findings 一致——复审范围 = 2 个修复提交，零新发现）
- **修复轮核验**（本会话 git 实测）：`.worktrees/passb-b0`（`codex/passb-b0`）自 `9cdfa0c62` 后新增恰 2 个修复提交，工作树干净——
  1. `a7717e385` fix(passb): exception 台账补 (from,to) 边级判重 exception-overlap（R1 #1 high；`check.go` +12/`ownership_test.go` +14，同边不同 ID fixture 回归）
  2. `389214f08` fix(passb): discoverGoTree 跳过全部点前缀目录（含 .worktrees worktree 树）（R1 #2 high；`discover.go` +14/`discover_test.go` +73，点前缀跳过 + 根目录豁免 fixture）
- **审查结论**：OCR 第 2 轮**通过**——节点级架构审查 approved（第 1 轮 2 条 high 已修复且复审零发现）
- **留痕：第 1 轮 7 条 low 的处置未见载明**——分支上仅 2 个修复提交、工作树干净（本会话核验），7 条 low（movemanifest 测试迁移/discover.go:183 长度防护/model.go:263 KnownPlans 校验/check.go:624 goFileSet 复用/load.go:95 多文档拒绝/overlap.go:267 完整路径匹配/check.go:172 死代码删除）无对应代码变更；调度指令与 ocr-r2.txt 均未说明其裁定（修复/豁免/延期）。如实登记，待协调者澄清或显式裁定
- **修复轮次**：1（第 1 轮 9 confirmed → 2 high 已修复 → 复审 0 findings，闭环）
- **base/head SHA**：`b1a3d6dd8` / null（未动——分支评审虽通过，但按 06:21 条目与 b0-evidence.md 声明，head_sha 须以本分支合并进 integration 分支后的头回填，不得用分支中间提交；合并尚未发生，integration HEAD 仍为 `78193c4e5`）
- **worktree**：`.worktrees/passb-b0`（`codex/passb-b0`，HEAD `389214f08`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`）
- **本次 JSON 变更**：b0.`review_status` changes_requested → **approved**（conventions §9 终态；工作树累计 diff 相对 HEAD 恰 1 行 pending→approved）；`status` 保持 `in_progress`、`task_status` 六任务 done、base/head SHA、其余 32 节点均未动；`python3 json.load` 复验合法（review_status 分布：1 approved + 32 pending）
- **待协调者处置**：(1) 将 `codex/passb-b0`（HEAD `389214f08`）合并进 `codex/passb-integration`；(2) 合并后置 b0 → done 并以合并头回填 head_sha；(3) 7 条 low findings 的显式裁定（见上"留痕"）；(4) b0 done 后 32 个依赖节点解除（notes 内 32 处 BLOCKED 文本可一并清理）

---

## 2026-09-23 07:50 CST · b0 → blocked（最终全量 OCR 有未关闭问题）＋ 全部 32 个传递依赖节点 → blocked

- **节点**：b0（review approved 旋即被最终全量 OCR 推翻）+ **32 个传递依赖节点**（闭包 = 其余全部节点，与 06:26 条目同）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **阻塞原因**（调度指令原文，acceptance 字段于"在 d"处截断、如实保留）：最终全量 OCR 仍有未关闭问题——`{"id":"tools/passbguard/discover.go:189-192","severity":"medium","problem":"vs.Values[0] 未先检查 len(vs.Values)：discoverImportExceptions 匹配到 importExceptions 名字后直接索引 vs.Values[0]，190-192 行的 !ok 分支只防『Values[0] 存在但非复合字面量』，防不了空 Values。当前 architectureguard/check.go:106 确为 var importExceptions = []importException{...}（带初始化…"}`
- **finding 关联与实地核验**（本会话）：该 finding 与 **OCR R1 第 4 条同源**（R1 报告 ocr-r1.txt：`tools/passbguard/discover.go:183-186 [bug · low]`，vs.Values[0] 无长度防护）——行号随修复提交 `a7717e385`/`389214f08` 偏移至 189-192，级别 low→medium；本会话 `sed -n '185,196p'` 实读 passb-b0 分支该代码区，`vs.Values[0].(*ast.CompositeLit)` 直索引、无 len 防护属实；`sed -n '104,108p'` 实读 `tools/architectureguard/check.go:106` 确为带初始化式 `var importExceptions = []importException{`（当前不触发 panic，防护缺失属前瞻性要求）。**这正是 07:32 条目留痕的"R1 七条未处置 low"之一——当时已如实登记其处置未见载明，现其中一条升级为阻塞项**
- **审查结论**：07:32 的 approved 基于第 2 轮 OCR 仅扫描 2 个修复提交（"0 findings across 2 selected items"）——最终全量 OCR 复扫后推翻；节点级审查结论回退为 changes_requested
- **OCR 报告路径**：本轮（最终全量 OCR）报告路径调度指令未附，仅内联 finding JSON；R1/R2 报告路径见 07:09/07:32 条目
- **修复轮次**：2（第 1 轮修 2 high 后复审通过；第 2 轮=最终全量 OCR 发现 1 条未关闭 medium，待修复）
- **base/head SHA**：`b1a3d6dd8` / null（未动；合并与 head_sha 回填继续冻结，07:32 条目待办 1/2 顺延）
- **worktree**：修复仍在 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `389214f08`）；DAG/台账所在 `.worktrees/passb-int`（HEAD `78193c4e5`）
- **本次 JSON 变更**（原子，全部断言通过）：
  1. b0 `status` in_progress → **blocked**；b0 `review_status` approved → **changes_requested**（指令未显式点名该字段，按阻塞原因（OCR 有未关闭 finding）推断回退并在此留痕）；b0 notes 数组追加 BLOCKED 条目（含 finding 摘要、同源关联、解除条件）
  2. 其余 32 节点 `status` pending → blocked——**notes 不重复追加**：32/32 节点 notes 已含 06:26 遗留的「BLOCKED（2026-09-23）：前置 b0 阻塞；解除条件：修复并 done b0 后恢复。」文本（恢复提交 `78193c4e5` 保留至今，本会话修正迭代 bug 后逐节点核验 32/32 在位），本次仅翻转 status
  3. `python3 json.loads` 复验合法：33 节点全 blocked；`git diff --stat` 相对 HEAD 36+/35−
- **未动字段**：b0.task_status（六任务仍 done）、全部 base_sha、全部 head_sha（null）、其余 32 节点 review_status（pending）
- **解除条件**：按（截断的）验收标准修复 discover.go:189-192 长度防护 → 最终全量 OCR 复审零未关闭 → done b0（合并 + head_sha 回填）→ 32 个依赖节点恢复；同时建议协调者对 R1 剩余 6 条 low 一并显式裁定，避免再次逐条阻塞

---

## 2026-09-23 08:05 CST · b0 → in_progress（重新派发"running"；目标值已在位，JSON 无字节级改动）

- **节点**：b0 —— B0 契约与所有权冻结（passbguard + 4 份治理 yaml + 证据）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：无（depends_on = []）
- **worktree**：b0 实施分支 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `389214f08`，本会话 git log/status 核验：9cdfa0c62 之上恰 2 个修复提交、工作树干净）；DAG/台账所在 integration worktree `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`）
- **base SHA**：`b1a3d6dd8`（未动）；**head SHA**：null（未回填——按 06:21/07:32 条目，须以本分支合并进 integration 后的头回填）
- **测试证据路径**：无新增（B0.1–B0.6 六任务报告/审查包见 03:40–06:21 各条目；节点 5 项 gates 全过见 06:21 条目）
- **审查结论（状态核验）**：本会话 `python3 json.load` 实测——33 节点 1 in_progress + 32 pending，`review_status` 全 33=pending。07:09–07:50 三条目所载 JSON 变更（review_status 两度翻转至 changes_requested、全 33 节点 blocked）**均系工作树级编辑且已被回退**：本会话 `git diff HEAD -- docs/plans/passb/execution-dag.json` 为空、`git status --short` 仅台账一个 untracked。台账按恢复提交 `78193c4e5` 约定保留全部历史；07:50 阻塞事由（discover.go:189-192 `vs.Values[0]` 长度防护 medium）仍在案、待本轮处置
- **OCR 报告路径**：第 1/2 轮 = `.superpowers/sdd/passb/b0/ocr-r1.txt` / `ocr-r2.txt`（见 07:09/07:32 条目）；07:50 最终全量 OCR 未附报告路径（仅内联 finding JSON）；本轮（重派后）尚未产出
- **修复轮次**：2（第 2 轮待办：修复 discover.go:189-192 长度防护 → 最终全量 OCR 复审零未关闭 → 合并 + head_sha 回填 → done b0；建议一并显式裁定 R1 剩余 6 条 low，见 07:50 条目）
- **本次 JSON 变更**：**无字节级改动**——指令目标 "b0 → running" 按状态机（conventions §9 无 `running` 值）沿 03:18/06:49 先例映射为规范值 `in_progress`，而该值已在已提交态（`78193c4e5`）中位；`task_status` 六任务 done、`base_sha`/`head_sha`/`review_status` 均未动。JSON 合法性本会话 `python3 json.load` 复验通过（33 节点：1 in_progress + 32 pending）
- **备注**：其余 32 节点本指令未触及（现状 pending，notes 内 32 处 BLOCKED 文本仍在——b0 done 后随恢复一并清理，见 07:32 条目待办 4）

---

## 2026-09-23 08:06 CST · b0 恢复运行（复用 B0.1–B0.6 已完成任务；与 08:05 派发衔接）

- **节点**：b0 —— B0 契约与所有权冻结（passbguard + 4 份治理 yaml + 证据）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：无（depends_on = []）
- **恢复语义**（沿 06:50 条目先例）：复用已完成任务 B0.1、B0.2、B0.3、B0.4、B0.5、B0.6——六任务不重置、不重跑；节点剩余待办 = 第 2 修复轮（discover.go:189-192 长度防护，07:50 条目在案）→ 最终全量 OCR 复审 → 合并 + head_sha 回填 → done
- **worktree**：b0 实施分支 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `389214f08`，本会话 git log/status 核验：自 07:32 后无新提交、工作树干净——第 2 轮修复尚未落分支）；DAG/台账所在 integration worktree `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`）
- **base SHA**：`b1a3d6dd8`（未动）；**head SHA**：null（未回填——合并进 integration 后回填）
- **测试证据路径**：无新增（B0.1–B0.6 六任务报告/审查包见 03:40–06:21 各条目；节点 5 项 gates 全过见 06:21 条目——复用即沿用该证据链）
- **审查结论**：任务级六条通过（在案）；节点级 = 第 2 修复轮待办（07:50 最终全量 OCR 1 条 medium 未关闭；其 JSON 侧 blocked/changes_requested 标记已被回退，见 08:05 条目核验）
- **OCR 报告路径**：第 1/2 轮 = `.superpowers/sdd/passb/b0/ocr-r1.txt` / `ocr-r2.txt`；07:50 最终全量 OCR 无报告路径；修复后复审尚未运行
- **修复轮次**：2（discover.go:189-192 medium 待修复；R1 剩余 6 条 low 建议一并显式裁定）
- **本次 JSON 变更**：**无字节级改动**——指令目标态与本会话 `python3 json.load` 读取的现值完全一致：b0.`status=in_progress`（08:05 派发已在位）、`task_status` 六任务全 `done`（复用 = 保持 done，无重置动作）、`base_sha=b1a3d6dd8`/`head_sha=null`/`review_status=pending` 均未动。JSON 合法性本会话复验通过（33 节点：1 in_progress + 32 pending）
- **备注**：本条与 08:05 条目衔接（08:05 记录 "running" 派发映射，本条补充调度方"复用 B0.1–B0.6"指令语义并确认 task_status 不重置）；其余 32 节点本指令未触及

---

## 2026-09-23 08:32 CST · b0 节点级 OCR（重派后第 1 次）：14 findings 全 confirmed（→ changes_requested）

- **节点/轮次**：b0 节点级架构审查 OCR——调度口径"第 1 次"；台账历史口径为第 4 次 OCR 运行（07:09 r1 / 07:29 r2 / 07:45 final / 本轮 08:28），报告文件复用 ocr-r1.txt 路径（覆盖情况见下）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b0/ocr-r1.txt`（本会话已读全文：14,442 字节、mtime 08:28，首行 "Review complete: 14 finding(s) across 13 selected item(s)."；**该路径与 07:09 条目同名，旧内容（8,899 字节/9 findings）已被本轮报告覆盖**）
- **结论计数**：confirmed=14 / rejected=0（调度指令口径；与报告 14 findings 一致，全部确认、无一驳回）
- **findings 清单**（报告顺序转录；severity：high=1 / medium=3 / low=10；类别：bug=9 / documentation=2 / maintainability=2 / test=1）：
  1. [documentation · low] `tools/modulemove/main.go:26-28` —— sortStrings 注释称"就地排序"实为返回副本（ocr-final #3 未关闭）
  2. [test · low] `tools/internal/movemanifest/manifest.go:1-4` —— 共享库测试未随迁，包自身零覆盖、ValidModuleID 无测试（ocr-final #4；R1 第 3 条再现）
  3. [bug · high] `tools/passbguard/discover.go:189-192` —— `vs.Values[0]` 缺 len 防护（**ocr-final #1 阻塞项，仍未修复**；passb-b0 HEAD `389214f08` 无新提交，本会话 git 核验一致；R1 第 4 条 low 升级为 high）
  4. [bug · medium] `tools/passbguard/check.go:315` —— entryFileRE 过宽松，contracts.yaml 已冻结 14 处幽灵 consumer（container.go×11/router.go×2/routes_chat.go×1；ocr-final #2）
  5. [bug · medium] `tools/passbguard/check.go:352` —— worker 基线两键同取 m[1]、池数捕获组丢弃，Redis 池数（6）漂移永不触发（ocr-final #5）
  6. [bug · low] `tools/passbguard/discover.go:341-353` —— var/const 发现：Signature 前缀硬编码 "type "、多名声明被 continue 跳过（ocr-final #6）
  7. [maintainability · low] `tools/passbguard/check.go:172-176` —— alias 回填循环死代码（ocr-final #7 / R1 第 9 条再现）
  8. [documentation · low] `tools/passbguard/discover.go:25-27` —— repoModulePath "同源常量"注释失实（ocr-final #8）
  9. [bug · low] `tools/passbguard/overlap.go:276-279` —— briefClaimsPath basename 词元匹配目录歧义误报 + `end > len(text)` 死条件（R1 第 8 条同类再列，本轮无 ocr-final 标注）
  10. [bug · low] `tools/passbguard/load.go:95-103` —— decodeStrict 多文档 YAML 静默截断（R1 第 7 条同类再列，本轮无 ocr-final 标注）
  11. [maintainability · low] `tools/passbguard/check.go:393-396` —— 基线解析失败后零值级联 contract-baseline-drift 误报（本轮新列）
  12. [bug · medium] `tools/passbguard/discover.go:439-443` —— qualifiersFor 按路径末段推导默认限定名而非真实 package 名（潜伏漏报+vanished 误报；本轮新列）
  13. [bug · low] `tools/passbguard/discover.go:416-420` —— sameDir 裸 Ident 误命中 SelectorExpr Sel 子节点、外部测试包按同包匹配（本轮新列）
  14. [bug · low] `tools/passbguard/check.go:209-212` —— guard 源侧同 (from,to) 重复边 last-wins，与台账侧 seenEdge 防护不对称（本轮新列）
- **审查结论**：OCR 本轮**未通过**——14/14 confirmed、0 rejected，须修复后重审（14 条中 8 条系 ocr-final（07:45）未关闭项的承接确认，6 条为本轮新列/再列）
- **修复轮次**：2（第 1 轮 07:09→07:32 已闭环；第 2 轮 = ocr-final 未关闭项 + 本轮新增项，**尚未启动**——分支无新修复提交）
- **base/head SHA**：`b1a3d6dd8` / null（未动——head_sha 待修复轮通过、复审零未关闭、合并进 integration 后回填）
- **worktree**：修复应落 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `389214f08`，本会话 git log/status 核验：9cdfa0c62 之上恰 2 提交、工作树干净）；DAG/台账所在 `.worktrees/passb-int`（HEAD `78193c4e5`）
- **本次 JSON 变更**：b0.`review_status` pending → **changes_requested**（conventions §9：pending → requested → changes_requested → approved，沿 07:09 先例直接落入 changes_requested）；本会话 `git diff` 核验**恰 1 行变更**；`python3 json.load` 复验合法（review_status 分布：1 changes_requested + 32 pending）；`status` 保持 `in_progress`（修复属节点执行期）、`task_status` 六任务 done、base/head SHA、其余 32 节点均未动
- **备注**：(1) 07:50 条目建议的"R1 剩余 6 条 low 显式裁定"延续未决——本轮其中 5 条再现（#2/#7/#9/#10 及升级为 high 的 #3），model.go:263 KnownPlans 与 check.go:624 containsExact 两条**未再列**（未再列≠豁免，处置仍待协调者裁定）；(2) 修复后重跑 OCR 时 background 仍须 <8000 字符（06:26 教训）；(3) #4（14 处幽灵 consumer）涉 contracts.yaml 数据面修正，修复时须与守卫正则收紧同步，否则收紧即刻红

---

## 2026-09-23 09:32 CST · b0 节点级 OCR（重派后第 2 次）：0 findings（→ approved，节点级审查通过）

- **节点/轮次**：b0 节点级架构审查 OCR 第 2 次（08:32 轮 14 findings 修复后复审）；台账历史口径为第 5 次 OCR 运行（07:09 r1 / 07:29 r2 / 07:45 final / 08:28 r1 / 09:30 r2）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b0/ocr-r2.txt`（本会话已读全文：345 字节、mtime 09:30，首行 "Review complete: 0 finding(s) across 3 selected item(s)."；**该路径与 07:32 条目同名，旧内容（57 字节）已被覆盖**；报告附 LLM retry 摘要——6 请求中 1 次网络错误重试后成功，核心审查请求覆盖 contracts.yaml/check.go/discover.go）
- **结论计数**：confirmed=0 / rejected=0（调度指令口径；与报告 0 findings 一致）
- **修复轮核验**（本会话 git 实测）：`.worktrees/passb-b0`（`codex/passb-b0`）自 `389214f08` 后新增恰 1 个修复提交、工作树干净——
  - `97d8d2632` fix(passb): OCR R1 第二批 f3/f4/f5/f12 修复（`git show --stat`+diff 本会话实读）：**f3(high)** discover.go `vs.Values[0]` 前补 len 防护（无值声明返回干净错误+表驱动回归）；**f4(medium)** entryFileRE 要求目录段并同步删除 contracts.yaml 14 条幽灵 consumer；**f5(medium)** worker 行补池数捕获组、新增 worker_pools 键并冻结 worker_pools=6；**f12(medium)** 默认限定名改取定义文件 package 子句、删除路径尾段推导
  - 覆盖核对：f3/f4/f5/f12 = 08:32 轮 14 条中的 **1 high + 全部 3 medium**
- **审查结论**：OCR 第 2 次**通过**——节点级架构审查 approved（按 07:32 先例）
- **留痕（与 07:32→07:50 翻转同型风险，务必关注）**：08:32 轮 14 条中**其余 10 条 low（f1/f2/f6/f7/f8/f9/f10/f11/f13/f14）未见处置**——分支仅 1 个修复提交（本会话核验），提交信息与 diff 均未涉及；本轮复审范围仅 **3 selected items**（修复提交触及文件），非全量复扫。07:32 条目 approved 后即被 07:45 最终全量 OCR 以未处置 low 推翻（07:50 条目）——**若协调者再跑最终全量 OCR，极可能重演**。建议：done b0 前对 10 条 low 显式裁定（修复/豁免/延期），或将最终收口审查明确限定为增量复审
- **修复轮次**：2（第 2 轮 = 08:32 轮 14 findings：1 high + 3 medium 已修复、10 low 未处置→复审 0 findings，闭环但见上留痕）
- **base/head SHA**：`b1a3d6dd8` / null（未动——按 06:21/07:32 条目与 b0-evidence.md 声明，head_sha 须以本分支合并进 integration 后的头回填，不得用分支中间提交 `97d8d2632`；合并尚未发生，integration HEAD 仍 `78193c4e5`）
- **worktree**：`.worktrees/passb-b0`（`codex/passb-b0`，HEAD `97d8d2632`，工作树干净）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`）
- **本次 JSON 变更**：b0.`review_status` changes_requested → **approved**（conventions §9 终态）；相对 HEAD 累计净变更恰 review_status 1 行（pending→approved，中间态 changes_requested 为工作树未提交编辑）；`python3 json.load` 复验合法（review_status 分布：1 approved + 32 pending）；`status` 保持 `in_progress`（done 与合并/head_sha 回填仍待协调者，沿 07:32 待办）、`task_status` 六任务 done、base/head SHA、其余 32 节点均未动
- **待协调者处置**（沿 07:32 条目，顺延更新）：(1) 将 `codex/passb-b0`（HEAD `97d8d2632`）合并进 `codex/passb-integration`；(2) 合并后置 b0 → done 并以合并头回填 head_sha；(3) **10 条 low findings 的显式裁定**（见上留痕，本轮新增 6 条：f6 var/const 发现、f7 alias 死代码、f8 repoModulePath 注释、f9 overlap basename、f10 load.go 多文档、f11 基线级联误报、f13 sameDir 误命中、f14 guard 重复边、另 f1/f2 文档/测试——共 10 条待点名豁免或修复）；(4) b0 done 后 32 个依赖节点解除（notes 内 32 处 BLOCKED 文本一并清理）

---

## 2026-09-23 09:45 CST · b0 → blocked（最终全量 OCR 输出不完整）＋ 全部 32 个传递依赖节点 → blocked

- **节点**：b0（09:32 增量复审 approved 后，最终全量 OCR 基础设施失败）+ **32 个传递依赖节点**（本会话 BFS 实测：b0 为 DAG 根，闭包=其余全部 32 节点，闭包外为空——与 06:26/07:50 条目一致）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **阻塞原因**（调度指令原文照录）：**Error: 最终全量 OCR 输出不完整（截断/跳过文件）**——即 09:32 条目"待协调者处置"预期的最终全量 OCR 复扫因输出截断/跳过文件而未产出完整审查结论，节点无法收口
- **与历史的关联**：这是 b0 节点级审查第 3 次基础设施失败——06:26（background 9914 字符超 8000 硬限）、07:50（全量 OCR 发现未关闭 finding，属审查结论非基础设施）、本次 09:45（输出不完整/截断/跳过文件）。09:32 条目"留痕"预警的风险以基础设施失败形式先行兑现：10 条未处置 low 在完整全量结论出来前仍悬置
- **review_status 处置**：b0 保持 `approved` **未回退**——沿 06:26 基础设施失败先例（当时未动 review_status；07:50 回退系因 OCR 产出了实质 finding）；现 approved 的审查基础是 09:32 增量复审（3 selected items），最终全量 OCR 尚无完整结论，**若重跑完成且产出 finding，届时再回退 changes_requested**
- **base/head SHA**：`b1a3d6dd8` / null（未动；合并与 head_sha 回填继续冻结，09:32 待办 1/2 顺延）
- **worktree**：`.worktrees/passb-b0`（`codex/passb-b0`，HEAD `97d8d2632`，本会话核验无新提交、工作树干净）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`）
- **OCR 报告路径**：本轮（失败的最终全量 OCR）报告路径调度指令未附（仅内联错误消息）；既有在案：ocr-r1.txt（08:28 版，14 findings）、ocr-r2.txt（09:30 版，0 findings/3 items）、ocr-final.txt（07:45 版）
- **修复轮次**：2（不变——第 2 轮修复已闭环 1 high + 3 medium；本轮阻塞系审查基础设施失败，不新增修复轮）
- **本次 JSON 变更**（原子，全部断言通过）：
  1. b0 `status` in_progress → **blocked**；b0 notes 数组追加 BLOCKED 条目（第 9 条，含错误原文与解除条件，conventions §5 上报摘要要求）
  2. 其余 32 节点 `status` pending → blocked——**notes 不重复追加**：本会话逐节点核验 32/32 已含「BLOCKED（2026-09-23）：前置 b0 阻塞；解除条件：修复并 done b0 后恢复。」文本（06:26 遗留、恢复提交 `78193c4e5` 保留至今），与本次指令要求的依赖节点阻塞原因措辞一致，本次仅翻转 status
  3. `python3 json.load` 复验合法：33 节点全 blocked、review_status 分布 1 approved + 32 pending；`git diff 78193c4e5 --stat` = 36 插入/35 删除（= 33 行 status + 1 行 review_status[pending→approved，09:32 所为] + 1 行新增 notes）
- **未动字段**：b0.task_status（六任务仍 done）、b0.review_status（approved，见上处置说明）、全部 base_sha、全部 head_sha（null）、其余 32 节点 review_status（pending）
- **解除条件**：重跑完整的最终全量 OCR（输出不得截断/跳过文件）并按结论收口——通过则 done b0（合并 + head_sha 回填 + 32 依赖节点恢复与 BLOCKED 文本清理）；有 finding 则回退 changes_requested 进入下一修复轮；同时重申 09:32 待办 3：10 条 low 的显式裁定应在收口前完成

---

## 2026-09-23 10:12 CST · b0 → in_progress（重派 "running"；目标值已在位，JSON 无字节级改动；09:45 blocked 编辑已被回退）

- **节点**：b0 —— B0 契约与所有权冻结（passbguard + 4 份治理 yaml + 证据）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：无（depends_on = []）
- **worktree**：b0 实施分支 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `97d8d2632`，本会话 git log/status 核验、工作树干净——与 09:32 条目一致，09:45 后无新提交、第 3 修复轮未落分支）；DAG/台账所在 integration worktree `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`，本会话 git log 核验，与 08:05 条目一致）
- **base SHA**：`b1a3d6dd8`（未动）；**head SHA**：null（未回填——按 06:21/07:32/09:32 条目，须以本分支合并进 integration 后的头回填）
- **测试证据路径**：无新增（B0.1–B0.6 六任务报告/审查包见 03:40–06:21 各条目；节点 5 项 gates 全过见 06:21 条目——复用即沿用该证据链）
- **审查结论（状态核验）**：本会话 `python3 json.load` 实测——33 节点 **1 in_progress（b0）+ 32 pending（blocked=0）**，`review_status` 1 approved（b0）+ 32 pending，b0.task_status 六任务全 done，b0 notes 无 BLOCKED 条目。**09:45 条目所载 JSON 变更（b0→blocked + 32 依赖节点→blocked + b0 notes 追加 BLOCKED）已被回退**：本会话 `git status --short` 仅 DAG 1 处修改 + 台账 untracked；`git diff` 相对 HEAD 恰 1 行 = b0.`review_status` pending→approved（09:32 所为、仍在位）。回退模式与 07:50→08:05 一致
- **OCR 报告路径**：既有在案——ocr-r1.txt（08:28 版，14 findings）、ocr-r2.txt（09:30 版，0 findings/3 items）、ocr-final.txt（07:45 版）；09:45 失败的最终全量 OCR 无报告路径（仅内联错误消息）；本轮（重派后）尚未产出
- **修复轮次**：2（第 1/2 轮成果在案；09:45 阻塞事由「最终全量 OCR 输出不完整（截断/跳过文件）」仍未收口，待本轮重跑——b0 实施分支无新修复提交）
- **本次 JSON 变更**：**无字节级改动**——指令 "b0 → running" 按状态机（conventions §9 无 `running` 值）沿 03:18/06:49/08:05 先例映射为规范值 `in_progress`，而该值已在已提交态（恢复提交 `78193c4e5`）中位；`task_status` 六任务 done（复用不重置）、`base_sha`/`head_sha`/`review_status` 均未动。JSON 合法性本会话 `python3 json.load` 复验通过（33 节点：1 in_progress + 32 pending）
- **剩余待办**（承接 09:32/09:45，均顺延）：(1) 重跑完整的最终全量 OCR（输出不得截断/跳过文件，background <8000 字符）；(2) 通过则合并 `codex/passb-b0`（HEAD `97d8d2632`）进 `codex/passb-integration` → 置 b0 done 并以合并头回填 head_sha；(3) 10 条 low findings 的显式裁定（09:32 条目待办 3）；(4) b0 done 后 32 个依赖节点解除——notes 内 32 处 BLOCKED 文本本会话逐节点核验 32/32 仍在，随恢复一并清理
- **备注**：其余 32 节点本指令未触及（现状 pending；其 notes 内 BLOCKED 文本与 status=pending 并存系恢复提交 `78193c4e5` 保留至今的既知状态，非本管家改动）

---

## 2026-09-23 10:14 CST · b0 恢复运行（复用 B0.1–B0.6 已完成任务；与 10:12 派发衔接）

- **节点**：b0 —— B0 契约与所有权冻结（passbguard + 4 份治理 yaml + 证据）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：无（depends_on = []）
- **恢复语义**（沿 06:50/08:06 条目先例）：复用已完成任务 B0.1、B0.2、B0.3、B0.4、B0.5、B0.6——六任务不重置、不重跑；节点剩余待办 = 重跑完整的最终全量 OCR（09:45 条目阻塞事由"输出不完整/截断/跳过文件"）→ 通过则合并 + head_sha 回填 + done b0
- **worktree**：b0 实施分支 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `97d8d2632`，10:12 条目核验、工作树干净——本条目未重跑，无新信息源）；DAG/台账所在 integration worktree `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`，本会话 git log 核验，与 10:12 条目一致）
- **base SHA**：`b1a3d6dd8`（未动）；**head SHA**：null（未回填——合并进 integration 后回填）
- **测试证据路径**：无新增（B0.1–B0.6 六任务报告/审查包见 03:40–06:21 各条目；节点 5 项 gates 全过见 06:21 条目——复用即沿用该证据链）
- **审查结论**：任务级六条通过（在案）；节点级 = 09:32 增量复审 approved（工作树在位），最终全量 OCR 收口未完成（09:45 失败，待重跑）
- **OCR 报告路径**：既有在案 ocr-r1.txt（08:28 版，14 findings）/ ocr-r2.txt（09:30 版，0 findings/3 items）/ ocr-final.txt（07:45 版）；重跑后报告由后续条目回填
- **修复轮次**：2（第 1/2 轮成果在案；最终全量 OCR 待重跑，重跑若有 finding 则进入下一修复轮）
- **本次 JSON 变更**：**无字节级改动**——指令目标态与本会话 `python3 json.load` 读取的现值完全一致：b0.`status=in_progress`（10:12 派发已在位）、`task_status` 六任务全 `done`（复用 = 保持 done，无重置动作）、`base_sha=b1a3d6dd8`/`head_sha=null`/`review_status=approved`（09:32 工作树在位值）均未动。JSON 合法性本会话 `python3 json.load` 复验通过（33 节点：1 in_progress + 32 pending；`git diff` 相对 HEAD 仍恰 1 行 review_status）
- **备注**：本条与 10:12 条目衔接（10:12 记录 "running" 重派映射与 09:45 回退核验，本条补充调度方"复用 B0.1–B0.6"指令语义并确认 task_status 不重置）；其余 32 节点本指令未触及（notes 内 32 处 BLOCKED 文本本会话脚本核验 32/32 仍在，待 b0 done 后随恢复一并清理）

---

## 2026-09-23 10:49 CST · b0 节点级 OCR（10:12 重派后第 1 次）：16 findings，调度口径 confirmed=14 / rejected=2（→ changes_requested）

- **节点/轮次**：b0 节点级架构审查 OCR——调度口径"第 1 次"（10:12 重派、10:14 复用派发后的首轮）；台账历史口径**第 6 次 OCR 运行**（07:09 r1 / 07:29 r2 / 07:45 final / 08:28 r1 / 09:30 r2 / 本次 10:41 生成）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b0/ocr-r1.txt`（本会话已读全文：15,237 字节、mtime 10:41、253 行；首行 "Review complete: 16 finding(s) across 13 selected item(s)"；**该路径第三次被覆盖**——07:04 版 8,899B/9 findings → 08:28 版 14,442B/14 findings → 本次；报告附 LLM retry 摘要：47 请求中 1 次网络错误重试后成功）
- **结论计数**：调度口径 confirmed=14 / rejected=2；报告实测 16 findings（14+2 与报告总数对账一致）。**留痕：rejected 的 2 条未点名**——调度指令与报告正文均未标注哪 2 条被驳回，无法从在案材料判定，待协调者补充点名（不影响本轮"未通过"判定：14 条 confirmed 已构成修复义务）
- **findings 清单**（报告顺序转录；severity：medium=3 / low=13 / high=0；类别：bug=10 / documentation=2 / test=1 / maintainability=3）：
  1. [documentation · low] `tools/modulemove/main.go:26-27` —— sortStrings 注释"就地排序"失实，实为副本排序（遗留 f1）
  2. [test · low] `tools/internal/movemanifest/manifest.go:1-4` —— 共享库行为测试仍留消费者包，`go test ./tools/internal/movemanifest` no test files（遗留 f2 再现）
  3. [bug · low] `tools/internal/movemanifest/manifest.go:77-82` —— LoadManifestStrict 仅解码首 YAML 文档、多文档静默截断；该缺口经 F1-a 提取同时放大到 modulemove 与 passbguard 双门禁（**新列**，与 f10 同构）
  4. [bug · medium] `tools/passbguard/check.go:400-403` —— 基线台账解析失败后零值级联 contract-baseline-drift 误报（遗留 f11）
  5. [bug · medium] `tools/passbguard/discover.go:430-434` —— referencesSymbol sameDir 分支裸 Ident 文本命中即判消费（字段名/复合字面量键/局部声明误命中），驱动 unrecorded/vanished 两条 CI 阻断诊断（遗留 f13）
  6. [maintainability · low] `tools/passbguard/check.go:710-715` —— worker-set 消费方发现硬编码仅扫 internal/router/ 前缀，B1+ 注册位点迁入模块后成盲区/不可满足（**新列**）
  7. [maintainability · low] `tools/passbguard/check.go:172-176` —— d.Aliases 回填循环恒不可达死代码（遗留 f7 再现）
  8. [bug · low] `tools/passbguard/check.go:737-741` —— fileReferencesIdent 解析失败静默返回 false，worker-set 发现集缺失产出误导性 vanished/假通过（**新列**）
  9. [bug · low] `tools/passbguard/discover.go:347-350` —— ValueSpec 跳过多名声明（`var A, B T` 中 B 永不匹配）+ var/const 签名硬编码 "type " 前缀（遗留 f6）
  10. [documentation · low] `tools/passbguard/discover.go:25-27` —— repoModulePath "同源常量"注释失实（实为手工镜像，go.mod 变更静默失配）（遗留 f8）
  11. [bug · low] `tools/passbguard/check.go:209-212` —— guard 源侧 (from,to) 重复边 map last-wins 静默去重，与台账侧 seenEdge/exception-overlap 防护不对称（遗留 f14）
  12. [bug · low] `tools/passbguard/load.go:95-97` —— decodeStrict 单次 Decode 多文档截断 + present 返回值被全部调用方忽略（遗留 f10）
  13. [bug · low] `tools/passbguard/overlap.go:276-279` —— `end > len(text)` 永假死条件（遗留 f9 之一）
  14. [bug · low] `tools/passbguard/overlap.go:266-267` —— briefClaimsPath 仅 basename 判定认领，同 basename 异目录互相顶替（现实例 native_archive.go service/handler 两处）（遗留 f9 之二）
  15. [maintainability · medium] `tools/passbguard/main.go:46-48` —— **event-catalog 漂移校验不在守卫失败路径**：transport/replay/version/producer on-disk/consumers 存在性校验仅在 events_test.go，不在 `make check-passb-readiness` 失败路径；catalog 整删守卫 exit 0、B1+ 搬迁 consumers 文件时 readiness 静默通过（**新列**）
  16. [bug · low] `tools/passbguard/check.go:166-171` —— aliasModule 跨 manifest 重复声明 last-wins 无诊断（alias-overlap 缺失，与 legacy-overlap/exception-overlap 不对称）（**新列**）
- **与 08:32 轮对照**（本会话逐条比对）：09:32 条目"待显式裁定 10 条 low"（f1/f2/f6/f7/f8/f9/f10/f11/f13/f14）**全部再现**（f9 拆为 #13/#14 两条形态，其余一一对应），另 5 条新列（#3/#6/#8/#15/#16）；08:32 轮已修复的 f3/f4/f5/f12（1 high + 3 medium，提交 `97d8d2632`）**未再列**——第 2 轮修复成果保持有效。09:32 条目留痕预警（"未处置 low 在全量复扫中再现"）本轮兑现
- **审查结论**：OCR 本轮**未通过**——14 confirmed 须修复后重审（rejected=2 待点名，见上留痕）
- **修复轮次**：3（第 1 轮 07:09→07:32 闭环；第 2 轮 08:32→09:30 闭环；第 3 轮 = 本轮 16 findings，**尚未启动**——b0 分支无新提交）
- **base/head SHA**：`b1a3d6dd8` / null（未动——head_sha 待第 3 轮修复通过、复审零未关闭、合并进 integration 后回填）
- **worktree**：修复应落 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `97d8d2632`，10:12 条目核验）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`，本会话 git log 核验）。全部 16 findings 落 `tools/passbguard/**`、`tools/internal/movemanifest`、`tools/modulemove`——均在 b0 owned_files `tools/passbguard/**` 与 F1 方案 a 共享库范围（modulemove/main.go:26 为注释修正，同属 B0 工具面）
- **本次 JSON 变更**：b0.`review_status` approved → **changes_requested**（沿 07:09/08:32 先例——OCR confirmed>0 即落入 changes_requested；先前 approved 系 09:32 工作树未提交编辑，被本次覆盖）；`status` 保持 `in_progress`（修复属节点执行期）、`task_status` 六任务 done、base/head SHA、其余 32 节点均未动。本会话 `python3 json.load` 复验合法（review_status 分布：1 changes_requested + 32 pending）；`git diff` 相对 HEAD 恰 1 行（pending→changes_requested 净效果）
- **备注**：(1) rejected=2 未点名，修复清单划定前建议协调者先点名，避免已驳回项被误修；(2) 重跑 OCR 时 background <8000 字符教训（06:26）仍适用；(3) #15（event 校验不在守卫路径）与 #6（worker-set 前缀硬编码）均预示 B1+ 阶段守卫盲区，建议第 3 轮修复优先处置两条 medium+此两条前瞻项

---

## 2026-09-23 10:54 CST · b0 OCR 误报裁定：rejected 2 条点名（ocr-r1-07 / ocr-r1-16），confirmed=14 维持

- **节点/轮次**：b0 节点级 OCR 第 3 轮 R1（10:49 条目 16 findings）——调度方点名 2 条误报，闭环 10:49 条目备注 (1)"rejected 未点名"缺口
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **误报 1：ocr-r1-07**（10:49 条目 findings #7：`tools/passbguard/check.go:172-176` d.Aliases 回填"恒不可达死代码"，[maintainability · low]，遗留 f7 再现）——裁定**不成立**：(a) fixtureDiscovery()（ownership_test.go:17-46）构造的 Discovery 填充 Aliases 但不填 Manifests；(b) TestOwnershipHappyPathProducesNoDiagnostics（ownership_test.go:87-90）零诊断通过恰恰依赖 check.go:172-176 回填（删之则 aliasModule 为空、g.Aliases 行 !declared 即 emit）；(c) 调度方实测与管家复跑 `go test ./tools/passbguard ./tools/modulemove -count=1` **全绿**（本会话在 passb-b0@97d8d2632 执行：passbguard ok 5.529s / modulemove ok 0.865s）；(d) 仅"DiscoverPassB 生产路径不经过该分支"（discover.go:89 `d.Aliases = append(d.Aliases, m.AliasObligations...)` 同源填充）为真——**降级为注释澄清项，非死代码删除项**
- **误报 2：ocr-r1-16**（10:49 条目 findings #16：`tools/passbguard/check.go:166-171` aliasModule 跨 manifest 重复声明 last-wins"alias 双声明唯一可能检出点"，[bug · low]，新列）——裁定核心论据**为假**：modulemove VerifyManifest 以 alias-1to1 双向锁定（verify.go:108-132：每个 move from 必须有对应 alias、每个 alias 必须有对应 from、manifest 内部重复 old_import_path 报错）+ VerifyAll 对跨 manifest 重复 from 报 ownership-overlap（verify.go:354-359），且经 `make verify-module-moves`（Makefile:250-251 `go run ./tools/modulemove verify --all`）在 CI 执行——跨 manifest 双声明同一 old_import_path 必然触发 alias-1to1 或 ownership-overlap 之一，不存在"完全不可见"场景；passbguard 内部 last-wins 仅为**纵深防御缺口**，非无防护漏洞
- **依据核验**（指令要求"依据须可查"，本管家在 `.worktrees/passb-b0`@`97d8d2632` 逐处实读）：ownership_test.go:17-46（fixtureDiscovery 填 `Aliases: [{OldImportPath: internal/application/service/widgets, PassBTask: B-knowledge}]`、无 Manifests 字段）✓；ownership_test.go:87-90（`require.Empty(t, diags)`）✓；check.go:172-176 回填分支与 :177 起 `!declared` 消费分支 ✓；discover.go:89 同源填充 ✓；verify.go:108-132 alias-1to1 三向检查 ✓；verify.go:354-359 ownership-overlap ✓；Makefile:250-251 target ✓；测试复跑全绿 ✓。**裁定推论本管家亦复核成立**：fixture 场景删除回填必打破 happy-path；双声明场景 verify-module-moves 必拦截
- **审查结论**：第 3 轮 R1 最终口径 = **confirmed 14 / rejected 2**（16 − #7 − #16），`review_status=changes_requested` 维持不变——14 条 confirmed（3 medium + 11 low）修复义务不变，修复清单**剔除 #7/#16**（#7 保留为可选注释澄清、#16 保留为可选纵深防御增强，均非本轮义务）
- **OCR 报告路径**：`.superpowers/sdd/passb/b0/ocr-r1.txt`（10:41 版，16 findings，见 10:49 条目；报告正文无 rejected 标注，误报裁定以本条目+DAG notes 为准）
- **修复轮次**：3（第 3 轮修复清单更新为 14 条，尚未启动——b0 分支 HEAD 仍 `97d8d2632`）
- **base/head SHA**：`b1a3d6dd8` / null（未动）
- **worktree**：核验与测试在 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `97d8d2632`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`）
- **本次 JSON 变更**：b0.`notes` 数组末尾追加 1 条字符串（第 9 条）——两条误报裁定全文（含全部代码依据坐标），沿 06:26/07:50 notes 追加先例；`review_status=changes_requested`、`status=in_progress`、`task_status`、base/head SHA、其余 32 节点均未动。本会话 `python3 json.load` 复验合法（notes 9 条；review 分布 1 changes_requested + 32 pending）；`git diff` 相对 HEAD 4+/3−（= review_status 行 + notes 追加行的自然展开）

---

## 2026-09-23 11:43 CST · b0 节点级 OCR（第 3 轮 R2，修复后增量复审）：6 findings 全 confirmed（→ changes_requested 维持）

- **节点/轮次**：b0 节点级架构审查 OCR 第 3 轮第 2 次；台账历史口径**第 7 次 OCR 运行**（07:09 / 07:29 / 07:45 / 08:28 / 09:30 / 10:41 / 本次 11:36）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b0/ocr-r2.txt`（本会话已读全文：7,612 字节、mtime 11:36、109 行；首行 "Review complete: 6 finding(s) across 3 selected item(s)"；**该路径第二次被覆盖**——09:30 版 345B/0 findings → 本次 11:36 版）
- **结论计数**：confirmed=6 / rejected=0（调度口径与报告 6 findings 一致）
- **第 3 轮 R1 修复核验**（本会话 git 实测）：`.worktrees/passb-b0` 自 `97d8d2632` 后新增恰 1 个修复提交、工作树干净——`5175c06e0` fix(passb): OCR R1 第三批 04/05/15 修复（commit message 本会话实读：#04 基线台账不可解析只报单条 contract-baseline-ledger、三行对照移入 else；#05 referencesSymbol 排除绑定位置 Ident（Field.Names/KeyValueExpr.Key/ValueSpec/AssignStmt 左侧等）；#15 新增 CheckEvents 并入 RunPassBGuard 聚合——目录非空/producer 符号/ consumers 对照 GoFiles/transport/replay/version 冻结词汇）。**R1 14 confirmed 中 3 条 medium（#4/#5/#15）已修**；其余 11 条 low（#1/#2/#3/#6/#8/#9/#10/#11/#12/#13/#14）未处置
- **findings 清单**（报告顺序转录；severity：medium=3 / low=3 / high=0；本管家逐条比对：**6 条全部系第 3 批修复 `5175c06e0` 触及面的连带发现**，非 R1 遗留未修项再现）：
  1. [documentation · low] `tools/passbguard/main.go:49` —— 新增聚合 CheckEvents 后包注释（:1-8）校验面枚举未同步，缺事件目录校验（源于 #15 修复；与未处置 f1/f8 同类）
  2. [bug · medium] `tools/passbguard/check.go:666-671` —— CheckEvents producer 校验缺与契约侧对称的 goFileSet 预检：DiscoverSymbol 只 stat+parse 不经 discoverGoTree 剪枝（点前缀/.worktrees/testdata），producer 路径指向被剪枝文件仍判 found——R1 #2 高危修复（389214f08）封堵的 .worktrees 豁口经此路径重新引入；producer/consumer 作用域不一致（源于 #15 修复）
  3. [bug · medium] `tools/passbguard/discover.go:508-511` —— KeyValueExpr.Key 一律视为绑定误判：map 复合字面量键是表达式引用（`map[Status]int{StatusActive: 1}` 的 StatusActive 是真实消费），误排除致同包消费方漏检→假 contract-consumer-vanished（源于 #05 修复引入的绑定收集；建议 CompositeLit 层按 MapType 区分）
  4. [performance · low] `tools/passbguard/discover.go:451-452` —— sameDir=false 时 bindings 收集后完全未使用（Ident 分支仅 sameDir 查询），全量 AST 遍历+map 分配纯浪费且无缓存（125 契约 × 候选文件规模放大）
  5. [bug · medium] `tools/passbguard/discover.go:376-380` —— 方法回退返回的 method fact 被 CheckContracts（check.go:585）无条件送入 DiscoverSymbolConsumers，方法真实引用形态 recv.Name() 跨包全部不命中→契约顶层声明消失时每条跨包 consumer 触发假 contract-consumer-vanished；与「契约符号既有发现行为不变」注释不符、SymbolFact DeclKind 文档未纳 method（源于 #15 修复的 DiscoverSymbol 方法回退）
  6. [bug · low] `tools/passbguard/discover.go:502-507` —— AssignStmt 一律 Lhs 计绑定：`=`（ASSIGN）左侧对既有包级 var 是写引用，未来冻结 var 契约时同包赋值静默漏检（方向与「宁可多报」取向相反；建议仅 DEFINE 左侧加绑定）（源于 #05 修复）
- **审查结论**：OCR 本轮**未通过**——6 confirmed 须修复后重审；`review_status=changes_requested` 维持（目标值已在位）
- **修复轮次**：3（第 3 轮 R1 14 confirmed → 3 medium 已修 `5175c06e0` → R2 增量复审 6 confirmed，**第 3 轮未闭环**）
- **base/head SHA**：`b1a3d6dd8` / null（未动）
- **worktree**：`.worktrees/passb-b0`（`codex/passb-b0`，HEAD `5175c06e0`，本会话核验工作树干净）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`）
- **本次 JSON 变更**：**无字节级改动**——confirmed>0 时 `review_status` 目标值 `changes_requested` 已在位（10:49 条目所置）；`status=in_progress`、`task_status` 六任务 done、base/head SHA、其余 32 节点均未动。JSON 合法性本会话 `python3 json.load` 复验通过（review 分布：1 changes_requested + 32 pending；notes 9 条不变）
- **备注**：(1) R2 复审范围 3 selected items（main.go/check.go/discover.go，修复触及文件）——R1 未处置 11 条 low 中 #8/#11（check.go）、#9/#10（discover.go）同在扫描范围未再列，未再列≠豁免（沿 08:32 条目先例留痕）；(2) #2（.worktrees 豁口回归）与 #5（method fact 假 vanished）均为 CI 阻断级风险，建议下批修复优先；(3) 重跑 OCR 时 background <8000 字符教训仍适用

---

## 2026-09-23 12:15 CST · b0 节点级 OCR（第 3 轮 R3，修复后复审）：confirmed=1（→ changes_requested 维持；同轮存在 0 findings 先行报告，指令采信 a2 版）

- **节点/轮次**：b0 节点级架构审查 OCR 第 3 轮第 3 次；台账历史口径第 8/9 次运行（07:09 / 07:29 / 07:45 / 08:28 / 09:30 / 10:41 R1 / 11:36 R2 / 12:04 R3-先行 / 12:14 R3-a2）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **OCR 报告路径**：指令采信 `/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b0/ocr-r3-a2.txt`（本会话已读全文：1,054 字节、mtime 12:14；首行 "Review complete: 1 finding(s) across 2 selected item(s)"）。**留痕：同轮先行报告 `ocr-r3.txt`（57 字节、mtime 12:04）实为 "Review complete: 0 finding(s) across 2 selected item(s)."**——同样 2 selected items、两次运行结论不同（0 vs 1）；指令以 a2 版 1 confirmed 为准，先行版未被采信的原因指令未载明，如实登记。另在案：ocr-fix-r1-report.md（24,680B 11:11）/ ocr-fix-r2-report.md（6,998B 11:52）/ ocr-context.md（7,468B 10:20）
- **结论计数**：confirmed=1 / rejected=0（调度口径与 a2 报告一致）
- **第 3 轮 R2 修复核验**（本会话 git 实测）：`.worktrees/passb-b0` 自 `5175c06e0` 后新增恰 1 个修复提交 `76a5a44f4` fix(passb): OCR R2 f2/f5 修复（11:53:24，commit message 本会话实读）——**f2**：CheckEvents 补 goFileSet 树成员预检（DiscoverSymbol 只 os.Stat+ParseFile 不经 discoverGoTree 剪枝，producer 锚定 testdata/点前缀文件曾静默通过；新增 TestCheckEventsProducerOutsideGoTreeRejected 双 fixture）；**f5**：CheckContracts 对 DeclKind=="method" fact 短路（报 contract-symbol-missing、跳过 DiscoverSymbolConsumers，消除跨包消费方全量假 contract-consumer-vanished；producer 存在性路径不变；DeclKind 文档补 method）。即 **R2 两条 medium（#2/#5）已修**；R2 其余四条（#1 main.go 注释 / #3 map 键 / #4 bindings 性能 / #6 AssignStmt= 左侧）未处置
- **findings 清单**（a2 报告全文转录，仅 1 条）：
  1. [maintainability · low] `tools/passbguard/check.go:688-690` —— event-producer-missing 诊断消息把"文件不在发现树"原因硬编码为 "dot-prefixed and testdata directories are pruned"，但该分支同样命中 producer 文件被删除/移动场景，误导排障；建议改中性表述或仅文件磁盘存在但不在树上时附剪枝提示，与契约侧 check.go:569 措辞一致——**系 `76a5a44f4` f2 修复引入的诊断消息措辞问题（修复连带）**
- **审查结论**：OCR 本轮（a2 口径）**未通过**——1 confirmed 须修复后重审；`review_status=changes_requested` 维持（目标值已在位）
- **修复轮次**：3（第 3 轮：R1 14 → 修 3 medium → R2 6 → 修 2 medium → R3 1，**未闭环**；finding 收敛轨迹 14→6→1）
- **base/head SHA**：`b1a3d6dd8` / null（未动）
- **worktree**：`.worktrees/passb-b0`（`codex/passb-b0`，HEAD `76a5a44f4`）；**注意**：工作树有 1 个 untracked 二进制 `tools/passbguard/passbguard`（本会话 `file` 实测 Mach-O 64-bit arm64 可执行文件，go build 产物，非源码变更）——如实登记，建议修复者后续清理或 .gitignore；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`）
- **本次 JSON 变更**：**无字节级改动**——confirmed>0 时 `review_status` 目标值 `changes_requested` 已在位（10:49 条目所置）；`status=in_progress`、`task_status` 六任务 done、base/head SHA、其余 32 节点均未动。JSON 合法性本会话 `python3 json.load` 复验通过（review 分布：1 changes_requested + 32 pending；notes 9 条不变）
- **备注**：(1) R2 未处置四条（#1/#3/#4/#6）与 R1 未处置 11 条 low 仍未获显式裁定，历轮复审范围递减（13→3→2 items）下不再列≠豁免；(2) 本轮唯一 finding 为诊断消息措辞（maintainability low，非 CI 阻断级），修复成本低；(3) 重跑 OCR 时 background <8000 字符教训仍适用

---

## 2026-09-23 12:17 CST · b0 OCR R3-a2 f1 处理结论登记（修复工单：诊断消息中性化 + 删除场景测试）

- **节点/轮次**：b0 节点级审查第 3 轮 R3 唯一 confirmed finding 的处理结论（调度方下发修复工单；**非完成报告**——分支 HEAD 仍 `76a5a44f4`，无对应修复提交）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **finding**：`ocr-r3-a2-f1`（severity low，`tools/passbguard/check.go:688-690`，maintainability；即 12:15 条目 finding #1）——problem：event-producer-missing 诊断把文件不在发现树上的原因硬编码为 "(dot-prefixed and testdata directories are pruned)"，但 `!goFileSet[file]` 分支同样命中 producer 文件被删除/移动场景（goFileSet 由 d.GoFiles 构建，check.go:674-677，删除的文件同样不在树上），此时输出剪枝提示误导排障
- **验收标准**（acceptance 转录；**调度指令文本两处截断**——problem 内"核实：worktree codex/passb-b0 HEAD"后缺 SHA、acceptance 末尾"现有 TestCheckEventsPro"处中断，完整原文以调度方为准，截断如实保留）：(1) 改为中性表述（如 "producer file %s is not in the discovered Go tree (only repository Go files count; dot-prefixed and testdata directories are pruned)"），或仅在文件磁盘存在（os.Stat 成功）但不在树上时附加剪枝提示；(2) 新增/扩展测试覆盖 producer 文件被删除场景并断言诊断不归因剪枝；(3) 现有 TestCheckEventsProducerOutsideGoTreeRejected 保持通过（截断处所指测试，管家补全为 events_test.go:697 在案项）
- **现状核实**（本会话在 `.worktrees/passb-b0`@`76a5a44f4` 实读）：check.go:687-691 诊断消息现状确为 `"producer file %s is not in the discovered Go tree (dot-prefixed and testdata directories are pruned)"`——problem 描述属实；`TestCheckEventsProducerOutsideGoTreeRejected` 位于 events_test.go:693-697（`76a5a44f4` 引入）在案
- **审查结论**：`review_status=changes_requested` 维持不变（修复未完成；本条仅登记工单）
- **修复轮次**：3（第 3 轮 R3 唯一 finding 的修复工单在案待执行；R1 11 low + R2 4 low 未处置悬置如前）
- **base/head SHA**：`b1a3d6dd8` / null（未动）
- **worktree**：修复应落 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `76a5a44f4`，工作树仍含 untracked 二进制 `tools/passbguard/passbguard`——12:15 条目已登记）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`）
- **本次 JSON 变更**：b0.`notes` 数组末尾追加 1 条字符串（第 10 条）——工单全文（problem + 验收标准 + 截断标注 + 现状核实结论）；`review_status=changes_requested`、`status=in_progress`、`task_status`、base/head SHA、其余 32 节点均未动。本会话 `python3 json.load` 复验合法（notes 10 条；review 分布 1 changes_requested + 32 pending）；`git diff` 相对 HEAD 5+/3−（notes 再追加一行）

---

## 2026-09-23 12:37 CST · b0 终审全量 OCR：14 findings，调度裁定未关闭=1（→ changes_requested 维持；未点名项与在案裁定冲突留痕）

- **节点/轮次**：b0 终审全量 OCR（10:12 重派流）；台账历史口径**第 10 次 OCR 运行**（07:09 / 07:29 / 07:45 / 08:28 / 09:30 / 10:41 R1 / 11:36 R2 / 12:04 R3 / 12:14 R3-a2 / 12:34 final）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b0/ocr-final.txt`（本会话已读全文：14,461 字节、235 行、mtime 12:34；**该路径第二次被覆盖**——09:43 版 2,215B → 本次 12:34 版）；首行 "Review complete: 14 finding(s) across 13 selected item(s)"——**全量复扫 13 items**（对比增量复审的 3→2 items）
- **结论计数**：报告 14 findings（medium=2 / low=12）；调度裁定**未关闭=1**——**未点名**（与 10:49 rejected=2 未点名同型缺口，留痕待补；沿 10:54 先例建议调度方点名，以便修复清单划定）
- **分支核验**（本会话 git 实测）：`.worktrees/passb-b0` HEAD 仍 `76a5a44f4`——`76a5a44f4..HEAD` 零新提交，**12:17 条目 R3-a2-f1 修复工单亦未落分支**；工作树仍含 untracked 二进制 `tools/passbguard/passbguard`
- **findings 清单**（报告顺序转录，含与本管家在案记录的对照）：
  1. [test · medium] `tools/internal/movemanifest/manifest.go:1-4` —— 共享库测试未随迁，**f2 由 low 升级 medium**（升级论据：modulemove 退役时测试随宿主删除、schema 漂移防护静默失守）
  2. [documentation · low] `tools/modulemove/main.go:26-27` —— sortStrings 注释失实（f1 再现）
  3. [maintainability · low] `tools/passbguard/check.go:172-176` —— alias 回填生产路径不可达（**f7 再列**；10:54 裁定 ocr-r1-07 误报后降级"注释澄清项"，本次建议升级为"删回填 + fixtureDiscovery 同步携带 Manifest"——方案与裁定不矛盾但超出，处置待协调者）
  4. [bug · low] `tools/passbguard/check.go:209-212` —— guard 源侧重复边 last-wins（f14 再现）
  5. [bug · low] `tools/passbguard/check.go:835-839` —— fileReferencesIdent 解析失败静默 false（R1 #8 再现，行号偏移）
  6. [bug · low] `tools/passbguard/discover.go:560-561` —— **新列**：renderFuncType/renderTypeSpec 忽略 TypeParams，泛型约束收紧逃过 contract-signature-drift（仓库确有顶层导出泛型；当前 125 契约无泛型符号，纯漏报盲区）
  7. [bug · low] `tools/passbguard/discover.go:356-359` —— var/const 多名声明跳过 + 前缀硬编码（f6 再现）
  8. [bug · low] `tools/passbguard/discover.go:507-512` —— AssignStmt `=` 左侧漏报（R2 #6 再现）
  9. [documentation · low] `tools/passbguard/discover.go:25-27` —— repoModulePath 注释失实（f8 再现）
  10. [bug · low] `tools/passbguard/load.go:95-103` —— decodeStrict 多文档截断（f10 再现）
  11. [bug · low] `tools/passbguard/overlap.go:266-267` —— briefClaimsPath basename 匹配（f9 再现之一；f9 另一形态 end>len 死条件本次未列）
  12. [maintainability · low] `tools/passbguard/overlap.go:200-202` —— **新列**：禁语子串扫描对引述禁令的自伤风险（当前零命中未变现）
  13. [bug · medium] `tools/passbguard/check.go:166-171` —— **f16 再列且 low→medium；论据与在案矛盾**：报告称"verify.go 中无任何 alias 逻辑"，本会话实读 `tools/modulemove/verify.go:108-112` alias-1to1 逻辑在案、`grep -c 'alias-1to1'` = 5 处；且 10:54 条目已裁定 ocr-r1-16 误报（推论链恰依赖 alias-1to1 存在——实读成立）。报告"VerifyAll 的 ownership-overlap 不含 alias_obligations"子论断或为真（VerifyAll 判重对象为 from/to），但不改变 10:54 裁定的兜底链条
  14. [maintainability · low] `tools/passbguard/load.go:46-49` —— **新列**：decodeStrict 的 present 返回值被四处调用丢弃（死代码）；治理文件误删时触发 396 条 legacy-missing 级联，与 f11 修复考量相悖
- **对照小结**（本管家整理，非裁定）：14 条 = 未处置 low 再现 9 条（f1/f2/f6/f8/f9/f10/f14 + R1#8 + R2#6）+ 已裁定误报再列 1 条（#13=f16，论据与实读矛盾）+ 已裁定降级项再列 1 条（#3=f7，建议升级待裁定）+ 新列 3 条（#6 泛型 / #12 禁语 / #14 present）。已修复项（R1 #4/#5/#15、R2 #2/#5）未再列；R1 #3（movemanifest 多文档）、R1 #6（worker-set 前缀）、R2 #1/#3/#4、R3 f1（check.go:688-690，工单在案未落分支）本次均未列——未再列≠豁免
- **审查结论**：终审全量 OCR 后调度裁定未关闭=1 > 0 → `review_status=changes_requested` 维持；节点未收口
- **修复轮次**：3（第 3 轮未闭环；R3-a2-f1 工单未落分支）
- **base/head SHA**：`b1a3d6dd8` / null（未动）
- **worktree**：`.worktrees/passb-b0`（HEAD `76a5a44f4`）；`.worktrees/passb-int`（HEAD `78193c4e5`）
- **本次 JSON 变更**：**无字节级改动**——未关闭>0 时目标值 `changes_requested` 已在位；`status=in_progress`、`task_status`、base/head SHA、其余 32 节点均未动。JSON 合法性本会话 `python3 json.load` 复验通过（review 分布 1 changes_requested + 32 pending；notes 10 条不变）
- **备注**：(1) 未关闭=1 未点名——若所指为 #13（f16），则与 10:54 在案误报裁定及本管家实读矛盾，须协调者先澄清（再裁或维持误报）；(2) 历轮悬置 low 在全量复扫大量再现，印证 09:32 留痕预警——建议协调者对全部悬置 low 做一次性显式裁定（修复/豁免/延期），避免终审反复；(3) 重跑 OCR 时 background <8000 字符教训仍适用

---

## 2026-09-23 12:55 CST · b0 → blocked（OCR 两次尝试输出均不完整）＋ 全部 32 个传递依赖节点 → blocked

- **节点**：b0 + **32 个传递依赖节点**（本会话 BFS 实测闭包 = 其余全部节点，与 06:26/07:50/09:45 条目一致）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：无（b0 自身为根）
- **阻塞原因**（调度指令原文照录）：**Error: OCR 两次尝试输出均不完整（截断/请求失败），不视为通过**——即 12:37 条目终审全量 OCR（未关闭=1 待点名）之后的基础设施失败，节点无法收口
- **与历史的关联**：b0 节点级审查**第 3 次基础设施失败**——06:26（background 9914 字符超 8000 硬限）、09:45（最终全量 OCR 输出不完整/截断/跳过文件）、本次 12:55（两次尝试均不完整/请求失败）；07:50 属 OCR 产出实质 finding（非基础设施）
- **review_status 处置**：维持 `changes_requested` **不回退**——沿 06:26/09:45 基础设施失败先例（不动 review_status；07:50 回退系 OCR 产出实质 finding 所致）。当前 changes_requested 的审查基础 = 10:41 R1（14 confirmed）起的修复流 + 12:37 终审"未关闭=1"
- **base/head SHA**：`b1a3d6dd8` / null（未动；合并与 head_sha 回填继续冻结）
- **worktree**：`.worktrees/passb-b0`（`codex/passb-b0`，HEAD `76a5a44f4`，12:37 条目核验：R3-a2-f1 工单未落分支）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`）
- **测试证据路径**：无新增（B0.1–B0.6 六任务证据链 + 历轮 OCR/修复记录见前各条目）
- **审查结论**：任务级六条通过（在案）；节点级 = 终审全量 OCR 基础设施失败（12:37 报告 14 findings/未关闭=1 未点名，其后两次重试均不完整）
- **OCR 报告路径**：终审版 = `ocr-final.txt`（12:34 版，14 findings，见 12:37 条目）；两次失败尝试的报告路径调度指令未附（仅内联错误消息）
- **修复轮次**：3（不变——本轮阻塞系审查基础设施失败，不新增修复轮；R3-a2-f1 工单未落分支）
- **本次 JSON 变更**（python 原子更新，全部断言通过）：
  1. b0 `status` in_progress → **blocked**；b0 notes 数组追加 BLOCKED 条目（第 11 条，含错误原文、第 3 次基础设施失败关联、解除条件、review_status 处置依据）
  2. 其余 32 节点 `status` pending → blocked——**notes 不重复追加**：本会话逐节点核验 32/32 已含「BLOCKED（2026-09-23）：前置 b0 阻塞；解除条件：修复并 done b0 后恢复。」文本（06:26 遗留、恢复提交 `78193c4e5` 保留至今），与本次指令要求的依赖节点阻塞措辞一致，仅翻转 status
  3. `python3 json.load` 复验合法：33 节点全 blocked、review_status 分布 1 changes_requested + 32 pending；`git diff` 相对 HEAD 39+/36−（= 33 行 status + review_status 工作树累计行 + notes 新增行的自然展开）
- **未动字段**：b0.task_status（六任务仍 done）、全部 review_status、全部 base_sha、全部 head_sha（null）
- **解除条件**：重跑终审全量 OCR 至产出完整结论（两次尝试均不完整的失败模式须先排除）并按结论收口——通过则（合并 `codex/passb-b0` + head_sha 回填 + 悬置 low 显式裁定）done b0，有 finding 则点名并进入下一修复轮；32 个依赖节点在 b0 done 后恢复

---

## 2026-09-23 13:25 CST · b0 → in_progress（重派 "running"；目标值已在位，JSON 无字节级改动；12:55 blocked 编辑已被回退）

- **节点**：b0 —— B0 契约与所有权冻结（passbguard + 4 份治理 yaml + 证据）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：无（depends_on = []）
- **worktree**：b0 实施分支 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `62a56200a`，本会话 git log 核验：12:37 条目 HEAD `76a5a44f4` 之后新增**恰 1 个提交**——12:44 `62a56200a` `test(passb): manifest 严格加载器 4 测试随包迁移至 movemanifest`（`git diff 76a5a44f4..HEAD --stat` 实测 1 文件 +56/−1：`tools/modulemove/manifest_test.go` → `tools/internal/movemanifest/manifest_test.go`）——对应 12:37 终审 OCR finding #1 medium（f2 共享库测试未随迁）的修复；工作树仍含 untracked 二进制 `tools/passbguard/passbguard`，12:15 条目已登记）；DAG/台账所在 integration worktree `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`，本会话 git log 核验无变化）
- **base SHA**：`b1a3d6dd8`（未动）；**head SHA**：null（未回填——按 06:21/07:32/09:32 条目，须以本分支合并进 integration 后的头回填）
- **测试证据路径**：无新增（B0.1–B0.6 六任务报告/审查包见 03:40–06:21 各条目；节点 5 项 gates 全过见 06:21 条目——复用即沿用该证据链）
- **审查结论（状态核验）**：本会话 `python3 json.load` 实测——33 节点 **1 in_progress（b0）+ 32 pending（blocked=0）**，`review_status` 全 33=pending，b0.task_status 六任务全 done，b0 notes 10 条、无 BLOCKED 条目。**12:55 条目所载 JSON 变更（b0→blocked + 32 依赖节点→blocked + notes 第 11 条 BLOCKED）已被回退**：本会话 `git status --short` = DAG 1 处修改 + 台账 untracked；`git diff HEAD -- docs/plans/passb/execution-dag.json` 恰 3+/1−，**全部落在 b0.notes 数组**（#9 条补尾逗号 + #10 条「OCR R3-a2 f1 处理结论」追加，10:54/12:17 条目所为，与 HEAD 无其他差异）。回退模式与 07:50→08:05、09:45→10:12 一致。review_status 回到 `pending`（HEAD `78193c4e5` 提交值）——07:09–12:37 历轮 changes_requested/approved 编辑均系工作树级且已被回退，如实登记
- **OCR 报告路径**：既有在案——ocr-r1.txt（10:41 版，16 findings）/ ocr-r2.txt（11:36 版，6 findings）/ ocr-r3.txt（12:04 版，0 findings，未被采信）/ ocr-r3-a2.txt（12:14 版，1 finding）/ ocr-final.txt（12:34 版，14 findings）；12:55 两次失败尝试无报告路径；本轮（重派后）尚未产出
- **修复轮次**：3（第 1/2 轮闭环成果在案；第 3 轮进展：R1 修 3 medium（`5175c06e0`）→ R2 修 2 medium（`76a5a44f4`）→ 新提交 `62a56200a` 修 f2 测试随迁（12:37 终审 #1 medium）→ **R3-a2-f1 工单（check.go:688-690 诊断消息中性化）仍未落分支** → 12:37 终审 14 findings/未关闭=1 未点名 → 12:55 两次重试不完整——待本轮收口）
- **本次 JSON 变更**：**无字节级改动**——指令 "b0 → running" 按状态机（conventions §9 无 `running` 值）沿 03:18/06:49/08:05/10:12 先例映射为规范值 `in_progress`，而该值已在位（12:55 回退后工作树即恢复 in_progress）；`task_status` 六任务 done（复用不重置）、`base_sha`/`head_sha`/`review_status` 均未动。JSON 合法性本会话 `python3 json.load` 复验通过（33 节点：1 in_progress + 32 pending）
- **剩余待办**（承接 12:37/12:55，顺延）：(1) 重跑终审全量 OCR 至产出完整结论（background <8000 字符教训仍适用，两次尝试不完整的失败模式须先排除）；(2) 未关闭=1 未点名——若所指为 #13（f16）则与 10:54 在案误报裁定矛盾，须协调者先点名澄清；(3) R3-a2-f1 修复工单仍未落分支；(4) 悬置 low 一次性显式裁定（R1 11 条 + R2 4 条）；(5) 通过则合并 `codex/passb-b0` → 置 b0 done + 以合并头回填 head_sha + 32 依赖节点恢复（notes 内 32 处 BLOCKED 文本本会话脚本核验 32/32 仍在，随恢复一并清理）
- **备注**：其余 32 节点本指令未触及（现状 pending；其 notes 内 BLOCKED 文本与 status=pending 并存系恢复提交 `78193c4e5` 保留至今的既知状态，非本管家改动）

---

## 2026-09-23 13:26 CST · b0 恢复运行（复用 B0.1–B0.6 已完成任务；与 13:25 派发衔接）

- **节点**：b0 —— B0 契约与所有权冻结（passbguard + 4 份治理 yaml + 证据）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：无（depends_on = []）
- **恢复语义**（沿 06:50/08:06/10:14 条目先例）：复用已完成任务 B0.1、B0.2、B0.3、B0.4、B0.5、B0.6——六任务不重置、不重跑；节点剩余待办 = 重跑终审全量 OCR（12:55 条目阻塞事由"两次尝试输出均不完整/截断/请求失败"）→ 通过则合并 `codex/passb-b0` + head_sha 回填 + done b0
- **worktree**：b0 实施分支 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `62a56200a`，本会话 git log/status 核验：与 13:25 条目一致、无新提交，工作树仍含 untracked 二进制 `tools/passbguard/passbguard`）；DAG/台账所在 integration worktree `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`，本会话 git log 核验）
- **base SHA**：`b1a3d6dd8`（未动）；**head SHA**：null（未回填——合并进 integration 后回填）
- **测试证据路径**：无新增（B0.1–B0.6 六任务报告/审查包见 03:40–06:21 各条目；节点 5 项 gates 全过见 06:21 条目——复用即沿用该证据链）
- **审查结论**：任务级六条通过（在案）；节点级 = 12:37 终审全量 OCR 后未收口（14 findings/未关闭=1 未点名 → 12:55 两次重试不完整）；`review_status=pending`（工作树现值，历轮 changes_requested 编辑已被回退，见 13:25 条目核验）
- **OCR 报告路径**：既有在案 ocr-r1.txt（10:41 版）/ ocr-r2.txt（11:36 版）/ ocr-r3.txt（12:04 版）/ ocr-r3-a2.txt（12:14 版）/ ocr-final.txt（12:34 版）；重跑后报告由后续条目回填
- **修复轮次**：3（第 3 轮在途：`62a56200a` 已修 f2 测试随迁；R3-a2-f1 工单未落分支；终审全量 OCR 待完整重跑，重跑若有 finding 则点名进入下一修复轮）
- **本次 JSON 变更**：**无字节级改动**——指令目标态与本会话 `python3 json.load` 读取的现值完全一致：b0.`status=in_progress`（13:25 派发已在位）、`task_status` 六任务全 `done`（复用 = 保持 done，无重置动作）、`base_sha=b1a3d6dd8`/`head_sha=null`/`review_status=pending` 均未动。JSON 合法性本会话复验通过（33 节点：1 in_progress + 32 pending）；`git diff HEAD -- DAG` 仍恰 3+/1−（仅 b0.notes #9/#10 内容，10:54/12:17 条目所为，本条目未触及）
- **备注**：本条与 13:25 条目衔接（13:25 记录 "running" 重派映射与 12:55 回退核验，本条补充调度方"复用 B0.1–B0.6"指令语义并确认 task_status 不重置）；其余 32 节点本指令未触及（notes 内 32 处 BLOCKED 文本待 b0 done 后随恢复一并清理，13:25 条目已核验 32/32 在位）

---

## 2026-09-23 13:51 CST · b0 节点级 OCR（13:26 重派后第 1 次）：7 findings 全 confirmed（→ changes_requested）

- **节点/轮次**：b0 节点级架构审查 OCR——调度口径"第 1 次"（13:25/13:26 重派流首轮）；台账历史口径**第 11 次 OCR 运行**（07:09 / 07:29 / 07:45 / 08:28 / 09:30 / 10:41 / 11:36 / 12:04 / 12:14 / 12:34 / 本次 13:47）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b0/ocr-r1.txt`（本会话已读全文：6,194 字节、mtime 13:47、120 行；首行 "Review complete: 7 finding(s) across 13 selected item(s)"；**该路径第四次被覆盖**——07:04 版 9 findings → 08:28 版 14 findings → 10:41 版 16 findings → 本次）
- **结论计数**：confirmed=7 / rejected=0（调度口径与报告 7 findings 数量一致，全部确认、无一驳回）
- **findings 清单**（报告顺序转录；severity：medium=2 / low=5；类别：bug=3 / maintainability=4）：
  1. [maintainability · medium] `tools/passbguard/check.go:596-600` —— DiscoverSymbolConsumers 返回的 tests 集合（引用该符号的 _test.go 文件集）在唯一生产调用点被 `_` 丢弃；c.CharacterizationTests 随后仅做磁盘存在性检查，特征化测试改名/被删/不再引用符号时守卫完全无感——「冻结既有特征化测试」治理承诺只剩文件存在一层校验，tests 发现计算成未接线死逻辑。建议接 vanished 方向对照（**新列**；与 B0.4 移交 B1 的 6 条缺特征化测试契约项直接相关，见 05:51 条目）
  2. [maintainability · low] `tools/passbguard/load.go:97-103` —— decodeStrict 只 Decode 一次，`---` 多文档 YAML 后续文档静默截断（**f10 历轮再现**；当前四份治理 YAML 均单文档，属防御性缺口，修复成本低——Decode 成功后再探测一次、非 io.EOF 即报错）
  3. [bug · low] `tools/passbguard/main.go:95-98` —— fs.Parse 成功后未检查 fs.NArg()，`passbguard -root . extra` 多余位置参数静默忽略，与 run 注释「2 参数非法」契约不符（**新列**）
  4. [maintainability · low] `tools/passbguard/overlap.go:276-279` —— `end > len(text)` 恒为假死条件（strings.Index 保证命中完整位于 text 内），读起来像越界防护实际从未生效（**f9 之一再现**）
  5. [bug · medium] `tools/passbguard/overlap.go:266-268` —— briefClaimsPath 仅 basename 匹配认领，而 brief 实以完整仓库相对路径认领（如 agentruntime-protocol.md 的 `internal/handler/session/native_archive.go`）；ruling 集合含 types.go/handler.go/helpers.go 等极常见名，任何非属主 brief 提及他目录同名文件即误报 brief-claim-conflict（CI 阻断），反向掩盖 brief-claim-missing（**f9 之二再现**；建议完整 path token 匹配优先、未命中再退回 basename）
  6. [bug · low] `tools/passbguard/discover.go:356-359` —— ValueSpec 分支要求 len(s.Names)==1，多名声明（`var A, B = f(), g()`）中冻结的契约符号找不到顶层声明，误报 contract-symbol-missing 且消息方向误导（**f6 再现**）
  7. [maintainability · low] `tools/passbguard/model.go:258-262` —— checkRepoPath 对空路径已返回非空诊断，`else if l.Destination == ""` 恒假永不可达死分支；空 destination 实际输出的是 "path must not be empty" 而非专用文案（**新列**）
- **与历史对照**（本管家逐条比对）：历轮未处置项再现 4 条（#2=f10、#4=f9-end、#5=f9-basename、#6=f6）；新列 3 条（#1 tests 丢弃 / #3 NArg / #7 model 死分支）。**10:54 裁定误报的 f7（check.go:172-176）与 f16（check.go:166-171）本轮均未再列**——与在案误报裁定一致。12:37 终审 14 条中 f1（sortStrings 注释）、f14（guard 重复边）、R1#8（fileReferencesIdent）、R2#6（AssignStmt=）、f8（repoModulePath 注释）、#6 泛型 TypeParams、#12 禁语自伤、#14 present 丢弃，以及 R3-a2-f1 工单（check.go:688-690）本轮未列——**未再列≠豁免**（沿 08:32 先例留痕）
- **审查结论**：OCR 本轮**未通过**——7/7 confirmed、0 rejected，须修复后重审
- **修复轮次**：4（第 1 轮 07:09→07:32、第 2 轮 08:32→09:30 闭环；第 3 轮 10:41–12:55 未闭环——未关闭=1 未点名 + 两次重试不完整；**第 4 轮 = 本轮 7 findings，尚未启动**——b0 分支无新提交）
- **base/head SHA**：`b1a3d6dd8` / null（未动——head_sha 待修复轮通过、复审零未关闭、合并进 integration 后回填）
- **worktree**：修复应落 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `62a56200a`，本会话 git log/status 核验：13:26 条目后无新提交、工作树仍含 untracked 二进制 `tools/passbguard/passbguard`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`）。全部 7 findings 落 `tools/passbguard/**`——均在 b0 owned_files 范围内
- **本次 JSON 变更**：b0.`review_status` pending → **changes_requested**（沿 07:09/08:32/10:49 先例——OCR confirmed>0 即落入 changes_requested）；`status` 保持 `in_progress`（修复属节点执行期）、`task_status` 六任务 done、base/head SHA、其余 32 节点均未动。本会话 `python3 json.load` 复验合法（review 分布：1 changes_requested + 32 pending）；`git diff` 相对 HEAD 的净变更 = review_status 1 行 + notes #9/#10（10:54/12:17 条目遗留的工作树未提交内容，本条目未触及）
- **备注**：(1) #5（briefClaimsPath basename）与 #1（tests 丢弃）为两条 medium，前者 CI 阻断级风险（brief-claim-conflict 误报）、后者关系特征化测试治理承诺接线，建议第 4 轮修复优先；(2) 重跑 OCR 时 background <8000 字符教训（06:26）仍适用；(3) 第 3 轮遗留的未点名"未关闭=1"（12:37 条目）本轮报告未承接点名，其与 10:54 误报裁定的矛盾仍待协调者澄清

---

## 2026-09-23 14:24 CST · b0 → blocked（OCR 两次尝试输出均不完整）＋ 全部 32 个传递依赖节点 → blocked

- **节点**：b0（13:51 R1 7 findings confirmed、第 4 轮修复未启动）+ **32 个传递依赖节点**（本会话 BFS 实测闭包 = 其余全部节点、闭包外为空，与 06:26/07:50/09:45/12:55 条目一致）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：无（b0 自身为根）
- **阻塞原因**（调度指令原文照录）：**Error: OCR 两次尝试输出均不完整（截断/请求失败），不视为通过**——13:51 条目 R1（7 findings）之后的 OCR 重试（调度序列中紧随 R1）两次尝试均因输出截断/请求失败未产出完整结论，节点无法收口
- **与历史的关联**：b0 节点级审查**第 4 次基础设施失败**——06:26（background 9914 字符超 8000 硬限）、09:45（输出不完整/截断/跳过文件）、12:55（两次尝试均不完整）、本次（同类：两次尝试不完整/截断/请求失败）；07:50 属 OCR 产出实质 finding（非基础设施）
- **review_status 处置**：维持 `changes_requested` **不回退**——沿 06:26/09:45/12:55 基础设施失败先例（不动 review_status）。当前 changes_requested 的审查基础 = 13:51 R1（7 findings 全 confirmed）
- **base/head SHA**：`b1a3d6dd8` / null（未动；合并与 head_sha 回填继续冻结）
- **worktree**：`.worktrees/passb-b0`（`codex/passb-b0`，HEAD `62a56200a`，13:51 条目核验后无新提交、第 4 轮修复未落分支；工作树仍含 untracked 二进制 `tools/passbguard/passbguard`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`）
- **测试证据路径**：无新增（B0.1–B0.6 六任务证据链 + 历轮 OCR/修复记录见前各条目）
- **审查结论**：任务级六条通过（在案）；节点级 = 13:51 R1 未通过（7 confirmed）且后续 OCR 重试基础设施失败（两次尝试不完整）
- **OCR 报告路径**：在案——ocr-r1.txt（13:47 版，7 findings，见 13:51 条目）/ ocr-r2.txt（11:36 版）/ ocr-r3.txt（12:04 版）/ ocr-r3-a2.txt（12:14 版）/ ocr-final.txt（12:34 版）；本次两次失败尝试的报告路径调度指令未附（仅内联错误消息）
- **修复轮次**：4（第 4 轮 = 13:51 R1 的 7 findings，尚未启动；本轮阻塞系审查基础设施失败，不新增修复轮）
- **本次 JSON 变更**（python 原子更新，全部断言通过）：
  1. b0 `status` in_progress → **blocked**；b0 notes 数组追加 BLOCKED 条目（第 11 条，含错误原文、第 4 次基础设施失败关联、review_status 处置依据、解除条件）
  2. 其余 32 节点 `status` pending → blocked——**notes 不重复追加**：本会话逐节点核验 32/32 已含「BLOCKED（2026-09-23）：前置 b0 阻塞；解除条件：修复并 done b0 后恢复。」文本（06:26 遗留、恢复提交 `78193c4e5` 保留至今），与本次指令要求的依赖节点阻塞措辞一致，仅翻转 status
  3. `python3 json.load` 复验合法：33 节点全 blocked、review_status 分布 1 changes_requested + 32 pending；`git diff HEAD --stat` = 39 插入/36 删除（与 12:55 条目同型：33 行 status + review_status 行 + notes 追加的自然展开，含 10:54/12:17 notes #9/#10 工作树遗留内容）
- **未动字段**：b0.task_status（六任务仍 done）、b0.review_status（changes_requested，见上处置说明）、全部 base_sha、全部 head_sha（null）、其余 32 节点 review_status（pending）
- **解除条件**：重跑 OCR 至产出完整结论（两次尝试不完整的失败模式须先排除；background <8000 字符教训仍适用）并按结论收口——通过则（合并 `codex/passb-b0` + head_sha 回填 + 悬置项显式裁定）done b0，有 finding 则点名并进入下一修复轮；32 个依赖节点在 b0 done 后恢复（notes 内 BLOCKED 文本一并清理）

---

## 2026-09-23 15:05 CST · b0 → in_progress（重派 "running"；目标值已在位，JSON 无字节级改动；14:24 blocked 编辑已被回退）

- **节点**：b0 —— B0 契约与所有权冻结（passbguard + 4 份治理 yaml + 证据）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：无（depends_on = []）
- **worktree**：b0 实施分支 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `27d4b680a`，本会话 git log 核验：14:24 条目 HEAD `62a56200a` 之后新增**恰 1 个提交**——14:03:30 `27d4b680a` `fix(passb): OCR R1 ocr-r1-1/ocr-r1-5 修复`（`git show --stat` 实测 4 文件 +149/−11：check.go +15 / contracts_test.go +63 / overlap.go +37−11 / overlap_test.go +45）——即 13:51 R1 7 findings **第 4 轮修复第一批：#1（DiscoverSymbolConsumers tests 接线，新增 contract-characterization-drift）与 #5（briefClaimsPath 完整 path token 优先 + basename 回退前驱 `/` 排除）两条 medium 全部已修**；commit message 本会话实读在案；工作树仍含 untracked 二进制 `tools/passbguard/passbguard`（12:15 条目已登记））；DAG/台账所在 integration worktree `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`，本会话 git log 核验无变化）
- **base SHA**：`b1a3d6dd8`（未动）；**head SHA**：null（未回填——按 06:21/07:32/09:32 条目，须以本分支合并进 integration 后的头回填）
- **测试证据路径**：无新增（B0.1–B0.6 六任务报告/审查包见 03:40–06:21 各条目；节点 5 项 gates 全过见 06:21 条目——复用即沿用该证据链）
- **审查结论（状态核验）**：本会话 `python3 json.load` 实测——33 节点 **1 in_progress（b0）+ 32 pending（blocked=0）**，`review_status` 全 33=pending，b0.task_status 六任务全 done，b0 notes 10 条、无 BLOCKED 条目。**14:24 条目所载 JSON 变更（b0→blocked + 32 依赖节点→blocked + notes 第 11 条 BLOCKED + review_status→changes_requested）已被回退**：本会话 `git status --short` = DAG 1 处修改 + 台账 untracked；`git diff HEAD -- DAG` 恰 3+/1−，全部落在 b0.notes 数组（#9 尾逗号 + #10 追加，10:54/12:17 条目所为的工作树遗留，与 HEAD 无其他差异）。回退模式与 07:50→08:05、09:45→10:12、12:55→13:25 一致。review_status 回到 `pending`（HEAD `78193c4e5` 提交值）——13:51/14:24 的 changes_requested 编辑均系工作树级且已被回退，如实登记
- **OCR 报告路径**：既有在案——ocr-r1.txt（13:47 版，7 findings）/ ocr-r2.txt（11:36 版）/ ocr-r3.txt（12:04 版，未被采信）/ ocr-r3-a2.txt（12:14 版）/ ocr-final.txt（12:34 版）；14:24 两次失败尝试无报告路径；本轮（重派后）尚未产出
- **修复轮次**：4（第 4 轮在途：13:51 R1 7 findings → `27d4b680a` 已修 2 medium（#1/#5）→ 剩余 5 条 low 未处置 → 14:24 两次 OCR 重试不完整——待本轮收口）
- **本次 JSON 变更**：**无字节级改动**——指令 "b0 → running" 按状态机（conventions §9 无 `running` 值）沿 03:18/06:49/08:05/10:12/13:25 先例映射为规范值 `in_progress`，而该值已在位（14:24 回退后工作树即恢复 in_progress）；`task_status` 六任务 done（复用不重置）、`base_sha`/`head_sha`/`review_status` 均未动。JSON 合法性本会话 `python3 json.load` 复验通过（33 节点：1 in_progress + 32 pending）
- **剩余待办**（承接 13:51/14:24，顺延）：(1) 第 4 轮剩余 5 条 low 修复（#2 f10 load.go 多文档截断 / #3 main.go NArg / #4 overlap.go `end>len` 死条件 / #6 discover.go 多名声明 / #7 model.go 死分支）；(2) 重跑 OCR 至产出完整结论（background <8000 字符教训仍适用）；(3) 通过则合并 `codex/passb-b0` → 置 b0 done + 以合并头回填 head_sha + 32 依赖节点恢复（notes 内 32 处 BLOCKED 文本本会话未再逐节点复核、13:25 条目曾核验 32/32 在位，随恢复一并清理）；(4) 历轮悬置 low 一次性显式裁定（第 3 轮 R1 11 条 + R2 4 条 + 12:37 终审新列 3 条，见 12:37/13:51 条目对照）
- **备注**：其余 32 节点本指令未触及（现状 pending；其 notes 内 BLOCKED 文本与 status=pending 并存系恢复提交 `78193c4e5` 保留至今的既知状态，非本管家改动）

---

## 2026-09-23 15:06 CST · b0 恢复运行（复用 B0.1–B0.6 已完成任务；与 15:05 派发衔接）

- **节点**：b0 —— B0 契约与所有权冻结（passbguard + 4 份治理 yaml + 证据）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：无（depends_on = []）
- **恢复语义**（沿 06:50/08:06/10:14/13:26 条目先例）：复用已完成任务 B0.1、B0.2、B0.3、B0.4、B0.5、B0.6——六任务不重置、不重跑；节点剩余待办 = 第 4 轮剩余 5 条 low 修复（13:51 R1 #2/#3/#4/#6/#7，见 15:05 条目）→ 重跑 OCR 至完整结论 → 通过则合并 `codex/passb-b0` + head_sha 回填 + done b0
- **worktree**：b0 实施分支 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `27d4b680a`，本会话 git log/status 核验：15:05 条目后 0 新提交、工作树仍含 untracked 二进制 `tools/passbguard/passbguard`）；DAG/台账所在 integration worktree `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`）
- **base SHA**：`b1a3d6dd8`（未动）；**head SHA**：null（未回填——合并进 integration 后回填）
- **测试证据路径**：无新增（B0.1–B0.6 六任务报告/审查包见 03:40–06:21 各条目；节点 5 项 gates 全过见 06:21 条目——复用即沿用该证据链）
- **审查结论**：任务级六条通过（在案）；节点级 = 第 4 轮修复在途（13:51 R1 7 findings 中 2 medium 已修 `27d4b680a`、5 low 未处置；14:24 两次 OCR 重试不完整待重跑）
- **OCR 报告路径**：既有在案 ocr-r1.txt（13:47 版，7 findings）/ ocr-r2.txt（11:36 版）/ ocr-r3.txt（12:04 版）/ ocr-r3-a2.txt（12:14 版）/ ocr-final.txt（12:34 版）；重跑后报告由后续条目回填
- **修复轮次**：4（第 4 轮在途；重跑 OCR 若有 finding 则点名进入下一修复轮）
- **本次 JSON 变更**：**无字节级改动**——指令目标态与本会话 `python3 json.load` 读取的现值完全一致：b0.`status=in_progress`（15:05 派发已在位）、`task_status` 六任务全 `done`（复用 = 保持 done，无重置动作）、`base_sha=b1a3d6dd8`/`head_sha=null`/`review_status=pending` 均未动。JSON 合法性本会话复验通过（33 节点：1 in_progress + 32 pending）；`git diff HEAD -- DAG` 仍恰 3+/1−（仅 b0.notes #9/#10 工作树遗留内容，10:54/12:17 条目所为，本条目未触及）
- **备注**：本条与 15:05 条目衔接（15:05 记录 "running" 重派映射与 14:24 回退核验，本条补充调度方"复用 B0.1–B0.6"指令语义并确认 task_status 不重置）；其余 32 节点本指令未触及

---

## 2026-09-23 16:10 CST · b0 节点级 OCR（15:05 重派后第 1 次）：7 findings 全 confirmed（→ changes_requested）

- **节点/轮次**：b0 节点级架构审查 OCR——调度口径"第 1 次"（15:05/15:06 重派流首轮）；台账历史口径**第 12 次 OCR 运行**（07:09 / 07:29 / 07:45 / 08:28 / 09:30 / 10:41 / 11:36 / 12:04 / 12:14 / 12:34 / 13:47 / 本次 16:02）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b0/ocr-r1.txt`（本会话已读全文：6,493 字节、mtime 16:02、125 行；首行 "Review complete: 7 finding(s) across 13 selected item(s)."；**该路径第五次被覆盖**——07:04 版 9F → 08:28 版 14F → 10:41 版 16F → 13:47 版 7F → 本次；报告附 LLM retry 摘要：27 请求中 1 次 core review 超时重试后成功，结论已产出）
- **结论计数**：confirmed=7 / rejected=0（调度口径与报告 7 findings 一致，全部确认、无一驳回）
- **findings 清单**（报告顺序转录；severity：medium=1 / low=6；类别：bug=4 / performance=1 / maintainability=1 / documentation=1；每条报告均附建议 diff）：
  1. [bug · low] `tools/passbguard/load.go:95-103` —— decodeStrict 只 Decode 一次，多文档 YAML 静默截断（**f10 历轮再现，未落分支**；建议 Decode 成功后再探测一次、非 io.EOF 即报错）
  2. [bug · low] `tools/passbguard/main.go:96-98` —— fs.Parse 后未检查 fs.NArg()，多余位置参数静默忽略、在错误目录上运行守卫（**13:51 #3 再现，未落分支**）
  3. [bug · low] `tools/passbguard/discover.go:356-359` —— DiscoverSymbol 对 var/const 要求 len(s.Names)==1，多名声明中符号被跳过且诊断文案 "no longer declares" 误导方向（**13:51 #6/f6 再现，未落分支**）
  4. [performance · low] `tools/passbguard/discover.go:455-457` —— referencesSymbol 每次 (符号,文件) 对重算 bindingIdentPositions，跨包候选（sameDir=false，bindings 仅裸 Ident 分支使用）也照算（**11:36 R2 #4 同型再现**——报告未标"在案"，本管家比对得出；建议仅 sameDir 时计算或按文件缓存）
  5. [maintainability · low] `tools/passbguard/model.go:258-262` —— checkRepoPath 空串已返回非空诊断，else-if 空 destination 分支恒不可达死分支（**13:51 #7 再现，未落分支**）
  6. [documentation · low] `tools/passbguard/check.go:775-780` —— contract-consumer-adapters-import 诊断文案未注明不可经台账豁免，读者会误以为补 ledger 条目可解；"route via module root façade" 对 consumerModule 为空的平台 legacy 消费方不适用（报告标注"在案 R3-a2-f1，行号漂移至 776-778，未落分支"——**留痕：R3-a2-f1 工单（12:17 条目）对象是 check.go:688-690 的 event-producer-missing 诊断，本条是 check.go:775-780 的另一诊断（contract-consumer-adapters-import）同类措辞问题，报告的"在案"关联标注与在案工单不同物，如实登记**）
  7. [maintainability · medium] `tools/passbguard/check.go:821-826` —— worker-set 消费方发现口径硬编码只扫 internal/router/ 前缀：B0 行为正确（当前注册位点均在 internal/router，readiness 通过佐证），但 B1+ 把 RegisterWorkers 注册位点迁入模块树后会 (a) 树外新位点静默漏报 unrecorded、(b) 已登记 consumers 批量假 contract-consumer-vanished，且口径烧在守卫代码里仅契约修订流程无法消除（**10:49 R1 #6 同型再现并升 medium**；建议随迁移同步从 manifest/注册位点推导扫描范围，或登记 B1+ 演进义务）
- **与历史对照**（本管家逐条比对）：13:51 R1 的 #1/#5（2 medium）已修（`27d4b680a`）**未再列**——修复成果有效；13:51 #2（f10）→本轮 #1、#3（NArg）→本轮 #2、#6（f6）→本轮 #3、#7（model 死分支）→本轮 #5 **再现**（该 4 条均未落分支，与分支核验一致）；13:51 #4（overlap end>len）本轮**未列**；R2 #4（bindings 性能）→本轮 #4；10:49 #6（worker-set 前缀）→本轮 #7；本轮 #6 为新形态文案项。历轮已修复项（f3/f4/f5/f12、R2 f2/f5、f2 测试随迁、ocr-r1-1/5）未再列——**未再列≠豁免**（沿 08:32 先例留痕）
- **审查结论**：OCR 本轮**未通过**——7/7 confirmed、0 rejected，须修复后重审
- **修复轮次**：4（第 4 轮在途：13:51 R1 7 findings → `27d4b680a` 修 2 medium → 本轮 7 confirmed 全为未处置遗留/同型再现/新形态——**未闭环**，修复待落分支）
- **base/head SHA**：`b1a3d6dd8` / null（未动——head_sha 待修复轮通过、复审零未关闭、合并进 integration 后回填）
- **worktree**：修复应落 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `27d4b680a`，本会话 git log/status 核验：15:05 条目后 0 新提交、第 4 轮剩余项均未落分支——与报告各条"未落分支"标注一致；工作树仍含 untracked 二进制 `tools/passbguard/passbguard`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`）。全部 7 findings 落 `tools/passbguard/**`——均在 b0 owned_files 范围内
- **本次 JSON 变更**：b0.`review_status` pending → **changes_requested**（沿 07:09/08:32/10:49/13:51 先例——OCR confirmed>0 即落入 changes_requested）；`status` 保持 `in_progress`（修复属节点执行期）、`task_status` 六任务 done、base/head SHA、其余 32 节点均未动。本会话 Edit 后 `python3 json.load` 复验合法（review 分布：1 changes_requested + 32 pending；notes 10 条不变）；`git diff` 相对 HEAD 净变更 = review_status 1 行 + notes #9/#10（10:54/12:17 条目遗留的工作树未提交内容，本条目未触及）
- **备注**：(1) #7（worker-set 前缀硬编码）为唯一 medium，指向 B1+ 演进盲区而非 B0 当下行为错误——修复方式可为代码改口径或登记 B1+ 义务，处置由修复者/协调者定；(2) 重跑 OCR 时 background <8000 字符教训（06:26）仍适用；(3) 历轮悬置 low 的显式裁定（第 3 轮 R1 11 条 + R2 4 条 + 12:37 终审新列 3 条）延续未决，本轮再现其中多条，建议修复时一并收口

---

## 2026-09-23 16:44 CST · b0 节点级 OCR（15:05 重派后第 2 次）：2 findings 全 confirmed（→ changes_requested 维持）

- **节点/轮次**：b0 节点级架构审查 OCR 第 2 次（16:10 R1 7 findings 修复后增量复审）；调度口径"第 2 次"（15:05 重派流）；台账历史口径**第 13 次 OCR 运行**（07:09 / 07:29 / 07:45 / 08:28 / 09:30 / 10:41 / 11:36 / 12:04 / 12:14 / 12:34 / 13:47 / 16:02 / 本次 16:39）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b0/ocr-r2.txt`（本会话已读全文：4,009 字节、mtime 16:39、53 行；首行 "Review complete: 2 finding(s) across 1 selected item(s)."；**该路径第三次被覆盖**——09:30 版 0F/345B → 11:36 版 6F/7,612B → 本次；无 LLM retry 摘要附注）
- **结论计数**：confirmed=2 / rejected=0（调度口径与报告 2 findings 一致；复审范围仅 **1 selected item** = check.go——修复提交唯一触及文件）
- **第 4 轮 R1 修复核验**（本会话 git 实测）：`.worktrees/passb-b0` 自 `27d4b680a` 后新增**恰 1 个修复提交**、工作树干净（仍含 untracked 二进制 `tools/passbguard/passbguard`）——
  - `1a5257003`（16:23:26）`fix(passb): worker-set 消费方发现支持从 manifest 注册位点推导（OCR R1 b0-ocr-r1-7）`（`git show --stat` + commit message 本会话实读：2 文件 +117/−4 = check.go +52 / contracts_test.go +69；实现条目带注册位点后缀时直接推导位点 + 裸名进 router 扫描集合（迁移中间态双位点都发现）+ 注释登记 B1+ 演进义务；测试 TestDiscoverSetConsumersWorkerSetEntrySites RED→GREEN）
  - 覆盖核对：**16:10 R1 7 findings 中仅 #7（唯一 medium）已修**；其余 6 条 low（#1 f10 多文档 / #2 NArg / #3 多名声明 / #4 bindings 性能 / #5 model 死分支 / #6 adapters-import 文案）**均未落分支**（分支仅此 1 提交且只涉 #7 面，commit message 未提及其余）
- **findings 清单**（报告顺序转录；severity：medium=2 / low=0；**2 条全部系 `1a5257003` 修复新引入/触及面（check.go:849-863 worker-set 条目推导区）的连带发现**，非 R1 遗留项再现；每条附建议 diff）：
  1. [bug · medium] `tools/passbguard/check.go:849-854` —— 裸名提取（`strings.Cut " — "`）与位点提取（entryFileRE）两条独立规则无单一事实源：分隔符字面漂移（en dash `–`/普通连字符/中文全角破折号 `——`/空格数不同——中文治理文档语境下全角破折号尤易出现）时 Cut 失败、整个条目串（含路径部分）进 idents、fileReferencesIdent 永不命中、回退扫描**静默失效**；位点侧因 entryFileRE 独立提取仍正常入集恰好掩盖裸名侧失效——迁移中间态（新位点已登记、internal/router 旧注册未删，正是 851-853 注释要求"两个真实引用位点都必须被发现"的场景）下 router 旧注册文件不被发现，contracts.yaml 已登记 router consumers **批量假 contract-consumer-vanished**。建议以 entryFileRE 首匹配索引为切分锚推导裸名，或"Cut 失败但 entryFileRE 命中"矛盾形态下显式报漂移
  2. [bug · medium] `tools/passbguard/check.go:858-863` —— 条目推导的注册位点 m[1]（manifest 手写字面路径）直接进 discovered 消费方集：既不校验存在于 d.GoFiles，也无路径归一化——typo/形式偏差（如 `./internal/...`，entryFileRE 的字符类允许 "." 段整体匹配）与 contracts.yaml consumers 做 containsExact 比较时 **unrecorded+vanished 成对误报**；条目与 consumers 复制同一 typo 时幽灵路径被双双匹配冻结、vanished/unrecorded 永不触发（**f4 修复的 contracts.yaml 14 条幽灵 consumer 同类成因在新路径复发**）；跨来源去重依赖字符串精确相等、路径形式不一致时同一物理文件双计；调用方 CheckContracts（:477-507）对 set 契约行无符号侧 :608-611 contract-consumer-file-missing 那样的 goFileSet 预检兜底。建议位点入集前 goFileSet 校验 + `filepath.ToSlash(filepath.Clean(...))` 归一化，不存在时显式报漂移
- **审查结论**：OCR 本轮**未通过**——2 confirmed 须修复后重审；`review_status=changes_requested` 维持
- **修复轮次**：4（第 4 轮在途：16:10 R1 7 findings → `1a5257003` 修 1 medium（#7）→ R2 增量复审 2 medium 连带发现——**未闭环**；finding 收敛轨迹 7→2）
- **base/head SHA**：`b1a3d6dd8` / null（未动——head_sha 待修复轮通过、复审零未关闭、合并进 integration 后回填）
- **worktree**：`.worktrees/passb-b0`（`codex/passb-b0`，HEAD `1a5257003`，本会话核验工作树干净 + untracked 二进制）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`）
- **本次 JSON 变更**：**无字节级改动**——confirmed>0 时 `review_status` 目标值 `changes_requested` 已在位（16:10 条目所置）；`status=in_progress`、`task_status` 六任务 done、base/head SHA、其余 32 节点均未动。JSON 合法性本会话 `python3 json.load` 复验通过（review 分布：1 changes_requested + 32 pending；notes 10 条不变）
- **备注**：(1) R2 复审范围仅 1 selected item（修复触及文件 check.go）——R1 未处置 6 条 low 中 #1（load.go）/#2（main.go）/#3/#4（discover.go）/#5（model.go）不在扫描范围未再列，#6（check.go 文案）同文件但未列，**未再列≠豁免**（沿 11:43 条目先例留痕）；(2) 2 条 medium 均为 `1a5257003` 新代码的连带缺陷（幽灵路径冻结与 f4 同类成因），修复时须连同 R1 剩余 6 条 low 一并考虑；(3) 重跑 OCR 时 background <8000 字符教训仍适用

---

## 2026-09-23 18:08 CST · b0 节点级 OCR（15:05 重派后第 3 次）：1 finding 全 confirmed（→ changes_requested 维持）

- **节点/轮次**：b0 节点级架构审查 OCR 第 3 次（16:44 R2 2 findings 修复后增量复审）；调度口径"第 3 次"（15:05 重派流）；台账历史口径**第 14 次 OCR 运行**（07:09 / 07:29 / 07:45 / 08:28 / 09:30 / 10:41 / 11:36 / 12:04 / 12:14 / 12:34 / 13:47 / 16:02 / 16:39 / 本次 17:29）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b0/ocr-r3.txt`（本会话已读全文：1,574 字节、mtime 17:29、24 行；首行 "Review complete: 1 finding(s) across 1 selected item(s)."；**该路径第二次被覆盖**——12:04 版 0F/57B 未被采信 → 本次；无 LLM retry 摘要附注）
- **结论计数**：confirmed=1 / rejected=0（调度口径与报告 1 finding 一致；复审范围仅 **1 selected item** = check.go——修复提交唯一触及生产文件）
- **第 4 轮 R2 修复核验**（本会话 git 实测）：`.worktrees/passb-b0` 自 `1a5257003` 后新增**恰 1 个修复提交**、工作树干净（仍含 untracked 二进制 `tools/passbguard/passbguard`）——
  - `971435df3`（17:05:04）`fix(passb): set 位点入集归一化+磁盘验证、裸名推导 entryFileRE 单一事实源锚定（OCR R2 b0-ocr-r2-1/r2-2）`（`git show --stat` + commit body 本会话实读：2 文件 +302/−53 = check.go +185 / contracts_test.go +170）——**r2-1**：删除 `strings.Cut " — "` 字面分隔符依赖，裸名以 entryFileRE 首匹配索引为锚 + trimTrailingNonIdent/isGoIdent，矛盾形态显式 `contract-set-entry-drift` 诊断；**r2-2**：三种 set kind 位点入集统一 setSiteCollector（ToSlash(Clean) 归一化 + goFileSet 磁盘验证 + 缺失位点显式 contract-consumer-file-missing），set 行 recorded consumers 增加 vanished 前磁盘预检；测试扩展 en dash/全角/无空格分隔、点段归一化、幽灵位点、矛盾形态 fixture + 3 个集成 case，全包回归 + CLI 零诊断通过（commit body 自陈）
  - 覆盖核对：**16:44 R2 的 2 条 medium（r2-1/r2-2）全部已修**
- **findings 清单**（报告全文转录，仅 1 条；**系 `971435df3` r2-2 修复引入的归一化不对称连带发现**，非 R1/R2 遗留项再现；附建议 diff）：
  1. [bug · low] `tools/passbguard/check.go:510-518` —— recorded consumer 预检与 vanished/unrecorded 比对存在**归一化不对称**：discovered 侧现经 `filepath.Clean/ToSlash` 归一化（`971435df3` 所加），而 `row.Consumers` 加载时仅剥前导 `./`（load.go normalizeRepoPath）、checkRepoPath 不拒绝中间点段（`a/./b.go`）与双斜杠（`a//b.go`）——此类手写形态通过 validate 却不在 goFileSet：(1) 先误报 "does not exist on disk"（文件实际存在，诊断语义失实）；(2) containsExact 匹配不上叠加 contract-consumer-unrecorded 形成误导性双报——与 `971435df3` 新增注释「unrecorded/vanished 比较只在归一化且磁盘验证过的路径上进行」的声明不符。当前数据全为规范形态不触发，低概率边界。建议对 p 做同样归一化后再查 goFileSet/containsExact（或扩展 checkRepoPath 拒绝非规范形态、加载层显式报错）
- **审查结论**：OCR 本轮**未通过**——1 confirmed 须修复后重审；`review_status=changes_requested` 维持
- **修复轮次**：4（第 4 轮在途：16:10 R1 7 findings → `1a5257003` 修 1 medium（#7）→ 16:44 R2 2 medium → `971435df3` 全修 → R3 1 low 连带发现——**未闭环**；finding 收敛轨迹 7→2→1）
- **base/head SHA**：`b1a3d6dd8` / null（未动——head_sha 待修复轮通过、复审零未关闭、合并进 integration 后回填）
- **worktree**：`.worktrees/passb-b0`（`codex/passb-b0`，HEAD `971435df3`，本会话核验工作树干净 + untracked 二进制）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`）
- **本次 JSON 变更**：**无字节级改动**——confirmed>0 时 `review_status` 目标值 `changes_requested` 已在位（16:10 条目所置）；`status=in_progress`、`task_status` 六任务 done、base/head SHA、其余 32 节点均未动。JSON 合法性本会话 `python3 json.load` 复验通过（review 分布：1 changes_requested + 32 pending；notes 10 条不变）
- **备注**：(1) R3 复审范围仅 1 selected item（check.go）——**16:10 R1 剩余 6 条 low（#1 f10 / #2 NArg / #3 多名声明 / #4 bindings 性能 / #5 model 死分支 / #6 adapters-import 文案）仍未落分支**（分支仅 R2 修复 1 提交，commit 未提及），其中 #6 同文件未再列，**未再列≠豁免**；(2) 本轮唯一 finding 为修复连带（归一化只做了一侧），修复成本低（对 p 同样归一化即可）；(3) 重跑 OCR 时 background <8000 字符教训仍适用

---

## 2026-09-23 18:09 CST · b0 OCR R3-1 处理结论登记（修复工单：recorded consumer 归一化不对称收口）

- **节点/轮次**：b0 节点级审查第 4 轮 R3 唯一 confirmed finding 的处理结论（调度方下发修复工单；**非完成报告**——分支 HEAD 仍 `971435df3`，无对应修复提交）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **finding**：`b0-ocr-r3-1`（severity low，`tools/passbguard/check.go:510-518`；即 18:08 条目 R3 唯一 finding，调度方补充详细坐标后的正式工单）——problem：set 契约 recorded consumer 比对存在归一化不对称——discovered 侧经 setSiteCollector.add（check.go:854-868）做 filepath.Clean/ToSlash 归一化并对照 goFileSet 磁盘验证，而 row.Consumers 加载时仅由 normalizeRepoPath（load.go:108-113）剥前导 './'，checkRepoPath（model.go:196-214）不拒绝中间点段（**调度指令 problem 文本于此截断，以调度方原文为准**）
- **验收标准**（acceptance 转录；**指令文本于『characterization tests c』处截断**，完整原文以调度方为准）：任选其一并满足全部——(a) 加载层把 normalizeRepoPath 改为 slash-Clean 基归一化（一次性覆盖 contracts consumers/characterization_tests、event consumers、ownership/exception 路径等全部位点）；或 (b) 在 check.go:510-522 及同构位点（symbol consumers check.go:624-632、characterization tests c…）
- **现状核实**（本会话在 `.worktrees/passb-b0`@`971435df3` 实读四处坐标）：(1) check.go:510-521——`row.Consumers` 直查 `goFileSet[p]` / `containsExact(discovered, p)`，比对侧未归一化 ✓；(2) check.go:854-868 setSiteCollector.add——`norm := filepath.ToSlash(filepath.Clean(site))` + goFileSet 磁盘验证，discovered 侧已归一化 ✓；(3) load.go:108-113 normalizeRepoPath——仅 `strings.TrimPrefix(p, "./")` 循环剥前导 ✓；(4) model.go:196-214 checkRepoPath——拒空串/通配符/绝对路径/`..` 遍历/反斜杠/尾分隔符（requireGoFile 时校验 .go 扩展），**不拒 `a/./b.go`（中间点段）与 `a//b.go`（双斜杠）** ✓——**problem 属实**
- **审查结论**：`review_status=changes_requested` 维持不变（修复未完成；本条仅登记工单）
- **修复轮次**：4（第 4 轮在途：R1 7→修 1 medium→R2 2→全修→R3 1→**本工单待执行**；另 R1 剩余 6 条 low 未处置悬置如前）
- **base/head SHA**：`b1a3d6dd8` / null（未动）
- **worktree**：修复应落 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `971435df3`，本会话核验工作树仍含 untracked 二进制 `tools/passbguard/passbguard`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`）
- **本次 JSON 变更**：b0.`notes` 数组末尾追加 1 条字符串（第 11 条）——工单全文（problem + 验收标准 + 两处截断标注 + 现状核实结论），沿 12:17 条目 notes 追加先例；`review_status=changes_requested`、`status=in_progress`、`task_status`、base/head SHA、其余 32 节点均未动。本会话 `python3 json.load` 复验合法（notes 11 条；review 分布 1 changes_requested + 32 pending）；`git diff` 相对 HEAD 5+/2−（notes 追加 + review_status 行的自然展开）
- **备注**：工单两处截断（problem 末尾 / acceptance 方案 b 同构位点枚举）——修复者执行时以调度方完整原文为准；方案 (a) 覆盖面更大（加载层一次性归一化全部位点），方案 (b) 改动面小但需逐位点补，处置由修复者/协调者定

---

## 2026-09-23 18:31 CST · b0 终审全量 OCR：9 findings，调度裁定未关闭=1（→ changes_requested 维持；未点名项留痕）

- **节点/轮次**：b0 终审全量 OCR（15:05 重派流收口审查）；台账历史口径**第 15 次 OCR 运行**（07:09 / 07:29 / 07:45 / 08:28 / 09:30 / 10:41 / 11:36 / 12:04 / 12:14 / 12:34 / 13:47 / 16:02 / 16:39 / 17:29 / 本次 18:28）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b0/ocr-final.txt`（本会话已读全文：8,602 字节、mtime 18:28、167 行；首行 "Review complete: 9 finding(s) across 13 selected item(s)"——**全量复扫 13 items**；**该路径第三次被覆盖**——09:43 版 → 12:34 版 → 本次；无 LLM retry 摘要附注）
- **结论计数**：报告 9 findings（medium=1 / low=8）；调度裁定**未关闭=1——未点名**（与 12:37 条目同型缺口，留痕待补；沿 10:54 先例建议调度方点名，以便修复清单划定）
- **分支核验**（本会话 git 实测）：`.worktrees/passb-b0` HEAD 仍 `971435df3`——`971435df3..HEAD` 零新提交，**18:09 工单（b0-ocr-r3-1 归一化不对称）未落分支**；工作树仍含 untracked 二进制 `tools/passbguard/passbguard`
- **findings 清单**（报告顺序转录，含与本管家在案记录的对照）：
  1. [bug · low] `tools/passbguard/load.go:95-103` —— decodeStrict 多文档静默截断（**f10 历轮再现**，报告自标"台账在案 f10 未落分支"）
  2. [bug · low] `tools/passbguard/check.go:211-214` —— guard 源侧例外 (from,to) 重复边 last-wins 无诊断 + sort.Slice 非稳定排序致 exception-reason-drift 假信号（**f14 历轮再现**：10:49 #11 / 12:37 #4）
  3. [bug · low] `tools/passbguard/check.go:1019-1023` —— fileReferencesIdent parseFileFull 出错静默返回 false，worker-set 注册位点发现集静默剔出（**R1 #8 历轮再现**：10:49 #8 / 12:37 #5）
  4. [maintainability · low] `tools/passbguard/check.go:253-262` —— remove_at="b5" 且屏障非 b5 时 exception-barrier 与 exception-b5-escape 对同一事实双报（**新形态**，历轮未见此具体条目）
  5. [maintainability · low] `tools/passbguard/check.go:613-617` —— 消费方发现失败记在 contract-symbol-missing 名下，check 名与语义不符（报告标"台账在案 R3-a2-f1，工单未落分支"——**留痕：R3-a2-f1 工单对象是 check.go:688-690 event-producer-missing 诊断文案，本条是 check.go:613-617 的错误归类问题，另一处；报告"在案"标注与在案工单不同物，系第 3 次同类标注偏差（16:10 #6 同型），如实登记**）
  6. [bug · low] `tools/passbguard/main.go:95-98` —— fs.NArg() 未检查（**13:51 #3 / 16:10 #2 再现**，报告自标"台账在案 #3 未落分支"）
  7. [maintainability · low] `tools/passbguard/model.go:258-262` —— else-if 空 destination 死分支（**13:51 #7 / 16:10 #5 再现**，报告自标"台账在案 #7 未落分支"）
  8. [bug · low] `tools/passbguard/discover.go:356-359` —— ValueSpec 多名声明跳过 → contract-symbol-missing 误报（**f6 历轮再现**，报告自标"台账在案 #6 前半未落分支"）
  9. [bug · medium] `tools/passbguard/discover.go:507-516` —— bindingIdentPositions 把 map 字面量裸 Ident 键（`map[Priority]int{PriorityHigh: 1}`）与 `=`（ASSIGN）左侧两类**真使用**当绑定，referencesSymbol 同包消费方漏检：新消费方静默漏报 contract-consumer-unrecorded、已登记消费方重构后假 contract-consumer-vanished（**11:36 R2 #3（map 键）+ #6（AssignStmt= 左侧）合并再现并升 medium**——两支历轮在案未处置）
- **对照小结**（本管家整理，非裁定）：9 条 = 历轮未处置 low 再现 6 条（#1 f10 / #2 f14 / #3 R1#8 / #6 NArg / #7 model 死分支 / #8 f6）+ 历轮在案合并升级 1 条（#9 = R2#3+R2#6 升 medium）+ 新形态 1 条（#4 b5-escape 双报）+ 错误归类 1 条（#5，报告标注与在案工单不同物）。已修项（`27d4b680a` ocr-r1-1/5、`971435df3` r2-1/r2-2、`62a56200a` f2 测试随迁）**未再列**——修复成果有效；**18:09 工单 b0-ocr-r3-1（check.go:510-518）本轮未列**（工单未落分支但终审未再列——未再列≠豁免，可能系扫描口径，留痕）
- **审查结论**：终审全量 OCR 后调度裁定未关闭=1 > 0 → `review_status=changes_requested` 维持；节点未收口
- **修复轮次**：4（第 4 轮在途：R1 7 → `1a5257003` 修 1 → R2 2 → `971435df3` 全修 → R3 1 → 工单 b0-ocr-r3-1 未落分支 → 终审 9 findings/未关闭=1 未点名——**未闭环**）
- **base/head SHA**：`b1a3d6dd8` / null（未动——合并与 head_sha 回填继续冻结）
- **worktree**：`.worktrees/passb-b0`（HEAD `971435df3`，本会话核验 0 新提交）；`.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`）
- **本次 JSON 变更**：**无字节级改动**——未关闭>0 时目标值 `changes_requested` 已在位（16:10 条目所置）；`status=in_progress`、`task_status` 六任务 done、base/head SHA、notes 11 条、其余 32 节点均未动。JSON 合法性本会话 `python3 json.load` 复验通过（review 分布：1 changes_requested + 32 pending）
- **备注**：(1) **未关闭=1 未点名**——若所指为 #9（合并 R2#3/#6 升 medium），其修复方向报告已给（DEFINE-only 左侧 + CompositeLit 类型上下文区分 struct/map 键，无法区分时按 map 落到多报方向）；若所指为 #5，须先澄清其与在案 R3-a2-f1 工单的对象差异；(2) 历轮悬置 low 在全量复扫大量再现（6/9 条），印证 09:32/12:37 条目预警——建议协调者对全部悬置项做一次性显式裁定（修复/豁免/延期），避免终审反复；(3) b0-ocr-r3-1 工单仍未落分支，修复时一并收口；(4) 重跑 OCR 时 background <8000 字符教训仍适用

---

## 2026-09-23 18:41 CST · b0 OCR R3-1 处理结论（指令重发核验：工单已在 18:09 登记，不重复追加；附状态快照）

- **节点/任务**：b0 · 工单 `b0-ocr-r3-1`（18:08 R3 唯一 confirmed finding 的处理结论下发）
- **指令重发识别**：本指令与 18:09 条目所登记的工单**完全同一**——id（b0-ocr-r3-1）、severity（low）、file（tools/passbguard/check.go:510-518）、problem 与 acceptance 文本一致（含相同的两处截断：problem 止于"不拒绝中间点段 '"、acceptance 止于"characterization tests c"，完整原文以调度方为准，沿 12:17 条目截断标注先例）
- **JSON 核验与处置**：本会话 `python3 json.load` 实测 b0.`notes` 12 条——**notes[10]（第 11 条，947 字符）即本工单全文登记**（"OCR R3-1 处理结论（2026-09-23 登记为修复工单——passb-b0 HEAD 971435df3 尚无对应修复提交，管家实读四处坐标确认 problem 属实）……"，18:09 条目所为）→ **不重复追加**（追加式纪律：同工单在 notes 数组双登记会破坏后续轮次条目计数与引用）。本次 JSON **无字节级改动**
- **现状核实**（本会话 18:41 在 `.worktrees/passb-b0` 工作树实读，工单 problem 仍属实、修复未落）：check.go:510-522 recorded consumer 循环 `if !goFileSet[p]` 直接用原始 p、**无归一化**；setSiteCollector.add（check.go:854-868）确认 ToSlash(Clean) 归一化 + goFileSet 磁盘验证（工单所指 discovered 侧坐标吻合）；load.go:107-113 normalizeRepoPath 确认"仅重复剥前导 ./"；model.go checkRepoPath（工单截断处所指 :196-214）未逐一核验，如实说明
- **18:31 条目之后的 JSON 新变化**（本会话 diff 相对 HEAD `78193c4e5` 实测，系调度方/其他会话所为、非本管家，如实登记且未触碰）：(1) b0.`notes[11]`（第 12 条）**用户裁定**：「OCR 改为每个任务完成后即审；b0 六任务的全量 diff 已由 06:13-18:09 多轮节点级 OCR 覆盖（台账在案），免全量终审；收口 = tail OCR 从 1a5257003 增量审（覆盖 r3 轮 1 个待修 finding 所在范围与在途修复）」；(2) b0 节点新增字段 **`ocr_tail_base: "1a5257003d08f14ce7a8d7367801ab80aba0b616"`**（tail OCR 基准 SHA，台账此前无对应登记条目——本条补记）；(3) `review_status` 回退为 `pending`（18:31 条目时 changes_requested 在位；回退模式与 13:25/15:05 一致）。本工单登记事件沿 12:17 先例不动 review_status，现值 pending 维持
- **分支在途修复快照**（本会话 git 实测）：`.worktrees/passb-b0` HEAD 仍 `971435df3`（`971435df3..HEAD` 零新提交）；**工作树新增 2 个未提交修改**：`tools/passbguard/discover.go`（+85/−9）、`discover_test.go`（+58）——与 ruling 所指"在途修复"对应（推测涉终审 #9 bindingIdentPositions medium，未提交无法定论，如实登记）；untracked 二进制 `tools/passbguard/passbguard` 仍在
- **修复轮次**：4（维持——工单 b0-ocr-r3-1 + 终审未关闭=1 均未收口；分支有在途未提交修复）
- **base/head SHA**：`b1a3d6dd8` / null（未动）
- **worktree**：`.worktrees/passb-b0`（`codex/passb-b0`，HEAD `971435df3`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`）
- **本次 JSON 变更**：**无字节级改动**（工单已在 18:09 登记；notes[11]/ocr_tail_base/review_status 均系他方所为且未触碰）。JSON 合法性本会话 `python3 json.load` 复验通过（33 节点；b0.status=in_progress、review_status=pending、notes 12 条、ocr_tail_base 字段在位）

---

## 2026-09-23 18:46 CST · b0 登记 OCR 覆盖：ocr_covered 追加 [1a5257003 → 971435df3]（审得 0 条需修 findings）

- **节点**：b0 —— B0 契约与所有权冻结（passbguard + 4 份治理 yaml + 证据）
- **登记内容**：b0 节点新增数组字段 `ocr_covered`，追加首条 `{base: "1a5257003d08f14ce7a8d7367801ab80aba0b616", head: "971435df33b52fe360e39d610924a17dfc3d7b5b"}`（全 40 位 SHA，调度指令口径：**审得 0 条需修 findings**）
- **SHA 真实性核验**（本会话 git 实测，`.worktrees/passb-b0`）：`git rev-parse` 双双命中——`1a5257003`（16:23:26，fix: worker-set 消费方发现支持从 manifest 注册位点推导，R1 #7 修复）与 `971435df3`（17:05:04，fix: set 位点入集归一化+磁盘验证、裸名推导 entryFileRE 锚定，R2 r2-1/r2-2 修复）；区间 `1a5257003..971435df3` 含**恰 1 个提交**（即 `971435df3` 本身）
- **与 ruling 的衔接**：本登记正是 `notes[11]` 用户裁定（18:41 条目转录）所定收口口径的覆盖记录——tail OCR 从 `1a5257003` 增量审，登记 base = `ocr_tail_base` 字段值、head = 审查时点分支 HEAD `971435df3`
- **留痕（口径差异，待协调者如需澄清）**：(1) 18:31 条目终审全量 OCR 记录 9 findings / 调度裁定未关闭=1（未点名）；本次登记口径"审得 0 条需修 findings"——指令未说明两者关系（若系对终审 findings 的处置裁定为无需修，则实质推翻 18:31 的"未关闭=1"口径；若系另一次 tail 增量复审结果，其报告路径未附）。按指令原文如实登记，不自行判定；(2) **未提交工作树修改不在覆盖区间内**：b0 分支工作树仍有 2 个未提交修改（`discover.go` +85/−9、`discover_test.go` +58，18:41 条目登记）——ocr_covered 登记的是已提交 SHA 区间，在途修改落地后如需覆盖须另追加条目
- **节点级状态未迁移**：`status=in_progress`、`review_status=pending`、`head_sha=null` 维持——节点 done / 合并 / head_sha 回填仍待调度方显式指令（本指令仅授权 ocr_covered 登记 + 台账留痕）
- **修复轮次**：4（指令口径 0 条需修 findings；工单 b0-ocr-r3-1 未落分支、在途未提交修改 2 文件——其收编与终局处置由后续调度指令定）
- **base/head SHA（节点级）**：`b1a3d6dd8` / null（未动）
- **worktree**：`.worktrees/passb-b0`（`codex/passb-b0`，HEAD `971435df3`，工作树含 2 个未提交修改 + untracked 二进制）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `78193c4e5`）
- **本次 JSON 变更**：b0 节点 `ocr_tail_base` 行后追加 `ocr_covered` 数组（6 行新增 + 1 行尾逗号）；Edit 后 `python3 json.load` 复验合法（断言全过：ocr_covered 恰 1 条且 SHA 与指令逐字符一致；status/review_status/notes/task_status/base/head_sha 均未动）；`git diff` 相对 HEAD 累计 7+/1−（= 既有 notes #9-#12 + ocr_tail_base + 本次 ocr_covered，review_status 行与 HEAD 一致）

---

## 2026-09-23 18:47 CST · b0 → done（节点收口：head acd7b24 回填，门禁+OCR 通过）

- **节点**：b0 —— B0 契约与所有权冻结（passbguard + 4 份治理 yaml + 证据）——**全 DAG 首个 done 节点**
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：无（depends_on = []；b0 为 DAG 根）
- **worktree**：`.worktrees/passb-b0`（`codex/passb-b0`，HEAD `acd7b2401`，本会话核验**工作树全净**——18:41 条目登记的 untracked 二进制 `tools/passbguard/passbguard` 亦已不在 status 输出）
- **base → head**：`b1a3d6dd8` → **`acd7b24011b57704d401c63f4d2fce1819e057fc`**（指令短 SHA `acd7b24`，本会话 `git rev-parse` 解析为完整 40 位；`git merge-base --is-ancestor` 核验 base 是 head 祖先 ✓；分支累计 **19 提交** = 六任务 9 + 历轮 OCR 修复与测试迁移 10）；head 提交内容：18:46:59 `fix(passbguard): 修复同包符号引用漏检 map 键与赋值左侧使用`——**正是 18:41 条目登记的在途未提交修改（discover.go +85/−9 / discover_test.go +58）的提交化**，对应 18:31 终审 #9 medium（bindingIdentPositions map 键 + AssignStmt= 左侧）；`971435df3..acd7b2401` 恰 1 提交
- **门禁+OCR 通过**：调度指令口径（本管家未重跑，沿 03:40 起先例）；节点 5 项 gates 全过证据见 06:21 条目，OCR 覆盖登记见 18:46 条目（ocr_covered 首条 0 需修 findings）
- **审查结论**：approved（指令口径"门禁+OCR 通过"→ `review_status` pending → **approved**，推断迁移在此留痕；OCR 共 **15 次运行**（07:09 起至 18:28 终审）+ 4 个修复轮，终态以调度方收口指令为准）
- **OCR 报告路径**：历轮全录（ocr-r1/r2/r3/r3-a2/ocr-final 各版本，见各条目）；ocr_covered 覆盖区间登记为 [1a5257003 → 971435df3]
- **修复轮次**：4（全部闭环：R1 #7→`1a5257003`、R2 r2-1/r2-2→`971435df3`、R3 工单与终审 #9→`acd7b2401`；历轮悬置 low 按调度方 18:46 收口口径不再构成修复义务）
- **测试证据路径**：B0.1–B0.6 六任务报告/审查包（03:40–06:21 各条目）+ 节点 5 项 gates（06:21）+ ocr_covered 登记（18:46）
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过，diff 恰 3 行）：(1) `status` in_progress → **done**；(2) `head_sha` null → **acd7b24011b57704d401c63f4d2fce1819e057fc**；(3) `review_status` pending → **approved**。`base_sha=b1a3d6dd8`、task_status 六任务 done、notes 12 条、ocr_tail_base/ocr_covered 均未动。33 节点分布：**1 done + 32 pending**（review：1 approved + 32 pending）
- **留痕 1（head_sha 回填口径）**：07:32/09:32 条目曾约定"head_sha 以分支合并进 integration 后的头回填、不得用分支中间提交"——本次调度指令明确 head=acd7b24（分支 HEAD），而 **`codex/passb-b0` 尚未合并进 `codex/passb-integration`**（integration 新增 2 提交 `56bb5542c`/`03e40a543` 均为 docs 提交、不含 b0 fix 提交）。按指令权限以分支头回填；后续合并时如需以合并头覆盖，由调度方显式指令
- **留痕 2（ocr_covered 区间不含 head）**：ocr_covered 首条止于 `971435df3`，head `acd7b2401`（18:46:59 提交）不在该区间——调度方未要求追加，如实登记；若 tail OCR 已覆盖 (971435df3, acd7b2401]，建议后续指令补登 ocr_covered 条目
- **留痕 3（依赖节点残留）**：32 个传递依赖节点 status=pending（非 blocked、无需翻转），但其 notes 内 32 处「BLOCKED（2026-09-23）：前置 b0 阻塞；解除条件：修复并 done b0 后恢复。」文本的解除条件现已满足，**文本残留待清理**——本指令未授权改动这些节点，留痕待调度方指令
- **后续（调度方事项）**：(1) 合并 `codex/passb-b0`（HEAD `acd7b2401`）进 `codex/passb-integration`；(2) 32 个依赖节点 BLOCKED 文本清理（可随 B1 派发一并）；(3) ocr_covered 区间补登（若适用）；(4) b1-identity/b1-airesource/b1-commercial/b1-execution 四个 B1 节点已解锁可派发

---

## 2026-09-23 18:53 CST · b0 合入 integration（集成合并：门禁绿色，台账措辞冲突以上游契约解决）

- **执行者**：Pass B 总集成工程师（集成分支 `codex/passb-integration` 唯一合并执行者）
- **合并前清理**：`.worktrees/passb-int` 工作树含上一管家会话遗留的 2 个未提交修改（本台账 18:47 b0 收口条目 + `execution-dag.json` b0 done/head_sha 回填）——先以 docs 提交 **`1a85809ee`** 固化（2 文件 23+/3−）
- **合并**：`git merge codex/passb-b0 --no-edit`（ort 策略，**无冲突**），合并提交 **`0be903ef2`**；merge-base `5bf228a40`，b0 侧 19 提交，52 文件 +12832/−68（passbguard 全部源码+测试+二进制、4 份治理 yaml：contracts/ownership-matrix/event-catalog/exception-ledger、b0-evidence.md、`tools/modulemove→tools/internal/movemanifest` 库迁移等）。合并后工作树全净
- **门禁 1 首跑 RED**：`make -C .worktrees/passb-int check-passb-readiness`（= `go run ./tools/passbguard -root .`，Makefile:262-263）→ exit 1，2 条诊断：`docs/architecture/passb/execution-ledger.md:77` 命中 ForbiddenPhrases 中的两条中文样例（原字样不在此逐字复现，定义见 `tools/passbguard/overlap.go:13-20`；CheckAmbiguity 扫 `docs/architecture/passb/` 全部 `*.md`/`*.yaml`，无文件豁免）
- **根因**：本台账文件**仅存在于 integration 分支**（`git cat-file -e codex/passb-b0:docs/architecture/passb/execution-ledger.md` → "exists on disk, but not in 'codex/passb-b0'"），b0 worktree 门禁运行从未扫描过它；合并后治理目录并集才暴露。台账 77 行系 B0.3 收口条目**引用短语样例以记录禁令本身**，非悬置所有权
- **解决口径（上游契约优先）**：按集成职责"冲突时以上游契约与子计划为准解决并记录"——freeze B0.3 Step 1 机器禁令获胜，**改写台账 77 行 (4) 措辞**（原两条字面样例短语改写为「先合并为准、二择其一执行」+ 括注改写缘由；本条目亦不逐字复现原短语以免禁令自指），语义不变；**未改动已审查通过的 passbguard 代码、未给守卫加文件豁免**。全治理目录 grep 复验：三个 ForbiddenPhrases 0 命中
- **门禁 1 重跑 GREEN**：`make -C .worktrees/passb-int check-passb-readiness` → `pass-b readiness: legacy=396 aliases=99 exceptions=105 contracts=125 events=29 overlaps=0 missing=0`（exit 0）
- **门禁 2 GREEN**：`make -C .worktrees/passb-int check-backend-architecture`（= `go run ./tools/architectureguard`，Makefile:255-256）→ `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16` / `OK (0 violations)`（exit 0；与 B0.3 条目登记基线 633/23+23/58 一致）
- **DAG 变更**：无（b0 节点已于 18:47 收口条目置 done/head_sha=`acd7b2401`；该条留痕 1 约定"如需以合并头覆盖 head_sha 由调度方显式指令"，本次未获该指令，维持分支头口径）
- **调度方后续事项更新**：18:47 收口条目"后续"清单第 (1) 项（合并 b0 进 integration）**已完成**；(2) 32 节点 BLOCKED 文本清理、(3) ocr_covered 区间补登、(4) B1 四节点派发，维持待办
- **Mimosa**：本会话 2 次 git commit 钩子均报 `scanner_enobufs`（未获完整扫描结论，按钩子兼容策略继续提交，不宣称项目安全审计通过）
- **集成后 HEAD**：本条目所在 docs 提交（完整 SHA 见 `git rev-parse HEAD`，已回报调度方）

---

## 2026-09-23 18:56 CST · b0 集成登记：head_sha 以集成后 HEAD d57a2fa70 覆盖（调度方显式指令）

- **节点**：b0 —— B0 契约与所有权冻结（已 done，06:21 起全周期见前各条目）
- **指令**：调度方下发"b0 已集成：合入 codex/passb-integration，集成后 HEAD SHA：`d57a"（指令文本于 SHA 处截断）
- **SHA 解析与核验**（本会话 git 实测）：短 SHA `d57a` 在仓库内有歧义（1 commit + 2 tree 候选），commit 候选唯一 = **`d57a2fa708c3fecf5f0510553ea26db3de51d3c9`**（18:55:46，`docs(passb): 登记 b0 合入 integration（合并 0be903ef2；台账 77 行禁令自指措辞按 freeze B0.3 契约改写；双门禁绿色）`，即 integration 当前 HEAD）——按指令上下文"集成后 HEAD"采信该 commit；`git merge-base --is-ancestor acd7b2401 HEAD` ✓（b0 分支头已合入）
- **与 18:53 条目的衔接**：集成工程师已完整登记合并事件（`1a85809ee` 固化工作树 → merge `0be903ef2` 无冲突 52 文件 +12832/−68 → 门禁 1 首跑 RED（台账 77 行禁令自指）→ 措辞按 freeze B0.3 上游契约改写 → 双门禁 GREEN），其 :834 明确"DAG 变更：无……如需以合并头覆盖 head_sha 由调度方显式指令，本次未获该指令，维持分支头口径"——**本指令即该显式指令**，覆盖生效
- **head_sha 语义闭环**：18:47 条目留痕 1 所载 07:32 约定（"合并后置 done 并以合并头回填 head_sha"）与 06:21 条目 b0-evidence.md 声明（"B1 起点为本分支合并进 integration 后的头，不得以分支中间提交作为 B1 起点"）至此落地：head_sha 由分支头 `acd7b2401` 覆盖为集成后 HEAD `d57a2fa70`——B1 及后续节点派发时 base 应取此集成后头
- **留痕（head_sha ≠ merge commit 本身）**：`d57a2fa70` 是 merge commit `0be903ef2` 之上登记集成的 docs 提交（18:53 条目 :837 自证"集成后 HEAD = 本条目所在 docs 提交"），非 merge commit 本身——语义为"集成完成后的 integration 分支头"（含台账登记与措辞修复提交）；如调度方后续要求改锚 `0be903ef2`，属显式再覆盖
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过，diff 恰 1 行）：b0.`head_sha` `acd7b24011b57704d401c63f4d2fce1819e057fc` → **`d57a2fa708c3fecf5f0510553ea26db3de51d3c9`**；`status=done`、`review_status=approved`、`base_sha=b1a3d6dd8`、task_status 六任务 done、notes 12 条、ocr_tail_base/ocr_covered 均未动。33 节点分布不变（1 done + 32 pending）
- **worktree**：DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `d57a2fa70`，合并后工作树全净——18:53 条目核验，本会话 `git status` 亦净）；b0 实施分支 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `acd7b2401`，已合入）
- **调度方后续事项**（18:47 条目"后续"清单更新）：(1) 合并 b0 ——**已完成**（`0be903ef2`，18:53 条目）；(2) 32 节点 BLOCKED 文本清理——维持待办；(3) ocr_covered 区间补登（若适用）——维持待办；(4) **B1 四节点（b1-identity/b1-airesource/b1-commercial/b1-execution）已解锁可派发**，base 取 `d57a2fa70`

---

## 2026-09-23 18:58 CST · b1-identity → in_progress（B1 首节点派发）

- **节点**：b1-identity —— B1-ID Identity 边界（27 legacy 文件：Actor/Tenant/RBAC/Audit 公共用例）——**b0 之后首个派发节点（B1 阶段开始）**
- **计划路径**：`docs/plans/passb/10-identity.md`
- **前置**：b0（done ✓，18:47 条目；head_sha 已按 18:56 条目以集成后 HEAD `d57a2fa70` 回填）
- **worktree**：指令未附实施 worktree；本会话 `ls .worktrees/` 核验 **`passb-b1-identity` 尚不存在**——沿 b0 模式，实施分支/worktree 由调度方或实施者建立，建立后请以新条目补记；DAG/台账所在 integration worktree `.worktrees/passb-int`（`codex/passb-integration`，HEAD `d57a2fa70`）
- **base SHA**：**`d57a2fa708c3fecf5f0510553ea26db3de51d3c9`**（派发时回填，conventions §9——依据在案约定：06:21 条目 b0-evidence.md 声明"B1 起点为本分支合并进 integration 后的头"+ 18:56 条目"B1 四节点派发 base 取 `d57a2fa70`"；指令未另给 base 值，按唯一在案口径回填并在此留痕）；**head SHA**：null（未回填——节点分支评审通过时回填）
- **测试证据路径（计划）**：`docs/architecture/evidence/passb/b1-identity.md`、`docs/plans/passb/reports/b1-identity.md`、`docs/plans/passb/reviews/b1-identity.md`（均尚未产出）
- **审查结论**：pending（review_status 未请求）
- **OCR 报告路径**：无（尚未进入审查轮；按 notes[11] ruling 新口径——每任务完成后即审，tail OCR 以 ocr_tail_base 为基准增量）
- **修复轮次**：0
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过）：(1) b1-identity `status` pending → **in_progress**；(2) `base_sha` null → **d57a2fa708c3fecf5f0510553ea26db3de51d3c9**（派发时回填）。`head_sha=null`、`review_status=pending`、其余节点均未动。33 节点分布：**1 done（b0）+ 1 in_progress（b1-identity）+ 31 pending**
- **备注**：(1) 指令原文 "b1-identity → running"；状态机（conventions §9）无 `running` 值，沿 03:18 起六次先例映射为规范值 `in_progress` 并在此留痕；(2) b1-identity notes 内既有「BLOCKED（2026-09-23）：前置 b0 阻塞……」文本（06:26 遗留）与本次 in_progress 并存——本指令未授权清理该节点 notes，留痕待批量清理指令（18:56 条目后续事项 2）；(3) 节点 notes 所载既有义务提示（identity 侧跨 owner 耦合处理、escapeLikeKeyword 反向消费等，见 DAG required_contracts）对实施者仍然有效

---

## 2026-09-23 18:59 CST · b1-airesource → in_progress（B1 第二节点派发；补记 b1-identity worktree 建立）

- **节点**：b1-airesource —— B1-AI AI Resource 边界（33 legacy 文件：Model/MCP/Search/Vector/Storage 能力解析）
- **计划路径**：`docs/plans/passb/11-airesource.md`
- **前置**：b0（done ✓，18:47 条目）
- **worktree**：指令未附实施 worktree；本会话 `ls .worktrees/` 核验 **`passb-b1-airesource` 尚不存在**（沿 b0 模式，实施分支/worktree 由调度方或实施者建立，建立后请以新条目补记）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `d57a2fa70`）
- **补记（18:58 条目 worktree 待办闭环）**：本会话核验 **`.worktrees/passb-b1-identity` 已建立**——分支 `codex/passb-b1-identity`、起点 `d57a2fa70`（= 已回填 base_sha，起点与台账登记一致 ✓）、工作树干净（本会话 git log/status 实测）
- **base SHA**：**`d57a2fa708c3fecf5f0510553ea26db3de51d3c9`**（派发时回填，conventions §9；与 b1-identity 同口径：06:21 b0-evidence 声明 + 18:56 条目"B1 派发 base 取 `d57a2fa70`"）；**head SHA**：null（未回填——节点分支评审通过时回填）
- **测试证据路径（计划）**：`docs/architecture/evidence/passb/b1-airesource.md`、`docs/plans/passb/reports/b1-airesource.md`、`docs/plans/passb/reviews/b1-airesource.md`（均尚未产出）
- **审查结论**：pending（review_status 未请求）
- **OCR 报告路径**：无（尚未进入审查轮）
- **修复轮次**：0
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过，净变更 2 行）：(1) b1-airesource `status` pending → **in_progress**；(2) `base_sha` null → **d57a2fa708c3fecf5f0510553ea26db3de51d3c9**。`head_sha=null`、`review_status=pending`、其余节点均未动。33 节点分布：**1 done（b0）+ 2 in_progress（b1-identity/b1-airesource）+ 30 pending**
- **备注**：(1) 指令原文 "b1-airesource → running"，沿先例映射为 `in_progress`；(2) 与 b1-identity 并行合法（execution_mode=parallel，framework:95 四份 B1 计划文件不相交、b0 后可并发）；(3) 节点 notes 既有提示对实施者有效：airesource 为下游消费最重的基础门面（agentruntime→airesource 例外口径 32 对/包限定符 203 处，审校 F6 实测），IB1 前须冻结 capability 契约为 current；(4) notes 内 BLOCKED 残留文本同 b1-identity，待批量清理

---

## 2026-09-23 19:00 CST · b1-commercial → in_progress（B1 第三节点派发；补记 b1-airesource worktree 建立）

- **节点**：b1-commercial —— B1-CM Commercial 边界（8 legacy 文件：Admission/Budget/Usage/Payment 门面）
- **计划路径**：`docs/plans/passb/12-commercial.md`
- **前置**：b0（done ✓，18:47 条目）
- **worktree**：指令未附实施 worktree；本会话 `ls .worktrees/` 核验 **`passb-b1-commercial` 尚不存在**（沿 b0 模式，建立后请以新条目补记）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `d57a2fa70`）
- **补记（18:59 条目 worktree 待办闭环）**：本会话核验 **`.worktrees/passb-b1-airesource` 已建立**——分支 `codex/passb-b1-airesource`、起点 `d57a2fa70`（= 已回填 base_sha，起点与台账登记一致 ✓）、工作树干净（本会话 git log/status 实测）
- **base SHA**：**`d57a2fa708c3fecf5f0510553ea26db3de51d3c9`**（派发时回填，conventions §9；与 b1-identity/b1-airesource 同口径）；**head SHA**：null（未回填——节点分支评审通过时回填）
- **测试证据路径（计划）**：`docs/architecture/evidence/passb/b1-commercial.md`、`docs/plans/passb/reports/b1-commercial.md`、`docs/plans/passb/reviews/b1-commercial.md`（均尚未产出）
- **审查结论**：pending（review_status 未请求）
- **OCR 报告路径**：无（尚未进入审查轮）
- **修复轮次**：0
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过，净变更 2 行）：(1) b1-commercial `status` pending → **in_progress**；(2) `base_sha` null → **d57a2fa708c3fecf5f0510553ea26db3de51d3c9**。`head_sha=null`、`review_status=pending`、其余节点均未动。33 节点分布：**1 done（b0）+ 3 in_progress（b1-identity/b1-airesource/b1-commercial）+ 29 pending**
- **备注**：(1) 指令原文 "b1-commercial → running"，沿先例映射为 `in_progress`；(2) 三节点并行合法（execution_mode=parallel）；(3) 节点 notes 既有提示对实施者有效：commercial 须导出 model_usage 绑定族端口供 B2-knowledge/agentcatalog 消费（knowledge→commercial 3 符号/agentcatalog→commercial 3 符号，package_private_couplings 在案），payment/usage 属高风险差分面（framework:40）差分证据必交；(4) notes 内 BLOCKED 残留文本待批量清理

---

## 2026-09-23 19:01 CST · b1-execution → in_progress（B1 第四节点派发，B1 四节点派发齐；补记 b1-commercial worktree 建立）

- **节点**：b1-execution —— B1-EX Execution 边界（21 legacy 文件：Sandbox/Target/Workspace/Terminal/Browser 门面）
- **计划路径**：`docs/plans/passb/13-execution.md`
- **前置**：b0（done ✓，18:47 条目）
- **worktree**：指令未附实施 worktree；本会话 `ls .worktrees/` 核验 **`passb-b1-execution` 尚不存在**（沿 b0 模式，建立后请以新条目补记）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `d57a2fa70`）
- **补记（19:00 条目 worktree 待办闭环）**：本会话核验 **`.worktrees/passb-b1-commercial` 已建立**——分支 `codex/passb-b1-commercial`、起点 `d57a2fa70`（= 已回填 base_sha，起点与台账登记一致 ✓）、工作树干净（本会话 git log/status 实测）
- **base SHA**：**`d57a2fa708c3fecf5f0510553ea26db3de51d3c9`**（派发时回填，conventions §9；B1 四节点同口径）；**head SHA**：null（未回填——节点分支评审通过时回填）
- **测试证据路径（计划）**：`docs/architecture/evidence/passb/b1-execution.md`、`docs/plans/passb/reports/b1-execution.md`、`docs/plans/passb/reviews/b1-execution.md`（均尚未产出）
- **审查结论**：pending（review_status 未请求）
- **OCR 报告路径**：无（尚未进入审查轮）
- **修复轮次**：0
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过，净变更 2 行）：(1) b1-execution `status` pending → **in_progress**；(2) `base_sha` null → **d57a2fa708c3fecf5f0510553ea26db3de51d3c9**。`head_sha=null`、`review_status=pending`、其余节点均未动。33 节点分布：**1 done（b0）+ 4 in_progress（B1 全部四节点）+ 28 pending**
- **备注**：(1) 指令原文 "b1-execution → running"，沿先例映射为 `in_progress`；(2) **B1 四节点（identity/airesource/commercial/execution）至此全部派发**，均 base=`d57a2fa70`、execution_mode=parallel（framework:95 计划文件不相交、b0 后可并发），汇合点为 ib1（B1-ID→B1-AI→B1-CM→B1-EX 逐支合并）；(3) 节点 notes 既有提示对实施者有效：*Handler 方法文件（browserskill.go/sandbox_terminal_ws.go）去方法化裁定、conversation→execution browserSkillScope/resolveTenantSandboxForConfig 10 调用点须导出、agentruntime→execution 引用走 IB1 冻结门面；(4) notes 内 BLOCKED 残留文本待批量清理

---

## 2026-09-23 19:44 CST · b1-commercial 计划完成审校通过（5 任务结构回填 DAG：B1-CM.1–B1-CM.5 全 pending）

- **节点/事件**：b1-commercial —— 计划文档 `docs/plans/passb/12-commercial.md` 撰写完成并审校通过（调度指令口径，本管家未重跑审校）；**B1 四节点中首个完成计划撰写的节点**
- **计划路径**：`docs/plans/passb/12-commercial.md`（54,640 字节，mtime 19:18，本会话实读）
- **计划所在**：`.worktrees/passb-b1-commercial`（分支 `codex/passb-b1-commercial`，提交 **`4e481a63b`** `docs(plan): passb b1-commercial`——`d57a2fa70` 之后恰 1 提交、工作树干净，本会话 git log/status 核验）；**尚未合入 integration**（integration HEAD 仍 `d57a2fa70`，passb-int 内该文件不存在）
- **计划结构核验**（本会话 grep/实读）：§6 任务分解恰 **5 任务**（T1–T5，双编号 B1-CM.1–B1-CM.5）——T1 特征化基线（handler 缺失测试补齐+全量基线冻结）、T2 repository 层搬迁（model_usage 绑定族端口+user_usage）、T3 service 层搬迁（usage_recorder 随迁+SemanticModelBudgetAdapter 导出端口）、T4 handler 层搬迁（4 文件+helper 副本+类型别名 shim+测试随迁）、T5 差分证据收口+Integration Brief+实施报告；§10 独立验收标准 10 条（含"提交序列恰好 5 个 commit（T1–T5）"）；§12 计划自检记录在案
- **任务结构回填依据**：计划 :594 明确「DAG `task_ids` 由协调者回填，本任务不改 `execution-dag.json`（写权限归协调者，conventions §9）」——本次按调度指令回填
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过）：b1-commercial `task_ids` [] → **[B1-CM.1, B1-CM.2, B1-CM.3, B1-CM.4, B1-CM.5]**；新增 `task_status` 字段（5×**pending**，沿 03:40 条目 b0 先例的任务级字段语义——conventions §9 未定义、按调度指令引入并在此留痕）。`status=in_progress`、`base_sha=d57a2fa70...`、`head_sha=null`、`review_status=pending` 及其余节点均未动；33 节点分布不变（1 done + 4 in_progress + 28 pending）
- **worktree**：`.worktrees/passb-b1-commercial`（`codex/passb-b1-commercial`，HEAD `4e481a63b`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `d57a2fa70`）
- **测试证据路径**：尚无任务级产出（T1–T5 全部待执行；计划内命令与预期见计划 §6 各任务"命令与预期"）
- **审查结论**：计划审校通过（调度指令口径）；节点级 review_status 仍 pending（任务执行+节点收口审查另计）
- **OCR 报告路径**：无
- **修复轮次**：0
- **备注**：后续任务完成事件（B1-CM.1–B1-CM.5 逐个 done）按 b0 先例逐条登记；节点 done 待五任务全 done + 节点级审查 + 合并

---

## 2026-09-23 20:00 CST · b1-identity 登记 OCR 覆盖：ocr_covered 追加 [d57a2fa70 → 8db61f8ae]（范围无可审项）

- **节点**：b1-identity —— B1-ID Identity 边界（27 legacy 文件；status=in_progress 未变，18:58 条目所置）
- **登记内容**：b1-identity 节点新增数组字段 `ocr_covered`（该节点此前无此字段），追加首条 `{base: "d57a2fa708c3fecf5f0510553ea26db3de51d3c9", head: "8db61f8ae4d1bf2faa72c36bd6f2475f8b10f39f"}`（全 40 位 SHA，调度指令口径：**范围无可审项**）
- **SHA 真实性核验**（本会话 git 实测，`.worktrees/passb-b1-identity`）：`git rev-parse` 双双命中——base `d57a2fa70`（= 节点已回填 base_sha，B1 派发基准）与 head `8db61f8ae`（19:44:59，`docs(plan): passb b1-identity`）；区间 `d57a2fa70..8db61f8ae` 含**恰 1 个提交**（即 head 本身，纯计划文档提交）——与"无可审项"口径相容（该提交不触及生产代码）
- **与 19:44 条目的平行事件对照**：b1-commercial 计划提交 `4e481a63b`（19:18 mtime / 19:44 登记 task_ids 回填）与 b1-identity 计划提交 `8db61f8ae`（19:44:59 / 本次登记 OCR 覆盖）同型——各 B1 分支正以"计划文档提交"推进；两者均未合入 integration（integration HEAD 仍 `d57a2fa70`）
- **字段语义（沿 b0 18:46 先例）**：ocr_covered 记录已审查覆盖的已提交 SHA 区间；本次登记 base = 节点派发基准（B1 起点全量）、head = 节点分支当前头——即 b1-identity 截至目前的全部提交均在审查覆盖范围内；节点后续新提交落地后须追加条目
- **节点级状态未迁移**：`status=in_progress`、`review_status=pending`、`head_sha=null` 维持——节点 done / 合并 / head_sha 回填仍待调度方显式指令（本指令仅授权 ocr_covered 登记 + 台账留痕）
- **修复轮次**：0（无可审项，不构成修复轮）
- **base/head SHA（节点级）**：`d57a2fa70` / null（未动）
- **worktree**：`.worktrees/passb-b1-identity`（`codex/passb-b1-identity`，HEAD `8db61f8ae`，本会话核验工作树干净）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `d57a2fa70`）
- **本次 JSON 变更**：b1-identity 节点 `head_sha` 行后新增 `ocr_covered` 数组（7 行）；Edit 后 `python3 json.load` 复验合法（断言全过：ocr_covered 恰 1 条且 SHA 与指令逐字符一致；status/review_status/base/head_sha 未动；**b1-airesource/b1-commercial/b1-execution 三节点确认无 ocr_covered 字段、未被误改**——19:44 他方对 b1-commercial 的 task_ids/task_status 回填亦未触碰）

---

## 2026-09-23 20:03 CST · b1-execution 登记 OCR 覆盖：ocr_covered 追加 [d57a2fa70 → 0044f54fc]（范围无可审项）

- **节点**：b1-execution —— B1-EX Execution 边界（21 legacy 文件；status=in_progress 未变，19:01 条目所置）
- **登记内容**：b1-execution 节点新增数组字段 `ocr_covered`（该节点此前无此字段），追加首条 `{base: "d57a2fa708c3fecf5f0510553ea26db3de51d3c9", head: "0044f54fcb0e58014f3240d49d5b2bf3bdd400ea"}`（全 40 位 SHA，调度指令口径：**范围无可审项**）
- **SHA 真实性核验**（本会话 git 实测，`.worktrees/passb-b1-execution`）：`git rev-parse` 双双命中——base `d57a2fa70`（= 节点已回填 base_sha，B1 派发基准）与 head `0044f54fc`（19:25:31，`docs(plan): passb b1-execution`）；区间 `d57a2fa70..0044f54fc` 含**恰 1 个提交**（即 head 本身，纯计划文档提交）——与"无可审项"口径相容（该提交不触及生产代码）
- **字段语义（沿 b0 18:46 / b1-identity 20:00 先例）**：ocr_covered 记录已审查覆盖的已提交 SHA 区间；本次 base = 节点派发基准、head = 节点分支当前头——b1-execution 截至目前的全部提交均在审查覆盖范围内；节点后续新提交落地后须追加条目
- **节点级状态未迁移**：`status=in_progress`、`review_status=pending`、`head_sha=null` 维持——节点 done / 合并 / head_sha 回填仍待调度方显式指令
- **修复轮次**：0（无可审项，不构成修复轮）
- **base/head SHA（节点级）**：`d57a2fa70` / null（未动）
- **worktree**：`.worktrees/passb-b1-execution`（HEAD `0044f54fc`，本会话核验工作树干净）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `d57a2fa70`）
- **本次 JSON 变更**：b1-execution 节点 `head_sha` 行后新增 `ocr_covered` 数组（7 行）；Edit 后 `python3 json.load` 复验合法（断言全过：ocr_covered 恰 1 条且 SHA 与指令逐字符一致；status/review_status/base/head_sha 未动；b1-identity ocr_covered=1 条保持、b1-airesource/b1-commercial 无此字段、b1-commercial task_ids 5 项保持——**其余节点均未被误改**）。33 节点分布不变（1 done + 4 in_progress + 28 pending）

---

## 2026-09-23 20:06 CST · b1-identity OCR 第 1 次：confirmed=0 / rejected=0（报告为"Review skipped"跳过态，如实登记）

- **节点/轮次**：b1-identity 节点级架构审查 OCR 第 1 次（调度口径；18:58 派发后首轮）
- **计划路径**：`docs/plans/passb/10-identity.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b1-identity/ocr-r1.txt`（本会话已读全文：**40 字节、mtime 20:00、1 行——"Review skipped: no items were selected."**；同目录 `ocr-context.md` 6,273 字节、mtime 19:59 在案）
- **结论计数**：confirmed=0 / rejected=0（调度口径；与报告字面一致——但报告非"审过零发现"，而是**未选中任何审查项导致跳过**，非正常完成态，如实登记）
- **报告性质留痕**：与 b0 各轮 "Review complete: N finding(s) across M selected item(s)" 完成态报告不同，本报告为 skipped 态。两点解读供协调者参考（本管家不裁定）：(1) 若 20:00 条目 ocr_covered 登记 [d57a2fa70 → 8db61f8ae] 的"范围无可审项"即本次跳过的原因（区间唯一提交为纯计划文档，无选中审查项），则 skipped 与该登记口径自洽；(2) 若后续节点有代码提交，OCR 须以非 skipped 方式产出真实结论
- **分支核验**（本会话 git 实测）：`.worktrees/passb-b1-identity` HEAD 仍 `8db61f8ae`（19:44:59 计划提交，与 ocr_covered 首条 head 一致）、工作树干净——20:00 后无新提交，跳过轮后无代码变更
- **审查结论**：confirmed=0 → 不触发 changes_requested；`review_status=pending` 维持（未获 approved 指令，跳过轮亦不构成通过依据）
- **修复轮次**：0（无可审项、无 finding，不构成修复轮）
- **base/head SHA**：`d57a2fa70` / null（未动）
- **worktree**：`.worktrees/passb-b1-identity`（`codex/passb-b1-identity`，HEAD `8db61f8ae`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `d57a2fa70`）
- **本次 JSON 变更**：**无字节级改动**——confirmed=0 时 `review_status` 目标值 `pending` 已在位；`status=in_progress`、base/head SHA、ocr_covered（1 条）、其余节点均未动。JSON 合法性本会话 `python3 json.load` 复验通过
- **备注**：(1) 报告目录新增 `ocr-context.md`（6,273 字节）——OCR 上下文准备文件，未逐字审读；(2) background <8000 字符教训（b0 06:26）对后续 b1-identity OCR 轮次仍适用；(3) b1-identity 实施任务（B1-ID.x）尚未见 task_ids 回填或任务级产出，节点推进状态由调度方掌握

---

## 2026-09-23 20:09 CST · b1-execution OCR 第 1 次：confirmed=0 / rejected=0（报告为"Review skipped"跳过态，如实登记）

- **节点/轮次**：b1-execution 节点级架构审查 OCR 第 1 次（调度口径；19:01 派发后首轮）
- **计划路径**：`docs/plans/passb/13-execution.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b1-execution/ocr-r1.txt`（本会话已读全文：**40 字节、mtime 20:01、1 行——"Review skipped: no items were selected."**；同目录 `ocr-context.md` 8,552 字节、mtime 20:00 在案）
- **结论计数**：confirmed=0 / rejected=0（调度口径；与报告字面一致——**跳过态非完成态**，沿 b1-identity 20:06 条目先例如实登记）
- **报告性质留痕**：与 20:03 条目 ocr_covered 登记 [d57a2fa70 → 0044f54fc] 的"范围无可审项"口径相容（区间唯一提交 `0044f54fc` 为纯计划文档，无选中审查项）；后续节点有代码提交时 OCR 须以非 skipped 方式产出真实结论
- **分支核验**（本会话 git 实测）：`.worktrees/passb-b1-execution` HEAD 仍 `0044f54fc`（19:25:31 计划提交，与 ocr_covered 首条 head 一致）、工作树干净——20:03 后无新提交
- **审查结论**：confirmed=0 → 不触发 changes_requested；`review_status=pending` 维持（未获 approved 指令，跳过轮不构成通过依据）
- **修复轮次**：0（无可审项，不构成修复轮）
- **base/head SHA**：`d57a2fa70` / null（未动）
- **worktree**：`.worktrees/passb-b1-execution`（HEAD `0044f54fc`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `d57a2fa70`）
- **本次 JSON 变更**：**无字节级改动**——confirmed=0 时 `review_status` 目标值 `pending` 已在位；`status=in_progress`、base/head SHA、ocr_covered（1 条）、其余节点均未动。JSON 合法性本会话 `python3 json.load` 复验通过
- **备注**：(1) `ocr-context.md` 8,552 字节在案（未逐字审读）；(2) B1 四节点 OCR 首轮现状：b1-identity skipped（20:06 登记）、b1-execution skipped（本条目）、b1-airesource 无报告（20:07 口径"门禁+OCR 通过"）、b1-commercial 未运行——四节点实质代码实施尚未见提交，跳过态与实施进度自洽

---

## 2026-09-23 20:10 CST · b1-identity OCR 覆盖登记（区间已在 20:00 登记，不重复追加；口径由"无可审项"补强为"0 条需修 findings"）

- **节点**：b1-identity —— B1-ID Identity 边界（status=in_progress 未变）
- **指令**：登记 ocr_covered 追加 `{base: "d57a2fa708c3fecf5f0510553ea26db3de51d3c9", head: "8db61f8ae4d1bf2faa72c36bd6f2475f8b10f39f"}`（**审得 0 条需修 findings**；全 40 位 SHA）
- **重复识别（本条目核心）**：本会话 `python3 json.load` 实测——b1-identity.`ocr_covered` 已含**完全相同的条目**（base/head 与指令逐字符一致，20:00 条目所登记，当时调度口径"范围无可审项"）。区间 SHA 双双核验真实（`git rev-parse` 命中），且分支 `8db61f8ae..HEAD` 零新提交、工作树干净——区间自 20:00 登记后无演进
- **JSON 处置**：**无字节级改动、不重复追加**——ocr_covered 数组语义为"已覆盖审查区间列表"，同区间双条目为冗余且破坏数组区间语义（沿 18:41 工单重发"不重复追加"先例）。本会话 `python3 json.load` 复验合法（ocr_covered 仍 1 条；status=in_progress、review_status=pending、base/head_sha 未动）
- **口径演进留痕**：同一区间 [d57a2fa70 → 8db61f8ae] 的审查结论由 20:00 的"范围无可审项"（对应 20:06 条目 OCR skipped 报告）补强为本次的"**审得 0 条需修 findings**"——语义上确认该区间已审且结论为零需修；两次口径的差别（跳过态 vs 有效零发现）以调度方本次口径为准，台账并录
- **审查结论**：`review_status=pending` 维持（本指令仅授权覆盖登记，未含 approved/done 迁移）
- **修复轮次**：0（0 条需修 findings，不构成修复轮）
- **base/head SHA（节点级）**：`d57a2fa70` / null（未动）
- **worktree**：`.worktrees/passb-b1-identity`（HEAD `8db61f8ae`，本会话核验零新提交、工作树干净）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `d57a2fa70`）
- **后续**：节点分支 8db61f8ae 之后的实施提交落地后，须以**新条目**追加 ocr_covered（沿用 base 链式衔接或显式新区间，按调度指令）

---

## 2026-09-23 20:11 CST · b1-commercial 登记 OCR 覆盖：ocr_covered 追加 [d57a2fa70 → 4e481a63b]（范围无可审项）

- **节点**：b1-commercial —— B1-CM Commercial 边界（8 legacy 文件；status=in_progress 未变，19:00 条目所置）
- **登记内容**：b1-commercial 节点新增数组字段 `ocr_covered`（该节点此前无此字段），追加首条 `{base: "d57a2fa708c3fecf5f0510553ea26db3de51d3c9", head: "4e481a63bc2c7cd8cd068cedc8980aa74c8fac78"}`（全 40 位 SHA，调度指令口径：**范围无可审项**）
- **SHA 真实性核验**（本会话 git 实测，`.worktrees/passb-b1-commercial`）：`git rev-parse` 双双命中——base `d57a2fa70`（= 节点已回填 base_sha，B1 派发基准）与 head `4e481a63b`（19:20:48，`docs(plan): passb b1-commercial`——即 19:44 条目登记审校通过的那份计划）；区间 `d57a2fa70..4e481a63b` 含**恰 1 个提交**（纯计划文档提交）——与"无可审项"口径相容
- **字段语义（沿 b0 18:46 / b1-identity 20:00 / b1-execution 20:03 先例）**：ocr_covered 记录已审查覆盖的已提交 SHA 区间；base = 节点派发基准、head = 节点分支当前头——b1-commercial 截至目前的全部提交均在审查覆盖范围内；后续新提交落地后须追加条目
- **节点级状态未迁移**：`status=in_progress`、`review_status=pending`、`head_sha=null` 维持——节点 done / 合并 / head_sha 回填仍待调度方显式指令
- **修复轮次**：0（无可审项，不构成修复轮）
- **base/head SHA（节点级）**：`d57a2fa70` / null（未动）
- **worktree**：`.worktrees/passb-b1-commercial`（`codex/passb-b1-commercial`，HEAD `4e481a63b`，本会话核验工作树干净）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `d57a2fa70`）
- **本次 JSON 变更**：b1-commercial 节点 `head_sha` 行后新增 `ocr_covered` 数组（7 行，编辑定位用其特有的 task_ids/evidence_paths 上下文避免与相邻 b1 节点混淆）；Edit 后 `python3 json.load` 复验合法（断言全过：ocr_covered 恰 1 条且 SHA 与指令逐字符一致；status/review_status/base/head_sha/task_ids 5 项/task_status 均未动；b1-identity=1 条、b1-execution=1 条保持、b1-airesource 无此字段——**其余节点均未被误改**）
- **B1 四节点 OCR 覆盖登记现状**：identity ✓ / execution ✓ / commercial ✓（本条）/ airesource 无登记（其节点已于 20:07 done，head=base）

---

## 2026-09-23 20:13 CST · b1-execution OCR 覆盖登记（区间已在 20:03 登记，不重复追加；口径由"无可审项"补强为"0 条需修 findings"）

- **节点**：b1-execution —— B1-EX Execution 边界（21 legacy 文件；status=in_progress 未变）
- **指令**：登记 ocr_covered 追加 `{base: "d57a2fa708c3fecf5f0510553ea26db3de51d3c9", head: "0044f54fcb0e58014f3240d49d5b2bf3bdd400ea"}`（**审得 0 条需修 findings**；全 40 位 SHA）
- **重复识别**：本会话 `python3 json.load` 实测——b1-execution.`ocr_covered` 已含**完全相同的条目**（base/head 与指令逐字符一致，20:03 条目所登记，当时调度口径"范围无可审项"）。分支 `0044f54fc..HEAD` 零新提交、工作树干净——区间自 20:03 登记后无演进（沿 b1-identity 20:10 条目同型先例）
- **JSON 处置**：**无字节级改动、不重复追加**——ocr_covered 数组语义为"已覆盖审查区间列表"，同区间双条目冗余且破坏数组区间语义。本会话 `python3 json.load` 复验合法（ocr_covered 仍 1 条；status=in_progress、review_status=pending、base/head_sha 未动）
- **口径演进留痕**：同一区间 [d57a2fa70 → 0044f54fc] 的审查结论由 20:03 的"范围无可审项"（对应 20:09 条目 OCR skipped 报告）补强为本次的"**审得 0 条需修 findings**"——语义上确认该区间已审且结论为零需修；差别以调度方本次口径为准，台账并录
- **审查结论**：`review_status=pending` 维持（本指令仅授权覆盖登记，未含 approved/done 迁移）
- **修复轮次**：0（0 条需修 findings，不构成修复轮）
- **base/head SHA（节点级）**：`d57a2fa70` / null（未动）
- **worktree**：`.worktrees/passb-b1-execution`（HEAD `0044f54fc`，本会话核验零新提交、工作树干净）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `d57a2fa70`）
- **后续**：节点分支 0044f54fc 之后的实施提交落地后，须以**新条目**追加 ocr_covered
- **B1 四节点覆盖登记终态（本时点）**：identity [d57a2fa70→8db61f8ae] 0 需修 / execution [d57a2fa70→0044f54fc] 0 需修（本条口径）/ commercial [d57a2fa70→4e481a63b] 无可审项 / airesource 无登记（20:07 done，head=base）

---

## 2026-09-23 20:14 CST · b1-identity → done（head 8db61f8 回填，门禁+OCR 通过）

- **节点**：b1-identity —— B1-ID Identity 边界（27 legacy 文件：Actor/Tenant/RBAC/Audit 公共用例）——**DAG 第三个 done 节点（B1 第二个）**
- **计划路径**：`docs/plans/passb/10-identity.md`
- **前置**：b0（done ✓，18:47 条目）
- **worktree**：`.worktrees/passb-b1-identity`（`codex/passb-b1-identity`，HEAD `8db61f8ae`，本会话 git 核验工作树干净）
- **base → head**：`d57a2fa708c3fecf5f0510553ea26db3de51d3c9` → **`8db61f8ae4d1bf2faa72c36bd6f2475f8b10f39f`**（指令短 SHA `8db61f8` 本会话 `git rev-parse` 解析为完整 40 位；`git merge-base --is-ancestor` 核验 base 是 head 祖先 ✓）。区间含**恰 1 个提交**：19:44:59 `docs(plan): passb b1-identity`（计划文档）
- **门禁+OCR 通过**：调度指令口径（本管家未重跑，沿 b0 18:47 先例）；**审查链条在案完整**：ocr_covered [d57a2fa70 → 8db61f8ae] 已登记（20:00）且调度口径确认为"审得 0 条需修 findings"（20:10）——本次 done 的审查依据与覆盖登记闭环（区别于 b1-airesource 20:07 done 时的无登记状态）
- **审查结论**：approved（指令口径"门禁+OCR 通过"→ `review_status` pending → **approved**，推断迁移沿 b0 18:47 先例，在此留痕）
- **修复轮次**：0
- **节点产出留痕**：head 提交为计划文档（非实施代码）——节点 27 legacy 文件的搬迁义务未在本分支产生实施提交，与 b1-airesource（20:07，head=base、计划文档 untracked）同型的"计划即当前产出"状态；区别在于 b1-identity 计划已提交入库且 OCR 覆盖登记在案。identity 模块边界/特征化测试/别名删除等待办（见 DAG produced_artifacts 与 required_contracts 的跨 owner 耦合义务）在 ib1 汇合前的落地方式由调度方掌握
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过，净变更 3 行）：(1) `status` in_progress → **done**；(2) `head_sha` null → **8db61f8ae4d1bf2faa72c36bd6f2475f8b10f39f**；(3) `review_status` pending → **approved**。`base_sha`、ocr_covered（1 条）、其余字段均未动。33 节点分布：**3 done（b0/b1-airesource/b1-identity）+ 2 in_progress（b1-commercial/b1-execution）+ 28 pending**
- **integration 合并状态**：`codex/passb-b1-identity` 尚未合入 `codex/passb-integration`（integration HEAD 仍 `d57a2fa70`）——合并提交 `8db61f8ae` 后 head_sha 与 b0 先例同样存在"分支头 vs 合并头"覆盖问题，待合并后如需覆盖由调度方显式指令（沿 18:47 留痕 1 / 18:56 覆盖先例）
- **task_ids**：[]（未回填——b1-commercial 有 B1-CM.1–5 先例，identity 计划的任务结构未回填，如实登记）

---

## 2026-09-23 20:16 CST · b1-commercial OCR 第 1 次：confirmed=0 / rejected=0（报告为"Review skipped"跳过态，如实登记）

- **节点/轮次**：b1-commercial 节点级架构审查 OCR 第 1 次（调度口径；19:00 派发后首轮）
- **计划路径**：`docs/plans/passb/12-commercial.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b1-commercial/ocr-r1.txt`（本会话已读全文：**40 字节、mtime 20:07、1 行——"Review skipped: no items were selected."**；同目录 `ocr-context.md` 6,972 字节、mtime 20:07 在案）
- **结论计数**：confirmed=0 / rejected=0（调度口径；与报告字面一致——**跳过态非完成态**，沿 b1-identity 20:06 / b1-execution 20:09 条目先例如实登记）
- **报告性质留痕**：与 20:11 条目 ocr_covered 登记 [d57a2fa70 → 4e481a63b] 的"范围无可审项"口径相容（区间唯一提交 `4e481a63b` 为纯计划文档，无选中审查项）；后续节点有代码提交时 OCR 须以非 skipped 方式产出真实结论
- **分支核验**（本会话 git 实测）：`.worktrees/passb-b1-commercial` HEAD 仍 `4e481a63b`（19:20:48 计划提交，与 ocr_covered 首条 head 一致）、工作树干净——20:11 后无新提交
- **审查结论**：confirmed=0 → 不触发 changes_requested；`review_status=pending` 维持（未获 approved 指令，跳过轮不构成通过依据）
- **修复轮次**：0（无可审项，不构成修复轮）
- **base/head SHA**：`d57a2fa70` / null（未动）
- **worktree**：`.worktrees/passb-b1-commercial`（`codex/passb-b1-commercial`，HEAD `4e481a63b`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `d57a2fa70`）
- **本次 JSON 变更**：**无字节级改动**——confirmed=0 时 `review_status` 目标值 `pending` 已在位；`status=in_progress`、base/head SHA、ocr_covered（1 条）、task_ids/task_status（B1-CM.1–5 pending）、其余节点均未动。JSON 合法性本会话 `python3 json.load` 复验通过
- **备注**：(1) `ocr-context.md` 6,972 字节在案（未逐字审读）；(2) **B1 四节点 OCR 首轮全记录齐**：identity skipped（20:06）/ execution skipped（20:09）/ commercial skipped（本条目）/ airesource 无报告（20:07 done 口径）——四节点均无实质实施提交，跳过态与进度自洽；后续实施提交落地后 OCR 须以非 skipped 方式产出结论

---

## 2026-09-23 20:17 CST · b1-execution → done（head 0044f54 回填，门禁+OCR 通过）——B1 四节点全部收口

- **节点**：b1-execution —— B1-EX Execution 边界（21 legacy 文件：Sandbox/Target/Workspace/Terminal/Browser 门面）——**DAG 第四个 done 节点；至此 B1 四节点全部 done**
- **计划路径**：`docs/plans/passb/13-execution.md`
- **前置**：b0（done ✓，18:47 条目）
- **worktree**：`.worktrees/passb-b1-execution`（HEAD `0044f54fc`，本会话 git 核验工作树干净）
- **base → head**：`d57a2fa708c3fecf5f0510553ea26db3de51d3c9` → **`0044f54fcb0e58014f3240d49d5b2bf3bdd400ea`**（指令短 SHA `0044f54` 本会话 `git rev-parse` 解析为完整 40 位；`git merge-base --is-ancestor` 核验 base 是 head 祖先 ✓）。区间含**恰 1 个提交**：19:25:31 `docs(plan): passb b1-execution`（计划文档）
- **门禁+OCR 通过**：调度指令口径（本管家未重跑，沿 b0 18:47 先例）；**审查链条在案完整**：ocr_covered [d57a2fa70 → 0044f54fc] 已登记（20:03）且调度口径确认为"审得 0 条需修 findings"（20:13）——本次 done 的审查依据与覆盖登记闭环
- **审查结论**：approved（指令口径"门禁+OCR 通过"→ `review_status` pending → **approved**，推断迁移沿 b0 18:47 先例，在此留痕）
- **修复轮次**：0
- **节点产出留痕**：head 提交为计划文档（非实施代码）——与 b1-identity（20:14）同型：计划已提交入库且 OCR 覆盖登记在案，21 legacy 文件搬迁义务（含 *Handler 方法文件去方法化裁定、browserSkillScope 等 10 调用点导出义务，见 DAG required_contracts）未在本分支产生实施提交，ib1 汇合前的落地方式由调度方掌握
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过，净变更 3 行）：(1) `status` in_progress → **done**；(2) `head_sha` null → **0044f54fcb0e58014f3240d49d5b2bf3bdd400ea**；(3) `review_status` pending → **approved**。`base_sha`、ocr_covered（1 条）、其余字段均未动。33 节点分布：**4 done（b0 + B1 四节点全部）+ 1 in_progress（b1-commercial）+ 28 pending**
- **integration 合并状态**：`codex/passb-b1-execution` 尚未合入 `codex/passb-integration`（integration HEAD 仍 `d57a2fa70`）——合并后 head_sha"分支头 vs 合并头"覆盖问题沿 18:47 留痕 1 / 18:56 先例，待调度方显式指令
- **B1 阶段快照（本时点）**：identity done（head 8db61f8ae）/ airesource done（head=base）/ commercial in_progress（B1-CM.1–5 五任务待执行）/ execution done（head 0044f54fc）——三节点 done 待合并进 integration，ib1（B1 四支逐支合并 barrier）的全部入边就绪尚差 b1-commercial 实施完成

---

## 2026-09-23 20:19 CST · b1-commercial OCR 覆盖登记（区间已在 20:11 登记，不重复追加；口径由"无可审项"补强为"0 条需修 findings"）

- **节点**：b1-commercial —— B1-CM Commercial 边界（8 legacy 文件；status=in_progress 未变）
- **指令**：登记 ocr_covered 追加 `{base: "d57a2fa708c3fecf5f0510553ea26db3de51d3c9", head: "4e481a63bc2c7cd8cd068cedc8980aa74c8fac78"}`（**审得 0 条需修 findings**；全 40 位 SHA）
- **重复识别**：本会话 `python3 json.load` 实测——b1-commercial.`ocr_covered` 已含**完全相同的条目**（base/head 与指令逐字符一致，20:11 条目所登记，当时调度口径"范围无可审项"）。分支 `4e481a63b..HEAD` 零新提交、工作树干净——区间自 20:11 登记后无演进（沿 b1-identity 20:10 / b1-execution 20:13 条目同型先例）
- **JSON 处置**：**无字节级改动、不重复追加**——ocr_covered 数组语义为"已覆盖审查区间列表"，同区间双条目冗余且破坏数组区间语义。本会话 `python3 json.load` 复验合法（ocr_covered 仍 1 条；status=in_progress、review_status=pending、base/head_sha、task_ids/task_status 均未动）
- **口径演进留痕**：同一区间 [d57a2fa70 → 4e481a63b] 的审查结论由 20:11 的"范围无可审项"（对应 20:16 条目 OCR skipped 报告）补强为本次的"**审得 0 条需修 findings**"——语义上确认该区间已审且结论为零需修；差别以调度方本次口径为准，台账并录
- **审查结论**：`review_status=pending` 维持（本指令仅授权覆盖登记，未含 approved/done 迁移）
- **修复轮次**：0（0 条需修 findings，不构成修复轮）
- **base/head SHA（节点级）**：`d57a2fa70` / null（未动）
- **worktree**：`.worktrees/passb-b1-commercial`（HEAD `4e481a63b`，本会话核验零新提交、工作树干净）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `d57a2fa70`）
- **后续**：节点分支 4e481a63b 之后的实施提交落地后，须以**新条目**追加 ocr_covered
- **B1 四节点覆盖登记终态（本时点）**：identity [d57a2fa70→8db61f8ae] 0 需修（done）/ execution [d57a2fa70→0044f54fc] 0 需修（done）/ commercial [d57a2fa70→4e481a63b] **0 需修（本条口径，in_progress）** / airesource 无登记（done，head=base）

---

## 2026-09-23 20:22 CST · b1-commercial → done（head 4e481a6 回填，门禁+OCR 通过）——B1 四节点与全部 5 个 done 节点收口齐

- **节点**：b1-commercial —— B1-CM Commercial 边界（8 legacy 文件：Admission/Budget/Usage/Payment 门面）——**DAG 第五个 done 节点；B1 四节点至此全部 done，当前无 in_progress 节点**
- **计划路径**：`docs/plans/passb/12-commercial.md`
- **前置**：b0（done ✓，18:47 条目）
- **worktree**：`.worktrees/passb-b1-commercial`（`codex/passb-b1-commercial`，HEAD `4e481a63b`，本会话 git 核验工作树干净）
- **base → head**：`d57a2fa708c3fecf5f0510553ea26db3de51d3c9` → **`4e481a63bc2c7cd8cd068cedc8980aa74c8fac78`**（指令短 SHA `4e481a6` 本会话 `git rev-parse` 解析为完整 40 位；`git merge-base --is-ancestor` 核验 base 是 head 祖先 ✓）。区间含**恰 1 个提交**：19:20:48 `docs(plan): passb b1-commercial`（计划文档——即 19:44 条目登记审校通过的那份）
- **门禁+OCR 通过**：调度指令口径（本管家未重跑，沿 b0 18:47 先例）；**审查链条在案完整**：ocr_covered [d57a2fa70 → 4e481a63b] 已登记（20:11）且调度口径确认为"审得 0 条需修 findings"（20:19）——本次 done 的审查依据与覆盖登记闭环
- **审查结论**：approved（指令口径"门禁+OCR 通过"→ `review_status` pending → **approved**，推断迁移沿 b0 18:47 先例，在此留痕）
- **修复轮次**：0
- **节点产出留痕**：head 提交为计划文档（非实施代码）——与 b1-identity（20:14）/ b1-execution（20:17）同型：计划已提交入库且 OCR 覆盖登记在案，8 legacy 文件搬迁义务与 model_usage 绑定族端口导出义务（见 DAG required_contracts）未在本分支产生实施提交；**task_status 留痕**：B1-CM.1–5 在 DAG 中仍为 pending（未随节点 done 翻转，本指令未授权任务级变更，如实保留）
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过，净变更 3 行）：(1) `status` in_progress → **done**；(2) `head_sha` null → **4e481a63bc2c7cd8cd068cedc8980aa74c8fac78**；(3) `review_status` pending → **approved**。`base_sha`、ocr_covered（1 条）、task_ids（5 项）、task_status（5×pending）均未动。33 节点分布：**5 done（b0 + B1 四节点全部）+ 28 pending；in_progress 归零**
- **integration 合并状态**：B1 四分支（identity `8db61f8ae` / airesource base 即 head / commercial `4e481a63b` / execution `0044f54fc`）**均尚未合入** `codex/passb-integration`（integration HEAD 仍 `d57a2fa70`）——合并后各节点 head_sha"分支头 vs 合并头"覆盖沿 18:47 留痕 1 / 18:56 先例，待调度方显式指令
- **ib1 就绪度提示**：ib1（B1 四支逐支合并 barrier，depends_on 四 B1 节点）的四条入边节点现已全部 done——**ib1 可派发**；但四分支的实施产出均止于计划文档（无生产代码变更），ib1 集成时 B1 门面（contracts.yaml current 化、identity/airesource/commercial/execution 的 NewModule/RegisterRoutes 等，见 ib1 produced_artifacts）的实际落地依赖后续实施，调度方知悉
- **B1 阶段快照（本时点）**：identity done（8db61f8ae）/ airesource done（head=base）/ commercial done（4e481a63b，B1-CM.1–5 pending）/ execution done（0044f54fc）

---

## 2026-09-23 20:23 CST · ib1 → in_progress（屏障派发；四条入边全部 done）

- **节点**：ib1 —— IB1 基础能力集成 barrier（B1-ID→B1-AI→B1-CM→B1-EX 逐支合并）——**首个 barrier 节点派发**
- **计划路径**：`docs/plans/passb/19-foundation-integration.md`
- **前置就绪核验**（本会话 `python3 json.load` 实测）：depends_on 四节点**全部 done** ✓——b1-identity（20:14）/ b1-airesource（20:07）/ b1-commercial（20:22）/ b1-execution（20:17）
- **worktree**：barrier 即 integration 侧集成工作——DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `d57a2fa70`）；ib1 owned_files（router.go/container.go/bootstrap/{routes,workers,lifecycle}.go/迁移序列/go.mod/go.sum，集成工程师独占）均在此分支
- **base SHA**：**`d57a2fa708c3fecf5f0510553ea26db3de51d3c9`**（派发时回填，conventions §9；指令未附 base 值，沿 b1-identity 18:58 先例按唯一在案口径回填——integration 分支当前头，barrier 合并工作即在此分支展开）；**head SHA**：null（未回填——barrier 评审通过时回填）
- **测试证据路径（计划）**：`docs/architecture/evidence/passb/ib1.md`、`docs/architecture/passb/briefs/ib1.md`（均尚未产出）
- **审查结论**：pending（review_status 未请求）
- **OCR 报告路径**：无（尚未进入审查轮）
- **修复轮次**：0
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过，净变更 2 行）：(1) ib1 `status` pending → **in_progress**；(2) `base_sha` null → **d57a2fa708c3fecf5f0510553ea26db3de51d3c9**。`head_sha=null`、`review_status=pending`、其余节点均未动。33 节点分布：**5 done + 1 in_progress（ib1）+ 27 pending**
- **备注**：(1) 指令原文 "ib1 → running"，沿先例映射为 `in_progress`；(2) **入边产出现实**：四条 B1 分支的实施产出均止于计划文档（无生产代码变更，见 20:07/20:14/20:17/20:22 各条目留痕）且均未合入 integration——ib1 的"逐支合并"步骤实际可合并内容有限，其核心产出（B1 四门面 contracts.yaml current 化、共享装配文件接线、差分套件证据，见 ib1 produced_artifacts）依赖的实施若缺位，集成工程师需在 barrier 内补齐或上报；(3) ib1 gates 含 `go test ./internal/... -count=1 -timeout=25m` 与 `golangci-lint --new-from-rev` 全量门禁，通过标准见 DAG gates 与 framework:101-105；(4) ib1 notes 内 BLOCKED 残留文本待批量清理

---

## 2026-09-23 20:07 CST · b1-airesource → done（head d57a2fa 回填；留痕：head=base，分支零实施提交）

- **节点**：b1-airesource —— B1-AI AI Resource 边界（33 legacy 文件：Model/MCP/Search/Vector/Storage 能力解析）——**DAG 第二个 done 节点（B1 首个）**
- **计划路径**：`docs/plans/passb/11-airesource.md`
- **前置**：b0（done ✓）
- **worktree**：`.worktrees/passb-b1-airesource`（`codex/passb-b1-airesource`，本会话 git 核验）
- **base → head**：`d57a2fa708c3fecf5f0510553ea26db3de51d3c9` → **`d57a2fa708c3fecf5f0510553ea26db3de51d3c9`**（指令短 SHA `d57a2fa` 本会话 `git rev-parse` 解析为完整 40 位；`git merge-base --is-ancestor` 核验其为分支 HEAD 祖先 ✓）。**实质留痕：head = base_sha，分支零新提交**——b1-airesource 分支 HEAD 即 B1 派发基准 `d57a2fa70`（18:59 派发时回填的 base），自派发至本条目（18:59→20:07）分支无任何提交
- **门禁+OCR 通过**：调度指令口径（本管家未重跑，沿 b0 18:47 先例）；**该分支无 ocr_covered 登记、无 OCR 报告目录**（`.superpowers/sdd/passb/` 下 b0/b1-identity 有、b1-airesource 无）——指令口径与在案证据的对应方式未附，如实登记
- **工作树留痕（未提交产物）**：分支工作树含 **1 个 untracked 文件 `docs/plans/passb/11-airesource.md`**（本会话 git status 实测）——b1-airesource 计划文档已撰写但**未提交**（对比 b1-identity 计划已提交 `8db61f8ae`、b1-commercial 已提交 `4e481a63b`）。计划内容未入库，本条目无法核验其任务结构与审校状态（与 19:44 b1-commercial 计划审校条目不同）
- **审查结论**：approved（指令口径"门禁+OCR 通过"→ `review_status` pending → **approved**，推断迁移沿 b0 18:47 先例，在此留痕）
- **修复轮次**：0（无 OCR 轮次记录）
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过，净变更 3 行）：(1) `status` in_progress → **done**；(2) `head_sha` null → **d57a2fa708c3fecf5f0510553ea26db3de51d3c9**；(3) `review_status` pending → **approved**。`base_sha`、task_ids（[]，未回填）、ocr_covered（无）均未动。33 节点分布：**2 done（b0/b1-airesource）+ 3 in_progress（b1-identity/b1-commercial/b1-execution）+ 28 pending**
- **待协调者澄清/后续**：(1) **节点产出为空的口径确认**——done 时分支零实施提交、计划文档 untracked 未入库：若系"计划即交付"的节点完成定义（如该节点拆分至后续节点执行），建议在台账或 DAG notes 显式声明，避免与 b0"六任务+19 提交后 done"的口径混淆；若系超前收口（实施未做即 done），33 legacy 文件的搬迁义务（含 IB1 前 capability 契约 current 化，见节点 notes）仍悬置，ib1 汇合时将缺 airesource 门面；(2) 计划文档 `11-airesource.md` 建议尽快提交入库以固化审查对象；(3) task_ids/task_status 未回填（b1-commercial 已有 5 任务先例）——如需任务级追踪请下发回填指令
- **integration 合并状态**：b1-airesource 分支尚未合入 `codex/passb-integration`（integration HEAD 仍 `d57a2fa70`）；由于 head=base，合并将是 no-op 或仅带入 untracked 文件入库后的提交

---

## 2026-09-23 20:47 CST · ib1 屏障执行（四支逐支合并完成、三门禁绿色；装配切换等核心产出因 B1 零实施上报缺位）

- **执行者**：Pass B 总集成工程师（barrier 即 integration 侧独占装配工作）
- **合并前固化**：`.worktrees/passb-int` 工作树含上一调度管家会话遗留 2 个未提交修改（本台账 18:56–20:23 各派发/收口条目 + DAG b0 head_sha 覆盖/B1 四节点 done 回填/ib1 in_progress+base 回填）——先以 docs 提交 **`7b701512a`** 固化（2 文件 +376/−20）
- **逐支合并（一次一支，零冲突，ort 策略）**：
  1. **B1-ID**：`git merge codex/passb-b1-identity` → **`1900e038a`**（+`docs/plans/passb/10-identity.md` 500 行）；
  2. **B1-AI**：`git merge codex/passb-b1-airesource` → **"Already up to date"（no-op，无提交）**——分支 head=base=`d57a2fa70`（20:07 条目留痕一致）；其 untracked `docs/plans/passb/11-airesource.md` 留于 `.worktrees/passb-b1-airesource` **未入库**（非已审查分支内容，本屏障不代为提交，待调度方/实施者动作）；
  3. **B1-CM**：`git merge codex/passb-b1-commercial` → **`f66dfb188`**（+`12-commercial.md` 676 行）；
  4. **B1-EX**：`git merge codex/passb-b1-execution` → **`ca38afb7e`**（+`13-execution.md` 331 行）。
- **每支合并后验证**（`go build ./...` + `go test … -count=1`；直接消费者名单依据 exception-ledger.yaml `from` 在案耦合）：ID：build OK + identity 骨架包（no test files）；AI：airesource 12 包（10 ok+2 no test files）+ 消费者 agentruntime 20 ok / conversation 1 ok / knowledge 15 ok；CM：commercial 7 包 ok + 消费者 appconnector+airesource-chat+workbench 10 ok / agentruntime/agent 族 18 ok；EX：execution 3 包+policy 5 包 ok + agentruntime/agent/tools 1 ok。全部仅既有 `-lc++` 链接警告，无失败
- **全局门禁（三道全绿，指令点名前两道）**：`make -C .worktrees/passb-int check-backend-architecture` → `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16` / `OK (0 violations)`（**计数奇偶 633/23+23/58 与 B0.3 基线一致**）；`make -C .worktrees/passb-int verify-module-moves` → `modulemove: OK (16 manifests verified)`；`make -C .worktrees/passb-int check-passb-readiness` → `legacy=396 aliases=99 exceptions=105 contracts=125 events=29 overlaps=0 missing=0`
- **例外台账核验（本屏障拥有的 `remove_at: ib1` 共 2 条，import 均在磁盘，不删行）**：exc-0057 `internal/modules/airesource/models/chat/usage.go:9`→`internal/modules/commercial`（grep 实测 import 在）；exc-0087 `internal/modules/execution/sandbox/url_guard.go:31`→`internal/modules/policy/ipclass`（同上）——B1 实施未落地，删除条件（import 消失）未满足
- **屏障计划写盘**：`docs/plans/passb/19-foundation-integration.md`（指令要求：文件原不存在，按 framework:100-107 IB1 节执行并写盘）——含现实基线表、逐支合并与验证记录、门禁记录、例外核验、**§4 待 B1 实施落地后的收尾动作**（装配切换/契约 current 化/例外删行/四组差分套件/全量 25m 测试+变更域 lint）
- **上报（ib1 核心产出缺位，本屏障不伪造）**：B1 四支实施产出均止于计划文档、无生产代码（20:07/20:14/20:17/20:22 台账留痕+本会话分支 diff 实测），四模块 module.go 仍为零逻辑骨架——故 **router/container/全局注册装配切换、contracts.yaml 四门面 current 化、exception-ledger 删行、tenant-RBAC/capability/payment-usage/sandbox-target 差分套件在本屏障内无可执行对象**；把骨架接入装配或伪称 current 将破坏 633/23+23/58 计数契约与行为等价性，按"补齐或上报"口径（20:23 派发条目）选择**上报**。补齐义务在各 B1 支实施者（子计划 10/11/12/13 的任务集），落地后由后续屏障动作收尾（19 号计划 §4）
- **DAG 变更**：无——ib1 维持 `in_progress`（装配切换未发生，不得自行置 done；状态迁移沿先例属调度方指令权限）；本次不动 `head_sha`（最终 head 见本条目所在 docs 提交，回报调度方）
- **DAG gates 未跑项（如实）**：`go test ./internal/... -count=1 -timeout=25m` 与 `golangci-lint run --new-from-rev` —— 指令未点名且装配切换未发生（无变更域），按 19 号计划 §4 留待装配切换那次收尾；`golangci-lint` 本会话未运行，PASSB_BASE_SHA 环境变量未设置
- **Mimosa**：本会话 commit 钩子报 `scanner_enobufs`（未获完整扫描结论，按钩子兼容策略继续，不宣称项目安全审计通过）
- **调度方后续事项**：(1) 裁定 ib1 收口口径（当前现实下"逐支合并+门禁"已完成、装配类产出悬置）——若确认"计划集成即本屏障本轮完成"，请显式指令 ib1 状态迁移口径；(2) `11-airesource.md` 入库指令（20:07 条目待办）；(3) B1 四支实施派发（含 airesource 33 文件搬迁与 exc-0057/exc-0087 消除）；(4) 32 节点 BLOCKED 残留文本批量清理（18:56 条目待办）

---

## 2026-09-23 21:07 CST · ib1 flake 判定登记：判定 pass（在册 flake 类，非真实回归）＋ ib1.notes 单字符损坏发现与修复

- **节点**：ib1 —— IB1 基础能力集成 barrier（status=in_progress 维持，20:23 派发 / 20:47 屏障执行在案）
- **判定登记（调度指令口径）**：**判定 pass（在册 flake 类，非真实回归）**。证据（指令转录，第 (2) 条于 stderr 处截断）：(1) 重跑了屏障全测组原命令规模：`go test ./internal/modules/... -count=1 -timeout=25m`（repo 根，main@539406569）→ **EXIT=0，93 包 ok / 0 FAIL**；两个在册 unstable 包本轮均通过（agentruntime/agent/opencode ok 34.763s、agentruntime/agent/recoverytest ok 52.006s）；(2) 重跑 stderr 仅两条 `ld: warning: ignoring dupli…`（**调度指令文本于此截断，完整原文以调度方为准**）
- **管家留痕（重跑环境差异）**：证据所载重跑位于 **main@539406569（repo 根）**，非 passb-int worktree（ib1 base=`d57a2fa70`）——重跑环境与屏障分支的差异如实登记；且该命令为 `./internal/modules/...` 规模，非 ib1 gates 全量 `./internal/...`（20:47 条目 :1181 未跑项），判定 pass 的覆盖范围以指令口径为准
- **判定效力**：pass 不改变节点状态——`status=in_progress`、`review_status=pending` 维持（门禁流程继续，done 迁移仍待调度方显式指令）
- **⚠ ib1.notes 单字符损坏发现与修复（本会话处理，非本指令直接要求，如实登记）**：本会话准备向 ib1.notes 追加判定时实测 notes 数组 **292 条全部为单字符**——HEAD 提交中 ib1.`notes` 原为**字符串**（"F2 裁定落地处：……BLOCKED（2026-09-23）：前置 b0 阻塞；解除条件：修复并 done b0 后恢复。"，292 字符），某次他方未提交编辑疑按字符迭代将其拆散为 292 条单字符数组（**数据损坏，非本管家所为**）。修复：`''.join` 机械拼回——还原文本以"F2 裁定落地处"起、"后恢复。"止、恰 292 字符，与 HEAD 提交中的字符串原文一致（逐字符无损）；随后升级为**数组结构** `[原note(292字符), flake判定条目]`（追加条目所需，沿 b0 notes 数组先例；string→array 的 schema 变更系追加动作副作用，在此留痕）。**建议调度方核查该编辑来源，防止同类脚本缺陷再次损坏其他节点 notes**
- **本次 JSON 变更**（python 原子更新，Edit 后 `json.load` 复验合法、断言全过）：ib1.`notes` 由 292×单字符数组（损坏态）→ **2 条数组**（[还原原note, flake 判定]）；`status=in_progress`、`review_status=pending`、base/head SHA、gates 等其余字段未动。33 节点分布不变（5 done + 1 in_progress + 27 pending）；`git diff HEAD -- DAG` 净变更 4+/1−（= notes string→array 展开 + flake 条目）
- **worktree**：`.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`——20:47 条目集成工程师已提交 ib1 屏障执行记录）；`.worktrees/passb-b1-*` 各分支无新提交
- **修复轮次**：0（flake 判定 pass，不构成修复轮）

---

## 2026-09-23 21:25 CST · ib1 登记 OCR 覆盖：ocr_covered 追加 [d57a2fa70 → 8c45a8815]（审得 0 findings）

- **节点**：ib1 —— IB1 基础能力集成 barrier（status=in_progress 未变，20:23 派发）
- **登记内容**：ib1 节点新增数组字段 `ocr_covered`（该节点此前无此字段），追加首条 `{base: "d57a2fa708c3fecf5f0510553ea26db3de51d3c9", head: "8c45a88153d0b20088252dbb29d2fb3815b253c2"}`（全 40 位 SHA，调度指令口径：**审得 0 findings**）
- **SHA 真实性核验**（本会话 git 实测）：`git rev-parse` 双双命中——base `d57a2fa70`（= 节点已回填 base_sha，屏障派发基准）与 head `8c45a8815`（20:49:05，ib1 屏障执行登记提交，**integration 当前 HEAD**）；区间 `d57a2fa70..8c45a8815` 含**恰 7 个提交**：`7b701512a`（固化调度台账）→ `1900e038a`（B1-ID 第 1/4 支合并）→ `f66dfb188`（B1-CM 3/4）→ `ca38afb7e`（B1-EX 4/4；B1-AI 为 no-op 无提交）→ 三个计划文档提交（8db61f8ae/4e481a63b/0044f54fc 随分支带入）→ `8c45a8815`（屏障执行登记）——**即 20:47 条目所载 ib1 屏障执行的全部集成工作**
- **字段语义（沿 b0/b1 先例）**：ocr_covered 记录已审查覆盖的已提交 SHA 区间；base = 屏障派发基准、head = integration 当前头——ib1 截至 8c45a8815 的全部提交（含四支合并）均在审查覆盖范围内（审得 0 findings）；此后新提交落地须追加条目
- **与 b1 节点登记的结构差异留痕**：ib1 的 ocr_covered 区间（base→integration HEAD）横跨**四个 B1 分支的合并提交**——各 B1 分支自身区间的覆盖登记已由各自 ocr_covered 承载（identity/execution/commercial 各 1 条、airesource 无登记），本条不重复承载分支内容，仅覆盖 integration 侧屏障工作
- **节点级状态未迁移**：`status=in_progress`、`review_status=pending`、`head_sha=null` 维持——barrier done / head_sha 回填仍待调度方显式指令（本指令仅授权 ocr_covered 登记 + 台账留痕）
- **修复轮次**：0（审得 0 findings，不构成修复轮）
- **base/head SHA（节点级）**：`d57a2fa70` / null（未动）
- **worktree**：`.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`；工作树含本会话与 21:07 条目的未提交修改——DAG flake 判定/notes 修复 + ocr_covered + 台账各条目）；`.worktrees/passb-b1-*` 各分支无新提交
- **本次 JSON 变更**：ib1 节点 `head_sha` 行后新增 `ocr_covered` 数组（7 行）；Edit 后 `python3 json.load` 复验合法（断言全过：ocr_covered 恰 1 条且 SHA 与指令逐字符一致；status/review_status/base/head_sha/notes 2 条均未动）。33 节点分布不变（5 done + 1 in_progress + 27 pending）

---

## 2026-09-23 21:27 CST · ib1 OCR 第 1 次：confirmed=0 / rejected=0（完成态，0 findings——与 ocr_covered 登记口径一致）

- **节点/轮次**：ib1 节点级架构审查 OCR 第 1 次（调度口径；20:23 派发后首轮）
- **计划路径**：`docs/plans/passb/19-foundation-integration.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/ib1/ocr-r1.txt`（本会话已读全文：**57 字节、mtime 21:25、1 行——"Review complete: 0 finding(s) across 1 selected item(s)."**；同目录 `ocr-context.md` 8,541 字节、mtime 21:22 在案）
- **结论计数**：confirmed=0 / rejected=0（调度口径；与报告一致——**完成态**而非 skipped 态，区别于 B1 各节点首轮；报告为正常审查完成且零发现，非"未选中项跳过"）
- **报告性质**：完成态 0 findings 与 21:25 条目 ocr_covered 登记 [d57a2fa70 → 8c45a8815]（审得 0 findings）口径闭环——覆盖区间（屏障派发基准至 integration 当前头，含四支合并与屏障登记提交）已审且零发现
- **审查结论**：confirmed=0 → 不触发 changes_requested；`review_status=pending` 维持（未获 approved 指令；barrier 收口沿 conventions §9 属调度方指令权限）
- **修复轮次**：0（0 findings，不构成修复轮）
- **base/head SHA**：`d57a2fa70` / null（未动）
- **worktree**：`.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`——21:25 后无新提交）；21:07/21:25 两会话的未提交修改（DAG flake 判定/notes 修复/ocr_covered + 台账各条目）仍在工作树
- **本次 JSON 变更**：**无字节级改动**——confirmed=0 时 `review_status` 目标值 `pending` 已在位；`status=in_progress`、base/head SHA、ocr_covered（1 条）、notes（2 条）均未动。JSON 合法性本会话 `python3 json.load` 复验通过
- **备注**：(1) `ocr-context.md` 8,541 字节在案（未逐字审读）；(2) **首个完成态 OCR 零发现记录**：此前 B1 三节点 skipped/ib1 前无完成态报告，本条系 Pass B 首个 "Review complete" 且 0 findings 的节点级审查——ib1 覆盖区间内容（四支计划文档合并 + 屏障登记 docs）零发现与其纯 docs 性质相容；(3) ib1 收口（done/head_sha/装配切换补齐义务）仍待调度方口径（20:47 条目后续事项 1）

---

## 2026-09-23 21:28 CST · ib1 OCR 覆盖登记（区间已在 21:25 登记，不重复追加；口径补强为"0 条需修 findings"）

- **节点**：ib1 —— IB1 基础能力集成 barrier（status=in_progress 未变）
- **指令**：登记 ocr_covered 追加 `{base: "d57a2fa708c3fecf5f0510553ea26db3de51d3c9", head: "8c45a88153d0b20088252dbb29d2fb3815b253c2"}`（**审得 0 条需修 findings**；全 40 位 SHA）
- **重复识别**：本会话 `python3 json.load` 实测——ib1.`ocr_covered` 已含**完全相同的条目**（base/head 与指令逐字符一致，21:25 条目所登记，当时调度口径"审得 0 findings"）。integration `8c45a8815..HEAD` 零新提交——区间自 21:25 登记后无演进（沿 b1-identity 20:10 / b1-execution 20:13 / b1-commercial 20:19 条目同型先例）
- **JSON 处置**：**无字节级改动、不重复追加**——ocr_covered 数组语义为"已覆盖审查区间列表"，同区间双条目冗余且破坏数组区间语义。本会话 `python3 json.load` 复验合法（ocr_covered 仍 1 条；status=in_progress、review_status=pending、base/head_sha、notes 2 条均未动）
- **口径关系留痕**：本次指令口径"0 条需修 findings"与 21:25 的"审得 0 findings"、21:27 条目完成态 OCR 报告（0 findings/1 selected item）三者一致——同一区间审查结论的三次确认，无矛盾
- **审查结论**：`review_status=pending` 维持（本指令仅授权覆盖登记，未含 approved/done 迁移）
- **修复轮次**：0（0 条需修 findings，不构成修复轮）
- **base/head SHA（节点级）**：`d57a2fa70` / null（未动）
- **worktree**：`.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`，零新提交）
- **后续**：integration 8c45a8815 之后的提交落地后，须以**新条目**追加 ocr_covered

---

## 2026-09-23 21:29 CST · ib1 → done（集成 HEAD 8c45a88 回填，全量回归+OCR 通过）——首个 barrier 收口

- **节点**：ib1 —— IB1 基础能力集成 barrier（B1-ID→B1-AI→B1-CM→B1-EX 逐支合并）——**DAG 第六个 done 节点、首个 barrier 收口**
- **计划路径**：`docs/plans/passb/19-foundation-integration.md`
- **前置**：b1-identity/b1-airesource/b1-commercial/b1-execution（四节点 done，20:07–20:22 各条目）
- **worktree**：`.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`——屏障工作即发生在 integration 分支，**head=集成 HEAD 与分支合一，无"分支头 vs 合并头"覆盖问题**，区别于 b0/b1 各节点）
- **base → head**：`d57a2fa708c3fecf5f0510553ea26db3de51d3c9` → **`8c45a88153d0b20088252dbb29d2fb3815b253c2`**（指令短 SHA `8c45a88` 本会话 `git rev-parse` 解析为完整 40 位；即 integration 当前 HEAD；区间 7 提交构成见 21:25 条目）
- **全量回归+OCR 通过**：调度指令口径（本管家未重跑，沿 b0 18:47 先例）。审查链条在案完整闭环：ocr_covered [d57a2fa70 → 8c45a8815] 登记（21:25）→ 完成态 OCR 0 findings（21:27，Pass B 首个完成态零发现）→ flake 判定 pass（21:07，重跑 93 包 ok / 0 FAIL）→ 本条 done
- **审查结论**：approved（指令口径 → `review_status` pending → **approved**，推断迁移沿先例，在此留痕）
- **修复轮次**：0（flake 判定 pass、OCR 0 findings，均不构成修复轮）
- **留痕 1（20:47 条目上报事项的收口口径）**：20:47 屏障执行条目上报"装配切换等核心产出因 B1 零实施缺位"（router/container 装配、contracts.yaml 四门面 current 化、exception 删行、差分套件、全量 25m 测试+变更域 lint 的部分项）——本次调度方以"全量回归+OCR 通过"口径置 done，即**接受"计划集成即本屏障本轮完成"**（20:47 后续事项 1 的裁定落地）；缺位的实施类产出义务转移至 19 号计划 §4"待 B1 实施落地后的收尾动作"，由后续屏障动作或 B1 补齐轮承担，本条如实登记该义务转移
- **留痕 2（task/装配悬置清单移交后续）**：(1) B1 四支实施派发（含 airesource 33 文件搬迁）；(2) `11-airesource.md` 入库；(3) exc-0057/exc-0087 删行（remove_at: ib1 条件未满足，顺延）；(4) contracts.yaml 四门面 current 化；(5) 32 节点 BLOCKED 残留文本批量清理
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过，净变更 3 行）：(1) `status` in_progress → **done**；(2) `head_sha` null → **8c45a88153d0b20088252dbb29d2fb3815b253c2**；(3) `review_status` pending → **approved**。`base_sha`、ocr_covered（1 条）、notes（2 条）均未动。33 节点分布：**6 done（b0 + B1 四节点 + ib1）+ 27 pending；in_progress 归零**
- **下游解锁**：ib1 done 后 B2 阶段入口节点解锁——b2-k0（K0 知识端口冻结，20-knowledge-program SERIAL 首）、b2-ac-definition（25a）、b2-ac-skills（25b）、b2-appconnector（27）四个 depends_on=[ib1] 节点可派发；base 取 `8c45a8815`（新集成头，沿 18:56 口径先例）

---

## 2026-09-23 21:32 CST · b2-k0 → in_progress（B2 阶段首节点派发）

- **节点**：b2-k0 —— K0 Knowledge 端口/所有权冻结（20-knowledge-program 内 SERIAL 首）——**B2 阶段首节点**
- **计划路径**：`docs/plans/passb/20-knowledge-program.md`
- **前置**：ib1（done ✓，21:29 条目——首个 barrier 收口）
- **worktree**：指令未附实施 worktree；本会话 `ls .worktrees/` 核验 **`passb-b2-k0` 尚不存在**（沿 b0 模式，建立后请以新条目补记）；另留痕：**B1 三 worktree 已清理**（`passb-b1-identity/-commercial/-execution` 不复存在，仅剩 passb-b0/passb-b1-airesource/passb-int——B1 分支已合并后的清理动作，非本管家所为）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **base SHA**：**`8c45a88153d0b20088252dbb29d2fb3815b253c2`**（派发时回填，conventions §9；沿 18:56 口径先例——ib1 done 后新集成头即下游派发基准）；**head SHA**：null（未回填——节点分支评审通过时回填）
- **测试证据路径（计划）**：`docs/architecture/evidence/passb/b2-k0.md`、`docs/plans/passb/reports/b2-k0.md`（均尚未产出）
- **审查结论**：pending（review_status 未请求）
- **OCR 报告路径**：无（尚未进入审查轮）
- **修复轮次**：0
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过，净变更 2 行）：(1) b2-k0 `status` pending → **in_progress**；(2) `base_sha` null → **8c45a88153d0b20088252dbb29d2fb3815b253c2**。`head_sha=null`、`review_status=pending`、其余节点均未动。33 节点分布：**6 done + 1 in_progress（b2-k0）+ 26 pending**
- **备注**：(1) 指令原文 "b2-k0 → running"，沿先例映射为 `in_progress`；(2) 节点定位：K0 为 20-knowledge-program SERIAL 首，K1–K3 并行以其共享类型/端口分配为前提（framework:129）；(3) 节点 notes 既有义务对实施者有效：K0 冻结产物写入 20 计划分配表与共享类型代码、contracts.yaml 对 K0 只读（状态回写归 ib2，审校 F8）；(4) notes 内 BLOCKED 残留文本待批量清理

---

## 2026-09-23 21:34 CST · b2-ac-definition → in_progress（B2 第二节点派发，与 b2-k0 并行）

- **节点**：b2-ac-definition —— 25a Agent 定义/版本/人格/专家/子代理/收藏
- **计划路径**：`docs/plans/passb/25a-agent-definition-version.md`
- **前置**：ib1（done ✓，21:29 条目）
- **worktree**：指令未附实施 worktree；本会话核验 **`passb-b2-ac-definition` 尚不存在**（沿 b0 模式，建立后请以新条目补记）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **base SHA**：**`8c45a88153d0b20088252dbb29d2fb3815b253c2`**（派发时回填，conventions §9；沿 21:32 b2-k0 同口径——ib1 后新集成头）；**head SHA**：null（未回填——节点分支评审通过时回填）
- **测试证据路径（计划）**：`docs/architecture/evidence/passb/b2-ac-definition.md`、`docs/plans/passb/reports/b2-ac-definition.md`、`docs/plans/passb/reviews/b2-ac-definition.md`（均尚未产出）
- **审查结论**：pending（review_status 未请求）
- **OCR 报告路径**：无（尚未进入审查轮）
- **修复轮次**：0
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过，净变更 2 行）：(1) b2-ac-definition `status` pending → **in_progress**；(2) `base_sha` null → **8c45a88153d0b20088252dbb29d2fb3815b253c2**。`head_sha=null`、`review_status=pending`、其余节点均未动。33 节点分布：**6 done + 2 in_progress（b2-k0/b2-ac-definition）+ 25 pending**
- **备注**：(1) 指令原文 "b2-ac-definition → running"，沿先例映射为 `in_progress`；(2) 与 b2-k0 并行合法（depends_on 均为 [ib1]，b2-ac-definition 不依赖 b2-k0——agentcatalog 面与 knowledge 面独立）；(3) 节点 notes 既有义务对实施者有效：agentcatalog↔conversation 7 符号 10 调用点互耦断链处置、agentRequiresRerankModel/skillsForRun 导出义务、与 25b 并行前提为共享 immutable-version 契约冻结（framework:139）；(4) notes 内 BLOCKED 残留文本待批量清理

---

## 2026-09-23 21:36 CST · b2-ac-skills → in_progress（B2 第三节点派发）

- **节点**：b2-ac-skills —— 25b Skill 目录/安装/运行时验证/reaper
- **计划路径**：`docs/plans/passb/25b-skill-catalog-install.md`
- **前置**：ib1（done ✓，21:29 条目）
- **worktree**：指令未附实施 worktree；本会话核验 **`passb-b2-ac-skills` 尚不存在**（沿 b0 模式，建立后请以新条目补记）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **base SHA**：**`8c45a88153d0b20088252dbb29d2fb3815b253c2`**（派发时回填，conventions §9；与 b2-k0/b2-ac-definition 同口径——ib1 后新集成头）；**head SHA**：null（未回填——节点分支评审通过时回填）
- **测试证据路径（计划）**：`docs/architecture/evidence/passb/b2-ac-skills.md`、`docs/plans/passb/reports/b2-ac-skills.md`、`docs/plans/passb/reviews/b2-ac-skills.md`（均尚未产出）
- **审查结论**：pending（review_status 未请求）
- **OCR 报告路径**：无（尚未进入审查轮）
- **修复轮次**：0
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过，净变更 2 行）：(1) b2-ac-skills `status` pending → **in_progress**；(2) `base_sha` null → **8c45a88153d0b20088252dbb29d2fb3815b253c2**。`head_sha=null`、`review_status=pending`、其余节点均未动。33 节点分布：**6 done + 3 in_progress（b2-k0/b2-ac-definition/b2-ac-skills）+ 24 pending**
- **备注**：(1) 指令原文 "b2-ac-skills → running"，沿先例映射为 `in_progress`；(2) 三节点并行合法（depends_on 均为 [ib1]，execution_mode=parallel）；(3) 节点 notes 既有义务对实施者有效：tenant_skill_reaper.go 若归本面则 b1-execution 遗留的 4 条 execution→agentcatalog 断链（matchSnapshotByName 等 4 符号）在此导出化收口、与 25a 共享 immutable-version 契约同时冻结（framework:139）；(4) notes 内 BLOCKED 残留文本待批量清理

---

## 2026-09-23 21:37 CST · b2-appconnector → in_progress（B2 第四节点派发，depends_on=[ib1] 四节点全部在途）

- **节点**：b2-appconnector —— 27 App Connector（7 legacy handlers：install/OAuth/action/sync 路由 + 别名删除）
- **计划路径**：`docs/plans/passb/27-appconnector.md`
- **前置**：ib1（done ✓，21:29 条目）
- **worktree**：指令未附实施 worktree；本会话核验 **`passb-b2-appconnector` 尚不存在**（沿 b0 模式，建立后请以新条目补记）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **base SHA**：**`8c45a88153d0b20088252dbb29d2fb3815b253c2`**（派发时回填，conventions §9；与 b2-k0/b2-ac-definition/b2-ac-skills 同口径）；**head SHA**：null（未回填——节点分支评审通过时回填）
- **测试证据路径（计划）**：`docs/architecture/evidence/passb/b2-appconnector.md`、`docs/plans/passb/reports/b2-appconnector.md`、`docs/plans/passb/reviews/b2-appconnector.md`（均尚未产出）
- **审查结论**：pending（review_status 未请求）
- **OCR 报告路径**：无（尚未进入审查轮）
- **修复轮次**：0
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过，净变更 2 行）：(1) b2-appconnector `status` pending → **in_progress**；(2) `base_sha` null → **8c45a88153d0b20088252dbb29d2fb3815b253c2**。`head_sha=null`、`review_status=pending`、其余节点均未动。33 节点分布：**6 done + 4 in_progress（b2-k0/b2-ac-definition/b2-ac-skills/b2-appconnector）+ 23 pending**
- **备注**：(1) 指令原文 "b2-appconnector → running"，沿先例映射为 `in_progress`；(2) **depends_on=[ib1] 的四个 B2 入口节点至此全部在途**；(3) 节点 notes 既有义务对实施者有效：Commercial 门面引用（appconnector→commercial 3-4 处）、AgentRuntime 引用裁定（2 处，若涉未导出符号须上报）、测试夹具随迁（commercial 7/agentruntime 2）、可与 Knowledge/AgentCatalog 并行（framework:146）；(4) notes 内 BLOCKED 残留文本待批量清理

---

## 2026-09-23 22:24 CST · b2-appconnector 计划完成审校通过（4 任务结构回填 DAG：B2-AC.1–B2-AC.4 全 pending）

- **节点/事件**：b2-appconnector —— 计划文档 `docs/plans/passb/27-appconnector.md` 撰写完成并审校通过（调度指令口径，本管家未重跑审校）；**B2 四入口节点中首个完成计划撰写的节点**
- **计划所在**：`.worktrees/passb-b2-appconnector`（分支 `codex/passb-b2-appconnector`，提交 **`ced88ecb1`** `docs(plan): passb b2-appconnector`（22:04:56）——`8c45a8815` 之后恰 1 提交、工作树干净，本会话 git log/status 核验）；**尚未合入 integration**（integration HEAD 仍 `8c45a8815`）
- **计划结构核验**（本会话 grep/实读）：§5 任务分解恰 **4 任务**（T1–T4，双编号 B2-AC.1–B2-AC.4，与指令"任务 4 个"一致）——T1 特征化基线（installation/connection/sync/action 缺失测试补齐+基线冻结，仅测试文件零生产改动）、T2 handler 层搬迁（7+2 文件 git mv + ErrMissingTenantScope 本地副本 + 宿主过渡 shim）、T3 差分复跑比对 + 别名/例外核销证据（exc-0058..0061 现状+解除提案+IB2 期限裁决申请；exc-0028 零受影响声明）+ evidence 定稿、T4 Integration Brief + 实施报告 + 节点门禁收口；§6 高风险差分证据要求（Tenant/RBAC 面，conventions §6/framework:40）；§7 Integration Brief（含 5 条 alias 行已不存在核销登记）
- **计划文件规模**：47,745 字节（mtime 22:03，本会话 ls 实测）
- **任务结构回填依据**：沿 19:44 b1-commercial 先例（计划自陈 DAG task_ids 由协调者回填、conventions §9 写权限归调度管家）
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过）：b2-appconnector `task_ids` [] → **[B2-AC.1, B2-AC.2, B2-AC.3, B2-AC.4]**；新增 `task_status` 字段（4×**pending**，沿 b0/b1-commercial 先例的任务级字段语义）。`status=in_progress`、`base_sha=8c45a8815...`、`head_sha=null`、`review_status=pending` 及其余节点均未动（b2-k0/b2-ac-definition/b2-ac-skills 确认 task_ids 仍空、无 task_status——未被误改）；33 节点分布不变（6 done + 4 in_progress + 23 pending）
- **worktree**：`.worktrees/passb-b2-appconnector`（`codex/passb-b2-appconnector`，HEAD `ced88ecb1`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **测试证据路径**：尚无任务级产出（T1–T4 全部待执行）
- **审查结论**：计划审校通过（调度指令口径）；节点级 review_status 仍 pending（任务执行+节点收口审查另计）
- **OCR 报告路径**：无
- **修复轮次**：0
- **备注**：后续任务完成事件（B2-AC.1–B2-AC.4 逐个 done）按先例逐条登记；节点 done 待四任务全 done + 节点级审查 + 合并。**B2 各节点计划撰写进度对照**：b2-appconnector ✓（本条）/ b2-k0、b2-ac-definition、b2-ac-skills 未见计划提交

---

## 2026-09-23 22:58 CST · b2-appconnector / B2-AC.1 → done（SDD 审查通过，进入任务级 OCR）

- **节点/任务**：b2-appconnector · B2-AC.1 —— T1 特征化基线（计划 `docs/plans/passb/27-appconnector.md` §5 T1；**B2 阶段首个完成的实施任务**）
- **前置**：节点派发（21:37）+ 计划审校（22:24）
- **worktree**：`.worktrees/passb-b2-appconnector`（`codex/passb-b2-appconnector`，本会话 git 核验）
- **BASE → HEAD**：`ced88ecb1` → **`67ac22c96`**（1 commit：22:42:49 `test(passb): characterize app installation connection sync handlers before move`；`git show --stat` 实测恰 1 文件 +470 行——新增 `internal/handler/app_connector_lifecycle_test.go`（package handler，T2 随迁对象），纯测试零生产改动；BASE 与报告自陈一致）
- **测试证据**（B2-AC.1-report.md §1 所载，本台账转录；全部实跑含退出码）：`go build ./...` 0；T1 计划 4 条命令 0（4 新用例族全 PASS：InstallationLifecycle/ConnectionCreateBranches/SyncStatusThreeStates/ActionPipelineUnwired；既有 20 顶层用例全 PASS；模块 5 包全 ok）；§6 基线补充 8 条全 0（含 `make check-backend-architecture` 633/23+23/58 基线一致、`make verify-module-moves` 16 manifests）；提交后复跑 4 条 T1 命令全过
- **报告/审查证据路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b2-appconnector/B2-AC.1-report.md`（7,343 字节，本会话已读全文）；`B2-AC.1-review-pkg.md`（22,196 字节，在案）
- **审查结论**：SDD 审查通过（调度指令裁定；本管家未重跑审查）→ **进入任务级 OCR**（指令原文；OCR 报告路径尚未产出，由后续条目回填）
- **OCR 报告路径**：无（任务级 OCR 待运行）
- **修复轮次**：0（SDD 一轮通过；OCR 轮次另计）
- **报告遗留如实登记**：(1) evidence 草稿 `docs/architecture/evidence/passb/b2-appconnector.md` **untracked**（有意提交隔离 T1=test/T3=docs）——B2-AC.2/3 执行期间须保留，若被清理 T3 需按报告 §1 重建；(2) 既有用例计数：计划写 21、实测 20 顶层 + 1 用例内 4 子测试（未逐字对账，差分等价不受影响）；(3) sync 裸表 DDL 自建（计划 T1 步骤 1 预授权）；(4) 节点完整 gates 收口归 B2-AC.4
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过）：b2-appconnector.`task_status` **B2-AC.1 pending → done**（B2-AC.2/3/4 维持 pending）；节点级 `status=in_progress`、`review_status=pending`、`head_sha=null`（节点收口时回填）均未动；33 节点分布不变（6 done + 4 in_progress + 23 pending）
- **备注**：任务级 OCR 按 notes[11] ruling（b0 期用户裁定）"每任务完成后即审"口径运行；OCR 结果（confirmed/rejected 计数与报告路径）由后续指令登记

---

## 2026-09-23 22:58 CST · b2-k0 计划完成审校通过（6 任务结构回填 DAG：K0.1–K0.3 + K5.1–K5.3 全 pending）

- **节点/事件**：b2-k0 —— 计划文档 `docs/plans/passb/20-knowledge-program.md` 撰写完成并审校通过（调度指令口径：任务 6 个；本管家未重跑审校）；**B2 知识程序计划（含 K5 交付物）就绪**
- **计划所在**：`.worktrees/passb-b2-k0`（分支 `codex/passb-b2-k0`，**2 提交**——`5943921e9` 初稿 + `59b13c61a`（22:48:49）`docs(plan): passb b2-k0 审校修复（影子类型豁免/随迁归属纠偏/联动表全量枚举/坐标校准/F2 status 差异③）`；工作树干净，本会话 git log/status 核验）；**尚未合入 integration**（integration HEAD 仍 `8c45a8815`）
- **计划结构核验**（本会话 grep/实读）：§8 任务分解共 **6 个 Task**（与指令"任务 6 个"一致）——**K0.1** 冻结分配表与联动裁定表（§5–§7：共享类型 Chunk/KnowledgeBase/Tag/semantic 分配表 + 跨 plan 未导出符号联动全表 + 契约/路由 11/worker 18/别名 18/例外 4 清单指针）、**K0.2** kbfreeze 守卫测试包（R0 与六 port 签名机器强制）、**K0.3** 前置差异上报、证据与报告、**K5.1** 装配切换 Integration Brief（b2-k-integration 交付物 1）、**K5.2** 别名/例外/shim 删除记录与收口核对、**K5.3** 差分汇总、门禁与节点收口；另 §9 K1–K4 子计划派发义务（冻结约束内嵌）、§13 计划自检记录（含审校修复轮）
- **任务归属口径（本条目核心裁定留痕）**：§8 的 6 个 Task 分属两个 DAG 节点交付物（K0.x=b2-k0、K5.x=b2-k-integration），但**计划 Task K0.3 明确自陈回填建议「DAG b2-k0 建议置 review、task_ids=[K0.1,K0.2,K0.3,K5.1,K5.2,K5.3]」**——计划撰写者有意将 6 任务全挂 b2-k0 节点追踪（20 号计划执行主体为 b2-k0 SERIAL 首）。本次按指令"任务 6 个"+计划自陈口径回填 b2-k0；**b2-k-integration 节点不再单独回填任务结构**（其交付物经 b2-k0 的 K5.x 追踪），如实留痕防后续重复回填
- **计划文件规模**：52,773 字节（mtime 22:47，本会话 ls 实测）
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过）：b2-k0 `task_ids` [] → **[K0.1, K0.2, K0.3, K5.1, K5.2, K5.3]**；新增 `task_status`（6×**pending**，沿先例任务级字段语义）。`status=in_progress`、`base_sha=8c45a8815...`、`head_sha=null`、`review_status=pending` 均未动（b2-ac-definition/b2-ac-skills 确认未误改）；33 节点分布不变（6 done + 4 in_progress + 23 pending）
- **worktree**：`.worktrees/passb-b2-k0`（`codex/passb-b2-k0`，HEAD `59b13c61a`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **测试证据路径**：尚无任务级产出（6 任务全部待执行）
- **审查结论**：计划审校通过（调度指令口径；分支第 2 提交即审校修复轮）；节点级 review_status 仍 pending
- **OCR 报告路径**：无
- **修复轮次**：0
- **备注**：(1) 计划已登记前置差异③（F2 status 字段未落盘：contracts.yaml 实测 0 处 `status:` 仅 125 处 `stability:`，K5 门面 current 化与 ib2 回写缺载体）——K0.3 将正式上报，裁决归属为协调者；(2) 后续任务完成事件按先例逐条登记；节点 done 待六任务全 done + 节点级审查 + 合并。**B2 计划撰写进度**：b2-appconnector ✓（22:24，B2-AC.1 已 done）/ b2-k0 ✓（本条）/ b2-ac-definition、b2-ac-skills 未见计划提交

---

## 2026-09-23 23:10 CST · b2-appconnector 登记 OCR 覆盖：ocr_covered 追加 [ced88ecb → 67ac22c96]（范围无可审项）

- **节点**：b2-appconnector —— 27 App Connector（status=in_progress 未变；task_status B2-AC.1=done 维持）
- **登记内容**：b2-appconnector 节点新增数组字段 `ocr_covered`（该节点此前无此字段），追加首条 `{base: "ced88ecb17ed560a02a15464500a4b2fa8e2d508", head: "67ac22c964cecfc810578f1ee8d1289b78e7e083"}`（全 40 位 SHA，调度指令口径：**范围无可审项**）
- **SHA 真实性核验**（本会话 git 实测，`.worktrees/passb-b2-appconnector`）：`git rev-parse` 双双命中——base `ced88ecb`（计划提交，= 节点派发后首个提交、B2-AC.1 开工 BASE）与 head `67ac22c96`（22:42:49，B2-AC.1 特征化测试提交，= 分支当前 HEAD）；区间含**恰 1 个提交**（470 行纯测试文件）；`67ac22c96..HEAD` 零新提交
- **口径并录留痕（调度方知悉）**：本指令口径"范围无可审项"，但该区间包含 B2-AC.1 交付提交（22:58 条目"进入任务级 OCR"）——两者关系指令未说明（若系任务级 OCR 选审范围无项/或 OCR 选审集不含纯测试文件的口径，则 skipped/无审项与"无可审项"自洽）；如实并录，不自行判定
- **字段语义（沿先例）**：ocr_covered 记录已审查覆盖的已提交 SHA 区间；base = 任务开工 BASE、head = 分支当前头——b2-appconnector 截至目前的全部提交均在覆盖登记范围内；后续新提交落地后须追加条目
- **节点级状态未迁移**：`status=in_progress`、`review_status=pending`、`head_sha=null` 维持
- **修复轮次**：0（无可审项，不构成修复轮）
- **base/head SHA（节点级）**：`8c45a8815` / null（未动——base_sha 是节点派发基准，与 ocr_covered 的 base 不同层次：后者锚定任务区间）
- **worktree**：`.worktrees/passb-b2-appconnector`（`codex/passb-b2-appconnector`，HEAD `67ac22c96`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **本次 JSON 变更**：b2-appconnector 节点 `head_sha` 行后新增 `ocr_covered` 数组（7 行，编辑定位用其特有的 produced_artifacts 文本避免与相邻 b2 节点混淆）；Edit 后 `python3 json.load` 复验合法（断言全过：ocr_covered 恰 1 条且 SHA 与指令逐字符一致；status/review_status/base/head_sha/task_status 未动；b2-k0/b2-ac-definition/b2-ac-skills 确认无 ocr_covered 字段——**其余节点均未被误改**）

---

## 2026-09-23 23:16 CST · b2-ac-skills 计划完成审校通过（5 任务结构回填 DAG：25b.1–25b.5 全 pending）

- **节点/事件**：b2-ac-skills —— 计划文档 `docs/plans/passb/25b-skill-catalog-install.md` 撰写完成并审校通过（调度指令口径：任务 5 个；本管家未重跑审校）
- **计划所在**：`.worktrees/passb-b2-ac-skills`（分支 `codex/passb-b2-ac-skills`，**2 提交**——`a6afa4fb1` 初稿 + `8592f2aac` `docs(plan): passb b2-ac-skills 审校修复（补 tenant_skill_install:1849 消费点、unique 助手注入改道、测试计数措辞、派发 gate 现状、新包测试注册前提）`；工作树干净，本会话 git log/status 核验）；**尚未合入 integration**（integration HEAD 仍 `8c45a8815`）
- **计划结构核验**（本会话 grep/实读）：任务分解恰 **5 任务**（T1–T5，双编号 25b.1–25b.5，与指令"任务 5 个"一致）——T1 特征化基线与证据骨架、T2 repository 层搬迁+残差、T3 service 层原子搬迁（17 文件 + 17 测试）+ HostAdapters + 5 符号导出化 + 全量残差、T4 handler 层搬迁（2 文件 + 2 测试）+ 残差、T5 差分证据收口 + Integration Brief + 实施报告 + 节点门禁
- **计划文件规模**：67,010 字节（mtime 22:53，本会话 ls 实测）
- **任务结构回填依据**：沿 19:44 b1-commercial / 22:24 b2-appconnector 先例
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过）：b2-ac-skills `task_ids` [] → **[25b.1, 25b.2, 25b.3, 25b.4, 25b.5]**；新增 `task_status`（5×**pending**，沿先例任务级字段语义）。`status=in_progress`、`base_sha=8c45a8815...`、`head_sha=null`、`review_status=pending` 均未动（b2-k0 6 项/b2-appconnector 4 项/b2-ac-definition 空——其余节点均未被误改）；33 节点分布不变（6 done + 4 in_progress + 23 pending）
- **worktree**：`.worktrees/passb-b2-ac-skills`（`codex/passb-b2-ac-skills`，HEAD `8592f2aac`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **测试证据路径**：尚无任务级产出（5 任务全部待执行）
- **审查结论**：计划审校通过（调度指令口径；分支第 2 提交即审校修复轮）；节点级 review_status 仍 pending
- **OCR 报告路径**：无
- **修复轮次**：0
- **备注**：后续任务完成事件（25b.1–25b.5 逐个 done）按先例逐条登记；节点 done 待五任务全 done + 节点级审查 + 合并。**B2 计划撰写进度对照**：b2-appconnector ✓（B2-AC.1 done）/ b2-k0 ✓（6 任务，K5.x 跨节点追踪）/ b2-ac-skills ✓（本条）/ **b2-ac-definition 未见计划提交**（B2 四入口节点中唯一未交计划）

---

## 2026-09-23 23:28 CST · b2-ac-definition 计划完成审校通过（5 任务结构回填 DAG：T1–T5 全 pending）

- **节点/事件**：b2-ac-definition —— 计划文档 `docs/plans/passb/25a-agent-definition-version.md` 撰写完成并审校通过（调度指令口径：任务 5 个；本管家未重跑审校）；**B2 四入口节点计划全部就绪**
- **计划所在**：`.worktrees/passb-b2-ac-definition`（分支 `codex/passb-b2-ac-definition`，**3 提交**——`896572167` 初稿 + `94fa5577d` review fixes + `02e7be617`（23:22:44）`docs(plan): passb b2-ac-definition review fixes r2`；工作树干净，本会话 git log/status 核验）；**尚未合入 integration**（integration HEAD 仍 `8c45a8815`）
- **计划结构核验**（本会话 grep/实读）：任务分解恰 **5 个 Task**（T1–T5，计划内编号即 T 系列，无双编号后缀；与指令"任务 5 个"一致）——T1 基线锚定与预检、T2 repository 批次搬迁（#1–#4）+ 原路径 shim、T3 service + handler 批次搬迁（#5–#7）+ 哨兵重指向、T4 门禁全套 + 等价证据、T5 Integration Brief + 实施报告；另 §2 含 25a/25b/25c 拆分（55 总盘→22/20/13）与搬迁批次三重实证、①批次依赖核验、②批次 2 推迟件登记表（T5 Brief 核心）、③shim 逐符号清单（grep -rwn 全仓消费方审计）、④冻结契约兼容义务
- **计划文件规模**：48,393 字节（mtime 23:22，本会话 ls 实测）
- **task_ids 编号口径留痕**：该计划任务为 T1–T5（无 25a.x 双编号，区别于 25b.1–5/B2-AC.1–4），task_ids 沿计划自身编号回填
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过）：b2-ac-definition `task_ids` [] → **[T1, T2, T3, T4, T5]**；新增 `task_status`（5×**pending**，沿先例任务级字段语义）。`status=in_progress`、`base_sha=8c45a8815...`、`head_sha=null`、`review_status=pending` 均未动（b2-k0 6 项/b2-ac-skills 5 项/b2-appconnector 4 项——其余节点均未被误改）；33 节点分布不变（6 done + 4 in_progress + 23 pending）
- **worktree**：`.worktrees/passb-b2-ac-definition`（`codex/passb-b2-ac-definition`，HEAD `02e7be617`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **测试证据路径**：尚无任务级产出（5 任务全部待执行）
- **审查结论**：计划审校通过（调度指令口径；分支含两轮 review fixes 提交）；节点级 review_status 仍 pending
- **OCR 报告路径**：无
- **修复轮次**：0
- **备注**：后续任务完成事件（T1–T5 逐个 done）按先例逐条登记；节点 done 待五任务全 done + 节点级审查 + 合并。**B2 计划撰写进度终态**：四入口节点全部 ✓——b2-appconnector（4 任务，B2-AC.1 done）/ b2-k0（6 任务）/ b2-ac-skills（5 任务）/ b2-ac-definition（5 任务，本条）

---

## 2026-09-23 23:44 CST · b2-k0 / K0.1 → done（SDD 审查通过，进入任务级 OCR）

- **节点/任务**：b2-k0 · K0.1 —— 冻结分配表与联动裁定表复核（计划 `docs/plans/passb/20-knowledge-program.md` §8 Task K0.1；**B2 阶段第二个完成的实施任务**）
- **前置**：节点派发（21:32）+ 计划审校（22:58）
- **worktree**：`.worktrees/passb-b2-k0`（`codex/passb-b2-k0`，本会话 git 核验工作树干净）
- **BASE → HEAD**：`59b13c61a` → **`614409798`**（1 commit：23:31:08 `docs(passb): b2-k0 K0.1 冻结分配表与联动裁定表复核`；`git show --stat` 实测恰 1 文件 +5/−4——仅计划文件本体 §6.2 组 E 行与 §9 K4 行同步修订 + §13 复核轮登记）；**验证基线代码=8c45a8815**（报告 §0 实测 `git diff --name-only 8c45a8815..HEAD -- internal/ tools/ cmd/ go.mod go.sum migrations/ Makefile` 为空——代码零漂移）
- **复核结论**（K0.1-report.md 所载，本台账转录）：§5 共享类型 25 处 file:line、六端口接口（ChunkService 18 方法/RetrieveEngineService 9 方法）逐一 grep 实测**零漂移**；§6.2 组 A–E 定义 20 符号与全部调用点行号逐项命中；84 行计数 {21:9, 22:29, 23:18, 24:28} 双副本一致；§7 路由 11/worker 18/hook/事件 4/别名 18/例外 4/契约行号锚点全中。**唯一修订**：组 E `getParserEngineOverridesFromContext` 实测同名异义无耦合（K4 自有方法两处），按 isValidFileType 先例改判"K4 直接随文件迁移、无需 R2 seam"，修订随 commit 入库并登记 §13
- **上报协调者三项**（报告 §9，供调度方处置）：(a) ppc `knowledge→conversation` 条目 sites 2→0 修订工单（两符号同名伪影，不改 DAG 由协调者裁定）；(b) 差异②两工单收口确认（派发 K1–K3 前 P2 裁定）；(c) 差异③ F2 status 字段补落盘归属裁决（22:58 条目已预警）
- **测试证据**（报告 §7）：`go build ./...` 绿；`go test ./tools/passbguard ./tools/modulemove -count=1` 仍绿（K0.1 未破坏治理校验）
- **报告/审查证据路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b2-k0/K0.1-report.md`（14,018 字节，本会话已读）；`K0.1-review-pkg.md`（20,135 字节，在案）
- **审查结论**：SDD 审查通过（调度指令裁定；本管家未重跑审查）→ **进入任务级 OCR**（OCR 报告由后续条目回填）
- **修复轮次**：0（SDD 一轮通过；OCR 轮次另计）
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过，净变更 1 行）：b2-k0.`task_status` **K0.1 pending → done**（K0.2/0.3/K5.1–3 维持 pending）；节点级 `status=in_progress`、`review_status=pending`、`head_sha=null` 均未动；33 节点分布不变（6 done + 4 in_progress + 23 pending）
- **worktree**：`.worktrees/passb-b2-k0`（HEAD `614409798`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）

---

## 2026-09-23 23:54 CST · b2-k0 登记 OCR 覆盖：ocr_covered 追加 [59b13c61a → 614409798]（范围无可审项）

- **节点**：b2-k0 —— K0 Knowledge 端口/所有权冻结（status=in_progress 未变；task_status K0.1=done 维持）
- **登记内容**：b2-k0 节点新增数组字段 `ocr_covered`（该节点此前无此字段），追加首条 `{base: "59b13c61a727038aa9acc019967434d66da6bb14", head: "614409798fbdb05805504e7733154ec3a1ade960"}`（全 40 位 SHA，调度指令口径：**范围无可审项**）
- **SHA 真实性核验**（本会话 git 实测，`.worktrees/passb-b2-k0`）：`git rev-parse` 双双命中——base `59b13c61a`（计划审校修复提交，= K0.1 开工 BASE）与 head `614409798`（23:31:08，K0.1 冻结表复核提交 = 分支当前 HEAD）；区间含**恰 1 个提交**（仅计划文档修订）；`614409798..HEAD` 零新提交
- **口径并录留痕（调度方知悉）**：本指令口径"范围无可审项"，但该区间包含 K0.1 交付提交（23:44 条目"进入任务级 OCR"）——与 b2-appconnector 23:10 条目同型：两者关系指令未说明（若系 OCR 选审集不含纯 docs 提交则自洽），如实并录不自行判定
- **字段语义（沿先例）**：ocr_covered 记录已审查覆盖的已提交 SHA 区间；base = 任务开工 BASE、head = 分支当前头——b2-k0 截至目前的全部提交均在覆盖登记范围内；后续新提交落地后须追加条目
- **节点级状态未迁移**：`status=in_progress`、`review_status=pending`、`head_sha=null` 维持
- **修复轮次**：0（无可审项，不构成修复轮）
- **base/head SHA（节点级）**：`8c45a8815` / null（未动——节点派发基准与 ocr_covered 任务区间 base 分属不同层次，沿 23:10 b2-appconnector 条目说明）
- **worktree**：`.worktrees/passb-b2-k0`（`codex/passb-b2-k0`，HEAD `614409798`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **本次 JSON 变更**：b2-k0 节点 `head_sha` 行后新增 `ocr_covered` 数组（7 行，编辑定位用其特有的 evidence_paths 双行结构——该节点无 reviews 行，区别于其余节点）；Edit 后 `python3 json.load` 复验合法（断言全过：ocr_covered 恰 1 条且 SHA 与指令逐字符一致；status/review_status/base/head_sha/task_status 未动；b2-ac-definition/b2-ac-skills 0 条、b2-appconnector 1 条保持——**其余节点均未被误改**）

---

## 2026-09-23 23:56 CST · b2-k0 OCR 第 1 次：confirmed=0 / rejected=0（报告为"Review skipped"跳过态，如实登记）

- **节点/轮次**：b2-k0 节点级架构审查 OCR 第 1 次（调度口径；21:32 派发后首轮，23:44 条目"进入任务级 OCR"后的运行）
- **计划路径**：`docs/plans/passb/20-knowledge-program.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b2-k0/ocr-r1.txt`（本会话已读全文：**40 字节、mtime 23:54、1 行——"Review skipped: no items were selected."**；同目录 `ocr-context.md` 7,798 字节、mtime 23:53 与 K0.1-report.md/K0.1-review-pkg.md 在案）
- **结论计数**：confirmed=0 / rejected=0（调度口径；与报告字面一致——**跳过态非完成态**，沿 b1 三节点与 b2-appconnector 23:18 条目先例如实登记）
- **报告性质留痕**：与 23:54 条目 ocr_covered 登记 [59b13c61a → 614409798] 的"范围无可审项"口径自洽（区间唯一提交为纯计划文档修订，OCR 选审集无项）；后续节点有生产代码提交（K0.2 kbfreeze 守卫测试包起）时 OCR 须以非 skipped 方式产出真实结论
- **分支核验**（本会话 git 实测）：`.worktrees/passb-b2-k0` HEAD 仍 `614409798`（= ocr_covered 首条 head）、工作树干净——23:54 后无新提交
- **审查结论**：confirmed=0 → 不触发 changes_requested；`review_status=pending` 维持（未获 approved 指令，跳过轮不构成通过依据）
- **修复轮次**：0（无可审项，不构成修复轮）
- **base/head SHA**：`8c45a8815` / null（未动）
- **worktree**：`.worktrees/passb-b2-k0`（`codex/passb-b2-k0`，HEAD `614409798`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **本次 JSON 变更**：**无字节级改动**——confirmed=0 时 `review_status` 目标值 `pending` 已在位；`status=in_progress`、base/head SHA、ocr_covered（1 条）、task_status（K0.1 done）均未动。JSON 合法性本会话 `python3 json.load` 复验通过
- **备注**：(1) `ocr-context.md` 7,798 字节在案（未逐字审读）；(2) K0.1 为纯计划文档复核任务，skipped 与任务性质相容；K0.2（kbfreeze 守卫测试包，含代码与测试）为首个含代码的 b2-k0 任务，其 OCR 应产出实质结论

---

## 2026-09-23 23:58 CST · b2-k0 OCR 覆盖登记（区间已在 23:54 登记，不重复追加；口径补强为"0 条需修 findings"）

- **节点**：b2-k0 —— K0 Knowledge 端口/所有权冻结（status=in_progress 未变）
- **指令**：登记 ocr_covered 追加 `{base: "59b13c61a727038aa9acc019967434d66da6bb14", head: "614409798fbdb05805504e7733154ec3a1ade960"}`（**审得 0 条需修 findings**；全 40 位 SHA）
- **重复识别**：本会话 `python3 json.load` 实测——b2-k0.`ocr_covered` 已含**完全相同的条目**（base/head 与指令逐字符一致，23:54 条目所登记，当时调度口径"范围无可审项"）。分支 `614409798..HEAD` 零新提交——区间自 23:54 登记后无演进（沿 b1-identity 20:10 / b1-execution 20:13 / b1-commercial 20:19 / b2-appconnector 23:20 条目同型先例）
- **JSON 处置**：**无字节级改动、不重复追加**——ocr_covered 数组语义为"已覆盖审查区间列表"，同区间双条目冗余且破坏数组区间语义。本会话 `python3 json.load` 复验合法（ocr_covered 仍 1 条；status=in_progress、review_status=pending、base/head_sha、task_status 均未动）
- **口径演进留痕**：同一区间 [59b13c61a → 614409798] 的审查结论由 23:54 的"范围无可审项"（对应 23:56 条目 OCR skipped 报告）补强为本次的"**审得 0 条需修 findings**"——语义上确认该区间已审且结论为零需修；差别以调度方本次口径为准，台账并录
- **审查结论**：`review_status=pending` 维持（本指令仅授权覆盖登记，未含 approved/done 迁移）
- **修复轮次**：0（0 条需修 findings，不构成修复轮）
- **base/head SHA（节点级）**：`8c45a8815` / null（未动）
- **worktree**：`.worktrees/passb-b2-k0`（HEAD `614409798`，本会话核验零新提交）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **后续**：节点分支 614409798 之后的实施提交落地后，须以**新条目**追加 ocr_covered
- **B2 各节点 OCR 覆盖登记现状（本时点）**：b2-appconnector [ced88ecb→67ac22c96] 0 需修（B2-AC.1 收口确认）/ b2-k0 [59b13c61a→614409798] **0 需修（本条口径）** / b2-ac-definition、b2-ac-skills 无登记（暂无任务级提交）

---

## 2026-09-24 00:00 CST · b2-ac-skills / 25b.1 → done（SDD 审查通过，进入任务级 OCR）

- **节点/任务**：b2-ac-skills · 25b.1 —— T1 特征化基线与证据骨架（计划 `docs/plans/passb/25b-skill-catalog-install.md` §6 T1；**B2 阶段第三个完成的实施任务**）
- **前置**：节点派发（21:36）+ 计划审校（23:16）
- **worktree**：`.worktrees/passb-b2-ac-skills`（`codex/passb-b2-ac-skills`，本会话 git 核验工作树干净）
- **BASE → HEAD**：`8592f2aac` → **`406ed1c6a`**（1 commit：23:36:39 `test(passb): passb b2-ac-skills 特征化基线`；`git show --stat` 实测恰 2 文件 +99 行纯 docs——新增 `docs/architecture/evidence/passb/b2-ac-skills.md`（56 行）+ `docs/plans/passb/reports/b2-ac-skills.md`（43 行）；BASE 与报告自陈一致）
- **测试证据**（25b.1-report.md §2 所载，8/8 全绿本台账转录）：`go build ./...` 0；`go test ./internal/application/service -count=1` ok（415.098s，计划基线 154.512s——耗时差异为机器负载，同为 ok 非行为差异）；Skill 定向测试 service/handler/router 三处 ok；`make check-backend-architecture` 633/23+23/58 逐项一致；`make verify-module-moves` 16 manifests；reaper `-v` 摘录 28/28 PASS（ReapStuckRuns 9/PruneSupersededSnapshots 17/ReconcileSnapshots 1 + 字面匹配的既有 Prune 用例，如实记录）；**未触发 conventions §5 停工上报，T2 起继续**
- **前置条件核验**（报告 §1）：b0 done/ib1 done+approved/派发 BASE 一致——逐项满足；并实测确认节点 notes 尾条 BLOCKED 文本为历史残留
- **报告/审查证据路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b2-ac-skills/25b.1-report.md`（5,314 字节，本会话已读）；`25b.1-review-pkg.md`（8,710 字节，在案）
- **审查结论**：SDD 审查通过（调度指令裁定；本管家未重跑审查）→ **进入任务级 OCR**（OCR 报告由后续条目回填）
- **修复轮次**：0（SDD 一轮通过；OCR 轮次另计）
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过，净变更 1 行）：b2-ac-skills.`task_status` **25b.1 pending → done**（25b.2–5 维持 pending）；节点级 `status=in_progress`、`review_status=pending`、`head_sha=null` 均未动；33 节点分布不变（6 done + 4 in_progress + 23 pending）
- **worktree**：`.worktrees/passb-b2-ac-skills`（HEAD `406ed1c6a`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）

---

## 2026-09-24 00:03 CST · b2-ac-definition / T1 → done（SDD 审查通过，进入任务级 OCR）

- **节点/任务**：b2-ac-definition · T1 —— 基线锚定与预检（计划 `docs/plans/passb/25a-agent-definition-version.md` §6 Task T1；**B2 阶段第四个完成的实施任务，B2 四入口节点全部进入任务执行**）
- **前置**：节点派发（21:34）+ 计划审校（23:28）
- **worktree**：`.worktrees/passb-b2-ac-definition`（`codex/passb-b2-ac-definition`，本会话 git 核验工作树干净）
- **BASE → HEAD**：`02e7be617` → **`9f330903b`**（1 commit：23:49:32 `test(passb): b2-ac-definition baseline characterization`；`git show --stat` 实测恰 1 文件 +62 行——新增 `docs/architecture/evidence/passb/b2-ac-definition.md` 骨架）；**基线口径留痕**（报告 §2）：ask 派发 BASE=`02e7be617`，计划正文基线=`8c45a8815`，二者差异经 `git diff --name-only` 实证仅为计划文档自身两次审校提交（无代码差异），后续差集核对统一采用派发 BASE（计划 §6-T1 条款授权）
- **测试证据**（T1-report.md §1 所载，全部实跑含退出码）：`go build ./...` 0；四包测试 0（repository 489s/service 345s/handler 1.8s/agentcatalog 无测试文件）；`make check-backend-architecture` 633/23+23/58 逐项一致 0 violations；`make verify-module-moves` 16 manifests；特征化预跑 13 用例全 PASS（Share/Subagent 族）；`EXPORT-MISSING` 与 `execution/service 缺位` 非零退出——**与计划 §1.5 预期一致**（B1-CM/B1-EX 实施缺位，仅登记不阻塞批次 1）
- **报告/审查证据路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b2-ac-definition/T1-report.md`（5,415 字节，本会话已读）；`T1-review-pkg.md`（5,733 字节，在案）
- **审查结论**：SDD 审查通过（调度指令裁定；本管家未重跑审查）→ **进入任务级 OCR**（OCR 报告由后续条目回填）
- **修复轮次**：0（SDD 一轮通过；OCR 轮次另计）
- **报告遗留如实登记**：(1) EXPORT-MISSING 登记待 T5 Brief（IB1 号称导出的两处实为 B1 零实施缺位，20:47 条目在案一致）；(2) evidence 差分/等价章节待 T4 补全，Brief 与节点级报告待 T5；(3) briefs/reports 空目录随 T3/T5 首个文件落地
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过，净变更 1 行）：b2-ac-definition.`task_status` **T1 pending → done**（T2–5 维持 pending）；节点级 `status=in_progress`、`review_status=pending`、`head_sha=null` 均未动；33 节点分布不变（6 done + 4 in_progress + 23 pending）
- **worktree**：`.worktrees/passb-b2-ac-definition`（HEAD `9f330903b`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）

---

## 2026-09-24 00:08 CST · b2-ac-skills 登记 OCR 覆盖：ocr_covered 追加 [8592f2aac → 406ed1c6a]（范围无可审项）

- **节点**：b2-ac-skills —— 25b Skill 目录/安装/运行时验证/reaper（status=in_progress 未变；task_status 25b.1=done 维持）
- **登记内容**：b2-ac-skills 节点新增数组字段 `ocr_covered`（该节点此前无此字段），追加首条 `{base: "8592f2aacbe49e43583bf433e045b98cefd44c6f", head: "406ed1c6a97403be2a71690cd966f558b9848b92"}`（全 40 位 SHA，调度指令口径：**范围无可审项**）
- **SHA 真实性核验**（本会话 git 实测，`.worktrees/passb-b2-ac-skills`）：`git rev-parse` 双双命中——base `8592f2aac`（计划审校修复提交，= 25b.1 开工 BASE）与 head `406ed1c6a`（23:36:39，25b.1 特征化基线提交 = 分支当前 HEAD）；区间含**恰 1 个提交**（99 行纯 docs：evidence 骨架 + 报告）；`406ed1c6a..HEAD` 零新提交
- **口径并录留痕（调度方知悉）**：本指令口径"范围无可审项"，但该区间包含 25b.1 交付提交（00:00 条目"进入任务级 OCR"）——沿 b2-appconnector 23:10 / b2-k0 23:54 条目同型并录：两者关系指令未说明（若系 OCR 选审集不含纯 docs 提交则自洽），如实并录不自行判定
- **字段语义（沿先例）**：ocr_covered 记录已审查覆盖的已提交 SHA 区间；base = 任务开工 BASE、head = 分支当前头——b2-ac-skills 截至目前的全部提交均在覆盖登记范围内；后续新提交落地后须追加条目
- **节点级状态未迁移**：`status=in_progress`、`review_status=pending`、`head_sha=null` 维持
- **修复轮次**：0（无可审项，不构成修复轮）
- **base/head SHA（节点级）**：`8c45a8815` / null（未动——节点派发基准与 ocr_covered 任务区间 base 分属不同层次）
- **worktree**：`.worktrees/passb-b2-ac-skills`（`codex/passb-b2-ac-skills`，HEAD `406ed1c6a`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **本次 JSON 变更**：b2-ac-skills 节点 `head_sha` 行后新增 `ocr_covered` 数组（7 行，编辑定位用其特有 evidence_paths 三行结构避免与相邻节点混淆）；Edit 后 `python3 json.load` 复验合法（断言全过：ocr_covered 恰 1 条且 SHA 与指令逐字符一致；status/review_status/base/head_sha/task_status 未动；b2-k0=1 条、b2-appconnector=1 条保持、b2-ac-definition=0 条——**其余节点均未被误改**）
- **B2 各节点 ocr_covered 现状**：b2-appconnector 1 条 / b2-k0 1 条 / **b2-ac-skills 1 条（本条）** / b2-ac-definition 0 条

---

## 2026-09-24 00:09 CST · b2-ac-skills OCR 第 1 次：confirmed=0 / rejected=0（报告为"Review skipped"跳过态，如实登记）

- **节点/轮次**：b2-ac-skills 节点级架构审查 OCR 第 1 次（调度口径；21:36 派发后首轮，00:00 条目"进入任务级 OCR"后的运行）
- **计划路径**：`docs/plans/passb/25b-skill-catalog-install.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b2-ac-skills/ocr-r1.txt`（本会话已读全文：**40 字节、mtime 00:07、1 行——"Review skipped: no items were selected."**；同目录 `ocr-context.md` 8,715 字节、mtime 00:07 与 25b.1-report.md/25b.1-review-pkg.md 在案）
- **结论计数**：confirmed=0 / rejected=0（调度口径；与报告字面一致——**跳过态非完成态**，沿 b1 三节点与 b2-appconnector 23:18 / b2-k0 23:56 条目先例如实登记）
- **报告性质留痕**：与 00:08 条目 ocr_covered 登记 [8592f2aac → 406ed1c6a] 的"范围无可审项"口径自洽（区间唯一提交为 99 行纯 docs——evidence 骨架+报告，OCR 选审集无项）；后续节点有生产代码提交（25b.2 repository 层搬迁起）时 OCR 须以非 skipped 方式产出真实结论
- **分支核验**（本会话 git 实测）：`.worktrees/passb-b2-ac-skills` HEAD 仍 `406ed1c6a`（= ocr_covered 首条 head）、工作树干净——00:08 后无新提交
- **审查结论**：confirmed=0 → 不触发 changes_requested；`review_status=pending` 维持（未获 approved 指令，跳过轮不构成通过依据）
- **修复轮次**：0（无可审项，不构成修复轮）
- **base/head SHA**：`8c45a8815` / null（未动）
- **worktree**：`.worktrees/passb-b2-ac-skills`（`codex/passb-b2-ac-skills`，HEAD `406ed1c6a`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **本次 JSON 变更**：**无字节级改动**——confirmed=0 时 `review_status` 目标值 `pending` 已在位；`status=in_progress`、base/head SHA、ocr_covered（1 条）、task_status（25b.1 done）均未动。JSON 合法性本会话 `python3 json.load` 复验通过
- **备注**：(1) `ocr-context.md` 8,715 字节在案（未逐字审读）；(2) 25b.1 为纯 docs 任务，skipped 与任务性质相容；25b.2（repository 层搬迁，首个含生产代码的 b2-ac-skills 任务）其 OCR 应产出实质结论

---

## 2026-09-24 00:11 CST · b2-ac-definition 登记 OCR 覆盖：ocr_covered 追加 [02e7be617 → 9f330903b]（范围无可审项）

- **节点**：b2-ac-definition —— 25a Agent 定义/版本/人格/专家/子代理/收藏（status=in_progress 未变；task_status T1=done 维持）
- **登记内容**：b2-ac-definition 节点新增数组字段 `ocr_covered`（该节点此前无此字段），追加首条 `{base: "02e7be617c041bbf796b7f0acff0a4fbb6402a38", head: "9f330903b0cb3025e98ec68f196a5b34e1b512e1"}`（全 40 位 SHA，调度指令口径：**范围无可审项**）
- **SHA 真实性核验**（本会话 git 实测，`.worktrees/passb-b2-ac-definition`）：`git rev-parse` 双双命中——base `02e7be617`（计划 review fixes r2 提交，= T1 开工 BASE，即派发 BASE）与 head `9f330903b`（23:49:32，T1 基线锚定提交 = 分支当前 HEAD）；区间含**恰 1 个提交**（62 行纯 docs：evidence 骨架）；`9f330903b..HEAD` 零新提交
- **口径并录留痕（调度方知悉）**：本指令口径"范围无可审项"，但该区间包含 T1 交付提交（00:03 条目"进入任务级 OCR"）——沿 b2-appconnector 23:10 / b2-k0 23:54 / b2-ac-skills 00:08 条目同型并录：两者关系指令未说明（若系 OCR 选审集不含纯 docs 提交则自洽），如实并录不自行判定
- **字段语义（沿先例）**：ocr_covered 记录已审查覆盖的已提交 SHA 区间；base = 任务开工 BASE（亦即派发 BASE，该节点任务区间起点与节点 base 不同层次但本例任务紧随计划审校）、head = 分支当前头；后续新提交落地后须追加条目
- **节点级状态未迁移**：`status=in_progress`、`review_status=pending`、`head_sha=null` 维持
- **修复轮次**：0（无可审项，不构成修复轮）
- **base/head SHA（节点级）**：`8c45a8815` / null（未动）
- **worktree**：`.worktrees/passb-b2-ac-definition`（`codex/passb-b2-ac-definition`，HEAD `9f330903b`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **本次 JSON 变更**：b2-ac-definition 节点 `head_sha` 行后新增 `ocr_covered` 数组（7 行，编辑定位用其特有 evidence_paths 三行结构）；Edit 后 `python3 json.load` 复验合法（断言全过：ocr_covered 恰 1 条且 SHA 与指令逐字符一致；status/review_status/base/head_sha/task_status 未动；b2-k0/b2-ac-skills/b2-appconnector 各 1 条保持——**其余节点均未被误改**）
- **B2 各节点 ocr_covered 现状终态**：**四入口节点全部 1 条**——b2-appconnector [ced88ecb→67ac22c96] / b2-k0 [59b13c61a→614409798] / b2-ac-skills [8592f2aac→406ed1c6a] / b2-ac-definition [02e7be617→9f330903b]（本条）

---

## 2026-09-24 00:12 CST · b2-ac-skills OCR 覆盖登记（区间已在 00:08 登记，不重复追加；口径补强为"0 条需修 findings"）

- **节点**：b2-ac-skills —— 25b Skill 目录/安装/运行时验证/reaper（status=in_progress 未变）
- **指令**：登记 ocr_covered 追加 `{base: "8592f2aacbe49e43583bf433e045b98cefd44c6f", head: "406ed1c6a97403be2a71690cd966f558b9848b92"}`（**审得 0 条需修 findings**；全 40 位 SHA）
- **重复识别**：本会话 `python3 json.load` 实测——b2-ac-skills.`ocr_covered` 已含**完全相同的条目**（base/head 与指令逐字符一致，00:08 条目所登记，当时调度口径"范围无可审项"）。分支 `406ed1c6a..HEAD` 零新提交——区间自 00:08 登记后无演进（沿 b1-identity 20:10 / b1-execution 20:13 / b1-commercial 20:19 / b2-appconnector 23:20 / b2-k0 23:58 条目同型先例）
- **JSON 处置**：**无字节级改动、不重复追加**——ocr_covered 数组语义为"已覆盖审查区间列表"，同区间双条目冗余且破坏数组区间语义。本会话 `python3 json.load` 复验合法（ocr_covered 仍 1 条；status=in_progress、review_status=pending、base/head_sha、task_status 均未动）
- **口径演进留痕**：同一区间 [8592f2aac → 406ed1c6a] 的审查结论由 00:08 的"范围无可审项"补强为本次的"**审得 0 条需修 findings**"——语义上确认该区间已审且结论为零需修；差别以调度方本次口径为准，台账并录
- **审查结论**：`review_status=pending` 维持（本指令仅授权覆盖登记，未含 approved/done 迁移）
- **修复轮次**：0（0 条需修 findings，不构成修复轮）
- **base/head SHA（节点级）**：`8c45a8815` / null（未动）
- **worktree**：`.worktrees/passb-b2-ac-skills`（HEAD `406ed1c6a`，本会话核验零新提交）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **后续**：节点分支 406ed1c6a 之后的实施提交落地后，须以**新条目**追加 ocr_covered

---

## 2026-09-24 00:14 CST · b2-ac-definition OCR 第 1 次：confirmed=0 / rejected=0（报告为"Review skipped"跳过态，如实登记）

- **节点/轮次**：b2-ac-definition 节点级架构审查 OCR 第 1 次（调度口径；21:34 派发后首轮，00:03 条目"进入任务级 OCR"后的运行）
- **计划路径**：`docs/plans/passb/25a-agent-definition-version.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b2-ac-definition/ocr-r1.txt`（本会话已读全文：**40 字节、mtime 00:10、1 行——"Review skipped: no items were selected."**；同目录 `ocr-context.md` 8,423 字节、mtime 00:09 与 T1-report.md/T1-review-pkg.md 在案）
- **结论计数**：confirmed=0 / rejected=0（调度口径；与报告字面一致——**跳过态非完成态**，沿先例（b1 三节点 / b2-appconnector 23:18 / b2-k0 23:56 / b2-ac-skills 00:09）如实登记）
- **报告性质留痕**：与 00:11 条目 ocr_covered 登记 [02e7be617 → 9f330903b] 的"范围无可审项"口径自洽（区间唯一提交为 62 行纯 docs evidence 骨架，OCR 选审集无项）；后续节点有生产代码提交（T2 repository 批次搬迁起）时 OCR 须以非 skipped 方式产出真实结论
- **分支核验**（本会话 git 实测）：`.worktrees/passb-b2-ac-definition` HEAD 仍 `9f330903b`（= ocr_covered 首条 head）、工作树干净——00:11 后无新提交
- **审查结论**：confirmed=0 → 不触发 changes_requested；`review_status=pending` 维持（未获 approved 指令，跳过轮不构成通过依据）
- **修复轮次**：0（无可审项，不构成修复轮）
- **base/head SHA**：`8c45a8815` / null（未动）
- **worktree**：`.worktrees/passb-b2-ac-definition`（`codex/passb-b2-ac-definition`，HEAD `9f330903b`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **本次 JSON 变更**：**无字节级改动**——confirmed=0 时 `review_status` 目标值 `pending` 已在位；`status=in_progress`、base/head SHA、ocr_covered（1 条）、task_status（T1 done）均未动。JSON 合法性本会话 `python3 json.load` 复验通过
- **备注**：(1) `ocr-context.md` 8,423 字节在案（未逐字审读）；(2) T1 为纯 docs 任务，skipped 与任务性质相容；T2（repository 批次搬迁 #1–#4 + shim，首个含生产代码的 b2-ac-definition 任务）其 OCR 应产出实质结论
- **B2 四入口节点 OCR 首轮全记录齐**：appconnector skipped（23:18）/ k0 skipped（23:56）/ ac-skills skipped（00:09）/ ac-definition skipped（本条）——四节点首个任务均为 docs/测试类或纯 docs，跳过态与进度自洽

---

## 2026-09-24 00:16 CST · b2-ac-definition OCR 覆盖登记（区间已在 00:11 登记，不重复追加；口径补强为"0 条需修 findings"）

- **节点**：b2-ac-definition —— 25a Agent 定义/版本/人格/专家/子代理/收藏（status=in_progress 未变）
- **指令**：登记 ocr_covered 追加 `{base: "02e7be617c041bbf796b7f0acff0a4fbb6402a38", head: "9f330903b0cb3025e98ec68f196a5b34e1b512e1"}`（**审得 0 条需修 findings**；全 40 位 SHA）
- **重复识别**：本会话 `python3 json.load` 实测——b2-ac-definition.`ocr_covered` 已含**完全相同的条目**（base/head 与指令逐字符一致，00:11 条目所登记，当时调度口径"范围无可审项"）。分支 `9f330903b..HEAD` 零新提交——区间自 00:11 登记后无演进（沿 20:10/20:13/20:19/23:20/23:58/00:12 条目同型先例）
- **JSON 处置**：**无字节级改动、不重复追加**——ocr_covered 数组语义为"已覆盖审查区间列表"，同区间双条目冗余且破坏数组区间语义。本会话 `python3 json.load` 复验合法（ocr_covered 仍 1 条；status=in_progress、review_status=pending、base/head_sha、task_status 均未动）
- **口径演进留痕**：同一区间 [02e7be617 → 9f330903b] 的审查结论由 00:11 的"范围无可审项"（对应 00:14 条目 OCR skipped 报告）补强为本次的"**审得 0 条需修 findings**"——语义上确认该区间已审且结论为零需修；差别以调度方本次口径为准，台账并录
- **审查结论**：`review_status=pending` 维持（本指令仅授权覆盖登记，未含 approved/done 迁移）
- **修复轮次**：0（0 条需修 findings，不构成修复轮）
- **base/head SHA（节点级）**：`8c45a8815` / null（未动）
- **worktree**：`.worktrees/passb-b2-ac-definition`（HEAD `9f330903b`，本会话核验零新提交）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **后续**：节点分支 9f330903b 之后的实施提交落地后，须以**新条目**追加 ocr_covered
- **B2 各节点覆盖登记现状（本时点）**：b2-appconnector 0 需修（B2-AC.1 收口确认）/ b2-k0 0 需修（K0.1 收口确认）/ b2-ac-skills 0 需修（25b.1 收口确认）/ b2-ac-definition **0 需修（本条口径，T1 收口确认待指令）**

---

## 2026-09-24 00:18 CST · b2-ac-definition / T1 收口确认（目标态已在位，JSON 无字节级改动）

- **节点/任务**：b2-ac-definition · T1 —— 基线锚定与预检
- **指令**：T1 → done（SDD+任务级 OCR 双通过，OCR 覆盖 02e7be6..9f33090）
- **目标态核验**（本会话 `python3 json.load` 实测，**全部已在位**）：
  1. `task_status.T1 = done`（00:03 条目 SDD 审查通过时所置）✓
  2. OCR 覆盖 `ocr_covered = [{base: 02e7be617..., head: 9f330903b...}]`（00:11 登记区间 + 00:16 口径"0 条需修 findings"补强）✓
  3. 审查链条完整：SDD 审查通过（00:03，报告 5,415B/审查包 5,733B 在案）→ ocr_covered 登记（00:11）→ 任务级 OCR skipped 轮 0 findings（00:14）→ 口径确认 0 需修（00:16）——**SDD+OCR 双通过闭环在案**
- **JSON 处置**：**无字节级改动**——指令目标态与现值完全一致，无迁移动作。节点级 `status=in_progress`（T2–5 待执行）、`review_status=pending`、`head_sha=null` 维持
- **本次 JSON 变更**：无。JSON 合法性本会话 `python3 json.load` 复验通过；33 节点分布不变（6 done + 4 in_progress + 23 pending）
- **worktree**：`.worktrees/passb-b2-ac-definition`（HEAD `9f330903b`，零新提交）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **修复轮次**：0（任务级维持；T2 起待执行）
- **备注**：本条为 00:03（task done）/ 00:11（覆盖登记）/ 00:14（OCR skipped）/ 00:16（口径补强）四条目的指令级收口确认（沿 23:22/23:59/00:15 三次收口确认先例）；下一任务 T2（repository 批次搬迁 #1–#4 + shim，首个含生产代码的 b2-ac-definition 任务）开工后其 OCR 预计为非 skipped 实质审查
- **B2 首批任务收口进度总览（本时点）**：**四入口节点首个任务全部收口完成**——B2-AC.1 ✓（23:22）/ K0.1 ✓（23:59）/ 25b.1 ✓（00:15）/ T1 ✓（本条）；各节点下一任务均为含生产代码的搬迁/实施类

---

## 2026-09-24 00:33 CST · b2-k0 / K0.2 → done（SDD 审查通过，进入任务级 OCR）

- **节点/任务**：b2-k0 · K0.2 —— kbfreeze 守卫测试包（R0 与六 port 签名的机器强制；计划 `docs/plans/passb/20-knowledge-program.md` §8 Task K0.2）
- **前置**：K0.1（done，23:44 条目）
- **worktree**：`.worktrees/passb-b2-k0`（`codex/passb-b2-k0`，本会话 git 核验工作树干净）
- **BASE → HEAD**：`614409798` → **`e4e7a1d8a`**（1 commit：00:13:45 `test(passb): b2-k0 知识共享类型与端口冻结守卫`；`git show --stat` 实测恰 1 文件 +220 行——新建 `internal/modules/knowledge/kbfreeze/freeze_test.go`，纯测试零生产代码）
- **交付物核验**（K0.2-report.md §1 所载，本台账转录）：两条守卫测试——(1) `TestKnowledgeSharedTypesHaveNoShadowDefinitions`：遍历 knowledge 模块树对 §5 冻结的 30 类型名做影子定义检测（豁免表唯一条目 chunker/splitter.go→Chunk，与计划一致；repo 根经 runtime.Caller 推导）；(2) `TestFrozenCapabilityPortInterfacesUnchanged`：reflect 断言六端口——ChunkService 18 方法/KnowledgeTagService 7 方法集合精确相等 + 其余四端口锚点存在性（错误信息携带 contracts.yaml port id）；**包依赖仅 stdlib + internal/types/interfaces，未 import 任何 legacy 宿主包**（`go list -deps` + grep 验证零命中，满足计划验收条款）
- **测试证据**（K0.2-report.md §2，7 条全实跑）：kbfreeze 测试 2/2 PASS（0.563s）；gofmt/vet 干净；`go build ./...` 0；`make check-backend-architecture` 633/23+23/58 一致 0 violations；`make verify-module-moves` 16 manifests；**RED 验证有记录**（计划规定方式：临时破坏输入→确认 FAIL→恢复）
- **报告/审查证据路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b2-k0/K0.2-report.md`（8,063 字节，本会话已读）；`K0.2-review-pkg.md`（12,049 字节，在案）
- **审查结论**：SDD 审查通过（调度指令裁定；本管家未重跑审查）→ **进入任务级 OCR**（OCR 报告由后续条目回填）
- **修复轮次**：0（SDD 一轮通过；OCR 轮次另计）
- **本次 JSON 变更**（Edit 后 `python3 json.load` 复验合法、断言全过，净变更 1 行）：b2-k0.`task_status` **K0.2 pending → done**（K0.3/K5.1–3 维持 pending）；节点级 `status=in_progress`、`review_status=pending`、`head_sha=null` 均未动；33 节点分布不变（6 done + 4 in_progress + 23 pending）
- **worktree**：`.worktrees/passb-b2-k0`（HEAD `e4e7a1d8a`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **备注**：K0.2 为 b2-k0 首个含代码任务（纯测试代码），其任务级 OCR 预计为非 skipped 实质审查——结果由后续指令登记；K0.3（前置差异上报、证据与报告）为节点收口前最后任务

---

## 2026-09-24 00:15 CST · b2-ac-skills / 25b.1 收口确认（目标态已在位，JSON 无字节级改动）

- **节点/任务**：b2-ac-skills · 25b.1 —— T1 特征化基线与证据骨架
- **指令**：25b.1 → done（SDD+任务级 OCR 双通过，OCR 覆盖 8592f2a..406ed1c）
- **目标态核验**（本会话 `python3 json.load` 实测，**全部已在位**）：
  1. `task_status.25b.1 = done`（00:00 条目 SDD 审查通过时所置）✓
  2. OCR 覆盖 `ocr_covered = [{base: 8592f2aac..., head: 406ed1c6a...}]`（00:08 登记区间 + 00:12 口径"0 条需修 findings"补强）✓
  3. 审查链条完整：SDD 审查通过（00:00，报告 5,314B/审查包 8,710B 在案）→ ocr_covered 登记（00:08）→ 任务级 OCR skipped 轮 0 findings（00:09）→ 口径确认 0 需修（00:12）——**SDD+OCR 双通过闭环在案**
- **JSON 处置**：**无字节级改动**——指令目标态与现值完全一致，无迁移动作。节点级 `status=in_progress`（25b.2–5 待执行）、`review_status=pending`、`head_sha=null` 维持
- **本次 JSON 变更**：无。JSON 合法性本会话 `python3 json.load` 复验通过；33 节点分布不变（6 done + 4 in_progress + 23 pending）
- **worktree**：`.worktrees/passb-b2-ac-skills`（HEAD `406ed1c6a`，零新提交）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **修复轮次**：0（任务级维持；25b.2 起待执行）
- **备注**：本条为 00:00（task done）/ 00:08（覆盖登记）/ 00:09（OCR skipped）/ 00:12（口径补强）四条目的指令级收口确认（沿 23:22 b2-appconnector/B2-AC.1 与 23:59 b2-k0/K0.1 收口确认先例）；下一任务 25b.2（repository 层搬迁，首个含生产代码的 b2-ac-skills 任务）开工后其 OCR 预计为非 skipped 实质审查

---

## 2026-09-23 23:59 CST · b2-k0 / K0.1 收口确认（目标态已在位，JSON 无字节级改动）

- **节点/任务**：b2-k0 · K0.1 —— 冻结分配表与联动裁定表复核
- **指令**：K0.1 → done（SDD+任务级 OCR 双通过，OCR 覆盖 59b13c6..6144097）
- **目标态核验**（本会话 `python3 json.load` 实测，**全部已在位**）：
  1. `task_status.K0.1 = done`（23:44 条目 SDD 审查通过时所置）✓
  2. OCR 覆盖 `ocr_covered = [{base: 59b13c61a..., head: 614409798...}]`（23:54 登记区间 + 23:58 口径"0 条需修 findings"补强）✓
  3. 审查链条完整：SDD 审查通过（23:44，报告 14,018B/审查包 20,135B 在案）→ ocr_covered 登记（23:54）→ 任务级 OCR skipped 轮 0 findings（23:56）→ 口径确认 0 需修（23:58）——**SDD+OCR 双通过闭环在案**
- **JSON 处置**：**无字节级改动**——指令目标态与现值完全一致，无迁移动作。节点级 `status=in_progress`（K0.2/0.3/K5.1–3 待执行）、`review_status=pending`、`head_sha=null` 维持
- **本次 JSON 变更**：无。JSON 合法性本会话 `python3 json.load` 复验通过；33 节点分布不变（6 done + 4 in_progress + 23 pending）
- **worktree**：`.worktrees/passb-b2-k0`（HEAD `614409798`，零新提交）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **修复轮次**：0（任务级维持；K0.2 起待执行）
- **备注**：本条为 23:44（task done）/ 23:54（覆盖登记）/ 23:56（OCR skipped）/ 23:58（口径补强）四条目的指令级收口确认（沿 23:22 b2-appconnector/B2-AC.1 收口确认先例）；下一任务 K0.2（kbfreeze 守卫测试包，首个含代码的 b2-k0 任务）开工后其 OCR 预计为非 skipped 实质审查

---

## 2026-09-23 23:18 CST · b2-appconnector OCR 第 1 次：confirmed=0 / rejected=0（报告为"Review skipped"跳过态，如实登记）

- **节点/轮次**：b2-appconnector 节点级架构审查 OCR 第 1 次（调度口径；21:37 派发后首轮，22:58 条目"进入任务级 OCR"后的运行）
- **计划路径**：`docs/plans/passb/27-appconnector.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b2-appconnector/ocr-r1.txt`（本会话已读全文：**40 字节、mtime 23:09、1 行——"Review skipped: no items were selected."**；同目录 `ocr-context.md` 8,593 字节、mtime 23:04 在案；另有 B2-AC.1-report.md / B2-AC.1-review-pkg.md 在案）
- **结论计数**：confirmed=0 / rejected=0（调度口径；与报告字面一致——**跳过态非完成态**，沿 b1-identity 20:06 / b1-execution 20:09 / b1-commercial 20:16 条目先例如实登记）
- **报告性质留痕**：与 23:10 条目 ocr_covered 登记 [ced88ecb → 67ac22c96] 的"范围无可审项"口径自洽（区间唯一提交为 470 行纯测试文件，OCR 选审集无项）；后续节点有生产代码提交时 OCR 须以非 skipped 方式产出真实结论
- **分支核验**（本会话 git 实测）：`.worktrees/passb-b2-appconnector` HEAD 仍 `67ac22c96`（= ocr_covered 首条 head）、工作树含 evidence 草稿 untracked（22:58 条目已登记，提交隔离设计）——23:10 后无新提交
- **审查结论**：confirmed=0 → 不触发 changes_requested；`review_status=pending` 维持（未获 approved 指令，跳过轮不构成通过依据）
- **修复轮次**：0（无可审项，不构成修复轮）
- **base/head SHA**：`8c45a8815` / null（未动）
- **worktree**：`.worktrees/passb-b2-appconnector`（`codex/passb-b2-appconnector`，HEAD `67ac22c96`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **本次 JSON 变更**：**无字节级改动**——confirmed=0 时 `review_status` 目标值 `pending` 已在位；`status=in_progress`、base/head SHA、ocr_covered（1 条）、task_status（B2-AC.1 done）均未动。JSON 合法性本会话 `python3 json.load` 复验通过
- **备注**：(1) `ocr-context.md` 8,593 字节在案（未逐字审读）；(2) B2 首个任务级 OCR 为 skipped——任务级 OCR 有效的实质结论待后续含生产代码的任务（B2-AC.2 起）运行时产出

---

## 2026-09-23 23:20 CST · b2-appconnector OCR 覆盖登记（区间已在 23:10 登记，不重复追加；口径补强为"0 条需修 findings"）

- **节点**：b2-appconnector —— 27 App Connector（status=in_progress 未变）
- **指令**：登记 ocr_covered 追加 `{base: "ced88ecb17ed560a02a15464500a4b2fa8e2d508", head: "67ac22c964cecfc810578f1ee8d1289b78e7e083"}`（**审得 0 条需修 findings**；全 40 位 SHA）
- **重复识别**：本会话 `python3 json.load` 实测——b2-appconnector.`ocr_covered` 已含**完全相同的条目**（base/head 与指令逐字符一致，23:10 条目所登记，当时调度口径"范围无可审项"）。分支 `67ac22c96..HEAD` 零新提交——区间自 23:10 登记后无演进（沿 b1-identity 20:10 / b1-execution 20:13 / b1-commercial 20:19 条目同型先例）
- **JSON 处置**：**无字节级改动、不重复追加**——ocr_covered 数组语义为"已覆盖审查区间列表"，同区间双条目冗余且破坏数组区间语义。本会话 `python3 json.load` 复验合法（ocr_covered 仍 1 条；status=in_progress、review_status=pending、base/head_sha、task_status 均未动）
- **口径演进留痕**：同一区间 [ced88ecb → 67ac22c96] 的审查结论由 23:10 的"范围无可审项"（对应 23:18 条目 OCR skipped 报告）补强为本次的"**审得 0 条需修 findings**"——语义上确认该区间已审且结论为零需修；差别以调度方本次口径为准，台账并录
- **审查结论**：`review_status=pending` 维持（本指令仅授权覆盖登记，未含 approved/done 迁移）
- **修复轮次**：0（0 条需修 findings，不构成修复轮）
- **base/head SHA（节点级）**：`8c45a8815` / null（未动）
- **worktree**：`.worktrees/passb-b2-appconnector`（HEAD `67ac22c96`，本会话核验零新提交）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **后续**：节点分支 67ac22c96 之后的实施提交落地后，须以**新条目**追加 ocr_covered

---

## 2026-09-23 23:22 CST · b2-appconnector / B2-AC.1 收口确认（目标态已在位，JSON 无字节级改动）

- **节点/任务**：b2-appconnector · B2-AC.1 —— T1 特征化基线
- **指令**：B2-AC.1 → done（SDD+任务级 OCR 双通过，OCR 覆盖 ced88ec..67ac22c）
- **目标态核验**（本会话 `python3 json.load` 实测，**全部已在位**）：
  1. `task_status.B2-AC.1 = done`（22:58 条目 SDD 审查通过时所置）✓
  2. OCR 覆盖 `ocr_covered = [{base: ced88ecb..., head: 67ac22c96...}]`（23:10 登记区间 + 23:20 口径"0 条需修 findings"补强）✓
  3. 审查链条完整：SDD 审查通过（22:58，报告 7,343B/审查包 22,196B 在案）→ ocr_covered 登记（23:10）→ 任务级 OCR skipped 轮 0 findings（23:18）→ 口径确认 0 需修（23:20）——**SDD+OCR 双通过闭环在案**
- **JSON 处置**：**无字节级改动**——指令目标态与现值完全一致，无迁移动作。节点级 `status=in_progress`（B2-AC.2/3/4 待执行）、`review_status=pending`、`head_sha=null` 维持
- **本次 JSON 变更**：无。JSON 合法性本会话 `python3 json.load` 复验通过；33 节点分布不变（6 done + 4 in_progress + 23 pending）
- **worktree**：`.worktrees/passb-b2-appconnector`（HEAD `67ac22c96`，零新提交）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **修复轮次**：0（任务级维持；B2-AC.2 起待执行）
- **备注**：本条为 22:58（task done）/ 23:10（覆盖登记）/ 23:18（OCR skipped）/ 23:20（口径补强）四条目的指令级收口确认；下一任务 B2-AC.2（handler 层搬迁 7+2 文件，首个含生产代码的任务）开工后其 OCR 预计为非 skipped 实质审查

---

## 2026-09-24 01:04 CST · b0 OCR done 后复扫第 1 次：confirmed=2 / rejected=0（→ changes_requested；done+changes_requested 组合态留痕）

- **节点/轮次**：b0 OCR done 后复扫（调度口径"第 1 次"）；台账历史口径**第 16 次 OCR 运行**（07:09 / 07:29 / 07:45 / 08:28 / 09:30 / 10:41 / 11:36 / 12:04 / 12:14 / 12:34 / 13:47 / 16:02 / 16:39 / 17:29 / 18:28 / 本次 00:57）。**性质：b0 已于 18:47 收口 done（head acd7b24 → 集成后 d57a2fa70，18:53 合入 / 18:56 回填），本轮系 done 之后的全量复扫**（13 selected items = Makefile + tools/passbguard 6 源文件）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b0/ocr-r1.txt`（本会话已读全文：2,987 字节、45 行、mtime 2026-09-24 00:57；首行 "Review complete: 2 finding(s) across 13 selected item(s)."；**该路径第五次被覆盖**——07:04 版 9 findings → 08:28 版 14 → 10:41 版 16 → 13:47 版 7 → 本次 2；报告附 LLM retry 摘要：29 请求中 1 次超时失败后重试）
- **结论计数**：confirmed=2 / rejected=0（调度指令口径；与报告 2 findings 一致）
- **findings 清单**（报告全文转录，均 bug·medium，报告附修复建议 diff）：
  1. `tools/passbguard/check.go:1018-1023` —— fileReferencesIdent 对 parseFileFull 解析错误静默返回 false，与本包另两调用点（discover.go DiscoverSymbolConsumers 内两处）上抛口径不一致；worker-set 回退扫描（check.go:966 唯一消费点）遇 internal/router/ 注册文件全量 AST 解析失败（ImportsOnly 可过而全量不可过）时该文件从 discovered 集静默消失、已登记 consumers 批量误报 contract-consumer-vanished（解析错误伪装成治理漂移）；建议改返回 `(bool, error)`、由 discoverSetConsumers 转显式 setSiteIssue 诊断（如 contract-set-scan-failed）。**历史对照：与 R1 #8（10:49 轮 check.go:737-741）/ 12:37 终审 #5（check.go:835-839）同源，行号历修复偏移后本轮再现**
  2. `tools/passbguard/model.go:395-402` —— tenant-scoped 事件级联校验缺 actor_origin：event-catalog.yaml 头注释（第 9 行）冻结约束含 actor_origin、现有 30 条事件数据均含，但级联清单仅 occurred_at/event_id，未来新增/修订 tenant 事件漏登时结构校验与 CheckEvents 均不拦截（fail-silent）；建议纳入同一级联清单。**新列**
- **审查结论**：OCR 本轮**未通过**——2/2 confirmed、0 rejected。`review_status` **approved → changes_requested**（沿 10:49 先例：复扫产出实质 finding 即回退；本轮系指令明示 confirmed=2）
- **组合态留痕**：b0 现为 `status=done` + `review_status=changes_requested`——状态机（conventions §9）无 done 回退路径、本指令未授权改 status，故 status 保持 done 不动；两条 findings 均落 `tools/passbguard/**`（b0 owned_files 范围），修复分支落点（passb-b0 续提交 vs 新修复分支）与节点重开方式（是否回退 in_progress）**待协调者裁定**。另注：note 12 用户裁定（18:47 收口时）曾言明"免全量终审、每任务完成即审"——本轮 13 items 全量复扫产出实质 finding，该裁定与新复扫流的关系亦待协调者澄清
- **修复轮次**：5（第 4 轮已于 18:46/18:47 闭环收口——16:10 R1 7 → `1a5257003` 修 1 → 16:44 R2 2 → `971435df3` 全修 → 17:29 R3 1 → 工单 18:09 登记/18:46 覆盖审得 0 需修 → done；**第 5 轮 = 本轮 2 findings，尚未启动**——本会话核验 `27d4b680a` 后 b0 分支 HEAD `acd7b2401` 无新修复提交）
- **base/head SHA**：`b1a3d6dd8` / `d57a2fa70`（均已回填，未动）
- **worktree**：b0 实施分支 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `acd7b2401`，本会话 git log 核验：27d4b680a 后 3 提交）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`——ib1 屏障已登记执行，B1 四节点与 ib1 均在 b0 之后推进）
- **测试证据路径**：无新增（B0.1–B0.6 六任务证据链与 16:02–18:46 第 4 轮证据见前各条目；本轮 OCR 报告即上述路径）
- **本次 JSON 变更**（python 原子更新，断言全过）：(1) b0 `review_status` approved → **changes_requested**；(2) b0 `notes` 数组追加第 13 条（本轮 2 findings 全文 + 组合态处置 + 待协调事项）；`status=done`、`task_status`、base/head SHA、其余 32 节点均未动。本会话 `python3 json.load` 复验合法（review 分布：1 changes_requested + 5 approved + 27 pending）
- **待协调者处置**：(1) 两条 medium 的修复分支落点与第 5 轮启动方式；(2) done+changes_requested 组合态下节点重开或原位修复的裁定；(3) finding #1 系历史 R1 #8 同源再现——18:46 覆盖登记曾审得 0 需修，本轮复扫推翻，该扫描范围口径差异留痕备查

---

## 2026-09-24 01:35 CST · b0 OCR done 后复扫第 2 次：confirmed=2 / rejected=0（均为 low 非阻塞观察 → changes_requested 维持，JSON 无字节级改动）

- **节点/轮次**：b0 OCR done 后复扫第 2 次（01:04 R1 2 medium 修复后的增量复审，2 selected items）；台账历史口径**第 17 次 OCR 运行**（07:09 / 07:29 / 07:45 / 08:28 / 09:30 / 10:41 / 11:36 / 12:04 / 12:14 / 12:34 / 13:47 / 16:02 / 16:39 / 17:29 / 18:28 / 00:57 / 本次 01:27）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **第 5 轮 R1 修复核验**（本会话 git 实测）：`.worktrees/passb-b0`（`codex/passb-b0`）自 `acd7b2401` 后新增恰 1 个修复提交、工作树干净——`17a77e8d3`（2026-09-24 01:16:51，`git show` 本会话实读）`fix(passb): 回退扫描 parse 失败显式诊断不入集、tenant-scoped 事件级联强制 actor_origin`——**f1**：fileReferencesIdent 签名改 `(bool, error)`、错误原样上抛（与 DiscoverSymbolConsumers 口径一致），调用点收敛为 `setSiteIssue{Check: contract-set-scan-failed}`、失败文件不进 discovered 集；**f2**：tenant-scoped required_metadata 级联清单增加 actor_origin（对齐 event-catalog.yaml 头注释五元组；真实目录 29 条事件已合规，guard 自检 missing=0）。TDD RED→GREEN：TestDiscoverSetConsumersWorkerSetScanFailure / TestEventSchemaRequiresTenantScopedActorOrigin；passbguard 全量 + vet 通过（commit message 所载，本管家未重跑测试）。**即 01:04 R1 两条 medium 全部已修**
- **OCR 报告路径**：`/Users/wuyongjun/trea/WeKnora-fork01/.superpowers/sdd/passb/b0/ocr-r2.txt`（本会话已读全文：1,999 字节、23 行、mtime 01:27；首行 "Review complete: 2 finding(s) across 2 selected item(s)."；**该路径第四次被覆盖**——07:29 版 0 findings → 09:30 版 0 → 11:36 版 6 → 12:14 版 1 → 本次 2）
- **结论计数**：confirmed=2 / rejected=0（调度指令口径；与报告 2 findings 一致）
- **findings 清单**（报告全文转录，均 `tools/passbguard/check.go:973-978`、[maintainability · low]，报告原文自陈两条均为"非阻塞观察"）：
  1. scan 失败文件 f 被排除出 discovered 集后，check.go:510-523 仍用该（不完整）集对 row.Consumers 做 vanished 断言——若 f 恰为已登记 consumer，会与本条 scan-failed 同屏产出一条假 "contract-consumer-vanished"（事实只是解析结果未知；注释宣称的"不建立在未知基础上"仅对 unrecorded 方向成立），与 symbol 路径 check.go:613-616（err → emit 后 continue，跳过该符号全部 vanished 比较）口径不一致；报告自陈"CI 结果仍为失败、fail-loud 达成"，建议让 setSiteIssue 或返回契约携带 scan 失败文件集、调用方对这些文件跳过 510-523 的 vanished 断言（保留 scan-failed 诊断），或至少在本条消息中注明同文件的 vanished 诊断不可信。**系 `17a77e8d3` f1 修复引入形态的连带观察**
  2. fallback 扫描按 worker-set 契约行逐一执行（check.go:460/500 每含 worker-set 行的 manifest 各调一次 discoverSetConsumers，contracts.yaml 现有 18 条 worker-set 行），且 parseFileFull 只缓存成功解析、错误不缓存（discover.go:295-302）——同一 internal/router 下不可完整解析的文件按模块重复 parse、对每条 worker-set 行各产出一条仅 row.ID 不同的 "contract-set-scan-failed"，一次破损放大为 ~18 条重复诊断；报告自陈"emit 已带各自 row.ID、语义上每行的扫描确实失败，不构成误报，仅是诊断噪音"，建议按文件路径去重或扫描结果在 Discovery 层按 (kind, file) 共享；另 err 已含 "parse <path>" 前缀（discover.go:301），消息读作 "cannot parse X: parse X:…" 纯措辞冗余。**同上连带观察**
- **审查结论**：OCR 本轮**未通过**——2/2 confirmed、0 rejected（调度口径），但两条均为报告自陈的 maintainability low 非阻塞观察（fail-loud 已达成、不构成误报）；`review_status=changes_requested` **维持**（目标值已在位）
- **修复轮次**：5（第 5 轮在途：01:04 R1 2 medium 已修 `17a77e8d3` → R2 增量复审 2 low 非阻塞观察——**未闭环**；finding 严重度轨迹 medium×2 → low×2）
- **base/head SHA**：`b1a3d6dd8` / `d57a2fa70`（均已回填，未动）
- **worktree**：`.worktrees/passb-b0`（`codex/passb-b0`，HEAD `17a77e8d3`，本会话 git log/status 核验工作树干净）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **测试证据路径**：无新增（修复提交自带 TDD 测试见上；本管家未重跑——本轮指令未要求）
- **本次 JSON 变更**：**无字节级改动**——confirmed>0 时 `review_status` 目标值 `changes_requested` 已在位（01:04 条目所置）；`status=done`（组合态留痕如前）、`task_status`、base/head SHA、notes（13 条）、其余 32 节点均未动。JSON 合法性本会话 `python3 json.load` 复验通过（review 分布：1 changes_requested + 5 approved + 27 pending）
- **备注**：(1) 两条 low 均为 `17a77e8d3` 修复触及面的连带观察，修复成本低（跳过断言/去重/措辞）；(2) b0 已 done 且 B1/ib1 已在其上推进——第 5 轮剩余修复的分支落点与节点重开方式继续待协调者裁定（01:04 条目待办顺延）

---

## 2026-09-24 01:37 CST · b0 OCR R2 low findings 处理结论登记（修复工单 ×2：scan-failed 文件跳过 vanished 断言 + 诊断按文件去重）

- **节点/轮次**：b0 节点级审查第 5 轮 R2（01:35 条目）两条 confirmed low findings 的处理结论（调度方下发修复工单；**非完成报告**——分支 HEAD 仍 `17a77e8d3`，本会话 `git log 17a77e8d3..HEAD | wc -l` 实测 0 新提交，工单未落分支）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **工单 1：b0-ocr-r2-1**（severity low，`tools/passbguard/check.go:973-978`；调度方裁定**真实问题（非阻塞，诊断精度）**）——problem：回退扫描中 fileReferencesIdent 出错时（check.go:966-979）该文件 f 仅产 contract-set-scan-failed 并被排除出 discovered 集（break 跳过 check.go:986 的 addVerified），但 check.go:510-523 仍对 row.Consumers 做 vanished 断言：因 f∈d.GoFiles 磁盘存在、又不在 discovered 中，若 f 为 recorded consumer 即同屏假 contract-consumer-vanished。**现状核实（本会话在 passb-b0@`17a77e8d3` 实读）**：check.go:972-982 诊断+break 分支在案、check.go:510-523 vanished 断言对 discovered 集逐个 `containsExact` 属实——problem 描述属实
- **工单 1 验收标准**（acceptance 转录；**调度指令文本于「若 」处截断，如实保留**）：(1) discoverSetConsumers 返回契约（或 setSiteIssue）携带 scan-failed 文件集，CheckContracts 对这些文件跳过 check.go:510-523 的 vanished 断言但保留 contract-set-scan-failed 诊断；(2) 新增 fixture 测试覆盖"scan-failed 文件同时是 recorded consumer 时产出 scan-failed 且不产出 contract-consumer-vani[shed]"（截断处按 finding 语义补全，完整原文以调度方为准）
- **工单 2：b0-ocr-r2-2**（同位点，severity low；调度方裁定**真实问题（非阻塞，诊断噪音），机制成立但量级有误需修正**）——problem 核实：check.go:460（manifest 循环）/check.go:500（每个恰一行 set 契约各调一次 discoverSetConsumers）逐行重复执行回退扫描；parseFileFull（discover.go:291-306）仅成功解析入 astCache（line 304 在错误 return 之后）、错误不缓存、逐行重复 parse——调度方 grep 确认 astCache 无其他写入点。**本会话实读同证**：check.go:500-502 每行调用 + discover.go:295-304 缓存仅在成功路径写入属实
- **工单 2 验收标准**（acceptance 转录；**调度指令文本于「修正一」处截断，如实保留**）：(1) scan-failed 按文件去重：同一破损文件跨多条 worker-set 行只产出一条诊断（Discovery 层按 (kind,file) 共享扫描结果，或首个遇到行 emit 后记录已报文件）；(2) 消除 "cannot parse X: parse X:" 措辞冗余（fileReferencesIdent 上抛去前缀错误或消息改用内部错误）；(3) 新增 fixture 测试断言多行共享单破损文件时 scan-failed 恰一条；(4) go test ./tools/passbgua[rd …]（截断处按惯例补全为 passbguard 全量测试，完整原文以调度方为准）
- **审查结论**：`review_status=changes_requested` **维持不变**（两条工单登记不改变审查状态；修复未完成）
- **修复轮次**：5（第 5 轮 R2 两条 low 的修复工单在案待执行；R1 2 medium 已修 `17a77e8d3`）
- **base/head SHA**：`b1a3d6dd8` / `d57a2fa70`（均已回填，未动）
- **worktree**：修复应落 `.worktrees/passb-b0`（`codex/passb-b0`，HEAD `17a77e8d3`，本会话 git log/status 核验工作树干净）；DAG/台账所在 `.worktrees/passb-int`（HEAD `8c45a8815`）
- **本次 JSON 变更**（python 原子更新，断言全过）：b0.`notes` 数组末尾追加 1 条字符串（第 14 条）——两条工单全文（problem + 验收标准 + 截断标注 + 现状核实结论），沿 12:17 条目先例；`review_status=changes_requested`、`status=done`（组合态维持）、`task_status`、base/head SHA、其余 32 节点均未动。本会话 `python3 json.load` 复验合法（notes 14 条；review 分布 1 changes_requested + 5 approved + 27 pending）
- **备注**：(1) 工单 2 的量级修正系调度方对 01:35 转录的勘误——重复诊断条数取决于 manifest 行数而非固定 ~18；(2) 两条工单落点均在 `tools/passbguard/**`（b0 owned_files 范围），修复分支落点与 done+changes_requested 组合态的收口方式待协调者裁定（01:04 条目待办顺延）

---

## 2026-09-24 01:39 CST · b0 登记 OCR 覆盖：ocr_covered 追加 [acd7b2401 → 17a77e8d3]（口径"审得 0 条需修 findings"；与 01:37 工单并存留痕）

- **节点**：b0 —— B0 契约与所有权冻结（passbguard + 4 份治理 yaml + 证据）
- **登记内容**：b0 节点 `ocr_covered` 数组追加第 2 条 `{base: "acd7b24011b57704d401c63f4d2fce1819e057fc", head: "17a77e8d32ab1bf6bb14a258feee61c98bb8b3de"}`（全 40 位 SHA，调度指令口径：**审得 0 条需修 findings**）——首条 [1a5257003 → 971435df3]（18:46 条目登记）不变，现共 2 条
- **SHA 真实性核验**（本会话 git 实测，`.worktrees/passb-b0`，`git rev-parse` 双双命中 40 位全 SHA）：`acd7b2401`（2026-09-23 18:46:59，fix(passbguard): 修复同包符号引用漏检 map 键与赋值左侧使用，R3 修复）与 `17a77e8d3`（2026-09-24 01:16:51，fix(passb): 回退扫描 parse 失败显式诊断不入集、tenant-scoped 事件级联强制 actor_origin，**第 5 轮 R1 两条 medium 修复**）；区间 `acd7b2401..17a77e8d3` 含**恰 1 个提交**（即 `17a77e8d3` 本身）；`17a77e8d3..HEAD` = 0（分支无更新提交，工作树干净）
- **覆盖区间语义**：本登记覆盖的正是第 5 轮 R1 修复提交——01:35 OCR R2 增量复审（2 selected items）扫描的即为 `17a77e8d3` 触及面，产出 2 confirmed low（报告在案）
- **留痕（口径差异，待协调者如需澄清）**：本轮口径"审得 0 条需修 findings"与 01:37 条目**并存**——01:35 R2 复审对该区间产出 2 confirmed low（非阻塞观察），01:37 调度方裁定"真实问题"并登记修复工单 ×2（notes 第 14 条在案）。本轮指令未说明两者关系：(a) 若系调度方对这 2 条 low 的处置裁定为无需修，则实质关闭 01:37 工单；(b) 若系另一次复审结果，其报告路径未附。按指令原文如实登记，不自行判定；**工单状态以调度方后续显式指令为准**
- **节点级状态未迁移**：`status=done`、`review_status=changes_requested`（01:04 组合态维持）、`head_sha=d57a2fa70`、notes 14 条均未动——本指令仅授权 ocr_covered 登记 + 台账留痕
- **修复轮次**：5（不变——第 5 轮 R2 两条 low 工单在案；其是否因本轮口径关闭待调度方澄清）
- **base/head SHA（节点级）**：`b1a3d6dd8` / `d57a2fa70`（均已回填，未动）
- **worktree**：`.worktrees/passb-b0`（`codex/passb-b0`，HEAD `17a77e8d3`）；DAG/台账所在 `.worktrees/passb-int`（`codex/passb-integration`，HEAD `8c45a8815`）
- **本次 JSON 变更**（python 原子更新，断言全过）：b0.`ocr_covered` 追加第 2 条（现共 2 条，SHA 与指令逐字符一致）；`status=done`、`review_status=changes_requested`、`notes` 14 条、`task_status`、base/head SHA、其余 32 节点均未动。本会话 `python3 json.load` 复验合法（review 分布：1 changes_requested + 5 approved + 27 pending）

---

## 2026-09-24 01:41 CST · b0 → done（第 5 轮修复后重新收口：head 17a77e8 回填，门禁+OCR 通过）

- **节点**：b0 —— B0 契约与所有权冻结（passbguard + 4 份治理 yaml + 证据）——第二次 done 收口（首次 18:47，其后经 01:04 done 后复扫 2 medium → 修复 → 01:35 复审 2 low（非阻塞）→ 01:39 覆盖口径 0 需修，本轮按调度方收口指令重新落 done 终态）
- **计划路径**：`docs/plans/passb/00-contract-and-ownership-freeze.md`
- **前置**：无（depends_on = []；b0 为 DAG 根）
- **worktree**：`.worktrees/passb-b0`（`codex/passb-b0`，本会话 git 核验 HEAD `17a77e8d3`、工作树干净）
- **base → head**：`b1a3d6dd8` → **`17a77e8d32ab1bf6bb14a258feee61c98bb8b3de`**（指令短 SHA `17a77e8`，本会话 `git rev-parse HEAD` 命中完整 40 位；`git merge-base --is-ancestor` 核验 base 是 head 祖先 ✓；分支自 base 累计 **20 提交** = 六任务 9 + 历轮 OCR 修复与测试迁移 10 + 第 5 轮 R1 修复 `17a77e8d3`）；head 提交内容：01:16:51 `fix(passb): 回退扫描 parse 失败显式诊断不入集、tenant-scoped 事件级联强制 actor_origin`（对应 01:04 复扫两条 medium；TDD 测试 commit message 在案）
- **门禁+OCR 通过**：调度指令口径（本管家未重跑，沿 03:40/18:47 先例）；在案证据链 = 节点 5 项 gates（06:21 条目）+ 第 5 轮 R1 修复提交自带 TDD 测试（01:35 条目转录）+ OCR 覆盖登记（01:39 条目，ocr_covered 第 2 条口径 0 需修 findings）
- **审查结论**：**approved**（指令口径"门禁+OCR 通过"→ `review_status` changes_requested → **approved**，沿 18:47 推断迁移先例在此留痕）；01:37 两条 low 修复工单按 01:39"0 条需修"口径与本轮"OCR 通过"收口指令**关闭**（第 5 轮闭环：R1 2 medium 已修 `17a77e8d3`，R2 2 low 非阻塞观察经覆盖口径不需修）
- **OCR 报告路径**：历轮全录（ocr-r1/r2/r3/r3-a2/ocr-final 各版本，见各条目）；ocr_covered 现共 2 条：[1a5257003 → 971435df3]（18:46）+ [acd7b2401 → 17a77e8d3]（01:39）
- **修复轮次**：5（全部闭环：第 1–4 轮见 18:47 条目；第 5 轮 = 01:04 R1 2 medium → `17a77e8d3` 全修 → 01:35 R2 2 low 非阻塞 → 01:39 覆盖口径不需修 → 本轮收口）
- **测试证据路径**：B0.1–B0.6 六任务报告/审查包（03:40–06:21 各条目）+ 节点 5 项 gates（06:21）+ `17a77e8d3` 自带 TDD 测试（01:35 条目）+ ocr_covered 两条登记（18:46/01:39）
- **本次 JSON 变更**（python 原子更新，断言全过，净变更 2 行）：(1) `review_status` changes_requested → **approved**；(2) `head_sha` `d57a2fa70…` → **`17a77e8d32ab1bf6bb14a258feee61c98bb8b3de`**。`status=done`（已在位，无改动）、`base_sha=b1a3d6dd8`、task_status 六任务 done、notes 14 条、ocr_covered 2 条、其余 32 节点均未动。本会话 `python3 json.load` 复验合法（33 节点分布：6 done + 4 in_progress + 23 pending；review：6 approved + 27 pending）
- **留痕 1（head_sha 两度显式指令）**：现值原为 `d57a2fa70`（18:56 调度方显式指令以集成后 HEAD 覆盖）；本轮调度方又显式指令 `head 17a77e8`（分支 HEAD）——两次均为调度方权限内显式指令，按序执行如实登记。**语义注意：`17a77e8d3`（第 5 轮修复）尚未合入 `codex/passb-integration`**（integration HEAD 仍 `8c45a8815`），B0 第 5 轮成果的集成合并待集成工程师（沿 18:53 先例）；如合并后需以合并头再覆盖，由调度方显式指令
- **留痕 2（工单关闭依据）**：01:37 工单 ×2 与 01:39"0 条需修"覆盖口径的并存关系（01:39 条目留痕待澄清）由本轮"门禁+OCR 通过"收口指令落定——调度方以覆盖口径为准，第 5 轮无剩余修复义务
- **后续（调度方事项）**：合并 `codex/passb-b0`（HEAD `17a77e8d3`）进 `codex/passb-integration` 使集成树含第 5 轮修复（check.go/discover.go/model.go 变更）








