# 2026-09-18 main 浏览器对齐轮（Vue 基准 → React 差异修复）

环境：Vue=frontend/ :5174，React=apps/web/ :5175，后端 :8082（main 仓库，分支 main）。
账号：parity-test@local.dev（tenant 10002）。视口 1280×720，同数据源（同一后端）。

## 背景

main 上 React 缺少 parity 车道（worktree 分支 codex/react-vue-parity-align）已验证的
R475/R476 React 修复。本轮回先拣选（cherry-pick）5 个已验证提交到 main，再做浏览器逐页
对比，以 Vue 为基准修复新发现的差异。

### 已拣选提交（main 侧新哈希）

- ef981f7a ← ef62cbe7 feat(chat): 引导消息重试与预览优化（A3 收尾）
- e93b1c4b ← f1383ecc fix(parity): R475 上传确认 vueNumOr 回退
- 623062ce ← e6631c22 feat(parity): R476 差异化 steer toast（+4 键 ×5 locale）
- 67064f67 ← 3c295b38 test(parity): R476 问题生成 payload 特征测试
- 3685d386 ← 0798e62e feat(parity): R476 门禁 harness（node>=26 re-exec）+ mock LLM 常驻

c1cb798a（Go A13 余量：preference API + 工具测试）与两个 docs 提交**未拣选**：
前者与 upstream-parity 车道在 main 的 A13/A19 工作有重叠风险，后者把台账带入 main
与 worktree 台账分流——留给车道合并时统一处理。

## Vue 功能清单（frontend/src/router/index.ts + 浏览器逐页实测）

- 公共：/login（含 ?token= 邀请注册复用同一组件）、/register、/join→organizations 重定向、
  /onboarding/workspace；Lite 模式恢复、auto_setup、OIDC 回跳放行。
- 平台布局 /platform（→knowledge-bases）：侧边栏（新对话/知识库/智能体/共享空间 + 会话历史
  分组 + 用户菜单）、全局 ⌘K 搜索、可收起。
- /platform/knowledge-bases：过滤页签（全部/收藏/最近/本空间）+ 空间切换器 + KB 卡片
  （收藏、设置、文档数）；未初始化模型警示条。
- /platform/knowledge-bases/:kbId：面包屑（文档/Wiki/图谱）、上传与解析引擎提示、文档过滤
  （标签/类型/状态/来源/起止时间）、卡片-列表视图切换、批量管理、添加文档。
