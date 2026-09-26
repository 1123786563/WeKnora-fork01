# Issue #106 最终交付报告：自行托管插件与清单安装（终版）

- 日期：2026-09-26（终版。本文件前身为 2026-09-24 撰写的中间版报告——写于分支 42 提交时点（HEAD `9a12d82ed`），当时结论为"部分完成：17/20 任务未实施"；其后主流程经 R9 轮指令转入实施模式（`fix-plans.md:854-862`），T03、T05–T20 全部续作完成，分支推进至本版 HEAD `28c46776`（149 提交）。中间版的关键事实（第一代/第二代 OCR 轮次、编号体系拆解）已在本版第 7 章沿革中保留，正文以终态为准）
- 仓库：`1123786563/WeKnora-fork01`（主 checkout `/Users/wuyongjun/trea/WeKnora-fork01`，本报告未改动主 checkout）
- **终局增补（2026-09-26 晚，第 10 章）**：B+A 裁决落地（9 项后端安全/正确性处置）+ 终局 OCR 三轮（19/27/30 findings，F/F2 修复轮 9 项）后，分支推进至 HEAD `4de4e4258`（163 提交）；终局结论 `ocrPass=false`、维持部分完成，剩余发现为用户裁决接受、随 diff review 处理——**最新结论以第 10 章为准**（第 1–9 章保留为上版时点 HEAD `28c46776`、149 提交的记录）
- Worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue106`（分支 `codex/issue-106-self-hosted-plugins`）
- 数据来源：① 主流程交付的执行数据 JSON（任务账、门控、OCR、Ruling 的权威记录，本报告逐字段采信并标注；**该 JSON 非仓库文件，系主流程随任务书内联传入，本报告撰写会话已将其原样转录落盘为 `docs/plans/issue-106-execution-data.json`（worktree 内绝对路径见第 4 章）供复核——转录件 `_meta` 字段注明转录性质，`taskSummary[].evidence` 字段在原内联文本中即存在截断，转录原样保留**）；② 本报告撰写会话在 worktree 内的**只读实测**（git 事实、迁移/挂载点/OCR 报告文件核验，逐一列于附录 B）；③ worktree 内 Ledger / fix-plans / OCR 报告 / 任务账本原文（均已读取，引用处标行号）
- **诚实边界**：本分支全部工作仅存在于本地 worktree。**未推送（本报告实测分支 `upstream` 为空——此实测仅能证明分支未被推送）、未合并、未部署、未发布、未关闭任何 GitHub Issue、未在 GitHub 发表任何评论**；其中"未关闭 Issue、未发评论"属本流程约束的宣称，本报告撰写会话未调用 gh api 核验 GitHub 当前状态（未验证项，如实标注）。执行数据中 12 个 Issue 的状态记录仍为规划时点快照。下文所有"完成/通过"均指本地 worktree 内的状态，门控结论为执行数据记录（本报告撰写会话未复跑，见 6.4）。

---

## 1. 总结论

**部分完成（本地）。实施与验收层面全部完成（20/20 任务、三条门控全绿、十条验收边界核销、整分支评审通过），但整分支 OCR 门控未通过（`ocrPass=false`，第 3 轮仍有 9 条有效阻塞发现未修复），不满足最终完成标准——如实报告部分完成（执行数据终裁 Ruling 原文："OCR 三轮后仍有 9 个未解决有效阻塞发现——不满足最终完成标准，如实报告部分完成"）。**

### 已完成（本地）

1. **20/20 实施任务全部完成**（执行数据 `taskSummary`：T01–T20 全部 `done`；账本 `.superpowers/sdd/2026-09-23-issue-106/tasks/T01.md`…`T20.md` 共 20 份，本报告 `ls` 实测齐全，抽查 T03/T06/T17/T20 状态行均记"完成"）。覆盖 #106 全部 11 个子 Issue（#108–#118）与根 Issue 的汇总验收（T20）。
2. **三条基线门控全绿**（执行数据 `gates` 记录，均 exit 0）：`pnpm run typecheck:shared`、`pnpm run test:shared`、`go test ./...`（`gatesGreen=true`）。
3. **#106 十条验收边界核销**：T20 验收套件 `TestPlugin106Acceptance` 十子测试（B1–B10）全部 PASS（真 PostgreSQL 容器，一次性 docker postgres:17-alpine，测毕销毁；账本 `tasks/T20.md` 记录 summary 行 `10 boundaries verified (B1..B10)`，最终干净复跑 3.52s）。详见第 3 章。
4. **整分支评审通过**（执行数据 `branchReviewApproved=true`）。
5. **OCR 质量闭环推进了大量轮次**：第三代整分支 OCR 三轮（45/35/22 findings）中第 1、2 轮的修复轮（09-26 R1 轮 33 条、R2 轮 22 条**有效**发现）**批次内全部修复、无剩余项**（原始 findings 与有效数的差额为有效性过滤，逐条下落无明细记载，见 7.2 差额说明；`fix-plans.md:1369-1650` 完成记录）；任务级 OCR 20 个任务全部登记干净（执行数据 `taskOcrRanges` 全部 `clean=true`）。
6. **范围缺口 GAP-1…GAP-6 全部落地**（第 3 章附表）。

### 未完成（及原因）

1. **整分支 OCR 未通过**（`ocrPass=false`）：第三代第 3 轮 OCR（22 findings）**未再组织修复轮即终裁结案**，9 条 high/medium 有效阻塞发现未修复（第 7.4 节逐条清单）。这是唯一的未过门控。
2. **GitHub 侧零动作**：分支未推送、Issue 未关闭——按约束属用户决策（第 9 章）。

### 执行数据一致性说明（需用户知悉）

执行数据 JSON 内部存在两套口径：`issueStatus` 记 12 个 Issue 全部"未实施（无关联任务）"，而 `taskSummary` 记 20 个任务全部 done、`branchReviewApproved=true`。两者不矛盾但时点不同：`issueStatus` 是 **2026-09-23 规划阶段**（层级树扫描时）的 GitHub 状态快照，此后从未更新；`taskSummary` 是终态。本报告以 `taskSummary` + git 实测为准，并在第 2 章对每个 Issue 给出终态判定。GitHub 上 12 个 Issue 的当前实际状态本报告未核验（本会话未调用 gh api）；可证的部分仅为：分支未推送（`upstream=[]` 实测），其余"远程零变化"（Issue 未关、无评论）系流程约束的宣称而非实证。

---

## 2. 每个子 Issue 状态

依据：执行数据 `taskSummary`（终态）+ Ledger 任务账本 + 追踪矩阵 B1–B10 的 T20 核销结果。任务↔Issue 映射取计划索引任务总表（`2026-09-23-issue-106-00-index.md:38-59`，本报告读取核实）。判定口径：**"本地完成"= 承载任务全部 done 且账本记完成；"已验证完成"= 另有 T20 验收边界真 PG 核销或任务级测试证据；全部 Issue 均带 OCR 未决项或结构性未验证项（见各行"残余/未决"列与第 9 章）。**

| Issue | 标题 | 状态 | 承载任务 | 关键证据（来源） | 残余/未决 |
| --- | --- | --- | --- | --- | --- |
| #106 | Spec: 自行托管插件与清单安装（根） | **本地完成（验收门通过；OCR 门未过）** | T20 | `TestPlugin106Acceptance` 十子测试 B1–B10 全 PASS（真 PG；`tasks/T20.md` 测试证据表）；R2 裁决其 implement 语义=汇总验收不承载独立实现 | 整分支 OCR `ocrPass=false`（9 条阻塞发现，7.4）；B9 承载的存量迁移 harness 缺陷转交（`tasks/T20.md` Concerns 1） |
| #108 | 插件 01｜管理员预览插件清单 | **已验证完成（本地）** | T01/T02/T03 | B1 核销（预览全链含非管理员 403/受限地址/声明不符，真 PG）；T03 前端面板 `PluginsSettingsPanel` + settings 分区 `registry.ts:43`（本报告读码实证） | round-3 未决 R3-01（两个 'plugins' key 冲突致管理面板深链被劫持，bug·high）直接落在本 Issue 交付面上 |
| #109 | 插件 02｜提供只读 Jira MCP 示例服务 | **已验证完成（本地）** | T04/T05 | B7 核销（Jira 协议消费全链真 PG）；示例服务自测 ok（含 OAuth 全流程/两成员隔离/超时）；R1 轮 G3 批次双 digest 断言（`fix-plans.md:1394-1400`） | 真实 Atlassian REST v3 行为维持外部不可验证（无真实 Jira 账号；rulings.md R3 以官方文档裁定字段名，fake 与裁定一致） |
| #110 | 插件 03｜确认空间安装并供成员发现 | **已验证完成（本地）** | T06/T07/T08 | B2（confirm→accept 链）、B4（发现/隔离）、B6（停用）、B10（确认重核）核销；迁移 000190/000111 落地（本报告 `ls` 实测）；R12 轮 F01/F02/F06/F14-F17/F21 补偿健壮性修复（`fix-plans.md:1100-1138`） | round-3 未决 R3-15（AcceptUpgrade 崩溃窗口 fail-open，security·high）、R3-16（SetInstallationState 未纳互斥，bug·medium）落在本 Issue 后端交付面 |
| #111 | 插件 04｜在对话中调用无账号插件工具 | **已验证完成（本地）** | T09/T10 | B8 核销（discover/describe/call 成功/停用后隐藏）；R1 轮 G1 批次 drift 三门测、G2 批次插件守卫四测（`fix-plans.md:1382-1390`） | 生产 server 进程未启动验证（fx DI 以编译+container 包测试闭合，6.4） |
| #112 | 插件 05｜成员个人授权与撤销 | **已验证完成（本地）** | T11/T12 | B4 三态/撤销/过期隔离核销（真 PG）；`connections/me` API + 前端入口（T12） | round-3 未决 R3-09（PluginsPanel epoch 竞态，bug·medium）落在授权面板上 |
| #113 | 插件 06｜在 WeKnora 对话中查询 Jira 待办 | **已验证完成（本地）** | T13 | B7 对话端到端核销（本人本周+来源链接/B 隔离/注入拒绝零外呼，真 PG） | round-3 未决 R3-17（兜底刷新缺卸载守卫，bug·low）相关面板路径 |
| #114 | 插件 07｜预览候选插件版本差异 | **已验证完成（本地）** | T14/T15 | B2 五维差异/降级标注/失败保旧/幂等核销；api-client 五维解析器 39/39（R1 轮 N1 批次证据） | round-3 未决 R3-08/R3-10（升级面板陈旧闭包竞态×2，bug·medium）落在本 Issue 前端交付面 |
| #115 | 插件 08｜管理员接受插件升级 | **已验证完成（本地）** | T16 | B2 接受切换/B5 升级新增写工具默认关核销；R2 轮 A 批次 F20（卸载入锁）RED/GREEN 双验证（`fix-plans.md:1573-1580`） | round-3 未决 R3-15（security·high，与 #110 同文件）直接涉及 AcceptUpgrade 幂等短路路径 |
| #116 | 插件 09｜阻止已接受版本的远端能力漂移 | **已验证完成（本地）** | T17 | B3 核销（发现被阻/detected+明细/resolve 重定基，真 PG）；R1 轮 F15 drift 三门、R2 轮 F33 协议相对 URL 凭据掩码（`fix-plans.md:1382-1386,1573-1576`） | round-3 未决 R3-18（driftResolve try 块串联误报，bug·medium）落在复审闭环前端 |
| #117 | 插件 10｜新增写工具默认关闭 | **已验证完成（本地）** | T18 | B5 核销（默认关/拒绝零派发/超时零派发）；`plugin_write_tools_integration_test`（升级新增写工具默认关+WriteCalls==0+启用后恰一次）在 integration 套件 PASS | round-3 未决 R3-22（GetMCPServiceResources 漏插件守卫，security·medium）落在相邻资源读取面 |
| #118 | 插件 11｜启用写工具并由成员审批 | **已验证完成（本地）** | T19 | B5 审批全场景核销（arm→审批卡带参数原文→reject/timeout 零派发，真 PG）；R1 轮 N2 批次升级面板接入、R2 轮 D/E 批次面板竞态修复（`fix-plans.md:1408-1420,1586-1600`） | round-3 未决 R3-09/R3-10 部分落点在审批面板联动路径 |

注 1：以上"已验证完成"中的 B1–B10 核销均出自 T20 验收套件（真 PG 一次性容器）与各任务账本测试证据；本报告撰写会话未复跑这些测试（6.4）。
注 2：执行数据 `issueStatus`（规划时点快照）对 12 个 Issue 全部记"未实施（无关联任务）"，与上表终态的关系见第 1 章一致性说明。

---

## 3. #106 验收标准覆盖情况

十条验收边界定义见追踪矩阵 A 节（`2026-09-23-issue-106-trace-matrix.md:10-19`）。**结论：10/10 边界经 T20 `TestPlugin106Acceptance` 真核销（B1..B10 十行 evidence + summary，`tasks/T20.md`）**；矩阵引用的全部 50 个测试函数名经 T20 账本逐一 grep 核实存在（"all verified"）。

| # | 验收边界（摘要） | 承接 Issue/Task | T20 核销 evidence（账本记录） |
| --- | --- | --- | --- |
| B1 | 清单交付 + 管理员安装前预览核验 | #108 / T01–T03 | 非管理员 403 / 字段齐备 / digest 口径 / 受限地址拒绝 / 声明不符拒绝 |
| B2 | 安装固定版本；差异+手动接受；失败保旧 | #110/#114/#115 / T06/T14/T16 | 固定 1.0.0 / diff / 失败保旧 / 接受 2.0.0 |
| B3 | 运行时以已接受快照阻断漂移 | #116 / T09/T17 | 发现被阻 / detected+明细 / resolve 重定基 |
| B4 | 成员发现 + 个人授权隔离 | #110/#112 / T07/T11 | 发现 / A-B 凭据隔离 / 撤销只影响本人 / 跨空间 not found |
| B5 | 新写工具默认关闭 + 审批零写入 | #117/#118 / T18/T19 | 默认关 / 拒绝零派发 / 超时零派发 / 审批卡带参数原文 |
| B6 | 停用/撤销/过期/候选不可达 | #110/#112/#113/#114/#115 | 停用不可见+旧 ref 拒绝 / 撤销 / 过期引导 / 候选不可达保 v1 |
| B7 | Jira 纵向案例（本人本周/服务端时间范围/拒绝注入） | #109/#113 / T04/T05/T13 | 本人本周+来源链接 / B 隔离 / 注入拒绝零外呼 |
| B8 | 无账号工具独立能力 | #111 / T09/T10 | discover/describe/call 成功 / 停用后隐藏 |
| B9 | 手工 MCP 兼容；旧工具不因迁移自动视为已批准 | #110 等 / T06/T07/T09/T18 | 同会话共存 / 缺行=启用（读写均然）/ 000190.down 后手工行幸存 |
| B10 | 网络与租户边界（横切） | #108/#109 / T01/T02/T04 | 预览/升级受限地址拒绝 / 漂移核验 fail-closed 零持久化 / 跨空间隔离 |

**B9 专门说明（存量缺陷如实呈现）**：矩阵 B9 行原承载的 `TestPluginInstallationsMigrationUpAndDown` 存在 harness 顺序缺陷（`pluginMigrationsOpenDB` 先 apply 000190.up（内含 `ALTER TABLE mcp_services`）后建最小父表 → 任何真 PG 运行必失败 `relation "mcp_services" does not exist`；属 T06 所有权，T11 发现转交、T20 真 PG 复现确认，至今未修——`tasks/T20.md` Concerns 1）。T20 按账本 Ruling 以 `accMigrationDownKeepsManual` 在本切片所有权内**复述同一迁移契约**（手工行幸存+列删除断言）核销 B9；迁移文件本身无罪（评审以正确顺序 apply 成功，rulings 核实类条目 `[T06] PG 迁移集成测试 env 门控`）。该缺陷仍是遗留项（第 9 章）。

**GAP-1…GAP-6 落实状态**（计划索引全局裁决表 `2026-09-23-issue-106-00-index.md:17-26` + 承载任务终态）：

| GAP | 裁决方案 | 承载任务 | 终态 |
| --- | --- | --- | --- |
| GAP-1 清单协议固化 | `weknora.plugin/1` JSON 协议 + canonical-JSON SHA-256 digest | T01/T02 | **已落地**（协议类型+校验测试族；R1 轮 F03 manifest 副本补 content_digest 双比对） |
| GAP-2 网络抓取限制 | 统一 `utils.ValidateURLForSSRF` 前置 + SSRF 安全客户端 + `SSRF_WHITELIST` 放行 + 凭据仅 env | T01/T02 | **已落地**（Global Constraints 横切 + B10 受限地址拒绝核销；R2 轮 F33 协议相对 URL 掩码补强） |
| GAP-3 数据迁移与回退 | PG 000189/000190 + SQLite 000110/000111 twin；up 零回填；down 反序清理保手工行 | T02/T06 | **已落地**（四组迁移文件本报告 `ls` 实测存在；up 零回填+down 保手工行经 T20 accMigrationDownKeepsManual 真 PG 核销；唯 T06 专属 harness 缺陷未修，见上） |
| GAP-4 OAuth 身份映射 | 复用 per-principal MCP OAuth（service_id=物化服务 ID），不新建表 | T11 | **已落地**（B4 三态/撤销/过期真 PG 核销） |
| GAP-5 插件远程调用授权链 | 走既有 approval.Gate，不挂 ADR-0013 ToolDispatchPreflight | T09/T19 | **已落地**（B5 审批全场景核销；ADR-0013 核可，见第 8C 章） |
| GAP-6 漂移处置闭环 | drift/check + drift + drift/resolve 三端点复审闭环 | T17 | **已落地**（B3 核销；R1 轮 F15 三门强化） |

---

## 4. 关键路径清单（worktree 内绝对路径，均经本报告 `ls`/读取实测存在）

| 类别 | 绝对路径 |
| --- | --- |
| 层级树（12 节点事实 + 依赖边） | `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue106/docs/plans/issue-106-issues-scope.md` |
| Scope-DAG（范围裁决 + 依赖 DAG，Kahn 无环验证） | `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue106/docs/plans/issue-106-scope-dag.md` |
| 代码基线勘察 | `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue106/docs/plans/issue-106-code-baseline.md` |
| 计划索引（20 任务总表 + GAP 全局裁决 + 波次 + Global Constraints） | `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue106/docs/plans/2026-09-23-issue-106-00-index.md` |
| 切片计划 01–12（T01–T20） | `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue106/docs/plans/2026-09-23-issue-106-01-manifest-preview.md` … `2026-09-23-issue-106-12-acceptance.md`（同目录 12 份，`ls` 实测逐一存在） |
| 追踪矩阵（B1–B10 → Issue → Task → 测试） | `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue106/docs/plans/2026-09-23-issue-106-trace-matrix.md` |
| Ledger 根目录 | `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue106/.superpowers/sdd/2026-09-23-issue-106/` |
| ├ 任务书 briefs（T01–T20，20 份实测存在） | `…/.superpowers/sdd/2026-09-23-issue-106/briefs/T01.md` … `T20.md` |
| ├ 任务执行记录（T01–T20，20 份实测存在） | `…/.superpowers/sdd/2026-09-23-issue-106/tasks/T01.md` … `T20.md` |
| ├ Ledger Ruling 记录（LR1–LR4） | `…/.superpowers/sdd/2026-09-23-issue-106/rulings.md` |
| ├ 修复批次计划（R3–R12 + 09-26 R1/R2 轮，含各轮完成记录） | `…/.superpowers/sdd/2026-09-23-issue-106/fix-plans.md` |
| ├ 冲突扫描（conflict 1/2/3 与 planUsable 生效条件） | `…/.superpowers/sdd/2026-09-23-issue-106/conflict-scan.md` |
| ├ 环境记录 | `…/.superpowers/sdd/2026-09-23-issue-106/environment.md`（即 `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue106/.superpowers/sdd/2026-09-23-issue-106/environment.md`） |
| ├ OCR 台账 | `…/.superpowers/sdd/2026-09-23-issue-106/ocr-ledger.json`、`ocr-ledger.md`（**内容停留在 09-23 22:16 时点未回写，见第 9 章 #6**） |
| OCR 报告——第三代整分支三轮（未跟踪文件，现文件内容） | `…/docs/plans/issue-106-ocr-round-1.md`（45 findings）、`issue-106-ocr-round-2.md`（35）、`issue-106-ocr-round-3.md`（22，终轮，本报告全文读取） |
| OCR 报告——任务级（未跟踪文件） | `…/docs/plans/issue-106-ocr-T01.md` … `issue-106-ocr-T20.md`（20 份，`git status` 实测存在） |
| 执行数据 JSON 转录件（主流程权威记录的本 worktree 固化副本） | `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue106/docs/plans/issue-106-execution-data.json`（本报告撰写会话自任务书内联转录落盘，`_meta` 注明性质；主流程原始载体为工作流运行时态、不在仓库内） |
| Spec（已批准）/ 设计文档 | `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue106/docs/specs/2026-09-23-self-hosted-plugins-spec.md`、`2026-09-23-self-hosted-plugins-design.md` |
| 本报告（终版，覆盖 2026-09-24 中间版） | `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue106/docs/plans/issue-106-final-report.md`（未跟踪） |

---

## 5. Worktree / 分支 / 基线 SHA / 最终 HEAD / 关键提交数

| 项 | 值（本报告 2026-09-26 git 实测，命令见附录 B） |
| --- | --- |
| Worktree | `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue106` |
| 分支 | `codex/issue-106-self-hosted-plugins`（`upstream` 为空——**无远端跟踪分支，未推送**，`git for-each-ref` 实测） |
| 基线 SHA | `4bcad69baf033a1310b4dce1372c8153e66adc81`（执行数据 `baseSha`；`git rev-list` 按此区间实测） |
| 最终 HEAD | `28c46776b3b4b75ae8be44d68aaead93505b6e33`（`git rev-parse HEAD` 实测，与执行数据 `headSha` 一致） |
| 基线..HEAD 提交数 | **149**（`git rev-list --count` 实测，与执行数据 `commits` 一致） |

**提交构成（按提交信息任务标签清点，`git log --format='%s' | grep -oE '\[T[0-9]{2}\]' | sort | uniq -c` 实测）**：带任务标签 123 处（个别提交含多标签会被重复计数，以 rev-list 总数 149 为准）、无标签 28 个。标签分布：T01×25、T04×13、T06×12、T07/T08/T09/T15 各 7、T16/T19 各 6、T02/T05/T13/T17/T18 各 4、T11×3、T03/T10/T12/T14/T20 各 2。

**28 个无标签提交的构成（本报告对 `git log --format='%h %s'` 逐条读提交信息实测归类，非推测）**：

| 类别 | 数量 | 实测依据（提交示例） |
| --- | --- | --- |
| 第三代整分支 OCR R1/R2 轮修复代码（提交信息标 "OCR R1/R2 Fxx"） | 12 | R1 轮 6 个（`9450839e9`/`4ada20359`/`246213520`/`1b0c82470`/`0abc330eb`/`77fc18b6a`）+ R2 轮 6 个（`f538ffa1a`/`2b1ba7943`/`b7c49e484`/`584141976`/`bfe3c788b`/`28c46776b`——**最终 HEAD 本身即 R2 轮 F34 补遗提交**） |
| 跨任务转交修复（R12-E 与 T06/T07/T08 转交） | 4 | `749afa047`/`a491224a1`/`79d873a48`/`561ed00c1`（其中 `79d873a48` 即 PluginsPanel 挂载接线提交） |
| 门控回归修复 | 3 | `e050f5e90`（node>=26 wrapper 自愈）/`dfc295b4e`（sqlite 迁移头）/`77f012799`（architectureguard） |
| 终评 R3 共享修复 | 1 | `2bf3bde8c`（身份见下节） |
| docs(sdd) 修复轮计划/完成记录 | 3 | `c5398a74c`/`ac1982396`/`38da1c0d3`（R12/R1/R2 轮 fix-plans 记录提交） |
| docs 规划（09-23） | 5 | `b07fa08c6`…`dc9f9d1e8`（层级/基线/DAG/计划/修订） |

**`2bf3bde8c` 的身份与 OCR 起扫点的选取依据（本报告 git 与台账实测）**：`2bf3bde8cebdc2e5c4e6714f54325bad4fbf6de6` = `fix(container): 整分支评审修复：SSRF 白名单测试清理改快照恢复，消除 -count>=2 二轮回归 [R3]`（09-23 终评 R3 轮共享修复提交），位于**基线后第 10 个提交**（`git rev-list --count 4bcad69ba..2bf3bde8c` = 10 实测）。它是第一轮收尾 OCR 的 `to` 与台账 `maxCleanHead`（`ocr-ledger.md:13-22`）：该收尾轮以 `ocr review --from 4bcad69ba --to 2bf3bde8c` 实跑、结论"干净：0 findings，ranClean"。**台账 clean 结论的精确覆盖语义**：仅指该区间**被 OCR 选片审查的 6 个代码文件** 0 findings——区间 27 个改动文件中 17 个 `docs/plans/*.md`（unsupported_ext）与 4 个 `*_test.go`（default_path）被 OCR 内置选片策略排除（`ocr-ledger.md:18-19` 原文）。后续所有整分支 OCR 轮从 `2bf3bde8c` 起扫增量（`--from` = maxCleanHead，`ocr-ledger.md:20`），**不重扫已判干净区间**——这就是执行数据 `ocrFrom=2bf3bde8c` 而非基线 `4bcad69ba` 的原因。

**关键交付物落地核验（本报告实测）**：

- 迁移：PG `migrations/versioned/000189_plugin_previews.*`、`000190_plugin_installations.*` + SQLite `migrations/sqlite/000110_*`、`000111_*` 全部存在（`ls` 实测）。
- 前端挂载：成员发现面板 `PluginsPanel` 挂载于 `apps/web/src/integrations/IntegrationsRoutePage.tsx:15`（import）与 `:217`（`pluginsSlot={<PluginsPanel client={client} />}`）——收尾评审时点"PluginsPanel 零挂载"的 finding 已在后续修复轮落地；管理面板 settings 分区注册于 `packages/views/src/settings/registry.ts:43`（key `'plugins'`）。
- `apps/web/src/integrations/route.ts:3` 的 `integrationTabs` 仍为 `['im','embed','api','cli','chrome','claw']`（不含 'plugins'）——成员插件面板经 `pluginsSlot` 通道而非 tab 项挂载（读码实证；与 round-3 R3-01 指出的 settings/integrations 双 'plugins' key 语义冲突相关，见 7.4）。

**工作树状态（`git status --porcelain` 实测）**：无已跟踪文件改动；**25 个未跟踪文件**——本报告 + 执行数据 JSON 转录件（`issue-106-execution-data.json`，本报告撰写会话落盘）+ 第三代整分支 OCR 三轮报告 + 任务级 OCR 报告 20 份（即第 4 章所列 OCR 文档均未提交；中间版报告被本版覆盖，旧文本未另行保存）。未跟踪文档若不提交，worktree 被清理即丢失（第 9 章决策项 #12）。

主 checkout：本报告未做任何改动（角色仅写 worktree 内交付文档）。

---

## 6. 测试与检查结果

### 6.1 三条基线门控（执行数据权威记录；本报告撰写会话未复跑，如实标注）

| 命令 | 结果（执行数据 `gates` 记录） | 本报告复跑 |
| --- | --- | --- |
| `pnpm run typecheck:shared` | exit 0 | **未复跑** |
| `pnpm run test:shared` | exit 0 | **未复跑** |
| `go test ./...` | exit 0 | **未复跑** |

执行数据 `gatesGreen=true`。

### 6.1.1 两套门控词汇的映射（读者答疑：`pnpm gates`"六道门"与"三条基线门控"的关系）

`pnpm gates`（`scripts/run-gates.mjs:33-43`，本报告读码实测）是一条串行流水线，**T20 执行时点（09-24）为六道**：`test:shared` → `typecheck:shared` → `test:web` → `typecheck:web` → `check:integrity` → `build:web`（首部另有 node ≥26 自检重执行逻辑，非独立门）；**R1 轮（09-26）后 GATES 数组首项新增 `node-gte26-selftest` 成七道**（R1-F17，`run-gates.mjs:33-36` 注释自述）。映射关系：

| 三条基线门控（执行数据 `gates` 字段） | 与 gates 流水线的关系 |
| --- | --- |
| `pnpm run typecheck:shared` | gates 流水线第 2 道（同命令） |
| `pnpm run test:shared` | gates 流水线第 1 道（同命令） |
| `go test ./...` | **不在 gates 流水线内**（gates 为 TS/前端侧门；Go 门独立运行） |

因此"三条门控全绿"的判定范围**不含 `test:web`**——`test:web` 在 T20 账本的 gates 运行中为 2333/2334（唯一失败为他人文件 flaky，见 6.1.2），该结果不进入也不影响执行数据 `gatesGreen=true` 的结论；第 1 章"三条门控全绿"的边界即本表。

### 6.1.2 历史佐证记录（账本/fix-plans，本报告未复现历史中间态）

- T20 账本（`tasks/T20.md` 测试证据表）：`go test ./internal/...` exit 0（**121 包** ok、0 FAIL——T20 执行时点、worktree 含未提交变更的树）；`go build ./...` exit 0；`pnpm gates`（node v26.4.0）六道门中五道 ✔、`test:web` **2333/2334**（唯一失败 `src/agents/agent-editor.test.tsx` 为他人文件并发 flaky，单文件复跑 49/49 pass）。
- R1/R2 轮完成记录（`fix-plans.md:1415-1448,1602-1640`）：go build/vet/gofmt、Go 六包测试、integration 真 PG、typecheck:web/shared 0 error、前端相关测试全绿（面板/api-client/views 定向）、craft:shared 113/113——**均未含全量 test:web**。

**`test:web` 全量的完整口径（统一三处表述）**：全量 `test:web` 在本分支**完整执行过且仅一次**——即 T20 账本 gates 运行（2333/2334，带 1 个他人文件 flaky 失败，**从未以全绿收场**）；其后各收尾评审轮均因 agent-editor 死循环/机器高负载（load 32）未再跑全量，以定向替代（agent-editor 单文件 49/49 等全绿）。即：'跑过一次、带 1 个 flaky 失败'与'评审轮从未完整跑过'并存，两句话均为事实。

**同指标双数字对账（不同时点/不同批运行，均为各自真实记录，非矛盾）**：

| 指标 | 数字 A | 数字 B | 对账 |
| --- | --- | --- | --- |
| T20 验收套件复跑时长 | 3.52s（T20 账本"最终干净复跑"，09-24） | 4.95s（收尾评审复跑，执行数据 T20 evidence 与评审核实 Ruling"[T20] GREEN 终态本评审复跑 PASS 4.95s"） | 两次独立运行的不同时长 |
| `go test ./internal/...` | 121 包 ok（T20 账本，T20 时点、worktree 现状树） | 129 ok + 19 no-test-files、另 2 超时包复跑 ok（收尾评审，干净 HEAD 树、更晚时点） | 不同时点、不同树（评审还把 application 两包在 -timeout 35m 下复跑 PASS）；包数差异为时点间测试文件增减与统计口径（no-test-files 单列与否）所致，两处各自真实 |

### 6.2 验收与测试证据（账本与执行数据记录）

- **T20 验收套件**：RED（骨架 Fatal）→ GREEN（十子测试全过，十行 `PLUGIN106-EVIDENCE` JSON + summary `10 boundaries verified (B1..B10)`）→ 最终干净复跑 PASS（3.52s）；一次性 docker postgres:17-alpine，密码经环境变量未落源码，测毕销毁（`tasks/T20.md`）。
- **各任务 TDD**：`taskSummary` 20 条 evidence 均含真实运行的 RED→GREEN 记录（如 T01 `TestFetchAndVerifyClassifiesNon2xxByFamily` 修复前 FAIL、T13 RED `tolerates an empty service_id` 用例 FAIL、T16 RED `TestAcceptUpgradeConcurrent` FAIL 等，证据摘录见执行数据）。
- **矩阵测试名核对**：追踪矩阵引用的全部 50 个测试函数名逐一 `grep -rl "func <Name>"` 核实存在（T20 账本"all verified"）。

### 6.3 OCR 与评审门控

| 门控 | 结果 | 来源 |
| --- | --- | --- |
| 整分支评审 | **通过**（`branchReviewApproved=true`） | 执行数据 |
| 任务级 OCR（20 任务干净区间） | **全部 clean**（`taskOcrRanges` 20/20 `clean=true`） | 执行数据 |
| 整分支 OCR | **未通过**（`ocrPass=false`；三代三轮后第 3 轮仍有 9 条有效阻塞发现） | 执行数据终裁 Ruling |

### 6.4 未验证 / 部分验证事项（收尾评审核实类 Ruling 汇总，如实标注）

以下为执行数据 rulings 中收尾评审对历史主张的核实结论，属**结构性未验证/部分验证**项，非本报告复跑结论：

1. **全量 `test:web` 仅完整执行过一次且未以全绿收场**（统一口径见 6.1.2：T20 账本 gates 2333/2334、唯一失败为他人文件 flaky 且单文件复跑 49/49 过）；各收尾评审轮均未再跑全量（agent-editor 死循环/机器高负载阻塞），以定向替代全部真跑（PluginsSettingsPanel、PluginsPanel、api-client plugins、views registry、SettingsPage、agent-editor 单文件 49/49 全绿）。
2. **生产 server 进程未启动验证**（fx DI 装配以 `go build` + container 包测试 + Provide 齐备静态闭合；`container.go:291/434/773` 等装配点读码实证；真实 server 全链未起——low finding 残余）。
3. **真实 Atlassian Jira / 真实浏览器 E2E 维持外部不可验证**：fake Jira 与 T04/T05 契约逐字对照一致、rulings.md R3 以官方文档裁定字段名；前端切片止于 jsdom 级面板测试，前端→后端实弹 round-trip 以 B1 后端真 HTTP 面 + 前端 stub 测试拼接覆盖。
4. **Mimosa 深度扫描无工具结论**（`scanner_enobufs` 多轮未完成完整扫描）；人工等价核查全净：SQL 全参数绑定（`internal/application/repository/plugin.go` 全部 `Where("tenant_id = ? AND …")`，无 Raw/Sprintf）、无凭据字面量（DSN 掩码核查）、SSRF host 校验前置（`fetcher.go:105-132`）。
5. **各提交时刻的 TDD RED 历史状态不可重放**（性质如此）；现状以 HEAD 全绿无反证（评审在干净 HEAD 树复核：plugins 包 ok、`go test ./internal/...` 129 ok + 2 超时包负载性复跑 ok、architectureguard PASS、examples 自测 ok）。
6. **测试在含未提交变更的树上运行的疑虑**：评审已改在 `git archive` 干净 HEAD 树（/tmp/issue106-head）隔离复验，全部关键验证通过。

---

## 7. 整分支评审与 OCR 结论及修复轮次

> **编号体系说明（fix-plans.md 物理文件内存在多套同名 R 编号，须按"轮系"区分）**：
> ① **范围/流程裁决 R1–R10**（执行数据规划阶段，第 8B 章）；
> ② **Ledger 裁决 LR1–LR4**（`rulings.md` 原文编号 R1–R4，加 L 前缀防冲突）；
> ③ **终评修复序列 R3/R4/R5 + 复核 R6**（`fix-plans.md:209-494`，09-23/24）；
> ④ **第一代整分支 OCR 修复轮 R5/R6**（`fix-plans.md:1-207`，09-24，与 ③ 同名不同轮）；
> ⑤ **范围缺口修复轮 R9/R10**（`fix-plans.md:854-1038`，09-24）——**转折点**：R9 记载"前序 F1/F7/F10 三轮均裁决『缺陷修复轮不可承载、remaining 如实上报』；本轮主流程指令明确要求『按优先级修复（先写计划再 TDD 实施）』，据此转入实施模式"，由此 17 个未实施任务（T03、T05–T20）全部续作；
> ⑥ **任务级 OCR 转交复核 R11 + OCR 第 1 轮修复 R12**（`fix-plans.md:1039-1234`，09-24）；
> ⑦ **第三代整分支 OCR 三轮 + 修复轮"R1 轮/R2 轮"**（round 报告文件现内容 + `fix-plans.md:1235-1650+`，09-26）——R1 轮/R2 轮与 ① 的 R1/R2 同名不同轮；
> ⑧ **任务级 OCR**：T01（多批增量）、T02/T04（各轮）等，终态 20/20 任务 clean（`taskOcrRanges`）。

### 7.1 总判定

| 门控 | 结果 | 说明 |
| --- | --- | --- |
| 整分支评审（branch review） | **通过** | 与 42 提交时点中间版的 `false` 相比，R9 转入实施、20 任务落地、历轮修复后翻转（执行数据 `branchReviewApproved=true`；中间历程见编号体系 ③④⑤） |
| 整分支 OCR | **未通过** | 三代三轮；第 3 轮 22 findings 未组织修复轮，9 条 high/medium 有效阻塞发现未解决（7.4） |

### 7.2 历程概要（09-23 → 09-26）

1. **09-23/24 第一波**（中间版报告详载）：T01/T02/T04 实施（42 提交时点）；终评 R3/R4/R5+复核 R6；第一代整分支 OCR 修复 R5/R6；两轮复审 `branchReviewApproved=false`（范围缺口 high）。
2. **09-24 R9/R10 转入实施**：主流程指令"按优先级修复（先写计划再 TDD 实施）"；R9 裁定"单轮不可闭合全部缺口，按断点优先级切纵向片"，承载 T06 安装闭环（第一断点），R10 同 finding 重复投递按优先级续切（`fix-plans.md:854-1038`）。
3. **09-24~26 续作完成**：T03、T05–T20 全部实施（账本 20 份齐、全记完成）；R11 任务级 OCR 转交 13 项复核（零新 diff）、R12 修复 20 项；各任务级 OCR 收敛至 20/20 clean。
4. **09-26 第三代整分支 OCR 三轮**（`--from 2bf3bde8c`，选取依据见第 5 章）：round-1 45 findings → **R1 轮 33 条有效发现全部修复、批次内无剩余项**（S1 scripts/G1 plugins/G2 MCP 服务/G3 示例/N1 api-client/N2+W1 面板，逐批 RED→GREEN 与全量回归证据，`fix-plans.md:1380-1448`）；round-2 35 findings → **R2 轮 22 条有效发现全部修复、批次内无剩余项**（A Go 安全并发/B 示例/C scripts/D PluginsPanel/E PluginsSettingsPanel，`fix-plans.md:1573-1640`）；round-3 22 findings → **无修复轮**（终裁见 7.4）。

**round-1/2 与 round-3 同样存在"有效性过滤"，差额下落说明（读者答疑）**：R1/R2 轮计划原文即自述"**输入：OCR R1 轮 33 条有效发现**"（`fix-plans.md:1238`）/"**输入：OCR R2 轮 22 条有效发现**"（`fix-plans.md:1453`）——修复轮前置原则为"处置前逐项读码复核"（`fix-plans.md:1240-1241`），原始 findings 先经有效性分流，只有**有效**项进修复批次（R1 轮标题另自述 F 编号"缺号跳过"，即 45 个编号中 12 个缺号未进批次；R2 轮同构，35-22=13）。历轮同类过滤有实例可考：终评 R4 的 F6.5 经 Atlassian 官方文档核实为**误报**（`fix-plans.md:310`）、R6 复核轮判"不可达跨文件误报"（`fix-plans.md:464`）、T01-R2-F5 部分误报记录（`fix-plans.md:617`）。**但 round-1 的 12 条与 round-2 的 13 条被过滤项的逐条清单与各自理由（误报裁否/重复/拒绝修复）在 fix-plans 与账本中均无明细记载——数据缺口，如实标注**；与 round-3 的区别仅在于 round-3 有终裁 Ruling 点名 15 个编号（7.4），前两轮的过滤结果只以"33/22 条有效"的总量形式留痕。
5. **终态**：门控三绿、评审通过、`ocrPass=false`。

### 7.3 OCR 轮次数据（执行数据 `ocrRounds`）

| 轮次 | findings | clean 标志 | 处置 |
| --- | --- | --- | --- |
| round-1（`issue-106-ocr-round-1.md`） | 45 | true | R1 轮 33 条**有效**发现全部修复（另 12 条经有效性过滤未进批次，逐条下落无明细记载，见 7.2 差额说明；09-26） |
| round-2（`issue-106-ocr-round-2.md`） | 35 | false → true（两条记录：修复前未净/修复后登记净） | R2 轮 22 条**有效**发现全部修复（另 13 条同上；09-26） |
| round-3（`issue-106-ocr-round-3.md`） | 22 | true（轮次已登记） | **无修复轮**；终裁 9 条有效阻塞发现未解决 |

注：`clean` 标志与 findings 数并存（round-1 clean=true 且 45 findings），其语义应为"该轮已完整登记/处置"，**不代表零发现**；最终门控判定以 `ocrPass=false` 与终裁 Ruling 为准（执行数据内部口径，如实呈现）。

### 7.4 未解决发现清单（终裁：9 个未解决有效阻塞发现）

round-3 报告 22 条发现的严重度分布（本报告对报告文件逐条 grep 实测）：**2 high + 7 medium + 13 low**。终裁 Ruling 点名 15 个 R3 编号；其中 **9 条 high/medium 即"9 个未解决有效阻塞发现"**（2+7=9，与终裁数字吻合——此对应关系为本报告按报告文件严重度标注的解读，终裁原文未逐条点名计数口径）：

| 编号（终裁点名） | 位置（终裁引用 / round-3 报告文件行定位） | 严重度 | 问题摘要（据 round-3 报告原文） |
| --- | --- | --- | --- |
| R3-01 | `packages/views/src/settings/registry.ts:43`（关联 `packages/views/src/integrations/settings-route.ts:13`）/ 报告 :3-23 | **bug·high** | settings 与 integrations 双 'plugins' 裸 key 冲突：`normalizeIntegrationSettingsSection` 把 `section=plugins` 归一化为 `integration-plugins` → 管理面板深链被劫持到成员只读发现 tab，侧边栏两个同名「插件」项指向不同分区 |
| R3-15 | `internal/application/service/plugin_install_service.go:1289-1305` / 报告 :279-293 | **security·high** | AcceptUpgrade 崩溃窗口 + 幂等短路导致"快照工具缺策略行"的永久 fail-open。**实际后果（round-3 报告原文）**：升级在"快照落库（7a）后、逐工具策略行写入（7c）完成前"被硬杀（SIGKILL/OOM/部署重启）→ 重试时幂等条件全部满足而零写入提前返回，7c 永不重跑；而运行时门（`approval/tool_policy.go` 的 `enabledToolsIndividually` 与 `MCPToolApprovalRepository.IsEnabled`）**对缺失策略行默认 enabled=true 且无插件感知**，`FilterToolsBySnapshot` 只校验快照成员资格不校验策略 → **新增写工具将在管理员从未启用的情况下可被发现并派发，直接违反验收边界 B5"新增写工具默认关闭"**；ConfirmInstallation 同理（step 5 后崩溃、重试被 409 挡回无法补齐）；更隐蔽的是治理视图对缺行显示 Enabled=ReadOnly（UI 呈现写工具"已禁用"）**与运行时 fail-open 实际行为相反，管理员在 UI 上看不出异常**。缺行默认启用的机制根源本报告已在 `internal/types/mcp.go:151-153` 读码实证（注释原文"Missing rows are treated as enabled for backwards compatibility"） |
| R3-03 | `internal/modules/plugins/fetcher.go:105-106` / 报告 :34-52 | security·medium | `invalid manifest URL: %w` 透传 `*url.Error`（Error 文本内嵌完整原始 URL）→ 畸形 URL 携带 userinfo 时凭据明文进入管理员可见 400 与日志，与相邻注释声明的纪律矛盾 |
| R3-08 | `apps/web/src/settings/PluginsSettingsPanel.tsx:207-228` / 报告 :121-148 | bug·medium | 陈旧闭包竞态残留窗口（epoch 守卫未覆盖的路径） |
| R3-09 | `apps/web/src/integrations/PluginsPanel.tsx:144-163` / 报告 :149-170 | bug·medium | 与 refreshInstallations 同款 epoch 竞态（loadConnection 路径） |
| R3-10 | `apps/web/src/settings/PluginsSettingsPanel.tsx:689-726、369-395` / 报告 :171-200 | bug·medium | 操作按钮冻结矩阵不对称，「检查升级」等并发治理动作存在竞态 |
| R3-16 | `internal/application/service/plugin_install_service.go:689-727` / 报告 :294-306 | bug·medium | SetInstallationState 未纳入 upgradeAcceptMutexes 序列化，与 AcceptUpgrade/ResolveDrift 并发竞态 |
| R3-18 | `apps/web/src/settings/PluginsSettingsPanel.tsx:440-461` / 报告 :316-332 | bug·medium | runDriftResolve 的 try 块把写操作与后续读取串联，getDrift 失败时误报并跳过快照失效 |
| R3-22 | `internal/application/service/mcp_service.go:641-668（GetMCPServiceResources）、401-476（TestMCPService）` / 报告 :382-404 | security·medium | plugin-managed 守卫系列漏掉资源读取面（Viewer+ 路由未拒插件物化服务的资源读取） |

终裁清单另点名 6 条 low（不计入"9 个"）：R3-02（`packages/views/src/integrations/registry.ts:27`，operations 声明与注释矛盾）、R3-04（`scripts/lib/node-gte26.mjs:27`，PATH 探测无超时）、R3-17（`apps/web/src/integrations/PluginsPanel.tsx:206`，兜底刷新缺卸载守卫）、R3-19（`scripts/run-gates.mjs:14`，注释引用未提交的本地文件）、R3-20/R3-21（`internal/modules/plugins/manifest.go:451-454、474-477`，userinfoOf/maskEndpointCredentials 只按 '/' 终止 authority，'?'/'#' 变体掩码失效）。round-3 报告其余 7 条 low（perf/security/doc/maint/style 类，集中在 `scripts/run-with-node-gte26.mjs` 与 PluginsPanel 样式）未进终裁点名清单（按位置匹配的推断，标注）。

**停止修复的原因与终裁来源（数据缺口，如实标注）**：第 3 轮后不再组织修复轮即终裁——**终裁由主流程发出**（执行数据终裁 Ruling 即其载体；fix-plans 由修复会话维护，最后一节为 R2 轮完成记录、其后无新批次计划），但**停止的具体动因（时间/预算/职权/规则触发）在执行数据、fix-plans、账本中均无记载**，本报告无法回答、只能如实标注；补记处置结论的责任主体建议为主流程（第 9 章决策项）。

**翻转 `ocrPass` 的操作路径（读者答疑）**：修复后重跑第 4 轮整分支 OCR 的**命令有成文模板**（`ocr-ledger.md:20` 原文）——`ocr review --from <maxCleanHead> --to <新HEAD> --output <报告路径>`（首跑完整形态见 `ocr-ledger.md:17`：含 `--audience agent --background "<#106 业务背景>"`；`--from` 取 `2bf3bde8c` 起或处置轮终点的最新 maxCleanHead）。**通过判定规则无成文门控**：先例口径是收尾轮的"0 findings，ranClean"（`ocr-ledger.md:14`），本报告保守建议为"全部 9 条阻塞级发现修复（或 22 条一并处置）后重跑至零有效 findings、至少零有效 high/medium"（建议，非门控规则）；重跑属 OCR 工具链/主流程职权，本报告未运行任何 OCR 命令（如实标注）。

### 7.5 修复轮次统计

基线..HEAD 149 提交中含历轮修复：终评 R3/R4/R5（③）、第一代 OCR R5/R6（④）、范围缺口 R9/R10（⑤）、R11/R12（⑥）、第三代 OCR R1 轮/R2 轮（⑦）、各任务级 OCR 修复（⑧）。R1/R2 轮各自"全部修复，无剩余项"并有逐批 RED→GREEN + 全量回归证据；任务级 20/20 clean。唯一未闭环：round-3（7.4）。

---

## 8. 全部 Ruling 及依据（含判断错误的代价）

> 本章收录执行数据 `rulings` 数组的全部条目与 Ledger `rulings.md` 的 LR1–LR4、T20 账本内 2 条任务 Ruling。"判断错误的代价"列为本报告的逻辑推演（非原文），用于说明裁决敏感性。核实类条目（约 85 条）按任务分组全录于 8F，每条一行。

### 8A. 环境与流程

| Ruling | 依据 | 判断错误的代价（推演） |
| --- | --- | --- |
| **模型路由**（执行数据首条/R1/原文照录共 3 次出现，合并呈现）：任务书要求按 gpt-5.6-sol/terra/luna 分级派发，本 host 仅配置 GLM-5.3 系列模型，动态工作流不支持按子代理选模型；实际全部子代理运行在会话模型 GLM-5.3（最高推理档）。记录替代方案，不声称使用了不可用模型。 | host 模型配置实测 + 工作流能力边界。 | 若虚报模型则交付记录失真（诚信问题）；若因此停工则无交付。记录替代方案使成本/质量归因可审计。 |
| **R2（#106 的 implement 语义）**：根 Spec Issue 的 implement=全部子 Issue 完成后的汇总验收（十条边界核销、覆盖矩阵复核、应用边界 seam 整体走查），不承载独立实现；拓扑序最后；父子不重复计算。 | 层级文档（#106 为根）+ Spec Testing Decisions。 | 当独立任务派发会与子 Issue 重复实现；漏掉则十条边界无人核销即宣称完成。 |
| **planUsable 裁决**：计划可派发。生效条件：conflict 1（US9 schema 展示粒度）在 T03 开工前、conflict 2（"本周" JQL 上界）在 T04 开工前裁决；W2 起派发遵守 conflict 3 调度约束（T09→T10 先于 T11/T14）。T01/T02/T05 无条件可立即开工。B1–B10、Out of Scope、实施前必须固定五项全部有承接且无占位；迁移下一编号（PG 000189/SQLite 000110）与代码基线事实实测属实。 | 逐项核对计划文本与仓库事实（`conflict-scan.md` §J 证据附录）。 | 计划不可派发而派发会在未裁决冲突处阻塞或产出冲突实现；W2 并行撞包产生编译竞态。 |
| **终裁（部分完成）**：OCR 三轮后仍有 9 个未解决有效阻塞发现——不满足最终完成标准，如实报告部分完成。 | round-3 报告 22 findings 未处置（9 条 high/medium）。 | 硬凑"全部完成"即伪造门控结论；如实部分完成使用户可基于真实状态决策。 |

### 8B. 范围与 DAG（R3–R10）

| Ruling | 依据 | 判断错误的代价（推演） |
| --- | --- | --- |
| **R3（#115 URL 字段修正）**：侦察员 JSON 中 #115 节点 url 误写 issues/116；只读 gh api 核对 #115=插件 08、#116=插件 09，按 number 修正采用 issues/115。 | 只读 gh api 实测。 | 按错 URL 派发会使插件 08/09 的依赖与验收错位。 |
| **R4（决定汇总）**：12 节点全部 open、全部属 Spec 范围、open-connector 5 个关键提交经 `git merge-base --is-ancestor` 逐个验证 5/5 in-baseline → 全部 implement；无 closed、无 out-of-scope（#107 Craft 树经层级文档核验不引用本树）、无 blocked。 | 层级文档 + git 实测。 | 误判 closed/out-of-scope 会漏实施；误判依赖会错序。 |
| **R5（边来源与无环）**：11 条业务依赖边全部来自 Issue 正文 blocked_by 并与 GitHub 时间线 blocked_by_added 事件核对一致；Kahn 算法验证 23 条边无环（12 节点全部出队），无需提取最小公共前置。 | 层级文档边图 + Kahn 实测。 | 有环未检出则波次调度死锁；边集不实则依赖顺序错。 |
| **R6（#115→#116 边的不确定性）**：#116 接口最小前置是 #110 验收 4 的已接受版本快照语义；#116 声明 blocked_by #115 解读为版本切换交付后"远端在已接受端点改变目录"的漂移场景才有完整验证路径。保持声明边不变，记录推断依据。 | Issue 正文声明 + 语义解读。 | 解读错误则 #116 在缺少升级前后对照条件下开工，漂移验收不完整。 |
| **R7（传递依赖不建边）**：#114 复用 #108 的清单抓取+远端工具发现 seam，经 #110 传递可达（#108→#110→#114），不新增直接边。 | DAG 传递可达性。 | 冗余边降低并行度；漏边导致并行踩踏。 |
| **R8（文件冲突调度约束）**：前端 `apps/web/src/settings`/`integrations` 为 7 个 Issue 共享改动面，后端插件模块为 4 个 Issue 共享面——同波并行须按 Issue 级波次（W0:#108+#109 … W7:#106）并考虑分工/串行；tdm-int 68 commits 未合入深耕同前端区域属未来 rebase 冲突面而非本 DAG 的边。 | 仓库文件面分析。 | 违反则同波并行互相踩踏；忽视 tdm-int 冲突面将付出高额解冲突成本。 |
| **R9（覆盖缺口处置）**：GAP-1…6 属 Spec Further Notes（spec.md:81）定位的"正式实施前需固定"工程前置项，不由现有子 Issue 摊派或改写验收（避免破坏纵向切片），统一移交实施计划作为 W0 前置任务承接，不留占位；GAP-1/2/3 应在 #110 开工前固定、GAP-5 在 #111 开工前裁决；如需独立跟踪再建议增补工程前置 Issue（无权在 GitHub 建）。 | spec.md:81 + 计划索引全局裁决表。 | 摊派破坏纵向切片；不承接则开工撞上未决前置。终态：六项全部落地（第 3 章附表）。 |
| **R10（边界 7 归属）**："时间范围由服务端限定"由 #109 承接——工具即"查询本人本周待办"且 schema 固定不接受任意 JQL/URL，时间范围由服务端工具语义锁定；#113 从对话端复核。与 spec.md:56 比对一致。 | spec.md:56 比对。 | 归属错误则该安全属性无验收承接。 |

### 8C. ADR / CONTEXT 核可

| Ruling | 依据 | 判断错误的代价（推演） |
| --- | --- | --- |
| **ADR-0013 核可**：GAP-5"不挂 ToolDispatchPreflight、走既有 approval.Gate"与 ADR 边界一致；实测 `mcp_tool.go:117-129` Execute 调用时重查 gate.IsEnabled、142-190 NeedsApproval→RequestAndWait，不信任预组装快照。 | ADR 文本 + 代码实测。 | 为插件接生产 MCP 到 Preflight seam 属超范围集成且违反 ADR 边界。 |
| **ADR-0001 核可**：计划未复用 open-connector 共享运行时 → Spec 57 行条件义务未触发；插件不获得其管理凭据，open-connector 域表不被插件域触碰。 | spec.md:57 + 计划核对。 | 误触 open-connector 域违反共享运行时 ADR、污染既有语义。 |
| **ADR-0014（pending）核可**：插件写工具审批复用既有 Gate 一次性决议与 fail-closed 语义（T19 以二次 Resolve 拒绝+单次派发+零写入验收，container.go:472 生产装配实证），不新建 pending 存储、不触碰 P2.4 seam。 | ADR + 代码实测。 | 新建 pending 存储违反 ADR-0014 pending 决议边界。 |
| **ADR-0014（commercial）核可**：与本需求无耦合，仅接口冻结/fail-closed 惯例参照。 | ADR 核对。 | 低风险，参照性裁决。 |
| **CONTEXT.md 物化裁决核可**：安装物化为一行 mcp_services 是执行基座复用（Spec 55 行明示方向），插件安装/版本快照/个人连接保持独立状态（plugin_installations 独立表），不构成"插件等同于 MCP 服务配置"的术语违规。 | CONTEXT.md 术语 + spec.md:55。 | 物化演变为术语塌缩即领域模型被静默重设计（应升级而非静默改）。 |

### 8D. Ledger 裁决（`rulings.md`，原文编号 R1–R4，本报告改称 LR1–LR4）

| Ledger 裁决 | 依据 | 判断错误的代价（原文/推演） |
| --- | --- | --- |
| **LR1 模型路由**＝8A 首行（同一 Ruling 的 Ledger 记录），不重复。 | — | — |
| **LR2 同意页钓鱼链路处置**：OCR 报告的钓鱼链路（开放动态注册+信任注册 redirect_uri+同意页文案固定→攻击者诱导成员提交 Jira 凭据兑换 30 天 refresh Bearer）属实且 T04 账本漏裁。裁决"保留开放注册+双层缓解"：同意页透明化（渲染 client_name 与跳转目的地 host，EscapeString）+ 可选白名单 `PLUGIN_ALLOWED_REDIRECT_HOSTS`（缺省空）；README 建议生产启用或置于专用 OAuth 提供方之后。 | RFC 7591 动态注册是示例演示契约；闭环注册需运营者身份体系，超出教学示例范围。 | 直接闭环注册破坏示例主流程；不缓解则成员凭据可被钓鱼——两层缓解把暴露面收窄到"名单外跳转被拒+同意页可辨识"。 |
| **LR3 Jira 字段名核实**：OCR 主张取 `_fields`；核对 Atlassian 官方 API 参考确认键名为 `fields`（无下划线），与 jira.go 现行映射一致——误报，不改代码；如集成实测分歧以实测为准重开。 | 官方文档 developer.atlassian.com。 | 按误报改代码会把正确映射改错。 |
| **LR4 插件卸载端点保留并收窄定位**（主 Agent 经升级机制裁定）：T06 承接提交中的 `DELETE /plugins/installations/:id` 与计划"本版不提供卸载/删除"明文相悖，属应升级裁决而非静默扩围。裁定保留端点、不回退，定位收窄为**运维自愈通道**（解决补偿事务失败残留的运维死锁）；依据：Spec Out of Scope 不含卸载、端点有 Admin 门控+硬级联清理+PASS 测试、移除将使已解决的运维缺陷回归；约束：前端不得出现任何卸载入口，用户可见治理终点仍是"停用"。 | Spec 文本比对 + 运维死锁分析。 | 原文明载：若用户不认可，revert 该端点部分即可单独摘除，与同轮修复无耦合。 |

### 8E. 任务内 Ruling（T20 账本，原样收录）

| Ruling | 依据 | 判断错误的代价（原文） |
| --- | --- | --- |
| **B2 失败保旧观察点**：远端已切 v2 后的成员目录会被漂移阻断（B3 语义），失败保旧的正确断言面是安装行零改动（version/endpoint/快照）。 | B2/B3 语义边界分析。 | 若错，B2 与 B3 的语义边界会重叠。 |
| **B9 迁移复述自建**：存量 harness 缺陷在他人所有权文件（migration_pg_integration_test.go 属 T06），按"缺陷回所属切片修复不在本切片绕过"与"不改他人文件"双约束，B9 在本切片所有权内以正确顺序复述同一迁移契约；既有缺陷记 concern 转交。 | 所有权约束。 | 若错，代价是复述与原测试存在覆盖面差异（复述只含手工行幸存+列删除断言，原测试还断言 up 形态与派生行清理——那部分由原测试承载）。 |

### 8F. 核实类 Ruling（收尾/集成评审对历史主张的核实结论，按任务分组全录；"已闭合"= 评审以真跑/读码核实了该主张）

**T01**（4 条）：TDD RED 历史状态——维持不可重放，现状干净 HEAD 树复核全绿无反证（已闭合·现状）；全仓 `go test ./...` exit 0（127 ok）——评审等价复核 build exit 0 + `go test ./internal/...` 129 ok+19 no-test-files（2 超时包负载性复跑 PASS）、architectureguard PASS、examples 自测 ok（已闭合）；T02/T04 产物跨任务运行时集成——`TestPlugin106Acceptance` B1/B7 在真 PG 真跑 PASS（已闭合）；各提交时刻测试运行证据——历史不可重放，现状全绿无反证（如实标注）。

**T02**（4 条）：T06 对预览消费方语义——B2 完整 confirm→accept 链真 PG PASS、预览服务测试在全量套件 ok（已闭合）；PG 迁移 000189 真实执行——真 PG 成功 apply 000189.up/000190.up 并执行 000190.down（已闭合）；测试在含未提交变更树上运行——评审改在 git archive 干净 HEAD 树执行（已闭合）；fx DI 真实 server 启动装配——**部分闭合**（build+Provide 齐备+container 包测试 ok；完整 server 进程未启动验证，low finding）。

**T04**（4 条）：T05/T11/T13 对行为契约的消费——examples 自测+plugintest OAuth 端点集+B7 全链真跑 PASS（已闭合）；真实 Atlassian REST v3——**维持不可本地验证**（无真实账号；LR3 以官方文档裁定，fake 一致）；T01 产物自身正确性——同一提交树被多测试族共同覆盖全绿（已闭合）；生产部署 PLUGIN_ALLOWED_REDIRECT_HOSTS 采纳——部署运维面不可从 diff 验证（代码与 README 在位）；oauth.go/ocr_fix_test.go 未提交状态——两文件均在 HEAD 提交树内（已闭合）。

**T05**（3 条）：TDD RED→GREEN 过程——历史不可重放，现状 GREEN（如实）；Client.Timeout 页级触发——回归钉子在 HEAD（cfg.Timeout=150ms vs sleep 400ms 断言页级先于整体 deadline，已闭合）；T13 复刻/#108 安装前核验依赖——B1/B7 真跑 PASS（已闭合）；提交时点运行时序——历史不可重放，现状一致（如实）。

**T06**（8 条）：真 PG 000190 up/down——**部分闭合**（up+down 契约经 T20 accMigrationDownKeepsManual 真 PG PASS；T06 专属 TestPluginInstallationsMigrationUpAndDown 仍 FAIL——harness 顺序缺陷立 finding，迁移文件本身无罪）；manifest_url 升级链消费——B2/B6/B10 直接 UPDATE manifest_url 驱动真 PG PASS（已闭合）；与 T07/T09/T17 联合集成——B3/B4/B8/B10 全链真跑 PASS（已闭合）；SQLite 外键级联——代码级闭合（000111.down 显式按 service_id 清理三派生表，测试 PASS；生产 DSN 不开 FK 前提已固化注释）；worktree 未提交变更归宿——超范围（评审以干净树隔离验证提交态）；PG 迁移集成测试 env 门控——docker PG 真跑确认缺陷真实存在而非门控问题（已闭合）；dig 容器运行时完整解析——同 T02 第 4 条（部分闭合）；停用后 Agent 侧不可见端到端——B6 disable 场景断言真 PG 真跑（已闭合）；T07/T09/T14/T16 接口消费兼容——全套件+B2 集成 PASS（已闭合）；测试在含未提交文件树上运行——干净树复跑全绿（已闭合）。

**T03**（4 条）：test:web 全量——**未跑全量**（agent-editor 死循环/高负载阻塞），定向替代全部真跑（PluginsPanel 11/11、PluginsSettingsPanel 29/29、api-client 34/34、views registry 5/5、SettingsPage 25/25、agent-editor 单文件 49/49）；面板与真实后端 POST preview 端到端——**部分闭合**（B1 后端真 HTTP 面 403/字段/受限地址；前端→后端实弹 round-trip 仍为 stub 测试）；R-T03-1/2 兑现——双解析器收敛已兑现（T08 删镜像，单一 plugins.ts 解析器 34/34）；client.plugins 挂载与页面接线——当期未兑现立 finding（**终态已被后续轮修复**：PluginsPanel 挂载于 IntegrationsRoutePage:15/217，本报告读码实证；R3-01 仍指出 key 语义冲突）；并行改动不影响 TS gates——定向 TS 测试全 PASS 与推断一致；全量 gates 未跑（如实）。

**T07**（3 条）：worktree go build 失败非 T07 造成——干净 HEAD 树 build exit 0、实现在 plugin_install_service.go:964（已闭合）；门控随 T06 同轮全量复跑——未复跑全量门控，以全量 Go internal+定向前端复核替代（如实）；gorm 真库行为——B4 真 PG ListInstallations/GetMyConnectionStatus 三态/撤销/过期隔离全 PASS（已闭合）；生产 RBAC g.Viewer() 链路——**部分闭合**（B1 用真实 RequireRole 中间件验证 Admin 面 403；Viewer+ 路由挂载静态核对 routes_plugins.go:69-77；真实 auth 中间件全链未起 server）。

**T09**（4 条）：dig pluginRepo 运行时解析——编译期+container 测试闭合，server 未启动；真实 DB 故障错误透传 fail-closed——单测级闭合（fake repo 故障→guard 失败→目录加载中止有断言 PASS；真实 GORM 故障注入未做，残余）；T10+ 应用边界集成——plugin_agent_integration_test+B3/B7/B8 真跑 PASS（已闭合）；未提交改动不触 ToolSchemaDigest——干净树复跑+已读 delta（已闭合）。

**T11**（3 条）：plugintest/server.go 与 T10 同文件冲突——增量共存于 HEAD 提交树，自测+全链消费 PASS（已闭合）；T06 harness 转交修复是否落地——**未落地**（两次复现 FAIL，立 finding；终态仍为 T20 Concerns 1）；模型路由 GLM-5.3——过程性事实不可从 diff 验证，记录留痕一致（如实）。

**T10**（4 条）：plugintest API 与 T13/T16–T19 消费一致——全消费同 API 且全绿（已闭合）；TDD RED 证据——历史不可重放，现状 GREEN；并行 T14 瞬态 3 FAIL——现状佐证 HEAD 全 integration 套件仅 1 已知失败（harness 缺陷）；Ruling 3 波次协调合理性——结果论闭合（两任务增量零冲突，server.go 钩子被 B1/B2 实际消费）。

**T14**（4 条）：T15/T16 对 PreviewUpgrade 契约消费——后端闭合（B2/B6 真跑）；T15 前端解析器在位 34/34；**acceptUpgrade 前端当期缺失立 finding（终态已被 R1 轮 N1 批次 F19 修复**：新增 acceptUpgrade+getDrift/checkDrift/resolveDrift 解析器与面板接入，`fix-plans.md:1402-1410`）；g.Admin() 真实鉴权——部分闭合（B1 以真实 RequireRole 验证同卫 preview 面 403；upgrade-preview 与 confirm/disable 同卫挂载静态核对；真实 server 全链未起）；pluginRepo/mcpRepo SQL 参数绑定——已闭合（全参数绑定、零拼接、无 Raw/Sprintf，grep 验证）；plugintest/newManagerLister 保真度——全链消费+验收套件 PASS（已闭合）。

**T08**（4 条）：前后端真机端到端——**维持部分**（后端真 HTTP 面 B1+前端 stub 测试；实弹 round-trip 当期不存在）；gates 低负载全量——未跑全量 pnpm gates（如实），定向组件级复跑全绿；T12 接线落地（i18n/route.ts 扩员/PluginsPanel 挂载）——**当期未落地立 finding（终态大部分已修复**：PluginsPanel 已挂载（本报告读码实证）、升级/漂移面板已接入（R1 轮 F19/F23）；R3-01 指出 settings/integrations key 冲突仍在、`route.ts:3` integrationTabs 仍无 'plugins' 项——以 pluginsSlot 通道挂载，语义冲突未解）；并行改动对 gates 影响——定向 TS 测试全绿与推断一致。

**T13**（4 条）：真实示例服务全链——**部分闭合**（示例服务自身自测 PASS+验收以同契约替身覆盖协议契约；「真实示例进程×生产装配」实弹端到端仍未运行，结构性残余）；needs_auth 文案生产 UI 呈现——当期前端无成员发现 UI（finding 关联；终态面板已挂载，needs_auth 引导断言在 B4 场景）；并行改动合流后全量门控——干净树 Go 全量+worktree 定向前端全绿（已闭合）；Basic(email:APIToken) 真实 Jira 等价性——维持外部不可验证，fake 与 T04/T05 契约逐字对照一致（如实）。

**T15**（4 条）：T14 后端 PreviewUpgrade 真实行为——B2/B6 驱动真跑（五维差异/降级标注/失败保旧/幂等，已闭合）；test:web 两文件卡死——agent-editor 单文件 49/49 真跑 PASS；GeneralPreferencesPanel 未跑（非 diff 文件，如实报告未运行）；pnpm gates 一键全量——未跑（如实），逐项定向替代全部真跑；RED 阶段证据——历史不可重放；真实浏览器/后端 E2E——维持未验证（切片止于 jsdom 级面板测试）。

**T12**（4 条）：前端与 connections/me 端到端——后端闭合（B4 三态/撤销/过期真跑）；前端当期无挂载点不存在真实端到端（finding 关联；终态已挂载）；历史 GREEN test:shared 记录——历史不可重放，现状 plugins.test.ts 复跑通过；全量 pnpm gates 单命令——未跑（如实）；GeneralPreferencesPanel 挂起根因归属——未验证（他人文件，未运行）。

**T16**（5 条）：T17/T18/T20 对 AcceptUpgrade 语义消费——B2 接受切换/B3 漂移重定基/B5 升级新增写工具默认关全部真跑 PASS（已闭合）；前端 acceptUpgrade 未实现/409 映射——**当期开放立 finding（终态已被 R1 轮 F19 修复**，api-client 新增 acceptUpgrade，面板接入）；upgrade-accept 端到端 HTTP 行为——部分闭合（路由+g.Admin()+dto binding+错误映射静态核对；service 直调真跑 B2；真实 server 中间件全链未起）；craft 域全包挂起基线——同向复现且归因印证（首轮默认 10m 超时为机器 load 32，-timeout 35m 复跑 PASS，负载性而非产品缺陷）；/tmp 干净树复验——同法复验全部关键验证通过。

**T17**（5 条）：生产容器必然注入 drift/upgrade writer——已闭合（编译期保证：gorm 实现三方法、构造签名按接口类型强制缺方法无法编译、container.go:291 已 Provide）；全 integration 套件既有基线失败——复现确认（真 PG 全套件唯一失败即该 harness 缺陷测试）；并行改动最终合流——干净树+全套件验证（已闭合）；Mimosa scanner_enobufs——无工具扫描结论（如实）；人工等价核查全净：SQL 全参数绑定、无凭据字面量、SSRF host 校验前置（fetcher.go:105-132）；US25 前端复审提示呈现——**部分闭合**（管理面漂移徽标 PluginsSettingsPanel.tsx:526-529+drift 三路由+B3 复审闭环后端全链；成员侧 UI 无入口——终态成员发现面板已挂载，复审入口仍以管理面为主）。

**T18**（5 条）：integration 基线失败同 T16/T17——复现确认（同一 harness 缺陷 finding）；T18 指名 6 测试在全套件 PASS；TDD RED 声明——历史不可重放，现状 GREEN；Mimosa enobufs——同上（人工等价核查净，无工具结论）；T19 requireApproval 消费/快照全工具策略行完备性——已闭合（B5 arm→审批卡（工具名+参数原文）→reject/timeout 零派发真跑；plugin_write_tools_integration_test 在 integration 套件 PASS）。

**T19**（6 条）：生产 container 装配与测试装配运行时等价——静态闭合（container.go:472 NewGate 与测试 newApprovalPGStack 同构；生产进程未起，low finding 关联）；routes_infra 生产 HTTP 全链——部分闭合（B1 以等价装配真跑；生产 server 全链未起）；T20 复用——已发生（TestPlugin106Acceptance 含 B5 审批全场景，复跑 PASS）；两 flaky 文件 CI 他轮表现——agent-editor 单文件 49/49（本评审）；CI 他轮不可验证（如实）；pnpm gates 纯净基线（stash 对照）——未做（stash 被禁），以干净树 Go+worktree 定向 TS 替代。

**T20**（4 条）：RED 轨迹——历史不可重放；GREEN 终态复跑 PASS 4.95s+十行 evidence 齐全；四条命令在 worktree 状态运行——已闭合（改在干净 HEAD 树复跑全部关键验证：acceptance/build/全量 internal/architectureguard/examples）；矩阵 B1–B9 前置任务测试正确性——已闭合（`go test ./internal/...` 129+2 ok、integration 套件仅 1 已知失败、验收套件 PASS，均为评审真跑）；B9 行注明的存量缺陷——已单独运行：FAIL 复现确认（relation mcp_services does not exist）；accMigrationDownKeepsManual 复述 PASS——缺陷属实且仍在（finding）。

**整分支终态**（1 条）：OCR 三轮后仍有 9 个未解决有效阻塞发现——不满足最终完成标准，如实报告部分完成（= 8A 终裁行）。

---

## 9. 遗留风险 / 延期 / 需用户决策事项

### 未完成工作与遗留风险

1. **整分支 OCR 未过（唯一未过门控）**：round-3 共 22 findings 全部未处置，其中 9 条有效阻塞（2 high：R3-01 深链劫持、R3-15 fail-open 崩溃窗口；7 medium：R3-03/08/09/10/16/18/22，7.4 表）+ 13 条 low（终裁点名 6 条 + 未点名 7 条）。第 3 轮后未再组织修复轮，原因无记载（数据缺口，7.4 尾节）。**翻转 `ocrPass` 需修复（至少全部 9 条阻塞级）后按 7.4 成文模板重跑第 4 轮整分支 OCR**。两条 high 分别落在前端分区路由与后端升级事务完整性，建议最优先。**工作量：未评估**（数据缺口，本报告不做虚构估算）；客观可得的信息：9 条阻塞级发现分布于 7 个文件（前端 tsx 5 条——`registry.ts`/`settings-route.ts`/`PluginsSettingsPanel.tsx`×2/`PluginsPanel.tsx`；后端 go 4 条——`fetcher.go`/`plugin_install_service.go`×2/`mcp_service.go`），round-3 报告对每条均附具体修复建议 diff（修复方向已明，非从零分析）。
2. **存量 harness 缺陷未修**：`TestPluginInstallationsMigrationUpAndDown` 顺序颠倒（属 T06 所有权文件），任何真 PG 运行必失败；B9 已由 `accMigrationDownKeepsManual` 复述承接，但缺陷本身仍在全 integration 套件中表现为唯一失败（`tasks/T20.md` Concerns 1）。
3. **结构性未验证项**（6.4 全列）：生产 server 进程未启动验证（fx DI 仅静态+container 测试闭合）；真实 Atlassian Jira 与真实浏览器 E2E 外部不可验证（fake/官方文档/jsdom 级替代）；全量 test:web 仅完整跑过一次且带 1 个他人文件 flaky 失败、从未以全绿收场（统一口径见 6.1.2）；Mimosa 完整深度扫描无工具结论（enobufs，人工等价核查净）。
4. **前端语义冲突残留**：settings 与 integrations 双 'plugins' key 的归一化冲突（R3-01）未解；`route.ts:3` integrationTabs 无 'plugins' 项（成员面板经 pluginsSlot 挂载，两条通道并存正是 R3-01 语境）。
5. **OCR 台账未回写**：`ocr-ledger.json` 仍停留在 09-23 22:16 时点（`taskRanges=[]`、`maxCleanHead=2bf3bde8c`），与执行数据 `taskOcrRanges`（20/20 clean）不一致——台账物理文件未随终态更新；恢复会话按台账重建会误判。台账的 clean 结论只覆盖基线..`2bf3bde8c` 区间（且仅限该轮被选片审查的 6 个代码文件，17 个 docs 与 4 个测试文件被选片策略排除——精确语义见第 5 章 `2bf3bde8c` 节与 `ocr-ledger.md:18-19`），其后全部轮次未回写；`ocr-ledger.md` 自述"本会话（OCR 执行员）只亲历 T01 两轮与收尾一轮；其余任务的 OCR 非本会话执行，未记入本表"——台账的任务级表本就不完整。
6. **执行数据双口径**：`issueStatus`（规划时点"未实施"快照）与 `taskSummary`（终态全 done）并存——排期应以 taskSummary + git 标签 + 账本为准（第 1 章一致性说明）。
7. **25 个未跟踪文档未提交**（本报告 + 执行数据转录件 + 23 份 OCR 报告）：不提交则 worktree 清理即丢失。
8. **tdm-int 68 commits 未合入**且深耕同前端区域（裁决 R8）：未来 rebase 冲突面。
9. **基线非本分支失败项**：agent-editor.test.tsx 并发 flaky（他人文件，单文件 49/49 过，建议低负载重跑或排查超时预算）；node v22 的 test:shared false-red 已由 node-gte26 包装器处理（R1/R2 轮 scripts 批次加固）。

### 需用户决策

10. **是否处置 round-3 的 22 条发现并重跑第 4 轮整分支 OCR**（至少 9 条阻塞级；各报告附具体修复建议 diff，工作量未评估；重跑命令模板见 7.4 尾节）——翻转 `ocrPass` 的唯一路径。
11. **分支去向**：本地保留继续修复，还是推送/建 PR/合并（本流程禁止 push/merge；当前未推送，`upstream=[]` 实测）。
12. **是否提交 25 个未跟踪文档**（含本报告、执行数据转录件与全部 OCR 报告）。
13. **是否修复存量 harness 缺陷**（#2）与 agent-editor flaky（#9）——均为他人所有权文件，需授权会话处理。
14. **是否回写 OCR 台账**（`ocr-ledger.json` 的 taskRanges/maxCleanHead）——需主流程或授权会话执行，本报告无职权。
15. **GAP 前置项是否增补独立工程前置 Issue**（裁决 R9 建议；本流程无权在 GitHub 建 Issue）——终态六项已全部落地（第 3 章附表），此项已成事后追认性质。

**推荐顺序（本报告建议，非决定）**：① 先修 round-3 的 2 条 high（R3-01/R3-15）与 7 条 medium，处置其余 low 后按 7.4 模板重跑第 4 轮整分支 OCR 至零有效 findings；② 同步授权修复 harness 缺陷与 agent-editor flaky，使全量门控可在低负载完整跑通；③ OCR 翻转后回写台账、提交未跟踪文档，再决策推送/PR。理由：OCR 门控不翻转，任何"完成"表述都只能维持"部分完成"，且未处置发现会随后续工作叠加（每条已附修复方向 diff，但工作量未评估，排期前应先做一轮评估）。

---

## 10. B+A 终局（2026-09-26 晚增补）

> 本章为终局增补章节，由交付整理员会话于 2026-09-26 晚在上版报告（第 1–9 章 + 附 A/B，时点 HEAD `28c46776`、149 提交）基础上追加，**最新终局结论以本章为准**。事实来源：① 主流程任务书内联给定的裁决材料（B+A 裁决内容、9 项处置 JSON、接受清单披露要求与诚实红线）；② worktree 内 git 实测（命令列于 10.9）；③ 仓库内权威记录 `.superpowers/sdd/2026-09-23-issue-106/fix-plans.md:1613-2076`（B+A 批次 + 终局 F/F2 修复轮）与终局 OCR 三份报告文件（本章实测读取）。本章所有"完成/修复"仍指本地 worktree 内状态。

### 10.1 用户裁决

上版结论为"部分完成：`ocrPass=false`，第三代第 3 轮 9 条有效阻塞发现未修复"（第 1 章、7.4）。用户此后作出 **B+A 裁决**。仓库内锚点为 `fix-plans.md:1615` 原文："**用户裁决 B+A：修复经 OCR 多轮裁决确认有效的 9 个后端发现**"。按裁决的实际执行内容（git 与 fix-plans 实证）：

- **B（定向安全修复）**：处置经 OCR 多轮裁决确认有效的 9 个后端安全/正确性发现——4 项本轮 RED→GREEN 修复（#1/#2/#4/#5），5 项经逐项核实已被此前 R1/R2 轮修复（#3/#6/#7/#8/#9，记录证据、无需改码），见 10.2。
- **A（终局收口）**：组织终局 OCR 轮次，每轮之后修复经逐项复核确认有效的发现（第 1 轮 5 项、第 2 轮 4 项，共 9 项全部 RED→GREEN 修复）；终局第 3 轮后的剩余发现**不再组织修复轮，由用户裁决接受、随 diff review 处理**（10.3/10.4）。

如实标注（终局判定的两个关键输入均无法在仓库内复核）：① **A/B 两个选项的原始逐字文本未落盘**（任务书与仓库均无），本章按裁决的已执行内容呈现；B+A 9 项与上版 9 条阻塞的精确集合关系见 10.2.1 映射表；② **任务书本身不在仓库**（主流程内联载体，无转录件——对照第 4 章先例：执行数据 JSON 有落盘转录件，任务书无），10.7 所引完成标准"ocrPass=true 且安全修复 fixed"系任务书转述，仓库内无独立文本可核对。"接受、随 diff review 处理"系任务书给定的披露口径。

### 10.2 定向安全修复 9 项的处置与提交

9 项处置逐条记录于 `fix-plans.md:1613-1712`（B+A 裁决批次节）：

| # | 发现（摘要） | 处置 | 证据（fix-plans 行号） |
| --- | --- | --- | --- |
| #1 | AcceptUpgrade/ResolveDrift 幂等短路崩溃窗口 fail-open（快照落库后、策略行写完前被硬杀 → 重试命中短路零写入放行，缺策略行写工具按"缺行=启用"默认启用，触碰 B5 边界；ConfirmInstallation 同族） | **本轮修复**：新增 installationPolicyRowsComplete，短路前校验每工具显式策略行、缺失落入增量补齐；ConfirmInstallation 新增 completeCrashedConfirm 反查自愈 | :1617-1640（三组 RED→GREEN 新测） |
| #2 | SetInstallationState 缺 per-installation 锁（syncService 整行 Update 与接受路径 7b 交错可回滚端点/OAuth 基线） | **本轮修复**：入口 `defer lockUpgradeAccept(installationID)()` | :1642-1654 |
| #3 | UninstallInstallation 缺锁 | **已修复核**（R2 F20）：入口持锁注释锚 + TestUninstallConcurrentAcceptSerializes 本轮复跑绿，无需改码 | :1656-1660 |
| #4 | GetMCPServiceResources（Viewer+）/TestMCPService（Admin+）缺插件物化守卫（实时远端绕过已接受快照暴露未接受能力；handler resources 落 500、test 被 200 包装） | **本轮修复**：两方法补 `PluginInstallationID != nil → ErrPluginManagedService`（与 R1 F07 同口径）+ handler 两面 409 | :1662-1674 |
| #5 | 掩码函数族 authority 只按 '/' 截断（query 内 '@' 被当 userinfo，掩码吞 host+query、userinfoOf 误报） | **本轮修复**：新增 authorityEnd 按 `/?#` 终止（RFC 3986 §3.2）+ 协议相对前导 `//` 保留 | :1676-1690 |
| #6 | 升级候选端点 512 校验 | **已修复核**（R1 F45）：validatePluginURLLength 在册 + 测试在册 | :1692-1696 |
| #7 | drift 明细四列表 `append(nil)` → JSON null | **已修复核**（R1 F09）：make+copy + 测试复跑绿 | :1698-1702 |
| #8 | jira.go pageToken→nextPageToken | **已修复核**（R2 F18）：payload 键与替身收发一致 | :1704-1708 |
| #9 | pluginManagedConflict 插在 swaggo 注释块中间 | **已修复核**（R1 F08）：位置已前移 + 测试复跑绿 | :1710-1712 |

**提交（本章 git log 实测，均在分支上，提交信息节选）**：

| 提交 | 全 SHA | 内容 |
| --- | --- | --- |
| `318de31a7` | `318de31a7fba152a76caa8b5ad7f77e63db4f2ae` | fix(plugins): [安全修复] B+A#5 掩码函数族 authority 终止符对齐 RFC 3986 |
| `25a21de58` | `25a21de58ffa1d71e31e6de45608b3f045d92cbd` | fix(mcp): [安全修复] B+A#4 GetMCPServiceResources/TestMCPService 补插件物化守卫 |
| `28fcaa96b` | `28fcaa96b8389fe4466da1dbf0dcbf0753cff033` | fix(plugins): [安全修复] B+A#2 SetInstallationState 入口补 lockUpgradeAccept |
| `a9d0fd0ce` | `a9d0fd0ce1819fc57d45516036da5f6ddf5b2145` | fix(plugins): [安全修复] B+A#1 幂等短路崩溃窗口 fail-open 收口 |
| `891f351d9` | `891f351d9d989098266ce0b6e7dd330dfa40c415` | style(plugins): [安全修复] B+A#5 补遗——manifest_test.go 新注释块 gofmt 规范化 |
| `ab4d9c75e` | `ab4d9c75ebe6e5ba5d5a8d216d789908cefbad78` | docs(sdd): B+A 裁决批次完成记录（9 项处置 + 全量门控真实退出码回填） |

#### 10.2.1 两个"9"的集合映射与遴选口径（上版 9 条阻塞 ≠ B+A 9 项，交叠仅 3 条）

上版第 7.4 章的"9 个未解决有效阻塞发现"（round-3 的 2 high + 7 medium）与本节 B+A 的"9 个后端发现"**不是同一集合**。完整映射（本章逐条核对 7.4 表与 10.2 表建立）：

| 上版阻塞 | 位置/性质 | B+A 是否承接 | 终局下落 |
| --- | --- | --- | --- |
| R3-01 bug·high | `packages/views/src/settings/registry.ts`（**前端 TS**） | 否（后端圈定外） | **已修复**——终局 F2 轮 f27（归一化劫持面；双入口语境仍在，10.8.2） |
| R3-03 security·medium | `internal/modules/plugins/fetcher.go`（**后端 Go**） | **否（未入，缘由无落盘）** | **未修复**——终局第 3 轮以同型 security·low 落入接受清单（10.3） |
| R3-08 bug·medium | `PluginsSettingsPanel.tsx`（前端） | 否 | 未修复——终局第 3 轮同族（升级在途按钮冻结矩阵，maint·low）落入接受清单 |
| R3-09 bug·medium | `PluginsPanel.tsx`（前端） | 否 | 未修复——终局第 3 轮同族（authorizeConnection 轮询终止条件，perf·medium）落入接受清单 |
| R3-10 bug·medium | `PluginsSettingsPanel.tsx`（前端） | 否 | 未修复——终局第 3 轮同型（maint·low）落入接受清单 |
| R3-15 security·high | `plugin_install_service.go`（后端） | **是 → #1**（扩展至 ResolveDrift/ConfirmInstallation 同族） | 已修复（10.2） |
| R3-16 bug·medium | `plugin_install_service.go`（后端） | **是 → #2** | 已修复（10.2） |
| R3-18 bug·medium | `PluginsSettingsPanel.tsx`（前端） | 否 | 未修复——终局第 3 轮同型（bug·medium）落入接受清单 |
| R3-22 security·medium | `mcp_service.go`（后端） | **是 → #4**（扩展至 TestMCPService 面 + handler 409） | 已修复（10.2） |

B+A 9 项的来源构成（与上表对账）：上版阻塞 3 条（R3-15/16/22）+ 上版终裁点名的 low 2 条合并为 1 项（R3-20/21 掩码 → #5；7.4 :275 明言 low 不计入"9 个"）+ 更早 R1/R2 轮已修复项的复核 5 条（#3=R2-F20、#6=R1-F45、#7=R1-F09、#8=R2-F18、#9=R1-F08）= 9 项。

**遴选依据与圈定主体（数据缺口，如实标注）**：仓库内唯一落盘依据是 `fix-plans.md:1615` 一句——圈定词仅"经 OCR 多轮裁决确认有效"+"后端"两个。**R3-03（后端、security·medium、凭据明文进管理员可见 400 与日志）为何不入 B+A、9 项清单由主流程按用户裁决拆解还是用户逐项点名，在 fix-plans B+A 节与任务书内联材料中均无记载**——本章不做推测，仅呈现事实：B+A 圈定的"后端"排除了全部前端项（R3-01/08/09/10/18），但同为后端的 R3-03 亦未入。

**任务书内联处置 JSON（原样收录）**：

```json
{"commits":["318de31a7","891f351d9","25a21de58","28fcaa96b","a9d0fd0ce","ab4d9c75e"],"testEvidence":"全部命令在 worktree /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue106 实跑：\n【RED→GREEN 逐项】#5 go test ./internal/modules/plugins/ -run TestUserinfoAndMaskTerminateAuthorityAtQueryAndFragment（RED: 期望原样返回实际 \"https://REDACTED@evil.example\" → GREEN ok）；#4 go test ./internal/application/service/ -run 'PluginManagedRow' + ./internal/handler/ -run 'PluginManaged'（RED: resources 500/test 200 包装 → GREEN ok）；#2 go test ./internal/modules/plugins/ -run 'TestSetInstallationState|TestUninstallConcurrentAccept⏦（任务书内联文本在此处截断）","remaining":["#1 ConfirmInstallation 崩溃窗口的「物化前被杀」子形态（安装行在、物化服务行不存在）保持 409 already-installed：该形态无暴露面（无服务行=运行时不供给工具，fail-closed），自愈路径为卸载+重装；completeCrashedConfirm 中的反查锚自愈仅覆盖「物化后、绑定前被杀」子形态（代码内已注明）","-tags integration 真 PG 五测未在本轮重跑（ask 门控清单未列；上轮 R2 记录为绿，本轮改动均为应用层逻辑、未触碰迁移/SQL）","Mimosa hook 在各次 git commit 前报 scanner_enobufs（其扫描器自身缓冲不足，非代码问题）；按其兼容策略继续提交，未宣称项目整体安全"]}
```

**截断处理（如实标注）**：任务书内联 JSON 的 `testEvidence` 字段存在截断（在 `#2 …TestUninstallConcurrentAccept` 处中断，上方以 ⏦ 标记）。本章不虚构截断尾文，按仓库内权威记录补全其内容：#2 的 RED→GREEN 为 `TestSetInstallationStateConcurrentAcceptSerializes`（RED 实测"接受在途期间状态切换零进展"不成立 → GREEN 后 accept 7b 中途并发切换全程阻塞、终态 URL=v2/Enabled=false，`fix-plans.md:1648-1652`）；#3 的已修复核证据为 `TestUninstallConcurrentAcceptSerializes` 本轮复跑仍绿（:1656-1658）；#1 的三组 RED→GREEN 为 `TestAcceptUpgradeIdempotentShortCircuitCompletesMissingPolicyRows` / `TestResolveDriftIdempotentShortCircuitCompletesMissingPolicyRows`（RED：重试成功但 create_issue 行仍缺）/ `TestConfirmInstallationHealsCrashedPolicyRows` + `TestConfirmInstallationHealsCrashedServiceBinding`（RED：重试吃 ErrPluginAlreadyInstalled）（:1636-1640）；全量门控真实退出码回填（:1713-1723）：`go test ./...` exit 0（129 包 ok、0 FAIL）、`pnpm run test:shared` exit 0（1023 pass/0 fail/1 skipped）、`pnpm run typecheck:shared` exit 0（0 error）。`remaining` 三项与仓库状态核对一致（fix-plans B+A 节未列 integration 重跑记录 ✓；Mimosa enobufs 沿袭 6.4 第 4 条 ✓）。

### 10.3 接受清单（显式披露：**用户裁决接受、随 diff review 处理**，非已修复、非已关闭）

| 清单项 | 数量与构成（本章对报告文件实测清点） | 来源 |
| --- | --- | --- |
| 终局第 3 轮（final-round-3）**全部 30 条** findings | **0 high、4 medium、26 low**。4 medium：PluginsPanel 首挂载假空态（bug）、runDriftResolve try 块串联（bug，与上版 R3-18 同型）、authorizeConnection 轮询终止条件不完整（performance，R3-09 同族）、win32 symlinkSync EPERM 全量复制回退（performance）。26 low：bug 5 / maintainability 12（含升级在途按钮冻结矩阵——R3-10 同型、i18n 直译兜底仅 zh/en 等）/ security 3（含 fetcher 侧 url.Parse 失败分支 `%w` 透传 `*url.Error`——R3-03 同型）/ style 3（嵌套三元族）/ documentation 3 | `issue-106-ocr-final-round-3.md` 逐条 grep 清点 |
| 终局第 1 轮 19 条中未进修复批次的 **14 条** | F 轮输入自述"5 条有效发现（1 high + 4 medium）"（`fix-plans.md:1730`）；其余 14 条逐条下落无明细记载——与第一代 round-1/2 的同类数据缺口一致（7.2） | `fix-plans.md:1728-1731` |
| 终局第 2 轮 27 条中未进修复批次的 **23 条** | F2 轮输入自述"4 条有效发现"（`fix-plans.md:1950`），其中 f14/f27 系对报告 bug·high 标注的严重性下调论证（"按 medium 处置"，:1952-1956）；其余 23 条逐条下落无明细记载（同上数据缺口） | `fix-plans.md:1948-1956` |

**接受理由（可从落盘记录确证的部分）**：终局两轮修复后剩余发现中**已无 high**（第 3 轮 0 high——口径与漂移风险见下节）；历轮确认有效的 1 条 high（F09 metadata 守卫）与 8 条 medium（F10/F13/F11/F02 + f14/f27/f01+f02）已全部 RED→GREEN 修复闭环（10.4）；剩余 4 条 medium 集中于既有面板竞态边界与脚本性能，low 以维护性/样式/文档为主。更细的逐条接受理由未见落盘记载（除上述严重度事实与 fix-plans 严重性下调论证外），按裁决口径披露。上版 9 条阻塞的终局下落见 **10.2.1 映射表**：4 条已修复（R3-15/R3-16/R3-22 经 B+A、R3-01 经终局 f27——f27 同时计入终局修复 9 项），**其余 5 条未修复落入本清单，其中 R3-03 为后端项**（`internal/modules/plugins/fetcher.go`，security·medium：`%w` 透传 `*url.Error` 使凭据明文进管理员可见 400 与日志，:267），另 4 条为前端（R3-08/09/10/18）——均在终局第 3 轮以同位置/同型发现复现。

**"0 high"口径与严重度漂移（如实披露）**：① 第 3 轮"0 high"是该轮 OCR 报告的**原始标注，未经独立复核**；② 同型发现跨轮严重度漂移无统一口径——R3-03 同型由第一代 security·medium 漂移为终局第 3 轮 security·low（两轮各自标注，无重评记录落盘）；③ 终局第 2 轮 2 条 bug·high 经修复会话重评按 medium 处置后修复：f14 的下调论证已落盘（"报告的 fail-open 链不成立：7c 跳过既有行是『existing rows keep the admin's verdicts』显式设计语义；孤儿行论据与生产 DSN `_foreign_keys=on` 相悖"，`fix-plans.md:1960-1964`），f27 仅有集体自述"严重性下调论证与代码实证一致"（:1952-1956）、单独论证未摘述。据此，"0 high"在多大程度上是重评而非修复的结果，本章呈现事实、不替读者下结论。

**"随 diff review 处理"的操作定义（数据缺口，如实标注）**：该处置路径的责任人、触发条件、跟踪载体（PR review 清单 / issue / 其他）在任务书与仓库均无落盘定义。且分支未推送（`upstream` 空，10.6 实测）、无 PR——**该 diff review 当前没有发生场所**，只能在用户决策推送/建 PR（第 9 章决策项 #11，终局状态见 10.7.1）之后进行。在此之前，本清单各项处于"已接受、未处置、无处跟踪"状态。

### 10.4 终局 OCR 轮次与结论

| 轮次（报告 mtime） | findings / 选片 | 严重度分布（报告实测） | 修复轮（提交实测） | 处置 |
| --- | --- | --- | --- | --- |
| 终局第 1 轮（09-26 13:03） | 19 / 44 items | 1 high（security）+ 6 medium + 12 low | **F 轮**：5 条有效（F09 high + F10/F13/F11/F02 medium）全部修复——`f2f5ba297`/`d7bff114f`/`d637647f2` + docs `b5d6865a8` | 批次内无剩余项（`fix-plans.md:1884-1944`） |
| 终局第 2 轮（09-26 17:00） | 27 / 48 items | 2 high（bug）+ 3 medium + 22 low | **F2 轮**：4 条有效（f14/f27/f01+f02，按 medium 处置）全部修复——`3ad99659c`/`d01ff28e1`/`beaf3f2f9` + docs `4de4e4258` | 批次内无剩余项（`fix-plans.md:2038-2076`） |
| 终局第 3 轮（09-26 19:43） | **30** / 52 items | **0 high、4 medium、26 low** | **无修复轮** | 终裁收口：**用户裁决接受、随 diff review 处理**（10.3） |

**F/F2 轮回归证据（`fix-plans.md:1884-1944` / `:2038-2076`，本章摘录；对照 7.5 对第一代 R1/R2 轮的证据形态）**：

- **F 轮**：Go 四包全量 ok（service 511s/plugins 11s/handler 4s/tools 31s）；`go build ./...` exit 0；改动 9 个 Go 文件 gofmt 无输出、四包 vet 干净；typecheck:web exit 0；web 全量套 `pnpm --filter @weknora/web test` exit 1——三个失败文件经 stash 基线对照全部实证固有（基线同样失败、依赖图零命中本轮改动），本轮相关面板测试全绿；Mimosa normal 深度重扫 completed（seal sha256 记录），聚焦本轮 8 个改动源文件零命中。
- **F2 轮**：Go 三包全量 ok（plugins 12.8s/service 331.7s/handler 2.9s）；`go build ./...` exit 0（仅既有 ld 告警）；改动 Go 文件 gofmt/vet 干净；test:shared 全量 1026 测试 1025 pass/0 fail/1 skipped；typecheck:web 与 typecheck:shared 均 exit 0。
- **未覆盖面（如实）**：两轮记录均无 `-tags integration` 真 PG 与 T20 验收套件复跑记载（10.7.2）；B+A 轮门控见 10.2 JSON 补全段（go test ./... / test:shared / typecheck:shared 三条，`fix-plans.md:1713-1723`）。

- **报告路径（真实文件，本章 `ls` 实测存在、git 未跟踪）**：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue106/docs/plans/issue-106-ocr-final-round-1.md`、`issue-106-ocr-final-round-2.md`、`issue-106-ocr-final-round-3.md`（注：任务书以 glob `issue-106-final-round-*.md` 指称，实际文件名多 `ocr-` 段，本章按真实文件名记录）。
- **结论：`ocrPass=false`。** 终局三轮（19→27→30 findings）后第 3 轮仍有 30 条 findings 未清零（0 high/4 medium/26 low），未达"零有效 findings"的先例口径（7.4 尾节）；用户裁决到此收口，剩余发现接受、随 diff review 处理。
- **"selected item(s)/选片"定义（全文首次出现于此，补定义）**：OCR 报告汇总行 `across N selected item(s)` 中的 item = OCR 工具内置选片策略从区间改动文件中选中进入审查的单元（文件）。第一代收尾轮的选片先例语义：27 个改动文件中 17 个 `docs/plans/*.md`（unsupported_ext）与 4 个 `*_test.go`（default_path）被内置策略排除、6 个代码文件入选（`ocr-ledger.md:18-19`）。**终局三轮 44/48/52 items 的逐文件选片清单未随报告落盘**（报告仅含汇总行与 findings 正文），其具体构成本章无法核验。
- **findings 逐轮上升（19→27→30）的构成**：选片数递增（44→48→52）与 F/F2 修复轮改动文件入区一致，属增量扫描口径（沿 `--from maxCleanHead` 语义，10.5）。第 3 轮 30 条中新增 vs 同型复现的精确占比**无统计**（完整逐条比对未做）；已核实的同型复现实例：R3-03 同型（fetcher 侧 `%w` 透传，security·low）、R3-09 同族（PluginsPanel 轮询终止，perf·medium）、R3-10 同型（按钮冻结矩阵，maint·low）、R3-18 同型（driftResolve try 串联，bug·medium）——即上版 5 条未修复阻塞在第 3 轮全部复现（10.2.1），其余为新增文件/维度发现。
- 如实标注：三轮的 `--from/--to` 区间与实跑命令未在报告文件或台账落盘（报告仅含 findings 正文与汇总行），本章按时间线（报告 mtime 与提交时刻交错：round-1 → F 轮 15:44-15:53 → round-2 → F2 轮 17:36-17:40 → round-3 19:43）呈现轮次顺序。

### 10.5 台账推进

- **推进值：maxCleanHead → `4de4e4258`**（终局第 3 轮扫描覆盖至 HEAD；未来增量重跑 `--from 4de4e4258`，沿 `ocr-ledger.md:20` 模板语义。注：任务书以 11 位缩写 `4de4e42586` 指称同一提交，本章正文统一用 10 位短缩写 `4de4e4258`，全 SHA 见 10.6）。
- **如实披露（实测）**：仓库内物理台账文件 `ocr-ledger.json` / `ocr-ledger.md`（被 `.gitignore` 的 `.*` 规则忽略、未跟踪，mtime 停留 09-23 22:16）**未随终局轮回写**，内容仍为 `maxCleanHead=2bf3bde8c`、`taskRanges=[]`——沿袭第 9 章决策项 #14 的同一缺口。**未回写的直接后果**：后续会话若按物理台账重建状态，将以 `--from 2bf3bde8c` 重扫 09-23 之后的全部已修复区间（重复劳动 + 同型发现重复计数风险，含"0 high"口径失真风险，10.3 漂移节）——已列入 10.7.1 行动清单第 4 项。台账文件回写属主流程/授权会话职权；本章仅记录推进值与未回写事实，**不宣称台账文件已更新**。

### 10.6 最终 HEAD 与总提交数（本章 git 实测）

| 项 | 值 |
| --- | --- |
| 最终 HEAD | `4de4e42586790463b7eb98caf61fd125401cd601`（`git rev-parse HEAD` 实测；即终局第 2 轮完成记录 docs 提交，2026-09-26 17:40:27 +0800） |
| 基线..HEAD 提交数 | **163**（`git rev-list --count 4bcad69ba..HEAD` 实测 = 上版 149 + B+A 批次 6（10.2 表）+ 终局 F/F2 修复轮 8（10.4 表）） |
| 分支 / upstream | `codex/issue-106-self-hosted-plugins`；`upstream` 为空（`git for-each-ref` 实测，**未推送**） |
| 工作树 | 无已跟踪文件改动；28 个未跟踪文档（上版 25 + 终局三轮报告 3，`git status --porcelain` 实测） |

### 10.7 总结论（终局更新——更新第 1 章，以本节为准）

**仍为部分完成（本地）。** 判定依据：任务书完成标准的声明条件为"`ocrPass=true` 且安全修复 fixed"（来源：任务书内联给定，仓库内无独立载体可复核，10.1）——安全修复侧，B+A 9 项已全部处置（4 项本轮 RED→GREEN 修复 + 5 项经核实已由此前轮次修复并留证，10.2）；OCR 门控侧，`ocrPass=false` 维持未翻转（终局三轮后第 3 轮仍有 30 条 findings，10.4）。条件不满足，**如实报告部分完成，不声明达到任务书完成标准**。

**"安全修复 fixed"的判定口径（应读者问，本章明示）**：= B+A 9 项各自具备 RED→GREEN 测试证据或已修复核复跑证据（10.2 表逐项列源），**不等于"零残留"**。已知残留二：① remaining 第 1 条——#1 的"物化前被杀"子形态保持 409（其"无暴露面（fail-closed）"判断的实现载体是 `plugin_install_service.go:703-707` 的 `resolved == ""` 分支及代码注释"no materialized row at all: nothing serves…fail-closed shape"，**属代码注释支撑的分析性断言，无专属测试钉住**——本章 grep 实测：install_service_test.go 仅覆盖行完备 409（TestConfirmInstallationIdempotent :664-680）与另两个崩溃变体（:684/:719 起），无"删物化行→重试 409"用例）；② B+A#1 收口的是重试自愈路径，崩溃后、重试前的窗口残留见 10.8.1。

与上版结论（第 1 章）的增量变化：① 上版 9 条阻塞中 **4 条已修复**（R3-15/R3-16/R3-22 经 B+A #1/#2/#4、R3-01 经终局 f27——f27 同时计入终局修复 9 项 F09/F10/F13/F11/F02+f14/f27/f01+f02），**其余 5 条未修复**（含后端 R3-03 与前端 R3-08/09/10/18），全部转入用户裁决接受清单（10.2.1 映射表为完整口径）；② 分支自 149 推进至 163 提交、HEAD `28c46776` → `4de4e4258`；③ 终局三轮后剩余发现（10.3 清单）转为**用户裁决接受、随 diff review 处理**。

诚实红线（终局重申）：以上全部为本地 worktree 状态——**未推送（`upstream` 空，实测）、未合并、未部署、未发布、未关闭任何 GitHub Issue、未在 GitHub 发表任何评论**；接受清单为"用户裁决接受、随 diff review 处理"，不表述为已解决。

#### 10.7.1 终局后开放决策 / 行动清单（第 9 章 #10–#15 的终局状态 + 终局新增项）

| # | 事项 | 终局状态 |
| --- | --- | --- |
| 1 | 第 9 章 #10（处置 round-3 22 条并重跑第 4 轮 OCR） | **已被用户裁决部分取代**：后端阻塞经 B+A 处置、R3-01 经 f27 修复、其余 5 条阻塞 + 第 3 轮 30 条转入"接受、随 diff review 处理"（10.3）；"重跑至零 findings"路径不再执行，`ocrPass` 维持 false 收口 |
| 2 | 第 9 章 #11（分支去向：推送/PR/合并） | 仍开放；**且为"diff review"处置路径的前置条件**——未推送、无 PR，该 review 当前无发生场所（10.3 操作定义节） |
| 3 | 第 9 章 #12（提交未跟踪文档） | 仍开放；数量 25→28（+终局三轮报告） |
| 4 | 第 9 章 #14（OCR 台账回写） | **仍未做，后果已具体化**（10.5）：按物理台账重跑将从 `2bf3bde8c` 重扫全部已修区间 |
| 5 | 第 9 章 #13（他人所有权文件：harness 缺陷 / agent-editor flaky） | 仍开放，终局各轮未触碰 |
| 6 | 第 9 章 #15（GAP 增补工程前置 Issue） | 事后追认性质，不变 |
| 7 | **终局新增**：integration 真 PG 五测与 T20 验收套件（B1–B10）在终局 HEAD 的复验（10.7.2） | 未执行，待授权会话 |
| 8 | **终局新增**："diff review"操作定义（责任人/触发条件/跟踪载体）的补记 | 无落盘（10.3），待主流程/用户补记 |
| 9 | **终局新增**：R3-15 UI 误导面在"崩溃→重试"窗口内的残留（10.8.1） | 未处置，随 diff review 或后续轮处理 |

#### 10.7.2 终局 HEAD 的验收/回归未复验面（第 3 章 B1–B10 结论对终局 HEAD 是否仍成立——如实回答：未经复验，无法断言）

- **TestPlugin106Acceptance（B1–B10 十边界，真 PG）与 `-tags integration` 真 PG 五测在 B+A/F/F2 各轮记录中均无复跑记载**（本章 grep 实测：`fix-plans.md` 全文 "Plugin106Acceptance/TestPlugin106" 零命中；B+A 回归证据仅 `go test ./...`（无 tag 单元全量）+ test:shared + typecheck:shared 三条，`fix-plans.md:1713-1723`；F/F2 轮见 10.4 回归证据段——同样无 integration/验收套件）。remaining 第 2 条所载"ask 门控清单未列"系**流程性理由，非技术豁免论证**。
- 因此**第 3 章"B1–B10 全部核销"的结论时点是 T20/收尾评审（旧 HEAD），对终局 HEAD `4de4e4258` 未经复验、是否仍成立本章无法断言**。影响面分析（读修复语义的推断，非测试证据，如实标注）：B+A#1 直触 B5（写工具默认关）运行时补齐逻辑（install_service_test.go:707-709 有单元级断言"healed write-tool row lands disabled (B5)"）；终局 F10 触 B2 确认链并发写序、F13 transport guard 触 B2/B6 候选不可达路径——各有单元级 GREEN 证据（10.2/10.4），但真 PG 端到端验收链（预览→确认→升级→漂移→审批全链）未在终局 HEAD 重跑。本章（交付整理员角色，只写文档不改代码不跑套件）未复跑；复验属 10.7.1 第 7 项。

### 10.8 终局后遗留面回访（两条上版阻塞的伴生症状在终局 HEAD 的现状，本章读码实测）

#### 10.8.1 R3-15 的 UI 误导面（治理视图对缺行显示 Enabled=ReadOnly，与运行时 fail-open 行为相反，:266）

B+A#1 修复提交 `a9d0fd0ce` 仅改 `plugin_install_service.go` + 3 个测试文件（`git show --stat` 实测：4 files），**未改治理视图的缺行显示逻辑，也未改运行时"缺行=启用"默认**——`internal/types/mcp.go:151-153` 注释"Missing rows are treated as enabled for backwards compatibility"仍在（本章实测）。修复收口的是**重试自愈**：幂等短路/heal 路径增量补齐缺行（`plugin_install_service.go:736-748`，新行 Enabled=ReadOnly），使缺行状态在管理员重试后不再存在、治理视图与运行时的相反呈现随之消失。**残留**：崩溃发生 → 管理员重试之前的窗口内，缺行仍在——该窗口内治理视图 ReadOnly 显示与运行时缺行默认启用的相反呈现仍可能出现（UI 呈现面未随修，已列 10.7.1 第 9 项）。

#### 10.8.2 R3-01 的双入口并存语境（pluginsSlot 通道 vs settings 分区）

f27 修复提交 `d01ff28e1` 仅改 `packages/views/src/integrations/settings-route.ts` + 其测试（`git show --stat` 实测：2 files）。**已修**：归一化劫持——已注册设置分区裸 key 保持原样（`settings-route.ts:15-22` 实测，`SETTINGS_SECTIONS.some(...) === section` 分支），`?section=plugins` 深链/popstate 不再被劫持到成员发现 tab。**未随修（仍并存）**：双 'plugins' key 两处注册仍在——`packages/views/src/settings/registry.ts:43`（PluginSettings/admin）与 `packages/views/src/integrations/registry.ts:27`（PluginDiscoverPanel/viewer）（本章 grep 实测）；`apps/web/src/integrations/route.ts:3` `integrationTabs` 仍为 `['im','embed','api','cli','chrome','claw']` 无 'plugins' 项（实测）——成员发现面板仍经 pluginsSlot 通道挂载（上版第 5 章读码，f27 未触碰该文件）。即：R3-01 的"劫持"症状已修，"两个同名「插件」入口指向不同分区"的并存语境按 f27 注释自述（"It also happens to name an integrations tab"，settings-route.ts:17-19）属已知并存；侧边栏层面两个同名标签的呈现是否消除，本章未核验导航层标签（如实标注）。

### 10.9 本章实测命令清单（均在 worktree 根执行）

| 命令/操作 | 结果 |
| --- | --- |
| `git rev-parse HEAD` / `git branch --show-current` | `4de4e42586790463b7eb98caf61fd125401cd601` / `codex/issue-106-self-hosted-plugins` |
| `git rev-list --count 4bcad69ba..HEAD` | 163 |
| `git rev-parse 318de31a7 891f351d9 25a21de58 28fcaa96b a9d0fd0ce ab4d9c75e` | 六提交全 SHA（10.2 表），均存在 |
| `git for-each-ref --format='%(refname) upstream=%(upstream)' refs/heads/codex/issue-106-self-hosted-plugins` | `upstream=` 空（未推送） |
| `git status --porcelain` | 无已跟踪改动；28 个未跟踪文档 |
| `git log -8 --format='%h %ci %s'` | 终局 F/F2 轮 8 提交与时刻（10.4 时间线） |
| `git ls-files` + `git check-ignore -v ocr-ledger.json/.md` | 两台账文件未跟踪、被 `.gitignore:2`（`.*`）忽略 |
| `cat ocr-ledger.json` / `grep -n maxCleanHead ocr-ledger.md` | 停留 `2bf3bde8c`（09-23 22:16 未回写，10.5） |
| `grep -rn "4de4e425" .superpowers/ docs/plans/` | 零命中（推进值此前无落盘，本章首记） |
| `head -1` + `grep -E '^\[' …/issue-106-ocr-final-round-{1,2,3}.md` | 19/27/30 findings 与严重度分布（10.3/10.4 表） |
| `sed -n '1605,2200p' fix-plans.md`（含 :1613/:1713/:1728/:1884/:1948/:2038 节锚） | B+A 批次与 F/F2 轮全记录（10.2–10.4 引用源） |
| `grep -n "B+A\|用户裁决" fix-plans.md` | B+A 裁决锚 :1615；"接受清单"原文无独立落盘（10.3 标注） |

**修订增补（读者问复核轮，2026-09-26 晚第二遍）**：

| 命令/操作 | 结果 |
| --- | --- |
| `grep -n "Plugin106Acceptance\|TestPlugin106" fix-plans.md` | **零命中**——B+A/F/F2 各轮无验收套件复跑记载（10.7.2 依据） |
| `sed -n '1948,1975p' fix-plans.md` | F2 轮 f14 严重性下调论证原文（:1960-1964，10.3 漂移节引用） |
| `grep -n "物化前被杀\|反查无行" internal/modules/plugins/*_test.go` + `sed -n '664,790p' install_service_test.go` | "物化前被杀保持 409"仅测试注释（:725）；无删物化行→409 专属用例（10.7 判定口径依据） |
| `sed -n '652,720p' internal/application/service/plugin_install_service.go` | `resolved == ""` → alreadyInstalled 409 分支与 fail-closed 注释（:703-707）；heal 增量补齐（:736-748） |
| `git show a9d0fd0ce --stat` / `git show d01ff28e1 --stat` | #1 仅改 service+3 测试文件；f27 仅改 settings-route.ts+test（10.8 依据） |
| `sed -n '1,8p' apps/web/src/integrations/route.ts` + `grep -n "plugins" packages/views/src/settings/registry.ts packages/views/src/integrations/registry.ts` | integrationTabs 仍无 'plugins'；双 'plugins' key 两处注册仍在（registry.ts:43 / integrations/registry.ts:27） |
| `sed -n '149,155p' internal/types/mcp.go` | "Missing rows are treated as enabled" 注释仍在（10.8.1 依据） |
| `sed -n '1884,1946p'`+grep exit/stash/Mimosa `fix-plans.md` | F 轮门控原文（10.4 回归证据段依据） |

---

## 附 A：诚实声明

- 本分支及本报告全部工作仅在本地 worktree `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue106` 内完成；**未推送（`upstream` 为空，实测）、未合并、未部署、未发布、未关闭任何 Issue、未在 GitHub 发表评论**。
- 第 6.1 章三条门控结果为**执行数据权威记录**（exit 0），本报告撰写会话未复跑；T20 验收套件、各任务 TDD、R1/R2 轮全量回归为账本/fix-plans 记录，本报告未复现历史中间态。本报告实测项限于附录 B 所列（git 事实、文件存在性、挂载点读码、OCR 报告读取）。
- 执行数据双口径（issueStatus/taskSummary）已如实呈现，未取其一硬凑结论；台账滞后、停止修复原因、7 条 low 未进终裁清单等数据缺口均已标注而非臆测。
- 就地标注的推断与数据缺口：① "9 个未解决有效阻塞发现 = round-3 的 2 high + 7 medium"为按报告严重度的数字吻合解读（终裁原文未点名计数口径）；② R3 编号与 round-3 报告条目的对应按位置匹配；③ round-1/2 修复轮"33/22 条有效发现"的判定出自 fix-plans 输入行原文，但 12/13 条被过滤项的逐条理由无明细记载（数据缺口）；④ R1 轮 F 编号"缺号跳过"含义按 fix-plans 标题自述解读（缺号=未进修复批次）；⑤ 旧版中间报告被本版覆盖，旧文本未另行保存（其关键结论已收录于本版沿革说明）；⑥ 执行数据转录件为任务书内联文本的原样转录（evidence 截断原样保留），非主流程原始文件；⑦ "未关闭 Issue/未发评论"系流程约束宣称，未经 gh api 实证；⑧ 第 3 轮后停止修复的具体动因、两条 high 修复工作量——均无记载/未评估，如实标注不估算。

## 附 B：本报告撰写会话实际运行的核实命令清单（均在 worktree 根或对其执行）

| 命令/操作 | 结果 |
| --- | --- |
| `git rev-parse HEAD` / `git branch --show-current` | `28c46776b3b4b75ae8be44d68aaead93505b6e33` / `codex/issue-106-self-hosted-plugins` |
| `git rev-list --count 4bcad69ba…HEAD` | 149 |
| `git for-each-ref --format='…%(upstream)…' refs/heads/codex/issue-106-self-hosted-plugins` | `upstream=` 空（未推送） |
| `git status --porcelain` | 24 个未跟踪文件（本报告 + 23 份 OCR 报告） |
| `git log --format='%s' 基线..HEAD \| grep -oE '\[T[0-9]{2}\]' \| sort \| uniq -c` | 第 5 章标签分布（123 处标签） |
| `git log --format='%h %s' 基线..HEAD \| grep -vE '\[T[0-9]{2}\]'`（28 条逐一读提交信息） | 第 5 章 28 个无标签提交构成表（OCR R1/R2 轮 12 + 转交 4 + 门控 3 + 终评 1 + docs(sdd) 3 + docs 规划 5） |
| `git log --oneline -1 2bf3bde8c` / `git rev-list --count 4bcad69ba..2bf3bde8c` | 提交信息为终评 R3 共享修复；位于基线后第 10 个提交 |
| `sed -n '27,60p' scripts/run-gates.mjs` / `grep -n '"gates"\|"test:shared"\|…' package.json` | GATES 数组七项构成（T20 时点六道 + R1-F17 新增首项）；三条基线与之的映射（6.1.1） |
| `cat .superpowers/sdd/2026-09-23-issue-106/ocr-ledger.md` | 首轮收尾 OCR 命令与"未来重跑"模板（:17/:20）、clean 区间选片 6 文件语义（:18-19）、任务级表免责说明 |
| `sed -n '1235,1245p;1451,1462p' …/fix-plans.md` | R1 轮"输入：33 条有效发现"（:1238）、R2 轮"输入：22 条有效发现"（:1453）、"处置前逐项读码复核"原则（7.2 差额说明） |
| `grep -n '有效\|误报\|重复\|无效\|裁否\|拒绝' …/fix-plans.md` | 历轮有效性过滤实例（F6.5 官方文档误报 :310、R6 复核误报 :464、T01-R2-F5 部分误报 :617） |
| `sed -n '279,293p' docs/plans/issue-106-ocr-round-3.md` | R3-15 全文（实际后果：缺策略行运行时默认 enabled=true、新增写工具可被发现并派发、治理视图显示与运行时相反） |
| `sed -n '145,155p' internal/types/mcp.go` | :151-153 注释"Missing rows are treated as enabled for backwards compatibility"（缺行默认启用的机制根源实证） |
| `sed -n '1,40p' scripts/run-gates.mjs` | 头部注释（node≥26 自检重执行、六道门流水线描述） |
| `ls migrations/versioned/ \| tail -3` / `ls migrations/sqlite/ \| tail -3` | 000189/000190 与 000110/000111 全部存在 |
| `grep -rn "PluginsPanel" apps/web/src --include='*.tsx' -l \| grep -v test` | 命中 `IntegrationsRoutePage.tsx` 等 3 文件 |
| `sed -n '1,10p' apps/web/src/integrations/route.ts` | `integrationTabs=['im','embed','api','cli','chrome','claw']`（无 'plugins'） |
| `grep -n "PluginsPanel\|plugins" apps/web/src/integrations/IntegrationsRoutePage.tsx` | :15 import、:217 `pluginsSlot={<PluginsPanel client={client} />}` |
| `grep -rn "PluginSettings\|plugins" packages/views/src/settings/registry.ts` | :43 `{ key: 'plugins', …, minRole: 'admin', … }` |
| `head -3 issue-106-ocr-round-1.md` / `round-2.md` | "45 finding(s)" / "35 finding(s)" |
| `grep -nE '^\[(bug\|security\|…)' issue-106-ocr-round-3.md` + `grep -n '─── '` | 22 条严重度与位置全列（2 high + 7 medium + 13 low） |
| `cat .superpowers/sdd/2026-09-23-issue-106/ocr-ledger.json` | taskRanges=[]、maxCleanHead=2bf3bde8c（未回写） |
| `cat …/rulings.md` | LR1–LR4 全文（含 LR4 卸载端点裁决） |
| `grep -n '^# ' …/fix-plans.md` | R3–R12 + 09-26 R1/R2 轮节结构（:1/:105/:209/:249/:854/:963/:1039/:1090/:1235/:1451） |
| `sed -n '854,900p' …/fix-plans.md` | R9"按主流程指令转入实施"原文 |
| `sed -n '1369,1650p' …/fix-plans.md` | R1 轮（33 条有效发现全部修复）/R2 轮（22 条有效发现全部修复）完成记录 |
| Read（全文或节选） | `issue-106-final-report.md` 旧版全文（42 提交时点中间版）、`2026-09-23-issue-106-trace-matrix.md`、`2026-09-23-issue-106-00-index.md`（:1-130）、`issue-106-ocr-round-3.md`（:1-120 + grep 全文）、`tasks/T20.md`（尾部验收与 Concerns）、`tasks/T03.md`/`T06.md`/`T17.md`（头部状态） |
