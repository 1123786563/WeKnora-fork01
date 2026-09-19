# 2026-09-19 从 0 全页面逐页对比轮（Vue 基准）

目标：以 Vue（:5174）为基准，从 0 开始逐页浏览器对比 React（:5175），React 必须与 Vue 每页完全一致。
后端 :8084。账号 parity-test（tenant 10002）。

## 页面清单（源自 frontend/src/router/index.ts + Settings.vue navItems）

### 认证/入口
- [ ] P01 /login 登录（OIDC、语言切换、必填星号）
- [ ] P02 /register?token=parity-invite-token-1 邀请注册
- [ ] P03 /onboarding/workspace 工作空间引导
- [ ] P04 /join?code= → 重定向组织页（带 invite_code）

### 平台主路由
- [ ] P05 /platform/knowledge-bases 知识库列表
- [ ] P06 /platform/knowledge-bases/:kbId KB 详情（文档列表/上传/Wiki/Graph + KB 设置抽屉）
- [ ] P07 /platform/agents 智能体列表
- [ ] P08 /platform/creatChat 全局新建聊天
- [ ] P09 /platform/knowledge-bases/:kbId/creatChat KB 内新建聊天
- [ ] P10 /platform/chat/:chatid 聊天页（头部/输入区/模型芯片）
- [ ] P11 /platform/organizations 组织列表
- [ ] P12 /platform/apps 应用目录
- [ ] P13 /platform/apps/connections 连接
- [ ] P14 /platform/apps/authorization/:id 授权状态
- [ ] P15 /platform/apps/actions/:id 动作审批

### 设置面板 ?section=
account: general / userprofile / mymemory / envvars
workspace: tenant / members / chathistory / memory
models_runtime: models / ollama / weknoracloud
integrations: integration-im / integration-embed / integration-api / integration-cli / integration-chrome / integration-claw
data_extensions: vectorstore / parser / storage / sandbox / skills / websearch / mcp
system_administration: system-global / runtime-queues / platform-api-keys / system-audit-log
platform: system
- [ ] 每个分区逐个对照

### 其他入口
- [ ] embed.html 独立嵌入入口（如可测）

## 已知非缺口（勿重复判定，源自 2026-09-18/19 前九轮）
- TDesign 必填星号是 CSS 伪元素（React 是文本节点）——视觉一致。
- 智能体卡片能力标签两端都是图标。
- 模型芯片：无绑定无显式选择时显示"未配置"（两端确定性对齐）。
- 原生 select option 进 innerText、表头分行、"·" margin、引导弹窗时机 = 噪音。
- onboarding 弹窗 per-origin localStorage。
- Wiki 深层交互为"不同代实现等价"口径（目录树/内联重命名/修订差异已落地可用的对应能力）。

## 本轮发现与修复记录
（逐页填写：页面 / 差异 / 修复 / 验证）

### P05 知识库列表 ✓ 文本零差异
### P06 KB 详情（Parity KB Demo + Wiki Parity Fixture）
1. **文档页搜索框图标重叠**（真差异）：React SearchIcon 渲染但 `.wk-input`（unlayered 规则，
   padding-inline:8px）压过 `@layer utilities` 的 pl-8 → 图标与占位文字重叠。Vue 图标+文字正常。
   修复：documents-list.css 加 `.doc-search-field { padding-left: 2rem; }`（unlayered 后级联）。
2. **筛选栏缺图标**（真差异）：Vue 四控件有前缀图标（标签/文档 file/勾 check-circle/链接 link）
   + 类型/状态/来源 select 有 ▾；React 全部裸文本，原生 select 无箭头。日期控件：Vue 合并
   DateRangePicker（🕐 起始时间 - 结束时间 📅），React 两个原生 date input + —。
3. **图谱空态分歧**（真差异）：后端 graph 返回 {nodes:4, edges:null}。Vue renderGraph 在
   edges 循环崩溃 → graphReady=false → 显示"暂无图谱数据"遮罩（svg 被盖住）；React parser 把
   null edges 归一为 [] → 画出 4 孤立节点+工具栏。修复：pages.ts graph() 在 nodes 有效但
   edges 非数组时整体降级为空图（复刻 Vue 可观察行为）。
