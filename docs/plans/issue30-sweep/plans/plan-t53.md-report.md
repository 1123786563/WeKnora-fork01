# Task 2 实施报告：空间连接 grant 存储 + A02 裁决真实接线（#53，2/5）

- **分支**：`codex/issue30-t53`（worktree `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t53`）
- **提交**：`389c228b9` `feat(appconnector): persist per-actor space-connection grants and ride the real A02 authorizer (#53 task 2)`（7 文件，+301 行）
- **状态**：DONE_WITH_CONCERNS（交付物完整且提交；唯一保留项见「阻塞与保留」）

## 一、实现内容

全部按计划 Task 2 的文件清单交付：

| 文件 | 内容 |
|---|---|
| `migrations/versioned/000198_space_connection_grants.up.sql` | `app_space_connection_grants` 表，PK `(tenant_id, connection_id, actor_id)`（逐字按计划） |
| `migrations/versioned/000198_space_connection_grants.down.sql` | `DROP TABLE IF EXISTS` |
| `migrations/sqlite/000119_space_connection_grants.up.sql` | sqlite 孪生（DATETIME/CURRENT_TIMESTAMP） |
| `migrations/sqlite/000119_space_connection_grants.down.sql` | `DROP TABLE IF EXISTS` |
| `internal/modules/appconnector/repository/appconnector/space_grant.go` | `SpaceConnectionGrantRow`（TableName 绑定）+ `SpaceConnectionGrantStore`：`GrantSpaceConnection`（幂等 upsert）、`RevokeSpaceConnection`（幂等 delete）、`ListSpaceConnectionGrants`、`SpaceConnectionGranted`（结构化满足 `appconnectorsvc.SpaceGrantSource`，`oc_authorizer.go:21-24` 签名逐一核对一致）；全部查询经 gorm 参数绑定 |
| `internal/modules/appconnector/repository/appconnector/space_grant_test.go` | store 测试：全量迁移轨 sqlite 上断言 upsert 幂等/List/查询/跨租户不可达/撤销幂等+即刻收敛 |
| `internal/modules/appconnector/service/appconnector/oc_space_grant_test.go` | authorizer 集成测试 ×3：grant 放行+撤销收敛 / grant 不外溢 personal（AC1）/ 跨租户 grant 不可达 |

前置接口核实：`SpaceGrantSource`（`oc_authorizer.go:21-24`）、`ErrConnectionForbidden`（`credentials.go:56`）、`newResolverFixture` 返回 `(*credentialResolver, *gorm.DB)`（`credentials_test.go:24`）、`dbInstallationSource`（`oc_authorizer_test.go:36`）、`seedOCSubjectFixture`（`oc_authorizer_test.go:116`，tenant 7 + alice/bob + `c-personal`/`c-space`、auth_version 2）——均与本计划记载一致，`OCSubject{TenantID uint64; ActorID string}`（`oc_binding.go:53-56`）亦核实。

## 二、与计划的三处偏差（均有现场证据）

1. **store 测试相对路径深度**：计划写 `../../../..`（4 级），从 `internal/modules/appconnector/repository/appconnector` 到仓库根实际需 **5 级**。首跑实证 root 被解析到 `.../internal/migrations/sqlite`（报错原文 `open .: no such file or directory`）；改为 `../../../../..` 并加注释，与同目录 `oc_store_test.go:22` 的 `../../../../../migrations/sqlite/...` 先例一致。
2. **authorizer 测试 helper 补 `OCBindingRow`**：计划 helper 基于 `newResolverFixture` + `NewOCStore(db)`，但 `newResolverFixture` 的 AutoMigrate 清单无 `connector_connection_bindings` 表，首跑 `TestSpaceGrantNeverOpensPersonalConnection` 实测报 `no such table: connector_connection_bindings`。在 helper 的 AutoMigrate 中加入 `&appconnectorrepo.OCBindingRow{}`——与既有 authorizer 夹具（`oc_authorizer_test.go:95-103`）完全一致。这正是计划「实现依赖说明：执行者现场核实一次」要求的情形。
3. **新迁移文件需 `git add -f`**：`.gitignore:96` 有 `migrations/` 规则，新建迁移文件默认被忽略。仓库先例证明新迁移确实入库（`000117_code_deliveries.*`、`000196_code_deliveries.*` 均被跟踪），故按先例 `git add -f` 四个迁移文件。

## 三、TDD 证据（命令与完整输出）

### RED（Step 3，实现前）

```
$ go test ./internal/modules/appconnector/repository/appconnector/ -run TestSpaceConnectionGrantStore -count=1
# github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector [github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector.test]
internal/modules/appconnector/repository/appconnector/space_grant_test.go:46:11: undefined: NewSpaceConnectionGrantStore
FAIL	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector [build failed]
FAIL
```

与计划 Step 3 预期完全一致（编译失败即本任务 RED 形态）。

### GREEN A：authorizer 集成测试（核心裁决证据，全绿）

```
$ go test ./internal/modules/appconnector/service/appconnector/ -run 'TestSpaceGrant' -count=1 -v
--- PASS: TestSpaceGrantAuthorizerGrantAdmitsAndRevokeConverges (0.01s)
--- PASS: TestSpaceGrantNeverOpensPersonalConnection (0.00s)
--- PASS: TestSpaceGrantAuthorizerCrossTenantGrantUnreachable (0.03s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector	2.640s
```

（输出中有一行 gorm 对预期内 record-not-found 的红色日志，为 `newResolverFixture` 默认 logger 的既有行为，非本测试引入的噪音。）

「无 grant 拒绝/跨租户拒绝/personal 不放行」三支在现状（nil fail-closed）下本就通过、真正的新 GREEN 是「grant admits」与「revoke 收敛」——与计划 Step 7 的 RED 说明一致，未伪造额外 RED。

### GREEN B（阻塞项的补充验证，全绿）：store 全断言 × 全量迁移轨（去重副本）

仓库内计划的 store 测试在本分支被**既有基线问题**挡住（见下节），且我不被授权代做 Task 0。为不留下未验证的「迁移 DDL ↔ gorm 模型」对齐风险，做了零仓库状态变更的补充验证：把 `migrations/sqlite` 复制到 `/tmp/t53-track`、在**副本**上按 t61 分支的完全相同方式去重（`000114_mobile_device_app.*` → `000118_*`，即 Task 0 顺延方案），确认副本无任何 >2 文件的版本号，再用一次性测试文件（已删除，未入库）对副本跑全轨 + 计划 store 测试的全部断言：

```
$ T53_SPACE_MIGRATIONS=/tmp/t53-track go test ./internal/modules/appconnector/repository/appconnector/ -run TestTmpSpaceGrantStoreAgainstDedupedTrackCopy -count=1 -v
=== RUN   TestTmpSpaceGrantStoreAgainstDedupedTrackCopy
--- PASS: TestTmpSpaceGrantStoreAgainstDedupedTrackCopy (1.30s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	3.066s
```

这证明：(a) 000119 DDL 在全轨序列中可装载，且排在顺延后的 000118 之后（与 t61 集成后的真实顺序一致）；(b) gorm 模型列名/类型与迁移 DDL 逐列对齐；(c) upsert 幂等/List/跨租户不可达/撤销幂等+即刻收敛全部成立。一次性文件删除后 `git status` 干净（仅剩三个待提交 Go 文件 + 四个迁移文件）。

