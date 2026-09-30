# Wave 4 — T11 Web 报告：一次性找岗页面与恢复历史（Issue #152）

- 实现者：frontend_implementer（T11-Web，独立 worktree）
- 现场：`/Users/wuyongjun/.codex/worktrees/issue-140-t11-web/WeKnora-fork01`（detached）
- BASE `6a21e0df5` → HEAD `36c963c36`（`feat(web): run one-shot searches with recovery`；本地提交，未 push；单提交）
- 详细版报告（含 E2E 截图清单）：worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t11-web/task-2-report.md`

## 文件清单（10 files changed, 853 insertions(+), 2 deletions(-)）

create：`apps/web/src/career/SearchPage.tsx`、`apps/web/src/career/SearchPage.test.tsx`、`apps/web/src/career/search.css`
modify：`packages/api-client/src/career.ts`（searchOnce/searchReceipt/search + decodeSearchOnceReceipt 严格解码）、`packages/api-client/src/career.test.ts`（3 聚焦测试）、`apps/web/src/router.tsx`（career/search 路由）、`apps/web/src/routes.tsx` + `routes.test.ts`（platform allowlist）、`apps/web/src/platform/PlatformShell.tsx` + `platform-shell-nav.test.ts`（"找岗"导航入口，exact match）。

后端未动；career-core 合同未动；无缺口（既有 `opportunityEvidencePath`/`importUrl` 复用）。

## RED → GREEN

- api-client RED（tsx --test，实现前）：`15 tests / 12 pass / 3 fail`，全为 `api.searchOnce is not a function` → GREEN（node26.4.0）：`15/15`。
- SearchPage RED（node26.4.0，实现前）：文件级 `ERR_MODULE_NOT_FOUND SearchPage.tsx` → 首版 7/10 → 修复（回执 requestId 不一致改 typed 错误；TDesign 禁用 submit 为 div 的定位修正；startNewSearch 清 draft）→ GREEN：`10/10`。
- 过程中修复 1 项测试资产过期：`platform-shell-nav.test.ts` M2 完整序列断言因新增入口更新，并新增 T11 入口测试（nav 5/5、routes 15/15、router 10/10 聚焦全绿）。

## 全量验证（node26.4.0，真实输出）

| 命令 | 输出 |
| --- | --- |
| `pnpm typecheck:web` | 通过（无错误输出） |
| `pnpm test:web` | `ℹ tests 2389 / ℹ pass 2389 / ℹ fail 0 / ℹ cancelled 0`，EXIT=0 |
| `pnpm build:web` | `✓ built in 23.74s` |
| `git diff --check` | 干净（exit 0） |

test:web 首跑与浏览器 E2E 并行时 `src/agents/agent-editor.test.tsx` 自旋 14+ 分钟（R 态、8:40 CPU、日志冻结），按简报 Wave 3 先例 kill 后干净重跑全绿——环境资源竞争，非代码缺陷（首跑污染汇总另暴露 M2 断言过期，已修）。

## 浏览器 E2E（端口 57808，关键验收）

真实 Chromium + Playwright；Lite 57808（隔离 SQLite，迁移 0→120）；Vite 57809。链路全通：
1. 登录（真实注册+登录）→ 侧边栏"找岗"入口。
2. 失败路径（真实空 registry）：alert"找岗未完成：暂无已核验来源，本次未抓取任何数据"+ 两条后端 scopeNotes 原文 + "本次没有结果"；整页无"全国"字样（`hasNationwide=false` 实测）。
3. 刷新 → 历史找岗列表 + 最近回执按原 searchId 自动恢复（真实 GET /searches/:id，同编号）。
4. 结果列表（fixture 代理 57810，仅拦截两个 searches 端点、其余透传真实 Lite）：两行结果各含 检查时间/资格状态（待人工判断·符合）/原始链接/不确定性（低置信度）+ 覆盖来源（城市/访问方式）。代理首版缺 CORS 头意外触发 unknown 态 → "查询回执"（真实 404 → "暂未找到回执"）→ 同 requestId 重试恢复——unknown 恢复路径完整实测。
5. 点结果"导入为岗位证据"→ 真实 import-url 创建 opportunity → "查看岗位证据"进入既有 OpportunityPage 证据页（环回未过 SourcePolicy → 提取字段"未知"、空正文，如实无伪造）。
6. 重进找岗页 → 历史与结果再次恢复（fixture 路径）。
截图 5 张：worktree `.superpowers/sdd/2026-09-24-issue-140-t11-web/evidence/t11-e2e-01…05-*.png`。环境已全部停止并清理（隔离库/存储/一次性凭据销毁）。

## 约束自查

- 只改简报所有权文件 + 授权的入口最小改动；未动后端/career-core/主仓库。
- WeKnora TDesign 浅色主题 + `#07c05f` 品牌色（search.css）。
- 零持续规则控件（页面无 checkbox/radio/switch，测试 count=0；"持续找岗规则"属 T13）。
- 所有写入 requestId+expectedRevision；revision_conflict 显示当前修订；空间切换清缓存（Desk 语义）。
- 覆盖清单如实，无全国表述、无演示数据；未知回执先 receipt 恢复、同 ID 重试；不复制搜索（终态禁用 + 重放同 search）。
- 本地提交未 push；未派发子 agent。

## 遗留 / 转述

- T11 评审 4 条 low：Wave 3 裁定不阻塞、不捆绑，留 T33 收口统一清点（progress.md:182、189）；具体条目未随简报下发，不在本任务范围。
- 历史为本地索引（按 userId:tenantId 键控 localStorage）+ 服务端按 ID 复核；后端无列表端点（冻结面所致），不声称服务端全量历史。
- git commit 时 Mimosa hook `scanner_enobufs`（未取得完整扫描结论按兼容策略放行）；本报告不据此做安全声明。
- `agent-editor.test.tsx` 重负载自旋为环境现象（干净重跑全绿）；如复发建议单独开票。
