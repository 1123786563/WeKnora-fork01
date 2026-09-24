# Issue #30 本轮交付最终报告（issue30-sweep）

- 报告日期：2026-09-24（本版为全轮终版，取代中途版 `5c8e592a5`）
- Worktree：`.worktrees/issue30-sweep`，分支 `codex/issue30-mobile-office`，HEAD `4f71e9d97`
- 提交范围：`29c1e5635..4f71e9d97`（共 **182 个提交**，`git log --oneline 29c1e5635..4f71e9d97 | wc -l` 实测；其中中途版报告 `3f72d96db` 之前的 83 个已在途中报告覆盖，本报告为全范围终版）
- 本轮实施范围：#30 的 41 个下级 Issue 中实施 **12 个**（#32、#33、#34、#35、#66、#36、#38、#41、#42、#44、#46、#59，对应 T02、T03、T04、T05、T36、T06、T08、T11、T12、T14、T16、T29），共 **90 个计划任务**（各计划文件 `### Task N` 标题计数实测：6+6+9+11+6+7+7+7+7+8+9+7 = 90），分三个并行波次（B1、B2、B3）交付
- 状态口径声明：以下所有「完成 / 已验证完成」均指**本地 worktree 分支上的交付与本地审查结论**；未推送远端、未合并到 `main`、未关闭任何 GitHub Issue（详见文末声明）

---

## 一、下级 Issue 状态与 #30 总体验收覆盖

### 1.1 本轮实施的 12 个 Issue

编排器执行统计（原样呈现）：

| Issue | T | 任务（编排器口径） | 最终审查（finalApproved） | 状态判定 |
|---|---|---|---|---|
| #32 | T02 | 6/6 完成 | **false**（1 项残留，见下） | **完成（含 1 项最终审查残留呈报，不宣称完全干净）** |
| #33 | T03 | 6/6 完成 | true | 已验证完成（本地审查口径） |
| #34 | T04 | 9/9 完成 | true | 已验证完成（本地审查口径） |
| #35 | T05 | 11/11 完成 | true | 已验证完成（本地审查口径） |
| #66 | T36 | 6/6 完成 | true | 已验证完成（本地审查口径） |
| #36 | T06 | 7/7 完成 | true | 已验证完成（本地审查口径） |
| #38 | T08 | 7/7 完成 | true | 已验证完成（本地审查口径） |
| #41 | T11 | 7/7 完成 | true | 已验证完成（本地审查口径） |
| #42 | T12 | 7/7 完成 | true | 已验证完成（本地审查口径） |
| #44 | T14 | 8/8 完成 | true | 已验证完成（本地审查口径） |
| #46 | T16 | 9/9 完成 | true | 已验证完成（本地审查口径） |
| #59 | T29 | 7/7 完成 | true | 已验证完成（本地审查口径） |

**各 Issue 目标与交付摘要**（目标引自编排器本轮计划结果，交付证据引自 Ledger/集成记录/提交）：

- **#32（T02）Active Tenant 切换与 Scoped Vault 隔离**（波次 1）：用户可在移动端选择并切换 Active Tenant；新建 Scoped Vault 深模块让加密缓存/草稿按 Deployment×用户×Tenant 隔离，切换/退出/换部署时旧 scope lease 同步撤销、wrapped key 轮换+行擦除（fail closed），迟到响应经 epoch 拒绝；租户切换具备真实 HTTP 最高稳定 Interface 的 opt-in 集成证据。计划 gate PASS（[plan-t32 Ledger](../../.superpowers/sdd/plan-t32/progress.md)，gate 行原文实测在案）。**残留**：最终审查发现（important）「`disallowedDeploymentHost` 可被 IPv6 形式绕过」——修复波已实测 ADDRESSED（修复报告 [t32/final-fix-report.md](../../.superpowers/sdd/t32/final-fix-report.md)：修复前 tsx 复现脚本对 `[fe80::1]`/`[fc00::1]`/`[fd12:3456:789a::1]`/`[::ffff:7f00:1]` 全部 `enabled:true` 确认漏洞，修复后全部 `enabled:false/disposition:'invalid'`，`parseIpv6Literal` 展开 16 字节，主机拒绝测试循环从 9 个 URL 扩到 25 个并含公网 IPv6 正向守护；`runtime.integration.test.ts` 6 tests 5 pass/1 opt-in skip）。按 SDD 规则最终审查后不再有第二波，该项作为残留呈报用户，故 #32 不宣称完全干净、finalApproved=false。
- **#33（T03）Resource Shelf**（波次 1）：新增 Resources 资源页，经新建 mobile-core Resource Shelf 深 Module（browse/selection/subscribe）端到端展示当前 Active Tenant 的 Available Agent、知识与 Connection 摘要并解释三态；撤权（403）或 scope/capability 变化后投影立即失效。计划 gate PASS（[plan-t33 Ledger](../../.superpowers/sdd/plan-t33/progress.md)）；最终审查 2 项 Minor 发现全部修复（[t33/final-fix-report.md](../../.superpowers/sdd/t33/final-fix-report.md)：render 相 controller 创建移入 effect、401 重试改结构化 status 判定，各附 RED/GREEN 证据）。
- **#34（T04）首页 Attention 与统一 Task 列表**（波次 1）：首页一次聚合读展示三段视图，统一 Task 列表支持搜索、筛选与归档（含归档写路径），全部读自当前 Tenant 的授权聚合读模型，经新建 Task Office 深模块 + opt-in 真实 HTTP 集成证据端到端验证。计划 gate PASS（[plan-t34 Ledger](../../.superpowers/sdd/plan-t34/progress.md)，含 Go 侧 `TestWorkbenchList|TestWorkbenchTask|TestMX013|TestWorkbenchNotifications`）。计划审查第 2 轮 7 项残留与冲突扫描 2 项 blocking（Ruling 3/5）作为 Review Focus 传入任务审查后吸收。
- **#35（T05）Task 详情 Snapshot、Timeline 与 SSE 恢复**（波次 2）：结果优先详情、Task/Run/Attention 三层状态与规范 Timeline，经 Snapshot 水合、游标 SSE、缺口/裁剪补洞与有界重同步完成 App 重启、断线与终态 drain 的可验证恢复；独立审查四项发现已修复（阻断：Task 9/10 装配补传 detail port 并增加冒烟源级守卫）。
- **#66（T36）多 Deployment 切换与兼容性降级**（波次 2）：一台设备可登记多个官方云/自托管 Deployment 并原子切换（不携带旧凭据、不接纳迟到响应），缺安全关键 capability 时进入「说明 + 有限只读」降级面，经 MobileRuntime Interface 测试 + opt-in 真实 HTTP 多实例切换证据验证。集成记录 [integrate-t66.md](../../.superpowers/sdd/integrate-t66.md)。
- **#36（T06）通用目标输入与耐久 Task 创建**（波次 3/B3）：统一 New 入口——描述目标、接受或改选推荐主理 Agent、附加资源与预算后创建 Task；request_id 在 Start POST 前耐久持久化（意图日志），ACK 丢失用同一 request_id 与同一 session 对账，相同意图绝不重复创建 Task 或预算预占，附件未就绪/离线/输入冲突保留草稿且零危险重放。审查 F1–F6 已修复；最终审查 F1（goal TextInput 无 maxLength）已修复（[t36/final-fix-report.md](../../.superpowers/sdd/t36/final-fix-report.md)，分支提交 `8c7d71134`：`GOAL_TEXT_MAX_LENGTH=500` 源头拦截 + 字节预算交叉验证测试，实测最坏单条 1791B ≤ 2048B SecureStore 信封）。
- **#38（T08）Attention Inbox 与类型化审批闭环**（波次 3/B3）：新增 GET /api/v1/workbench/interactions 收件箱读与 Task Office inbox()/decide() 接口，决定经冻结 decision_id 幂等重放与 revision/digest CAS 只生效一次，receipt 如实区分 recorded/delivery-unknown/superseded/gone；真机多设备端到端如实列为 blocked-env 并给本地替代证据；审查发现 F1–F7 全部修复并复核落盘。集成记录 [integrate-b3-t38.md](../../.superpowers/sdd/integrate-b3-t38.md)。
- **#41（T11）注册设备、行动通知与安全深链**（波次 3/B3）：可撤销设备注册（两步 intent→register、token 接管、撤销幂等），行动通知只作同步 hint 触发权威重投影（点击 = 安全深链解析→重新鉴权→权威 Task 详情→本地已读，绝不执行业务操作），错误 deep link 全部拒绝。与 #38 的 `/inbox` 撞路径在集成时分流（#41 占 `/inbox`，T08 迁 `/attention`，集成裁量详见 [integrate-b3-t41.md](../../.superpowers/sdd/integrate-b3-t41.md)）；最终审查 F1（markRead 失败逃逸）已修复（分支 tip `c3076ce20`）。
- **#42（T12）Task Owner、Collaborator、Viewer 协作**（波次 3/B3）：Task 级显式协作授权（Owner 授予同租户成员 Viewer/Collaborator），在扩额、个人连接、副作用审批三个通道落地权限严格分离门禁，全部行为在 HTTP wire 级集成测试验证；审查 6 项 findings 逐一修复。其中一处「approve 移出 capability gate」经升级裁决落地（提交 `d45593bd9` 标注 ruling via escalation）。
- **#44（T14）旧 Session 投影为 Legacy Task**（波次 3/B3）：从未有过 Run 的旧 Session 以同一身份（taskId=sessionId）进入显式 Legacy Task 投影（Go 读模型+端点+同身份归档），原生 App 可查看历史并经既有聊天 wire 继续普通追问，Run 级新安全语义显式门禁为「需新建 Run」，真实迁移测试锁定验收。集成记录 [integrate-b3-t44.md](../../.superpowers/sdd/integrate-b3-t44.md)。
- **#46（T16）Task Material**（波次 3/B3）：经 Task Material 深模块 Interface 列出并打开 Task 材料（Artifact/Files/Diff/测试报告/Evidence 引用/只读 Terminal），短时效签名授权下载与系统分享；独立审查两处阻断与三处低危已全部实跑/等价性验证修复。集成记录 [integrate-b3-t46.md](../../.superpowers/sdd/integrate-b3-t46.md)。
- **#59（T29）Tenant Adoption、Agent Variant 与移动 Available Agent**（波次 3/B3）：在 #58 Tenant Release 闭环之上实现 Adoption→多 Variant→本地能力映射→测试→本地 Agent Version 发布的治理层，端到端 HTTP 测试证明发布产物进入移动 Resource Shelf 消费的 GET /api/v1/agents 投影与新 available-agents 读模型，缺能力时拒绝原因逐项点名；审查 4 项 findings 全部修复（分支 tip `a52207942` final fix）。

