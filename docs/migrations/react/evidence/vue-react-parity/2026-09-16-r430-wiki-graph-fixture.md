# 2026-09-16 Round R430 — Wiki 图谱 fixture 创建与双端图谱 canvas 同条件对比指南

- 目的：解除 R428 证据第 5 条的 `blocked-fixture`——图谱 canvas 同条件对比需要 wiki 型知识库，当时 fixture 为文档型。
- 后端：真实 docker `:8080`（WeKnora-app）。测试账号与 R428 相同（见 `2026-09-16-r428-browser-parity-pair.md` 第 2 行），tenant 10000 / owner；本文不重复明文凭据。
- 结果：**fixture 创建成功**（3 页面互链，graph 接口返回 3 节点 / 5 边）。

## 1. 契约结论（源码依据）

### 1.1 "wiki 型知识库" 的定义

后端没有独立的 wiki KB `type`。wiki 身份由 `indexing_strategy.wiki_enabled` 决定：

- `internal/types/knowledgebase.go:822` — `IsWikiEnabled()` 返回 `kb.IndexingStrategy.WikiEnabled`（单一事实来源）。
- `internal/handler/wiki_page.go:48` — `validateWikiKB` 对非 wiki KB 返回 400 `Wiki feature is not enabled for this knowledge base`。
- Vue 端 `frontend/src/views/knowledge/KnowledgeBase.vue:89` — `isWiki = !!kbInfo.value?.indexing_strategy?.wiki_enabled`；wiki/graph 面包屑 tab 仅在 `isWiki` 时渲染（同文件 :2359、:2412）。
- Vue KB 创建对话框的 type radio 只有 `document` / `faq`（`frontend/src/views/knowledge/KnowledgeBaseEditorModal.vue:72-73`）；`wiki_config` + `indexing_strategy` 在 document 类型下随创建 payload 提交（同文件 `buildSubmitData`，:1295 附近）。
- React 端 `apps/web/src/knowledge-bases/editor.ts:70-71` — create 模式 payload 为 `{ ...common, wiki_config, indexing_strategy }`（faq 时换成 `faq_config`）。

**创建 payload 结论**：`POST /api/v1/knowledge-bases`，body `{ name, description, type: "document", indexing_strategy: { vector_enabled, keyword_enabled, wiki_enabled: true, graph_enabled } }`。`wiki_config` 可省略（各字段有默认值）。注意后端 `capabilities.graph` 与 LLM 实体抽取（`extract_config`）绑定，创建后为 `false`，但这不影响 wiki 图谱 tab（Vue 只看 `isWiki`）。

### 1.2 wiki 页面创建

- `POST /api/v1/knowledgebase/{kb_id}/wiki/pages`（Vue `frontend/src/api/wiki/index.ts:177`；React `packages/api-client/src/wiki/pages.ts:289`；后端 handler `internal/handler/wiki_page.go:355`）。
- body 绑定到 `types.WikiPage`；`page_type` 合法值：`summary | entity | concept | index | synthesis | comparison`（`internal/types/wiki_page.go:135-146,165`）；`status` 合法值：`draft | published | archived`（:179）。非法值 400。
- **页面间建边**：`out_links` 由 content 中的 `[[slug]]` / `[[slug|display]]` 语法解析（service `internal/application/service/wiki_page.go:91` `parseOutLinks`；正则注释见 `internal/application/service/wiki_ingest.go:1506`）。slug 归一化：小写、空格转 `-`。`in_links` 在创建时自动回填（service :106 `updateInLinks`）。

### 1.3 登录

- `POST /api/v1/auth/login`，body `{ email, password }`，响应顶层 `token` 字段（Vue `frontend/src/api/auth/index.ts:210`、`LoginResponse` :12-55）。

### 1.4 wiki 图谱数据接口

- `GET /api/v1/knowledgebase/{kb_id}/wiki/graph`（Vue `frontend/src/api/wiki/index.ts:315` `getWikiGraph`；React `packages/api-client/src/wiki/pages.ts:312` `graph()`）。支持 `mode=overview|ego`、`center`、`depth`、`limit`、`types`。overview 模式返回 top-500 最连通页。

## 2. Fixture 创建结果

KB id：**`7cea6ec0-8a07-4c83-9309-f802a61b3d5c`**，名称 **Wiki图谱fixture**（`type: document`，`indexing_strategy.wiki_enabled: true`，已确认出现在 `GET /api/v1/knowledge-bases` 列表）。

创建命令（token 打码；登录后把 token 存入环境再调用，本文不落明文凭据）：

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"<见 R428 证据文档>","password":"<见 R428 证据文档>"}' | python3 -c "import sys,json;print(json.load(sys.stdin)['token'])")

KB=7cea6ec0-8a07-4c83-9309-f802a61b3d5c

