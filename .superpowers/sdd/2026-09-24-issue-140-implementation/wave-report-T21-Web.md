# T21 Web 子任务报告：额度预估与超额阻断（Issue #161）

- BASE：`38c22abe9`（Wave 12 集成 HEAD，T21 后端已集成）
- HEAD：`942af8050`（`feat(web): show quota estimates and overage gates`，本地提交未 push）
- 交付：Web 面的预估展示、超额阻断与恢复态 + api-client usage 方法严格解码 + 浏览器 E2E 验收。

## 1. 范围与路径选择（最小侵入报告）

主计划 Step 3 允许"`apps/web/src/career/UsagePage.tsx` 或并入既有页面——最小侵入选择并报告"。**选择：并入收费入口**——预估面板做成独立组件 `apps/web/src/career/UsagePanel.tsx`（+`usage.css`+测试），由 SearchPage（一次性找岗，收费动作）与 RulePage（规则启用，收费路径）在 ready 视图顶部渲染。理由：

1. 验收要求"执行前展示将消耗额度与触发条件"——预估属于收费入口本身，独立路由页会把展示与执行分离；
2. 文件所有权为 `apps/web/src/career/` 与 `packages/api-client/src/career.ts`；独立路由页需改 `apps/web/src/router.tsx`/`routes.tsx`/`PlatformShell.tsx`（所有权之外）；
3. 组件化同样满足"usage.tsx 用户可观察行为测试"——`UsagePanel.test.tsx` 覆盖来源/权限/失败/恢复。

## 2. RED→GREEN 证据（TDD）

### 2.1 api-client（`packages/api-client/src/career.test.ts` +3 test）

- RED（`red-api-client.txt`）：`TypeError: api.usageEstimate is not a function`，tests 43 / pass 40 / **fail 3**。
- GREEN（`green-api-client.txt`）：tests 43 / **pass 43** / fail 0。
- 实现：`UsageOperation`/`UsageEstimateView` 类型 + `decodeUsageEstimateView` 严格解码（kind/operation 冻结、costUnits≥1 安全整数、conditions 非空非空白数组、RFC3339 窗口、limit≥1、reserved/settled≥0、remaining 安全整数、wouldAdmit 布尔）+ `createCareerApi.usageEstimate(operation, signal)` → `GET /api/v1/career/usage/estimate?operation=search_once`。typed 拒绝（`search_quota_refused` 429 / `admission_unavailable` 503）经 transport `ApiError` 透传，客户端不发明任何码。

### 2.2 UsagePanel（`apps/web/src/career/UsagePanel.test.tsx`，6 test）

- RED（`red-usage-panel.txt`）：`ERR_MODULE_NOT_FOUND: .../UsagePanel.tsx`（组件不存在）。
- GREEN（`green-usage-panel.txt`）：tests 6 / **pass 6** / fail 0。
- 覆盖：预估 verbatim（消耗 1 单位+冻结条件+余额 47/50+窗口，且断言前端不重算——不出现派生值 46）；超额态（本期额度已耗尽→新收费找岗被阻+既有档案仍可读）；预估不可得（typed 原因+"不会先执行后补报"+重试）；权限（forbidden 态+重试+不显示消耗行）；恢复（重试后余额恢复）；scope 切换（换身份重读）。
- 导出 `usageAllowsChargedRun`（单一准入判定：仅 ready 且 wouldAdmit 放行收费 Run）。

### 2.3 SearchPage 集成（+4 test）

- RED（`red-search-page.txt`）：新 4 test 失败（面板缺失/门控缺失），既有 10 test 全绿（未破坏）。
- GREEN（`green-search-page.txt`）：tests 14 / **pass 14** / fail 0。
- 覆盖：执行前面板（含"付费状态不改变岗位排序或资格判定"逐字）；预估不可得→提交禁用+0 次搜索（无先执行路径）；超额→提交禁用+0 次搜索+可读面保留；终态回执与 quota 拒绝都刷新余额+拒绝后重试沿用原 requestId（重复不二扣的 replay 呈现）。

### 2.4 RulePage 集成（+2 test）

- RED（`red-rule-page.txt`）：新 2 test 失败，既有 9 test 全绿。
- GREEN（`green-rule-page.txt`）：tests 11 / **pass 11** / fail 0。
- 覆盖：启用需 live 预估（不可得→启用被阻+0 次写入；停用配置仍可保存——停用态不会触发收费 Run）；超额态呈现（启用仍可保存——后端语义：每次触发被可见拦截 blocked_no_quota；面板明示"既有档案…仍可完整读取"）+保存后余额刷新。

## 3. 全量验证（命令与真实输出）

