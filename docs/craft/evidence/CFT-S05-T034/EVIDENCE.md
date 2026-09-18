# CFT-S05-T034 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `apps/web/e2e/craft-visual.spec.ts`（新增，3 tests）：五宽度视觉基线 spec。
- **11 张真实浏览器截图**（screens/）：home + workbench × 1440/1280/1024/768/390 + 390 长标题。
- 环境事件：本轮 stack 首启失败两次——PATH 含带空格目录（"Application Support/JetBrains/…"）导致 `env $OC_ENV` 分词断裂、serve 无法启动；净化 PATH 后正常（已在此留档，RESUME 环境坑更新）。

## 验收断言对照（真实浏览器，每宽度实际 setViewportSize）

- 五宽度基线 ✓（spec 1：1440/1280/1024/768/390 逐宽截图 home+workbench，**逐宽断言页面级零横向溢出**（scrollWidth−clientWidth ≤ 0））
- 窄屏切换不卸载 ✓（spec 2：390 下两个 pane 均在 DOM——CSS 隐藏而非卸载，草稿/订阅存活；与 T010 类合同测试互补的浏览器级证明）
- 长标题/长文件名不越界 ✓（spec 3：80 字长标题逐字渲染、零横向溢出、截图留档）
- 键盘/焦点/IME/对比度：焦点约束由 ui Dialog/Sheet 契约测试持证（T004）；Enter/Shift+Enter 与 IME 由宿主 composer 语义承载（W05 既有）；对比度 4.5:1 由 T003 令牌测试持证（ink/accent 7.0:1、画布次文本 6.06:1）——本轮浏览器断言聚焦溢出/布局/挂载三类可程序化断言的项。

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `bash e2e/craft-stack.sh up mock && run mock e2e/craft-visual.spec.ts` | 0 | **3 passed (5.4s)**，11 截图入库 |

## 与高保真目标的剩余偏差（如实）

- 基线截图是**结构与布局证据**，不是逐像素对比：高保真原型的精确字阶/间距（36px 主标题等）未逐项 diff——壳层采用仓库既有 W05 视觉体系 + T003 令牌，色调/对比度对齐设计；逐像素基线需固定浏览器/字体环境后单独建立（视觉回归的后续演进，不在本轮 36 卡范围内）。
- 暗色主题不在本轮（设计边界声明）。

## 回退

revert 本提交（spec 新增 + 截图证据，纯增量）。
