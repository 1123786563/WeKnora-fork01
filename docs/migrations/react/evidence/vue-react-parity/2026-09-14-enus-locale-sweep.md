# 2026-09-14 — en-US / zh-CN 双语全状态核验（S00 语言维度切片）

- 分支：`codex/react-multiclient`（worktree `.worktrees/react-multiclient`）
- 执行脚本：`.parity-tools/enus-sweep.cjs`（Playwright / chromium，1440×900）
- 原始断言数据：`.parity-tools/enus-sweep-results.json`
- 截图目录：`screenshots/enus-sweep-20260914/`（16 张，见 §4）

## 1. 目的与方法

补齐 62 行 review 中缺失的 **en-US 全状态复核**：Vue（:5180）与 React（:5181）双端 × zh-CN / en-US 双语 × 4 条核心路由，逐格截屏并做语言断言。

**Locale 注入**：两端的 locale 存储键均为 `localStorage['locale']`（React：`apps/web/src/i18n.ts` 的 `LOCALE_STORAGE_KEY`；Vue：`frontend/src/i18n/index.ts:18`）。脚本用 `addInitScript` 在应用启动前写入 `zh-CN` / `en-US`，保证首帧即目标语言。同时写入 8 个 guide 关闭键（与 2026-09-13 locale-sweep 相同），排除新手引导遮罩干扰。

**登录**：`parity-test@local.dev / Parity123456!`（已先用 `POST http://localhost:8080/api/v1/auth/login` 验证可用）。选择器为结构化（`input[autocomplete=email]`、`input[type=password]`、`button.submit-button`），与语言无关。

**断言逻辑**（每格）：
1. **锚点断言**：页面可见文本含该格语言的关键 UI 文案（kb-list=知识库/Knowledge Base，agents=智能体/Agents，settings-general=常规设置/General Settings，creatChat=让你的知识触手可及/your knowledge, within reach）。
2. **en-US 反向残留扫描**：收集 UI chrome 元素（button / h1-h6 / a / label / th / [role=…] / [placeholder] / [aria-label] / [title] / Element Plus 类名节点）中含 CJK 汉字（`\u3400-\u9FFF`）的文本，逐条记录 tag+class+text。
3. **zh-CN 反向标题扫描**：h1-h4 / [role=heading] 中纯 ASCII 文案（白名单：WeKnora、MCP、API、ReRank、RAG、FAQ、ID、TopK、Embedding、Token、PDF、CSV、JSON、IM、URL、HTTP、HTTPS）。
4. **残留定性**：en-US 格中的 CJK 命中按 class 上下文分类——用户数据（聊天会话标题 / 知识库名，双端同源、两种语言下均出现）不算 i18n 缺口；其余逐条溯源到代码。

## 2. 结果总览

| 指标 | 结果 |
| --- | --- |
| 格数（app×locale×route） | 16 |
| 锚点断言通过 | **16 / 16（100%）** |
| 页面 JS 错误（pageerror） | 0 / 16 |
| zh-CN 格出现英文标题 | **0**（8/8 干净） |
| en-US 格可见中文残留（UI 文案） | **0**（8/8 干净） |
| en-US 格不可见（a11y）中文残留 | **1**（React creatChat，见 §3-F1） |

en-US 各格 CJK 命中全部为**用户数据**（侧边栏 7 条历史会话标题 + 知识库卡片「产品知识库」），双端一致、zh-CN 格同样存在，属 fixture 数据而非 i18n 缺口。

## 3. 发现清单（移交协调者定夺，未改代码）

### F1 · React en-US creatChat：上传附件按钮 aria-label/title 为中文「上传附件」

- 证据格：`react-en-US-creatChat.png`；捕获元素 `button.wk-chat-control-icon`（aria-label 与 title 均为「上传附件」）。
- 溯源：`packages/views/src/chat/chat-copy.ts` 中 `uploadAttachment: '上传附件'` 标注为 *"zh-only (no Vue locale source)"*，en/ja/ko/ru 四张表均沿用 zh 值；消费点 `packages/views/src/chat/composer.tsx:77`（`aria-label={t.uploadAttachment} title={t.uploadAttachment}`）。
- 影响：**不可见文案**（悬停 tooltip + 读屏标签）；en-US 页面可见 UI 无中文。Vue 端同一控件无此问题（Vue en-US 格该控件未命中 CJK）。
- 定性：源码已自认的既有缺口（非本次回归）。建议：为 zh-only 键补 en 值（如 "Upload attachment"），或在 chat-i18n 缺口清单中显式关闭。

