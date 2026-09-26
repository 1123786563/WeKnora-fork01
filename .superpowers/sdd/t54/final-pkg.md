# T24 #54 终审审查包（final-pkg）

本文件是计划 `docs/plans/issue30-sweep/plans/plan-t54.md`（GitLab 代码交付通路）的终审审查包清单。
补建于终审修复轮：终审实际未依赖本文件（直接基于 merge-base 全量 diff + 计划 + Issue 完成态审查），
按审查发现 3 如实补档，使审查输入可追溯。

## 审查范围

- 分支：`codex/issue30-t54`（worktree `.worktrees/issue30-sweep-t54`）
- 审查基线：merge-base `11a067674`（"docs(issue30-sweep): b5 plans for 47,49,50,51,53,54,57,61,62,69,70"，即本计划 plan 文件入库提交）
- 审查终点：`09a55a51b`（"docs(parity): T24 #54 task 5 实现报告追加…"）
- 全量 diff：`git diff --stat 11a067674..HEAD` → **17 files changed, 2202 insertions(+), 36 deletions(-)**（本文件撰写时实测）

## Diff 文件清单（按 `git diff --stat 11a067674..HEAD` 实测输出）

| 文件 | 变更 | 角色 |
| --- | --- | --- |
| `internal/modules/codedelivery/code_platform.go` | +123 / 新建 | 平台中立词汇：`CodePlatformClient` 别名族、`ProviderOfTarget`、`clientForPlatform`（唯一平台 switch）、`DraftMRTitle`、`ProtectedBranchGlobMatch` |
| `internal/modules/codedelivery/gitlab_client.go` | +463 / 新建 | GitLab 适配器：project %2F 寻址、Bearer OAuth 头、commits API 单次收敛、`Draft:` 标题、通配保护分支、服务端自定 SHA——全部 GitLab 形状翻译点 |
| `internal/modules/codedelivery/dispatcher.go` | +43/-12 | 派发器接入统一 seam：`clientForTarget` 前置平台路由门（未知 target/未接线适配器零远端调用拒绝）；推送后读回远端权威 head |
| `internal/modules/codedelivery/service.go` | +82/-… | prepare/baseline 面接入 `clientForPlatform` + 安装行权威 provider 解析（`platformProvider`） |
| `internal/modules/codedelivery/gitlab_wire_test.go` | +509 / 新建 | 适配器 wire 测试（httptest 模拟器 `newGitLabEmulator` + `TestGitLabClientAgainstRealGitLab` env 门控真实回归）；`TestDraftMRTitleAndProtectedGlob` 钉 `ProviderOfTarget` 解析面 |
| `internal/modules/codedelivery/gitlab_real_test.go` | +30 / 新建 | 真实 GitLab env 门控冒烟（blocked-env skip，不伪造通过） |
| `internal/modules/codedelivery/service_gitlab_test.go` | +334 / 新建 | 服务级 GitLab 语义组：E2E 回执落账、基线物化、fail-closed（含派发面）、保护分支零远端写、部分完成恢复、unknown 收敛、二次交付收敛、删除型交付、A02 失效拒绝 |
| `internal/modules/codedelivery/service_prepare_test.go` | +24/-… | 夹具扩展：`conn-gl`/`inst-gl` 种子、`gitlabFactory`、dispatcher/service 双接线、`membersDrop` 复用 |
| `internal/container/code_delivery.go` | +11/-… | 生产容器接线：GitLab 工厂（`NewGitLabClientFactory(nil, GitLabAPIBaseURL)`）与 Providers 装配 |
| `internal/handler/app_connector_oauth.go` | +22 | OAuth 注册面：`case "gitlab"` 默认 app 配置（与 `appOAuthDefaults` 同源） |
| `internal/handler/app_connector_oauth_gitlab_test.go` | +67 / 新建 | GitLab OAuth 注册/400 映射 handler 测试 |
| `internal/handler/session/workbench_delivery.go` | +4 | 会话工作台交付面的 GitLab 错误分类接线 |
| `internal/handler/session/workbench_delivery_test.go` | +1 | 对应断言 |
| `apps/mobile/src/screens/TaskDetailScreen.tsx` | +11/-… | 移动面 PR/MR 中性化：`DELIVERY_STATE_COPY` pushed/delivered 新文案、回执标签 `PR/MR：` |
| `apps/mobile/src/app-smoke.test.tsx` | +2/-1 | pushed 新文案断言钉（终审修复轮补 delivered 文案与 `PR/MR：` 标签钉） |
| `.superpowers/issue30-sweep/plans/plan-t54.md-ledger.md` | +12 | 执行台账（5 任务 complete + gate PASS） |
| `.superpowers/issue30-sweep/plans/plan-t54.md-report.md` | +500 | 实现报告（RED→GREEN 取证） |

## 审查输入

- 计划：`docs/plans/issue30-sweep/plans/plan-t54.md`
- 计划执行状态：`.superpowers/sdd/plan-t54/`（`progress.md`、`task-1-brief.md`、`task-1-report.md`、`plan-path`）
- Issue 完成态：plan-t54 五任务全部 complete、gate PASS（见 ledger 与 progress.md:23-31）

## 验证入口（计划 gate，progress.md:31 逐字）

```
go build ./...
go test ./internal/modules/codedelivery/ -count=1
go test ./internal/handler/ -run 'TestGitLabOAuth' -count=1
go test ./internal/handler/session/ -run 'TestDelivery' -count=1
pnpm --filter @weknora/mobile test
pnpm --filter @weknora/mobile typecheck
```

## 已知非阻塞弱点（派发前冲突扫描自报，progress.md:15/:20；终审复核确认）

1. Task 2 弱点：`TestGitLabUnsupportedProviderFailsClosed` 派发面 leg 未设 ConnectionID，
   实际由 A02 门拒绝，注释声称的「未知 target 在平台路由被拒」分支未触达。
   → **终审修复轮已修**：拆两条 leg（A02 门 / 平台路由门），后者以错误链
   `code_delivery_unsupported_provider` 钉住拒绝点（`service_gitlab_test.go`）。
2. Task 5 轻微点：delivered 新文案「草稿 PR/MR 已创建」与回执标签「PR/MR：」无断言钉。
   → **终审修复轮已修**：app-smoke 交付回执测试补 delivered 态渲染与标签断言。
