# 2026-09-13 settings drawer visual polish（Vue↔React 抽屉视觉对齐切片）

Round-3 heatmap（2026-09-13-round3-live-sweep-theme-fix.md）登记的三项抽屉内可行动差异：
(a) 面板标题纵向节奏 ~10px；(b) 字体大小分段控件选中态；(c) 原生 select → TDesign 外观。
本切片以「Vue 计算样式值 → React 实现 → live 截图验证」闭环完成，三项全部修复。

## 证据来源（Vue 值的出处）
- frontend/src/views/settings/Settings.vue（.content-wrapper 40px 48px、.section 无 margin、.nav-item.active）
- frontend/src/views/settings/GeneralSettings.vue（.section-header 32px、.setting-row 20px 0、.font-preview）
- frontend/src/assets/theme/theme.css:1-19（light 品牌色 --td-brand-color=#07c05f=brand-color-4、active=#06b04d、focus=brand 20% mix）、theme.css:120-141（t-radio-button outline 选中态全局覆盖：实心品牌底 + 白字）
- frontend/node_modules/tdesign-vue-next/dist/tdesign.css（.t-input / .t-input--focused / t-radio-group size-m 计算 token：height 32px、radius 3px、border #dcdcdc、padding 0 8px、focus ring 0 0 0 2px brand-20%）

## (a) 标题纵向节奏 —— 已修复
| 值 | Vue | React 前 | React 后（settings-wrapper.css） |
|---|---|---|---|
| 内容区 padding | .content-wrapper 40px 48px | .wks-content-wrapper 40px 48px（已相等） | 不变 |
| section 顶边距 | .section 无（Settings.vue:892） | 旧 inline 页规则 .wk-settings-section{margin-top:1rem}（styles.css:70）泄漏进抽屉，标题整体下压 ~16px | .wks-content-wrapper .wk-settings-section{margin-top:0}（仅抽屉内生效，不影响旧页面） |
| section-header margin-bottom | 32px（GeneralSettings.vue:283） | 20px | 32px |
| h2 | 20px/600、margin 0 0 8px | 相等 | 不变 |
| .section-description line-height | 1.5 | 1.6 | 1.5 |
| .setting-row padding | 20px 0（GeneralSettings/SystemInfo/TenantInfo/UserProfile/ChatHistory/MemoryWorkspace 六个 Vue 面板逐一核对均为 20px 0） | 16px 0 | 20px 0（共享规则，React 侧各 ported 面板同享，与各自 Vue 原值一致） |

截图验证：标题 y≈114 vs Vue ≈113；语言/主题/界面字体/代码字体/字体大小各行位置与 Vue 对齐在 ~4px 内。

## (b) 字体大小分段控件选中态 —— 已修复
Vue 实际渲染链：t-radio-group(outline) + theme.css:129-141 全局覆盖 → 选中项 = background var(--td-brand-color) **#07c05f 实心** + color var(--td-text-color-anti) #fff；选中 hover = --td-brand-color-active #06b04d；未选中 border --td-component-stroke #e7e7e7、hover 文字/边框转品牌绿。按钮盒：size-m height calc(32px−2×2px)=**28px**、padding 0 16px（--td-comp-paddingLR-l）、组圆角 3px（--td-radius-default）。

React 前：rgba(7,192,95,.14) 浅绿底 + #0a8f4c 绿字 + font-weight 600，按钮 ~34px、组圆角 6px。
React 后（.wk-segmented）：border #e7e7e7、radius 3px、button 28px / padding 0 16px、hover 未选中项文字转 #07c05f、.is-active = **#07c05f 实心底 + #fff 字**（hover #06b04d）。
交互语义测试（新增 GeneralPreferencesPanel.test.tsx）：aria-checked 随点击迁移、is-active 类迁移、writeLocalPreferences 持久化 fontSize、--wk-font-scale=1.125 即时生效、4 个 select 均带 aria-label。

