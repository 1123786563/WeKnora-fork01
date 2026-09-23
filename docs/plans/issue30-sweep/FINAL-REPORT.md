# Issue #30 本轮交付最终报告（issue30-sweep）

- 报告日期：2026-09-24
- Worktree：`.worktrees/issue30-sweep`，分支 `codex/issue30-mobile-office`，HEAD `3f72d96db`
- 提交范围：`29c1e5635..3f72d96db`（共 83 个提交，本报告第三节附完整关键提交清单）
- 本轮实施范围：#30 的 41 个下级 Issue 中实施 **5 个**（#32、#33、#34、#35、#66，对应 T02、T03、T04、T05、T36）
- 状态口径声明：以下所有「完成 / 已验证完成」均指**本地 worktree 分支上的交付与本地审查结论**；未推送远端、未合并到 `main`、未关闭任何 GitHub Issue（详见文末声明）。

---

## 一、下级 Issue 状态与 #30 总体验收覆盖

### 1.1 本轮实施的 5 个 Issue

编排器执行统计（原样呈现）：

| Issue | T | 任务（编排器口径） | 最终审查（finalApproved） | 状态判定 |
|---|---|---|---|---|
| #32 | T02 | 6/6 完成 | **false**（1 项残留，见下） | **完成（含 1 项最终审查残留呈报，不宣称完全干净）** |
| #33 | T03 | 6/6 完成 | true | 已验证完成（本地审查口径） |
| #34 | T04 | 9/9 完成 | true | 已验证完成（本地审查口径） |
| #35 | T05 | 6/6 完成（口径存疑，见注） | true | 已验证完成（本地审查口径；任务计数口径不一致，见 7.3） |
| #66 | T36 | 6/6 完成 | true | 已验证完成（本地审查口径） |

**各 Issue 目标与交付摘要**（目标引自编排器本轮计划结果，交付证据引自 Ledger/提交）：

- **#32（T02）Active Tenant 切换与 Scoped Vault 隔离**：用户可在移动端选择并切换 Active Tenant；新建 Scoped Vault 深模块让加密缓存/草稿按 Deployment×用户×Tenant 隔离，切换/退出/换部署时旧 scope lease 同步撤销、wrapped key 轮换+行擦除（fail closed），迟到响应经 epoch 拒绝；租户切换具备真实 HTTP 最高稳定 Interface 的 opt-in 集成证据形态。本轮已按独立审查结论修复全部 8 项发现并补充部署 URL host 防线。计划 gate PASS（[Ledger](../.superpowers/sdd/plan-t32/progress.md)）。**残留**：最终审查发现（important）「`disallowedDeploymentHost` 可被 IPv6 形式绕过」——已实测 ADDRESSED：`mobileRuntimeIntegrationConfig` 对 `https://[fe80::1]`、`[fc00::1]`、`[fd12:3456:789a::1]`、`[::ffff:7f00:1]` 全部返回 `enabled:false / disposition:'invalid'`（运行报告同款 tsx 复现脚本；hostname 含 `:` 时经 `parseIpv6Literal` 展开为 16 字节，实现见 `apps/mobile/src/runtime-integration-smoke.ts:55-80`，本轮报告撰写时实读核实该函数存在且含 zone-id 拒绝与 `::ffff:` 映射展开）。按 SDD 规则最终审查后不再有第二波修复，该项作为残留呈报用户，故 #32 不宣称完全干净、finalApproved=false。
- **#33（T03）Resource Shelf**：移动端新增 Resources 资源页，经新建 mobile-core Resource Shelf 深 Module（browse/selection/subscribe）端到端展示当前 Active Tenant 的 Available Agent、知识与 Connection 摘要并解释三态；撤权（403）或 scope/capability 变化后投影立即失效，Screen 只消费 Resource Shelf Interface。计划 gate PASS（[Ledger](../.superpowers/sdd/plan-t33/progress.md)）。
- **#34（T04）首页 Attention 与统一 Task 列表**：首页一次聚合读展示「需要我处理/正在运行/最近完成」三段视图，统一 Task 列表支持搜索、筛选与归档（含归档写路径），全部读自当前 Tenant 的授权聚合读模型，经新建 Task Office 深模块 + opt-in 真实 HTTP 集成证据形态端到端验证。计划 gate PASS（[Ledger](../.superpowers/sdd/plan-t34/progress.md)，含 Go 侧测试）。计划审查第 2 轮曾有 7 项残留，作为 Review Focus 传入任务审查后在实现阶段吸收。
- **#35（T05）Task 详情 Snapshot、Timeline 与 SSE 恢复**：打开 Task 得到结果优先详情、Task/Run/Attention 三层状态与规范 Timeline，经 Snapshot 水合、游标 SSE、缺口/裁剪补洞与有界重同步在 Task Office Interface 完成 App 重启、断线与终态 drain 的可验证恢复。独立审查四项发现已修复（阻断项：Task 9/10 装配补传 detail port 并增加冒烟源级守卫；三项低级：行号锚点实测修正、循环导入警示、app-smoke stub 扩展）。**注意**：持久化计划文件 [plan-t35.md](plans/plan-t35.md) 含 11 个任务（Task 1–11），t35 分支 14 个提交覆盖全部 11 任务；编排器统计记录 6/6——两处口径不一致，如实呈报（见 7.3）。
- **#66（T36）多 Deployment 切换与兼容性降级**：一台设备可登记多个官方云/自托管 Deployment 并在实例间原子切换（不携带旧凭据、不接纳迟到响应），部署缺少安全关键 capability 时进入「说明 + 有限只读」降级面，经 MobileRuntime Interface 测试 + opt-in 真实 HTTP 多实例切换证据形态端到端验证。

