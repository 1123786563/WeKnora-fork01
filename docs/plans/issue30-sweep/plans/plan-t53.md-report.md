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
