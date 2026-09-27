Review partially complete: 369 finding(s); 40 of 319 selected item(s) failed.

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


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:96-97 ───
[test · medium] 「进入工作空间」是后续所有 career 数据断言的前置步骤，但其 tap 失败（entered === false）只走 log、既不 record FAIL 也不
throw，与脚本自身的失败处理标准不一致（consent 步骤注释明确「tap 超时即 throw，不允许把根因埋进后续步骤」，且 finally 中按 results 判定退出码）。若该 tap
失败而页面文案恰好仍含「求职工作台」，record('login') 可能误判 PASS，后续 career-page/apply-material-data 的 FAIL
会掩盖真实根因（工作空间未进入），证据评审者从 RESULT JSON 无法定位。建议与 consent 同样处理：record 后失败即 throw。

        const entered = await tapText(page, '进入工作空间', 12000);
-       log('enter workspace', entered);
+       record('enter-workspace', entered, entered ? 'entered workspace' : 'enter-workspace button not reachable');
+       if (!entered) throw new Error('enter-workspace button not reachable');


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


─── apps/miniprogram/src/services/workbench.ts:27-27 ───
[bug · low] parsed.origin 与 apiOrigin 的全等比较存在默认端口规范化差异：new URL('https://host:443/x').origin 会去掉默认端口
443，而 normalizeApiOrigin（auth.ts）只做小写化和去尾斜杠、保留 ':443'。若 WEKNORA_API_ORIGIN 配置为带默认端口的
https://host:443（config/index.ts 的 /^https:\/\/[^/?#]+$/ 校验允许该形态），所有合法签名 URL 都会被判为
Untrusted，产物打开整体不可用。建议先用 new URL(apiOrigin).origin 做同样的规范化再比较。

-  if(!apiOrigin||parsed.origin!==apiOrigin||parsed.pathname!=='/api/v1/workbench/artifacts/download'||!parsed.searchParams.has('signature'))throw new Error('Untrusted signed artifact URL');
+  const expectedOrigin=apiOrigin?new URL(apiOrigin).origin:'';
+  if(!apiOrigin||parsed.origin!==expectedOrigin||parsed.pathname!=='/api/v1/workbench/artifacts/download'||!parsed.searchParams.has('signature'))throw new Error('Untrusted signed artifact URL');


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


─── apps/miniprogram/src/subpackages/execution/artifact/index.tsx:51-51 ───
[bug · low] run 缺失时（routeParam 返回空字符串）该按钮仍会执行 navigate('execution',{id:''})：pageUrl 的 filter 只排除
undefined，空字符串会被序列化成 `?id=`，进入执行详情页后 routeParam('id') 为空串，watchExecution('') 会向
`/api/v1/workbench/executions//snapshot` 这类畸形路径发请求，页面停留在"正在读取服务端快照"。这与顶部 Empty
的兜底（navigate('tasks')）不一致。建议 !run 时隐藏该按钮或同样跳转 tasks。

-   <Action secondary onClick={()=>void navigate('execution',{id:run})}>返回任务</Action>
+   {run?<Action secondary onClick={()=>void navigate('execution',{id:run})}>返回任务</Action>:<Action secondary onClick={()=>void navigate('tasks')}>返回任务</Action>}


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


─── apps/miniprogram/tests/application-material.test.mjs:37-37 ───
[maintainability · low] fixture 工厂 materialView 声明后未被任何用例引用（本文件没有覆盖 GET /materials/:id
完整视图解码的用例），属声明未读取的死变量。建议删除该行，或补充一个消费它的用例（如经 GET /api/v1/career/materials/mat-1 断言
versionCount/versions/status 的解码）。



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


─── apps/miniprogram/tests/career-discovery.test.mjs:61-61 ───
[maintainability · low] 谓词 ambiguous 全文无调用点（第 129 行只是测试标题字符串），属死代码；且它复刻了
src/services/career-intent.ts ambiguousOutcome 的口径，留着易让读者误以为歧义分类已被独立断言覆盖。建议删除；若想建立口径同步守护，可改为直接断言生产
ambiguousOutcome 的行为。



─── apps/miniprogram/tests/career-discovery.test.mjs:10-10 ───
[bug · low] URL→pathname→URL 双重转换在路径含空格或非 ASCII 时会解析到不存在的文件（POSIX 下 URL.pathname 已百分号编码，再经
pathToFileURL 会把 % 二次编码成 %25，如 my%20repo → my%2520repo），导致模块替换在 import 阶段整体失败；Windows 下 pathname 形如
/C:/... 同样脆弱。该写法沿用 assembly.test.mjs 既有惯例，但新文件修正成本为零。本批 application-material / career-platform /
export-deletion / progress-preparation / rules-usage-reminders 共 6 份新测试均为同一写法，建议统一改为直接取 href。

- const stubURL = pathToFileURL(new URL('./helpers/taro-stub.mjs', import.meta.url).pathname).href;
+ const stubURL = new URL('./helpers/taro-stub.mjs', import.meta.url).href;
+ // 同时可移除不再使用的 pathToFileURL 导入


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


─── apps/web/src/administration/AdministrationPage.tsx:0-0 ───
[bug · medium] 邀请邮箱输入框由旧 `<Input type="email" required>` 迁移为 TDesign `TInput`
后，丢失了浏览器原生的邮箱格式与必填校验（TInput 未透传 type="email"/required，且其 type 枚举不含 'email'）。而 `onInviteFormSubmit` 与
`sendInvitation` 目前只做 `email.trim()` 非空判断，任意非空字符串（如 "abc"）都能通过表单进入确认步骤并直接调用
`identity.tenants.invitations.create`，形成校验回退。建议在提交/发送前补充邮箱格式校验（正则或 TDesign Form rules），保持迁移前的前端校验语义。

- <TInput className="wk-admin-invite-email" value={email} onChange={(value) => setEmail(String(value))} />
+ <TInput className="wk-admin-invite-email" value={email} onChange={(value) => setEmail(String(value))} status={!EMAIL_PATTERN.test(email) && email !== '' ? 'error' : undefined} />
+ // 并在 onInviteFormSubmit / sendInvitation 中：
+ // if (!manageTenant || !EMAIL_PATTERN.test(email.trim())) return;


─── apps/web/src/administration/AdministrationPage.tsx:206-207 ───
[bug · low] 跳页输入框从原生 `<Input>` 迁移到 TDesign `TInput` 时丢失了 `inputMode="numeric"`（原代码为 `type="text"
inputMode="numeric"`），移动端用户将弹出全键盘而非数字键盘，属于迁移中的属性遗漏。TInput 可透传原生属性，建议补回。

-       <TInput className="wk-admin-pager-jump wk-admin-25" type="text" value={jump}
+       <TInput className="wk-admin-pager-jump wk-admin-25" type="text" inputMode="numeric" value={jump}
          onChange={(value) => setJump(String(value))}


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



─── apps/web/src/apps/AppsPages.tsx:370-370 ───
[maintainability · low] 风险级→主题映射此处仍用三重嵌套三元（违反嵌套三元禁令），且与同一变更新增的 riskThemeOf()（第 131 行，catalog
列已在用）逻辑完全重复，后续调整映射时易漏改，建议直接复用 riskThemeOf(riskValue)。

-       <div><dt className="wk-apps-11">{t('apps.actions.riskLabel')}</dt><dd className="wk-apps-13" title={riskLabel ? undefined : t('apps.actions.riskUnknownHint')}>{riskLabel ? <Tag theme={riskValue === 'read' ? 'success' : riskValue === 'write' ? 'warning' : riskValue === 'send' || riskValue === 'delete' ? 'danger' : 'default'}>{riskLabel}</Tag> : <span className="wk-apps-16">—</span>}</dd></div>
+       <div><dt className="wk-apps-11">{t('apps.actions.riskLabel')}</dt><dd className="wk-apps-13" title={riskLabel ? undefined : t('apps.actions.riskUnknownHint')}>{riskLabel ? <Tag theme={riskThemeOf(riskValue)}>{riskLabel}</Tag> : <span className="wk-apps-16">—</span>}</dd></div>


─── apps/web/src/apps/AppsPages.tsx:226-228 ───
[maintainability · low] TPopconfirm 显式指定 `placement="left"` 偏离了事实源：ConnectionsView.vue:61-67 的
t-popconfirm 未传 placement（默认 top），而本函数头注释声明的是「ConnectionsView.vue DOM 1:1 / t-popconfirm
直译」。本文件中其它刻意偏差（如 maxHeight 空态省略）均有台账注释说明缘由，此处却无任何说明，后续维护者无法判断这是有意调整（如避免右缘溢出）还是误植，也容易被 Vue
对照回归时当作差异误报。建议要么对齐 Vue 默认（删除 placement），要么补注释说明为何选择 left。



─── apps/web/src/apps/apps-u.css:5-8 ───
[maintainability · medium] wk-apps-1~4 四条规则仅服务于 AppsPages.tsx 中已无调用点的本地 Popconfirm 死代码（该页撤销确认已改用
TPopconfirm），应连带删除。



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


─── apps/web/src/career/progress.css:36-37 ───
[bug · medium] 同 inbox.css 的特异性问题：`.wk-progress button`（0,1,1）覆盖 `.wk-progress__submit`（0,1,0）的绿色
background/color/border-color，hover 规则（0,3,1 vs 0,3,0）同样被压制。「记录进展」主按钮（ProgressPage.tsx
L198）渲染为白底深字而非品牌绿。建议与同批文件统一修复方式：