**iOS 模拟器实测对上述交付的覆盖**（[ios-test-report.md](ios-evidence/ios-test-report.md)、[fix-report-round-1.md](ios-evidence/fix-report-round-1.md)）：Release 构建/安装/启动/首屏渲染 ✅（修复轮后）；`weknora:///resources` deep link warm + 冷启动导航 ✅（到达未授权 Resources 提示文案）；**#32/#34/#35/#66 的授权面交互 ❌ 未验证**——本环境无 deployment 凭据与后端（详见第四节、第七节）。

### 1.2 其余 36 个下级 Issue 的 DAG 状态与现状

依据 [dag.md](dag.md)（2026-09-23 生成时点的调查快照；本轮实施后其状态总表未回写，下表「现状」列为报告撰写时按本轮交付更新的口径）：

| 分组 | Issue | DAG 批次 | 现状 |
|---|---|---|---|
| 前期已完成（非本轮） | #31（T01 登录 Deployment） | B0 | 实现已集成分支（feature/mobile-office@0c5a6bdc），验收开放：本轮 iOS 修复轮已解决 iOS 27 scene 生命周期/冷启动深链/构建问题，但 HTTPS staging 登录/OIDC 真实凭据与 Android 设备证据仍外部阻塞（blocked-external） |
| 前期已关闭 | #58（T28 Catalog Release） | -（done-evidenced） | 2026-09-20 关闭；残余缺口仅 PostgreSQL 迁移复跑（环境性，不重开） |
| **本轮已实施** | #32（B1）、#33/#34/#35/#66（B2） | B1、B2 | 见 1.1；**B1、B2 两批次全部节点已在本轮交付** |
| 未实施（B3） | #36、#38、#41、#42、#44、#46、#59 | B3 | open、无实现痕迹。**本轮后其全部前置已满足**（#36←#33✅#35✅；#38←#34✅#35✅；#41←#32✅#34✅#35✅；#42←#34✅#35✅；#44←#34✅#35✅；#46←#35✅；#59←#33✅且 #58 已满足）——即 B3 的 7 个节点现已就绪可并行启动 |
| 未实施（B4） | #37、#39、#40、#43、#45、#48、#52、#56、#60、#67、#68 | B4 | open、未实施（其中 #48 有 NOTION_TOKEN 类环境性阻塞、#41 相关推送验收外部阻塞，见 dag.md 第 8 节） |
| 未实施（B5） | #47、#49、#50、#51、#53、#54、#57、#61、#62、#69、#70 | B5 | open、未实施（#69/#70 安装包验收 blocked-env：签名/真机缺失） |
| 未实施（B6–B8） | #55、#63、#64（B6）；#65（B7）；#71（B8） | B6–B8 | open、未实施；#71 为跨平台发布证据矩阵，是 #30 首版验收的收口节点 |

**#30 总体验收覆盖情况**：#30 的完成定义是 41 个下级 Issue 全部交付并以 #71 证据矩阵收口。本轮覆盖 **5/41**；加上前期 #31（验收开放）与已关闭 #58，尚有 **34 个未实施**。#30 远未达到总体验收状态，本轮属于 DAG 关键路径 B1+B2 批次的纵向推进。

---

## 二、交付物路径清单