### 包级回归与构建（Step 7）

```
$ go test ./internal/modules/appconnector/... -count=1
FAIL	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	1.674s   ← 唯一失败：TestSpaceConnectionGrantStoreUpsertListRevoke，唯一原因：duplicate migration file（见下）
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector	4.887s

$ go test ./internal/modules/appconnector/repository/appconnector/ -count=1 -skip 'TestSpaceConnectionGrantStore'
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	1.328s   ← 包内其余既有测试全部通过

$ go build ./...
BUILD OK（仅既有 ld: warning: ignoring duplicate libraries: '-lc++'，与本改动无关）
```

## 四、阻塞与保留（计划 Step 5 预言的情形，如实报告）

**仓库内 `TestSpaceConnectionGrantStoreUpsertListRevoke` 当前在本分支无法转绿**，失败唯一原因：

```
failed to open source, "file:///...migrations/sqlite": duplicate migration file: 000114_public_agent_marketplace.down.sql
```

事实链：
- 这是**既有基线问题**，先于我的任何改动存在：本 worktree 在我动手前 `go test ./internal/database/` 即 FAIL（同一 duplicate 错误）、`go test ./internal/application/repository/ -run TestTaskGrantStore` 即 FAIL（同一错误）。
- 根因是本计划 **Task 0（mobile_device_app 顺延）未在本分支落地**；等价修复已存在于兄弟分支 `codex/issue30-t61`（提交 `96e579ad0`，`000114->000118 / 000193->000197`），不是本分支祖先。计划 Step 5 原文即预言：「若报 duplicate migration 错误说明 Task 0 未完成，先回 Task 0」。
- 我的授权仅限 Task 2 文件，且不得合并/代做他人任务（集成属主 Agent 职责），故未越权修复。t61 集成落地的瞬间该测试即应转绿（其对齐性已由第三节 GREEN B 的去重副本提前证明）。
- 因此 `go test ./internal/database/` 与 `./internal/application/repository/` 的基线红不因本提交而变化；本提交也未使任何原本绿的包变红（repository 包除新测试外全绿，service 包全绿）。

## 五、自检发现

- 完整性：计划 Task 2 文件清单 7/7 交付；Produces 的四个方法签名与 Task 3 消费约定逐字一致。
- 纪律：仅创建授权文件；未触碰 `container.go`/`router.go`/谓词编码；一次性验证文件已删除；未派发任何子代理；未推送远端。
- YAGNI：store 未加计划外方法；迁移 DDL 与模型逐列对应。
- 测试真实性：所有 PASS/FAIL 均为本 session 真实运行输出；`/tmp` 副本验证如实标注为补充手段，不冒充计划测试的仓库内 GREEN。

## 六、遗留建议（供主 Agent）

1. 集成 `codex/issue30-t61`（含 `96e579ad0` 的 Task 0 等价修复）后，重跑：`go test ./internal/modules/appconnector/... ./internal/database/ ./internal/application/repository/ -count=1`——预期全绿，含 `TestSpaceConnectionGrantStoreUpsertListRevoke`。
2. 后续任务（Task 3）可直接消费 `NewSpaceConnectionGrantStore` 四方法与 `SpaceConnectionGrantRow`，签名未偏离计划。

---

# 修复轮 1/5 报告（审查发现处置）

## 一、审查发现处置结论

唯一发现（important）经审查者自行判定为「非本 diff 缺陷，集成门禁」：仓库内 `TestSpaceConnectionGrantStoreUpsertListRevoke` 红色系 Task 0/t61 范围（重复文件对在 base 提交即存在）。**无代码需修改，本分支代码零变更**；本轮工作为补证据、明状态、留复现路径。

**仓库内 GREEN 状态的明确声明（消除任何误读）**：
- 计划 Step 5（store 测试仓库内 GREEN）：**未达成**，被挂起的 Task 0 阻塞，本分支不声称通过。
- 计划 Step 7（包级回归仓库内 GREEN）：**部分达成**——`service/appconnector` 包全绿（含 3 个新 authorizer 集成测试）；`repository/appconnector` 包除被阻塞的这一个测试外全绿。
- **集成 `codex/issue30-t61`（96e579ad0）后，主控必须重跑**：`go test ./internal/modules/appconnector/... ./internal/database/ ./internal/application/repository/ -count=1`，此前任何集成级 GREEN 声明无效。

## 二、本轮新增证据（全部为本 session 真实运行）

### 1. base 基线红从「推断」升级为「直接运行证据」（待核实项 4 关闭）

用临时 detached worktree 检出 base 提交 `1b89047e2`（不含本任务任何改动）直接运行，跑完即清理：

```
$ git worktree add --detach /tmp/t53-base-check 1b89047e2 && cd /tmp/t53-base-check && go test ./internal/database/ -count=1
Error:      	Received unexpected error:
            	failed to create sqlite migrate instance: failed to open source, "file://migrations/sqlite": duplicate migration file: 000114_public_agent_marketplace.down.sql
Test:       	TestWorkbenchSQLiteDownRefusesPaseo
FAIL
FAIL	github.com/Tencent/WeKnora/internal/database	1.470s

$ （同法）go test ./internal/application/repository/ -run TestTaskGrantStore -count=1
Test:       	TestTaskGrantStoreTaskOwnerIDResolvesSessionOwner
FAIL
FAIL	github.com/Tencent/WeKnora/internal/application/repository	1.401s
```

两个套件在 base 上即红，与本任务 diff 无关。辅助复证：`git ls-tree --name-only 1b89047e2 migrations/sqlite/ | grep 000114` 输出 `000114_mobile_device_app.{up,down}.sql` 与 `000114_public_agent_marketplace.{up,down}.sql` 双对共存（与审查者的 ls-tree 证据一致）。两次临时 worktree 均已 `git worktree remove --force` 清理（`git worktree list` 复核为 0）。

### 2. HEAD 回归重跑（本轮回归覆盖测试，与提交时状态一致）

```
$ go test ./internal/modules/appconnector/... -count=1
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	1.270s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/connectorcontrol	2.333s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/openconnector	0.485s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/publish	1.566s
--- FAIL: TestSpaceConnectionGrantStoreUpsertListRevoke (0.01s)
FAIL	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	1.455s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector	2.973s
```

失败唯一且原因唯一（duplicate migration，失败点在 migrate 源打开、store 代码未执行——与审查复现一致）。

### 3. GREEN B（/tmp 去重副本全轨验证）第二轮独立复跑 PASS

```
$ T53_SPACE_MIGRATIONS=/tmp/t53-track go test ./internal/modules/appconnector/repository/appconnector/ -run TestTmpSpaceGrantStoreAgainstDedupedTrackCopy -count=1 -v
=== RUN   TestTmpSpaceGrantStoreAgainstDedupedTrackCopy
--- PASS: TestTmpSpaceGrantStoreAgainstDedupedTrackCopy (1.08s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	1.648s
```

一次性测试文件跑后即删，`git status` 仅剩报告文件 untracked。

**GREEN B 完整复现命令序列（任何人可复验，包括审查者）**：

