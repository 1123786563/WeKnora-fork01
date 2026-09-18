# CFT-S00-T003 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `packages/ui/src/theme.css`：新增 `.wk-craft` 作用域语义别名块（27 个 `--craft-*` 别名全部解析既有源变量；含两项设计 §2.1 可访问性修正：`--craft-action-fg: var(--color-ink)` 绿按钮深色前景、`--craft-secondary-text: var(--color-muted-strong)` 画布安全次文本）。
- `packages/views/src/craft/craft.css`：17 种硬编码色值 → `var(--craft-*)`（品牌蓝 7 处、危险红 8 处、边框 12 处等全部替换；仅终端视图保留 2 个功能色字面值 #10151f/#d7e3f4 并注释）。
- `packages/views/src/craft/home.tsx` + `workbench.tsx`：根元素挂 `.wk-craft` 作用域类（`className="wk-craft wk-craft-page"`）——别名的作用域前提。
- `packages/design-tokens/src/craft.test.ts`（新增，4 tests）。

## 验收断言对照

- 蓝品牌仍是 #2e6de6、绿动作仍是 #07c05f ✓（测试 1：designTokens 源断言 primary/accent/accentHover/accentActive/canvas）
- 未在 Craft 外覆盖全局 primary ✓（测试 2：别名仅在 `.wk-craft` 选择器内；`@theme` 全局块 `--color-primary` 唯一定义且无 `--craft-` 泄漏——注意 theme.css 是 Tailwind v4 `@theme` 结构而非 :root，断言按真实结构切片）
- 绿色正常字号按钮前景至少 4.5:1 ✓（测试 3：WCAG 公式计算 #172033 on #07c05f = 7.0+；白字 2.3 记录为被修正项）
- 附加：craft.css 品牌色字面值 0 散落（测试 4）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `pnpm exec tsx --test packages/design-tokens/src/craft.test.ts` | 0 | 4 pass / 0 fail |
| `pnpm run test:craft:shared` | 0 | 68 pass / 0 fail（无回归） |
| `pnpm run typecheck:web` | 0 | tsc 通过（含改动的 home/workbench） |
| `bash e2e/craft-stack.sh up mock && run mock` | 0 | **6 passed (26.3s)**——令牌替换后全栈浏览器验证无回归（workbench-after-token-scope.png） |

## 未验证事项 / 回退

- 绿色按钮（@weknora/ui Button primary）的前景修正需在 Button 层接入 `--craft-action-fg`——Button 是共享组件且其 primary=绿映射属 Tailwind 桥（tokens.ts tailwind.colors.primary），本轮不全局改动（禁令：不把所有 primary 强制改色）；craft 作用域内消费 `--craft-action-fg` 由 T004（共享壳）/T010（工作台）在 craft 专用按钮上落实。
- 暗色主题：本轮仅浅色（设计边界），`.wk-craft` 别名解析源变量，暗色覆盖属宿主既有机制，未验证不声称。
- 回退：revert 本提交（theme.css 块为追加、craft.css 为变量替换、tsx 为单行类名）。
