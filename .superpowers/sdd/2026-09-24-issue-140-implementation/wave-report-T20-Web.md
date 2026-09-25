# T20 Web 子任务报告：站内待办收件箱（implement）

- BASE：`14b81d24c`（Wave 11 集成 HEAD，T20 后端已集成并评审通过）
- HEAD：`921f9988b`（`feat(web): read privacy-safe inbox todos`，本地提交，未 push）
- worktree：`/Users/wuyongjun/.codex/worktrees/issue-140-t20-web/WeKnora-fork01`
- 日期：2026-09-26 Asia/Shanghai

## 1. 交付物

| 文件 | 内容 |
| --- | --- |
| `packages/api-client/src/career.ts` | T20 reminder 冻结合同消费：`ReminderReceipt/ReminderView/ReminderList/SetReminderInput` 类型、冻结隐私正文表镜像、`decodeReminderReceipt/decodeReminderView/decodeReminderList` 解码器、`setReminder/reminders/reminderReceipt` 三方法 |
| `packages/api-client/src/career.test.ts` | 6 个新测试（编码/拒绝/隐私冻结/推送报告/列表形状） |
| `apps/web/src/career/InboxPage.tsx` | 收件箱页面：隐私待办列表、进入权威申请深链、登记待办（request ID + expected revision）、去重呈现、推送订阅管理、失败/权限/未知回执恢复态 |
| `apps/web/src/career/InboxPage.test.tsx` | 7 个用户可观察行为测试 |
| `apps/web/src/career/inbox.css` | TDesign 浅色主题 + `#07c05f` 品牌色样式 |
| `apps/web/src/career/CareerPage.tsx` | 最小入口：`<InboxPage>` 嵌入（同 ExportDeletionPage 先例） |
| `apps/web/src/career/CareerPage.test.tsx` | mount stub 默认 `reminders`（1 行） |

后端与 career-core 合同零改动。迁移：无（无 DB 变更）。

## 2. 行为与事实源对齐

- **隐私正文**：解码器强制 notice 必须等于后端冻结模板字面量（`progress_updated` → "你有新的求职进展，请登录查看。"；`discovery_found` → "持续找岗有新发现，请登录查看。"），任何插值正文（公司/岗位/面试细节）在到达 UI 前被 `TypeError` 拒绝。页面横幅呈现"推送只是提醒，站内待办才是事实源；通知正文不含公司、岗位或面试详情"。
- **去重语义**：后端 `(tenant,user,source_kind,source_id)` 唯一索引是事实源；UI 侧 `deduplicated:true` 回执呈现"该来源已有待办：同一来源事件只保留一条，未新增第二条"，列表每来源恒一行。
- **站内权威**：`ReminderView` 结构上不含公司/岗位/面试字段；`push` 报告仅存在于回执（response-only），`delivery_failed` 时 UI 显示"推送投递失败——待办已保存，以站内为准；推送失败不改变站内事实"。
- **订阅管理**：退订 = 通过 house confirm 流写档案事实 `notifications.push=unsubscribed`（`client.career.act`，request ID + expected revision）；订阅状态从 `career.open` 的 facts 读取（**后端冻结面如此**：`ListRemindersHandler` 响应仅含 `{"reminders":[...]}`，无订阅字段——简报 §3 所述"待办列表+订阅状态"以后端实核为准，前端分两个读取面实现，未改后端）。
- **恢复态**：写路径统一 request ID + pinned revision；`outcome_unknown` → 查询回执（`reminderReceipt`/`receipt`）/同编号重试；`revision_conflict` → 显示当前修订并刷新档案修订；`forbidden` → 跨租户错误态 + 清除；`not_found` → 来源不存在（可能不属于当前空间）。

## 3. TDD 证据

### 3.1 api-client（packages/api-client）

**RED**（`red-api-client.txt`）：实现前运行 `node --import tsx --test src/career.test.ts`，5 个新测试全部失败于 `TypeError: api.setReminder is not a function` / `api.reminders is not a function`，既有 35 个测试全绿。

**GREEN**（`green-api-client.txt`）：实现后同命令 `ℹ tests 40 / ℹ pass 40 / ℹ fail 0`。

GREEN 轮修正（记录）：
- 测试 fixture 最初缺 `deduplicated` 字段（后端该字段无 omitempty，恒在 wire 上）→ fixture 补齐（测试缺陷）。
- discovery 分支 stub 未按请求体回对应 receipt → stub 修正（测试缺陷）。
- `deepStrictEqual` 区分"缺键"与"值为 undefined 的键"：解码器按仓库先例省略缺省键 → 期望对象改为显式字面量（测试缺陷）。

### 3.2 InboxPage（apps/web）

**RED**（`red-inbox-page.txt`）：实现前运行 `node --import tsx --test src/career/InboxPage.test.tsx`，失败于 `ERR_MODULE_NOT_FOUND: Cannot find module '.../InboxPage.tsx'`（页面不存在，7 测试全 RED）。

**GREEN**（`green-inbox-page.txt`）：`7/7 pass, 0 fail`。

GREEN 轮修正（记录）：
- 成功通知最初被 `writePhase !== 'idle'` 门控隐藏 → 引入独立持久 `notice` 状态（实现缺陷）。
- 订阅测试 stub 返回固定 requestId 触发 ReceiptMismatchError → stub 回显请求编号（测试缺陷）。
- forbidden 分支补「重新读取」按钮（实现按测试要求补齐）。

### 3.3 既有面回归

- `CareerPage.test.tsx`：13/13 通过（mount stub 加 `reminders` 默认值后无其他改动）。