### F2 · chat-copy.ts 同类潜在缺口（源码静态盘点，本次路由未触发）

chat-copy.ts 共 **11 个 zh-only 键**（en-US 下渲染中文）：`groupLabel`（分组）、`groupByDate`（按日期）、`uploadAttachment`（上传附件）、`artifactsPending`（产物生成中…）、`sending`（发送中…）、`sendFailed`（发送失败）、`terminalInput`（终端输入）、`sendInput`（发送输入）、`closeTerminal`（断开终端）、`thinkingAndTools`（思考与工具）。其中后 7 个为瞬态/沙箱终端文案，本次 4 条路由未触达，属同类潜在 en-US 缺口。

### F3 · chat-copy.ts 引用的证据文档不存在（悬空引用）

文件头与 11 处注释指向 `docs/migrations/react/evidence/vue-react-parity/2026-09-13-chat-i18n.md`，该文件**不存在**（目录中仅有 2026-09-13-five-locale-sweep.md 等）。建议协调者补建或改指向。

### F4 · 观察项（非语言缺陷）

- `document.title`：Vue 全部为 "WeKnora"，React 全部为 "WeKnora React migration"——双端不一致，建议后续切片对齐。
- zh-CN 格中英文仅出现在合法术语（RAG、ReAct、Wiki、CSV/Excel、SQL、IM、API、MCP、Chrome），无英文标题残留。

## 4. 截图清单（16 张）

目录：`docs/migrations/react/evidence/vue-react-parity/screenshots/enus-sweep-20260914/`

| 格 | 文件 | 锚点 | 可见残留 |
| --- | --- | --- | --- |
| vue zh-CN kb-list | `vue-zh-CN-kb-list.png` | ✅ 知识库 | —（本语言即中文） |
| vue zh-CN agents | `vue-zh-CN-agents.png` | ✅ 智能体 | — |
| vue zh-CN settings-general | `vue-zh-CN-settings-general.png` | ✅ 常规设置 | — |
| vue zh-CN creatChat | `vue-zh-CN-creatChat.png` | ✅ 让你的知识触手可及 | — |
| vue en-US kb-list | `vue-en-US-kb-list.png` | ✅ Knowledge Base | 无 |
| vue en-US agents | `vue-en-US-agents.png` | ✅ Agents | 无 |
| vue en-US settings-general | `vue-en-US-settings-general.png` | ✅ General Settings | 无 |
| vue en-US creatChat | `vue-en-US-creatChat.png` | ✅ your knowledge, within reach | 无 |
| react zh-CN kb-list | `react-zh-CN-kb-list.png` | ✅ 知识库 | — |
| react zh-CN agents | `react-zh-CN-agents.png` | ✅ 智能体 | — |
| react zh-CN settings-general | `react-zh-CN-settings-general.png` | ✅ 常规设置 | — |
| react zh-CN creatChat | `react-zh-CN-creatChat.png` | ✅ 让你的知识触手可及 | — |
| react en-US kb-list | `react-en-US-kb-list.png` | ✅ Knowledge Base | 无 |
| react en-US agents | `react-en-US-agents.png` | ✅ Agents | 无 |
| react en-US settings-general | `react-en-US-settings-general.png` | ✅ General Settings | 无 |
| react en-US creatChat | `react-en-US-creatChat.png` | ✅ your knowledge, within reach | 无（a11y 1 条，见 F1） |

人工抽查复核（看图确认）：react-en-US-creatChat / vue-en-US-creatChat（侧边栏英文 chrome + 用户数据中文标题）、react-en-US-settings-general（Settings 弹窗全英文，Language=English）、react-en-US-kb-list（Knowledge bases 标题/英文告警条）、react-zh-CN-agents（全中文，内置 4 卡片）。

## 5. 限制

1. 仅 4 条核心路由；settings 仅 general 段，agents/kb-list 仅列表态（未进编辑器/详情）。
2. 视口截图（1440×900），非 fullPage；折叠区文案未入镜（DOM 扫描已排除隐藏元素）。
3. 断言针对 UI chrome；正文/用户生成内容（会话标题、KB 名）明确排除，en-US 下中文用户数据为预期。
4. 瞬态文案（发送中/失败、沙箱终端）未构造触发，F2 为静态盘点而非运行时证据。
5. locale 经 localStorage 注入，未覆盖「切换语言即时生效」的运行时切换路径。
6. 测试数据为 parity-test 空间当时状态（7 条会话、3 个知识库、4 个内置智能体），后续批次数据变化不影响断言逻辑。
