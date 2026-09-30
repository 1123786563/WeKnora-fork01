# T12 Web 子任务报告：岗位差异展示与历史可访问（Issue #151）

- BASE：`5f32c77b8`（Wave 13 集成 HEAD，第 4 轮复审通过）
- HEAD：`21ae96899`（`feat(web): show job change diffs with accessible history`，本地提交，未 push）
- worktree：`/Users/wuyongjun/.codex/worktrees/issue-140-t12-web/WeKnora-fork01`
- 日期：2026-09-26（Asia/Shanghai）
- 委托简报：`.superpowers/sdd/2026-09-24-issue-140-implementation/wave-14-t12-web-brief.md`

## 1. 交付物

### 1.1 api-client（`packages/api-client/src/career.ts` + 聚焦测试）

简报点名的三方法（对账三端点，`internal/router/routes_career.go:26-28` 实核冻结）：

1. `opportunityReconciliations(opportunityId)` → `GET /api/v1/career/opportunities/:id/reconciliations` → `decodeReconciliationList`
2. `reconcileOpportunities({requestId, targetId, candidateId})` → `POST /api/v1/career/opportunities/reconcile` → `decodeReconcileReceipt`（输入校验：ID 非空、target ≠ candidate）
3. `reconciliationReceipt(requestId)` → `GET /api/v1/career/reconciliations/receipt?requestId=` → `decodeReconcileReceipt`

**超出"三方法"的补充（如实声明）**：为满足简报 §5 的「stale 标注」「覆盖说明」行为（其数据源是 T12 后端已冻结的另两个只读端点），同一所有权文件内另加两个只读方法：

4. `opportunityStatus(opportunityId)` → `GET .../:id/status` → `decodeOpportunityStatusView`（过期/下架/要求变化标注、stale、lastHealthyAt、mergedInto；Go 零值时间戳原样接受）
5. `careerCoverage()` → `GET /api/v1/career/coverage` → `decodeCareerCoverageView`（configured + observed + cities，空清单原样）

严格解码要点：封闭枚举（decision/annotation）、`merged ⇒ !suspectedDuplicate` 冻结一致性校验、conflictingBatches 仅在非空数组时保留、observed 计数为正整数、observedCities 元素非空。请求/响应 JSON 以 `internal/modules/career/reconciliation.go` 类型为最终依据（先读后写）。

### 1.2 Web 面板（`apps/web/src/career/`；简报写 `caree/`，实际目录为 `career/`——按现行布局，简报笔误）

主计划 Files 命名 `reconciliation.tsx`，按简报"最小侵入选择"落地为独立面板文件：

- **`reconciliation.tsx`**（新）：
  - `OpportunityStatusPanel`：状态标注（已过期/已下架/要求已变化）；stale 横幅（失败检查 + 最后成功时间；从未成功时显示「尚无成功观察记录」而非 0001 年）；mergedInto 披露；不可变来源观察历史（每条含原始链接 `<code>`、检查时间 `<time>`、「打开固定快照」永久链接）；**变化前后对比**（旧/新快照选择器 + 五字段差异表(已变化/未变化标记) + 双原文并排 + 摘要 + 「原文内容已变化」+ 两个快照的永久链接）；**对账判定**（充分证据才合并话术、并列+疑似重复呈现、conflictingBatches 披露、双方身份四元组含「未提供」、unknown 状态用原请求编号回执恢复/重试、idempotency_conflict 错误态、forbidden 清除）；对账历史列表。
  - `CareerCoveragePanel`：已接入（已核验）来源 / 实际观察来源（含次数与最后检查时间）/ 实际覆盖城市；空清单如实（"生产环境从空清单开始，不会虚构来源"）。
