# CFT-S01-T011 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `packages/domain/src/craft/version-selection.ts`（新增）：版本/来源状态独立性的纯规则——`craftVersionSelection`（view 只动 viewVersionId；restore 移基线+revision 恰好 +1 且**不发布任何版本**；select-sources 永不改已发布 manifest 的引用）、`sourceAccessibility`（撤权 → 'revoked' 占位，永不透出摘录载荷）、`restoreEligibility`（**可下载 ≠ 可恢复**：恢复需写权+无活动任务+完整快照；文件下载恒可用；禁用必带可读原因）。
- `packages/domain/src/craft/version-selection.test.ts`（新增，5 tests）：四组验收断言 + restore 单调推进。
- `packages/views/src/craft/versions.tsx`（新增）：版本抽屉内容（P06）——版本行（基线徽标/时间/run/检查计数/查看/下载/继续）+ **恢复二次确认 Dialog**（复用 ui Dialog 焦点契约；文案明示恢复源、"历史版本不会被覆盖"、"下一轮交付才发布新版本"）；恢复禁用带原因。
- `packages/views/src/craft/versions.test.tsx`（新增，2 tests：SSR 规则渲染 + JSDOM 点击确认对话框）。
- `packages/views/src/craft/sources.tsx`：`revokedCitationIds` prop——撤权行只保留引用占位（标题/摘要/大小均占位、打开按钮禁用+title 原因），不显示缓存原文。
- domain 包 exports + typecheck:shared 接线。

## 验收断言对照

- 查看 v1 不改变 base=v3/revision ✓（domain 测试 1：仅 viewVersionId 移动，versions deepEqual 不变）
- 下一次来源选择不修改 v1 引用 ✓（domain 测试 2：select-sources 后 v1 citations 原样）
- 撤权不显示原文缓存 ✓（domain sourceAccessibility + sources.tsx 占位行渲染——`data-revoked` 行无标题/摘要/大小内容）
- 缺快照禁用恢复但可读下载 ✓（domain 测试 4 全矩阵 + 组件测试 1：v2 下载可用、恢复禁用且原因可见）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `pnpm exec tsx --test packages/domain/src/craft/version-selection.test.ts` | 0 | 5 pass / 0 fail |
| `pnpm exec tsx --test packages/views/src/craft/versions.test.tsx` | 0 | 2 pass / 0 fail |
| `pnpm run test:craft:shared` | 0 | **104 pass / 0 fail**（97→104） |
| `pnpm run typecheck:shared` / `typecheck:web` | 0 / 0 | 通过 |
| `bash e2e/craft-stack.sh up mock && run mock` | 0 | **6 passed (26.6s)** 无回归 |

## 与既有实现的关系

- workbench 顶栏版本选择/恢复入口（W05/C05 既有，`restorableVersionIds` 服务端集合）与本任务规则一致——domain 规则把该语义升为纯函数并钉死；versions.tsx 是抽屉形态的正式视图（挂载进 CraftDrawer 属视觉轮 T034 组合；顶栏入口已可用且 e2e 持证）。
- 撤权行的 ACL 事实来源（哪个 citation 被撤）由装配层从打开错误/服务端提示回填（`revokedCitationIds`）；打开路径本就每次重走权限链（C01 语义）。

## 回退

revert 本提交（三新文件 + sources props 增量 + exports/typecheck 列表）。
