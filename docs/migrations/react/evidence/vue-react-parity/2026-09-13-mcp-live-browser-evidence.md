# MCP 设置 live 浏览器/真实后端证据（R024/R043–R046/N016，2026-09-13）

## 环境

- 后端：make dev-app → :8080（隔离 dev 库，tenant 10000，账号 parity-test@local.dev）。
  本轮以 `SSRF_WHITELIST_EXTRA=mcp.parity-invalid.example` 重启，以便创建指向不可解析域名的 MCP 服务（用于失败态取证，不放宽其他校验）。
- React dev：:5181（VITE_API_BASE_URL=http://localhost:8080）；Vue dev：:5180。视口 1440x900，zh-CN。
- 测试服务：`Parity Evidence MCP`（SSE，https://mcp.parity-invalid.example/sse，id 3562caac-614f-4895-a58c-9a58d184154c），由 API 创建（POST /api/v1/mcp-services）。
- 脚本：.parity-tools/mcp-live-evidence.cjs、mcp-test-evidence.cjs、mcp-vue-test2.cjs、mcp-react-dialog-dump.cjs。

## 已采集状态（截图均在 screenshots/）

| 状态 | React | Vue |
|---|---|---|
| 列表（空） | mcp-react-list.png（首跑，创建前） | — |
| 列表（有服务） | mcp-react-list-after-create.png；DOM 文本核对：MCP 服务管理/描述/添加服务/卡片（名称、编辑、删除、尚未填写使用说明、SSE、已启用） | mcp-vue-list.png；同卡片结构（编辑/删除 aria-label、启用开关） |
| 新增 第1步 | mcp-react-dialog-step1.png（+ -validation、-filled） | mcp-vue-dialog-step1.png（+ -validation） |
| 编辑 第1步 | mcp-react-edit-step1.png | （drawer 打开 DOM 文本核对） |
| 第2步（工具/用途） | mcp-react-edit-step2-tools.png | mcp-vue-edit-step2-tools.png |
| 元数据刷新失败态 | mcp-react-test-result-failure.png（Connection failed: ... no such host） | mcp-vue-test-result-failure 场景（drawer 内联 "Failed to refresh MCP tools. Check the connection and try again."） |

## 第1步表单结构对比（DOM 文本逐项）

- 共有一致项：步骤条（1 连接配置 → 2 工具与用途说明）、从代码导入、服务名称、描述、传输类型（SSE/HTTP Streamable）、服务 URL、自定义请求头（可选）+添加请求头+hint、认证方式（无/自定义 Header、API Key/Token、OAuth 2.0 首次连接授权）、高级配置（超时时间(秒)/重试次数/重试延迟(秒)）、取消、保存并下一步。
- Vue 独有分组顺序：基本信息（服务名称+启用服务+hint 关闭后该服务不会被调用）在前；React 将“启用服务”置于高级配置之后。
- React 独有：传输类型含 `stdio` 选项（列表视图标注 unsupported editor；Vue 新增/编辑 drawer 不提供 stdio）。
- 按钮顺序：Vue 取消→保存并下一步；React 保存并下一步→取消。

## 第2步结构对比

- Vue：服务用途说明区（模型先读取服务用途…）、使用说明 可编辑 textarea + 0/16000 计数 + “AI 生成” + “根据已同步且启用的 Tools 生成精简说明…”、Tools 清单 + 重新拉取、未同步/刷新失败内联提示、footer 上一步/取消/保存。
- React：工具计数/服务器名/版本/stale 提示、McpToolsDirectory（搜索分页+策略开关）、generateUsage 按钮、usage 只读 pre（编辑入口为主表单 textarea）、测试连接 按钮 + McpTestResultBody。
- 差异登记：
  1. React 提供“测试连接”按钮与结果体；Vue 当前可达 UI 无任何测试入口——`McpTestResultBody.vue` 与 `testMCPService()` 在 Vue 基准中均未被挂载/调用（孤儿代码）。按“Vue 为权威”，React 暴露 Vue 不可达的功能属于偏差，待裁决（保留=超出基准；移除=向基准收敛）。
  2. React 第2步缺少 Vue 的 0/16000 计数器与“根据已同步且启用的 Tools 生成精简说明”帮助文案（generate 按钮存在）。
  3. React 元数据区将 usage 呈现为只读 pre；Vue 的编辑体验集中在 drawer 的 textarea（React 主表单同样有 required textarea，等价能力存在，视觉呈现不同）。

