# CFT-S01-T009 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `packages/views/src/craft/library.tsx`（新增）：作品库页——授权会话列表 + 类型筛选 + 标题搜索（客户端过滤服务端分页数据）；**四态分离**（loading / 空集合 / 无匹配 / error）+ 独立"本页加载失败可重试"；无硬编码"6 件"。
- `packages/views/src/craft/templates.tsx`（新增）：模板页——5 个起步模板（版本化目标提示）；**只填表**（onUse 只回传 goal+kind，无执行/授权/知识预选）；closed kind 卡片禁用 + 原因。
- `packages/views/src/craft/home.tsx`：kind 选择器接服务端能力（closed kind 的 option disabled + 原因内联 + alert 兜底，草稿保留）；`capabilitiesFromView`（wire → domain 投影适配）；`initial` 一次性预填（模板 → 创建器）。
- `apps/web/src/features/craft/routes.tsx`：`/craft/library`、`/craft/templates` 路由；homeList 状态携带 capabilities（来自 list 响应）；模板预填 state。
- 后端：`ListCraftSessions` 响应携带 `capabilities{enabled, allowed_kinds}`（入口页在任何会话存在前就能拿到开放类型）。
- contracts：`CraftCapabilitiesView` + page parser 透传（宽松可选）。
- 装配：vite.config.ts alias + apps/web tsconfig paths + views/domain 包 exports 各补新路径（本仓库三条解析链都要登记——遗漏 vite alias 曾导致 e2e 红，已修复并全绿）。
- `packages/views/src/craft/entry-pages.test.ts`（新增，4 tests）。

## 验收断言对照

- 空目标不能提交 ✓（测试 1：goal 为空时创建按钮 disabled——既有行为钉死）
- 模板只填表不授权/执行 ✓（测试 3：无执行类按钮词汇；渲染不触发 onUse；closed kind 禁用+原因）
- 列表 404/空/无匹配/分页失败分开 ✓（测试 4：四态文案各自断言 + load-more 独立错误提示 role=alert）
- 创建成功后上传失败不重复创建 Session ✓（T008 命令桥 + routes 既有会话复用：上传失败保留附件与 sessionId，重试走 enrichedSend 同会话；e2e 01 spec 真实浏览器走该链）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `pnpm exec tsx --test packages/views/src/craft/entry-pages.test.ts` | 0 | 4 pass / 0 fail |
| `pnpm run test:craft:shared` | 0 | **94 pass / 0 fail**（90→94） |
| `pnpm run typecheck:web` | 0 | 通过 |
| `go test -count=1 -run "TestCraftHTTPList|TestCraftHTTPCapabilities" ./internal/handler/session/` | 0 | ok（list 增量字段无回归） |
| `bash e2e/craft-stack.sh up mock && run mock` | 0 | **6 passed (25.0s)**（含新路由装配后的真实浏览器全链路） |

## 未验证事项 / 回退

- /craft/library、/craft/templates 尚无专属 e2e spec（本轮以组件契约测试 + 主链路无回归为证；专属浏览器断言并入 T034 视觉回归轮）。
- 分页 loadMore 目前沿用"取下一页"语义（替换而非追加，与首页最近列表既有行为一致）；追加式分页在 T034 一并评估。
- 回退：revert 本提交（三新文件 + home/routes/contracts/handler/装配点增量）。
