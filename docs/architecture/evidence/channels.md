# Evidence — Task A4: Move Channels Packages（Pass A）

分支 `bm-passa-a4`（worktree `.worktrees/bm-passa-a4`），基线 commit `703884315`。

## 1. Pre-move gate

```
$ go run ./tools/modulemove verify --module channels
modulemove: OK (channels)
```

## 2. 测试基线（搬迁前，旧路径等价命令）

`go test ./internal/im/... -count=1`（= manifest `go test ./internal/modules/channels/... -count=1`
的前搬迁路径等价形式）：

```
ok  internal/im           1.105s
ok  internal/im/dingtalk  0.946s
ok  internal/im/feishu    1.103s
ok  internal/im/mattermost 1.104s
ok  internal/im/qqbot     1.084s
ok  internal/im/slack     0.941s
ok  internal/im/telegram  0.922s
?   internal/im/wechat    [no test files]
ok  internal/im/wecom     1.107s
ok  internal/im/yunzhijia 1.017s
```

基线 `go build ./...`：exit 0（仅 cgo "duplicate libraries" 链接告警，属既有噪音）。
已知不稳定用例（agentruntime 域）：本次未触及、未出现。

## 3. 搬迁后验证（新路径）

`go test ./internal/modules/channels/... -count=1`：

```
ok  internal/modules/channels/im           3.791s
ok  internal/modules/channels/im/dingtalk  1.787s
ok  internal/modules/channels/im/feishu    2.298s
ok  internal/modules/channels/im/mattermost 2.480s
ok  internal/modules/channels/im/qqbot     2.289s
ok  internal/modules/channels/im/slack     2.337s
ok  internal/modules/channels/im/telegram  2.243s
?   internal/modules/channels/im/wechat    [no test files]
ok  internal/modules/channels/im/wecom     3.099s
ok  internal/modules/channels/im/yunzhijia 2.324s
```

别名包（旧路径）：10 包编译通过、`[no test files]`（纯转发，无测试）。

直接消费方：`go test ./internal/handler/... ./internal/container/... -count=1`

```
ok  internal/handler         1.188s
ok  internal/handler/dto     1.221s
ok  internal/handler/session 12.036s
ok  internal/container       3.427s
```

构建与静态检查：

```
go build ./...            → exit 0
go vet ./internal/modules/channels/... ./internal/im/... \
       ./internal/handler/... ./internal/container/...  → exit 0
go run ./tools/architectureguard → OK (0 violations)
                                  literal=564 apiKeyRoute=69 handle=0 total=633
                                  redis=23 lite=23 hooks=58 modules=16
                                  （routes 633 与 docs/architecture/backend-baseline.md 一致）
gofmt -l（touched 范围）   → 空（internal/handler 下另有若干**搬迁前即未格式化**的
                              文件：analytics_test.go、commercial.go（A2 域）、
                              mobile_voice*、session/*、usage_test.go——未触碰）
```

## 4. Rename 证据

MOVE commit（1ee86e06b）staged diff：`git diff --cached --summary` = 99 条
`rename internal/{ => modules/channels}/im/... (100%)`，无其他条目；
`git diff --cached --find-renames --stat` = `99 files changed, 0 insertions(+), 0 deletions(-)`。

全分支（703884315..HEAD）：

- `git diff --summary` → 99 条 rename + 10 条 `create mode .../alias.go`（别名包新建）。
- 函数体变更检查：`git diff --find-renames -U0 703884315..HEAD` 中全部 +/- 行
  逐行过滤后，除 import 行与别名文件自身的 `type X = ...` / `var X = ...` 转发声明外
  **为空**——无任何函数体改动。

## 5. Forbidden 文件未触碰证明

```
$ git diff --stat 703884315..HEAD -- internal/router/router.go internal/router/task.go \
    internal/router/sync_task.go internal/container/container.go go.mod go.sum migrations/
（空输出 —— 零改动）
```

`internal/container/container.go` 对旧路径的 10 条 import（:82-91）由
`internal/im`、`internal/im/<sub>` 十个无逻辑别名包满足；别名面 = container.go
实际引用：`imPkg.NewService`、`imPkg.Service`、各子包 `NewFactory`、
`feishu.RegionFeishu`、`feishu.RegionLark`。

## 6. Commits

| SHA | 主题 |
|---|---|
| 1ee86e06b | refactor(channels): move packages to internal/modules/channels（纯改名，树不构建） |
| db79358a2 | refactor(channels): repair imports and add pass-a aliases（import 行 + 别名包 + gofmt） |

文档（integration/channels.md、evidence/channels.md）按 brief 允许折入 repair 提交。

## 7. Review Result

（占位——待独立 review pass 填写。）