| 命令 | 结果 |
| --- | --- |
| `pnpm typecheck:web` | 通过（无错误输出，exit 0） |
| `pnpm test:web`（全量 node26 v26.7.0） | `ℹ tests 2478 / pass 2478 / fail 0 / cancelled 0`（基线 2466 + 12 新增：UsagePanel 6 + SearchPage 4 + RulePage 2；跑前 ps 清理孤儿 runner） |
| `pnpm build:web` | `✓ built in 29.62s`（仅既有 chunk>500kB 提示） |
| `git diff --check` | 干净（无空白错误） |
| `pnpm typecheck:shared`（额外，因改了 api-client） | exit 0 |
| `pnpm test:shared`（额外） | 950/951；唯一失败 `packages/design-tokens/src/craft.test.ts`：`ENOENT packages/ui/src/theme.css`——**BASE 38c22abe9 即存在的环境缺口**（packages/ui 在本仓库不存在），与本轮 diff（仅 career.ts/career.test.ts/apps/web/src/career/*）无关 |

## 4. 浏览器 E2E（T21 verified 关键验收）

### 4.1 环境（隔离，跑毕已清理）

- Lite 服务器：临时目录 SQLite/local/memory，loopback `127.0.0.1:57825`，`CAREER_SEARCH_QUOTA_LIMIT=2`（**声明：超额经启动 env 收缩窗口构造**——后端冻结的部署覆盖，无任何服务端代码改动）；生成 JWT/AES；一次性随机账号。
- Vite dev：`127.0.0.1:57824`（本轮指定端口），`VITE_DEV_PROXY_TARGET=http://127.0.0.1:57825`。
- 真实浏览器：Playwright（Chromium）。进程已停、端口已释放、临时目录/临时脚本已删除。

### 4.2 声明层级

- 数据构造：**HTTP API 层**（python 标准库直连 57825）：register/login → propose+confirm 档案事实（revision 2）→ import JD（rawText 含批次 SSE-2027-001）→ evaluate（unknown 态）→ create application（linkState ready）。
- 浏览器只做登录、预估读取、找岗执行、超额观察、旧申请阅读（用户可观察行为）。

### 4.3 观察结果（截图 `screenshots/`）

| 步骤 | 观察结果 |
| --- | --- |
| 登录 → /platform/career/search | 预估面板（截图 01）："下一次找岗将消耗 1 个额度单位"、"本期剩余 2 / 2（已预占 0 · 已结算 0）"、"计费窗口 2026-09-01 至 2026-10-01 UTC"，5 条冻结条件逐字含"付费状态不改变岗位排序或资格判定：评估与排序输入不含任何付费维度" |
| 找岗 1（上海 后端开发 校招） | 诚实空覆盖（no_vetted_sources，无演示数据）；终态后面板自动刷新"本期剩余 1 / 2（已结算 1）"（截图 02） |
| 找岗 2（杭州 Go 工程师） | 面板进入超额态（截图 03）：alert"本期额度已耗尽"“新的收费找岗已被阻止；既有档案、申请、评估与搜索记录仍可完整读取”+“本期剩余 0 / 2（已结算 2）” |
| 超额后新找岗 | 输入指令后提交控件 `t-is-disabled`（DOM 断言 true），点击无效——不提供先执行路径 |
| **旧申请（E2E 关键路径）** | 超额后深链 `/platform/career/opportunities/{opp}?snapshotId={snap}&application={appId}` 完整可读（截图 04）："申请已创建 申请编号 348ab087…"+"本次申请固定的证据（评估/档案修订 2/评估结论）"+评估区+材料编辑区全部呈现 |
| /platform/career/rules | 同一超额态呈现（截图 05）+ 中性条件逐字；规则配置面完整 |
| 浏览器 console | 0 个应用错误（仅一条既有 `/auth/auto-setup` 403 首屏探测，与本轮无关） |

### 4.4 API 层补充探针（声明）

- **重复不二扣**：replay 找岗 1 的原 requestId → 200 且返回**同一 searchId**（625898d6-…）；replay 前后 estimate 均为 settled 2 / remaining 0——未新增预占或结算。
- **typed 超额拒绝**：新 requestId 找岗 → **429 `{"error":{"code":"search_quota_refused",…}}`**；拒绝后 estimate 不变（无持久残留）。
- **旧申请 API 读取**：超额后 `GET /career/applications/{id}` → 200 linkState ready。

### 4.5 预估不可得场景（seam 声明）

`admission_unavailable` 需账本不可读（生产装配无法在 live 链路注入故障，未注入）。覆盖于单元测试：`UsagePanel.test.tsx` "an unreadable ledger is fail-closed…"、`SearchPage.test.tsx` "an unreadable estimate closes the charged path…"、`RulePage.test.tsx` "enabling requires a live estimate…"（门控+文案+0 次请求断言）；后端 Wave 12 已有 fail-closed 测试（评审通过）。

## 5. 文件清单（HEAD vs BASE，9 files，+580/−10）

- `packages/api-client/src/career.ts`（usage 类型+严格解码+client 方法）
- `packages/api-client/src/career.test.ts`（+3 test）
- `apps/web/src/career/UsagePanel.tsx`（新）、`UsagePanel.test.tsx`（新）、`usage.css`（新）
- `apps/web/src/career/SearchPage.tsx` / `.test.tsx`（面板+门控+余额刷新；+4 test）
- `apps/web/src/career/RulePage.tsx` / `.test.tsx`（面板+启用门控+超额呈现；+2 test）

既有 SearchPage/RulePage 测试全部保持通过（共享 mount helper 默认注入 admitting estimate，个别 usage 测试显式覆盖）。

## 6. 已知局限与遗留

- **后端遗留（Wave 12 评审结论转述，非本轮引入）**：F2 low——UsageEstimate 注释"a free read"与 reconcile 状态机 UPDATE 措辞不符；F3 low——真实账本 gate 下 TriggerDueRules 端到端用例缺位。本轮 Web 面未发现新增缺口；消费的 API 面与集审冻结一致（usage.go 实读核对）。
- 超额下的规则触发可见拦截（blocked_no_quota run 记录）依赖 T13 既有呈现，本轮以单元断言+面板文案覆盖，未在 live 触发周期 Run（TriggerDueRules 为服务端 seam）。
- 提交时 Mimosa 钩子提示 commit 前完整扫描未完成（scanner_enobufs，按兼容策略放行）；本报告不据此宣称项目安全。
- T21 verified 由主控裁决；本报告仅陈述事实与实测输出。