以下路径相对仓库根（worktree `.worktrees/issue30-sweep`）；从本报告所在目录（`docs/plans/issue30-sweep/`）出发的相对链接已可直接点击。

### 层级树 / DAG / 需求

- 层级树与 41 个子 Issue 清单：[issues/index.md](issues/index.md)（单 Issue 详情 `issues/issue-30.md` … `issues/issue-71.md`）
- 依赖 DAG（92 边、9 批次、无环验证、状态总表）：[dag.md](dag.md)

### 实施计划（5 份）

- T02：[plans/plan-t32.md](plans/plan-t32.md)（6 任务）
- T03：[plans/plan-t33.md](plans/plan-t33.md)（6 任务）
- T04：[plans/plan-t34.md](plans/plan-t34.md)（9 任务）
- T05：[plans/plan-t35.md](plans/plan-t35.md)（11 任务；编排器统计口径 6，见 7.3）
- T36：[plans/plan-t66.md](plans/plan-t66.md)（6 任务）

### SDD Ledger（`.superpowers/sdd/`，git 未跟踪的执行台账）

- plan-t32：`.worktrees/issue30-sweep/.superpowers/sdd/plan-t32/progress.md`（含派发前冲突扫描、逐任务 complete 记录、计划 gate PASS）
- plan-t33：`.worktrees/issue30-sweep/.superpowers/sdd/plan-t33/progress.md`
- plan-t34：`.worktrees/issue30-sweep/.superpowers/sdd/plan-t34/progress.md`
- plan-t35 / plan-t66：**progress.md 未持久化**——两计划在并行 worktree 执行，worktree 已清理且 Ledger 从未提交入库（全仓库 `find` 检索无 `plan-t35`/`plan-t66` 目录，本报告撰写时核实）。其执行证据以提交本身与集成 merge 记录（`979762df2`、`417b7dedf`）替代。

### iOS 模拟器实测证据

- 测试轮报告：[ios-evidence/ios-test-report.md](ios-evidence/ios-test-report.md)（截图 `01-first-screen.png`…`09-resources-route.png`、日志 `app-launch-log.txt`、`final-run-keylog.txt`、`xcodebuild-release.log`、`xcodebuild-debug.log`、`metro.log`）
- 修复轮报告：[ios-evidence/fix-report-round-1.md](ios-evidence/fix-report-round-1.md)（证据目录 [ios-evidence/fix-round-1/](ios-evidence/fix-round-1/)：修复后首屏 ×3、warm/cold deep link 截图与 AX 标签、构建/启动日志）

### OCR 报告与修复计划

- 第二批增量 OCR：[ocr/ocr-increment-batch2.md](ocr/ocr-increment-batch2.md)（Review complete：46 findings / 29 selected items）
- 第二批增量修复报告：[ocr/fix-report-increment-batch2.md](ocr/fix-report-increment-batch2.md)（10/10 任务）
- 最终轮 OCR：[ocr/ocr-round-1.md](ocr/ocr-round-1.md)（Review **partially** complete：50 findings；28/62 selected items 因 LLM 429 限流审查失败）
- 最终轮修复报告：[ocr/fix-report-round-1.md](ocr/fix-report-round-1.md)（13/13 任务）
- 修复计划：[plans/ocr-fix-increment-batch2.md](plans/ocr-fix-increment-batch2.md)、[plans/ocr-fix-round-1.md](plans/ocr-fix-round-1.md)
- **OCR 干净范围台账 `ocr/ocr-ledger.md`：不存在。** 任务输入将其列为既有材料（每行 `CLEAN <sha> | 轮次 | 报告`，作为「重跑只审新增量」的续审依据——理想用法是：下一轮 OCR 只扫描上轮 CLEAN 基线之后的新增提交，CLEAN 行锚定的 sha 即为各轮次重扫起点），但本报告撰写时在 worktree、主仓库全树 `find` 与 git 历史（`git log --all -- "**/ocr-ledger*"`）中均未找到该文件（唯一同名文件位于另一 Issue 的 worktree `.worktrees/issue106/.superpowers/sdd/2026-09-23-issue-106/ocr-ledger.md`，与本轮无关）。因此**本轮 OCR 的续审基线未落账**，后续重跑只能全量或以提交区间人工界定，详见 7.2。

---

## 三、Worktree、分支与提交拓扑

### 3.1 Worktree 与分支

