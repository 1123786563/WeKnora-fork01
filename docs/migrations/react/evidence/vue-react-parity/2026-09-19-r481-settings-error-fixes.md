# R481 证据：settings 错误态六缺口修复（R028/R030/R034/R040/R041/R042）

- 日期：2026-09-19；三修复代理并行（文件域互斥）+ 协调者合并验证 + 独立浏览器复验代理
- 基线：R480 锚定（`2026-09-19-r480-settings-error-anchoring.md`，21 截图）
- 截图：`screenshots/r481-20260919/`（10 张，修复后 React 端各分区 500 态）
- 代理报告：`.omc/state/r481/report-{A1,A2,A3,verify}.md`
- 提交：见台账（代码 7 文件 +300/−35 + 文档 + 截图）

## 改动清单（TDD 红→绿）

| 域 | 文件 | 内容 | 红→绿 |
|----|------|------|-------|
| A1 页级策略 | `apps/web/src/settings/SettingsPage.tsx` + `settings-error-ux.test.tsx` | `sectionErrorMode` 重排：**silent**（新类型）= storage/vectorstore/websearch/weknoracloud/ollama/retrieval（加载失败 setError(null)，面板以 null payload 渲染空态/默认值）；banner-retry = members/parser/system/userprofile（后三者**新增**，补齐 Vue 内嵌重试）；models/skills/mcp/members 原样。过时注释（StorageEngineSettings 错误归因）改引 R480 基线。重试文案 parser/system/userprofile 用 `settings.parser.retry`（5 locale 与 Vue system.retry/tenant.retry 逐字节一致，零新键） | 10 红 → 15/0；SettingsPage.test 23/0 回归 |
| A2 Ollama 面板 | `OllamaSettingsPanel.tsx` + 其 test | 加载失败（initialValue null）或 available=false 且非 testing → 地址行下渲染 `<Status tone="warning">` = `ollamaSettings.address.failed`（Vue t-alert theme=warning 逐键对齐）；`loadFailed` state 在 refresh 成功后清除；不透传原始错误体；顶部「重新检测」即重试 | 2 红 → 7/7 |
| A3 检索抽屉 | `platform/retrieval-settings-panel.tsx` + 新 test + `ConfigSettingsPanel.test.tsx` | `retrieval.get()` 失败 `.catch(() => null)` 静默降级 → ConfigSettingsPanel(initialValue=null) 渲染 Vue 默认表单（Top K=50、阈值默认、Rerank 空）；删裸错误 `<p>` 分支；models 回退 [] 保持；ConfigSettingsPanel 实现**零改动**（null 默认值行为由新测试锁死：outputs 50/0.15/0.30/10/0.20） | 1 红 → 10/10 |

合并 scoped 五套件（node v26.4.0）：**55/55，exit 0**；typecheck:web exit 0。

## 门禁

- `pnpm gates` 首跑：test:shared 1 失败 —— `performance harness: authoritative state restored within the frozen window (5s)` 实测 64s（packages/domain/src/mobile/compatibility.test.ts）。与本次改动无关（test:shared 不含 settings 文件）；**负载型抖动**（浏览器复验代理并行抢 CPU）。单独复跑该文件 11/11 绿。
- 无并行负载复跑完整 gates：**六门禁全绿**（test:shared / typecheck:shared / test:web / typecheck:web / check:integrity / build:web，exit 0）。

## 浏览器复验（独立代理，10/10 PASS）

方法：拦截+整页 goto 每分区独立挂载（无 keep-alive 污染）；代理内 Playwright MCP 不可用 → 仓库 playwright-core 1.63.0 + 缓存 chromium headless 直跑（与 R480 锚定法等价）。

| 分区（行） | 预期 | 实测 | 判定 |
|------------|------|------|------|
| storage (R034) | 静默+空态+添加按钮 | 无 R480 文本、重试 0、alertNode 0；「添加存储实例」存在 | PASS |
| vectorstore (R040) | 静默 | 完全静默；分区标题存在 | PASS |
| websearch (R042) | 静默 | 完全静默；添加按钮存在 | PASS |
| weknoracloud (R041) | 静默 | 完全静默；「模型」行存在 | PASS |
| ollama (R028) | 友好警告+不可用+重新检测、无原文 | 全符合（hits=2） | PASS |
| retrieval 深链 (R030) | 默认表单无横幅 | 「向量检索数量 (Top K)」=50、Rerank 选择存在 | PASS |
| retrieval 抽屉 (R030) | 默认表单无裸文本 | ⌘K→齿轮→抽屉默认表单，无 R480 字样 | PASS |
| parser (R029) | 横幅原文+重试、内容替换 | 横幅+「重试」；MinerU absent | PASS |
| system (R036) | 同上 | 横幅+「重试」；版本行 absent | PASS |
| userprofile (R039) | 同上、不登出 | 横幅+「重试」；分区内容 absent；不跳转 | PASS |

## 行升级（矩阵）

- R028/R030/R034/R040/R041/R042：C → **A**（缺口修复且浏览器级验证对齐）。
- R029/R036/R039：A 维持，重试维度缺口关闭（Vue v-else-if 内嵌重试已复刻）。
- 覆盖度地图：A56→**A62** / B17 / C16→**C10** / D0。

## 新发现与延后池

1. **auth/me 整页 500 → /login**：React router `ensureSessionHydrated`（router.tsx protectBeforeLoad）boot 守卫行为；SPA 分区切换不会跳转（R480/R481 均锚定）。Vue 整页刷新下同条件行为未锚定 → **新增裁决项**（若 Vue 同跳则 parity 无需改；若 Vue 不跳需修 React boot 守卫）。
2. 子代理内 Playwright MCP/BrowserControl 均不可用（"Browser is not available in subagent"）→ playwright-core headless 直跑为已验证替代路径。
3. R039 复杂密码策略失败变体取证仍延后（复杂度开关状态需登录会话读取；React 侧 `packages/domain/src/auth/password-policy.ts` validatePassword 已在位）。
4. N015（role-denied/未迁移占位裁决）、N021（tool fixture）仍排队。
5. 首测遇「欢迎使用 WeKnora」引导遮罩拦截指针事件——测试环境噪声，跳过后无碍。

## 环境注记

- 未修改 Vue（frontend/）、mobile、Go 任何文件。
- 测试/门禁均 node v26.4.0（v22 createPortal 伪红纪律）。
