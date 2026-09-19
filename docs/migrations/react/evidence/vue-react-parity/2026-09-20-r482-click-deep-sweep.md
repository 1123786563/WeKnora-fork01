# 2026-09-20 R482 从零逐页「点击深扫」轮（每个可打开的交互面）

用户本轮新指令（2026-09-20）：
1. 从 0 开始一个页面一个页面对比，包括**每个可以点击、可以打开的页面/交互面**。
2. **React 后来加的菜单不用管**（React 独有导航/入口=豁免，不报差异、不要求删除）。
3. 完全一致标准不变（≤1%）。

前置状态：主仓 23 轮路由级全扫已收敛 0 真差异（见 `2026-09-19-fresh-full-scan.md`）；本轮在其上加一层——**弹窗/抽屉/弹层/菜单级**逐面对比（Vue 全库 76 个含 t-dialog/t-drawer/t-popup 的文件为底册）。

## 环境

- Vue :5174（frontend/）、React :5175（apps/web），主仓 main 分支 HEAD 5c4ae83f，用户启动。
- 后端经 vite 代理可达（capabilities 401=健康）；实际可用账号由代理记录（候选 parity-test@local.dev/tenant 10002 或 wu18349270334@gmail.com）。
- 方法：playwright-core（主仓 node_modules）+ headless chromium（ms-playwright 缓存）双端隔离 context，同操作打开同一交互面 → 可见文本集合对比 + 截图；禁破坏性操作（删除不确认/保存不提交）。
- 已知噪音清单沿用 23 轮口径（select option 泄漏、表头合行、星号伪元素、图标装饰、动态数据、引导弹窗）。

## 分工

- B1 知识库域（.omc/state/r482-main/report-B1.md）：KB 列表新建弹窗/卡片菜单；KB 详情上传/FAQ/行菜单/设置抽屉各 tab；Wiki/Graph 工具。
- B2 智能体+聊天域（report-B2.md）：AgentEditorModal 新建/编辑；卡片菜单；creatChat 推荐问题/选择弹层；聊天页头部菜单/模型弹层/消息工具条/⌘K 面板/工件抽屉/工具结果弹层；共享空间。
- B3 设置+组织+系统域（report-B3.md）：userprofile 密码 popup；tenant/members 邀请与行菜单；models 编辑弹窗；各资源分区新建/编辑弹窗与行菜单；MCP 弹层；组织三弹窗；系统管理四弹窗；集成分区重置/生成类弹窗。

## 结果

### B2 智能体+聊天域（34 面：14 PASS / 18 DIFF / 3 豁免 / 4 未覆盖）

账号 parity-test@local.dev（双端直登，无租户选择页）；会话 340bade1-0e49-4c65-8cf5-f9b80b950e39。报告 `.omc/state/r482-main/report-B2.md`，59 截图。

高优先（5）：
- **D1** React 智能体编辑器缺 3 分区：问题推荐/附件上传/MCP 服务（agent-editor.ts:315-317 类型联合即无）
- **D3+D10** 新建预填体系未移植：Vue 开新建即应用 rag-qa 预设（名称/描述/系统提示词/工具预填）+最终启用工具 4 项；React 全空+「当前没有可用工具」
- **D12** @ 知识范围弹层：Vue 无结果（兼容性+初始化过滤）vs React 显示 2 KB+2 文件（list({creator:'all'}) 无过滤）
- **D13** 模型芯片禁用：React creatChat/chat 芯片 disabled 无下拉无「添加模型」（models API 200 含 KnowledgeQA，3 次复现）
- **D16** 聊天头部菜单缺 4 项：复制会话 ID/复制对话链接/复制为 Markdown/在新窗口打开

其余：D2 缺智能体类型字段；D4 提示词缺变量/恢复默认/模板；D5 新建不预填模型+ReRank；D6 ReRank 必填判定；D7 温度文案分叉；D8 缺「支持的文件类型」；D9 检索阈值默认（租户配置 vs 硬编码）；D11 技能缺「管理沙箱」链接；D14 Agent 选择器就绪判定；D15 输入条缺网页搜索按钮；D17 ⌘K 缺「新手引导」命令；D18 检索抽屉阈值 0.15/0.30 vs 0.00/0.00（Vue ||回退 vs React 原始值）。

豁免（React 后加）：编辑器「个性化」分区（Octop M1）、消息点赞/点踩（SP11）、⌘K「打开专家模板」。
未覆盖（数据不满足）：工件抽屉、工具结果弹层（会话全 builtin 无工具）、共享空间卡片菜单（空态）、编辑模式发布分区。