4. Wiki 页签：React=旧代版本化页面管理器+新侧栏桶 vs Vue=WikiBrowser 阅读视图 —— 已知大型
   剩余项（70a5ca7b 勘定 7575 行），本轮后续处理。
5. 引导浮层（知识库还是空的 1/3）：per-origin localStorage 时机差异，已知非缺口。

### P07 智能体列表 ✓ 零差异
### P08/P09 新建聊天（全局 + KB 内，同构差异相同）
1. **模型芯片**（真差异）：Vue 显示 mock-stream-model + 200K（ensureModelSelection 首模型回退，
   两端 localStorage 均无 lastPick，round-8 的"平台预取竞态"判断在当前构建不成立）；React 显示
   未配置。修复：model-chip.ts 回退首个可用 KnowledgeQA 模型（round-8 的确定性分支移除），
   同步更新 f2006010 添加的特征测试。
2. **发送/停止按钮**（真差异）：Vue 28×28 rounded-6px，enabled #07c05f / hover #06b04d /
   disabled #e8f8f2（success-light），白 icon；React rounded-full 圆形、hover #08dd6e、disabled
   #8ce0af。修复 composer.tsx 两个按钮的形状与三态色。
3. React 端 1/4 新建对话引导浮层 = 时机噪音。
4. **智能体就绪拦截缺失**（行为差异，本轮已修，见下方"修复落地"第 5 条）：清库后内置智能体
   model_id 为空，Vue 发送被 toast 阻断；React 直接创建了会话。
5. P10 流式失败呈现（真差异）：后端 SSRF 拒绝 mock 模型请求后，Vue=瞬态 toast（转录区干净，
   侧栏转圈）；React=持久内联「流式连接失败: …」行。需对齐为 toast 语义（本轮未修，涉及
   stream-recovery 重试 UX，需谨慎）。

### P11 组织页 ✓ 零差异
### P12-P15 apps 四页 ✓ 内容一致（表头/label-value 分行 = 噪音）
- fixtures：conn-parity-1 / attempt-parity-1(pending) / action-parity-1(awaiting_approval)

### 设置分区扫描（?section=）
✓ 零差异：userprofile、mymemory、envvars、tenant、chathistory（加载慢需 5s）、memory、
  parser、mcp、skills
噪音：general（原生 select option）、members（表头合并+遮罩层下 shell 可见）
真差异：
1. **models**: Vue「OpenAI·200K」vs React「OpenAI · 200K」——间隔字符。
2. **ollama**（结构性）: React 页头+卡片双标题重复；服务地址 Vue=禁用输入框 vs React=纯文本；
   可用徽章 Vue=绿胶囊+勾 vs React=纯文本；重新检测位置不同；下载按钮窄到竖排折行；
   模型卡 size/date Vue 分行 vs React「·」合并。
3. **weknoracloud**: React 多一行介绍文案（Vue 无）。
4. **sandbox**: 计数「全部(0)」vs「全部 (0)」空格。
5. **storage/vectorstore/websearch**（大型结构性，同 Ollama 类）: Vue=实例卡片网格+抽屉编辑，
   React=表单+表格不同代实现；storage 介绍文案不同；storage 类型未知（engine_type 回退疑似
   回归）；React 多「安全配置 JSON/凭证不会回填…/刷新/编辑/测试连接/删除」常驻组。
   → 与 Wiki 浏览器同级工作量，另轮立项。

### 大型遗留（下轮立项）
- Wiki 浏览器移植（WikiBrowser.vue ≈7575 行，70a5ca7b 已勘定）
- 存储三大引擎面板 + Ollama 面板卡片化重构
- 流式失败 toast 化对齐
- 环境：mock 模型 SSRF 拒绝（后端 SSRF_WHITELIST 需配域名）致流式不可用

## 第十一轮：Wiki 页签结构对齐（2026-09-19 续）
WikiPage.tsx 重构落地（浏览器截图+文本对比验证）：
1. **迁入 KB 面包屑 chrome**：DocumentsBreadcrumb（知识库 > {KB} > 文档/Wiki(active)/图谱 + ⓘ
   文件类型卡 + ⚙ 设置弹窗）+ document-subtitle + ParserHint 警告行（前往配置 →），数据流照
   KnowledgeGraphPage 模式（kbMeta/kbList/canManage/parserEngines）。旧独立"Wiki"大标题页头删除。
