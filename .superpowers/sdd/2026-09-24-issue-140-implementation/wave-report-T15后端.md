# Wave 4 报告 — T15 后端：可信结构化材料与不可变版本（Issue #153，implement）

- 状态：**DONE**（TDD RED→GREEN→重构完成，全部验证命令原样通过）
- BASE `6a21e0df5` → HEAD `f78b85a48`（worktree `/Users/wuyongjun/.codex/worktrees/issue-140-t15-material/WeKnora-fork01`，本地提交未 push）
- 提交：`f78b85a48` `feat(career): version structured materials immutably`（13 files，+1971/−13）
- worktree 内详报：`.superpowers/sdd/2026-09-24-issue-140-t15-material/task-1-report.md`
- 评审包：worktree 内 `.superpowers/sdd/2026-09-24-issue-140-implementation/review-6a21e0df5..f78b85a48.diff`（1 commit，106442 字节）

## RED 证据（先写失败测试）

1. `go test ./internal/modules/career/... -count=1` → build failed：`material_test.go:43: undefined: MaterialClaim/MaterialBody/EditMaterialInput`、`handler_test.go:293: h.EditMaterial undefined`。
2. `go test ./internal/database/... -run TestMaterialMigrationUpAndDown` → FAIL：`unable to find file .../000200_career_materials.up.sql`。
3. `go test ./internal/router/... -run TestCareerMaterialRoutesAreRegistered` → FAIL（7 条路由未注册）。

实现中一次中间失败（2 tests）：创建路径 claim 审阅位于事务内 revision CAS 之后，Propose/Confirm 推进 revision 后 stale CAS 抢先报错。修复为统一次序 **replay → claim 审阅 → revision CAS** 后全绿。

## GREEN 证据（verbose，10 个命名测试 + HTTP/路由合同）

```
--- PASS: TestEditMaterialFreezesOpportunityAndProfileEvidence (0.14s)
--- PASS: TestMaterialClaimsLinkOnlyConfirmedFacts (0.12s)
--- PASS: TestMaterialDoesNotFabricateMissingExperienceCertificatesNumbers (0.19s)
--- PASS: TestConfirmBodyCreatesImmutableVersionAndOldVersionsComparable (0.08s)
--- PASS: TestMaterialEditNeverOverwritesExistingVersions (0.12s)
--- PASS: TestMaterialFailurePreservesDraftAndReason (0.11s)
--- PASS: TestMaterialExactReplayAndChangedIntentConflict (0.46s)
--- PASS: TestMaterialRevisionConflictReturnsCurrentRevision (0.08s)
--- PASS: TestMaterialScopeRejectsOtherTenantAndOwner (0.20s)
--- PASS: TestMaterialMigrationUpAndDown (1.46s)      [internal/database]
--- PASS: TestCareerMaterialHTTPContract (0.01s)      [handler 合同]
--- PASS: TestCareerMaterialRoutesAreRegistered       [router]
```

## 验证命令与真实输出（简报 §4 原样）

```
gofmt -w internal/modules/career internal/router   → 无输出（干净）
go test ./internal/modules/career/... -count=1     → ok  12.097s
go test ./internal/database/... -count=1           → ok  28.910s
go test ./internal/router/... -count=1             → ok   3.487s
go test ./tools/architectureguard/... -count=1     → ok   1.083s
git diff --check                                   → 干净
go build ./...                                     → 干净（仅存量链接器警告）
```

architectureguard 精确新值：**literal 590→597、total 659→666**（+7 路由）；apiKey 69 / handle 0 / workers 23+23 / hooks 58 不变。

## 接口冻结（Web/移动/小程序消费，JSON 形态）

路由（前缀 `/api/v1/career`）：`POST /materials`（创建/编辑草稿）、`POST /materials/confirm`（确认成不可变版本）、`GET /materials/receipt?requestId=`、`GET /materials/:materialId`、`GET /materials/:materialId/versions`、`GET /materials/:materialId/versions/:versionId`（版本号自 1）、`GET /materials/:materialId/versions/:versionId/compare?baseline=N`。

请求/收据 JSON、冻结语义、错误码（409 `material_claim_unconfirmed` 等）见 worktree 详报 §3；CareerRemote seam：`act({kind:"edit_material", payload}, requestId, expectedRevision)` 即 `POST /materials`。

关键语义：
1. 创建冻结 `{opportunityId, snapshotId, snapshotSha256, profileRevision}`（ApplicationEvidencePin 先例 + 快照摘要），后续档案/岗位变更不回写，编辑不可重定向 pin。
2. claim.factKey 必须指向已确认 fact（pending proposal 拒绝，HTTP 409）；裸主张（无 factKey 且非 needs_review）= 补造，拒绝。
3. 缺失实习/证书/数字只能 `needsReview=true` 显式占位 + `missing_placeholder` 审阅风险。
4. confirm → 版本自增 + 整快照（正文/风险/证据/确认时事实基准 factBasisRevision）；`(tenant,user,material,version)` 唯一 → DB 层不可覆盖；旧版本永远可读可 compare（section/claim diff）；为 T18 预留 `(materialId, version)` 寻址，不实现投递。
5. 审阅失败保留草稿与 `failure_code=claim_unconfirmed`/`failure_message`（typed 可恢复；成功编辑清除并回 draft）。
6. 幂等/并发/scope 全 house 语义：exact replay 返回原收据、同 request ID 变更 409、revision 冲突返回 currentRevision、跨租户/他人 404。

## 迁移

`migrations/versioned/000200_career_materials.{up,down}.sql` + `migrations/sqlite/000121_career_materials.{up,down}.sql`（本轮唯一分配编号；目录最大 199/120 实核一致）。三表：`career_materials` / `career_material_versions` / `career_material_receipts`。SQLite 链尾版本 120→121；`NewOffice` AutoMigrate 清单与 `validateSQLiteCareerSchema`（列+唯一约束）同步扩展；down→120 删三表保既有 schema，重放 up 恢复（TestMaterialMigrationUpAndDown 覆盖）。注：`.gitignore` 排除 `migrations/`，按 000120 先例 `git add -f`。

## 文件清单（全部在声明 ownership 内）

新建：`internal/modules/career/material.go`（1042 行）、`material_test.go`（445 行）、4 个迁移 SQL。
修改：`office.go`（+8：模型清单+schema 校验）、`handler.go`（+165：7 handlers+错误映射）、`handler_test.go`（+96）、`routes_career.go`（+7）、`routes_career_test.go`（+20）、`internal/database/career_migration_test.go`（+92：新测试+版本 121+表清单）、`tools/architectureguard/discovery_test.go`（597/666+注释）。

## 自查与遗留

- 只改声明文件；未 push/merge；未动主仓库与集成分支。
- PDF/DOCX 属 T16 未实现；投递绑定属 T18 仅预留版本寻址。
- claim 校验基准=当前确认集合（版本快照固化 factBasisRevision）；创建 pin 的 profileRevision 是不可移动的生成基准——两层语义由测试 1/2 分别钉住。
- 已知局限：并发同草稿编辑 last-writer-wins（版本永不覆盖）；审阅风险码为服务端推导（missing_placeholder/needs_review）。