## (c) 原生 select → TDesign 外观 —— 已修复（DOM 未动）
TDesign .t-input（medium）计算值：height **32px**、border 1px **#dcdcdc**、radius **3px**、bg #fff、font 14px/22px、文本左 padding 8px、右侧 16px chevron（颜色 text-placeholder rgba(0,0,0,.4)）；hover 边框转品牌绿；focus 边框品牌绿 + box-shadow 0 0 0 2px brand-20%（light 主题 ≈ rgba(7,192,95,.2)）。

React 后（.general-settings .setting-control select，特异性压过 styles.css:76 的旧 select 规则）：appearance:none + data-URI chevron（right 8px / 16px）、height 32px、border #dcdcdc、radius 3px、padding 0 26px 0 8px、hover/focus 转绿、focus ring rgba(7,192,95,.2)。原生 `<select>` 与 aria-label 全部保留（语言/主题/无衬线/等宽 4 个）。
顺带（同文件、仅本面板使用）：.font-preview 底色 rgba(127,142,166,.08)→#fff、边框→#e7e7e7（Vue --td-bg-color-container / --td-component-stroke）。

## live 截图验证
- 脚本：.parity-tools/settings-visual-polish-shot.cjs（复用 locale-sweep 登录流，parity-test@local.dev，1440×900 zh-CN，Vue :5180 / React :5181）
- 产物：screenshots/accept-20260913-settings-visual-polish/{vue,react}-settings-general.png
- 像素差（阈值 |ΔRGB|>48）：整幅 39.57%（与 round3 的 settings 簇 37.9-41.6% 同带，主导项仍是遮罩模糊放大的底层页面噪声）；**抽屉内 7.07%；右侧内容区 6.14%**（残差主要是字体栅格化/抗锯齿）。

## 测试与类型
- cd apps/web && npx tsx --test src/settings/GeneralPreferencesPanel.test.tsx → **2/2 pass**（新增）
- cd apps/web && npx tsx --test src/chat/chat-page.test.ts → **16/16 pass**（不受影响）
- pnpm run typecheck:web → 仅 1 个错误：apps/web/src/platform/shell-sessions-header.test.tsx:159 TS2353（**他 agent 在途的未跟踪新文件**，非本切片引入；本切片文件 0 诊断）。待该 agent 修复后应复跑。

## 遗留 / 提请协调者决策
1. **侧栏导航选中态**（截图可见，未改——超出本次三项授权）：React .wks-nav-item.is-active = #eff4ff/#2e6de6（蓝），Vue = --td-bg-color-secondarycontainer **#f3f3f3** + --td-brand-color **#07c05f**（绿，Settings.vue:768-772）。值已备好，.wks-* 块在本切片可写范围内，经确认后一行即可改。
2. 侧栏 chrome 微差：.wks-sidebar #f7f9fc/#e7ebf2 vs Vue #f9f9f9(--td-bg-color-settings-modal)/#e7e7e7(--td-component-stroke)；.wks-nav-item:hover #eef1f6 vs #f3f3f3；.wks-nav-group-title #8a94a6 vs rgba(0,0,0,.4)。
3. TDesign select 外观目前只作用于 .general-settings；其余抽屉面板（resource/config/ollama/cloud/envvar）的 select 仍是旧输入框样式，待各自切片稳定后在 .wk-settings-section 层统一铺开。
4. Vue 分段控件是真实 t-radio-button；React 保持 button+role=radio（语义等价，按简报未改 DOM 结构）。

## 本切片改动文件
- apps/web/src/settings/settings-wrapper.css（a/b/c 全部样式修复）
- apps/web/src/settings/GeneralPreferencesPanel.test.tsx（新增，(b) 交互语义）
- .parity-tools/settings-visual-polish-shot.cjs（新增，取证脚本）
- docs/migrations/react/evidence/vue-react-parity/2026-09-13-settings-visual-polish.md（本文）
- docs/migrations/react/evidence/vue-react-parity/screenshots/accept-20260913-settings-visual-polish/*.png（新截图）