- .wk-progress__submit { border-color: #07c05f; background: #07c05f; color: #fff; }
- .wk-progress__submit:hover:not(:disabled) { color: #fff; opacity: .9; }
+ button.wk-progress__submit { border-color: #07c05f; background: #07c05f; color: #fff; }
+ button.wk-progress__submit:hover:not(:disabled) { color: #fff; opacity: .9; }


─── apps/web/src/career/search.css:29-31 ───
[style · low] 焦点描边做法与同批表面不一致：search.css 与 rule.css 手写 rgba(7, 192, 95, .45) 半透明描边，其余 career 表面统一使用
var(--td-brand-color-focus)（主题已定义为 color-mix(in srgb, var(--td-brand-color) 20%,
transparent)），两套描边透明度不同、换主题时本文件不跟随令牌。另外 `.wk-career-search__row-note` 的回退色 #999 若真生效（主题未加载时）在白底上对比度约
2.8:1，不满足 WCAG AA 4.5:1，回退值建议至少 #767676：

  .wk-career-search textarea:focus-visible,
  .wk-career-search button:focus-visible,
- .wk-career-search a:focus-visible { outline: 3px solid rgba(7, 192, 95, .45); outline-offset: 2px; }
+ .wk-career-search a:focus-visible { outline: 3px solid var(--td-brand-color-focus, #07c05f); outline-offset: 2px; }


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


─── apps/web/src/chat/chat-header.tsx:125-126 ───
[bug · low] 守卫条件失效：`!menuMode` 恒为 false（menuMode 是 'menu' | 'clear' | 'delete' 的非空字符串联合），该检查真正想挡的
'menu' 态没有挡住——menuMode === 'menu' 时会落入 else 分支调用 `props.onDeleteSession?.()`。当前确认按钮只在弹层 confirm
分支渲染所以未实际触发，但这是一层失效的防护：后续若在 menu 态复用/暴露该函数即会误删会话。建议显式排除 'menu' 态。

    async function submitDangerAction(): Promise<void> {
-     if (!menuMode || dangerBusy) return;
+     if (menuMode === 'menu' || dangerBusy) return;


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


─── apps/web/src/commercial/BillingPage.tsx:119-120 ───
[maintainability · low] wk-bill-1（表头行底边框）与 wk-bill-2（表头字重）为无意图的序号式类名，无法自我表达用途，后续读者需跳到
commercial-u.css 才能理解，且与同文件其余语义化命名（wk-bill-usage-table-cell 等）风格不一致。这两个类仅在 BillingPage
表头使用，建议改为语义化命名。

- <tr className="wk-bill-1">
-                   <th className={USAGE_TABLE_CELL + ' wk-bill-2'}>资源</th>
+ <tr className="wk-bill-thead-row">
+                   <th className={USAGE_TABLE_CELL + ' wk-bill-th'}>资源</th>


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


─── apps/web/src/commercial/commercial-u.css:2-2 ───
[documentation · low] 头注释声明「导入顺序：须在本域 .td.css 之前」，但 apps/web/src/commercial/ 目录下并不存在任何 .td.css
文件（本域仅新增了 commercial-u.css），该约束指向不存在的文件，疑为其他域（settings/agents 等确有
td.css）模板复制残留，会误导后续维护者去寻找/创建不存在的文件。建议删除该半句，或改为说明与全局 TDesign 主题 CSS 的实际顺序关系。



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


─── apps/web/src/configuration/ConfigurationOperations.tsx:64-64 ───
[bug · medium] tdesign-react 的 Input 只读属性名是全小写 `readonly`（与 Vue 端 tdesign-vue-next 一致，见 InputProps
定义），JSX 属性大小写敏感，驼峰 `readOnly` 不会被组件识别、也不会透传到内层 input。迁移前 @weknora/ui 的 Input 会把 readOnly 透传给原生 input
生效；迁移后该 "Share permission" 展示字段变为可编辑，且受控 value 无 onChange 还会触发 React 受控组件警告（输入回弹）。建议改为全小写
`readonly`。

-   return <Card className="wk-configuration-operations"><h2>Agent sharing and visibility</h2><p className="wk-muted wk-cfg-ops-1">Select an agent here for sharing or hide/show operations. To choose an agent for a conversation, use the Agent selector in the chat entry. Shared agents are read-only in the receiving workspace, so every share uses Viewer permission. Sharing and hide/show preferences remain server-authorized.</p>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}<div className="wk-form-grid wk-form-grid--two wk-cfg-ops-2"><label>Selected agent<Select value={selected} onChange={(value) => setSelected(String(value))}><Select.Option value="" label="No agent">No agent</Select.Option>{agents.map((agent) => <Select.Option key={agent.id} value={agent.id} label={`${agent.name}${disabledIds.includes(agent.id) ? ' · disabled' : ''}`}>{agent.name}{disabledIds.includes(agent.id) ? ' · disabled' : ''}</Select.Option>)}</Select></label><label>Organization ID<Input value={organizationId} onChange={(value) => setOrganizationId(String(value))} placeholder="org_…" /></label><label>Share permission<Input value="Viewer (read-only)" readOnly /></label></div><div className="wk-list-actions wk-cfg-ops-3"><Button type="button" theme="default" variant="outline" disabled={busy || !selected} onClick={() => void setDisabled(!selectedDisabled)}>{selectedDisabled ? 'Show selected agent' : 'Hide selected agent'}</Button><Button type="button" theme="default" variant="outline" loading={busy} disabled={!selected || !organizationId.trim()} onClick={() => void share()}>Share agent</Button></div></Card>;
+ <label>Share permission<Input value="Viewer (read-only)" readonly /></label>


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


─── apps/web/src/knowledge/knowledge-u.css:251-253 ───
[style · low] 选择器 `.wk-kg-24 :-webkit-details-marker` 中的空格使其成为后代选择器——`::-webkit-details-marker` 是
summary 元素自身的伪元素而非其后代,该规则永远匹配不到(原 Tailwind `[&::-webkit-details-marker]:hidden` 指向元素自身)。当前因
`.wk-kg-24` 的 `display: inline-flex` 使 Chrome 不再生成 marker 才未暴露问题,但这是一条无效的死规则,应去掉空格修正。

- .wk-kg-24 :-webkit-details-marker {
+ .wk-kg-24::-webkit-details-marker {
    display: none;
  }


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


─── apps/web/src/organizations/OrganizationsPage.tsx:14-14 ───
[maintainability · low] FormEvent 具名导入未被使用：文件内
submitCreate/submitBasic/submitUpgradeRequest（741/760/952 行）仍写的是 React.FormEvent（依赖 UMD 全局命名空间），导致本行
FormEvent 成为死导入。要么删掉，要么顺手把三处 React.FormEvent 改用该导入以统一风格。

- import type { CSSProperties, FormEvent, MouseEvent as ReactMouseEvent } from 'react';
+ import type { CSSProperties, MouseEvent as ReactMouseEvent } from 'react';


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


─── apps/web/src/settings/ChatHistorySettingsPanel.tsx:51-53 ───
[bug · medium] 防抖保存竞态导致丢更新：保存请求在途时（savingRef.current === true），防抖定时器到期触发的 save() 被静默
return，且不会重新调度定时器——用户在保存往返期间的新开关/模型变更被直接丢弃。更糟的是 save 成功后 onSaved?.() 触发壳层 load(true)
全量刷新，initialValue 变化后本组件的同步 useEffect 会用服务端旧值重置
enabled/embeddingModelId/latestRef，用户在往返期间的修改被回滚丢失（UI 上表现为操作被撤销）。建议：① in-flight 期间将待存快照排队，save
完成后补发；② 同步 effect 重置前比对 latestRef 与服务端值，本地有未落库变更时保留本地值。



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


─── apps/web/src/settings/EnvVarSettingsPanel.tsx:127-127 ───
[other · low] 可访问性回退：旧实现帮助触发按钮带 aria-expanded={helpOpen} 且 onMouseOver/onFocus 双通道展开（键盘聚焦可打开弹层）；迁移为
TDesign Popup trigger="hover" 后 aria-expanded 丢失，且 hover 触发不响应键盘焦点，键盘用户失去等价操作路径。建议通过 onVisibleChange
维护受控可见状态并回填 aria-expanded，必要时改用对 focus 友好的触发方式。



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


─── apps/web/src/settings/GeneralPreferencesPanel.tsx:351-351 ───
[other · low] 可访问性回退：自动更新开关迁移到 TDesign Switch 时丢失了 aria-label={autoUpdateCopy.label}；同样，语言/主题/字体三处
Select 迁移后也丢失了 aria-label，屏幕阅读器无法获知控件用途。建议补回 aria-label（TDesign 组件支持属性透传）。

-             <Switch value={autoCheckUpdate} onChange={(value) => handleAutoCheckUpdateChange(Boolean(value))} />
+             <Switch value={autoCheckUpdate} aria-label={autoUpdateCopy.label} onChange={(value) => handleAutoCheckUpdateChange(Boolean(value))} />


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


─── apps/web/src/shared/shared-u.css:29-33 ───
[style · low] 生成器产物残留：.wk-shared-4 / .wk-shared-8 / .wk-shared-13 / .wk-shared-15 均出现重复的
border-style: solid; 声明（同一规则内连续两次）。虽然无功能影响，但与文件头「值 = utilities 编码的生效值」的约定相悖，容易误导后续手工维护，建议清理重复声明。

    border-radius: 6px;
    border-style: solid;
    border-width: 1px;
-   border-style: solid;
    border-color: #dcdcdc;


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


─── apps/web/src/wiki/wiki-u.css:188-192 ───
[style · low] 同类残留：.wk-wiki-17 中 font-size: 13px 紧跟 font-size: inherit（后者生效，前者是死代码），.wk-wiki-63 中
font-weight: inherit 紧跟 font-weight: 650 !important 同理。既然文件头声明「值 = utilities
编码的生效值」，这些被覆盖的声明应删除，否则维护者会误以为 13px/inherit 是两个并存的意图。

    padding-inline: 8px;
    padding-block: 4px;
-   font-size: 13px;
    font-family: inherit;
    font-size: inherit;


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


─── docs/design/job-search/prototype/index.html:12-12 ───
[bug · low] file:// 协议下 ES module 脚本会被浏览器 CORS 策略拦截（Chrome/Firefox 均如此），直接双击 index.html 将得到空白页——这与
app.js 中 syncUrl() 的 replaceState 抛错是两条叠加的失败路径。本文件并未使用 import/export，若希望支持本地直接打开可改用 classic
script（配合 app.js 的降级修复）；否则建议在头部注释标明必须经 serve.py 等 HTTP 服务访问。

-     <script type="module" src="./app.js"></script>
+     <!-- 需通过 serve.py（http://127.0.0.1:4178）访问；file:// 下 ES module 会被 CORS 拦截 -->
+     <script defer src="./app.js"></script>


─── internal/handler/session/artifact_download.go:679-683 ───
[maintainability · medium] streamArtifactVersion 与同文件 DownloadArtifactVersion（约 585-629 行）中的“租户解析 →
ParseStorageBackendPath → ResolveTenantFileServiceWithFallback → GetFile →
filetransport.Serve（相同参数）”近乎逐行重复，成为该文件内第三份流式实现（另有
streamResolvedArtifact）。这是安全敏感的下载路径：后续对存储解析、no-store 缓存策略或 Content-Disposition
的任何单边修复都容易漏改另一份，导致两条下载链路行为分叉。建议让 DownloadArtifactVersion 复用
streamArtifactVersion（两处差异仅在错误响应形态：c.Error vs AbortWithStatus，可用参数或返回错误区分）。



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


─── packages/views/src/craft/interaction-card.tsx:10-10 ───
[test · medium] 切换到 td Button 后，按钮文本被包进 `<span class="t-button__text">`（td.tsx:84-86），而配套测试
interaction-card.test.tsx:84 仍用 `/<button[^>]*>[^<]*<\/button>/g` 统计按钮——该正则永远无法匹配 td Button
的输出（button 开标签后紧跟
`<span`），「终态卡不暴露任何决策控件」（本文件头部声明的红线：迟到审批不能复活已取消的请求）这一不变量就此失去自动化护栏，断言恒真。本次更新未同步修改该测试（对照
versions.test.tsx:56-58 已为同一 span 包裹结构适配了正则）。建议同步更新测试断言，改为按 `<button` 开标签存在性检测。

- import { Button } from './td.tsx';
+ // interaction-card.test.tsx:84
+ const buttons = markup.match(/<button[\s>]/g) ?? [];


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



─── apps/embed/src/button.tsx:1-1 ───
[maintainability · low] 未使用的导入：apps/embed 的 tsconfig 配置为 "jsx": "react-jsx"（JSX 由 react/jsx-runtime
自动转换），文件中 `React` 标识符从未被引用（仅使用了具名类型 ButtonHTMLAttributes、ReactNode）。建议移除默认导入，避免
lint（no-unused-vars）告警并保持与 modern React 代码风格一致。

- import React, { type ButtonHTMLAttributes, type ReactNode } from 'react';
+ import { type ButtonHTMLAttributes, type ReactNode } from 'react';


─── apps/miniprogram/src/career/application-material.tsx:213-216 ───
[bug · medium] 保存草稿以 material 状态（而非 materialId 是否已输入）区分新建/编辑：用户手输已有材料编号但未点「读取材料」时（onChange 已把
material 清空），点「保存草稿」会静默走新建分支，按 application 的
pinnedEvidence（或页面顶部的岗位编号）创建一份全新材料，并把输入框覆盖为新材料编号——与用户「编辑该编号对应材料」的预期相反，产生冗余材料。服务层
materialRequestBody（career.ts:280）在无 application 且无 opportunityId 时会抛「材料编辑缺少岗位证据或材料编号」，有 application
时则静默新建，行为不一致。建议：materialId 非空且尚未读取时阻断并提示先读取，或直接按 materialId 走编辑分支。

            const body = career.bodyFromEditable(sections);
+           if (!material && materialId.trim()) throw Object.assign(new Error('已填写材料编号：请先点「读取材料」核对后再保存，避免误建新材料'), { code: 'material_not_loaded', recoverable: true });
            const receipt = material
              ? await career.editMaterial({ materialId, body })
              : await career.editMaterial({ opportunityId: application?.pinnedEvidence.opportunityId ?? opportunityId, snapshotId: application?.pinnedEvidence.snapshotId ?? snapshotId, body });


─── apps/miniprogram/src/career/application-material.tsx:181-184 ───
[bug · low] 材料编号 onChange 重置了 material/sections/confirmedVersion/exports/checks，但未重置
versionChoice：切换材料后投递版本仍持有旧材料的 exportId。虽然
resolveSubmissionVersion（career.ts:381-386）对不在当前可提交列表中的选择会返回 unselected 并阻断提交（不会发生跨材料污染，安全红线守住），但 UI
状态不一致——radio 视觉上全部未选中、`!versionChoice` 的「请先选择投递版本」警示也不显示，用户直接点「记录投递」才收到阻断报错，易困惑。建议在失效逻辑中一并
setVersionChoice('')。

          setConfirmedVersion(undefined);
          setExports(undefined);
          setChecks([]);
+         setVersionChoice('');
        }} placeholder='材料编号' />


─── apps/miniprogram/src/career/application-material.tsx:99-101 ───
[maintainability · low] 投递对账回调中 `const receipt = await ...; void receipt;` 是死代码：接收返回值后仅用 void
压制未使用告警。应直接 await 不接收返回值。

-         const receipt = await career.reconcilePendingSubmission();
+         await career.reconcilePendingSubmission();
          if (application) await loadSubmissions(application.applicationId);
-         void receipt;


─── apps/miniprogram/src/career/application-material.tsx:223-224 ───
[bug · low] 材料卡片在仅 application 在场（materialId 为空）时即渲染「确认为不可变新版本」与「发布双格式」按钮且未传 disabled：点击后
confirmMaterial('')/publishMaterial('', v) 的客户端前置校验抛出「缺少材料编号」，该裸 Error 经 useAction 内的
errorMessage()（ui.tsx:56）坍缩为通用文案「操作未完成，请检查网络或刷新状态后重试」，误导用户以为是网络问题。下方发布按钮（254 行）同理。建议按 materialId
是否非空禁用，或让客户端校验错误携带 code 并在 errorMessage 中透出。

-         <t-button block size='large' theme='default' ariaLabel='确认材料新版本' customStyle={tdesignButtonStyle} loading={matConfirmBusy.busy} onTap={() => void matConfirmBusy.run(async () => {
+         <t-button block size='large' theme='default' ariaLabel='确认材料新版本' customStyle={tdesignButtonStyle} loading={matConfirmBusy.busy} disabled={!materialId.trim()} onTap={() => void matConfirmBusy.run(async () => {
            const receipt = await career.confirmMaterial(materialId);


─── apps/miniprogram/src/career/application-material.tsx:154-154 ───
[style · low] applyBusy.error 的 Notice 用三层嵌套三元表达式按 appErrCode 渲染提示文案，违反嵌套三元规范；本文件上方
evaluationLabel/evaluationTone 已采用 Record 映射模式，此处应保持一致。

-       {applyBusy.error && <Notice tone='danger'>{applyBusy.error}{appErrCode === 'revision_conflict' ? ' 档案已更新：下拉刷新读取最新修订后重试（新提交将使用新请求编号）。' : appErrCode === 'application_conflict' ? ' 此岗位与该批次已存在申请：一个岗位和招聘批次只有一个申请与 Task；可为其他批次创建。' : appErrCode === 'hard_ineligible_requires_continue' ? ' 硬性条件不符，需要先勾选显式继续才能提交申请。' : ''}</Notice>}
+ const applyErrorHint: Record<string, string> = {
+   revision_conflict: ' 档案已更新：下拉刷新读取最新修订后重试（新提交将使用新请求编号）。',
+   application_conflict: ' 此岗位与该批次已存在申请：一个岗位和招聘批次只有一个申请与 Task；可为其他批次创建。',
+   hard_ineligible_requires_continue: ' 硬性条件不符，需要先勾选显式继续才能提交申请。',
+ };
+ // ...
+       {applyBusy.error && <Notice tone='danger'>{applyBusy.error}{appErrCode ? (applyErrorHint[appErrCode] ?? '') : ''}</Notice>}


─── apps/miniprogram/src/career/application-material.tsx:23-23 ───
[maintainability · low] tdesignButtonStyle 这条长常量在
discovery.tsx、export-deletion.tsx、progress-preparation.tsx、rules-usage-reminders.tsx 与本文件共 5 处逐字重复（经
code_search 确认）。任何一处需要调整（如补充 CSS 变量）都要同步 5 份，易漂移。建议提取到共享模块（如 src/career/shared.ts 或
components/ui.tsx）统一导出。

- const tdesignButtonStyle = '--td-brand-color:var(--wk-color-action-primary);--td-brand-color-active:var(--wk-color-action-pressed);--td-brand-color-disabled:var(--wk-color-action-disabled);--td-button-primary-bg-color:var(--wk-color-action-primary);--td-button-primary-active-bg-color:var(--wk-color-action-pressed);--td-button-primary-disabled-bg-color:var(--wk-color-action-disabled);--td-button-primary-disabled-color:var(--wk-color-action-disabled-text);--td-button-large-height:var(--wk-component-button-height);';
+ // src/career/shared.ts
+ export const tdesignButtonStyle = '--td-brand-color:var(--wk-color-action-primary);…'; // 各 career 页面统一 import


─── apps/miniprogram/src/career/application-material.tsx:271-272 ───
[maintainability · low] downloadBusy 被所有导出条目的所有格式下载按钮共用：useAction 的 running
守卫（ui.tsx:56）虽防住了并发写入，但任一下载进行时全部下载按钮同时
loading，期间点击其他格式按钮会被静默忽略（不排队、无提示），无法区分进行中的是哪一个。建议记录当前下载目标的键（exportId+format）做按钮粒度的 loading 展示。

-             <t-button size='medium' theme='default' ariaLabel={`下载并打开 ${file.format}`} customStyle={tdesignButtonStyle} loading={downloadBusy.busy} onTap={() => void downloadBusy.run(async () => {
+             <t-button size='medium' theme='default' ariaLabel={`下载并打开 ${file.format}`} customStyle={tdesignButtonStyle} loading={downloadBusy.busy && downloadingKey === `${exportReceipt.exportId}:${file.format}`} onTap={() => void downloadBusy.run(async () => {
+               setDownloadingKey(`${exportReceipt.exportId}:${file.format}`);
                const { check } = await career.openMaterialExport(materialId, exportReceipt.exportId, file.format);


─── apps/miniprogram/src/career/discovery.tsx:37-37 ───
[bug · medium] useShareAppMessage 把核对中的 JD 全文经 encodeURIComponent 放进分享卡片的 path 查询参数：长
JD（数百至数千字，编码后成倍膨胀）会超出微信分享 path 的长度承受范围，导致分享失败或 query 被截断；接收侧
readSharedEntry（career-platform.ts:57-62）只做解码与 trim，没有长度/完整性校验，截断后的文本会静默进入「先核对后提交」预览（fullLength
显示的也是截断后长度），用户难以察觉导入的是不完整 JD。建议对携带全文设置长度上限（超限降级为不带 jd 的入口卡片），并在 readSharedEntry 侧对可疑截断给出提示。

-   useShareAppMessage(() => ({ title: draft ? `职位核对：${draft.preview.excerpt.slice(0, 20)}` : 'WeKnora 求职工作台', path: `/career/discovery${draft ? `?jd=${encodeURIComponent(draft.rawText)}` : ''}` }));
+   // 超长 JD 不随卡片携带全文，避免分享 path 截断导致接收方拿到不完整原文
+   const SHARE_JD_MAX = 600;
+   useShareAppMessage(() => ({
+     title: draft ? `职位核对：${draft.preview.excerpt.slice(0, 20)}` : 'WeKnora 求职工作台',
+     path: `/career/discovery${draft && draft.rawText.length <= SHARE_JD_MAX ? `?jd=${encodeURIComponent(draft.rawText)}` : ''}`,
+   });


─── apps/miniprogram/src/career/discovery.tsx:43-43 ───
[maintainability · low] quotaRefused 以 errorMessage 产出文案的子串「搜索额度不足」判定 typed
429（search_quota_refused）：判定逻辑与 core/errors.ts 中的文案跨文件硬耦合——文案微调（如「搜索配额不足」）会让 includes 永假，专属 warning
降级静默退化为 danger 失败态，且无编译期保护。根源是 useAction 只暴露成串文案（ui.tsx:56 setError(errorMessage(e))）。建议在 useAction
中同时保留原始错误码，页面改用 typed 判定。

-   const quotaRefused = (message?: string) => message?.includes('搜索额度不足') ?? false;
+ // ui.tsx: useAction 返回 { busy, error, errorCode, run }，catch 中记录 (e as {code?:unknown})?.code
+ // 页面：const quotaRefused = searchBusy.errorCode === 'search_quota_refused';


─── apps/miniprogram/src/career/discovery.tsx:45-45 ───
[bug · low] copyLink 中 `void Taro.setClipboardData({...}).then(...)` 链上没有 catch：剪贴板授权被拒或系统失败会产生未处理的
Promise 拒绝（用户无任何反馈，且可能在控制台告警）。adapters/career-platform.ts:305 的同类调用处于 async 函数内有外层错误处理，此处应保持一致的容错。

-   const copyLink = (link: string) => { void Taro.setClipboardData({ data: link }).then(() => Taro.showToast({ title: '链接已复制', icon: 'none' })); };
+   const copyLink = (link: string) => {
+     void Taro.setClipboardData({ data: link })
+       .then(() => Taro.showToast({ title: '链接已复制', icon: 'none' }))
+       .catch(() => Taro.showToast({ title: '复制失败，请长按链接手动复制', icon: 'none' }));
+   };


─── apps/miniprogram/src/career/discovery.tsx:22-23 ───
[maintainability · low] confirmBusy/dismissBusy 分别被全部待确认事实行共用，recoverBusy 同时服务「档案对账」与「同步 Web
端档案变更」两个语义不同的动作：useAction 的 running 守卫能防并发写入，但任一操作进行时全部共用按钮同时
loading，期间点击其他按钮被静默忽略（如连续确认两条事实时第二条被吞掉、需重试），且两处错误提示共用同一份文案。建议按操作目标记录键（如进行中的 proposalId）区分
loading，同步操作单独建 useAction 实例。

    const confirmBusy = useAction(); const dismissBusy = useAction(); const proposeBusy = useAction(); const uploadBusy = useAction();
-   const searchBusy = useAction(); const recoverBusy = useAction(); const importBusy = useAction();
+   const searchBusy = useAction(); const recoverBusy = useAction(); const syncBusy = useAction(); const importBusy = useAction();
+   // …底部「同步 Web 端档案变更」改用 syncBusy，并按 proposalId 区分行级 loading


─── apps/miniprogram/src/services/career-intent.ts:40-41 ───
[bug · medium] 歧义判据把"无 code 且无 status"一律判为歧义，但 packages/api-client/src/career.ts 的解码器抛的是裸
TypeError（如 decodeMaterialExportDownload 第 740/756 行，无 code/status）：服务端返回 200 而正文畸形（合同漂移/服务端
bug）时会被判为"结果未知"，落 intent 并抛 outcome_unknown；同 requestId 重放又确定性复现同样的解码失败，用户被困在恢复循环。且恢复出口不齐——abandon
显式放弃仅 rule-write/reminder-write
有（career-platform.ts:489/537），search/application/material/export/submission/progress/preparation/spa
ceExport/spaceDeletion 链路均无放弃出口，intent 只能靠登出清空。建议：给解码失败附加确定性标记（如 Object.assign(new TypeError(...), {
code: 'contract_violation' })，在 ambiguousOutcome 中判 false），并为各 intent kind 提供统一 abandon 出口。



─── apps/miniprogram/src/services/career.ts:141-141 ───
[maintainability · medium] searchOnce 此处手写了与 recoverableWrite 完全同构的 stamp 守卫 +
歧义判据；deleteWholeSpace/retryPendingSpaceDeletion 同样手写（deleteWholeSpace 因 partial-success 需保留 intent
的语义确属合理特例）。这正是 career-intent.ts 头注释里 med-39 要消除的"两套副本漂移"模式：将来守卫口径再变（如新增确定失败码）时，这些手写点最易被遗漏。searchOnce
的写入形状（query + expectedRevision + 单端点）可完全复用 recoverableWrite，建议收敛。

-     if (ambiguousOutcome(error)) { store.write(searchKey(stamp), { requestId: id, query: trimmed, expectedRevision: expected }); throw unknownOutcome(id, trimmed, error); }
+ export async function searchOnce(query: string): Promise<SearchOutcome> {
+   const trimmed = query.trim();
+   if (!trimmed) throw new Error('请先输入想找的岗位或要求');
+   return recoverableWrite<SearchOutcome>(store, {
+     kind: 'search', describe: '找岗', input: { query: trimmed }, expected: revision(),
+     send: async (id, expected) => decodeSearchOutcome(await client.request({ method: 'POST', path: '/api/v1/career/searches', body: { requestId: id, query: trimmed, expectedRevision: expected } })),
+   });
+ }


─── apps/miniprogram/src/adapters/career-platform.ts:153-163 ───
[maintainability · low] ExportDownloadGrant 是 api-client
MaterialExportDownload（packages/api-client/src/career.ts:667，当前九字段完全同形）的手工结构副本，career.ts 的
openMaterialExport 以 `grant as ExportDownloadGrant` 无校验转换。TS 的 `as`
在源类型新增/改名字段时仍可静默通过（结构兼容方向不报错），两侧漂移不会产生编译错误，digest/url 参数可能静默错位使校验链失效。本文件已从同一模块导入其它类型，直接别名即可消除副本。

- export interface ExportDownloadGrant {
-   exportId: string;
-   materialId: string;
-   version: number;
-   format: 'pdf' | 'docx';
-   digest: string;
-   size: number;
-   expiresAt: number;
-   signature: string;
-   url: string;
- }
+ import type { MaterialExportDownload } from '../../../../packages/api-client/src/career.ts';
+ /** 与 api-client MaterialExportDownload 同形（直接别名复用，避免手工副本漂移）。 */
+ export type ExportDownloadGrant = MaterialExportDownload;


─── apps/miniprogram/src/adapters/career-platform.ts:49-50 ───
[bug · low] "解码成功且更短才采用"的启发式只对含 CJK 的场景安全：JD 原文里的字面 %XX（纯 ASCII，如「含%20佣金」「%41
分位」）解码必然更短而被采纳，用户原文被静默改写，后续导入解析基于被篡改文本。career-platform.test.mjs 只钉了裸
%（不匹配正则）与损坏序列（抛异常回退）两个边界，未覆盖此场景。建议收紧判据（例如仅当解码结果引入非 ASCII 字符、或 %XX 序列占比显著时才采用），并补字面 %XX 不被改写的用例。

+   try {
      const decoded = decodeURIComponent(raw);
-     return decoded.length < raw.length ? decoded : raw;
+     // 仅当解码确实引入了多字节字符（分享 path 的 percent-encoding 特征）且更短时采用，
+     // 纯 ASCII 字面 %XX（如「含%20佣金」）保持原文。
+     const introducesNonAscii = /[\u0080-\uffff]/.test(decoded);
+     return introducesNonAscii && decoded.length < raw.length ? decoded : raw;
+   } catch { return raw; }


─── apps/miniprogram/src/services/career.ts:9-10 ───
[maintainability · low] 同文件混用两种跨包导入方式：NativeFileSource 走包名 '@weknora/api-client'，而 career 解码器/类型走
'../../../../packages/api-client/src/career.ts' 深层相对路径（career-platform.ts
亦然）。packages/api-client/src/index.ts 目前只再导出 createCareerApi 与少量类型，未导出这批解码器——目录结构调整或包 exports
收紧时小程序端编译会直接断裂。建议在包入口补 re-export，统一走包名导入。



─── apps/miniprogram/src/adapters/career-platform.ts:551-551 ───
[maintainability · low] 订阅消息模板 id 以源码常量硬编码为空数组，且已确认 config/index.ts、app.config.ts 均无模板 id
注入点：上线订阅消息必须改代码发版。同时导出的是可变数组，外部可就地篡改内容。建议下沉为构建期注入（如 config/index.ts 的 defineConstants / 环境变量），并导出为
readonly 防篡改。

- export const REMINDER_SUBSCRIBE_TEMPLATE_IDS: string[] = [];
+ export const REMINDER_SUBSCRIBE_TEMPLATE_IDS: readonly string[] = (globalThis.__CAREER_SUBSCRIBE_TEMPLATES__ as readonly string[] | undefined) ?? [];


─── apps/miniprogram/src/services/career.ts:285-286 ───
[maintainability · low] 为触发参数校验而生成了即弃 requestId——requestId() 会递增模块级 sequence
计数器并拼随机段（core/intent.ts:26），纯为副作用调用意图不清晰。saveRule（career-platform.ts）、recordSubmission、appendProgres
sEvent、correctProgressEvent、generatePreparation 均为同模式。建议拆出无 id 的纯校验函数（validateXxx(input)），或让
*RequestBody 的 id 参数可选。

  export async function editMaterial(intent: EditMaterialIntent): Promise<MaterialReceipt> {
-   materialRequestBody(newRequestId(), intent, revision()); // 先做参数校验，再进入可恢复写入
+   materialRequestBody(undefined, intent, revision()); // 先做参数校验，再进入可恢复写入（id 参数可选）


─── apps/miniprogram/src/adapters/career-platform.ts:238-240 ───
[performance · low] 校验阶段用同步 readFileSync 读入最大 20MB（MAX_EXPORT_BYTES）的完整 ArrayBuffer，再在小程序 JS 单线程上跑纯
TS sha256：大文件在低端机上会阻塞 UI 数秒，且用户对"校验中"状态无感知（下载完成到打开之间界面冻结）。建议分块读取 + 增量哈希（块间 await 让出事件循环），或至少在 UI 层配合
loading 提示并评估把查看上限降到实际导出体量。



─── apps/miniprogram/src/career/progress-preparation.tsx:138-146 ───
[bug · medium] saveRevision 的错误处理边界有两处缺陷：① 成功提交 editMaterial 并已 clearPreparationDraft 之后，服务端回读
`(await career.material(materialId)).body)` 仍在同一 try 内——回读失败（如提交后网络抖动）会落入 catch，此时 genNotice
已声称“已提交”、draftNotice 却改称“已保留本地草稿（未提交）”，两条文案互相矛盾且与事实不符（草稿已清除、修订已成功），还会诱导用户重复点保存；这与同页材料重试路径 `try {
setRevisedBody(...) } catch { /* 回读失败不掩埋重试成功的事实 */ }` 的口径不一致。② 提交前的主张取回 `(await
career.material(materialId)).body.sections)` 位于 try 之外，断网时错误直接冒泡到 useAction，既不设置 reviseErrCode，也不触发
looksOffline 的“已保留本地草稿”提示，用户只能看到裸传输错误。建议将提交后回读单独 try/catch 包裹，并将提交前取回纳入统一的错误分型。

      try {
        await career.editMaterial({ materialId, body });
        clearPreparationDraft(editTarget!.applicationId, editTarget!.focus);
        setDraftNotice(''); setReviseErrCode(undefined);
        setGenNotice('准备草稿修订已提交（仍是可审阅草稿，发布需另行确认材料版本）。');
-       // 与 Web 同语义：回执只是回声，修订的持久事实从材料域回读。
-       setRevisedBody((await career.material(materialId)).body);
+       // 与 Web 同语义：回执只是回声，修订的持久事实从材料域回读；回读失败不掩埋已成功提交的修订。
+       try { setRevisedBody((await career.material(materialId)).body); } catch { /* 回读失败不掩埋修订成功的事实 */ }
        void listBusy.run(loadPreparations);
      } catch (error) {


─── apps/miniprogram/src/career/progress-preparation.tsx:276-276 ───
[style · low] 嵌套三元表达式：状态标签为两层三元（draft → failed → 兜底），违反检查清单“禁止嵌套三元”。本页与
export-deletion.tsx、rules-usage-reminders.tsx 均存在同类写法，建议抽为标签映射函数（如 `preparationStatusLabel(status)`
/ `deletionCardTone(status)`），与既有 statusLabels 等映射风格保持一致。

-           <Text className='wk-row-title'>{focusLabels[item.focus] ?? item.focus} · {item.status === 'draft' ? '草稿（可审阅、可修订）' : item.status === 'failed' ? `生成失败（${item.failureCode ?? ''}）：${item.failureMessage ?? '生成未完成'}` : '生成中（可恢复）'}</Text>
+           <Text className='wk-row-title'>{focusLabels[item.focus] ?? item.focus} · {preparationStatusLabel(item)}</Text>
+ 
+ // 抽出为组件外纯函数：
+ const preparationStatusLabel = (item: PreparationReceipt): string =>
+   item.status === 'draft' ? '草稿（可审阅、可修订）'
+     : item.status === 'failed' ? `生成失败（${item.failureCode ?? ''}）：${item.failureMessage ?? '生成未完成'}`
+     : '生成中（可恢复）';


─── apps/miniprogram/src/career/progress-preparation.tsx:230-230 ───
[maintainability · low] 录入进展按钮未对 applicationId 为空做禁用，与同页“生成准备草稿”按钮的 `disabled={!applicationId.trim()
|| ...}` 口径不一致：清空编号后点击会直接打到服务端，靠 progressRequestBody 抛出裸错误兜底。建议统一在前端禁用。

-         <t-button block size='large' theme='primary' ariaLabel={correcting ? '提交更正' : '录入进展'} customStyle={tdesignButtonStyle} loading={correcting ? correctBusy.busy : appendBusy.busy} disabled={pendingProgress !== null} onTap={() => void (correcting ? correctBusy : appendBusy).run(async () => {
+         <t-button block size='large' theme='primary' ariaLabel={correcting ? '提交更正' : '录入进展'} customStyle={tdesignButtonStyle} loading={correcting ? correctBusy.busy : appendBusy.busy} disabled={pendingProgress !== null || !applicationId.trim()} onTap={() => void (correcting ? correctBusy : appendBusy).run(async () => {


─── apps/miniprogram/src/career/export-deletion.tsx:26-26 ───
[maintainability · low] 重复代码：tdesignButtonStyle 这串 CSS 变量连同 typedCode、digestHead
在本组三个新页面（export-deletion / progress-preparation / rules-usage-reminders）各复制一份，加上既有的
application-material.tsx、discovery.tsx，全仓已达 5 份。后续主题变量调整需 5 处同步修改，极易漂移。建议抽到共享模块（如
src/career/tdesign.ts 或 components/ui.tsx）统一导出。

- const tdesignButtonStyle = '--td-brand-color:var(--wk-color-action-primary);--td-brand-color-active:var(--wk-color-action-pressed);--td-brand-color-disabled:var(--wk-color-action-disabled);--td-button-primary-bg-color:var(--wk-color-action-primary);--td-button-primary-active-bg-color:var(--wk-color-action-pressed);--td-button-primary-disabled-bg-color:var(--wk-color-action-disabled);--td-button-primary-disabled-color:var(--wk-color-action-disabled-text);--td-button-large-height:var(--wk-component-button-height);';
+ // 抽到共享模块（src/career/tdesign.ts）后各页统一 import：
+ // export const tdesignButtonStyle = '--td-brand-color:var(--wk-color-action-primary);…';
+ // export const typedCode = (error: unknown): string | undefined => { … };
+ import { tdesignButtonStyle } from './tdesign.ts';


─── apps/miniprogram/src/career/export-deletion.tsx:220-220 ───
[style · low] 嵌套三元表达式：Card tone 与下方 Badge tone（`deleted ? 'success' : deletion.status === 'partial'
? 'danger' : 'warning'`）、步骤 Badge（`step.status === 'done' ? 'success' : step.status === 'failed' ?
'danger' : 'neutral'`）均为两层三元，违反检查清单“禁止嵌套三元表达式”。建议抽为小映射函数（如 `deletionCardTone(status)` /
`stepBadgeTone(status)`）。

-     {deletion && <Card tone={deletion.status === 'deleted' ? 'mint' : deletion.status === 'partial' ? 'warning' : 'white'}>
+     {deletion && <Card tone={deletionCardTone(deletion.status)}>
+ 
+ // 组件外纯函数：
+ const deletionCardTone = (status: string): 'mint' | 'warning' | 'white' =>
+   status === 'deleted' ? 'mint' : status === 'partial' ? 'warning' : 'white';


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:208-208 ───
[style · low] 三层嵌套三元（no_templates → api_unavailable → 兜底，兜底分支内还含模板串三元），违反检查清单“禁止嵌套三元表达式”，可读性差。建议抽为
`unavailableReasonText(subscription)` 映射函数。

-       {subscription?.status === 'unavailable' && <Notice tone='warning'>订阅消息当前不可用（{subscription.reason === 'no_templates' ? '本构建未配置订阅消息模板 id——模板需在微信公众平台与本 appid 绑定后申请' : subscription.reason === 'api_unavailable' ? '当前环境没有 wx.requestSubscribeMessage（模拟器或基础库不支持）' : `原生调用失败${subscription.errMsg ? `：${subscription.errMsg}` : ''}`}）。如实告知未订阅：站内待办为准，不伪造已送达。</Notice>}
+       {subscription?.status === 'unavailable' && <Notice tone='warning'>订阅消息当前不可用（{unavailableReasonText(subscription)}）。如实告知未订阅：站内待办为准，不伪造已送达。</Notice>
+ 
+ // 组件外纯函数：
+ const unavailableReasonText = (s: SubscriptionRequestOutcome): string => {
+   if (s.reason === 'no_templates') return '本构建未配置订阅消息模板 id——模板需在微信公众平台与本 appid 绑定后申请';
+   if (s.reason === 'api_unavailable') return '当前环境没有 wx.requestSubscribeMessage（模拟器或基础库不支持）';
+   return `原生调用失败${s.errMsg ? `：${s.errMsg}` : ''}`;
+ };


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:48-48 ───
[maintainability · low] usageErrCode 是只写状态：loadEstimate 中 setUsageErrCode 落值后，全文件再无任何读取（额度预估的错误展示只用
usageBusy.error），属于死代码。建议删除该 state 及两处 setUsageErrCode，或参照其他页面的做法在 Notice 中消费 typed 分型。

-   const [usageErrCode, setUsageErrCode] = useState<string>();
+ // 删除该 state，loadEstimate 简化为：
+ const loadEstimate = async (): Promise<void> => {
+   setEstimate(await fetchUsageEstimate());
+ };


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:210-211 ───
[maintainability · low] 退订推送入口未按 revision 缺失禁用，仅靠运行时 throw 兜底，与同页“登记站内待办”入口 `disabled={revision ===
undefined || pendingReminder !== null}` 的口径不一致。建议统一加 disabled，避免可点击后才报错。

-       <Action secondary loading={optOutBusy.busy} onClick={() => void optOutBusy.run(async () => {
+       <Action secondary loading={optOutBusy.busy} disabled={revision === undefined} onClick={() => void optOutBusy.run(async () => {
          if (revision === undefined) throw new Error('请先读取档案修订');


─── apps/miniprogram/src/career/discovery.config.ts:1-2 ───
[bug · high] discovery.tsx 使用了 useShareAppMessage（第 37 行，携带 JD 原文的分享卡片），但 Taro（本项目 4.2.1）要求使用该 hook
的页面必须在页面 config 中显式声明 enableShareAppMessage: true，否则编译期不会把 onShareAppMessage
挂载到页面。当前配置缺失，分享回调永不生效：转发卡片将使用微信默认标题与当前页路径（不带 jd 参数），接收方 readSharedEntry 拿不到 JD
原文，「分享导入」只剩手动粘贴一条路，整个分享链路特性实际不可达。

  export default definePageConfig({
+   enableShareAppMessage: true,
    usingComponents: {


─── apps/miniprogram/src/career/discovery.tsx:105-105 ───
[bug · medium] pendingSearch 存在时搜索按钮未禁用，而 services/career.ts 的 searchOnce 在歧义失败时无条件
store.write(searchKey(stamp),…)（单键 wk:career:search:<scope>，无已有 pending
守卫）：用户忽略顶部「有一次结果未知的搜索」警示直接再搜，若新搜索也歧义失败，会静默覆盖旧 intent——旧未知搜索（可能已送达并消耗额度、已创建搜索记录）永久失去「用原 request ID
对账」的入口，违反未知结果可恢复的红线。对比同 PR 中发布按钮的 disabled={pendingPublish !== null} 与 desk.mutate 的
unresolved_action 守卫，此处缺同等防护。建议 pendingSearch !== null 时禁用新搜索（或在 searchOnce 内抛「先对账」守卫）。

-         <t-button block size='large' theme='primary' ariaLabel='发起一次性搜索' customStyle={tdesignButtonStyle} loading={searchBusy.busy} onTap={() => void searchBusy.run(async () => { setSearchOut(await career.searchOnce(searchInput)); })}>搜索</t-button>
+         <t-button block size='large' theme='primary' ariaLabel='发起一次性搜索' customStyle={tdesignButtonStyle} loading={searchBusy.busy} disabled={pendingSearch !== null} onTap={() => void searchBusy.run(async () => { setSearchOut(await career.searchOnce(searchInput)); })}>搜索</t-button>


─── apps/miniprogram/src/career/discovery.tsx:93-95 ───
[bug · low] 空 factValue 时「添加为待确认事实」按钮可点击：career.proposeFact 对空 value
无任何客户端校验（services/career.ts:53），会把 value 为空串的 propose 直接发给服务端，可能落一条空值待确认事实（「学历：（空）」）或得到
invalid_request 拒绝。同页粘贴导入按钮有 disabled={!pasteText.trim()}、searchOnce 有空值守卫，防护口径不一致。建议同样按空值禁用，或在
proposeFact 内补校验。

-         <Action secondary disabled={proposeBusy.busy} loading={proposeBusy.busy} onClick={() => void proposeBusy.run(async () => {
+         <Action secondary disabled={proposeBusy.busy || !factValue.trim()} loading={proposeBusy.busy} onClick={() => void proposeBusy.run(async () => {
            await career.proposeFact(factKey, factValue.trim()); setFactValue(''); query.reload();
          })}>添加为待确认事实</Action>


─── apps/miniprogram/src/career/discovery.tsx:97-97 ───
[bug · low] pendingFactAction（desk.unresolved）存在期间，确认/放弃/添加按钮均未禁用，而 desk.mutate 会立即抛
unresolved_action（desk.ts:104）——经 errorMessage 坍缩为通用文案，且此处附加的「冲突时下拉重读最新档案后再试」是错误指引（下拉重读无法清
unresolved，唯一解法是顶部「用原请求对账恢复」）。用户会被引导去刷新而不是对账，卡死循环。建议 pendingFactAction 存在时禁用这三类写入按钮，并把提示改为指向对账。



─── apps/miniprogram/src/career/application-material.tsx:212-213 ───
[bug · high] pendingMaterialWrite/pendingApplication/pendingSubmission
存在时，「保存草稿」「确认为不可变新版本」「记录投递」「创建申请」均未禁用，而 career.ts 的 writeRecoverable 在歧义失败时无条件 store.write 到单键
wk:career:material/application/submission:<scope>（career-intent.ts:85，无已有 pending 守卫）：顶部恢复 Notice
只是提醒，用户被下方「冲突时请重读材料后重试」之类文案引导再次写入，二次歧义失败即用新 requestId 覆盖旧 intent——旧未知申请/材料/投递写入永久失去「同 request ID
对账」能力（一岗一批一申请不可逆，可能产生用户无感知的孤儿申请）。同文件发布按钮已有 disabled={pendingPublish !== null} 的正确模式，其余三类写入应跟进（或在
writeRecoverable 链效仿 desk.mutate 的 unresolved 守卫）。

-         <t-button block size='large' theme='primary' ariaLabel='保存材料草稿' customStyle={tdesignButtonStyle} loading={matEditBusy.busy} onTap={() => void matEditBusy.run(async () => {
+         <t-button block size='large' theme='primary' ariaLabel='保存材料草稿' customStyle={tdesignButtonStyle} loading={matEditBusy.busy} disabled={pendingMaterialWrite !== null} onTap={() => void matEditBusy.run(async () => {
            const body = career.bodyFromEditable(sections);


─── apps/miniprogram/src/career/application-material.tsx:307-307 ───
[bug · medium] 这批客户端校验错误已精心撰写了面向用户的中文文案并携带 code，但 errorMessage（core/errors.ts）对这些 code 无映射，useAction
会把它们全部坍缩成「操作未完成，请检查网络或刷新状态后重试」：declared_time_invalid 没有任何前置 UI
警示，用户输错时间格式只会看到网络类提示，完全不知道该改哪里；publish_receipt_missing（recoverPubBusy 的「导出列表中没有这次发布的记录：请求可能未送达」）与
submission_version_unselected（versionChoice 持有失效 exportId 时，前置警示也不显示）同理。建议在 errorMessage 为这些 code
补映射（同 ARTIFACT_GRANT_EXPIRED 的模式）。

-               throw Object.assign(new Error('声明投递时间无法识别（示例 2026-09-25 20:00）：请改正或留空（留空记录确认时间）'), { code: 'declared_time_invalid' });
+ throw Object.assign(new Error('声明投递时间无法识别（示例 2026-09-25 20:00）：请改正或留空（留空记录确认时间）'), { code: 'declared_time_invalid' });
+ // 并在 core/errors.ts 增加映射：
+ // if(e.code==='declared_time_invalid')return '声明投递时间无法识别（示例 2026-09-25 20:00）：请改正或留空（留空记录确认时间）。';


─── apps/miniprogram/src/career/application-material.tsx:160-160 ───
[style · low] Badge 的 tone 用嵌套三元（ready→success / linking→warning / 其他→danger），违反嵌套三元规范；同文件上方已有
linkStateLabel 的 Record 映射模式（评估区 evaluationTone 同理），此处应保持一致补 linkStateTone 映射。

-         <Badge tone={application.linkState === 'ready' ? 'success' : application.linkState === 'linking' ? 'warning' : 'danger'}>{linkStateLabel[application.linkState]}</Badge>
+ const linkStateTone: Record<string, 'success' | 'warning' | 'danger'> = { ready: 'success', linking: 'warning', link_failed: 'danger' };
+ // <Badge tone={linkStateTone[application.linkState] ?? 'warning'}>{linkStateLabel[application.linkState]}</Badge>


─── apps/miniprogram/src/career/application-material.tsx:10-10 ───
[maintainability · low] errorMessage 在本文件仅导入未使用（typedCode 直接读 error.code，错误展示全部交给 useAction 内部的
errorMessage）——死导入，建议移除。

- import { errorMessage } from '../core/errors.ts';
+ // 删除该 import（discovery.tsx 有实际调用，本文件无）


─── apps/miniprogram/src/adapters/career-platform.ts:235-235 ───
[security · high] openExportedDocument/taroDownload 是本组代码中唯一绕过作用域守卫的网络路径，且比它自称"与 T06 files.ts
同源"的先例（platform/files.ts openProtectedDocument）缺失了三层保护：① files.ts:55/75/78/80/86 捕获 scope stamp、在
controller.signal 上挂 abort 监听（登出 stopSubscriptions/切换空间 abortAll 会中止下载）、下载与 getFileInfo 后各做一次
isCurrent 校验并 release controller——此处全部缺失，登出或切换账号后旧账号的职业材料仍会下载完成并经 openDocument
打开，违反"空间切换后旧响应失效"红线（runtime.ts scoped() 与本文件 T30 恢复链都严格执行该红线）；② files.ts:76 有 onProgressUpdate 的
20MB 中途中止，此处只在下载完成后检查 info.size，异常/恶意响应可无上限下载；③ 文件头注释宣称"网络一律经 services/career.ts
的认证客户端，本适配器永不直接发请求"，与 taroDownload 直接调 Taro.downloadFile 的事实矛盾。建议对齐 files.ts 先例：stamp 捕获 + abort 联动
+ 下载/复制/打开前的 isCurrent 校验 + controller release。

-   const outcome = await taroDownload({ url: `${parts.origin()}${path}`, header: { Authorization: `Bearer ${parts.bearer()}` }, timeout: 60000 });
+   const stamp = auth.scope.capture();
+   if (!auth.scope.isCurrent(stamp)) throw new Error('SCOPE_CHANGED');
+   const controller = auth.scope.controller();
+   try {
+     const outcome = await taroDownload({ url: `${parts.origin()}${path}`, header: { Authorization: `Bearer ${parts.bearer()}` }, timeout: 60000 }, stamp, controller);
+     if (!auth.scope.isCurrent(stamp)) throw new Error('SCOPE_CHANGED');
+     // taroDownload 内部对齐 files.ts:75-76：controller.signal abort 即 task.abort()，
+     // 并 onProgressUpdate 超过 MAX_EXPORT_BYTES 时中途中止；复制/打开前再各做一次 isCurrent 校验
+   } finally { auth.scope.release(controller); }


─── apps/miniprogram/src/services/career.ts:180-185 ───
[bug · medium] uploadResume 是本文件头宣称"写入一律 requestId + expectedRevision；结果未知用原 requestId
对账"纪律下唯一游离在外的写入：每次调用新生成 requestId，且没有 pending/reconcile/retry 三件套（同文件
searches/applications/materials/submissions/progress/preparations/exports/deletions 全部具备）。而服务端
Upload 合同恰恰是按 requestId 幂等恢复设计的（handler.go:1635 FindUploadClaim(requestID)、:1650 ClaimUpload 按
RequestID+IntentHash 建claim、202 in-progress、ResumeAndParse 重放）——上传超时（结果未知，服务端可能已建 source
并在解析）后用户重传，新 requestId 会创建全新 claim：同一简历重复存储、重复解析、重复产生待确认 proposal。建议至少把 requestId 随 intent 持久化（复用
writeRecoverable 或手工 store），结果未知时引导同编号重放（用户重新选同一文件即可通过 intent-hash 校验），而非静默换号重传。

  export async function uploadResume(file: NativeFileSource): Promise<CareerUpload> {
-   return decodeCareerUpload(await client.request({
+   // requestId 随 intent 持久化：结果未知时同编号重放（服务端 ClaimUpload 按
+   // requestId+intent hash 幂等恢复，FindUploadClaim 可查回既有 claim）
+   return writeRecoverable<CareerUpload>('upload', '简历上传', { name: file.name, size: file.size }, async (id, expected) =>
+     decodeCareerUpload(await client.request({
-     method: 'POST', path: '/api/v1/career/sources/upload', nativeFile: file,
+       method: 'POST', path: '/api/v1/career/sources/upload', nativeFile: file,
-     multipartFields: { requestId: newRequestId(), expectedRevision: String(revision()) },
-   }));
+       multipartFields: { requestId: id, expectedRevision: String(expected) },
+     })));
  }


─── apps/miniprogram/src/services/career-intent.ts:72-76 ───
[bug · medium] recoverableWrite 在发送前不检查同 kind 键下是否已有未对账的 intent：用户在第一次写入超时（intent 已落盘、页面呈恢复
Notice）后再次触发同类写入，若第二次成功或失败，store.write(key,…) 会静默覆盖第一条 intent——而第一条请求可能已在服务端落地，其 requestId
是唯一的对账/重放凭据，覆盖后永久丢失，用户从此无法确认那笔写入是否生效。同仓库的 CareerDesk.mutate（career-core/desk.ts:104）在服务层硬性抛
unresolved_action 阻止此场景，writeRecoverable 应保持同一口径（页面 Notice 只是提示，不构成硬封锁）。

  export async function recoverableWrite<T>(store: ControlledCareerStore, options: RecoverableWriteInput<T>): Promise<T> {
    const id = options.reuseId ?? newRequestId();
    const stamp = auth.scope.capture();
    const key = intentKeyFor(options.kind, stamp);
+   if (!options.reuseId && readStoredIntent(store, key)) {
+     // 与 CareerDesk.mutate 同口径：先对账/放弃上一笔结果未知的写入，再发新写
+     throw Object.assign(new Error(`已有结果未知的${options.describe}，请先对账或放弃恢复`), { code: 'unresolved_action' });
+   }
    try {


─── apps/miniprogram/src/adapters/career-platform.ts:201-205 ───
[maintainability · low] exportUserCopyPath 与 platform/files.ts 的
toUserCopyPath（:31-39）、downloadErrorCode 与 files.ts 的同名函数（:7-16）是近乎逐行重复的实现，且已经开始漂移：files.ts 版先查
size<1 再读、只解析顶层 code；本文件版无 size 预检、额外解析 error.code 嵌套。本适配器已经从 '../platform/files.ts' 导入
chooseDocument，却复制了其余 seam 而非复用——D3 结论（USER_DATA_PATH 唯一可删目录、扩展名决定 openDocument
类型、临时副本立即清理）一旦在实测中再更新，两处需要同步修改，极易出现只有一侧被修的情况。建议把 toUserCopyPath/downloadErrorCode（可带参数化前缀
wk-open-/wk-export-）下沉到 platform/files.ts 共享导出。

- function exportUserCopyPath(name: string): string {
-   // D3（T06 实测）：运行时临时文件不可删；副本必须落在 USER_DATA_PATH 且保留原始
-   // 扩展名（openDocument 靠扩展名识别类型），打开后立即删除我们创建的副本。
-   const env = (globalThis as { wx?: { env?: { USER_DATA_PATH?: string } } }).wx?.env;
-   const dir = env?.USER_DATA_PATH;
+ import { chooseDocument, toUserCopyPath } from '../platform/files.ts';
+ // files.ts 导出参数化版本：export function toUserCopyPath(name: string, prefix = 'wk-open-'): string
+ // 此处：const userCopy = toUserCopyPath(`career-material-v${grant.version}.${grant.format}`, 'wk-export-');


─── apps/web/src/career/SubmissionPage.tsx:288-288 ───
[bug · medium] 成功消息是死代码：acceptReceipt 在写入成功与 lookupReceipt 恢复成功两条路径都执行 setWritePhase('idle') 并
setMessage('投递已记录…')，而此处渲染条件要求 `message && writePhase !==
'idle'`，成功消息永远不会显示，用户只能靠时间线隐式推断结果。建议让成功消息可渲染（去掉 idle 排除条件，或引入显式 'saved' 阶段）。

-     {message && writePhase !== 'idle' ? <p className={writePhase === 'error' ? 'wk-submission__message wk-submission__message--error' : 'wk-submission__message'} role={writePhase === 'error' ? 'alert' : 'status'} aria-live="polite">{message}</p> : null}
+     {message ? <p className={writePhase === 'error' ? 'wk-submission__message wk-submission__message--error' : 'wk-submission__message'} role={writePhase === 'error' ? 'alert' : 'status'} aria-live="polite">{message}</p> : null}


─── apps/web/src/career/SubmissionPage.tsx:22-22 ───
[maintainability · medium] 本文件重复实现了 protocol.ts 已抽出的 ReceiptMismatchError / errorDetails /
newRequestId / isUncertainWrite 四个助手（protocol.ts 自述为该红线的唯一实现，reconciliation.tsx、OpportunityPage.tsx
均已迁移至共享版本），且本地副本已分叉：本地 isUncertainWrite 缺少 `instanceof ReceiptMismatchError` 短路，仅靠 catch 中分支顺序兜底；本地
errorDetails 未做 currentRevision 的 string→number 收窄。一旦 catch 分支顺序被重构，回执不匹配会被判 unknown 路由进回执恢复，重蹈
ocr2-067/082 已修复的红线违规。建议改为从 './protocol.ts' 导入并删除本地副本。

- class ReceiptMismatchError extends Error {}
+ import { ReceiptMismatchError, errorDetails, isUncertainWrite, newRequestId } from './protocol.ts'


─── apps/web/src/career/SubmissionPage.tsx:4-4 ───
[maintainability · medium] 同一文件混用两种导入风格：WeKnoraClient 走包入口 `@weknora/api-client`，而
CareerValidationError 等值/类型走跨包源码深相对路径（此模式同样出现在本组的 UsagePanel.tsx、reconciliation.tsx）。深路径绕过包入口与
exports 约定，目录重构或包改为消费构建产物时会集中断裂。本次 packages/api-client/src/index.ts 已新增 career 相关导出（createCareerApi
及部分类型），建议补齐所需类型与 CareerValidationError 后统一改走包入口。

- import { CareerValidationError, type MaterialExportReceipt, type MaterialVersionView, type RecordSubmissionInput, type SubmissionChannel, type SubmissionReceipt } from '../../../../packages/api-client/src/career.ts'
+ import { CareerValidationError, type MaterialExportReceipt, type MaterialVersionView, type RecordSubmissionInput, type SubmissionChannel, type SubmissionReceipt } from '@weknora/api-client'


─── apps/web/src/career/SubmissionPage.tsx:148-151 ───
[bug · low] materialExports 加载失败被静默
setExports([])，界面随后显示「尚无可投递版本：请先在材料区发布导出（双格式核验通过）…」——传输失败被表述为业务空态，会误导用户去重复发布导出。建议区分 error 与 empty
两种状态（例如引入 exportsState），失败时提示刷新重试而非引导发布。



─── apps/web/src/career/SubmissionPage.tsx:171-171 ───
[maintainability · low] occurredAtFromInput(occurredAt) 在同一对象字面量的条件判断与取值处各调用一次（versionChoice
两个分支皆然），同一输入被重复解析两次。建议先提升为局部常量（如 const occurredAtIso = occurredAtFromInput(occurredAt)）再展开。

-     ? { requestId, applicationId, channel, versionUnknown: true, expectedRevision: revision, ...(occurredAtFromInput(occurredAt) ? { occurredAt: occurredAtFromInput(occurredAt) } : {}), ...(note.trim() ? { note: note.trim() } : {}) }
+     ? { requestId, applicationId, channel, versionUnknown: true, expectedRevision: revision, ...(occurredAtIso ? { occurredAt: occurredAtIso } : {}), ...(note.trim() ? { note: note.trim() } : {}) }


─── apps/web/src/career/UsagePanel.tsx:27-28 ───
[documentation · low] 注释声称 usageAllowsChargedRun 是收费入口的「唯一准入门禁」且 RulePage enable 已消费，但实际只有
SearchPage 导入了该函数；RulePage 自行实现 enableRequiresEstimate（仅检查 usage.state.phase !== 'ready'，不含
wouldAdmit），两个收费入口判定口径不一致（SearchPage 要求 wouldAdmit，RulePage 额度耗尽时依赖后端 blocked_no_quota 兜底）。建议让
RulePage 复用 usageAllowsChargedRun 统一口径，或修正本注释如实描述两处判定，避免「唯一门禁」的契约失实。



─── apps/web/src/career/protocol.ts:37-37 ───
[maintainability · low] 白名单中的 'PAYLOAD_TOO_LARGE' 是死条目：career 后端统一写小写
`request_too_large`（internal/modules/career/handler.go:235/1251 等 16 处，HTTP 413），大写变体不会出现在 career
端点响应中，留着易让读者误以为线上存在两套码。建议删除；同时建议在注释中注明此清单是端点无关的基础契约，调用方需先处理各自端点的专用 4xx 码（如 revision_conflict /
submission_already_confirmed / export_not_submittable）再落入本函数（当前 409 会经 status
分支正确判为确定性失败，但依赖调用方先行处理才稳妥）。

-  if (['forbidden', 'invalid_request', 'idempotency_conflict', 'request_too_large', 'PAYLOAD_TOO_LARGE', 'not_found', 'unauthorized'].includes(error.code ?? '')) return false
+  if (['forbidden', 'invalid_request', 'idempotency_conflict', 'request_too_large', 'not_found', 'unauthorized'].includes(error.code ?? '')) return false


─── apps/web/src/career/reconciliation.tsx:83-83 ───
[bug · medium] 当 nextStatus.opportunityId !== opportunityId（回执与请求不匹配的确定性协议错误）时直接 return，面板将永久停留在
'loading' 态：无错误文案、无重试入口，确定性错误被伪装成瞬态加载，与本模块红线及同文件 fetchDiff 校验不匹配置 diffPhase 'error'
的既有做法不一致。建议将回执校验从短路条件中拆出，不匹配时 setPhase('error') 走既有错误渲染（含重试按钮）。

-    if (!active || !scopeController.isCurrent(requestScope.scope) || nextStatus.opportunityId !== opportunityId) return
+    if (!active || !scopeController.isCurrent(requestScope.scope)) return
+    if (nextStatus.opportunityId !== opportunityId) { setPhase('error'); return }


─── apps/web/src/career/reconciliation.tsx:6-6 ───
[maintainability · medium] 循环依赖：本文件从 OpportunityPage.tsx 导入 opportunityEvidencePath，而
OpportunityPage.tsx 又导入本文件的 OpportunityStatusPanel（其第 7 行）。ESM 循环引用的初始化顺序依赖打包器求值时机，重构（如改为 const
箭头函数、调整导入顺序）易踩 undefined。opportunityEvidencePath 是纯路径工具，建议下沉到共享模块（如 protocol.ts 或新建
paths.ts），OpportunityPage / ApplicationPage / InboxPage / SearchPage 一并改从该模块导入。

- import { opportunityEvidencePath } from './OpportunityPage.tsx'
+ import { opportunityEvidencePath } from './paths.ts' // opportunityEvidencePath 下沉至独立共享模块，消除与 OpportunityPage.tsx 的循环依赖


─── apps/web/src/career/reconciliation.tsx:221-221 ───
[bug · low] 当新旧快照选择同一 snapshotId 时，diff effect 将 diffPhase 置
'idle'，而此处兜底文案是「正在读取两个固定快照…」——「无需对比」被呈现为永久加载，误导用户等待。建议对同快照情形给出明确提示或引入独立状态。

-    </div> : diffPhase === 'error' ? <p role="status">快照对比暂时无法读取，可稍后重试；两个快照本身仍可分别打开。</p> : <p role="status">正在读取两个固定快照…</p>}
+    </div> : diffPhase === 'error' ? <p role="status">快照对比暂时无法读取，可稍后重试；两个快照本身仍可分别打开。</p> : oldObservation && newObservation && oldObservation.snapshotId === newObservation.snapshotId ? <p role="status">两次选择的是同一快照，请选择两个不同的快照进行对比。</p> : <p role="status">正在读取两个固定快照…</p>}


─── internal/modules/career/application.go:200-202 ───
[bug · high] 合并前生成的评估在合并后永久无法用于创建申请（与 "Pre-merge references keep resolving" 的设计意图矛盾）。推演：合并前对
candidate C 评估得到 E（evaluation.go 将 E.OpportunityID 写为快照当时归属 C）；reconciliation 合并 C→T 时迁移了
observations/snapshots/applications 的 opportunity_id（reconciliation.go:318-354），但**不迁移
career_evaluations**。此后无论客户端持旧引用（C）还是新引用（T）：快照经 merge-chain 回退解析到 T（snapshot.OpportunityID=T），而
E.OpportunityID 仍为 C，此处比对必然失败并返回
ErrInvalidRequest(400)——错误码语义为"请求非法"（客户端归为不可重试类），但真正原因是引用被合并，且该评估行永久不可用（无自愈路径，除非用户重新评估生成新评估
ID）。reconciliation_test.go 中合并后申请用的是合并后新评估（:693-696），未覆盖此场景。建议：比对时将 evaluation.OpportunityID 经
canonicalOpportunityID 解析后再与快照归属比较，或在合并事务中同步迁移 career_evaluations.opportunity_id（与
observations/snapshots/applications 对齐）。

- 		if evaluation.OpportunityID != snapshot.OpportunityID || evaluation.SnapshotID != input.SnapshotID {
+ 		evaluationOwner := canonicalOpportunityID(tx, scope, evaluation.OpportunityID)
+ 		if evaluationOwner != snapshot.OpportunityID || evaluation.SnapshotID != input.SnapshotID {
  			return ErrInvalidRequest
  		}


─── internal/modules/career/career_export.go:814-814 ───
[bug · medium] DeleteCareer 缺少空间级执行互斥，两个并发问题均可达：(1) 并发重试同一 request ID——初始 First 读取无锁，第二个调用若在 record
已存在（deleting/partial）后进入，会与第一个 runner 并发执行 runDeletionSteps 与 finalizeDeletion。各子步骤幂等，但
finalizeDeletion 会二次 bump profile revision、再次清空并重建 career_changes，导致 revision 双跳，且先持久化方写入的
receipt.Revision 与实际 profile revision 不一致（persistDeletionOutcome 的 status<>deleted
条件只防回退，不防已描述的双跑）。(2) purge 事务与 finalize 之间存在无锁窗口：purge 提交后、finalize 提交前，并发的
EditMaterial/ConfirmMaterial（其 CAS 在 finalize bump revision 前仍读到旧 revision 而通过）、ImportJD（完全不接触
profile 行锁）等可提交新的 career 行，随后回执宣称 status=deleted——残留行违背"只在全部步骤完成后才宣称已删除"的诚实性承诺。建议：删除执行期间持有 profile 行
FOR UPDATE（或以 careerDataDeletionRecord 行的确定性 CAS 抢占唯一执行权）串行化整个 run+finalize 流程，使与 profile-CAS
写路径及并发重试互斥。



─── internal/modules/career/career_export.go:307-310 ───
[performance · medium] ExportCareer 在持有 profile 行 FOR UPDATE 锁的事务内对约 20 张表做全量扫描（含单条上限 1MB 的 JD
原文快照），并将完整归档双份持久化（archive_body 与 receipt_body 内嵌同一 archive 载荷），HTTP 层同步内联返回（handler 直接 c.JSON 整个
receipt），全链路无体积/条数上限。后果：大空间下事务时长与数据量线性增长，期间阻塞同空间所有争用 profile
行锁的写入（mutate/EditMaterial/ConfirmMaterial/EvaluateOpportunity/CreateApplication 均锁该行；SQLite
上更是整库写锁），且单行 TEXT 体积翻倍、响应体可能超出网关/客户端承受力。个人空间虽为单成员，但快照数量与 1MB
上限乘积无界。建议：为归档设置总量上限（超限拒绝或分批导出）、将归档构建移出锁事务（先以 revision 快照点落 ledger 行再渲染），或至少让 receipt_body 不再内嵌完整
archive 而按 digest 引用 archive_body。



─── internal/modules/career/career_export.go:625-628 ───
[bug · low] sectionCount 将数据库查询错误吞掉并返回 0：存储层瞬时故障时，删除边界视图（数据主权确认界面）会把各 in-space 分区误报为"0
条数据"，用户可能基于"无数据可删"的误导性展示做出删除决定。计数是删除确认的关键输入，静默降级不安全。建议让闭包返回错误并由 CareerDeletionBoundary
上抛（或至少在视图中显式标记该计数不可用），而非以 0 冒充真实计数。

  		if err := query.Count(&total).Error; err != nil {
- 			return 0
+ 			// 将 sectionCount 改为返回 (int, error)，并让 CareerDeletionBoundary
+ 			// 在任一计数失败时返回错误，而不是以 0 冒充真实计数。
+ 			return 0, err
  		}
- 		return int(total)
+ 		return int(total), nil


─── apps/web/src/career/SubmissionPage.tsx:292-292 ───
[bug · medium] 版本回看头部展示的是页面级 materialId prop，而数据实际按记录绑定的 binding.materialId 拉取。MaterialVersionView
不含 materialId 字段，ApplicationPage 传入的 careerMaterialId 是 useState<string>()（MaterialPage 上报前为
undefined，且切换申请时不重置），因此会出现两种错误展示：careerMaterialId 未就绪时头部渲染「材料 + 空 code」；记录绑定的材料与页面 prop
不一致（或沿用上一申请的值）时展示错误材料编号。这违反本面板「诚实出处」的定位：回看的是 binding.materialId 的不可变版本，出处标识却来自另一个来源。建议在
reviewVersion 中把 binding（或至少 binding.materialId）与 versionDetail 一起存入 state，头部渲染绑定值而非页面 prop。

- <p className="wk-submission__version-meta">材料 <code>{materialId ?? ''}</code> · 固定档案修订 {versionDetail.pinnedEvidence.profileRevision} · 事实基准修订 {versionDetail.factBasisRevision} · 确认请求 <code>{versionDetail.requestId}</code> · <time dateTime={versionDetail.createdAt}>{versionDetail.createdAt}</time></p>
+ // reviewVersion 中：
+ // setVersionBinding(binding); setVersionDetail(next)
+ // 渲染时：材料 <code>{versionBinding?.materialId ?? ''}</code>


─── apps/web/src/career/SubmissionPage.tsx:17-17 ───
[bug · low] 哨兵值 '__unknown__' 与服务端签发的 exportId 共用同一字符串取值空间：api-client 的 validIdentifier
只要求非空字符串（packages/api-client/src/career.ts:50），若某个真实导出的 exportId 恰为 '__unknown__'，下拉里它作为普通选项出现，但
runWrite 会把 versionChoice === UNKNOWN_VERSION_CHOICE 判为显式未知，用户对该导出的显式绑定被静默改写为 versionUnknown:
true——正是本面板红线强调的「不得把绑定替换为推断未知」。建议不走魔法值判别：runWrite 先查 exports.find((receipt) => receipt.exportId ===
versionChoice)，命中即绑定、未命中且等于哨兵才归未知；或改用独立的 versionMode 状态（'export' | 'unknown'）与 exportId 选择分离。

- const UNKNOWN_VERSION_CHOICE = '__unknown__'
+ // runWrite 内先解析选中项，再构造 input：
+ const chosenExport = exports?.find((receipt) => receipt.exportId === versionChoice)
+ const input: RecordSubmissionInput = chosenExport
+  ? { requestId, applicationId, channel, versionUnknown: false, materialId: chosenExport.materialId, exportId: chosenExport.exportId, expectedRevision: revision, ... }
+  : { requestId, applicationId, channel, versionUnknown: true, expectedRevision: revision, ... }


─── apps/web/src/career/UsagePanel.tsx:80-80 ───
[maintainability · low] wk-career-usage-wrap 在本文件出现 4 次，但 usage.css（及全仓库检索）均未定义该类——usage.css 只样式化
.wk-career-usage 及其子元素。与其他 career 表面不一致：.wk-reconciliation / .wk-coverage 等根节点都有 box-sizing/width:
min(100%, 960px)/margin 等布局规则，这里的外层 section 完全无样式约束，类名成了死引用。建议在 usage.css 中补齐 wrap
布局（与其他表面口径一致），或删除该死类名。

-  return <section aria-label="额度预估" className="wk-career-usage-wrap"><Card className="wk-career-usage">
+ /* usage.css 补充： */
+ .wk-career-usage-wrap {
+   box-sizing: border-box;
+   width: min(100%, 960px);
+   margin: 0 auto 16px;
+ }


─── internal/modules/career/search_once.go:240-247 ───
[bug · medium] 准入与确定性失败的收费不一致：executeSearch 对空注册表返回终态 failed(no_vetted_sources)，reconcileUsageTx
会将该终态 settle，即一次从未发起任何网络抓取的确定性失败消耗 1 单位额度；而 triggerRulePeriod 对同一情形在准入前预检 len(sources)==0，走
blocked_no_vetted_sources 且完全不占用额度。当前生产注册表为空（emptySearchSourceRegistry），用户每次手动搜索都会必败并扣减 50/月 额度中的 1
单位。建议与规则路径对齐：注册表为空（结果在准入前即可确定）时跳过预占，或让 settle 豁免 no_vetted_sources 终态。

- 	// Quota admission is a narrow injected seam. A refusal is typed and leaves
- 	// no durable state, so the same request ID can be replayed later.
  	if o.searchQuotaGate == nil {
  		o.searchQuotaGate = passThroughSearchQuotaGate{}
  	}
+ 	// 与 triggerRulePeriod 一致：空注册表意味着结果确定为
+ 	// no_vetted_sources（零网络 I/O），不应为一个必然失败的运行预占并结算额度。
+ 	if len(o.ruleVettedSources()) > 0 {
- 	if err = o.searchQuotaGate.AdmitSearch(ctx, s, input.RequestID, input.Query); err != nil {
+ 		if err = o.searchQuotaGate.AdmitSearch(ctx, s, input.RequestID, input.Query); err != nil {
- 		return SearchOnceReceipt{}, err
+ 			return SearchOnceReceipt{}, err
+ 		}
  	}


─── internal/modules/career/usage.go:336-342 ───
[bug · low] 配额滞留：若执行者在 claimSearchRequest 提交后、commitSearch 提交前崩溃，且无人以同一 request ID
重试，career_searches 行将永久停留 claiming；此分支无条件 continue 持有预留（预留自身的租约仅在"无 search 行"分支被检查，search 行的 60s
claim 租约只对重试方生效，无后台回收）。该预留整个当月都计入 reserved，占用可用额度（月度窗口切换后才自然失效）。建议：search 行存在但仍是 claiming 且其
lease_until 已过期超过一个宽限期时同样释放；若后续真有执行者接管并到达终态，可经 reviveReleasedReservationTx 重占。



─── internal/modules/career/reconciliation.go:206-208 ───
[maintainability · low] 死代码：该提前退出条件包含 evidence.JobCode，但 JobCode 只在随后的 observation
循环中赋值，快照循环内它恒为空字符串，因此整个条件恒假、循环从不提前终止（结果仍正确，只是失去短路优化）。该恒假条件会误导读者以为存在提前退出保护。建议从条件中移除 JobCode
维度，或先在循环前解析 JobCode。

- 		if evidence.JobCode != "" && evidence.SnapshotID != "" && evidence.Title != "" && evidence.Company != "" && evidence.Location != "" && evidence.Batch != "" {
+ 		if evidence.SnapshotID != "" && evidence.Title != "" && evidence.Company != "" && evidence.Location != "" && evidence.Batch != "" {
  			break
  		}


─── internal/modules/career/reconciliation.go:691-696 ───
[performance · medium] SourceCoverage 只需要每个快照 Extracted JSON 中的 Location
字段，却无列裁剪地全量加载作用域内全部快照行——包括整页抓取原文 RawText（大文本列）。快照数随每次观察/规则触发持续累积，同步请求路径的内存与延迟随之线性放大；此前对 observations
的全量加载同理。建议用 Model+Select("extracted") 只取所需列；collectIdentityEvidence / requirementsChanged 也可做同样裁剪。

  	var snapshots []opportunitySnapshot
- 	err = o.db.WithContext(ctx).
+ 	err = o.db.WithContext(ctx).Model(&opportunitySnapshot{}).
+ 		Select("extracted").
  		Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Find(&snapshots).Error
  	if err != nil {
  		return CareerCoverageView{}, err
  	}


─── internal/modules/career/search_once.go:519-523 ───
[bug · low] truncated 误报：上限检查在链接判断之前执行。当来源恰好包含 maxSearchResultsPerSource 条链接、其后仍存在任意非链接
token（正文文字、标点等）时，下一次迭代会先命中 len(rows)>=max 并置
truncated=true，向用户虚报"来源返回了更多条目被截断"——而实际上没有任何链接被丢弃。对一个以诚实披露为核心契约的系统，应在真正丢弃一条新链接时才置位。

  	for _, field := range strings.Fields(text) {
+ 		if !strings.HasPrefix(field, "http://") && !strings.HasPrefix(field, "https://") {
+ 			continue
+ 		}
  		if len(rows) >= maxSearchResultsPerSource {
  			truncated = true
  			break
  		}


─── internal/modules/career/search_once.go:605-618 ───
[bug · low] 未知结果契约不一致：runImportTransaction 在 ctx 取消时返回原始 context 错误，此处被原样透传，handler writeError 将落入默认
500 internal；而同模块的 ReconcileOpportunities、SetReminder、mutate 等写路径都把 ctx.Err()!=nil 映射为
OutcomeUnknownError（504 outcome_unknown + requestId）。提交与取消存在竞态（事务可能在取消生效瞬间已落库），本路径的结果同样是"未知、可用同一
request ID 恢复"，应保持一致的类型化恢复契约。

  	if errors.Is(err, errSearchClaimLost) || isSQLiteBusy(err) {
- 		// Our claim was taken over after a lease expiry, or writer-lock
- 		// contention exhausted the retry budget. Reconcile once with the
- 		// original request ID instead of guessing the outcome.
- 		replay, found, lookupErr := o.awaitSearchTerminal(context.WithoutCancel(ctx), s, input.RequestID, fingerprint, 0)
- 		if lookupErr != nil {
- 			return SearchOnceReceipt{}, lookupErr
- 		}
- 		if found {
- 			return replay, nil
+ 		// ... 原有 reconcile 逻辑不变 ...
- 		}
+ 	}
+ 	if ctx.Err() != nil {
  		return SearchOnceReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
  	}
  	return SearchOnceReceipt{}, err


─── internal/modules/career/submission.go:455-457 ───
[maintainability · low] 语义混叠：对"提交记录存在但版本未知"（versionUnknown=true 是合法记录形态）返回
ErrSubmissionNotFound，把存在性与版本未确认两个语义合并为同一 404 契约。内部 preparation 路径已用
PreparationVersionUnknownError{SubmissionRecorded:true} 精确区分这两种状态。该导出方法作为下游 preparation
的版本锚定入口，应同样区分，避免调用方无法向用户提示"已确认投递但版本未知"（与"该投递不存在"需要不同的产品应对）。



─── internal/modules/career/preparation.go:311-319 ───
[bug · medium] 预留阶段（attemptPreparationReserve）缺少竞态回放分支：两个并发请求携带同一 request ID 时，后提交方的 Create
会以唯一约束冲突失败（career_preparations 的 (tenant_id,user_id,request_id) 唯一索引），isReceiptRaceError
可命中该错误，但此循环既不回放胜者的存储回执、也不返回 OutcomeUnknown，而是把原始 DB 错误直接上抛（HTTP 层 writeError 无匹配项 → 500
internal）。同模块的 EditMaterial/ConfirmMaterial（material.go:440/602）、attemptProgressWrite
循环（progress.go）与 submission.go:169-183 在完全相同的场景都先做 replay 再决定返回值。客户端虽可用同一 request ID
重试恢复，但当下响应违反了本模块"同一 request ID 决定结果、未知结果必须类型化"的一致契约。建议在 busy 分支后补充 race 回放（not-found 时结果真不确定，也应为
OutcomeUnknown）。另外 finalizePreparation 中 replayPreparationReceipt 的 lookupErr 同样被原样上抛，宜一并处理。

- 		if isSQLiteBusy(txErr) && attempt < progressWriteBusyRetries {
- 			time.Sleep(progressWriteBackoff)
- 			continue
- 		}
  		if isSQLiteBusy(txErr) {
+ 			return PreparationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
+ 		}
+ 		if isReceiptRaceError(txErr) {
+ 			stored, found, lookupErr := o.replayPreparationReceipt(operationCtx, s, input.RequestID, fingerprint)
+ 			if errors.Is(lookupErr, ErrIdempotencyConflict) {
+ 				return PreparationReceipt{}, lookupErr
+ 			}
+ 			if lookupErr != nil || !found {
- 			return PreparationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
+ 				return PreparationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
+ 			}
+ 			return stored, nil
  		}
  		return PreparationReceipt{}, txErr
  	}


─── internal/modules/career/application.go:307-314 ───
[bug · low] 竞态回放查找用原始请求 ctx 执行，且 lookupErr 被原样上抛：当事务以可被 isReceiptRaceError 命中的错误失败（如 SQLite
"database is locked" 或唯一约束冲突）而客户端恰在此时断开（ctx 已取消）时，replayApplication 会立即以 context.Canceled
失败并以原始错误形式漏出（writeError 无匹配 → 500），而本文件下方 "if ctx.Err() != nil → OutcomeUnknown" 的兜底在该路径永远不可达。对照
mutate（office.go 中对 replayReceipt 的 lookupErr 先映射 context.Canceled/DeadlineExceeded 为
OutcomeUnknown）与 submission.go:173-179、progress.go 的同名处理，这里应把取消的查找映射为类型化 OutcomeUnknown（504
outcome_unknown），保证"未知创建结果用同一 request ID 恢复"契约在断连窗口内依然成立。

  		if isReceiptRaceError(err) {
  			// A concurrent writer may have committed this request (identical or
  			// different content) or the same job/batch. Re-read under the same
  			// scope to decide; only the stored state is ever returned.
  			replay, found, lookupErr := o.replayApplication(ctx, scope, input.RequestID, fingerprint)
  			if lookupErr != nil {
+ 				if ctx.Err() != nil || errors.Is(lookupErr, context.Canceled) || errors.Is(lookupErr, context.DeadlineExceeded) {
+ 					return ApplicationReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
+ 				}
  				return ApplicationReceipt{}, lookupErr
  			}


─── internal/modules/career/material.go:431-444 ───
[bug · low] EditMaterial 与 ConfirmMaterial（约 602 行处完全相同的分支）的竞态回放查找都用原始请求 ctx 执行且 lookupErr
原样上抛：当事务错误被 isReceiptRaceError 命中（如 SQLite "database is locked"
或唯一约束冲突）而客户端恰在回放前断开时，replayMaterialReceipt 立即以 context.Canceled 失败并被原样返回（writeError → 500），下方 "if
ctx.Err() != nil → OutcomeUnknown" 在该路径不可达。对照 mutate（office.go 对 replayReceipt lookupErr 先映射
context.Canceled/DeadlineExceeded）与 submission.go:173-179、progress.go 的处理，应把取消的查找映射为类型化
OutcomeUnknown（504 outcome_unknown），使断连窗口内幂等回放契约保持一致。

- 		if errors.Is(err, ErrIdempotencyConflict) || errors.Is(err, ErrInvalidRequest) ||
- 			errors.Is(err, ErrOpportunityNotFound) || errors.Is(err, ErrMaterialNotFound) ||
- 			errors.Is(err, ErrMaterialClaimUnconfirmed) {
- 			return MaterialReceipt{}, err
- 		}
- 		var revisionConflict *RevisionConflictError
- 		if errors.As(err, &revisionConflict) {
- 			return MaterialReceipt{}, err
- 		}
  		if isReceiptRaceError(err) {
  			replay, found, lookupErr := o.replayMaterialReceipt(ctx, s, input.RequestID, fingerprint)
  			if lookupErr != nil {
+ 				if ctx.Err() != nil || errors.Is(lookupErr, context.Canceled) || errors.Is(lookupErr, context.DeadlineExceeded) {
+ 					return MaterialReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
+ 				}
  				return MaterialReceipt{}, lookupErr
  			}


─── internal/modules/career/search_rule.go:560-563 ───
[bug · high] 规则触发的 requestID（"rule:<id>:<period>"）是系统生成的固定值，但 SearchOnce
的幂等指纹（searchFingerprint）把调用方采样到的 ExpectedRevision 也纳入意图，而 revision 只是首次 claim 的 CAS
检查项、并非搜索意图的一部分。一旦某次触发的 claim 已提交（searchRecord 停留在 claiming）但未到达终态——进程崩溃、SQLite busy 耗尽 6s
重试预算、commitSearch 事务因 ctx 取消而回滚等都会造成这种状态——LastPeriod 不推进，下次触发仍复用同一 requestID。此时只要用户执行了任何推进 profile
revision 的操作（档案确认、材料、申请等写入），重试的 fingerprint 必与已存行不同：replaySearch 与 claimSearchRequest 都直接返回
ErrIdempotencyConflict，claiming 行没有按存储指纹接管的路径，revision 也不可能回退。结果：该规则在该 period 永久卡死，每次
TriggerDueRules 都因这条规则报错中止（并连带阻塞其余规则），同时其 admission 预留整月滞留（与已确认的配额滞留问题叠加）。建议：规则路径捕获
ErrIdempotencyConflict 后改用已存 searchRecord 的存储指纹进行重放/接管（或提供忽略 revision 的 SearchOnce 内部变体，将 revision
仅保留为首次 claim 的 CAS 检查）；若直接从指纹中移除 ExpectedRevision，需注意存量已落库指纹的兼容。

  		var searchErr error
  		receipt, searchErr = o.SearchOnce(ctx, SearchOnceInput{
  			RequestID: requestID, Query: rule.Query, ExpectedRevision: head.Revision,
  		})
+ 		if errors.Is(searchErr, ErrIdempotencyConflict) {
+ 			// 同一 period 的 requestID 已按旧采样 revision 落过 claim：
+ 			// 改用已存 searchRecord 的存储指纹重放/接管，绝不能让规则永久卡死。
+ 			receipt, searchErr = o.replayRulePeriodSearch(ctx, s, requestID, rule.Query)
+ 		}


─── internal/modules/career/search_rule.go:515-521 ───
[bug · medium] TriggerDueRules 对每条到期规则串行触发，但任何一条 triggerRulePeriod
返回错误（ErrIdempotencyConflict、RevisionConflictError、OutcomeUnknownError、ErrAdmissionUnavailable、数据库错误等
）都直接 return nil, triggerErr：已成功触发的规则 summaries 被整体丢弃，排在其后的到期规则本轮完全不执行（它们的 NextDueAt
未变，只能等待下一次调用方再触发）。triggerRulePeriod 内部只把 ErrSearchQuotaRefused 降级为可记录的 blocked
run，其余错误都会中止整批。建议：单条规则失败应记录后 continue（或收集为逐规则错误一并返回），让一次触发批次尽量完整，避免一条规则的异常吞掉整批调度结果。

  	for _, rule := range due {
  		outcome, triggerErr := o.triggerRulePeriod(ctx, s, rule, now)
  		if triggerErr != nil {
- 			return nil, triggerErr
+ 			// 单条规则失败只影响自身：记录后继续触发其余到期规则，
+ 			// 而不是丢弃已完成的 outcomes 并跳过后续规则。
+ 			outcome = RuleRunSummary{
+ 				RuleID: rule.ID, Period: rule.LastPeriod + 1,
+ 				RequestID: fmt.Sprintf("rule:%s:%d", rule.ID, rule.LastPeriod+1),
+ 				Status: RuleRunStatusFailed,
+ 			}
+ 			outcomes = append(outcomes, outcome)
+ 			continue
  		}
  		outcomes = append(outcomes, outcome)
  	}


─── packages/api-client/src/career.ts:1326-1326 ───
[bug · high] 红线违规：文件头注释（CareerValidationError 段）明确「本地同步校验拒绝时请求从未发出，必须与网络层裸 TypeError 区分，否则会被页面侧
isUncertainWrite 判 unknown 误入回执恢复」，但该异常只在 recordSubmission 使用；本方法及
editMaterial/confirmMaterial/publishMaterial/appendProgress/correctProgress/generatePreparation/setR
eminder/exportCareer/deleteCareer/setRule/searchOnce/importUrl/importOpportunity/reconcileOpportunit
ies/evaluateOpportunity/revokeMaterialExport/materialExportSignedURL/upload 等全部写方法的本地预检仍抛裸
TypeError。已核实页面侧 apps/web/src/career/protocol.ts 的 isUncertainWrite（及 MaterialPage 等内联版本）对无
code/status 的错误一律 return true（判 unknown 进回执恢复），仅 SubmissionPage 有 instanceof CareerValidationError
判断。应统一改抛 CareerValidationError。

-    if (!input.requestId.trim() || !input.opportunityId.trim() || !input.snapshotId.trim() || !input.evaluationId.trim() || !input.batchIdentity.trim() || !Number.isSafeInteger(input.expectedRevision) || input.expectedRevision < 0) throw new TypeError('application request identifiers and revision must be valid')
+    if (!input.requestId.trim() || !input.opportunityId.trim() || !input.snapshotId.trim() || !input.evaluationId.trim() || !input.batchIdentity.trim() || !Number.isSafeInteger(input.expectedRevision) || input.expectedRevision < 0) throw new CareerValidationError('application request identifiers and revision must be valid')
+ // 其余写方法（editMaterial/confirmMaterial/publishMaterial/appendProgress/correctProgress/generatePreparation/setReminder/exportCareer/deleteCareer/setRule/searchOnce/upload 等）的本地预检同步改抛 CareerValidationError


─── packages/api-client/src/career.ts:1-1 ───
[maintainability · medium] 跨包边界违规：通过相对路径 '../../career-core/src/contracts.ts' 引用另一个 workspace 包，且
packages/api-client/package.json 的 dependencies 只有 @weknora/contracts 与 @weknora/domain、未声明
@weknora/career-core（其 package.json 已定义 exports "./contracts"）。web 侧 203 个文件均以包名
'@weknora/api-client' 导入，api-client 自身对 contracts 也是包名导入，唯独此处走文件系统相对路径：依赖图不完整（pnpm 不会把 career-core
纳入 api-client 的依赖保证），且 api-client 单独打包发布或 career-core 目录调整时将直接断裂。

- import type { CareerAction, CareerChangeSet, CareerDocumentSource, CareerFact, CareerProposal, CareerReceipt, CareerSource, CareerUpload, CareerView, Evaluation, EvaluationReceipt, OpportunityEvidence, OpportunityImportInput, OpportunityReceipt, OpportunitySource, OpportunityStatus } from '../../career-core/src/contracts.ts'
+ // packages/api-client/package.json 增加: "@weknora/career-core": "workspace:*"
+ import type { CareerAction, CareerChangeSet, CareerDocumentSource, CareerFact, CareerProposal, CareerReceipt, CareerSource, CareerUpload, CareerView, Evaluation, EvaluationReceipt, OpportunityEvidence, OpportunityImportInput, OpportunityReceipt, OpportunitySource, OpportunityStatus } from '@weknora/career-core/contracts'


─── packages/api-client/src/index.ts:211-211 ───
[maintainability · medium] 同 career.ts：再导出 career-core 契约类型同样使用相对路径
'../../career-core/src/contracts.ts' 跨包引用，绕过 workspace 依赖边界（api-client 未声明 @weknora/career-core
依赖）。应随 career.ts 一并改为包名导入 '@weknora/career-core/contracts' 并在 package.json 声明 workspace:* 依赖，两处保持一致。

- export type { CareerAction, CareerView, CareerFact, CareerProposal, CareerReceipt, CareerChangeSet, CareerChange, CareerSource } from '../../career-core/src/contracts.ts';
+ export type { CareerAction, CareerView, CareerFact, CareerProposal, CareerReceipt, CareerChangeSet, CareerChange, CareerSource } from '@weknora/career-core/contracts';


─── packages/api-client/src/career.ts:1255-1257 ───
[maintainability · medium] 防线缺口：open()/list()/changes() 是全文件仅有的三个把响应直接 as
断言、不经解码器校验的端点（decodeCareerExportReceipt 的注释也自认这一点），与本文件「解码器在进 UI 前拒绝编造载荷」的既定防线不一致；career-core 未提供
decodeCareerView，但 decodeExportedProfile（facts/proposals/revision 逐项校验）已经证明视图完全可解码，可提取为
decodeCareerView 复用，CareerChangeSet 同理。畸形/编造的档案或变更响应会以 as 断言直达页面状态。

-   async open(signal?: AbortSignal): Promise<CareerView> { return await request({ method: 'GET', path: '/api/v1/career/open', ...(signal ? { signal } : {}) }) as CareerView },
-   async list(signal?: AbortSignal): Promise<CareerView> { return await request({ method: 'GET', path: '/api/v1/career/list', ...(signal ? { signal } : {}) }) as CareerView },
-   async changes(since: number, signal?: AbortSignal): Promise<CareerChangeSet> { return await request({ method: 'GET', path: `/api/v1/career/changes?since=${encodeURIComponent(String(since))}`, ...(signal ? { signal } : {}) }) as CareerChangeSet },
+   async open(signal?: AbortSignal): Promise<CareerView> { return decodeCareerView(await request({ method: 'GET', path: '/api/v1/career/open', ...(signal ? { signal } : {}) })) },
+ // 将 decodeCareerExportReceipt 内联的 decodeExportedProfile 提取为导出的 decodeCareerView/decodeCareerChangeSet，open/list/changes 统一经解码器返回


─── packages/api-client/src/career.ts:1471-1471 ───
[bug · low] 字节语义错位：input.note.length 是 UTF-16 码元数，而后端 submission.go 用 len(input.Note) 按字节计数（同为 4096
上限）。含 CJK 的备注（每汉字 3 字节/1 码元）在 1366–4096 字之间时客户端放行、服务端 400 拒绝，本地校验形同虚设且错误文案 'must not exceed 4096
bytes' 误导用户。建议按编码后字节数校验。

-    if (input.note !== undefined && input.note.length > maxSubmissionNoteBytes) throw new CareerValidationError('submission note must not exceed 4096 bytes')
+    if (input.note !== undefined && new TextEncoder().encode(input.note).length > maxSubmissionNoteBytes) throw new CareerValidationError('submission note must not exceed 4096 bytes')


─── packages/api-client/src/career.ts:261-261 ───
[maintainability · low] 重复维护：searchCoverageFailureCodes 中除 'source_misconfigured' 外的 10 个码与
opportunityFailureCodes 逐字重复。后端新增或改名任一 fetch 失败码时客户端易单边漂移，导致合法响应被解码器误拒。建议抽取共享常量相互引用。

- const searchCoverageFailureCodes: SearchCoverageFailureCode[] = ['source_misconfigured', 'login_required', 'access_blocked', 'not_found', 'timeout', 'source_unverified', 'unsupported_content', 'empty_content', 'response_too_large', 'network_error', 'redirect_disallowed']
+ const sharedSourceFetchFailureCodes = ['login_required', 'access_blocked', 'not_found', 'timeout', 'source_unverified', 'unsupported_content', 'empty_content', 'response_too_large', 'network_error', 'redirect_disallowed'] as const
+ const searchCoverageFailureCodes: SearchCoverageFailureCode[] = ['source_misconfigured', ...sharedSourceFetchFailureCodes]
+ const opportunityFailureCodes: OpportunityFailureCode[] = [...sharedSourceFetchFailureCodes]


─── packages/api-client/src/career.ts:1355-1355 ───
[maintainability · low] 死代码：`operation.trim() !== operation` 恒为 false——若 operation !== 'search_once'
则第一个条件已短路抛出，能走到第二个条件时 operation 必为 'search_once'（trim 后不变）。请删除冗余条件。

-    if (operation !== 'search_once' || operation.trim() !== operation) throw new TypeError('usage operation must be search_once')
+    if (operation !== 'search_once') throw new TypeError('usage operation must be search_once')


─── packages/api-client/src/career.ts:943-943 ───
[style · low] 嵌套三元表达式（团队规范禁止）：rawFactKeys 的解析使用两层嵌套三元，维护时易引入错误。建议展开为显式分支。

-  const factKeys = rawFactKeys === null || rawFactKeys === undefined ? [] : Array.isArray(rawFactKeys) ? rawFactKeys : undefined
+  let factKeys: unknown[] | undefined
+  if (rawFactKeys === null || rawFactKeys === undefined) factKeys = []
+  else if (Array.isArray(rawFactKeys)) factKeys = rawFactKeys


─── packages/api-client/src/user-favorites.ts:33-33 ───
[maintainability · low]
异常类型不一致：本文件全部校验函数（record/required/text/numberValue/resourceType/successfulData/expectSuccess 等）抛普通
Error，而同包 career.ts 的约定是本地校验抛 TypeError（构造失败）/CareerValidationError（写预检失败），消费方无法用统一 instanceof
方式识别「请求未发出」的确定失败。另外 user_id/tenant_id/created_at 等 snake_case 字段与包内其余 camelCase 视图不一致（虽有注释说明镜像后端
DTO，仍建议在解码层转换）。

-   if (typeof value !== 'string' || value.trim() === '') throw new Error(`${path} must be a non-empty string`);
+   if (typeof value !== 'string' || value.trim() === '') throw new TypeError(`${path} must be a non-empty string`);


─── packages/api-client/src/career.ts:1431-1431 ───
[maintainability · low] 授权过期未预检：materialExportDownload 在发起兑换请求前未检查 grant.expiresAt（web 端
MaterialPage 亦未检查，搜索无 expiresAt 处理），过期 grant 只能依赖服务端 export_grant_invalid(404)
兜底，错误语义丢失且浪费一次请求。digest 校验已由 web 调用方以 crypto.subtle 补做（小程序端未使用此方法，库内不依赖 crypto.subtle 可理解），但
expiresAt 预检仅用 Date.now() 即可移植完成。

+    if (Date.now() / 1000 >= grant.expiresAt) throw new Error('material export grant has expired')
     const response = await binaryRequest({ method: 'GET', path: `/api/v1/career/materials/${encodeURIComponent(grant.materialId)}/exports/${encodeURIComponent(grant.exportId)}/download?format=${grant.format}&expires=${grant.expiresAt}&signature=${encodeURIComponent(grant.signature)}`, ...(signal ? { signal } : {}) })


─── packages/api-client/src/career.ts:1437-1437 ───
[other · low] 部署兼容性风险：以 DELETE 方法携带 JSON 请求体（requestId/expectedRevision）。已核实服务端 handler.go
RevokeMaterialExport 确实经 decodeStrictJSON 从 body 绑定（客户端与服务端自洽），但 HTTP 生态中部分网关/反向代理会丢弃或拒绝 DELETE
body，届时撤销操作在该部署链路下会因 body 解析失败而整体不可用且难排查。建议与后端协同将 requestId/expectedRevision
迁移为查询参数，或至少在部署文档中注明链路需支持 DELETE body。



─── apps/web/src/router.tsx:719-719 ───
[bug · medium] careerRoute 是本文件中唯一一个 lazy 页面组件没有包裹 Suspense + RoutePending 的平台子路由（同批新增的
careerSearchRoute / careerRulesRoute / careerOpportunityRoute / careerEvaluationRoute
以及其余所有平台子路由均有各自的边界）。虽然 rootRoute 与 platformRoute 的 shellPage 各有祖先 Suspense 边界可以兜底（不会崩溃），但懒加载
CareerPage chunk 首次挂起时会一路冒泡到 platformRoute 包住整个 PlatformShell 的那个 Suspense——导致首次客户端导航到
/platform/career 时整个 PlatformShell 被卸载并回退为 RoutePending，壳层内部状态（侧栏展开等）全部丢失，chunk 加载完成后壳层重新挂载、内部
effects 重跑，表现为整页闪烁。建议与兄弟路由保持一致，包一层 Suspense。

-     component: (): ReactNode => <CareerPage client={client} scopeController={scopeController} userId={scopeController.current().scope.userId} />,
+     component: (): ReactNode => (
+       <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>{' '}
+         <CareerPage client={client} scopeController={scopeController} userId={scopeController.current().scope.userId} />
+       </Suspense>
+     ),


─── apps/web/src/router.tsx:716-718 ───
[documentation · low] 「Octop M2 expert-template catalog; …(routes_expert.go guard)」这条注释描述的是
expertsRoute 的鉴权语义，career 路由插入后它现在悬挂在 careerRoute 定义上方（expertsRoute 实际在 772 行），语义错位会误导后续维护者将 expert
鉴权说明错误关联到 career 路由。建议把注释移回 expertsRoute 定义处。

    const careerRoute = createRoute({
      getParentRoute: () => platformRoute,
      path: 'career',
+     // （expert 注释应移回下方 expertsRoute 定义处）


─── apps/web/src/routes.tsx:58-59 ───
[maintainability · medium] resolveRoute 与 router.tsx 的 careerOpportunityRoute 对同一 URL
形态给出了不一致的契约：这里要求 snapshotId 必须非空（缺失/纯空白即返回 not-found），而 router.tsx 端 `?? ''` 容忍空值、由
OpportunityEvidencePage 渲染友好的 invalid 状态（「链接缺少有效的快照编号」）。两套语义分叉已产生可观察的守卫行为差异：缺失 snapshotId 时本分支落入
guardRoute 的 not-found 处理（routes.tsx:201-203），已认证但无租户的用户会被直接放行进入页面；而携带 snapshotId 时同一用户会走
workspace-required 检查被重定向到 /onboarding/workspace。建议两侧统一：既然页面端已内建 invalid 状态兜底，此处可放宽为容忍缺失并继续返回
career-opportunity 匹配（保持守卫链一致），或反向在 router.tsx 端对空 snapshotId 走 notFound。

-     const snapshotId = query.get('snapshotId')?.trim();
-     return opportunityId && snapshotId ? { kind: 'career-opportunity', path, opportunityId, snapshotId } : { kind: 'not-found', path };
+     const snapshotId = query.get('snapshotId')?.trim() ?? '';
+     return opportunityId ? { kind: 'career-opportunity', path, opportunityId, snapshotId } : { kind: 'not-found', path };


─── apps/web/src/analytics/AnalyticsPage.tsx:355-356 ───
[maintainability · medium] tdesign Tabs.TabPanel 默认 destroyOnHide=false：非激活面板以 display:none 常驻
DOM，与原 Radix TabsContent 仅挂载激活项的语义不同。切到「用量」页签时 charts 面板内 6 个 recharts 图表容器宽度归零，切回后依赖
ResponsiveContainer 的 ResizeObserver 重新测量，存在图表间歇性空白/尺寸错误的迁移回归风险；usage
表格（data-testid="usage-by-user-table"）DOM 也会常驻。建议对 usage 面板显式设置 destroyOnHide
对齐原按需挂载语义，或至少人工验证两个页签往返切换后所有图表渲染正常。



─── apps/web/src/analytics/analytics-u.css:74-75 ───
[maintainability · medium] 原 Tailwind 语义 token（bg-accent / text-accent / hover:bg-accent-wash /
bg-surface / focus:border-accent）在编译产物中引用 apps/web/src/styles.css:457-463 已定义的 CSS
变量（--color-accent、--color-accent-wash、--color-surface 等），本次平移为字面量
#07c05f、rgba(7,192,95,0.08)、#ffffff
后丢失了变量跟随能力：主题/品牌色覆写对该页失效，后续调色需在多处同步修改。仓库已有先例采用变量+回退形式（administration-u.css:220、settings-wrapper.css、
kb-list.td.css 的 var(--color-accent, #07c05f)），建议对齐写法，例如 background-color: var(--color-accent,
#07c05f)、hover 底色用 var(--color-accent-wash, rgba(7,192,95,0.08))。



─── apps/web/src/analytics/analytics-u.css:242-243 ───
[style · low] 两条声明挤在同一行：`border-bottom-width: 1px;border-color: #eef1f5;`（.wk-anl-2 处的
`1px;border-color: #e7e7ea` 同样问题）。另外 .wk-anl-an-usage-num 在文件中部（text-align:
right）与尾部（font-variant-numeric）重复定义两处，建议合并为单处定义并拆分挤行声明，保持与文件内其他规则一致的格式，避免后续 lint/格式化工具产生大面积噪音 diff。



─── apps/web/src/data-sources/data-sources-u.css:770-770 ───
[bug · medium] `.wk-ds-self-start`（justify-self:start）仍只定义在懒加载的 knowledge-u.css:700（仅
KnowledgeGraphPage 导入），未像 .wk-dsui-card 一样按 R1-04/31 迁入本域。DataSourcesPage.tsx:441 在 prereq
setup-guide 中使用该类，直接进入数据源页（未访问知识图谱路由）时规则缺失，grid 容器（.wk-ds-8）内该链接会被拉伸为整行，与删除前的 justify-self-start
表现不一致。建议在本域 CSS 补上同名规则。

  .wk-dsui-card { border-radius: 8px; border: 1px solid #e7e7e7; background-color: #ffffff; padding: 16px; }
+ /* 同 R1-04/31：原唯一定义在懒加载的 knowledge-u.css，迁入本域 */
+ .wk-ds-self-start { justify-self: start; }


─── apps/web/src/data-sources/DataSourcesPage.tsx:456-456 ───
[bug · medium] TDesign Checkbox 的根节点本身就是 <label>，外层再包 <label> 会产生 label 嵌套——HTML 规范禁止 label 后代
label，且点击时存在双重激活（复选框点击无效/需点两次）的风险。此处外层 label 已不含文本（文本移入 Checkbox 的 label prop），完全冗余，建议直接去掉外层
<label>。同理，删除弹窗中的 <label className="wk-ds-21"><Checkbox ... label={purgeLabelText}/></label>（约 484
行）也应改为 <div className="wk-ds-21">。

- <label><Checkbox checked={form.deletions} onChange={(checked) => updateForm('deletions', checked)} label={t('dataSource.syncDeletions')} /></label>
+ <Checkbox checked={form.deletions} onChange={(checked) => updateForm('deletions', checked)} label={t('dataSource.syncDeletions')} />


─── apps/web/src/data-sources/DataSourcesPage.tsx:484-484 ───
[bug · medium] 删除弹窗中 Checkbox（根节点为 <label>）外又包了一层 <label className="wk-ds-21">，构成 label 嵌套（HTML
无效，且有双重激活/点击失效风险）。外层元素仅为承载 wk-ds-21 的 flex 布局，改为 <div> 即可。

- <label className="wk-ds-21"><Checkbox checked={deletePurge} disabled={deleteSubmitting} onChange={(checked) => setDeletePurge(checked)} label={purgeLabelText} /></label>
+ <div className="wk-ds-21"><Checkbox checked={deletePurge} disabled={deleteSubmitting} onChange={(checked) => setDeletePurge(checked)} label={purgeLabelText} /></div>


─── apps/web/src/data-sources/DataSourcesPage.tsx:0-0 ───
[bug · medium] 迁移到 TDesign 后 settings 字段的 required={!field.optional} 被移除，而 save() 对
name/type/schedule、凭据、gitlab project_id 都有 JS 兜底校验，唯独 settings 字段没有——VUE_SETTINGS_FIELDS 中 rss 的
feed_urls（唯一非 optional 字段）现在可留空直接提交，创建出无效数据源（旧代码会被原生 required 拦截）。TDesign Textarea 会将未知属性透传到原生
textarea，建议恢复 required，或在 save() 中补充校验。

- <Textarea autosize={{ minRows: 3, maxRows: 3 }} placeholder={field.placeholder || t('dataSource.credential.inputPlaceholder')} value={credentialValue(form.settingsText, field.key)}
+ <Textarea autosize={{ minRows: 3, maxRows: 3 }} required={!field.optional} placeholder={field.placeholder || t('dataSource.credential.inputPlaceholder')} value={credentialValue(form.settingsText, field.key)}


─── apps/web/src/data-sources/DataSourcesPage.tsx:0-0 ───
[maintainability · low] 残留空字符串 className=""，无任何作用，建议删除。

- size="small" className="" onClick={() => { setCredentialStep((current) => credentialStepReducer(current, 'cancel-replace'));
+ size="small" onClick={() => { setCredentialStep((current) => credentialStepReducer(current, 'cancel-replace'));


─── apps/web/src/platform/platform-u.css:47-51 ───
[bug · high] 无效 CSS：`calc()` 内 `+`/`-` 运算符两侧必须有空白，`calc(100vw-32px)` 解析失败，整条 `max-width` 声明会被浏览器丢弃。原
Tailwind 任意值 `max-w-[calc(100vw-32px)]` 会被 Tailwind 自动归一化为 `calc(100vw -
32px)`（合法），手写平移丢失了这层归一化——结果是在窄视口（<672px）下 640px 宽的命令面板不再被钳制，会横向溢出屏幕。

  .wk-cmdk-6 {
    display: flex;
    max-height: 70vh;
    width: 640px;
-   max-width: calc(100vw-32px);
+   max-width: calc(100vw - 32px);


─── apps/web/src/platform/platform-u.css:347-348 ───
[bug · high] 同 `.wk-cmdk-6`：`max-width: calc(100vw-32px)` 缺少运算符两侧空白，是无效 CSS，浏览器会丢弃该声明，窄视口下 420px
检索抽屉不再被 `100vw - 32px` 钳制。

    width: 420px;
-   max-width: calc(100vw-32px);
+   max-width: calc(100vw - 32px);


─── apps/web/src/platform/PlatformShell.tsx:70-72 ───
[bug · medium] 键盘分支永不导航：对 React 合成 KeyboardEvent 而言 `button` 属性不存在（`undefined`），`undefined !== 0` 恒为
true，导致提前 return——账号卡 `role="button"` 的 `onKeyDown`（Enter）调用（第 1268 行）实际是死代码，Enter 键永远无法打开个人设置（a11y
回归；注释 "keyboard events keep the early return" 把这个死分支固化了）。建议仅对鼠标事件应用左键守卫，键盘 Enter 直接放行。

  function handleInternalLink(event: ReactMouseEvent<Element> | ReactKeyboardEvent<Element>, path: string, afterNavigate?: () => void): void {
-   if (event.defaultPrevented || (event as ReactMouseEvent<Element>).button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
+   if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
+   // button 仅存在于鼠标事件（左键守卫）；键盘 Enter 无 button，直接放行。
+   if (event.type === 'click' || event.type === 'mousedown' || event.type === 'mouseup') {
+     if ((event as ReactMouseEvent<Element>).button !== 0) return;
+   }
    event.preventDefault();
+   afterNavigate?.();
+   navigate(path);


─── apps/web/src/platform/PlatformShell.tsx:218-218 ───
[maintainability · medium] career 系列导航标签硬编码中文字符串（'求职档案'/'找岗'/'持续找岗'），绕过了同表中所有其它条目统一使用的 `labels.*` /
`t()` i18n 通道——多语言（Locale/formatMessage）环境下这三个标签将无法翻译。建议与 labels 一并接入本地化。

-     { key: 'career', href: '/platform/career', label: '求职档案', icon: 'career', iconNode: <ReactOnlyNavIcon paths={['M4 7.5H20', 'M6 4.5H18V20H6Z', 'M9 12H15', 'M9 15.5H13']} />, match: (p: string) => p === '/platform/career' },
+     { key: 'career', href: '/platform/career', label: labels.careerProfile, icon: 'career', iconNode: <ReactOnlyNavIcon paths={['M4 7.5H20', 'M6 4.5H18V20H6Z', 'M9 12H15', 'M9 15.5H13']} />, match: (p: string) => p === '/platform/career' },


─── apps/web/src/platform/GlobalCommandPalette.tsx,apps/web/src/platform/InvitationInbox.tsx,apps/web/src/platform/PlatformShell.tsx,apps/web/src/platform/platform-shell.td.css,apps/web/src/platform/platform-u.css,apps/web/src/platform/retrieval-settings-panel.tsx,apps/web/src/platform/session-activity.ts:0-0 ───
[maintainability · low] 未使用的导入：全文件没有任何 `React.` 命名空间引用（类型均通过具名 import 获取，JSX 走 automatic
runtime，此前该文件也无此导入），这行 `import * as React` 是死代码，建议删除。



─── apps/web/src/platform/platform-shell.td.css:1568-1572 ───
[bug · low] z-index 层级冲突：移动端侧栏 overlay 的 `z-index: 1001` 高于全局命令面板（`.wk-cmdk-5`，z-index
1000）。窄视口下侧栏展开时点击 logo_row 的搜索按钮打开 palette，面板会被固定定位的侧栏（左侧 260px 全高）遮住一部分。建议把 overlay 降到 palette 之下（如
999），aside 内部的 `.user-dropdown`（z-index 1000）在 aside 自身层叠上下文内，不受影响。

  @media (max-width: 640px) {
    .aside_box--mobile-overlay {
      position: fixed;
      inset: 0 auto 0 0;
-     z-index: 1001;
+     z-index: 999;


─── apps/web/src/analytics/AnalyticsPage.tsx:68-68 ───
[documentation · low] 注释与实现脱节：文件头（第 6 行 "Tailwind class constants"）与本行上方的 "Tailwind v4 utility
recipes shared across the page's cards and controls" 仍声称这些常量是 Tailwind v4 utility 配方，但本次迁移后它们已是指向
analytics-u.css 的语义化类名常量。过时注释会误导后续维护者按 Tailwind 语法去改这些字符串（例如往里加 `flex gap-2` 之类的 utility
不会再生效）。建议同步更新这两处注释为语义类名 + analytics-u.css 的说明。



─── apps/web/src/analytics/analytics-u.css:294-300 ───
[maintainability · low] 无语义编号类名：`.wk-anl-1`（JSX 中用于日期范围
label）、`.wk-anl-2`（表头行）、`.wk-anl-3`（表头加粗）是纯数字命名，与本文件其余描述性命名（wk-anl-an-usage-note 等）及
AnalyticsPage.tsx 中内联使用（className="wk-anl-1"）的约定不一致，无法从名字推断用途，也无法按域 grep 定位。建议改为描述性名称并同步 JSX，如
wk-anl-an-range-label / wk-anl-an-usage-head-row / wk-anl-an-th。



─── apps/web/src/data-sources/data-sources-u.css:361-366 ───
[bug · low] `.wk-ds-42`（新建数据源按钮）基础态为 `border-style: solid` 后被同规则的 `border-style: dashed` 覆盖（等效原
`border border-dashed`），但 `:hover` 里又写了 `border-style: solid`，导致悬停时虚线边框变为实线。原 utilities 为
`hover:border-primary`（仅改色，hover 仍保持 dashed），与文件头注「值 = utilities 编码的生效值」及
ui.tsx「视觉零变化」目标不符，属于平移引入的视觉回归。建议删除 hover 中的 `border-style: solid`（.wk-ds-2:hover / .wk-ds-78:hover
中的 `border-style: solid` 因基础态本就是 solid 而冗余，可一并清理）。

  .wk-ds-42:hover {
-   border-style: solid;
    border-color: #2e6de6;
    background-color: color-mix(in srgb, #2e6de6 5%, transparent);
    color: #2e6de6;
  }


─── apps/web/src/experts/ExpertsPage.tsx:46-47 ───
[maintainability · medium] 无生效对象的跨域样式导入：`views-chat-u.css`（约 2545
行）中不含任何本页使用的类——`wk-exp-*`/`wk-exp-xp-*` 全部由 experts-u.css 承接；`wk-page` 全仓库无基类样式（仅 platform-u.css 中
`.plat-shell__outlet .wk-page` 等壳层作用域覆盖，由 PlatformShell 侧加载）；`wk-experts-persona` 全仓库无 CSS 定义（仅作 DOM
钩子）；`renderChatMarkdown`（packages/views/src/chat/markdown.ts）输出不携带任何 class。该导入只会把 chat 域整包样式打进
experts 路由 chunk（包体膨胀），并建立无依据的跨域耦合——chat 侧后续重构该文件时 experts
页会被不可预期地波及。docs/plans/issue-140/ocr/ocr-round-1.md:394-402 的评审结论与此一致并建议删除。

- import '../chat/views-chat-u.css';
  import './experts-u.css';


─── apps/web/src/experts/experts-u.css:66-70 ───
[maintainability · medium] 语义令牌被固化为字面值，且同文件内两种写法混用：本域卡片/页签/输入框/按钮统一写死 `#ffffff`、`#07c05f`、`rgba(7,
192, 95, 0.08)`（对应 Tailwind 的 bg-surface / border-accent / text-accent / bg-accent-wash），而同文件面板类
`.wk-exp-8/.wk-exp-22/.wk-exp-26/.wk-exp-36` 却保留 `var(--wk-bg,#fff)`——且 `--wk-bg` 全仓库并无定义，恒走
fallback。经查 styles.css @theme 中这三个令牌当前确为静态字面值且无暗色/运行时覆盖，故本改动暂无视觉回归；但兄弟域迁移惯例是保留 var() 引用（如
documents-u.css 用 `var(--wk-surface,#fff)`、administration-u.css 用
`var(--color-accent,#07c05f)`），一旦令牌体系接入运行时主题，本域十余处卡片/页签/按钮将无法跟随，且文件内部两类写法会先行分裂。建议统一改用
`var(--color-surface, #ffffff)`、`var(--color-accent, #07c05f)`、`var(--color-accent-wash, rgba(7,
192, 95, 0.08))` 形式。

  .wk-exp-xp-tab[aria-selected="true"] {
-   border-color: #07c05f;
-   background-color: rgba(7, 192, 95, 0.08);
-   color: #07c05f;
+   border-color: var(--color-accent, #07c05f);
+   background-color: var(--color-accent-wash, rgba(7, 192, 95, 0.08));
+   color: var(--color-accent, #07c05f);
  }


─── apps/web/src/experts/experts-u.css:420-424 ───
[maintainability · low] 格式噪声（不影功能）：1) `.wk-exp-9/.wk-exp-22/.wk-exp-34` 中 `border-bottom-width:
1px;border-color: rgba(127,127,127,0.2);` 两条声明挤写同一行，与全文件一行一声明风格不一致；2) hover/selected 规则（如
`.wk-exp-xp-tab:hover`、`.wk-exp-48`）重复声明基类已有的 `border-style: solid`，属冗余；3) 文件头注释「导入顺序：须在本域 .td.css
之前」引用的 experts.td.css 目前并不存在（experts 域仅有 experts-u.css），建议注明为前向约束或移除，避免误导。

    border-bottom-style: solid;
-   border-bottom-width: 1px;border-color: rgba(127,127,127,0.2);
+   border-bottom-width: 1px;
+   border-color: rgba(127,127,127,0.2);
    padding-inline: 24px;
    padding-block: 20px;
  }


─── apps/web/src/integrations/ApiPlaygroundDrawer.tsx:342-342 ───
[bug · high] 移除 @weknora/ui Input/Textarea 后，原生 input/textarea 仅剩 wk-apd-1（box-sizing）和内联
fieldStyle（display/margin/width），无任何边框、内边距、字体规则：全仓搜索
.wk-api-playground-agent/.wk-api-playground-external-user/.wk-api-playground-query 仅在
integrations.td.css 命中一条 box-sizing 兜底。原生 input 不继承 body 字体（UA 默认 13.33px Arial），且 styles.css 已移除
Tailwind utilities 引擎，无从生成外观。注释引用的 ModelSettingsPanel Ollama combobox 先例在 settings.td.css:5077 有
.wk-ollama-combobox-wrap input 的完整 scoped 外观规则，本处没有对应规则，三个输入框将以无边框、无内距、系统字体的形态渲染，视觉回归明显。建议在
integrations.td.css 为这三个类补齐输入框外观。

-               <input className="wk-api-playground-external-user wk-apd-1" type="text" value={form.externalUserId} disabled={mode === 'tenant'} placeholder={t('integrations.api.playgroundExternalUserPlaceholder')} onChange={(event) => setForm((prev) => ({ ...prev, externalUserId: event.target.value }))} style={fieldStyle} />
+ /* integrations.td.css 内补齐（对齐 Ollama combobox 先例） */
+ .wk-api-playground-overlay .wk-api-playground-agent,
+ .wk-api-playground-overlay .wk-api-playground-external-user,
+ .wk-api-playground-overlay .wk-api-playground-query {
+   box-sizing: border-box;
+   border: 1px solid var(--color-line-input, #dcdcdc);
+   border-radius: var(--radius-control, 6px);
+   padding: 0.45rem 0.6rem;
+   font: inherit;
+   color: var(--color-ink, #172033);
+ }


─── apps/web/src/integrations/EmbedPreviewModal.tsx:93-93 ───
[bug · high] 预览 iframe 除条件类 wk-epm-15（visibility:hidden）外没有任何类名，而 apps/web 全部样式表中不存在任何 iframe
选择器规则（已全局搜索确认），wk-epm-13 只提供容器定位（position/inset/grid）。styles.css 已移除 Tailwind utilities
引擎；integrations.td.css 回填的 .wk-embed-preview-screen 系列是组件已不再挂载的旧类名，且其中也不含 iframe 尺寸规则。结果 iframe
将以浏览器默认 300×150 + 默认边框渲染，无法铺满预览设备屏幕区（min-height 480px），与注释所述 "Former .wk-embed-preview-screen +
iframe rules" 的 Vue 基线行为不符。建议为实际容器补 iframe 尺寸规则。

-             {layoutReady ? <iframe title={label} src={src} onLoad={() => setReady(true)} className={ready ? '' : 'wk-epm-15'} allow="clipboard-write" /> : null}
+ /* integrations-u.css */
+ .wk-epm-13 iframe { width: 100%; height: 100%; border: 0; }


─── apps/web/src/integrations/EmbedPreviewModal.tsx:92-92 ───
[maintainability · low] '正在加载预览…' 与第 70 行的 fallback '预览' 均为硬编码中文，而同组件 closeLabel()/label 链路其余文案均经
integrationsT 按本地化输出。packages/views/src/integrations/embedWizardMessages.ts 已定义
embedPublish.previewLoading（五种语言，中文值恰为 "正在加载预览…"）与 embedPublish.preview 键，views
端预览面板（page.tsx:669）正是消费这些键，可直接复用，避免 locale 切换时该弹窗中英混排。

-             {!ready ? <span className="wk-muted wk-epm-14">正在加载预览…</span> : null}
+ {!ready ? <span className="wk-muted wk-epm-14">{integrationsT(integrationsLocale(locale ?? window.localStorage.getItem('locale')), 'embedPublish.previewLoading')}</span> : null}


─── apps/web/src/integrations/IntegrationsPage.tsx:3-4 ───
[maintainability · low] 本地 IntegrationsPage 没有任何运行时引用：IntegrationsRoutePage 渲染的是
@weknora/views/integrations/page 的同名组件（import { IntegrationsPage } from
'@weknora/views/integrations/page'），全仓对 './IntegrationsPage' 的导入为零；唯一消费方 route.test.ts:75 是
readFileSync 源码文本的静态 marker 断言（其断言列表已同步 tdesign 标记，测试可通过）。对不被渲染的组件做 @weknora/ui→tdesign
迁移，实质是在维护一个测试 fixture。建议在文件头明确注明其 "仅作 route.test.ts 源码契约、非运行时页面"
的地位，或直接删除组件并同步移除对应测试断言，避免后续维护者误以为这是线上集成页而重复维护两套实现。



─── apps/web/src/integrations/integrations-u.css:100-107 ───
[maintainability · medium] 按钮双源定义：组件同时挂 wk-button/wk-button--text 与
wk-apd-8/wk-apd-13，integrations.td.css 的 .wk-api-playground-overlay .wk-button 族（活规则）与本处 wk-apd-8/13
对 padding/border/background/color 同属性各写一遍，靠 u.css 的 !important + 文件加载顺序 +
选择器特异性三层裁决，且方向不一致——.wk-muted 的色值最终听 td.css（特异性 0,2,0 > 0,1,0），按钮却又靠 u.css !important 压回 td.css。另外
wk-apd-13 内 border-style: solid 重复声明两次（普通 + !important）。建议收敛为单一来源：组件去掉 wk-apd-8/13 完全走 td.css 的
.wk-button 族（它已带 var() token），或反向删除 td.css 按钮规则。



─── apps/web/src/integrations/integrations-u.css:154-159 ───
[maintainability · medium] 原 utilities（bg-surface/text-ink/text-muted-strong/line-control/hover-wash
等）对应的 token 已在 styles.css :root
定义（--color-surface/--color-ink/--color-muted-strong/--color-line-control/--color-hover-wash），本文件却几乎全
部硬编码字面量（#ffffff、rgba(0,0,0,.9)、rgba(0,0,0,.6)、#cbd5e1、#f2f5fa、#2e6de6、#e7e7e7…），而 td.css 的同义规则用的是
var(--color-*, fallback)——同域两套口径。硬编码侧在主题定制/换肤时不可覆盖。另注意 wk-apd-2 注释声称值为 rgba(0,0,0,.6)，但抽屉内实际被 td.css
的 .wk-api-playground-overlay .wk-muted（var(--color-muted,#66758b)，特异性更高）覆盖，注释与实际生效值不符。建议统一改用
var(--color-*, 字面量) 形式。

  .wk-epm-2 {
    box-sizing: border-box;
    height: 100%;
    width: min(720px,100vw);
    overflow: auto;
-   background-color: #ffffff;
+   background-color: var(--color-surface, #ffffff);


─── apps/web/src/integrations/integrations.td.css:19-21 ───
[maintainability · medium] 本节大部分选择器与组件实际类名脱节：ApiPlaygroundDrawer 已改用 wk-apd-*
语义类（wk-apd-3/4/5/6/9/11/14~17），因此
.wk-api-playground-aside/.wk-api-playground-body/.wk-api-playground-section(>h4)/.wk-api-playground-
step-label 及 .wk-api-playground-status[data-status=…] 共 10 行规则无任何元素挂载，是死样式（全仓搜索确认这些类名仅在本文件出现）。其中
.wk-api-playground-error 还与组件实际类名
wk-api-playground-field-error（ApiPlaygroundDrawer.tsx:339）拼写不一致——后者无任何样式规则。后续维护者若在本文件调整状态色或 section
布局将静默不生效。另注：仍活着的 .wk-api-playground-pre 定义在本文件，但 ApiPlaygroundDrawer 自身只 import
integrations-u.css，其可用性完全依赖 IntegrationsRoutePage 的导入，存在跨文件隐式耦合。建议删除死规则、修正或移除 error 拼写，保留
.wk-api-playground-pre 与 .wk-button 活规则。



─── apps/web/src/integrations/integrations.td.css:86-89 ───
[maintainability · medium] 本节约 100 行 .wk-embed-preview-* 规则全部为死样式：apps/web 的 EmbedPreviewModal 已改用
wk-epm-*，packages/views 的 deploy-step 预览面板也已改用 wk-vi-*（EMBED_PREVIEW_FRAME_CLASS =
'wk-vi-embed-preview-frame-class'，page.tsx:824），全仓对这些旧类名的引用只剩 "Former …" 注释。本节唯一活规则是
.wk-integration-drawer-close（EmbedPreviewModal 与 views page.tsx 均仍挂载该类）。此外回填内容并未包含原
.wk-embed-preview-screen 下的 iframe 尺寸规则，导致预览 iframe 无样式来源（见 EmbedPreviewModal 评论）。建议删除本节死规则、保留
.wk-integration-drawer-close，并将 iframe 尺寸规则补到实际类名（.wk-epm-13 iframe / wk-vi 对应容器）。



─── apps/web/src/experts/ExpertsPage.tsx:50-50 ───
[documentation · low] 注释与实现脱节：本变更将全部 XP_* 常量值从 Tailwind 工具类（如 'box-border h-full overflow-y-auto
...'）替换为 experts-u.css 中的语义类名，但上方注释仍写 "Tailwind v4 utility recipes shared across the page
(AnalyticsPage constants)"（文件头第 3-4 行 "Tailwind class constants" 同样过时）。后续维护者按注释指引会误以为应在这里追加 Tailwind
工具类，与新的纯语义类方案冲突。建议同步更新注释，说明这些常量对应 experts-u.css 中的 .wk-exp-xp-* 规则。

+ /* Semantic class-name constants shared across the page; each maps to a
+  * rule in ./experts-u.css (wk-exp-xp-*). */
  const XP_PAGE = 'wk-page wk-exp-xp-page';


─── apps/web/src/integrations/views-integrations-u.css:2323-2323 ───
[bug · medium] 开关选中态永不生效：本条（layer 外，!important）与 `.wk-vi-42` 的 `background-color:#cbd5e1
!important`（@layer utilities 内，!important）竞争同一属性。按 CSS Cascade 5，同为 !important 时 layered 恒胜
unlayered（与 normal 声明方向相反），因此灰色轨道永远压过选中色——渠道启停开关选中后轨道不变色（原 Tailwind 中 peer-checked:bg-primary! 与
bg-[#cbd5e1]! 同在 utilities 层靠特异性取胜，平移拆层后断裂）。修复：将本条移入 @layer utilities（置于 .wk-vi-42 之后，特异性更高可胜出），或去掉
.wk-vi-42 中 background-color 的 !important。

+ /* 移入 @layer utilities 内、.wk-vi-42 规则之后（并删除层外本条）：
  .wk-vi-41:checked + .wk-switch-knob--vi { background-color: var(--color-primary, #2e6de6) !important; }
+ 或：.wk-vi-42 中 background-color 去掉 !important，让层外选中态规则按 normal 声明（unlayered 恒胜 layered）生效 */


─── apps/web/src/integrations/views-integrations-u.css:902-904 ───
[bug · medium] calc(100%+4px) 为非法值：calc 中 + 两侧必须有空白，否则整条声明被浏览器丢弃。`.wk-vi-33`（智能体筛选下拉 listbox）为
absolute 定位，left:0 生效但 top 回退 auto/静态位置，下拉框将不再相对触发按钮下移 4px，可能与触发按钮本身重叠。该写法沿袭自原 Tailwind 任意值
top-[calc(100%+4px)]（原本就非法），平移时应顺带修正；同族错误还出现在 documents-u.css L348/L859。

  .wk-vi-33 {
    position: absolute;
-   top: calc(100%+4px);
+   top: calc(100% + 4px);


─── apps/web/src/integrations/views-integrations-u.css:39-44 ───
[bug · low] color: color:inherit; 属性名重复，属非法声明，整条被解析器丢弃（本文件共 3 处：L43/L64/L148 对应
channel-card-clickable/-static/-title-add）。当前元素为 article/span 自然继承影响有限，但应修正为 color: inherit。另
`.wk-vi-channel-badge-static-class` 中 rgba(0, 0, 0, 0.6);; 存在双分号笔误，一并清理。

  .wk-vi-channel-card-clickable-class {
    width: 100%;
    cursor: pointer;
    background-color: #ffffff;
-   color: color:inherit;
+   color: inherit;
  }


─── apps/web/src/integrations/views-integrations-u.css:1530-1537 ───
[style · low] 同一规则内 font-size 声明了两次：先 14px 后 inherit，后者在相同特异性下胜出，显式字号实际是死声明（.wk-vi-160 的
font-size:13px/line-height:18px、.wk-vi-164 的 font-size:14px 同病）。而同类场景 .wk-vi-137/.wk-vi-142 用
font-size: 12px !important 保值，本文件平移口径不一致，易让后续维护者误以为 14px 生效。建议删除死声明（若以 inherit 为准），或补 !important 对齐
.wk-vi-137 口径（若以 14px 为准）。

    padding-inline: 15px;
-   font-size: 14px;
+   font-size: 14px !important; /* 对齐 .wk-vi-137 口径；若 inherit 才是意图则删除本行 */
    color: rgba(0,0,0,0.9);
    font-family: inherit;
-   font-size: inherit;
    line-height: inherit;
    font-weight: inherit;
  }


─── apps/web/src/integrations/views-integrations-u.css:2341-2345 ───
[maintainability · low] 手工段整体置于 @layer utilities 之外，与文件头声明的契约（这些规则原为 Tailwind utilities/layered，对
unlayered 页面 CSS 恒输，经 layer 归位逐字保留）相反：`.wk-vi-chip--active/--idle` 的 base `.wk-vi-chip-base`
在层内而状态类在层外，unlayered 状态类会反向压过页内 unlayered 规则（.wk-vi-seg 系列、.wk-vi-accent-primary
同）。目前未发现具体冲突受害者，但层叠关系已被反转，属埋雷。建议将无伪类结构限制的规则一并移入 @layer utilities（置于对应 base 之后），层外只保留必须用兄弟选择器复刻 peer
语义的开关 knob 两条。

- .wk-vi-chip--active {
-   border-color: #2e6de6;
-   background-color: #eff4ff;
-   color: #1849a9;
- }
+ /* 移入 @layer utilities 尾部（.wk-vi-chip-base 之后）：
+ .wk-vi-chip--active { border-color: #2e6de6; background-color: #eff4ff; color: #1849a9; }
+ .wk-vi-chip--idle { border-color: #dce3ed; background-color: #ffffff; color: #506078; } */


─── packages/views/src/integrations/page.tsx:1768-1769 ───
[maintainability · low] {false && ...} 三段（cli/chrome/claw
各一处）为条件恒假、永不渲染的死分支，且本次平移仍在其中持续替换类名与图标（⧉→CopyIcon），后续每次样式调整都要陪着改，徒增维护成本并误导读者以为该路径可达。建议整体删除这三段；若确有保留意图
（如待启用的内容占位），请加注释说明。

-     {false && tab === 'cli' ? <section className="wk-vi-121">
-       <h4 className="wk-vi-122">{t('integrations.cli.commandsTitle')}</h4>
+ // 删除全部 {false && tab === 'cli'/'chrome'/'claw' ? ... : null} 死分支；
+ // 若需保留占位请显式注释：/* 保留：待启用，勿改 */


─── packages/views/src/integrations/page.tsx:1917-1917 ───
[style · low] 图标尺寸口径不一：注入尺寸 15px 与回退 SVG 的 14px 以及上一行注释 "1em = 14px" 三者矛盾——apps/web 注入渲染器时图标为
15px，views 包直渲染/测试回退时为 14px，两环境尺寸漂移。另请求示例复制按钮由原 CopyIcon(16px) 改为内联 file-copy
14px，与其余代码工具栏复制按钮（CopyIcon 16px）不一致。建议统一为一个尺寸（并同步修正注释），可提取共享常量。

-   return <SpriteIcon name="jump" size="15px" fallback={<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" aria-hidden="true"><path d="M9 4L4 4L4 20L20 20L20 15" /><path d="M19.25 4.75L12 12M14 4H20L20 10" /></svg>} />;
+   return <SpriteIcon name="jump" size="14px" fallback={<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" aria-hidden="true"><path d="M9 4L4 4L4 20L20 20L20 15" /><path d="M19.25 4.75L12 12M14 4H20L20 10" /></svg>} />;


─── packages/views/src/integrations/page.tsx:1743-1743 ───
[style · low] 透明底改写口径漂移：原 rgba(7,192,95,0.1)/rgba(232,93,42,0.12) 是随底色变化的透明叠加，改为 color-mix(in srgb,
... 10%, #fff) 后固定向纯白混合；该 span 位于 hero 区域，而 hero 自身经 style
注入了自定义渐变底色（heroBackground），两种算法在其上的观感不一致（`.wk-vi-133` 的 claw 徽标同族改写）。若非对齐某个明确的 Vue 端基准，建议保留 rgba
表达，避免在自定义底色上出现可感知色差。

-           <span className="wk-vi-105" style={{ background: isClaw ? 'color-mix(in srgb, #e85d2a 12%, #fff)' : 'color-mix(in srgb, #07c05f 10%, #fff)', color: brandDark }}>{tab === 'cli' ? <LandingIcon name="code" size={14} /> : tab === 'chrome' ? <LandingIcon name="extension" size={18} /> : <span aria-hidden="true" className="wk-vi-106">🦞</span>}</span>
+           <span className="wk-vi-105" style={{ background: isClaw ? 'rgba(232,93,42,0.12)' : 'rgba(7,192,95,0.1)', color: brandDark }}>...


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:27-29 ───
[maintainability · low] 死导入：isSharedKbEditable 在本文件内零调用（全文件仅此 import 一处出现），属 unused import，触发 lint
告警并误导维护者。建议删除。

    isKnowledgeBaseInitialized,
-   isSharedKbEditable,
    groupKnowledgeBaseSections,


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:822-823 ───
[bug · medium] 死状态 + 冗余网络请求：editorActivity/editorActivityLoading 两个 state 只写不读（JSX
无任何消费），loadEditorActivity 的请求结果全部丢弃；且导航 onClick 中 `if (section === 'activity' && editingId) void
loadEditorActivity(editingId)` 会在每次切到活动记录页签时发起一次多余的 settings.activity 请求——KnowledgeBaseActivityPanel
组件自身会独立加载数据。建议整组删除这两个 state、loadEditorActivity 函数及 onClick 中的调用分支。

-   const [editorActivity, setEditorActivity] = useState<Array<{ id: number; action: string; outcome: string; created_at: string }>>([]);
-   const [editorActivityLoading, setEditorActivityLoading] = useState(false);
+ // 删除这两个 state，以及 loadEditorActivity 函数与导航 onClick 中的
+ // `if (section === 'activity' && editingId) void loadEditorActivity(editingId)` 分支
+ //（活动面板 KnowledgeBaseActivityPanel 自行加载数据）。


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:54-55 ───
[style · low] 同一模块存在两条 'tdesign-react' import（第 36-49 行主导入与这里），违背单一 import 源的团队约定，也增加合并冲突面。建议将
TCheckbox/TInput/TSelect/TTextarea 并入主 import。

- // S6 抽屉收编：kb 编辑器深设置留守段离开 packages/ui 旧栈（T15 硬前置），换 tdesign。
- import { Checkbox as TCheckbox, Input as TInput, Select as TSelect, Textarea as TTextarea } from 'tdesign-react';
+ // 并入第 36 行的主 import：
+ // import { Button, Checkbox, Checkbox as TCheckbox, Dialog, Input, Input as TInput,
+ //   Loading, Popup, Radio, RadioGroup, Select as TSelect, Skeleton, Textarea,
+ //   Textarea as TTextarea, Tooltip, MessagePlugin } from 'tdesign-react';


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:848-848 ───
[style · low] 以 `&&` 短路作为表达式语句（side-effectful short-circuit）调用 setViewer，可读性差且违背团队代码风格（等价于裸 if
但语义模糊）。建议改为显式 if 语句。

-     scopeController.current().scope.userId && setViewer((current) => ({ ...current, userId: scopeController.current().scope.userId ?? '' }));
+     const scopedUserId = scopeController.current().scope.userId;
+     if (scopedUserId) setViewer((current) => ({ ...current, userId: scopedUserId }));


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:1616-1616 ───
[maintainability · low] 可达性债务：删除确认的「确认」与「取消」均为 span+onClick，无 role/tabIndex/键盘事件，键盘用户无法触达删除确认（Focus
顺序中不可聚焦）。虽是 Vue 1:1 复刻，建议至少改为 <button type="button"> 或补齐
role="button"/tabIndex={0}/onKeyDown(Enter|Space)。

-             <span className="circle-btn-txt confirm" onClick={() => { void confirmDelete(); }}>{t('knowledgeList.delete.confirmButton')}</span>
+             <button type="button" className="circle-btn-txt confirm" onClick={() => { void confirmDelete(); }}>{t('knowledgeList.delete.confirmButton')}</button>


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:1869-1869 ───
[maintainability · low] 业务硬编码 URL：KG 指南 fallback 直写字符串字面量（github.com），且本文件还有多处
'/platform/settings?section=models&subsection=chat' 等路由字面量散落在 JSX 中，违背「禁止业务相关硬编码
URL」规则。建议集中为路由/指南链接常量（或配置模块），便于统一调整与审计。

-                     <GraphSettings graphExtract={editorConfig.nodeExtractConfig as GraphExtractConfig} modelId={summaryModelId} client={client} canRunGraphExtract={viewer.isAdmin} onChange={(value) => setEditorConfig((current) => ({ ...current, nodeExtractConfig: value }))} onOpenGraphGuide={() => { window.open(((import.meta as { env?: Record<string, string | undefined> }).env?.VITE_KG_GUIDE_URL) || 'https://github.com/Tencent/WeKnora/blob/main/docs/KnowledgeGraph.md', '_blank', 'noopener'); }} />
+ // 于文件顶部集中：const KG_GUIDE_FALLBACK_URL = 'https://github.com/Tencent/WeKnora/blob/main/docs/KnowledgeGraph.md';
+ // 使用处：window.open(((import.meta as { env?: Record<string, string | undefined> }).env?.VITE_KG_GUIDE_URL) || KG_GUIDE_FALLBACK_URL, '_blank', 'noopener');


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:585-586 ───
[style · low] 嵌套三元：卡片头部的 `isSpaceCard ? … : isSharedCard ? … : …` 三层分支（另见 basic 段 wiki granularity
hint 的三层三元 `=== 'focused' ? … : … === 'exhaustive' ? … : …`）违背团队「禁止嵌套三元表达式」规则，可读性差。前两个分支的 JSX
完全相同（info-circle 详情触发），建议抽 `const detailTrigger = (<Tooltip …>…) 或渲染函数消除嵌套。

-         ) : isSharedCard ? (
-           /* Vue :311-316：「全部」视图共享卡片的详情触发替代三点菜单。 */
+ {/* isSpaceCard 与 isSharedCard 分支 JSX 完全一致，可合并为： */}
+ {isSharedCard || isSpaceCard ? detailTrigger : moreMenu}
+ {/* detailTrigger 提为局部变量/渲染函数 */}


─── apps/web/src/knowledge-bases/SharedKnowledgeBaseDrawer.tsx:54-58 ───
[maintainability · low] 死导出：permissionTone 兼容包装在全库无任何调用方（本文件内部也已改用 permissionTheme，测试亦未引用）。既然 t-tag
迁移已完成，建议删除该兼容层，缩小维护面。



─── apps/web/src/knowledge-bases/SharedKnowledgeBaseDrawer.tsx:60-61 ───
[maintainability · low] 重复实现：OrgGreenIcon（organization-green.svg 内联副本，SVG path 完全相同）已在
KnowledgeBasesPage.tsx 定义一份（无参版），加上既有的 agents/AgentsPage.tsx 共三处并存。同一图形后续主题/尺寸调整极易漂移，建议抽到共享模块（如
kb-list-icons.tsx 或 shared/）三处共用。



─── apps/web/src/knowledge-bases/SharedKnowledgeBaseDrawer.tsx:102-103 ───
[bug · medium] 自绘 overlay 丢失对话框键盘语义：原 @weknora/ui Sheet 自带 Esc 关闭与焦点圈闭/回归，重写后仅剩遮罩点击关闭，键盘用户无法用 Esc
关闭抽屉（Vue 版可通过 overlay focus/Tab 逃逸场景关闭）。建议添加 Escape 监听；如需对齐 Sheet 体验，可再补初始 focus
与关闭后焦点回归。注意：组件条件挂载（!visible return null）也使 kb-list.td.css 中的 *-enter/*-leave 过渡类永远无 DOM
可挂载（见该文件评论），动画实际不生效。

-     <div className="shared-detail-drawer-overlay" onClick={(event) => { if (event.target === event.currentTarget) onClose(); }}>
-       <div className="shared-detail-drawer" role="dialog" aria-label={t('knowledgeList.detail.title')}>
+ // 组件内补（需 import { useEffect } from 'react'）：
+ useEffect(() => {
+   if (!visible) return;
+   const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
+   window.addEventListener('keydown', onKey);
+   return () => window.removeEventListener('keydown', onKey);
+ }, [visible, onClose]);


─── apps/web/src/knowledge-bases/kb-list.td.css:1818-1821 ───
[maintainability · low] 死 CSS：SharedKnowledgeBaseDrawer 已改为条件挂载（关闭即整体卸载，直接 return null），DOM 中从不出现
*-enter / *-enter-from / *-leave-to 类，这组 Vue Transition 平移规则（1818-1840
行）永远无匹配元素，进出场动画实际不生效。建议要么删除这组规则，要么在抽屉实现中真正补上过渡类/状态机（与 Esc 关闭评论一并处理）。



─── apps/web/src/knowledge-bases/kb-list.td.css:1840-1841 ───
[maintainability · low] 死 CSS：抽样确认 .kb-list-main-loading（612
行）、.kb-tabs/.tab-item、.shared-badge、.personal-source、.shared-by-me-badge、.nav-badge、.create-kb-dialo
g（1841-1853 行）、.kb-multimodal-settings 整段等在当前 JSX 中均无生产者（注释自述「保留以备兼容」）。2800+
行文件中这些不可达规则持续抬升维护成本，建议趁本次迁移收口删除，或以 TODO 注明具体删除批次（如 knowledge-settings 批次）。另外 6. 节
`.kb-section-header` 连续两个规则块（grid-column/pointer-events 与 sticky 定位）可合并。



─── apps/web/src/integrations/views-integrations-u.css:2286-2289 ───
[bug · medium] 分段单选（主体模式 tenant/direct/signed）边框迁移失真且存在层叠冲突：
1) 原 idle 态为字面量 border-[#dcdcdc]，现 .wk-vi-166 改为 #e7e7e7（无注释说明的口径变更），而 .wk-vi-167 又单独给末项右缘保留
#dcdcdc——idle 态下末项三边 #e7e7e7、右缘 #dcdcdc，同组颜色不一致。
2) 更实质的缺陷：page.tsx 中 `(index === 2 ? ' wk-vi-167' : '')` 无条件挂在末项上，而 .wk-vi-167 位于 .wk-vi-165（选中态
border-color:#07c05f，非 important）之后、同层同特异性，源顺序恒胜——选中"签名令牌"（末项）时按钮右侧边框被压成灰色
#dcdcdc，绿底按钮出现灰右边线。原代码（last:rounded-r-[3px]）并无任何 border-right 规则，.wk-vi-167 属无出处的新增。
建议：删除 .wk-vi-167 及 page.tsx 中对应的 `+ (index === 2 ? ' wk-vi-167' : '')`，并将 .wk-vi-166 的 border-color
恢复为原字面量 #dcdcdc；若确需末项右缘特殊化，应写成 `.wk-vi-166.wk-vi-167`（或限定 :not 选中态）避免压过选中色。

- .wk-vi-167 {
-   border-right-style: solid;
-   border-right-color: #dcdcdc;
+ /* 删除 .wk-vi-167；.wk-vi-166 恢复原字面量 */
+ .wk-vi-166 {
+   border-style: solid;
+   border-color: #dcdcdc;
+   background-color: #ffffff;
+   color: rgba(0,0,0,0.9);
  }


─── apps/web/src/integrations/views-integrations-u.css:170-182 ───
[bug · medium] 同一原始 token 在本文件内被映射为两个互相矛盾的值：本规则把
`border-line`→#e7e7e7、`text-muted-strong`→rgba(0,0,0,.6)，而文件尾 `.wk-vi-chip--idle` 把同一对 utility 映射为
#dce3ed/#506078。apps/web styles.css:376-455（原 packages/ui theme.css @theme 块平移的"事实源"）明确
--color-line:#dce3ed、--color-muted-strong:#506078、--color-line-neutral:#e7e7e7——即迁移前 Tailwind
工具类的真实渲染色。两组至少有一组与替换前的渲染不符：本组（int-tab、.wk-vi-11/.wk-vi-18/.wk-vi-19、.wk-vi-embed-preview-frame-class
、im/embed step chrome 等同注释处）会让集成页 tab、加载状态文本、预览框边框整体变色，并与相邻的连接模式 chips（#dce3ed/#506078）观感割裂。同一 token
只允许一个口径；若确要统一改用 TDesign var(--td-component-stroke)/--td-text-color-secondary，则 chips 一侧需同步，且建议直接引用
var(--color-*) 以避免再次双值。

  .wk-vi-int-tab-class {
    cursor: pointer;
    border-radius: 999px;
    border-style: solid;
    border-width: 1px;
-   /* 旧栈 line/line-soft 硬编码→Vue var(--td-component-stroke)=#e7e7e7 */
-   border-color: #e7e7e7;
+   border-color: #dce3ed;   /* = --color-line，与 .wk-vi-chip--idle 统一 */
    background-color: #ffffff;
    padding-inline: .75rem;
    padding-block: .5rem;
-   /* 旧栈 muted/muted-strong 硬编码→Vue var(--td-text-color-secondary)=rgba(0,0,0,.6) */
-   color: rgba(0, 0, 0, 0.6);
+   color: #506078;          /* = --color-muted-strong */
  }


─── apps/web/src/integrations/views-integrations-u.css:220-225 ───
[maintainability · medium] 本规则的 `color: #9fb4d8 !important` 与 `.wk-vi-48`/`.wk-vi-170` 的 `color: …
!important` 在全部 4 处在用按钮（page.tsx L1124 回调地址复制、L1392/1393 部署代码预览/复制、L1408 服务端示例复制）上恒同时挂载：两者同层（@layer
utilities）、同特异性 (0,1,0)、同为 important，胜负仅由文件内规则顺序决定——.wk-vi-48 在文件后部恒胜，#9fb4d8 在所有元素上永不生效，成为死声明。原
Tailwind 产物中 `text-muted-strong!` 与任意值 `text-[#9fb4d8]!` 的先后由 Tailwind 生成序决定（v4
任意值通常排在主题值之后，即原为浅蓝生效），平移后获胜者可能翻转、按钮文字色发生未声明变更，也违背文件头"!important 例外照旧胜出"的迁移契约。两处类名相距 1000+
行，后人调整任一侧都会静默翻转。建议显式消解：删除本规则的 color（或降为非 important），让 .wk-vi-48 的 muted-strong 唯一生效；若仍需深底浅字场景（原
CODE_TOOLBAR_PRE_CLASS 暗底），为该场景另设专用类。

  .wk-vi-code-toolbar-button-class {
    position: absolute;
    top: .35rem;
    right: .35rem;
-   color: #9fb4d8 !important;
+   /* color 移除：与 .wk-vi-48 的 text-muted-strong 重复竞争，交由 .wk-vi-48 唯一决定 */
  }


─── packages/views/src/integrations/page.tsx:1921-1921 ───
[maintainability · low] copyButtonForExternal 为死代码：全仓检索该标识符唯一匹配就是本函数定义（无调用点、无
export，测试也未引用），且本次迁移仍在持续为其替换类名（→wk-vi-170）与图标（⧉→CopyIcon），CSS 侧的 .wk-vi-170 规则也仅被该函数引用——与已确认的 {false
&& …} 死分支同病，后续每次样式调整都要陪改。建议整体删除函数及 views-integrations-u.css 中仅服务于它的 .wk-vi-170 规则。

-   return <button className={'wk-button wk-button--text wk-vi-170 ' + CODE_TOOLBAR_BUTTON_CLASS} type="button" title={t(key)} aria-label={t(key)} onClick={() => copy(value)}><CopyIcon /></button>;
+ /* 删除整个 copyButtonForExternal 函数（全仓无调用点），
+    并同步移除 views-integrations-u.css 中的 .wk-vi-170 规则 */


─── apps/web/src/settings/ModelDebugPanel.tsx:323-325 ───
[bug · medium] TDesign InputNumber 清空时 onChange 回调 value 为 null，Number(null)=0 会把 topP（min=0.01）与
maxTokens（min=1）写成越界 0。同一批次的 ModelSettingsPanel 已为此引入 fromTInputNumber 归一（注释引 OCR ocr2-016），Debug
面板三处未同步，建议复用同一工具（可提升至 model-settings.ts 导出），null 时保留上一个有效值或回退默认。

-             <div className="form-item"><label>Temperature</label><TInputNumber min={0} max={2} step={0.1} value={temperature} onChange={(value) => setTemperature(Number(value))} /></div>
-             <div className="form-item"><label>Top P</label><TInputNumber min={0.01} max={1} step={0.1} value={topP} onChange={(value) => setTopP(Number(value))} /></div>
-             <div className="form-item"><label>Max Tokens</label><TInputNumber min={1} max={8192} step={128} value={maxTokens} onChange={(value) => setMaxTokens(Number(value))} /></div>
+ <div className="form-item"><label>Top P</label><TInputNumber min={0.01} max={1} step={0.1} value={topP} onChange={(value) => { if (value != null) setTopP(Number(value)); }} /></div>


─── apps/web/src/settings/ModelSettingsPanel.tsx:996-996 ───
[bug · medium] 卡片 onKeyDown 对 Enter 未判断事件来源：焦点在卡内省略号/删除等按钮上按 Enter 时，按钮自身的 click（打开
Dropdown/Popconfirm）与冒泡上来的 keydown（openEdit）会同时触发，编辑抽屉与确认弹层叠加出现。应加 event.target ===
event.currentTarget 防护；另 role="button" 惯例还应覆盖 Space 键。

-                 onKeyDown={canEdit ? (event) => { if (event.key === "Enter") openEdit(model); } : undefined}
+ onKeyDown={canEdit ? (event) => { if ((event.key === "Enter" || event.key === " ") && event.target === event.currentTarget) { event.preventDefault(); openEdit(model); } } : undefined}


─── apps/web/src/settings/ModelSettingsPanel.tsx:967-967 ───
[maintainability · low] TLoading 的 loading 恒为 false：面板内没有列表加载态变量（列表加载由壳层 SettingsPage 承担），该包裹永远不会显示
loading，属于迁移残留/漏接状态的死代码，会误导后续维护者以为此处有加载语义。建议删除包裹或绑定真实状态。



─── apps/web/src/settings/OllamaSettingsPanel.tsx:132-138 ───
[bug · medium] 进度轮询的 catch 把任意一次瞬时失败当作终态：clearInterval + setDownloading(false) 后下载 UI
直接复位，而后端可能仍在拉取多 GB 模型，且无恢复入口。同文件 sibling 实现 ModelSettingsPanel.startDownloadPolling 明确注释「keep
polling; transient progress errors are non-fatal」并容忍瞬时错误。另外 setInterval 内无 in-flight 守卫，progress()
响应超 1s 时回调叠加，完成时可能重复触发 completed toast/refreshModels；完成分支 setDownloadModelName('')
也会清掉用户下载期间重新输入的内容。建议对齐 sibling 的容错口径并加防重叠守卫。

          } catch {
-           window.clearInterval(progressTimerRef.current!);
-           progressTimerRef.current = null;
-           pushSettingsToast(t('ollamaSettings.toasts.progressFailed'));
-           setDownloading(false);
-           setDownloadProgress(0);
+           // 对齐 ModelSettingsPanel.startDownloadPolling：瞬时错误不终止轮询；
+           // 可累计连续失败次数，超过阈值再收尾报错。
          }


─── apps/web/src/settings/PersonalMemoryPanel.tsx:49-49 ───
[bug · medium] num() 以 row[key] !== 0 作为取值条件，把合法的 0 当缺失回退默认值：extract_min_interval_seconds 的合法域是
0..86400（surface.ts memoryWorkspacePatch 校验允许 0，且本组件 InputNumber min=0），保存 0 后重载即被静默改写为 300，下次防抖保存又把
300 写回，属于无用户意图的数据漂移。旧实现（typeof === 'number' ? value : default）是保留 0 的，本次迁移引入回归。

-   const num = (key: string, fallback: number): number => (typeof row[key] === 'number' && row[key] !== 0 ? row[key] as number : fallback);
+   const num = (key: string, fallback: number): number => (typeof row[key] === 'number' ? row[key] as number : fallback);


─── apps/web/src/settings/PersonalMemoryPanel.tsx:196-196 ───
[bug · medium] InputNumber 的 onChange 用 Number(value) 未处理 TDesign 清空时的
null（Number(null)=0）：extract_delay_seconds/interest_threshold 等低于 min 的 0 写入 draft 后，debouncedSave 里
memoryWorkspacePatch 会抛出英文范围校验错误（toast 走 saveFailed 分支），且 draft 中的 0
不清除，之后任意其他字段的防抖保存都会连锁失败直到用户手工修正该字段。同批 ModelSettingsPanel 已用 fromTInputNumber 修复同类问题（OCR
ocr2-016），四处（extract_delay/extract_min_interval/interest_threshold/max_items）应统一归一。

-             <InputNumber value={draft.extract_delay_seconds} min={5} max={3600} step={15} suffix="s" disabled={!canEdit} onChange={(value) => { update({ extract_delay_seconds: Number(value) }); debouncedSave(); }} />
+ onChange={(value) => { update({ extract_delay_seconds: value == null ? DEFAULT_DRAFT.extract_delay_seconds : Number(value) }); debouncedSave(); }}


─── apps/web/src/settings/PlatformApiKeysPanel.tsx:5-5 ───
[maintainability · low] 裸 Button 导入未在组件内使用（全部使用处均为 TButton 别名），属死导入，在 noUnusedLocals/未用变量 lint
下会报错；且与上一行同源于 tdesign-react 的导入可合并。

- import { Alert, Button } from 'tdesign-react';
+ import { Alert, Button as TButton, Checkbox as TCheckbox, Input as TInput } from 'tdesign-react';


─── apps/web/src/settings/PersonalMemorySettingsPanel.tsx:791-793 ───
[style · low] 文件末尾缺少换行符（\ No newline at end of file），影响后续 diff 卫生与部分工具链（POSIX 文本约定、cat
拼接）兼容，补一个换行即可。



─── apps/web/src/settings/ModelSelector.tsx:80-80 ───
[performance · low] 每次 render 重建 settingsT 翻译器纯属浪费（readInitialLocale 语义为会话初值，结果固定），同目录
ParserEngineSettingsPanel 已有注释警告「recreating it per render」模式；建议模块级常量或 useMemo。另外 TSelect 上的静态
style={{ width: '100%' }} 与 PersonalMemoryPanel 的 style={{ minWidth: '280px' }} 等均为静态值，按规范应下沉到
settings.td.css 的 .model-selector / .setting-control 类。

-   const t = settingsT(readInitialLocale());
+ const T = settingsT(readInitialLocale()); // 模块级：locale 为会话初值，恒定


─── apps/web/src/settings/ResourceSettingsPanel.tsx:234-235 ───
[bug · medium] 数字字段从 Input(type=number) 迁移到 TInput 时丢失了 type="number" 与 min/max 约束（原 min={field.min
?? 1}、replica 字段上限 10、其余上限 64），客户端校验回归：用户可输入非数字或越界文本，当 Number(text) 非 finite 时该文本会以字符串原样写入
connection/index 配置并随保存提交，可能导致向量库连接/索引配置创建失败。另外第 13 行导入的 isReplicaField
已无任何使用点（死导入）。建议补回数字语义与范围约束（TDesign Input 支持透传 type/min/max），或在 onChange 内 clamp。

          onChange={(value) => {
            const text = String(value).trim();
+           if (!text) { onChange(undefined); return; }
+           const parsed = Number(text);
+           if (!Number.isFinite(parsed)) { onChange(undefined); return; }
+           const min = field.min ?? 1;
+           const max = field.max ?? (isReplicaField(field.name) ? 10 : 64);
+           onChange(Math.min(Math.max(parsed, min), max));


─── apps/web/src/settings/SystemGlobalSettingsPanel.tsx:474-474 ───
[bug · medium] int 型系统设置的 InputNumber 迁移时删除了原 max={9999} 上限（现在仅保留 min={minimumFor(item.key)}），且
onBlur 只做 Number 解析无范围校验，用户可输入超大整数并直接 persist 保存，缺少范围防护。建议补回上限（若各 key 上限不同可按 key 配置映射）或在 persist
前做范围校验。

-                         ? <InputNumber className="setting-input" value={typeof current === 'number' ? current : Number(current ?? 0)} min={minimumFor(item.key)} disabled={itemSaving} aria-label={keyLabel(item.key)} theme="normal" step={1} placeholder={t('system.globalSettings.tagInputPlaceholder')} onChange={(value) => { setEditValues((state) => ({ ...state, [item.key]: value })); }} onBlur={(value) => { const parsed = value === '' || value === null || value === undefined ? null : Number(value); if (parsed !== null && !Number.isNaN(parsed)) void persist(item, parsed); }} />
+                         ? <InputNumber className="setting-input" value={typeof current === 'number' ? current : Number(current ?? 0)} min={minimumFor(item.key)} max={maximumFor(item.key)} disabled={itemSaving} aria-label={keyLabel(item.key)} theme="normal" step={1}


─── apps/web/src/settings/SandboxSettingsPanel.tsx:0-0 ───
[bug · medium] 卡片保留 role="button" + tabIndex={0}，但键盘处理从原来的 Enter/Space 双键收窄为仅 Enter，且丢失了
event.preventDefault()：按 Space 既不会激活卡片还会触发页面滚动，违反 ARIA button 交互模式，键盘可达性回归。

-                     onKeyDown={(event: KeyboardEvent<HTMLDivElement>) => { if (event.key === 'Enter') openEdit(item); }}
+                     onKeyDown={(event: KeyboardEvent<HTMLDivElement>) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); openEdit(item); } }}


─── apps/web/src/settings/SandboxSettingsPanel.tsx:1631-1632 ───
[maintainability · medium] SandboxBackendBadge 在 SkillSettingsPanel.tsx（第 237
行）存在同名同逻辑的第二份实现（providerLogo mono mask + glyph 回落逻辑逐字重复，仅 type
可选性有差异），两处注释均声称“与对方共用”但实为复制而非共享。建议抽到共享模块（如 providerLogos.ts 旁或 settings
共享目录）单一实现，避免后续徽章样式/回落逻辑改动需双处同步而漂移。



─── apps/web/src/settings/SkillSettingsPanel.tsx:237-239 ───
[maintainability · medium] 此 SandboxBackendBadge 与 SandboxSettingsPanel.tsx（约 1630 行）重复实现了同一个 Vue
SandboxBackendBadge.vue 组件（logo mask + glyph 回落逻辑一致，仅 type 可选性差异）。建议合并为共享组件（单一实现），避免两处后续漂移。



─── apps/web/src/settings/SystemGlobalSettingsPanel.tsx:6-6 ───
[maintainability · low] tdesign-react 被拆成两条 import，且本条中的 Button、Input、Switch 在文件内没有任何使用点（实际渲染均走别名
TButton/TInput/TSwitch，ConfirmInline 用的是原生 <button>）——未使用导入 + 同模块重复导入。建议合并为一条并移除未用项。

- import { Button, Input, InputNumber, Loading, Select, Switch, Tag, TagInput, Tabs } from 'tdesign-react';
+ import { Button as TButton, Checkbox as TCheckbox, Dialog as TDialog, Input as TInput, InputNumber, Loading, Select, Switch as TSwitch, Tag, TagInput, Tabs } from 'tdesign-react';


─── apps/web/src/settings/SystemAuditLogPanel.tsx:92-92 ───
[maintainability · low] 紧随此注释之后的表格主体与明细抽屉被压缩成单行逾千字符的 JSX，内部包含多层嵌套三元（rows/loadError/cursor 分支、key 回退
`text(row,'id')==='—' ? ... : ...`、tone 回退 `AUDIT_TAG_TONES[...] ?? AUDIT_TAG_TONES.neutral`
等），违反嵌套三元表达式规范且严重损害可读性与 diff 可维护性。建议把 rows.map 的行渲染拆为局部渲染函数/组件，分支条件提取为变量，按列拆分 JSX 块。



─── apps/web/src/settings/PersonalMemoryPanel.tsx:116-118 ───
[maintainability · medium] `as never` 断言会完全关闭该 API 载荷调用的类型检查：memoryWorkspacePatch 的返回类型与
client.settings.memory.workspace.update 的参数 SettingsPayload（Record<string,
unknown>）本可直接赋值（surface.ts 签名已核对匹配），旧实现也无此断言。never 可赋值给任意参数类型，今后 patch 返回结构或 update
签名变化时编译器将静默放行。建议直接删除该断言。

                embeddingModelId: next.embedding_model_id,
              },
-           ) as never);
+           ));


─── apps/web/src/settings/PersonalMemorySettingsPanel.tsx:810-812 ───
[bug · low] usage 帮助弹层从旧实现（aria-expanded + onFocus/onBlur/onClick 三通道切换）退化为仅 trigger='hover'：TDesign
中 'hover' 与 'focus' 是互斥的独立触发值，键盘用户 Tab 聚焦 info 按钮后将无法打开记忆使用说明，属可访问性回归。若当前版本 Popup 支持空格分隔的组合 trigger
则写 'hover focus'；否则改回受控 visible + 按钮 onClick 切换，恢复键盘可达。

              placement='bottom-left'
-             trigger='hover'
+             trigger='hover focus'
              overlayClassName='memory-usage-popup-overlay'


─── apps/web/src/settings/ParserEngineSettingsPanel.tsx:289-290 ───
[style · low] 新引入嵌套三元 `loading ? … : error ? … : …`（EngineCard 状态徽处 `statusTone === 'on' ? … :
statusReason ? … : …` 同款一处），违反检查项「禁止嵌套三元表达式」，JSX 中多层三元也降低分支可读性。建议提取为独立 render 分支函数，按 if 提前返回。

-       <span>{t('settings.parser.loading')}</span>
-     </div> : error ? <div className="error-inline">
+   // 组件内提取分支函数，渲染处改为 {renderListArea()}
+   function renderListArea() {
+     if (loading) return <div className="loading-state"><Loading size="small" /><span>{t('settings.parser.loading')}</span></div>;
+     if (error) return <div className="error-inline"><Alert theme="error" message={error} operation={…} /></div>;
+     return <>…列表/空态…</>;
+   }


─── apps/web/src/settings/TenantMembersPanel.tsx:710-710 ───
[bug · medium] 过期闭包（bug）：`revoke` 内 `if (!canManage || busy) return` 的防重守卫与成功后 `await
loadInvitations()` 的默认参数（invitationsPage/invitationsPageSize）都来自闭包捕获，而本 useMemo 依赖缺
`busy`、`invitationsPage`、`invitationsPageSize`，且 `tr`（locale 派生）/`canManage` 基本不变——memo
只在挂载时执行一次，单元格永远持有挂载时的 `revoke`。后果：① busy 守卫永远读到挂载时的 false，请求在途时再点另一行的 Popconfirm
确认会并发发起第二次撤销（旧实现确认按钮有 disabled={busy}，新 TPopconfirm 的 confirmBtn 也未设 disabled）；② 翻到第 2 页后执行撤销，成功后
`loadInvitations()` 刷新的是捕获的第 1 页，列表跳回。建议补全依赖数组。

-   ]), [tr, canManage, locale]);
+   ]), [tr, canManage, locale, busy, invitationsPage, invitationsPageSize]);


─── apps/web/src/settings/TenantMembersPanel.tsx:763-763 ───
[bug · medium] 同族过期闭包（bug）：`update`/`remove` 成功后 `await load()` 走默认参数（捕获的 `page/queried/pageSize`），本
useMemo 依赖虽含 `busy`，但缺 `page`、`queried`、`pageSize`——在最近一次 busy 翻转之后发生的翻页/搜索不会重建列定义。例：挂载 → 搜索 "alice"
→ 对结果执行改角色/移除 → `load()` 用挂载时捕获的 `page=1, queried=''` 刷新，搜索结果被整表第 1 页覆盖而搜索框文本仍在。建议补全依赖。

-   ]), [tr, canManage, currentUserId, busy, locale]);
+   ]), [tr, canManage, currentUserId, busy, locale, page, queried, pageSize]);


─── apps/web/src/settings/TenantMembersPanel.tsx:299-302 ───
[bug · medium] 双请求竞态（bug）：tdesign Pagination 在 pageSize 变更时常会同时调整 `current`（如第 3 页改大 pageSize 被
clamp/重置），此时两个 if 同时命中：`onPageSize(size)` → `loadInvitations(1, size)`，`onPage(current)` →
`loadInvitations(current)`（后者仍用旧 pageSize 默认值）。两次并发请求对同一组
state（列表/total/pageSize）竞争写入，最终展示取决于响应到达顺序，可能出现 pageSize 状态与数据不一致。旧实现两控件分离无此问题。建议 pageSize
变更分支短路返回（两个消费方 onPageSize 均已重置到第 1 页，语义等价且单请求）。

        onChange={(pageInfo: { current: number; previous: number; pageSize: number }) => {
-         if (pageInfo.pageSize !== pageSize) onPageSize(pageInfo.pageSize);
+         if (pageInfo.pageSize !== pageSize) { onPageSize(pageInfo.pageSize); return; }
          if (pageInfo.current !== page) onPage(pageInfo.current);
        }}


─── apps/web/src/settings/TenantMembersPanel.tsx:696-698 ───
[maintainability · low] 死代码（maintainability）：手搓确认气泡被 TPopconfirm
取代后，以下代码已无消费者，建议一并清理：`revokeConfirmKey`/`removeConfirmKey` 状态（368-369 行，现仅剩
`setRevokeConfirmKey(null)`/`setRemoveConfirmKey(null)` 的空调用，状态恒为 null）、`pageWindow`（240
行）、`maxMembersPage`（663 行）、TablePager 内的 `maxPage`/`jump`/`setJump`/同步
useEffect/`commitJump`（267-275 行，跳页已由 tdesign showJumper 内部处理）。



─── apps/web/src/settings/TenantUserProfileSections.tsx:312-312 ───
[bug · medium] parity 缺口（bug）：`onDeleted` 硬编码跳转 /login。Vue TenantInfo.vue 的 deleteCurrentTenant
在删除成功后会切换到下一个剩余工作空间（home 或首个 membership），仅当无剩余 membership 时才登出。多空间 owner
删除当前空间后被强制登出，其余空间的会话被无谓丢弃。此行为沿袭自旧 SettingsPage 挂载点，但既然本次迁入 TenantInfoSection，建议顺手对齐 Vue
语义（先尝试切换剩余空间，无剩余再 assign('/login')）。



─── apps/web/src/settings/TenantUserProfileSections.tsx:346-346 ───
[documentation · low] 注释复制粘贴残留：「T12a TDesign 同构迁移」一句在同一行内重复拼接两次，影响可读性，删其一。



─── apps/web/src/settings/TenantUserProfileSections.tsx:300-300 ───
[style · low] 静态 inline style（style）：`style={{ flex: 1 }}` 非动态样式，应平移入 settings.td.css（§5
usage-control 族），与本次迁移「utilities/inline 收编进 td.css」的口径保持一致。



─── apps/web/src/settings/SystemInfoPanel.tsx:93-93 ───
[maintainability · low] 魔法下标（maintainability）：`index === 1` 隐式耦合 systemInfoRows 的行序（第 2
行为前端版本行）来挂「版本不一致」警告标签。surface.ts 行序一旦调整（如新增行、重排），警告会静默挂到错误行上，无任何报错。建议在 systemInfoRows 的行对象上加稳定标识（如
`key: 'uiVersion'`）并按 key 匹配，替代下标判断。



─── apps/web/src/settings/SystemInfoPanel.tsx:37-37 ───
[maintainability · low] 硬编码 + 每渲染重建（maintainability）：外链 URL 硬编码在组件体内，且 `reportIssueURL` 的 IIFE（含
URLSearchParams 拼装）每次渲染都重新执行。建议将 URL 与 body 拼装提为模块级常量/纯函数（仅依赖 migrationError 的部分可做成
`buildReportIssueURL(migrationError)`），与项目 RBAC_DOC_URL 等集中管理的外链常量惯例对齐。



─── apps/web/src/settings/SystemInfoPanel.tsx:92-92 ───
[style · low] 嵌套三元 + 静态 inline style（style）：tagTone 的 danger?warning?default 链式三元违反嵌套三元规约，且
`style={{ marginLeft: '8px' }}` 为静态样式（下方 migration Alert 的 `style={{ width: '100%' }}` 同理）。建议：tone
映射用查表（`const TAG_THEME = { danger: 'danger', warning: 'warning' }`，缺省 'default'），间距/宽度平移到
settings.td.css §16。



─── apps/web/src/settings/SystemInfoPanel.tsx:110-110 ───
[security · low] 类型绕行（security）：以 `{...EXTERNAL_LINK_REL}` spread 绕过 tdesign Link 未声明 `rel`
的类型检查，依赖库运行时把未知 props 透传到 <a>。若日后升级的库版本过滤未声明 props，这两个外链将静默丢失 noopener/noreferrer（反向 tabnabbing
风险）。建议改用原生 <a>（样式对齐 t-link 即可），或封装一个显式携带 rel 的 Link 包装组件，消除对透传行为的隐式依赖。



─── apps/web/src/settings/settings-toast.tsx:63-67 ───
[style · low] 嵌套三元（style）：tone 三分支 className 链可整体消除——CSS 侧
`wk-settings-toast--error/--warning/--success` 三态类已齐备且与 tone 值一一对应，直接模板拼接即可，同时消除嵌套三元与三份重复类名前缀。

-               ? 'wk-settings-toast wk-settings-toast--error'
-               : toast.tone === 'warning'
-                 // MessagePlugin.warning 琥珀色语义（WeKnoraCloud 部分成功/fillRequired，T12b）。
-                 ? 'wk-settings-toast wk-settings-toast--warning'
-                 : 'wk-settings-toast wk-settings-toast--success'
+           className={'wk-settings-toast wk-settings-toast--' + toast.tone}


─── apps/web/src/settings/settings.td.css:3422-3427 ───
[bug · medium] 同名类取值冲突（bug）：`.hint-popover` 在本文件 §4（envvars，gap:12px、__text margin 4px 0 0）与
§14（sandbox，gap:4px、__text margin 0）两处全局定义且取值不同。两条规则同为 unscoped、特异性相同，§14 靠文件顺序无条件覆盖 §4——envvars
的提示弹层将错误应用 sandbox 的紧凑间距。两处分别对应不同 Vue SFC 的事实源，应各自改名（如 `.env-hint-popover` /
`.sandbox-hint-popover`）或并入语义前缀，恢复各归其位。



─── apps/web/src/settings/settings.td.css:5000-5013 ───
[bug · medium] 跨文件同名类级联冲突（bug）：settings-wrapper.css 仍保留通用版 `.wk-settings-panel-heading { …
margin-bottom: 32px }`（0,1,0），本条 §S6 新增的抽屉版带 `.wk-settings-drawer-root` 前缀（0,2,0）且附
sticky/border/白色背景/16px 下边距。凡在设置抽屉内使用该类、仍期望壳层 32px 标题的分区（usage/query-history/mcp 等，wrapper.css 中
McpSettings 段注释明确声明「display/gap/margin-bottom 由共享 .wk-settings-panel-heading 规则继续供给」）都会被高特异性的
sticky+border+16px 版本覆盖，产生下边距减半、多出粘性边框的视觉回归。建议将本条改名为模型编辑器专属类（如
`.wk-model-editor-heading`），避免一个类名承载两种解剖结构、靠注入顺序/特异性分胜负。



─── apps/web/src/settings/SandboxSettingsPanel.tsx:1135-1135 ───
[bug · high] SandboxConfigEditor 表单布局样式丢失：`import './sandbox-settings.css'` 已移除且该文件已删除，但表单内仍在大量使用
wk-net-row、wk-sandbox-rows、wk-sandbox-editor-section、wk-sandbox-section-row、wk-net-rule、wk-template-
card、wk-template-row、wk-template-fields 等类，而全仓库（settings.td.css / settings-wrapper.css）已无任何对应规则——仅
.wk-form-grid/.wk-field-error/.wk-tag/.wk-settings-editor 被迁入 settings.td.css。后果：网络规则行/注入 header
行/环境变量行（TInput + 删除按钮）失去 flex 行布局会垂直堆叠，编辑器各 section 失去垂直间距，运行时/网络/环境变量步骤 UI
明显回归。注释虽称编辑器"待后续批次收编"，但删除样式文件使当前状态既非旧栈也非新栈。建议将这些类的规则补迁入 settings.td.css（或恢复 sandbox-settings.css
直至组件收编完成）。



─── apps/web/src/settings/ResourceSettingsPanel.tsx:960-960 ───
[bug · high] websearch 卡片的 admin 操作下拉按钮永久不可见：按钮 inline style 设了 `opacity: 0`，但 settings-wrapper.css
中 `.rs-card__more` 仅有 `flex-shrink: 0`、`.rs-card:hover` 只有 box-shadow，没有任何将 opacity 恢复的规则（普通规则也无法覆盖
inline style，除非 !important）。上方注释自称复刻 Vue"常驻 provider-card__actions"——Vue 常驻即可见；storage 卡的隐形按钮是 hover
显形模式，而这里既非常驻也无显形，结果 admin 只能盲点这个不可见但仍占位可点击的按钮（编辑尚可点卡片进入，删除需经抽屉）。建议去掉 inline opacity: 0（常驻可见），或改由 CSS
实现 `.rs-card:hover` 显形并加 !important/移除 inline。



─── apps/web/src/settings/SandboxSettingsPanel.tsx:1344-1344 ───
[bug · medium] TInputNumber 的 onChange 判空不完整：TDesign InputNumber 在清空输入时 onChange 回调传的是 null
而非空字符串。此处 `value === '' ? undefined : Number(value)` 会把 null 变成 Number(null) = 0（"清空/未设置"退化为 0），而
defaultTimeoutSec 分支的 `String(value)` 会得到字符串 "null" 写入表单状态。同批迁移的 SystemGlobalSettingsPanel 中
InputNumber onBlur 特意做了 `value === '' || value === null || value === undefined`
三重判空，说明该路径真实存在，此处应统一按 null/undefined 一并归一为未设置。



─── apps/web/src/settings/SystemGlobalSettingsPanel.tsx:420-420 ───
[bug · medium] 高危列表的 TagInput 新增了 `clearable`：一键清除按钮会触发 `onChange([])` → 直接 `onAdminsCommit([])`
把空的系统管理员邮箱列表提交到后端，误触一下即清空全部系统管理员（SSRF 白名单行同理），无任何确认步骤。原手搓 TagInput 只能逐个删除 tag，不存在一键清空路径；Vue
t-tag-input 基线也未见对 admins 这一 danger 徽章配置使用 clearable。建议对 admins/SSRF 这类高危配置移除 clearable，或在清除触发时要求确认。



─── apps/web/src/settings/SettingsPage.tsx:118-118 ───
[maintainability · low] 死代码：集合剩余的五个条目（models/members/mcp/sandbox/skills）在下方 portedPanel
三元链上都有各自的显式分支（`key === 'mcp' ? … : key === 'models' ? …` 等），永远先于
`PARTIALLY_PORTED_SECTIONS.has(key)` 被消费，因此 `.has(key)` 恒为 false——与已删除的四个死条目同性质，整个 Set
已无实际作用。同理，fallback 分支的 `sectionErrorMode(key) === 'banner-retry'` 壳层横幅段也不可达（banner-retry 的 4 个 key 中
members 被 setError(null)、parser/system/userprofile 均走 SELF_HEADER_SECTIONS 早退），而第 63-69
行注释仍描述"parser 的横幅在壳层渲染并替代内容"，与现实相反（ParserEngineSettingsPanel 已自持 error）。建议清空/删除该 Set 与不可达的
banner-retry 段，并同步修正注释。



─── packages/views/src/chat/page.tsx:617-621 ───
[bug · high] 会话视图（selectedSessionId 早退路径）整体丢失了引用面板：referencesOpen 经 MessageList 的 onToggleReferences
/ RagPipelineProgressFace 的「检索完成」折叠根切换后，仅在 .chat 上追加 has-references-panel class，但该路径下没有任何
<ReferenceList> 渲染——ReferenceList 只在早退之后的空态路径（约 781 行）出现，而那里 selectedSessionId
恒为空、永远不可能有带引用的消息。activateCitation 设置的 activeCitationId 同样只被 781
行消费。结果：选中会话后点击「检索完成」展开或点击正文引用角标，界面上无任何面板出现（apps/web 宿主也没有 ReferencesDrawer 兜底）。旧实现中引用面板位于会话视图的
.wk-chat-conversation 内。建议在早退 JSX 内（如 .chat_thread 之后）补回 ReferenceList 渲染。

-     return <div
-       className={'chat'
-         + (referencesOpen ? ' has-references-panel' : '')
-         + (terminalOpen ? ' has-sandbox-panel' : '')}
-       style={{ '--sandbox-panel-width': '420px' } as React.CSSProperties}>
+       {/* 补回会话视图的共享引用面板（旧 .wk-chat-conversation 内行为） */}
+       {referencesOpen && references.length > 0 ? (
+         <ReferenceList references={references} activeId={activeCitationId} onActivate={activateCitation} copy={copy} />
+       ) : null}
+       <div
+         className="scroll-to-bottom-btn wk-chat-scroll-bottom"


─── packages/views/src/chat/page.tsx:629-631 ───
[bug · medium] headerSlot 缺省时的回退面 ⋯ 按钮（chat-header__menu-btn）没有任何
onClick，是一个死端控件：点击无反应。ChatPageProps 仍保留
onRenameSession/onToggleSessionPin/onDeleteSession/onClearSession/headerUtilityItems
回调，但包内回退路径一个也不消费——重命名/置顶/清空/删除/复制链接等会话管理动作在未注入 headerSlot 的消费方（当前如 chat-header-hook-order.test 等直接渲染
ChatPage 的用例，以及未来任何复用方）全部不可达。建议：回退面至少挂接既有 props 回调的最简菜单，或在没有 headerSlot 且无任何会话动作回调时不渲染该按钮。

-           <button type="button" className="chat-header__menu-btn wk-chat-header-menu" aria-label={copy.moreActions}>
-             <svg className="t-icon t-icon-ellipsis" viewBox="0 0 24 24" width="16px" height="16px" fill="none" aria-hidden="true"><use href="#t-icon-ellipsis" /></svg>
-           </button>
+           <button type="button" className="chat-header__menu-btn wk-chat-header-menu" aria-label={copy.moreActions}
+             onClick={() => {
+               /* 回退面：至少接通既有回调（或当无任何会话动作回调时直接不渲染） */
+               if (props.onToggleSessionPin && selectedSession) void props.onToggleSessionPin(selectedSession.id, !selectedSession.is_pinned);
+             }}
+           >


─── packages/views/src/chat/page.tsx:681-688 ───
[bug · low] 回底按钮从带 aria-label 的 <button> 降级为纯 onClick 的 <div>：无 role、无 tabIndex、无
aria-label，键盘用户（Tab/Enter）与读屏用户均无法触达。旧实现是 button[aria-label]，虽然 Vue 事实源 index.vue:147 也是 div，但相比迁移前的
React 行为这是可访问性回退，且成本极低。建议补 role="button"/tabIndex={0}/aria-label 并处理 Enter/Space。

        <div
+         role="button" tabIndex={0} aria-label={copy.chatScrollBottom}
          className="scroll-to-bottom-btn wk-chat-scroll-bottom"
          style={{ display: userScrolledUp ? undefined : 'none' }}
          onClick={() => {
+           const box = scrollBoxRef.current;
+           if (box) box.scrollTo({ top: box.scrollHeight });
+         }}
+         onKeyDown={(event) => {
+           if (event.key === 'Enter' || event.key === ' ') {
+             event.preventDefault();
-           const box = scrollBoxRef.current;
+             const box = scrollBoxRef.current;
-           if (box) box.scrollTo({ top: box.scrollHeight });
+             if (box) box.scrollTo({ top: box.scrollHeight });
+           }
          }}
        >


─── packages/views/src/chat/message-list.tsx:217-218 ───
[bug · medium] hasStream 读取的 row.agentEventStream 在整个 React 代码库（packages +
apps/web）中没有任何写入点——全库唯一出现处就是本行（Vue 侧由 useChatStreamHandler.ensureRagPipelineHistoryStream 恢复填充，React
没有对应物）。该分支恒为 false，属于死条件：完成态消息若正文/引用为空但 agent_steps 非空（如 steer 分段、答案在 agent 事件里的历史消息，Vue
注释明确警告『Hiding that row loses the reply entirely on reload』），在 React 侧会被误判为 is-empty-segment 整行隐藏。同文件
384 行的 row.steerForked 同样无写入方。建议改用 React 数据面真实存在的信号（如 assistantTimelineItems(message).length > 0）替换
agentEventStream，或补齐事件流恢复逻辑后再启用该判定。

-   const hasStream = Array.isArray(row.agentEventStream) && (row.agentEventStream as unknown[]).length > 0;
+   // agentEventStream 在 React 数据面无写入方（Vue 侧由 ensureRagPipelineHistoryStream
+   // 填充），此处用 React 可得的等价信号（agent_steps 工具时间线）替代。
+   const hasStream = assistantTimelineItems(message).length > 0;
    if (hasStream) return true;


─── packages/views/src/chat/message-face.tsx:199-199 ───
[bug · medium] docCount 与 webCount 均为 0 时整个「检索完成」折叠根 return null，与被替换的旧 AssistantExtras
完成态不一致（旧实现恒渲染折叠根按钮，仅计数 span 在 0 时不渲染），也与注释宣称的 Vue 对齐不符：Vue RagPipelineProgress 的 visible
计算（RagPipelineProgress.vue:476-481）为 steps.length > 0 || ... 即「检索跑了但零引用」仍渲染，referenceSummaryText
为空串而已。纯 DB 查询/工具型完成回合在 React 侧会丢失检索摘要行。建议：零引用时不提前 return，referenceText 走空串并在 span 渲染处判空。

-   if (docCount === 0 && webCount === 0) return null;
+   // 与旧 AssistantExtras/Vue visible 保持一致：零引用仍渲染「检索完成」折叠根，
+   // 仅引用计数 span 判空隐藏。
+   // if (docCount === 0 && webCount === 0) return null;


─── packages/views/src/chat/message-face.tsx:401-401 ───
[bug · low] renderChatMarkdown(props.content, {}) 丢失了旧路径 renderMessageHtml(message,
t.invalidImageLink) 传入的本地化 invalidImageLink 标签，失效图片链接将回退到 markdown.ts 内置的
INVALID_IMAGE_LINK_PLACEHOLDER 常量文案，非中英文 locale 下出现未本地化占位（XSS 方面无碍，marked 渲染链路有转义/白名单）。建议第二参传 {
invalidImageLabel: props.copy.invalidImageLink }（与 message-list.tsx:97 的 renderMessageHtml 包装等价）。

-   const html = renderChatMarkdown(props.content, {});
+   const html = renderChatMarkdown(props.content, { invalidImageLabel: props.copy.invalidImageLink });


─── packages/views/src/chat/message-face.tsx:287-287 ───
[maintainability · low] 此处私有复刻了 writeClipboardText（clipboard API + execCommand 回退），而
message-list.tsx:101 已导出功能相同的同名函数，两份实现将来极易漂移（如某一份补 secure-context 处理）。建议删除本地副本，改为 `import {
writeClipboardText } from './message-list.tsx'` 复用。

- async function writeClipboardText(text: string): Promise<void> {
+ // 删除本地实现，复用导出：import { writeClipboardText } from './message-list.tsx';


─── packages/views/src/chat/message-face.tsx:127-127 ───
[bug · low] images 数组元素类型为 { url?: string }，img.url 可能为 undefined，直接绑定 src={img.url} 会渲染出无 src 的
<img>（不同浏览器可能显示断裂图标）。与同文件 attachments 卡片的判空风格也不一致。建议 url 为空时跳过渲染。

-           {images.map((img, idx) => <img key={idx} src={img.url} className="user_image_thumb" alt="" />)}
+           {images.map((img, idx) => img.url ? <img key={idx} src={img.url} className="user_image_thumb" alt="" /> : null)}


─── packages/views/src/chat/composer.tsx:238-239 ───
[bug · medium] kbTipTimer 只在 mouseEnter/mouseLeave 中互相 clear，组件卸载时没有任何 clearTimeout 清理：若在 300ms 开（或
80ms 关）窗口内卸载组件，定时器仍会触发并对已卸载组件调用 setKbTipOpen，同时造成定时器泄漏。建议补一个卸载清理 effect（与同文件/同库 AnswerToolbar 的
timer 清理模式一致）。

    const [kbTipOpen, setKbTipOpen] = useState(false);
    const kbTipTimer = useRef<number | null>(null);
+   useEffect(() => () => { if (kbTipTimer.current !== null) window.clearTimeout(kbTipTimer.current); }, []);


─── packages/views/src/chat/composer.tsx:538-538 ───
[bug · low] 提及菜单重构后两个残留问题：1) 搜索输入框已删除，mentionQuery 仅剩 toggleMentions/closeMentions 两处重置写入，恒为空串，306
行的 filter 对空串恒真——成为死状态，且「输入过滤」能力实际失效；2) 删除输入框后，wk-chat-mention-option-* id 不再被任何
aria-activedescendant 引用（textarea 也未设），读屏无法播报 ArrowUp/Down 移动时的高亮项，listbox 的 aria 键盘语义断链。建议：移除
mentionQuery 死状态（或恢复过滤入口），并在 mentionOpen 时给 textarea 补 aria-activedescendant 指向当前 activeMentionIndex
项。

-             <div id="wk-chat-mention-listbox" role="listbox" aria-label={t.mentionKnowledge} className="mention-menu" style={mentionMenuStyle ?? undefined} onClick={(event) => event.stopPropagation()}>
+             <div id="wk-chat-mention-listbox" role="listbox" aria-label={t.mentionKnowledge} className="mention-menu" style={mentionMenuStyle ?? undefined} onClick={(event) => event.stopPropagation()}
+               /* 键盘焦点保留在 textarea，需由其 aria-activedescendant 锚定当前项：
+                  aria-activedescendant={`wk-chat-mention-option-${filteredMentionOptions[Math.min(activeMentionIndex, filteredMentionOptions.length - 1)]?.id ?? ''}`} */
+             >


─── packages/views/src/chat/chat-copy.ts:503-503 ───
[bug · low] ko（韩语）与 ru（俄语）两表的 requestInfoEmpty 新键均为英文占位 'No request info available'，而同批次新增的 noResult
/ referencesWebCount / referencesDocAndWebCount 在这两表里都已本地化——韩语/俄语用户在请求信息弹层空态会看到未翻译文案。建议补译（ko: '요청
정보가 없습니다'，ru: 'Сведения о запросе отсутствуют'）。

- requestInfoEmpty: 'No request info available',
+ /* ko */ requestInfoEmpty: '요청 정보가 없습니다',
+ /* ru */ requestInfoEmpty: 'Сведения о запросе отсутствуют',


─── packages/views/src/chat/session-sidebar.tsx:307-307 ───
[maintainability · low] 每行渲染中 apiOwnerTagOf(session) 被调用 3 次（条件判断 + kind + full + label 实为 4 次），并伴随
3 处非空断言 `!`——若函数内部演进为带副作用/开销的实现会成倍放大，且断言掩盖了返回 null 的可能性。建议在 map 回调顶部求值一次（const ownerTag =
apiOwnerTagOf(session)）后复用。

-                       {apiOwnerTagOf(session) ? <span className={'session-owner-tag session-owner-tag--' + apiOwnerTagOf(session)!.kind} title={apiOwnerTagOf(session)!.full}>{apiOwnerTagOf(session)!.label}</span> : null}
+                       {(() => {
+                         const ownerTag = apiOwnerTagOf(session);
+                         return ownerTag
+                           ? <span className={'session-owner-tag session-owner-tag--' + ownerTag.kind} title={ownerTag.full}>{ownerTag.label}</span>
+                           : null;
+                       })()}


─── apps/web/src/settings/TenantMembersPanel.tsx:911-911 ───
[bug · critical] 编译错误（未定义标识符 Icon）：本文件 import 已改为 `import { Icon as TIcon } from
'tdesign-icons-react'`，全文件不存在名为 `Icon` 的绑定，也无全局 Icon/JSX 声明兜底，但仍有 3 处裸 `<Icon>` 用法：① 本行（audit 展开箭头
chevron-down）；② 审计刷新按钮 `<Icon name="refresh" />`（audit-refresh-btn TButton 内）；③ 分享链接对话框 `<Icon
name="copy" />`（copyText TButton 内）。三处均为 TS2304 "Cannot find name 'Icon'"，文件无法编译。建议统一改为
`<TIcon>`（tdesign Icon 的 name prop 兼容这些图标名）。

-                             <span className={'audit-expand-toggle' + (expanded ? ' is-open' : '')}><Icon name="chevron-down" /></span>
+                             <span className={'audit-expand-toggle' + (expanded ? ' is-open' : '')}><TIcon name="chevron-down" /></span>
+                             {/* 同文件另两处：<Icon name="refresh" /> 与 <Icon name="copy" /> 一并改为 <TIcon name="refresh" /> / <TIcon name="copy" /> */}


─── apps/web/src/settings/TenantUserProfileSections.tsx:223-223 ───
[bug · high] 行为回归（parity 缺口）：Vue 事实源 TenantInfo.vue:472-480 的 canSubmitDescription
明确注释「描述允许为空（业务上是可选字段），可提交条件不要求非空，只要内容变了即可」，即 `!savingDescription && editDescriptionTrimmed !==
(description || '')`。React 版新增 `disabled={!descriptionDraft.trim()}`
后，「把已有描述清空保存」这一合法操作被按钮禁用阻断（saveDescription 函数体内无此守卫，按钮是唯一闸门）。建议对齐 Vue 条件，去掉非空要求。顺带：名称字段 Vue
canSubmit 还含「内容变更」判断（editNameTrimmed !== name），React 的 saveName/按钮仅判非空，未变更也会发请求，可一并对齐。

-                       <TButton theme="primary" size="small" loading={savingDescription} disabled={!descriptionDraft.trim()} onClick={() => { void saveDescription(reload); }}>
+                       <TButton theme="primary" size="small" loading={savingDescription} disabled={savingDescription || descriptionDraft.trim() === (currentDescription || '')} onClick={() => { void saveDescription(reload); }}>


─── apps/web/src/settings/settings.td.css:5349-5352 ───
[bug · medium] 同名 @keyframes 重复定义且参数冲突：`wk-sandbox-inventory-enter` 在本文件出现两处定义——§14 保留段（约 3800
行，from translateX(18px) opacity .7）与本处 §S6 段（from translateX(24px) opacity 0）。@keyframes
全局生效且后者按文件顺序无条件覆盖前者，inventory 抽屉实际只走本处版本，§14 版本成为死代码且参数差异会误导后续维护（两段注释各自宣称是「事实源平移」）。建议删除其一，只保留单一定义（若
Vue 原版为 24px/0，删 §14 段；反之删本段）。

- @keyframes wk-sandbox-inventory-enter {
-   from { transform: translateX(24px); opacity: 0; }
-   to { transform: translateX(0); opacity: 1; }
- }
+ /* 删除本段重复定义，仅保留 §14 段（或反向：删 §14 段并确认本处参数与 Vue SandboxSettings.vue 原版一致），全文件只留一个 wk-sandbox-inventory-enter 定义 */


─── apps/web/src/settings/TenantDeleteZone.tsx:66-66 ───
[style · medium] 静态 inline style + 作用域类误命中：① `style={{ color/fontSize/margin }}` 全为静态值，违反「inline
style 仅用于动态样式」规约，应平移入 settings.td.css；② 该 aside 渲染在 `.tenant-info` 子树内（TenantInfoSection 的
tenant-info-body），会命中 settings.td.css §5 的 `.wk-settings-drawer-root .tenant-info .error-inline {
padding: 20px 0; }`——该类语义是「加载失败横幅容器」，此处却是危险区下方的行内错误文本，padding 20px 0 与 inline margin 8px 叠加后上间距达
28px，明显偏离预期。建议改用独立语义类（如 .delete-space-error）并全部入 CSS。

-         {error ? <p role="alert" className="error-inline" style={{ color: 'var(--td-error-color)', fontSize: 13, margin: '8px 0 0' }}>{error}</p> : null}
+         {error ? <p role="alert" className="delete-space-error">{error}</p> : null}
+ /* settings.td.css §5：.delete-space-error { margin: 8px 0 0; font-size: 13px; color: var(--td-error-color); } */


─── packages/views/src/chat/message-face.tsx:120-120 ───
[bug · high] steer 失败面字段名不匹配，导致整条失败 UI 永不渲染：React 宿主 apps/web/src/chat/steer-preview.ts
markSteerPreviewFailed（:67）在乐观行上写入的是 `steer_failed: true`（该文件头注释明确 `steer_failed` → Vue
`_steerFailed` 的映射），而这里读取 `row._steerFailed === true` 在 React 侧永远为 false——注入失败后 t.steerFailed
提示与重试/移除按钮都不会出现，乐观气泡看起来像已正常发送（静默失败，steerFailed 文案键也随之失去全部消费者）。另外 message-list.tsx 调用 UserMessageFace
时把 onRetrySteer/onRemoveSteer 硬编码为 undefined，而 ChatPageProps 已有 onSteerRetry/onSteerRemove 且
ChatRoutePage（:1991）有传——建议改为读 `row.steer_failed === true`，并在 MessageListProps 增加对应回调透传，避免失败态双重死亡。

-   const steerFailed = row._steerFailed === true;
+   const steerFailed = row.steer_failed === true;


─── packages/views/src/chat/session-sidebar.tsx:305-305 ───
[bug · medium] 旧版行渲染在 session.parent_session_id 存在时显示 ⑂ 分叉徽标（aria/title = forkBadgeTooltip），本次 Vue
结构平移把它删掉了且无替代：contracts 的 lineage 字段（ChatSession.parent_session_id，A11
注释）、分叉功能链路（onForkMessage/canForkMessage/fork-point）以及 chat-copy.ts 五个语言包的 forkBadgeTooltip 全部保留，但新
DOM 不再消费——分叉会话与普通会话在侧边栏不可区分，forkBadgeTooltip 成为孤儿文案键。建议在 pin 图标旁按 parent_session_id 补回徽标。

                        {session.is_pinned ? <svg className="t-icon submenu_pin_icon" viewBox="0 0 24 24" width="1em" height="1em" fill="none" aria-hidden="true"><use href="#t-icon-pin" /></svg> : null}
+                       {session.parent_session_id ? <span className="submenu_fork_icon" role="img" aria-label={t.forkBadgeTooltip} title={t.forkBadgeTooltip}>⑂</span> : null}


─── packages/views/src/chat/message-list.tsx:375-375 ───
[bug · low] msg_list 从 <ol aria-label> 降级为无 role 的 <div aria-label>：aria-label 在无角色元素上会被读屏忽略，子项也由
<li> 变为普通 <div class="msg-item-wrapper">，列表结构/条目数不可感知；同批删除的还有时间戳的 role="separator"。Vue 事实源使用无语义 div
可以理解，但本迁移在其他节点（role/aria 锚点、details 菜单、wk-* hook）都刻意保留了 React 侧既有语义钩点，此处属可访问性回退。建议容器补
role="list"、条目补 role="listitem"，时间戳恢复 role="separator"。

-     <div className="msg_list wk-chat-messages" aria-label={t.messagesLabel}>
+     <div className="msg_list wk-chat-messages" role="list" aria-label={t.messagesLabel}>


─── packages/views/src/chat/message-face.tsx:462-462 ───
[style · low] BotMessageFace 内层布局 div 使用固定的内联 style（display/flexDirection/gap 均为常量），与本次迁移「几何全部语义化为
wk-vc-*/语义类、由 views-chat-u.css 承载」的方向不一致（同文件其余结构均已类化），内联样式无法被 chat.td.css 主题层覆盖也不易测试。建议改为语义类（如
wk-vc-message-face-stack 或并入 .rag-answer-stack 的容器规则）。

-       <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
+       <div className="bot_msg__stack">


LLM retry report summary: 18 of 654 requests affected -- 12 requests failed, 3 requests cancelled, 3 requests recovered after retry

Review planning (4 requests):
- apps/web/src/agents/AgentEditorModal.tsx,apps/web/src/agents/AgentParserRules.tsx,apps/web/src/agents/AgentsPage.tsx,apps/web/src/agents/MbtiTestModal.tsx,apps/web/src/agents/PersonaSection.tsx,apps/web/src/agents/SubagentsSection.tsx,apps/web/src/agents/agents-u.css,apps/web/src/agents/agents.css,apps/web/src/agents/agents.td.css,apps/web/src/agents/list.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/data-sources/DataSourcesPage.tsx,apps/web/src/data-sources/data-sources-u.css,apps/web/src/data-sources/ui.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/platform/GlobalCommandPalette.tsx,apps/web/src/platform/InvitationInbox.tsx,apps/web/src/platform/PlatformShell.tsx,apps/web/src/platform/platform-shell.td.css,apps/web/src/platform/platform-u.css,apps/web/src/platform/retrieval-settings-panel.tsx,apps/web/src/platform/session-activity.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/analytics/AnalyticsPage.tsx,apps/web/src/analytics/analytics-u.css: rate limited (HTTP 429) -> succeeded

Core review (9 requests):
- apps/miniprogram/src/career/export-deletion.config.ts,apps/miniprogram/src/career/export-deletion.gating.ts,apps/miniprogram/src/career/export-deletion.tsx,apps/miniprogram/src/career/progress-preparation.config.ts,apps/miniprogram/src/career/progress-preparation.tsx,apps/miniprogram/src/career/rules-usage-reminders.config.ts,apps/miniprogram/src/career/rules-usage-reminders.tsx: timed out -> failed
- apps/web/src/App.tsx,apps/web/src/DevMarkdownPage.tsx,apps/web/src/NotFoundPage.tsx,apps/web/src/router.tsx,apps/web/src/routes.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/agents/AgentEditorModal.tsx,apps/web/src/agents/AgentParserRules.tsx,apps/web/src/agents/AgentsPage.tsx,apps/web/src/agents/MbtiTestModal.tsx,apps/web/src/agents/PersonaSection.tsx,apps/web/src/agents/SubagentsSection.tsx,apps/web/src/agents/agents-u.css,apps/web/src/agents/agents.css,apps/web/src/agents/agents.td.css,apps/web/src/agents/list.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/appconnector/ActionApproval.tsx,apps/web/src/appconnector/AppsPage.tsx,apps/web/src/appconnector/AuthorizationPage.tsx,apps/web/src/appconnector/ConnectionsPage.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/integrations/ApiPlaygroundDrawer.tsx,apps/web/src/integrations/EmbedPreviewModal.tsx,apps/web/src/integrations/IntegrationsPage.tsx,apps/web/src/integrations/IntegrationsRoutePage.tsx,apps/web/src/integrations/integrations-u.css,apps/web/src/integrations/integrations.td.css: timed out -> failed
- ... and 4 more

Context compaction (5 requests):
- apps/web/src/knowledge-bases/KnowledgeBaseActivityPanel.tsx,apps/web/src/knowledge-bases/KnowledgeBaseShareDialog.tsx,apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx,apps/web/src/knowledge-bases/SharedKnowledgeBaseDrawer.tsx,apps/web/src/knowledge-bases/kb-editor-parity.css,apps/web/src/knowledge-bases/kb-list-icons.tsx,apps/web/src/knowledge-bases/kb-list.td.css,apps/web/src/knowledge-bases/kb-u.css: rejected by provider (HTTP 400) -> failed
- apps/web/src/platform/GlobalCommandPalette.tsx,apps/web/src/platform/InvitationInbox.tsx,apps/web/src/platform/PlatformShell.tsx,apps/web/src/platform/platform-shell.td.css,apps/web/src/platform/platform-u.css,apps/web/src/platform/retrieval-settings-panel.tsx,apps/web/src/platform/session-activity.ts: rejected by provider (HTTP 400) -> failed
- apps/web/src/settings/SystemInfoPanel.tsx,apps/web/src/settings/TenantAuditDrawer.tsx,apps/web/src/settings/TenantDeleteZone.tsx,apps/web/src/settings/TenantMembersPanel.tsx,apps/web/src/settings/TenantUserProfileSections.tsx,apps/web/src/settings/UsagePanel.tsx,apps/web/src/settings/settings-toast.tsx,apps/web/src/settings/settings-wrapper.css,apps/web/src/settings/settings.td.css,apps/web/src/settings/surface.ts: cancelled
- apps/web/src/settings/SystemInfoPanel.tsx,apps/web/src/settings/TenantAuditDrawer.tsx,apps/web/src/settings/TenantDeleteZone.tsx,apps/web/src/settings/TenantMembersPanel.tsx,apps/web/src/settings/TenantUserProfileSections.tsx,apps/web/src/settings/UsagePanel.tsx,apps/web/src/settings/settings-toast.tsx,apps/web/src/settings/settings-wrapper.css,apps/web/src/settings/settings.td.css,apps/web/src/settings/surface.ts: cancelled
- apps/web/src/settings/SystemInfoPanel.tsx,apps/web/src/settings/TenantAuditDrawer.tsx,apps/web/src/settings/TenantDeleteZone.tsx,apps/web/src/settings/TenantMembersPanel.tsx,apps/web/src/settings/TenantUserProfileSections.tsx,apps/web/src/settings/UsagePanel.tsx,apps/web/src/settings/settings-toast.tsx,apps/web/src/settings/settings-wrapper.css,apps/web/src/settings/settings.td.css,apps/web/src/settings/surface.ts: cancelled

Per-attempt detail: --format json (retry_report).