```bash
cd <本worktree>
rm -rf /tmp/t53-track && cp -R migrations/sqlite /tmp/t53-track
mv /tmp/t53-track/000114_mobile_device_app.up.sql /tmp/t53-track/000118_mobile_device_app.up.sql
mv /tmp/t53-track/000114_mobile_device_app.down.sql /tmp/t53-track/000118_mobile_device_app.down.sql
ls /tmp/t53-track | sed 's/_.*//' | sort | uniq -c | awk '$1 > 2'   # 应无输出
# 将首轮报告第三节的一次性测试文件内容存为
# internal/modules/appconnector/repository/appconnector/space_grant_tmpverify_test.go
# （openSpaceGrantDBTmp 从 T53_SPACE_MIGRATIONS 读迁移根，其余断言与 space_grant_test.go 逐条相同）
T53_SPACE_MIGRATIONS=/tmp/t53-track go test ./internal/modules/appconnector/repository/appconnector/ -run TestTmpSpaceGrantStoreAgainstDedupedTrackCopy -count=1 -v
rm internal/modules/appconnector/repository/appconnector/space_grant_tmpverify_test.go
```

## 三、待核实项逐项回应

| # | 待核实项 | 本轮处置 |
|---|---|---|
| 1 | TDD RED 时序仅报告为证（单提交无法从 diff 复验） | **维持如实声明**：RED 运行（`undefined: NewSpaceConnectionGrantStore` 编译失败）发生于实现文件写入之前、提交之前，属本 session 真实时序；单提交粒度下 git 无法复验先后，审查者已确认错误文本与缺失符号的预期编译形态一致。不再有可补的客观证据。 |
| 2 | GREEN B 仅报告为证 | 本轮**第二次独立运行 PASS**（见上），并补齐完整可复现命令序列。仍需说明：它是当前唯一的全轨 DDL↔模型对齐证据，仓库内对齐验证须待 Task 0 集成后由主控重跑关闭。 |
| 3 | 集成 96e579ad0 后是否转绿 | 主控集成职责，我无权合并。重跑命令清单已在「仓库内 GREEN 状态声明」中列明。 |
| 4 | base 套件已红未直接运行验证 | **已关闭**：本轮在 base 检出上直接运行两个套件均 FAIL（见本轮证据 1）。 |

## 四、本轮变更清单

- 代码：**零变更**（审查发现无需代码修改）。
- 报告：本文件追加修复轮章节；随本报告一并提交（docs commit）。
- 临时产物：两次 /tmp detached worktree、一次性测试文件——均已清理/删除，工作区复核干净。

---

# Task 3 实施报告：空间连接 grant 管理端点 + router/container 生产接线（#53，任务 3/5）

- **分支**：`codex/issue30-t53`（worktree `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t53`）
- **提交**：
  - `e6dd975b6` `feat(appconnector): manage space-connection grants over HTTP and wire the real grant store into the A02 guard (#53 task 3)`（5 文件，+320/−5）
  - `f5dd64d66` `fix(container): provide the space-grant store to dig — the #53 task-3 guard closure takes it, startup would fail with missing type otherwise`（1 文件，+6；计划缺口的必要补线，见偏差 4）
- **状态**：DONE_WITH_CONCERNS（交付物完整且提交；仓库内三包既有失败集合保持不变，集成 Task 0 后重跑收口，见第四/五/六节）

## 一、实现内容

全部按计划 Task 3 文件清单交付（5/5）：

| 文件 | 内容 |
|---|---|
| `internal/handler/app_connection_grants.go`（新建） | `AppConnectionGrantHandler`：`grantScope`（appTenantScope + `appconnector.CanManageConnections`，非管理 403 `SPACE_GRANT_FORBIDDEN` 且先于任何读行）、`loadSpaceConnection`（tenant 绑定查询；跨租户/缺失统一 404 `CONNECTION_NOT_FOUND`；personal 连接 400 `SPACE_GRANT_NOT_APPLICABLE`——AC1）、`requireActiveMember`（`TenantMemberRepository.Get` miss/非 active → 400 `GRANTEE_NOT_ACTIVE_MEMBER`）；`Grant`/`Revoke`/`List` 三方法，全部 SQL 经 gorm 参数绑定 |
| `internal/handler/app_connection_grants_test.go`（新建） | 2 个 handler 测试：生命周期（owner 授予 201→贡献者管理 403→list→撤销幂等→撤销后即时收敛）与拒绝面（personal 拒授/跨租户 404/非 active 成员拒授） |
| `internal/router/routes_app_connectors.go` | 文件末尾追加 `RegisterAppConnectionGrantRoutes`（nil-guard 跳过；POST/GET `/apps/connections/:id/grants` + DELETE `/:grantee_id`） |
| `internal/router/router.go` | `RouterParams` 在 `AppActionHandler` 后加 `AppConnectionGrantHandler` 字段（带两行注释，见偏差 2）；`RegisterAppConnectorRoutes(v1, ...)` 调用块后加一行 `RegisterAppConnectionGrantRoutes(v1, params.AppConnectionGrantHandler)` |
| `internal/container/container.go` | `:956` 后加 `must(container.Provide(handler.NewAppConnectionGrantHandler))`；`:979-986` 过时注释末句按计划替换；guard 闭包加 `grants *repoappconn.SpaceConnectionGrantStore` 形参、`nil` → 真实 store；**另补计划遗漏的** `must(container.Provide(repoappconn.NewSpaceConnectionGrantStore))`（偏差 4，提交 `f5dd64d66`） |

前置接口逐项现场核实（均与计划一致）：`appTenantScope`（`app_connector.go:45-54`）、`appFail`（`:32`）、`CanManageConnections`（`access.go:33`，owner/admin）、`TenantRole` 四值（`types/tenant_member.go:19-32`）、`TenantMemberRepository.Get` miss 返回 `(nil, nil)`（`interfaces/tenant_member.go:23`）、`repository.NewTenantMemberRepository`（`tenant_member.go:35`）、`InstallationRow`/`ConnectionRow` 字段（`install.go:37-62`）、Task 2 的 `SpaceConnectionGrantStore` 四方法签名（`space_grant.go:38-72`）、上下文键类型断言（`context_helpers.go:26/66/149`：uint64/string/TenantRole）。

## 二、与计划的四处偏差（均有现场证据）

1. **计划测试夹具笔误**：计划第 671-672 行的 `ConnectionRow` 字面量把 `TenantID: 7` 写了两次，Go 结构体字面量不允许重复字段名——首跑 RED 即暴露（`duplicate field name TenantID in struct literal`）。删除每行中重复的一个（值相同，语义零变化）。
2. **router.go 新增字段带注释**：计划说"加一行"字段；实际加字段 + 两行注释（对齐该结构体既有字段的注释风格与 nil-guard 惯例）。纯注释差异。
3. **计划 Step 6 的前提在当前基线不成立**：计划称"若注册冲突会在 `go test ./internal/router/` 挂载时即时 panic 暴露"——实测 router 包全部 9 个测试在 fixture 的 `m.Up()`（`routes_agent_marketplace_test.go:361`）即因 duplicate migration 失败，**未执行到路由挂载**，gin 静态段 `oauth/callback` 与参数段 `:id` 共存未被任何现存测试检验。用一次性测试（task 2 GREEN B 先例，跑完即删）直接检验：按生产注册顺序挂载，4 条路由全部在场、无 panic——**PASS**（证据见第三节；文件内容存档于附录 A）。
4. **计划 Step 5(c) 漏掉 dig provider（真实生产缺口，已修复）**：闭包新形参 `grants *repoappconn.SpaceConnectionGrantStore` 需要 dig 里有对应 provider，而 Task 2 提交未触碰 container.go、计划的 container 编辑清单也只有两处。若按计划字面执行，`A02Guard`（被 `code_delivery.go:108`、`open_connector.go:608,644`、`notion_publish.go:31` 生产消费）在容器启动时解析失败。用一次性 dig 测试实证（附录 B）：无 provider 时 `missing type: *appconnector.SpaceConnectionGrantStore`；补 `must(container.Provide(repoappconn.NewSpaceConnectionGrantStore))` 后解析成功。修复即提交 `f5dd64d66`。当前没有任何绿色测试覆盖 BuildContainer（container 包唯一测试被迁移问题挡住），此缺口靠现场推演+实证捕获。