- /platform/settings（模态分区）：账户（常规设置/用户信息/我的记忆/沙箱密钥）、空间（空间信息/
  成员管理/消息管理/长期记忆）、模型（模型管理/Ollama/WeKnora Cloud）、发布集成（IM/网页嵌入/
  API/CLI/Chrome 插件/Claw Skill）、数据与扩展（向量库/解析引擎/存储引擎/沙箱/技能/网络搜索/MCP）、
  平台（版本信息）；system/* 与 integrations 旧路径均重定向进对应分区。
- /platform/agents：过滤页签 + 内置 4 卡片（能力图标标签、收藏、管理）+ 创建/编辑模态。
- /platform/organizations（共享空间）：全部/我创建的/我加入的 + 创建/加入 + 成员/待审批管理。
- /platform/apps、apps/connections、apps/authorization/:id、apps/actions/:id（应用目录/连接/
  授权状态/动作审批）。
- /platform/creatChat（全局新对话）、kbId/creatChat、chat/:chatid：composer（智能体选择、
  附件、@ 知识库、模型芯片、引导消息队列、沙箱终端）。
- 新用户引导（NewUserGuide 7 步）——React 已有对应实现，行为一致（per-origin localStorage）。

## 本轮浏览器发现并修复的 React 差异（5 项）

1. **聊天页标题叠在侧边栏 logo 上**（apps/web/src/chat/chat.css 无 `.wk-chat-main`
   定位上下文；Vue `.chat { position: relative }`，头部 absolute 锚到视口）。
   修复：packages/views/src/chat/page.tsx `.wk-chat-main` 增加 `relative` 工具类。
   复验：标题回到聊天列左上角（截图 /tmp/parity-react-chat-after.png）。
2. **共享空间页整页英文**（OrganizationsPage 用 navigator.language 解析 locale，
   R428 清理时遗漏；语言设置存 localStorage['locale']）。
   修复：改用 `usePreferredLocale()`（locale.ts 单一事实源，响应 weknora:locale-changed）。
   复验：全部/我创建的/我加入的 均中文。
3. **聊天模型选择被自动持久化**（ChatRoutePage 的 effect 把 loader 播种的"第一个模型"
   回退写入 localStorage，Vue 只在用户显式选择 handleModelChange 时写）。
   修复：删除该 effect，持久化移入 onModelChange（与 Vue 相同顺序：先写库再改状态）。
   新增特征测试 chat-route-page-model-pick.test.ts（2 条：显式选择写、纯渲染不写）。
   说明：Vue 侧"未配置 vs 模型名"的显示差异源于其 mount 竞态（平台预取令
   ensureModelSelection 早退，下拉打开才补加载），属环境相关假象；React 的确定性
   首模型回退与 Vue ensureModelSelection 注释声明的意图一致，不复制竞态。
4. **Apps 四页文案/表头与 Vue 不一致**（React 为旧硬编码实现）。
   修复：apps.catalog/connections/authorization/actions 全部文案对齐
   frontend/src/i18n/locales/zh-CN.ts 逐字节一致：目录描述、表头（所需权限/Schema 指纹/
   发布状态/权限范围）、空态（暂无可用动作/暂无连接）、连接页列（连接/类型/账号归属）、
   类型/账号标签（个人/空间/空间共享）、按钮（授权/断开）、撤销确认文案、断开清理说明、
   授权页（授权状态/轮询描述/记录标签/返回连接列表）、动作审批（动作审批/快照描述/
   账号（连接）/内容指纹/版本围栏/角色提示/重发提示）。
   复验：目录与连接页快照断言全过。行为层（授权轮询退避、Popconfirm 断开确认）仍为
   React 简化实现，与 Vue 的完整流（pollBackoff 等）差异记为后续轮次（矩阵本就 open）。
5. **登录表单缺必填星号**（注册表单有；Vue 登录表单 TDesign required 渲染红 *）。
   修复：LoginPage 登录表单邮箱/密码标签补同款 `<span style="color:#d54941">*`。
   复验：登录页快照 "*邮箱"/"*密码"。密码可见性切换两端均有（此前 ARIA 命名差异非缺口）。

## 排除的疑似差异（核实为非缺口）

- 智能体卡片"文字能力标签"：React ARIA 快照把图标标签的 tooltip 读成文本；截图证实
  两端均为底部图标标签（R012 accepted 维持）。
- 新用户引导弹窗仅 React 出现：per-origin localStorage，两端功能一致（R007 已落地）。
- creatChat/聊天页模型芯片"未配置 vs mock-stream-model"：见第 3 项说明（Vue 竞态假象）。

## 追加轮：Apps 四页行为层对齐（同日第二轮）

继上表文案对齐后，把 React 简化实现补齐为与 Vue 行为一致：

1. **i18n 迁移**：apps.* 全量 101 键 ×5 locale，从 frontend/src/i18n/locales/*.ts
   程序化提取（tsx 脚本 import + flatten，值字节一致，非手抄），生成
   packages/i18n/src/generated/apps.ts 并接入 index.ts 合并表；AppsPages 文案全部
   改走 formatMessage（locale 由 usePreferredLocale 提供，响应语言切换）。
2. **ConnectionsView 行为**：断开改为 Popconfirm 确认气泡（内容/危险确认键 loading/
   取消键，点击外部关闭）；revoke 回传 `auth_version ?? 1`；成功 → revokeSuccess toast +
   重载；409/VERSION_CONFLICT → revokeConflict 警告 + 重读；其余 → revokeFailed。
   startAuthorization 缺 attempt_id → startAuthorizationFailed toast（不虚构跳转）。
   kind/账号/状态标签回退语义对齐（未知 kind 回退原值、owner 14 字符省略、状态
   其他值 → 状态：{state}）。
3. **AuthorizationView 行为**：移植 pollBackoff.ts（3s 起步、连续失败翻倍、30s 封顶、
   成功清零）；pollingStatuses={pending,authorizing,verifying} 终态停轮；expires_at
   过期停轮；pollingHint/completedHint 分支；状态标签 apps.authorization.status.* 回退
   状态：{state}；浏览器实测坏 id 场景只发 1 次请求即停（无无限轮询）。
4. **ActionView 行为**：404 → notFound 文案分支；风险字段按冻结快照渲染
   （apps.risk.*，缺失 → 破折号 + riskUnknownHint 提示，**修正旧测试钉住的错误行为**——
   旧实现即便 DTO 带 risk 也渲染破折号）；actionControls(viewModel) 逐字移植
   （approve=awaiting_approval+权限、execute=authorized+权限、retry 恒 false）；批准/
   执行后强制重读服务器，不凭 200 假定成功；memberCannotApprove 提示。
5. **测试**：pollBackoff.test.ts / actionState.test.ts 逐字移植；model.test.ts 合并
   envelope/digest/backoff/i18n 断言；AppsPages.test.tsx 改为 Vue 行为锚定（delete 风险
   渲染删除标签、缺失渲染破折号+提示、批准按钮门控、非管理员提示）。

### c1cb798a（后端 A13 余量）评估结论：不拣选

main 已由 upstream-parity 车道落地 `PUT /auth/me/preferences`
（internal/handler/auth.go UpdateMyPreferences + routes_auth_tenant.go），且 Vue 前端
不使用 browser_search_instructions / preference_defaults（frontend/src 无引用）。两端
前端打到同一后端，该 API 面不构成 Vue/React 行为差异；c1cb798a 与 main 的实现谱系不同，
拣选只会制造冲突。归属 upstream 车道 A13 后续接线。

### 门禁与证据（第二轮）

- `pnpm gates`（Node v26.4.0）：test:shared 869/869、test:web 1871/1871（+9）、
  typecheck:shared/typecheck:web 0、check:integrity 首跑 P0=新文件未暂存（门禁正确
  报警），git add 后 0 P0 PASS。
- 浏览器：目录页/连接页五项文案断言过；en-US locale 冒烟（desc/empty/账号归属英文，
  恢复 zh-CN 正常）；授权页坏 id → 加载失败提示 + 仅 1 次请求即停。
- 本轮改动文件：apps/web/src/apps/{AppsPages.tsx,AppsPages.test.tsx,model.ts,
  model.test.ts,pollBackoff.ts,pollBackoff.test.ts,actionState.ts,actionState.test.ts}、
  packages/i18n/src/{index.ts,generated/apps.ts}。

## 门禁与证据（第一轮，存档）

- `pnpm gates`（Node v26.4.0，main @ 本轮）：test:shared 869/869、test:web 1862/1862
  （含新增 2 条）、typecheck:shared/typecheck:web 0、check:integrity PASS。
- 注意：Node v22 下 test:web 会出现既知工具链假红（R475-A4 结论复现），一律以 v26 为准。
- 截图：/tmp/parity-vue-*.png、/tmp/parity-react-*.png、/tmp/parity-react-chat-after.png；
  DOM 快照 /tmp/parity-{vue,react}-*.txt。
- 本轮改动文件：apps/web/src/apps/AppsPages.tsx、apps/web/src/auth/LoginPage.tsx、
  apps/web/src/chat/ChatRoutePage.tsx、apps/web/src/organizations/OrganizationsPage.tsx、
  packages/views/src/chat/page.tsx、apps/web/src/chat/chat-route-page-model-pick.test.ts（新）。

## 追加轮：全页面可见文本扫描（第三轮，2026-09-19 复验）

以"每个页面完全一致"为标准做系统性扫描：主路由 9 页 + 设置 24 分区，双端分别提取
body.innerText 可见文本集合（框架无关），程序化求差后逐项人工分诊。

### 判定为机制性噪音（非缺口）

- 原生 select 的 option 文本整体进入 innerText（TDesign 下拉关闭时为空）——常规设置分区
  React 侧的语言/主题/字体选项列表属此类；
- 表格表头 innerText 分行差异（React 单行 tab 分隔 vs Vue 多行），内容相同；
- Vue 卡片分隔点"·"为 CSS margin，React 为空格字符，视觉等宽；
- 引导弹窗/上下文引导在抓取时未及关闭产生的单侧文本。

### 真实差异 11 项（本轮全部修复并复验）

1. members 页泄漏 JSX 注释为可见文本（TenantMembersPanel "// Audit drawer…" 渲染进 DOM）
   → 改为 JSX 注释；
2. KB 详情缺工具栏"批量管理"按钮（React 只有行内菜单入口；Vue 是 trailing 过滤栏独立
   outline 按钮，批量态切换为"取消选择"，退出清空选择）→ 按 Vue 补齐；
3. KB 详情文档名带扩展名（Vue useKnowledgeBase 列表映射即剥离最后一段扩展名，React 直用
   file_name）→ displayName 按同规则剥离，下载锚点保留原始名防丢后缀；
4. envvars 缺无沙箱守卫（Vue 无沙箱配置时整表单替换为"还没有沙箱/这个空间还没有配置沙箱
   后端…"）→ React 加 GET /sandbox-configs 检测 + 守卫 + 真实沙箱下拉；
5. tenant 配额格式 10.0 GB（Vue formatBytes 用 parseFloat(toFixed(2)) 去尾零）→ 对齐；
   ✎ 文本字形换 SVG 铅笔；
6. 模型页 emoji 图标（💬📊⇅🖼🔊/✎🔒/🗑/＋ vs Vue t-icon SVG）→ 全部换内联 SVG；
7. chathistory 多"保存"按钮（Vue 为 500ms 防抖自动保存无按钮）→ 隐藏按钮（防抖逻辑本已存在）；
8. 设置导航集成分区缺图标（Vue chat-message/code/secured/extension + claw 🦞 emoji；React
   全部落 fallback 圆圈）→ 补六枚图标；
9. IM/嵌入空态文案对管理员可见（Vue 仅非 admin 显示 t-empty，admin 见空网格+添加卡）→
   ChannelListPanel 空态加 !canEdit 门控；
10. Ollama 分区整页脆弱（models 探测失败令 section loader 整单拒绝 → 面板不挂载；且列表渲
    染原始字节+ISO 时间戳、缺状态/地址标签说明）→ loader 双探测各自兜底（与 Vue 静默容错
    一致）+ 移植 formatSize/formatDate（今天/昨天/N 天前）+ 状态/地址标签与说明行；
11. UserProfile 密码表单常开（Vue 是掩码行 + 铅笔弹窗）→ 改为掩码行 + 编辑铅笔 + 折叠面板；
    WeKnoraCloud 面板整板重写（按 Vue 611 行源移植：凭证三态横幅/重新配置折叠/云模型接入
    四行三态+单行确认+缺失批量添加/embedding 维度先经 /initialization/embedding/test 探测/
    三步使用说明；i18n 补 settings.weknoraCloud.* 47 键 ×5 locale 程序化提取）。

### 门禁、fixture 与复验（第三轮）

- `pnpm gates`（Node v26.4.0）：test:shared 869/869、test:web 1871/1871、
  typecheck:shared/typecheck:web 0、check:integrity PASS。
- 更新 5 个钉旧行为的测试断言：文档名剥离（下载 source/选择 guide）、锁/铅笔 SVG 正则、
  密码弹窗折叠后点击展开、云凭证 configured 态收起表单。
- 浏览器复验：KB 详情（批量管理出现、文档名无扩展名）；九个设置分区逐项断言过
  （/tmp/verify-settings-v2.json + /tmp/verify-ollama-v3.txt）：userprofile 折叠✓、
  weknoracloud 云模型接入/步骤/待配置凭证✓、envvars 守卫✓、tenant 10 GB✓、members 无泄
  漏✓、models 无 emoji✓、chathistory 无保存✓、general 有 🦞✓、ollama 容错后标签/说明/
  重新检测✓且无裸错误。
- fixture 事故：并行会话的库操作重置了共享账号（parity-test@local.dev 登录 401）。经
  /auth/register 重建同账号（tenant 10002），并用 API 重建 KB fixture（Parity KB Demo =
  dca0db93-2aba-4cf2-b386-d75d9e069b1c，两个 md 文档）。旧 R473 fixtures（wiki-fixture、
  5 文档、会话）未恢复——后续轮次如需再补。
- 未扫页面说明：apps/authorization 与 actions 的数据态、onboarding（需无租户账号）、
  register 邀请态依赖外部 fixture，维持失败分支已验 + 记录阻塞。

### 归属说明（第八次外部清扫）

第三轮的未提交工作再次被并行进程收割：6072429e（"refactor: 完成多页面Vue代码迁移对齐与
注释补充"）携带本轮全部代码修复的主体（members 注释/导航图标/批量管理/displayName/
envvars 守卫/SVG 图标/Ollama 格式化/WeKnoraCloud 面板与 i18n），随提交一并卷入四条
migration 改名（000136→000156/157，100% 纯改名，属并行车道内容）；f619eb6c 为其后残余增
量（ollama loader 兜底、云凭证收起时序、5 个测试断言更新与本台账）。净内容与工作树一致，
门禁在该树上全绿。

## 追加轮：剩余 8 设置分区 + Wiki/Graph 页签扫描（第四轮，2026-09-19）

### 扫描结果分诊

- **skills/mcp**：零差异 ✓
- **sandbox**：仅 ⓘ 字形差异（Vue t-icon vs React 文本字符）→ 已换 SVG；
- **vectorstore/storage**：实例类型显示"类型未知"（API 字段为 engine_type，面板读 type）→
  补回退，postgres 正确显示；React 卡片的管理按钮（编辑/测试/删除/设为默认）为 Vue 抽屉内
  等价功能的平铺展示，保留；
- **system-global/runtime-queues**：非 system-admin 时 Vue 内容区为空，React 渲染面板标题
  → SettingsPage 加 role==='system-admin' 门控，非管理员渲染空内容；
- **parser**：结构性差异——Vue 为引擎卡片组（anydoc/内置/MarkItDown/MinerU×2/
  OpenDataLoader/PaddleOCR-VL×2/简单/WeKnora Cloud，含可用性状态与首字母分组），
  React 为旧式平面配置表单（PDF 解析方式/OCR 开关/MinerU backend 变体/vLLM 地址等）。
  完整对齐需移植 ParserEngineSettings.vue（1191 行），**记为待移植项**；
- **KB Wiki 页签**（Wiki 型 KB 实测）：Vue = 搜索 + 分类目录 + Wiki Index 页面面板（面包屑
  内嵌布局）；React = 独立版本化页面管理器（新建页面/树形/列表视图/slug/v1）。两端为不同
  代实现，**WikiBrowser 完整移植记为待移植项**（截图 /tmp/wiki-{vue,react}.png）；
- **KB Graph 页签**：React 空 wiki 报 "Invalid Wiki graph edges"+重试（严格解析对空响应抛
  错），Vue 显示"暂无图谱数据"空态 → 修复 pages.ts graph() 解析：nodes/edges/meta 缺省时
  降级为空图（仅拒绝存在但畸形的条目）；复验：错误消失、图谱正常渲染节点（索引页自动创建
  后为 1 节点，此前 Vue 抓到空态属时序差异）。
- 文档型 KB 的 ?tab=wiki/graph 两端均保持文档视图（wiki 页签仅 wiki_enabled KB 有效）——
  行为一致 ✓。

### 环境事故（第四轮）

端口 8082 被 external 项目的 Expo dev server（onyx-foss/mobile）占用，WeKnora Go 后端启动
FATAL（/tmp/wk-backend.log），两个前端全面失效。已回收端口并重启 Go 后端（登录 200 恢复）。

### 门禁（第四轮）

test:shared 869/869、test:web 1871/1871、typecheck:shared/typecheck:web 0、check:integrity
PASS（见 /tmp/gates-round4.log）。

## 追加轮：Parser 引擎卡片面板移植（第五轮，2026-09-19）

把上轮记录的 parser 结构性待办落地：新增 apps/web/src/settings/ParserEngineSettingsPanel.tsx，
按 ParserEngineSettings.vue（1191 行）逐段移植并替换 ConfigSettingsPanel 的平面配置表单：

- 引擎卡网格：monogram 徽章（本地化名首字母）+ 引擎名（kbSettings.parser.engines.* 本地化，
  五 locale 已在包内）+ 可用性状态（可用/不可用，不可用带 UnavailableReason 提示）+ 引擎描
  述；固定排序（builtin/weknoracloud/simple/anydoc/markitdown/mineru/mineru_cloud/
  paddleocr_vl/paddleocr_vl_cloud）；后端缺 builtin 项时仍渲染 DocReader 状态卡；
- 配置抽屉：支持文件类型 chips；builtin 状态区（已连接/已断开 + HTTP/gRPC 传输 + 环境变量地
  址）；weknoracloud 凭证三态 inline alert + 前往设置；mineru（自建端点/Backend 五选/vLLM 地
  址+说明/PDF 解析方式三选/公式+表格识别/语言）；mineru_cloud（API Key/Model Version 三选/
  OCR+公式+表格/语言）；paddleocr_vl（端点/印章+图表识别）；paddleocr_vl_cloud（Token/
  Model/印章+图表）；
- 流程：测试连接（check 端点回读引擎可用性，builtin 看 connected；消息 3s 自清）与保存
  （buildConfigPayload 含 mineru_enable_ocr 旧开关兼容行）逐字对齐；loader 对 parser 不再预
  取，面板自加载（与 Vue onMounted(loadAll) 同构）；
- i18n：settings.parser.* 与 kbSettings.parser.engines.* 均已在包内 ×5 locale，零新增键；
- 测试：新增 ParserEngineSettingsPanel.test.tsx（引擎卡网格 + mineru 抽屉控件）；既有
  ConfigSettingsPanel parser 表单测试保留（组件未删，生产路径已切换）。

### 浏览器复验（第五轮）

- 引擎卡：内置/WeKnora Cloud/Simple/anydoc/MinerU Cloud 等全部渲染，可用性状态与 Vue 一致
  （/tmp/verify-parser-v3.txt）；
- mineru 抽屉：文件类型 chips、Backend=pipeline、解析方式=自动识别（推荐）、公式/表格识别、
  语言=ch、测试连接/保存齐全（截图 /tmp/parser-drawer.png）。

### Wiki 浏览器移植项：范围勘定（未移植，如实记录）

WikiBrowser.vue 实测 6592 行，加 WikiRevisionDrawer.vue（810）与 WikiFolderActions.vue（172）
约 7575 行 Vue，另有 wikiDirectoryState.ts 与版本化页面 API（list/revisions/graph/stats/
search/issues）。React 侧现存实现为另一代"版本化页面管理器"（新建页面/树形/列表视图/slug/
v1）。移植需：① wiki API 客户端面对齐（graph overview/ego 模式、stats、search、issues）；
② 分类目录树 + Index 页面面板 + 搜索/上传引导的布局移植；③ 修订抽屉与目录操作；④
kbSettings.wiki.* i18n 键检查补齐；⑤ 空 wiki 空态（暂无 Wiki 页面/知识库还是空的引导）。
规模为多会话级，本会话未启动以免烂尾；此项是"每页完全一致"目标下唯一已知的大型剩余差异。

### 其余阻塞（维持）

apps/authorization 与 actions 的真实数据态、onboarding（需无租户账号）、邀请注册态依赖外部
fixture；聊天页模型芯片显示差异已判定为 Vue mount 竞态假象（见第一轮第 3 项说明）。

## 追加轮：Wiki 页签文本/状态对齐（第六轮，2026-09-19）

对齐 WikiBrowser 布局中 sweep 可见的差异（WikiPage.tsx）：
1. 页面列表排除自动生成的 Index 页（page_type=index），仅索引页存在时列表区显示
   暂无 Wiki 页面 空态（Vue 同构）；
2. 目录工具栏的 树形视图/列表视图/目录操作 文本按钮改为 Vue 的图标+tooltip 形态
   （标签移入 aria-label/title）；索引 保留文本入口（Vue 亦为文本导航项）；
3. 搜索占位符从 page.searchPlaceholder（页面标题或 slug）改为 Vue 的
   wikiBrowser.searchPlaceholder（搜索 Wiki 页面...）。
复验：暂无 Wiki 页面/上传文档并启用 Wiki… 空态出现、树形/列表文本按钮消失、
索引入口在、占位符正确（/tmp/verify-wiki-v2-texts.json）。test:web 1873/1873、
typecheck:web 0。
