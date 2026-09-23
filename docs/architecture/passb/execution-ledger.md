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
- **要点留痕**：(1) 上条预告闭环——`native_archive.go`（service+handler 两文件）已改派 **34-protocol**，engine brief 计数同步 39→38；(2) shared-host 36 文件 HandlerSessionRuling 全量冻结（wiki_fixer_scope→23、workbench 族→40、craft 族→41、agent_run/agent_stream_handler→33、session 族 15 文件→35；pagination/upload-limit/error helper 保持 platform 零认领）；(3) housekeeping 裁定入 knowledge-process.md（业务规则归 24、System 经窄端口调度、禁双实现）；(4) 歧义短语（先合并者/二选一执行等）已由 CheckAmbiguity 机器禁止；(5) Mimosa hook 仍 `scanner_enobufs`

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







