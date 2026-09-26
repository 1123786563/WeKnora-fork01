# plan-t62 Task 5 实现报告：AC1/AC2/AC3 端到端证据（真实 HTTP）

**状态：DONE**
**提交：`6f2fe47ad` test(marketplace): end-to-end fork lineage, license gate and public lane evidence (#62 AC1/AC2/AC3)**
**分支：`codex/issue30-t62`（worktree `.worktrees/issue30-sweep-t62`）**
**任务序：Issue #62 实施计划第 5/6 个任务（Task 5，本计划最后一个实现任务）**

> 注：本文件路径为编排层为每个任务指定的报告输出路径，此前内容为 Task 3 实现员的报告（提交 `942efb48c`，已由编排层消费）；本报告按任务授权覆盖写入。

---

## 一、实现内容

按 plan-t62.md Task 5 的 4 个步骤完成。本任务是证据收集任务：唯一交付物为 `internal/router/routes_agent_fork_lineage_test.go`（新文件，308 行），实现代码（Task 0-4）已由前置任务完成并提交（`34849775b`/`e0e164d58`/`37f0cdac6`/`942efb48c`/`0e2e54a8f`），本任务零生产代码改动。

### 1.1 前置接口核对（Consumes，全部实测在位）

开工前逐一核对了任务描述与计划声称的前置接口，全部确认：

| 前置接口 | 证据 |
|---|---|
| HTTP wire：`POST/GET /api/v1/marketplace/licenses`（Admin+） | `internal/router/routes_agent_marketplace.go:37`（`licenses := g.apiKeyGroup(...)` + `licenses.POST/GET("", g.Admin(), ...)`） |
| submission/release DTO 5 个 lineage 字段（string 均 omitempty） | `internal/handler/agent_marketplace.go:69-73`（submission）、`:98-102`（release）、`:124`、`:150`（DTO 映射） |
| 租户 lane 409/400 错误映射 | `internal/handler/agent_marketplace.go:133`（`ErrReleaseRedistributionForbidden`→Conflict）、`:281`（`ErrAgentLicenseInvalid`→ValidationError） |
| 公共 lane 409 错误映射 | `internal/handler/public_marketplace.go:191-194` |
| `ReviewAndPublishTx` 逐列传播 lineage 5 列 | `internal/application/repository/agent_marketplace.go:154`（release 字面量含 `IsFork/ForkSourceListingID/ForkSourceReleaseID/ForkNotes/LineageLicenseID`） |
| `SubmitPublicRelease` 公共 lane live 许可证门 | `internal/application/service/public_marketplace.go:124-136`（digest 校验后、`CreatePublicSubmission` 前查询，fail closed） |
| 测试基建 `newPublicMarketplaceTestApp` / `publicCall` / `publishAdoptionRelease` | `internal/router/routes_public_marketplace_test.go:29,83`、`internal/router/routes_agent_adoption_test.go:93`（且确认该 app 挂载了全部所需路由：AgentVersion/AgentMarketplace/AgentAdoption/PublicMarketplace/CustomAgent，`PUT /agents/:id` 在 `routes_agent.go:43`） |

### 1.2 端到端测试文件（AC3 最高稳定 Interface 证据）

`internal/router/routes_agent_fork_lineage_test.go`：真实 sqlite 迁移流（`openTenantAgentMarketplaceHTTPTestDB`，含 000119）+ 真实 `CustomAgentService/AgentVersionService/AgentMarketplaceService/AgentAdoptionService/PublicMarketplaceService` + 真实 freeze/submit/review/adopt/variant/publish/license 端点链路，唯一测试替身是测试自带的 RBAC 上下文注入中间件。三个测试：

1. **`TestAgentForkLineageMappingEditsAreNotForks`（AC1）**：原始内容提交（license 未注册仍 201，无 lineage 字段、manifest 无 `"lineage"` 段）→ 纯映射派生（两次重映射知识库绑定后提交发布冻结版本）→ `is_fork=false` 且 lineage 完整（`fork_source_listing_id/fork_source_release_id/lineage_license_id=MIT/fork_notes=ChangeNotes`、manifest 含 `"is_fork":false`）→ 伪造 `fork_source_release_id` 的请求体 strict decode 400 → 对照组：改 system prompt 后提交 `is_fork=true`、manifest 含 `"is_fork":true`。
2. **`TestAgentForkLineageRedistributionForbiddenRejectsSubmission`（AC2 租户 lane）**：已注册但禁止再分发 → 派生提交 409 且消息点名许可证（含 `tenant-private` 与 `redistribution`）；未注册 → 409 fail closed（含 `not registered`）；翻转注册表为允许 → 同形态派生链 live 放行 201 且如实记为 Fork。源审批 semver 逐次递增（1.0.0/1.0.1/1.0.2）规避 `uq_agent_releases_semantic` 唯一索引。
3. **`TestAgentForkLineagePublicSubmissionLicenseGate`（AC2 公共 lane）**：两条 fork 链在放行期完成租户 lane 提交与审批 → F1 升公共提交 201 → 翻转 MIT 禁止 → F2 升公共提交 409（live 查询，历史放行不缓存）→ 恢复允许后同一 F2 重试 201。

### 1.3 对计划草稿的两处修正（计划假设缺口，实现代码零改动）

计划 Task 5 Step 1 声称测试代码是「完整最终形态」，但其中两处对既有行为的假设不成立，实测失败后修正（均为测试侧修正，服务端行为是正确的既有行为，无生产代码改动）：

**修正 1：公共提升必须用 fork submission 自身的 listing_id，而非源 listing。**
- 现象：`TestAgentForkLineagePublicSubmissionLicenseGate` 中 F1 升公共提交（`source_listing_id=源listing, release_id=forkOneReleaseID`）返回 400 `invalid public marketplace request`（`routes_agent_fork_lineage_test.go:279` 断言处）。
- 定位过程：临时调试测试直接调用 `PublicMarketplaceService.SubmitPublicRelease` 拿到完整错误 `release does not belong to the source listing`（调试文件已删除，未入库）；`GetRelease` 找不到行时返回 `(nil,nil)`，service 把 nil 与归属不符统一报该错（`public_marketplace.go:112-114`）。
- 根因：`CreateSubmission` 按 `(tenant_id, source_agent_id)` 复用/新建 listing（`internal/application/repository/agent_marketplace.go` 的 listing 查找逻辑）；派生提交的 `SourceAgentID` 是 variant 发布的本地 agent（新 uuid），因此 fork Release 落在**派生 agent 自己的新 listing**，不属于源 listing。这恰符合领域语义（CONTEXT.md:134「Fork……作为独立 Agent Definition 维护」）；`SubmitPublicRelease` 的所有权检查拒绝跨 listing 提升是正确防护。
- 修正：三条公共提升调用改用 `forkOne.Data.ListingID`/`forkTwo.Data.ListingID`（`forkLineageBody` 本就携带该字段），并加断言 `require.NotEqual(t, listingID, forkOne.Data.ListingID)` 钉死该归属语义。许可证门语义不受影响——门读的是 release 行的 `lineage_license_id`。

**修正 2：submission metadata 必须携带 `capability_requirements`。**
- 现象：`TestAgentForkLineageRedistributionForbiddenRejectsSubmission` 第一次 `publishSourceThenFork` 内 `seedForkChain` 的 capability-mapping 步骤返回 400 `invalid agent adoption request`（`seedForkChain` 内 mapped 断言处）。
- 根因：`UpdateCapabilityMapping` 要求映射的 capability 必须在源 release manifest 的 `CapabilityRequirements` 内（`internal/application/service/agent_adoption.go:182` `capability %q is not required by release %s`）；Test 2 的源 release 经 `submitForkVersionRaw` 提交，metadata 缺该字段 → 空 requirement 表 → 任何映射都被拒。既有 `publishAdoptionRelease`（Test 1/3 的源链）之所以 PASS，正因其 metadata 带 `"capability_requirements": ["model","knowledge"]`（`routes_agent_adoption_test.go:112`）。
- 修正：`submitForkVersionRaw` 的 metadata 统一补 `"capability_requirements": []string{"model","knowledge"}`。fork 提交多带该字段无害（fork submission 不经 adoption 校验）。

**TDD 说明**：本任务的 RED 阶段对应实现已在前置任务 0-4 完成（任务描述明示「此前任务已产出」），本任务交付物即测试本身——测试首次运行失败暴露的正是上述两处计划假设缺口，修正后转绿；未伪造任何通过。

---

## 二、测试命令与完整输出

### 2.1 Step 2：端到端测试（计划指定命令）

```
$ go test ./internal/router/ -run 'TestAgentForkLineage' -count=1 -v
=== RUN   TestAgentForkLineageMappingEditsAreNotForks
--- PASS: TestAgentForkLineageMappingEditsAreNotForks (17.51s)
=== RUN   TestAgentForkLineageRedistributionForbiddenRejectsSubmission
--- PASS: TestAgentForkLineageRedistributionForbiddenRejectsSubmission (11.42s)
=== RUN   TestAgentForkLineagePublicSubmissionLicenseGate
--- PASS: TestAgentForkLineagePublicSubmissionLicenseGate (30.02s)
PASS
ok  	github.com/Tencent/WeKnora/internal/router	68.403s
```

（修正后终态；中间失败过程见 1.3。）

### 2.2 Step 3：既有 marketplace/adoption/public 全量回归（计划指定命令）

```
$ go test ./internal/router/ -run 'TestTenantAgent|TestPublicMarketplace|TestAvailableAgents' -count=1
ok  	github.com/Tencent/WeKnora/internal/router	241.941s
```

既有 #58/#59/#60 行为零回归（Review Focus 4 的接口级证据：原始内容 digest 与既有 lifecycle 不漂移）。

### 2.3 计划级验证命令全链（计划「计划级验证命令」一节，收尾复跑）

```
$ go build ./... \
  && go test ./internal/application/service/ -run 'TestAgentForkLineageMigration|TestSubmitReleaseLineageVerdict|TestSubmitReleaseRedistributionGate|TestRegisterLicense' -count=1 \
  && go test ./internal/application/repository/ -run 'TestFindDerivation|TestLicenseUpsertGetAndList' -count=1 \
  && go test ./internal/modules/agentruntime/agent/experts/ -count=1 \
  && go test ./internal/router/ -run 'TestAgentForkLineage|TestTenantAgentAdoption|TestTenantAgentMarketplace|TestTenantAgentVariant|TestPublicMarketplace|TestAvailableAgents' -count=1
# github.com/Tencent/WeKnora/cmd/desktop
ld: warning: ignoring duplicate libraries: '-lc++'
# github.com/Tencent/WeKnora/cmd/server
ld: warning: ignoring duplicate libraries: '-lc++'
ok  	github.com/Tencent/WeKnora/internal/application/service	169.326s
ok  	github.com/Tencent/WeKnora/internal/application/repository	31.786s
ok  	github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/experts	6.389s
ok  	github.com/Tencent/WeKnora/internal/router	350.536s
```

退出码 0（`go build ./...` 通过，ld 警告为既有环境噪音非错误）。全部 PASS。

---

## 三、提交

- `6f2fe47ad` test(marketplace): end-to-end fork lineage, license gate and public lane evidence (#62 AC1/AC2/AC3)
  - 1 file changed, 308 insertions(+)：`internal/router/routes_agent_fork_lineage_test.go`（新）

仅改动任务授权文件；未触碰 `internal/container/`、TS 全部、以及任何他人文件；未推送远端。

---

## 四、自检发现与遗留

1. **两处计划假设缺口已修正并在提交信息中记录**（见 1.3）。均为计划草稿对既有行为的错误假设，非实现缺陷；服务端行为（fork 自有 listing、adoption capability 校验）经确认为正确既有行为，符合 spec §9 与 CONTEXT.md:134。
2. **对计划文档的同步**：plan-t62.md 本身的 Task 5 草稿代码仍含这两处旧假设。按「只修改本任务授权文件」约束未改计划文档；如需计划与实现对齐，建议编排层决定是否由文档维护者回填（修正点：Test 3 的三条公共提升 `source_listing_id` 应为 fork submission 响应的 `listing_id`；`submitForkVersionRaw` metadata 需含 `capability_requirements`）。
3. **运行时长**：router 包全量模式约 6 分钟（350s），属既有规模（Step 3 既有回归亦 242s），无新增 flakiness 迹象（本轮各命令均一次通过）。
4. **未运行项**：无——计划 Task 5 全部步骤（含计划级验证命令）均已在本会话实际执行。

---

# 修复轮 1 报告（2026-09-26）

**状态：DONE**
**提交：`2032e0c42` test(marketplace): make forged-lineage 400 attribution sound with legal metadata and honest-body control (#62 修复轮 1)**

## 审查项：forge 断言无法归因（important，plan-mandated）

### 核实（审查发现成立）

- 现状代码（修复前）`internal/router/routes_agent_fork_lineage_test.go:194-200`：forge 请求体为 `{"agent_version_id", "metadata": {}, "fork_source_release_id": "fake"}`，断言只看 400。
- 空 metadata 的独立 400 点核实：`internal/modules/agentruntime/agent/experts/agent_release.go:181`（`at least one supported language is required`）、`:187`（`the license id is required`）、`:190`（semantic version 校验）——空 metadata 在 bundle 构造阶段必然 400。
- 若 handler 无 `DisallowUnknownFields`（`internal/handler/agent_marketplace.go:155-156`，实测确认存在），decode 会放行该 body，随后 bundle 校验仍 400——原断言在两种实现下都通过，无法归因，Review Focus 3 的 HTTP 层钉死实为恒真断言。审查描述与代码事实完全一致。
- 生产行为本身正确：`submitAgentReleaseBody`（`internal/handler/agent_marketplace.go:45-48`）字段集确无任何 lineage 字段。

### 修复（测试侧，生产代码零改动）

按审查给出的修法改写 forge 段：

1. **forge body 改为完整合法 metadata**（semantic_version/display_name/summary/supported_languages/use_cases/capability_requirements/minimum_weknora_capability/license_id=MIT——experts 校验不会独立拒绝它），断言 400 且响应消息点名 `json: unknown field \"fork_source_release_id\"`（`invalidMarketplaceBody` 透传 decode 错误，`agent_marketplace.go:169-172`；归因断言）。
2. **同 body 去掉伪造字段 → 201 对照**（`agent_release_submissions` 无 (agent_version_id)/semver 唯一索引——`migrations/sqlite/000109_tenant_agent_marketplace.up.sql:38-43` 的唯一索引全部在 listings/reviews/releases 表，Test 1 不审批，二次提交安全），证明除伪造字段外请求完全合法，400 不可能由其他校验产生。

三重归因闭环：metadata 合法性（honest 201 证明）→ 400 只能来自 decode → 消息点名被拒字段即伪造的 lineage 字段。

### TDD 过程（真实 RED→GREEN）

- **RED**：首次运行新断言失败——`Messages: {"error":{"code":1000,...,"message":"invalid marketplace request: json: unknown field \"fork_source_release_id\""}} does not contain "unknown field \"fork_source_release_id\""`。生产行为正确（400 且消息点名 unknown field），失败原因是断言未算上响应 JSON 对消息内引号的转义（body 中是 `\"` 两字节）。这一失败同时反证了新断言真的在校验 decode 错误内容，不是恒真断言。
- **GREEN**：修正 Contains 参数为转义形态 `"json: unknown field \\\"fork_source_release_id\\\""` 后通过。

### 测试命令与完整输出

```
$ go test ./internal/router/ -run 'TestAgentForkLineage' -count=1 -v
=== RUN   TestAgentForkLineageMappingEditsAreNotForks
--- PASS: TestAgentForkLineageMappingEditsAreNotForks (5.36s)
=== RUN   TestAgentForkLineageRedistributionForbiddenRejectsSubmission
--- PASS: TestAgentForkLineageRedistributionForbiddenRejectsSubmission (5.55s)
=== RUN   TestAgentForkLineagePublicSubmissionLicenseGate
--- PASS: TestAgentForkLineagePublicSubmissionLicenseGate (2.96s)
PASS
ok  	github.com/Tencent/WeKnora/internal/router	19.802s
```

回归覆盖（计划级验证命令全链，与首轮相同的收尾验证）：

```
$ go build ./... \
  && go test ./internal/application/service/ -run 'TestAgentForkLineageMigration|TestSubmitReleaseLineageVerdict|TestSubmitReleaseRedistributionGate|TestRegisterLicense' -count=1 \
  && go test ./internal/application/repository/ -run 'TestFindDerivation|TestLicenseUpsertGetAndList' -count=1 \
  && go test ./internal/modules/agentruntime/agent/experts/ -count=1 \
  && go test ./internal/router/ -run 'TestAgentForkLineage|TestTenantAgentAdoption|TestTenantAgentMarketplace|TestTenantAgentVariant|TestPublicMarketplace|TestAvailableAgents' -count=1
# github.com/Tencent/WeKnora/cmd/server
ld: warning: ignoring duplicate libraries: '-lc++'
# github.com/Tencent/WeKnora/cmd/desktop
ld: warning: ignoring duplicate libraries: '-lc++'
ok  	github.com/Tencent/WeKnora/internal/application/service	13.943s
ok  	github.com/Tencent/WeKnora/internal/application/repository	2.679s
ok  	github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/experts	1.452s
ok  	github.com/Tencent/WeKnora/internal/router	43.581s
```

退出码 0，全 PASS。（运行时长显著短于首轮是 Go 测试缓存的构建结果复用，`-count=1` 保证测试逻辑本身全部重新执行。）

### 提交

- `2032e0c42` test(marketplace): make forged-lineage 400 attribution sound with legal metadata and honest-body control (#62 修复轮 1)——1 file changed, 21 insertions(+), 3 deletions(-)，仅 `internal/router/routes_agent_fork_lineage_test.go`（本任务授权文件）。

### 对「待核实项」的回应

- 本轮实测复跑了端到端三测试与计划级全链（输出如上），独立复核了首轮报告声称的结果形态（三测试 PASS、全链 exit 0）。
- Task 0-4 生产代码仍不在本任务 diff 范围内，其正确性由上述全链测试覆盖，同意在后续全分支合并审查中复核。

---

# 收口验证轮报告（2026-09-26，修复轮 1 之后的第 6/6 任务重派发）

**状态：DONE**
**本轮提交：仅本报告文件（docs 记录）；生产代码与测试代码零改动**

## 本轮性质

本轮是修复轮 1（`2032e0c42`）之后对 Task 5（计划第 6/6 任务）的收口验证：任务描述给定的前置接口「修复后的 forge 断言组（合法 metadata + unknown field 归因断言 + honest 201 对照）」已由 `2032e0c42` 完整产出，交付物 `internal/router/routes_agent_fork_lineage_test.go`（327 行）在位且与两轮报告一致。本轮职责为真实运行计划 Task 5 全部指定验证命令并如实报告终态；验证全绿，无实现缺口，故无新 RED→GREEN 周期（TDD 的 RED→GREEN 已由首轮两处修正与修复轮 1 的 forge 归因断言真实完成，见上文两轮报告）。

## 交付物核对（开工时逐项实读）

- `internal/router/routes_agent_fork_lineage_test.go`（327 行）：
  - forge 归因闭环断言在位：`:194-218`（完整合法 `forgedMetadata` + 400 + 响应点名 `json: unknown field \"fork_source_release_id\"` + 同 body 去掉伪造字段 honest 201 对照）——Review Focus 3 的 HTTP 层钉死不再是恒真断言；
  - 首轮两处修正仍在位：`:124-135`（`submitForkVersionRaw` metadata 含 `capability_requirements`）、`:282-291`（fork Release 落派生 agent 自有 listing 的注释与 `NotEqual` 断言）、`:306/:315/:323`（公共提升用 fork submission 自身 `listing_id`）；
  - 三个测试函数齐全：`TestAgentForkLineageMappingEditsAreNotForks`（AC1）、`TestAgentForkLineageRedistributionForbiddenRejectsSubmission`（AC2 租户 lane）、`TestAgentForkLineagePublicSubmissionLicenseGate`（AC2 公共 lane）。
- worktree `.worktrees/issue30-sweep-t62` 的实际分支名为 `codex/issue30-t62`；工作区在开工与收尾时均 `git status --porcelain` 干净，开工时 HEAD = `638435601`。

## 测试命令与完整输出（全部于本会话真实运行）

### Task 5 Step 2（计划指定命令）

```
$ go test ./internal/router/ -run 'TestAgentForkLineage' -count=1 -v
=== RUN   TestAgentForkLineageMappingEditsAreNotForks
--- PASS: TestAgentForkLineageMappingEditsAreNotForks (1.72s)
=== RUN   TestAgentForkLineageRedistributionForbiddenRejectsSubmission
--- PASS: TestAgentForkLineageRedistributionForbiddenRejectsSubmission (2.21s)
=== RUN   TestAgentForkLineagePublicSubmissionLicenseGate
--- PASS: TestAgentForkLineagePublicSubmissionLicenseGate (2.35s)
PASS
ok  	github.com/Tencent/WeKnora/internal/router	8.505s
```

### Task 5 Step 3（计划指定命令）

```
$ go test ./internal/router/ -run 'TestTenantAgent|TestPublicMarketplace|TestAvailableAgents' -count=1
ok  	github.com/Tencent/WeKnora/internal/router	17.988s
（exit=0）
```

### 计划级验证命令全链（计划「计划级验证命令」一节）

```
$ go build ./... \
  && go test ./internal/application/service/ -run 'TestAgentForkLineageMigration|TestSubmitReleaseLineageVerdict|TestSubmitReleaseRedistributionGate|TestRegisterLicense' -count=1 \
  && go test ./internal/application/repository/ -run 'TestFindDerivation|TestLicenseUpsertGetAndList' -count=1 \
  && go test ./internal/modules/agentruntime/agent/experts/ -count=1 \
  && go test ./internal/router/ -run 'TestAgentForkLineage|TestTenantAgentAdoption|TestTenantAgentMarketplace|TestTenantAgentVariant|TestPublicMarketplace|TestAvailableAgents' -count=1
# github.com/Tencent/WeKnora/cmd/desktop
ld: warning: ignoring duplicate libraries: '-lc++'
# github.com/Tencent/WeKnora/cmd/server
ld: warning: ignoring duplicate libraries: '-lc++'
ok  	github.com/Tencent/WeKnora/internal/application/service	6.345s
ok  	github.com/Tencent/WeKnora/internal/application/repository	1.325s
ok  	github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/experts	1.470s
ok  	github.com/Tencent/WeKnora/internal/router	23.418s
chain-exit=0
```

（`ld` 警告为既有环境噪音非错误，与前两轮一致；`-count=1` 保证全部测试逻辑重新执行，时长缩短为构建缓存复用。）

## 自检发现

1. **无新缺陷、无新改动需求**：三组验证命令全绿，AC1（`is_fork=false` 不误判）、AC2 两条 lane（409 fail closed / 翻转 live 放行 / 恢复 201）、Review Focus 3（forge 400 归因 + honest 201 对照）、Review Focus 4（原始内容无 lineage + 既有回归零漂移）的钉死证据全部在真实 HTTP 层成立。
2. **未运行项**：无——计划 Task 5 的 Step 2/3 与计划级验证命令均已在本会话实际执行；未运行的只有计划外的全仓 `go test ./...`（计划明示以定向 `-run` 规避全量 flaky 套件，非本任务验收项）。
3. **约束遵守**：本轮零生产代码、零测试代码改动；未触碰 `internal/container/`、TS 全部与他人文件；未推送远端；未撤销任何前序轮次修改。