### B1 知识库域（25 面：8 PASS / 17 DIFF）

kbId：Parity KB Demo=dca0db93-2aba-4cf2-b386-d75d9e069b1c；Wiki Parity Fixture=4279ddcc-efc2-4930-9196-f31581af0d57。报告 `.omc/state/r482-main/report-B1.md`，60+ 截图。

PASS：新建知识库弹窗、KB 卡片菜单（置顶/创建副本/设置/删除）、共享空间空态、上传下拉菜单、导入网页弹窗、Wiki 子页基线、Wiki 目录视图。

DIFF：
1. 文件类型筛选文案：「手动创建」vs「在线编辑」（键名错配 knowledgeBase.typeManual vs upload.onlineEdit）
2. 标签筛选下拉：Vue 有「管理标签…」入口；React 无（仅 FAQPage 有）
3. **文档行菜单**：Vue 7 项+trace 摘要块；React 4 项（缺 下载/查看Trace/跨库移动/trace 块）
4. **KB 设置抽屉 13 tab 全 DIFF（系统性差异族）**：底部按钮「取消/保存并关闭」vs「保存配置」；各 tab 标题/描述整段不同（React 独立重实现）；解析引擎逐文件清单 vs 聚合；数据源 tab Vue 空 vs React 有连接器内容；活动记录「没有更早的记录了」vs「加载更多/刷新」
5. **Graph 空态**：Vue 隐藏全部工具（v-if=graphReady）+解析引擎警告 banner；React 空态即渲染整套工具栏
6. Wiki 新建页面弹窗字段分叉（「标题/slug/页面类型/取消/确认」vs「一句话摘要/新建页面」）

未覆盖：FAQ 新建弹窗（无 FAQ KB）、上传确认弹窗（禁真传）、graph 帮助弹层内容。

### B3 设置/组织/系统域（24 有效面：14 PASS / 9 DIFF + 若干一致不可用）

报告 `.omc/state/r482-main/report-B3.md`，38 对截图 + 原始 JSON。不可测面均为数据/权限受限且两端一致（MCP 无服务、无组织、无 system-admin、builtin 模型、记忆未启用）。

DIFF：
- **D0（高）API Key 创建**：Vue=完整能力授权矩阵（8 组能力含说明）；React 仅「名称」+提交（packages/views/src/integrations/page.tsx L1396-1405 仅 apiKeyName）
- **D1（中）三引擎添加弹窗**：React「名称/类型/安全配置 JSON」裸表单 vs Vue 分字段结构化表单（addr/username/password/HTTP 代理/设为默认/测试连接/保存）
- **D2（中）i18n 转义 bug**：React skills 弹窗两处字面 `{'@'}` 未渲染（同类隐患 organization.settings.sharedAgentsKbHint）
- 低危：D3 envvars 说明弹层缺失；D4 models 三小差（服务商默认/思考模式说明/按钮序）；D5「SSE已启用」vs「已启用」；D6 orgs 三小差（nav「基础」/按钮文案/0-500 计数器）；D7 role-denied 态多渲染标题；D8 chrome/claw「打开 API 信息」入口缺失
- PASS 代表：密码 popup、tenant 修改、members 邀请/共享链接/审计抽屉、sandbox 添加、chathistory、IM/embed 渠道向导、cli 复制

## R482 总计与修复优先级

**36 PASS / 44 DIFF**（B1 8/17、B2 14/18、B3 14/9；44 含大量低危文案项）。豁免 3（React 后加），未覆盖面两端一致受限（FAQ KB、工具调用会话、组织、system-admin 弹窗——fixture 需求记入池）。

修复批次（按系统性归族，后续轮次执行）：
1. **族 A：KB 设置抽屉 13-tab**（B1-D4）——按钮/文案/结构系统性重对齐，工作量最大
2. **族 B：智能体编辑器**（B2-D1/2/3/4/5/6/10）——补 3 分区+预填体系+类型字段+温度文案
3. **D0：API Key 能力矩阵**（B3）——补授权矩阵 UI+payload
4. **D13：模型芯片禁用**（B2）——疑似主仓回归（23 轮曾对齐），先 git 定位
5. **族 C：引擎添加弹窗结构化**（B3-D1）
6. **快修批**：B3-D2 i18n 转义、B1-1 键名错配、B2-D16 头部菜单 4 项、B2-D18 检索阈值回退、B1-3 行菜单 4 项+trace 块
7. **低危文案批**：其余菜单项/入口/说明差

本轮零代码修改（纯取证轮）；证据=三报告+157+ 张截图+原始 JSON。
