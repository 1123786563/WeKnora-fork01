# T24 #54 终审修复报告（final-fix）

分支 `codex/issue30-sweep-t54`（worktree `.worktrees/issue30-sweep-t54`），终审 3 项 minor 发现一次批次全部修复。
每项修复附覆盖测试与本轮实跑输出；无法安全修复的项如实说明（本批无此类项）。

---

## 发现 1（minor）：TestGitLabUnsupportedProviderFailsClosed 派发面 leg 的拒绝门错位

**审查发现**：派发面 leg（snap 未设 ConnectionID）实际由 A02 门拒绝（空 connectionID 在守卫的连接行查询处失败，
`oc_authorizer.go:125` `FindConnectionByID` → `dispatcher.go:59-64` 包装 `ErrDispatchNotStarted: a02`），注释声称的
「未知 target 在平台路由被拒」分支（`dispatcher.go:70` `clientForTarget` → `ProviderOfTarget`/`clientForPlatform`）未被触达。

**修复**（`internal/modules/codedelivery/service_gitlab_test.go`，`TestGitLabUnsupportedProviderFailsClosed`）：
派发面拆为两条 leg，注释与被测行为对齐——

- **leg 1（保留，如实标注）**：无连接快照 → A02 门拒绝，新增 `require.ErrorContains(err, "a02")` 钉住拒绝门身份；
- **leg 2（新增，真正触达平台路由门）**：快照携带可用连接 `ConnectionID: "conn-notion", AuthVersion: 1`
  （夹具种子为 active 个人连接、owner=u1、AuthVersion=1；fixture `Resolve` 放行令牌），A02 与凭据解析均放行后，
  `ProviderOfTarget("notion.deliver")` → `clientForPlatform` default 分支返回 `ErrUnsupportedProvider` → 派发器以
  `ErrDispatchNotStarted` 拒绝。新增 `require.ErrorContains(err, "code_delivery_unsupported_provider")` 钉住
  「拒绝发生在平台路由门而非 A02 门」；`before/after` 调用计数相等 + commits/MR 计数为零钉住「零远端调用、
  外来 target 永不触达任何适配器」。

注：派发器对内层错误用 `%v` 包装（`dispatcher.go:72`），`errors.Is(ErrUnsupportedProvider)` 不成立，
故以错误链子串断言区分拒绝门——与既有 A02/凭据门 `"a02:"/"credential:"` 的消息区分模式一致。

**覆盖测试与输出**：

```
$ go test ./internal/modules/codedelivery/ -run 'TestGitLabUnsupportedProviderFailsClosed' -count=1 -v
=== RUN   TestGitLabUnsupportedProviderFailsClosed
--- PASS: TestGitLabUnsupportedProviderFailsClosed (0.02s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/codedelivery	6.137s
```

`ProviderOfTarget` 解析面的单元钉维持原状（`gitlab_wire_test.go` `TestDraftMRTitleAndProtectedGlob`，:503-507），
与本修复互补：现在端到端面与解析面各有独立钉。

---

## 发现 2（minor）：delivered 新文案与「PR/MR：」回执标签无 app-smoke 断言钉

**审查发现**：移动面仅 pushed 新文案有断言钉（app-smoke.test.tsx:1398）；delivered 新文案
「草稿 PR/MR 已创建」（TaskDetailScreen.tsx:48）与回执标签「PR/MR：」（TaskDetailScreen.tsx:62）仅注释与人工验收覆盖。

**修复**（`apps/mobile/src/app-smoke.test.tsx`，交付回执测试
`the task detail screen renders the code delivery receipt section with honest state copy`）：
按该测试既有范式（嵌套函数组件经 `descendants` 找到 delivery section 元素后手动渲染）追加 delivered 态渲染：
`state: 'delivered'` + `prUrl`/`prNumber`，新增三条断言——

1. `deliveredText.includes('草稿 PR/MR 已创建')` — delivered 新文案；
2. `deliveredText.includes('PR/MR：')` — 平台中性回执标签；
3. `deliveredText.includes('https://gitlab.com/octocat/hello/-/merge_requests/1')` — URL 实际渲染。

**覆盖测试与输出**：

```
$ cd apps/mobile && npx tsx --test src/app-smoke.test.tsx
✔ the task detail screen renders the code delivery receipt section with honest state copy (3.312167ms)
ℹ tests 66  ℹ pass 66  ℹ fail 0
```

---

## 发现 3（minor）：审查包文件 `.superpowers/sdd/t54/final-pkg.md` 缺失

**审查发现**：ask 指定的审查包文件不存在（该 worktree 仅有 `.superpowers/sdd/plan-t54/`）；审查未依赖它，
不影响交付质量判定，仅审查输入缺失。

**修复**：补建 `.superpowers/sdd/t54/final-pkg.md`，内容全部基于本轮实测与在库文件——
审查范围（merge-base `11a067674`，即 plan 入库提交 "docs(issue30-sweep): b5 plans for …"，终点 `09a55a51b`）、
`git diff --stat 11a067674..HEAD` 实测清单（17 files, +2202/-36）逐文件角色表、审查输入指针（计划/progress/task-1）、
验证入口（计划 gate 命令，progress.md:31 逐字）、两条已知非阻塞弱点及本批修复标注。
后续终审可直接以该文件为输入索引。

---

## 回归证据（完整计划 gate，progress.md:31 逐字命令，本轮实跑）

```
$ go build ./ ...
BUILD OK                              # 仅预存在 ld duplicate libraries 警告
$ go test ./internal/modules/codedelivery/ -count=1
ok  	github.com/Tencent/WeKnora/internal/modules/codedelivery	3.454s
$ go test ./internal/handler/ -run 'TestGitLabOAuth' -count=1
ok  	github.com/Tencent/WeKnora/internal/handler	2.201s
$ go test ./internal/handler/session/ -run 'TestDelivery' -count=1
ok  	github.com/Tencent/WeKnora/internal/handler/session	1.642s
$ pnpm --filter @weknora/mobile test
ℹ tests 226  ℹ pass 215  ℹ fail 0  ℹ skipped 11   # skipped 为 env 门控的 opt-in 真机集成（实测输出：missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD，非本批改动）
$ pnpm --filter @weknora/mobile typecheck
TYPECHECK OK
```

## 变更文件

- `internal/modules/codedelivery/service_gitlab_test.go`（发现 1，仅测试）
- `apps/mobile/src/app-smoke.test.tsx`（发现 2，仅测试）
- `.superpowers/sdd/t54/final-pkg.md`（发现 3，新增文档，`git add -f` 入库——该路径被根 `.gitignore` 的 `.*` 规则忽略）
- `.superpowers/sdd/t54/final-fix-report.md`（本报告）

生产代码零改动：三项发现均为测试钉强度与审查输入档案问题，与终审「均不影响交付质量判定」的定性一致。
