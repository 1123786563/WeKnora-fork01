# T22-Web 任务报告：导出与完整删除 Web 面（Issue #162，implement）

- Worktree：`/Users/wuyongjun/.codex/worktrees/issue-140-t22-web/WeKnora-fork01`（独立，detached）
- BASE：`f6b14cf95`（Wave 9 集成 HEAD，T22 后端第 2 轮复审通过）
- HEAD：`d9b1a6d17` + 报告提交（见文末提交列表）
- 角色：frontend_implementer；只做本简报范围；未 push/merge；未改动主仓库与集成分支。

## 1. 交付物

| 文件 | 说明 |
| --- | --- |
| `packages/api-client/src/career.ts` | T22 合同段 + 五方法：`exportCareer` / `careerExportReceipt` / `careerDeletionBoundary` / `deleteCareer` / `careerDeletionReceipt`，严格解码（闭合枚举、sha256 摘要、真话状态不变式：`deleted ⟺ 全部步骤 done`、`partial ⟺ ≥1 失败步骤`、非 deleted 不得携带 completedAt） |
| `packages/api-client/src/career.test.ts` | 追加 4 个聚焦测试（编码路径、拒绝清单、删除真话契约、边界解码） |
| `apps/web/src/career/ExportDeletionPage.tsx` | 导出/完整删除页（挂载于 CareerPage `/platform/career`）：导出发起→包内容呈现→下载；删除先拉边界清单→强制确认→发起→步骤/保留范围呈现；部分失败保留原请求编号可恢复重试；删除完成自动复验旧导出回执不可读取并回调清缓存 |
| `apps/web/src/career/ExportDeletionPage.test.tsx` | 7 个用户可观察行为测试 |
| `apps/web/src/career/export-deletion.css` | TDesign 浅色主题 + `#07c05f` 品牌色样式 |
| `apps/web/src/career/CareerPage.tsx` | 最小侵入挂载（import + 1 行 render + `onCareerDeleted` 清缓存重载） |
| `apps/web/src/career/CareerPage.test.tsx` | 仅补 2 行 CSS resolve stub（页面链上新样式表的测试钩子，同 SubmissionPage.test 等先例） |

路由选择报告：按简报给出的两个选项，选择了"独立组件 + 挂载 CareerPage"（`/platform/career` 是全空间档案页，导出/删除是全空间操作；不改 router.tsx/routes.tsx/PlatformShell.tsx，最小侵入）。

## 2. RED → GREEN 证据

### RED（先写测试，记录真实失败）

- api-client（`pnpm --filter`目录下 `node --import tsx --test src/career.test.ts`）：
  ```
  ℹ tests 31 / pass 27 / fail 4
  TypeError: api.exportCareer is not a function
  TypeError: refusing.exportCareer is not a function
  TypeError: api.careerDeletionReceipt is not a function
  TypeError: api.careerDeletionBoundary is not a function
  ```
- Web 页面（`node --import tsx --test src/career/ExportDeletionPage.test.tsx`）：
  ```
  Error [ERR_MODULE_NOT_FOUND]: Cannot find module '.../ExportDeletionPage.tsx'
  ```

### GREEN（最小实现后）

- `packages/api-client`：`ℹ tests 31 / pass 31 / fail 0`
- `apps/web` 页面测试：`ℹ tests 7 / pass 7 / fail 0`（7 个行为：导出包含性呈现+下载、不确定结果原请求编号恢复、边界清单强制呈现+确认门、部分失败不称完全删除+同请求编号重试、完成态披露保留范围+旧授权失效复验+缓存清理、revision 冲突、跨租户 forbidden）
- `CareerPage.test.tsx`：`ℹ tests 13 / pass 13 / fail 0`

### 测试侧修正记录（诚实披露）

1. RED 期间发现与其他测试块的同名常量冲突（`progressEvent`），重命名为 `exportedProgressEvent`。
2. 两处删除 stub 未回显 requestId，被页面"回执与请求对账"正确拒绝——这本身是被测行为，修正 stub 回显后通过。
3. checkbox 触发方式采用仓库既有模式（prototype setter + click 派发，见 MaterialPage.test.tsx:58）。

## 3. 全量验证（真实命令与输出）

| 命令 | 结果 |
| --- | --- |
| `pnpm typecheck:web` | exit 0（修复 2 个 TS 错误后：proposal status 收窄 cast、boundaryState 联合类型补 'ready'） |
| `pnpm test:web` | `ℹ tests 2438 / pass 2437 / fail 1 / cancelled 0`；唯一 fail 是 `src/agents/agent-editor.test.tsx`——**已知并发 flaky**（见 §5），先例口径单独复跑 `node --import tsx --test src/agents/agent-editor.test.tsx` → `ℹ tests 49 / pass 49 / fail 0 / cancelled 0` |
| `pnpm build:web` | exit 0，`✓ built in 29.77s`（chunk >500kB 警告为存量提示） |
| `git diff --check` | exit 0，无输出 |

基线 2442 + 本次新增 7 = 预期 2449；全量计数 2438 = 2449 − 11（agent-editor 被杀子进程未计数的 11 个测试；该文件 49 个中 38 个已完成）。

## 4. 浏览器 E2E（T22 verified 关键验收）