- 集成 worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep`，分支 `codex/issue30-mobile-office`（`git worktree list` 核实）
- 并行分支（worktree 已清理、分支仍在）：`codex/issue30-t35`（指向 `2151425c4`，14 个提交全部经 `979762df2` 并入）、`codex/issue30-t66`（指向 `75dcdeaab`，6 个提交全部经 `417b7dedf` 并入）
- 基线：`29c1e5635`；最终 HEAD：`3f72d96db`；范围共 **83 个提交**（`git log --oneline 29c1e5635..3f72d96db | wc -l`）

### 3.2 提交拓扑（按时间序，关键节点）

```
29c1e5635（基线）
├─ 16dddb2d5  docs: collect issue tree and inventory        ┐
├─ 387459617  docs: build issue dependency dag              │ 规划产物
├─ 74ab7c856  docs: plans for 32                            │
├─ dfe5e0f82  docs: plans for 33,34                         ┘
├─ #32 六任务（串行落在集成分支）：3d3fe3db0 → 406df4c1e → f532c3fc1 →
│   c77a419c6 → 947fe12c3 → 6fcd7d472（+ 4993dcdd3 host 防线补丁）
├─ #33 六任务：470fd8181 → ea12dc883 → 7cb820415 → 3423cbde7 → f4f51d314 →
│   2a27ef739 → 54f8de97b → 2a7691bd0（+ 766b64577 最终审查清扫）
├─ #34 九任务：49cb2dfbf → 9fdda6efe → 11046e68d → 99f24103a → 7418e1b56 →
│   9ad9c2f98 → a1a6de115 → 0561c5ff3 → 8733e7bc9 → 028f72b11（T04 AC3）
├─ cfccdf6a4  docs: second-batch plans for 35,66
├─ 并行波次 2（同 DAG 批次全并行、独立 worktree/分支）：
│   ├─ codex/issue30-t35（14 提交）：63411e5c5 → f6cf87307 → 95c804a87 →
│   │   3f7c24e66 → 99ff33003 → c6d3bc0d9 → ec15500b8 → b318efa52* →
│   │   d671b61dc* → 250b2bc13 → f0fb3bbc3 → 9afc675a6 → 7f2eb1d6c → 2151425c4
│   │   （*为 T05 review 修复提交）
│   └─ codex/issue30-t66（6 提交）：1a35e29a3 → ef30a057d → ffdb61152 →
│       193abe9ac → 1b2e25005 → 75dcdeaab（T36 AC3）
├─ 979762df2  merge: integrate #35 (second batch)
├─ 417b7dedf  merge: integrate codex/issue30-t66 with T05 task detail
│              （3 个冲突文件按双方意图手工合并：runtime/ports.ts、
│                composition.ts、app-smoke.test.tsx）
├─ bbe4cf7ec  docs: ocr fix batch increment
├─ OCR 增量批次 2 修复（10 提交）：3445425d3 → ea94112bb → aea1be8d4 →
│   159e82bee → f41f9d36d → f23779bce → 15f2e5fc8 → afb05d8d0 →
│   095ddc9d8 → 3e2c17e51
├─ 0c07fd627  docs: ocr increment batch2 fix report
├─ 56a859cb0  test: ios simulator verification
├─ d6f75a9ea  fix(mobile/ios): default prebuild output runs on iOS 27 SDK
├─ c71d6a03f  docs: ocr fix batch round 1
├─ OCR 最终轮修复（13 提交）：195a1e77c → 90084e2b4 → 7f0469b7a → fd8393b8b →
│   01e219594 → 48093a348 → 784f435e5 → 8fa915a34 → 9fdb5b971 → c60def06e →
│   ef9442b13 → 52791e0fa → f7753fa16（Task 13，由并行进程以相同内容代提交）
└─ 3f72d96db  docs(issue30-sweep): ocr fix round 1 report（最终 HEAD）
```

首批 #32/#33/#34 的 Ledger 头部均记录「分支：codex/issue30-mobile-office」，且该区段 git 历史线性、无 merge 提交——即首批按「完成后按序集成」落为串行快进拓扑（Ruling 2）；第二批 #35/#66 为真正的并行分支 + 两个 merge 提交（Ruling 6）。

### 3.3 关键单点引用

- 首个实现提交：`3d3fe3db0`（switch-tenant remote adapter）；最后一个实现提交：`f7753fa16`（iOS 插件四项）
- 五份 opt-in 真实 HTTP 集成证据提交：#32 `6fcd7d472`、#33 `2a7691bd0`、#34 `028f72b11`、#35 `7f2eb1d6c`、#66 `75dcdeaab`
- 后端（Go）侧提交：`49cb2dfbf`/`9fdda6efe`/`11046e68d`（#34 迁移+读模型）、`63411e5c5`/`f6cf87307`（#35 task facts）、`3445425d3`（OCR B2-F12 属主错配 P0 修复）

---

## 四、实际运行的测试与检查结果

**来源声明**：本节汇总自各 Ledger、修复报告与集成记录中记载的实跑命令与输出；除「报告撰写时核实」标注外，本报告撰写会话本身未重跑测试套件（仅运行 git/文件核验命令）。各数字均可在上列报告文件中查到对应原文。

### 4.1 计划级 gate（SDD Ledger 记载）

| 计划 | gate 结果 | 实跑命令（原文见 Ledger） |
|---|---|---|
| t32 | PASS | `npx tsx --test`（runtime.test.ts、runtime.integration.test.ts、mobile-runtime.test.ts、runtime-vault.test.ts、scoped-vault.test.ts）+ `pnpm --filter @weknora/mobile test` + `pnpm --filter @weknora/mobile typecheck` |
| t33 | PASS | `npx tsx --test`（9 个测试文件：domain 2、api-client 2、shelf 1、runtime 4）+ mobile test + typecheck |
| t34 | PASS | `go test ./internal/application/repository/ ./internal/handler/session/ -run 'TestWorkbenchList|TestWorkbenchTask|TestMX013|TestWorkbenchNotifications' -count=1` + `npx tsx --test`（contracts 1、mobile-core 2、api-client 3 文件）+ mobile test + typecheck |
| t35/t66 | 无持久化 Ledger | 集成 merge 提交 `417b7dedf` 记录的集成验证：`tsx --test` 6 文件 71 tests（69 pass / 0 fail / 2 skipped）+ `pnpm --filter @weknora/mobile test` 40/40 + typecheck 通过 |

### 4.2 OCR 修复批次验收（修复报告记载，全部实跑）

- **增量批次 2**（[fix-report-increment-batch2.md](ocr/fix-report-increment-batch2.md) 附录 B）：
  - `go test ./internal/handler/session/ ./internal/application/repository/` → 两个包 ok
  - mobile-core 5 文件 → 84/84 pass
  - apps/mobile 5 文件 → 50/50 pass（内含 typecheck 实跑通过）
  - api-client 2 文件 → 11 tests：10 pass / 0 fail / **1 skipped（opt-in 真实 HTTP 用例，无环境凭据按既有语义 skip，非伪通过）**
- **最终轮 Round 1**（[fix-report-round-1.md](ocr/fix-report-round-1.md) 总览）：
  - `pnpm --filter @weknora/mobile-core exec tsx --test 'src/**/*.test.ts'` → 146/146
  - `pnpm --filter @weknora/mobile test` → 72/72
  - `pnpm exec tsx --test packages/domain/src/mobile/resource-presentation.test.ts` → 6/6
  - `pnpm --filter @weknora/mobile exec tsc --noEmit` → exit 0
  - `git diff --stat pnpm-lock.yaml` → 空（无新依赖）

### 4.3 iOS 模拟器实测（iPhone 18 Pro / iOS 27.0 / Xcode 27）

- 测试轮（[ios-test-report.md](ios-evidence/ios-test-report.md)）：preflight 全过；Release 构建第 5 轮 BUILD SUCCEEDED；默认配置启动 SIGTRAP（iOS 27 强制 UIScene）；scene 补丁 + 关闭新架构后首屏 DeploymentLoginScreen 渲染成功；授权面 4 项（#32/#34/#35/#66）未验证（无凭据）。
- 修复轮（[fix-report-round-1.md](ios-evidence/fix-report-round-1.md)）：5/5 发现处置完毕；`pnpm --filter @weknora/mobile test` 64/64（含 5 个新插件单测）；typecheck 通过；两次 Release 构建成功；三轮启动进程存活；`weknora:///resources` warm + 冷启动均导航到 Resources 屏（AX 快照证据）。

