Review partially complete: 290 finding(s); 111 of 315 selected item(s) failed.

─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:4-7 ───
[maintainability · medium] 脚本硬编码了机器强相关的绝对路径与端口：require 依赖 /tmp/wk-t33-automator、SHOTS 指向个人 worktree
绝对路径 /Users/wuyongjun/…、connectRetry(9433, 4)
固定自动化端口。同仓既有约定（apps/miniprogram/tests/live/t24r1-live-driver.cjs）是 require('miniprogram-automator')
+ process.env.T24R1_SHOTS / T24R1_AUTO_PORT 注入并做缺失检查。换机器、换目录或 /tmp 清理后本证据脚本即不可重放，建议改为 env 注入（缺省回落到
__dirname 存证据）并对齐 T24 写法。

- const automator = require('/tmp/wk-t33-automator/node_modules/miniprogram-automator');
+ const automator = require('miniprogram-automator');
  const sleep = ms => new Promise(r => setTimeout(r, ms));
  const log = (...a) => console.log('[t33]', ...a);
- const SHOTS = '/Users/wuyongjun/.codex/worktrees/issue-140-t33-closure/WeKnora-fork01/.superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence';
+ const SHOTS = process.env.T33_SHOTS || __dirname; // 截图目录经 env 注入，缺省落在本 evidence 目录
+ // 连接处：const mp = await connectRetry(Number(process.env.T33_AUTO_PORT || 9433), 4);


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:68-70 ───
[test · medium] 「勾选同意」是登录成功的前置条件，但失败时只 log 不 record，流程仍继续点登录：若勾选未生效，最终只有 login 步骤 FAIL
且无法定位根因，RESULT 与真实状态可能偏离。另外 tapListRow 命中的是包含目标文本的第一个 view（文档序中靠前的祖先容器，而非 checkbox 本身），tap
是否真的切换了勾选态也未校验。建议该步纳入 record 并在失败时快速失败，与 T24 驱动「tap 超时即 throw」的约定保持一致。

      const consentOk = await tapListRow(page, '我已了解平台的数据使用与服务说明', 8000);
-     log('consent tap', consentOk);
+     record('consent', consentOk, 'consent checkbox tapped before login');
+     if (!consentOk) throw new Error('consent checkbox not reachable');
      await sleep(800);


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:118-123 ───
[test · medium] 脚本任何步骤 FAIL 或捕获 driver-error 后仍以退出码 0 结束。对照 T24 驱动收尾的 process.exitCode =
failed.length ? 1 : 0，本脚本若被门禁/复验工具按退出码判定会误判为通过，削弱证据可信度。建议在 finally 中依据 results 设置退出码。

    } catch (e) {
      record('driver-error', false, e.message);
    } finally {
      log('RESULT', JSON.stringify(results));
+     process.exitCode = results.some(r => !r.pass) ? 1 : 0;
      try { await mp.disconnect(); } catch {}
    }


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:11-15 ───
[maintainability · low] connectRetry/allTexts/waitText/tapText/relaunch/shot 与
apps/miniprogram/tests/live/t24r1-live-driver.cjs 中的同名辅助函数近乎逐行复制（仅 sleep 间隔、超时默认值、tapText 返回 false
而非 throw 等细微漂移），后续选择器或 automator 协议变更需多点同步修改。建议抽公共 live 驱动辅助模块供各脚本 require
复用；若本文件定位为一次性存档证据、不参与后续维护，请在文件头显式注明，避免后人误当活代码同步。

- async function connectRetry(port, minutes) {
-   const deadline = Date.now() + minutes * 60000;
-   while (Date.now() < deadline) { try { return await automator.connect({ wsEndpoint: `ws://127.0.0.1:${port}` }); } catch { await sleep(5000); } }
-   throw new Error('connect timeout');
- }
+ // 建议抽取公共辅助模块复用，例如：
+ // const { connectRetry, allTexts, waitText, relaunch } = require('apps/miniprogram/tests/live/wx-live-helpers.cjs');
+ // 若确为一次性存档证据，请在文件头注明「本文件为 T33 存档快照，不再同步维护」。


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:108-110 ───
[test · medium] apply-material-page 断言正则存在假阳性：`/2026 秋招 A 批|前端开发实习|投递|材料/`
中的「材料」「投递」是页面常驻静态文案（application-material.tsx 中 `<Screen
title='申请与材料'>`、副标题「一份申请，一份材料，一次本人投递。」等），页面只要渲染出来就会命中。即使用于验证的 Web 同源申请数据（「2026 秋招 A
批」「前端开发实习」）完全未加载/不可见，seenApply 仍为 true 并记 PASS，无法证明「Web 同源申请可见」这一证据目标。建议与第 3 步 career-page 断言（精确匹配
毕业时间：2026-06 且 修订 2）对齐，仅断言数据专属标记，并把静态文案从匹配集中移除；如需确认页面可达，可拆成独立的 page-reached 断言。

      const applyText = await waitText(page, /申请|材料|批次/, 25000).catch(() => allTexts(page));
-     const seenApply = /2026 秋招 A 批|前端开发实习|投递|材料/.test(applyText);
+     const seenApply = /2026 秋招 A 批|前端开发实习/.test(applyText);
      record('apply-material-page', seenApply, applyText.slice(0, 500).replace(/\n/g, '|'));