- **`reconciliation.css`**（新）：TDesign 浅色变量 + `#07c05f` 品牌色（变化标记/按钮/收据框），focus-visible 轮廓沿用 `--td-brand-color-focus`。
- **挂载（各一行）**：`OpportunityPage.tsx` 的 `OpportunityEvidencePage`（每岗位证据页，meta 区之后）；`SearchPage.tsx` 末尾（`CareerCoveragePanel`）。
- **`ApplicationPage.tsx`**：申请固定证据的「快照」字段改为指向 `opportunityEvidencePath(pinned.opportunityId, pinned.snapshotId)` 的链接，并标注「（固定不变，旧申请始终展示此旧快照）」——与 T14 pinned 语义一致的 UI 呈现。

## 2. TDD 证据（RED → GREEN）

### 2.1 api-client（`packages/api-client/src/career.test.ts`，43 → 50 tests）

- **RED**（实现前）：`npx tsx --test packages/api-client/src/career.test.ts` → `tests 50 / pass 43 / fail 7`，7 个新测试全部 `TypeError: api.opportunityReconciliations/reconciliationReceipt/opportunityStatus/careerCoverage is not a function`。
- **GREEN**：实现后同命令 → `tests 50 / pass 50 / fail 0 / cancelled 0`。
- 过程中修正 1 处测试期望错误（candidate 证据应保留 title/company），修正后全绿。

### 2.2 Web 面板（`apps/web/src/career/reconciliation.test.tsx`，11 tests）

- **RED**：实现前运行 → `ERR_MODULE_NOT_FOUND: .../reconciliation.tsx`（模块不存在），1 文件级失败。
- **GREEN**：实现后 → `tests 11 / pass 11 / fail 0`。中途修复：① diff/状态抓取改为 async 包装（未 mock 的方法同步抛错不再炸 effect，进入显式 error 态）；② stale/diff 失败提示由 `role="alert"` 改 `role="status"`（alert 保留给可操作错误，与既有模式一致）；③ 修正测试 mock 使其遵循既有"回显 requestId/opportunityId"约定。
- 回归：`OpportunityPage.test.tsx` 38/38、`ApplicationPage.test.tsx` 17/17、`SearchPage.test.tsx` 14/14（单文件运行均全绿）。

## 3. 全量验证（真实命令与输出）

| 命令 | 结果 |
| --- | --- |
| `pnpm typecheck:web` | **EXIT=0**（先修复 1 处 TS2367 收窄冲突 + 1 处测试 querySelector 泛型） |
| `pnpm build:web` | **EXIT=0**，`✓ built in 4m 23s`（仅既有 chunk>500kB 警告） |
| `git diff --check`（对 BASE） | 无输出，exit 0（无空白错误） |
| `pnpm test:web`（首轮） | **EXIT=1：tests 2473 / pass 2470 / fail 3 / cancelled 0**，时长 1489s。3 个失败全部定性（见 §3.1） |

### 3.1 首轮 3 个失败的定性（含证据）

1. **`src/agents/agent-editor.test.tsx`（文件级，1484s）** 与 **`src/settings/GeneralPreferencesPanel.test.tsx`（文件级，1236s）**：环境 CPU 争用下的卡死 runner（单文件正常耗时秒级，实跑 20–26 CPU 分钟仍不结束；`sample` 取证显示 node 主线程 + 全部 V8 worker 满载）。我在套件运行中按简报授权清理孤儿 runner 时误杀了本套件的这两个子进程（当时同款文件名有兄弟 worktree 实例并行 spinning，无法区分归属）。两次隔离重跑均因兄弟 wave 持续灌入同类进程（load 一度 60.59）未能在合理时间内完成。该两个文件与本任务改动无任何 import 关系（我的 diff 仅 career/ + api-client）。
2. **`keeps datasource editing in a body-level 640px drawer with a real isolated form`（DataSourcesPage.test.tsx）**：**BASE 预存失败，与本任务无关**。证据：`git stash push -u` 回到纯净 5f32c77b8 后运行同文件 → `tests 36 / pass 35 / fail 1`，同一断言 `/<Drawer visible header=\{editorTitle\}/` 不匹配源码 `<Drawer visible footer={false} header=...`（BASE 源码即如此）。stash pop 后改动完整恢复。基线声称的 2478/2478 在本机不可复现（至少此文件必失败）。

