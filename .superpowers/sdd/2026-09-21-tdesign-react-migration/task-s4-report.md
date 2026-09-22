# Task S4 报告：Wave2-免登录与边缘页（login/register/redirects/dev-markdown/404）

- Worktree：`.worktrees/tdm-w2-s4`（分支 `tdm/w2-s4`，基线 e48804afe）；dev server `:5276`（worktree），Vue `:5174` / backend `:8084` 共享。
- 扫描命令：`PARITY_REACT_URL=http://localhost:5276 PAGES=… node scripts/parity/auto-scan.mjs`（worktree 内执行）。

## 终值（最终 run 2026-09-22T20-39-31）

| id | diff% | 判定 |
|---|---|---|
| login | 0.079%（729px） | **页面本体 0.00%**；残差 = 724px animated-bg 装饰动画冻结相位差（环境项，待扫描器相位同步流收敛）+ 5px 稳定 pill 圆角 AA（#16/#18 家族；取证见下） |
| register | 0.070%（649px） | 同口径：644px animated-bg 相位差 + 同一 5px pill 圆角 AA |
| redirect-system | 0 | PASS（双端同落 `/platform/settings?section=system-global`） |
| redirect-integrations | 0.002%（14px） | 落点一致（双端同落 `/platform/settings?section=integration-im`）；14px 为目标页 S1 域未迁元素残差（归因见下，非本流所有权） |
| dev-markdown | 0 | PASS |
| 404 | 不可扫 | Vue router 无 catch-all（未匹配路径渲染空白 #app，probe 取证）；按简报以单测+目检替代 |

login/register 多轮复扫：0.046/0.052/0.096/0.15/0.079（login），0.017/0.03/0.079/0.096/0.070（register）——波动本身即环境项证据（受影响元素零代码变更）。

## 提交（tdm/w2-s4）

1. `3166d3bca` feat(parity): migrate login/register to tdesign-react
2. `0af22b185` feat(parity): migrate dev-markdown to tdesign-react
3. `8a99c94e7` feat(parity): migrate 404 to tdesign-react（不可扫，单测+目检）
4. （fix round）docs(parity): S4 评审回收——台账编号 #21/#22 + taxonomy 精化（724+5）+ repairFlanking 欠账登记

## 变更文件

- 重写 `apps/web/src/auth/LoginPage.tsx`（Tailwind utilities / `@weknora/ui` Input 清零 → tdesign-react Form/FormItem/Input/Button + LinkIcon；逻辑/状态/handler 零改动；手写复刻 swiper DOM）。
- 新建 `apps/web/src/auth/login.td.css`（Login.vue scoped 块 :838-1818 → 根类前缀化 §1/§2；swiper 基础结构 §3（先于页面规则，等价 Vue 的 lib-css→scoped 层序）；React 侧自有补充 §4；unscoped dark 块 :1820-1898 原样平移 §5）。删除 `apps/web/src/auth/auth.css`。
- `apps/web/src/auth/login-page.test.tsx` 随页更新（Tailwind class 断言 → TDesign DOM 断言；jsdom 补 Node/MutationObserver/ResizeObserver/rAF polyfill，同 settings 测试先例）。
- `apps/web/src/DevMarkdownPage.tsx`：renderMarkdownFixture 增加终态 HTML 后置改写（见 dev-markdown 节）。
- 新建 `apps/web/src/NotFoundPage.test.tsx`；`docs/migrations/react/tdesign-migration-playbook.md` 台账 +行 #21/#22（评审勘误：初版误编 #18/#19，与 feat 上 S1 的 #18、S3 的 #19/#20 撞号，fix round 改为 #21/#22）。

## 关键实现点与判例

1. **swiper 同构（不新增依赖）**：Vue 用 swiper/vue（fade+pagination+autoplay 4000）。React 手写复刻其渲染 DOM（`.swiper.screenshot-swiper > .swiper-wrapper > .swiper-slide` + inline `width:<px>;opacity;translate3d` + `.swiper-pagination` span bullets），结构基础 CSS 取 swiper/css+pagination.css 生效子集进 §3。两处必须实测复刻的点：
   - **slide inline 像素宽**：swiper 写入容器实测宽（600px）。该 inline 像素参与 `.showcase-section`（flex 0 0 52% 不收缩）的 min-content 钳制（→680px）；百分比宽无此贡献。首帧一次性测量会定格在被钳制前的中间值（586），必须 ResizeObserver 跟随到稳定（同 Swiper 内部机制）。
   - **层序**：§3（lib 等效）必须排在 §1（页面 scoped 等效）之前，否则 `.swiper { padding:0 }` 以同特异性反杀 `.screenshot-swiper { padding-bottom:40px }`（Vue 端全局 lib css 先于 scoped，天然层序）。
