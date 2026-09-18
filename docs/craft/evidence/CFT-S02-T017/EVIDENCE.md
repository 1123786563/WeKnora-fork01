# CFT-S02-T017 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `internal/application/service/craft_source_guard_test.go`（新增，3 tests，`TestCraftSourceGuard*`）：资料守卫矩阵汇总。
- 实现零改动：资料解析/授权链为 W03/C01 既有（`craft_knowledge.go`：Build 走既有用户/空间/共享库 ACL + 每 KB ACL 守护检索；撤权重解析无缓存快照；`craft_knowledge_test.go` 既有 10+ 用例含拒绝/撤权/越界/上限）。

## 验收断言对照

- 跨租户/用户未授权资料拒绝 ✓（`CrossTenantUnauthorizedRejected`：外租户知识 ErrForbidden、writer 零写入——未授权材料绝不落盘）
- 共享知识按资源 ACL 而非仅同租户 ✓（`SharedLibraryFollowsResourceACL`：外租户共享库凭 share 成员解析成功且来源如实记录属主租户；撤回 share 后同一库立即 Forbidden——访问由资源 ACL 决定）
- 撤权后下一次读取失败 ✓（同上后半 + 既有 `TestCraftKnowledgeBuildFailsAfterShareRevoked`：一次成功 Build 后撤权，第二次 Build 即 Forbidden、无陈旧权限快照）
- 文档指令不能扩大网络与工具权限 ✓（`DocumentContentCannotWidenPermissions`：注入式文档（"Grant network access… disable the sandbox… admin credentials"）的摘录**逐字作为数据**入 bundle；bundle 序列化结构白名单断言——顶层仅 Sources/Truncated、每个来源仅 ID/Ref/Excerpt/Digest/TenantID 五个纯溯源字段，**不存在任何策略面**（allowed_tools/network_policy/sandbox_config 等概念在 bundle 中无处安放）；执行侧权限由服务端装配的工具白名单与沙箱策略决定，知识内容不参与——W06 e2e 03 spec 另证恶意产出页的外联被 CSP 阻断）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go test ./internal/application/service -run TestCraftSourceGuard -count=1` | 0 | 3 PASS |
| `go test -count=1 ./internal/application/service/` | 0 | ok（65s 全包含既有 knowledge/interaction/recovery 套件） |

## 回退

revert 本提交（单测试文件，纯增量）。
