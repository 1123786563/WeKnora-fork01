# 文档页形状切片证据（React 多客户端 Vue 对齐）

日期：2026-09-13
worktree：/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient（分支 codex/react-multiclient）
切片：文档型知识库详情页页面骨架（Vue 路由 /platform/knowledge-bases/:kbId 非 FAQ 分支；React 路由 /knowledgeBase/:kbId）。修复 GAP：React 旧版裸 UUID eyebrow「Knowledge base · 9727d104-…」+ 来源/文件/上传文件 旧表单 + 英文状态下拉 + 纯文本空态。

## Vue 基准（只读参考）

- frontend/src/views/knowledge/KnowledgeBase.vue 非 FAQ 分支（:2328-2729）：
  - 面包屑（document-title-row / document-breadcrumb）：知识库（→ /platform/knowledge-bases）› kbName（KBSwitcherDropdown，选择后跳对应 KB 详情）› 文档（knowledgeEditor.document.title）；右侧 kb-title-actions：KBInfoPopover 信息按钮（传 supportedFileTypes）+ 设置齿轮（canManage → uiStore.openKBSettings）。
  - 副标题 knowledgeEditor.document.subtitle；警告行 parser-hint（:2395-2402）：unsupportedFileTypes 非空时显示 knowledgeBase.unsupportedTypesHint（扩展名按「.ext、」拼接）+ 前往配置 →（goToParserSettings → KB 设置 parser 页签）。
  - **unsupportedFileTypes 来源**（:186-233）：tenant parserEngines（GET 引擎列表，含 Name/FileTypes/Available）∪ kbInfo.chunking_config.parser_engine_rules 解析：显式规则命中「可用引擎」才受支持；无规则时声明引擎 Available!==false 才受支持；全类型差集排序即警告清单（本 KB 实测 .docm、.odp、.ods、.odt、.pptm、.rtf、.xlsm）。
  - doc-filter-bar：全宽圆角搜索（docSearchPlaceholder「搜索文档名称...」）+ 筛选行 全部标签 / 全部类型 / 全部状态 / 全部来源 / 起始时间—结束时间（updatedTimeRange → filterParams :668-683：start_time="YYYY-MM-DD 00:00:00"、end_time="…23:59:59"、folder_recursive=isFiltering）+ trailing 视图切换（卡片/列表）+ KbUploadSourceDropdown（addDocument 提示，file/folder/url/manual）。
  - 空态（:2699-2708）：筛选中 → folderTree.emptySearch「没有匹配的文档」；选中文件夹 → folderTree.emptyFolder「这个文件夹里还没有文档」；否则 EmptyKnowledge（empty-knowledge.vue：upload.svg 插画 + emptyKnowledgeDragDrop「知识为空，拖放上传」+ pdfDocFormat + textMarkdownFormat）。
- 后端（引用）：internal/handler/knowledge.go:906-938 列表筛选 tag_ids/keyword/file_type/parse_status/source/start_time/end_time/folder_path/folder_recursive（parseFilterTime :2565 接受 "2006-01-02 15:04:05" 与 "2006-01-02"）。packages/api-client KnowledgeDocumentListParams 已含 file_type/source/start_time/end_time。

## 变更文件（本切片全部产出）

