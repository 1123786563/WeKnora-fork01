Review partially complete: 160 finding(s); 181 of 304 selected item(s) failed.

─── docs/design/job-search/prototype/app.js:17-17 ───
[security · medium] HTML 转义覆盖不一致：esc() 只应用于 state.prompt、state.notice 与状态 JSON 转储三处插值，而 jobs
数组的所有字段（title/company/city/salary/reason/gap/source/tag）在
jobRow、evidenceCard、workbenchView、journeyView、commandView 等渲染函数中均以裸字符串拼入
root.innerHTML。当前数据为同文件静态常量且 URL 参数（variant/platform）已白名单校验，尚无实际注入面；但该原型是 Issue #140
求职空间后续实现的参照模板，产品红线明确"JD 是不可信数据"，一旦将粘贴的 JD 文本或导入岗位数据填入 jobs（尤其 reason/gap/source 这些本来就是外部 JD
衍生字段），将直接形成 XSS 注入路径。建议将 esc() 作为模板插值的默认行为统一应用到所有动态字段（数字类字段如 fit 可用 Number() 归一），而不是逐点手工判断哪些需要转义。

- const esc = value => String(value).replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char]));
+ // 所有插值统一走 esc()（数字字段用 Number() 归一），例如 jobRow：
+ // '<strong>' + esc(item.title) + '</strong><small>' + esc(item.company) + ' · ' + esc(item.city) + ' · ' + esc(item.salary) + '</small>...'
+ // '<span class="fit-score">' + Number(item.fit) + '<small>%</small></span>'


─── docs/design/job-search/prototype/app.css:9-11 ───
[maintainability · low] 文件尾部"Primary surfaces"追加分段与前段规则形成同选择器重复定义且取值冲突：前段（第 5 行大段）已声明 .mobile/.mini
.details-pane .evidence-card{display:none}，此处改回 display:block（前段规则实际已成死代码）；:root 的 background 由
--td-gray-color-3 被此处覆写为 --td-gray-color-2；.btn.primary 及其
:hover、.prototype-badge、.caption-index、.brand-icon 等主色也被此处整组改写。最终效果完全依赖 CSS
层叠顺序，阅读者必须通读全文件才能推断真实样式，后续若有人只改前段规则会静默被尾段覆盖。建议把尾部取值直接合并进首次声明处并删除覆盖段；若确需保留覆盖式追加，至少在尾段注释中明确说明"以下覆盖块整体改
写前段规则"。

- /* Primary surfaces use the same brand ramp as the TDesign theme. */
- :root{color:var(--td-gray-color-13);background:var(--td-gray-color-2)}
- .prototype-badge,.caption-index,.platform-tabs button.active,.brand-icon,.btn.primary,.assistant-avatar,.composer button{background:var(--td-brand-color-4)}
+ /* 将最终值合并进首段声明，删除尾部覆盖段，例如： */
+ /* :root{font-family:var(--app-font-family);color:var(--td-gray-color-13);background:var(--td-gray-color-2);font-synthesis:none} */
+ /* .mobile .details-pane .evidence-card,.mini .details-pane .evidence-card{display:block;margin-top:12px} */


─── apps/web/src/apps/apps-u.css:187-190 ───
[bug · medium] `.wk-apps-25` 的 `line-height: 5` 是 Tailwind `leading-5` 的错误平移：`leading-5` 生效值为
20px（1.25rem），而无单位的 `line-height: 5` 是 5 倍乘数（5×12px=60px）。该类作用于 AuthorizationPage/ActionPage 的本地 Tag
组件（状态/风险胶囊），元素固定 `height: 20px`，行盒高达 60px 会导致文字垂直溢出 20px 胶囊并挤压相邻内容。同文件其余 line-height 均以 px 为单位（如
.wk-apps-6 的 28px），此处应改为 20px。另外排查发现同类无单位转换错误还存在于其他 -u.css（如 data-sources-u.css:45 的 line-height:
5、experts-u.css:721 的 line-height: 6、knowledge-u.css 的 line-height: 4、kb-u.css 的 line-height:
7/5、platform-u.css:521 的 line-height: 5），建议对全部 -u.css 的无单位 line-height 做一次统一排查修复。

    padding-inline: 4px;
    font-size: 12px;
-   line-height: 5;
+   line-height: 20px;
  }


─── apps/web/src/apps/AppsPages.tsx:80-80 ───
[maintainability · medium] 死代码：ConnectionsPage 迁移到 tdesign-react 的 TPopconfirm 后，此自定义 Popconfirm
组件（含全局 mousedown 监听的 useEffect）在全文件已无任何调用点（搜索确认仅剩第 223 行的 <TPopconfirm>），配套样式
.wk-apps-1~.wk-apps-4（apps-u.css）也随之失效。建议连同组件与这四条样式规则一并删除，避免后续维护时与 TPopconfirm 混淆或被误复用。



─── apps/web/src/apps/AppsPages.tsx:164-164 ───
[performance · low] 每次渲染都通过 catalogColumns(t) 新建列定义数组传入 TTable（ConnectionsPage 组件体内的 columns
字面量同理），columns 引用每次变化会触发 TDesign 表格列配置整体重建。项目内既有 TTable 迁移惯例是用 useMemo 缓存列定义（见
TenantMembersPanel.tsx:667 的 invitationTableColumns），建议保持一致：将 catalogColumns(t) 结果以 t 为依赖 useMemo
缓存，ConnectionsPage 的 columns 视依赖（t/canManage/revokingId）同样处理。



─── apps/web/src/data-sources/ui.tsx:10-13 ───
[bug · high] `.wk-dsui-card` 在 data-sources-u.css 中没有定义，唯一定义位于 knowledge 域的
knowledge/knowledge-u.css:702（与 .wk-cs-card 合并分组）。该 CSS 仅随 KnowledgeGraphPage 的懒加载分块引入，而
DataSourcesPage
的两个挂载入口（KnowledgeSettingsPage.tsx:6、knowledge-bases/KnowledgeBasesPage.tsx:77）的分块均不会传递加载它——用户未访问过知识图
谱页时，KB 设置「数据源」tab 首次打开的整页 Card 都会丢失白底/描边/圆角/内边距。请把 .wk-dsui-card 的定义移入本域 data-sources-u.css（ui.tsx
已导入该文件，可保证随组件加载）。