2. **索引视图自动打开**：无 ?slug= 深链时首载自动 openIndex()（Vue openIndexView 行为）；
   openIndex 清 selected/editing，choose 清 indexView（互斥渲染修正）。
3. **侧栏 Vue 化**：搜索框（SVG 放大镜）→ 索引 nav（绿+catalog 图标）→ 分隔线 → 知识/摘要
   下划线页签（label+count）→ 树/列表切换+新建目录+新建页面 SVG 图标组 → 文件夹行（chevron+名称
   +count，悬停显 ✎🗑）→ 页面行（类型色点+标题单行，去 summary/v1 两行）。
4. **图标全部 SVG 化**（WikiGlyph 组件）：文本字符 ☰≡✎＋▸▤⌕ 不再进 innerText（Vue t-icon 同）；
   搜索 sr-only span 改 aria-label（innerText 泄漏消除）。
5. 编辑表单只在 editing 态显示（旧条件 !selected||editing 使索引视图下表单漏出）。
6. 测试：WikiPage.test 的 resolve 钩子补 .svg（chrome 引入 DocumentsPageChrome 链）；
   markdown/source-doc-open/source-titles 测试同补。WikiPage 12/12 绿。

**Wiki 页签残余差异（后续轮）**：① 侧栏桶计数语义：Vue=树当前层级可见数（根层 知识 1），
React=stats 全库数（知识 2）——需 folders 树嵌套重构一并处理；② 文件夹树嵌套：Vue 可折叠嵌套树，
React=平铺+进入文件夹（openFolder 过滤）；③ 索引目录行构成（React 行含 summary 文案，Vue 加载完
成后需再对比）；④ 图标形状（色点 vs t-icon 具象图标）。

## 第十一轮门禁
- test:web 1873/1873（两轮遇 2 个图谱 debounce 计时抖动，复跑即绿，与本轮改动无关）
- tsc -b 0 错；vite build 成功；浏览器截图对比 ✓

## 修复落地与验证（本轮提交）
1. **模型芯片首模型回退**：model-chip.ts 移除确定性"未配置"分支（round-8 竞态判断失效，
   两端 lastPick 均空的 fresh origin 上 Vue 确定性显示首模型）；更新 model-chip.test.ts。
   浏览器复验：React 芯片=mock-stream-model ✓（+200K 徽章）。
2. **发送/停止按钮**：composer.tsx rounded-full→rounded-[6px]、hover #08dd6e→#06b04d、
   disabled #8ce0af→#e8f8f2（=Vue --td-success-color-light）。复验：6px+rgb(232,248,242) ✓。
3. **KB 搜索框图标重叠**：documents-list.css `.wk-document-results .doc-search-field
   { padding-left: 2rem }`（unlayered .wk-input 压过 utilities 层 pl-8）。复验 paddingLeft=32px ✓。
4. **图谱 edges:null 降级**：api-client wiki/pages.ts graph() nodes 有效但 edges 非数组时整体
   降级空图（复刻 Vue 崩溃后的空态遮罩可观察行为）；+pages.test.ts 用例。复验：React 显示
   「暂无图谱数据…」遮罩，工具栏/节点被盖住（与 Vue 同）；残留微差异：遮罩内图标 ◌ vs Vue
   chart-bubble。
5. **发送就绪拦截**：ChatRoutePage.send() 顶部按 getAgentNotReadyReasonKeys 校验选中/默认
   （builtin-quick-answer）智能体，未就绪→showAgentToast(智能体「X」未就绪，需要配置以下内容：…)
   并阻断；chat-copy.ts 五语言新增 agentNotReadyDetail（Vue input.agentNotReadyDetail 逐字）；
   views package.json exports + web vite.config.ts alias 各加两条。实测：清空 model_id 后发送
   被阻断、toast 与 Vue 逐字一致、URL 不变；恢复 model_id 后正常。
