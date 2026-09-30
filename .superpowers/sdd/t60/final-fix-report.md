# T60 整计划最终修复报告

日期：2026-09-25　分支：`codex/issue30-t60`　基线：`a9d01848b`

最终审查发现 2 项（均 minor），1 项代码修复、1 项审批记录确认，全部处理完毕。

---

## 发现 1（minor，已修复）：GetPublicListing 不按目录可见性过滤

**审查原文要点**：`GET /marketplace/public/listings/:id`（Viewer+）可检视首个 submission 已落库但平台尚未批准（`current_release_id` 为 NULL）的公共 listing，提前暴露 `display_name/summary/publisher_tenant_id`；`ListPublicCatalog` 只列 `current_release_id` 非空行而 detail 不过滤。

### 根因核实（本会话读取证据）

- `internal/application/repository/public_marketplace.go:152-159`：首个 submission 落库即创建公共 listing，`State: "listed"` 且 `current_release_id` 未设置（NULL）——审批窗口期真实存在。
- `internal/application/repository/public_marketplace.go:304`：`ListPublicCatalog` 在 SQL 层过滤 `state = 'listed' AND current_release_id IS NOT NULL`——列表端点窗口期不可见。
- `internal/application/service/public_marketplace.go:243-278`（修前）：detail 读到 `listing != nil` 即返回 `display_name/summary/publisher_tenant_id`，无窗口期过滤——泄漏确认。
- 路由：`internal/router/routes_public_marketplace.go:35`，Viewer+ 可达；handler `publicMarketplaceClientError`（`internal/handler/public_marketplace.go:186-188`）已将 `marketrepo.ErrPublicMarketplaceNotFound` 映射 404，服务层返回 NotFound 即可闭环。

### 修复（internal/application/service/public_marketplace.go:255-260）

在 `listing == nil` 检查后新增目录可见性闸：`State != listed` 或 `CurrentReleaseID == nil` 一律返回 `repository.ErrPublicMarketplaceNotFound`（→ handler 404），与 `ListPublicCatalog` 同口径。

**范围裁定**：过滤只加在服务层 detail 读取，**不动 repo 层**——`ReviewPublicSubmission`（`public_marketplace.go:191`）内部同样走 `repo.GetPublicListing`，首个批准时 `CurrentReleaseID` 必为 NULL，repo 层加过滤会令首个批准永久失败。`AdoptPublicListing` 原有 `State != listed → NotFound`（:289）+ release 归属校验（:303），窗口期无 release 行可返回，不泄漏 listing 元数据，无需改动（与审查"AC1/AC2 均不受影响"结论一致）。

### 覆盖测试与执行证据

新增 `TestPublicMarketplaceServiceGetListingMatchesCatalogVisibility`（`internal/application/service/public_marketplace_test.go:185-237`），四段断言：

1. 审批窗口内 detail 404 + 同刻 `ListPublicCatalog` 为空（口径一致性）；前置断言证明窗口期 `state == "listed"` 且 `CurrentReleaseID == NULL`；
2. 批准后 detail 可见且带 current release、`PublisherVerified` 正确（正常路径不被误伤）；
3. state 转 `unlisted` 后 detail 404（state 分支，衔接 #63 unlist）；
4. 未知 id 保持 404 语义。

**RED**（暂存修复跑新测试）：`git stash push -- internal/application/service/public_marketplace.go` 后
`go test ./internal/application/service/ -run TestPublicMarketplaceServiceGetListingMatchesCatalogVisibility -count=1` → **FAIL**，失败输出正是泄漏本体：批准前返回 `DisplayName:"Public helper", Summary:"Portable", PublisherTenantID:0x1`。`git stash pop` 恢复修复。

**GREEN**（恢复修复后）：
`go test ./internal/application/service/ -run 'TestPublicMarketplace' -count=1 -v` → 5/5 PASS（含既有 4 个回归）。

### 回归证据

`go build ./...` 通过（仅既存 ld duplicate libraries 警告）；`go vet` 三包无输出。

四包全量：`go test ./internal/application/service/ ./internal/application/repository/ ./internal/handler/ ./internal/router/ -count=1` 首轮 repository/handler/router 三包 ok，service 包 FAIL（该轮未带 `-v`，未能捕获失败测试名；当时 4 大包联跑 + build 并行）。随后两次单独重跑均全绿：

- `go test ./internal/application/service/ -count=1 -v`（过滤 FAIL 行）→ 无任何 FAIL 行；
- `go test ./internal/application/service/ -count=1 > /tmp/t60-svc-test.log 2>&1` → **exit=0，ok 354.042s**。

判定：首轮 service FAIL 为偶发（若系本修复引入，后续重跑应稳定复现；且 service 包内仅新增测试调用 `GetPublicListing`，grep 已核实无其他调用点），非本修复所致——如实记录，不做无据断言。

---

## 发现 2（minor，审批记录确认，无需代码动作）：c71b1f37a 计划外越界修复的裁定记录

审查结论为"已核实无遗留问题"，本会话逐项复核记录完整性：

- **commit 存在且纯测试改动**：`git show --stat c71b1f37a` → 仅 `internal/router/routes_agent_adoption_test.go` 1 file changed, 11 insertions(+), 10 deletions(-)（15 行测试改动，无生产代码影响）。
- **Ledger 完整记录**：`.superpowers/sdd/plan-t60/progress.md:35` 完整记载 escalation 裁定——授权越界修复 routes 层 publish 闭包（B3 9f273fe49 漏改遗留、handler 已钉死 data 即 variant 本体 B3-F86、15 行测试改动、无其他 owner、污染 Task 7/8 gate）、单独提交 c71b1f37、双向同绿。
- **双向同绿本会话复验**：
  - `go test ./internal/router/ -run TestTenantAgentAdoptionPublishesIndependentVariantsIntoMobileAvailableAgents -count=1 -v` → PASS；
  - `go test ./internal/handler/ -run TestPublishVariantReturnsVariantBodyDirectly -count=1 -v` → PASS。

结论：裁定授权、单独提交、Ledger 记录、双向同绿四要素齐备，无遗留问题，无需任何代码或文档动作。

---

## 变更汇总

| 文件 | 变更 |
|---|---|
| `internal/application/service/public_marketplace.go` | +7：GetPublicListing 目录可见性闸（与 ListPublicCatalog 同口径） |
| `internal/application/service/public_marketplace_test.go` | +52：覆盖测试 `TestPublicMarketplaceServiceGetListingMatchesCatalogVisibility` |

合计 2 files changed, 59 insertions(+)。报告：`.superpowers/sdd/t60/final-fix-report.md`。
