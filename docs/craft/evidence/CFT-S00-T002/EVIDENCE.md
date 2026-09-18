# CFT-S00-T002 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `packages/contracts/src/craft/command-ports.ts`（新增）：冻结命令合同——`CraftSubmitDraftCommand` / `CraftRestoreCommand` / `CraftDecisionCommand` 类型、三个校验器、三个 wire body 序列化器、`CRAFT_DECISION_ACTIONS` 词汇、客户端权威字段黑名单。
- `packages/contracts/test/craft-commands.test.ts`（新增，7 tests）：缺 requestId 拒绝、非法 revision 拒绝、同意图 inputs/knowledge/baseVersion 逐字段保留、客户端 sandbox/tenant/user 字段拒绝、action 词汇冻结、restore 映射。
- `packages/contracts/src/index.ts`：导出上述合同。
- 根 `package.json`：`test:craft:shared` 增加 `packages/contracts/test/craft-*.test.ts`；`typecheck:shared` 增加 `command-ports.ts`。

## 冻结映射（设计词汇 → 已核对 wire DTO）

| 设计（command-bridge.types.ts） | wire（已核对 Go DTO） | 备注 |
|---|---|---|
| DraftIntent.text | prompt | craftRunRequestDTO |
| DraftIntent.inputs[].ref | input_refs[] | 同上 |
| DraftIntent.knowledge[] | knowledge_scope | 服务器编码 scope 串 |
| DraftIntent.baseVersionId | base_version_id | 同上 |
| DraftIntent.expectedWorkspaceRevision | **不在 wire** | D003：意图保留字段（校验正整数），T016 决定是否升为后端 CAS；当前由 request_id 重放 + live-run guard 保护 |
| RestoreIntent.versionId | snapshot_id | craftRestoreRequestDTO（revision 即 CAS，已支持） |
| decidePermission allow_once/deny | action | decideRequestBody（expected_revision CAS 已支持） |
| answerQuestion | answers[]{question_id,choices,text} | 同上 |
| RunReceipt.replayed | —— | 由服务端 202/幂等语义承载，本轮不新增字段 |

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `pnpm exec tsx --test packages/contracts/test/craft-commands.test.ts` | 0 | 7 pass / 0 fail |
| `pnpm run test:craft:shared`（含新 glob） | 0 | **68 pass / 0 fail**（61 旧 + 7 新，无下降） |
| `pnpm run typecheck:shared`（含 command-ports.ts） | 0 | tsc --noEmit 通过 |

## 验收断言对照

- 缺少 requestId 或非法 revision 被拒绝 ✓（测试 2、3）
- 同意图的输入/知识/baseVersion 完整保留 ✓（测试 1：draft 校验后逐字段 + wire body deepEqual，键集精确锁定无泄漏）
- 拒绝客户端提供的 sandbox URL 与身份授权事实 ✓（测试 5：sandbox_url/tenant_id/user_id 等黑名单字段在三个命令上均抛错）

## 未验证事项 / 回退

- wire body 序列化器尚未被 api-client 实际消费（T008 统一附件与提交命令桥时接入）。
- 回退：revert 本提交（contracts 新文件为纯增量，index.ts 导出与 package.json glob 一并回退）。