## 三、TDD 证据（命令与完整输出）

### RED（Step 2，实现前）

首跑（计划逐字夹具，暴露偏差 1）：

```
$ go test ./internal/handler/ -run TestAppConnectionGrant -count=1
# github.com/Tencent/WeKnora/internal/handler [github.com/Tencent/WeKnora/internal/handler.test]
internal/handler/app_connection_grants_test.go:38:253: duplicate field name TenantID in struct literal
internal/handler/app_connection_grants_test.go:39: duplicate field name TenantID in struct literal
internal/handler/app_connection_grants_test.go:52:7: undefined: NewAppConnectionGrantHandler
FAIL	github.com/Tencent/WeKnora/internal/handler [build failed]
FAIL
```

修正夹具后纯 RED，与计划 Step 2 预期逐字一致：

```
$ go test ./internal/handler/ -run TestAppConnectionGrant -count=1
# github.com/Tencent/WeKnora/internal/handler [github.com/Tencent/WeKnora/internal/handler.test]
internal/handler/app_connection_grants_test.go:52:7: undefined: NewAppConnectionGrantHandler
FAIL	github.com/Tencent/WeKnora/internal/handler [build failed]
FAIL
```

### GREEN（Step 4，实现后）

```
$ go test ./internal/handler/ -run TestAppConnectionGrant -count=1 -v
=== RUN   TestAppConnectionGrantLifecycleOwnerAdmitsMemberRefuses
--- PASS: TestAppConnectionGrantLifecycleOwnerAdmitsMemberRefuses (0.00s)
=== RUN   TestAppConnectionGrantRefusesPersonalConnectionAndForeignTargets
--- PASS: TestAppConnectionGrantRefusesPersonalConnectionAndForeignTargets (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/handler	2.498s
```

### 补充证据 1：gin 路由挂载共存（一次性测试，PASS 后即删）

```
$ go test ./internal/router/ -run TestTmpGrantRoutesMountOnProductionShape -count=1 -v
=== RUN   TestTmpGrantRoutesMountOnProductionShape
--- PASS: TestTmpGrantRoutesMountOnProductionShape (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/router	2.278s
```

（gin v1.12.0，`go.mod:23`。静态 `oauth/callback` 与参数 `:id/grants` 同树共存 + 4 条路由在场断言。文件内容存档附录 A。）

### 补充证据 2：dig 缺口实证（一次性测试，两阶段，PASS 后即删）

```
$ go test ./internal/container/ -run TestTmpA02GuardDigResolution -count=1 -v
=== RUN   TestTmpA02GuardDigResolution
    a02_guard_dig_tmpverify_test.go:46: phase1 (no provider): could not build arguments for function ... failed to build appconnector.A02Guard: ... missing type: *appconnector.SpaceConnectionGrantStore
--- PASS: TestTmpA02GuardDigResolution (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/container	4.676s
```

phase1 证明计划字面执行会启动失败；补 Provide 行后 phase2 解析成功。文件内容存档附录 B。

### 构建与全链验证（Step 6）

```
$ go build ./... ; echo exit=$?
exit=0        ← 唯一输出为既有 ld: warning: ignoring duplicate libraries: '-lc++'，与本改动无关

$ go test ./internal/router/ ./internal/container/ ./internal/handler/ -count=1   # 改动后
--- FAIL ×14（router 9 / container 1 / handler 4，名单与基线逐一相同，见第四节）

$ gofmt -l（5 个触碰文件）
无输出
```

## 四、基线红直接证据（本 session 改动前直接运行）

**改动前三包运行**（我在写任何代码之前执行）：

```
$ go test ./internal/router/ ./internal/container/ ./internal/handler/ -count=1
--- FAIL: TestPublicMarketplaceAdoptRejectsTamperedPublicRelease        ← router（共 9 个 marketplace 系）
--- FAIL: TestPublicMarketplaceCatalogAndAdoptAuthorization
--- FAIL: TestPublicMarketplaceCrossTenantEndToEndAndPrivacy
--- FAIL: TestPublicMarketplacePublisherVerificationAndSubmissionAuthorization
--- FAIL: TestPublicMarketplaceReviewAuthorization
--- FAIL: TestTenantAgentAdoptionPublishesIndependentVariantsIntoMobileAvailableAgents
--- FAIL: TestTenantAgentAdoptionRoutesAndAuthorization
--- FAIL: TestTenantAgentMarketplaceLifecycleAndAuthorization
--- FAIL: TestTenantAgentVariantCapabilityMappingAndTestGate
--- FAIL: TestWireCraftInteractionRegistrarRegistersPendingInteractions  ← container（1 个）
--- FAIL: TestAppPublicationsTableExistsAfterMigrations                  ← handler（共 4 个）
--- FAIL: TestNotionPublishEndToEndCreateApprovePublishReceipt
--- FAIL: TestNotionPublishEndToEndUnknownReconcilesRemoteFirst
--- FAIL: TestNotionPublishEndToEndUpdateConflict
FAIL（三包）
```

**根因样例（三包各一例，原文）**——全部同一既有根因 `duplicate migration file: 000114_public_agent_marketplace.down.sql`（即挂起的 Task 0）：

```
handler:  internal/handler/app_connector_notion_publish_e2e_test.go:77:
          failed to open source, "file:///…migrations/sqlite": duplicate migration file: 000114_public_agent_marketplace.down.sql
router:   internal/router/routes_public_marketplace_test.go:204（经 routes_agent_marketplace_test.go:361 的 m.Up()）：
          failed to open source, "file:///…migrations/sqlite": duplicate migration file: 000114_public_agent_marketplace.down.sql
container: TestWireCraftInteractionRegistrarRegistersPendingInteractions:
          failed to open source, "file:/…migrations/sqlite": duplicate migration file: 000114_public_agent_marketplace.down.sql
```

**经典基线红（`internal/database`）本 session 直接运行**：

```
$ go test ./internal/database/ -count=1
ERROR[] migration.go:134[RunMigrationsWithOptions] | Failed to create sqlite migrate instance: failed to open source, "file://migrations/sqlite": duplicate migration file: 000114_public_agent_marketplace.down.sql
--- FAIL: TestSQLiteMigrationsIncludeAutoTagConfig
--- FAIL: TestSQLiteMigrationsCreateVersionedSchema
FAIL	github.com/Tencent/WeKnora/internal/database
```