### 4.4 未运行的检查（如实声明）

1. **opt-in 真实 HTTP 集成证据未在带凭据环境运行**：五个 Issue 的真实 HTTP 证据用例（`WEKNORA_MOBILE_TEST_*` 环境变量门控）在本环境一律 skip（iOS 报告第 7 节、修复报告附录 B 均有记载）。「具备真实 HTTP 集成证据」仅指证据代码与 skip 语义已落地并被测试钉住，不指凭据环境实跑。
2. **本报告撰写会话未重跑任何测试套件**：仅运行了 git 历史核验（`git log/rev-parse/ls-tree`）与文件实读（`parseIpv6Literal` 等）。
3. **PostgreSQL 迁移复跑**（#58 残余）与 **Android 侧任何验证**：从未在本轮运行（无环境）。

---

## 五、审查、OCR 结论、修复轮次与并行波次

### 5.1 Superpowers SDD 审查链

- **派发前冲突扫描**：三份首批 Ledger 均含逐任务对（T1↔T2…）接口/文件冲突扫描记录；t34 扫描判定 **blocking**（Task 6 归一化自相矛盾、Task 8 surface 断言与实现矛盾，Ruling 5）——扫描结论要求先修订计划再执行，实现阶段按修订后计划完成（T6/T8 各有一轮 fix round 吸收）。t35 扫描记录了 T1→T2、T2→T3→T8 两处接口一致性核对（一致，Ruling 8）。
- **任务级审查**：t32 Task 3、t33 Task 2/3/5、t34 Task 8 各有 1 轮 fix round（每轮 ≤5 项裁决、新破坏 0）；其余任务 review clean。
- **计划审查**：#34 第 2 轮 7 项残留、#35 第 2 轮 4 项残留——按 Ruling 3/7 作为 Review Focus 传入任务审查，未阻塞执行。
- **最终审查（final review）**：#33/#34/#35/#66 通过（finalApproved=true）；**#32 修复波后仍有 1 项 important 发现**（IPv6 绕过），已实测 ADDRESSED 但按 SDD 规则不再开第二波，残留呈报（Ruling 4）。

