# 双端功能一致性测试（T17.5 / R17 最终功能验收）

- 日期：2026-09-24（矩阵执行）；仓库：main @ 898864c07（T16 合并后）
- 端点：Vue http://localhost:5174 / React http://localhost:5175 / 后端 :8084
- 账号：`~/.weknora-func-creds.env` 的 `FUNC_TEST_EMAIL` / `FUNC_TEST_PASSWORD`（真实邮箱账号）
- 方法：repo playwright（apps/web `@playwright/test` headless chromium，1280×720），
  **双端均通过 UI 登录表单登录**（输入邮箱密码点「登录」，未注入 localStorage）。
  每步 DOM 探测记录行为结果 + 截图取证（本目录 `png/`，54 张，命名 `<步骤>-<端>`）。
- 写操作最小化：语言偏好 zh→en→zh 改回原值；agents 创建弹层仅打开后取消；
  聊天流按矩阵发送 1 条消息（每端各产生 1 个新会话，内容标注「自动测试，可忽略」）；
  未创建/删除任何知识库、智能体、组织实体。

## 环境偏差说明

1. **Parity KB Demo 不在本账号**：该 fixture 属 parity 测试账号（`~/.weknora-parity-creds.env`），
   功能账号知识库列表仅有 1 个文档型 KB「11」（45 文档）。矩阵第 4 项改用 **KB「11」** 执行
   （只读：进入详情 + tab 切换）。
2. **FAQ tab 不适用**：KB「11」为文档型，双端 tab 条均只有 文档 / Wiki / 图谱（FAQ tab 仅
   FAQ 型 KB 存在）。双端 tab 集合与切换行为一致，「FAQ 切换」子项双端一致地不可执行。
3. 新手引导（「欢迎使用 WeKnora」遮罩）在 fresh 浏览器上下文必然出现，双端均以「跳过引导」
   关闭后继续（该行为本身也验证了一致）。

## 测试矩阵

| # | 项目 | Vue 结果 | React 结果 | 一致 |
|---|------|----------|------------|------|
| 1a | 错误密码登录 | 报错 "Invalid email or password"，停留 /login | 同左（同文案） | ✅ |
| 1b | 正确登录 | 进入 /platform/knowledge-bases | 同左 | ✅ |
| 2a | 侧栏入口：知识库/智能体/共享空间 | 三项点击均可达对应路由 | 同左 | ✅ |
| 2b | 用户菜单「设置」入口 | 打开用户菜单→「个人设置」→ 设置页 | 同左流程（**但落点 section 不同，见不一致 #1**） | ❌ |
| 2c | 侧栏折叠/展开 | 折叠↔展开成功（.sidebar-toggle / .sidebar-toggle-item） | 同左 | ✅ |
| 3 | 聊天流：新建对话→发送消息 | 新对话页、composer 可输入、发送按钮激活；消息上屏、composer 清空、无错误提示、无助手气泡（LLM 未配置，形态与 React 相同） | 同左（DOM 状态完全一致） | ✅ |
| 4a | KB 列表加载 | 列表加载（KB「11」，卡片可见） | 同左 | ✅ |
| 4b | 进入 KB 详情 | 点击卡片进入 /platform/knowledge-bases/66342241-…（同 KB id） | 同左 | ✅ |
| 4c | tab 切换（文档/FAQ/Wiki） | tab 条=文档/Wiki/图谱；Wiki 切换 urlTab=wiki；文档切回正常；FAQ tab 不存在（文档型 KB） | 同左（tab 集合与切换行为逐项相同） | ✅ |
| 5a | 设置打开 general / userprofile | 两 section 均正常渲染（语言/主题、邮箱/资料） | 同左 | ✅ |
| 5b | 语言切换 zh→en→zh | 选 English 后 UI 全量切英文（Knowledge 出现/知识库消失），改回简体中文后恢复 | 同左 | ✅ |
| 6a | agents 列表加载 | 列表加载（卡片节点渲染） | 同左 | ✅ |
| 6b | 创建弹层打开+取消 | 编辑器弹层打开（模型已就绪路径），取消后关闭 | 同左 | ✅ |
| 7 | 登出 | 用户菜单→退出→回 /login | 同左 | ✅ |

补充观察（双端一致的行为，非矩阵必测）：无模型就绪竞态时点击「创建智能体」双端均走守卫路径
（警告 + 跳转 settings?section=models），守卫代码两端对齐（Vue AgentList.vue:1597-1602 /
React AgentsPage.tsx:1044-1055）。

## 不一致清单

### #1（实锤，建议修复）用户菜单「个人设置」落点 section 不同

- 现象：双端用户菜单均有同名入口「个人设置」（i18n `general.personalSettings`），点击后：
  - Vue → `/platform/settings?section=general`
  - React → `/platform/settings?section=userprofile`
- 源码定位：
  - Vue `frontend/src/components/UserMenu.vue:77-80`：`handleQuickNav('general')`
  - React `apps/web/src/platform/PlatformShell.tsx:1265-1268`：`href="/platform/settings?section=userprofile"`
- 影响面：同名菜单项导航目标不同；且双端的用户菜单头部卡片均指向 userprofile
  （Vue UserMenu.vue:35-38 / React PlatformShell.tsx:1225-1228），Vue 因此保留 general 入口、
  React 因此丢失 general 直达入口——菜单功能集合也出现偏差。
- 截图：`png/02-user-menu-open-{vue,react}.png`、`png/02-personal-settings-landed-{vue,react}.png`、
  `png/02-nav-settings-{vue,react}.png`
- 建议：以 Vue 为基线，将 React PlatformShell.tsx:1265 的菜单项 href 改为 `?section=general`
  （头部卡片保持 userprofile）。

### 备注（非缺陷，仅记录）

- Vue 设置 general 面板 DOM 存在 1 个隐藏空 `.t-select` 节点（0×0，React 无）：不可见、无功能影响。
- 新手引导 DOM 前缀不同（Vue `.guide__*` / React `.wk-guide__*`）：实现层差异，可见行为一致
  （「跳过引导」均生效）。

## 结论

7 大项 / 14 子项矩阵中 **13 项双端一致，1 项不一致**（用户菜单「个人设置」落点 section，
React 偏离 Vue 基线，源码级定位明确、修复面小）。核心流程（登录、导航壳、聊天流、KB 列表与
tab、设置读写含语言切换、agents 弹层、登出）双端行为一致。**在修复不一致 #1 之前，
功能一致性验收判为 PASS with 1 finding**；该单项不影响其余流程，建议作为收尾修复项并入
React 侧小改后复测该项即可。

凭据未写入本报告与源码（引用 `~/.weknora-func-creds.env` 键名）。