### 3.2 终轮重跑（真实输出）

`pnpm test:web`（第二轮，在 load 37–64 的兄弟 wave 争用下完成）：**EXIT=1：tests 2479 / pass 2476 / fail 3 / cancelled 0**。3 个失败：

1. `src/agents/agent-editor.test.tsx`（文件级，3894873ms）：唯一在飞子进程空转 64.9 分钟后由我终结（同款文件在本机两轮全量均无法自然完成；与本 diff 无 import 关系）。**隔离补测（`--test-concurrency=1`）：37 个测试通过、0 断言失败后挂起于 idle 等待（19 分钟仅 0.45s CPU，事件循环级挂起而非自旋），未完成**。即该文件所有已执行断言全部通过，失败形态是环境级挂起/吞吐，非断言错误。
2. `keeps datasource editing…`（DataSourcesPage）：BASE 预存失败（§3.1-2，stash 取证）。
3. `member list supports the Vue debounced server-side search contract`（AdministrationPage，3.6s debounce 时序断言）：**纯净 BASE 同负载下同样失败（stash 后 4/5，同断言 "no server call before the 320ms debounce window elapses"）**——负载敏感的环境性失败，与本 diff 无关。

**结论**：与本任务改动有依赖关系的全部测试（新增 7+11 个、受影响的 OpportunityPage/ApplicationPage/SearchPage 既有 69 个）全绿；全量套件的 3 个失败均为环境性/预存（两个经 BASE 对照实证），非本任务引入。

## 4. 浏览器 E2E（T12 verified 关键验收，端口 57826）

- 环境：一次性 SQLite Lite API `127.0.0.1:57825`（`DB_DRIVER=sqlite` + 独立 tmpdir + 随机 JWT/AES 密钥，`go run ./cmd/server`）+ Vite `127.0.0.1:57826`（`VITE_DEV_PROXY_TARGET` 代理 /api）。端口按调度分配使用（浏览器面 57826）。真实 Chromium（Playwright 驱动）。一次性账号注册→登录。截图存 `.superpowers/sdd/2026-09-24-issue-140-t12-web/e2e/`（5+1 张）。结束后 API/Vite 进程已停止、tmpdir 已删除、主仓库无残留文件（截图误落主仓库目录已移走，`git status` 干净）。

| 步骤 | 构造层级 | 观察结果（程序化断言 + 截图） |
| --- | --- | --- |
| 登录 | UI | 注册+登录成功，进入 /platform/creatChat，见「保存职位描述」面板 |
| 双来源同岗 | UI（URL 导入 board-a.test/jobs/8848 + 粘贴；新草稿粘贴 board-b.test/jobs/8848） | O1=`198b9ca0…`、O2=`82bcabf9…` 两条独立记录 |
| 对账 → 并列 | UI（O1 页输入候选 O2 →「对账判定」） | **「不确定重复：两条记录并列保留」**+「身份证据不足以判定…不会静默合并」；双方证据栏岗位编号均 `8848`（自 sourceRef 提取），公司/地点/批次如实显示「未提供」；对账历史列表出现（时间 + 目标←候选）。截图 02 |
| JD 更新 → 变化前后差异 | **API 追加**（声明：UI 的追加入口仅在 URL 导入后首次粘贴提供，且 prior 必须 URL 类观察，故经 import API 以 priorObservationId=URL 观察追加 v2/v3 到 O1） | 选择器切 v1↔v2 后：旧原文「本科及以上」/新原文「硕士及以上」并排可读、「原文内容已变化」、五字段差异表（提取无权限→全部「未知/未变化」，如实）、旧/新快照永久链接（cd510c4d/3a569bfc）。程序化断言全部 true。截图 03/03b |
| 历史快照永久可访问 | UI | O1 共 4 条观察，每条均有「打开固定快照」链接且逐一直接可打开（含最早的空快照 b5f62a4b） |
| 旧申请旧快照 | UI（先评估→申请 pin cd510c4d，再追加 v2/v3） | 申请 `713bf034` 的固定证据区快照为链接指向旧快照 + 「（固定不变，旧申请始终展示此旧快照）」；JD 已更新至 v3 后申请仍指向 v1。截图 03 内可见 |
| 过期标注 | API 追加 v3 含「该岗位招聘已截止」 | 「状态标注：已过期」（后端冻结标记 → 前端如实呈现）。截图 03 |
| 覆盖清单 | UI（/platform/career/search 底部新面板） | 「已接入（已核验）来源：暂无…不会虚构来源」；「实际观察来源：manual_paste · 4 次观察 / board-a.test · 2 次观察（含最后检查时间）」；「实际覆盖城市：暂无覆盖城市记录（如实呈现，不虚构范围）」。截图 05 |
| stale 场景 | UI（board-c.test/jobs/9917 URL-only 记录） | 「最近一次来源检查失败：尚无成功观察记录，以下内容来自失败的检查，数据可能已陈旧。」原始链接与检查时间完整保留。截图 04 |

