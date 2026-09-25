# Wave 11 — T32：微信小程序导出与删除（implement）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 依赖均就绪：#162=T22 verified（导出/删除后端合同）、#164=T24 verified（小程序 Career 基础设施；T26 已再验证同源合同模式）。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t32-miniprogram/WeKnora-fork01 --detach ff5b2e843
cd /Users/wuyongjun/.codex/worktrees/issue-140-t32-miniprogram/WeKnora-fork01
```
BASE = `ff5b2e843`（含 T26 verified 的小程序申请/材料/投递与同源合同模式）。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 32 文本（verbatim）

### Task 32: T32/#173 微信小程序（Taro 4 + TDesign Miniprogram）导出与删除
**Depends:** #162、#164。**Files:** `apps/miniprogram/src/career/export-deletion.tsx`、`export-deletion.test.tsx`、`adapters/career-platform.ts`。
**验收：** 可见小程序控件使用 TDesign Miniprogram 或记录明确的原生能力例外；实际开发工具与真机验证；**导出内容与其他端一致**；**删除说明保留规则和外部平台边界**；**删除后微信账号重进不恢复旧私有资料**；**旧材料链接和本地缓存不可访问**
**原始证据：** Career Desk 删除与重新登录测试；真实环境检查导出、删除、重进和旧授权。**失败处理：** **导出能力受平台限制时提供等效可取得的文件流程，不静默截断。**
- Step 1-4 框架与验证命令同小程序先例（`pnpm --filter @weknora/miniprogram test && typecheck && build:weapp`；RED→GREEN；真实 DevTools 验证；未通过真实设备门槛则保留 blocked）。

## 3. 可消费的服务端合同（集成 HEAD 冻结）

T22 五路由（`internal/router/routes_career.go:57-61`）：`POST /exports`、`GET /exports/receipt`（导出状态/下载）、`GET /deletions/boundary`（边界清单）、`POST /deletions`、`GET /deletions/receipt`（删除状态+保留披露）。JSON 以 `internal/modules/career/career_export.go` 为最终依据。Web 端 ExportDeletionPage（本 BASE 已集成）呈现语义可作参照（导出六段包含性/边界强制确认/部分失败可恢复不称完全删除/删除后旧授权 404 复验）。小程序侧复用 T26 的 career service/adapter 模式扩展。

## 4. 工程事实（沿用，勿重蹈覆辙）

- TDesign 构建机制/事件绑定/DevTools 验证模式（T06/T24 修复链成果）；typecheck 13 个 CommercialSummary 基线豁免。
- T24/T26 教训为验收前置：证据截图逐张可区分（字节级不同+关键元素断言）、无死代码提示（所有提示真实可达）、404/未知分支有恢复态。
- 小程序文件保存/打开经 T06 openDocument+受控清理模式；导出下载认证兑付（带 Authorization）。

## 5. 行为要求（RED 测试先行）

- **导出**：发起→状态→**保存导出包到本机/可取得文件**（openDocument 或保存并提示路径——受平台限制时提供等效可取得流程并如实说明，**不静默截断**）；导出内容与 Web 端一致（同后端包）。
- **删除**：先呈现**保留规则与外部平台边界**清单→用户确认→发起→状态（进行/部分失败+可恢复/完成+保留披露）；**绝不显示"已完全删除"当部分失败**。
- **重进不恢复**：删除完成后**退出登录→重新登录（同一微信账号关联）→旧私有资料不出现**（服务端已删+本地 storage 缓存清理双重验证）。
- **旧链接/缓存不可访问**：删除前保存的材料/导出链接删除后访问被拒（404 态+恢复态为零数据）；本地缓存（storage 中的 career 数据）清理。
- 冲突回执/未知对账/空间切换三 seam 可观察测试。

## 6. 验证（全跑附原始输出）

```bash
pnpm --filter @weknora/miniprogram test       # 118 基线+新增全绿
pnpm --filter @weknora/miniprogram typecheck  # 13 基线外零新增
pnpm --filter @weknora/miniprogram build:weapp
git diff --check
```
**真实 DevTools 验证（本轮端口 57819，全新隔离临时 DB）**：官方构建加载→登录→构造数据→导出→保存/打开→边界确认→删除→完成态→**重进（重新登录）验证不恢复**→旧链接拒绝→缓存清理验证→跨租户拒绝。截图逐张可区分；真机无设备单列 blocked。

## 7. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree；local commits authorized, no push/merge/deploy/GitHub action；禁止派发子 agent。
- 删除验证只用隔离临时数据库；不长期缓存 URL/凭据；凭据 disposable 结束清理。
- 测试端口（57819）与构建目录隔离。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 8. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t32-miniprogram/` 下写 `task-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、三命令输出、DevTools 各步证据（可区分截图+重进验证+旧链接拒绝记录）、平台限制等效流程说明（如有）、已知局限（真机 blocked）。COMMIT 按逻辑分批。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成；T32 verified 由主控裁决。
