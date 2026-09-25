# Wave 8 — T13 Web 报告：持续找岗规则页面（Issue #154）

- 实现者：frontend_implementer（T13-Web，独立 worktree）
- 现场：`/Users/wuyongjun/.codex/worktrees/issue-140-t13-web/WeKnora-fork01`（detached）
- BASE `24708145c` → HEAD `6945af9f0`（`feat(web): manage recurring search rules with run plans`；本地提交，未 push；单提交）
- 详细版报告（含相位级 E2E 证据与披露）：worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t13-web/task-2-report.md`

## 文件清单（10 files changed, 912 insertions(+), 2 deletions(-)）

create：`apps/web/src/career/RulePage.tsx`、`apps/web/src/career/RulePage.test.tsx`、`apps/web/src/career/rule.css`
modify：`packages/api-client/src/career.ts`（setRule/ruleReceipt/getRule + decodeSetRuleReceipt/decodeRuleView 严格解码，冻结枚举/边界/RFC3339/http 链接）、`packages/api-client/src/career.test.ts`（3 聚焦测试）、`apps/web/src/router.tsx` + `routes.tsx` + `routes.test.ts`（/platform/career/rules 路由与 allowlist）、`apps/web/src/platform/PlatformShell.tsx` + `platform-shell-nav.test.ts`（"持续找岗"导航项，exact match）。

**披露**：路由/导航注册 5 文件超出简报 Files 字面清单——独立 RulePage 可达是浏览器 E2E 关键验收的机械前提，改动与已集成 T11-Web 完全同型；并入 SearchPage 的替代方案会与其冻结测试（no continuous-search control）冲突，不采纳。

后端未动；career-core 合同未动。临时 Go 触发工具 `cmd/t13ruletrigger`（E2E 中充当 seam 调度方）已删除、未提交。

## RED → GREEN（node26.4.0 真实输出）

- api-client RED（实现前）：`tests 25 / pass 22 / fail 3`，全为 `api.setRule/ruleReceipt/getRule is not a function` → GREEN `25/25`。
- RulePage RED：文件级 `ERR_MODULE_NOT_FOUND` → 首版 5/9 → 修复（React19 惰性 updater 提前读 value；更新携带既有 ruleId）→ GREEN `9/9`；注册测试聚焦 `31/31`。

## 全量验证（真实输出）

| 命令 | 输出 |
| --- | --- |
| `pnpm typecheck:web` | 通过（exit 0，0 error TS） |
| `pnpm test:web` | `ℹ tests 2430 / ℹ pass 2430 / ℹ fail 0 / ℹ cancelled 0`，exit 0 |
| `pnpm build:web` | `✓ built in 10.84s`，exit 0 |
| `git diff --check` | 干净（exit 0） |

首跑 `agent-editor.test.tsx` 自旋（99.7% CPU 15+ 分钟，T11-Web 记录过的环境现象，非代码缺陷），按 Wave 3-7 先例 kill 后干净重跑全绿；跑前清理两组孤儿 runner。

## 浏览器 E2E（端口 57814，关键验收）

真实 Chromium + Lite SQLite（隔离一次性库）+ vite 57816 代理。**11/11 相位 OK**：

1. 登录（真实注册+登录）→ /platform/career/rules：**默认停用** radio + "默认不开启/不会在后台运行"文案。
2. 创建规则（默认关闭）：条件/频率/状态可见，**预计消耗启用前可见**（后端数字 + basis 英文原文逐字 + "前端如实展示不重算、非额度余额" + 额度耗尽可读声明）；计划=规则未启用。
3. 启用 → **下次运行计划**（UTC 时间行）；修改（杭州 后端/60 分钟）→ **计划更新显示**，旧时间消失。
4. 暂停 → "已暂停：下一次触发已取消…恢复启用后重新排程（顺延）"，无排程时间。
5. **真实 seam 触发**（临时 Go 工具经 `WithScope`+`TriggerDueRules` 直连隔离库）：period 1/2 均 `no_vetted_sources`；同刻重复 sweep `triggered: none due`（对账不复制）；页面执行历史两行均可见状态 + 后端 note 原文，无第三行。
6. 未知回执恢复：POST /rules 连接中断 → 原 requestId 面板 → 查询回执（真实 404）→ "暂未找到回执" → 同 requestId 重试成功（captured body 复用同 ID）。
7. 发现待办空态如实 + "同一岗位链接只生成一条待办（服务端按链接去重）"。
8. HTTP 探针：未认证 401；跨租户读他人规则 404 not_found；同 ID 同内容重放 200 同 receipt；同 ID 异内容 409 idempotency_conflict。

截图 8 张 + evidence.log + e2e-run.mjs + server/vite log + captured.json（**已剔除 bearer token**）：本目录 `t13-web-shots/`。服务器/vite 已停，57814/57816 已释放。

## 约束自查

- 只改上述 10 文件；后端/career-core/主仓库未动；本地提交未 push；未派发子 agent。
- WeKnora TDesign 浅色主题 + `#07c05f`（rule.css，search.css 同款约定）。
- 规则默认不开启、显式启停、无暗中后台运行暗示；预计消耗后端口径如实展示不重算；暂停取消顺延可见；blocked 状态不静默；未知回执原 requestId 对账；空间切换 abort 清缓存。
- git commit 时 Mimosa hook `scanner_enobufs`（T11-Web 同现象；不据此做安全声明）。

## 遗留 / 转述

- T13 后端评审 4 low（sweep 串行中止、触发 revision 竞态、todo 无闭环、一条 T11 测试语义保持改写）：不阻断，转述自 wave-report-T13后端.md。
- todos 与 blocked_no_quota 无法在真实 Lite 产生（空 registry 与 pass-through 配额门为后端设计，T21 未落地）：UI 呈现由组件测试以冻结 payload 覆盖；todo 唯一性属后端唯一索引（后端测试已证）。
- 留在 worktree 等独立评审与集成；T13 verified 由主控裁决。
