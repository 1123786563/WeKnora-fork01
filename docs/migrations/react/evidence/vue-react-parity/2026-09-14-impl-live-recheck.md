# 2026-09-14 implementing 行 live 重验（R013 R027 R031 R033 R043 R044 R046）

七个 implementing 行的代码切片均已落地（各自行 note 内含切片证据），普遍缺「当前实现的 live 双端对照」。本轮以 .parity-tools/impl-live-sweep.cjs 补齐（只读：分区导航 + 截图；真实后端 :8080，parity owner 账号，zh-CN 1440x900）：

| 分区 | 行 | live 锚点（双端一致） | 截图 |
|---|---|---|---|
| 模型管理 | R027 | 模型配置 h2+描述、模型测试入口、类型页签计数（全部(1)/对话(1)/Embedding(0)/ReRank(0)/视觉(0)/语音(0)）、内置模型灰盒说明 + 管理指南链接、模型卡片（mock-stream-model OpenAI·200K 锁标）、添加模型虚线卡 | i1-models-{vue,react}.png |
| MCP 服务 | R043 R044 R046 | MCP 分区标题 + 服务区渲染（无真实同步服务时空态） | i2-mcp-{vue,react}.png |
| 沙箱 | R031 | 沙箱分区标题 + 配置面渲染 | i3-sandbox-{vue,react}.png |
| 技能 | R033 | 技能分区标题 + 目录面渲染 | i4-skills-{vue,react}.png |
| API 集成 | R013 | API 集成分区 + API Key 表面渲染（Playground 为行内抽屉入口，非正文常驻文案） | i5-integrations-api-{vue,react}.png |

## 顺手修复（R027 范围）

React 内置模型说明盒（.wk-builtin-hint）原无任何样式——live 对照发现与 Vue 灰盒差异。按 Vue ModelSettings.vue less:893-913 补齐：#f3f3f3 底 + #e7e7e7 边框 + 6px 圆角、placeholder-tone 标签（12px/500/字距 .02em）、13px 次级描述、plain 文档链接。修复后截图 i1-models-react-after.png（对照 i1-models-vue.png 逐元素一致）。

## 状态变更

七行 implementing → review（实现已落地 + live 当前证据齐备），note 记录各自精确残余项：
- R043/R044/R046：真实同步 MCP 服务的对照（环境无 MCP 服务）
- R031：真实沙箱集群交互
- R033：安装/管理抽屉 live 交互对照
- R013：真实 Session/SSE（需 API Key/智能体）
- R027：真实连接/保存流程
- 全部：Wails/native 平台证据

## 门禁

typecheck:web 0 错误（样式 + 截图变更）；全套门禁见上轮（856/856、444/444、build ✓）。
