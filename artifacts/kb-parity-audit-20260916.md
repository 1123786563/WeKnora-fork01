# 知识库列表/详情 Vue → React 状态审查

- 审查日期：2026-09-16（Asia/Shanghai）
- 工作树：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`
- 对照范围：知识库列表、知识库文档详情（含文档列表/独立文档详情路由）、创建/编辑/分享/删除/上传/标签/解析 trace 弹层。
- 明确排除：`apps/mobile`、`apps/desktop`、移动端/原生运行时；本次没有修改业务代码。
- Vue 权威源：`frontend/src/views/knowledge/KnowledgeBaseList.vue`、`frontend/src/views/knowledge/KnowledgeBase.vue`、`frontend/src/views/knowledge/KnowledgeBaseEditorModal.vue`。
- React 对照源：`apps/web/src/App.tsx`、`apps/web/src/documents/KnowledgeDocumentsPage.tsx`、`apps/web/src/documents/KnowledgeDocumentDetailPage.tsx`、`apps/web/src/knowledge-bases/{list,states,detail}.ts`、`apps/web/src/knowledge/permissions.ts`。

## 证据等级

本报告以源码/测试为主，属于静态与 focused Web test evidence；没有把截图、构建或静态 helper 测试当作完整浏览器、真实后端或生产验收证据。当前工作树原有未提交修改保持不动。

验证命令：

```text
pnpm --filter @weknora/web test -- src/knowledge-bases/detail.test.ts src/knowledge-bases/states.test.ts src/knowledge/permissions.test.ts src/documents/page-chrome.test.tsx src/knowledge-bases/kb-list-anatomy.test.tsx
```

结果：Web test glob 全量执行，`1164 passed, 0 failed`（脚本会展开 glob，因此不是只运行命令行列出的 5 个文件）。测试过程中有既有 React `act(...)` 警告与非本审查范围的 `Not implemented: navigation to another Document` 输出，但没有失败。

## 状态矩阵

| 面 | Vue 行为 | React 行为 | 结论 |
| --- | --- | --- | --- |
| 列表首次加载 | `loading` 时显示 6 张骨架卡（`KnowledgeBaseList.vue:66-87`） | `pageState=loading` 时显示 6 张骨架卡并有 `aria-busy`（`App.tsx:253-260, 865-868`） | 对齐；有 focused anatomy 测试 |
| 列表空结果 | 全部/我的、收藏、最近、空间分别使用不同空文案；创建 CTA 只给 contributor（`KnowledgeBaseList.vue:634-678`） | `emptyVisible` 覆盖 success-empty 与 list-error，按 scope 渲染对应空态；CTA 受 `viewer.isContributor` 控制（`App.tsx:744-748, 870-895`） | 对齐；列表请求失败沿用 Vue 的“空态”表现，不向 UI 泄漏原始错误 |
| 列表请求错误 | `fetchList` 无可见 error branch，失败后仍由空态条件渲染（`KnowledgeBaseList.vue:1219-1242`） | `loadKnowledgeBaseListPage` 记录 error，页面仍用 `emptyVisible`；仅 console breadcrumb（`App.tsx:253-267, 744-748`） | 行为对齐，但错误与真空结果不可区分；记录为产品/可观测性限制，不作为本次业务代码修复 |
| 列表权限/菜单 | pin 对可见卡片开放；duplicate 需 contributor 且是本人；settings/delete 需 creator 或 admin（`KnowledgeBaseList.vue:1339-1362`） | `duplicable`/`manageable` 分别控制菜单；pin 不设管理门槛（`App.tsx:924-929, 966-985`） | 对齐；已有列表解剖与权限 helper 测试 |
| 列表弹窗 | 编辑器、删除确认、分享对话框；删除确认期间由确认回调提交（`KnowledgeBaseList.vue:684-707`） | 编辑/删除/分享均使用 `Dialog`；删除有 `deleting` 与 `createDeleteGuard` 防重复 DELETE（`App.tsx:671-685, 1013-1071`） | React 有额外防重复保护，状态意图对齐 |
| KB 详情首次加载 | KB 信息失败时 `kbInfo=null`，面包屑保留 skeleton；文档列表独立 loading；没有独立 KB 错误面板（`KnowledgeBase.vue:1017-1054, 2330-2359`） | KB metadata/auth/list 并行；metadata 失败后 `kbMeta=null`、`canContribute=false`，文档列表仍独立请求与渲染（`KnowledgeDocumentsPage.tsx:2243-2273, 2294-2332`） | 表层行为相近，但 React 的只读提示见下方 F-01 |
| 文档列表 loading/empty/error | skeleton；文档/文件夹空态按搜索/文件夹区分；源码中列表请求错误没有独立 retry UI（`KnowledgeBase.vue:2621-2707`） | 文档 loading、search/folder empty、error + `tryAgain` 都显式渲染（`KnowledgeDocumentsPage.tsx:3644-3665`） | React 比 Vue 多出可见 retry；属于可接受的增强，但不是完整视觉 parity 证明 |
| 详情权限与写操作 | `canEdit/canManage/canMutateKnowledge/canDownload` 分层；共享 KB 优先采用 share grant，避免本地 admin 越权（`KnowledgeBase.vue:261-334`） | 文档列表使用 `canUploadKnowledgeDocuments`；上传、批量删除/重解析、标签、move、manual 等控件均以 `canContribute` 门控（`KnowledgeDocumentsPage.tsx:164-181, 3257-3261, 3514-3577, 4173-4244`） | 列表/KB 文档页大体对齐；必须单独核对独立文档详情页（F-02） |
| 上传/编辑/删除/trace 弹层 | 上传确认、手工编辑、URL、删除、tag、trace 等由 Vue 对话框/抽屉控制，上传中不可关闭且失败保留 staged 状态（`KnowledgeBase.vue:1785-1816, 2064-2200`） | 对应 `Dialog`/`Sheet`；上传中禁止关闭，失败保留 dialog/error；删除、批量重解析、取消解析、标签弹层都有权限门控（`KnowledgeDocumentsPage.tsx:2600-2610, 2658-2701, 3789-4244`） | 状态契约对齐；focused upload/page-chrome tests 通过 |

## Current-state amendment (2026-09-16)

The two implementation findings below were rechecked against the current
worktree after the audit was written. F-01 is now represented by
`classifyKnowledgeBaseMetadataError` and the metadata error/forbidden retry
branch in `KnowledgeDocumentsPage.tsx`; F-02 is addressed by the shared
`computeKBPermissions` helper accepting tenant `admin`/`contributor`/`editor`
memberships, with focused tests covering both tenant admin and contributor.
The original finding text is retained as historical audit context. This
amendment is source/unit evidence only; authenticated paired browser and real
backend permission verification remain open.

## Findings (historical audit context)

### F-01 — KB metadata 失败没有可见的 KB 级 error/403 状态

- 严重度：P1（状态语义/用户反馈；不是已证明的后端权限绕过）。
- React 证据：`KnowledgeDocumentsPage.tsx:2243-2273` 将 `settings.get(knowledgeBaseId)`、`auth.me()`、KB 列表放在一个 `Promise.all`；任一关键请求失败时只执行 `setKbMeta(null); setCanContribute(false)`（`2267-2269`）。页面随后仍渲染文档 surface，且 `!canContribute` 显示 `viewerReadonly`（`3257-3261`），没有 KB not-found/forbidden/error 分支或 retry。
- Vue 对照：`KnowledgeBase.vue:1017-1054` 同样清空 `kbInfo` 后继续保持 skeleton/文档区域，因此这是现有行为的已知限制；但 React 把“metadata 请求失败”进一步呈现为“viewer 只读”，用户无法区分真实 viewer、KB 不存在、403、网络错误。
- 影响：权限/错误状态被折叠，可能误导排障；文档列表请求若仍成功，用户会看到没有 KB 名称的详情页。
- 证据状态：源码确认；没有在本次任务中新增 runtime/浏览器错误注入证据。
- 建议后续：增加独立 `kbMetaState = loading | ready | forbidden | error`，在保留 Vue skeleton 初始态的同时，对非初始失败显示本地化 error/403 与 retry；不要把失败默认投影为 viewer。

### F-02 — 独立文档详情页的权限 helper 漏掉普通 tenant admin/contributor

- 严重度：P1（可见能力错误，可能造成合法用户只读）。
- React 证据：`KnowledgeDocumentDetailPage.tsx:142-156` 读取文档所属 KB 后调用 `computeKBPermissions(...).canContribute`；`permissions.ts:30-50` 只认可 `system_admin/admin`（用户全局字段）、creator，或 `user_id/created_by` 匹配，明确不使用 tenant membership 的普通 `admin`/`contributor`。
- Vue 对照：KB 详情的 `canEdit`/`canManage` 在 `KnowledgeBase.vue:288-303` 对 home-tenant 的 `authStore.hasRole('admin')` 放行，并把 share grant 交给 `orgStore.canEditKB/canManageKB`；文档操作进一步由 `canMutateKnowledge` 分层（`308-320`）。
- 影响：普通 tenant admin 打开 React `/knowledgeBase/:kbId/documents/:documentId` 时，独立详情页会把合法用户当 viewer，隐藏编辑/下载等 capability；这条 route 不是 Vue 的独立页面，而是 React 增加的 deep-link surface，因此必须明确它是否承诺与 Vue DocContent 同权限。
- 证据状态：源码确认；现有 `permissions.test.ts` 只验证 creator/system-admin、viewer 和“contributor alone 不编辑他人 KB”，没有覆盖 tenant admin 对 home-tenant KB 的合法路径。
- 建议后续：若该独立 route 属于正式 Web surface，复用与 `KnowledgeDocumentsPage` 相同的 KB/share/tenant permission 解析，并补 tenant-admin、share-editor、share-viewer、metadata-error 的 UI tests；若不属于承诺范围，应在路由层阻止或明确标注为只读 preview。

## 审查结论

列表与 KB 文档详情的 loading、主要 empty、文档请求 error、权限门控和弹窗关闭/提交状态大体按 Vue 意图迁移，focused Web tests 全部通过；当前不能标记为“完整 parity accepted”。F-01 与 F-02 仍需产品/实现决策和运行时验证，且本报告没有覆盖移动端。