- export function Card({ children, className, ...props }: HTMLAttributes<HTMLDivElement> & { children?: ReactNode }) {
-   const classes = ['wk-dsui-card', className].filter(Boolean).join(' ');
-   return <section className={classes} {...props}>{children}</section>;
- }
+ /* data-sources-u.css —— 移入本域，替代 knowledge-u.css 中的外域定义 */
+ .wk-dsui-card { border-radius: 8px; border: 1px solid #e7e7e7; background-color: #ffffff; padding: 16px; }


─── apps/web/src/embed/embed-u.css:315-318 ───
[bug · high] 畸形 CSS 声明：`color: color:var(--embed-primary,#2563eb);`
属性名重复，整条声明无效并被浏览器丢弃。后果：引用链接（wk-emb-34 为基础态而非 hover 态）完全丢失 --embed-primary
主题色，回退为浏览器默认链接色（蓝/已访问紫色），在自定义主题色的嵌入场景构成可见视觉回归。同类笔误在本文件共 3
处（.wk-emb-7:hover、.wk-emb-27:hover、.wk-emb-34），且 views-integrations-u.css 也有 3 处 `color:
color:inherit;` 同型错误，建议一并修复。

  .wk-emb-34 {
    overflow-wrap: anywhere;
-   color: color:var(--embed-primary,#2563eb);
+   color: var(--embed-primary, #2563eb);
  }


─── apps/web/src/embed/embed-u.css:71-73 ───
[bug · high] 同 .wk-emb-34：`color: color:var(...)` 属性名重复导致声明被丢弃，"新建会话"按钮 hover 主题色失效（保持 #6b7280 不变）。

  .wk-emb-7:hover {
-   color: color:var(--embed-primary,#2563eb);
+   color: var(--embed-primary, #2563eb);
  }


─── apps/web/src/embed/embed-u.css:257-259 ───
[bug · high] 同 .wk-emb-34：`color: color:var(...)` 属性名重复导致声明被丢弃，"刷新建议问题"与"关闭"按钮的 hover 主题色失效。

  .wk-emb-27:hover {
-   color: color:var(--embed-primary,#2563eb);
+   color: var(--embed-primary, #2563eb);
  }


─── apps/web/src/embed/EmbedEntryPage.tsx:826-826 ───
[bug · medium] 平移遗漏：`underline-offset-2` 是本次全量替换后 EmbedEntryPage.tsx 中唯一残留的 Tailwind 工具类（文件内其余类均已迁移为
wk-emb-*）。S7 目标是移除 embed 产物对 Tailwind 的依赖，一旦移除，该 `text-underline-offset: 2px` 将静默失效。建议移除该类并把声明并入
.wk-emb-34。

-             className="underline-offset-2 wk-emb-34"
+             className="wk-emb-34"
+ /* 并在 embed-u.css 的 .wk-emb-34 中补：text-underline-offset: 2px; */


─── apps/web/src/embed/embed-u.css:324-328 ───
[maintainability · low] 原 Tailwind `break-all` 的等价映射应为 `word-break: break-all`，此处替换为 `overflow-wrap:
anywhere` 语义不等价：break-all 对任意字符无条件断行（普通单词也会被拆断），anywhere 仅在单词无法整行容纳时才断行并影响 min-content 尺寸。长
URL/长字符串标题的折行表现会改变，且该行为变更未加注释。另外同文件 .wk-emb-22（break-words）正确映射为 `overflow-wrap:
break-word`，三处断行策略不一致。若确有意改为 anywhere（仓库其它域确有此约定），请补注释说明意图并统一策略。

  .wk-emb-35 {
    cursor: pointer;
-   overflow-wrap: anywhere;
+   word-break: break-all;
    color: #1f2329;
  }


─── apps/web/src/embed/embed-u.css:30-30 ───
[style · low] 可维护性瑕疵（不影响功能）：1) 本行与第 298 行（.wk-emb-31 内 `border-top-width: 1px;border-color:
#e7eaef;`）两条声明挤在同一行，与文件其余一声明一行的格式不一致；2) .wk-emb-36 的 sr-only 替代使用了已废弃的 `clip: rect(0,0,0,0)`，现代写法为
`clip-path: inset(50%)`（Tailwind 3.3+ 同款）；3) 文件末尾的全局 `@keyframes pulse` 与 views-chat-u.css
中已有的两份相同定义（第 640、2364 行）在 embed 产物中将共存三份，内容暂同无实害，但同名全局动画按加载顺序互相覆盖，建议后续收敛为单一来源。

-   border-bottom-width: 1px;border-color: #eef1f5;
+   border-bottom-width: 1px;
+   border-color: #eef1f5;


─── apps/web/src/data-sources/DataSourcesPage.tsx:0-0 ───
[bug · medium] 设置字段的 `required={!field.optional}` 在迁移中被移除，而 save() 的 JS
校验只覆盖了凭据字段（firstMissingRequiredCredential）、name/type/schedule 与 gitlab
projectId——VUE_SETTINGS_FIELDS 中 rss 的 feed_urls 是必填字段（无 optional 标记）却无任何兜底校验，用户现在可以创建空 feed URL 的
RSS 数据源直达服务端。建议仿照 firstMissingRequiredCredential 增加 settings 字段的必填 walk，或在 save() 中补一条等价校验。

- VUE_SETTINGS_FIELDS[form.type] ? VUE_SETTINGS_FIELDS[form.type].map((field) => <label key={field.key} className="wk-ds-29"><span>{t(field.label)}</span><Textarea autosize={{ minRows: 3, maxRows: 3 }}
+ // save() 内、gitlab 校验之后：
+ const missingSetting = firstMissingRequiredSetting(form.type, form.settingsText);
+ if (missingSetting) {
+   setMessage({ tone: 'warning', text: `${t(missingSetting)} ${t('dataSource.isRequired')}` });
+   return;
+ }
+ 
+ // form.ts：仿 firstMissingRequiredCredential 遍历 VUE_SETTINGS_FIELDS 跳过 optional 字段


─── apps/web/src/data-sources/DataSourcesPage.tsx:474-475 ───
[bug · low] 删除确认 Dialog 在提交期间已禁用 cancel 按钮与 purge checkbox，但 TDesign Dialog 的 closeOnEscKeydown 默认为
true，右上角 closeBtn 也未随 deleteSubmitting 禁用——ESC 或点击 X 仍会触发 onClose
关闭对话框，绕过「提交中锁定」的意图（关闭后若立即重开同一源并再次确认，还可能触发重复删除请求）。建议提交期间同时关闭 ESC 与关闭图标。

      cancelBtn={{ content: t('common.cancel'), disabled: deleteSubmitting }}
      closeOnOverlayClick={false}
+     closeOnEscKeydown={!deleteSubmitting}
+     closeBtn={!deleteSubmitting}


─── apps/web/src/data-sources/DataSourcesPage.tsx:451-451 ───
[maintainability · low] 迁移残留：`className=""` 空字符串属性应删除（filter(Boolean) 之外的无意义透传），保持代码整洁。

- {editing && credentialStep.replaceMode ? <Button type="button" theme="default" variant="text" size="small" className="" onClick={() => { setCredentialStep((current) => credentialStepReducer(current, 'cancel-replace')); setForm((current) => ({ ...current, credentialsText: '', authHeaders: [] })); }}>{t('common.cancel')}</Button> : null}
+ <Button type="button" theme="default" variant="text" size="small" onClick={() => { setCredentialStep((current) => credentialStepReducer(current, 'cancel-replace'));


─── apps/web/src/data-sources/data-sources-u.css:43-45 ───
[bug · medium] line-height 换算错误：原 Tailwind `text-xs leading-5` 中 leading-5 = 1.25rem（20px
定值），被误转为无单位的 `line-height: 5`（即 5×字号 ≈ 60px）。连接器类型选择卡片（创建流程第一步）的描述文本将以约 60px 行高渲染，撑破 min-height:
92px 的卡片布局。应改为 1.25rem。

  .wk-ds-3 {
    font-size: 12px;
-   line-height: 5;
+   line-height: 1.25rem;


─── apps/web/src/data-sources/data-sources-u.css:27-29 ───
[maintainability · low] 平移产物中存在多处重复声明：.wk-ds-2 与 .wk-ds-42 的 transition-duration 150ms 后紧跟
200ms；.wk-ds-25/.wk-ds-42/.wk-ds-68 的 border-style solid 后紧跟 dashed；.wk-ds-58/.wk-ds-71 的
`border-bottom-width: 1px;border-color: #e7e7e7;` 挤在同一行。虽然层叠后生效值（200ms/dashed）与原 utility
覆盖语义一致，但重复声明易被误读为笔误或被后续维护者误删其一。建议统一只保留最终生效值并规范单行单声明的格式。

    transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
-   transition-duration: 150ms;
    transition-duration: 200ms;


─── apps/web/src/configuration/config-u.css:44-51 ───
[bug · medium] `.wk-cfg-ops-4 input` / `.wk-cfg-ops-4 textarea` 后代选择器是旧原生控件时代的 utility 平移，但该表单内现已是
tdesign Input/Textarea（渲染 `.t-input`/`.t-textarea` 包裹层 + 内层原生元素）。config-u.css 为非 layered
CSS，`.wk-cfg-ops-4 input`（特异性 0,1,1）会反超 tdesign 自身的 `.t-input__inner`（0,1,0），把
border/padding/background 直接叠到内层元素上，与包裹层边框形成双重边框并破坏 tdesign
控件内距。项目内已有明确先例：settings-wrapper.css:658-662/738-744 对同类 `[&_input]` 规则注明为死样式且"不移植"，仅保留
width/height；且本域编辑器侧已改用 `.wk-cfg-field`（作用于组件根）方案，两处方案不一致。建议参照 settings
域先例，删除内层边框/内距/底色声明，仅保留尺寸，或统一改用 `.wk-cfg-field` 类挂到 tdesign 组件根上。

- .wk-cfg-ops-4 input {
+ .wk-cfg-ops-4 input,
+ .wk-cfg-ops-4 textarea {
+   /* tdesign 控件自带边框/内距（.t-input__inner 等），
+    * 参照 settings-wrapper.css McpSettingsPanel 的处理仅保留尺寸，避免双重边框。 */
    width: 100%;
    box-sizing: border-box;
-   border-style: solid;
-   border-width: 1px;
-   border-color: #cbd5e1;
-   border-radius: 6px;
-   background-color: #fff;
+ }


─── apps/web/src/configuration/config-u.css:258-269 ───
[maintainability · low] .wk-cfg-page-16 同一规则内存在被 !important 覆盖的死声明：`font-size: 0.8rem` 被 `font-size:
0.85rem !important` 覆盖、`color: #6941c6` 被 `color: rgba(0, 0, 0, 0.6) !important` 覆盖。虽为旧 Tailwind 重复
utility 的忠实平移，但平移后已无法像 utilities 源码那样一眼看出层叠关系，保留死代码会误导后续维护。同类问题：`.wk-cfg-ed-5` 在 `display: flex
!important` 下保留了无效的 `grid-template-columns: auto 1fr`。建议删除无效声明。

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


─── apps/web/src/configuration/ConfigurationOperations.tsx:64-64 ───
[bug · low] tdesign-react Input 的只读 prop 为 `readonly`（小写），此处沿用 React DOM 命名 `readOnly`。若 tdesign
Input 未把该 rest prop 透传到内层原生 input，"Viewer (read-only)" 字段将变为可编辑。同一迁移中密码框的 `autoComplete` 已刻意改为小写
`autocomplete`（LoginPage 及其测试证实小写透传有效），此处命名不一致。建议统一改为 `readonly` 并实际验证内层 input 带上 readonly 属性。

-   return <Card className="wk-configuration-operations"><h2>Agent sharing and visibility</h2><p className="wk-muted wk-cfg-ops-1">Select an agent here for sharing or hide/show operations. To choose an agent for a conversation, use the Agent selector in the chat entry. Shared agents are read-only in the receiving workspace, so every share uses Viewer permission. Sharing and hide/show preferences remain server-authorized.</p>{error ? <Status tone="error">{error}</Status> : null}{notice ? <Status tone="success">{notice}</Status> : null}<div className="wk-form-grid wk-form-grid--two wk-cfg-ops-2"><label>Selected agent<Select value={selected} onChange={(value) => setSelected(String(value))}><Select.Option value="" label="No agent">No agent</Select.Option>{agents.map((agent) => <Select.Option key={agent.id} value={agent.id} label={`${agent.name}${disabledIds.includes(agent.id) ? ' · disabled' : ''}`}>{agent.name}{disabledIds.includes(agent.id) ? ' · disabled' : ''}</Select.Option>)}</Select></label><label>Organization ID<Input value={organizationId} onChange={(value) => setOrganizationId(String(value))} placeholder="org_…" /></label><label>Share permission<Input value="Viewer (read-only)" readOnly /></label></div><div className="wk-list-actions wk-cfg-ops-3"><Button type="button" theme="default" variant="outline" disabled={busy || !selected} onClick={() => void setDisabled(!selectedDisabled)}>{selectedDisabled ? 'Show selected agent' : 'Hide selected agent'}</Button><Button type="button" theme="default" variant="outline" loading={busy} disabled={!selected || !organizationId.trim()} onClick={() => void share()}>Share agent</Button></div></Card>;
+ <label>Share permission<Input value="Viewer (read-only)" readonly /></label>


─── apps/web/src/configuration/ConfigurationEditor.tsx:28-30 ───
[maintainability · low] 新增校验文案 'Fill in the required fields.' 为硬编码英文，而同域 ConfigurationPage 已在用 i18n
机制（createTranslator/useAppLocale，错误回退用 t('common.error')），非英文 locale 下会出现中英混排。建议复用域内 translator
输出该文案。

      if (!draft.name.trim() || (section === 'models' && (!draft.type?.trim() || !draft.source?.trim()))) {
-       setError('Fill in the required fields.'); setBusy(false); return;
+       setError(t('configuration.fillRequiredFields')); setBusy(false); return;
      }


─── apps/web/src/configuration/ConfigurationEditor.tsx:28-30 ───
[test · low] save() 新增的必填守卫是本次迁移中唯一新增的行为逻辑（tdesign Input 不透传 required
后的等价替代），但域内测试文件（ConfigurationPage.test.ts / ConfigurationOperations.test.tsx）均未随本次变更更新：空 name、models
缺 type/source 时应提示且不发起 API 调用的关键路径无任何回归保障。建议补充针对该守卫的用例。



─── apps/web/src/configuration/ConfigurationOperations.tsx:146-146 ───
[test · low] 本次迁移改变了组件渲染结构与交互接线（onChange 从 event.target.value 改为 value 直传、Select.Option 替代原生
option、Button 换 tdesign），但既有测试未随变更更新：1) ConfigurationOperations.test.tsx 的 SSR 断言（如
`/>Stop</`）依赖文本节点紧邻标签边界，需确认 tdesign Button 渲染后仍通过；2) 全仓无其他 `Select.Option value=""` 空字符串选项先例（其余域均用
options prop 且无空值），"No agent/No model" 选项在 tdesign Select 下的显示与回传行为无验证。建议补测或确认既有用例在迁移后仍绿。



─── apps/web/src/career/SearchPage.tsx:175-176 ───
[bug · medium] send() 的最终兜底分支（确定性失败且未命中前面任何分支，如 unauthorized、not_found、或带 4xx status 的错误如 429）执行
`setError; setPhase('idle')` 但未清除 attempt。此时 textarea 与提交按钮因 `attempt !== undefined`
被禁用；「开始新的一次找岗」仅在 `attempt && phase === 'terminal'` 时渲染；错误卡只给 revision_conflict /
search_quota_refused 提供动作按钮；「结果暂时未知」卡又要求 phase ===
'unknown'——四条恢复路径全部不可达，界面进入无出口的死状态，用户只能刷新页面。与上一分支（invalid_request 等）已 `setAttempt(undefined)`
的做法保持一致即可。

     if (isUncertainOutcome(cause)) { setPhase('unknown'); setNotice(''); return }
-    setError(parsed); setPhase('idle'); setNotice('')
+    setError(parsed); setPhase('idle'); setAttempt(undefined); setNotice('')


─── apps/web/src/career/OpportunityPage.tsx:82-83 ───
[bug · low] acceptReceipt 以 throw TypeError 表达「回执编号不匹配」，但 throw 发生在同一 try 块内，会被本函数的 catch
捕获：errorDetails 取不到 code、无 status，isUncertainWrite 判为
true，于是把一个确定性的协议错误（服务端返回了别人的回执）误归类为「结果暂时未知」，引导用户走回执查询/原编号重试的恢复流程。同组的 PreparationPage/ProgressPage
已建立 ReceiptMismatchError 分类模式并在 catch 中以 `cause instanceof ReceiptMismatchError`
走确定失败分支，本文件应保持一致（runEvaluation/lookupEvaluationReceipt 的「回执与固定快照不匹配」TypeError 同理）。另：本文件评估按钮的
onClick/文案存在 3-4 层嵌套三元，建议拆为具名处理函数与标签映射。

+ class ReceiptMismatchError extends Error {}
   const acceptReceipt = (next: OpportunityReceipt, expected: Attempt): void => {
-   if (!sameReceipt(next, expected)) throw new TypeError('服务返回的请求编号与本次导入不匹配')
+   if (!sameReceipt(next, expected)) throw new ReceiptMismatchError('服务返回的请求编号与本次导入不匹配')
+   // catch 中在 isUncertainWrite 之前增加：
+   // if (cause instanceof ReceiptMismatchError) { setState('error'); setMessage(`导入未完成：${cause.message}`); return }


─── apps/web/src/career/SearchPage.tsx:5-6 ───
[maintainability · low] 对同一模块 '../../../../packages/api-client/src/career.ts' 存在两条相邻的 import
语句，应合并为一条。另外，newRequestId/makeId、errorDetails、isUncertainWrite 在本组 5
个页面（Career/Opportunity/Preparation/Progress/Search）及
Application/Inbox/Material/Submission/ExportDeletion 中共重复定义约 10 份，且「确定性错误码清单」已出现分叉（如本文件的
isUncertainOutcome 含 search_quota_refused、OpportunityPage 的清单不含
revision_conflict），后续新增错误码极易漏改，建议抽取共享的 career 请求工具模块统一维护。

- import type { SearchFailureCode, SearchOnceReceipt, SearchQualification, SearchUncertainty, SearchResultRow } from '../../../../packages/api-client/src/career.ts'
- import type { OpportunityURLImportReceipt } from '../../../../packages/api-client/src/career.ts'
+ import type { OpportunityURLImportReceipt, SearchFailureCode, SearchOnceReceipt, SearchQualification, SearchUncertainty, SearchResultRow } from '../../../../packages/api-client/src/career.ts'


─── apps/web/src/career/SearchPage.tsx:141-143 ───
[maintainability · low] acceptReceipt（及 openHistoryEntry 的 not_found 分支）在 setHistoryEntries 的
updater 函数内部执行 writeHistory 写 localStorage。State updater 必须是纯函数：StrictMode 开发模式会双调用、并发特性下也可能重放
updater，副作用应移出更新器（例如用 useEffect 订阅 historyEntries 变化后持久化，或先基于当前 state 计算再统一 set +
write）。当前写入幂等所以无可见损害，但属于 React 反模式。

     const merged = [entry, ...current.filter((item) => item.searchId !== next.searchId)].slice(0, 8)
-    writeHistory(typeof window === 'undefined' ? undefined : window.localStorage, storageKey, merged)
     return merged
+   // writeHistory 移到 updater 之外（如 useEffect([historyEntries]) 中持久化）


─── apps/web/src/career/CareerPage.tsx:176-176 ───
[style · low] 本页大面积使用内联 style（maxWidth/padding/grid 模板，以及 #666、#087a55、#e7e7e7、#eee 等硬编码颜色），而同目录其余
career 页面均已采用「类名 + CSS 文件」方案（opportunity.css/preparation.css/progress.css/search.css/inbox.css
等，且统一走 --td-* token）。内联样式无法随主题 token 调整、也难以做响应式收敛，建议抽到 career.css 并复用既有 token 变量。

-  return <main className="wk-page wk-page--std" style={{ maxWidth: 1040, margin: '0 auto', padding: '24px 20px' }}>
+  return <main className="wk-page wk-page--std wk-career">
+ /* career.css: .wk-career { max-width: 1040px; margin: 0 auto; padding: 24px 20px; } */


─── apps/web/src/career/preparation.css:67-72 ───
[maintainability · low] 品牌绿 #07c05f 在 preparation.css/progress.css/search.css 中以裸字面量反复硬编码（本文件 10+
处），而 design-tokens 已提供 --wk-color-brand 且 tdesign-theme.css 已将 --td-brand-color 映射为
#07c05f——本文件其他规则也都在用 var(--td-*, fallback) 形式，唯独品牌色绕过了 token，主题调整时会漏改。这里的 !important 只是为了压过前面的
`.wk-preparation__actions button` 通用规则，建议调整选择器优先级（如提高特异性）而非 !important。

- .wk-preparation__submit,
- .wk-preparation__revise {
-   border-color: #07c05f !important;
-   background: #07c05f !important;
-   color: #fff !important;
+ .wk-preparation__actions .wk-preparation__submit,
+ .wk-preparation__actions .wk-preparation__revise {
+   border-color: var(--td-brand-color, #07c05f);
+   background: var(--td-brand-color, #07c05f);
+   color: #fff;
  }


─── apps/web/src/career/preparation.css:102-108 ───
[bug · low] `--td-bg-color-secondary-container`（secondary 与 container 之间多了连字符）在 tdesign-theme.css
中并未定义——那里定义的是 `--td-bg-color-secondarycontainer`（opportunity.css 用的正是这个正确名称）。该变量永远解析不到，背景恒为 fallback
#f5f5f5，主题切换时不会跟随 token 变化。

- .wk-preparation__sources {
-   margin: 10px 0 0;
-   padding: 10px 12px;
-   border: 1px dashed var(--td-component-border, #dcdcdc);
-   border-radius: 8px;
-   background: var(--td-bg-color-secondary-container, #f5f5f5);
- }
+   background: var(--td-bg-color-secondarycontainer, #f5f5f5);


─── apps/web/src/experts/experts-u.css:717-722 ───
[bug · medium] `.wk-exp-37` 的 line-height 换算错误：原 utility 为 `leading-6`（Tailwind v4 =
calc(var(--spacing) * 6) = 24px，项目未覆盖 --spacing）。迁移后写成无单位的 `line-height: 6`，按 CSS 规范这是 6 倍行高乘数（16px
× 6 = 96px），下线发布确认对话框（unpublish dialog）的标题行框会被撑到 96px，产生大块异常留白。应改为 `line-height: 24px`。

  .wk-exp-37 {
    margin-bottom: 8px;
    font-size: 16px;
    font-weight: 600;
-   line-height: 6;
+   line-height: 24px;
  }


─── apps/web/src/experts/ExpertsPage.tsx:46-47 ───
[maintainability · low] 新增的 `import '../chat/views-ui.css'`（views-chat-u.css）在本页没有生效对象：ExpertsPage
自身的类全部由 experts-u.css 覆盖（wk-exp-*），`wk-page` 全仓库无基类样式定义（仅作 DOM 钩子/PlatformShell
覆盖点），`wk-experts-persona` 全仓库无 CSS 定义；`renderChatMarkdown` 的输出不携带任何 class，且 views-chat-u.css
中无裸元素选择器（已检索确认）。该导入会把 ~2500 行 chat 域样式打进 experts 路由 chunk，并引入无依据的跨域耦合。若确有依赖（如后续 persona 共用 chat
样式），建议加注释说明用途；否则应删除。

- import '../chat/views-chat-u.css';
  import './experts-u.css';


─── apps/web/src/apps/AppsPages.tsx:111-112 ───
[bug · medium] 目录表/安装表的标识列（action_id、app_id、app_version、provider 及 app_key、version）cell 用 appShort()
截断到 18 字符且无 title 提示：由于 JS 已预先截断，单元格不会发生 CSS 溢出，TTable `ellipsis: true` 的溢出 tooltip
永远不会触发，完整标识在本页彻底不可见。两处事实基线均提供全值查看：AppsView.vue 这些列直接渲染原值（依赖 t-table ellipsis 悬停查看全值），被替换的旧 React
实现也在每个 TableCell 上带 `title={String(value ?? '')}`。action_id
是操作详情路由（/platform/apps/action/:id）的键，截断后无法核对全值，属信息可见性回退。建议与 Vue 一致去掉 appShort 截断、交由 ellipsis+tooltip
呈现，或保留截断但给这几列补 title 提示。

-     { colKey: 'action_id', title: t('apps.catalog.colAction'), ellipsis: true, cell: ({ row }: { row: AppRow }) => appShort(row.action_id) },
-     { colKey: 'app_id', title: t('apps.catalog.colApp'), ellipsis: true, cell: ({ row }: { row: AppRow }) => appShort(row.app_id) },
+     { colKey: 'action_id', title: t('apps.catalog.colAction'), ellipsis: true },
+     { colKey: 'app_id', title: t('apps.catalog.colApp'), ellipsis: true },
+     /* 或保留截断但补 title：cell: ({ row }: { row: AppRow }) => <span title={String(row.action_id ?? '')}>{appShort(row.action_id)}</span> */


─── apps/web/src/data-sources/DataSourcesPage.tsx:429-429 ───
[bug · high] 编辑 Drawer 未传 `footer={false}`：tdesign-react 的 Drawer 默认渲染「取消/确认」页脚（FAQPage.tsx:1718
注释明确提到「默认 footer 的确认按钮」；仓库内其余
TDrawer——IntegrationsPage.tsx:68、KnowledgeBaseActivityPanel.tsx:128、ResourceSettingsPanel.tsx:993——均
显式 footer={false}）。后果：表单自带「保存」按钮下方会多出一排默认按钮——「确认」没有 onConfirm 回调是死按钮，「取消」会触发 onClose →
setEditing(undefined) 直接丢弃用户已填写的表单草稿。建议补 footer={false}（或像 FAQPage:1379 一样以自定义 footer 承载保存动作）。

-   const editorSurface = editing === undefined ? null : <Drawer visible header={editorTitle} onClose={() => setEditing(undefined)} size="640px" placement="right" className="wk-data-source-drawer">
+   const editorSurface = editing === undefined ? null : <Drawer visible header={editorTitle} footer={false} onClose={() => setEditing(undefined)} size="640px" placement="right" className="wk-data-source-drawer">


─── apps/web/src/data-sources/DataSourcesPage.tsx:441-441 ───
[bug · medium] `wk-ds-self-start` 在本域 data-sources-u.css 中没有定义，唯一定义位于 knowledge 域的
knowledge/knowledge-u.css:698，而该 CSS 仅随 KnowledgeGraphPage 的懒加载分块引入（与已确认问题 #6 的分块结论一致）。用户未访问过知识图谱页时
`justify-self: start` 丢失：链接作为 grid item 被拉伸为整列宽度，可点击/下划线区域扩大到整行，与旧栈 `justify-self-start` 视觉不一致。建议在本域
CSS 中补定义（或下沉到共享 utilities 层），消除跨域依赖。

-           {guide.permissionPageUrl ? <a className="wk-ds-15 wk-ds-self-start" href={guide.permissionPageUrl} target="_blank" rel="noopener">{prereqCopy(t, `dataSource.prereqOpenConsole_${form.type}`, 'dataSource.prereqOpenConsole')}<span aria-hidden="true">↗</span></a> : null}
+ .wk-ds-self-start { justify-self: start; }


─── apps/web/src/data-sources/DataSourcesPage.tsx:456-456 ───
[maintainability · low] 嵌套 label：tdesign-react Checkbox 的根元素本身就是 `<label>`（内部已含 input +
`t-checkbox__label`），外层再包 `<label>` 形成 label 嵌套，HTML 规范不允许，且 Checkbox 已通过 `label` prop 渲染文案，外层 label
冗余（辅助技术的标签计算可能出现歧义）。同一模式也出现在删除对话框的 purge 复选框处。建议去掉外层 `<label>`。

- <label><Checkbox checked={form.deletions} onChange={(checked) => updateForm('deletions', checked)} label={t('dataSource.syncDeletions')} /></label>
+       <Checkbox checked={form.deletions} onChange={(checked) => updateForm('deletions', checked)} label={t('dataSource.syncDeletions')} /><Button type="submit" theme="default" variant="outline" loading={saving}>{t('dataSource.save')}</Button>


─── apps/web/src/data-sources/DataSourcesPage.tsx:484-484 ───
[maintainability · low] 与同步删除项同款问题：tdesign-react Checkbox 根元素本身是 `<label>`，外层再包 `<label
className="wk-ds-21">` 构成无效的 label 嵌套，且 Checkbox 已自带 label 文案，外层 label 冗余。如需保留 wk-ds-21 布局可改用
`<div>`。

-       <label className="wk-ds-21"><Checkbox checked={deletePurge} disabled={deleteSubmitting} onChange={(checked) => setDeletePurge(checked)} label={purgeLabelText} /></label>
+       <div className="wk-ds-21"><Checkbox checked={deletePurge} disabled={deleteSubmitting} onChange={(checked) => setDeletePurge(checked)} label={purgeLabelText} /></div>


─── apps/web/src/faq/FAQPage.tsx:2214-2216 ───
[security · high] canManage 权限映射错误且失败分支未复位。Vue 事实源中 canManage 是独立于 canEdit 的
computed（FAQEntryManager.vue:986-993 → orgStore.canManageKB，organization.ts:818-822 仅 owner/admin 返回
true），而 canEdit 才包含 editor；此处却把 canManage 直接等于 canContribute（computeKBPermissions 的 canContribute 含
editor/成员写权限）。结果是普通 editor 也会看到批量删除（Popconfirm）、卡片三点菜单（删除入口）和设置齿轮——UI 层破坏性操作的门控从 owner/admin
放宽到了所有可贡献者，与平移目标（FAQPageViewProps 注释自称"Vue canManage（三点菜单/批量删除门槛）"）相悖。另外 `.catch(() => {
setCanContribute(false); ... })` 分支只复位了 canContribute：KB 设置拉取失败时（如切换 KB 后瞬时失败）canManage 会残留上一个库的
true。建议在 permissions.ts 增加独立的 owner/admin 判定（或扩展 KBPermissions 返回 canManage），并在 catch 分支同步
setCanManage(false)。

        const permissions = computeKBPermissions(kbRow as KBSurfaceKB, me as KBSurfaceMe | null);
        setCanContribute(permissions.canContribute);
-       setCanManage(permissions.canContribute);
+       // Vue canManage = owner/admin（orgStore.canManageKB），非 canEdit/canContribute
+       setCanManage(computeKBManagePermissions(kbRow as KBSurfaceKB, me as KBSurfaceMe | null));
+     }).catch(() => { if (active) { setCanContribute(false); setCanManage(false); setFaqGate('allowed'); } });


─── apps/web/src/faq/FAQPage.tsx:1501-1503 ───
[bug · medium] onKeydown 回调参数疑似与 tdesign-react 签名不符，Ctrl+Enter 添加答案可能静默失效。tdesign-react 的
Input/Textarea onKeydown 签名是 (value, context: { e })——第一个参数是输入值而非 context。此处 `(context as { e?:
KeyboardEvent }).e` 若首参实际是 string，则 .e 恒为 undefined，Ctrl/Cmd+Enter 添加答案永远不触发；FAQTagManageDialog 两处
`onKeydown={(context) => ...Escape...}`（行 1973/1988）同理，Escape 取消也将失效。旧实现基于原生 textarea onKeyDown
是可用的，属平移回归。防御性双转义的写法本身就说明类型对不上，请对照所用版本 tdesign-react 的类型声明确认，并改为使用第二个参数（或解构 { e }）。

-                           onKeydown={(context) => {
-                             const event = (context as { e?: KeyboardEvent }).e;
+                           onKeydown={(_value, { e: event }) => {
                              if (event && (event.ctrlKey || event.metaKey) && event.key === 'Enter') {
+                               event.preventDefault();
+                               addAnswer();
+                             }
+                           }}


─── apps/web/src/faq/FAQPage.tsx:1147-1151 ───
[bug · medium] 新建按钮 Tooltip>Dropdown 与导出按钮 Dropdown>Tooltip 的嵌套顺序相反，两处至少有一处 tooltip 无法正常工作。TDesign 的
Tooltip/Dropdown 都通过 cloneElement 把 ref 与 hover/click 事件注入直接子元素：当子元素是 Dropdown 这类组件（非 DOM）时，事件与 ref
大概率不会被转发到真实触发节点，外层 Tooltip 不会显示；而导出按钮的 Dropdown>Tooltip>Button 顺序才是可行写法。建议两处统一为 Dropdown 在外、Tooltip
包住 Button（或直接用 TDesign Popup/Dropdown 的触发器 tooltip 能力），并实测 tooltip 是否出现。

-                     <Tooltip content={t('knowledgeEditor.faq.createGroup')} placement="top">
-                       <Dropdown options={faqCreateOptions} trigger="click" placement="bottom-right" onClick={(item) => handleFaqAction((item as { value?: unknown }).value)}>
+                     <Dropdown options={faqCreateOptions} trigger="click" placement="bottom-right" onClick={(item) => handleFaqAction((item as { value?: unknown }).value)}>
+                       <Tooltip content={t('knowledgeEditor.faq.createGroup')} placement="top">
                          <Button variant="text" theme="default" className="content-bar-icon-btn" size="small" icon={<TIcon name="add" size="16px" />} />
-                       </Dropdown>
-                     </Tooltip>
+                       </Tooltip>
+                     </Dropdown>


─── apps/web/src/faq/FAQPage.tsx:2176-2181 ───
[maintainability · low] message 状态改为仅驱动 toast 后，FAQPageView 的 message 与 editorTitle 两个 prop 已成死
prop：新 JSX 删除了全部 Status/错误块渲染（导入弹窗、编辑抽屉的错误提示均已移除），Drawer header 也改用 editorMode 三元，视图内解构出的
message/editorTitle（行 785/790）不再被任何渲染读取，但 FAQPage
仍在传（message={message}、editorTitle={...}）。留着会误导后续维护者以为视图还有内联错误渲染路径；同时该 effect 内 theme 选择也是一层嵌套三元。建议从
FAQPageViewProps 中移除这两个 prop 及其传参，theme 映射改为查表对象。

-   // Vue MessagePlugin 语义：message 状态变化即 toast（页面 DOM 无内联错误块）。
    useEffect(() => {
      if (!message) return;
-     const theme = message.tone === 'error' ? 'error' : message.tone === 'success' ? 'success' : 'warning';
-     MessagePlugin[theme](message.text);
+     const themes = { error: 'error', success: 'success', warning: 'warning' } as const;
+     MessagePlugin[themes[message.tone]](message.text);
    }, [message]);


─── apps/web/src/faq/FAQPage.tsx:275-280 ───
[bug · medium] FaqTagTooltip 丢失了点击切换与 role="tooltip"，触屏设备无法查看被截断内容。旧实现带 `onClick={(event) => {
event.stopPropagation(); setOpen((current) => !current); }}` 和气泡 role="tooltip"（parity
任务为指针/触屏补充的能力）；新版仅保留 hover（onMouseEnter/Leave），且气泡 div 不再声明
role。问答/答案标签普遍超长截断，平板与触屏用户将完全没有途径查看完整文本。建议恢复 onClick 切换与 role="tooltip"。

      <div
        ref={wrapperRef}
        className="faq-tag-wrapper"
        onMouseEnter={() => setOpen(true)}
        onMouseLeave={() => setOpen(false)}
+       onClick={(event) => { event.stopPropagation(); setOpen((current) => !current); }}
      >


─── apps/web/src/faq/FAQPage.tsx:1558-1559 ───
[bug · medium] 无障碍/触屏可达性回归，三处叠加：1) 导入与批量标签弹层由旧版的 role="dialog" + aria-modal="true" + aria-label 降级为裸
div，读屏用户失去对话框语义与 Esc 之外的语义关闭线索；2) 标签筛选清除按钮仅在 tagFilterTriggerHover 为 true
时渲染（showTagFilterClear），触屏设备无 hover，无法清除标签筛选；3) 搜索框丢失了旧版的 aria-label。建议恢复
role/aria-modal/aria-label，清除按钮改为常驻（或同时在 focus 时显示）。

-           <div className="faq-import-overlay" onClick={(event) => { if (event.target === event.currentTarget) onCloseImport(); }}>
+           <div className="faq-import-overlay" role="dialog" aria-modal="true" aria-label={t('knowledgeEditor.faqImport.title')} onClick={(event) => { if (event.target === event.currentTarget) onCloseImport(); }}>
              <div className="faq-import-modal">


─── apps/web/src/faq/FAQPage.tsx:620-620 ───
[style · low] 多处嵌套三元表达式违反团队规则（"Nested ternary expressions are not allowed"）：此处 importTask
图标名为三层嵌套（running/success/failed/默认）；导入弹窗底部按钮文案（importTask?.status === 'success' ? close : failed ?
retry : importButton，行 1659）、activeTagFilterLabel（行 956-960）也是两层嵌套。建议改为映射表或提取具名函数（如
faqImportIconName(status) / faqImportActionText(status)），可读性更好也便于单测。

-               name={importTask.status === 'running' ? 'loading' : importTask.status === 'success' ? 'check-circle-filled' : importTask.status === 'failed' ? 'error-circle-filled' : 'time-filled'}
+               name={faqImportStatusIcon(importTask.status)}
+ // 模块级：
+ // function faqImportStatusIcon(status: string) {
+ //   const map: Record<string, string> = { running: 'loading', success: 'check-circle-filled', failed: 'error-circle-filled' };
+ //   return map[status] ?? 'time-filled';
+ // }


─── apps/web/src/faq/FAQPage.tsx:2340-2341 ───
[documentation · low] 注释与实现不符：注释声称"updateFAQEntryTagBatch + 回滚"，但实现既无乐观更新也无任何回滚（失败仅
toast），previousTagId 只用于同值短路。与 Vue 事实源（FAQEntryManager.vue:1444-1463）的真实差异是：Vue 成功后还会 `await
loadTags(true)` 刷新标签的 chunk_count，而这里只
load(false)——单卡片改标签后，标签筛选面板/下拉里的条目计数会保持陈旧直到整页刷新。建议修正注释，并补上标签列表刷新（或说明为何可省略）。

-   // Vue handleEntryTagChange — 单卡片改标签（updateFAQEntryTagBatch + 回滚）。
+   // Vue handleEntryTagChange（FAQEntryManager.vue:1444-1463）— 单卡片改标签，
+   // 成功后刷新条目与标签（Vue 侧 loadTags(true) 同步 chunk_count）。
    async function updateEntryTag(entryId: number, tagSeqId: string) {


─── apps/web/src/faq/faq.td.css:2240-2244 ───
[bug · medium] 非法 CSS：`var(--td-brand-color)1a` / `var(--td-brand-color)33` 在 var()
函数后直接拼接两位十六进制透明度不是合法语法（应是 Vue less 端
fade()/暗色拼接的转换错误），浏览器会整条丢弃这两个声明——检索结果里答案标签（.answer-tag）的背景与边框色将完全失效，只剩继承的默认样式。仓库内其它处已使用 color-mix
写法（如 .tag-filter-chip.active），建议改为 color-mix(in srgb, var(--td-brand-color) 10%, transparent) 与 20%
同款，或定义带透明度的独立 token。

  .answer-tag {
-   background: var(--td-brand-color)1a;
+   background: color-mix(in srgb, var(--td-brand-color) 10%, transparent);
    color: var(--td-brand-color);
-   border-color: var(--td-brand-color)33;
+   border-color: color-mix(in srgb, var(--td-brand-color) 20%, transparent);
  }


─── apps/web/src/faq/faq.td.css:3275-3279 ───
[maintainability · medium] §C 共享弹层类与其它域 CSS 全量重复定义，靠 bundle 加载顺序互相竞争。.popup-menu-item 在
agents.td.css:66、kb-list.td.css:66、orgs.td.css:59
已有三份全局定义，这里是第四份；.tag-filter-popup（documents.td.css:2075）与
.kb-switcher-card/.kb-switcher-row（documents.td.css:2599 起）也与本文件逐条重复。这些弹层均 portal 到
body、无法靠页面作用域隔离，同名规则冲突时以加载顺序定胜负——任一域后续微调（间距、圆角、z-index）都会静默波及其余域的弹层。注释自称"与 documents.td.css
同源平移"，建议真正抽成共享样式文件（如 shared/dropdown-menu.css）统一引入，或给各类名加域前缀。



─── apps/web/src/faq/faq.td.css:43-46 ───
[maintainability · low] 成片 Vue 残留死代码与重复声明块：1) Vue 过渡类
.fade-*、.modal-enter/leave-*、.slide-down-*、.faq-batch-bar-fade-* 在 React 中永不生成（无 Transition
组件挂这些类名）；2)
.tag-menu、.faq-header-meta、.faq-meta-item、.empty-tip、.tag-filter-bar、.tag-load-more、.match-type-tag、
.kb-info-card-ext、.status-item（非 compact）等类在 FAQPage.tsx 中无任何引用（已逐一检索确认）；3) .faq-editor-drawer
.full-width-input-wrapper .add-item-btn（1726/1737）与 .faq-search-drawer
.question-tag（2670/2680）各有一对连续同选择器声明块应合并；4) .faq-manager .faq-header h2 { font-size: 24px }
会被同一元素上更高特异性的 .faq-breadcrumb（20px）恒覆盖，形同虚设。约 300+ 行无效规则会持续抬高维护与排查成本，建议清理。

- .fade-enter-active,
- .fade-leave-active {
-   transition: opacity 0.15s ease;
- }
+ /* 删除 .fade-*、.modal-*、.slide-down-*、.faq-batch-bar-fade-*、.tag-menu、.faq-header-meta、
+  * .faq-meta-item、.empty-tip、.tag-filter-bar、.tag-load-more、.match-type-tag、
+  * .kb-info-card-ext、.status-item 等 React 侧无引用的 Vue 残留规则 */


─── apps/web/src/faq/faq.td.css:3248-3250 ───
[maintainability · low] 弹层层级体系未统一且依赖 !important 强压：card-more-popup 被强制 z-index 99（低于 TDesign Popup
默认 6000+ 层、也低于 .faq-import-overlay/.batch-tag-overlay 的 1000 与 .faq-tag-tooltip 的
9999——若卡片菜单与这些层同屏，菜单会被压在下层），tag-filter-popup 5500、faq-import-panel 200、faq-batch-bar-anchor
6，各值间缺少统一的层级约定；同时 §B/§C 大量使用 !important 覆盖组件库默认样式（add-item-btn、t-popup__content、t-input 系列等），后续升级
tdesign 版本时排查成本高。建议定义一组层级 CSS 变量（--z-popup/--z-drawer/--z-toast 等）统一取值，并尽量用更高特异性选择器替代 !important。

  .card-more-popup {
-   z-index: 99 !important;
+   /* 与其它弹层统一层级变量，避免低于 overlay(1000)/tooltip(9999) 被遮挡 */
+   z-index: var(--z-popup, 6000) !important;
  }


─── apps/web/src/configuration/ui.tsx:20-20 ───
[style · low] toneClass 的计算是三层嵌套三元表达式（error→success→warning→''），项目 Code Quality 规则明确禁止嵌套三元。这里本质是
tone→class 的映射，用查表对象更直观且便于后续扩展 tone 变体：如 `const toneClasses: Record<string, string> = { error:
'wk-cfg-status--error', success: 'wk-cfg-status--success', warning: 'wk-cfg-status--warning' };
const toneClass = toneClasses[tone] ?? '';`

-   const toneClass = tone === 'error' ? 'wk-cfg-status--error' : tone === 'success' ? 'wk-cfg-status--success' : tone === 'warning' ? 'wk-cfg-status--warning' : '';
+   const toneClasses: Record<StatusTone, string> = { neutral: '', error: 'wk-cfg-status--error', success: 'wk-cfg-status--success', warning: 'wk-cfg-status--warning' };
+   const toneClass = toneClasses[tone];


─── apps/web/src/integrations/views-integrations-u.css:39-44 ───
[bug · medium] 非法 CSS 声明 `color: color:inherit;`（属性名重复，系 text-[color:inherit]
误译），浏览器解析时会整条丢弃。同样的错误还出现在 .wk-vi-channel-card-static-class 与 .wk-vi-channel-card-title-add-class
两处。当前因 color 本身默认继承、卡片为 article/span 无 UA 色干扰，视觉暂未变化，但显式 inherit 的意图已丢失，一旦置于按钮/链接等 UA
色上下文或未来主题变量介入即会偏离。应改为 `color: inherit;`。

  .wk-vi-channel-card-clickable-class {
    width: 100%;
    cursor: pointer;
    background-color: #ffffff;
-   color: color:inherit;
+   color: inherit;
  }
+ /* .wk-vi-channel-card-static-class / .wk-vi-channel-card-title-add-class 两处同步改为 color: inherit; */


─── apps/web/src/integrations/integrations.td.css:19-21 ───
[maintainability · medium] 本节选择器与 ApiPlaygroundDrawer.tsx
现状脱节：`.wk-api-playground-aside`/`-body`/`-section`(+>h4)/`-error`/`-step-label`/`-status[data-status
]` 在 JSX 中已无任何元素挂载（aside 现为 wk-apd-3、容器 wk-apd-4、section wk-apd-5、状态色 wk-apd-14..17、错误 p 为
wk-apd-9），全仓库搜索仅本文件命中，属迁移残留的死规则，并与 integrations-u.css 的 wk-apd-* 形成双源。建议整段删除，仅保留仍被引用的
.wk-api-playground-pre（preClassName 仍指向它）。

- .wk-api-playground-overlay .wk-api-playground-aside { overflow: auto; height: 100vh; }
- .wk-api-playground-overlay .wk-api-playground-body { display: grid; gap: 18px; }
- .wk-api-playground-overlay .wk-api-playground-section {
+ /* 删除 -aside/-body/-section/-section>h4/-error/-step-label/-status[data-status] 死规则
+  * （JSX 已改用 wk-apd-3/4/5/9/14..17）；保留 live 的 .wk-api-playground-pre。 */


─── apps/web/src/integrations/integrations.td.css:33-33 ───
[bug · medium] 该规则特异性 (0,2,0) 高于 integrations-u.css 的 .wk-apd-2 (0,1,0)，且全局 --color-muted:
#66758b（styles.css:432），导致抽屉内所有 wk-muted 文本实际渲染 #66758b，而非 wk-apd-2 注释声明的 Vue
--td-text-color-secondary=rgba(0,0,0,.6) 映射——两处单源互相矛盾，实际生效的是旧栈色值。同理 `.wk-api-playground-overlay
.wk-button { color: var(--color-ink,#172033) }` 覆盖 wk-apd-13 的 rgba(0,0,0,.9)（无 !important）。建议与
wk-apd-* 收敛为单源：删除本节重复的 .wk-muted/.wk-button 族（或对齐为 rgba(0,0,0,.6)/rgba(0,0,0,.9)）。

- .wk-api-playground-overlay .wk-muted { color: var(--color-muted, #66758b); }
+ /* 删除本行，muted 色由 integrations-u.css .wk-apd-2 统一承载（rgba(0,0,0,.6)）
+  * 或改值为 color: rgba(0, 0, 0, 0.6); 与 wk-apd-2 对齐 */


─── apps/web/src/integrations/integrations.td.css:86-93 ───
[maintainability · medium] 第 3 节整个 .wk-embed-preview-*
家族（overlay/drawer/header/body/device/chrome/screen，约 70 行）已无任何 JSX 挂载点：EmbedPreviewModal.tsx 现用
wk-epm-1..15，views page.tsx 的 EmbedChannelPreviewPanel 现用 wk-vi-13..28，且
embedWizardRender.test.tsx:253 / embedPreviewFallback.test.tsx:116 注释明确写着"replaces the former
.wk-embed-preview-drawer class hook"。整节为按旧类名回填的死代码，应删除以避免与 wk-epm-*/wk-vi-* 双源漂移。

- .wk-embed-preview-overlay {
-   position: fixed;
-   inset: 0;
-   z-index: 1300;
-   display: flex;
-   justify-content: flex-end;
-   background: rgba(0, 0, 0, 0.5);
- }
+ /* 删除第 3 节 .wk-embed-preview-* 全部规则：现由 integrations-u.css .wk-epm-1..15
+  * 与 views-integrations-u.css .wk-vi-13..28 承载，旧类名无元素引用。 */


─── apps/web/src/integrations/views-integrations-u.css:2217-2233 ───
[maintainability · low] 同一规则内同名属性先后两次声明：font-size: 13px / line-height: 18px 随后被
font-size/line-height: inherit 覆盖。这是按 Tailwind 类串顺序机械展开的产物——虽然原 [font:inherit] 简写在构建中确实胜出（CHIP_BASE
注释改用长属性正说明简写会压过
text-[13px]），当前生效值未变，但双声明使真实生效值不可读，任何后续重排或"顺手去重"都会静默翻转字号。同类问题：.wk-vi-91（14px→inherit）、.wk-vi-137（12p
x !important 与非 important inherit 混用）、.wk-apd-13/.wk-vi-104 的重复
border-style、.wk-vi-channel-card-class 的重复 transition-duration。建议每条规则只保留生效值并注明来源。

  .wk-vi-160 {
    display: block;
    width: 100%;
    cursor: pointer;
    border-style: solid;
    border-width: 0;
    background-color: transparent;
    padding-inline: 10px;
    padding-block: 6px;
    text-align: left;
-   font-size: 13px;
-   line-height: 18px;
+   /* 原 [font:inherit] 简写胜出 → 仅保留生效值 inherit */
    font-family: inherit;
    font-size: inherit;
    line-height: inherit;
    font-weight: inherit;
  }


─── packages/views/src/integrations/page.tsx:1916-1918 ───
[bug · low] JumpIcon 三处口径不一致：注释写 "1em = 14px"，SpriteIcon 传 size="15px"（生产路径，注入渲染器后生效），fallback SVG
仍是 14px（views 包直渲染/未注入时生效）。注册与回退两条渲染路径尺寸不同，且注释与实现矛盾。若 15px 是实测 Vue 对齐值，应同步 fallback 与注释；否则统一为 14px。

  function JumpIcon() {
-   return <SpriteIcon name="jump" size="15px" fallback={<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" aria-hidden="true"><path d="M9 4L4 4L4 20L20 20L20 15" /><path d="M19.25 4.75L12 12M14 4H20L20 10" /></svg>} />;
+   return <SpriteIcon name="jump" size="14px" fallback={<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" aria-hidden="true"><path d="M9 4L4 4L4 20L20 20L20 15" /><path d="M19.25 4.75L12 12M14 4H20L20 10" /></svg>} />;
  }


─── packages/views/src/integrations/page.tsx:1768-1770 ───
[maintainability · low] `{false && tab === 'cli' ...}` 及下文 chrome/claw 两处同款永假分支属死代码（条件恒为
false，无任何保留意图标注），且本次迁移还在逐行改写其内部 className（wk-vi-121/122/123/128..132），持续为不可达代码付出维护成本并干扰类名完整性核对。建议连同
`{false && tab === 'chrome' ...}`、`{false && tab === 'claw' ...}` 三段一并删除。



─── packages/views/src/integrations/page.tsx:1900-1902 ───
[maintainability · low] `LANDING_ICON_TDESIGN_NAMES[name] ?? name` 会把未映射的 key 原样透传给 TIcon：apps/web
注入渲染器后 SpriteIcon 不再走 fallback，tdesign-icons 无该名称时图标整片空白，且 views 包内测试（回退手绘 path）发现不了。当前 chrome
(qa/clip/notes/shortcuts)、claw (upload/url/manual/search/browse) 及 code/extension
恰好全部有映射，属潜伏陷阱。建议未映射 key 直接走手绘 fallback（或至少显式白名单断言）。

  function LandingIcon({ name, size = 14 }: { name: string; size?: number }) {
-   return <SpriteIcon name={LANDING_ICON_TDESIGN_NAMES[name] ?? name} size={`${size}px`} fallback={<svg viewBox="0 0 24 24" width={size} height={size} fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" strokeLinejoin="miter" aria-hidden="true">{LANDING_ICON_PATHS[name] ?? null}</svg>} />;
+   const mapped = LANDING_ICON_TDESIGN_NAMES[name];
+   const handDrawn = <svg viewBox="0 0 24 24" width={size} height={size} fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" strokeLinejoin="miter" aria-hidden="true">{LANDING_ICON_PATHS[name] ?? null}</svg>;
+   // 未映射 key 直接走手绘兜底，避免注入渲染器后透传无效图标名渲染空白
+   return mapped ? <SpriteIcon name={mapped} size={`${size}px`} fallback={handDrawn} /> : handDrawn;
  }


─── apps/web/src/integrations/ApiPlaygroundDrawer.tsx:317-317 ───
[style · low] statusTag 使用三层嵌套三元选择 className，违反团队"禁止嵌套三元表达式"规范，且与 data-status
状态键重复罗列。建议改为查表映射，既扁平也便于与 integrations.td.css 的 [data-status] 规则对照维护。

-     <span className={status === 'success' ? 'wk-apd-14' : status === 'failed' ? 'wk-apd-15' : status === 'stopped' ? 'wk-apd-16' : 'wk-apd-17'} data-status={status || 'none'}>{status || '-'}</span>
+ const PLAYGROUND_STATUS_CLASS: Record<NonNullable<PlaygroundStepStatus>, string> = {
+   success: 'wk-apd-14',
+   failed: 'wk-apd-15',
+   stopped: 'wk-apd-16',
+ };
+ const statusTag = (status: PlaygroundStepStatus) => (
+   <span className={status ? PLAYGROUND_STATUS_CLASS[status] : 'wk-apd-17'} data-status={status || 'none'}>{status || '-'}</span>
+ );


─── packages/design-tokens/src/styles.css:19-23 ───
[maintainability · low] @import 位置违反 CSS 规范（@import 必须先于除 @charset/@layer
之外的所有规则）。当前消费方（apps/web、apps/embed）均经 Vite/postcss-import 构建期内联，位置不影响结果；但若任一消费方将来以原生 CSS 方式加载本文件（未经
postcss-import 处理，如直接 link 或某些 SSR/内联工具链），该 @import 会被浏览器静默忽略，整套 --td-* 主题变量（含暗色分支）全局失效且无任何报错。建议将
@import 移至文件最顶部（:root 规则之前）：tdesign-theme.css 内部为 :root:root 高优先级选择器且变量互不冲突，前移不影响现有级联结果，可彻底消除该隐患。

- /* TDesign 组件主题（Vue frontend/src/assets/theme/theme.css 原样平移，Phase 1 Task 4）。
-    注意：消费方若将本文件整体 layer() 引入（如 apps/web layer(theme)），这里的
-    --td-* 覆盖会输给 unlayered 的 tdesign.css 默认值——apps/web/src/styles.css 在
-    tdesign.css 之后另有对本文件的不带 layer() 直引，确保覆盖生效。 */
  @import "./tdesign-theme.css";
+ /* Generated from packages/design-tokens/src/index.ts. Vue theme-mode is the
+    runtime switch used by the React parity shell. */
+ :root {


─── apps/web/src/main.tsx:20-20 ───
[maintainability · low] career 域其余页面
CSS（application/inbox/material/preparation/progress/rule/search/submission/export-deletion.css
等）均在各自页面组件内按页引入，唯独 opportunity.css 在应用根全局引入（全仓库仅此一处引用，OpportunityPage.tsx 自身未
import）。引入策略不一致带来两个问题：① opportunity.css 被无条件加载进所有路由的初始包，未与页面一同按需加载；②
后续维护者按其余页面的惯例补一次按页引入即形成重复样式入口。建议移入 OpportunityPage.tsx 顶部，与其他 career 页面对齐。

- import './career/opportunity.css';
+ // main.tsx 删除此行；在 apps/web/src/career/OpportunityPage.tsx 顶部按页引入：
+ // import './opportunity.css';


─── apps/web/src/tdesign-icon-offline.ts:8-9 ───
[maintainability · low] 拦截 URL 硬编码单一版本 0.4.5，与 tdesign-icons-react@0.6.11
内部地址强耦合。依赖当前虽被精确锁定，但一旦升级且库内 CDN 版本号变化，querySelector 匹配失败、守卫静默失效，离线/内网/合规部署环境将恢复对 tdesign.gtimg.com
的运行时外联请求（隐私与可用性风险），且无任何告警。Vue 端对位实现（frontend/src/utils/tdesign-icon-offline.ts）以多版本（0.4.0–0.4.4）构造
URL 提升升级韧性，React 侧未沿用。建议二选一：① 与 Vue 端一致改为按版本数组生成候选 URL；② 增加守护测试——从已安装的 tdesign-icons-react 源码中提取其引用的
icon 版本号，断言与这里拦截的版本一致，升级漂移时测试即失败提醒同步更新。



─── apps/web/src/knowledge/knowledge-u.css:291-293 ───
[bug · high] codemod 误译（同文件已修正 grid repeat 与 border-2 两处，漏掉此处及 .wk-kg-42/.wk-kg-55 共三处）：源码 Tailwind
`leading-4` = `line-height: 1rem`（16px），而无单位 `line-height: 4` 是 4× 字号乘数（12px 字号 → 48px 行高）。帮助弹窗的 7
行双列说明行距将被撑到约 3 倍，属可见渲染缺陷。

    gap: 12px;
    font-size: 12px;
-   line-height: 4;
+   line-height: 1rem;


─── apps/web/src/knowledge/knowledge-u.css:452-454 ───
[bug · high] 同样是 `leading-4` 的 codemod 误译：应为 1rem（16px），无单位 4 = 4×
字号（12px→48px），图例状态卡主行（graphStatusCard.primary）行高错误。

    white-space: nowrap;
    font-size: 12px;
-   line-height: 4;
+   line-height: 1rem;


─── apps/web/src/knowledge/knowledge-u.css:583-585 ───
[bug · high] 第三处 `leading-4` 误译：抽屉邻居提示（drawerNeighborHint）12px 文本配 48px 行高，与 Vue 版 16px 行高明显不一致。

    user-select: none;
    font-size: 12px;
-   line-height: 4;
+   line-height: 1rem;


─── apps/web/src/knowledge/knowledge-u.css:700-702 ───
[bug · high] 跨域错置导致样式不可达：`.wk-cs-card` 只被 commercial/surface.tsx 使用、`.wk-dsui-card` 只被
data-sources/ui.tsx 使用，而这两个文件只导入各自域 CSS（commercial-u.css / data-sources-u.css，均未定义这两个类）；本文件仅被懒加载的
KnowledgeGraphPage 导入（router.tsx:67 lazy，无任何 @import 链）。结果：未访问过知识图谱页时，商业与数据源页面的 Card
壳（边框/圆角/内边距）整体缺失。参照同型做法——configuration 域的 `.wk-cfg-card` 定义在自己的 config-u.css:408——应将 `.wk-cs-card`
移入 commercial-u.css、`.wk-dsui-card` 移入 data-sources-u.css，并从本文件删除。



─── apps/web/src/knowledge/knowledge-u.css:325-328 ───
[maintainability · low] codemod 叠加残留：源码是 `transition-all duration-300`，两段 duration 均被落盘，150ms 是死声明（被
300ms 覆盖）。仅保留 300ms 即可。

    transition-property: all;
    transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
-   transition-duration: 150ms;
    transition-duration: 300ms;


─── apps/web/src/knowledge-settings/KnowledgeSettingsPage.css:736-739 ───
[bug · medium] 键盘焦点反馈缺失：原生 checkbox 已视觉隐藏（1px/clip），可见盒由 .kb-checkbox-input 承担，但整个文件没有
`input:focus-visible + .kb-checkbox-input` 的焦点描边规则（styles.css 亦无全局 focus-visible 兜底）。键盘用户 Tab
聚焦向量/向量+全文索引复选框时无任何可视焦点指示，相对原生 checkbox 的默认焦点环是可访问性回退。

  .indexing-check-head input:checked + .kb-checkbox-input {
    border-color: var(--wk-color-brand, #07c05f);
    background-color: var(--wk-color-brand, #07c05f);
+ }
+ .indexing-check-head input:focus-visible + .kb-checkbox-input {
+   outline: 2px solid var(--wk-color-brand, #07c05f);
+   outline-offset: 2px;
  }


─── apps/web/src/knowledge-settings/KnowledgeSettingsPage.css:831-831 ───
[maintainability · low] `border-color: none` 是非法值（合法值须为颜色或 `transparent`/`currentColor`
等），整条声明会被浏览器丢弃；且内层 input 已 `border: 0`，此 focus 规则实际不起任何作用，属死规则，建议直接删除，避免误导后续维护者以为这里承载了焦点样式。

- .kb-text-input:focus { outline: none; border-color: none; }
+ /* 删除该规则：内层 input 无边框，焦点描边由 .kb-text-input-wrap:focus-within 承担 */


─── apps/web/src/knowledge-settings/parserSettings.tsx:226-230 ───
[bug · low] 可访问名称丢失：data-parser-group 迁到包裹 span 后相关测试选择器已同步（5 个测试文件均按新结构查询），但 span 无 role，其
aria-label 不会为 TDesign Select 内部真实的 input 提供可访问名称——读屏用户无法得知各解析器组（pdf/office/…）选择框的用途，而原 `<select
aria-label>` 是有效的。建议将 aria-label 直接传给 `<Select>`（若 tdesign 透传至内部 input），至少给包裹 span 补 role="group"
使标签可被朗读。

                <span
                  data-parser-group={group.key}
+                 role="group"
                  aria-label={group.label}
                  style={{ display: 'block' }}
                >


─── apps/web/src/knowledge-settings/GraphSettings.tsx:5-5 ───
[documentation · low] 换装映射注释自指无信息量：`theme="primary" → theme="primary"` 前后完全相同，未表达旧栈→TDesign
的实际差异（对照本文件代码，应为旧 ui.Button 的 `variant="primary"` → TDesign `theme="primary"`），会误导后续按 playbook
迁移的读者以为 theme 属性直译不变。已确认该错误注释仅此一处，未被批量复制。

- // variant="text" 直译、theme="primary" → theme="primary"）；Status 无
+ // variant="text" 直译、variant="primary" → theme="primary"）；Status 无


─── apps/web/src/knowledge-bases/kb-u.css:109-109 ───
[bug · high] `top: calc(100%+4px)` 是无效 CSS——calc 中 `+` 两侧必须有空白，整条 top
声明会被浏览器丢弃，KnowledgeBaseActivityPanel 的筛选下拉菜单（.wk-kba-2）回落到静态定位、直接压在表头上。这是原 Tailwind 任意值
`top-[calc(100%+4px)]` 手工平移为原生 CSS 时未补空格的系统性误译（同文件 `.wk-kbs-25` 的 `calc(100vw-2rem)` 同题）。

-   top: calc(100%+4px);
+   top: calc(100% + 4px);


─── apps/web/src/knowledge-bases/kb-u.css:675-675 ───
[bug · high] `max-width: calc(100vw-2rem)` 同样因缺空格整条无效——KnowledgeBaseShareDialog 的
TDialog（dialogClassName="wk-kbs-25"）在窄视口失去 max-width 兜底，`width: 520px !important`
固定宽会横向溢出屏幕，移动端不可关闭/操作。

-   max-width: calc(100vw-2rem);
+   max-width: calc(100vw - 2rem);


─── apps/web/src/knowledge-bases/kb-u.css:13-17 ───
[bug · high] `line-height: 7` 是 Tailwind `leading-7` 的误译：leading-7 = 1.75rem = 28px，这里却成了无单位倍数（7 ×
20px 字号 = 140px 行盒），编辑器留守段所有 h3 标题行高被拉到
140px，布局严重变形。同题共三处：`.wk-kbl-2`、`.wk-kba-5`（活动面板标题）、`.wk-kbs-23`（共享 inline 面板标题），应统一改为 28px；建议顺带全局
grep 本次其它 *-u.css 是否存在同批 codemod 误译（如 `line-height: 5;`）。

    /* Vue KnowledgeBaseEditorModal.vue .section-title color: var(--td-text-color-primary) */
    margin: 0;
    font-size: 20px;
    font-weight: 600;
-   line-height: 7;
+   line-height: 28px;


─── apps/web/src/knowledge-bases/kb-u.css:88-92 ───
[bug · medium] `line-height: 5` 是 `leading-5`（20px）的误译。当前表格元数据计数器显示正常纯属偶然：kb-list.td.css 里同名
`.kb-editor-desc-count { line-height: 20px }`（同特异性、后加载）把它兜住了——两处定义任意一处被删或导入顺序变化即回归。建议本类直接改为 20px，并把
`.kb-editor-desc-count` 在 kb-list.td.css 与 kb-editor-parity.css 的双重定义收敛为单处。

  .wk-kbl-13 {
    font-size: 12px;
-   line-height: 5;
+   line-height: 20px;
    color: var(--color-text-placeholder);
  }


─── apps/web/src/knowledge-bases/kb-list.td.css:1601-1606 ───
[bug · medium] `.t-dialog__position.t-dialog--top { padding-top: 40vh !important; }`
是无任何作用域限定的全局规则且带 ！important：KB 列表页 chunk 一旦加载即常驻，全站所有默认 top 布局的 TDesign Dialog（含本页
KnowledgeBaseShareDialog 的 TDialog、以及设置页等其它页面的默认弹窗）都会被整体下压 40vh。该规则在 agents.td.css / orgs.td.css
已各有一份，本文件是第三份复制。建议改用 TDesign Dialog 的 `top="40vh"` 属性作用于 del-knowledge-dialog 单个实例，删除全局规则。

- /* Vue :deep(.t-dialog__position.t-dialog--top)：删除确认弹窗上移（40vh）。
-    弹层渲染在 body 下，Vue scoped 穿透规则在 portal DOM 上等效全局——按
-    agents.td.css 同源处理原样落全局（同 pilot 先例）。 */
- .t-dialog__position.t-dialog--top {
-   padding-top: 40vh !important;
- }
+ /* 删除整条全局规则；在 KnowledgeBasesPage.tsx 的删除确认 Dialog 上直接传 top */
+ <Dialog visible={deletingKb !== null} top="40vh" dialogClassName="del-knowledge-dialog" ...>


─── apps/web/src/knowledge-bases/kb-list.td.css:1877-1880 ───
[maintainability · medium] `.settings-modal` / `.settings-overlay` 是无页面前缀的全局类，本文件取
max-width:1000px、z-index:1000，而 orgs.td.css 的同名全局类取 max-width:1100px、z-index:2000，agents.td.css 又以
`.settings-overlay .settings-modal`（1100px）参与叠加。SPA 内多个页面 CSS 挂载后按 chunk 导入顺序全局互相覆盖——KB 编辑器按 Vue 平移的
1000px 几何会被其它页面悄无声息改写（反之亦然）。另外 `@keyframes spin` 已在 documents-u.css / platform-u.css /
本文件三处重名。建议：本文件规则至少统一挂在 `.settings-overlay` 前缀下（agents 先例），并把跨页共享块（popup-menu / dropdown-menu 子集）抽到公共
CSS，在迁移台账记一笔归并项。

- .settings-modal {
+ .settings-overlay .settings-modal {
    position: relative;
    width: 90vw;
    max-width: 1000px;


─── apps/web/src/knowledge-bases/kb-list.td.css:2429-2430 ───
[maintainability · medium] 本段起的 `wk-kb-activity-*`、`wk-kb-share-*`、`wk-kb-des-*` 三大段（约至文件尾，~400
行）在全部 TSX/TS 中无任何生产者——组件实际使用的是 kb-u.css 的 wk-kba-* / wk-kbs-* / wk-kbl-* 前缀类。同文件内
`.create-kb-dialog`、`@keyframes dropdownSlideInUp`、`shared-detail-drawer-enter/leave-*`
过渡类（自绘抽屉瞬间卸载，DOM 上永远不会出现这些类）同样无消费方。这批死样式与生效的 kb-u.css
同语义并存，后续维护者改这里的值会发现「不生效」。建议整段删除（`shared-detail-drawer-enter*` 与 agents.td.css 中的同款死块一并处理）。



─── apps/web/src/knowledge-bases/kb-editor-parity.css:21-24 ───
[maintainability · low] `label.grid` 两条规则的消费方已随本批收编清零：留守段 vectorStore/parser/storage 的 label 已改用
wk-kbl-*，共享的 ChunkingSettingsFields / GraphSettings 中也无 `className="grid"` 的 label——规则成为死代码。同时
`.kb-editor-content h3 + p { margin-top: 6px }` 的特异性 (0,2,2) 高于 `.wk-kbl-3` 的
(0,1,0)，留守段标题间距实际始终由这条旧规则决定、wk-kbl-3 里的 `margin-top: 4px` 永远不生效——同一间距存在两个互相打架的事实源，本文件按注释「待
knowledge-settings 批次一并清理」，建议至少在注释中标注 h3+p 当前胜出，避免后续有人「修」wk-kbl-3 却无效。

- /* 留守段字段行（vectorStore/parser/storage 的 label.grid）：label 15px/21 +
-    8px 间距（Vue form-label mb8 语义）。 */
- .wk-kb-editor-dialog .kb-editor-content label.grid { row-gap: 8px; line-height: 21px; }
- .wk-kb-editor-dialog .kb-editor-content label.grid > span { line-height: 21px; }
+ /* label.grid 两条规则删除（留守段 label 已收编为 wk-kbl-*，无生产者）。
+  * 标题间距事实源：本文件 h3 + p { margin-top: 6px }（特异性高于 .wk-kbl-3）。 */


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:822-823 ───
[maintainability · medium] editorActivity / editorActivityLoading 两个 state 只写不读：活动段实际渲染的是自带数据获取的
KnowledgeBaseActivityPanel，二者无任何渲染消费点，loadEditorActivity 成为死函数；但侧栏 nav 的 onClick 仍会在每次点击 activity
项时触发一次 `client.knowledgeBases.settings.activity` 冗余网络请求（与面板自身的加载叠加为双请求）。建议删除两个
state、loadEditorActivity 及 onClick 中的调用。

-   const [editorActivity, setEditorActivity] = useState<Array<{ id: number; action: string; outcome: string; created_at: string }>>([]);
-   const [editorActivityLoading, setEditorActivityLoading] = useState(false);
+ // 删除 editorActivity / editorActivityLoading 两个 state、loadEditorActivity 函数，
+ // 以及侧栏 nav-item onClick 中的 `if (section === 'activity' && editingId) void loadEditorActivity(editingId)`；
+ // 活动数据由 KnowledgeBaseActivityPanel 挂载时自行加载。


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:28-28 ───
[maintainability · low] `isSharedKbEditable` 导入后文件内无任何使用（已核实 apps/web/tsconfig.json 未开
noUnusedLocals，不会阻断构建，但属死导入，会触发 lint 噪音并误导后续读者以为存在共享可编辑判断）。建议从 import 中移除。



─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:257-260 ───
[style · low] 嵌套三元违反项目「禁止嵌套三元表达式」约定，且同类分支在 KbCard 头部再次出现（isSpaceCard ? … : isSharedCard ? … :
…），两处后续维护都易插错分支。建议提取为早返回的辅助函数。

-   const text = variant === 'mine'
-     ? ORIGIN_TEXT.mine[locale] ?? '我创建'
-     : variant === 'creator' ? (creatorName || tenantText)
-       : tenantText;
+ function resolveOriginText(variant: 'mine' | 'tenant' | 'creator', creatorName: string | undefined, locale: Locale): string {
+   if (variant === 'mine') return ORIGIN_TEXT.mine[locale] ?? '我创建';
+   if (variant === 'creator') return creatorName || (ORIGIN_TEXT.tenant[locale] ?? '本空间');
+   return ORIGIN_TEXT.tenant[locale] ?? '本空间';
+ }


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:1614-1615 ───
[bug · low] 删除确认弹窗用 `<span onClick>` 充当「取消 / 删除」按钮：无 tabindex、无键盘事件处理、无焦点管理，键盘用户完全无法完成删除确认（Tab
也进不去）。建议换成 `<button type="button">`，沿用同一 class（circle-btn-txt 样式为纯文本样式，button 需补
border/background/padding 重置）。

-             <span className="circle-btn-txt" onClick={() => { if (!deleting) setDeletingKb(null); }}>{t('common.cancel')}</span>
-             <span className="circle-btn-txt confirm" onClick={() => { void confirmDelete(); }}>{t('knowledgeList.delete.confirmButton')}</span>
+             <button type="button" className="circle-btn-txt" onClick={() => { if (!deleting) setDeletingKb(null); }}>{t('common.cancel')}</button>
+             <button type="button" className="circle-btn-txt confirm" onClick={() => { void confirmDelete(); }}>{t('knowledgeList.delete.confirmButton')}</button>


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:1073-1078 ───
[bug · low] 在 setState updater 内执行 localStorage 写副作用：React 要求 updater 是纯函数，StrictMode 双调用 /
并发特性下的重放会重复执行该副作用（此处幂等才侥幸无害）。另外同文件 `useRef(createDeleteGuard(client))` 每次渲染都会执行一次
createDeleteGuard（仅首个结果被保留），若该构造非轻量可改为惰性初始化。建议 favorites 持久化移到 `useEffect(() => { … },
[favorites])`。

    const toggleFavorite = (kbId: string) => {
      setFavorites((current) => {
        const next = new Set(current);
        if (next.has(kbId)) next.delete(kbId);
        else next.add(kbId);
-       try { window.localStorage.setItem('wk-kb-favorites', JSON.stringify([...next])); } catch { /* storage unavailable */ }
+       return next;
+     });
+   };
+ 
+   useEffect(() => {
+     try { window.localStorage.setItem('wk-kb-favorites', JSON.stringify([...favorites])); } catch { /* storage unavailable */ }
+   }, [favorites]);


─── apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx:217-217 ───
[maintainability · low] 本文件新增并导出了 Vue 同构版 SpaceAvatar（SPACE_GRADIENTS 色板 + 装饰 SVG），而同屏的
KnowledgeBaseShareDialog 及列表项仍引用 organizations/SpaceAvatar.tsx
旧实现——两份实现的渐变色板、尺寸阶梯、装饰图形均不同，同屏并排渲染视觉不一致且后续必然漂移。文件头对 UPLOAD_SVG 已记「Phase 4 归并」，SpaceAvatar
建议同样在迁移台账明确归并时点，或本页直接复用 organizations 版避免双实现。



─── apps/web/src/knowledge-bases/SharedKnowledgeBaseDrawer.tsx:102-102 ───
[bug · low] 从旧栈 Sheet 换成自绘 overlay 后，丢失了 Esc 关闭与焦点圈定：键盘用户打开共享详情抽屉后无法用 Escape 关闭（原 Sheet 有），只能 Tab 到
footer 按钮或点遮罩。建议补一个 keydown Escape 监听，并考虑打开时聚焦关闭按钮 / 关闭时归还焦点。

-     <div className="shared-detail-drawer-overlay" onClick={(event) => { if (event.target === event.currentTarget) onClose(); }}>
+   useEffect(() => {
+     if (!visible) return;
+     const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
+     window.addEventListener('keydown', onKey);
+     return () => { window.removeEventListener('keydown', onKey); };
+   }, [visible, onClose]);


─── apps/web/src/knowledge-bases/SharedKnowledgeBaseDrawer.tsx:54-58 ───
[maintainability · low] permissionTone「兼容旧名」导出在全库无任何消费方（原调用点已随本次迁移改为
permissionTheme），属死代码，建议直接删除，避免留下两套语义（tone/theme）并存的双入口。



─── apps/web/src/organizations/OrganizationsPage.tsx:1868-1868 ───
[bug · low] 迁移到 TDesign 组件时控件 id 丢失，label 的 htmlFor 悬空：本行 TInput 仅有 name="join-code"（旧实现为
id="join-code" name="join-code"），上方 label htmlFor="join-code" 失去关联；同类回归共 4 处——join-search 的
TInput（label htmlFor="join-search"）、upgrade-role 与 join-request-role 的 TSelect、create 模式的 name
输入框（TInput 仅 name="organization-name"，label htmlFor="organization-name"）。点击 label 无法聚焦输入框，读屏
label/控件关联断开。TDesign Input/Select 均会将多余 props 透传到根元素，补回 id 即可。

-                         <TInput name="join-code" className={ORG_FIELD + ' wk-org-6'} value={joinInputCode} maxlength={32} placeholder={t(locale, 'organization.inviteCodePlaceholder')} onChange={(value) => setJoinInputCode(String(value))} onKeydown={(_, context) => { if (context.e.key === 'Enter') void doPreviewFromInput(); }} />
+                         <TInput id="join-code" name="join-code" className={ORG_FIELD + ' wk-org-6'} value={joinInputCode} maxlength={32} placeholder={t(locale, 'organization.inviteCodePlaceholder')} onChange={(value) => setJoinInputCode(String(value))} onKeydown={(_, context) => { if (context.e.key === 'Enter') void doPreviewFromInput(); }} />


─── apps/web/src/organizations/OrganizationsPage.tsx:1954-1957 ───
[bug · low] 可达性回退：删除/退出确认弹窗的取消与确认操作从旧实现的 <button> 改为无 role、无 tabIndex、无键盘事件的 span（onClick
only），纯键盘用户无法完成删除/退出确认。同模式还有设置弹窗导航项 nav-item 与侧栏 icon-item-labeled/sidebar-item（旧实现 nav-item 为
<button type=\"button\"> + onKeyDown Enter）。虽是 Vue DOM 1:1 平移（Vue 侧同样无键盘支持），但建议至少补 role="button"
tabIndex={0} 与 Enter/Space 处理，避免 React 端旧有的键盘路径丢失。

            <div className="circle-btn">
-             <span className="circle-btn-txt" onClick={() => setConfirmState(null)}>{t(locale, 'common.cancel')}</span>
-             <span className="circle-btn-txt confirm" onClick={() => void confirmLeaveOrDelete()}>{confirmState?.kind === 'delete' ? t(locale, 'common.delete') : t(locale, 'organization.leave')}</span>
+             <span className="circle-btn-txt" role="button" tabIndex={0} onClick={() => setConfirmState(null)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setConfirmState(null); } }}>{t(locale, 'common.cancel')}</span>
+             <span className="circle-btn-txt confirm" role="button" tabIndex={0} onClick={() => void confirmLeaveOrDelete()} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); void confirmLeaveOrDelete(); } }}>{confirmState?.kind === 'delete' ? t(locale, 'common.delete') : t(locale, 'organization.leave')}</span>
            </div>


─── apps/web/src/organizations/OrganizationsPage.tsx:14-14 ───
[maintainability · low] FormEvent 命名类型导入未被使用：文件内三处签名（741/760/952 行）均为
React.FormEvent<HTMLFormElement>，直接使用 React 命名空间，无需该导入。建议移除以免误导后续修改。

- import type { CSSProperties, FormEvent, MouseEvent as ReactMouseEvent } from 'react';
+ import type { CSSProperties, MouseEvent as ReactMouseEvent } from 'react';


─── apps/web/src/organizations/SpaceAvatar.tsx:29-29 ───
[bug · high] 模板字符串拼接 className 时缺少空格：`wk-avatar-2${...}` 实际生成形如 "wk-avatar-2wk-avatar-3"
的合并类名，org-u.css 中定义的 .wk-avatar-3/4/5（尺寸圆角与阴影）将永远无法命中——所有尺寸（含默认 medium 的
wk-avatar-5）头像的圆角/阴影全部失效。该组件被 KnowledgeBaseShareDialog.tsx 与 KBShareSettingsSection.tsx 引用，影响面不止组织页。

-   return <span className={`wk-avatar-2${size === 'small' ? 'wk-avatar-3' : size === 'large' ? 'wk-avatar-4' : 'wk-avatar-5'} ${className}`} style={style} aria-hidden="true">
+   return <span className={`wk-avatar-2 ${size === 'small' ? 'wk-avatar-3' : size === 'large' ? 'wk-avatar-4' : 'wk-avatar-5'} ${className}`} style={style} aria-hidden="true">


─── apps/web/src/organizations/SpaceAvatar.tsx:32-32 ───
[bug · high] 与第 29 行同源的问题：`wk-avatar-6${...}` 缺少空格，生成 "wk-avatar-6wk-avatar-7"
等合并类名，.wk-avatar-7/8/9（large 20px / small 11px / medium 14px 字号）全部失效，字母头像将回退到继承字号。需在 wk-avatar-6
与插值之间补一个空格。

-       <span className={`wk-avatar-6${size === 'large' ? 'wk-avatar-7' : size === 'small' ? 'wk-avatar-8' : 'wk-avatar-9'}`} style={{ textShadow: `0 1px 2px ${to}80, 0 0 8px ${from}30` }}>{letter}</span>
+       <span className={`wk-avatar-6 ${size === 'large' ? 'wk-avatar-7' : size === 'small' ? 'wk-avatar-8' : 'wk-avatar-9'}`} style={{ textShadow: `0 1px 2px ${to}80, 0 0 8px ${from}30` }}>{letter}</span>


─── apps/web/src/organizations/orgs.td.css:827-834 ───
[bug · medium] 本区块（及
.card-title/.card-content/.card-bottom/.card-decoration/.more-wrap/.feature-badge/.pending-requests-
badge/.relation-role-tag/.empty-state 等后续同批规则）未按本文件 §4 头注声明的 "scoped 块 → 根类前缀 .org-list-container"
约定加根前缀，与同源先例相悖：kb-list.td.css 将同名类限定在 .kb-list-container 下（如 .kb-list-container .card-header
L1195），agents.td.css 限定在 .agent-list-container 下。CSS 打包后全局生效，这些裸类名会直接命中 kb/agents 页卡片 DOM 及任何使用
.empty-state/.card-header 等通用类名的其他页面（如 settings 系 scoped empty-state 之外未覆盖的属性 flex:1、padding:60px
20px 会泄漏），最终视觉取决于各页面 CSS 的 import 顺序，属于跨页样式互染隐患。建议为这批规则补 .org-list-container（或
.org-card）前缀，与既有两页判例对齐。

- .card-header {
+ .org-list-container .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
    position: relative;
    z-index: 2;
  }


─── apps/web/src/settings/PersonalMemoryPanel.tsx:117-118 ───
[maintainability · medium] `memoryWorkspacePatch` 返回 `Record<string, unknown>`，而
`client.settings.memory.workspace.update` 的入参 `SettingsPayload` 同为 `Record<string,
unknown>`，类型本就兼容。`as never` 会完全关闭该调用的类型检查：若后续补丁负载形状与 update 期望发生漂移（如键名 camelCase/snake_case
不一致），编译期无法发现，长期记忆配置将静默保存失败且难以定位。建议直接删除断言。

              },
-           ) as never);
+           ));


─── apps/web/src/settings/PersonalMemoryPanel.tsx:49-49 ───
[bug · medium] `row[key] !== 0` 的回退会把合法存储的 0 值替换为默认值：surface.ts 的 `memoryWorkspacePatch` 校验
`extractMinIntervalSeconds` 允许 0–86400（UI `min={0}`），用户保存 0 后，重载面板时 readDraft 会把 0 静默回退成 300
并显示；此后用户改动任意其他设置触发防抖保存，300 就会被写回服务端，用户配置被无声改写。建议仅以 typeof/Number.isFinite 判缺省。

-   const num = (key: string, fallback: number): number => (typeof row[key] === 'number' && row[key] !== 0 ? row[key] as number : fallback);
+   const num = (key: string, fallback: number): number => (typeof row[key] === 'number' && Number.isFinite(row[key]) ? row[key] as number : fallback);


─── apps/web/src/settings/PersonalMemoryPanel.tsx:70-71 ───
[maintainability · low] 渲染期间直接写入 `draftRef.current` 属于渲染期副作用（违反渲染纯度基线，React 18
并发渲染下有被丢弃/重复执行的隐患）。建议移入 useEffect 同步。

    const draftRef = useRef(draft);
-   draftRef.current = draft;
+   useEffect(() => { draftRef.current = draft; }, [draft]);


─── apps/web/src/settings/PersonalMemoryPanel.tsx:283-283 ───
[bug · low] TDesign InputNumber 在输入框被清空时 onChange 回调的 value 为 null，`Number(null)` 会得到 0 并写入草稿；500ms
后防抖自动保存触发 `memoryWorkspacePatch` 校验抛错（如 max_items 须 10–2000），用户清空重输的瞬间就会收到报错 toast（本行与
extract_delay_seconds / interest_threshold 等行同款问题）。建议空值时跳过更新与保存。

-             <InputNumber value={draft.max_items} min={10} max={2000} step={10} disabled={!canEdit} onChange={(value) => { update({ max_items: Number(value) }); debouncedSave(); }} />
+             <InputNumber value={draft.max_items} min={10} max={2000} step={10} disabled={!canEdit} onChange={(value) => { if (value === null || value === undefined) return; update({ max_items: Number(value) }); debouncedSave(); }} />


─── apps/web/src/settings/ModelSettingsPanel.tsx:964-964 ───
[maintainability · low] `<TLoading loading={false}>` 恒不进入加载态，属死包装：既浪费一层无意义的
DOM（.model-list-loading），又误导维护者以为模型列表存在加载中状态。建议移除包装、直接渲染内容；若确需加载态应接入真实 loading 状态。



─── apps/web/src/settings/ModelSettingsPanel.tsx:1035-1035 ───
[bug · low] 原实现删除按钮带 `disabled={busy}`，迁移后丢失：busy（如删除/复制进行中）期间按钮仍可点击并弹出 Popconfirm，确认后 `remove()`
入口的 busy 守卫静默 no-op，用户感知为「确认删除无响应」。建议恢复 `disabled={busy}`。

-                               <TButton theme="danger" shape="square" variant="text" size="small" className="model-card__action-btn model-card__delete" icon={<TIcon name="delete" />} onClick={(event) => event.stopPropagation()} />
+                               <TButton theme="danger" shape="square" variant="text" size="small" className="model-card__action-btn model-card__delete" icon={<TIcon name="delete" />} disabled={busy} onClick={(event) => event.stopPropagation()} />


─── apps/web/src/settings/PlatformApiKeysPanel.tsx:5-5 ───
[maintainability · low] 第二条 `import { Alert, Button } from 'tdesign-react'` 中的 `Button`
在文件内没有任何使用点（全部使用的是 TButton），构成死导入；且与第 2 行构成对同一模块的重复导入。建议只保留 `Alert` 并合并进首条导入。

- import { Alert, Button } from 'tdesign-react';
+ // 合并至首条：import { Alert, Button as TButton, Checkbox as TCheckbox, Input as TInput } from 'tdesign-react';


─── apps/web/src/settings/OllamaSettingsPanel.tsx:166-173 ───
[style · low] 状态徽标为三层嵌套三元（testing → connectionStatus === true → === false → 兜底），下方已装模型区
`loadingModels ? ... : models.length > 0 ? ... : ...` 又是一处嵌套三元，可读性差且违反评审基线。建议抽成局部变量或分支函数（如
renderStatusTag()）后按序 return。



─── apps/web/src/settings/ParserEngineSettingsPanel.tsx:375-376 ───
[style · low] `<button type="button">` 的内容模型只允许 phrasing content，此处新增的
`engine-card__badge`/`engine-card__body` 两个 `<div>`（内含 h3/p）违反 HTML 规范，可能影响辅助技术语义与浏览器默认样式重置；旧实现用的是
span 包装。另外状态徽标 `statusTone === 'on' ? ... : statusReason ? ... : ...` 也是嵌套三元，建议一并抽函数。

-     <div className="engine-card__badge">{initial}</div>
-     <div className="engine-card__body">
+     <span className="engine-card__badge">{initial}</span>
+     <span className="engine-card__body">


─── apps/web/src/settings/ParserEngineSettingsPanel.tsx:17-18 ───
[documentation · low] 注释称「配置抽屉沿用 React 表单栈…待后续批次收编」，但本次改动中 EngineDrawer 的 Input/Select/Checkbox
已全部换成 tdesign-react 的 TInput/TSelect/TCheckbox，注释与代码自相矛盾，会误导后续批次误判收编范围。建议同步更新注释。

-    配置抽屉沿用 React 表单栈（批次先例：resource/mcp/models 编辑器同口径，
-    扫描稳态不可达，待后续批次收编）。 */
+    配置抽屉已随本批次切换 tdesign-react 表单控件（TInput/TSelect/TCheckbox）。 */


─── apps/web/src/settings/ModelDebugPanel.tsx,apps/web/src/settings/ModelOptionSelect.tsx,apps/web/src/settings/ModelSelector.tsx,apps/web/src/settings/ModelSettingsPanel.tsx,apps/web/src/settings/OllamaSettingsPanel.tsx,apps/web/src/settings/ParserEngineSettingsPanel.tsx,apps/web/src/settings/PersonalMemoryPanel.tsx,apps/web/src/settings/PersonalMemorySettingsPanel.tsx,apps/web/src/settings/PlatformApiKeysPanel.tsx,apps/web/src/settings/PortedSectionsPanel.tsx:0-0 ───
[style · low] 文件末尾缺少换行符（diff 标记 No newline at end of file），不符合 POSIX 文本文件与仓库惯例，也会在后续 diff
中产生噪音。请补齐行尾换行。



─── packages/design-tokens/src/styles.css:23-23 ───
[maintainability · medium] 将 tdesign-theme.css 内嵌进共享令牌文件，会把整套 TDesign
组件库全局覆盖（`input:-webkit-autofill … !important`、暗色滚动条 `!important`、`.t-radio-group
.t-radio-button`、`.doc-link`、`.more-icon`）泄漏给 design-tokens/styles.css 的所有消费方，而 Vue
端架构中该主题只在应用入口引入（frontend/src/main.ts:9），并非共享包的一部分：
1) apps/embed（apps/embed/src/styles.css:4 以 layer(theme) 引入本文件）完全不使用 TDesign，却被迫在轻量嵌入式 widget 中打包这约
200 行死规则，且 autofill/滚动条的 !important 规则会实际作用于 embed 自己的原生 input/滚动条（背景取 `--td-bg-color-container`
#fff/#242424，与 embed 自身 `--wk-*` 输入面配色不一致）；!important 在 layer 内按级联规则仍会压过未分层声明。
2) apps/web 已在 styles.css:7 直引 tdesign-theme.css（unlayered，真正生效的那份），此处嵌套 @import 只是向构建产物
layer(theme) 内重复打入同一份约 200 行样式，纯冗余字节。

建议：从共享 styles.css 中移除该 @import，TDesign 主题仅由使用 TDesign 的应用（apps/web）在入口直引——这与 Vue 端事实源一致，也避免与已确认的
@import 位置问题修完后仍存在的跨应用泄漏。

- @import "./tdesign-theme.css";
+ /* TDesign 组件主题不放入共享令牌文件：由使用 TDesign 的应用在入口直引
+    （apps/web/src/styles.css 已直引本包的 tdesign-theme.css，与 Vue frontend/src/main.ts:9 一致），
+    避免 TDesign 全局组件覆盖泄漏到不使用 TDesign 的消费方（apps/embed）。 */


─── apps/web/src/test-tdom-harness.ts:6-8 ───
[documentation · low] 用法说明中"ESM import 按序执行，本模块先于后续组件 import 运行"给出的心智模型有误：ESM 静态导入的整个模块图（含被测组件及其依赖的
.css）在**任何模块求值之前**就已解析完毕，本模块体内注册的 registerHooks css/svg 桩只对注册之后的解析生效——即仅对**动态** `await import()`
的组件生效。这正是现有全部消费方（如
FAQPage.test.tsx:24、doc-row-menu-interaction.test.tsx:31、agent-editor.test.tsx:47）都以 `await
import('./X.tsx')` 加载被测组件的原因。后续测试作者若按此注释在被测文件顶部静态 import 组件（组件依赖链含 .css），模块图解析先于 hook 注册，测试将以
"Unknown file extension .css" 失败。建议在用法说明中明确：被测组件必须在引入本 harness 之后用 `const { X } = await
import('./X.tsx')` 动态加载。

   * 用法：在被测组件渲染 tdesign 组件的测试文件**首个 import** 处引入：
   *   import './test-tdom-harness.ts';
-  * （ESM import 按序执行，本模块先于后续组件 import 运行。）
+  * 注意：ESM 静态导入的模块图（含组件依赖的 .css/.svg）在任何模块求值前就已解析，
+  * registerHooks 的 css/svg 桩只对本模块求值之后的解析生效——被测组件必须在此后
+  * 动态加载：const { MyPage } = await import('./MyPage.tsx');
+  * （jsdom 全局与 renderAdapter 注入则按求值顺序先于组件模块执行，静态/动态均可受益。）


─── apps/web/src/integrations/EmbedPreviewModal.tsx:93-93 ───
[bug · medium] 预览 iframe 自身没有任何尺寸/边框规则：Vue 基线 frontend/src/components/EmbedChannelPreview.vue 的
`.preview-iframe { width:100%; height:100%; border:none; display:block }`（L189-195）未随本迁移回填到 wk-epm-*
或 integrations.td.css——组件注释自述的 "Former .wk-embed-preview-screen + iframe rules" 中 iframe 部分丢失。容器
wk-epm-13 是 `place-items:center`，iframe 将以 UA 默认 300×150 + 2px inset 边框渲染在 480px
设备框中央，预览严重缩水。packages/views 的 EmbedChannelPreviewPanel（wk-vi-24 容器、wk-vi-156 仅管 visibility、wk-vi-28
widget 面板）同样缺失。建议在新 CSS 中按 Vue 基线补齐 iframe 规则。

-             {layoutReady ? <iframe title={label} src={src} onLoad={() => setReady(true)} className={ready ? '' : 'wk-epm-15'} allow="clipboard-write" /> : null}
+ /* integrations-u.css 或 integrations.td.css 补齐（对齐 Vue .preview-iframe） */
+ .wk-epm-13 iframe {
+   width: 100%;
+   height: 100%;
+   border: 0;
+   display: block;
+   background: #fff;
+ }


─── packages/views/src/integrations/page.tsx:669-669 ───
[bug · medium] 同 apps/web 侧 EmbedPreviewModal：本组件 iframe 与 widget 面板（wk-vi-28）内的 iframe
均无任何尺寸/边框规则——Vue 基线 .preview-iframe 的 width/height 100% + border:none 未随 wk-vi-13..28 工具类平移回填，iframe
将以 UA 默认 300×150 + inset 边框渲染，与本次迁移声明的值平移目标不符。建议在 views-integrations-u.css 补 `.wk-vi-24 iframe` /
`.wk-vi-28 iframe` 规则。

-           <div className="wk-vi-24">{!ready ? <span className="wk-muted wk-vi-3">{t('embedPublish.previewLoading')}</span> : null}{layoutReady ? <iframe title={preview.channel.name || t('embedPublish.preview')} src={src} onLoad={() => setReady(true)} className={ready ? '' : 'wk-vi-156'} allow="clipboard-write" /> : null}</div>
+ /* views-integrations-u.css */
+ .wk-vi-24 iframe,
+ .wk-vi-28 iframe {
+   width: 100%;
+   height: 100%;
+   border: 0;
+   display: block;
+   background: #fff;
+ }


─── apps/web/src/integrations/ApiPlaygroundDrawer.tsx:342-342 ───
[bug · medium] Input/Textarea 离栈后，agent combobox、external-user、query 三个原生字段仅剩
wk-apd-1（box-sizing），边框/内边距/高度/聚焦态全部丢失——被移除的 @weknora/ui 组件内部自带字段 chrome。文件头部注释引用的 Ollama combobox
先例实际配套了 `.wk-ollama-combobox-wrap input` 的完整 chrome 对齐规则（settings.td.css
L5077-5092：border/padding/height/focus），而 integrations.td.css 对这三个类名只有 `box-sizing:
border-box`，抽屉字段将以 UA 默认样式渲染，属可感知的视觉回归。建议在 integrations.td.css 为这三个字段补齐对齐 t-input 的规则。

-               <input className="wk-api-playground-external-user wk-apd-1" type="text" value={form.externalUserId} disabled={mode === 'tenant'} placeholder={t('integrations.api.playgroundExternalUserPlaceholder')} onChange={(event) => setForm((prev) => ({ ...prev, externalUserId: event.target.value }))} style={fieldStyle} />
+ /* integrations.td.css —— 参照 settings.td.css .wk-ollama-combobox-wrap input 的 chrome 对齐口径 */
+ .wk-api-playground-overlay .wk-api-playground-agent,
+ .wk-api-playground-overlay .wk-api-playground-external-user {
+   box-sizing: border-box;
+   width: 100%;
+   height: 32px;
+   padding: 0 8px;
+   border: 1px solid var(--td-component-stroke, #e7e7e7);
+   border-radius: 6px;
+   background: #ffffff;
+   color: var(--td-text-color-primary, rgba(0, 0, 0, 0.9));
+   font: inherit;
+   font-size: 14px;
+ }
+ .wk-api-playground-overlay .wk-api-playground-query {
+   box-sizing: border-box;
+   width: 100%;
+   min-height: 32px;
+   padding: 6px 8px;
+   border: 1px solid var(--td-component-stroke, #e7e7e7);
+   border-radius: 6px;
+   background: #ffffff;
+   font: inherit;
+   font-size: 14px;
+   resize: vertical;
+ }


─── apps/web/src/integrations/views-integrations-u.css:903-905 ───
[bug · low] `top: calc(100%+4px)` 是非法 calc 表达式（`+` 两侧必须有空格），该声明会被浏览器整条丢弃 → position:absolute 的下拉框
top 落为 auto（静态位置，即与触发按钮同一行的相邻位置），agent 筛选下拉会与按钮并排/重叠渲染，而非落在其下方 4px。原 Tailwind `top-[calc(100%+4px)]`
同样无效，属迁移原样保留的坏值；修复只需补空格，布局意图即可生效。

-   position: absolute;
-   top: calc(100%+4px);
-   left: 0;
+   top: calc(100% + 4px);


─── apps/web/src/integrations/IntegrationsPage.tsx:2-3 ───
[maintainability · low] 该文件运行时不可达：路由实际挂载的是 packages/views 的 IntegrationsPage（经 IntegrationsRoutePage
→ @weknora/views/integrations/page），全仓库对本文件的引用仅剩 route.test.ts 的源码扫描断言。本次变更为其做了 tdesign
化改造（TAlert/TDrawer/TInput），等于持续为死代码投入维护成本，且两份 IntegrationsPage 会随时间漂移。建议删除本文件并把 route.test.ts 断言改指
views 包版本；若确需保留作为测试契约锚点，请在文件头注释显式声明其定位。



─── apps/web/src/knowledge-settings/knowledge-settings-u.css:46-49 ───
[bug · high] 无效 calc() 声明：CSS 规范要求 calc() 内 `+`/`-` 运算符两侧必须有空白符，`calc(100%+4px)` 会被浏览器按语法错误丢弃，整条
`top` 声明失效。`.wk-kss-7`（共享提示弹窗）回退到绝对定位元素的静态位置——其定位容器 `.wk-kss-6` 是 inline-flex，dl 的静态位置在触发按钮右侧而非下方
4px，弹窗会横向弹出遮挡标题区，而非原 Tailwind `top-[calc(100%+4px)]` 预期的下拉位置。源码任意值本应写成 `calc(100%_+_4px)`，平移到静态 CSS
时漏了空格，应补齐运算符两侧空格。

  .wk-kss-7 {
    position: absolute;
    right: 0;
-   top: calc(100%+4px);
+   top: calc(100% + 4px);


─── apps/web/src/knowledge/knowledge-u.css:107-109 ───
[bug · medium] 同文件第 49 行 `.wk-kss-7` 同根因的无效 calc()（本次修正漏网）：`calc(100%-2rem)` 中 `-` 两侧无空格，整条
max-width 声明被浏览器丢弃。图谱搜索面板固定 320px（width: 20rem），小屏（容器 < 352px）时失去 `calc(100%-2rem)`
的收窄保护，面板会溢出画布右缘（surface overflow:hidden 会裁掉图例/输入内容）。应写为 `calc(100% - 2rem)`。

    display: flex;
    width: 20rem;
-   max-width: calc(100%-2rem);
+   max-width: calc(100% - 2rem);


─── apps/web/src/knowledge/knowledge-u.css:197-200 ───
[bug · medium] 无效 calc()：`calc(100%+4px)` 因 `+` 两侧缺空格整条 `top` 声明失效，搜索结果下拉（.wk-kg-21）回退到绝对定位元素的静态位置（约
top:32px，紧贴输入框），丢失预期的 +4px 偏移；一旦容器内前面兄弟元素尺寸变化（如换行、加图标），下拉还会明显错位。应写为 `calc(100% + 4px)`。

  .wk-kg-21 {
    position: absolute;
    left: 0;
-   top: calc(100%+4px);
+   top: calc(100% + 4px);


─── apps/web/src/knowledge/knowledge-u.css:253-256 ───
[bug · medium] 无效 calc()：`calc(100%+8px)` 因 `+` 两侧缺空格整条 `top` 声明失效，帮助弹窗（.wk-kg-25，`?` 按钮下的
dl）回退到静态位置（约 top:32px），与预期的 100%+8px 相差 8px。应写为 `calc(100% + 8px)`。

  .wk-kg-25 {
    position: absolute;
    right: 0;
-   top: calc(100%+8px);
+   top: calc(100% + 8px);


─── apps/web/src/knowledge/knowledge-u.css:249-251 ───
[bug · medium] 选择器双重错误导致规则永不匹配：(1) `-webkit-details-marker` 是伪元素，应为双冒号 `::`，单冒号写法不是有效的伪类；(2)
`.wk-kss-24` 与其之间有空格，构成后代选择器，匹配的是 summary 后代元素的 marker，而 marker 伪元素只存在于 summary 自身。原 Tailwind
`[&::-webkit-details-marker]:hidden` 编译为
`.x::-webkit-details-marker{display:none}`。当前写法使隐藏规则失效，Chrome/Safari 下帮助按钮
`?`（details/summary）会重新显示默认 disclosure 三角。

- .wk-kg-24 :-webkit-details-marker {
+ .wk-kg-24::-webkit-details-marker {
    display: none;
  }


─── apps/web/src/shared/SharedSessionPage.tsx:104-104 ───
[bug · high] 类名拼接缺少分隔空格：ROLE_TONE 的值已是 'wk-shared-role-user' 等类名，模板串拼出
'wk-shared-18wk-shared-role-user' 这样的合并类名。shared-u.css 中 .wk-shared-18 与 .wk-shared-role-*
是两个独立选择器，合并类名永远无法命中，角色徽标的圆角/边框/内边距（.wk-shared-18）与角色色调（绿/蓝/灰）全部失效，消息列表视觉完全回归。

-                   <span className={`wk-shared-18${ROLE_TONE[message.role]}`}>
+                   <span className={`wk-shared-18 ${ROLE_TONE[message.role]}` }>


─── apps/web/src/shared/shared-u.css:48-51 ───
[bug · high] border-style-bottom / border-width-bottom 不是合法 CSS 属性（应为 border-bottom-style /
border-bottom-width），浏览器会整条丢弃。同时规则内的 border-style: solid 与 border-color 作用于四边，border-width 回落为初始值
medium（≈3px），结果是原设计中仅 1px 的下边框分隔线变成四边粗实线边框，头部视觉严重回归。建议直接收敛为一条 border-bottom 简写并删除四边生效的
border-style/border-color。

-   border-style-bottom: solid;
-   border-width-bottom: 1px;
-   border-style: solid;
-   border-color: #eef1f5;
+   border-bottom: 1px solid #eef1f5;


─── apps/web/src/shared/shared-u.css:177-180 ───
[bug · medium] .wk-shared-1 在文件内定义了两段：首段携带标题语义（font-size:20px / font-weight:600 /
margin-bottom:8px），末尾这段为其补充 gap 供 <dl class="wk-shared-9 wk-shared-1"> 复用。两段同时命中 dl
时，font-size/color/margin 可被更靠后的 .wk-shared-9 覆盖，但 font-weight:600
无任何对冲规则，元信息行（引擎、来源等）将被错误加粗（原样式为常规字重）。一个语义类承载两种元素语义也易在后续维护中层叠冲突，建议为 gap 定义独立类并从 dl 移除 wk-shared-1。

- .wk-shared-1 {
+ .wk-shared-meta-gap {
    column-gap: 16px;
    row-gap: 4px;
  }


─── apps/web/src/shared/wk-legacy.tsx:75-82 ───
[bug · high] 焦点管理 effect 以 [onClose, open] 为依赖，但消费方普遍以内联箭头函数传 onClose（如 KnowledgeDocumentsPage 的 URL
导入弹层 onClose={() => setSourceUrlDialogOpen(false)}，内部 TdInput 为受控输入）。每敲一键父组件重渲染 → onClose 引用变化 →
effect cleanup 先执行 restoreRef.current?.focus() 把焦点拉回弹层外的触发元素，随后 effect 重新执行 focus()
弹层容器——输入框每次按键后即失焦（autofocus 不会再触发），弹层内输入基本不可用。建议用 ref 持有最新 onClose，依赖数组只保留 [open]。

-     return () => {
-       document.removeEventListener('keydown', onKeyDown);
-       const index = dialogRef.current ? openDialogStack.indexOf(dialogRef.current) : -1;
-       if (index >= 0) openDialogStack.splice(index, 1);
-       restoreRef.current?.focus();
-       restoreRef.current = null;
-     };
-   }, [onClose, open]);
+   // 组件体内：const onCloseRef = useRef(onClose); onCloseRef.current = onClose;
+   // onKeyDown 中调用 onCloseRef.current()，依赖数组改为 [open]


─── apps/web/src/shared/wk-legacy.tsx:174-176 ───
[bug · high] WkSheet 的焦点 effect 与 WkDialog 同样以 [onClose, open] 为依赖，存在相同的焦点抖动/失焦问题：消费方（如
KnowledgeDocumentDetailPage:334 的 trace 抽屉 onClose={() =>
setTraceOpen(false)}）内联传参时，抽屉打开期间父组件任何受控状态更新都会触发 cleanup → restoreRef.current?.focus()（焦点被拉回抽屉外）→ 重新
focus 抽屉面板，正在交互的控件失去焦点。建议与 WkDialog 一致改为 ref 持有 onClose、依赖仅 [open]。

-     document.addEventListener('keydown', onKeyDown);
-     return () => { document.removeEventListener('keydown', onKeyDown); restoreRef.current?.focus(); restoreRef.current = null; };
-   }, [onClose, open]);
+   }, [open]); // onClose 通过 ref 持有最新引用


─── apps/web/src/shared/wk-legacy.tsx:141-145 ───
[bug · medium] 拖宽逻辑存在两处健壮性缺陷：1) body 的 cursor='col-resize' 与 userSelect='none' 仅在 mouseup 的 stop()
中复位，本 cleanup 只移除监听——若拖拽进行中组件被卸载（路由切换等），body 将永久残留 col-resize 光标与禁选状态；2) stop() 内
window.localStorage.setItem 无 try/catch，Safari 隐私模式或配额满时抛 QuotaExceededError，会中断其后的光标复位语句（读取处
getItem 建议同样保护）。建议 cleanup 中复位 body 样式，setItem 包 try/catch。

      return () => {
        window.removeEventListener('mousemove', move);
        window.removeEventListener('mouseup', stop);
+       document.body.style.cursor = '';
+       document.body.style.userSelect = '';
      };
    }, [maxWidth, minWidth, panelWidth, resizable, side, storageKey]);


─── apps/web/src/shared/wk-legacy.tsx:186-186 ───
[bug · low] title 的类型是 ReactNode，传入 JSX/复合标题时 String(title) 得到 "[object Object]"。虽然同元素的
aria-labelledby（指向 h2）在可访问名称计算中优先级更高、多数场景不会被播报，但该无效 aria-label 会污染名称回退链，且语义上就是错误的。建议仅在 title 为
string 时输出，或直接删除该属性交由 aria-labelledby 承担。

-           aria-label={String(title)}
+           {...(typeof title === 'string' ? { 'aria-label': title } : {})}


─── apps/web/src/shared/shared-u.css:29-33 ───
[maintainability · low] 同一规则内 border-style: solid
声明了两次（.wk-shared-4/.wk-shared-8/.wk-shared-13/.wk-shared-15
均存在同样重复），是迁移生成器的残留冗余。虽不影响渲染结果，但会误导后续维护者对生效样式的判断，建议去重或收敛为 border: 1px solid … 简写，并排查生成器避免其它 *-u.css
产出同类残留。

    border-radius: 6px;
-   border-style: solid;
-   border-width: 1px;
-   border-style: solid;
-   border-color: #dcdcdc;
+   border: 1px solid #dcdcdc;


─── apps/web/src/organizations/org-u.css:1420-1423 ───
[bug · high] Tailwind utilities 平移遗漏 `left: 50%`：旧实现为 `fixed left-1/2 top-[24px] z-[3000] …
-translate-x-1/2`，本类只剩 `top: 24px` 与 `translate: -50% 0`。fixed 元素 left 为 auto 时定位在静态位置，再叠加 -50%
水平位移后，toast 不再顶部居中且位置随渲染锚点漂移——页面所有 showToast 操作反馈（复制成功/保存失败/审核结果等）位置错乱。

  .wk-org-128 {
    position: fixed;
+   left: 50%;
    top: 24px;
    z-index: 3000;


─── apps/web/src/organizations/org-u.css:639-644 ───
[bug · high] `calc()` 内 `-` 运算符两侧缺少空白，按 CSS 规范整条 width 声明无效被浏览器丢弃。旧 Tailwind 任意值
`w-[min(520px,calc(100vw-24px))]` 编译时会自动规范化为带空格形式，平移时照抄了源字符串。该权限矩阵弹层为 absolute 定位，宽度退化为
shrink-to-fit，失去 520px 上限，内容较长时直接溢出视口。

- .wk-org-37 {
-   position: absolute;
-   left: 26px;
-   top: 0;
-   z-index: 30;
-   width: min(520px,calc(100vw-24px));
+   width: min(520px, calc(100vw - 24px));


─── apps/web/src/organizations/org-u.css:759-764 ───
[bug · high] 与 .wk-org-37 同款问题：`calc(100vw-48px)` 运算符两侧无空格，整条 width 声明无效。添加成员弹层（absolute
定位）宽度退化为内容收缩宽度，340px 上限与 100vw 防溢出约束同时失效，小视口下会溢出屏幕。

- .wk-org-49 {
-   position: absolute;
-   right: 0;
-   top: 36px;
-   z-index: 30;
-   width: min(340px,calc(100vw-48px));
+   width: min(340px, calc(100vw - 48px));


─── apps/web/src/organizations/org-u.css:928-929 ───
[bug · medium] `calc(90vh-120px)` 同样因 `-` 两侧无空格整条无效（旧 Tailwind 生效值为 `calc(90vh - 120px)`）。加入组织弹框内容区
max-height 失效；虽外层 .wk-org-122 的 max-height 90vh 仍有兜底约束，但与迁移前生效值不等价，极端内容下弹框底部按钮可能被推出视口。

- .wk-org-67 {
-   max-height: calc(90vh-120px);
+   max-height: calc(90vh - 120px);


─── apps/web/src/organizations/orgs.td.css:1320-1328 ───
[bug · medium] 移动端侧栏隐藏规则在平移中丢失：旧实现 settings 导航容器带 `max-[720px]:hidden`，而本文件无任何 720px 断点规则（仅有 5 个卡片网格
min-width 断点）；同时 org-u.css 又保留了 `.wk-org-5`（<720px 显示的 section 下拉选择器）。结果 <720px 视口下 208px
固定侧栏与下拉选择器同时渲染——同一移动端适配只剩一半，两侧导航互相挤占内容区。建议补回 sidebar 的 720px 隐藏（或与 Vue 事实源核对后同步移除 .wk-org-5）。

+ @media (max-width: 720px) {
- .settings-modal .settings-sidebar {
+   .settings-modal .settings-sidebar {
-   width: 208px;
-   background-color: var(--td-bg-color-settings-modal);
-   border-right: 1px solid var(--td-component-stroke);
-   flex-shrink: 0;
-   display: flex;
-   flex-direction: column;
-   overflow: hidden;
+     display: none;
+   }
  }


─── packages/views/src/craft/shell.tsx:100-100 ───
[bug · medium] 注释承诺「full width under 760px via CSS」，但全仓库（craft.css、apps/web 各样式文件）不存在任何
`.wk-craft-drawer` 规则；且 td.tsx 的 Drawer 在 `.t-drawer__content-wrapper` 上写死了内联 `style={{ width:
'460px' }}`（td.tsx:164），普通媒体查询无法覆盖内联样式——视口窄于 460px（如 375px 手机）时抽屉会横向溢出，承诺的响应式行为实际不存在且比旧实现更难兑现。建议在
craft.css 补一条窄屏规则（需 `!important` 覆盖内联宽度），或修正此注释避免误导。

-     <Drawer open={open} title={title} onClose={onClose} size="460px" placement="right" className="wk-craft-drawer">
+ /* craft.css */
+ @media (max-width: 760px) {
+   .wk-craft-drawer .t-drawer__content-wrapper { width: 100% !important; }
+ }


─── packages/views/src/craft/interaction-card.tsx:10-10 ───
[test · medium] 切换到 td Button 后，按钮文本被包进 `<span class="t-button__text">`（td.tsx:84-86），而配套测试
interaction-card.test.tsx:84 用 `/<button[^>]*>[^<]*</button>/g` 统计按钮——该正则永远无法匹配 td Button
的输出（按钮开标签后紧跟 `<span`），「终态卡不暴露任何决策控件」（文件头声明的红线：迟到审批不能复活已取消的请求）这一不变量就此失去自动化护栏，断言恒真。本次更新未同步修改该测试（对照
versions.test.tsx:56-58 已为同一 span 包裹结构适配了正则）。建议同步更新测试断言，改为按 `<button` 开标签存在性检测。

- import { Button } from './td.tsx';
+ // interaction-card.test.tsx:84
+ const buttons = markup.match(/<button[\s>]/g) ?? [];


─── apps/web/src/shared/shared-u.css:86-92 ───
[bug · low] text-xs 的生效值平移不完整：Tailwind `text-xs` 同时产出 `font-size: 0.75rem` 与 `line-height:
1rem`（16px）。`.wk-shared-9`（dl 元信息行）与下文 `.wk-shared-17`（truncated 提示，原文同样是 text-xs）都只写了 font-size:
12px，line-height 丢失后回落到继承值（视 body line-height 而定，≈14.4px 或 1.5 倍），元信息换行与截断提示的行距与迁移前不一致，违背本文件头部"值 =
迁移时 utilities 编码的生效值"的承诺。建议两处补 `line-height: 16px;`（对照 .wk-shared-8/15/18 均显式携带了 leading 的做法）。

  .wk-shared-9 {
    margin: 0;
    display: flex;
    flex-wrap: wrap;
    font-size: 12px;
+   line-height: 16px;
    color: #8a96a8;
  }


─── apps/web/src/shared/wk-legacy.tsx:35-35 ───
[test · low] wk-legacy.tsx 与 wk-legacy.css 在本变更集中没有任何消费方：apps/web 全库搜索
WkDialog/WkSheet/WkCard/WkStatus 导入以及 .wk-dialog*/.wk-sheet* 类名均无匹配，本 PR 内修改的
DocumentsPageChrome、UploadConfirmDialog 等弹层也未切换到该兼容层，且没有任何测试锚定。文件承载的是非平凡交互逻辑——焦点陷阱、openDialogStack
多层 Esc 关闭语义、抽屉拖宽 + localStorage 持久化、焦点恢复——其中焦点管理缺陷（已在其他评审意见中指出）正是因为缺少行为测试才未被拦截，与文件头"DOM 同构、0%
回归"的目标缺乏验证手段。建议与首批消费方同 PR 落地接入，或至少为 WkDialog/WkSheet 补充最小行为测试（Esc 只关最上层、关闭后焦点恢复触发元素、resize 边界与 body
样式复位），避免 T15 删除 packages/ui 时该兼容层处于无验证网状态。



─── packages/views/src/craft/td.tsx:109-115 ───
[bug · medium] useOverlayFocus 将 onClose 纳入 effect 依赖，但唯一生产调用方 versions.tsx 传入的是内联箭头函数（onClose={()
=> setConfirming(null)}），每次父组件重渲染都会产生新引用。workbench 中
controller.onChange(setControllerState)、turns/ticket/terminal 等状态在弹层打开期间会持续触发重渲染，每次都会执行一遍
cleanup（restoreRef.current?.focus() 把焦点拉回触发按钮）+ 重建（重新聚焦面板根节点）——用户在弹层内已聚焦的控件（如 Cancel/Restore
按钮）焦点被强制夺走，还伴随一次焦点闪跳。建议将 onClose 存入 ref，依赖数组仅保留 [open]，语义上也与"focus 归还只应在 open 翻转时发生"一致。

+   const onCloseRef = useRef(onClose);
+   onCloseRef.current = onClose;
+   useEffect(() => {
+     if (!open) return undefined;
+     // ...
+     const onKeyDown = (event: KeyboardEvent) => {
+       if (event.key !== 'Escape') return;
+       const active = typeof document !== 'undefined' ? document.activeElement : null;
+       if (active && panelRef.current && !panelRef.current.contains(active)) return;
+       event.preventDefault();
+       event.stopPropagation();
+       onCloseRef.current();
+     };
      document.addEventListener('keydown', onKeyDown);
      return () => {
        document.removeEventListener('keydown', onKeyDown);
        restoreRef.current?.focus();
        restoreRef.current = null;
      };
-   }, [onClose, open]);
+   }, [open]);


─── packages/views/src/craft/td.tsx:166-166 ───
[bug · medium] 关闭钮 role="button" 但没有 tabIndex 和键盘事件处理：键盘用户 Tab 无法到达、Enter/Space 无法触发关闭（本文件的 Dialog 中
t-dialog__close span 存在同样问题）。另外弹层声明 aria-modal="true" 但未做焦点陷阱（Tab 可移出到背景内容），与文件头宣称的"与原 Sheet
键盘/读屏契约等价"不符。建议至少补 tabIndex={0} + onKeyDown（Enter/Space 触发 onClose），或直接改用原生 <button> 元素恢复原生键盘语义。

-         <div className="t-drawer__close-btn" role="button" aria-label={closeLabel} onClick={onClose}><CloseIcon /></div>
+         <button type="button" className="t-drawer__close-btn" aria-label={closeLabel} onClick={onClose}><CloseIcon /></button>


─── packages/views/src/craft/td.tsx:39-41 ───
[bug · low] CloseIcon 硬编码 id="close" 与 id="stroke1"：Drawer 与 Dialog 同时打开（或未来多实例并存）时页面出现重复 id，违反 HTML
唯一性约定。手写复刻层中这两个 id 不承担任何样式或 ARIA 引用作用（宿主 tdesign.css 只按 .t-icon/.t-icon-close 类名命中），可直接删除。

-       <g id="close">
+       <g>
          <path
-           id="stroke1"


─── packages/views/src/craft/td.tsx:205-205 ───
[style · low] 静态内联样式 style={{ marginLeft: 'auto' }} 用于关闭钮布局：本层其余视觉均由 t-* 类名驱动，此处应下沉到 CSS（或复用宿主
tdesign.css 对 .t-dialog__close 的既有布局规则），避免内联样式与类样式混杂形成第二套来源。

-               <span className="t-dialog__close" style={{ marginLeft: 'auto' }} role="button" aria-label={closeLabel} onClick={onClose}>
+               <span className="t-dialog__close" role="button" aria-label={closeLabel} onClick={onClose}>


─── package.json:13-13 ───
[test · high] test:shared 将 packages/i18n/test 的 glob 从 *.test.ts 改为 *.test.tsx，但该目录现有 19
个测试文件（authMessages、backfill、chat、keys、runtime、settingsMessages 等）全部是 .test.ts，没有任何 .test.tsx
文件，且本次更新未重命名任何 i18n 测试。glob 匹配不到任何文件，整个 i18n 测试目录被静默排除出共享 CI（node>=22 的 --test 将未命中 glob 视为 0
个用例、正常退出），测试覆盖回退而流水线保持绿色。若目的是兼容未来 .test.tsx，应并列两个 glob 而非替换。

-     "test:shared": "tsx --test packages/contracts/test/*.test.ts packages/contracts/test/mobile-*.test.ts packages/contracts/src/craft/*.test.ts packages/domain/src/*.test.ts packages/domain/src/auth/*.test.ts packages/domain/src/access/*.test.ts packages/domain/src/craft/*.test.ts packages/domain/src/knowledge/*.test.ts packages/domain/src/mobile/*.test.ts packages/domain/src/wiki/*.test.ts packages/domain/src/chat/*.test.ts packages/domain/src/sandbox/*.test.ts packages/domain/src/settings/*.test.ts packages/api-client/src/*.test.ts packages/api-client/src/analytics/*.test.ts packages/api-client/src/auth/*.test.ts packages/api-client/src/craft/*.test.ts packages/api-client/src/identity/*.test.ts packages/api-client/src/knowledge/*.test.ts packages/api-client/src/wiki/*.test.ts packages/api-client/src/chat/*.test.ts packages/api-client/src/sandbox/*.test.ts packages/api-client/src/mobile/*.test.ts packages/api-client/src/settings/*.test.ts packages/api-client/src/embed/*.test.ts packages/api-client/src/transport/*.test.ts packages/core/src/craft/*.test.ts packages/design-tokens/src/*.test.ts packages/i18n/test/*.test.tsx packages/views/src/chat/*.test.ts packages/views/src/chat/*.test.tsx packages/views/src/craft/*.test.ts packages/views/src/craft/*.test.tsx packages/views/src/guides/*.test.ts packages/views/src/guides/*.test.tsx packages/views/src/settings/*.test.ts packages/views/src/embed/*.test.ts packages/views/src/integrations/*.test.ts packages/views/src/integrations/*.test.tsx",
+ … packages/design-tokens/src/*.test.ts packages/i18n/test/*.test.ts packages/i18n/test/*.test.tsx packages/views/src/chat/*.test.ts …


─── package.json:13-13 ───
[test · medium] 本次新增的 packages/career-core 含 contracts.test.ts 与 desk.test.ts 两个测试文件，但 test:shared
清单未加入 packages/career-core/src/*.test.ts（api-client 侧的 career.test.ts 恰好被
packages/api-client/src/*.test.ts 命中，掩盖了这一缺口），desk.ts 也不在 typecheck:shared 的显式清单里（contracts.ts 仅经
api-client/src/index.ts 传递覆盖）。结果是冻结契约解码器（decodeCareerReceipt/decodeEvaluation
等）与未知结果恢复状态机（CareerDesk）的测试在共享 CI 中不会执行，且无任何其他脚本兜底。建议把 packages/career-core/src/*.test.ts 追加进
test:shared，并在 typecheck:shared 显式列入 packages/career-core/src/desk.ts。

-     "test:shared": "tsx --test packages/contracts/test/*.test.ts packages/contracts/test/mobile-*.test.ts packages/contracts/src/craft/*.test.ts packages/domain/src/*.test.ts packages/domain/src/auth/*.test.ts packages/domain/src/access/*.test.ts packages/domain/src/craft/*.test.ts packages/domain/src/knowledge/*.test.ts packages/domain/src/mobile/*.test.ts packages/domain/src/wiki/*.test.ts packages/domain/src/chat/*.test.ts packages/domain/src/sandbox/*.test.ts packages/domain/src/settings/*.test.ts packages/api-client/src/*.test.ts packages/api-client/src/analytics/*.test.ts packages/api-client/src/auth/*.test.ts packages/api-client/src/craft/*.test.ts packages/api-client/src/identity/*.test.ts packages/api-client/src/knowledge/*.test.ts packages/api-client/src/wiki/*.test.ts packages/api-client/src/chat/*.test.ts packages/api-client/src/sandbox/*.test.ts packages/api-client/src/mobile/*.test.ts packages/api-client/src/settings/*.test.ts packages/api-client/src/embed/*.test.ts packages/api-client/src/transport/*.test.ts packages/core/src/craft/*.test.ts packages/design-tokens/src/*.test.ts packages/i18n/test/*.test.tsx packages/views/src/chat/*.test.ts packages/views/src/chat/*.test.tsx packages/views/src/craft/*.test.ts packages/views/src/craft/*.test.tsx packages/views/src/guides/*.test.ts packages/views/src/guides/*.test.tsx packages/views/src/settings/*.test.ts packages/views/src/embed/*.test.ts packages/views/src/integrations/*.test.ts packages/views/src/integrations/*.test.tsx",
+ … packages/api-client/src/transport/*.test.ts packages/career-core/src/*.test.ts packages/core/src/craft/*.test.ts …


─── packages/api-client/src/career.ts:1-2 ───
[maintainability · medium] career.ts 通过相对路径 '../../career-core/src/contracts.ts' 跨包导入，而
packages/api-client/package.json 的 dependencies 只声明了 @weknora/contracts 与 @weknora/domain，没有
@weknora/career-core；同时也绕过了 career-core package.json exports 暴露的 './contracts' 入口。当前全靠 tsx/tsc
从仓库根直接解析源码才能工作，依赖图不完整（pnpm why/lockfile 看不到该依赖），包边界与 exports
映射形同虚设；小程序端（apps/miniprogram/src/services/career.ts）同样以 ../../../../packages/career-core/src/*.ts
深层相对路径引入，全仓库无任何 '@weknora/career-core' 包名导入。既然已把 career-core 注册进 pnpm-workspace 并定义了 exports，建议在
api-client（及小程序）package.json 声明 "@weknora/career-core": "workspace:*" 并改用包名子路径导入。

- import type { CareerAction, CareerChangeSet, CareerDocumentSource, CareerFact, CareerProposal, CareerReceipt, CareerSource, CareerUpload, CareerView, Evaluation, EvaluationReceipt, OpportunityEvidence, OpportunityImportInput, OpportunityReceipt, OpportunitySource, OpportunityStatus } from '../../career-core/src/contracts.ts'
- import { decodeCareerReceipt, decodeCareerSources, decodeCareerUpload, decodeEvaluation, decodeEvaluationReceipt, decodeOpportunityReceipt } from '../../career-core/src/contracts.ts'
+ import type { CareerAction, CareerChangeSet, CareerDocumentSource, CareerFact, CareerProposal, CareerReceipt, CareerSource, CareerUpload, CareerView, Evaluation, EvaluationReceipt, OpportunityEvidence, OpportunityImportInput, OpportunityReceipt, OpportunitySource, OpportunityStatus } from '@weknora/career-core/contracts'
+ import { decodeCareerReceipt, decodeCareerSources, decodeCareerUpload, decodeEvaluation, decodeEvaluationReceipt, decodeOpportunityReceipt } from '@weknora/career-core/contracts'
+ // 并在 packages/api-client/package.json 的 dependencies 中补充："@weknora/career-core": "workspace:*"


─── packages/api-client/src/index.ts:207-209 ───
[maintainability · low] index.ts 同样以相对路径 '../../career-core/src/contracts.ts' 再导出契约类型，与 career.ts
的跨包相对导入同源问题：绕过包 exports 入口、且 api-client 的 package.json 未声明 @weknora/career-core 依赖。建议统一改为从
'@weknora/career-core/contracts' 再导出。

  export { createCareerApi } from './career.ts';
  export type { CareerRequest } from './career.ts';
- export type { CareerAction, CareerView, CareerFact, CareerProposal, CareerReceipt, CareerChangeSet, CareerChange, CareerSource } from '../../career-core/src/contracts.ts';
+ export type { CareerAction, CareerView, CareerFact, CareerProposal, CareerReceipt, CareerChangeSet, CareerChange, CareerSource } from '@weknora/career-core/contracts';


─── packages/career-core/src/contracts.ts:148-153 ───
[bug · medium] decodeEvaluation 使用 TextEncoder/TextDecoder 做 UTF-8 字节切片与 quotedText 比对，但 career-core
是设计为微信小程序可直载的共享包（见 desk.ts 关于小程序 node --experimental-strip-types
直载的注释）。小程序逻辑层不提供这两个全局对象：apps/miniprogram/src/platform/polyfills.ts 只补了
URL/URLSearchParams/AbortController，且仓库自身的 apps/miniprogram/src/core/utf8.ts 注释明确写着 'Works without a
browser TextDecoder'。当前小程序端只调用 decodeEvaluationReceipt 尚未踩中，但一旦消费 GET
/api/v1/career/evaluations/{id}（decodeEvaluation 路径，Web 端 OpportunityPage 已在用）就会在真机抛
ReferenceError。建议在 career-core 内抽出不依赖 TextEncoder/TextDecoder 的共享 utf8 字节工具（可参考小程序 core/utf8.ts
的实现）完成该比对。

   const rawText = value.snapshot.rawText
-  const bytes = new TextEncoder().encode(rawText)
+  // 共享 utf8 工具（不依赖 TextEncoder/TextDecoder，小程序逻辑层可用）
+  const bytes = utf8Encode(rawText)
   const validEvidence = (evidence: unknown): evidence is EvaluationJobEvidence => {
    if (!validEvaluationJob(evidence) || evidence.snapshotId !== value.snapshot.snapshotId || evidence.observationId !== value.snapshot.observationId || evidence.acquiredAt !== value.snapshot.acquiredAt || evidence.rawSha256 !== value.snapshot.rawSha256 || evidence.spanEnd > bytes.length) return false
-   return new TextDecoder().decode(bytes.slice(evidence.spanStart, evidence.spanEnd)) === evidence.quotedText
+   return utf8Decode(bytes.slice(evidence.spanStart, evidence.spanEnd)) === evidence.quotedText
   }


─── packages/api-client/src/errors.ts:71-75 ───
[bug · low] currentRevision 只从 nested（record.error）读取，而同函数中 code、message、requestId 都有 record
顶层的兜底取值链。career 后端的 CareerError 契约确实是嵌套形态，但 errorFromResult 是全部端点共用的解析器：若有任何处理器把 currentRevision
放在顶层错误体返回，revision_conflict 冲突恢复所需的当前修订号会被静默丢弃，前端自动重试拿不到最新 revision。建议与 requestId 一致补一层 record 顶层兜底。

+   let currentRevision: number | undefined;
+   if (typeof nested?.currentRevision === 'number' && Number.isFinite(nested.currentRevision)) currentRevision = nested.currentRevision;
+   else if (typeof record?.currentRevision === 'number' && Number.isFinite(record.currentRevision)) currentRevision = record.currentRevision;
    return new ApiError({
      status, code, message, requestId,
-     ...(typeof nested?.currentRevision === 'number' && Number.isFinite(nested.currentRevision) ? { currentRevision: nested.currentRevision } : {}),
+     ...(currentRevision !== undefined ? { currentRevision } : {}),
      details: nested?.details ?? record?.details,
    });


─── packages/api-client/src/career.ts:779-780 ───
[style · low] factKeys 的判定是嵌套三元表达式（条件 ? A : (条件 ? B : C)），违反代码质量规则（禁止嵌套三元），且该行承担
draft/失败行事实键的关键语义，可读性差、易在后续维护中误改。建议展开为 if/else。

   const rawFactKeys: unknown = sourcesRecord.factKeys
-  const factKeys = rawFactKeys === null || rawFactKeys === undefined ? [] : Array.isArray(rawFactKeys) ? rawFactKeys : undefined
+  let factKeys: string[] | undefined
+  if (rawFactKeys === null || rawFactKeys === undefined) factKeys = []
+  else if (Array.isArray(rawFactKeys)) factKeys = rawFactKeys


─── packages/api-client/src/career.ts:1281-1281 ───
[bug · low] 注释自述该上限对应后端 submission.go 的 maxSubmissionNoteBytes（4096 字节），但客户端用
input.note.length（UTF-16 码元数）做校验：纯中文备注每个字符 UTF-8 占 3 字节，约 1366 个汉字即可通过客户端校验（length 1366 < 4096）却被后端按
4096+ 字节拒绝，产生一次本可避免的失败往返。建议按 UTF-8 字节数校验（如 TextEncoder 编码长度，或与 career-core 共享的 utf8 工具）后再放行。

-    if (input.note !== undefined && input.note.length > maxSubmissionNoteBytes) throw new TypeError('submission note must not exceed 4096 bytes')
+    if (input.note !== undefined && utf8ByteLength(input.note) > maxSubmissionNoteBytes) throw new TypeError('submission note must not exceed 4096 bytes')


─── packages/api-client/src/career.ts:37-39 ───
[maintainability · low] validIdentifier/validTimestamp/decodeRecord 等校验辅助在本文件与
packages/career-core/src/contracts.ts 各自独立实现，且时间戳校验已经分叉：本文件是正则 + Date.parse，contracts.ts
是手工日历/闰年/偏移校验。同一线上 RFC3339 格式的两套校验会随时间漂移产生行为不一致（不同宿主 Date.parse 宽松度也不同），排查线上解码差异时成本翻倍。既然 career-core
已是共享包，建议把校验器抽到 career-core 导出（如 './validate'）供 api-client 复用，保持单一实现。

- function validIdentifier(value: unknown): value is string { return typeof value === 'string' && value.trim().length > 0 }
- function validTimestamp(value: unknown): value is string { return typeof value === 'string' && rfc3339Timestamp.test(value) && Number.isFinite(Date.parse(value)) }
- function validOptionalString(value: unknown): boolean { return value === undefined || typeof value === 'string' }
+ // 从 career-core 复用单一实现，避免两套校验漂移：
+ // import { validIdentifier, validTimestamp, validOptionalString, decodeRecord } from '@weknora/career-core/validate'


─── internal/modules/career/upload.go:120-125 ───
[maintainability · low] ResumeAndParse 每次重试都会对同一 (resource, career_source, sourceID) 再次 Bind,但
resource_bindings 表上没有 (resource_id, owner_type, owner_id, relation) 唯一约束(主键是每次新生成的
UUID),repository.CreateBinding 里的 clause.OnConflict{DoNothing} 实际无可冲突索引、每次都会插入新行。首次
StoreAndParseWithID 已 Bind 一次,之后每次崩溃恢复/resume 重试都会累积一行重复绑定:CountBindings 计数虚高会让 Release 的
remaining>0 判断偏向保留文件(方向安全),并留下冗余行直到该 owner 终态清理。建议为 resource_bindings 增加相应唯一索引使 OnConflict DoNothing
真正生效,或在 ResumeAndParse 重新 Bind 前确认该 owner 绑定尚不存在。

  	result.Upload.ResourceRef = resourceRef
  	if existingRef != "" {
+ 		// Make re-binding idempotent: resource_bindings has no unique index on
+ 		// (resource_id, owner_type, owner_id), so each resume retry inserts a
+ 		// duplicate row unless the binding is checked first (or the table gains
+ 		// a unique constraint that makes CreateBinding's OnConflict effective).
  		if err := a.catalog.Bind(ctx, resourceRef, careerSourceOwner, result.SourceID, types.ResourceRelationSourceFile); err != nil {
  			return result, fmt.Errorf("bind resume source: %w", err)
  		}
  	}


─── packages/views/src/craft/td.tsx:103-104 ───
[bug · medium] Escape 焦点域守卫存在盲区：用户点击弹层内不可聚焦内容（纯文本、空白区域）后，浏览器会把 document.activeElement 置为
<body>（点击非聚焦元素会令焦点从 panel 根节点失焦回落到 body）。此时 `!panelRef.current.contains(active)` 为 true 直接
return，Escape 从此无法关闭 Drawer/Dialog——而这是用户打开弹层后最常见的操作路径之一（选中/点击说明文字）。建议把 body（以及 null）视为"仍在弹层域内"，例如
`active !== document.body && !panelRef.current.contains(active)` 才 return；或改为记录"焦点曾被归还 body"的状态而非仅做
contains 判断。

        const active = typeof document !== 'undefined' ? document.activeElement : null;
-       if (active && panelRef.current && !panelRef.current.contains(active)) return;
+       if (active && panelRef.current && active !== document.body && !panelRef.current.contains(active)) return;


─── packages/views/src/craft/td.tsx:145-147 ───
[maintainability · low] closeLabel 硬编码英文 'Close' 且两个生产调用方（shell.tsx CraftDrawer、versions.tsx
Dialog）均未传入：craft 域其余文案都走 locale 双语（如 versions.tsx 的 `zh ? '取消' : 'Cancel'`），中文界面下关闭钮的 aria-label
却固定朗读英文 "Close"，读屏体验与同域文案口径不一致。建议由调用方按 locale 传入（如 `closeLabel={zh ? '关闭' :
'Close'}`），或将默认值改为中英并列；Dialog 的同款默认值一并处理。

    className,
-   closeLabel = 'Close',
+   closeLabel,
  }: CraftDrawerProps) {
+   const resolvedCloseLabel = closeLabel ?? 'Close';


─── packages/api-client/src/career.ts:1089-1091 ───
[bug · medium] open()/list()/changes() 是本文件中唯一直接把响应体 `as CareerView` / `as CareerChangeSet`
裸断言返回的读接口，其余所有 career 读端点都经过 decode* 严格校验。这破坏了本包自述的冻结契约纪律（"decoders reject invented ... before they
reach the UI"）：畸形/越界的 facts、proposals（如 status 不在枚举内、revision 非数字、facts 缺失）会原样进入
CareerDesk.applyView/syncChanges 的状态并直接渲染到 Web CareerPage
与小程序档案页，表现为难排查的运行时错误，甚至把不可信数据当作已确认事实展示。代码注释用导出归档解码器（decodeExportedProfile）来解释这一豁免，但该校验只覆盖 export
归档路径，并不保护日常的 open/list/changes 主通道；career-core 中 validFact/validProposal 已存在，补一个导出的
decodeCareerView/decodeCareerChangeSet 即可闭环。

-   async open(signal?: AbortSignal): Promise<CareerView> { return await request({ method: 'GET', path: '/api/v1/career/open', ...(signal ? { signal } : {}) }) as CareerView },
-   async list(signal?: AbortSignal): Promise<CareerView> { return await request({ method: 'GET', path: '/api/v1/career/list', ...(signal ? { signal } : {}) }) as CareerView },
-   async changes(since: number, signal?: AbortSignal): Promise<CareerChangeSet> { return await request({ method: 'GET', path: `/api/v1/career/changes?since=${encodeURIComponent(String(since))}`, ...(signal ? { signal } : {}) }) as CareerChangeSet },
+ // career-core 侧新增并导出（复用现有 validFact/validProposal）：
+ // export function decodeCareerView(value: unknown): CareerView { ... }
+ // export function decodeCareerChangeSet(value: unknown): CareerChangeSet { ... }
+   async open(signal?: AbortSignal): Promise<CareerView> { return decodeCareerView(await request({ method: 'GET', path: '/api/v1/career/open', ...(signal ? { signal } : {}) })) },
+   async list(signal?: AbortSignal): Promise<CareerView> { return decodeCareerView(await request({ method: 'GET', path: '/api/v1/career/list', ...(signal ? { signal } : {}) })) },
+   async changes(since: number, signal?: AbortSignal): Promise<CareerChangeSet> { return decodeCareerChangeSet(await request({ method: 'GET', path: `/api/v1/career/changes?since=${encodeURIComponent(String(since))}`, ...(signal ? { signal } : {}) })) },


─── internal/modules/career/career_export.go:673-679 ───
[bug · high] 删除记录创建时 ReceiptBody 以 "{}" 占位，而 DeleteCareer 的 Create 竞争恢复路径（isReceiptRaceError →
FindCareerDeletion）以及公开的删除回执端点 FindCareerDeletion 都会把 "{}" 解码成零值回执并作为成功返回（Kind=""、Status=""、Steps
为空）。步骤执行期间（撤销导出、清理 27 张表、跨模块移除 Workbench 投影可能耗时数秒）该窗口一直打开：并发同 requestID
的第二次调用会拿到这个空回执直接返回，轮询回执端点的客户端也会收到 200 + 空回执——对一个破坏性操作的确认面而言，这违背了本文件自己声明的"receipt never claims ...
until every step is done"契约。建议创建记录时写入真实的进行中回执（Status=deleting、Steps 全 pending），并让
FindCareerDeletion/竞争重放路径对 status==deleting 的回执返回类型化错误（如 ErrDeletionNotFound 或新增
in_progress），而不是把占位符当最终答案。

  		record = careerDataDeletionRecord{
  			ID: uuid.NewString(), TenantID: s.TenantID, UserID: s.UserID,
  			RequestID: input.RequestID, Fingerprint: fingerprint,
  			ExpectedRevision: input.ExpectedRevision, Status: DeletionStatusDeleting,
- 			StateBody: string(mustJSON(execution)), ReceiptBody: "{}",
+ 			StateBody: string(mustJSON(execution)),
+ 			ReceiptBody: string(mustJSON(CareerDeletionReceipt{
+ 				Kind: CareerKindDeleted, RequestID: input.RequestID,
+ 				Status: DeletionStatusDeleting, Steps: execution.Steps,
+ 				Retention: careerDeletionRetention(), StartedAt: now,
+ 			})),
  			CreatedAt: now, UpdatedAt: now,
  		}
+ // 并在 FindCareerDeletion 中：
+ // if receipt.Status == DeletionStatusDeleting { return CareerDeletionReceipt{}, ErrDeletionInProgress }


─── internal/modules/career/career_export.go:829-840 ───
[security · high] delete_career 只清掉了 career_material_exports 的数据库行，但 PublishMaterial 通过
exportStorage.SaveExport 写入文件服务的渲染简历 PDF/DOCX 字节从未被删除——materialExportStorage 接口只有 Save/Read
两个方法（rendering.go），全仓库也没有任何针对这些 object key 的删除调用；RevokeMaterialExport
只改状态不删字节。结果是：整空间"完整删除"后，含个人简历数据的渲染文件仍永久留在对象存储/本地磁盘上，与 CareerDeletionBoundary 向用户披露的
"material_exports: 材料导出与下载授权（将删除）" 不符，构成隐私/数据保留缺口。另外 PublishMaterial 在 revision
冲突、幂等冲突、与并发孪生竞争失败等路径下，先落盘的 career_export_<uuid> 文件也会成为无引用孤儿（无 catalog 绑定，任何 GC 都看不到），同样无法回收。建议：给
materialExportStorage 增加 DeleteExport；删除流程在 purge 行之前先读出该 scope 的全部 pdf/docx object key 并逐个删除（失败可随
partial 状态恢复重试，保持幂等）；PublishMaterial 的失败/冲突路径回收本次新写的对象。



─── internal/modules/career/career_export.go:240-245 ───
[bug · medium] 这里对 career_profiles 的 First 没有容忍 ErrRecordNotFound。requireSpace 只保证 career_spaces
行存在；profile 行是由首次 mutate/ClaimUpload 懒创建的（ClaimSpace 不建 profile）。因此对"已认领空间但从未写过档案"的用户调用
export_career（客户端先 Open 拿到 revision 0 再导出是合法流程）会把裸的 gorm.ErrRecordNotFound 返回，writeError 未映射该错误 →
500 internal。同包 Open()、attemptReminderWrite、PublishMaterial、claimSearchRequest 都按 head.Revision=0
容忍缺行，此处应保持一致。

  		var head profile
  		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
  			Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error
- 		if e != nil {
+ 		if errors.Is(e, gorm.ErrRecordNotFound) {
+ 			head.Revision = 0
+ 		} else if e != nil {
  			return e
  		}


─── internal/modules/career/career_export.go:664-668 ───
[bug · medium] 与 ExportCareer 事务内的 head 读取同样的问题：这里 First(&head) 未容忍 ErrRecordNotFound，而 profile
行是懒创建的。空空间（已 ClaimSpace、从未 mutate/上传）执行 delete_career 也会返回裸 gorm.ErrRecordNotFound →
500，而删除一个刚认领的空空间是合法操作。finalizeDeletion 内的 First 也继承同一问题。建议同样按 head.Revision=0 容忍缺行。

  		var head profile
  		if err = o.db.WithContext(ctx).
  			Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error; err != nil {
+ 			if !errors.Is(err, gorm.ErrRecordNotFound) {
- 			return CareerDeletionReceipt{}, err
+ 				return CareerDeletionReceipt{}, err
+ 			}
+ 			head.Revision = 0
  		}


─── internal/modules/career/career_export.go:525-528 ───
[bug · low] 计数查询失败被吞掉并按 0 展示。这是删除前的确认面（pre-deletion explanation）：一次瞬时 DB 错误会让 "applications: 3" 显示成
"applications: 0"，用户基于失真的范围确认删除，与模块"如实披露"的原则相悖。建议让 sectionCount 返回错误并在 CareerDeletionBoundary
聚合失败（整体返回错误），或显式标记 count 未知，而不是静默地呈现 0。

  		if err := query.Count(&total).Error; err != nil {
- 			return 0
+ 			return 0, fmt.Errorf("count %s: %w", table, err)
  		}
- 		return int(total)
+ 		return int(total), nil


─── internal/modules/career/rendering.go:580-586 ───
[bug · medium] bfrange 展开时把目标码位按高/低字节分别加 offset，低字节相加的进位丢失：当一段连续 used-code 跨越 0x100 边界（例如正文同时用到
U+4EFF 与 U+4F00，或任何 run 内 low&0xFF + offset&0xFF > 0xFF）时，边界之后的码位会被映射成错误字符（如 0x4F00 被映射成
0x4E01），verifyMaterialPDF 提取出的文本与正文不一致 → 渲染完全正确的 PDF 被误判为校验失败，导出退化为 staged/failed 而不可投递。写入端按 PDF 规范写
`<lo> <hi> <lo>`（连续映射）是正确的，只是校验端的展开算术有误。建议以 16 位整数做加法后再拆字节。

+ 			base := uint16(low)
  			for code := low; ; code++ {
- 				offset := int(code - low)
- 				mapping[uint16(code)] = string([]byte{dst[0] + byte(offset>>8), dst[1] + byte(offset)})
+ 				mapped := base + uint16(code-low)
+ 				mapping[uint16(code)] = string([]byte{byte(mapped >> 8), byte(mapped)})
  				if code == high {
  					break
  				}
  			}


─── internal/modules/career/profile_intake.go:414-416 ───
[maintainability · low] `if row.ResourceRef == "" { row.ResourceRef = "" }`
是空操作（无任何副作用），且紧邻上方对非空分支追加 cleanup_pending_ 前缀，读者会误以为这里遗漏了清理语义或本应有 else。建议直接删除这两行遗留代码。

- 			if row.ResourceRef == "" {
- 				row.ResourceRef = ""
- 			}
+ 			// 删除该空操作分支；失败路径保留 ResourceRef 以驱动 cleanup_pending 流程即可。


─── internal/modules/career/career_export.go:791-795 ───
[bug · high] 删除恢复缺口会让回执虚假地声明"deleted"：runDeletionSteps 对已完成步骤（尤其 purge_career_data）直接 continue，而
DeleteCareer 的整条链路（mutate、ImportJD、SetRule、SetReminder、PublishMaterial 等，均只做 requireSpace
校验，不检查空间是否存在 deleting/partial 状态的删除记录）没有任何写守卫。时序：① 首次 delete_career 在 purge 完成后、后续步骤失败 → 状态
partial；② 用户（或仍持有旧 revision 的客户端缓存）在此窗口内重新写入档案/申请/材料数据；③ 同 requestID 重试恢复——purge 已标记 done 被跳过，直接
finalize → 回执 status=deleted。结果是步骤②写入的求职者个人数据在"完整删除"回执之后仍留在 27 张表里，与 DeleteCareer 注释（"receipt never
claims full deletion until every step is done"）和删除审计契约直接矛盾。建议二选一：恢复执行时对幂等的 purge 步骤无条件重跑（DELETE
全量本就幂等），或在空间存在未终结删除记录时于所有写路径拒绝新写入。

  func (o *Office) runDeletionSteps(ctx context.Context, s Scope, requestID string, execution *deletionExecution) string {
  	for i, step := range execution.Steps {
- 		if step.Status == DeletionStepStatusDone {
+ 		if step.Status == DeletionStepStatusDone && step.Name != DeletionStepPurgeCareerData {
  			continue
  		}
+ 		// purge 是幂等的全量 DELETE：恢复时无条件重跑，
+ 		// 吞掉 partial 窗口内新写入的数据，保证 "deleted" 回执可信


─── internal/modules/career/career_export.go:530-532 ───
[bug · medium] 预删除披露与实际清理范围不一致：InSpace 清单遗漏了 careerPurgeTables
中会被清空的多张含个人数据的表——career_source_revisions（extracted_text
保存简历全文原文）、career_evaluations（三值资格判断及其推理依据）、career_preparations、career_source_import_receipts 类回执表，以及
career_data_exports（archive 里内联了完整档案 JSON）。CareerDeletionBoundary 的注释承诺"explains, before any
deletion, exactly what will be
deleted"，而这是一次不可逆的整空间删除确认面：用户基于失真清单（尤其是"上传的简历原文"与"完整导出档案副本"并未被告知将被删除/实际不会删除导出文件字节）确认删除，违背模块"如实披露"原则。
建议 InSpace 与 careerPurgeTables 对齐：逐表列出，并在表清单与实现间建立单一事实来源（例如由 careerPurgeTables 生成披露清单）防止再次漂移。

  view := CareerDeletionBoundaryView{
  		InSpace: []CareerDeletionSection{
- 			{Section: "profile", Description: "已确认的档案事实与待处理提案", Count: sectionCount("career_facts", "")},
+ 			{Section: "sources", Description: "上传的简历原文与提取文本", Count: sectionCount("career_source_revisions", "")},
+ 			{Section: "evaluations", Description: "资格判断及其推理依据", Count: sectionCount("career_evaluations", "")},
+ 			// ... 与 careerPurgeTables 逐一对齐


─── internal/modules/career/rendering.go:1094-1097 ───
[bug · medium] PublishMaterial 在事务提交前就把渲染文件写入存储：pdfKey/docxKey 落盘成功后，后续事务仍可能以
RevisionConflictError、ErrIdempotencyConflict、isReceiptRaceError、SQLite busy 或 ctx 取消而失败——此时
materialExportRecord 行从未创建，这两个文件（含求职者简历全文）成为无引用、无所有者、无 TTL 的永久孤儿。这和已确认的"delete_career
不删除已导出字节"是不同触发路径：后者至少有 DB 行可追溯，这里连行都没有，任何清理机制都无法回收（文件名前缀 career_export_ 无 TTL，本地/对象存储侧无生命周期）。正常使用中
revision 冲突和幂等竞争都真实可触发，孤儿会持续累积。建议：① 事务失败（且确认未提交）时删除刚保存的两个 object key（exportStorage 需补 Delete 方法）；或 ②
先在事务内占用 requestID/写入行（staged 状态），提交成功后再落盘并回填状态，失败路径据此回收。



LLM retry report summary: 105 of 830 requests affected -- 47 requests failed, 2 requests cancelled, 56 requests recovered after retry

Review planning (27 requests):
- apps/web/index.html,apps/web/package.json,apps/web/src/App.tsx,apps/web/src/DevMarkdownPage.tsx,apps/web/src/NotFoundPage.tsx,apps/web/src/router.tsx,apps/web/src/routes.tsx,apps/web/src/styles.css,apps/web/tsconfig.json,apps/web/vite.config.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/administration/AdministrationPage.tsx,apps/web/src/administration/administration-u.css,apps/web/src/analytics/AnalyticsPage.tsx,apps/web/src/analytics/analytics-u.css: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/agent-marketplace/AgentVersionActions.tsx,apps/web/src/agent-marketplace/TenantReleaseReview.tsx,apps/web/src/agent-marketplace/am-u.css: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/agents/AgentEditorModal.tsx,apps/web/src/agents/AgentParserRules.tsx,apps/web/src/agents/AgentsPage.tsx,apps/web/src/agents/MbtiTestModal.tsx,apps/web/src/agents/PersonaSection.tsx,apps/web/src/agents/SubagentsSection.tsx,apps/web/src/agents/agents-u.css,apps/web/src/agents/agents.css,apps/web/src/agents/agents.td.css: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/career/ApplicationPage.tsx,apps/web/src/career/ExportDeletionPage.tsx,apps/web/src/career/SubmissionPage.tsx,apps/web/src/career/application.css,apps/web/src/career/export-deletion.css,apps/web/src/career/submission.css: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- ... and 22 more

Core review (74 requests):
- apps/embed/src/EmbedApp.tsx,apps/embed/src/button.tsx,apps/embed/src/styles.css: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/miniprogram/config/index.ts,apps/miniprogram/package.json,apps/miniprogram/src/platform/files.ts,apps/miniprogram/src/subpackages/execution/artifact/index.config.ts,apps/miniprogram/src/subpackages/execution/artifact/index.tsx,apps/miniprogram/src/subpackages/execution/artifact/t-button.d.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/miniprogram/src/adapters/career-platform.ts,apps/miniprogram/src/career/application-material.config.ts,apps/miniprogram/src/career/application-material.tsx,apps/miniprogram/src/career/discovery.config.ts,apps/miniprogram/src/career/discovery.tsx,apps/miniprogram/src/career/export-deletion.config.ts,apps/miniprogram/src/career/export-deletion.tsx,apps/miniprogram/src/career/progress-preparation.config.ts,apps/miniprogram/src/career/progress-preparation.tsx,apps/miniprogram/src/services/career.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/miniprogram/src/app.config.ts,apps/miniprogram/src/app.scss,apps/miniprogram/src/core/errors.ts,apps/miniprogram/src/core/routes.ts,apps/miniprogram/src/features/home/pages.tsx,apps/miniprogram/src/services/workbench.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/miniprogram/tests/application-material.test.mjs,apps/miniprogram/tests/career-discovery.test.mjs,apps/miniprogram/tests/career-platform.test.mjs,apps/miniprogram/tests/export-deletion.test.mjs,apps/miniprogram/tests/progress-preparation.test.mjs: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- ... and 69 more

Context compaction (4 requests):
- apps/web/src/organizations/OrganizationsPage.tsx,apps/web/src/organizations/SpaceAvatar.tsx,apps/web/src/organizations/org-u.css,apps/web/src/organizations/orgs.td.css: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/settings/TenantDeleteZone.tsx,apps/web/src/settings/TenantMembersPanel.tsx,apps/web/src/settings/TenantUserProfileSections.tsx,apps/web/src/settings/UsagePanel.tsx,apps/web/src/settings/settings-toast.tsx,apps/web/src/settings/settings-wrapper.css,apps/web/src/settings/settings.td.css,apps/web/src/settings/surface.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> cancelled
- internal/modules/career/career_export.go,internal/modules/career/model_input.go,internal/modules/career/opportunity.go,internal/modules/career/profile_intake.go,internal/modules/career/reminder.go,internal/modules/career/rendering.go,internal/modules/career/resource_recovery.go,internal/modules/career/resume_extract.go,internal/modules/career/search_once.go,internal/modules/career/search_rule.go: cancelled
- apps/web/src/faq/FAQPage.tsx,apps/web/src/faq/faq.td.css: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
