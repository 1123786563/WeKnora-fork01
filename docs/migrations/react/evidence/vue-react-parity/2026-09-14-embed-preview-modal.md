# R013/N029 · embed 预览模态（EmbedPreviewModal）切片（Vue→React 一致性）

- 日期：2026-09-14（重试派发轮）
- 工作区：`.worktrees/react-multiclient`（分支 `codex/react-multiclient`，直接改动、未 commit，留待协调者复核集成）
- 本切片改动（均在独占片区 `apps/web/src/integrations/**`）：
  - 新增 `apps/web/src/integrations/EmbedPreviewModal.tsx`（73 行）
  - 新增 `apps/web/src/integrations/EmbedPreviewModal.test.tsx`（86 行，1 条交互渲染测试）
  - 修改 `apps/web/src/integrations/IntegrationsRoutePage.tsx`（+12/−2）
- 只读遵守：`frontend/**`、`packages/views/src/integrations/**`（含 page.tsx 既有 in-page 预览面板与 embedWizard.ts）均未改动；未触碰并行 FAQ/documents 切片的在途文件。

## 1. 核对结论（任务 1）

| 维度 | Vue 基准 | React 现状（本切片前） | 结论 |
|---|---|---|---|
| 预览入口 | AgentEmbedChannelPanel.vue 部署步代码面板「预览」按钮（L317-320 → openPreviewFromDrawer L984-991） | page.tsx 部署步 `预览` 按钮（L1118 → previewEmbedChannel L331-345），embedWizardRender.test.tsx 已断言点击后渲染预览面板 + iframe | ✅ 已对齐（既有实现，本切片复核） |
| previewSession 端口 | issueEmbedPreviewSession（api/embed）→ session_token | `actions.onPreviewSession`（page.tsx L90/L336）由路由页接 `client.embed.channels.previewSession`（api-client L157-159，EmbedSessionToken） | ✅ 已存在（IntegrationsRoutePage.tsx L81） |
| iframe src 构建 | buildEmbedURL（api/embed L563-577）：/embed/:id + ?locale=&r= + #token= | page.tsx 面板用相对 `${origin}/embed/:id#token=token`；本切片组件补齐 locale/refreshKey 查询参数语义 | ✅ 语义一致（组件版见 buildEmbedPreviewSrc） |
| 模态形态 | EmbedChannelPreview.vue：720px 抽屉模态（t-drawer size=720px attach=body），iframe 模式设备框 + 加载态 + 关闭 | page.tsx in-page 面板复用同一组 wk-embed-preview-* 样式（overlay + 720px aside + 设备框 chrome + iframe + Esc/遮罩/×关闭） | ✅ 已对齐 |
| 卡片级预览回退 | openPreviewForChannel 拿不到 token → MessagePlugin.warning(previewUnavailable)，**绝不另开标签页**（L1005-1010） | 路由页 openEmbed 拿到 token 后 `window.open` 新标签页（❌ 模态性偏差，且 token 为空时仍开空 token 页） | ⚠️ → 本切片修复（见 §2） |

## 2. 本切片交付（任务 2）

1. **新增 `EmbedPreviewModal.tsx`**：受控模态组件，镜像 Vue EmbedChannelPreview.vue（iframe 模式）——
   - `buildEmbedPreviewSrc`：逐语义移植 Vue buildEmbedURL（origin 显式入参；`/embed/:id` + `?locale=`/`?r=` + `#token=`，全部 encodeURIComponent）；
   - 遮罩点击 / Esc / 头部 × 三关闭面；`role="dialog"` + `aria-modal`；加载态（「正在加载预览…」，取自 embedPublish.previewLoading zh-CN 逐字值）；`allow="clipboard-write"`；
   - channelId/token 为空不渲染 iframe（对齐 Vue iframeSrc L78-84 守卫）；
   - 复用已入库的 wk-embed-preview-* 样式（styles.css，080bb8c8），零新增 CSS、未碰他人片区。