**iOS 模拟器实测对上述交付的覆盖**：Release 构建/安装/启动/首屏渲染 ✅（首轮修复轮后）；`weknora:///resources` deep link warm+冷启动 ✅（首轮）；B3 复验轮对 7 条 B3 路由（`/inbox`、`/new`、`/tasks/legacy`、`/tasks/materials`、`/attention`、`/resources`、`/tasks/detail`）deep link 可达且各自渲染正确的未授权 gate 文案、冷启动直达、正常重启不粘连、无崩溃/无 JS 错误 ✅（[b3-recheck.md](ios-evidence/b3-recheck.md)）。**全部 12 个 Issue 的授权面交互 ❌ 未验证**——本环境无 deployment 凭据与后端（详见第四节、第七节）。

### 1.2 其余 29 个下级 Issue 的 DAG 状态与现状

依据 [dag.md](dag.md)（2026-09-23 生成时点的调查快照；状态总表未回写，下表「现状」列为报告撰写时按本轮交付更新的口径）：

| 分组 | Issue | DAG 批次 | 现状 |
|---|---|---|---|
| 前期已实现（非本轮） | #31（T01 登录 Deployment） | B0 | 实现已集成分支（feature/mobile-office@0c5a6bdc）；首轮 iOS 修复轮解决 iOS 27 scene 生命周期/冷启动深链/构建问题，但 HTTPS staging 登录/OIDC 真实凭据与 Android 设备证据仍外部阻塞（blocked-external），验收开放 |
| 前期已关闭 | #58（T28 Catalog Release） | -（done-evidenced） | 2026-09-20 关闭；残余缺口仅 PostgreSQL 迁移复跑（环境性，不重开） |
| **本轮已实施** | #32（B1）；#33/#34/#35/#66（B2）；#36/#38/#41/#42/#44/#46/#59（B3） | B1–B3 | 见 1.1；**B1、B2、B3 三个批次全部节点已在本轮交付** |
| 未实施（B4，11 个） | #37、#39、#40、#43、#45、#48、#52、#56、#60、#67、#68 | B4 | open、未实施。**本轮后其全部前置已满足**（逐节点核对 dag.md 边表：#37←#35✅#36✅；#39←#36✅#38✅+推断 #42✅；#40←#32✅#35✅#36✅；#43←#42✅；#45←#33✅#35✅#36✅；#48←#38✅#46✅；#52←#36✅#38✅#46✅；#56←#36✅；#60←#59✅且 #58 已满足；#67←#41✅#66✅；#68←#34✅#35✅#36✅#38✅#46✅）——B4 全部 11 节点现已就绪可并行启动；其中 #48 有 NOTION_TOKEN 类环境性阻塞（仅真实集成证据，dag.md 第 8 节） |
| 未实施（B5，11 个） | #47、#49、#50、#51、#53、#54、#57、#61、#62、#69、#70 | B5 | open、未实施；前置部分满足（#53←#42✅、#57←#35✅、#61/#62←#59✅、#69/#70←#41✅#66✅），其余待 B4；#69/#70 安装包验收 blocked-env（签名/真机缺失） |
| 未实施（B6–B8，5 个） | #55、#63、#64（B6）；#65（B7）；#71（B8） | B6–B8 | open、未实施；#71 为跨平台发布证据矩阵，是 #30 首版验收的收口节点 |

**#30 总体验收覆盖情况**：#30 的完成定义是 41 个下级 Issue 全部交付并以 #71 证据矩阵收口。本轮覆盖 **12/41**；加上前期 #31（验收开放）与已关闭 #58，尚有 **27 个未实施**（B4–B8 全部节点）。#30 远未达到总体验收状态，本轮属于 DAG 关键路径 B1+B2+B3 三个批次的纵向推进（B4 已整批解锁）。

---

## 二、交付物路径清单

以下路径相对 worktree 根 `.worktrees/issue30-sweep`；从本报告所在目录（`docs/plans/issue30-sweep/`）出发的相对链接可直接点击。**主仓库（`/Users/wuyongjun/trea/WeKnora-fork01/docs/plans/issue30-sweep/`）目前仅有本报告一份副本，其余材料均在 worktree 内**（主仓库该目录实测仅 `FINAL-REPORT.md` 一文件）。

### 层级树 / DAG / 需求

- 层级树与 41 个子 Issue 清单：[issues/index.md](issues/index.md)（单 Issue 详情 `issues/issue-30.md` … `issues/issue-71.md`，42 个文件实测在册）
- 依赖 DAG（92 边、9 批次、Kahn 无环验证、42 节点状态总表）：[dag.md](dag.md)

