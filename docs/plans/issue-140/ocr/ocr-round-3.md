Review partially complete: 186 finding(s); 154 of 319 selected item(s) failed.

─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:129-130 ───
[documentation · medium] 第 6 步注释与脚本实际行为不符：本脚本不含任何 HTTP 请求代码（无 http/fetch 调用），但注释第一行「直接以同 token 发起真实
POST……验证失败态」读起来是本脚本执行的动作。经核对，真实 POST /api/v1/career/searches 并验证 failureCode=no_vetted_sources 的是
T24 的 t24r1-live-driver.cjs（其 api() 函数），本脚本并未执行。作为与其他步骤并列的编号步骤却无对应代码，会让证据评审者误判 T33
的验证覆盖范围。建议改写为明确说明该验证不在本脚本执行。

-     // 6. 找岗执行（一次性搜索）走 request-layer seam：直接以同 token 发起真实 POST（T24 先例）验证失败态
-     // （已在 Web 端真实执行过一次搜索并取得 no_vetted_sources 失败态；小程序侧由 172 单测覆盖 D2/D3/D4）
+     // 6. 找岗执行（一次性搜索）的失败态验证不在本脚本内执行：真实 POST /api/v1/career/searches 已由
+     // T24 live-driver 以同 token 完成（failureCode=no_vetted_sources）；小程序侧行为由 172 单测覆盖 D2/D3/D4。


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:80-80 ───
[test · medium] tapListRow 遍历 page.$$('view') 并 tap 首个文本包含目标串的节点：小程序中 view 的 text
会聚合子孙节点文案，且祖先容器在文档顺序上先返回——登录页 consent 行外层的 Card/CheckboxGroup 容器 view
同样包含「我已了解平台的数据使用与服务说明」，首个命中的大概率是外层容器而非 .wk-consent wrapper。tap 落点偏离 checkbox 后勾选不生效，登录按钮因
disabled={!consent||...} 点击无效，整个 T33 流程将在登录前置步骤提前 throw。T24 live-driver
已有精确选择器先例（page.$('.wk-consent checkbox') || page.$('checkbox')），建议对齐而非按文本遍历 view。

-     const consentOk = await tapListRow(page, '我已了解平台的数据使用与服务说明', 8000);
+     const box = (await page.$('.wk-consent checkbox')) || (await page.$('checkbox'));
+     let consentOk = false;
+     if (box) { try { await box.tap(); consentOk = true; } catch {} }


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:75-76 ───
[maintainability · low] 脚本头部承诺「机器强相关一律 env 注入」并给出换机重放示例，但登录账号 t33a@t33.io
硬编码（密码已是脱敏占位值，重放本就需重新提供）。一次性测试账号属环境相关数据，换机重放时该账号可能已删除/密码已失效，导致登录步骤静默 FAIL 且需改脚本。UI
文案与路由（请输入账号邮箱/登录并继续/进入工作空间/wk-button/career 分包路径）已核对与当前小程序代码一致，可保留；账号建议与 T33_AUTOMATOR 等同样走 env 注入。

-     await fillInput(page, '请输入账号邮箱', 't33a@t33.io');
-     await fillInput(page, '请输入密码', '[REDACTED-disposable]');
+     const ACCOUNT = process.env.T33_ACCOUNT || 't33a@t33.io';
+     const PASSWORD = process.env.T33_PASSWORD || '[REDACTED-disposable]';
+     await fillInput(page, '请输入账号邮箱', ACCOUNT);
+     await fillInput(page, '请输入密码', PASSWORD);


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:47-56 ───
[maintainability · low] tapText 与 tapListRow 除选择器（'button, .wk-button' 与 'view'）外逻辑逐行相同（deadline 轮询
+ 文本包含匹配 + tap + 返回布尔）。建议提取带 selector 参数的公共重试 tap 函数，两个调用点改为薄封装，避免后续修改轮询间隔/超时策略时出现双份维护漂移。

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


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:11-11 ───
[maintainability · low] 空 catch 会把「模块存在但损坏/依赖缺失」等真实加载错误一并吞掉，最终统一抛出泛化的 "not
found"，排障时无法区分「路径未配置」与「安装损坏」。allTexts/shot 处的空 catch 属驱动脚本可接受的容错，但模块加载失败建议区分错误类型：仅 MODULE_NOT_FOUND
时继续尝试下一个候选路径，其余错误直接抛出。

-   for (const candidate of candidates) { try { return require(candidate); } catch {} }
+   for (const candidate of candidates) {
+     try { return require(candidate); } catch (e) { if (e.code !== 'MODULE_NOT_FOUND') throw e; }
+   }


─── apps/miniprogram/config/index.ts:40-43 ───
[maintainability · low] tdesignClosure 隐式假设 usingComponents 的引用均为“无扩展名的相对路径”，但该假设未被防御：(1) 若升级后出现以
'/' 开头的绝对引用，会被当作相对段拼进父目录（'button/button' + '/icon/icon' → 'button/icon/icon'），报出误导性的 missing 路径；(2)
连续 '..' 越界时 out.pop() 在空数组上静默吞掉。当前 1.17.0 精确锁版下假设成立，但与注释“TDesign 升级新增依赖自动跟进”的承诺不匹配，建议对绝对引用和越界 pop
显式抛出带 ref 的错误。另：下方 tdesignDirs 校验中 readdirSync 对 'common'/'miniprogram_npm' 缺失时会抛裸 ENOENT，建议先
existsSync 给出与 button 同级的友好报错。

+    if(ref.startsWith('/'))throw new Error(`tdesign closure: absolute component ref unsupported: ${ref}`);
     const segs=[...base.split('/'),...ref.split('/')];
     const out:string[]=[];
-    for(const s of segs){if(s==='.'||s==='')continue;if(s==='..')out.pop();else out.push(s)}
+    for(const s of segs){if(s==='.'||s==='')continue;if(s==='..'){if(!out.length)throw new Error(`tdesign closure: ref escapes dist root: ${ref}`);out.pop()}else out.push(s)}
     queue.push(out.join('/'));


─── apps/miniprogram/tests/build-output.test.mjs:103-104 ───
[maintainability · low] deepEqual 精确锁定目录集合与 config/index.ts 中“闭包在构建期按 usingComponents 递归收集、TDesign
升级新增依赖自动跟进”的设计意图相矛盾：升级后闭包正确扩展（如 button 新增 popup 依赖）时构建产物是对的，但此测试反而失败，需要人工同步维护这份清单。建议改为“子集”断言（实际目录 ⊆
允许清单），既保留“防回归到全量拷贝”的意图，又不阻塞合法升级。

+   const allowed = new Set(['button', 'common', 'icon', 'loading', 'miniprogram_npm']);
    const dirs = readdirSync(tdesignDir, { withFileTypes: true }).filter(e => e.isDirectory()).map(e => e.name).sort();
-   assert.deepEqual(dirs, ['button', 'common', 'icon', 'loading', 'miniprogram_npm'].sort(), 'tdesign 拷贝范围必须收窄到 button 闭包 + miniprogram_npm');
+   const unexpected = dirs.filter(d => !allowed.has(d));
+   assert.deepEqual(unexpected, [], 'tdesign 拷贝范围必须收窄到 button 闭包 + miniprogram_npm（防回归到全量拷贝）');


─── apps/miniprogram/tests/build-output.test.mjs:92-93 ───
[test · low] 该正则假设产物是特定的压缩形态（from"…"）：TDesign 或压缩器输出格式一变（如 from '…' 或
require("../common/…")），matchAll 零匹配时循环体不执行，断言静默空转——测试仍通过但不再校验 common
运行时依赖的存在。建议放宽匹配形态并在零匹配时显式失败，让格式变化可感知。

    const buttonJs = readFileSync(resolve(tdesignDir, 'button/button.js'), 'utf8');
