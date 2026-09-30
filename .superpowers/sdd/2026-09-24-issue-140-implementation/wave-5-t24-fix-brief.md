# Wave 5 — T24 修复轮 1/5：F1/F2/F3（fix-resume）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 使用 superpowers `systematic-debugging`。你的 Wave 4 四提交已按主控指令集成（92ea9fe83/cc7a45466/63f0b5e56/a515b8b50 ← 本地链至 9ef4ee4c0），但独立评审留有 **三项 medium**，修复并复审通过前 T24 不 verified。

## 1. Worktree（沿用你的现场，不换基）

```bash
cd /Users/wuyongjun/.codex/worktrees/issue-140-t24-miniprogram/WeKnora-fork01
git status --short   # 必须 clean（调度员 2026-09-25 实核为空，HEAD 9ef4ee4c0）
```
继续在此 HEAD 上追加修复提交（集成侧已有你的 cherry-pick 副本；本地提交独立编号，集成时集成员对账）。绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 三项 medium findings（Wave 4 集成结果载明）

- **F1 额度不足专属提示为死代码**：评审认定"额度不足专属提示"代码路径不可达。修复二选一（报告说明依据）：(a) 使其真实可达——mock/测试注入真实触发路径并在 UI 呈现专属提示（后端 T11 SearchQuotaGate seam 已存在，fake 注入可触发 typed 拒绝）；(b) 论证保留死代码无理则删除，统一走通用失败态（Spec 允许"额度不足给可操作状态"由通用态承担时需评审认可）。禁止第三态：留着不可达代码不处理。
- **F2 未知对账 404 分支无 UI 恢复**：未知结果对账（原 requestId receipt 查询）返回 404 时，UI 无恢复路径。修复：404 对账分支呈现可操作恢复态（如"尚未找到该请求的回执——重试对账/返回"），遵循"分享负载缺失或授权失效时展示恢复入口"的任务失败处理约定；补可观察测试。
- **F3 三张关键截图字节级相同不具证明力**：三张关键证据截图字节级一致（自动化时序下未捕获到真实不同状态）。修复：重取证——确保每张截图捕获**可区分**的真实状态（断言截图间差异或捕获时断言页面关键文本/元素存在），替换证据并在报告说明每张截图证明什么。不改生产代码也能完成（若取证手段需要小改测试/自动化脚本，允许改 tests/ 与自动化辅助，不动业务逻辑——如需动业务逻辑先在报告说明并最小化）。

每项先写失败/缺失测试（F3 为取证断言）再修；GREEN 后分项 commit（`fix(miniprogram): <项>`）。

## 3. 验证（全跑附原始输出）

```bash
pnpm --filter @weknora/miniprogram test       # 87 基线 + 新增全绿
pnpm --filter @weknora/miniprogram typecheck  # 13 基线错误外零新增
pnpm --filter @weknora/miniprogram build:weapp
git diff --check
```
**真实 DevTools 复验（本轮端口 57811）**：F2 的 404 恢复态需真实运行时可观察（构造未知对账 404：如先发起后清 receipt/用不存在 requestId 对账——如实声明构造法）；F1 若走可达路线需真实触发截图；F3 重取的三张截图替换入证据。复用你 Wave 4 的环境模式（Lite 服务器 + fixture + `cli auto` + automator，冷编译 settle-retry；先例 `docs/plans/issue-140/task-6-live-validation.md` 复现提纲）。

## 4. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree；local commits authorized, no push/merge/deploy/GitHub action；禁止派发子 agent。
- 不以演示数据冒充结果；失败/待核实给可操作状态；凭据 disposable 结束清理；报告不留 token。
- 测试端口（57811）与构建目录隔离。
- 报告只写事实与实测输出；受限如实 blocked 附证据，绝不伪造。

## 5. 报告与完成

更新 worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t24-miniprogram/task-report.md`：追加修复轮节（F1/F2/F3 各自：依据→测试→修复→复验证据）。最终消息报告三个 commit SHA 与全部验证结果。留在 worktree 等独立 scoped 复审（diff 范围 9ef4ee4c0..新 HEAD）；T24 verified 由主控裁决。修复轮上限 5 轮，本轮 1/5。
