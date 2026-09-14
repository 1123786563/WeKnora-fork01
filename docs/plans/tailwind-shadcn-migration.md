# Tailwind CSS + shadcn/ui 迁移实施计划（react-multiclient worktree）

对象：本 worktree（codex/react-multiclient 分支）的 React Web 界面（apps/web、packages/ui、packages/views）。
不变项：React 19 + TS + Vite 7、页面布局、品牌色、字体层级、信息架构、路由、i18n、
API 契约、鉴权、权限、表单校验、聊天流式、工具审批、上传、错误处理；Vue parity 规范优先。
不涉及：apps/mobile（RN，不引用 @weknora/ui）、apps/desktop（独立 renderer，无 ui/views 依赖）、Go 后端、frontend/ Vue。
注意：apps/embed 依赖 @weknora/views，views 层改动需补跑 embed 门禁（typecheck/test/build）。

## 基线（改造前，HEAD 3076630a）

- 门禁全绿：test:shared 445、test:web 856、typecheck:web 0、build:web 通过（chunk 体积警告为既有）。
- 本地真实栈可用：Go 后端 :8080（worktree 启动）、vite :5181（VITE_API_BASE_URL=localhost:8080）。
- 视觉基线：artifacts/tailwind-shadcn/baseline/*.png（21 张：18 路由 @1440px、kb-list @390px、login/register/not-found）。
- 截图账号：uimig@local.dev（公共注册接口创建；kb-list 含新用户引导弹层，前后状态一致）。
- 既有问题（非迁移引入，记录不掩盖）：Tailwind 入口仅 utilities.css 且无 @source，
  packages/ui 内 utilities（Input/NumberInput 的 h-8、border-[#dcdcdc] 等）从未生成——
  这些组件的 Tailwind 类当前实际不生效，仅 .wk-* 兜底 CSS 生效（switch.tsx 注释即此问题）。
  基础设施修复后这些既有声明开始生效，属"让已写样式真正生效"，需截图复核无视觉漂移。

## 基础设施

1. styles.css 入口：引入 tailwindcss/theme.css（默认令牌）+ utilities.css，
   并以 @source 指向 packages/ui/src、packages/views/src（修复扫描盲区）。
   Preflight 决策：暂不引入。理由：约 1.23 万行既有 CSS 与大量依赖浏览器默认样式的区域
   （标题间距、列表标记等）在迁移期会被 reset 破坏 parity；待领域 CSS 全部转为 utilities
   后再评估启用（同视口截图对比验收）。保留 theme-mode 属性 + color-scheme 主题机制。
2. 语义令牌：packages/ui/src/theme.css（@theme），由 apps/web 引入。品牌色值 1:1 映射：
   ink #172033、muted #66758b、surface #ffffff、canvas #f7f9fc、line #dce3ed、primary #2e6de6、
   danger #b42318、control accent #07c05f（Input/Switch/NumberInput 现用绿色，Vue parity）、
   warning #9a6700、hairline #edf0f5、field-line #b8c5d6；radius control 6px / card 8px。
3. 生产构建扫描验收点：build 后抽查 dist css 含包内 utilities（如 h-8）。

## 组件体系（packages/ui，shadcn 规范：cva variants + cn）

保留公开 API 兼容优先，逐步 shadcn 化：
- Button：cva variants（default/outline/ghost/text/destructive + 尺寸 + loading 保留），
  吸收 wk-button--text（22 处）等用法。
- Input/NumberInput/Switch：类名改语义令牌 utilities（视觉不变）。
- Card/Status 保持；新增 Badge、Alert、Textarea、Select（原生 select 样式化，保表单语义）、
  Checkbox（原生样式化）、Label、Separator、Table、Tabs（受控无头）。
- Dialog：保留 DOM 契约（focus/Escape/restore），内部迁移到 @radix-ui/react-dialog
  （portal/focus-trap/aria），样式 tokens 化；Sheet（侧滑抽屉）基于同库新增。
- 不安装 sonner/toast：现有通知面为内联 Status/Alert/popconfirm（按需原则）。

## 进度账本

运维记录：首批并行子任务因始终未执行文件写入被中断（只读分析循环）；
前台探针证实绝对路径写入可用。已带侦查结论 + 「边读边写 + 后台门禁」纪律重新派发。

### 续作指引（上下文交接，2026-09-14）
- 已完成批次：auth、sandbox、memory-workspace、knowledge-list、command-palette（5/域）+ 基础设施/组件体系。
- 子任务经验：小域（css<200 行）可成功；大域子任务会在长读阶段耗尽自身上下文而失败
  （skill-settings 568/TenantMembers 751 两个 agent 均失败于读取阶段，无写入）。
  → 大域改由 Orchestrator 分片自做（staged 引擎，auth/KB 已验证）或拆更小子任务。
- 剩余域与规模：shell.css 581（PlatformShell.tsx）｜faq 1420（FAQPage）｜
  organizations 1235（OrganizationsPage）｜documents 1288（KnowledgeDocumentsPage）｜
  agents 759+367（AgentsPage/AgentEditorModal）｜settings-wrapper 280（多面板）｜
  TenantMembers 751｜personal-memory 604｜skill 568｜chat 1668（ChatRoutePage）｜
  guides 243（views，影响 embed，需补 embed 门禁）。
- 共享收尾（最后做）：styles.css 共享类规则删除（wk-page/wk-header/wk-muted/wk-list/wk-form/
  wk-toolbar/wk-tag/wk-switch/wk-popconfirm/wk-settings-* 等）需逐类确认最后一个使用方已迁移；
  packages/ui/src/styles.css 的 wk-* 规则随组件迁移已部分失效，最终清理；依赖审计。
- 验证资产：tailwind-shadcn-screens.mjs / tailwind-shadcn-diff.mjs / section-shot 脚本；
  基线+after-{foundation,auth,kblist} 截图；本地栈（:8080 sqlite / vite :5181，账号 uimig@local.dev）。
- 本地栈注意：先前 dev 栈进程已结束；vite/后端均可用后台 job 方式自启（后端二进制 /tmp/uimig-server，
  sqlite /tmp/uimig-weknora.db，含 uimig 账号与一条 KB）。
### 批次5：command-palette 域 ✅（Orchestrator 执行）
- GlobalCommandPalette.tsx 23 处 className 全部 utilities 化（overlay/面板/输入行/结果组/
  分组头/条目+选中态/快捷键徽章 kbd 变体/空态/footer）；cmdk 类名保留为测试 DOM 钩子
  （cmdk/cmdk__input/cmdk__item/cmdk__item-shortcut，无样式规则）。
- command-palette.css 删除（181 行，全部可 1:1 表达，无 keyframes）。
- 验收：typecheck 0、web 856/856（含 shortcuts/render 38 项）、build ✓。
### 批次4：knowledge-list 域 ✅（Orchestrator 执行，分阶段原子替换）
- App.tsx KnowledgeBasesPage 全部 kb-list-* 样式 → utilities（3 阶段：结构层 21 处、
  网格/骨架/空态/section 头 19 处、卡片层 20 处）；类名保留为 DOM/测试解剖钩子（无样式规则）。
- knowledge-list.css 823 行 → keyframes-only（kbListFadeIn/kbListFlash/kbListShimmer）。
- 验收：typecheck 0、web 856/856、shared 445/445、build ✓、anatomy 16/16、
  kb-list 截图复核（rail/标题/warning/卡片全正常；像素差 1.5% 为转换预期差异）。
- 事故记录：与并行子任务并发写 App.tsx 导致截断一次，git checkout 恢复后重放三段转换；
  该教训已固化：同一文件禁止双写，子任务仅用于无冲突域。
### 批次3：memory-workspace 域 ✅（Orchestrator 自做）
- PersonalMemoryPanel.tsx 全部样式 → utilities（header/intro/group/rows/stacked row/textarea、
  720px 媒体查询 → max-[720px]: 前缀、focus color-mix → outline rgba）；
  memory-workspace.css 删除（153 行，全部可 1:1 表达，无 keyframes）。
- wk-segmented 保留（settings-wrapper.css 所有，后续批次处理）。
- 验收：typecheck 0、web 856/856、build ✓、settings-memory 截图复核（intro 盒/行布局/开关正常）。
### 批次4（进行中）：knowledge-list 域 —— Orchestrator 亲自执行
- 侦查完备：css 823 行 60+ 类全读；App.tsx 用点在 L652-895（67 处）；
  kb-list-anatomy.test.tsx 约 20 处 .kb-list-* 选择器待改。
- 分片计划：①结构层（page frame/container/header/rail/warning/grid/skeleton/empty）
  ②卡片层（card/variants/favorite/more-menu/badges/org-chip/flash）③测试选择器+css 缩减为
  3 条 keyframes（kbListFadeIn/kbListFlash/kbListShimmer）。注意 hover 链（card:hover .more、
  .kb-favorite-star）与 ::after 角标渐变用 group/ arbitrary variant 表达。
- 其余并行子任务（shell/faq/organizations）继续后台执行，完成即验收。
### 批次2：sandbox 域 ✅（子任务执行，Orchestrator 验收）
- SandboxSettingsPanel.tsx 7 处 className 迁移；sandbox-settings.css 106→44 行。
- 删除 16 条规则全转 utilities；保留 6 条（details/summary 子选择器 4 条、
  @keyframes+prefers-reduced-motion 一对）原因已写入文件头注释。
  var(--wks-*) 11 处按规则转任意值保留；DOM 钩子类保留（inventory overlay/drawer）。
- 验收：typecheck 0、面板测试 28/28、build ✓、settings-sandbox 截图人工复核
  （开关绿色 on 态、policy 行、hint、tabs、空态全部正常）。
### 批次1：auth 域 ✅（Orchestrator 自做，子任务环境无法写文件已回退）
- LoginPage.tsx 全部样式 → utilities；.language-switch 保留为 closest() DOM 钩子；
  auth.css 缩减为 2 条 @keyframes（nodePulse/lineFlow，复杂动画按约定保留）；
  login-page.test.tsx 2 处选择器改 type=submit / data-testid（toast 增 data-testid）。
- 验证：typecheck 0、web 856/856、build ✓、login 截图人工复核（渐变/动画/表单/语言菜单正常）；
  login/register/not-found 的像素差异为轮播 4s 自动播放时序非确定（后续批次对比时将冻结时钟）。

- [x] 基础设施：theme.css 语义令牌（packages/ui/src/theme.css）+ styles.css 入口
  （theme/utilities layer + @source 扫描 ui/views）+ ui 包导出 theme.css。
  构建产物已验证：包内 utilities（如 .h-8）开始生成（修复了基线记录的既有缺陷）。
- [x] 基础组件一批：button.tsx（cva：default/primary/text/danger + loading 兼容）、
  Card/Status/Input/NumberInput/Switch 令牌化（视觉=各自既有生效样式）。
- [x] 门禁：shared 445、web 856、typecheck 0、build ✓（web 1 处失败为 70e9f061 i18n
  sweep 与 upload-dialog 旧断言的漂移，已随 77202404 修复；同提交修复了该提交截断的
  KnowledgeSettingsPage.tsx 尾部）。
- [x] 截图对比一批（baseline vs after-foundation，21 张）：差异集中在 Button 实际生效、
  i18n sweep 文案与 kb-settings 修复；无布局/配色漂移。artifacts/tailwind-shadcn/。
- 共享树注意：该 worktree 有并行任务在改 i18n/KB-settings（70e9f061、02a938ee）；
  本迁移坚持路径隔离，不动 i18n 与他人文件；截图账号 uimig@local.dev。

## 页面批次（每批：转换 → 门禁 → 截图对比）

1. styles.css 共享层 + App.tsx（KB 列表）+ NotFoundPage  ← 共享类规则在最后一个使用方迁移后删除
2. auth.css（login/register/join/onboarding）
3. platform/shell.css + command-palette.css（外壳导航/命令面板）
4. knowledge-list.css
5. settings/ 6 个 css + views/settings 面板
6. documents/documents.css
7. agents/agents.css + agent-editor.css
8. organizations/organizations.css
9. faq/faq.css
10. chat/chat.css（最长；富文本/markdown 内容区如需保留必要 CSS，按 III.7 说明保留）
11. views/guides/guides.css（补跑 embed 门禁）
12. 清理：删除已迁移 css；wk-* 从 TSX/tests 移除（316 处测试选择器随批更新）；
    移除不再使用的依赖；抽查 dist。

## 验收标准

- 每批后：test:shared ≥445、test:web ≥856（选择器更新计数如实记录）、typecheck:web 0、build 通过。
- 关键路由截图对比：同视口/同语言/同数据（uimig@local.dev，1440px + 390px），归档
  artifacts/tailwind-shadcn/after-<batch>/。
- 最终：embed + desktop 门禁补跑；views 改动导致 embed 截图抽查。
- 环境受限项如实标注 blocked-env（当前无；本地栈可用）。