### 5.2 OCR 三阶段

| 阶段 | 扫描 | 发现 | 处置 |
|---|---|---|---|
| 第二批增量 | [ocr-increment-batch2.md](ocr/ocr-increment-batch2.md)（complete：46 findings / 29 items） | 15 项已验证 + 24 项 lowWorth | 10 任务修复（10 提交）→ [fix-report-increment-batch2.md](ocr/fix-report-increment-batch2.md)；B2-F12（snapshot 属主错配，P0）等全部纳入；B2-F23/F4/F6/F11/F28/F35/F46 延期（附录 A） |
| 最终轮 | [ocr-round-1.md](ocr/ocr-round-1.md)（**partial**：50 findings；28/62 items 因 LLM 429 限流失败） | 24 项有效 + 20 项 lowWorthFixing | 13 任务修复（13 提交）→ [fix-report-round-1.md](ocr/fix-report-round-1.md)：23/24 有效发现修复 + R1-F26 领域决策排除；18/20 lowWorth 修复 + F1/F28 排除；R1-F17/R1-F20 部分修复（Round 2 范围） |
| 重扫 | **未执行**（用户指示最终轮缩减为 1 轮，Ruling 10） | - | 修复增量未被二次 OCR 覆盖，以修复报告内的定向复审与全量回归（146+72+6）兜底；第二批增量 OCR 曾有 1 项未决随最终轮核销（Ruling 9） |

**OCR 覆盖缺口（如实）**：最终轮 62 个 selected items 中 28 个因审查模型 429 限流未获审查（ocr-round-1.md 尾部 retry report 列明失败文件组），该部分代码未经本轮 OCR 结论覆盖。

### 5.3 并行波次与集成记录

- **波次 1**（Ruling 2）：#32→#33→#34 同 DAG 批次、计划文件范围不重叠，独立 worktree 并行实施、完成后按序集成——落为集成分支上的线性串行段（3.2）。
- **波次 2**（Ruling 6）：#35 与 #66 同批次**全并行**（不再因 filesTouched 重叠降级串行），独立分支；集成顺序 t35 先（`979762df2`）、t66 后（`417b7dedf`），后者 3 个文件冲突（`ports.ts`、`composition.ts`、`app-smoke.test.tsx`）由集成修复员按双方意图合并，集成验证 71 tests（69 pass/2 skipped）+ mobile 40/40 + typecheck 通过（merge 提交 `417b7dedf` 消息记载）。
- **OCR/iOS 修复波**：与实现串行收尾；期间发生一次并行进程代提交（`f7753fa16`，内容逐字一致，fix-report-round-1 偏离 #10 如实记录）。

---

## 六、裁决记录（全部 Ruling，含判断错误时的代价）