─── apps/web/src/embed/embed-u.css:225-226 ───
[maintainability · medium] 此处底色由原 utility bg-[#f8fafc] 改为 #f3f3f3，与文件头声明的"值 = utilities
编码的生效值"平移承诺不符。虽然注释说明了 TDesign 基线来源（已核实 light 下 --td-bg-color-secondarycontainer = --td-gray-color-1
= #f3f3f3），但同页消息气泡浅底仍为 #f5f7fa（.wk-emb-10/.wk-emb-18），跟进问题面板将呈现两种近似的浅灰底并存，视觉上易显不一致。建议：要么保持 1:1 平移恢复
#f8fafc，将 TDesign 基线对齐作为独立变更并附视觉回归验证；要么同域统一浅底取值。

-   /* 旧栈浅底→Vue var(--td-bg-color-secondarycontainer)=#f3f3f3 */
-   background-color: #f3f3f3;
+   /* 平移原 utility 生效值；TDesign 基线对齐需另行评审 */
+   background-color: #f8fafc;


─── apps/embed/src/styles.css:45-45 ───
[bug · medium] primary 边框色硬编码 #07c05f（与旧栈 TDesign brand 同色，作为独立按钮时视觉无误），但 EmbedApp
中发送按钮（variant="primary" className="embed-send"）的 .embed-send 规则只用 --embed-primary（默认 #2864dc
蓝）覆盖了背景/文字色，未覆盖 border-color；基类 border-width: 1px + 此处 #07c05f 将在蓝色发送按钮四周显出 1px 绿色描边（焦点环 color-mix
#07c05f 35% 同理与蓝色主色不协调）。packages/ui 源码已删除、旧行为无法对照，建议在 .embed-send 中一并覆盖 border-color（如
transparent），使边框跟随 --embed-primary，避免复刻硬编码值在重着色场景下显形。

  .embed-btn--primary { border-color: #07c05f; background-color: #07c05f; color: #ffffff; }
+ /* 同步在 .embed-send 中补充：border-color: transparent !important;（或跟随 var(--embed-primary)） */


─── apps/web/src/embed/EmbedEntryPage.tsx:826-826 ───
[bug · low] 残留 Tailwind utility：apps/web 已移除 Tailwind（styles.css 与 vite.config.ts 均无 tailwindcss
引入），underline-offset-2 不再生成任何 CSS，外链引用 hover 下划线将退化为默认偏移。应把 text-underline-offset: 2px 并入 .wk-emb-34
平移，删除该残留类名。

-             className="underline-offset-2 wk-emb-34"
+             className="wk-emb-34"


─── apps/web/src/embed/EmbedEntryPage.tsx:718-718 ───
[maintainability · low] 平移后保留的域类 embed-followups / embed-suggested / embed-answer-row /
embed-citation-float / embed-refs / embed-ref-chunk / embed-history-loading 在 apps/web 全域没有任何 CSS
定义（embed-chat.css 只定义 .embed-chat-markdown 与 mermaid 样式），也没有 querySelector/getElementsByClassName
引用（TS 中仅引用 .citation-kb），属无效类名残留。注意 apps/embed/src/styles.css 中的 .embed-followups 是另一个应用（embed
独立包）的同名规则，对 web 端不生效。若确认无外部脚本按类名定位，建议清理这些残留类名，避免后续维护者误以为存在对应域样式。



─── apps/web/src/embed/embed-u.css:315-318 ───
[bug · low] 换行属性平移存在语义偏差且未附说明：原 utility break-all 的等效映射是 word-break: break-all（仓库其他平移/域样式如
settings.td.css、chat.td.css 均如此使用），wk-emb-22 原 break-words 的映射是 overflow-wrap: break-word；此处统一改为
overflow-wrap: anywhere 会改变 min-content 固有宽度计算与长 URL 换行时机。与 .wk-emb-1/.wk-emb-24
的例外注释不同，此处未说明取舍依据，违背文件头"生效值"承诺。建议按原 utility 映射改回，或补注释说明有意选择 anywhere。

  .wk-emb-34 {
-   overflow-wrap: anywhere;
+   word-break: break-all;
    color: var(--embed-primary,#2563eb);
  }


─── apps/web/src/embed/embed-u.css:341-341 ───
[maintainability · low] 文件末尾的全局 @keyframes pulse 与
../chat/views-chat-u.css（两处）、../documents/documents-u.css 中的同名定义重复（EmbedEntryPage 的导入顺序使
views-chat-u.css 的后定义覆盖此处的，当前取值恰好一致故无实际影响）。同名全局关键帧散落多份且靠导入顺序兜底，一旦某份取值调整即会产生跨页覆盖隐患。建议改用域前缀命名（如
wk-emb-pulse）使 .wk-emb-14 的骨架动画自包含，或在注释中声明与 views-chat-u.css 的取值同步约束。

- @keyframes pulse { 50% { opacity: .5; } }
+ @keyframes wk-emb-pulse { 50% { opacity: .5; } }
+ /* .wk-emb-14 的 animation 相应改为 pulse → wk-emb-pulse */


─── apps/web/src/embed/embed-u.css:30-30 ───
[style · low] 格式与冗余声明：.wk-emb-3（及 .wk-emb-31 的 border-top-width: 1px;border-color: #e7eaef;）出现
"1px;border-color" 两条声明挤在同一行，与文件其余逐行风格不一致，影响可读性；另外 .wk-emb-15:hover 与 .wk-emb-29:hover 中重复声明
border-style: solid（基类已设）属无效冗余，可一并清理。

-   border-bottom-width: 1px;border-color: #eef1f5;
+   border-bottom-width: 1px;
+   border-color: #eef1f5;


─── apps/web/src/styles.css:2-2 ───
[bug · medium] Tailwind 全量移除（utilities import 与 @source 扫描删除）后仍存在依赖 utility 的残留消费者，样式将无编译报错地静默丢失：(1)
裸 `wk-muted` —— 本文件注释声称 "every consumer carries the text-muted utility"，但 utility 引擎已删除；全仓库 CSS 中
`.wk-muted` 仅 settings/settings.td.css:4913、knowledge-settings/KnowledgeSettingsPage.css:544 及
integrations.td.css 的 overlay
作用域内有定义，appconnector/ActionApproval.tsx:151,172、appconnector/AppsPage.tsx:14、appconnector/Authorizat
ionPage.tsx:12、appconnector/ConnectionsPage.tsx:16、integrations/IntegrationsPage.tsx:63,66、settings/
ConfigSettingsPanel.tsx:202（configuration 页面路径不加载 settings.td.css）等裸类消费者将退回 body 主文本色。(2) 残留
Tailwind 类 ——
`inset-x-0`（KnowledgeDocumentsPage.tsx:2286、KnowledgeBaseShareDialog.tsx:36，kb-u.css:443 的 .wk-kbs-4
仅定义 position:absolute 而未承接
left/right:0）、`underline-offset-2`（EmbedEntryPage.tsx:826）均无任何定义。建议：迁移收尾时全局扫描一次 `className` 中的
utility 模式并补齐域 css 承接规则，或恢复一条兜底的 `.wk-muted` 全局定义。



─── packages/design-tokens/src/styles.css:23-23 ───
[maintainability · medium] 此 @import 插在 :root 规则之后违反 CSS 规范（@import 须先于所有规则），当前依赖 Vite/postcss
构建期内联才生效，任何不经打包直接消费本文件的场景会整条丢弃 import，--td-* 品牌色覆盖静默失效。另外两点：(1) tdesign-theme.css 不只含 token，还含
`input:-webkit-autofill`、`.t-radio-group .t-radio-button`、`.doc-link` 等全局元素/组件选择器规则，随本文件无条件进入
apps/embed（layer(theme)，不使用 TDesign）——embed 的所有 input 被注入 autofill 盒阴影样式，属跨包样式泄漏；(2) apps/web 对
tdesign-theme.css 形成双份引入（直引 unlayered 生效 + 本文件内嵌进 layer(theme) 被压制的死代码），产物重复一份。建议：把
tdesign-theme.css 从本文件拆出、由使用 TDesign 的应用显式引入（apps/web 已有直引），embed 即不承接副作用，apps/web 也消除双份；若保留则至少将
@import 移到文件首行。



─── apps/web/src/tdesign-icon-offline.ts:8-9 ───
[maintainability · medium] 守卫硬编码单一 0.4.5 CDN URL 与 tdesign-icons-react 0.6.11 内部实现强耦合：依赖升级改变内部 URL
或去重类名后 querySelector 不命中，守卫静默失效，内网/离线部署恢复对 tdesign.gtimg.com 的外发请求（图标加载失败 + 环境信息外泄），无任何告警。Vue 端
frontend/src/utils/tdesign-icon-offline.ts:27 是 5 个版本（0.4.0–0.4.4）参数化生成 URL 的模式，React
端却退化为单版本硬编码。建议对齐 Vue 端的版本列表模式，并在安装后用 MutationObserver 观测
`script.t-svg-js-stylesheet--unique-class:not([data-weknora-blocked-cdn])` 的出现并
console.warn，使失效可被发现。

- const BLOCKED_SCRIPT_URL = 'https://tdesign.gtimg.com/icon/0.4.5/fonts/index.js';
- const BLOCKED_LINK_URL = 'https://tdesign.gtimg.com/icon/0.4.5/fonts/index.css';
+ // 对齐 Vue 端多版本并存模式；升级 tdesign-icons-react 时同步扩充版本列表。
+ const BLOCKED_ICON_VERSIONS = ['0.4.5'];
+ const BLOCKED_SCRIPT_URLS = BLOCKED_ICON_VERSIONS.map((v) => `https://tdesign.gtimg.com/icon/${v}/fonts/index.js`);
+ const BLOCKED_LINK_URLS = BLOCKED_ICON_VERSIONS.map((v) => `https://tdesign.gtimg.com/icon/${v}/fonts/index.css`);


─── apps/web/src/styles.css:41-42 ───
[documentation · low] 注释指向的覆盖规则实际不在 platform-shell.td.css（该文件中无 plat-shell__outlet/wk-page 规则），而在
platform/platform-u.css:658（`.plat-shell__outlet .wk-page { ... max-width: none !important;
}`）。后续维护者按注释找壳层覆盖链会找错文件，建议更正文件名。

-    PlatformShell 壳内由 platform-shell.td.css 的 .plat-shell__outlet .wk-page
+    PlatformShell 壳内由 platform-u.css 的 .plat-shell__outlet .wk-page
     覆盖为全宽滚动（max-width:none!important，层叠与先前壳层 utility 等价）。 */


─── packages/design-tokens/src/tdesign-theme.css:101-106 ───
[maintainability · low] 深色滚动条颜色硬编码 #4b4b4b/#5e5e5e，绕过本文件自身的 token 体系——两值恰为
--td-gray-color-10/--td-gray-color-9，直接引用 var() 可保持主题定制一致性；同时 `:root[theme-mode="dark"]
*::-webkit-scrollbar-*` 全选择器 + !important 会压制任何组件级滚动条定制，后续需要逐条对抗该声明。虽为 Vue 端平移，建议至少将色值 token
化并补一行说明。

  :root[theme-mode="dark"] *::-webkit-scrollbar-thumb {
-     background-color: #4b4b4b !important;
+     background-color: var(--td-gray-color-10, #4b4b4b) !important;
  }
  :root[theme-mode="dark"] *::-webkit-scrollbar-thumb:hover {
-     background-color: #5e5e5e !important;
+     background-color: var(--td-gray-color-9, #5e5e5e) !important;
  }


─── internal/modules/career/handler.go:1550-1554 ───
[bug · medium] GET /sources 的读路径上，cleanupCatalogCandidates 内部的 h.upload.Release(ctx, ref, sourceID)
直接使用请求 ctx 且无超时上限。对比同文件 reconcileStaleSourcesExcept 对同类 Release 调用特意构造了 5s 超时的 detached
context，这里一旦存储后端停滞，列表请求会随客户端一直挂起；且该清理对每个 source 逐个执行（N+1 次查询+可能的外部删除），混入 GET 语义。建议给清理调用套用与
reconcileStaleSourcesExcept 相同的有界 context（或复用该 helper 的超时模式），并将清理移出同步读路径/限制每次请求的清理量。

  	for _, source := range v {
- 		if cleanupErr := h.cleanupCatalogCandidates(ctx, source.ID); cleanupErr != nil {
+ 		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
+ 		if cleanupErr := h.cleanupCatalogCandidates(cleanupCtx, source.ID); cleanupErr != nil {
  			slog.Warn("career catalog cleanup pending", "source_id", source.ID)
  		}
+ 		cancel()
  	}


─── internal/modules/career/profile_intake.go:419-425 ───
[bug · low] finishSource 的成功（ready）路径没有写入 completed_at（下方 updates map 中也没有该列），而 completeIntake 的
ready 路径会设置 CompletedAt。两条置 ready 的路径产物不一致：通过 FinishSourceClaim 完成的无字段上传（handler Upload 的
len(result.Fields)==0 分支）返回的 CareerSource.CompletedAt 恒为 nil，与带字段上传走 CompleteIntakeClaim 的结果不同。建议在
ready 分支补充 completed_at 写入，保持两条路径契约一致。

  		} else {
  			if strings.TrimSpace(text) == "" {
  				return ErrInvalidRequest
  			}
  			row.Status = "ready"
  			row.LeaseUntil = nil
  			row.ExtractedText = text
+ 			completed := time.Now().UTC()
+ 			row.CompletedAt = &completed


─── internal/modules/career/handler.go:163-167 ───
[bug · low] writeError 未映射 ErrUploadClaimLost 与 ErrUploadInProgress。Upload handler 中存在兜底路径（如
CompleteIntakeClaim 返回 ErrUploadClaimLost 后 GetSource 也失败时）会将这两个哨兵错误直接传入 writeError，以 500 internal +
"career upload claim was superseded" 的语义返回，与既定的 202/冲突契约不符。建议补充映射（claim lost → 409，in progress → 202
语义由调用方处理，此处至少给一个类型化 code），避免泄漏为 500。

  	case errors.Is(e, ErrExportSigningKeyMissing):
  		status = http.StatusNotImplemented
  		code = "export_signing_key_missing"
+ 	case errors.Is(e, ErrUploadClaimLost):
+ 		status = 409
+ 		code = "upload_claim_lost"
+ 	case errors.Is(e, ErrUploadInProgress):
+ 		status = http.StatusAccepted
+ 		code = "upload_in_progress"
  	}
- 	body := gin.H{"code": code, "message": e.Error()}


─── packages/api-client/src/career.ts:1-2 ───
[maintainability · medium] 未声明依赖的跨包相对路径导入：通过 '../../career-core/src/contracts.ts' 深层相对路径跨包引用，同时绕过了
career-core package.json 中 exports 声明的 './contracts' 入口，且 packages/api-client/package.json 的
dependencies 未声明 @weknora/career-core（apps/miniprogram 同样以相对路径引入）。当前全靠磁盘路径与 tsx/tsc
直载生效；一旦目录调整、包独立发布或单独对 api-client 做 typecheck/构建即会断裂。建议在 dependencies 中声明 "@weknora/career-core":
"workspace:*" 并改用包名子路径导入。

- import type { CareerAction, CareerChangeSet, CareerDocumentSource, CareerFact, CareerProposal, CareerReceipt, CareerSource, CareerUpload, CareerView, Evaluation, EvaluationReceipt, OpportunityEvidence, OpportunityImportInput, OpportunityReceipt, OpportunitySource, OpportunityStatus } from '../../career-core/src/contracts.ts'
- import { decodeCareerReceipt, decodeCareerSources, decodeCareerUpload, decodeEvaluation, decodeEvaluationReceipt, decodeOpportunityReceipt } from '../../career-core/src/contracts.ts'
+ import type { CareerAction, CareerChangeSet, CareerDocumentSource, CareerFact, CareerProposal, CareerReceipt, CareerSource, CareerUpload, CareerView, Evaluation, EvaluationReceipt, OpportunityEvidence, OpportunityImportInput, OpportunityReceipt, OpportunitySource, OpportunityStatus } from '@weknora/career-core/contracts'
+ import { decodeCareerReceipt, decodeCareerSources, decodeCareerUpload, decodeEvaluation, decodeEvaluationReceipt, decodeOpportunityReceipt } from '@weknora/career-core/contracts'
+ // 并在 packages/api-client/package.json 的 dependencies 补充：
+ // "@weknora/career-core": "workspace:*"


─── packages/api-client/src/index.ts:209-209 ───
[maintainability · medium] 这里的再导出同样通过 '../../career-core/src/contracts.ts' 相对路径跨包引用，绕过 career-core 的
exports 入口且依赖未在 package.json 中声明（与 career.ts 的导入同源问题）。建议统一改为从 '@weknora/career-core/contracts'
再导出，并声明 workspace 依赖。

- export type { CareerAction, CareerView, CareerFact, CareerProposal, CareerReceipt, CareerChangeSet, CareerChange, CareerSource } from '../../career-core/src/contracts.ts';
+ export type { CareerAction, CareerView, CareerFact, CareerProposal, CareerReceipt, CareerChangeSet, CareerChange, CareerSource } from '@weknora/career-core/contracts';


─── packages/api-client/src/career.ts:1239-1241 ───
[bug · medium] open()/list()/changes() 是本文件中唯一未经过解码器校验的端点：响应被直接 as CareerView / as CareerChangeSet
强转返回。这些视图会直接流入 CareerDesk.applyView 的状态机和 UI，而文件自身的冻结契约声明"解码器在任何发明负载到达 UI
前拒绝"——畸形或被篡改的服务端负载将未经拒绝直接进入客户端状态。建议补充 decodeCareerView / decodeCareerChangeSet 解码器（archive 解码中的
decodeExportedProfile 已有对 CareerView 形状的完整校验，可抽取复用）。

-   async open(signal?: AbortSignal): Promise<CareerView> { return await request({ method: 'GET', path: '/api/v1/career/open', ...(signal ? { signal } : {}) }) as CareerView },
-   async list(signal?: AbortSignal): Promise<CareerView> { return await request({ method: 'GET', path: '/api/v1/career/list', ...(signal ? { signal } : {}) }) as CareerView },
-   async changes(since: number, signal?: AbortSignal): Promise<CareerChangeSet> { return await request({ method: 'GET', path: `/api/v1/career/changes?since=${encodeURIComponent(String(since))}`, ...(signal ? { signal } : {}) }) as CareerChangeSet },
+   async open(signal?: AbortSignal): Promise<CareerView> { return decodeCareerView(await request({ method: 'GET', path: '/api/v1/career/open', ...(signal ? { signal } : {}) })) },
+   async list(signal?: AbortSignal): Promise<CareerView> { return decodeCareerView(await request({ method: 'GET', path: '/api/v1/career/list', ...(signal ? { signal } : {}) })) },
+   async changes(since: number, signal?: AbortSignal): Promise<CareerChangeSet> { return decodeCareerChangeSet(await request({ method: 'GET', path: `/api/v1/career/changes?since=${encodeURIComponent(String(since))}`, ...(signal ? { signal } : {}) })) },


─── packages/api-client/src/career.ts:38-38 ───
[maintainability · medium] validIdentifier、validTimestamp、decodeRecord/isRecord、sha256 正则等基础校验器在
packages/career-core/src/contracts.ts 与本文件重复实现，且两份 validTimestamp 行为并不一致：career-core
版做完整日历校验（闰年/月日/时分秒/时区偏移范围），本版仅正则 + Date.parse——同一时间戳在 V8、Hermes、JSCore 等不同引擎的 Date.parse
宽松度不同，可能出现一层接受、另一层拒绝的解码漂移，且后续维护中两份实现各自演化后难以排查。career-core 已是本包的导入来源，建议将共享校验器从 career-core 导出（例如新增
'./validate' 导出）后统一导入。

- function validTimestamp(value: unknown): value is string { return typeof value === 'string' && rfc3339Timestamp.test(value) && Number.isFinite(Date.parse(value)) }
+ // packages/career-core/src/contracts.ts（或新增 validate.ts）导出后统一导入：
+ import { validIdentifier, validTimestamp, validOptionalString, decodeRecord } from '@weknora/career-core/contracts'


─── packages/api-client/src/career.ts:1027-1027 ───
[bug · medium] 解码器比其注释声明的冻结契约宽松：注释（与后端 resolveReminderSource 的真实输出一致）声明 progress_event 必带
application 并带其 opportunity、discovery 两者皆不带，但代码只校验 applicationId 存在性与 sourceKind 等价——discovery 回执携带
opportunityId 时仍会通过解码并保留该字段，progress_event 缺失 opportunityId 时同样通过，发明负载可借此绕过校验到达 UI，违背本文件"任何发明负载到达 UI
前被拒绝"的承诺。建议将 opportunityId 一并纳入等价校验（或如实修正注释）。

-  if ((record.sourceKind === 'progress_event') !== hasApplication) throw new TypeError(message)
+  const isProgressEvent = record.sourceKind === 'progress_event'
+  if (isProgressEvent !== validIdentifier(record.applicationId) || isProgressEvent !== validIdentifier(record.opportunityId)) throw new TypeError(message)


─── package.json:13-13 ───
[test · low] 新增的 packages/i18n/test/*.test.tsx 通配当前匹配 0 个文件：该目录下只有 19 个 .test.ts 文件，不存在任何
.test.tsx。npm 脚本经 sh 执行时，未匹配的 glob 会原样传给 node test runner（node>=26 下静默跑 0 个测试，形成死配置并掩盖事实），而在 zsh
等对未匹配 glob 直接报错的 shell 中手工执行会整条命令失败。同批新增的 packages/views/src/integrations/*.test.tsx 有 page.test.tsx
匹配，仅 i18n 一处落空。建议移除该通配，或补充对应 .test.tsx 测试文件。

-     "test:shared": "tsx --test packages/contracts/test/*.test.ts packages/contracts/test/mobile-*.test.ts packages/contracts/src/craft/*.test.ts packages/domain/src/*.test.ts packages/domain/src/auth/*.test.ts packages/domain/src/access/*.test.ts packages/domain/src/craft/*.test.ts packages/domain/src/knowledge/*.test.ts packages/domain/src/mobile/*.test.ts packages/domain/src/wiki/*.test.ts packages/domain/src/chat/*.test.ts packages/domain/src/sandbox/*.test.ts packages/domain/src/settings/*.test.ts packages/api-client/src/*.test.ts packages/api-client/src/analytics/*.test.ts packages/api-client/src/auth/*.test.ts packages/api-client/src/craft/*.test.ts packages/api-client/src/identity/*.test.ts packages/api-client/src/knowledge/*.test.ts packages/api-client/src/wiki/*.test.ts packages/api-client/src/chat/*.test.ts packages/api-client/src/sandbox/*.test.ts packages/api-client/src/mobile/*.test.ts packages/api-client/src/settings/*.test.ts packages/api-client/src/embed/*.test.ts packages/api-client/src/transport/*.test.ts packages/core/src/craft/*.test.ts packages/design-tokens/src/*.test.ts packages/i18n/test/*.test.ts packages/i18n/test/*.test.tsx packages/views/src/chat/*.test.ts packages/views/src/chat/*.test.tsx packages/views/src/craft/*.test.ts packages/views/src/craft/*.test.tsx packages/views/src/guides/*.test.ts packages/views/src/guides/*.test.tsx packages/views/src/settings/*.test.ts packages/views/src/embed/*.test.ts packages/views/src/integrations/*.test.ts packages/views/src/integrations/*.test.tsx",
+ // 移除 packages/i18n/test/*.test.tsx（该目录暂无 .test.tsx 文件），保留：
+ // ... packages/i18n/test/*.test.ts packages/views/src/chat/*.test.ts ...


─── packages/api-client/src/career.ts:929-930 ───
[style · low] 该行使用了嵌套三元表达式（外层 null/undefined 判断内再嵌 Array.isArray
判断），违反项目"禁止嵌套三元"的规范，且单行承载三态推导可读性差、后续修改易引入优先级错误。建议改写为 if/else 分支。

   const rawFactKeys: unknown = sourcesRecord.factKeys
-  const factKeys = rawFactKeys === null || rawFactKeys === undefined ? [] : Array.isArray(rawFactKeys) ? rawFactKeys : undefined
+  let factKeys: string[] | undefined
+  if (rawFactKeys === null || rawFactKeys === undefined) factKeys = []
+  else if (Array.isArray(rawFactKeys)) factKeys = rawFactKeys


─── packages/career-core/src/desk.ts:26-26 ───
[bug · low] sameAction 用 JSON.stringify 做逐字符比较，对对象键插入顺序敏感：retryUnknown 要求重试携带"完全相同的
action"，若调用方以相同语义字段但不同键序重建 action（例如从持久化存储反序列化恢复的 pendingAction、或由不同构造路径生成的等价负载），将被误判为
retry_payload_mismatch 而拒绝重试，直接阻断 outcome_unknown
场景的恢复路径。建议按键名做规范化（递归排序键）后再比较，或对判别字段（action/requestId/key/value/source/expectedRevision）做结构化比较。

- function sameAction(left: CareerAction, right: CareerAction): boolean { return JSON.stringify(left) === JSON.stringify(right) }
+ function sortedStringify(value: unknown): string {
+  if (Array.isArray(value)) return `[${value.map(sortedStringify).join(',')}]`
+  if (value !== null && typeof value === 'object') return `{${Object.entries(value as Record<string, unknown>).sort(([a], [b]) => a.localeCompare(b)).map(([k, v]) => `${JSON.stringify(k)}:${sortedStringify(v)}`).join(',')}}`
+  return JSON.stringify(value)
+ }
+ function sameAction(left: CareerAction, right: CareerAction): boolean { return sortedStringify(left) === sortedStringify(right) }


─── apps/embed/src/button.tsx:1-1 ───
[maintainability · low] 默认导入的 React 未被使用：apps/embed 的 tsconfig 为 "jsx": "react-jsx"（自动 JSX
运行时），本文件未以 React.createElement 或 React.xxx 形式引用 React，仅需类型导入即可；同应用 EmbedApp.tsx 的既有风格也是只按需导入（无默认
React）。建议改为纯类型导入，避免死代码。

- import React, { type ButtonHTMLAttributes, type ReactNode } from 'react';
+ import type { ButtonHTMLAttributes, ReactNode } from 'react';


─── internal/modules/career/career_export.go:825-829 ───
[security · high] 删除仅撤销 DB 状态，随后 purge 直接清空 career_material_exports
行，但物理文件从未删除：materialExportStorage 接口只有 SaveExport/ReadExport，全代码库（含 DeleteUnbound 调用点，仅 upload.go
使用）没有任何路径能删除已写入的 PDF/DOCX
对象——文件内容是用户简历正文，在"彻底删除"后永久残留于磁盘，违反"导出删除需撤销相关数据访问并彻底删除"红线。同样，career_source_revisions 被 purge 后
resource_ref 失联、目录绑定未释放，上传的简历原件也永久残留。建议：为存储缝隙补 DeleteExport，并在删除步骤中先读出对象键、物理删除后再清行（对上传源经 catalog
Release 释放）。

- func (o *Office) deletionRevokeExports(ctx context.Context, s Scope) error {
- 	return o.db.WithContext(ctx).Model(&materialExportRecord{}).
- 		Where("tenant_id=? AND user_id=? AND status<>?", s.TenantID, s.UserID, ExportStatusRevoked).
- 		Updates(map[string]any{"status": ExportStatusRevoked, "revoked_at": time.Now().UTC(), "updated_at": time.Now().UTC()}).Error
+ // materialExportStorage 需补充 DeleteExport(ctx, objectKey) error，并在删除步骤中先删文件再清行：
+ func (o *Office) deletionRemoveExportFiles(ctx context.Context, s Scope) error {
+ 	if o.exportStorage == nil {
+ 		return nil
+ 	}
+ 	var rows []materialExportRecord
+ 	if err := o.db.WithContext(ctx).
+ 		Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Find(&rows).Error; err != nil {
+ 		return err
+ 	}
+ 	for _, row := range rows {
+ 		for _, key := range []string{row.PDFObjectKey, row.DOCXObjectKey} {
+ 			if key != "" {
+ 				if err := o.exportStorage.DeleteExport(ctx, key); err != nil {
+ 					return fmt.Errorf("remove career export object: %w", err)
+ 				}
+ 			}
+ 		}
+ 	}
+ 	return nil
  }


─── internal/modules/career/career_export.go:678-680 ───
[bug · medium] 删除记录创建到最终 Updates 之间 ReceiptBody 固定为 "{}"：(1) 并发 Create 命中 isReceiptRaceError 时经
FindCareerDeletion 回放，若胜者仍在执行中，decodeCareerDeletionReceipt("{}")
会把零值回执（Kind=""、Status=""、Steps=nil）当成功返回，客户端用同一 request ID 恢复未知结果时收到无意义响应；(2) 执行窗口内 GET
/deletions/receipt 同样返回 200 + 空回执，客户端可能将进行中/未知状态误判。建议 FindCareerDeletion 在 Status==deleting 且回执为初始
"{}" 时，从 StateBody + record.Status 合成进行中回执（或返回 typed in-progress 错误），DeleteCareer 的竞态回放也仅应在状态为
deleted/partial 时直接返回。

- 			ExpectedRevision: input.ExpectedRevision, Status: DeletionStatusDeleting,
- 			StateBody: string(mustJSON(execution)), ReceiptBody: "{}",
- 			CreatedAt: now, UpdatedAt: now,
+ // FindCareerDeletion 内增加进行中防护：
+ 	if record.Status == DeletionStatusDeleting && record.ReceiptBody == "{}" {
+ 		return CareerDeletionReceipt{
+ 			Kind: CareerKindDeleted, RequestID: record.RequestID,
+ 			Status: DeletionStatusDeleting,
+ 			Steps: decodeDeletionExecutionSteps(record.StateBody),
+ 			StartedAt: record.CreatedAt,
+ 		}, nil
+ 	}


─── internal/modules/career/rendering.go:1094-1101 ───
[bug · medium] 两个存储对象在修订 CAS 事务之前就已写入：随后发生 RevisionConflict（客户端修订过期是常见竞态）、幂等竞态回放或任意 DB 错误时，已写入的
PDF/DOCX 对象成为孤儿——存储缝隙没有删除能力，也没有任何补偿路径或清扫机制，冲突重试会持续泄漏含简历正文的文件。建议把 expectedRevision
校验提前（或事务成功后再写存储），至少在失败路径上对已写入的对象做补偿删除。

- 	pdfKey, err := o.exportStorage.SaveExport(ctx, s.TenantID, "career_export_"+exportID+pdfExportExtension, pdfBytes)
+ // 失败路径上补偿删除已写入的对象（DeleteExport 为缝隙需新增的能力）：
  	if err != nil {
- 		return ExportReceipt{}, fmt.Errorf("store career material pdf: %w", err)
+ 		for _, key := range []string{pdfKey, docxKey} {
+ 			if key != "" {
+ 				_ = o.exportStorage.DeleteExport(ctx, key)
- 	}
+ 			}
- 	docxKey, err := o.exportStorage.SaveExport(ctx, s.TenantID, "career_export_"+exportID+docxExportExtension, docxBytes)
- 	if err != nil {
- 		return ExportReceipt{}, fmt.Errorf("store career material docx: %w", err)
+ 		}
+ 		return ExportReceipt{}, err
  	}


─── internal/modules/career/career_export.go:525-528 ───
[bug · low] sectionCount 在 Count 出错时静默返回 0：数据库瞬态故障下，删除前确认面会向用户展示所有分区计数为 0
的"空空间"，用户可能在错误认知下确认删除，与"删除前如实披露"的契约不符。建议把计数错误向上返回，或在视图中显式标注"计数不可用"。

  		if err := query.Count(&total).Error; err != nil {
- 			return 0
+ 			return -1 // 调用方识别负值并返回错误或标注"计数不可用"，避免展示误导性的 0
  		}
  		return int(total)


─── internal/modules/career/reminder.go:452-456 ───
[bug · low] 去重命中路径经 reminderReceiptFromRow 构造的回执未填 Revision（恒为 0），而该回执又被原样持久化到
reminder_receipts，重放端点返回的 Revision 也为 0——与首次写入回执的契约不一致，客户端无法据此做修订对齐。建议在去重事务内读取当前 profile revision
回填。另外 ListReminders 无分页/上限，长期使用后全量返回，建议加上限。

- func reminderReceiptFromRow(row reminderRecord, requestID string, deduplicated bool) ReminderReceipt {
+ func reminderReceiptFromRow(row reminderRecord, requestID string, deduplicated bool, revision uint64) ReminderReceipt {
  	return ReminderReceipt{
  		Kind:          ReminderKindSet,
  		RequestID:     requestID,
+ 		Revision:      revision,
  		ReminderID:    row.ID,


─── internal/modules/career/reconciliation.go:204-206 ───
[performance · low] 快照循环的 break 条件包含 evidence.JobCode != ""，但 JobCode 只在随后的 observation
循环中赋值，快照循环内恒为空字符串，条件恒假——break 永不触发，每次调用都会全量扫描该岗位的所有快照。仅造成不必要的查询/反序列化开销，不影响取值正确性（take
已按最近优先）。建议从条件中移除 JobCode 维度。

- 		if evidence.JobCode != "" && evidence.SnapshotID != "" && evidence.Title != "" && evidence.Company != "" && evidence.Location != "" && evidence.Batch != "" {
+ 		if evidence.SnapshotID != "" && evidence.Title != "" && evidence.Company != "" && evidence.Location != "" && evidence.Batch != "" {
  			break
  		}


─── internal/modules/career/usage.go:415-417 ───
[bug · low] 预估是承诺"免费读"的接口，但 reconcileUsage 内含 UPDATE 写操作且直接走 o.db，未复用模块内其他写入路径统一使用的
runImportTransaction（SQLite 忙时整事务重试）；写锁竞争下读接口会返回 503
admission_unavailable。建议将对账写入套上忙重试事务助手，或在读路径容忍对账失败（跳过对账给出保守余额）。

- 	if err = o.reconcileUsage(ctx, s); err != nil {
+ 	if err := o.runImportTransaction(ctx, func(tx *gorm.DB) error {
+ 		return reconcileUsageTx(tx, s)
+ 	}); err != nil {
  		return UsageEstimateView{}, ErrAdmissionUnavailable
  	}


─── apps/web/src/career/ApplicationPage.tsx:140-144 ───
[bug · medium] 回执校验不匹配时抛出的 TypeError（确定性协议错误）会被外层 catch 的 isUncertainWrite 判为 true（无 code/status
时默认未知），phase 进入 'unknown' 并提示"暂时无法确认申请是否已创建…用原请求编号查询回执"——把确定性协议错误误导为可恢复的未知写入。ExportDeletionPage /
InboxPage 已引入 ReceiptMismatchError 并在 catch 中、isUncertainWrite 之前单独分流，本页面（及
MaterialPage、OpportunityPage 的同类 throw）应对齐该模式。

+ class ReceiptMismatchError extends Error {}
   const acceptReceipt = (next: ApplicationReceipt, expected: Attempt): void => {
    if (next.requestId !== expected.requestId
     || next.pinnedEvidence.opportunityId !== expected.input.opportunityId
     || next.pinnedEvidence.snapshotId !== expected.input.snapshotId
-    || next.pinnedEvidence.evaluationId !== expected.input.evaluationId) throw new TypeError('申请回执与本次固定证据不匹配')
+    || next.pinnedEvidence.evaluationId !== expected.input.evaluationId) throw new ReceiptMismatchError('申请回执与本次固定证据不匹配')
+ // catch 中、isUncertainWrite 判断之前：
+ //   if (cause instanceof ReceiptMismatchError) { setAttempt(undefined); setPhase('error'); setMessage(cause.message); return }


─── apps/web/src/career/ApplicationPage.tsx:25-28 ───
[maintainability · medium] errorDetails / isUncertainWrite / newRequestId 在 career 目录 5
个页面（ApplicationPage、ExportDeletionPage、InboxPage、MaterialPage、OpportunityPage）逐字重复，且各份错误码白名单已漂移：本文件多
出 application_conflict、hard_ineligible_requires_continue，MaterialPage 多出
material_claim_unconfirmed，OpportunityPage 少 revision_conflict。后端新增错误码时需同步 5
处，漏改任一份会直接改变"未知写入"判定，影响幂等恢复正确性。建议抽取为 career 目录共享模块（如 recovery.ts），错误码白名单以参数按端点扩展。

- function isUncertainWrite(cause: unknown): boolean {
-  const error = errorDetails(cause)
-  if (error.code === 'TIMEOUT' || error.code === 'outcome_unknown') return true
-  if (['forbidden', 'invalid_request', 'idempotency_conflict', 'revision_conflict', 'application_conflict', 'hard_ineligible_requires_continue', 'request_too_large', 'PAYLOAD_TOO_LARGE', 'not_found', 'unauthorized'].includes(error.code ?? '')) return false
+ // apps/web/src/career/recovery.ts
+ export const newRequestId = (): string => ...
+ export function errorDetails(cause: unknown) { ... }
+ export function isUncertainWrite(cause: unknown, definiteCodes: readonly string[] = baseDefiniteCodes): boolean { ... }
+ // 页面内：isUncertainWrite(cause, ['application_conflict', 'hard_ineligible_requires_continue'])


─── apps/web/src/career/OpportunityPage.tsx:83-84 ───
[bug · medium] 与 ApplicationPage / MaterialPage 同一问题：回执不匹配抛出的 TypeError 会被 catch 尾部统一置为
'unknown'（importURLAttempt / importAttempt 均是"非白名单即 unknown"，runEvaluation 则经 isUncertainWrite 默认
true），确定性协议错误被表述为"暂时无法确认…请用原请求编号恢复"，恢复语义失真。应对齐 ExportDeletionPage / InboxPage 已建立的
ReceiptMismatchError 分流模式。

+ class ReceiptMismatchError extends Error {}
   const acceptReceipt = (next: OpportunityReceipt, expected: Attempt): void => {
-   if (!sameReceipt(next, expected)) throw new TypeError('服务返回的请求编号与本次导入不匹配')
+   if (!sameReceipt(next, expected)) throw new ReceiptMismatchError('服务返回的请求编号与本次导入不匹配')
+ // catch 分支在置 unknown 之前先判断：
+ //   if (cause instanceof ReceiptMismatchError) { setState('error'); setAttempt(undefined); setMessage(cause.message); return }


─── apps/web/src/career/MaterialPage.tsx:213-217 ───
[bug · medium] saveDraft / confirmDraft 成功路径中 await reloadView(receipt.materialId)
无保护：若此只读刷新失败（网络错误无 code/status），会落入外层 catch 且 isUncertainWrite 判 true，phase 进入 'unknown'
并提示"暂时无法确认材料写入是否完成"——但写入实际已成功、materialId 与 URL 已更新，误导用户用原编号"恢复"。此外 catch 内
material_claim_unconfirmed 分支的 `if (materialId) await reloadView(materialId)` 再抛错时（已处于 catch 块）将产生
unhandled rejection（调用方为 void）。建议 reloadView 失败仅作非致命提示、不参与成功/失败判定。

     const url = materialParamUrl(receipt.materialId)
     if (url) window.history.replaceState({}, document.title, url)
-    await reloadView(receipt.materialId)
-    if (!scopeController.isCurrent(requestScope.scope)) return
     setPhase('idle'); setMessage(`草稿已保存（材料编号 ${receipt.materialId}）`)
+    try { await reloadView(receipt.materialId) }
+    catch { /* 只读刷新失败不影响已确认的写入结果，可在消息中附注 */ }


─── apps/web/src/career/MaterialPage.tsx:112-112 ───
[bug · medium] dirty 用 JSON.stringify 全等比较，依赖"savedBody（由 decodeMaterialBody 固定键序重建）与
bodyFromEditable 产出键序、trim 行为完全对称"这一隐式不变式：bodyFromEditable 会对 heading/factKey 做 trim
再比较，一旦服务端（或其他端如小程序写入的同一材料）返回的 body 含带空白的 heading/factKey，或服务端对存储 body 做任何规范化/字段增减，保存后 dirty 永真 →
"确认发布不可变版本"持续禁用且按钮 title 提示"正文有未保存修改"。建议改为键序无关的结构化深度比较（或比较前对两侧统一规范化）。

-  const dirty = savedBody === undefined || JSON.stringify(bodyFromEditable(sections)) !== JSON.stringify(savedBody)
+  const normalizeBody = (body: MaterialBody): string => JSON.stringify({
+   sections: body.sections.map((section) => ({
+    heading: section.heading.trim(),
+    content: section.content,
+    claims: section.claims.map(({ claimId, text, factKey, needsReview, reviewNote }) => ({ claimId, text: text.trim(), factKey: factKey?.trim() ?? '', needsReview, reviewNote: reviewNote?.trim() ?? '' })),
+   })),
+  })
+  const dirty = savedBody === undefined || normalizeBody(bodyFromEditable(sections)) !== normalizeBody(savedBody)


─── apps/web/src/career/MaterialPage.tsx:395-397 ───
[bug · low] publishExport 与 revokeExport 的 revision_conflict 分支调用的是主编辑区的
setMessage，而"重新读取档案修订"按钮（exportPhase==='error' && exportConflict!==undefined 时）渲染在下方导出区内且读取的是
exportMessage；错误文案与恢复按钮分属两个区域，导出区消息为空时用户在按钮旁看不到错误说明。应与同函数其余分支一致使用 setExportMessage。

     if (parsed.code === 'revision_conflict') {
      setExportAttempt(undefined); setExportPhase('error'); setExportConflict(parsed.currentRevision)
-     setMessage(`档案已更新${parsed.currentRevision !== undefined ? `（当前修订 ${parsed.currentRevision}）` : ''}。请重新读取档案修订后再次发布导出；新提交会使用新的请求编号。`)
+     setExportMessage(`档案已更新${parsed.currentRevision !== undefined ? `（当前修订 ${parsed.currentRevision}）` : ''}。请重新读取档案修订后再次发布导出；新提交会使用新的请求编号。`)


─── apps/web/src/career/MaterialPage.tsx:16-18 ───
[documentation · low] 常量与注释自相矛盾：注释声称"Mirrors MaxExportGrantTTL …至多 15 分钟"，但 600 秒 = 10 分钟；后端
rendering.go 实际为 MaxExportGrantTTL = 15 * time.Minute（900s），600s 在上限内虽可被接受，但"镜像"的说法不成立，后续维护者据注释改成
≥900 或误读上限都会出问题。建议要么改为 900 并保留注释，要么把注释改为"低于后端 15 分钟上限的保守值"。

- // Mirrors MaxExportGrantTTL in internal/modules/career/rendering.go: a
- // career export download grant lives at most 15 minutes.
- const EXPORT_GRANT_TTL_SECONDS = 600
+ // Kept below MaxExportGrantTTL (15 min) in internal/modules/career/rendering.go;
+ // the backend rejects any requested TTL above that cap.
+ const EXPORT_GRANT_TTL_SECONDS = 900


─── apps/web/src/career/MaterialPage.tsx:552-555 ───
[style · low] statusText 为三层嵌套三元（外层三分支内还套 deliverable 判断），违反团队"禁止嵌套三元"规则，同类模式还见于本文件的导出成功文案
success、ApplicationPage 的 evaluationStatusLabel、CareerPage 的来源状态文案等多处。长条件链可读性差、修改时易错分支，建议提取为函数/映射表。

-      const statusText = receipt.kind === 'material_export_revoked' ? '已撤销（revoked）：旧下载授权立即失效'
-       : receipt.status === 'submittable' ? (deliverable ? '双格式核验通过（submittable）' : '双格式核验记录不一致，暂不提供投递')
-       : receipt.status === 'staged' ? '已暂存（staged）：仅一种格式核验通过，不可投递'
-       : '双格式核验失败（failed），不可投递'
+      const statusText = (() => {
+       if (receipt.kind === 'material_export_revoked') return '已撤销（revoked）：旧下载授权立即失效'
+       if (receipt.status === 'submittable') return deliverable ? '双格式核验通过（submittable）' : '双格式核验记录不一致，暂不提供投递'
+       if (receipt.status === 'staged') return '已暂存（staged）：仅一种格式核验通过，不可投递'
+       return '双格式核验失败（failed），不可投递'
+      })()


─── apps/web/src/career/InboxPage.tsx:126-131 ───
[performance · medium] 读取待办时在 for 循环内逐条 await client.career.application(...) 串行解析申请引用（N+1
请求）；各申请回执彼此独立，待办数量增多时首屏线性变慢。建议先去重后用 Promise.all 并行请求，保留逐条 'failed' 降级与 scope 失效检查。

-     const refs: Record<string, ApplicationRef | 'failed'> = {}
-     for (const item of next.reminders) {
-      const applicationId = item.applicationId
-      if (!applicationId || applicationId in refs) continue
+     const ids = [...new Set(next.reminders.map((item) => item.applicationId).filter((id): id is string => Boolean(id)))]
+     const entries = await Promise.all(ids.map(async (applicationId) => {
       try {
        const receipt = await client.career.application(applicationId, requestScope.signal)
+       return [applicationId, { snapshotId: receipt.pinnedEvidence.snapshotId, opportunityId: receipt.pinnedEvidence.opportunityId }] as const
+      } catch { return [applicationId, 'failed'] as const }
+     }))
+     if (!scopeController.isCurrent(requestScope.scope)) return
+     const refs: Record<string, ApplicationRef | 'failed'> = Object.fromEntries(entries)


─── apps/web/src/career/InboxPage.tsx:189-189 ───
[bug · low] 订阅推送写入的 expectedRevision 取 view?.revision ?? 0：?? 0 会把"档案修订未知"伪装成修订 0 发出无意义请求（同模块其他页面在
revision===undefined 时一律阻断）；且恢复路径 runWrite(attempt) 重试时动态取当下 view.revision 而非固化发起时的修订——与 reminder
分支把 expectedRevision 固化进 input、其他页面 attempt 固化原修订的恢复语义不一致，档案修订变化后的"原请求编号重试"会得到 revision_conflict。

-     const action: CareerAction = { action: 'confirm', key: PUSH_FACT_KEY, value: current.value, source: { kind: 'user', label: '本人确认' }, requestId: current.requestId, expectedRevision: view?.revision ?? 0 }
+ // 发起时固化修订（与 reminder 分支一致），恢复重试复用固化的修订：
+ // type WriteAttempt = ... | { kind: 'subscription'; requestId: string; value: 'subscribed' | 'unsubscribed'; expectedRevision: number }
+ // 发起处：const revision = view?.revision; if (revision === undefined) return
+ // 写入处：expectedRevision: current.kind === 'subscription' ? current.expectedRevision : ...


─── apps/web/src/career/InboxPage.tsx:20-20 ───
[maintainability · low] 推送订阅事实键 'notifications.push' / 'unsubscribed' 是前后端冻结契约字面量（后端 reminder.go 的
ReminderPushFactKey），当前值一致；但 Web 与小程序各自硬编码（miniprogram 的 REMINDER_PUSH_FACT_KEY），api-client
未集中导出，后端改键时各端静默判读错误且无编译期保护。建议在 packages/api-client 的 career 模块导出该常量供各端引用。

- const PUSH_FACT_KEY = 'notifications.push'
+ // packages/api-client/src/career.ts:
+ // export const REMINDER_PUSH_FACT_KEY = 'notifications.push'
+ // export const REMINDER_PUSH_UNSUBSCRIBED = 'unsubscribed'
+ // 页面内：import { REMINDER_PUSH_FACT_KEY } from '../../../../packages/api-client/src/career.ts'


─── apps/web/src/career/ExportDeletionPage.tsx:191-196 ───
[bug · medium] downloadExport 在 anchor.click() 后立即 URL.revokeObjectURL(url)，且 anchor 未 append 到
document；Safari/部分 Firefox 下立即撤销 objectURL 可能中断下载，出现"导出包已下载"提示但文件未落盘——对隐私导出场景是误导性成功反馈。同目录
MaterialPage.saveBlob 已采用 append + 5 秒延迟释放的稳妥实现，建议两处统一为共享的 saveBlob。

    const blob = new Blob([body], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url; anchor.download = `career-export-${exported.exportId}.json`
+   document.body.append(anchor)
    anchor.click()
-   URL.revokeObjectURL(url)
+   anchor.remove()
+   setTimeout(() => URL.revokeObjectURL(url), 5_000) // 与 MaterialPage.saveBlob 一致，或直接复用该共享函数


─── apps/web/src/career/CareerPage.tsx:176-177 ───
[style · low] CareerPage 大量使用静态内联 style 对象（maxWidth:
1040、grid、margin/padding、颜色等），违反团队"避免内联样式（动态样式除外）"约定，且与同目录其余 Career 页面（application.css /
material.css / inbox.css / export-deletion.css 独立样式文件 + 类名）模式不一致，样式无法复用与主题化。建议迁移到 career-page.css
等独立文件。

-  return <main className="wk-page wk-page--std" style={{ maxWidth: 1040, margin: '0 auto', padding: '24px 20px' }}>
-   <header style={{ marginBottom: 20 }}><p style={{ color: '#666', margin: 0 }}>个人求职空间</p><h1 style={{ margin: '6px 0' }}>求职档案</h1><p>只有已确认的档案事实会成为后续求职判断的输入。空间由当前 WeKnora 身份与 Tenant 授权。</p></header>
+  return <main className="wk-page wk-page--std wk-career">
+   <header className="wk-career__header"><p className="wk-career__eyebrow">个人求职空间</p><h1>求职档案</h1>...</header>
+ /* career-page.css：.wk-career { max-width: 1040px; margin: 0 auto; padding: 24px 20px; } .wk-career__header { margin-bottom: 20px; } ... */


─── apps/web/src/career/CareerPage.tsx:48-51 ───
[maintainability · low] load / doAction / refreshSources 等 useCallback 的依赖数组未包含闭包内引用的
invalidateForbidden 与 isCurrent（每次渲染重建）。当前因被引用值内容稳定（desk、setState、ref）未产生实际 bug，但属埋雷式过期闭包：后续往
invalidateForbidden 引入可变状态即会出错，也不满足 exhaustive-deps。建议将 invalidateForbidden、isCurrent 用 useCallback
固化后补入依赖。

-  const load = useCallback(async () => {
-   const epoch = scopeEpoch.current
-   setLoading(true); setError(undefined)
-   desk.activate(scope.userId ?? userId, scope.tenantId)
+  const isCurrent = useCallback((epoch: number) => scopeEpoch.current === epoch, [])
+  const invalidateForbidden = useCallback((parsed?: ...) => { ... }, [desk])
+  const load = useCallback(async () => { ... }, [client, desk, isCurrent, invalidateForbidden, scope.userId, userId, scope.tenantId])


─── apps/web/src/career/application.css:58-60 ───
[maintainability · low] 本文件其余属性均已使用 var(--td-*) 令牌，唯独品牌绿硬编码 #07c05f（多处），而本次新增的
packages/design-tokens/src/tdesign-theme.css 已定义 --td-brand-color: #07c05f（--td-brand-color-4）；focus
态 fallback #0052d9 为 TDesign 默认蓝，与品牌绿体系不一致（令牌缺省时应回退品牌绿系）。警示色 #ffb648/#fff7e8 同理可用 --td-warning-color
系。建议统一替换为令牌引用。

  .wk-application button:focus-visible,
  .wk-application input:focus-visible,
- .wk-application a:focus-visible { outline: 3px solid var(--td-brand-color-focus, #0052d9); outline-offset: 2px; }
+ .wk-application a:focus-visible { outline: 3px solid var(--td-brand-color-focus, var(--td-brand-color, #07c05f)); outline-offset: 2px; }
+ /* 其余 #07c05f 替换为 var(--td-brand-color, #07c05f) */


─── apps/web/src/career/export-deletion.css:43-47 ───
[maintainability · low] 品牌绿硬编码 #07c05f（tdesign-theme.css 已提供 --td-brand-color: #07c05f 可直接引用），且此处用 6
个 !important 压制优先级——.wk-lifecycle__export 与 .wk-lifecycle__actions button 同为单类选择器，仅靠源码顺序叠加即可命中，无需
!important；!important 会连带压制后续主题化与覆盖。建议改用 var(--td-brand-color) 并去掉 !important（必要时提升选择器特异度）。

- .wk-lifecycle__export, .wk-lifecycle__download {
-   border-color: #07c05f !important;
-   background: #07c05f !important;
-   color: #fff !important;
+ .wk-lifecycle__actions .wk-lifecycle__export,
+ .wk-lifecycle__actions .wk-lifecycle__download {
+   border-color: var(--td-brand-color, #07c05f);
+   background: var(--td-brand-color, #07c05f);
+   color: #fff;
  }


─── apps/web/src/career/inbox.css:46-46 ───
[maintainability · low] 品牌绿硬编码 #07c05f 多处出现（含 hover 边框/文字色），本次新增的 tdesign-theme.css 已定义
--td-brand-color: #07c05f 与 --td-brand-color-hover/--td-brand-color-active
渐变（#08dd6e/#06b04d），design-tokens 亦有 --wk-color-brand 令牌；硬编码会导致后续主题调整需逐处改动且 hover 色与令牌渐变脱节。建议替换为
var(--td-brand-color) / var(--td-brand-color-hover)。

- .wk-inbox__submit { border-color: #07c05f; background: #07c05f; color: #fff; }
+ .wk-inbox__submit { border-color: var(--td-brand-color, #07c05f); background: var(--td-brand-color, #07c05f); color: #fff; }
+ .wk-inbox button:hover:not(:disabled) { border-color: var(--td-brand-color-hover, #08dd6e); color: var(--td-brand-color, #07c05f); }


─── apps/web/src/career/material.css:133-137 ───
[maintainability · low] 品牌绿硬编码 #07c05f（tdesign-theme.css 已定义 --td-brand-color: #07c05f 可直接引用），focus
fallback #0052d9 为 TDesign 默认蓝、与本页品牌绿按钮体系不一致；待审阅警示色 #ffb648/#fff7e8/#b45309 同样未走 --td-warning-color
令牌体系。建议统一替换为令牌引用，避免色值漂移。

  .wk-material__confirm:not(:disabled) {
-   border-color: #07c05f;
-   background: #07c05f;
+   border-color: var(--td-brand-color, #07c05f);
+   background: var(--td-brand-color, #07c05f);
    color: #fff;
  }
+ /* focus 态 fallback 同步改为 var(--td-brand-color, #07c05f) 系 */


─── internal/modules/career/source_import.go:663-666 ───
[bug · high] 接管分支的 UPDATE 存在丢失更新（lost update）竞态：WHERE 仅按 (tenant_id, user_id, request_id) 定位，既不校验
body 仍是本事务先前读到的过期 claim，也不检查 RowsAffected。在 Postgres/MySQL READ COMMITTED 下可复现：原持有者 A 的 claim
租约（60s）因进程停顿过期后才提交 commitImportURLObservation 的终态 receipt（该提交只校验 claim token，不校验租约是否仍有效）；此时重试方 B 的
claim 事务先 SELECT 到过期 claim，其 UPDATE 阻塞在 A 的行锁上，A 提交后该 UPDATE 重新匹配 WHERE 并把刚落库的终态 receipt 覆盖为 B 的
claim body。B 随后提交时 body 中正是自己的 token，于是同一 requestID 会创建第二个
opportunity/observation/snapshot，且首个已返回给客户端的 receipt 被销毁——直接破坏"同一 request ID
幂等恢复"的冻结契约。对比本模块其余全部状态迁移（PersistUploadResource、markCatalogCleanupPending、finishSource、ClearSourceRes
ource 及 profile_intake.go:187 的租约接管）都采用条件更新 + RowsAffected==1 守卫，此分支是唯一例外。建议对 body 做 CAS 并在
RowsAffected!=1 时降级为 importClaimInFlight：

- 			if err = tx.Model(&opportunityReceipt{}).Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).Update("body", takeover).Error; err != nil {
- 				return err
+ 			res := tx.Model(&opportunityReceipt{}).Where("tenant_id=? AND user_id=? AND request_id=? AND body=?", s.TenantID, s.UserID, requestID, existing.Body).Update("body", takeover)
+ 			if res.Error != nil {
+ 				return res.Error
+ 			}
+ 			if res.RowsAffected != 1 {
+ 				// The body changed since this transaction read it (a terminal
+ 				// receipt or another takeover committed); never overwrite it.
+ 				outcome.state = importClaimInFlight
+ 				return nil
  			}
  			outcome.state, outcome.token = importClaimProceed, token


─── packages/design-tokens/src/tdesign-theme.css:2-2 ───
[bug · low] 平移的 16 个 --td-line-height-* 令牌中，唯独 --td-line-height-title-large 引用的是
--td-font-size-title-medium（16px）而非 --td-font-size-title-large（20px），按文件内其余 15 个令牌「同名字号 +
common」的一致模式判断，这是 Vue 源 frontend/src/assets/theme/theme.css 的复制笔误被原样带入（light 块与 dark
块两处相同）：title-large（20px）文本将得到 24px 行高而非 28px，影响所有消费 --td-font-title-large 的 TDesign 组件。本文件已成为
packages/ui 退役后的 token 事实源，建议与 Vue 端同步改为
var(--td-font-size-title-large)；若刻意保留像素对齐，请加注释标注为已知偏差，避免后续维护者误判。

- :root:root[theme-mode="dark"]{
+ --td-line-height-title-large: calc(    var(--td-font-size-title-large) + var(--td-line-height-common)  );


─── apps/web/src/career/RulePage.tsx:195-197 ───
[bug · high] unknown 阶段未锁定写入入口：submit 与表单控件（textarea/input[type=number]/radio/提交按钮）都只按 phase ===
'busy' 锁定（busy = phase === 'busy'）。当上一次保存结果未知（phase === 'unknown'）时，用户仍可修改内容并点击"保存规则"，此时会用 makeId()
另起一个全新
requestId，违背本目录"未知写入必须用同一请求编号恢复"的协议（未知恢复卡片与表单同时可用，用户很容易直接走表单）。同批其他页面（ProgressPage/MaterialPage/Inbox
Page/ExportDeletionPage 等）均为 composeBlocked = busy || unknown 的实现，此处是唯一缺口。建议将锁定条件扩展为 phase ===
'busy' || phase === 'unknown' 并应用到表单与提交按钮。

+  const composeLocked = phase === 'busy' || phase === 'unknown'
   const submit = (event: FormEvent): void => {
    event.preventDefault()
-   if (phase === 'busy' || !draft.query.trim() || !intervalValid || revision === undefined || enableRequiresEstimate) return
+   if (composeLocked || !draft.query.trim() || !intervalValid || revision === undefined || enableRequiresEstimate) return
+   // 表单 textarea/input/radio 与保存按钮的 disabled 也应改用 composeLocked


─── apps/web/src/career/RulePage.tsx:97-101 ───
[bug · high] load() 中 getRule 的非 forbidden/not_found 失败（瞬时 5xx、网络抖动）被静默吞掉：ruleView 保持
undefined、无任何提示，existingRuleId 随之为空，下一次保存走"无 ruleId 新建"路径。后端 SetRule 对空 ruleId 是直接新建（search_rule.go
仅在 ruleId 非空时按 id 更新，没有单用户单规则约束），一次瞬时读失败就可能让用户在不知情时创建第二条规则，两条 enabled
规则会各自触发计费搜索。建议在该分支给出明确提示（"已存规则暂时读取失败，此时保存会创建新规则"）或在读取成功前禁用保存，与上面 unknown 未锁定的问题合并即构成重复规则/重复计费的完整链路。

-    } catch (cause) {
-     if (!scopeController.isCurrent(requestScope.scope)) return
-     const parsed = errorDetails(cause)
-     if (parsed.code === 'forbidden') { clearForScopeChange('当前空间不可访问，已清除持续找岗状态。'); return }
      if (parsed.code === 'not_found') {
+      // ...existing not_found handling...
+     }
+     setNotice('已保存的规则暂时读取失败；此时直接保存会创建一条新规则，请先重试读取。')


─── apps/web/src/career/SearchPage.tsx:252-252 ───
[bug · medium] 导入写入没有恢复机制且每次点击都换新 requestId：importUrl 的后端幂等键是 requestId（source_import.go 的
fingerprint = ["import_url", requestId, url]，按 tenant+user+request_id 查重），同一 URL
换新编号会创建第二条岗位证据记录；而这里任何失败（包括 outcome_unknown 与网关超时这类结果不确定的失败）都统一显示为"导入未成功：…"，用户被误导重试，再点一次即以新
requestId 重发。api-client 也没有 importUrl 的回执查询端点，与全目录"未知写入用同一 request ID 恢复"的约定不一致。建议按 resultId 生成并缓存一次
requestId（存入 imports 状态），重试复用同一编号；对结果不确定的失败显示"导入结果暂时未知"并提供用原编号重试的入口。

-    const imported = await client.career.importUrl({ requestId: makeId(), url: row.link }, requestScope.signal)
+    const requestId = imports[row.resultId]?.requestId ?? makeId()
+    const imported = await client.career.importUrl({ requestId, url: row.link }, requestScope.signal)
+    // 并在 setImports 时记录该 requestId，失败后重试复用同一编号


─── apps/web/src/career/PreparationPage.tsx:250-252 ───
[bug · medium] 写入成功后的回读失败会被误报为写入失败/未知：editMaterial 成功后紧接着回读 material，若该回读失败（网络错误、5xx）会落入与写入共用的
catch——确定性错误码分支显示"修订未保存：…"（写入实际已成功，明确误报失败），不确定分支显示"修订结果暂时未知…可稍后在材料区用原请求编号查询回执"（回读按 materialId
查询，并不消费请求编号，提示本身不成立）。用户会误以为内容丢失并重复保存。建议把回读包进独立的
try/catch：失败时仍按已保存处理（如提示"修订已保存；回显暂时失败，可稍后刷新查看"），不复用写入错误文案。

-    const view = await client.career.material(current.materialId, requestScope.signal)
+    let view: MaterialView
+    try {
+     view = await client.career.material(current.materialId, requestScope.signal)
+    } catch {
+     if (scopeController.isCurrent(requestScope.scope)) {
+      setReviseBusy(false); setEditing(undefined)
+      setReviseMessage('修订已保存；保存内容回显暂时失败，可稍后刷新查看。')
+     }
+     return
+    }
     if (!scopeController.isCurrent(requestScope.scope)) return
-    if (view.materialId !== current.materialId) throw new ReceiptMismatchError('材料视图与本次修订不匹配')


─── apps/web/src/career/PreparationPage.tsx:28-30 ───
[maintainability · medium] 协议助手在本次新增的 5 个页面中逐份复制且已出现分叉：isUncertainWrite/isUncertainOutcome（11
份）、errorDetails（各页一份）、newRequestId/makeId、ReceiptMismatchError（5 份）、formatCheckTime（2
份）。实现细节已经开始不一致：本文件的 errorDetails 会把 currentRevision 从 string|number 收敛为
number，ProgressPage/SubmissionPage 直接透传；lookupReceipt 对 invalid_request
的处理只有本文件有，ProgressPage/SubmissionPage 缺失。协议演进（新增错误码、调整"结果不确定"判定）时极易改漏其中一份。建议抽取到共享模块（如
src/career/protocol.ts）统一导出，各页面仅保留各自的业务文案分支。

- function isUncertainWrite(cause: unknown): boolean {
-  const error = errorDetails(cause)
-  if (error.code === 'TIMEOUT' || error.code === 'outcome_unknown') return true
+ // 新建 src/career/protocol.ts 统一导出：
+ // export class ReceiptMismatchError extends Error {}
+ // export function errorDetails(cause: unknown): TypedError { … }
+ // export function isUncertainWrite(cause: unknown): boolean { … }
+ // export const newRequestId = (): string => …
+ // 各页面改为 import { ReceiptMismatchError, errorDetails, isUncertainWrite, newRequestId } from './protocol.ts'


─── apps/web/src/career/PreparationPage.tsx:4-4 ───
[maintainability · medium] 类型导入用四层相对路径穿透包源码边界：同一文件已用 '@weknora/api-client' 别名引入 WeKnoraClient，类型却直连
'../../../../packages/api-client/src/career.ts'；packages/api-client/src/index.ts 目前只导出了
CareerRequest 等少量类型，未导出这些页面类型。该写法对目录结构调整（文件改名/移动、包内拆分）极其脆弱，且本次变更中有 12+ 文件同此写法。建议在 index.ts 补充 career
类型导出（export type * from './career.ts' 或具名导出），再统一改为别名导入（同样适用于
ProgressPage/RulePage/SearchPage/SubmissionPage/UsagePanel）。另：SearchPage.tsx 第 5-6
行从同一模块分两行导入类型，可合并为一行。

- import type { MaterialBody, PreparationFocus, PreparationReceipt } from '../../../../packages/api-client/src/career.ts'
+ // packages/api-client/src/index.ts 增加：
+ // export type * from './career.ts';
+ // 页面侧改为：
+ import type { MaterialBody, PreparationFocus, PreparationReceipt } from '@weknora/api-client'


─── apps/web/src/career/preparation.css:102-108 ───
[bug · low] 两处问题：(1) PreparationPage.tsx 引用的 .wk-preparation__body（第 60
行）、.wk-preparation__revised（第 333 行）、.wk-preparation__edit-claims（第 315
行）在本文件均无样式定义，对应容器（草稿正文区、修订后草稿区、编辑区主张列表）没有任何样式；(2) 此处使用的 --td-bg-color-secondary-container（带连字符）在
design-tokens 主题包中不存在——主题包定义的是 --td-bg-color-secondarycontainer（无连字符，opportunity.css/progress.css
使用的即是无连字符版本），该 token 会恒走回退值 #f5f5f5。建议补齐缺失类并统一 token 拼写。

- .wk-preparation__sources {
-   margin: 10px 0 0;
-   padding: 10px 12px;
-   border: 1px dashed var(--td-component-border, #dcdcdc);
-   border-radius: 8px;
-   background: var(--td-bg-color-secondary-container, #f5f5f5);
- }
+   background: var(--td-bg-color-secondarycontainer, #f5f5f5);
+ /* 并补充缺失定义：
+ .wk-preparation__body { … }
+ .wk-preparation__edit-claims { … }
+ .wk-preparation__revised { … } */


─── apps/web/src/career/rule.css:1-3 ───
[maintainability · low] RulePage.tsx 第 243 行的 Card 使用了
className="wk-career-rule__compose"，但本文件中没有该类的样式定义（同为本次新增的 preparation.css 也存在同类遗漏）。缺失定义意味着这块表单卡片只有
Card 默认样式，与文件内其他命名块（__live、__runs 等）的定制程度不一致；若确属有意依赖 Card 默认样式，建议移除该 className 以免误导后续维护者去寻找不存在的样式。



─── apps/web/src/career/PreparationPage.tsx:42-42 ───
[style · low] 状态文案使用嵌套三元（failed ? (item.status === 'failed' ? … : …) : …），违反项目"禁止嵌套三元"的约定；下方
readState 的 loading→error→ready 渲染链（readState === 'loading' ? … : readState === 'error' ? … :
…，各页同构）也属链式嵌套三元。另外编辑表单中 body.sections[sectionIndex]!.claims[claimIndex]! 连用非空断言，虽然 cloneBody
后索引必然存在，仍建议先解构到局部常量再判空。建议抽一个状态文案帮助函数（接收 item 返回字符串）消解嵌套三元，非空断言改为显式取值判空。

-    <span>{failed ? (item.status === 'failed' ? '生成失败' : '生成中（可恢复）') : '草稿（可审阅、可修订）'}</span>
+  const preparationStatusText = (item: PreparationReceipt): string => {
+   if (item.status === 'draft') return '草稿（可审阅、可修订）'
+   if (item.status === 'failed') return '生成失败'
+   return '生成中（可恢复）'
+  }
+ // 渲染处：<span>{preparationStatusText(item)}</span>


─── apps/web/src/career/SubmissionPage.tsx:168-169 ───
[maintainability · low] 写入负载被声明为 Record<string, unknown>，再在调用处 as 强转为 recordSubmission
的参数类型：字段拼写错误或类型不符在编译期都不会被发现（例如第二个分支的 materialId 来自 exports?.find(…)?.materialId，可能为 undefined
也会被静默序列化进请求体，仅靠服务端拒绝兜底）。同时 occurredAtFromInput(occurredAt) 在同一表达式中被重复调用两次（两次日期解析）。建议按 versionUnknown
建立可辨识联合类型（{ versionUnknown: true } | { versionUnknown: false; materialId: string; exportId: string
}），并先 const occurredAtIso = occurredAtFromInput(occurredAt) 一次求值后展开。

-    const input: Record<string, unknown> = versionChoice === UNKNOWN_VERSION_CHOICE
-     ? { requestId, applicationId, channel, versionUnknown: true, expectedRevision: revision, ...(occurredAtFromInput(occurredAt) ? { occurredAt: occurredAtFromInput(occurredAt) } : {}), ...(note.trim() ? { note: note.trim() } : {}) }
+    const occurredAtIso = occurredAtFromInput(occurredAt)
+    const input: RecordSubmissionInput = versionChoice === UNKNOWN_VERSION_CHOICE
+     ? { requestId, applicationId, channel, versionUnknown: true, expectedRevision: revision, ...(occurredAtIso ? { occurredAt: occurredAtIso } : {}), ...(note.trim() ? { note: note.trim() } : {}) }
+     : { requestId, applicationId, channel, versionUnknown: false, materialId: exports?.find((receipt) => receipt.exportId === versionChoice)?.materialId ?? '', exportId: versionChoice, expectedRevision: revision, ...(occurredAtIso ? { occurredAt: occurredAtIso } : {}), ...(note.trim() ? { note: note.trim() } : {}) }


─── package.json:13-13 ───
[test · medium] 新增共享包 @weknora/career-core 的测试未被聚合测试命令覆盖：仓库中实际存在
packages/career-core/src/contracts.test.ts 与 packages/career-core/src/desk.test.ts（实施计划
2026-09-24-issue-140-implementation.md 也明确要求运行 `tsx --test
packages/career-core/src/*.test.ts`），但本行新增的 test:shared 通配列表没有任何 career-core 模式，根 package.json
其他脚本也没有。这意味着求职模块的合同解码器与 CareerDesk 状态机（含 outcome_unknown 恢复、scope 切换失效等关键红线行为）在 CI
的共享测试入口中永远不会被执行，回归保护静默缺失。建议在 test:shared 中补充 `packages/career-core/src/*.test.ts`。

-     "test:shared": "tsx --test packages/contracts/test/*.test.ts packages/contracts/test/mobile-*.test.ts packages/contracts/src/craft/*.test.ts packages/domain/src/*.test.ts packages/domain/src/auth/*.test.ts packages/domain/src/access/*.test.ts packages/domain/src/craft/*.test.ts packages/domain/src/knowledge/*.test.ts packages/domain/src/mobile/*.test.ts packages/domain/src/wiki/*.test.ts packages/domain/src/chat/*.test.ts packages/domain/src/sandbox/*.test.ts packages/domain/src/settings/*.test.ts packages/api-client/src/*.test.ts packages/api-client/src/analytics/*.test.ts packages/api-client/src/auth/*.test.ts packages/api-client/src/craft/*.test.ts packages/api-client/src/identity/*.test.ts packages/api-client/src/knowledge/*.test.ts packages/api-client/src/wiki/*.test.ts packages/api-client/src/chat/*.test.ts packages/api-client/src/sandbox/*.test.ts packages/api-client/src/mobile/*.test.ts packages/api-client/src/settings/*.test.ts packages/api-client/src/embed/*.test.ts packages/api-client/src/transport/*.test.ts packages/core/src/craft/*.test.ts packages/design-tokens/src/*.test.ts packages/i18n/test/*.test.ts packages/i18n/test/*.test.tsx packages/views/src/chat/*.test.ts packages/views/src/chat/*.test.tsx packages/views/src/craft/*.test.ts packages/views/src/craft/*.test.tsx packages/views/src/guides/*.test.ts packages/views/src/guides/*.test.tsx packages/views/src/settings/*.test.ts packages/views/src/embed/*.test.ts packages/views/src/integrations/*.test.ts packages/views/src/integrations/*.test.tsx",
+ "test:shared": "tsx --test packages/contracts/test/*.test.ts ... packages/design-tokens/src/*.test.ts packages/career-core/src/*.test.ts packages/i18n/test/*.test.ts ...",


─── packages/api-client/src/career.ts:1339-1339 ───
[maintainability · low] 死条件：`operation.trim() !== operation` 永远不会为真。由于 `||` 短路，该子句只在 `operation ===
'search_once'` 时才会被求值，而字面量 'search_once' 的 trim()
结果恒等于自身，因此第二段判断是永假死代码，给读者一种"此处会处理空白字符操作名"的错误暗示。直接删掉第二段即可。

-    if (operation !== 'search_once' || operation.trim() !== operation) throw new TypeError('usage operation must be search_once')
+    if (operation !== 'search_once') throw new TypeError('usage operation must be search_once')


─── packages/api-client/src/career.ts:925-926 ───
[bug · low] 解码器比其注释声明的校验更宽松：注释写明"whatever it does carry must still be a well-formed digest"，但当前第二个
if 只在 opportunityId/snapshotId 存在时才校验 snapshotSha256。对一个 failed/generating 行，若后端负载只携带
`snapshotSha256: "garbage"`（不带两个 ID），两个分支都不会抛错，且返回值处 `typeof snapshotRecord.snapshotSha256 ===
'string'` 会把这个畸形摘要原样透传到 UI，违背本文件"发明负载在到达 UI 前被拒绝"的承诺。建议把"携带 sha 则必须为合法 64 位十六进制"作为独立无条件校验。

-  if (!draft && (validIdentifier(snapshotRecord.opportunityId) || validIdentifier(snapshotRecord.snapshotId))
+  if ((snapshotRecord.snapshotSha256 !== undefined || validIdentifier(snapshotRecord.opportunityId) || validIdentifier(snapshotRecord.snapshotId))
    && (typeof snapshotRecord.snapshotSha256 !== 'string' || !sha256Hex.test(snapshotRecord.snapshotSha256))) throw new TypeError('invalid preparation sources')


─── packages/api-client/src/career.ts:907-910 ───
[maintainability · low] 重复校验逻辑：`record.anchor` 与 `sourcesRecord.submittedVersion`
是同一形状（PreparationAnchor，5 个字段的校验一字不差地写了两遍）。按冻结合同 anchor
就是提交版本本身，两处规则必须同步演化；任一侧后续收紧/放宽都会造成同一回执内两个字段校验不一致。建议提取 `decodePreparationAnchor(value: unknown,
message: string): PreparationAnchor` 供两处复用。

-  const anchorRecord = decodeRecord(record.anchor, 'invalid preparation anchor')
-  if (!validIdentifier(anchorRecord.submissionId) || !validIdentifier(anchorRecord.materialId) || !validIdentifier(anchorRecord.exportId)
-   || !validPositiveVersion(anchorRecord.version)
-   || typeof anchorRecord.contentDigest !== 'string' || !sha256Hex.test(anchorRecord.contentDigest)) throw new TypeError('invalid preparation anchor')
+  function decodePreparationAnchor(value: unknown, message: string): PreparationAnchor {
+   const record = decodeRecord(value, message)
+   if (!validIdentifier(record.submissionId) || !validIdentifier(record.materialId) || !validIdentifier(record.exportId)
+    || !validPositiveVersion(record.version)
+    || typeof record.contentDigest !== 'string' || !sha256Hex.test(record.contentDigest)) throw new TypeError(message)
+   return { submissionId: record.submissionId, materialId: record.materialId, exportId: record.exportId, version: record.version as number, contentDigest: record.contentDigest }
+  }
+  // 调用处：const anchor = decodePreparationAnchor(record.anchor, 'invalid preparation anchor')
+  //        submittedVersion: decodePreparationAnchor(sourcesRecord.submittedVersion, 'invalid preparation sources')


─── packages/api-client/src/career.ts:1455-1455 ───
[bug · low] 字节上限用字符数校验：注释声明 maxSubmissionNoteBytes 镜像后端 `maxSubmissionNoteBytes`（Go 侧按字节计），但 JS
`input.note.length` 统计的是 UTF-16 码元数。中文备注 2047 个字符约 6141 字节，已超 4096
字节上限却能通过此客户端前置校验，最终由后端拒绝并返回不友好的错误；前置校验对多字节文本系统性失真。建议按 UTF-8 字节数比较（与文件其他处一致的 TextEncoder
即可），或改为保守的码元×阈值收紧。

-    if (input.note !== undefined && input.note.length > maxSubmissionNoteBytes) throw new TypeError('submission note must not exceed 4096 bytes')
+    if (input.note !== undefined && new TextEncoder().encode(input.note).length > maxSubmissionNoteBytes) throw new TypeError('submission note must not exceed 4096 bytes')


─── apps/web/src/career/reconciliation.tsx:75-75 ───
[bug · high] 空间切换（以及 opportunityId 切换）时只清除了 status/history，但
receipt、attempt、reconcilePhase、reconcileMessage、candidateDraft 等对账状态未清理。新空间数据加载成功（phase 回到
ready）后，旧空间的合并回执（含岗位编号、公司、地点、批次等身份证据）会继续渲染在新空间中，违反「空间切换后旧响应失效」红线；更严重的是旧 attempt 的
targetId/candidateId 被保留，「使用原请求编号重试」会向新空间重放旧空间的对账请求。建议在 clearForScopeChange 及 opportunityId
变化时统一重置这组状态，并在 reconciliation.test.tsx 补充空间切换后无残留回执的用例。

-  const clearForScopeChange = () => { active = false; setStatus(undefined); setHistory(undefined); setPhase('scope-changed') }
+ const clearForScopeChange = () => {
+   active = false
+   setStatus(undefined); setHistory(undefined)
+   setReceipt(undefined); setAttempt(undefined); setCandidateDraft('')
+   setReconcilePhase('idle'); setReconcileMessage('')
+   setOldEvidence(undefined); setNewEvidence(undefined); setDiffPhase('idle')
+   setPhase('scope-changed')
+ }


─── apps/web/src/career/reconciliation.tsx:137-137 ───
[bug · medium] `opportunity_not_found` 是死分支：后端 internal/modules/career/handler.go 的 writeError 将
ErrOpportunityNotFound 统一映射为 `not_found`（404），全仓库没有任何地方产生
`opportunity_notNotFound`/`opportunity_not_found` 这个错误码。实际后果：候选记录不存在时返回
code=`not_found`，不会命中此分支，落入兜底文案「对账未完成，请检查记录编号后重试」，服务端的具体 message 丢失。应与后端契约对齐改用 `not_found`。

-    if (parsed.code === 'invalid_request' || parsed.code === 'opportunity_not_found') { setReconcilePhase('error'); setReconcileMessage(`对账请求未被接受：${parsed.message}`); return }
+    if (parsed.code === 'invalid_request' || parsed.code === 'not_found') { setReconcilePhase('error'); setReconcileMessage(`对账请求未被接受：${parsed.message}`); return }


─── apps/web/src/career/reconciliation.tsx:29-29 ───
[bug · low] `network_error` 同样是死字面量：api-client 的 client.ts 在传输失败时对非超时/非取消错误原样抛出（无 code 的
TypeError），不会包装成 code='network_error' 的 ApiError；该场景已由 `code === undefined`
兜底覆盖。保留这个永不成立的判断会误导后续维护者以为存在该契约，建议删除或与真实错误码对齐。

-  return parsed.code === 'outcome_unknown' || parsed.code === 'TIMEOUT' || parsed.code === 'network_error' || parsed.code === undefined
+  return parsed.code === 'outcome_unknown' || parsed.code === 'TIMEOUT' || parsed.code === undefined


─── apps/web/src/career/reconciliation.tsx:213-213 ───
[bug · medium] 对比区三元链缺少 idle 分支：当用户将旧/新快照选为同一条（snapshotId 相同）时，diff useEffect 会把 diffPhase 置为
'idle'，但渲染链只有 ready 和 error 两个分支，落入兜底文案「正在读取两个固定快照…」永久显示——把无效选择伪装成加载态，误导用户无限等待。应显式处理 idle 并提示选择不同快照。

-    </div> : diffPhase === 'error' ? <p role="status">快照对比暂时无法读取，可稍后重试；两个快照本身仍可分别打开。</p> : <p role="status">正在读取两个固定快照…</p>}
+    </div> : diffPhase === 'error' ? <p role="status">快照对比暂时无法读取，可稍后重试；两个快照本身仍可分别打开。</p> : diffPhase === 'idle' ? <p role="status">请选择两个不同的快照进行对比。</p> : <p role="status">正在读取两个固定快照…</p>}


─── apps/web/src/career/reconciliation.tsx:6-6 ───
[maintainability · medium] 与 OpportunityPage.tsx 构成模块级循环依赖：OpportunityPage.tsx 第 7 行导入本文件的
OpportunityStatusPanel，本文件又导入其 opportunityEvidencePath（ApplicationPage/InboxPage
也导入该函数）。目前因函数声明提升可正常工作，但任一侧改为 const 箭头函数导出就会在运行时得到 undefined 并崩溃。建议把 opportunityEvidencePath
抽到独立的路径工具模块（如 career/paths.ts），三方统一从该模块导入。

- import { opportunityEvidencePath } from './OpportunityPage.tsx'
+ import { opportunityEvidencePath } from './paths.ts'


─── apps/web/src/career/reconciliation.tsx:271-271 ───
[style · low] 这里存在 4 分支嵌套三元链（loading/error/forbidden/scope-changed），OpportunityStatusPanel 的 phase
文案与 diff 三态渲染也是同样写法，违反「禁止嵌套三元表达式」规则，且新增状态时极易漏改分支（对比区缺 idle 分支的问题即为例证）。建议抽取 phase→文案的映射对象或子组件。

-   {phase !== 'ready' || !coverage ? <p role={phase === 'error' ? 'alert' : 'status'}>{phase === 'loading' ? '正在读取来源覆盖…' : phase === 'error' ? '来源覆盖暂时无法读取。' : phase === 'forbidden' ? '当前空间不可读取来源覆盖。' : '空间已切换，已清除来源覆盖。'}{phase === 'error' ? <button type="button" onClick={retry}>重试</button> : null}</p> : <>
+   {phase !== 'ready' || !coverage ? <p role={phase === 'error' ? 'alert' : 'status'}>{coveragePhaseText[phase]}{phase === 'error' ? <button type="button" onClick={retry}>重试</button> : null}</p> : <>
+ // 组件外定义：
+ // const coveragePhaseText: Record<Phase, string> = { loading: '正在读取来源覆盖…', error: '来源覆盖暂时无法读取。', forbidden: '当前空间不可读取来源覆盖。', 'scope-changed': '空间已切换，已清除来源覆盖。', ready: '' }


─── apps/web/src/career/submission.css:55-57 ───
[style · low] `--td-text-color-warning` 令牌在
packages/design-tokens/src/tdesign-theme.css（及全仓库）均未定义，主题中警告文字的正确令牌是 `--td-warning-color-5`（浅色主题实际值
#ed7b2f）。因此 `.wk-submission__record-version--unknown` 与第 112 行 `.wk-submission__claim--needs-review`
两处会永远使用 fallback #e37318，不随主题变化，且与 reconciliation.css 中使用的 `--td-warning-color-5` 命名体系不一致。

  .wk-submission__record-version--unknown {
-   color: var(--td-text-color-warning, #e37318);
+   color: var(--td-warning-color-5, #ed7b2f);
  }


─── internal/modules/career/career_export.go:168-178 ───
[bug · medium] 导出/删除覆盖面不对称：DeleteCareer 的 careerPurgeTables 会清空 career_preparations（求职信/面试稿正文存于
preparationRecord.ReceiptBody）、career_search_rules / career_search_rule_runs、career_reminders 等表，且
CareerDeletionBoundary 把这些分区明确列为"将被删除的空间内数据"；但 buildCareerExportArchive 只导出
profile/factHistory/opportunities/applications/materials/submissions
六类。用户按"先导出、后删除"闭环操作后，准备稿与周期搜索规则被永久销毁且没有任何导出路径，与 ExportCareer 注释声明的 "one complete export package of
the whole Career space" 契约矛盾。建议把 preparations（至少 ReceiptBody/状态/锚点）以及 search_rules、reminders
补入归档结构，或在归档与删除边界中显式披露这些分区不随导出携带。

- // CareerExportArchive is the frozen structure of one complete export
- // package: profile, original job snapshots, application events, material
- // versions, and submission records.
  type CareerExportArchive struct {
- 	Profile       View                      `json:"profile"`
- 	FactHistory   []Fact                    `json:"factHistory"`
- 	Opportunities []CareerExportOpportunity `json:"opportunities"`
+ 	Profile       View                       `json:"profile"`
+ 	FactHistory   []Fact                     `json:"factHistory"`
+ 	Opportunities []CareerExportOpportunity  `json:"opportunities"`
  	Applications  []CareerExportApplication `json:"applications"`
- 	Materials     []CareerExportMaterial    `json:"materials"`
- 	Submissions   []CareerExportSubmission  `json:"submissions"`
+ 	Materials     []CareerExportMaterial     `json:"materials"`
+ 	Preparations  []CareerExportPreparation  `json:"preparations"`
+ 	Submissions   []CareerExportSubmission   `json:"submissions"`
+ 	SearchRules   []CareerExportSearchRule   `json:"searchRules"`
+ 	Reminders     []CareerExportReminder     `json:"reminders"`
  }


─── internal/modules/career/reminder.go:292-297 ───
[bug · low] 重复推送：同一 request ID 的并发/超时重试请求会走到这个事务内重放分支——前置 replayReminderReceipt 未命中（胜者尚未提交），而本事务的
FOR UPDATE 等到胜者提交后读到已存储回执并以 txErr == nil 返回。该回执是首次创建时持久化的（Deduplicated=false），于是 SetReminder 的 txErr
== nil 分支会再次调用 remindAfterCommit，向用户重复推送。这违反了本文件冻结的不变量 "Push happens only after the commit and only
for a newly created todo"。建议在 outcome 中标记本次为重放，跳过 remindAfterCommit。

+ type reminderWriteOutcome struct {
+ 	receipt    ReminderReceipt
+ 	subscribed bool
+ 	replayed   bool
+ }
+ 
+ ...
  		if e == nil {
  			if stored.Fingerprint != fingerprint {
  				return ErrIdempotencyConflict
  			}
+ 			outcome.replayed = true
  			return decodeReminderReceipt(stored.Body, &outcome.receipt)
+ 		}
+ 
+ // SetReminder 内：
+ 		if txErr == nil {
+ 			if !outcome.replayed {
+ 				o.remindAfterCommit(ctx, s, &outcome.receipt, outcome.subscribed)
+ 			}
+ 			return outcome.receipt, nil
  		}


─── internal/modules/career/handler.go:218-230 ───
[security · medium] Act 是 Career 组里唯一没有 http.MaxBytesReader 的 POST 写端点：ShouldBindJSON
会把任意大小的请求体完整读入内存（router/middleware 层无全局 body 限制），且 office 层 propose/confirm 对 Key/Value 没有任何长度校验（对比
completeIntake 对字段值有 20000 字节上限），Value 会被原样持久化到 fact/factVersion/proposal 行。任何认证用户（个人空间 owner）都可用超大
JSON body 造成内存峰值和无上限存储增长。建议与同文件其他写端点对齐，加上 16KB 左右的 MaxBytesReader（并处理 *http.MaxBytesError → 413），或在
office 层补充 key/value 长度上限。

+ func (h *Handler) Act(c *gin.Context) {
+ 	ctx, ok := h.scope(c, false)
+ 	if !ok {
+ 		return
+ 	}
+ 	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
  	var req struct {
  		Action           string `json:"action"`
  		ProposalID       string `json:"proposalId"`
  		Key              string `json:"key"`
  		Value            string `json:"value"`
  		RequestID        string `json:"requestId"`
  		ExpectedRevision uint64 `json:"expectedRevision"`
  		Source           Source `json:"source"`
  	}
  	if e := c.ShouldBindJSON(&req); e != nil {
+ 		var maxBytesError *http.MaxBytesError
+ 		if errors.As(e, &maxBytesError) {
+ 			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"code": "request_too_large", "message": "act request is too large"}})
+ 			return
+ 		}
  		writeError(c, ErrInvalidRequest)
  		return
  	}


─── internal/modules/career/handler.go:1612-1618 ───
[performance · medium] 与已确认发现（GET /sources 中 cleanupCatalogCandidates 无界）同根因的第二处实例：Upload 路径同样在请求
ctx 上逐个 source 调用 cleanupCatalogCandidates（内部 h.upload.Release 使用调用方 ctx、无超时上限，且为 N+1
查询+外部删除），一旦存储后端停滞，上传请求会随客户端一直挂起。同文件的 reconcileStaleSourcesExcept 对同类 Release 特意构造了 5s 超时的 detached
context，这里也应对齐：套用有界 context（或复用 reconcileStaleSourcesExcept 的模式），并考虑与 Sources
一起收敛到统一的有界清理入口，避免修复时遗漏此处。

  	if sources, listErr := h.office.ListSources(ctx); listErr == nil {
+ 		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
+ 		defer cancel()
  		for _, source := range sources {
- 			if cleanupErr := h.cleanupCatalogCandidates(ctx, source.ID); cleanupErr != nil {
+ 			if cleanupErr := h.cleanupCatalogCandidates(cleanupCtx, source.ID); cleanupErr != nil {
  				slog.Warn("career catalog cleanup pending", "source_id", source.ID)
  			}
  		}
  	}


─── internal/modules/career/application.go:313-317 ───
[bug · medium] CreateApplication 缺少模块内其他写路径统一的 SQLite busy 处理：两个并发申请创建时，事务在 tx.Create 处报
SQLITE_BUSY/LOCKED（isSQLiteBusy 为真，错误消息 "database is locked" 也会命中 isReceiptRaceError），随后
replayApplication 与 occupied 查询都查不到行，最终落到 `return ApplicationReceipt{}, err`，把底层 busy 错误以 500
internal + "database is locked" 原始消息返回给客户端——既暴露内部细节，也不符合本模块 "未知创建结果用同一 request ID 恢复" 的 504
outcome_unknown 契约。对比 submission.RecordSubmission / progress / preparation / reminder /
office.mutate 都有 busy 重试 + OutcomeUnknown 收敛（mutate 还专门用 25ms busy_timeout 控制等待）。此场景事务已回滚、行未创建，返回
OutcomeUnknown 让客户端原样重试是安全的（request ID 幂等重放）。

- 		if ctx.Err() != nil {
+ 		if ctx.Err() != nil || isSQLiteBusy(err) {
  			return ApplicationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
  		}
  		return ApplicationReceipt{}, err
  	}


─── apps/web/src/career/PreparationPage.tsx:150-153 ───
[bug · medium] acceptReceipt 只校验 requestId/applicationId/focus，不校验
next.status。preparation_generation_failed 分支刻意保留 attempt（提示可用原编号重试/查回执），此时点击「查询准备回执」取回的是
status='failed' 的回执（后端 FindPreparationReceipt 对失败行返回 preparationFailureReceipt(row)，requestId
匹配），acceptReceipt
照样通过校验并展示成功文案「准备草稿已生成：正文与来源链已呈现…」，与列表中渲染的「生成失败」行直接矛盾，用户会误以为草稿已可用。generatePreparation 若以非 draft 状态回执
resolve 也同样误报。建议按 next.status 分支：仅 status === 'draft' 才显示成功文案并清 attempt；failed/generating 保持恢复态提示并保留
attempt。

   const acceptReceipt = (next: PreparationReceipt, expected: GenerateAttempt): void => {
    if (next.requestId !== expected.requestId || next.applicationId !== applicationId || next.focus !== expected.focus) throw new ReceiptMismatchError('准备回执与本次请求不匹配')
+   if (next.status !== 'draft') {
+    setWritePhase('error')
+    setMessage(next.status === 'failed' ? '该请求生成失败（原请求编号与锚定信息已保留），可用原请求编号重试。' : '该请求仍在生成中，可稍后用原请求编号查询回执。')
+    refresh()
+    return
+   }
    setAttempt(undefined); setWritePhase('idle'); setFocus('')
    setMessage('准备草稿已生成：正文与来源链已呈现，可审阅并修订。系统不会自动发送任何内容。')


─── apps/web/src/career/RulePage.tsx:73-75 ───
[bug · high] 空间切换/登出路径上 removeItem 删除的是「切出身份」的规则引用，且删错后不可恢复。packages/domain/src/scope.ts 的 advance()
是先同步 abortController.abort() 再更新内部 scope，因此 abort 回调（「空间已切换或登录已失效」）执行时 scopeController.current()
返回的仍是旧 userId/tenantId，这里 removeItem 删的是旧身份的 weknora:career:rule-id:{userId}:{tenantId}。该 key
本身已按身份隔离、不存在跨身份泄漏，切换时无需删除；删除后该用户再次进入本页时 load() 读不到 storedRuleId（后端没有按用户列出规则的接口，rule-id
是唯一恢复途径），existingRuleId 为空，下一次保存走「无 ruleId 新建」路径——旧规则（可能仍
enabled）在服务端继续按周期触发计费搜索，形成双倍消耗。与已确认的「getRule 瞬时读失败导致静默新建」是不同触发路径的同类后果。建议 scope-change/invalidate
清理只清内存状态，本地引用仅在 load() 中服务端明确返回 not_found 时删除。

   const clearForScopeChange = useCallback((message: string): void => {
-   const activeScope = scopeController.current().scope
-   try { window.localStorage.removeItem(ruleIdKey(activeScope.userId, activeScope.tenantId)) } catch { /* private mode */ }
+   // 空间切换时 current() 仍指向切出的旧身份；rule-id key 已按 userId/tenantId 隔离，
+   // 切换无需删除（删除会让该用户回访时无法恢复既有规则而静默新建第二条）。
+   // 本地引用仅在 load() 中服务端明确 not_found 时移除。
+   setDraft({ query: '', interval: '1440', status: 'disabled' }); setAttempt(undefined); setReceipt(undefined); setRuleView(undefined)
+   setPhase('idle'); setNotice(message); setError(undefined)
+   setRevision(undefined); setViewPhase('scope-changed'); setViewError(undefined)
+  }, [scopeController])


─── apps/web/src/career/SearchPage.tsx:285-285 ───
[bug · medium] 确定性失败后写入入口被锁死：search_quota_refused 与 revision_conflict 都是 outcome 明确的失败分支，但二者都保留
attempt 且 phase 回到 'idle'；此时表单 textarea/提交按钮按 attempt !== undefined 禁用，而「开始新的一次找岗」仅在 phase ===
'terminal' 时渲染。结果是用户被锁死在「只能用原请求编号重试同一条指令」上，无法改写指令另起新找岗——额度被拒后想换个更窄的条件、或修订持续冲突时，页面上没有任何出路。这两个分支不像
unknown 需要锁定入口（结果已确定，不存在双写风险）；对比同批 PreparationPage/ProgressPage/SubmissionPage 的 compose 只在
busy/unknown 时禁用，均无此问题。建议在这两个确定性失败分支 setAttempt(undefined)（或让 startNewSearch 在非 busy/unknown 且
attempt 存在时也可渲染）。

-       {attempt && phase === 'terminal' ? <Button variant="outline" onClick={startNewSearch}>开始新的一次找岗</Button> : null}
+    if (parsed.code === 'search_quota_refused') { setAttempt(undefined); setError(parsed); setPhase('idle'); setNotice('额度受限不是结果未知的写入，可稍后用原请求编号重试，也可另起新的一次找岗。'); usage.reload(); return }
+    // revision_conflict 分支同理：setAttempt(undefined)（或保留 attempt 但在错误卡中提供「放弃本次指令」入口）


─── apps/web/src/career/UsagePanel.tsx:71-71 ───
[maintainability · low] 四个渲染分支的容器 className="wk-career-usage-wrap" 在 usage.css（以及 apps/web/src
全局样式）中都没有定义：同文件其余类（wk-career-usage、__status、__actions、__cost、__balance、__overage、__conditions、__note
）均有对应规则，唯独最外层 __wrap 落空，面板只剩 Card 默认样式。这与本次新增 preparation.css（__body/__revised/__edit-claims
缺失）、rule.css（__compose 缺失）是同一类遗漏。若确属有意依赖 Card 默认样式，建议删除该 className 以免误导维护者去找不存在的样式；否则在 usage.css
补齐定义。



─── apps/miniprogram/src/services/career.ts:139-139 ───
[bug · high] searchOnce 的歧义失败分支缺少 writeRecoverable/t30Recoverable 都具备的 auth.scope.isCurrent 守卫，且
searchKey() 在 catch 内才求值。作用域在请求在途期间切换时（含 runtime 传输层抛出的 SCOPE_CHANGED——该错误无 status 属性，会被 ambiguous()
判为歧义），searchKey()
取到的是切换后的新作用域，旧作用域的在途失败会把恢复意图落到新作用域的键下：新空间将看到一条来历不明的"结果未知搜索"，"安全重发"会以新空间身份执行一次收费搜索——违反"空间切换后旧响应绝不当成新事
实落地"红线。建议与 writeRecoverable 对齐：发送前捕获 stamp 并预铸 key，catch 中 !isCurrent(stamp) 时抛 SCOPE_CHANGED 且不落
intent。

-     if (ambiguous(error)) { store.write(searchKey(), { requestId: id, query: trimmed }); throw unknownOutcome(id, trimmed, error); }
+   const id = newRequestId();
+   const expected = revision();
+   const stamp = auth.scope.capture();
+   const key = searchKey(); // 发送前铸键，避免作用域切换后写入新作用域
+   try {
+     return decodeSearchOutcome(await client.request({ method: 'POST', path: '/api/v1/career/searches', body: { requestId: id, query: trimmed, expectedRevision: expected } }));
+   } catch (error) {
+     if (ambiguous(error) && !auth.scope.isCurrent(stamp)) throw Object.assign(new Error('SCOPE_CHANGED'), { cause: error });
+     if (ambiguous(error)) { store.write(key, { requestId: id, query: trimmed, expectedRevision: expected }); throw unknownOutcome(id, trimmed, error); }
+     throw error;
+   }


─── apps/miniprogram/src/services/career.ts:163-163 ───
[bug · high] 安全重发用当前 revision() 而非首次发送时的 expectedRevision，且 searchOnce 的 intent 只持久化 { requestId,
query }。服务端幂等指纹是 [kind, requestId, query, expectedRevision] 全量（search_once.go
searchFingerprint），指纹不一致时直接 ErrIdempotencyConflict：desk 修订在失败与重发之间前进（如期间确认过一条事实）后，重放必然收到
idempotency_conflict——正是同文件 writeRecoverable 注释声明要避免的"用当前值重发必然 ErrIdempotencyConflict"。请把
expectedRevision 随 intent 持久化并在重发时逐字节重放（与 StoredIntent.expectedRevision 同构）。

-     const outcome = decodeSearchOutcome(await client.request({ method: 'POST', path: '/api/v1/career/searches', body: { requestId: pending.requestId, query: pending.query, expectedRevision: revision() } }));
+ interface PendingSearchIntent { requestId: string; query: string; expectedRevision?: number }
+ // searchOnce 写 intent 时带上发送前捕获的 expectedRevision
+ export async function retryPendingSearch(): Promise<SearchOutcome> {
+   const pending = pendingSearch();
+   if (!pending) throw new Error('没有待恢复的搜索');
+   const expected = pending.expectedRevision ?? revision();
+   // 重放体使用 expected 而非 revision()


─── apps/miniprogram/src/services/career.ts:119-119 ───
[maintainability · medium] 本文件与 adapters/career-platform.ts
各自维护了一套几乎相同的恢复设施（ambiguous/t30Ambiguous、writeRecoverable/t30Recoverable、intentKey/t30Key、readIntent/
t30Intent），且两份实现已经出现行为漂移：search 版缺作用域守卫、不持久化 expectedRevision，writeRecoverable
版两者都有。重复实现正在放大不一致风险，建议把"歧义判定 + intent 读写 + 可恢复写入/对账/重试"抽成共享助手（如 core/intent.ts 或本目录
career-intent.ts）供两处使用。



─── apps/miniprogram/src/services/career.ts:7-7 ───
[maintainability · low] 同文件既以 @weknora/api-client 别名导入 NativeFileSource，又以
../../../../packages/api-client/src/career.ts 深层相对路径导入解码器（career-platform.ts 及其余 career 页面同款）。根因是包入口
index.ts 只导出了 createCareerApi/CareerRequest，未导出 career
的类型与解码器。深层相对导入绕过包边界、对包内文件布局脆弱，建议在包入口补齐导出后统一走别名导入。



─── apps/miniprogram/src/core/routes.ts:19-19 ───
[bug · high] app.config.ts 在 career 分包注册了第五个页面 rules-usage-reminders，但 ROUTES 没有对应键，且全库没有任何
navigate(...) 指向该页面（discovery 页为 careerProgress/careerLifecycle 都补了入口并注明"页面可达，无死代码"，唯独 T30
规则/额度/提醒页没有入口）。该页承载的持续找岗规则、额度预估与提醒功能将从 UI 完全不可达。建议补 careerRules 路由键，并在求职工作台（discovery 页）增加入口。

    careerProgress:{path:'career/progress-preparation',title:'申请进展与准备',tab:false},
+   careerRules:{path:'career/rules-usage-reminders',title:'持续找岗与提醒',tab:false},


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:80-81 ───
[bug · medium] loadRule 没有 try/catch：readRule 失败时 ruleErrCode 永远不会被赋值，下方 loadRuleBusy.error 的 Notice
中 ruleErrCode === 'not_found' / 'forbidden' 两个定制分支成为不可达死代码，读回失败只能显示通用"可稍后重试"文案；与 loadEstimate（catch
中 setUsageErrCode）的错误分型处理不一致。建议捕获错误写入 ruleErrCode 后 rethrow。

    const loadRule = async (): Promise<void> => {
      setRuleErrCode(undefined);
+     try {
+       const stored = readStoredRuleId();
+       if (!stored) { setRuleNotice('本机还没有保存过的规则引用。…'); setRuleView(undefined); return; }
+       const view = await readRule(stored);
+       // …
+     } catch (error) { setRuleErrCode(typedCode(error)); throw error; }
+   };


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:159-161 ───
[style · low] "下次运行计划"与"订阅不可用原因"两处均为三层嵌套三元表达式（检查清单明确禁止嵌套三元），分支文案较长时可读性差。建议抽成局部函数（如
runPlanText(live)、unavailableReason(subscription)）以 if/else 返回。

-         {live.status === 'enabled' && live.nextDueAt
-           ? <Text className='wk-row-title'>下次运行（计划）：{formatCheckTime(live.nextDueAt)}。修改规则后该计划按新频率重新排程。</Text>
-           : live.status === 'paused'
+   const runPlanText = (live: SetRuleReceipt | RuleView): string => {
+     if (live.status === 'enabled' && live.nextDueAt) return `下次运行（计划）：${formatCheckTime(live.nextDueAt)}。修改规则后该计划按新频率重新排程。`;
+     if (live.status === 'paused') return '已暂停：下一次触发已取消，当前没有排程。…';
+     return '规则未启用：不会运行，也不会在后台执行任何搜索。…';
+   };


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:143-143 ───
[maintainability · low] 本页 t-button 的 JSX 类型检查通过，依赖的是
src/subpackages/execution/artifact/t-button.d.ts 中的 declare global JSX augmentation——career
分包隐式依赖另一个分包目录下的声明文件。一旦 artifact 页面或该 d.ts 被移动/删除，本页及所有使用 t-button 的 career 页面会同时编译失败。建议把 't-button'
的全局 JSX 声明提升到共享位置（如 src/types/t-button.d.ts 或 shims 目录）。



─── apps/miniprogram/src/career/rules-usage-reminders.tsx:65-66 ───
[maintainability · low] 渲染期同步调用 pendingRuleWrite()/pendingReminderWrite()/readStoredRuleId()
直接读本地存储，属于非响应式外部状态：恢复横幅与按钮禁用态只在其他 state 变化引发重渲染时才刷新，当前依赖"每个相关动作结束后必有 setState"这一隐式约定才保持一致。建议把
pending 状态提升为组件 state（动作完成后显式更新）或用 useSyncExternalStore 订阅，避免出现陈旧横幅。



─── apps/miniprogram/src/adapters/career-platform.ts:143-143 ───
[maintainability · low] 常量名 EXPORT_GRANT_EXPIRED 与其值 'export_grant_invalid' 名值不符。后端对"过期"与"吊销"统一返回
export_grant_invalid（internal/modules/career/handler.go），Web
端也使用同一字面量——值本身正确，但命名不一致容易诱导后续维护者把值"纠正"成不存在的 export_grant_expired，导致授权过期后的自动重签重试链路（openMaterialExport
依赖该码触发重取授权）静默失效。建议改名为 EXPORT_GRANT_INVALID 并在注释中说明该码同时覆盖过期与吊销。

- export const EXPORT_GRANT_EXPIRED = 'export_grant_invalid';
+ /** 导出下载授权失效（服务端对过期/吊销统一返回此码）：可重取授权后重试。 */
+ export const EXPORT_GRANT_INVALID = 'export_grant_invalid';


─── apps/web/src/career/usage.css:18-19 ───
[bug · medium] 错误提示的选择器作用域与实际 DOM 不匹配：UsagePanel.tsx 中 phase 为 forbidden / unavailable 时，容器 section
的类是 wk-career-usage-wrap（该类全仓库无任何样式定义），内部 <div role="alert"> 不在 .wk-career-usage 元素之内，因此这两条最关键的
fail-closed 提示（「当前空间不可访问」「额度预估暂不可用」）拿不到这里的红色左边框与段落样式，只有 ready 分支的 __overage（恰好在 Card
className="wk-career-usage" 内）能命中。额度耗尽/不可用是“阻止收费找岗”的关键状态，视觉强调缺失会削弱提示。建议把选择器放宽到 .wk-career-usage-wrap
[role='alert']，或让 UsagePanel 各分支统一容器类名。

- .wk-career-usage [role='alert'] { border-left: 3px solid var(--td-error-color, #d54941); padding-left: 10px; margin: 10px 0; }
- .wk-career-usage [role='alert'] p { margin: 6px 0 0; color: var(--td-text-color-secondary, #555); }
+ .wk-career-usage-wrap [role='alert'] { border-left: 3px solid var(--td-error-color, #d54941); padding-left: 10px; margin: 10px 0; }
+ .wk-career-usage-wrap [role='alert'] p { margin: 6px 0 0; color: var(--td-text-color-secondary, #555); }


─── apps/web/src/career/reconciliation.tsx:20-20 ───
[maintainability · medium] errorDetails / isUncertainWrite / newRequestId / sourceStatusLabels 与
OpportunityPage.tsx（第 13-28 行）逐字重复，且两处 isUncertainWrite 的判定口径已经分叉：OpportunityPage 版本还依据 HTTP
status（>=500 或 <400 视为未知），本文件版本仅按错误码判断。同一“未知写入需按原 requestId 恢复”的红线逻辑存在两套实现，后续任何一侧调整（如新增错误码、修正
network_error 之类的不存在字面量）都极易漏改另一侧，直接造成两处页面对同一失败给出不同的恢复指引。建议将这组工具与标签映射抽取到 apps/web/src/career 下的共享模块（如
shared.ts），OpportunityPage / SearchPage / 本文件统一导入同一份实现。

- function errorDetails(cause: unknown): { code?: string; message: string } {
+ // 建议抽取到 apps/web/src/career/shared.ts，与 OpportunityPage/SearchPage 共用同一实现：
+ // export const newRequestId = ...
+ // export function errorDetails(cause: unknown) ...
+ // export function isUncertainWrite(cause: unknown) ...
+ // export const sourceStatusLabels: Record<OpportunitySourceStatus, string> = ...
+ import { errorDetails, isUncertainWrite, newRequestId, sourceStatusLabels } from './shared.ts'


─── apps/miniprogram/tests/application-material.test.mjs:60-64 ───
[maintainability · medium] 本批 5 个新测试文件（application-material / career-discovery / export-deletion /
progress-preparation / rules-usage-reminders）逐字复制了同一套装配：backend() 路由器、freshLogin() 登录流、me()/open3()
fixture、careerCall()/errorCode() 过滤器，各约 40-60 行。这套装配承载的是与 api-client 冻结合同耦合的口径（登录 envelope、career 裸
JSON 错误形态、路由前缀匹配规则）；一旦合同调整需要 5 处同步修改，漏改任一处会导致不同文件对同一端点的断言彼此矛盾。项目已有 tests/helpers/
目录（taro-stub.mjs），建议抽取为共享 harness 复用。

- function backend(routes) {
+ // tests/helpers/career-harness.mjs
+ export function backend(stub, routes) {
    stub.use(call => {
      const method = call.options.method ?? (call.kind === 'uploadFile' ? 'POST' : 'GET');
      const path = new URL(call.options.url).pathname;
      let fn = routes[`${method} ${path}`];
+     // …（与现实现一致，各测试文件改为 import { backend, freshLogin, careerCall } from './helpers/career-harness.mjs'）
+   });
+ }


─── apps/miniprogram/tests/application-material.test.mjs:37-37 ───
[maintainability · low] materialView fixture 定义后在本文件任何用例中均未被引用（材料视图断言均走
materialReceipt/materialVersions 路径），属死代码。保留它会误导读者以为存在 MaterialView 解码的覆盖，建议删除。



─── apps/miniprogram/tests/career-discovery.test.mjs:60-61 ───
[maintainability · low] errorCode 与 ambiguous 两个辅助函数在本文件没有任何调用点（ambiguous 仅出现在 C2 用例标题字符串里，errorCode
仅被 dead 的 ambiguous 引用），属死代码；它暗示存在"歧义结果判定"的行为覆盖，实际并没有。建议删除，或若后续需要歧义分类语义则补上真实调用它的用例。



─── apps/miniprogram/tests/export-deletion.test.mjs:384-384 ───
[test · low] 第二个 M1 用例中 receipt.status 已在上一行断言为 'partial'，因此 `receipt.status !== 'partial'` 恒为
false，整个 deletionUnknown 表达式退化为常量——断言只覆盖了默认解锁分支，且是手写内联的页面公式。本文件 F1 用例已改用与页面同源的
deletionOutcomeUnknown 计算 deletionUnknown
输入；此处若沿用内联副本，页面语义（export-deletion.gating.ts）调整时该断言不会察觉漂移。建议同样调用共享函数，显式构造 intentPresent=true +
inMemoryStatus='partial' 的组合。

-   const partial = pageGatingFromServiceState({ deletionUnknown: career.pendingSpaceDeletion() !== null && receipt.status !== 'partial' });
+   const { deletionOutcomeUnknown } = lifecycleGatingModule;
+   const partial = pageGatingFromServiceState({ deletionUnknown: deletionOutcomeUnknown({ intentPresent: career.pendingSpaceDeletion() !== null, inMemoryStatus: receipt.status }) });


─── apps/miniprogram/tests/export-deletion.test.mjs:339-339 ───
[maintainability · low] lifecycleGatingModule/fix2Gating 两个可变模块级容器 + 每个用例内 Object.assign(await
import(...)) 动态注入的方式脆弱：lifecycleGatingModule 的声明位于消费它的 pageGatingFromServiceState 之后，仅靠 node:test
延迟执行才规避 TDZ；后续若把该函数提前调用会直接 ReferenceError。已核实 export-deletion.gating.ts 是零依赖的纯函数模块（不 import
@tarojs/taro），完全可以在文件顶层静态导入一次（本文件其他 import 已使用顶层 await），删除两个容器与 'M1 行为缺失' 守卫分支。

- const lifecycleGatingModule = {};
+ // 与其他顶层 import 并列，删除 lifecycleGatingModule/fix2Gating 及各用例内的 Object.assign 注入
+ const { lifecycleGating, deletionOutcomeUnknown, deletionRecoveryUnresolvedAfter } = await import('../src/career/export-deletion.gating.ts');


─── apps/miniprogram/tests/progress-preparation.test.mjs:405-410 ───
[test · low] R1-P4 自述"逐句复刻页面逻辑"，手工重写了 progress-preparation.tsx saveRevision 的组合保存链（editable 映射 →
bodyFromEditable → claims 判空 → recoverEmptyClaims →
editMaterial）。一旦页面调整判空条件或取回时机，本用例不会失败而是与页面静默漂移，F1 主张保全的端到端保障失效。建议把这条组合链抽为页面与测试同源的纯函数（例如 career.ts
中导出 composeClaimSafeMaterialBody(sections, loadServerBody)），saveRevision 与本用例共同调用它，测试才真正锁定页面行为。

-   // 页面 saveRevision 的同链组合（t-button 不可单测，逐句复刻页面逻辑）：
-   const editable = draft.sections.map(section => ({ heading: section.heading, content: section.content, claims: Array.isArray(section.claims) ? section.claims : [] }));
-   let body = career.bodyFromEditable(editable);
+ // services/career.ts：页面 saveRevision 与测试同源调用
+ export async function composeClaimSafeMaterialBody(sections: EditableMaterialSection[], loadServerSections: () => Promise<MaterialBody['sections']>): Promise<MaterialBody> {
+   const body = bodyFromEditable(sections);
    if (body.sections.some(section => (section.claims ?? []).length === 0)) {
-     body = { sections: platform.recoverEmptyClaims(body.sections, (await career.material('mat-9')).body.sections) };
+     return { sections: recoverEmptyClaims(body.sections, await loadServerSections()) };
+   }
+   return body;
-   }
+ }


─── apps/miniprogram/tests/progress-preparation.test.mjs:292-293 ───
[test · low] C1 以固定 30ms 的 setTimeout 窗口"证明"重连后不自动提交，属时序启发式断言：若实现引入更长退避（如 >30ms
的自动重试定时器），自动提交可绕过窗口而断言仍通过；CI 慢启动则只会平白增加等待。建议改为确定性信号：在替身中记录定时器/延迟任务注册并断言为零，或引入可注入的时钟 seam
由测试显式推进时间后再断言调用计数无增长。



─── apps/miniprogram/tests/build-output.test.mjs:16-16 ───
[test · medium] 该文件是 D1/D2/F1 三类真实线上缺陷的回归防线，但 package.json 的 test script 是 `node
--experimental-strip-types --test tests/*.test.mjs`，不含构建步骤。CI 若只跑 `pnpm test`（未先 `pnpm
build:weapp`），本文件所有用例恒定 skip、零覆盖却绿灯通过，防线形同虚设。建议：在 package.json 增加 `test:dist`（先 build:weapp 再
test）并让 CI 至少在一个 job 中串联执行；同时在 skip 分支输出显式 console.warn，避免静默跳过。

  const skipReason = existsSync(resolve(dist, 'app.json')) ? false : 'dist/ 不存在——先运行 pnpm build:weapp';
+ if (skipReason) console.warn(`[build-output] 全部用例被跳过：${skipReason}。CI 需保证 build:weapp && test 的串联 job 真正执行本文件。`);


─── apps/miniprogram/tests/build-output.test.mjs:92-93 ───
[test · medium] 正则 `from"(\.\.\/common[^"]*)"` 假设产物 JS 为无空格的压缩格式（当前 project.config.json 设
minified:true 才成立）。一旦构建配置改为非压缩输出（`from "../common/..."` 带空格），matchAll 返回空数组、循环体一次都不执行，"button
运行时依赖必须存在"这组断言会静默全部通过（假阴性），恰好在构建配置变更这个最需要防线的时刻失效。建议：正则放宽为 `from\s*"..."`，并在零匹配时显式 fail——button.js
必然依赖 ../common 运行时，零匹配本身就是异常信号。

    const buttonJs = readFileSync(resolve(tdesignDir, 'button/button.js'), 'utf8');
-   for (const m of buttonJs.matchAll(/from"(\.\.\/common[^"]*)"/g)) {
+   const commonDeps = [...buttonJs.matchAll(/from\s*"(\.\.\/common[^"]*)"/g)];
+   assert.ok(commonDeps.length > 0, 'button.js 必然 import ../common 运行时；零匹配说明产物格式变化，需同步更新本正则');
+   for (const m of commonDeps) {


─── apps/miniprogram/tests/build-output.test.mjs:103-104 ───
[maintainability · low] 该硬编码清单与 config/index.ts 的 tdesignClosure（按 button.json 递归动态收集，注释明言"TDesign
升级新增依赖自动跟进"）形成双向耦合：TDesign 升级后配置端自动通过、本清单必然失败，维护者容易误判为回归而非预期升级。作为"防全量拷贝"哨兵意图明确，但建议：① 从
config/index.ts 导出 tdesignClosure 结果供本测试推导期望清单（同一事实来源）；或至少在断言消息中提示"TDesign 升级新增闭包依赖时需同步更新此清单"。另外
walkFiles 对 dist/npm/tdesign 整体缺失（copy 配置失效）时会直接抛 ENOENT 而非走断言/skip 分支，错误形态与文件其余部分"缺 dist
给出可读提示"的约定不一致，建议先 existsSync 判定再 walk。



─── apps/miniprogram/tests/helpers/taro-stub.mjs:99-101 ───
[maintainability · low] copyFile 在校验 srcPath 是否存在之前就执行 state.copies.push，copies 记录的是"尝试"而非"成功"；而同文件中
removedFiles（unlink）只在成功路径记录——同一 state 面板两种语义并存。assembly.test.mjs 已有
`assert.equal(stub.state.copies.length, 1, '只有成功的下载才创建私有副本')` 这类断言，一旦未来出现 copyFile 失败的用例，失败尝试也会使
copies 计数 +1，断言消息与实际语义脱节。writeFile 的 fileWrites 同样是先 push 后校验。建议统一为"仅成功才记录"（把 push 移到校验之后），与
removedFiles 对齐；现有用例均为成功路径，不受影响。

        copyFile({ srcPath, destPath, success, fail }) {
          queueMicrotask(() => {
+           if (!state.fileContents.has(srcPath)) { fail?.({ errMsg: `copyFile:fail no such file or directory, open ${srcPath}` }); return; }
            state.copies.push({ srcPath, destPath });


─── apps/miniprogram/tests/helpers/taro-stub.mjs:129-129 ───
[maintainability · low] `??` 条件赋值意味着：若加载顺序中已有别的替身先定义了不含 env.USER_DATA_PATH 的 wx，本文件会静默沿用旧对象，随后
files.ts 的 userDir() 会以"本机存储目录不可用"这种远离根因的方式失败——与文件头"替身必须决定 wx 契约、防止单测掩盖平台差异"的意图相悖。当前 node --test
默认每测试文件独立子进程，尚无实际污染，但一旦以 --test-isolation=none 或同进程方式运行就会踩中。建议无条件赋值：本替身就是要拥有 wx 契约的唯一来源。

- globalThis.wx = globalThis.wx ?? { env: { USER_DATA_PATH: 'wxfile://usr' } };
+ globalThis.wx = { env: { USER_DATA_PATH: 'wxfile://usr' } };


─── apps/miniprogram/src/career/application-material.tsx:81-81 ───
[bug · high] 材料对账丢弃了 reconcilePendingMaterial() 返回的回执，转而用页面输入态 materialId
读取。最常见的结果未知场景恰是「保存草稿创建新材料失败」——此时 materialId 仍为 ''（receipt.materialId 尚未到达页面），对账成功后
career.material('') 必然抛「缺少材料编号」；且对账成功即清除 intent，回执随丢弃而丢失，材料卡片 ((application || materialId) 条件)
也不渲染，用户失去进入该材料的唯一入口，违背「未知创建结果用同一 request ID 恢复」红线。对比 progress-preparation.tsx 的重试链路正确使用了
receipt.materialId。

-       <Action secondary loading={recoverMatBusy.busy} onClick={() => void recoverMatBusy.run(async () => { await career.reconcilePendingMaterial(); setMaterial(await career.material(materialId)); })}>用原请求对账材料</Action>
+       <Action secondary loading={recoverMatBusy.busy} onClick={() => void recoverMatBusy.run(async () => {
+         const receipt = await career.reconcilePendingMaterial();
+         setMaterialId(receipt.materialId);
+         const view = await career.material(receipt.materialId);
+         setMaterial(view);
+         setSections(career.editableFromBody(view.body));
+         if (view.versionCount > 0) setExports((await career.listMaterialExports(receipt.materialId)).exports);
+       })}>用原请求对账材料</Action>


─── apps/miniprogram/src/career/application-material.tsx:162-163 ───
[bug · medium] confirmedVersion 只在确认真材料时设置，切换材料（读取材料）与创建新申请的重置块均未清空。时序：确认材料 A 得 V2 → 粘贴编号读取材料 B（3
个版本）→ 发布按钮直接以旧值发布：publishMaterial(B, 2) 会把 B 的 V2（可能与用户预期不符的内容）固化为不可变导出并供投递绑定；「已确认版本
V2」提示也会跨材料残留误导。读取材料处应 setConfirmedVersion(undefined)；「为其他批次创建新申请」的
setMaterialId('')/setExports(undefined) 重置块同理应一并重置。

          const view = await career.material(materialId);
          setMaterial(view);
+         setConfirmedVersion(undefined);


─── apps/miniprogram/src/career/application-material.tsx:87-89 ───
[maintainability · low] 「void receipt;」是为消除未用变量告警的死语句，直接不接收返回值即可（loadSubmissions 已刷新列表，回执无需保留）。

-         const receipt = await career.reconcilePendingSubmission();
+         await career.reconcilePendingSubmission();
          if (application) await loadSubmissions(application.applicationId);
-         void receipt;


─── apps/miniprogram/src/career/application-material.tsx:146-146 ───
[style · low] linkState Badge 使用嵌套三元（规范禁止）。本文件已有 evaluationTone 映射表先例，同一模式即可消除嵌套。export-deletion.tsx
/ progress-preparation.tsx / discovery.tsx 存在同款嵌套三元，建议一并处理。

-         <Badge tone={application.linkState === 'ready' ? 'success' : application.linkState === 'linking' ? 'warning' : 'danger'}>{linkStateLabel[application.linkState]}</Badge>
+         <Badge tone={linkStateTone[application.linkState] ?? 'neutral'}>{linkStateLabel[application.linkState]}</Badge>
+ // 文件顶部补充：const linkStateTone: Record<string, 'success' | 'warning' | 'danger'> = { ready: 'success', linking: 'warning', link_failed: 'danger' };


─── apps/miniprogram/src/career/application-material.tsx:227-227 ───
[bug · low] downloadBusy 单一实例被所有导出卡片的所有格式下载按钮共享，任一下载进行中会让全部下载按钮同时进入 loading（无法分辨哪个在进行）；listBusy
也同时驱动「刷新导出列表」与「读取投递记录」两个不相干区块。建议按 `${exportId}:${format}` 维度跟踪 loading 状态。discovery.tsx 的
confirmBusy/dismissBusy 跨全部提案共享属同款问题。



─── apps/miniprogram/src/career/application-material.tsx:23-25 ───
[maintainability · medium] tdesignButtonStyle / digestHead / typedCode 在
discovery.tsx、export-deletion.tsx、progress-preparation.tsx（及 rules-usage-reminders.tsx 的
tdesignButtonStyle/typedCode）逐字重复定义，共 5 处。主题 CSS 变量串或错误码取值调整需同步改多个文件，极易漂移。建议抽取到共享模块（如
src/career/shared.ts 或 core/errors.ts 扩展 typedCode），各页面统一引用。

- const tdesignButtonStyle = '--td-brand-color:var(--wk-color-action-primary);--td-brand-color-active:var(--wk-color-action-pressed);--td-brand-color-disabled:var(--wk-color-action-disabled);--td-button-primary-bg-color:var(--wk-color-action-primary);--td-button-primary-active-bg-color:var(--wk-color-action-pressed);--td-button-primary-disabled-bg-color:var(--wk-color-action-disabled);--td-button-primary-disabled-color:var(--wk-color-action-disabled-text);--td-button-large-height:var(--wk-component-button-height);';
- const digestHead = (digest: string): string => digest.slice(0, 12);
- const typedCode = (error: unknown): string | undefined => { const code = (error as { code?: unknown } | null | undefined)?.code; return typeof code === 'string' ? code : undefined; };
+ // 新建 apps/miniprogram/src/career/shared.ts 统一导出：
+ export const tdesignButtonStyle = '...';
+ export const digestHead = (digest: string): string => digest.slice(0, 12);
+ export const typedCode = (error: unknown): string | undefined => { const code = (error as { code?: unknown } | null | undefined)?.code; return typeof code === 'string' ? code : undefined; };
+ // 各页面改为 import { tdesignButtonStyle, digestHead, typedCode } from './shared.ts';


─── apps/miniprogram/src/career/discovery.tsx:37-37 ───
[bug · medium] 分享 path 把 draft.rawText 全文 encodeURIComponent 后拼入 ?jd=：中文经 UTF-8 percent-encoding 约膨胀
9 倍，微信分享卡片 path 有长度上限，长 JD（几百字以上很常见）会导致分享失败或 query
被截断失真；生产端与接收端（decodeEntryPayload）均无长度防护（PREVIEW_LIMIT=160 只作用于摘要展示）。建议对超长原文降级为不带 jd 的入口
path（接收端已有粘贴恢复入口），并设定明确字符上限。

-   useShareAppMessage(() => ({ title: draft ? `职位核对：${draft.preview.excerpt.slice(0, 20)}` : 'WeKnora 求职工作台', path: `/career/discovery${draft ? `?jd=${encodeURIComponent(draft.rawText)}` : ''}` }));
+   // encodeURIComponent 后中文约 9 倍膨胀，分享 path 有长度上限：超长原文降级为不带 jd 的入口，
+   // 接收端已有粘贴原文恢复入口兜底。
+   useShareAppMessage(() => {
+     const jd = draft && encodeURIComponent(draft.rawText).length <= 1024 ? `?jd=${encodeURIComponent(draft.rawText)}` : '';
+     return { title: draft ? `职位核对：${draft.preview.excerpt.slice(0, 20)}` : 'WeKnora 求职工作台', path: `/career/discovery${jd}` };
+   });


─── apps/miniprogram/src/career/discovery.tsx:43-43 ───
[maintainability · medium] 以用户可见中文文案子串「搜索额度不足」判定 typed 429（search_quota_refused）来降级 Notice
语气：core/errors.ts:8 的措辞一旦微调（或本地化），判定即静默失效退化为 danger 失败态，无人报错。错误对象本就携带
code，应与本文件其他页（application-material/export-deletion 的 setExpErrCode 模式）一致：catch 中 typedCode(error) 存入
state，按 code === 'search_quota_refused' 判定。

-   const quotaRefused = (message?: string) => message?.includes('搜索额度不足') ?? false;
+   // 与其他 career 页一致：catch 中 typedCode(error) 捕获存入 state（如 searchErrCode），按 code 判定而非文案子串
+   const quotaRefused = (code?: string) => code === 'search_quota_refused';
+   // 使用处：{searchErrCode && <Notice tone={quotaRefused(searchErrCode) ? 'warning' : 'danger'}>…


─── apps/miniprogram/src/career/discovery.tsx:129-129 ───
[style · low] 来源标签一行内嵌套三层三元（available 外层 + failureCode 内层），违反禁止嵌套三元规范且可读性差。建议抽 helper：const
sourceLabel = (source) => source.available ? source.label : `${source.label}（不可用${source.failureCode
? `：${source.failureCode}` : ''}）`。



─── apps/miniprogram/src/career/export-deletion.tsx:222-222 ───
[style · low] deletion 状态 Badge 为嵌套三元；下方 <Card tone={deletion.status === 'deleted' ? 'mint' :
deletion.status === 'partial' ? 'warning' : 'white'}> 同款。文件内已有 deletionStatusLabels 映射先例，补一个 tone
映射表即可。

-         <Badge tone={deleted ? 'success' : deletion.status === 'partial' ? 'danger' : 'warning'}>{deletionStatusLabels[deletion.status]}</Badge>
+         <Badge tone={deletionTone[deletion.status] ?? 'warning'}>{deletionStatusLabels[deletion.status]}</Badge>
+ // 文件顶部补充：const deletionTone: Record<string, 'success' | 'danger' | 'warning'> = { deleted: 'success', partial: 'danger', deleting: 'warning' };


─── apps/miniprogram/src/career/progress-preparation.tsx:276-276 ───
[style · low] 准备列表状态文案为嵌套三元（draft/failed/生成中三层）。文件内已大量使用 *Labels 映射表模式，此处抽出
preparationStatusLabel(item) 或 Record 映射即可保持一致。

-           <Text className='wk-row-title'>{focusLabels[item.focus] ?? item.focus} · {item.status === 'draft' ? '草稿（可审阅、可修订）' : item.status === 'failed' ? `生成失败（${item.failureCode ?? ''}）：${item.failureMessage ?? '生成未完成'}` : '生成中（可恢复）'}</Text>
+           <Text className='wk-row-title'>{focusLabels[item.focus] ?? item.focus} · {preparationStatusLabel(item)}</Text>
+ // 辅助函数：const preparationStatusLabel = (item: PreparationReceipt): string => item.status === 'draft' ? '草稿（可审阅、可修订）' : item.status === 'failed' ? `生成失败（${item.failureCode ?? ''}）：${item.failureMessage ?? '生成未完成'}` : '生成中（可恢复）';


─── apps/miniprogram/tests/build-output.test.mjs:64-65 ───
[bug · low] Windows 兼容性缺陷：`resolve('/')` 在 Windows 上返回的是盘符根（如 `C:\`）而非路径分隔符，导致 `r + resolve('/')`
拼出的前缀字符串永远无法匹配任何文件路径，`isSub` 恒为 false（`p === r` 也永不成立，因为 walkFiles 只收集文件而 r
是目录）。结果是所有分包文件被计入主包体积，本用例在 Windows 开发机上会给出偏大的错误体积、可能误报"超过微信 2MB 上限"。应改用 `path.sep`（POSIX 上
`resolve('/') === '/' === path.sep`，两种平台均正确）。

    const subRoots = (appJson.subpackages ?? appJson.subPackages ?? []).map(s => resolve(dist, s.root));
-   const isSub = p => subRoots.some(r => p === r || p.startsWith(r + resolve('/')));
+   const isSub = p => subRoots.some(r => p === r || p.startsWith(r + sep));


─── apps/miniprogram/tests/build-output.test.mjs:24-24 ───
[maintainability · low] 解析基准与小程序契约不一致：该断言把组件引用一律相对 dist 根解析（先剥离 `/` 前缀再 `resolve(dist,
...)`），这只对绝对引用成立。小程序 usingComponents 的相对引用（`./`、`../`）语义是相对当前页面 json 所在目录解析——一旦 Taro
构建输出改为相对引用，`resolve(dist, ref)` 会解析到 dist 之外的错误位置，四处 existsSync
全部误报失败，且报错信息（"组件文件必须存在"）不会提示是解析基准错了。建议按引用形态区分解析基准，或在 ref 不以 `/` 开头时显式 fail 提示契约变化。

-   const base = resolve(dist, ref.replace(/^\//, ''));
+   const pageDir = resolve(dist, 'subpackages/execution/artifact');
+   const base = ref.startsWith('/') ? resolve(dist, ref.slice(1)) : resolve(pageDir, ref);


─── apps/miniprogram/src/subpackages/execution/artifact/index.tsx:10-11 ───
[maintainability · low] 本页重写后，features/execution/pages.tsx 中原有的 ArtifactPage（基于
client.chat.artifacts.session 的会话级产物实现）已无任何引用——原先唯一的引用方就是本文件被删除的 `import { ArtifactPage as Feature
}`（全仓搜索确认仅剩定义处）。两套产物页语义（session 级 vs 任务级）并存容易造成误用，建议在同一次变更中一并删除旧实现。



─── apps/miniprogram/src/subpackages/execution/artifact/index.tsx:9-9 ───
[bug · low] supported() 第二个分支缺少 ^ 锚定：任何以 officedocument.wordprocessingml.document 结尾的 MIME
都会被判为支持，而非仅匹配 DOCX 的标准 MIME，与 openProtectedDocument 的扩展名白名单口径不一致。建议锚定完整标准 MIME，避免服务端 MIME 推导变化时误放行。

- const supported=(name:string,mime:string)=>/^application\/pdf$|officedocument\.wordprocessingml\.document$/i.test(mime)||/\.(pdf|docx)$/i.test(name);
+ const supported=(name:string,mime:string)=>/^(application\/pdf|application\/vnd\.openxmlformats-officedocument\.wordprocessingml\.document)$/i.test(mime)||/\.(pdf|docx)$/i.test(name);


─── apps/miniprogram/src/subpackages/execution/artifact/index.tsx:35-35 ───
[style · low] customStyle 是纯静态字符串，却在列表 map 内随每个卡片、每次渲染重建，且与页面其余样式组织分离。建议提升为模块级常量（与 supported
同级声明），减少内联样式重复并便于集中维护。

-      customStyle='--td-brand-color:var(--wk-color-action-primary);--td-brand-color-active:var(--wk-color-action-pressed);--td-brand-color-disabled:var(--wk-color-action-disabled);--td-button-primary-bg-color:var(--wk-color-action-primary);--td-button-primary-active-bg-color:var(--wk-color-action-pressed);--td-button-primary-disabled-bg-color:var(--wk-color-action-disabled);--td-button-primary-disabled-color:var(--wk-color-action-disabled-text);--td-button-large-height:var(--wk-component-button-height);'
+ const TD_BUTTON_STYLE='--td-brand-color:var(--wk-color-action-primary);--td-brand-color-active:var(--wk-color-action-pressed);--td-brand-color-disabled:var(--wk-color-action-disabled);--td-button-primary-bg-color:var(--wk-color-action-primary);--td-button-primary-active-bg-color:var(--wk-color-action-pressed);--td-button-primary-disabled-bg-color:var(--wk-color-action-disabled);--td-button-primary-disabled-color:var(--wk-color-action-disabled-text);--td-button-large-height:var(--wk-component-button-height);';
+ // 使用处：customStyle={TD_BUTTON_STYLE}


─── apps/miniprogram/src/platform/files.ts:43-45 ───
[maintainability · low] 注释承诺"清理失败不静默、必须可见"，但该 Error 既无 code 也无 status，经 components/ui.tsx 的
errorMessage() 兜底后，UI
只会显示通用文案"操作未完成，请检查网络或刷新状态后重试"——清理失败与下载失败在用户视角不可区分，承诺的辨识度在展示层丢失；且消息内嵌设备本地完整路径（wxfile://usr/...），不宜进入可
展示的错误文案。建议附加机器可读 code 并在 core/errors.ts 中映射专属文案，消息本身不携带路径（路径可通过 cause 保留给日志/调试）。

- function unlinkFile(fs:ReturnType<typeof Taro.getFileSystemManager>,filePath:string):Promise<void>{
-  return new Promise((resolve,reject)=>fs.unlink({filePath,success:()=>resolve(),fail:e=>reject(Object.assign(new Error(`临时副本清理失败：${filePath}`),{cause:e}))}));
- }
+ fail:e=>reject(Object.assign(new Error('临时副本清理失败'),{code:'ARTIFACT_CLEANUP_FAILED',cause:e}))
+ // 并在 core/errors.ts 增加映射：if(e.code==='ARTIFACT_CLEANUP_FAILED')return '文件已打开，但本机清理临时副本失败，请稍后重试或重新进入页面。';


─── apps/miniprogram/tests/artifact-cleanup.test.mjs:95-95 ───
[test · low] 该函数与 helpers/taro-stub.mjs 内部的 isRuntimeTempPath 逻辑重复，且两份实现已经漂移：stub 还判定 'tmp/'
前缀，此处副本没有。断言"副本不能仍在运行时临时目录"依赖这份二手口径，stub 后续调整判定范围时测试仍会按旧口径通过，D3 契约防回规能力被削弱。建议从 taro-stub.mjs 导出
isRuntimeTempPath 并在此处导入复用。



─── docs/design/job-search/prototype/app.js:17-17 ───
[security · medium] XSS 转义覆盖依赖手工调用且不完整：esc() 仅覆盖 state.prompt、state.notice、JSON.stringify(state)
三处，而 render() 整体经 root.innerHTML 写入的字符串中，jobRow/evidenceCard/journeyView/commandView 里
item.title、job().company、job().gap、job().source 等十余处动态插值均为原文拼接。当前 jobs 是文件内静态数据故不可利用，但按需求红线
JD/岗位数据属不可信输入，一旦后续把粘贴的 JD 文本或导入数据接入这些字段、任一插值点漏掉 esc()，即形成随 state 反复渲染的存储型 XSS。建议统一插值出口：所有动态文本一律经
esc()（或封装 text()/模板函数自动转义），避免依赖逐点手工转义的完整性。

- const esc = value => String(value).replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char]));
+ '<strong>' + esc(item.title) + '</strong><small>' + esc(item.company) + ' · ' + esc(item.city) + ' · ' + esc(item.salary) + '</small><span class="job-meta">' + esc(item.source) + ' · ' + esc(item.posted) + '</span>


─── docs/design/job-search/prototype/app.js:118-118 ───
[bug · low] setNotice 触发的 render() 会全量重建 innerHTML，而页面上的输入控件均不回填 state：搜索框 value 硬编码为"前端开发 · AI
产品团队"，composer 指令输入框为非受控且无 value。用户在输入框中键入内容后，任何点击交互（选岗位、切视图、切平台等 17 处 setNotice
入口）都会把已输入内容静默重置/清空；键盘方向键切换仅排除了输入框聚焦场景，点击路径无此保护。建议在 render() 前将输入值存入 state 并回填（如 value="' +
esc(state.searchText) + '"），或对 notice 更新做局部 DOM 更新而非整体重渲染。



─── docs/design/job-search/prototype/app.js:87-87 ───
[maintainability · low] journeyView、commandView、profileView、materialsView、applicationsView
五个视图函数均声明了 platform 形参但函数体内从未使用（仅 workbenchView 用它计算
compact），属声明未读取的死参数，易误导读者以为这些视图存在平台分支逻辑。建议删除未使用的形参，或如确需平台差异则真正使用它。

- function journeyView(platform) {
+ function journeyView() {


─── docs/design/job-search/prototype/app.js:34-36 ───
[style · low] shell() 返回值第二行的 (web ? '<header class="web-header">…' : '<div class="phone-status">…'
+ (mobile ? '<div class="app-header">…' : '<div class="mini-header">…')) 构成嵌套三元表达式，违反项目 JS
规约（禁止嵌套三元）；且该行拼接链长达数百字符，可读性差、修改时极易括号错配。建议将设备头部构造提取为独立辅助函数，用 if/return 展开分支。

- function shell(platform) {
-   const web = platform === 'web';
-   const mobile = platform === 'mobile';
+ function deviceHeader(platform) {
+   if (platform === 'web') return '<header class="web-header">…</header>';
+   const app = '<div class="app-header">…</div>';
+   const mini = '<div class="mini-header">…</div>';
+   return '<div class="phone-status"><span>9:41</span><span>●●● ▰</span></div>' + (platform === 'mobile' ? app : mini);
+ }


─── docs/design/job-search/prototype/app.js:144-145 ───
[maintainability · low] platform=all 且 variant=C 时，render() 会为 web/mobile/mini 三份设备各渲染一次
commandView，产生三个 id="command-form" 与三个 id="prompt-input"：同一文档重复 id 属非法 HTML，且 getElementById
只能取到第一个。当前 submit 事件按 event.target.id 判断碰巧可用，但任何按 id 查找的逻辑都会失效。建议改用 class + closest 判定，去掉全局唯一 id。

  root.addEventListener('submit', event => {
-   if (event.target.id !== 'command-form') return;
+   if (!event.target.closest('form.composer')) return;


─── docs/design/job-search/prototype/app.css:9-10 ───
[maintainability · low] 文件采用"末尾追加覆盖块"修改既有规则，同一文件内规则互相抵消、留下死规则：(1) .mobile/.mini .details-pane
.evidence-card 在上方声明 display:none，本行又覆盖为 display:block，前者成为死规则；(2) :root 三次声明，页面背景由 gray-3 被本行改为
gray-2(#eee)，与 index.html 的 theme-color #f3f3f3(gray-1) 不一致；(3) .btn.primary:hover 的原
background:gray-color-13 被下方追加的 brand-color-5
覆盖为死规则；.device、.journey-hero、.orbit-core、.variant-switcher
等也均被二次声明改写。后续维护者删除或重排追加块会导致样式静默回退。建议把追加块的值直接合并进首次定义处（或删掉被覆盖的死规则），使每条规则只有一个生效来源，并同步修正 index.html 的
theme-color。

- /* Primary surfaces use the same brand ramp as the TDesign theme. */
- :root{color:var(--td-gray-color-13);background:var(--td-gray-color-2)}
+ /* 将覆盖值合并入首次定义：*/
+ :root{font-family:var(--app-font-family);color:var(--td-gray-color-13);background:var(--td-gray-color-2);font-synthesis:none}
+ /* 并删除上方 .mobile .details-pane .evidence-card{display:none} 与旧 .btn.primary:hover 死规则 */


─── internal/modules/workbench/service/workbench/application_task.go:119-122 ───
[bug · medium] 重试预算耗尽后返回 ErrApplicationTaskConflict 与接口契约矛盾。career_application_task.go 中 Conflict
被定义为「确定拒绝（request ID 绑定不同内容，或 application 已绑定其他请求），调用方必须停止」，而锁竞争（database is locked /
sqlite_busy）耗尽时创建结果是未决的：并发携带同一 request ID 的孪生请求可能已成功提交 mapping（本文件 ensureOnce 的 OnConflict→replay
路径正说明该孪生可成功）。Career 侧 application.go:338-341 收到 Conflict 后会把申请链接置为终态 ApplicationLinkStateFailed 并报
"workbench rejected the task link"，此后同 request ID 重放直接返回 failed 终态，与实际可能已存在且 ready 的任务分叉（只能靠
ReconcileApplicationLink 手动修复）。建议：重试耗尽且最后一次仍为 race 类错误时，先做一次恢复性 Find——命中则走 applicationTaskReplay
返回成功；未命中则返回一个区别于 Conflict 的未决错误（如 ErrApplicationTaskUndecided），让 Career 维持 linking 态进入
OutcomeUnknown/对账恢复，而不是终态 Failed。

+ 	if racedLink, found, findErr := c.FindCareerApplicationTask(ctx, tenantID, ownerID, intent.RequestID); findErr == nil && found {
+ 		return racedLink, nil
+ 	}
  	return interfaces.CareerApplicationTaskLink{}, fmt.Errorf(
  		"%w: creation still racing after %d attempts: %v",
- 		ErrApplicationTaskConflict, applicationTaskMaxAttempts, lastRace,
+ 		ErrApplicationTaskUndecided, applicationTaskMaxAttempts, lastRace,
  	)


─── internal/modules/workbench/service/workbench/application_task.go:312-314 ───
[bug · medium] request ID 的 64 字符上限与 Career 侧契约不一致，且校验失败复用 Conflict 哨兵。Career 的 CreateApplication
接受最长 128 字符的 RequestID（application.go:123 `len(input.RequestID) > 128`，DB 列为 gorm size:128），而这里超过 64
即返回 ErrApplicationTaskConflict——65~128 字符的「合法」请求 ID 会确定性失败：Career 将申请永久标记为
ApplicationLinkStateFailed（终态，重放不再重试）。另外 normalizeApplicationTaskIntent/Scope 把非 UUID 的
ApplicationID、标题长度、scope 缺失等纯输入校验错误也统一归入 Conflict，而接口文档定义 Conflict 仅覆盖两种绑定冲突，Career 无法区分「输入错误（应返回
ErrInvalidRequest）」与「真实冲突」，最终用户看到的是误导性的 "workbench rejected the task link"。建议：与 Career 对齐上限（Career
侧收紧到 64，或 linker 侧放宽并在 Career 侧生成 ≤64 的 ID），并为校验类失败引入独立哨兵（或让 Career 在 Ensure 前先行校验）以便映射为
ErrInvalidRequest。附带：len(intent.Title) > 255 按字节计数，但错误文案写的是 characters，多字节中文标题会被提前拒绝。



─── internal/modules/workbench/service/workbench/application_task_removal.go:45-56 ───
[bug · medium] 快照读与删除窗口不一致，破坏文件自身承诺的幂等性。rows 在事务外 Find，事务内 mapping 表按 Where 条件删除（会连带删掉并发新提交的行），而
agent_runs/sessions 却按事务前快照的 taskIDs 删除。若并发 EnsureCareerApplicationTask 在 Find 与事务提交之间落地（Career 侧
Ensure 在 Career commit 之后调用，与删除步骤并发是可达的），新任务的 mapping 会被 Where 删除、但其 run/session
不在快照中而残留为孤儿；更关键的是重跑无法恢复——重试时 Find 已无 mapping 行而提前返回 nil，孤儿 run/session 永不可清理，违反注释中 "a retry after a
partial failure simply removes whatever is still there" 的承诺，也削弱「导出删除需撤销相关 Task」的红线。建议在同一事务内以 mapping
表为唯一事实源驱动三张表删除：先在事务内重查 rows（或用子查询 DELETE agent_runs/sessions WHERE session_id IN (SELECT task_id
FROM workbench_application_tasks WHERE tenant_id=? AND owner_id=? AND origin=?))，再删
mapping，保证三个删除作用于同一快照。

+ 	condition := "tenant_id = ? AND owner_id = ? AND origin = ?"
+ 	args := []any{tenantID, ownerID, careerApplicationTaskOrigin}
  	err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
- 		if err := tx.Table("workbench_application_tasks").
- 			Where(
- 				"tenant_id = ? AND owner_id = ? AND origin = ?",
- 				tenantID, ownerID, careerApplicationTaskOrigin,
- 			).
- 			Delete(nil).Error; err != nil {
+ 		// 以 mapping 表为同一事实源驱动三表删除，避免事务外快照与并发创建竞态
+ 		if err := tx.Exec(
+ 			"DELETE FROM agent_runs WHERE session_id IN (SELECT task_id FROM workbench_application_tasks WHERE "+condition+")",
+ 			args...,
+ 		).Error; err != nil {
  			return err
  		}
- 		if err := tx.Table("agent_runs").Where("session_id IN ?", taskIDs).Delete(nil).Error; err != nil {
+ 		if err := tx.Exec(
+ 			"DELETE FROM sessions WHERE id IN (SELECT task_id FROM workbench_application_tasks WHERE "+condition+")",
+ 			args...,
+ 		).Error; err != nil {
  			return err
  		}
+ 		return tx.Table("workbench_application_tasks").Where(condition, args...).Delete(nil).Error
+ 	})


─── apps/miniprogram/src/career/application-material.tsx:186-189 ───
[bug · high] 保存草稿的分支判据是 `material`（是否已读取过材料），而编辑分支使用的却是输入框当前值 `materialId`，两者可自由分叉（Field
随时可改编号，placeholder 还引导「重进页面可粘贴读取」）。典型事故路径：读取材料 A（sections 载入 A 的正文）→ 把编号改为 B 但未点「读取材料」→ 点「保存草稿」：走
`editMaterial({ materialId: B, body })`，A 的正文整体替换 B 的草稿——服务端 expectedRevision
校验的是全局档案修订而非材料修订（career.ts materialRequestBody），无法拦截；「确认为不可变新版本」「发布」「下载」同样作用于 B 而页面仍展示 A
的状态/版本/风险。另外创建申请成功的重置块清了 materialId/material/exports 却未清
`sections`，旧材料正文会在下一次不经读取的「保存草稿」中被静默写入新申请名下的新材料。建议：materialId onChange
时同步失效读取态（setMaterial(undefined)/setSections([])/setExports(undefined)/setConfirmedVersion(undefined)
/setChecks([])），或在保存/确认/发布前校验编号与已读取材料一致。

-         <t-button block size='large' theme='primary' ariaLabel='保存材料草稿' customStyle={tdesignButtonStyle} loading={matEditBusy.busy} onTap={() => void matEditBusy.run(async () => {
-           const body = career.bodyFromEditable(sections);
-           const receipt = material
-             ? await career.editMaterial({ materialId, body })
+ <Field label='材料编号（创建后自动带入；重进页面可粘贴读取）' value={materialId} onChange={value => { setMaterialId(value); setMaterial(undefined); setSections([]); setExports(undefined); setConfirmedVersion(undefined); setChecks([]); }} placeholder='材料编号' />;


─── apps/miniprogram/src/career/discovery.tsx:52-52 ───
[bug · medium] recoverBusy 的错误状态从未渲染：useAction.run 会捕获异常存入 error 且不
rethrow（components/ui.tsx:56），而本页没有任何地方显示 recoverBusy.error。「用原请求对账恢复」与底部「同步 Web 端档案变更」共用该
action，二者失败时（如对账被拒「请求不属于当前空间」、同步网络失败）按钮 loading 结束后页面毫无反馈，用户无从得知失败原因——违反「异步操作必须有用户可见的错误提示」。同文件其他
action（confirmBusy/searchBusy/importBusy 等）均有 error Notice，此处应补齐或拆成两个独立 action 各自显示。

-     {pendingFactAction && <Action secondary loading={recoverBusy.busy} onClick={() => void recoverBusy.run(async () => { await career.reconcilePending(); query.reload(); })}>用原请求对账恢复</Action>}
+ {pendingFactAction && <Action secondary loading={recoverBusy.busy} onClick={...}>用原请求对账恢复</Action>}
+ {recoverBusy.error && <Notice tone='danger'>{recoverBusy.error} 对账被拒时说明该请求不属于当前空间或不存在；可稍后再试。</Notice>}


─── apps/miniprogram/src/career/progress-preparation.tsx:135-137 ───
[bug · medium] claims 取回的 `await career.material(materialId)` 位于下方 try/catch 之外。当正文存在空 claims
小节时断网点「保存修订」：这次网络调用失败会直接冒泡，不会进入 catch 的 looksOffline
分支，用户看不到「网络不可用：已保留本地草稿（可继续编辑，未提交）……联网后请再点『保存修订』显式同步」的断网语义提示（本地草稿其实已保存），只得到通用错误；而正文各节 claims
均非空时断网失败却能走 try 内 catch 给出断网提示——两条路径行为不一致。建议把 claims 取回并入同一个 try 块（语义不变：取不回 claims 仍不提交正文）。

+ try {
-     if (body.sections.some(section => (section.claims ?? []).length === 0)) {
+   if (body.sections.some(section => (section.claims ?? []).length === 0)) {
-       body = { sections: recoverEmptyClaims(body.sections, (await career.material(materialId)).body.sections) };
+     body = { sections: recoverEmptyClaims(body.sections, (await career.material(materialId)).body.sections) };
-     }
+   }
+   await career.editMaterial({ materialId, body });


─── apps/miniprogram/src/career/progress-preparation.tsx:250-250 ───
[bug · low] fromTimeline 置 true 后全页没有任何重置路径（无
setFromTimeline(false)）。用户从时间线点「为这场面试做准备」后，再把准备焦点切换到「求职信」时，仍显示「已从时间线进入：将为这场面试准备」，文案与实际选中的焦点不符且持续到重进页
面。建议在 setFocus 时重置，或直接以 focus === 'interview_prep' 派生显示条件。

-       {fromTimeline && <Notice tone='info'>已从时间线进入：将为这场面试准备（基于该申请实际投递的版本）。</Notice>}
+ {fromTimeline && focus === 'interview_prep' && <Notice tone='info'>已从时间线进入：将为这场面试准备（基于该申请实际投递的版本）。</Notice>}


─── apps/miniprogram/src/career/application-material.tsx:10-10 ───
[maintainability · low] errorMessage 导入后从未使用（全文错误展示均使用 busy.error / typedCode），属于死导入；若 tsconfig 开启
noUnusedLocals 会直接编译失败。建议删除该导入。

- import { errorMessage } from '../core/errors.ts';
+ // 删除该行：errorMessage 未被使用


─── apps/miniprogram/src/career/export-deletion.tsx:2-2 ───
[maintainability · low] Taro 导入后未使用（本页全部平台能力经由 components/ui、adapters 与 services 封装），属于死导入，建议删除。

- import Taro from '@tarojs/taro';
+ // 删除该行：Taro 未被使用


─── apps/miniprogram/src/career/export-deletion.gating.ts:36-39 ───
[maintainability · low] exportBlocked 与 deletionBlocked 两个字段没有任何消费者：页面只使用
exportDisabled/deletionDisabled/acknowledgeDisabled，tests/export-deletion.test.mjs 的断言（第 353/354/397
行）也全部使用 *Disabled，仅注释提及 blocked 语义。作为「页面与测试共用」的公共 seam，未被读取的字段是冗余 API
表面，后续维护者可能误以为有运行时分支依赖它们。建议从返回值与接口中移除（或让 Disabled 计算保留局部变量、不导出）。

    return {
-     exportBlocked,
-     deletionBlocked,
      exportDisabled: exportBlocked || !input.revisionLoaded,
+     deletionDisabled: deletionBlocked || !input.revisionLoaded || !input.boundaryShown || !input.acknowledged || input.deleted,
+     acknowledgeDisabled: deletionBlocked,
+   };


─── apps/miniprogram/src/career/export-deletion.tsx:75-79 ───
[bug · low] 恢复入口成功后陈旧错误提示不清理：发起导出失败（如 ambiguous 网络失败留 intent）时 exportBusy.error + expErrCode
已展示；随后走「用原请求对账导出」成功并 acceptExport，但 exportBusy.error/expErrCode 不会被清除（useAction 的 error 只在下一次 run
时重置），页面同时呈现失败 Notice（以及 expErrCode==='revision_conflict'
时的「重新读取档案修订」Action）与成功的导出包卡片，误导用户。acceptDeletion 同理：delErrCode 只在发起路径的 try
成功时清理，recDelBusy/retryDelBusy 成功路径不清。建议在 acceptExport/acceptDeletion 内重置对应错误码（exportBusy.error
无法外部清除，可让错误 Notice 以「最近一次发起未成功」的独立 state 控制）。

    const acceptExport = (receipt: CareerExportReceipt): void => {
      setExported(receipt);
      setSaved(undefined);
      setCopyNotice('');
+     setExpErrCode(undefined);
    };


─── apps/miniprogram/tests/export-deletion.test.mjs:137-138 ───
[test · medium] A2 最后一条断言是恒真式：fixture 的 archive.submissions[0]（文件顶部定义）根本没有 boundVersion 字段，且
versionConfirmed 固定为 true，因此 `boundVersion === undefined || versionConfirmed === true` 无论解码器行为如何都为
true——即使 decodeCareerExportReceipt 静默丢弃 versionConfirmed
或绑定版本四元组，该断言照样通过。它并未真正保护「归档投递行要么带绑定版本、要么显式未确认」的合同语义。建议改为直接断言冻结的扁平字段（versionConfirmed +
materialId/exportId/version/contentDigest），或补一条 versionConfirmed=false 的 fixture 行覆盖另一分支。

    assert.equal(view.submissions.length, 1, 'submission records');
-   assert.equal(view.submissions[0].boundVersion === undefined || view.submissions[0].versionConfirmed === true, true);
+   assert.equal(view.submissions[0].versionConfirmed, true, 'the archive row carries the confirmed binding');
+   assert.equal(view.submissions[0].materialId, 'mat-1');
+   assert.equal(view.submissions[0].exportId, 'exp-1');
+   assert.equal(view.submissions[0].version, 2);
+   assert.equal(view.submissions[0].contentDigest, 'c'.repeat(64));


─── apps/miniprogram/tests/progress-preparation.test.mjs:310-312 ───
[test · medium] 注释声称验证「作用域键隔离第二层：即使缓存未被清除，B 的 scopeKey 也读不到 A 的草稿」，但该层从未被真正测试：freshLogin 的第一步就是
stub.reset()（清空整个 storage），所以 B 读到 undefined 是测试基建清库的结果——即使 preparationDraftKey 完全去掉
scopeKey（隔离失效、跨账号可见上一用户草稿），这条断言依然通过。账号切换后不得残留上一用户的可见草稿是本项目红线，值得真实覆盖。savePreparationDraft
会返回实际写入的受控键，可在「不清库、仅切换作用域」的场景下比对 A/B 两次写入的原始键并断言 B 读不到 A 的记录。

-   // 作用域键隔离第二层：即使缓存未被清除，B 的 scopeKey 也读不到 A 的草稿。
-   await freshLogin(meB, {});
-   assert.equal(platform.readPreparationDraft('app-1', 'interview_prep'), undefined, 'account B sees no trace of account A');
+   // 第二层：模拟缓存残留——A 的草稿仍在库里，仅把作用域切到 B（不执行 stub.reset()/清库）。
+   const keyA = platform.savePreparationDraft({ applicationId: 'app-1', focus: 'interview_prep', materialId: 'mat-9', sections: [{ heading: '面试要点', content: 'A 的私人草稿' }] });
+   await switchScopeTo(meB); // 复用 backend 装配 + 登录，但跳过 stub.reset()
+   assert.equal(platform.readPreparationDraft('app-1', 'interview_prep'), undefined, 'account B sees no trace of account A even with residual cache');
+   const keyB = platform.savePreparationDraft({ applicationId: 'app-1', focus: 'interview_prep', sections: [{ heading: '面试要点', content: 'B 的草稿' }] });
+   assert.notEqual(keyB, keyA, 'draft storage keys are scope-isolated');


─── apps/miniprogram/tests/rules-usage-reminders.test.mjs:25-26 ───
[maintainability · low] clearPrivateCache 在本文件导入后从未被调用（A5 用例走的是
runtime.auth.clear()，其余用例均不涉及登出清库），属死导入。保留它会让读者误以为本文件覆盖了「登出清除 wk:career:*
受控存储」的语义，实际并没有。建议删除该行；若确需覆盖，请补充真正调用它的用例。



─── apps/miniprogram/src/career/rules-usage-reminders.tsx:105-106 ───
[bug · high] 规则保存的恢复链存在死锁，页面文案承诺的"对账无记录后可用原请求编号安全重发"在常见多端场景下不成立：当原请求确认未达服务端（对账
404）且期间档案头修订已前进（规则写入不推进档案修订，但用户在 Web 端确认事实、或本端「申请与材料」页的写入都会推进）时，retryPendingRule 原样重放持久化的
input.expectedRevision，服务端幂等重放检查先于 CAS（search_rule.go:293 在 :324 之前），收不到已存回执就必然走到 CAS →
revision_conflict。而 t30Recoverable 把 revision_conflict 判为确定失败、不清 intent；reconcilePendingRule 又持续
404——intent 永久滞留，保存按钮因 disabled 中的 pendingRule !== null 永久禁用，页面只有对账/重试两个动作、没有"放弃恢复"出口，唯一清除路径是登出
clearPrivateCache。建议二选一（或都做）：① 适配器在 retryPendingRule 捕获到确定 revision_conflict 时清除
intent——收到该错即可证明原请求从未落地（若已落地会先命中幂等回执），intent 已不可达，提示用户按当前修订重新保存；② 恢复区块增加显式"放弃本次恢复"操作。注意
services/career.ts 的 writeRecoverable/retryIntent
各写入（application/material/submission/progress/preparation/spaceExport）具有完全相同的结构，建议一并评估。

-       <Action secondary loading={retryRuleBusy.busy} onClick={() => void retryRuleBusy.run(async () => { acceptRuleReceipt(await retryPendingRule()); })}>用原请求编号重试规则保存</Action>
-       {retryRuleBusy.error && <Notice tone='danger'>{retryRuleBusy.error} 重试沿用原请求编号与原档案修订，服务端幂等不会写入第二条规则。</Notice>}
+ // adapters/career-platform.ts —— 重放收到确定 revision_conflict 即证明原请求从未落地（幂等检查先于 CAS），intent 不可达，安全清除：
+ export async function retryPendingRule(): Promise<SetRuleReceipt> {
+   const pending = pendingRuleWrite();
+   if (!pending) throw new Error('没有待恢复的规则保存');
+   try {
+     return await t30Recoverable<SetRuleReceipt>('rule-write', '规则保存', pending.input, async id => {
+       const receipt = decodeSetRuleReceipt(await client.request({ method: 'POST', path: '/api/v1/career/rules', body: ruleRequestBody(id, pending.input) }));
+       t30Clear('rule-write'); acceptRuleReceipt(receipt);
+       return receipt;
+     }, pending.requestId);
+   } catch (error) {
+     if ((error as { code?: unknown })?.code === 'revision_conflict') {
+       t30Clear('rule-write');
+       throw Object.assign(new Error('原保存未到达服务端且档案修订已前进，请按当前修订重新保存'), { cause: error, recoverable: true });
+     }
+     throw error;
+   }
+ }


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:47-48 ───
[maintainability · low] usageErrCode 是只写不读的死状态：全文件仅在第 77/78 行调用 setUsageErrCode，usageErrCode
本身从未被读取。相比之下 ruleErrCode 驱动了 loadRuleBusy.error/saveBusy.error 两处 Notice 的 typed 分型文案，而额度预估的失败
Notice（第 120 行）只用 usageBusy.error 展示通用文案——两者不对称。建议要么删除该 state，要么像 ruleErrCode 一样在预估错误 Notice 中按
typed code（如 search_quota_refused/forbidden）做分型提示。

    const [estimate, setEstimate] = useState<UsageEstimateView>();
-   const [usageErrCode, setUsageErrCode] = useState<string>();
+   // 删除 usageErrCode；或保留并在第 120 行错误 Notice 中消费：
+   // {usageBusy.error && <Notice tone='danger'>{usageBusy.error}{usageErrCode === 'forbidden' ? ' 当前空间不可访问额度预估。' : ' 可稍后重试。'}</Notice>}


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:1-3 ───
[maintainability · low] Taro 导入从未使用：本页没有任何 Taro.xxx 调用（分享入口、文件选择等平台能力都经由 adapters/career-platform.ts
间接使用）。属于死导入，建议删除。

  import { useState } from 'react';
- import Taro from '@tarojs/taro';
  import { Text, View } from '@tarojs/components';


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:154-154 ───
[style · low] 除已确认的"下次运行计划/订阅不可用原因"两处外，本页还有三处嵌套三元表达式，属检查清单禁止的同类问题：第 141 行规则状态单选文案（option ===
'enabled' ? … : option === 'paused' ? … : …）、第 150 行 saveBusy.error Notice
的三分支文案链（revision_conflict/idempotency_conflict/outcome_unknown）、第 154 行 Badge tone。建议抽成查表或局部函数（如
statusLabels 已有先例，可补 optionLabel: Record<RuleStatus, string> 与 badgeToneOf(status)）。

-           <Badge tone={live.status === 'enabled' ? 'success' : live.status === 'paused' ? 'warning' : 'neutral'}>{statusLabels[live.status]}</Badge>
+ const statusOptionLabels: Record<RuleStatus, string> = { enabled: '启用（按间隔自动触发）', paused: '暂停（取消下一次触发，恢复后顺延）', disabled: '停用（不运行）' };
+ const badgeToneOf = (status: RuleStatus): 'success' | 'warning' | 'neutral' => status === 'enabled' ? 'success' : status === 'paused' ? 'warning' : 'neutral';
+ // 使用：<Badge tone={badgeToneOf(live.status)}>{statusLabels[live.status]}</Badge>


─── docs/design/job-search/prototype/app.js:60-62 ───
[maintainability · low] `compact` 参数与类名是无效死代码，且两处调用口径不一致：jobRow/evidenceCard 会向容器输出
class="...compact"，但 app.css 中不存在任何 `.compact` 选择器，该类名不产生任何样式效果；同时 workbenchView 内 jobRow 使用 `const
compact = platform !== 'web'` 按平台计算，而同一函数里 `evidenceCard(true)` 却把 compact 硬编码为 true，导致 web 端岗位行非
compact、证据卡恒为 compact，口径互相矛盾，容易误导维护者以为存在平台差异样式。建议删除 compact 形参与类名拼接（或补充对应 `.compact` CSS
规则并统一两处取值口径）。

- function jobRow(item, index, compact = false) {
+ function jobRow(item, index) {
    const selected = state.selectedJob === index;
-   return '<button class="job-row ' + (selected ? 'selected ' : '') + (compact ? 'compact' : '') + '" data-job="' + index + '"><span class="company-mark ' + item.accent + '">' + item.company.slice(0, 1) + '</span><span class="job-text"><strong>' + item.title + '</strong><small>' + item.company + ' · ' + item.city + ' · ' + item.salary + '</small><span class="job-meta">' + item.source + ' · ' + item.posted + '</span></span><span class="fit-score">' + item.fit + '<small>%</small></span></button>';
+   return '<button class="job-row ' + (selected ? 'selected' : '') + '" data-job="' + index + '">...';
+ // 同步删除 evidenceCard 的 compact 形参与 workbenchView 中的 const compact = platform !== 'web';
+ // evidenceCard() 直接无参调用


─── internal/application/repository/resource.go:144-153 ───
[bug · low] 租约到期后的并发清理会把"实际已成功的删除"报告为错误：清理者 A claim 后物理删除耗时超过
guardedDeleteLease（大文件/慢存储/进程暂停），租约到期后清理者 B 重新 claim 并先完成 Finish（state→deleted）。随后 A 调用本方法时 WHERE
state='deleting' 匹配 0 行，返回 ErrResourceUnavailable（文案还是 "resource unavailable for
binding"），DeleteUnbound 将其作为失败上抛——删除已彻底完成却给调用方（Career 导出删除链路）报错。虽然重试可经 IsDeleted
自愈，但误导性哨兵错误会被放大为用户可见失败。建议在 RowsAffected==0 时复核终态墓碑（id+tenant+state='deleted'），命中则幂等返回 nil，仅在非终态时才报错。

- func (r *resourceRepository) FinishUnboundResourceDelete(ctx context.Context, tenantID uint64, resourceID string) error {
- 	res := r.db.WithContext(ctx).Model(&types.StoredResource{}).Where("id=? AND tenant_id=? AND state=?", resourceID, tenantID, types.ResourceStateDeleting).Updates(map[string]any{"state": types.ResourceStateDeleted, "deleted_at": time.Now().UTC()})
- 	if res.Error != nil {
- 		return res.Error
- 	}
  	if res.RowsAffected != 1 {
+ 		var finished int64
+ 		if err := r.db.WithContext(ctx).Unscoped().Model(&types.StoredResource{}).
+ 			Where("id=? AND tenant_id=? AND state=?", resourceID, tenantID, types.ResourceStateDeleted).
+ 			Count(&finished).Error; err == nil && finished == 1 {
+ 			return nil
+ 		}
  		return interfaces.ErrResourceUnavailable
  	}
  	return nil
  }


─── internal/handler/session/artifact_download.go:644-652 ───
[bug · low] 与同一端点上 legacy grant 的错误契约不一致：DownloadWorkbenchArtifactGrant 对签名密钥缺失返回 501 +
code=artifact_signing_disabled，对验签失败返回 401 并区分
artifact_grant_expired/artifact_grant_invalid（注释明确说"过期单独上报以便客户端重新授权"）。本方法对同样的情形全部返回裸 404。客户端在
/workbench/artifacts/download 上按 grant_type 分发后，对过期的 version
链接无法区分"过期→重新走认证端点签发"与"已撤销/不存在"，密钥未配置时也表现为 404 而非明确的部署配置错误。建议对齐 legacy 契约：密钥缺失→501
artifact_signing_disabled，验签失败→401 并区分 expired/invalid（过期信息由链接自身携带，不泄露版本存在性；撤销/所有权复核仍保持 404 不泄露）。

  	secret, err := workbench.ArtifactSigningKeyFromEnv()
  	if err != nil {
- 		c.AbortWithStatus(http.StatusNotFound)
+ 		c.AbortWithStatusJSON(http.StatusNotImplemented, gin.H{"success": false, "code": "artifact_signing_disabled", "error": "artifact signing key not configured"})
  		return
  	}
  	if err := workbench.VerifyArtifactVersionGrantAt(secret, grant, signature, time.Now()); err != nil {
- 		c.AbortWithStatus(http.StatusNotFound)
+ 		code := "artifact_grant_invalid"
+ 		if strings.Contains(err.Error(), "expired") {
+ 			code = "artifact_grant_expired"
+ 		}
+ 		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "code": code})
  		return
  	}


─── internal/handler/session/artifact_download.go:702-707 ───
[maintainability · low] 凭证无关下载路径的存储读取失败完全静默：同一文件中已认证的 DownloadArtifactVersion 在 GetFile 失败时会
logger.Warnf("artifact version download read failed...") 再返回 404，本方法直接 404 无任何日志。两类路径都是防探测式
404，但存储后端真实故障（挂载丢失、桶不可达）时运维无法从日志区分故障与探测噪音。同理，上方 GetOwnedRun/members.Get 的 DB 错误也被静默吞为
404。建议对齐兄弟路径，至少为 GetFile 失败补一条含 tenant/version 的 Warnf。

  	reader, err := fileService.GetFile(ctx, version.ObjectKey)
  	if err != nil {
+ 		logger.Warnf(ctx, "artifact version grant read failed: run=%s version=%s err=%v", version.RunID, version.ID, err)
  		c.AbortWithStatus(http.StatusNotFound)
  		return
  	}
  	name := artifactVersionFileName(version)


─── apps/web/src/appconnector/ActionApproval.tsx:185-191 ───
[bug · critical] 严重 bug：这里的 `<textarea>` 仍是原生 HTML 元素（小写标签，且本文件未从 tdesign-react 导入 Textarea），但
onChange 却被改成了 TDesign 的回调签名。原生 `<textarea>` 的 onChange 第一个参数是
`React.ChangeEvent<HTMLTextAreaElement>` 事件对象而非字符串，`String(event)` 会得到 `"[object Event]"`——用户每敲一个键
draftContent 就被整体覆盖为 `"[object Event]"`，导致 `reprepare()` 中 `JSON.parse(draftContent)`
必然失败，"编辑内容→重新准备内容"功能完全失效。这是一次半截迁移：只改了 handler、漏换了组件。两种修复方式任选其一：(1) 保留原生 textarea，恢复事件签名
`onChange={(event) => setDraftContent(event.target.value)}`；(2) 完成迁移，改为 `import { Button as TButton,
Textarea as TTextarea } from 'tdesign-react'` 并使用 `<TTextarea>`（参考 PersonalMemorySettingsPanel.tsx
中的既有写法）。

-                   <textarea
+                   <TTextarea
                      id="action-content-edit"
                      aria-label="编辑操作内容 JSON"
                      value={draftContent}
-                     rows={4}
-                     onChange={(value) => setDraftContent(String(value))}
+                     autosize={{ minRows: 4, maxRows: 4 }}
+                     onChange={(value) => setDraftContent(String(value ?? ''))}
                    />
+ // 并同步修改导入：import { Button as TButton, Textarea as TTextarea } from 'tdesign-react';


─── apps/miniprogram/src/subpackages/execution/artifact/index.tsx:48-48 ───
[maintainability · medium] 错误文案追加句与 errorMessage() 的映射文案重复/错位：当错误为 ARTIFACT_GRANT_EXPIRED
时，action.error 已经是 core/errors.ts
中的「下载授权已过期，请再次点击"打开或保存"重新获取。」，再拼接「授权过期时再次点击"打开或保存"会重新获取。」后用户会看到同一句指导连续出现两遍；而错误为其他原因（网络失败、清理失败、SCOPE_
CHANGED 等）时，追加的「授权过期时…」与实际错误无关，反而暗示了错误的成因。授权过期的引导语义已由 errorMessage 按 code
映射承担，建议删除这句硬编码追加文案；若确需补充引导，应基于具体错误 code 条件展示。

-   {action.error&&<Notice tone='danger'>{action.error} 授权过期时再次点击“打开或保存”会重新获取。</Notice>}
+   {action.error&&<Notice tone='danger'>{action.error}</Notice>


─── apps/miniprogram/config/index.ts:19-20 ───
[maintainability · low] 存在性检查的顺序颠倒了：realpathSync 在最常见的失败场景（依赖未执行 pnpm install）会直接抛裸的 ENOENT（"no such
file or directory, realpath ..."），而下一行精心准备的「run pnpm install first」友好提示在该场景下永远不可达——它只能覆盖「包已安装但
miniprogram_dist 内缺 button/button.js」这种罕见情况。应先用 existsSync 检查包路径存在、再 realpath 解符号链接；同理下方
readdirSync(resolve(tdesignDist,dir)) 在 common/miniprogram_npm 目录缺失时也会抛裸 ENOENT。

- const tdesignDist=realpathSync(resolve(process.cwd(),'node_modules/tdesign-miniprogram/miniprogram_dist'));
+ const tdesignSource=resolve(process.cwd(),'node_modules/tdesign-miniprogram/miniprogram_dist');
+ if(!existsSync(tdesignSource))throw new Error(`tdesign-miniprogram not found at ${tdesignSource} — run pnpm install first`);
+ const tdesignDist=realpathSync(tdesignSource);
  if(!existsSync(resolve(tdesignDist,'button/button.js')))throw new Error(`tdesign-miniprogram miniprogram_dist not found at ${tdesignDist} — run pnpm install first`);


─── apps/miniprogram/src/subpackages/execution/artifact/t-button.d.ts:16-16 ───
[maintainability · low] theme 联合类型包含了 'warning'，但 TDesign Miniprogram 的 t-button 实际只支持
default/primary/danger/light 四种（'warning' 是 TDesign Web/React 版的取值，小程序版没有）。当前类型声明允许传入一个运行时无任何效果的
theme 值，编译期不报错、真机上静默失效，后续使用者容易踩坑。建议与所装 1.17.0 版本的 button.d.ts（miniprogram_dist/button/type.ts 中
Theme 的定义）对齐后收窄。

-         theme?: 'default' | 'primary' | 'danger' | 'light' | 'warning';
+         theme?: 'default' | 'primary' | 'danger' | 'light';


─── apps/miniprogram/tests/artifact-cleanup.test.mjs:75-88 ───
[maintainability · low] 401 用例几乎完整复制了 freshLogin 的 me()/stub.use 骨架（login/me 路由分发完全相同），两份代码仅在
downloadFile 分支的响应上有差异。测试场景继续增加（如 403、500、超时）时会进一步复制。建议让 freshLogin 接收一个 downloadResponder 参数（或导出
makeLoginHandler(download)），把差异点收敛到一处，登录桩的后续维护（如新增路由）只需改一份。



─── apps/mobile/src/task-office-integration-smoke.ts:130-138 ───
[documentation · medium] 探针实际从不发出任何网络请求，建议修正注释/证据语义以免被过度解读为"服务端边界已验证"。已核实调用链：signIn 前
`runtime.scopeLease()` 返回 `undefined`（mobile-runtime.ts 中 `lease` 仅在 `authenticate()`
成功后赋值），`createTaskOffice` 的 `requireLease()` 在客户端同步抛 `TASK_OFFICE_SCOPE_CHANGED`，请求根本到不了
`backend.list`；即便 lease 意外通过，`authorizedRequest` 也会在无凭证时抛 `RUNTIME_UNAUTHORIZED`（同为客户端拒绝）。因此
`unauthenticatedRead: 'rejected'` 是与目标部署安全姿态完全无关的确定性客户端结论——即使服务端 API 完全敞开，该字段仍会记 'rejected'。同仓测试第 57
行也自证了 no-lease Office 未发生 backend 调用。当前注释 "Prove the exact production Task Office boundary... do not
send credentials" 暗示曾向服务端发出无凭证请求，容易让 evidence 读者（如验收/合规审查）误读为服务端认证边界的证据。建议改写注释明确说明：本探针只验证客户端 Task
Office 门禁（lease 缺失即 fail-closed，无任何请求离机），如需验证服务端边界需另行直连探测。

-   // Prove the exact production Task Office boundary fails closed before sign-in.
-   // Reuse the same deployment origin and transport, but do not send credentials.
-   const unauthenticatedOffice: TaskOffice = createTaskOffice({
+   // Client-gate check only: with no signed-in session, runtime.scopeLease() is
+   // undefined and createTaskOffice's lease guard rejects synchronously
+   // (TASK_OFFICE_SCOPE_CHANGED) — no request ever leaves the device. This
+   // proves the production Task Office fails closed pre-login; it does NOT
+   // attest the server's own authN boundary.
+   const buildOffice = (): TaskOffice => createTaskOffice({
      backend: createTaskOfficeRemote({
        origin: config.deploymentOrigin,
        request: (input) => runtime.authorizedRequest(input),
      }),
      lease: () => runtime.scopeLease(),
    });
+   const unauthenticatedOffice: TaskOffice = buildOffice();


─── apps/mobile/src/task-office-integration-smoke.ts:154-160 ───
[maintainability · low] 此处与上方 unauthenticatedOffice 的构造块逐字节相同（backend 工厂、origin、request、lease
全部一致，且闭包引用同一个 runtime）。两份拷贝未来若只改其一（例如给认证 office 补充 detail/stream 端口），会造成"探针验证的门禁"与"实际使用的
office"悄然分叉。建议提取单一工厂或直接复用同一实例（signIn 前后该 office 的行为差异完全由 runtime 状态决定，构造本身无差别）。

-   const office: TaskOffice = createTaskOffice({
-     backend: createTaskOfficeRemote({
-       origin: config.deploymentOrigin,
-       request: (input) => runtime.authorizedRequest(input),
-     }),
-     lease: () => runtime.scopeLease(),
-   });
+   // 复用与探针相同的构造（或提取 const buildOffice = () => createTaskOffice({...})）
+   const office: TaskOffice = buildOffice();


─── apps/web/src/administration/AdministrationPage.tsx:164-164 ───
[bug · medium] 常量丢失了原实现中的前导空格：原代码为 `const PAGER_BTN_DISABLED = ' disabled:...'`（带前导空格）。现在上一页/下一页按钮处
`PAGER_BTN + PAGER_BTN_DISABLED` 会拼出非法类名
`wk-admin-pager-btnwk-admin-pager-btn-disabled`，两个类的规则（inline-flex 布局、24px 尺寸、cursor、禁用态颜色/透明度、hover
变色）全部匹配失败，造成分页器样式回归。请补回前导空格（或改用数组/join 拼接）。

- const PAGER_BTN_DISABLED = 'wk-admin-pager-btn-disabled';
+ const PAGER_BTN_DISABLED = ' wk-admin-pager-btn-disabled';


─── apps/web/src/analytics/analytics-u.css:73-75 ───
[maintainability · medium] Token 引用被硬编码替代：原 Tailwind 的
`bg-accent`/`bg-surface`/`bg-accent-wash`/`text-accent`/`border-accent` 解析为 `var(--color-accent)` /
`var(--color-surface)` / `var(--color-accent-wash)` 等 @theme token。styles.css 的 @theme 注释明确说明这些别名就是供
`var(--color-*)` 形式的消费者使用的，且 packages/design-tokens 已有 `:root[theme-mode="dark"]`
运行时切换机制（--wk-color-* 系列）。本文件多处（btn-primary/btn-outline/btn-pager 的 #07c05f 与
rgba(7,192,95,0.08)、各输入/卡片的 background-color: #ffffff、focus 边框色）写死后，后续品牌色或主题调整需逐条改值、无法跟随 token。建议改回
`var(--color-accent)` / `var(--color-surface)` / `var(--color-accent-wash)` 引用（当前这两个 token
仍是静态值，行为无回归，改动零风险）。

    border-style: solid;
    border-width: 0;
-   background-color: #07c05f;
+   background-color: var(--color-accent); /* 同理：#ffffff → var(--color-surface)；rgba(7,192,95,0.08) → var(--color-accent-wash) */


─── apps/web/src/analytics/analytics-u.css:242-243 ───
[style · low] 迁移产物存在若干格式/一致性问题：1) 一行双声明（本处及 .wk-anl-2 的 `border-bottom-width: 1px;border-color:
#eef1f5;`）；2) `.wk-anl-an-usage-num` 在文件中段定义 `text-align: right`，末尾又补 `font-variant-numeric:
tabular-nums;`，同一选择器拆两处（连同 @media 块内部选择器未缩进），后续维护易漏改；3) `:focus`/`:hover` 规则中重复声明基础规则已有的
`border-style: solid;`。建议收敛为单条 border 简写、合并同一选择器的声明并清理冗余属性。

-   border-bottom-style: solid;
-   border-bottom-width: 1px;border-color: #eef1f5;
+   border-bottom: 1px solid #eef1f5;


─── apps/web/src/analytics/AnalyticsPage.tsx:335-336 ───
[maintainability · low] `wk-anl-1` / `wk-anl-2` / `wk-anl-3` 为数字序号命名，且直接以内联字面量散落在 JSX（label、thead
tr、th），与页面其余所有样式类经 `AN_*` 语义常量引用的模式不一致，可读性和可检索性差（在 CSS 里看到 .wk-anl-3 无法推断用途）。建议改为语义命名并纳入常量，如
`AN_FILTER_LABEL = 'wk-anl-filter-label'`、`wk-anl-usage-thead`、`wk-anl-usage-th`。

-           <label className="wk-anl-1">
+           <label className={AN_FILTER_LABEL}>
              {t(locale, 'analytics.rangeFrom')}


─── apps/web/src/administration/AdministrationPage.tsx:0-0 ───
[bug · medium] 邮箱输入从 `<Input type="email" required ...>` 迁移到 TInput 时丢失了 `type="email"` 与
`required`，浏览器原生的必填与邮箱格式校验随之移除。而 `onInviteFormSubmit`/`sendInvitation` 只有 `!email.trim()`
的空值兜底，非法格式（如 "abc"）会直接进入确认步骤并触发后端失败；空值提交也从原先的浏览器提示变为静默 no-op。建议在提交处理中补回格式校验（TDesign Input 的 type
联合类型不含 'email'，不宜直接透传）：

- <TInput className="wk-admin-invite-email" value={email} onChange={(value) => setEmail(String(value))} />
+ function onInviteFormSubmit(event: React.FormEvent<HTMLFormElement>) {
+     event.preventDefault(); if (!manageTenant || !email.trim()) return;
+     if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) return; // 原 type="email" 的浏览器格式校验兜底
+     setInviteStep('confirm');
+   }


─── apps/web/src/administration/AdministrationPage.tsx:198-198 ───
[maintainability · low] `wk-admin-page-size`、`wk-admin-invite-email`、`wk-admin-pager-jump` 三个类名在
administration-u.css（及全仓样式文件）中均无对应规则，仅被 AdministrationPage.test.tsx 用作 DOM
查询锚点。无样式语义的类名容易被误认为遗漏的样式规则，也易在清理时被误删。建议改用 `data-testid` 表达测试锚点意图，或在 CSS/TSX 中加注释说明这些类仅作测试选择器。



─── apps/web/src/administration/AdministrationPage.tsx:206-206 ───
[other · low] 跳页输入迁移到 TInput 时丢失了 `inputMode="numeric"`，移动端不再弹出数字键盘，属迁移遗漏的轻微 UX 回归（仓内其他数字输入如
TaskBudget 均保留该属性）。若 TDesign Input 不透传原生属性，可考虑改回原生 input 或通过其他方式补齐。

-       <TInput className="wk-admin-pager-jump wk-admin-25" type="text" value={jump}
+       <TInput className="wk-admin-pager-jump wk-admin-25" type="text" inputMode="numeric" value={jump}


─── apps/web/src/administration/administration-u.css:32-34 ───
[maintainability · low] 硬编码 `#07c05f` 与 `#2e6de6`（.wk-admin-3 的 color）分别对应 styles.css 中单一来源定义的
`--color-accent` 与 `--color-primary`，而同文件 `.wk-admin-pager-current` 已使用 `var(--color-accent,
#07c05f)` 形式，风格不一致。品牌色调整或主题切换时硬编码处不会跟随变化，建议统一改为 var() 引用（.wk-admin-3 同理改 `color:
var(--color-primary, #2e6de6)`）。

  .wk-admin-pager-btn-disabled:enabled:hover {
-   color: #07c05f;
+   color: var(--color-accent, #07c05f);
  }


─── apps/web/src/administration/administration-u.css:18-23 ───
[maintainability · low] `.wk-admin-pager-btn` 中 `font-weight: 400;` 随后被 `font-weight: inherit;`
覆盖（源自原 Tailwind `font-normal` + `[font:inherit]` 简写展开的冗余），前者是死声明，会误导后续维护者以为生效值为 400。建议删除被覆盖的声明。

-   font-weight: 400;
    color: rgb(0 0 0/90%);
    font-family: inherit;
    font-size: inherit;
    line-height: inherit;
    font-weight: inherit;


─── apps/web/src/agent-marketplace/am-u.css:216-222 ───
[maintainability · low] `.wk-ava-11` / `.wk-amr-18` 把语义令牌 `bg-accent` 固化为 `#07c05f`，`.wk-amr-7` 把
`bg-surface` 固化为 `#ffffff`。已核实当前 `--color-accent`/`--color-surface` 为固定值（无暗色变体），故无即时视觉回归；但同批次
`administration-u.css:220` 已采用 `var(--color-accent, #07c05f)` 形式保持单一事实源。若后续令牌在 styles.css
中调整，此处硬编码副本会被遗漏，建议统一改为 var() + fallback 写法。

  .wk-ava-11 {
    border-radius: 4px;
-   background-color: #07c05f;
+   background-color: var(--color-accent, #07c05f);
    padding-inline: 12px;
    padding-block: 6px;
    color: #fff;
  }


─── apps/web/src/agent-marketplace/am-u.css:97-98 ───
[bug · low] `.wk-amr-14`（digest 展示）将原 `break-all` 译为 `overflow-wrap: anywhere`，两者断行语义不同：`word-break:
break-all` 在字符放不下时立即断行，`overflow-wrap: anywhere` 先尝试整词换行、仅当整词超过整行宽度才折行。对本例 64 位 hex digest
视觉差异极小，但作为"值 = utilities 编码的生效值"的机械迁移，忠实译法应为 `word-break: break-all`。

  .wk-amr-14 {
-   overflow-wrap: anywhere;
+   word-break: break-all;


─── apps/web/src/agent-marketplace/am-u.css:117-124 ───
[maintainability · low] 同一文件内两套前缀各持一份完全相同的规则：`.wk-amr-17` ≡ `.wk-ava-2`、`.wk-amr-18` ≡
`.wk-ava-11`、`.wk-amr-8` ≡ `.wk-ava-4`、`.wk-amr-4` ≡ `.wk-ava-12`、`.wk-amr-12` ≡ `.wk-ava-13`，且
`.wk-ava-self-start`/`.wk-amr-self-start` 为同一规则的重复命名。机械迁移阶段便于逐条校验可以理解，但定稿后任何样式调整都需同步改两处，建议合并为共享选择器（如
`.wk-amr-17, .wk-ava-2 { … }`）或抽取公共基类，并为编号类补充与其原 utility 的映射注释以便后续维护。

- .wk-amr-17 {
+ .wk-amr-17,
+ .wk-ava-2 {
    border-radius: 4px;
    border-style: solid;
    border-width: 1px;
    border-color: #dcdcdc;
    padding-inline: 8px;
    padding-block: 6px;
  }


─── apps/mobile/src/task-office-integration-smoke.ts:71-71 ───
[bug · medium] 此处 archive 已确认提交成功（office.archive 正常返回），restore 失败意味着用户的真实任务确定停留在归档态；而上方 archive
失败分支中，仅在"写可能已提交且 scope 变更"这一不确定性更低的情况下才标记 'cleanup-required'。确定需要人工清理的场景反而拿到语义更弱的 'failed'，以
'cleanup-required' 为告警键的消费方会漏掉这个最需要清理的状态。建议该分支改用 'cleanup-required'（或至少与 archive
失败分支的恢复失败语义保持一致并加以区分）。

-   if (restoreFailed) return { archiveRoundtrip: 'failed', archiveRestore: 'failed' };
+   // archive 已确认提交：restore 失败即确定遗留归档态，必须显式标记需人工清理。
+   if (restoreFailed) return { archiveRoundtrip: 'failed', archiveRestore: 'cleanup-required' };


─── apps/mobile/src/task-office-integration-smoke.ts:59-60 ───
[bug · low] 归档可见性只查了第一页：normalizeQuery 默认 limit=20（上限 100），集成账号累积超过一页归档任务时，新归档的 taskId 可能不在首页 items
中，导致 archivedVisible===false，最终把"archive+restore 均成功"误报为 archiveRoundtrip:'failed'（假阴性证据）。下方
restoredPage 的检查同理。建议跟随 nextCursor 翻页查找（office.moreTasks() 基于累积查询继续翻页），或至少显式传 limit:100 缩小窗口。

-     const archivedPage = await office.tasks({ archived: true });
+     let archivedPage = await office.tasks({ archived: true, limit: 100 });
+     archivedVisible = archivedPage.items.some((card) => card.taskId === taskId);
+     while (!archivedVisible && archivedPage.nextCursor !== undefined) {
+       archivedPage = await office.moreTasks();
-     archivedVisible = archivedPage.items.some((card) => card.taskId === taskId);
+       archivedVisible = archivedPage.items.some((card) => card.taskId === taskId);
+     }


─── apps/mobile/src/task-office-integration-smoke.ts:193-195 ───
[maintainability · low] 这个 catch 实际不可达：runArchiveRoundtrip 内部已把全部
await（archive、tasks、restore、scopeStillMatches 回调）包进 try/catch，函数体内 try 块之外没有任何可抛出语句。若未来重构使其可达，此处只回写
archiveRoundtrip 而把 archiveRestore 残留在初始值 'not-attempted'，但此时任务可能已被归档遗留——证据会误导排查。建议直接删除该兜底
catch，或同时补写 archiveRestore（如 'cleanup-required'）。

      } catch {
        evidence.archiveRoundtrip = 'failed';
+       evidence.archiveRestore = 'cleanup-required';
      }


─── apps/web/src/apps/apps-u.css:187-189 ───
[bug · high] `.wk-apps-25` 的 `line-height: 5` 是对原 Tailwind `leading-5` 的误译：`leading-5` =
`line-height: 1.25rem`（20px），而无单位 `5` 表示 5 倍字号（12px × 5 = 60px 行盒）。该 Tag 仍被
AuthorizationPage/ActionPage 的状态/风险徽标渲染，行盒远超 20px 定高容器——当前因 flex
居中（align-items:center）视觉上被部分掩盖，但基线已随之下移，一旦标签与同行文本混排或容器样式调整即会明显错位，且偏离了本文件声明的「值 = utilities
编码的生效值」事实源。建议改为 20px。另注意：同模式误译已扩散到其他迁移产物（data-sources-u.css:45、kb-u.css:90、platform-u.css:521 的
`line-height: 5`，experts-u.css:721 的 `line-height: 6`，均不在本次评审组），疑似平移脚本对 `leading-N` 的系统性误译，建议一并排查。

    padding-inline: 4px;
    font-size: 12px;
-   line-height: 5;
+   line-height: 20px; /* leading-5 = 1.25rem */


─── apps/web/src/apps/AppsPages.tsx:80-80 ───
[maintainability · medium] 死代码：本地 `Popconfirm` 组件（L69-90）在本次迁移后已无任何调用方——connections 页改用 tdesign 的
`TPopconfirm`，authorization/action 两页不涉及撤销确认，全文件搜索确认无 `<Popconfirm` 渲染点。连带 `apps-u.css` 中仅供其使用的
`.wk-apps-1` ~ `.wk-apps-4` 四条规则也成为死样式。建议整体删除组件与对应 CSS。

-   return <span className="wk-apps-1" ref={wrapper}>
+ /* 建议整体移除本地 Popconfirm 组件（connections 页已改用 tdesign TPopconfirm），
+    并同步删除 apps-u.css 中 .wk-apps-1 ~ .wk-apps-4 四条死样式。 */


─── apps/web/src/apps/AppsPages.tsx:161-164 ───
[bug · medium] rowKey 兜底丢失：旧实现对 catalog/installations 两表显式写了 `row.action_id ?? index`、`row.id ??
index`，而 `model.ts` 的 `appRows` 仅做类型 cast（`object()`），并不保证每行存在 `action_id`/`id` 字段。现在三处 TTable（本处
`rowKey="action_id"`、installations 与 connections 的 `rowKey="id"`）直接取字段值，若后端返回缺主键的行，将产生 undefined key
/ 重复 key（React key 告警、行复用错乱，在固定表头场景更明显）。tdesign Table 的 rowKey 支持函数形式，建议恢复 index 兜底。

          <TTable
-           rowKey="action_id"
+           rowKey={(row, index) => String(row.action_id ?? index)}
            data={catalog}
            columns={catalogColumns(t)}
+           /* installations 处同理：rowKey={(row, index) => String(row.id ?? index)} */


─── apps/web/src/apps/AppsPages.tsx:149-152 ───
[other · medium] 可访问性回退：catalog/connections 两页由 `<main className="wk-page">` + `<h1>` 降级为普通 `<div
className="apps-view">` + `<h2>`，而 PlatformShell 根容器是 `<div className="wk-shell-1">`（无
`<main>`/`role="main"` 地标），因此这两页现在整页缺失 main 地标与 h1；同时带 `role="status"` 的加载文案（common.loading）也被移除，仅剩
TTable 内置 spinner，加载状态不再被读屏器播报，且与仍在 PageFrame（`<main>` + role="status"）下的 authorization/action
两页行为不一致。若 Vue DOM 1:1 仅约束类名结构，建议将外层标签换回 `<main>`（class 不变）以保留语义地标。

-     <div className="apps-view">
+     <main className="apps-view">
        <div className="apps-view__header">
          <div className="apps-view__heading">
            <h2 className="apps-view__title">{t('apps.catalog.title')}</h2>
+           /* 标签改回 <main> 即可，Vue 同构的类名与布局不受影响；connections 页同理 */


─── apps/web/src/apps/AppsPages.tsx:126-126 ───
[maintainability · medium] 三层嵌套三元计算 state label（active/disabled/其他/空 四分支）违反「禁止嵌套三元表达式」规范，且是自旧
InstallationsTable 原样搬运。同文件 ConnectionsPage 已有 if-return 风格的 `stateLabel`
helper（L213）可作参照，建议提取独立函数消除嵌套。

-     { colKey: 'state', title: t('apps.catalog.colState'), width: 110, cell: ({ row }: { row: AppRow }) => { const state = String(row.state ?? '').trim(); const label = state === 'active' ? t('apps.common.stateActive') : state === 'disabled' ? t('apps.common.stateDisabled') : state ? t('apps.common.stateOther', { state }) : '—'; return <TTag theme={state === 'active' ? 'success' : 'default'} size="small">{label}</TTag>; } },
+ function installationStateLabel(state: string, t: AppsTranslate): string {
+   if (state === 'active') return t('apps.common.stateActive');
+   if (state === 'disabled') return t('apps.common.stateDisabled');
+   if (state) return t('apps.common.stateOther', { state });
+   return '—';
+ }
+ /* cell 内：const state = String(row.state ?? '').trim();
+    return <TTag theme={state === 'active' ? 'success' : 'default'} size="small">{installationStateLabel(state, t)}</TTag>; */


─── apps/web/src/apps/AppsPages.tsx:155-157 ───
[maintainability · low] 刷新按钮加载反馈回退：旧代码传 `loading={loading}`（按钮自带 spinner），迁移后仅
`disabled={loading}`（catalog L155 与 connections L243 两处），刷新期间缺少进行中视觉反馈，与留守页 PageFrame 的刷新按钮（仍传
loading）行为不一致。tdesign Button 支持 `loading` prop，建议补上。

-         <TButton variant="outline" disabled={loading} aria-label={t('apps.catalog.refresh')} onClick={() => void load()} icon={<TIcon name="refresh" />}>
+         <TButton variant="outline" disabled={loading} loading={loading} aria-label={t('apps.catalog.refresh')} onClick={() => void load()} icon={<TIcon name="refresh" />}>
            {t('apps.catalog.refresh')}
          </TButton>


─── apps/web/src/apps/AppsPages.tsx:95-95 ───
[maintainability · low] 死参数：迁移后唯一传 `gapClass="gap-4"` 的调用方 ConnectionsPage 已不再使用
PageFrame，AuthorizationPage/ActionPage 均不传该参数，`gapClass` 形参与 `.wk-apps-frame-gap`
常量现在只服务于一个无人覆写的默认值。建议删除该参数，把 `gap: 1.25rem` 直接并入 `.wk-apps-26`。

- function PageFrame({ title, description, loading, onReload, refreshLabel, loadingLabel, gapClass = 'wk-apps-frame-gap', children }: { title: string; description: string; loading: boolean; onReload: () => void; refreshLabel: string; loadingLabel: string; gapClass?: string; children: ReactNode }) {
+ function PageFrame({ title, description, loading, onReload, refreshLabel, loadingLabel, children }: { title: string; description: string; loading: boolean; onReload: () => void; refreshLabel: string; loadingLabel: string; children: ReactNode }) {
+   /* gap: 1.25rem 直接并入 .wk-apps-26，删除 gapClass 形参与 .wk-apps-frame-gap */


─── apps/web/src/apps/apps.td.css:117-119 ───
[maintainability · low] 命名误导：`connections-view__error` 在 React 端同时挂在 error（theme="error"）与
info（theme="info"）两个 TAlert 上，而规则值 `margin-top: 0` 对无默认外边距的 tdesign Alert 实际是空操作。类名只表达 error 语义却承载
info 提示，后续维护者容易误解其用途。建议改用中性的共享命名（如 `connections-view__notice`），或干脆去掉该空操作规则。

- .connections-view .connections-view__error {
+ /* React 端 error 与 info 两个 TAlert 共用，建议改为中性命名 */
+ .connections-view .connections-view__notice {
    margin-top: 0;
  }


─── apps/web/src/commercial/surface.tsx:9-13 ───
[maintainability · medium] 新增的 Card/Status 与既有共享兼容层 shared/wk-legacy.tsx（WkCard/WkStatus）语义完全重复：同为
<section>/<p> 渲染、相同 tone 色值（#b42318/#137333/#9a6700、13px、#506078）、相同 role=alert/status
映射；documents、knowledge-settings、platform、settings 等域均直接复用 wk-legacy。本域另起 wk-cs-* 类名形成平行副本，后续调色需同步
shared/commercial/configuration/data-sources 多处。另注意：文件头注释称"类值与 packages/ui/src/index.tsx:29-37
逐字一致"，但 .wk-cs-card 边框 #e7e7e7 实际取自 knowledge-u.css 的误置定义（packages/ui 原版为 #dce3ed，见 wk-legacy.css
头注），与 commercial-u.css:41-44 的说明相互矛盾。建议直接复用 shared/wk-legacy 的 WkCard/WkStatus（如需保留 #e7e7e7
生效值可为其扩展变体），并修正注释中的出处描述。



─── apps/web/src/commercial/surface.tsx:18-18 ───
[maintainability · medium] toneClass 使用三层嵌套三元表达式，违反项目规则"禁止嵌套三元表达式"，新增 tone 时需整链改写且易出错。tone
与类名后缀本就一一对应，可用模板字符串直接拼接（shared/wk-legacy.tsx 的 WkStatus 即此写法），同时也能消除与评论 1 重复实现中的差异点。

-   const toneClass = tone === 'error' ? 'wk-cs-status--error' : tone === 'success' ? 'wk-cs-status--success' : tone === 'warning' ? 'wk-cs-status--warning' : '';
+   const toneClass = tone === 'neutral' ? '' : `wk-cs-status--${tone}`;


─── apps/web/src/commercial/commercial-u.css:40-41 ───
[maintainability · low] .wk-bill-usage-number-cell 在本文件被定义了两次（第 19 行 text-align: right 与此处
font-variant-numeric: tabular-nums），同一选择器分裂两处，后续修改易改一漏一导致视觉回归。建议合并为单条规则。另：第 12 行
`border-bottom-width: 1px;border-color: #eef1f5;` 分号后缺空格，与文件其余规则格式不一致，可顺手统一。

- /* T15 语义化：原 tabular-nums（用量数值单元格）。 */
- .wk-bill-usage-number-cell { font-variant-numeric: tabular-nums; }
+ /* 与上文 .wk-bill-usage-table-cell 定义合并处保持单一定义： */
+ .wk-bill-usage-number-cell { text-align: right; font-variant-numeric: tabular-nums; }


─── apps/web/src/commercial/AdminCommercialPage.tsx:194-194 ───
[maintainability · low] 本域 5 个文件共 12 处 Button 均重复书写 theme="default" variant="outline"
样板属性，后续若需统一样式（如换 variant 或补 size）需多点修改。surface.tsx 已是本域 UI 封装点，建议在其中一并导出预设按钮组件（透传 tdesign Button 其余
props），将视觉决策收敛到一处。



─── apps/web/src/auth/LoginPage.tsx:253-260 ───
[bug · medium] toggleMode 未清空 email，与两处非受控 tdesign
Form（initialData=""）组合成状态失同步：登录/注册卡互斥渲染，模式切换即重挂载、email 输入框显示为空，但共享的 React state email
残留注册时输入的值。register→login 后用户仅补输密码提交，会以界面上不可见的旧邮箱发起登录请求（反向同理：注册卡 email 显示为空但 validateRegister 以残留
state 通过非空校验）。Vue 端 formData/registerData 是两份独立 v-model 数据且 toggleMode 清空全部
registerData（Login.vue:509-515），不存在此问题；login-page.test.tsx 也未覆盖切换后再提交的路径。建议在 toggleMode 中一并
setEmail('')，使 state 与重挂载后显示为空的表单一致（或改用受控 value 绑定）。

    function toggleMode() {
      // Vue toggleMode (Login.vue:509-515) — flips the card and clears register fields.
+     // React 侧两卡互斥渲染即重挂载（initialData="" 显示为空），共享 state 需同步清空，
+     // 否则 email 残留导致以界面不可见的旧邮箱提交。
      setMode((current) => (current === 'register' ? 'login' : 'register'));
      setState('idle');
      setMessage('');
      setFieldErrors({});
-     setUsername(''); setPassword(''); setConfirmPassword('');
+     setUsername(''); setEmail(''); setPassword(''); setConfirmPassword('');
    }


─── apps/web/src/auth/WorkspaceOnboardingPage.tsx:172-175 ───
[bug · low] 第二个弹窗迁移时丢失了 closeLabel={msg(locale, 'auth.workspaceOnboarding.close')}：WkDialog 的
closeLabel 默认值为英文 'Close'（wk-legacy.tsx:35），关闭按钮 × 的 aria-label 将不再随 locale 本地化，属
i18n/可访问性回退（迁移前为本地化文案）。WkDialog 完整支持该 prop，建议恢复传参。

        open={invitationsVisible}
        title={msg(locale, 'auth.workspaceOnboarding.invitations')}
        onClose={() => setInvitationsVisible(false)}
+       closeLabel={msg(locale, 'auth.workspaceOnboarding.close')}
        className="wk-onb-11"


─── apps/web/src/auth/LoginPage.tsx:369-369 ───
[bug · low] 语言菜单项由 <button role="menuitem"> 改为 <div role="menuitem" tabIndex={0}> 后仅处理 Enter：Space
键既不触发选中还会滚动页面，键盘可达性较迁移前回退（button 天然支持 Enter+Space，且 test 'language menu options are
keyboard-operable' 只测了 click 路径，未覆盖键盘）。建议补 Space 分支并 preventDefault，或改回原生 button 承载 menuitem。

-             <div key={option.value} role="menuitem" tabIndex={0} className={`language-option${option.value === locale ? ' active' : ''}`} onClick={() => selectLanguage(option.value)} onKeyDown={(event) => { if (event.key === 'Enter') selectLanguage(option.value); }}>
+             <div key={option.value} role="menuitem" tabIndex={0} className={`language-option${option.value === locale ? ' active' : ''}`} onClick={() => selectLanguage(option.value)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); selectLanguage(option.value); } }}>


─── apps/web/src/auth/LoginPage.tsx:404-404 ───
[bug · low] 轮播分页 bullet 由迁移前的 <button type="button" aria-label onClick> 改为纯 <span onClick>：span
不可聚焦、无键盘激活路径，键盘用户无法切换幻灯片（aria-label 也因不可聚焦而基本失效）。虽然手写复刻的 swiper 事实源 bullet 确为 span，但迁移前 React
实现可达性更好，建议至少补 tabIndex={0}、role="button" 与 Enter/Space keydown，保持既有键盘能力不回退。

-                 <span key={slide.titleKey} className={`swiper-pagination-bullet${index === slideIndex ? ' swiper-pagination-bullet-active' : ''}`} aria-label={t(slide.titleKey)} title={t(slide.titleKey)} onClick={() => setSlideIndex(index)} />
+                 <span key={slide.titleKey} role="button" tabIndex={0} className={`swiper-pagination-bullet${index === slideIndex ? ' swiper-pagination-bullet-active' : ''}`} aria-label={t(slide.titleKey)} title={t(slide.titleKey)} onClick={() => setSlideIndex(index)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSlideIndex(index); } }} />


─── apps/web/src/auth/login.td.css:193-201 ───
[maintainability · low] 两处冗余：① prefers-reduced-motion 块在 §1（此处，无 !important、保留节点 opacity .65）与 §2
末尾（!important + .animated-bg{display:none}）重复定义——文末块将 .animated-bg 整体隐藏后，本块的
animation:none/opacity:.65 均被短路不再产生效果，两块并存表达同一意图易误导后续维护；② 同文件 .swiper-slide 连续两条 transition-property
声明（transform → transform, opacity），首条为死代码。建议删除本块与 .swiper-slide 的首条 transition-property，统一由文末 Vue
平移块承载 reduced-motion 行为。

- @media (prefers-reduced-motion: reduce) {
-   .login-layout .knowledge-node {
-     animation: none;
-     opacity: 0.65;
-   }
-   .login-layout .connection-line {
-     animation: none;
-   }
- }
+ /* prefers-reduced-motion 统一由 §2 末尾块承载（!important + .animated-bg 整体隐藏），此处不再重复定义 */


─── apps/web/src/configuration/config-u.css:44-51 ───
[bug · medium] `.wk-cfg-ops-4 input` / `.wk-cfg-ops-4 textarea` 是为旧 @weknora/ui（直接渲染原生
input/textarea）准备的 `[&_input]:border…` utility 平移，但该类作用的两个表单（ModelDebugPanel 的 debug
表单、SkillOperations 的 register 表单）现已迁移到 tdesign Input/Textarea。tdesign 的边框、背景、内边距由外层
`.t-input`/`.t-textarea` 包裹层提供，内层 `.t-input__inner`/`.t-textarea__inner` 是无边框透明的 reset 样式（特异性
0-1-0）；本规则以 0-1-1 的特异性命中内层元素并重新叠加边框/背景/padding，会渲染出双边框与双重内边距，且与全站其他 tdesign 输入框风格不一致。同文件中
`.wk-cfg-field`（仅 width/box-sizing）和 `.wk-cfg-ed-4`（不含 input/textarea 边框规则）是正确参照。建议删除这两段元素选择器；若原生
file input 需要样式，用属性选择器单独限定。

- .wk-cfg-ops-4 input {
+ /* tdesign Input/Textarea 的边框、背景、内边距由外层 .t-input/.t-textarea 提供，
+    内层 .t-input__inner/.t-textarea__inner 需保持无边框 reset，删除整段覆盖；
+    原生 file input 如需样式请单独限定： */
+ .wk-cfg-ops-4 input[type="file"] {
    width: 100%;
    box-sizing: border-box;
-   border-style: solid;
-   border-width: 1px;
-   border-color: #cbd5e1;
-   border-radius: 6px;
-   background-color: #fff;
+ }


─── apps/web/src/configuration/config-u.css:258-266 ───
[maintainability · low] 本文件存在几处死声明/重复规则，虽为忠实平移旧 utility，但会误导后续维护：1) `.wk-cfg-page-16` 中 `font-size:
0.8rem` 与 `color: #6941c6` 恒被后面的 `font-size: 0.85rem !important` / `color: rgba(0,0,0,0.6)
!important` 覆盖，前两条永不生效；2) `.wk-cfg-ed-5` 在 `display: flex !important` 之下声明的 `grid-template-columns:
auto 1fr` 永不生效（flex 布局不消费该属性）；3) 若保留 `.wk-cfg-ops-4 input` 与 `.wk-cfg-ops-4
textarea`（见另一条评论），两段规则逐字相同，可合并为 `.wk-cfg-ops-4 input, .wk-cfg-ops-4
textarea`。建议清理恒被覆盖的声明，保持每个类只含生效值。

  .wk-cfg-page-16 {
    border-radius: 999px;
    padding-inline: 0.55rem;
    padding-block: 0.2rem;
-   font-size: 0.8rem;
-   color: #6941c6;
    background-color: #f4f3ff;
    margin-right: auto;
    font-size: 0.85rem !important;
+   color: rgba(0, 0, 0, 0.6) !important;
+ }


─── apps/web/src/chat/chat-header.tsx:188-188 ───
[bug · high] 置顶切换恒为无效操作：这里把「当前状态」传给 onTogglePin，而宿主 ChatRoutePage 的 toggleSessionPin(sessionId,
pinned) 语义是「目标状态」（pinned ? pin : unpin，见
ChatRoutePage.tsx:1412-1415）。同项目既有约定（packages/views/src/chat/session-sidebar.tsx:320）也是传取反后的下一状态。当前组
合下：未置顶点击「置顶」→ unpin（无变化），已置顶点击「取消置顶」→ pin（无变化），功能完全失效。

-                 {props.onTogglePin ? <button type="button" className="chat-header-menu__item" data-menu-action="pin" onClick={() => { setMenuVisible(false); props.onTogglePin!(Boolean(props.isPinned)); }}>
+ onClick={() => { setMenuVisible(false); props.onTogglePin!(!props.isPinned); }}


─── apps/web/src/chat/chat-header.tsx:115-119 ───
[bug · medium] 重命名失败对用户完全静默且丢失草稿：try 块在 await 之前已执行 setTitleEditing(false)，错误分支 setRenameError 设置的
renameError 只渲染在 .chat-header__edit-error（编辑表单内部）里，而表单此时已卸载；宿主 renameSession 失败时也不
toast（ChatRoutePage:1404-1410 直接抛出）。建议失败时保持编辑态、仅成功后退出，让 renameTitleFailed 真正可见：

-     } catch {
+     try {
+       await props.onRenameSession(title);
        setTitleEditing(false);
        setTitleDraft('');
+     } catch {
        setRenameError(props.renameTitleFailed);
-     } finally {
+       titleInputRef.current?.focus();
+     }


─── apps/web/src/chat/chat-header.tsx:161-161 ───
[bug · medium] onBlur 自动提交 + 空标题时 refocus 形成焦点陷阱：输入框为空时用户点击页面任意位置触发 blur → submitTitleEdit 走空标题分支 →
titleInputRef.current?.focus() 抢回焦点，用户无法自然离开编辑框（只能 Esc 或输入内容）。建议去掉空标题分支里的 refocus，允许 blur 即取消（或仅在
titleDraft 非空时提交）。



─── apps/web/src/chat/chat-header.tsx:177-178 ───
[maintainability · low] 死分支：if (context?.trigger === 'document') 分支与后面的兜底调用执行完全相同的
onMenuVisibleChange(visible)，条件判断没有任何效果。要么在 document 触发分支内做差异化处理（如额外重置状态），要么删除 if 直接调用。



─── apps/web/src/chat/chat-header.tsx:126-126 ───
[maintainability · low] 守卫条件脆弱：menuMode 类型为 'menu' | 'clear' | 'delete'，!menuMode 恒为 false 形同虚设；若未来在
'menu' 态意外触发 submitDangerAction，会落入 else 分支执行 onDeleteSession（把「清空」语义的操作变成「删除」）。建议显式限定确认态：

-     if (!menuMode || dangerBusy) return;
+     if ((menuMode !== 'clear' && menuMode !== 'delete') || dangerBusy) return;


─── apps/web/src/chat/chat-header.tsx:41-43 ───
[maintainability · low] renameSaving 是死 prop：仅在接口中声明，组件内从未读取（提交态由内部 renameBusy 驱动），ChatRoutePage
却每次传入 copy.renameSaving。要么用它替换保存按钮的 busy 文案（如「{renameSaving}…」），要么从 ChatHeaderProps 与调用方一并移除。



─── apps/web/src/chat/ChatRoutePage.tsx:1935-1936 ───
[bug · medium] 双重确认：ChatHeader 已在弹层内内置 clear/delete 的二次确认面板（menuMode 为 'clear'/'delete' 时的
chat-header-confirm），而这里的 clearMessages/deleteSession 内部又各自调用一次
window.confirm（ChatRoutePage.tsx:1420、1433）——用户执行一次删除要连续确认两次。建议为接入点提供无原生 confirm 的变体（或从宿主函数中移除
window.confirm，统一由 ChatHeader 弹层承担）。



─── apps/web/src/chat/ChatRoutePage.tsx:10-10 ───
[maintainability · medium] 聊天主路由静态引入整个 career 域模块（OpportunityPage 连带 ApplicationPage/reconciliation
与 api-client career 契约），且 conversationActionSlot
未做任何门控，在所有工作空间的会话视图与空态都常驻渲染「保存职位描述」表单；非求职空间的用户要等点击保存后才靠 forbidden 响应清空。已核验该面板无挂载即请求（全部由按钮触发）、scope
abort 有清理逻辑，红线未破——但建议用 React.lazy 按需加载或按工作空间类型门控，控制聊天首屏包体并降低 chat→career 的跨域静态耦合。



─── apps/web/src/chat/ChatRoutePage.tsx:1937-1938 ───
[maintainability · low] headerUtilityItems 已是双份传参：本次更新后它仅由 headerSlot 内的 ChatHeader 消费（此行正确）；而传给
ChatPage 的那份已是死代码——page.tsx 只声明该 prop（packages/views/src/chat/page.tsx:249）却在全文件无任何消费，fallback
头部也不渲染工具项。建议同步移除 ChatPage 的传参与 props 声明，避免后续维护者误以为旧头部仍在渲染这些工具项。



─── apps/web/src/chat/SessionShareDialog.tsx:6-7 ───
[documentation · low] 注释与实现已脱节：按钮已从内联 Tailwind utilities 迁移为 chat-u.css 的 wk-ssd-* 语义类，且新增了 import
'./chat-u.css'，「这里全部用内联 Tailwind utilities」不再成立（node 测试侧依赖各页面测试已有的 .css resolve stub
钩子）。另「packages/ui 旧栈 的」多出一个空格。建议按现状改写注释，说明本模块现在依赖 chat-u.css 及测试对 CSS 导入的 stub 要求。



─── apps/web/src/chat/chat.td.css:662-667 ───
[maintainability · low] .steer-failure 同一规则内 color 声明两次：首个 var(--td-text-color-secondary) 恒被末行的
var(--td-error-color) 覆盖（即使 --td-error-color 未定义，var() 失效也不会回退到前一条声明，而是继承/初始值），属于死声明。若为 1:1 平移 Vue
源的重复，建议删除首个 color 行，生效值不变且更清晰。

    font-size: 12px;
-   color: var(--td-text-color-secondary);
    margin-bottom: 4px;
    margin-top: 6px;
    color: var(--td-error-color);
- }


─── apps/web/src/chat/views-chat-u.css:2357-2364 ───
[maintainability · low] @keyframes pulse 在本文件定义了两次（page 段 wk-vc-page-54 之后一次、此处 message-list
段又一次），内容完全相同。重复定义后续易漂移（改一处漏一处），建议只保留一处（如文件头公共段），两处引用共用。



─── apps/web/src/commercial/commercial-u.css:23-30 ───
[maintainability · low] `.wk-bill-1`（表头行底边框）与
`.wk-bill-2`（font-semibold）是完全无语义的序号命名，无法从类名推断用途；同一批迁移中 UsagePanel 采用了 `usage-th` / `usage-td--num`
这类语义命名，本文件其余规则（`wk-bill-usage-table`、`wk-cs-status--error`）也都是语义化的，仅这两条例外。后续维护者看到 `wk-bill-2`
无法知道它是加粗，改样式时也难以检索。建议按同批迁移惯例语义化命名，如 `wk-bill-usage-head-row` / `wk-bill-usage-head-cell`。

- .wk-bill-1 {
-   border-bottom-style: solid;
-   border-bottom-width: 1px;border-color: #e7e7ea;
+ /* 表头行底边框（原 tailwind border-b border-[#e7e7ea]） */
+ .wk-bill-usage-head-row {
+   border-bottom: 1px solid #e7e7ea;
  }
  
- .wk-bill-2 {
+ /* 表头单元格加粗（原 tailwind font-semibold） */
+ .wk-bill-usage-head-cell {
    font-weight: 600;
  }


─── apps/web/src/commercial/BillingPage.tsx:54-55 ───
[documentation · low] 该注释已过时：USAGE_* 常量已从 tailwind utilities 改为语义类名（wk-bill-*），且参照对象 UsagePanel
也已迁移到 settings-wrapper.css 的 .usage-* 语义类（注释 "（tailwind utilities）" 两边都不再成立）。「同款样式」现在实际是分别复制在
commercial-u.css 与 settings-wrapper.css 的两份取值，注释应指向真实来源，避免误导后续维护者去查 tailwind。

- // 与 UsagePanel 模型表格同款样式（tailwind utilities）。
+ // 与 UsagePanel 模型表格同款样式（语义类，见 ./commercial-u.css；UsagePanel 对应 settings-wrapper.css 的 .usage-*）。
  const USAGE_TABLE = 'wk-bill-usage-table';


─── apps/web/src/data-sources/DataSourcesPage.tsx:453-453 ───
[bug · medium] 迁移到 tdesign 后表单控件的 required 全部移除,name/type/schedule 有 save() 中 `!form.name.trim() ||
!form.type.trim() || !form.schedule.trim()` 兜底,凭据字段有 firstMissingRequiredCredential,gitlab 有
projectRequired,但 VUE_SETTINGS_FIELDS 的必填 settings 字段(目前 rss 的 feed_urls,optional 未标记即为必填)在 save()
中没有任何校验——原 HTML required 防线移除后此字段成为唯一缺口。空 feed_urls 会因第 219 行 `!form.credentialsText.trim() &&
!rssAuthHeadersSerialized` 跳过 validateCredentials,直接把空配置 create 到服务端。建议在 save() 的凭据校验旁补充 settings
必填字段校验,复用 credentialValue 解析与 isRequired 提示。

-       <label>{t('dataSource.syncScheduleLabel')} <Input value={form.schedule} onChange={(value) => updateForm('schedule', String(value))} /></label>
+ // save() 中,与 firstMissingRequiredCredential 校验并列:
+ const missingSetting = (VUE_SETTINGS_FIELDS[form.type] ?? []).find((field) => !field.optional && !credentialValue(form.settingsText, field.key));
+ if (missingSetting) {
+   setMessage({ tone: 'warning', text: `${t(missingSetting.label)} ${t('dataSource.isRequired')}` });
+   return;
+ }


─── apps/web/src/data-sources/DataSourcesPage.tsx:441-441 ───
[bug · medium] 此处的 `wk-ds-self-start` 类(对应原 Tailwind justify-self-start)在 data-sources-u.css
中没有定义——全仓库唯一定义在 knowledge-u.css:698,而该文件仅被懒加载的 KnowledgeGraphPage.tsx 导入,用户未访问知识图谱页时此类不存在,prereq
的"打开控制台"链接在 .wk-ds-8 网格中会被默认 stretch 拉伸占满整行。这正是文件尾注释中 OCR R1-04 修复的同类问题(.wk-dsui-card
已因此迁入本域),但该类漏迁了,应同样补入 data-sources-u.css。

-           {guide.permissionPageUrl ? <a className="wk-ds-15 wk-ds-self-start" href={guide.permissionPageUrl} target="_blank" rel="noopener">{prereqCopy(t, `dataSource.prereqOpenConsole_${form.type}`, 'dataSource.prereqOpenConsole')}<span aria-hidden="true">↗</span></a> : null}
+ /* data-sources-u.css 迁入本域定义(与 .wk-dsui-card 同理) */
+ .wk-ds-self-start { justify-self: start; }


─── apps/web/src/data-sources/DataSourcesPage.tsx:451-451 ───
[bug · low] 原实现为 className="justify-self-start",迁移后变成空字符串:既丢失了 grid 布局(.wk-ds-4)下的
justify-self:start——tdesign Button 作为 grid item 会被默认 stretch 拉伸占满整行,造成视觉回归;空 className
本身也是无效残留。建议删除空 className 并补上与 prereq 链接相同的对齐类。

-       {editing && credentialStep.replaceMode ? <Button type="button" theme="default" variant="text" size="small" className="" onClick={() => { setCredentialStep((current) => credentialStepReducer(current, 'cancel-replace')); setForm((current) => ({ ...current, credentialsText: '', authHeaders: [] })); }}>{t('common.cancel')}</Button> : null}
+ <Button type="button" theme="default" variant="text" size="small" className="wk-ds-self-start" onClick={() => { ... }}>{t('common.cancel')}</Button>


─── apps/web/src/data-sources/data-sources-u.css:44-45 ───
[bug · medium] `.wk-ds-3` 对应原 Tailwind `text-xs leading-5`(12px 字号、20px 行高),但 `line-height: 5`
是无单位值,会按 5×12px=60px 解析,创建流程的连接器类型选择卡片描述文字行高被撑到 5 倍,卡片布局明显异常。应为 `line-height: 1.25rem`(即
20px)。注意这是迁移工具对数值型 leading 的系统性误编码:apps-u.css:189、kb-u.css:90、platform-u.css:521 同为 `line-height:
5`,experts-u.css:721 为 `line-height: 6`(leading-6→1.5rem),这些文件虽不在本评审组,建议一并排查修复。

    font-size: 12px;
-   line-height: 5;
+   line-height: 1.25rem; /* leading-5 生效值 = 20px */


─── apps/web/src/data-sources/data-sources-u.css:28-29 ───
[maintainability · low] 文件内存在多处被随后声明覆盖的死属性,与文件头注释"值 = utilities 编码的生效值"自相矛盾:(1)
.wk-ds-2/.wk-ds-42/.wk-ds-77 均先声明 `transition-duration: 150ms` 再被 `200ms` 覆盖(共 3 处);(2)
.wk-ds-25/.wk-ds-42/.wk-ds-68 先 `border-style: solid` 再被 `dashed` 覆盖;(3) .wk-ds-58
两个声明挤在同一行(`border-bottom-width: 1px;border-color: #e7e7e7;`),且 .wk-ds-58/.wk-ds-71 用全边 border-color
却只设置 border-bottom-width。不影响最终渲染,但会误导后续维护者,建议清理为最终生效值。

-   transition-duration: 150ms;
    transition-duration: 200ms;


─── apps/web/src/data-sources/data-sources-u.css:739-739 ───
[maintainability · low] 同一主色两种来源并存:本文件约 20 处硬编码 #2e6de6(含 color-mix 变体),而 .wk-ds-create-focus 又用
`var(--color-primary, #2e6de6)`——该令牌已在全局 styles.css:451 定义为 #2e6de6。主题令牌调整或暗色适配时硬编码处极易漏改,建议统一改为
var(--color-primary) 引用;同理 #b42318(danger)、#137333(success)等语义色也建议收敛到设计令牌。

- .wk-ds-create-focus:focus-visible { outline: 2px solid var(--color-primary, #2e6de6); outline-offset: 2px; }
+ .wk-ds-create-focus:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }
+ /* 其余 #2e6de6 → var(--color-primary) 统一替换 */


─── apps/web/src/apps/apps.td.css:102-104 ───
[bug · medium] 悬空 CSS 变量导致字体回退：`--td-font-family-code` 全仓（apps/web/src 与
packages）仅此一处引用、无任何定义——design-tokens/tdesign-theme.css 定义的字形 token 只有 `--td-font-family` /
`--td-font-size-*` 等（且 main.tsx 并未引入 tdesign 官方基础样式表），因此该规则运行时实际解析为裸 `monospace`。而迁移前此处的 Tailwind
`font-mono` 解析为 styles.css:387 的 `--font-mono: var(--app-font-family-mono, ui-monospace,
SFMono-Regular, Menlo, Monaco, Consolas, monospace)`，且本文件头注声称「--td-* 变量原样平移（T4 token
已接线）」——该变量并未接线。结果是 connections 页 ID 列的等宽字体与应用其余等宽处（如同文件 AuthorizationPage 的
`.wk-apps-12`）不一致，中英文环境下裸 monospace（Courier/宋体等宽）与 SF Mono/Menlo 栈差异明显。建议改用已接线的应用等宽栈。

  .connections-view .connections-view__mono {
-   font-family: var(--td-font-family-code, monospace);
+   font-family: var(--app-font-family-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace);
  }


─── apps/web/src/apps/AppsPages.tsx:370-370 ───
[maintainability · medium] 重复逻辑 + 三层嵌套三元：本行内联重写了 read/write/send/delete → Tag theme
的映射（三层嵌套三元，违反「禁止嵌套三元表达式」规范），而本次迁移已在同文件 L131 抽出了行为完全相同的 `riskThemeOf(risk)` helper（catalogColumns
即用它）。风险等级语义今后若调整（如新增 risk 档位），此处极易与 helper 漂移。L360 的 `riskLabel` 内联 IIFE 同样与 L138 的 `riskLabelOf`
重复，可一并收敛。

-       <div><dt className="wk-apps-11">{t('apps.actions.riskLabel')}</dt><dd className="wk-apps-13" title={riskLabel ? undefined : t('apps.actions.riskUnknownHint')}>{riskLabel ? <Tag theme={riskValue === 'read' ? 'success' : riskValue === 'write' ? 'warning' : riskValue === 'send' || riskValue === 'delete' ? 'danger' : 'default'}>{riskLabel}</Tag> : <span className="wk-apps-16">—</span>}</dd></div>
+       <div><dt className="wk-apps-11">{t('apps.actions.riskLabel')}</dt><dd className="wk-apps-13" title={riskLabel ? undefined : t('apps.actions.riskUnknownHint')}>{riskLabel ? <Tag theme={riskThemeOf(riskValue)}>{riskLabel}</Tag> : <span className="wk-apps-16">—</span>}</dd></div>


─── apps/web/src/apps/AppsPages.tsx:218-218 ───
[style · low] 嵌套三元（active/revoked/其他 三分支）：theme 计算用了二层嵌套三元，违反「禁止嵌套三元」规范；同文件 L213 的 `stateLabel` 已是
if-return 风格，可对称提取 theme helper。同页 L232 ops 单元格的 `canManage && row.state === 'active' ? (...) :
row.state === 'revoked' ? … : '—'` 也是同款二层嵌套三元，建议一并处理。

-     { colKey: 'state', title: t('apps.connections.colState'), width: 110, cell: ({ row }: { row: AppRow }) => <TTag theme={row.state === 'active' ? 'success' : row.state === 'revoked' ? 'danger' : 'default'} size="small">{stateLabel(row.state)}</TTag> },
+   const stateThemeOf = (state: unknown): 'success' | 'danger' | 'default' => {
+     if (state === 'active') return 'success';
+     if (state === 'revoked') return 'danger';
+     return 'default';
+   };
+   // …
+     { colKey: 'state', title: t('apps.connections.colState'), width: 110, cell: ({ row }: { row: AppRow }) => <TTag theme={stateThemeOf(row.state)} size="small">{stateLabel(row.state)}</TTag> },


─── apps/web/src/experts/experts-u.css:717-722 ───
[bug · high] `.wk-exp-37`（取消发布确认弹窗标题）将原 Tailwind v4 `leading-6` 误译为无单位的 `line-height: 6`。v4 中
`leading-6` 走 spacing 刻度（6 × 0.25rem = 24px），而无单位值 6 表示 6 倍字号（16px × 6 = 96px），标题行框将被撑至约
96px，弹窗布局明显错位。同批已完成平移的 u.css 对 `leading-6` 一律译为 `line-height:
24px`（market-u.css:109、views-chat-u.css:789 等），应保持一致。

  .wk-exp-37 {
    margin-bottom: 8px;
    font-size: 16px;
    font-weight: 600;
-   line-height: 6;
+   line-height: 24px;
  }


─── apps/web/src/experts/experts-u.css:820-822 ───
[bug · high] “死样式”论断与迁移前的真实级联不符，选中态样式被误删。迁移前 styles.css 以 `layer(utilities)` 引入
tailwindcss/utilities.css、token 来自 packages/ui theme.css 的 @theme 块（本 PR 已删，见
styles.css/package.json diff），三个 `aria-selected:*` utility
与基类同层：`.aria-selected\:*[aria-selected="true"]` 的属性选择器优先级（0,2,0）高于
`border-[#e7e7ea]`/`bg-surface`/`text-[…]`（0,1,0），且当时不存在任何 unlayered 规则命中这些按钮——选中页签实际渲染为绿描边 +
accent-wash 底 + 绿字。“layered utilities 恒输 unlayered 基类”仅对“基类已落 u.css 而 utility
尚未摘除”的中间态成立，该中间态从未发布。现状：ExpertsPage.tsx:515 仍输出 aria-selected="true"，但选中页签与未选中完全同貌，页面三个 role="tab"
主导航失去激活指示（视觉与 aria 状态脱节）。对比同文件对 aria-pressed（wk-exp-48/49）按状态补齐了样式，此处应同样补齐规则并更正注释。

- /* T15 勘误：原 aria-selected:* 三联 utility 为死样式——layered utilities 恒输
-    unlayered 的 .wk-exp-xp-tab 基类（选中态实际渲染=基类白底灰描边，仅 :hover
-    变绿）。语义化时不补规则，保持生效值不变。 */
+ /* 原 aria-selected:border-accent/bg-accent-wash/text-accent 迁移前同层生效
+    （属性选择器优先级 0,2,0 > 基类 0,1,0），此处补齐选中态。 */
+ .wk-exp-xp-tab[aria-selected="true"] {
+   border-color: #07c05f;
+   background-color: rgba(7, 192, 95, 0.08);
+   color: #07c05f;
+ }


─── apps/web/src/experts/experts-u.css:56-60 ───
[maintainability · low] 迁移将语义 token
译为字面量：accent→#07c05f、accent-wash→rgba(7,192,95,0.08)、surface→#ffffff，散布于
.wk-exp-xp-tab/.wk-exp-xp-card/.wk-exp-xp-btn-*/.wk-exp-48/49 等多处，而 .wk-exp-8/22/26/36 又保留
var(--wk-bg,#fff)——译法不统一。经核实 --color-accent/--color-surface 当前为 styles.css
中的静态字面量（:root，全库无重定义），计算值确实一致，非功能回归；但同批迁移文件（administration-u.css:220、kb-list.td.css:2476、settings-wr
apper.css）均保留 var(--color-accent, #07c05f) 形式的间接引用，且 --color-brand 等别名已链到 --wk-*（随暗色变化）——若后续
accent/surface 同样接入主题链，本域将静默脱钩。建议统一改用 var(--color-accent, #07c05f) 等带回退的写法。

  .wk-exp-xp-tab:hover {
    border-style: solid;
-   border-color: #07c05f;
-   color: #07c05f;
+   border-color: var(--color-accent, #07c05f);
+   color: var(--color-accent, #07c05f);
  }


─── apps/web/src/experts/experts-u.css:410-411 ───
[style · low] .wk-exp-9/.wk-exp-22/.wk-exp-34 三处出现同一行挤两条声明的生成痕迹（`border-bottom-width:
1px;border-color: …`），且用全边 border-color 而非 border-bottom-color——当前这些元素只声明了底边框，语义上应为
border-bottom-color，否则后续若补其他边描边会被误伤。建议拆行并改用边维度的 color 属性。

    border-bottom-style: solid;
-   border-bottom-width: 1px;border-color: rgba(127,127,127,0.2);
+   border-bottom-width: 1px;
+   border-bottom-color: rgba(127,127,127,0.2);


─── apps/web/src/configuration/ConfigurationOperations.tsx:0-0 ───
[bug · low] tdesign Input 的只读属性是小写 `readonly`，而非 React DOM 的驼峰 `readOnly`。同一迁移中
ConfigurationEditor.tsx 的 `autoComplete` 已改为小写 `autocomplete` 以适配 tdesign props，此处 `readOnly` 漏改。若
tdesign Input 未把该未知 prop 透传到内层 input，"Share permission" 这个只读展示框会变为可编辑（用户可改动显示值，产生可改权限的误导，尽管 share()
提交的 permission 固定为 'viewer'）。建议统一改为 tdesign 规范的 `readonly`。

- <label>Share permission<Input value="Viewer (read-only)" readOnly /></label>
+ <label>Share permission<Input value="Viewer (read-only)" readonly /></label>


─── apps/web/src/chat/views-chat-u.css:1697-1707 ───
[bug · medium] 平移数值回归：`.wk-vc-tool-result-6` 对应 tool-result.tsx DatabaseQueryRenderer 的 `<th>`，原
Tailwind utility 是 `border-b-2 border-b-[#e3e8ef]`（2px 下边框，见本次 tool-result.tsx diff 中被替换的旧类串），这里写成了
`border-bottom-width: 8px`，表头下边框变成 4 倍粗的 8px 色带。且本规则无任何 T15 语义化注释，与文件头「值 = utilities
编码的生效值」的约定矛盾。应改回 2px。

  .wk-vc-tool-result-6 {
    white-space: nowrap;
    border-bottom-style: solid;
-   border-bottom-width: 8px;
+   border-bottom-width: 2px;
    border-bottom-color: #e3e8ef;
    background-color: #f6f8fa;
    padding-inline: 0.6rem;
    padding-block: 0.4rem;
    text-align: left;
    font-weight: 600;
  }


─── apps/web/src/chat/chat-header.tsx:5-5 ───
[maintainability · low] CSS 导入指向了错误的样式表：ChatHeader/SandboxHeaderToggle
使用的全部类名（.chat-header*、.chat-header-menu*、.chat-header-confirm*、.sandbox-header-toggle*）都定义在
chat.td.css（§1/§2），views-chat-u.css 里没有任何 chat-header 规则（已搜索确认）。当前能正常显示只因宿主 ChatRoutePage 自己 import
了 './chat.td.css'；若该组件被单独引用（测试/Storybook/其他页面），样式会静默丢失，而这份对 views-chat-u.css 的导入既无作用又与 ChatRoutePage
的同名导入重复。建议改为 import './chat.td.css'（使组件自足），或直接删除该导入。

- import './views-chat-u.css';
+ import './chat.td.css';


─── apps/web/src/chat/ChatRoutePage.tsx:1931-1932 ───
[maintainability · low] 同一查找重复三次：`sessions.find((session) => session.id === selectedSessionId)` 在
title、isPinned、renameTitle 三行各执行一遍，且 title 与 renameTitle 的取值表达式完全相同。建议在 return 前提取 `const
selectedSession = sessions.find(...)`，或者干脆让 ChatHeader 内部用 props.title 派生
renameTitle（两者语义本就应一致），避免列表增长时三次 O(n) 扫描与表达式漂移。

-         title={sessions.find((session) => session.id === selectedSessionId)?.title || copy.newSession}
-         isPinned={sessions.find((session) => session.id === selectedSessionId)?.is_pinned === true}
+         title={selectedSessionTitle}
+         isPinned={selectedSession?.is_pinned === true}
+         renameTitle={selectedSessionTitle}


─── apps/web/src/chat/views-chat-u.css:220-233 ───
[maintainability · low] 同规则内死声明成片（与已确认的 .steer-failure 重复 color、pulse 重复定义同类，均为本文件新实例）：
1) `.wk-vc-page-23`：`font-size: inherit` 被末行 `font-size: 13px` 覆盖，inherit 恒不生效；
2) `.wk-vc-page-52/-53`、`.wk-vc-composer-18/-20/-22/-24`：`transition-duration: 150ms` 后紧跟
`200ms`，150ms 恒不生效；
3) `.wk-vc-tool-result-2/-14/-15/-25`：普通 `font-size`/`color` 声明被后面的同名 `!important` 行覆盖，恒不生效。
后续维护者无法分辨哪行是生效值，改错行不会有任何视觉反馈。建议每条规则只保留生效值（150ms/inherit/0.7rem 等死行删除）。

  .wk-vc-page-23 {
    min-height: 40px;
    resize: none;
    border-radius: 6px;
    border-style: solid;
    border-width: 0;
    padding-inline: 8px;
    padding-block: 6px;
    font-family: inherit;
-   font-size: inherit;
+   font-size: 13px;
    line-height: inherit;
    font-weight: inherit;
-   font-size: 13px;
  }


─── apps/web/src/auth/LoginPage.tsx:432-432 ───
[bug · high] onEnter 与表单隐式提交叠加导致一次 Enter 双重提交：tdesign Form 渲染原生 <form class="t-form"> 且卡内已有 Button
type="submit"（login-page.test.tsx:106-108/224 已实证该原生提交链路），真实浏览器在文本/密码框按 Enter 本就会触发隐式提交；tdesign
Input 的 onEnter 是纯回调、不做 preventDefault。于是一次 Enter 会先后执行 onEnter → submit() 与 form submit → Form
onSubmit → submit(e)，而 submit 无重入保护（setState 异步，闭包内 state 恒为旧值）：登录将发起两次
client.auth.login（onAuthenticated 被调用两次），注册将发起两次 client.auth.register——第二次常因"用户已存在"失败，在注册成功 toast
后紧接弹错误 toast。注册卡 confirmPassword（行 501）同样存在。建议给 submit 加 ref 重入锁（无论 tdesign 是否阻止默认行为都安全），或直接去掉
onEnter 改由隐式提交覆盖 Enter 路径。

- <Input placeholder={t('auth.passwordPlaceholder')} type="password" autocomplete="current-password" size="large" disabled={loading} onEnter={() => { void submit(); }} {...ariaFor('password')} />
+ const submittingRef = useRef(false);
+ async function submit(event?: { preventDefault?: () => void }) {
+   event?.preventDefault?.();
+   if (submittingRef.current) return;
+   submittingRef.current = true;
+   try {
+     // …原校验/请求逻辑（含提前 return 的校验失败分支）…
+   } finally {
+     submittingRef.current = false;
+   }
+ }


─── apps/web/src/auth/LoginPage.tsx:426-427 ───
[bug · medium] 登录卡 email 字段固定 initialData="" 使"注册成功后预填邮箱"行为丢失：submit() 注册成功路径按注释（Vue
Login.vue:744-746 "switch to login and prefill the email"）保留 state.email 并
setMode('login')，登录卡恰在此刻重新挂载，而非受控 tdesign Form 以 initialData 空串起步——邮箱框显示为空。迁移前旧实现为受控
value={email}，能完成预填；现在用户看不到邮箱、但再次提交登录时 submit 仍以 state.email 发起请求（与已确认的 toggleMode 未清 email
问题同源的"界面与 state 失同步"）。建议改为 initialData={email}：该 Form 只在卡片切换时挂载，恰可取到当前 state（配合 toggleMode 清空 email
后，常规路径仍为空串，行为不变）。

- <Form.FormItem label={t('auth.email')} name="email" requiredMark initialData="">
-                 <Input placeholder={t('auth.emailPlaceholder')} type="text" autocomplete="email" size="large" disabled={loading} {...ariaFor('email')} />
+ <Form.FormItem label={t('auth.email')} name="email" requiredMark initialData={email}>


─── apps/web/src/data-sources/DataSourcesPage.tsx:477-478 ───
[bug · high] tdesign Dialog 确认按钮的默认生命周期:onConfirm 返回值不是 false / rejected Promise 时,点击后会继续触发
onClose(官方文档:"当 onConfirm 返回值是 rejected promise 或 resolve false 时,会阻止弹窗关闭";基础示例仅靠 onClose 即可关闭
footer 按钮)。此处 `() => void confirmDelete()` 返回 undefined,所以用户点击"删除/删除并清除"的瞬间 Dialog 就经 onClose →
setDeleteSource(null) 立即关闭:confirmBtn.loading、cancelBtn.disabled、purge checkbox
disabled={deleteSubmitting} 三处提交期防护永远不会出现(死交互),删除请求在无任何进行中反馈的情况下后台执行,与注释声明对齐的 Vue
DataSourceSettings.vue 行为(loading 期间保持打开)相悖。建议返回 false 阻止自动关闭,交由 confirmDelete 成功路径的
setDeleteSource(null) 关闭;同时注意失败路径 Dialog 会保持打开,而错误 message 渲染在主页 Card 中会被 modal 遮罩遮挡,需把失败反馈挪进 Dialog
内(或关闭后 toast)才完整。

      onClose={() => setDeleteSource(null)}
-     onConfirm={() => void confirmDelete()}
+     onConfirm={() => { void confirmDelete(); return false; }}


─── apps/web/src/data-sources/data-sources-u.css:360-365 ───
[bug · low] `.wk-ds-42:hover` 里的 `border-style: solid` 是迁移回归:原 Tailwind 为 `border border-dashed ...
hover:border-primary`,hover 只改边框颜色、保持虚线。现在 hover 时虚线卡片(空态"添加数据源"按钮与列表尾按钮)会变实线。应删去 hover 内的
border-style 声明;同理 `.wk-ds-2:hover`、`.wk-ds-78:hover` 中的 `border-style: solid`
相对基线(solid)是冗余声明,建议一并清理。

  .wk-ds-42:hover {
-   border-style: solid;
    border-color: #2e6de6;
    background-color: color-mix(in srgb, #2e6de6 5%, transparent);
    color: #2e6de6;
  }


─── apps/web/src/knowledge-settings/parserSettings.tsx:226-230 ───
[bug · medium] aria-label 挂在无 role 的包裹 <span> 上：无角色的 span 不参与可访问名称计算，屏幕阅读器不会把该标签关联到内部 tdesign Select
渲染的 combobox，相比原先 <select aria-label={group.label}>（每个解析器行都有可访问名称）是无障碍标注回归。data-parser-group 挂 span
是台账 #8 的既定约定（Select 根不透传 data-*），但 aria-label 不必随之失效——给 span 补 role="group" 即可让标签纳入可访问树，或尝试直接传给
Select 由其透传。

                <span
                  data-parser-group={group.key}
+                 role="group"
                  aria-label={group.label}
                  style={{ display: 'block' }}
                >


─── apps/web/src/knowledge-settings/parserSettings.tsx:11-11 ───
[maintainability · low] 冗余导入：本组件未使用 knowledge-settings-u.css 中的任何 wk-kss-* 类（行样式全部为内联
style，wkbs-parser-* 类名也无对应 CSS 规则），该文件唯一的消费方 KBShareSettingsSection.tsx
已自行导入。疑似迁移脚本为域内所有改动文件统一附加的无效导入，建议移除，避免误导后续维护者以为本组件依赖 u.css。



─── apps/web/src/knowledge-settings/KnowledgeSettingsPage.css:831-831 ───
[style · low] border-color: none 是非法 CSS 值（border-color 仅接受 <color>{1,4}），整条声明会被浏览器丢弃；且
.kb-text-input 自身 border: 0，该声明即使合法也无效果。focus 视觉已由外层 .kb-text-input-wrap:focus-within 承担，此处应删除
border-color: none（或连同整条 :focus 规则一并精简为仅 outline），避免误导后续维护者以为 input 自带 focus 边框逻辑。

- .kb-text-input:focus { outline: none; border-color: none; }
+ .kb-text-input:focus { outline: none; }


─── apps/web/src/faq/FAQPage.tsx:982-983 ───
[bug · medium] seq_id 过滤缺失：contracts 中 `KnowledgeTag.seq_id?: number`
为可选字段（packages/contracts/src/index.ts:200），本文件 975 行的展示映射保留了 `typeof tag.seq_id === 'number'`
防御，但这两处选项数组没有。若标签缺 seq_id：卡片下拉的 value 为字符串 "undefined"，选中后经 onEntryTagChange → Number("undefined") =
NaN，NaN === previousTagId 恒为 false，faq.updateTags 序列化 NaN 为 null，会**静默清空该条目标签并弹成功提示**；TdSelect 侧
value: undefined 也会渲染出选中值异常的选项。建议过滤缺 seq_id 的标签，并在 updateEntryTag（2345 行附近）补 NaN 防护。

-   const tagDropdownOptions = tags.map((tag) => ({ content: tag.name, value: String(tag.seq_id) }));
-   const tagSelectOptions = tags.map((tag) => ({ label: tag.name, value: tag.seq_id }));
+   const selectableTags = tags.filter((tag) => typeof tag.seq_id === 'number');
+   const tagDropdownOptions = selectableTags.map((tag) => ({ content: tag.name, value: String(tag.seq_id) }));
+   const tagSelectOptions = selectableTags.map((tag) => ({ label: tag.name, value: tag.seq_id }));
+   // updateEntryTag 内同步防护：
+   // if (normalized !== null && !Number.isSafeInteger(normalized)) return;


─── apps/web/src/faq/FAQPage.tsx:1561-1561 ───
[bug · low] i18n key 误用：共享目录中 `general.close` 的值是「关闭设置」/"Close Settings"（设置面板语义，见
packages/i18n/src/settings.ts），用作弹窗关闭按钮的 aria-label 语义不符；同文件 611 行的关闭按钮用的是
`common.close`（「关闭」）。两处弹窗关闭按钮（此行与 1672 行 batch-tag-close-btn）应统一改用 `common.close`。另外本文件 1561-1564 与
1672-1675 的内联关闭 SVG 完全相同，可提取为一个局部常量（或直接复用 TIcon name="close"）减少重复。

-               <button className="close-btn" aria-label={t('general.close')} onClick={() => onCloseImport()}>
+               <button className="close-btn" aria-label={t('common.close')} onClick={() => onCloseImport()}>


─── apps/web/src/faq/FAQPage.tsx:1253-1255 ───
[bug · medium] 可访问性回归：旧实现的三点按钮是 `<button aria-haspopup="menu" aria-expanded aria-label>`，此处改为裸 div
且丢失全部 aria 标注——键盘用户无法聚焦触发，读屏用户无从得知按钮用途；1242/1246 行的 popup-menu-item（编辑/删除）同样是 div
onClick，删除这类破坏性操作对键盘用户完全不可达（1838 行 result-header 也由 button 改为 div onClick）。虽然 kb/orgs 等域也用 div
菜单，但至少应恢复触发器的 button 语义与 aria-label，菜单项建议改回 button 或补 role/tabIndex/onKeyDown。

-                                     <div className="card-more-btn" onClick={(event) => event.stopPropagation()}>
+                                     <button type="button" className="card-more-btn" aria-label={t('knowledgeBase.columnActions')} aria-haspopup="menu" onClick={(event) => event.stopPropagation()}>
                                        <img className="more-icon" src={MORE_PNG} alt="" />
-                                     </div>
+                                     </button>


─── apps/web/src/faq/FAQPage.tsx:1128-1133 ───
[bug · low] 清除按钮可达性问题：`showTagFilterClear = activeTagIds.length > 0 && tagFilterTriggerHover`
使清除入口仅在鼠标悬停时渲染——触屏设备（无 hover）与键盘用户永远看不到它；且该 span 只有 aria-label，缺少
role="button"/tabIndex/键盘事件（旧实现三者俱全）。建议补充 role/tabIndex/onKeyDown，并在可见性条件中加入触发器聚焦态（如 focus-within 或
trigger focus state），保证非鼠标用户可以清空标签筛选。

                              <span
+                               role="button"
+                               tabIndex={0}
                                className="t-input__suffix t-input__suffix-icon t-input__clear"
                                aria-label={t('common.clear')}
                                onClick={(event) => { event.stopPropagation(); setTagPanelOpen(false); setTagFilterCleared(true); onClearTagFilter(); }}
+                               onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); event.stopPropagation(); setTagPanelOpen(false); setTagFilterCleared(true); onClearTagFilter(); } }}
                                onMouseDown={(event) => event.stopPropagation()}
                              >


─── apps/web/src/faq/FAQPage.tsx:620-620 ───
[style · low] 嵌套三元表达式（规范明确禁止）：此处为三层嵌套，另有 1209 行（tag theme 两层）与 1659 行（导入按钮文案两层）。建议提取映射表，可读性和可测试性都更好。

-               name={importTask.status === 'running' ? 'loading' : importTask.status === 'success' ? 'check-circle-filled' : importTask.status === 'failed' ? 'error-circle-filled' : 'time-filled'}
+   // 模块级映射：const IMPORT_TASK_ICON: Record<string, string> = { running: 'loading', success: 'check-circle-filled', failed: 'error-circle-filled' };
+   name={IMPORT_TASK_ICON[importTask.status] ?? 'time-filled'}


─── apps/web/src/faq/FAQPage.tsx:2177-2181 ───
[maintainability · low] message prop 成为死参数：错误提示已完全 toast 化（本 effect），FAQPageView 内已无任何
`<Status>`/`message?.tone` 渲染残留，但 FAQPageView 仍在 790 行解构 `message = null` 且 FAQPage 在 2466 行传入
`message={message}`——这条 prop 链已无消费者，会误导读者以为视图还有内联错误区。建议从 FAQPageViewProps 中移除 message
及其传参；若为兼容测试快照保留，请加注释说明。

    useEffect(() => {
      if (!message) return;
      const theme = message.tone === 'error' ? 'error' : message.tone === 'success' ? 'success' : 'warning';
      MessagePlugin[theme](message.text);
    }, [message]);
+   // 并删除 FAQPageViewProps.message 与 <FAQPageView message={message} ...> 传参


─── apps/web/src/faq/faq.td.css:2240-2244 ───
[bug · medium] 非法 CSS 语法：`var(--td-brand-color)1a` 与 `var(--td-brand-color)33` 不合法——CSS
变量替换后不能直接拼接十六进制透明度后缀，这两条声明会被整体丢弃，`.answer-tag`（检索结果答案标签）退化为 TDesign Tag 默认配色，与 Vue 事实源不一致。0x1a/0x33
约为 10%/20% 透明度，建议改用 color-mix()（本文件 .tag-filter-chip.active 已有先例）。

  .answer-tag {
-   background: var(--td-brand-color)1a;
+   background: color-mix(in srgb, var(--td-brand-color) 10%, transparent);
    color: var(--td-brand-color);
-   border-color: var(--td-brand-color)33;
+   border-color: color-mix(in srgb, var(--td-brand-color) 20%, transparent);
  }


─── apps/web/src/faq/faq.td.css:43-51 ───
[maintainability · low] Vue Transition
残留类：`.fade-enter-*`/`.modal-enter-*`/`.slide-down-*`/`.faq-batch-bar-fade-*` 是 Vue `<Transition>`
钩子类名，React 端（FAQPage.tsx 无任何对应类切换/过渡库）永远不会给节点加上这些类，属于惰性死样式；且
`.fade-enter-active`、`.modal-*`、`.slide-down-*`
均无页面前缀、直接暴露在全局作用域，若其他组件或三方库恰好使用同名类会被意外命中。建议删除这批过渡块（或至少加上 `.faq-` 前缀限定作用域）。

- .fade-enter-active,
- .fade-leave-active {
-   transition: opacity 0.15s ease;
- }
- 
- .fade-enter-from,
- .fade-leave-to {
-   opacity: 0;
- }
+ /* 删除 .fade-* / .modal-enter-* / .slide-down-* / .faq-batch-bar-fade-* 过渡块； */
+ /* 如需保留平移对照，至少加前缀：.faq-manager .fade-enter-active { ... } */


─── apps/web/src/market/market-u.css:653-654 ───
[maintainability · low] 勘误注释的结论经核实成立：全局确无其他样式源接管这些页签的选中态（platform-shell.css 的 `.platform-palette
button[aria-selected=true]` 有作用域限定，wk-legacy.css 等均无相关规则），且与 experts 域（experts-u.css:820
同款勘误）行为一致。但实际效果是：market/tenant 主页签与排行页签（role=tab + aria-selected）选中时没有任何视觉区分——hover
有反馈而选中态反而没有，视觉用户无从辨识当前页签。这是迁移前即存在、本次刻意保留的缺陷；既然样式所有权已落到本文件，补一条选中态规则成本极低，建议顺手补齐（或至少把该 UX
债务显式记入勘误说明，避免后续域迁移继续复制该缺陷）。

  /* T15 勘误：原 aria-selected:* 三联 utility 为死样式（同 experts：unlayered
     .wk-mkt-mk-tab 基类恒胜），不补规则保持生效值。 */
+ .wk-mkt-mk-tab[aria-selected="true"] {
+   border-color: #07c05f;
+   background-color: rgba(7, 192, 95, 0.08);
+   color: #07c05f;
+ }


─── apps/web/src/market/market-u.css:378-379 ───
[style · low] wk-mkt-13 与 wk-mkt-32 中 `border-bottom-width: 1px;border-color: ...` 两条声明挤压在同一行（第 543
行 wk-mkt-32 的 `border-top-width: 1px;border-color: ...` 同样），与文件其余“一声明一行”的格式不一致，疑似生成/拼接瑕疵，建议拆行。另外
:hover/:disabled 等状态块重复声明基类已有的 `border-style: solid`（如第 44/57/195/208 行）属无害冗余，可顺手清理。

    border-bottom-style: solid;
-   border-bottom-width: 1px;border-color: rgba(127,127,127,0.2);
+   border-bottom-width: 1px;
+   border-color: rgba(127,127,127,0.2);


─── apps/web/src/market/MarketPage.tsx:539-539 ───
[maintainability · low] 此处 toast.tone 的双层嵌套三元（以及下方安装进度行 `failed || failedToStart ? ... : ready ? ...
: ...` 的同类嵌套）违反项目“禁止嵌套三元表达式”的代码质量规约。结构虽承袭自原有代码，但本行已被本次变更重写，建议顺带提取 tone→class 映射消除嵌套。

-           className={'wk-mkt-34 ' + (toast.tone === 'success' ? 'wk-mkt-35' : toast.tone === 'warning' ? 'wk-mkt-36' : 'wk-mkt-37')}
+ const MK_TOAST_TONE: Record<NonNullable<ToastState>['tone'], string> = {
+   success: 'wk-mkt-35',
+   warning: 'wk-mkt-36',
+   error: 'wk-mkt-37',
+ };
+ 
+ // 使用处：
+ className={'wk-mkt-34 ' + MK_TOAST_TONE[toast.tone]}


─── apps/web/src/knowledge/permissions.ts:96-101 ───
[security · high] 新增的 canManage 集中门控未被本组的 KnowledgeGraphPage
消费，破坏性管理门控实际仍被放宽。事实链：KnowledgeGraphPage.tsx:85 仍是 `setCanManage(canUploadKnowledgeDocuments(kb,
me))`，而该函数对 `my_permission: 'editor'` 返回
true（permissions.ts:127）；这个宽松值随后驱动两处破坏性入口——DocumentsBreadcrumb 的设置 gear（L616
`canManage={canManage}`）与 KnowledgeSettingsPage 的 `role={canManage ? 'admin' : 'viewer'}`（L861）。结果
shared editor 会以 admin 角色进入 KB 设置面板，正是本字段注释引用的 R1-17 红线，也是 permissions.test.ts:42 明示的"canContribute
误用作 canManage 是授权回归根因"。请将图谱页的加载 effect 改用集中口径。

-   const canManage = !!me && (
-     isSystemAdmin(me)
-     || isCreator(kb, me)
-     || sharePermission === 'owner'
-     || sharePermission === 'admin'
-   );
+ // KnowledgeGraphPage.tsx L85 应改用集中门控（本组同 PR 文件）：
+ // setCanManage(computeKBPermissions(kb as KBSurfaceKB, me as KBSurfaceMe | null).canManage);
+ // 同时从 './permissions.ts' 导入 computeKBPermissions，移除对 canUploadKnowledgeDocuments 的 canManage 误用。


─── apps/web/src/knowledge/knowledge-u.css:109-109 ───
[bug · medium] 三处 calc() 运算符两侧缺少空格，是非法 CSS 值（规范要求 +、- 两侧必须留白，否则整条声明被浏览器丢弃）：wk-kg-14 的 `max-width:
calc(100%-2rem)`、wk-kg-21 的 `top: calc(100%+4px)`、wk-kg-25 的 `top:
calc(100%+8px)`。后果：搜索容器在窄屏（<336px）下失去 320px→可用宽度的收缩约束会溢出画布；搜索结果下拉与帮助面板的垂直偏移声明失效（absolute 元素回退静态位置，丢失
4/8px 间距并可能与其他浮层错位）。

-   max-width: calc(100%-2rem);
+ /* wk-kg-14 */ max-width: calc(100% - 2rem);
+ /* wk-kg-21 */ top: calc(100% + 4px);
+ /* wk-kg-25 */ top: calc(100% + 8px);


─── apps/web/src/knowledge/knowledge-u.css:249-251 ───
[bug · medium] `.wk-kg-24 :-webkit-details-marker` 类名与伪元素之间的空格使其成为后代选择器（匹配 .wk-kg-24 内部元素的 marker），而
summary 的披露三角是 summary 自身的伪元素，因此该规则永不匹配，"?"帮助按钮在 Chrome/Safari 下会显示原生披露三角，与 Vue 视觉不一致。应为无空格的
`.wk-kg-24::-webkit-details-marker`（对应原 Tailwind `[&::-webkit-details-marker]:hidden`）。

- .wk-kg-24 :-webkit-details-marker {
+ .wk-kg-24::-webkit-details-marker {
    display: none;
  }


─── apps/web/src/knowledge/KnowledgeGraphPage.tsx:639-639 ───
[bug · medium] 此处残留的 `touch-none` 是本次 S7 平移后整个 apps/web 中唯一的 Tailwind utility（原串的
absolute/cursor-grab/select-none/active:cursor-grabbing 均已平移进 .wk-kg-3，唯独 touch-action: none 未平移）。一旦
Tailwind 旧栈按计划摘除，SVG 画布将失去 touch-action: none，触屏设备上拖动图谱会被浏览器原生 pan/zoom
手势抢占，beginPan/moveGraphGesture 的 pointer 交互失效。

-             <svg className="touch-none wk-kg-3" viewBox={`0 0 ${surfaceSize.width} ${surfaceSize.height}`} role="img" aria-label={t('knowledgeBase.graph.ariaLinks')} onPointerDown={beginPan} onPointerMove={moveGraphGesture} onPointerUp={endGraphGesture} onPointerCancel={cancelGraphGesture} onClick={(event) => {
+ /* knowledge-u.css .wk-kg-3 追加 */
+ .wk-kg-3 {
+   /* ...existing rules... */
+   touch-action: none;
+ }
+ /* TSX */ <svg className="wk-kg-3" ...>


─── apps/web/src/knowledge/KnowledgeGraphPage.tsx:629-629 ───
[bug · low] 外层 `mt-[6px]` wrapper 被删除后，ParserHint 与标题区之间的间距由 6px 塌缩为 documents.td.css:600
`.parser-hint { margin: 2px 0 0 }` 的 2px（原 6px wrapper 与 hint 自身 2px 折叠取 max=6px），产生 4px
视觉回归；且上方注释仍描述"wrapper margin (collapsing with the hint's own 2px) yields Vue's 6px
gap"这一已被删除的机制，与代码不符。建议补回 6px 间距（wrapper 语义类或 .parser-hint 上方 margin），并同步更新陈旧注释。

+         <div className="wk-kg-parser-hint-gap">
-         <ParserHint t={t} types={unsupportedFileTypes} onConfigure={openParserSettings} />
+           <ParserHint t={t} types={unsupportedFileTypes} onConfigure={openParserSettings} />
+         </div>
+ /* knowledge-u.css */ .wk-kg-parser-hint-gap { margin-top: 6px; }


─── apps/web/src/knowledge/knowledge-u.css:327-328 ───
[maintainability · low] 可维护性杂项：(1) wk-kg-31 连续声明两次 transition-duration，150ms 是 codemod 死残留（被 300ms
覆盖，最终值正确但应删除前者）；(2) wk-kg-39 与 wk-kg-48 中 `border-top-width: 1px;border-color: #e7e7e7;`
两条声明挤在同一行，格式与其余文件不一致；(3) 配套 TSX 中 `<line ... className=" wk-graph-arrow-stroke wk-kg-4"`
存在前导空格。均为无功能影响的一致性瑕疵。

-   transition-duration: 150ms;
    transition-duration: 300ms;


─── apps/web/src/knowledge/KnowledgeGraphPage.tsx:602-602 ───
[other · low] 页面根由 <main> 改为 div.knowledge-layout，而 PlatformShell 并未提供 <main> 地标兜底，本页将从拥有 main
landmark 退化为无地标页面，影响屏幕阅读器按区域跳转。若这是 KB 域跟随 KnowledgeDocumentsPage 的既定模式可接受，建议确认平台外壳层是否有统一补 main
的计划，否则可考虑在根节点补 role="main"。

-     <div className="knowledge-layout">
+ <div className="knowledge-layout" role="main">


─── apps/web/src/knowledge-settings/KnowledgeSettingsPage.css:635-636 ───
[bug · medium] 分段 Tab 首段左边框缺失（FAQ 型 KB 可见）：基础规则对所有段 border-left: 0，仅靠既有的 `.kb-type-tab +
.kb-type-tab` 恢复后续段左边框，首段没有任何恢复规则。当 KB 类型为 FAQ 时（tsx:1340-1346，首段为 is-disabled、末段 is-checked），首段左边框为
0 —— 容器 .kb-type-tabs 自身无边框、overflow:hidden 只裁圆角不补线，整组左边缘会缺 1px 描边线（顶/底描边悬空收口）。参照系 tdesign 原版有
`.t-radio-button:first-child { border-left: 1px solid …; border-radius: 3px 0 0 3px }`，注释自述对齐该行为但未移植
first-child 规则。doc 模式（首段 is-checked 全边框）不受影响，建议补首段规则（置于 .is-checked 规则之前即可，同 specificity 下
is-checked 的 border 简写在后仍会覆盖选中态）。

-   border: 1px solid var(--wk-color-border, #dcdcdc);
-   border-left: 0;
+ .kb-type-tab:first-child {
+   border-left: 1px solid var(--wk-color-border, #dcdcdc);
+ }


─── apps/web/src/knowledge-settings/KnowledgeSettingsPage.css:701-704 ───
[bug · medium] 隐藏 checkbox 后丢失键盘焦点指示：原生 input 以 1px/clip 方式视觉隐藏（可见盒由 .kb-checkbox-input span
承担），但本文件没有任何 `input:focus-visible + .kb-checkbox-input` 规则（全文件 focus 规则仅
.kb-text-input-wrap/:focus-within、.kb-text-input:focus、.kb-textarea:focus 三处）。input 仍在 tab 序列中，键盘用户
Tab 到它时看不到任何焦点位置 —— 相比旧实现（原生 checkbox 可见且自带 focus ring）是 WCAG 2.4.7 焦点可见性回归；tdesign 原版对隐藏的
.t-checkbox__former 正是在可见盒上加 focus 描边，移植时漏掉。两处 checkbox（tsx:1360/1381，向量+关键词、Wiki 索引）均受影响。

- .indexing-check-head input[type='checkbox'] {
-   position: absolute;
-   width: 1px;
-   height: 1px;
+ .indexing-check-head input:focus-visible + .kb-checkbox-input {
+   box-shadow: 0 0 0 2px var(--wk-color-brand-light, #e9f8ec);
+   border-color: var(--wk-color-brand, #07c05f);
+ }


─── apps/web/src/platform/PlatformShell.tsx:68-69 ───
[bug · medium] 键盘分支为死代码：React 合成键盘事件没有 `button` 属性（属 MouseEvent 接口），`(event as
ReactMouseEvent).button` 取值为 `undefined`，`undefined !== 0` 恒为 true，导致 Enter keydown 永远走早退——既不
preventDefault 也不 navigate。账号卡片（role="button" + tabIndex=0，第 1266 行 onKeyDown Enter
调用点）的键盘激活因此完全失效，与注释宣称的『账号卡片可复用该守卫做 Enter keydown』目的自相矛盾（注释自己也承认 keyboard events keep the early
return，则该 onKeyDown 就是纯死代码）。建议按事件类型区分：键盘事件跳过 button 检查（恢复键盘可达性），或删除无效的 onKeyDown。

  function handleInternalLink(event: ReactMouseEvent<Element> | ReactKeyboardEvent<Element>, path: string, afterNavigate?: () => void): void {
-   if (event.defaultPrevented || (event as ReactMouseEvent<Element>).button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
+   const viaKeyboard = event.type === 'keydown';
+   if (event.defaultPrevented || (!viaKeyboard && (event as ReactMouseEvent<Element>).button !== 0) || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;


─── apps/web/src/platform/PlatformShell.tsx:216-216 ───
[maintainability · medium] career / careerSearch / careerRules 三个导航项 label
硬编码中文（'求职档案'/'找岗'/'持续找岗'），绕过同数组其余条目统一使用的 t()/labels i18n 通道（newChat/knowledgeBases/agents 等均走词条）。已确认
i18n 资源中无 career 导航词条——非英文 locale 用户的全局侧栏将出现未翻译中文，且后续无法与 career 页面共享词条。建议补充 t('menu.career') 等词条或收进
labels record，与相邻条目保持一致。



─── apps/web/src/platform/PlatformShell.tsx:766-767 ───
[bug · low] 渲染体内直接写 ref（`sessionActivityRef.current = sessionActivityEntries`）违反 React
渲染纯性约定：并发模式下渲染可能被丢弃或重放，会短暂写入过期/未提交的值。建议移入 useEffect 同步（间隔回调读取的是提交后的最新值，语义不变）。

    const sessionActivityRef = useRef<SessionActivityEntries>({});
-   sessionActivityRef.current = sessionActivityEntries;
+   useEffect(() => { sessionActivityRef.current = sessionActivityEntries; }, [sessionActivityEntries]);


─── apps/web/src/platform/PlatformShell.tsx:553-553 ───
[maintainability · low] tenantSwitcherVisible 不再看 onTenantSwitch 是否传入，而 switchTenant 对
`!onTenantSwitch` 静默 return。已核实当前唯一生产挂载点 router.tsx:213 恒传入该 prop，故今天无实际故障，但可选 prop 的防御语义被移除后：宿主省略
onTenantSwitch 时切换 glyph 与租户子菜单仍渲染，点击后零反馈。同时 shouldShowTenantSwitcher 运行时已无调用方（仅
platform-shell-nav.test.ts 引用），注释「保留给未来 TenantSelector 挂载」与现行行为存在偏差。建议保留 hasSwitchHandler 门控，或在
switchTenant 的无处理函数分支给出禁用态提示。



─── apps/web/src/platform/PlatformShell.tsx:1069-1071 ───
[bug · low] 拖拽监听挂在 document 上，仅在「右拖 >40px 越过阈值」或 mouseup 时通过 cleanup() 移除；若组件在拖拽会话进行中卸载（路由切换/租户切换重挂
shell），mousemove/mouseup 监听器泄漏，且越阈回调还会对已卸载组件 setState。建议将监听器生命周期纳入 useEffect 卸载清理，或在组件级 cleanup
中兜底移除当前会话的 handler。



─── apps/web/src/platform/PlatformShell.tsx:1185-1185 ───
[style · low] 嵌套三元（检查清单明确禁止）且 iconPair 缺失时回退 `src=''`——空字符串 src 会被浏览器解析为当前页面
URL，触发一次多余的页面请求。当前全部条目键均配对成功（NAV_ICON_URLS 覆盖
creatChat/knowledge-bases/agents/organizations，React-only 条目走 iconNode），但未来新增 icon 键名与 NAV_ICON_URLS
拼写不一致时会静默退化为空图。建议先解出 navIconSrc 局部变量（undefined 时不渲染 img）。

- {item.iconNode ?? <img className="icon" src={iconPair ? (active ? iconPair.active : iconPair.default) : ''} alt="" />}
+               const navIconSrc = iconPair ? (active ? iconPair.active : iconPair.default) : undefined;
+               // ...
+               {item.iconNode ?? (navIconSrc ? <img className="icon" src={navIconSrc} alt="" /> : null)}


─── apps/web/src/platform/PlatformShell.tsx:770-775 ───
[bug · low] 与 Vue 语义存在偏差：Vue 端 store 同时 watch [auth.user.id,
auth.effectiveTenantId]（flush:'sync'）清空条目，此处只看 client 对象身份。若租户切换复用同一 client 实例（switchTenantFromShell
的实现决定），旧租户的 running 条目会跨租户保留并继续 5s 轮询，直到服务端 403 才被 refreshSessionActivityError 清除，期间侧栏会显示他租户会话的
running spinner。最直接的补法：switchTenant 成功后同步 setSessionActivityEntries({})。



─── apps/web/src/platform/PlatformShell.tsx:893-899 ───
[maintainability · low] clearShellSessionMessages / deleteShellSession / deleteShellSessions
三处内几乎相同的「按 id 删除 sessionActivity 条目」更新块重复（前两处逐字符相同，第三处为批量版）。建议抽一个 removeSessionActivity(ids) helper
统一维护，避免后续修改删除语义时漏改。

-     // Vue menu.vue handleSessionMutation — messagesCleared → sessionActivity.update(id, false).
+   function removeSessionActivity(ids: Iterable<string>): void {
      setSessionActivityEntries((prev) => {
-       if (!(sessionId in prev)) return prev;
        const next = { ...prev };
-       delete next[sessionId];
-       return next;
+       let changed = false;
+       for (const id of ids) {
+         if (id in next) { delete next[id]; changed = true; }
+       }
+       return changed ? next : prev;
      });
+   }


─── apps/web/src/platform/platform-u.css:51-51 ───
[bug · medium] `.wk-cmdk-6`（命令面板容器）：`max-width: calc(100vw-32px)` 缺少 `-` 运算符两侧空格，属非法 CSS（calc 的 + /
- 必须两侧留白），整条 max-width 声明被解析器静默丢弃——迁移自 `max-w-[calc(100vw-32px)]` 时未修正。视口 <672px 时 640px
宽的面板将溢出屏幕。修为 `calc(100vw - 32px)`（同类误译全仓还有 data-sources/kb 等域，建议一并 grep 排查）。



─── apps/web/src/platform/platform-u.css:345-348 ───
[bug · medium] `.wk-cmdk-32`（检索设置抽屉）：与 .wk-cmdk-6 相同的 calc 误译，`calc(100vw-32px)` 非法、声明被丢弃，420px
抽屉在小屏溢出。修为 `calc(100vw - 32px)`。



─── apps/web/src/platform/platform-u.css:520-523 ───
[bug · medium] `.wk-inv-3`（邀请铃铛计数角标）：`line-height: 5` 是 Tailwind `leading-5` 的误译——unitless
值是字号乘数而非像素，原语义为 1.25rem(20px)。11px 字号下行盒被撑到约 55px 高，角标视觉严重破损。同类误译（wk-cmdk-12 的 border-2）已被人工修正为
2px，此处漏改；同款 `line-height: 5` 还出现在 apps-u.css:189 / data-sources-u.css:45 /
kb-u.css:90（不在本组文件，建议一并修）。修为 `line-height: 1.25rem`。

    font-size: 11px;
-   line-height: 5;
+   line-height: 1.25rem;
    color: #fff;
  }


─── apps/web/src/platform/platform-u.css:629-632 ───
[maintainability · low] outlet 节点同时挂 `plat-shell__outlet wk-shell-5` 两个类，`.wk-page` 的
height/overflow-y 规则在 `.wk-shell-5 .wk-page` 与文件尾部的 `.plat-shell__outlet .wk-page`（后者还多
max-width:none!important）两套选择器下重复定义，两处需同步维护且优先级不同，易在后续改动中出现分歧。建议合并为一处（保留带 !important 的完整版本即可）。



─── apps/web/src/platform/platform-shell.td.css:56-56 ───
[maintainability · low] `--sidebar-text-inset` 定义后全仓无任何引用（已确认唯一出现即此定义处），属死代码；另外 `.menu_box` 在 §1 以
`.aside_box .menu_box { display:flex; flex-direction:column }` 定义、又在顶层以 `.menu_box {
position:relative }` 二次定义，可合并减少维护面。建议删除未用变量并合并重复选择器。



─── apps/web/src/platform/platform-u.css:47-51 ───
[bug · medium] `.wk-cmdk-6`（命令面板容器）：`max-width: calc(100vw-32px)` 缺少 `-` 运算符两侧空格，属非法 CSS（calc 的
`+`/`-` 必须两侧留白，否则被解析为 `100vw` 与负值 `-32px` 的歧义而整体报错），整条 max-width 声明被解析器静默丢弃——迁移自
`max-w-[calc(100vw-32px)]` 时未修正。视口 <672px 时 640px 宽的面板将溢出屏幕。修为 `calc(100vw - 32px)`（本文件 348 行
.wk-cmdk-32 同款误译另报）。

- .wk-cmdk-6 {
-   display: flex;
-   max-height: 70vh;
-   width: 640px;
-   max-width: calc(100vw-32px);
+   max-width: calc(100vw - 32px);


─── apps/web/src/organizations/OrganizationsPage.tsx:1026-1027 ───
[bug · medium] 键盘与无障碍回归（相对旧版为功能性退化）：org-card 丢失了旧版的 role="button"、tabIndex 与 onKeyDown（Enter
打开设置）；同批还有：① more-wrap 触发器由可聚焦 button 退化为纯 div onClick，且内为 alt="" 图片，无可访问名称；② 设置弹窗 nav-item 由
<button> 改为 div onClick；③ 侧栏 icon-item-labeled/sidebar-item（旧版为带 title 的 button）、del-org-dialog 的
circle-btn-txt（span onClick）均不可聚焦、无 role。键盘用户与读屏用户无法打开卡片设置、切换
section、确认删除/退出。建议恢复交互语义（role/tabIndex/onKeyDown 或直接用原生 button/TDesign 组件）。

        <div key={org.id || index} className={'org-card' + (owner ? '' : ' joined-org')} style={rowHidden ? { display: 'none' } : undefined}
-         onClick={() => openSettingsModal(org)}>
+         role="button" tabIndex={0}
+         onClick={() => openSettingsModal(org)}
+         onKeyDown={(event) => { if (event.key === 'Enter') openSettingsModal(org); }}>


─── apps/web/src/organizations/OrganizationsPage.tsx:1400-1400 ───
[bug · low] create 模式 label htmlFor="organization-name" 与该 TDesign Input 失去关联：Input 仅传了 name 而未传
id（旧版两版均带 id="organization-name"）。点击标签无法聚焦输入框，读屏也无法关联字段名称。同批还有加入组织弹框的 label htmlFor="join-search" 对应
TInput 同样丢失 id。建议补回 id（或去掉 htmlFor）。

-                                 <Input name="organization-name" className="name-input" value={formName} onChange={(value: string) => setFormName(value)} placeholder={t(locale, 'organization.namePlaceholder')} />
+                                 <Input id="organization-name" name="organization-name" className="name-input" value={formName} onChange={(value: string) => setFormName(value)} placeholder={t(locale, 'organization.namePlaceholder')} />


─── apps/web/src/organizations/orgs.td.css:871-882 ───
[bug · high] §4 区块违反本文件头部声明的「scoped 块 → 根类前缀
.org-list-container」约定：.more-wrap/.card-header/.card-title/.card-content/.card-bottom/.feature-badge
/.relation-role-tag/.empty-state 等十余个通用类名以无前缀形式落入全局命名空间（姊妹迁移页 agents.td.css、kb-list.td.css 对同名类均带
.agent-list-container/.kb-list-container 前缀）。实际后果：SPA 内 chunk CSS 加载后常驻，documents.td.css 的
.knowledge-card .more-wrap（1349 行）只声明了尺寸/圆角/光标、从未声明 opacity，orgs 的全局 .more-wrap { opacity: 0 }
会外溢到文档页——访问过 org 页后，文档卡片的三点按钮默认不可见、仅 hover 时被 opacity:1 !important 救回，属跨页功能性回归。建议按姊妹页先例整体补
.org-list-container/.org-card 前缀。

- .more-wrap {
+ .org-list-container .more-wrap,
+ .org-card .more-wrap {
    display: flex;
    width: 28px;
    height: 28px;
    justify-content: center;
    align-items: center;
    border-radius: 8px;
    cursor: pointer;
    flex-shrink: 0;
    transition: all 0.2s ease;
    opacity: 0;
  }


─── apps/web/src/organizations/org-u.css:928-935 ───
[bug · low] calc(90vh-120px) 缺少减号两侧空格，属无效 CSS，整条 max-height 被丢弃，加入组织弹窗内容区高度上限失效（长列表可溢出视口）。该缺陷自旧
Tailwind 任意值 max-h-[calc(90vh-120px)] 原样平移而来（旧代码同样从未生效），建议趁迁移修正为带空格写法。

  .wk-org-67 {
-   max-height: calc(90vh-120px);
+   max-height: calc(90vh - 120px);
    min-height: 0;
    overflow-x: hidden;
    overflow-y: auto;
    padding-inline: 24px;
    padding-top: 20px;
  }


─── apps/web/src/settings/ChatHistorySettingsPanel.tsx:51-52 ───
[bug · medium] debounce 保存存在丢保存窗口：save() 在 savingRef.current 为 true 时直接 return
且不重新调度，定时器已被消费——若用户在上一轮 PATCH 在途期间再次切换开关或改选 Embedding 模型，该次变更被静默丢弃（UI 显示新值、服务端为旧值，刷新后回退）。对比
ConfigSettingsPanel 的 effect 驱动 debounce（保存完成 merge 服务器值后 dirty 仍在则自动重调度，具备隐式重试），本面板的 scheduleSave
手动调度模式丢失了该语义。另外 onSaved → 父层重拉 → initialValue 变化会触发 useEffect 整体覆写
enabled/embeddingModelId/latestRef，把用户尚未保存的编辑一并回滚。建议：save 被拒时记录 pendingRef，在 finally 中重放；或改用 effect
驱动（依赖 latestRef.current 与 saved 的差异）。

+   const pendingRef = useRef<ChatHistoryConfig | null>(null);
    async function save(next: ChatHistoryConfig) {
-     if (savingRef.current) return;
+     if (savingRef.current) { pendingRef.current = next; return; }
+     // ...
+     } finally {
+       savingRef.current = false;
+       const pending = pendingRef.current;
+       pendingRef.current = null;
+       if (pending) void save(pending);
+     }


─── apps/web/src/settings/ConfigSettingsPanel.tsx:147-149 ───
[bug · medium] clearable + 裸 String(value) 会产生 "undefined" 脏值：TDesign Select 单选点击清除时 onChange 回调的
value 为 undefined（仓库内 ModelSelector.tsx:95 已有 `typeof value === 'string' ? value : ''`
防护作证），String(undefined) 会把字面量 "undefined" 写入 rerank_model_id。retrieval 分区 500ms
自动保存后，settingsConfigPatch → modelIdSelection 因 "undefined" 不在 allowedModelIds 抛错，用户在仅点了清除的情况下收到
'rerank_model_id must be one of the tenant models' 保存失败提示，且无法通过清除按钮清空选择（只能选 '—' 选项）。建议与
ModelSelector 的清空兜底保持一致。

-     options={[{ value: '', label: '—' }, ...modelOptions.map((model) => ({ value: model.id, label: model.name ? model.name + ' (' + model.id + ')' : model.id }))]}
-     onChange={(value) => setValue(key, String(value))}
-   />;
+     onChange={(value) => setValue(key, typeof value === 'string' ? value : '')}


─── apps/web/src/settings/CloudSettingsPanel.tsx:133-137 ───
[bug · low] 丢失旧实现的 trim 前置校验：disabled 与提交校验均改为裸 truthy 判断（!form.appId），纯空白串可通过前置检查并触发
cloudCredentialPatch 抛出英文 'WeKnora Cloud app ID is required'，以 error toast（saveFailed
语义）呈现，而非旧实现的本地化 warning（fillRequired）。功能上被 surface 层拦截，不会落库无效凭证，但错误文案与语气回归。建议恢复 trim 判断。

    async function handleSave() {
-     if (!form.appId || !form.appSecret) {
+     if (!form.appId.trim() || !form.appSecret.trim()) {
        pushSettingsToast(t('settings.weknoraCloud.fillRequired'), 'warning');
        return;
      }


─── apps/web/src/settings/CloudSettingsPanel.tsx:316-316 ───
[security · low] 使用说明从安全的 React 子节点渲染（旧实现 split('\n').map(<span>)）回归为 dangerouslySetInnerHTML
注入。虽然当前 usageSteps 是五个语言包的静态受信文案、无用户输入注入路径，但任一译文未来包含 < 或 & 即破坏渲染/产生标记注入，且违背 innerHTML 安全基线（checklist
明确禁用 innerHTML 直插）。换行符换 <br /> 完全可用 CSS（white-space: pre-line）或保留 span 映射实现，建议回退安全渲染。

-         <p className="hint-text" dangerouslySetInnerHTML={{ __html: t('settings.weknoraCloud.usageSteps').replace(/\n/g, '<br />') }} />
+         <p className="hint-text">{t('settings.weknoraCloud.usageSteps').split('\n').map((line, index) => <span key={index} className="block">{line}</span>)}</p>


─── apps/web/src/settings/GeneralPreferencesPanel.tsx:273-273 ───
[bug · low] 迁移到 TDesign Select/Switch 后丢失原有 aria-label：语言/主题/两个字体 Select 只剩 placeholder（placeholder
不能替代可访问名称），自动更新 Switch 的 aria-label 也被移除（见下方 Switch 行）。屏幕阅读器用户将失去控件语义。TDesign Select 支持 ariaLabel
属性，Switch 可用 aria-label 透传，建议补回。

-             <Select value={locale} placeholder={t('language.selectLanguage')} onChange={(value) => handleLanguageChange(String(value))} style={{ width: '280px' }}>
+             <Select value={locale} ariaLabel={t('language.selectLanguage')} placeholder={t('language.selectLanguage')} onChange={(value) => handleLanguageChange(String(value))} style={{ width: '280px' }}>


─── apps/web/src/settings/McpSettingsPanel.tsx:1301-1303 ───
[bug · low] 组件从 @weknora/ui Textarea 换成 TTextarea 后，上方的 maxLength={16000}（驼峰）不再被组件识别：TDesign
Textarea 的 prop 为全小写 maxlength（仓库约定见同文件 name 输入的 maxlength={128}，以及
ModelSettingsPanel/PersonalMemoryPanel/ResourceSettingsPanel 等处；ModelSettingsPanel 测试注释明确 'tdesign
Input 的 maxlength 走 JS 截断'）。驼峰写法仅可能通过 DOM 属性透传侥幸生效，TDesign 的受控截断/计数逻辑不会执行（generateUsage 以 JS 赋值回填时可超
16000 上限而不被截断）。建议改为小写并统一。

-                       onChange={(value) =>
-                         setField("usageInstructions", String(value))
-                       }
+                     <TTextarea
+                       rows={5}
+                       maxlength={16000}
+                       value={draft.usageInstructions}
+                       onChange={(value) => setField("usageInstructions", String(value))}
+                     />


─── apps/web/src/settings/McpSettingsPanel.tsx:1240-1240 ───
[bug · low] 移除 type="number" 后数字字段可输入任意文本：onChange 即时执行 Number(String(value))，输入非数字（如 'abc'）得到 NaN
存入 draft，渲染端 String(NaN) 会让输入框直接显示字面量 'NaN'，直到 onBlur 才被 normalizeMcpAdvancedNumber 归一化到
fallback（NaN 不会入库，payload 侧有 Number.isFinite 防护，但 UI 呈现错误且与旧 type="number" 行为不一致）。建议 onChange
时对非有限数字即时归一化（或直接用 TDesign InputNumber + min/max）。

-                           onChange={(value) => setField("timeout", String(value) === "" ? "" : Number(String(value)))}
+                           onChange={(value) => {
+                             const parsed = Number(String(value));
+                             setField("timeout", String(value) === "" ? "" : Number.isFinite(parsed) ? parsed : "");
+                           }}


─── apps/web/src/settings/EnvVarSettingsPanel.tsx:164-164 ───
[bug · low] 移除原生 required 与禁用占位项（<option value="" disabled>）后，空 scopeId/name 可触发表单提交：功能上被 envVarSet
的 trim 校验拦截不会到达服务端，但用户会看到英文硬编码错误（errorText 取 reason.message，如 'Variable name is
required'），而旧实现由浏览器原生 required 拦截并按用户语言提示。建议提交前用本地化文案做前置校验（同文件 setVariable 已有 valueRequired 先例）。

            : <Input value={scopeId} onChange={(next) => setScopeId(String(next ?? ''))} />}
+       {/* 提交前校验：setVariable 中先以本地化文案检查 scopeId/name 非空，避免透出 envVarSet 的英文错误 */}


─── apps/web/src/shared/shared-u.css:174-177 ───
[bug · high] `.wk-shared-1` 在文件内定义了两段语义冲突的规则：首段（标题样式：font-weight:600 / font-size:20px /
margin-bottom:8px）与末尾本段（dl 的 gap 平移）同名碰撞。TSX 中无效态 `<h1 className="wk-shared-1">` 与元信息 `<dl
className="wk-shared-9 wk-shared-1">`（SharedSessionPage.tsx:88）共用该类：对 dl 而言，font-size/color/margin
可被源序更靠后的 .wk-shared-9 覆盖，但 font-weight:600 没有任何对冲声明，元信息行（会话
ID/时间/来源/引擎）将被错误渲染为粗体——这是公开分享页上的确定性视觉回归（OCR round-1 已指出此问题但未修复）。同时暴露出生成器类名碰撞缺陷：本批新增的数十个 *-u.css
若由同一生成器产出，可能存在同类隐患。建议为 gap 定义独立类名并同步修改 TSX 中 dl 的 className。

- .wk-shared-1 {
+ /* 为 dl 元信息 gap 使用独立类名，避免与标题样式 .wk-shared-1 碰撞 */
+ .wk-shared-meta-gap {
    column-gap: 16px;
    row-gap: 4px;
  }
+ 
+ /* 同步：SharedSessionPage.tsx 中 <dl className="wk-shared-9 wk-shared-1"> 改为 <dl className="wk-shared-9 wk-shared-meta-gap"> */


─── apps/web/src/shared/shared-u.css:83-89 ───
[bug · low] Tailwind `text-xs` 同时产出 font-size: 0.75rem 与 line-height: 1rem（16px），此处只平移了 font-size:
12px，line-height 丢失后回落到继承值，元信息换行行距与迁移前不一致，违背文件头「值 = 迁移时 utilities 编码的生效值」的承诺（对照 .wk-shared-8/15/18
均显式携带了 line-height）。文件末尾的 .wk-shared-17（truncated 提示，原文同为 text-xs）存在同样缺失，请一并补上 `line-height: 16px;`。

  .wk-shared-9 {
    margin: 0;
    display: flex;
    flex-wrap: wrap;
    font-size: 12px;
+   line-height: 16px;
    color: #8a96a8;
  }


─── apps/web/src/shared/shared-u.css:26-37 ───
[maintainability · low] 生成器输出未做声明去重：`.wk-shared-4` 同一规则块内 `border-style: solid`
声明了两次（.wk-shared-8/13/15 同样如此）。虽是幂等声明不影响渲染，但会干扰后续维护者对生效值的核对，也说明平移工具缺少去重与校准环节。另外原 utilities 的
`break-all`（word-break: break-all）被译为 `overflow-wrap: anywhere`，二者对超长无空格串的断行行为并不等价（break-all
可在任意字符断行，anywhere 优先在正常断点），建议平移时按 Tailwind 实际产出属性逐项对照。

  .wk-shared-4 {
    min-height: 32px;
    cursor: pointer;
    border-radius: 6px;
    border-style: solid;
    border-width: 1px;
-   border-style: solid;
    border-color: #dcdcdc;
    background-color: #fff;
    padding-inline: 12px;
    font-size: 13px;
  }


─── apps/web/src/shared/wk-legacy.tsx:82-83 ───
[bug · medium] openDialogStack 的清理路径存在时序缺陷：当 open 置 false（或组件卸载）时，React 在 commit 阶段先卸载 section 并把
dialogRef.current 置空，之后才执行本 passive effect 的 cleanup——此时 `dialogRef.current` 已为 null，index 恒为
-1，splice 永远不会执行。每次弹层开合都会向模块级数组残留一个已分离的 DOM
节点（携带整个弹层子树），在长驻页面（workbench/文档页反复开关弹层）中无限增长，构成内存泄漏。修复方式：在 effect 建立时把节点捕获进局部变量，cleanup 中使用该捕获值。

-       const index = dialogRef.current ? openDialogStack.indexOf(dialogRef.current) : -1;
+ useEffect(() => {
+     if (!open) return;
+     const dialogEl = dialogRef.current;
+     if (dialogEl) openDialogStack.push(dialogEl);
+     // ……（其余逻辑不变）
+     return () => {
+       document.removeEventListener('keydown', onKeyDown);
+       const index = dialogEl ? openDialogStack.indexOf(dialogEl) : -1;
        if (index >= 0) openDialogStack.splice(index, 1);
+       restoreRef.current?.focus();
+       restoreRef.current = null;
+     };
+   }, [open]);


─── apps/web/src/shared/wk-legacy.tsx:192-194 ───
[bug · low] `aria-label={String(title)}` 是有害的死属性：title 类型为 ReactNode，真实消费方
KnowledgeDocumentDetailPage:318 以 JSX 元素作 title 传入，String() 结果为 "[object Object]"（title 缺省时则是
"undefined"）；且本元素同时设置了 aria-labelledby={titleId}，按 ARIA 规范 aria-labelledby 优先级高于 aria-label，该
aria-label 在任何情况下都不会生效。建议直接删除，无障碍名称统一由 aria-labelledby（指向含完整 title 的 h2）提供。

            aria-labelledby={titleId}
            tabIndex={-1}
-           aria-label={String(title)}


─── apps/web/src/shared/wk-legacy.tsx:147-153 ───
[performance · low] 拖宽 effect 将 panelWidth 列入依赖数组：拖动期间每次 mousemove 触发 setPanelWidth → 重渲染 → 本 effect
整体重跑（先 removeEventListener 再 addEventListener 两个 window 监听并重建全部闭包），在 60Hz+ 的 mousemove
频率下产生高频监听器抖动。doc-detail 抽屉（KnowledgeDocumentDetailPage:316，resizable）正是该路径的长期消费方。panelWidth 入 deps
仅是为了让 stop 闭包拿到最新宽度用于 localStorage 持久化，建议改用 width ref（与 setPanelWidth 同步更新），将 panelWidth
移出依赖数组，使监听器只在挂载配置变化时重建。

-     window.addEventListener('mousemove', move);
-     window.addEventListener('mouseup', stop);
-     return () => {
-       window.removeEventListener('mousemove', move);
-       window.removeEventListener('mouseup', stop);
-     };
-   }, [maxWidth, minWidth, panelWidth, resizable, side, storageKey]);
+   // 组件体内：const panelWidthRef = useRef(initialWidth);
+   // move/stop 中以 panelWidthRef.current 读写宽度（setPanelWidth 的同时同步 ref），
+   // stop 持久化改为 window.localStorage.setItem(storageKey, String(panelWidthRef.current));
+   }, [maxWidth, minWidth, resizable, side, storageKey]);


─── apps/web/src/shared/SharedSessionPage.tsx:105-105 ───
[style · low] 角色文案使用嵌套三元表达式（user/assistant/system 三分支内联判断），违反代码质量规则中「禁止嵌套三元表达式」。与同文件顶部的 ROLE_TONE
映射表对照，建议提取平行的 ROLE_LABEL 映射，渲染处一行取值即可，可读性与可维护性更好。

-                     {message.role === 'user' ? t('settings.queryHistory.roleUser') : message.role === 'assistant' ? t('settings.queryHistory.roleAssistant') : t('settings.queryHistory.roleSystem')}
+ // 组件外：与 ROLE_TONE 平行
+ const ROLE_LABEL: Record<'user' | 'assistant' | 'system', string> = {
+   user: 'settings.queryHistory.roleUser',
+   assistant: 'settings.queryHistory.roleAssistant',
+   system: 'settings.queryHistory.roleSystem',
+ };
+ // 渲染处：{t(ROLE_LABEL[message.role])}


LLM retry report summary: 146 of 1122 requests affected -- 30 requests failed, 3 requests cancelled, 113 requests recovered after retry

Review planning (18 requests):
- apps/web/src/analytics/AnalyticsPage.tsx,apps/web/src/analytics/analytics-u.css: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/settings/TenantDeleteZone.tsx,apps/web/src/settings/TenantMembersPanel.tsx,apps/web/src/settings/TenantUserProfileSections.tsx,apps/web/src/settings/UsagePanel.tsx,apps/web/src/settings/settings-toast.tsx,apps/web/src/settings/settings-wrapper.css,apps/web/src/settings/settings.td.css,apps/web/src/settings/surface.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- internal/modules/career/material.go,internal/modules/career/office.go,internal/modules/career/opportunity.go,internal/modules/career/preparation.go,internal/modules/career/search_once.go,internal/modules/career/search_rule.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- packages/views/src/chat/agent-selector.tsx,packages/views/src/chat/chat-copy.ts,packages/views/src/chat/composer.tsx,packages/views/src/chat/mermaid.ts,packages/views/src/chat/message-face.tsx,packages/views/src/chat/message-list.tsx,packages/views/src/chat/page.tsx,packages/views/src/chat/session-sidebar.tsx,packages/views/src/chat/tool-approval.tsx,packages/views/src/chat/tool-result.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- packages/views/src/craft/sources.tsx,packages/views/src/craft/spreadsheet.tsx,packages/views/src/craft/td.tsx,packages/views/src/craft/templates.tsx,packages/views/src/craft/versions.tsx,packages/views/src/craft/workbench.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- ... and 13 more

Core review (119 requests):
- apps/miniprogram/tests/live/quota-inject-proxy.mjs,apps/miniprogram/tests/live/t24r1-live-driver.cjs: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/index.html,apps/web/package.json,apps/web/src/App.tsx,apps/web/src/DevMarkdownPage.tsx,apps/web/src/NotFoundPage.tsx,apps/web/src/main.tsx,apps/web/src/router.tsx,apps/web/src/routes.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/agents/AgentEditorModal.tsx,apps/web/src/agents/AgentParserRules.tsx,apps/web/src/agents/AgentsPage.tsx,apps/web/src/agents/MbtiTestModal.tsx,apps/web/src/agents/PersonaSection.tsx,apps/web/src/agents/SubagentsSection.tsx,apps/web/src/agents/agents-u.css,apps/web/src/agents/agents.css,apps/web/src/agents/agents.td.css: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/career/ApplicationPage.tsx,apps/web/src/career/CareerPage.tsx,apps/web/src/career/ExportDeletionPage.tsx,apps/web/src/career/InboxPage.tsx,apps/web/src/career/MaterialPage.tsx,apps/web/src/career/OpportunityPage.tsx,apps/web/src/career/application.css,apps/web/src/career/export-deletion.css,apps/web/src/career/inbox.css,apps/web/src/career/material.css: timed out -> failed
- apps/web/src/faq/FAQPage.tsx,apps/web/src/faq/faq.td.css: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- ... and 114 more

Context compaction (7 requests):
- apps/web/src/platform/GlobalCommandPalette.tsx,apps/web/src/platform/InvitationInbox.tsx,apps/web/src/platform/PlatformShell.tsx,apps/web/src/platform/platform-shell.td.css,apps/web/src/platform/platform-u.css,apps/web/src/platform/retrieval-settings-panel.tsx,apps/web/src/platform/session-activity.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rejected by provider (HTTP 400) -> failed
- apps/web/src/settings/QueryHistoryPanel.tsx,apps/web/src/settings/ResourceSettingsPanel.tsx,apps/web/src/settings/RuntimeQueuesPanel.tsx,apps/web/src/settings/SandboxSettingsPanel.tsx,apps/web/src/settings/SettingsPage.tsx,apps/web/src/settings/SkillSettingsPanel.tsx,apps/web/src/settings/SystemAuditLogPanel.tsx,apps/web/src/settings/SystemGlobalSettingsPanel.tsx,apps/web/src/settings/SystemInfoPanel.tsx,apps/web/src/settings/TenantAuditDrawer.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/agents/AgentEditorModal.tsx,apps/web/src/agents/AgentParserRules.tsx,apps/web/src/agents/AgentsPage.tsx,apps/web/src/agents/MbtiTestModal.tsx,apps/web/src/agents/PersonaSection.tsx,apps/web/src/agents/SubagentsSection.tsx,apps/web/src/agents/agents-u.css,apps/web/src/agents/agents.css,apps/web/src/agents/agents.td.css: cancelled
- apps/web/src/documents/DocumentsPage.tsx,apps/web/src/documents/DocumentsPageChrome.tsx,apps/web/src/documents/KnowledgeDocumentDetailPage.tsx,apps/web/src/documents/KnowledgeDocumentsPage.tsx,apps/web/src/documents/TagPickerDialog.tsx,apps/web/src/documents/UploadConfirmDialog.tsx,apps/web/src/documents/documents-list.css,apps/web/src/documents/documents-u.css,apps/web/src/documents/documents.td.css,apps/web/src/documents/preview.ts: cancelled
- apps/web/src/faq/FAQPage.tsx,apps/web/src/faq/faq.td.css: cancelled
- ... and 2 more

Comment filtering (2 requests):
- apps/web/src/organizations/OrganizationsPage.tsx,apps/web/src/organizations/SpaceAvatar.tsx,apps/web/src/organizations/org-u.css,apps/web/src/organizations/orgs.td.css: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/miniprogram/src/adapters/career-platform.ts,apps/miniprogram/src/app.config.ts,apps/miniprogram/src/app.scss,apps/miniprogram/src/career/rules-usage-reminders.config.ts,apps/miniprogram/src/career/rules-usage-reminders.tsx,apps/miniprogram/src/core/errors.ts,apps/miniprogram/src/core/routes.ts,apps/miniprogram/src/features/home/pages.tsx,apps/miniprogram/src/services/career.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