## 发现的缺陷（已定位并修复）

- 🐛→✅ **React 连接步保存必 400**：React step-0 PUT 携带 `usage_instructions: ""`，后端校验 1..16000 字符直接 400（live 隔离复现：Vue PUT 省略该字段返回 200、React 400）。Vue buildPayload（McpServiceDialog.vue:898-938）从不发送 description/usage_instructions。修复：新增 buildMcpConnectionPayload 纯函数（含 headers 过滤、url||undefined、auth_config 组装），save() 改用它，并移除 Vue 没有的“描述”表单字段。回归测试：McpSettingsPanel.test.tsx 新增 payload 断言（6/6 通过）；浏览器复跑 PUT 200、进入第2步、enabled 保持 true。截图：mcp-react-list-fixed.png、mcp-react-edit-step2-fixed.png；复现脚本 .parity-tools/mcp-enabled-isolate.cjs。
- 此前观察到的 enabled=false 为取证脚本自身对 API 的显式写入与早期混乱点击所致；隔离验证（Vue 流/React 流各测）均不翻转 enabled，后端对省略字段保持原值。

## 其他记录

- Vue settings overlay 存在重复挂载的过渡 DOM（.service-card--add 双份），Playwright 原生点击 actionability 失败；取证脚本统一使用 dispatched MouseEvent（对齐用户点击语义）。
- 后端 SSRF 校验错误信息（zh）在两端保存失败路径中均应呈现——已留 React 保存失败截图（首跑 dialog-step2 前）供后续核对文案。

## 结论

- R024/R043/R044/R045/R046/N016 的“浏览器+真实后端”证据缺口大幅收窄：列表/新增/编辑/两步流/失败态均有同后端双端截图与 DOM 文本。
- 行状态维持 implementing：待处理 enabled 翻转缺陷、stdio 选项偏差裁决、第2步计数器/帮助文案对齐、测试连接偏差裁决、Wails 证据。

（2026-09-13 round 2 更新：stdio/测试连接/第2步计数器三项均已向 Vue 基准收敛落地并在浏览器复验，见下。）

## Round 2 复验（2026-09-13，抽屉结构对齐后）

- 环境：同后端 :8080（Parity Evidence MCP 仍指向不可解析域名，用于未同步/失败态）；React :5181 vite dev（本轮代码，提交 6b408a23 + CSS 修复）；1440x900 zh-CN。
- 脚本：.parity-tools/mcp-round2-verify.cjs；截图 mcp-react-round2-edit-step1.png / mcp-react-round2-step2.png。
- DOM 断言（React 编辑抽屉，Parity Evidence MCP）全部通过：
  - 区块顺序 legend：基本信息 → 连接配置（含 自定义请求头（可选））→ 认证配置 → 高级配置；
  - 传输类型选项仅 SSE / HTTP Streamable（stdio 已按 Vue 基准移除，加载时强制转 sse）；
  - 页脚顺序 取消 → 保存并下一步；抽屉内无 测试连接 按钮；
  - 启用服务开关 + 「关闭后该服务不会被调用」hint 位于基本信息内（水平布局）；
  - 第 2 步依序渲染：服务用途 + usageHint、使用说明 textarea、0/16000 计数、AI 生成（未同步时禁用）、generateHint、Tools 清单 + 重新拉取；
  - 未同步态下 保存 与 AI 生成 均禁用（对齐 Vue confirm-disabled 门控）。
- 结论：R024/R043/R044/R046/N016 的浏览器级结构复验通过。剩余：Wails 运行证据、shadcn/ui 视觉扫描、可同步 MCP 服务下的已同步态取证。