## 4. 全量验证（真实输出摘要）

| 命令 | 结果 |
| --- | --- |
| `pnpm typecheck:web` | 通过（无错误输出；中途修复一处 `SubscriptionAttempt` 类型收窄） |
| `pnpm test:web`（改前基线） | `ℹ tests 2459 / pass 2459 / fail 0 / cancelled 0` |
| `pnpm test:web`（交付后终验） | `ℹ tests 2466 / pass 2466 / fail 0 / cancelled 0`（2459 基线 + 7 InboxPage 测试；node26 v26.7.0） |
| `pnpm build:web` | `✓ built in 16.96s`（仅既有 chunk>500kB 提示） |
| `git diff --check` | 干净（无空白错误） |

## 5. 浏览器 E2E（T20 verified 关键验收）

### 5.1 环境

- Lite 服务器：隔离临时目录 SQLite/local/memory，loopback `127.0.0.1:57821`（`DB_DRIVER=sqlite`、`STREAM_MANAGER_TYPE=memory`、`STORAGE_TYPE=local`、生成 JWT/AES、一次性随机口令账号；迁移目录随服务器启动载入）。跑毕进程已停、端口已释放、临时目录已删除。
- Vite dev：`127.0.0.1:57823`，`VITE_DEV_PROXY_TARGET=http://127.0.0.1:57821`。
- 真实浏览器：Playwright（Chromium）。

### 5.2 声明层级

- 事件/待办构造：**HTTP API 层**（python 标准库直连 57821）：注册/登录 → confirm 档案事实 → import 机会（rawText 含隐私 bait：字节跳动/高级后端工程师/一面）→ evaluate → create application → append progress（note 含 bait）→ `POST /reminders` ×2（不同 requestId）。
- 浏览器只做登录、读取与导航（用户可观察行为）。

### 5.3 观察结果

| 步骤 | 观察结果 |
| --- | --- |
| API 链路 | 全链 200：两次登记同一来源 → 第二次 `deduplicated:true`；列表恰 1 条；通知正文等于冻结字面量且不含 bait 词（字节/高级后端工程师/面试/一面/岗位 均未出现）；receipt 端点回放 200 |
| 登录 → /platform/career | 收件箱区块出现，订阅状态"已订阅推送提醒"，待办正文为冻结字面量（见截图 01） |
| 待办 → 进入权威申请详情 | 深链 `/platform/career/opportunities/{oppId}?snapshotId={snapId}&application={appId}`，页面显示"申请已创建：申请编号 12be4888… · Task 已就绪"、固定证据（岗位/快照/评估/档案修订/批次）（见截图 02） |
| 重复触发（UI 第三次登记同一事件） | 提示"该来源已有待办：同一来源事件只保留一条，未新增第二条"，列表仍 1 行（见截图 03） |
| 退订推送（UI） | 状态"已退订推送提醒（站内待办不受影响，仍可读取）"+ 提示"不再发送推送，已存在的站内待办仍可读取"；待办仍完整可读；提供"重新订阅推送提醒"（见截图 04） |
| 退订事实持久化（API 核对） | `career/open` facts 含 `notifications.push=unsubscribed`，revision 2 |
| 跨租户拒绝（API 层声明） | 账号 B 携 A 的 X-Tenant-ID 读 `/career/reminders` → **403**；B 读 A 的 application → 404（不可区分设计） |
| 浏览器 console | 0 errors |

截图（`.superpowers/sdd/2026-09-24-issue-140-t20-web/screenshots/`）：
`t20-e2e-01-inbox-privacy.png`、`t20-e2e-02-authoritative-application.png`、`t20-e2e-03-dedupe-single-row.png`、`t20-e2e-04-unsubscribed-todos-readable.png`。

### 5.4 推送失败场景（fixture/seam 层级声明）

生产装配 `reminderNotifier = nil`（无推送通道），live 服务器回执无 `push` 字段，无法在真机触发投递失败。该场景覆盖于：
1. Web 单元测试（stub 层）：`InboxPage.test.tsx` "a push delivery failure never loses the in-station todo" —— `push:{attempted:true,delivered:false,reason:'delivery_failed'}` 回执下待办仍保存、UI 呈现"以站内为准"；
2. 后端 Wave 11 单元测试（`remindAfterCommit` 的 delivery_failed 语义，评审通过）。
未在 live 链路注入故障。

## 6. 已知局限与遗留

- **后端遗留（Wave 11 评审结论转述，非本轮引入）**：1 medium 并发缺口（可自愈）+ 2 low。本轮 Web 面未发现新增缺口；消费的 API 面与集审冻结一致。
- **订阅状态读取面**：`GET /reminders` 响应不含订阅状态（后端冻结面），前端经 `career/open` facts 读取；如后续后端在列表响应中补充订阅字段，前端可单点切换（一处派生）。
- **discovery 待办入口**：链接到 `/platform/career/search`（发现所在面板）；无申请深链（该来源本无申请，行为如实）。
- 提交时 Mimosa 钩子提示 commit 前完整扫描未完成（scanner_enobufs，按兼容策略放行）；本报告不据此宣称项目安全，建议后续完整审计复扫。
- T20 verified 由主控裁决；本报告仅陈述事实与实测输出。

## 7. 验证命令清单（均可复跑）

```bash
cd /Users/wuyongjun/.codex/worktrees/issue-140-t20-web/WeKnora-fork01
pnpm typecheck:web
pnpm test:web
pnpm build:web
git diff --check
node --import tsx --test src/career/InboxPage.test.tsx   # (apps/web)
node --import tsx --test src/career.test.ts              # (packages/api-client)
```
