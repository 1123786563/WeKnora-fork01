# Wave 4 — T24：微信小程序 C 找岗、建档与分享导入（implement）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** T06（verified）已打通小程序 TDesign 构建与真实 DevTools 验证链路；T11 后端 searches 合同已集成冻结。你交付小程序端 Career 找岗/建档/分享导入。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t24-miniprogram/WeKnora-fork01 --detach 6a21e0df5
cd /Users/wuyongjun/.codex/worktrees/issue-140-t24-miniprogram/WeKnora-fork01
```
BASE = `6a21e0df5`。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 24 文本（verbatim）

### Task 24: T24/#164 微信小程序（Taro 4 + TDesign Miniprogram）C 找岗、建档与分享导入
**Depends:** #148、#152。**Owner/validator:** frontend_implementer / frontend_validator。**Files:** `apps/miniprogram/src/career/discovery.tsx`、`apps/miniprogram/src/career/discovery.test.tsx`、`apps/miniprogram/src/adapters/career-platform.ts`。**Produces:** `CareerRemote.open/list/act/changes/receipt`；本端文件/分享/通知/受控存储适配器。
**验收：** 可见小程序控件使用 TDesign Miniprogram 或记录明确的原生能力例外；实际开发工具与真机验证；**微信身份只作为关联方式，不生成第二份求职档案**；可上传已有简历或逐步建档，并逐项确认抽取事实；**分享导入先展示可核对内容再提交**；资格冲突、来源和待核实项在窄屏可见；**同一用户 Web 的档案变更可在微信小程序同步看到**
- Step 1：先在 `discovery.test.tsx` 写可观察失败测试：本 Task 验收、冲突回执、未知结果对账、空间切换旧响应失效；`pnpm --filter @weknora/miniprogram test && typecheck && build:weapp` 确认 RED。
- Step 2：接入共享 Career Desk 和本平台适配器，建立真实服务端数据页面；失败、无权限、额度不足与 pending 均给用户可操作状态，**不以原型静态数据代替**。
- Step 3：三命令 GREEN；在真实平台验证认证、文件、分享、通知/存储与窄屏硬冲突可见性，保存截图及受保护数据访问证据。
- Step 4：自查版本与缓存失效，独立 frontend_validator 验证，reviewer 双结论；**未通过真实设备门槛则保留 blocked**。
**失败处理：** 分享负载缺失或授权失效时展示恢复入口，不用演示数据冒充结果。

依赖就绪说明：#148=T06 verified（Wave 2）；#152=T11 后端已集成评审通过（searches 三路由冻结，`internal/router/routes_career.go:32-34`），T11 Web 子任务与本任务并行不冲突——调度员裁定按"前置后端合同 reviewed+integrated 即可派发"先例放行（同 T09-Web/T14-Web 先例）。

## 3. 可消费的服务端合同（集成 HEAD 冻结）

Career 认证 scope + 既有端点：`/api/v1/career/open|list|changes|receipt|act`、opportunities（import/import-url/observations/evidence）、evaluations、applications、**searches**（`POST /searches {requestId, query, expectedRevision}` / `GET /searches/receipt` / `GET /searches/:searchId`，结果行含 checkedAt/qualification/link/uncertainty，coverage.sources 如实清单——`internal/modules/career/search_once.go` 为最终依据）。小程序侧新建 career service（参照 `apps/miniprogram/src/services/workbench.ts` 的 client.request 模式）+ 平台适配器 `career-platform.ts`（分享、文件、受控存储）。

## 4. T06 既得工程事实（必须沿用）

- TDesign 组件经构建期闭包收集 + `miniprogram_npm` 拷贝才能在真实 DevTools 启动（T06 修复已入库；新页面 usingComponents 按同样机制注册，勿重蹈 D1）。
- t-button 事件绑定经 T06 修复链可用；新交互控件优先 TDesign，确需原生例外时在报告记录明确例外理由（验收允许"记录明确的原生能力例外"）。
- 真实 DevTools 验证模式：`WEKNORA_API_ORIGIN` 构建注入、gitignored `project.private.config.json` urlCheck=false、`cli auto` + `miniprogram-automator`（装仓库外）、冷编译 ≥45-60s settle-retry、tourist appid——完整复现提纲见 `docs/plans/issue-140/task-6-live-validation.md`。
- 基线豁免：typecheck 13 个 CommercialSummary 错误（account/pages.tsx）为既有基线，不修不增。

## 5. 行为要求（RED 测试先行，写进 discovery.test.tsx 与适配器测试）

- **身份关联**：微信登录只关联既有 WeKnora 身份/档案，**绝不创建第二份求职档案**（同一用户 Web 已有档案 → 小程序看到同一份；无档案 → 引导建档而非新建平行档案）。
- **找岗**：入口发起一次性 search（消费 searches 合同）；结果行窄屏展示 检查时间/资格状态/原始链接/不确定性；覆盖清单如实（无来源时说明，不用演示数据）。
- **建档**：上传已有简历或逐步建档，抽取事实**逐项确认**（未确认不生效）；冲突回执、未知结果对账（原 requestId 恢复）、空间切换旧响应失效——三个 seam 都要有可观察测试。
- **分享导入**：先展示可核对内容（解析出的 JD/事实预览）再提交；负载缺失/授权失效显示恢复入口。
- **窄屏可见性**：资格冲突、来源状态、待核实项在窄屏完整可见。
- **Web 同步**：同一用户在 Web 修改档案后，小程序侧刷新可见（E2E 中实测）。

## 6. 验证（全跑附原始输出）

```bash
pnpm --filter @weknora/miniprogram test       # 既有 62+ 全绿 + 新增
pnpm --filter @weknora/miniprogram typecheck  # 13 基线错误外零新增
pnpm --filter @weknora/miniprogram build:weapp
git diff --check
```
**真实 DevTools 验证（本轮端口 57809）**：本地 Lite 服务器 + fixture（复现提纲见 task-6-live-validation.md；Career 需真实注册用户与建档/岗位/搜索数据——可用 Web API 直接构造，如实声明构造层级）。官方构建产物（无手工补丁）加载 → 登录关联 → 找岗流程 → 逐项确认建档 → 分享导入预览→提交 → Web 侧改档案→小程序同步可见 → 跨租户拒绝 → 受保护数据访问证据（截图清单入报告）。**真机不可用**（无设备）：如实记录环境门槛（T06 先例：官方 DevTools 模拟器为可达成环境，真机单列 blocked）。

## 7. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree；local commits authorized, no push/merge/deploy/GitHub action；禁止派发子 agent。
- 微信身份只作为关联方式，不生成第二份求职档案；Career scope 服务端派生。
- 不长期缓存 URL/凭据；受控存储；分享负载先核对后提交。
- 测试端口（57809）与构建目录隔离；凭据 disposable 结束清理，报告不留 token。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 8. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t24-miniprogram/` 下写 `task-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、三命令输出、真实 DevTools 各步证据（截图清单、观测记录）、原生例外清单（如有）、已知局限（真机 blocked 等）。COMMIT：按逻辑分批（如 `feat(miniprogram): link wechat identity to career desk` / `feat(miniprogram): discover jobs and confirm facts on weapp`）。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成；T24 verified 由主控裁决。
