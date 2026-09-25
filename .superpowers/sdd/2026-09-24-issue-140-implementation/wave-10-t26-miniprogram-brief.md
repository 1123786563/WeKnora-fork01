# Wave 10 — T26：微信小程序申请、材料与本人投递（implement）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 依赖均就绪：#148=T06 verified（TDesign 工程链）、#159=T18 verified（投递确认合同）、#164=T24 verified（career 服务层与登录关联已就位）。你复用 T24 建立的小程序 Career 基础设施。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t26-miniprogram/WeKnora-fork01 --detach f6b14cf95
cd /Users/wuyongjun/.codex/worktrees/issue-140-t26-miniprogram/WeKnora-fork01
```
BASE = `f6b14cf95`（当前集成 HEAD，含 T24 verified 的小程序 Career 基础设施）。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 26 文本（verbatim）

### Task 26: T26/#166 微信小程序（Taro 4 + TDesign Miniprogram）申请、材料与本人投递
**Depends:** #148、#159、#164。**Files:** `apps/miniprogram/src/career/application-material.tsx`、`apps/miniprogram/src/career/application-material.test.tsx`、`apps/miniprogram/src/adapters/career-platform.ts`。**Produces:** CareerRemote 语义；本端文件/分享/通知/受控存储适配器。
**验收：** 可见小程序控件使用 TDesign Miniprogram 或记录明确的原生能力例外；实际开发工具与真机验证；**与 Web、移动 App 使用同一申请和结构化正文版本**；**PDF/DOCX 实际下载且内容对应确定版本**；硬条件不符时**警示常驻**，用户显式继续；**投递确认不触发招聘平台自动提交**
- Step 1：先在 `application-material.test.tsx` 写可观察失败测试：本 Task 验收、冲突回执、未知结果对账、空间切换旧响应失效；`pnpm --filter @weknora/miniprogram test && typecheck && build:weapp` 确认 RED。
- Step 2：接入共享 Career Desk 和本平台适配器，建立真实服务端数据页面；失败、无权限、额度不足与 pending 均给用户可操作状态，**不以原型静态数据代替**。
- Step 3：三命令 GREEN；真实平台验证认证、文件、分享、通知/存储与窄屏硬冲突可见性，保存截图及受保护数据访问证据。
- Step 4：自查版本与缓存失效，独立 frontend_validator 验证，reviewer 双结论；**未通过真实设备门槛则保留 blocked**。
**验证命令：** `pnpm --filter @weknora/miniprogram test && pnpm --filter @weknora/miniprogram typecheck && pnpm --filter @weknora/miniprogram build:weapp`。**原始证据：** Career Desk 版本冲突测试；真实环境完成岗位到文件下载和投递确认。**失败处理：** 下载授权过期可重取；结果未知先对账。

## 3. 可消费的服务端合同（集成 HEAD 冻结）

- **申请**（T14）：`POST /api/v1/career/applications`（CreateApplicationInput：requestId/opportunityId/snapshotId/evaluationId/batchIdentity/continueDespiteHardFailure/expectedRevision）+ receipt/get/reconcile 四路由；linkState ∈ linking|ready|link_failed。
- **材料**（T15）：`POST /materials`、`POST /materials/confirm`、`GET /materials/receipt|:id|versions...` 七路由（结构化正文/claim/不可变版本）。
- **导出**（T16）：`POST /materials/:id/exports`（发布双格式）+ list/signed-url/**认证兑付 download**（下载须带 Authorization——T06 downloadFile 带 header 先例）/revoke。
- **投递**（T18）：`POST /applications/:id/submissions`（渠道 email/web/other+声明时间+submittable 版本或显式未知；一申请一记录重复 typed conflict）+ list/receipt。
JSON 形态以 `internal/modules/career/{application,material,rendering,submission}.go` 为最终依据。小程序侧复用 T24 的 career service 模式扩展（`apps/miniprogram/src/services/` 与 `adapters/career-platform.ts`）。

## 4. T24/T06 既得工程事实（必须沿用）

- TDesign 构建闭包收集 + miniprogram_npm 拷贝机制已入库——新页面 usingComponents 按同机制注册（勿重蹈 D1）；t-button 事件经修复链可用；原生例外须记录明确理由。
- 真实 DevTools 验证模式：`WEKNORA_API_ORIGIN` 构建注入、urlCheck=false（gitignored private config）、`cli auto`+automator（仓库外安装）、冷编译 settle-retry——先例 `docs/plans/issue-140/task-6-live-validation.md` 与 T24 归档证据。
- typecheck 13 个 CommercialSummary 基线错误豁免（不修不增）。
- T24 的教训：证据截图必须可区分（字节级不同+关键元素断言）；死代码提示必须真实可达；404 分支要有恢复态。

## 5. 行为要求（RED 测试先行）

- **同一版本语义**：申请/材料/投递与 Web 同源同版本（同后端 API；不同批次申请分别展示；材料修改产生新版本不覆盖旧版）。
- **PDF/DOCX 下载**：从 submittable 导出经 signed-url→**带认证下载**→本地打开（T06 openDocument 模式）→内容对应确定版本（E2E 中对下载字节做 digest 比对或文件大小+首字节校验，如实声明校验层级）；下载授权过期可重取（重新签发，T06 恢复模式）。
- **硬条件警示**：ineligible 申请警示常驻（窄屏完整可见），显式继续后才可提交；qualified=false 可见。
- **投递确认**：渠道/声明时间/版本或显式未知；**零自动提交零外发**；重复确认被拒呈现。
- 冲突回执/未知结果对账（原 requestId）/空间切换旧响应失效三 seam 可观察测试。
- 失败/无权限/pending 可操作状态；不用静态演示数据。

## 6. 验证（全跑附原始输出）

```bash
pnpm --filter @weknora/miniprogram test       # 90 基线+新增全绿
pnpm --filter @weknora/miniprogram typecheck  # 13 基线外零新增
pnpm --filter @weknora/miniprogram build:weapp
git diff --check
```
**真实 DevTools 验证（本轮端口 57817）**：本地 Lite 服务器+fixture（Career 全链数据经 API 构造，声明层级）。官方构建（无手工补丁）加载 → 登录（T24 关联模式）→ 申请创建（硬条件警示+显式继续）→ 材料编辑/确认新版本 → 发布双格式 → **下载 PDF/DOCX 打开**（digest/校验记录）→ 记录投递（版本绑定）→ 重复确认拒绝 → 过期授权重取 → 跨租户拒绝。截图**逐张可区分**（T24-F3 教训）。**真机无设备单列 blocked**（T06/T24 先例）。

## 7. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree；local commits authorized, no push/merge/deploy/GitHub action；禁止派发子 agent。
- 投递确认不触发招聘平台自动提交；不自动投递/发信；不长期缓存 URL/凭据。
- 测试端口（57817）与构建目录隔离；凭据 disposable 结束清理，报告不留 token。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 8. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t26-miniprogram/` 下写 `task-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、三命令输出、DevTools 各步证据（可区分截图清单+digest 校验记录）、原生例外清单、已知局限（真机 blocked 等）。COMMIT 按逻辑分批。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成；T26 verified 由主控裁决。
