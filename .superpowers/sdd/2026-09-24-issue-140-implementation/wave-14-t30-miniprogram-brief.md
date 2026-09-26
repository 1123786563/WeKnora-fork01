# Wave 14 — T30：微信小程序持续规则、额度与提醒（implement）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 依赖均就绪：#154=T13、#160=T20、#161=T21、#164=T24 全部 verified。这是**小程序链最后一个功能票**（T24→T26→T28→T30→T32 中仅剩本票未 verified）。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t30-miniprogram/WeKnora-fork01 --detach 5f32c77b8
cd /Users/wuyongjun/.codex/worktrees/issue-140-t30-miniprogram/WeKnora-fork01
```
BASE = `5f32c77b8`（含全部已集成小程序 Career 能力）。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 30 文本（verbatim）

### Task 30: T30/#170 微信小程序（Taro 4 + TDesign Miniprogram）持续规则、额度与提醒
**Depends:** #154、#160、#161、#164。**Files:** `apps/miniprogram/src/career/rules-usage-reminders.tsx`、`rules-usage-reminders.test.tsx`、`adapters/career-platform.ts`。
**验收：** 可见小程序控件使用 TDesign Miniprogram 或记录明确的原生能力例外；实际开发工具与真机验证；**规则与 Web、移动 App 读写同一版本**；**超额不阻断历史访问**；**订阅消息只提醒同步，不携带敏感岗位细节**；**拒绝订阅后站内待办仍可用**
**原始证据：** Career Desk 规则冲突测试；真实环境检查订阅授权、提醒与隐私文案。**失败处理：** **订阅消息不可用时显示站内待办，不伪造已送达。**
- Step 1-4 框架与验证命令同小程序先例（三命令 RED→GREEN；真实 DevTools 验证；未通过真实设备门槛则保留 blocked）。

## 3. 可消费的服务端合同（集成 HEAD 冻结）

- **规则**（T13）：`POST /rules`、`GET /rules/receipt`、`GET /rules/:ruleId`（下次运行计划/可见状态）。
- **额度**（T21）：`GET /usage/estimate` + 收费 act 内嵌 admission（超额 typed 拒绝 429、既有档案申请永读）。
- **待办/提醒**（T20）：`POST/GET /reminders` + `GET /reminders/receipt`（隐私正文冻结模板零公司/岗位/面试细节；退订停推待办可读）。
JSON 以对应 go 文件为最终依据。小程序侧复用既有 career service/adapter 模式扩展（`adapters/career-platform.ts` 唯一所有者）；微信**订阅消息**用 `wx.requestSubscribeMessage`（平台原生 API，属记录明确的原生能力例外——不在 TDesign 组件域）。

## 4. 工程事实（沿用）

TDesign 构建机制/DevTools 验证模式（T06/T24 修复链成果）；typecheck 13 基线豁免；T24/T26/T32 三教训为验收前置（可区分截图/无死代码提示/unknown 门控对齐——T32-M1 刚修复的口径：unknown/对账中主按钮禁用+重新对账入口）。

## 5. 行为要求（RED 测试先行）

- **规则同版本**：小程序读写与 Web 同一规则（同 API；修改后下次运行计划更新呈现——对齐 T13-Web 语义）；规则冲突（revision/同 ID 变更）回执呈现。
- **额度**：执行前预估展示（将消耗+触发条件）；超额只阻新收费动作，**历史（档案/申请/时间线）完整可访问**（关键断言）；重复请求不二扣呈现。
- **订阅消息**：`wx.requestSubscribeMessage` 请求授权（模板 id 处理如实声明环境限制）；**拒绝订阅后站内待办仍可用**（收件箱呈现——复用 T20 合同）；**订阅消息只提醒同步，正文不携带公司/岗位/面试细节**（与后端隐私模板一致，前端不额外拼敏感内容）。
- **订阅消息不可用**（模拟器不支持/拒绝）：显示站内待办，**不伪造已送达**。
- 冲突回执/未知对账/空间切换三 seam 可观察测试。

## 6. 验证（全跑附原始输出）

```bash
pnpm --filter @weknora/miniprogram test       # 157 基线+新增全绿
pnpm --filter @weknora/miniprogram typecheck  # 13 基线外零新增
pnpm --filter @weknora/miniprogram build:weapp
git diff --check
```
**真实 DevTools 验证（本轮端口 57827，隔离临时 DB）**：官方构建加载→登录→规则页（同版本读写/下次计划）→额度预估→消耗至超额→**历史仍可访问**→订阅授权流程（模拟器限制如实声明；拒绝路径必测）→拒绝后站内待办可读→隐私文案核对→跨租户拒绝。截图逐张可区分；真机无设备单列 blocked。

## 7. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree；local commits authorized, no push/merge/deploy/GitHub action；禁止派发子 agent。
- 订阅消息不可用时显示站内待办，不伪造已送达；不长期缓存 URL/凭据。
- 测试端口（57827）与构建目录隔离；凭据 disposable 结束清理。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 8. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t30-miniprogram/` 下写 `task-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、三命令输出、DevTools 各步证据（可区分截图+订阅拒绝路径+超额历史访问）、原生例外清单、已知局限（真机 blocked 等）。COMMIT 按逻辑分批。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成；T30 verified 由主控裁决。
