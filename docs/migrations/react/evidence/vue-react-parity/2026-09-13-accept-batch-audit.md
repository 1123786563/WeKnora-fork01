# 2026-09-13 全页 live 验收批扫（accept-batch）与差异目录

环境：Vue :5180 / React :5181 / 后端 :8080（同库同账号 parity-test@local.dev，tenant 10000），1440x900 zh-CN，全新浏览器上下文（故 Vue 侧出现 NewUserGuide 欢迎引导浮层，属真实首访行为）。
工具：.parity-tools/accept-batch.cjs（扩展自 matrix-shots.cjs；截图 + h1/h2/button DOM 文本 dump）。
截图与 JSON：screenshots/accept-20260913/（40 张 + 40 个文本 dump）。

## 方法
1. 双端登录后逐路由截图 + DOM 文本抓取（h1/h2/buttons 前 25 个）。
2. 文本 diff 找头/按钮差异 → 逐对人工目检截图确认。
3. 差异追溯 React 组件源码定位根因。

## 差异目录（全部为 live 证据确认的真实差异）

### A. 整页级形态差异（React 为英文调试壳，非 Vue 页面）
1. R012 /platform/agents：React 渲染 ConfigurationPage（"Agents, models, MCP and skills" 英文调试页）；Vue 为真实智能体列表页（智能体标题+副标题、左侧图标轨 全部/收藏/最近/本空间、内置分组卡片、能力 chips、操作图标行、详情抽屉）。根因：main.tsx:173 将 /platform/agents 映射到 ConfigurationPage；AgentList 页面从未实现。→ 已派 R012 AgentList 实施代理。
2. R017 /platform/organizations：React 为英文调试页（WORKSPACE ORGANIZATIONS + 纯文本卡片 + 内联创建表单）；Vue 为 共享空间 卡片栅格页（头像/名称/描述/星点装饰/成员与共享统计行、我创建的 section chip、头部加入/创建图标按钮）。→ 已派 R017 重建代理。
3. R013 integrations IM tab：React 为 "IM channels / Owned by the im API domain." 英文调试表单；Vue 为 IM 集成 面板（标题+副标题+查看接入文档链接、IM 渠道 计数头、添加渠道 虚线卡）。→ 已派 integrations 重建代理。
4a. R010/N006 kb-documents 页：React 为 raw UUID eyebrow（KNOWLEDGE BASE · 9727D104-…）+ 文档/刷新 链接 + 简单上传盒（来源/文件/Choose Files）+ Root 文件夹树 + 搜索/状态/标签行 + 批量按钮行 + 纯文本空态；Vue 为 知识库 › Parity KB Demo › 文档 面包屑（caret + info + gear 图标）+ 拖拽上传副标题 + ⚠ 无解析引擎警告行（前往配置→）+ 全宽搜索文档名称 + 视图切换（栅格/列表）+ 全部标签/类型/状态/来源 + 起始-结束时间筛选行 + 知识为空，拖放上传 插画空态（含大小限制文案）。功能（上传/文件夹/筛选/批量）已在，但页面骨架、面包屑、警告行、日期筛选、空态插画全部缺失。→ 待派 documents 页形态切片（N007 代理当前持有 KnowledgeDocumentsPage.tsx，待其落地后派）。
4. FAQ 型 KB 详情（R010/N010 域）：React 露出 RAW UUID eyebrow（KNOWLEDGE BASE · 8B26F48E-…）、问答库标题、英文空态、无面包屑；Vue 为 知识库 › parity-faq-kb › 问答 面包屑（含 info/设置图标）+ 结构化问答管理副标题 + 全宽圆角搜索 + 全部标签筛选 + 居中中文空态（暂无 FAQ 条目）。→ 已派 FAQ 重建代理。
5. R016 chat 会话页/creatChat：packages/views/src/chat/* 共享组件仍为英文调试形态（Conversations/New chat/Source/Group 侧栏、Sandbox terminal 常驻内联块、User/Assistant 卡片、Copy 长条、Message composer）；Vue 为 会话标题+⋯头部、右对齐用户气泡、助手纯文本+图标行、大圆角 composer（快速问答▾/附件/@ chips + 模型 chip + 圆形发送）、沙箱终端按需抽屉。此前 54f2e59 只加了 UX 功能未重建形态。→ 已派 chat 形态重建代理。

### B. 面板级差异（settings 抽屉内）
6. R027 models 面板：React 有调试头（模型管理/configuration.models/刷新）+ 模型记录 RAW JSON dump；Vue 为 模型配置 h2 + ▶ 模型测试 右上链接 + 模型卡片栅格 + 添加模型 虚线 tile。→ 并入 settings 代理范围。
7. R031 sandbox 面板：React 有调试头（SandboxSettings/sandbox.configs/刷新）+ items[]/workspaceScriptsDisabled raw dump + 折叠三角形代替内联开关；Vue 为 沙箱配置 内联副标题、行内绿色 toggle、右上 集群搭建指南 链接。→ 主代理集成期修复（SandboxSettingsPanel.tsx 本轮无主）。
8. SettingsPage 包装层（全部面板受影响）：(a) 头部 刷新 按钮 Vue 无；(b) loading 文案 "Loading from {apiDomain}…" 英文泄漏；(c) general section 未挂载 GeneralPreferencesPanel（组件存在但 wrapper 链落到 read-note 占位；Vue 常规设置含 语言/主题模式/界面字体+示例预览/代码字体+预览/字体大小 小·正常·大）；(d) wk-settings-values raw payload dump 在所有 section 渲染；(e) tenant/userprofile 内联表单为英文，Vue TenantInfo.vue/UserProfile.vue 为本地化字段。→ 全部并入 settings 代理工作清单（A1-A5）。
9. settings-envvars：Vue h2 沙箱密钥；React "Personal environment variables" 英文标题。→ 并入 settings 代理（C 项）。
10. R024 MCP 列表卡片（除去包装层后仍存）：React 卡片为纯文本行（名称 + 编辑/删除按钮行 + 尚未填写使用说明 + SSE/已启用 徽标）；Vue 卡片为 图标+名称、+添加使用说明 链接、尚未同步工具 › 行，且 添加服务 为卡片栅格右侧虚线 tile（非头部按钮）。→ 集成期修复（McpSettingsPanel 本轮无主）。
11. settings-userprofile 实证：React 英文副标题 + 英文密码表单 + raw dl dump（id/username/email/avatar）；Vue 为 用户信息 + 说明行 + 描述列表（用户 ID/用户名/邮箱/注册时间 含帮助文案）+ 修改密码行（•••••• + ✏️ 编辑入口）。→ 已含 settings 代理 A5。
12. settings-tenant 实证：React 英文副标题 + Danger zone 英文 + Name/Description 表单 + Save tenant information + raw dump；Vue 为 空间信息 + 描述列表（空间 ID/空间名称✏️/空间描述✏️/空间状态 活跃徽标/空间创建时间/存储配额/已使用存储/存储使用率+✓）。→ 已含 settings 代理 A5（注意 Vue 为只读展示行 + 按行编辑入口，非内联表单）。
13. settings-system 实证：React 英文副标题 + read-note + raw dump（version/edition/commit_id/build_time/go_version/keyword_index_engine/…）；Vue 为 系统信息 本地化展示列表（应用版本+Standard 徽标+commit、UI 版本、构建时间、Go 版本、服务启动时间、运行时长 人性化 1小时3分钟30秒、数据库版本、关键词索引引擎 postgres、图数据库引擎 未启用）。→ 已发 settings 代理 D 项。

### C2. Embed 入口 live 冒烟（2026-09-13 补充）
- embed dev :5182（base /embed/，VITE_API_BASE_URL=:8080）冷启动冒烟通过：应用正常挂载（标题 WeKnora/AI assistant、composer、发送按钮），无 pageerror；无 channel id 时 React 显示英文错误态 “Unable to start chat / Missing embed channel id.”，Vue embed-main.ts 路由为 /embed/:channelId（无 id 时路由不匹配渲染空白）。→ 登记平台差异（React 错误态为有意的 UX 补充）；截图 screenshots/embed-smoke-20260913.png。完整 embed 渠道流程验收仍被后端 preview-session 令牌语义待决项阻塞（Round 30 登记，curl 复现 ems_ 令牌被 POST /embed/sessions 401 拒绝）。

### C. 跨页 chrome
10. NewUserGuide 产品引导（全站）：Vue 首访显示 7 步欢迎引导（欢迎使用 WeKnora、1/7 圆点、跳过引导/下一步）；React 无。→ 已落地 773e723b（packages/views/src/guides + PlatformShell 挂载，localStorage weknora:new-user-guide-done:v1 语义照抄 Vue；guides 14/14、jsdom 6/6、platform 75/75、live 复验通过）。开放：contextual guides（kbList/kbCreate/kbDetail/chat/tenantModels/agentCreate）未移植、user-menu 无重新打开入口、newUserGuide.* 26 键×5 语待迁 packages/i18n（暂于 steps.ts 字节级同键复制）。
11. Vue 平台侧栏图标轨（KB 列表/智能体/共享空间页左缘 全部/收藏/最近/本空间）React 侧栏无（Round 12 已登记偏差，维持开放）。

### D. 已核对一致（本批截图内未见形态差异）
- kb-list、kb-documents、抽屉 chrome/分组导航、settings-chathistory/members/memory/tenant/userprofile/system 面板标题与结构（表单内文案见 B-8e）、settings-skills、主导航高亮。
- Vue 侧截图普遍含欢迎引导浮层遮挡，属真实首访行为；React 侧待 guide 代理落地后同状态复拍。

## 后续
- 各实施代理落地后：主代理集成（SandboxSettingsPanel 调试残留清除等）、全量门禁、逐页复拍对比。
- 矩阵行状态随代理落地更新（R012/R017/R013/R016 → implementing；R023/R031 增加 wiring/调试残留开放项）。
- 已派代理：R012 agents 页、R017 organizations 页、R013 integrations、FAQ 页、chat 形态、NewUserGuide、N007 upload graph、R033 skill SSE、settings R027+包装层（前一个 settings 代理无改动失败后重新派发）。