1. **模型路由替代**：用户指定 gpt-5.6-sol/terra/luna 分级路由，但本运行环境子代理只能运行会话模型（GLM-5.3），全部子代理实际使用会话模型；架构与审查类任务通过独立 fresh-eyes 双代理交叉制衡补偿。代价：关键决策深度不足，可由人工复核最终报告发现。
2. **执行并行化（用户指示）**：同一 DAG 批次且计划文件范围不重叠的计划并行实施，各自独立 worktree 与分支，完成后按序 merge 集成；计划内部任务保持串行（SDD 同文件冲突规则）。代价：merge 顺序决定最终提交拓扑，冲突时需集成修复轮。
3. **计划 #34 审查第 2 轮未全通过（7 项残留）**：残留项作为任务审查的额外关注点传入 Review Focus。代价：实现阶段可能暴露这些缺口并触发修复轮。
4. **#32 最终审查修复波后仍有残留**（important：`disallowedDeploymentHost` 可被 IPv6 形式绕过 → ADDRESSED：实测对 `[fe80::1]`/`[fc00::1]`/`[fd12:3456:789a::1]`/`[::ffff:7f00:1]` 全部 `enabled:false/disposition:'invalid'`，`parseIpv6Literal` 展开 16 字节，`apps/mobile/src/runtime-integration-smoke.ts:45-80`）：按 SDD 规则不再有第二波，残留呈报用户。代价：该 Issue 不能宣称完全干净。
5. **计划 #34 冲突扫描判定 blocking**（阻塞①：Task 6 测试期望 `search:'quarterly review'` 与 `normalizeQuery` 仅 `trim().slice(0,200)` 的实现矛盾（`'   quarterly   review '.trim()` 保留内部多空格，`node -e` 实测）；阻塞②：Task 8 断言过滤 `type==='Text'` 检查 `'View all tasks'`，但该文案是 Button 的 title 属性、react-native 桩无 Text 子节点（`app-smoke.test.tsx:23、:60-68` 实读），断言必为 false）——已按扫描结论先修订再执行并在任务审查中重点核对。代价：可能触发额外修复轮。
6. **第二批并行实施（用户指示）**：#35、#66 同 DAG 批次节点全部并行（不再因 filesTouched 重叠降级串行），独立 worktree 与分支，重叠文件合并冲突由集成修复员按双方意图解决。代价：merge 冲突概率上升、集成修复轮可能增加；若集成失败该节点如实记受阻。
7. **第二批计划 #35 审查第 2 轮未全通过（4 项残留）**：残留项传入任务审查关注点。代价：实现阶段可能触发修复轮。
8. **计划 #35 冲突扫描结论**（T1→T2 `ReadTaskFactsForRun` 签名一致，`workbench_list.go:135`/`attentionOf:49-58` 核实；T2→T3→T8 `TaskSnapshotFacts` snake_case wire 形状一致，`contracts.go:52-58`、`executions.ts:174/282` 核实）——已按扫描结论继续执行并在任务审查中重点核对。代价：可能触发额外修复轮。
9. **第二批增量 OCR 有 1 项未决**：单轮封顶不再重扫，随最终全量轮与报告核销。代价：个别问题可能带入最终交付。
10. **OCR 最终轮缩减为 1 轮（用户指示）**：本批发现修复并定向复审后不再重扫。代价：修复增量未被二次 OCR 覆盖，以复审裁决与最终报告兜底，残留如实呈报。

---

## 七、未完成节点、遗留风险、延期项与需用户决策

### 7.1 未完成 / 阻塞节点

- **34 个下级 Issue 未实施**（B3–B8 全部节点，见 1.2）；#31 验收开放（staging 凭据 + Android 证据外部阻塞）。
- **授权面端到端验证缺口**：#32/#34/#35/#66 的授权交互与全部 opt-in 真实 HTTP 集成证据未在带凭据环境运行（本环境无 deployment 凭据/后端；也不允许写入凭据字面量）。iOS 实测仅覆盖：构建/安装/启动/首屏/deep link 到未授权 Resources 提示。
- **t35/t66 SDD Ledger 未持久化**：并行 worktree 清理时 `progress.md` 未入库，任务级审查记录缺失，只能以提交与集成记录替代。

### 7.2 遗留风险