隔离环境：Lite 服务器 `127.0.0.1:57816`（全新临时 SQLite DB `/tmp/weknora-t22-e2e.uOhrie/weknora.db`、memory stream、local storage、一次性账号 `t22e2e@example.test`）；Web dev server `127.0.0.1:57818`（`VITE_DEV_PROXY_TARGET=http://127.0.0.1:57816`）；真实浏览器（Playwright Chromium）。未认证 `GET /career/open` → 401；注册 201；登录 200。

数据构造层级声明：**档案事实、岗位、评估、申请（含 Task）、进展事件、材料+确认+导出发布、投递记录全部经真实后端 API 链路构造**（同一隔离 DB 与账号），非 fixture。

流程与证据（截图与导出包在 `evidence/`）：

1. **登录（真实浏览器 UI）** → `/platform/career`，页面显示"当前档案修订 1（导出与删除将按此修订提交）"（截图 01）。
2. **导出**：点击"发起导出" → 导出包面板呈现导出编号/摘要/六段包含性清单：档案事实 1 条、事实历史 1 条、岗位与原始快照 1/1、申请与进展事件 1/1、材料与版本 1/1、投递记录 1 条（截图 02）。
3. **下载导出包（认证）**：浏览器真实下载 `career-export-252b0241-….json`；结构核对（Python 独立解析）：`sections=[applications, factHistory, materials, opportunities, profile, submissions]`，各段计数与构造一致，application 内含 `taskId`；**客户端独立重算 archive sha256 = 64e24a6f7f464ea0… 与服务端 digest 逐位一致**（JSON 键序稳定时可复算）。
4. **边界清单**：点击"查看删除边界" → 呈现空间内 12 节（含计数）、外部平台资料 2 项（均"不可撤回"）、保留范围 4 行；"发起完整删除"在清单呈现且勾选确认前保持禁用（截图 03）。
5. **发起删除（勾选确认后）** → 删除结果：状态"已完全删除"、4 步骤全部已完成、完成时间、保留范围 4 行披露（截图 04）。
6. **旧授权/旧入口不可读取验证**：
   - 页面自动复验（删除前导出的回执原请求编号重放）→"删除后验证：导出回执 f6356e60-… 已不可读取（旧授权与旧入口已失效）"。
   - 浏览器同会话 API 探测：旧 API 导出回执 404、旧岗位证据 404、旧材料 404、**旧材料导出下载（旧 Artifact 授权）404**；`/career/open` 200 且 facts=0（空间 epoch 保留）。
   - 页面缓存清理：`onCareerDeleted` 回调触发 CareerPage desk 清空重载，"本科计算机科学"事实从页面消失，显示"还没有确认事实"。
   - 旧 UI 入口：直接访问旧岗位 URL → 后端 404，UI 呈现失效态"无法读取职位证据"（截图 05；OpportunityPage 既有 404 文案，非本任务范围）。
7. **部分失败场景**：真实 Lite 服务器无故障注入通道，**如实声明不可在 E2E 构造**；该路径由组件级测试（部分失败呈现/不称完全删除/同请求编号重试/保留披露）与 api-client 解码真话契约测试覆盖。

清理：浏览器已关闭；vite/服务器进程已停止（端口 57816/57818 复查无监听）；临时 DB 目录与一次性凭据文件已删除；主仓库落盘的截图/导出包已移至本 worktree `evidence/` 并从主仓库清除。

## 5. agent-editor 并发 flaky（环境事项，非本任务回归）

- 现象：全量 `--test-concurrency` 运行时 `src/agents/agent-editor.test.tsx` 停滞在 `agent-editor.test.tsx:1249`（"multimodal embeds the per-file-type parser rows…"，TDesign Select 下拉真定时器交互），高 CPU 不结束；单独运行该文件稳定 49/49（本会话 3 次验证）。
- 归属：**既有已知 concern**——T19/T20 执行记录（提交 `86040459f`、`a98837877`）原文记录"agent-editor 并发 flaky 单独复跑 49/49"。本轮另观察到并行会话（`.worktrees/issue106`）在跑跨包测试加剧机器负载。
- 因果核查：该测试的运行时模块图不含我的任何改动文件（`@weknora/api-client` 仅 type import）；A/B 复现（2 文件并发、150s 超时、各 3 次）：带改动 1 次挂/基线 0 次挂——样本不足以归因。按 T19/T20 先例处理并如实记录。
- 一次额外的存量 flake：污染运行中 `AdministrationPage debounced server-side search`（320ms debounce 计时断言）在机器高负载下失败，干净重跑通过。

## 6. 自查与遗留

- 简报"验收（Web 可观察部分）"逐条对照：导出包含性呈现 ✓、可下载 ✓（结构核对+摘要一致）、删除前边界清单强制呈现（空间内 vs 外部）✓、旧 Task/Artifact 授权/客户端缓存不可访问 ✓（旧下载授权 404、旧入口 404、desk 清空）、保留范围显式 ✓、部分失败不称完全删除 ✓（组件+解码层）。
- 遗留/声明：①部分失败 E2E 不可构造（无故障注入）；②删除 digest 的客户端逐字节校验依赖 JSON 序列化键序（Go json.Marshal 与 JS JSON.stringify 转义差异），页面不声称实时校验摘要，仅在下载文件中保留摘要供离线核验；③OpportunityPage 对 404 的"无法读取"文案属存量 UI 行为。
- 迁移：无数据库迁移（T22 后端迁移已在 Wave 9 集成）。

## 7. 提交列表

见 `git log f6b14cf95..HEAD`：实现提交 `d9b1a6d17 feat(web): export and completely delete career data` + 本报告/证据提交（HEAD 以最终 `git log` 为准）。
