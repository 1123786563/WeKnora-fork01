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
