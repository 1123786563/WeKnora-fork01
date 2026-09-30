# Wave 2 — T06 真实环境缺陷修复：D1/D2/D3（implement/fix）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 使用 superpowers `systematic-debugging` 方法论修复三个已定位根因的真实运行时缺陷。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t06-gui-fix/WeKnora-fork01 --detach 7cbad8941
cd /Users/wuyongjun/.codex/worktrees/issue-140-t06-gui-fix/WeKnora-fork01
```
BASE = `7cbad8941`。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 背景

T06（微信小程序 Task 产物下载）代码已集成且 53/53 单测 + build:weapp 绿，但 Wave 1 真实微信开发者工具实测（报告：`docs/plans/issue-140/task-6-live-validation.md`，先完整读它）发现三个真实运行时缺陷，其中 D1/D2 使"官方构建产物 GUI 链路验收"未达成——修完并真实 DevTools 复验通过后 T06 才可能 verified（进而解锁 T24/T26/T28/T30/T32）。

## 3. 三个缺陷与根因（报告 verbatim 摘录）

**D1 — 官方构建产物在真实 DevTools 无法启动。** `src/subpackages/execution/artifact/index.config.ts` 的 `usingComponents: {'t-button': 'tdesign-miniprogram/button/button'}` 编译进 `dist/subpackages/execution/artifact/index.json` 后变成 `/npm/.pnpm/tdesign-miniprogram@1.17.0/node_modules/...` 路径，而构建从不产出 `dist/npm`；DevTools 报 `[BackendInitEnv] getAppJSON error … 路径下未找到组件` 白屏。实测者用 gitignored 构建产物补丁（拷 `node_modules/tdesign-miniprogram/miniprogram_dist` 到 `dist/npm/tdesign/...` 并改页面 JSON 引用）本地解锁——**那是临时补丁不是源码修复**；且路径含 `node_modules` 段的引用 DevTools 一律不接受。
**D2 — 产物页唯一动作按钮在真实运行时无事件。** `createElement('t-button', {onTap})` 的 `onTap` 被此 Taro 构建对 string-tag 自定义组件丢弃（编译出的 `dist/base.wxml` 中 `t-button` 分支无 `bindtap`）；模板级补 `bindtap="eh"` 也无法送达 React handler。同页原生 button（重新读取）点击可靠触发请求——证明事件链健康、问题特定于 t-button 的使用方式。
**D3 — "受控删除临时文件"在 DevTools 静默失败。** `openProtectedDocument` finally 块 `unlinkSync(tempFilePath)` 对 `http://tmp/…pdf` 抛 `permission denied`，403 字节临时文件事后仍存在，而 UI 承诺"临时打开后立即清理"。

## 4. 修复要求

**必须保持 Spec：小程序使用 TDesign Miniprogram（Taro 4 + TDesign Miniprogram）**。不得把 t-button 静默替换为原生 button 冒充修复——若你经 systematic-debugging 论证 TDesign 组件在此 Taro 构建下确实无法在真实 DevTools 工作，如实报告根因与证据（复现命令+输出），由主控裁决替代方案；这是升级路径，不是自作主张授权。可行方向提示（非限定）：修正 usingComponents 引用使 Taro 把 `tdesign-miniprogram/miniprogram_dist` 拷入 dist 并以可接受路径引用（如 config copy 选项或相对路径映射）；D2 检查 Taro 对自定义组件事件 prop 的注册机制（可能需用 Taro 注册的自定义组件包装而非 string-tag createElement）；D3 改用合适的小程序文件 API（如 FileSystemManager 异步 unlink / 先拷贝到可写目录再清理）并让 UI 文案与真实行为一致——承诺必须真实成立，或如实降级文案并在报告记录。

TDD：每个缺陷先在 `apps/miniprogram/tests/` 写能表达缺陷的失败测试（D1/D2 属构建/运行时问题，若单测无法表达，用最小构建产物断言测试——如断言编译后页面 JSON 不含 node_modules 路径段、事件绑定存在于产物——能写多少写多少，并在报告声明单测覆盖边界；真实判据是 DevTools 复验）。D3 可在服务/适配器层单测（清理失败时的行为与状态）。

## 5. 文件所有权

`apps/miniprogram/src/subpackages/execution/artifact/`、`apps/miniprogram/config/index.ts`、`apps/miniprogram/src/services/workbench.ts`（如需）、`apps/miniprogram/tests/`、`apps/miniprogram/project.config.json`/`project.private.config.json` 模板（如需纳入 urlCheck=false 的本地开发说明，保持 gitignored 约定）。不改后端、不改 Web/移动目录、不动 13 个基线既有的 CommercialSummary typecheck 错误。

## 6. 验证（全跑附原始输出）

```bash
pnpm --filter @weknora/miniprogram test            # ≥53/53（新增测试后更多），0 fail
pnpm --filter @weknora/miniprogram build:weapp     # 成功（既有 size/async-chunk 警告可接受）
git diff --check
```
**真实 DevTools 复验（本轮核心验收，不可省略）**：复现提纲在 `docs/plans/issue-140/task-6-live-validation.md` 末尾（服务器端口本轮分配 **57804**；CLI `cli auto --project apps/miniprogram --auto-port <port>` + `miniprogram-automator`（装在仓库外）；冷编译 ≥45-60s 需 settle-and-retry）。必须达成：官方构建产物**不经任何手工补丁**加载启动 → GUI 登录 → 任务列表 → 产物页 → **真实点击下载按钮**走应用自身 `openProtectedDocument` 全链路（mint→download→openDocument）→ SHA-256 三方比对 → 临时文件清理行为与 UI 承诺一致 → 过期 grant GUI 提示（"重新获取"）。跨租户拒绝至少 HTTP 层复核。D2 修复的判据是**GUI 点击**（非请求层 seam）触发 signed-url POST 到达服务器。

## 7. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree；local commits authorized; no push/merge/deploy/GitHub action；禁止派发子 agent。
- 不长期缓存 URL；按用户动作取授权；下载前后检查 scope epoch。
- 不绕过登录/反爬；测试数据库、端口（57804）、构建目录隔离。
- 报告只写事实与实测输出；若真实 DevTools 复验仍无法达成（如 TDesign 路线死路），如实报告 blocked 附完整证据，绝不伪造 GUI 通过。
- 凭据不入库：fixture 密钥/disposable 账号只存在于临时进程与目录，结束即清理；报告不保留 token/signed URL。

## 8. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t06-gui-fix/` 下写 `task-report.md`：BASE/HEAD、每缺陷的根因分析→修复→测试→复验证据（命令+输出+截图清单）、单测覆盖边界声明。COMMIT 建议：每缺陷一个 commit（`fix(miniprogram): <缺陷>`）。最终消息报告 HEAD SHA、53+/测试结果、build 结果、真实 DevTools 复验各步结果。留在 worktree 等独立评审与集成；verified 由主控裁决。