curl -s -X POST http://localhost:8080/api/v1/knowledge-bases \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"Wiki图谱fixture","description":"R430 parity fixture: wiki KB with interlinked pages for graph canvas comparison","type":"document","indexing_strategy":{"vector_enabled":true,"keyword_enabled":true,"wiki_enabled":true,"graph_enabled":true}}'
```

页面清单（均为 `status: published`，版本 1）：

| slug | title | page_type | content 内链接 |
|---|---|---|---|
| `r430-overview` | R430 图谱总览 | summary | `[[r430-entity-weknora]]`、`[[r430-concept-graph\|图谱概念]]` |
| `r430-entity-weknora` | 实体: WeKnora | entity | `[[r430-overview]]` |
| `r430-concept-graph` | 概念: 链接图谱 | concept | `[[r430-entity-weknora]]`、`[[r430-overview]]` |

API 验证结果：

- `GET /api/v1/knowledgebase/{KB}/wiki/pages` → `total: 3`，三条 `out_links`/`in_links` 与上表一致。
- `GET /api/v1/knowledgebase/{KB}/wiki/graph` → `nodes: 3`（link_count 分别 4/2/2），`edges: 5`（overview↔entity、overview↔concept、concept→entity、entity→overview、concept→entity 方向组合），`meta: { mode: overview, total: 3, returned: 3, truncated: false }`。

## 3. 双端图谱面入口 URL（确切地址）

两端路由同形：`/platform/knowledge-bases/:kbId?tab=graph`（React 另保留 `/knowledgeBase/:kbId?tab=graph` 旧路径）。

- **React**（dev :5181）：
  `http://localhost:5181/platform/knowledge-bases/7cea6ec0-8a07-4c83-9309-f802a61b3d5c?tab=graph`
  - 路由解析 `apps/web/src/routes.tsx:86-88`（`platformKnowledgeBaseMatch`）+ `:24-28`（`tab` query 白名单 `documents|wiki|graph`）。
  - 渲染 `apps/web/src/main.tsx:277` → `KnowledgeGraphPage`（`apps/web/src/knowledge/KnowledgeGraphPage.tsx`），数据 `client.wiki.graph`（:58）。
- **Vue**（dev :5180）：
  `http://localhost:5180/platform/knowledge-bases/7cea6ec0-8a07-4c83-9309-f802a61b3d5c?tab=graph`
  - 路由 `frontend/src/router/index.ts:120-123`（`knowledge-bases/:kbId` → KnowledgeBase.vue）。
  - `frontend/src/views/knowledge/KnowledgeBase.vue:92` 从 `route.query.tab` 初始化 `activeKbTab`；:1088-1096 watch 回写 query（`documents` 时删除 query）。
  - **Vue 的 wiki 图谱 canvas 渲染组件是 `frontend/src/views/knowledge/wiki/WikiBrowser.vue`**（`view === 'graph'` 分支，:4-133：`.wiki-graph-canvas`、搜索、帮助弹层、图例 `wiki-graph-legend`、ego/frontier 动作、480px 抽屉），由 KnowledgeBase.vue :2412-2415 以 `:view="activeKbTab === 'graph' ? 'graph' : 'browser'"` 挂载，数据来自 `getWikiGraph`。
  - **不要把 `GraphSettings.vue`（KB 设置内"知识图谱配置"，LLM 实体-关系抽取配置）当作 canvas 面**——即 R428 证据第 5 条的澄清。React 侧对应物是 `apps/web/src/knowledge-settings/GraphSettings.tsx`，同样不是图谱 canvas。

## 4. Canvas 同条件对比核对清单

打开上述两个 URL 后（两端同视口 1440×900、浅色、简体中文），按以下条目核对（基线 = Vue）：

1. **节点**：应见 3 个节点，颜色按 `page_type` 区分（summary/entity/concept 各一色，节点半径随 link_count：overview 最大）。节点标题截断规则（>14 字符省略号）。
2. **边**：5 条有向边（双箭头 marker）；Vue/React 均以 `#c0c4cc` 细线渲染。核对双向边（overview↔entity、overview↔concept）是否双端箭头。
3. **图例（legend）**：五个内容类型 dot（summary/entity/concept/synthesis/comparison），可点击过滤；点击后节点隐藏/恢复行为一致。本 fixture 无 synthesis/comparison 节点，dot 应仍显示。
4. **搜索**：左上搜索框输入 "WeKnora" 应命中 `r430-entity-weknora`，选中进入 ego 模式并以该页为中心。
5. **交互**：单击节点 → 480px 抽屉（标题、summary、content、邻居提示 hint）；双击节点 → ego 模式；Shift+点击 → bloom 邻居；空白拖拽 pan、滚轮 zoom；节点可拖拽。
6. **状态卡**：overview 模式右上状态卡 "X/Y"（returned/total=3/3，truncated=false 提示）；ego 模式显示中心页 + 关联节点数。back 到 overview 的动作（Vue legend 内 "返回全图" 按钮 / React 对应）。
7. **帮助弹层**：? 图标展开操作说明（click/dblclick/shift+click/hover +/drag/pan/zoom 七行）。
8. **空态/加载**：清空过滤后全部节点隐藏时的空态文案（Vue `graphNoData`，React 对应 locale）。

## 5. React 侧源码差异核查结论

逐结构比对 `KnowledgeGraphPage.tsx` 与 Vue `WikiBrowser.vue` graph 分支：图例（可点击过滤 + familiar 环 + frontier 动作位）、搜索 + 帮助弹层、状态卡、480px 抽屉、expansion 虚线环、双向箭头、pan/zoom/drag 均已存在（R428 图谱域切片 N012 已对齐，`graph.test.ts` 14 用例）。**本轮未发现 React wiki 图谱页缺失 Vue 已有结构性元素，无代码改动。**

留待浏览器实测确认的项（源码无法判定，记录不猜）：
- 两侧力导布局的稳定形态差异（布局算法参数是否观感一致）。
- Vue 图例在 ego 模式下的 "grow frontier / 返回全图" 动作与 React 对应入口的视觉位置。
- 空态/加载态在真实后端延迟下的表现。

## 6. 约束遵守

- 未修改 `frontend/**`、`apps/mobile/**`、`internal/**`、`packages/api-client/src/auth/oidc.ts`、`docs/superpowers/plans/**`。
- 未 git add / commit；未删除任何后端数据（仅登录 + 创建 KB + 创建 3 页面）。
- 凭据未写入本文件（引用 R428 证据文档）。