**结论**：三包 14 个失败 + `internal/database` 红全部是 base 提交既有的 Task 0 范围问题，先于我的任何改动存在；我的改动前后失败名单逐一相同（第三节改动后运行 ×14 vs 本节基线 ×14），零新增失败。

## 五、仓库内 GREEN 状态声明（消除任何误读）

- 计划 Step 2（RED）：**达成**——`undefined: NewAppConnectionGrantHandler` 编译失败，实现前运行。
- 计划 Step 4（handler 新测试 GREEN）：**达成**——2/2 PASS，仓库内。
- 计划 Step 6（`go build ./...` + 三包全绿）：**部分达成**——build exit 0；三包测试因 14 个**既有**失败（唯一根因：挂起的 Task 0）不能全绿，**本分支不声称 Step 6 全绿**。可声明的是更强的保守事实：改动前后失败名单逐一相同，新测试全绿，零新增失败，零回归。
- 关联回归：`go test ./internal/modules/appconnector/... -count=1` 改动后运行——除 Task 2 已报告的被阻塞项 `TestSpaceConnectionGrantStoreUpsertListRevoke`（同一 duplicate migration 根因）外全部 `ok`，含 3 个 authorizer 集成测试所在的 service 包。
- **集成 `codex/issue30-t61`（96e579ad0 的 Task 0 等价修复）后，主控必须按第六节序列重跑**；此前任何集成级 GREEN 声明无效。

## 六、GREEN B 复现命令序列（任何人可复验，供集成后验证消费）

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t53
# 前提：Task 0 等价修复（codex/issue30-t61 的 96e579ad0：000114→000118 / 000193→000197）已集成进本分支

# 1) 计划 Step 6 命令原样重跑——预期三包全绿（14 个既有失败全部转绿）
go build ./... && go test ./internal/router/ ./internal/container/ ./internal/handler/ -count=1

# 2) 本任务新测试——预期 2/2 PASS
go test ./internal/handler/ -run TestAppConnectionGrant -count=1 -v

