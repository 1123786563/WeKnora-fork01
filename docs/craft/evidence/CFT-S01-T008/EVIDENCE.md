# CFT-S01-T008 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `packages/core/src/craft/command-bridge.ts`（新增）：`submitDraftWithAttachments(intent, ports)`——命令桥唯一顺序所有者：
  - 全部附件 upload+associate 成功后**恰好一次** run 提交；上传失败提交 0 次且错误可归属到附件名；
  - requestId 由调用方一次铸造、逐字透传（bridge 绝不铸造）；每次尝试从同一冻结意图构建 wire payload（T002 的 validate + craftDraftWireBody），重试 payload deepEqual 一致 → 服务端 request_id 幂等重放；
  - 无 viewVersion 概念——查看历史版本不可能泄入提交；expectedWorkspaceRevision 留意图层（D003）；
  - 校验先于一切副作用（空 prompt/空 session 在 ports 调用前拒绝）。
- `packages/core/src/craft/command-bridge.test.ts`（新增，4 tests）。
- `apps/web/src/features/craft/routes.tsx`：enrichedSend 接入 bridge——`pendingRequestIdRef`（意图级幂等键：首送铸造、失败保留、成功清除）；上传/状态更新移入 ports 闭包（单附件失败标自己而非全部）。
- `packages/core/package.json` exports 增加 `./craft/command-bridge`；根 typecheck:shared 纳入。

## 修复的真实缺口

原 enrichedSend 每次 `request_id: crypto.randomUUID()`——响应丢失后用户重试会换新键，服务端视为新意图重复入场。现在同意图重试复用键（e2e 06 spec 的"刷新恰一次入场"语义从首送扩展到重试路径）。

## 验收断言对照

- 上传失败时 Run 提交次数为 0 ✓（测试 1：两次 upload、零 submit）
- 响应丢失后重试键不变且 Run 计数为 1 ✓（测试 2：两次提交 payload deepEqual + request_id 相同——服务端幂等重放由 Go 侧 admission 测试与 e2e 06 持证；bridge 层保证键与 payload 一致性）
- baseVersion 与查看历史版本不同也不被覆盖 ✓（测试 3：bridge 输入无 viewVersion 概念，base_version_id 逐字透传）
- 调用顺序和完整 payload 快照一致 ✓（测试 2：upload,upload,submit 顺序 + payload 快照断言）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `pnpm exec tsx --test packages/core/src/craft/command-bridge.test.ts` | 0 | 4 pass / 0 fail |
| `pnpm run test:craft:shared` | 0 | **86 pass / 0 fail**（82→86） |
| `pnpm run typecheck:web`（routes 改动） | 0 | 通过 |
| `bash e2e/craft-stack.sh up mock && run mock` | 0 | **6 passed (31.3s)**——真实浏览器走新提交链（01 spec 的 upload→generate 即 enrichedSend 路径） |

## 未验证事项 / 回退

- plain send（无附件轮）仍走 controller.submit 直发（W04 既有路径，语义未变）；后续统一收敛到 bridge 由 T010 工作台整合时评估。
- 重试复用键的浏览器级交互（断网模拟）未单独 e2e（bridge 层已钉 payload 一致性；网络故障注入在 T035 总回归）。
- 回退：revert 本提交（bridge 两文件新增、routes 单函数替换、exports/typecheck 列表增量）。