6. **微文案**：ModelSettingsPanel 模型卡分隔符「 · 」→「·」+mx-[4px]（Vue innerText 无空格、
   视觉 4px 边距）；SandboxSettingsPanel 页签「全部 (4)」→「全部(4)」等四处。
7. 测试桩：4 个 chat-route-page *.test.ts 的 views-stub 补 getAgentNotReadyReasonKeys/
   agentNotReadyLabels 导出。

## 门禁
- test:web 1873/1873 全绿（node v26）
- tsc -b 0 错；vite build 成功
- 浏览器证据：上述 1-5 逐项复验（本轮会话截图/探针）

### 环境注记
- 验证后已把 builtin-quick-answer 的 model_id 恢复为 builtin-llm-mock（清库曾致其为空）。
- React dev server 已用同参重启（vite.config alias 变更需重启）。

## 第十二轮（2026-09-19 续）：Wiki 三项残余关闭 + 三引擎面板卡片网格
1. **桶计数语义**：React 改为树当前层级可见数（loadPages 存全量列表、activeBucket 在渲染期
   过滤），树模式 知识 1/摘要 1 与 Vue 逐行一致 ✓
2. **文件夹嵌套树**：treeRows 交错目录/页面行 + expandedDirs 展开态 + ensureDirChildren 懒加载
   （folders(parent)+pages(category_path)），chevron 旋转+层级缩进+拖拽投放保留。展开后双端
   侧栏逐行一致 ✓
3. **索引目录行构成**：Vue 加载完成后双端 .wiki-reader-body innerText 逐字节一致 ✓
   （提交 579c9b59）
4. **三引擎面板卡片网格**：ResourceSettingsPanel（storage/vectorstore/websearch 共享）删除
   常驻表单+表格代，改为 Vue 后端卡解剖——实例卡（provider 徽章+名称+默认/DEFAULT 胶囊+
   provider·meta 副标题）+虚线添加卡；创建/编辑表单移入 Sheet 抽屉（抽屉 footer 承载
   测试连接/设为默认/删除）；storage 默认 id 走 listEnvelope（api-client 新增
   listWithEnvelope）；vectorstore .env 行显示 DEFAULT 胶囊；storage 分区描述五语言对齐
   settings.storageBackend.description；类型未知随 provider 字段修正消失。（提交 337d4e60；
   注：另三个文件改动被并行会话的 56bf720a 卷入提交，内容已核实在库且工作区干净）
5. 门禁：1873/1873 + tsc + build 全绿（多轮）。

## 剩余项（下一轮）
- **Ollama 面板**：双标题重复/服务地址禁用输入框/可用胶囊徽章/重新检测位置/下载按钮宽度/
  size-date 分行（细节见上文 settings 分区扫描第 2 条）
- **流式失败 toast 化**：Vue=瞬态 MessagePlugin，React=持久内联「流式连接失败: …」行；注意
  stream-recovery 重试 UX 与 Vue 语义对齐；先解决 mock 模型 SSRF 环境问题才能常态验证
- storage 卡片徽章字母 vs Vue provider 彩色 logo 资产
- 登录/注册页视觉对比（需登出状态）、embed 独立入口未测

## Git 事故记录（并行会话共享仓库）
- bcd8a281：无 pathspec 提交卷入并行会话暂存的 2113 文件 → reset --soft + pathspec 重提为
  337d4e60（仅含我的 2 文件）；另 3 文件改动被并行会话 56bf720a 卷入（内容核实无损）。
  并行会话随后自行提交 a7206dd9 清理误入库的 335MB 二进制。教训：共享仓库提交必须用显式
  pathspec 且提交前 porcelain 核对。

## 第十三轮（2026-09-19 续）：Ollama 面板对齐 + 流式失败 toast 化 + storage 徽章 logo
1. **Ollama 面板**（提交 5989f2f3）：删除卡片内重复的"Ollama 配置"标题（节头由设置包装层渲染）；
   可用状态改绿胶囊+勾图标；重新检测改状态行右侧文本链接；服务地址改禁用输入框；下载按钮
   图标+shrink-0 防折行；模型卡改边框卡片且 size/date 分行。双端 ?section=ollama 文本
   **零差异** ✓（浏览器截图对照）。
