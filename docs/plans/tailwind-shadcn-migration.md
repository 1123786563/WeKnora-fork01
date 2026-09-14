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

## 页面批次（每批：转换 → 门禁 → 截图对比）

1. styles.css 共享层 + App.tsx（KB 列表）+ NotFoundPage
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