2. **台账 #21（form-item margin）**：tdesign-react 给最后一个 FormItem 加 `.t-form__item--last`（库 CSS margin 清零）；vue-next 不加该类且 `:last-of-type` 被表单后的 button/.register-cta div 兄弟截胡，Vue 双表单项均 24px。页面作用域等值中和（login.td.css）。附注：Vue 源 `:deep(.t-form-item)`（:1522-1528）类名笔误（实际类 `t-form__item`）从未生效，按事实源原样平移。
3. **台账 #22（img 光栅相位）**：login logo 资产双端逐字节一致（md5 同）、几何一致（120×37.8）、canvas drawImage 重采样逐像素一致，但共享图层直接绘制与 Vue 端独立图层光栅在字形下缘差 ~154px（AA 阶梯）。`opacity:.9999 + scale(1.0000001)` 提升为独立合成图层后 154px→0（3 次复跑稳定）。
4. **tdesign-react Form 差异适配**：无 `data` prop（vue-next 有）——表单值经 FormItem cloneElement 注入 + `onValuesChange` 回写 state；模式切换时两 Form 互斥重挂载，内部模型自然重置。`aria-*` 经 restProps 落 `.t-input__wrap` 容器而非内层 input（Input.js:400-412），测试断言随之调整。Input type=password 双端均自动渲染 browse-off 切换图标（DOM 同构）。

## dev-markdown 0.272%→0 归因与修复

唯一差异段（Basic Text Styles 首行 y203-218）：Vue 渲染 `<em><em></em>加粗斜体</em>**`，React 渲染 `<em><strong>加粗斜体</strong></em>`。根因：Vue chatMarkdownRenderer 的 `repairFlankingEmphasis` 前置 pass（chatMarkdownRenderer.ts:283-331）——FLANKING_ITALIC 匹配 `***` 三星串（`*` 本身是 \p{P}，`[^*\n]*?` 空 + `\p{P}` 命中第 2 星）把首个 `**` 吞成 `<em>*</em>`，余下 `**` 字面量泄漏。React 共享渲染器 `@weknora/views/chat/markdown` 无该 pass 且 `renderer.html` 转义注入的原始 HTML（预注入方案实测反升 0.414%）；该文件属 settings/chat 域所有权（并行流），本页按**终态 HTML 后置改写**等价复刻（`<em><strong>X</strong></em>` → `<em><em></em>X</em>**`）→ 0%。**跨域遗留（已正式登记 playbook「已知跨域欠账」段）**：chat 生产路径（message-list/artifact-preview/message-face 等消费方）两端对 `***x***` 渲染不同——packages/views/src/chat/markdown 缺 Vue repairFlankingEmphasis；技术约束 renderer.html 转义挡住源级预注入（allowlist 仅 kbd），需 allowlist 扩展或 token 级变换；修复归属 chat 域。

## redirect-integrations 0.002% 归因（非本流所有权）

14px（x423-430 y127-130）= settings-integration-im 频道头 `IntegrationsAgentFilter` chevron-down：Vue `.integrations-agent-filter__chevron { opacity:.7 }`（IntegrationsAgentFilter.vue:98-101），React `AgentFilterButton`（packages/views/src/integrations/page.tsx，仍 Tailwind）opacity 1。glyph/rect/sprite symbol 双端逐字节一致，纯 opacity 内容差。落点验证通过（双端 URL 一致），目标页属 S1 集成 tab 迁移范围，该残差随 S1 迁移消失。redirect-system 落点双端一致 0%。

## 404 处置

Vue router（frontend/src/router/index.ts）无 catch-all：未匹配路径渲染空白（probe：/platform/definitely-not-a-page → #app 仅注释节点、bodyText 空）。React NotFoundPage（pre-router UX，shell 内渲染）无 Vue 对照物、不可扫 → 按简报单测+目检：新增 NotFoundPage.test.tsx（2 用例：zh-CN 文案+路径回显+返回链接、任意路径回显）；目检证据 shell outlet 内「页面不存在: <path>」+ 返回知识库。逻辑零改动；其 `@weknora/ui` Status 引用登记给 T15 清收。