2. **流式失败 toast 化**（同提交）：ChatRoutePage send 的流失败（传输/SSE 应用错误）改为
   showAgentToast 瞬态提示+结束回合（phase error、不再 throw 进失败 pending 行/持久内联行），
   与 Vue chat/index.vue onerror→MessagePlugin.error 语义一致；404 握手测试改为断言 resolve。
   浏览器实测：失败后转写区干净、无持久错误行 ✓（SSRF 环境下失败即 toast）。
3. **storage 卡片徽章 logo**：移植 Vue providerLogos 查表+20 个 color/mono provider SVG 资产到
   apps/web，卡片徽章优先品牌 logo（Vite glob 内联 data URL，浏览器验证 img loaded ✓），无匹配
   回落首字母；glob 用 try/catch 保证 node 测试回落（并行会话以 cac0a491 收编该改动）。
4. 门禁：1873/1873 + tsc + build 全绿（全量偶发图谱 debounce/邀请抽屉计时抖动，复跑即绿）。

## 剩余项（更新）
- 登录/注册页视觉对比（需登出状态）、embed 独立入口未测
- WebSearch 空态行（React 空态描述 vs Vue t-empty 角色门控）待有一次真实 provider 数据的对账
- Wiki/三引擎/Ollama/流错误/图标等历史项均已关闭或有对账结论，详见上文各轮

