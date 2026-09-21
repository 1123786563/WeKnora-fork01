# Evidence — commercial（Pass A 任务 A2）

Worker: bm-passa-a2（worktree `.worktrees/bm-passa-a2`，base `703884315`）
Manifest: `docs/architecture/moves/commercial.yaml`

## 1. Pre-move gate

```
$ go run ./tools/modulemove verify --module commercial
modulemove: OK (commercial)
```

## 2. 基线测试（pre-move，旧路径等价集合，-count=1）

```
ok  github.com/Tencent/WeKnora/internal/commercial                              0.040s
ok  github.com/Tencent/WeKnora/internal/payment                                 3.566s
ok  github.com/Tencent/WeKnora/internal/usage                                   0.041s
ok  github.com/Tencent/WeKnora/internal/application/repository/commercial       0.107s
ok  github.com/Tencent/WeKnora/internal/application/service/commercial          0.393s
ok  github.com/Tencent/WeKnora/internal/infrastructure/commercialplatform      70.145s
ok  github.com/Tencent/WeKnora/internal/infrastructure/openmeter                0.087s
```

manifest 首条命令 `go test ./internal/modules/commercial/... -count=1` pre-move 仅有
模块骨架（no test files）。已知 flaky（agentruntime 域）未出现。

## 3. 搬迁（from → to，git mv 纯重命名，commit `638ced419`）

| from | to | 文件数 |
|---|---|---|
| internal/application/repository/commercial | internal/modules/commercial/repository/commercial | 28 |
| internal/application/service/commercial | internal/modules/commercial/service/commercial | 23 |
| internal/commercial | internal/modules/commercial | 42 |
| internal/infrastructure/commercialplatform | internal/modules/commercial/commercialplatform | 11 |
| internal/infrastructure/openmeter | internal/modules/commercial/openmeter | 2 |
| internal/payment | internal/modules/commercial/payment | 8 |
| internal/usage | internal/modules/commercial/usage | 2 |
| **合计** | | **116** |

move commit：`git diff --cached --stat` = `116 files changed, 0 insertions(+), 0 deletions(-)`；
status 中无非 R 条目（全部纯 rename，含 1 个非 Go 资产
`internal/commercial/prototype-lago-billing-state-machine.html`）。

## 4. 修复（commit `2bdfb60e0`）

- import 路径重写：114 个非禁改 .go 文件（`internal/container/container.go` 除外），
  均为 import 行级修改；范围 = manifest `owned_files.importers` 再 grep 全仓确认
  （cmd/saas-migrate、internal/agent、agent/trpc、appconnector、application/repository、
  application/service{,/appconnector,/workbench}、container（非 container.go 两文件）、
  craft、handler、models/chat、router 测试文件、modules/commercial 内部互引）。
- 新增 7 个零逻辑别名包（每包单文件 `alias.go`，头注释
  `// Pass A compatibility alias for internal/modules/commercial/... — zero logic. Deleted by Pass B task B-commercial.`）：

| 旧路径 | 别名面（type = / var =） |
|---|---|
| internal/application/repository/commercial | type BudgetStore；var NewBudgetStore |
| internal/application/service/commercial | type BenefitsService/BillingAccountService/FulfillmentService/OrderService/PlanVersionService；var New{Benefits,BillingAccount,ExecutionGate,Fulfillment,Order,PlanVersion}Service、NewQuotaGuard、NewRecoveryService |
| internal/commercial | type CommercialGateway/CommercialPlatform/ExecutionGate |
| internal/infrastructure/commercialplatform | var NewPlatformFromEnv |
| internal/infrastructure/openmeter | var NewGatewayFromEnv |
| internal/payment | var ProvidersFromEnv |
| internal/usage | type ModelRates；var RatesFromEnv |

别名面来源：禁改文件中唯一引用旧路径的 `internal/container/container.go`
（grep 实证，见 §6）；router.go/task.go/sync_task.go/go.mod/go.sum/migrations 零引用。

- 搬迁机械性测试夹具路径修复（仅测试文件，相对深度 +1）：
  `repository/commercial/{budget_migration_test.go, budget_pg_test.go:60,106,
  account_pg_test.go:131, billing_account_pg_test.go:145}`（migrations 前缀 4→5 级）；
  `commercialplatform/lago_integration_test.go:27`（deploy 前缀 3→4 级）。
- gofmt：全部 touched 文件已格式化；`gofmt -l` 余量均为未触碰文件的 base 既有状态
  （含搬移来的 `repository/commercial/benefits.go`、`subscription_command.go` ——
  base 版本即未格式化，按“不引入无关改动”保留原样）。

## 5. 禁改文件未触碰证明

```
$ git status --short | grep -E 'internal/router/router\.go|internal/router/task\.go|
  internal/router/sync_task\.go|internal/container/container\.go|^ M go\.mod|
  ^ M go\.sum|migrations/'
（空输出）
```

过程记录（透明披露）：修复阶段一次 sed 文件列表排除模式失配，曾误改
`internal/container/container.go` 的 7 条 import 行；当场发现后
`git restore internal/container/container.go` 恢复为 HEAD 版本，
最终工作树与两个提交均不含该文件的任何改动（上方空 grep 为最终证据）。

## 6. 验证（post-move）

- `go build ./...` — 通过（exit 0，仅既存 darwin 链接 duplicate-libraries 警告）。
- `go vet` 于 internal/modules/commercial/...、7 个旧路径别名包、container、router、
  handler — 无发现。
- manifest 测试命令（-count=1，全部 ok）：

```
ok  …/internal/modules/commercial                        0.042s
ok  …/internal/modules/commercial/commercialplatform    70.090s
ok  …/internal/modules/commercial/openmeter              0.053s
ok  …/internal/modules/commercial/payment                3.333s
ok  …/internal/modules/commercial/repository/commercial  0.105s
ok  …/internal/modules/commercial/service/commercial     0.366s
ok  …/internal/modules/commercial/usage                  0.027s
```

- 直接消费方测试（全部 ok）：internal/agent/...（含 opencode 16.9s、recoverytest）、
  internal/appconnector/...、internal/craft、internal/models/chat、
  internal/application/...（repository 145s、service 109s、workbench 等）、
  internal/handler/...、internal/router、internal/container。
- 一次 `TestProvidersFromEnvRejectsPartialAlipay` 全量并发跑中 FAIL、隔离与两次
  复跑均 PASS —— 记录为一次性 env 型偶发（非搬迁回归；夹具未改动该测试逻辑）。

## 7. 重命名证据

```
$ git diff --summary -M 703884315..HEAD | grep -c ' rename '
116
```

非别名新增文件仅 7 个 `create mode … alias.go`。`git diff -M base..HEAD` 中
重命名文件的全部 +/- 行分类：

1. import 路径行（`github.com/Tencent/WeKnora/internal/...` 旧→新）；
2. §4 所列 5 个测试文件的夹具相对深度（`../` 计数）；
3. gofmt 空白对齐（结构体字段/map 字面量/注释对齐，含 base 既有塌缩行
   `knowledge_create.go:1208` 被 gofmt 拆为两行 —— 仅格式）。

**零函数体语义变更**（无逻辑增删改；package 子句未变，包名保持）。

## 8. Review 结果

（占位）独立 Review Agent 结论待附：__未执行__
