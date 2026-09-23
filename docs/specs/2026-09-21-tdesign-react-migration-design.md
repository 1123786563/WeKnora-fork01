# React 端 TDesign 同构迁移设计（tailwind/shadcn 退役）

- 日期：2026-09-21
- 状态：已实施（2026-09-23 T16 三轮全量验收 PASS：60 项均值 0.09%、9 项豁免像素级取证；tailwind/@weknora/ui 已删除。验收证据：docs/migrations/react/evidence/vue-react-parity/final-acceptance/2026-09-23-final-acceptance.md；执行账本：.superpowers/sdd/2026-09-21-tdesign-react-migration/progress.md（Rulings R1-R21））
- 关联：`docs/specs/2026-09-16-react-client-routing-design.md`、parity 台账 `docs/migrations/react/evidence/vue-react-parity/`、`scripts/parity/auto-scan.mjs`

## 1. 背景与动机

WeKnora 双前端长期通过像素扫描对齐 React（`apps/web` + `packages/views`）与 Vue（`frontend/`）的 UI。经 11 轮修复，60 项扫描均值已从 15.08% 收敛至 5.36%，但残差已进入"不可修"区间：

- **TDesign AA 类残差**：Vue 端组件是 tdesign-vue-next 原生实现（`t-*` 类名 + `--td-*` CSS 变量），React 端用 Tailwind + 自研 shadcn 风格组件（`@weknora/ui`，Radix + cva）手工仿制其外观，非同源实现，像素永远无法完全对齐。
- 每轮修复都在给 React 端追加手写 parity 补丁 CSS（`kb-editor-parity.css`、`agents.css`、`settings-wrapper.css` 等），维护成本持续上升。

**根因解法**：React 端整体切换到 tdesign-react + tdesign-icons-react，与 Vue 端共用同一设计系统与类名体系，让像素一致成为"构造出来的"而非"调出来的"。

## 2. 现状事实（设计依据）

| 维度 | Vue 端（frontend/） | React 端（apps/web + packages/views） |
|---|---|---|
| 框架 | Vue 3.5 | React 19.3 + TanStack Router + Vite 7 |
| UI 库 | tdesign-vue-next ^1.19.2（npm 最新 1.20.8） | `@weknora/ui`（Radix + cva + tailwind-merge，16 个组件，peer `react>=19`） |
| 图标 | tdesign-icons-vue-next 0.4.4（overrides 锁定） | 无统一图标库，手写对齐 |
| 样式 | 205 个 .vue 文件，样式内联于 SFC `<style>` 块；仅 21 个独立 less/css | Tailwind v4（无 preflight，`styles.css` 359 行手工对齐 body 字体/平滑）；94/135 个 tsx 用 Tailwind 工具类做布局 |
| 主题 | `frontend/src/assets/theme/theme.css`（含 `--app-font-family`） | `packages/design-tokens` |

npm 查证（2026-09-21）：

- **tdesign-react 1.18.3，peerDependencies `react >= 16.13.1` —— 涵盖 React 19，安装无 ERESOLVE 冲突**。
- tdesign-icons-react 0.6.11，peer 同上。
- 版本错位：tdesign-react（1.18.3）落后 tdesign-vue-next（1.19.x/1.20.x），两库为独立实现，个别组件 DOM/默认行为存在细微差异，需 pilot 逐组件核对。

parity 工具链：`scripts/parity/auto-scan.mjs`（Playwright 双 context 1280×720 截图，页面清单含 settings 23 个 section + integrations 6 个 tab）+ `pixdiff.py`（容差 8）；Cron automation-41aaa5e6 每 30 分钟扫描+修复闭环。

## 3. 目标

1. React 端 UI 层与 Vue 端**同构**：tdesign-react + tdesign-icons-react + 平移的页面 CSS。
2. **逐项 0 diff**（验收口径见 §8）。
3. **彻底移除 Tailwind 与 shadcn 栈**：apps/web 最终不依赖 tailwindcss / @tailwindcss/vite / @weknora/ui / cva / tailwind-merge；删除绝大多数 parity 补丁 CSS。

## 4. 非目标