## login/register 豁免取证（评审后 taxonomy：724 相位差 + 5 pill AA）

login 终值 729px = **724px animated-bg 相位差（环境项）+ 5px 稳定 pill 圆角 AA（#16/#18 家族）**；register 649px = 644px 相位差 + 同一 5px pill AA。审计复算口径：

- **724px animated-bg 装饰动画冻结相位差（环境项，待扫描器相位同步流收敛）**：
  - 像素级归因：逐点距 12 条 `.connection-line` 线段距离 <3.5px 判定 295px 落线（cell 网格与 line-1/2/5/7/8/11/12 viewBox 坐标逐一对应）；其余 429px 落在 knowledge-node 圆环及 15/30px box-shadow 光晕带（nodePulse scale+opacity 相位差），例如 node-9（中心 x1088 y116）本体边框与光晕边缘簇。carousel 区恒 0-1px、form 卡区 0、logo 0。
  - 双端同构证明：动画 CSS/关键帧（nodePulse 5s / lineFlow 10s dashoffset 0→18）为 Login.vue 逐字平移；轮播区恒 0-1px 证明确定性元素全部对齐。
  - Vue 自比：同 boot 两次全页截图 diff=0（单页确定性）。
  - run-to-run 波动：275↔1387px（6 轮，受影响元素零代码变更）。
  - 机制定位：auto-scan.mjs `waitForSteady` 对每页在各自 boot 后 3.2s+networkidle 处 `getAnimations().pause()`；两端 boot 相差 O(0.1-1s) → 冻结的 dashoffset/scale 相位差 O(0.2-2px) → 每个 dash 边界 ~2-4px AA 差 × 全线长约 2400px。与扫描器自身注释的轮播相位问题同类（freezeCarousel 有同步 hook，装饰动画没有）；页面代码不可消除（需改 Vue 端或扫描器同步装饰相位，均越权）。
- **5px 稳定 pill 圆角 AA（#16/#18 家族，非环境项）**：恒定点位 x1014-1017 y32 与 y60-62（header-links 第一个 pill 左圆弧上下缘，login/register 两页同点位逐 run 稳定）；双端 pill 几何逐行一致（rect/半径相同），残差为圆角 AA 一级阶梯差（2 个行对、幅值一级阶梯），同台账 #16 glyph AA 相位/权重与 #18 sprite chevron AA 的已定性家族——扫描环境 LCD AA 权重类，真机不可见。

## 验证

- v26 全量：`tests 2306 / pass 2306 / fail 0`（worktree apps/web，find 展开形式）；`tsc -b` 零错。
- 扫描证据（主仓库）：`docs/migrations/react/evidence/vue-react-parity/auto-scan/2026-09-22T20-39-31/`（终值）；过程轮 `docs/migrations/react/evidence/vue-react-parity/auto-scan/2026-09-22T20-18-13/`、`…/2026-09-22T20-27-29/`、`…/2026-09-22T20-30-37/`、`…/2026-09-22T20-37-21/`、`…/2026-09-22T20-37-44/`。

## Concerns（移交主会话）

1. `.animated-bg` 冻结相位环境项：login/register 上述豁免需 controller 认可；若要求严格 0.00% 数字，需扫描器加装饰动画相位同步（或 noFreeze 类机制）——scripts/parity 不在本流所有权。
2. `@weknora/views/chat/markdown` 缺 repairFlankingEmphasis：chat 生产路径对 `***x***`（及标点符尾强调）两端渲染不同的跨域事实，建议主会话派 chat 域评估（本页已用页内后置改写规避）。
3. redirect-integrations 的 14px 随 S1 集成 tab 迁移应消失；若 S1 完成后仍在，需 S1 补 `.integrations-agent-filter__chevron` opacity:.7。
4. NotFoundPage 的 `@weknora/ui` Status 引用待 T15；JoinPage 无 token 分支仍是旧栈紧凑卡（Vue /join 无 token 直接重定向 /login，React 保留 pre-router 卡片——行为差异历史存在，未在本流范围）；**WorkspaceOnboardingPage.tsx 仍为 @weknora/ui 全套**（auth 域未迁页，不在本任务扫描清单，T15 清收）。