2. **挂载**：`IntegrationsRoutePage.tsx` 的 `openEmbed` 由 `window.open` 新标签页改为 in-page `EmbedPreviewModal`（先 previewSession 签发短期 token，空 token 报错不再开页）。该路径正是部署步预览面板在拿不到 token 时的回退臂（page.tsx L338 `onOpenEmbed?.(channel)`）——即部署步预览流程的路由侧挂载点。窗口打开式预览与 Vue 模态性相悖，自此消除。
3. **说明**：部署步「预览」按钮的主路径（page.tsx in-page 面板）在本切片权限外（packages/views 只读），其行为已满足本切片验收（点击预览 → 模态渲染 iframe + 关闭，既有测试锚定）；本组件即该形态的可复用提取版，page.tsx 归属方后续可采用以收敛两份渲染。

## 3. 测试证据（任务 3/4，TDD）

```
# 先红：EmbedPreviewModal.tsx 缺失 → EmbedPreviewModal.test.tsx fail（模块缺失）
# 后绿：
cd apps/web && npx tsx --test src/integrations/*.test.*
  tests 46 / pass 46 / fail 0   （基线 45 + 本切片 EmbedPreviewModal 1）
pnpm run typecheck:web
  exit 1 —— 11 个错误全部位于 src/faq/faq-import-result.test.tsx(7) 与
  src/faq/faq-tag-tooltip.test.tsx(4)，属并行 FAQ 切片在途 TDD 红文件；
  grep 反证：integrations/** 零错误（本切片 typecheck 干净）。
```

新测试断言锚点：预览按钮点击前无 iframe；点击后 `[role="dialog"]` 渲染、iframe src === `https://weknora.test/embed/ch-1?locale=zh-CN#token=tok_preview`（previewSession token 入 hash、locale 入查询串）、`allow="clipboard-write"`；点击 关闭（title）后模态卸载。

## 4. 遗留与报备

1. **部署步主路径提取（建议）**：page.tsx 的 EmbedChannelPreviewPanel（L575-602）与本组件形态/语义一致，属 packages/views（本切片只读）。建议其归属方以本组件语义收敛（views 不可反向依赖 apps/web，需把组件下沉 views 或经协调者裁决落点）。
2. **widget 模式**：本组件仅实现 iframe 模式（部署步预览按钮按 snippet tab 决定 iframe/widget；widget 模拟宿主页仍由 page.tsx 面板承担）。Vue 同构，未引入偏差。
3. **草稿外观/locale 透传**：组件已具备 locale/refreshKey 入参（Vue previewLocale/previewNonce 语义）；路由页当前仅传渠道默认（不传草稿值），与既有 page.tsx 面板行为一致，待草稿预览课题一并收口。
4. **真实后端浏览器预览**：沿用账本既有登记（embed preview-session 签发的 ems_ 令牌被 /embed/sessions 拒绝，后端待决，Round 30）；本切片未新增浏览器证据。
5. **并行改动的门禁时点记录（报备）**：本切片完成门禁时（46/46 绿）之后，检测到另一并行代理正在改 `packages/views/src/integrations/page.tsx`（EmbedChannelPreviewPanel 加 locale/refreshKey 透传，调用 `embedChannelUrl`），且其中途态在 L52 引入语法错误（`embedChannelUrl,` 落在 `} from './embedWizard.ts';` 闭括号之后，esbuild `Unexpected "export"`），导致 embedWizardRender/imWizardRender 两条渲染测试文件加载失败（37 tests / 35 pass / 2 文件级 fail）。本切片未触碰该文件；修复归其归属方。本切片独立测试集（EmbedPreviewModal + tenant + apiPlaygroundModel/SSE）26/26 绿。

## 5. 重试轮收敛（同日第二次派发，追加于本节；§1-§4 为上一轮记录，保持原样）