- 不改 Vue 端（除非撞到 TDesign 版本 bug 必须绕行，且需在实施计划中单列）。
- 不改后端、不做任何功能/交互变更——纯 UI 同构重写。
- apps/desktop、apps/mobile 不动；apps/embed 仅在其依赖的 packages/views 部分被牵连的范围内处理。
- 不追求绝对容差 0 的逐像素为零（跨框架渲染管线下亚像素抗锯齿差异不可消除，见 §8）。

## 5. 已定决策（用户拍板，不再重议）

| 决策点 | 结论 |
|---|---|
| 方案选型 | 方案一"同构重写"（否决"适配层过渡"——shadcn API 与 TDesign DOM 差异大，适配层达不成 0 diff 且等于做两遍） |
| Tailwind 终态 | 彻底移除，React 端终态与 Vue 端同为"TDesign 组件 + 手写 CSS"，按页渐进执行 |
| 验收标准 | 逐项 0 diff（pixdiff 容差 8 口径下每项 0.00%，含交互态） |
| `@weknora/ui` 处置 | 直接删除，页面直接 import tdesign-react（与 Vue 端写法 1:1） |
| 迁移节奏 | 按页渐进，Phase 4 前新旧两套 UI 库并存（已迁页面不再引用旧栈） |

## 6. 终态架构

- **组件**：页面直接 `import { Button } from 'tdesign-react'`；图标 `tdesign-icons-react`。DOM 结构复刻 Vue 模板，`t-*` 类名天然一致。
- **样式**：页面级普通 CSS（从 Vue SFC `<style>` 块平移，类名与 Vue 端一致）；scoped 选择器改显式类名前缀（平移规则在 Phase 1 固化）。TDesign reset + 主题置于 styles.css 最前层；现有 body 字体/平滑处理保留。
- **主题**：`frontend/src/assets/theme/theme.css` 的 token 平移进 `packages/design-tokens`，`--td-*` 覆盖与 Vue 端逐项核对。
- **测试 hook**：`wk-*` 类名保留在 DOM 上（现有测试以其为查询 hook），只换样式实现不删 hook。
- **删除物**：`@weknora/ui` / packages/ui、tailwindcss、`@tailwindcss/vite`、cva、tailwind-merge、parity 补丁 CSS、styles.css 的 theme/utility 层。

## 7. 阶段设计

### Phase 0 — Spike：React 19 验证闸门（通过才继续）

1. apps/web 安装 tdesign-react@1.18.3 + tdesign-icons-react@0.6.11（pnpm，无 peer 冲突）。
2. 引入 TDesign reset/主题，验证与 styles.css 现有 layer 组合无冲突。
3. 验证矩阵：Button / Input / Select / Table / Dialog / Tabs / Switch / Tooltip 的渲染与事件；**重点压 Message / Notification 命令式 API**（React 19 移除 `ReactDOM.render`，唯一可能触雷处）。
4. 图标抽检：20 个常用图标，比对 icons-react 0.6.11 与 Vue 端锁定的 icons-vue-next 0.4.4 渲染像素。
5. **退出决策树**：
   - 全部通过 → Phase 1；
   - 命令式 API 触雷 → patch-package 将 `render` 替换为 `createRoot` 后复测；
   - patch 不可行 → 评估降级 React 18（apps/web 未用 React 19 专有 API，需核对 TanStack Router / recharts 兼容）；
   - 仍不可行 → 止损汇报，迁移取消，React 端维持现状栈。

### Phase 1 — 基建

1. 依赖与样式管线：TDesign reset + 主题引入顺序固化；styles.css 重组。
2. 主题 token 平移（frontend theme.css → packages/design-tokens），`--td-*` 覆盖逐项核对。
3. TDesign 组件内置文案（分页"共 x 条"、日期选择器等）locale 与 Vue 端对齐。
4. 过渡期并存约定：Phase 4 前旧栈保留可用，已迁页面禁止新增引用。
5. 固化"Vue SFC → React + CSS"平移规则文档（样式块提取、scoped 改显式前缀、`wk-*` hook 保留）。

### Phase 2 — Pilot + 扫描工具稳态化

