# Wave 3 — T11 后端子任务报告：C 对话入口的一次性真实找岗（Issue #152，implement）

- **Worktree**: `/Users/wuyongjun/.codex/worktrees/issue-140-t11-search-once/WeKnora-fork01`（独立，未 push）
- **BASE**: `d88cd513a` → **HEAD**: `cfde041e19f9dd89b2495552c1202155279264aa`
- **Commits**:
  1. `054ef4263` test(career): cover frozen source failure classifications（Step 0 偿还 T09 五分类断言债务；附带修复 fetch-error 路径 Completeness 与冻结枚举不一致的真 RED）
  2. `cfde041e1` feat(career): run one-shot job searches（主任务）

完整报告（RED/GREEN 证据、接口冻结 JSON、命令原始输出、已知局限、遗留）见 worktree 内
`.superpowers/sdd/2026-09-24-issue-140-t11-search-once/task-1-report.md`。

## 摘要

- **HTTP 合同（冻结，供 Web 子任务）**：`POST /api/v1/career/searches`（requestId/query/expectedRevision）→ `SearchOnceReceipt`（kind=search_once；status completed|failed；coverage=实际已核验来源的如实清单；results 行带 checkedAt/qualification(未评估=needs_review，消费 T10 冻结映射)/link(仅字面抓到的 URL)/uncertainty(low_confidence)）；`GET /career/searches/receipt?requestId=`；`GET /career/searches/:searchId`。429 search_quota_refused / 504 outcome_unknown / 409 revision_conflict(currentRevision) / 409 idempotency_conflict / 404 not_found 均可恢复。一次性同步执行可完成，未设 reconcile 路由（receipt 端点即恢复路径，循 ImportURL 先例）。
- **诚实边界**：生产空 allowlist/空 registry → no_vetted_sources+空覆盖+零伪造，无"全国"表述；额度仅注入式 `SearchQuotaGate` seam（生产 pass-through 不虚构，真实额度 T21）；绝不创建持续规则（无 %rule% 表/零 change 行/revision 不动，测试固定）；不经 CareerApplicationTaskLinker、不写 Workbench 表；网络 I/O 全在 DB 事务外且复用 T09 SSRF transport，未另造 fetch。
- **迁移**：versioned 000199 / sqlite 000120（career_searches + career_search_results，唯一键实测）；architectureguard 587/656 → 实测 **590/659**。
- **验证（全跑）**：gofmt 干净；`go test ./internal/modules/career/...`（15.5s ok）、`./internal/database/...`（54.3s ok）、`./internal/router/...`（7.3s ok）、`./tools/architectureguard/...`（0.8s ok）、`git diff --check` 干净；9 个 SearchOnce 测试全过（含固定 fixture 与 httptest+生产 transport 两套合同场景）；提交后全套复跑 9/9 绿。
- **遗留**：T09 既有 `TestImportURLReplayConflictAndConcurrentSingleObservation` 在整套高负载下偶发 "database table is locked"（预存在 flake：Step 0 仅加测试文件时即复现；隔离 5/5 通过），建议单独开票加固；`docs/architecture/moves/README.md` 注记未更新（不在所有权内）。
