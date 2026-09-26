# Wave 14 — T30/#170 微信小程序持续规则、额度与提醒 — 实现报告

- 完整报告：worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t30-miniprogram/task-report.md`（含 RED/GREEN 原始输出、fixture/驱动/订阅探针日志、14 截图 manifest、脱敏 fixture）
- Worktree：`/Users/wuyongjun/.codex/worktrees/issue-140-t30-miniprogram/WeKnora-fork01`（BASE `5f32c77b8` → HEAD `2041db18c`，本地提交未 push）
- 提交：`6aa2d78eb`（RED→GREEN 服务层+15 测试）、`2041db18c`（页面+config+注册）

## 验证结果（全过）

| 命令 | 结果 |
| --- | --- |
| `pnpm --filter @weknora/miniprogram test` | **172**（157 基线 + **15 新增**）全绿，0 fail（基线 5 skipped 为 build-output 条件跳过，构建后在场景中实跑通过） |
| `pnpm --filter @weknora/miniprogram typecheck` | **13**（全部为基线豁免 `features/account/pages.tsx`；新增/改动文件 **0 新增**） |
| `pnpm --filter @weknora/miniprogram build:weapp` | Compiled successfully（页面产物 dist/career/rules-usage-reminders.* 产出） |
| `git diff --check` | clean |

## 行为要点（简报 §5 逐条）

- **规则同版本**：POST /rules、GET /rules/receipt、GET /rules/:id 与 Web 同一冻结合同与 CAS 域（档案头修订）；app 写入（修订 2）与 Web 写入（修订 3）同一 ruleId 同一历史；下次运行计划三态呈现（enabled+nextDueAt / paused 顺延 / disabled 不运行）；A1/A2+A4/A4b 钉死。
- **额度**：GET /usage/estimate 执行前只读展示（costUnits+触发条件后端原文不重算）；超额只阻新收费（typed 429 search_quota_refused），**历史（档案/申请/时间线）完整可访问**——单测 B2 + live exhaust 探针双重断言；重复请求同 requestId 幂等重放不二扣（B3+live）。
- **订阅消息**：`wx.requestSubscribeMessage` 原生例外（Ruling）——载荷只带模板 id（C1 断言 wire payload 无岗位文本）；拒绝/主开关/不可用如实呈现并回落站内待办，**绝不伪造已送达**（outcome.delivered 恒 false）；正文口径=后端冻结隐私模板（C4 断言收件箱正文恰为两条冻结字面量）。
- **拒绝订阅后站内待办可用**：C2 单测 + live「查看站内待办」读取成功；退订（notifications.push=unsubscribed 事实写）只停推送待办可读（C6+live T30-10）。
- **三 seam 可观察**：冲突回执（A3 revision_conflict typed 无 intent）/ 未知对账（A4/A4b 原 requestId+原修订幂等重放）/ 空间切换（A5 SCOPE_CHANGED 零残留）。
- **T32-M1 口径**：未知规则保存恢复态置顶+主按钮禁用+对账/原编号重试双入口（live T30-0 截图）。

## 真实 DevTools（57827 隔离临时 DB + automator 9436）

- fixture 探针 **26/26**（setup 14 + webedit 2 + exhaust 4 + verify 6）；App 驱动 **16/16**（online 6 + webchat 7 + cross 3）；**14 截图 14 个不同 sha256**；关键截图经图像模型复核。
- `CAREER_SEARCH_QUOTA_LIMIT=2` 真实耗尽：第 3 次 charged 搜索 429 typed 拒绝；超额后 open/申请/进展 GET 全 200。
- 跨租户：B 读 A 规则 404 not_found（scope 隔离读不泄露存在性，T26 H1 同语义）；B 额度/收件箱零泄漏。
- **订阅消息原生探针**：模拟器内 API 在场（typeof function）；未配置模板真实调用失败不弹层 → 页面如实 unavailable+errMsg+站内回落。**拒绝路径真机级弹层环境不可得**（touristappid 无模板可配）——seam 由单测 C2 injectable invoke 钉死，如实声明非未实现。

## 已知局限（blocked 单列）

- **真机验证 blocked**（无设备）：可补订阅授权真实弹层（含拒绝路径）与真实送达链路。
- t-button GUI 直点不被 automator 触达（T24 已知限制）：保存按钮业务链=172 单测+恢复入口真实 POST（同一 client.request 链）组合覆盖（T28 同口径）。
- 订阅模板 id 未配置（`REMINDER_SUBSCRIBE_TEMPLATE_IDS=[]`，与 appid 绑定需公众平台申请）：未配置→如实不可用+站内回落。

## 环境清理（已执行）

57827/9436 空闲；DevTools/服务器停止；SQLite、fixture.json（含凭据）、automator、project.private.config.json 删除；脱敏归档于 worktree `.superpowers/sdd/2026-09-24-issue-140-t30-miniprogram/`（grep Bearer/密码=0）。

## 文件清单

- 新增：`apps/miniprogram/tests/rules-usage-reminders.test.mjs`、`apps/miniprogram/src/career/rules-usage-reminders.tsx`、`apps/miniprogram/src/career/rules-usage-reminders.config.ts`
- 修改：`apps/miniprogram/src/adapters/career-platform.ts`（+216 行 T30 服务层与订阅 seam）、`apps/miniprogram/src/app.config.ts`（career 分包注册）
- 注：计划 Files 列的 `rules-usage-reminders.test.tsx` 落在本仓可执行测试约定 `tests/*.test.mjs`（T24/T26/T28 同款）；config/app.config 为平台机械必需。