### 实施计划（12 份，任务数为 `### Task N` 标题实测计数）

- T02：[plans/plan-t32.md](plans/plan-t32.md)（6 任务）
- T03：[plans/plan-t33.md](plans/plan-t33.md)（6 任务）
- T04：[plans/plan-t34.md](plans/plan-t34.md)（9 任务）
- T05：[plans/plan-t35.md](plans/plan-t35.md)（11 任务）
- T36：[plans/plan-t66.md](plans/plan-t66.md)（6 任务）
- T06：[plans/plan-t36.md](plans/plan-t36.md)（7 任务）
- T08：[plans/plan-t38.md](plans/plan-t38.md)（7 任务）
- T11：[plans/plan-t41.md](plans/plan-t41.md)（7 任务）
- T12：[plans/plan-t42.md](plans/plan-t42.md)（7 任务）
- T14：[plans/plan-t44.md](plans/plan-t44.md)（8 任务）
- T16：[plans/plan-t46.md](plans/plan-t46.md)（9 任务）
- T29：[plans/plan-t59.md](plans/plan-t59.md)（7 任务）

### SDD Ledger 与集成记录（`.superpowers/sdd/`，git 未跟踪的执行台账）

- 首批计划 Ledger（含派发前冲突扫描、逐任务记录、计划 gate PASS）：[plan-t32/progress.md](../../.superpowers/sdd/plan-t32/progress.md)、[plan-t33/progress.md](../../.superpowers/sdd/plan-t33/progress.md)、[plan-t34/progress.md](../../.superpowers/sdd/plan-t34/progress.md)
- 最终审查修复报告（t 前缀目录）：[t32/final-fix-report.md](../../.superpowers/sdd/t32/final-fix-report.md)、[t33/final-fix-report.md](../../.superpowers/sdd/t33/final-fix-report.md)、[t36/final-fix-report.md](../../.superpowers/sdd/t36/final-fix-report.md)
- 集成记录：[integrate-t66.md](../../.superpowers/sdd/integrate-t66.md)、[integrate-b3-t38.md](../../.superpowers/sdd/integrate-b3-t38.md)、[integrate-b3-t41.md](../../.superpowers/sdd/integrate-b3-t41.md)、[integrate-b3-t44.md](../../.superpowers/sdd/integrate-b3-t44.md)、[integrate-b3-t46.md](../../.superpowers/sdd/integrate-b3-t46.md)
- **未持久化的 Ledger（如实声明）**：plan-t35/plan-t66 及 B3 全部 7 份计划的 `progress.md` 未入库——这些计划在并行 worktree（已清理，如 t36 的 `.worktrees/issue30-sweep-t36`）执行，Ledger 从未提交；其中 t35/t66/t38/t41/t44/t46 的执行证据以集成记录与提交本身替代，**t36/t42/t59 三个 merge 提交（`40bbd82e5`/`8f74258ca`/`d58675a67`）只有单行消息、无集成报告**（本次报告撰写时实测 `git log -1` 确认无 body），其集成验证证据只能追溯到分支内测试提交与后续 OCR/iOS 全量轮。

### iOS 模拟器实测证据

- 首轮测试报告：[ios-evidence/ios-test-report.md](ios-evidence/ios-test-report.md)；首轮修复报告：[ios-evidence/fix-report-round-1.md](ios-evidence/fix-report-round-1.md)（证据目录 `ios-evidence/fix-round-1/`）
- B3 复验报告：[ios-evidence/b3-recheck.md](ios-evidence/b3-recheck.md)（证据目录 [ios-evidence/b3-recheck/](ios-evidence/b3-recheck/)：7 路由截图+AX、冷启动、启动日志、xcodebuild 日志）
- B3 复验修复报告：[ios-evidence/b3-recheck-fix.md](ios-evidence/b3-recheck-fix.md)（证据目录 [ios-evidence/b3-recheck/fix/](ios-evidence/b3-recheck/fix/)）

### OCR 报告与修复计划（四轮扫描、三份修复报告）

- 第二批增量扫描：[ocr/ocr-increment-batch2.md](ocr/ocr-increment-batch2.md)（Review complete：46 findings / 29 selected items）；修复报告：[ocr/fix-report-increment-batch2.md](ocr/fix-report-increment-batch2.md)（10/10 任务）
- 最终轮扫描：[ocr/ocr-round-1.md](ocr/ocr-round-1.md)（Review **partially** complete：50 findings；28/62 selected items 因 LLM 429 限流失败，尾部 retry report 列明失败文件组）；修复报告：[ocr/fix-report-round-1.md](ocr/fix-report-round-1.md)（13/13 任务）
- B3 增量扫描：[ocr/ocr-increment-b3.md](ocr/ocr-increment-b3.md)（Review complete：88 findings / 95 selected items；尾部 retry report：440 请求中 16 个受影响——5 个失败、11 个重试后恢复）；修复报告：[ocr/fix-report-increment-b3.md](ocr/fix-report-increment-b3.md)（15/15 任务）。**注意：该扫描报告文件目前是 worktree 中的未跟踪文件（`git status` 实测 `?? docs/plans/issue30-sweep/ocr/ocr-increment-b3.md`），未提交入库**。
- 修复计划：[plans/ocr-fix-round-1.md](plans/ocr-fix-round-1.md)、[plans/ocr-fix-increment-batch2.md](plans/ocr-fix-increment-batch2.md)、[plans/ocr-fix-increment-b3.md](plans/ocr-fix-increment-b3.md)
- **OCR 干净范围台账 `ocr/ocr-ledger.md`：不存在。** 任务输入将其列为既有材料（每行 `CLEAN <sha> | 轮次 | 报告`）。其设计用法：每轮 OCR 完成后把「确认干净」的基线提交 sha 记入台账，下一轮重跑时只审该 sha 之后的新增提交增量（CLEAN 行即各轮次续审起点），从而把全量重扫收敛为增量续审。本报告撰写时在 worktree、主仓库全树 `find`（含 `find /Users/wuyongjun/trea/WeKnora-fork01 -maxdepth 6 -name "ocr-ledger.md"` 实测）与 git 历史中均未找到该文件（唯一同名文件位于另一 Issue 的 worktree `.worktrees/issue106/`，与本轮无关）。因此**本轮 OCR 的续审基线从未落账**，后续重跑只能全量或以提交区间人工界定（详见 7.2）。

---

## 三、Worktree、分支与提交拓扑

### 3.1 Worktree 与分支