**E2E 未覆盖项（如实）**：`merged`（充分证据合并）与 `conflictingBatches` 披露在 Live 服务器不可构造——生产 DI 从未接入 `opportunityExtractor`（`NewOffice(db)` 无注入，`internal/modules/career/opportunity.go:143` 无权限即全部字段 unknown），四元组永不全知 ⇒ 后端永远判 side_by_side。这两态由本任务单测覆盖（merged receipt + conflictingBatches 渲染断言）并依赖 Wave 13 后端测试；Live E2E 展示的是诚实并列态。

## 5. 提交与文件清单

- 提交：`21ae96899`（单提交，含全部实现+测试），BASE `5f32c77b8`。
- 评审包：`.superpowers/sdd/2026-09-24-issue-140-implementation/review-5f32c77b8..21ae96899.diff`（91,664 字节，1 commit）。
- 文件：`packages/api-client/src/career.ts`、`packages/api-client/src/career.test.ts`、`apps/web/src/career/reconciliation.tsx`、`apps/web/src/career/reconciliation.test.tsx`、`apps/web/src/career/reconciliation.css`、`apps/web/src/career/OpportunityPage.tsx`（+2 行挂载/import）、`apps/web/src/career/SearchPage.tsx`（+2 行）、`apps/web/src/career/ApplicationPage.tsx`（pinned 快照链接）。未改后端、未改 career-core 合同、无迁移。
- 证据（未入库，.superpowers 被 gitignore）：`e2e/t12-e2e-0[1-5]*.png`。

## 6. 自查与已知局限

1. **简报路径笔误**：`apps/web/src/caree/` → 实际 `apps/web/src/career/`（简报自身亦允许"按现行布局"）。
2. **"三方法"+2**：为满足 §5 行为另加 `opportunityStatus`/`careerCoverage` 只读方法（同一所有权文件、同一严格解码模式、聚焦测试覆盖）；已在本报告置顶声明。
3. **基线不可全绿复现**：`DataSourcesPage.test.tsx` 在纯净 BASE 即 35/36 失败（regex 与源码不匹配），与本任务无关；`pnpm test:web` 全绿在本机当前不可达，除非修复该 BASE 预存失败（超出本任务所有权）。
4. **CPU 争用**：兄弟 wave 并行全量套件导致 load 34→60；两个重型测试文件（agent-editor、GeneralPreferencesPanel）在本机持续 spinning。已按 Wave 13 先例 ps/sample 取证并清理确认孤儿（含一次误伤本套件子进程的记录，如实保留）。
5. **提取字段差异表**：生产无提取权限时字段全「未知」，差异可读性由原文并排对比承载（字段表仍如实渲染）。
6. E2E 数据构造层级已逐行声明（双来源=UI；JD 更新/过期=API 追加；stale=UI）。