- 背景：本节作者为同一切片的重试派发，到达时 §2 组件已在树且接线完成；实施期间另有并行写者仍在活跃修改 IntegrationsRoutePage.tsx（见 §5.5）。以下变更均经协调者逐项裁决批准。
- 变更（均在独占片区 apps/web/src/integrations/**）：
  1. **采纳 EmbedPreviewModal 为 shell 唯一预览组件**（裁决 1）。重试轮初建的 EmbedChannelPreviewDrawer.tsx / embedPreviewMessages.ts 双实现已删除（存在窗口内无任何外部引用，无残留 import）。
  2. **路由页补齐 locale + refreshKey nonce 接线**（裁决 2，即上一轮 §4.3 自登记遗留）：embedPreview state 增加 locale/refreshKey；useRef nonce 每次打开 +1 → Modal refreshKey → iframe src `?r=N`（Vue previewLocale L1021 / previewNonce L1022，同 token 再预览强制全量重载）；locale 取 channel.default_locale（草稿值仍归 page.tsx 草稿预览课题）。
  3. **空 token / 签发失败面**（裁决 3）：setError → shell 本地 `<p class="wk-status wk-status-error" role="alert">`。页级 error prop 会卸载全部 integrations 面板，重于 Vue MessagePlugin.warning；就地告警保持页面挂载。文案为 embedPublish.previewUnavailable zh-CN 逐字值——该 key 尚未迁移 @weknora/i18n，formatMessage 会返回裸 key，故用字面量；key 上游迁移后可切回。成功路径清除告警；setError 仅保留给列表加载失败。
  4. **EmbedPreviewModal 补 layout 延迟挂载**（裁决 4）：`layoutReady` setTimeout(0) 门，iframe 不再随抽屉同帧挂载（Vue EmbedChannelPreview.vue watch visible L95-104 nextTick；与 views 面板 coordinator 所加 layoutReady 对齐），公开 API 与标记不变。
- 联动测试时序修复（语义锚点不变）：
  - EmbedPreviewModal.test.tsx：预览点击后加一次 tick flush（组件现延迟挂载 iframe）；
  - embedWizardRender.test.tsx：views 面板 layoutReady 50ms 门使 L242「preview iframe mounted」在 flush 前必红（上一轮门禁后由 page.tsx 改动引入，非本切片语义变更），加 60ms flush。经协调者授权的一次性跨文件修补。
- 新增 embedPreviewFallback.test.tsx（2 条路由页回退臂集成测试，JSDOM + stub client，collect-then-assert）：
  - 空 token：previewSession 恰好调用 2 次（views onPreviewSession 尝试 + shell 回退尝试）、零 window.open、无抽屉、渠道列表保持挂载（反页级卸载锚点）、role=alert 告警「预览暂时不可用…」；
  - 回退签发成功：in-page 模态打开、iframe src === `https://weknora.test/embed/ch-1?locale=en-US&r=1#token=tok_fresh`（locale + nonce 入查询串）、无告警、零 window.open。
- 冲突报备：实施中检测到并行写者于 03:24:35 将路由页接线改指本重试轮已删除的 EmbedChannelPreviewDrawer（与裁决 1 方向相反），触发 ERR_MODULE_NOT_FOUND；03:25:28 该写者自行收敛回 EmbedPreviewModal 接线（与裁决一致）。最终落盘状态即本节所述；若上一轮实例仍有在途写，请协调者叫停以免复跑。

## 6. 门禁（重试轮最终，全部对当前落盘状态执行）

```
cd apps/web && npx tsx --test src/integrations/*.test.*
  tests 48 / pass 48 / fail 0 （45 基线 + EmbedPreviewModal 1 + 重试轮 fallback 2）
npx tsx --test ../../packages/views/src/integrations/*.test.*
  tests 57 / pass 57 / fail 0 （基线无回退）
pnpm run typecheck:web
  exit 0（全仓干净）
```

遗留（承 §4，本轮更新）：§4.3 的路由侧 locale/nonce 已闭合；草稿外观（form 值透传）仍归 page.tsx 草稿预览课题（协调者自行接线）；§4.4 浏览器证据未新增。
