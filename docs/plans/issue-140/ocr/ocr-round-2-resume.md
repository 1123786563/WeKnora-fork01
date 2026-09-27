Review partially complete: 409 finding(s); 13 of 319 selected item(s) failed.

─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:37-56 ───
[maintainability · low] tapText 与 tapListRow 是同一套「deadline 轮询 + 文本匹配 + tap + 返回
boolean」逻辑，仅元素选择器不同（'button, .wk-button' vs 'view'）。调整超时/轮询策略需双处同步；且与 t24r1-live-driver.cjs 中的同类
helper 也存在跨文件复制。建议在本文件内合并为一个带 selector 参数的 tapMatching helper。

- async function tapText(page, wanted, timeoutMs = 20000) {
-   const deadline = Date.now() + timeoutMs;
-   while (Date.now() < deadline) {
-     for (const el of await page.$$('button, .wk-button')) {
-       try { if (((await el.text()) || '').includes(wanted)) { await el.tap(); return true; } } catch {}
-     }
-     await sleep(900);
-   }
-   return false;
- }
- async function tapListRow(page, wanted, timeoutMs = 20000) {
+ async function tapMatching(page, selector, wanted, timeoutMs = 20000) {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
-     for (const el of await page.$$('view')) {
+     for (const el of await page.$$(selector)) {
        try { if (((await el.text()) || '').includes(wanted)) { await el.tap(); return true; } } catch {}
      }
      await sleep(900);
    }
    return false;
  }
+ const tapText = (page, wanted, timeoutMs) => tapMatching(page, 'button, .wk-button', wanted, timeoutMs);
+ const tapListRow = (page, wanted, timeoutMs) => tapMatching(page, 'view', wanted, timeoutMs);


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:47-56 ───
[bug · low] tapListRow 遍历 view 元素时命中第一个「文本包含目标串」的元素即 tap，但嵌套布局中外层容器的聚合文本同样包含目标串，且 $$ 按 DOM
序返回（父容器先出现），el.tap() 点击的是父容器中心，可能未落在勾选行上，导致同意勾选偶发未生效；脚本只对「行不可达」fail-fast，tap 之后未校验勾选状态是否翻转，失败会以
login 步骤 FAIL 形式呈现，根因难定位。建议优先匹配最内层元素（如倒序遍历或取文本长度最接近 wanted 的元素），并在 tap 后校验勾选状态（如复查
checkbox/登录页错误提示）再继续。

+ // 倒序取最内层匹配元素，避免命中外层容器
  async function tapListRow(page, wanted, timeoutMs = 20000) {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
-     for (const el of await page.$$('view')) {
-       try { if (((await el.text()) || '').includes(wanted)) { await el.tap(); return true; } } catch {}
+     const els = await page.$$('view');
+     for (let i = els.length - 1; i >= 0; i--) {
+       try { if (((await els[i].text()) || '').includes(wanted)) { await els[i].tap(); return true; } } catch {}
      }
      await sleep(900);
    }
    return false;
  }


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:75-76 ───
[maintainability · low] 凭据注入方式与头部注释宣称不一致：注释说「机器强相关路径/端口一律 env 注入（对齐 T24 live-driver 写法）」，且 automator
路径/端口/截图目录确实都有 T33_* 变量，但测试账号邮箱硬编码在源码中（密码已脱敏为 [REDACTED-disposable]，实际重放时同样需要还原到源码里）。对照 T24 的
T24R1_USER_A / T24R1_TOKA 注入模式，此处换环境重放登录步骤必须手改源码，破坏可重放性。

-     await fillInput(page, '请输入账号邮箱', 't33a@t33.io');
-     await fillInput(page, '请输入密码', '[REDACTED-disposable]');
+     await fillInput(page, '请输入账号邮箱', process.env.T33_USER_EMAIL || 't33a@t33.io');
+     await fillInput(page, '请输入密码', process.env.T33_USER_PASS || '[REDACTED-disposable]');


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:27-31 ───
[performance · low] allTexts 对页面全部 text 元素逐个串行 await 读取，各元素读取相互独立；在元素较多的页面上串行往返会拉长单次轮询周期，压缩 25s
waitText 窗口内的实际采样次数。可用 Promise.all 并行读取（若 automator 会话对并发命令不兼容则保留现状即可）。

  async function allTexts(page) {
-   const els = await page.$$('text'); const out = [];
-   for (const el of els) { try { const t = await el.text(); if (t && t.trim()) out.push(t.trim()); } catch {} }
-   return out.join('\n');
+   const els = await page.$$('text');
+   const texts = await Promise.all(els.map(el => el.text().catch(() => '')));
+   return texts.filter(t => t && t.trim()).map(t => t.trim()).join('\n');
  }


─── apps/embed/src/button.tsx:1-1 ───
[maintainability · low] 冗余的 React 默认导入：apps/embed 的 tsconfig 为 jsx: "react-jsx"，组件内未使用 React
命名空间（仅引用类型），同项目 EmbedApp.tsx 的惯例也是纯具名导入。保留默认导入会触发 no-unused-vars 类 lint 告警，建议移除。

- import React, { type ButtonHTMLAttributes, type ReactNode } from 'react';
+ import { type ButtonHTMLAttributes, type ReactNode } from 'react';


─── apps/embed/src/styles.css:43-44 ───
[maintainability · low] .embed-btn--text 的文字/hover 底色为亮色硬编码值，而 styles.css
明确支持暗色主题（:root[data-theme="dark"]）。当前 EmbedApp.tsx 的三个用点恰好被 .embed-icon-button/.embed-send 的
!important color/background 覆盖而未暴露；但 Button 是可复用导出组件，未来出现不带覆盖类的 variant="text" 用法时，#506078 文字在
#131923 深底上对比度约 2.5:1（低于 WCAG AA 4.5:1），hover 的近白 #f2f5fa 色块在暗色 UI
中也会突兀，与头注"视觉基线不变"的承诺在暗色场景不成立。建议补充暗色覆盖（示意值请按设计基线确认），或在 button.tsx 头注中显式说明暗色由调用方类接管。

  .embed-btn--text { border-color: transparent; background-color: transparent; padding-inline: 0.5rem; color: #506078; }
  .embed-btn--text:hover:not(:disabled), .embed-btn--text:focus-visible:not(:disabled) { background-color: #f2f5fa; }
+ :root[data-theme="dark"] .embed-btn--text { color: #a8b3c7; }
+ :root[data-theme="dark"] .embed-btn--text:hover:not(:disabled) { background-color: rgba(255,255,255,0.08); }


─── apps/miniprogram/config/index.ts:19-20 ───
[maintainability · low] realpathSync 在依赖未安装（miniprogram_dist 目录不存在）时直接抛原生 ENOENT，丢失了下一行 "run pnpm
install first" 的指引；该指引目前只覆盖 button/button.js 缺失的场景。建议先做 existsSync 预检查再
realpath，保证两类失败都拿到带修复指引的自定义错误。

- const tdesignDist=realpathSync(resolve(process.cwd(),'node_modules/tdesign-miniprogram/miniprogram_dist'));
- if(!existsSync(resolve(tdesignDist,'button/button.js')))throw new Error(`tdesign-miniprogram miniprogram_dist not found at ${tdesignDist} — run pnpm install first`);
+ const tdesignPkg=resolve(process.cwd(),'node_modules/tdesign-miniprogram/miniprogram_dist');
+ if(!existsSync(tdesignPkg))throw new Error(`tdesign-miniprogram miniprogram_dist not found at ${tdesignPkg} — run pnpm install first`);
+ const tdesignDist=realpathSync(tdesignPkg);


─── apps/miniprogram/config/index.ts:49-51 ───
[maintainability · low] readdirSync 在目录缺失（如 TDesign 升级后重构掉 common/miniprogram_npm 目录）时抛原生 ENOENT
而非这里带上下文的错误信息，构建失败的排查线索不友好。建议补一个 existsSync 检查把 "缺失" 与 "为空" 两种情况统一收敛到自定义错误。

  for(const dir of tdesignDirs){
-  if(!readdirSync(resolve(tdesignDist,dir)).length)throw new Error(`tdesign closure: empty dir ${dir}`);
+  const dirPath=resolve(tdesignDist,dir);
+  if(!existsSync(dirPath)||!readdirSync(dirPath).length)throw new Error(`tdesign closure: missing or empty dir ${dirPath}`);
  }


─── apps/miniprogram/config/index.ts:54-54 ───
[maintainability · low] copy 目标路径硬编码了 `dist/` 前缀，与下方 defineConfig 中 outputRoot:'dist' 形成隐式耦合：一旦调整
outputRoot，copy 产物仍落在旧目录而构建不会报错，只会表现为页面运行时组件 404 白屏（正是注释里 D1 类故障）。建议提取共享常量（或直接复用
outputRoot）派生该前缀，消除两处硬编码。

- const withTdesignCopy={copy:{patterns:tdesignDirs.map(dir=>({from:resolve(tdesignDist,dir),to:`dist/npm/tdesign/${dir}`,ignore:['**/*.d.ts']})),options:{}}};
+ const outputRoot='dist';
+ const withTdesignCopy={copy:{patterns:tdesignDirs.map(dir=>({from:resolve(tdesignDist,dir),to:`${outputRoot}/npm/tdesign/${dir}`,ignore:['**/*.d.ts']})),options:{}}};
+ // defineConfig 中相应使用 outputRoot


─── apps/miniprogram/src/adapters/career-platform.ts:235-240 ───
[performance · medium] openExportedDocument 的下载校验链有三处问题：(a) 20MB 上限检查发生在完整下载之后——授权里已有服务端权威
size（decodeMaterialExportDownload 已校验为安全整数），应在发起下载前用 grant.size 预检；同次更新的 T06 files.ts
openProtectedDocument 还用 task.onProgressUpdate 做了渐进中止（超限即 abort），本实现没有，异常超大响应会先完整落盘再被拒绝；(b) 达上限的
20MB 走同步 readFileSync + 纯 JS sha256 单线程计算（约 32 万分组 × 64 轮），weapp JS
线程可冻结数秒；FileSystemManager.readFile 支持 position/length，可分段异步读取；(c) 本函数与 files.ts
的「认证下载→错误码读取→USER_DATA_PATH 私有副本→打开→立即清理」流程几乎整段重复且已经漂移（对方有渐进中止、本实现没有），建议抽取共用 seam 而非维护两份。

+   if (grant.size > MAX_EXPORT_BYTES) throw new Error('文件超出本端查看上限');
    const outcome = await taroDownload({ url: `${parts.origin()}${path}`, header: { Authorization: `Bearer ${parts.bearer()}` }, timeout: 60000 });
-   const info = await Taro.getFileInfo({ filePath: outcome.tempFilePath }) as { size: number };
-   if (info.size > MAX_EXPORT_BYTES) throw new Error('文件超出本端查看上限');
-   const buffer = fs.readFileSync(outcome.tempFilePath) as unknown as ArrayBuffer;
-   const bytes = new Uint8Array(buffer);
-   const actualDigest = sha256Hex(bytes);


─── apps/miniprogram/src/adapters/career-platform.ts:2-3 ───
[maintainability · low] 同一文件混用两种包边界口径：ports 类型（NativeFileSource）走 `@weknora/api-client` 包名，career
合同却走 4 层相对路径直达源文件。根因是 api-client 的 index.ts 目前只导出 createCareerApi/CareerRequest 与 career-core
契约类型，career.ts 的解码器与类型面未导出，深层相对导入是被迫的；但这批相对路径在目录移动或包 exports
收紧时会集体编译失败，且同一模块经两种说明符解析存在双实例隐患（当前仅类型导入未触发）。建议在 index.ts 补齐 career 面导出后统一走包名。

- import type { NativeFileSource } from '@weknora/api-client';
- import type { MaterialBody, RuleStatus, SetRuleReceipt, RuleView, UsageEstimateView, ReminderReceipt, ReminderList, ReminderSourceKind } from '../../../../packages/api-client/src/career.ts';
+ // packages/api-client/src/index.ts 先补齐导出：
+ // export { decodeApplicationReceipt, decodeMaterialReceipt, decodeSetRuleReceipt, ... } from './career.ts';
+ // export type { ApplicationReceipt, MaterialBody, SetRuleReceipt, ... } from './career.ts';
+ import type { NativeFileSource, MaterialBody, SetRuleReceipt } from '@weknora/api-client';


─── apps/miniprogram/src/adapters/career-platform.ts:48-51 ───
[low] “仅当解码后更短才采用”挡不住原文含字面 %XX 的场景：如 JD 原文“绩点前20%2024届”中 %20+“24” 会解码成空格且结果更短（9 字 → 7 字），“100%25”
同理变成“100%”，用户原文被静默篡改后进入导入链——与注释声称的“避免把用户原文里的字面 '%' 误解码”不符。更稳的判据：整体被 percent-encoding 的载荷必然是纯
ASCII，原文含非 ASCII（JD 是中文文本）即说明并非整体编码，直接按原文返回；纯 ASCII 时再叠加“更短”启发式。

+ function decodeEntryPayload(raw: string): string {
+   // 整体被 percent-encoding 的载荷必为纯 ASCII；原文含非 ASCII（如中文）说明并非
+   // 整体编码，即使含字面 %20/%25 也不解码，避免“前20%2024届”被截成“前20 24届”。
+   if (!/[\x80-\uFFFF]/.test(raw)) return raw;
+   if (!/%[0-9A-Fa-f]{2}/.test(raw)) return raw;
    try {
      const decoded = decodeURIComponent(raw);
      return decoded.length < raw.length ? decoded : raw;
    } catch { return raw; }
+ }


─── apps/miniprogram/src/career/application-material.tsx:57-59 ───
[performance · medium] 渲染期同步读取受控存储：pendingApplication()/pendingMaterialWrite()/pendingSubmission()
每次渲染共执行 3 次 Taro.getStorageSync 同步 IO，属渲染期副作用。本页小节编辑的每个 Field onChange（每键入一个字符触发 setSections →
重渲染）都会重复这 3 次同步读。建议把 pending 快照放入 state（初始与 useDidShow 时读取，各写入/对账 action 落定后刷新），渲染期只消费快照，保持渲染纯函数。

-   const pendingApplication = career.pendingApplication();
-   const pendingMaterial = career.pendingMaterialWrite();
-   const pendingSubmission = career.pendingSubmission();
+ const [pendings, setPendings] = useState(() => ({
+   application: career.pendingApplication(),
+   material: career.pendingMaterialWrite(),
+   submission: career.pendingSubmission(),
+ }));
+ useDidShow(() => setPendings({
+   application: career.pendingApplication(),
+   material: career.pendingMaterialWrite(),
+   submission: career.pendingSubmission(),
+ }));


─── apps/miniprogram/src/career/application-material.tsx:111-112 ───
[bug · low] 岗位/快照编号输入变更后不重置 evaluation：用户先评估岗位 A，再修改编号为 B 后，旧评估徽章仍显示“符合已识别条件”且可携旧 evaluationId
直接提交创建申请。服务端已兜底（internal/modules/career/application.go:194-196 校验评估与岗位/快照一致，不一致返回
invalid_request），无数据风险，但本地必然提交失败且徽章结论与即将提交的岗位不符，易误导。建议两处 onChange 时同步 setEvaluation(undefined)。

-       <Field label='岗位编号' value={opportunityId} onChange={setOpportunityId} placeholder='从分享导入后自动带入，或粘贴岗位编号' />
-       <Field label='快照编号' value={snapshotId} onChange={setSnapshotId} placeholder='岗位快照编号' />
+       <Field label='岗位编号' value={opportunityId} onChange={value => { setOpportunityId(value); setEvaluation(undefined); }} placeholder='从分享导入后自动带入，或粘贴岗位编号' />
+       <Field label='快照编号' value={snapshotId} onChange={value => { setSnapshotId(value); setEvaluation(undefined); }} placeholder='岗位快照编号' />


─── apps/miniprogram/src/career/application-material.tsx:251-251 ───
[bug · low] 导出列表中每个 format 的下载按钮共享同一个 downloadBusy：任一文件下载进行时所有下载按钮同时转圈，且 downloadBusy.error
也无法定位是哪个导出/格式失败。建议按 `${exportId}:${format}` 维度维护 busy/error，仅让被点击的按钮进入 loading 态。

-             <t-button size='medium' theme='default' ariaLabel={`下载并打开 ${file.format}`} customStyle={tdesignButtonStyle} loading={downloadBusy.busy} onTap={() => void downloadBusy.run(async () => {
+ const [downloadKey, setDownloadKey] = useState<string>();
+ // loading={downloadBusy.busy && downloadKey === `${exportReceipt.exportId}:${file.format}`}
+ // onTap 前置 setDownloadKey(`${exportReceipt.exportId}:${file.format}`)


─── apps/miniprogram/src/career/application-material.tsx:97-99 ───
[maintainability · low] 死代码：接收 reconcilePendingSubmission 返回值后仅以 void
丢弃，属无用赋值。对账成功后刷新投递记录只需要副作用，直接不接收返回值即可。

-         const receipt = await career.reconcilePendingSubmission();
+         await career.reconcilePendingSubmission();
          if (application) await loadSubmissions(application.applicationId);
-         void receipt;


─── apps/miniprogram/src/career/application-material.tsx:158-158 ───
[style · low] 嵌套三元表达式（a ? x : b ? y : z）违反嵌套三元禁令，且同文件已有 linkStateLabel/evaluationTone 的 Record
映射先例可循。建议提取 linkState 的 tone 映射，与 linkStateLabel 并列定义。

-         <Badge tone={application.linkState === 'ready' ? 'success' : application.linkState === 'linking' ? 'warning' : 'danger'}>{linkStateLabel[application.linkState]}</Badge>
+ const linkStateTone: Record<string, 'success' | 'warning' | 'danger'> = { ready: 'success', linking: 'warning', link_failed: 'danger' };
+ // <Badge tone={linkStateTone[application.linkState] ?? 'neutral'}>


─── apps/miniprogram/src/career/application-material.tsx:234-239 ───
[bug · high] 发布结果未知时没有任何恢复链，违反本页与 export-deletion
页反复声明的“结果未知的写入都可以用原请求编号对账或安全重发”。career.publishMaterial 走 writeRecoverable('export',
…)（services/career.ts:324）：歧义失败（超时/断网）会把 intent 持久化到 wk:career:export:<scope> 并抛
outcome_unknown，错误文案即“材料发布结果未知：请用原请求对账后再试”——但 services/career.ts 全文件没有任何 'export' kind 的
pending/对账/重试读取器（其余 application/material/submission/spaceExport/spaceDeletion 等 kind
均有成套恢复函数），本页也没有发布恢复入口，Web MaterialPage 同场景有“用原请求编号重试发布”（MaterialPage.tsx:559）。后果：a) 落盘的发布 intent
永不被读取/清除，成为死数据（仅登出清理）；b) 用户只能换新 requestId 盲目重发，若原发布实际已在服务端成功，同一版本会再次发布产生重复导出记录。建议在服务层补
pendingMaterialPublish()/reconcilePendingMaterialPublish()/retryPendingMaterialPublish()（kind
'export'，服务端已有 GET/POST 幂等合同），并在发布卡片按同页申请/材料/投递三个 pending 恢复块的样式渲染对账/重发入口。



─── apps/miniprogram/src/career/discovery.tsx:37-37 ───
[bug · medium] 分享 path 内嵌完整 JD 原文存在长度风险：encodeURIComponent 后每个汉字膨胀约 9 个字符，数百字的 JD 会使 path
达数千字符，超出微信分享卡片 path 实际可承载长度时会导致卡片打不开或 query
被截断。更严重的是截断后果是静默的：readSharedEntry（adapters/career-platform.ts）只处理 percent-encoding 解码、无长度校验，截断后的 JD
仍会进入“核对后导入”流程，存档的岗位原文不完整，破坏“先核对后提交”的完整性前提。建议对 rawText 设长度上限（超限时降级为不带 jd 的分享路径并提示接收方改用粘贴），或改为分享仅携带短
token 由服务端暂存原文。

-   useShareAppMessage(() => ({ title: draft ? `职位核对：${draft.preview.excerpt.slice(0, 20)}` : 'WeKnora 求职工作台', path: `/career/discovery${draft ? `?jd=${encodeURIComponent(draft.rawText)}` : ''}` }));
+ const SHARED_JD_MAX = 300; // encodeURIComponent 后约 2700 字符，仍在分享 path 安全范围内
+ useShareAppMessage(() => {
+   const jd = draft && draft.rawText.length <= SHARED_JD_MAX ? `?jd=${encodeURIComponent(draft.rawText)}` : '';
+   return { title: draft ? `职位核对：${draft.preview.excerpt.slice(0, 20)}` : 'WeKnora 求职工作台', path: `/career/discovery${jd}` };
+ });


─── apps/miniprogram/src/career/discovery.tsx:39-40 ───
[performance · medium] 渲染期同步读取受控存储：pendingSearch() 每次渲染都经 store.read → Taro.getStorageSync 执行同步
IO，属于渲染期副作用（React 渲染应保持纯函数）。本页 Field 输入、searchBusy 状态变化都会触发重渲染放大该开销；application-material.tsx 与
export-deletion.tsx 也采用同一模式（后者注释自述“每次渲染重读”）。建议把 pending 快照放进 state：初始/页面显示（useDidShow）时读取，写入、对账、重试等
action 落定后刷新，渲染期只消费快照。

-   const pendingFactAction = career.pendingAction();
-   const pendingSearch = career.pendingSearch();
+ const [pendingSearch, setPendingSearch] = useState(() => career.pendingSearch());
+ useDidShow(() => setPendingSearch(career.pendingSearch()));
+ // 各 action 完成回调中同步刷新：setPendingSearch(career.pendingSearch())


─── apps/miniprogram/src/career/discovery.tsx:17-17 ───
[maintainability · low] 三页（discovery/application-material/export-deletion）各自复制定义完全相同的
tdesignButtonStyle、digestHead、typedCode（artifact 子包还有第 4 份 tdesignButtonStyle），任一 CSS
变量名调整需同步多处。建议提取到共享模块（如 src/career/shared.ts）统一导出，三页改为 import。

- const tdesignButtonStyle = '--td-brand-color:var(--wk-color-action-primary);--td-brand-color-active:var(--wk-color-action-pressed);--td-brand-color-disabled:var(--wk-color-action-disabled);--td-button-primary-bg-color:var(--wk-color-action-primary);--td-button-primary-active-bg-color:var(--wk-color-action-pressed);--td-button-primary-disabled-bg-color:var(--wk-color-action-disabled);--td-button-primary-disabled-color:var(--wk-color-action-disabled-text);--td-button-large-height:var(--wk-component-button-height);';
+ // src/career/shared.ts
+ export const tdesignButtonStyle = '--td-brand-color:var(--wk-color-action-primary);…';
+ export const digestHead = (digest: string): string => digest.slice(0, 12);
+ export const typedCode = (error: unknown): string | undefined => { … };
+ // 各页面：import { tdesignButtonStyle, digestHead, typedCode } from './shared.ts';


─── apps/miniprogram/src/career/discovery.tsx:43-43 ───
[maintainability · low] quotaRefused 靠匹配 errorMessage 渲染文案的中文子串“搜索额度不足”来决定 Notice 降为 warning——行为分支与
core/errors.ts 的提示措辞强耦合：措辞微调会静默把 typed 429（search_quota_refused）专属提示降回 danger
红色，且无编译期保护（代码注释也自述了这一脆弱性）。建议让 useAction 的错误态保留原始错误对象或 typed code（如 error 扩展为 { message, code }），页面按
code === 'search_quota_refused' 分支，与 typed 错误合同的其余用法保持一致。

-   const quotaRefused = (message?: string) => message?.includes('搜索额度不足') ?? false;
+   // useAction 额外暴露 code（errorMessage 的输入即含 code），页面按 typed code 分支：
+   const quotaRefused = (code?: string) => code === 'search_quota_refused';


─── apps/miniprogram/src/career/discovery.tsx:129-129 ───
[style · low] 覆盖来源一行内是嵌套三元（available 三元的 false 分支里再嵌 failureCode 三元），违反嵌套三元禁令；本文件已有
qualificationLabel/qualificationTone 等 Record 映射先例。建议提取 helper 拍平，保持模板可读。

-         <Text className='wk-muted wk-small'>覆盖来源（{searchOut.sources.length}）：{searchOut.sources.map(source => `${source.label}${source.available ? '' : `（不可用${source.failureCode ? `：${source.failureCode}` : ''}）`}`).join('、') || '无'}</Text>
+         <Text className='wk-muted wk-small'>覆盖来源（{searchOut.sources.length}）：{searchOut.sources.map(sourceLabel).join('、') || '无'}</Text>
+ // 与 CHANNELS 等常量并列定义：
+ const unavailableNote = (failureCode?: string) => (failureCode ? `：${failureCode}` : '');
+ const sourceLabel = (source: { label: string; available: boolean; failureCode?: string }) =>
+   source.available ? source.label : `${source.label}（不可用${unavailableNote(source.failureCode)}）`;


─── apps/miniprogram/src/career/export-deletion.tsx:55-57 ───
[performance · medium] 渲染期同步读取受控存储：注释自述“每次渲染重读”，pendingSpaceExport()/pendingSpaceDeletion() 每次渲染执行 2
次 Taro.getStorageSync 同步 IO，属渲染期副作用（React 渲染函数应保持纯函数，且与 gating 计算耦合后对渲染频率敏感）。由于恢复态只在写入落 intent /
对账或重试落定后才变化，完全可以在这些 action 完成回调与页面显示（useDidShow）时刷新 state 快照，渲染期只消费快照，语义不变且消除每渲染 IO。

-   // 恢复态读取自受控存储：每次渲染重读（useAction 状态变化触发重渲染）。
-   const pendingExport = career.pendingSpaceExport();
-   const pendingDeletion = career.pendingSpaceDeletion();
+   const [pendingExport, setPendingExport] = useState(() => career.pendingSpaceExport());
+   const [pendingDeletion, setPendingDeletion] = useState(() => career.pendingSpaceDeletion());
+   // useDidShow 与各 action 落定后刷新：setPendingExport(career.pendingSpaceExport()) 等


─── apps/miniprogram/src/career/export-deletion.tsx:180-183 ───
[bug · medium] 剪贴板复制完整导出无长度防护：copySpaceExportToClipboard 直接把整份空间导出 JSON（含全部材料版本正文、岗位快照，轻易数百 KB）一次性
setClipboardData。微信剪贴板对超大内容可能被系统静默截断而不抛错——与保存文件路径（saveSpaceExportPackage
会回读落盘字节复算摘要）不同，此路径无任何完整性校验，而文案已承诺“完整导出内容（未截断）已复制”，截断时用户留档的是不完整 JSON。建议对超长载荷（如
>100KB）禁用该入口并引导使用“保存导出包到本机”，或复制后回读剪贴板校验长度一致才展示成功文案。

-         <Action secondary loading={copyBusy.busy} onClick={() => void copyBusy.run(async () => {
-           await copySpaceExportToClipboard(spaceExportPayload(exported));
+ onClick={() => void copyBusy.run(async () => {
+   const payload = spaceExportPayload(exported);
+   if (payload.length > COPY_MAX) { setCopyNotice('导出内容过大，剪贴板可能无法完整承载：请使用「保存导出包到本机」留档。'); return; }
+   await copySpaceExportToClipboard(payload);
-           setCopyNotice('完整导出内容（未截断）已复制到剪贴板，可粘贴到任何位置留存。');
+   setCopyNotice('完整导出内容（未截断）已复制到剪贴板，可粘贴到任何位置留存。');
-         })}>复制完整导出内容</Action>
+ })}


─── apps/miniprogram/src/career/export-deletion.tsx:220-222 ───
[style · low] 两处嵌套三元（Card tone 与 Badge tone 均为 a ? x : b ? y : z 形式）违反嵌套三元禁令；本文件已有
deletionStatusLabels/stepStatusLabels 等 Record 映射先例，建议为删除状态提取 card/badge tone 映射。

-     {deletion && <Card tone={deletion.status === 'deleted' ? 'mint' : deletion.status === 'partial' ? 'warning' : 'white'}>
-       <View className='wk-between'>
-         <Badge tone={deleted ? 'success' : deletion.status === 'partial' ? 'danger' : 'warning'}>{deletionStatusLabels[deletion.status]}</Badge>
+ const deletionCardTone: Record<string, 'mint' | 'warning' | 'white'> = { deleted: 'mint', partial: 'warning', deleting: 'white' };
+ const deletionBadgeTone: Record<string, 'success' | 'danger' | 'warning'> = { deleted: 'success', partial: 'danger', deleting: 'warning' };
+ // <Card tone={deletionCardTone[deletion.status] ?? 'white'}>
+ //   <Badge tone={deletionBadgeTone[deletion.status] ?? 'neutral'}>


─── apps/miniprogram/src/career/progress-preparation.tsx:142-144 ───
[bug · medium] saveRevision 的 try 边界有两处问题：① 成功路径的回读 `setRevisedBody((await
career.material(materialId)).body)` 在 try 内——此时 editMaterial 已成功、本地草稿已 clear，若这次回读 GET
恰好失败（网络抖动），会落入 catch 被「当作保存失败」呈现：reviseErrCode 被设置、looksOffline
时还会显示「已保留本地草稿（可继续编辑，未提交）」，与事实相反（修订已提交且草稿已清），违背本页「如实呈现、不误导」的口径。同文件 retryMatBusy 处理器已有正确先例（回读单独
try/catch 吞错，注释「回读失败不掩埋重试成功的事实」），此处应保持一致。② claims 兜底取回 `(await career.material(materialId))` 在 try
之外：该 GET 失败时 reviseErrCode 不会被设置、断网提示文案也不会显示，用户只能看到裸错误（此时本地草稿已写入却无 outcome_unknown/断网口径引导）。建议：把
claims 取回移入 try（失败走统一分型），把提交成功后的回读移出 try 单独吞错。

+     try {
+       if (body.sections.some(section => (section.claims ?? []).length === 0)) {
+         body = { sections: recoverEmptyClaims(body.sections, (await career.material(materialId)).body.sections) };
+       }
+       await career.editMaterial({ materialId, body });
+       clearPreparationDraft(editTarget!.applicationId, editTarget!.focus);
+       setDraftNotice(''); setReviseErrCode(undefined);
        setGenNotice('准备草稿修订已提交（仍是可审阅草稿，发布需另行确认材料版本）。');
-       // 与 Web 同语义：回执只是回声，修订的持久事实从材料域回读。
-       setRevisedBody((await career.material(materialId)).body);
+       void listBusy.run(loadPreparations);
+     } catch (error) {
+       setReviseErrCode(typedCode(error));
+       if (looksOffline(error)) setDraftNotice('网络不可用：已保留本地草稿（可继续编辑，未提交）。申请进展没有任何改动；联网后请再点「保存修订」显式同步，不会自动提交。');
+       throw error;
+     }
+     // 与 Web 同语义：回执只是回声；回读失败不掩埋提交成功的事实（与页首 retryMatBusy 同做法）。
+     try { setRevisedBody((await career.material(materialId)).body); } catch { /* 回读失败不影响已提交事实，可稍后用「回读材料草稿正文」重试 */ }


─── apps/miniprogram/src/career/progress-preparation.tsx:215-215 ───
[bug · low] `try { void Taro.pageScrollTo(...) } catch {}` 的同步 catch 兜不住 Promise 拒绝：`void` 丢弃了
promise，pageScrollTo 失败（如选择器未命中）时会产生未处理的 Promise rejection，与注释「滚动失败不阻断入口」的意图不符。应改为 `.catch(() =>
{})`。

-               {event.eventType === 'interview' ? <Text className='wk-small' onClick={() => { setFocus('interview_prep'); setFromTimeline(true); try { void Taro.pageScrollTo({ selector: '#wk-preparation', duration: 300 }); } catch { /* 滚动失败不阻断入口 */ } }}>为这场面试做准备 ›</Text> : <Text />}
+ {event.eventType === 'interview' ? <Text className='wk-small' onClick={() => { setFocus('interview_prep'); setFromTimeline(true); Taro.pageScrollTo({ selector: '#wk-preparation', duration: 300 }).catch(() => { /* 滚动失败不阻断入口 */ }); }}>为这场面试做准备 ›</Text> : <Text />}


─── apps/miniprogram/src/career/progress-preparation.tsx:200-200 ───
[maintainability · medium] 此处是两层嵌套三元（viewErrCode === 'forbidden' ? ... : viewErrCode === 'not_found'
? ... : ...），违反「禁止嵌套三元」规范；同文件还有多处同类：genErrCode 提示链、reviseErrCode 提示链、item.status 三态展示（'草稿（可审阅、可修订）'
: '生成失败…' : '生成中'）。typed 错误码→提示文案本质是查表映射，建议提取 Record 映射或小函数，新增错误码时也只需加一行。

-       {loadBusy.error && <Notice tone='danger'>{loadBusy.error}{viewErrCode === 'forbidden' ? ' 当前空间不可访问此申请的进展。' : viewErrCode === 'not_found' ? ' 未找到此申请（可能不属于当前空间）。可核对编号后重试。' : ' 可稍后重试。'}</Notice>}
+ const progressLoadHints: Record<string, string> = {
+   forbidden: ' 当前空间不可访问此申请的进展。',
+   not_found: ' 未找到此申请（可能不属于当前空间）。可核对编号后重试。',
+ };
+ // 渲染处：
+ // {loadBusy.error && <Notice tone='danger'>{loadBusy.error}{(viewErrCode && progressLoadHints[viewErrCode]) ?? ' 可稍后重试。'}</Notice>}


─── apps/miniprogram/src/career/progress-preparation.tsx:64-66 ───
[performance · low] 渲染体每次重渲染都会执行 3 次 pendingXxxWrite()（内部为同步 Taro.getStorageSync，经
createControlledStore→storage.read），加上 localDraft 的 readPreparationDraft 共 4 次同步 storage
IO；本页任何输入框敲一个字都会触发重渲染。与兄弟页面（application-material.tsx 顶部同款）是同一既有约定，但建议至少把 localDraft 用 useMemo 按
[applicationId, focus] 缓存，pending 读取后续与兄弟页面统一收敛为 state + 动作后刷新，避免每次按键都打同步存储。

    const pendingProgress = career.pendingProgressWrite();
    const pendingPreparation = career.pendingPreparationWrite();
    const pendingMaterial = career.pendingMaterialWrite();
+   // 同步 storage 读取收敛：仅在申请/焦点变化时重读本地草稿
+   const localDraft = useMemo(
+     () => applicationId.trim() ? readPreparationDraft(applicationId.trim(), focus) : undefined,
+     [applicationId, focus],
+   );


─── apps/miniprogram/src/career/progress-preparation.tsx:119-120 ───
[maintainability · low] draftFromSections 与修订卡片、saveRevision 中对 editTarget! 的非空断言共 6 处以上，全部依赖渲染条件
(editing || draftEditing) 间接保证；任何一处重构（如把按钮移出该 Card、提取子组件）都会让断言在运行时变成 undefined 崩溃。建议在入口一次收窄：handler
开头 `const target = editing ?? draftEditing; if (!target) throw/return;`，后续全部使用 target，消除分散断言。

-   const draftFromSections = (): PreparationDraftRecord => ({
-     applicationId: editTarget!.applicationId, focus: editTarget!.focus,
+   const saveRevision = async (): Promise<void> => {
+     const target = editing ?? draftEditing;
+     if (!target) throw new Error('没有正在修订的准备草稿');
+     const materialId = target.materialId;
+     if (!materialId) throw new Error('本地草稿缺少材料编号：请联网读取准备列表后再保存修订');
+     …
+     clearPreparationDraft(target.applicationId, target.focus);
+   };


─── apps/miniprogram/src/career/progress-preparation.tsx:305-305 ───
[maintainability · medium] 「保存修订」主按钮未按 pendingMaterial 禁用：页首提示『保存准备修订走同一恢复链』，但 pendingMaterial
未对账期间用户仍可再次点保存（新请求编号的新写入），随后页首『用原请求编号重试材料修订』会以原正文重放，新旧两次写入的内容与落地次序容易让用户误判最终正文。同页『录入进展/提交更正』按钮（对应
pendingProgress）与『生成准备草稿』按钮（对应 pendingPreparation，其 disabled 仅查 applicationId/revision）同样未在 pending
存在时禁用——而同 PR 的 rules-usage-reminders.tsx 已按 T32-M1 口径（『未知/对账中主按钮禁用+重新对账入口』）对保存规则、登记待办做了 `pendingXxx
!== null` 禁用，两端口径不一致。建议对齐：恢复未决时禁用对应主按钮，引导先对账/重试。

-         <t-button block size='large' theme='primary' ariaLabel='保存准备草稿修订' customStyle={tdesignButtonStyle} loading={reviseBusy.busy} onTap={() => void reviseBusy.run(saveRevision)}>保存修订（显式提交）</t-button>
+         <t-button block size='large' theme='primary' ariaLabel='保存准备草稿修订' customStyle={tdesignButtonStyle} loading={reviseBusy.busy} disabled={pendingMaterial !== null} onTap={() => void reviseBusy.run(saveRevision)}>保存修订（显式提交）</t-button>


─── apps/miniprogram/src/career/progress-preparation.tsx:252-252 ───
[bug · low] fromTimeline 置位后没有重置路径：从时间线点『为这场面试做准备』后，用户再手动切换准备焦点（如切到求职信），『已从时间线进入：将为这场面试准备…』的 Notice
仍原样显示，与当前实际选择的焦点不符，构成误导。建议手动 setFocus 时一并重置，或让该提示文案跟随 focus 动态生成。

-       {(Object.keys(focusLabels) as PreparationFocus[]).map(option => <Text key={option} className='wk-small' onClick={() => setFocus(option)}>{focus === option ? '● ' : '○ '}{focusLabels[option]}</Text>)}
+       {(Object.keys(focusLabels) as PreparationFocus[]).map(option => <Text key={option} className='wk-small' onClick={() => { setFocus(option); setFromTimeline(false); }}>{focus === option ? '● ' : '○ '}{focusLabels[option]}</Text>)}


─── apps/miniprogram/src/career/progress-preparation.tsx:21-21 ───
[maintainability · low] 重复代码：tdesignButtonStyle 这段 TDesign 主题 CSS 变量串（及本文件的 typedCode 工具函数）与本组另一文件
rules-usage-reminders.tsx 逐字重复，且 application-material.tsx、export-deletion.tsx、discovery.tsx
也是同款拷贝（全目录 4–5 份）。主题变量一旦调整需同步改多处，易漏改不一致。建议提取到共享模块（如 src/career/shared.ts 或并入 components/ui.tsx）统一导出。

- const tdesignButtonStyle = '--td-brand-color:var(--wk-color-action-primary);--td-brand-color-active:var(--wk-color-action-pressed);--td-brand-color-disabled:var(--wk-color-action-disabled);--td-button-primary-bg-color:var(--wk-color-action-primary);--td-button-primary-active-bg-color:var(--wk-color-action-pressed);--td-button-primary-disabled-bg-color:var(--wk-color-action-disabled);--td-button-primary-disabled-color:var(--wk-color-action-disabled-text);--td-button-large-height:var(--wk-component-button-height);';
+ import { tdesignButtonStyle, typedCode } from './shared.ts'; // 与 rules-usage-reminders.tsx 等页面共享同一份定义


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:160-160 ───
[maintainability · medium] 此处是三层嵌套三元（ruleErrCode === 'revision_conflict' ? … :
'idempotency_conflict' ? … : 'outcome_unknown' ? … : ''），违反「禁止嵌套三元」规范；同文件还有多处同类：subscription.reason
两层链（no_templates/api_unavailable/默认）、live.status 三态 next 运行计划链、est**imate 提示链等。错误码→文案是典型查表映射，建议提取
Record 映射，三层链改独立变量或小函数。

-       {saveBusy.error && <Notice tone='danger'>{saveBusy.error}{ruleErrCode === 'revision_conflict' ? ' 档案已更新：请重新读取修订后再保存（新保存会使用新的请求编号）。' : ruleErrCode === 'idempotency_conflict' ? ' 本次请求与已保存的规则内容不一致，已放弃；请重新保存。' : ruleErrCode === 'outcome_unknown' ? ' 保存结果未知：请用页首「用原请求对账规则保存」恢复（幂等可重放），本页不会自动重发。' : ''}</Notice>}
+ const ruleSaveHints: Record<string, string> = {
+   revision_conflict: ' 档案已更新：请重新读取修订后再保存（新保存会使用新的请求编号）。',
+   idempotency_conflict: ' 本次请求与已保存的规则内容不一致，已放弃；请重新保存。',
+   outcome_unknown: ' 保存结果未知：请用页首「用原请求对账规则保存」恢复（幂等可重放），本页不会自动重发。',
+ };
+ // 渲染处：
+ // {saveBusy.error && <Notice tone='danger'>{saveBusy.error}{(ruleErrCode && ruleSaveHints[ruleErrCode]) ?? ''}</Notice>}


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:65-66 ───
[performance · low] 渲染体每次重渲染执行 pendingRuleWrite()/pendingReminderWrite() 两次同步 storage 读取（内部
Taro.getStorageSync），且 JSX 中 `{readStoredRuleId() ? '读回已保存的规则' : '读取规则'}` 也在渲染期直接同步读存储；本页输入
query/间隔/来源编号每个按键都会触发。建议：按钮文案的 readStoredRuleId() 用 useMemo（以 receipt/ruleView 为刷新信号）缓存，pending
读取后续与兄弟页面统一收敛为 state + 动作后刷新。

-   const pendingRule = pendingRuleWrite();
-   const pendingReminder = pendingReminderWrite();
+   const hasStoredRule = useMemo(() => readStoredRuleId() !== undefined, [receipt, ruleView]);
+   // JSX：
+   // <Action secondary loading={loadRuleBusy.busy} onClick={() => void loadRuleBusy.run(loadRule)}>{hasStoredRule ? '读回已保存的规则' : '读取规则'}</Action>


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:78-78 ───
[maintainability · low] usageErrCode 是死状态：loadEstimate 失败时写入 `typedCode(error) ?? 'error'`，但整个 JSX
没有任何地方读取 usageErrCode（预估失败的提示只依赖 usageBusy.error），`?? 'error'` 兜底值也无意义。对比同文件 ruleErrCode
有真实分型消费（not_found/forbidden 定制文案），此处要么删除该 state（连同 useState 声明与 `setUsageErrCode(undefined)`），要么像
loadRule 一样真正用于分型提示。

-     try { setEstimate(await fetchUsageEstimate()); } catch (error) { setUsageErrCode(typedCode(error) ?? 'error'); throw error; }
+     try { setEstimate(await fetchUsageEstimate()); } catch (error) { throw error; }


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:231-231 ───
[bug · low] 「登记站内待办」未校验 sourceId：按钮 disabled 只查 revision 与
pendingReminder，来源事件编号为空或纯空格也能直接提交，只能等服务端报错才暴露；且提交值未 trim。同文件保存规则按钮对 query 做了 `!query.trim()`
校验，此处口径不一致。

-       <Action secondary loading={remindBusy.busy} disabled={revision === undefined || pendingReminder !== null} onClick={() => void remindBusy.run(async () => {
+       <Action secondary loading={remindBusy.busy} disabled={!sourceId.trim() || revision === undefined || pendingReminder !== null} onClick={() => void remindBusy.run(async () => {


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:26-26 ───
[maintainability · low] 同款重复：本文件的 tdesignButtonStyle 与 typedCode 与本组另一文件 progress-preparation.tsx（及
application-material.tsx、export-deletion.tsx 等）逐字重复，建议在按上一条提取共享模块时一并收敛本文件。

- const tdesignButtonStyle = '--td-brand-color:var(--wk-color-action-primary);--td-brand-color-active:var(--wk-color-action-pressed);--td-brand-color-disabled:var(--wk-color-action-disabled);--td-button-primary-bg-color:var(--wk-color-action-primary);--td-button-primary-active-bg-color:var(--wk-color-action-pressed);--td-button-primary-disabled-bg-color:var(--wk-color-action-disabled);--td-button-primary-disabled-color:var(--wk-color-action-disabled-text);--td-button-large-height:var(--wk-component-button-height);';
+ import { tdesignButtonStyle, typedCode } from './shared.ts';


─── apps/miniprogram/src/platform/files.ts:66-68 ───
[style · low] 401 错误码映射使用了嵌套三元表达式（外层 statusCode===401 ? 内层三元 : undefined），违反检查清单“禁止嵌套三元”。建议改为 if
语句或先算内层再判断，提升可读性。

-        const code=r.statusCode===401
-         ?responseCode==='artifact_grant_expired'?'ARTIFACT_GRANT_EXPIRED':'ARTIFACT_GRANT_INVALID'
-         :undefined;
+        let code:string|undefined;
+        if(r.statusCode===401)code=responseCode==='artifact_grant_expired'?'ARTIFACT_GRANT_EXPIRED':'ARTIFACT_GRANT_INVALID';


─── apps/miniprogram/src/platform/files.ts:85-87 ───
[other · low] 新实现完全放弃对 downloadFile 落下的 tempFilePath 的清理（旧版至少有 unlinkSync 的静默尝试）。D3 证据来自 DevTools
运行时（unlink 一律 permission denied），但真机行为可能不同——tmp 文件在部分平台/基础库版本下可删除。建议保留一次尽力而为的 unlink 尝试（fail
静默忽略，成本为零），可缩短敏感产物字节在平台临时目录的滞留时间；若后续真机验证确认同样不可删，再彻底移除并在注释中补充真机结论。



─── apps/miniprogram/src/platform/files.ts:44-44 ───
[bug · medium] 新增的三条错误消息（'本机存储目录不可用'、'本机文件准备失败'、'临时副本清理失败：…'）均只写在 Error.message 中，而 UI 侧 useAction
展示错误统一经 core/errors.ts 的 errorMessage() 转换——该函数只按 status/code 及少数精确
message（API_ORIGIN_MISSING/AUTH_REQUIRED/SCOPE_CHANGED）映射，不透传
message。结果这三类失败最终都显示为兜底文案"操作未完成，请检查网络或刷新状态后重试。"，尤其 unlinkFile
上方注释明确承诺"清理失败不静默：违背承诺必须可见"，但用户实际只能看到通用文案，清理失败这一事件对用户不可辨。建议在 errors.ts 的 errorMessage 中为这些消息补充映射，或像
401 分支那样以 code 属性传递并映射。

-  return new Promise((resolve,reject)=>fs.unlink({filePath,success:()=>resolve(),fail:e=>reject(Object.assign(new Error(`临时副本清理失败：${filePath}`),{cause:e}))}));
+ // core/errors.ts 中补充：
+ // if(error instanceof Error&&error.message==='本机存储目录不可用')return '本机存储目录不可用，请重试或更新客户端。';
+ // if(error instanceof Error&&error.message.startsWith('临时副本清理失败'))return '文件已在本地打开，但临时副本清理失败，请重新进入页面后重试。';


─── apps/miniprogram/src/services/career-intent.ts:75-80 ───
[bug · medium] recoverableWrite 首写侧只判作用域，没有套用 retryRecoverable 已有的 definiteLocalFailure
守卫，两者失败定性不一致。runtime.ts 的 AUTH_REQUIRED 是裸 message 的 Error（无 code/status，见 runtime.ts
L28/L51），ambiguousOutcome 会将其判为歧义：会话失效但 scope 未变（如同用户重新登录前、或 token 刷新失败阶段）时，一次从未离开设备的写入会落入 intent 并抛
outcome_unknown，引导用户去对账一个服务端不可能存在的回执——直接违背本文件 L43-44 注释冻结的口径「AUTH_REQUIRED 是确定失败」。services/career.ts
的 searchOnce/deleteWholeSpace 手写副本存在同样缺口。

-   try {
-     return await options.send(id, options.expected);
    } catch (error) {
-     if (ambiguousOutcome(error) && !auth.scope.isCurrent(stamp)) {
+     if (ambiguousOutcome(error) && (!auth.scope.isCurrent(stamp) || definiteLocalFailure(error))) {
        throw Object.assign(new Error('SCOPE_CHANGED'), { cause: error });
      }


─── apps/miniprogram/src/services/career-intent.ts:81-84 ───
[medium] recoverableWrite 落 intent 时未检查同键是否已有未决 intent：同 kind 的第二条写入再次歧义失败会直接 store.write 覆盖第一条的 {
requestId, expectedRevision }，把前一条“结果未知”的恢复句柄静默销毁——违背本模块自述的“歧义失败落
intent…绝不静默丢弃”与恢复链红线。该路径实际可达：进展录入/纠错、生成准备草稿（progress-preparation.tsx 的提交按钮均无 pending
禁用）、申请创建/材料编辑/投递确认（application-material.tsx 全文无 disabled 门控）都允许在 pending 期间再次提交；只有规则/待办页做了
`pendingRule !== null` 禁用，且共享 desk.mutate 对同一语义抛 unresolved_action 拒绝叠加。建议在进入发送前按共享实现统一拒绝（或至少在各页面强制
gating），而不是失败时无条件覆盖。

-     if (ambiguousOutcome(error)) {
-       store.write(key, { requestId: id, input: options.input, expectedRevision: options.expected });
-       throw unknownOutcomeError(options.describe, id, error);
-     }
+ export async function recoverableWrite<T>(store: ControlledCareerStore, options: RecoverableWriteInput<T>): Promise<T> {
+   const id = newRequestId();
+   const stamp = auth.scope.capture();
+   const key = intentKeyFor(options.kind, stamp);
+   // 与 desk.mutate 的 unresolved_action 同语义：同键已有未决 intent 时拒绝新写入，
+   // 防止第二条写入的歧义失败静默覆盖第一条的恢复句柄。
+   if (readStoredIntent(store, key)) throw Object.assign(new Error(`已有结果未知的${options.describe}：请先用原请求对账或放弃，再发起新写入`), { code: 'unresolved_intent', recoverable: true });


─── apps/miniprogram/src/services/career-intent.ts:72-72 ───
[low] ReuseId 是死参数：全仓检索无任何调用方传入 reuseId（career.ts 的 writeRecoverable、career-platform.ts 的
saveRule/createReminder 都不传），恢复链重试走的是独立的 retryRecoverable（从 intent 读原
requestId）。留着它会暗示一条并不存在的“复用编号新写入”路径，建议连同 RecoverableWriteInput.reuseId 字段一并删除。

-   const id = options.reuseId ?? newRequestId();
+   const id = newRequestId();


─── apps/miniprogram/src/services/career.ts:136-140 ───
[maintainability · low] searchOnce/retryPendingSearch（以及 deleteWholeSpace，其 partial 终态语义除外）手写了
career-intent.ts 已沉淀的恢复链守卫，且首写侧同样缺 definiteLocalFailure 守卫（见对 career-intent.ts 的意见）；pendingSearch
也手写 intent 解析而未复用同文件已导入的 readStoredIntent。本文件与 career-platform.ts 的注释均自述「两套副本漂移已产出
high-7/high-8，已合并共用一份实现」，这里是残留的第三套变体，正是再漂移的起点。search 恢复没有终态特例，可直接迁移到
recoverableWrite/retryRecoverable。

-   } catch (error) {
-     if (ambiguousOutcome(error) && !auth.scope.isCurrent(stamp)) {
-       throw Object.assign(new Error('SCOPE_CHANGED'), { cause: error });
+ export async function searchOnce(query: string): Promise<SearchOutcome> {
+   const trimmed = query.trim();
+   if (!trimmed) throw new Error('请先输入想找的岗位或要求');
+   return recoverableWrite<SearchOutcome>(store, {
+     kind: 'search', describe: '搜索', input: { query: trimmed }, expected: revision(),
+     send: async id => decodeSearchOutcome(await client.request({ method: 'POST', path: '/api/v1/career/searches', body: { requestId: id, query: trimmed, expectedRevision: revision() } })),
+   });
-     }
+ }
-     if (ambiguousOutcome(error)) { store.write(searchKey(stamp), { requestId: id, query: trimmed, expectedRevision: expected }); throw unknownOutcome(id, trimmed, error); }


─── apps/miniprogram/src/services/career.ts:339-341 ───
[maintainability · low] `grant as ExportDownloadGrant` 依赖两份「同形」接口靠人工维持：api-client 的
MaterialExportDownload 与 career-platform.ts 的影子接口任一侧增删字段，编译器都不会报警。本文件本就深度导入 api-client
源码，可直接引用其类型消除影子副本。顺带：openExportedDocument 对 grant.url 做了前缀校验但实际兑付路径由 materialId/exportId
重建，该校验只覆盖一个从未被消费的字段（decodeMaterialExportDownload 已做同等校验），语义上易误导读者以为 url 参与了请求构造。

-   const grant = await issueExportGrant(materialId.trim(), exportId.trim(), format);
-   try {
-     return { grant, check: await openExportedDocument(grant as ExportDownloadGrant) };
+ // adapters/career-platform.ts:
+ // import type { MaterialExportDownload } from '../../../../packages/api-client/src/career.ts';
+ // export type ExportDownloadGrant = MaterialExportDownload;
+   return { grant, check: await openExportedDocument(grant) };


─── apps/miniprogram/src/subpackages/execution/artifact/index.tsx:35-35 ───
[maintainability · medium] 这段 7 个 CSS 变量的 TDesign 主题覆盖串已是第 6
份逐字拷贝：career/application-material.tsx、discovery.tsx、export-deletion.tsx、progress-preparation.tsx、rul
es-usage-reminders.tsx 中均以模块级 `const tdesignButtonStyle`
重复定义，此处又以内联字符串形式再添一份（内联形式最容易被全局搜索遗漏）。建议提取为共享导出常量（如放入 core/ui 层的公共模块），各页面统一引用，避免未来调色板或尺寸变量变更时出现遗漏漂移。

-      customStyle='--td-brand-color:var(--wk-color-action-primary);--td-brand-color-active:var(--wk-color-action-pressed);--td-brand-color-disabled:var(--wk-color-action-disabled);--td-button-primary-bg-color:var(--wk-color-action-primary);--td-button-primary-active-bg-color:var(--wk-color-action-pressed);--td-button-primary-disabled-bg-color:var(--wk-color-action-disabled);--td-button-primary-disabled-color:var(--wk-color-action-disabled-text);--td-button-large-height:var(--wk-component-button-height);'
+      customStyle={tdesignButtonStyle} // 从共享模块导入，与 career 各页面共用同一份定义


─── apps/miniprogram/src/subpackages/execution/artifact/index.tsx:30-30 ───
[maintainability · low] `supported(file.name,file.mime)` 在同一张卡片内重复计算 3 次（disabled、ariaLabel、children
文案）。建议在 map 回调开头提取局部变量，减少重复调用并提升可读性（map 回调需从表达式改为带 return 的语句体）。

-      disabled={action.busy || !supported(file.name,file.mime)}
+     {items.map(file=>{const ok=supported(file.name,file.mime);return <Card key={`${file.index}:${file.id}`}>
+      ...
+      disabled={action.busy || !ok}
+      ariaLabel={ok?`打开或保存 ${file.name}`:`${file.name} 暂不支持`}
+      ...
+      {ok?'打开或保存':'此格式暂不支持'}


─── apps/miniprogram/src/subpackages/execution/artifact/index.tsx:9-9 ───
[bug · medium] 按钮可用性判定（supported）与 openProtectedDocument 的实际打开能力判定使用两套不一致的标准：supported 是"mime 匹配 OR
扩展名匹配"，而 openProtectedDocument 只认文件扩展名白名单
['pdf','doc','docx','xls','xlsx','ppt','pptx']（toUserCopyPath 与 openDocument 也依赖扩展名识别类型）。当某产物的 mime
为 application/pdf（或 wordprocessingml.document）但文件名无 .pdf/.docx 扩展名时（listTaskArtifacts 中 name
的默认值就是无扩展名的 '未命名文件'，mime 默认 application/octet-stream 但真实值来自服务端），supported 为 true、按钮可点击，点击后
openProtectedDocument 必然抛出"此格式请使用授权 Web 工作台查看"，UI 启用状态与实际能力矛盾。建议两处统一以文件扩展名为准（mime 仅作展示），或将 supported
改为与白名单共享同一判定函数。

- const supported=(name:string,mime:string)=>/^application\/pdf$|officedocument\.wordprocessingml\.document$/i.test(mime)||/\.(pdf|docx)$/i.test(name);
+ // 与 openProtectedDocument 的扩展名白名单保持同一判定，避免"按钮可点但点击必失败"
+ const supported=(name:string)=>/\.(pdf|docx)$/i.test(name);


─── apps/miniprogram/src/subpackages/execution/artifact/index.tsx:48-48 ───
[bug · low] 在 action.error 后硬编码追加"授权过期时再次点击……会重新获取"会产生两处问题：当错误恰为 ARTIFACT_GRANT_EXPIRED
时，errorMessage
已返回"下载授权已过期，请再次点击『打开或保存』重新获取。"，拼接后同一句式连续出现两遍；而对其他错误（如网络失败、清理失败）则追加了与实际错误无关的授权提示，误导用户。建议移除该追加文案（error
s.ts 已为两种授权错误提供了精准提示），或仅在该次错误 code 为授权类时条件显示。

- {action.error&&<Notice tone='danger'>{action.error} 授权过期时再次点击“打开或保存”会重新获取。</Notice>}
+ {action.error&&<Notice tone='danger'>{action.error}</Notice>}


─── apps/miniprogram/src/subpackages/execution/artifact/t-button.d.ts:9-11 ───
[maintainability · low] 这份 t-button 的 JSX.IntrinsicElements 声明是整个 src 下唯一一份（declare global
全局生效），但被放置在 execution/artifact 子包目录内；career 目录下 5
个页面（application-material/discovery/export-deletion/progress-preparation/rules-usage-reminders）共 20+
处 <t-button> 的类型检查全部隐式依赖这份藏在另一个子包里的声明。一旦 artifact 页面被重构或删除，career 页面的类型将整体失效/报错，且全局搜索 t-button
类型定义时很难找到此处。建议将其移至全局类型聚合位置（如 src/types/ 或 core 层的 d.ts），使其作用域与声明范围一致。

- declare global {
-   namespace JSX {
-     interface IntrinsicElements {
+ // 移至 src/types/t-button.d.ts（全局声明应位于全局类型目录，而非某个子包页面目录）


─── apps/miniprogram/tests/application-material.test.mjs:37-37 ───
[maintainability · low] 死代码：materialView fixture 定义后全文件无任何引用（materialVersions 用例返回的是内联对象）。未引用的
fixture 会误导维护者以为存在对应读取断言，建议删除。

- const materialView = (over = {}) => ({ materialId: 'mat-1', status: 'confirmed', pinnedEvidence: matPin, body: materialBody, reviewRisks: [], versionCount: 2, versions: [{ version: 1, createdAt: T }, { version: 2, createdAt: T }], createdAt: T, updatedAt: T, ...over });
+ // 删除未引用的 materialView fixture（materialVersions 用例已内联返回数据）


─── apps/miniprogram/tests/application-material.test.mjs:395-395 ───
[test · low] 断言有效性：errorMessage 喂的是手工构造的 { code:'forbidden' }（无 status），而本用例真实 reject 出的错误是
ApiError{ status:403, code:'forbidden' }（api-client errors.ts errorFromResult 会带上 HTTP
status），errorMessage 对真实对象走 e.status===403 的专属分支「你没有访问当前空间资源的权限」，手工对象走的是兜底分支——断言「the error chain
renders a prompt」实际验证的不是 UI 将渲染的错误对象。同批 career-discovery D4b 已示范正确做法（捕获真实错误再喂 errorMessage），建议对齐。

-   assert.ok(errorMessage({ code: 'forbidden', message: 'forbidden' }).length > 0, 'the error chain renders a prompt');
+   const refused = await career.material('mat-9').catch(error => error);
+   assert.equal(errorCode(refused), 'forbidden');
+   assert.ok(errorMessage(refused).length > 0, 'the error chain renders a prompt');


─── apps/miniprogram/tests/artifact-cleanup.test.mjs:95-95 ───
[maintainability · low] isRuntimeTemp 与 helpers/taro-stub.mjs 的 isRuntimeTempPath
是两份相同的平台契约判定（http://tmp/、wxfile://tmp、/tmp/、tmp/）。该判定是 D3 修复的核心契约（运行时临时目录 unlink 被
deny），若日后在一处补充分支（如真机新临时路径前缀）而另一处遗漏，测试将与替身各自为政。建议从 taro-stub 导出 isRuntimeTempPath 并在此复用。

- function isRuntimeTemp(p) { return p.startsWith('http://tmp/') || p.startsWith('wxfile://tmp') || p.startsWith('/tmp/'); }
+ // helpers/taro-stub.mjs: export function isRuntimeTempPath(p) {…}
+ // 本文件：
+ import { stub, isRuntimeTempPath } from './helpers/taro-stub.mjs';
+ // 删除底部本地的 isRuntimeTemp，直接使用 isRuntimeTempPath(destPath)


─── apps/miniprogram/tests/artifact-cleanup.test.mjs:76-89 ───
[maintainability · low] 本用例把文件头部 freshLogin 的登录脚手架（me() 闭包 + /auth/login、/auth/me 路由 + no-route
兜底）整体复制了一份，唯一差别是 downloadFile 分支返回 401 错误体。同一文件里两份登录路由会随后续演进（登录字段/路径调整）各自漂移。建议给 freshLogin
增加一个下载响应参数，本用例只提供 401 分支。

+ // 参数化 freshLogin，401 用例只注入下载分支：
+ function freshLogin(downloadRespond = call => {
+   stub.state.fileContents.set(TEMP_PDF, 'PDF-CONTENT');
+   call.options.success({ statusCode: 200, tempFilePath: TEMP_PDF, header: {} });
+ }) {
    stub.reset();
    const me = () => ({ success: true, data: { user: { id: 'u1', username: 'Lin' }, tenant: { id: 1, name: 'Space' }, memberships: [] } });
    stub.use(call => {
      if (call.kind === 'request') {
        const path = new URL(call.options.url).pathname;
        if (path === '/api/v1/auth/login') { stub.succeed(call, { data: { success: true, data: { token: 't1', refresh_token: 'r1' } } }); return; }
        if (path === '/api/v1/auth/me') { stub.succeed(call, { data: me() }); return; }
        call.options.fail({ errMsg: `no route for ${path}` });
        return;
+     }
+     downloadRespond(call);
+   });
+   return runtime.auth.login('u@example.test', 'pw');
-     }
+ }
+ // 401 用例改为：
+ await freshLogin(call => {
-     stub.state.fileContents.set(TEMP_PDF, JSON.stringify({ success: false, code: 'artifact_grant_expired' }));
+   stub.state.fileContents.set(TEMP_PDF, JSON.stringify({ success: false, code: 'artifact_grant_expired' }));
-     call.options.success({ statusCode: 401, tempFilePath: TEMP_PDF, header: {} });
+   call.options.success({ statusCode: 401, tempFilePath: TEMP_PDF, header: {} });
-   });
+ });
-   await runtime.auth.login('u@example.test', 'pw');


─── apps/miniprogram/tests/build-output.test.mjs:92-96 ───
[test · medium] 该正则假设 button.js 的 common 依赖以 ESM `from"…"` 形态出现，但同一文件下方注释与 config/index.ts 的第 2
轮实测记录（"require args is 'tslib'"）都表明 tdesign miniprogram_dist 产物是 CommonJS `require("../common/…")`
形态。若产物中 common 依赖同为 require 形态（与 tslib 一致），此 matchAll 将一个都匹配不到，for 循环体不执行，"button
运行时依赖必须存在"的断言静默空转（假绿方向）。建议同时匹配 require 形态，并断言匹配数 > 0 防止正则与产物格式漂移。

-   const buttonJs = readFileSync(resolve(tdesignDir, 'button/button.js'), 'utf8');
-   for (const m of buttonJs.matchAll(/from"(\.\.\/common[^"]*)"/g)) {
+   const depMatches = [...buttonJs.matchAll(/(?:from|require\()\s*"(\.\.\/common[^"]*)"/g)];
+   assert.ok(depMatches.length > 0, 'button.js 未匹配到任何 ../common 依赖引用——正则与产物模块格式不符，断言正在空转');
+   for (const m of depMatches) {
      const dep = resolve(tdesignDir, 'button', m[1].endsWith('.js') ? m[1] : m[1] + '.js');
      assert.ok(existsSync(dep), `button 运行时依赖 ${m[1]} 必须存在`);
    }


─── apps/miniprogram/tests/career-discovery.test.mjs:59-61 ───
[maintainability · low] 死代码：errorCode 与 ambiguous 两个工具在本文件均无调用点（C2 用例直接断言 error.code ===
'outcome_unknown'，未经过 ambiguous 分类；errorCode 仅被 ambiguous 引用）。ambiguous 的存在还暗示本应有一处按 TIMEOUT/5xx
归类歧义结果的分类断言被内联绕过，容易误导后续维护。若确需歧义分类断言，应在 C2 中实际使用它；否则一并删除。

  const careerCall = suffix => stub.state.calls.filter(c => new URL(c.options.url).pathname.startsWith('/api/v1/career') && (!suffix || new URL(c.options.url).pathname.includes(suffix)));
- const errorCode = error => error?.code;
- const ambiguous = error => ['TIMEOUT', 'NETWORK_ERROR', 'CANCELLED'].includes(errorCode(error)) || errorCode(error) === 'outcome_unknown' || (error?.status === undefined || error.status >= 500);
+ // 删除未引用的 errorCode/ambiguous；若要保留歧义分类语义，应在 C2 断言中实际使用：
+ // assert.ok(ambiguous(caught), 'timeout 归类为歧义结果，走 requestId 对账');


─── apps/miniprogram/tests/career-discovery.test.mjs:241-247 ───
[maintainability · low] 重复场景：D4b 与紧邻的 D4 搭建完全相同的 429 quota 拒绝路由（open revision 0 + POST /searches 429
search_quota_refused），D4b 只是在其上追加了 errorMessage 渲染断言。两份 fixture 将来若只改其一（如错误码或 envelope 调整）会静默漂移。建议把
D4b 的 errorMessage 断言并入 D4 末尾，或抽取共享的 quota-refused setup 复用。



─── apps/miniprogram/tests/export-deletion.test.mjs:384-385 ───
[maintainability · low] 双源漂移：此处内联计算 deletionUnknown（`pendingSpaceDeletion() !== null &&
receipt.status !== 'partial'`），而页面实际消费的是同批引入的同源判定函数
deletionOutcomeUnknown（export-deletion.tsx:68、export-deletion.gating.ts:49，本文件 F1 用例的
pageDeletionUnknown 已包装该函数）。页面后续调整判定（例如新增维度）时，这条 M1 断言可能继续通过而不再代表真实门控行为。建议复用同源函数。

-   const partial = pageGatingFromServiceState({ deletionUnknown: career.pendingSpaceDeletion() !== null && receipt.status !== 'partial' });
+   const { deletionOutcomeUnknown } = fix2Gating;
+   const partial = pageGatingFromServiceState({
+     deletionUnknown: deletionOutcomeUnknown({ intentPresent: career.pendingSpaceDeletion() !== null, inMemoryStatus: receipt.status, recoveryUnresolved: false }),
+   });
    assert.equal(partial.deletionDisabled, false, 'a reported partial is not an unknown outcome (Web partial keeps the main action open)');


─── apps/miniprogram/tests/export-deletion.test.mjs:342-342 ───
[maintainability · low] 重复样板：M1/F1 共六个测试各自执行 Object.assign(<模块容器>, await
import('../src/career/export-deletion.gating.ts'))（lifecycleGatingModule 3 处 + fix2Gating 3
处），而本文件对其他依赖（runtime/career/platform）都是文件顶层一次性 await
import。建议在顶层导入一次；若仍需「模块缺失」的友好报错，pageGatingFromServiceState/pageDeletionUnknown 里已有的判空 throw 已能覆盖。

-   Object.assign(lifecycleGatingModule, await import('../src/career/export-deletion.gating.ts'));
+ // 文件顶部一次性导入：
+ // const gating = await import('../src/career/export-deletion.gating.ts');
+ // 各测试直接解构：const { lifecycleGating } = gating;


─── apps/miniprogram/tests/live/quota-inject-proxy.mjs:29-33 ───
[bug · medium] 转发链路的错误处理不完整，两个真实场景会把整个代理进程打崩，导致 DevTools 复验会话全部断网且报错难以定位：(1) 上游 Lite 服务器在响应体传输中途断开（如
SSE/长响应 socket reset）时，错误事件发生在响应流 up 上而非 upstream 请求对象上——up 没有任何 'error' 监听，未处理的 stream error
会以未捕获异常终止代理进程；(2) 若 upstream 的 'error' 在响应头已转发之后触发（headersSent 为真），再次 res.writeHead(502) 会抛
ERR_HTTP_HEADERS_SENT。建议：给 up 补 error 监听，且在 headersSent 后改用 res.destroy() 而不是 writeHead。

    const upstream = http.request({ host: '127.0.0.1', port: BACK, method: req.method, path: req.url, headers: { ...req.headers, host: `127.0.0.1:${BACK}` } }, up => {
      res.writeHead(up.statusCode ?? 502, up.headers);
+     // 上游中途断流：响应头已转发，不能再 writeHead，直接销毁下游连接
+     up.on('error', err => { console.error('[quota-proxy] upstream stream error', err.message); res.destroy(); });
      up.pipe(res);
    });
-   upstream.on('error', err => { console.error('[quota-proxy] upstream error', err.message); res.writeHead(502); res.end('quota-proxy upstream error'); });
+   upstream.on('error', err => {
+     console.error('[quota-proxy] upstream error', err.message);
+     if (res.headersSent) { res.destroy(); return; }
+     res.writeHead(502); res.end('quota-proxy upstream error');
+   });


─── apps/miniprogram/tests/live/quota-inject-proxy.mjs:21-21 ───
[bug · low] 注释声明"只在冻结合同边界 POST /api/v1/career/searches 注入拒绝，其余请求原样转发"，但 startsWith
匹配比该边界更宽：会连带拒绝任何以该前缀开头的 POST 路径（如未来出现的 POST /api/v1/career/searches/:id/cancel、或恰以 searches
开头的其他端点），且 req.url 还包含 query 串。当前后端（routes_career.go）恰好只有 POST /searches 一个该前缀的 POST
端点所以暂未出错，但注入面与冻结合同不再逐端点一致，后续新增子路由会被静默拒绝、复验结果失真。建议剥掉 query 后做全等匹配。

-   const refused = fs.existsSync(FLAG) && req.method === 'POST' && req.url.startsWith('/api/v1/career/searches');
+   const pathOnly = (req.url ?? '').split('?')[0];
+   const refused = fs.existsSync(FLAG) && req.method === 'POST' && pathOnly === '/api/v1/career/searches';


─── apps/miniprogram/tests/live/t24r1-live-driver.cjs:28-31 ───
[bug · low] 两处初始化缺少防护：(1) fs.readFileSync(process.env.T24R1_TOKA, …) 在 SHOTS/USER_A/TENANT
校验之前执行，TOKA 未设置时抛出的 TypeError（"The \"path\" argument must be of type string"）会掩盖 'missing T24R1_*
env' 的明确提示；(2) li[0]/li[1]/box 均无空值检查，页面结构变化或加载未就绪时以 "Cannot read properties of undefined"
崩溃，取证失败时难以定位是哪一步、哪个选择器落空。实机复验驱动的价值在于失败时可诊断，建议先校验全部 env，再对元素做存在性断言后使用。

- const TA = fs.readFileSync(process.env.T24R1_TOKA, 'utf8').trim();
- const USER_A = process.env.T24R1_USER_A;
- const TENANT = String(process.env.T24R1_TENANT);
- if (!SHOTS || !USER_A || !TENANT) { console.error('missing T24R1_* env'); process.exit(2); }
+ const TA_PATH = process.env.T24R1_TOKA;
+ if (!TA_PATH || !SHOTS || !USER_A || !TENANT) { console.error('missing T24R1_* env'); process.exit(2); }
+ const TA = fs.readFileSync(TA_PATH, 'utf8').trim();
+ // …
+ const li = await page.$$('.wk-input');
+ if (li.length < 2) throw new Error(`login page exposes ${li.length} .wk-input, expected 2`);
+ const box = (await page.$('.wk-consent checkbox')) || (await page.$('checkbox'));
+ if (!box) throw new Error('consent checkbox not found on login page');


─── apps/miniprogram/tests/progress-preparation.test.mjs:292-295 ───
[test · low] 时序性断言：用 setTimeout 30ms 的静默等待证明"重连后不自动提交"只能证伪不能证明 absence——若实现引入大于 30ms 的延迟自动重试（如 50ms
退避定时器），断言将假绿。建议改为确定性手段：翻转 materialUp 后仅排空微任务/立即回调队列（stub 的异步动作均走
queueMicrotask），或断言实现不在失败路径注册任何定时器，以显式 retryPendingMaterial 为唯一触发点。

    materialUp = true;
-   await new Promise(resolve => setTimeout(resolve, 30));
+   // 排空微任务队列即可覆盖 stub 的全部异步动作，不依赖毫秒时序：任何延迟重试都会使计数在显式同步前超过 1。
+   await new Promise(resolve => setImmediate(resolve));
+   await new Promise(resolve => setImmediate(resolve));
    const materialPosts = stub.state.calls.filter(c => new URL(c.options.url).pathname === '/api/v1/career/materials');
    assert.equal(materialPosts.length, 1, 'only the failed attempt — nothing auto-submits after the network returns');


─── apps/miniprogram/tests/progress-preparation.test.mjs:425-430 ───
[maintainability · low] 双源漂移：注释明言"逐句复刻页面 saveRevision 逻辑"——editable 映射、空 claims 判断与
recoverEmptyClaims 的组合链在 progress-preparation.tsx:136 附近存在另一份实现。页面保存链后续改动（例如改判断条件或调整 material
读取）时，本测试将继续通过而不再覆盖真实行为。建议把这条组合链抽为共享纯函数（如 career-platform.ts 或页面模块导出的
composePreparationSaveBody(draft, serverBody)），页面与测试同源调用，recoverEmptyClaims/attachClaimsFromServer
单测保留。

-   // 页面 saveRevision 的同链组合（t-button 不可单测，逐句复刻页面逻辑）：
-   const editable = draft.sections.map(section => ({ heading: section.heading, content: section.content, claims: Array.isArray(section.claims) ? section.claims : [] }));
-   let body = career.bodyFromEditable(editable);
-   if (body.sections.some(section => (section.claims ?? []).length === 0)) {
-     body = { sections: platform.recoverEmptyClaims(body.sections, (await career.material('mat-9')).body.sections) };
-   }
+ // career-platform.ts 增加：export function composePreparationSaveBody(draftSections, serverSections) { /* editable 映射 + 空 claims 恢复的唯一实现 */ }
+ // 页面与测试同源调用：
+ const body = platform.composePreparationSaveBody(draft.sections, (await career.material('mat-9')).body.sections);


─── apps/miniprogram/tests/progress-preparation.test.mjs:243-243 ───
[test · low] 脆弱断言：过滤条件未限定 method。同文件 D2b 与 R1-P4 对同一端点的断言均加了 (c.options.method ?? 'GET') ===
'POST'，此处 [0] 在实现将来于 editMaterial 之前增加同路径的 GET（材料列表读取）时会取错调用对象且 options.data 为
undefined，报错点远离真实原因。建议补上 method 过滤保持一致。

-   const body = stub.state.calls.filter(c => new URL(c.options.url).pathname === '/api/v1/career/materials')[0].options.data;
+   const body = stub.state.calls.filter(c => new URL(c.options.url).pathname === '/api/v1/career/materials' && (c.options.method ?? 'GET') === 'POST')[0].options.data;


─── apps/miniprogram/tests/rules-usage-reminders.test.mjs:65-77 ───
[maintainability · medium] 重复样板：backend()/freshLogin()/careerCall()/errorCode() 以及顶部 registerHooks
替换块在六个新增测试文件（application-material / career-discovery / export-deletion / progress-preparation /
rules-usage-reminders 及本文件）中近乎逐字重复（`function backend(routes)` 共 6 处）。冻结合同一旦调整（如错误 envelope
结构、登录路由、career 前缀）需同步修改六处，漂移与遗漏成本高。建议抽取到 tests/helpers/career-testkit.mjs 共享（如 export function
mountTaroStub() / createBackend() / freshLogin(me, extraRoutes)），各文件仅保留 fixture 与断言。

- function backend(routes) {
-   stub.use(call => {
-     const method = call.options.method ?? (call.kind === 'uploadFile' ? 'POST' : 'GET');
-     const path = new URL(call.options.url).pathname;
-     let fn = routes[`${method} ${path}`];
-     if (fn === undefined) {
-       const prefix = Object.keys(routes).filter(k => k.endsWith('/') && `${method} ${path}`.startsWith(k)).sort((a, b) => b.length - a.length)[0];
-       if (prefix) fn = routes[prefix];
-     }
-     if (fn === undefined) { call.options.fail({ errMsg: `no backend route for ${method} ${path}` }); return; }
-     fn(call);
-   });
- }
+ // tests/helpers/career-testkit.mjs
+ export function careerBackend(stub, routes) { /* 统一的 method+pathname 路由分发 */ }
+ export async function careerFreshLogin(stub, runtime, career, { me, extraRoutes }) { /* 统一的登录 + desk 重置 */ }
+ // 各测试文件：
+ import { careerBackend, careerFreshLogin } from './helpers/career-testkit.mjs';


─── apps/miniprogram/tests/rules-usage-reminders.test.mjs:122-122 ───
[test · low] 脆弱替换：stub.use 是整体替换 state.handler（helpers/taro-stub.mjs:174 `use(fn){ state.handler =
fn; }`），此处把 backend() 安装的整张路由表丢弃，换成只认一个路径的 handler——替换后任何其他请求（readRule 实现未来先读 desk、token
刷新、或新增的任何调用）都不会被响应，测试将以挂起超时而非明确的断言失败收场。建议改用可变的 fixture 引用让 backend 路由继续生效，模拟 Web 端编辑只需重赋值该引用。

-   stub.use(call => { if (new URL(call.options.url).pathname === '/api/v1/career/rules/rule-1') stub.succeed(call, { data: afterWebEdit }); });
+   // freshLogin 时改为可变 fixture：
+   //   let ruleFixture = ruleView();
+   //   'GET /api/v1/career/rules/rule-1': call => stub.succeed(call, { data: ruleFixture }),
+   // 模拟 Web 端编辑时只需：
+   ruleFixture = afterWebEdit;


─── apps/mobile/src/task-office-integration-smoke.ts:139-139 ───
[test · medium] 探针从未触达服务端:mobile-core 的 createTaskOffice 在 tasks() 入口先调用
requireLease()(task-office.ts:209),而 runtime.scopeLease() 在 signIn 之前恒为 undefined,所以
probeUnauthenticatedRead 的 office.tasks({}) 会在发起任何网络请求之前同步抛出 TASK_OFFICE_SCOPE_CHANGED——这里精心装配的
createTaskOfficeRemote/authorizedRequest 传输层是死代码,永远不会被调用。结果是 evidence.unauthenticatedRead ===
'rejected' 是恒真的同义反复(只要 mobile-core 保留客户端门禁就必过),无法发现服务端鉴权回归(如某次改动让 executions 列表路由漏掉 auth 中间件),而注释却宣称
"Prove the exact production Task Office boundary fails closed"、"Reuse the same deployment origin and
transport"。建议二选一:(1) 直接用无凭证的 fetch 请求 executions 列表端点并断言 401/403,真正验证服务端边界;(2)
若只打算验证客户端门禁,请修正注释与证据语义(如 'client-gate-rejected'),避免证据被过度解读。



─── apps/mobile/src/task-office-integration-smoke.ts:58-60 ───
[bug · medium] 归档可见性判断只看第一页,会产生假 'failed' 证据:normalizeQuery 默认 limit=20(task-office.ts:125),而服务端
ListOwnedExecutions 按 created_at DESC 稳定排序(workbench_list.go:206-210)。archived_at 的归档动作不改变
created_at,因此 office.tasks({ archived: true }) 的第一页是"创建时间最新的 20 条已归档 run"——若测试账号已有 ≥20
条比目标(最旧活跃任务被选中时)更晚创建的已归档任务,刚归档的目标会落在第 2 页,archivedVisible 误判为 false,一次成功的归档往返被记为 archiveRoundtrip:
'failed'。restoredPage 检查形态相同,但因目标本就是最新创建的活跃任务,实际风险低。建议用 moreTasks() 沿 nextCursor 翻页直到找到或游标耗尽,并同时传
limit: 100 缩小窗口:

+   const visibleSomewhere = async (query: { archived?: boolean }): Promise<boolean | 'failed'> => {
-   try {
+     try {
-     const archivedPage = await office.tasks({ archived: true });
-     archivedVisible = archivedPage.items.some((card) => card.taskId === taskId);
+       let page = await office.tasks({ ...(query.archived === true ? { archived: true } : {}), limit: 100 });
+       for (;;) {
+         if (page.items.some((card) => card.taskId === taskId)) return true;
+         if (page.nextCursor === undefined) return false;
+         page = await office.moreTasks();
+       }
+     } catch {
+       return 'failed';
+     }
+   };


─── apps/mobile/src/task-office-integration-smoke.ts:196-200 ───
[test · low] 中段读取失败(如 search nonce 或 activePage 的 tasks({}) 抛错)会落入这个外层 catch,此时 archiveRoundtrip
保持初始值 'unavailable'、archiveRestore 保持 'not-attempted'。但 'unavailable'
的既定语义是"账号确无任务、未执行归档"(见上方注释),失败与未执行在证据里不可区分,排障时会误导。建议在 catch 中把 archiveRoundtrip 显式置为 'failed'(或引入独立的
'indeterminate' 值),保留 'unavailable' 专用于"确无任务"。



─── apps/mobile/src/task-office-integration-smoke.ts:196-200 ───
[other · low] 外层 catch 中的三元赋值是恒等无效语句（no-op）：此处 evidence.home 的值域只有初始的 'failed' 和成功路径的
'loaded'，'loaded' ? 'loaded' : 'failed' 不会改变任何值。如果本意是「中段失败时标记 home」，这个写法没有达到效果（与先前确认的
archiveRoundtrip 语义问题同源但独立）；如果本意是「保留已得信息」，则应删除该语句，home 在成功路径已如实记录为 'loaded'。建议删除以消除误导。

    } catch {
-     evidence.home = evidence.home === 'loaded' ? evidence.home : 'failed';
+     // home 已在成功路径如实记录为 'loaded'，失败路径保持初始 'failed'；
+     // 原三元对 'loaded' | 'failed' 值域恒等，删除以免误导。
    } finally {
      runtime.dispose();
    }


─── apps/web/src/DevMarkdownPage.tsx:259-262 ───
[maintainability · low] 对全部渲染结果做无差别的 `<em><strong>X</strong></em>` → `<em><em></em>X</em>**`
改写，正则无法区分该结构是否真源自 `***三重强调***`：任何未来 fixture 中合法的 em>strong 嵌套（如 `*__x__*`）都会被误改写为泄漏 `**`
字面量的残缺输出，且该行为与 fixture 名无关联、难以排查。建议以源串包含 `***` 作为门控，把改写限定在确实会产生 Vue repairFlankingEmphasis 副作用的输入上。

-   return renderChatMarkdown(markdown).replace(
-     /<em><strong>([^<]*)<\/strong><\/em>/g,
-     '<em><em></em>$1</em>**',
-   );
+   const html = renderChatMarkdown(markdown);
+   // 仅当源串含三重星号（Vue repairFlankingEmphasis 的触发形态）时执行等价
+   // 改写，避免误伤未来 fixture 中合法的 em>strong 嵌套。
+   return markdown.includes('***')
+     ? html.replace(/<em><strong>([^<]*)<\/strong><\/em>/g, '<em><em></em>$1</em>**')
+     : html;


─── apps/web/src/agent-marketplace/am-u.css:126-128 ───
[maintainability · medium] `.wk-amr-18`（及同文件 `.wk-ava-11`）将 Tailwind 语义令牌类 `bg-accent` 的值固化为字面量
`#07c05f`。`bg-accent` 实际经 `var(--color-accent)`（apps/web/src/styles.css L457 事实源）生效；design-tokens
已引入 `:root[theme-mode="dark"]` 暗色机制（`--wk-color-brand: #06b04d` 等），apps/web 的令牌别名链也在向 `--wk-*`
过渡。硬编码使这两处主按钮脱离令牌体系：后续令牌换值或接入暗色主题时颜色不再跟随，且需多点同步修改。建议按迁移原则（"值 = utilities 编码的生效值"）改用变量引用。

  .wk-amr-18 {
    border-radius: 4px;
-   background-color: #07c05f;
+   background-color: var(--color-accent);


─── apps/web/src/agent-marketplace/am-u.css:50-55 ───
[maintainability · medium] `.wk-amr-7` 将 `bg-surface` 固化为 `#ffffff`。`bg-surface` 经
`var(--color-surface)`（styles.css L424）生效；design-tokens 暗色块已将 `--wk-color-surface` 覆盖为
`#242424`，卡片背景是最典型的主题敏感面。硬编码后该卡片背景与令牌体系解耦，主题/令牌调整无法传导到此组件。建议改用变量引用保持单一事实源。

    border-color: #e7e7ea;
-   background-color: #ffffff;
+   background-color: var(--color-surface);
    padding: 16px;
  }
  
  .wk-amr-8 {


─── apps/web/src/agent-marketplace/am-u.css:97-98 ───
[maintainability · low] `.wk-amr-14`（data-review-digest）用 `overflow-wrap: anywhere` 替代原 Tailwind
`break-all`（生效值应为 `word-break: break-all`）。两者断行语义不等价：`break-all` 允许在任意字符间换行，`anywhere`
仅在整词放不下当前行时才断行，且对 min-content 内在尺寸的计算规则也不同。对超长十六进制 digest 实际渲染差异可忽略，但与文件头声明的"值 = utilities
编码的生效值"迁移原则不符，建议逐值对齐。

  .wk-amr-14 {
-   overflow-wrap: anywhere;
+   word-break: break-all;


─── apps/web/src/agent-marketplace/am-u.css:229-233 ───
[maintainability · low] 同一文件内存在逐字相同的重复声明：`.wk-ava-12` 与 `.wk-amr-4`（margin:0; font-size:13px;
color:#d54941）完全一致，`.wk-ava-13` 与 `.wk-amr-12`（margin:0; font-size:13px）亦然。当前是两份独立维护的拷贝，后续修改（如错误色改用
`var(--color-error)` 令牌）容易只改一处导致两个组件表现漂移。可用选择器分组合并，TSX 中的类名无需改动。

+ /* 与 .wk-amr-4 合并，避免同文件重复维护 */
+ .wk-amr-4,
  .wk-ava-12 {
    margin: 0;
    font-size: 13px;
    color: #d54941;
  }


─── apps/web/src/career/ApplicationPage.tsx:25-28 ───
[maintainability · medium] 同目录的 protocol.ts（本次同批新增）已导出
newRequestId/errorDetails/isUncertainWrite，OpportunityPage.tsx 与 reconciliation.tsx 均已引用；本页与
ExportDeletionPage/InboxPage/MaterialPage 却各自整段复制，且 isUncertainWrite 的确定错误码白名单四份互有差异（本页额外含
application_conflict、hard_ineligible_requires_continue 等）。后续在 protocol.ts
基础列表上新增/修正错误码时极易漏改某一份拷贝，导致同一错误在不同页面走相反的恢复语义（unknown 恢复 vs 确定失败）。建议从 './protocol.ts'
导入基础实现，页面特有错误码在调用处叠加判断。

- function errorDetails(cause: unknown): { code?: string; requestId?: string; currentRevision?: number; status?: number; message: string } {
-  const error = cause as { code?: string; requestId?: string; currentRevision?: number; status?: number; message?: string }
-  return { code: error?.code, requestId: error?.requestId, currentRevision: error?.currentRevision, status: error?.status, message: error?.message || '请求未完成' }
- }
+ import { errorDetails, isUncertainWrite, newRequestId } from './protocol.ts'
+ // 页面特有错误码在调用处叠加：
+ // if (['application_conflict', 'hard_ineligible_requires_continue'].includes(parsed.code ?? '')) { /* definite */ }


─── apps/web/src/career/ApplicationPage.tsx:192-192 ───
[style · low] 此处为嵌套三元表达式（idempotency_conflict → not_found → 默认），违反「禁止嵌套三元」规则；本页
evaluationStatusLabel、revisionState 文案链与 JSX 中的 phase
条件链同属此类。多分支文案建议改为查表函数或提前计算的局部变量，避免后续扩展错误码时继续加深嵌套。

-     setMessage(parsed.code === 'idempotency_conflict' ? '请求编号已对应其他内容，服务器拒绝了本次提交。请检查后重新提交。' : parsed.code === 'not_found' ? '所选评估或岗位快照不存在（可能不属于当前空间）。请重新评估后再申请。' : `申请未被接受：${parsed.message}`)
+ const submitRejectMessages: Record<string, string> = {
+  idempotency_conflict: '请求编号已对应其他内容，服务器拒绝了本次提交。请检查后重新提交。',
+  not_found: '所选评估或岗位快照不存在（可能不属于当前空间）。请重新评估后再申请。',
+ }
+ setMessage(submitRejectMessages[parsed.code ?? ''] ?? `申请未被接受：${parsed.message}`)


─── apps/web/src/career/CareerPage.tsx:176-176 ───
[maintainability · low] 本页几乎全部布局走静态内联 style（maxWidth、margin、grid、gap、边框色等），违反「避免内联
style」规范，且与同目录其余四个页面（application/material/inbox/export-deletion 均有独立
CSS）的做法不一致——内联样式无法随主题令牌统一调整，也使本页游离于 tdesign-theme.css 之外（如边框色 #e7e7e7、文字色 #666 为硬编码近似值）。建议迁移到独立 CSS
文件并改用 --td-* 令牌。

-  return <main className="wk-page wk-page--std" style={{ maxWidth: 1040, margin: '0 auto', padding: '24px 20px' }}>
+  return <main className="wk-page wk-page--std wk-career">
+ /* career.css */
+ .wk-career { max-width: 1040px; margin: 0 auto; padding: 24px 20px; }


─── apps/web/src/career/CareerPage.tsx:5-5 ───
[maintainability · low] career-core 本次已新增 package.json 并定义 exports（"./desk":
"./src/desk.ts"、"./contracts": "./src/contracts.ts"），本页对 CareerDesk
的运行时导入仍用四层相对路径跨包穿透源码，包结构（目录重命名、加构建产物）一旦调整即编译失败。建议改用包名子路径导入（页内其余 type-only 导入同理，风险较低可随后跟进）。

- import { CareerDesk } from '../../../../packages/career-core/src/desk.ts'
+ import { CareerDesk } from '@weknora/career-core/desk'


─── apps/web/src/career/CareerPage.tsx:64-65 ───
[maintainability · low] load 的 useCallback 依赖数组缺少其闭包内引用的
invalidateForbidden（refreshSources、refreshSourceList、sendUpload、doAction
等同样闭包引用但未声明）。invalidateForbidden 未做 useCallback 包装、每次渲染重建，当前因内部仅引用稳定的 desk 与 setter 而无实际 stale
值，但这属于 exhaustive-deps 违规：后续任何人往 invalidateForbidden 里加入对 state/props 的引用，这些回调就会静默持有过期闭包（例如错误地清掉或漏清新
scope 的状态）。建议用 useCallback 包装 invalidateForbidden 并补全各回调依赖。



─── apps/web/src/career/ExportDeletionPage.tsx:28-31 ───
[maintainability · medium] 同目录 protocol.ts（本次同批新增）已导出
newRequestId/errorDetails/isUncertainWrite，OpportunityPage.tsx 与 reconciliation.tsx
已引用；本页仍整段复制三份助手，且与 ApplicationPage/MaterialPage/InboxPage 的 isUncertainWrite 白名单互有差异。后续在 protocol.ts
基础列表上调整错误码分类时极易漏改本拷贝，使同一错误在不同页面走向相反的恢复分支（unknown 恢复 vs 确定失败），破坏「未知写入用原请求编号恢复」的一致语义。

- function errorDetails(cause: unknown): { code?: string; requestId?: string; currentRevision?: number; status?: number; message: string } {
-  const error = cause as { code?: string; requestId?: string; currentRevision?: number; status?: number; message?: string }
-  return { code: error?.code, requestId: error?.requestId, currentRevision: error?.currentRevision, status: error?.status, message: error?.message || '请求未完成' }
- }
+ import { errorDetails, isUncertainWrite, newRequestId } from './protocol.ts'


─── apps/web/src/career/ExportDeletionPage.tsx:193-196 ───
[bug · medium] downloadExport 用未插入文档的 anchor 直接调用 click()，并在 click() 后立即
URL.revokeObjectURL(url)。Firefox 对未挂载到 DOM 的节点触发 click() 不会执行下载，Safari 对立即 revoke 的 blob URL
可能中止尚未开始的下载，导出包在这类浏览器里点「下载导出包」无任何反应。同批新增的 MaterialPage.saveBlob 已实现正确模式（document.body.append → click
→ remove → setTimeout 延迟 revoke），建议两处复用同一实现，保持行为一致。

    const anchor = document.createElement('a')
-   anchor.href = url; anchor.download = `career-export-${exported.exportId}.json`
+   anchor.href = url
+   anchor.download = `career-export-${exported.exportId}.json`
+   document.body.append(anchor)
    anchor.click()
-   URL.revokeObjectURL(url)
+   anchor.remove()
+   setTimeout(() => URL.revokeObjectURL(url), 5_000)


─── apps/web/src/career/InboxPage.tsx:127-131 ───
[performance · medium] 待办定位权威申请时在 for 循环内逐条 await：N 个不同申请即 N
次串行往返，且位于收件箱首屏加载路径上，待办数量多时明显拖慢。各申请回执彼此独立（已按 applicationId 去重），应改为对去重后的编号列表用 Promise.all
并行获取，完成后统一做一次 scopeController.isCurrent 检查。

-     for (const item of next.reminders) {
-      const applicationId = item.applicationId
-      if (!applicationId || applicationId in refs) continue
+     const ids = [...new Set(next.reminders.flatMap((item) => (item.applicationId ? [item.applicationId] : [])))]
+     await Promise.all(ids.map(async (applicationId) => {
       try {
        const receipt = await client.career.application(applicationId, requestScope.signal)
+       refs[applicationId] = { snapshotId: receipt.pinnedEvidence.snapshotId, opportunityId: receipt.pinnedEvidence.opportunityId }
+      } catch { refs[applicationId] = 'failed' }
+     }))
+     if (!scopeController.isCurrent(requestScope.scope)) return


─── apps/web/src/career/InboxPage.tsx:28-31 ───
[maintainability · medium] 同目录 protocol.ts（本次同批新增）已导出
newRequestId/errorDetails/isUncertainWrite，OpportunityPage.tsx 与 reconciliation.tsx
已引用；本页仍整段复制三份助手，与其他三页的白名单互有差异。后续在 protocol.ts 基础列表上调整错误码分类时极易漏改本拷贝，使同一错误在不同页面走向相反的恢复分支（unknown 恢复 vs
确定失败），破坏「未知写入用原请求编号恢复」的一致语义。

- function errorDetails(cause: unknown): { code?: string; requestId?: string; currentRevision?: number; status?: number; message: string } {
-  const error = cause as { code?: string; requestId?: string; currentRevision?: number; status?: number; message?: string }
-  return { code: error?.code, requestId: error?.requestId, currentRevision: error?.currentRevision, status: error?.status, message: error?.message || '请求未完成' }
- }
+ import { errorDetails, isUncertainWrite, newRequestId } from './protocol.ts'


─── apps/web/src/career/InboxPage.tsx:189-189 ───
[bug · low] 订阅推送的 act 请求里 expectedRevision 用 view?.revision ?? 0 兜底：0 是伪造的修订号，一旦可达（恢复按钮组的
runWrite(attempt) 没有任何 view 守卫，且该按钮组在 readState 为 loading/error 时也渲染），会以错误修订提交，产生无意义的
revision_conflict 或误导性的幂等行为。另外 reminder 分支的 expectedRevision 冻结在 attempt.input 中重试，而 subscription
分支每次动态读取 view.revision，同编号重试的语义不一致。建议与 reminder 分支对齐：创建 attempt 时冻结 revision，执行前缺修订则显式报错。

-     const action: CareerAction = { action: 'confirm', key: PUSH_FACT_KEY, value: current.value, source: { kind: 'user', label: '本人确认' }, requestId: current.requestId, expectedRevision: view?.revision ?? 0 }
+     const revision = view?.revision
+     if (revision === undefined) { setWritePhase('error'); setMessage('档案修订尚未读取，请刷新后重试。'); return }
+     const action: CareerAction = { action: 'confirm', key: PUSH_FACT_KEY, value: current.value, source: { kind: 'user', label: '本人确认' }, requestId: current.requestId, expectedRevision: revision }


─── apps/web/src/career/MaterialPage.tsx:20-23 ───
[maintainability · medium] 同目录 protocol.ts（本次同批新增）已导出
newRequestId/errorDetails/isUncertainWrite，OpportunityPage.tsx 与 reconciliation.tsx
已引用；本页仍整段复制三份助手，且 isUncertainWrite 白名单额外多出 material_claim_unconfirmed，与其他页互有差异。后续在 protocol.ts
基础列表上调整错误码分类时极易漏改本拷贝，使同一错误在不同页面走向相反的恢复分支。

- function errorDetails(cause: unknown): { code?: string; requestId?: string; currentRevision?: number; status?: number; message: string } {
-  const error = cause as { code?: string; requestId?: string; currentRevision?: number; status?: number; message?: string }
-  return { code: error?.code, requestId: error?.requestId, currentRevision: error?.currentRevision, status: error?.status, message: error?.message || '请求未完成' }
- }
+ import { errorDetails, isUncertainWrite, newRequestId } from './protocol.ts'
+ // material_claim_unconfirmed 在本页错误处理分支中单独判断


─── apps/web/src/career/MaterialPage.tsx:211-211 ───
[bug · medium] 本文件 6
处（saveDraft:211、confirmDraft:271、lookupReceipt:320、publishExport:398、revokeExport:443、downloadExport
:479）用 TypeError 表示「回执与请求不匹配」这一确定性协议错误；但网络失败的 fetch 恰恰抛出无 code/status 的 TypeError，会被
isUncertainWrite 判为 true 而落入 unknown「用原请求编号恢复」分支。同组
ApplicationPage/ExportDeletionPage/InboxPage/OpportunityPage 均显式定义 ReceiptMismatchError 并在
isUncertainWrite 之前用 instanceof 区分（其顶部注释也明确了这一区分理由）。结果是回执确不匹配时用户被引导用原编号重试/查回执，该恢复永远失败且 UI 永久停留在
unknown，给出误导性指引。

-    if (receipt.requestId !== currentAttempt.requestId || (currentAttempt.input.materialId !== undefined && receipt.materialId !== currentAttempt.input.materialId)) throw new TypeError('材料回执与本次请求不匹配')
+ class ReceiptMismatchError extends Error {}
+ // ...
+ throw new ReceiptMismatchError('材料回执与本次请求不匹配')
+ // 各 catch 中置于 isUncertainWrite 判断之前：
+ if (cause instanceof ReceiptMismatchError) { setAttempt(undefined); setPhase('error'); setMessage(`材料未完成：${cause.message}`); return }


─── apps/web/src/career/MaterialPage.tsx:16-18 ───
[documentation · medium] 注释声称镜像后端 MaxExportGrantTTL 且「授权最多存活 15 分钟」，后端
internal/modules/career/rendering.go 实际为 15 * time.Minute（900 秒），而常量为 600（10 分钟）：注释与数值矛盾。600
不会被后端拒绝（后端仅校验 ttl <= 0 || ttl > MaxExportGrantTTL），但授权时长被静默缩短三分之一，长下载需提前二次签发。应统一为 900，或更正注释为 10
分钟并说明偏离原因。

  // Mirrors MaxExportGrantTTL in internal/modules/career/rendering.go: a
  // career export download grant lives at most 15 minutes.
- const EXPORT_GRANT_TTL_SECONDS = 600
+ const EXPORT_GRANT_TTL_SECONDS = 900


─── apps/web/src/career/MaterialPage.tsx:378-379 ───
[bug · medium] 该守卫把「控制器当前 scope 与自身比较」：scopeController.current() 永远返回最新
handle，isCurrent(current.scope) 恒为 true，条件永假，属于死守卫。导出列表请求因空间切换被中止/失败时，此 catch 仍会执行 clearPrivate（内部还会
history.replaceState 清空 material 参数）或 setExportMessage，污染新空间的页面状态与 URL，违背「空间切换后旧响应失效」红线的前端侧防护。应先在
effect 内捕获 requestScope，再判断 isCurrent(requestScope.scope)（与 reloadExports 内部做法一致）。

+   const requestScope = scopeController.current()
    void reloadExports(materialId).catch((cause: unknown) => {
-    if (!scopeController.isCurrent(scopeController.current().scope)) return
+    if (!scopeController.isCurrent(requestScope.scope)) return


─── apps/web/src/career/MaterialPage.tsx:411-413 ───
[bug · low] publishExport（此处）与 revokeExport（451-454 行）的 revision_conflict 分支误调
setMessage：冲突文案落入材料主消息区且按 phase（此时仍为 idle）以非错误样式渲染，而导出区的 exportMessage
仍停留在旧的「正在渲染并核验导出…」/「正在撤销导出…」并以 error 样式渲染——真实失败原因展示错位，用户在导出区看到与事实不符的提示。应改为 setExportMessage。

     if (parsed.code === 'revision_conflict') {
      setExportAttempt(undefined); setExportPhase('error'); setExportConflict(parsed.currentRevision)
-     setMessage(`档案已更新${parsed.currentRevision !== undefined ? `（当前修订 ${parsed.currentRevision}）` : ''}。请重新读取档案修订后再次发布导出；新提交会使用新的请求编号。`)
+     setExportMessage(`档案已更新${parsed.currentRevision !== undefined ? `（当前修订 ${parsed.currentRevision}）` : ''}。请重新读取档案修订后再次发布导出；新提交会使用新的请求编号。`)


─── apps/web/src/career/MaterialPage.tsx:520-520 ───
[performance · low] 章节列表支持「移除章节」删除中间项却以数组下标作 key：React 按索引复用
DOM，删除中间章节后，后续章节的受控值虽会跟随数据重排，但输入框焦点、光标位置等 DOM 状态会错位到相邻章节（claims 已正确用 claimId 作 key，章节缺稳定
id）。建议新增章节时生成本地稳定 id。另外 `const dirty = ...` 在每次渲染对全部正文执行两次 JSON.stringify 全量序列化，大材料下每次击键都重复序列化，建议
useMemo 缓存。

-     {sections.map((section, sectionIndex) => <fieldset className="wk-material__section" key={sectionIndex}>
+ // 新增章节时生成稳定 id：{ sectionId: makeId(), heading: '', content: '', claims: [] }
+ {sections.map((section) => <fieldset className="wk-material__section" key={section.sectionId}>
+ // dirty 缓存：
+ const dirty = useMemo(() => savedBody === undefined || JSON.stringify(bodyFromEditable(sections)) !== JSON.stringify(savedBody), [savedBody, sections])


─── apps/web/src/career/MaterialPage.tsx:568-571 ───
[style · low] statusText 为三层嵌套三元（receipt.kind → status → deliverable），且外层还有 success（400-404
行）两层嵌套三元，违反「禁止嵌套三元」规则。枚举状态到文案是典型的查表场景，建议用提前计算的常量表或函数，避免后续新增导出状态时继续加深嵌套。

- const statusText = receipt.kind === 'material_export_revoked' ? '已撤销（revoked）：旧下载授权立即失效'
-       : receipt.status === 'submittable' ? (deliverable ? '双格式核验通过（submittable）' : '双格式核验记录不一致，暂不提供投递')
-       : receipt.status === 'staged' ? '已暂存（staged）：仅一种格式核验通过，不可投递'
-       : '双格式核验失败（failed），不可投递'
+ const exportStatusText = (receipt: MaterialExportReceipt, deliverable: boolean): string => {
+  if (receipt.kind === 'material_export_revoked') return '已撤销：旧下载授权立即失效'
+  if (receipt.status === 'submittable') return deliverable ? '双格式核验通过' : '双格式核验记录不一致，暂不提供投递'
+  if (receipt.status === 'staged') return '已暂存：仅一种格式核验通过，不可投递'
+  return '双格式核验失败，不可投递'
+ }


─── apps/web/src/career/MaterialPage.tsx:182-185 ───
[bug · medium] URL 恢复材料的 catch 不区分错误类型：任何失败（含瞬时网络错误、5xx、超时）都走 clearPrivate，把 phase 置为
forbidden——此时整个编辑区被隐藏（forbidden/scope-changed 分支只渲染一条消息），且 clearPrivate 内部 history.replaceState 清掉了
?material= 参数，用户刷新也无法再定位该材料。对照 ApplicationPage 的同型恢复：失败时仅 setPhase('error')、保留 URL
参数并提示「可刷新重试」。两页行为应一致，至少 forbidden 才清空，其余错误保留指针可重试。

-   }).catch(() => {
+   }).catch((cause: unknown) => {
     if (!scopeController.isCurrent(requestScope.scope)) return
-    clearPrivate('当前空间不可访问此材料，已清除编辑内容。')
+    const parsed = errorDetails(cause)
+    if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问此材料，已清除编辑内容。'); return }
+    setPhase('error'); setMessage('材料暂时无法读取。可刷新重试；URL 中的材料编号已保留。')
    })


─── apps/web/src/career/MaterialPage.tsx:212-212 ───
[bug · medium] saveDraft 成功后只 setSavedBody(receipt.body) 而没有像 acceptView 那样同步
setSections(editableFromBody(receipt.body))（lookupReceipt 成功路径是同一句语句，问题相同）。dirty 的判定是
bodyFromEditable(sections) 与 savedBody 的 JSON 全等比较，因此代码隐式依赖「服务器逐字节原样回显提交的 body」：一旦服务端对 body
做任何规范化（trim heading、键序、字段增删），保存后 sections 与 savedBody 立即不一致，dirty 恒为 true →
confirmBlocked，「确认发布不可变版本」永久禁用；且再次保存也不会消除（再次提交的仍是本地未规范化 sections，回执又替换
savedBody），用户陷入无法发布的死锁，只能整页刷新走 acceptView。建议成功路径与 acceptView 保持同一套状态同步。

-    setMaterialId(receipt.materialId); setSavedBody(receipt.body); syncClaimCounter(receipt.body)
+    setMaterialId(receipt.materialId); setSavedBody(receipt.body); setSections(editableFromBody(receipt.body)); syncClaimCounter(receipt.body)


─── apps/web/src/career/OpportunityPage.tsx:212-212 ───
[bug · low] 评估回执错配被误分类为 unknown，违背本文件自身声明的契约。runEvaluation/lookupEvaluationReceipt（以及下方
EvaluationAction.accept）在回执不匹配时抛的是普通 TypeError，而 catch 中 isUncertainWrite 对无 code、无 status 的错误默认返回
true（protocol.ts:24），于是走「暂时无法确认评估是否已保存」分支。这与文件顶部 ReceiptMismatchError
注释「回执不匹配是确定性协议错误，不得按未知写入恢复」直接矛盾——同一文件里 importAttempt/importURLAttempt 抛 ReceiptMismatchError 并显式
instanceof 检查，评估流程却退化为 TypeError。一旦服务端返回错配回执，用户会陷入 unknown→查询回执→再次错配→unknown 的死循环，永远到不了确定的 error
提示。建议统一抛 ReceiptMismatchError 并在 catch 中优先 instanceof 分流。

-    if (next.requestId !== requestId || next.opportunityId !== fixedReceipt.opportunityId || next.snapshotId !== fixedReceipt.snapshotId) throw new TypeError('评估回执与固定职位快照不匹配')
+ if (next.requestId !== requestId || next.opportunityId !== fixedReceipt.opportunityId || next.snapshotId !== fixedReceipt.snapshotId) throw new ReceiptMismatchError('评估回执与固定职位快照不匹配')
+ // catch 中，在 isUncertainWrite 判定之前优先分流：
+ if (cause instanceof ReceiptMismatchError) { setEvaluationState('error'); setEvaluationMessage(`评估未完成：${cause.message}，已放弃本次结果。请重新评估。`); return }


─── apps/web/src/career/OpportunityPage.tsx:151-151 ───
[style · low] 嵌套三元表达式，违反检查清单「不允许嵌套三元」。此处 setMessage 内是「idempotency_conflict →
PAYLOAD_TOO_LARGE/request_too_large → 默认」的两层链式赋值；importURLAttempt 中有同构写法，评估按钮的 onClick
与文案（evaluationState 五分支）更是三到四层链式三元，可读性差且难以维护。建议把错误码→文案的映射收敛为小函数或 Record 映射，分支逻辑抽成具名辅助函数。

-     setMessage(parsed.code === 'idempotency_conflict' ? '请求编号已对应其他内容，服务器拒绝了本次提交。请检查内容后使用新的请求重新保存。' : parsed.code === 'PAYLOAD_TOO_LARGE' || parsed.code === 'request_too_large' ? '职位描述超过服务端允许的大小，请缩短后重新保存。' : `职位描述未被接受：${parsed.message}`)
+ function importRejectMessage(code: string | undefined, message: string): string {
+   if (code === 'idempotency_conflict') return '请求编号已对应其他内容，服务器拒绝了本次提交。请检查内容后使用新的请求重新保存。'
+   if (code === 'PAYLOAD_TOO_LARGE' || code === 'request_too_large') return '职位描述超过服务端允许的大小，请缩短后重新保存。'
+   return `职位描述未被接受：${message}`
+ }
+ // 使用：setMessage(importRejectMessage(parsed.code, parsed.message))


─── apps/web/src/career/PreparationPage.tsx:11-11 ───
[maintainability · medium] 与 protocol.ts 已导出的共享实现重复，且存在漂移。同目录 protocol.ts 已导出
newRequestId/errorDetails/isUncertainWrite（OpportunityPage 正在使用），本页（以及
ProgressPage、RulePage、SearchPage）又完整复制了一份，且不确定错误码清单各自漂移：本页多排除
preparation_version_unknown/preparation_generation_failed，SearchPage 多排除
search_quota_refused，RulePage 又是另一个子集。这份清单差异是真实的 bug 温床——日后向 protocol.ts 补充新的确定性别错误码（如新增 4xx
码）时，四份拷贝极易漏改，某页会把确定失败误判为 unknown 引导用户走无意义的回执恢复。建议以 protocol.ts 为基座，把端点特定的错误码作为参数叠加（protocol.ts
注释本身也预期「Pages layer their endpoint-specific error-code whitelists on top of this base
contract」），而不是整函数复制。

- const newRequestId = (): string => typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`
+ // protocol.ts 扩展为可叠加端点码：
+ export function isUncertainWrite(cause: unknown, definiteCodes: readonly string[] = []): boolean {
+   const error = errorDetails(cause)
+   if (error.code === 'TIMEOUT' || error.code === 'outcome_unknown') return true
+   if (['forbidden', 'invalid_request', 'idempotency_conflict', 'request_too_large', 'PAYLOAD_TOO_LARGE', 'not_found', 'unauthorized', ...definiteCodes].includes(error.code ?? '')) return false
+   if (error.status !== undefined) return error.status >= 500 || error.status < 400
+   return true
+ }
+ // 页面侧：
+ import { newRequestId, errorDetails, isUncertainWrite } from './protocol.ts'
+ const definite = ['revision_conflict', 'preparation_version_unknown', 'preparation_generation_failed']


─── apps/web/src/career/ProgressPage.tsx:10-10 ───
[maintainability · medium] 本页完整复制了 protocol.ts 已导出的
newRequestId/errorDetails/isUncertainWrite（OpportunityPage 在用共享版），且不确定错误码清单与
PreparationPage/SearchPage/RulePage 各自漂移（本页多排除 revision_conflict）。清单漂移意味着日后补充确定性别错误码时此页会漏改、把确定失败误判为
unknown。建议收敛到 protocol.ts，端点特定码以参数叠加（详见 PreparationPage 处的建议方案）。

- const newRequestId = (): string => typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`
+ import { newRequestId, errorDetails, isUncertainWrite } from './protocol.ts'


─── apps/web/src/career/RulePage.tsx:330-330 ───
[security · medium] 同 SearchPage 的结果行：todo.link 同样源自服务端抓取的外部岗位链接（search_rule.go 的发现待办继承自
SearchResultRow.link），按项目红线属不可信外部数据，直接渲染为 <a href> 缺少前端协议白名单这一纵深防线（后端目前仅靠 http(s) 前缀白名单兜底）。建议与
SearchPage 一致，在渲染前用 externalCitationTarget（chat/citation.ts）校验，非 http(s) 退化为纯文本。

-       <a href={todo.link} target="_blank" rel="noreferrer noopener">岗位链接</a>
+ const target = externalCitationTarget(todo.link)
+ {target ? <a href={target} target="_blank" rel="noreferrer noopener">岗位链接</a> : <span>{todo.link}</span>}


─── apps/web/src/career/RulePage.tsx:23-23 ───
[maintainability · medium] 本页复制了 protocol.ts 已导出的 errorDetails（newRequestId 另以 makeId 之名重复），并另名实现
isUncertainOutcome，与 PreparationPage/ProgressPage/SearchPage
构成四份漂移拷贝（各自排除的错误码子集不同）。日后向共享版补充确定性别错误码时此页会漏改。建议收敛到 protocol.ts 并以参数叠加端点特定码（见 PreparationPage
处的建议方案）。

- function errorDetails(cause: unknown): TypedError {
+ import { newRequestId as makeId, errorDetails, isUncertainWrite as isUncertainOutcome } from './protocol.ts'


─── apps/web/src/career/SearchPage.tsx:323-323 ───
[security · medium] 外部链接渲染缺少协议白名单（纵深防御缺失）。row.link 来自服务端对已核验招聘站点抓取的内容，按本项目红线属于不可信外部数据。虽然后端
search_once.go 在建行时只接受 http:// 或 https:// 前缀的 token（当前不可达 javascript:/data: URI），但前端将跨边界外部数据直接渲染为 <a
href> 是单一防线：一旦后端来源扩展、校验调整或中间层引入偏差即暴露，且 rel="noreferrer noopener" 只防反向
tabnabbing、不拦截危险协议。项目已有先例：chat/citation.ts 的 externalCitationTarget 会拒绝非 http(s) 协议并在 wiki/embed
等处使用。建议在渲染前复用同等的 URL 协议校验，不通过时退化为纯文本展示。

-         <a href={row.link} target="_blank" rel="noreferrer noopener">原始链接</a>
+ // 复用 chat/citation.ts 的 externalCitationTarget，或新增同等校验：
+ const target = externalCitationTarget(row.link)
+ {target ? <a href={target} target="_blank" rel="noreferrer noopener">原始链接</a> : <span>{row.link}</span>}


─── apps/web/src/career/SearchPage.tsx:26-26 ───
[maintainability · medium] 本页复制了 protocol.ts 已导出的 errorDetails/isUncertainWrite（makeId 重复
newRequestId），且不确定错误码清单与 PreparationPage/ProgressPage/RulePage 各自漂移（本页独有
search_quota_refused）。清单漂移意味着共享版修复时此页漏改、会把确定失败（如新增的 4xx 码）误判为 unknown 引导用户走无意义的回执恢复。建议收敛到
protocol.ts 并以参数叠加端点特定码（见 PreparationPage 处的建议方案）。

- function errorDetails(cause: unknown): TypedError {
+ import { newRequestId as makeId, errorDetails, isUncertainWrite as isUncertainOutcome } from './protocol.ts'


─── apps/web/src/career/SearchPage.tsx:44-46 ───
[security · low] 搜索历史（含找岗指令明文）持久化在 localStorage，登出/空间切换后不清理。clearForScopeChange
只清内存态（setHistoryEntries([])），磁盘上的 weknora:career:search-history:userId:tenantId
条目在登出后仍残留——键虽按身份隔离避免了跨用户读取，但共用设备上登出后明文求职意图仍留在磁盘，与求职场景的隐私预期有偏差。RulePage 的 rule-id
保留是注释中有意的设计决策（避免误删下一空间的引用），而本页的搜索指令明文没有同等理由。建议：或改为 sessionStorage / 仅内存，或至少在登出（scope abort
到非新登录场景）时清除当前身份的搜索历史键。

- function historyKey(userId: string | null, tenantId: string | null): string {
-  return `weknora:career:search-history:${userId ?? ''}:${tenantId ?? ''}`
+ // 方案一：登出时清理当前身份的键（与内存清理同步）：
+ const clear = () => {
+   const storage = typeof window === 'undefined' ? undefined : window.localStorage
+   try { storage?.removeItem(historyKey(scope.userId, scope.tenantId)) } catch { /* private mode */ }
+   clearForScopeChange('空间已切换或登录已失效，已清除本次找岗状态。')
  }
+ // 方案二：改用 sessionStorage，随会话结束自动清除。


─── apps/web/src/career/SearchPage.tsx:3-5 ───
[maintainability · low] 同一文件混用两种导入方式：WeKnoraClient 走 @weknora/api-client 包名，career 类型却用
../../../../packages/api-client/src/career.ts 四层深层相对路径，绕过了包边界。packages/career-core/package.json 已声明
exports（./contracts、./desk），api-client
也有包名入口，说明包名/子路径导入是可行的；深层相对路径在目录重构或包结构调整时极易编译失败，且同一来源并存两种导入路径增加维护成本。此问题同样存在于
PreparationPage/ProgressPage/RulePage/OpportunityPage（career 目录整体如此），建议作为模块级统一项收敛为包名导入（必要时在 web 的
tsconfig/vite 中补 @weknora/career-core 别名），避免只改单文件造成新的不一致。

  import type { WeKnoraClient } from '@weknora/api-client'
  import type { ScopeController } from '@weknora/domain/scope'
- import type { SearchFailureCode, SearchOnceReceipt, SearchQualification, SearchUncertainty, SearchResultRow } from '../../../../packages/api-client/src/career.ts'
+ import type { SearchFailureCode, SearchOnceReceipt, SearchQualification, SearchUncertainty, SearchResultRow } from '@weknora/api-client/career' // 或经 api-client 主入口 re-export


─── apps/web/src/career/SubmissionPage.tsx:157-160 ───
[bug · medium] 成功反馈永远不会显示：acceptReceipt 在写入成功后同时执行 setWritePhase('idle') 与
setMessage('投递已记录…')，但渲染条件是 `{message && writePhase !== 'idle' ? … : null}`，'idle'
阶段下成功消息被直接吞掉。用户完成投递确认后只能从时间线刷新间接感知结果，与错误分支的可见性不对称。建议为 WritePhase 增加 'saved' 阶段（composeBlocked 与
runWrite 的守卫均只判 busy/unknown，不受影响），渲染条件即可正常显示成功消息。

   const acceptReceipt = (next: SubmissionReceipt, expected: WriteAttempt): void => {
    if (next.requestId !== expected.requestId || next.applicationId !== applicationId) throw new ReceiptMismatchError('投递回执与本次请求不匹配')
-   setAttempt(undefined); setWritePhase('idle'); setMessage('投递已记录。记录已呈现在投递时间线，可回看绑定的材料版本。'); resetCompose(); refresh()
+   setAttempt(undefined); setWritePhase('saved'); setMessage('投递已记录。记录已呈现在投递时间线，可回看绑定的材料版本。'); resetCompose(); refresh()
   }
+ // 同时将 type WritePhase 扩展为 'idle' | 'busy' | 'unknown' | 'error' | 'saved'


─── apps/web/src/career/SubmissionPage.tsx:10-10 ───
[maintainability · medium] 本文件重复定义了 newRequestId / errorDetails / isUncertainWrite，而同批新增的
protocol.ts 已声明为共享"写入恢复红线"的唯一实现，且本组的 reconciliation.tsx 已改为从 protocol.ts 导入。两份 isUncertainWrite
的确定拒绝码表已经分叉（本地版多出 revision_conflict / submission_already_confirmed /
export_not_submittable），后续任一侧调整都会导致各页面写入恢复行为不一致。建议删除本地副本统一从 './protocol.ts' 导入；submission 专属的确定码应并入
protocol.ts 的清单（见对 protocol.ts 的评论）。

- const newRequestId = (): string => typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`
+ import { errorDetails, isUncertainWrite, newRequestId } from './protocol.ts'


─── apps/web/src/career/SubmissionPage.tsx:4-4 ───
[maintainability · low] 同一文件里 WeKnoraClient 走 '@weknora/api-client' 别名，类型却经
'../../../../packages/api-client/src/career.ts' 深相对路径跨包引入（本组 UsagePanel.tsx、reconciliation.tsx
同样如此）。深相对导入绕过包导出映射，在 tsconfig include 收紧、vite fs.allow 调整或包 exports 变更时易造成构建断裂。当前 index.ts 只导出
createCareerApi/CareerRequest 等少数符号，建议在 packages/api-client/src/index.ts 补充导出 SubmissionReceipt
等视图类型后，统一改走包入口。

- import type { MaterialExportReceipt, MaterialVersionView, SubmissionChannel, SubmissionReceipt } from '../../../../packages/api-client/src/career.ts'
+ import type { MaterialExportReceipt, MaterialVersionView, SubmissionChannel, SubmissionReceipt } from '@weknora/api-client'
+ // 前置：在 packages/api-client/src/index.ts 增加 export type { MaterialExportReceipt, MaterialVersionView, SubmissionChannel, SubmissionReceipt, UsageEstimateView, … } from './career.ts'


─── apps/web/src/career/SubmissionPage.tsx:177-177 ───
[maintainability · medium] 写入载荷先构造为 Record<string, unknown> 再 `as Parameters<typeof
client.career.recordSubmission>[0]` 强转，编译期不再校验 versionUnknown/expectedRevision/occurredAt/materialId
等字段名，拼写漂移只能运行期以 invalid_request 暴露。RecordSubmissionInput 在 packages/api-client/src/career.ts:834
已有完整类型，应按 versionUnknown 分支直接构造类型化对象。另注意一个连带后果：career.ts:1460-1461 会在请求发出前对绑定关系做同步校验并抛
TypeError（例如刷新后所选 exportId 已不在 exports 列表、materialId 为 undefined），而本页把无 code/status
的错误一律判为"写入结果未知"，这类确定未发出的客户端校验失败会被错误地置入 'unknown' 回执恢复态，违背"确定失败不得进入恢复路径"的红线。建议先本地求值并在找不到 materialId
时直接置 error，或区分客户端校验错误与传输错误。

-    const next = await client.career.recordSubmission(current.input as Parameters<typeof client.career.recordSubmission>[0], requestScope.signal)
+    const input: RecordSubmissionInput = versionChoice === UNKNOWN_VERSION_CHOICE
+     ? { requestId, applicationId, channel, versionUnknown: true, expectedRevision: revision, ...(occurred ? { occurredAt: occurred } : {}), ...(trimmedNote ? { note: trimmedNote } : {}) }
+     : { requestId, applicationId, channel, versionUnknown: false, materialId: exports?.find((receipt) => receipt.exportId === versionChoice)?.materialId ?? '', exportId: versionChoice, expectedRevision: revision, ...(occurred ? { occurredAt: occurred } : {}), ...(trimmedNote ? { note: trimmedNote } : {}) }
+    const next = await client.career.recordSubmission(input, requestScope.signal)


─── apps/web/src/career/SubmissionPage.tsx:252-252 ───
[style · low] 渲染层存在嵌套三元链（此处 readState 的 loading→error→内容三分支，以及下方 revisionState 的
loading→error→ready→空 四分支），违反检查表"禁止嵌套三元"，可读性差。另外 runWrite 的 input 对象字面量中
occurredAtFromInput(occurredAt) 在每个分支的展开条件与取值处被重复求值两次（每次都解析一遍 Date），应先求值一次存入局部变量再使用。

-    {readState === 'loading' ? <p className="wk-submission__state" role="status" aria-busy="true">正在读取投递记录…</p> : readState === 'error' ? <p className="wk-submission__message wk-submission__message--error" role="alert">{readMessage}</p> : <>
+  const renderReadState = (): ReactNode => {
+   if (readState === 'loading') return <p className="wk-submission__state" role="status" aria-busy="true">正在读取投递记录…</p>
+   if (readState === 'error') return <p className="wk-submission__message wk-submission__message--error" role="alert">{readMessage}</p>
+   return <>…</>
+  }
+ // runWrite 内：const occurred = occurredAtFromInput(occurredAt)，后续仅引用 occurred


─── apps/web/src/career/SubmissionPage.tsx:284-284 ───
[bug · medium] 版本回看元信息展示了错误的材料来源：此处渲染的是组件 prop `materialId ?? ''`，但实际回看的是 `binding.materialId`
对应的版本（reviewVersion 用 binding.materialId 发起请求）。MaterialVersionView 本身不含 materialId 字段，而
ApplicationPage 传入的 careerMaterialId 初始为 undefined，且历史记录的 boundVersion.materialId
完全可能不同于当前材料（如材料重建后回看旧投递绑定）。结果是在"如实呈现绑定版本"的证据面上，材料编号会显示为空或张冠李戴。建议在 setVersionDetail 的同时保存 binding（或其
materialId），元信息行渲染绑定来源而非 prop。

- <p className="wk-submission__version-meta">材料 <code>{materialId ?? ''}</code> · 固定档案修订 {versionDetail.pinnedEvidence.profileRevision} · 事实基准修订 {versionDetail.factBasisRevision} · 确认请求 <code>{versionDetail.requestId}</code> · <time dateTime={versionDetail.createdAt}>{versionDetail.createdAt}</time></p>
+ // 在 reviewVersion 中同时保存 binding，例如：
+ // const [versionBinding, setVersionBinding] = useState<NonNullable<SubmissionReceipt['boundVersion']>>()
+ <p className="wk-submission__version-meta">材料 <code>{versionBinding?.materialId ?? ''}</code> · 固定档案修订 {versionDetail.pinnedEvidence.profileRevision} · …</p>


─── apps/web/src/career/SubmissionPage.tsx:225-226 ───
[bug · medium] lookupReceipt 的 catch 缺少 ReceiptMismatchError 分支：acceptReceipt
在回执与请求不匹配时抛出的是确定性协议错误（文件顶部注释与本页 runWrite 的处理均如此界定），但这里被笼统落入
setWritePhase('unknown')，文案为"投递回执暂时无法读取…可稍后重试查询"——把确定错误伪装成瞬态故障，用户会被困在"查询/原编号重试"循环里。reviewVersion
中同样的 mismatch 也被标为"版本回看暂时无法读取"。建议在 forbidden 分支之后先判断 `cause instanceof ReceiptMismatchError`，单独置
'error' 并给出明确文案，与 runWrite 的处理对齐。

+    if (cause instanceof ReceiptMismatchError) { setWritePhase('error'); setMessage(`投递回执与本次请求不匹配：${cause.message}`); return }
     setWritePhase('unknown')
     setMessage(parsed.code === 'not_found' ? `尚未找到投递回执（原请求编号 ${current.requestId}）。可以继续查询，或使用原请求编号重试。` : '投递回执暂时无法读取。原请求编号已保留，可稍后重试查询。')


─── apps/web/src/career/SubmissionPage.tsx:170-170 ───
[bug · low] 刷新窗口内可提交注定失败的载荷：refresh() 会把 exports 重置为 undefined 再重新拉取，但 versionChoice 仍保留旧的
exportId，submitBlocked 只检查 channel/versionChoice/revision，因此用户在"刷新投递记录→确认投递"的间隙可以提交。此时该表达式求值为
undefined，而后端（internal/modules/career/submission.go:218）在 versionUnknown=false 时强制要求非空
materialId，请求必然以 invalid_request 被拒。同时版本下拉因取值无匹配项而显示空白，且 `(exports ?? []).length === 0`
的提示在加载中（exports===undefined）就提前显示"尚无可投递版本"。建议：exports 重新加载时重置 versionChoice（或在 submitBlocked 中校验所选
exportId 仍存在于当前 exports），并把"加载中"与"确为空"两种状态区分开。

- : { requestId, applicationId, channel, versionUnknown: false, materialId: exports?.find((receipt) => receipt.exportId === versionChoice)?.materialId, exportId: versionChoice, expectedRevision: revision, ...(occurredAtFromInput(occurredAt) ? { occurredAt: occurredAtFromInput(occurredAt) } : {}), ...(note.trim() ? { note: note.trim() } : {}) }
+ // exports 重新拉取时重置失效的选择：
+ useEffect(() => { setVersionChoice('') }, [reload])
+ // 或在提交前校验：const chosen = exports?.find((receipt) => receipt.exportId === versionChoice)
+ // if (!chosen) return  // 所选导出已不在当前清单中，等待刷新后重选


─── apps/web/src/career/SubmissionPage.tsx:46-46 ───
[maintainability · low] 可投递导出的判定谓词与 MaterialPage.tsx:567 的内联实现逐字重复（status==='submittable' &&
submittable && files.length>=2 && every verified），注释自己也承认是"mirrors
MaterialPage"。这是投递准入的关键业务规则，两份手抄副本意味着后端规则一旦调整，两个页面会各自漂移、且没有任何编译期约束。建议将该谓词提取为共享 helper（如
packages/career-core 或本目录共享模块），MaterialPage 与 SubmissionPage 共同引用。

- const deliverableExports = (list: MaterialExportReceipt[]): MaterialExportReceipt[] => list.filter((receipt) => receipt.status === 'submittable' && receipt.submittable && receipt.files.length >= 2 && receipt.files.every((file) => file.verified))
+ // 提取到共享模块（如 career-core 或本目录 material-shared.ts）：
+ // export const isDeliverableExport = (receipt: MaterialExportReceipt): boolean =>
+ //   receipt.status === 'submittable' && receipt.submittable && receipt.files.length >= 2 && receipt.files.every((file) => file.verified)
+ const deliverableExports = (list: MaterialExportReceipt[]): MaterialExportReceipt[] => list.filter(isDeliverableExport)


─── apps/web/src/career/UsagePanel.tsx:22-25 ───
[maintainability · low] usageErrorDetails 与 protocol.ts 的 errorDetails
重复（仅少两个字段），同一变更批次内已出现第三份错误解析实现。建议直接复用：`import { errorDetails } from './protocol.ts'`，取 `.code` /
`.message` 即可，避免后续错误结构演进时各副本不同步。

- function usageErrorDetails(cause: unknown): { code?: string; text: string } {
-  const error = cause as { code?: string; message?: string }
-  return { code: error?.code, text: error?.message || '请求未完成' }
- }
+ import { errorDetails } from './protocol.ts'
+ // 用 errorDetails(cause).code / errorDetails(cause).message 替换 usageErrorDetails


─── apps/web/src/career/application.css:74-78 ───
[maintainability · low] 品牌绿 #07c05f 在本文件多处硬编码（另有警示色 #ffb648/#fff7e8/#4a3200），而 design-tokens 已提供
--wk-color-brand 与 tdesign-theme.css 的 --td-brand-color（值同为 #07c05f，且本文件其余部分已在用 --td-* 令牌）。主题色调整时需跨
application/export-deletion/inbox/material 四个新增 CSS 手工同步，易漏改造成色值漂移。建议品牌绿改用 var(--td-brand-color,
#07c05f)，警示色对齐 --td-warning-* 系列。

  .wk-application__submit:not(:disabled) {
-   border-color: #07c05f;
-   background: #07c05f;
+   border-color: var(--td-brand-color, #07c05f);
+   background: var(--td-brand-color, #07c05f);
    color: #fff;
  }


─── apps/web/src/career/application.css:44-46 ───
[maintainability · low] input[type='text'] 是死选择器：ApplicationPage 中唯一的文本输入 #wk-application-batch
未显式声明 type（属性选择器不匹配缺省 type 的 input），其余 input 均为
radio/checkbox，该行永不命中，实际样式全部由后两个选择器承担。建议删除以免误导后续维护者以为存在显式 text 输入。

- .wk-application input[type='text'],
  .wk-application__label + input,
  .wk-application #wk-application-batch {


─── apps/web/src/career/export-deletion.css:43-47 ───
[maintainability · low] #07c05f 品牌绿硬编码，且用 !important 压制同文件 .wk-lifecycle__actions button
的背景/边框（后者选择器特异性更高）。design-tokens 的 tdesign-theme.css 已提供 --td-brand-color（同为
#07c05f），--td-error-color 亦已存在。建议令牌化的同时理顺选择器（如为按钮变体单列类名），去掉 !important，避免主题调整时四处同步 + 特异性军备竞赛。

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
[maintainability · low] 品牌绿 #07c05f 在 .wk-inbox__intro、.wk-inbox
button:hover、.wk-inbox__submit、.wk-inbox__todo-link 多处硬编码，而 design-tokens 已提供 --wk-color-brand 与
tdesign-theme.css 的 --td-brand-color（值同为 #07c05f）。主题色调整时需跨四个新增 CSS 手工同步，易漏改造成色值漂移，建议统一改用令牌变量。

- .wk-inbox__submit { border-color: #07c05f; background: #07c05f; color: #fff; }
+ .wk-inbox__submit { border-color: var(--td-brand-color, #07c05f); background: var(--td-brand-color, #07c05f); color: #fff; }


─── apps/web/src/career/material.css:133-137 ───
[maintainability · low] 品牌绿 #07c05f 在 .wk-material__compare-pane
h4、.wk-material__confirm、.wk-material__export-submittable 等多处硬编码，警示色 #ffb648/#fff7e8/#b45309
同样散落（.wk-material__claim--needs-review 等）；design-tokens 已提供 --wk-color-brand 与 --td-brand-color（同为
#07c05f）及 --td-warning-* 系列。主题调整时需跨四个新增 CSS 手工同步，建议收敛为令牌变量。

  .wk-material__confirm:not(:disabled) {
-   border-color: #07c05f;
-   background: #07c05f;
+   border-color: var(--td-brand-color, #07c05f);
+   background: var(--td-brand-color, #07c05f);
    color: #fff;
  }


─── apps/web/src/career/material.css:51-53 ───
[maintainability · low] input[type='text'] 是死选择器：MaterialPage 的标题/事实编号输入框均未显式声明 type（属性选择器不匹配缺省 type
的 input），唯一的显式 type 是 checkbox，该行永不命中。实际样式由后两个选择器承担，建议删除该行避免误导。

- .wk-material input[type='text'],
  .wk-material .wk-material__label input,
  .wk-material .wk-material__label textarea {


─── apps/web/src/career/protocol.ts:22-22 ───
[maintainability · medium] "共享唯一实现"的承诺目前未兑现：同批新增的 SubmissionPage.tsx（及 MaterialPage.tsx）仍保留本地
newRequestId/errorDetails/isUncertainWrite 副本，且 SubmissionPage 版的确定拒绝码表与本清单已经分叉（多出
revision_conflict、submission_already_confirmed、export_not_submittable）。这些 submission
专属码在页面里虽已被显式分支先处理，但本地兜底清单与共享清单不一致，意味着任何绕过显式分支的新码在两个页面的"未知/确定"判定会不同。建议把端点专属确定码作为可组合参数并入本模块（页面叠加白名单），让
全站共享同一判定入口。

-  if (['forbidden', 'invalid_request', 'idempotency_conflict', 'request_too_large', 'PAYLOAD_TOO_LARGE', 'not_found', 'unauthorized'].includes(error.code ?? '')) return false
+ const baseDefiniteCodes = ['forbidden', 'invalid_request', 'idempotency_conflict', 'request_too_large', 'PAYLOAD_TOO_LARGE', 'not_found', 'unauthorized']
+ export function isUncertainWrite(cause: unknown, extraDefiniteCodes: readonly string[] = []): boolean {
+  const error = errorDetails(cause)
+  if (error.code === 'TIMEOUT' || error.code === 'outcome_unknown') return true
+  if ([...baseDefiniteCodes, ...extraDefiniteCodes].includes(error.code ?? '')) return false
+  if (error.status !== undefined) return error.status >= 500 || error.status < 400
+  return true
+ }


─── apps/web/src/career/reconciliation.tsx:6-6 ───
[maintainability · medium] 循环依赖：本文件从 OpportunityPage.tsx 导入 opportunityEvidencePath，而
OpportunityPage.tsx 第 7 行又反向导入本文件导出的 OpportunityStatusPanel（并在 421 行渲染），构成 OpportunityPage ↔
reconciliation 模块环。打包器会告警，且未来若把路径函数改为模块初始化期求值或调整加载顺序，容易触发运行时 TDZ 错误。建议把 opportunityEvidencePath
下沉到独立的路径模块（如 ./paths.ts），双方及 ApplicationPage/InboxPage/SearchPage 等现有引用方一并切换，消除环。

- import { opportunityEvidencePath } from './OpportunityPage.tsx'
+ // 新建 apps/web/src/career/paths.ts：
+ // export function opportunityEvidencePath(opportunityId: string, snapshotId: string): string { … }
+ import { opportunityEvidencePath } from './paths.ts'


─── apps/web/src/career/reconciliation.tsx:83-83 ───
[bug · low] 回执 opportunityId 与请求不匹配时直接 return，phase 停留在
'loading'，面板永久显示"正在读取岗位状态与对账历史…"且无任何错误提示或重试入口。回执与请求不匹配是确定性协议错误（参考 SubmissionPage 对
ReceiptMismatchError 的处理），应与 active/scope 失效区分开，单独置为 error 状态。

-    if (!active || !scopeController.isCurrent(requestScope.scope) || nextStatus.opportunityId !== opportunityId) return
+    if (!active || !scopeController.isCurrent(requestScope.scope)) return
+    if (nextStatus.opportunityId !== opportunityId) { setPhase('error'); return }


─── apps/web/src/career/reconciliation.tsx:128-128 ───
[bug · medium] 回执 requestId 不匹配抛出的 TypeError 会被下方 catch 的 isUncertainWrite 判为"结果未知"（无 code、无 status
→ true），进入回执恢复流程——这与 protocol.ts 注释"确定性协议错误不得路由进回执恢复"的红线相悖，也与同批 SubmissionPage 用
ReceiptMismatchError 明确区分同类情况的做法不一致。更糟的是 lookupReconcileReceipt 内部同样的 TypeError 会被自己的 catch 吞掉再次置
'unknown'，用户可能陷入"查询/重试"循环。建议定义专门的 ReceiptMismatchError 并在 catch 顶部识别、直接置 'error'。

-    if (next.requestId !== currentAttempt.requestId) throw new TypeError('对账回执与本次请求编号不匹配')
+ class ReceiptMismatchError extends Error {}
+ // runReconcile / lookupReconcileReceipt 中：
+    if (next.requestId !== currentAttempt.requestId) throw new ReceiptMismatchError('对账回执与本次请求编号不匹配')
+ // catch 顶部：
+    if (cause instanceof ReceiptMismatchError) { setReconcilePhase('error'); setReconcileMessage(`对账未完成：${cause.message}`); return }


─── apps/web/src/career/reconciliation.tsx:144-145 ───
[bug · low] beginReconcile/retryReconcile 仅靠 reconcilePhase 状态守卫防重入，与 SubmissionPage 使用
writeInFlight ref 的防御深度不一致；在状态提交前的极端并发（如辅助技术重复派发事件、同一帧内多入口触发）下，可携带两个不同 requestId 并发发起两次对账判定请求。建议补一个
in-flight ref 防抖，与 SubmissionPage 对齐。

+  const reconcileInFlight = useRef(false)
   const beginReconcile = async (): Promise<void> => {
-   if (reconcilePhase === 'busy' || reconcilePhase === 'unknown' || !candidateDraft.trim()) return
+   if (reconcileInFlight.current || reconcilePhase === 'busy' || reconcilePhase === 'unknown' || !candidateDraft.trim()) return
+   reconcileInFlight.current = true
+   try { … } finally { reconcileInFlight.current = false }
+  }


─── apps/web/src/career/reconciliation.tsx:174-174 ───
[style · low] 嵌套三元链（此处 phase 的 error→forbidden→scope-changed 三分支；下方 diffPhase 的 ready→error→loading
链，以及 CareerCoveragePanel 的 loading→error→forbidden→scope-changed 链同样如此），违反检查表"禁止嵌套三元"。建议改为提前 return
或抽取局部渲染函数，按 phase 逐个分支渲染。

-  if (phase !== 'ready' || !status) return <section className="wk-reconciliation"><h2>岗位状态与对账</h2><p role={phase === 'error' ? 'alert' : 'status'}>{phase === 'error' ? '岗位状态暂时无法读取。' : phase === 'forbidden' ? '当前空间不可访问此岗位。' : '空间已切换，已清除岗位状态。'}</p>{phase === 'error' ? <button type="button" onClick={retry}>重试</button> : null}</section>
+  if (phase !== 'ready' || !status) {
+   const text = phase === 'error' ? '岗位状态暂时无法读取。' : phase === 'forbidden' ? '当前空间不可访问此岗位。' : '空间已切换，已清除岗位状态。'
+   return <section className="wk-reconciliation"><h2>岗位状态与对账</h2><p role={phase === 'error' ? 'alert' : 'status'}>{text}</p>{phase === 'error' ? <button type="button" onClick={retry}>重试</button> : null}</section>
+  }


─── apps/web/src/career/reconciliation.tsx:215-215 ───
[bug · low] diffPhase 的 'idle' 被当作 loading 渲染：当用户在两个下拉中选中同一快照（oldObservation.snapshotId ===
newObservation.snapshotId）时，副作用把 diffPhase 置回 'idle' 并跳过请求，但这里的三元链最终 else 分支对 idle 与 loading
一视同仁，永久显示"正在读取两个固定快照…"。这是可达的边界（旧/新下拉选项相同），用户会一直等待一个不会开始的加载。建议为 idle 状态渲染明确文案（如"两次选择为同一快照，无需对比"），与真正的
loading 区分。

- </div> : diffPhase === 'error' ? <p role="status">快照对比暂时无法读取，可稍后重试；两个快照本身仍可分别打开。</p> : <p role="status">正在读取两个固定快照…</p>}
+ </div> : diffPhase === 'error' ? <p role="status">快照对比暂时无法读取，可稍后重试；两个快照本身仍可分别打开。</p> : diffPhase === 'loading' ? <p role="status">正在读取两个固定快照…</p> : <p role="status">两次选择为同一快照，无需对比。</p>}


─── apps/web/src/career/submission.css:55-57 ───
[maintainability · low] --td-text-color-warning
在主题（packages/design-tokens/src/tdesign-theme.css）中不存在，全仓库仅本文件两处引用（56 行与 112 行），因此永远落入 fallback
#e37318；且 fallback 值与主题实际 --td-warning-color-5 (#ed7b2f) 也不一致。同批的 reconciliation.css 对同类警示场景使用的是规范
token var(--td-warning-color-5, …)，两文件口径不统一。建议统一改用 --td-warning-color-5，fallback
对齐主题值，避免主题演进时这两处颜色静默脱离体系。

  .wk-submission__record-version--unknown {
-   color: var(--td-text-color-warning, #e37318);
+   color: var(--td-warning-color-5, #ed7b2f);
  }


─── apps/web/src/chat/ChatRoutePage.tsx:1935-1936 ───
[bug · low] 双重确认：ChatHeader 弹层内已渲染清空/删除的二次确认（clearConfirm*/deleteConfirm* 文案与按钮），而
clearMessages/deleteSession 内部还会再弹一次 window.confirm（同为 clearConfirmBody/deleteConfirmBody
文案），用户需连续确认两次；且若用户在原生弹窗点取消，clearMessages 静默返回，ChatHeader 却按成功关闭确认层并复位菜单。建议为 headerSlot 传入跳过原生
confirm 的执行函数，或由 ChatHeader 移除弹层确认，保证单一确认入口。



─── apps/web/src/chat/ChatRoutePage.tsx:10-10 ───
[maintainability · low] chat 路由顶层静态引入 career 域的 OpportunityPage 模块，该文件内部又引入
ApplicationPage、reconciliation、protocol 等整页依赖，会把整组 career 页面（含其样式）打进 chat chunk，并使 chat 域对 career
域产生编译期硬依赖。OpportunityImportPanel 的 props（client/scopeController）契约匹配、功能可用，但建议将其抽为独立小组件文件，或以
React.lazy 异步挂载以收敛影响面。



─── apps/web/src/chat/chat-header.tsx:108-119 ───
[bug · medium] 重命名失败时错误提示永远不可见：在 await 之前已执行 setTitleEditing(false)，而 renameError 的 <span
class="chat-header__edit-error"> 只渲染在 titleEditing 为 true 的表单分支内。宿主 ChatRoutePage 的
renameSession（1404 行）无内部 try/catch、失败会抛错，因此该 catch 必然可达，但用户得不到任何失败反馈，catch 中的 setRenameError
属死代码。建议失败时保持编辑态（input 已支持 disabled={renameBusy}）展示错误并聚焦，成功后再退出编辑。

      renameSubmittingRef.current = true;
      setRenameBusy(true);
      setRenameError(null);
      try {
-       setTitleEditing(false);
-       setTitleDraft('');
        await props.onRenameSession(title);
-     } catch {
        setTitleEditing(false);
        setTitleDraft('');
+     } catch {
        setRenameError(props.renameTitleFailed);
+       titleInputRef.current?.focus();
      } finally {


─── apps/web/src/chat/chat-header.tsx:175-179 ───
[maintainability · low] 死代码：两个分支的调用完全相同（onMenuVisibleChange(visible)），if
条件判断为冗余，注释声称的差异化处理并未实现。直接调用即可；若确需区分 document 触发的关闭，请按注释意图实现差异逻辑。

-           onVisibleChange={(visible, context) => {
-             // trigger click 以外（Esc/外点）的关闭同样回到 menu 态
-             if (context?.trigger === 'document') { onMenuVisibleChange(visible); return; }
+           onVisibleChange={(visible) => {
              onMenuVisibleChange(visible);
            }}


─── apps/web/src/chat/chat-header.tsx:125-131 ───
[maintainability · low] 恒假守卫 + 脆弱语义：menuMode 类型为 'menu' | 'clear' | 'delete'，永远是非空字符串，`!menuMode` 恒为
false；且当 menuMode === 'menu' 时 else 分支会误调 onDeleteSession——当前 UI 不可达（确认按钮仅在 clear/delete
态渲染），但语义脆弱，后续改动易踩坑。建议显式排除 menu 态。

    async function submitDangerAction(): Promise<void> {
-     if (!menuMode || dangerBusy) return;
+     if (menuMode === 'menu' || dangerBusy) return;
      setDangerBusy(true);
      setDangerError(null);
      try {
        if (menuMode === 'clear') await props.onClearSession?.();
        else await props.onDeleteSession?.();


─── apps/web/src/chat/chat-header.tsx:4-5 ───
[maintainability · low]
样式依赖未随组件导入：本组件全部样式（.chat-header*、.chat-header-menu*、.sandbox-header-toggle*）位于
chat.td.css，但这里只导入了不含这些规则的 views-chat-u.css；当前仅因宿主 ChatRoutePage 恰好导入了 chat.td.css
才正常显示，组件一旦被其它入口复用（含测试渲染）将整体无样式。建议组件自行导入所需样式文件。

  import type { ChatCopyTable } from '@weknora/views/chat/chat-copy';
  import './views-chat-u.css';
+ import './chat.td.css';


─── apps/web/src/chat/chat.td.css:657-667 ───
[maintainability · low] 同一块内 color 声明两次，前值 var(--td-text-color-secondary) 为死代码（最终生效的是
--td-error-color）。从 Vue 源平移层叠覆盖时应保留最终值并删除被覆盖的声明。



─── apps/web/src/chat/chat.td.css:288-288 ───
[maintainability · low] 缺失样式：chat-header.tsx:164 使用了 .chat-header__edit-error（重命名错误 span），但全库没有任何
CSS 定义该类（chat-page.test.ts 仅断言类名存在于源码中）。即便修复了"失败后退出编辑态导致错误不可见"的问题，该错误提示也会以浏览器默认样式裸奔。建议在 ChatHeader
样式段补充错误文本样式。

  .chat-header__edit-input:disabled { opacity: 0.7; }
+ 
+ .chat-header__edit-error {
+   color: var(--td-error-color-6);
+   font-size: 12px;
+   line-height: 18px;
+ }


─── apps/web/src/chat/views-chat-u.css:2364-2367 ───
[maintainability · low] @keyframes pulse 在本文件定义了两次（page 段与 message-list 段各一次），此处为重复定义；另外
.wk-vc-page-50/-54 引用的 wk-chat-skeleton / wk-chat-sq-refresh-rotate 实际定义在 chat.css 中（chat
路由下已加载可用，但本文件被单独消费时会静默失效）。建议删除重复的 keyframes，并在文件头注明跨文件动画依赖（或将 keyframes 内联到本文件）。

- @keyframes pulse { 50% { opacity: .5; } }
- 
- 
  /* ---- tool-approval.tsx ---- */


─── apps/web/src/chat/views-chat-u.css:1670-1677 ───
[maintainability · low] codemod 平移痕迹（死声明）：font-size/color 的前值（0.7rem / #8a94a6）在同一块内即被自身 !important
版本覆盖，成为永不生效的死代码。同类问题还有 wk-vc-tool-result-8/-14/-15/-24/-25、wk-vc-page-23（font-size 先 inherit 后
13px），以及多处 "border-*-width: 1px;border-color: …" 连写。建议清理为单一最终值，避免误导后续维护并减少 diff 噪音。

    margin-right: 0.35rem;
-   font-size: 0.7rem;
    font-weight: 600;
-   color: #8a94a6;
    font-family: var(--app-font-family-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace) !important;
    font-size: 0.8rem !important;
    color: rgba(0, 0, 0, 0.6) !important;
  }


─── apps/web/src/commercial/BillingPage.tsx:7-8 ───
[style · low] './commercial-u.css' 为冗余副作用导入：本页经由 './surface.tsx' 引入 Card/Status，而 surface.tsx 内部已
import 该 CSS，样式必然生效，此处可删（若想显式声明样式依赖，也应统一为 surface.tsx 单点导入）。另外：USAGE_NUMBER_CELL 常量后连续两个空行属多余；模板中
wk-bill-1 / wk-bill-2 命名无语义（实际分别是表头行底边框与表头加粗），与同文件 wk-bill-usage-table 等语义命名风格混用，建议改为
wk-bill-thead-row / wk-bill-thead-cell 类语义名。



─── apps/web/src/commercial/commercial-u.css:40-41 ───
[maintainability · low] 同一选择器 .wk-bill-usage-number-cell 在本文件出现两次（上文第 20-22 行的 text-align: right
与此处的 font-variant-numeric），分散定义在后续维护中极易只改一处漏改另一处，建议合并为单条规则（text-align: right; font-variant-numeric:
tabular-nums;），注释合并说明两项均来自旧栈 utility（text-right + tabular-nums）。

- /* T15 语义化：原 tabular-nums（用量数值单元格）。 */
- .wk-bill-usage-number-cell { font-variant-numeric: tabular-nums; }
+ /* T15 语义化：原 text-right + tabular-nums（用量数值单元格）。 */
+ .wk-bill-usage-number-cell { text-align: right; font-variant-numeric: tabular-nums; }


─── apps/web/src/commercial/commercial-u.css:13-13 ───
[style · low] 一行内挤入两条声明（border-bottom-width 与 border-color），与文件其余一处一行的格式不一致，降低可读性，建议拆为两行。

-   border-bottom-width: 1px;border-color: #eef1f5;
+   border-bottom-width: 1px;
+   border-color: #eef1f5;


─── apps/web/src/commercial/commercial-u.css:45-45 ───
[maintainability · low] #e7e7e7 在全局已有现成变量：本轮新增的 packages/design-tokens/src/tdesign-theme.css 定义了
--td-component-stroke / --td-gray-color-3（= #e7e7e7），design-tokens styles.css 亦有
--wk-color-component: #e7e7e7——文件注释本身也写明来源是
"var(--td-component-stroke)=#e7e7e7"。硬编码字面量绕过了变量体系：一旦后续启用 tdesign theme-mode 切换或调整 token，此处不会跟随，将与
tdesign 控件描边脱节。建议改为 var(--td-component-stroke, #e7e7e7)（保留字面量兜底，视觉零变化）。

- .wk-cs-card { border-radius: 8px; border: 1px solid #e7e7e7; background-color: #ffffff; padding: 16px; }
+ .wk-cs-card { border-radius: 8px; border: 1px solid var(--td-component-stroke, #e7e7e7); background-color: #ffffff; padding: 16px; }


─── apps/web/src/commercial/surface.tsx:9-13 ───
[maintainability · medium] 与既有共享兼容层重复，且 JSDoc 与实际值矛盾。仓库已有 apps/web/src/shared/wk-legacy.tsx 的
WkCard/WkStatus（.wk-card/.wk-status）：其中 .wk-status 的全部值（margin
0.25rem、13px、#506078、error/success/warning 色值 #b42318/#137333/#9a6700）与本域 .wk-cs-status
逐项相同，属纯重复；Card 仅边框色不同。更关键的是 JSDoc 声称"视觉 = 既有 .wk-card"，但既有 .wk-card（wk-legacy.css）边框为 #dce3ed，而
.wk-cs-card 为 #e7e7e7（td stroke），两者视觉并不相等——后续维护者按注释会去对齐错误的值。建议：Status 直接复用 shared WkStatus；Card
若保留域内实现，至少修正 JSDoc 为"视觉 = 旧栈 utilities（line/line-soft → td-component-stroke #e7e7e7），≠ .wk-card 的
#dce3ed"。



─── apps/web/src/commercial/surface.tsx:18-18 ───
[maintainability · low] 三层嵌套三元表达式违反项目检查清单（Ternary Expressions: Nested ternary expressions are not
allowed）。tone 到类名是确定性映射，可简化为模板字符串（与 shared WkStatus 的 `tone !== 'neutral' && \`wk-status--${tone}\``
同款写法）。另外 Status 未透传 className/id 等 HTMLAttributes，与上方 Card 的透传设计不对称，调用方无法追加样式或测试锚点，建议一并对齐。

-   const toneClass = tone === 'error' ? 'wk-cs-status--error' : tone === 'success' ? 'wk-cs-status--success' : tone === 'warning' ? 'wk-cs-status--warning' : '';
+   const toneClass = tone !== 'neutral' ? `wk-cs-status--${tone}` : '';


─── apps/web/src/configuration/ConfigurationPage.tsx:111-111 ───
[style · low] 本次已重写的该行内含 4 层嵌套三元链（errors ? … : loading ? … : agents ? … : empty ? … :
list），违反团队“禁止嵌套三元表达式”约定，且整段 JSX 近 4000 字符单行，排查渲染分支困难。建议拆为局部渲染函数（同文件已有
renderConfigurationRow/renderAgentGroups 先例），如 renderSectionBody(section, items)。

- {errors[section.key] ? <Status tone="error">{errors[section.key]}</Status> : loading ? <Status>Loading…</Status> : section.key === 'agents' && items.length > 0 ? renderAgentGroups() : items.length === 0 ? <Status>No configured entries.</Status> : <ul className="wk-list wk-cfg-page-5">{items.map((item, index) => renderConfigurationRow(section.key, item, index))}</ul>}
+ // 拆出局部函数，消除嵌套三元：
+ function sectionBody(section: ConfigurationSection, items: unknown[]) {
+   if (errors[section.key]) return <Status tone="error">{errors[section.key]}</Status>;
+   if (loading) return <Status>Loading…</Status>;
+   if (section.key === 'agents' && items.length > 0) return renderAgentGroups();
+   if (items.length === 0) return <Status>No configured entries.</Status>;
+   return <ul className="wk-list wk-cfg-page-5">{items.map((item, index) => renderConfigurationRow(section.key, item, index))}</ul>;
+ }


─── apps/web/src/configuration/config-u.css:44-49 ───
[bug · medium] `.wk-cfg-ops-4 input` / `.wk-cfg-ops-4 textarea` 是后代选择器（特异性 0,1,1），平移自旧 Tailwind
arbitrary variant。但旧栈 Input/Textarea 渲染的是原生元素，而现在该 form（ModelDebugPanel 与 SkillOperations 注册表单）内是
tdesign 控件，内部会渲染 `.t-input__inner` input、`.t-textarea__inner` textarea（tdesign 自身类特异性仅 0,1,0）。此规则会在
tdesign 自带描边容器（.t-input/.t-textarea 外框+内边距）之上，再给内部 input/textarea 叠加 #cbd5e1 边框、内边距与白底，造成双重描边和
padding 错位的视觉回归。ConfigurationEditor 的迁移做法是把通栏样式打到组件根
`className={fieldClass}`（.wk-cfg-field），不触达内部元素，建议对齐该模式（或用 :not() 排除 tdesign 内部类）。

- .wk-cfg-ops-4 input {
-   width: 100%;
-   box-sizing: border-box;
-   border-style: solid;
-   border-width: 1px;
-   border-color: #cbd5e1;
+ /* 建议改为组件根类，如 ConfigurationEditor 的 .wk-cfg-field： */
+ /* .wk-cfg-ops-4 .t-input, .wk-cfg-ops-4 .t-textarea { width: 100%; } */
+ /* 并移除对内部 input/textarea 的边框、内边距覆盖 */


─── apps/web/src/configuration/config-u.css:5-8 ───
[bug · medium] 本项目存在暗色模式机制：theme.ts 在 <html> 上切换 theme-mode，design-tokens 的
`:root[theme-mode="dark"]` 会翻转
--td-text-color-secondary、--td-component-stroke、--wk-color-text-secondary、--wk-color-surface
等变量。注释本身已指出取值来源是 var(--td-text-color-secondary)，却硬编码为定值——暗色下这些文本（rgba(0,0,0,.6) 深灰）落在暗背景上可读性受损，#fff
卡片/输入背景、#e7e7e7 边框也不会切换。文件内十余处同模式硬编码（含 .wk-cfg-card 的 #ffffff/#e7e7e7）建议统一替换为对应 CSS
变量，变量全局已定义，改动零成本。

  .wk-cfg-ops-1 {
-   /* 旧栈 muted/muted-strong→Vue var(--td-text-color-secondary)=rgba(0,0,0,.6) */
-   color: rgba(0, 0, 0, 0.6);
+   color: var(--td-text-color-secondary, rgba(0, 0, 0, 0.6));
  }


─── apps/web/src/configuration/config-u.css:262-268 ───
[maintainability · low] .wk-cfg-page-16 内 font-size（0.8rem → 0.85rem !important）和 color（#6941c6 →
rgba(0,0,0,.6) !important）各声明两次，前一组永远是死声明；.wk-cfg-ed-5 在 `display: flex !important` 容器上声明
grid-template-columns 同样无效。虽是忠实还原旧 utility 的层叠结果，但死声明会误导后续维护者以为 #6941c6/0.8rem 生效，建议只保留最终生效值。

-   font-size: 0.8rem;
-   color: #6941c6;
    background-color: #f4f3ff;
    margin-right: auto;
    font-size: 0.85rem !important;
    /* 旧栈 muted/muted-strong→Vue var(--td-text-color-secondary)=rgba(0,0,0,.6) */
    color: rgba(0, 0, 0, 0.6) !important;


─── apps/web/src/configuration/config-u.css:324-330 ───
[maintainability · low] 本文件内多组完全相同的规则按文件前缀复制：actions 行（.wk-cfg-ops-3 / .wk-cfg-page-4 /
.wk-cfg-ed-6）、列表重置（.wk-cfg-ops-9 / .wk-cfg-page-5 / .wk-cfg-ed-7 / .wk-cfg-mun-4）、列表行（.wk-cfg-ops-10
/ .wk-cfg-page-1 / .wk-cfg-ed-8 / .wk-cfg-mun-5）、panel heading（.wk-cfg-ops-6 / .wk-cfg-ed-1 /
.wk-cfg-mun-1）。同一视觉改动需要同步 3-4 处，建议在域内提取共享类（如 .wk-cfg-actions / .wk-cfg-list /
.wk-cfg-row）组合使用；ui.tsx 的 Card/Status 与其他域同期新增的本地封装同理，后续可上移到共享模块。

- .wk-cfg-ed-6 {
-   margin-bottom: 0.75rem;
-   display: flex;
-   align-items: center;
-   justify-content: flex-end;
-   gap: 0.5rem;
- }
+ /* 提取共享： */
+ /* .wk-cfg-actions { display: flex; align-items: center; justify-content: flex-end; gap: 0.5rem; margin-bottom: 0.75rem; } */


─── apps/web/src/data-sources/DataSourcesPage.tsx:452-452 ───
[bug · medium] 迁移到 TDesign Textarea 后，VUE_SETTINGS_FIELDS 分支丢失了旧代码的 `required={!field.optional}`，且
save() 的 JS 校验（name/type/schedule trim、firstMissingRequiredCredential、gitlab
project_id）未覆盖设置字段——VUE_SETTINGS_FIELDS 中唯一条目 rss 的 feed_urls（form.ts:187，无 optional
标记即必填）成为校验盲区：用户现在可以不填 feed_urls 直接保存 RSS 数据源（旧实现会被浏览器原生 required 拦截），只能依赖后端报错。建议在 save() 的凭据 walk
之后补一段设置字段校验，与既有 `${t(label)} ${t('dataSource.isRequired')}` 警告保持一致。

-       <fieldset className="wk-ds-27"><legend className="wk-ds-28">{t('dataSource.connectorSettingsLabel')}</legend>{form.type === 'gitlab' ? <div className="wk-ds-8" data-kind="gitlab-projects"><div className="wk-ds-30"><span className="wk-ds-31">{t('dataSource.gitlab.projects')}</span><Button type="button" theme="default" variant="text" size="small" onClick={() => updateForm('gitlabProjects', [...(form.gitlabProjects ?? []), { ...emptyGitLabProject }])}>{t('dataSource.gitlab.addProject')}</Button></div><small className="wk-ds-12">{t('dataSource.gitlab.projectsHint')}</small>{(form.gitlabProjects ?? []).map((project, index) => <div key={index} className="wk-ds-32" data-kind="gitlab-project-row"><div className="wk-ds-33"><strong className="wk-ds-34">{t('dataSource.gitlab.project')} {index + 1}</strong><Button type="button" theme="danger" variant="text" size="small" aria-label={t('common.delete')} onClick={() => updateForm('gitlabProjects', (form.gitlabProjects ?? []).filter((_, at) => at !== index))}>×</Button></div><label className="wk-ds-29"><span>{t('dataSource.gitlab.projectId')} *</span><Input placeholder={t('dataSource.gitlab.projectIdPlaceholder')} value={project.project_id} onChange={(value) => updateForm('gitlabProjects', (form.gitlabProjects ?? []).map((item, at) => at === index ? { ...item, project_id: String(value) } : item))} /></label><label className="wk-ds-29"><span>{t('dataSource.gitlab.ref')}</span><Input placeholder={t('dataSource.gitlab.refPlaceholder')} value={project.ref} onChange={(value) => updateForm('gitlabProjects', (form.gitlabProjects ?? []).map((item, at) => at === index ? { ...item, ref: String(value) } : item))} /></label><label className="wk-ds-29"><span>{t('dataSource.gitlab.paths')}</span><Textarea autosize={{ minRows: 2, maxRows: 2 }} placeholder={t('dataSource.gitlab.pathsPlaceholder')} value={project.pathsText} onChange={(value) => updateForm('gitlabProjects', (form.gitlabProjects ?? []).map((item, at) => at === index ? { ...item, pathsText: String(value) } : item))} /></label></div>)}</div> : VUE_SETTINGS_FIELDS[form.type] ? VUE_SETTINGS_FIELDS[form.type].map((field) => <label key={field.key} className="wk-ds-29"><span>{t(field.label)}</span><Textarea autosize={{ minRows: 3, maxRows: 3 }} placeholder={field.placeholder || t('dataSource.credential.inputPlaceholder')} value={credentialValue(form.settingsText, field.key)} onChange={(value) => updateForm('settingsText', setCredentialValue(form.settingsText, field.key, String(value)))} />{field.hint ? <small className="wk-ds-12">{t(field.hint)}</small> : null}</label>) : <Textarea autosize={{ minRows: 4, maxRows: 4 }} value={form.settingsText} onChange={(value) => updateForm('settingsText', String(value))} placeholder={t('dataSource.settingsPlaceholder')} />}</fieldset>
+ // save() 内、凭据 walk 之后补充：
+ const missingSetting = VUE_SETTINGS_FIELDS[form.type]?.find((field) => !field.optional && !credentialValue(form.settingsText, field.key));
+ if (missingSetting) {
+   setMessage({ tone: 'warning', text: `${t(missingSetting.label)} ${t('dataSource.isRequired')}` });
+   return;
+ }


─── apps/web/src/data-sources/DataSourcesPage.tsx:441-441 ───
[bug · medium] `wk-ds-self-start`（原 Tailwind justify-self-start）在 data-sources-u.css 中未定义，唯一定义位于
apps/web/src/knowledge/knowledge-u.css:698——而 knowledge-u.css 仅由懒加载路由
KnowledgeGraphPage（router.tsx:67 lazy）引入。未先访问知识图谱页时该类不会加载，prereq 控制台链接会按 grid item 默认 stretch
拉伸为整行。这与本域 CSS 底部注释自述已修复的 `.wk-dsui-card` "误置于懒加载 knowledge-u.css"（OCR R1-04/31）是同一类 bug，建议将定义一并迁入
data-sources-u.css。

-           {guide.permissionPageUrl ? <a className="wk-ds-15 wk-ds-self-start" href={guide.permissionPageUrl} target="_blank" rel="noopener">{prereqCopy(t, `dataSource.prereqOpenConsole_${form.type}`, 'dataSource.prereqOpenConsole')}<span aria-hidden="true">↗</span></a> : null}
+ /* data-sources-u.css 手工段新增： */
+ .wk-ds-self-start { justify-self: start; }


─── apps/web/src/data-sources/DataSourcesPage.tsx:484-484 ───
[maintainability · low] TDesign Checkbox 传入 label prop 时组件自身渲染 <label class="t-checkbox">
根元素，与这里手写的外层 <label className="wk-ds-21"> 形成嵌套 label（HTML 规范禁止 label 嵌套，影响可访问性语义）；上方 456 行的
`<label><Checkbox ... label={t('dataSource.syncDeletions')} /></label>` 是同一问题。label prop 已自带关联，外层请改用
<div>/<span>。

-       <label className="wk-ds-21"><Checkbox checked={deletePurge} disabled={deleteSubmitting} onChange={(checked) => setDeletePurge(checked)} label={purgeLabelText} /></label>
+       <div className="wk-ds-21"><Checkbox checked={deletePurge} disabled={deleteSubmitting} onChange={(checked) => setDeletePurge(checked)} label={purgeLabelText} /></div>


─── apps/web/src/data-sources/DataSourcesPage.tsx:475-477 ───
[bug · low] 删除提交进行中已禁用 cancel 按钮和 purge checkbox（disabled={deleteSubmitting}），但 TDesign Dialog 默认
closeOnEscKeydown=true 且右上角关闭图标始终可用：ESC/X 触发 onClose 会卸载弹窗，与刻意的禁用语义相矛盾（请求本身会继续完成，成败仅通过页面级 message
反馈）。建议提交期间同步关闭这两条通道，保持 affordance 一致。

      closeOnOverlayClick={false}
+     closeOnEscKeydown={!deleteSubmitting}
+     closeBtn={!deleteSubmitting}
      width={440}
      onClose={() => setDeleteSource(null)}


─── apps/web/src/data-sources/DataSourcesPage.tsx:451-451 ───
[style · low] 迁移残留：cancel-replace 按钮的 className="" 为空串死属性（旧实现此处携带 justify-self-start utility），建议删除。

-       {editing && credentialStep.replaceMode ? <Button type="button" theme="default" variant="text" size="small" className="" onClick={() => { setCredentialStep((current) => credentialStepReducer(current, 'cancel-replace')); setForm((current) => ({ ...current, credentialsText: '', authHeaders: [] })); }}>{t('common.cancel')}</Button> : null}
+       {editing && credentialStep.replaceMode ? <Button type="button" theme="default" variant="text" size="small" onClick={() => { setCredentialStep((current) => credentialStepReducer(current, 'cancel-replace')); setForm((current) => ({ ...current, credentialsText: '', authHeaders: [] })); }}>{t('common.cancel')}</Button> : null}


─── apps/web/src/data-sources/DataSourcesPage.tsx:481-481 ───
[bug · medium] 删除提交失败时反馈不可见：confirmDelete 的 catch 分支只 setMessage(...)、不关闭弹窗（deleteSource 保持非空），而错误
Status 渲染在页面顶部 Card 内——迁移为居中 TDesign Dialog（默认全屏 mask + closeOnOverlayClick=false）后，该 message
完全位于遮罩之下，用户在弹窗内只能看到确认按钮 loading 结束、界面"毫无反应"。旧实现（右置 Sheet、无全屏遮罩）下页面级 message 仍部分可见；且对齐的 Vue 版用
MessagePlugin 全局 toast（浮于遮罩之上），本移植的反馈通道实际被截断。仅成功路径（setDeleteSource(null) 后 message 可见）不受影响。建议：失败时在弹窗
body 内渲染错误文案（局部 error 状态），或失败时同样关闭弹窗让页面 message 可见。

      <div className="wk-ds-35" data-kind="delete-panel">
+       {deleteError ? <p className="wk-ds-37" role="alert">{deleteError}</p> : null}


─── apps/web/src/data-sources/data-sources-u.css:26-30 ───
[maintainability · low] 存在多处被后续声明覆盖的死代码（不影响渲染但降低可维护性）：(1) .wk-ds-2/.wk-ds-42/.wk-ds-77 中
transition-duration: 150ms 随即被 200ms 覆盖；(2) .wk-ds-25/.wk-ds-42/.wk-ds-68 中 border-style: solid 被
dashed 覆盖；(3) .wk-ds-58/.wk-ds-71 出现 `border-bottom-width: 1px;border-color: #e7e7e7;`
两条声明挤压单行的格式异常。建议统一清理为仅保留生效值。

    transition-property: border-color,box-shadow;
    transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
-   transition-duration: 150ms;
    transition-duration: 200ms;
  }


─── apps/web/src/embed/EmbedEntryPage.tsx:826-826 ───
[bug · medium] underline-offset-2 是本次 Tailwind utilities 平移的遗漏残留：apps/web 已无任何 Tailwind
入口（vite.config.ts 与 styles.css 均无 tailwindcss 引用），该 utility 类不会再生成，属于死类名；同时 .wk-emb-34 也未包含对应的
text-underline-offset: 2px，导致引用链接 hover 下划线偏移这一生效值静默丢失，与文件其余部分 1:1 平移的做法不一致。建议删掉该残留类名，并将 offset 值并入
.wk-emb-34。

-             className="underline-offset-2 wk-emb-34"
+             className="wk-emb-34"


─── apps/web/src/embed/embed-u.css:315-318 ───
[bug · low] 原 utility 为 break-all，其生效值是 word-break: break-all（任意字符处可断行）；平移写成 overflow-wrap: anywhere
语义不等价（仅在溢出时断行，且参与 min-content 尺寸计算），与文件头注释"值 = utilities 编码的生效值"的承诺不符，长
URL/无空格引用标题在窄容器中的断行表现可能改变。.wk-emb-35 存在同样问题，建议一并修正为 word-break: break-all。

  .wk-emb-34 {
-   overflow-wrap: anywhere;
+   word-break: break-all;
    color: var(--embed-primary,#2563eb);
  }


─── apps/web/src/embed/embed-u.css:337-337 ───
[maintainability · low] sr-only 以已废弃的 clip: rect(0,0,0,0) 单独实现，未附带回退更稳妥的 clip-path:
inset(50%)（Tailwind v4 / Bootstrap 现行惯例）。clip
属性虽未被浏览器移除，但在逐步弃用过程中一旦引擎收紧解析，屏幕阅读器隐藏将失效（文本意外可见）。建议按现代惯例补写 clip-path 声明。另请顺手整理 .wk-emb-3/.wk-emb-31 中
"border-bottom-width: 1px;border-color: ..." 的同行双声明书写瑕疵。

    clip: rect(0,0,0,0);
+   clip-path: inset(50%);


─── apps/web/src/embed/embed-u.css:1-2 ───
[documentation · low] 文件头注释声明"导入顺序：须在本域 .td.css 之前"，但 embed 域（apps/web/src/embed/）并不存在 .td.css
文件，实际需要保证的后置文件是 embed-chat.css 与 katex.min.css（见 EmbedEntryPage.tsx
中的导入顺序与注释）。该头注释应为从其他域（agents/settings 等确有 .td.css 的目录）复制模板后未修改的残留，描述了不存在的前提，会误导后续维护者按图索骥寻找或新建
embed.td.css。建议将头注释修正为与本域实际文件一致，例如"导入顺序：须在 embed-chat.css / katex 之前"。



─── apps/web/src/experts/ExpertsPage.tsx:46-47 ───
[maintainability · medium] 新增的跨域导入未发现任何消费点：views-chat-u.css 的规则前缀全部为 wk-vc-*(其文件头注明"由各直接消费方
import"),而本页类名仅有 wk-page(无基础样式的钩子,壳层覆盖在 platform-u.css)、wk-exp-*(本域 experts-u.css)与
wk-experts-persona(全仓库无 CSS 定义,纯测试钩子);renderChatMarkdown 注入的 wk-chat-citation/wk-chat-image 类也不在
views-chat-u.css 中定义。若确有依赖(如共享平移层的顺序约束),请补充注释说明依赖点;否则建议移除该导入,避免约 2500 行无关 chat 样式进入本页依赖图,防止 chat
域重构时误伤 experts 页面。



─── apps/web/src/experts/experts-u.css:46-48 ───
[maintainability · medium] 主题令牌被硬编码为字面量,与原语义和项目既有模式脱钩:迁移前
text-accent/border-accent/bg-surface/bg-accent-wash 等 utilities 编译产物是
var(--color-accent)/var(--color-surface)/var(--color-accent-wash) 的运行时引用;本文件将其写死为
#07c05f/#ffffff/rgba(7,192,95,0.08)(涉及 .wk-exp-xp-tab、.wk-exp-xp-card 系、三个
btn、input/textarea、.wk-exp-48/.wk-exp-49 等约 20 处),而同批/既有 CSS(settings-wrapper.css 的
var(--color-accent,#07c05f)、integrations.td.css 与 kb-list.td.css 的 var(--color-surface,#ffffff))均保留
var() 引用,本文件内 .wk-exp-8/.wk-exp-22/.wk-exp-26/.wk-exp-36 也仍用 var(--wk-bg,#fff)。当前
--color-surface/--color-accent 在 @theme 中为静态值故视觉暂无差异,但后续调整品牌色或令牌运行时覆盖时 experts
域将不跟随,且同页两种来源不一致。建议统一改为 var(--color-accent, #07c05f) / var(--color-surface, #ffffff) /
var(--color-accent-wash, rgba(7,192,95,0.08)) 形式。

    border-color: #e7e7ea;
-   background-color: #ffffff;
+   background-color: var(--color-surface, #ffffff);
    padding-inline: 14px;


─── apps/web/src/experts/experts-u.css:120-125 ───
[maintainability · low] 近重复规则堆积:.wk-exp-xp-card 与 .wk-exp-xp-card-static 仅差 cursor:pointer 与
transition/hover 态;.wk-exp-2 与 .wk-exp-15 仅差 margin-top:auto;.wk-exp-9/.wk-exp-22/.wk-exp-34
的边框+padding 容器几乎一致(border-bottom/top
方向不同)。机械平移可以理解,但后续统一调整间距/边框色时需多处同步、易漏改造成视觉漂移,建议合并公共部分为共享选择器,差异部分单独声明。

+ .wk-exp-xp-card,
  .wk-exp-xp-card-static {
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    gap: 10px;
    border-radius: 10px;
+ /* …公共声明合并,差异项(cursor/transition/hover)留在 .wk-exp-xp-card 单独规则 */


─── apps/web/src/experts/experts-u.css:420-421 ───
[style · low] 格式瑕疵:.wk-exp-9 中 "border-bottom-width: 1px;border-color: rgba(127,127,127,0.2);"
两条声明黏连在一行(.wk-exp-22/.wk-exp-34 的 border-top-width 行同病);此外全文件大量规则照搬编译产物重复声明 border-style:
solid。不影响功能,仅影响可读性,建议按一声明一行整理。

    border-bottom-style: solid;
-   border-bottom-width: 1px;border-color: rgba(127,127,127,0.2);
+   border-bottom-width: 1px;
+   border-color: rgba(127,127,127,0.2);


─── apps/web/src/faq/FAQPage.tsx:2481-2481 ───
[bug · medium] 批量删除失败时仍会清空选择集：`removeMany` 内部 try/catch 吞掉异常后正常返回（L2337），因此这里的 `.then(() =>
setSelected(new Set()))` 无论成败都会执行。删除失败时用户收到错误 toast 的同时勾选被清空，需要重新逐个勾选才能重试。建议让 `removeMany`
返回成功与否（或重新抛出），仅在成功时清空选择。

-       onBatchDelete={() => void removeMany([...selected]).then(() => setSelected(new Set()))}
+ onBatchDelete={() => void removeMany([...selected]).then((ok) => { if (ok) setSelected(new Set()); })}
+ // removeMany 改为：async function removeMany(ids: number[]): Promise<boolean> { ... try { ...; return true; } catch { ...; return false; } finally { setBatchDeleteLoading(false); } }


─── apps/web/src/faq/FAQPage.tsx:1519-1519 ───
[maintainability · medium] 答案计数硬编码为 `/5`，与同文件禁用条件使用的 `FAQ_ANSWER_CAP`
常量脱钩（旧代码用的是常量）。未来调整答案上限时，此处的展示计数会与实际校验不一致。

-                       <div className="item-count">{form.answers.length}/5</div>
+ <div className="item-count">{form.answers.length}/{FAQ_ANSWER_CAP}</div>


─── apps/web/src/faq/FAQPage.tsx:2177-2181 ───
[maintainability · low] 提示展示职责已迁移至本处 MessagePlugin toast（视图内原 `<Status>` 内联块已全部移除），但 `FAQPageView`
仍声明并解构 `message` prop、`FAQPage` 也仍传入 `message={message}`，该 prop 在视图中已无任何消费点，成为死属性。建议同步删除视图侧的
`message` prop 声明/解构及页面的传参，避免后续误以为页面仍有内联错误展示区。

    useEffect(() => {
      if (!message) return;
      const theme = message.tone === 'error' ? 'error' : message.tone === 'success' ? 'success' : 'warning';
      MessagePlugin[theme](message.text);
    }, [message]);
+ // 同时移除 FAQPageView 的 message prop 与 FAQPage 的 message={message} 传参


─── apps/web/src/faq/FAQPage.tsx:956-960 ───
[style · low] 三层嵌套三元表达式（外层 length===0 / ===1 / 多选，内层再嵌 tagFilterCleared 与 find
回退判断），违反项目「禁止嵌套三元」规范，可读性差。建议提取为具名辅助函数（如 computeActiveTagFilterLabel），用提前返回逐层拆解。

-   const activeTagFilterLabel = activeTagIds.length === 0
-     ? (tagFilterCleared ? t('knowledgeBase.tagFilterPlaceholder') : t('knowledgeBase.allTags'))
-     : activeTagIds.length === 1
-       ? (tags.find((tag) => tag.id === activeTagIds[0])?.name ?? t('knowledgeBase.allTags'))
-       : t('knowledgeBase.tagFilterMulti', { count: activeTagIds.length });
+ function computeActiveTagFilterLabel(activeTagIds: readonly number[], tagFilterCleared: boolean, tags: readonly KnowledgeTag[], t: Translate): string {
+   if (activeTagIds.length === 0) return tagFilterCleared ? t('knowledgeBase.tagFilterPlaceholder') : t('knowledgeBase.allTags');
+   if (activeTagIds.length === 1) return tags.find((tag) => tag.id === activeTagIds[0])?.name ?? t('knowledgeBase.allTags');
+   return t('knowledgeBase.tagFilterMulti', { count: activeTagIds.length });
+ }


─── apps/web/src/faq/FAQPage.tsx:1561-1565 ───
[maintainability · low] 此段关闭按钮内联 SVG 在导入弹窗与批量标签弹窗（batch-tag-close-btn）逐字重复了两份，且旧 CloseIcon
组件删除后未沉淀共享实现。文件已引入 tdesign-icons-react，可直接使用 `<TIcon name="close" />`，或提取一个共享的 CloseIconButton
组件；同时注意此处文案用的是 `general.close` 而文件其余关闭按钮均为 `common.close`，建议统一。

-               <button className="close-btn" aria-label={t('general.close')} onClick={() => onCloseImport()}>
-                 <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor">
-                   <path d="M15 5L5 15M5 5L15 15" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
-                 </svg>
+ <button className="close-btn" aria-label={t('common.close')} onClick={() => onCloseImport()}>
+   <TIcon name="close" size="20px" />
-               </button>
+ </button>


─── apps/web/src/faq/FAQPage.tsx:989-991 ───
[maintainability · low] `case 'export'` 分支不可达：`faqExportOptions` 只产出 `export_csv` / `export_json`
两个值，`faqCreateOptions` 产出 `create` / `import`，搜索入口传 `'search'`，没有任何调用路径会产生 `'export'`。属于死分支，建议删除。

        case 'export_csv': onExport('csv'); break;
        case 'export_json': onExport('json'); break;
-       case 'export': onExport('csv'); break;


─── apps/web/src/faq/FAQPage.tsx:1127-1135 ───
[maintainability · low] 本次 TDesign 平移存在可访问性/键盘退化，建议补回语义：(1) 此清除按钮由原 button+tabIndex+键盘事件退化为纯 span
onClick，键盘用户无法触发；(2) 自定义弹层 faq-import-overlay / batch-tag-overlay 丢失了原 role="dialog" / aria-modal /
aria-label；(3) popup-menu-item 与检索结果 result-header 由 button 降级为 div onClick，无键盘可达性；(4) FaqTagTooltip
气泡移除了 role="tooltip"，且删除了点击切换（触屏设备 hover 不可用，无法查看完整标签内容）。

                            {showTagFilterClear ? (
                              <span
+                               role="button"
+                               tabIndex={0}
                                className="t-input__suffix t-input__suffix-icon t-input__clear"
                                aria-label={t('common.clear')}
                                onClick={(event) => { event.stopPropagation(); setTagPanelOpen(false); setTagFilterCleared(true); onClearTagFilter(); }}
+                               onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); event.stopPropagation(); setTagPanelOpen(false); setTagFilterCleared(true); onClearTagFilter(); } }}
                                onMouseDown={(event) => event.stopPropagation()}
                              >
-                               <TIcon name="close-circle-filled" className="t-input__suffix-clear" />
-                             </span>


─── apps/web/src/faq/FAQPage.tsx:2322-2323 ───
[maintainability · low] `batchTagBusy` 与 `batchTagLoading` 两个状态始终同步置位/复位（成对 true、finally 中成对
false），语义完全重叠（分别供弹窗确认按钮与批量栏按钮的 loading 使用），属冗余状态。建议合并为单个状态，避免未来只改一处造成两处 loading 表现不一致。

      setBatchTagBusy(true);
-     setBatchTagLoading(true);
+     // 删除 batchTagLoading 状态，视图侧统一消费 batchTagBusy


─── apps/web/src/faq/FAQPage.tsx:1659-1659 ───
[bug · high] 导入对话框主按钮的文案分支与实际行为错配：confirmImport 成功后立即 setImportOpen(false)（L2386）并把 importTask 留存，而
onOpenImport（L2489）重开对话框时并不重置 importTask。于是上一次导入任务为 success 时再开对话框：主按钮显示「关闭」，onClick 却执行
confirmImport——用户已选新文件时会非预期地再次导入（replace 模式下会清空替换现有数据），未选文件则报「请先选择文件」警告，两种结果都不是「关闭」；failed
态显示「重试」，但重试的是当前新选文件而非旧任务。由于对话框打开期间 importTask 只可能是陈旧状态，这三个文案分支没有正确语义。建议在 onOpenImport 时重置
importTask（setImportTask(null)），或移除按钮上的 importTask 文案/disabled 分支。

-                     {importTask?.status === 'success' ? t('common.close') : importTask?.status === 'failed' ? t('common.retry') : t('knowledgeEditor.faqImport.importButton')}
+ onOpenImport={() => { setImportFile(null); setImportPreview([]); setImportTask(null); setImportOpen(true); }}


─── apps/web/src/faq/FAQPage.tsx:2351-2354 ───
[bug · medium] t('common.operationFailed') 引用了不存在的 i18n key：apps/web 的翻译目录由 knowledgeSurfaces /
knowledgeEditorMessages / settings / chatCopy 四个 catalog 合并，其中均无
common.operationFailed（@weknora/i18n 全量 messages 亦无）。formatMessage 的回退链是 messages[locale][key] ??
messages['en-US'][key] ?? key，因此单卡片改标签失败（此处，新增代码）与检索测试失败（L2414 runSearchTest）时，用户 toast 将直接显示原文
"common.operationFailed"。建议改用已存在的 key（如 common.error 或 settings catalog 中的通用操作失败文案），或补充该
key。另注：函数注释声称「+ 回滚」，但实现中 catch 后没有任何回滚逻辑（Vue 侧有回滚），注释与行为不符，建议一并修正。

      } catch (error) {
-       setMessage({ tone: 'error', text: error instanceof Error && error.message ? error.message : t('common.operationFailed') });
+       setMessage({ tone: 'error', text: error instanceof Error && error.message ? error.message : t('common.error') });
      }
    }


─── apps/web/src/faq/FAQPage.tsx:521-521 ───
[bug · low] kb-switcher 空态兜底是不可达的死分支，且其文案 key 缺失：该行位于外层 `kbList.length ? (...) : (...)` 的真分支内，而
sortedKbList = kbList 为空时必然与 kbList 同为空数组（sortKbListForSwitcher 对空输入返回空副本），`!sortedKbList.length`
在此分支恒为 false，kb-switcher-empty 永不渲染。此外 common.noData 在全部合并 catalog 中不存在（仅有无关的
wikiBrowser.graphNoData），一旦该分支被激活，用户会看到原文 key。建议直接删除该行；若确需空态，key 应换成已有的空态文案。



─── apps/web/src/faq/FAQPage.tsx:288-288 ───
[bug · low] FaqTagTooltip 气泡内容丢失换行保留：旧实现为 tooltip-content 显式加了 whitespace-pre-wrap（且气泡
word-break:break-word），本次平移后 faq.td.css 中没有任何 .tooltip-content 规则，多行 FAQ
答案/相似问在悬停气泡内的换行符会被折叠成一行连续文本，与 Vue 源及旧 React 实现不一致（现有测试仅断言 textContent，无法拦住此回归）。建议在 §B
弹层段补上：.faq-tag-tooltip .tooltip-content { white-space: pre-wrap; word-break: break-word; }

-           <div className="tooltip-content">{content}</div>
+ .faq-tag-tooltip .tooltip-content {
+   white-space: pre-wrap;
+   word-break: break-word;
+ }


─── apps/web/src/faq/FAQPage.tsx:1546-1546 ───
[style · low] 两处 TdSelect onChange 使用宽松相等 `value == null`（编辑器标签选择与批量标签选择各一处），违反项目「禁止 == /
!=」规则。Tdesign 的 onChange value 类型为 number | undefined，建议改为严格相等：value === undefined（或 value === null
|| value === undefined 以兼容清空时的两种空值）。此处也是嵌套三元（== null || === '' ? ... : ...），可顺势拆成小函数提升可读性。

- onChange={(value) => onFormChange({ tagId: value == null || value === '' ? '' : String(value) })}
+ onChange={(value) => onFormChange({ tagId: value === null || value === undefined || value === '' ? '' : String(value) })}


─── apps/web/src/faq/faq.td.css:2240-2244 ───
[bug · medium] `.answer-tag` 用 `var(--td-brand-color)1a` / `var(--td-brand-color)33` 的字符串拼接生成 8 位
hex 透明色：当前 token 恰为 6 位 hex（#07c05f）时可拼接成功，但一旦 token 改为
rgb()/oklch()/命名色，这两条声明会在计算值阶段静默失效，检索结果答案标签底色/边框回退为 TDesign 默认值且无任何报错。建议改用 color-mix() 或显式 hex 常量。

  .answer-tag {
-   background: var(--td-brand-color)1a;
+   background: color-mix(in srgb, var(--td-brand-color) 10%, transparent);
    color: var(--td-brand-color);
-   border-color: var(--td-brand-color)33;
+   border-color: color-mix(in srgb, var(--td-brand-color) 20%, transparent);
  }


─── apps/web/src/faq/faq.td.css:43-51 ───
[maintainability · low] 这批 Vue 过渡类与动画是 React 端永远不会命中的死代码：React 组件不会生成
`.fade-enter-active`/`.fade-leave-to`、`.modal-enter-from`/`.modal-leave-to`、`.slide-down-*`、`.faq-ba
tch-bar-fade-*` 等 Vue transition 类名（本页弹层/抽屉均由 TDesign 组件或条件渲染实现，无对应的 transition class
挂载点），`dropdownSlideInUp` keyframes 也无引用（仅 `dropdownSlideIn` 被 `.card-more-popup` 使用）。建议删除这些不可达段落，降低
3300+ 行样式文件的维护噪音。

- .fade-enter-active,
- .fade-leave-active {
-   transition: opacity 0.15s ease;
- }
- 
- .fade-enter-from,
- .fade-leave-to {
-   opacity: 0;
- }
+ /* 删除 .fade-*、.modal-enter/leave-*、.slide-down-*、.faq-batch-bar-fade-* 及 @keyframes dropdownSlideInUp（React 端无对应类名生成点） */


─── apps/web/src/faq/faq.td.css:1079-1080 ───
[maintainability · low] 本文件存在一组 TSX 永远不会生成匹配 DOM 的死选择器，建议清理或修正：(1) `.faq-manager .question-tag
.t-tag` 及其子规则——question-tag 类是直接挂在 TdTag 根元素上（<TdTag className="question-tag">），不存在 .question-tag 包裹
.t-tag 的结构；(2) `.batch-tag-form .t-form-item` / `.batch-tag-form .t-form-item__label`——TSX 实际渲染
batch-tag-form-item / batch-tag-form-label 自定义类，标签的字号/字重/间距样式因此全部未生效；(3) `.status-item`（非 compact
版）、`.faq-header-meta`、`.faq-meta-item`、`.tag-menu`、`.tag-menu-item`、`.tag-filter-bar`、`.empty-tip`、`
.form-tip`、`.match-type-tag`、`.tag-load-more`、`.kb-info-card-ext`、`.kb-info-card-hint` 在 FAQPage.tsx
中均无对应类名使用。



─── apps/web/src/faq/faq.td.css:2670-2684 ───
[maintainability · low] 存在连续重复的同名选择器块，应合并为一处：`.faq-editor-drawer .full-width-input-wrapper
.add-item-btn` 连续声明两次（第二块把第一块的 border-radius 覆写为 !important 8px）；`.faq-search-drawer .question-tag`
也连续出现两块（第二块补 background/color）。分散的重复声明容易在后续修改时只改其一造成样式漂移，建议逐组合并。



─── apps/web/src/knowledge-bases/KnowledgeBaseShareDialog.tsx:0-0 ───
[bug · high] 残留 Tailwind 工具类 `inset-x-0` 未收编：本次 S7 漂零收编后该类不再有样式来源，kb-u.css 的 `.wk-kbs-4` 只平移了
position/top/z-index 等，未包含 inset-x-0 对应的 `left: 0; right: 0;`。组织选择下拉 listbox
将丢失水平定位（原语义：与触发器左右撑满对齐），在收编完成后成为实际布局回归。建议在 kb-u.css 的 `.wk-kbs-4` 中补 `left: 0; right: 0;` 并移除该残留类名。

- {open ? <div id="wk-share-org-picker-list" className="inset-x-0 wk-kbs-4" role="listbox" aria-label={placeholder}>
+ {open ? <div id="wk-share-org-picker-list" className="wk-kbs-4" role="listbox" aria-label={placeholder}>
+ 
+ /* kb-u.css 中 .wk-kbs-4 需补充：
+ .wk-kbs-4 {
+   position: absolute;
+   left: 0;      /* 原 inset-x-0 */
+   right: 0;     /* 原 inset-x-0 */
+   top: calc(100% + 0.25rem);
+   ...
+ } */


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:585-597 ───
[maintainability · medium] KbCard 头部 `isSpaceCard` 与 `isSharedCard` 两个分支渲染完全相同的 JSX（Tooltip +
info-circle 详情触发，逐字重复 13 行），既违反重复代码规则又构成嵌套三元。两分支注释来源不同（Vue :595-600 与 :311-316）但 DOM 完全一致，建议合并为
`isSpaceCard || isSharedCard` 单一分支。

-         ) : isSharedCard ? (
-           /* Vue :311-316：「全部」视图共享卡片的详情触发替代三点菜单。 */
+         ) : isSpaceCard || isSharedCard ? (
+           /* Vue :311-316 / :595-600：空间视图与「全部」视图的共享卡片都以
+              info-circle 触发详情（替代三点菜单，React 数据面无 is_mine 分叉）。 */
            <Tooltip content={t('knowledgeList.menu.viewDetails')} placement="top">
              <button
                type="button"
                className="shared-detail-trigger"
                aria-label={t('knowledgeList.menu.viewDetails')}
                onClick={(event) => { event.stopPropagation(); onOpenDetail(card); }}
              >
                <TIcon name="info-circle" size="16px" />
              </button>
            </Tooltip>
          ) : (


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:51-55 ───
[maintainability · low] 注释与实际不符 + 同库双 import 别名混用：注释声称「旧栈组件仅供编辑器留守段继续使用」，但其后 import 的全部是
tdesign-react（文件中已无任何 @weknora/ui 引用），描述的"旧栈"不存在，误导读者。且同一组件库拆成两条 import，`Checkbox/Input/Textarea` 与
`TCheckbox/TInput/TTextarea` 两种别名前缀并存（同文件里 Button/Dialog/RadioGroup 又无 T 前缀），增加维护噪音。建议合并为单条 import
并统一别名约定（如全部加 T 前缀或全部不加），同步改写注释为留守段待迁移说明。

- /* 旧栈组件仅供编辑器留守段（models / vectorStore / parser / storage / multimodal /
-  * asr / advanced 段内部）继续使用——这些段待 knowledge-settings 批次迁移，本页
-  * 未新增任何旧栈用法（playbook §4.2 留守例外，见文件头边界说明）。 */
- // S6 抽屉收编：kb 编辑器深设置留守段离开 packages/ui 旧栈（T15 硬前置），换 tdesign。
- import { Checkbox as TCheckbox, Input as TInput, Select as TSelect, Textarea as TTextarea } from 'tdesign-react';
+ // S6 抽屉收编：kb 编辑器深设置留守段（models / vectorStore / parser / storage /
+ // multimodal / asr / advanced）离开 packages/ui 旧栈（T15 硬前置），换 tdesign。
+ // 这些段待 knowledge-settings 批次（R8 批次 3）同构迁移。
+ import {
+   Button as TButton,
+   Checkbox as TCheckbox,
+   Dialog as TDialog,
+   Input as TInput,
+   Select as TSelect,
+   Textarea as TTextarea,
+   // ...其余组件统一别名
+ } from 'tdesign-react';


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:0-0 ───
[bug · low] questionCount 输入转换无数值校验：`Number(String(value))` 在清空输入或键入非数字时产生 NaN，状态被污染（输入框回显
"NaN"，JSON 序列化变 null）。虽然 editor-config 保存管道有 `|| 3` 兜底不至于污染服务端数据，但 UI 状态已脏且不符合数值输入的基本健壮性。建议用
Number.isFinite 清洗或回退原值。

- <TInput value={String(editorConfig.questionGenerationConfig.questionCount)} onChange={(value) => setEditorConfig((current) => ({ ...current, questionGenerationConfig: { ...current.questionGenerationConfig, questionCount: Number(String(value)) } }))} />
+ <TInput value={String(editorConfig.questionGenerationConfig.questionCount)} onChange={(value) => { const parsed = Number(String(value)); setEditorConfig((current) => ({ ...current, questionGenerationConfig: { ...current.questionGenerationConfig, questionCount: Number.isFinite(parsed) ? parsed : current.questionGenerationConfig.questionCount } })); }} />


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:1748-1748 ───
[style · low] 嵌套三元表达式（禁用项）：granularity hint 的 i18n key 在模板串内拼双层三元，且 `extractionGranularity`
被重复判空取值三次，可读性差且 key 拼错难以发现。建议在组件体或渲染前提取 key 映射。

-                               <p className="form-tip granularity-hint">{t(`knowledgeEditor.wiki.granularity${(editorConfig.wikiConfig?.extractionGranularity ?? 'standard') === 'focused' ? 'Focused' : (editorConfig.wikiConfig?.extractionGranularity ?? 'standard') === 'exhaustive' ? 'Exhaustive' : 'Standard'}Hint`)}</p>
+ /* 渲染前提取（组件体顶部）：
+ const GRANULARITY_HINT_KEY = { focused: 'Focused', standard: 'Standard', exhaustive: 'Exhaustive' } as const;
+ const hintKey = GRANULARITY_HINT_KEY[editorConfig.wikiConfig?.extractionGranularity ?? 'standard']; */
+ <p className="form-tip granularity-hint">{t(`knowledgeEditor.wiki.granularity${hintKey}Hint`)}</p>


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:217-217 ───
[maintainability · medium] 组件重复实现：本文件新增并导出的 SpaceAvatar 已是代码库第 4
份实现（organizations/SpaceAvatar.tsx、AgentsPage.tsx、OrganizationsPage.tsx 各有一份），且本页编辑器 share 段经
KnowledgeBaseShareDialog 仍 import organizations/SpaceAvatar.tsx 的另一套——同一页面 rail 用本渐变版、share
对话框用旧版，两套头像视觉与行为并行漂移。OrgGreenIcon 同样在本文件与 SharedKnowledgeBaseDrawer.tsx 各持一份内联 SVG。建议抽公共模块（如 shared/
下的空间头像组件）收敛，本页与 share 对话框统一引用。

- export function SpaceAvatar({ name, size = 'medium', className }: { name: string; size?: 'small' | 'medium' | 'large'; className?: string }) {
+ // 建议抽取到共享模块（如 apps/web/src/shared/SpaceAvatar.tsx），
+ // KnowledgeBasesPage / KnowledgeBaseShareDialog / AgentsPage / OrganizationsPage 统一引用：
+ import { SpaceAvatar } from '../shared/SpaceAvatar.tsx';


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:245-249 ───
[maintainability · low] 业务文案硬编码绕过 i18n（注释已自述为 gap，已验证 packages/i18n 确无 resourceOrigin.*
键）：ORIGIN_TEXT 直引 Vue 中英文案，新增语言时 ResourceOriginBadge 将静默回落中文默认值；下方 text 的计算（256-260
行）同时也是嵌套三元。建议本批次补齐 i18n 键走 t()，text 计算改映射对象或早返回。

- const ORIGIN_TEXT: Record<'mine' | 'tenant' | 'creator', Partial<Record<Locale, string>>> = {
-   mine: { 'zh-CN': '我创建', 'en-US': 'Created by me' },
-   tenant: { 'zh-CN': '本空间', 'en-US': 'This space' },
-   creator: { 'zh-CN': '本空间', 'en-US': 'This space' },
- };
+ /* packages/i18n 补键：
+ resourceOrigin.mine = 我创建 / Created by me
+ resourceOrigin.tenant = 本空间 / This space */
+ const text = variant === 'mine'
+   ? t('resourceOrigin.mine')
+   : variant === 'creator' ? (creatorName || t('resourceOrigin.tenant'))
+     : t('resourceOrigin.tenant');


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:822-823 ───
[maintainability · medium] 死状态 + 冗余请求：`editorActivity` / `editorActivityLoading`（822-823 行）与
`loadEditorActivity`（1111-1120 行）从未被渲染读取——activity 段实际渲染 `KnowledgeBaseActivityPanel`，该组件自带数据加载（内部
load/entries 状态）。结果是：① 两个 state 为纯死代码；② 每次点击 activity 导航项（1650 行 `if (section === 'activity' &&
editingId) void loadEditorActivity(editingId)`）都会发起一次与面板内部加载完全重复的 `settings.activity` API
请求，且响应写入死状态后被丢弃。建议删除这三个符号及 nav onClick 中的调用。

-   const [editorActivity, setEditorActivity] = useState<Array<{ id: number; action: string; outcome: string; created_at: string }>>([]);
-   const [editorActivityLoading, setEditorActivityLoading] = useState(false);
+ // 删除 822-823 行两个 useState、1111-1120 行 loadEditorActivity，
+ // 以及 nav onClick 中的：
+ //   if (section === 'activity' && editingId) void loadEditorActivity(editingId);
+ // activity 段由 <KnowledgeBaseActivityPanel> 自行加载数据。


─── apps/web/src/knowledge-bases/SharedKnowledgeBaseDrawer.tsx:101-102 ───
[other · medium] 键盘可达性回退：由 @weknora/ui Sheet 改为自绘 overlay + createPortal 后，丢失 Escape 关闭、焦点圈定与归还（Tab
可穿透到底层页面）、焦点初始落点等行为，关闭仅剩鼠标点遮罩与右上角按钮。项目内弹层基线（shared/wk-legacy.tsx 的 WkDialog/WkSheet）均保留 ESC/焦点语义，且
98 行注释声称复刻 Vue Transition，但 enter/leave 过渡类（.shared-detail-drawer-enter*）从未被添加，抽屉瞬时出现/消失。建议补 Escape
监听与焦点管理（或复用 WkSheet），并接线过渡类。

+   // 组件体内补充（需 import { useEffect } from 'react'）：
+   useEffect(() => {
+     if (!visible) return;
+     const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
+     window.addEventListener('keydown', onKey);
+     return () => window.removeEventListener('keydown', onKey);
+   }, [visible, onClose]);
+ 
    return createPortal(
      <div className="shared-detail-drawer-overlay" onClick={(event) => { if (event.target === event.currentTarget) onClose(); }}>


─── apps/web/src/knowledge-bases/SharedKnowledgeBaseDrawer.tsx:54-58 ───
[maintainability · low] 死代码：permissionTone 兼容导出在全库（含测试）中已无任何引用——本次重写后组件内部唯一消费的是
permissionTheme。兼容层若无人使用即应删除，避免后续读者误以为存在外部依赖。



─── apps/web/src/knowledge-bases/kb-editor-parity.css:21-24 ───
[bug · medium] 留守段锚点规则失配：留守段（models/vectorStore/parser/storage/multimodal/asr/advanced）的 label 已随 S7
收编改用 wk-kbl-4 / wk-kbl-12 等类（已验证文件内无任何 `label ... grid` 的 Tailwind 用法），这两条 `label.grid` 规则匹配不到任何
DOM——Vue 的 row-gap 8px / line-height 21px 对齐实际失效。而 kb-u.css 的 .wk-kbl-4 是 gap: 4px（原 gap-1 直译），留守段表单
label→控件行距由 8px 回归为 4px，与 Vue 形态脱节。建议把选择器改到现类名（并注意 kb-u.css「冲突以 .td.css 为准」的导入顺序约定）或删除本规则、在 wk-kbl-4
中直接落 8px。

- /* 留守段字段行（vectorStore/parser/storage 的 label.grid）：label 15px/21 +
+ /* 留守段字段行（vectorStore/parser/storage 等 label.wk-kbl-4）：label 15px/21 +
     8px 间距（Vue form-label mb8 语义）。 */
- .wk-kb-editor-dialog .kb-editor-content label.grid { row-gap: 8px; line-height: 21px; }
- .wk-kb-editor-dialog .kb-editor-content label.grid > span { line-height: 21px; }
+ .wk-kb-editor-dialog .kb-editor-content label.wk-kbl-4 { row-gap: 8px; line-height: 21px; }
+ .wk-kb-editor-dialog .kb-editor-content label.wk-kbl-4 > span { line-height: 21px; }


─── apps/web/src/knowledge-bases/kb-list-icons.tsx:285-287 ───
[maintainability · low] 新增字形无消费者：本页编辑器导航已改用 tdesign-icons-react 的 TIcon（datasource 图标走
EDITOR_NAV_ICONS + TIcon），而 TDesignNavIcon 的唯一引用方 KnowledgeSettingsPage 的 tdesignNames 映射不含
'cloud-download'（已检索确认）。新增即死代码，建议移除，待 knowledge-settings 批次确需时再补。



─── apps/web/src/knowledge-bases/kb-list.td.css:2429-2430 ───
[maintainability · medium] 死样式段（约 390 行）：`.wk-kb-activity-*` / `.wk-kb-share-*` / `.wk-kb-des-*`
三个类族在 TSX/TS 中零引用（已全局检索确认）——实际生效的是 kb-u.css 的 wk-kba-* / wk-kbs-* / wk-kbl-*
族。两套类族并存极易漂移，且死代码膨胀包体。同类问题还有 §6 中的 .kb-tabs / .shared-badge / .personal-source / .shared-by-me-badge
/ .org-tag / .feature-badge.role-* 等 Vue 平移选择器无对应 DOM。建议整段删除，统一以 kb-u.css 单套类族为准（如确需保留请在注释标明消费者）。



─── apps/web/src/knowledge-bases/kb-list.td.css:1820-1824 ───
[maintainability · medium] Vue Transition 类无 React 生产者：`.shared-detail-drawer-enter*` / `-leave-*` 是
Vue <Transition name="shared-detail-drawer"> 自动添加的类名，React 端（SharedKnowledgeBaseDrawer.tsx 直接
createPortal 挂载/卸载，无过渡状态机）永远不会给 DOM 加这些类——规则为死代码，且抽屉因此丢失 Vue 端的 0.25s 滑入/淡出过渡（与 98 行注释声称复刻
Transition 不符）。建议删除该段，或在 React 侧实现等价 enter 过渡（挂载后下一帧加 is-open 类切换 transform）。



─── apps/web/src/knowledge-bases/kb-list.td.css:1604-1606 ───
[maintainability · medium] 全局强规则的第三份复制：`.t-dialog__position.t-dialog--top { padding-top: 40vh
!important; }` 无任何作用域限定，随 KB 列表页加载后在 SPA 全局驻留，影响所有默认 top 定位的 TDesign Dialog；且与
agents.td.css:1449、orgs.td.css:1187
完全相同（三份同值复制）。当前三处值一致尚无错位，但任一处后续调整即造成全站弹窗布局互相覆盖。建议收敛到共享样式（tdesign-theme.css 或公共 td 基础文件）单点定义。



─── apps/web/src/knowledge-bases/kb-list.td.css:1877-1882 ───
[bug · high] §8 全局类与 agents/orgs 同名规则不同值冲突：本段以「类名全局唯一（§2.5）」为由使用无页面前缀的全局类，但该前提不成立——同批 agents.td.css
§7（AgentEditorModal 平移）已定义同名规则且值不同：① `agents.td.css:1672 .settings-overlay .settings-modal {
max-width: 1100px }` 特异性 (0,2,0) > 本文件 `.settings-modal { max-width: 1000px }` (0,1,0)，只要用户本会话访问过
agents 页，KB 编辑器弹窗（DOM 同为 `.settings-overlay > .settings-modal`）宽度即被拉到 1100px，偏离 Vue 事实源 1000px；②
`.close-btn` 两处几何不同（本文件 top/right 20px + 灰底 vs agents 16px + 透明背景），同特异性 (0,2,0) 下按 CSS
加载顺序互相覆盖，结果非确定；③ orgs.td.css:1277 的 `.settings-modal { max-width: 1100px }` (0,1,0) 亦参与级联。§10 的
`.settings-overlay .t-radio-group …` 同样会跨页命中 agents 编辑器。建议为本段加页面前缀（如根类改为 `.wk-kb-settings-overlay`
或统一挂在 `.wk-kb-editor-dialog` 上），或将共享壳样式收敛到公共文件单点定义。

- .settings-modal {
-   position: relative;
-   width: 90vw;
-   max-width: 1000px;
-   height: 85vh;
-   max-height: 750px;
+ /* §8 根类建议带页面前缀（KnowledgeBasesPage 的 overlay 根节点
+    className 同步加 'wk-kb-settings-overlay'），避免与 agents.td.css /
+    orgs.td.css 的同名全局规则互相覆盖： */
+ .wk-kb-settings-overlay.settings-overlay { /* … */ }
+ .wk-kb-settings-overlay .settings-modal { max-width: 1000px; /* … */ }
+ .wk-kb-settings-overlay .close-btn { top: 20px; right: 20px; /* … */ }


─── apps/web/src/knowledge-settings/KnowledgeSettingsPage.css:759-761 ───
[bug · medium] 无障碍回归：原生 checkbox 被视觉隐藏（1px + clip）后，新增的 .kb-checkbox-input 规则族只覆盖了 :checked /
:disabled 态，缺少 input:focus-visible + .kb-checkbox-input 的焦点指示样式。键盘用户 Tab 到「知识检索/Wiki
索引」复选框时完全无法感知焦点位置（1px 元素上的默认 outline 实际不可见）。被复刻的 tdesign 原生 t-checkbox 在
.t-checkbox__former:focus-visible 时有 brand 色焦点环，此处应一并对齐。另外顺带说明：既有 .indexing-checks.is-locked
input[type='checkbox'] { accent-color: #dcdcdc }（889 行）在 input 被视觉隐藏后已无任何视觉效果，可一并清理。

  .indexing-check-head input:disabled:checked + .kb-checkbox-input::after {
    border-color: rgba(0, 0, 0, 0.26);
+ }
+ .indexing-check-head input:focus-visible + .kb-checkbox-input {
+   /* tdesign .t-checkbox__former:focus-visible 焦点环：brand 边 + 2px 外扩投影。 */
+   border-color: var(--wk-color-brand, #07c05f);
+   box-shadow: 0 0 0 2px var(--td-brand-color-focus, rgba(7, 192, 95, 0.2));
  }


─── apps/web/src/knowledge-settings/KnowledgeSettingsPage.css:831-831 ───
[maintainability · low] 无效 CSS 声明：border-color: none 不是合法取值（border-color 只接受 <color> | transparent |
currentColor），会被浏览器丢弃。且换装后内层 input 已 border: 0，这条 border-color 声明本身已无意义——焦点视觉由外层
.kb-text-input-wrap:focus-within 承担。建议移除无效声明，避免误导后续维护者以为这里有焦点态边框逻辑。

- .kb-text-input:focus { outline: none; border-color: none; }
+ .kb-text-input:focus { outline: none; }


─── apps/web/src/knowledge-settings/parserSettings.tsx:226-230 ───
[bug · medium] 无障碍回归：原生 `<select aria-label={group.label}>` 的可访问名称在换装后丢失了。包裹 span 是无语义的 generic
元素（display: block 且无 role），按 WAI-ARIA 规则 aria-label 挂在 generic 元素上会被辅助技术忽略，也不会传播给 TDesign Select
内部真正可聚焦的 input/触发器。结果是读屏用户 Tab 进入解析器设置的一排 Select 时听不到「PDF 文档 / Word
文档…」等组名，无法区分各文件族对应的引擎下拉。项目内已有既定模式是 `role="group" + aria-label`（见
KnowledgeDocumentsPage.tsx:4488、TagPickerDialog.tsx:278、PlatformShell.tsx:1283），建议对齐：给该 span 加
`role="group"`，让读屏在进入组内控件时播报分组名。

                <span
                  data-parser-group={group.key}
+                 role="group"
                  aria-label={group.label}
                  style={{ display: 'block' }}
                >


─── apps/web/src/knowledge/KnowledgeGraphPage.tsx:644-644 ───
[bug · high] Tailwind 已从 apps/web 移除（package.json 无相关依赖，全仓 CSS 也不存在 `.touch-none` 定义），此处保留的
`touch-none` 是死类，图谱画布因此丢失 `touch-action: none`。触屏设备上原生滚动/双指缩放会抢占指针手势并触发
pointercancel，导致图谱拖拽平移、节点拖动失效。建议把 `touch-action: none` 补进 knowledge-u.css 的 `.wk-kg-3`，并删除这个失效类名。

-             <svg className="touch-none wk-kg-3" viewBox={`0 0 ${surfaceSize.width} ${surfaceSize.height}`} role="img" aria-label={t('knowledgeBase.graph.ariaLinks')} onPointerDown={beginPan} onPointerMove={moveGraphGesture} onPointerUp={endGraphGesture} onPointerCancel={cancelGraphGesture} onClick={(event) => {
+             <svg className="wk-kg-3" viewBox={`0 0 ${surfaceSize.width} ${surfaceSize.height}`} role="img" aria-label={t('knowledgeBase.graph.ariaLinks')} onPointerDown={beginPan} onPointerMove={moveGraphGesture} onPointerUp={endGraphGesture} onPointerCancel={cancelGraphGesture} onClick={(event) => {
+ /* 同时在 knowledge-u.css 的 .wk-kg-3 中补一行：touch-action: none; */


─── apps/web/src/knowledge/knowledge-u.css:108-109 ───
[bug · high] calc() 内 `+`/`-` 运算符两侧必须有空白（`calc(100%-2rem)` 会被 CSS 解析器整条丢弃，`max-width` 实际不生效）。原
Tailwind 语义是 `max-w-[calc(100%-2rem)]`（即 `calc(100% - 2rem)`），窄视口下检索面板将失去宽度钳制、溢出画布。本文件 wk-kg-21 /
wk-kg-25 还有两处同类问题，建议一并修正。

    width: 20rem;
-   max-width: calc(100%-2rem);
+   max-width: calc(100% - 2rem);


─── apps/web/src/knowledge/knowledge-u.css:198-200 ───
[bug · high] `calc(100%+4px)` 缺少运算符两侧空白，属无效 CSS，`top` 声明会被整条丢弃（下拉回退到静态定位，不再保证贴住检索框下缘 4px）。应为
`calc(100% + 4px)`。

    position: absolute;
    left: 0;
-   top: calc(100%+4px);
+   top: calc(100% + 4px);


─── apps/web/src/knowledge/knowledge-u.css:254-256 ───
[bug · high] `calc(100%+8px)` 同样缺少运算符两侧空白，整条 `top` 声明无效，帮助浮层的 8px 下移间距会丢失。应为 `calc(100% + 8px)`。

    position: absolute;
    right: 0;
-   top: calc(100%+8px);
+   top: calc(100% + 8px);


─── apps/web/src/knowledge/knowledge-u.css:249-251 ───
[bug · medium] `.wk-kg-24 :-webkit-details-marker` 是「后代组合器 + 伪元素」的非法选择器，整条规则会被丢弃，"?" 帮助按钮在
Chrome/Safari 上会重新显示默认展开三角。原 utility 语义是 `[&::-webkit-details-marker]:hidden`，选择器应直接附在本体上。

- .wk-kg-24 :-webkit-details-marker {
+ .wk-kg-24::-webkit-details-marker {
    display: none;
  }


─── apps/web/src/knowledge/knowledge-u.css:327-329 ───
[maintainability · low] `.wk-kg-31` 中 `transition-duration: 150ms` 随即被下一行 `300ms` 覆盖，是 codemod
残留的死声明（原 utility 仅 `duration-300`），建议删除以免误导后续维护。

-   transition-duration: 150ms;
    transition-duration: 300ms;
  }


─── apps/web/src/knowledge/knowledge-u.css:698-698 ───
[bug · medium] `.wk-ds-self-start` 是 data-sources 域的规则（唯一消费方 DataSourcesPage.tsx:441
的「打开控制台」链接），但唯一定义落在了 knowledge-u.css —— 该文件只被懒加载的 KnowledgeGraphPage chunk（router.tsx:67）引入，且 vite
未关闭 CSS code-splitting。因此用户未访问过知识图谱页时，数据源页（KB 设置抽屉 / KB 编辑弹窗内嵌的 DataSourcesPage）拿不到 `justify-self:
start`，链接在 .wk-ds-8 的 grid 容器中回退为默认 stretch、占满整列。这正是本项目已定性过的 OCR R1-04/31 缺陷类型：commercial-u.css:44 与
data-sources-u.css:769 均注明「原唯一定义误置于懒加载的 knowledge-u.css……迁入本域」，但 `.wk-dsui-card` 迁走后这条被漏掉了。建议按同样方式迁入
data-sources-u.css 并从本文件删除。

- .wk-ds-self-start { justify-self: start; }
+ /* 迁移至 apps/web/src/data-sources/data-sources-u.css（随该页所在 chunk 加载） */
+ /* .wk-ds-self-start { justify-self: start; } */


─── apps/web/src/main.tsx:20-20 ───
[performance · low] career/opportunity.css 是唯一在应用入口全局引入的 career 页面样式——其余 11 个 career
页面（search.css/rule.css/application.css 等）均由各自页面模块自行 import、随懒加载分包。此处全局引入把仅 OpportunityEvidencePage
消费的 160 行页面 CSS 放进了首屏关键路径，且与同批页面做法不一致。建议删除本行，改在 career/OpportunityPage.tsx 顶部 `import
'./opportunity.css'`（该页目前未自行引入，仅测试用 readFileSync 直接读文件，移动不影响测试）。

- import './career/opportunity.css';
+ // 移除本行；改由 apps/web/src/career/OpportunityPage.tsx 模块内：
+ // import './opportunity.css'


─── apps/web/src/market/MarketPage.tsx:539-539 ───
[maintainability · low] 嵌套三元表达式（tone →
类名），违反检查清单"不允许嵌套三元"。虽系沿用原代码模式、本次仅替换类名，但该行为修改行，建议改为映射对象以符合规范并提升可读性。

-           className={'wk-mkt-34 ' + (toast.tone === 'success' ? 'wk-mkt-35' : toast.tone === 'warning' ? 'wk-mkt-36' : 'wk-mkt-37')}
+ const TOAST_TONE_CLASS: Record<NonNullable<ToastState>['tone'], string> = { success: 'wk-mkt-35', warning: 'wk-mkt-36', error: 'wk-mkt-37' };
+ // 使用处：className={'wk-mkt-34 ' + TOAST_TONE_CLASS[toast.tone]}


─── apps/web/src/market/MarketPage.tsx:727-727 ───
[maintainability · low] 第二处嵌套三元（安装行状态色），与 toast 色调同类问题，违反检查清单"不允许嵌套三元"。建议提取为显式映射（如按 failed/ready
归组的三分支函数或 Record），避免双重三元嵌套。

-                             <span className={'wk-mkt-43 ' + (failed || failedToStart ? 'wk-mkt-44' : ready ? 'wk-mkt-45' : 'wk-mkt-46')}>
+ const ROW_STATUS_CLASS = failed || failedToStart ? 'wk-mkt-44' : ready ? 'wk-mkt-45' : 'wk-mkt-46';
+ // 或拆为独立小函数 rowStatusClass(failed, failedToStart, ready) 以消除行内嵌套三元


─── apps/web/src/market/market-u.css:653-655 ───
[bug · high] T15 勘误结论有误，导致 Tab 选中态样式丢失（视觉回归）。注释断言原 aria-selected:border-accent / bg-accent-wash /
text-accent 三联 utility 为死样式故不补规则，但同一批次 experts-u.css（L62-70、L831-834）已对该同一论断做出"勘误之勘误"：迁移前基类与
aria-selected:* 三联同处 utilities layer，属性选择器特异性 (0,2,0) 高于单类 (0,1,0)，选中页签实际渲染绿描边 + accent-wash 底 +
绿字，并非死样式，并在该域补齐了 .wk-exp-xp-tab[aria-selected="true"]。market 域未同步修正且全仓库无任何 .wk-mkt-mk-tab
选中态规则，MarketPage 的"市场/租户"Tab（aria-selected={tab === ...}）与排行榜 Tab 选中后无任何视觉区分（仅 hover
有反馈）。建议补齐规则并修正注释；另该注释紧贴 ::placeholder 规则，易被误读为其一部分。

- /* T15 勘误：原 aria-selected:* 三联 utility 为死样式（同 experts：unlayered
-    .wk-mkt-mk-tab 基类恒胜），不补规则保持生效值。 */
+ /* T15 勘误之勘误（同 experts-u.css L62-70）：迁移前基类与 aria-selected:* 三联
+    同处 utilities layer，属性选择器特异性 (0,2,0) 高于单类 (0,1,0)，选中页签
+    实际渲染绿描边 + accent-wash 底 + 绿字，并非死样式，此处补齐承接。 */
+ .wk-mkt-mk-tab[aria-selected="true"] {
+   border-color: #07c05f;
+   background-color: rgba(7, 192, 95, 0.08);
+   color: #07c05f;
+ }
+ 
  .wk-mkt-mk-input::placeholder { color: rgba(23, 26, 29, 0.35); }


─── apps/web/src/market/market-u.css:378-379 ───
[style · low] 两条声明挤在同一行（border-bottom-width: 1px;border-color: ...），wk-mkt-32 处 border-top 同样写法；另多处
hover/选中规则重复书写恒真的 border-style: solid。不影响生效值，仅影响可读性，建议按一行一声明排版与文件其余部分保持一致。

    border-bottom-style: solid;
-   border-bottom-width: 1px;border-color: rgba(127,127,127,0.2);
+   border-bottom-width: 1px;
+   border-color: rgba(127,127,127,0.2);


─── apps/web/src/platform/PlatformShell.tsx:68-69 ───
[bug · medium] 键盘分支不可达：React 合成 KeyboardEvent 没有 `button` 属性（运行时为 undefined），`(event as
ReactMouseEvent).button !== 0` 恒为 true，Enter 按键永远提前 return。line 1266 的 `onKeyDown={event.key ===
'Enter' → handleInternalLink}` 因此成为死代码，dropdown-user-header（role="button" tabIndex={0}）无法用 Enter
激活，键盘可达性回退。注释里 "keyboard events keep the early return" 实际固化了这个缺陷——既然引入联合类型是为了复用 Enter 路径，就应让键盘事件跳过
button 判断。

  function handleInternalLink(event: ReactMouseEvent<Element> | ReactKeyboardEvent<Element>, path: string, afterNavigate?: () => void): void {
-   if (event.defaultPrevented || (event as ReactMouseEvent<Element>).button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
+   const mouseButton = (event as ReactMouseEvent<Element>).button;
+   if (event.defaultPrevented || (mouseButton !== undefined && mouseButton !== 0) || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;


─── apps/web/src/platform/PlatformShell.tsx:553-553 ───
[bug · low] 门控丢失 hasSwitchHandler 检查带来静默 no-op：`onTenantSwitch` 是可选 prop（line 78 `onTenantSwitch?:
...`），旧实现通过 `Boolean(onTenantSwitch)` 隐藏切换入口；现在 `user.memberships.length > 0` 对几乎任何登录用户恒真，未注入
onTenantSwitch 的宿主也会渲染切换 glyph，点击后 `switchTenant` 开头直接 return，无任何反馈。对齐 Vue "memberships >= 1 即显示"
没问题，但 React 壳层的切换能力是宿主注入的可选依赖，应保留该检查（或至少在未传时禁用按钮）。

-   const tenantSwitcherVisible = !isLiteEdition && user.memberships.length > 0;
+   const tenantSwitcherVisible = !isLiteEdition && user.memberships.length > 0 && Boolean(onTenantSwitch);


─── apps/web/src/platform/PlatformShell.tsx:216-216 ───
[maintainability · low] career 三个导航项 label 硬编码中文（'求职档案'/'找岗'/'持续找岗'，含下方 careerSearch/careerRules
两行），绕过了同函数内其余条目统一使用的 `labels.*` / `t()` i18n 通道——切换语言后导航仍显示中文，与其余入口（newChat 走 labels、knowledgeBases
走 t()）约定不一致。建议在 labels 注入或 i18n 资源中增加对应 key（career 页面文案目前同样硬编码，导航侧先建立 key 可避免双源）。

-     { key: 'career', href: '/platform/career', label: '求职档案', icon: 'career', iconNode: <ReactOnlyNavIcon paths={['M4 7.5H20', 'M6 4.5H18V20H6Z', 'M9 12H15', 'M9 15.5H13']} />, match: (p: string) => p === '/platform/career' },
+ // 走 labels 注入通道（与 newChat/agents 等条目一致）：
+     { key: 'career', href: '/platform/career', label: labels.career, icon: 'career', ... }


─── apps/web/src/platform/PlatformShell.tsx:1104-1107 ───
[bug · low] 窄屏 overlay 侧栏缺少关闭路径：`.aside_box--mobile-overlay`（platform-shell.td.css，fixed + z-index
1001）展开后没有对应遮罩层，PlatformShell 也没有点击外部 / Esc 关闭逻辑（`narrowSidebarExpanded` 只能由侧栏内的折叠按钮翻转）。用户在 640px
视口展开侧栏后，260px 面板持续覆盖路由内容左缘，交互闭环不完整。建议窄屏展开时同时渲染一个遮罩（onMouseDown →
setNarrowSidebarExpanded(false)），与桌面 modal 侧栏的常规模式对齐。

+       {isNarrowViewport && !collapsed ? (
+         <div className="aside_box-overlay" role="presentation"
+           onMouseDown={() => setNarrowSidebarExpanded(false)} />
+       ) : null}
        <aside className={[
          collapsed ? 'aside_box aside_box--collapsed' : 'aside_box',
          isNarrowViewport && !collapsed ? 'aside_box--mobile-overlay' : '',
        ].filter(Boolean).join(' ')}>


─── apps/web/src/platform/PlatformShell.tsx:59-59 ───
[bug · high] getImgSrc 用含变量的模板字符串构造 `new URL(\`./assets/img/${url}\`, import.meta.url)`，注释声称 "vite
资产管线在 dev/build 均支持"，但这与 Vite 的文档契约不符：URL 必须是静态字符串才能被分析，含动态段的表达式会被原样保留，引用的文件**不会**被 `vite build`
跟踪/产出。后果：dev 正常（dev server 直读源码目录），但生产构建后 import.meta.url 指向 hash 后的 chunk（如
/assets/PlatformShell-xxx.js），`./assets/img/*.svg` 运行时解析为 /assets/assets/img/...——dist
中不存在，logo、搜索图标、四个主导航 <img>（prefixIcon/zhishiku/agent/organization 及 -green 变体）全部 404，侧栏视觉在 prod
构建下整排缺失。Vue 原实现（menu.vue:1143 用 origin 绝对路径 `/src/assets/img/${url}`）同样只在 dev 成立，不能作为 build
正确性的依据。建议改为静态 import（Vite 会对静态 import 的资产在 build 产出 hash 文件；node 测试侧旧代码本就静态 import 过
weknora.png，已有资产 stub 路径），或用 import.meta.glob 构建 url 映射；合并前请实际跑一次 `vite build` 确认 dist 内包含这些文件。

- const getImgSrc = (url: string): string => new URL(`./assets/img/${url}`, import.meta.url).href;
+ // 静态 import：Vite build 产出 hash 资产 URL，dev/node 各自可解析
+ import searchIconUrl from './assets/img/search.svg';
+ import weknoraLogo from './assets/img/weknora.png';
+ import prefixIconUrl from './assets/img/prefixIcon.svg';
+ import prefixIconActiveUrl from './assets/img/prefixIcon-green.svg';
+ // ...zhishiku/agent/organization 同理；NAV_ICON_URLS 直接引用这些常量。


─── apps/web/src/platform/PlatformShell.tsx:1110-1110 ───
[style · low] logo_box 锚点带静态内联样式 `style={{ cursor: 'pointer' }}`：项目约定避免静态内联
style（除动态样式外）；且该规则本身冗余——带 href 的 <a> 在 UA 默认样式下已是 pointer 光标。建议删除内联样式，如需显式声明则随其余平移样式一起落入
platform-shell.td.css（如 `.aside_box .logo_box { cursor: pointer; }`）。

-           <a className="logo_box" style={{ cursor: 'pointer' }} href="/platform/knowledge-bases" aria-label="WeKnora" onClick={(event) => {
+           <a className="logo_box" href="/platform/knowledge-bases" aria-label="WeKnora" onClick={(event) => => {
+           /* cursor: pointer 移入 platform-shell.td.css：.aside_box .logo_box { cursor: pointer; } */


─── apps/web/src/platform/platform-shell.td.css:1568-1576 ───
[maintainability · low] `.aside_box--mobile-overlay` 为 fixed + z-index 1001
的覆盖式侧栏，但配套的遮罩/点击外部关闭未落地（见 PlatformShell.tsx
对应评论）：展开后仅能通过侧栏内折叠按钮收回。此处补充遮罩层样式（或至少在注释中记录该缺口），避免窄屏交互闭环缺失被样式层掩盖。

  @media (max-width: 640px) {
+   .aside_box-overlay {
+     position: fixed;
+     inset: 0;
+     z-index: 1000;
+     background: rgba(15, 23, 32, 0.35);
+   }
    .aside_box--mobile-overlay {
      position: fixed;
      inset: 0 auto 0 0;
      z-index: 1001;
      height: 100vh;
      box-shadow: 8px 0 28px rgba(15, 23, 32, 0.2);
    }
  }


─── apps/web/src/platform/platform-u.css:47-51 ───
[bug · medium] `calc(100vw-32px)` 缺少减号两侧空格，是非法 calc() 值，浏览器会整条丢弃 max-width——命令面板在 <672px
视口失去宽度上限而横向溢出。这与本次特意补齐的 640px 窄屏适配（platform-u.css 尾部放开 .wk-shell-1 的 min-width）直接矛盾。文件内
`.wk-cmdk-32`（line 348 附近，检索设置抽屉）有同款错误，需一并修复。

- .wk-cmdk-6 {
-   display: flex;
-   max-height: 70vh;
-   width: 640px;
-   max-width: calc(100vw-32px);
+   max-width: calc(100vw - 32px);


─── apps/web/src/platform/platform-u.css:341-348 ───
[bug · medium] `.wk-cmdk-32` 的 `max-width: calc(100vw-32px)` 同样是非法 calc()（`-`
两侧需空格），整条声明被浏览器丢弃；检索设置抽屉在窄视口将失去 max-width 约束（420px 面板在小屏溢出）。同文件 `.wk-cmdk-6` 的同款错误见另一条评论。

- .wk-cmdk-32 {
-   position: absolute;
-   right: 0;
-   top: 0;
-   display: flex;
-   height: 100%;
-   width: 420px;
-   max-width: calc(100vw-32px);
+   max-width: calc(100vw - 32px);


─── apps/web/src/platform/platform-u.css:445-449 ───
[bug · medium] 层叠顺序颠倒导致覆盖语义失效：`.wk-cmdk-42`（本处，flex-start / gap 2px）定义在文件中部，而
`.wk-cmdk-item-row`（align-items: center / gap 0.5rem，line 671）定义在文件尾部。chunk
与消息条目同时挂载两个类（`className={`${itemRowClass(...)} wk-cmdk-42`}`），二者特异度相同（0-1-0），后定义的
`.wk-cmdk-item-row` 胜出——实际渲染为 8px 行距而非原 utility 串 `flex-col items-start gap-0.5` 编码的
2px，两行堆叠条目的行距放大约 4 倍，属可从层叠顺序直接判定的迁移保真回归。建议将 `.wk-cmdk-42` 移至 `.wk-cmdk-item-row` 规则之后，或提升特异度。

- .wk-cmdk-42 {
+ /* 移至文件尾部 .wk-cmdk-item-row 之后，或改用组合选择器提升特异度： */
+ .wk-cmdk-item-row.wk-cmdk-42 {
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
  }


─── apps/web/src/platform/platform-u.css:387-388 ───
[maintainability · low] 死声明：`transition-duration: 150ms` 紧接着被 `100ms` 覆盖（原 utility 串 duration-100
覆盖默认 duration，codemod 把两个值都输出了）。前者永不生效，应删除。

-   transition-duration: 150ms;
    transition-duration: 100ms;


─── apps/web/src/router.tsx:719-719 ───
[bug · medium] careerRoute 是本批 5 条 career 路由中唯一未包 Suspense
的（careerSearchRoute/careerRulesRoute/careerOpportunityRoute/careerEvaluationRoute 均有局部边界）。CareerPage
为 lazy 组件，首次访问 /platform/career 时其挂起会向上冒泡到 platformRoute 的 shellPage
Suspense（router.tsx:211-215），该边界位于 PlatformShell 之外，导致整个平台壳（导航侧栏等）被 RoutePending 整体替换，chunk
加载完成后壳层重新挂载，产生整壳闪烁；其余路由均为壳内局部回退。建议与兄弟路由保持一致补上局部 Suspense。

-     component: (): ReactNode => <CareerPage client={client} scopeController={scopeController} userId={scopeController.current().scope.userId} />,
+     component: (): ReactNode => (
+       <Suspense fallback={<RoutePending loadingText={deps.loadingText} />>
+         <CareerPage client={client} scopeController={scopeController} userId={scopeController.current().scope.userId} />
+       </Suspense>
+     ),


─── apps/web/src/router.tsx:716-720 ───
[documentation · low] 注释错位：careerRoute 被插入到了原本属于 expertsRoute 的注释（「Octop M2 expert-template catalog;
... (routes_expert.go guard)」）与其目标之间。该注释描述的是 experts 路由的读写权限语义（Viewer+ 读取 / Contributor+ 写入由
routes_expert.go 服务端守卫），现在却直接挂在 careerRoute 上方，会误导读者以为 career 路由遵循同样的 expert 目录权限模型；而真正的
expertsRoute（本 diff 下方第 772 行）反而失去了说明。建议把这两行注释移回 expertsRoute 之上，careerRoute 若需要说明则另写（或留空）。

    const careerRoute = createRoute({
      getParentRoute: () => platformRoute,
      path: 'career',
      component: (): ReactNode => <CareerPage client={client} scopeController={scopeController} userId={scopeController.current().scope.userId} />,
    });
+ 
+   // ...（career 其余路由）...
+ 
+   // Octop M2 expert-template catalog; list/detail are Viewer+ reads, the
+   // instantiate write stays Contributor+ server-side (routes_expert.go guard).
+   const expertsRoute = createRoute({


─── apps/web/src/routes.tsx:57-59 ───
[maintainability · low] snapshotId 判定口径与 router.tsx 分叉：resolveRoute 要求 snapshotId 非空且 trim 后非空（缺失即
not-found，routes.test.ts:45 已固化该语义），而 router.tsx careerOpportunityRoute 对缺失 snapshotId 取 `?? ''`
继续渲染（页面侧 :400 自行置 invalid 态兜底）。两套真相源对同一 URL 给出不同结局：守卫侧判 not-found、渲染侧进 invalid
页。当前两者都是错误态无数据风险，但口径应统一——要么 router 侧对空 snapshotId 走 notFound
渲染与守卫一致，要么在两处注释中显式声明「守卫严格、页面宽松兜底」的分工，避免后续维护者只改一侧。

-     const opportunityId = decodeSegment(careerOpportunity[1]!);
      const snapshotId = query.get('snapshotId')?.trim();
+     // 口径说明：此处缺失 snapshotId 判 not-found（守卫语义）；router.tsx
+     // careerOpportunityRoute 对缺失值宽松渲染并交由页面 invalid 态兜底，
+     // 两处行为需同步维护。
      return opportunityId && snapshotId ? { kind: 'career-opportunity', path, opportunityId, snapshotId } : { kind: 'not-found', path };


─── apps/web/src/routes.tsx:93-93 ───
[maintainability · low] 平台路径白名单仍以单行超长 || 串硬编码，本次再追加 3 个 career 路径后已接近 700 字符，后续每加一个页面都要改动这一行，review
diff 也难以核对。建议抽为模块级 Set 常量 + 前缀规则数组，resolveRoute 内用 has() / some() 判定。

-   if (path === '/platform' || path === '/platform/knowledge-bases' || path === '/platform/knowledge-search' || path === '/platform/career' || path === '/platform/career/search' || path === '/platform/career/rules' || path === '/platform/agents' || path === '/platform/experts' || path === '/platform/market' || path === '/platform/integrations' || path === '/platform/creatChat' || path === '/platform/tenant' || path === '/platform/organizations' || path === '/platform/analytics' || path === '/platform/settings' || path === '/platform/configuration' || path === '/platform/administration' || path === '/platform/system' || path === '/platform/system/settings' || path === '/platform/system/admins' || path === '/platform/system/queues' || path === '/platform/billing' || path === '/platform/billing/checkout' || path === '/platform/billing/admin' || (development && path === '/platform/dev/markdown') || path.startsWith('/platform/chat/') || path.startsWith('/platform/shared/')) return { kind: 'platform', path };
+ const PLATFORM_EXACT_PATHS: ReadonlySet<string> = new Set([
+   '/platform', '/platform/knowledge-bases', '/platform/knowledge-search',
+   '/platform/career', '/platform/career/search', '/platform/career/rules',
+   /* …其余精确路径… */
+ ]);
+ const PLATFORM_PREFIX_RULES = ['/platform/chat/', '/platform/shared/'];
+ // 判定：
+ // if (PLATFORM_EXACT_PATHS.has(path) || PLATFORM_PREFIX_RULES.some((p) => path.startsWith(p)) || (development && path === '/platform/dev/markdown')) return { kind: 'platform', path };


─── apps/web/src/settings/ChatHistorySettingsPanel.tsx:51-54 ───
[bug · medium] 防抖保存在途竞态会静默丢弃用户编辑：scheduleSave 的 setTimeout 回调先置 saveTimer.current = null 再调
save()，若此时上一次 save 仍在进行（savingRef.current === true），save() 在第 52 行直接
return——定时器已清空、快照被丢弃、无任何重排机制，保存期间用户拨动开关/切换模型产生的修改将永久丢失且无提示。更糟的是 save 成功后 onSaved?.()
触发父层重新拉取（SettingsPage 中 onSaved={() => void load(true)}），initialValue 引用变化后第 41-46 行的同步 effect
会用服务端旧状态覆写本地未保存的新编辑，造成设置回退。建议：save() 遇到在途时记录 pending 快照并在 finally 中重排，或在 finally 中比对 latestRef 与
saved 后再次 scheduleSave。

    async function save(next: ChatHistoryConfig) {
-     if (savingRef.current) return;
+     if (savingRef.current) {
+       pendingRef.current = next; // 保存进行中：暂存而非丢弃
+       return;
+     }
      savingRef.current = true;
      try {


─── apps/web/src/settings/ChatPreferencesPanel.tsx:96-96 ───
[bug · low] S6 Tailwind 收编时丢失了移动端 max-width 覆盖：原 utility 串为 w-[280px] max-w-[280px] shrink-0
max-[720px]:w-full max-[720px]:max-w-full，而 settings-wrapper.css 中 .wk-chat-prefs-control 仅声明 {
width: 280px; max-width: 280px; flex-shrink: 0; }，≤720px 媒体查询里只覆盖了 width: 100%，max-width: 280px
仍然生效——窄屏下该控件实际被钳制在 280px，无法如迁移前那样撑满整行（ModelOptionSelect 跟随容器宽度，同样受影响）。建议在媒体查询块中补回 max-width 覆盖。

- <div className="wk-chat-prefs-control">
+ /* settings-wrapper.css */
+ @media (max-width: 720px) {
+   .wk-chat-prefs-control { width: 100%; max-width: 100%; }
+ }


─── apps/web/src/settings/CloudSettingsPanel.tsx:316-316 ───
[security · low] 使用 dangerouslySetInnerHTML 注入 t('settings.weknoraCloud.usageSteps')：迁移前该处为
split('\n').map(line => <span>) 的安全节点渲染。虽已核实当前五种语言的 usageSteps 均为静态开发者文案、无用户输入，暂不构成实际
XSS，但一旦未来文案引入插值参数或翻译源被改为含 HTML，此处即成注入点，属于相对原实现的模式降级。建议改回节点渲染（或 white-space: pre-line + 纯文本），消除
innerHTML 红线隐患。

-         <p className="hint-text" dangerouslySetInnerHTML={{ __html: t('settings.weknoraCloud.usageSteps').replace(/\n/g, '<br />') }} />
+         <p className="hint-text">{t('settings.weknoraCloud.usageSteps').split('\n').map((line, index) => <span key={index} className="block">{line}</span>)}</p>


─── apps/web/src/settings/CloudSettingsPanel.tsx:134-135 ───
[bug · low] 空值守卫从旧实现的 !appId.trim() / !appSecret.trim()
弱化为纯真值判断：纯空格输入现在能通过守卫并使保存按钮（disabled={!form.appId || !form.appSecret}）保持可点，随后 cloudCredentialPatch
内部 trim 后抛出英文硬编码错误（"WeKnora Cloud app ID is required"），经 catch 以 error toast 直出——替代了迁移前本地化的
fillRequired 警告，且错误文案语言与界面 locale 脱钩。建议守卫与 disabled 均恢复 trim 语义（与 surface.ts cloudCredentialPatch
的校验对齐）。

-     if (!form.appId || !form.appSecret) {
+     if (!form.appId.trim() || !form.appSecret.trim()) {
        pushSettingsToast(t('settings.weknoraCloud.fillRequired'), 'warning');


─── apps/web/src/settings/ConfigSettingsPanel.tsx:148-149 ───
[bug · medium] TSelect 新增 clearable 但 onChange 直接 String(value) 无空值守卫：TDesign Select 点击清除按钮时
onChange 回调收到 null（SandboxSettingsPanel.test.tsx:985 的注释亦证实该库行为），String(null) 会产生字面量 'null' 字符串经
setValue 写入 rerank_model_id，随 500ms 防抖持久化到服务端，导致检索 rerank 模型配置损坏。本批其他迁移点（如 CloudSettingsPanel 的
TInput）均写 String(value ?? '')，此处应判空映射为空串（options 中已有 value: '' 的占位项）。

-     onChange={(value) => setValue(key, String(value))}
+     onChange={(value) => setValue(key, value == null ? '' : String(value))}
    />;


─── apps/web/src/settings/GeneralPreferencesPanel.tsx:273-273 ───
[maintainability · low] TDesign 同构迁移导致可访问性属性回退：语言/主题/衬线字体/等宽字体四处 Select、字号 RadioGroup 及 liteMode
自动更新 Switch 的 aria-label 均被移除（原实现分别为
aria-label={t('language.selectLanguage')}、t('theme.selectTheme')、t('font.selectFont')×2、t('font.font
Size')、autoUpdateCopy.label）。全文件已无任何 aria-label（ConfigSettingsPanel 迁移后保留了 TSwitch/TSlider 的
aria-label，可见并非 TDesign 组件不支持）。屏幕阅读器用户将无法区分这些无可见 label 关联的控件，建议按迁移前清单补回。

-             <Select value={locale} placeholder={t('language.selectLanguage')} onChange={(value) => handleLanguageChange(String(value))} style={{ width: '280px' }}>
+             <Select value={locale} aria-label={t('language.selectLanguage')} placeholder={t('language.selectLanguage')} onChange={(value) => handleLanguageChange(String(value))} style={{ width: '280px' }}>


─── apps/web/src/settings/McpSettingsPanel.tsx:1237-1239 ───
[style · low] 三处超时/重试/重试延时输入的 className="wk-mcp-number-input"（本行及 1250、1262 行）顶格无缩进，疑似粘贴残留，破坏 JSX
属性缩进结构，影响可读性且易在后续 diff 中被误改。顺带说明：这三处迁移虽去掉了 type="number"/min/max，但 normalizeMcpAdvancedNumber 已在
onBlur 与 payload 构建双重钳制 NaN，功能上无缺陷，仅需修复缩进。

                          <TInput
- className="wk-mcp-number-input"
+                           className="wk-mcp-number-input"
                            value={draft.timeout === "" ? "" : String(draft.timeout)}


─── apps/web/src/settings/McpSettingsPanel.tsx:1238-1238 ───
[bug · medium] wk-mcp-number-input 在全代码库（settings-wrapper.css / settings.td.css /
styles.css）中均无对应规则，是一个空挂的 hook 类。旧实现此处为原生 Input + Tailwind pr-9（padding-right: 36px），用于给绝对定位的单位后缀
.wk-mcp-unit（position: absolute; right: 12px）让位；迁移到 TInput 后该右内边距完全丢失，较长数值（粘贴/连续输入在 onBlur
钳制前）会直接从"秒/次"单位下方穿过。与本文件其余已移植 hook 类（wk-mcp-docs-btn、wk-mcp-resize-bar 等均有 scoped
规则）的约定不一致，属迁移遗漏。建议在 settings-wrapper.css 补充等效规则，例如：.wk-settings-drawer-root .wk-mcp-number-input
.t-input__inner { padding-right: 36px; }（三处 1238/1250/1262 行共用）。

- className="wk-mcp-number-input"
+ /* settings-wrapper.css */
+ .wk-settings-drawer-root .wk-mcp-number-input .t-input__inner { padding-right: 36px; }


─── apps/web/src/settings/ResourceSettingsPanel.tsx:231-238 ───
[bug · medium] 数字字段由 type="number" Input(带 min={field.min ?? 1} 与副本数 10/64 上限钳制)迁移为纯文本 TInput 后:(1)
min/max 约束整体丢失;(2) onChange 的 `Number.isFinite(parsed) ? parsed : text`
分支允许字母等非法文本以字符串形式写入向量库连接/索引配置并随保存提交,服务端按数字解析时将报错或落库脏配置。建议非数字文本直接回退 undefined(或给出校验提示),如需保留数值钳制可改用
InputNumber。

-       <TInput
-         value={raw}
-         placeholder={field.default != null ? String(field.default) : ''}
-         onChange={(value) => {
-           const text = String(value).trim();
-           if (!text) { onChange(undefined); return; }
            const parsed = Number(text);
-           onChange(Number.isFinite(parsed) ? parsed : text);
+           // 非数字文本不落配置,避免字符串透传到数字型 schema 字段
+           onChange(Number.isFinite(parsed) ? parsed : undefined);


─── apps/web/src/settings/ResourceSettingsPanel.tsx:13-13 ───
[maintainability · low] isReplicaField 在本次删除 max 计算后已无任何使用处(全文件仅剩此 import),属死导入,建议一并移除。

-   isReplicaField,
+ // 从 import 中删除 isReplicaField(唯一调用点 max 计算已随 TInput 迁移移除)


─── apps/web/src/settings/ResourceSettingsPanel.tsx:950-950 ───
[maintainability · low] `theme: 'error' as never` 是绕过 DropdownOption theme 类型检查的断言写法:若当前 tdesign 版本的
DropdownOption 类型确实不含 'error',运行时该删除项不会按预期红显(静默退化为主题默认色),类型与行为不一致。建议核对所依赖 tdesign-react
版本的类型定义,支持则去掉断言写明字面量,不支持则显式扩展类型或移除该字段,不要用 never 伪装。



─── apps/web/src/settings/RuntimeQueuesPanel.tsx:387-387 ───
[maintainability · low] updatedAt 处的 ClockIcon 用法被 TIcon name="time" 替换后,ClockIcon 组件(约 484
行)已无任何调用点(ErrorIcon 仍在错误通知处使用,需保留),属死代码,建议删除 ClockIcon 定义。



─── apps/web/src/settings/SandboxSettingsPanel.tsx:1902-1902 ───
[bug · medium] 沙箱卡片的键盘激活由迁移前的 Enter/Space 双键(带 preventDefault)收窄为仅 Enter:对 role="button" 元素,ARIA
交互规范要求 Enter 与 Space 均可激活,且 Space 缺省行为还会滚动页面。建议恢复双键分支并 preventDefault。

- onKeyDown: (event: KeyboardEvent<HTMLDivElement>) => { if (event.key === 'Enter') openEdit(item); },
+                     onKeyDown={(event: KeyboardEvent<HTMLDivElement>) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); openEdit(item); } }}


─── apps/web/src/settings/SandboxSettingsPanel.tsx:1631-1633 ───
[maintainability · medium] SandboxBackendBadge 在本文件与 SkillSettingsPanel.tsx 中各有一份完整实现(providerLogo
mono 回退、--logo-url CSS 变量注入、TDesign glyph
回退、类名拼接逻辑完全一致),同源组件双份维护,后续改动(如新增后端类型、调整徽章尺寸档)极易只改一处造成两面板漂移。建议提取为共享模块(如
settings/SandboxBackendBadge.tsx)供两个面板复用。

- function SandboxBackendBadge({ type, size = 'md' }: { type: string; size?: 'xs' | 'sm' | 'md' }) {
-   const logo = providerLogo('sandbox', type);
-   const iconName = type === 'cube' ? 'server' : type === 'disabled' ? 'minus-circle' : 'cloud';
+ // 提取到 apps/web/src/settings/SandboxBackendBadge.tsx,Sandbox/Skill 两面板统一 import 复用


─── apps/web/src/settings/SettingsPage.tsx:494-498 ───
[bug · high] SELF_HEADER 早退分支的返回结构相对旧渲染路径丢掉了三类反馈,造成功能回归:
1) notice 渲染点丢失——notice 目前仅在能力不支持回退 general 时写入(setNotice('settings.capabilityUnavailable...')),而
general ∈ SELF_HEADER_SECTIONS,旧代码在该分区内容上方渲染 `{isActive && notice ? <Status
tone="success">...}`,现在用户切换到受限分区后零提示;
2) inline 错误模式分区的 sectionError 丢失——chathistory/memory 均为 inline 模式且面板只消费壳层 payload(无 error
props、不自持错误态),加载失败时面板以 null payload 的默认值静默渲染;两面板均带防抖自动保存(开关/模型选择 500ms
后持久化),用户在无感知的失败态下误触开关会把默认值(甚至空 embedding_model_id)持久化覆盖服务端配置;
3) sectionLoading 丢失——首屏加载期间默认值即可交互,同样有提前自动保存的风险。tenant/userprofile/system 已通过 error/loading/onRetry
props 补偿,chathistory/memory 需同步处理,或在本分支为 inline 模式分区渲染兜底反馈。

        return (
          <div key={key} className="section" style={isActive ? undefined : { display: 'none' }}>
+           {isActive && notice ? <Status tone="success">{notice}</Status> : null}
+           {!sectionIntegrationTab && sectionError && sectionErrorMode(key) === 'inline' ? <Status tone="error">{sectionError}</Status> : null}
            {content}
          </div>
        );


─── apps/web/src/settings/SettingsPage.tsx:580-584 ───
[bug · medium] 导航项由 <button class="wks-nav-item focus-visible:outline-2
focus-visible:outline-accent/35 ..."> 改为 div[role=button] 后,旧的 Tailwind 焦点样式类被删除,但 settings.td.css 中
.nav-item 仅有 base(:122)/hover(:135)/active(:140) 三条规则,没有任何 :focus-visible 样式;叠加
settings-wrapper.css:427 在非 kbd-nav 模式下对 drawer 内 :focus-visible 的全局 outline:none,键盘 Tab
浏览侧边栏时焦点指示从「品牌色描边」退化为依赖 .wk-kbd-nav 模式的 UA 默认样式甚至完全不可见。建议在 settings.td.css 补
`.nav-item:focus-visible` 规则。

-                       <div
-                         key={item.key}
-                         className={'nav-item' + (item.key === selectedKey ? ' active' : '')}
-                         role="button"
-                         tabIndex={0}
+ /* settings.td.css 补充 */
+ .wk-settings-drawer-root .nav-item:focus-visible {
+   outline: 2px solid var(--td-brand-color, #07c05f);
+   outline-offset: 2px;
+ }


─── apps/web/src/settings/SettingsPage.tsx:728-729 ───
[bug · low] NAV_ICON_NAMES 缺少 usage 与 chat-preferences 两个键(websearch/weknoracloud/sandbox/claw
有自定义分支不受影响),这两个分区的导航图标经 `NAV_ICON_NAMES[itemKey] ?? 'setting'`
回退为通用齿轮,与迁移前的统计柱状图/滑杆图标不一致,属视觉回归。建议补齐映射。

+   usage: 'chart-bar',
+   'chat-preferences': 'setting-1',
    'integration-chrome': 'extension',
  };


─── apps/web/src/settings/SettingsPage.tsx:531-532 ───
[maintainability · low] banner-retry 分支现在仅 members 可达(parser/system/userprofile 已进入 SELF_HEADER
早退分支,storage 的错误模式是 silent),但:上方注释仍声称覆盖 parser/system/userprofile;下方重试文案三元 `key === 'members' || key
=== 'storage' ? t('settings.storage.retry') : t('settings.parser.retry')` 中 storage 与 parser
两分支均为死条件,实际恒走 storage.retry。建议同步注释并简化该三元,避免误导后续维护者。

-           {systemAdminOnlyPanelDenied ? null : (
-           sectionError && sectionErrorMode(key) === 'banner-retry' ? (
+ <TButton type="button" onClick={() => { void load(true); }}>{t('settings.storage.retry')}</TButton>


─── apps/web/src/settings/SystemGlobalSettingsPanel.tsx:382-382 ───
[bug · high] draftRef 陈旧草稿会被持久化到服务端:所有重置 editValues 的路径(重新拉取设置、保存回填、分区切换重载)都不清理 draftRef,而 TInput 的
onBlur 读取的正是这个可能过期的草稿。复现:用户在 string 字段输入新值(draftRef 记下草稿)→ 未 blur 前列表刷新重置了 editValues →
用户点击该输入框再移开焦点,onBlur 会把已过期的草稿 persist 回服务端,静默覆盖刷新后的配置。
另外 TDesign TInput 的 onBlur 回调本身就携带最新值 `(value, ctx) => ...`(InputNumber 分支已经这么用了),draftRef
整体没有必要——直接用 onBlur 的 value 参数即可同时修复闭包过期与本缺陷。

-   const draftRef = useRef<Record<string, unknown>>({});
+   // 删除 draftRef;TInput 改用 onBlur 携带的最新 value(与 InputNumber 分支同口径):
+   // onBlur={(value) => { void persist(item, typeof value === 'string' ? value : String(current ?? '')); }}


─── apps/web/src/settings/SystemGlobalSettingsPanel.tsx:473-473 ───
[bug · medium] int 字段由 NumberInput 迁移到 InputNumber 时丢失了原有的 max={9999} 上限,现在仅剩 min 约束,任意大数值(如
999999)可直接 blur-persist 到服务端系统设置。建议补回 max 上限。

-                         ? <InputNumber className="setting-input" value={typeof current === 'number' ? current : Number(current ?? 0)} min={minimumFor(item.key)} disabled={itemSaving} aria-label={keyLabel(item.key)} theme="normal" step={1} placeholder={t('system.globalSettings.tagInputPlaceholder')} onChange={(value) => { draftRef.current[item.key] = value; setEditValues((state) => ({ ...state, [item.key]: value })); }} onBlur={(value) => { const parsed = value === '' || value === null || value === undefined ? null : Number(value); if (parsed !== null && !Number.isNaN(parsed)) void persist(item, parsed); }} />
+                         ? <InputNumber className="setting-input" value={typeof current === 'number' ? current : Number(current ?? 0)} min={minimumFor(item.key)} max={9999} disabled={itemSaving} aria-label={keyLabel(item.key)} theme="normal" step={1} placeholder={t('system.globalSettings.tagInputPlaceholder')} onChange={(value) => { setEditValues((state) => ({ ...state, [item.key]: value })); }} onBlur={(value) => { const parsed = value === '' || value === null || value === undefined ? null : Number(value); if (parsed !== null && !Number.isNaN(parsed)) void persist(item, parsed); }} />


─── apps/web/src/settings/SystemInfoPanel.tsx:93-93 ───
[maintainability · low] 用魔法下标 index === 1 定位「前端版本」行来挂 versionMismatch 告警标签,与 surface.ts 中
systemInfoRows 的行序(当前 0=后端版本、1=前端版本)隐式强耦合:行序或载荷形状一旦调整,漂移告警会静默挂到错误行。建议改为按行标识匹配(row.labelKey)或让
systemInfoRows 显式携带行语义。
另:上一行 Tag 的 theme 使用 `row.tagTone === 'danger' ? 'danger' : row.tagTone === 'warning' ? 'warning' :
'default'` 嵌套三元(违反项目禁用嵌套三元的约定),建议提为映射函数;troubleshootingDocsURL 为硬编码业务 URL,建议收敛到常量/配置统一维护。

-                   {index === 1 && versionMismatch ? <Tag theme="warning" variant="light" size="small" style={{ marginLeft: '8px' }}>{t('system.versionMismatch')}</Tag> : null}
+ // surface.ts 行结构中给前端版本行加标识,如 kind: 'frontend-version';
+ // 此处改为 {row.kind === 'frontend-version' && versionMismatch ? <Tag .../> : null}
+ // Tag theme 映射提为局部函数:
+ const TAG_THEME: Record<string, 'default' | 'danger' | 'warning'> = { default: 'default', danger: 'danger', warning: 'warning' };
+ <Tag theme={TAG_THEME[row.tagTone ?? 'default']} ...>


─── apps/web/src/settings/TenantDeleteZone.tsx:66-66 ───
[style · low] 错误提示 <p> 携带静态内联样式（color/fontSize/margin），违反内联样式规范。settings.td.css §5 已有 .tenant-info
.error-inline 语义类，建议扩展该类（或新增 .delete-space-panel .error-inline）承接这些值。

-         {error ? <p role="alert" className="error-inline" style={{ color: 'var(--td-error-color)', fontSize: 13, margin: '8px 0 0' }}>{error}</p> : null}
+ {error ? <p role="alert" className="error-inline delete-space-panel__error">{error}</p> : null}


─── apps/web/src/settings/TenantMembersPanel.tsx:988-989 ───
[bug · medium] 邀请邮箱输入从旧实现的 `required type="email"`（原生必填+格式校验）改为 TInput `type="text"`，且 submitInvite
内仅有 `!inviteEmail.trim()` 空值守卫、无邮箱格式校验。输入 "abc" 等非法格式将直接提交到后端，属迁移造成的校验回归。建议在提交路径补格式校验（或改回 email
类型语义）。

-           <TInput type="text" className="wk-tenant-invite-input" value={inviteEmail} placeholder={tr('tenantMember.add.emailPlaceholder')}
-             onChange={(value) => setInviteEmail(String(value))} />
+ // submitInvite 内补格式校验：
+ const email = inviteEmail.trim();
+ if (!email || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
+   setInviteError(tr('tenantMember.add.emailInvalid')); // 或行内提示
+   return;
+ }


─── apps/web/src/settings/TenantMembersPanel.tsx:710-710 ───
[bug · medium] invitationTableColumns 的 useMemo 依赖为 [tr, canManage, locale]，遗漏
busy（memberTableColumns 已正确包含）。cell 内 revoke 的 `if (busy) return` 守卫捕获的是过期闭包（busy 恒为 false），busy
期间在邀请表可再次点击撤销重复发请求。同时 revoke 的 TPopconfirm confirmBtn 未在 busy 期间禁用，UI 层也无拦截。建议补 busy 依赖并禁用确认按钮。

-   ]), [tr, canManage, locale]);
+   ]), [tr, canManage, busy, locale]);
+ // 并在 TPopconfirm 上：
+ confirmBtn={{ content: tr('tenantInvitation.revoke.confirm'), theme: 'danger', disabled: busy }}


─── apps/web/src/settings/TenantMembersPanel.tsx:281-281 ───
[maintainability · low] 两表迁 tdesign Table/Pagination 后残留多处死代码：模块级 pageWindow 函数、TablePager 内
maxPage/jump/setJump/commitJump（已被 Pagination 内置 jumper 取代）、revokeConfirmKey/removeConfirmKey 两个只写
state（setter 仅被置 null，值从未读取，且 553/576 行的置 null
调用本身也已不可达）、maxMembersPage（声明后未使用）。建议一并删除，避免每次写入触发无意义重渲染。



─── apps/web/src/settings/TenantMembersPanel.tsx:299-302 ───
[bug · low] Pagination 的 onChange 在 pageSize 变化时可能同时满足两个条件：先调 onPageSize(size) → loadInvitations(1,
size)，再因 pageInfo.current 与旧 page 不一致调 onPage(current) → 再次 loadInvitations。loadInvitations（414 行）无
in-flight 守卫（对比 loadAudit 有完整 auditLoadingRef 守卫），两个并发请求以最后完成者为准，可能出现页码/数据不一致。建议 pageSize 变化分支提前
return，或为 loadInvitations 增加请求守卫。

        onChange={(pageInfo: { current: number; previous: number; pageSize: number }) => {
-         if (pageInfo.pageSize !== pageSize) onPageSize(pageInfo.pageSize);
+         if (pageInfo.pageSize !== pageSize) { onPageSize(pageInfo.pageSize); return; }
          if (pageInfo.current !== page) onPage(pageInfo.current);
        }}


─── apps/web/src/settings/TenantMembersPanel.tsx:770-772 ───
[maintainability · low] 权限说明弹层已改为非受控 TPopup（trigger="hover"，无 visible 绑定），但旧实现的配套状态未同步清理，成为死代码：1)
`permissionsOpen` state（约 335 行）——搜索确认仅剩自身 useEffect 内的 set 调用，无任何 UI 读取；2) `permissionsRef`（约 359
行）——JSX 中已无 `ref={permissionsRef}` 挂载点，`ref.current` 恒为 null；3) 依赖该 ref 的 outside-click/Escape 关闭
useEffect（约 505-515 行）——`permissionsRef.current` 恒 null 使整块分支不可达，effect
无可观察效果，且每次点击文档都会空跑一次监听回调。建议连同 state/ref 一起删除这三块残留。



─── apps/web/src/settings/TenantUserProfileSections.tsx:223-223 ───
[bug · medium] 描述编辑确认按钮新增 `disabled={!descriptionDraft.trim()}`，而旧实现仅 `disabled={saving}`，且
saveDescription 本身允许空描述（tenantPatch(currentName || ' ', descriptionDraft)）。这使租户描述一旦设置就无法从 UI 清空，属
Vue→React 迁移的行为回归（名称必填合理，描述应允许为空）。若非有意收紧，建议恢复仅按保存中状态禁用。

-                       <TButton theme="primary" size="small" loading={savingDescription} disabled={!descriptionDraft.trim()} onClick={() => { void saveDescription(reload); }}>
+ <TButton theme="primary" size="small" loading={savingDescription} disabled={savingDescription} onClick={() => { void saveDescription(reload); }}>


─── apps/web/src/settings/TenantUserProfileSections.tsx:346-346 ───
[style · low] 注释行「UserProfileSection —— T12a TDesign 同构迁移」在同一行重复拼接了两次（复制粘贴笔迹），建议清理为单次。

- // UserProfileSection —— T12a TDesign 同构迁移// UserProfileSection —— T12a TDesign 同构迁移：逐节点复刻
+ // UserProfileSection —— T12a TDesign 同构迁移：逐节点复刻


─── apps/web/src/settings/TenantUserProfileSections.tsx:300-300 ───
[style · low] Progress 的 `style={{ flex: 1 }}` 为静态内联样式，违反内联样式规范；建议下沉到 settings.td.css 的
.usage-control 规则（如 `.tenant-info .usage-control .t-progress { flex: 1; }`）。



─── apps/web/src/settings/settings-toast.tsx:62-64 ───
[style · low] tone→className 使用嵌套三元表达式，违反「禁止嵌套三元」规则；warning 变体样式已在 settings-wrapper.css
齐备。建议改为查表对象，可读且便于扩展新 tone。

-             toast.tone === 'error'
-               ? 'wk-settings-toast wk-settings-toast--error'
-               : toast.tone === 'warning'
+ const TOAST_CLASS: Record<SettingsToast['tone'], string> = {
+   error: 'wk-settings-toast wk-settings-toast--error',
+   warning: 'wk-settings-toast wk-settings-toast--warning',
+   success: 'wk-settings-toast wk-settings-toast--success',
+ };
+ // ...
+ className={TOAST_CLASS[toast.tone]}


─── apps/web/src/settings/settings-wrapper.css:599-599 ───
[maintainability · low] `.wk-settings-panel-heading` 同时在 settings-wrapper.css（此处：margin-bottom
32px、无 sticky/边框）与 settings.td.css S6 段（`.wk-settings-drawer-root .wk-settings-panel-heading`：sticky
+ 底边框 + padding + margin-bottom 1rem）以不同特异性定义。抽屉内 td.css 版恒胜（wrapper 的
margin/布局属性在抽屉场景永不生效），双事实源靠加载顺序与特异性脆弱取胜；wrapper 侧 h2/p 字号规则仍被抽屉外场景依赖。建议明确注释分工或合并为单一来源，避免后续两处漂移。



─── apps/web/src/settings/settings.td.css:3422-3427 ───
[bug · medium] `.hint-popover` 全局类在本文件定义了两次且取值冲突：§4（env-settings，~752 行）为 gap:12px、__text margin 4px
0 0；§14（sandbox，本处）为 gap:4px、__text margin 0。两条规则特异性相同，后者按源码顺序覆盖前者，env 设置提示弹层的间距将被 sandbox
取值回归。两个面板的弹层均 portal 到 body 共用此类名，建议合并为单一事实源（按各自 overlay 类名区分：.env-hint-popover /
.sandbox-hint-popover）。

- .hint-popover {
+ /* §14 改名，避免与 §4 冲突 */
+ .sandbox-hint-popover {
    display: flex;
    flex-direction: column;
    gap: 4px;
    max-width: 340px;
  }


─── apps/web/src/settings/settings.td.css:5349-5352 ───
[bug · medium] `@keyframes wk-sandbox-inventory-enter` 在本文件重复定义且参数不同：§14（~3802 行）为
translateX(18px)/opacity .7，此处 S6 段为 translateX(24px)/opacity 0。同名 keyframes 后定义者生效，§14
的定义成为死代码，实际动画行为取决于文件内出现顺序，维护时极易误改其中一处。建议删除其一，或改名区分（如 wk-sandbox-inventory-enter--drawer）。

+ /* 保留一处定义（建议保留 §14 Vue 原名平移版），删除本段重复：
  @keyframes wk-sandbox-inventory-enter {
    from { transform: translateX(24px); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
  }
+ */


─── apps/web/src/settings/surface.ts:8-9 ───
[maintainability · low] 新增的 `export const uiBuild` 被插入在第 7-8 行 const 声明与第 10 行起的 import 语句之间，把
import 块从模块顶部切开。ESM 的 import 会提升、运行时无碍，但违反 import/first 惯例（ESLint import/first
会直接报错），也破坏本文件既有「declare const → import → const」的组织结构。建议将 uiBuild 的声明（连同其注释）移到所有 import 语句之后。



─── apps/web/src/shared/shared-u.css:29-33 ───
[maintainability · low] 生成器产物未整理：.wk-shared-4 块内 `border-style: solid;` 声明了两次（.wk-shared-8 /
.wk-shared-13 / .wk-shared-15 存在同样的重复）；且 `.wk-shared-main` 规则孤悬在编号块序列（17/18）之外。重复声明本身无害，但该文件头注明'值 =
迁移时 utilities 编码的生效值'，重复项会给后续逐项对照审计带来噪音，建议去重并将 .wk-shared-main 归位到块序列末尾。

+ .wk-shared-4 {
+   min-height: 32px;
+   cursor: pointer;
    border-radius: 6px;
-   border-style: solid;
-   border-width: 1px;
-   border-style: solid;
-   border-color: #dcdcdc;
+   border: 1px solid #dcdcdc;
+   background-color: #fff;
+   padding-inline: 12px;
+   font-size: 13px;
+ }


─── apps/web/src/shared/shared-u.css:172-172 ───
[maintainability · low] .wk-shared-main 的 `padding-block: 40px` 用于覆盖 styles.css 中 .wk-page--std 的
`padding: 48px 20px`（ready 分支原为 py-10 而非 py-12）。两者同为单类选择器、特异性相同，覆盖是否生效完全取决于样式表注入顺序——当前依赖懒加载路由 chunk
的 CSS 晚于入口 styles.css 注入这一隐式事实。一旦打包/测试装配顺序变化，padding-block 会静默回退为 48px。建议提高特异性（或至少加注释声明该级联契约）。

- .wk-shared-main { padding-block: 40px; }
+ /* 覆盖 .wk-page--std 的 padding: 48px 20px（ready 分支原为 py-10）。
+    显式双类选择器提高特异性，不依赖与 styles.css 的注入顺序。 */
+ .wk-page--std.wk-shared-main { padding-block: 40px; }


─── apps/web/src/shared/shared-u.css:147-148 ───
[documentation · low] 此处原 Tailwind 编码为 `break-all`（word-break: break-all），迁移后写成 `overflow-wrap:
anywhere`——两者断行策略不同：break-all 无条件逐字符断行（含空格英文也会劈开单词），anywhere 仅在无其他断点且溢出时才断、并参与 min-content
测量；窄容器下英文标题/消息内容的断行位置是可观察差异。.wk-shared-7 同样被替换。文件头声明"值 = 迁移时 utilities
编码的生效值"并以此为逐项对照审计依据，此处偏离了该契约：要么改回 `word-break: break-all` 保持逐项相等，要么在头注释显式记录这处有意替换。

    white-space: pre-wrap;
-   overflow-wrap: anywhere;
+   word-break: break-all;


─── apps/web/src/shared/wk-legacy.tsx:80-86 ───
[bug · medium] WkDialog 关闭时的栈清理使用 `dialogRef.current`，但 `if (!open) return null` 使 React 在 commit
阶段（mutation phase 卸载 ref）就将其置为 null，而 useEffect 清理在 commit 之后才执行，三元永远走 -1 分支：每次开-关都会向
openDialogStack 残留一个游离 DOM 节点，长会话中无界增长（内存泄漏）。另外，组件以 open=true 首次挂载时（如 onboarding 类弹窗），首次渲染走 inline
分支、mounted effect 后才迁入 portal 导致子树重挂，open effect push 进栈的是已废弃的 inline 节点，栈顶 `!== dialogRef.current`
会使 Esc 关闭的栈顶判定失效。建议在 effect 内捕获元素引用，并将 mounted 纳入依赖。

+   useEffect(() => {
+     if (!open || !mounted) return;
+     const dialogEl = dialogRef.current;
+     if (dialogEl) openDialogStack.push(dialogEl);
+     // ...
      return () => {
        document.removeEventListener('keydown', onKeyDown);
-       const index = dialogRef.current ? openDialogStack.indexOf(dialogRef.current) : -1;
+       const index = openDialogStack.indexOf(dialogEl);
        if (index >= 0) openDialogStack.splice(index, 1);
        restoreRef.current?.focus();
        restoreRef.current = null;
      };
+   }, [open, mounted]);


─── apps/web/src/shared/wk-legacy.tsx:192-194 ───
[bug · medium] `aria-label={String(title)}`：title 的类型是
ReactNode，实际消费方（KnowledgeDocumentDetailPage.tsx:318 的 doc-detail 抽屉）传入的是含嵌套 span 与按钮的 JSX
节点，`String()` 序列化结果是 '[object Object]'；且按 ARIA 优先级，同一元素上 aria-label 会覆盖 aria-labelledby，h2
的正确可访问名被无效值取代，屏幕阅读器对核心抽屉的播报完全失效。已有 `aria-labelledby={titleId}` 指向标题节点，建议直接删除该 aria-label；若确需保留，仅在
title 为 string 时设置。

            aria-labelledby={titleId}
            tabIndex={-1}
-           aria-label={String(title)}
+           aria-label={typeof title === 'string' ? title : undefined}


─── apps/web/src/shared/wk-legacy.tsx:130-130 ───
[bug · medium] WkSheet 的两处 localStorage 访问均无异常防护：挂载 effect 的 getItem 在存储被禁用的环境（部分浏览器隐私模式、受限
iframe——本项目存在 embed 嵌入场景）会直接抛异常导致抽屉挂载失败；stop() 中的 setItem 在配额满时抛 QuotaExceededError
成为事件处理器中的未捕获错误。项目内既有惯例是对存储访问统一 try/catch（如
RulePage.tsx:158、KnowledgeBasesPage.tsx:352、ApiPlaygroundDrawer.tsx:215、FAQPage.tsx:322 均带 'storage
unavailable' 注释），此处应保持一致。

+     try {
-     const saved = Number.parseFloat(window.localStorage.getItem(storageKey) || '');
+       const saved = Number.parseFloat(window.localStorage.getItem(storageKey) || '');
+       if (Number.isFinite(saved)) setPanelWidth(Math.max(minWidth, Math.min(maxWidth, saved)));
+     } catch { /* storage unavailable */ }
+     // stop() 内同理：
+     // if (storageKey) { try { window.localStorage.setItem(storageKey, String(panelWidthRef.current)); } catch { /* storage unavailable */ } }


─── apps/web/src/shared/wk-legacy.tsx:153-153 ───
[performance · low] 拖宽 effect 的依赖数组包含 panelWidth：拖拽期间每次 mousemove 触发 setPanelWidth → effect 重跑 →
mousemove/mouseup 监听器整体卸载重挂，造成监听器抖动与额外的提交开销。panelWidth 进依赖仅是为了让 stop() 闭包拿到最新宽度以便持久化，用 ref
解耦即可移除该依赖。

-   }, [maxWidth, minWidth, panelWidth, resizable, side, storageKey]);
+   const panelWidthRef = useRef(initialWidth);
+   useEffect(() => { panelWidthRef.current = panelWidth; }, [panelWidth]);
+   // stop() 内改用 panelWidthRef.current；effect 依赖去掉 panelWidth：
+   }, [maxWidth, minWidth, resizable, side, storageKey]);


─── apps/web/src/shared/wk-legacy.tsx:149-152 ───
[bug · medium] WkSheet 拖宽会话在异常路径下不回收，全局 body 样式泄漏：① `beginResize` 设置了 `document.body.style.cursor =
'col-resize'` 与 `userSelect = 'none'`，但本 effect 的 cleanup 只移除 mousemove/mouseup 监听器、不重置这两个 body
内联样式——拖拽进行中组件卸载（父级关闭抽屉、路由切换）时，整页永久残留 col-resize 光标且文本不可选中；② `move` 未校验 `event.buttons`，当用户按住 handle
拖出浏览器窗口后释放（Chrome 等不向页面派发 mouseup）时 `resizeRef` 悬挂，之后未按键的 mousemove 仍会持续修改面板宽度并同样卡住 body 样式。建议
cleanup 中在拖拽进行时（resizeRef 非空）重置 body 样式并清空 resizeRef，move 内补 `if (!event.buttons) { stop(); return;
}` 兜底丢失的 mouseup。

+     const releaseBody = () => {
+       document.body.style.cursor = '';
+       document.body.style.userSelect = '';
+     };
+     // move 内：if (!event.buttons) { stop(); return; }
      return () => {
        window.removeEventListener('mousemove', move);
        window.removeEventListener('mouseup', stop);
+       if (resizeRef.current) {
+         resizeRef.current = null;
+         releaseBody();
+       }
      };


─── apps/web/src/styles.css:7-7 ───
[maintainability · medium] tdesign-theme.css 在 Web 构建中被打包两次：此处不带 layer() 的直引一份，第 10 行 `@import
"...design-tokens/src/styles.css" layer(theme)` 内部又经新增的 `@import "./tdesign-theme.css"` 带入一份（theme
层）。层叠上 unlayered 副本恒胜，layered 副本在 Web
端是被完全遮蔽的死代码，纯粹增加产物体积；更关键的是同一物理文件在级联中存在两份拷贝，后续任一侧单独修改（例如只改直引这份）会造成两端难以察觉的不一致。建议从
design-tokens/styles.css 移除内部 @import（见对该文件的评论），令 Web 端每个主题文件只引入一次。

+ /* design-tokens/styles.css 移除内部 @import "./tdesign-theme.css" 后，
+    此处直引成为 Web 端唯一一份（unlayered，位于 tdesign.css 之后），
+    消除同文件 unlayered/layered 双份打包与层叠分歧风险。 */
  @import "../../../packages/design-tokens/src/tdesign-theme.css";


─── apps/web/src/styles.css:41-42 ───
[documentation · low] 注释声称壳内全宽覆盖位于 platform-shell.td.css，但实际 `.plat-shell__outlet .wk-page { ...
max-width: none !important; }` 规则在 `apps/web/src/platform/platform-u.css:659`（已核实
platform-shell.td.css 内无任何 wk-page 规则）。功能本身成立，但注释指向错误文件会误导后续维护者到错误位置排查层叠问题，建议更正为 platform-u.css。

-    PlatformShell 壳内由 platform-shell.td.css 的 .plat-shell__outlet .wk-page
+    PlatformShell 壳内由 platform-u.css 的 .plat-shell__outlet .wk-page
     覆盖为全宽滚动（max-width:none!important，层叠与先前壳层 utility 等价）。 */


─── apps/web/vite.config.ts:15-18 ───
[maintainability · low] FRONTEND_VERSION 改读仓库根 ../../frontend/package.json 属跨应用目录耦合：frontend/
缺失（独立检出/子目录裁剪构建）时静默退化为 'unknown'，system 分区前端版本行随之失真且无任何告警，与 T12c「两端版本字符串完全一致」的目标相悖时不易察觉。建议 catch
中输出一次性告警（execSync 分支已有 'unknown' 兜底可一并覆盖），让退化可观测。

-     return frontendPkg.version ?? 'unknown';
    } catch {
+     console.warn('[vite.config] 读取 frontend/package.json 失败，FRONTEND_VERSION 回退为 unknown');
      return 'unknown';
    }


─── docs/design/job-search/prototype/app.css:6-7 ───
[maintainability · low] 末尾追加的补丁块与同文件基础规则自相冲突：基础块声明 .mobile/.mini .details-pane
.evidence-card{display:none} 并配套了一系列 details-pane 移动端压缩规则（表明移动端原设计隐藏证据卡），而此处又覆盖为
display:block；同时文件出现第三个 :root 块（页面背景由 --td-gray-color-3 改为
--td-gray-color-2），并以整组选择器（.prototype-badge、.btn.primary、.journey-hero、.orbit-core
等）成批覆盖前文颜色。同文件内"先隐藏后显示、多重 :root
叠加"的补丁式改写使最终计算样式难以推断——维护者必须通读全文并按源码顺序在脑内求值层叠，才能确定移动端证据卡是否可见及各主题色的实际生效来源。建议把覆盖值直接合并进基础声明（删除
evidence-card 的 display:none 与冗余 :root/颜色规则），保持每个样式单一事实来源；若确需保留主题层，至少补一条注释说明这是刻意分层。



─── docs/design/job-search/prototype/app.js:59-61 ───
[security · medium] pageHeading 的三个参数中唯独 title 未经 esc() 即拼入 innerHTML（为容纳 title-dot 标记）。当前 5
处调用均为文件内写死的静态字面量，暂无实际可利用输入，但该接口形态直接违背文件头部红线注释（"一切 state 派生文本拼入 innerHTML 前必须经 esc()，新增插值点同样必须走
esc()"）：后续调用者一旦传入 state 派生文本（如 state.prompt、岗位标题、用户粘贴的 JD 片段——本原型明确支持粘贴 JD），即形成 XSS 注入点，且
kicker/subtitle 均已转义的不对称设计会让人误以为 title 也是安全的。建议把 title-dot 标记收进 helper 内部、对 title 一律
esc()（各调用点去掉手动拼接的 <span class="title-dot">.</span>；commandView 中内联的 h1 同理）。

  function pageHeading(kicker, title, subtitle) {
-   return '<div class="page-heading"><div class="micro-label">' + esc(kicker) + '</div><h1>' + title + '</h1><p>' + esc(subtitle) + '</p></div>';
+   return '<div class="page-heading"><div class="micro-label">' + esc(kicker) + '</div><h1>' + esc(title) + '<span class="title-dot">.</span></h1><p>' + esc(subtitle) + '</p></div>';
  }


─── internal/modules/career/application.go:402-405 ───
[bug · high] ReconcileApplicationLink 缺少终态守卫：行处于 link_failed（definite rejection）时仍会调用 Find 并把状态翻转为
ready。workbench 侧 FindCareerApplicationTask 仅按 requestID 查找、不校验 application 归属，而 Ensure 的
ErrCareerApplicationTaskConflict 有两个来源——其中"requestID 已绑定其他 application"（applicationTaskReplay 的
ApplicationID/Title 不匹配分支）必然意味着 workbench 存在同 requestID 但属于其他 application 的
task。触发链：CreateApplication 收到 Conflict 后标记 failed 的 updateApplicationLink 失败（该函数无 busy 重试，SQLite
busy 即触发）→ 返回 504 outcome_unknown → 客户端按契约以同一 requestID 恢复调用本方法 → Find 命中他人的 task → 本行的 link_failed
终态被翻转为 ready 并写入他人 TaskID/RunID。这违反文件头注释"link_failed is a definite rejection"与接口注释"Conflict must
stop, not retry"，并造成跨 application 的错误任务关联。

+ 	var receipt ApplicationReceipt
+ 	if err = decodeApplicationReceipt(row.ReceiptBody, &receipt); err != nil {
+ 		return ApplicationReceipt{}, err
+ 	}
+ 	// link_failed 是终态：Conflict 是明确拒绝，恢复端点不得将其翻转为 ready，
+ 	// 更不能把绑定同一 requestID 的其他 application 的 task 关联到本行。
+ 	if receipt.LinkState == ApplicationLinkStateFailed {
+ 		return receipt, nil
+ 	}
  	link, findErr := o.linker.FindCareerApplicationTask(ctx, scope.TenantID, scope.UserID, requestID)
  	if findErr == nil {
  		return o.updateApplicationLink(ctx, scope, requestID, ApplicationLinkStateReady, link)
  	}


─── internal/modules/career/application.go:156-161 ───
[bug · medium] replay 重试路径的 task title 使用未解析的 input ID：首次创建时若 input.OpportunityID 是 merge 前的旧
ID，事务内会解析为 canonical 并以 applicationTaskTitle(canonical) 调用 Ensure（workbench 持久化该 title），行也落在
canonical owner 上。但 linker 首次失败（504）后客户端以相同 input（旧 ID）重试时，本 replay 分支提前返回、未回填
resolvedOpportunityID，外层 Ensure 的 title 变成 applicationTaskTitle(旧ID)。workbench applicationTaskReplay
以 row.Title != intent.Title 判定 Conflict，于是合法恢复重试被误判为终态拒绝、行被标记 link_failed——若首次 Ensure 实际已成功（仅 ready
回写丢失），workbench task 已存在且属于本 application，career 却永久拒绝，两侧状态分叉。replay 时应把 resolvedOpportunityID
回填为已固定证据中的 canonical ID。

  		if e == nil {
  			if prior.Fingerprint != fingerprint {
  				return ErrIdempotencyConflict
  			}
- 			return decodeApplicationReceipt(prior.ReceiptBody, &receipt)
+ 			if e = decodeApplicationReceipt(prior.ReceiptBody, &receipt); e != nil {
+ 				return e
+ 			}
+ 			// 重试必须沿用首次创建时解析的 canonical owner，否则合并场景下
+ 			// Ensure 的 task title 与 workbench 持久化的 title 不一致，会被
+ 			// applicationTaskReplay 误判为 Conflict 并把可恢复的申请终态失败。
+ 			resolvedOpportunityID = receipt.PinnedEvidence.OpportunityID
+ 			return nil
  		}


─── internal/modules/career/career_export.go:293-302 ───
[bug · high] ExportCareer 事务内读取 career_profiles 行时未处理
gorm.ErrRecordNotFound，行缺失会把底层错误原样返回，writeError 无法识别，客户端收到未类型化 500。profile 行只在
mutate（propose/confirm/dismiss/completeIntake）、ClaimUpload、CreateSource/CreateProcessingSource
中创建；ClaimSpace（Open/ImportJD/SearchOnce 的 claim 路径）只写 career_spaces。因此"仅开通空间或仅导入过
JD、从未写档案"的用户调用整空间导出会稳定 500。同包 evaluation.go:156 与 material.go:327 对同一读取均显式容忍缺失并按 revision=0 参与
CAS，此处应保持一致（ExpectedRevision=0 时应可导出空档案）。注意 buildCareerExportArchive 内部（career_export.go:393）的
First(&head) 也需同样容忍缺失，否则修完此处仍会在归档阶段失败。

  		var head profile
  		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
  			Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error
- 		if e != nil {
+ 		if errors.Is(e, gorm.ErrRecordNotFound) {
+ 			head.Revision = 0
+ 		} else if e != nil {
  			return e
  		}
  		if head.Revision != input.ExpectedRevision {
  			return &RevisionConflictError{CurrentRevision: head.Revision}
  		}
  		archive, e := buildCareerExportArchive(tx, s)


─── internal/modules/career/career_export.go:758-763 ───
[bug · high] DeleteCareer 预检查同样未处理 profile 行缺失：仅开通空间或仅导入过 JD（从未写档案、从未上传简历）的用户执行 delete_career
时，First(&head) 返回 gorm.ErrRecordNotFound 被原样透传，writeError 无匹配分支，得到 500
internal。后果是该类用户完全无法删除自己的求职数据（数据主权/合规红线被阻断）。同包 evaluation.go:156、material.go:327 对同一读取均按 revision=0
容忍；修复此处时必须同步修改 finalizeDeletion（career_export.go:1017 附近的 First(&head)），否则预检查放行后 finalize
步骤仍会因行缺失失败，删除永远停在 partial。

  	} else if errors.Is(err, gorm.ErrRecordNotFound) {
  		var head profile
  		if err = o.db.WithContext(ctx).
  			Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error; err != nil {
+ 			if !errors.Is(err, gorm.ErrRecordNotFound) {
- 			return CareerDeletionReceipt{}, err
+ 				return CareerDeletionReceipt{}, err
+ 			}
+ 			head.Revision = 0
  		}


─── internal/modules/career/career_export.go:821-830 ───
[bug · medium] DeleteCareer 的恢复路径没有任何互斥：两个携带同一 requestID 的并发调用（如客户端超时重发）都会读到 partial/deleting
记录、并发执行 runDeletionSteps（外部副作用重复执行——导出对象删除、source Release、Workbench 投影移除，所幸端口各自幂等），并且末尾 Updates
无条件覆盖 status/state_body/receipt_body。若实例 A 已成功 finalize 并写入终态 deleted，实例 B 某步瞬时失败后执行本
Updates，会把终态回退为 partial，receipt 出现状态倒退，客户端据此重试还会再次 bump revision。建议至少给终态写入加前置条件（status <> deleted
时不允许被覆盖回 partial），更完整的做法是给删除记录加租约/claim 防止并发恢复。

  	if err = o.db.WithContext(ctx).Model(&careerDataDeletionRecord{}).
- 		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
+ 		Where("tenant_id=? AND user_id=? AND request_id=? AND status <> ?", s.TenantID, s.UserID, input.RequestID, DeletionStatusDeleted).
  		Updates(map[string]any{
  			"status":       status,
  			"state_body":   string(mustJSON(execution)),
  			"receipt_body": string(mustJSON(receipt)),
  			"updated_at":   time.Now().UTC(),
  		}).Error; err != nil {
  		return CareerDeletionReceipt{}, err
  	}


─── internal/modules/career/career_export.go:617-620 ───
[bug · low] sectionCount 在 Count 出错时静默返回 0。CareerDeletionBoundary 是删除前的确认界面数据源：数据库故障时各分区显示为"0
条"，用户可能据此误以为空间无数据而确认整空间删除，错误与"真的没有数据"完全不可区分。建议失败时返回错误（让 boundary 端点 5xx）或以负数/显式标记区分"计数不可用"，至少记录日志。

  		if err := query.Count(&total).Error; err != nil {
- 			return 0
+ 			slog.Warn("career deletion boundary count failed", "table", table, "error", err)
+ 			return -1
  		}
  		return int(total)


─── internal/modules/career/career_export.go:775-782 ───
[bug · medium] DeleteCareer 的 Create 竞态回退不校验 fingerprint：并发两个相同 requestID 但不同 expectedRevision 的
delete_career，后者 Create 撞唯一索引后直接 FindCareerDeletion 返回前者的删除回执，同 requestID 不同内容本应返回
ErrIdempotencyConflict（预读分支和同包 CreateApplication/ExportCareer 的竞态回退均做了 fingerprint
校验，此处为不一致遗漏）。回退时应在读取记录后比对 fingerprint，不匹配则拒绝。

  		if err = o.db.WithContext(ctx).Create(&record).Error; err != nil {
  			if isReceiptRaceError(err) {
- 				if replay, lookupErr := o.FindCareerDeletion(ctx, input.RequestID); lookupErr == nil {
- 					return replay, nil
+ 				var raced careerDataDeletionRecord
+ 				if e := o.db.WithContext(ctx).
+ 					Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
+ 					First(&raced).Error; e == nil {
+ 					if raced.Fingerprint != fingerprint {
+ 						return CareerDeletionReceipt{}, ErrIdempotencyConflict
+ 					}
+ 					if raced.Status == DeletionStatusDeleted {
+ 						return decodeCareerDeletionReceipt(raced.ReceiptBody)
+ 					}
  				}
  			}
  			return CareerDeletionReceipt{}, err
  		}


─── internal/modules/career/career_export.go:639-639 ───
[bug · medium] CareerDeletionBoundary 的 InSpace 披露与 careerPurgeTables
不一致，遗漏多张将被清除的个人数据表：career_evaluations（三值资格判断结论）、career_source_revisions（上传简历的文件名/digest/提取文本，且原件在
purge 步骤经 sourceUploadReleaser 被物理删除）、career_data_exports（含全量个人数据的完整导出归档）均无对应 section（简历相关仅
usage_reservations 的"额度账本"描述）。该视图是删除确认界面的数据源，函数注释承诺"exactly what will be deleted
in-space"，用户会在不知"简历原件、资格判断历史、导出归档将被删除"的情况下确认整空间删除，构成知情披露缺口。建议补齐与 purge 表清单对应的 section（或至少覆盖含个人数据的表）。

  			{Section: "workbench_tasks", Description: "Workbench 侧申请任务投影（经删除端口移除）", Count: sectionCount("career_applications", "task_id <> ''")},
+ 			{Section: "evaluations", Description: "岗位资格判断记录（含判断结论与证据引用）", Count: sectionCount("career_evaluations", "")},
+ 			{Section: "sources", Description: "上传的简历来源记录（提取文本；原件随删除一并物理释放）", Count: sectionCount("career_source_revisions", "")},
+ 			{Section: "data_exports", Description: "历史完整导出归档（含档案、岗位、申请与投递的全量副本）", Count: sectionCount("career_data_exports", "")},


─── internal/modules/career/handler.go:45-47 ───
[maintainability · low] ExportSigningKeyFromEnv 返回的 keyErr
被完全吞掉：环境变量"未配置"（ErrExportSigningKeyMissing，可接受的降级）与"配置了非法值"（hex 解码失败或不足 32 字节，rendering.go:79-82
返回明确 error）走同一分支静默跳过。密钥配置错误时服务照常启动，直到用户签发下载授权才以 501 失败，且无任何日志区分两种情况，运维排障成本高。至少应对非
ErrExportSigningKeyMissing 的错误记录警告日志（或将非法配置作为启动错误）。

  	if key, keyErr := ExportSigningKeyFromEnv(); keyErr == nil {
  		o.SetExportSigningKey(key)
+ 	} else if !errors.Is(keyErr, ErrExportSigningKeyMissing) {
+ 		slog.Warn("career export signing key invalid", "error", keyErr)
  	}


─── internal/modules/career/handler.go:1631-1633 ───
[bug · low] 客户端完全控制 multipart part 的 Content-Type 头，此处仅 TrimSpace 后直接写入 sourceRevision.MIMEType（gorm
size:128）。超长 MIME 字符串在 PostgreSQL 上触发 "value too long for type character varying(128)" 插入错误，经
ClaimUpload→Create 路径以未类型化 500 暴露，而非 400 invalid_request。文件名侧已通过 SafeFileName（≤255）约束，MIME
侧应做等价的长度/格式防护（与 ID 侧 maxApplicationRequestIDLen 的前置拒绝模式一致）。

  	digestBytes := sha256.Sum256(data)
  	digest := hex.EncodeToString(digestBytes[:])
  	declaredMIME := strings.TrimSpace(header.Header.Get("Content-Type"))
+ 	if len(declaredMIME) > 128 {
+ 		writeError(c, ErrInvalidRequest)
+ 		return
+ 	}


─── internal/modules/career/profile_intake.go:411-416 ───
[maintainability · low] `if row.ResourceRef == "" { row.ResourceRef = "" }` 是无任何效果的死代码（自赋值）。它紧邻上面的
cleanup_pending_ 前缀分支，看起来像是本想做某种清理（例如释放后清空引用）却写成了自赋值。若确无意图请直接删除，避免误导后续维护者以为存在清理语义。

  			if row.ResourceRef != "" {
  				row.ErrorCategory = "cleanup_pending_" + row.ErrorCategory
- 			}
- 			if row.ResourceRef == "" {
- 				row.ResourceRef = ""
  			}


─── internal/modules/career/reconciliation.go:181-185 ───
[bug · medium] collectIdentityEvidence 吞掉了 snapshots/observations
的查询错误并返回空证据,而它唯一的调用点(ReconcileOpportunities 事务内 301-302 行)会把空证据直接喂给
sufficientIdentityEvidence:任一次瞬时读故障(锁超时、连接抖动)都会让本应合并的记录被判定为 side_by_side,并为该 request ID
持久化终局回执——此后同一 request ID 永远回放这个错误决策,违背"证据充分才合并"的冻结语义。建议让 collectIdentityEvidence 返回
(IdentityEvidence, error) 并在调用点上抛,使读失败导致整个事务回滚,由调用方以同一 request ID 重试,而不是固化一个证据为空的终局决策。同类吞错还有
mergedInto/requirementsChanged/containsExpiryMarker,但它们只影响读时标注、下次重算可恢复,严重性低得多;合并路径是唯一会把吞错变成不可变事实的位置。

  	var snapshots []opportunitySnapshot
  	if err := tx.Where("tenant_id=? AND user_id=? AND opportunity_id=?", scope.TenantID, scope.UserID, opportunityID).
  		Order("acquired_at DESC, id DESC").Find(&snapshots).Error; err != nil {
- 		return evidence
+ 		return evidence, fmt.Errorf("collect identity evidence: %w", err)
  	}
+ // 签名改为 (IdentityEvidence, error),调用点:
+ //   targetEvidence, err := collectIdentityEvidence(tx, s, input.TargetID)
+ //   if err != nil { return err }


─── internal/modules/career/reconciliation.go:399-405 ───
[bug · medium] runImportTransaction 重试耗尽(6 秒 operationCtx 先到期)后返回的裸 SQLite busy 错误,在父 ctx 无 Err()
时会穿透到 `return ReconcileReceipt{}, err`:客户端拿到的是默认 500 + "database is locked"
内部报文,而不是本模块所有其他写路径(progress.go writeProgress、search_once.go commitSearch、reminder.go)统一使用的
OutcomeUnknownError(带 requestId、明示同 request ID 可恢复)。事务已回滚、回执必然不存在,replay 分支查不到后没有任何 busy
归一化,与既有幂等恢复契约不一致。建议在尾部与 writeProgress 对齐:busy 耗尽即返回类型化未知结果。

- 		if ctx.Err() != nil {
+ 		if ctx.Err() != nil || isSQLiteBusy(err) {
+ 			// busy 重试耗尽:事务已回滚,同 request ID 重放是唯一恢复路径。
  			return ReconcileReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
  		}
  		return ReconcileReceipt{}, err
  	}
  	return receipt, nil
  }


─── internal/modules/career/reconciliation.go:676-678 ───
[performance · low] SourceCoverage 通过 GET /career/coverage 直接暴露,每次调用把该 scope 下全部
opportunityObservation 行整表载入内存聚合,再全表加载所有 opportunitySnapshot 并逐行 json.Unmarshal 解析 Extracted
以提取城市——空间数据随观察/快照增长无界放大,读取接口成本线性上升且包含大量一次性 JSON 反序列化。观察侧聚合可下推为 GROUP BY source_kind, source_label 带
COUNT(*)/MAX(acquired_at);城市侧至少可先以 SQL 过滤/收敛候选(如 SQLite json_extract 或仅对近期快照解码),避免全量快照正文加载。当前
registry 为空、数据量小,问题不紧急,但这是唯一无分页的全量读端点,建议在数据增长前收口。

- 	var snapshots []opportunitySnapshot
- 	err = o.db.WithContext(ctx).
- 		Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Find(&snapshots).Error
+ // 观察侧聚合下推:
+ //
+ // type srcAgg struct {
+ // 	SourceKind string
+ // 	SourceLabel string
+ // 	Observations int64
+ // 	LastCheckedAt time.Time
+ // }
+ // o.db.WithContext(ctx).Model(&opportunityObservation{}).
+ // 	Select("source_kind, source_label, COUNT(*) AS observations, MAX(acquired_at) AS last_checked_at").
+ // 	Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).
+ // 	Group("source_kind, source_label").Scan(&srcAggs)
+ //
+ // 城市侧同样优先用聚合(SQLite json_extract / PG jsonb)而非全量加载快照正文。


─── internal/modules/career/reconciliation.go:204-206 ───
[performance · low] collectIdentityEvidence 的快照循环早退条件是死条件:该循环内从不给 evidence.JobCode 赋值(JobCode
只在函数后半的 observations 循环里通过 jobCodeFromRef 赋值),所以 `evidence.JobCode != ""` 在此恒为假,break
永远不会执行。结果是即使最新的快照已经凑齐 Title/Company/Location/Batch 四维证据,循环仍会把该 opportunity 的全部快照逐个
json.Unmarshal(Extracted)——而快照按 acquired_at DESC 排列、take 只取首个 known 值,后续解析全部是空转。每次
ReconcileOpportunities 对 target 和 candidate
各调用一次,快照数随重复观察增长,证据收集成本随之线性上升。与已确认的『吞错误』问题不同,这是循环退出条件的意图失效。建议把 JobCode 的求值移出 break 条件(例如循环条件只检查
SnapshotID/Title/Company/Location/Batch 五项,或先完成 observations 的 SourceRef/JobCode 收集再进入快照循环),使早退真正生效。

- 		if evidence.JobCode != "" && evidence.SnapshotID != "" && evidence.Title != "" && evidence.Company != "" && evidence.Location != "" && evidence.Batch != "" {
+ 		// JobCode comes from the observations loop below, so the early-exit
+ 		// condition must not require it here.
+ 		if evidence.SnapshotID != "" && evidence.Title != "" && evidence.Company != "" && evidence.Location != "" && evidence.Batch != "" {
  			break
  		}


─── internal/modules/career/reminder.go:309-311 ───
[bug · low] 按 (source_kind, source_id) 的 dedupe 查找未加锁:并发两个不同 request ID 绑定同一 source 的事务在 READ
COMMITTED 下都读不到对方未提交的 todo,双双走到 Create,败者触发 career_reminder_scope_source 唯一约束冲突。该冲突会被
isReceiptRaceError(office.go:836,匹配 ErrDuplicatedKey/"unique constraint")归入 ambiguous 分支,但败者自己的
request ID 回执不存在,replay 未命中后既不命中 busy 重试也不命中 OutcomeUnknown,最终 `return ReminderReceipt{}, txErr`
把裸唯一冲突错误交给 handler → 500。同 request ID 重试虽可恢复,但首次响应应是干净的 Deduplicated 回执。建议:对 todo 插入的唯一冲突按 source
维度重读既有 todo 并落一条 dedupe 回执返回,而不是把约束错误上抛。

- 		var existing reminderRecord
- 		e = tx.Where("tenant_id=? AND user_id=? AND source_kind=? AND source_id=?",
- 			s.TenantID, s.UserID, input.SourceKind, input.SourceID).First(&existing).Error
+ // 在 SetReminder 的 ambiguous 分支 replay 未命中后,针对 todo 唯一冲突做一次按 source 维度的去重重读:
+ //
+ // if isReceiptRaceError(txErr) {
+ // 	if row, e := o.latestReminderBySource(ctx, s, input.SourceKind, input.SourceID); e == nil && row.ID != "" {
+ // 		// 对端已建 todo:落 dedupe 回执并返回,而非上抛约束错误
+ // 	}
+ // }
+ // 或在 attemptReminderWrite 捕获 Create 的唯一冲突后在同事务内重读 existing 并走既有 dedupe 落回执路径。


─── internal/modules/career/rendering.go:111-113 ───
[security · low] ExportedFile.ObjectKey 携带 json tag,而 PublishMaterialHandler 与
ListMaterialExports(handler.go:1008/1026)把 ExportReceipt 原样 c.JSON 下发,文件服务内部对象键(如 local://
存储布局与目录结构)会直接进入客户端契约。下载仍需 HMAC 授权,无越权,但属于不必要的内部实现细节外泄,且一旦前端依赖该字段就很难再收回。生产代码读取对象键全部走
materialExportRecord 的 PDFObjectKey/DOCXObjectKey 列,回执 JSON 中的 objectKey
无人消费(仅测试直接读它),可将该字段从序列化中剔除而不影响发布/回放/下载链路。

  	ContentDigest string `json:"contentDigest"`
- 	ObjectKey     string `json:"objectKey,omitempty"`
+ 	ObjectKey     string `json:"-"` // 内部存储键不下发;持久键在 materialExportRecord 的 PDF/DOCXObjectKey 列
  	FileDigest    string `json:"fileDigest,omitempty"`


─── internal/modules/career/search_rule.go:515-521 ───
[maintainability · low] 任一规则的 triggerRulePeriod 返回错误(最现实的是 SearchOnce 拿到的 head.Revision 与并发档案确认竞争导致的
RevisionConflict,或配额 gate 非拒绝类错误)即 return nil, err 中断整批:排在其后的到期规则本轮全部不执行且无任何失败记录。虽然失败规则未推进
next_due_at、下轮 tick 会重试,但单规则瞬时故障被放大为整批推迟,且调用方无法从返回值区分是哪条规则失败。该 seam
是冻结的调度契约("谁调用谁持有调度"),一旦接入生产调度方此语义会直接影响可用性。建议逐规则容错:失败规则记录一个 failed 状态的 RuleRunSummary(或带错误的
summary),继续执行其余到期规则,最后聚合返回。

  	for _, rule := range due {
  		outcome, triggerErr := o.triggerRulePeriod(ctx, s, rule, now)
  		if triggerErr != nil {
- 			return nil, triggerErr
+ 			// 单规则失败不阻断整批:记录失败摘要并继续,下轮 tick 自然重试。
+ 			outcomes = append(outcomes, RuleRunSummary{
+ 				RuleID: rule.ID, Period: rule.LastPeriod + 1, Status: "failed",
+ 			})
+ 			slog.Warn("career rule trigger failed", "rule_id", rule.ID, "error", triggerErr.Error())
+ 			continue
  		}
  		outcomes = append(outcomes, outcome)
  	}


─── internal/modules/career/usage.go:336-342 ───
[bug · low] 预占释放条件与对外契约不完全一致：reconcileUsageTx 只在 searchRecord 完全不存在（ErrRecordNotFound
分支）且预占租约过期时才释放；若 claimSearchRequest 已创建搜索记录（claim 体）但进程在 commitSearch 前崩溃且用户不再用同一 requestID
重试，该预占将保持 reserved 状态直到月末滚动，持续计入 usageTotalsTx 的余额。而 usageAdmissionConditions
对用户承诺"已预占但从未执行的请求在租约过期后自动释放，不占余额"——此场景下用户视角即"从未执行"却被占用了额度，与展示文案相悖（代码注释中的保守取舍只覆盖了"从未 claim"路径）。建议：在
err==nil 分支中，当 decode 出的是 claim 体且搜索记录自身的 claim 租约已过期时也执行释放——该释放是接管安全的，因为 SearchOnce 在任何 claim
接管前都会先走 AdmitSearch，经 reviveReleasedReservationTx
重新预占，不会产生免费运行；若维持现状，则应修正对用户展示的条件文案，明确"已开始但中断的运行会占用本期额度直至月末"。影响有界（每次弃用占 1/50 单位、月末自愈、方向保守），故定为低危。

  		case err == nil:
- 			receipt, _, decodeErr := decodeSearchBody(search.ReceiptBody)
+ 			receipt, claim, decodeErr := decodeSearchBody(search.ReceiptBody)
  			if decodeErr != nil || receipt == nil {
  				// The search is still claiming or in flight: the reserved
- 				// unit keeps holding until the run turns terminal.
+ 				// unit keeps holding until the run turns terminal. A claim
+ 				// whose own lease expired can no longer complete without a
+ 				// takeover, and every takeover re-runs AdmitSearch first
+ 				// (reviving a released reservation), so releasing here never
+ 				// undercounts a run that later executes.
+ 				if search.LeaseUntil != nil && search.LeaseUntil.Before(now) &&
+ 					(row.LeaseUntil == nil || row.LeaseUntil.Before(now)) {
+ 					if updateErr := tx.Model(&usageReservationRecord{}).
+ 						Where("id=? AND status=?", row.ID, usageStatusReserved).
+ 						Updates(map[string]any{"status": usageStatusReleased, "lease_until": nil}).Error; updateErr != nil {
+ 						return updateErr
+ 					}
+ 				}
  				continue
  			}


─── internal/modules/career/usage.go:395-400 ───
[documentation · low] UsageEstimate 的注释声称"预估是免费读取且从不改动账本(never mutates the ledger)",但该方法第 416 行调用了
o.reconcileUsage(ctx, s),其内部 reconcileUsageTx 会对 career_usage_reservations 执行 UPDATE(将 reserved 迁移为
settled 或 released)。惰性 reconcile 本身是设计意图(reconcileUsage 注释明确说"在读取路径上惰性驱动"),但此处注释与实际行为相矛盾——该注释描述的是
T21 冻结的对外契约,后续维护者若据此假设 UsageEstimate
为纯只读(例如对其做无锁并发调用、缓存或幂等重试假设),可能得出错误结论。建议修正注释,例如:"预估读取本身免费,但会像其它读取路径一样惰性驱动预留的 settle/release
状态迁移,不产生新预占"。



─── internal/modules/workbench/service/workbench/application_task.go:180-183 ───
[maintainability · low] session.EngineType "builtin" 与同一事务内创建的 run.engine_type "trpc"(第 192
行)是全库首次出现的混搭:既有代码中 trpc 引擎的 run 其 session 一律为 EngineType "trpc"(如
agent_run_blackbox_test.go:71、steer_durable_test.go:69),而消费方都按 session.engine_type
路由——session_agent_qa.go:161 与 steer.go:596 以 session.EngineType == "trpc" 决定走 trpc
网关,agent_run_tools.go:131-133 的 lockToolRun 要求 sessions.engine_type = 'trpc' 且 active_agent_run_id
匹配才能写工具日志。当前占位任务不会触发这些路径(无 chat、无工具调用、waiting_user 不进 recovery
扫描),未发现立即故障;但未来若给申请任务接入交互/续跑,这对不一致的取值会成为隐患。建议统一为 "trpc"/"trpc",或在此处加注释说明为何 session 用 builtin 而 run 用
trpc。



─── internal/modules/workbench/service/workbench/application_task.go:125-129 ───
[bug · medium] ensureWithRetry 重试耗尽后的恢复 Find 直接返回 link，缺少与 ensureOnce 内部恢复路径一致的 intent
内容校验。ensureOnce 内的两处恢复（按 requestID 命中走 applicationTaskReplay；按 applicationID 命中报
ErrApplicationTaskConflict）都保证了「同 requestID 必须绑定相同内容」，且接口文档（career_application_task.go）明确
ErrCareerApplicationTaskConflict 是 definite rejection——request ID 绑定到不同内容时调用方必须停止。但这条恢复路径：若并发 twin
以相同 requestID、不同 ApplicationID/Title 抢先提交（SQLite 持续 busy 或 PG 唯一索引等待窗口下 3 次 attempt 全部 race 失败、随后
twin 提交，可达），FindCareerApplicationTask 会把 twin 的 link 当作成功返回。Career 侧（application.go:345-350）拿到 nil
error 后会立即 updateApplicationLink 置 ApplicationLinkStateReady 并持久化该 link——application A 的 receipt 被写成
application B 的 task，错位是持久化的，违背「一个岗位+批次只有一个申请和 Task」红线。既有测试
TestEnsureApplicationTaskRetryExhaustionRecoversDurableTwin 只覆盖了 intent
一致的场景，TestEnsureCareerApplicationTaskConcurrentChangedIntentIsTypedConflict
的时序不落入此路径，因此未被暴露。建议改为行级查询 + applicationTaskReplay 校验：

- 	if link, err := c.FindCareerApplicationTask(ctx, tenantID, ownerID, intent.RequestID); err == nil {
- 		return link, nil
- 	} else if !errors.Is(err, ErrApplicationTaskNotFound) {
+ 	row, found, err := findApplicationTask(c.db.WithContext(ctx), tenantID, ownerID, intent.RequestID, false)
+ 	if err != nil {
  		return interfaces.CareerApplicationTaskLink{}, err
+ 	}
+ 	if found {
+ 		// 与 ensureOnce 内部恢复保持同一契约：durable 行必须匹配本次 intent，
+ 		// 内容不同是确定性的 Conflict，而不是错误的成功。
+ 		return applicationTaskReplay(row, intent)
  	}


─── internal/modules/workbench/service/workbench/application_task_removal.go:56-63 ───
[bug · low] READ COMMITTED 下三条 DELETE 的子查询在各自语句开始时取新快照,与事务开头的 Find 快照可能不一致:(1) 若某新映射在 sessions
DELETE 与 agent_runs DELETE 之间提交,后者的子查询会看到它并删除其 run,随后 mappings DELETE 又删除映射,但前者的子查询没看到它——留下一个无
run、无映射的孤儿 session 行(带申请标题的隐私残留,与 T22 "导出删除需撤销相关 Task"的红线相悖),且幂等重试(找不到映射)永远清不掉它;(2) removed 审计列表来自
Find 快照,与子查询实际删除的集合存在漂移。窗口虽小(同 owner 在删除事务进行中并发 Ensure),但修复很直接:三条 DELETE 都改为按步骤 1 捕获的 rows 显式 ID
列表删除,使"审计集合 = 实际删除集合",并发晚到的新映射留给幂等重试。

- 		if err := tx.Exec(
- 			`DELETE FROM sessions WHERE tenant_id = ? AND id IN (
- 				SELECT task_id FROM workbench_application_tasks
- 				WHERE tenant_id = ? AND owner_id = ? AND origin = ?)`,
- 			tenantID, tenantID, ownerID, careerApplicationTaskOrigin,
- 		).Error; err != nil {
+ 		taskIDs := make([]string, 0, len(rows))
+ 		runIDs := make([]string, 0, len(rows))
+ 		for _, row := range rows {
+ 			taskIDs = append(taskIDs, row.TaskID)
+ 			runIDs = append(runIDs, row.RunID)
+ 		}
+ 		if err := tx.Exec(`DELETE FROM sessions WHERE tenant_id = ? AND id IN ?`, tenantID, taskIDs).Error; err != nil {
+ 			return err
+ 		}
+ 		if err := tx.Exec(`DELETE FROM agent_runs WHERE tenant_id = ? AND run_id IN ?`, tenantID, runIDs).Error; err != nil {
  			return err
  		}


─── package.json:0-0 ───
[test · medium] 新增的 glob `packages/i18n/test/*.test.tsx` 在仓库中没有任何匹配文件（packages/i18n/test 下只有
*.test.ts，共 19 个文件，不存在 .test.tsx）。该条目要么是误加，要么目标路径写错：在 POSIX shell 下未匹配的 glob 会以字面量
`packages/i18n/test/*.test.tsx` 传给 tsx，取决于 tsx/Node 版本可能直接报 "找不到文件" 导致整个 test:shared 启动即失败；即使 Node
按空 glob 处理，也是一条无效的哑配置，误导后续维护。建议删除该 glob，或改为实际存在 .test.tsx 测试文件的路径。



─── package.json:19-19 ───
[test · low] typecheck:shared 枚举了所有共享包的核心源文件，但本次未把新建共享包 career-core 的
packages/career-core/src/contracts.ts、packages/career-core/src/desk.ts 加入（test:shared
却已纳入该包测试）。contracts.ts 含大量类型窄化断言（`value as CareerReceipt`、`value as unknown as Evaluation` 等），只有运行
tsx 测试、没有 --strict tsc 检查的话，类型不匹配只能靠运行时兜底。虽然 web 端 tsc 会经相对导入间接覆盖到，但与其它所有包的枚举约定不一致，建议补上这两个入口。



─── packages/api-client/src/career.ts:2-2 ───
[maintainability · medium] career.ts 与 index.ts:211 通过相对路径 '../../career-core/src/contracts.ts' 跨包直连
career-core 的源文件，但 packages/api-client/package.json 的 dependencies 并未声明 @weknora/career-core（该包已收录进
pnpm-workspace 且 exports 已定义 './contracts'）。本包其余跨包导入（@weknora/contracts、@weknora/domain）均遵循包名导入 +
workspace:* 声明的惯例，此处却绕过了 career-core 的 exports 契约直达内部文件：依赖图上 api-client 对 career-core
的依赖处于未声明状态，任何依赖图工具、目录重构（如拆分 contracts.ts）或将 api-client 抽离/单独构建都会使该导入静默断裂且无编译期契约保护。建议改为包名导入并在
package.json 中声明 workspace 依赖。

- import { decodeCareerReceipt, decodeCareerSources, decodeCareerUpload, decodeEvaluation, decodeEvaluationReceipt, decodeOpportunityReceipt } from '../../career-core/src/contracts.ts'
+ import { decodeCareerReceipt, decodeCareerSources, decodeCareerUpload, decodeEvaluation, decodeEvaluationReceipt, decodeOpportunityReceipt } from '@weknora/career-core/contracts'
+ // 并在 packages/api-client/package.json 的 dependencies 中新增：
+ // "@weknora/career-core": "workspace:*"


─── packages/api-client/src/career.ts:1242-1243 ───
[maintainability · medium] open()/list() 将响应直接 as CareerView 断言返回（changes() 的 CareerChangeSet
同理），未做任何运行时校验——与本文件其余所有端点的 fail-closed 解码纪律不一致，甚至导出档案解码路径都特意用 decodeExportedProfile 重校验了完全相同的
profile 形状（revision + facts + proposals）。一旦后端 CareerView 结构漂移，未校验数据将直达 UI 状态层。同文件已有现成的形状校验器，建议直接复用。

-   async open(signal?: AbortSignal): Promise<CareerView> { return await request({ method: 'GET', path: '/api/v1/career/open', ...(signal ? { signal } : {}) }) as CareerView },
-   async list(signal?: AbortSignal): Promise<CareerView> { return await request({ method: 'GET', path: '/api/v1/career/list', ...(signal ? { signal } : {}) }) as CareerView },
+   async open(signal?: AbortSignal): Promise<CareerView> { return decodeExportedProfile(await request({ method: 'GET', path: '/api/v1/career/open', ...(signal ? { signal } : {}) })) },
+   async list(signal?: AbortSignal): Promise<CareerView> { return decodeExportedProfile(await request({ method: 'GET', path: '/api/v1/career/list', ...(signal ? { signal } : {}) })) },
+   // changes() 同理需要一个 CareerChangeSet 解码器


─── packages/api-client/src/career.ts:262-262 ───
[maintainability · low] RuleRunView.failureCode 的类型声明为宽泛的 string，但 decodeRuleRun 实际只放行
searchFailureCodes 两值集合（'no_vetted_sources' | 'all_sources_unavailable'）——已核实后端 search_rule.go:588 中
run.FailureCode 仅复制 receipt 级失败码，而 receipt 级失败码确实只有这两个。解码保证与类型声明不符，消费方无法据类型穷尽分支。建议将类型收紧为
SearchFailureCode。

- export type RuleRunView = { kind: 'rule_run'; ruleId: string; period: number; requestId: string; status: RuleRunStatus; searchId?: string; failureCode?: string; note?: string; triggeredAt: string }
+ export type RuleRunView = { kind: 'rule_run'; ruleId: string; period: number; requestId: string; status: RuleRunStatus; searchId?: string; failureCode?: SearchFailureCode; note?: string; triggeredAt: string }


─── packages/api-client/src/career.ts:1458-1458 ───
[bug · low] 客户端用 input.note.length（UTF-16 码元数）与 maxSubmissionNoteBytes 比较，而后端 submission.go:208 的
len(input.Note) > 4096 按 UTF-8 字节数计量。中文笔记每字 3 字节但只计 1 个码元：约 1366 个汉字即超 4098
字节，客户端预检放行、服务端拒绝，产生不必要的失败往返与不一致的错误提示。建议按字节计量（如 TextEncoder），与错误文案中 '4096 bytes' 的语义对齐。

-    if (input.note !== undefined && input.note.length > maxSubmissionNoteBytes) throw new TypeError('submission note must not exceed 4096 bytes')
+    if (input.note !== undefined && new TextEncoder().encode(input.note).length > maxSubmissionNoteBytes) throw new TypeError('submission note must not exceed 4096 bytes')


─── packages/api-client/src/career.ts:484-484 ───
[bug · medium] decodeApplicationReceipt 未校验后端冻结不变量「warning 存在 ⟺ qualified === false」。后端
internal/modules/career/application.go:216-224 中 qualified 初始为 true，唯一置 false
的路径（evaluationStatus==ineligible 且 continueDespiteHardFailure）同时写入 warning；253-260 行持久化时两者绑定。当前解码器放行
`qualified: true` 且携带 ineligible warning 的矛盾 payload，UI
可能同时渲染"合格"与"硬性不符"警告——这正触碰"硬性冲突不被匹配分掩盖"的红线，且与本文件对
submission（versionConfirmed↔boundVersion）、export（submittable↔status/files）、deletion（status↔steps）的一致
性校验纪律不一致。

+  // Frozen backend coherence: the hard-failure warning is written exactly
+  // when the qualified metric flips to false (application.go CreateApplication).
+  if ((warning !== undefined) === record.qualified) throw new TypeError('invalid application receipt')
+  return {
+   applicationId: record.applicationId, requestId: record.requestId, linkState: record.linkState as ApplicationLinkState,
+   ...(record.taskId !== undefined ? { taskId: record.taskId } : {}), ...(record.runId !== undefined ? { runId: record.runId } : {}),
    qualified: record.qualified, ...(warning ? { warning } : {}), pinnedEvidence: decodeApplicationPin(record.pinnedEvidence),


─── packages/api-client/src/career.ts:383-384 ───
[bug · medium] decodeSearchOnceReceipt 未校验「status === 'failed' ⟺ failureCode 存在」的冻结绑定。后端
search_once.go:415-416 与 478-479 两处置 SearchStatusFailed 的代码都紧随写入
receipt.FailureCode（no_vetted_sources / all_sources_unavailable），completed 时该字段为零值被 omitempty
省略。当前解码放行「failed 但无 failureCode」（UI 显示搜索失败却无任何原因）和「completed 却携带 failureCode」（成功与失败码并存）两类矛盾
payload。与本文件 receipt 级解码对 reconcile（merged↔suspectedDuplicate）等的一致性纪律不一致，建议补充绑定校验。

    || (failureCode !== undefined && !searchFailureCodes.includes(failureCode as SearchFailureCode))
    || !validTimestamp(record.checkedAt)) throw new TypeError('invalid search receipt')
+  // Frozen backend coherence: only a failed search ever carries the
+  // receipt-level failure code (search_once.go assigns both together).
+  if ((record.status === 'failed') !== (failureCode !== undefined)) throw new TypeError('invalid search receipt')


─── packages/api-client/src/career.ts:958-958 ───
[bug · low] decodePreparationReceipt 分别校验 anchor 与 sources.submittedVersion 的形状，但未校验两者互相一致。后端
preparation.go:521、676 构造 ReceiptBody 时二者是同一个 anchor 变量的复制（SubmittedVersion:
anchor），即「锚定的提交版本」与「引用的提交版本」是一个事实写两份。当前解码放行 anchor 与 submittedVersion 指向不同 materialId/version/digest
的矛盾 payload，UI 将无法判断以哪个为准。建议在构建 sources 前补充等价校验。

+  // The anchor and the cited submitted version are one fact copied twice
+  // (preparation.go writes SubmittedVersion: anchor); disagreement is invented.
+  if (anchor.submissionId !== submittedRecord.submissionId || anchor.materialId !== submittedRecord.materialId || anchor.exportId !== submittedRecord.exportId || anchor.version !== submittedRecord.version || anchor.contentDigest !== submittedRecord.contentDigest) throw new TypeError('invalid preparation sources')
+  sources: {
     submittedVersion: { submissionId: submittedRecord.submissionId as string, materialId: submittedRecord.materialId as string, exportId: submittedRecord.exportId as string, version: submittedRecord.version as number, contentDigest: submittedRecord.contentDigest as string },


─── packages/api-client/src/career.ts:1249-1253 ───
[maintainability · low] upload() 是本文件唯一未做 expectedRevision
前置校验的写方法：createApplication、searchOnce、setRule、editMaterial、confirmMaterial、publishMaterial、revokeMat
erialExport、appendProgress、correctProgress、recordSubmission、generatePreparation、setReminder、exportCa
reer、deleteCareer 全部校验 Number.isSafeInteger 且 >= 0，此处 NaN/负数/小数会被 String() 成 "NaN"/"-1"/"1.5"
直接发给后端，只能靠服务端 400 兜底，且错误信息与本地校验口径不一致。建议对齐。

     if (!requestId.trim()) throw new TypeError('career upload requestId must not be empty')
+    if (!Number.isSafeInteger(expectedRevision) || expectedRevision < 0) throw new TypeError('career upload expectedRevision must be a non-negative integer')
     const body = new FormData()
     body.append('file', file, fileName)
     body.append('requestId', requestId)
     body.append('expectedRevision', String(expectedRevision))


─── packages/api-client/src/career.ts:1429-1431 ───
[maintainability · low] appendProgress / correctProgress 完全未做 note 长度预检，而后端 progress.go:221 以字节计量拒绝
len(note) > maxProgressNoteBytes(4096)。同文件 recordSubmission 对称地做了 note 预检，此处缺失导致中文 note（每字 3 字节，约
1366 字即超限）只能收到后端 400 而无前置提示。建议对齐预检，并按后端语义用字节（TextEncoder）而非 UTF-16 码元计量。

     if (input.occurredAt !== undefined && !validTimestamp(input.occurredAt)) throw new TypeError('progress occurredAt must be an RFC3339 timestamp')
+    if (input.note !== undefined && new TextEncoder().encode(input.note).length > maxProgressNoteBytes) throw new TypeError('progress note must not exceed 4096 bytes')
     if (!Number.isSafeInteger(input.expectedRevision) || input.expectedRevision < 0) throw new TypeError('progress expected revision must be a non-negative integer')
     const body = { requestId: input.requestId, applicationId: input.applicationId, eventType: input.eventType, ...(input.note !== undefined ? { note: input.note } : {}), ...(input.occurredAt !== undefined ? { occurredAt: input.occurredAt } : {}), source: { kind: 'manual' }, expectedRevision: input.expectedRevision }


─── packages/api-client/src/index.ts:211-211 ───
[maintainability · medium] index.ts 对 career-core 契约类型的再导出同样使用相对路径
'../../career-core/src/contracts.ts'，与 career.ts 的问题同源：@weknora/api-client 的公共入口依赖了一个 package.json
中未声明、且绕过其 exports 映射（'./contracts'）的相邻包源文件。外部消费者经 '@weknora/api-client' 入口加载时会随之加载这条脆弱的相对导入。应与
career.ts 一并改为包名导入。

- export type { CareerAction, CareerView, CareerFact, CareerProposal, CareerReceipt, CareerChangeSet, CareerChange, CareerSource } from '../../career-core/src/contracts.ts';
+ export type { CareerAction, CareerView, CareerFact, CareerProposal, CareerReceipt, CareerChangeSet, CareerChange, CareerSource } from '@weknora/career-core/contracts';


─── packages/career-core/src/desk.ts:26-26 ───
[maintainability · low] sameAction 用 JSON.stringify 做结构相等比较，对对象键的插入顺序敏感：同一 action 若经持久化（如小程序 storage
/ localStorage）序列化-反序列化后由调用方按不同字段顺序重建，即使内容完全一致也会被判 retry_payload_mismatch，导致结果未知的写操作无法用同一 requestId
安全重试，违背“未知创建结果用同一 request ID 恢复”的设计意图。当前 Web/小程序调用方都传 desk 内保存的原始对象引用所以未触发，但作为共享库 API
建议改为逐字段比较（或先按固定键序规范化再比较）。

- function sameAction(left: CareerAction, right: CareerAction): boolean { return JSON.stringify(left) === JSON.stringify(right) }
+ function sameAction(left: CareerAction, right: CareerAction): boolean {
+   const keys = new Set([...Object.keys(left), ...Object.keys(right)])
+   return keys.size === Object.keys(left).length && keys.size === Object.keys(right).length && [...keys].every((key) => (left as Record<string, unknown>)[key] === (right as Record<string, unknown>)[key])
+ }


─── packages/career-core/src/desk.ts:22-22 ───
[maintainability · medium] 错误码词表在多处手抄硬编码，且已经开始分叉：contracts.ts 的 decodeCareerError 内部有一份 8 码数组、本函数有一份
6 码确定性列表、retryUnknown 里还有一份 4 码清除列表，小程序 services/career-intent.ts 的 ambiguousOutcome 又复制了一份并已多出
`search_quota_refused`。一旦服务端新增确定性错误码（如配额/风控类 4xx）而只更新了 contracts.ts，desk 侧会把它误判为『结果未知』：send 会设置
unresolved 并走回执探测流程，后续 mutate 全部被 `unresolved_action` 阻塞直到对账。建议在 contracts.ts 导出单一运行时数组（如
`DEFINITIVE_CAREER_ERROR_CODES`）供各处复用，而不是维护四份手抄清单。

-  if (code && ['forbidden', 'revision_conflict', 'idempotency_conflict', 'invalid_request', 'not_found', 'proposal_resolved'].includes(code)) return false
+  if (code && DEFINITIVE_CAREER_ERROR_CODES.includes(code as CareerErrorCode)) return false // 从 contracts.ts 导出的单一词表


─── packages/career-core/src/desk.ts:21-21 ───
[maintainability · low] `TIMEOUT` / `NETWORK_ERROR` / `CANCELLED` 三个字面量在仓库内没有任何生产方（api-client 的
ApiError 回退码是 `HTTP_${status}`，中止错误是 name 为 AbortError 的 DOMException、网络错误是 fetch TypeError，均无 code
属性），这三个分支永远不会命中，属于死代码。实际的中止/网络/超时错误已由末尾的 `status === undefined || status >= 500`
兜底覆盖。建议删除这三个字面量，避免误导后续维护者以为某层会抛出这些码。

-  if (code === 'outcome_unknown' || code === 'TIMEOUT' || code === 'NETWORK_ERROR' || code === 'CANCELLED') return true
+  if (code === 'outcome_unknown') return true


─── packages/career-core/src/desk.ts:165-166 ───
[bug · low] pending→pending 的合并没有像 mergeFact 那样比较 revision：`current.status === 'pending'` 短路了后面的
revision 比较，导致乱序到达的旧 pending 快照（revision 更低）会覆盖本地已应用的更新 pending 状态。触发路径：act 回执已把 proposal 合并到较新
revision，随后一个在途的 syncChanges 响应（其 set.revision 不小于当前 view.revision，通过前置守卫）携带同一 proposal 的较旧 pending
版本，合并后 UI 回退展示过期提案内容。既然上一行已经防了『已决→pending』的回退，pending→pending 也应做同样的 revision 防护。

   if (current.status !== 'pending' && incoming.status === 'pending') return
-  if (current.status === 'pending' || incoming.status !== 'pending' || (incoming.revision ?? 0) >= (current.revision ?? 0)) proposals[index] = incoming
+  if (incoming.status !== 'pending' || (current.status === 'pending' && (incoming.revision ?? 0) >= (current.revision ?? 0))) proposals[index] = incoming


─── packages/design-tokens/src/styles.css:19-23 ───
[maintainability · medium] 在令牌包聚合入口中引入 tdesign-theme.css，会把约 200 行 TDesign
主题及文件尾部的应用级全局组件样式（`.t-input:focus` 覆盖、autofill `!important` 修复、`.doc-link`/`.more-icon`、暗色
`*::-webkit-scrollbar` `!important`）注入到 design-tokens 的所有消费方。已核实 `apps/embed/src/styles.css:4` 也以
`layer(theme)` 引入本文件，而 embed 全程不使用 TDesign——它将无差别继承暗色滚动条强制重着色与 `input:-webkit-autofill` 的
`-webkit-text-fill-color: var(--td-text-color-primary) !important` 等覆盖，属于跨应用样式泄漏；同时这些组件规则与
design-tokens "纯令牌" 的包职责不符（web 端 settings.td.css 等还各自维护了一份作用域化的 .doc-link 副本，形成双源）。建议拆分入口：styles.css
保持纯令牌，tdesign-theme.css（或至少其应用级组件规则部分）通过独立导出/入口由 TDesign 消费方显式引入，embed 等非 TDesign 应用不再被动带入。

- /* TDesign 组件主题（Vue frontend/src/assets/theme/theme.css 原样平移，Phase 1 Task 4）。
-    注意：消费方若将本文件整体 layer() 引入（如 apps/web layer(theme)），这里的
-    --td-* 覆盖会输给 unlayered 的 tdesign.css 默认值——apps/web/src/styles.css 在
-    tdesign.css 之后另有对本文件的不带 layer() 直引，确保覆盖生效。 */
- @import "./tdesign-theme.css";
+ /* TDesign 组件主题改为独立入口（tdesign-theme.css），由 TDesign 消费方显式引入：
+    styles.css 保持纯令牌聚合，避免 embed 等非 TDesign 应用被动带入全局组件覆盖。 */
+ /* （移除此处 @import "./tdesign-theme.css"，在 package.json exports 中新增
+    "./tdesign-theme.css" 子入口；apps/web 已有对该文件的直引，仅需调整路径。） */


─── packages/design-tokens/src/styles.css:19-23 ───
[bug · medium] `@import "./tdesign-theme.css"` 被放在第 3–17 行 `:root { ... }` 样式规则块之后。按 CSS 规范（@import
只能位于 @charset 与空 @layer 语句之后、所有其他规则之前），位于样式规则之后的 @import 应被整体忽略。当前仅因 Vite 的 postcss-import
宽容处理（不校验位置、照样内联）才得以生效；一旦构建管道切换为规范符合的实现（如 Lightning CSS）或浏览器原生处理该文件，整个 TDesign 主题（约 200 行 --td-*
令牌及组件覆盖）会被静默丢弃，且不会有任何报错，排查成本极高。建议将此 @import（连同注释）移到文件顶部 `:root` 块之前；或直接移除该聚合引入（apps/web 已另行直引
tdesign-theme.css，见关于聚合引入污染消费方的问题）。

- /* TDesign 组件主题（Vue frontend/src/assets/theme/theme.css 原样平移，Phase 1 Task 4）。
-    注意：消费方若将本文件整体 layer() 引入（如 apps/web layer(theme)），这里的
-    --td-* 覆盖会输给 unlayered 的 tdesign.css 默认值——apps/web/src/styles.css 在
-    tdesign.css 之后另有对本文件的不带 layer() 直引，确保覆盖生效。 */
+ /* TDesign 组件主题（Vue frontend/src/assets/theme/theme.css 原样平移，Phase 1 Task 4）。 */
  @import "./tdesign-theme.css";
+ 
+ /* 以下为 --wk-* 令牌（保持原有内容不动） */
+ :root {
+   --wk-color-brand: #07c05f; ...


─── packages/design-tokens/src/tdesign-theme.css:0-0 ───
[bug · low] 亮色与暗色两个块中 `--td-line-height-title-large` 均按 `--td-font-size-title-medium`（16px）计算，得到
24px 行高；而 TDesign 官方默认按 `--td-font-size-title-large`（20px）计算为 28px。已核实 Vue 端
`frontend/src/assets/theme/theme.css`（亮色第 1 行与暗色块）存在完全相同的公式，即这是上游笔误被"原样平移"继承。由于
`--td-font-title-large` 消费该值，所有 title-large 文本行高偏挤（24px vs 28px）。虽保持了与 Vue
现网的像素一致，但建议两端一并修正，或在此处注释显式标注"已知上游偏差、刻意对齐"，避免后续被当作迁移错误反复排查。

-   --td-line-height-title-large: calc(    var(--td-font-size-title-medium) + var(--td-line-height-common)  );
+   /* 修正上游笔误：title-large 行高应基于自身字号（20+8=28px）；
+      如刻意保持与 Vue 现网一致，请保留原式并注释说明。 */
+   --td-line-height-title-large: calc(    var(--td-font-size-title-large) + var(--td-line-height-common)  );


─── packages/views/src/chat/chat-copy.ts:1176-1177 ───
[bug · low] 韩语表（ko，第 1177 行）与俄语表（ru，第 1514 行）的 requestInfoEmpty 均为未翻译的英文原文 'No request info
available'，而 zh/en/ja 三表已本地化（'暂无请求信息' / 'リクエスト情報はありません'）。同批新增的 noResult、referencesWebCount 等键在 ko/ru
都有对应译文，说明这两处是遗漏而非策略。该文案经 message-face.tsx RequestInfoButton 空态直接展示，ko/ru 用户会在调试卡片里看到英文。请补译两表（ru 表见第
1514 行同一行文本）。

     requestInfoSentAt: '전송 시간',
- requestInfoEmpty: 'No request info available',
+ requestInfoEmpty: '요청 정보가 없습니다',


─── packages/views/src/chat/composer.tsx:416-416 ───
[bug · high] Enter 双触发：提及菜单打开时按 Enter，handleMentionKeyDown 会 preventDefault 并选中提及项，但随后
handleDraftKeyDown 仍继续执行——shouldSubmitFromKeyboard 不检查 event.defaultPrevented，Enter 判定为 true，草稿非空时
submitDraft() 同步发出。结果是「选中提及项 + 立即发送整条消息」两个动作叠加。旧实现中 Enter 由提及搜索 input 自身的 onKeyDown 消费，不会到达
textarea；迁移后两个 handler 串联在同一事件上，需要短路。建议提及 handler 消费按键后提前返回（或检查 defaultPrevented）。同理
ArrowUp/ArrowDown 也应短路，避免未来在 handleDraftKeyDown 增加快捷键时再次叠加。

-         onKeyDown={(event) => { if (mentionOpen) handleMentionKeyDown(event); handleDraftKeyDown(event); }}
+         onKeyDown={(event) => {
+           if (mentionOpen) {
+             handleMentionKeyDown(event);
+             if (event.defaultPrevented) return; // 提及菜单已消费 Enter/方向键/Esc
+           }
+           handleDraftKeyDown(event);
+         }}


─── packages/views/src/chat/composer.tsx:306-306 ───
[maintainability · medium] mentionQuery 成为只读死状态：提及搜索 input 删除后，setMentionQuery 仅剩
toggleMentions/closeMentions 两处复位为 ''，永远没有任何非空写入——filteredMentionOptions 的 query
过滤恒等于全集，「按关键字过滤知识库」的能力静默失效，t.mentionNoResults 文案也没有消费点了。请二选一：按 Vue 行为在 textarea 输入时同步派生 query（如截取 @
后缀），或删除 mentionQuery 状态与过滤条件（仅保留已选项排除），并同步清理 mentionNoResults 死文案。

-   const filteredMentionOptions = mentionOptions.filter((item) => item.name.toLocaleLowerCase().includes(mentionQuery.trim().toLocaleLowerCase()) && !mentionedItems.some((selected) => selected.id === item.id));
+   const filteredMentionOptions = mentionOptions.filter((item) => !mentionedItems.some((selected) => selected.id === item.id));
+   // 若确认不再支持关键字过滤，同步删除 mentionQuery state 与 t.mentionNoResults 文案


─── packages/views/src/chat/composer.tsx:510-513 ───
[bug · low] kbTipTimer 无卸载清理：mouseenter/mouseleave 各自 clearTimeout，但组件卸载时没有 useEffect 清理挂起的定时器——悬停
300ms 窗口内卸载会留下悬挂 timer 并对已卸载组件 setKbTipOpen(true)。建议补一行卸载清理（与 AnswerToolbar 的 copied timer 同款写法）。

-           <div className="wk-chat-kb-btn-wrap wk-vc-composer-23"
-             onMouseEnter={() => { if (kbTipTimer.current !== null) window.clearTimeout(kbTipTimer.current); kbTipTimer.current = window.setTimeout(() => setKbTipOpen(true), 300); }}
-             onMouseLeave={() => { if (kbTipTimer.current !== null) window.clearTimeout(kbTipTimer.current); kbTipTimer.current = window.setTimeout(() => setKbTipOpen(false), 80); }}
-           >
+   useEffect(() => () => { if (kbTipTimer.current !== null) window.clearTimeout(kbTipTimer.current); }, []);


─── packages/views/src/chat/composer.tsx:528-529 ───
[bug · low] 提及菜单键盘导航的读屏接线丢失：旧实现中搜索 input 带 aria-activedescendant /
aria-controls="wk-chat-mention-options"，选项 id（wk-chat-mention-option-*）也有对应消费；迁移后键盘导航落在 textarea 上，但
textarea 未携带 aria-expanded / aria-controls / aria-activedescendant 任何一项——focus 停留在 textarea、列表以
fixed 弹层渲染时，读屏用户按 ArrowUp/ArrowDown 无法感知当前激活项，按 Enter 选中也没有播报。建议给 textarea 补回 ARIA combobox 关联（至少
aria-activedescendant 与 aria-controls），复用现有选项 id。

-           {mentionOpen ? (
-             <div id="wk-chat-mention-listbox" role="listbox" aria-label={t.mentionKnowledge} className="mention-menu" style={mentionMenuStyle ?? undefined} onClick={(event) => event.stopPropagation()}>
+       <textarea
+         ref={draftRef}
+         value={draft}
+         aria-expanded={mentionOpen}
+         aria-controls={mentionOpen ? 'wk-chat-mention-listbox' : undefined}
+         aria-activedescendant={mentionOpen && filteredMentionOptions.length > 0 ? `wk-chat-mention-option-${filteredMentionOptions[Math.min(activeMentionIndex, filteredMentionOptions.length - 1)].id}` : undefined}
+         onChange={(event) => onDraftChange(event.target.value)}


─── packages/views/src/chat/message-face.tsx:308-310 ───
[bug · high] 点赞/点踩（SP11 反馈）UI 随 FeedbackButtons 删除而整体消失，但回调链原样保留：page.tsx 仍向 MessageList 传
onRateMessage/onRemoveRating/ratingOf（670-672、805-807 行），MessageList 仍在解构这三个参数，AnswerToolbarProps
也声明了这三个回调——但 AnswerToolbar 组件体从未使用它们。这同时构成功能回归（宿主提供评分能力时用户无入口）与三层死代码。请二选一：恢复 AnswerToolbar 中的 👍/👎
按钮（旧 FeedbackButtons 的 toggle 语义），或将三个回调从 AnswerToolbarProps、MessageListProps 与 page.tsx 透传链一并删除。注意
message-list.test.tsx 仍引用 isFeedbackAvailable，删除时需同步清理。

-   onRateMessage?(messageId: string, rating: FeedbackRating): void;
-   onRemoveRating?(messageId: string): void;
-   ratingOf?(messageId: string): FeedbackRating | undefined;
+ {isFeedbackAvailable(props.onRateMessage) ? (
+   <FeedbackButtons copy={copy} message={props.message} ratingOf={props.ratingOf} onRateMessage={props.onRateMessage} onRemoveRating={props.onRemoveRating} />
+ ) : null}
+ // 或：从 AnswerToolbarProps / MessageListProps / ChatPageProps 及透传链中删除这三个回调


─── packages/views/src/chat/message-face.tsx:360-360 ───
[bug · medium] 无效图片占位文案本地化回归：renderChatMarkdown 的清洗在内部完成（原始 HTML 转义、链接白名单），直接调用不存在 XSS 问题，但第二参传 {} 后
invalidImageLabel 回退到 markdown.ts:161 的硬编码中文常量 INVALID_IMAGE_LINK_PLACEHOLDER（'无效的图片链接'）。en/ja/ko/ru
语言下无效图片将显示中文占位。旧路径 renderMessageHtml(content, t.invalidImageLink) 传入了本地化文案，chat-copy 五语言均有
invalidImageLink key，且 props.copy 在此作用域可用，直接补上即可。

-   const html = renderChatMarkdown(props.content, {});
+   const html = renderChatMarkdown(props.content, { invalidImageLabel: props.copy.invalidImageLink });


─── packages/views/src/chat/message-face.tsx:254-254 ───
[maintainability · low] 剪贴板实现三处并存：本文件私有 writeClipboardText（带 execCommand 降级）、message-list.tsx 导出的
writeClipboardText（同款降级、多一个 clipboard 注入参数）、此处 copyAll 直用 navigator.clipboard 无降级（非安全上下文/旧 WebView
下静默失败）。建议收敛为单一实现：把降级版提到共享工具，AnswerToolbar 与 copyAll 复用同一函数，删除重复副本。

-     void navigator.clipboard?.writeText(text).catch(() => undefined);
+     void writeClipboardText(text).catch(() => undefined); // 复用统一的降级实现（合并 message-list 导出版）


─── packages/views/src/chat/message-face.tsx:414-414 ───
[maintainability · low] 静态 inline style 违反「仅动态值用 style」的约定：BotMessageFace 内层 div 的
display/flexDirection/gap 是纯静态值（Vue 的 flex col gap 8 布局），应下沉到语义类（如 .rag-answer-stack 的父级规则或新增
wk-chat-bot-message-col）。同类问题：RequestInfoButton wrap span 的 position/display 静态 inline
style、session-sidebar 骨架行固定 width/height、menu-more 的 fontSize inline、page.tsx 的
--sandbox-panel-width: 420px（业务尺寸硬编码在组件里，应由 CSS 变量默认值承载）。



─── packages/views/src/chat/message-face.tsx:334-335 ───
[bug · medium] 推荐问题「生成中」的 UI 反馈随旧工具条一并消失：删除前的 message-list 工具条在 suggestions?.status === 'generating'
且为最后一条消息时渲染 .wk-chat-follow-up-loading 加载指示（role=status + t.followUpQuestionsLoading）；新的
AnswerToolbar（及 BotMessageFace/MessageList）没有任何消费 'generating' 状态的分支——建议区只在 status === 'ready'
时渲染。结果是生成期间用户毫无反馈，且 chat-copy 五个语言的 followUpQuestionsLoading 文案与 .wk-chat-follow-up-loading
类成为零引用死键（全仓库搜索确认）。建议：在 AnswerToolbarProps 增加可选 followUpGenerating（或 suggestions）属性并恢复该指示 span；若确属
Vue 对齐有意移除，请同步删除五个语言表中的 followUpQuestionsLoading 键。

      <div className="answer-toolbar wk-chat-answer-toolbar">
+       {props.followUpGenerating ? (
+         <span className="wk-chat-follow-up-loading" role="status">{copy.followUpQuestionsLoading}</span>
+       ) : null}
        <ToolbarButton icon="copy" title={copied ? copy.copied : copy.copy} className="wk-chat-copy" onClick={() => void copyAnswer()} />


─── packages/views/src/chat/message-list.tsx:397-402 ───
[bug · medium] steer 失败重试/移除按钮为 no-op 死控件：注释自述「占位 no-op」，但 _steerFailed 并非死分支——宿主 apps/web
ChatRoutePage 的 markSteerPreviewFailed 会 setMessages 把失败标记写入消息行，UserMessageFace 随即渲染出「重试/移除」两个按钮，点击后
onRetrySteer?.()/onRemoveSteer?.() 因传入 undefined 而无任何效果，用户在失败气泡下看到的是失效控件。ChatPageProps 已有
onSteerRetry/onSteerRemove（composer 的 chip 就在用），建议给 MessageListProps 新增同名 props 并由 page.tsx
透传，而不是硬编码 undefined。

+ // MessageListProps 新增：onRetrySteer?(steerId: string): void; onRemoveSteer?(steerId: string): void;
                <UserMessageFace
                  copy={t}
                  message={message}
-                 onRetrySteer={undefined}
-                 onRemoveSteer={undefined}
+                 onRetrySteer={onRetrySteer}
+                 onRemoveSteer={onRemoveSteer}
                />


─── packages/views/src/chat/message-list.tsx:217-219 ───
[bug · medium] 消息渲染门与引用计数口径不一致 + agentEventStream 死字段：(1)
RagPipelineProgressFace（message-face.tsx）的引用计数合并 knowledge_references 与 references 两个字段，而本门函数只检查
knowledge_references——完成消息若仅有 references（如网页引用）而无 knowledge_references、content 为空且带 agent_steps，会被判为
is-empty-segment 整行隐藏，其引用入口随行丢失，与同文件计数逻辑自相矛盾。(2) row.agentEventStream 在本 React 包内没有任何写入方（仅 Vue
侧数据模型使用，全包唯一引用点就是本行），hasStream 恒为 false，是搬运自 Vue 的死分支。建议门的字段集合与 RagPipelineProgressFace 对齐，并移除或接入真实的
agentEventStream。

-   const hasStream = Array.isArray(row.agentEventStream) && (row.agentEventStream as unknown[]).length > 0;
-   if (hasStream) return true;
-   if (Array.isArray(row.knowledge_references) && (row.knowledge_references as unknown[]).length > 0) return true;
+   const referenceRows = [
+     ...(Array.isArray(row.knowledge_references) ? row.knowledge_references as unknown[] : []),
+     ...(Array.isArray(row.references) ? row.references as unknown[] : []),
+   ];
+   if (referenceRows.length > 0) return true; // 与 RagPipelineProgressFace 的计数字段对齐


─── packages/views/src/chat/page.tsx:681-688 ───
[bug · medium] 回底按钮与 main 地标的可达性回退：回底控件由原先带 aria-label 的 <button> 改为 <div onClick>，无
role/tabIndex/onKeyDown，键盘用户无法触达（copy.chatScrollBottom 文案现成可用）。同时会话视图根节点由 <main
className="wk-chat-page"> 改为 <div className="chat">，丢失 main 地标（landmark），读屏用户失去页面主区域锚点。建议回底改回
button（或补 role="button"+tabIndex+键盘事件），.chat 外层可保留 main 语义。

-       <div
+       <button
+         type="button"
          className="scroll-to-bottom-btn wk-chat-scroll-bottom"
          style={{ display: userScrolledUp ? undefined : 'none' }}
+         aria-label={copy.chatScrollBottom}
          onClick={() => {
            const box = scrollBoxRef.current;
            if (box) box.scrollTo({ top: box.scrollHeight });
          }}
        >


─── packages/views/src/chat/page.tsx:629-629 ───
[maintainability · low] 回退 headerSlot 的 ⋯ 按钮是死控件 + headerUtilityItems 成为死 prop：ChatHeaderMenu
删除后，未注入 headerSlot 的消费方（packages/views 是共享包）看到的 ⋯ 按钮只有 aria-label 没有
onClick，点击无反应——要么给它接上最小回退菜单，要么在无 headerSlot 时不渲染该按钮。另外 headerUtilityItems 在包内已无任何消费点（apps/web 的
chat-header.tsx 自行承载工具项），该 prop 现在是死接口字段，建议从 ChatPageProps 移除或注明由 headerSlot 完全取代。



─── packages/views/src/chat/session-sidebar.tsx:304-304 ───
[maintainability · medium] fork 徽标与来源徽标移除后遗留死代码：本行所在的新 DOM 不再渲染 parent_session_id 的 ⑂ 分叉徽标，也不再渲染
sessionSourceBadge 的 is-embed/is-im/is-api 来源徽标，但相关契约没有同步清理——chat-copy.ts 五个语言的 forkBadgeTooltip
已无任何消费点（全仓库唯一引用就是定义处），MessageListProps/page.tsx 的 onForkMessage/canForkMessage 仍在解构与透传却无渲染入口，domain
的 sessionSourceBadge 在 React 侧也已无消费方。用户同时失去从列表识别分叉会话与来源的能力。请确认该能力由宿主（platform-shell
侧栏）补齐；否则恢复徽标渲染，并清理死 copy key 与死 props。



─── packages/views/src/chat/session-sidebar.tsx:307-307 ───
[maintainability · low] apiOwnerTagOf(session) 在同一行内重复调用 4 次（条件 + kind + full + label），每次都做两次
startsWith 与字符串切分。建议在 map 回调开头计算一次存变量（const ownerTag = apiOwnerTagOf(session);），顺带消除 4 处非空断言 !。

-                       {apiOwnerTagOf(session) ? <span className={'session-owner-tag session-owner-tag--' + apiOwnerTagOf(session)!.kind} title={apiOwnerTagOf(session)!.full}>{apiOwnerTagOf(session)!.label}</span> : null}
+           const ownerTag = apiOwnerTagOf(session);
+           // ...
+           {ownerTag ? <span className={'session-owner-tag session-owner-tag--' + ownerTag.kind} title={ownerTag.full}>{ownerTag.label}</span> : null}


─── packages/views/src/craft/shell.tsx:100-100 ───
[bug · medium] CraftDrawer 注释承诺 "full width under 760px via CSS"，但该行为在本次迁移后实际不成立：craft.css 及全库均无任何
`.wk-craft-drawer` CSS 规则，也没有 760px 媒体查询；更关键的是 td.tsx 的 Drawer 将 size 写为内联样式 `style={{ width: size
}}`，内联样式优先级高于普通 CSS 规则，即使后续补媒体查询、不加 `!important` 也无法覆盖。结果是 ≤460px 视口（如 390px 手机）下右侧抽屉固定 460px
宽、左缘超出视口，标题/内容左侧被裁剪。建议在 craft.css 补上 `@media (max-width: 760px) { .wk-craft-drawer
.t-drawer__content-wrapper { width: 100% !important; } }`，或让 td Drawer 通过 class/CSS
变量承载宽度以恢复可覆盖性；若暂不处理窄屏，请同步修正注释，避免误导后续维护者。

+ // craft.css 补充窄屏全宽规则（内联 style 需 !important 覆盖）：
+ // @media (max-width: 760px) {
+ //   .wk-craft-drawer .t-drawer__content-wrapper { width: 100% !important; }
+ // }
      <Drawer open={open} title={title} onClose={onClose} size="460px" placement="right" className="wk-craft-drawer">


─── packages/views/src/craft/shell.tsx:100-100 ───
[maintainability · low] CraftDrawer 未向 Drawer 传递 closeLabel，抽屉关闭按钮的 aria-label 会使用 td.tsx 的英文默认值
'Close'。craft 域其余文案均走 craftStrings(locale) 多语言体系（本组件 title 也常为中文，如测试中的 "来源"），中文界面下读屏会混读英文。建议为
CraftDrawer 增加可选的 closeLabel（或 locale）prop 并透传给 Drawer，保持读屏文案与界面语言一致。

-     <Drawer open={open} title={title} onClose={onClose} size="460px" placement="right" className="wk-craft-drawer">
+ export function CraftDrawer({ open, title, onClose, closeLabel, children }: {
+   open: boolean;
+   title: string;
+   onClose: () => void;
+   closeLabel?: string;
+   children: ReactNode;
+ }) {
+   return (
+     <Drawer open={open} title={title} onClose={onClose} size="460px" placement="right" className="wk-craft-drawer" closeLabel={closeLabel}>
+       {children}
+     </Drawer>
+   );
+ }


─── packages/views/src/craft/td.tsx:7-8 ───
[bug · medium] 头注释的前提有误：`overflow: clip` 与 `hidden` 对 fixed 后代的裁剪行为不同。按 CSS Overflow Module Level 3
的规定及现代浏览器实现（Chromium/Firefox/WebKit），`overflow: clip` 会裁剪 position: fixed 的后代（这正是 hidden 与 clip
的标志性差异之一）；"fixed 的 containing block 是视口，不受 shell overflow-x: clip 裁剪"仅对 hidden 成立。而 craft.css 将
`.wk-craft-shell`/`.wk-craft-page` 固定为 overflow-x: clip（shell.test.tsx 亦断言此契约），且并行的
shell.tsx（CraftShell × CraftDrawer）正是以该裁剪 shell 承载 td 弹层的指定组合——一旦按此契约组合，遮罩只覆盖 shell 条带、右侧 460px
抽屉在宽屏下会被裁掉绝大部分。建议：与真实 tdesign 一致把弹层挂到 document.body（需重新评估本包不带 react-dom 的约束或由调用方注入 portal
容器），至少应修正此注释前提，并在 shell 契约组合下补真实浏览器验证。



─── packages/views/src/craft/td.tsx:112-115 ───
[bug · medium] effect 依赖含 onClose，但唯一 Dialog 调用点（versions.tsx:98）传的是内联箭头 `onClose={() =>
setConfirming(null)}`，每次渲染都是新引用：弹窗打开期间父组件任何重渲染都会重跑 effect——cleanup 先把焦点归还到弹窗外的触发元素、effect
再重新聚焦面板，用户在弹窗内 Tab 到的焦点位置（如"确认恢复"按钮）被重置回面板根部，且 document 级键盘监听反复卸载重挂。建议用 latest-ref 模式（`const
onCloseRef = useRef(onClose); onCloseRef.current = onClose;`）只依赖 open。另外：焦点落在 body
时（点击弹窗内非聚焦文本后）`!panelRef.current.contains(active)` 会提前 return，Escape 随之失效——建议把 body 也视为域内。

-       restoreRef.current?.focus();
-       restoreRef.current = null;
+   const onCloseRef = useRef(onClose);
+   onCloseRef.current = onClose;
+   useEffect(() => {
+     if (!open) return undefined;
+     // ...
+     const onKeyDown = (event: KeyboardEvent) => {
+       if (event.key !== 'Escape') return;
+       const active = typeof document !== 'undefined' ? document.activeElement : null;
+       const inScope = active === null || active === document.body
+         || (panelRef.current !== null && panelRef.current.contains(active));
+       if (!inScope) return;
+       event.preventDefault();
+       event.stopPropagation();
+       onCloseRef.current();
      };
-   }, [onClose, open]);
+     // ...
+   }, [open]);


─── packages/views/src/craft/td.tsx:166-166 ───
[bug · medium] 关闭控件给了 role="button" 却未设 tabIndex 也无 Enter/Space 激活——读屏用户会听到"按钮"却永远 Tab
不到、无法操作（WAI-ARIA 要求 role=button 元素可聚焦），键盘用户只能依赖 Escape/遮罩；Drawer 的关闭 div 与 Dialog 的关闭 span 同病。同时
aria-modal="true" 声明了模态语义却未做焦点圈禁，Tab 可逃逸至被遮罩覆盖的背景内容，与头注宣称的"键盘/读屏契约等价"不符（且参照物 packages/ui 及其
interaction.test.tsx 已在本变更集中删除，等价性无从追溯）。建议改用真实 button 元素，并补最小 Tab 圈禁或降级为非 modal 语义。

-         <div className="t-drawer__close-btn" role="button" aria-label={closeLabel} onClick={onClose}><CloseIcon /></div>
+         <button type="button" className="t-drawer__close-btn" aria-label={closeLabel} onClick={onClose}><CloseIcon /></button>


─── packages/views/src/craft/td.tsx:119-122 ───
[maintainability · low] useBodyPortal 名不副实：既不 createPortal 也不挂 body，只是"浏览器环境且
open"的渲染开关，容易让维护者误以为弹层已脱离祖先的裁剪/层叠上下文（真实 tdesign 恰恰是 body portal，本文件的两个差异声明也依赖读者不误解这一点）。另外作为客户端判定，若
SSR 首帧 open=true，服务端输出 null、客户端渲染弹层，会产生 hydration 不一致。建议改名为 shouldRenderOverlay 一类并注明"树内渲染、受祖先
clip/stacking 影响"的语义。

- function useBodyPortal(open: boolean): boolean {
-   // 树内渲染：仅在浏览器（非 SSR 静态标记）且 open 时挂载。
+ function shouldRenderOverlay(open: boolean): boolean {
+   // 树内渲染（非 body portal，受祖先裁剪/层叠影响）：仅在浏览器且 open 时挂载。
    return open && typeof document !== 'undefined';
  }


─── packages/views/src/craft/td.tsx:35-37 ───
[style · low] CloseIcon 的 style={{ fill: 'none' }} 与已设置的 fill="none" 属性冗余；Dialog 关闭钮的 style={{
marginLeft: 'auto' }} 也是纯静态样式。非动态样式应避免内联，建议删除冗余项并把 marginLeft 移入 t-dialog__close 相关 CSS（tdesign.css
或 craft.css 覆盖）。

        width="1em"
        height="1em"
-       style={{ fill: 'none' }}


─── packages/views/src/craft/td.tsx:105-107 ───
[bug · medium] 嵌套弹层时一次 Escape 会同时关闭 Dialog 与背后的 Drawer：两个弹层（如 versions.tsx 的确认 Dialog 渲染在 Drawer
body 内——树内渲染无 portal）都在 document 上挂 keydown 监听，stopPropagation 无法阻止同一节点上的其他监听（那需要
stopImmediatePropagation，且只能拦住后注册者）；Drawer 先打开先注册，其 contains(active) 检查又因 Dialog DOM 嵌在 Drawer
内而通过，于是两层 onClose 都被触发。建议引入模块级弹层栈（仅最顶层弹层响应 Escape），或至少改用 stopImmediatePropagation + 后注册者优先的协调机制，并在
shell.test.tsx 补嵌套用例。

-       event.preventDefault();
-       event.stopPropagation();
-       onClose();
+ // 模块级弹层栈：仅最顶层弹层响应 Escape
+ const overlayStack: symbol[] = [];
+ // useEffect(open) 内：
+ //   const layer = Symbol(); overlayStack.push(layer);
+ //   const onKeyDown = (event: KeyboardEvent) => {
+ //     if (event.key !== 'Escape') return;
+ //     if (overlayStack[overlayStack.length - 1] !== layer) return; // 只拦最顶层
+ //     ...
+ //   };


─── packages/views/src/craft/td.tsx:159-159 ───
[bug · low] 模态打开期间未锁定 body 滚动：真实 tdesign Drawer/Dialog
挂载时会锁定页面滚动（useLockScroll），本手写层未复刻，且头部"差异注记"也未列明——Drawer/Dialog
打开后在遮罩上滚动滚轮/触摸滑动会滚动背后的工作区内容，与"同构"口径不符。建议在 useOverlayFocus/useBodyPortal 的 effect 中为 document.body 加
overflow:hidden（计数式，防多弹层互相解锁），或至少将该差异补进文件头部的差异注记。



─── apps/web/src/administration/AdministrationPage.tsx:164-164 ───
[bug · high] PAGER_BTN_DISABLED 平移时丢失了原 Tailwind 常量的前导空格（原为 ' disabled:...'）：line 199/203 处
`PAGER_BTN + PAGER_BTN_DISABLED` 直接拼接会生成
'wk-admin-pager-btnwk-admin-pager-btn-disabled'，两个类名粘连后均无法命中 administration-u.css
中的任何规则，导致分页器"上一页/下一页"按钮的 inline-flex/尺寸/光标基础样式与 disabled 态、hover 态样式全部失效（对照 line 202 的 `PAGER_BTN +
' wk-admin-pager-current'` 带空格所以正常）。请补回前导空格或改用模板字符串拼接。

- const PAGER_BTN_DISABLED = 'wk-admin-pager-btn-disabled';
+ const PAGER_BTN_DISABLED = ' wk-admin-pager-btn-disabled';


─── apps/web/src/analytics/AnalyticsPage.tsx:355-356 ───
[bug · medium] Tabs 迁移存在未声明的面板挂载语义变化：原 @weknora/ui（Radix）TabsContent 非激活时默认卸载，而 tdesign TabPanel 默认
destroyOnHide=false，非激活面板内容常驻 DOM（仅 display:none 隐藏）。迁移后 charts（4 个 recharts ResponsiveContainer
图表）与 usage（data-testid="usage-by-user-table" 表格）两面板同时挂载：(1) 切回 charts 后 usage 表格仍留在 DOM，改变"仅激活面板在
DOM"的测试/查询语义；(2) 隐藏的 recharts 容器持续挂载，恢复显示完全依赖 ResizeObserver 正确重算尺寸。仓库其他 tdesign Tabs
用法（PersonalMemorySettingsPanel/SystemGlobalSettingsPanel）的 TabPanel 仅作 tab
头声明、实际内容在外部渲染，不受影响——本页是唯一把重内容放进面板的用法。建议为两个 TabPanel 显式加 destroyOnHide 恢复 Radix
卸载语义，或在注释中记录保留挂载是有意决策。

        <Tabs value={tab} onChange={(value) => setTab(value === 'usage' ? 'usage' : 'charts')}>
-         <Tabs.TabPanel value="charts" label={t(locale, 'analytics.tabCharts')}>
+         <Tabs.TabPanel value="charts" destroyOnHide label={t(locale, 'analytics.tabCharts')}>
+         ...
+         <Tabs.TabPanel value="usage" destroyOnHide label={t(locale, 'analytics.usageTab')}>


─── apps/web/src/analytics/analytics-u.css:311-313 ───
[maintainability · low] 文件内部质量问题：(1) `.wk-anl-an-usage-num` 在文件中部（text-align:
right）与文件尾（font-variant-numeric: tabular-nums）两处定义，应合并为一条规则；(2) `.wk-anl-an-usage-cell` 与
`.wk-anl-2` 中 `border-bottom-width: 1px;border-color: ...` 分号后缺空格，与全文件其余格式不一致；(3) `:focus`/`:hover`
规则里重复声明基类已有的 `border-style: solid`（date-input、btn-primary/outline/pager、text-input 均有），纯冗余；(4)
文件头声称"导入顺序：须在本域 .td.css 之前"，但 analytics 域当前不存在 .td.css（仅 analytics-u.css），该说明与实际不符，易误导后续维护。

- /* T15 语义化：原 placeholder:text-[rgba(23,26,29,0.35)] / tabular-nums 旧栈 utility。 */
- .wk-anl-an-text-input::placeholder { color: rgba(23, 26, 29, 0.35); }
- .wk-anl-an-usage-num { font-variant-numeric: tabular-nums; }
+ /* 合并至文件中部原有的 .wk-anl-an-usage-num 定义： */
+ .wk-anl-an-usage-num {
+   text-align: right;
+   font-variant-numeric: tabular-nums;
+ }
+ /* usage-cell / wk-anl-2 的边框声明补空格： */
+   border-bottom-width: 1px; border-color: #eef1f5;


─── apps/web/src/analytics/analytics-u.css:104-110 ───
[maintainability · low] 原 Tailwind
语义令牌被完全字面化：bg-surface→#ffffff、bg-accent/text-accent→#07c05f、bg-accent-wash→rgba(7,192,95,0.08)。经核验字面
值与旧栈 token 生效值一致（wk-legacy.css 记录 --color-surface #ffffff / --color-accent
#07c05f），渲染不变；但同批迁移文件（administration-u.css、kb-list.td.css 等）对同类值采用 `var(--color-accent, #07c05f)`
带兜底的写法，保留了未来重建 token 体系/主题化（如暗色模式）的间接层，本文件直接写死后此能力丢失，且与既有 -u.css 风格不一致。建议统一为 var(--color-*, 字面量兜底)
形式。

-   border-color: rgba(7,192,95,0.5);
-   background-color: #ffffff;
+   border-color: var(--color-accent, rgba(7,192,95,0.5));
+   background-color: var(--color-surface, #ffffff);
    padding-inline: 15px;
    font-family: inherit;
    font-size: 14px;
    font-weight: 500;
-   color: #07c05f;
+   color: var(--color-accent, #07c05f);


─── apps/web/src/administration/AdministrationPage.tsx:155-155 ───
[bug · medium] 邀请邮箱输入由 `<Input type="email" required ...>` 迁移为 TInput 时丢失了 type="email" 与
required，浏览器原生邮箱格式校验随之消失；而 onInviteFormSubmit/sendInvitation 中仅有 `email.trim()` 非空检查、无格式兜底，任意非空字符串（如
"abc"）都能进入确认步骤并提交后端，只能依赖后端报错且提示不友好。建议加回 type="email"（TDesign Input 透传原生 type，同处 memberSearch 的
TInput 也保留了 type="search"），并在 onInviteFormSubmit 中补充邮箱格式校验兜底。

- return <main className="wk-page wk-admin-1"><header className="wk-header wk-admin-2"><div><p className="wk-eyebrow wk-admin-3">{t('mobileAdministration.tenant', { tenant: tenantId })}</p><h1 className="wk-admin-4">{systemAdmin ? t('settings.navGroups.systemAdministration') : t('mobileAdministration.title')}</h1><p className="wk-muted wk-admin-5">{t('mobileAdministration.readOnly')}</p></div><TButton type="button" onClick={() => void load()} disabled={loading}>{t('mobileAdministration.refresh')}</TButton></header>{error ? <Status tone="error">{error}</Status> : null}<div className="wk-admin-12"><Card><h2 className="wk-admin-6">{t('mobileAdministration.members', { count: membersTotal })}</h2><TInput type="search" value={memberSearch} placeholder={t('mobileAdministration.searchPlaceholder')} aria-label={t('mobileAdministration.searchPlaceholder')} onChange={(value) => setMemberSearch(String(value))} className="wk-admin-7" />{loading ? <Status>{t('mobileAdministration.loading')}</Status> : members.length === 0 ? <Status>{appliedMemberSearch ? t('mobileAdministration.noMembersForQuery', { q: appliedMemberSearch }) : t('mobileAdministration.noMembers')}</Status> : <ul className="wk-list wk-admin-8">{members.map((member) => <li key={member.user_id} className="wk-admin-9"><div className="wk-list-item-copy wk-admin-10"><strong>{member.username}</strong><span className="wk-admin-11">{member.email} · {roleText(member.role)} · {member.status}</span></div><div className="wk-list-actions wk-admin-13"><TSelect value={member.role === 'owner' ? 'owner' : member.role} disabled={member.role === 'owner'} onChange={(value) => void updateRole(member, String(value) as TenantMember['role'])} className="wk-admin-14" options={[{ value: 'owner', label: roleText('owner') }, { value: 'admin', label: roleText('admin') }, { value: 'contributor', label: roleText('contributor') }, { value: 'viewer', label: roleText('viewer') }]} /><TButton type="button" disabled={member.role === 'owner'} onClick={() => void removeMember(member)}>{t('mobileAdministration.remove')}</TButton></div></li>)}</ul>}{membersTotal > 0 ? <MembersPager total={membersTotal} page={membersPage} pageSize={membersPageSize} onPage={onMembersPage} onPageSize={onMembersPageSize} t={t} /> : null}</Card><Card><h2 className="wk-admin-6">{inviteStep === 'confirm' ? t('mobileAdministration.confirmInviteTitle') : t('mobileAdministration.invite')}</h2>{inviteStep === 'confirm' ? <div className="wk-admin-15"><p className="wk-muted wk-admin-16">{t('mobileAdministration.confirmInviteBody', { email: email.trim(), role: roleText(role) })}</p><div className="wk-admin-17"><TButton type="button" disabled={saving} onClick={() => setInviteStep('form')}>{t('mobileAdministration.back')}</TButton><TButton type="button" loading={saving} onClick={() => void sendInvitation()}>{t('mobileAdministration.confirmSend')}</TButton></div></div> : <form className="wk-admin-15" onSubmit={onInviteFormSubmit}><label className="wk-admin-18">{t('mobileAdministration.inviteEmail')}<TInput className="wk-admin-invite-email" value={email} onChange={(value) => setEmail(String(value))} /></label><label className="wk-admin-18">{t('mobileAdministration.role', { role: '' }).replace(/: $/, '')}<TSelect value={role} onChange={(value) => setRole(String(value) as typeof role)} options={[{ value: 'admin', label: roleText('admin') }, { value: 'contributor', label: roleText('contributor') }, { value: 'viewer', label: roleText('viewer') }]} /></label><TButton type="submit" loading={saving}>{t('mobileAdministration.sendInvitation')}</TButton></form>}</Card><Card><h2 className="wk-admin-6">{t('mobileAdministration.openInvitations')}</h2>{invitations.filter((item) => invitationIsOpen(item.status)).length === 0 ? <Status>{t('mobileAdministration.noPendingInvitations')}</Status> : <ul className="wk-list wk-admin-8">{invitations.filter((item) => invitationIsOpen(item.status)).map((item) => <li key={item.id} className="wk-admin-9"><div className="wk-list-item-copy wk-admin-10"><strong>{item.invitee_email ?? item.invitee_user_id}</strong><span className="wk-admin-11">{roleText(item.role)} · {t('mobileAdministration.expires', { date: item.expires_at })}</span></div><TButton type="button" onClick={() => void revokeInvitation(item)}>{t('mobileAdministration.revoke')}</TButton></li>)}</ul>}</Card><Card><h2 className="wk-admin-6">{t('mobileAdministration.auditLog')}</h2>{audit.length === 0 ? <Status>{t('mobileAdministration.noAuditEntries')}</Status> : <ul className="wk-list wk-admin-8">{audit.map((item) => <li key={item.id} className="wk-admin-9"><div className="wk-list-item-copy wk-admin-10"><strong>{item.action}</strong><span className="wk-admin-11">{item.outcome} · {item.actor_role}</span><small>{item.created_at} · {item.request_method} {item.request_path}</small></div></li>)}</ul>}</Card>{systemAdmin ? <><Card><h2 className="wk-admin-6">{t('settings.navGroups.systemAdministration')}</h2><ul className="wk-list wk-admin-8">{admins.map((item, index) => <li key={String(item.id ?? index)} className="wk-admin-9"><strong>{String(item.username ?? item.email ?? item.id)}</strong><span className="wk-admin-11">{item.is_active === false ? t('common.disabled') : t('mobileAdministration.status.active')}</span></li>)}</ul></Card><div data-testid="system-administration-panels" className="wk-admin-19">{systemAdminPanelKeys(systemAdmin).map((panel) => panel === 'system-global' ? <SystemGlobalSettingsPanel key={panel} client={client} initialSettings={settings} /> : panel === 'runtime-queues' ? <RuntimeQueuesPanel key={panel} client={client} payload={queues} loading={loading} /> : panel === 'platform-api-keys' ? <PlatformApiKeysPanel key={panel} client={client} initialKeys={apiKeys} /> : <SystemAuditLogPanel key={panel} client={client} payload={systemAudit} />)}</div></> : null}</div></main>;
+ <TInput className="wk-admin-invite-email" type="email" value={email} onChange={(value) => setEmail(String(value))} />


─── apps/web/src/administration/administration-u.css:32-34 ───
[maintainability · low] accent 色在同文件内写法不一致：手工段的 .wk-admin-pager-current 用 var(--color-accent,
#07c05f)，此处 hover 却直接硬编码 #07c05f。--color-accent 已在 styles.css:457 全局定义（值同为
#07c05f），此处应统一改为变量引用，否则未来主题调整 accent 时 pager 的 hover 色会与当前页高亮色脱钩。（muted/stroke 的字面量平移是本次各域 u.css
的统一模式，已有注释标注来源变量，不强求改。）

  .wk-admin-pager-btn-disabled:enabled:hover {
-   color: #07c05f;
+   color: var(--color-accent, #07c05f);
  }


─── apps/web/src/administration/administration-u.css:18-18 ───
[style · low] .wk-admin-pager-btn 中 font-weight 先声明 400（原 Tailwind font-normal），随后又声明 inherit（原
[font:inherit] 任意值变体的层叠结果），400 为被覆盖的死声明，且会误导后续维护者以为生效值是 400，建议删除。



─── apps/web/src/administration/AdministrationPage.tsx:206-206 ───
[maintainability · low] wk-admin-pager-jump、wk-admin-invite-email、wk-admin-page-size 三个类在
administration-u.css（及全项目样式文件）中均无定义，仅被 AdministrationPage.test.tsx 用作 DOM 查询锚点；且本域并不存在
administration-u.css 头注释所要求的本域 .td.css。这些类名实际承担 data-testid
职责却无任何注释说明，后续很容易被当作"悬空无用类"清理而连带破坏测试。建议在使用处加注释标明"测试选择器锚点，勿删"，或直接改用 data-testid。



─── apps/web/src/administration/AdministrationPage.tsx:206-206 ───
[bug · low] 跳页输入由原生 Input 迁移为 TInput 时丢失了 inputMode="numeric"，移动端跳页不再弹出数字键盘，属迁移遗漏的体验回退（TDesign Input
可透传原生属性，同文件 TInput 已依赖 aria-label 透传），建议加回。

-       <TInput className="wk-admin-pager-jump wk-admin-25" type="text" value={jump}
+       <TInput className="wk-admin-pager-jump wk-admin-25" type="text" inputMode="numeric" value={jump}


─── apps/web/src/administration/administration-u.css:52-59 ───
[maintainability · medium] wk-admin-3（原 Tailwind text-primary）直接硬编码 color: #2e6de6，且未像本文件
muted/stroke 平移那样标注来源变量。--color-primary 已在 styles.css:451 全局定义（值同为 #2e6de6），且本次其他域 u.css 对 primary
色一律使用变量引用（data-sources-u.css:740、views-integrations-u.css:2323/2328、platform-u.css:662），同文件手工段
accent 也用了 var(--color-accent, …)。此处应改为 var(--color-primary, #2e6de6)，否则未来主题调整 primary 时页眉 eyebrow
色会与全局及其他域脱钩。

  .wk-admin-3 {
    margin: 0;
    font-size: 0.78rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
-   color: #2e6de6;
+   /* 旧栈 primary → styles.css --color-primary；同 data-sources/views-integrations/platform 各域 u.css 的变量引用模式 */
+   color: var(--color-primary, #2e6de6);
  }


─── apps/web/src/apps/AppsPages.tsx:80-80 ───
[maintainability] 死代码：ConnectionsPage 撤销操作已迁移至 TPopconfirm，自研 Popconfirm 组件在全文件（及整个 apps/
目录）已无任何引用点（该组件未导出，无外部使用可能），建议连同仅为其服务的 apps-u.css 中 .wk-apps-1~.wk-apps-4 规则一并删除。另注意其气泡 z-index:60 低于
TDesign 弹层层级，若后续被误复用存在被遮挡风险。



─── apps/web/src/apps/AppsPages.tsx:161-164 ───
[performance] 性能：columns={catalogColumns(t)} 与下方 columns={installationColumns(t)} 在 JSX
内联调用、ConnectionsPage 的 columns 数组在组件体内联定义，每次渲染（loading/error/数据到达/revokingId 变化）都会重建列数组与 cell 闭包，导致
TTable 全列比对与单元格重渲染。建议用 useMemo 收口：catalog/installation 列仅依赖 t；connections 列依赖 [t, canManage,
revokingId]。



─── apps/web/src/apps/AppsPages.tsx:59-63 ───
[style] 风格：此处（以及 Toast 的 tone、installationColumns 中 state
标签）为链式嵌套三元表达式，违反检查单"禁止嵌套三元"，且映射关系可读性差。建议改为映射对象直接查表。

-   const palette = theme === 'success' ? 'wk-apps-tag--success'
-     : theme === 'warning' ? 'wk-apps-tag--warning'
-     : theme === 'danger' ? 'wk-apps-tag--danger'
-     : theme === 'primary' ? 'wk-apps-tag--primary'
-     : 'wk-apps-tag--default';
+ const TAG_PALETTE: Record<'success' | 'warning' | 'danger' | 'primary' | 'default', string> = {
+   success: 'wk-apps-tag--success',
+   warning: 'wk-apps-tag--warning',
+   danger: 'wk-apps-tag--danger',
+   primary: 'wk-apps-tag--primary',
+   default: 'wk-apps-tag--default',
+ };
+ // 组件内：const palette = TAG_PALETTE[theme];


─── apps/web/src/apps/apps-u.css:5-8 ───
[maintainability] 死规则：.wk-apps-1~.wk-apps-4 四条规则仅服务于 AppsPages.tsx 中已无引用的自研 Popconfirm
组件（该组件建议删除），应随之一并移除，避免残留误导后续维护者以为仍有使用点。



─── apps/web/src/apps/AppsPages.tsx,apps/web/src/apps/apps-u.css,apps/web/src/apps/apps.td.css:0-0 ───
[bug] 兼容性：原 Tailwind -translate-x-1/2 编译为 transform，此处平移为独立 translate 属性，Safari<14.1
等旧内核不支持该属性，toast 将失去水平居中（左边缘贴 50% 位置）。建议改回 transform 写法以保持兼容面不变。

-   translate: -50% 0;
+   transform: translateX(-50%);


─── apps/web/src/agents/AgentsPage.tsx:443-444 ───
[bug · high] 卡片三点菜单的点击会冒泡触发卡片自身的 onClick（onOpen→openCard）。旧实现菜单容器上有 `onClick={(event) =>
event.stopPropagation()}`，迁移后丢失；React portal 内容的合成事件仍沿 React 树冒泡到 `.agent-card` 的
onClick，于是每个菜单动作（删除/复制/启停/编辑）都会同时执行 openCard：自有 agent 会叠加打开编辑器弹窗（如「删除」= 删除确认框 +
编辑器同时弹出）。需在菜单容器或菜单项上补 stopPropagation。

              content={(
-               <div className="popup-menu">
+               <div className="popup-menu" onClick={(event) => event.stopPropagation()}>


─── apps/web/src/agents/AgentsPage.tsx:255-259 ───
[maintainability · low] ResourceOriginBadge 引用的 `resourceOrigin.mine/tenant/space/shared` 键在
packages/i18n 中不存在（已全局检索确认），缺失时 formatMessage 会回显原始键名；且 locale 硬编码
'zh-CN'，忽略用户语言偏好（KnowledgeBasesPage 版本走 locale prop + 本地字面量兜底）。当前唯一调用点 variant="creator" 且
cornerBadge 保证 creatorName 非空，缺键分支暂不可达，但组件是导出的公共组件，属潜伏陷阱：建议补齐 i18n 键并接受 locale prop，或仿照
KnowledgeBasesPage 用本地字面量表。另外此处 4 层嵌套三元违反项目「禁止嵌套三元」规则，建议改为 map/函数表。



─── apps/web/src/agents/AgentEditorModal.tsx,apps/web/src/agents/AgentParserRules.tsx,apps/web/src/agents/AgentsPage.tsx,apps/web/src/agents/MbtiTestModal.tsx,apps/web/src/agents/PersonaSection.tsx,apps/web/src/agents/SubagentsSection.tsx,apps/web/src/agents/agents-u.css,apps/web/src/agents/agents.css,apps/web/src/agents/agents.td.css,apps/web/src/agents/list.ts:0-0 ───
[bug · low] AgentRail 导航项由旧版 `<button type="button">` 改为纯 `<div onClick>`（两处：baseItems / orgItems），无
role、tabIndex、键盘事件——键盘用户（Tab/Enter）完全无法切换空间，属可访问性回退。建议加 role="button" tabIndex={0} 及 Enter/Space
处理（同文件 AgentSectionHeader 已示范该模式）。

                <div
+                 role="button"
+                 tabIndex={0}
                  className={`icon-item-labeled${item.key === 'mine' ? ' workspace-item' : ''}${item.active ? ' active' : ''}`}
                  data-space-key={item.key}
                  onClick={() => onSelect(item.key)}
+                 onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); onSelect(item.key); } }}
                >


─── apps/web/src/agents/AgentsPage.tsx:1177-1177 ───
[bug · low] 初次加载失败时（loading={!data && !loadError} 变 false、hasCards=false），页面落入
EmptyState「创建智能体」空态，错误信息无任何呈现：视图解构丢弃了 error prop，notice 恒为 null（Status 已随迁移移除），loadError
静默。网络/接口失败被误导为「没有 agent」。建议失败时渲染错误态（或 MessagePlugin.error + 重试入口），而非空态。



─── apps/web/src/agents/AgentEditorModal.tsx:599-600 ───
[bug · low] kbWarnTimer 已成死代码：setTimeout 回调为空注释（MessagePlugin 自带超时），句柄无任何消费方，上方 clearTimeout 与 ref
声明只是无意义清理。建议连同 kbWarnTimer ref、clearTimeout 分支一并删除。

        void MessagePlugin.warning(t('agentEditor.agentType.kbIncompatibleWarn', { count }));
-       kbWarnTimer.current = window.setTimeout(() => { /* 4s 后自然消失（MessagePlugin 自身超时） */ }, 4000);


─── apps/web/src/agents/AgentEditorModal.tsx:1207-1207 ───
[security · low] `href="javascript:"` 伪协议链接（storage/sandbox/parser 三处）在启用 CSP（unsafe-inline 之外的
script-src）的部署下会被拒，且属已知坏味道。既然 onClick 已 preventDefault，建议改用 `<button type="button">` 或去掉 href
仅保留可点击元素语义。

-                 <a href="javascript:void(0)" className="go-settings-link" data-go-storage-settings onClick={(event) => { event.preventDefault(); navigate('/platform/settings?section=storage'); }}>
+                 <button type="button" className="go-settings-link" data-go-storage-settings onClick={() => navigate('/platform/settings?section=storage')}>


─── apps/web/src/agents/AgentsPage.tsx:1004-1009 ───
[bug · low] 乐观收藏的失败回滚存在竞态：回滚基于 favoritesRef.current（可能已含后续更快的再次切换）。序列「取消收藏→立即再收藏→首个请求失败」会把 UI
回滚成未收藏，而 DB 最终为已收藏，UI 与 DB 不一致直至刷新。建议回滚时仅当 favoritesRef.current 仍等于本次 next 才执行，或携带请求序号比对。

+     const pending = next;
      void persist.catch(() => {
+       if (favoritesRef.current !== pending) return; // 已有更新的切换，放弃过期回滚
        const rollback = new Set(toggleFavoriteId([...favoritesRef.current], id));
        favoritesRef.current = rollback;
        writeFavoriteIds(window.localStorage, viewer.userId, tenantKey, [...rollback]);
        setFavorites(rollback);
      });


─── apps/web/src/auth/LoginPage.tsx:262-269 ───
[bug · high] toggleMode 漏清 email：tdesign Form 为非受控（FormItem 只用 initialData="" 初始化，Input 未传
value），切卡后新表单组件卸载/重挂、输入框显示为空，但 React state 中的 email 仍保留上一张卡的旧值。而
submit（validateLogin/validateRegister、client.auth.login/register）读取的全部是 state 中的 email。复现路径：登录卡输入
email A → 点"创建账户" → 注册卡 email 框显示为空 → 用户只填用户名/密码/确认密码提交 → 校验通过并以屏幕上不存在的 email A 完成注册（反向切卡同理，可用旧
email 登录）。Vue 端因 v-model 受控，切卡后旧 email 会回显在新表单中，用户可见可改；这是本次非受控迁移引入的行为差异。同根问题也存在于 submit 内 register
成功后的 setMode('login') 预填路径（注释称 "prefill the email"，但非受控表单同样不回显）。建议在 toggleMode 中与其它字段一并清空
email（或改为受控 value 绑定以复刻 Vue 回显语义）。

    function toggleMode() {
      // Vue toggleMode (Login.vue:509-515) — flips the card and clears register fields.
      setMode((current) => (current === 'register' ? 'login' : 'register'));
      setState('idle');
      setMessage('');
      setFieldErrors({});
-     setUsername(''); setPassword(''); setConfirmPassword('');
+     // Vue 端 v-model 受控，切卡后旧 email 会回显；React 端 tdesign Form 非受控
+     // （initialData=""），切卡后输入框显示为空，必须同步清空 state，否则
+     // submit 会提交屏幕上不存在的旧 email。
+     setUsername(''); setEmail(''); setPassword(''); setConfirmPassword('');
    }


─── apps/web/src/auth/WorkspaceOnboardingPage.tsx:171-176 ───
[bug · low] invitations 弹窗丢失了 closeLabel 透传：迁移前 Dialog 接收 closeLabel={msg(locale,
'auth.workspaceOnboarding.close')}，而 WkDialog 的 closeLabel 默认值为英文
'Close'（wk-legacy.tsx:35，用作右上角关闭按钮的 aria-label）。未传参时中文/俄文/韩文/日文环境下屏幕阅读器会读到英文 "Close"，构成 i18n/a11y
回归。同文件的 create 弹窗本就没有传，但 invitations 弹窗是迁移中有既有本地化传参被删除，应补回。

      <TDialog
        open={invitationsVisible}
        title={msg(locale, 'auth.workspaceOnboarding.invitations')}
        onClose={() => setInvitationsVisible(false)}
+       closeLabel={msg(locale, 'auth.workspaceOnboarding.close')}
        className="wk-onb-11"
      >


─── apps/web/src/auth/login.td.css:39-50 ───
[maintainability · low] .swiper-slide 的 transition-property 连续声明两次，第一条（transform）恒被第二条（transform,
opacity）覆盖，属 swiper 基础 css 与 fade effect 附加规则合并时留下的死声明，应删除第一条以免误导后续维护。

  .login-layout .swiper-slide {
    flex-shrink: 0;
    width: 100%;
    height: 100%;
    position: relative;
-   transition-property: transform;
    display: block;
    /* effect-fade crossFade（speed 800）：稳态下 opacity 由 inline style 决定 */
    transition-property: transform, opacity;
    transition-duration: 800ms;
    transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
  }


─── apps/web/src/auth/login.td.css:193-201 ───
[maintainability · low] 此 prefers-reduced-motion 块为死规则：文件末尾的第二个 prefers-reduced-motion 块已将整个
.animated-bg display:none（且对 node/line 追加 animation/transition !important），本块的 animation:none 与
opacity:0.65 永远不会生效。若两个块均为事实源原样平移，建议在保留时补注释说明后者覆盖前者（与文件内 t-form-item 笔误保真注释同例），否则应删除本块避免误导。



─── apps/web/src/auth/login.td.css:1031-1033 ───
[bug · low] dark 主题遗漏 .oidc-divider span 的背景覆盖：§1 中 .oidc-divider span
用白色背景（rgba(255,255,255,0.95)）遮挡分隔线，§5 dark 块对同构的 .register-cta__divider span
补了深色覆盖（rgba(36,36,36,0.97)），却没有 .oidc-divider span 的对应规则——OIDC 启用且 dark 模式下，深色 form-card
内的分隔文案会呈白色色块。若 Vue 事实源同样缺失可按保真原则保留，否则应补齐。

- html[theme-mode="dark"] .login-layout .register-cta__divider span {
+ html[theme-mode="dark"] .login-layout .register-cta__divider span,
+ html[theme-mode="dark"] .login-layout .oidc-divider span {
    background: rgba(36, 36, 36, 0.97);
  }


─── apps/web/src/auth/LoginPage.tsx:413-413 ───
[other · low] 轮播分页 bullet 由迁移前的 <button> 改为 <span onClick>：span 不可聚焦、无键盘激活路径，键盘用户失去了切换轮播的能力（迁移前
button 天然支持 Tab + Enter/Space）；aria-label 挂在无 role 的 span 上也不会被辅助技术播报为可操作控件。虽然这是对齐 Vue swiper 原生 DOM
结构的有意决策，但相对旧 React 实现构成可达性回退，建议至少补 role="button" + tabIndex={0} +
onKeyDown(Enter)（swiper-pagination-bullet 样式不受影响）。

-                 <span key={slide.titleKey} className={`swiper-pagination-bullet${index === slideIndex ? ' swiper-pagination-bullet-active' : ''}`} aria-label={t(slide.titleKey)} title={t(slide.titleKey)} onClick={() => setSlideIndex(index)} />
+                 <span key={slide.titleKey} role="button" tabIndex={0} className={`swiper-pagination-bullet${index === slideIndex ? ' swiper-pagination-bullet-active' : ''}`} aria-label={t(slide.titleKey)} title={t(slide.titleKey)} onClick={() => setSlideIndex(index)} onKeyDown={(event) => { if (event.key === 'Enter') setSlideIndex(index); }} />


─── apps/web/src/apps/AppsPages.tsx:96-96 ───
[bug · medium] PageFrame 的刷新 TButton 未指定 variant，tdesign-react 默认 variant="base"（主题色实心按钮）。而
.wk-apps-8 是按原 outline 形态平移的覆盖样式（border-color #dcdcdc、color rgba(0,0,0,0.9)、height
32px），叠加在实心按钮上会出现「绿色实心背景 + 深色文字 + 灰边框色」的混合形态，且实际生效值取决于与 TDesign 组件样式的加载顺序，结果脆弱。同文件
CatalogPage/ConnectionsPage 的刷新按钮均已显式 variant="outline"（头部注释亦声明迁移目标为 outline
button），authorization/action 两页不一致。建议补上 variant="outline"，让 .wk-apps-8 回归为对 outline 形态的尺寸/配色微调。

- return <main className={`wk-page wk-apps-26 ${gapClass}`}><header className="wk-apps-5"><div><h1 className="wk-apps-6">{title}</h1><p className="wk-apps-7">{description}</p></div><TButton aria-label={refreshLabel} disabled={loading} onClick={onReload} loading={loading} className="wk-apps-8"><IconRefresh size={14} />{refreshLabel}</TButton></header>{loading ? <div role="status"><Status>{loadingLabel}</Status></div> : null}{children}</main>;
+ <TButton variant="outline" aria-label={refreshLabel} disabled={loading} onClick={onReload} loading={loading} className="wk-apps-8"><IconRefresh size={14} />{refreshLabel}</TButton>


─── apps/web/src/auth/LoginPage.tsx:434-436 ───
[bug · medium] 注册成功切回登录卡后 email 预填失效：submit 的注册成功分支仅清空 username/password/confirmPassword，有意保留
state.email 作为预填（注释引 Login.vue:744-746 "switch to login and prefill the email"）。但登录卡 tdesign Form
是非受控的（FormItem 仅以 initialData="" 初始化，Input 未绑 value），setMode('login') 使登录卡整体重挂后，email 输入框显示为空，而
state.email 仍持有注册时的值——形成 UI 空/state 有值的幽灵 email：用户只填密码提交会以看不见的注册 email 登录（碰巧成功但违背预期），Vue
端此处应显示预填。与已确认的 toggleMode 漏清 email 是同一非受控脱节的不同表现。建议通过 Form 实例在切卡后回填：

-             <Form labelAlign="top" layout="vertical" onValuesChange={onLoginValuesChange} onSubmit={({ e }) => { void submit(e); }}>
+             <Form ref={loginFormRef} labelAlign="top" layout="vertical" onValuesChange={onLoginValuesChange} onSubmit={({ e }) => { void submit(e); }}>
                <Form.FormItem label={t('auth.email')} name="email" requiredMark initialData="">
                  <Input placeholder={t('auth.emailPlaceholder')} type="text" autocomplete="email" size="large" disabled={loading} {...ariaFor('email')} />
+                 ...
+ // 注册成功分支 setMode('login') 后同步回填：
+ //   loginFormRef.current?.setFieldsValue({ email });


─── apps/web/src/auth/LoginPage.tsx:276-279 ───
[maintainability · low] fieldError 内错误 span 的 id 内联重复实现了紧随其后的 fieldErrorId
模板（`auth-${field}-error`）。两处格式一旦单边调整，ariaFor→fieldErrorId 生成的 aria-describedby 将指向不存在的
id，读屏用户丢失错误播报（测试 login-page.test.tsx:162 恰好 pin 住该关联）。fieldError 在渲染期才被调用，届时 fieldErrorId 已完成初始化，无
TDZ 风险，建议复用：

+   const fieldErrorId = (field: string) => `auth-${field}-error`;
    const fieldError = (field: string): ReactNode => (fieldErrors[field] ?? []).length
-     ? <span id={`auth-${field}-error`} role="alert" className="auth-field-error">{(fieldErrors[field] ?? []).map((key) => t(key)).join(' ')}</span>
+     ? <span id={fieldErrorId(field)} role="alert" className="auth-field-error">{(fieldErrors[field] ?? []).map((key) => t(key)).join(' ')}</span>
      : null;
-   const fieldErrorId = (field: string) => `auth-${field}-error`;


─── apps/web/src/auth/login.td.css:948-951 ───
[bug · low] dark 主题缺少 .auth-toast 的暗色覆盖：§4 自绘 toast 固定白底（background: #fff），而 §5 dark 块未提供
.auth-toast / .auth-toast__text 的对应规则。Vue 端同位置用 MessagePlugin（tdesign 暗色主题自动渲染深色底），dark
模式下每次登录/注册/OIDC 失败都会弹出刺眼白块，与 .register-cta__divider span 被补深色覆盖、.oidc-divider span 被遗漏是同型 parity
缺口。建议在 §5 补齐：

- .auth-toast {
-   position: fixed;
-   left: 50%;
-   top: 32px;
+ html[theme-mode="dark"] .auth-toast {
+   background: rgba(36, 36, 36, 0.97) !important;
+   box-shadow: 0 6px 16px rgba(0, 0, 0, 0.4), 0 0 0 1px rgba(255, 255, 255, 0.08) !important;
+ }


─── apps/web/src/auth/login.td.css:675-678 ───
[maintainability · low] .t-form-item__control 选择器与上方 .t-form-item 一样源自 Vue 源码类名笔误——tdesign
实际渲染的表单控制区类名是 .t-form__controls（复数、无 item 中缀），本规则永不命中（控制区宽度实际由库默认 flex 布局承担）。与上方 .t-form-item
规则不同（该处已附"笔误保真、同样不命中"注释），此处没有任何说明，容易让后续维护者误以为控制区宽度被显式约束。建议补同款保真注释或直接删除。



─── apps/web/src/auth/login.td.css:790-795 ───
[maintainability · low] 1024px 断点的两条 nth-of-type(n + 13) 规则是死规则：.animated-bg 内只有 12 个
.knowledge-node（div）和 12 条 .connection-line（line），n+13 永不匹配任何元素（768px 断点的 n+9 则可正常隐藏第 9-12 号，测试也确认
Vue 就是 12 个节点）。若按事实源原样平移，建议比照文件内 .t-form-item 笔误保真的写法补充注释说明该规则在两端均不生效，否则应删除以免误导后续维护。



─── apps/web/src/integrations/views-integrations-u.css:39-44 ───
[bug · medium] 非法 CSS 声明：`color: color:inherit;` 属性名重复，整条声明会被浏览器按语法错误丢弃，显式继承意图失效。同文件第 64
行（.wk-vi-channel-card-static-class）与第 148 行（.wk-vi-channel-card-title-add-class）存在完全相同的笔误。当前因 color
天然继承暂无可见差异，但这些类挂在 button/h3 等带 UA 默认色的元素上，一旦继承链被切断即出现颜色回归，应修正为 `color: inherit;`。

  .wk-vi-channel-card-clickable-class {
    width: 100%;
    cursor: pointer;
    background-color: #ffffff;
-   color: color:inherit;
+   color: inherit;
  }


─── apps/web/src/integrations/views-integrations-u.css:28-29 ───
[maintainability · low] 直译残留的死声明集合，建议清理为单一生效声明：① 此处 transition-duration 150ms 被下一行 180ms 覆盖（永不生效）；②
.wk-vi-104 的 `border-style: solid` 被后面的 `border-style: dashed` 覆盖；③ .wk-vi-91 / .wk-vi-160 的
`font-size`/`line-height` 先写具体值随后又写 inherit（具体值死）；④ .wk-vi-142 的 `font-size: 12px !important` 压死末尾的
`font-size: inherit`；⑤ .wk-vi-channel-badge-static-class 行尾双分号。均为原 Tailwind 简写冲突的直译产物，干扰后续维护与生效值判读。

-   transition-duration: 150ms;
+   transition-property: border-color,box-shadow;
+   transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
    transition-duration: 180ms;


─── apps/web/src/integrations/views-integrations-u.css:902-904 ───
[bug · low] 非法 calc 表达式：`+` 运算符两侧必须有空格，`calc(100%+4px)` 会被浏览器整条丢弃，Agent 过滤下拉框的 top 回退为
auto（绝对定位静态位置，与触发按钮同行相邻而非其下方 4px）。该值忠实迁移自同样非法的 Tailwind 任意值
`top-[calc(100%+4px)]`，行为与旧版一致，但既然已手工平移为原生 CSS，应顺手修正为合法写法。

  .wk-vi-33 {
    position: absolute;
-   top: calc(100%+4px);
+   top: calc(100% + 4px);


─── apps/web/src/integrations/views-integrations-u.css:334-334 ───
[maintainability · low] 本文件引用的 @keyframes wk-integration-drawer-fade-in /
wk-integration-drawer-slide-in 定义在 apps/web/src/styles.css（L120-121），属跨文件隐式依赖：styles.css 正处于 S6/S7
家族清除期（文件内多处注释记录整族删除），一旦后续清理到这两条关键帧，抽屉进出场动画将静默失效（animation 引用不存在的关键帧不报错）。建议把 @keyframes
就近迁入本文件，或在注释中显式标注依赖位置。

-   animation: wk-integration-drawer-fade-in .18s ease-out;
+ @keyframes wk-integration-drawer-fade-in { from { opacity: 0; } to { opacity: 1; } }
+ @keyframes wk-integration-drawer-slide-in { from { transform: translateX(24px); } to { transform: translateX(0); } }


─── apps/web/src/integrations/integrations-u.css:103-105 ───
[maintainability · low] .wk-apd-13 中 `border-style: solid`（第 4 行）随后被 `border-style: solid
!important` 覆盖，前一条为直译 Tailwind `border border-solid …!` 冲突的死声明，永不生效，建议删除以免干扰生效值判读。

-   border-style: solid;
    border-width: 1px;
    border-style: solid !important;


─── apps/web/src/integrations/integrations.td.css:19-20 ───
[bug · medium] 本节与组件现状脱节，存在死选择器与级联冲突两类问题：① ApiPlaygroundDrawer 已改用
wk-apd-3/4/5/6/9，`-aside`/`-body`/`-section(>h4)`/`-error`/`-step-label`/`-status[data-status]`
全仓无任何消费方（组件与测试均不引用），纯死 CSS；② 仍命中的 `.wk-api-playground-overlay .wk-muted { color:
var(--color-muted,#66758b) }` 与 `.wk-api-playground-overlay .wk-button { color:
var(--color-ink,#172033) }` 特异性 (0,2,0) 高于 integrations-u.css 的 .wk-apd-2/.wk-apd-13
(0,1,0)，且本文件加载在后——实际渲染色 #66758b/#172033（styles.css L430/432 已定义这两个 token）偏离迁移注释声明的
rgba(0,0,0,.6)/rgba(0,0,0,.9)，同一元素双源取值分裂。建议删除死选择器，并将仍需的覆盖并入 integrations-u.css 单一来源或对齐取值。



─── apps/web/src/integrations/integrations.td.css:86-89 ───
[maintainability · medium] 第 3 节整组
`.wk-embed-preview-overlay/-drawer/-header(-h2)/-body/-device/-chrome(+span/code)/-screen` 规则为死
CSS：EmbedPreviewModal 与 page.tsx 的 EmbedChannelPreviewPanel 均已改用
wk-epm-*/wk-vi-*，全仓检索这些旧类名仅命中本文件与注释（embedPreviewFallback.test.tsx/embedWizardRender.test.tsx 注释明言
'replaces the former .wk-embed-preview-drawer class hook'）。注意其中 `.wk-integration-drawer-close`
基础规则仍会命中两个关闭按钮，与 .wk-epm-5 / .wk-vi-integration-drawer-close-class 双源重复且 hover 取值不一致（本文件
var(--color-ink,#172033) vs u.css rgba(0,0,0,.9)，本文件加载在后且胜出）。建议整节删除，关闭按钮样式收敛到单一来源。



─── apps/web/src/integrations/ApiPlaygroundDrawer.tsx:347-347 ───
[bug · medium] 离栈 @weknora/ui Input/Textarea 后，query/agent/external-user 三个字段仅剩 box-sizing
样式（wk-apd-1，integrations.td.css 对这三个类也只补 box-sizing）。已验证：Tailwind 已从 apps/web 全量移除（无
preflight），styles.css 无任何裸 input/textarea 元素规则——字段将以 UA 默认样式渲染（默认边框、不继承 body
字体族的控件字体、默认内距），与整页迁移后的设计明显脱节，构成视觉与可用性回归。建议为这三个类补齐边框/内距/字体口径（对齐页面内 tdesign 输入控件或本域其它表单）。

-               <textarea ref={queryRef} className="wk-api-playground-query wk-apd-1" rows={2} value={form.query} placeholder={t('integrations.api.playgroundQuestionPlaceholder')} onChange={(event) => setForm((prev) => ({ ...prev, query: event.target.value }))} style={fieldStyle} />
+ /* integrations.td.css 或 integrations-u.css */
+ .wk-api-playground-overlay .wk-api-playground-agent,
+ .wk-api-playground-overlay .wk-api-playground-external-user,
+ .wk-api-playground-overlay .wk-api-playground-query {
+   box-sizing: border-box;
+   border: 1px solid var(--wk-border, #e5e7eb);
+   border-radius: 6px;
+   padding: 7px 9px;
+   background: var(--wk-bg, #fff);
+   color: inherit;
+   font-family: inherit;
+   font-size: inherit;
+   line-height: inherit;
+ }


─── packages/views/src/integrations/page.tsx:1916-1918 ───
[bug · low] 注入/回退两条渲染路径口径漂移：注释写 '1em = 14px'，但注入路径（apps/web 注册的 tdesign-icons TIcon）传 15px，回退手绘 svg
为 14px，两路径尺寸不一致。另外 LandingIcon 对 LANDING_ICON_TDESIGN_NAMES 之外的名称直接透传原始
key（`LANDING_ICON_TDESIGN_NAMES[name] ?? name`），注入模式下 TIcon
渲染未知图标名为空白且无回退，健壮性完全依赖映射表完备——新增图标名漏配时将静默消失。建议统一为 14px，并在名称未映射时改走 fallback。

  function JumpIcon() {
-   return <SpriteIcon name="jump" size="15px" fallback={<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" aria-hidden="true"><path d="M9 4L4 4L4 20L20 20L20 15" /><path d="M19.25 4.75L12 12M14 4H20L20 10" /></svg>} />;
+   return <SpriteIcon name="jump" size="14px" fallback={<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" aria-hidden="true"><path d="M9 4L4 4L4 20L20 20L20 15" /><path d="M19.25 4.75L12 12M14 4H20L20 10" /></svg>} />;
  }


─── packages/views/src/integrations/page.tsx:1768-1768 ───
[maintainability · low] ExternalLandingPanel 中 `{false && tab === 'cli' …}`、`{false && tab ===
'chrome' …}`、`{false && tab === 'claw' …}` 三段永假分支为不可达死代码，且本轮迁移还逐段改写了其内部的 wk-vi-* 类名（如
wk-vi-121/122/123/128-132），持续扩大无执行可能的维护面。建议删除这三段，或改为显式特性开关（如常量标志）以表达保留意图。



─── apps/web/src/organizations/OrganizationsPage.tsx:1364-1364 ───
[bug · medium] a11y 回归：迁移到 TDesign 控件时丢失了原生 id，导致 5 处 label 的 htmlFor 关联失效（旧 @weknora/ui 实现中这些控件均带
id）：①create 模式名称输入 `<Input name="organization-name">`（无 id，label
htmlFor="organization-name"）；②upgrade-role 的 TSelect（label
htmlFor="upgrade-role"）；③join-request-role 的 TSelect；④join-code 的 TInput（仅有 name）；⑤join-search 的
TInput（无 id）。label 点击不再聚焦控件，读屏器的 label 关联断裂。建议：TInput/TTextarea 补回 id（属性会透传到内部原生控件）；TSelect 无原生 id
透传时，可将 htmlFor 改为指向包裹元素 id 或给 label 加 aria-labelledby 指向 Select。

-                               <label htmlFor="organization-name">{t(locale, 'organization.name')}{' '}<span className="required">*</span></label>
+ <label htmlFor="organization-name">{t(locale, 'organization.name')}{' '}<span className="required">*</span></label>
+ <!-- 对应控件补回 id： -->
+ <Input id="organization-name" name="organization-name" className="name-input" value={formName} onChange={(value: string) => setFormName(value)} placeholder={t(locale, 'organization.namePlaceholder')} />
+ <!-- TSelect 场景可改为包裹关联： -->
+ <TSelect aria-label={t(locale, 'organization.upgrade.selectRole')} ... />


─── apps/web/src/organizations/SpaceAvatar.tsx:1-2 ───
[maintainability · medium] 共享组件反向依赖页面级样式：`import './org-u.css'` 拉入整份 1531 行页面样式（含全部 wk-org-*
规则），而组件实际只需末尾约 70 行 wk-avatar-*。该组件被 KnowledgeBaseShareDialog / KBShareSettingsSection
引用，knowledge-bases、knowledge-settings 页面会因此被动加载组织页全量样式；后续 org-u.css 的任何改动都会波及无关页面。建议将 wk-avatar-1 ~
wk-avatar-12 拆分为独立的 space-avatar.css（或 wk-legacy 类小文件）由组件自持，org-u.css 中移除该段。

  import type { CSSProperties } from 'react';
- import './org-u.css';
+ import './space-avatar.css'; /* 仅含 wk-avatar-1 ~ wk-avatar-12，由组件自持 */


─── apps/web/src/organizations/org-u.css:928-935 ───
[bug · low] 无效 CSS：`calc(90vh-120px)` 中运算符与操作数之间缺少空格，是不合法的 calc 表达式，该 max-height 声明会被浏览器整条丢弃（该值沿袭自旧
Tailwind 写法 `max-h-[calc(90vh-120px)]`，在旧栈中同样是静默失效的）。虽然外层弹窗已有 max-height:90vh
兜底，实际视觉影响有限，但既然本次已语义化为独立类，建议修正为 `calc(90vh - 120px)` 或直接删除该行，避免无效值被继续固化传播。

  .wk-org-67 {
-   max-height: calc(90vh-120px);
+   max-height: calc(90vh - 120px);
    min-height: 0;
    overflow-x: hidden;
    overflow-y: auto;
    padding-inline: 24px;
    padding-top: 20px;
  }


─── apps/web/src/organizations/OrganizationsPage.tsx:195-197 ───
[maintainability · low] 死代码清理：①IconUsergroup 组件已无任何引用（分组标题与关系标签均已改用 TIcon
name="usergroup"），连同其定义一并删除；②第 14 行具名类型导入 FormEvent 未被使用（三处 submit 处理器实际使用的是
`React.FormEvent`），建议删除或统一改为使用已导入的 FormEvent；③列表渲染尾部 `: ordered.length > 0 ? (<div
className="org-card-wrap">{cardRows}</div>) : null` 的 `: null` 分支不可达（前序分支已穷尽 loading / listError /
ordered.length===0 的组合），可简化条件链。

- const IconUsergroup = ({ size = 14 }: { size?: number }) => (
-   <TIcon size={size}><g stroke="currentColor" strokeWidth="2" strokeLinecap="square"><path d="M16 8a4 4 0 1 1-8 0 4 4 0 0 1 8 0ZM5 19a4 4 0 0 1 4-4h6a4 4 0 0 1 4 4v2H5v-2Z" /><path d="M7 4a4 4 0 1 0 0 8 6 6 0 0 0-6 6v3m22 0v-3a6 6 0 0 0-6-6 4 4 0 0 0 0-8" /></g></TIcon>
- );
+ /* 删除 IconUsergroup 定义；并清理第 14 行未用的 FormEvent 具名导入：
+    import type { CSSProperties, MouseEvent as ReactMouseEvent } from 'react'; */


─── apps/web/src/organizations/OrganizationsPage.tsx:21-22 ───
[maintainability · low] 导入规约：同一模块 tdesign-react 分两条
import，且同一组件持有双别名（TInput/Input、TTextarea/Textarea）在文件内混用——create 模式表单用 Input/Textarea，编辑模式与加入弹框用
TInput/TTextarea，实际渲染完全相同。双别名会让读者误以为存在两套行为差异的组件，后续维护也容易拿错名字。建议合并为一条导入，并统一使用其中一套命名（保留被外部语义引用更明确的一套）。

- import { Input as TInput, Select as TSelect, Switch as TSwitch, Textarea as TTextarea } from 'tdesign-react';
- import { Button, Dialog, Input, Popup, Skeleton, Tag, Textarea, Tooltip } from 'tdesign-react';
+ import { Button, Dialog, Input, Popup, Select, Skeleton, Switch, Tag, Textarea, Tooltip } from 'tdesign-react';
+ /* 文件内统一直接使用 Input / Select / Switch / Textarea（删除 T 前缀别名用法） */


─── apps/web/src/settings/PersonalMemoryPanel.tsx:196-196 ───
[bug · medium] InputNumber 的 onChange 直接 Number(value) 且立即触发 debouncedSave，缺少空值防护（此处及 L206/L254/L283
共 4 处同模式）。TDesign InputNumber 被清空时 value 为 undefined/空串：Number(undefined) 得 NaN（经 JSON 序列化为 null
落库），Number('') 得 0（低于 max_items min=10、interest_threshold min=1 的合法下限）。由于本面板是 500ms
防抖自动保存，用户清空输入重输的瞬间就会把非法值静默写入服务端配置。同批 ModelSettingsPanel 已提供 fromTInputNumber 处理
string/空值情形，建议提取共用防护，空值时不更新草稿、不触发保存。

- <InputNumber value={draft.extract_delay_seconds} min={5} max={3600} step={15} suffix="s" disabled={!canEdit} onChange={(value) => { update({ extract_delay_seconds: Number(value) }); debouncedSave(); }} />
+ // 与 ModelSettingsPanel 的 fromTInputNumber 提取为共用辅助：
+ // function fromTInputNumber(value: number | string | undefined | null): number | '' {
+ //   if (typeof value === 'number') return value;
+ //   if (value == null || value.trim() === '') return '';
+ //   const parsed = Number(value);
+ //   return Number.isFinite(parsed) ? parsed : '';
+ // }
+ <InputNumber value={draft.extract_delay_seconds} min={5} max={3600} step={15} suffix="s" disabled={!canEdit} onChange={(value) => { const parsed = fromTInputNumber(value); if (parsed !== '') { update({ extract_delay_seconds: parsed }); debouncedSave(); } }} />


─── apps/web/src/settings/PersonalMemoryPanel.tsx:49-49 ───
[bug · medium] readDraft 的 num 回退把 0 一律视为缺失（row[key] !== 0），但 extract_min_interval_seconds 的输入约束是
min={0}（L206），用户显式保存 0 后重新加载会被回显为默认值 300，造成配置往返失真。迁移前的旧实现（typeof row.extract_min_interval_seconds
=== 'number' ? ... : 300）是保留 0 的，此处属于回归。注释声称对齐 Vue 的 cfg.x || default，但与同面板自身的 min=0 输入约束矛盾——若 0
不是合法值应同步收紧输入框 min。建议改为 Number.isFinite 判定，仅将缺失/非数值回退默认。

- const num = (key: string, fallback: number): number => (typeof row[key] === 'number' && row[key] !== 0 ? row[key] as number : fallback);
+ const num = (key: string, fallback: number): number => (typeof row[key] === 'number' && Number.isFinite(row[key]) ? row[key] as number : fallback);


─── apps/web/src/settings/ModelSettingsPanel.tsx:964-964 ───
[maintainability · low] TLoading 以硬编码 loading={false} 包裹整个模型列表，属恒假死代码：组件没有任何 loading 状态，无任何路径会置
true，该包装永远不会显示加载态，只增加一层无意义的 DOM/语义噪音。要么移除包装（连同结尾 </TLoading>），要么接入真实的加载状态。

- <TLoading loading={false} size="small" className="model-list-loading">
+ // 移除 <TLoading loading={false} ...> 与对应 </TLoading>，或接入真实 loading 状态：
+ // const [loading, setLoading] = useState(false);
+ // <TLoading loading={loading} size="small" className="model-list-loading">


─── apps/web/src/settings/PlatformApiKeysPanel.tsx:5-5 ───
[maintainability · low] 第二条 tdesign-react import 中的 Button 全文件未使用（实际全部通过 TButton 别名引用），且与第 2
行构成同模块重复导入。删除未用的 Button，并将 Alert 合并进首条 import。

- import { Alert, Button } from 'tdesign-react';
+ import { Alert } from 'tdesign-react';
+ // 并建议与文件顶部 import { Button as TButton, Checkbox as TCheckbox, Input as TInput } from 'tdesign-react'; 合并为一条


─── apps/web/src/settings/PersonalMemoryPanel.tsx:118-118 ───
[maintainability · low] as never 强转不必要且有害：memoryWorkspacePatch 的返回类型是 Record<string, unknown>，与
client.settings.memory.workspace.update 的入参类型 SettingsPayload（= Record<string, unknown>，见
packages/api-client/src/settings/index.ts kvApi.update）完全兼容，且此处六个实参与函数签名逐项匹配，无需任何强转。as never 使实参类型变为
never（可赋给任何形参），一旦后续 surface.ts 签名或 API 客户端类型漂移，该调用点将不再报编译错误，产生静默数据错误。直接移除强转即可恢复类型检查。

-           ) as never);
+           ));


─── apps/web/src/settings/PersonalMemorySettingsPanel.tsx:979-982 ───
[style · low] 文件末尾缺少换行符（diff 中有 \ No newline at end of file 标记），不符合 POSIX 文本文件约定，也会在后续 diff
中产生噪音，请补上行尾换行。



─── apps/web/src/integrations/views-integrations-u.css:2286-2289 ───
[bug · low] 分段按钮（principalMode 单选组）激活态右边框被错误灰化。page.tsx 中该规则无条件加在第三个按钮上（`(index === 2 ? ' wk-vi-167'
: '')`），与激活态 `.wk-vi-165 { border-color: #07c05f }` 同处 @layer
utilities、同为单类选择器（0,1,0）且源序在后——选中『签名令牌』时其 border-right-color 被 #dcdcdc 覆盖，绿色激活按钮右侧出现灰边。这同时偏离迁移前渲染（旧
utilities 无此规则）与 Vue 事实源（ApiIntegrationSettings.vue:203-207 用 t-radio-button，选中态整边品牌色）。且
`.wk-vi-166:hover` 带 !important 能压过本规则而 `.wk-vi-165` 不能，级联行为自相矛盾（hover
蓝边、激活灰边）。建议删除该规则，或把灰右边框限定在未激活态。

- .wk-vi-167 {
+ /* 限定灰右边框仅未激活态生效（page.tsx 侧无需改动） */
+ .wk-vi-166.wk-vi-167 {
    border-right-style: solid;
    border-right-color: #dcdcdc;
  }


─── apps/web/src/integrations/views-integrations-u.css:2346-2350 ───
[bug · low] 同一文件对旧栈 text-muted-strong 存在两套映射基线：此处 chip 未选中文字取 #506078（注释称 token 值来自 ui theme.css），而
.wk-vi-int-tab-class、.wk-vi-3、.wk-vi-11、.wk-vi-48、.wk-vi-85 等按本文件 ledger 统一映射为 rgba(0,0,0,0.6)（Vue
var(--td-text-color-secondary)）。迁移前 chip 与 tab 胶囊的未选中文字同为
text-muted-strong（同色），迁移后集成页同屏出现两种灰，视觉口径不一致。建议统一基线：与 int-tab 一致用 rgba(0,0,0,0.6)，或全部改引
var(--color-muted-strong, #506078)。

  .wk-vi-chip--idle {
    border-color: #dce3ed;
    background-color: #ffffff;
-   color: #506078;
+   /* 与 .wk-vi-int-tab-class 等同基线：旧栈 muted-strong→rgba(0,0,0,0.6) */
+   color: rgba(0, 0, 0, 0.6);
  }


─── apps/web/src/integrations/IntegrationsPage.tsx:2-3 ───
[maintainability · low] 本文件运行时不可达：路由实际挂载的是 packages/views 的 IntegrationsPage（经 IntegrationsRoutePage
→ @weknora/views/integrations/page），全仓库对本文件的唯一引用是 route.test.ts:71-79 的源码字符串扫描断言（readFileSync
读取本文件源码后 match 标记）。本次变更为这份不可达代码做了 tdesign 化改造（TAlert/TDrawer/TInput），等于持续为死代码投入维护成本，且与 views
包版本形成两份会随时间漂移的 IntegrationsPage 实现。建议删除本文件并把 route.test.ts 的源码断言改指向 packages/views
版本；若确需保留作为测试契约锚点，请在文件头注释显式声明其定位。



─── apps/web/src/organizations/OrganizationsPage.tsx:1026-1027 ───
[bug · medium] 键盘可操作性回归：旧 React 实现中组织卡片是 role="button" tabIndex={0} 并带 onKeyDown（Enter
打开设置弹窗），更多菜单按钮（现 more-wrap）也有 role="button"/tabIndex/aria-label/onKeyDown。本次迁移为纯 onClick 的 div
后全部丢失，键盘/辅助技术用户无法进入组织设置（页面的核心操作），更多菜单同样不可达。这些属性不新增 DOM 节点，不会破坏 ix-orgs-* 扫描态的 DOM 结构复刻，建议恢复。

        <div key={org.id || index} className={'org-card' + (owner ? '' : ' joined-org')} style={rowHidden ? { display: 'none' } : undefined}
-         onClick={() => openSettingsModal(org)}>
+         role="button" tabIndex={0}
+         onClick={() => openSettingsModal(org)}
+         onKeyDown={(event) => { if (event.key === 'Enter') openSettingsModal(org); }}>
+         {/* more-wrap 同样补回：role="button" tabIndex={0} aria-label={t(locale, 'common.moreActions')} + onKeyDown */}


─── apps/web/src/settings/ModelSettingsPanel.tsx:112-115 ───
[bug · high] fromTInputNumber 缺少 null/undefined 防护：TDesign InputNumber 在输入框被清空时 onChange 回调的 value 为
null（同批 PersonalMemoryPanel 的同类问题已实测确认），null 进入后 `typeof null === "object"` 走不进首分支，`null.trim()` 直接抛
TypeError，导致 dimension / contextWindow / maxConcurrency 三个数字输入框（L1442/L1468/L1503）一清空就崩溃（React
事件处理器异常），编辑器无法正常清空重输。参数类型 `number | string` 也与 TDesign 实际回调值不符。建议显式接收 null/undefined 并返回空串语义。

- function fromTInputNumber(value: number | string): number | "" {
+ function fromTInputNumber(value: number | string | null | undefined): number | "" {
    if (typeof value === "number") return value;
-   return value.trim() === "" ? "" : Number(value);
+   if (value === null || value === undefined || value.trim() === "") return "";
+   return Number(value);
  }


─── apps/web/src/settings/ModelSettingsPanel.tsx:1035-1035 ───
[bug · medium] 模型卡删除按钮丢失了旧实现自带的 `disabled={busy}`：busy（删除/复制/保存进行中 setBusy(true)）期间按钮仍可点击并弹出
Popconfirm，用户确认后 remove() 入口的 `busy` 守卫静默 no-op，表现为「确认删除无响应」。与旧代码（disabled={busy}）相比属迁移回归，建议恢复禁用条件。

- <TButton theme="danger" shape="square" variant="text" size="small" className="model-card__action-btn model-card__delete" icon={<TIcon name="delete" />} onClick={(event) => event.stopPropagation()} />
+ <TButton theme="danger" shape="square" variant="text" size="small" className="model-card__action-btn model-card__delete" disabled={busy} icon={<TIcon name="delete" />} onClick={(event) => event.stopPropagation()} />


─── apps/web/src/settings/OllamaSettingsPanel.tsx:166-170 ───
[style · low] 状态徽标为三层嵌套三元（testing → connectionStatus === true → === false → 兜底），下方已装模型区
`loadingModels ? ... : models.length > 0 ? ... : 空态` 又是一处嵌套三元，可读性差且违反评审基线（禁止嵌套三元）。建议抽成局部渲染函数（如
renderStatusTag() / renderModelList()）后按序分支返回。



─── apps/web/src/settings/ParserEngineSettingsPanel.tsx:375-376 ───
[style · low] EngineCard 的根节点是 <button type="button">，其内容模型只允许 phrasing content，此处新增的
div.engine-card__badge / div.engine-card__body（内含 h3/p）违反 HTML 规范，可能影响辅助技术语义与浏览器解析；迁移前旧实现用的是 span
包装。建议恢复 span 承载（对应闭合标签同步调整）。

-     <div className="engine-card__badge">{initial}</div>
-     <div className="engine-card__body">
+     <span className="engine-card__badge">{initial}</span>
+     <span className="engine-card__body">


─── apps/web/src/settings/ParserEngineSettingsPanel.tsx:15-18 ───
[documentation · low] 头注与代码自相矛盾：注释称「配置抽屉沿用 React 表单栈…待后续批次收编」，但本次改动中 EngineDrawer 的
Input/Select/Checkbox/Alert/Loading/Tooltip 已全部换成 tdesign-react 的
TInput/TSelect/TCheckbox/Alert/Loading/Tooltip。注释会误导后续批次误判收编范围（以为抽屉还在 @weknora/ui
旧栈），建议同步更新注释说明抽屉已完成 TDesign 化。



LLM retry report summary: 19 of 265 requests affected -- 4 requests failed, 3 requests cancelled, 12 requests recovered after retry

Review planning (2 requests):
- apps/web/src/agents/AgentEditorModal.tsx,apps/web/src/agents/AgentParserRules.tsx,apps/web/src/agents/AgentsPage.tsx,apps/web/src/agents/MbtiTestModal.tsx,apps/web/src/agents/PersonaSection.tsx,apps/web/src/agents/SubagentsSection.tsx,apps/web/src/agents/agents-u.css,apps/web/src/agents/agents.css,apps/web/src/agents/agents.td.css,apps/web/src/agents/list.ts: timed out -> failed
- apps/web/src/wiki/WikiPage.tsx,apps/web/src/wiki/wiki-reader.css,apps/web/src/wiki/wiki-u.css: rate limited (HTTP 429) -> succeeded

Core review (12 requests):
- apps/web/src/agents/AgentEditorModal.tsx,apps/web/src/agents/AgentParserRules.tsx,apps/web/src/agents/AgentsPage.tsx,apps/web/src/agents/MbtiTestModal.tsx,apps/web/src/agents/PersonaSection.tsx,apps/web/src/agents/SubagentsSection.tsx,apps/web/src/agents/agents-u.css,apps/web/src/agents/agents.css,apps/web/src/agents/agents.td.css,apps/web/src/agents/list.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/integrations/ApiPlaygroundDrawer.tsx,apps/web/src/integrations/EmbedPreviewModal.tsx,apps/web/src/integrations/IntegrationsPage.tsx,apps/web/src/integrations/IntegrationsRoutePage.tsx,apps/web/src/integrations/integrations-u.css,apps/web/src/integrations/integrations.td.css,apps/web/src/integrations/views-integrations-u.css,packages/views/src/integrations/page.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/agents/AgentEditorModal.tsx,apps/web/src/agents/AgentParserRules.tsx,apps/web/src/agents/AgentsPage.tsx,apps/web/src/agents/MbtiTestModal.tsx,apps/web/src/agents/PersonaSection.tsx,apps/web/src/agents/SubagentsSection.tsx,apps/web/src/agents/agents-u.css,apps/web/src/agents/agents.css,apps/web/src/agents/agents.td.css,apps/web/src/agents/list.ts: rate limited (HTTP 429) -> succeeded
- apps/web/src/agents/AgentEditorModal.tsx,apps/web/src/agents/AgentParserRules.tsx,apps/web/src/agents/AgentsPage.tsx,apps/web/src/agents/MbtiTestModal.tsx,apps/web/src/agents/PersonaSection.tsx,apps/web/src/agents/SubagentsSection.tsx,apps/web/src/agents/agents-u.css,apps/web/src/agents/agents.css,apps/web/src/agents/agents.td.css,apps/web/src/agents/list.ts: rate limited (HTTP 429) -> succeeded
- apps/web/src/agents/AgentEditorModal.tsx,apps/web/src/agents/AgentParserRules.tsx,apps/web/src/agents/AgentsPage.tsx,apps/web/src/agents/MbtiTestModal.tsx,apps/web/src/agents/PersonaSection.tsx,apps/web/src/agents/SubagentsSection.tsx,apps/web/src/agents/agents-u.css,apps/web/src/agents/agents.css,apps/web/src/agents/agents.td.css,apps/web/src/agents/list.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- ... and 7 more

Context compaction (5 requests):
- apps/web/src/wiki/WikiPage.tsx,apps/web/src/wiki/wiki-reader.css,apps/web/src/wiki/wiki-u.css: rejected by provider (HTTP 400) -> failed
- apps/web/src/agents/AgentEditorModal.tsx,apps/web/src/agents/AgentParserRules.tsx,apps/web/src/agents/AgentsPage.tsx,apps/web/src/agents/MbtiTestModal.tsx,apps/web/src/agents/PersonaSection.tsx,apps/web/src/agents/SubagentsSection.tsx,apps/web/src/agents/agents-u.css,apps/web/src/agents/agents.css,apps/web/src/agents/agents.td.css,apps/web/src/agents/list.ts: cancelled
- apps/web/src/agents/AgentEditorModal.tsx,apps/web/src/agents/AgentParserRules.tsx,apps/web/src/agents/AgentsPage.tsx,apps/web/src/agents/MbtiTestModal.tsx,apps/web/src/agents/PersonaSection.tsx,apps/web/src/agents/SubagentsSection.tsx,apps/web/src/agents/agents-u.css,apps/web/src/agents/agents.css,apps/web/src/agents/agents.td.css,apps/web/src/agents/list.ts: cancelled
- apps/web/src/integrations/ApiPlaygroundDrawer.tsx,apps/web/src/integrations/EmbedPreviewModal.tsx,apps/web/src/integrations/IntegrationsPage.tsx,apps/web/src/integrations/IntegrationsRoutePage.tsx,apps/web/src/integrations/integrations-u.css,apps/web/src/integrations/integrations.td.css,apps/web/src/integrations/views-integrations-u.css,packages/views/src/integrations/page.tsx: cancelled
- apps/web/src/agents/AgentEditorModal.tsx,apps/web/src/agents/AgentParserRules.tsx,apps/web/src/agents/AgentsPage.tsx,apps/web/src/agents/MbtiTestModal.tsx,apps/web/src/agents/PersonaSection.tsx,apps/web/src/agents/SubagentsSection.tsx,apps/web/src/agents/agents-u.css,apps/web/src/agents/agents.css,apps/web/src/agents/agents.td.css,apps/web/src/agents/list.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