- apps/web/src/documents/page-chrome.ts（新）—— 纯逻辑：computeSupportedFileTypes / computeUnsupportedFileTypes（Vue :186-233 逐条 port）、dateRangeToTimeParams（filterParams 时间界）、documentsKBListPath/documentsKBDetailPath/documentsKBSettingsPath、isFilteringDocuments（folderTree.ts 语义：搜索下钻子目录）。
- apps/web/src/documents/DocumentsPageChrome.tsx（新）—— DocumentsBreadcrumb（面包屑 + KB 切换菜单 + 信息卡（类型/描述/创建时间/可上传格式 chips）+ 齿轮，onNavigate 三目的地同 FAQ 切片约定）、ParserHint（警告行，types 为空渲染 null）、DocumentEmptyState（illustration/folder/search 三变体）、SearchIcon、DOCUMENT_FILE_TYPE_OPTIONS / DOCUMENT_PARSE_STATUS_OPTIONS / DOCUMENT_SOURCE_OPTIONS（Vue fileTypeOptions/parseStatusOptions/sourceOptions 原样搬移，中文标签走共享 key）。
- apps/web/src/documents/KnowledgeDocumentsPage.tsx —— 页头重建（去 UUID eyebrow；面包屑 + 副标题 + 警告行 + 保留 文档/Wiki/图谱 导航与刷新按钮）；删除旧 上传表单（来源/文件/Choose Files/上传文件），上传入口改为 Vue 式加源下拉（filter-bar trailing，tooltip=knowledgeBase.addDocument，file/folder/url/manual 四项；manual 经标题/内容小弹窗进入既有 pendingManual 确认流）；筛选行重建（全宽搜索 docSearchPlaceholder、全部标签/全部类型/全部状态/全部来源、双 date 输入 起始时间/结束时间）；**列表查询接入 file_type/source/start_time/end_time**（folder_recursive 改为 isFiltering 语义）；parserEngines+kbMeta.chunking_config.parser_engine_rules 派生 supported/unsupported；空态换 DocumentEmptyState 三变体；kbMeta 拉取时并取 client.knowledgeBases.list() 供面包屑切换器。全部既有功能接线保留：拖拽 stageFiles、字节进度 UploadProgressMask、文件夹树、批量（重新解析/移动/设置标签/删除/取消解析）、标签、分页、预览跳转、权限 gating。
- apps/web/src/documents/documents.css —— 追加 chrome 样式（面包屑/信息卡/齿轮/警告行/doc-filter-bar/搜索框/筛选控件/日期区间/空态插画，对照 KnowledgeBase.vue less 与 faq.css port；[hidden] 重申 display:none）。
- apps/web/src/documents/empty-documents.svg（新）—— Vue frontend/src/assets/img/upload.svg 逐字节副本（organizations/empty-organizations.svg 同源）。
- apps/web/src/documents/page-chrome.test.ts（新）/ page-chrome.test.tsx（新）—— 17 条新断言（helper 8 + 组件/页面 9；renderToStaticMarkup 模式与 upload-confirm-dialog.test.tsx 同构；UUID 仅允许出现在导航 href，禁止出现在可见文案）。
- apps/web/src/documents/upload-confirm-dialog.test.tsx —— 仅测试基建：resolve hook 增加 .svg stub（页面现打包插画；OrganizationsPage.test.tsx 同法），断言零改动。

