# CFT-S03-T020 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `internal/handler/session/craft_preview_policy_test.go`（新增，1 test）：预览续期策略 pin（落点说明：断言是 HTTP 链路组合，放 handler 包复用 previewEnv fixture；任务卡建议 service 路径的逐点套件已存在于 craft_preview_test.go）。
- 修复：`craft_test.go` 的 list 精确断言补 capabilities 字段（T009 增量当时漏跑 handler 全包回归，本轮补上——全包绿）。
- 实现零改动：预览链为 W02 既有（issue/redeem/resolve 三段能力、票据摘要存储、TTL、manifest-only 服务、独立 Origin 校验）。

## 验收断言对照

- 票据过期只取新票 ✓（新增 `PolicyRefreshReauthorizesSameVersion` + 既有 `ExpiredGrantsReturn404`：过期票据与能力均 404；**v2 已发布后对 v1 重取票，新票仍绑定 v1、capability 服务 v1 字节而非最新 v2**——不创建 Run、不重建作品、不偷偷切版本；v2 经自己的票据独立可达）
- 跨租户和撤权访问失败 ✓（既有 `CrossTenantNotFound`：外租户 404 不可见；撤权=票据/能力过期即失效 + 每文件读取重验能力）
- 子资源合法机制下可读但越界失败 ✓（既有 `RejectsTraversal`：dot-dot/绝对路径/编码走私 404；`ValidatePreviewRequestPath`/`ValidateArtifactPathRejectsEnvVariants`（craft 包）钉路径与环境变体拒绝；manifest-only 服务）
- 预览不能读取业务 cookie ✓（独立 https Origin——`RejectsMainOrigin` 拒绝主源误配 + `AllowedHostileOrigins` 白名单；**浏览器级**：e2e 03 spec 沙箱无 allow-same-origin → 父站 cookie 读写 SecurityError、CSP connect-src 'none' 阻断外联，W06 与本轮 T010 e2e 持证）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `go test ./internal/handler/session -run TestCraftPreviewPolicy -count=1` | 0 | 1 PASS |
| `go test -count=1 ./internal/handler/session/` | 0 | ok（16.4s 全包——含 T009 修复后回归） |

## 回退

revert 本提交（policy 测试新增 + craft_test 断言补字段）。
