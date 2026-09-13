# 2026-09-13 — Settings chrome rollout(select 铺开 + 抽屉 rail hover 对齐)

上一切片 2026-09-13-settings-visual-polish.md 在 .general-settings 作用域完成了 select 的 tdesign 化外观。本切片把同一外观铺开到设置抽屉其余面板,并将抽屉 rail hover 色阶对齐 Vue Settings.vue 基准。实施 agent:settings-chrome-rollout(分支 codex/react-multiclient,未 commit)。

## 铺开的作用域清单(摸底结果)

select 出现位置 → 铺开后 chrome 来源:

| 面板 | select 位置 | 铺开途径 |
| --- | --- | --- |
| General | GeneralPreferencesPanel.tsx ×4 | 通用规则 + .general-settings 保留 280px 宽 |
| EnvVar | EnvVarSettingsPanel.tsx:69(scope) | 通用规则(integration tab 在 .wks-content-wrapper 下) |
| Config | ConfigSettingsPanel.tsx:91(model select) | 通用规则 |
| Model / ModelDebug | ModelSettingsPanel.tsx:1184,1448;ModelDebugPanel.tsx:244 | 通用规则(.setting-control--model 宽度 280px 保留) |
| MCP | McpSettingsPanel.tsx:961,1049 | styles.css .wks-mcp-drawer 块精简为布局;chrome 由通用规则接管(该 drawer 元素同时带 .wks-modal class) |
| Sandbox | SandboxSettingsPanel.tsx:1116,1375,1397,1406,1411(向导) | 通用规则(sandbox-settings.css 无本地 select 规则,无需覆盖) |
| Skill | SkillSettingsPanel.tsx:572(installer model) | 通用规则(skill-drawer-host 无 portal,DOM 在主抽屉内;skill-settings.css 无本地 select 规则) |
| Members | TenantMembersPanel.tsx:249,623,702,730 | 通用规则胜过 TenantMembersPanel.css 本地 chrome(特异性 .wks-modal 前缀 (0,3,1) > (0,2,1)/(0,2,0),不依赖注入顺序) |
| PersonalMemory | PersonalMemoryPanel.tsx:36 | 通用规则 |

排除项:原生多选 select[multiple] 保持浏览器外观(当前抽屉内无此类元素,规则层面排除以备未来)。宽度不进通用规则,维持各上下文现状(legacy width:100%;general/model 280px)。

## 变更文件(均在本切片独占归属内)

- apps/web/src/settings/settings-wrapper.css:提升通用 select 规则(32px、#dcdcdc 边、3px 圆角、appearance:none+chevron、hover/focus 品牌绿 #07c05f、disabled #eee/rgba(0,0,0,.26)/#dcdcdc),.general-settings 缩减为宽度职责。
- apps/web/src/styles.css(.wks-* 块,仅 select/抽屉 rail 相关):
  - .wks-nav-item:hover → #f3f3f3 + rgba(0,0,0,0.9)(Vue --td-bg-color-container-hover / --td-text-color-primary)
  - .wks-nav-item.is-active → #f3f3f3 + #07c05f + 500(Vue secondarycontainer / brand)
  - .wks-sidebar 底 #f9f9f9、右 border #e7e7e7(--td-bg-color-settings-modal / --td-component-stroke);header 底边 #e7e7e7;标题 rgba(0,0,0,.9)
  - .wks-nav-group-title → rgba(0,0,0,0.4)(--td-text-color-placeholder);nav-item 基础色 rgba(0,0,0,.9)
  - .wks-mcp-drawer .wk-settings-editor select 精简为 box-sizing/width,chrome 让位通用规则

## Live 验证(双端 :5180 Vue / :5181 React,账号 parity-test@local.dev,zh-CN,GUIDE_KEYS 种子完成)

脚本:.parity-tools/settings-chrome-rollout-shot.cjs(主)、-sandbox-wizard-shot.cjs / -vue-shot / -react-shot(向导取证)。

截图(evidence/vue-react-parity/screenshots/settings-chrome-rollout/):
- vue-settings-general.png / react-settings-general.png — 已验证面板回归:双端 4 select、字体预览、字体大小分段控件一致,280px 保留
- vue-settings-sandbox.png / react-settings-sandbox.png — 列表态 + rail 激活态
- vue-settings-sandbox-wizard.png / react-settings-sandbox-wizard.png、react-settings-sandbox-wizard-select.png — 向导“沙箱类型”select 特写
- vue-settings-members.png / react-settings-members.png — 成员面板 rail 激活 + 分页/角色 select
- computed-styles.json、sandbox-wizard-computed.json — 计算样式证据

计算样式要点(computed-styles.json):
- select chrome 双端一致:React border 1px solid rgb(220,220,220)、radius 3px、height 32px、appearance none、svg-chevron;Vue t-input 同值(32px/3px/rgb(220,220,220))
- rail:双端 hover #f3f3f3 + rgba(0,0,0,0.9);active #f3f3f3 + rgb(7,192,95) + 500;sidebar rgb(249,249,249)/rgb(231,231,231)
- members role select 命中通用规则且宽度维持 100%(738.8px),general select 280px — 宽度职责未回归

## 测试(最终)

- npx tsx --test src/settings/SandboxSettingsPanel.test.tsx → **28/28 绿**(任务基线 26;并行 agent 新增 2 例,全过)
- npx tsx --test src/chat/chat-page.test.ts → **20/20 绿**(任务基线 16;并行 chat agent 新增 4 例,全过。首跑曾红于 packages/views/src/chat/composer.tsx:92 modelContext 未定义 —— 该 agent 的未提交中间态,其完成编辑后复跑即绿,与本切片 CSS 变更无关)
- pnpm run typecheck:web → **通过**(首跑仅 composer.tsx TS2304,同上归因;复跑零错误)

## 遗留 / 需协调

1. .wk-pager__size(members 分页每页条数)宽度 100% 全宽,Vue 为 auto 宽 —— width 来源 styles.css:76 legacy 非 .wks-* 块或 TenantMembersPanel.css,均不归本切片;建议协调者授权将其收窄为 auto(约 88px)。
2. React 沙箱向导步骤条(1连接/2模板/3运行配置)为纯文本,无 Vue 的圆形标记/连接线 —— SandboxSettingsPanel.tsx 归属他人。
3. members 页头出现未翻译 key "identity.tenants.members" —— i18n 资源问题,归 SettingsPage/文案侧。
4. (已解除)首跑时 chat-page.test/typecheck:web 红为并行 agent 中间态,其完成编辑后复跑全绿。