未触碰 MUST-NOT-EDIT 文件（main.tsx、App.tsx、platform/**、settings/**、agents/**、organizations/**、faq/**、configuration/**、packages/**、apps/mobile|desktop|embed/**）。未 git commit。

## TDD 红/绿

- RED（实现前）：tsx --test page-chrome.test.ts page-chrome.test.tsx → 2 files / 0 pass / 2 fail（模块缺失：page-chrome.ts、DocumentsPageChrome.tsx；页面级断言 UUID eyebrow 存在、无面包屑、无警告行组件、旧空态文案、旧上传表单均在）。
- 中间绿：page-chrome.test.ts 8/8；page-chrome.test.tsx 8/9 → 修正 UUID 断言语义（href 中的路由 id 不算可见文案）后 9/9。
- GREEN（实现后）：pnpm --filter @weknora/web exec tsx --test 'src/documents/*.test.ts' 'src/documents/*.test.tsx' → **79 tests / 79 pass / 0 fail**（62 基线全绿 + 17 新增）。
- 类型检查：pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit → **0 错误**。

## i18n（packages/i18n 只读，未改）

全部新文案走共享 catalog：menu.knowledgeBase；knowledgeEditor.document.title/subtitle；knowledgeEditor.basic.typeDocument/typeFAQ；knowledgeBase.docSearchPlaceholder/allTags/allFileTypes/fileTypeFilter/allParseStatuses/parseStatusFilter/parseStatus{Pending,Processing,Completed,Failed,Cancelled,Finalizing,Draft}/sourceFilter/allSources/source{Upload,Url,Manual,Api,BrowserExtension}/channel{Feishu,FeishuDrive,Notion,Yuque,GitLab,Ima,Wechat,Wecom,Dingtalk,Slack,Im}/updatedTimeFrom/updatedTimeTo/unsupportedTypesHint/goToParserSettings/addDocument/typeManual/settings/description/infoCard.{tooltip,title,type,createdAt,supportedFileTypes}/folderTree.{emptyFolder,emptySearch}/emptyKnowledgeDragDrop/pdfDocFormat/textMarkdownFormat；upload.uploadDocument/upload.uploadFolder（uploadConfirm 表）。React 自造 key knowledgeBase.documents.subtitle/searchPlaceholder 在本页不再使用（其他 key 维持原接线）。

缺失（已用现有 key 降级，未阻塞）：upload.onlineEdit（Vue 手动创建菜单项「在线编辑」，共享 catalog 无此键 —— 用 knowledgeBase.typeManual「手动创建」替代）。

## 实况证据（1440x900 / zh-CN / parity-test@local.dev / KB Parity KB Demo 9727d104-cde4-4d03-879f-d7e3897b69a3）

| 项 | Vue :5180 | React :5181 |
|---|---|---|
| 页面 URL | /platform/knowledge-bases/9727d104-… | /knowledgeBase/9727d104-… |
| 整页截图 | screenshots/documents-page-slice/vue-documents.png | screenshots/documents-page-slice/react-documents.png |
| 面包屑 | 知识库 › Parity KB Demo ▾ › 文档 + (i) + ⚙ | 同左（innerText 无「Knowledge base ·」eyebrow，UUID 仅存在于 href） |
| 副标题 | 支持点击或拖拽上传… | 逐字一致 |
| 警告行 | 部分文档类型（.docm、.odp、.ods、.odt、.pptm、.rtf、.xlsm）暂无可用解析引擎，上传后将无法解析 前往配置 → | 逐字一致（同派生逻辑，同扩展名集合） |
| 搜索/筛选 | 搜索文档名称... + 全部标签/全部类型/全部状态/全部来源/起始时间—结束时间 | 同左（日期为双原生 date 输入） |
| 空态 | 知识为空，拖放上传 + pdf、doc…10M + text、markdown…200K | 同左（同插画资产） |
| 筛选空态 | 没有匹配的文档（vue-documents-empty-search.png） | 同左（react-documents-empty-search.png） |
| 交互截图 | — | react-documents-info-card.png（知识库信息卡：类型/描述/创建时间/可上传格式 chips）、react-documents-add-menu.png（＋加源下拉：上传文档/上传文件夹/URL 导入/手动创建） |

采集方式：playwright-core(1.63) + 缓存 chromium-1243；API 登录（POST /api/v1/auth/login）后按 legacy-session.ts 契约注入 weknora_token/weknora_refresh_token/weknora_selected_tenant_id + weknora_react_session_v1 + locale=zh-CN；vite-error-overlay 重试门 + 新手引导弹窗自动跳过（跳过引导/Escape/DOM 移除）；capture-meta.json 附每张图的 URL/innerText 抽样断言（eyebrow-copy=false、overlay=false）。

## 剩余差距（不阻塞本切片验收；按任务书逐条记录）

1. 视图切换（grid/list 图标）与搜索行右上 open-in-new 图标未渲染：React 目前只有列表视图（DocumentCardView 卡片视图未 port），按任务书「only one view exists → record delta」记录，未挂假开关。
2. 标签筛选为普通下拉；Vue 是带计数 chips/搜索/管理链接的标签面板（tag-filter-panel）。
3. 文件夹树：React 始终显示 文件夹/Root 侧栏；Vue 仅在 KB 有文件夹时显示（showFolderTree=hasFolders），本 KB 无文件夹故 Vue 无此栏。
4. 日期筛选为两个原生 date 输入（起始时间/结束时间 aria/title 完整）；Vue 为单个 TDesign range picker。查询参数语义一致（同后端格式）。
5. 齿轮按 canContribute（查看者隐藏）而非 Vue 的 canManage 精确角色；与 FAQ 切片行为一致的近似。
6. 手动创建：React 用标题/内容弹窗进入既有确认流；Vue 打开 uiStore.openManualEditor 在线编辑器（后者未 port）。
7. Wiki KB 的 文档/Wiki/图谱 页签仍为页头导航（Vue 在面包屑内联，仅 wiki KB 出现）；Vue 框选批量（marquee）与底部悬浮批量条（DocumentBatchBar）为 React 内联批量行替代（既有功能保留）。
8. Vue 的 storage-engine-warning（缺存储引擎）行未做（需 storage_backend_id 判定，本 KB 不触发；key 已在共享 catalog）。