Pilot 页：**agents**（`/platform/agents`：表格+弹窗+表单全覆盖，parity 清单内，历史修复最多）。

流程 SOP：Vue 源码对照 → React 重写 → CSS 平移 → 删该页 Tailwind → `PAGES=agents` 单页扫描 → 0.00% 收敛 → 沉淀 playbook（含 tdesign-react↔vue-next 组件 DOM 差异台账）。

**扫描器改造（本阶段一并完成，0 diff 标准的前提）**：

- 截图前稳态门：网络空闲 + 字体加载 + 动画停止，消除"Vue 瞬态空白"伪差；
- hover / 弹层交互探针加确定性等待。

### Phase 3 — 按页全量（每批完成 = 批内每页扫描 0.00%）

| 批次 | 页面 |
|---|---|
| 1 平台核心 | kb-list、kb 详情、orgs、apps、apps-connections、creatchat |
| 2 设置与集成 | settings 23 个 section + integrations 6 个 tab（packages/views/settings、integrations） |
| 3 对话与文档 | chat 家族（packages/views/chat，最复杂）+ documents、faq |
| 4 其余全部 | analytics、wiki、market、experts、administration、embed、guides、craft、404 等 |

- 页面间相互独立：按 AGENTS.md 规则可用独立 Git worktree 并行，主会话负责集成。
- 单页验证：`PAGES=<id> node scripts/parity/auto-scan.mjs`。

### Phase 4 — 清理与验收

1. 删除 `@weknora/ui` / packages/ui、tailwindcss、`@tailwindcss/vite`、cva、tailwind-merge、parity 补丁 CSS、styles.css theme/utility 层；清理 workspace 引用。
2. 全量 60 项扫描：逐项 0.00% × **连续 3 轮**稳定。
3. 恢复 parity automation（automation-41aaa5e6）为守护模式。
4. 记录构建产物体积对比。

## 8. 验收标准

- **逐项 0 diff** = `auto-scan` 60 项清单中每项的差异像素占比为 **0.00%**（pixdiff 容差 8，1280×720 稳态截图口径，与历史 15.08%→5.36% 及 0.5% 阈值同源）。
- 交互态（hover / 弹层打开态）在受控探针下同样 0.00%。
- 最终验收：全量扫描逐项 0.00% 连续 3 轮。
- 诚实边界：容差 0 的绝对逐像素为零跨框架不可物理达成，本口径即为可达成的最严格标准。

## 9. 风险与回滚

| 风险 | 缓解 |
|---|---|
| tdesign-react 1.18.3 与 vue-next 1.19.x 版本错位，个别组件 DOM 差异 | Pilot 起逐组件核对并记台账；差异项用 CSS 补齐，或评估版本对调（vue-next 降/升版本需另行评估，默认不动 Vue 端） |
| React 19 运行时不兼容（`ReactDOM.render` 移除） | Phase 0 闸门 + 三级决策树（patch → 降级 18 → 止损） |
| icons-react 0.6.11 与 icons-vue-next 0.4.4 图标差异 | Spike 抽检 20 个；全量扫描兜底捕获 |
| 双 UI 库并存期包体积上升 | 过渡期接受，Phase 4 消除并记录对比 |
| `wk-*` hook 变更破坏现有测试 | 约定 hook 保留；涉及测试改动随页提交 |
| 迁移与 parity 自动化互相收割 | 开工即停 automation-41aaa5e6 自动修复（保留手动扫描），Phase 4 恢复 |

**回滚**：按页独立提交，可单页 revert；Phase 0/1 改动集中于独立分支，可整体放弃。

## 10. 运维与前置条件

1. **开工前必须先收尾提交当前工作树未提交的 parity 修改**（2026-09-21 时点约 30 个文件处于 modified 状态），严禁与迁移分支混合。
2. 迁移全程在独立分支/worktree 进行，主分支只进过验收的批次合并。
3. 本 spec 与实施计划（writing-plans 产出）冲突时，以本 spec 为准；实现阶段发现 TDesign 同源差异不可调和时，升级回设计阶段重新决策，不得静默偏离。
