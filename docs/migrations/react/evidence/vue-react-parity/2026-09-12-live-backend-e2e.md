# 真实后端隔离环境与首轮 live E2E（2026-09-12，Round 9）

## 环境搭建（可重复）
- 基础设施：`make dev-start`（docker compose dev：postgres:5432/redis:6379/docreader:50051，本地 .env 由 .env.example 改 host 为 localhost；langfuse-init 竞态失败可忽略）。
- 后端：`make dev-app` → :8080（/api/v1/auth/config 200）。
- React dev：`VITE_API_BASE_URL=http://localhost:8080 vite --port 5181`；Vue dev：:5180（CORS 后端全开）。
- 测试账号（本地隔离库）：parity-test@local.dev / Parity123456!（self_serve 注册，tenant 10000）；测试 KB：产品知识库、Parity KB Demo。

## 首轮 live E2E 结论
- React：UI 登录 → 跳转 /platform/knowledge-bases 成功（round-2/3 auth 修复实测通过）；KB 列表 i18n 文案、未初始化横幅、卡片警告样式、收藏星、置顶/副本/编辑/设置/删除操作、类型徽章+计数全部渲染（screenshots/kblist-react-live.png）。
- Vue：同账号登录成功，登录成功 toast、新手引导 1/7、左侧导航（新对话/知识库/智能体/共享空间）+ 图标轨（全部/收藏/最近/本空间）（screenshots/kblist-vue-live.png）。

## 新发现的结构性缺口（登记为高优待办）
1. **平台外壳缺失**：React 各 /platform 页面为裸页，无 Vue 的全局导航外壳（左侧栏、图标轨、页头、toast、新手引导、用户菜单）——影响所有 /platform 页面验收（对应 T04/T05 shell 范畴）。
2. React KB 列表一处调试输出 `scope key: [...]` 已修复移除（本次提交）。
3. React 未初始化卡片边框为黄色高亮，Vue 为普通卡片+顶部横幅（视觉细节待对齐）。

## 注
- 本轮 web 全量测试中 4 个文件失败为 chat 实施子代理进行中修改（message-list.tsx 模块解析）所致，与本轮改动无关；集成时以子代理完成态统一验证。