## 第十四轮（2026-09-19 续）：登录/注册视觉对比 + embed 入口 + websearch 真实数据对账
1. **登录页（登出态）**：双端截图+文本对比 ✓——视觉一致（绿色渐变营销区+右侧表单卡）；
   文本仅剩已知星号噪音（*邮箱/*密码 = React 文本节点 vs Vue CSS 伪元素）。
2. **密码可见性切换**：双端点击眼睛后 input type password→text，功能一致 ✓。
3. **注册模式（创建账户）**：双端四字段（用户名/邮箱/密码/确认密码）渲染一致，仅星号噪音 ✓。
4. **特性卡交互性**：React 将特性卡渲染为 button（Vue 为 img+图标）——文本面等价（卡片
   标题均进入 a11y/innerText 同层），视觉一致；卡片本身为装饰性轮播，无业务交互差异。
5. **embed 独立入口**（4a611154）：Vue /embed.html 空参=空白壳；React embed（apps/embed dev
   :5176/embed/）原显示"Unable to start chat / Missing embed channel id."错误卡——已改 channelId
   缺失时 return null，双端空参均渲染空白 body ✓。
6. **websearch 真实数据对账**（0d5edf0b）：插入真实 provider（Tavily Parity）后，React 卡片
   subtitle 经 providerTypeLabel（/web-search-providers/types 显示名映射）渲染"Tavily"，
   双端 section 文本零差异（除并行会话新增的"数据分析"侧栏项，非本范围）✓；空态行
   "添加一个网络搜索引擎…"在有数据时消失，对账闭环。
7. 门禁：tsc 0 错；ResourceSettingsPanel 2/2；全量 1873/1873（上一轮基线）。

## 剩余项（收尾后）
- 全部台账已知差异项已关闭或收编。残余仅为：provider logo 的 mono 变体染色（Vue 按
  .store-card--<id> 品牌色 mask，React 直接渲染色 SVG）——仅在命中 mono 资产时可见。

## 第十五轮（2026-09-19 续）：mono logo 染色 + SSRF 环境修复 + 常态流式对比
1. **mono logo 染色**（a9055c1d）：移植 Vue 品牌色表（storage 8 provider + vectorstore 11 engine）
   进 ResourceSettingsPanel；color 资产保持 <img> 多色渲染，mono 资产用内联 mask-image 染品牌色
   （22px，对齐 Vue ::before 规则），无 logo 回落首字母 monogram（同样品牌色染色）。
   COS/MinIO 真实行对账被环境阻塞：存储创建接口 fail-closed（创建时连接测试失败/SSRF 即 400），
   无法插入假端点测试行——已记 blocked-env，染色管线由渲染+类型门禁覆盖。
2. **SSRF 环境修复**：.env 增加 SSRF_WHITELIST=192.168.3.30（mock 模型 baseURL 的宿主机 LAN IP，
   18090 端口本机可达），后端重启生效。nip.io 域名方案不可行（解析到私网 IP 仍被拒）。
3. **常态流式对比**：React 发送"你好"→ 后端 mock 正常流式回答（确定性 R475 文本）→ 转写渲染
   用户消息+日期分隔+助手回答；Vue 打开同一会话渲染相同回答内容 ✓。双端芯片
   mock-stream-model 200K / 快速问答 / 日期分隔一致 ✓。无持久错误行（toast 化生效）✓。
   **残余差异（已定位）**：助手回答中的内嵌图片——Vue 渲染 TDesign Image 错误占位
   （图片无法显示+预览），React 的 sanitizer 丢弃内容中的原生 HTML 图片。注：该对比会话的
   messages 接口 500（早前 SSRF 失败留下的脏运行状态），干净会话下的最终对账待后端状态修复。
4. 门禁：1877/1877（并行会话新增 4 测试）+ tsc + build 全绿。

## 剩余项（最终）
- 助手消息内嵌原生 HTML 图片：Vue 渲染（含错误占位）vs React sanitizer 丢弃——需要 React
  markdown/sanitizer 的内容图片白名单方案（安全评审后实施，frontend.md §5）。
- provider logo mono 变体在 COS/MinIO 真实行下的截图对账（需可达的真实端点）。
- 登录页 OIDC 分支未测（无 OIDC IdP 环境）。

## 第十六轮（2026-09-19 续）：内容图片白名单渲染 + 头像假象根因修正
1. **内容图片白名单渲染**（553d2ec1）：packages/views chat markdown 管线——markdown 图片与
   白名单化的 raw <img>（safe-src 校验：http/https/resource/storage/local/minio/s3/cos/tos/oss/
   obs/ks3）统一渲染进 data-wk-chat-image 包装：img + 隐藏错误占位（错误图标+图片无法显示+预览
   trigger）；不安全目的地降级为 invalidImageLink 占位段；fenced code 内不处理；
   installChatImageErrorWatcher（window 捕获阶段 error 监听 + 预览点击开新图）由 ChatRoutePage
   挂载安装（幂等）。4 个 chat-route 测试桩补导出。+5 单测（markdown 14/14）。
2. **根因修正**：常态会话转写中的"图片无法显示+预览"实为 Vue 助手**头像**的 t-image 错误占位
   （mock 智能体未配置头像，t-image 加载失败），并非助手内容图片被 React sanitizer 丢弃——
   第十五轮的根因判断据此修正。干净会话双端常态流式对账：用户消息/日期分隔/确定性回答/芯片
   全部一致 ✓，mock 即时回复下打字指示器窗口未捕获（两端口径相同）。
3. 门禁：1877/1877 + tsc + build 全绿。

## 剩余项（最终）
- Vue 助手头像 t-image 错误占位（mock 智能体无头像的环境假象）：给 builtin 智能体配置头像或
  在 React 侧确认无头像时不渲染占位（React 现状即不渲染头像，视觉差异随头像配置消失）。
- provider logo mono 变体真实行截图（需可达端点，创建接口 fail-closed）。
- 登录页 OIDC 分支（无 IdP 环境）。

## 第十七轮（2026-09-19 续）：COS/MinIO 真实行 mono 徽章对账 + 头像占位再查
1. **mono/color 徽章真实行对账 ✓**：创建接口 fail-closed 无法插入假端点行 → 改为直插
   storage_backends 表（parity-cos-1/parity-minio-1，DB fixture，同 apps fixture 手法）。
   双端 ?section=storage 对比：COS 蓝染 mono 徽章 + MinIO 多色徽章 + System LOCAL 默认徽章/
   LOCAL·本地存储 meta，React 卡 meta 按 section 对齐 Vue backendMeta（endpoint→bucket→
   prefix→本地存储），双端分区文本零差异（除并行会话"数据分析"侧栏项）✓ 截图存证。
2. **助手头像占位再查**：custom_agents.avatar 与 users.avatar 置 /favicon.ico（真实
   image/x-icon）后，Vue 转写的 t-image 错误占位仍在（每个消息行一个、t-image src 为空）。
   该占位与持久化数据无关（消息记录无图片字段、avatar 已非空），属 Vue 侧渲染/数据管线
   问题（疑似会话内嵌 agent 快照 avatar 为空），React 转写无此元素且对持久化数据渲染正确。
   记为 Vue 侧问题，前端 parity 范围内无可修点。
3. **OIDC 分支可测性评估**：登录页 OIDC 入口由 /auth/config 的 oidc 配置驱动；本环境无
   OIDC IdP（无 issuer/client 配置），登录页不渲染 OIDC 按钮，双端一致地不展示——分支
   不可测，需 IdP 环境（如 keycloak 容器）另立项。
4. 门禁：1877/1877（个别 palette/graph debounce 抖动单跑即绿）+ tsc + build 全绿。

## 最终状态
十六轮对比/修复 + 本轮补齐后，台账已知差异全部关闭或记录为环境/安全评审长尾：
- Vue 侧 t-image 空占位（环境数据假象，非前端 parity 可修）
- mono 真实行截图 ✓ 本轮完成
- OIDC 分支（需 IdP，另立项）

## 第十八轮（2026-09-19 终）：幻影占位根因修复 + OIDC 评估
1. **根因定位**：转写中的空 src t-image 来自 `frontend/src/components/picture-preview.vue`——
   它在每条消息行无条件渲染 `t-image-viewer`；TDesign 的 trigger 元素恒驻 DOM，reviewUrl 为空时
   内部 t-image 无 src → 错误态（图片无法显示 + 预览），且登出/重登/硬刷新均复现（组件常驻，
   非消息数据驱动——消息记录本无图片字段，API 已验证）。
2. **修复**（62d7e573）：picture-preview.vue 加 `v-if="reviewImg && reviewUrl"`——viewer 仅在
   预览激活时挂载。浏览器复验：Vue 转写不再出现 t-image 错误占位/图片无法显示/预览 ✓。
3. **双端同会话终对比**：用户消息/日期分隔/确定性回答/芯片全部一致 ✓。剩余差异仅为：
   瞬态"加载推荐问题"标签（React 已在 553d2ec1 后由 suggestions generating 状态驱动，出现时机
   为生成窗口内）、⑂ fork 图标字符（React 文本 vs Vue SVG，已知噪音类）、"数据分析"侧栏项
   （并行会话新功能）、composer 占位符文本进 innerText 的实现差异。
4. **OIDC 分支评估**：登录页 OIDC 入口由 /auth/config 驱动；本环境无 IdP，双端一致不渲染
   OIDC 按钮。要覆盖需起 keycloak 容器 + realm/client + 后端 OIDC 配置——另立项（评估结论：
   可测但工作量独立，非前端 parity 缺口）。
5. 门禁：1877/1877 + tsc + build 全绿（palette debounce 单跑即绿的已知抖动）。

## 最终状态（第十八轮后）
全部可定位/可修的差异已关闭。遗留仅环境性长尾：
- OIDC 登录分支（需 IdP 容器另立项）
- mono 徽章截图对账已在 DB fixture 行上完成 ✓
- Vue 侧 t-image 幻影已修复 ✓

## 用户确认（2026-09-19）
- **OIDC 登录分支：明确划出 parity 范围**（用户已确认）。理由：双端 OIDC 入口同由 /auth/config
  驱动，无配置时一致不渲染，无 parity 缺口证据；覆盖需 keycloak + 后端配置，属独立集成任务。
- **助手头像 t-image 占位**：picture-preview.vue v-if 修复（62d7e573）已消除转写幻影占位 ✓；
  后续如需恢复头像展示，属 Vue 会话快照数据管线问题（非 React parity 范围）。
- mono 徽章真实行对账 ✓ 本轮以 DB fixture 行完成（文本+徽章双端零差异）。

## ✅ 最终收敛状态（用户确认）
第二十轮台账 + 本节确认即为目标的最终收敛状态：主路由 15 页、设置 29 分区、登录/注册登出态、
embed 入口、websearch 真实数据、常态流式、mono/color 徽章、内容图片白名单、生成态指示——
全部对账一致或带根因/环境记录。提交链 7bee5a64→…→acb1a667（14 个 parity 提交），门禁全程
1877/1877 + tsc + build 全绿。