- **OCR 干净范围台账未建立**：`ocr/ocr-ledger.md` 不存在（第二节）。「重跑只审新增量」的续审机制因此缺位——后续若再跑 OCR，需先人工确定基线提交区间，否则只能全量重扫。
- **OCR 最终轮覆盖部分失败**：28/62 selected items 因 429 限流未审（5.2），这部分改动没有 OCR 结论。
- **OCR 修复增量未经二次 OCR**（Ruling 10）：13 + 10 个修复提交本身只被定向复审与回归测试覆盖。
- **#32 残留项未经第二波审查**：IPv6 绕过的修复证据是「实测 ADDRESSED」，未走独立审查波次（Ruling 4）。
- **iOS Fabric 白屏根因未修**：以 `newArchEnabled:false` 规避（RN 0.83.10 + iOS 27.0 runtime 组合问题，上游超本仓范围）；RN 编译期 `-DRCT_NEW_ARCH_ENABLED` 标志仍作用于源码构建 pods；升级 Expo/RN 后需复测再翻转。
- **Expo 模板锚定**：`ios-xcode27` 插件锚定 SDK 55 模板原文，Expo 升级致模板漂移时 prebuild 会显式报错（有单测覆盖该行为），需同步更新锚点。
- **Mimosa 安全扫描不完整**：OCR 修复轮多次 commit 前提示「未得到完整扫描结论（library_source_unavailable / scanner_enobufs / callgraph_fact_partial）」，按兼容策略放行（fix-report-round-1 偏离 #11）——本轮不宣称项目安全，完整审计需另行重跑。
- **Metro/localhost 联调受阻**：宿主 VPN/TUN 全局代理（127.0.0.1:17890）拦截模拟器 localhost 流量，未改动用户网络环境（iOS 报告 3.2/第 7 节）。

### 7.3 口径不一致（如实呈报，需用户知悉）

- **#35 任务计数**：编排器执行统计 6/6，但持久化计划 plan-t35.md 含 11 个任务，且 t35 分支 14 个提交与 11 任务一一对应（Task 1–11 各至少 1 提交 + 2 个 review 提交 + 1 个 typecheck 修复）。以计划文件与提交证据为准（11 任务全部落地）；统计口径差异未能在本轮材料内解释。
- **t66 计划验证命令预期值与实跑差异**：计划预期「既有 33 + 新 14 + 改写 1」，集成时实跑为 71 tests（6 文件，含 2 skipped）——数量口径随合并演进，结果全绿（417b7dedf）。

### 7.4 延期低优先级事项（计划明示、未实现）

- OCR Round 2 范围：SQLite 持久 TaskProjectionStore（R1-F20 完整版 / B2-F23，需存储选型 ADR）、R1-F17 行存储后端（当前 `VAULT_ROW_TOO_LARGE` 受控失败）。
- 按计划排除项：R1-F26（领域决策，未动 CONTEXT.md）、R1-F1（证据行号漂移无法定位）、R1-F28（独立重构）；B2-F4/F6/F11/F28/F35/F46（理由见 ocr-fix-increment-batch2.md 附录 A）。
- #58 PostgreSQL 迁移复跑（000187/000188，环境性）。

### 7.5 需要用户决策

1. **模型路由替代**：本轮全部子代理实际使用会话模型 GLM-5.3（Ruling 1）——请人工复核本报告与关键决策（尤其 #32 残留修复与两处计划 blocking 修订）以补偿路由深度缺口；如需原定 gpt-5.6 分级路由，需在支持该路由的环境重跑关键审查。
2. **#32 残留处置**：接受实测 ADDRESSED 证据并关闭，还是安排一次独立复审波次。
3. **OCR 台账**：是否补建 `ocr-ledger.md`（以下一轮重扫为起点记账），以及是否对 28 个限流未审 items 与修复增量补跑 OCR。
4. **授权面验证**：何时提供 `WEKNORA_MOBILE_TEST_*` 凭据环境（官方云或自托管 staging）以运行五个 Issue 的真实 HTTP 集成证据与 iOS 授权面交互。
5. **下一批次**：B3 的 7 个节点（#36/#38/#41/#42/#44/#46/#59）前置已全部满足，是否启动及其并行策略。
6. **分支去向**：`codex/issue30-mobile-office`（及 t35/t66 并行分支）是否推送远端、开 PR、合并 `main`；对应 5 个 GitHub Issue 的关闭时机。

---

## 交付声明

本轮全部成果（83 个提交，含 5 个 Issue 的实现、两轮 OCR 修复、iOS 适配与全部文档证据）均为**本地 worktree 分支 `codex/issue30-mobile-office` 上的本地提交**：**未推送远端、未合并到 `main` 主分支、未关闭任何 GitHub Issue**（#30–#71 全部保持原 open 状态，#58 维持既有 closed）。本报告主仓库副本仅为文件落盘，不改变主仓库 git 状态。
