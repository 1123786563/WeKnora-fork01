Review complete: 511 finding(s) across 333 selected item(s).

─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:77-78 ───
[security · medium] 测试账号与口令硬编码在脚本内，且口令是脱敏占位符 '[REDACTED-disposable]'：(a) 按提交态运行必然登录失败（fillInput
会把占位符当真实密码输入），脚本无法重放出已提交的证据截图，与文件头「机器强相关配置一律 env 注入」约定及门禁证据可重放目标矛盾；(b)
后续维护者若直接回填真实口令，凭据将永久写入仓库。脚本注释自述「对齐 T24 live-driver 写法」，但 T24（t24r1-live-driver.cjs:145-146）实际是经
process.env.T24R1_USER_EMAIL / T24R1_USER_PASS 注入且缺失即 exit(2)，并非硬编码。建议对齐该先例改为 env 注入并 fail-fast。

-     await fillInput(page, '请输入账号邮箱', 't33a@t33.io');
-     await fillInput(page, '请输入密码', '[REDACTED-disposable]');
+ const USER_EMAIL = process.env.T33_USER_EMAIL;
+ const USER_PASS = process.env.T33_USER_PASS;
+ if (!USER_EMAIL || !USER_PASS) { console.error('missing T33_USER_EMAIL / T33_USER_PASS env'); process.exit(2); }
+ // ...
+ await fillInput(page, '请输入账号邮箱', USER_EMAIL);
+ await fillInput(page, '请输入密码', USER_PASS);


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:115-115 ───
[maintainability · low] 断言文案（'毕业时间：2026-06'、'修订 2'、'2026 秋招 A 批'、'前端开发实习'）与账号同为本次 E2E
造数的一次性数据，全部内联在断言正则中；换环境重放（重新造数/文案调整）必须改源码。已核对当前路由拼写与注册一致（app.config.ts root "career" + pages
"discovery"/"application-material"，core/routes.ts career/careerApply），路径漂移会经 relaunch throw / record
FAIL 以非零退出码暴露、不致假阳性，故仅是重放维护成本问题。建议把数据断言值收敛为脚本顶部常量（或 env），与头部声明的注入策略保持一致。

-     record('career-page', /毕业时间：2026-06/.test(careerText) && /修订 2/.test(careerText), 'web-created profile fact 毕业时间=2026-06 · 修订 2 visible in weapp');
+ // 顶部集中声明本次造数的断言锚点，便于换环境重放时统一调整
+ const PROFILE_FACT_TEXT = '毕业时间：2026-06';
+ const PROFILE_REV_TEXT = '修订 2';
+ // ...
+ record('career-page', careerText.includes(PROFILE_FACT_TEXT) && careerText.includes(PROFILE_REV_TEXT), 'web-created profile fact visible in weapp');


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:53-53 ───
[bug · low] tapConsent 的回退分支取页面第一个 checkbox，正确性依赖「登录页唯一 checkbox」这一未验证的隐含假设（当前已核对
features/auth/pages.tsx 登录页确实只有一个 consent checkbox，假设暂成立）。但若登录页后续新增其他勾选项（如「记住账号」），'.wk-consent
checkbox' 未命中时回退分支可能点错控件，使 consent 前置被虚假满足并产生假阳性 PASS，而该函数正是登录前置条件、影响后续全部断言的可信度。建议回退前校验全页 checkbox
唯一性，不唯一即判 FAIL。

-     const box = (await page.$('.wk-consent checkbox')) || (await page.$('checkbox'));
+     const boxes = (await page.$$('.wk-consent checkbox'));
+     if (boxes.length === 1) { try { await boxes[0].tap(); return true; } catch {} }
+     // 回退仅在页面 checkbox 唯一时允许，避免点错控件产生假阳性
+     const all = await page.$$('checkbox');
+     if (all.length === 1) { try { await all[0].tap(); return true; } catch {} }


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:18-18 ───
[bug · low] T33_WS_PORT 注入非数字时 Number() 返回 NaN，会拼出 ws://127.0.0.1:NaN，connectRetry 空转 4
分钟后只抛出难以定位根因的 'connect timeout'。建议读取后立即做整数/端口范围校验并 fail-fast（对齐 T24 缺失 env 即 exit(2)
的做法）。另：loadAutomator/allTexts/tapText 等处的空 catch 属轮询容错（与 T24 一致，可接受），但 connectRetry 的空 catch
会吞掉每次连接失败的具体原因，建议至少记录首个错误便于排查。

  const WS_PORT = Number(process.env.T33_WS_PORT || 9433);
+ if (!Number.isInteger(WS_PORT) || WS_PORT <= 0 || WS_PORT > 65535) {
+   console.error(`invalid T33_WS_PORT: ${process.env.T33_WS_PORT}`); process.exit(2);
+ }


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:107-108 ───
[test · medium] 登录等待循环 30 轮超时后 homeText 保持空串，此处仅 record FAIL 而不 throw，脚本继续执行第 3-5
步；此时小程序无会话，career-page/apply-material-data 等数据断言必然级联 FAIL，把「未登录/未进 home」的根因伪装成数据断言失败——这正是本脚本在
consent 与 enter-workspace 处（ocr3-023 注释：「不把『未进入空间』伪装成后续数据断言 FAIL、掩盖真实根因」）明确采用 fail-fast
要避免的反模式，同类前置却是双重标准。建议对齐：未达 home 即 record + throw。

-     record('login', /求职工作台/.test(homeText), 'reached home with 求职工作台 entry');
+     const loginReached = /求职工作台/.test(homeText);
+     record('login', loginReached, 'reached home with 求职工作台 entry');
+     if (!loginReached) throw new Error('home not reached after login (precondition of all later assertions)');
      await shot(mp, '02-home');


─── .superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs:64-64 ───
[test · low] 截图失败仅 console.log，既不进入 results
数组也不影响退出码。本脚本是终局门禁证据采集脚本（01-login-filled~04-application-material 截图供报告引用），若 T33_SHOTS
目录不可写等原因导致全部截图失败，脚本仍可 RESULT 全 PASS、exit 0，按退出码/RESULT JSON 消费的门禁与复验工具完全无感知证据缺失。建议失败时 record 进
results（使其影响退出码），与文件尾「任一步骤 FAIL 必须非零退出码供门禁消费」的约定保持一致。

- async function shot(mp, name) { try { await mp.screenshot({ path: `${SHOTS}/wx-${name}.png` }); log('shot', name); } catch (e) { log('shot-fail', name, e.message); } }
+ async function shot(mp, name) { try { await mp.screenshot({ path: `${SHOTS}/wx-${name}.png` }); log('shot', name); } catch (e) { record(`shot-${name}`, false, `screenshot failed: ${e.message}`); } }


─── apps/embed/src/button.tsx:1-1 ───
[maintainability · low] 未使用的默认导入：apps/embed 的 tsconfig 为 `"jsx": "react-jsx"`（自动 JSX 运行时），JSX
转换由编译器自动注入 `react/jsx-runtime`，本文件中 `React` 标识符从未被引用（仅 `ButtonHTMLAttributes`/`ReactNode`
两个类型被使用），且这是 apps/embed 下唯一 `import React` 的文件。属死代码，建议改为纯类型导入：`import type { ButtonHTMLAttributes,
ReactNode } from 'react';`

- import React, { type ButtonHTMLAttributes, type ReactNode } from 'react';
+ import type { ButtonHTMLAttributes, ReactNode } from 'react';


─── apps/miniprogram/config/index.ts:48-48 ───
[maintainability · low] TDesign 闭包入口硬编码为 'button/button'：当前 6 个页面配置（5 个 career 页 + artifact 页）确实都只引用
button/button，暂无现行缺陷；但闭包只能自动跟进 button 自身的传递依赖，一旦任何页面在 usingComponents 引入其它 TDesign 组件（如
t-dialog、t-icon），构建不会报错、产物也不会拷入该组件，真机将直接复现注释中描述的 "has not been registered yet" 白屏，且无任何构建期防线。注释中
"TDesign 升级新增依赖自动跟进" 的表述容易让人误以为新增组件同样自动跟进。建议在构建期扫描 src/**/*.config.ts 中所有以 /npm/tdesign/ 开头的
usingComponents 引用，自动推导闭包入口集合（顺带消除 '/npm/tdesign' 前缀在构建配置与页面配置间的双重维护）。

- const tdesignDirs=['common','miniprogram_npm',...new Set([...tdesignClosure('button/button')].map(stem=>stem.split('/')[0]))];
+ const tdesignEntries = collectTdesignEntries('src'); // 扫描 src/**/*.config.ts 的 usingComponents，收集 /npm/tdesign/<entry> 引用
+ const tdesignDirs=['common','miniprogram_npm',...new Set(tdesignEntries.flatMap(entry=>[...tdesignClosure(entry)]).map(stem=>stem.split('/')[0]))];


─── apps/miniprogram/config/index.ts:19-20 ───
[bug · low] realpathSync 在 existsSync 友好检查之前执行：最常见 failure 场景（依赖未安装，node_modules/tdesign-miniprogram
不存在）会由 realpathSync 直接抛出原始 ENOENT，精心准备的 "run pnpm install first" 提示在该场景下不可达，只有包存在但缺 button/button.js
时才会触发。建议先对原始路径做 existsSync 检查，再 realpath。

- const tdesignDist=realpathSync(resolve(process.cwd(),'node_modules/tdesign-miniprogram/miniprogram_dist'));
- if(!existsSync(resolve(tdesignDist,'button/button.js')))throw new Error(`tdesign-miniprogram miniprogram_dist not found at ${tdesignDist} — run pnpm install first`);
+ const tdesignRaw=resolve(process.cwd(),'node_modules/tdesign-miniprogram/miniprogram_dist');
+ if(!existsSync(tdesignRaw))throw new Error(`tdesign-miniprogram miniprogram_dist not found at ${tdesignRaw} — run pnpm install first`);
+ const tdesignDist=realpathSync(tdesignRaw);
+ if(!existsSync(resolve(tdesignDist,'button/button.js')))throw new Error(`tdesign closure: missing ${resolve(tdesignDist,'button/button.js')}`);


─── apps/miniprogram/src/adapters/career-platform.ts:260-262 ───
[performance · medium] 性能:MAX_EXPORT_BYTES 放行的上限是 20MB,而这里用同步 readFileSync 全量读入 + 纯 JS sha256(约 32.8
万分组 × 64 轮压缩)在单次调用内算完,小程序 JS 线程会被完全阻塞——低端真机上导出打开主流程可能出现数秒级界面冻结;saveSpaceExportPackage 更重(writeFile
落盘后再次同步全量回读并完整哈希一遍,等于同一载荷两遍全量计算)。另 copySpaceExportToClipboard 把完整 JSON 载荷交给
Taro.setClipboardData,真机剪贴板有实际容量上限(测试替身来者不拒,会掩盖该失败),全空间归档较大时「等效可取得流程」第二步可能必然失败。建议:把 sha256Hex
改造为可增量喂入的实现,按 1MB 左右分块读取、块间让出事件循环;剪贴板路径按载荷大小设阈值,超限时如实提示只走本机文件留存,不承诺复制成功。



─── apps/miniprogram/src/adapters/career-platform.ts:173-177 ───
[bug · medium] 凭据路径偏离冻结纪律：runtime.ts 明确规定直连二进制下载通道取 token 的唯一入口是 currentBearerToken()（先走授权 GET 触发
refresh-once 并把轮换落盘，"不得绕行 auth.credential() 现读"），被声称"同源"的 T06 platform/files.ts 也是这么做的（files.ts:57
`await currentBearerToken()` 后再复验作用域）。本函数直接现读 auth.credential()：存量 access token 过期/临界过期时（如
signed-url 签发与兑付下载之间、或 EXPORT_GRANT_EXPIRED 重试链路上），兑付下载会带过期 Bearer 发出，401 只能得到通用"下载失败"，没有
refresh-once 兜底。建议对齐 files.ts：scope 捕获后先 `await currentBearerToken()` 铸造新 token、复验 isCurrent
再进入下载（deps.bearer 注入可保留给测试）。

- function defaultExportParts(): ExportPlatformParts {
-   // 运行时认证凭据与可信 origin 与 T06 files.ts 同源（services/runtime 单例）。
-   const credential = auth.credential();
-   if (credential.kind !== 'bearer') throw new Error('AUTH_REQUIRED');
-   return {
+   const stamp = auth.scope.capture();
+   if (!auth.scope.isCurrent(stamp)) throw new Error('SCOPE_CHANGED');
+   // 对齐 T06 files.ts：直连下载的 token 经授权通道 refresh-once 铸造后再现读
+   const accessToken = await currentBearerToken();
+   if (!auth.scope.isCurrent(stamp)) throw new Error('SCOPE_CHANGED');
+   // …下载 header 使用该 accessToken


─── apps/miniprogram/src/adapters/career-platform.ts:357-359 ───
[bug · low] 草稿键的归一化不对称：applicationId 入键前 trim，focus 却原样拼接。savePreparationDraft 校验的是
record.focus.trim() 非空，但键与存储都保留原始 focus——调用方一旦在两处传入带空白的 focus（如 ' 面试 ' vs '面试'），save/read/clear
会命中不同键，断网草稿"凭空消失"且 clear 漏清。建议 focus 同样 trim 后入键（并存入归一化后的 record）。

  function preparationDraftKey(applicationId: string, focus: string): string {
-   return `${PREPARATION_DRAFT_PREFIX}${scopeKey(auth.scope.capture())}:${applicationId.trim()}:${focus}`;
+   return `${PREPARATION_DRAFT_PREFIX}${scopeKey(auth.scope.capture())}:${applicationId.trim()}:${focus.trim()}`;
  }


─── apps/miniprogram/src/career/application-material.tsx:108-110 ───
[bug · low] 对账成功后的回读/列表加载也在同一 run 内，但错误附注固定写死「对账被拒时说明该请求不属于当前空间或不存在；intent 保留，可稍后再试」——若 reconcile
已成功而 loadSubmissions 读失败，intent 实际已被清除，提示失实。recoverMatBusy（成功后 material() 回读 + listMaterialExports）与
recoverPubBusy（成功后 loadExports）同型问题。另 `void receipt;` 是死语句，可直接不绑定返回值。建议把成功后的回读放进独立 try/catch（同页
retryMatBusy 已有「回读失败不掩埋重试成功的事实」先例）。

-         const receipt = await career.reconcilePendingSubmission();
-         if (application) await loadSubmissions(application.applicationId);
-         void receipt;
+         await career.reconcilePendingSubmission();
+         if (application) { try { await loadSubmissions(application.applicationId); } catch { /* 列表读取失败不掩埋对账成功的事实 */ } }


─── apps/miniprogram/src/career/application-material.tsx:284-287 ───
[maintainability · low] 单个 downloadBusy 被所有格式的下载按钮共享：任一下载进行中，所有按钮同时转 loading 且未随 busy
禁用，可并发发起多次下载；checks 记录只增不减，长会话无上限累积。另外 listBusy 同时服务「刷新导出列表」与「读取投递记录」两个语义，会互相误显 loading。建议按
exportId+format 维护进行中标记，busy 时禁用其余按钮，并考虑限制 checks 展示条数。



─── apps/miniprogram/src/career/application-material.tsx:165-165 ───
[style · low] appErrCode 的三层链式三元违反「禁止嵌套三元」约定。本页已有 evaluationLabel/evaluationTone 映射表先例，建议同样提取为
Record 映射；linkState 的 tone 判定（`'ready' ? 'success' : 'linking' ? 'warning' : 'danger'`）同理。

-       {applyBusy.error && <Notice tone='danger'>{applyBusy.error}{appErrCode === 'revision_conflict' ? ' 档案已更新：下拉刷新读取最新修订后重试（新提交将使用新请求编号）。' : appErrCode === 'application_conflict' ? ' 此岗位与该批次已存在申请：一个岗位和招聘批次只有一个申请与 Task；可为其他批次创建。' : appErrCode === 'hard_ineligible_requires_continue' ? ' 硬性条件不符，需要先勾选显式继续才能提交申请。' : ''}</Notice>}
+ const applyErrorHints: Record<string, string> = {
+   revision_conflict: ' 档案已更新：下拉刷新读取最新修订后重试（新提交将使用新的请求编号）。',
+   application_conflict: ' 此岗位与该批次已存在申请：一个岗位和招聘批次只有一个申请与 Task；可为其他批次创建。',
+   hard_ineligible_requires_continue: ' 硬性条件不符，需要先勾选显式继续才能提交申请。',
+ };
+ // {applyBusy.error && <Notice tone='danger'>{applyBusy.error}{appErrCode ? applyErrorHints[appErrCode] ?? '' : ''}</Notice>}


─── apps/miniprogram/src/career/application-material.tsx:23-25 ───
[maintainability · low] tdesignButtonStyle / digestHead / typedCode 在 career 页面家族已分别重复 5/3/4 份（本 PR
又新增两份）。长 CSS 变量串一旦主题变量改名（如 --wk-color-action-primary）需要逐页同步，极易漂移。建议提取到共享模块（如 src/career/shared.ts 或
components/ui.tsx）统一导出。



─── apps/miniprogram/src/career/application-material.tsx:196-198 ───
[bug · high] 「读取材料」存在在途竞态，可把材料 A 的正文写进材料 B 名下：run 完成后无条件 setMaterial/setSections/setExports，而
185-195 行 Field onChange 的失效逻辑只处理「变更之后」的新读取，挡不住「读取在途时改编号」。时序：点读取（A，两次串行 GET，弱网窗口数秒）→ 在途时把编号改为
B（onChange 已清空 material/sections/exports）→ A 的响应返回后重新落地 A 的正文与编辑态，输入框却显示 B → 此时点「保存草稿」，226 行
`material ? editMaterial({ materialId, body })` 会用输入框 B + A 的 sections 提交——正是本页 187-189 行注释（OCR
high-11「绝不把 A 的 sections 写进 B 名下」）声称防御的场景。对比同页 recoverMatBusy 用回执的权威 materialId
规避了此问题，progress-preparation 的 loadTimeline 也有完成后守卫（`next.applicationId !== applicationId.trim()`
抛错），此处遗漏。建议用 ref 跟踪最新输入，run 完成时校验 view.materialId 与之一致，不符则丢弃响应。

        <Action secondary loading={matLoadBusy.busy} onClick={() => void matLoadBusy.run(async () => {
          const view = await career.material(materialId);
+         if (view.materialId !== materialIdRef.current.trim()) return; // 输入已改：丢弃过期响应，绝不把 A 的正文落进 B 名下
          setMaterial(view);


─── apps/miniprogram/src/career/application-material.tsx:234-238 ───
[bug · medium] 「确认为不可变新版本」成功后的回读 `setMaterial(await career.material(materialId))` 与写同在一个
run：confirmMaterial 已成功（不可变版本已铸、confirmedVersion 已设置）而回读失败（瞬时 request:fail）时，matConfirmBusy.error
会把已成功的确认误报为失败，且「已确认版本 V2」与错误 Notice 并存、material
未刷新，可能诱导用户再次点「确认」——服务端合同是每次确认新增一个不可变版本，会产生多余版本。223-230 行「保存草稿」（matEditBusy）的 229
行回读同型（保存成功被误报失败，诱导重复保存草稿）。这是与已确认发现（recover 系列按钮、progress saveRevision）同型但独立位置的新实例；同页 retryMatBusy
已有「回读失败不掩埋重试成功的事实」的隔离先例，此处应采用同样写法。

-         <t-button block size='large' theme='default' ariaLabel='确认材料新版本' customStyle={tdesignButtonStyle} loading={matConfirmBusy.busy} disabled={pendingMaterial !== null} onTap={() => void matConfirmBusy.run(async () => {
            const receipt = await career.confirmMaterial(materialId);
            setConfirmedVersion(receipt.version);
-           setMaterial(await career.material(materialId));
-         })}>确认为不可变新版本</t-button>
+           try { setMaterial(await career.material(materialId)); } catch { /* 回读失败不掩埋确认成功的事实（同 retryMatBusy 先例） */ }


─── apps/miniprogram/src/career/discovery.config.ts:8-8 ───
[maintainability · low] 构建产物绝对路径 '/npm/tdesign/button/button' 在全部 5 个 career 页面
config（discovery/application-material/export-deletion/progress-preparation/rules-usage-reminders）中重复
硬编码，与 config/index.ts 的拷贝目标 dist/npm/tdesign 强耦合：拷贝目标或目录结构变更时需同步 5 处，且漏改只会在运行期以组件缺失暴露。建议抽取共享常量（如
src/career/tdesign-components.ts 导出 TDESIGN_BUTTON_PATH），各页面 config 统一 import 引用，收敛为单一事实源。



─── apps/miniprogram/src/career/discovery.tsx:37-37 ───
[bug · medium] 分享 path 无长度上限防护：JD 全文经 encodeURIComponent 后中文每字约膨胀 9 字节（适配器注释自证 71 字 JD 编码后 400
字），而接收侧 readSharedEntry/prepareSharedImport 均无长度校验。较长 JD（数百字很常见）会使分享卡片 path 超出微信可接受长度，导致分享失败或 jd
参数被截断；被截断的 percent-encoding 文本在 decodeEntryPayload 解码失败时会原样保留，接收方核对预览里呈现的是乱码残文，仍可"确认导入"落成坏数据。config
注释已承认分享导入链路此前曾整体不可达，属敏感链路。建议：发送侧对 rawText 超限（如 >800
字）时改用引导粘贴的回落策略或仅携带截断摘要并在接收侧明确提示不完整；接收侧对含解码残缺/超长文本给出显式警告而非直接进入核对态。



─── apps/miniprogram/src/career/discovery.tsx:134-134 ───
[bug · medium] receiptMissing 状态残留：对账 404 置 receiptMissing=true 后，点"放弃本次搜索恢复"清掉了 pendingSearch
intent，但 receiptMissing 未复位，且下方 `{receiptMissing && <>…}` 区块未按 pendingSearch
门控——"尚未找到该回执"提示与"安全重发原搜索"按钮在已无恢复意图的情况下继续渲染。此时点击重发，career.retryPendingSearch()
会直接抛出"没有待恢复的搜索"（services/career.ts:166），形成必失败的误导性恢复入口，与"显式放弃即解除封锁"的语义矛盾。建议 abandon 时同步
setReceiptMissing(false)，并将该区块门控在 pendingSearch 上：

-       {pendingSearch && <Action secondary onClick={() => { career.abandonPendingSearch(); setImportNotice('已放弃本次搜索恢复：可重新发起搜索。若原搜索实际已生效，额度以服务端记录为准。'); }}>放弃本次搜索恢复</Action>}
+       {pendingSearch && <Action secondary onClick={() => { career.abandonPendingSearch(); setReceiptMissing(false); setImportNotice('已放弃本次搜索恢复：可重新发起搜索。若原搜索实际已生效，额度以服务端记录为准。'); }}>放弃本次搜索恢复</Action>}
+       {receiptMissing && pendingSearch && <>


─── apps/miniprogram/src/career/discovery.tsx:97-97 ───
[bug · low] 简历安全重发前缺少文件一致性校验：pendingUpload() 的 intent 已持久化
input.fileName（career.ts:192），但重发路径直接把用户本次新选的文件以原 requestId
上传。用户重选不同文件时只能依赖服务端幂等指纹拒绝（且已白白消耗一次完整文件上传流量），交互上仅靠文案提醒。chooseResume 返回的 NativeFileSource 携带
name/size，建议重发前先与 intent 记录比对，不一致时直接提示"请选择与原上传相同的文件"并中止，避免把校验责任全部推给服务端。

+             const pending = career.pendingUpload();
+             if (pending?.input?.fileName && pending.input.fileName !== file.name) throw Object.assign(new Error('重发需选择与原上传相同的文件（原文件：' + pending.input.fileName + '），否则请先放弃本次恢复再重新上传。'), { code: 'resend_file_mismatch', recoverable: true });
              const upload = await career.retryPendingUpload(file);


─── apps/miniprogram/src/career/discovery.tsx:44-44 ───
[maintainability · low] quotaRefused 以 message.includes('搜索额度不足') 判定降级语气，与 core/errors.ts 第 8
行的专属文案强耦合：文案措辞一旦调整，此判定静默退化为 danger 失败提示，破坏"额度不足仍可读"的保证且无任何告警。错误对象本身携带 typed
code（search_quota_refused），建议改为基于 code 判定——例如在 core/errors.ts 导出 isQuotaRefusedError(e)（比对 e.code
=== 'search_quota_refused'），并让 useAction 在存文案的同时保留原始 error/code 供本判定使用。



─── apps/miniprogram/src/career/discovery.tsx:69-69 ───
[maintainability · low] 此处 <t-button> 依赖的全局 JSX.IntrinsicElements 声明位于
src/subpackages/execution/artifact/t-button.d.ts（declare global，当前全局生效，编译无碍）。但 career
页面对该声明是跨包隐式依赖：后续若 execution 子包重构或该 d.ts 被移除，career 页面会静默失去类型检查（Taro 构建经 babel 不做类型检查，不会报错）。建议将该声明移至
src 顶层全局类型文件（如 src/types/t-button.d.ts），使声明位置与其服务范围（全部使用 t-button 的页面）一致。



─── apps/miniprogram/src/career/discovery.tsx:53-53 ───
[bug · high] 档案动作恢复链不完整且失败完全静默：(1) recoverBusy 同时承担"用原请求对账恢复"与"同步 Web 端档案变更"两个动作，但全页没有任何
{recoverBusy.error && <Notice…>} 渲染——careerDesk().reconcile 在服务端回执 404（not_found，desk.ts:101
重抛）或网络失败时，用户点击后按钮 loading 结束却零反馈，恢复入口形同虚设；(2) 若原 act 请求从未送达服务端，回执将永久 404，而 desk.unresolved 未清除前
propose/confirm/dismiss 全部被 mutate() 以 unresolved_action 拒绝（desk.ts:104），且仅换空间或冷启动才解除。服务层已暴露安全重发出口
career.retryPending（career.ts:68，reconcile 404 后同 requestId
重放），但页面未接线，也没有像同页搜索域（安全重发+放弃）/上传域（安全重发+放弃）那样的兜底出口。建议：渲染 recoverBusy.error；对账
404（career.isReceiptMissing 同款判据）时提供 retryPending(pendingFactAction) 安全重发与显式放弃入口。

      {pendingFactAction && <Action secondary loading={recoverBusy.busy} onClick={() => void recoverBusy.run(async () => { await career.reconcilePending(); query.reload(); })}>用原请求对账恢复</Action>}
+     {recoverBusy.error && <Notice tone='danger'>{recoverBusy.error}</Notice>}
+     {pendingFactAction && <Action secondary loading={resendBusy.busy} onClick={() => void resendBusy.run(async () => { await career.retryPending(pendingFactAction); query.reload(); })}>安全重发原档案操作</Action>}


─── apps/miniprogram/src/career/discovery.tsx:107-109 ───
[bug · low] 逐项建档提交缺少空值守卫：按钮仅 disabled={proposeBusy.busy}，factValue
为空/纯空白时仍可点击；career.proposeFact（services/career.ts:53）对空值也不校验，空值会直达冻结合同 POST /career/act（服务端 400
或产生空值 proposal）。同页粘贴导入按钮已有 disabled={!pasteText.trim()}、searchOnce 内部也有空查询守卫，此处应对齐。

-         <Action secondary disabled={proposeBusy.busy} loading={proposeBusy.busy} onClick={() => void proposeBusy.run(async () => {
+         <Action secondary disabled={proposeBusy.busy || !factValue.trim()} loading={proposeBusy.busy} onClick={() => void proposeBusy.run(async () => {
            await career.proposeFact(factKey, factValue.trim()); setFactValue(''); query.reload();
          })}>添加为待确认事实</Action>


─── apps/miniprogram/src/career/discovery.tsx:82-85 ───
[bug · low] 用户取消选档被当成失败呈现：chooseDocument（platform/files.ts:47）在取消时以 chooseMessageFile:fail cancel
的普通对象 reject，该对象无 code/status，经 useAction→errorMessage() 落到兜底文案"操作未完成，请检查网络或刷新状态后重试。"并以 danger
Notice 展示——常规取消被报成网络故障（下方"安全重发"路径同样受影响，且会拼接"重发沿用原请求编号…"的误导文案）。建议在 catch 中识别 errMsg 含 cancel 时静默返回。

              const stamp = auth.scope.capture();
-             const file = await careerPlatform.chooseResume();
+             let file;
+             try { file = await careerPlatform.chooseResume(); }
+             catch (error) {
+               if (/cancel/i.test(String((error as { errMsg?: string })?.errMsg ?? ''))) return; // 用户取消不是错误
+               throw error;
+             }
              if (!auth.scope.isCurrent(stamp)) throw new Error('SCOPE_CHANGED');
              const upload = await career.uploadResume(file);


─── apps/miniprogram/src/career/export-deletion.tsx:316-318 ───
[bug · medium] 删除主按钮 onTap 中 gating.deletionDisabled 只在 confirmAction 之前检查（且是渲染闭包快照），await
弹窗返回后未复验。Taro 事件桥接下，"发起导出"后 `setBusy(true)` 的重渲染/数据同步尚未提交时快速连点"发起完整删除"，会以陈旧闭包（exportBusy.busy 仍为
false）通过检查；原生 showModal 期间虽不可再交互，但用户确认弹窗时若该导出仍在途或已落为结果未知
intent，删除将照常发出——违反本文件自述不变量"导出未决联动封锁删除（删除会吊销导出授权，导出结果必须先落定）"。已核实服务层
career.deleteWholeSpace（career.ts:608-615）只守卫 pending 删除（unresolved_action），不检查 pending
导出，页面层检查是唯一防线。建议 confirm 后用最新状态复验，更稳妥是在 career.deleteWholeSpace 补 pending 导出守卫。

-               if (gating.deletionDisabled) return; // 防重入（Web runDeletion busy/unknown 守卫）
                const confirmed = await confirmAction('确认完整删除？', '空间内求职数据将被删除且不可恢复；外部平台的投递与已发出的副本不受本系统控制。');
                if (!confirmed) return;
+               // 弹窗异步期间状态可能已变化：用最新读取复验导出未决/进行中（服务层只守卫 pending 删除）。
+               if (career.pendingSpaceExport() !== null || exportBusy.busy || recExportBusy.busy || retryExportBusy.busy) {
+                 setDelErrCode('unresolved_action');
+                 return;
+               }


─── apps/miniprogram/src/career/export-deletion.tsx:85-89 ───
[bug · medium] 恢复成功后旧失败提示残留：useAction 的 error 保留到该 action 下一次 run（ui.tsx:56），而恢复流程走的是
recExportBusy/retryExportBusy 等不同 action 实例，acceptExport 又未清 expErrCode（也无法清
exportBusy.error）。发起导出失败（如 outcome_unknown）后经"用原请求对账导出"成功，页面仍显示旧的 exportBusy.error danger 提示和
expErrCode==='revision_conflict' 的"重新读取档案修订"按钮，误导用户当前状态仍失败。Web 端 acceptExport 会
setExportPhase('idle') 并覆盖 exportMessage（ExportDeletionPage.tsx:123-128），两端语义漂移。建议 acceptExport 清
expErrCode，并让发起错误提示在已有更新回执时不再渲染（或为 useAction 增加 reset 能力）。

    const acceptExport = (receipt: CareerExportReceipt): void => {
      setExported(receipt);
      setSaved(undefined);
      setCopyNotice('');
+     setExpErrCode(undefined); // 恢复成功落定后，旧发起失败提示不再代表当前状态
    };


─── apps/miniprogram/src/career/export-deletion.tsx:114-120 ───
[bug · medium] 同 acceptExport：acceptDeletion 未清 delErrCode（也无法清 deletionBusy.error）。发起删除以
outcome_unknown 失败后经对账/重试拿到确定回执，页面仍显示旧的 deletionBusy.error danger 提示及
delErrCode==='revision_conflict' 的"重新读取档案修订"按钮（与已恢复的回执卡片并存，状态自相矛盾）。对账失败后再重试成功的场景同理残留
recDelBusy/retryDelBusy.error。Web 端 acceptDeletion 会覆盖 deletionMessage 并按 phase
控制错误呈现（ExportDeletionPage.tsx:241-258、383-384）。建议此处清 delErrCode，并让发起/恢复错误提示在已有更新回执时不再渲染。

    const acceptDeletion = (receipt: CareerDeletionReceipt): void => {
      deletionController.accept(receipt.status);
      setDeletion(receipt);
      setAcknowledged(false);
+     setDelErrCode(undefined); // 确定回执落定后，旧发起失败提示不再代表当前状态
      setDelRecoveryUnresolved(false); // 确定回执落定上次恢复尝试的未决（Web acceptDeletion）
      if (receipt.status === 'deleted') void finalizeAfterDeletion(receipt);
    };


─── apps/miniprogram/src/career/export-deletion.tsx:96-98 ───
[bug · low] finalizeAfterDeletion 以 void 调用，但 try/catch 仅覆盖 spaceExportReceipt 段：clearCareerCaches
若同步抛错（storage.remove 失败等）会成为未处理
rejection，缓存清理呈报（clearedKeys/verifyNotice）静默中断，用户看不到任何失败提示——与函数注释"两者都如实呈现结果；读得到旧回执是验证失败，绝不静默通过"的承诺不符
。desk.reload（ui.tsx:46）内部已兜底错误，无需 await；建议把 clearCareerCaches 也纳入错误处理，失败时至少呈报。

-     const cleared = career.clearCareerCaches();
+     let cleared: string[];
+     try {
+       cleared = career.clearCareerCaches();
-     setClearedKeys(cleared);
+       setClearedKeys(cleared);
+     } catch (error) {
+       setVerifyNotice(`本机缓存清理失败：${(error as Error).message}。请退出本页重新进入核对删除结果。`);
+       return;
+     }
      desk.reload();


─── apps/miniprogram/src/career/export-deletion.tsx:159-165 ───
[style · low] 三层嵌套三元表达式（本处与 abandonDeletionRecovery 的同款映射、deletionBusy.error Notice 中 delErrCode
的三段映射均同）违反检查清单"禁止嵌套三元"，且新增 result/错误码分支时易漏改。文件内已有 stepNameLabels 等查表先例，建议统一改为 Record 查表。

-       setRecoveryNotice(result === 'abandoned'
-         ? '已放弃导出恢复：仅清除了本机恢复记录；原导出请求可能已在服务端生效。'
-         : result === 'cancelled'
-           ? '未放弃导出恢复：已取消确认，本机恢复记录仍保留。'
-           : result === 'busy'
-             ? '未放弃导出恢复：导出或恢复操作正在进行，本机恢复记录仍保留。'
-             : '未放弃导出恢复：当前请求编号已变化，本机恢复记录未清除。');
+       const exportAbandonNotices: Record<'abandoned' | 'cancelled' | 'busy' | 'changed', string> = {
+         abandoned: '已放弃导出恢复：仅清除了本机恢复记录；原导出请求可能已在服务端生效。',
+         cancelled: '未放弃导出恢复：已取消确认，本机恢复记录仍保留。',
+         busy: '未放弃导出恢复：导出或恢复操作正在进行，本机恢复记录仍保留。',
+         changed: '未放弃导出恢复：当前请求编号已变化，本机恢复记录未清除。',
+       };
+       setRecoveryNotice(exportAbandonNotices[result]);


─── apps/miniprogram/src/career/export-deletion.tsx:124-128 ───
[maintainability · low] reconcileDeletion/retryDeletion 两两结构完全相同（仅 service 调用与 busy 实例不同）；导出恢复区的两个内联
on⼾ap handler、abandonExportRecovery/abandonDeletionRecovery 亦成对近似复制。双份维护易导致语义漂移（本轮 F1 修复就需同时改两处
catch）。建议抽公共包装，如 runDeletionRecovery(busy, invoke) 与 runExportRecovery(busy, invoke)，abandon 流程同理参数化
service 调用与文案表。

-   const reconcileDeletion = (): void => {
+   const runDeletionRecovery = (busy: typeof recDelBusy, invoke: () => Promise<CareerDeletionReceipt>): void => {
      if (deletionOperationInFlight.current) return;
      deletionOperationInFlight.current = true;
-     void recDelBusy.run(async () => {
-       try { acceptDeletion(await career.reconcilePendingSpaceDeletion()); }
+     void busy.run(async () => {
+       try { acceptDeletion(await invoke()); }
+       catch (error) { deletionController.recoveryFailed(busy === recDelBusy ? 'reconcile' : 'retry', error, career.pendingSpaceDeletion() !== null); setDelRecoveryUnresolved(deletionController.isRecoveryUnresolved()); throw error; }
+       finally { deletionOperationInFlight.current = false; }
+     });
+   };
+   const reconcileDeletion = (): void => runDeletionRecovery(recDelBusy, () => career.reconcilePendingSpaceDeletion());
+   const retryDeletion = (): void => runDeletionRecovery(retryDelBusy, () => career.retryPendingSpaceDeletion());


─── apps/miniprogram/src/career/export-deletion.tsx:351-354 ───
[maintainability · low] partial 回执已呈报时（deletionController.accept('partial') 后
gating.deletionUnknown=false），恢复区（`pendingDeletion && !deleted`）与这里的 partial
回执卡片会同时渲染两个功能完全相同的"用原请求编号重试删除"按钮（均绑定 retryDeletion/retryDelBusy，loading 同步闪烁）。Web 基准在
partial（deletionPhase='error'）时恢复区不显示重试按钮（ExportDeletionPage.tsx:378 条件仅
unknown/deleting），只保留回执卡片入口（:382）。同页双入口让"恢复区=未决/进行中、卡片=已呈报回执"的职责划分模糊，用户易重复点击。建议 partial 已呈报且无
recoveryUnresolved 时恢复区隐藏重试按钮（或去掉卡片这一处），保持单一入口对齐 Web 渲染语义。



─── apps/miniprogram/src/career/export-deletion.tsx:159-160 ───
[bug · low] recoveryNotice 一经设置永不清除（只有被下一条 recoveryNotice
覆盖）。实际混淆场景：放弃导出恢复成功（显示"已放弃导出恢复：仅清除了本机恢复记录…"）→ 用户再次发起导出又以 outcome_unknown 失败 →
恢复区重新出现"有一次结果未知的导出（xxx）…"警告，但其下方仍并列残留上一次的"已放弃导出恢复…"旧提示——用户会误以为"已放弃"针对的是当前这次未知导出（实际是上一次的处置记录）；对账/重试落定
（acceptExport/acceptDeletion）后旧提示同样残留。与已确认的 expErrCode/delErrCode 残留（发现 2/3）同族但属不同 state，且不在
useAction 生命周期内。建议在新的恢复 intent 出现（pendingExport/pendingDeletion 由 null→非 null）或发起新导出/删除时清空
recoveryNotice。



─── apps/miniprogram/src/career/export-deletion.tsx:54-55 ───
[maintainability · low] 该 useState 丢弃值只保留 setter，且经逐路径核对在渲染触发上完全冗余：每处调用都伴随必然触发重渲染的 state
变化——acceptDeletion 的 setDeletion(receipt)（每次 API 返回都是新对象引用）、删除/恢复 catch 的 useAction busy true→false
翻转（ui.tsx:56）、abandon 流程的 confirmationBusy true→false 翻转。而 gating 的真正数据源是 deletionController（useRef
闭包，页面外可变状态），此 useState 与 controller 形成"第二事实源"，读者会误以为它是 deletionUnknown 的输入（React
state）而实际从未被读取。建议删除该 state（重渲染已由伴随更新保证），或反向把 unresolved 落为唯一 React state 并让 controller 从它派生，消除双源。



─── apps/miniprogram/src/career/progress-preparation.tsx:144-146 ───
[bug · medium] 编辑写入成功后的回读 GET 仍在写事务的 try 内：若 editMaterial 已成功而回读失败（如瞬时 request:fail），catch
会把已成功的修订误报为失败——looksOffline 分支会设置「网络不可用：已保留本地草稿（可继续编辑，未提交）」，与刚 setGenNotice 的「修订已提交」相互矛盾，且此时本地草稿已被
clearPreparationDraft 清除，提示完全失实，可能诱导用户重复保存修订。同页 retryMatBusy 的回读已单独
try/catch（「回读失败不掩埋重试成功的事实」），此处应采用同样的隔离。

        setGenNotice('准备草稿修订已提交（仍是可审阅草稿，发布需另行确认材料版本）。');
        // 与 Web 同语义：回执只是回声，修订的持久事实从材料域回读。
-       setRevisedBody((await career.material(materialId)).body);
+       try { setRevisedBody((await career.material(materialId)).body); } catch { /* 回读失败不掩埋修订已提交的事实（与 retryMatBusy 同语义） */ }


─── apps/miniprogram/src/career/progress-preparation.tsx:137-139 ───
[bug · medium] recoverEmptyClaims / attachClaimsFromServer 均只按 trim 后同名 heading
匹配服务端主张，无重命名兜底：旧格式本地草稿（无 claims 快照，正是 R1-F1 要兜底的场景）中被用户改过标题的小节将匹配不到服务端同名节，claims
保持空即整体提交；服务端材料编辑是整体替换正文（material.go
DraftBody），原小节已建立的主张会被静默丢弃——与本页「绝不静默丢弃主张」的声明相悖。建议在匹配后检测「服务端存在带主张但未找到同名小节」的情况，通过 draftNotice
如实警示用户哪些节的主张未能回填，而不是静默通过。

      if (body.sections.some(section => (section.claims ?? []).length === 0)) {
-       body = { sections: recoverEmptyClaims(body.sections, (await career.material(materialId)).body.sections) };
+       const serverView = await career.material(materialId);
+       body = { sections: recoverEmptyClaims(body.sections, serverView.body.sections) };
+       const unmatched = serverView.body.sections.filter(server => server.claims.length > 0
+         && !body.sections.some(section => section.heading.trim() === server.heading.trim()));
+       if (unmatched.length) setDraftNotice(`注意：以下小节未找到同名对应，其原有主张不会保留：${unmatched.map(section => section.heading).join('、')}`);
      }


─── apps/miniprogram/src/career/progress-preparation.tsx:280-280 ───
[style · low] genErrCode 的链式三元（reviseErrCode、viewErrCode、appendErrCode/correctErrCode
合并判定处同型）违反「禁止嵌套三元」约定。建议提取 code→文案的 Record 映射（与 application-material.tsx 的 evaluationLabel
同风格，两页可共用）。

-       {genBusy.error && <Notice tone='danger'>{genBusy.error}{genErrCode === 'revision_conflict' ? ' 档案已更新：重新读取修订后再生成（新生成会使用新的请求编号）。' : genErrCode === 'preparation_generation_failed' ? ' 生成失败但请求已保留：可用原请求编号恢复重试，不会留下空白成功产物。' : ''}</Notice>}
+ const genErrorHints: Record<string, string> = {
+   revision_conflict: ' 档案已更新：重新读取修订后再生成（新生成会使用新的请求编号）。',
+   preparation_generation_failed: ' 生成失败但请求已保留：可用原请求编号恢复重试，不会留下空白成功产物。',
+ };
+ // {genBusy.error && <Notice tone='danger'>{genBusy.error}{genErrCode ? genErrorHints[genErrCode] ?? '' : ''}</Notice>}


─── apps/miniprogram/src/career/progress-preparation.tsx:208-208 ───
[bug · medium] 申请编号改动后 view 与 correcting 不失效，expectedRevision（CAS 域=每申请事件计数）会跨申请错配：读取申请 A 的时间线后把编号改为
B（view 仍是 A 的），录入/纠错时 246-247 行用 `applicationId.trim()`（B）作请求路径、却用 `view.revision`（A 的事件计数）作
expectedRevision。若 B 的事件计数恰好等于 A 的 revision，事件会静默写入 B，而用户看到并以为在补录 A 的时间线（数据污染）；不等时返回误导性的
revision_conflict/progress_event_not_found。纠错场景 correcting.eventId 也是 A 的事件。loadTimeline
的完成后守卫只覆盖「读取在途改输入」，未覆盖「读取完成后改输入」。建议 onChange 时同步失效（与 application-material.tsx 材料编号 onChange
的失效模式一致）。

-       <Field label='申请编号' value={applicationId} onChange={setApplicationId} placeholder='申请编号（可从「申请与材料」页带入）' />
+       <Field label='申请编号' value={applicationId} onChange={value => { setApplicationId(value); setView(undefined); setCorrecting(undefined); setPreparations(undefined); }} placeholder='申请编号（可从「申请与材料」页带入）' />


─── apps/miniprogram/src/career/progress-preparation.tsx:137-139 ───
[bug · low] claims 取回的 `await career.material(materialId)`（137-139 行）在 140 行 try/catch 之外：断网时该 GET
先于 editMaterial 失败，抛出的 transport 错误绕过 catch 的 looksOffline 分支——尽管 133 行 savePreparationDraft
已保存本地草稿，「网络不可用：已保留本地草稿（可继续编辑，未提交）……」的断网语义提示不会出现，reviseErrCode 也不被设置，与 132
行注释声明的断网语义不符（用户只看到原始网络错误，不知道草稿已保留）。测试 C1 只覆盖 editMaterial POST 失败路径，未覆盖此 GET 失败路径。建议把 claims 取回纳入
try（或单独 catch 走同一 looksOffline 文案）。

      if (body.sections.some(section => (section.claims ?? []).length === 0)) {
+       try {
-       body = { sections: recoverEmptyClaims(body.sections, (await career.material(materialId)).body.sections) };
+         body = { sections: recoverEmptyClaims(body.sections, (await career.material(materialId)).body.sections) };
+       } catch (error) {
+         if (looksOffline(error)) setDraftNotice('网络不可用：已保留本地草稿（可继续编辑，未提交）。联网后请再点「保存修订」显式同步。');
+         throw error;
+       }
      }


─── apps/miniprogram/src/career/progress-preparation.tsx:286-286 ───
[style · low] 准备条目状态文案使用嵌套三元（`item.status === 'draft' ? … : item.status === 'failed' ? … :
'生成中（可恢复）'`），违反「禁止嵌套三元」约定；此位置不在已确认 finding（genErrCode/reviseErrCode/viewErrCode/appendErrCode
链式三元）列举的范围内。本页已有 statusLabels/stageLabels 等 Record 映射先例，建议同样提取 statusLabel 映射（本页与
application-material.tsx 可共用）。

-           <Text className='wk-row-title'>{focusLabels[item.focus] ?? item.focus} · {item.status === 'draft' ? '草稿（可审阅、可修订）' : item.status === 'failed' ? `生成失败（${item.failureCode ?? ''}）：${item.failureMessage ?? '生成未完成'}` : '生成中（可恢复）'}</Text>
+           <Text className='wk-row-title'>{focusLabels[item.focus] ?? item.focus} · {preparationStatusLabel[item.status] ?? item.status}{item.status === 'failed' && item.failureCode ? `（${item.failureCode}）` : ''}</Text>


─── apps/miniprogram/src/career/progress-preparation.tsx:173-173 ───
[bug · low] 对账准备成功后 `setGenNotice('已对账到准备回执。')` 未清 genErrCode：若此前生成失败残留 genErrCode ===
'preparation_version_unknown'，281 行 tone 判定会把这条成功提示以 warning 渲染，让用户误以为仍处于「未确认投递版本」提示态；且
genBusy.error 的旧错误 Notice 也不会随之消失。对比 178 行 retryPrepBusy 成功时正确 `setGenErrCode(undefined)`，此处不一致。

-       <Action secondary loading={recPrepBusy.busy} onClick={() => void recPrepBusy.run(async () => { await career.reconcilePendingPreparation(); setGenNotice('已对账到准备回执。'); void listBusy.run(loadPreparations); })}>用原请求对账准备</Action>
+       <Action secondary loading={recPrepBusy.busy} onClick={() => void recPrepBusy.run(async () => { await career.reconcilePendingPreparation(); setGenErrCode(undefined); setGenNotice('已对账到准备回执。'); void listBusy.run(loadPreparations); })}>用原请求对账准备</Action>


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:46-47 ───
[bug · medium] estimate/todos/ruleView/receipt/subscription/query 等视图 state 均为普通
useState，没有随会话身份（desk 数据键含 userId+tenantId）重置。同一页面栈中 export-deletion 页内即有「退出登录」入口且登出后无
reLaunch，页面实例不会销毁：用户 A 登出、用户 B 登录后返回本页时，上一账号的额度预估、站内待办、规则视图与查询词仍会原样展示（desk 会因 key 变化重载为新账号数据，但这些本地
state 不会）。适配器侧 intent/rule-id 已按 scope 键隔离（t30Key），页面 state 应同样处理。

    // —— 额度预估（执行前只读）——
    const [estimate, setEstimate] = useState<UsageEstimateView>();
+   const scopeKey = `${session.userId}:${session.tenantId}`;
+   const [renderedScope, setRenderedScope] = useState(scopeKey);
+   if (renderedScope !== scopeKey) { // 会话/空间切换：清空上一账号的本地视图，避免跨账号残留
+     setRenderedScope(scopeKey);
+     setEstimate(undefined); setRuleView(undefined); setReceipt(undefined);
+     setTodos(undefined); setSubscription(undefined); setUsageErrCode(undefined);
+     setRuleNotice(''); setReminderNotice(''); setPushNotice('');
+   }


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:48-48 ───
[maintainability · low] usageErrCode 是死状态：loadEstimate 中两处写入（setUsageErrCode(undefined) 与 catch
分支），但整个组件没有任何读取——额度错误提示只用 usageBusy.error 文案展示。对照 ruleErrCode（被 not_found/forbidden 分型使用），这里应删除该
state，或按原意图在额度 Notice 中按 code 分型展示。

-   const [usageErrCode, setUsageErrCode] = useState<string>();
+   // 删除 usageErrCode state；若需分型，改为在额度错误 Notice 中读取：
+   // {usageBusy.error && <Notice tone='danger'>{usageBusy.error}{usageErrCode === 'quota_unavailable' ? ' …' : ''}</Notice>}


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:65-66 ───
[maintainability · low] pendingRuleWrite()/pendingReminderWrite()（以及第 142 行附近的
readStoredRuleId()）在组件渲染体内同步读取本地存储并参与 JSX/disabled 计算，属渲染副作用：若另一入口（如登出的
clearPrivateCache、或后续新增的页外对账）清掉了 intent，本页视图不会自动刷新，恢复态置顶区块可能滞后。与 discovery.tsx 的
pendingAction()/pendingSearch() 是同族既有模式，但既然 desk 用 useData 走了 hook 化加载，建议后续统一收敛为 state（在
accept/abandon 等动作后同步 set）而非渲染期直读。



─── apps/miniprogram/src/career/rules-usage-reminders.tsx:169-170 ───
[style · low] 嵌套三元（本块 enabled/paused/disabled 三分支；第 208 行订阅 unavailable 的
no_templates/api_unavailable/其它三分支同构；第 146/160 行 ruleErrCode
分型亦为双层三元）违反团队「禁止嵌套三元」规范，分支多、文案长，维护时极易读错层级。建议提取为映射函数或提前 return 的辅助函数。

-         {live.status === 'enabled' && live.nextDueAt
-           ? <Text className='wk-row-title'>下次运行（计划）：{formatCheckTime(live.nextDueAt)}。修改规则后该计划按新频率重新排程。</Text>
+         {renderNextRunPlan(live)}
+ // 组件外提取：
+ function renderNextRunPlan(rule: SetRuleReceipt | RuleView): ReactNode {
+   if (rule.status === 'enabled' && rule.nextDueAt) return <Text className='wk-row-title'>下次运行（计划）：{formatCheckTime(rule.nextDueAt)}。修改规则后该计划按新频率重新排程。</Text>;
+   if (rule.status === 'paused') return <Text className='wk-muted wk-small'>已暂停：下一次触发已取消，当前没有排程。恢复启用后按恢复时刻重新排程（顺延，不追补暂停期间的周期）。</Text>;
+   return <Text className='wk-muted wk-small'>规则未启用：不会运行，也不会在后台执行任何搜索。启用后才会排出下次运行计划。</Text>;
+ }


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:154-154 ───
[maintainability · low] expectedRevision: revision! 的非空保证完全依赖按钮 disabled 前置条件（disabled={… ||
revision === undefined || …}），回调内无兜底；同页「退订推送」回调却有显式守卫（if (revision === undefined) throw new
Error('请先读取档案修订')），同一文件内守卫风格不一致。disabled 条件后续若被调整（如允许无修订保存）这里会直接发出
undefined。建议与退订回调对齐，回调内显式判空。（sourceId 空串已有适配器 typed 校验兜底，无需重复。）

-           const input: RuleWriteInput = { ...(existingRuleId ? { ruleId: existingRuleId } : {}), query: query.trim(), intervalMinutes: intervalNumber, status, expectedRevision: revision! };
+           if (revision === undefined) throw new Error('请先读取档案修订');
+           const input: RuleWriteInput = { ...(existingRuleId ? { ruleId: existingRuleId } : {}), query: query.trim(), intervalMinutes: intervalNumber, status, expectedRevision: revision };


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:200-204 ───
[bug · high] 订阅成功后未写回服务端订阅事实，退订后推送在本端永久无法恢复。后端 reminder.go 以档案事实 notifications.push
门禁推送（unsubscribed → Attempted=false 永不推送），而小程序端全局只有本页第 212 行调用
setPushSubscription('unsubscribed')，没有任何 'subscribed' 写回路径；Web InboxPage 在已退订态提供「重新订阅推送提醒」按钮写回
value:'subscribed'。后果：用户一旦点过「退订推送提醒」，再点本按钮永远只是原生授权——服务端事实仍为 unsubscribed、永不尝试推送，但 accepted
文案「是否真的送达由服务端后续推送决定」暗示推送可能发生，且退订提示承诺的「（随时可重新订阅）」在本页无法兑现（只能去 Web 恢复），与本页「如实声明、绝不误导」的口径矛盾。建议 accepted
分支追加写回（并补一条断言该路径的用例）。

          <t-button block size='large' theme='primary' ariaLabel='订阅提醒' customStyle={tdesignButtonStyle} loading={subscribeBusy.busy} onTap={() => void subscribeBusy.run(async () => {
            const outcome = await requestReminderSubscription();
            setSubscription(outcome);
+           // 原生授权≠服务端订阅事实：曾退订（notifications.push=unsubscribed）时必须写回
+           // subscribed，否则服务端永不尝试推送（与 Web InboxPage「重新订阅」路径对齐）。
+           if (outcome.status === 'accepted' && revision !== undefined) await setPushSubscription('subscribed', revision);
            void inboxBusy.run(loadInbox);
          })}>订阅提醒（微信订阅消息）</t-button>


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:210-212 ───
[maintainability · medium] 退订是一次性 client.request 写入，没有 outcome_unknown
恢复链：超时（结果未知）后只显示错误，无对账/原号重试出口，用户重试会用新 requestId 重发；与本页规则/待办写入的恢复链以及 Web InboxPage 的 unknown 分支（查回执 +
原请求编号重试）不一致，也与页首「与 Web 同源同版本」的口径不符。建议复用 career-intent 的 recoverableWrite/intent 机制，或至少对 typed 的
outcome_unknown 提示原号恢复路径。



─── apps/miniprogram/src/career/rules-usage-reminders.tsx:232-232 ───
[bug · low] 登记待办回调的 expectedRevision: revision! 与保存规则回调（先前已确认的问题）是同一模式的第二处实例：非空保证完全依赖 Action 的
disabled 前置条件，回调内无守卫，而同卡片的退订回调有显式判空。修保存规则那一处时请一并覆盖这里，避免 disabled 条件调整后发出 undefined。

-         const next = await createReminder({ sourceKind, sourceId, expectedRevision: revision! });
+         if (revision === undefined) throw new Error('请先读取档案修订');
+         const next = await createReminder({ sourceKind, sourceId, expectedRevision: revision });


─── apps/miniprogram/src/career/rules-usage-reminders.tsx:2-2 ───
[maintainability · low] Taro 导入在本文件内未使用（页面只用 @tarojs/components 与 UI 组件），属冗余导入。career
目录其他页面同样存在此模板残留，可一并在统一清理中删除。



─── apps/miniprogram/src/core/errors.ts:4-5 ───
[documentation · low] 大写 'ARTIFACT_GRANT_EXPIRED'/'ARTIFACT_GRANT_INVALID' 与后端原始 code（小写
artifact_grant_expired/artifact_grant_invalid，见
internal/handler/session/artifact_download.go）并不一致：这是依赖 platform/files.ts 中 downloadErrorCode 把后端小写
code 映射为大写后挂到错误对象的跨文件契约。该契约无任何注释，后续维护者极易"顺手统一大小写"（改任一侧都会使专属提示静默退化为"登录已失效，请重新登录"这类误导文案）。建议像下方
search_quota_refused 分支一样补注释，标明大写值的来源与映射位置。

+   // 大写 ARTIFACT_GRANT_* 非后端原始值（后端为小写 artifact_grant_expired/invalid）：
+   // platform/files.ts 的 downloadErrorCode 会把响应体小写 code 映射为大写后挂到错误对象。
+   // 两端任一侧改大小写都会使本分支不可达，退化为“登录已失效”的误导文案。
    if(e.code==='ARTIFACT_GRANT_EXPIRED')return '下载授权已过期，请再次点击“打开或保存”重新获取。';
    if(e.code==='ARTIFACT_GRANT_INVALID')return '下载授权无效或已被撤销，请重新读取任务产物。';


─── apps/miniprogram/src/platform/files.ts:87-90 ───
[bug · medium] openDocument 的 success 回调仅表示 API 调用成功，真机上原生文档查看器通常仍会异步读取文件句柄；成功后同一 tick 内立即 unlink
私有副本是微信真机的已知坑（iOS 上易出现文档空白/打开失败）。当前验证依据仅为 DevTools（见本文件 D3 注释与 wx-driver.cjs 头部"T33 终局 DevTools
抽查"），真机行为未验证。建议在真机回归确认查看器可正常渲染后再删除，或对删除做短延迟/留待下次启动清理，至少标注该风险待真机验证。



─── apps/miniprogram/src/platform/files.ts:69-71 ───
[style · low] 嵌套三元表达式违反前端规范（"Nested ternary expressions are not allowed"）。建议改为 if 语句或提取小函数，保持与
errors.ts 错误码判定逻辑同风格。

-        const code=r.statusCode===401
-         ?responseCode==='artifact_grant_expired'?'ARTIFACT_GRANT_EXPIRED':'ARTIFACT_GRANT_INVALID'
-         :undefined;
+        let code:string|undefined;
+        if(r.statusCode===401){
+         code=responseCode==='artifact_grant_expired'?'ARTIFACT_GRANT_EXPIRED':'ARTIFACT_GRANT_INVALID';
+        }


─── apps/miniprogram/src/platform/files.ts:44-44 ───
[maintainability · low] 清理失败的错误文案把本机 USER_DATA_PATH
绝对路径（wxfile://usr/...）直接拼进用户可见消息，既不友好也无诊断增量。建议改为稳定错误码（如 LOCAL_CLEANUP_FAILED）并沿用 core/errors.ts
的映射模式输出文案，路径仅留在 cause 中供日志。

-  return new Promise((resolve,reject)=>fs.unlink({filePath,success:()=>resolve(),fail:e=>reject(Object.assign(new Error(`临时副本清理失败：${filePath}`),{cause:e}))}));
+  return new Promise((resolve,reject)=>fs.unlink({filePath,success:()=>resolve(),fail:e=>reject(Object.assign(new Error('临时副本清理失败'),{code:'LOCAL_CLEANUP_FAILED',cause:e}))}));


─── apps/miniprogram/src/platform/files.ts:34-36 ───
[bug · low] 扩展名提取与入口白名单不一致：白名单用 `name.split('.').pop()`，此处用 `lastIndexOf('.')`。对 `.pdf`
这类点开头（无主名）的文件名，白名单判定为合法 "pdf"，但此处 `dot===0` 使 `dot>0` 为假、`ext=''`，副本丢失扩展名——按本文件 D3 注释自述，openDocument
对无正确扩展名的文件会报 filetype not supported，导致该类合法文件在真机无法打开。另外 stem 未限长（OriginalName 可达 1024
字符），超长文件名可能使副本路径超出文件系统限制。建议直接复用入口处已通过白名单校验的 extension 入参，并对 stem 截断。

-  const dot=name.lastIndexOf('.');
-  const ext=dot>0?name.slice(dot+1).toLowerCase():'';
-  const stem=(dot>0?name.slice(0,dot):name).replace(/[^A-Za-z0-9._-]+/g,'-').replace(/^[-.]+/,'')||'artifact';
+ function toUserCopyPath(name:string,validatedExt:string):string{
+  // 复用 openProtectedDocument 已通过白名单校验的扩展名，避免两处取扩展名逻辑不一致
+  const ext=validatedExt;
+  const stem=name.replace(/\.[^.]*$/,'').replace(/[^A-Za-z0-9._-]+/g,'-').replace(/^[-.]+/,'').slice(-64)||'artifact';
+  const suffix=`${Date.now().toString(36)}${Math.random().toString(36).slice(2,8)}`;
+  return `${userDir()}/wk-open-${stem}-${suffix}${ext?`.${ext}`:''}`;
+ }
+ // 调用处：userCopy=toUserCopyPath(name,extension);


─── apps/miniprogram/src/services/career-intent.ts:141-144 ───
[bug · medium] 缺陷:在途恢复与「显式放弃」的竞态未在本模块收口。retryRecoverable 自身不注册
withActiveRecovery,直接使用它的恢复入口——career.ts 的 retryPendingSearch / retryPendingUpload /
reconcilePendingMaterialPublish,career-platform.ts 的 retryPendingRule / retryPendingReminder /
reconcilePendingRule——都没有在途标记;而 UI 侧(discovery.tsx)「放弃恢复」按钮未随重发 loading 禁用。可达序列:重发在途 → 用户点放弃(intent
被清,rule/reminder 的 t30Clear 甚至完全绕过 isRecoveryActive 检查)→ 用户发起新写入、结果未知、落了新 intent → 在途旧重发成功后无条件
store.remove(key) 把新 intent 一并删掉——新写入的唯一对账凭据被静默销毁,正是本文件注释里 ocr3-030 声明绝不可接受的同类事故。建议:在
retryRecoverable 内部统一包 withActiveRecovery(kind, stamp, pending.requestId, …)(单一收口点,所有 kind
自动获得防护),并让 abandonPendingRuleWrite/abandonPendingReminderWrite 走 abandonRecoverable 而非直接
t30Clear;同时各 abandon* 薄包装应把 abandonRecoverable 的布尔返回值透出给 UI(目前 application/material/submission 等
void 包装丢弃了拒绝信号)。另:RecoverableWriteInput.reuseId 全仓库无调用方传参,属死参数,建议删除。



─── apps/miniprogram/src/services/career-intent.ts:121-124 ───
[bug · medium] unresolved_action 守卫是 check-then-act，且发送在途期间没有任何活跃标记：两个并发同 kind 写入（如双击保存触发的两次
saveRule/searchOnce）都会通过未对账检查（intent 只在 catch 里落盘）；若双双进入歧义失败，后写者的 store.write(key, …) 会覆盖前写者的
requestId——这正是守卫注释声称要防的"静默丢掉旧 requestId（唯一对账凭据）"，且第一个请求可能已在服务端落地、本端却永久失去对账入口。同文件 deleteWholeSpace
已示范正确模式（'starting' sentinel + withActiveRecovery 包住发送段）。另外：发送在途时显式放弃会先返回 true，随后 catch
仍会把用户以为已放弃的写入落成 intent，再次触发封锁。建议把 options.send 一并纳入 withActiveRecovery(kind, stamp, id,
…)，让守卫能看见在途写入。

-   const unresolved = readStoredIntent(store, key);
-   if (unresolved) throw Object.assign(new Error(`有一次结果未知的${options.describe}（${unresolved.requestId.slice(0, 10)}…）：请先用原请求对账、安全重发，或显式放弃本次恢复`), { code: 'unresolved_action', requestId: unresolved.requestId });
+   // 发送在途登记活跃标记（对齐 deleteWholeSpace 的 'starting' 模式）：
+   // 并发同 kind 写入被挡在 unresolved_action，放弃也不会与 intent 落盘竞态
+   return withActiveRecovery(options.kind, stamp, id, async () => {
-   try {
+     try {
-     return await options.send(id, options.expected);
+       return await options.send(id, options.expected);
+     } catch (error) { /* …原 catch 三分支… */ }
+   });


─── apps/miniprogram/src/services/career-intent.ts:105-106 ───
[maintainability · low] 死代码：RecoverableWriteInput.reuseId 全仓库没有任何调用方传值（仅此定义与 `options.reuseId ??
newRequestId()` 消费处），且该选项一旦被启用，会绕过 unresolved_action 未对账检查、让"新写入复用旧
requestId"这一需要单独审查的语义无守卫地进入。建议删除该字段，待真有复用需求时连同守卫语义一起设计。

-   /** 重试复用原请求编号（新写入不传）。 */
-   reuseId?: string;
+   // 删除 reuseId 字段：无调用方使用；如未来需要复用请求编号，
+   // 须连同 unresolved_action 守卫语义一并设计。


─── apps/miniprogram/src/services/career.ts:224-227 ───
[bug · medium] 缺陷(与 sweep 重点「unknown request-ID
持久化」不一致):evaluateOpportunity、setPushSubscription(adapters/career-platform.ts)、confirmSharedImport
三处写入即时生成 requestId 且不落任何 intent、不走 recoverableWrite,结果未知(超时/断网)时没有任何对账出口。服务端其实已备好合同:routes_career.go
提供 GET /career/evaluations/receipt,evaluation.go 按 requestId+全量指纹做幂等回放——但客户端超时后即丢失
requestId,该路由永远无法使用;用户用新 requestId 重试会在服务端创建第二条评估记录(服务端仅按 requestId
去重),原请求的落地产出永远无人对账。setPushSubscription 的档案 confirm
事实写同样无未知结果出口,且其兄弟写入(proposeFact/confirmProposal)都走了 desk 的恢复链。建议:evaluateOpportunity 与
setPushSubscription 均改走 recoverableWrite(如 kind 'evaluation'/'push-subscription',配对 reconcile/retry
薄封装);confirmSharedImport 至少可复用 draft.requestId 补一个对账入口。



─── apps/miniprogram/src/services/career.ts:3-3 ───
[maintainability · medium] 可维护性:本文件与 adapters/career-platform.ts 混用两种导入机制——跨包四层相对路径直接 import
packages 内部源文件,与同文件里按包名导入 '@weknora/api-client'(如 NativeFileSource)并存。两个被引用包都声明了 exports
白名单(career-core 只暴露 ./contracts 与 ./desk;api-client 的 exports 无 ./career 子路径,index.ts
也未导出这批解码器),深层相对路径完全绕开了包公开边界:包内文件移动/exports 收紧/构建工具切换(如未来强制 package-relative
解析)时这些路径会静默失效,且同包两套解析入口存在漂移面。建议:在 packages/api-client 的 exports 增加 "./career" 子路径(或从 index.ts 导出所需
decode*/类型),career-core 直接用
'@weknora/career-core/contracts'、'@weknora/career-core/desk',全家族(career/*.tsx 页面同样如此)统一收敛到包名导入。



─── apps/miniprogram/src/services/career.ts:48-48 ───
[bug · low] 一致性缺陷:loadCareer/refreshCareer 都先 activateCareerScope() 把共享 desk 单例绑定到当前身份,唯独
syncFromWeb 直连 desk.syncChanges()。desk(career-core/desk.ts)的作用域只在 activate() 时切换并清空旧视图;若登录身份切换后未经过
open/refresh 就触发同步,changes 请求虽以新凭据发出(服务端按认证派生新 scope),返回的新身份变更会被合并进仍绑定旧身份的 desk——旧
currentView/revision 与新数据糅合,后续 revision() 取到的 CAS 基线也是旧档。建议与两个兄弟函数对齐,先 activateCareerScope() 再
syncChanges()。



─── apps/miniprogram/src/services/career.ts:343-345 ───
[bug · low] 恢复重放的默认方向危险：stored.op 既不是 'edit' 也不是 'confirm'（历史/畸形 intent 缺 op 字段）时会静默落入 confirm
分支——用编辑类 intent 的 materialId + 原 expectedRevision 去 POST
/materials/confirm，可能把用户只想编辑的草稿直接"确认"（不可逆的状态推进）。edit 与 confirm 的重放端点、语义完全不同，未知形状应显式拒绝而非默认取
confirm。建议加一个形状守卫。

-   const stored = pending.input as { op?: 'edit' | 'confirm'; materialId?: unknown; opportunityId?: unknown; snapshotId?: unknown; body?: unknown };
+   if (stored.op !== 'edit' && stored.op !== 'confirm') throw Object.assign(new Error('恢复记录缺少写入种类：请显式放弃本次恢复后重新保存'), { code: 'intent_shape_unknown' });
    return retryIntent<MaterialReceipt>('material', '材料写入', async (id, expected) => {
      if (stored.op === 'edit') {


─── apps/miniprogram/src/services/career.ts:164-168 ───
[maintainability · low] 守卫口径收敛不完整（与 med-39 的收敛声明相悖）：searchOnce 已并入 recoverableWrite
并注释"守卫口径再变时只改一处"，但紧随其后的 retryPendingSearch 仍手写 retryRecoverable catch 里的整套三件套（ambiguousOutcome +
isCurrent/definiteLocalFailure + SCOPE_CHANGED 包装 + unknownOutcome）。两处判据再次并行演化，正是当初 high-7/high-8
漂移的复发形态。建议改为委托 retryRecoverable(store, 'search', '搜索', resend, revision)，仅在最外层保留"附带 query"的错误再包装。

  export async function retryPendingSearch(): Promise<SearchOutcome> {
    const pending = pendingSearch();
    if (!pending) throw new Error('没有待恢复的搜索');
-   const expected = pending.expectedRevision ?? revision();
-   const stamp = auth.scope.capture();
+   try {
+     return await retryRecoverable<SearchOutcome>(store, 'search', '搜索', async (id, expected) =>
+       decodeAs(decodeSearchOutcome, await client.request({ method: 'POST', path: '/api/v1/career/searches', body: { requestId: id, query: pending.query, expectedRevision: expected } })), revision);
+   } catch (error) {
+     if ((error as { code?: unknown })?.code === 'outcome_unknown') throw unknownOutcome((error as { requestId: string }).requestId, pending.query, (error as { cause?: unknown }).cause);
+     throw error;
+   }
+ }


─── apps/miniprogram/src/subpackages/execution/artifact/index.tsx:35-35 ───
[maintainability · medium] customStyle 以内联硬编码形式承载整套 --td-* → --wk-* 主题映射。同一字符串已在本仓库 5 个 career 页面以
`tdesignButtonStyle`
常量重复定义（application-material/discovery/export-deletion/progress-preparation/rules-usage-reminders
各一份），此处是第 6 份拷贝且改为直接内联。主题令牌一旦调整（如新增 pressed 态映射），6 处需同步修改，漂移风险高。由于 .wk-tdesign-scope 已在 app.scss
中定义了 --wk-* 源变量，建议把 --td-* 映射收敛到共享常量（如 components/ui.tsx 导出）或直接落入 app.scss 的 .wk-tdesign-scope
样式规则（CSS 自定义属性可继承穿透小程序自定义组件），本页仅引用。

-      customStyle='--td-brand-color:var(--wk-color-action-primary);--td-brand-color-active:var(--wk-color-action-pressed);--td-brand-color-disabled:var(--wk-color-action-disabled);--td-button-primary-bg-color:var(--wk-color-action-primary);--td-button-primary-active-bg-color:var(--wk-color-action-pressed);--td-button-primary-disabled-bg-color:var(--wk-color-action-disabled);--td-button-primary-disabled-color:var(--wk-color-action-disabled-text);--td-button-large-height:var(--wk-component-button-height);'
+ // 共享模块（如 components/ui.tsx）导出一次：
+ // export const tdesignPrimaryButtonStyle='--td-brand-color:var(--wk-color-action-primary);…';
+ // 页面内：
+      customStyle={tdesignPrimaryButtonStyle}


─── apps/miniprogram/src/subpackages/execution/artifact/index.tsx:48-48 ───
[bug · low] action.error 后无条件追加"授权过期时再次点击……会重新获取"会造成两类文案错误：(1) 当错误恰为 ARTIFACT_GRANT_EXPIRED
时，errorMessage 已返回"下载授权已过期，请再次点击『打开或保存』重新获取。"，追加句与映射文案语义重复，用户看到两句几乎相同的指引；(2) 当错误是其他任何失败（网络失败、403
越权、SCOPE_CHANGED、本地副本清理失败等）时，追加句把非过期故障误导为授权过期场景。errorMessage 对过期错误已内含重取指引，直接展示 action.error
即可；若要保留通用提示，应基于错误 code 条件渲染（需 useAction 暴露原始错误而非仅映射后的字符串）。

-   {action.error&&<Notice tone='danger'>{action.error} 授权过期时再次点击“打开或保存”会重新获取。</Notice>}
+   {action.error&&<Notice tone='danger'>{action.error}</Notice>}


─── apps/miniprogram/tests/application-material.test.mjs:74-75 ───
[maintainability · low] backend() / freshLogin() 测试骨架与 progress-preparation.test.mjs（及既有
career-discovery 等测试）逐字重复，且两组 fixture 工厂（materialReceipt/submission
等）语义高度重叠。后端路由骨架一旦调整（如鉴权头、capabilities 路径）需逐文件同步。建议提取 tests/helpers/career-testbed.mjs 共享
backend/freshLogin 与公共 fixture，各测试文件只保留差异化路由与断言。



─── apps/miniprogram/tests/assembly.test.mjs:108-111 ───
[test · medium] 本用例的 stub.succeed 目标错位，且未真正覆盖「响应迟到被丢弃」路径。时序上：runtime.client.request 经
authorizedTransport.send → sendWithCredential 的首个 `await
credentialStore.read`（mobile-runtime.ts:190）挂起，Taro.request 至少要等一个微任务才会派发；而 runtime.auth.logout() 的
signOut 同步执行 `reserve()`（epoch+1，mobile-runtime.ts:272-274）也先于任何微任务。因此同步执行到
`stub.lastCall('request')` 时：① 挂起的 execution-targets 请求尚未记录，lastCall 拿到的是登录阶段已结算的调用（如富集
/auth/me），succeed 会对它二次触发 success 并灌入无关载荷；② pending 请求随后在 pre-send 的 `current()`
检查（mobile-runtime.ts:191）即被 RUNTIME_SCOPE_CHANGED 拒绝，响应从未「落地」，post-send
丢弃检查（mobile-runtime.ts:195）——即用例名称声称的场景——实际未被行使。旧版（先 await signOut 再
lastCall/succeed）才真正走过迟到响应丢弃路径。建议：轮询等待 execution-targets 调用出现在 stub.state.calls 后再 logout，并 succeed
那个具体捕获的调用（同文件 'a scope switch during transfer' 用例已采用该模式）；另建议 stub.succeed 增加已结算守卫防二次结算。

    const pending = runtime.client.request({ method: 'GET', path: '/api/v1/execution-targets' });
-   const logout = runtime.auth.logout(); // 注销/切空间使旧 scope 失效
+   const dispatchDeadline = Date.now() + 1500;
+   while (!stub.state.calls.some(c => c.kind === 'request' && new URL(c.options.url).pathname === '/api/v1/execution-targets') && Date.now() < dispatchDeadline) {
+     await new Promise(resolve => setTimeout(resolve, 10));
+   }
    const call = stub.lastCall('request');
+   const logout = runtime.auth.logout(); // 注销/切空间使旧 scope 失效
    stub.succeed(call, { data: { success: true, data: [{ id: 'x', kind: 'platform', state: 'active' }] } });


─── apps/miniprogram/tests/assembly.test.mjs:42-44 ───
[maintainability · medium] 本文件内联的 backend/freshLogin/me/authorization 与 helpers/assembly-harness.mjs
近逐字重复——harness 文件头明确记载「最终审查修复 F5」就是为了把两文件 ~40 行重复收敛为单一事实源，本重写又把它展开回来了；且同文件内 TaskOffice 用例仍使用
harness，同一文件内两套并存。本地副本已出现行为漂移：backend 不为 native 调用（downloadFile/uploadFile 无 method 字段）补全
method（harness 版补全为 GET/POST）、不 decodeURIComponent 路径、无 enableChunked 挂起分支；me 固定 tenant 1 不支持
setActiveTenant。建议删掉本地副本，统一从 loadAssemblyHarness 解构使用。

- function freshLogin(extraRoutes = {}) {
-   stub.reset();
-   backend({
+ const { stub, runtime, harness: { freshLogin } } = await loadAssemblyHarness();
+ // 复用 helpers/assembly-harness.mjs 的 backend/freshLogin/me/authorization，避免双份漂移


─── apps/miniprogram/tests/assembly.test.mjs:83-84 ───
[test · medium] 本次重写删除了三组既有回归锚点且未在任何测试文件补位（已全量检索确认）：① bootstrap 恢复存储凭据；② 网络不可达时保持可重试 error 而非静默登出；③
受保护下载通道陈旧 token 先经 refresh-once 再下载（F2 修复锚点，currentBearerToken 的 /auth/me 铸造路径）。切租户中止订阅场景在
office-assembly.test.mjs:214/231 仍有覆盖，但 bootstrap 与 F2 场景当前零覆盖——此 PR 同时改动了 mobile-runtime 的 401
语义（readOnly 分支），锚点丢失会让后续改动无声回退。建议恢复 bootstrap 两用例，并为 currentBearerToken 补「/auth/me 401 → 单飞刷新 →
下载携带轮换 token」用例。



─── apps/miniprogram/tests/assembly.test.mjs:24-25 ───
[maintainability · low] execDto/execEvent 在本文件内定义后无任何引用（全文件仅第 24-25
行出现；office-assembly.test.mjs:16-17 有自己的同款副本并真正使用），属死代码，会误导维护者以为存在未展示的执行流用例。建议直接删除。



─── apps/miniprogram/tests/assembly.test.mjs:237-242 ───
[test · low] 此处替换安装的全局 handler 不区分 call.kind：openProtectedDocument 每次都先经 currentBearerToken() 发起 GET
/auth/me 铸造 token（runtime.ts:61-66），该请求会被此 handler 以 {statusCode:200, tempFilePath, 无 data}
的下载形态响应。用例目前能通过，仅因为 currentBearerToken 从凭据仓现读 token 且授权通道容忍无信封的 200——一旦 runtime 校验 /auth/me
响应形状，这两个工件用例会以与本意无关的方式碎裂。下方 'protected DOCX' 用例的 stub.use 同样如此。建议按邻近用例的模式先判 call.kind ===
'downloadFile' 再响应。

    stub.use(call => {
+     if (call.kind !== 'downloadFile') { call.options.fail({ errMsg: `unexpected ${call.kind}` }); return; }
      stub.state.fileContents.set('/tmp/report.pdf', 'PDF-CONTENT');
      call.options.success({ statusCode: 200, tempFilePath: '/tmp/report.pdf' });
    });
-   const files = await import('../src/platform/files.ts');
-   await files.openProtectedDocument(second, items[0].name, true);


─── apps/miniprogram/tests/assembly.test.mjs:64-65 ───
[maintainability · low] meCount 声明后在本用例（及全文件）无任何引用，属死变量——命名暗示曾想断言 /auth/me
刷新次数但断言未落地，会误导维护者以为存在该守护。建议删除，或补上真正断言（如刷新轮换后 /auth/me 不再触发二次 refresh）。

  test('assembly: scoped GET 401 refreshes single-flight and replays with the rotated token', async () => {
-   let meCount = 0, refreshCount = 0, replayAuth = '';
+   let refreshCount = 0, replayAuth = '';


─── apps/miniprogram/tests/assembly.test.mjs:465-467 ───
[test · medium] 已确认发现的「全局 handler 不区分 call.kind、吞掉 currentBearerToken 的 GET
/auth/me」除了被点名的两个工件用例外，还延伸到本用例及 476（native viewer failure）、488（file-info failure）三处：`stub.use(call
=> stub.succeed(call, { tempFilePath }))` 会把 /auth/me 以无 data
的下载形态响应，用例目前只是「碰巧」通过。修复该问题时请一并覆盖这三处，按同文件 310/357/375/402 用例的写法显式分流 /auth/me，避免后续 runtime 收紧 /auth/me
响应形状校验时这三个用例以无关方式碎裂。

  test('assembly: oversized files are rejected before any private copy exists', async () => {
    await freshLogin();
-   stub.use(call => stub.succeed(call, { tempFilePath: '/tmp/candidate.pdf' }));
+   stub.use(call => {
+     if (call.kind === 'request' && new URL(call.options.url).pathname === '/api/v1/auth/me') { stub.succeed(call, { data: me() }); return; }
+     stub.succeed(call, { tempFilePath: '/tmp/candidate.pdf' });
+   });


─── apps/miniprogram/tests/build-output.test.mjs:14-16 ───
[test · medium] dist 不存在时四个门禁用例（D1/D2/主包 2MB/tdesign 拷贝闭包）全部静默 skip，而 package.json 的 test 脚本（node
--test tests/*.test.mjs）不含前置 build:weapp。在干净检出或不含构建步骤的 CI 作业上，这组门禁整体空转：config/index.ts 的
tdesignClosure 收窄逻辑或组件引用格式一旦回归，门禁仍绿。作为「最终全量门禁」存在漏放风险——建议 CI 路径下改为显式失败（如 process.env.CI 时 skipReason
改为抛错），或在门禁流水线中把 build:weapp 串接在测试之前。

- const appDir = resolve(import.meta.dirname, '..');
- const dist = resolve(appDir, 'dist');
- const skipReason = existsSync(resolve(dist, 'app.json')) ? false : 'dist/ 不存在——先运行 pnpm build:weapp';
+ const hasDist = existsSync(resolve(dist, 'app.json'));
+ const skipReason = hasDist ? false : (process.env.CI ? undefined : 'dist/ 不存在——先运行 pnpm build:weapp');
+ if (process.env.CI && !hasDist) throw new Error('CI 门禁要求先运行 pnpm build:weapp 产出 dist/');


─── apps/miniprogram/tests/career-discovery.test.mjs:62-62 ───
[other · low] 顶部定义的 ambiguous 辅助函数在全部用例中从未被调用（第 130/444 行的匹配仅为测试标题文案），属死代码；且歧义判据的真实实现在
src/services/career-intent.ts 的 ambiguousOutcome，测试文件内留存一份副本会随实现演进漂移、误导后续维护者以为测试复刻了实现判据。建议删除该行。



─── apps/miniprogram/tests/core.test.mjs:49-50 ───
[test · medium] 本文件删除了 normalizeApiOrigin 与 isBearer 的纯函数用例，且已确认无任何其它测试文件覆盖
src/core/auth.ts。normalizeApiOrigin 仍在生产路径上（runtime.ts:18-20 注释明确说明 host 大小写归一化错漏会导致同一 host
不同大小写构建间会话孤立、旧 token 残留），isBearer 保护凭据形状校验——这两个安全相关不变量现在零回归保护。建议保留这两组用例而非随路由清单断言一起删除。

- test('route manifest has twenty-five screens and only four primary tabs',()=>{
-   assert.ok(routing.ROUTES);
+ const authPure = await import(`../src/core/auth.ts`);
+ test('normalizeApiOrigin lowercases the host and drops trailing slashes only', () => {
+   assert.equal(authPure.normalizeApiOrigin('https://API.example.test/'), 'https://api.example.test');
+   assert.equal(authPure.normalizeApiOrigin('https://api.example.test/x').endsWith('/x'), true);
+ });


─── apps/miniprogram/tests/export-deletion.test.mjs:615-615 ───
[test · low] 断言弱化：此处把页面应有的 unknown 判定逻辑（deletionController.isUnknown 的语义：intentPresent &&
inMemoryStatus !== 'partial'）在测试里手工重写为 gating 输入，而非经 createDeletionPageController
的真实接线取值。若页面后续偏离该判定（如不再透传 inMemoryStatus 或改用别的推导），本用例仍会通过，无法证伪 partial 场景的页面接线——F3 只覆盖纯 controller
状态迁移，N3 只做源码正则匹配，均不补此缺口。建议参照 F1/N9 的做法，用真实 service 生命周期驱动 controller（accept(partial) →
recoveryFailed）后断言 isUnknown，再喂给 lifecycleGating。



─── apps/miniprogram/tests/helpers/taro-stub.mjs:99-102 ───
[test · low] copyFile（及同模式的 writeFile）在校验 srcPath 存在性/数据合法性之前就 push 进
state.copies/state.fileWrites，使「尝试复制」与「复制成功」语义混同：依赖 copies 内容的断言（如 copies.length===1、removedFiles 指向
copies[0].destPath）在复制实际失败时仍会留下成功痕迹，削弱对 files.ts 清理路径的守护。建议仅在成功分支记录，或把尝试单独记到 attempts 字段。

        copyFile({ srcPath, destPath, success, fail }) {
          queueMicrotask(() => {
-           state.copies.push({ srcPath, destPath });
            if (!state.fileContents.has(srcPath)) { fail?.({ errMsg: `copyFile:fail no such file or directory, open ${srcPath}` }); return; }
+           state.copies.push({ srcPath, destPath });


─── apps/miniprogram/tests/live/quota-inject-proxy.mjs:3-5 ───
[documentation · high] 头注释的核心前提已过时，与当前代码事实矛盾：office.go:345 虽初始化 passThroughSearchQuotaGate，但 349-351
行随即以 `office.searchQuotaGate = searchUsageGate{office: office}` 覆盖（注释原文 "T21 upgrades the T11
pass-through into the real usage ledger"），usage.go:118-123 也明确声明 searchUsageGate 是生产 gate、拒绝即 typed
ErrSearchQuotaRefused(429)。即「真实额度账本属 T21，未落地」不成立：本仓库构建的任何含 career 模块的服务器在额度耗尽（或部署 env 配一个极小的
CAREER_SEARCH_QUOTA_LIMIT，见 usage.go:141-151）时都能原生产出 typed 429，未必「必须注入」。注入的响应形状本身与
handler.go:122-124/196 冻结合同逐字一致、客户端链路验证仍有效，但这条失实前提会随 manifest 写入门禁证据，误导对生产系统状态的认定，且 office.go:299
行号引用也错误（实为 345/351）。建议改写注释如实反映 T21 已落地，并评估 Lite 环境改用真实 gate 触发（如设极小 quota limit）替代注入。



─── apps/miniprogram/tests/live/quota-inject-proxy.mjs:29-29 ───
[other · low] 代理健壮性不足：(1) upstream 请求未设超时，上游长挂起会让驱动卡在 waitText 直至超时；(2) 未监听入站 req 的
'aborted'/'close'，客户端（小程序/驱动）断开后 upstream 仍继续消耗并可能向已关闭的 res 写入触发二次错误；(3) 转发时未剥离
connection/keep-alive 等 hop-by-hop 头。建议 upstream 设 timeout、req.on('close') 时 upstream.destroy()，并过滤
hop-by-hop 头。另注意：FLAG 若在驱动被强杀（SIGKILL，非异常路径）时残留，之后所有 POST 搜索都会持续被拒，只能人工 rm 恢复——可在代理侧给 FLAG 加过期时间兜底。



─── apps/miniprogram/tests/live/quota-inject-proxy.mjs:30-31 ───
[bug · medium] upstream 响应流 `up` 的 error 事件未监听：`up.pipe(res)` 期间若上游连接中断（Lite 服务器重启、socket 半途断开），`up`
上的 'error' 事件没有 handler，会以 uncaughtException 直接杀死整个代理进程，小程序侧所有后续请求（含 F1
取证本身的注入路径）全部失败，且驱动侧只能看到连接拒绝，根因不可追溯。已确认的健壮性意见覆盖了请求方向的 timeout/close 与 hop-by-hop 头，但未覆盖响应流。建议在回调内补
`up.on('error', ...)`（销毁 res 并记录日志），或改用 `stream.pipeline(up, res, err => ...)` 统一处理。

+   }, up => {
+     up.on('error', err => { console.error('[quota-proxy] upstream response error', err.message); res.destroy(); });
      res.writeHead(up.statusCode ?? 502, up.headers);
      up.pipe(res);
+   });


─── apps/miniprogram/tests/live/t24r1-live-driver.cjs:235-235 ───
[test · medium] F1 截图的 proves 字符串把 quota-inject-proxy.mjs 头注释中已过时的声明（「生产 gate 为 passThrough、真实账本属
T21」）原样固化进 SHOTS/manifest.json 证据。实际 office.go:349-351 已用 searchUsageGate 覆盖
passThrough，usage.go:118 明确其为生产 gate。门禁证据中记录关于被审计系统的失实陈述会损害 F1 取证结论的可信度，应随代理注释一并更正为当前事实。



─── apps/miniprogram/tests/live/t24r1-live-driver.cjs:180-181 ───
[bug · medium] record('B server-pending', ...) 记录失败后未提前终止，下一行立即访问 pendingProposal.id：当服务端未返回预期
proposal（如接口形状变化或提案已被并发确认）时将抛 TypeError「Cannot read properties of undefined (reading 'id')」，顶层 catch
只剩该解析错误，既掩盖真实根因（提案缺失）又直接中断后续 C/F2/F1 全部取证。建议 pendingProposal 缺失时抛出带上下文的显式错误（含 openAfter 的 proposals
概要）并退出。同理登录处 li[0]/li[1] 与 inputs[0] 的元素获取未校验数量，选择器落空时报错难定位。



─── apps/miniprogram/tests/live/t24r1-live-driver.cjs:28-31 ───
[maintainability · low] 环境变量校验顺序不当且不完整：T24R1_TOKA 缺失时 fs.readFileSync(undefined) 会以「The \"path\"
argument must be of type string」崩溃，而非走到下方 missing T24R1_* env
的友好提示；T24R1_USER_EMAIL/T24R1_USER_PASS/TA 本身也未纳入校验；ORIGIN 不含端口时 Number(new URL(ORIGIN).port) 为
NaN，api() 与 connectRetry 会以难以定位的方式失败。建议先集中校验全部必需 env（含 TOKA 文件存在性）再执行读取。



─── apps/miniprogram/tests/live/t24r1-live-driver.cjs:178-178 ───
[maintainability · low] api() 响应体直接 JSON.parse 无容错：当上游经代理返回 502 纯文本（quota-proxy upstream error）或非
JSON 时抛 SyntaxError，顶层捕获后 detail 仅剩解析错误，无法区分是哪一步接口先失败（openAfter、cur1、searchResp 三处同病）。建议 api() 内校验
status 并在 JSON.parse 外包一层带 method/path/status 上下文的错误。



─── apps/miniprogram/tests/live/t24r1-live-driver.cjs:232-233 ───
[test · low] 该断言存在假失败风险：snapshotMatching(/搜索额度不足/) 返回的是第一张命中额度提示的快照，而按本文件自述「单次快照可能不完整」，待确认搜索 Notice
的 text 节点可能恰好缺席该快照，导致 /有一次结果未知的搜索/.test(afterQuota) 假失败（回退值 quotaText
同样不保证包含）。建议改为单一组合正则在同一快照中同时命中两段文案（如 /搜索额度不足[\s\S]*有一次结果未知的搜索|有一次结果未知的搜索[\s\S]*搜索额度不足/），与
verifiedShot 的状态断言口径一致。



─── apps/miniprogram/tests/live/t24r1-live-driver.cjs:145-147 ───
[test · medium] writeIntent 注入的是历史 T24 兼容形状 `{requestId, query}`，而非当前生产形状 `{requestId,
input:{query}, expectedRevision}`（见 src/services/career.ts:130-131 注释：intent 形状已收敛为 input.query +
expectedRevision 持久化，pendingSearch 仅兼容读取旧形状）。后果：(1) F2/F1 的「安全重发原搜索」走的是 retryPendingSearch 中
`pending.expectedRevision ?? revision()` 的兜底分支（用当前修订值），生产主路径「逐字节重放原 expectedRevision」（幂等红线，OCR
high-8 修复的核心）在这套取证中实际未被覆盖；(2) 恢复态与生产 intent 形状不一致，降低证据保真度。建议注入新形状并携带与原请求一致的 expectedRevision，使 F2/F1
的重发取证覆盖真实重放分支。

- async function writeIntent(mp, requestId, query) {
-   await mp.callWxMethod('setStorageSync', searchStoreKey, { requestId, query });
+ async function writeIntent(mp, requestId, query, expectedRevision) {
+   await mp.callWxMethod('setStorageSync', searchStoreKey, { requestId, input: { query }, expectedRevision });
  }


─── apps/miniprogram/tests/live/t24r1-live-driver.cjs:118-121 ───
[test · medium] verifiedShot 对「要证明的关键元素」的定位只做单次 `page.$$('text')`
扫描，与文件自述矛盾：注释明确说「单次快照可能不完整（偶发缺节点甚至只剩页面标题）」，因此状态断言一律走 snapshotMatching 多拍——但 visible
定位若恰好赶上不完整快照未命中，scrolledTo 静默保持 0（沿用上一次截图遗留的滚动位置或从顶部拍摄），关键元素可能不在视口内，与设计意图「把关键元素滚动进视口再截」相悖，而后续
mustMatch 三拍断言与截图时刻的视口并不保证一致。建议 visible 定位同样多拍重试（复用 snapshotMatching 的思路）。另附带：4 次 attempt 全因『text
lost after scroll』continue 耗尽时，最终抛出的是『stayed byte-identical』错误信息，与真实失败原因（文本丢失）不符，误导排障。

      if (visible) {
+       for (let poll = 0; poll < 6 && scrolledTo === 0; poll++) {
-       for (const el of await page.$$('text')) {
+         for (const el of await page.$$('text')) {
-         const t = ((await el.text()) || '').trim();
+           const t = ((await el.text()) || '').trim();
-         if (t && visible.test(t)) {
+           if (t && visible.test(t)) {
+             try {
+               const off = await el.offset();
+               const top = Number((off && (off.top ?? off.y)) ?? 0);
+               scrolledTo = Math.max(0, Math.round(top - nudges[attempt - 1]));
+               break;
+             } catch (e) { log('offset failed', e.message); }
+           }
+         }
+         if (scrolledTo === 0) await sleep(700); // 单次快照可能不完整，多拍找 visible 元素
+       }
+       if (scrolledTo > 0) { await mp.pageScrollTo(scrolledTo); await sleep(700); }
+     }


─── apps/miniprogram/tests/live/t24r1-live-driver.cjs:218-218 ───
[test · medium] F2-recovered 与 C-search-reconciled 的 mustMatch、visible
正则完全相同（/no_vetted_sources…覆盖来源（0）/ 与 /搜索未完成/），二者的最终可视状态在语义上是同一页面（同一个诚实失败回执 + 覆盖来源（0）+
档案事实），差异只在到达路径（对账成功 vs 安全重发成功）——路径差异不产生任何可视差异。此时 verifiedShot 的 clash 机制会用 nudges=[120,40,200,0]
换滚动偏移重拍，制造出字节级不同的两张截图，使 F3 的『两两哈希不等』断言被『同一状态的不同视口』满足——manifest 里两张图声称证明不同结论，实际只是同一可视状态的两个视口，恰是 Wave
4 finding（不可区分证据）要防的问题换个形态重现。建议对这两处要求各自独特的可视锚点（例如 F2-recovered 额外断言重发成功后 receiptMissing
恢复入口消失/档案事实仍在视口内的组合），或在 manifest 中显式声明两图可视等价、仅以滚动位置区分，避免夸大证据区分度。



─── apps/miniprogram/tests/rules-usage-reminders.test.mjs:106-106 ───
[test · medium] 测试组对规则写入只覆盖了创建路径：A1 断言请求体键集合为
['expectedRevision','intervalMinutes','query','requestId','status']（不含
ruleId）。而页面（rules-usage-reminders.tsx 第 154 行）在本地已有 receipt/ruleView 时会携带 ruleId
走更新分支——这是首次保存之后每次保存的主路径。后端合同本身允许（search_rule.go SetRuleInput.RuleID：空=创建、非空=更新），但本组没有任何携带 ruleId
的用例，该主路径零覆盖；若适配器序列化或合同演进回归，无法被本组测试发现。建议补一条更新用例。

-   assert.deepEqual(Object.keys(body).sort(), ['expectedRevision', 'intervalMinutes', 'query', 'requestId', 'status'], 'the body must match the frozen SetRuleInput (DisallowUnknownFields)');
+ // 在 A1 之后补更新路径用例：
+ test('A1b: saving with the stored rule id posts the update contract (ruleId included)', async () => {
+   await freshLogin({
+     'POST /api/v1/career/rules': call => stub.succeed(call, { data: ruleReceipt({ revision: 2, intervalMinutes: 60, status: 'paused' }) }),
+   });
+   await career.loadCareer();
+   const receipt = await platform.saveRule({ ruleId: 'rule-1', query: '上海 前端开发 实习', intervalMinutes: 60, status: 'paused', expectedRevision: 3 });
+   assert.equal(receipt.revision, 2);
+   const body = ruleWrites()[0].options.data;
+   assert.deepEqual(Object.keys(body).sort(), ['expectedRevision', 'intervalMinutes', 'query', 'requestId', 'ruleId', 'status'], 'the update body carries the optional ruleId of the frozen SetRuleInput');
+ });


─── apps/miniprogram/tests/rules-usage-reminders.test.mjs:26-26 ───
[test · low] clearPrivateCache 导入后从未使用（A5 用的是 runtime.auth.logout()，progress-preparation.test.mjs 的
logoutLikeRuntime 模式才需要它），属死代码，建议删除以免误导后续维护者以为本组有登出清缓存断言。



─── apps/mobile/src/task-office-integration-smoke.ts:46-50 ───
[maintainability · low] 两个稳健性问题:(1) 端点路径 '/api/v1/workbench/executions?limit=1' 硬编码——api-client 的
executions.ts 使用同一路径但同样内联,无共享常量;一旦服务端路由调整,探针会静默打到错误路径(非 401/403 → 误报 failed-open),违背探针目的。建议在
api-client 导出路径常量供探针复用,或至少在探针旁注明与 executions.ts 的对应关系。(2) fetch 无超时:服务端挂起(接受连接但不响应)时 Node fetch
没有默认总超时,smoke 采集会长时间阻塞后才落入 catch 记为 unreachable。建议加 AbortSignal.timeout,让挂起与不可达一样被有界地如实记录。

      const response = await fetch(new URL('/api/v1/workbench/executions?limit=1', deploymentOrigin).toString(), {
        method: 'GET',
        redirect: 'error',
        headers: { accept: 'application/json' },
+       signal: AbortSignal.timeout(10_000),
      });


─── apps/mobile/src/task-office-integration-smoke.ts:95-95 ───
[test · low] 证据语义缺口:restoreFailed 为 true 时(archive 已成功、restore 失败——可能是网络故障,也可能是 scope 被撤销导致 mutate 的
requireLease 先行拒绝),任务大概率仍处归档态需要人工清理,但证据只记 archiveRestore: 'failed',未复用 'cleanup-required' 语义(该值目前仅在
archive 失败 + scope 变化分支使用)。对采集诚实性而言,'failed' 无法区分"已恢复但列表校验失败"与"任务仍被归档、需要清理"。建议 restore 失败时同样检查
scopeStillMatches():scope 仍匹配 → 'failed'(可重试),scope 已失效 → 'cleanup-required'。

-   if (restoreFailed) return { archiveRoundtrip: 'failed', archiveRestore: 'failed' };
+   if (restoreFailed) {
+     let sameScope = false;
+     try { sameScope = scopeStillMatches(); } catch { /* treat an unreadable scope as changed */ }
+     return sameScope
+       ? { archiveRoundtrip: 'failed', archiveRestore: 'failed' }
+       : { archiveRoundtrip: 'failed', archiveRestore: 'cleanup-required' };
+   }


─── apps/web/package.json:9-9 ───
[maintainability · low] 硬编码 --test-concurrency=4 且无取值依据说明：Node 默认并发为 os.availableParallelism()-1，固定
4 在高核机器上人为压低吞吐、在 ≤4 核 CI 上仍可能超订。若是为规避 jsdom/端口争用而收敛，建议改为环境变量透传（如
`--test-concurrency=${WEKNORA_TEST_CONCURRENCY:-4}` 的等价 shell 形态）或至少补一行注释记录动机，避免后续无人敢动/不知为何而设。



─── apps/web/src/DevMarkdownPage.tsx:4-4 ───
[maintainability · medium] 无消费点的跨域样式导入：本页 DOM
类名（.markdown-test-page/.test-rendered/.chat-code-block*/.shimmer-demo 等）全部由文件内 PAGE_CSS 承载，与
views-chat-u.css（前缀全为 .wk-vc-*，本文件无任何 wk-vc- 引用）零交集。该导入既把约 2500 行无关 chat 样式拉入 dev 页面依赖图，又与第 274-275
行「node 测试无法解析 CSS 所以 PAGE_CSS 保持内联」的既定约束冲突——现已迫使 DevMarkdownPage.test.ts 额外增加 registerHooks stub
才能加载本模块。建议删除该导入；若确有依赖（如与 ChatRoutePage 的样式顺序约束），请在导入处注释说明具体依赖点。

- import './chat/views-chat-u.css';
+ // 若无实际消费点则移除；确需保留时注明依赖类名/顺序约束：
+ // import './chat/views-chat-u.css'; /* 消费点：<类名/规则>，顺序约束：<说明> */


─── apps/web/src/DevMarkdownPage.tsx:259-262 ───
[maintainability · low] 该后置改写正则存在两处静默失效/误伤边界（当前 fixture 下行为正确，作为 parity 工具建议在注释中声明或加防护）：1) `([^<]*)`
要求 strong 内容为纯文本，三重强调内容含内联元素（如 `***a `b` c***` → <em><strong>a <code>…）时不再匹配，改写会静默失效，输出回退为 React
原生渲染，与 Vue 端（repairFlankingEmphasis 仍会破坏该形态）不一致；2) 正则会命中所有 <em><strong> 来源（如 `_**x**_`），而 Vue 端
repairFlankingEmphasis 针对下划线形式的 flanking 行为未必相同，custom 输入区这类写法会被错误改写。建议在注释中补充此局限，或在
DevMarkdownPage.test.ts 中加一条针对纯文本三重强调的快照断言，防止未来扩充 fixture 时静默漂移。

+   // 已知局限：仅覆盖纯文本内容的三重强调（strong 内含 <code>/<a> 等内联元素时不改写，
+   // _**x**_ 等下划线组合也会被一并改写）；当前 fixtures 仅 ***加粗斜体*** 一处命中。
    return renderChatMarkdown(markdown).replace(
      /<em><strong>([^<]*)<\/strong><\/em>/g,
      '<em><em></em>$1</em>**',
    );


─── apps/web/src/administration/AdministrationPage.tsx:155-155 ───
[bug · medium] 邀请邮箱输入由原 `<Input type="email" required ...>` 改为 TInput 后，`type="email"` 与 `required`
携带的浏览器原生提交校验一并丢失；`onInviteFormSubmit` 与 `sendInvitation` 均只做 `email.trim()` 非空检查，任意非空字符串（如
"abc"）都能进入 confirm 步骤并直接调用
`identity.tenants.invitations.create`，邮箱格式校验只剩服务端兜底（AdministrationPage.test.tsx
也未覆盖非法格式用例）。建议在提交处理中补回格式校验（正则或 TInput 的 status+tips 反馈）。

- return <main className="wk-page wk-admin-1"><header className="wk-header wk-admin-2"><div><p className="wk-eyebrow wk-admin-3">{t('mobileAdministration.tenant', { tenant: tenantId })}</p><h1 className="wk-admin-4">{systemAdmin ? t('settings.navGroups.systemAdministration') : t('mobileAdministration.title')}</h1><p className="wk-muted wk-admin-5">{t('mobileAdministration.readOnly')}</p></div><TButton type="button" onClick={() => void load()} disabled={loading}>{t('mobileAdministration.refresh')}</TButton></header>{error ? <Status tone="error">{error}</Status> : null}<div className="wk-admin-12"><Card><h2 className="wk-admin-6">{t('mobileAdministration.members', { count: membersTotal })}</h2><TInput type="search" value={memberSearch} placeholder={t('mobileAdministration.searchPlaceholder')} aria-label={t('mobileAdministration.searchPlaceholder')} onChange={(value) => setMemberSearch(String(value))} className="wk-admin-7" />{loading ? <Status>{t('mobileAdministration.loading')}</Status> : members.length === 0 ? <Status>{appliedMemberSearch ? t('mobileAdministration.noMembersForQuery', { q: appliedMemberSearch }) : t('mobileAdministration.noMembers')}</Status> : <ul className="wk-list wk-admin-8">{members.map((member) => <li key={member.user_id} className="wk-admin-9"><div className="wk-list-item-copy wk-admin-10"><strong>{member.username}</strong><span className="wk-admin-11">{member.email} · {roleText(member.role)} · {member.status}</span></div><div className="wk-list-actions wk-admin-13"><TSelect value={member.role === 'owner' ? 'owner' : member.role} disabled={member.role === 'owner'} onChange={(value) => void updateRole(member, String(value) as TenantMember['role'])} className="wk-admin-14" options={[{ value: 'owner', label: roleText('owner') }, { value: 'admin', label: roleText('admin') }, { value: 'contributor', label: roleText('contributor') }, { value: 'viewer', label: roleText('viewer') }]} /><TButton type="button" disabled={member.role === 'owner'} onClick={() => void removeMember(member)}>{t('mobileAdministration.remove')}</TButton></div></li>)}</ul>}{membersTotal > 0 ? <MembersPager total={membersTotal} page={membersPage} pageSize={membersPageSize} onPage={onMembersPage} onPageSize={onMembersPageSize} t={t} /> : null}</Card><Card><h2 className="wk-admin-6">{inviteStep === 'confirm' ? t('mobileAdministration.confirmInviteTitle') : t('mobileAdministration.invite')}</h2>{inviteStep === 'confirm' ? <div className="wk-admin-15"><p className="wk-muted wk-admin-16">{t('mobileAdministration.confirmInviteBody', { email: email.trim(), role: roleText(role) })}</p><div className="wk-admin-17"><TButton type="button" disabled={saving} onClick={() => setInviteStep('form')}>{t('mobileAdministration.back')}</TButton><TButton type="button" loading={saving} onClick={() => void sendInvitation()}>{t('mobileAdministration.confirmSend')}</TButton></div></div> : <form className="wk-admin-15" onSubmit={onInviteFormSubmit}><label className="wk-admin-18">{t('mobileAdministration.inviteEmail')}<TInput className="wk-admin-invite-email" value={email} onChange={(value) => setEmail(String(value))} /></label><label className="wk-admin-18">{t('mobileAdministration.role', { role: '' }).replace(/: $/, '')}<TSelect value={role} onChange={(value) => setRole(String(value) as typeof role)} options={[{ value: 'admin', label: roleText('admin') }, { value: 'contributor', label: roleText('contributor') }, { value: 'viewer', label: roleText('viewer') }]} /></label><TButton type="submit" loading={saving}>{t('mobileAdministration.sendInvitation')}</TButton></form>}</Card><Card><h2 className="wk-admin-6">{t('mobileAdministration.openInvitations')}</h2>{invitations.filter((item) => invitationIsOpen(item.status)).length === 0 ? <Status>{t('mobileAdministration.noPendingInvitations')}</Status> : <ul className="wk-list wk-admin-8">{invitations.filter((item) => invitationIsOpen(item.status)).map((item) => <li key={item.id} className="wk-admin-9"><div className="wk-list-item-copy wk-admin-10"><strong>{item.invitee_email ?? item.invitee_user_id}</strong><span className="wk-admin-11">{roleText(item.role)} · {t('mobileAdministration.expires', { date: item.expires_at })}</span></div><TButton type="button" onClick={() => void revokeInvitation(item)}>{t('mobileAdministration.revoke')}</TButton></li>)}</ul>}</Card><Card><h2 className="wk-admin-6">{t('mobileAdministration.auditLog')}</h2>{audit.length === 0 ? <Status>{t('mobileAdministration.noAuditEntries')}</Status> : <ul className="wk-list wk-admin-8">{audit.map((item) => <li key={item.id} className="wk-admin-9"><div className="wk-list-item-copy wk-admin-10"><strong>{item.action}</strong><span className="wk-admin-11">{item.outcome} · {item.actor_role}</span><small>{item.created_at} · {item.request_method} {item.request_path}</small></div></li>)}</ul>}</Card>{systemAdmin ? <><Card><h2 className="wk-admin-6">{t('settings.navGroups.systemAdministration')}</h2><ul className="wk-list wk-admin-8">{admins.map((item, index) => <li key={String(item.id ?? index)} className="wk-admin-9"><strong>{String(item.username ?? item.email ?? item.id)}</strong><span className="wk-admin-11">{item.is_active === false ? t('common.disabled') : t('mobileAdministration.status.active')}</span></li>)}</ul></Card><div data-testid="system-administration-panels" className="wk-admin-19">{systemAdminPanelKeys(systemAdmin).map((panel) => panel === 'system-global' ? <SystemGlobalSettingsPanel key={panel} client={client} initialSettings={settings} /> : panel === 'runtime-queues' ? <RuntimeQueuesPanel key={panel} client={client} payload={queues} loading={loading} /> : panel === 'platform-api-keys' ? <PlatformApiKeysPanel key={panel} client={client} initialKeys={apiKeys} /> : <SystemAuditLogPanel key={panel} client={client} payload={systemAudit} />)}</div></> : null}</div></main>;
+ // onInviteFormSubmit 内补回格式校验（TDesign InputType 不含 'email'，无法仅靠 type 属性还原原生校验）
+ const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
+ function onInviteFormSubmit(event: React.FormEvent<HTMLFormElement>) {
+   event.preventDefault();
+   if (!manageTenant || !EMAIL_RE.test(email.trim())) return;
+   setInviteStep('confirm');
+ }


─── apps/web/src/administration/AdministrationPage.tsx:206-207 ───
[bug · low] 跳页输入由原 Input 换为 TInput 时丢失了 `inputMode="numeric"`，移动端不再唤起数字键盘，与该输入框仅接受页码的语义不符，属轻微 UX
回归。建议补回该属性（若 tdesign-react Input 类型未声明 inputMode，需确认其透传到内部原生 input 的行为）。

-       <TInput className="wk-admin-pager-jump wk-admin-25" type="text" value={jump}
+       <TInput className="wk-admin-pager-jump wk-admin-25" type="text" inputMode="numeric" value={jump}
          onChange={(value) => setJump(String(value))}


─── apps/web/src/administration/administration-u.css:18-23 ───
[maintainability · low] .wk-admin-pager-btn 内先声明 `font-weight: 400` 又声明 `font-weight:
inherit`（后者胜出）——这是原 utilities 中 `font-normal` 被 `[font:inherit]` 覆盖的平移残留，`font-weight: 400`
为死声明，会误导后续维护者以为生效字重是 400。建议删除 `font-weight: 400;` 一行。

-   font-weight: 400;
    color: rgb(0 0 0/90%);
    font-family: inherit;
    font-size: inherit;
    line-height: inherit;
    font-weight: inherit;


─── apps/web/src/administration/administration-u.css:52-59 ───
[maintainability · low] .wk-admin-3 将原 `text-primary` 硬编码为 #2e6de6，而 apps/web/src/styles.css 已定义
`--color-primary: #2e6de6`，且本文件末尾 .wk-admin-pager-current 已采用 `var(--color-accent, #07c05f)`
的引用模式——此处是文件内唯一未引用令牌的 primary 色，主题令牌调整后会产生漂移。建议改为 var() 引用。另注：muted/边框的 rgba(0,0,0,.6)/#e7e7e7
字面量虽有映射注释且为本次 sweep 的统一策略，但 tdesign-theme.css 在暗色主题下对
--td-text-color-secondary/--td-component-stroke 有覆盖值，字面量不会跟随切换，若策略允许建议统一改为 var() 引用。

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
[maintainability · low] 手工段的 hover 态 accent 色写字面量 #07c05f，而紧随其后的 .wk-admin-pager-current 两条规则用的是
var(--color-accent, #07c05f)（styles.css:457 已定义 --color-accent，且全库迁移文件均采用 var()
引用模式）。同一令牌在同一文件内两种编码，一旦主题令牌调整，‹ › 按钮的 hover 色会与当前页高亮色脱钩漂移。建议统一改为 var() 引用。

  .wk-admin-pager-btn-disabled:enabled:hover {
-   color: #07c05f;
+   color: var(--color-accent, #07c05f);
  }


─── apps/web/src/agent-marketplace/am-u.css:97-99 ───
[maintainability · low] `.wk-amr-14` 将原 Tailwind `break-all`（语义 = word-break: break-all）改写为
`overflow-wrap: anywhere`，两者断行语义不等价：`anywhere` 仅在溢出时断行且会参与 min-content 尺寸计算，而 `break-all`
在任意字符间允许断行。该 code 位于 `.wk-amr-13` 的 grid 容器内，长十六进制 bundle_digest
的换行表现可能与迁移前不一致。项目内其他迁移文件（settings.td.css、kb-list.td.css、documents.td.css、career/*.css 等 20+ 处）对
break-all 统一映射为 `word-break: break-all`，此处属单点偏差，建议对齐：

  .wk-amr-14 {
-   overflow-wrap: anywhere;
+   word-break: break-all;
    border-radius: 4px;


─── apps/web/src/agent-marketplace/am-u.css:126-128 ───
[maintainability · low] `.wk-amr-18` 与 `.wk-ava-11` 将 `bg-accent` 硬编码为 `#07c05f`（`.wk-amr-7` 同理将
`bg-surface` 硬编码为 `#ffffff`）。当前取值与 styles.css :root 的 `--color-accent: #07c05f` / `--color-surface:
#ffffff` 一致，且其他多数 *-u.css 亦如此硬编码，属既定约定；但项目内已有使用 CSS
变量带兜底的先例（administration-u.css:220、kb-list.td.css:2473、settings-wrapper.css 均为 `var(--color-accent,
#07c05f)`），建议改为变量写法，使 token 取值演进或后续主题化时此处自动跟随，避免多点漂移：

  .wk-amr-18 {
    border-radius: 4px;
-   background-color: #07c05f;
+   background-color: var(--color-accent, #07c05f);


─── apps/web/src/agents/AgentEditorModal.tsx:1033-1033 ───
[maintainability · medium] 16 处 `as never` 类型逃逸（本处 inputProps 传 data-field；另见
816/892/916/1103/1118/1174/1233/1276/1339/1399/1425/1624/1654/1863/1877）。测试套件（agentEditorModal.test.
tsx 29+ 处 [data-field] 选择器）依赖 tdesign 在运行时把未知属性透传到内部 input，而该契约已被 as never 从类型层屏蔽——tdesign
升级改动透传策略时编译期零报警、测试成批静默失效。建议收敛为单一类型化辅助函数（一处强转 + 注释说明运行时依据），或通过模块扩展声明 data-* 支持。

-                   inputProps={{ "data-field": "max_completion_tokens" } as never}
+                 <InputNumber
+                   className="wk-ae-num-max_completion_tokens"
+                   inputProps={numberFieldProps('max_completion_tokens')}
+ 
+ // 文件顶部统一收口（tdesign 运行时会把未知 props 展开到内部 input；测试依赖此钩子）：
+ const numberFieldProps = (field: string): ComponentProps<typeof InputNumber>['inputProps'] =>
+   ({ 'data-field': field } as ComponentProps<typeof InputNumber>['inputProps']);


─── apps/web/src/agents/AgentEditorModal.tsx:2011-2017 ───
[maintainability · medium] 可访问性回退：导航项由 <button type="button"> 降级为 div + onClick，丢失键盘可达性（无
tabIndex/role/键盘事件），Tab 无法聚焦、Enter/Space 无法切换分区；同文件 placeholder-tag（702-706 的 span，点击插入占位符）同类问题。建议恢复
button 或补 role="button" tabIndex={0} onKeyDown。

                      <div
                        key={item.key}
+                       role="button"
+                       tabIndex={0}
                        className={`nav-item${section === item.key ? ' active' : ''}`}
                        data-guide={`agent-editor-nav-${item.key}`}
                        data-section-key={item.key}
                        onClick={() => setSection(item.key)}
+                       onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSection(item.key); } }}
                      >


─── apps/web/src/agents/AgentEditorModal.tsx:784-784 ───
[bug · low] clipboard 写入的 Promise 被 void 丢弃：剪贴板权限被拒（或非安全上下文）时产生未处理的 promise rejection，用户也无任何反馈。建议补
catch（至少静默吞掉，理想是失败提示）。

-                   <Button theme="default" size="small" variant="text" className="agent-id-copy" onClick={() => { void navigator.clipboard?.writeText(editId); }}>
+                   <Button theme="default" size="small" variant="text" className="agent-id-copy" onClick={() => { void navigator.clipboard?.writeText(editId).catch(() => { void MessagePlugin.warning(t('common.copyFailed')); }); }}>


─── apps/web/src/agents/AgentEditorModal.tsx:599-600 ───
[maintainability · low] kbWarnTimer 成为死代码：回调体为空（MessagePlugin 自带超时管理），这个 4000ms
定时器不产生任何效果，配套的声明（282）、清除（597）、卸载清理（297）均无实际作用。建议连同 ref 与两处清理一并删除，减少无意义状态。

        void MessagePlugin.warning(t('agentEditor.agentType.kbIncompatibleWarn', { count }));
-       kbWarnTimer.current = window.setTimeout(() => { /* 4s 后自然消失（MessagePlugin 自身超时） */ }, 4000);
+       // MessagePlugin 自带超时：删除 kbWarnTimer ref 及其 set/clear/unmount 清理


─── apps/web/src/agents/AgentEditorModal.tsx:91-91 ───
[maintainability · low] 导入的 agents-u.css 仅含 2 行注释、没有任何规则（本文件使用的 wk-ae-* 样式实际全部在 agents.td.css，而该文件只被
AgentsPage 导入——本组件的样式链接是传递性的、较脆弱）。属死文件/死导入：要么删除文件与导入，要么把标题声明的「平移段」真正落进来。

- import './agents-u.css';
+ // agents-u.css 无任何规则：删除该文件与导入；若要消除传递依赖，
+ // 可在本文件直接 import './agents.td.css'（CSSModule 语义下重复导入无副作用）。


─── apps/web/src/agents/AgentEditorModal.tsx:1207-1207 ───
[security · low] go-settings 跳转链接使用 href="javascript:void(0)"（本文件两处：storage / sandbox
跳转）：javascript: URI 是 eslint no-script-url 的标准告警项，且在配置了 script-src 的严格 CSP 环境下会被直接拦截导致点击无响应。onClick
内已有 preventDefault，href 实际不承担导航语义，建议改用 <button type="button">（配 .go-settings-link 样式）或至少 href="#"。



─── apps/web/src/agents/AgentEditorModal.tsx:887-888 ───
[style · low] 静态内联样式散布多处：position:'relative'（本段 prompts 各 Row
×3）、width:'280px'/'240px'/'160px'（storage/provider/ocr 等 Select、InputNumber）、flexDirection:'column',
alignItems:'flex-end'（storage provider 行）——均为固定值，不属动态样式，违反「避免内联 style（仅动态值例外）」约定，且每处都伴随一次 as
CSSProperties 强转。建议下沉为 agents.td.css 类（如 .setting-control-full--anchor / .wk-ae-sel--w240
等），一处定义多处复用。



─── apps/web/src/agents/AgentEditorModal.tsx:1822-1826 ───
[bug · low] webSearch provider 选项丢失默认标记：迁移前每个选项渲染 provider.is_default ? ' · ' + t('common.default')
: ''，迁移后 Option 只剩 provider.name——用户无法在下拉中识别哪个是默认搜索引擎，属相对迁移前 React 基线的信息回退。建议在 Option 内容（及
label，供选中态展示）恢复该标记。

-                 {deps.providers.map((provider) => (
-                   <Select.Option key={provider.id} value={provider.id} label={provider.name} title={provider.name}>
-                     <span>{provider.name}</span>
+                 {deps.providers.map((provider) => {
+                   const label = provider.is_default ? `${provider.name} · ${t('common.default')}` : provider.name;
+                   return (
+                     <Select.Option key={provider.id} value={provider.id} label={label} title={label}>
+                       <span>{label}</span>
-                   </Select.Option>
+                     </Select.Option>
-                 ))}
+                   );
+                 })}


─── apps/web/src/agents/AgentParserRules.tsx:177-177 ───
[maintainability · low] go-settings 链接没有 href 属性：无 href 的 <a> 不可 Tab 聚焦、无键盘激活，读屏也不会播报为链接；onClose
面对键盘用户完全不可达。旧实现是 <button type="button">。建议恢复 button（样式钩子 .go-settings 保留），或补 href="#" +
role/tabIndex/键盘处理。

-                       <a className="go-settings" data-parser-go-config onClick={(event) => { event.preventDefault(); navigate('/platform/settings?section=parser'); }}>
+                       <button type="button" className="go-settings" data-parser-go-config onClick={() => navigate('/platform/settings?section=parser')}>


─── apps/web/src/agents/AgentsPage.tsx:1006-1011 ───
[bug · high] toggleFavorite 失败回滚存在竞态：回滚通过 toggleFavoriteId 对「当前 favoritesRef」再取反，而不是按本操作的
wasFavorited 定向撤销。同一星标快速双击且两次请求都失败时（先 remove 失败、后 add 失败），两次回滚的净效果是 {id}（已收藏），而 DB
实际未收藏；顺序相反则结果又不同——UI 与 DB 状态发散。建议回滚时按操作前语义确定性恢复：wasFavorited 为 true 则 add(id)、否则 delete(id)，并对同一 id
的 in-flight 操作做串行化。

      void persist.catch(() => {
-       const rollback = new Set(toggleFavoriteId([...favoritesRef.current], id));
+       // 按操作前语义定向撤销，避免连续失败时两次回滚互相抵消/叠加
+       const rollback = new Set(favoritesRef.current);
+       if (wasFavorited) rollback.add(id); else rollback.delete(id);
        favoritesRef.current = rollback;
        writeFavoriteIds(window.localStorage, viewer.userId, tenantKey, [...rollback]);
        setFavorites(rollback);
      });


─── apps/web/src/agents/AgentsPage.tsx:926-929 ───
[bug · medium] 收藏水合存在覆盖竞态：用户在组件挂载早期点击星标（乐观更新 + DB add）后，favoritesApi.list() 才 resolve
时会无条件用服务端快照覆盖本地 favoritesRef/setFavorites，乐观切换被静默回滚，UI 与 DB 不一致。此外 .catch(() => {}) 完全吞错（对比 hydrate
viewer 失败有注释说明，这里失败用户无感知且无注释说明策略）。建议用 in-flight 计数（pendingMutations > 0 时丢弃过期快照）或操作序号比对后再落地。

            const ids = rows.map((row) => row.resource_id).filter(Boolean);
+           if (pendingMutations.current > 0) return; // 存在未落库的乐观切换时丢弃过期服务端快照
            favoritesRef.current = new Set(ids);
            setFavorites(new Set(ids));
            writeFavoriteIds(window.localStorage, userId, tenantKey, ids);


─── apps/web/src/agents/AgentsPage.tsx:645-646 ───
[bug · medium] error/notice 成为死属性链：AgentsPageViewProps 仍声明 error/notice（645-646），页面侧仍维护
loadError（946 赋值）并传 error={loadError}/notice={null}（1178-1179），但视图层既未解构也未渲染任何一个——列表加载失败时 loading
停止后用户只看到空态，错误信息被静默吞掉（迁移前有 Status 提示，属错误显示回归）。建议在视图中恢复错误提示（如 MessagePlugin.error 或页面内 banner），或删除
props 声明与传参。

-   error: string | null;
-   notice: string | null;
+   // error/notice 已无渲染点：要么恢复视图内错误提示，要么连同页面侧
+   // error={loadError} / notice={null} 传参一并移除，避免加载失败静默无提示。


─── apps/web/src/agents/AgentsPage.tsx:447-449 ───
[maintainability · medium] 可访问性回退：原 <button type="button"> 交互元素被替换为 div/span + onClick，全部缺少
role/tabIndex/键盘事件——本处 popup-menu-item（菜单项，role=menuitem 缺失），同文件还有 icon-item-labeled（303/316，div
替代按钮）、AgentDeleteDialog 的 circle-btn-txt（619-620，确认/取消按钮用
span，无键盘可达且删除确认是不可逆操作）。键盘用户与读屏用户无法操作，menu/dialog 的 aria 语义也丢失。建议恢复 button（CSS 已有样式钩子，可保持视觉），或统一补
role + tabIndex={0} + onKeyDown(Enter/Space)。

                    <div
                      key={action}
+                     role="menuitem"
+                     tabIndex={0}
+                     onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); onMenuAction(action, agent); } }}
                      className={`popup-menu-item${action === 'delete' ? ' delete' : ''}`}


─── apps/web/src/agents/AgentsPage.tsx:255-259 ───
[style · low] 嵌套三元违反项目编码规则（禁止嵌套三元）：ResourceOriginBadge 的 text 为 4 层嵌套（255-258），AgentCard 的 modeClass
为 2 层（397）。建议改为映射对象或提前返回的辅助函数。

-   const text = variant === 'mine' ? formatMessage('zh-CN', 'resourceOrigin.mine')
-     : variant === 'creator' ? (creatorName || formatMessage('zh-CN', 'resourceOrigin.tenant'))
-       : variant === 'space' ? (tenantName || formatMessage('zh-CN', 'resourceOrigin.space'))
-         : variant === 'shared' ? (tenantName || formatMessage('zh-CN', 'resourceOrigin.shared'))
-           : (tenantName || formatMessage('zh-CN', 'resourceOrigin.tenant'));
+   const fallbackText: Record<ResourceOriginBadge['variant'], () => string> = {
+     mine: () => formatMessage('zh-CN', 'resourceOrigin.mine'),
+     creator: () => creatorName || formatMessage('zh-CN', 'resourceOrigin.tenant'),
+     space: () => tenantName || formatMessage('zh-CN', 'resourceOrigin.space'),
+     shared: () => tenantName || formatMessage('zh-CN', 'resourceOrigin.shared'),
+     tenant: () => tenantName || formatMessage('zh-CN', 'resourceOrigin.tenant'),
+   };
+   const text = fallbackText[variant]();


─── apps/web/src/agents/AgentsPage.tsx:887-888 ───
[maintainability · low] 渲染阶段执行 favoritesRef.current = favorites 属于渲染期副作用（React 官方劝阻模式），且冗余：所有
setFavorites 写入点（921/928-929/1002-1005/1008-1011）都已同步维护 ref。删除该行，保持「变更点写 ref」单一模式即可，避免两种写入来源。

    const favoritesRef = useRef<ReadonlySet<string>>(favorites);
-   favoritesRef.current = favorites;
+   // 各 setFavorites 调用点已同步维护 ref，渲染期无需再写（避免渲染期副作用）


─── apps/web/src/agents/AgentsPage.tsx:255-259 ───
[bug · medium] ResourceOriginBadge 引用的 resourceOrigin.* 文案键在 packages/i18n
中不存在（已全文检索确认），formatMessage 缺键时会按 locale→en-US→原始 key 的回退链直接渲染字面字符串——variant='creator' 且 creatorName
为空等兜底场景下，徽标会显示 "resourceOrigin.tenant" 这类原始键名。同时 formatMessage 硬编码 'zh-CN'，完全忽略当前 locale。同库
KnowledgeBasesPage.tsx:243-260 已记录同一 gap 并改用本地 ORIGIN_TEXT 字面值映射 + locale 入参（注释明确点名 "agents
同题"），本组件未跟进。建议照搬该方案：本地 Record<variant, Partial<Record<Locale,string>>> 映射并传入当前 locale。

-   const text = variant === 'mine' ? formatMessage('zh-CN', 'resourceOrigin.mine')
-     : variant === 'creator' ? (creatorName || formatMessage('zh-CN', 'resourceOrigin.tenant'))
-       : variant === 'space' ? (tenantName || formatMessage('zh-CN', 'resourceOrigin.space'))
-         : variant === 'shared' ? (tenantName || formatMessage('zh-CN', 'resourceOrigin.shared'))
-           : (tenantName || formatMessage('zh-CN', 'resourceOrigin.tenant'));
+   /* resourceOrigin.* 键尚未进 packages/i18n（KnowledgeBasesPage 同题 gap）——
+      字面值直引 Vue zh-CN/en-US 文案，随 locale 切换。 */
+   const ORIGIN_TEXT: Record<'mine' | 'tenant' | 'space' | 'shared', Partial<Record<Locale, string>>> = { /* … */ };
+   const fallback = ORIGIN_TEXT[variant][locale] ?? ORIGIN_TEXT[variant]['zh-CN']!;
+   let text = fallback;
+   if (variant === 'creator' && creatorName) text = creatorName;
+   else if ((variant === 'space' || variant === 'shared') && tenantName) text = tenantName;


─── apps/web/src/agents/AgentsPage.tsx:540-540 ───
[bug · low] 关闭按钮 aria-label 使用 t('general.close')，但该键在全部语言包中的值均为「关闭设置 / Close Settings」（settings
域文案），语义与通用关闭按钮不匹配，读屏会播报错误含义。本文件与弹窗其余处（编辑器 close-btn、MbtiTestModal）均使用语义正确的
common.close（「关闭」），此处应统一改用 common.close。

-           <button type="button" className="shared-detail-drawer-close" aria-label={t('general.close')} onClick={onClose}>
+           <button type="button" className="shared-detail-drawer-close" aria-label={t('common.close')} onClick={onClose}>


─── apps/web/src/agents/AgentsPage.tsx:536-537 ───
[bug · low] 可访问性回退：迁移前的抽屉是 <aside role="dialog" aria-label={t('agent.detail.title')}>，迁移后
.shared-detail-drawer 丢失 role="dialog"/aria-modal/aria-label——读屏用户无法感知弹出的对话框及其名称，焦点语义也一并丢失。建议在
drawer 容器上补齐这三个属性（与 AgentEditorModal/MbtiTestModal 的做法一致）。

      <div className="shared-detail-drawer-overlay" onClick={(event) => { if (event.target === event.currentTarget) onClose(); }}>
-       <div className="shared-detail-drawer">
+       <div className="shared-detail-drawer" role="dialog" aria-modal="true" aria-label={t('agent.detail.title')}>


─── apps/web/src/agents/AgentsPage.tsx:771-780 ───
[bug · low] 仅图标按钮失去可访问名称：迁移前该按钮带 title + aria-label（= t('agent.createAgent')），迁移后只剩 Tooltip
视觉提示——Tooltip content 不会成为按钮的 accessible name，读屏用户只会听到「按钮」而不知其用途。建议在 Button 上恢复 aria-label（title 可由
Tooltip 承担）。

                    <Button
                      variant="text"
                      theme="default"
                      size="small"
                      className="header-action-btn"
                      data-guide="agent-list-create"
+                     aria-label={t('agent.createAgent')}
                      style={{ '--wails-draggable': 'no-drag' } as CSSProperties}
                      onClick={props.onCreate}
                      icon={<span className="btn-icon-wrapper"><SparklesIcon size={19} /></span>}
                    />


─── apps/web/src/agents/MbtiTestModal.tsx:124-124 ───
[style · low] 静态 color 内联样式模式：style={{ color: 'var(--td-error-color)' }}（本文件 2 处
loadFailed/submitFailed，同模式另见 PersonaSection.tsx 2 处、SubagentsSection.tsx 2
处）为固定值，不属动态样式。agents.td.css 已有同类 .field-error（错误色小字）样式，建议新增/复用一个 .wk-ae-error-text 类统一收口，替代 6 处重复的内联
style。

-           <p className="wk-ae-mbti-meta" role="alert" style={{ color: 'var(--td-error-color)' }}>{t('agentEditor.personalization.testLoadFailed')}</p>
+           <p className="wk-ae-mbti-meta wk-ae-error-text" role="alert">{t('agentEditor.personalization.testLoadFailed')}</p>


─── apps/web/src/agents/agents.td.css:861-868 ───
[maintainability · low] 「.agent-list-container .agent-section-header」连续出现两个规则块（本块设置
grid/display/pointer-events，紧随其后的同名块设置 sticky/背景/排版），是 less
嵌套展平的产物。同选择器分两段会误导后续维护（改样式时易漏看其中一段）。建议合并为一个规则块。

  .agent-list-container .agent-section-header {
    grid-column: 1 / -1;
    display: flex;
    align-items: center;
    gap: 6px;
    /* 整行只用来铺背景；点击靠子元素冒泡，避免点到标题右侧空白误折叠。 */
    pointer-events: none;
+   /* ↓ 与紧邻同名规则块合并，避免同选择器分裂两段 */
+   position: sticky;
+   top: 0;
+   z-index: 5;
+   /* …sticky/背景/排版属性… */
  }


─── apps/web/src/agents/agents.td.css:3952-3959 ───
[maintainability · low] .wk-ae-mbti-footer 与 .wk-ae-mbti-question 在本文件（§8 React-only 段）重复定义且声明冲突：首个
.wk-ae-mbti-question 声明 font-size: 15px，被文件末尾补充段的 .wk-ae-mbti-question { font-size: 13px; … }
无条件覆盖，15px 成为死值；.wk-ae-mbti-footer 也分两处定义（此处含 border-top + padding 12px 20px + flex-shrink，末尾段只重复
display/justify/gap）——且迁移前 Tailwind 字面值中 footer 并无 border-top，与该段「值 =
迁移前字面值」的收口意图相悖。同名选择器拆两段会误导后续维护（改样式易漏看另一段），建议合并为单一定义并核对生效值。



─── apps/web/src/agents/list.ts:427-427 ───
[maintainability · low] 死分支：进入该 return 时 TypeScript 已将 scope 收窄为 { kind: 'selected'; count: number
}，scope.kind === undefined 恒为 false，'': '' 分支永远不会执行。虽然本行不在本次 diff 内（本文件 diff 仅改注释），但属真实死代码，建议直接简化为
t('agent.shareScope.kbSelected', { count: scope.count })。

-   if (scope.kind === 'selected') return scope.kind === undefined ? '' : t('agent.shareScope.kbSelected', { count: scope.count });
+   if (scope.kind === 'selected') return t('agent.shareScope.kbSelected', { count: scope.count });


─── apps/web/src/analytics/AnalyticsPage.tsx:335-336 ───
[maintainability · low] wk-anl-1 / wk-anl-2 / wk-anl-3 是自增式占位类名，不表达意图（分别对应筛选 label
布局、表头行底边框、表头字重），与本文件既有约定不一致：其余样式均采用 wk-anl-an-* 描述性命名并经 AN_* 常量引用，这三处却以内联字符串硬编码在 JSX 中。后续重命名或全局检索（如查
'表头样式'）时极易遗漏这三处。建议改为描述性命名并收进 AN_* 常量，CSS 侧同步改名。

-           <label className="wk-anl-1">
-             {t(locale, 'analytics.rangeFrom')}
+ // 常量区新增：
+ const AN_FILTER_LABEL = 'wk-anl-an-filter-label';
+ const AN_USAGE_HEAD_ROW = 'wk-anl-an-usage-head-row';
+ const AN_USAGE_HEAD_CELL = 'wk-anl-an-usage-head-cell';
+ 
+ // JSX 引用处：
+ <label className={AN_FILTER_LABEL}>
+ <tr className={AN_USAGE_HEAD_ROW}>
+ <th className={AN_USAGE_CELL + ' ' + AN_USAGE_HEAD_CELL}>


─── apps/web/src/analytics/analytics-u.css:73-75 ───
[maintainability · medium] 原 Tailwind
语义令牌类（bg-accent/text-accent/bg-accent-wash/border-accent）在此被平移为裸色值 #07c05f /
rgba(7,192,95,0.08)（本文件共 7 处 #07c05f + 2 处 wash）。而 apps/web/src/styles.css 已定义 --color-accent:
#07c05f（:457）与 --color-accent-wash: rgba(7, 192, 95, 0.08)（:463），值完全对应；同批迁移的
documents-u.css、administration-u.css 也已采用 var(--wk-accent,#07c05f) / var(--color-accent, #07c05f)
兜底写法。硬编码会使品牌色/主题令牌调整时本页不跟随，且与兄弟文件口径不一致。建议统一改为 var(--color-accent, #07c05f)、var(--color-accent-wash,
rgba(7,192,95,0.08)) 形式（同文件 border/focus/hover 各处一并替换）。

    border-style: solid;
    border-width: 0;
-   background-color: #07c05f;
+   background-color: var(--color-accent, #07c05f);


─── apps/web/src/analytics/analytics-u.css:311-313 ───
[maintainability · low] .wk-anl-an-usage-num 在同一文件内被拆成两处定义（约 269 行的 text-align: right 与此处的
font-variant-numeric: tabular-nums），检索维护时易漏看后半段。建议把 font-variant-numeric 合并进主规则块，仅在文件头注释说明
tabular-nums 来源即可。

- /* T15 语义化：原 placeholder:text-[rgba(23,26,29,0.35)] / tabular-nums 旧栈 utility。 */
+ /* T15 语义化：原 placeholder:text-[rgba(23,26,29,0.35)] 旧栈 utility。 */
  .wk-anl-an-text-input::placeholder { color: rgba(23, 26, 29, 0.35); }
- .wk-anl-an-usage-num { font-variant-numeric: tabular-nums; }
+ /* .wk-anl-an-usage-num 的 tabular-nums 已合并至上方主规则块。 */


─── apps/web/src/analytics/analytics-u.css:114-118 ───
[bug · low] AN_BTN_OUTLINE 在 AnalyticsPage.tsx:443 被用于 disabled={exporting} 的"导出 CSV"按钮，但本样式（及原
Tailwind 串）未提供 :disabled 态，导出进行中按钮无任何视觉反馈；同文件的 .wk-anl-an-btn-primary 与 .wk-anl-an-btn-pager 均有
:disabled 规则。建议补齐以保持三个按钮类一致（原串缺失属忠实平移，但此处为本次新增代码，宜一并修正）。

  .wk-anl-an-btn-outline:hover {
    border-style: solid;
    border-color: #07c05f;
    background-color: rgba(7, 192, 95, 0.08);
+ }
+ 
+ .wk-anl-an-btn-outline:disabled {
+   cursor: not-allowed;
+   opacity: 0.55;
  }


─── apps/web/src/apps/AppsPages.tsx:80-80 ───
[maintainability · medium] 死代码：局部 Popconfirm 组件已无任何调用点。ConnectionsPage 的撤销确认已改用 TDesign 的
TPopconfirm（见 ConnectionsPage 列定义处），本文件中该组件为模块私有（未导出），全文件检索确认仅剩定义无使用。其内部的 wk-apps-1~4
定位气泡样式也随之下线，应连同组件一并删除，避免后续读者误以为自绘气泡仍在使用。



─── apps/web/src/apps/AppsPages.tsx:161-167 ───
[bug · medium] rowKey 丢失了旧实现的索引兜底。原代码为 `String(row.action_id ?? index)` / `String(row.id ??
index)`，而 appRows（model.ts）会把非对象条目整形为 `{}`，此时 action_id/id 为 undefined；tdesign-react Table 以 rowKey
字段值作为行 React key，缺失或重复时会产生 key 冲突，行更新（如撤销后刷新、轮询重载）可能出现渲染错乱。建议在 load 成功后对 rows 归一化（剔除/补全唯一 key 字段，或
map 追加 `__key: value ?? index` 后以其为 rowKey），三处 TTable（action_id / 两处 id）同此。



─── apps/web/src/apps/AppsPages.tsx:249-249 ───
[bug · low] 可访问性回归：迁移后 CatalogPage/ConnectionsPage 不再渲染原 PageFrame 中 `role="status"` 的
common.loading 文案，仅依赖 TTable 内部 loading 遮罩（无 aria-live/aria-busy
播报），屏幕阅读器失去加载开始/结束的提示。authorization/action 两页经 PageFrame 保留播报，同一域内行为不一致。建议在两页加载分支补充 sr-only 的 live
region（或对容器加 aria-busy={loading}）。



─── apps/web/src/apps/AppsPages.tsx:218-218 ───
[style · low] 新增列定义中存在链式嵌套三元（`row.state === 'active' ? 'success' : row.state === 'revoked' ?
'danger' : 'default'`；installationColumns 的 state label 亦为三层链式），违反项目"禁止嵌套三元"规则。文件内已有
riskThemeOf/riskLabelOf 的提取范式，建议同样提取 stateThemeOf(state)/stateLabelOf(state, t)。另 maxHeight 520/360
为字面 UI 度量，宜提为具名常量并注明来源（与台账 #14 注释呼应）。



─── apps/web/src/apps/AppsPages.tsx:62-63 ───
[maintainability · low] 死分支：ConnectionsPage 迁移到 TTag 后，本地 Tag 仅剩 AuthorizationPage（statusTheme）与
ActionPage（riskValue/stateTheme）调用，传入主题只可能是 success/danger/warning/default（'neutral' 经 fallthrough 走
default）。'primary' 分支已不可达，apps-u.css 的 .wk-apps-tag--primary 规则也随之成为无消费方死样式（可与已确认的 wk-apps-1~4
清理一并处理）。

-     : theme === 'primary' ? 'wk-apps-tag--primary'
      : 'wk-apps-tag--default';


─── apps/web/src/apps/AppsPages.tsx:227-227 ───
[maintainability · low] 无台账说明的对齐偏差：事实源 ConnectionsView.vue 的 t-popconfirm 未设置
placement（tdesign-vue-next 与 tdesign-react 默认均为 'top'），而本文件头注释声称 "ConnectionsView.vue DOM 1:1"。同文件中
maxHeight 偏差均附台账 #14 注释，此处确认气泡方位（top→left）却无任何说明，后续视觉对齐审查会将其误判为移植错误。建议去掉 placement 恢复默认，或补注偏差原因（如避免
ops 列靠右时气泡溢出视口）。

-             placement="left"
+             /* 若确需 left（避免最右列气泡溢出视口），请补充台账说明；否则删除以对齐 Vue 默认 top */


─── apps/web/src/apps/apps-u.css:5-8 ───
[maintainability · low] wk-apps-1~4 为死样式：全仓检索确认这四个类仅被 AppsPages.tsx 中已无调用点的局部 Popconfirm
组件引用（ConnectionsPage 已改用 TDesign TPopconfirm）。若删除该组件，应连同本组规则一并移除，避免与 wk-apps-2 气泡视觉相关的维护误导。



─── apps/web/src/apps/apps-u.css:193-198 ───
[maintainability · low] 与 apps.td.css 的 .apps-view 存在同源双拷贝且已出现漂移：此处 wk-apps-26 缺少后者的 box-sizing:
border-box 与 line-height: normal；wk-apps-5/6/7 与 .apps-view__header/__title/__desc 是同一套 Vue
度量的两份拷贝。留守的 authorization/action 两页将不跟随 .apps-view 的后续修正。另建议 wk-apps-11/16/17
的字面色（rgba(0,0,0,0.6)/rgba(23,26,29,0.4)/#f3f3f3）改用
var(--td-text-color-secondary)/var(--td-text-color-placeholder)/var(--td-bg-color-secondarycontainer
)（tdesign-theme.css 已接线），使留守页在主题切换时与迁移页保持一致。



─── apps/web/src/auth/JoinPage.tsx:8-8 ───
[documentation · low] 导入顺序与 auth-u.css 头注释声明的契约相反：auth-u.css 头注写明"导入顺序：需在本域
.td.css（login.td.css）之前——冲突时以 .td.css 为准"，但本文件中 `import { LoginPage }`（携带 login.td.css，第 5 行）先于
`import './auth-u.css'` 执行，在 JoinPage 携带 token 渲染 LoginPage 的路由上 login.td.css 实际先注入。当前
wk-join-*/wk-onb-* 与 login.td.css 选择器无交集、无功能影响，但声明的不变量出生即被违反，后续若任一侧新增冲突规则将按相反方向层叠。建议把本行上移到 LoginPage
导入之前（或将 auth-u.css 头注改为说明"当前无选择器交集、顺序无约束"）。

+ // 需先于 LoginPage（login.td.css）注入：auth-u.css 头注声明的层叠契约
  import './auth-u.css';
+ import { LoginPage } from './LoginPage.tsx';


─── apps/web/src/auth/LoginPage.tsx:441-441 ───
[bug · medium] 换装后登录密码框丢失了"明文/密文切换"能力：旧实现有 eye/eye-off 切换按钮（且被删除的旧注释明确描述 Vue 端也有此交互），而 tdesign Input
type="password" 无内置切换。若新版 Vue 事实源已确认去掉该交互请忽略；否则建议用 Input 的 suffix 插槽补回切换按钮，避免主登录表单可用性回退。

-                 <Input placeholder={t('auth.passwordPlaceholder')} type="password" autocomplete="current-password" size="large" disabled={loading} onEnter={() => { void submit(); }} {...ariaFor('password')} />
+                 <Input placeholder={t('auth.passwordPlaceholder')} type={showLoginPassword ? 'text' : 'password'} autocomplete="current-password" size="large" disabled={loading} onEnter={() => { void submit(); }} suffix={<span role="button" aria-label={t('auth.password')} aria-pressed={showLoginPassword} onClick={() => setShowLoginPassword((v) => !v)}>…eye/eye-off icon…</span>} {...ariaFor('password')} />


─── apps/web/src/auth/LoginPage.tsx:413-413 ───
[bug · low] 轮播分页 bullet 从旧的 <button type="button" aria-label> 换成了 <span onClick>，键盘用户无法 Tab 聚焦、Enter
无法切换。swiper 原生 bullet 也带 tabindex="0" role="button"。建议补上 tabIndex/role 与 onKeyDown，与同文件语言菜单项（已带
tabIndex+onKeyDown）保持一致。

-                 <span key={slide.titleKey} className={`swiper-pagination-bullet${index === slideIndex ? ' swiper-pagination-bullet-active' : ''}`} aria-label={t(slide.titleKey)} title={t(slide.titleKey)} onClick={() => setSlideIndex(index)} />
+                 <span key={slide.titleKey} role="button" tabIndex={0} className={`swiper-pagination-bullet${index === slideIndex ? ' swiper-pagination-bullet-active' : ''}`} aria-label={t(slide.titleKey)} title={t(slide.titleKey)} onClick={() => setSlideIndex(index)} onKeyDown={(event) => { if (event.key === 'Enter') setSlideIndex(index); }} />


─── apps/web/src/auth/LoginPage.tsx:346-346 ───
[maintainability · low] 节点/连线的 animation-delay 现在双源维护：此处 inline style（AUTH_NODE_DELAYS /
AUTH_LINE_DELAYS）与 login.td.css §1 的 .node-1..12 / .line-1..12 各写一份（当前取值一致，inline 覆盖
CSS）。两处任一后续改动都会静默漂移，建议只保留一处（如删去 inline style，或删去 CSS 中的 animation-delay 声明）。

-         <div key={index} className={`knowledge-node node-${index + 1}`} style={{ animationDelay: AUTH_NODE_DELAYS[index] }}>
+         <div key={index} className={`knowledge-node node-${index + 1}`}>  {/* delay 事实源在 login.td.css .node-N */}


─── apps/web/src/auth/LoginPage.tsx:435-436 ───
[bug · medium] 注册成功后邮箱预填丢失，且可见输入与提交值不一致。Vue 事实源 Login.vue:744-746 在自助注册成功后 `formData.email =
registerData.email`（v-model 绑定，切回登录卡时输入框显示该邮箱），本文件 submit() 成功分支也刻意不清空 email state（注释 "prefill the
email"）。但登录卡 email FormItem 是非受控的 `initialData=""`——切卡时登录 Form 整体重挂载，initialData 固定为空串，输入框显示为空，而
React state 中的 email 仍持有注册邮箱：用户看到的邮箱框是空的，只补密码直接提交时会用这个不可见的旧邮箱发起登录（可见输入 ≠ 实际提交值）。该路径也无测试覆盖。建议将登录卡
email 的 initialData 绑到 state（卡片切换必然重挂载，挂载时会取到保留的 email）：

-               <Form.FormItem label={t('auth.email')} name="email" requiredMark initialData="">
+               <Form.FormItem label={t('auth.email')} name="email" requiredMark initialData={email}>
                  <Input placeholder={t('auth.emailPlaceholder')} type="text" autocomplete="email" size="large" disabled={loading} {...ariaFor('email')} />


─── apps/web/src/auth/WorkspaceOnboardingPage.tsx:175-175 ───
[bug · low] 邀请列表弹窗换 WkDialog 后丢掉了原 Dialog 的 closeLabel={msg(locale,
'auth.workspaceOnboarding.close')}。WkDialog 支持 closeLabel 且默认值为英文 'Close'，非英文 locale 下右上角关闭按钮的
aria-label 将退化为英文。建议补回。

+       closeLabel={msg(locale, 'auth.workspaceOnboarding.close')}
        className="wk-onb-11"


─── apps/web/src/auth/login.td.css:43-47 ───
[style · low] .swiper-slide 中 transition-property 声明了两次：第一处 `transition-property: transform` 被下方的
`transition-property: transform, opacity` 覆盖，是死声明，易误导后续维护。建议删除第一处。

    position: relative;
-   transition-property: transform;
    display: block;
    /* effect-fade crossFade（speed 800）：稳态下 opacity 由 inline style 决定 */
    transition-property: transform, opacity;


─── apps/web/src/auth/login.td.css:193-201 ───
[maintainability · low] 文件中存在两个 @media (prefers-reduced-motion: reduce) 块：本块（node 静态展示 + opacity
0.65）会被 §2 末尾那块（animation/transition: none !important + .animated-bg { display: none
}）整体覆盖而完全失效。双块并存易让维护者误以为本块在生效。若两块均系 Vue 原样平移，建议在本块加注释注明被末块覆盖（与 .t-form-item 处理方式一致）；否则合并删除其一。



─── apps/web/src/auth/login.td.css:675-678 ───
[style · low] .t-form-item__control 与上文已注释说明的 .t-form-item__label 同属 Vue 源码笔误选择器（tdesign 实际类名为
.t-form__controls / .t-form__control），本规则永不命中。label 那对规则有注释交代，这条没有——建议同样加注说明"原样平移、不命中"或直接删除，避免误导。



─── apps/web/src/auth/login.td.css:223-227 ───
[maintainability · low] .showcase-subtitle 中首行 `margin-top: 0` 被 4 行之后的 `margin: 0 0 8px 0`
简写整体覆盖，是死声明（Vue 源 Login.vue:1090/:1093 同样存在，属原样平移）。与本文件已确认的 .t-form-item / 双 prefers-reduced-motion
块同类问题——建议删除该行，或按既有惯例加注释说明"Vue 事实源原样平移、不生效"，避免误导后续维护者以为 margin-top 有独立作用。

  .login-layout .showcase-subtitle {
-   margin-top: 0;
+   /* margin-top: 0 为 Vue 源死声明（被下方 margin 简写覆盖），原样平移不生效 */
    font-size: 22px;
    color: rgba(255, 255, 255, 0.95);
    margin: 0 0 8px 0;


─── apps/web/src/career/ApplicationPage.tsx:232-232 ───
[bug · low] reconcile 的 finally 以 `if (scopeController.isCurrent(...))` 为条件复位
reconcileBusy：对账请求因空间切换/登出被 abort 时，isCurrent 为 false，setReconcileBusy(false) 永不执行；而
clearPrivate（abort 监听触发）重置了 attempt/receipt/phase/message 等状态却不包含
reconcileBusy。组件保持挂载的情况下（ApplicationPage 的 key 只随 opportunity/snapshot 变化），用户在新空间重新创建或恢复出 linkState
为 linking/link_failed 的申请后，「用原请求编号对账/重试对账」按钮将永久处于 disabled。建议 finally 无条件复位，并在 clearPrivate 中一并重置。

-   } finally { if (scopeController.isCurrent(requestScope.scope)) setReconcileBusy(false) }
+   } finally { setReconcileBusy(false) }
+ // 同时在 clearPrivate 中补一行 setReconcileBusy(false)，与其它私有状态一并清理


─── apps/web/src/career/ApplicationPage.tsx:248-248 ───
[style · low] 嵌套三元违反本仓库「禁止嵌套三元表达式」规则且可读性差，本处为三层（loading/error/有值/空串）；同款模式还出现在
MaterialPage（success、statusText 两处三层链）与 SubmissionPage（readState、exportsError/exports
分支等）。建议提取为映射函数或提前返回的小函数，各页统一处理。

-    <p className="wk-application__revision" role="status">{revisionState === 'loading' ? '正在读取当前档案修订…' : revisionState === 'error' ? '暂时无法读取当前档案修订，可稍后重试；申请创建会被暂缓。' : revision !== undefined ? `当前档案修订 ${revision}（申请将按此修订固定）` : ''}</p>
+ function revisionNotice(state: 'loading' | 'ready' | 'error', revision: number | undefined): string {
+  if (state === 'loading') return '正在读取当前档案修订…'
+  if (state === 'error') return '暂时无法读取当前档案修订，可稍后重试；申请创建会被暂缓。'
+  return revision !== undefined ? `当前档案修订 ${revision}（申请将按此修订固定）` : ''
+ }
+ // 渲染处：<p className="wk-application__revision" role="status">{revisionNotice(revisionState, revision)}</p>


─── apps/web/src/career/ApplicationPage.tsx:122-126 ───
[other · low] 刷新恢复时若 URL 中的申请与当前岗位/快照不匹配，仅静默 replaceState 剥离 application 参数后直接
return——用户刷新后会看到申请面板凭空消失且无任何解释（页面其余状态仍是 idle）。剥离参数防止错误关联是对的，但同时应给出可见提示，避免用户误以为申请丢失。

     if (next.pinnedEvidence.opportunityId !== opportunityId || next.pinnedEvidence.snapshotId !== snapshotId) {
      const url = applicationParamUrl(undefined)
      if (url) window.history.replaceState({}, document.title, url)
+     setPhase('error'); setMessage('链接中的申请不属于当前岗位快照，已移除该申请参数；请从对应岗位卡重新进入。')
      return
     }


─── apps/web/src/career/ApplicationPage.tsx:234-238 ───
[bug · low] startAnotherBatch 复位了
attempt/receipt/phase/message/revisionConflict/acknowledged/batchIdentity，但未复位
progressOpen/submissionOpen/preparationOpen（以及
careerMaterialId）。若用户在展开「申请进展时间线/投递确认/面试准备」的状态下点击此按钮，为其他批次创建新申请后，三个面板会随新 receipt
立即以展开状态渲染——与各处注释声明的「入口默认收起，创建流程不受影响」相悖（面板虽按 applicationId 重挂载，但开关状态残留）。建议一并复位三个开关。

   const startAnotherBatch = (): void => {
    setAttempt(undefined); setReceipt(undefined); setPhase('idle'); setMessage(''); setRevisionConflict(undefined); setAcknowledged(false); setBatchIdentity('')
+   setProgressOpen(false); setSubmissionOpen(false); setPreparationOpen(false); setCareerMaterialId(undefined)
    const url = applicationParamUrl(undefined)
    if (url) window.history.replaceState({}, document.title, url)
   }


─── apps/web/src/career/CareerPage.tsx:12-12 ───
[maintainability · low] makeId 与同目录 protocol.ts 导出的 newRequestId 逐字符相同，属于重复定义。Career
家族页面对「请求编号生成」应统一走 protocol.ts，避免 protocol.ts 后续调整生成策略（如增加长度约束）时此处被遗漏。

- const makeId = (): string => typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`
+ import { newRequestId } from './protocol.ts'
+ // 删除本地 makeId，所有 makeId() 调用点改为 newRequestId()


─── apps/web/src/career/CareerPage.tsx:209-209 ───
[style · low] 全文件使用静态内联 style 对象布局（网格、边框、间距、颜色均硬编码在 JSX 中），违反「除动态样式外避免内联 style」规范：每次渲染重建样式对象，且与同目录
OpportunityPage/SearchPage 的 CSS 类名体系（opportunity.css/search.css）不一致，后续主题化（TDesign 变量）无法覆盖这些硬编码色值（如
#e7e7e7、#666、#087a55）。建议按同目录页面的既有模式抽取为 CSS 类文件。

- {view.facts.length ? <dl style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 12 }}>{view.facts.map((fact) => <div key={`${fact.key}:${fact.revision}`} style={{ border: '1px solid #e7e7e7', borderRadius: 8, padding: 14 }}><dt style={{ color: '#666' }}>{fieldLabel[fact.key] || fact.key}</dt><dd style={{ margin: '6px 0', fontWeight: 600 }}>{fact.value}</dd><small>{sourceText(fact.source)} · {fact.confirmation.confirmedAt}</small></div>)}</dl> : <p>还没有确认事实。可以先提交档案提案，或直接确认本人提供的信息。</p>}
+ <dl className="wk-career-profile__facts">
+ 
+ /* career.css */
+ .wk-career-profile__facts {
+   display: grid;
+   grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
+   gap: 12px;
+ }


─── apps/web/src/career/CareerPage.tsx:179-179 ───
[style · low] 错误标题使用 3 层嵌套三元链（forbidden → revision_conflict → outcome_unknown →
默认），违反「禁止嵌套三元」规范；同文件来源状态标签 `source.status === 'processing' ? '处理中' : source.status === 'ready' ?
'已解析' : '失败'` 也是同类结构。错误码→标题是典型映射表场景，与 OpportunityPage 已确认问题的修复方向保持一致，改为 Record 查表即可。

-   {error ? <Card bordered style={{ marginBottom: 16 }}><div role="alert"><strong>{error.code === 'forbidden' ? '当前空间不可访问' : error.code === 'revision_conflict' ? '档案已更新' : error.code === 'outcome_unknown' ? '提交结果暂时未知' : '暂时无法打开档案'}</strong><p>{error.text}{error.currentRevision !== undefined ? `（当前修订 ${error.currentRevision}）` : ''}</p></div><Button variant="outline" onClick={() => void load()}>重新读取</Button></Card> : null}
+ const errorTitles: Record<string, string> = { forbidden: '当前空间不可访问', revision_conflict: '档案已更新', outcome_unknown: '提交结果暂时未知' }
+ // ...
+ <strong>{errorTitles[error.code ?? ''] ?? '暂时无法打开档案'}</strong>


─── apps/web/src/career/ExportDeletionPage.tsx:118-119 ───
[bug · medium] verifyOldGrants 的 forbidden 分支调用 clearPrivate，会把刚展示的删除完成回执（deletion
receipt、保留范围披露、完成时间、verifyMessage）整体清空并进入无恢复按钮的 forbidden 态。deletedAnnounced
已置位不会重播，用户可能在删除成功的瞬间丢失页面声称'绝不隐瞒'的完成凭证。删除后的验证属于附带动作，其失败不应清掉删除结果本身——建议 forbidden 只记入
verifyMessage，把回执保留在页面上。

-    if (parsed.code === 'forbidden') { clearPrivate('当前空间不可访问导出与删除。'); return }
+    if (parsed.code === 'forbidden') { setVerifyMessage('删除后验证暂时无法完成：当前空间不可访问。删除回执已在上方保留，请刷新核对删除结果。'); return }
     setVerifyMessage(parsed.code === 'not_found' ? `删除后验证：导出回执 ${exportRequestId} 已不可读取（旧授权与旧入口已失效）。` : `删除后验证暂时无法完成：${parsed.message}`)


─── apps/web/src/career/ExportDeletionPage.tsx:30-30 ───
[maintainability · medium] snapshotCount 在整份文件中没有任何调用点（边界清单已改为整包统计
snapshotTotal/eventTotal/versionTotal），属死代码；它的存在还暗示边界分段计数可能本应被渲染。建议删除，或补齐其应有的展示用途。



─── apps/web/src/career/ExportDeletionPage.tsx:10-11 ───
[maintainability · low] endpointDefiniteCodes=['revision_conflict'] 与 protocol.ts isUncertainWrite
的确定性失败码列表（第 37 行已包含 revision_conflict）完全重叠，这个包装函数与 baseIsUncertainWrite
行为逐输入等价，'端点特定叠加'实际为零效果，反而误导读者以为存在端点差异。建议直接复用 baseIsUncertainWrite，或在此真正补充本端点特有而基础契约未覆盖的确定码。

- const endpointDefiniteCodes: readonly string[] = ['revision_conflict']
- const isUncertainWrite = (cause: unknown): boolean => endpointDefiniteCodes.includes(errorDetails(cause).code ?? '') ? false : baseIsUncertainWrite(cause)
+ const isUncertainWrite = baseIsUncertainWrite


─── apps/web/src/career/ExportDeletionPage.tsx:193-194 ───
[bug · low] downloadExport 在 anchor.click() 后同步 revokeObjectURL，Safari 及部分旧浏览器会在下载开始前回收 Blob URL
导致下载中断。同目录 MaterialPage.tsx（第 58 行）已采用 setTimeout(…, 5_000) 延迟回收的实践，建议对齐。

    anchor.click()
-   URL.revokeObjectURL(url)
+   setTimeout(() => URL.revokeObjectURL(url), 5_000)


─── apps/web/src/career/ExportDeletionPage.tsx:339-339 ───
[bug · low] revisionState==='error' 时提示'可稍后重试'，但页面没有任何重读修订的入口：重读按钮只在导出/删除 revision_conflict
态出现，'查看删除边界'也不刷新修订，导出与删除按钮因 revision===undefined 长期禁用，形成操作死胡同（只能整页刷新）。另外该行本身是 loading→error→ready
的三连嵌套三元，违反'禁止嵌套三元'约定，拆分时可一并解决。

-    <p className="wk-lifecycle__revision" role="status">{revisionState === 'loading' ? '正在读取当前档案修订…' : revisionState === 'error' ? '暂时无法读取当前档案修订，可稍后重试；导出与删除会被暂缓。' : revision !== undefined ? `当前档案修订 ${revision}（导出与删除将按此修订提交）` : ''}</p>
+    <p className="wk-lifecycle__revision" role="status">{revisionState === 'loading' ? '正在读取当前档案修订…' : revisionState === 'error' ? '暂时无法读取当前档案修订；导出与删除会被暂缓。' : revision !== undefined ? `当前档案修订 ${revision}（导出与删除将按此修订提交）` : ''}</p>
+    {revisionState === 'error' ? <div className="wk-lifecycle__actions"><button type="button" onClick={() => void readRevision()}>重新读取档案修订</button></div> : null}


─── apps/web/src/career/ExportDeletionPage.tsx:385-385 ───
[style · low] 删除回执 className 使用 deleted→partial→默认的链式嵌套三元，违反'嵌套三元表达式不允许'的清单约定。文件顶部已有
deletionStatusLabels 映射的先例，此处同样可改为按 status 映射 tone 类名，顺带消除重复的 'wk-lifecycle__receipt' 前缀。

-     {deletion ? <section className={deletion.status === 'deleted' ? 'wk-lifecycle__receipt wk-lifecycle__receipt--deleted' : deletion.status === 'partial' ? 'wk-lifecycle__receipt wk-lifecycle__receipt--partial' : 'wk-lifecycle__receipt'} aria-label="删除结果">
+ const receiptTone: Record<CareerDeletionStatus, string> = { deleting: '', partial: ' wk-lifecycle__receipt--partial', deleted: ' wk-lifecycle__receipt--deleted' }
+ // ...
+     {deletion ? <section className={`wk-lifecycle__receipt${receiptTone[deletion.status]}`} aria-label="删除结果">


─── apps/web/src/career/ExportDeletionPage.tsx:195-195 ───
[bug · medium] downloadExport 的两条反馈消息（下载成功、以及 URL.createObjectURL 不可用时的降级提示）永远不会渲染：exported 只在
acceptExport 中被设置，而 acceptExport 已把 exportPhase 置为 'idle'，渲染条件却是 `exportMessage && exportPhase !==
'idle'`。用户点击「下载导出包」后得不到任何页面内确认；在无 Blob URL 支持的环境里降级提示同样被吞掉。acceptExport 中的
`setExportMessage('导出包已生成：…')` 属同一死写入。建议：要么在 idle 态也渲染 exportMessage（例如拆出独立的 feedback
状态），要么删掉这些永不显示的 setExportMessage。

-   setExportMessage(`导出包已下载（career-export-${exported.exportId}.json）。摘要 ${exported.digest}。`)
+ const downloadExport = (): void => {
+   if (!exported) return
+   // …
+   setExportFeedback(`导出包已下载（career-export-${exported.exportId}.json）。摘要 ${exported.digest}。`)
+  }


─── apps/web/src/career/ExportDeletionPage.tsx:247-247 ───
[maintainability · low] 死代码：deleted 分支里 setDeletionMessage('空间已完全删除…') 之后紧跟
setDeletionPhase('idle')，而消息段落只在 `deletionPhase !== 'idle'` 时渲染——该消息永远不会显示（后续任何非 idle
转换都会先覆写它）。完成信息实际由下方回执区承担，这行写入只会误导读者以为存在完成提示。建议删除该 setDeletionMessage，或改为独立于 phase 的完成提示状态。



─── apps/web/src/career/ExportDeletionPage.tsx:256-257 ───
[bug · low] status === 'deleting'（删除进行中，服务端已确认的正常中间态）被置入 deletionPhase 'error'：消息会以
wk-lifecycle__message--error 红色错误样式、role="alert" 被 screen reader
当作错误警报播报。「仍在进行中」不是错误，用错误语义呈现会误导用户以为删除已失败（页面红线恰恰是『绝不声称失败/成功以外的歧义』）。建议为 deleting 引入中性展示态（或至少按 status
区分消息样式与 role），避免复用 error 相位。



─── apps/web/src/career/ExportDeletionPage.tsx:298-299 ───
[bug · medium] 未知写结局的请求编号只存在于内存（exportAttempt / deletionAttempt / lastExportRequest
均未持久化），而回执查询接口（GET /deletions/receipt?requestId=…）必须凭原 requestId 才能访问。一旦用户在 unknown
态刷新或离开页面，原编号永久丢失、恢复入口消失，随后「发起导出/完整删除」会静默改用新请求编号——与页面承诺『请先用原请求编号查询回执，或用同一编号重试；不会自动更换请求编号』直接矛盾。对删除这种不可
逆操作，进行中删除的回执将无法再被核验。建议参照 RulePage 的 ruleIdKey 做法，把未决 attempt（含 requestId）按 userId/tenantId 写入
sessionStorage/localStorage，恢复时优先回放；小程序侧 services/career.ts 已有 pending intent 持久化先例可对齐。



─── apps/web/src/career/ExportDeletionPage.tsx:348-348 ───
[bug · low] revision_conflict 后点击「重新读取档案修订」：exportConflict 被清、revision 会刷新，但 exportPhase 仍停留
'error'、exportMessage 仍是『请重新读取档案修订后再发起导出』——补救动作已完成，红色 role="alert" 错误块却无限期残留，与已恢复的状态自相矛盾（删除侧
deletionConflict 的同名按钮存在同样问题）。建议在 readRevision 成功回调里，若 exportConflict/deletionConflict
相关错误是当前显示的消息则一并清空（置 phase 'idle' 或更新为『修订已更新，可重新发起』）。



─── apps/web/src/career/InboxPage.tsx:78-78 ───
[bug · high] unknown 写恢复所依赖的 attempt/requestId 仅保存在组件 state，未做任何持久化：用户刷新或路由切换后原请求编号即丢失，而 UI
明确承诺「原请求编号已保留，可稍后重试查询」「不会自动更换请求编号」（第 205/230 行文案）。这与本次门禁重点「unknown request-ID
持久化」直接不符——一旦丢失，用户只能换新编号提交，恢复闭环断裂。career 家族已有 per-scope localStorage 先例（RulePage 的
weknora:career:rule-id:{userId}:{tenantId}），建议将 unknown 写的 { requestId, 冻结请求体, scope 标识 }
持久化（sessionStorage 即可），挂载时读回并校验 scope 一致才恢复，验收成功或确定性失败后清除。

   const [attempt, setAttempt] = useState<WriteAttempt>()
+  // 建议：unknown 时持久化到 per-scope 存储，例如
+  // sessionStorage.setItem(`weknora:career:pending-write:${scope.userId}:${scope.tenantId}`, JSON.stringify(attempt))
+  // 挂载时读回校验 scope 后恢复；acceptXxxReceipt / 确定性失败时移除。


─── apps/web/src/career/InboxPage.tsx:182-182 ───
[bug · medium] 订阅分支重试时以现取的 view?.revision 重建请求体，而 reminder 分支的 input 在 attempt 中冻结——同一 requestId
携带不同 expectedRevision 重放违反幂等原则。后端 office.go confirm 的幂等指纹包含 rev（[]any{"confirm", pid, k, v, rev,
…}），指纹不匹配必返回 409 idempotency_conflict。典型触发场景：unknown 写实际已落库（revision 已前进）→ 用户点「刷新待办」→ readView 取回新
revision → 点「用原请求编号重试」→ 携带新 expectedRevision → 被判 idempotency_conflict，恢复通道在它设计应对的场景里反而被堵死，且 UI
只能提示换新编号。建议在 attempt 创建时冻结 expectedRevision（或完整 CareerAction），重试原样重放。

-     const action: CareerAction = { action: 'confirm', key: PUSH_FACT_KEY, value: current.value, source: { kind: 'user', label: '本人确认' }, requestId: current.requestId, expectedRevision: view?.revision ?? 0 }
+ type SubscriptionAttempt = { kind: 'subscription'; requestId: string; value: 'subscribed' | 'unsubscribed'; expectedRevision: number }
+ // 发起时冻结：current = { kind: 'subscription', requestId, value, expectedRevision: view?.revision ?? 0 }
+ // 重试与首次发送统一使用冻结值：
+     const action: CareerAction = { action: 'confirm', key: PUSH_FACT_KEY, value: current.value, source: { kind: 'user', label: '本人确认' }, requestId: current.requestId, expectedRevision: current.expectedRevision }


─── apps/web/src/career/InboxPage.tsx:119-124 ───
[performance · medium] 各待办的 applicationId 定位请求相互独立，却在 for 循环内逐条 await 串行执行（N+1 串行）；待办较多时首屏时延线性累加，且每次
refresh()（每次写成功后都会触发）整轮重复。客户端无批量接口（career.ts 仅有单条 application(id)），但至少应将去重后的 ID 并行请求，单项失败独立标记
'failed'，不拖垮整批。

      const refs: Record<string, ApplicationRef | 'failed'> = {}
-     for (const item of next.reminders) {
-      const applicationId = item.applicationId
-      if (!applicationId || applicationId in refs) continue
+     const ids = [...new Set(next.reminders.map((item) => item.applicationId).filter((id): id is string => !!id))]
+     await Promise.all(ids.map(async (applicationId) => {
       try {
        const receipt = await client.career.application(applicationId, requestScope.signal)
+       if (!scopeController.isCurrent(requestScope.scope)) return
+       refs[applicationId] = { snapshotId: receipt.pinnedEvidence.snapshotId, opportunityId: receipt.pinnedEvidence.opportunityId }
+      } catch { if (scopeController.isCurrent(requestScope.scope)) refs[applicationId] = 'failed' }
+     }))


─── apps/web/src/career/InboxPage.tsx:102-106 ───
[bug · medium] readView 的 catch 静默吞掉 career.open 的所有失败（两个分支都 return undefined，不记录任何错误态）。view 恒为
undefined 时，「登记待办」与推送订阅按钮永久 disabled（disabled={… || view ===
undefined}），用户既不知原因也无重试入口，形成无声的功能不可用。对照同族 ApplicationPage.readRevision，其 catch 会
setRevisionState('error') 并提供「重新读取档案修订」重试。建议增加 view 读取失败状态并在 UI 呈现提示与重试。

    } catch {
     if (!scopeController.isCurrent(requestScope.scope)) return undefined
+    setViewReadFailed(true) // 渲染层据此提示「档案暂时无法读取」并提供重试，而非让按钮无声禁用
     return undefined
    }
   }, [client, scopeController])


─── apps/web/src/career/InboxPage.tsx:221-222 ───
[bug · medium] 订阅回执验收只校验 kind 与 fact.key，未校验 next.fact.value 与 current.value 一致；且
acceptSubscriptionReceipt 的成功文案基于 expected.value（用户意图）而非回执实际生效值 next.fact.value（事实）。本页
pushOutcomeNote 已示范「按回执渲染」，订阅结果却按意图渲染——回执生效值与意图相反时 UI 会宣称与事实相反的状态，违背「回执诚实性」红线。建议补 value 校验并按回执值渲染文案。

-     if (next.kind !== 'confirmed' || next.fact.key !== PUSH_FACT_KEY) throw new ReceiptMismatchError('回执不是本次订阅结果')
+     if (next.kind !== 'confirmed' || next.fact.key !== PUSH_FACT_KEY || next.fact.value !== current.value) throw new ReceiptMismatchError('回执不是本次订阅结果')
      acceptSubscriptionReceipt(next, current)
+ // acceptSubscriptionReceipt 内改按回执生效值渲染：
+ //  setNotice(next.fact.value === PUSH_UNSUBSCRIBED ? '推送提醒已退订：…' : '已重新订阅推送提醒。')


─── apps/web/src/career/InboxPage.tsx:208-209 ───
[bug · low] 恢复流程互斥不完整：writeInFlight ref 仅在 runWrite 置位，lookupReceipt 只依赖异步生效的 writePhase
状态判断。恢复区「查询待办回执」与「用原请求编号重试」快速连点（或陈旧闭包捕获 'unknown'）时可对同一 requestId 并发发出查询 GET 与写入
POST，回执验收与重试结果可能交错覆盖状态。建议 lookupReceipt 与 runWrite 共用同一 in-flight ref 守卫。

   const lookupReceipt = async (): Promise<void> => {
-   if (!attempt || writePhase === 'busy') return
+   if (writeInFlight.current || !attempt || writePhase === 'busy') return
+   writeInFlight.current = true
+   try {
+    // …原逻辑…
+   } finally { writeInFlight.current = false }
+  }


─── apps/web/src/career/InboxPage.tsx:50-55 ───
[style · low] 两处规范违反：1) 本块为三层嵌套三元（applicationId ? / 'failed' ? / applicationRef ?），runWrite 错误分支的
not_found/idempotency_conflict 文案也是嵌套三元链，违反「禁止嵌套三元」规范，建议提取为独立函数/子组件用早返回分支表达；2) 本块硬编码业务路由
'/platform/career/search'，建议复用路由常量或与其他 career 页一致的路径构造函数（如 opportunityEvidencePath 的做法）。

-   {applicationId ? (applicationRef === 'failed'
-    ? <p className="wk-inbox__todo-note" role="note">权威申请暂时无法定位（可能已不可见），可刷新重试；待办本身不受影响。</p>
-    : applicationRef
-     ? <a className="wk-inbox__todo-link" href={applicationDetailPath(applicationRef, applicationId)}>进入权威申请详情</a>
-     : <p className="wk-inbox__todo-note" role="status">正在定位权威申请…</p>)
-    : <a className="wk-inbox__todo-link" href="/platform/career/search">前往持续找岗查看发现</a>}
+ function renderApplicationLink(applicationId: string | undefined, applicationRef: ApplicationRef | 'failed' | undefined): ReactNode {
+  if (!applicationId) return <a className="wk-inbox__todo-link" href={careerSearchPath()}>前往持续找岗查看发现</a>
+  if (applicationRef === 'failed') return <p className="wk-inbox__todo-note" role="note">权威申请暂时无法定位…</p>
+  if (!applicationRef) return <p className="wk-inbox__todo-note" role="status">正在定位权威申请…</p>
+  return <a className="wk-inbox__todo-link" href={applicationDetailPath(applicationRef, applicationId)}>进入权威申请详情</a>
+ }
+ // careerSearchPath() 统一收敛 '/platform/career/search' 常量


─── apps/web/src/career/InboxPage.tsx:4-5 ───
[maintainability · low] 同一文件混用包导入（@weknora/api-client / @weknora/domain/scope）与跨包深层相对路径导入（含 .ts
扩展名）。根因是 api-client 的 index.ts 只导出了 createCareerApi/CareerRequest 及少量 contracts 类型（第 209-211
行），ReminderView/ReminderReceipt/SetReminderInput 等未从包入口导出。深层相对路径绕过包边界、对目录结构调整脆弱，建议在
packages/api-client/src/index.ts 补充导出 career 类型后统一改为包导入。

- import type { ReminderReceipt, ReminderView, SetReminderInput } from '../../../../packages/api-client/src/career.ts'
- import type { CareerAction, CareerReceipt, CareerView } from '../../../../packages/career-core/src/contracts.ts'
+ // packages/api-client/src/index.ts 增加：
+ // export type { ReminderReceipt, ReminderView, SetReminderInput, CareerCoverageView, … } from './career.ts'
+ // 本文件改为：
+ import type { ReminderReceipt, ReminderView, SetReminderInput } from '@weknora/api-client'


─── apps/web/src/career/MaterialPage.tsx:22-25 ───
[maintainability · high] MaterialPage 是 career 目录下唯一仍保留 newRequestId/errorDetails/isUncertainWrite
整套本地副本的页面：ApplicationPage、SubmissionPage（以及
ExportDeletionPage/InboxPage/PreparationPage/ProgressPage）已按 ocr3-054/055 统一改为从 protocol.ts
导入共享实现，并以 endpointDefiniteCodes 在基础契约上叠加端点失败码。本地副本与共享实现已经出现实际漂移：protocol.ts 的 errorDetails 会把
currentRevision 从 string|number 规范化为 number（本地副本直接透传，revision_conflict 时 setRevisionConflict
可能收到字符串），protocol.ts 的 isUncertainWrite 内置 ReceiptMismatchError
判定（本地副本没有）。两套实现并存后，一旦确定/不确定失败码集合再漂移，会把确定性失败误判为未知写（用户被困在永远失败的回执恢复流程），或把未知写误判为确定失败（自动换新 requestId
造成重复保存草稿/重复发布不可变版本），威胁版本与投递数据的诚实性。建议照搬 SubmissionPage 的模式。

- function errorDetails(cause: unknown): { code?: string; requestId?: string; currentRevision?: number; status?: number; message: string } {
-  const error = cause as { code?: string; requestId?: string; currentRevision?: number; status?: number; message?: string }
-  return { code: error?.code, requestId: error?.requestId, currentRevision: error?.currentRevision, status: error?.status, message: error?.message || '请求未完成' }
- }
+ // 文件顶部统一从 protocol.ts 导入，端点码叠加（同 SubmissionPage）：
+ import { ReceiptMismatchError, errorDetails, isUncertainWrite as baseIsUncertainWrite, newRequestId } from './protocol.ts'
+ const endpointDefiniteCodes: readonly string[] = ['revision_conflict', 'material_claim_unconfirmed']
+ const isUncertainWrite = (cause: unknown): boolean => endpointDefiniteCodes.includes(errorDetails(cause).code ?? '') ? false : baseIsUncertainWrite(cause)


─── apps/web/src/career/MaterialPage.tsx:20-20 ───
[maintainability · medium] EXPORT_GRANT_TTL_SECONDS = 900 是对后端 internal/modules/career/rendering.go
中 MaxExportGrantTTL = 15 * time.Minute 的跨端镜像，仅靠注释维持同步。后端在签发时会拒绝 ttl > MaxExportGrantTTL
的请求（rendering.go:1309 `ttl <= 0 || ttl > MaxExportGrantTTL`
即拒绝），一旦后端调小该上限，前端每次下载都会先签发失败，下载链路功能性中断且没有任何编译期或测试提示。建议把上限作为唯一事实源由 packages/api-client/src/career.ts
导出（或由 career open 视图下发），前端引用而非镜像。

- const EXPORT_GRANT_TTL_SECONDS = 900
+ // packages/api-client/src/career.ts 导出唯一事实源，前端引用而非镜像：
+ import { MAX_EXPORT_GRANT_TTL_SECONDS } from '../../../../packages/api-client/src/career.ts'
+ const EXPORT_GRANT_TTL_SECONDS = MAX_EXPORT_GRANT_TTL_SECONDS


─── apps/web/src/career/MaterialPage.tsx:409-411 ───
[bug · medium] publishExport / revokeExport / downloadExport 三个写入路径缺少 saveDraft/confirmDraft 那样的
in-flight ref 守卫：新 attempt 构造只检查闭包里的 exportPhase === 'busy'，而 setState 在重渲染前不可见，快速双击「发布导出」会用两个不同的
requestId 并发发布同一版本（requestId 幂等只在同号内生效），产生重复导出记录或重复签发下载授权；撤销与下载同理。同文件
writeInFlight（saveDraft/confirmDraft）与 ApplicationPage submitInFlight、SubmissionPage writeInFlight
都有 ref 守卫，唯独导出链路缺口。建议补一个 exportInFlight ref（publish/revoke/download 共用），并在 finally 无条件复位。

+  const exportInFlight = useRef(false)
   const publishExport = useCallback(async (version: number, fixedAttempt?: ExportAttempt): Promise<void> => {
+   if (exportInFlight.current) return
    const currentAttempt: ExportAttempt | undefined = fixedAttempt ?? ((): ExportAttempt | undefined => {
     if (exportPhase === 'busy' || materialId === undefined || revision === undefined) return undefined
+   })()
+   if (!currentAttempt) return
+   exportInFlight.current = true
+   try {
+    // ...
+   } finally { exportInFlight.current = false }
+  }, [/* ... */])


─── apps/web/src/career/MaterialPage.tsx:115-115 ───
[performance · low] dirty 判定在每次渲染都对 bodyFromEditable(sections) 与 savedBody 各做一次 JSON.stringify
全量序列化，材料较大时每次键入触发两次深序列化。且直接比较两个来源不同的 JSON 字符串：savedBody 来自服务端响应，序列化键序/可选字段（factKey、reviewNote
的条件展开）若与服务端回显存在任何差异，会误判 dirty 从而错误禁用「确认发布不可变版本」按钮。建议用 useMemo 缓存序列化结果，并使用固定键序的稳定序列化（或结构化深度比较）。

-  const dirty = savedBody === undefined || JSON.stringify(bodyFromEditable(sections)) !== JSON.stringify(savedBody)
+  const dirty = useMemo(
+   () => savedBody === undefined || stableStringify(bodyFromEditable(sections)) !== stableStringify(savedBody),
+   [savedBody, sections],
+  )
+  // stableStringify：按固定键序递归序列化，避免服务端回显键序差异误判 dirty


─── apps/web/src/career/MaterialPage.tsx:249-250 ───
[bug · low] saveDraft 与 confirmDraft 的 material_claim_unconfirmed 分支中 `await reloadView(materialId)`
都未包 try/catch：此处已处于 catch 块内，reloadView 再抛错会使异常冒泡到外层 Promise；而调用方是 `void saveDraft()`/`void
confirmDraft()`，将产生未处理的 Promise 拒绝（虽然 finally 仍会复位 writeInFlight，但回读失败没有任何用户反馈，还会污染错误上报）。同函数其他
reloadView 调用点（如成功路径 ocr1-072 修复处）都已单独包 try/catch，这两处是遗漏。

-     setMessage(`草稿保留未发布：引用了未确认事实（${parsed.message}）。请改为缺失占位，或先在档案中确认该事实。`)
-     if (materialId) await reloadView(materialId)
+     if (materialId) await reloadView(materialId).catch(() => { /* 回读失败不改变已置的 error 提示，与 ocr1-072 成功路径回读处理一致 */ })


─── apps/web/src/career/MaterialPage.tsx:546-546 ───
[bug · low] 章节列表以数组下标作 React key：删除中间章节时，后续章节组件身份整体前移一位，输入框（标题/正文/主张
textarea）会复用错位的组件状态，焦点与受控值映射错乱（尤其配合 disabled 切换时）。主张列表已用 claim.claimId 作 key，章节应同样持有稳定 ID。建议在
EditableSection 增加创建时生成的本地 sectionId（如 `section-${++sectionCounter}`，与 claimCounter 同款防碰撞），以它作 key。

-     {sections.map((section, sectionIndex) => <fieldset className="wk-material__section" key={sectionIndex}>
+     {sections.map((section, sectionIndex) => <fieldset className="wk-material__section" key={section.sectionId}>


─── apps/web/src/career/MaterialPage.tsx:435-439 ───
[bug · medium] publishExport 的 revision_conflict 分支把说明写入 setMessage（渲染在材料主消息区，且 phase 未被置
error，只能以非错误的 status 样式呈现），而本分支的 exportMessage 仍停留在请求开始时的「正在渲染并核验导出（PDF 与 DOCX 双格式核验）…」。结果导出面板里
exportPhase==='error' 会把这句过时的进度文案按 wk-material__message--error + role=alert
渲染成错误警报，真正的修订冲突说明却不在导出面板内；revokeExport 的同款分支（「再次撤销导出」文案处）有一模一样的问题。应与同函数其他错误分支一致改用
setExportMessage，让导出面板显示真实原因。

     if (parsed.code === 'revision_conflict') {
      setExportAttempt(undefined); setExportPhase('error'); setExportConflict(parsed.currentRevision)
-     setMessage(`档案已更新${parsed.currentRevision !== undefined ? `（当前修订 ${parsed.currentRevision}）` : ''}。请重新读取档案修订后再次发布导出；新提交会使用新的请求编号。`)
+     setExportMessage(`档案已更新${parsed.currentRevision !== undefined ? `（当前修订 ${parsed.currentRevision}）` : ''}。请重新读取档案修订后再次发布导出；新提交会使用新的请求编号。`)
      return
     }


─── apps/web/src/career/MaterialPage.tsx:43-47 ───
[bug · low] sha256Hex 直接调用 crypto.subtle.digest 而未检测 crypto.subtle 是否存在：非安全上下文（本项目支持自托管，LAN 上以 http
访问很常见）中 crypto.subtle 为 undefined，会抛出 TypeError，被 downloadExport 的兜底 catch
吞掉后只显示「下载未完成：请求未完成」——每次下载必然失败且文案误导（下载从未发出校验问题，而是能力缺失）。同文件 newRequestId 已对 crypto.randomUUID
做了特性检测，这里应保持一致并给出明确提示（或提前禁用下载入口）。

  const sha256Hex = async (body: string | Blob | ArrayBuffer): Promise<string> => {
+  if (typeof crypto === 'undefined' || !crypto.subtle) throw new Error('当前环境不支持摘要校验（需 HTTPS 安全上下文），已拒绝保存文件')
   const bytes = typeof body === 'string' ? new TextEncoder().encode(body) : body instanceof ArrayBuffer ? body : await body.arrayBuffer()
   const sum = await crypto.subtle.digest('SHA-256', bytes)
   return [...new Uint8Array(sum)].map((byte) => byte.toString(16).padStart(2, '0')).join('')
  }


─── apps/web/src/career/OpportunityPage.tsx:0-0 ───
[bug · medium] 软匹配渲染使用非空断言 `!` 但未做空值兜底：fact 由 `${factKey}:${factRevision}` 拼键从顶层 facts 查找，当前后端
evaluateSnapshot（evaluation.go:268-280）确实保证每条 soft.match 的证据都同步进入 facts 数组，但这是隐式跨端契约——若后端新增匹配类型、或
facts 去重逻辑调整导致任一证据缺失，`fact.factKey` 会在渲染期抛 TypeError，整个 EvaluationDetailPage
白屏。注意同文件硬性条件路径（hard.rules）对同样的 `facts.get(...)` 已采用 `fact ? ... : <p>档案依据：没有可用的已确认事实。</p>`
的条件渲染，两条路径防御姿态不一致。建议与硬性条件路径对齐，改为条件渲染兜底。

- {currentEvaluation.soft.matches.map((match, index) => { const fact = facts.get(`${match.profileEvidence.factKey}:${match.profileEvidence.factRevision}`)!;
+ {currentEvaluation.soft.matches.map((match, index) => { const fact = facts.get(`${match.profileEvidence.factKey}:${match.profileEvidence.factRevision}`)
+   return <li key={`${match.kind}-${index}`}>...
+   <p>已确认档案依据：{fact ? <a href={`#${factAnchor(fact)}`}>{fact.factKey} = {fact.value}（事实版本 {fact.factRevision}）</a> : '没有可用的已确认事实。'}</p>


─── apps/web/src/career/OpportunityPage.tsx:147-147 ───
[maintainability · low] 嵌套三元表达式链（此处为 3 层：not_found → idempotency_conflict →
PAYLOAD_TOO_LARGE/request_too_large → 默认），违反「禁止嵌套三元」规范；同文件 importURLAttempt 的 setUrlMessage、以及 JSX 中
state/evaluationState 的按钮文案链也有同类结构。错误码到文案的映射是典型的映射表场景，改为 Record 查表后新增错误码只需加一行，且可消除
'PAYLOAD_TOO_LARGE' 与 'request_too_large' 双码并列的重复文案。

-     setMessage(parsed.code === 'not_found' ? '职位或来源不存在，职位描述未被导入。请检查来源后重新提交。' : parsed.code === 'idempotency_conflict' ? '请求编号已对应其他内容，服务器拒绝了本次提交。请检查内容后使用新的请求重新保存。' : parsed.code === 'PAYLOAD_TOO_LARGE' || parsed.code === 'request_too_large' ? '职位描述超过服务端允许的大小，请缩短后重新保存。' : `职位描述未被接受：${parsed.message}`)
+ const importRejectMessages: Record<string, string> = {
+   not_found: '职位或来源不存在，职位描述未被导入。请检查来源后重新提交。',
+   idempotency_conflict: '请求编号已对应其他内容，服务器拒绝了本次提交。请检查内容后使用新的请求重新保存。',
+   request_too_large: '职位描述超过服务端允许的大小，请缩短后重新保存。',
+   PAYLOAD_TOO_LARGE: '职位描述超过服务端允许的大小，请缩短后重新保存。',
+ }
+ setMessage(importRejectMessages[parsed.code ?? ''] ?? `职位描述未被接受：${parsed.message}`)


─── apps/web/src/career/OpportunityPage.tsx:277-277 ───
[bug · low] factAnchor 把 factKey 中所有非 ASCII 字符替换为连字符：career 档案的 factKey 全部是中文（毕业时间/学历/城市/意向，见
CareerPage fields），生成的锚点退化为 `fact------{factRevision}`，键名信息完全丢失。锚点/DOM id/React key（`<li
id={factAnchor(fact)} key={factAnchor(fact)}>`）的唯一性就完全依赖「不同事实的 factRevision
永不相同」这一隐式后端不变量（factRevision = 该事实确认时的档案修订号，见 evaluation.go:400 `FactRevision:
fact.Revision`）。一旦后端出现同修订号下多条事实（如批量导入确认），就会产生重复 DOM id 与重复 React key，锚点跳转错位。建议保留键名信息，用
encodeURIComponent 编码而非丢弃。

- const factAnchor = (fact: Evaluation['facts'][number]): string => `fact-${fact.factKey.replace(/[^a-zA-Z0-9_-]/g, '-')}-${fact.factRevision}`
+ const factAnchor = (fact: Evaluation['facts'][number]): string => `fact-${encodeURIComponent(fact.factKey)}-${fact.factRevision}`


─── apps/web/src/career/PreparationPage.tsx:87-91 ───
[bug · medium] reviseBusy 卡死缺陷：saveRevision 在途时若发生 scope 切换（abort 信号触发），editMaterial 的成功分支（`if
(!scopeController.isCurrent(requestScope.scope)) return`）与 catch 分支的同款早退都会跳过 setReviseBusy(false)，而
clearPrivate 未重置 reviseBusy。结果：切回原空间后 `revising={reviseBusy || ...}` 恒为 true，所有「修订草稿」按钮及编辑输入永久
disabled，直到面板卸载重挂。对比 writePhase/writeInFlight 均有复位通道（clearPrivate + finally），唯独 reviseBusy 缺失。建议在
clearPrivate 中一并复位，或为 saveRevision 增加 finally 兜底。

   const clearPrivate = useCallback((notice: string, nextState: 'forbidden' | 'scope-changed' = 'forbidden') => {
    setItems(undefined); setReadState(nextState); setReadMessage(notice)
    setAttempt(undefined); setWritePhase('idle'); setMessage(''); setFocus('')
    setEditing(undefined); setRevised(undefined); setReviseMessage(''); setReviseConflict(undefined)
+   setReviseBusy(false)
   }, [])


─── apps/web/src/career/PreparationPage.tsx:357-357 ───
[maintainability · low] 以 `reviseMessage.startsWith('修订已保存')` 中文文案前缀判定消息语义（成功 status / 失败 alert
及错误样式），是脆弱的隐式契约：文案一旦微调（本地化、标点、措辞），成功消息会被误判为错误并以 role=alert 向读屏用户播报。且该前缀需与 saveRevision
中两条成功文案（含回读失败回退分支）保持同步。建议改用显式状态字段（如 reviseOk: boolean）驱动样式与 role，消息仅作展示。

-     {reviseMessage ? <p className={reviseMessage.startsWith('修订已保存') ? 'wk-preparation__message' : 'wk-preparation__message wk-preparation__message--error'} role={reviseMessage.startsWith('修订已保存') ? 'status' : 'alert'} aria-live="polite">{reviseMessage}</p> : null}
+     {reviseMessage ? <p className={reviseOk ? 'wk-preparation__message' : 'wk-preparation__message wk-preparation__message--error'} role={reviseOk ? 'status' : 'alert'} aria-live="polite">{reviseMessage}</p> : null}


─── apps/web/src/career/PreparationPage.tsx:239-243 ───
[bug · medium] 修订基线过期导致静默覆盖已保存修订：startRevise 用列表回执的 receipt.body 初始化编辑器，但后端
ApplicationPreparations（preparation.go:666-677）逐行 decode career_preparations.receipt_body——即生成时的快照；而
saveRevision → editMaterial 只更新
career_materials.draft_body，不会回写该快照。因此用户保存修订后再次从同一列表行点「修订草稿」，会以生成时旧正文为基线编辑，保存后无条件覆盖 material
草稿，之前的修订内容被静默丢弃（行内正文也始终显示旧快照，无任何「材料域已有更新草稿」提示；revised 区块只保留最后一次）。这与页面自己声明的「两者都以同一材料草稿为准继续演进」相矛盾。建议
startRevise 优先取材料域当前草稿：revised.materialId 命中时复用 revised.body，未命中时异步读取
client.career.material(receipt.materialId) 的 view.body 作为基线（或至少在行内已有更新草稿时给出覆盖警示）。

   const startRevise = (receipt: PreparationReceipt): void => {
    if (!receipt.materialId) return
-   setEditing({ materialId: receipt.materialId, body: cloneBody(receipt.body), source: receipt })
+   // 以材料域当前草稿为基线，避免生成时回执快照覆盖已保存的修订
+   const base = revised?.materialId === receipt.materialId ? revised.body : receipt.body
+   setEditing({ materialId: receipt.materialId, body: cloneBody(base), source: receipt })
    setReviseMessage(''); setReviseConflict(undefined)
   }


─── apps/web/src/career/PreparationPage.tsx:202-202 ───
[maintainability · low] 确定性失败码数组 ['invalid_request', 'idempotency_conflict', 'not_found',
'request_too_large', 'PAYLOAD_TO_LARGE'] 在本文件 runGenerate 与 saveRevision
中重复出现两次，ProgressPage.runWrite 中还有第三份完全相同的字面量；protocol.ts 的 isUncertainWrite
内部也维护着同一语义的列表。四处重复意味着新增/调整确定性失败码时需要多点同步修改，存在漂移风险（漏改处会落入通用错误文案分支）。建议在 protocol.ts 导出共享常量（如
definiteClientErrorCodes），页面复用。

-    if (['invalid_request', 'idempotency_conflict', 'not_found', 'request_too_large', 'PAYLOAD_TOO_LARGE'].includes(parsed.code ?? '')) {
+ // protocol.ts
+ export const definiteClientErrorCodes: readonly string[] = ['invalid_request', 'idempotency_conflict', 'not_found', 'request_too_large', 'PAYLOAD_TOO_LARGE']
+ 
+ // 页面内
+ if (definiteClientErrorCodes.includes(parsed.code ?? '')) {


─── apps/web/src/career/PreparationPage.tsx:295-295 ───
[style · low] 嵌套三元表达式：本行是三层嵌套三元（loading → error → revision!==undefined → ''），违反项目检查规则「不允许嵌套三元」。同文件
PreparationRow 中还有两处两层嵌套（failed ? (status==='failed' ? … : …) : … 与 claim.factKey ? … :
claim.needsReview ? … : ''）。建议提取为具名辅助函数（如 revisionStatusText / claimSuffix），用 if-return
或映射表表达，提升可读性与可测性。

-     <p className="wk-preparation__revision" role="status">{revisionState === 'loading' ? '正在读取当前档案修订…' : revisionState === 'error' ? '暂时无法读取当前档案修订，可稍后重试；准备生成会被暂缓。' : revision !== undefined ? `当前档案修订 ${revision}（准备生成将按此修订提交）` : ''}</p>
+ function revisionStatusText(state: 'loading' | 'ready' | 'error', revision?: number): string {
+  if (state === 'loading') return '正在读取当前档案修订…'
+  if (state === 'error') return '暂时无法读取当前档案修订，可稍后重试；准备生成会被暂缓。'
+  return revision !== undefined ? `当前档案修订 ${revision}（准备生成将按此修订提交）` : ''
+ }
+ // 使用：{revisionStatusText(revisionState, revision)}


─── apps/web/src/career/PreparationPage.tsx:311-311 ───
[bug · low] revisionState === 'error'
时缺少恢复入口：该状态下生成按钮被禁用（generateBlocked）、修订保存被暂缓，提示文案承诺「可稍后重试」，但「重新读取档案修订」按钮仅在 writePhase==='error' &&
revisionConflict!==undefined 时渲染；「刷新准备列表」只触发列表 reload、不会重跑 readRevision（其 useEffect 仅依赖
[readRevision]，readRevision 的 useCallback 依赖 [client, scopeController] 均稳定，scope
变化也不会触发重读）。结果是同空间内一次读取失败后，档案修订永远无法恢复，生成/修订功能持续不可用，直到组件卸载重挂。建议把重读按钮的渲染条件扩展到 revisionState ===
'error'。

-     {writePhase === 'error' && revisionConflict !== undefined ? <div className="wk-preparation__actions"><button type="button" onClick={() => { setRevisionConflict(undefined); void readRevision() }}>重新读取档案修订</button></div> : null}
+     {(revisionState === 'error' || (writePhase === 'error' && revisionConflict !== undefined)) ? <div className="wk-preparation__actions"><button type="button" onClick={() => { setRevisionConflict(undefined); void readRevision() }}>重新读取档案修订</button></div> : null}


─── apps/web/src/career/ProgressPage.tsx:27-27 ───
[style · low] 嵌套三元表达式：ProgressEventRow 的 className 是两层嵌套三元（corrected → kind==='progress_corrected' →
默认），违反项目检查规则「不允许嵌套三元」。建议提取为具名辅助函数（如 eventRowClass），用 if-return 表达，与 PreparationRow 等行组件保持一致的可读性。

-  return <li className={item.corrected ? 'wk-progress__event wk-progress__event--corrected' : item.kind === 'progress_corrected' ? 'wk-progress__event wk-progress__event--correction' : 'wk-progress__event'} aria-label={`进展事件 ${item.eventId}`}>
+ function eventRowClass(item: ProgressEventView): string {
+  if (item.corrected) return 'wk-progress__event wk-progress__event--corrected'
+  if (item.kind === 'progress_corrected') return 'wk-progress__event wk-progress__event--correction'
+  return 'wk-progress__event'
+ }
+ // 使用：<li className={eventRowClass(item)} aria-label={`进展事件 ${item.eventId}`}>


─── apps/web/src/career/RulePage.tsx:23-26 ───
[maintainability · medium] errorDetails / isUncertainOutcome 是 protocol.ts 共享实现（errorDetails /
isUncertainWrite）的本地复制，且确定性失败码列表逐项相同。ExportDeletionPage 已按 ocr3-054/055 迁移到共享实现（其顶部注释明示'本地副本与共享类同名但
instanceof 不互通'的坑），本页未跟进：protocol.ts 若将来调整确定码集合或增加 ReceiptMismatchError 前置判定，规则页将静默漂移。另外本地
errorDetails 对 currentRevision 缺少 typeof 守卫（protocol 版处理 string|number）。建议复用 protocol.ts（text 即其
message），至少保持两处判定完全委托共享实现。

- function errorDetails(cause: unknown): TypedError {
-  const error = cause as { code?: string; currentRevision?: number; message?: string }
-  return { code: error?.code, currentRevision: error?.currentRevision, text: error?.message || '请求未完成' }
+ import { errorDetails as sharedErrorDetails, isUncertainWrite } from './protocol.ts'
+ const errorDetails = (cause: unknown): TypedError => {
+  const parsed = sharedErrorDetails(cause)
+  return { code: parsed.code, currentRevision: parsed.currentRevision, text: parsed.message }
  }
+ // isUncertainOutcome 直接改用 isUncertainWrite


─── apps/web/src/career/RulePage.tsx:92-93 ───
[bug · low] load() 成功后未重置 notice：storedRuleUnreadable
场景点击'重新读取'成功后（setStoredRuleUnreadable(undefined) 解除锁、表单恢复可编辑），compose
卡内的过期告警'读取已保存规则未成功…保存已暂时停用'仍残留，与已解锁状态自相矛盾；not_found 分支设置的提示在重读后同样残留。建议在读取成功（viewPhase 置 ready）时清空
notice。

     setRevision(view.revision)
     setViewPhase('ready')
+    setNotice('')


─── apps/web/src/career/RulePage.tsx:299-301 ───
[style · low] 下次运行计划使用 enabled→paused→默认的链式嵌套三元，违反'嵌套三元表达式不允许'的清单约定，且三分支各是长 JSX 难以扫读。建议提取为
renderNextRunPlan(live) 辅助函数或先计算 planTone/planText 再渲染。

-      {live.status === 'enabled' && live.nextDueAt ? <p className="wk-career-rule__plan wk-career-rule__plan--scheduled">下次运行（计划）：{formatCheckTime(live.nextDueAt)}。修改规则后该计划按新频率重新排程。</p>
-       : live.status === 'paused' ? <p className="wk-career-rule__plan wk-career-rule__plan--paused">已暂停：下一次触发已取消，当前没有排程。恢复启用后将按恢复时刻重新排程（顺延，不追补暂停期间的周期）。</p>
-       : <p className="wk-career-rule__plan wk-career-rule__plan--off">规则未启用：不会运行，也不会在后台执行任何搜索。启用后才会排出下次运行计划。</p>}
+ function renderNextRunPlan(live: RuleView | SetRuleReceipt): ReactNode {
+  if (live.status === 'enabled') return live.nextDueAt ? <p className="wk-career-rule__plan wk-career-rule__plan--scheduled">下次运行（计划）：{formatCheckTime(live.nextDueAt)}。修改规则后该计划按新频率重新排程。</p> : <p className="wk-career-rule__plan wk-career-rule__plan--off">已启用但暂无排程。</p>
+  if (live.status === 'paused') return <p className="wk-career-rule__plan wk-career-rule__plan--paused">已暂停：下一次触发已取消，当前没有排程。恢复启用后将按恢复时刻重新排程（顺延，不追补暂停期间的周期）。</p>
+  return <p className="wk-career-rule__plan wk-career-rule__plan--off">规则未启用：不会运行，也不会在后台执行任何搜索。启用后才会排出下次运行计划。</p>
+ }
+ // 渲染处：{renderNextRunPlan(live)}


─── apps/web/src/career/RulePage.tsx:330-330 ───
[security · low] todo.link 直接作为 <a href> 渲染，未校验协议。链接来自外部搜索抓取，若后端未严格校验 URL 格式，被污染的 javascript:/data:
链接可经新标签执行（noopener 只切断 opener，不拦截 javascript: URL）。前端做一层 http(s) 白名单即可防御纵深。

-       <a href={todo.link} target="_blank" rel="noreferrer noopener">岗位链接</a>
+       {/^https?:\/\//i.test(todo.link) ? <a href={todo.link} target="_blank" rel="noreferrer noopener">岗位链接</a> : <span>链接不可用</span>}


─── apps/web/src/career/RulePage.tsx:192-192 ───
[bug · low] unknown 态的 attempt.requestId 仅存于组件内存：若这是一次『新建规则』（尚无 ruleId，acceptReceipt 成功前不写
localStorage），用户在结果未知时刷新页面，load() 读不到 storedRuleId、attempt
也已丢失，表单解锁后再次保存会静默铸造新请求编号——若首次写入实际已成功，将产生第二条规则（本页注释自述后端无 one-rule-per-user
限制，旧启用规则会持续扣费），违背卡片承诺『重试不会写入第二条规则』。更新路径有持久化 ruleId 兜底，唯独创建路径暴露。建议在进入 unknown 态时把 pending
attempt（requestId + 内容摘要）按 userId/tenantId 持久化，载入时先对账再解锁表单（可与 ExportDeletionPage 的同款问题一并治理）。



─── apps/web/src/career/RulePage.tsx:14-14 ───
[maintainability · low] makeId 与 protocol.ts 的 newRequestId（第 25
行）逐字符相同，是共享实现的又一份本地副本（errorDetails/isUncertainOutcome 的同款问题已另列）。protocol.ts
若将来调整降级策略或增加熵源，本页将静默漂移。本目录其余页面均已改用 `import { newRequestId } from './protocol.ts'`，建议直接复用并删除 makeId。

- const makeId = (): string => typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`
+ import { newRequestId } from './protocol.ts'
+ // 删除本地 makeId，submit 内改用 newRequestId()


─── apps/web/src/career/SearchPage.tsx:26-29 ───
[maintainability · medium] 重复实现共享协议工具：protocol.ts 已集中提供
errorDetails/isUncertainWrite/newRequestId（OpportunityPage.tsx 已在使用），本文件又本地实现了
errorDetails、isUncertainOutcome、makeId 三份等价逻辑，且确定性错误码白名单已经分叉（本地版额外排除
search_quota_refused、revision_conflict）。「未知写按原 requestId
恢复、确定性拒绝不得进入回执恢复」是本次变更的红线语义，同一判定散落三处实现，后续调整白名单时极易漏改其中一处造成恢复路径误判。建议复用 protocol.ts 的 errorDetails 与
newRequestId，isUncertainOutcome 仅在其上叠加 search 专属的确定性错误码。

- function errorDetails(cause: unknown): TypedError {
-  const error = cause as { code?: string; currentRevision?: number; message?: string }
-  return { code: error?.code, currentRevision: error?.currentRevision, text: error?.message || '请求未完成' }
+ import { errorDetails, isUncertainWrite, newRequestId } from './protocol.ts'
+ 
+ // 删除本地 errorDetails 与 makeId，isUncertainOutcome 收敛为叠加 search 专属码：
+ function isUncertainOutcome(cause: unknown): boolean {
+  if (['search_quota_refused', 'revision_conflict'].includes(errorDetails(cause).code ?? '')) return false
+  return isUncertainWrite(cause)
  }


─── apps/web/src/career/SearchPage.tsx:147-152 ───
[bug · low] 在 setState 的 updater 函数内部执行 localStorage 写入副作用，违反 React updater 必须纯函数的约定：StrictMode
开发模式下 updater 会被双调用（当前写入幂等尚无数据损坏），但并发渲染中 updater 可能被丢弃或重放，持久化内容可能与最终提交的状态不一致。`openHistoryEntry` 的
not_found 分支里 `setHistoryEntries((current) => { ... writeHistory(...) ... })` 是同一模式。建议用 ref 镜像当前列表，在
updater 外计算并持久化后再 setState。

-   setHistoryEntries((current) => {
-    const entry: HistoryEntry = { searchId: next.searchId, requestId: next.requestId, query: next.query, status: next.status, checkedAt: next.checkedAt }
-    const merged = [entry, ...current.filter((item) => item.searchId !== next.searchId)].slice(0, 8)
+   const merged = [entry, ...historyRef.current.filter((item) => item.searchId !== next.searchId)].slice(0, 8)
+   historyRef.current = merged
-    writeHistory(typeof window === 'undefined' ? undefined : window.localStorage, storageKey, merged)
+   writeHistory(typeof window === 'undefined' ? undefined : window.localStorage, storageKey, merged)
-    return merged
-   })
+   setHistoryEntries(merged)


─── apps/web/src/career/SearchPage.tsx:305-305 ───
[style · low] 错误标题使用嵌套三元链（revision_conflict → search_quota_refused → 默认），违反「禁止嵌套三元」规范；`send` 中
`setNotice(parsed.code === 'invalid_request' ? ... : ...)` 同类。与 CareerPage/OpportunityPage
的修复方向一致，改为 Record 查表。

-    {error ? <Card bordered><div role="alert"><strong>{error.code === 'revision_conflict' ? '档案已更新' : error.code === 'search_quota_refused' ? '找岗额度受限' : '找岗未成功'}</strong><p>{error.text}{error.currentRevision !== undefined ? `（当前修订 ${error.currentRevision}）` : ''}</p></div>
+ const errorTitles: Record<string, string> = { revision_conflict: '档案已更新', search_quota_refused: '找岗额度受限' }
+ // ...
+ <strong>{errorTitles[error.code ?? ''] ?? '找岗未成功'}</strong>


─── apps/web/src/career/SubmissionPage.tsx:36-36 ───
[maintainability · medium] 「双格式核验通过才可投递」是投递诚实性的关键门控，却在两处各自实现一份：本页 deliverableExports（第 36 行）与
MaterialPage 导出列表的内联 deliverable（第 594 行）完全相同的布尔表达式。任一侧规则调整（例如后端新增第三种格式或改变 files 数量要求）易遗漏另一侧，导致
MaterialPage 展示可下载而 SubmissionPage 不可选（或反之）的门控不一致。此外三页 catch 中的错误码阶梯（forbidden / revision_conflict /
idempotency_conflict / not_found / request_too_large …）也大量复制，建议与 protocol.ts 一样下沉为共享判定。

- const deliverableExports = (list: MaterialExportReceipt[]): MaterialExportReceipt[] => list.filter((receipt) => receipt.status === 'submittable' && receipt.submittable && receipt.files.length >= 2 && receipt.files.every((file) => file.verified))
+ // protocol.ts（或新建 career-exports.ts）共享门控，两页复用：
+ export const isDeliverableExport = (receipt: MaterialExportReceipt): boolean =>
+  receipt.status === 'submittable' && receipt.submittable && receipt.files.length >= 2 && receipt.files.every((file) => file.verified)
+ // SubmissionPage：const deliverableExports = (list) => list.filter(isDeliverableExport)
+ // MaterialPage 内联处：const deliverable = isDeliverableExport(receipt)


─── apps/web/src/career/SubmissionPage.tsx:176-178 ───
[maintainability · low] occurredAtFromInput(occurredAt) 在两个分支里各被调用两次（共 4 次）——每次调用都会 new Date
并重新解析，理论上多次调用间日期翻转会得到不一致结果，也让条件展开难读；selectedExport 使用非空断言 `!`（虽有上方 `if (versionChoice !==
UNKNOWN_VERSION_CHOICE && !selectedExport) return` 守卫保护）。建议先算一次局部变量复用，并避免非空断言。

+    const occurredAtClaim = occurredAtFromInput(occurredAt)
     const input: RecordSubmissionInput = versionChoice === UNKNOWN_VERSION_CHOICE
-     ? { requestId, applicationId, channel, versionUnknown: true, expectedRevision: revision, ...(occurredAtFromInput(occurredAt) ? { occurredAt: occurredAtFromInput(occurredAt) } : {}), ...(note.trim() ? { note: note.trim() } : {}) }
-     : { requestId, applicationId, channel, versionUnknown: false, materialId: selectedExport!.materialId, exportId: versionChoice, expectedRevision: revision, ...(occurredAtFromInput(occurredAt) ? { occurredAt: occurredAtFromInput(occurredAt) } : {}), ...(note.trim() ? { note: note.trim() } : {}) }
+     ? { requestId, applicationId, channel, versionUnknown: true, expectedRevision: revision, ...(occurredAtClaim ? { occurredAt: occurredAtClaim } : {}), ...(note.trim() ? { note: note.trim() } : {}) }
+     : { requestId, applicationId, channel, versionUnknown: false, materialId: (selectedExport ?? exports.find((receipt) => receipt.exportId === versionChoice))!.materialId, exportId: versionChoice, expectedRevision: revision, ...(occurredAtClaim ? { occurredAt: occurredAtClaim } : {}), ...(note.trim() ? { note: note.trim() } : {}) }


─── apps/web/src/career/SubmissionPage.tsx:160-163 ───
[bug · medium] acceptReceipt 在成功时 setWritePhase('idle') 并 setMessage('投递已记录。…')，但渲染处的条件是 `{message
&& writePhase !== 'idle' ? … : null}`——成功后 writePhase 恒为 'idle'，这条成功消息永远不会被渲染（本文件中 message
只有这一处渲染点），等于死消息；lookupReceipt 成功路径同样经 acceptReceipt 被抑制。用户只能靠时间线刷新间接感知。对比 MaterialPage 的渲染门为
`{message ? …}`（无 phase 门）。建议放宽渲染条件，或改为显式成功提示/清理消息。

-  const acceptReceipt = (next: SubmissionReceipt, expected: WriteAttempt): void => {
-   if (next.requestId !== expected.requestId || next.applicationId !== applicationId) throw new ReceiptMismatchError('投递回执与本次请求不匹配')
-   setAttempt(undefined); setWritePhase('idle'); setMessage('投递已记录。记录已呈现在投递时间线，可回看绑定的材料版本。'); resetCompose(); refresh()
-  }
+ {message ? <p className={writePhase === 'error' ? 'wk-submission__message wk-submission__message--error' : 'wk-submission__message'} role={writePhase === 'error' ? 'alert' : 'status'} aria-live="polite">{message}</p> : null}


─── apps/web/src/career/UsagePanel.tsx:27-34 ───
[documentation · low] 注释声称 usageAllowsChargedRun 是 SearchPage submit 与 RulePage enable
两个收费入口共同的'单一准入闸门'，但 RulePage 实际只以 usage.state.phase !== 'ready' 判定（enableRequiresEstimate），未检查
wouldAdmit——两处准入口径并不相同（SearchPage 要求 ready+wouldAdmit；RulePage 仅要求 ready，额度耗尽仍可启用并依赖
blocked_no_quota 可见拦截，这是 RulePage 注释里的有意设计）。该注释失实此前 OCR 轮次已记录（ocr-round-3-resume.md
2924-2927）仍未修正，会误导后续维护者据'统一门禁'的假契约改代码。请如实改写注释，或让 RulePage 真正复用该函数统一口径。

- // usageAllowsChargedRun is the single admission gate consumed by the charged
- // entries (SearchPage submit, RulePage enable): a charged run may only start
- // from a live, admitting estimate. Loading, unreadable, forbidden and
- // exhausted estimates all fail closed — there is never an execute-first
- // path that reports the cost afterwards.
+ // usageAllowsChargedRun is the admission gate consumed by SearchPage submit
+ // (ready + wouldAdmit). RulePage enable intentionally only requires a live
+ // estimate (phase === 'ready'): an exhausted window does not block enabling;
+ // each trigger is then visibly blocked (blocked_no_quota).
  export function usageAllowsChargedRun(usage: CareerUsageState): boolean {
   return usage.phase === 'ready' && usage.estimate.wouldAdmit
  }


─── apps/web/src/career/UsagePanel.tsx:5-5 ───
[maintainability · low] 类型经 '../../../../packages/api-client/src/career.ts' 深相对路径（含显式 .ts 扩展）跨包导入，绕过
@weknora/api-client 包入口——同文件中 WeKnoraClient 却走别名，风格割裂；career 的视图/回执类型目前未从 index.ts 导出，导致 career 目录下
20+ 文件被迫复制这种脆弱写法，目录层级或包产物调整时会成片断裂。建议推动在 packages/api-client/src/index.ts 补导出这些类型（如
UsageEstimateView），再统一改为 '@weknora/api-client' 别名导入。

- import type { UsageEstimateView } from '../../../../packages/api-client/src/career.ts'
+ import type { UsageEstimateView } from '@weknora/api-client'
+ // 前置：在 packages/api-client/src/index.ts 中 export type { UsageEstimateView } from './career.ts'


─── apps/web/src/career/application.css:74-78 ───
[maintainability · low] 品牌绿 #07c05f 多处硬编码，而 token 已存在且值一致：tdesign-theme.css 中
--td-brand-color(--td-brand-color-4) = #07c05f，design-tokens styles.css 中 --wk-color-brand =
#07c05f。同理警示橙 #ffb648/#fff7e8/#4a3200/#b45309 未走 --td-warning-color 色阶（--td-warning-color-1: #fef3e6
等已定义）；focus-visible 回退色 #0052d9 是 TDesign 默认蓝，与品牌绿并存导致主题不一致。另外 application.css / material.css /
submission.css
三文件重复了约百行同款基础样式（容器卡、hint、label、message、code、actions、按钮、focus-visible、媒体查询），建议品牌/警示色统一走
token，基础样式提取共享 career-base.css。

  .wk-application__submit:not(:disabled) {
-   border-color: #07c05f;
-   background: #07c05f;
+   border-color: var(--td-brand-color, #07c05f);
+   background: var(--td-brand-color, #07c05f);
    color: #fff;
  }
+ /* 其余 #07c05f 同步替换为 var(--td-brand-color, #07c05f)；警示色改用 --td-warning-color 系 token */


─── apps/web/src/career/export-deletion.css:43-47 ───
[maintainability · low] 品牌色 #07c05f 在本文件多处硬编码且用 !important 强制覆盖（.wk-lifecycle__notice、__package
左边框同此）。design-tokens 已提供 --td-brand-color（=#07c05f，tdesign-theme.css 第 1 行）与
--wk-color-brand（styles.css 第 4 行），品牌色一旦调整这些散落硬编码不会跟随，造成主题漂移。建议改用 var(--td-brand-color,
#07c05f)，并以提高选择器 specificity（如 .wk-lifecycle__actions .wk-lifecycle__export）替代 !important。

- .wk-lifecycle__export, .wk-lifecycle__download {
-   border-color: #07c05f !important;
-   background: #07c05f !important;
-   color: #fff !important;
+ .wk-lifecycle__actions .wk-lifecycle__export,
+ .wk-lifecycle__actions .wk-lifecycle__download {
+   border-color: var(--td-brand-color, #07c05f);
+   background: var(--td-brand-color, #07c05f);
+   color: var(--td-text-color-anti, #fff);
  }


─── apps/web/src/career/inbox.css:46-46 ───
[maintainability · low] 品牌绿 #07c05f 在本文件与 reconciliation.css 中共以字面量出现 10 余次（submit 边框/背景、hover、intro
边框、todo-link 等）。design-tokens 已提供 --wk-color-brand: #07c05f（packages/design-tokens/src/styles.css）及
TDesign 主题映射 --td-brand-color = --td-brand-color-4 = #07c05f（tdesign-theme.css），本文件其余属性也已在用 --td-*
token，唯独品牌色硬编码。建议统一改用 var(--td-brand-color, #07c05f)，降低换肤/改色维护成本。

- .wk-inbox__submit { border-color: #07c05f; background: #07c05f; color: #fff; }
+ .wk-inbox__submit { border-color: var(--td-brand-color, #07c05f); background: var(--td-brand-color, #07c05f); color: #fff; }


─── apps/web/src/career/material.css:193-193 ───
[maintainability · low] 品牌绿 #07c05f 及警示色 #ffb648/#fff7e8/#b45309 硬编码未走 token：tdesign-theme.css 已定义
--td-brand-color = #07c05f（design-tokens 亦有 --wk-color-brand），警示色有 --td-warning-color
色阶可用；focus-visible 回退 #0052d9 为 TDesign 默认蓝，与品牌绿冲突。且本文件与 application.css / submission.css
存在约百行同款基础样式重复（容器卡、hint、label、input/textarea、claim 警示块、message、code、actions、focus-visible、媒体查询），建议统一
token 化并提取共享 career-base.css，避免三处漂移。

- .wk-material__export-submittable { color: #07c05f; font-weight: 600; }
+ .wk-material__export-submittable { color: var(--td-brand-color, #07c05f); font-weight: 600; }


─── apps/web/src/career/opportunity.css:127-128 ───
[maintainability · low] 硬性规则条目的枚举覆盖不完整：`rule.outcome` 的冻结枚举是 eligible/ineligible/unknown
三态（statusLabel 已覆盖三态），但 CSS 只定义了 `--ineligible` 与 `--unknown`
的左边框强调色，`wk-evaluation-detail__rule--eligible` 落空，符合的硬性条件没有任何视觉标识——而同页
verdict（`__verdict--eligible`）和评估状态（`wk-evaluation-status--eligible`）都有绿色强调，同一页面内视觉语义不一致。

  .wk-evaluation-detail__rule--ineligible { border-left: 4px solid var(--td-error-color, #b42318); }
  .wk-evaluation-detail__rule--unknown { border-left: 4px solid var(--td-warning-color, #d68b00); }
+ .wk-evaluation-detail__rule--eligible { border-left: 4px solid var(--td-success-color, #2ba471); }


─── apps/web/src/career/preparation.css:107-107 ───
[bug · low] token 拼写错误：`--td-bg-color-secondary-container`（secondary 与 container 之间多了连字符）在
packages/design-tokens/src/tdesign-theme.css 中不存在，真实命名为
`--td-bg-color-secondarycontainer`（progress.css:65、application.css、material.css 等 10
处均使用正确名）。该变量永远不生效，回退色 #f5f5f5 被固定使用——暗色主题下（tdesign-theme.css 第 68 行将此 token 重定义为深灰）来源链区块背景将与其它
career 页面不一致。

-   background: var(--td-bg-color-secondary-container, #f5f5f5);
+   background: var(--td-bg-color-secondarycontainer, #f5f5f5);


─── apps/web/src/career/reconciliation.css:123-125 ───
[maintainability · low] 两个问题：1) 该规则缺少 :hover 伪类——inbox.css 的对应规则是 button:hover:not(:disabled)
才变绿，此处所有启用态按钮常态即绿边绿字（呈永久激活态），与收件箱交互不一致，疑似漏写 :hover；2) 本文件与 inbox.css 将品牌绿 #07c05f 以字面量重复硬编码（含
color-mix 中），design-tokens 已有 --wk-color-brand / TDesign --td-brand-color 映射，建议统一改用 token。

- .wk-reconciliation__actions button:not(:disabled),
- .wk-reconciliation > button:not(:disabled),
- .wk-coverage button:not(:disabled) { border-color: #07c05f; color: #07c05f; }
+ .wk-reconciliation__actions button:hover:not(:disabled),
+ .wk-reconciliation > button:hover:not(:disabled),
+ .wk-coverage button:hover:not(:disabled) { border-color: var(--td-brand-color, #07c05f); color: var(--td-brand-color, #07c05f); }


─── apps/web/src/career/reconciliation.tsx:58-58 ───
[bug · high] 与 InboxPage 同款的家族性缺口：unknown 对账恢复所依赖的 attempt/requestId 仅存于组件
state，无持久化；而恢复文案承诺「对账回执暂时无法读取。原请求编号已保留。」（第 187
行）——刷新或离开页面后该编号不可恢复，用户只能换新编号重新判定（对账历史中留下重复判定记录）。与门禁重点「unknown request-ID 持久化」不符，建议按 per-scope
存储持久化待恢复请求并在验收后清除。

   const [attempt, setAttempt] = useState<{ requestId: string; targetId: string; candidateId: string }>()
+  // 建议：进入 'unknown' 时将 attempt 持久化（per-scope sessionStorage），
+  // 挂载时读回校验 scope 后恢复；receipt 验收成功或确定性失败时清除。


─── apps/web/src/career/reconciliation.tsx:104-104 ───
[bug · low] 当用户将旧/新快照选择为同一 snapshotId 时，本分支将 diffPhase 置 'idle'，但渲染端的三元链没有 idle
分支——落入默认的「正在读取两个固定快照…」，界面永久显示加载中而实际不会发起任何请求，误导用户等待。两个 select 均可任选 index，同一快照是用户可达状态。建议为「同快照」渲染明确提示。

-   if (!oldObservation || !newObservation || oldObservation.snapshotId === newObservation.snapshotId) { setDiffPhase('idle'); return () => { active = false } }
+ // 渲染端补 idle/同快照分支：
+ // : oldObservation && newObservation && oldObservation.snapshotId === newObservation.snapshotId
+ //   ? <p role="status">已选择同一快照，请选择两个不同的快照进行对比。</p>
+ // : <p role="status">正在读取两个固定快照…</p>


─── apps/web/src/career/reconciliation.tsx:183-183 ───
[style · low] phase → 文案的链式三元（error ? … : forbidden ? … : …）在本文件多处出现（本行、CareerCoveragePanel 第 282
行、快照对比分支等），违反「禁止嵌套三元」规范；建议提取为映射函数或早返回辅助函数，提高可读性。

-  if (phase !== 'ready' || !status) return <section className="wk-reconciliation"><h2>岗位状态与对账</h2><p role={phase === 'error' ? 'alert' : 'status'}>{phase === 'error' ? '岗位状态暂时无法读取。' : phase === 'forbidden' ? '当前空间不可访问此岗位。' : '空间已切换，已清除岗位状态。'}</p>{phase === 'error' ? <button type="button" onClick={retry}>重试</button> : null}</section>
+ function panelNotice(phase: 'error' | 'forbidden' | 'scope-changed'): string {
+  if (phase === 'error') return '岗位状态暂时无法读取。'
+  if (phase === 'forbidden') return '当前空间不可访问此岗位。'
+  return '空间已切换，已清除岗位状态。'
+ }
+ // JSX: <p role={phase === 'error' ? 'alert' : 'status'}>{panelNotice(phase)}</p>


─── apps/web/src/career/rule.css:65-65 ───
[bug · low] 暂停态取 var(--td-warning-color-7, #e37318)：全局 tdesign-theme.css 中 --td-warning-color-7 实际为
#ba431b（深红棕），回退值 #e37318 在 token 注入后永远不会生效——实际渲染色与橙色警示意图不符；而 export-deletion.css 同类警示用的是
--td-warning-color（#ed7b2f）。两处'警示色'取了不同色阶且回退值彼此不同，呈现不一致。建议统一为 --td-warning-color 并让回退值与 token 实际值对齐。

- .wk-career-rule__plan--paused { color: var(--td-warning-color-7, #e37318); }
+ .wk-career-rule__plan--paused { color: var(--td-warning-color, #e37318); }


─── apps/web/src/career/rule.css:12-12 ───
[maintainability · low] 品牌色 #07c05f 在本文件多处硬编码：eyebrow、focus-visible 描边
rgba(7,192,95,.45)、__run-status--completed、__todos a、__basis 左边框。design-tokens 已提供
--td-brand-color（=#07c05f）与 --wk-color-brand，品牌色调整时这些散落值不会跟随（export-deletion.css / usage.css
的同款问题已另列）。建议统一改为 var(--td-brand-color, #07c05f)。

- .wk-career-rule__header .wk-career-rule__eyebrow { margin: 0; color: #07c05f; font-size: .8rem; font-weight: 600; letter-spacing: .04em; }
+ .wk-career-rule__header .wk-career-rule__eyebrow { margin: 0; color: var(--td-brand-color, #07c05f); font-size: .8rem; font-weight: 600; letter-spacing: .04em; }


─── apps/web/src/career/submission.css:92-92 ───
[maintainability · low] 品牌绿 #07c05f 硬编码未走 token（tdesign-theme.css 已定义 --td-brand-color =
#07c05f，design-tokens 有 --wk-color-brand），警示橙 var(--td-text-color-warning, #e37318) 用了
text-color-warning 而同目录 material.css/application.css 用 #ffb648/#b45309 系，三文件警示色体系不统一；focus 回退
#0052d9 为 TDesign 默认蓝与品牌绿冲突。容器卡、hint、label、message、按钮、actions、focus-visible 等基础样式与
application.css/material.css 大面积重复，建议 token 化 + 提取共享 career-base.css。

- .wk-submission__submit { border-color: #07c05f; background: #07c05f; color: #fff; }
+ .wk-submission__submit { border-color: var(--td-brand-color, #07c05f); background: var(--td-brand-color, #07c05f); color: #fff; }


─── apps/web/src/career/submission.css:82-89 ───
[maintainability · low] submission.css 没有任何 :focus-visible 规则：同族的 application.css 与 material.css 都为
button/input/textarea 定义了 3px outline 的键盘焦点样式，本文件的 select（datetime
输入、两个下拉）与全部按钮只能依赖浏览器默认焦点环，三个文件键盘可达性表现不一致。建议补充与同族一致的 :focus-visible 规则（含 select）。

- .wk-submission button {
-   border: 1px solid var(--td-component-border, #dcdcdc);
-   border-radius: 6px;
-   padding: 8px 14px;
-   background: var(--td-bg-color-container, #fff);
-   color: var(--td-text-color-primary, #222);
-   cursor: pointer;
- }
+ .wk-submission button:focus-visible,
+ .wk-submission select:focus-visible,
+ .wk-submission input:focus-visible,
+ .wk-submission textarea:focus-visible { outline: 3px solid var(--td-brand-color-focus, #0052d9); outline-offset: 2px; }


─── apps/web/src/career/usage.css:14-14 ───
[maintainability · low] 品牌色 #07c05f 直接硬编码（本文件 eyebrow、__basis 左边框同此）。design-tokens 已定义
--td-brand-color（=#07c05f）与 --wk-color-brand，建议收敛到 token 变量（保留原值为回退），与 export-deletion.css /
rule.css 的同类问题一并治理，避免品牌色调整时多处散落不同步。

- .wk-career-usage__conditions li { border-left: 2px solid #07c05f; padding-left: 8px; }
+ .wk-career-usage__conditions li { border-left: 2px solid var(--td-brand-color, #07c05f); padding-left: 8px; }


─── apps/web/src/career/usage.css:18-19 ───
[maintainability · low] [role='alert'] 的错误强调样式只作用于 .wk-career-usage 后代，但 UsagePanel.tsx 的 forbidden
/ unavailable 两个分支（最需要视觉警示的『空间不可访问』『预估不可用、阻止收费』状态）外层是 className="wk-career-usage-wrap" 且内部 Card 未带
wk-career-usage 类——选择器命中不了，左边框红色警示丢失；同时 wk-career-usage-wrap 在全仓库没有任何样式定义。loading 分支的 __status 与
__actions 是独立类选择器所以能命中，唯独 alert 选择器有此作用域错位。建议把两条选择器放宽到 .wk-career-usage-wrap（四个分支通吃）。

- .wk-career-usage [role='alert'] { border-left: 3px solid var(--td-error-color, #d54941); padding-left: 10px; margin: 10px 0; }
- .wk-career-usage [role='alert'] p { margin: 6px 0 0; color: var(--td-text-color-secondary, #555); }
+ .wk-career-usage-wrap [role='alert'] { border-left: 3px solid var(--td-error-color, #d54941); padding-left: 10px; margin: 10px 0; }
+ .wk-career-usage-wrap [role='alert'] p { margin: 6px 0 0; color: var(--td-text-color-secondary, #555); }


─── apps/web/src/chat/ChatRoutePage.tsx:1929-1932 ───
[maintainability · low] `sessions.find((session) => session.id === selectedSessionId)` 在 headerSlot
中连续重复三次（title/isPinned/renameTitle），聊天页流式输出期间高频重渲染会反复扫描会话数组，且三处表达式未来易发生不一致。已核实
renameSession/toggleSessionPin/deleteSession/clearMessages 回调与 copy 文案键均存在，仅剩重复问题。建议提取一次查找结果复用。

-       <ChatHeader
-         copy={copy}
-         title={sessions.find((session) => session.id === selectedSessionId)?.title || copy.newSession}
-         isPinned={sessions.find((session) => session.id === selectedSessionId)?.is_pinned === true}
+ const activeSession = useMemo(
+   () => sessions.find((session) => session.id === selectedSessionId),
+   [sessions, selectedSessionId],
+ );
+ // …
+ title={activeSession?.title || copy.newSession}
+ isPinned={activeSession?.is_pinned === true}
+ renameTitle={activeSession?.title || copy.newSession}


─── apps/web/src/chat/ChatRoutePage.tsx:2024-2024 ───
[maintainability · medium] chat 路由静态 import ../career/OpportunityPage.tsx（其内部再引入
ApplicationPage/reconciliation/career-core/api-client career 等子树），把整个 career UI 拉进 chat 路由
chunk：router.tsx 中对 career 页面的 lazy 代码切分被此静态入口绕过，并形成 chat→career 的跨域耦合。同时 conversationActionSlot 在
page.tsx 空态与会话视图两处（647/748 行）均无条件渲染该面板，未见租户/feature 门禁——无权限租户会先看到入口、触发后端 forbidden 后才收到清理提示（面板内
clearPrivate 仅是兜底）。建议改为 lazy 加载并在挂载前做门禁判定。

-     conversationActionSlot={<OpportunityImportPanel client={client} scopeController={scopeController} />}
+ const OpportunityImportPanel = lazy(() => import('../career/OpportunityPage.tsx').then((m) => ({ default: m.OpportunityImportPanel })));
+ // …
+ conversationActionSlot={careerEnabled ? (
+   <Suspense fallback={null}>
+     <OpportunityImportPanel client={client} scopeController={scopeController} />
+   </Suspense>
+ ) : undefined}


─── apps/web/src/chat/SessionShareDialog.tsx:6-7 ───
[documentation · low] 注释尾部仍声称“这里全部用内联 Tailwind utilities”，但本组件类名（含 SECONDARY/PRIMARY/DANGER_BUTTON
常量）已全部替换为 chat-u.css 的 wk-ssd-* 语义类，描述已过时，会误导后续维护者对样式来源与依赖约束的判断，建议同步修正。

- // 刻意不依赖 packages/ui 旧栈：PlatformShell 在每个 platform 页面与全部 node
- // 测试里直接 import 本模块，而 packages/ui 旧栈 的 theme.css 会破坏 node 下的
+ // 模块加载（router.tsx 对 craft 的 lazy 处理同理）——样式走本域 chat-u.css 的
+ // wk-ssd-* 语义类（QueryHistorySnapshotDrawer 抽屉同风格）。


─── apps/web/src/chat/chat-header.tsx:157-161 ───
[bug · medium] 标题编辑输入同时存在 Escape 取消与 onBlur 自动提交双出口：按 Escape 后 cancelTitleEdit() 触发输入框卸载，部分浏览器/JSDOM
在卸载聚焦元素时仍会派发 blur，此时 onBlur 闭包中的 titleEditing 仍为 true、renameSubmittingRef 已在 finally 复位，会用旧
titleDraft 再次调用 onRenameSession——用户明确取消的改名仍被提交；Enter 提交成功后卸载输入框同理可能造成二次提交。建议引入取消意图 ref 守卫。

+ const cancelEditRef = useRef(false);
+ // …
-             onKeyDown={(event) => {
+ onKeyDown={(event) => {
-               if (event.key === 'Escape') { event.preventDefault(); cancelTitleEdit(); }
+   if (event.key === 'Escape') { event.preventDefault(); cancelEditRef.current = true; cancelTitleEdit(); }
-               if (event.key === 'Enter') { event.preventDefault(); void submitTitleEdit(); }
+   if (event.key === 'Enter') { event.preventDefault(); void submitTitleEdit(); }
-             }}
+ }}
-             onBlur={() => { void submitTitleEdit(); }}
+ onBlur={() => {
+   if (cancelEditRef.current) { cancelEditRef.current = false; return; }
+   void submitTitleEdit();
+ }}


─── apps/web/src/chat/chat-header.tsx:175-179 ───
[maintainability · low] onVisibleChange 中 `context?.trigger === 'document'` 分支与兜底路径执行完全相同的
onMenuVisibleChange(visible)，属冗余死分支——注释声称的差异处理并未体现。要么删除该 if，要么在分支内做真正差异化的逻辑。

-           onVisibleChange={(visible, context) => {
-             // trigger click 以外（Esc/外点）的关闭同样回到 menu 态
-             if (context?.trigger === 'document') { onMenuVisibleChange(visible); return; }
+           onVisibleChange={(visible) => {
+             // Esc/外点等任意关闭路径同样回到 menu 态
              onMenuVisibleChange(visible);
            }}


─── apps/web/src/chat/chat-header.tsx:125-126 ───
[maintainability · low] menuMode 类型为 'menu' | 'clear' | 'delete'，`!menuMode` 恒为 false，该守卫条件形同虚设；若未来在
menu 态误触发本函数，会落入 else 分支直接调用 onDeleteSession（删除会话）。建议显式排除 menu 态。

    async function submitDangerAction(): Promise<void> {
-     if (!menuMode || dangerBusy) return;
+     if (menuMode === 'menu' || dangerBusy) return;


─── apps/web/src/chat/chat-header.tsx:188-188 ───
[maintainability · low] 虽有 `props.onTogglePin ?` 条件守卫，仍建议先把可选回调收进局部变量再调用，避免非空断言
`props.onTogglePin!(...)`（守卫与断言之间隔着 JSX，重构时易脱钩）；另外 h1 的 title 属性经 titleDisplay 回退
copy.newSession，而可见 span 直接渲染 {props.title}，空标题时 tooltip 有文案、正文为空，两处不一致——宿主已传入带回退的
title，titleDisplay 实际冗余，可删除或让 span 同样走回退。

-                 {props.onTogglePin ? <button type="button" className="chat-header-menu__item" data-menu-action="pin" onClick={() => { setMenuVisible(false); props.onTogglePin!(!props.isPinned); }}>
+                 {props.onTogglePin ? <button type="button" className="chat-header-menu__item" data-menu-action="pin" onClick={() => { setMenuVisible(false); props.onTogglePin?.(!props.isPinned); }}>


─── apps/web/src/chat/chat-u.css:201-209 ───
[maintainability · low] `border-top-width: 1px;border-color: #eef1f5` 用全边 border-color 简写配合单边
width（同模式还有本文件 wk-ssd-14 及 views-chat-u.css 的
wk-vc-page-32/-42、wk-vc-agent-selector-4/-24、wk-vc-message-list-9/-15、wk-vc-session-sidebar-12
等）：当前其他边 width 为 0 尚无视觉影响，但元素一旦再声明其他边框宽度即四边串色；两声明挤在一行也降低可读性。建议统一改用单边 border-top-color/bottom-color
并分行书写。

  .wk-bad-19 {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    border-top-style: solid;
-   border-top-width: 1px;border-color: #eef1f5;
+   border-top-width: 1px;
+   border-top-color: #eef1f5;
    padding-top: 12px;
  }


─── apps/web/src/chat/views-chat-u.css:1697-1702 ───
[bug · medium] 迁移源是 Tailwind `border-b-2`（= 2px，见 tool-result.tsx 原 thead th 类 `whitespace-nowrap
border-b-2 border-b-[#e3e8ef] …`），此处误写为 border-bottom-width: 8px。数据库查询工具结果的表头单元格会渲染出 8px
粗底边框，属视觉回归，应改回 2px。

  .wk-vc-tool-result-6 {
    white-space: nowrap;
    border-bottom-style: solid;
-   border-bottom-width: 8px;
+   border-bottom-width: 2px;
    border-bottom-color: #e3e8ef;
    background-color: #f6f8fa;


─── apps/web/src/chat/views-chat-u.css:220-233 ───
[maintainability · low] 本文件存在系统性死声明/重复定义：(1) 此处 font-size 先 inherit 后 13px 双写，前者恒被覆盖；(2)
wk-vc-page-52 及 composer-18/20/22/24 等多处 transition-duration 先 150ms 再 200ms/120ms 双写；(3) @keyframes
pulse 在 page 段与 message-list 段重复定义两次；(4) wk-vc-tool-result-2/8/14/15/24/25 以普通声明开头、同属性以 !important
结尾，前段均为死代码。虽是 utilities 迁移的忠实还原，但显著增加维护噪声，建议清理被覆盖的死声明并将 keyframes 收敛为单处定义。

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


─── apps/web/src/commercial/BillingPage.tsx:55-55 ───
[documentation · low] 本次将 USAGE_TABLE / USAGE_TABLE_CELL / USAGE_NUMBER_CELL 的值从 tailwind utilities
串改为语义类名后,紧邻的注释「与 UsagePanel 模型表格同款样式(tailwind utilities)」已失真:这些常量不再是 tailwind utilities,且同款样式如今依赖
commercial-u.css 中的 .wk-bill-* 规则与 UsagePanel 的实现保持同步(而非共享同一 utilities 值)。建议同步更新该注释,说明样式来源已迁至
commercial-u.css,避免后来者按注释去找不存在的 utilities。

+ // 与 UsagePanel 模型表格同款样式（样式值已语义化到 ./commercial-u.css 的 .wk-bill-* 规则）。
  const USAGE_TABLE = 'wk-bill-usage-table';


─── apps/web/src/commercial/commercial-u.css:19-21 ───
[maintainability · low] `.wk-bill-usage-number-cell` 在本文件第 41 行被第二次定义（font-variant-numeric:
tabular-nums），同一选择器分散两处，后续修改极易漏改其一；另 `.wk-bill-usage-table-cell` 内 `border-bottom-width:
1px;border-color: #eef1f5;` 两个声明挤在同一行且未用简写，与文件其余一声明一行的风格不一致。建议合并为单一规则并使用 border-bottom 简写（删除文末第 41
行的重复规则）；顺带建议将 wk-bill-1 / wk-bill-2（thead 行底描边 / 表头加粗）改成语义化命名，降低跨文件复用时的理解成本。

+ .wk-bill-usage-table-cell {
+   border-bottom: 1px solid #eef1f5;
+   padding-inline: 10px;
+   padding-block: 8px;
+   text-align: left;
+ }
+ 
  .wk-bill-usage-number-cell {
    text-align: right;
+   font-variant-numeric: tabular-nums;
  }


─── apps/web/src/commercial/surface.tsx:10-13 ───
[maintainability · low] Card/Status 与 shared/wk-legacy.tsx 的 WkCard/WkStatus（以及同批新建的
configuration/ui.tsx、data-sources/ui.tsx）逐行同构（section
元素、role=alert/status、tone→色值映射完全一致），仅类名前缀不同，现已形成 4 份并行拷贝；.wk-cs-status--*
的色值（#506078/#b42318/#137333/#9a6700）与 wk-legacy.css 逐字重复。后续任何 a11y 或色值修正都需同步 4 处，漂移风险高（.wk-card 描边
#dce3ed 与 .wk-cs-card 描边 #e7e7e7 已经分叉）。建议收敛到共享实现（如 re-export shared/wk-legacy
并参数化类名前缀），域内视觉差异（#e7e7e7 描边）改由本域 CSS 覆盖表达。



─── apps/web/src/commercial/surface.tsx:18-18 ───
[style · low] 嵌套三元表达式（项目规范禁止嵌套三元）。shared/wk-legacy.tsx:29 的同款组件用 tone 拼接类名后缀，行为一致且无嵌套，建议对齐。

-   const toneClass = tone === 'error' ? 'wk-cs-status--error' : tone === 'success' ? 'wk-cs-status--success' : tone === 'warning' ? 'wk-cs-status--warning' : '';
+   const toneClass = tone !== 'neutral' ? `wk-cs-status--${tone}` : '';


─── apps/web/src/commercial/surface.tsx:4-5 ───
[maintainability · low] Button 迁移后在域内 5 个文件共 13 处调用点逐字重复 `type="button" theme="default"
variant="outline"` 三连样板(AdminCommercialPage 5 处、RefundPage 3 处、BillingPage/CheckoutPage 各 2
处、TaskBudget 1 处)。注释本身已声明「本域调用点一律 theme=default variant=outline」,这正是应收敛为默认值的信号:若后续视觉基线调整(如改 primary
或调整尺寸),需同步修改 13 处且极易遗漏。建议在本文件导出一个带默认值的 Button 封装,调用点只保留业务属性(disabled/onClick/aria-label
等),个别需要不同主题的按钮仍可通过传参覆盖。

- // 处）的 utilities 逐字一致，视觉零变化。Button 一律走 tdesign-react（本域
- // 调用点 theme="default" variant="outline" 对应旧栈默认白底细描边）。
+ import { Button as TdButton } from 'tdesign-react';
+ import type { ButtonProps } from 'tdesign-react';
+ 
+ /** 域内按钮：默认即旧栈白底细描边（theme=default + variant=outline），可按需覆盖。 */
+ export function Button({ theme = 'default', variant = 'outline', ...props }: ButtonProps) {
+   return <TdButton theme={theme} variant={variant} {...props} />;
+ }


─── apps/web/src/commercial/surface.tsx:10-13 ───
[maintainability · low] Card 的 props 类型存在两处类型卫生问题:(1) 泛型参数标注为 HTMLAttributes<HTMLDivElement>,但实际渲染的是
<section> 元素,泛型语义与真实元素不一致,后续若补充 ref 支持(如 React 19 ref-as-prop)会直接产生 Ref<HTMLDivElement> 与 section
的类型错配;(2) HTMLAttributes 经 DOMAttributes 已内含 children?: ReactNode,交叉类型 & { children?: ReactNode }
是冗余声明。建议改用 HTMLAttributes<HTMLElement> 并去掉重复的 children 交叉类型,children 仍由 props 展开正常传递。

- export function Card({ children, className, ...props }: HTMLAttributes<HTMLDivElement> & { children?: ReactNode }) {
+ export function Card({ className, ...props }: HTMLAttributes<HTMLElement>) {
    const classes = ['wk-cs-card', className].filter(Boolean).join(' ');
-   return <section className={classes} {...props}>{children}</section>;
+   return <section className={classes} {...props} />;
  }


─── apps/web/src/configuration/ConfigurationPage.tsx:111-111 ───
[style · low] 该行为 4 层嵌套三元（errors → loading → agents → 空列表 →
列表渲染），虽是旧代码结构的忠实迁移，但整行为本次重写的新增代码，且违反项目检查项"禁止嵌套三元表达式"。连续条件渲染建议抽为独立渲染函数（如
renderSectionBody），既提升可读性，也降低后续插入分支时出优先级错误的风险。

-       {errors[section.key] ? <Status tone="error">{errors[section.key]}</Status> : loading ? <Status>Loading…</Status> : section.key === 'agents' && items.length > 0 ? renderAgentGroups() : items.length === 0 ? <Status>No configured entries.</Status> : <ul className="wk-list wk-cfg-page-5">{items.map((item, index) => renderConfigurationRow(section.key, item, index))}</ul>}
+ function renderSectionBody(sectionKey: ConfigurationSectionKey, items: ConfigurationRecord[]) {
+   if (errors[sectionKey]) return <Status tone="error">{errors[sectionKey]}</Status>;
+   if (loading) return <Status>Loading…</Status>;
+   if (sectionKey === 'agents' && items.length > 0) return renderAgentGroups();
+   if (items.length === 0) return <Status>No configured entries.</Status>;
+   return <ul className="wk-list wk-cfg-page-5">{items.map((item, index) => renderConfigurationRow(sectionKey, item, index))}</ul>;
+ }
+ // 使用：{renderSectionBody(section.key, items)}


─── apps/web/src/configuration/config-u.css:44-51 ───
[bug · medium] `.wk-cfg-ops-4 input` / `.wk-cfg-ops-4 textarea` 是后代元素选择器，但使用该类的两个表单（ModelDebugPanel
调试表单、SkillOperations 注册表单）在本次迁移后控件已是 tdesign-react 组件，会渲染出 `input.t-input__inner` /
`textarea.t-textarea__inner`（内层）+ `.t-input` / `.t-textarea`（带边框的包装层）。该选择器（特异性
0,1,1）会命中内层元素并叠加边框/底色/内边距，与 tdesign 包装层自身的 chrome 叠加，出现双描边、内边距翻倍等视觉回归；仅原生 `<input type="file">` 不经
tdesign 包装。settings 域在 settings-wrapper.css:738-744 对同类情况已有明确先例：边框/内距/底色交由 unlayered 的 tdesign
规则拥有，只平移实际生效的 width/height。建议按先例裁剪，或仅对原生 file input 保留完整视觉。

- .wk-cfg-ops-4 input {
+ /* tdesign 组件自带 .t-input/.t-textarea 包装层 chrome，内层元素仅平移布局尺寸
+    （先例：settings-wrapper.css .wk-mcp-editor-form 同款说明）。 */
+ .wk-cfg-ops-4 input,
+ .wk-cfg-ops-4 textarea {
    width: 100%;
    box-sizing: border-box;
-   border-style: solid;
-   border-width: 1px;
-   border-color: #cbd5e1;
+ }
+ 
+ /* 原生 file input 不经过 tdesign，保留原边框/内距视觉 */
+ .wk-cfg-ops-4 input[type='file'] {
+   border: 1px solid #cbd5e1;
    border-radius: 6px;
    background-color: #fff;
+   padding-inline: .65rem;
+   padding-block: .55rem;
+ }


─── apps/web/src/configuration/config-u.css:406-408 ───
[bug · medium] `.wk-cfg-card` 边框用了 #e7e7e7（td-component-stroke），但 ui.tsx 注释声称与旧 Card
"逐字一致，视觉零变化"、"视觉 = 既有 .wk-card"，而仍在生效的共享 `.wk-card`（wk-legacy.css，JoinPage/DocumentsPage/settings
等域在用）边框为旧 theme token 字面量 #dce3ed。两处声明不可能同时成立：配置页卡片将与其他页面的卡片呈现不同边框色（中性灰 vs
蓝灰），造成跨页面视觉分叉。若目标是"视觉零变化"应取 #dce3ed；若有意统一到 TDesign stroke，则需修正 ui.tsx/commercial 等处注释，并明确接受与
.wk-card 页面的差异。

- /* ui.tsx Card 壳：rounded-card border-line bg-surface p-4（theme token utilities）平移 */
-   /* 旧栈 line/line-soft→Vue var(--td-component-stroke)=#e7e7e7 */
- .wk-cfg-card { border-radius: 8px; border: 1px solid #e7e7e7; background-color: #ffffff; padding: 16px; }
+ /* ui.tsx Card 壳：rounded-card border-line bg-surface p-4 平移。
+    border-line = 旧 theme token --color-line（#dce3ed，与 shared/wk-legacy.css .wk-card 一致），
+    保持"视觉零变化"承诺。 */
+ .wk-cfg-card { border-radius: 8px; border: 1px solid #dce3ed; background-color: #ffffff; padding: 16px; }


─── apps/web/src/configuration/config-u.css:262-266 ───
[style · low] .wk-cfg-page-16 中 font-size 先声明 0.8rem 后被 0.85rem !important 覆盖、color 先 #6941c6 后被
muted !important 覆盖——这是对旧 utility 串（text-[0.8rem] text-[#6941c6] … text-[0.85rem]!
text-muted!）的忠实平移，但前两条声明在新 CSS
里是永远不生效的死代码，且让读者误以为徽标是紫色/0.8rem。建议只保留实际生效的声明；若为保留迁移审计痕迹，请用注释说明被覆盖值的存在原因。

-   font-size: 0.8rem;
-   color: #6941c6;
-   background-color: #f4f3ff;
-   margin-right: auto;
-   font-size: 0.85rem !important;
+   /* 旧 utility 串中 text-[0.8rem]/text-[#6941c6] 已被 text-[0.85rem]!/text-muted! 覆盖，
+      平移时省略死声明，仅保留生效值。 */
+   font-size: 0.85rem;
+   color: rgba(0, 0, 0, 0.6);


─── apps/web/src/configuration/ui.tsx:12-15 ───
[maintainability · low] 本域 Card/Status 与 shared/wk-legacy.tsx 已导出的 WkCard/WkStatus 重复，且和
commercial/surface.tsx、data-sources/ui.tsx 构成第三份逐字复制。复制当日值就已分叉（.wk-cfg-card 边框 #e7e7e7 vs .wk-card
#dce3ed），印证多点同步的维护成本：后续任一处修正都需要人肉同步三处以上。建议本域直接复用 shared/wk-legacy.tsx 的共享导出（类名 .wk-card/.wk-status
也是既有测试与消费方 CSS 的查询锚点），域内差异通过 className 传入。

- export function Card({ children, className, ...props }: HTMLAttributes<HTMLDivElement> & { children?: ReactNode }) {
-   const classes = ['wk-cfg-card', className].filter(Boolean).join(' ');
-   return <section className={classes} {...props}>{children}</section>;
- }
+ import { WkCard, WkStatus } from '../shared/wk-legacy.tsx';
+ 
+ // 域内别名，消费方 import 路径不变；类名复用 .wk-card/.wk-status 既有锚点。
+ export const Card = WkCard;
+ export const Status = WkStatus;


─── apps/web/src/data-sources/DataSourcesPage.tsx:446-446 ───
[bug · medium] 迁移到 TDesign 后所有 Input/Textarea 的 required 被移除，但校验兜底存在两处缺口：(1) rss 的 settings 字段
feed_urls（VUE_SETTINGS_FIELDS 中无 optional 标记，旧代码 required={!field.optional} 生效）在 save()
中无任何走查——firstMissingRequiredCredential 对 rss 直接返回 null（无凭据字段映射），且 line 219 处无 authHeaders
时连接测试也被跳过，空 feed_urls 现在可直接提交生成无效数据源；(2) name/type/schedule 虽有 save() 兜底（line 188），但 setMessage
渲染在主页面 Card 内、位于打开中的 Drawer 遮罩之后，用户在抽屉内点保存后看不到任何反馈（旧代码原生 required 气泡在表单内可见），表现为"保存按钮无响应"。建议：为
VUE_SETTINGS_FIELDS 增加与凭据平行的必填走查，并把校验错误反馈渲染到 Drawer 表单内（或将 message Status 提升进 Drawer）。

-       <label>{t('dataSource.nameLabel')} <Input value={form.name} onChange={(value) => updateForm('name', String(value))} /></label>
+ // save() 中与凭据走查平行：settings 必填走查
+ const missingSetting = VUE_SETTINGS_FIELDS[form.type]?.find((field) => !field.optional && !credentialValue(form.settingsText, field.key));
+ if (missingSetting) {
+   setMessage({ tone: 'warning', text: `${t(missingSetting.label)} ${t('dataSource.isRequired')}` });
+   return;
+ }


─── apps/web/src/data-sources/DataSourcesPage.tsx:441-441 ───
[bug · medium] wk-ds-self-start（原 Tailwind justify-self-start）在本域 data-sources-u.css 中无定义，全仓唯一定义位于
knowledge-u.css:700，而该文件仅由懒加载的 KnowledgeGraphPage chunk 导入（router.tsx:67）。本页经 KB
设置或独立路由访问时若未走过知识图谱页，该类不存在，prereq 面板中的"打开控制台"链接在 .wk-ds-8 网格里会被默认 stretch 拉伸占满整行。这与本域 CSS 尾注自述已修复的
.wk-dsui-card"误置于懒加载 CSS"（OCR R1-04/31）是同一类 bug，且已被 OCR R1-R4 多轮标记仍未迁入。建议把定义迁入 data-sources-u.css 并从
knowledge-u.css 移除。

-           {guide.permissionPageUrl ? <a className="wk-ds-15 wk-ds-self-start" href={guide.permissionPageUrl} target="_blank" rel="noopener">{prereqCopy(t, `dataSource.prereqOpenConsole_${form.type}`, 'dataSource.prereqOpenConsole')}<span aria-hidden="true">↗</span></a> : null}
+ /* data-sources-u.css 手工段（并从 knowledge-u.css:700 移除） */
+ .wk-ds-self-start { justify-self: start; }


─── apps/web/src/data-sources/DataSourcesPage.tsx:475-477 ───
[bug · low] 删除确认 Dialog 的失败分支体验：confirmDelete 失败时 deleteSource 不重置，弹窗保持打开、loading 结束，而 setMessage
的错误文本渲染在 Dialog 遮罩之后的主 Card 内——模态场景下用户完全无法感知失败原因，只能看到"按钮恢复但弹窗还在"。此行为自旧 Sheet 沿袭而来（非本次回归），但该面板已重写为居中
Dialog（closeOnOverlayClick=false、提交期间 cancel 禁用），建议在 Dialog body 内渲染错误反馈（或失败时收进 Dialog 顶部一条错误行），与 S5
对齐 Vue t-dialog 的交互闭环。

-     closeOnOverlayClick={false}
-     width={440}
-     onClose={() => setDeleteSource(null)}
+ // confirmDelete 的 catch 分支同时写入一个 Dialog 内可见的错误 state，例如：
+ // const [deleteError, setDeleteError] = useState<string | null>(null);
+ // catch (error) { setDeleteError(...) }
+ // 并在 Dialog body 顶部渲染 {deleteError ? <p className="wk-dsui-status wk-dsui-status--error" role="alert">{deleteError}</p> : null}


─── apps/web/src/data-sources/DataSourcesPage.tsx:451-451 ───
[maintainability · low] cancel-replace 按钮残留空 className=""，是旧 justify-self-start 工具类被移除（迁为
wk-ds-self-start，见另一条评论）后的迁移残留物，建议直接删除该属性。

- {editing && credentialStep.replaceMode ? <Button type="button" theme="default" variant="text" size="small" className="" onClick={() => { setCredentialStep((current) => credentialStepReducer(current, 'cancel-replace')); setForm((current) => ({ ...current, credentialsText: '', authHeaders: [] })); }}>{t('common.cancel')}</Button> : null}
+ <Button type="button" theme="default" variant="text" size="small" onClick={() => { ... }}>


─── apps/web/src/data-sources/data-sources-u.css:27-29 ───
[maintainability · low] .wk-ds-2（以及 .wk-ds-42、.wk-ds-77 同型块）中 transition-duration: 150ms 紧跟
transition-duration: 200ms，前者是恒被覆盖的死声明（Tailwind transition-* 默认 150ms 与 duration-200 的级联残留）。文件头自述"值
= utilities 编码的生效值"，生效值即 200ms，建议三处统一删除 150ms 行，避免读者误以为存在某种级联意图。

    transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
-   transition-duration: 150ms;
    transition-duration: 200ms;


─── apps/web/src/data-sources/ui.tsx:4-5 ───
[documentation · medium] 注释存在两处与事实不符：(1) 引用的 packages/ui/src/index.tsx:29-37 已随本变更删除（@weknora/ui
旧栈整体拆除），成为悬空引用；(2) "逐字一致，视觉零变化"与实际分叉矛盾——.wk-dsui-card 边框为 #e7e7e7（Vue --td-component-stroke），而共享层
wk-legacy.css 的 .wk-card 为 #dce3ed（原 packages/ui --color-line token，已由
NotFoundPage/JoinPage/DocumentsPage 等复用），同一应用内出现两种卡片描边，且这是代码库中第 4 份 Card/Status
域内副本（shared/wk-legacy、configuration、commercial 之外再添一份）。建议：优先复用 shared/wk-legacy.tsx 的
WkCard/WkStatus 消除重复与漂移；若 #e7e7e7 是对齐 Vue token 的有意分叉，请在注释中显式说明与
.wk-card（#dce3ed）的差异原因，并移除对已删除文件的引用。

- // 组件——保留原生标签 + 既有类名；类值与 packages/ui/src/index.tsx:29-37
- // （Card/Status 合并定义处）的 utilities 逐字一致，视觉零变化。
+ // 优先复用共享兼容层：
+ // import { Card, Status } from '../shared/wk-legacy.tsx';
+ // 若保留域内实现，注释应改为：
+ // 边框取 Vue --td-component-stroke #e7e7e7（有意与 shared .wk-card 的 #dce3ed 分叉，对齐 Vue token）；
+ // 视觉 = 旧栈 utilities 生效值，Status 值与 .wk-status 族一致。


─── apps/web/src/data-sources/ui.tsx:18-18 ───
[style · low] toneClass 使用三层嵌套三元链，违反"嵌套三元不允许"的代码规范；DataSourcesPage.tsx 中
statusToneClass/syncResultToneClass 也是同型嵌套三元（本次仅改了返回值）。建议统一改为映射对象查表，可读性更好且便于后续扩展 tone。

-   const toneClass = tone === 'error' ? 'wk-dsui-status--error' : tone === 'success' ? 'wk-dsui-status--success' : tone === 'warning' ? 'wk-dsui-status--warning' : '';
+ const STATUS_TONE_CLASS: Record<'neutral' | 'error' | 'success' | 'warning', string> = {
+   neutral: '',
+   error: 'wk-dsui-status--error',
+   success: 'wk-dsui-status--success',
+   warning: 'wk-dsui-status--warning',
+ };
+ const toneClass = STATUS_TONE_CLASS[tone];


─── apps/web/src/documents/DocumentsPage.tsx:137-137 ───
[bug · low] `load` 是 useCallback 包装的函数引用，`() => void load` 仅求值后丢弃引用、并未调用 —— Reload 与 Try again
两个按钮点击后都不会发起列表请求（对比同文件正确写法 `() => void upload()`）。属本次改动行上沿袭的既有缺陷，建议补上调用括号。

-       <TButton type="button" onClick={() => void load} disabled={loading}>Reload</TButton>
+       <TButton type="button" onClick={() => void load()} disabled={loading}>Reload</TButton>


─── apps/web/src/documents/DocumentsPageChrome.tsx:132-132 ───
[maintainability · low] 不可达分支 + 死常量：该空态渲染位于 `kbList.length` 为真的分支内，而 sortedKbList
两个排序分支都保持源数组长度（≥1），`!sortedKbList.length` 恒为 false，kb-switcher-empty 永远不渲染；另外 TIcon 替换后本文件顶部
Chevrons 常量已无任何消费点。两处死代码可一并清理。



─── apps/web/src/documents/KnowledgeDocumentDetailPage.tsx:176-183 ───
[bug · medium] chunks 预取与文档详情请求并行、消费时机不确定：若文档先返回，DocumentChunks 挂载时 seed 仍为 null，走子组件 load(1) ——
每次打开抽屉对同一 /chunks 接口发起两份请求；晚到的 seed 永不被消费（子组件消费 effect 的 deps 仅 [client, document.id]，seed
到达不会重跑），长期滞留父 state，重挂载时还可能注入过期第 1 页数据覆盖当前分页态。另外 `.catch(() => {})` 把预取失败静默吞掉，徽章缺数时无法排障。建议让子组件订阅
seed（加入 deps、到达即消费），或收敛为单端请求，并在 catch 中至少记录日志。

-     void client.knowledgeBases.documents.chunks(documentId, 1).then((result) => {
-       if (active) {
-         setChunksTotal(result.total);
-         setChunkSeed({ chunks: result.data, total: result.total });
-       }
-     }).catch(() => {});
-     return () => { active = false; };
-   }, [client, documentId, loadAttempt]);
+     }).catch((error) => { console.warn('[doc-detail] chunks prefetch failed', error); });


─── apps/web/src/documents/KnowledgeDocumentsPage.tsx:4220-4220 ───
[bug · high] className 拼接缺空格且类名体系错位：stageNoticeClass(tone) 返回 `wk-documents-toast
${tone}`（无尾随空格），与字面量直接拼成 `errorwk-stage-notice` 这类无效类；同时 `wk-documents-toast` 全仓无任何 CSS 定义，裸 tone
类（neutral/success/…）也不匹配 documents.td.css 中的 `.wk-stage-notice.is-*` 选择器。三重错位叠加，全局上传/解析状态 toast
完全失去固定定位与配色（浮在文档流内挤压布局）。建议直接输出 td.css 定义的 `wk-stage-notice is-${tone}` 类族，并删除 stageNoticeClass。

-       {stageNotice ? <div className={`${stageNoticeClass(stageNotice.tone)}wk-stage-notice`} role="alert" aria-live="polite">{stageNotice.text}</div> : null}
+ {stageNotice ? <div className={`wk-stage-notice is-${stageNotice.tone}`} role="alert" aria-live="polite">{stageNotice.text}</div> : null}


─── apps/web/src/documents/KnowledgeDocumentsPage.tsx:2769-2774 ───
[maintainability · low] 死代码：toast className 重写后 STAGE_NOTICE_TONE_CLASS 已无任何引用点（全仓仅剩定义处），其映射的
wk-kd-notice-* 工具类随之全部失效。建议随上一条修复一并删除，避免误导后续维护者以为 tone 配色仍生效。



─── apps/web/src/documents/KnowledgeDocumentsPage.tsx:463-469 ───
[bug · medium] 可访问性回归：菜单项由 <button> 改为 div[role=menuitem]，无 tabIndex、无
onKeyDown，键盘用户完全无法聚焦/激活卡片与列表行的更多菜单；同类问题还有 move 确认视图的
div[role=radio]（无方向键/空格支持）、card-analyze-trace-link 的 span[role=button]，以及 DocumentsPageChrome 面包屑
tab 由 <a href> 改为 span[role=link]（丢失 ctrl/中键新开标签的原生行为且无 Enter/Space 处理）。建议交互元素恢复原生 button/a，或统一补
tabIndex 与键盘事件。

- const menuItem = (label: string, icon: ReactNode, handler: () => void, danger = false) => (
+   const menuItem = (label: string, icon: ReactNode, handler: () => void, danger = false) => (
-     <div
+     <button
+       type="button"
        className={'doc-action-menu-item' + (danger ? ' danger' : '')}
        role="menuitem"
        onClick={(event) => { event.stopPropagation(); close(); handler(); }}
-     >{icon}<span>{label}</span></div>
+     >{icon}<span>{label}</span}</button>
    );


─── apps/web/src/documents/KnowledgeDocumentsPage.tsx:4610-4617 ───
[bug · medium] 批量操作双重确认交互矛盾：批量删除 Popconfirm 的 onConfirm 仅 setConfirmingDelete(true)，随后又弹出文案几乎相同的确认
Dialog（confirmBatchDeleteDocument），用户需连续两次确认才真正执行 deleteSelected；重建路径同理：Popconfirm onConfirm →
reparseSelected() → setPendingBatchReparse 弹出 pendingBatchReparse Dialog → confirmBatchReparse
才发请求。属迁移叠加失误，建议 Popconfirm onConfirm 直接执行动作并删除对应 Dialog（或去掉 Popconfirm 保留 Dialog 单次确认）。

-                           <Popconfirm
-                             theme="warning"
-                             content={t("knowledgeBase.confirmBatchDeleteDocument", { count: selected.size })}
-                             confirmBtn={{ content: t("knowledgeBase.confirmDelete"), theme: "danger" }}
-                             cancelBtn={{ content: t("common.cancel") }}
-                             placement="top"
-                             onConfirm={() => setConfirmingDelete(true)}
-                           >
+                             onConfirm={() => void deleteSelected()}


─── apps/web/src/documents/KnowledgeDocumentsPage.tsx:517-522 ───
[bug · low] 卡片触发器的 open 态双通道驱动 + 非纯 updater：Popup（trigger=click + onVisibleChange 回写）与 .more-wrap 自身
onClick 手动 toggle 同时写同一 state，打开路径上 onMenuOpen 被两条通道各调一次（probeTrace 重复请求）；且手动 toggle 把 onMenuOpen
副作用写在 setState updater 内 —— updater 必须是纯函数，React 18 StrictMode 下会双调用导致副作用翻倍。建议移除 more-wrap 的
onClick，由 onVisibleChange 单通道控制（stopPropagation 移入触发器包装层），副作用放在事件处理器主体中。



─── apps/web/src/documents/KnowledgeDocumentsPage.tsx:4565-4565 ───
[maintainability · low] 业务数字硬编码：批量下载上限 200 为裸字面量，无常量提取亦无注释说明来源（后端限制？产品约定？）；同类问题还有
KnowledgeDocumentDetailPage.tsx 中分块分页页大小 25 散落多处（`(state.page - 1) * 25`、`state.total >
25`、`state.page * 25 >= state.total`），改动页大小时需多点同步。建议提取 BATCH_DOWNLOAD_MAX / CHUNK_PAGE_SIZE 常量并注明依据。



─── apps/web/src/documents/TagPickerDialog.tsx:278-284 ───
[maintainability · low] TagFilterPanel 重写后组件体内已不再引用 onClose 与 onClear（头部关闭按钮与“清除选中”footer 均被移除），但
props 接口保留且调用方仍传入，属死参数；同时用户失去一键清除标签筛选的显式入口（旧版在 selectedIds
非空时展示清除链接，而筛选触发器上的清除小图标仅在已有选中时才出现、可发现性差）。建议删除死参数，或恢复选中非空时的清除 footer。



─── apps/web/src/documents/documents-u.css:2704-2707 ───
[bug · medium] 无效颜色声明：`#07c05f]/4` 是 Tailwind 任意值 token（border-[#07c05f]/40）截断后泄漏进原生
CSS，整条声明被浏览器丢弃，.wk-kdd-101（视图切换按钮非激活态）hover 边框色静默失效，与像素对齐目标相悖。

- .wk-kdd-101:hover {
-   border-style: solid;
-   border-color: #07c05f]/4;
- }
+ border-color: rgb(7 192 95 / 40%);


─── apps/web/src/documents/documents-u.css:75-79 ───
[bug · medium] calc 运算符缺空格导致整条声明无效：此处 `calc(100vw-20px)`、以及 .wk-kd-29 与 .wk-kd-71 的 `calc(100%+4px)`
均不合法（CSS calc 中 + / - 两侧必须有空白符）。后果是卡片悬停气泡的 max-width 约束与各下拉弹层 top 偏移全部失效，小屏溢出/定位错位。与已修正注释的 .wk-kdd-6
属同一类 codemod 误译，建议全文件排查 `calc(` 无空格写法。

- .wk-kd-6 {
-   position: fixed;
-   z-index: 250;
-   width: 360px;
-   max-width: calc(100vw-20px);
+   max-width: calc(100vw - 20px);


─── apps/web/src/documents/documents-u.css:2087-2093 ───
[bug · low] blockquote 的 border-left-width: 8px 是 codemod 误译残留：原 Tailwind 为
border-l-2（=2px），与已修复注释（.wk-kdd-6 的 "border-2 → 2px 非 8px"）同类错误；即便注释声明要对齐 Vue 的 4px，8px 也与 2px/4px
都不符，实际渲染边条宽度翻倍。建议先改回 2px 保持迁移前后一致，Vue 4px 对齐随专项跟进。

- .wk-kdd-32 blockquote {
-   margin-block: 0.6em;
-   border-left-style: solid;
-   /* Vue markdown.less blockquote border-left: 4px var(--td-brand-color)（宽度 4px 差异另行跟进） */
-   border-left-width: 8px;border-color: #07c05f;
-   padding-left: 12px;
- }
+   border-left-width: 2px;border-color: #07c05f;


─── apps/web/src/documents/preview.ts:7-8 ───
[maintainability · low] documents 模块直接引入 chat 域 2500+ 行全局工具样式表 views-chat-u.css：与 chat
视图自身的导入形成重复注入，且实际生效取决于模块导入顺序（影响全局级联优先级）；同时形成跨域耦合 —— chat 侧调整工具类可能悄悄改变 documents 预览的渲染。建议将 preview.ts
实际依赖的规则下沉到 documents-u.css 或 shared 层，避免跨域全局样式互引。



─── apps/web/src/embed/EmbedEntryPage.tsx:826-826 ───
[bug · medium] 平移遗漏：apps/web 已无 Tailwind 构建来源（tailwindcss 仅在 apps/embed/package.json
中声明，apps/web/src/styles.css 与 vite.config.ts 均无 tailwindcss 引用/@source），残留的 `underline-offset-2`
类不会生成任何 CSS，是死类名。同元素原来的 `hover:underline` 已平移进 `.wk-emb-34:hover`，但配套的 `text-underline-offset: 2px`
未编码进 embed-u.css，链接 hover 下划线将紧贴文字（原 2px 偏移丢失），构成视觉回归。建议从 className 移除死类，并在 `.wk-emb-34` 中补写
`text-underline-offset: 2px;`。

-             className="underline-offset-2 wk-emb-34"
+             className="wk-emb-34"


─── apps/web/src/embed/embed-u.css:315-318 ───
[bug · medium] 平移失真：`.wk-emb-34`/`.wk-emb-35` 平移自 Tailwind `break-all`（v4 官方映射为 `word-break:
break-all`），但编码为 `overflow-wrap: anywhere`。二者断行语义不同：`word-break: break-all`
允许在任意字符间立即断行（行尾即断、行利用率高），`overflow-wrap: anywhere` 仅在单词放不下整行时才断行，长 URL/引用标题的折行位置会变化。同文件 `.wk-emb-22`
将 `break-words` 正确映射为 `overflow-wrap: break-word`，可见此处并非有意的规范化，而是失真。建议改回 `word-break: break-all;`
以保持视觉基线。

  .wk-emb-34 {
-   overflow-wrap: anywhere;
+   word-break: break-all;
    color: var(--embed-primary,#2563eb);
+   text-underline-offset: 2px;
  }


─── apps/web/src/embed/embed-u.css:225-226 ───
[bug · medium] 级联考古假设缺乏支撑：原 utility 为 `bg-[#f8fafc]`（utilities 编码值 #f8fafc），此处改为 #f3f3f3 的依据是"旧栈生效值被
Vue var(--td-bg-color-secondarycontainer) 覆盖"，但当前仓库中搜索不到任何匹配 `.embed-followups`
的背景规则（embed-chat.css、apps/web/src/styles.css、tdesign 样式均无）。若旧栈实际渲染即为 utilities 的
#f8fafc，这里就是可见的视觉变化（冷白调 → 灰调）。建议核对历史构建产物/截图确认旧栈真实渲染值；无法证实时应回归 utilities 原值 #f8fafc。

    /* 旧栈浅底→Vue var(--td-bg-color-secondarycontainer)=#f3f3f3 */
-   background-color: #f3f3f3;
+   background-color: #f8fafc; /* 若已确认旧栈确有 #f3f3f3 覆盖来源，请补充考古依据 */


─── apps/web/src/embed/embed-u.css:1-2 ───
[documentation · low] 头注释"导入顺序：须在本域 .td.css 之前"引用了不存在的文件：embed 域内检索不到任何 td.css（其他域如 chat/agents 均有对应
*.td.css，此处应为模板化注释残留）。这会误导后续维护者在 embed 域寻找/新增 td.css。建议改为指向真实存在的域样式（embed-chat.css、katex）。

  /* embed-u.css — S7 Tailwind utilities 平移（embed 域漂零段：EmbedEntryPage 等）
-  * 前缀见各规则；值 = utilities 编码的生效值。导入顺序：须在本域 .td.css 之前。 */
+  * 前缀见各规则；值 = utilities 编码的生效值。导入顺序：须在域样式（embed-chat.css、katex）之前。 */


─── apps/web/src/embed/embed-u.css:150-153 ───
[style · low] 冗余声明：`.wk-emb-15:hover` 与 `.wk-emb-29:hover` 中的 `border-style: solid` 是多余的——对应基类已声明
`border-style: solid`，且 Tailwind `hover:border-[#d8dde5]` 只生成 border-color 一条声明。建议删除 hover 规则中的
border-style，保持与被平移 utility 的一致性。

  .wk-emb-15:hover {
-   border-style: solid;
    border-color: #d8dde5;
  }


─── apps/web/src/embed/embed-u.css:29-30 ───
[style · low] 排版不一致：`border-bottom-width: 1px;border-color: #eef1f5;` 两条声明挤在同一行（`.wk-emb-31` 中同样存在
`border-top-width: 1px;border-color: #e7eaef;`），与本文件其余一声明一行的风格不一致，影响可读性。建议拆分为两行。

    border-bottom-style: solid;
-   border-bottom-width: 1px;border-color: #eef1f5;
+   border-bottom-width: 1px;
+   border-color: #eef1f5;


─── apps/web/src/experts/ExpertsPage.tsx:46-47 ───
[maintainability · medium] 跨域导入 `views-chat-u.css` 对本页并无实际用途:ExpertsPage 未使用任何 `wk-vc-*` 类(全文搜索 0
命中);对 views/chat 包的唯一依赖是纯函数 `renderChatMarkdown`,其输出类(wk-chat-citation / wk-chat-image-* /
language-*)在 views-chat-u.css 中均无样式定义(实际位于 chat.css 且限定在 .wk-chat-message-content 作用域内,本页 persona
区块也不在该作用域)。views-chat-u.css 文件头明确该文件"由各直接消费方 import",而本页并不渲染那 7 个迁移的聊天视图组件。此导入将 2546 行无关聊天 CSS 拉入
experts 页面依赖图,并形成 experts→chat 的虚假耦合(DevMarkdownPage.test.ts 已表明这类导入迫使测试运行器对 CSS 解析做 stub)。建议移除该导入;若
persona 区块确需聊天 markdown 样式,应显式引入对应样式而非整个 chat 域文件。



─── apps/web/src/experts/ExpertsPage.tsx:50-50 ───
[documentation · low] 迁移后常量注释已过时:第 49 行注释 "Tailwind v4 utility recipes shared across the page
(AnalyticsPage constants)" 及文件头第 3-4 行的 "Tailwind class constants" 描述的都是迁移前的形态;这些常量现在是指向
experts-u.css 的语义类名(wk-exp-xp-*),不再内联 Tailwind 工具类。保留过时注释会误导后续维护者继续按 Tailwind recipes 方式(如追加
`disabled:` 变体、Arbitrary values)拼接这些常量,而这类写法在纯 CSS 类上不会生效。建议同步更新两处注释,例如改为 "Semantic class constants
backed by ./experts-u.css (S7 utilities migration)"。



─── apps/web/src/experts/experts-u.css:66-70 ───
[maintainability · low] 原 Tailwind 语义 token(text-accent / bg-accent-wash /
bg-surface)在此被解析为裸色值(#07c05f / rgba(7,192,95,0.08) / #ffffff)。当前 styles.css @theme 事实源中
--color-accent/--color-surface 为常量,故无功能回归;但兄弟迁移文件 documents-u.css
统一保留间接引用(var(--wk-accent,#07c05f)、var(--wk-bg,#fff)),且本文件自身对话框(wk-exp-8/26/36)保留了 var(--wk-bg,#fff)
而卡片/页签却硬编码 #ffffff——约定不一致。一旦 token 事实源引入 var 间接引用(@theme 中 surface-hover 已是
var(--wk-color-surface-hover,#f3f3f3)),这些硬编码值将静默不跟随主题。建议统一为 var(--color-accent, #07c05f) 形式或与
documents-u.css 对齐同一约定。



─── apps/web/src/experts/experts-u.css:1-2 ───
[documentation · low] 文件头注释"导入顺序：须在本域 .td.css 之前"与实际不符:experts 域不存在任何 .td.css(file_find 仅见
experts-u.css),该顺序约束无从谈起,易误导后续维护者去寻找不存在的文件。另:文件头与文件尾("旧 T15 勘误注释…")存在两段内容重复的长注释;多处状态规则冗余重写未变化的
border-style(如 .wk-exp-xp-tab:hover 只需 border-color);wk-exp-9/22/34 中 `border-bottom-width:
1px;border-color: …` 挤在同一行。建议修正头注释、合并重复注释块并清理冗余声明。



─── apps/web/src/experts/experts-u.css:796-798 ───
[maintainability · low] .wk-exp-45(width:100%) 仅因在文件中晚于 .wk-exp-xp-input(width:200px)
声明而生效(两者同为单类特异性 (0,1,0))。TSX 中 XP_INPUT 的全部 3 处使用均追加了 wk-exp-45,故基类的 width:200px
实为永不生效的死值;未来若重排本文件规则顺序,输入框宽度会静默从 100% 回退为 200px。建议直接将基类宽度改为 100%(或删除基类 width),消除对声明顺序的隐式依赖。



─── apps/web/src/faq/FAQPage.tsx:1561-1561 ───
[bug · medium] i18n 键误用：`general.close` 在 i18n 目录中的文案是「关闭设置 / Close Settings」（设置面板语义，定义于
packages/i18n/src/settings.ts），用作导入弹窗关闭按钮的 aria-label 会让读屏器播报错误文案。同文件其余关闭按钮（如导入结果条的关闭按钮）均使用
`common.close`（「关闭 / Close」），此处应统一改为 `t('common.close')`。

-               <button className="close-btn" aria-label={t('general.close')} onClick={() => onCloseImport()}>
+               <button className="close-btn" aria-label={t('common.close')} onClick={() => onCloseImport()}>


─── apps/web/src/faq/FAQPage.tsx:1672-1672 ───
[bug · medium] 同上 i18n 键误用：批量标签弹窗关闭按钮的 aria-label 使用 `general.close`（实际文案「关闭设置 / Close
Settings」，语义不符），同文件既有关闭按钮均使用 `common.close`，应统一。

-               <button className="batch-tag-close-btn" aria-label={t('general.close')} onClick={() => onCloseBatchTag()}>
+               <button className="batch-tag-close-btn" aria-label={t('common.close')} onClick={() => onCloseBatchTag()}>


─── apps/web/src/faq/FAQPage.tsx:2352-2352 ───
[bug · medium] `common.operationFailed` 键在整个 i18n 目录中不存在（packages/i18n 仅定义了
`mobileConfiguration.operationFailed`），而 formatMessage 对缺失键回退为原始键名（packages/i18n/src/index.ts:1075
`?? key`）——单卡片改标签失败时 toast 将直接显示字面量 "common.operationFailed"。建议改用本文件已在用的 `common.error`，或先在 i18n
目录补齐该键（apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx 也在使用同一缺失键，可一并修复）。

-       setMessage({ tone: 'error', text: error instanceof Error && error.message ? error.message : t('common.operationFailed') });
+       setMessage({ tone: 'error', text: error instanceof Error && error.message ? error.message : t('common.error') });


─── apps/web/src/faq/FAQPage.tsx:1200-1200 ───
[bug · medium] 可访问性回归：卡片分区折叠标签由 `<button>`（原实现自带可聚焦与 Enter/Space 支持）改为裸 `<div
onClick>`。sectionButton 仍返回 aria-expanded，但 div 不可聚焦且无键盘事件，键盘用户无法折叠/展开 similar/negative/answers
分区。建议恢复 button 或补齐 role="button"、tabIndex={0} 与 onKeyDown。

-                             <div className="faq-section-label clickable" {...sectionButton(entry.id, name)}>
+                             <button type="button" className="faq-section-label clickable" {...sectionButton(entry.id, name)}>


─── apps/web/src/faq/FAQPage.tsx:1838-1838 ───
[bug · medium] 同上可访问性回归：检索测试结果行头部由 `<button aria-expanded>` 改为裸 `<div onClick>`，丢失了可聚焦性与
aria-expanded（原实现两者皆有），键盘用户无法展开/收起结果明细。建议恢复 button 或补齐 role/tabIndex/键盘事件。

-                 <div className="result-header" onClick={() => onToggle(result.id)}>
+                 <button type="button" className="result-header" aria-expanded={expanded} onClick={() => onToggle(result.id)}>


─── apps/web/src/faq/FAQPage.tsx:277-277 ───
[bug · low] FaqTagTooltip 本次删除了旧实现的点击切换兜底（原 onClick stopPropagation + toggle，注释标注用于 pointer/touch
场景），现在仅剩 hover 触发，触屏设备无法查看被截断标签的完整内容。若为对齐 Vue 事实源有意为之可忽略，否则建议补回 onClick 切换。



─── apps/web/src/faq/FAQPage.tsx:2345-2345 ───
[bug · low] `updateEntryTag` 对 `tagSeqId` 仅做 `Number()` 转换，未像同文件 `confirmBatchTag` 那样校验
`Number.isSafeInteger` 且非负。若上游传入非数字串（如某标签 seq_id 缺失时 String(undefined)），normalized 为 NaN，JSON 序列化为
null 后会意外清空该条目的标签。建议镜像 confirmBatchTag 的防护。

      const normalized = tagSeqId ? Number(tagSeqId) : null;
+     if (normalized !== null && (!Number.isSafeInteger(normalized) || normalized < 0)) return;


─── apps/web/src/faq/FAQPage.tsx:956-960 ───
[style · low] 嵌套三元表达式（项目规则禁止）：此处 activeTagFilterLabel 两层嵌套。同批新增代码中还有多处同款：importTask
图标名（running/success/failed 三层）、导入确认按钮文案（success?failed?:importButton）、FAQBreadcrumb 中
`showImportResultBadge && importResult ? … : importTask ? … : null`。建议改为早返回的辅助函数或映射对象。

-   const activeTagFilterLabel = activeTagIds.length === 0
-     ? (tagFilterCleared ? t('knowledgeBase.tagFilterPlaceholder') : t('knowledgeBase.allTags'))
-     : activeTagIds.length === 1
-       ? (tags.find((tag) => tag.id === activeTagIds[0])?.name ?? t('knowledgeBase.allTags'))
-       : t('knowledgeBase.tagFilterMulti', { count: activeTagIds.length });
+   const activeTagFilterLabel = (() => {
+     if (activeTagIds.length === 0) return tagFilterCleared ? t('knowledgeBase.tagFilterPlaceholder') : t('knowledgeBase.allTags');
+     if (activeTagIds.length === 1) return tags.find((tag) => tag.id === activeTagIds[0])?.name ?? t('knowledgeBase.allTags');
+     return t('knowledgeBase.tagFilterMulti', { count: activeTagIds.length });
+   })();


─── apps/web/src/faq/FAQPage.tsx:991-991 ───
[maintainability · low] 死分支：`faqExportOptions` 只会产生 'export_csv' / 'export_json' 两个值，`case 'export'`
永远不会被命中（疑似旧自绘导出菜单的遗留），建议删除。



─── apps/web/src/faq/FAQPage.tsx:2481-2481 ───
[bug · low] 批量删除失败时选中集仍被清空：removeMany 内部 catch 吞掉错误后正常 resolve，`.then(() => setSelected(new Set()))`
照常执行，删除失败的用户需要全部重选。建议仅成功后清空（参照 confirmBatchTag 只在成功路径清空 selected 的做法）。

-       onBatchDelete={() => void removeMany([...selected]).then(() => setSelected(new Set()))}
+       onBatchDelete={() => void removeMany([...selected]).then((ok) => { if (ok) setSelected(new Set()); })}


─── apps/web/src/faq/FAQPage.tsx:1546-1546 ───
[style · low] 项目规则禁止 `==` / `!=`：此处 `value == null` 应改为显式判空（同批新增的批量标签 TdSelect onChange
中还有一处同款写法，需一并修改）。

-                       onChange={(value) => onFormChange({ tagId: value == null || value === '' ? '' : String(value) })}
+                       onChange={(value) => onFormChange({ tagId: value === null || value === undefined || value === '' ? '' : String(value) })}


─── apps/web/src/faq/FAQPage.tsx:1519-1519 ───
[maintainability · low] 硬编码业务数字：编辑器答案计数由原 `{form.answers.length}/{FAQ_ANSWER_CAP}` 改为字面量 `5`，而同字段
disabled 判断仍引用 FAQ_ANSWER_CAP，两处来源不一致，将来调整答案上限时会漏改此处。

-                       <div className="item-count">{form.answers.length}/5</div>
+                       <div className="item-count">{form.answers.length}/{FAQ_ANSWER_CAP}</div>


─── apps/web/src/faq/FAQPage.tsx:1127-1133 ───
[bug · medium] 可访问性回归：标签筛选的清除按钮由旧实现的 `role="button" tabIndex={0}` + Enter/Space
onKeyDown（faq-tag-filter-clear）改为裸 `<span onClick>`，键盘用户无法触发清除；且 `showTagFilterClear =
activeTagIds.length > 0 && tagFilterTriggerHover` 仅在 hover 时出现，触屏与键盘聚焦场景下图标根本不可见。另借用 TDesign
内部类名（t-input__suffix 系）伪装样式较脆弱，建议改用自定义类名。

-                           {showTagFilterClear ? (
-                             <span
+ <span
-                               className="t-input__suffix t-input__suffix-icon t-input__clear"
+   role="button"
+   tabIndex={0}
+   className="faq-tag-filter-clear"
-                               aria-label={t('common.clear')}
+   aria-label={t('common.clear')}
-                               onClick={(event) => { event.stopPropagation(); setTagPanelOpen(false); setTagFilterCleared(true); onClearTagFilter(); }}
+   onClick={(event) => { event.stopPropagation(); setTagPanelOpen(false); setTagFilterCleared(true); onClearTagFilter(); }}
+   onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); event.stopPropagation(); setTagPanelOpen(false); setTagFilterCleared(true); onClearTagFilter(); } }}
-                               onMouseDown={(event) => event.stopPropagation()}
+   onMouseDown={(event) => event.stopPropagation()}
-                             >
+ >


─── apps/web/src/faq/FAQPage.tsx:521-521 ───
[maintainability · low] 死分支 + 缺失 i18n 键：外层 `kbList.length ?` 为真才会渲染该 Popup，而 `sortedKbList` 与
`kbList` 恒等长（sortKbListForSwitcher 只做重排），`!sortedKbList.length` 永远为 false，此空状态不可达；且 `common.noData`
键在整个 @weknora/i18n 目录中不存在，一旦渲染将直接显示原始键名。建议删除该行（空列表由下方非 Popup 的回退按钮分支展示）。



─── apps/web/src/faq/FAQPage.tsx:1558-1558 ───
[bug · medium] 可访问性回归：导入弹窗覆盖层由旧实现的 `role="dialog" aria-modal="true"
aria-label={t('knowledgeEditor.faqImport.title')}` 降级为裸
div，读屏器不再将其识别为对话框，也无语义边界/标题播报。下方批量标签弹窗（batch-tag-overlay）同款丢失。建议补回 role/aria-modal/aria-label。

-           <div className="faq-import-overlay" onClick={(event) => { if (event.target === event.currentTarget) onCloseImport(); }}>
+ <div className="faq-import-overlay" role="dialog" aria-modal="true" aria-label={t('knowledgeEditor.faqImport.title')} onClick={(event) => { if (event.target === event.currentTarget) onCloseImport(); }}>


─── apps/web/src/faq/FAQPage.tsx:1669-1669 ───
[bug · medium] 同导入弹窗：批量标签覆盖层丢失了旧实现的
role="dialog"/aria-modal="true"/aria-label={t('knowledgeEditor.faq.batchUpdateTag')}，读屏器无法将其识别为对话框，建
议一并补回。

-           <div className="batch-tag-overlay" onClick={(event) => { if (event.target === event.currentTarget) onCloseBatchTag(); }}>
+ <div className="batch-tag-overlay" role="dialog" aria-modal="true" aria-label={t('knowledgeEditor.faq.batchUpdateTag')} onClick={(event) => { if (event.target === event.currentTarget) onCloseBatchTag(); }}>


─── apps/web/src/faq/FAQPage.tsx:1653-1660 ───
[maintainability · low] 不可达条件：弹窗打开期间 importTask 恒为 null —— onOpenImport 在打开前已 setImportTask(null)，而
confirmImport 是先 setImportOpen(false) 再创建任务（:2386-2389），导入进度实际由面包屑标题行的提示条展示。因此取消按钮的
`disabled={importBusy && importTask?.status === 'running'}` 永不生效、确认按钮的 `&& !importTask`
恒真、success/failed 文案分支永不可达。建议简化为 `loading/disabled={importBusy}` 与固定文案，避免误导后续维护者以为弹窗内还有任务态。

-                   <Button
+ <Button
-                     theme="primary"
+   theme="primary"
-                     loading={importBusy && !importTask}
-                     disabled={importTask?.status === 'running'}
+   loading={importBusy}
+   disabled={importBusy}
-                     onClick={() => onImportConfirm()}
+   onClick={() => onImportConfirm()}
-                   >
+ >
-                     {importTask?.status === 'success' ? t('common.close') : importTask?.status === 'failed' ? t('common.retry') : t('knowledgeEditor.faqImport.importButton')}
+   {t('knowledgeEditor.faqImport.importButton')}
-                   </Button>
+ </Button>


─── apps/web/src/faq/FAQPage.tsx:1373-1373 ───
[maintainability · low] 死 prop：抽屉标题已改为用 editorMode 三元直接生成（本行），editorTitle（:693 接口定义、:785 解构、:2461
父组件传参）在视图内再无任何读取点，建议连同接口字段与父组件传参一并删除。



─── apps/web/src/faq/FAQPage.tsx:2177-2181 ───
[maintainability · low] 死 prop：message 已全量改为 MessagePlugin toast（本 effect），FAQPageView 内不再渲染任何内联
Status/错误块，视图的 message prop（:698 接口定义、:790 解构、:2466 父组件传参）成为死代码，建议一并移除，避免误导后续维护者以为视图仍有内联错误展示。



─── apps/web/src/faq/FAQPage.tsx:2489-2489 ───
[bug · low] 后台任务状态丢失：导入进行中（processing，1.5s 轮询）时用户仍可从新建下拉再次打开导入弹窗，此处 setImportTask(null)
会清掉正在运行的任务——面包屑进度条消失、轮询 useEffect 终止，任务完成后不再触发
load(false)/saveLastCompletedTaskId/loadLastResult，列表与导入结果条静默不更新。建议打开弹窗时不清任务（或导入期间禁用导入入口）。

-       onOpenImport={() => { setImportFile(null); setImportPreview([]); setImportTask(null); setImportOpen(true); }}
+ onOpenImport={() => { setImportFile(null); setImportPreview([]); setImportOpen(true); }}


─── apps/web/src/faq/faq.td.css:2240-2244 ───
[bug · medium] 非法 CSS 颜色值：`var(--td-brand-color)1a` / `var(--td-brand-color)33` 是把 Less/SCSS 的
fade() 十六进制透明度结果误写进了标准 CSS——var() 后拼接十六进制不生效，这两条声明会被浏览器整条丢弃，检索结果中的答案标签将回退到 TDesign 默认底色/边框，与 Vue
基线视觉不一致。文件内已有 color-mix 先例（.tag-filter-chip.active），建议改用 color-mix。

  .answer-tag {
-   background: var(--td-brand-color)1a;
+   background: color-mix(in srgb, var(--td-brand-color) 10%, transparent);
    color: var(--td-brand-color);
-   border-color: var(--td-brand-color)33;
+   border-color: color-mix(in srgb, var(--td-brand-color) 20%, transparent);
  }


─── apps/web/src/faq/faq.td.css:43-46 ───
[maintainability · low] 死样式与重复选择器：`.fade-enter-active/.fade-leave-*` 是 Vue transition 组件类，React TSX
中无任何触发点（`.modal-enter-*/.modal-leave-*`、`.slide-down-*`、`.faq-batch-bar-fade-*`、`@keyframes
dropdownSlideInUp`、`.tag-menu` 系列同属 Vue 平移残留，均无引用）；另外 `.faq-search-drawer .question-tag`
连续声明了两次、`.faq-editor-drawer .full-width-input-wrapper .add-item-btn` 也重复定义两段，建议合并清理。



─── apps/web/src/faq/faq.td.css:1079-1083 ───
[bug · medium] 选择器永不匹配：`question-tag` 是直接挂在 TdTag 根元素上的 className（如卡片分区 `<TdTag
className="question-tag">`），DOM 中不存在 `.question-tag` 的 `.t-tag` 后代，本块及下方 `.faq-manager .question-tag
.t-tag span` 的 `!important` 覆盖（inline-flex、overflow:hidden、内部文本单行省略）全部失效，长标签会回退 TDesign 默认样式并换行，与
Vue 基线不符。应改用复合选择器 `.question-tag.t-tag` 并以 `.t-tag__text` 定位文本（可参考同文件生效的 `.faq-manager .faq-card-tag
.t-tag` / `.faq-tag-wrapper .t-tag` 写法）。

- .faq-manager .question-tag .t-tag {
+ /* question-tag 直接位于 TdTag 根元素，需用复合选择器 */
+ .faq-manager .question-tag.t-tag {
    max-width: 100% !important;
    min-width: 0 !important;
    width: auto !important;
    display: inline-flex !important;
+   align-items: center;
+   vertical-align: middle;
+   overflow: hidden !important;
+   box-sizing: border-box;
+   background: var(--td-bg-color-container);
+   border-color: var(--td-component-stroke);
+   color: var(--td-text-color-primary);
+ }
+ 
+ .faq-manager .question-tag.t-tag .t-tag__text {
+   display: block !important;
+   overflow: hidden !important;
+   text-overflow: ellipsis !important;
+   white-space: nowrap !important;
+   max-width: 100% !important;
+   min-width: 0 !important;
+   line-height: 1.4;
+ }


─── apps/web/src/integrations/ApiPlaygroundDrawer.tsx:317-317 ───
[maintainability · low] 四分支三层嵌套三元表达式选择状态色类，违反“禁止嵌套三元”规则且可读性差。元素已带 `data-status` 属性，建议改为查表对象（或复用
data-status 驱动的 CSS 规则——注意 integrations.td.css 中按 data-status 落色的 `.wk-api-playground-status[...]`
选择器因元素缺少该类名 currently 不命中，属死规则）。

-     <span className={status === 'success' ? 'wk-apd-14' : status === 'failed' ? 'wk-apd-15' : status === 'stopped' ? 'wk-apd-16' : 'wk-apd-17'} data-status={status || 'none'}>{status || '-'}</span>
+     <span className={STATUS_COLOR_CLASS[status || 'none'] ?? 'wk-apd-17'} data-status={status || 'none'}>{status || '-'}</span>


─── apps/web/src/integrations/EmbedPreviewModal.tsx:92-92 ───
[maintainability · low] 硬编码中文文案“正在加载预览…”：同组件其余文案均经 locale/integrationsT 机制传入（文件顶部已 import），且 views 侧
`embedPublish.previewLoading` 键已覆盖 5 种语言（embedWizardMessages.ts
L90/217/344/471/598），多语言场景下该提示无法翻译。建议复用既有 i18n 键。



─── apps/web/src/integrations/IntegrationsRoutePage.tsx:23-26 ───
[maintainability · low] 模块顶层向全局可变单例注册 sprite 渲染器且无清理入口：全仓仅此一处注册（page.tsx 中 spriteIconRenderer
为模块级可变状态），同进程下 apps/web 侧用例（注入 TIcon）与 packages/views 直渲染用例（期望回退手绘 path）共用模块状态时会相互串扰，且注册后无法重置。建议约定测试
afterEach 调 `setIntegrationSpriteIconRenderer(null)` 清理，或改为 context/prop 显式注入。



─── apps/web/src/integrations/integrations.td.css:107-114 ───
[maintainability · medium] 死 CSS 与双轨定义冲突：(1) 组件已改用 wk-epm-*/wk-vi-*/wk-apd-* 类，本文件第 3 节
`.wk-embed-preview-overlay/-drawer/-header/-device/-chrome/-body/-screen` 与第 2 节
`.wk-api-playground-aside/-body/-section/-status/-error/-step-label` 在 TSX
中已无任何元素输出（仅存于注释），规则永不命中；(2) 残留的 `.wk-api-playground-overlay .wk-muted { color:
var(--color-muted,#66758b) }` 特异性 (0,2,0) 高于 wk-apd-2 (0,1,0)，会静默覆盖 u.css 已迁移的 Vue 对齐值
rgba(0,0,0,.6)，与迁移注释自相矛盾；(3) `.wk-integration-drawer-close` 在 td.css / wk-epm-5 /
wk-vi-integration-drawer-close-class 三处定义，ink 色值（var(--color-ink,#172033) vs
rgba(0,0,0,.9)）不一致，最终呈现取决于打包 CSS 顺序。另注：ApiPlaygroundDrawer 换原生 input
后的输入框样式（height/border/focus）只在本文件定义，组件自身仅 import integrations-u.css，依赖 IntegrationsRoutePage
的导入顺序才生效。建议删除死规则块、收敛重复定义。



─── apps/web/src/integrations/views-integrations-u.css:39-44 ───
[bug · medium] 无效 CSS 声明：`color: color:inherit;` 是把 Tailwind 的类型提示 `text-[color:inherit]`
误拷进了属性值（本文件共 3 处：L43 clickable-class、L64 static-class、L148
title-add-class）。该声明会被浏览器整条丢弃，元素颜色不再受控（依赖父级继承的偶然结果），且可能导致 stylelint/严格构建校验失败。应写为 `color: inherit;`。

  .wk-vi-channel-card-clickable-class {
    width: 100%;
    cursor: pointer;
    background-color: #ffffff;
-   color: color:inherit;
+   color: inherit;
  }


─── apps/web/src/integrations/views-integrations-u.css:902-904 ───
[bug · medium] 无效 calc 表达式：`top: calc(100%+4px)` 中 `+`
两侧必须有空格，整条声明无效被浏览器丢弃，`.wk-vi-33`（AgentFilterButton 代理筛选下拉框）将退回 `top: auto` 静态定位，下拉框失去 4px
下移偏移，可能贴住/覆盖触发按钮。应为 `calc(100% + 4px)`。

  .wk-vi-33 {
    position: absolute;
-   top: calc(100%+4px);
+   top: calc(100% + 4px);


─── apps/web/src/integrations/views-integrations-u.css:824-829 ───
[bug · high] views 侧预览 iframe 铺满规则丢失：apps/web 侧已承认平移丢失并为自家模态补上 `.wk-epm-13 iframe { width:100%;
height:100% }`（integrations-u.css L301-307 注释 ocr3-007），但 views 端同类容器 `.wk-vi-24`（iframe 模式屏幕区）与
`.wk-vi-28`（widget 面板）没有任何 iframe 限定规则；styles.css 中旧 `.wk-embed-preview-*` 家族已删（仅剩 L123
迁移注释），也无全局兜底。EmbedChannelPreviewPanel 的预览 iframe 将回退默认 300×150 尺寸，嵌入预览功能视觉损坏。建议镜像 ocr3-007 的补救。

  .wk-vi-24 {
    position: absolute;
    inset: 37px 0 0;
    display: grid;
    place-items: center;
+ }
+ 
+ /* 预览 iframe 铺满容器（对齐 apps/web 侧 .wk-epm-13 iframe 的 ocr3-007 补救） */
+ .wk-vi-24 iframe,
+ .wk-vi-28 iframe {
+   width: 100%;
+   height: 100%;
+   border: 0;
+   background: #ffffff;
  }


─── apps/web/src/integrations/views-integrations-u.css:28-30 ───
[maintainability · low] 同一规则内残留恒被覆盖的死声明（Tailwind 冲突未收敛）：本处双 `transition-duration`（150ms 恒被 180ms
覆盖）；同类问题还有 `.wk-vi-91/.wk-vi-137/.wk-vi-160/.wk-vi-164` 的 `font-size` 具体值后跟 `font-size:
inherit`、`.wk-vi-104` 的 `border-style: solid` 后跟 `dashed`、`.wk-vi-57/.wk-vi-87` 的 `font` 简写覆盖其前的
`font-size`；integrations-u.css 的 `.wk-apd-13` 也有 `border-style: solid` 后跟 `solid
!important`。死声明会误导后续维护，建议只保留实际生效的一条（并确认保留值与 Tailwind 原语义一致）。

-   transition-duration: 150ms;
    transition-duration: 180ms;
  }


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:822-823 ───
[other · medium] 死状态 + 冗余网络请求：`editorActivity` / `editorActivityLoading`
在整个文件中只有写入（loadEditorActivity，L1111-1119）没有任何读取渲染——activity 段实际渲染的是自带数据加载的
`<KnowledgeBaseActivityPanel client={client} knowledgeBaseId={editingId} />`（L1888 附近）。当前点击 activity
导航项（L1651 `void loadEditorActivity(editingId)`）会调用 `client.knowledgeBases.settings.activity(id)`
拉取全量活动数据后丢弃，面板随后再拉一遍。建议删除这两个 state 与 loadEditorActivity（或改为由面板复用该数据）。

-   const [editorActivity, setEditorActivity] = useState<Array<{ id: number; action: string; outcome: string; created_at: string }>>([]);
-   const [editorActivityLoading, setEditorActivityLoading] = useState(false);
+ // activity 段由 KnowledgeBaseActivityPanel 自行加载数据，此处无需预取。


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:192-192 ───
[maintainability · low] 重复实现：本地 SpaceAvatar 与 `apps/web/src/organizations/SpaceAvatar.tsx` 重复——同一份
12 组渐变表、同一 hash 算法、同一装饰 SVG 与首字母逻辑（organizations 版还多 emoji avatar 支持）。同一页面 bundle
中两份并存（KnowledgeBaseShareDialog 内用的是 organizations 版），后续极易漂移。若为 Vue 1:1 平移保真（space-avatar-* 类名配对
kb-list.td.css §2）需保留，请像 UPLOAD_SVG 那样标注归并计划；否则建议复用 organizations/SpaceAvatar。

- const SPACE_GRADIENTS: ReadonlyArray<{ from: string; to: string }> = [
+ // TODO(Phase 4 归并): 与 organizations/SpaceAvatar.tsx 重复（渐变表/hash/装饰 SVG），
+ // 待 knowledge-settings 批次归并为单一实现。


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:55-55 ───
[style · low] 同一模块重复 import：`tdesign-react` 在上方已有一条具名导入（Button/Checkbox/Dialog/...），此处又开一条。虽然合法，但违反
no-duplicate-imports 惯例且增加维护成本，建议合并为一条。

- import { Checkbox as TCheckbox, Input as TInput, Select as TSelect, Textarea as TTextarea } from 'tdesign-react';
+ import {
+   Button, Checkbox, Checkbox as TCheckbox, Dialog, Input, Input as TInput,
+   Loading, Popup, Radio, RadioGroup, Select as TSelect, Skeleton, Textarea,
+   Textarea as TTextarea, Tooltip, MessagePlugin,
+ } from 'tdesign-react';


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:1749-1749 ───
[style · low] 嵌套三元 + 重复求值：`extractionGranularity ?? 'standard'`
在同一表达式里计算三次，且是双层嵌套三元，可读性差（项目规约禁止嵌套三元）。建议在 JSX 外提取局部变量。

-                               <p className="form-tip granularity-hint">{t(`knowledgeEditor.wiki.granularity${(editorConfig.wikiConfig?.extractionGranularity ?? 'standard') === 'focused' ? 'Focused' : (editorConfig.wikiConfig?.extractionGranularity ?? 'standard') === 'exhaustive' ? 'Exhaustive' : 'Standard'}Hint`)}</p>
+ // 在外层提取：const granularity = editorConfig.wikiConfig?.extractionGranularity ?? 'standard';
+ // const granularityHintKey = granularity === 'focused' ? 'Focused' : granularity === 'exhaustive' ? 'Exhaustive' : 'Standard';
+ <p className="form-tip granularity-hint">{t(`knowledgeEditor.wiki.granularity${granularityHintKey}Hint`)}</p>


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:178-179 ───
[maintainability · low] 重复代码：`OrgGreenIcon`（同一路径数据的 organization-green.svg）在本文件与
SharedKnowledgeBaseDrawer.tsx 各内联一份，两处后续修改（如换色/尺寸）会漂移。建议抽到共享模块（如 kb-list-icons.tsx）供两处导入。



─── apps/web/src/knowledge-bases/SharedKnowledgeBaseDrawer.tsx:54-58 ───
[other · low] 死导出：`permissionTone` 兼容层在全仓库（含测试 .mjs/.tsx）已无任何消费者——本文件已改用
`permissionTheme`，也无外部引用。建议删除该兼容导出及注释。



─── apps/web/src/knowledge-bases/kb-editor-parity.css:23-24 ───
[other · low] `label.grid` 两条规则已无对应
DOM：留守段（models/vectorStore/parser/storage/multimodal/asr/advanced）经 kb-u.css 收编后统一使用 `wk-kbl-4/5/12`
等类，页面中已不存在 `label.grid` 元素。另外 `.kb-editor-desc-count` 在 kb-u.css（.wk-kbl-13 +
.wk-kbl-count-end）、kb-list.td.css 与本文件三处定义（值目前一致），建议只保留一处，避免后续漂移。

- .wk-kb-editor-dialog .kb-editor-content label.grid { row-gap: 8px; line-height: 21px; }
- .wk-kb-editor-dialog .kb-editor-content label.grid > span { line-height: 21px; }
+ /* 删除 label.grid 两条死规则；.kb-editor-desc-count 建议收敛到 kb-u.css 单一定义。 */


─── apps/web/src/knowledge-bases/kb-list.td.css:2426-2427 ───
[maintainability · medium] 约 250 行死样式与 kb-u.css
双轨重复：`wk-kb-activity-*`、`wk-kb-share-*`、`wk-kb-des-*` 三段收编类在全仓库 TSX/TS 中均无消费者——activity 面板实际用
`wk-kba-*`、share 对话框用 `wk-kbs-*`、编辑器留守段用 `wk-kbl-*`（全部在 kb-u.css，且逐值等价）。两套并存极易在后续调色/间距时只改一份。同类的还有 §7
尾部的 `.shared-detail-drawer-enter-*`：Vue Transition 类，React 版 SharedKnowledgeBaseDrawer 直接
mount/unmount 不挂这些类（滑入动画实际丢失），也是死规则。建议删除这三段死前缀及过渡类，`.kb-editor-desc-count` 收敛到一处定义（当前
kb-u.css/.wk-kbl-13、本文件、kb-editor-parity.css 三处重复）。

- /* ==== kb activity 面板/详情抽屉 Tailwind 收编（S6，原 utilities 逐值平移） ==== */
- .wk-kb-activity { display: grid; gap: 12px; }
+ /* 删除：wk-kb-activity-* / wk-kb-share-* / wk-kb-des-* 及 .shared-detail-drawer-enter-*
+  * （无 TSX 消费者，活类为 kb-u.css 的 wk-kba-* / wk-kbs-* / wk-kbl-*）。
+  * 仅保留 .kb-editor-desc-count（页面 advanced 段在用），并收敛至单一定义。 */


─── apps/web/src/knowledge-settings/KnowledgeSettingsPage.css:736-739 ───
[bug · medium] 键盘焦点指示缺失：原生 checkbox 改为 1px/clip 视觉隐藏、可见面孔由 .kb-checkbox-input span 绘制后，整份 CSS 中没有任何
input:focus-visible + .kb-checkbox-input 的替代焦点样式（文件内 focus 相关规则仅覆盖 kb-text-input-wrap /
kb-text-input / kb-textarea）。改造前原生 checkbox 自带浏览器焦点环，现在键盘用户 Tab 到「向量检索 / Wiki 索引」复选框时完全无焦点反馈，构成 WCAG
2.4.7（焦点可见）回归。

  .indexing-check-head input:checked + .kb-checkbox-input {
    border-color: var(--wk-color-brand, #07c05f);
    background-color: var(--wk-color-brand, #07c05f);
+ }
+ .indexing-check-head input:focus-visible + .kb-checkbox-input {
+   outline: 2px solid var(--wk-color-brand, #07c05f);
+   outline-offset: 1px;
  }


─── apps/web/src/knowledge-settings/KnowledgeSettingsPage.css:632-636 ───
[bug · low] 首段左边框缺失：基类的 border-left: 0 会把首段的左边框一并去掉——首段是唯一不命中既有规则 .kb-type-tab + .kb-type-tab {
border-left: 1px } 的分段。FAQ 型知识库时首段「标准」为未选中的禁用段，整组分段控件的左缘 1px 描边缺失，与紧邻注释引用的 tdesign 模型（.t-is-checked
+ .t-radio-button { border-left: 0 }，即只清除紧跟选中段之后那一段）不一致。基类去掉 border-left: 0
后首段自带左边框，既有两条相邻规则即可组成完整边框链。

    line-height: 22px;
    /* tdesign .t-radio-button 基础边框：#dcdcdc（theme.css 覆盖后 border-level-2），
       选中段整圈 #8ce0af、其后段左边框清除。 */
    border: 1px solid var(--wk-color-border, #dcdcdc);
-   border-left: 0;


─── apps/web/src/knowledge-settings/KnowledgeSettingsPage.css:831-831 ───
[style · low] 无效 CSS 值：border-color: none 不是合法取值（border-color 仅接受 <color>），该声明会被浏览器整条丢弃。由于
.kb-text-input 自身 border: 0，当前无实际影响，但无效声明会误导后续维护者以为焦点态存在边框色变化，建议删除（或改 transparent）。

- .kb-text-input:focus { outline: none; border-color: none; }
+ .kb-text-input:focus { outline: none; }


─── apps/web/src/knowledge-settings/knowledge-settings-u.css:163-164 ───
[style · low] 格式不一致：.wk-kss-19 与 .wk-kss-21（同样写法）把两条声明挤在同一行（border-bottom-width: 1px;border-color:
#dcdcdc;），语法合法但与文件内其余规则的一行一声明格式不一致，影响可读性与后续 diff 审查。

-   border-bottom-width: 1px;border-color: #dcdcdc;
+   border-bottom-width: 1px;
+   border-color: #dcdcdc;
    background-color: #f3f3f3;


─── apps/web/src/knowledge-settings/parserSettings.tsx:226-230 ───
[bug · medium] 可访问性回归：aria-label 从原生 select 迁移后挂在无角色的包裹 <span> 上。ARIA 规范中无角色元素的 aria-label
不参与后代可聚焦控件（tdesign Select 内部 combobox input）的可访问名称计算，读屏用户聚焦该下拉框时将听不到组名 group.label；迁移前 aria-label
位于可聚焦的 select 元素上是生效的。项目内既有做法是直接把 aria-label 传给 tdesign Select（如 SystemGlobalSettingsPanel.tsx 的
<Select aria-label=... />），或将包裹元素改为 <label> 隐式关联内部 input，建议对齐其一。

-               <span
+               <label
                  data-parser-group={group.key}
-                 aria-label={group.label}
                  style={{ display: 'block' }}
                >


─── apps/web/src/knowledge/KnowledgeGraphPage.tsx:0-0 ───
[style · low] className 存在前导空格(`" wk-graph-arrow-stroke wk-kg-4"`),属迁移格式残留,无功能影响但建议清理以保持类名列表整洁。

- markerStart={showArrows && edge.bidirectional ? `url(#wk-graph-arrow-start${lit ? '-hl' : ''})` : undefined} className=" wk-graph-arrow-stroke wk-kg-4" style={highlight ? { stroke, strokeWidth: lit ? 2 : 1, strokeOpacity: lit ? 0.9 : 0.08, transition: 'stroke 0.2s, stroke-width 0.2s, stroke-opacity 0.2s' } : undefined} />;
+ className="wk-graph-arrow-stroke wk-kg-4"


─── apps/web/src/knowledge/knowledge-u.css:251-253 ───
[bug · medium] 选择器写法错误,该规则整体失效:`:-webkit-details-marker` 是单冒号伪类形式(CSS 对伪元素单冒号的向后兼容仅限
:before/:after/:first-line/:first-letter 四个),Blink 会将其视为未知伪类而丢弃整条规则;且即使被容错解析,`.wk-kg-24
:-webkit-details-marker` 中的空格后代组合符匹配的是 .wk-kg-24 的后代元素而非 summary 自身的 marker 伪元素。原 Tailwind
`[&::-webkit-details-marker]:hidden` 的意图是隐藏 summary 默认展开三角,此写法导致帮助按钮「?」旁回归显示浏览器默认三角。同仓库正确写法参照
platform-shell.td.css:1055(`summary::-webkit-details-marker`)。

- .wk-kg-24 :-webkit-details-marker {
+ .wk-kg-24::-webkit-details-marker {
    display: none;
  }


─── apps/web/src/knowledge/knowledge-u.css:699-700 ───
[bug · medium] `.wk-ds-self-start` 是 data-sources 域的类(唯一消费方 DataSourcesPage.tsx:441,`wk-ds-15
wk-ds-self-start`),却被定义在 knowledge 域的 knowledge-u.css 尾部。knowledge-u.css 仅由 KnowledgeGraphPage.tsx
导入,而该页面在 router.tsx:67 是 `lazy()` 懒加载——这条规则只随图谱页 chunk 下发。用户从 KnowledgeBasesPage 编辑对话框或 KB
设置内嵌面板直接进入 DataSourcesPage 时(两者均静态导入 DataSourcesPage 而不依赖图谱 chunk),该规则不在文档中,前置条件链接的 `justify-self:
start` 静默丢失,且样式是否生效取决于用户此前的导航路径。应将该规则移入消费方所在域的 data-sources-u.css(其头注释同样声明为「data-sources 域漂零段」)。

- .wk-graph-arrow-stroke { stroke: #c0c4cc; }
+ /* 从本文件删除,移入 data-sources-u.css(消费方 DataSourcesPage.tsx 所在域):
  .wk-ds-self-start { justify-self: start; }
+ */


─── apps/web/src/knowledge/knowledge-u.css:327-331 ───
[maintainability · low] `transition-duration` 声明了两次(150ms 后跟 300ms):原 Tailwind 为 `transition-all
duration-300`,300ms 才是有效值,前一条 150ms 是 codemod 残留的死声明,最终生效值依赖书写顺序,容易在后续维护中被误改。建议删除 150ms 一行。另附:第 397
行与第 497 行附近 `border-top-width: 1px;border-color: ...` 两条声明挤在同一行(`;` 后无换行),属同类格式残留,建议一并整理。

    transition-property: all;
    transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
-   transition-duration: 150ms;
    transition-duration: 300ms;
  }


─── apps/web/src/knowledge/knowledge-u.css:613-615 ───
[bug · medium] chevron 旋转过渡失效（平移回归）：.wk-kg-58 声明 `transition-property: transform`，但 .wk-kg-59 用的是独立
`rotate: 180deg` 属性——`rotate` 与 `transform` 是两个不同属性，transform 过渡不会覆盖
rotate，展开/收起检索下拉时箭头（KnowledgeGraphPage.tsx:739 `wk-kg-58 wk-kg-59`）会瞬时跳变而非原 Tailwind
`transition-transform duration-150` + `rotate-180` 的 150ms 动画（v3 经 transform 链、v4 的
transition-transform 显式含 rotate，原本均带动画）。建议改回 `transform: rotate(180deg)`（与 .wk-kg-58 的
transition-property: transform 对齐），或在 .wk-kg-58 中把 transition-property 扩为 `transform, rotate`。

  .wk-kg-59 {
-   rotate: 180deg;
+   transform: rotate(180deg);
  }


─── apps/web/src/main.tsx:20-20 ───
[performance · low] opportunity.css 在应用入口全局同步引入，但 OpportunityPage 本身是路由级 lazy chunk——career
家族其余页面（application/material/inbox/rule/search/progress/preparation/submission 等）均由页面自身 `import
'./xxx.css'` 随懒加载 chunk 按需载入。此写法把约 160 行仅机会页使用的样式打进首屏主包，且与其余兄弟页面的 CSS
组织方式不一致；OpportunityPage.test.tsx 通过 readFileSync 直接读文件断言，不依赖该全局引入。建议将引入移入 OpportunityPage.tsx 自身。

- import './career/opportunity.css';
+ // 删除此行；在 career/OpportunityPage.tsx 顶部改为 `import './opportunity.css'`


─── apps/web/src/market/MarketPage.tsx:539-539 ───
[style · low] 本行（及 660 行 installTarget 的双层三元、727
行行状态色的嵌套三元）为嵌套三元表达式，违反评审规则"禁止嵌套三元"。虽系原代码结构平移，但既然本次已重写该行，建议顺手抽为映射常量提升可读性。

-           className={'wk-mkt-34 ' + (toast.tone === 'success' ? 'wk-mkt-35' : toast.tone === 'warning' ? 'wk-mkt-36' : 'wk-mkt-37')}
+ const TOAST_TONE_CLASS: Record<'success' | 'warning' | 'error', string> = {
+   success: 'wk-mkt-35',
+   warning: 'wk-mkt-36',
+   error: 'wk-mkt-37',
+ };
+ // className={'wk-mkt-34 ' + TOAST_TONE_CLASS[toast.tone]}


─── apps/web/src/market/market-u.css:46-47 ───
[maintainability · medium]
迁移时将主题令牌类平铺为字面值：bg-surface→#ffffff（.wk-mkt-mk-tab/.wk-mkt-mk-card/.wk-mkt-mk-btn-outline/.wk-mkt-mk-
input/.wk-mkt-9/.wk-mkt-29/.wk-mkt-40）、bg/text/border-accent→#07c05f（页签 hover/选中、按钮、进度条 wk-mkt-49
等）、bg-accent-wash→rgba(7,192,95,0.08)。styles.css 的 @theme 中这些颜色是运行时 CSS 变量（Tailwind v4 utilities 按
var(--color-*) 解析，注释亦声明令牌 stylesheet 为 runtime light/dark source，且 design-tokens 含 dark 调色板）。平铺后
market 域与令牌单源脱钩，后续令牌改值或接入暗色主题时本页会静默保持旧色，与仍引用 var(--wk-bg,#fff) 的 wk-mkt-12/32 也形成取值口径不一致。建议保留 var()
间接引用。

    border-color: #e7e7ea;
-   background-color: #ffffff;
+   background-color: var(--color-surface);


─── apps/web/src/market/market-u.css:587-591 ───
[maintainability · low] wk-mkt-38 对 .wk-mkt-mk-tab（height/padding-inline/font-size）、wk-mkt-41/42 对
.wk-mkt-40（background-color/border-color）的覆盖均为 (0,1,0) 单类、无 @layer，生效完全依赖本文件内的书写顺序；一旦规则被重排或未来本域引入
market.td.css 改变层叠（文件头注释即声明"须在本域 .td.css
之前导入"，该约束目前无文件兜底），排行榜页签高度与复选框选中底色将静默回退为基类值。建议在覆盖类旁加行内注释标注"覆盖基类，须置于 .wk-mkt-mk-tab / .wk-mkt-40
之后"，或改为基类规则内属性选择器（如 [aria-selected]）承接变体，消除隐式顺序契约。



─── apps/web/src/market/market-u.css:378-379 ───
[style · low] 生成痕迹格式瑕疵：.wk-mkt-13 与 .wk-mkt-32 中 `border-bottom-width: 1px;border-color: rgba(...)`
两条声明挤在同一行，且以全边 border-color 配单边宽度；同时基类已声明 border-style: solid
的情况下，.wk-mkt-mk-tab:hover、.wk-mkt-mk-input:focus、.wk-mkt-41/42 等变体重复声明 border-style:
solid。均不影响渲染，但降低可读性，建议拆行规范并去除冗余声明。

    border-bottom-style: solid;
-   border-bottom-width: 1px;border-color: rgba(127,127,127,0.2);
+   border-bottom-width: 1px;
+   border-color: rgba(127,127,127,0.2);


─── apps/web/src/organizations/OrganizationsPage.tsx:1026-1027 ───
[bug · medium] org 卡片由旧实现的 `role="button" tabIndex={0} onKeyDown(Enter)` 降级为纯 onClick 的 div：键盘用户（Tab
+ Enter）无法打开组织设置弹窗，读屏器也不会播报为可操作元素。同类问题还有：① 底部 del-org-dialog 的「取消/删除」为 `<span
className="circle-btn-txt" onClick>`（删除组织是不可逆操作，键盘无任何确认路径）；② 侧栏 icon-item-labeled /
sidebar-item、设置弹窗 nav-item 均为 div+onClick。OCR round-3/round-4 已记录为遗留未修，建议至少为卡片恢复键盘语义，弹窗确认按钮恢复 button
语义（css 需补 background/border/padding 重置）。

        <div key={org.id || index} className={'org-card' + (owner ? '' : ' joined-org')} style={rowHidden ? { display: 'none' } : undefined}
-         onClick={() => openSettingsModal(org)}>
+         role="button" tabIndex={0}
+         onClick={() => openSettingsModal(org)}
+         onKeyDown={(event) => { if (event.key === 'Enter') openSettingsModal(org); }}>


─── apps/web/src/organizations/OrganizationsPage.tsx:1400-1400 ───
[bug · low] 5 处 label htmlFor 悬空：本行 create 模式 Input（name="organization-name"）未传 id，对应 `<label
htmlFor="organization-name">`（L1364）找不到目标控件；同类还有 join-code（L1868）、join-search（L1878）的 TInput，以及
upgrade-role（L1713）、join-request-role（L1842）的 TSelect。点击 label 不聚焦、控件可访问名称缺失。TDesign 控件支持透传
id，建议逐一补上。

-                                 <Input name="organization-name" className="name-input" value={formName} onChange={(value: string) => setFormName(value)} placeholder={t(locale, 'organization.namePlaceholder')} />
+                                 <Input id="organization-name" name="organization-name" className="name-input" value={formName} onChange={(value: string) => setFormName(value)} placeholder={t(locale, 'organization.namePlaceholder')} />


─── apps/web/src/organizations/OrganizationsPage.tsx:1332-1334 ───
[bug · low] <720px 双导航：注释称「侧栏不可见时的等价 section 切换」，但迁移后 orgs.td.css 的 .settings-modal
.settings-sidebar 没有任何移动端隐藏规则（旧代码有 max-[720px]:hidden），小屏下 208px
侧栏与该下拉选择器同时显示，出现两套导航，且注释描述与实际行为相悖。建议在 orgs.td.css 补 `@media (max-width: 720px){ .settings-modal
.settings-sidebar{ display:none; } }`，或删除 wk-org-5 并修正注释。

-                 {/* React 韧性补充（<720px 时侧栏不可见时的等价 section 切换），
-                     Vue 无对应物；桌面扫描态 display:none 零像素影响。 */}
+                 {/* React 韧性补充（Vue 无对应物）；wk-org-5 仅 <720px 显示，
+                     配合 orgs.td.css 中 settings-sidebar 的移动端隐藏规则。 */}
                  <div className="wk-org-5">


─── apps/web/src/organizations/OrganizationsPage.tsx:21-22 ───
[maintainability · low] ① 同模块两条 tdesign-react import 产生 Input/TInput、Textarea/TTextarea
双别名（同一组件两个名字），编辑时极易拿错——事实上编辑模式名称字段已用原生 `<input>` 而创建模式用 TDesign Input，控件栈不一致；② L14 裸导入的 FormEvent
未被使用（文件内均为 React.FormEvent），属死导入。建议合并为单条 import、删除未用的 FormEvent、统一两模式输入控件。

- import { Input as TInput, Select as TSelect, Switch as TSwitch, Textarea as TTextarea } from 'tdesign-react';
- import { Button, Dialog, Input, Popup, Skeleton, Tag, Textarea, Tooltip } from 'tdesign-react';
+ import { Button, Dialog, Input, Popup, Select, Skeleton, Switch, Tag, Textarea, Tooltip } from 'tdesign-react';


─── apps/web/src/organizations/OrganizationsPage.tsx:291-291 ───
[maintainability · low] 嵌套三元（stat-member → stat-kb → agent）违反项目嵌套三元禁令；且三种徽标图标尺寸不一致——IconUser 默认
12px，folder/agent 内联 svg 为 14px，并排渲染时视觉不齐（该组件现用于加入弹框预览态）。建议改为查表（tone → icon 映射）并统一尺寸为 14px。

-       {props.tone === 'stat-member' ? <IconUser /> : props.tone === 'stat-kb' ? (
+       {props.tone === 'stat-member' ? <IconUser size={14} /> : props.tone === 'stat-kb' ? (


─── apps/web/src/organizations/OrganizationsPage.tsx:1289-1291 ───
[maintainability · low] 不可达分支：渲染链 loading / listError / ordered.length===0&&!loading /
ordered.length>0 四个条件已穷尽全部状态组合，末尾 `: null` 永不执行，属死代码；且首分支 `loading && ordered.length === 0` 与第三分支
`ordered.length === 0 && !loading` 不对称，读者需推理才能确认穷尽。建议清理冗余分支。

-           ) : ordered.length > 0 ? (
+           ) : (
              <div className="org-card-wrap">{cardRows}</div>
-           ) : null}
+           )}


─── apps/web/src/organizations/OrganizationsPage.tsx:264-264 ───
[style · low] 嵌套三元违反项目「禁止嵌套三元表达式」规则：`size === 'small' ? ' space-avatar-small' : size === 'large' ? '
space-avatar-large' : ''` 的 false 分支又是一个三元。本函数为本次重写的新代码（与已确认问题 #5 的 FeatureBadge 同类但位于不同代码点），建议改为
size → 类名查表，可读性更好且与下方 space-avatar-small/large 的 CSS 分支一一对应。

-       className={'space-avatar' + (size === 'small' ? ' space-avatar-small' : size === 'large' ? ' space-avatar-large' : '') + (isEmoji ? ' space-avatar-emoji' : '') + (props.className ? ' ' + props.className : '')}
+ const SPACE_AVATAR_SIZE_CLASS: Record<'small' | 'medium' | 'large', string> = {
+   small: ' space-avatar-small',
+   medium: '',
+   large: ' space-avatar-large',
+ };
+ // ...
+       className={'space-avatar' + SPACE_AVATAR_SIZE_CLASS[size] + (isEmoji ? ' space-avatar-emoji' : '') + (props.className ? ' ' + props.className : '')}


─── apps/web/src/organizations/SpaceAvatar.tsx:29-29 ───
[style · low] 本文件本次改写引入了 3 处嵌套三元（容器类此处 L29、emoji 字号 L30、字母字号 L32，均为 `size === 'large'/'small' ? … :
… ? … : …` 结构），违反项目「禁止嵌套三元表达式」规则。三处共用同一 size 维度，建议抽一个尺寸→类名后缀的查表 Record，一次消除三处嵌套。

-   return <span className={`wk-avatar-2 ${size === 'small' ? 'wk-avatar-3' : size === 'large' ? 'wk-avatar-4' : 'wk-avatar-5'} ${className}`} style={style} aria-hidden="true">
+ const AVATAR_SHELL_CLASS: Record<'small' | 'medium' | 'large', string> = {
+   small: 'wk-avatar-3',
+   medium: 'wk-avatar-5',
+   large: 'wk-avatar-4',
+ };
+ // 同理为 emoji/字母字号各建一张尺寸→类名表（wk-avatar-10/11/12 与 wk-avatar-7/8/9）
+   return <span className={`wk-avatar-2 ${AVATAR_SHELL_CLASS[size]} ${className}`} style={style} aria-hidden="true">


─── apps/web/src/organizations/org-u.css:928-929 ───
[bug · medium] `max-height: calc(90vh-120px)` 减号两侧缺少空格，属无效 CSS 声明，解析失败后整条 max-height
被浏览器静默丢弃——加入组织弹窗正文区因此失去高度上限与滚动约束，长内容（搜索结果列表 + 表单）会顶出弹窗并被外层 overflow:hidden 裁切而无法查看。该值系从旧 Tailwind
同样无效的 max-h-[calc(90vh-120px)] 忠实平移的历史 bug，OCR round-3/round-4 均已记录未修，新文件应顺手修正。

  .wk-org-67 {
-   max-height: calc(90vh-120px);
+   max-height: calc(90vh - 120px);


─── apps/web/src/organizations/org-u.css:53-58 ───
[bug · low] `.wk-org-org-field:focus` 在 TDesign 组合控件上永不命中：ORG_FIELD（wk-org-org-field）现在挂在
TInput/TSelect/TTextarea 的根级包裹元素（.t-input/.t-select/.t-textarea）上，而键盘焦点始终落在内层
input/textarea，包裹元素自身不会进入 :focus 状态——因此加入组织弹框的全部输入框/下拉/多行文本、成员上限输入、各处角色 Select 的绿色聚焦描边被静默丢弃（仅编辑模式原生
input（wk-org-94）与有效期按钮（wk-org-98）仍生效）。旧实现（@weknora/ui 控件 className 直挂原生元素）时该规则是生效的，属迁移引入的静默回归。建议改用
:focus-within，可同时覆盖原生控件与 TDesign 包裹层两种宿主。

- .wk-org-org-field:focus {
+ .wk-org-org-field:focus-within {
    border-style: solid;
    border-color: #07c05f;
+ }
+ 
+ .wk-org-org-field:focus-visible {
    outline: 2px solid transparent;
    outline-offset: 2px;
  }


─── apps/web/src/organizations/orgs.td.css:769-773 ───
[maintainability · low] card-header / card-content / card-bottom 各存在两套同目标规则：此处 6px/6px/6px 与文件后部
`.org-list-container .card-header`（margin-bottom: 8px）、`.card-content`（margin-bottom:
8px）、`.card-bottom`（padding-top: 8px）数值冲突，仅靠三类选择器特异性（0-3-0 压过 0-2-0）隐式取胜才得到旧 Tailwind 的 6px
生效值。后续维护者改任一块都极易引入视觉回归，建议删除重复的一组或合并为单一规则块。



─── apps/web/src/platform/PlatformShell.tsx:70-71 ───
[bug · medium] 键盘 Enter 分支是死代码：React 合成事件只按事件接口拷贝属性，SyntheticKeyboardEvent 实例上不存在 `button`（它是
MouseEvent 接口属性），键盘路径读 `(event as ReactMouseEvent).button` 恒为 undefined，`undefined !== 0` 恒真而提前
return——下方账号卡（dropdown-user-header，role="button" tabIndex={0}）的 onKeyDown Enter 分支永远不导航、不
preventDefault，键盘用户无法激活该卡片（a11y 回退；注释「keyboard events keep the early return」与保留 onKeyDown
的意图自相矛盾）。建议按事件类型跳过 button 检查。

  function handleInternalLink(event: ReactMouseEvent<Element> | ReactKeyboardEvent<Element>, path: string, afterNavigate?: () => void): void {
-   if (event.defaultPrevented || (event as ReactMouseEvent<Element>).button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
+   // SyntheticKeyboardEvent 实例上不存在 button（React 只按事件接口拷贝属性），
+   // 键盘事件读取得到 undefined !== 0 恒早退——需按事件类型跳过 button 检查。
+   const isKeyboard = event.type.startsWith('key');
+   if (event.defaultPrevented || (!isKeyboard && (event as ReactMouseEvent<Element>).button !== 0) || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;


─── apps/web/src/platform/PlatformShell.tsx:218-218 ───
[maintainability · medium] 两个问题：1) career/careerSearch/careerRules 三个 label
硬编码中文（'求职档案'/'找岗'/'持续找岗'），绕过同数组其余条目统一使用的 t()/labels i18n 通道（i18n 资源中无 career 导航词条），非中文 locale
下侧栏仍显示中文；2) router.tsx 存在 /platform/career/opportunities/$opportunityId 与
/platform/career/evaluations/$evaluationId 详情路由，而 match 仅精确等于 /platform/career，进入详情页后侧栏无任何激活项。建议补
i18n 词条并将 match 扩展到两个详情子路径前缀。

-     { key: 'career', href: '/platform/career', label: '求职档案', icon: 'career', iconNode: <ReactOnlyNavIcon paths={['M4 7.5H20', 'M6 4.5H18V20H6Z', 'M9 12H15', 'M9 15.5H13']} />, match: (p: string) => p === '/platform/career' },
+     { key: 'career', href: '/platform/career', label: t('menu.career'), icon: 'career', iconNode: <ReactOnlyNavIcon paths={['M4 7.5H20', 'M6 4.5H18V20H6Z', 'M9 12H15', 'M9 15.5H13']} />, match: (p: string) => p === '/platform/career' || p.startsWith('/platform/career/opportunities') || p.startsWith('/platform/career/evaluations') },


─── apps/web/src/platform/PlatformShell.tsx:1-1 ───
[maintainability · low] 死导入：全文件无任何 `React.` 值引用（ReactNode/ReactMouseEvent/ReactKeyboardEvent 均已单独
type-import），tsconfig 为 jsx: "react-jsx" 也无需 React 命名空间做 JSX 运行时。建议删除该行。



─── apps/web/src/platform/PlatformShell.tsx:768-769 ───
[other · low] 渲染体内直写 ref 属于渲染期副作用，违反渲染纯净约束（当前赋值幂等无碍，但在 StrictMode/并发渲染语义下不被推荐，也会被 react-hooks lint
规则命中）。建议移入 useEffect 同步，5s 轮询读取 ref 的时序不受影响（effect 在 interval 回调触发前必然已提交）。

    const sessionActivityRef = useRef<SessionActivityEntries>({});
-   sessionActivityRef.current = sessionActivityEntries;
+   useEffect(() => { sessionActivityRef.current = sessionActivityEntries; }, [sessionActivityEntries]);


─── apps/web/src/platform/PlatformShell.tsx:1187-1187 ───
[maintainability · low] 未登记键会渲染 src="" 的 <img>（浏览器向当前页 URL 发起无效请求并显示裂图）：NavItem.icon 声明为宽泛的
string，career/careerSearch/careerRules/experts/market/analytics 六个键均不在 NAV_ICON_URLS 中，仅靠 iconNode
隐式兜底且 TS 不校验。未来新增条目若两者都漏配即触发。建议把 icon 类型收紧为 `keyof typeof NAV_ICON_URLS | (string & {})`，并在未登记且无
iconNode 时渲染 null。

-                         {item.iconNode ?? <img className="icon" src={iconPair ? (active ? iconPair.active : iconPair.default) : ''} alt="" />}
+                         {item.iconNode ?? (iconPair ? <img className="icon" src={active ? iconPair.active : iconPair.default} alt="" /> : null)}


─── apps/web/src/platform/PlatformShell.tsx:1106-1109 ───
[bug · low] 窄屏 overlay 形态两个交互缺口：1) .aside_box--mobile-overlay 为 position:fixed +
z-index:1001（platform-shell.td.css:1569），高于全局命令面板根（platform-u.css wk-cmdk-5 为 z-1000），从侧栏搜索按钮打开 ⌘K
面板时其左缘 260px 被侧栏覆盖；2) 点击导航项仅 navigate 不收起 overlay，窄屏跳转后路由内容仍被 260px 侧栏遮挡，需手动点折叠按钮。建议 overlay
层级低于命令面板，并在 isNarrowViewport 时于导航 onClick 内同步 setNarrowSidebarExpanded(false)。

-       <aside className={[
-         collapsed ? 'aside_box aside_box--collapsed' : 'aside_box',
-         isNarrowViewport && !collapsed ? 'aside_box--mobile-overlay' : '',
-       ].filter(Boolean).join(' ')}>
+                   onClick={(event) => {
+                     if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
+                     event.preventDefault();
+                     if (isNarrowViewport) setNarrowSidebarExpanded(false);
+                     navigate(item.href);
+                   }}


─── apps/web/src/platform/PlatformShell.tsx:1112-1112 ───
[style · low] 静态内联样式：cursor:pointer 不随状态变化，不应内联（项目规则仅允许动态样式内联）；platform-shell.td.css 已有 .aside_box
.logo_box 规则，建议并入该规则并移除 style 属性。

-           <a className="logo_box" style={{ cursor: 'pointer' }} href="/platform/knowledge-bases" aria-label="WeKnora" onClick={(event) => {
+ /* platform-shell.td.css */
+ .aside_box .logo_box {
+   cursor: pointer;
+ }


─── apps/web/src/platform/platform-u.css:386-388 ───
[style · low] wk-cmdk-36 内连续两条 transition-duration 声明（150ms 随即被 100ms 覆盖），是 codemod 平移
duration-150/duration-100 的残迹；原 utility 串目标值为 duration-100，建议删除 150ms 一行，避免读样式时误判生效时长。

    transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
-   transition-duration: 150ms;
    transition-duration: 100ms;


─── apps/web/src/router.tsx:716-720 ───
[bug · medium] careerRoute 是唯一未包 Suspense 的 career 家族路由：CareerPage 是 React.lazy 组件（第 47
行），此处直接渲染。createRouter（第 1008-1012 行）未配置 defaultPendingComponent，因此 chunk 未就绪时挂起会冒泡到 platformRoute 的
shellPage Suspense（第 211-215 行）——整个 PlatformShell 连同侧栏一起被 RoutePending 替换，首次进入 /platform/career
时整壳闪白，与同批新增的 careerSearchRoute/careerRulesRoute/careerOpportunityRoute/careerEvaluationRoute（均在页级
Suspense 内、壳保持稳定）行为不一致。

    const careerRoute = createRoute({
      getParentRoute: () => platformRoute,
      path: 'career',
-     component: (): ReactNode => <CareerPage client={client} scopeController={scopeController} userId={scopeController.current().scope.userId} />,
+     component: (): ReactNode => (
+       <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
+         <CareerPage client={client} scopeController={scopeController} userId={scopeController.current().scope.userId} />
+       </Suspense>
+     ),
    });


─── apps/web/src/router.tsx:35-37 ───
[documentation · low] 此注释在本次修改后已失实：packages/ui 已随 TDesign 迁移整体退役（仓库内已无该目录，pnpm-workspace 已裁剪），Craft
的共享装配不再"pulls in packages/ui 旧栈 theme.css"。注释继续宣称依赖一个不存在的包会误导后续维护者对 router
模块懒加载必要性的判断。另注：careerRoute 插入后，上方 "Octop M2 expert-template catalog" 注释现在紧邻 careerRoute 而非其描述的
expertsRoute，建议顺带归位。

- // Craft mounts through the shared @weknora/views/craft assembly, which pulls
- // in packages/ui 旧栈 (theme.css) — keep it lazy so the router module stays
- // importable under the node test runner.
+ // Craft mounts through the shared @weknora/views/craft assembly — keep it
+ // lazy so the router module stays importable under the node test runner.


─── apps/web/src/routes.tsx:59-59 ───
[maintainability · low] resolveRoute 对 /platform/career/opportunities/:id 强制要求非空 snapshotId（缺失判
not-found，routes.test.ts:45 已固化该口径），而 router.tsx 的 careerOpportunityRoute 用 `?? ''` 宽容渲染空
snapshotId。当前该 kind 的运行时消费方（main.tsx embed 判定、guardRoute 的 not-found allow 分支、routeRedirect 的非
platform 早退）使分歧无用户可见差异，但两套解析层口径不一致是潜在陷阱：任何未来消费 career-opportunity kind 的逻辑（预加载、埋点、外链校验）会把合法可渲染的深链判为
404。建议二者取其一：resolveRoute 容忍空 snapshotId（与路由器一致），或在 careerOpportunityRoute 中 snapshotId 为空时 throw
notFound()。



─── apps/web/src/settings/ChatHistorySettingsPanel.tsx:51-53 ───
[bug · medium] 防抖保存存在丢变更竞态：500ms 定时器触发时若上一笔 save 仍在途（慢网络下很常见），`if (savingRef.current) return`
会静默丢弃本次变更——无 toast、无重排队。时序：用户先拨开关（t=500ms 发起 save A，在途）→ 再选 embedding 模型（t=1000ms 定时器触发）→ 守卫命中直接
return，该编辑既不落库也无任何提示，界面仍显示新值。建议守卫命中时记录 pending 快照，在 finally 中检测 pending 并重新调度保存（或把 save 改为串行队列）。



─── apps/web/src/settings/ChatHistorySettingsPanel.tsx:41-46 ───
[bug · medium] save 成功后 `onSaved?.()` 触发壳层 `load(true)` 重取（SettingsPage L441），initialValue 更新为服务端值后此
effect 会无条件重置本地 state 与 latestRef：若用户在请求往返窗口内又做了未保存的编辑（或编辑因上一条评论的 in-flight
守卫被丢弃），将看到"选完又弹回"且该值从未持久化。建议回灌前先与 latestRef 的 pending 值 diff——仅当本地无未保存变更（或服务端值等于确认值）时才重置本地状态。



─── apps/web/src/settings/ChatHistorySettingsPanel.tsx:114-118 ───
[bug · low] embedding 模型选择丢失空列表回退：拆分前 ConfigSettingsPanel 的 chathistory 分支在 modelOptions 为空时回退为原生
Input 手输 embedding_model_id（retrieval 分支至今保留该回退，见同文件 `modelOptions.length === 0 ? <TInput .../>`）；壳层
models 预取失败时会 `setModels([])`（SettingsPage L242），此时本面板得到空下拉且未传 onAddModel——ModelSelector
中"前往全局设置添加模型"项点击后 `onAddModel?.()` 为空操作，用户完全无法设置 embedding 模型，属功能回退。建议补 onAddModel 跳转到 models
分区，或在空列表时保留手输回退。



─── apps/web/src/settings/CloudSettingsPanel.tsx:316-316 ───
[security · medium] 使用说明段落由原先安全的 `split('\n').map(line => <span className="block">{line}</span>)`
文本渲染改为 dangerouslySetInnerHTML 注入 `t(usageSteps).replace(/\n/g,'<br />')`。当前 i18n 值虽是静态纯文本（无 XSS
即时风险），但该写法绕过了 React 转义：今后任何语种文案若含 `<`/`>` 字符或该 key 加入插值（含用户可控数据）都会被浏览器直接解释。这是本次迁移引入的防护回归，且旧实现（span
逐行渲染）同样能达成换行效果，建议恢复文本节点渲染方式。

-         <p className="hint-text" dangerouslySetInnerHTML={{ __html: t('settings.weknoraCloud.usageSteps').replace(/\n/g, '<br />') }} />
+         <p className="hint-text">{t('settings.weknoraCloud.usageSteps').split('\n').map((line, index) => <span key={index} className="hint-line">{line}</span>)}</p>


─── apps/web/src/settings/CloudSettingsPanel.tsx:134-137 ───
[bug · low] 凭证校验由旧的 `!appId.trim() || !appSecret.trim()` 改为 `!form.appId || !form.appSecret`，且保存按钮
disabled 条件同样不再 trim：纯空白的 APPID/APPSECRET 会跳过本地化警告直接进入 cloudCredentialPatch（surface.ts:268-273 会
trim 后抛出英文 Error 'WeKnora Cloud app ID is required'），用户看到的是未本地化的报错而非 fillRequired 警告，行为回退。建议恢复 trim
判断。

-     if (!form.appId || !form.appSecret) {
+     if (!form.appId.trim() || !form.appSecret.trim()) {
        pushSettingsToast(t('settings.weknoraCloud.fillRequired'), 'warning');
        return;
      }


─── apps/web/src/settings/EnvVarSettingsPanel.tsx:84-86 ───
[bug · low] 移除原生 `required`（scopeId/name 两处 Input 与 sandbox Select 原先均带 required + 禁用空选项）后，空字段提交改走
setVariable 的 JS 守卫：value 为空有本地化 key（envVarSettings.valueRequired），但 name/scopeId 为空时落入 `envVarSet`
的 throw，catch 中 `errorText(reason, ...)` 优先取 reason.message——surface.ts L291-292 抛出的是硬编码英文 'Variable
name is required' / 'Skill ID is required' / 'Sandbox config ID is
required'，中文/日文等界面下用户会看到未本地化的英文报错。旧实现由浏览器原生校验拦截（消息随浏览器语言本地化），此处构成 i18n 回退。建议仿照 value 的守卫，为
name/scopeId 增加本地化的前置空值校验（或新增对应 i18n key 后在 catch 中优先展示）。

+     if (!name.trim()) { setNotice(null); setError(t('envVarSettings.valueRequired')); return; }
+     if (!scopeId.trim()) { setNotice(null); setError(t('envVarSettings.valueRequired')); return; }
      let mutation;
      try { mutation = envVarSet(scope, scopeId, name, value); }
      catch (reason) { setError(errorText(reason, t('envVarSettings.valueRequired'))); return; }


─── apps/web/src/settings/GeneralPreferencesPanel.tsx:351-351 ───
[other · low] 可达性回退：自动更新 Switch 丢失了 aria-label（原 aria-label={autoUpdateCopy.label}），且语言/主题/字体三处
Select 也由 aria-label 改为仅 placeholder——placeholder 不是可靠的 accessible
name，屏幕阅读器对这四行控件只能读到裸"开关/下拉"而无语义。建议恢复各控件的 aria-label（TDesign Switch/Select 均透传该属性）。



─── apps/web/src/settings/McpSettingsPanel.tsx:1237-1242 ───
[bug · low] 三个数值字段由 type="number" 改为纯文本 TInput 后，onChange 中 `Number(String(value))` 对非数字输入产生 NaN 并写入
draft，渲染时 String(NaN) 会把输入框内容直接替换为字面 "NaN"（如输入 "12a"），直到 onBlur
才归一化。保存路径已有保护（buildMcpConnectionPayload → normalizeMcpAdvancedNumber 对非有限值取 fallback，NaN 不会提交到后端），但
UI 瞬时显示 "NaN" 且破坏用户输入。建议 onChange/value 做 NaN 兜底（非有限值存 ""）。另：`className="wk-mcp-number-input"`
一行顶格缩进（列 0），与本文件其余 JSX 缩进不一致，建议修正。

                          <TInput
- className="wk-mcp-number-input"
+                           className="wk-mcp-number-input"
-                           value={draft.timeout === "" ? "" : String(draft.timeout)}
-                           onChange={(value) => setField("timeout", String(value) === "" ? "" : Number(String(value)))}
+                           value={draft.timeout === "" || !Number.isFinite(draft.timeout) ? "" : String(draft.timeout)}
+                           onChange={(value) => { const next = Number(String(value)); setField("timeout", String(value) === "" || !Number.isFinite(next) ? "" : next); }}
                            onBlur={() => setField("timeout", normalizeMcpAdvancedNumber(draft.timeout, 30, 1, 300))}
                          />


─── apps/web/src/settings/ModelDebugPanel.tsx:323-325 ───
[bug · low] TDesign InputNumber 清空时 onChange 回调 null，Number(null) === 0：三处均会把"清空输入框"变成 0——最坏情况 Max
Tokens 以 0 发起调试请求。同 PR 中 ModelSettingsPanel 已为此新增 fromTInputNumber 做空值归一（注释明确标注 ocr2-016
先例），此处应复用同一处理保持一致，例如 `setTemperature(fromTInputNumber(value))`。



─── apps/web/src/settings/ModelSelector.tsx:23-24 ───
[maintainability · medium] 此处内联了整套 contextWindow 工具（DEFAULT_MODEL_CONTEXT_WINDOW /
effectiveContextWindow / isDefaultContextWindow / formatTokenCount / formatContextWindow），但同目录
model-settings.ts 已导出全部同名同义函数（L129-182），ModelSettingsPanel 正是从那里导入使用——全仓现在共三份拷贝（model-chip.ts /
model-settings.ts / 本文件）。默认窗口 200000 与 1024 取整等格式化规则一旦调整将三处漂移（卡片 chip 与选择器 chip 显示不一致）。建议删除内联实现，直接复用
model-settings.ts 的导出。



─── apps/web/src/settings/ModelSelector.tsx:134-134 ───
[maintainability · low] `className="add-model-option"` 是死类名：全仓 CSS 无任何 `.add-model-option`
规则，该项样式实际由内部 div 的 `.model-option.add`（settings.td.css L2074 `.model-option.add
.model-name`）承载。若是为对齐 Vue 基线中 add 项的附加样式（如分隔线，参照 ModelOptionSelect 的 `__add` 有
border-top），该样式已丢失；否则应删除该残留类名，避免误导后续维护者以为存在对应规则。

-             <TSelect.Option key="__add_model__" value="__add_model__" className="add-model-option">
+             <TSelect.Option key="__add_model__" value="__add_model__">


─── apps/web/src/settings/ModelSettingsPanel.tsx:967-967 ───
[maintainability · low] loading 恒为 false 且全文件无任何状态来源（models 由壳层 initialModels 直传，loadingProviders
只服务于编辑器），该 TLoading 是无意义的死代码包装。若是为复刻 Vue t-loading 的 DOM
结构，应接入真实的列表加载状态；否则建议直接移除包裹层，避免误导后续维护者以为存在加载态。



─── apps/web/src/settings/OllamaSettingsPanel.tsx:54-54 ───
[style · low] connectionStatus 使用嵌套三元（testing ? null : status ? … : null），违反项目规约「禁止嵌套三元表达式」；同文件 JSX
中状态徽标处还有三层条件链（testing / true / false / 默认）。建议提取为具名函数或提前 return 的辅助函数以保持可读性。

-   const connectionStatus: boolean | null = testing ? null : status ? status.available === true : null;
+   const connectionStatus = (() => {
+     if (testing) return null;
+     return status ? status.available === true : null;
+   })();


─── apps/web/src/settings/OllamaSettingsPanel.tsx:234-239 ───
[bug · low] 下载轮询期间模型名输入框未禁用：进度轮询 setInterval 闭包持有下载发起时的 downloadModelName，完成/失败 toast 显示的是闭包旧值；而渲染区
`t('download.downloading', { name: downloadModelName })` 用的是实时
state——用户中途改名后进度条标签会显示新名字（任务实际跟踪旧名字），完成时 `setDownloadModelName('')`
还会清掉用户已输入的新内容。建议下载期间禁用输入框（disabled={downloading}），或把任务名存入独立 state 供进度展示使用。另：style={{ flex: 1 }}
为静态内联样式（同文件 marginTop: '8px'、CloudSettingsPanel width: '280px' 等同类），按规约应移入 CSS 类。

              <TInput
                value={downloadModelName}
                placeholder={t('ollamaSettings.download.placeholder')}
-               style={{ flex: 1 }}
+               className="download-input"
+               disabled={downloading}
                onChange={(value) => setDownloadModelName(String(value ?? ''))}
              />


─── apps/web/src/settings/PersonalMemoryPanel.tsx:196-196 ───
[bug · medium] 数值类控件由旧的 onBlur 触发保存改为 onChange 即调度 debouncedSave：TDesign InputNumber 的 onChange
逐键触发，(1) 点击清除按钮/清空输入时 Number(null)===0，500ms 后 memoryWorkspacePatch（surface.ts:174-180）对越界值抛错（如
extract delay 0 < 5、max_items 0 < 10），编辑过程中会弹出 saveFailed 错误 toast 且携带英文校验消息；(2) 多位输入中途停顿 ≥500ms
时，合法的中间值（如想输 90 时先落库 9、想输 1000 时先落库 1 抛错）会被持久化。建议数值字段保持 onChange 仅更新 draft、在 onBlur 时触发
debouncedSave（与本文件 textarea 的处理一致）。

-             <InputNumber value={draft.extract_delay_seconds} min={5} max={3600} step={15} suffix="s" disabled={!canEdit} onChange={(value) => { update({ extract_delay_seconds: Number(value) }); debouncedSave(); }} />
+             <InputNumber value={draft.extract_delay_seconds} min={5} max={3600} step={15} suffix="s" disabled={!canEdit} onChange={(value) => update({ extract_delay_seconds: Number(value) })} onBlur={() => debouncedSave()} />


─── apps/web/src/settings/PersonalMemoryPanel.tsx:118-118 ───
[maintainability · low] memoryWorkspacePatch 的返回类型是 Record<string, unknown>，而
client.settings.memory.workspace.update（api-client kvApi）的形参 SettingsPayload 同为 Record<string,
unknown>，本可直接赋值，`as never` 断言既无必要也会绕过所有类型检查（never 可赋给任意参数，若未来签名变化将静默放行错误 payload）。建议删除该断言。

-           ) as never);
+           ));


─── apps/web/src/settings/PersonalMemorySettingsPanel.tsx:791-793 ───
[style · low] 文件结尾缺少换行符（\ No newline at end of file），与仓库其余文件惯例不一致，会产生无意义 diff 噪音，建议补上结尾换行。

      </div>
    );
  }


─── apps/web/src/settings/PlatformApiKeysPanel.tsx:5-5 ───
[maintainability · low] 死导入：`Button`（未别名）在本文件中无任何 JSX 使用点（实际渲染均为 TButton），且与上一行 `import { Button as
TButton, ... } from 'tdesign-react'` 构成同模块重复导入。应删除 Button 或与上一条 import 合并。

- import { Alert, Button } from 'tdesign-react';
+ import { Alert } from 'tdesign-react';


─── apps/web/src/settings/PortedSectionsPanel.tsx:56-56 ───
[bug · low] wk-settings-read-note 是死类引用：全仓唯一存活规则是 settings-wrapper.css 的 `.env-settings
.wk-settings-read-note`（作用域限定），styles.css:70 注释也确认全局规则已删，而本组件不在 .env-settings 子树内；原先承载样式的
text-muted-strong/text-[.9rem] utilities 又被本次收编删除 → 该提示文本退化为默认段落样式（原为 muted-strong 色 + .9rem）。建议在
wrapper.css 增加全局（或 PortedSections 作用域）的 .wk-settings-read-note 规则，或直接复用已有全局类
wk-ported-note/wk-muted。

-       ? <p className="wk-settings-read-note">No rows were returned by the API for this section.</p>
+       ? <p className="wk-ported-note">No rows were returned by the API for this section.</p>
+       /* 或在 settings-wrapper.css 补全局规则：.wk-settings-read-note { color: var(--color-muted-strong, #506078); font-size: 0.9rem; } */


─── apps/web/src/settings/ResourceSettingsPanel.tsx:234-235 ───
[bug · medium] 数字字段从 `Input type="number"` 迁到 TInput 后丢失了数字类型与 min/max 约束（min=1、max=field.max ?? 副本数
10 / 其余 64），移动端数字键盘与步进也随之消失，越界值只能靠后端兜底；同时 `isReplicaField` 仅剩 import（13 行）成为死导入。建议改用 TInputNumber（带
min/max）或恢复数字输入约束，并移除未使用的 isReplicaField 导入。

          onChange={(value) => {
-           const text = String(value).trim();
+           const text = value == null ? '' : String(value).trim();


─── apps/web/src/settings/ResourceSettingsPanel.tsx:950-950 ───
[maintainability · low] `theme: 'error' as never` 用断言绕过类型检查：同批次 SandboxSettingsPanel.cardMenuOptions
对同一 TDesign Dropdown option 直接写 `theme: 'error'` 并显式声明 `theme?: 'error'`，两种口径必有一处与 DropdownOption
类型不符。建议去掉 as never，与 SandboxSettingsPanel 的写法统一（或抽一个共享的 option 构造函数）。

-                     { content: t('common.delete'), value: 'delete', theme: 'error' as never },
+                     { content: t('common.delete'), value: 'delete', theme: 'error' },


─── apps/web/src/settings/RuntimeQueuesPanel.tsx:387-387 ───
[maintainability · low] ClockIcon 的最后一个使用点被 TIcon name="time" 替换后，函数定义（本文件 484
行附近）已无任何引用，成为死代码，应一并删除（ErrorIcon 在 235 行 error 分支仍有使用，需保留）。



─── apps/web/src/settings/SandboxSettingsPanel.tsx:1331-1331 ───
[bug · high] TInputNumber 清空回调值是 null 而非空字符串，`value === ''` 分支永假（同仓库
ModelSettingsPanel.fromTInputNumber 的注释已明确此行为，ocr2-016 修复记录）。后果：清空 docker 数值字段时 `Number(null)` 写入
0（如 idle_ttl_seconds/cpu_limit/memory_limit_mb/pids_limit 及 cube/e2b 超时字段，"未设置"被污染为
0）；defaultTimeoutSec/terminalIdleDisconnectSec 走 `String(null)` 写入字符串 "null"。建议参照 fromTInputNumber 将
null/undefined 归一为清空语义后再 Number()/String()。

-                   <TInputNumber min={0} max={Number.MAX_SAFE_INTEGER} placeholder="1800" value={numberInputValue(form.docker.idle_ttl_seconds)} onChange={(value) => setDocker({ idle_ttl_seconds: value === '' ? undefined : Number(value) })} />
+                   <TInputNumber min={0} max={Number.MAX_SAFE_INTEGER} placeholder="1800" value={numberInputValue(form.docker.idle_ttl_seconds)} onChange={(value) => setDocker({ idle_ttl_seconds: value == null || value === '' ? undefined : Number(value) })} />


─── apps/web/src/settings/SandboxSettingsPanel.tsx:0-0 ───
[maintainability · low] 可访问性回退：原实现是 Enter 或 Space 均可激活（role="button" 元素按 WAI-ARIA 应支持空格激活），迁移后
keyDown 只判 Enter，Space 将滚动页面而不打开编辑器。建议恢复 `event.key === 'Enter' || event.key === ' '`（注意 Space 需
preventDefault）。

-                     onKeyDown={(event: KeyboardEvent<HTMLDivElement>) => { if (event.key === 'Enter') openEdit(item); }},
+                     onKeyDown={(event: KeyboardEvent<HTMLDivElement>) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); openEdit(item); } }},


─── apps/web/src/settings/SettingsPage.tsx:113-113 ───
[bug · high] envvars 静默失败回归（ocr2-017 同款遗漏）：envvars 在 SELF_HEADER_SECTIONS 早退分支中，但不在
SHELL_FEEDBACK_SELF_PANELS 白名单内；而 EnvVarSettingsPanel 并不自拉环境变量列表（仅自取 sandbox-configs），行数据完全依赖壳层
payload，且其 sectionErrorMode 为 'inline'（load 失败会 setError）。早退分支对非白名单面板不渲染任何错误/加载占位 → 壳层加载失败时面板以 null
payload 静默渲染空列表，用户无法区分「失败」与「确实无配置」。改动前（legacy 分支）会渲染 inline 错误 Status，属回归。与 chathistory/memory
同款成因，建议将 'envvars' 一并加入白名单（或让面板自加载）。

- const SHELL_FEEDBACK_SELF_PANELS = new Set(['chathistory', 'memory']);
+ // envvars 与 chathistory/memory 同款：只消费壳层 payload、无自持 loading/错误态，
+ // 壳层须为其渲染 inline 错误/加载占位。
+ const SHELL_FEEDBACK_SELF_PANELS = new Set(['chathistory', 'memory', 'envvars']);


─── apps/web/src/settings/SettingsPage.tsx:556-560 ───
[maintainability · low] 嵌套三元表达式（评审规则明确禁止），且 SELF_HEADER 分支与下方 legacy 分支的 Suspense
面板链高度重复、各含永不可达的死分支（legacy 链中的 chatHistoryPanel/systemPanel —— 二者所在 key 均已进 SELF_HEADER_SECTIONS
早退；SELF_HEADER 链中的 chatPreferencesPanel/resourcePanel/usagePanel/queryHistoryPanel —— 对应 key
永不走该分支）。建议改为 if/else 或查表函数计算 wrapperModifier，并抽出公共的面板渲染链（如 panelForKey(key) 函数）供两条分支复用，消除双链漂移风险。

-   const wrapperModifier = selectedKey === 'members'
-     ? 'content-wrapper--wide wks-content-wrapper--wide'
-     : SYSTEM_ADMIN_SECTIONS.has(selectedKey) || integrationTab
-       ? 'content-wrapper--full wks-content-wrapper--full'
+   const wrapperModifier = 
+     selectedKey === 'members' ? 'content-wrapper--wide wks-content-wrapper--wide'
+     : SYSTEM_ADMIN_SECTIONS.has(selectedKey) || integrationTab ? 'content-wrapper--full wks-content-wrapper--full'
-       : '';
+     : '';
+   // 或改为 if/else 赋值，避免嵌套三元。


─── apps/web/src/settings/SettingsPage.tsx:595-597 ───
[other · low] 可达性回退：导航项由原生 <button> 改为 div[role=button]（Enter/Space 已处理），但 settings.td.css 的
.nav-item 只定义了 :hover/.active，无任何 :focus-visible 样式；叠加 settings-wrapper.css 的 html:not(.wk-kbd-nav)
.wk-settings-drawer-root :focus-visible { outline: none } 门控，键盘焦点指示只剩浏览器默认轮廓（且在非 kbd-nav
模式下被抑制）。另外弹层根节点由 <main> 改为 <div> 丢失了 landmark 语义。建议在 settings.td.css 为 .nav-item 补 :focus-visible
规则，并评估保留 main 或以 role 属性补回 landmark。

                          className={'nav-item' + (item.key === selectedKey ? ' active' : '')}
                          role="button"
                          tabIndex={0}
+                         /* 并在 settings.td.css 补：.wk-settings-drawer-root .nav-item:focus-visible { outline: 2px solid var(--td-brand-color); outline-offset: -2px; } */


─── apps/web/src/settings/SettingsPage.tsx:113-113 ───
[bug · medium] SHELL_FEEDBACK_SELF_PANELS 白名单遗漏 mymemory（与已确认的 envvars 问题同构）：mymemory 在
SELF_HEADER_SECTIONS 早退分支中，但不在本白名单内；PersonalMemorySettingsPanel 只接收 client/initialSettings（无
error/loading props），initialSettings 完全来自壳层 payload，且其内部自拉 loadSettings 的 catch 仅
console.error、不设置面板自身的错误态。sectionErrorMode('mymemory') 为 'inline'（壳层失败会
setError），但早退分支对非白名单面板不渲染任何错误/加载占位 → 壳层加载失败时面板以 null settings 静默渲染默认状态（开关关闭、列表为空），改动前 legacy 分支会渲染
inline 错误，属回归。建议将 'mymemory' 加入白名单（或为面板补 error/onRetry props 并同步锚定测试）。

- const SHELL_FEEDBACK_SELF_PANELS = new Set(['chathistory', 'memory']);
+ const SHELL_FEEDBACK_SELF_PANELS = new Set(['chathistory', 'memory', 'mymemory']);


─── apps/web/src/settings/SettingsPage.tsx:545-549 ───
[maintainability · low] 死分支：该 banner-retry 横幅块已不可达。sectionErrorMode 返回 'banner-retry' 的四个 key
中，parser/system/userprofile 均已进 SELF_HEADER_SECTIONS 早退分支（永不到达 legacy 分支），members ∈
SELF_ERROR_SECTIONS 在 load() 中被 setError(null) → 条件 sectionError && sectionErrorMode(key) ===
'banner-retry' 恒为假。连带仅被此块使用的 TAlert/TButton import（第 11 行）与内部 key === 'members' || key === 'storage'
三元（storage 为 silent，同样永不可达）一并成为死代码。建议删除该分支及对应 import，或随 SELF_HEADER 收编一并清理。



─── apps/web/src/settings/SettingsPage.tsx:507-507 ───
[style · low] 嵌套三元表达式（A ? B : C ? D : null，评审规则明确禁止嵌套三元）：shellFeedback 的错误/加载占位链与已确认的
wrapperModifier、toast tone 问题同属一类。建议提取为局部函数/提前返回（如 renderShellFeedback()），条件扩张时也更易维护。

-           {shellFeedback && sectionError ? <Status tone="error">{sectionError}</Status> : shellFeedback && sectionLoading ? <Status>{t('common.loading')}</Status> : null}
+           {(() => {
+             if (shellFeedback && sectionError) return <Status tone="error">{sectionError}</Status>;
+             if (shellFeedback && sectionLoading) return <Status>{t('common.loading')}</Status>;
+             return null;
+           })()}


─── apps/web/src/settings/SkillSettingsPanel.tsx:237-238 ───
[maintainability · medium] SandboxBackendBadge 与 SandboxSettingsPanel.tsx（约 1631
行）逐行重复实现（providerLogo mask + TDesign glyph 回落），且两份类型签名已经漂移（此处 `type?: string`，沙箱面板为 `type:
string`）。徽章配色/图标后续调整极易双份不一致，建议提取到共享模块（如 providerLogos.ts 同级）供两个面板复用。



─── apps/web/src/settings/SkillSettingsPanel.tsx:1368-1368 ───
[bug · low] TTextarea 的长度限制属性是小写 `maxlength`（本批次 TInput 处已统一用小写 `maxlength={128}` 口径），传 camelCase
`maxLength` 不会进入组件的长度限制逻辑（guidanceText 的 10000 字上限失效）。

-       <TTextarea value={guidanceText} maxLength={10000} rows={2} disabled={sendingGuidance}
+       <TTextarea value={guidanceText} maxlength={10000} rows={2} disabled={sendingGuidance}


─── apps/web/src/settings/SkillSettingsPanel.tsx:255-257 ───
[maintainability · low] 可访问性回退：帮助提示触发器由可聚焦的 <button> 改为 TIcon（span，不可聚焦），键盘用户无法再触达 hover 型
tooltip。建议包一层 button/tabIndex=0（可结合 TTooltip 的 focus 触发）恢复键盘可达性。



─── apps/web/src/settings/SystemAuditLogPanel.tsx:86-86 ───
[bug · medium] 表格域样式迁移后存在跨挂载点作用域缺口：wk-audit-table / wk-audit-time / wk-audit-actor / wk-audit-target
/ wk-audit-tag(--* tone) / wk-audit-load-more 的规则仅存在于 settings-wrapper.css 中
`.wk-settings-drawer-root` 前缀下（1076-1095 行）。本面板除 SettingsPage 抽屉外还挂载于 AdministrationPage（wk-admin-19
容器，不在 .wk-settings-drawer-root 内），该挂载点下表格将失去全部边框/对齐/tone 配色/hover 样式（旧实现的 Tailwind utilities
在任意挂载点均生效）。建议将这组规则的作用域放宽为 `.system-audit-log`（面板自持根类，两种挂载点都命中）。

-   <div className="audit-page-body">
+   <div className="audit-page-body"> {/* 配套将 .wk-settings-drawer-root .wk-audit-* 表格规则改为 .system-audit-log 前缀，覆盖 AdministrationPage 挂载点 */}


─── apps/web/src/settings/SystemGlobalSettingsPanel.tsx:474-474 ───
[bug · medium] int 类型配置项从 NumberInput 迁移到 tdesign InputNumber 时丢失了原有 max={9999} 上限，仅剩
min={minimumFor(item.key)} 约束；且 onBlur 直接将值 Number(value) 交给 persist，persist 内无任何钳制逻辑（仅透传
client.administration.settings.update）。若后端也不校验，管理员可写入超界运行时配置（如并发数、配额类数值）。建议补回 max（或按 key 建立上限表）。

-                         ? <InputNumber className="setting-input" value={typeof current === 'number' ? current : Number(current ?? 0)} min={minimumFor(item.key)} disabled={itemSaving} aria-label={keyLabel(item.key)} theme="normal" step={1} placeholder={t('system.globalSettings.tagInputPlaceholder')} onChange={(value) => { setEditValues((state) => ({ ...state, [item.key]: value })); }} onBlur={(value) => { const parsed = value === '' || value === null || value === undefined ? null : Number(value); if (parsed !== null && !Number.isNaN(parsed)) void persist(item, parsed); }} />
+                         ? <InputNumber className="setting-input" value={typeof current === 'number' ? current : Number(current ?? 0)} min={minimumFor(item.key)} max={9999} disabled={itemSaving} aria-label={keyLabel(item.key)} theme="normal" step={1} placeholder={t('system.globalSettings.tagInputPlaceholder')} onChange={(value) => { setEditValues((state) => ({ ...state, [item.key]: value })); }} onBlur={(value) => { const parsed = value === '' || value === null || value === undefined ? null : Number(value); if (parsed !== null && !Number.isNaN(parsed)) void persist(item, parsed); }} />


─── apps/web/src/settings/SystemGlobalSettingsPanel.tsx:395-395 ───
[bug · medium] React 保留段（消息条 / ConfirmInline / PriorityHint 弹层）的类名规则（settings-wrapper.css 1000-1013
行）均挂在 `.wk-settings-drawer-root` 前缀下，但本面板同时挂载于 AdministrationPage（不在该作用域内），该挂载点下：PriorityHint 弹层丢失
position:absolute 等定位规则，hover 时会以静态块插入标题行内破坏布局；消息条与确认气泡丢失全部样式。与 SystemAuditLogPanel
同族的跨挂载点作用域问题，建议将这组规则的作用域改为 `.system-settings`（面板自持根类）。

-     {message ? <p className={'wk-system-global-message' + (messageTone === 'success' ? ' is-success' : ' is-error')} role="status">{message}</p> : null}
+     {message ? <p className={'wk-system-global-message' + (messageTone === 'success' ? ' is-success' : ' is-error')} role="status">{message}</p> : null} {/* 配套将 .wk-settings-drawer-root .wk-system-global-* 规则改为 .system-settings 前缀 */}


─── apps/web/src/settings/SystemGlobalSettingsPanel.tsx:6-6 ───
[maintainability · low] 本行导入的 Button / Input / Switch 未加别名且在全文件 JSX 中无任何使用点（所有用法均走第 4 行的
TButton/TInput/TSwitch 别名；Select/Tag/TagInput/Loading/InputNumber/Tabs 有使用），属冗余死导入，且与第 4 行构成同模块双
import 语句。建议合并为一条 import 并移除未用标识符，避免 lint（no-unused-vars）报错与阅读干扰。

- import { Button, Input, InputNumber, Loading, Select, Switch, Tag, TagInput, Tabs } from 'tdesign-react';
+ import { InputNumber, Loading, Select, Tag, TagInput, Tabs } from 'tdesign-react';


─── apps/web/src/settings/SystemGlobalSettingsPanel.tsx:465-465 ───
[bug · medium] i18n 双重插值 bug：modifiedMeta 辅助函数（140-145 行）内部已经调用
t('system.globalSettings.modifiedAt', { value: `${formatDate(...)} · ${actor}` })
返回完整本地化文本，此处渲染又把它的返回值作为 value 再次套入同一个模板，界面上会出现重复前缀（如「上次修改：上次修改：2026/… · user」）。所有带 last_modified_by
且 updated_at 有效的配置行均受影响。建议直接渲染 modifiedMeta(item) 本身，不要再包一层 t()。

-                 {modifiedMeta(item) ? <div className="setting-meta">{t('system.globalSettings.modifiedAt', { value: modifiedMeta(item) })}</div> : null}
+                 {modifiedMeta(item) ? <div className="setting-meta">{modifiedMeta(item)}</div> : null}


─── apps/web/src/settings/SystemGlobalSettingsPanel.tsx:8-8 ───
[maintainability · low] useRef 为死导入：全文件仅此一处出现 useRef，无任何使用点（注释所述「onBlur 不再经 ref」的 OCR ocr2-018 修复已把
ref 方案移除，但导入残留）。建议从导入中删掉 useRef，避免 no-unused-vars 报错。

- import { useEffect, useMemo, useRef, useState } from 'react';
+ import { useEffect, useMemo, useState } from 'react';


─── apps/web/src/settings/SystemInfoPanel.tsx:93-93 ───
[maintainability · low] 版本漂移告警 Tag 以魔法下标 index === 1 锚定 systemInfoRows 的第二行（当前确为 frontendVersion
行，语义正确），但该行序由 surface.ts 的 systemInfoRows 决定，一旦行序调整（如新增行或重排），告警会静默挂到错误行上。建议改为按行标识锚定（如 row.labelKey
=== 'system.frontendVersionLabel'）。

-                   {index === 1 && versionMismatch ? <Tag theme="warning" variant="light" size="small" style={{ marginLeft: '8px' }}>{t('system.versionMismatch')}</Tag> : null}
+                   {row.labelKey === 'system.frontendVersionLabel' && versionMismatch ? <Tag theme="warning" variant="light" size="small" style={{ marginLeft: '8px' }}>{t('system.versionMismatch')}</Tag> : null}


─── apps/web/src/settings/TenantDeleteZone.tsx:91-97 ───
[bug · low] 确认弹窗中的租户名输入框丢失了原有的 aria-label={t('tenant.deleteDangerZone.confirmTitle')}（旧实现有），当前 Input
仅有 placeholder，屏幕阅读器无法获知该输入框的用途。建议补回 aria-label。另注：error 提示用了静态 inline style（color/fontSize/margin
固定值），按规范宜并入 error-inline 类规则。

            <Input
              placeholder={tenantName}
+             aria-label={t('tenant.deleteDangerZone.confirmTitle')}
              disabled={busy}
              clearable
              value={confirmText}
              onChange={(value) => setConfirmText(String(value ?? ''))}
            />


─── apps/web/src/settings/TenantMembersPanel.tsx:770-773 ───
[bug · high]
权限说明弹层的类名（permissions-popup-overlay、permissions-compact*、perm-role-block、perm-items/perm-item、me-bad
ge）在整个仓库的 CSS（settings.td.css / settings-wrapper.css 及所有 *.css）中均无任何规则定义。旧实现使用内联 Tailwind
utilities（卡片圆角边框、grid auto-fit 210px、✓/✗ 配色、me
徽章等），迁移后这些样式全部丢失，弹层内容将以无样式的裸文本堆叠呈现。另外参照同文件族先例（settings.td.css:670-673
user-profile-password-popup-overlay 需 z-index:3050 才能浮于设置全屏遮罩之上），permissions-popup-overlay 缺少 body
弹层的 z-index 处理，弹层可能被设置抽屉遮罩遮挡。需要在样式表中补齐这组规则。

            <TPopup
              placement="bottom-left"
              trigger="hover"
-             overlayClassName="permissions-popup-overlay"
+             overlayClassName="permissions-popup-overlay" /* 需在 settings.td.css 补齐 .permissions-popup-overlay（z-index ≥ 设置遮罩）与 .permissions-compact* / .perm-* / .me-badge 内容规则 */


─── apps/web/src/settings/TenantMembersPanel.tsx:709-710 ───
[bug · medium] invitationTableColumns 的 useMemo 依赖数组为 [tr, canManage, locale]，遗漏了 cell 闭包中 revoke
所捕获的 busy 状态。memo 命中期间单元格持有过期闭包，revoke 开头的 `if (!canManage || busy) return;` 并发守卫将读到过期值
busy=false；且新的 TPopconfirm confirmBtn 未设置 disabled={busy}（旧实现有
disabled={busy}），在撤销请求进行中再次点击确认会绕过守卫发出重复 revoke 请求。memberTableColumns 虽已含 busy，但其 update/remove 调用的
load() 以默认参数捕获 query/page/pageSize，搜索条件变化后列 memo 不重算，仍会用旧搜索词重载列表，建议一并确认。

      }] : []),
-   ]), [tr, canManage, locale]);
+   ]), [tr, canManage, busy, locale]); // 并建议 confirmBtn 增加 disabled: busy


─── apps/web/src/settings/TenantMembersPanel.tsx:988-989 ───
[bug · low] 邀请邮箱输入由原生 `<Input required type="email">` 改为 `<TInput
type="text">`，丢失了浏览器原生的必填与邮箱格式校验。submitInvite 中仅有 `!inviteEmail.trim()` 的空值守卫：邮箱为空时点击提交按钮会静默
return，用户得不到任何反馈（旧实现会被原生 required 拦截并提示）；格式非法的输入（如 "abc"）则直接发往后端依赖后端报错。建议补回约束（TInput 加 rules/正则或
type email 语义 + 空值行内提示）。

            <TInput type="text" className="wk-tenant-invite-input" value={inviteEmail} placeholder={tr('tenantMember.add.emailPlaceholder')}
+             status={inviteEmail && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(inviteEmail) ? 'error' : undefined}
+             tips={inviteEmail && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(inviteEmail) ? tr('tenantInvitation.errors.invalidEmail') : undefined}
              onChange={(value) => setInviteEmail(String(value))} />


─── apps/web/src/settings/TenantMembersPanel.tsx:677-680 ───
[maintainability · low] 本次替换为 TPopup/TPopconfirm 自管可见性后，旧的 permissionsOpen/permissionsRef 及其外点关闭
useEffect（约 505-515 行）、revokeConfirmKey/removeConfirmKey 两个 state（约 368-369 行）成为死代码：JSX 已无任何路径将它们置为
true/非空，setter 仅在 revoke/remove 内以 null 调用，effect 因 permissionsOpen 恒为 false 永不挂载监听。建议连同
setRevokeConfirmKey(null)/setRemoveConfirmKey(null) 调用一并删除。

- <div className="relative inline-flex" ref={permissionsRef}>
-             <button type="button" className="inline-flex h-6 w-6 cursor-pointer items-center justify-center rounded-full border-0 bg-transparent p-0 text-[rgba(0,0,0,0.6)] hover:text-primary hover:[outline:none] focus-visible:text-primary focus-visible:outline-offset-2 focus-visible:[outline:var(--wk-focus-ring,3px_solid_rgb(46_109_230/35%))]" aria-label={tr('tenantMember.permissions.title')}
-               title={tr('tenantMember.permissions.iconHint')} aria-expanded={permissionsOpen}
-               onClick={() => setPermissionsOpen((open) => !open)}>
+             <button type="button" className="permissions-trigger-btn" aria-label={tr('tenantMember.permissions.title')} title={tr('tenantMember.permissions.iconHint')}
+               aria-haspopup="dialog">
+               <TIcon name="info-circle" size="16px" />
+             </button>
+ {/* 同时删除 permissionsOpen/permissionsRef/revokeConfirmKey/removeConfirmKey 及关联 effect、setter 调用 */}


─── apps/web/src/settings/TenantMembersPanel.tsx:770-773 ───
[bug · low] 无障碍回归：权限说明由原「可聚焦 button + aria-expanded + onFocus/onBlur 展开」改为 TPopup
trigger="hover"，且新触发按钮未保留任何键盘展开路径（无 focus 触发、无 aria-expanded/aria-haspopup），纯键盘用户将无法查看权限矩阵。旧实现 focus
即展开。建议 trigger 改为含 focus 的方案或补充键盘可展开的交互（并补 aria 属性）。

            <TPopup
              placement="bottom-left"
-             trigger="hover"
+             trigger="hover focus"
              overlayClassName="permissions-popup-overlay"


─── apps/web/src/settings/TenantMembersPanel.tsx:281-282 ───
[maintainability · medium] 死代码：TablePager 迁移到 tdesign Pagination 后，组件内的 maxPage、jump/setJump
state、同步 useEffect 与 commitJump（当前 267-275 行）以及模块级 pageWindow 函数（当前 240-252 行）已无任何调用点（旧的自绘分页 JSX
已删除；AdministrationPage 使用的是自己文件内的同名副本，与本文件无关）。注意 load() 内 377-378 行另有一个同名 maxPage
局部变量仍在使用，不在清理范围。建议整块删除这约 30 行死代码。



─── apps/web/src/settings/TenantMembersPanel.tsx:4-4 ───
[style · low] 同模块重复 import：本行与下方 `import { Button as TButton, Input as TInput, Pagination, ... }
from 'tdesign-react'` 构成对 'tdesign-react' 的两条导入语句（import/no-duplicates）。建议将 Dialog as TDialog
并入另一条导入，合并为一条。

- import { Dialog as TDialog } from 'tdesign-react';
+ import { Button as TButton, Dialog as TDialog, Input as TInput, Pagination, Popconfirm as TPopconfirm, Popup as TPopup, Select as TSelect, Table, Tag } from 'tdesign-react';


─── apps/web/src/settings/TenantUserProfileSections.tsx:312-312 ───
[maintainability · low] 删除成功后的跳转路径 '/login' 为硬编码业务 URL 路径（同文件 usage > 80 的告警阈值亦为硬编码业务数值）。建议提取为具名常量（如
REDIRECT_AFTER_DELETE = '/login'、USAGE_WARN_THRESHOLD = 80），便于后续统一调整；仓库内其他面板（如 SandboxSettingsPanel
的 CLUSTER_GUIDE_URL）均采用常量约定。

-             <TenantDeleteZone client={client} tenantId={tenantId} tenantName={currentName || String(tenantId)} onDeleted={() => { window.location.assign('/login'); }} />
+             <TenantDeleteZone client={client} tenantId={tenantId} tenantName={currentName || String(tenantId)} onDeleted={() => { window.location.assign(REDIRECT_AFTER_TENANT_DELETE); }} />


─── apps/web/src/settings/TenantUserProfileSections.tsx:353-353 ───
[maintainability · low] 重复常量定义：PASSWORD_SPECIAL_CHARS 与 ../auth/validation.ts 第 5 行导出的同名常量逐字符相同（本仓库
SystemGlobalSettingsPanel 即从该处导入使用）。本地重新定义存在漂移风险——密码策略调整只改 validation.ts
时，此处的特殊字符正则会与注册/重置密码校验静默失配。建议删除本地声明，改为从 '../auth/validation.ts' 导入。

- const PASSWORD_SPECIAL_CHARS = '!@#$%^&*()_+-=[]{}|;:,.<>?';
+ import { PASSWORD_SPECIAL_CHARS } from '../auth/validation.ts';


─── apps/web/src/settings/TenantUserProfileSections.tsx:311-311 ───
[documentation · low] 注释与实现不一致：注释声称危险区「owner 且为当前空间时渲染……与 Vue evaluateLeaveGate 同判」，但实际门控是
canEditTenant（owner || system-admin），system-admin 即使不是当前空间 owner 也会渲染「删除工作空间」入口，与所述 Vue owner-only
行为不符。建议或将门控收紧为 role === 'owner'，或修正注释明确说明 system-admin 因 React 侧档位折叠同样放行该入口。



─── apps/web/src/settings/settings-toast.tsx:63-67 ───
[maintainability · low] 嵌套三元表达式（tone → className 三层，评审规则明确禁止）。tone 类型已收窄为 'error' | 'success' |
'warning'，建议用查表对象消除嵌套，新增色调时也更不易漏改。

-               ? 'wk-settings-toast wk-settings-toast--error'
-               : toast.tone === 'warning'
-                 // MessagePlugin.warning 琥珀色语义（WeKnoraCloud 部分成功/fillRequired，T12b）。
-                 ? 'wk-settings-toast wk-settings-toast--warning'
-                 : 'wk-settings-toast wk-settings-toast--success'
+ /* 模块级：const TOAST_TONE_CLASS: Record<SettingsToast['tone'], string> = {
+    error: 'wk-settings-toast--error',
+    warning: 'wk-settings-toast--warning', // MessagePlugin.warning 琥珀色语义（T12b）
+    success: 'wk-settings-toast--success',
+  } */
+ // JSX：className={`wk-settings-toast ${TOAST_TONE_CLASS[toast.tone]}`}


─── apps/web/src/settings/settings.td.css:5000-5005 ───
[bug · medium] 该规则与 settings-wrapper.css:599 的 `.wk-settings-panel-heading`（0,1,0：margin-bottom
32px、无分割线）冲突且特异性更高（0,2,0），会命中设置抽屉内所有同名元素——不只 models 编辑抽屉：SettingsPage legacy
分支壳层标题（SettingsPage.tsx:537，影响
usage/query-history/retrieval/storage/vectorstore/websearch/chat-preferences 等分区）以及
McpSettingsPanel(:476/:999/:1275)、SandboxSettingsPanel(:1091/:1969)、ModelSettingsPanel(:1104)、ModelD
ebugPanel(:234) 的标题都会被强加上 sticky/top:-1.25rem/白色背景/底部分割线，并把 32px 下边距压成 1rem，与 Vue「无分割线 + 裸 32px
margin」语义相悖。settings-wrapper.css:708 的注释「margin-bottom 由共享 .wk-settings-panel-heading
规则继续供给」在该覆盖关系下已不成立。建议将本规则收窄到实际消费点（models 编辑抽屉），如 `.wk-settings-drawer-root .wk-model-editor-drawer
.wk-settings-panel-heading`，或改用抽屉专属类名。

- .wk-settings-drawer-root .wk-settings-panel-heading {
+ /* 收窄到 models 编辑抽屉内的标题，避免覆盖壳层/其他面板的
+    .wk-settings-panel-heading（wrapper.css：32px 下边距、无分割线）。 */
+ .wk-settings-drawer-root .wk-model-editor-drawer .wk-settings-panel-heading {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
    border-bottom: 1px solid #eef1f5;


─── apps/web/src/settings/settings.td.css:3421-3427 ───
[bug · medium] .hint-popover 在本文件内被平移了两次且取值冲突：§4（env，~752 行）为 gap: 12px、__text margin: 4px 0 0；本处
§14（sandbox）为 gap: 4px、__text margin: 0。两块均为 unscoped 全局规则、同特异性，后者按文件顺序覆盖前者 → envvars 的提示弹层间距被
sandbox 版本污染（gap 12→4、首段 margin 归零）。两个 Vue 事实源（EnvVarSettings.vue 与 SandboxSettings.vue）各自 scoped
本不冲突，平移到同一全局作用域后撞名。建议按来源改名（如 .env-hint-popover / .sandbox-hint-popover）或合并为一套取值。

- /* hint-popover 服务 t-popup portal 到 body 的内容（§2.5：原样平移不加前缀）。 */
- .hint-popover {
+ /* SandboxSettings.vue 的提示弹层，与 §4 EnvVarSettings 的 .hint-popover 撞名，
+    改名隔离避免全局级联互相覆盖： */
+ .sandbox-hint-popover {
    display: flex;
    flex-direction: column;
    gap: 4px;
    max-width: 340px;
  }


─── apps/web/src/settings/settings.td.css:5349-5352 ───
[maintainability · low] @keyframes wk-sandbox-inventory-enter 在本文件定义了两次且取值不一致：§14 末尾（~3802 行）为 from
{ translateX(18px); opacity: .7 }，本处为 from { translateX(24px); opacity: 0 }。同名 keyframes
后定义整体覆盖前定义，生效值取决于文件内顺序而非事实源，留下维护陷阱。请删除其一并统一取值（两处注释各自声称「原样式随迁/原 utility 串平移」，应核对 sandbox-settings.css
原 keyframes 实际值）。

- @keyframes wk-sandbox-inventory-enter {
-   from { transform: translateX(24px); opacity: 0; }
-   to { transform: translateX(0); opacity: 1; }
- }
+ /* 与 §14 末尾同名 keyframes 重复（该处 from 为 translateX(18px)/opacity .7），
+    保留其中与事实源一致的一份，删除另一份。 */


─── apps/web/src/settings/settings.td.css:2308-2312 ───
[maintainability · low] @keyframes spin 未在 settings.td.css 中定义，注释依据有误：注释声称「kb-list.td.css
已有同名同值定义」，但 kb-list.td.css 仅由 KnowledgeBasesPage.tsx（懒加载页面 chunk）导入——用户未访问 KB 列表页时该文件并不在 bundle
中。当前实际兜底是 PlatformShell.tsx 静态导入的 platform-u.css（settings 抽屉挂在 platformRoute
下必然已加载），功能暂时可用，但这是隐式跨文件依赖：platform 域清理 spin 时会静默破坏 ollama 状态图标的旋转。与本文件
fadeIn/skill-chip-dot/rq-refresh-rotate 的本地自持口径不一致，建议按同款做法本地定义 @keyframes spin。

  .wk-settings-drawer-root .ollama-settings .status-display .status-icon.spinning {
-   /* keyframes 名保留 Vue 原名 spin（scoped 不改名；kb-list.td.css 已有同名
-      同值定义，重复声明等价）。 */
+   /* keyframes 名保留 Vue 原名 spin（playbook §2.4）。 */
    animation: spin 1s linear infinite;
+ }
+ 
+ @keyframes spin {
+   from { transform: rotate(0deg); }
+   to { transform: rotate(360deg); }
  }


─── apps/web/src/shared/shared-u.css:172-172 ───
[maintainability · low] .wk-shared-main 用单类选择器覆盖 styles.css:43 `.wk-page--std { padding: 48px 20px;
}` 的简写 padding：两者同为 (0,1,0) 特异性，层叠结果完全取决于 CSS 注入顺序——当前仅因 SharedSessionPage 为 lazy
chunk（router.tsx:57）、shared-u.css 晚于主包 styles.css 注入才碰巧生效；若该组件未来改为同步导入或构建顺序调整，ready 分支纵向留白会静默回退为
48px（原为 py-10=40px）。建议提升选择器特异性使覆盖显式化、消除对加载顺序的隐式依赖。

- .wk-shared-main { padding-block: 40px; }
+ .wk-page--std.wk-shared-main { padding-block: 40px; }


─── apps/web/src/shared/shared-u.css:26-33 ───
[maintainability · low] .wk-shared-4/.wk-shared-8/.wk-shared-13/.wk-shared-15 四条规则均在同一声明块内重复书写
border-style: solid（border-width 与 border-color 各带一次）。CSS 重复声明本身渲染无害，但属迁移生成器冗余产物，会给后续"值 = 迁移时
utilities 编码的生效值"的人工核对增加噪音，建议生成器去重后重新产出（四处均删除第二条 border-style）。

  .wk-shared-4 {
    min-height: 32px;
    cursor: pointer;
    border-radius: 6px;
    border-style: solid;
    border-width: 1px;
-   border-style: solid;
    border-color: #dcdcdc;


─── apps/web/src/shared/shared-u.css:145-152 ───
[bug · low] break-all → overflow-wrap: anywhere 是不等价替换：原 SharedSessionPage 的 h1 与消息 p 使用 Tailwind
`break-all`（= word-break: break-all，任意字符间可断行，即使该英文单词整行能放下也允许在行尾拆词）；此处写成 overflow-wrap:
anywhere（仅当单词长于整行时才逐字符断行，且影响 min-content
计算）。对行尾放不下的普通长英文单词，原实现当场拆词续行，新实现整词换行——行文折行位置与既有视觉快照存在可观测差异，与文件头"值 = 迁移时 utilities
编码的生效值"的承诺不符。严格保真应为 word-break: break-all（.wk-shared-7 同样问题，可一并修正）；若有意改用 anywhere
的更温和断行，应在注释中显式记录该行为变更。

  .wk-shared-16 {
    margin: 0;
    white-space: pre-wrap;
-   overflow-wrap: anywhere;
+   word-break: break-all;
    font-size: 13px;
    line-height: 1.55;
    color: rgba(23,26,29,0.92);
  }


─── apps/web/src/shared/wk-legacy.css:31-31 ───
[bug · medium] wk-dialog-backdrop 的 --wk-overlay 兜底值与原取值不符：该变量在现存任何 CSS 中均已无定义（packages/ui
已删除），var() 的兜底将 100% 生效。而项目内记录的原值是墨蓝系 rgb(23 32 51/45%)——KnowledgeSettingsPage.css:9-12 注释明确记载 "The
shared Dialog backdrop defaults to --wk-overlay (rgb(23 32 51/45%))"；同文件 .wk-sheet-backdrop 也用
rgb(23 32 51 / 0.45)，settings.td.css:1846 的抽屉遮罩惯例亦然。当前兜底 rgba(0,0,0,0.5)（纯黑 50%）与原值不等，T15 后所有
WkDialog 消费方（kb-documents / knowledge-graph / invitation-inbox / wiki 等）遮罩将发生视觉回归，且同一兼容层内 dialog 与
sheet 遮罩颜色自相矛盾。建议兜底直接改为字面量 rgb(23 32 51 / 0.45)（或恢复变量定义使其可核验）。

- .wk-dialog-backdrop { position: fixed; inset: 0; display: grid; place-items: center; padding: 1rem; background: var(--wk-overlay, rgba(0, 0, 0, 0.5)); z-index: var(--wk-overlay-dialog-z, 3000); }
+ .wk-dialog-backdrop { position: fixed; inset: 0; display: grid; place-items: center; padding: 1rem; background: rgb(23 32 51 / 0.45); z-index: var(--wk-overlay-dialog-z, 3000); }


─── apps/web/src/shared/wk-legacy.css:47-48 ───
[bug · low] keyframes 使用全局命名空间下的
sheet-in-right/sheet-in-left，会产生跨文件副作用：settings/TenantAuditDrawer.tsx:161 的抽屉面板 inline `animation:
'sheet-in-right .2s ease-out'` 在此之前是注释明确记录的 no-op（其 138 行注释："现状即未定义 keyframes 的 no-op"）；而
TenantAuditDrawer 所在模块链（TenantMembersPanel → wk-legacy.tsx）会加载本 CSS，同名 keyframes 使该审计抽屉意外开始播放 200ms
滑入动画——超出本兼容层"渲染不变"的目标，且令那条注释过时。若非有意恢复动画，建议改用带命名空间的 keyframes 名（如 wk-sheet-in-right，WkSheet 的 inline
animation 同步更名）；若有意恢复，请同步更新 TenantAuditDrawer 的注释。

- @keyframes sheet-in-right { from { transform: translateX(24px); opacity: 0; } to { transform: translateX(0); opacity: 1; } }
- @keyframes sheet-in-left { from { transform: translateX(-24px); opacity: 0; } to { transform: translateX(0); opacity: 1; } }
+ @keyframes wk-sheet-in-right { from { transform: translateX(24px); opacity: 0; } to { transform: translateX(0); opacity: 1; } }
+ @keyframes wk-sheet-in-left { from { transform: translateX(-24px); opacity: 0; } to { transform: translateX(0); opacity: 1; } }
+ /* wk-legacy.tsx WkSheet 的 style animation 同步改为 'wk-sheet-in-right .2s ease-out' / 'wk-sheet-in-left .2s ease-out' */


─── apps/web/src/shared/wk-legacy.tsx:198-198 ───
[bug · high] WkSheet 的 aside 同时声明 aria-labelledby={titleId} 与 aria-label={String(title)}：title 类型为
ReactNode，消费方 KnowledgeDocumentDetailPage.tsx:318 传入的是含按钮组的 JSX（<span>…</span>），String() 求值结果为
"[object Object]"；且 aria-label 的可访问名称优先级高于 aria-labelledby，屏幕阅读器将把抽屉朗读为 "[object Object]
对话框"，aria-labelledby 完全失效。纯字符串 title 的调用点（如 KnowledgeDocumentsPage.tsx:5138）虽不致错，但双标签声明本身冗余。建议直接删除
aria-label，仅保留 aria-labelledby 指向 h2#wk-sheet-title。

-           aria-label={String(title)}
+           {/* 删除 aria-label={String(title)}，保留 aria-labelledby={titleId} 即可 */}


─── apps/web/src/shared/wk-legacy.tsx:153-156 ───
[bug · medium] 拖宽 cleanup 只移除 window 监听器，不重置 beginResize 写入的
document.body.style.cursor/userSelect：拖拽进行中按 Esc（document keydown 触发 onClose → open=false → 卸载执行
cleanup）后，全局光标残留 col-resize、整页文本选择被禁用，直到用户重开抽屉并完整拖一次才恢复。建议 cleanup 检测 resizeRef.current 非空时重置 body
样式。附带：panelWidth 列在依赖数组中导致拖拽期间每次宽度变化都拆除/重绑 window 监听，且 stop 闭包捕获的 panelWidth 依赖重挂才拿到新鲜值——改用 ref 读取
panelWidth（同 onCloseRef 模式）可一并消除这两个问题。

      return () => {
        window.removeEventListener('mousemove', move);
        window.removeEventListener('mouseup', stop);
+       if (resizeRef.current) {
+         resizeRef.current = null;
+         document.body.style.cursor = '';
+         document.body.style.userSelect = '';
+       }
      };


─── apps/web/src/shared/wk-legacy.tsx:134-134 ───
[bug · medium] localStorage 读写无异常保护：存储被禁用环境（旧版 Safari 隐私模式、无 allow-same-origin 的 iframe sandbox）下
getItem 抛 SecurityError，异常位于 useEffect 内会沿 effect 冒泡，无 ErrorBoundary 时整棵组件树被卸载（doc-detail/trace 等依赖
storageKey 的长驻抽屉受影响）；stop 中的 setItem 在配额耗尽时同样抛 QuotaExceededError。项目内同类"抽屉宽度持久化"均带 try/catch
惯例（ApiPlaygroundDrawer.tsx:215、career/RulePage.tsx:158），建议对齐。

-     const saved = Number.parseFloat(window.localStorage.getItem(storageKey) || '');
+     let saved = Number.NaN;
+     try {
+       saved = Number.parseFloat(window.localStorage.getItem(storageKey) || '');
+     } catch {
+       /* 存储被禁用环境（隐私模式/sandbox iframe）：宽度持久化降级为会话内默认值 */
+     }


─── apps/web/src/shared/wk-legacy.tsx:75-75 ───
[maintainability · low] WkDialog 与 WkSheet 各自维护了一份几乎相同的 focusable 元素查询 + Tab
循环焦点陷阱，且过滤细节不一致：WkDialog 在选择器 [tabindex]:not([tabindex="-1"]) 之外还做了 getAttribute('tabindex') !==
'-1' 的二次过滤（与选择器语义重复，属冗余），WkSheet 无此二次过滤。当前两者行为等价，但双副本 + 不对称写法在后续维护中容易漂移出真实差异。建议提取共享的
getFocusables(container) 工具函数供两处使用。

-       const focusable = Array.from(dialog.querySelectorAll<HTMLElement>('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])')).filter((element) => element.getAttribute('tabindex') !== '-1');
+ const FOCUSABLE_SELECTOR = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';
+ function getFocusables(container: HTMLElement): HTMLElement[] {
+   return Array.from(container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR));
+ }
+ // WkDialog / WkSheet 内统一改为：const focusable = getFocusables(dialog);


─── apps/web/src/styles.css:41-42 ───
[documentation · low] 注释文件归属有误：`.plat-shell__outlet .wk-page { height: 100%; overflow-y: auto;
max-width: none !important; }` 这条壳层覆盖规则实际定义在 platform-u.css（第 659 行，"壳层 outlet"
注释段），platform-shell.td.css 中并不存在任何 wk-page/outlet/max-width 规则（PlatformShell.tsx:1389 的注释也正确指向
platform-u.css）。按本注释去 platform-shell.td.css 排查壳内全宽级联会扑空，建议改为指向实际文件；同源笔误也出现在
NotFoundPage.tsx:14（"shell.td.css 把 .wk-page 覆盖"），可一并修正。

-    PlatformShell 壳内由 platform-shell.td.css 的 .plat-shell__outlet .wk-page
+    PlatformShell 壳内由 platform-u.css 的 .plat-shell__outlet .wk-page
     覆盖为全宽滚动（max-width:none!important，层叠与先前壳层 utility 等价）。 */


─── apps/web/src/tdesign-icon-offline.ts:8-9 ───
[maintainability · low] BLOCKED_SCRIPT_URL/BLOCKED_LINK_URL 与 tdesign-icons-react 0.6.11 内部硬编码的 CDN
地址强耦合。虽然 apps/web/package.json 已将版本精确锁定为 "0.6.11"（无 ^ 范围），日常不会静默漂移，但一旦有人显式升级依赖，库内 URL
变化后占位节点将无法命中其去重选择器，守卫静默失效并恢复对 tdesign.gtimg.com 的外网请求——这正是该守卫要防止的离线/隐私回归，且无任何告警。建议补充失效检测：开发模式下延迟检查
document 中是否出现未带 data-weknora-blocked-cdn 标记的真实 tdesign.gtimg.com 节点并 console.warn，或增加一个 grep
node_modules/tdesign-icons-react 校验内部 URL 未变的测试，让升级漂移尽早暴露。

  const BLOCKED_SCRIPT_URL = 'https://tdesign.gtimg.com/icon/0.4.5/fonts/index.js';
  const BLOCKED_LINK_URL = 'https://tdesign.gtimg.com/icon/0.4.5/fonts/index.css';
+ 
+ // dev 环境下检测守卫失效（依赖升级导致库内 URL 漂移时尽早暴露）
+ if (import.meta.env?.DEV) {
+   setTimeout(() => {
+     const leaked = document.querySelector('script[src^="https://tdesign.gtimg.com"]:not([data-weknora-blocked-cdn]), link[href^="https://tdesign.gtimg.com"]:not([data-weknora-blocked-cdn])');
+     if (leaked) console.warn('[tdesign-icon-offline] guard bypassed: real CDN node detected; verify tdesign-icons-react internal URLs');
+   }, 3000);
+ }


─── apps/web/src/tdesign-icon-offline.ts:15-22 ───
[maintainability · low] installed 标志先置 true、在无 body 分支再重置 false 的状态流转可读性差：状态在单次调用内经历了
false→true→false，且 body 未就绪时每次提前调用都会注册一个新的 once DOMContentLoaded 监听（虽有 querySelector
幂等兜底，最终只创建一组占位节点，但监听会累积）。下方两个 querySelector 的存在性判断本身已保证幂等，可直接以占位节点存在性作唯一幂等依据，去掉标志与重置逻辑。

-   installed = true;
- 
    const body = document.body;
    if (!body) {
      document.addEventListener('DOMContentLoaded', () => installTDesignIconOfflineGuard(), { once: true });
-     installed = false;
      return;
    }
+   // 幂等由下方 stubScript/stubLink 的 querySelector 存在性判断保证，无需 installed 标志


─── apps/web/src/test-tdom-harness.ts:51-51 ───
[maintainability · low] tdomWindow 导出在全仓无任何消费方——所有 import 该 harness
的测试文件均未引用它，属新文件中的推测性死导出。若有意保留作为逃生舱（让个别测试访问 harness 的 jsdom window
以覆盖全局），建议在文件头注释说明其用途；否则直接删除，避免误导后续使用者以为这是受支持的扩展点。



─── apps/web/src/wiki/WikiPage.tsx:1224-1224 ───
[bug · low] 搜索框同时保留 form onSubmit 与 TdInput onEnter 两条提交路径：按回车时 tdesign 的 onEnter 触发一次，原生表单隐式提交又触发一次
onSubmit，submitSearch 会被连续调用两次。LoginPage.tsx:98 的「submit 在途锁」注释与 login-page.test.tsx:323
的测试证明该项目已确认该双路径真实并发（其在非幂等场景需要加锁防护）。此处 submitSearch 是幂等的同步状态更新，重复执行无用户可见 bug，但冗余路径会误导后续维护者，建议删掉
onEnter（原生隐式提交已覆盖回车场景），或参考 LoginPage 明确加注释说明幂等性。



─── apps/web/src/wiki/WikiPage.tsx:1220-1220 ───
[maintainability · low] 同文件内 tdesign Input 的 onChange 签名风格不统一：此处显式标注 (value: unknown) 并做
String(value ?? "") 兜底，而标题/slug/摘要/内容等处（及 AdministrationPage、DataSourcesPage 等全仓先例）均用推断签名 (value) =>
String(value)。统一为后者可减少阅读负担，也与其余 6 处写法一致。

-                 onChange={(value: unknown) => setSearchDraft(String(value ?? ""))}
+                 onChange={(value) => setSearchDraft(String(value))}


─── apps/web/src/wiki/wiki-u.css:765-768 ───
[bug · low] 目录折叠 chevron 的旋转过渡动画丢失：.wk-wiki-79 用的是独立 rotate 属性（rotate: 90deg），而此处
transition-property 只列了 transform——rotate 属性不在过渡范围内，展开/收起时图标会瞬跳而非 150ms 平滑旋转。原 Tailwind v4 的
transition-transform 实际展开为 transition-property: transform, translate, scale, rotate，平移时漏掉了
rotate。建议补上：transition-property: transform, rotate;

-   align-items: center;
-   justify-content: center;
    color: rgba(0,0,0,0.4);
-   transition-property: transform;
+   /* Tailwind v4 transition-transform = transform, translate, scale, rotate —— rotate 属性需显式列入 */
+   transition-property: transform, rotate;


─── apps/web/src/wiki/wiki-u.css:190-192 ───
[maintainability · low] .wk-wiki-17 中 font-size: 13px 在三行后被 font-size: inherit 覆盖，13px 永不生效（原
Tailwind text-[13px] 与 [font:inherit] 的级联结果本就是 inherit 胜出）。死声明会误导后续维护者以为输入框是 13px，建议删除无效的 font-size:
13px。

-   font-size: 13px;
    font-family: inherit;
    font-size: inherit;


─── apps/web/src/wiki/wiki-u.css:641-642 ───
[maintainability · low] .wk-wiki-63 中 font-weight: inherit 随即被 font-weight: 650 !important
覆盖，前者为死声明（原 Tailwind [font:inherit] 与 [font-weight:650]! 的级联）。建议删除无效的 font-weight: inherit，避免误导维护者。

-   font-weight: inherit;
    font-weight: 650 !important;


─── docs/design/job-search/prototype/app.js:131-131 ───
[maintainability · low] 点击委托中 navItems.find(n => n[0] === state.view)[1] 对返回值无空值保护：当前所有 data-view
发射点恰好都落在 navItems 四个 id 内不会崩溃，但同文件 renderView 对未知 view 采取了宽容回退（if 链兜底到变体视图），此处却在 find 返回 undefined
时直接解引用抛 TypeError——且该异常发生在 root 级全局委托处理器内，一旦抛出会导致原型全部交互失效。原型本就预期快速迭代，新增视图 id 而漏更新 navItems
时极易踩中。建议加兜底：

-   if (view) { state.view = view.dataset.view; setNotice('已打开' + navItems.find(n => n[0] === state.view)[1]); return; }
+   if (view) { state.view = view.dataset.view; const entry = navItems.find(n => n[0] === state.view); setNotice('已打开' + (entry ? entry[1] : state.view)); return; }


─── internal/container/workbench.go:173-173 ───
[bug · high] 装配回归:删除了 MOBILE_ENTERPRISE_APP_ID 的读取、ValidateMobileAppID fail-closed 校验与
WithMobileAppPolicy 装配,但该策略未迁移到任何其他生产路径。handler.MobileAppPolicy 零值语义是"仅 official
可注册"(handler/mobile_device.go appAllowed 在注册/绑定/presence 共 8 处强制),而 container.go
newMobileNotificationProvider 仍按同一环境变量装配 enterprise 推送 lane(resolveFor(enterpriseApp))。结果是:合法声明的企业
App 无法通过设备注册端点注册(企业推送链路整体断裂),且注册层与推送层装配不一致。虽然安全方向仍 fail-closed(不会放宽到任意 AppID),但这是 enterprise
移动通道的功能回归,应恢复 policy 装配。

- 	return handler.NewMobileDeviceHandler(store, mobileEnvironment())
+ 	enterpriseApp := strings.TrimSpace(os.Getenv("MOBILE_ENTERPRISE_APP_ID"))
+ 	if enterpriseApp != "" && repository.ValidateMobileAppID(enterpriseApp) != nil {
+ 		enterpriseApp = "" // 非法声明 fail closed：仅 official 通道
+ 	}
+ 	return handler.NewMobileDeviceHandler(store, mobileEnvironment()).
+ 		WithMobileAppPolicy(handler.MobileAppPolicy{EnterpriseAppID: enterpriseApp})


─── internal/container/workbench.go:31-31 ───
[bug · high] 功能回归:移除了 WithGrantedRuns(runs) 后,生产装配(container.go:272
Provide(NewWorkbenchReadHandler))的读处理器 h.granted 恒为 nil。workbench_read.go resolveReadableRun(用于
GetWorkbenchExecution/GetWorkbenchSnapshot)在 owner-miss 且 h.granted == nil 时直接 404——T12 的
Viewer/Collaborator grant 持有者读取受托任务将从成功退化为 404。同时 provideWorkbenchTaskHandlers 仍注册
NewWorkbenchTaskGrantsHandler(grant 授予路径存活),形成"可授权、不可读"的不一致。测试(task_collaboration_http_test.go
等)直接构造时仍链 WithGrantedRuns,无法暴露此装配断链。若移除是有意的授权收紧,应同步移除 grant 授予端点并在 workbench_read.go 删除回退逻辑;否则应恢复接线。

- 	return session.NewWorkbenchReadHandler(runs, snapshots, ingestor).WithTaskFacts(lists)
+ 	return session.NewWorkbenchReadHandler(runs, snapshots, ingestor).WithTaskFacts(lists).WithGrantedRuns(runs)


─── internal/handler/session/artifact_download.go:702-706 ───
[maintainability · low] streamArtifactVersion 的 GetFile 失败分支静默 404，而两条同族路径均有日志：认证版
DownloadArtifactVersion 记录 "artifact version download read failed"，旧版 streamResolvedArtifact 记录
"artifact download read failed"。授权下载路径上，存储故障（blob 缺失/后端解析失败）与链接被撤销/过期在响应上完全同形（均
404）且无任何可观测输出，运维排障时无法区分内部故障与正常撤销。建议至少在 GetFile 失败分支补 Warnf，与认证路径对齐。

  	reader, err := fileService.GetFile(ctx, version.ObjectKey)
  	if err != nil {
+ 		logger.Warnf(ctx, "artifact version grant read failed: run=%s version=%s err=%v", version.RunID, version.ID, err)
  		c.AbortWithStatus(http.StatusNotFound)
  		return
  	}


─── internal/handler/session/artifact_download.go:644-652 ───
[bug · medium] 新版本 grant 消费端点把所有失败统一折叠为裸 404，与同文件既有
DownloadWorkbenchArtifactGrant（L312-330）刻意设计的状态码语义不一致：既有路径对签名密钥未配置返回 501 +
`artifact_signing_disabled`，对过期/无效签名分别返回 401 + `artifact_grant_expired` /
`artifact_grant_invalid`（其注释明确说明区分过期是为了让客户端提供重新授权 UX）。两个端点共享同一路由
`/api/v1/workbench/artifacts/download` 与同一客户端传输（miniprogram 的 openProtectedDocument 按 401+code 映射
ARTIFACT_GRANT_EXPIRED → "再次点击重新获取" 文案；mobile-core 的 mapError 同样按 401+code 区分
GRANT_EXPIRED/GRANT_INVALID）。当前实现下：(1) 一旦客户端切换到版本寻址 grant（W26 是 index
寻址的既定后继），过期只能呈现为通用"下载失败"，重新授权交互无法触发；(2) 运维无法从下载侧状态码发现签名密钥未配置的部署错误（签发侧 501、下载侧
404）。VerifyArtifactVersionGrantAt 的错误信息已区分 expired 与 mismatch，可低成本镜像既有语义。

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


─── internal/handler/session/artifact_download.go:679-683 ───
[maintainability · medium] streamArtifactVersion 与认证版 DownloadArtifactVersion（L585-629）逐行重复了约 30
行"租户查找 → ParseStorageBackendPath → ResolveTenantFileServiceWithFallback → GetFile →
filetransport.Serve"骨架，且重复副本已经实际漂移：认证路径的 GetFile 失败记录 "artifact version download read failed"
Warnf，本副本静默 404（即已确认发现 #2）。两路径唯一实质差异是错误响应形态（c.Error 结构化 errors.NewNotFoundError vs 裸
AbortWithStatus）。建议提取共享的 resolve+stream helper（返回
error，由各调用方决定响应形态），认证版直接复用；这同时消除已确认的日志漂移根因，并避免后续任何一侧单独调整（如新增审计日志、Content-Disposition 策略）时的再度分叉。

- func (h *ArtifactVersionDownloadHandler) streamArtifactVersion(c *gin.Context, ctx context.Context, tenantID uint64, version repository.ArtifactVersion) {
- 	if h.files == nil {
- 		c.AbortWithStatus(http.StatusNotFound)
- 		return
+ func (h *ArtifactVersionDownloadHandler) resolveVersionStream(ctx context.Context, tenantID uint64, version repository.ArtifactVersion) (io.ReadCloser, interfaces.FileService, error) {
+ 	// tenant lookup + backend resolve + GetFile，GetFile 失败统一 Warnf；
+ 	// 由 DownloadArtifactVersion / DownloadArtifactVersionGrant 各自映射错误响应形态。
- 	}
+ }


─── internal/handler/session/workbench_artifacts.go:277-285 ───
[maintainability · low] ttl_seconds 的解析/封顶逻辑与既有 CreateWorkbenchArtifactSignedURL（L199-207）逐字重复。两代链接的
TTL 策略目前一致，但任何后续调整（如改为下限保护、不同封顶、负值拒绝）只改一处就会造成两代链接策略静默漂移。建议提取共享 helper，两个端点共用。

- 	ttl := h.ttl
- 	if raw := strings.TrimSpace(c.Query("ttl_seconds")); raw != "" {
- 		if requested, parseErr := strconv.Atoi(raw); parseErr == nil && requested > 0 {
+ // 包级 helper，两个 signed-url 端点共用：
+ func resolveGrantTTL(defaultTTL time.Duration, raw string) time.Duration {
+ 	ttl := defaultTTL
+ 	if raw = strings.TrimSpace(raw); raw != "" {
+ 		if requested, err := strconv.Atoi(raw); err == nil && requested > 0 {
  			ttl = time.Duration(requested) * time.Second
  		}
  	}
  	if ttl > workbench.MaxArtifactGrantTTL || ttl <= 0 {
  		ttl = workbench.MaxArtifactGrantTTL
+ 	}
+ 	return ttl
- 	}
+ }
+ // 调用处：ttl := resolveGrantTTL(h.ttl, c.Query("ttl_seconds"))


─── internal/modules/career/career_export.go:860-865 ───
[bug · high] finalize 防御性清扫只重跑 purgeCareerRows（删行），不重跑物理对象释放与 Workbench
投影移除，竞态窗口内会产生永久孤儿，违反「删除完整性（无孤儿文件/行/投影）」门禁：

1）孤儿投影：CreateApplication 的 Career 行在事务内提交（link_state=linking）后，linker 外呼发生在提交之后。若其提交落在 purge 步骤之后（它持
profile 行锁，finalize 会等它提交后再清扫），清扫删除该 application 行，但随后的
EnsureCareerApplicationTask（application_task.go:143-220，仅读 workbench 表，不校验 Career 行存续）仍会创建
session/run/mapping —— 而 DeletionStepRemoveProjections 已在 finalize 之前执行完毕，且 status=deleted
的重放路径直接返回回执、永不重跑步骤，投影永久残留。同时 CreateApplication 侧 updateApplicationLink 返回 ErrApplicationNotFound →
504 outcome_unknown，FindApplicationReceipt 永远 404，客户端 unknown 恢复轮询无法收敛。

2）孤儿文件：同一窗口内新上传原件（PersistUploadResource 已写入物理对象）与新导出 PDF/DOCX 对象同样只被删行（career_source_revisions /
career_material_exports），deletionPurgeCareerData 的 Release 与 deletionRevokeExports 的对象删除均不会对该行重跑。

建议：finalize 事务提交后、持久化 deleted 回执前，在同一请求内幂等重跑投影移除（RemoveCareerApplicationTaskProjections
自身幂等）与对象释放；并/或让 CreateApplication 外呼前在 profile 锁下复核 application 行存续，行已消失时放弃创建投影并返回可终止的错误而非
outcome_unknown。



─── internal/modules/career/career_export.go:1079-1081 ───
[bug · low] exportStorage 为 nil 时静默跳过物理对象删除却仍把导出行改为 revoked 并继续（deletionPurgeCareerData 中
sourceUploadReleaser 为 nil 同样静默跳过 Release），与 deletionRemoveProjections 对 nil remover
直接报错形成不对称：一旦装配遗漏（NewHandler 中 SetExportStorage 受 files != nil 条件保护），删除回执仍宣称 complete，而 PDF/DOCX
对象与上传原件永久残留为孤儿文件。建议与 remover 保持一致：端口缺失时让该步骤失败（return error），保证删除回执不会在未做物理清理的情况下宣称完成。



─── internal/modules/career/career_export.go:625-627 ───
[bug · low] sectionCount 将 Count 查询错误吞为 0：DB 异常时删除确认界面会展示「将删除 0
条」，用户在数据实际存在的情况下确认删除，与删除回执的诚实性目标相悖。建议至少记录错误（slog）或在视图层标注计数不可用，而非静默归零。



─── internal/modules/career/career_export.go:307-310 ───
[performance · low] 导出在持有 profile FOR UPDATE 锁的事务内通过 buildCareerExportArchive 做 N+1 全量读取（每个
opportunity/application/material 各一次子查询），并将完整 archive 同步写入 text 列且内联返回、无大小上限：大空间下会长时间阻塞该用户所有 Career
写路径（所有写事务都要先取 profile 行锁），并产生超大响应与存储行。个人空间规模下影响有限，但建议考虑：快照读取移出锁外（或改共享锁）、以及导出体积上限防护。



─── internal/modules/career/career_export.go:632-632 ───
[bug · low] profile 节描述含「待处理提案」但 Count 只统计 career_facts，未计入 career_proposals（该表在 careerPurgeTables
中会被删除）：删除确认界面对待删数据量系统性低估。建议补上 sectionCount("career_proposals", "") 或单独拆分一节。



─── internal/modules/career/career_export.go:414-417 ───
[bug · medium] 导出归档只携带 status='pending' 的提案，但 careerPurgeTables 会清空整个 career_proposals 表（含 confirmed
与 dismissed 行）。dismissed 提案的 key/value/evidence（例如简历提取出的被用户否决的字段）以及 confirmed
提案的原始证据（Evidence、ResolvedAt、ResolutionSource）既不会进入 Profile.Facts/FactHistory（confirm 只落 fact 的
value），也不在本导出中——删除后这些数据被不可恢复地销毁。这与紧邻的 CareerExportArchive 注释自相矛盾："Deletion purges every one of these
sections, so the export must carry all of them or the data would be destroyed
unrecoverably"，也违背删除完整性的数据主权目标。建议去掉 status='pending' 过滤，导出全部提案及其状态/确认/解决字段（Proposal 视图已含
Status/Confirmation/ResolutionSource 字段，可直接承载）。

  	var proposals []proposal
- 	if err := tx.Where("tenant_id=? AND user_id=? AND status='pending'", s.TenantID, s.UserID).Order("created_at,id").Find(&proposals).Error; err != nil {
+ 	if err := tx.Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Order("created_at,id").Find(&proposals).Error; err != nil {
  		return archive, err
  	}


─── internal/modules/career/evaluation.go:218-219 ───
[bug · medium] persistenceMayHaveCommitted 在 tx.Create 之前置位，导致 Create
的任何非类型化确定性失败（约束冲突、列超长、磁盘满等，回滚已确定、结果不可能已提交）也会走 reconcile 未命中 → OutcomeUnknownError 分支，被 handler 映射为
504 outcome_unknown，客户端进入永不收敛的恢复轮询，根因（500/4xx）被掩盖。同族
seam（EditMaterial/CreateApplication/RecordSubmission）均以 isReceiptRaceError(err) 门控 replay
分支、其余原样返回错误，此处不一致。建议改为仅在 isReceiptRaceError(err) 或 ctx.Err() != nil 时进入
reconcile/OutcomeUnknown，其余直接返回原始错误。



─── internal/modules/career/handler.go:45-47 ───
[maintainability · low] ExportSigningKeyFromEnv 的错误被完全静默吞掉。该函数区分两种失败：未配置（返回
ErrExportSigningKeyMissing，属合法的可选配置）与已配置但非法（非 hex 或不足 32
字节，返回格式错误）。当前写法下，管理员配置了非法密钥时启动期无任何日志，运行期导出签名/下载静默退化为 501，且错误消息 "export_signing_key_missing"
会误导排障方向（提示未配置，实际是配错）。建议对非 ErrExportSigningKeyMissing 的失败至少记录 warn 日志（或直接令 NewHandler
返回错误使误配置在装配期暴露）。

  	if key, keyErr := ExportSigningKeyFromEnv(); keyErr == nil {
  		o.SetExportSigningKey(key)
+ 	} else if !errors.Is(keyErr, ErrExportSigningKeyMissing) {
+ 		// 已配置但非法的签名密钥应在装配期暴露，而非运行期静默退化为 501。
+ 		slog.Warn("career export signing key misconfigured; export signing disabled", "error", keyErr)
  	}


─── internal/modules/career/handler.go:1584-1588 ───
[bug · low] Upload 端点对超限请求返回 400 invalid_request，与本文件其他端点的错误契约不一致：body 已被 http.MaxBytesReader 包裹（上限
maxSize+1MB），超限时 c.Request.FormFile 会返回 *http.MaxBytesError，而 Act/ImportJD/ImportURL 等所有端点对同一错误类型均返回
413 request_too_large。客户端在此处无法区分"文件体积超限"与"请求非法"，影响上传重试/提示逻辑。建议对 FormFile 错误先做
errors.As(*http.MaxBytesError) 判断后返回 413。

  	file, header, err := c.Request.FormFile("file")
  	if err != nil {
+ 		var maxBytesError *http.MaxBytesError
+ 		if errors.As(err, &maxBytesError) {
+ 			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"code": "request_too_large", "message": "uploaded file exceeds size limit"}})
+ 			return
+ 		}
  		writeError(c, ErrInvalidRequest)
  		return
  	}


─── internal/modules/career/material.go:948-951 ───
[bug · low] diffMaterialBodies 以 heading 作为 section 的唯一对齐键（map 覆盖同名 heading），但 validateMaterialShape
并不禁止重复 heading——同一 body 中两个同名 section 是合法输入。此时两个 target 同名 section 都会与 baseline 中最后一个同名 section
比较，产生重复且失真的 section_changed/claim_* 变更（例如 baseline 的第一个同名 section 的删除会被漏报、两个 target section
各自报一次变更）。作为"honest diff"契约的 CompareMaterialVersions 语义被破坏。建议在 validateMaterialShape 中拒绝重复
heading（新写入路径收紧），或在 diff 中使用 (heading, 出现序号) 复合键对齐以兼容历史数据。

- 	baselineSections := map[string]MaterialSection{}
- 	for _, section := range baseline.Sections {
- 		baselineSections[section.Heading] = section
+ 	seenHeadings := map[string]bool{}
+ 	for _, section := range body.Sections {
+ 		if seenHeadings[section.Heading] {
+ 			return ErrInvalidRequest
+ 		}
+ 		seenHeadings[section.Heading] = true
+ 		// ...
  	}


─── internal/modules/career/office.go:667-670 ───
[bug · medium] dismiss 将该 SELECT 的所有错误一律折叠为 ErrProposalResolved（handler 映射为 409
proposal_resolved）。瞬时 DB 故障（连接中断、PG 序列化失败、语句超时等非 NotFound
错误）会被上报为确定性的业务冲突，客户端会误以为提案已被处理而放弃重试；同一函数族里的 confirm 分支正确区分了 gorm.ErrRecordNotFound 与其他错误。建议仅将
NotFound（以及确实非 pending 的行）映射为 ErrProposalResolved，其余错误原样上抛（mutate 外层的 operationCtx/OutcomeUnknown
逻辑会正确处理取消类错误）。

  		var p proposal
- 		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=? AND public_id=? AND status='pending'", s.TenantID, s.UserID, pid).First(&p).Error; e != nil {
+ 		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=? AND public_id=?", s.TenantID, s.UserID, pid).First(&p).Error
+ 		if errors.Is(e, gorm.ErrRecordNotFound) || (e == nil && p.Status != "pending") {
  			return Receipt{}, ErrProposalResolved
+ 		}
+ 		if e != nil {
+ 			return Receipt{}, e
  		}


─── internal/modules/career/office.go:606-608 ───
[bug · low] confirm 对 key 的校验只做了 TrimSpace 后的非空判断，未像 propose 那样先 TrimSpace 再存储：带首尾空格的 key（如 "
education.graduation_year"）可以通过校验并以未规范化的形式写入 career_facts。后果：与 propose 产生的同名事实形成重复行（"x" 与 " x"），且此类
key 不在 safeModelKeys/extractedModelKeyPattern 中，会被 BuildModelInput 静默排除。建议与 propose 一致，先规范化再校验。

- 	if r == "" || confirmationSrc.Kind == "" || (pid == "" && (strings.TrimSpace(k) == "" || len(k) > 128 || src.Kind == "")) {
+ 	k = strings.TrimSpace(k)
+ 	if r == "" || confirmationSrc.Kind == "" || (pid == "" && (k == "" || len(k) > 128 || src.Kind == "")) {
  		return out, ErrInvalidRequest
  	}


─── internal/modules/career/preparation.go:280-281 ───
[bug · medium] GeneratePreparation 的 fingerprint 混入了服务端从数据库解析的 snapshotRef（以及
anchor），而非仅客户端原始输入。reconciliation merge 会把 snapshot 的 opportunity_id re-parent 到 merge
target（reconciliation.go 中 `Update("opportunity_id", input.TargetID)`，同时迁移 application
行），此后客户端用完全相同的 requestID 重试时，重新解析出的 snapshotRef.OpportunityID 已变化，fingerprint
不再匹配，attemptPreparationReserve 会返回
ErrIdempotencyConflict——已成功的请求无法重放存储回执，失败/超时（OutcomeUnknown）后的恢复流程也被 409 卡死，只能弃用原 requestID。这与同族
seam 的既有设计模式相悖：EvaluateOpportunity 明确注释了 fingerprint 必须基于原始输入（"computed from the original input and
stays stable for replays"），CreateApplication/EditMaterial/RecordSubmission 也都只用客户端输入。建议 fingerprint
仅覆盖 input（requestID/applicationID/focus/expectedRevision），anchor/snapshotRef 作为服务端校验与 receipt
字段即可（anchor 来自不可变的 submission 行，本身稳定；不稳定的只有 snapshotRef.OpportunityID）。

  	fingerprint, err := materialFingerprint("generate_preparation", input.RequestID, input.ApplicationID,
- 		input.Focus, anchor, snapshotRef, input.ExpectedRevision)
+ 		input.Focus, input.ExpectedRevision)


─── internal/modules/career/profile_intake.go:583-583 ───
[bug · low] completeIntake 的更新 SQL 无条件携带 `AND claim_token=?`：当 token==""（即无 claim 的 CompleteIntake
入口）时，该条件退化为要求行的 claim_token 必须是空字符串。这与两处既有语义矛盾：(1) 同函数在锁内读取后做的 Go 侧检查是 `if token != "" &&
source.ClaimToken != token`——token 为空时明确放行任意行；(2) finishSource 对同一场景做了条件化拼接（`if token != "" { q =
q.Where("claim_token=?", token) }`）。结果是 ClaimUpload 创建的行（ClaimToken 恒为 uuid 非空）被 CompleteIntake
完成时，前置检查通过却在 SQL 更新处 RowsAffected=0，返回误导性的 ErrUploadClaimLost。当前生产 handler 只走 CompleteIntakeClaim
所以未触发，但这是一个会困住未来调用方的潜伏不一致。建议与 finishSource 对齐条件化拼接，或让 Go 侧检查与 SQL 条件一致（token 为空时要求 ClaimToken 也为空）。

- 		res := tx.Model(&sourceRevision{}).Where("tenant_id=? AND user_id=? AND id=? AND status='processing' AND claim_token=?", s.TenantID, s.UserID, sourceID, token).Updates(map[string]any{"status": source.Status, "lease_until": source.LeaseUntil, "extracted_text": source.ExtractedText, "missing_categories": source.MissingCategories, "review_flags": source.ReviewFlags, "completed_at": source.CompletedAt})
+ 		q := tx.Model(&sourceRevision{}).Where("tenant_id=? AND user_id=? AND id=? AND status='processing'", s.TenantID, s.UserID, sourceID)
+ 		if token != "" {
+ 			q = q.Where("claim_token=?", token)
+ 		}
+ 		res := q.Updates(map[string]any{"status": source.Status, "lease_until": source.LeaseUntil, "extracted_text": source.ExtractedText, "missing_categories": source.MissingCategories, "review_flags": source.ReviewFlags, "completed_at": source.CompletedAt})


─── internal/modules/career/reminder.go:538-542 ───
[bug · low] reminderReceiptFromRow 未填 Revision，导致所有去重路径（attemptReminderWrite 的同源 todo
命中、reconcileReminderSource 的并发和解）生成的持久化回执 revision 恒为 0；而新建路径写入的是 head.Revision。同一个提醒事件因触发时机不同产生
revision 不一致的回执，客户端若以回执 revision 链式做后续 CAS 会拿到错误的 0。建议为该函数补传当前 profile head revision（事务内已读取
head，可透传）。

  		Notice:        ReminderNoticeBodies[row.NoticeKey],
  		Status:        row.Status,
+ 		Revision:      revision,
  		CreatedAt:     row.CreatedAt,
  	}
  }


─── internal/modules/career/reminder.go:231-238 ───
[bug · low] 当 attemptReminderWrite 在事务内命中请求 ID 回放（早前 replayReminderReceipt
检查与事务读取之间的竞态窗口）时，outcome.subscribed 保持零值 false，随后 remindAfterCommit 会为已订阅用户挂上 Push{Attempted:false,
Reason:"unsubscribed"} 的错误上报——回放并非新建 todo，本不应有任何推送尝试或推送报告；且与事务前回放路径（完全不带 Push 字段）返回结构不一致。建议让
attemptReminderWrite 显式区分「新建」与「回放」两种结果（例如在 outcome 中加 created 标志），仅新建时执行 remindAfterCommit。

  		outcome, txErr := o.attemptReminderWrite(operationCtx, s, input, fingerprint)
  		if txErr == nil {
- 			// The inbox fact is committed; the push is a post-commit,
- 			// best-effort reminder that can neither fail the write nor
- 			// mutate the todo.
+ 			// Push is a post-commit, best-effort reminder for a NEWLY created
+ 		// todo only; a replayed receipt is returned verbatim.
+ 			if outcome.created {
- 			o.remindAfterCommit(ctx, s, &outcome.receipt, outcome.subscribed)
+ 				o.remindAfterCommit(ctx, s, &outcome.receipt, outcome.subscribed)
+ 			}
  			return outcome.receipt, nil
  		}


─── internal/modules/career/rendering.go:1248-1257 ───
[bug · medium] PublishMaterial 的错误收尾缺少 SQLite busy→OutcomeUnknownError 映射：本事务以 receipt 的 SELECT FOR
UPDATE 开头（读先行），首个写语句存在 shared→reserved 锁升级撞 BUSY 的窗口，且这里既无 busy 重试、也未折叠错误。busy 错误字符串命中
isReceiptRaceError("database is locked")，在 ctx 仍健康时重放查询未命中后落入 `return ExportReceipt{}, err`
原样上抛，handler 将其映射为 500 internal（"unmapped career office
error"），而不是本模块其余写路径（mutate、SetReminder、ReconcileOpportunities、ImportURL/usage 的
runImportTransaction）统一提供的 504 outcome_unknown + requestId。这会破坏客户端按 requestId 的未知写恢复契约（500 促使客户端放弃或换
ID 重试，而该事务此时确已整体回滚、同 ID 重试是安全且必要的）。建议在重放未命中后、ctx.Err() 检查前补上与 ReconcileOpportunities 相同的
isSQLiteBusy 分支。

  		if isReceiptRaceError(err) {
  			if replay, found, lookupErr := o.replayExportReceipt(ctx, s, input.RequestID, fingerprint, MaterialKindPublished); lookupErr != nil {
  				return ExportReceipt{}, lookupErr
  			} else if found {
  				return replay, nil
+ 			}
- 			}
+ 		}
+ 		if isSQLiteBusy(err) {
+ 			// 事务已整体回滚：同 requestId 重试安全，必须以 outcome_unknown 契约返回。
+ 			return ExportReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
  		}
  		if ctx.Err() != nil {
  			return ExportReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
  		}


─── internal/modules/career/rendering.go:1511-1520 ───
[bug · medium] RevokeMaterialExport 的错误收尾与 PublishMaterial 有同样的缺陷：读先行事务在 SQLite 锁升级撞 BUSY
且重放未命中时，busy 原始错误被原样上抛（handler → 500 internal），未映射为 OutcomeUnknownError（504 +
requestId）。同模块其余写路径（ReconcileOpportunities、SetReminder、ImportURL、admitSearchUsage）均显式做了 isSQLiteBusy
→ outcome_unknown 的折叠；此处撤销操作同样整体回滚、同 ID 重试安全且必要，应保持契约一致。

  		if isReceiptRaceError(err) {
  			if replay, found, lookupErr := o.replayExportReceipt(ctx, s, input.RequestID, fingerprint, MaterialKindExportRevoked); lookupErr != nil {
  				return ExportReceipt{}, lookupErr
  			} else if found {
  				return replay, nil
+ 			}
- 			}
+ 		}
+ 		if isSQLiteBusy(err) {
+ 			return ExportReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
  		}
  		if ctx.Err() != nil {
  			return ExportReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
  		}


─── internal/modules/career/search_once.go:425-429 ───
[maintainability · low] decodeSearchBody 用 errSearchReceipt 哨兵完全替换了底层 JSON 解码错误，两处 Unmarshal
失败均丢失原始因果。当某条 career_searches.receipt_body 损坏（例如部分写入或版本不兼容）时，运维只能看到 "unreadable career search
receipt"，无法定位是编码问题还是数据截断，而该错误还会沿 claimSearchRequest/commitSearch/awaitSearchTerminal 一路以相同面目出现。建议用
%w 包一层保留哨兵可比较性的同时保留底层错误。

  func decodeSearchBody(body string) (*SearchOnceReceipt, searchClaimBody, error) {
  	var claim searchClaimBody
  	if err := json.Unmarshal([]byte(body), &claim); err != nil {
- 		return nil, searchClaimBody{}, errSearchReceipt
+ 		return nil, searchClaimBody{}, fmt.Errorf("%w: %v", errSearchReceipt, err)
  	}


─── internal/modules/career/search_once.go:298-300 ───
[bug · low] searchOnceUnderStoredFingerprint 在请求行不存在时返回 ErrIdempotencyConflict，把「request ID
未知/已消失」与「同 request ID 不同内容」两种语义混为一谈。唯一调用方 triggerRulePeriod 在 SearchOnce 返回
ErrIdempotencyConflict（意味着行曾存在）后调用本函数；若该行在两次读取之间被并发清除（delete_career 清除会删除 career_searches
行），这里会把"行已不存在"当作幂等冲突硬错误向上抛，进而触发 TriggerDueRules 的整体中止。建议区分语义：行不存在时返回
ErrSearchNotFound（或让调用方将其视为本周期无事可做），而不是复用冲突哨兵。

  	if errors.Is(err, gorm.ErrRecordNotFound) {
- 		return SearchOnceReceipt{}, ErrIdempotencyConflict
+ 		// The claiming row vanished (e.g. purged mid-flight); this is a
+ 		// not-found, not a same-request-different-content conflict.
+ 		return SearchOnceReceipt{}, ErrSearchNotFound
  	}


─── internal/modules/career/search_rule.go:514-522 ───
[bug · high] TriggerDueRules 在任一规则触发失败时 `return nil, triggerErr`：既中止剩余到期规则的处理，又丢弃本轮已成功规则的
outcomes。可达的错误源已验证：(1) admitSearchUsage 将任何账本失败统一映射为
ErrAdmissionUnavailable（usage.go:265-271），triggerRulePeriod 对非 ErrSearchQuotaRefused 的准入错误直接向上返回；(2)
triggerRulePeriod 在事务外读取 head.Revision 后交给 SearchOnce 复核，并发 profile 写会触发
RevisionConflictError（瞬时冲突，未在 switch 中处理）；(3) 规则在 due 查询与触发事务之间被删除返回 ErrRuleNotFound；(4) 回执体损坏返回
errSearchReceipt。更严重的是队头阻塞：失败规则的 next_due_at 只在成功事务内推进，而 due 查询按 `next_due_at ASC`
排序，持续失败的规则每次都会排在最前并先中止整个循环，导致同 scope 下所有其他规则的周期被无限期饿死——这正是本次审查重点「search rule 周期
claim/pause-before-trigger 线性化」的可靠性缺陷。建议：单规则失败时记录并继续处理其余规则（返回部分 outcomes + 汇总错误），或在触发失败时也推进/重排
next_due_at 以避免同一失败规则永久占据队头。

  	outcomes := []RuleRunSummary{}
+ 	var firstErr error
  	for _, rule := range due {
  		outcome, triggerErr := o.triggerRulePeriod(ctx, s, rule, now)
  		if triggerErr != nil {
- 			return nil, triggerErr
+ 			// One failing rule must not starve the remaining due rules:
+ 			// keep processing and surface the first failure alongside the
+ 			// partial outcomes.
+ 			if firstErr == nil {
+ 				firstErr = triggerErr
+ 			}
+ 			continue
  		}
  		outcomes = append(outcomes, outcome)
  	}
- 	return outcomes, nil
+ 	return outcomes, firstErr


─── internal/modules/career/search_rule.go:456-461 ───
[performance · medium] Rule() 无分页、无上限地加载规则的全部 run 记录（每条还要 json.Unmarshal 一次 body）和全部 discovery
todos，并由 GetRule handler（handler.go:800-811）直接作为响应返回。minRuleIntervalMinutes=1 允许单规则每天产生 1440 条
run（ruleEstimateBasis 自己也声明 triggers_per_day = 1440/interval_minutes），而 run/todo 行没有任何周期清理（仅在
delete_career 清除时删除）。长期运行的规则会让该读路径的内存占用、JSON 解码耗时与响应体积无界增长，最终拖垮 GET /career/rules/:ruleId。建议对
Runs/Todos 增加有界窗口（如最近 N 条 + 截断标记）或分页参数。

  	var runRows []searchRuleRunRecord
  	if err = o.db.WithContext(ctx).
  		Where("tenant_id=? AND user_id=? AND rule_id=?", s.TenantID, s.UserID, ruleID).
- 		Order("period ASC").Find(&runRows).Error; err != nil {
+ 		Order("period DESC").Limit(maxRuleRunsInView).Find(&runRows).Error; err != nil {
  		return RuleView{}, err
+ 	}
+ 	// serve the bounded window in period ASC order
+ 	for i, j := 0, len(runRows)-1; i < j; i, j = i+1, j-1 {
+ 		runRows[i], runRows[j] = runRows[j], runRows[i]
  	}


─── internal/modules/career/search_rule.go:551-554 ───
[bug · medium] triggerRulePeriod 在执行 SearchOnce（网络 I/O + quota 预占）之前不再复核规则状态。规则状态唯一的检查点是
TriggerDueRules 的到期扫描（status=enabled AND next_due_at<=now），此后到 SearchOnce 启动之间要经历 quota 预占事务、profile
读取、claim 事务；且多条到期规则顺序处理时，靠后规则要等前面规则完成整个网络搜索，窗口可达数秒。此窗口内用户通过 SetRule 提交的 pause/disable 会正常提交（trigger
侧不持有规则行锁），但该规则仍会：为 requestID rule:<id>:<period> 预占并结算 1 单位搜索配额、实际抓取 vetted 源、并在终态事务中记录 run 与
discovery todos、推进 last_period——终态事务只在计算 next_due_at 时读取新状态，无法撤回已发生的副作用。这违反本文件头部冻结的契约"a disabled or
paused rule never triggers, never enqueues, and never reaches a
source"（TestDisabledRuleNeverTriggersOrEnqueues 也断言 disabled 规则 never consults quota admission /
never reaches a source，但只覆盖了扫描前 pause 的情形，未覆盖扫描后、搜索前的窗口）。建议在 quota 预占之前按 rule.ID 重读规则行，status !=
enabled 时直接落一条 blocked 风格的 run（不搜索、不占配额），把 pause/disable 的线性化点收紧到副作用发生前。

+ 	if blockedStatus == "" {
+ 		// Pause/disable linearization: the due scan is the only status check
+ 		// so far. Re-validate before quota admission or any network I/O so a
+ 		// rule paused or disabled in between never reaches a source.
+ 		var live searchRuleRecord
+ 		if e := o.db.WithContext(ctx).
+ 			Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, rule.ID).
+ 			First(&live).Error; e == nil && live.Status != RuleStatusEnabled {
+ 			blockedStatus, blockedNote = RuleRunStatusPausedBeforeTrigger, ruleRunNotePausedBeforeTrigger
+ 		}
+ 	}
  	if blockedStatus == "" {
  		var head profile
  		err := o.db.WithContext(ctx).
  			Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error


─── internal/modules/codedelivery/dispatcher.go:242-245 ───
[maintainability · low] DispatchOutcome.Status 的 "succeeded" 是本文件四处（226/243/287/297 行）出现的跨模块契约值：该值经
codeDeliveryActionBridge 原样透传后由 appconnectorsvc.settleOutcome 与 ActionSucceeded 常量比较决定终态（不匹配则落入
default→unknown），ResolveUnknown 同样只接受 ActionSucceeded/ActionFailed 两个值。当前取值正确且被 gitlab/dispatch
集成测试间接钉死，但与 service.go 的字面量同属一个无编译期联动的词表，建议与 service.go 一并在 contracts.go 定义命名常量并用一致性测试钉死（见
service.go 评论）。

  	return DispatchOutcome{
- 		Status:         "succeeded",
+ 		Status:         OutcomeSucceeded, // contracts.go: OutcomeSucceeded = "succeeded"
  		ProviderResult: deliveredReceipt(receipt.Number, receipt.URL, login),
  	}, nil


─── internal/modules/codedelivery/service.go:460-462 ───
[maintainability · low] 依赖倒置后，codedelivery 与 appconnector 之间共享的持久化状态词表（连接 kind/state、action
state、risk）从编译期常量联动退化为散落的裸字符串（本处 "personal"/"active"，及 PrepareDelivery 的
"deliver"、"awaiting_approval"、"failed"）。当前六个字面量与 appconnector 常量逐字一致，且 service_prepare_test
等集成测试通过真实 ActionService+fixture（fixture 用 appconnector
常量写库、生产代码用字面量比较）提供了偶发性漂移防护，故无现行缺陷；但该防护是间接的——若未来新增比较点或测试重构，常量漂移将静默改变审批/越权判定语义（如合法投递被判
ErrInvalidMaterial、非激活连接通过校验）。建议在 contracts.go 集中定义本地命名常量，并在 *_test.go 中显式与 appconnector
常量做一致性钉死（测试导入不破坏生产依赖倒置）。

- 	if conn.Kind != "personal" || conn.OwnerID != callerID ||
- 		conn.TenantID != tenantID || conn.State != "active" {
- 		return "", ConnectionIdentity{}, ErrConnectionNotUsable
+ // contracts.go 中集中定义（生产代码不导入 appconnector）：
+ // const (
+ // 	RiskDeliver            = "deliver"
+ // 	ActionAwaitingApproval = "awaiting_approval"
+ // 	ActionFailed           = "failed"
+ // 	OutcomeSucceeded       = "succeeded"
+ // 	ConnectionKindPersonal = "personal"
+ // 	ConnectionStateActive  = "active"
+ // )
+ //
+ // contracts_parity_test.go 中显式钉死（测试包可导入 appconnector）：
+ // func TestBoundaryLiteralsMatchAppconnector(t *testing.T) {
+ // 	require.Equal(t, appconnector.RiskDeliver, RiskDeliver)
+ // 	require.Equal(t, appconnector.ActionAwaitingApproval, ActionAwaitingApproval)
+ // 	require.Equal(t, appconnector.ActionSucceeded, OutcomeSucceeded)
+ // 	require.Equal(t, appconnector.ConnectionKindPersonal, ConnectionKindPersonal)
+ // 	require.Equal(t, appconnector.ConnectionActive, ConnectionStateActive)
+ // }
+ //
+ // service.go 使用处改为命名常量：
+ // 	if conn.Kind != ConnectionKindPersonal || conn.OwnerID != callerID ||
+ // 		conn.TenantID != tenantID || conn.State != ConnectionStateActive {
+ // 		return "", ConnectionIdentity{}, ErrConnectionNotUsable


─── internal/modules/workbench/service/workbench/application_task.go:84-88 ───
[maintainability · low] 重复逻辑:EnsureCareerApplicationTask 已对 intent 调用 normalizeApplicationTaskIntent
并把规范化结果传入 ensureWithRetry,而 ensureWithRetry 入口又对同一 intent 再次调用
normalizeApplicationTaskIntent。两次调用幂等无害,但属于可删除的重复校验——建议只在公共入口(EnsureCareerApplicationTask)规范化一次,ensu
reWithRetry 直接信任入参,或将 normalize 收敛到 ensureWithRetry 单处。



─── internal/modules/workbench/service/workbench/application_task.go:269-275 ───
[bug · medium] race 分类器在 SQLite 部署下漏判唯一约束冲突:主 gorm 连接未启用 TranslateError(全仓无 `TranslateError:
true`),SQLite 唯一冲突返回的是 "UNIQUE constraint failed: agent_runs.tenant_id, agent_runs.owner_id,
agent_runs.request_id"(只含列清单、不含索引名,migrations/sqlite/000014 中 uq_agent_runs_request 确实存在且列为
tenant_id/owner_id/request_id)。该文本不匹配任何 marker,errors.Is(gorm.ErrDuplicatedKey) 也为 false →
isApplicationTaskCreationRace 判为非 race → ensureWithRetry 把原始驱动错误当作确定性失败直接抛给 Career,既不重试也不走 recovery
Find/ErrApplicationTaskUndecided。并发双发(幂等重放契约的核心场景)在 SQLite 部署下,twin 已提交 request_id 后,loser 收到的是未分类的裸
DB 错误而非 replay/undecided,违反接口文档"只有 typed 错误才是 definite outcome"的契约(PG 因错误文本含索引名不受影响)。OCR round-4
已将该缺口判为 VALID(medium),但后续 marker 收敛删掉 generic unique 标记时未补上精确的 SQLite 列清单 marker,缺口重新出现。建议补充精确到列清单的
SQLite marker(不会误吞其他表的约束失败),或在 SQLite 主连接启用 gorm.Config.TranslateError:

  	for _, marker := range []string{
  		"uq_agent_runs_request",
  		"uq_workbench_application_tasks_request",
+ 		// SQLite(未启用 TranslateError)唯一冲突只报列清单、不含索引名:
+ 		// 并发 twin 提交后 loser 的 agent_runs 插入撞的就是这条文本。
+ 		"unique constraint failed: agent_runs.tenant_id, agent_runs.owner_id, agent_runs.request_id",
+ 		"unique constraint failed: workbench_application_tasks.tenant_id, workbench_application_tasks.owner_id, workbench_application_tasks.origin, workbench_application_tasks.origin_request_id",
  		"database is locked",
  		"database table is locked",
  		"sqlite_busy",
  	} {


─── package.json:0-0 ───
[maintainability · low] packages/ui 已在本次 TDesign 迁移中整体删除（pnpm-workspace.yaml 已移除该包、仓库中不存在
packages/ui 目录，且同一 diff 的 typecheck:shared 也同步移除了 packages/ui/src/styles.d.ts），但 test:shared 仍残留
`packages/ui/src/*.test.ts` 与 `packages/ui/src/*.test.tsx` 两个 glob，永远匹配不到任何文件，属于死配置。虽不会导致 runner
硬失败，但会误导维护者以为 ui 测试仍在共享门禁中运行；建议与 typecheck:shared 的清理保持一致，一并移除这两段。

- packages/i18n/test/*.test.ts packages/ui/src/*.test.ts packages/ui/src/*.test.tsx packages/views/src/chat/*.test.ts packages/views/src/chat/*.test.tsx
+ packages/i18n/test/*.test.ts packages/views/src/chat/*.test.ts packages/views/src/chat/*.test.tsx


─── packages/api-client/src/career.ts:1471-1471 ───
[bug · medium] note 长度校验与后端字节契约不一致：后端 submission.go:208 以 len(input.Note)（UTF-8 字节数）执行 4096 字节上限，此处用
input.note.length（UTF-16 码元数）比较。约 1366 个汉字（1366 码元、4098 字节）即可通过本地校验却被服务端 400
拒绝——面向中文备注的主场景下该前置校验基本失效，且失败发生在用户提交之后。建议按 UTF-8 字节数统计（TextEncoder），与 maxSubmissionNoteBytes
的语义及错误文案「4096 bytes」保持一致。

-    if (input.note !== undefined && input.note.length > maxSubmissionNoteBytes) throw new CareerValidationError('submission note must not exceed 4096 bytes')
+    if (input.note !== undefined && new TextEncoder().encode(input.note).length > maxSubmissionNoteBytes) throw new CareerValidationError('submission note must not exceed 4096 bytes')


─── packages/api-client/src/career.ts:1432-1432 ───
[maintainability · low] 该方法把 grant.digest 原样作为下载结果的 digest 返回，未对 response.body 计算 SHA-256
复核，也未在赎回前检查 grant.expiresAt；而 MaterialExportDownload 的注释声称 digest 是「下载 blob
必须哈希出的值」，新消费方容易直接信任该字段保存文件。当前 web（MaterialPage.tsx 以 sha256Hex
自行比对）与小程序各自兜底校验，但契约层留此缺口意味着任何未做兜底的消费方会静默保存被篡改/截断的导出文件。建议在方法内用 crypto.subtle 计算摘要并与 grant.digest
比对、不一致即抛错；过期校验可继续依赖后端 export_grant_invalid（两端已处理）。



─── packages/api-client/src/career.ts:1437-1437 ───
[other · low] 撤销以携带 JSON body 的 DELETE 发出，而后端 RevokeMaterialExport（routes_career.go:55 → handler.go
decodeStrictJSON）从 body 绑定 requestId/expectedRevision，body 被剥离即 400 invalid_request。RFC 9110 §9.3.5
对 DELETE 请求体未定义语义，部分网关/代理/CDN 会剥离 DELETE body，届时「撤销立即杀死已签发下载授权」的门禁目标将确定性失败。当前浏览器 fetch + gin
直连可工作，但若部署链路存在反代/网关需改走查询参数（需后端同步调整绑定），或确认链路不剥离后再保留现状。



─── packages/api-client/src/career.ts:1355-1355 ───
[maintainability · low] 死代码：第二个条件不可达。能通过第一个条件（operation !== 'search_once' 为 false）的值只有
'search_once'，其 trim() 结果恒等于自身，因此 operation.trim() !== operation 永远为 false，永远不会因该条件抛错。建议删除冗余判断。

-    if (operation !== 'search_once' || operation.trim() !== operation) throw new CareerValidationError('usage operation must be search_once')
+    if (operation !== 'search_once') throw new CareerValidationError('usage operation must be search_once')


─── packages/api-client/src/career.ts:773-775 ───
[documentation · low] 注释与代码矛盾：注释称「the write inputs carry no source at all」，但
appendProgress/correctProgress 的请求体均显式携带 source: { kind: 'manual' }，且这是服务端必需的——handler.go:1109 的
progressClientSource 对空 Kind 直接以 ErrInvalidRequest 拒绝，通过白名单后才覆写为 manual。若后续维护者按注释删除请求体中的 source
字段，服务端将 400。建议更正注释为「类型不暴露 source，但线上请求体必须携带 kind: manual/user 才能通过服务端白名单，随后被服务端覆写为 manual」。



─── packages/api-client/src/career.ts:1272-1272 ───
[maintainability · low] 标识符归一化策略不一致：多数方法（importUrl、searchOnce、upload、createApplication 等）仅以 .trim()
判空但将原值直接发送，而 setRule/editMaterial 发送 trim 后的值（input.ruleId.trim()、input.materialId.trim()）。后端 JSON
路径不做 TrimSpace（仅 multipart 上传在 handler.go:1614 修剪），因此带空白包裹的 ID 在两类方法下会生成不同幂等键，削弱「同 requestId
重放回执恢复」语义的一致性。建议统一策略：所有标识符一律 trim 后发送，或一律原样发送并注释声明契约。



─── packages/api-client/src/career.ts:1-1 ───
[maintainability · low] 以相对路径 '../../career-core/src/contracts.ts' 跨包导入兄弟包源码，绕过了
@weknora/career-core 已在 package.json 声明的 exports 映射（"./contracts"）。任何目录重组、rootDir 调整或构建器解析策略变化都会直接使
api-client 编译断裂。建议改用包名导入 '@weknora/career-core/contracts'（import 与 decode 两条语句，及 index.ts
中的同源相对导入一并调整）。

- import type { CareerAction, CareerChangeSet, CareerDocumentSource, CareerFact, CareerProposal, CareerReceipt, CareerSource, CareerUpload, CareerView, Evaluation, EvaluationReceipt, OpportunityEvidence, OpportunityImportInput, OpportunityReceipt, OpportunitySource, OpportunityStatus } from '../../career-core/src/contracts.ts'
+ import type { CareerAction, CareerChangeSet, CareerDocumentSource, CareerFact, CareerProposal, CareerReceipt, CareerSource, CareerUpload, CareerView, Evaluation, EvaluationReceipt, OpportunityEvidence, OpportunityImportInput, OpportunityReceipt, OpportunitySource, OpportunityStatus } from '@weknora/career-core/contracts'


─── packages/api-client/src/index.ts:209-211 ───
[maintainability · low] 包入口只导出 createCareerApi/CareerRequest 和 8 个类型，而消费方实际需要
CareerValidationError（SubmissionPage 以 instanceof 判别本地校验失败）、全部 view/receipt 类型及 decode* 函数，导致
web/小程序二十余处被迫以 '../../../../packages/api-client/src/career.ts' 深层相对路径导入。深层导入与入口引用同一模块实例，instanceof
目前仍成立，但入口只露出一角会诱导后来者从入口导入却拿不到错误类与类型，形成两条并行导入路径。建议补全 career.ts 的公开导出（至少 CareerValidationError、各
decode* 与本组页面用到的类型），或移除这三行避免半截入口；同时此处的 career-core 相对路径导入也应改用包名 '@weknora/career-core/contracts'。



─── packages/career-core/package.json:1-1 ───
[maintainability · medium] exports 声明的 ./contracts、./desk
子路径当前无任何消费方经包名引用：packages/api-client/src/career.ts、apps/web/src/career/*、apps/miniprogram/src/servic
es/career.ts 等全部以相对路径（../../career-core/src/contracts.ts）绕过包边界导入，且 packages/api-client/package.json
的 dependencies 只声明了 @weknora/contracts 与 @weknora/domain，未声明
@weknora/career-core（workspace:*）。当前全靠磁盘相对路径 + tsx/tsc 直载生效，exports 映射属于零消费、零验证的死配置：一旦目录调整、单独对
api-client 做 install/typecheck/构建或包独立发布，解析即断裂，依赖图也失真。建议在 api-client（及其他消费方）package.json 补充
"@weknora/career-core": "workspace:*" 并将相对路径导入统一改为包名子路径导入，与本 exports 声明对齐。

- {"name":"@weknora/career-core","version":"0.0.0","private":true,"type":"module","exports":{"./contracts":"./src/contracts.ts","./desk":"./src/desk.ts"}}
+ // packages/api-client/package.json
+ // "dependencies": { ... "@weknora/career-core": "workspace:*" }
+ // 并将 import ... from '../../career-core/src/contracts.ts' 改为 from '@weknora/career-core/contracts'


─── packages/career-core/src/contracts.ts:158-158 ───
[maintainability · low] decodeEvaluation 开头的 !isRecord(value) 与第二份 12 字段 onlyKeys
列表是恒假的死检查：validEvaluationReceipt(value) 已经验证过 isRecord 和完全相同的 ['kind','requestId',...,'facts']
键列表，二者在其通过后不可能再失败。两处硬编码键列表（validEvaluationReceipt
与本函数）需同步维护，后续增删字段极易漂移（改了一处漏一处时，类型收窄与运行时校验将不一致）。建议提取共享常量（如 const EVALUATION_KEYS = [...] as
const）并删除本函数内重复的 onlyKeys/isRecord 检查。

- export function decodeEvaluation(value: unknown): Evaluation {
+ const EVALUATION_KEYS = ['kind', 'requestId', 'evaluationId', 'opportunityId', 'snapshotId', 'profileRevision', 'status', 'createdAt', 'rulesetVersion', 'snapshot', 'hard', 'soft', 'facts'] as const
+ // validEvaluationReceipt 与 decodeEvaluation 共用 EVALUATION_KEYS，删除 decodeEvaluation 中重复的 onlyKeys/isRecord 检查


─── packages/career-core/src/contracts.ts:160-161 ───
[maintainability · low] decodeEvaluation 使用 TextEncoder/TextDecoder 按 UTF-8 字节校验证据锚点，而同包 desk.ts
的注释明确约束"共享包需保持（小程序端）可直载"。微信小程序 JS 运行时并不保证提供全局 TextEncoder/TextDecoder（依基础库版本而异）；当前小程序侧确实未调用
decodeEvaluation（services/career.ts 只引入 decodeEvaluationReceipt 等轻量解码器），但没有任何机制阻止未来误用——一旦小程序页面调用将直接
ReferenceError 崩溃且无编译期信号。建议：为 decodeEvaluation 增加 TextEncoder 存在性守卫（缺失时给出明确错误码），或在包内文档/注释标注该函数仅限具备
WHATWG 编码 API 的运行时（web/Node）使用。

   const rawText = value.snapshot.rawText
+  if (typeof TextEncoder === 'undefined' || typeof TextDecoder === 'undefined') throw Object.assign(new TypeError('evaluation span verification requires TextEncoder/TextDecoder (web/Node runtime only)'), { code: 'runtime_unsupported' })
   const bytes = new TextEncoder().encode(rawText)


─── packages/career-core/src/contracts.ts:89-89 ───
[maintainability · low] OpportunitySource 与文件顶部已定义的 CareerSource 结构逐字段相同（kind: string; label?;
referenceId?），属重复定义；且对同一"source"结构包内并存三种校验强度：validSource 仅要求 kind 为 string（允许空串），而
decodeOpportunityEvidence 与 decodeEvaluation（snapshot.source）用 validIdentifier
要求非空非空白。同一概念多套校验会随演化继续分叉（例如未来 CareerSource 增加 referenceKind 字段时只有一处会更新）。建议 OpportunitySource 直接复用
CareerSource（或互相别名），校验统一收敛到 validSource（如需非空约束则提高 validSource 本身的强度，一处生效）。

- export type OpportunitySource = { kind: string; label?: string; referenceId?: string }
+ export type OpportunitySource = CareerSource // kind/label/referenceId 与 CareerSource 同构，避免重复定义与校验分叉


─── packages/career-core/src/desk.ts:134-134 ───
[bug · low] send() 中 decodeCareerReceipt 抛出的 TypeError 没有 code/status，isAmbiguousOutcome
会将其判为"结果未知"（status === undefined → true）并置 unresolved——把 act() 归为 ambiguous 本身是正确的防双写选择，但随后回退
receipt() 查询若返回同样畸形的回执，会再次解码失败：unresolved 永不解除、receiptMissing=false，此后同空间内所有 mutate() 恒抛
unresolved_action，retryUnknown/reconcile 也只能反复抛 outcome_unknown(safeToRetry=false)，除
activate()/clear()（整空间重置）外没有任何就地解除路径，且消费方（如 CareerPage 的恢复 UI）只提供对账/重试入口。建议给边界解码失败附加显式错误码（如
contract_violation）以便与传输类歧义区分，并在回执持续解码失败时向调用方暴露可确认的本地重置出口（clear() 路径 + 文档说明），避免服务端契约回归时客户端永久锁死。

-    const receipt = decodeCareerReceipt(await this.remote.act(action, signal))
+    let decoded: CareerReceipt
+    try { decoded = decodeCareerReceipt(await this.remote.act(action, signal)) } catch (cause) { throw Object.assign(new TypeError('career act receipt failed boundary decoding', { cause }), { code: 'contract_violation' }) }
+    const receipt = decoded


─── packages/career-core/src/desk.ts:172-174 ───
[bug · low] mergeProposal 在 current 与 incoming 均为 pending 时不校验 revision（current.status === 'pending'
短路直接覆盖），与 mergeFact 的 incoming.revision >= current.revision 防回退守卫不对称：若乱序到达（如 syncChanges 已合入较新的
pending 修订后，一次滞留 reconcile 的旧回执再应用），旧 pending 数据会覆盖较新的 pending 更新。虽然 syncChanges
批内按服务端顺序应用、常规路径不易触发，但 applyReceipt（reconcile/send 回退）与 syncChanges 交错时存在该窗口。建议 pending→pending 分支同样加
revision 防回退（对 revision 缺失的情况放行，兼容服务端未填充该字段的提案）。

   const current = proposals[index]!
   if (current.status !== 'pending' && incoming.status === 'pending') return
-  if (current.status === 'pending' || incoming.status !== 'pending' || (incoming.revision ?? 0) >= (current.revision ?? 0)) proposals[index] = incoming
+  const stalePending = current.status === 'pending' && incoming.status === 'pending' && incoming.revision !== undefined && current.revision !== undefined && incoming.revision < current.revision
+  if (!stalePending && (current.status === 'pending' || incoming.status !== 'pending' || (incoming.revision ?? 0) >= (current.revision ?? 0))) proposals[index] = incoming


─── packages/career-core/src/desk.ts:146-146 ───
[bug · medium] 回执归属校验缺失：decodeCareerReceipt 只做形态校验，不校验 receipt.requestId 与本次查询的 requestId 一致。此处（以及
reconcile() 中同样模式）拿到形态合法但属于其他请求的回执时，会直接 applyReceipt 并清除 unresolved——把 pending 的 unknown action
用错配回执"了结"，之后 mutate 解锁、原 requestId 的对账/安全重试通道随之关闭，调用方 UI 还会把 receipt.requestId 展示成"已找回回执"误导用户。仓库在
web 端 protocol.ts 已把 ReceiptMismatchError（回执与本次请求不匹配）定为"确定性协议错误"红线，共享 desk 路径应有等价守卫。建议
decodeCareerReceipt 增加期望 requestId 参数（或 desk 侧校验），不匹配时抛带 code 的确定性错误而非当作成功回执。

      const receipt = decodeCareerReceipt(await this.remote.receipt(action.requestId, signal))
+     if (receipt.requestId !== action.requestId) throw Object.assign(new Error('career receipt belongs to a different request'), { code: 'receipt_mismatch' })
+     // reconcile() 内同理：if (receipt.requestId !== requestId) throw ...


─── packages/career-core/src/desk.ts:22-22 ───
[maintainability · medium] Career 错误码词表在本包内三处手抄硬编码：contracts.ts decodeCareerError 的 8 码 codes 数组、此处
isAmbiguousOutcome 的 6 码确定性表、retryUnknown 的 4 码清除列表
['revision_conflict','invalid_request','proposal_resolved','not_found']；小程序
services/career-intent.ts 的 ambiguousOutcome 又是第 4 份副本。三处语义高度耦合（某码被判"确定性"就绝不能落入 ambiguous 分支置
unresolved，反之误清除会导致同 requestId 换 payload 双写）。后续服务端新增确定性错误码时只更新一处，其余各处会静默把新码判为歧义（卡死
mutate）或漏掉清除逻辑，且无编译期信号。建议从 contracts.ts 导出单一词表常量（如 CAREER_DEFINITE_ERROR_CODES），desk 两处与外部副本均由其派生。

-  if (code && ['forbidden', 'revision_conflict', 'idempotency_conflict', 'invalid_request', 'not_found', 'proposal_resolved'].includes(code)) return false
+ // contracts.ts:
+ // export const CAREER_DEFINITE_ERROR_CODES = ['forbidden', 'revision_conflict', 'idempotency_conflict', 'invalid_request', 'not_found', 'proposal_resolved'] as const
+ // desk.ts:
+  if (code && (CAREER_DEFINITE_ERROR_CODES as readonly string[]).includes(code)) return false


─── packages/career-core/src/desk.ts:26-26 ───
[bug · low] sameAction 用 JSON.stringify 逐字符比较，对对象键插入顺序敏感：retryUnknown 的契约是"重试携带完全相同的
action"，但两个语义完全相同的 action 对象（字段一致、构造时键顺序不同，例如跨持久化重建或由新调用点拼装）会序列化成不同字符串，被误判 retry_payload_mismatch——而
mutate 又被 unresolved_action 阻塞，唯一的安全重试出口被堵死，只能走 reconcile。当前 web
端恰好持有原对象引用（setUnknownAction(action)）未触发，小程序 retryPending(action) 包装也无调用方，但这是共享包的公开
API，防御应内建。建议改为逐字段比较（action/requestId/expectedRevision/source/key/value/proposalId）或先按键排序做规范化序列化。

- function sameAction(left: CareerAction, right: CareerAction): boolean { return JSON.stringify(left) === JSON.stringify(right) }
+ function sameAction(left: CareerAction, right: CareerAction): boolean {
+  if (left.action !== right.action || left.requestId !== right.requestId || left.expectedRevision !== right.expectedRevision) return false
+  if (JSON.stringify(left.source) !== JSON.stringify(right.source)) return false
+  const field = (a: CareerAction, b: CareerAction, key: 'key' | 'value' | 'proposalId'): boolean => (a[key] ?? null) === (b[key] ?? null)
+  return field(left, right, 'key') && field(left, right, 'value') && field(left, right, 'proposalId')
+ }


─── packages/design-tokens/src/styles.css:23-23 ───
[bug · medium] 新增的 @import "./tdesign-theme.css" 位于 :root 规则块（本文件第 3-17 行）之后，违反 CSS 规范：@import
必须出现在除 @charset/@layer 声明外的所有规则之前，规范严格的处理器或浏览器直载会静默丢弃该 import，导致 --td-* 主题覆盖整体失效（品牌色回退 TDesign
默认蓝）。当前 apps/web 与 apps/embed 均依赖 Vite/postcss-import 对任意位置 @import 的宽容内联才生效，属侥幸路径。此外该嵌套 import
还有两个连带问题：(1) apps/web/src/styles.css:7 已直接 @import tdesign-theme.css（不带 layer），此处又随 styles.css
layer(theme) 内联一份惰性副本，同一 200 行主题文件被打包两份；(2) apps/embed/src/styles.css:4 引入本文件，但 embed 全程无任何 tdesign
使用，被连带注入 input:-webkit-autofill !important 覆盖、.t-* 规则、*::-webkit-scrollbar !important 及
.doc-link/.more-icon 通用类（layer 内 !important 声明按层叠规则会压过 embed 自身未分层样式）。建议直接移除该 @import（TDesign
消费方自行显式引入 tdesign-theme.css，apps/web 已如此），若 tokens 包确需携带主题，至少将 @import 移到文件最顶部所有规则之前。

+ /* 方案 A（推荐）：移除本行，由使用 TDesign 的应用显式引入 tdesign-theme.css
+    （apps/web/src/styles.css:7 已直引），避免 embed 等非 TDesign 消费方被注入全局规则。
+    方案 B：若保留，移至文件第 1 行（任何 :root 规则之前），符合 CSS @import 规范：
- @import "./tdesign-theme.css";
+    @import "./tdesign-theme.css";
+    :root { ... }


─── packages/design-tokens/src/tdesign-theme.css:2-2 ───
[maintainability · low] 文件是 Vue 主题的原样平移，但文件本身未标注来源路径（来源说明只出现在 styles.css 的 @import
注释里，本文件被单独直引时无从溯源）。另外 --td-line-height-title-large 在 light/dark 两段均以
--td-font-size-title-medium（16px）而非 title-large（20px）为基数计算（20px 标题行高被算成 24px），与同族其他条目的规律不一致，疑似 Vue
源（frontend/src/assets/theme/theme.css 第 1、92
行同样写法）的上游笔误被原样带入。平移保真可以接受，但建议在文件头注明来源并标注该疑点，供后续与上游确认后两端一并修复；顺带提醒 --td-brand-color-focus 使用
color-mix() 无旧浏览器回退值，与 Vue 行为一致，若构建目标需覆盖 Chrome <111 需补回退。

+ /* 来源：frontend/src/assets/theme/theme.css 原样平移（Issue #140 TDesign 闭包构建，Phase 1 Task 4）。
+    已知保留疑点：--td-line-height-title-large 以 --td-font-size-title-medium 为基数
+    （Vue 源同款写法，疑似上游笔误，为保真暂不修改，待与上游确认后两端统一修复）。 */
  :root:root[theme-mode="dark"]{


─── packages/design-tokens/src/tdesign-theme.css:38-38 ───
[maintainability · low] 疑似上游色值笔误被原样平移（与已确认的 --td-line-height-title-large 笔误同类）：深色块 error 色阶 1→9 从
#472324 递亮到 #edb1b6 全部为红色系，而 color-10 却是黄绿色调的 #eeced0；对照同族 warning-10 #f3e9dc、success-10 #deede8
均为各自色相的淡色，error 系此处应为淡粉（如 #f0dfe0 一带）。error-color-10
多用作错误态渐变/淡背景端点，绿色偏色会造成观感异常。既然文件定位是保真平移，建议至少在此处加注释标注疑点并同时向上游 Vue 仓库确认修正，避免笔误被双端固化。



─── packages/mobile-core/src/runtime/mobile-runtime.ts:199-200 ───
[bug · medium] 写请求 401 时（retryUnauthorized=false）既不重放也不刷新凭据，过期的 access token 会继续留在
credentialStore：后续所有写请求仍读到旧 token、反复 401，只有等一次 GET/HEAD/OPTIONS（或 shelf 的主动
refresh）才会触发轮换。对纯写流程（如后台投递、进度回执重试推送）会陷入无法自愈的 401 循环。建议改为「刷新但不重放」：仍执行单飞 refreshedCredential 轮换凭据后原样抛出
401——重放抑制已由该分支保证不会双重写入，而刷新能让用户下一次重试直接用上新 token（注意需同步调整 mobile-runtime.test.ts 中断言 POST 401 后
refreshes === 0 的用例）。

        if (!current(requestEpoch, deployment)) throw new Error('RUNTIME_SCOPE_CHANGED');
-       if (!retryUnauthorized) throw error;
+       if (!retryUnauthorized) {
+         // 刷新但不重放：非幂等写不重发，仅轮换凭据，避免后续写持续命中过期 token。
+         await refreshedCredential(requestEpoch, deployment, credential);
+         throw error;
+       }


─── packages/mobile-core/src/runtime/mobile-runtime.ts:189-189 ───
[documentation · low] 函数上方的文档注释仍写着「replays exactly once through a single-flight refresh on a
pre-send 401」，但现在只有安全方法（GET/HEAD/OPTIONS）才会重放，写方法 401 是原样上抛、不刷新不重放。建议同步更新该注释（以及 types.ts 中
authorizedRequest 的「refresh-once on 401」接口描述），说明按方法的安全/非幂等性区分 401 处理，避免误导后续维护者。



─── packages/mobile-core/src/runtime/mobile-runtime.ts:389-390 ───
[maintainability · medium] 401 重放安全策略只在此调用点以内联白名单实现，而 authorizedEventStream（本文件 line 411）调用
sendWithCredential 时不传第 4 参、走默认 retryUnauthorized = true，完全绕过该判定——一旦流通道的 method 非安全方法（如 POST 型
SSE/订阅），就会违反本次改动建立的「非幂等请求绝不重放」不变量。且默认值 true 属
fail-open：后续新增调用点若漏传标志，非幂等写会被静默重放。建议将安全方法判定提取为模块级常量/辅助函数，两个调用点统一显式传参，并把默认值改为
false（fail-closed）。另：局部变量名 readOnly 与语义略有偏差（HEAD/OPTIONS 并非「读」，判定本质是 RFC 9110 safe/可重放方法），建议命名为
replaySafe 之类。

-       const readOnly = ['GET', 'HEAD', 'OPTIONS'].includes(input.method.toUpperCase());
-       return await sendWithCredential(epoch, deployment, (token) => transport(input, token), readOnly);
+ // 模块级提取共享判定：
+ const REPLAY_SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS']);
+ const isReplaySafeMethod = (method: string): boolean => REPLAY_SAFE_METHODS.has(method.toUpperCase());
+ 
+ // authorizedRequest：
+ return await sendWithCredential(epoch, deployment, (token) => transport(input, token), isReplaySafeMethod(input.method));
+ 
+ // authorizedEventStream 同步显式传参：
+ await sendWithCredential(requestEpoch, deployment, (token) => transport({ ...input, signal: controller.signal }, token, guardedChunk), isReplaySafeMethod(input.method));
+ 
+ // sendWithCredential 默认值改为 fail-closed：
+ const sendWithCredential = async <R>(requestEpoch: number, deployment: Deployment, send: (token: string) => Promise<R>, retryUnauthorized = false): Promise<R> => { /* ... */ }


─── packages/views/src/chat/chat-copy.ts:1177-1178 ───
[documentation · low] 新增键 requestInfoEmpty 的翻译不完整：日文表已本地化为「リクエスト情報はありません」，但韩文表与俄文表均直接落英文兜底 'No
request info available'（其余同期新增键 noResult 两表都做了本地化，如 '결과 없음' / 'Нет результатов'），五个语言表行为不一致，ko/ru
用户将在请求信息空态卡（message-face.tsx RequestInfoButton）看到英文文案。建议补齐韩文/俄文翻译。

- requestInfoEmpty: 'No request info available',
+ requestInfoEmpty: '요청 정보가 없습니다',
     thinkingAlt: '생각 중',


─── packages/views/src/chat/composer.tsx:538-538 ───
[maintainability · low] 新版提及菜单按 Vue MentionSelector 事实源移除了搜索输入，但 mentionQuery 状态被残留：setMentionQuery
仅在 openMentions/closeMentions 写入空串，第 306 行的 name.includes(mentionQuery) 过滤恒为真——按关键字过滤候选的能力静默失效，且
chat-copy.ts 五个语言表的 mentionNoResults 键已无任何代码消费点。建议删除 mentionQuery 状态与过滤子句（仅保留已选项排除），并同步清理
mentionNoResults 死文案；若要保留过滤能力，需在 textarea 输入时派生 query。



─── packages/views/src/chat/composer.tsx:324-324 ───
[bug · medium] mentionMenuStyle 仅在 toggleMentions 打开瞬间依据 textarea 的 getBoundingClientRect 计算一次 fixed
坐标，打开期间窗口 resize、移动端软键盘弹出、或 has-sandbox-panel/has-references-panel 面板开合导致 composer
移位时均不会重算——弹层将停留在视口旧位置发生漂移（fixed 定位在带 transform 的祖先内还会改锚）。建议监听 resize/scroll 重算，或参照常见做法在这些事件时直接
closeMentions() 关闭菜单。

            setMentionMenuStyle({ position: 'fixed', left: `${rect.left}px`, bottom: `${vh - rect.top + 8}px`, top: 'auto' });
+   // …并补：
+   useEffect(() => {
+     if (!mentionOpen) return;
+     const reposition = () => closeMentions(); // 或按 textarea rect 重算
+     window.addEventListener('resize', reposition);
+     window.addEventListener('scroll', reposition, true);
+     return () => {
+       window.removeEventListener('resize', reposition);
+       window.removeEventListener('scroll', reposition, true);
+     };
+   }, [mentionOpen]);


─── packages/views/src/chat/composer.tsx:238-239 ───
[bug · low] kbTipTimer 的两个 setTimeout（300ms 显示 / 80ms 隐藏）缺少组件卸载清理：卸载后回调仍会触发 setKbTipOpen，定时器泄漏。同批新增的
message-face.tsx AnswerToolbar 对 copied timer 已做卸载清理，两处标准不一致。建议补齐卸载清理 effect。

    const [kbTipOpen, setKbTipOpen] = useState(false);
    const kbTipTimer = useRef<number | null>(null);
+   useEffect(() => () => { if (kbTipTimer.current !== null) window.clearTimeout(kbTipTimer.current); }, []);


─── packages/views/src/chat/message-face.tsx:289-290 ───
[maintainability · low] message-face.tsx 私有实现了 writeClipboardText（含 execCommand 兜底），而
message-list.tsx 已导出同名同逻辑函数（且 message-list.test.tsx 对其有测试覆盖）。两份剪贴板降级实现并行维护，后续修 bug
易漏改一处。建议复用导出版本或将实现下沉到共享模块后两处引用。

- async function writeClipboardText(text: string): Promise<void> {
-   const target = typeof navigator !== 'undefined' ? navigator.clipboard : undefined;
+ import { writeClipboardText } from './message-list.tsx';
+ // 删除本文件的私有实现，直接复用已导出（且有测试覆盖）的版本


─── packages/views/src/chat/message-face.tsx:401-403 ───
[bug · low] 空配置对象使 invalidImageLabel 回退到 markdown.ts 硬编码的中文占位符
INVALID_IMAGE_LINK_PLACEHOLDER（'无效的图片链接'）——en/ja/ko/ru 语言环境下失效图片链接将显示中文占位，与旧 message-list 路径传入
t.invalidImageLink 的本地化行为不一致。组件已持有 props.copy，建议显式透传本地化文案。

-   const row = props.message as Record<string, unknown>;
-   const showRequestInfo = Boolean(row.request_id || props.message.id);
-   const html = renderChatMarkdown(props.content, {});
+   const html = renderChatMarkdown(props.content, {
+     invalidImageLabel: props.copy.invalidImageLink,
+   });


─── packages/views/src/chat/message-face.tsx:203-207 ───
[style · low] referenceText 使用两层嵌套三元选择三分支文案，违反「禁止嵌套三元表达式」的编码约定。建议提取为具名函数（if 早返回）或查表映射，提升可读性。

-   const referenceText = docCount > 0 && webCount > 0
-     ? props.copy.referencesDocAndWebCount.replace('{docCount}', String(docCount)).replace('{webCount}', String(webCount))
-     : docCount > 0
-       ? props.copy.referencesDocCount.replace('{count}', String(docCount))
-       : props.copy.referencesWebCount.replace('{count}', String(webCount));
+   const referenceText = formatReferenceCount(props.copy, docCount, webCount);
+ 
+ function formatReferenceCount(copy: ChatCopyTable, docCount: number, webCount: number): string {
+   if (docCount > 0 && webCount > 0) {
+     return copy.referencesDocAndWebCount.replace('{docCount}', String(docCount)).replace('{webCount}', String(webCount));
+   }
+   if (docCount > 0) return copy.referencesDocCount.replace('{count}', String(docCount));
+   return copy.referencesWebCount.replace('{count}', String(webCount));
+ }


─── packages/views/src/chat/message-face.tsx:372-373 ───
[bug · medium] 答案工具条从 message-list.tsx 迁入本文件时丢失了「推荐问题生成中」的加载指示：旧实现在工具条内渲染 `suggestions?.status ===
'generating' && index === messages.length - 1` 的 role="status" 元素（wk-chat-follow-up-loading + 灯泡图标 +
followUpQuestionsLoading 文案），新 AnswerToolbar 及其调用链（BotMessageFace/MessageList）均未透传
suggestions，generating 态在 UI 上完全无反馈，且 chat-copy.ts 五个语言表的 followUpQuestionsLoading 键已无任何消费点（孤儿键）。另外旧
CopyAnswerButton 有 `if (!text) return null` 门控，新工具条对空内容消息仍渲染复制按钮（点击静默无效），一并确认是否为刻意行为。建议在
AnswerToolbar（或 message-list 的 assistant 行）补回 generating 提示并透传 suggestions/文案；若确认 Vue 事实源无此面，请同步清理
followUpQuestionsLoading 死键。

-     <div className="answer-toolbar wk-chat-answer-toolbar">
-       <ToolbarButton icon="copy" title={copied ? copy.copied : copy.copy} className="wk-chat-copy" onClick={() => void copyAnswer()} />
+ export function AnswerToolbar(props: AnswerToolbarProps) {
+   // ...
+   if (!props.rendered) return null;
+   // 透传 suggestions generating 态（Vue follow-up loading 面等价物）：
+   // {props.suggestions?.status === 'generating' ? (
+   //   <span className="wk-chat-follow-up-loading" role="status">{copy.followUpQuestionsLoading}</span>
+   // ) : null}


─── packages/views/src/chat/message-list.tsx:377-378 ───
[maintainability · low] 手动「加载更早消息」按钮（wk-chat-load-older）随本次重构删除，加载旧消息现在仅依赖 onScroll 中 scrollTop<=0
的自动触发；同时 chat-copy.ts 五个语言表的 loadOlder / loadingHistory 文案键在 packages/views 与 apps/web
中均已无任何消费点，成为孤儿键。若删除按钮属 Vue 事实源对齐（触顶自动加载），建议同步清理这两个死键；否则应保留手动入口作为自动触发失效时的兜底。



─── packages/views/src/chat/page.tsx:629-631 ───
[bug · medium] 回退面（未注入 headerSlot 时）的 ⋯ 按钮没有任何 onClick——内置 ChatHeaderMenu
删除后，onRenameSession/onToggleSessionPin/onDeleteSession/onClearSession/headerUtilityItems
等回调在包内回退路径一个也不消费，重命名/置顶/清空/删除等会话管理动作对未注入插槽的消费方（如 chat-header-hook-order.test.tsx 直接渲染 ChatPage
的用例，以及未来任何复用方）全部不可达，headerUtilityItems 沦为死 prop。建议：回退按钮至少挂接既有 props 回调的最简菜单；或在无 headerSlot
且无任何会话动作回调时整行不渲染该按钮，并同步从 ChatPageProps 移除/注明 headerUtilityItems 由 headerSlot 取代。

-           <button type="button" className="chat-header__menu-btn wk-chat-header-menu" aria-label={copy.moreActions}>
+           {hasSessionActions ? (
+             <button type="button" className="chat-header__menu-btn wk-chat-header-menu" aria-label={copy.moreActions} aria-haspopup="menu" onClick={openFallbackSessionMenu}>
-             <svg className="t-icon t-icon-ellipsis" viewBox="0 0 24 24" width="16px" height="16px" fill="none" aria-hidden="true"><use href="#t-icon-ellipsis" /></svg>
+               <svg className="t-icon t-icon-ellipsis" viewBox="0 0 24 24" width="16px" height="16px" fill="none" aria-hidden="true"><use href="#t-icon-ellipsis" /></svg>
-           </button>
+             </button>
+           ) : null}


─── packages/views/src/chat/page.tsx:687-694 ───
[bug · medium] 回底控件由原 <button aria-label={t.chatScrollBottom}> 改为仅带 onClick 的 div：无
role/tabIndex/aria-label，键盘用户无法触达，屏幕阅读器无语义；同时 chat-copy.ts 五个语言表的 chatScrollBottom
文案键在代码中已无任何消费点（孤儿键）。建议恢复 button 语义（或补 role="button" tabIndex={0} + 键盘事件）并接回 copy.chatScrollBottom。

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


─── packages/views/src/chat/session-sidebar.tsx:305-305 ───
[bug · medium] 行结构 Vue 化时删除了 parent_session_id 分叉徽标⑂与 sessionSourceBadge
来源徽标（IM/Embed/API）的渲染：分叉会话与普通会话在侧栏不可区分，packages/domain 的 sessionSourceBadge
现仅剩自身测试引用（无生产消费方），chat-copy.ts 五个语言表的 forkBadgeTooltip 成为无任何消费点的孤儿键。若确属 Vue 事实源收敛，请同步清理死键与死函数；否则建议在
pin 图标旁补回 parent_session_id 徽标。

                        {session.is_pinned ? <svg className="t-icon submenu_pin_icon" viewBox="0 0 24 24" width="1em" height="1em" fill="none" aria-hidden="true"><use href="#t-icon-pin" /></svg> : null}
+                       {session.parent_session_id ? <span className="submenu_fork_icon" role="img" aria-label={t.forkBadgeTooltip} title={t.forkBadgeTooltip}>⑂</span> : null}


─── packages/views/src/chat/session-sidebar.tsx:307-307 ───
[maintainability · low] 同一行 JSX 内 apiOwnerTagOf(session) 被重复调用 4 次，并依赖 3 处非空断言
!——冗余计算且脆弱（任一处判断与后续断言不一致即运行时异常）。建议在 map 回调顶部提为 const 后单次使用。

-                       {apiOwnerTagOf(session) ? <span className={'session-owner-tag session-owner-tag--' + apiOwnerTagOf(session)!.kind} title={apiOwnerTagOf(session)!.full}>{apiOwnerTagOf(session)!.label}</span> : null}
+         {group.items.map((session) => {
+           const active = session.id === selectedSessionId;
+           const ownerTag = apiOwnerTagOf(session);
+           // …
+                       {ownerTag ? <span className={'session-owner-tag session-owner-tag--' + ownerTag.kind} title={ownerTag.full}>{ownerTag.label}</span> : null}


─── packages/views/src/chat/tool-approval.tsx:190-190 ───
[style · low] 计时器颜色的三分支选择写成嵌套三元（a ? x : b ? y :
z），违反「禁止嵌套三元表达式」约定，且与旧实现一样难以扩展第四种状态。建议提取为查表映射或早返回的辅助函数。

-           className={`wk-chat-approval-timer wk-vc-tool-approval-12 ${timerClass === 'wk-timer-warning' ? 'wk-timer-warning wk-vc-tool-approval-13' : timerClass === 'wk-timer-critical' ? 'wk-timer-critical wk-vc-tool-approval-14' : 'wk-vc-tool-approval-11'}`}
+ const TIMER_VARIANT_CLASS: Record<string, string> = {
+   'wk-timer-warning': 'wk-timer-warning wk-vc-tool-approval-13',
+   'wk-timer-critical': 'wk-timer-critical wk-vc-tool-approval-14',
+ };
+ // ...
+ <span className={`wk-chat-approval-timer wk-vc-tool-approval-12 ${TIMER_VARIANT_CLASS[timerClass] ?? 'wk-vc-tool-approval-11'}`}>


─── packages/views/src/chat/tool-result.tsx:857-857 ───
[style · low] WebFetchRenderer 的状态徽标配色沿用 `statusKind === 'ok' ? A : statusKind === 'failed' ? B : C`
嵌套三元（本次重写 className 时保留），违反「禁止嵌套三元表达式」约定。statusKind 本身已是有限字面量联合（ok/failed/redirect
等），查表映射更直观也便于补充分支。

-             {row.status ? <span className={`wk-tool-status-pill is-${row.statusKind} wk-vc-tool-result-30 ${row.statusKind === 'ok' ? 'wk-vc-tool-result-31' : row.statusKind === 'failed' ? 'wk-vc-tool-result-32' : 'wk-vc-tool-result-33'} wk-vc-tool-result-8`}>{row.status}</span> : null}
+ const STATUS_KIND_CLASS: Record<string, string> = {
+   ok: 'wk-vc-tool-result-31',
+   failed: 'wk-vc-tool-result-32',
+ };
+ // ...
+ {row.status ? <span className={`wk-tool-status-pill is-${row.statusKind} wk-vc-tool-result-30 ${STATUS_KIND_CLASS[row.statusKind] ?? 'wk-vc-tool-result-33'} wk-vc-tool-result-8`}>{row.status}</span> : null}


─── packages/views/src/craft/shell.tsx:100-100 ───
[bug · medium] CraftDrawer 的 JSDoc 承诺 "full width under 760px via CSS"，但该契约未落地：(1) craft.css 及全库均无任何
.wk-craft-drawer 规则或 760px 媒体查询；(2) td.tsx Drawer 把 size 写成内联样式 style={{ width: size }}，内联样式优先级高于普通
CSS，即使后续补媒体查询也必须 !important 才能覆盖。结果是 ≤460px 视口（如 390px 手机）下抽屉固定 460px 宽、左缘超出视口被裁剪。建议在 craft.css 补
@media (max-width: 760px) { .wk-craft-drawer .t-drawer__content-wrapper { width: 100% !important; }
}，或让 td Drawer 改用 class/CSS 变量承载宽度；若暂不处理窄屏，请同步修正注释以免误导后续维护者。



─── packages/views/src/craft/td.tsx:6-8 ───
[bug · medium] 头注释对 fixed 树内渲染的前提论断有误："fixed 的 containing block 是视口，不受 shell overflow-x: clip 裁剪" 只对
overflow: hidden/scroll 成立。按 CSS Overflow Module Level 3 及 Chromium/Firefox/WebKit 实现，overflow: clip
与 hidden 的标志性差异之一正是它会裁剪 position:fixed 后代。而 craft.css:5/.wk-craft-page 与
.wk-craft-shell（craft.css:115）都固定为 overflow-x: clip（shell.test.tsx:140 还断言了该契约），CraftShell ×
CraftDrawer 正是本层的指定组合——一旦抽屉渲染在 shell 内，fixed 遮罩/面板会被裁剪到 ≤1180px 的 shell 条带：宽屏下右侧 460px
面板完全落在裁剪区外不可见。建议与真实 tdesign 一致改挂 document.body（portal 或由调用方注入容器）；至少应修正该注释前提。另 useBodyPortal 名为
portal 实际只返回布尔值做树内渲染，命名与行为不符，建议改名（如 shouldMountOverlay）。

- // 宿主 app 已全局加载 tdesign.css（apps/web styles.css:2），t-* 类即获得真实
- // tdesign 视觉。弹层为 fixed 定位树内渲染（fixed 的 containing block 是视口，
- // 不受 shell overflow-x: clip 裁剪），焦点/Escape/焦点归还契约与原 Sheet 实现
+ // 弹层经 portal 挂载 document.body（与真实 tdesign Drawer/Dialog 一致），
+ // 避免任何祖先（含 .wk-craft-shell 的 overflow-x: clip，按 CSS Overflow L3
+ // 它会裁剪 fixed 后代）裁剪/捕获弹层定位。


─── packages/views/src/craft/td.tsx:112-115 ───
[bug · medium] effect 依赖含 onClose，而唯一 Dialog 调用点 versions.tsx:98 传的是内联箭头 onClose={() =>
setConfirming(null)}，每次渲染都是新引用：弹层打开期间父组件任何重渲染都会重跑 effect——cleanup 先 restoreRef.current?.focus()
把焦点移回弹层外的触发元素（焦点瞬间逃出模态），effect 体内再 panelRef.current?.focus() 把焦点抢回面板根，用户在弹层内 Tab
到的焦点位置被重置回面板根，document 级监听也反复卸载重挂。建议用 latest-ref 模式，effect 只依赖 open。

+   const onCloseRef = useRef(onClose);
+   onCloseRef.current = onClose;
+   useEffect(() => {
+     if (!open) return undefined;
+     // ...
+     onCloseRef.current();
+     // ...
-       restoreRef.current?.focus();
+     restoreRef.current?.focus();
-       restoreRef.current = null;
+     restoreRef.current = null;
-     };
-   }, [onClose, open]);
+   }, [open]);


─── packages/views/src/craft/td.tsx:103-104 ───
[bug · medium] 该守卫要求 activeElement 位于面板内才允许 Escape 关闭：浏览器中点击弹层内不可聚焦内容（纯文本、div）后 activeElement 会变为
body，body 不被 panelRef 包含，此分支直接 return——最常见的"点开抽屉看内容再按 Escape"序列下 Escape 失效（shell.test.tsx:82
也正因此必须显式 panel.focus() 才能让断言通过，侧面印证了守卫过严）。建议把 active === document.body 与 active === null 也视为域内。

        const active = typeof document !== 'undefined' ? document.activeElement : null;
-       if (active && panelRef.current && !panelRef.current.contains(active)) return;
+       const inScope = active === null || active === document.body
+         || (panelRef.current !== null && panelRef.current.contains(active));
+       if (!inScope) return;


─── packages/views/src/craft/td.tsx:107-109 ───
[bug · low] Drawer 与 Dialog 的 keydown 都注册在 document 上，而 event.stopPropagation()
无法阻止同一元素上注册的其他监听器（那需要 stopImmediatePropagation）。按设计组合（versions.tsx 头注释自称 "the versions drawer
content"，其确认 Dialog 预期渲染在 CraftDrawer children 内），外层 Drawer 的监听先注册、且 contains(activeElement) 对嵌套在自身
DOM 内的 Dialog 焦点判定为真——一次 Escape 会同时触发两层 onClose，确认弹窗与抽屉一起被连锁关闭。建议维护模块级 overlay 栈，Escape 仅由最顶层弹层响应。

+ // 模块级 overlay 栈协调：Escape 仅最顶层弹层响应
+ const overlayStack: symbol[] = [];
+ // effect 内：
+ const id = Symbol('overlay');
+ overlayStack.push(id);
+ // onKeyDown 中：
+ if (overlayStack[overlayStack.length - 1] !== id) return;
+ event.preventDefault();
-       onClose();
+ onClose();
-     };
-     document.addEventListener('keydown', onKeyDown);


─── packages/views/src/craft/td.tsx:154-158 ───
[bug · medium] 弹层声明 role="dialog" + aria-modal="true"，但 useOverlayFocus 只做"打开聚焦 + Escape +
卸载归还"，没有任何 Tab/Shift+Tab 焦点循环：键盘用户可 Tab 逸出到被遮罩的背景内容，读屏器则按 modal 语义隐藏背景——与 shell.tsx:2-3 新注释 "The td
Drawer owns focus trapping" 的声明相悖（原 Sheet 交互契约）。建议在 useOverlayFocus 内实现 Tab
循环（面板内首尾焦点环绕），或至少在落地陷阱前移除 aria-modal 并同步修正 shell.tsx 的契约声明，避免误导。



─── packages/views/src/craft/td.tsx:166-166 ───
[bug · medium] 关闭钮是 div + role="button" + onClick，但无 tabIndex、无 onKeyDown——键盘不可聚焦、不可回车/空格触发，role
与实际行为不符（头注释却声称"维持原 Sheet 的键盘/读屏契约"；Dialog 的 t-dialog__close span 同样问题）。结合 Escape 的域守卫，焦点落到 body
后键盘用户可能没有任何关闭路径。另外 closeLabel 默认硬编码英文 'Close'，而两个调用方（shell.tsx CraftDrawer、versions.tsx
Dialog）都未传入——craft 域全部文案走 zh/en 双语契约，中文界面下读屏会混读英文；aria-label={typeof title === 'string' ? ... :
undefined} 在非字符串标题时还会令弹层失去可访问名称。建议补 tabIndex + 键盘事件，closeLabel 由调用方按 locale 传入。

-         <div className="t-drawer__close-btn" role="button" aria-label={closeLabel} onClick={onClose}><CloseIcon /></div>
+         <div
+           className="t-drawer__close-btn"
+           role="button"
+           tabIndex={0}
+           aria-label={closeLabel}
+           onClick={onClose}
+           onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); onClose(); } }}
+         ><CloseIcon /></div>


─── packages/views/src/integrations/page.tsx:1743-1743 ───
[bug · low] 内联背景由 rgba 改为 `color-mix(in srgb, …)`（L1743 徽标底、L1804 claw 能力卡图标底）且无回退：不识别 color-mix
的旧引擎会整条丢弃内联声明，claw/chrome 徽标与能力卡将失去背景色；且 color-mix 与 #fff 混合在非白底父级上的合成结果与原 rgba 半透明叠加并不等价。2026
年基线浏览器已普遍支持，风险有限，但建议保留 rgba 兜底或确认目标浏览器基线后再移除。



─── packages/views/src/integrations/page.tsx:1915-1917 ───
[documentation · low] 注释与实现不一致：注释写“1em = 14px”，但 SpriteIcon 实际传 `size="15px"`，fallback SVG 仍是
14px——注入 tdesign 图标与未注入回退两条渲染路径尺寸差 1px（wk-vi-110 容器 30px、font-size 14px，注入后图标可能偏大）。建议统一为 14px 并修正注释。



─── apps/web/src/wiki/wiki-reader.css:267-273 ───
[maintainability · medium] 新增段落全部使用硬编码的浅色值（#dcdcdc / #fff / rgba(0,0,0,.9) /
rgba(0,0,0,.4)），与本文件自身的约定不一致：文件头注释明确记录了 `--td-* → --wk-*` 的令牌映射，且既有规则（如 `.wiki-reader-footer` 的
`var(--wk-color-border, #e7e7e7)`、`.wiki-reader-footer-label` 的 `var(--wk-color-text-muted,
rgba(0,0,0,.4))`）都采用「token + 原值兜底」写法。这些硬编码值恰好就是浅色主题下
`--wk-color-border/--wk-color-surface/--wk-color-text/--wk-color-text-placeholder` 的值（见
packages/design-tokens/src/styles.css），而该体系以 `theme-mode="dark"` 作为运行时开关：暗色下 hover/focus 边框会切到
`--td-brand-color` 的暗色值，但输入框仍是白底黑字，呈现混合状态。另外 hover/focus 用的 `var(--td-brand-color)` 按文件头映射应写
`var(--wk-color-brand)`。建议改用「token + 原值兜底」，今天渲染完全不变，且为暗色主题预留正确行为。

  .wk-wiki-search .t-input {
    height: 32px;
    border-radius: 3px;
-   border-color: #dcdcdc;
-   background: #fff;
+   border-color: var(--wk-color-border, #dcdcdc);
+   background: var(--wk-color-surface, #fff);
    font-size: 14px;
  }
+ 
+ /* 对应地：
+    color: var(--wk-color-text, rgba(0, 0, 0, 0.9));
+    placeholder / prefix-icon: var(--wk-color-text-placeholder, rgba(0, 0, 0, 0.4));
+    hover/focus: var(--wk-color-brand, #07c05f);
+ */


─── apps/web/src/wiki/wiki-reader.css:262-265 ───
[maintainability · low] 选择器组存在两处小问题：1) 第一个选择器 `.wk-wiki-search .wk-wiki-search-input.t-input__wrap`
要求自定义类落在 `.t-input__wrap` 上才命中，但 tdesign-react 的 `Input` 会把 `className` 挂到内层 `.t-input`
根元素（本仓库其他样式均按 `.t-input` 或 `.x .t-input__wrap` 结构定位），该选择器实际是死代码，宽度其实一直由第二条生效；2) 第二条
`.wk-wiki-search-input` 未限定在 `.wk-wiki-search` 作用域内，属于全局泄漏（目前该类只在 WikiPage.tsx
的搜索表单里用到一次，风险尚可控，但违背了本文件其余规则的嵌套作用域写法）。改为单一后代选择器即可同时解决两点，且无论 tdesign 把 className 挂在哪个元素都能命中。

- .wk-wiki-search .wk-wiki-search-input.t-input__wrap,
- .wk-wiki-search-input {
+ .wk-wiki-search .wk-wiki-search-input {
    width: 100%;
  }