# 3) Task 2 遗留阻塞项一并收口——预期全绿（含 TestSpaceConnectionGrantStoreUpsertListRevoke）
go test ./internal/modules/appconnector/... ./internal/database/ ./internal/application/repository/ -count=1
```

说明：第三节两份一次性证据（路由挂载、dig 解析）不依赖迁移轨、已在仓库内 PASS，集成后无需重跑；其文件内容存档于附录，如需独立复验可按附录重建临时文件后运行。

## 七、自检发现

- 完整性：计划 Task 3 文件清单 5/5 交付；Task 2 的 Produces（`NewSpaceConnectionGrantStore` 四方法 + `SpaceConnectionGrantRow`）被 handler/container 按签名逐字消费。
- 纪律：仅改动 5 个授权文件；`access.go` 谓词编码零触碰；router/container 均为单点插入；未派发任何子代理；未推送远端；两份一次性测试文件删除后 `git status` 干净（仅报告文件待提交）。
- 参数绑定：handler 全部 SQL 经 gorm `Where("tenant_id = ? AND id = ?", ...)` 参数化，无字符串拼接。
- 安全面：身份只取自 `appTenantScope`（认证上下文），响应体只含 connection_id/grantee_id/granted_by/created_at，无凭据字段；非管理者在读取任何行之前被 403。
- API-key 词表：新路由不进 authorizer 词表，与 `/apps/*` 家族一致；`assertAPIKeyPoliciesMatchRoutes`（`rbac.go:410-435`）只检查"已声明 policy → 已注册路由"方向，新增路由不影响其断言（现场读码核实）。
- 测试真实性：所有 PASS/FAIL 均为本 session 真实运行输出；一次性验证如实标注为补充手段并给出复现路径，不冒充计划测试。

## 附录 A：一次性路由挂载测试内容（`internal/router/routes_grant_mount_tmpverify_test.go`，跑后即删）

```go
package router

import (
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

func TestTmpGrantRoutesMountOnProductionShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	// 生产注册顺序（routes_app_connectors.go）：先静态 oauth 回调腿，
	// 再同一 /apps/connections 子树下的 grant 路由（"oauth" 静态段与
	// ":id" 参数段为兄弟节点）。
	v1.GET("/apps/connections/oauth/callback", func(c *gin.Context) { c.Status(http.StatusOK) })
	RegisterAppConnectionGrantRoutes(v1, handler.NewAppConnectionGrantHandler(nil))

	want := map[string]bool{
		"POST /api/v1/apps/connections/:id/grants":               false,
		"GET /api/v1/apps/connections/:id/grants":                false,
		"DELETE /api/v1/apps/connections/:id/grants/:grantee_id": false,
		"GET /api/v1/apps/connections/oauth/callback":            false,
	}
	for _, rt := range r.Routes() {
		if _, ok := want[rt.Method+" "+rt.Path]; ok {
			want[rt.Method+" "+rt.Path] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Fatalf("route missing after mount: %s", k)
		}
	}
}
```

## 附录 B：一次性 dig 缺口验证测试内容（`internal/container/a02_guard_dig_tmpverify_test.go`，跑后即删）

```go
package container

import (
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"go.uber.org/dig"
	"gorm.io/gorm"
)

func tmpGuardContainer(t *testing.T, provideStore bool) error {
	t.Helper()
	c := dig.New()
	must(c.Provide(func() *gorm.DB { return &gorm.DB{} }))
	must(c.Provide(repository.NewMCPOAuthBindingStore, dig.As(new(appconnectorsvc.ConnectionCredentialSource))))
	must(c.Provide(repoappconn.NewOCStore))
	must(c.Provide(repoappconn.NewInstallationStore))
	if provideStore {
		must(c.Provide(repoappconn.NewSpaceConnectionGrantStore))
	}
	// #53 task-3 闭包，与 container.go 逐字一致。
	must(c.Provide(func(src appconnectorsvc.ConnectionCredentialSource,
		installs *repoappconn.InstallationStore, grants *repoappconn.SpaceConnectionGrantStore,
		oc *repoappconn.OCStore,
	) appconnectorsvc.A02Guard {
		return appconnectorsvc.NewOCSubjectGuard(src, appconnectorsvc.NewInstallationStateSource(installs), grants, oc)
	}))
	return c.Invoke(func(g appconnectorsvc.A02Guard) { _ = g })
}

func TestTmpA02GuardDigResolution(t *testing.T) {
	err := tmpGuardContainer(t, false)
	if err == nil || !strings.Contains(err.Error(), "missing type") {
		t.Fatalf("phase1: expected missing-type failure without the store provider, got %v", err)
	}
	t.Logf("phase1 (no provider): %v", err)
	if err := tmpGuardContainer(t, true); err != nil {
		t.Fatalf("phase2: with the store provider the guard must resolve, got %v", err)
	}
}
```

---

# 修复轮 1/5 报告（Task 3 审查发现处置）

## 一、两项发现处置结论

| # | 发现 | 处置 |
|---|---|---|
| 1 | 集成门禁：挂起的 Task 0 令 Step 6 无法全绿（审查员实测 HEAD 三包 14 个既有失败，与本报告基线逐一相同；Task 2 遗留项同因仍红） | **维持非本 diff 缺陷判定，零代码变更**。审查员已独立复跑证实；本轮我补上了缺失的 base 直接运行证据（base `febb8d2cc` 临时 worktree 复跑，名单 14/14 逐一相同，见本轮证据 1）。集成 `codex/issue30-t61`（96e579ad0）后主控必须按第六节序列重跑收口——该声明不变。 |
| 2 | 计划 Step 5(c) 缺口修复正确：`f5dd64d66` 的 Provide 行是生产启动必要条件（审查员静态复核 dig paramSingle 语义确认） | **无需进一步修改**。审查员结论与我的实证（附录 B 两阶段）一致。 |

**本轮代码变更：零**（报告文件除外）。本轮工作为补证据、关待核实项、维持状态声明。

## 二、本轮新增证据（全部为本 session 真实运行）

### 1. 待核实项 1 已关闭：base（`febb8d2cc`）三包失败名单直接运行比对（逐一相同）

沿用 Task 2 修复轮先例：临时 detached worktree 检出 base 提交（不含本任务任何改动），跑完即清理：

```
$ git worktree add --detach /tmp/t53t3-base-check febb8d2cc
HEAD is now at febb8d2cc docs(issue30-sweep): plan-t53 task2 实施报告（…）

$ cd /tmp/t53t3-base-check && go test ./internal/router/ ./internal/container/ ./internal/handler/ -count=1
--- FAIL ×14（router 9 / container 1 / handler 4）
```

HEAD 同命令复跑后剔除耗时后缀严格 diff：

```
$ diff <(sed -E 's/ \([0-9.]+s\)$//' base 名单) <(sed -E 's/ \([0-9.]+s\)$//' HEAD 名单)
NAMES-IDENTICAL(14/14)   ← diff 无输出
```

base 上根因抽验（与 HEAD 同因）：

```
failed to open source, "file:///tmp/t53t3-base-check/migrations/sqlite": duplicate migration file: 000114_public_agent_marketplace.down.sql
```

清理复核：`git worktree remove --force /tmp/t53t3-base-check` 后 `git worktree list` 中该条目为 0。

### 2. 待核实项 3 部分关闭：附录 A/B 第二轮独立复跑（重建→跑→删）

按报告附录 A/B 原文重建两份一次性测试文件并重跑（与首轮完全独立）：

```
$ go test ./internal/router/ -run TestTmpGrantRoutesMountOnProductionShape -count=1 -v
--- PASS: TestTmpGrantRoutesMountOnProductionShape (0.00s)
ok  	github.com/Tencent/WeKnora/internal/router	4.287s

$ go test ./internal/container/ -run TestTmpA02GuardDigResolution -count=1 -v
a02_guard_dig_tmpverify_test.go:46: phase1 (no provider): … missing type: *appconnector.SpaceConnectionGrantStore
--- PASS: TestTmpA02GuardDigResolution (0.00s)
ok  	github.com/Tencent/WeKnora/internal/container	3.749s
```

两文件重跑后即删，`git status` 复核干净（仅报告文件待提交）。至此附录 A 有三个独立来源（我首轮、我本轮、审查员同版 gin 复刻 11 路由）、附录 B 有两个独立来源（我两轮）+ 审查员静态复核。

### 3. HEAD 回归重跑（与提交时状态一致）

```
$ go test ./internal/handler/ -run TestAppConnectionGrant -count=1
ok  	github.com/Tencent/WeKnora/internal/handler	1.882s

$ go test ./internal/modules/appconnector/... -count=1
ok（appconnector / connectorcontrol / openconnector / publish / service/appconnector 五包）
--- FAIL: TestSpaceConnectionGrantStoreUpsertListRevoke   ← 唯一失败，Task 2 遗留阻塞项，同因（duplicate migration）
FAIL	repository/appconnector

$ go build ./... ; echo exit=$?
exit=0
```

## 三、待核实项逐项回应

| # | 待核实项 | 本轮处置 |
|---|---|---|
| 1 | base 三包失败名单与改动后逐一相同（未检出 base 复跑） | **已关闭**：base `febb8d2cc` 临时 detached worktree 直接运行，14/14 名单逐一相同、根因同因（本轮证据 1）。 |
| 2 | TDD RED 先于实现的时序（单提交不可复验） | **维持如实声明**：RED 运行（`undefined: NewAppConnectionGrantHandler` 编译失败）发生于实现文件写入之前、提交之前，属本 session 真实时序；单提交粒度下 git 无法复验先后。审查员已确认错误文本与预期编译形态一致。不再有可补的客观证据。 |
| 3 | 附录 A/B 一次性测试无法原样重放 | **部分关闭**：本轮按附录原文第二次独立运行，双双 PASS（本轮证据 2）；文件可随时按附录重建重放（复现命令：将附录内容存为对应路径 → `go test ./internal/router/ -run TestTmpGrantRoutesMountOnProductionShape -count=1 -v` 与 `go test ./internal/container/ -run TestTmpA02GuardDigResolution -count=1 -v` → 删除文件）。 |
| 4 | Task 2 GREEN B 与集成后全绿预期 | **主控职责，维持原状**：须待集成 96e579ad0 后按第六节序列重跑方可关闭；我无权合并。 |

## 四、本轮变更清单

- 代码：**零变更**（两项发现均无需代码修改：发现 1 属 Task 0 范围、发现 2 已由 f5dd64d66 修复并获审查确认）。
- 报告：本文件追加修复轮章节，随本报告一并提交（docs commit）。
- 临时产物：base 临时 worktree（`/tmp/t53t3-base-check`）与两份一次性测试文件——均已清理/删除，工作区复核干净。

---

# Task 4 实施报告：端到端证据——真实全链 httptest（AC1/AC2/AC3 + 归因三元组）（#53，任务 4/5）

- **分支**：`codex/issue30-t53`（worktree `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t53`）
- **提交**：`376b64280` `test(delivery): end-to-end collaboration evidence over the real HTTP face — AC1/AC2/AC3 + traceability triple (#53 task 4)`（1 文件，+412）
- **状态**：DONE_WITH_CONCERNS（交付物完整且提交；全链断言级 GREEN 由去重副本一次性验证证明；仓库内 GREEN 与 Task 2/3 同因被挂起的 Task 0 阻塞，集成后主控重跑收口，见第四/五节）

## 一、实现内容

计划 Task 4 文件清单 1/1 交付：`internal/application/repository/delivery_collaboration_http_test.go`（整个新文件，package `repository_test`，412 行）——3 个端到端测试：

| 测试 | 证据 |
|---|---|
| `TestDeliveryCollaborationEndToEndAC1PersonalLoopAndAttribution` | owner 经 personal 连接的完整 baseline→edit→prepare→approve(真实 A03 HTTP)→dispatch→读回链全 200/201；同一 owner 用 space 连接交付被 403 `code_delivery_forbidden`（AC1：互不替代）；归因三元组 `"initiator":"u1"` / `"approver":"u1"` / `"remote_login":"octocat-remote"` + `pr_url` 同一读回断言；`require.NotContains(body, deliveryToken)`（归因面永不携带凭据） |
| `TestDeliveryCollaborationEndToEndAC2CollaboratorCannotInheritPersonalConnection` | u3 成为协作者后：读交付 200（#42 granted-read 面），写面（baseline/prepare 经 owner 的 personal 连接）裸 404，无 grant 的 u4 读 404 `run_not_found`（AC2） |
| `TestDeliveryCollaborationEndToEndCrossTenantIsolated` | tenant 2 主体探测 tenant 1 的 run：读面统一 404 `run_not_found` |

被测系统全真实：gin 路由 + 真实 `WorkbenchDeliveryHandler`/`WorkbenchTaskGrantsHandler`/`AppActionHandler`（approve 走真实 HTTP 端点，digest/fence 从持久行读取）+ 真实 `CodeDeliveryService`/`ActionService`/`TaskGrantService` + 真实 gorm store + `openTaskGrantDB` 真实全量迁移 sqlite。唯一替身是 GitHub（进程外 provider，blocked-env）：httptest Server 按 `github_client.go` 现场 wire 契约应答（git-data 链 blobs→trees→commits→refs→draft PR + GET /user），`gitHubRestClient` 的全部真实 HTTP 代码在跑。mock 未触碰任何被测授权/归因组件（AC3 替身论证与计划一致）。

**前置接口逐项现场核实**（均与计划一致）：`openTaskGrantDB`（`task_grant_store_test.go:27`，seed tenant 1/2 + u1–u5 + session s1 owner u1）、`taskGrantAdmission`（`task_grant_read_test.go:16`，run r1）、`NewWorkbenchDeliveryHandler(runs, granted, service)`（`workbench_delivery.go:34`）、`NewWorkbenchTaskGrantsHandler`（`workbench_task_grants.go:29`）、`NewTaskGrantService(grants, sessions, members)`（`task_grant.go:41`）、`CodeDeliveryDeps` 九字段（`service.go:48-61`）、`NewActionService(store, guard, gate, dispatcher, unknown)` gate 可 nil（`action.go:192`）、`DispatcherDeps` 八字段（`dispatcher.go:17-26`）、`NewGitHubClientFactory(httpClient, baseURL)`（`github_client.go:18`）、`NewLocalWorkspaceSource(root)`（`workspace_local.go:16`）、`NewSubjectGuard(src)` 单参（`oc_authorizer.go:82`）、`ConnectionCredentialSource` 4 方法 + `CredentialResolver.Resolve`（`credentials.go:61-66/21-23`——`deliveryCredentialSource` 实现 5 方法）、`NewAppActionHandler`/`SetActionService`/`ApproveAction`（`app_connector_action.go:35/43/211`）、六个 store 构造器、`appActionApproveInput{digest, expected_version}`（`:202-205`）、fence 列（action store `gorm:"column:fence"`）、workspace 布局（`WriteSessionWorkspaceFiles` 经 `resolve` 落盘 `wsRoot/octocat/hello/main.go`，与计划的编辑路径一致）、`authorize` personal-only（`service.go:435-447`）、`writeDeliveryError` 错误码表（`workbench_delivery.go:208-246`）。

## 二、与计划的两处偏差（均有现场证据）

1. **计划夹具笔误（同 Task 3 报告偏差 1 同款）**：计划第 1267-1268 行的 `ConnectionRow` 字面量把 `TenantID: 1` 写了两次——Go 结构体字面量不允许重复字段名（`duplicate field name TenantID in struct literal`）。删除每行中重复的一个（值相同，语义零变化）。
2. **`deliveryBaseSHA` 字面量长度差 1（计划「40 hex chars」注释与字面量不符）**：计划给定的 `b00000000000000000000000000000000000000` 实测 **39 字符**（`printf '%s' ... | wc -c` = 39），不满足 `baselineSHALegal`（`service.go:514-516`，要求 `len==40`）。首跑一次性验证即暴露：baseline 返回 400，service 层真实错误 `code_delivery_invalid_baseline_sha: "b000..."`（一次性诊断测试 `TestTmpDiagBaselineFailure` 直接调 `svc.MaterializeBaseline` 取得，跑后即删）。修正为 40 字符（`b` + 39 个 `0`），保持计划「40 hex chars」的意图。此为测试夹具数据修正，未改动任何被测代码。

## 三、TDD 证据（命令与完整输出）

**TDD 形态如实声明**：本任务交付物是纯 e2e 测试文件（被测实现已由 Task 1–3 完成并提交），无生产代码变更，无传统 RED→GREEN。TDD 对本任务的落点是「测试先行验证」：先写文件、立即运行、以现场证据修正夹具后转绿。

### 1. 仓库内首跑（实现后、现状分支）——被挂起的 Task 0 阻塞

```
$ go vet ./internal/application/repository/ ; echo exit=$?
exit=0                          ← 测试代码与现场 API 编译对齐

$ go test ./internal/application/repository/ -run 'TestDeliveryCollaboration' -count=1
--- FAIL: TestDeliveryCollaborationEndToEndAC1PersonalLoopAndAttribution (0.02s)
    ... task_grant_store_test.go:38 ... failed to open source, "file:///…migrations/sqlite":
    duplicate migration file: 000114_public_agent_marketplace.down.sql
--- FAIL: TestDeliveryCollaborationEndToEndAC2CollaboratorCannotInheritPersonalConnection (0.01s)
    （同因）
--- FAIL: TestDeliveryCollaborationEndToEndCrossTenantIsolated (0.01s)
    （同因）
FAIL	github.com/Tencent/WeKnora/internal/application/repository	5.400s
```

失败点在 `openTaskGrantDB` 的迁移源打开（`task_grant_store_test.go:38`），先于任何被测组件执行——与计划排查表预言一致（「`openTaskGrantDB` 报迁移错误 → Task 0 未完成」），与 Task 2 的 `TestSpaceConnectionGrantStoreUpsertListRevoke`、Task 3 的三包 14 失败同因。提交后收尾复跑结论相同。

### 2. 一次性全链验证（GREEN 等价物，断言级，去重副本，PASS 后即删）

沿用 Task 2/3 审查已接受的 GREEN B 先例（我无权代做 Task 0）：把 `migrations/sqlite` 复制到 `/tmp` 按 Task 0 方案去重，一次性测试文件 `delivery_collab_tmpverify_test.go` **直接复用正式文件的全部 helper 与方法**（同包；`githubStub`/`deliveryCredentialSource`/`deliveryCollabEnv` 及其 `do`/`prepareDelivery`/`approveThroughHTTP` 方法零复刻，仅 DB 打开器换成从 `T53_DELIVERY_MIGRATIONS` 读副本根、测试函数名加 `Tmp` 前缀、3 个测试体与正式逐字相同），对副本跑全链：

```
$ rm -rf /tmp/t53-delivery-track && cp -R migrations/sqlite /tmp/t53-delivery-track
$ mv /tmp/t53-delivery-track/000114_mobile_device_app.up.sql /tmp/t53-delivery-track/000118_mobile_device_app.up.sql
$ mv /tmp/t53-delivery-track/000114_mobile_device_app.down.sql /tmp/t53-delivery-track/000118_mobile_device_app.down.sql
$ ls /tmp/t53-delivery-track | sed 's/_.*//' | sort | uniq -c | awk '$1 > 2'   # 无输出（无重复版本号）

