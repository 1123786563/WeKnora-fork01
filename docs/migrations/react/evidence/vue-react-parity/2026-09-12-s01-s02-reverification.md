# S01/S02 复核证据（2026-09-12，Task 3 Round）

范围：`docs/migrations/react/vue-react-parity-progress.md` Slice S01（route/guard compatibility）与 S02（public auth and no-tenant onboarding）。两片在本轮 BASE `f6a69cd8` 上代码均已存在且此前测试通过；本轮仅做**复核**（reproduce existing evidence），未修改任何 S01/S02 owned 文件。

## S01 — route/guard compatibility and hidden aliases

- 目标测试命令：`pnpm --filter @weknora/web exec tsx --test src/routes.test.ts src/chat/session-route.test.ts` → **8/8 pass**（复现绿，无需 red/green，因未改动代码）。
- Live 场景（playwright-core headless Chromium，React :5181，账号 `parity-test@local.dev`）：
  - 未登录：`/` → `/login?next=%2F`；`/join?code=X` → `/login?next=...join%3Fcode%3DX`；`/platform/knowledge-search?q=hello` → `/login?next=...knowledge-search%3Fq%3Dhello`；`/creatChat` → `/login?next=...creatChat`；`/platform/system/queues` → `/login?next=...`。`next` 全部正确保留原始 query。
  - 已登录：`/` → `/platform/knowledge-bases`；`/platform/knowledge-search?q=hello` → `/platform/knowledge-bases`（**本轮起 `cmdk` 已被 GlobalCommandPalette 消费并从 URL 中剥离**，见 command-palette 证据文件，此前该 query 曾保留在 URL 上，为 R011/N003 的已知缺口，现已闭合）；`/creatChat?agentId=a` 保持原 URL 渲染（既有别名行为，`routes.test.ts` 覆盖，未改动）；`/knowledgeBase` 原地渲染；`/platform/dev/markdown` 开发模式可访问。
- 结论：S01 所有既有行为在本轮复核中保持通过，无回归；R011 的 `cmdk` 剥离行为因 S03 变更而改善（见下）。

## S02 — public auth and no-tenant onboarding

- 目标测试命令：`pnpm --filter @weknora/web exec tsx --test src/auth/*.test.ts` → **19/19 pass**（复现绿）。
- 移动端 auth 测试文件确认：`find apps/mobile -iname "*auth*test*"` 无匹配，S02 附加条款（"mobile auth tests if files exist"）不适用，无需运行。
- Live 截图：`screenshots/login-form.png`（zh-CN 登录页，2026-09-12，session workspace 保存，未纳入仓库）确认登录表单渲染与提交成功跳转 `/platform/knowledge-bases`。
- 结论：S02 所有既有行为在本轮复核中保持通过，无回归。

## 门禁

- `tsc -b --noEmit`（web）：通过。
- `pnpm --filter @weknora/web test`：229/229（含本轮 S03 新增 21 例）。
- `pnpm run test:shared`：330/330（含本轮 S03 新增 4 例 i18n 测试）。

## 备注

S01/S02 本轮无代码改动，仅验证 + 文档/矩阵更新，因此没有对应的功能性 commit；本文件与矩阵更新一并计入 docs-only 证据提交。