- 集成 worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep`，分支 `codex/issue30-mobile-office`（`git worktree list` 实测）
- 并行分支（worktree 已清理、分支仍在，`git branch` 实测 9 条）：`codex/issue30-t35`、`codex/issue30-t66`（波次 2），`codex/issue30-t36`/`t38`/`t41`/`t42`/`t44`/`t46`/`t59`（波次 3/B3）
- 基线：`29c1e5635`；最终 HEAD：`4f71e9d97`；范围共 **182 个提交**；merge 提交 9 个（波次 2 两个 + B3 七个，`git log --merges` 实测）

### 3.2 提交拓扑（关键节点，按时间序）

```
29c1e5635（基线）
├─ 规划产物：16dddb2d5（issue 清单）→ 387459617（DAG）→ 74ab7c856/dfe5e0f82（首批计划）
├─ 波次 1（#32/#33/#34，独立 worktree 并行实施、按序集成，落为线性串行段）：
│   #32 六任务（3d3fe3db0…6fcd7d472 + 4993dcdd3 host 防线）
│   #33 六任务（470fd8181…2a7691bd0 + 766b64577 清扫）
│   #34 九任务（49cb2dfbf…8733e7bc9 + 028f72b11 AC3）
├─ cfccdf6a4（第二批计划 35/66）
├─ 波次 2（同批次全并行、独立 worktree/分支）：
│   codex/issue30-t35（14 提交，63411e5c5…2151425c4）
│   codex/issue30-t66（6 提交，1a35e29a3…75dcdeaab）
├─ 979762df2 merge: integrate #35 (second batch)
├─ 417b7dedf merge: integrate codex/issue30-t66（3 冲突文件手工合并：
│   runtime/ports.ts、composition.ts、app-smoke.test.tsx）
├─ OCR 增量批次 2：bbe4cf7ec（计划）+ 10 修复提交（3445425d3…3e2c17e51）+ 0c07fd627（报告）
├─ iOS 首轮：56a859cb0（实测）+ d6f75a9ea（iOS 27 SDK 修复）
├─ OCR 最终轮：c71d6a03f（计划）+ 13 修复提交（195a1e77c…f7753fa16）
├─ 3f72d96db docs: ocr fix round 1 report（中途版终态，83 提交）
├─ 5c8e592a5 docs: 中途版最终报告（本报告所取代）
├─ 57e701a01 docs: b3 plans for 36,38,41,42,44,46,59
├─ 波次 3 / B3（7 节点全并行、独立 worktree/分支，commit 数为
│   git log 57e701a01..<branch> 实测）：
│   codex/issue30-t36（10 提交，832a272d0…8c7d71134 最终审查 F1）
│   codex/issue30-t38（11 提交，0081974bd…bbbec33d9 review fix）
│   codex/issue30-t41（8 提交，e16561bf7…c3076ce20 最终审查 F1）
│   codex/issue30-t42（10 提交，ca9b66ee1…214857e2f AC3 wire 证据）
│   codex/issue30-t44（10 提交，af98a3729…60c91bc6b AC3）
│   codex/issue30-t46（12 提交，d30378d72…8e8463107 AC3）
│   codex/issue30-t59（9 提交，a3132eaa0…a52207942 final fix）
├─ B3 按序集成（7 个 merge）：40bbd82e5(#36) → 1b6c89e09(#38) → fe61df857(#41，
│   /inbox 撞路径分流) → 8f74258ca(#42) → 21010717b(T44) → cff3786fd(T46)
│   → d58675a67(#59)
├─ 48abbd14f docs: ocr fix batch b3（计划）
├─ OCR B3 增量修复（15 提交）：096a5b1de → 5aa0c7cd1 → d10bf5bf9 → 9f273fe49
│   → e21047cb2 → 6ec1b9faa → d9b6ce54f → 2e2c328d8 → c36fdbb90 → 068c974c6
│   → dc520d199 → 34a2c1c8b → f66593b7a → b0d983f65 → e5e9ec9aa
├─ 13a761d2a docs: ocr fix increment b3 execution report (15/15)
├─ 01c9ccf5c test: b3 ios recheck
├─ aa51eeeaf test: b3 ios recheck fix round（MCP idb 根因 + 封印深度扫描）
└─ 4f71e9d97 docs: record second mimosa hook enobufs on b3 fix commit（最终 HEAD）
```

### 3.3 关键单点引用

- 首个实现提交：`3d3fe3db0`（#32 switch-tenant remote adapter）；最后一个实现提交波：B3 OCR Task 15 `e5e9ec9aa`
- 12 份 opt-in 真实 HTTP 集成证据提交：#32 `6fcd7d472`、#33 `2a7691bd0`、#34 `028f72b11`、#35 `7f2eb1d6c`、#66 `75dcdeaab`、#36 `c373e2a70`、#38 `7e3a3cf37`、#41 `4b226c0cd`、#42 `214857e2f`、#44 `60c91bc6b`、#46 `8e8463107`、#59 `0746e25a0`
- 后端（Go）侧代表提交：#34 `49cb2dfbf`/`9fdda6efe`/`11046e68d`（迁移+读模型）、#35 `63411e5c5`/`f6cf87307`（task facts）、#38 `0081974bd`/`8529fdb7a`（inbox store+service+端点）、#42 `ca9b66ee1`/`974231868`/`523d06252`（grants 迁移+服务+API）、#44 `af98a3729`/`533c40c5e`/`d8e6a1212`（legacy 读模型+端点+归档）、#46 `d30378d72`/`731e344bf`（terminal log 分页+端点）、#59 `a3132eaa0`…`d783ee503`（adoption 全链）
- OCR 修复代表提交：B2-F12 P0 属主错配 `3445425d3`；B3-F85 typecheck 门禁 `096a5b1de`

---

## 四、实际运行的测试与检查结果

**来源声明**：本节汇总自各 Ledger、集成记录、修复报告与 iOS 报告中记载的实跑命令与输出（出处逐一标注）；除「报告撰写时核实」标注外，本报告撰写会话本身未重跑测试套件（仅运行 git 历史核验、文件实读与 `find`/`diff` 类核验命令）。

### 4.1 计划级 gate（首批 SDD Ledger 记载，原文实测在案）

| 计划 | gate 结果 | 实跑命令（Ledger 原文） |
|---|---|---|
| t32 | PASS | `npx tsx --test`（runtime.test.ts、runtime.integration.test.ts、mobile-runtime.test.ts、runtime-vault.test.ts、scoped-vault.test.ts）+ `pnpm --filter @weknora/mobile test` + typecheck |
| t33 | PASS | `npx tsx --test`（domain 2、api-client 2、shelf 1、runtime 4 共 9 文件）+ mobile test + typecheck |
| t34 | PASS | `go test ./internal/application/repository/ ./internal/handler/session/ -run 'TestWorkbenchList\|TestWorkbenchTask\|TestMX013\|TestWorkbenchNotifications' -count=1` + `npx tsx --test`（contracts/mobile-core/api-client 6 文件）+ mobile test + typecheck |
| t35/t66 | 无持久化 Ledger | 集成 merge `417b7dedf` 记载的集成验证：`tsx --test` 6 文件 71 tests（69 pass/0 fail/2 skipped）+ mobile 40/40 + typecheck 通过（[integrate-t66.md](../../.superpowers/sdd/integrate-t66.md)） |

### 4.2 B3 集成验证（四份集成记录记载，全部本机实跑）

| Merge | 冲突 | 计划测试命令结果（摘自集成报告第 3/4 节） |
|---|---|---|
| `1b6c89e09`（#38） | 5 文件并集 | Go 3 组 `ok`（workbench service 0.397s / session handler 0.852s / router 1.000s）+ `go build` 通过；contracts 4 pass；task-office `*.test.ts` 63 pass；api-client 17 pass；mobile 105 pass/0 fail/2 skip（opt-in）；typecheck 通过 |
| `fe61df857`（#41） | 2 显性 + 2 隐藏语义冲突（`/inbox` 撞路径分流、`InboxItem` 撞名 barrel 别名） | tsx 30/30；mobile 116 tests（113 pass/3 skip opt-in）；typecheck exit 0；Go 3 组 `ok`（repository 2.057s / session 0.994s / handler 0.956s） |
| `21010717b`（T44） | 7 文件并集 | Go 2 组 `ok`（5.281s/1.375s）+ `go build` exit 0；task-office legacy 17 pass；api-client legacy 8 pass；mobile 119 pass/0 fail/4 skip；typecheck exit 0 |
| `cff3786fd`（T46） | 4 文件（barrel、composition、package.json exports、app-smoke 测试块重排） | Go 3 组 `ok`（2.301s/1.604s/0.342s）；material 28/28；materials+task-office 18/18；mobile 134 tests（129 pass/5 skip）；typecheck 无错误 |
| `40bbd82e5`/`8f74258ca`/`d58675a67`（#36/#42/#59） | **无集成报告**（merge 消息仅单行） | 集成验证未单独持久化；分支内任务级测试见各提交，随后 OCR B3 附录 B 全量轮（4.3）与 iOS 复验（4.4）覆盖合并后状态 |

### 4.3 OCR 修复批次验收（修复报告记载，全部实跑）

- **增量批次 2**（[fix-report-increment-batch2.md](ocr/fix-report-increment-batch2.md) 附录 B）：`go test ./internal/handler/session/ ./internal/application/repository/` 两包 ok；mobile-core 5 文件 84/84；apps/mobile 5 文件 50/50（含 typecheck 实跑通过）；api-client 2 文件 11 tests（10 pass/1 opt-in skip）。
- **最终轮 Round 1**（[fix-report-round-1.md](ocr/fix-report-round-1.md) 总览）：mobile-core 146/146；apps/mobile 72/72；domain 展示层 6/6；`tsc --noEmit` exit 0；`git diff --stat pnpm-lock.yaml` 空。
- **B3 增量轮**（[fix-report-increment-b3.md](ocr/fix-report-increment-b3.md) 附录 B，7 项验收）：
  - TS 全量：`pnpm exec tsx --test "packages/domain/src/mobile/*.test.ts" "packages/mobile-core/src/**/*.test.ts" "packages/api-client/src/mobile/*.test.ts" "apps/mobile/src/**/*.test.ts*"` → **518 tests / 509 pass / 0 fail / 9 skipped**（skip 均为 opt-in integration）
  - Go 四包：`go test ./internal/application/service/ ./internal/application/repository/ ./internal/router/ ./internal/handler/` → handler `ok`；service/repository/router 存在 FAIL（计数 **135/276/4**），与 `git stash` 后干净 HEAD 基线**完全一致**（预存在失败，见 7.2 迁移 000112 冲突）；session 包定向 20 个 `--- PASS`，全包 27 个 FAIL = 基线 27
  - `pnpm run typecheck:mobile` PASS；`typecheck:shared` 失败集合收窄为 `packages/views/src/chat/mermaid.ts` ×2（agent-adoption 清零，mermaid 为范围外预存在）
  - `requireDeploymentOrigin` 私有拷贝清零（共享 `deployment-origin.ts`，7 处适配器统一引用）
  - 6 项 high 用户可见症状逐条对应测试证据抽查通过

### 4.4 iOS 模拟器实测（iPhone 18 Pro / iOS 27.0 / Xcode 27）

- **首轮**（[ios-test-report.md](ios-evidence/ios-test-report.md) + [fix-report-round-1.md](ios-evidence/fix-report-round-1.md)）：preflight 全过；Release 构建第 5 轮 BUILD SUCCEEDED；iOS 27 强制 UIScene 导致的 SIGTRAP 经 scene 补丁修复；首屏 DeploymentLoginScreen 渲染成功；`weknora:///resources` warm+冷启动均导航到 Resources 屏（AX 证据）；修复轮 5/5 发现处置、mobile test 64/64（含 5 个新插件单测）、两次 Release 构建、三轮启动进程存活。
- **B3 复验**（[b3-recheck.md](ios-evidence/b3-recheck.md)，HEAD `13a761d2a`）：Release 增量构建 BUILD SUCCEEDED（约 105 秒）；安装/启动正常；7 条 B3 路由 deep link 全部可达且渲染正确未授权 gate 文案；冷启动直达 `/inbox`；正常重启回登录屏（deep link 不粘连）；启动日志 620 行无 SIGTRAP/fatal/崩溃/JS 异常。
- **B3 复验修复轮**（[b3-recheck-fix.md](ios-evidence/b3-recheck-fix.md)，基线 `01c9ccf5c`）：两项 minor 问题闭合——(1) MCP ios-simulator ui backend 误报 idb unavailable 的双层根因（launchd GUI 域 PATH 不含 `/opt/homebrew/bin`，实测复现 exit 127；以及上游 fb-idb 1.6.1 不支持 `idb --version` 的探测缺陷），落地 `launchctl setenv PATH` 修复（对当前运行中的调度进程需重启 ZCode 才生效，未执行）；(2) Mimosa `scanner_enobufs` 无结论 → 补跑深度扫描至完成并封印（scanId `scan-2026-09-24T12-27-16.425Z-f50edfe3ac10`，241 findings **全部为 B3 范围外既有静态发现，B3 范围 0 findings**；coverage 自评 partial/inconclusive 为静态分析固有边界）。定向测试 518/509/0/9 与 B3 基线完全一致；模拟器重复构建+启动+路由+冷启动验证全部通过。

### 4.5 未运行的检查（如实声明）

1. **opt-in 真实 HTTP 集成证据未在带凭据环境运行**：12 个 Issue 的真实 HTTP 证据用例（`WEKNORA_MOBILE_TEST_*` 环境变量门控）在本环境一律 skip（各集成记录与修复报告均有记载，如 mobile 105+2skip/116+3skip/119+4skip/134+5skip/518+9skip）。「具备真实 HTTP 集成证据」仅指证据代码与 skip 语义已落地并被测试钉住，不指凭据环境实跑。
2. **本报告撰写会话未重跑任何测试套件**：仅运行 git 历史核验（`git log`/`git branch`/`git worktree list`/`git status`）、文件实读与 `find`/`diff` 核验。
3. **PostgreSQL 迁移复跑**（#58 残余）与 **Android 侧任何验证**：从未在本轮运行（无环境）。
4. **Mimosa 钩子侧扫描始终未取得完整结论**：多次 commit 前 `scanner_enobufs`（含最终 `aa51eeeaf`，已在 `4f71e9d97` 记录第二次）——与 MCP 深度扫描（已完成封印）是不同执行路径，钩子侧从未通过。

---

## 五、审查、OCR 结论、修复轮次与并行波次

### 5.1 Superpowers SDD 审查链

- **派发前冲突扫描**：首批三份 Ledger 均含逐任务对接口/文件冲突扫描记录；t34 扫描判定 **blocking**（Ruling 5：T6 归一化自相矛盾、T8 surface 断言与 react-native 桩矛盾，Ledger 原文实测在案）——先修订计划再执行；t35 扫描记录两处接口一致性核对（Ruling 8）；B3 计划 t42/t59/t38/t36 各有扫描记录（Ruling 16/17/18/19，结论均为咬合一致、继续执行并在任务审查重点核对）。
- **计划审查**：#34 第 2 轮 7 项残留（Ruling 3）、#35 第 2 轮 4 项残留（Ruling 7）、#46 第 2 轮 9 项残留（Ruling 12）、#59 第 2 轮 3 项残留（Ruling 13）、#36 第 2 轮 1 项残留（Ruling 14）、#42 第 2 轮 6 项残留（Ruling 15）——均作为 Review Focus 传入任务审查，未阻塞执行。
- **任务级审查**：各计划任务在分支内含 fix round 提交（如 t38 `bbbec33d9`/`a9284a599`、t44 `ca2f50d90`、t59 `c75fa6e06`、t46 `935cb1e9d`、t42 `5ed587aaf` 等，见 3.2 分支提交列表中标注 review fix 的提交）。
- **最终审查（final review）**：#33/#34/#35/#66/#36/#38/#41/#42/#44/#46/#59 通过（finalApproved=true，其中 #36/#41/#59 的最终审查 F1 修复提交为分支 tip：`8c7d71134`/`c3076ce20`/`a52207942`；#32/#33 的最终修复报告持久化于 `t32/`、`t33/` 目录）；**#32 修复波后仍有 1 项 important 发现**（IPv6 绕过），已实测 ADDRESSED 但按 SDD 规则不再开第二波，残留呈报（Ruling 4）。
- **task-brief 提取失败 6 次**（Ruling 20–25，涉及任务 5/6/7 × 两批计划）：实现者直接读计划文件对应任务节替代。

### 5.2 OCR 四阶段

| 阶段 | 扫描 | 处置 |
|---|---|---|
| 第二批增量（波次 2 后） | complete：46 findings / 29 items | 10 任务修复（10 提交）→ fix-report-increment-batch2；B2-F12（snapshot 属主错配 P0）等纳入；B2-F4/F6/F11/F23/F28/F35/F46 延期（附录 A） |
| 最终轮（波次 2 收尾） | **partial**：50 findings；28/62 items 因 429 限流失败 | 13 任务修复（13 提交）→ fix-report-round-1：23/24 有效发现修复 + R1-F26 领域决策排除；18/20 lowWorth 修复；R1-F17/R1-F20 部分修复（Round 2 范围） |
| B3 增量（波次 3 后） | complete：88 findings / 95 items（retry report：440 请求 16 受影响，5 失败/11 恢复） | 15 任务修复（15 提交）→ fix-report-increment-b3：6 项 high 全部修复（F85/F82/F84/F83/F78/F64/F79/F65/F51/F38/F42/F37/F41/F40/F59/F39/F44/F62 对应任务均 GREEN）；附录 A 延期项 F53/F47/F70/F71 等未动；4 项计划外发现如实记录（goalKey 记忆表、grep 口径、F48/F18 无对应物） |
| 重扫 | **未执行**（用户指示最终轮缩减为 1 轮，Ruling 10；B3 轮单轮封顶，Ruling 26） | 三轮修复增量均未被二次 OCR 覆盖，以各修复报告内定向复审与全量回归兜底 |

**OCR 覆盖缺口（如实）**：最终轮 28/62 selected items 因 429 限流未获审查（ocr-round-1.md 尾部列明失败文件组）；B3 增量轮按 Ruling 26 有 2 项未决（报告尾部 retry report 列出 5 个失败请求的文件组）；`ocr-ledger.md` 干净范围台账从未建立（第二节）。

### 5.3 并行波次与集成记录

- **波次 1**（Ruling 2）：#32→#33→#34 同 DAG 批次、计划文件范围不重叠，独立 worktree 并行实施、完成后按序集成——落为集成分支线性串行段（首批 Ledger 头部「分支：codex/issue30-mobile-office」+该区段无 merge 提交实测）。
- **波次 2**（Ruling 6）：#35 与 #66 同批次**全并行**（不再因 filesTouched 重叠降级串行），独立分支；集成 t35 先（`979762df2`）、t66 后（`417b7dedf`，3 文件冲突手工合并，集成验证 71 tests + mobile 40/40 + typecheck，[integrate-t66.md](../../.superpowers/sdd/integrate-t66.md)）。
- **波次 3 / B3**（Ruling 11）：#36/#38/#41/#42/#44/#46/#59 七节点全并行（独立 worktree 与分支），按序集成 7 个 merge。四份集成报告在册：
  - **t38**：5 冲突文件全部「并集」解决（task-office ports/错误码/api-client/测试尾部/app-smoke 按钮断言数组按合并后 HomeScreen 实际渲染顺序重排）；
  - **t41**：**产品级撞路径**——#38（T08 审批复盘）与 #41（行动通知）两计划原文都指定 `/inbox`；集成裁量 `/inbox` 归 #41（其测试行为绑定该路由），T08 迁 `/attention`（新建 `app/attention.tsx`），HomeScreen 去重后新增 'Open Approvals' 入口；另修 barrel 撞名（`AttentionInboxItem`/`AttentionInboxView` 别名）；
  - **t44**：7 冲突文件并集（Go 容器/路由两侧 provider 共存等）；
  - **t46**：4 冲突文件（api-client package.json exports 并列、app-smoke 测试块重排 + fs-import 归属）。
  - t36/t42/t59 三个 merge 无集成报告（见 1.1/二节如实声明）。
- **OCR/iOS 修复波**：与实现串行收尾；波次 2 收尾时发生过一次并行进程代提交（`f7753fa16`，内容逐字一致，fix-report-round-1 偏离 #10 记录）。

---

## 六、裁决记录（全部 Ruling，含判断错误时的代价）

1. **模型路由替代**：用户指定 gpt-5.6-sol/terra/luna 分级路由，但本运行环境子代理只能运行会话模型（GLM-5.3），全部子代理实际使用会话模型；架构与审查类任务通过独立 fresh-eyes 双代理交叉制衡补偿。代价：关键决策深度不足，可由人工复核最终报告发现。
2. **执行并行化（用户指示）**：同一 DAG 批次且计划文件范围不重叠的计划并行实施，各自使用独立 worktree 与分支，完成后按序 merge 集成；计划内部任务保持串行（SDD 同文件冲突规则）。代价：merge 顺序决定最终提交拓扑，冲突时需集成修复轮。
3. **计划 #34 审查第 2 轮未全通过（7 项残留）**：残留项作为任务审查的额外关注点传入 Review Focus。代价：实现阶段可能暴露这些缺口并触发修复轮。
4. **#32 最终审查修复波后仍有残留**（important：`disallowedDeploymentHost` 可被 IPv6 形式绕过 → ADDRESSED：实测 `mobileRuntimeIntegrationConfig` 对 `https://[fe80::1]`、`[fc00::1]`、`[fd12:3456:789a::1]`、`[::ffff:7f00:1]` 全部返回 `enabled:false/disposition:'invalid'`，运行报告同款 tsx 复现脚本；hostname 含 `:` 时经 `parseIpv6Literal` 展开为 16 字节，`runtime-integration-smoke.ts:45-75`）：按 SDD 规则不再有第二波，残留呈报用户。代价：该 Issue 不能宣称完全干净。
5. **计划 #34 冲突扫描判定 blocking**（阻塞①·T6 自洽：Task 6 测试断言 `search:'quarterly review'` 与 `normalizeQuery` 仅 `search.trim().slice(0,200)` 矛盾——实跑 `node -e` 验证 `'   quarterly   review '.trim()` 保留内部多空格 ≠ 期望，Step 4「全部 PASS」按计划原文不可能达成，须先修订（折叠内部空白对齐 Go 侧 `strings.Fields` 语义或改测试期望）；阻塞②·T8 自洽：断言过滤 `type==='Text'` 检查 `'View all tasks'`，但该文案是 Button 的 title 属性（`app-smoke.test.tsx:23、:60-68` 实读：react-native 桩把 Button 渲染为 `{type:'Button',props:{title}}` 无 Text 子节点），断言必为 false）——已按扫描结论先修订再执行并在任务审查中重点核对。代价：可能触发额外修复轮。
6. **第二批并行实施（用户指示）**：#35、#66 与首批同一套 SDD 流程，但同 DAG 批次节点全部并行（不再因 filesTouched 重叠降级串行），各自独立 worktree 与分支，重叠文件的合并冲突由集成修复员按双方意图解决。代价：merge 冲突概率上升、集成修复轮可能增加；若集成失败该节点如实记受阻。
7. **第二批计划 #35 审查第 2 轮未全通过（4 项残留）**：残留项传入任务审查关注点。代价：实现阶段可能触发修复轮。
8. **计划 #35 冲突扫描判定 blocking**（T1→T2：`(*WorkbenchListStore).ReadTaskFactsForRun` 签名与 T2 `OwnedTaskFactsReader` 接口/容器 `.WithTaskFacts(lists)` 消费一致（`workbench_list.go:135`、attentionOf `:49-58`、stub 强制 owner 见 `workbench_read_test.go:31-35` 核实）；T2→T3→T8：`TaskSnapshotFacts` snake_case wire 形状（`omitempty`）与 T3 解析器、T8 detail 映射一致（`contracts.go:52-58`、`executions.ts:174/282` 核实））——已按扫描结论继续执行并在任务审查中重点核对。代价：可能触发额外修复轮。
9. **第二批增量 OCR 有 1 项未决**：单轮封顶不再重扫，随最终全量轮与报告核销。代价：个别问题可能带入最终交付。
10. **OCR 最终轮缩减为 1 轮（用户指示）**：本批发现修复并定向复审后不再重扫。代价：修复增量未被二次 OCR 覆盖，以复审裁决与最终报告兜底，残留如实呈报。
11. **第三批 B3 并行实施（用户指示）**：#36、#38、#41、#42、#44、#46、#59 前置已全部满足，同批次 7 节点全部并行（独立 worktree 与分支），重叠文件冲突由集成修复员按双方意图解决；批后增量 OCR（干净则记台账首条）与条件性 iOS 复验。代价：merge 冲突链较长、运行时长与配额消耗显著。
12. **B3 计划 #46 审查第 2 轮未全通过（9 项残留）**：残留项传入任务审查关注点。代价：实现阶段可能触发修复轮。
13. **B3 计划 #59 审查第 2 轮未全通过（3 项残留）**：残留项传入任务审查关注点。代价：实现阶段可能触发修复轮。
14. **B3 计划 #36 审查第 2 轮未全通过（1 项残留）**：残留项传入任务审查关注点。代价：实现阶段可能触发修复轮。
15. **B3 计划 #42 审查第 2 轮未全通过（6 项残留）**：残留项传入任务审查关注点。代价：实现阶段可能触发修复轮。
16. **计划 #42 冲突扫描判定 blocking**（T1↔T2：T1 产出 `repository.TaskGrantStore` 五方法与 `types.TaskGrantRole/TaskAccess`（plan:101-106），T2 的 `TaskGrantStorePort` 与服务逐字消费同一组签名（plan:826-832），接口完全咬合；T1↔T3：T3 容器 wiring 以 `*repository.TaskGrantStore` 为第一参数构造 `TaskGrantService`（plan:1369-1375），与 T1 `NewTaskGrantStore(db)` 返回类型吻合）——已按扫描结论继续执行并在任务审查中重点核对。代价：可能触发额外修复轮。
17. **计划 #59 冲突扫描判定 blocking**（Task1→Task2：任务1 产出三实体与 sqlite 000112 迁移，任务2 经 `openRunTestDB` 消费（`agent_run_test.go:29-61` 实测跑全量 migrations/sqlite 流会包含新 000112），实体字段齐备，一致；Task1→Task3：任务3 服务测试经 `openAgentVersionServiceTestDB` 依赖任务1 新表（`agent_version_test.go:31-63` 同样应用全量迁移流），一致）——已按扫描结论继续执行并在任务审查中重点核对。代价：可能触发额外修复轮。
18. **计划 #38 冲突扫描判定 blocking**（T1→T2：T1 产出 `Service.ListInbox`/`GormInteractionStore.ListPending` 与 `InteractionDecision.CreatedAt`（plan-t38.md:81），T2 的 `ListInboxInteractions` 调用 `h.interactions.ListInbox` 并断言 wire 行携带 `created_at`（plan:579,420）——与现有代码（`service/workbench/interaction.go:416`、decision `:151`、identity `:438`、结构 `:26-36`）衔接一致，无冲突；T2→T4：T2 产出 GET /api/v1/workbench/interactions 路由（plan:605-607；`RegisterWorkbenchCommandRoutes` 在 `routes_workbench.go:159-170`、v1 挂载 `router.go:374`）与 502+command_recovery_unknown 错误 wire（plan:591-597，插入点 `workbench_commands.go:99-124` 属实），T4 的 interactions.ts 路径与 409/400→SUPERSEDED、502→DELIVERY_UNKNOWN、404/403/410→GONE 分类消费同一 wire（plan:903,991-993）——逐字一致，无冲突）——已按扫描结论继续执行并在任务审查中重点核对。代价：可能触发额外修复轮。
19. **计划 #36 冲突扫描判定 blocking**（T1→T2：Task 1 产出 coordinator.resume（对象字面量内已有 `this.reconcile` 先例，`submission.ts:124`），Task 2 的 office 经 submissions.resume 消费——`SubmissionStore.load/save`（`submission.ts:52-56`）与 `SubmissionConflictError(requestId, storedDigest, incomingDigest)`（`submission.ts:58-60`）同 resume 实现的调用完全匹配，无冲突；T1→T5：Task 1 产出 recommendLeadAgent 并在 domain index.ts 追加 export 行，Task 5 从 `@weknora/domain/mobile` 导入——index.ts 为 export * 聚合（`packages/domain/src/mobile/index.ts:1-15`），导出链路成立，无冲突）——已按扫描结论继续执行并在任务审查中重点核对。代价：可能触发额外修复轮。
20. **task-brief 脚本提取任务 5 失败**：实现者直接读计划文件对应任务节。代价：实现者上下文略宽。
21. **task-brief 脚本提取任务 6 失败**：实现者直接读计划文件对应任务节。代价：实现者上下文略宽。
22. **task-brief 脚本提取任务 7 失败**：实现者直接读计划文件对应任务节。代价：实现者上下文略宽。
23. **task-brief 脚本提取任务 5 失败**（第二批计划）：实现者直接读计划文件对应任务节。代价：实现者上下文略宽。
24. **task-brief 脚本提取任务 6 失败**（第二批计划）：实现者直接读计划文件对应任务节。代价：实现者上下文略宽。
25. **task-brief 脚本提取任务 7 失败**（第二批计划）：实现者直接读计划文件对应任务节。代价：实现者上下文略宽。
26. **B3 增量 OCR 有 2 项未决**：单轮封顶不重扫、不记台账，随最终报告呈报。代价：个别问题可能带入最终交付。

---

## 七、未完成节点、遗留风险、延期项与需用户决策

### 7.1 未完成 / 阻塞节点

- **27 个下级 Issue 未实施**（B4–B8 全部节点，见 1.2）；#31 验收开放（staging 凭据 + Android 证据外部阻塞）。
- **授权面端到端验证缺口**：12 个 Issue 的授权交互与全部 opt-in 真实 HTTP 集成证据未在带凭据环境运行（本环境无 deployment 凭据/后端，且按约束不得写入凭据字面量）。iOS 实测仅覆盖：构建/安装/启动/首屏/未授权 gate 文案路由可达性/冷启动 deep link。
- **B3 三个 merge（#36/#42/#59）无集成报告**；plan-t35/t66 与 B3 全部 7 份计划的 `progress.md` 未持久化——任务级审查过程记录缺失，只能以提交与后续全量轮替代。
- **`ocr-increment-b3.md` 未提交入库**（worktree 未跟踪文件，本次报告提交亦未纳入，属后续动作）。

### 7.2 遗留风险

- **migrations/sqlite 000112 序号冲突（预存在，未解决）**：`000112_task_grants.*` 与 `000112_agent_adoption_variants.*` 并存（merge `d58675a67` 引入），任何走全量迁移轨道的 Go 测试夹具打开迁移源即报 `duplicate migration file`。B3 修复轮实测失败计数 service 135 / repository 276 / router 4 / session 27 与干净 HEAD 基线**完全一致**（未扩大、非本轮 OCR 修复引入），但该冲突使 Go 全量迁移轨道持续不可用；修复报告明示「序号重编应升级为独立决策，不顺手修改」。**这是当前最需要用户裁决的技术债**。
- **Mimosa 预提交钩子始终未通过**：多次 `scanner_enobufs`（含 B3 修复轮与最终 iOS 修复轮提交，第二次已在 `4f71e9d97` 记录）。已有替代证据：MCP 深度扫描完成并封印（B3 范围 0 findings、241 条范围外既有发现未处置、coverage partial/inconclusive）——本轮不宣称项目安全，完整审计需另行重跑。
- **OCR 覆盖缺口**：最终轮 28/62 items 因 429 限流未审；B3 增量 2 项未决（Ruling 26）；三轮修复增量（13+10+15 提交）均未被二次 OCR 覆盖（Ruling 10）。
- **OCR 干净范围台账未建立**：`ocr-ledger.md` 不存在（第二节），「重跑只审新增量」的续审机制缺位——后续若再跑 OCR，需先人工确定基线提交区间，否则只能全量重扫。
- **#32 残留项未经第二波审查**：IPv6 绕过的修复证据是「实测 ADDRESSED」（t32/final-fix-report.md 内含 RED→GREEN 与 25 URL 回归），未走独立审查波次（Ruling 4）。
- **iOS Fabric 白屏根因未修**：以 `newArchEnabled:false` 规避（RN 0.83.10 + iOS 27.0 runtime 组合问题，上游超本仓范围）；RN 编译期 `-DRCT_NEW_ARCH_ENABLED` 仍作用于源码构建 pods；升级 Expo/RN 后需复测再翻转。
- **Expo 模板锚定**：`ios-xcode27` 插件锚定 SDK 55 模板原文，Expo 升级致模板漂移时 prebuild 显式报错（有单测覆盖），需同步更新锚点。
- **MCP ios-simulator ui backend 不可用**：层 A（launchd PATH）修复需重启 ZCode 才进入 MCP 进程（未执行，会终止工作流）；层 B（fb-idb 1.6.1 不支持 `idb --version` 的探测缺陷）在 ZCode 应用包内，需上游修复；跨重启持久化 `sudo launchctl config user path` 需 sudo，未执行。idb CLI 直调为当前 AX 校验替代路径。
- **Metro/localhost 联调受阻**：宿主 VPN/TUN 全局代理（127.0.0.1:17890）拦截模拟器 localhost 流量，未改动用户网络环境（首轮 iOS 报告 3.2/第 7 节）。

### 7.3 口径记录（如实呈报）

- **#35 任务计数**：编排器本轮统计 11/11，与 plan-t35.md 的 11 任务、t35 分支 14 提交（11 任务各≥1 提交 + 2 review 提交 + 1 typecheck 修复）一致——中途版报告曾记录的「6/6 vs 11」口径不一致已在本轮统计中收敛，以计划文件与提交证据为准（11 任务全部落地）。
- **t66 计划验证命令预期值与实跑差异**：计划预期「既有 33 + 新 14 + 改写 1」，集成时实跑 71 tests（6 文件，含 2 skipped）——数量口径随合并演进，结果全绿（`417b7dedf`）。
- **Go 失败计数基线口径**：B3 附录 B 的「失败集合不扩大」以 `git stash` 干净 HEAD 对比（135/276/4/27），非零失败即预存在状态（000112 冲突族 + 范围外既有），非本轮引入。

### 7.4 延期低优先级事项（计划明示、未实现）

- OCR Round 2 范围：SQLite 持久 TaskProjectionStore（R1-F20 完整版 / B2-F23，需存储选型 ADR）、R1-F17 行存储后端（当前 `VAULT_ROW_TOO_LARGE` 受控失败）。
- B3 增量轮附录 A 延期项：F53/F47/F70/F71 等（理由见 [ocr-fix-increment-b3.md](plans/ocr-fix-increment-b3.md) 附录 A）；计划外处置记录（goalKey 记忆表仅内存、F48/F18 无对应物）见该修复报告「计划外发现与处置」节。
- 按计划排除项：R1-F26（领域决策，未动 CONTEXT.md）、R1-F1（证据行号漂移无法定位）、R1-F28（独立重构）；B2-F4/F6/F11/F28/F35/F46（理由见 ocr-fix-increment-batch2.md 附录 A）。
- #58 PostgreSQL 迁移复跑（000187/000188，环境性）。
- Mimosa 深度扫描的 241 条 B3 范围外静态 findings 的逐条处置。

### 7.5 需要用户决策

1. **模型路由替代**：本轮全部子代理实际使用会话模型 GLM-5.3（Ruling 1）——请人工复核本报告与关键决策（尤其 #32 残留修复、两处计划 blocking 修订、t41 `/inbox` 撞路径集成裁量、t42 approve 门禁升级裁决）以补偿路由深度缺口；如需原定 gpt-5.6 分级路由，需在支持该路由的环境重跑关键审查。
2. **migrations/sqlite 000112 序号冲突处置**：是否授权独立任务重编迁移序号并恢复 Go 全量迁移轨道（当前 135/276/4/27 预存在失败被基线口径掩盖但持续存在）。
3. **#32 残留处置**：接受实测 ADDRESSED 证据（t32/final-fix-report.md 的 RED→GREEN 与 25 URL 回归）并视为关闭，还是安排一次独立复审波次。
4. **OCR 台账与补扫**：是否补建 `ocr-ledger.md`（以本轮 `4f71e9d97` 为 CLEAN 基线起账）；是否对 28 个限流未审 items、B3 的 2 项未决与三轮修复增量补跑 OCR。
5. **授权面验证**：何时提供 `WEKNORA_MOBILE_TEST_*` 凭据环境（官方云或自托管 staging）以运行 12 个 Issue 的真实 HTTP 集成证据与 iOS 授权面交互。
6. **下一批次**：B4 的 11 个节点（#37/#39/#40/#43/#45/#48/#52/#56/#60/#67/#68）前置已全部满足（1.2 逐节点核对），是否启动及其并行策略；#48 的 NOTION_TOKEN 环境性阻塞是否先行解决。
7. **分支去向**：`codex/issue30-mobile-office`（及 9 条 t* 并行分支）是否推送远端、开 PR、合并 `main`；对应 12 个 GitHub Issue 的关闭时机；未跟踪文件 `ocr-increment-b3.md` 是否补提交。
8. **iOS 环境后续**：是否重启 ZCode 使 launchd PATH 修复进入 MCP 进程；是否执行 `sudo launchctl config user path` 持久化；上游 idb 探测缺陷的跟进。
9. **安全审计**：Mimosa 钩子侧 enobufs 从未通过——是否安排一次完整的项目级深度安全扫描并处置 241 条范围外发现。

---

## 交付声明

本轮全部成果（**182 个提交**，`29c1e5635..4f71e9d97`：12 个 Issue 的实现（含 Go 后端与移动端全栈）、三个并行波次共 9 个集成 merge、三轮 OCR 修复共 38 个修复提交、两轮 iOS 实测与修复、全部文档证据）均为**本地 worktree 分支 `codex/issue30-mobile-office` 上的本地提交**：**未推送远端、未合并到 `main` 主分支、未关闭任何 GitHub Issue**（#30–#71 全部保持原 open 状态，#58 维持既有 closed）。本报告主仓库副本（`/Users/wuyongjun/trea/WeKnora-fork01/docs/plans/issue30-sweep/FINAL-REPORT.md`）仅为文件落盘，不改变主仓库 git 状态。