$ T53_DELIVERY_MIGRATIONS=/tmp/t53-delivery-track go test ./internal/application/repository/ -run 'TestTmpDeliveryCollab' -count=1 -v
=== RUN   TestTmpDeliveryCollabAC1PersonalLoopAndAttribution
--- PASS: TestTmpDeliveryCollabAC1PersonalLoopAndAttribution (9.30s)
=== RUN   TestTmpDeliveryCollabAC2CollaboratorCannotInheritPersonalConnection
--- PASS: TestTmpDeliveryCollabAC2CollaboratorCannotInheritPersonalConnection (14.73s)
=== RUN   TestTmpDeliveryCollabCrossTenantIsolated
--- PASS: TestTmpDeliveryCollabCrossTenantIsolated (12.48s)
PASS
ok  	github.com/Tencent/WeKnora/internal/application/repository	42.792s
```

这证明正式测试文件的**全部断言在真实全链上成立**（含 SHA 修正后的完整链路）；仓库内不能转绿的唯一原因是挂起的 Task 0。两份一次性文件（tmpverify + diag）已删除，`git status` 仅剩待提交文件（提交后复核干净）。

### 3. AC3 完整性复核（Step 3，对照 issue-53.md 验收标准）

1. 「个人与空间连接不能互相替代」→ 本任务 `…AC1PersonalLoopAndAttribution`（同一 owner：personal 全链 200、space 403 `code_delivery_forbidden`，拒绝发生在任何 GitHub 调用之前）+ Task 2 `TestSpaceGrantNeverOpensPersonalConnection`（服务级：grant 不外溢 personal）。
2. 「Collaborator 不能继承 Owner 个人连接」→ 本任务 `…AC2CollaboratorCannotInheritPersonalConnection`（协作者读 200 / 写 404 / 无 grant 读 404）。
3. 「端到端行为通过最高稳定 Interface 验证」→ 全链 HTTP + 真实迁移 sqlite + 真实 handler/service/store；GitHub 替身论证见测试文件头注释与计划替身论证节；归因三元组在同一读回中断言且无凭据泄漏。

### 4. 计划级验证命令（Step 4）与 base 严格比对——零新增失败

```
$ go build ./... ; echo exit=$?
exit=0                          ← 仅既有 ld: warning: ignoring duplicate libraries: '-lc++'
```

六组包全量测试（HEAD `376b64280` 与 base `5c8c89d00` 各一轮，后者用临时 detached worktree `/tmp/t53t4-base-check`，跑后已清理，`-count=1`，名单剔除耗时后缀严格 diff）：

```
HEAD: 328 失败 —— database 10 / appconnector repo 1 / handler 4 / application/repository 286 / workbench 27
base: 325 失败 —— database 10 / appconnector repo 1 / handler 4 / application/repository 286 / workbench 27
diff 唯一非耗时差异（HEAD 侧多出）：
> --- FAIL: TestDeliveryCollaborationEndToEndAC1PersonalLoopAndAttribution
> --- FAIL: TestDeliveryCollaborationEndToEndAC2CollaboratorCannotInheritPersonalConnection
> --- FAIL: TestDeliveryCollaborationEndToEndCrossTenantIsolated
既有失败逐一相同（325/325），零新增既有失败、零回归。
```

全绿的包：`internal/modules/codedelivery` ok；`internal/modules/appconnector` 五包 ok（除 Task 2 遗留的 `TestSpaceConnectionGrantStoreUpsertListRevoke` 外）。

**基线红全量规模的新记录（供主控集成后收口预期）**：Task 2/3 报告只记录了 router/container/handler 三包 14 个失败与 store 测试 1 个；本轮 base 直接运行证实挂起的 Task 0 实际波及全部走全量迁移轨的测试，本轮六组包内共 **325 个既有失败**（`internal/database` 10、`internal/application/repository` 286、`internal/modules/workbench/service/workbench` 27、handler 4、repository/appconnector 1；`internal/router`/`internal/container` 不在本轮六包命令内，其 10 个沿用 Task 3 记录）。全部同因 `duplicate migration file: 000114_public_agent_marketplace.*`。

## 四、仓库内 GREEN 状态声明（消除任何误读）

- 计划 Step 1（写 e2e 文件）：**达成**（1/1 文件，含两处夹具修正，见第二节）。
- 计划 Step 2（端到端测试 PASS）：**仓库内未达成，被挂起的 Task 0 阻塞**；断言级 GREEN 由第三节 2 的一次性去重副本验证证明（3/3 PASS，零 helper 复刻漂移）。
- 计划 Step 4（build + 六组包全绿）：**部分达成**——build exit 0；`codedelivery`/`appconnector` 族全绿；其余失败全部为 base 既有（325/325 逐一相同），零新增失败。
- **集成 `codex/issue30-t61`（96e579ad0）后，主控必须按第五节序列重跑**；此前任何集成级 GREEN 声明无效。

## 五、GREEN 复现命令序列（任何人可复验，供集成后验证消费）

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t53
# 前提：Task 0 等价修复（96e579ad0：000114→000118 / 000193→000197）已集成进本分支

# 1) 本任务新测试——预期 3/3 PASS（openTaskGrantDB 全轨可装载后）
go test ./internal/application/repository/ -run 'TestDeliveryCollaboration' -count=1 -v

# 2) Task 2 遗留阻塞项一并收口——预期全绿
go test ./internal/modules/appconnector/repository/appconnector/ -run TestSpaceConnectionGrantStore -count=1

# 3) 计划级全量——预期六组包全绿（本轮记录的 325 个既有失败全部转绿）
go build ./... && go test ./internal/database/ ./internal/modules/codedelivery/ ./internal/modules/appconnector/... ./internal/handler/ ./internal/application/repository/ ./internal/modules/workbench/service/workbench/ -count=1
```