-   for (const m of buttonJs.matchAll(/from"(\.\.\/common[^"]*)"/g)) {
+   const matches = [...buttonJs.matchAll(/(?:from|require\()\s*["'](\.\.\/common[^"']*)["']/g)];
+   assert.ok(matches.length > 0, 'button.js 应至少声明一个 ../common 运行时依赖（产物格式变化时需同步更新匹配规则）');
+   for (const m of matches) {


─── apps/miniprogram/src/app.config.ts:45-48 ───
[maintainability · low] 新分包根目录为 'career'，偏离本文件其余分包统一的 'subpackages/<域>' + '<页>/index'
命名约定（subpackages/auth、subpackages/execution、subpackages/knowledge、subpackages/account）。src/career/
源码目录与 routes.ts 也随之使用顶层 'career/...' 路径，功能上没问题，但后续做分包治理（体积分析、按 root
前缀批量过滤等）时这类例外容易被漏算。若非刻意保留顶层命名，建议对齐为 'subpackages/career'（需同步迁移 src/career 目录结构与
routes.ts、build-output.test.mjs 中的对应路径）。

      {
-       "root": "career",
+       "root": "subpackages/career",
        "pages": [
-         "discovery",
+         "discovery/index",


─── apps/web/src/router.tsx:718-719 ───
[bug · high] careerRoute 的 component 直接渲染 lazy 组件 CareerPage，未像同批新增的
careerSearchRoute/careerRulesRoute/careerOpportunityRoute/careerEvaluationRoute
以及文件内所有其他惰性路由（agentsRoute/expertsRoute 等）一样包裹 <Suspense fallback={<RoutePending ...>}>。已确认本文件未配置
defaultPendingComponent，main.tsx 根渲染也无外层 Suspense 边界：首次导航到 /platform/career 时惰性 chunk 加载期间组件挂起无任何
fallback 兜底，React 19 下会抛出「suspended without fallback UI」错误，career 首页（本功能的核心入口）首次进入白屏/报错。

      path: 'career',
-     component: (): ReactNode => <CareerPage client={client} scopeController={scopeController} userId={scopeController.current().scope.userId} />,
+     component: (): ReactNode => (
+       <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
+         <CareerPage client={client} scopeController={scopeController} userId={scopeController.current().scope.userId} />
+       </Suspense>
+     ),


─── apps/web/src/router.tsx:716-718 ───
[documentation · low] 上方的「Octop M2 expert-template catalog…routes_expert.go guard」注释原本描述
expertsRoute，本次 career 路由块插入后二者被隔开（expertsRoute 现位于约 56 行之后），注释现悬挂在 careerRoute 上方，会被误读为 career
路由的职责/权限说明。建议把该注释移回 expertsRoute 定义处，并为 careerRoute 补一句自己的说明。

+   // Issue #140 career desk entry (Viewer+; server-side scope guards in routes_career.go).
    const careerRoute = createRoute({
      getParentRoute: () => platformRoute,
      path: 'career',


─── apps/web/src/routes.tsx:58-59 ───
[maintainability · low] career-opportunity 在此强制要求 snapshotId 非空才返回匹配，而 router.tsx 的
careerOpportunityRoute 对缺失 snapshotId 容错为空串并照常渲染 OpportunityEvidencePage（后者对空串有页面内 invalid 状态兜底）。同一
URL 在两套解析层表现不同：经 resolveRoute 的初始入口（main.tsx 分流/守卫）判 404，TanStack
路由内导航则进入页面内无效提示。建议统一口径：要么此处与渲染层一致地容错（交给页面 invalid 兜底），要么渲染层对空 snapshotId 重定向到
404。另：该行同时承担三值判断+对象构造，建议拆开以提升可读性。

-     const snapshotId = query.get('snapshotId')?.trim();
-     return opportunityId && snapshotId ? { kind: 'career-opportunity', path, opportunityId, snapshotId } : { kind: 'not-found', path };
+     const snapshotId = query.get('snapshotId')?.trim() ?? '';
+     // 与 router.tsx careerOpportunityRoute 的容错口径一致：空 snapshotId 交给
+     // OpportunityEvidencePage 的 invalid 状态兜底，两个入口不分裂。
+     return opportunityId ? { kind: 'career-opportunity', path, opportunityId, snapshotId } : { kind: 'not-found', path };


─── apps/web/src/routes.tsx:93-93 ───
[maintainability · low] 平台路径白名单继续以超长 || 全等链膨胀（本次新增
/platform/career、/platform/career/search、/platform/career/rules 三条）。该写法一处遗漏即静默 404 且难以审查，建议收敛为数组 +
includes（或前缀表）驱动，例如 const PLATFORM_PATHS = new Set([...]) 后 if (PLATFORM_PATHS.has(path) ||
path.startsWith('/platform/chat/') ...) 。本次延续既有风格可接受，但链已接近不可维护。

-   if (path === '/platform' || path === '/platform/knowledge-bases' || path === '/platform/knowledge-search' || path === '/platform/career' || path === '/platform/career/search' || path === '/platform/career/rules' || path === '/platform/agents' || path === '/platform/experts' || path === '/platform/market' || path === '/platform/integrations' || path === '/platform/creatChat' || path === '/platform/tenant' || path === '/platform/organizations' || path === '/platform/analytics' || path === '/platform/settings' || path === '/platform/configuration' || path === '/platform/administration' || path === '/platform/system' || path === '/platform/system/settings' || path === '/platform/system/admins' || path === '/platform/system/queues' || path === '/platform/billing' || path === '/platform/billing/checkout' || path === '/platform/billing/admin' || (development && path === '/platform/dev/markdown') || path.startsWith('/platform/chat/') || path.startsWith('/platform/shared/')) return { kind: 'platform', path };
+   const PLATFORM_EXACT_PATHS = new Set([
+     '/platform', '/platform/knowledge-bases', '/platform/knowledge-search',
+     '/platform/career', '/platform/career/search', '/platform/career/rules',
+     '/platform/agents', '/platform/experts', '/platform/market',
+     '/platform/integrations', '/platform/creatChat', '/platform/tenant',
+     '/platform/organizations', '/platform/analytics', '/platform/settings',
+     '/platform/configuration', '/platform/administration', '/platform/system',
+     '/platform/system/settings', '/platform/system/admins', '/platform/system/queues',
+     '/platform/billing', '/platform/billing/checkout', '/platform/billing/admin',
+     ...(development ? ['/platform/dev/markdown'] : []),
+   ]);
+   if (PLATFORM_EXACT_PATHS.has(path) || path.startsWith('/platform/chat/') || path.startsWith('/platform/shared/')) return { kind: 'platform', path };


─── apps/web/src/DevMarkdownPage.tsx:259-262 ───
[bug · medium] 后置正则会无差别改写所有 <em><strong>X</strong></em>，而该函数同时被 customInput（用户在 textarea 输入的任意
markdown，见第 456 行 customHtml）和 streamBuffer 调用。Vue 端 repairFlankingEmphasis 只在 `*`/`**` 标点 flanking
场景触发（且是有条件触发）：用户输入 `_**粗体**_`（下划线定界）时 marked 输出 <em><strong>粗体</strong></em>，Vue 正常渲染为粗斜体，本页却错误改写为
<em><em></em>粗体</em>**，反而与该 parity 页要对齐的 Vue 事实源不一致。固定 fixture 中仅 basic 的 `***加粗斜体***`
是预期触发源，建议至少将改写限定在源文本含 *** 定界时，长期方案是把 repairFlankingEmphasis 前置 pass 移植进共享渲染器。

-   return renderChatMarkdown(markdown).replace(
-     /<em><strong>([^<]*)<\/strong><\/em>/g,
-     '<em><em></em>$1</em>**',
-   );
+   const html = renderChatMarkdown(markdown);
+   // 仅当源文本确实含 *** 三重强调（Vue repairFlankingEmphasis 的触发源）时
+   // 做终态改写，避免 _**粗体**_ 等正常嵌套被误伤偏离 Vue 事实源。
+   return /\*\*\*[^*\n]+\*\*\*/.test(markdown)
+     ? html.replace(/<em><strong>([^<]*)<\/strong><\/em>/g, '<em><em></em>$1</em>**')
+     : html;


─── apps/web/src/main.tsx:20-20 ───
[maintainability · low] career 目录下其余 11 个页面组件（SearchPage/RulePage/MaterialPage 等）均在组件文件内按需引入自己的
css，唯独 OpportunityPage.tsx 未引入 opportunity.css，而由 main.tsx 在入口全局静态引入。后果：① 该样式进入主 bundle，所有用户（包括从不访问
career 页面的用户）无条件加载；② 无法随 career 惰性 chunk 分包；③ 与同目录加载策略不一致。建议移入 OpportunityPage.tsx 顶部 import
'./opportunity.css'。

- import './career/opportunity.css';
+ // 移除本行，改为在 apps/web/src/career/OpportunityPage.tsx 顶部：
+ // import './opportunity.css';


─── apps/miniprogram/tests/progress-preparation.test.mjs:426-431 ───
[maintainability · medium] R1-P4 用内联代码逐句复刻 progress-preparation.tsx saveRevision（127-139
行）的保存链组合（editable 映射 → bodyFromEditable → 空 claims 条件分支 → recoverEmptyClaims →
editMaterial），而非调用共享函数。bodyFromEditable/recoverEmptyClaims
本身是共享的，但「组合顺序与触发条件」是平行复制：一旦页面保存链调整（如改为无条件取回主张、或先经 attachClaimsFromServer
合并），本测试仍按旧链通过，「主张不丢失」的回归保护会静默失效。建议把保存链组合提取为共享函数（如 career-platform.ts 导出
composeClaimPreservingBody(sections, materialId)），页面 saveRevision 与本测试共用同一实现。

-   const editable = draft.sections.map(section => ({ heading: section.heading, content: section.content, claims: Array.isArray(section.claims) ? section.claims : [] }));
-   let body = career.bodyFromEditable(editable);
-   if (body.sections.some(section => (section.claims ?? []).length === 0)) {
-     body = { sections: platform.recoverEmptyClaims(body.sections, (await career.material('mat-9')).body.sections) };
-   }
-   await career.editMaterial({ materialId: 'mat-9', body });
+ // 提取到 src/adapters/career-platform.ts（页面 saveRevision 与测试共用）：
+ // export async function composeClaimPreservingBody(sections: EditableMaterialSection[], materialId: string): Promise<MaterialBody> {
+ //   let body = career.bodyFromEditable(sections);
+ //   if (body.sections.some(section => (section.claims ?? []).length === 0)) {
+ //     body = { sections: recoverEmptyClaims(body.sections, (await career.material(materialId)).body.sections) };
+ //   }
+ //   return body;
+ // }
+ // 测试侧：const body = await platform.composeClaimPreservingBody(draft.sections, 'mat-9');
+ //         await career.editMaterial({ materialId: 'mat-9', body });


─── apps/miniprogram/tests/export-deletion.test.mjs:384-384 ───
[maintainability · medium] M1 partial 分支内联计算 deletionUnknown（intentPresent && status !==
'partial'），绕过了同文件 F1 用例以及页面 export-deletion.tsx:68 实际使用的
deletionOutcomeUnknown()——与文件注释「页面与测试同源同函数」相悖，两处判定易漂移。Object.assign 已把 gating 模块全部导出复制进
lifecycleGatingModule，可直接复用。另：pageGatingFromServiceState 引用的 lifecycleGatingModule 在其后才声明，仅靠调用时序避开
TDZ，建议把模块持有对象移到使用它的箭头函数之前。

-   const partial = pageGatingFromServiceState({ deletionUnknown: career.pendingSpaceDeletion() !== null && receipt.status !== 'partial' });
+   const partial = pageGatingFromServiceState({ deletionUnknown: lifecycleGatingModule.deletionOutcomeUnknown({ intentPresent: career.pendingSpaceDeletion() !== null, inMemoryStatus: receipt.status, recoveryUnresolved: false }) });


─── apps/miniprogram/tests/live/t24r1-live-driver.cjs:143-144 ───
[maintainability · low] writeIntent/readIntent 依赖手工拼接的存储键 wk:career:search:${JSON.stringify([origin,
userId, tenantId])}，逐字节复刻 src/core/scope.ts scopeKey() + career-intent.ts intentKeyFor
的产物（当前核对一致）。任一端调整元素顺序、序列化方式或前缀，注入的恢复 intent 将不被页面读取（C 步 waitText 超时导致整条取证链失败）。建议将键构造提取为可被 .cjs
复用的共享片段（如 tests/helpers 下镜像实现并在 CI 中与 src/core/scope.ts 比对），或注入后先经生产读取路径回读校验再继续取证。



─── apps/miniprogram/tests/application-material.test.mjs:60-63 ───
[maintainability · medium] backend()（method+pathname
假后端路由器，含尾部斜杠前缀匹配）、freshLogin()、careerCall()、errorCode() 在本批 5 份新测试（application-material /
career-discovery / export-deletion / progress-preparation / rules-usage-reminders，assembly.test.mjs
另有一份 backend）中逐字重复，每份约 25-35 行。后续 career 路由或登录装配变化需同步修改 5-6 处，遗漏即产生测试间口径不一致。建议提取到
tests/helpers/career-harness.mjs 统一导出（freshLogin 可参数化 me() 与 extraRoutes）。

- function backend(routes) {
-   stub.use(call => {
-     const method = call.options.method ?? (call.kind === 'uploadFile' ? 'POST' : 'GET');
-     const path = new URL(call.options.url).pathname;
+ // tests/helpers/career-harness.mjs
+ // export function backend(routes) { /* 现 backend() 实现 */ }
+ // export async function freshLogin({ me = defaultMe, extraRoutes = {} } = {}) { /* 现 freshLogin 实现 */ }
+ // export const careerCall = suffix => stub.state.calls.filter(/* ... */);
+ // export const errorCode = error => error?.code;
+ // 各测试文件：import { backend, freshLogin, careerCall, errorCode } from './helpers/career-harness.mjs';


─── apps/miniprogram/tests/career-discovery.test.mjs:61-61 ───
[maintainability · low] 谓词 ambiguous 全文无调用点（第 129 行只是测试标题字符串），属死代码；且它复刻了
src/services/career-intent.ts ambiguousOutcome 的口径，留着易让读者误以为歧义分类已被独立断言覆盖。建议删除；若想建立口径同步守护，可改为直接断言生产
ambiguousOutcome 的行为。



─── apps/miniprogram/tests/application-material.test.mjs:37-37 ───
[maintainability · low] fixture 工厂 materialView 声明后未被任何用例引用（本文件没有覆盖 GET /materials/:id
完整视图解码的用例），属声明未读取的死变量。建议删除该行，或补充一个消费它的用例（如经 GET /api/v1/career/materials/mat-1 断言
versionCount/versions/status 的解码）。



─── apps/miniprogram/tests/career-discovery.test.mjs:10-10 ───
[bug · low] URL→pathname→URL 双重转换在路径含空格或非 ASCII 时会解析到不存在的文件（POSIX 下 URL.pathname 已百分号编码，再经
pathToFileURL 会把 % 二次编码成 %25，如 my%20repo → my%2520repo），导致模块替换在 import 阶段整体失败；Windows 下 pathname 形如
/C:/... 同样脆弱。该写法沿用 assembly.test.mjs 既有惯例，但新文件修正成本为零。本批 application-material / career-platform /
export-deletion / progress-preparation / rules-usage-reminders 共 6 份新测试均为同一写法，建议统一改为直接取 href。

- const stubURL = pathToFileURL(new URL('./helpers/taro-stub.mjs', import.meta.url).pathname).href;
+ const stubURL = new URL('./helpers/taro-stub.mjs', import.meta.url).href;
+ // 同时可移除不再使用的 pathToFileURL 导入


─── apps/miniprogram/tests/live/t24r1-live-driver.cjs:28-31 ───
[test · low] T24R1_TOKA 缺省时 fs.readFileSync(undefined) 在友好校验之前就抛出难定位的 TypeError（ENOFD: undefined,
open 'undefined'），应把 TA 读取移到校验之后并纳入必填项。另外登录步骤的 li[0].input / box.tap() 未判空，页面结构变化时会以裸 TypeError
失败而非清晰的步骤信息，建议一并加存在性检查。

- const TA = fs.readFileSync(process.env.T24R1_TOKA, 'utf8').trim();
  const USER_A = process.env.T24R1_USER_A;
  const TENANT = String(process.env.T24R1_TENANT);
- if (!SHOTS || !USER_A || !TENANT) { console.error('missing T24R1_* env'); process.exit(2); }
+ if (!SHOTS || !USER_A || !TENANT || !process.env.T24R1_TOKA) { console.error('missing T24R1_* env'); process.exit(2); }
+ const TA = fs.readFileSync(process.env.T24R1_TOKA, 'utf8').trim();


─── apps/miniprogram/tests/live/t24r1-live-driver.cjs:241-243 ───
[test · low] 「全部截图两两字节不同」的全局硬断言可能误伤两个合法渲染相同的终态：C-search-reconciled 与 F2-recovered 的 mustMatch
断言正则相同（no_vetted_sources + 覆盖来源（0）），若该终态页面不可滚动，verifiedShot 的 4
次换偏移重拍均无效果，会把合法状态误报为「不可接受的取证」失败。建议把互异约束限定在需要证明互异的截图子集（如各状态增量），或在 manifest 中记录 mustMatch
断言文本并以「断言文本互异」替代全局字节互异。



─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:96-97 ───
[test · medium] 「进入工作空间」是后续所有 career 数据断言的前置步骤，但其 tap 失败（entered === false）只走 log、既不 record FAIL 也不
throw，与脚本自身的失败处理标准不一致（consent 步骤注释明确「tap 超时即 throw，不允许把根因埋进后续步骤」，且 finally 中按 results 判定退出码）。若该 tap
失败而页面文案恰好仍含「求职工作台」，record('login') 可能误判 PASS，后续 career-page/apply-material-data 的 FAIL
会掩盖真实根因（工作空间未进入），证据评审者从 RESULT JSON 无法定位。建议与 consent 同样处理：record 后失败即 throw。

        const entered = await tapText(page, '进入工作空间', 12000);
-       log('enter workspace', entered);
+       record('enter-workspace', entered, entered ? 'entered workspace' : 'enter-workspace button not reachable');
+       if (!entered) throw new Error('enter-workspace button not reachable');


─── apps/miniprogram/src/subpackages/execution/artifact/index.tsx:11-11 ───
[maintainability · medium] 死代码：本页由引用 features/execution/pages.tsx 的 ArtifactPage 改为内联实现后，旧
ArtifactPage（基于会话级产物接口 client.chat.artifacts.session，与新页的 task-scoped 实现完全不同）已无任何调用方——src
下仅剩本处定义，tests 亦无引用，detail/approval/tasks 页各自引用的是该文件的其他导出。两套产物页并存会误导后续维护者，建议删除
features/execution/pages.tsx 中的旧 ArtifactPage 及其独占的导入。



─── apps/miniprogram/src/subpackages/execution/artifact/index.tsx:9-9 ───
[bug · low] supported() 与 openProtectedDocument 的扩展名白名单不一致，会产生两个矛盾场景：(1) 服务端返回
.doc/.xls/.xlsx/.ppt/.pptx 产物时按钮显示"此格式暂不支持"并禁用，但底层 openProtectedDocument 实际支持打开（白名单含这七种）；(2) mime
合法但文件名无白名单扩展名（如 report、report.bin）时按钮启用，点击后却必然抛"此格式请使用授权 Web 工作台查看"。另外 mime 第二分支仅锚定结尾，任意以
officedocument.wordprocessingml.document 结尾的字符串都会通过。建议将扩展名白名单提取为共享常量（与 files.ts 同源），supported
与打开校验用同一份判定。

- const supported=(name:string,mime:string)=>/^application\/pdf$|officedocument\.wordprocessingml\.document$/i.test(mime)||/\.(pdf|docx)$/i.test(name);
+ const OPENABLE_EXTENSIONS=['pdf','doc','docx','xls','xlsx','ppt','pptx'];
+ const supported=(name:string,mime:string)=>OPENABLE_EXTENSIONS.includes(name.split('.').pop()?.toLowerCase()??'')&&(/^application\/pdf$/i.test(mime)||/^application\/vnd\.openxmlformats-officedocument\.wordprocessingml\.document$/i.test(mime)||mime==='application/octet-stream');


─── apps/miniprogram/src/services/workbench.ts:27-27 ───
[bug · low] parsed.origin 与 apiOrigin 的全等比较存在默认端口规范化差异：new URL('https://host:443/x').origin 会去掉默认端口
443，而 normalizeApiOrigin（auth.ts）只做小写化和去尾斜杠、保留 ':443'。若 WEKNORA_API_ORIGIN 配置为带默认端口的
https://host:443（config/index.ts 的 /^https:\/\/[^/?#]+$/ 校验允许该形态），所有合法签名 URL 都会被判为
Untrusted，产物打开整体不可用。建议先用 new URL(apiOrigin).origin 做同样的规范化再比较。

-  if(!apiOrigin||parsed.origin!==apiOrigin||parsed.pathname!=='/api/v1/workbench/artifacts/download'||!parsed.searchParams.has('signature'))throw new Error('Untrusted signed artifact URL');
+  const expectedOrigin=apiOrigin?new URL(apiOrigin).origin:'';
+  if(!apiOrigin||parsed.origin!==expectedOrigin||parsed.pathname!=='/api/v1/workbench/artifacts/download'||!parsed.searchParams.has('signature'))throw new Error('Untrusted signed artifact URL');


─── apps/miniprogram/src/platform/files.ts:44-44 ───
[maintainability · low] 清理失败的错误消息把内部存储路径（wxfile://usr/wk-open-…-随机后缀.pdf）原样拼进用户可见文案——页面会通过
action.error 直接展示该错误，且此时文档已成功打开、清理失败并不影响用户已获得的预览，"报错"与体验矛盾。建议用户文案不携带路径，路径放入 cause/detail 供排查。

-  return new Promise((resolve,reject)=>fs.unlink({filePath,success:()=>resolve(),fail:e=>reject(Object.assign(new Error(`临时副本清理失败：${filePath}`),{cause:e}))}));
+  return new Promise((resolve,reject)=>fs.unlink({filePath,success:()=>resolve(),fail:e=>reject(Object.assign(new Error('临时文件清理失败，可忽略；如反复出现请重新打开'),{cause:e,detail:filePath}))}));


─── apps/miniprogram/src/platform/files.ts:36-36 ───
[bug · low] stem 未做长度截断：`wk-open-` 前缀 + 未截断的原始文件名主体 + 时间戳/随机后缀 +
扩展名，超长文件名（服务端产物名来自上传文件名，无长度约束）可能超出微信文件路径上限，导致 copyFile 失败、整个打开流程报"本机文件准备失败"。建议对 stem 截断（如 64 字符）。

- const stem=(dot>0?name.slice(0,dot):name).replace(/[^A-Za-z0-9._-]+/g,'-').replace(/^[-.]+/,'')||'artifact';
+  const stem=((dot>0?name.slice(0,dot):name).replace(/[^A-Za-z0-9._-]+/g,'-').replace(/^[\-.]+/,'')||'artifact').slice(0,64);


─── apps/miniprogram/src/platform/files.ts:83-83 ───
[other · low] openDocument 成功回调后立即删除副本：注释与测试固化的是 DevTools
实测契约，但真机（iOS/Android）上文档预览由独立进程异步读取文件，resolve 即 unlink 存在预览加载失败/白屏的竞态风险（此类行为 DevTools
与真机经常不一致）。建议在真机（至少 iOS 一台）回归验证"打开→立即清理"链路；若真机出现白屏，可改为下次进入页面时清扫 wk-open- 前缀残留副本兜底。



─── apps/web/src/career/OpportunityPage.tsx:212-212 ───
[bug · medium] 回执不匹配时抛出的是 TypeError：TypeError 没有 code/status，catch 中 isUncertainWrite(cause) 会返回
true，落入 'unknown' 恢复分支并提示"请先查询原请求回执，再决定是否使用同一编号重试"——而用原编号重试/查询得到的结果永远 mismatch，用户被锁进无法退出的恢复循环。这与
protocol.ts 中 ReceiptMismatchError 的注释红线（"确定性协议错误不得路由进回执恢复"，ocr2-067）以及本文件
importAttempt/lookupReceipt 的既有处理（catch 中先 instanceof ReceiptMismatchError 再分流）相悖。同一问题还出现在本文件
lookupEvaluationReceipt 与 EvaluationAction.accept（同样抛 TypeError）；RulePage/SearchPage 的 queryReceipt
在 requestId 不匹配时也直接 setPhase('unknown')。建议统一改用 protocol.ts 导出的 ReceiptMismatchError，并在各 catch 顶部先于
isUncertainWrite 做 instanceof 判断、置 'error' 终态。

-    if (next.requestId !== requestId || next.opportunityId !== fixedReceipt.opportunityId || next.snapshotId !== fixedReceipt.snapshotId) throw new TypeError('评估回执与固定职位快照不匹配')
+    if (next.requestId !== requestId || next.opportunityId !== fixedReceipt.opportunityId || next.snapshotId !== fixedReceipt.snapshotId) throw new ReceiptMismatchError('评估回执与固定职位快照不匹配')
+ // catch 中：
+ // if (cause instanceof ReceiptMismatchError) { setEvaluationState('error'); setEvaluationMessage(`评估未完成：${cause.message}，已放弃本次结果。请重新评估。`); return }


─── apps/web/src/career/ProgressPage.tsx:24-27 ───
[maintainability · medium] errorDetails / isUncertainWrite / newRequestId / ReceiptMismatchError
在本文件整段复制自共享模块 protocol.ts（该模块已存在且被 MaterialPage、OpportunityPage 部分引用）。同样的复制还出现在
ApplicationPage、ExportDeletionPage、InboxPage、PreparationPage、MaterialPage、RulePage、SearchPage 共约 9
处；且本文件还额外声明了一个与 protocol.ts 同名的本地 ReceiptMismatchError 类，导致 protocol.isUncertainWrite 中的 instanceof
防线对本文件抛出的实例不生效（目前靠 catch 里的本地 instanceof 先行判断才未出错）。后端错误码清单一旦调整需要同步 8+ 处，极易漏改造成页面间"确定/未知"分类漂移。建议全部从
'./protocol.ts' 导入基础实现，页面特定错误码（如
search_quota_refused、material_claim_unconfirmed）通过在共享函数外包装叠加，而不是整段复制。

- function errorDetails(cause: unknown): { code?: string; requestId?: string; currentRevision?: number; status?: number; message: string } {
-  const error = cause as { code?: string; requestId?: string; currentRevision?: number; status?: number; message?: string }
-  return { code: error?.code, requestId: error?.requestId, currentRevision: error?.currentRevision, status: error?.status, message: error?.message || '请求未完成' }
- }
+ import { ReceiptMismatchError, errorDetails, isUncertainWrite, newRequestId } from './protocol.ts'


─── apps/web/src/career/InboxPage.tsx:127-131 ───
[performance · low] 读取待办列表后，对每个携带 applicationId 的条目在 for 循环内串行 await application()：N
个不同申请的待办会让收件箱首屏延迟线性叠加（每次一个 RTT）。各申请回执彼此独立，应并行获取后再统一做一次 isCurrent 校验。

-     for (const item of next.reminders) {
-      const applicationId = item.applicationId
-      if (!applicationId || applicationId in refs) continue
+     const targets = [...new Set(next.reminders.map((item) => item.applicationId).filter((id): id is string => Boolean(id)))].filter((id) => !(id in refs))
+     await Promise.all(targets.map(async (applicationId) => {
       try {
        const receipt = await client.career.application(applicationId, requestScope.signal)
+       if (!scopeController.isCurrent(requestScope.scope)) return
+       refs[applicationId] = { snapshotId: receipt.pinnedEvidence.snapshotId, opportunityId: receipt.pinnedEvidence.opportunityId }
+      } catch {
+       if (!scopeController.isCurrent(requestScope.scope)) return
+       refs[applicationId] = 'failed'
+      }
+     }))
+     if (!active || !scopeController.isCurrent(requestScope.scope)) return


─── apps/web/src/career/ExportDeletionPage.tsx:192-196 ───
[bug · low] downloadExport 创建的 anchor 未挂载到 document 就调用 click()，且点击后立即 revokeObjectURL——与同批
MaterialPage.saveBlob 的安全实现（append 到 body + 5 秒延迟 revoke）不一致。部分浏览器/时序下 blob URL
在下载管理器取数据前已被回收，导出包下载会落空，而提示仍宣称"已下载"。这是 T22 删除前留存导出包的关键路径，建议抽取统一的下载工具函数（以 MaterialPage 版本为准）供两处复用。

    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url; anchor.download = `career-export-${exported.exportId}.json`
+   document.body.append(anchor)
    anchor.click()
-   URL.revokeObjectURL(url)
+   anchor.remove()
+   setTimeout(() => URL.revokeObjectURL(url), 5_000)


─── apps/web/src/career/ExportDeletionPage.tsx:310-310 ───
[bug · medium] exportMessage 的渲染条件包含 exportPhase !== 'idle'，但 acceptExport 成功后 phase 被置回
'idle'，而"下载导出包"按钮只在 exported 存在（phase 为 idle）时可点。因此 downloadExport 里 setExportMessage
设置的"导出包已下载（…）。摘要 …"与"当前环境不支持文件下载…"等反馈永远不会渲染——用户点击下载后没有任何 UI 反馈，下载静默失败时也无从得知。建议参照 MaterialPage 的
downloads 状态为下载反馈建立独立通道，或将过滤条件改为 exportPhase !== 'busy' 之类的下载无关条件。



─── apps/web/src/career/MaterialPage.tsx:45-45 ───
[bug · low] sha256Hex 直接访问 crypto.subtle.digest：非安全上下文（HTTP 部署、部分内网环境）下 crypto.subtle 为
undefined，此处抛出 TypeError 后被 downloadExport 的 catch
兜底成"下载未完成：请求未完成"，用户无法定位真实原因，且已通过授权的合法下载被一刀切阻断。建议前置检测 crypto.subtle
是否可用：可用才走摘要校验，不可用时给出明确降级提示（如"当前环境不支持摘要校验，已跳过"）而不是笼统失败。

+  if (!crypto?.subtle) throw new Error('当前环境非安全上下文（HTTPS），无法执行 SHA-256 摘要校验')
   const sum = await crypto.subtle.digest('SHA-256', bytes)


─── apps/web/src/career/MaterialPage.tsx:423-427 ───
[bug · low] publishExport 与 revokeExport 的 revision_conflict 分支调用的是 setMessage（主编辑区消息）而非
setExportMessage：此时导出区 exportPhase 为 'error' 但 exportMessage 为空，错误文案却出现在编辑器区域的消息位（role=status 而非
alert），语义错位且"重新读取档案修订"按钮与文案分离。revokeExport 内同样的分支请一并修正。

     if (parsed.code === 'revision_conflict') {
      setExportAttempt(undefined); setExportPhase('error'); setExportConflict(parsed.currentRevision)
-     setMessage(`档案已更新${parsed.currentRevision !== undefined ? `（当前修订 ${parsed.currentRevision}）` : ''}。请重新读取档案修订后再次发布导出；新提交会使用新的请求编号。`)
+     setExportMessage(`档案已更新${parsed.currentRevision !== undefined ? `（当前修订 ${parsed.currentRevision}）` : ''}。请重新读取档案修订后再次发布导出；新提交会使用新的请求编号。`)
      return
     }


─── apps/web/src/career/MaterialPage.tsx:582-585 ───
[style · low] statusText
是三层嵌套三元表达式，违反项目规范（禁止嵌套三元）。同类模式还散落在：ApplicationPage.evaluationStatusLabel、RulePage"下次运行计划"三分支、Prepara
tionPage.reviseMessage 的 className 前缀判断等。建议改为按 kind/status 查映射表或抽取独立函数，便于扩展新的导出状态。

-      const statusText = receipt.kind === 'material_export_revoked' ? '已撤销（revoked）：旧下载授权立即失效'
-       : receipt.status === 'submittable' ? (deliverable ? '双格式核验通过（submittable）' : '双格式核验记录不一致，暂不提供投递')
-       : receipt.status === 'staged' ? '已暂存（staged）：仅一种格式核验通过，不可投递'
-       : '双格式核验失败（failed），不可投递'
+      const statusText = exportStatusText(receipt, deliverable)
+ // 独立函数：
+ function exportStatusText(receipt: MaterialExportReceipt, deliverable: boolean): string {
+  if (receipt.kind === 'material_export_revoked') return '已撤销（revoked）：旧下载授权立即失效'
+  if (receipt.status === 'submittable') return deliverable ? '双格式核验通过（submittable）' : '双格式核验记录不一致，暂不提供投递'
+  if (receipt.status === 'staged') return '已暂存（staged）：仅一种格式核验通过，不可投递'
+  return '双格式核验失败（failed），不可投递'
+ }


─── apps/web/src/career/CareerPage.tsx:194-194 ───
[bug · low] "查询来源状态"按钮在 refreshSources()
成功后无条件提示"来源列表不包含请求编号，无法确认本次上传结果"——若上传实际已成功，新来源已可见地出现在下方列表中，文案与界面事实自相矛盾。refreshSources 本身返回 items
却未参与判断；且来源列表 UI 不展示 requestId，用户本就无法自行核对请求编号归属。建议利用返回的 items 与
uploadUnknown（revision/requestId）做区分提示（如出现了同修订的新来源则提示"可能已成功，请核对"），或在来源条目中呈现请求编号。

-      try { await refreshSources(); if (!isCurrent(epoch)) return; setUploadNotice('来源列表不包含请求编号，无法确认本次上传结果。请用保留的原文件、请求编号和修订精确重试。') }
+      try {
+       const items = await refreshSources()
+       if (!isCurrent(epoch)) return
+       const likelySucceeded = items.some((source) => source.fileName === uploadUnknown.file.name && source.revision === uploadUnknown.expectedRevision)
+       setUploadNotice(likelySucceeded
+        ? '来源列表出现了同名的全新来源，本次上传可能已成功；请核对后决定是否重试。'
+        : '来源列表未出现与本次上传对应的新来源，无法确认结果。请用保留的原文件、请求编号和修订精确重试。')
+      }


─── apps/web/src/career/CareerPage.tsx:176-176 ───
[style · low] 本页大量使用静态内联 style 对象（maxWidth、grid 布局、间距、颜色等固定值），违反"内联样式仅限动态样式"的约定，且每次渲染都重建这些对象。同目录的其余
career 页面均已配套独立 css 文件（application.css、material.css、inbox.css 等），本页是唯一例外，风格不一致。建议把这批静态样式迁移到 career
相关 CSS 文件（可复用已新增的 usage.css / material.css 或新建 profile.css）。



─── apps/web/src/career/inbox.css:46-47 ───
[bug · medium] 主操作按钮的品牌绿样式被自身基础规则覆盖：`.wk-inbox button`（特异性 0,1,1）声明了 background/color/border，而
`.wk-inbox__submit`（0,1,0）对其 border-color/background/color 的声明全部失效；hover 侧 `.wk-inbox
button:hover:not(:disabled)`（0,3,1）同样压制
`.wk-inbox__submit:hover:not(:disabled)`（0,3,0）。「登记待办」主按钮实际渲染为白底深字（InboxPage.tsx L267 确认按钮在
.wk-inbox 容器内且挂有该类）。同一问题在 progress.css / submission.css 复制存在，而 application.css / material.css 用
`:not(:disabled)` 提升特异性、preparation.css / export-deletion.css 用 `!important`
硬压，同一批文件存在三种互不一致的解法。建议统一为元素限定写法（或择一收敛现有三种方案）：

- .wk-inbox__submit { border-color: #07c05f; background: #07c05f; color: #fff; }
- .wk-inbox__submit:hover:not(:disabled) { color: #fff; opacity: .9; }
+ button.wk-inbox__submit { border-color: #07c05f; background: #07c05f; color: #fff; }
+ button.wk-inbox__submit:hover:not(:disabled) { color: #fff; opacity: .9; }


─── apps/web/src/career/progress.css:36-37 ───
[bug · medium] 同 inbox.css 的特异性问题：`.wk-progress button`（0,1,1）覆盖 `.wk-progress__submit`（0,1,0）的绿色
background/color/border-color，hover 规则（0,3,1 vs 0,3,0）同样被压制。「记录进展」主按钮（ProgressPage.tsx
L198）渲染为白底深字而非品牌绿。建议与同批文件统一修复方式：

- .wk-progress__submit { border-color: #07c05f; background: #07c05f; color: #fff; }
- .wk-progress__submit:hover:not(:disabled) { color: #fff; opacity: .9; }
+ button.wk-progress__submit { border-color: #07c05f; background: #07c05f; color: #fff; }
+ button.wk-progress__submit:hover:not(:disabled) { color: #fff; opacity: .9; }


─── apps/web/src/career/submission.css:92-94 ───
[bug · medium] 同款特异性问题，且本文件受影响的不止主按钮：`.wk-submission button`（0,1,1）不仅使
`.wk-submission__submit`（0,1,0）的绿色样式全部失效，「确认投递」主按钮（SubmissionPage.tsx L282）呈白底深字；还覆盖了
`.wk-submission__review`（L94，0,1,0）的 padding: 2px 10px 与 font-size: .85rem——回看按钮（SubmissionPage.tsx
L56）以 8px 14px 的全尺寸按钮挤在版本文本段落内。建议统一用元素限定写法修复两处：

- .wk-submission__submit { border-color: #07c05f; background: #07c05f; color: #fff; }
- .wk-submission__submit:hover:not(:disabled) { color: #fff; opacity: .9; }
- .wk-submission__review { margin-left: 8px; padding: 2px 10px; font-size: .85rem; }
+ button.wk-submission__submit { border-color: #07c05f; background: #07c05f; color: #fff; }
+ button.wk-submission__submit:hover:not(:disabled) { color: #fff; opacity: .9; }
+ .wk-submission button.wk-submission__review { margin-left: 8px; padding: 2px 10px; font-size: .85rem; }


─── apps/web/src/career/submission.css:55-57 ───
[maintainability · medium] `--td-text-color-warning`
在项目令牌表（packages/design-tokens/src/tdesign-theme.css）中未定义——文本色令牌只有
primary/secondary/placeholder/disabled/anti/brand/link，警示色令牌名为
`--td-warning-color`。本文件两处引用（`__record-version--unknown` 与 `__claim--needs-review`）将永远落入回退值
#e37318，主题迭代警示色时这两处不跟随。建议改用已定义的令牌：

  .wk-submission__record-version--unknown {
-   color: var(--td-text-color-warning, #e37318);
+   color: var(--td-warning-color, #e37318);
  }


─── apps/web/src/career/preparation.css:102-108 ───
[maintainability · medium] `--td-bg-color-secondary-container`（带连字符）在项目令牌表中不存在——实际定义的名称是无连字符的
`--td-bg-color-secondarycontainer`（application.css/material.css 等同批文件均用无连字符形式）。该声明永远走回退值
#f5f5f5，不跟随主题：

  .wk-preparation__sources {
    margin: 10px 0 0;
    padding: 10px 12px;
    border: 1px dashed var(--td-component-border, #dcdcdc);
    border-radius: 8px;
-   background: var(--td-bg-color-secondary-container, #f5f5f5);
+   background: var(--td-bg-color-secondarycontainer, #f5f5f5);
  }


─── apps/web/src/career/export-deletion.css:34-41 ───
[maintainability · low] 本文件（以及 inbox.css / preparation.css / progress.css / submission.css）完全没有定义
:focus-visible 键盘焦点样式，已确认全局 styles.css 也不含任何 focus/outline 兜底规则——键盘焦点将退回浏览器默认描边，与同批
application/material/opportunity/rule/search 五个表面统一书写的品牌焦点环（outline: 3px solid
var(--td-brand-color-focus, …)）不一致。本表面包含「整空间删除」高危确认流程，建议补齐与其他表面一致的品牌焦点指示：

  .wk-lifecycle__actions button {
    padding: 6px 14px;
    border: 1px solid var(--td-component-border, #dcdcdc);
    border-radius: 6px;
    background: var(--td-bg-color-container, #fff);
    color: var(--td-text-color-primary, #222);
    cursor: pointer;
  }
+ .wk-lifecycle button:focus-visible,
+ .wk-lifecycle input:focus-visible,
+ .wk-lifecycle a:focus-visible { outline: 3px solid var(--td-brand-color-focus, #07c05f); outline-offset: 2px; }


─── apps/web/src/career/application.css:74-78 ───
[maintainability · low] 令牌收敛问题（10 个文件共性，此处为代表）：品牌绿 #07c05f 在本批文件中硬编码 30+ 处，警示色
#ffb648/#fff7e8/#4a3200/#b45309 也全部裸写。而项目主题（tdesign-theme.css，经 styles.css 第 7 行引入）中
--td-brand-color 亮色取值就是 #07c05f，硬编码完全可以直接引用令牌。此外同一令牌的回退值各文件漂移：--td-bg-color-secondarycontainer 的回退出现
#f3f3f3/#f7f7f7/#eee/#fafafa/#f4f6fa 五种；--td-brand-color 的回退既有 #07c05f（progress.css）也有
#0052d9（application/material/opportunity 本文件等，语义与 token 实际值相反，token 未加载时会闪蓝）。建议改引用
var(--td-brand-color) 或收敛为一个自定义属性：

  .wk-application__submit:not(:disabled) {
-   border-color: #07c05f;
-   background: #07c05f;
+   border-color: var(--td-brand-color, #07c05f);
+   background: var(--td-brand-color, #07c05f);
    color: #fff;
  }


─── apps/web/src/career/search.css:29-31 ───
[style · low] 焦点描边做法与同批表面不一致：search.css 与 rule.css 手写 rgba(7, 192, 95, .45) 半透明描边，其余 career 表面统一使用
var(--td-brand-color-focus)（主题已定义为 color-mix(in srgb, var(--td-brand-color) 20%,
transparent)），两套描边透明度不同、换主题时本文件不跟随令牌。另外 `.wk-career-search__row-note` 的回退色 #999 若真生效（主题未加载时）在白底上对比度约
2.8:1，不满足 WCAG AA 4.5:1，回退值建议至少 #767676：

  .wk-career-search textarea:focus-visible,
  .wk-career-search button:focus-visible,
- .wk-career-search a:focus-visible { outline: 3px solid rgba(7, 192, 95, .45); outline-offset: 2px; }
+ .wk-career-search a:focus-visible { outline: 3px solid var(--td-brand-color-focus, #07c05f); outline-offset: 2px; }


─── apps/web/src/career/material.css:93-99 ───
[style · low] `.wk-material__compare` 与紧邻其上的 `.wk-material__risks, .wk-material__versions,
.wk-material__version-readonly` 声明块逐字相同，可直接并入同一选择器组。同批还有两处小遗留：preparation.css 的
`.wk-preparation__list` 与 submission.css 的 `.wk-submission__records` 均设 list-style: none 却保留
padding: 0 0 0 18px，缩进无意义：

+ .wk-material__risks,
+ .wk-material__versions,
+ .wk-material__version-readonly,
  .wk-material__compare {
    margin-top: 16px;
    padding: 14px;
    border: 1px solid var(--td-component-border, #e7e7e7);
    border-radius: 10px;
    background: var(--td-bg-color-secondarycontainer, #f7f7f7);
  }


─── apps/miniprogram/src/platform/files.ts:35-35 ───
[bug · low] 点开头的文件名（如 ".pdf"）会丢失扩展名：入口校验 `name.split('.').pop()` 对 ".pdf" 得到 "pdf" 而放行，但这里 `dot>0` 对
dot===0 判否，ext 落空、stem 变为 "pdf"，最终副本路径 `wk-open-pdf-<随机>` 不带 .pdf 结尾。按本文件自己的实测注释（无 .pdf 结尾的文件
openDocument 报 filetype not supported），该产物将必然打开失败，与入口白名单自相矛盾。建议 `dot>=0`，使 ".pdf" 的 ext="pdf"、stem
回退为 "artifact"。

-  const ext=dot>0?name.slice(dot+1).toLowerCase():'';
+  const ext=dot>=0?name.slice(dot+1).toLowerCase():'';


─── apps/miniprogram/src/platform/files.ts:66-68 ───
[style · low] 嵌套三元表达式：`cond1 ? (cond2 ? A : B) : undefined` 属于三元套三元，不符合团队编码约定（禁止嵌套三元）。建议展开为 if
语句，可读性更好且便于后续为其他状态码扩展 code 映射。

-        const code=r.statusCode===401
-         ?responseCode==='artifact_grant_expired'?'ARTIFACT_GRANT_EXPIRED':'ARTIFACT_GRANT_INVALID'
-         :undefined;
+        let code:string|undefined;
+        if(r.statusCode===401){
+         code=responseCode==='artifact_grant_expired'?'ARTIFACT_GRANT_EXPIRED':'ARTIFACT_GRANT_INVALID';
+        }


─── apps/miniprogram/src/subpackages/execution/artifact/index.tsx:51-51 ───
[bug · low] run 缺失时（routeParam 返回空字符串）该按钮仍会执行 navigate('execution',{id:''})：pageUrl 的 filter 只排除
undefined，空字符串会被序列化成 `?id=`，进入执行详情页后 routeParam('id') 为空串，watchExecution('') 会向
`/api/v1/workbench/executions//snapshot` 这类畸形路径发请求，页面停留在"正在读取服务端快照"。这与顶部 Empty
的兜底（navigate('tasks')）不一致。建议 !run 时隐藏该按钮或同样跳转 tasks。

-   <Action secondary onClick={()=>void navigate('execution',{id:run})}>返回任务</Action>
+   {run?<Action secondary onClick={()=>void navigate('execution',{id:run})}>返回任务</Action>:<Action secondary onClick={()=>void navigate('tasks')}>返回任务</Action>}


─── apps/miniprogram/tests/artifact-cleanup.test.mjs:22-22 ───
[test · low] 死代码：`ORIGIN` 声明后在整个测试文件中从未被引用（各处直接使用字面量或 TEMP_PDF/DOWNLOAD 常量）。建议删除，避免与
`globalThis.__API_ORIGIN__` 的真实来源产生混淆。

- const ORIGIN = 'https://api.example.test';
+ // 删除该未使用变量；如需集中管理可复用 globalThis.__API_ORIGIN__


─── apps/miniprogram/tests/artifact-cleanup.test.mjs:26-27 ───
[maintainability · low] 该用例完整复刻了 freshLogin() 的 handler 登录路由逻辑（login/me 分支约 8 行），仅 downloadFile
分支不同，属重复代码。一旦登录契约变化需要同步改两处。建议给 freshLogin 增加可选的 download 分支参数（如 `freshLogin(onDownload)`），401
用例复用同一登录装配。

-   stub.reset();
-   const me = () => ({ success: true, data: { user: { id: 'u1', username: 'Lin' }, tenant: { id: 1, name: 'Space' }, memberships: [] } });
+ // 改造 freshLogin 以支持自定义 download 分支：
+ // function freshLogin(onDownload) {
+ //   ...
+ //   stub.use(call => {
+ //     if (call.kind === 'request') { /* login/me 路由不变 */ return; }
+ //     if (onDownload) { onDownload(call); return; }
+ //     call.options.success({ statusCode: 200, tempFilePath: TEMP_PDF, header: {} });
+ //   });
+ //   ...
+ // }
+ // 401 用例：await freshLogin(call => {
+ //   stub.state.fileContents.set(TEMP_PDF, JSON.stringify({ success: false, code: 'artifact_grant_expired' }));
+ //   call.options.success({ statusCode: 401, tempFilePath: TEMP_PDF, header: {} });
+ // });


─── packages/career-core/src/desk.ts:109-118 ───
[bug · medium] retryUnknown 在 await this.reconcile(...) 期间未防范作用域切换：read() 返回 undefined 的语义是「epoch
已失效，应丢弃」，但这里把 undefined 当作「未找到回执」继续落入 this.send(action)，而 send 会重新 capture() 拿到新作用域的
controller/signal，用同一个 requestId 把旧用户/租户空间的 action
提交到新作用域（服务端幂等按作用域隔离，会当成新请求执行），违反「空间切换后旧响应失效」红线。当前小程序端 runtime.ts 的 SCOPE_CHANGED 守卫与 Web 端 fetch
abort 在多数时序下能兜住（请求被拒→抛 outcomeUnknown），但只要传输层在 abort 后仍 resolve（响应先于切换落地、或自定义 CareerRemote
不检查作用域），跨租户写入就会发生——共享状态机不应依赖传输层兜底。建议在 reconcile 的 await 之后、send 之前重新校验 epoch 与 unresolved
归属，失配时直接终止而非重发。

+   const epochAtEntry = this.epoch
    if (!this.receiptMissing) {
     try {
      const existing = await this.reconcile(action.requestId)
      if (existing) return existing
+     // read() 返回 undefined 仅代表 epoch 失效（作用域已切换），不是「未找到回执」
+     if (!this.current(epochAtEntry)) return undefined
     } catch (error) {
      if (errorCode(error) === 'forbidden') throw error
      if (errorCode(error) !== 'not_found') throw outcomeUnknown(action, error, false)
+    }
-    }
+   }
+   // send 前二次校验：activate()/invalidatePrivateState() 已清空 unresolved 时不得用新作用域重发旧 action
+   if (!this.current(epochAtEntry) || this.unresolved?.requestId !== action.requestId) {
+    throw Object.assign(new Error('Career scope changed during retry'), { code: 'scope_changed' })
    }
    try { return await this.send(action) } catch (error) {


─── packages/career-core/src/desk.ts:78-79 ───
[bug · medium] open/refresh/syncChanges 把 remote 返回的 CareerView/CareerChangeSet 未经任何解码校验直接
applyView/mergeFact/mergeProposal 合入本地状态，contracts.ts 也没有对应的
decodeCareerView/decodeCareerChangeSet（全仓确认不存在）。两个适配层（api-client career.ts:1255-1257 与小程序
services/career.ts 的 remote()）都只是 `as CareerView` 强转，与 receipt
路径的严格解码（decodeCareerReceipt）形成防御不对称。畸形响应的后果是静默污染：例如 revision 缺失/非数字时，applyView 的
Math.max(view.revision, candidate.revision) 与 syncChanges 的 Math.max 会把 NaN 写进
currentView.revision，后续 mutate 的 expectedRevision=NaN、syncChanges 的 since=NaN 全部中毒，直到下次 open
才能恢复。建议在 contracts.ts 补 decodeCareerView/decodeCareerChangeSet，并在 desk 边界（read 的 commit 前）统一校验。

-  async open(): Promise<CareerView | undefined> { return this.read((signal) => this.remote.open(signal), (candidate) => this.applyView(candidate), () => this.currentView!) }
-  async refresh(): Promise<CareerView | undefined> { return this.read((signal) => this.remote.list(signal), (candidate) => this.applyView(candidate), () => this.currentView!) }
+  // 在 contracts.ts 增加 decodeCareerView/decodeCareerChangeSet（校验 revision 为安全非负整数、facts/proposals 逐项复用 validFact/validProposal），并在 desk 读取边界统一校验：
+  async open(): Promise<CareerView | undefined> { return this.read((signal) => this.remote.open(signal), (candidate) => this.applyView(decodeCareerView(candidate)), () => this.currentView!) }
+  async refresh(): Promise<CareerView | undefined> { return this.read((signal) => this.remote.list(signal), (candidate) => this.applyView(decodeCareerView(candidate)), () => this.currentView!) }
+  // syncChanges 的 commit 闭包内对 set 先执行 decodeCareerChangeSet(set) 再合并


─── packages/career-core/src/contracts.ts:148-150 ───
[maintainability · low] decodeEvaluation 在函数体内直接依赖全局 TextEncoder/TextDecoder 做 span 字节重解码。本包按
desk.ts 头部注释的约束需要可被微信小程序直载（strip-types、无 transform），而小程序逻辑层（尤其 iOS JavaScriptCore）普遍不提供这两个全局，且
apps/miniprogram/src/platform/polyfills.ts 也未补齐。当前 decodeEvaluation 仅被 Web 端（OpportunityPage →
api-client.evaluation）调用，模块顶层无全局引用故 import 安全，今天不是 bug；但小程序端已具备
evaluateOpportunity（轻量回执），后续接「查看评估详情」时将在运行时抛
ReferenceError。建议至少做特性检测给出可定位的错误，或将编解码能力作为参数注入，或在调用方平台补充 polyfill。

   const rawText = value.snapshot.rawText
+  if (typeof TextEncoder === 'undefined' || typeof TextDecoder === 'undefined') throw new TypeError('evaluation span verification requires TextEncoder/TextDecoder (missing in this runtime, e.g. WeChat mini-program JSCore)')
   const bytes = new TextEncoder().encode(rawText)
   const validEvidence = (evidence: unknown): evidence is EvaluationJobEvidence => {


─── internal/modules/career/rendering.go:616-622 ───
[bug · medium] bfrange 展开的进位丢失：`dst[1] + byte(offset)` 在低字节相加超过 0xFF 时按 byte 截断且未向 `dst[0]` 进位。PDF
规范中 bfrange 的目标是 dstStart + offset（按 UTF-16 码元算术加法）。渲染器 pdfToUnicodeCMap 生成的正是 identity 映射（dstStart
== low），因此任何跨越 256 边界的连续码元 run（如 CJK/全角文本中相邻的 xxFF 与 (xx+1)00 码元）都会在此处得到错误映射（例如 run <4DFF> <4E01>
<4DFF> 中 code 0x4E00 会被映射为 0x4D00 而非 0x4E00）。后果：verifyMaterialPDF 解码出的文本与 body 不符，对渲染正确的 PDF 也会报告
"claim missing from extracted text"，导出永远停在 staged/failed、无法成为
submittable，且错误信息对用户完全不可诊断。建议改为整数加法后再拆分高低字节。

  			for code := low; ; code++ {
  				offset := int(code - low)
- 				mapping[uint16(code)] = string([]byte{dst[0] + byte(offset>>8), dst[1] + byte(offset)})
+ 				target := (int(dst[0])<<8 | int(dst[1])) + offset
+ 				mapping[uint16(code)] = string([]byte{byte(target >> 8), byte(target)})
  				if code == high {
  					break
  				}
  			}


─── internal/modules/career/profile_intake.go:411-416 ───
[maintainability · low] `if row.ResourceRef == "" { row.ResourceRef = "" }`
是无任何效果的自赋值死代码：它既不改变行状态，也不影响随后的 updates 映射。该分支掩盖了本应表达的意图（例如清除 resource_ref
或纯粹的空占位），建议直接删除，避免误导后续维护者以为此处存在清理逻辑。

  			if row.ResourceRef != "" {
  				row.ErrorCategory = "cleanup_pending_" + row.ErrorCategory
- 			}
- 			if row.ResourceRef == "" {
- 				row.ResourceRef = ""
  			}


─── docs/design/job-search/prototype/app.js:24-29 ───
[bug · medium] syncUrl() 的 history.replaceState 在 origin 为 null 的场景（如直接双击 index.html 以 file://
打开，或被嵌入沙箱 iframe）会抛 SecurityError；而 render() 中 syncUrl() 先于 root.innerHTML
执行且无任何捕获，异常冒泡会导致整个原型渲染中断、页面完全空白。建议对协议做判断或 try/catch 降级，URL 同步失败不应阻断渲染。

  function syncUrl() {
+   if (location.protocol === 'file:') return;
    const url = new URL(location.href);
    url.searchParams.set('variant', state.variant);
    url.searchParams.set('platform', state.platform);
-   history.replaceState(null, '', url);
+   try { history.replaceState(null, '', url); } catch (error) { console.warn('syncUrl skipped:', error); }
  }


─── docs/design/job-search/prototype/index.html:12-12 ───
[bug · low] file:// 协议下 ES module 脚本会被浏览器 CORS 策略拦截（Chrome/Firefox 均如此），直接双击 index.html 将得到空白页——这与
app.js 中 syncUrl() 的 replaceState 抛错是两条叠加的失败路径。本文件并未使用 import/export，若希望支持本地直接打开可改用 classic
script（配合 app.js 的降级修复）；否则建议在头部注释标明必须经 serve.py 等 HTTP 服务访问。

-     <script type="module" src="./app.js"></script>
+     <!-- 需通过 serve.py（http://127.0.0.1:4178）访问；file:// 下 ES module 会被 CORS 拦截 -->
+     <script defer src="./app.js"></script>


─── docs/design/job-search/prototype/app.js:64-65 ───
[maintainability · low] jobRow() 与 evidenceCard() 的 compact 形参会拼入 "compact" class，workbenchView
还专门计算 compact = platform !== 'web'、journeyView 固定传 true，但 app.css 中不存在任何 .compact
选择器（已全目录搜索确认），该分支没有任何视觉效果，属于无效代码，容易误导后续维护者以为存在紧凑态样式分支。建议删除 compact 形参与 class 拼接（evidenceCard(true)
调用同步简化），或在样式表中补齐对应规则。

- function jobRow(item, index, compact = false) {
+ function jobRow(item, index) {
    const selected = state.selectedJob === index;


─── docs/design/job-search/prototype/app.js:91-91 ───
[maintainability · low] journeyView、commandView、profileView、materialsView、applicationsView 五个函数均声明
platform 形参但函数体从未引用（仅 workbenchView 实际使用）。若为保持 renderView 统一分发签名而刻意保留，建议加注释说明意图，否则移除形参以免误导。

- function journeyView(platform) {
+ function journeyView() { // 签名与 renderView 分发保持一致即可，多余的 platform 实参会被忽略


─── docs/design/job-search/prototype/app.js:20-20 ───
[security · low] 已清点全部插值点：所有动态值（含 composer 输入 state.prompt、setNotice 写入的 state.notice、URL 参数）均统一经
esc()，当前无 XSS 实际风险，符合红线要求。两点改进建议：(1) journeyView 中 fit-bar 的内联 style="width:...%" 使用的是 HTML 转义而非 CSS
值上下文校验，fit 目前为写死数字无风险，若后续改为服务端/用户可控值，应先做数值归一再拼入；(2) 逐点手工 esc 的模式较脆弱，新增插值点漏写即引入 XSS，建议将 esc 收敛为统一的模板
helper 降低遗漏概率。

- const esc = value => String(value).replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char]));
+ // 拼入内联 style 的数值先归一，再进入 HTML：
+ // const fit = Math.max(0, Math.min(100, Number(job().fit) || 0));
+ // ... '<div class="fit-bar"><span style="width:' + fit + '%"></span></div>'


─── internal/modules/career/usage.go:337-347 ───
[bug · medium] 配额结算不区分终态成败，与规则触发路径的计费语义互相矛盾。reconcileUsageTx 只要 decodeSearchBody 得到终态回执就无条件
settle，包括 Status=failed、FailureCode=no_vetted_sources 的运行——这种 run 没有执行任何网络 I/O。而 triggerRulePeriod
对同一事实在 SearchOnce 之前预检 len(sources)==0，根本不调 AdmitSearch，且文案明确声明 "no search was consumed" / "the
trigger was not searched"。手动路径（Handler.SearchOnce → Office.SearchOnce）没有这个预检：admission 先预占 1
单位，executeSearch 才发现注册表为空并返回失败回执，随后 reconcile 全额 settle。生产注册表当前为空（NewOffice 注入
emptySearchSourceRegistry），用户每换一个 requestId 调一次 search_once 就净扣 1 单位且必然失败，默认 50 单位/月会被无效失败耗尽并触发
ErrSearchQuotaRefused。建议：对 Status=failed 且 FailureCode=no_vetted_sources 的终态回执执行 release（退回预占）而非
settle，或在 SearchOnce 的 admission 之前复用规则路径的源数量预检，保证"从未触达任何源的 run 不收费"在两条路径一致。

  			receipt, _, decodeErr := decodeSearchBody(search.ReceiptBody)
  			if decodeErr != nil || receipt == nil {
- 				// The search is still claiming or in flight: the reserved
- 				// unit keeps holding until the run turns terminal.
+ 				continue
+ 			}
+ 			if receipt.Status == SearchStatusFailed && receipt.FailureCode == SearchFailureNoVettedSources {
+ 				// 与规则路径一致：从未触达任何源的 run 不收费，退回预占。
+ 				if updateErr := tx.Model(&usageReservationRecord{}).
+ 					Where("id=? AND status=?", row.ID, usageStatusReserved).
+ 					Updates(map[string]any{"status": usageStatusReleased, "lease_until": nil}).Error; updateErr != nil {
+ 					return updateErr
+ 				}
  				continue
  			}
  			if updateErr := tx.Model(&usageReservationRecord{}).
  				Where("id=? AND status=?", row.ID, usageStatusReserved).
  				Updates(map[string]any{"status": usageStatusSettled, "settled_at": now, "lease_until": nil}).Error; updateErr != nil {
  				return updateErr
  			}


─── internal/modules/career/search_rule.go:514-521 ───
[bug · medium] TriggerDueRules 在循环中遇到任一规则返回错误即整体 return nil, err，同批其余到期规则本轮全部饿死；并且失败规则的 run
事务未提交、next_due_at 不变，due 列表按 next_due_at ASC 排序，持续出错的那条规则永远排在最前，会反复独占调度并长期饿死其他规则。triggerRulePeriod
可返回的非配额错误并不罕见：OutcomeUnknownError（commit 竞态且 3 秒内未 reconcile 到）、数据库错误、以及上文的
ErrIdempotencyConflict。作为唯一调度入口的 seam，应把单规则失败隔离到结果里继续处理其余规则，由调用方按规则维度观察/重试，而不是一条规则拖垮整批。

  	outcomes := []RuleRunSummary{}
+ 	var firstErr error
  	for _, rule := range due {
  		outcome, triggerErr := o.triggerRulePeriod(ctx, s, rule, now)
  		if triggerErr != nil {
- 			return nil, triggerErr
+ 			// 隔离单条规则失败，继续处理其余到期规则。
+ 			if firstErr == nil {
+ 				firstErr = triggerErr
+ 			}
+ 			outcomes = append(outcomes, RuleRunSummary{
+ 				RuleID: rule.ID, Period: rule.LastPeriod + 1,
+ 				RequestID: fmt.Sprintf("rule:%s:%d", rule.ID, rule.LastPeriod+1),
+ 				Status: RuleRunStatusFailed,
+ 			})
+ 			continue
  		}
  		outcomes = append(outcomes, outcome)
  	}
+ 	return outcomes, firstErr


─── internal/modules/career/search_rule.go:565-575 ───
[bug · medium] 规则周期触发的崩溃恢复路径会因 fingerprint 携带现场 revision 而永久卡死。period
requestID（rule:ID:period）由服务端生成且稳定，但 SearchOnceInput.ExpectedRevision 取自每次触发时无锁读到的
head.Revision，并被纳入 fingerprint。恢复场景推演：前次触发的 SearchOnce 终态回执已提交，而其后的 run 事务因 busy/进程崩溃未提交（last_period
未推进、next_due_at 停在过去）；用户随后写档案使 revision 从 R 推进到 R'；下次触发用同 requestID、expectedRevision=R'
重放，replaySearch 命中已存终态回执但 Fingerprint 不匹配 → ErrIdempotencyConflict → 走 else
分支整体报错。此后每次触发都重复同一冲突，last_period 永不推进，该规则永久无法恢复，还会连带 TriggerDueRules 整体失败。服务端自生成 requestID 的
fingerprint 冲突不是用户错误，应当可恢复：建议对 ErrIdempotencyConflict 用 FindSearchReceipt(requestID) 取回已持久化回执继续走 run
记录路径（或让规则触发的 fingerprint 不含现场读取的 revision）。

  		if searchErr != nil {
  			if errors.Is(searchErr, ErrSearchQuotaRefused) {
- 				// The gate refused between the pre-check and the search: the
- 				// trigger still must not vanish silently.
  				ran = false
  				blockedStatus, blockedNote = RuleRunStatusBlockedNoQuota, ruleRunNoteNoQuota
  				receipt = SearchOnceReceipt{}
+ 			} else if errors.Is(searchErr, ErrIdempotencyConflict) {
+ 				// 服务端生成的周期 requestID：终态回执可能在 revision 推进前已持久化。
+ 				if prior, priorErr := o.FindSearchReceipt(ctx, requestID); priorErr == nil {
+ 					receipt = prior
+ 				} else {
+ 					return RuleRunSummary{}, searchErr
+ 				}
  			} else {
  				return RuleRunSummary{}, searchErr
  			}
  		}


─── internal/modules/career/search_rule.go:456-461 ───
[bug · medium] 规则视图存在无界存储增长与超大响应向量：SetRule 未限制每 scope 的规则数量；blocked
run（blocked_no_quota、no_vetted_sources）不经过配额预占也每周期持久化一行，最小 1 分钟间隔意味着单条 enabled 规则每天 1440 行 run（月
43200 行），且配额耗尽后 blocked_no_quota 行照常增长；Rule() 又把全部 runs 与 todos 无分页 Find 后整体序列化返回（GetRule 注释自述 "full
run history"）。认证用户创建多条高频规则即可让该端点响应体与 career_search_rule_runs 表无界膨胀。建议：限制每 scope 活跃规则数量，为 Rule() 的
runs/todos 加分页或最近窗口上限，并为 blocked run 设计聚合/截断保留策略。

  	var runRows []searchRuleRunRecord
  	if err = o.db.WithContext(ctx).
  		Where("tenant_id=? AND user_id=? AND rule_id=?", s.TenantID, s.UserID, ruleID).
- 		Order("period ASC").Find(&runRows).Error; err != nil {
+ 		Order("period DESC").Limit(maxRuleRunsInView).Find(&runRows).Error; err != nil {
  		return RuleView{}, err
  	}
+ 	// 返回最近窗口并按 period ASC 展示，避免全量加载无界增长的 run 历史。


─── internal/modules/career/usage.go:395-399 ───
[documentation · low] 文档与行为矛盾：UsageEstimate 的 doc 宣称 "The estimate itself is a free read and never
mutates the ledger"，handler 侧注释也写 "a free read-only projection"，但实现先调用 o.reconcileUsage 执行
settle/release 写事务（GET
路径上的写放大）。若后续维护者据此把该端点接入只读副本或缓存层，会直接失败或产生账本视图分歧。建议修正注释，明确估算前会惰性对账（包含有界写），或将对账从估算读路径中拆出。

  // UsageEstimate answers, before anything executes, what the next charged run
  // of one operation would consume and under which conditions, together with
  // the live balance of the current window. An unreadable ledger is a typed
  // refusal (ErrAdmissionUnavailable); an unknown operation is ErrInvalidRequest.
- // The estimate itself is a free read and never mutates the ledger.
+ // The estimate is free of charge, but not side-effect free: it lazily
+ // reconciles stale reservations (settle/release) with bounded ledger writes
+ // before reading the balance.


─── packages/career-core/src/contracts.ts:146-146 ───
[maintainability · low] decodeEvaluation 的顶层校验与 validEvaluationReceipt 完全重复：同一份 13 键 onlyKeys
列表在两处内联，且 `!isRecord(value)` 在 `!validEvaluationReceipt(value)` 短路之后属于恒假的死代码（validEvaluationReceipt
已保证 isRecord）。后续给 Evaluation 增删字段时需要同步修改两份键列表，漏改任一处会导致 decodeEvaluationReceipt 与 decodeEvaluation
的接受面不一致（一侧拒绝合法载荷或另一侧放行非法载荷）。建议把键列表提取为模块级常量供两处复用，并删除冗余的 isRecord 重查。

+ const evaluationTopLevelKeys = ['kind', 'requestId', 'evaluationId', 'opportunityId', 'snapshotId', 'profileRevision', 'status', 'createdAt', 'rulesetVersion', 'snapshot', 'hard', 'soft', 'facts']
+ function validEvaluationReceipt(value: unknown): value is Evaluation & EvaluationReceipt {
+  return isRecord(value) && onlyKeys(value, evaluationTopLevelKeys) && value.kind === 'evaluation_created' && ...
+ }
  export function decodeEvaluation(value: unknown): Evaluation {
+  if (!validEvaluationReceipt(value) || !validTimestamp(value.createdAt) || ...) throw new TypeError('invalid career evaluation')


─── packages/career-core/src/desk.ts:19-22 ───
[maintainability · low] isAmbiguousOutcome 内联的「确定性错误码」列表与 contracts.ts 的 CareerErrorCode
词表（decodeCareerError 中的 codes 数组）重复定义。若服务端契约后续新增一个确定结果错误码并更新了
contracts.ts，而此处列表漏更新，该错误会落入兜底分支被按「结果未知」处理：desk 会设置 unresolved 并触发 receipt 回查，用户重试又得到同一确定错误，形成
outcome_unknown 循环并阻塞后续 mutate（unresolved_action）。建议由 contracts.ts 导出错误码常量（如
careerDefiniteErrorCodes），desk.ts 引用而非内联字面量，保证两处词汇表同步演进。

- function isAmbiguousOutcome(error: unknown): boolean {
-  const code = errorCode(error)
-  if (code === 'outcome_unknown' || code === 'TIMEOUT' || code === 'NETWORK_ERROR' || code === 'CANCELLED') return true
-  if (code && ['forbidden', 'revision_conflict', 'idempotency_conflict', 'invalid_request', 'not_found', 'proposal_resolved'].includes(code)) return false
+ // contracts.ts
+ export const careerDefiniteErrorCodes = ['forbidden', 'revision_conflict', 'idempotency_conflict', 'invalid_request', 'not_found', 'proposal_resolved'] as const
+ // desk.ts
+ import { careerDefiniteErrorCodes } from './contracts.ts'
+ if (code && (careerDefiniteErrorCodes as readonly string[]).includes(code)) return false


─── apps/mobile/src/task-office-integration-smoke.ts:48-54 ───
[bug · medium] probeServerAuthBoundary 的 redirect 语义与注释矛盾：`redirect: 'error'` 下任何 3xx 响应会使 fetch 直接
reject（TypeError），落入 catch 被记为 'unreachable'。而函数注释明确宣称“任何其他状态（含重定向后 200 的登录页）都是
failed-open”。结果：若部署前置了鉴权代理、对无凭证 API 请求 302 跳登录页（即鉴权边界未在 API 层拒绝），证据会误记为“网络不可达”，把最需要暴露的 failed-open
场景低估为连通性问题。建议改用 `redirect: 'manual'` 并将 3xx（浏览器下为 response.type === 'opaqueredirect'）归为
'failed-open'，仅真正的网络异常归 'unreachable'。

-       redirect: 'error',
+   try {
+     const response = await fetch(new URL(WORKBENCH_EXECUTIONS_PATH, deploymentOrigin).toString(), {
+       method: 'GET',
+       redirect: 'manual',
        headers: { accept: 'application/json' },
      });
+     // 3xx/opaqueredirect：鉴权代理跳转登录页，边界未在 API 层拒绝 → failed-open
+     if (response.type === 'opaqueredirect' || (response.status >= 300 && response.status < 400)) return 'failed-open';
      return response.status === 401 || response.status === 403 ? 'rejected' : 'failed-open';
    } catch {
      return 'unreachable';
    }


─── apps/mobile/src/task-office-integration-smoke.ts:46-46 ───
[style · medium] 硬编码业务 URL 路径 `/api/v1/workbench/executions?limit=1`（本项目检查规则明令禁止硬编码 URL 路径）。该路径必须与
`packages/api-client/src/mobile/executions.ts` 中构造的列表端点保持一致；一旦服务端路由变更或基路径调整，探针会拿到 404 并被误判为
'failed-open'，产生假警报且难以定位。建议提取为模块级常量（并与 api-client 侧共享），同时让 404 与 failed-open 可区分。

-     const response = await fetch(new URL('/api/v1/workbench/executions?limit=1', deploymentOrigin).toString(), {
+ const WORKBENCH_EXECUTIONS_PATH = '/api/v1/workbench/executions?limit=1';
+ // ...
+     const response = await fetch(new URL(WORKBENCH_EXECUTIONS_PATH, deploymentOrigin).toString(), {


─── apps/mobile/src/task-office-integration-smoke.ts:95-95 ───
[test · medium] 归档成功后 restore 失败仅记 `archiveRestore: 'failed'`，与证据契约语义不一致：此处 office.archive
已确定成功、restore 确定失败，用户任务必然滞留归档态、需要人工清理——这正是 'cleanup-required' 被引入的场景（当前它只在“归档失败+scope
失配”路径产生）。证据消费方（progress 追踪）以 cleanup-required 作为触发清理的信号，本路径下无法据此区分“需要清理”与普通失败。

-   if (restoreFailed) return { archiveRoundtrip: 'failed', archiveRestore: 'failed' };
+   if (restoreFailed) return { archiveRoundtrip: 'failed', archiveRestore: 'cleanup-required' };


─── apps/mobile/src/task-office-integration-smoke.ts:223-225 ───
[maintainability · low] 该 catch 中的赋值是无效自赋值：home 为 'loaded' 时三元写回 'loaded'，否则写回 'failed'（初始值本就是
'failed'）——两个分支都不会改变任何值。这是死代码，且容易被误读为某种状态回退逻辑。evidence 各字段已如实记录每步结果，catch 体留空（加注释说明）即可。

    } catch {
-     evidence.home = evidence.home === 'loaded' ? evidence.home : 'failed';
+     // 各步骤证据已如实写入 evidence，无需在此改写 home。
    } finally {


─── internal/handler/session/artifact_download.go:679-683 ───
[maintainability · medium] streamArtifactVersion 与同文件 DownloadArtifactVersion（约 585-629 行）中的“租户解析 →
ParseStorageBackendPath → ResolveTenantFileServiceWithFallback → GetFile →
filetransport.Serve（相同参数）”近乎逐行重复，成为该文件内第三份流式实现（另有
streamResolvedArtifact）。这是安全敏感的下载路径：后续对存储解析、no-store 缓存策略或 Content-Disposition
的任何单边修复都容易漏改另一份，导致两条下载链路行为分叉。建议让 DownloadArtifactVersion 复用
streamArtifactVersion（两处差异仅在错误响应形态：c.Error vs AbortWithStatus，可用参数或返回错误区分）。



─── internal/modules/workbench/service/workbench/application_task.go:78-89 ───
[maintainability · low] EnsureCareerApplicationTask 在第 84 行调用 normalizeApplicationTaskIntent
后，ensureWithRetry 第 106 行又对同一 intent 重复规范化一次。normalize
是纯函数，双次调用无副作用但属冗余，且容易让读者误以为中间有状态变化。ensureWithRetry 作为可注入 ensure
的入口保留防御性规范化即可（测试直接调用它时仍受保护），外层这次可删除；或反之在 ensureWithRetry 内加注释说明为何重复校验。

  func (c *ApplicationTaskCoordinator) EnsureCareerApplicationTask(
  	ctx context.Context,
  	tenantID uint64,
  	ownerID string,
  	intent interfaces.CareerApplicationTaskIntent,
  ) (interfaces.CareerApplicationTaskLink, error) {
- 	intent, err := normalizeApplicationTaskIntent(tenantID, ownerID, intent)
- 	if err != nil {
- 		return interfaces.CareerApplicationTaskLink{}, err
- 	}
+ 	// normalizeApplicationTaskIntent 由 ensureWithRetry 统一执行（含注入 ensure 的测试路径）。
  	return c.ensureWithRetry(ctx, tenantID, ownerID, intent, c.ensureOnce)
  }


─── internal/modules/workbench/service/workbench/application_task.go:137-140 ───
[maintainability · low] Undecided 错误用 %v 内嵌 lastRace，底层 DB 竞态错误（如具体的 duplicate key / database locked
错误）的链路身份丢失，调用方与日志消费者无法用 errors.Is/As 还原竞态证据，而注释明确说要“wrapping the last race evidence”。项目 go 指令为
1.26，fmt.Errorf 支持多个 %w 谓词（或用 errors.Join），可同时保留 ErrApplicationTaskUndecided 哨兵与竞态错误的可判定性。

  	return interfaces.CareerApplicationTaskLink{}, fmt.Errorf(
- 		"%w: creation still racing after %d attempts: %v",
+ 		"%w: creation still racing after %d attempts: %w",
  		ErrApplicationTaskUndecided, applicationTaskMaxAttempts, lastRace,
  	)


─── apps/web/src/settings/ConfigSettingsPanel.tsx:146-148 ───
[bug · high] TSelect 新增 clearable 后 onChange 未做空值守卫：TDesign 清空按钮走 handleClear →
onChange(null)（SandboxSettingsPanel.test.tsx:985 的注释已证实该行为），String(null) 会得到字符串 "null"。后果分两种：①
常规场景（allowedModelIds 非空）——settingsConfigPatch 的模型白名单校验会对 "null" 抛错，500ms 防抖后保存失败并弹报错，清空功能损坏且报错信息困惑；②
tenantModelIds 为空的边缘场景——"null" 被原样持久化为租户 rerank_model_id，污染检索配置。本组其他面板（Cloud/EnvVar/Mcp 等）对 TDesign
受控值清空统一采用 `String(value ?? '')` 守卫，此处为偏差项，建议对齐。

-     clearable
-     options={[{ value: '', label: '—' }, ...modelOptions.map((model) => ({ value: model.id, label: model.name ? model.name + ' (' + model.id + ')' : model.id }))]}
-     onChange={(value) => setValue(key, String(value))}
+     onChange={(value) => setValue(key, String(value ?? ''))}


─── apps/web/src/settings/ChatHistorySettingsPanel.tsx:51-53 ───
[bug · medium] 防抖保存竞态导致丢更新：保存请求在途时（savingRef.current === true），防抖定时器到期触发的 save() 被静默
return，且不会重新调度定时器——用户在保存往返期间的新开关/模型变更被直接丢弃。更糟的是 save 成功后 onSaved?.() 触发壳层 load(true)
全量刷新，initialValue 变化后本组件的同步 useEffect 会用服务端旧值重置
enabled/embeddingModelId/latestRef，用户在往返期间的修改被回滚丢失（UI 上表现为操作被撤销）。建议：① in-flight 期间将待存快照排队，save
完成后补发；② 同步 effect 重置前比对 latestRef 与服务端值，本地有未落库变更时保留本地值。



─── apps/web/src/settings/CloudSettingsPanel.tsx:143-147 ───
[bug · medium] 保存成功后凭证状态固化：仅乐观置位 setNeedsReinit(false)/setHasCredentials(true)，不再拉取权威
weknoraCloud.status（旧实现的 reload 已删除）；且本组件没有随 initialValue 变化的同步 effect（与同批新增的
ChatHistorySettingsPanel 做法不一致）。若服务端保存后实际判定 needs_reinit=true（或壳层后续刷新
initialValue），凭证状态条将停留在过期的「已配置」态并放行云模型接入，误导用户。建议保存成功后重新调用 status() 回灌
needsReinit/reinitReason/hasCredentials，并补一个 initialValue 同步 effect。



─── apps/web/src/settings/CloudSettingsPanel.tsx:134-137 ───
[bug · low] 判空校验回归：旧实现以 appId.trim()/appSecret.trim() 判空，纯空白字符串会被拦截；迁移后 !form.appId
直接判空，空白凭证可通过前端校验提交入库。建议恢复 trim 后判空。

-     if (!form.appId || !form.appSecret) {
+     if (!form.appId.trim() || !form.appSecret.trim()) {
        pushSettingsToast(t('settings.weknoraCloud.fillRequired'), 'warning');
        return;
      }


─── apps/web/src/settings/CloudSettingsPanel.tsx:316-316 ───
[security · low] usageSteps 从旧的按 \n 拆分逐行 span 安全渲染改为 dangerouslySetInnerHTML 注入
innerHTML。当前文案为开发方控制的静态 i18n（zh/en/ja/ko/ru 均有定义）风险较低，但一旦翻译串未来引入插值或外部数据即成 XSS 通道，且与 innerHTML
禁用规范相悖。建议还原为按行渲染。

-         <p className="hint-text" dangerouslySetInnerHTML={{ __html: t('settings.weknoraCloud.usageSteps').replace(/\n/g, '<br />') }} />
+         <p className="hint-text">{t('settings.weknoraCloud.usageSteps').split('\n').map((line, index) => <span key={index} className="hint-line">{line}</span>)}</p>


─── apps/web/src/settings/McpSettingsPanel.tsx:1237-1240 ───
[bug · medium] timeout/retryCount/retryDelay 由 type="number"（含 min/max）迁移为无类型 TInput 后出现 NaN 卡死问题：输入
'12a'、'-' 等中间态时 Number(String(value)) 得 NaN 存入 draft，随后 String(NaN) === 'NaN' 回显，输入框显示脏值 "NaN"
且后续按键持续被 NaN 覆盖，仅靠失焦 normalize 兜底；失焦前误触保存虽由载荷构建归一化兜底，但输入体验已损坏。建议 onChange 内校验：非有限数值时不落
draft（维持原值）或暂存原字符串。另注：className="wk-mcp-number-input" 缩进异常（顶格），建议一并修复格式。

                          <TInput
- className="wk-mcp-number-input"
+                           className="wk-mcp-number-input"
                            value={draft.timeout === "" ? "" : String(draft.timeout)}
-                           onChange={(value) => setField("timeout", String(value) === "" ? "" : Number(String(value)))}
+                           onChange={(value) => {
+                             const next = String(value);
+                             setField("timeout", next === "" ? "" : Number.isFinite(Number(next)) ? Number(next) : draft.timeout);
+                           }}


─── apps/web/src/settings/McpSettingsPanel.tsx:5-5 ───
[maintainability · low] 死导入：TSelect 全文件仅出现在此导入行，无任何使用（本组迁移后表单改用自绘 radiogroup 按钮），会触发
noUnusedLocals/eslint 报错，请移除。

- import { Button as TButton, Checkbox as TCheckbox, Input as TInput, Select as TSelect, Textarea as TTextarea } from "tdesign-react";
+ import { Button as TButton, Checkbox as TCheckbox, Input as TInput, Textarea as TTextarea } from "tdesign-react";


─── apps/web/src/settings/GeneralPreferencesPanel.tsx:351-351 ───
[other · low] 可访问性回退：自动更新开关迁移到 TDesign Switch 时丢失了 aria-label={autoUpdateCopy.label}；同样，语言/主题/字体三处
Select 迁移后也丢失了 aria-label，屏幕阅读器无法获知控件用途。建议补回 aria-label（TDesign 组件支持属性透传）。

-             <Switch value={autoCheckUpdate} onChange={(value) => handleAutoCheckUpdateChange(Boolean(value))} />
+             <Switch value={autoCheckUpdate} aria-label={autoUpdateCopy.label} onChange={(value) => handleAutoCheckUpdateChange(Boolean(value))} />


─── apps/web/src/settings/EnvVarSettingsPanel.tsx:127-127 ───
[other · low] 可访问性回退：旧实现帮助触发按钮带 aria-expanded={helpOpen} 且 onMouseOver/onFocus 双通道展开（键盘聚焦可打开弹层）；迁移为
TDesign Popup trigger="hover" 后 aria-expanded 丢失，且 hover 触发不响应键盘焦点，键盘用户失去等价操作路径。建议通过 onVisibleChange
维护受控可见状态并回填 aria-expanded，必要时改用对 focus 友好的触发方式。



─── apps/web/src/chat/chat-header.tsx:115-119 ───
[bug · high] 重命名失败分支的错误反馈永远不可见：catch 中先 setTitleEditing(false) 关闭编辑表单，再
setRenameError(renameTitleFailed)，而 chat-header__edit-error 只在 titleEditing 为 true 的 form 分支内渲染（且
startTitleEdit 会立即 setRenameError(null)），用户得不到任何失败提示。已核实宿主 ChatRoutePage.renameSession（:1404）直接
await client.sessions.update 且不捕获（无 setError/toast 兜底），此处是唯一的失败通道。另外 chat-header__edit-error 与
chat-header-confirm__error 在所有 CSS 中均无样式定义（chat.td.css 只迁移了相邻的 __edit-input/__confirm__*
类）。建议失败时保持编辑态打开并聚焦，成功后再关闭；同时补上两个错误类的样式。

-     } catch {
+     try {
+       await props.onRenameSession(title);
        setTitleEditing(false);
        setTitleDraft('');
+     } catch {
+       // 保持编辑态打开，让 chat-header__edit-error 可见（需在 chat.td.css 补该类样式）
        setRenameError(props.renameTitleFailed);
+       titleInputRef.current?.focus();
      } finally {


─── apps/web/src/chat/ChatRoutePage.tsx:1935-1936 ───
[bug · medium] 双重确认：ChatHeader 的 ⋯ 菜单已用 clearConfirmBody/deleteConfirmBody 在弹层内做二次确认，而被接入的
clearMessages（ChatRoutePage:1433 的 window.confirm(chatClearConfirmation(...))）和 deleteSession（:1420
的 window.confirm(copy.deleteConfirmBody)）内部又各弹一个原生 confirm，且文案与弹层用的是同一份
copy——用户对破坏性操作需要连续确认两次（TDesign 弹层 + 原生对话框）；若在原生 confirm 点取消，函数静默返回，弹层还会当作成功关闭。已核实这两个函数当前仅被
headerSlot 消费（ChatPage/侧栏均未传入）。建议抽出跳过 window.confirm 的内核函数给 ChatHeader（弹层即确认），或为这两个函数增加 skipConfirm
参数。

-         onClearSession={clearMessages}
-         onDeleteSession={async () => { await deleteSession(selectedSessionId); }}
+         onClearSession={clearSessionCore}
+         onDeleteSession={async () => { await deleteSessionCore(selectedSessionId); }}
+ // 将 clearMessages/deleteSession 中 await client.sessions.* 之后的逻辑抽为无 confirm 的
+ // clearSessionCore/deleteSessionCore；window.confirm 仅保留给未来不经弹层确认的调用方。


─── apps/web/src/chat/ChatRoutePage.tsx:1955-1955 ───
[bug · low] void openTerminal() 丢弃 Promise：openTerminal（:1639-1642）在 terminalController.current
为空时同步 throw，terminal.open 网络失败时 reject，都会成为未处理 Promise 拒绝且用户无感知。同文件 onTogglePin 的 void
是安全的（toggleSessionPin 内部已 catch → setError），此处不一致。建议补 .catch 反馈。

-       <SandboxHeaderToggle copy={copy} label={copy.openSandboxPanel} onOpen={() => void openTerminal()} />
+       <SandboxHeaderToggle copy={copy} label={copy.openSandboxPanel} onOpen={() => {
+         void openTerminal().catch((cause) => setError(cause instanceof Error ? cause.message : copy.operationFailed));
+       }} />


─── apps/web/src/chat/ChatRoutePage.tsx:10-10 ───
[maintainability · low] 静态导入整个 career 页面模块（OpportunityPage.tsx，429 行，含证据页/评估页渲染逻辑）到 chat 路由：Vite
会把该模块打进 chat chunk，增加包体；同时形成 chat → career 的跨域硬耦合（已核实 career 无反向依赖 chat，暂无循环）。且
conversationActionSlot 挂载无任何能力/工作区门控，无 career 功能的空间里每个会话视图顶部都会出现这套导入表单，提交后才收到 forbidden。建议 lazy
拆分该导出并按工作区能力门控。

- import { OpportunityImportPanel } from '../career/OpportunityPage.tsx';
+ const OpportunityImportPanel = React.lazy(() =>
+   import('../career/OpportunityPage.tsx').then((module) => ({ default: module.OpportunityImportPanel })));
+ // 挂载处用 <Suspense fallback={null}> 包裹，并在无 career 能力的工作区返回 null。


─── apps/web/src/chat/chat-header.tsx:177-178 ───
[maintainability · low] onVisibleChange 中 if (context?.trigger === 'document') 分支与后续默认执行完全相同（都是
onMenuVisibleChange(visible) 后隐式返回），if/return 结构是死代码，会让维护者误以为两条路径有差异，直接合并为单次调用即可。

-             if (context?.trigger === 'document') { onMenuVisibleChange(visible); return; }
+           onVisibleChange={(visible) => {
+             // trigger click 与 Esc/外点关闭同样回到 menu 态
              onMenuVisibleChange(visible);
+           }}


─── apps/web/src/chat/chat-header.tsx:5-5 ───
[maintainability · low] 该副作用导入冗余：组件内未使用任何 wk-vc-* 类（chat-header* / sandbox-header-toggle* 全部定义在
chat.td.css，而本文件反而未导入 chat.td.css）；唯一消费方 ChatRoutePage 已自行导入 views-chat-u.css 与
chat.td.css。建议移除，避免误导后续维护者以为本组件依赖 views 段样式。

- import './views-chat-u.css';
+ // 删除该 import：ChatHeader 的样式位于 chat.td.css，由宿主（ChatRoutePage）负责导入。


─── apps/web/src/chat/views-chat-u.css:1668-1677 ───
[maintainability · low] 本文件存在成片死声明，建议清理为单一首选值：(1) .wk-vc-tool-result-2/-8/-14/-15/-24/-25 同一规则内先声明
font-size/color 再以 !important 重复声明（如本处的 font-size: 0.7rem / color: #8a94a6 永不生效），应删除被覆盖的首值；(2)
.wk-vc-page-23 的 font-size: inherit 被 font-size: 13px 覆盖、.wk-vc-page-52 与 composer-18/-20/-22/-24 的
transition-duration: 150ms 被 200ms 覆盖；(3) @keyframes pulse 在本文件定义两次（:640 与 :2364）；(4)
.wk-vc-tool-approval-12 拆成两处定义可合并；(5) 多处 codemod 产物把两条声明挤在同一行（border-bottom-width: 1px;border-color:
#e7e7e7）。不影响渲染但显著降低可维护性。

  .wk-vc-tool-result-2 {
    /* Vue .arguments-label/.fallback-label secondary */
    margin-right: 0.35rem;
-   font-size: 0.7rem;
    font-weight: 600;
-   color: #8a94a6;
-   font-family: var(--app-font-family-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace) !important;
-   font-size: 0.8rem !important;
-   color: rgba(0, 0, 0, 0.6) !important;
+   font-family: var(--app-font-family-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace);
+   font-size: 0.8rem;
+   color: rgba(0, 0, 0, 0.6);
  }


─── apps/web/src/chat/chat.td.css:657-667 ───
[maintainability · low] .steer-failure 重复声明 color：var(--td-text-color-secondary) 是死值，立即被
var(--td-error-color) 覆盖，应只保留后者。

  .steer-failure {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 4px;
    font-size: 12px;
-   color: var(--td-text-color-secondary);
+   color: var(--td-error-color);
    margin-bottom: 4px;
    margin-top: 6px;
-   color: var(--td-error-color);
  }


─── apps/web/src/chat/chat-u.css:201-209 ───
[style · low] codemod 产物把两条声明挤在同一行且 border-color 未限定边（border-top-width: 1px;border-color:
#eef1f5）：当前仅因其余三边 width 为 0 才等价于 border-top-color，后续若追加 border-x-width 会意外显色；.wk-ssd-14
存在同样问题。建议规范书写为单条 border-top 简写。

  .wk-bad-19 {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
-   border-top-style: solid;
-   border-top-width: 1px;border-color: #eef1f5;
+   border-top: 1px solid #eef1f5;
    padding-top: 12px;
  }


─── apps/web/src/knowledge-settings/KnowledgeSettingsPage.css:635-636 ───
[bug · low] 分段组左缘边框缺失(边界条件):基础规则对所有段设 `border-left: 0`,`.kb-type-tab + .kb-type-tab`
只恢复了第二段的左边框;首段仅当自身 `.is-checked`(全边框 #8ce0af)时才有左边框。当 KB 类型为 FAQ(末段选中、首段为 disabled 未选中)时,分段组左缘会缺 1px
边框。TDesign 原生实现以 `.t-radio-button:first-child { border-left-width: 1px }` 兜底,此处应补齐(注意放在
`.kb-type-tab.is-checked` 之前,二者同为 0-2-0 特异度,靠源顺序让选中态覆盖)。

    border: 1px solid var(--wk-color-border, #dcdcdc);
    border-left: 0;
+ 
+ /* tdesign `.t-radio-button:first-child`：首段恢复左边框，否则 FAQ 型 KB
+    （首段未选中）分段组左缘缺 1px 边框。须置于 .is-checked 规则之前。 */
+ .kb-type-tab:first-child {
+   border-left: 1px solid var(--wk-color-border, #dcdcdc);
+ }


─── apps/web/src/knowledge-settings/KnowledgeSettingsPage.css:759-761 ───
[bug · medium] 自绘 checkbox 缺键盘焦点指示:原生 input 视觉隐藏(1px clip)后仍是 Tab 焦点,但 `.kb-checkbox-input` 没有任何
focus 样式,键盘用户无法看到当前聚焦项。注释声明逐项复刻 tdesign 官方几何,但 tdesign checkbox 自带聚焦环(`.t-checkbox:focus-visible +
.t-checkbox__input` 的 box-shadow),此处应补 `:focus-visible` 规则。

+ .indexing-check-head input:focus-visible + .kb-checkbox-input {
+   outline: 2px solid var(--wk-color-brand, #07c05f);
+   outline-offset: 2px;
+ }
  .indexing-check-head input:disabled:checked + .kb-checkbox-input::after {
    border-color: rgba(0, 0, 0, 0.26);
  }


─── apps/web/src/knowledge-settings/KnowledgeSettingsPage.css:831-831 ───
[style · low] `border-color: none` 是非法声明(border-color 不接受 none,整条会被 CSS 解析器丢弃)。内层 input 已 `border:
0`、聚焦描边由外层 `.kb-text-input-wrap:focus-within` 承担,这条 `:focus` 规则实际只剩 `outline: none`(且 base
已有),建议直接删除或改为合法值。

- .kb-text-input:focus { outline: none; border-color: none; }
+ /* 内层 input 无边框，聚焦态由外层 .kb-text-input-wrap:focus-within 呈现，此规则可删除 */


─── apps/web/src/knowledge/knowledge-u.css:251-253 ───
[style · low] 选择器 `.wk-kg-24 :-webkit-details-marker` 中的空格使其成为后代选择器——`::-webkit-details-marker` 是
summary 元素自身的伪元素而非其后代,该规则永远匹配不到(原 Tailwind `[&::-webkit-details-marker]:hidden` 指向元素自身)。当前因
`.wk-kg-24` 的 `display: inline-flex` 使 Chrome 不再生成 marker 才未暴露问题,但这是一条无效的死规则,应去掉空格修正。

- .wk-kg-24 :-webkit-details-marker {
+ .wk-kg-24::-webkit-details-marker {
    display: none;
  }


─── apps/web/src/knowledge-settings/parserSettings.tsx:10-11 ───
[maintainability · low] 此处导入的 `knowledge-settings-u.css` 仅含 KBShareSettingsSection 使用的 `.wk-kss-*`
规则,parserSettings.tsx 未使用任何 wk-kss 类(KBShareSettingsSection 已自行导入该样式)。多余导入会误导后续维护者以为 Select
依赖此样式,建议移除。

  import { Select } from 'tdesign-react';
- import './knowledge-settings-u.css';


─── apps/web/src/knowledge-settings/parserSettings.tsx:226-230 ───
[bug · low] `aria-label` 挂在无 role 的包裹 `<span>` 上,多数辅助技术不会将其作为可访问名称播报;原实现是原生 `<select
aria-label={group.label}>`,迁移后组合框内部 input 实际丢失了可访问名称(台账 #8 只解决了 data-* 不透传,data-*
与可访问名称应区别处理)。建议参照仓库既有 `inputProps` 透传惯例(AgentEditorModal.tsx)把 label 传给 Select 内部 input,span 只保留
data-* 标识。

                <span
                  data-parser-group={group.key}
-                 aria-label={group.label}
                  style={{ display: 'block' }}
                >
+                 <Select
+                   inputProps={{ 'aria-label': group.label } as never}
+                   …


─── apps/web/src/commercial/surface.tsx:9-13 ───
[maintainability · medium] Card/Status 与本次更新中新增的共享兼容层 apps/web/src/shared/wk-legacy.tsx 的
WkCard/WkStatus 完全同构（DOM 标签、role={tone==='error'?'alert':'status'}、tone 变体逐项一致，.wk-cs-status 色值
#506078/#b42318/#137333/#9a6700 与 wk-legacy.css 的 .wk-status 族逐字相同），documents/settings/platform/auth
等域均已复用共享层，商业域又维护了一份域内副本，属重复代码。且两份实现的卡片边框色已分叉：共享层 .wk-card 为 #dce3ed（packages/ui 原 --color-line），本域
.wk-cs-card 为 #e7e7e7，同为旧 Card 的替代品却产生跨域视觉不一致；注释引用的 packages/ui/src/index.tsx:29-37
所在包已被整体删除，该锚点亦失效。建议直接复用共享层；若确认商业域历史上实际生效的就是 #e7e7e7（注释所述 knowledge-u.css 误置定义），应在共享层以 CSS
变量或变体类承载该差异，而非再造一份副本。

- /** 语义卡片：白底、line 描边、card 圆角（视觉 = 既有 .wk-card）。 */
- export function Card({ children, className, ...props }: HTMLAttributes<HTMLDivElement> & { children?: ReactNode }) {
-   const classes = ['wk-cs-card', className].filter(Boolean).join(' ');
-   return <section className={classes} {...props}>{children}</section>;
- }
+ // 复用共享兼容层（S6 已收编 packages/ui 旧栈 Card/Status），避免域内第三份同构副本：
+ import { WkCard, WkStatus } from '../shared/wk-legacy';
+ 
+ export const Card = WkCard;
+ export const Status = WkStatus;


─── apps/web/src/commercial/surface.tsx:18-18 ───
[style · low] toneClass 使用三层嵌套三元表达式，违反「禁止嵌套三元表达式」规则；且 shared/wk-legacy.tsx 的 WkStatus
已有更简洁的模板字符串写法（tone !== 'neutral' && `wk-status--${tone}`）。建议改为一层三元 + 模板字符串。

-   const toneClass = tone === 'error' ? 'wk-cs-status--error' : tone === 'success' ? 'wk-cs-status--success' : tone === 'warning' ? 'wk-cs-status--warning' : '';
+   const toneClass = tone === 'neutral' ? '' : `wk-cs-status--${tone}`;


─── apps/web/src/commercial/commercial-u.css:41-41 ───
[maintainability · low] .wk-bill-usage-number-cell 在本文件第 19 行已定义（text-align: right），此处第二次定义
font-variant-numeric: tabular-nums，同一选择器分散两处，后续维护（如再调数字列样式）易漏改其一，应合并为一条规则。另第 16 行
.wk-bill-usage-table-cell 中 `border-bottom-width: 1px;border-color: #eef1f5;`
两个声明挤在一行且缺分隔空格，格式不规范，建议整理。

- .wk-bill-usage-number-cell { font-variant-numeric: tabular-nums; }
+ /* 合并至第 19 行规则： */
+ .wk-bill-usage-number-cell {
+   text-align: right;
+   font-variant-numeric: tabular-nums;
+ }


─── apps/web/src/commercial/commercial-u.css:45-45 ───
[maintainability · low] 卡片边框/背景写死 #e7e7e7、#ffffff，而 packages/design-tokens/src/tdesign-theme.css 已提供
--td-component-stroke（亮色 = #e7e7e7，暗色模式第 75 行覆盖为 var(--td-gray-color-11)）与
--td-bg-color-container；注释自述该值来源即 var(--td-component-stroke)，Vue 端用的是变量而此处写死字面量。暗色模式下商业域卡片将保持白底亮边，与
TDesign 主题脱节；.wk-bill-1 的 #e7e7ea 同理。建议改用 token 变量。

- .wk-cs-card { border-radius: 8px; border: 1px solid #e7e7e7; background-color: #ffffff; padding: 16px; }
+ .wk-cs-card {
+   border-radius: 8px;
+   border: 1px solid var(--td-component-stroke, #e7e7e7);
+   background-color: var(--td-bg-color-container, #ffffff);
+   padding: 16px;
+ }


─── apps/web/src/commercial/BillingPage.tsx:119-120 ───
[maintainability · low] wk-bill-1（表头行底边框）与 wk-bill-2（表头字重）为无意图的序号式类名，无法自我表达用途，后续读者需跳到
commercial-u.css 才能理解，且与同文件其余语义化命名（wk-bill-usage-table-cell 等）风格不一致。这两个类仅在 BillingPage
表头使用，建议改为语义化命名。

- <tr className="wk-bill-1">
-                   <th className={USAGE_TABLE_CELL + ' wk-bill-2'}>资源</th>
+ <tr className="wk-bill-thead-row">
+                   <th className={USAGE_TABLE_CELL + ' wk-bill-th'}>资源</th>


─── internal/modules/workbench/service/workbench/application_task.go:84-88 ───
[bug · low] Ensure 链路对 ownerID 的校验值与实际使用值不一致：normalizeApplicationTaskIntent 内部经
normalizeApplicationTaskScope 校验的是 trim 后的 ownerID（并丢弃返回的规范化值），但此处继续把未 trim 的原始 ownerID 传给
ensureWithRetry/ensureOnce，所有查询与 workbench_application_tasks/agent_runs/sessions 的写入都以原始值为准。对照
FindCareerApplicationTask 会用 normalizeApplicationTaskScope 的返回值回写（`tenantID, ownerID, requestID, err
:= normalizeApplicationTaskScope(...)`），一旦调用方传入带空白前缀的 ownerID：Ensure 会以 " user1" 落库（校验通过因为 trim
后非空），Find 却以 "user1" 查询返回 NotFound——同一调用方 Ensure/Find 语义分裂，恢复循环永远无法收敛。建议与 Find 对齐，在入口处用规范化后的值继续传递。

+ 	ownerID = strings.TrimSpace(ownerID)
  	intent, err := normalizeApplicationTaskIntent(tenantID, ownerID, intent)
  	if err != nil {
  		return interfaces.CareerApplicationTaskLink{}, err
  	}
  	return c.ensureWithRetry(ctx, tenantID, ownerID, intent, c.ensureOnce)


─── apps/mobile/src/task-office-integration-smoke.ts:47-50 ───
[performance · low] probeServerAuthBoundary 的 fetch 没有显式超时信号：对端若接受 TCP 连接但不响应（LB
黑洞、误配端口），探针会一直挂起——Node 18+ 的 undici 默认 headersTimeout 约 300s，其他运行环境（Expo/React
Native）可能无界。该探针的职责是尽快如实产出 'unreachable' 证据，挂起会导致整个 smoke 命令长时间无产出。建议加
AbortSignal.timeout(...)，超时后落入现有 catch 记为 'unreachable'。

        method: 'GET',
        redirect: 'error',
        headers: { accept: 'application/json' },
+       signal: AbortSignal.timeout(10_000),
      });


─── internal/handler/session/workbench_artifacts.go:254-263 ───
[maintainability · low] TTL 解析/封顶块与 CreateWorkbenchArtifactSignedURL（约 176-184
行）逐字重复。两处都是安全敏感的签发端点，后续若调整封顶策略（例如按版本类缩短上限）很容易只改一处，导致两类 grant 的有效期语义静默分叉。建议提取为共享 helper（如
workbenchGrantTTL(c, fallback)），两个端点共用。

- 	ttl := h.ttl
+ // workbenchGrantTTL 解析并封顶调用方请求的 TTL，两个签发端点共用。
+ func workbenchGrantTTL(c *gin.Context, fallback time.Duration) time.Duration {
+ 	ttl := fallback
  	if raw := strings.TrimSpace(c.Query("ttl_seconds")); raw != "" {
  		if requested, parseErr := strconv.Atoi(raw); parseErr == nil && requested > 0 {
  			ttl = time.Duration(requested) * time.Second
  		}
  	}
  	if ttl > workbench.MaxArtifactGrantTTL || ttl <= 0 {
- 		ttl = workbench.MaxArtifactGrantTTL
+ 		return workbench.MaxArtifactGrantTTL
  	}
- 	grant := workbench.ArtifactVersionGrant{TenantID: run.Key.TenantID, OwnerID: run.UserID, RunID: run.Key.RunID, SessionID: run.SessionID, VersionID: version.ID, ExpiresAt: time.Now().Add(ttl).Unix()}
+ 	return ttl
+ }
+ // 调用处：
+ 	grant := workbench.ArtifactVersionGrant{TenantID: run.Key.TenantID, OwnerID: run.UserID, RunID: run.Key.RunID, SessionID: run.SessionID, VersionID: version.ID, ExpiresAt: time.Now().Add(workbenchGrantTTL(c, h.ttl)).Unix()}


─── apps/web/src/organizations/OrganizationsPage.tsx:1026-1027 ───
[bug · medium] 无障碍回归：卡片此前带 role="button" tabIndex={0} 和 Enter 键 onKeyDown，可键盘打开设置弹窗；迁移后只剩 onClick 的纯
div，键盘用户完全无法进入组织设置。同段的 .more-wrap（原 aria-label="common.moreActions" + role=button + 键盘支持）也退化为纯
div。建议保留 Vue DOM 类名的同时补回键盘语义。

        <div key={org.id || index} className={'org-card' + (owner ? '' : ' joined-org')} style={rowHidden ? { display: 'none' } : undefined}
-         onClick={() => openSettingsModal(org)}>
+         role="button" tabIndex={0}
+         onClick={() => openSettingsModal(org)}
+         onKeyDown={(event) => { if (event.key === 'Enter') openSettingsModal(org); }}>


─── apps/web/src/organizations/OrganizationsPage.tsx:1878-1878 ───
[bug · low] label 关联断裂：改为 TInput/TSelect 时丢失了 id（原为 <Input id="join-search">），而 label 仍是
htmlFor="join-search"，点击标签不再聚焦输入框、读屏也无法关联。同类问题还有 htmlFor="join-code"（1866 行，TInput 只传了 name）、forHtml
升级角色 TSelect（upgrade-role/join-request-role）以及 create 模式 name 字段（1364 行 label
htmlFor="organization-name"，1373 行 TInput 无 id）。建议给对应 TInput/TSelect 补上 id（TDesign 会透传到内部 input）。

-                           <TInput className={ORG_FIELD + ' wk-org-6'} value={searchQuery} placeholder={t(locale, 'organization.join.searchSpacesPlaceholder')} onChange={(value) => onSearchQueryChange(String(value))} onKeydown={(_, context) => { if (context.e.key === 'Enter') runSearch(searchQuery.trim()); }} />
+ <TInput id="join-search" className={ORG_FIELD + ' wk-org-6'} value={searchQuery} placeholder={t(locale, 'organization.join.searchSpacesPlaceholder')} onChange={(value) => onSearchQueryChange(String(value))} onKeydown={(_, context) => { if (context.e.key === 'Enter') runSearch(searchQuery.trim()); }} />


─── apps/web/src/organizations/OrganizationsPage.tsx:1277-1278 ───
[bug · low] Tooltip 包裹禁用按钮的守卫提示不可达：disabled 的原生 button 不会派发 mouseenter/click 等鼠标事件，TDesign Tooltip
把事件挂在子元素上，因此 !canManageOrg 时（正是需要提示的时候）tooltip 永远显示不出来；且 Tooltip 的 disabled 属性在全仓仅此处使用，需确认
tdesign-react 版本是否支持。旧实现用原生 title={writeGuardTitle}（禁用态仍有效），建议保留 title 或在 Button 外再包一层 span 作为
tooltip trigger。

-                 <Tooltip content={writeGuardTitle} placement="top" disabled={canManageOrg}>
-                   <Button theme="default" variant="outline" className="org-join-btn" disabled={!canManageOrg} onClick={openJoinModal} icon={<TIcon name="enter" />}>
+ <Button theme="default" variant="outline" className="org-join-btn" disabled={!canManageOrg} onClick={openJoinModal} icon={<TIcon name="enter" />} title={!canManageOrg ? writeGuardTitle : undefined}>


─── apps/web/src/organizations/OrganizationsPage.tsx:21-22 ───
[maintainability · low] 同一组件以两个别名重复导入：Input（Input 与 TInput）和 Textarea（Textarea 与 TTextarea）各自从
'tdesign-react' 引入两次，同一文件里两种写法并存（create 模式用 <Input>/<Textarea>，留守段用
<TInput>/<TTextarea>），增加阅读与检索成本。建议合并为一条 import 并统一别名（例如全部用 T 前缀别名）。

- import { Input as TInput, Select as TSelect, Switch as TSwitch, Textarea as TTextarea } from 'tdesign-react';
- import { Button, Dialog, Input, Popup, Skeleton, Tag, Textarea, Tooltip } from 'tdesign-react';
+ import { Button, Dialog, Input as TInput, Popup, Select as TSelect, Skeleton, Switch as TSwitch, Tag, Textarea as TTextarea, Tooltip } from 'tdesign-react';


─── apps/web/src/organizations/orgs.td.css:662-665 ───
[maintainability · low] .org-section-header 选择器在相邻位置被拆成两个独立块（前一块设置 grid-column/pointer-events，本块设置
sticky 布局与视觉），且本块的 cursor: pointer 落在 pointer-events: none
的容器上，实际只对子元素生效。建议合并为一个规则块（或补注释说明拆分原因），避免后续读者在两处推断层叠结果。

  .org-section-header {
+   grid-column: 1 / -1;
+   display: flex;
+   align-items: center;
+   gap: 6px;
+   pointer-events: none;
+   /* 粘性吸顶 + 上下遮罩（容器 pointer-events:none，子元素恢复 auto） */
    position: sticky;
    top: 0;
    z-index: 5;
+   ...


─── apps/web/src/commercial/surface.tsx:5-5 ───
[maintainability · medium] surface.tsx 作为本域封装层封装了 Card/Status，但 Button 的域统一视觉基准 `theme="default"
variant="outline"`（注释自述=旧栈默认白底细描边）未收敛到本层，而是以字面量散布在
AdminCommercialPage(5)/RefundPage(3)/BillingPage(2)/CheckoutPage(2)/TaskBudget(1) 共 13
处调用点。后续若调整描边风格或对齐 TDesign 主题，需同步修改 13 处，新增按钮也容易漏写这对 props
造成视觉不一致。建议在本层导出一个带默认值的按钮封装（渲染结果与现状完全一致），调用点只传业务 props。

- // 调用点 theme="default" variant="outline" 对应旧栈默认白底细描边）。
+ import { Button as TdButton, type ButtonProps } from 'tdesign-react';
+ 
+ /** 域内统一按钮：默认白底细描边（= 旧栈 Button 默认视觉），调用点无需重复写 theme/variant。 */
+ export function Button({ theme = 'default', variant = 'outline', ...props }: ButtonProps) {
+   return <TdButton theme={theme} variant={variant} {...props} />;
+ }


─── apps/web/src/commercial/commercial-u.css:2-2 ───
[documentation · low] 头注释声明「导入顺序：须在本域 .td.css 之前」，但 apps/web/src/commercial/ 目录下并不存在任何 .td.css
文件（本域仅新增了 commercial-u.css），该约束指向不存在的文件，疑为其他域（settings/agents 等确有
td.css）模板复制残留，会误导后续维护者去寻找/创建不存在的文件。建议删除该半句，或改为说明与全局 TDesign 主题 CSS 的实际顺序关系。



─── apps/web/src/chat/views-chat-u.css:1697-1701 ───
[bug · medium] 下边框宽度回归：该类对应的原始 utility 串是 `border-b-2
border-b-[#e3e8ef]`（packages/views/src/chat/tool-result.tsx 数据库查询表头 th，border-b-2 = 2px），这里被写成
8px。表头与表体之间会出现 4 倍粗的分隔线，视觉与 Vue 事实源不一致。同表其余边框均为 1px（wk-vc-tool-result-40/-11/-12），此处 8px 明显是 codemod
换算错误。

  .wk-vc-tool-result-6 {
    white-space: nowrap;
-   border-bottom-style: solid;
-   border-bottom-width: 8px;
-   border-bottom-color: #e3e8ef;
+   border-bottom: 2px solid #e3e8ef;


─── apps/web/src/chat/chat-header.tsx:125-126 ───
[bug · low] 守卫条件失效：`!menuMode` 恒为 false（menuMode 是 'menu' | 'clear' | 'delete' 的非空字符串联合），该检查真正想挡的
'menu' 态没有挡住——menuMode === 'menu' 时会落入 else 分支调用 `props.onDeleteSession?.()`。当前确认按钮只在弹层 confirm
分支渲染所以未实际触发，但这是一层失效的防护：后续若在 menu 态复用/暴露该函数即会误删会话。建议显式排除 'menu' 态。

    async function submitDangerAction(): Promise<void> {
-     if (!menuMode || dangerBusy) return;
+     if (menuMode === 'menu' || dangerBusy) return;


─── apps/web/src/chat/ChatRoutePage.tsx:1931-1932 ───
[maintainability · low] 同一会话对象在此处重复 find 三次（title / isPinned / renameTitle 各一次）。sessions
变更时三次查找结果需保持一致，也多做了两次线性扫描；建议像 page.tsx 的 selectedSession 派生模式那样在渲染前计算一次并复用。

-         title={sessions.find((session) => session.id === selectedSessionId)?.title || copy.newSession}
-         isPinned={sessions.find((session) => session.id === selectedSessionId)?.is_pinned === true}
+         title={selectedSession?.title || copy.newSession}
+         isPinned={selectedSession?.is_pinned === true}
+         /* selectedSession = sessions.find((session) => session.id === selectedSessionId) 在组件体中计算一次 */


─── apps/web/src/chat/SessionShareDialog.tsx:6-7 ───
[documentation · low] 头注释已过时：本次改动把内联 Tailwind utilities 全部替换为 wk-ssd-* 语义类并新增 `import
'./chat-u.css'`，注释里“这里全部用内联 Tailwind utilities（QueryHistorySnapshotDrawer 抽屉同风格）”不再成立；而现用的
chat-u.css 是普通样式文件，不涉及旧栈 theme.css 破坏 node 模块加载的问题。保留旧表述会误导后续维护者继续沿“禁止样式依赖”的旧约束修改本文件，建议同步改写。



─── apps/web/src/chat/views-chat-u.css:2529-2530 ───
[maintainability · low] 同一文件内重复定义：`.wk-vc-tool-approval-12` 在上方 tool-approval
段已定义（font-size/line-height 等），这里又补一条 tabular-nums；`@keyframes pulse` 也在 wk-vc-page-54 与
wk-vc-message-list-32 之后声明了两次。拆成多处会让后续维护者只改其一，建议将同一选择器的属性合并到一处规则、keyframes 只保留一份。

- /* 工具审批倒计时等宽数字（原 tabular-nums）。 */
- .wk-vc-tool-approval-12 { font-variant-numeric: tabular-nums; }
+ /* 合并进上方 tool-approval 段的规则： */
+ .wk-vc-tool-approval-12 {
+   white-space: nowrap;
+   font-size: 12px;
+   line-height: 1.55;
+   font-variant-numeric: tabular-nums;
+ }


─── apps/web/src/chat/views-chat-u.css:347-356 ───
[maintainability · low] 与 chat-u.css 已确认的 wk-bad-19 同源的 codemod 挤行产物：`border-bottom-width:
1px;border-color: #eef1f5;` 两条声明同行且 border-color 未限定边（设置了四边颜色），当前仅因其余三边 style/width
未设而恰好只显示下边框；后续若给其他边补 width 会意外显色。本文件同类写法还出现在
.wk-vc-page-41/-42、.wk-vc-agent-selector-4/-24、.wk-vc-session-sidebar-12、.wk-vc-message-list-9/-15（后
几处为 #e7e7e7），建议统一改写为单条 border-bottom 简写。

  .wk-vc-page-32 {
    margin: 0;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
-   border-bottom-style: solid;
-   border-bottom-width: 1px;border-color: #eef1f5;
+   border-bottom: 1px solid #eef1f5;
    padding-bottom: 1rem;
  }


─── apps/web/src/settings/ChatHistorySettingsPanel.tsx:98-102 ───
[bug · low] 可访问性回退：被本 PR 从 ConfigSettingsPanel 删除的 chathistory 分支里，启用开关带有
aria-label={t('chatHistorySettings.enableLabel')}；新独立面板的 TSwitch 未提供可访问名称，且可见 <label> 位于兄弟节点（无
htmlFor/id 关联），屏幕阅读器只能读到未命名开关，无法得知其控制的是"启用聊天记录"。与已确认的 GeneralPreferencesPanel Switch 丢 aria-label
同类问题，此处为新增文件中的独立实例，建议同样补回 aria-label（TDesign Switch 支持属性透传）。

-             <TSwitch value={enabled} onChange={(value) => {
+             <TSwitch value={enabled} aria-label={t('chatHistorySettings.enableLabel')} onChange={(value) => {
                setEnabled(value === true);
                latestRef.current = { ...latestRef.current, enabled: value === true };
                scheduleSave();
              }} />


─── apps/web/src/settings/EnvVarSettingsPanel.tsx:166-167 ───
[bug · low] 校验回退：迁移到 TDesign 控件时丢失了原有的三处原生 required（sandbox/skill 的 scopeId 控件、变量名输入框、变量值输入框）以及
sandbox 下拉的禁用占位项 <option value="" disabled>。此前空值提交会被浏览器本地化校验直接拦截；现在空的 name/scopeId 会一路提交到
envVarSet，由其抛出硬编码英文错误（"Variable name is required" / "Sandbox config ID is required"）并经 errorText
原样展示——在五语言界面下出现英文报错，且 sandbox 场景用户可在未选择任何沙箱时提交表单。value 已有本地化预校验（valueRequired），建议在 setVariable 内为
name/scopeId 补同样的本地化预校验（或改用 TDesign Form rules），而非依赖 surface.ts 抛出的英文 message。

-       <label>{t('envVarSettings.namePlaceholder')}<Input value={name} onChange={(next) => setName(String(next ?? ''))} /></label>
-       <label>{t('envVarSettings.valuePlaceholder')}<Input type="password" autocomplete="new-password" value={value} onChange={(next) => setValue(String(next ?? ''))} /></label>
+   function setVariable(event: React.FormEvent<HTMLFormElement>) {
+     event.preventDefault();
+     if (!name.trim()) { setNotice(null); setError(t('envVarSettings.nameRequired')); return; }
+     if (!scopeId.trim()) { setNotice(null); setError(t('envVarSettings.scopeRequired')); return; }
+     if (!value) { setNotice(null); setError(t('envVarSettings.valueRequired')); return; }
+     // …


─── apps/web/src/knowledge/knowledge-u.css:699-700 ───
[maintainability · medium] `.wk-ds-self-start` 是 data-sources 域的规则（前缀 wk-ds-*），却定义在 knowledge 域的
knowledge-u.css 中。其唯一使用方是 apps/web/src/data-sources/DataSourcesPage.tsx:441 的 `<a
className="wk-ds-15 wk-ds-self-start">`，而该页面导入的是 data-sources-u.css（其中定义了 .wk-ds-15 但没有
.wk-ds-self-start）。这形成跨域隐性依赖：DataSourcesPage 同时被 KnowledgeBasesPage.tsx:1879 挂载，用户只走 KB 列表页路由时
knowledge-u.css 未必加载（它由 KnowledgeGraphPage.tsx 导入），该链接的 justify-self: start 会失效退化为 stretch。建议把这条规则移入
data-sources-u.css（与 .wk-ds-15 同域）。

  .wk-graph-arrow-stroke { stroke: #c0c4cc; }
- .wk-ds-self-start { justify-self: start; }
+ /* .wk-ds-self-start 移至 apps/web/src/data-sources/data-sources-u.css（与 .wk-ds-15 同域） */


─── apps/web/src/configuration/ui.tsx:12-15 ───
[maintainability · low] 此 Card/Status 封装与同一变更批次中 commercial/surface.tsx、data-sources/ui.tsx
的实现逐字相同（仅 CSS 类前缀 wk-cfg-/wk-cs-/wk-dsui- 不同），同一 PR 内已有三份拷贝。后续若需调整可访问性属性（role）或新增 tone
变体，必须三处同步修改，易漏改。建议抽取到共享模块（如 apps/web/src/shared/），通过 base class 或 prefix 参数化，各域仅保留自己的样式文件。

- export function Card({ children, className, ...props }: HTMLAttributes<HTMLDivElement> & { children?: ReactNode }) {
-   const classes = ['wk-cfg-card', className].filter(Boolean).join(' ');
+ // 抽取共享壳组件（示意）：
+ // apps/web/src/shared/wk-shell.tsx
+ export function WkCard({ base, children, className, ...props }: HTMLAttributes<HTMLDivElement> & { base: string; children?: ReactNode }) {
+   const classes = [base, className].filter(Boolean).join(' ');
    return <section className={classes} {...props}>{children}</section>;
  }


─── apps/web/src/configuration/ui.tsx:20-20 ───
[style · low] toneClass 采用三层链式三元表达式，违反团队规范"嵌套三元表达式不允许"。建议改为 Record 映射，可读性更好且与 tone 类型联动（新增 tone
时编译器会强制补全）。

-   const toneClass = tone === 'error' ? 'wk-cfg-status--error' : tone === 'success' ? 'wk-cfg-status--success' : tone === 'warning' ? 'wk-cfg-status--warning' : '';
+ const TONE_CLASS: Record<'neutral' | 'error' | 'success' | 'warning', string> = {
+   neutral: '',
+   error: 'wk-cfg-status--error',
+   success: 'wk-cfg-status--success',
+   warning: 'wk-cfg-status--warning',
+ };
+ // ...
+ const toneClass = TONE_CLASS[tone];


─── apps/web/src/configuration/config-u.css:258-269 ───
[maintainability · low] 同一规则内 font-size: 0.8rem 与 color: #6941c6 均被后续 !important 声明覆盖，属死声明；同类问题还有
.wk-cfg-ed-5 在 display: flex !important 下保留的无效 grid-template-columns: auto 1fr。虽为忠实平移旧 utility
覆盖链（text-[0.8rem]…text-[0.85rem]!），但死代码会误导后续维护者以为可调，建议只保留实际生效的声明（视觉效果不变）。

  .wk-cfg-page-16 {
    border-radius: 999px;
    padding-inline: 0.55rem;
    padding-block: 0.2rem;
-   font-size: 0.8rem;
-   color: #6941c6;
    background-color: #f4f3ff;
    margin-right: auto;
    font-size: 0.85rem !important;
    /* 旧栈 muted/muted-strong→Vue var(--td-text-color-secondary)=rgba(0,0,0,.6) */
    color: rgba(0, 0, 0, 0.6) !important;
  }


─── apps/web/src/configuration/config-u.css:23-29 ───
[maintainability · low]
本文件内多组规则完全重复：.wk-cfg-ops-3/.wk-cfg-ed-6/.wk-cfg-page-4（操作条）、.wk-cfg-ops-9/.wk-cfg-page-5/.wk-cfg-ed-
7/.wk-cfg-mun-4（列表重置）、.wk-cfg-ops-10/.wk-cfg-page-1/.wk-cfg-ed-8/.wk-cfg-mun-5（列表项）、.wk-cfg-ops-6/.w
k-cfg-ed-1/.wk-cfg-mun-1（含 720px 媒体查询的面板标题）逐字相同，.wk-cfg-ops-4 的 input 与 textarea 两大段属性也逐字重复（约 15 行 ×
2）。改一处样式需同步 N 处。建议合并为逗号选择器，保持类名不变的同时收敛维护点。

- .wk-cfg-ops-3 {
+ .wk-cfg-ops-3,
+ .wk-cfg-ed-6,
+ .wk-cfg-page-4 {
    margin-bottom: 0.75rem;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 0.5rem;
  }
+ 
+ /* 同理：.wk-cfg-ops-4 input, .wk-cfg-ops-4 textarea { … } 合并两大段 */


─── apps/web/src/configuration/config-u.css:5-8 ───
[maintainability · low] muted/line/ink 等颜色全部落成字面值（rgba(0,0,0,0.6)、#e7e7e7、rgba(0,0,0,0.9)）。这些值对应的
CSS 变量在 packages/design-tokens/src/tdesign-theme.css 中已定义，且 :root[theme-mode="dark"] 会改写它们（如
--td-text-color-secondary → rgba(255,255,255,.55)）；应用非 embed 路由启动时 initTheme()
会应用存储主题并跟随系统深浅色切换（main.tsx:106-110）。同一变更批次中新增的 career/*.css、agents.td.css 均采用
var(--td-text-color-secondary, fallback) 形式。建议改用变量+fallback：浅色下视觉不变，深色下可自适应，且与同批文件模式一致。

  .wk-cfg-ops-1 {
    /* 旧栈 muted/muted-strong→Vue var(--td-text-color-secondary)=rgba(0,0,0,.6) */
-   color: rgba(0, 0, 0, 0.6);
+   color: var(--td-text-color-secondary, rgba(0, 0, 0, 0.6));
  }


─── apps/web/src/organizations/OrganizationsPage.tsx:1216-1227 ───
[bug · medium] 页头「加入/创建」两个图标按钮重演了空状态处已确认的 Tooltip 守卫问题：disabled 的 Button 不派发鼠标事件，tdesign Tooltip
把事件绑定在子元素上，导致 !canManageOrg（正是需要展示 writeGuardTitle 提示的时候）tooltip 永远无法弹出；旧实现用原生 title
属性在禁用态仍可见。此处为独立位置，需单独修复：可在 Button 外包一层 span 作为 tooltip trigger，同时建议给 icon-only 按钮补
aria-label（kb-list 同款 header-action-btn 有意省略 aria 有注释说明，本页可沿用其先例，但禁用提示问题仍需处理）。

                  <Tooltip content={canManageOrg ? t(locale, 'organization.joinOrg') : writeGuardTitle} placement="bottom">
+                   <span className="header-action-wrap" aria-label={t(locale, 'organization.joinOrg')} title={canManageOrg ? undefined : writeGuardTitle}>
-                   <Button
+                     <Button
-                     variant="text"
+                       variant="text"
-                     theme="default"
+                       theme="default"
-                     size="small"
+                       size="small"
-                     className="header-action-btn"
+                       className="header-action-btn"
-                     style={{ '--wails-draggable': 'no-drag' } as CSSProperties}
+                       style={{ '--wails-draggable': 'no-drag' } as CSSProperties}
-                     disabled={!canManageOrg}
+                       disabled={!canManageOrg}
-                     onClick={openJoinModal}
+                       onClick={openJoinModal}
-                     icon={<TIcon name="enter" size="16px" />}
+                       icon={<TIcon name="enter" size="16px" />}
-                   />
+                     />
+                   </span>
                  </Tooltip>


─── apps/web/src/organizations/OrganizationsPage.tsx:1955-1958 ───
[bug · medium] 删除/退出确认弹窗的「取消/删除」操作元素退化为
span+onClick：无焦点、无键盘激活（Enter/Space）、无按钮语义，而这其中「删除组织」是不可逆的破坏性操作——键盘用户在弹窗打开后无任何可达路径提交或取消，只能 Esc
关闭。与已确认的 org-card 键盘回归同属一类，但此处是独立代码位置。注：kb-list.td.css 的 del-knowledge-dialog 存在 span
先例（KnowledgeBasesPage.tsx:1615-1616），如需与先例保持 DOM 一致请同步修复两页，否则建议此处恢复 button 语义（css 需补
background/border/padding 重置）。

            <div className="circle-btn">
-             <span className="circle-btn-txt" onClick={() => setConfirmState(null)}>{t(locale, 'common.cancel')}</span>
-             <span className="circle-btn-txt confirm" onClick={() => void confirmLeaveOrDelete()}>{confirmState?.kind === 'delete' ? t(locale, 'common.delete') : t(locale, 'organization.leave')}</span>
+             <button type="button" className="circle-btn-txt" onClick={() => setConfirmState(null)}>{t(locale, 'common.cancel')}</button>
+             <button type="button" className="circle-btn-txt confirm" onClick={() => void confirmLeaveOrDelete()}>{confirmState?.kind === 'delete' ? t(locale, 'common.delete') : t(locale, 'organization.leave')}</button>
            </div>
+ /* orgs.td.css .circle-btn-txt 补充：background: transparent; border: 0; padding: 0; font: inherit; */


─── apps/web/src/organizations/OrganizationsPage.tsx:377-381 ───
[bug · low] 侧栏 collapsed 态的「全部/我创建的/我加入的」切换项与 expanded 态的 .sidebar-item（同文件下方）均为纯
div+onClick：迁移前的旧实现是 <button>（含 cursor/font 继承），现在键盘用户无法用 Tab+Enter 切换筛选，也没有
aria-current/aria-pressed 表达选中态。与已确认的卡片键盘回归同类但为独立位置；kb-list 的 ListSpaceSidebar 存在同样的 div 先例，若按先例保留
div，也至少应补 role="button" tabIndex={0} 和 onKeyDown。

-               <div
+               <button
+                 type="button"
                  className={`icon-item-labeled${selection === item.key ? ' active' : ''}`}
                  data-space-key={item.key}
+                 aria-pressed={selection === item.key}
                  onClick={() => onSelect(item.key)}
                >


─── apps/web/src/organizations/OrganizationsPage.tsx:1395-1398 ───
[bug · low] create 模式头像 emoji 选择器的 Popup 触发器是纯 div：不可聚焦、无 aria-label，鼠标专属。编辑模式同功能仍是 <button
aria-label={t('organization.avatarPickerHint')}>（本文件 wk-org-9
处），同一页面同一交互两模式语义不一致，且丢失了原实现携带的可访问名称。Popup 的 disabled={!settingsCanManage} 保留了权限门控，但键盘/读屏完全无法打开选择器。

-                                   <div className="avatar-trigger-wrap">
+                                   <button type="button" className="avatar-trigger-wrap" aria-label={t(locale, 'organization.avatarPickerHint')} disabled={!settingsCanManage}>
                                      <SpaceAvatar name={formName || '?'} avatar={formAvatar} size="medium" />
                                      {settingsCanManage ? <span className="avatar-change-hint">{t(locale, 'organization.avatar')}</span> : null}
-                                   </div>
+                                   </button>


─── apps/web/src/organizations/org-u.css:928-935 ───
[bug · low] .wk-org-67 的 max-height: calc(90vh-120px) 是无效声明：CSS calc 要求 + 和 - 两侧有空白，解析失败后整条
max-height 被静默丢弃，加入弹窗正文区实际没有高度上限（仅靠父级 .wk-org-122 的 max-height:90vh 和 flex 布局兜底，footer 不会被顶出）。原
Tailwind 源 max-h-[calc(90vh-120px)] 同样无效，属忠实平移了 bug——既然本文件是新写产物，应顺手修正为合法 calc。

  .wk-org-67 {
-   max-height: calc(90vh-120px);
+   max-height: calc(90vh - 120px);
    min-height: 0;
    overflow-x: hidden;
    overflow-y: auto;
    padding-inline: 24px;
    padding-top: 20px;
  }


─── apps/web/src/organizations/orgs.td.css:827-834 ───
[maintainability · low] 本块 .card-header 的 margin-bottom: 8px 与前文 .org-list-container .org-card
.card-header 的 margin-bottom: 6px 冲突：后者特异性 (0,3,0) 更高，页面上所有卡片头部实际生效 6px，本块 8px 是永不生效的死声明。同理
.card-content 的 margin-bottom: 8px 和 .card-bottom 的 padding-top: 8px 也分别被 .org-card 前缀版本的 6px
遮蔽。两套数值并存会误导后续维护者推断实际间距，建议删除被遮蔽声明或合并为一条规则并注明取舍。

  .org-list-container .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
-   margin-bottom: 8px;
    position: relative;
    z-index: 2;
  }
+ /* margin-bottom 统一由 .org-list-container .org-card .card-header 的 6px 承载 */


─── apps/web/src/organizations/OrganizationsPage.tsx:14-14 ───
[maintainability · low] FormEvent 具名导入未被使用：文件内
submitCreate/submitBasic/submitUpgradeRequest（741/760/952 行）仍写的是 React.FormEvent（依赖 UMD 全局命名空间），导致本行
FormEvent 成为死导入。要么删掉，要么顺手把三处 React.FormEvent 改用该导入以统一风格。

- import type { CSSProperties, FormEvent, MouseEvent as ReactMouseEvent } from 'react';
+ import type { CSSProperties, MouseEvent as ReactMouseEvent } from 'react';


─── apps/web/src/apps/AppsPages.tsx:96-96 ───
[bug · medium] PageFrame 的刷新按钮未传 variant：tdesign-react Button 默认
variant="base"（主题色实心填充、白字），wk-apps-8 仅覆盖了尺寸/边框色/文字色（rgba(0,0,0,0.9) 黑字）而未覆盖背景色，实际渲染为「主题绿底 + 黑字 +
灰边」，对比度差，且与被替换代码注释声明的 "Vue's medium outline button"（白底黑字灰边）视觉回归。同文件 CatalogPage/ConnectionsPage
的同款刷新按钮（155/243 行）均显式传了 variant="outline"，此处应保持一致。

-   return <main className={`wk-page wk-apps-26 ${gapClass}`}><header className="wk-apps-5"><div><h1 className="wk-apps-6">{title}</h1><p className="wk-apps-7">{description}</p></div><TButton aria-label={refreshLabel} disabled={loading} onClick={onReload} loading={loading} className="wk-apps-8"><IconRefresh size={14} />{refreshLabel}</TButton></header>{loading ? <div role="status"><Status>{loadingLabel}</Status></div> : null}{children}</main>;
+   return <main className={`wk-page wk-apps-26 ${gapClass}`}><header className="wk-apps-5"><div><h1 className="wk-apps-6">{title}</h1><p className="wk-apps-7">{description}</p></div><TButton variant="outline" aria-label={refreshLabel} disabled={loading} onClick={onReload} loading={loading} className="wk-apps-8"><IconRefresh size={14} />{refreshLabel}</TButton></header>{loading ? <div role="status"><Status>{loadingLabel}</Status></div> : null}{children}</main>;


─── apps/web/src/apps/AppsPages.tsx:69-69 ───
[maintainability · medium] 死代码：ConnectionsPage 改用 TPopconfirm（tdesign-react）后，本地自定义 Popconfirm 组件（第
67-90 行）在全文件已无任何 JSX 调用点，应连同其独占的 wk-apps-1~4 样式（apps-u.css）一并删除，避免维护者误以为该气泡仍是撤销确认的生效实现。



─── apps/web/src/apps/apps-u.css:5-8 ───
[maintainability · medium] wk-apps-1~4 四条规则仅服务于 AppsPages.tsx 中已无调用点的本地 Popconfirm 死代码（该页撤销确认已改用
TPopconfirm），应连带删除。



─── apps/web/src/apps/AppsPages.tsx:370-370 ───
[maintainability · low] 风险级→主题映射此处仍用三重嵌套三元（违反嵌套三元禁令），且与同一变更新增的 riskThemeOf()（第 131 行，catalog
列已在用）逻辑完全重复，后续调整映射时易漏改，建议直接复用 riskThemeOf(riskValue)。

-       <div><dt className="wk-apps-11">{t('apps.actions.riskLabel')}</dt><dd className="wk-apps-13" title={riskLabel ? undefined : t('apps.actions.riskUnknownHint')}>{riskLabel ? <Tag theme={riskValue === 'read' ? 'success' : riskValue === 'write' ? 'warning' : riskValue === 'send' || riskValue === 'delete' ? 'danger' : 'default'}>{riskLabel}</Tag> : <span className="wk-apps-16">—</span>}</dd></div>
+       <div><dt className="wk-apps-11">{t('apps.actions.riskLabel')}</dt><dd className="wk-apps-13" title={riskLabel ? undefined : t('apps.actions.riskUnknownHint')}>{riskLabel ? <Tag theme={riskThemeOf(riskValue)}>{riskLabel}</Tag> : <span className="wk-apps-16">—</span>}</dd></div>


─── apps/web/src/auth/login.td.css:82-86 ───
[bug · medium] `.login-layout` 的 `min-height: 100%` 在 React 端高度链断裂：Vue 事实源中该值生效依赖
frontend/src/App.vue:292-296 的 `body, html, #app { height: 100% }`，而 apps/web 的 styles.css body
规则只复刻了该块的字体/字号/颜色（styles.css:19-33），并未迁移 height；#root、html 也无任何高度设定（全库 CSS 检索无命中）。containing block
高度为 auto 时百分比 min-height 按 0 处理，容器高度退化为纯内容高度。旧实现用的是
`min-h-screen`（100vh，与父链无关），迁移后在高于内容高度的视口（1440p+、外接显示器）上渐变背景与动态装饰背景只覆盖内容区，页面底部露出 body 背景（浅色
#eee），为确定性视觉回退。建议改为视口单位（`min-height: 100vh`，或考虑移动端地址栏用 `100dvh`），或在 styles.css 补齐 Vue 侧等价的高度链。

  .login-layout {
    display: flex;
    width: 100%;
-   min-height: 100%;
+   min-height: 100vh;
    overflow: hidden;


─── apps/web/src/auth/LoginPage.tsx:435-435 ───
[bug · low] 表单改为非受控后，邀请链路存在用户输入丢失的时序窗口：`inviteMode = inviteToken && invite ?
landingModeForInvite(...) : mode` 在 `lookupInvitation` 异步解析（LoginPage.tsx:142-150）后翻转（如 /join?token=
首帧渲染登录卡、解析后翻到注册卡），两卡互斥渲染导致 Form 整体重挂载、输入框以 `initialData=""` 呈现空值。旧实现输入框受控绑定
`value={email}`，翻转后已输入/浏览器 autofill 的邮箱密码仍会回显到新卡。更隐蔽的是 state 与显示值自此错位：submit 读取的
state.email/password 仍保留旧值，注册卡却显示为空。建议在表单挂载时以 state 值作 initialData（如 `initialData={email}`），或翻转
inviteMode 时显式清空对应 state，保证显示值与提交值一致。

- <Form.FormItem label={t('auth.email')} name="email" requiredMark initialData="">
+ <Form.FormItem label={t('auth.email')} name="email" requiredMark initialData={email}>


─── apps/web/src/auth/LoginPage.tsx:378-378 ───
[other · low] 语言菜单项由原生 `<button role="menuitem">` 改为 `<div role="menuitem" tabIndex={0}>` 且仅处理
Enter：div 非原生可激活元素，按 Space 不会触发 click，键盘用户无法用空格选择语言（WAI-ARIA menuitem 惯例还需方向键导航）。建议保留 button 元素或至少补
`event.key === ' '` 分支。

- <div key={option.value} role="menuitem" tabIndex={0} className={`language-option${option.value === locale ? ' active' : ''}`} onClick={() => selectLanguage(option.value)} onKeyDown={(event) => { if (event.key === 'Enter') selectLanguage(option.value); }}>
+ onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); selectLanguage(option.value); } }}


─── apps/web/src/auth/LoginPage.tsx:413-413 ───
[other · low] 轮播分页圆点由旧实现的 `<button aria-label onClick>` 改为不可键盘聚焦的 `<span
onClick>`，键盘用户失去切换幻灯片的途径（span 无 tabIndex，Tab 无法到达）。虽为 Vue swiper 原生结构的复刻，但相对旧 React 实现是可达性回退。建议补
`tabIndex={0}` + `role="button"` + Enter/Space 处理，或恢复 button 元素（样式层面 button 可重置为外观一致）。

- <span key={slide.titleKey} className={`swiper-pagination-bullet${index === slideIndex ? ' swiper-pagination-bullet-active' : ''}`} aria-label={t(slide.titleKey)} title={t(slide.titleKey)} onClick={() => setSlideIndex(index)} />
+ <span key={slide.titleKey} role="button" tabIndex={0} className={`swiper-pagination-bullet${index === slideIndex ? ' swiper-pagination-bullet-active' : ''}`} aria-label={t(slide.titleKey)} title={t(slide.titleKey)} onClick={() => setSlideIndex(index)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSlideIndex(index); } }} />


─── apps/web/src/auth/WorkspaceOnboardingPage.tsx:171-176 ───
[bug · low] 邀请列表弹窗相对旧实现丢掉了 `closeLabel={msg(locale, 'auth.workspaceOnboarding.close')}`：WkDialog
的关闭按钮 aria-label 取 closeLabel，缺省值为英文 'Close'（wk-legacy.tsx:35），中文及其它 locale
下读屏用户听到的不再是本地化文案。同页第一个弹窗本来就没传 closeLabel，此处是迁移中意外移除的本地化回退，建议补回。

  <TDialog
        open={invitationsVisible}
        title={msg(locale, 'auth.workspaceOnboarding.invitations')}
        onClose={() => setInvitationsVisible(false)}
+       closeLabel={msg(locale, 'auth.workspaceOnboarding.close')}
        className="wk-onb-11"
      >


─── apps/web/src/configuration/ConfigurationOperations.tsx:64-64 ───
[bug · medium] tdesign-react 的 Input 只读属性名是全小写 `readonly`（与 Vue 端 tdesign-vue-next 一致，见 InputProps
定义），JSX 属性大小写敏感，驼峰 `readOnly` 不会被组件识别、也不会透传到内层 input。迁移前 @weknora/ui 的 Input 会把 readOnly 透传给原生 input
生效；迁移后该 "Share permission" 展示字段变为可编辑，且受控 value 无 onChange 还会触发 React 受控组件警告（输入回弹）。建议改为全小写
`readonly`。

-   return <Card className="wk-configuration-operations"><h2>Agent sharing and visibility</h2><p className="wk-muted wk-cfg-ops-1">Select an agent here for sharing or hide/show operations. To choose an agent for a conversation, use the Agent selector in the chat entry. Shared agents are read-only in the receiving workspace, so every share uses Viewer permission. Sharing and hide/show preferences remain server-authorized.</p>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}<div className="wk-form-grid wk-form-grid--two wk-cfg-ops-2"><label>Selected agent<Select value={selected} onChange={(value) => setSelected(String(value))}><Select.Option value="" label="No agent">No agent</Select.Option>{agents.map((agent) => <Select.Option key={agent.id} value={agent.id} label={`${agent.name}${disabledIds.includes(agent.id) ? ' · disabled' : ''}`}>{agent.name}{disabledIds.includes(agent.id) ? ' · disabled' : ''}</Select.Option>)}</Select></label><label>Organization ID<Input value={organizationId} onChange={(value) => setOrganizationId(String(value))} placeholder="org_…" /></label><label>Share permission<Input value="Viewer (read-only)" readOnly /></label></div><div className="wk-list-actions wk-cfg-ops-3"><Button type="button" theme="default" variant="outline" disabled={busy || !selected} onClick={() => void setDisabled(!selectedDisabled)}>{selectedDisabled ? 'Show selected agent' : 'Hide selected agent'}</Button><Button type="button" theme="default" variant="outline" loading={busy} disabled={!selected || !organizationId.trim()} onClick={() => void share()}>Share agent</Button></div></Card>;
+ <label>Share permission<Input value="Viewer (read-only)" readonly /></label>


─── apps/web/src/configuration/config-u.css:44-49 ───
[bug · medium] `.wk-cfg-ops-4 input` / `.wk-cfg-ops-4 textarea` 是旧栈"裸 input/textarea + Tailwind
`[&_input]:…`"的逐字平移，但该类现在挂在含 tdesign 控件的 form 上（ModelDebugPanel、SkillOperations 的 `<form
className="wk-settings-editor wk-cfg-ops-4">`）。tdesign Input 与 Select 的内层
`input.t-input__inner`、Textarea 的内层 textarea 都会被此后代选择器命中，在这些元素上叠加 1px #cbd5e1 边框、.65rem/.55rem
内边距与白底——与 `t-input` / `t-textarea` wrapper 自身的边框形成双重边框和内边距错乱，并非"视觉零变化"。这两段规则实际只需覆盖 ModelDebugPanel
中唯一的原生控件（file input），建议删除这两段并给 file input 单独的显式类名，其余交给 tdesign 默认样式。

- .wk-cfg-ops-4 input {
+ /* 删除 .wk-cfg-ops-4 input / .wk-cfg-ops-4 textarea 两段； */
+ /* 若 file input 需保持原样式，改用显式类： */
+ .wk-cfg-file {
    width: 100%;
    box-sizing: border-box;
-   border-style: solid;
-   border-width: 1px;
-   border-color: #cbd5e1;
+   border: 1px solid #cbd5e1;
+   border-radius: 6px;
+   background-color: #fff;
+   padding-inline: .65rem;
+   padding-block: .55rem;
+ }


─── apps/web/src/configuration/config-u.css:62-67 ───
[bug · medium] 同上一条：`.wk-cfg-ops-4 textarea` 与 `.wk-cfg-ops-4 input` 一样会命中 tdesign Textarea 的内层
textarea（wrapper 已有边框），造成双重边框；tdesign Select 内部的 `input.t-input__inner`（单选显示框）同样被 input 规则命中。两段约 15
行的属性逐字重复本身也已在既有反馈中提及，这里的问题是选择器作用域在 tdesign 迁移后已不再正确。



─── apps/web/src/faq/FAQPage.tsx:789-790 ───
[maintainability · medium] 死属性：`message` 与 `editorTitle` 两个 props 在本次平移后已无任何使用（内联 <Status>
渲染已删、错误提示改由 FAQPage 层 MessagePlugin toast 展示，编辑抽屉标题已内联为 editorMode
三元计算），但接口声明（:698/:693）、默认解构与父组件传参（:2466/:2461）仍在保留，形成接口与实现不一致的死代码。建议连同 interface
字段、父组件传参一并删除，或恢复视图内消费点。



─── apps/web/src/faq/FAQPage.tsx:1653-1658 ───
[maintainability · low] 导入弹窗主按钮的状态分支不可达：`onOpenImport`（:2500）每次打开弹窗都会 `setImportTask(null)`，而
`confirmImport` 成功即 `setImportOpen(false)`（弹窗关闭后任务态由 header 的 faq-import-strip 展示），失败路径从不设置
importTask——因此弹窗打开期间 `importTask` 恒为 null，`status === 'success'`（关闭文案）、`'failed'`（重试文案）与
`disabled={importTask?.status === 'running'}` 永不生效，`loading={importBusy && !importTask}` 退化为恒等于
importBusy。这些条件既属死逻辑，语义上也与 onClick 固定调用 onImportConfirm
不配套（若未来放开重置，成功态点「关闭」会再次发起导入）。建议：要么删除三态文案改为恒定导入文案+`loading={importBusy}`，要么让 onClick
按状态分流（成功→onCloseImport、失败→重新导入）。



─── apps/web/src/faq/FAQPage.tsx:1220-1226 ───
[other · medium] 键盘可达性回归：卡片选择现在仅靠 <div onClick>（无 role、无 tabIndex、无键盘事件），标签筛选的清除控件同为 span
onClick；被删除的旧实现为卡片提供隐藏 Checkbox（label+input），清除按钮带 role="button" tabIndex={0}
onKeyDown。canSelectEntries（含 canManage 只读管理者）用户无法用键盘完成选择/清除操作。建议恢复键盘语义：卡片容器补
role="button"/tabIndex={0}/onKeyDown（Enter/Space 触发 onToggleSelect），清除 span 改用 <button
type="button">。



─── apps/web/src/faq/FAQPage.tsx:989-991 ───
[maintainability · low] 死分支：`case 'export'` 无任何来源——faqExportOptions 只产生
'export_csv'/'export_json'，页面内也无其他调用方传入 'export'。建议删除该行，避免误导后续维护者以为存在第三种导出入口。



─── apps/web/src/faq/faq.td.css:3151-3154 ───
[maintainability · low] §C
共享弹层类（.kb-switcher-card、.kb-switcher-row*、.card-more-popup、.popup-menu-item 等）与 documents.td.css /
kb-list.td.css 中的同名全局规则重复定义（值目前一致）。这些类无页面前缀、随 bundle 全局生效，加载顺序决定最终值，两处维护漂移会互相污染 documents/KB
列表域。既然注释标明「与 documents.td.css 同源平移」，建议抽成单一共享样式层（或至少在此注明上游文件与同步责任人），避免三份拷贝各自演化。



─── apps/web/src/faq/faq.td.css:2240-2244 ───
[bug · low] `.answer-tag` 用 `var(--td-brand-color)1a` / `...33` 的文本拼接方式追加 alpha，仅当 token 恰为 6 位 hex
时才生成合法颜色（当前 tdesign-theme.css 中 #07c05f 可工作）；token 一旦改为 rgb()/hsl() 或 8 位 hex 即静默失效。建议改用
color-mix(in srgb, var(--td-brand-color) 10%, transparent)（本文件 tag-filter-chip 已用该写法），与既有模式保持一致。

  .answer-tag {
-   background: var(--td-brand-color)1a;
+   background: color-mix(in srgb, var(--td-brand-color) 10%, transparent);
    color: var(--td-brand-color);
-   border-color: var(--td-brand-color)33;
+   border-color: color-mix(in srgb, var(--td-brand-color) 20%, transparent);
  }


─── apps/web/src/shared/wk-legacy.tsx:82-83 ───
[bug · high] WkDialog 的 openDialogStack 清理逻辑实际永远不会生效：cleanup 里读取的是 dialogRef.current，但 open 由
true→false 时组件重渲染返回 null，<section> 在 commit 的 mutation 阶段被卸载、ref 同步被 React 置回 null，随后才执行 passive
effect cleanup——此时 dialogRef.current 恒为 null，indexOf 恒为 -1，栈中的元素永远不会被 splice 移除。后果：(1) 模块级
openDialogStack 只增不减，长时间会话持续泄漏已分离的 DOM 节点；(2)
栈顶仲裁失效——层叠弹窗场景（如设置弹层内再打开分享/确认弹层）关闭内层弹窗后，残留的栈顶元素会挡住外层弹窗的 Escape 处理，外层弹窗无法用 Esc 关闭。建议在 effect
体内捕获元素引用，供 cleanup 使用（与 onCloseRef 同理）。

-       const index = dialogRef.current ? openDialogStack.indexOf(dialogRef.current) : -1;
+   useEffect(() => {
+     if (!open) return;
+     // cleanup 执行时 ref 已被 React 置空（open→false 重渲染 return null），须在 effect 内捕获
+     const element = dialogRef.current;
+     if (element) openDialogStack.push(element);
+     restoreRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
+     element?.focus();
+     // ...
+     return () => {
+       document.removeEventListener('keydown', onKeyDown);
+       const index = element ? openDialogStack.indexOf(element) : -1;
        if (index >= 0) openDialogStack.splice(index, 1);
+       restoreRef.current?.focus();
+       restoreRef.current = null;
+     };
+   }, [open]);


─── apps/web/src/shared/wk-legacy.tsx:149-152 ───
[bug · low] WkSheet 拖宽过程中若组件被卸载（resizeRef.current 仍非 null），mousemove/mouseup 监听虽被移除，但 beginResize
设置的 document.body.style.cursor='col-resize' 与 userSelect='none' 不会被复位，body 光标会卡在 col-resize
且无法选中文本，直到下次完整拖放。注意不能直接在本 cleanup 里重置——该 effect 依赖 panelWidth，拖动过程中每次宽度变化都会
teardown/re-add，直接重置会打断拖拽。建议补一个仅卸载时执行的 effect 做兜底复位。

+   // 卸载兜底：拖宽中途卸载时复位 body 光标/选择样式（resize effect 的 cleanup 会随 panelWidth 变化频繁执行，不宜放在那里）
+   useEffect(() => {
      return () => {
-       window.removeEventListener('mousemove', move);
-       window.removeEventListener('mouseup', stop);
+       if (resizeRef.current) {
+         resizeRef.current = null;
+         document.body.style.cursor = '';
+         document.body.style.userSelect = '';
+       }
      };
+   }, []);


─── apps/web/src/wiki/WikiPage.tsx:1224-1225 ───
[bug · low] 新增的 clearable 清除按钮只重置了搜索草稿：onClear 里 setSearchDraft("") 之后 keyword
仍保持旧值，侧栏列表仍按旧关键字过滤，而输入框已显示为空，二者状态不一致，直到用户再按一次 Enter 才恢复。注意此处不能直接调 submitSearch()——它读取的 searchDraft
状态在本轮仍是旧值。建议清空时同步重置 keyword 与分页。

                  onEnter={() => submitSearch()}
-                 onClear={() => setSearchDraft("")}
+                 onClear={() => { setSearchDraft(""); setKeyword(""); setPage(1); }}


─── apps/web/src/shared/shared-u.css:29-33 ───
[style · low] 生成器产物残留：.wk-shared-4 / .wk-shared-8 / .wk-shared-13 / .wk-shared-15 均出现重复的
border-style: solid; 声明（同一规则内连续两次）。虽然无功能影响，但与文件头「值 = utilities 编码的生效值」的约定相悖，容易误导后续手工维护，建议清理重复声明。

    border-radius: 6px;
    border-style: solid;
    border-width: 1px;
-   border-style: solid;
    border-color: #dcdcdc;


─── apps/web/src/wiki/wiki-u.css:188-192 ───
[style · low] 同类残留：.wk-wiki-17 中 font-size: 13px 紧跟 font-size: inherit（后者生效，前者是死代码），.wk-wiki-63 中
font-weight: inherit 紧跟 font-weight: 650 !important 同理。既然文件头声明「值 = utilities
编码的生效值」，这些被覆盖的声明应删除，否则维护者会误以为 13px/inherit 是两个并存的意图。

    padding-inline: 8px;
    padding-block: 4px;
-   font-size: 13px;
    font-family: inherit;
    font-size: inherit;


─── packages/views/src/craft/shell.tsx:100-100 ───
[bug · medium] CraftDrawer 的 JSDoc 承诺「full width under 760px via CSS」，但该行为实际不存在：全仓库（craft.css 及
apps/web 各样式文件）没有任何 `.wk-craft-drawer` 规则，而 `className="wk-craft-drawer"` 只落在了 t-drawer 根节点上；同时
td.tsx 的 Drawer 在 `.t-drawer__content-wrapper` 上写死了内联 `style={{ width: '460px'
}}`（td.tsx:164），内联样式优先级高于普通 CSS 规则，普通媒体查询根本无法覆盖。结果视口窄于 460px（如 375px 手机）时抽屉固定 460px 宽横向溢出。建议在
craft.css 补一条需 `!important` 的窄屏规则（或改由 CSS 变量承载宽度），否则应修正此处注释避免误导。

-     <Drawer open={open} title={title} onClose={onClose} size="460px" placement="right" className="wk-craft-drawer">
+ /* craft.css */
+ @media (max-width: 760px) {
+   .wk-craft-drawer .t-drawer__content-wrapper { width: 100% !important; }
+ }


─── packages/views/src/craft/interaction-card.tsx:10-10 ───
[test · medium] 切换到 td Button 后，按钮文本被包进 `<span class="t-button__text">`（td.tsx:84-86），而配套测试
interaction-card.test.tsx:84 仍用 `/<button[^>]*>[^<]*<\/button>/g` 统计按钮——该正则永远无法匹配 td Button
的输出（button 开标签后紧跟
`<span`），「终态卡不暴露任何决策控件」（本文件头部声明的红线：迟到审批不能复活已取消的请求）这一不变量就此失去自动化护栏，断言恒真。本次更新未同步修改该测试（对照
versions.test.tsx:56-58 已为同一 span 包裹结构适配了正则）。建议同步更新测试断言，改为按 `<button` 开标签存在性检测。

- import { Button } from './td.tsx';
+ // interaction-card.test.tsx:84
+ const buttons = markup.match(/<button[\s>]/g) ?? [];


─── apps/web/src/administration/AdministrationPage.tsx:0-0 ───
[bug · medium] 邀请邮箱输入框由旧 `<Input type="email" required>` 迁移为 TDesign `TInput`
后，丢失了浏览器原生的邮箱格式与必填校验（TInput 未透传 type="email"/required，且其 type 枚举不含 'email'）。而 `onInviteFormSubmit` 与
`sendInvitation` 目前只做 `email.trim()` 非空判断，任意非空字符串（如 "abc"）都能通过表单进入确认步骤并直接调用
`identity.tenants.invitations.create`，形成校验回退。建议在提交/发送前补充邮箱格式校验（正则或 TDesign Form rules），保持迁移前的前端校验语义。

- <TInput className="wk-admin-invite-email" value={email} onChange={(value) => setEmail(String(value))} />
+ <TInput className="wk-admin-invite-email" value={email} onChange={(value) => setEmail(String(value))} status={!EMAIL_PATTERN.test(email) && email !== '' ? 'error' : undefined} />
+ // 并在 onInviteFormSubmit / sendInvitation 中：
+ // if (!manageTenant || !EMAIL_PATTERN.test(email.trim())) return;


─── apps/web/src/administration/administration-u.css:18-23 ───
[maintainability · low] `.wk-admin-pager-btn` 中的 `font-weight: 400`（对应原 utility
`font-normal`）会被其后展开的 `font-weight: inherit`（对应原 `[font:inherit]`
简写的生效值）无条件覆盖，属永不生效的冗余声明，建议删除以免误导后续维护者对生效值的判断。

-   font-weight: 400;
    color: rgb(0 0 0/90%);
    font-family: inherit;
    font-size: inherit;
    line-height: inherit;
    font-weight: inherit;


─── apps/web/src/agent-marketplace/am-u.css:126-132 ───
[maintainability · low] accent 背景硬编码为 `#07c05f`，而同批迁移的
administration-u.css（.wk-admin-pager-current）统一使用 `var(--color-accent, #07c05f)` 引用主题
token（styles.css :root 定义）。一旦 `--color-accent` 被主题定制或覆盖，此处不会跟随变化，且两种写法不一致。建议改为 `background-color:
var(--color-accent, #07c05f);`（同文件 `.wk-ava-11` 的 `#07c05f`、`.wk-amr-7` 的 `#ffffff`（原
bg-surface）同理）。

  .wk-amr-18 {
    border-radius: 4px;
-   background-color: #07c05f;
+   background-color: var(--color-accent, #07c05f);
    padding-inline: 12px;
    padding-block: 6px;
    color: #fff;
  }


─── apps/web/src/agent-marketplace/am-u.css:216-222 ───
[maintainability · low] 此处 `#07c05f` 为原 `bg-accent` 的 token 生效值，建议与同批 administration-u.css
的做法保持一致，改用 `var(--color-accent, #07c05f)` 引用主题变量，避免主题定制时出现颜色失配。

  .wk-ava-11 {
    border-radius: 4px;
-   background-color: #07c05f;
+   background-color: var(--color-accent, #07c05f);
    padding-inline: 12px;
    padding-block: 6px;
    color: #fff;
  }


─── apps/web/src/apps/apps-u.css:92-96 ───
[bug · medium] `.wk-apps-11` 的颜色平移取值错误。旧代码的 `text-muted` 工具类实际生效值是 `--color-muted:
#66758b`（apps/web/src/styles.css:432），并非 rgba(0,0,0,0.6)——后者是 `--color-text-secondary`/Vue
`--td-text-color-secondary` 的值。本文件头注释自述的平移契约是「值 = utilities 编码的生效值」，且兄弟域迁移先例均按 #66758b
平移（documents-u.css:1455/2003/2280，settings.td.css:4912 注释亦明确「原 text-muted utility，--color-muted
#66758b」）。authorization/action 两个留守页的 <dt> 标签色将从蓝灰 #66758b 变为中性 60% 黑，违反本次迁移「0% 回归」的目标。建议改回
#66758b（或引用 var(--color-muted, #66758b)）。

  .wk-apps-11 {
    font-weight: 500;
-   /* 旧栈 muted/muted-strong→Vue var(--td-text-color-secondary)=rgba(0,0,0,.6) */
-   color: rgba(0, 0, 0, 0.6);
+   /* 旧栈 text-muted → --color-muted（styles.css:432），值 = utility 生效值 */
+   color: var(--color-muted, #66758b);
  }


─── apps/web/src/apps/AppsPages.tsx:226-228 ───
[maintainability · low] TPopconfirm 显式指定 `placement="left"` 偏离了事实源：ConnectionsView.vue:61-67 的
t-popconfirm 未传 placement（默认 top），而本函数头注释声明的是「ConnectionsView.vue DOM 1:1 / t-popconfirm
直译」。本文件中其它刻意偏差（如 maxHeight 空态省略）均有台账注释说明缘由，此处却无任何说明，后续维护者无法判断这是有意调整（如避免右缘溢出）还是误植，也容易被 Vue
对照回归时当作差异误报。建议要么对齐 Vue 默认（删除 placement），要么补注释说明为何选择 left。



─── apps/web/src/administration/administration-u.css:52-59 ───
[maintainability · medium] `.wk-admin-3` 将原 `text-primary` 的生效值硬编码为 `#2e6de6`，而 styles.css :root
单源定义了 `--color-primary: #2e6de6`（design-tokens 测试还断言 primary 保持单源）。同批迁移的 `.wk-admin-pager-current`
已经采用 `var(--color-accent, #07c05f)` 的写法，此处建议同样改为 `color: var(--color-primary,
#2e6de6);`，否则主题定制/换肤时页眉 eyebrow 色不会跟随变化。

  .wk-admin-3 {
    margin: 0;
    font-size: 0.78rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
-   color: #2e6de6;
+   color: var(--color-primary, #2e6de6);
  }


─── apps/web/src/administration/administration-u.css:32-34 ───
[maintainability · medium] 分页器按钮 hover 态把原 `enabled:hover:text-accent` 硬编码为 `#07c05f`，与同一文件末尾
`.wk-admin-pager-current` 使用的 `var(--color-accent, #07c05f)` 写法不一致——一旦 `--color-accent` 被主题覆盖，hover
色与当前页高亮色会失配。建议统一为 `color: var(--color-accent, #07c05f);`（同 am-u.css 已确认问题的同批修法）。

  .wk-admin-pager-btn-disabled:enabled:hover {
-   color: #07c05f;
+   color: var(--color-accent, #07c05f);
  }


─── apps/web/src/agent-marketplace/am-u.css:97-103 ───
[bug · low] `.wk-amr-14` 对应原 utility `break-all`（Tailwind 展开为 `word-break: break-all`），但此处写成
`overflow-wrap: anywhere`（对应的是另一条 utility `wrap-anywhere`）。两者语义不同：`break-all`
会在任意字符处换行，`overflow-wrap: anywhere` 仅在溢出时才断行且影响 min-content 尺寸计算，普通单词的换行表现会有差异。迁移值应保持逐项一致，建议改为
`word-break: break-all;`（对照：`.wk-amr-11` 对 `break-words` 的 `overflow-wrap: break-word` 是正确的）。

  .wk-amr-14 {
-   overflow-wrap: anywhere;
+   word-break: break-all;
    border-radius: 4px;
    background-color: rgba(127,127,127,0.07);
    padding: 8px;
    font-size: 12px;
  }


─── apps/web/src/administration/AdministrationPage.tsx:206-207 ───
[bug · low] 跳页输入框从原生 `<Input>` 迁移到 TDesign `TInput` 时丢失了 `inputMode="numeric"`（原代码为 `type="text"
inputMode="numeric"`），移动端用户将弹出全键盘而非数字键盘，属于迁移中的属性遗漏。TInput 可透传原生属性，建议补回。

-       <TInput className="wk-admin-pager-jump wk-admin-25" type="text" value={jump}
+       <TInput className="wk-admin-pager-jump wk-admin-25" type="text" inputMode="numeric" value={jump}
          onChange={(value) => setJump(String(value))}


─── packages/views/src/craft/td.tsx:109-115 ───
[bug · medium] useOverlayFocus 将 onClose 放入依赖数组 [onClose, open]：craft 调用方传的是内联箭头函数（如 versions.tsx:98
`onClose={() => setConfirming(null)}`），每次父组件重渲染身份都会变化。弹层打开期间父组件一旦重渲染（例如 versions
列表刷新、locale/状态更新），effect 会先执行
cleanup——把焦点归还给触发元素——再重新挂载并把焦点拉回面板，用户正在弹层内交互时焦点被瞬间夺走；若此刻触发元素已被卸载（列表刷新移除该行），cleanup 中 focus()
静默失败，重挂载后 restoreRef 会误捕获为 body，最终关闭时焦点归还失效。建议用 ref 缓存 onClose，effect 依赖只保留 [open]（只在 open 生命周期的真正
teardown 时归还焦点）。

+   const onCloseRef = useRef(onClose);
+   onCloseRef.current = onClose;
+   useEffect(() => {
+     if (!open) return undefined;
+     // ...
+     const onKeyDown = (event: KeyboardEvent) => {
+       // ...
+       onCloseRef.current();
+     };
      document.addEventListener('keydown', onKeyDown);
      return () => {
        document.removeEventListener('keydown', onKeyDown);
        restoreRef.current?.focus();
        restoreRef.current = null;
      };
-   }, [onClose, open]);
+   }, [open]); // onClose 经 ref 读取，避免依赖抖动导致的焦点归还/夺走


─── packages/views/src/craft/td.tsx:101-108 ───
[bug · medium] Escape 焦点域判定存在失效场景：(1) 用户点击弹层内不可聚焦内容（正文文本、留白）后，浏览器会将 activeElement 回落为 <body>，此时
panelRef.current.contains(body) 为 false → 提前 return，Escape 从此完全失效（shell.test.tsx 只覆盖了焦点仍在面板内的用例）；(2)
监听挂在 document 上，stopPropagation 无法阻止同一 document 节点上的其它 keydown 监听器（需 stopImmediatePropagation），一旦出现
Drawer 内嵌 Dialog 的组合（两者都在 document 注册监听），一次 Escape 会同时关闭两层。建议至少将 activeElement === document.body
视为域内，嵌套场景配合弹层栈/topmost-wins 处理。

      const onKeyDown = (event: KeyboardEvent) => {
        if (event.key !== 'Escape') return;
        const active = typeof document !== 'undefined' ? document.activeElement : null;
-       if (active && panelRef.current && !panelRef.current.contains(active)) return;
+       // 点击面板内不可聚焦内容后 activeElement 回落为 body，视为仍在焦点域内
+       const inScope = active === null || active === document.body
+         || (panelRef.current !== null && panelRef.current.contains(active));
+       if (!inScope) return;
        event.preventDefault();
-       event.stopPropagation();
-       onClose();
+       onCloseRef.current();
      };


─── packages/views/src/craft/td.tsx:166-166 ───
[bug · low] 关闭钮用 div + role="button"（Dialog 里是 span），但没有 tabIndex 和键盘事件——键盘用户无法聚焦/触发关闭钮，与
role="button" 的 WAI-ARIA 契约不符（注释声称与原 Sheet 的键盘契约等价，此处实际是缺口）。建议补 tabIndex={0} 与 Enter/Space
激活，或直接复用本文件的 Button 组件（tdesign 真实 Drawer/Dialog 的关闭钮本就是 t-button）。

-         <div className="t-drawer__close-btn" role="button" aria-label={closeLabel} onClick={onClose}><CloseIcon /></div>
+         <div
+           className="t-drawer__close-btn"
+           role="button"
+           tabIndex={0}
+           aria-label={closeLabel}
+           onClick={onClose}
+           onKeyDown={(event) => {
+             if (event.key === 'Enter' || event.key === ' ') {
+               event.preventDefault();
+               onClose();
+             }
+           }}
+         ><CloseIcon /></div>


─── packages/views/src/craft/td.tsx:191-192 ───
[bug · low] 可访问性回退：(1) 外层 t-dialog__ctx 设 tabIndex={0} 在对话框之前引入一个多余的 tab stop，真实 tdesign 的容器并不聚焦；(2)
title 为非字符串 ReactNode 时 aria-label 为 undefined（Drawer 同样问题），弹层失去可访问名称；(3) aria-modal="true"
向读屏宣告模态，但实现并无焦点圈禁（Tab 仍可离开弹层到背景页），语义与行为不一致。建议移除 ctx 的 tabIndex，title 用 aria-labelledby 指向
header-content，或收敛 title 类型为 string。

-     <div className="t-dialog__ctx t-dialog__modal t-dialog__ctx--fixed" tabIndex={0}>
+     <div className="t-dialog__ctx t-dialog__modal t-dialog__ctx--fixed">
        <div className="t-dialog__mask" onClick={onClose} />


─── packages/views/src/craft/td.tsx:205-205 ───
[style · low] 静态内联样式违反仓库约定（内联 style 仅限动态值）：(1) marginLeft: 'auto' 是纯静态值，应下沉到 CSS（craft.css 或 tdesign
类），真实 tdesign 的 __close 定位由类承载；(2) CloseIcon 里 style={{ fill: 'none' }} 与 svg 的 fill="none"
属性完全重复，保留属性即可。

-               <span className="t-dialog__close" style={{ marginLeft: 'auto' }} role="button" aria-label={closeLabel} onClick={onClose}>
+               <span className="t-dialog__close" role="button" aria-label={closeLabel} onClick={onClose}>
+ /* craft.css:
+ .t-dialog .t-dialog__close { margin-left: auto; } */


─── packages/views/src/craft/td.tsx:119-122 ───
[maintainability · low] useBodyPortal 名不副实：它没有创建任何 portal，只是返回 open 的布尔门控（头注也明确 body portal
未复刻），后来者极易误以为弹层挂在 body 上。另外真实 tdesign Drawer/Dialog 挂 body 时附带 body
滚动锁定，本同构层未复刻——弹层打开期间背景页面仍可滚动（在遮罩/抽屉上滚动鼠标滚轮会带动背景），与声称的视觉/行为同构存在功能缺口。建议改名为
useOverlayMounted（或直接内联布尔），并在 open 生命周期内锁定 body 滚动。

- function useBodyPortal(open: boolean): boolean {
+ function useOverlayMounted(open: boolean): boolean {
    // 树内渲染：仅在浏览器（非 SSR 静态标记）且 open 时挂载。
    return open && typeof document !== 'undefined';
  }
+ 
+ // Drawer/Dialog 内补滚动锁定（与焦点 effect 同生命周期）：
+ // useEffect(() => {
+ //   if (!open) return undefined;
+ //   const prev = document.body.style.overflow;
+ //   document.body.style.overflow = 'hidden';
+ //   return () => { document.body.style.overflow = prev; };
+ // }, [open]);


─── apps/web/src/auth/LoginPage.tsx:434-435 ───
[bug · medium] 注册成功后的邮箱预填失效（与已确认的邀请链路时序丢输入为同根因、不同路径的独立问题）：submit 的 register
分支（LoginPage.tsx:207-215）刻意保留 email state、仅清空 username/password/confirmPassword 并 setMode('login')
切回登录卡，注释声称 "switch to login and prefill the email"（Vue
Login.vue:744-746）。但登录卡表单已改为非受控（initialData=""），isRegister 翻转使登录卡 Form 重挂载，email 输入框以 initialData
空值呈现——state 中的 email 不再回显（旧受控实现 value={email} 会回显）。每个非邀请注册用户在注册成功后都需重新手输完整邮箱，且 login-page.test.tsx
未覆盖该场景。建议登录卡 email 的 FormItem initialData 改用 state 值，使重挂载时回显预填。

              <Form labelAlign="top" layout="vertical" onValuesChange={onLoginValuesChange} onSubmit={({ e }) => { void submit(e); }}>
-               <Form.FormItem label={t('auth.email')} name="email" requiredMark initialData="">
+               <Form.FormItem label={t('auth.email')} name="email" requiredMark initialData={email}>


─── apps/web/src/wiki/wiki-u.css:773-775 ───
[bug · low] 展开/收起旋转动画回归：`.wk-wiki-78` 声明的 `transition-property: transform` 只作用于 `transform` 属性，而
`.wk-wiki-79` 用的是独立的 `rotate: 90deg` 属性——CSS transition 不会跨属性生效，文件夹 chevron 的展开/收起将由原
Tailwind（transition-transform 覆盖 transform/translate/scale/rotate）的 150ms
过渡变成瞬时跳变。建议二选一：`.wk-wiki-79` 改回 `transform: rotate(90deg)`，或把 `.wk-wiki-78` 的 transition-property 扩为
`transform, rotate`。

  .wk-wiki-79 {
-   rotate: 90deg;
+   transform: rotate(90deg);
  }


─── apps/web/src/wiki/wiki-u.css:354-356 ───
[bug · low] `.wk-shared-31`（wiki 分隔线）同样存在映射失真：原始 utilities 仅为 `my-[6px] border-t
border-[#e7e7e7]`，没有任何右侧外边距，且 `.wiki-sidebar-divider` 在全仓无 CSS 定义（已检索确认），`margin-right: 8px`
无任何来源。它使分隔线右端比原渲染缩短 8px（叠加 nav 自身的 pr-2 后距右缘 16px，原本 8px），违背文件头「值 = utilities 编码的生效值」的平移约定，建议删除该行。

  .wk-wiki-31 {
    margin-block: 6px;
-   margin-right: 8px;
+   border-top-style: solid;
+   border-top-width: 1px;
+   border-color: #e7e7e7;
+ }


─── apps/web/src/shared/shared-u.css:147-149 ───
[bug · low] break-all 的映射错误：原 Tailwind 类为 `break-all`（= `word-break: break-all`），这里平移成了
`overflow-wrap: anywhere`（对应的是另一个 utility）。二者语义不同——`word-break: break-all`
允许在任意两个字符间断行，行尾单词会被拆开；`overflow-wrap: anywhere` 仅在单词自身放不下时才折断。对分享会话的标题（第 61 行 `.wk-shared-7`
同样问题）与消息正文渲染有可观察的换行差异（长英文词/URL/连续 ASCII 串），且违背文件头「值 = utilities 编码的生效值」约定。建议改为 `word-break:
break-all;`。

    white-space: pre-wrap;
-   overflow-wrap: anywhere;
+   word-break: break-all;
    font-size: 13px;


─── packages/views/src/craft/td.tsx:148-151 ───
[bug · medium] Drawer/Dialog 在 open=false 时直接 return null，等价于 destroyOnClose 恒为 true；而真实
tdesign-react 1.18.3 的 Drawer/Dialog destroyOnClose 默认为 false（关闭时仅隐藏，DOM
与子组件状态保留、重开不重置）。头注「差异注记」披露了动画类/body portal/关闭钮 role
等差异，唯独漏掉这一条，与「逐行核对/契约等价」的声明相悖。实际后果：弹层内子组件的滚动位置、展开项、输入草稿等状态在每次关闭后全部丢失，重开重置（craft
抽屉/确认弹窗均有体感）。建议在差异注记中显式披露，或改为隐藏式渲染（display:none / 条件 className）以对齐真实库默认语义。



─── packages/views/src/craft/td.tsx:144-147 ───
[bug · low] closeLabel 默认值硬编码英文 'Close'（Dialog 中同样）。craft 域全量双语（presentation.ts 的 craftStrings 提供完整
zh/en 词表，调用方如 versions.tsx 的 Dialog 文案均按 locale 切换），但关闭钮的可访问名称不走 locale：中文 locale 下读屏用户听到的是英文
"Close"。且现有两个调用方（versions.tsx 的 Dialog、shell.tsx 的 CraftDrawer→td Drawer）都未传
closeLabel，缺口必然暴露。建议默认值接入 craft locale（如 zh 时为「关闭」），或将 closeLabel 收敛为必传 prop 由调用方随 locale 提供。



LLM retry report summary: 118 of 797 requests affected -- 40 requests failed, 4 requests cancelled, 74 requests recovered after retry

Review planning (29 requests):
- apps/web/src/administration/AdministrationPage.tsx,apps/web/src/administration/administration-u.css,apps/web/src/agent-marketplace/AgentVersionActions.tsx,apps/web/src/agent-marketplace/TenantReleaseReview.tsx,apps/web/src/agent-marketplace/am-u.css: timed out -> failed
- apps/web/src/data-sources/DataSourcesPage.tsx,apps/web/src/data-sources/data-sources-u.css,apps/web/src/data-sources/ui.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/embed/EmbedEntryPage.tsx,apps/web/src/embed/embed-u.css,apps/web/src/faq/FAQPage.tsx,apps/web/src/faq/faq.td.css,apps/web/src/market/MarketPage.tsx,apps/web/src/market/market-u.css: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/integrations/ApiPlaygroundDrawer.tsx,apps/web/src/integrations/EmbedPreviewModal.tsx,apps/web/src/integrations/IntegrationsPage.tsx,apps/web/src/integrations/IntegrationsRoutePage.tsx,apps/web/src/integrations/integrations-u.css,apps/web/src/integrations/integrations.td.css,apps/web/src/integrations/views-integrations-u.css,packages/views/src/integrations/page.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> timed out -> failed
- apps/web/src/knowledge-bases/KnowledgeBaseActivityPanel.tsx,apps/web/src/knowledge-bases/KnowledgeBaseShareDialog.tsx,apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx,apps/web/src/knowledge-bases/SharedKnowledgeBaseDrawer.tsx,apps/web/src/knowledge-bases/kb-editor-parity.css,apps/web/src/knowledge-bases/kb-list-icons.tsx,apps/web/src/knowledge-bases/kb-list.td.css,apps/web/src/knowledge-bases/kb-u.css: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- ... and 24 more

Core review (82 requests):
- apps/embed/src/EmbedApp.tsx,apps/embed/src/button.tsx,apps/embed/src/styles.css: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/miniprogram/config/index.ts,apps/miniprogram/package.json,apps/miniprogram/src/app.config.ts,apps/miniprogram/src/app.scss,apps/miniprogram/src/core/errors.ts,apps/miniprogram/src/core/routes.ts,apps/miniprogram/src/features/home/pages.tsx,apps/miniprogram/tests/assembly.test.mjs,apps/miniprogram/tests/build-output.test.mjs,apps/miniprogram/tests/core.test.mjs: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/miniprogram/src/adapters/career-platform.ts,apps/miniprogram/src/career/rules-usage-reminders.config.ts,apps/miniprogram/src/career/rules-usage-reminders.tsx,apps/miniprogram/src/services/career-intent.ts,apps/miniprogram/src/services/career.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/miniprogram/src/career/application-material.config.ts,apps/miniprogram/src/career/application-material.tsx,apps/miniprogram/src/career/discovery.config.ts,apps/miniprogram/src/career/discovery.tsx,apps/miniprogram/src/career/export-deletion.config.ts,apps/miniprogram/src/career/export-deletion.gating.ts,apps/miniprogram/src/career/export-deletion.tsx,apps/miniprogram/src/career/progress-preparation.config.ts,apps/miniprogram/src/career/progress-preparation.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/miniprogram/tests/application-material.test.mjs,apps/miniprogram/tests/career-discovery.test.mjs,apps/miniprogram/tests/career-platform.test.mjs,apps/miniprogram/tests/export-deletion.test.mjs,apps/miniprogram/tests/helpers/taro-stub.mjs,apps/miniprogram/tests/live/quota-inject-proxy.mjs,apps/miniprogram/tests/live/t24r1-live-driver.cjs,apps/miniprogram/tests/progress-preparation.test.mjs,apps/miniprogram/tests/rules-usage-reminders.test.mjs: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- ... and 77 more

Context compaction (6 requests):
- apps/web/src/settings/ModelDebugPanel.tsx,apps/web/src/settings/ModelOptionSelect.tsx,apps/web/src/settings/ModelSelector.tsx,apps/web/src/settings/ModelSettingsPanel.tsx,apps/web/src/settings/OllamaSettingsPanel.tsx,apps/web/src/settings/ParserEngineSettingsPanel.tsx,apps/web/src/settings/PersonalMemoryPanel.tsx,apps/web/src/settings/PersonalMemorySettingsPanel.tsx,apps/web/src/settings/PlatformApiKeysPanel.tsx,apps/web/src/settings/PortedSectionsPanel.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/miniprogram/tests/application-material.test.mjs,apps/miniprogram/tests/career-discovery.test.mjs,apps/miniprogram/tests/career-platform.test.mjs,apps/miniprogram/tests/export-deletion.test.mjs,apps/miniprogram/tests/helpers/taro-stub.mjs,apps/miniprogram/tests/live/quota-inject-proxy.mjs,apps/miniprogram/tests/live/t24r1-live-driver.cjs,apps/miniprogram/tests/progress-preparation.test.mjs,apps/miniprogram/tests/rules-usage-reminders.test.mjs: cancelled
- apps/web/src/agents/AgentEditorModal.tsx,apps/web/src/agents/AgentParserRules.tsx,apps/web/src/agents/AgentsPage.tsx,apps/web/src/agents/MbtiTestModal.tsx,apps/web/src/agents/PersonaSection.tsx,apps/web/src/agents/SubagentsSection.tsx,apps/web/src/agents/agents-u.css,apps/web/src/agents/agents.css,apps/web/src/agents/agents.td.css,apps/web/src/agents/list.ts: cancelled
- apps/web/src/documents/DocumentsPage.tsx,apps/web/src/documents/DocumentsPageChrome.tsx,apps/web/src/documents/KnowledgeDocumentDetailPage.tsx,apps/web/src/documents/KnowledgeDocumentsPage.tsx,apps/web/src/documents/TagPickerDialog.tsx,apps/web/src/documents/UploadConfirmDialog.tsx,apps/web/src/documents/documents-list.css,apps/web/src/documents/documents-u.css,apps/web/src/documents/documents.td.css,apps/web/src/documents/preview.ts: cancelled
- apps/web/src/settings/QueryHistoryPanel.tsx,apps/web/src/settings/ResourceSettingsPanel.tsx,apps/web/src/settings/RuntimeQueuesPanel.tsx,apps/web/src/settings/SandboxSettingsPanel.tsx,apps/web/src/settings/SettingsPage.tsx,apps/web/src/settings/SkillSettingsPanel.tsx,apps/web/src/settings/SystemAuditLogPanel.tsx,apps/web/src/settings/SystemGlobalSettingsPanel.tsx,apps/web/src/settings/SystemInfoPanel.tsx,apps/web/src/settings/TenantAuditDrawer.tsx: cancelled
- ... and 1 more

Comment filtering (1 request):
- apps/web/src/knowledge-settings/GraphSettings.tsx,apps/web/src/knowledge-settings/KBShareSettingsSection.tsx,apps/web/src/knowledge-settings/KnowledgeSettingsPage.css,apps/web/src/knowledge-settings/KnowledgeSettingsPage.tsx,apps/web/src/knowledge-settings/knowledge-settings-u.css,apps/web/src/knowledge-settings/parserSettings.tsx,apps/web/src/knowledge/KnowledgeGraphPage.tsx,apps/web/src/knowledge/knowledge-u.css,apps/web/src/knowledge/permissions.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed

Per-attempt detail: --format json (retry_report).