## 六、自检发现

- 完整性：计划 Task 4 文件清单 1/1 交付；3 个测试 + GitHub stub + 凭据适配器 + 手动路由挂载与计划逐字一致（除第二节两处夹具修正）。
- 纪律：仅创建授权文件（`delivery_collaboration_http_test.go`）；未触碰任何被测代码、fixture、router/container；一次性文件跑后即删；未派发任何子代理；未推送远端；临时 base worktree 已 `git worktree remove --force` 清理（`git worktree list` 复核该条目为 0）。
- 替身面：GitHub 是唯一替身（进程外 + 凭据门控），其余全真实；stub 按 `github_client.go` 现场 wire 契约应答，认证头钉 `Bearer` token；`gitHubRestClient` 真实 HTTP 代码全程在跑。
- 参数绑定：凭据适配器读生产 `connections` 表经 `Where("tenant_id = ? AND id = ?", ...)` 参数化。
- 测试真实性：所有 PASS/FAIL 均为本 session 真实运行输出；一次性验证如实标注为补充手段并给出复现路径，不冒充计划测试的仓库内 GREEN。
- **本任务的 TDD RED 形态如实声明**：无生产代码变更，无传统 RED；首跑暴露的是基线阻塞（duplicate migration）与计划夹具的 SHA 长度笔误（经一次性诊断实证后修正），而非断言先红后绿的实现循环。

## 七、待核实项（留给审查/主控）

1. **一次性验证仅报告为证**（同 Task 2 GREEN B 性质）：本轮一次性 e2e 验证未做第二独立复跑（本轮时间花在 base 全量比对上）。复现方式：按第三节 2 的命令重建副本与一次性文件（结构：复用正式文件全部 helper + env-aware DB 打开器 `T53_DELIVERY_MIGRATIONS`）。如审查要求，我可在修复轮把一次性文件全文存档为附录（同 Task 3 附录 A/B 惯例）。
2. base 全量比对本身已直接运行（325/325 逐一相同），非推断。
