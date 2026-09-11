# SaaS 商业能力与 Connector 实施进度

更新：2026-09-11。总计划：[实施计划](2026-09-11-saas-billing-connectors-implementation.md)。状态：计划已整理，功能实现未开始。

状态值：pending / implementing / review / done / blocked。done必须包含实现提交、定向验证和对应门槛证据；没有真实环境时保留blocked并说明影响，不能用skip充当成功。

| 任务 | 状态 | 实现提交 | 验证结果 | 下一步 |
| --- | --- | --- | --- | --- |
| V01 | review | 待提交 | `python3 -m unittest scripts.saas.contract_inventory_test -v`：3/3 通过；外部只读探测未运行（blocked-env） | 在固定官方 schema 文件上运行 hash 校验与四个 GET 探测 |
| V02 | review | `待提交` | focused regression suite and diff checks pass; external OM-02～OM-10 remains blocked-env | 提交后复核 |
| V03 | blocked | f84b98b | `python3 -m unittest scripts.saas.gate_test -v`：4/4 通过；`git diff --check` 通过；CLI 对 selected-model.json 返回 blocked；OM-01～OM-10 blocked-env（无 OpenMeter 服务／商户凭据，未伪造 pass） | 准备隔离环境后重跑两套官方模型实验；在 OM-01～OM-09 全部 runtime pass 前 gate 拒绝 P01～P03 |
| F01 | done | 5a1111e8e77b3338cbc364656c475c6c90cacf7a | `go test ./internal/commercial -run 'Test(MonthBoundary|ParseCredits|CreditsString)' -count=1`：通过；`go test ./internal/commercial -count=1`：通过；`git diff --check`：通过 | 提交后复核 |
| F02 | done | bb7e8c6b64c64ba34fda6d8bb7176b577a52e1cb | `go test ./internal/commercial ./internal/application/repository/commercial -run 'Test(BillingAccess|Account)' -count=1`：4/4 通过；`go vet`：通过；`go build`：通过；`git diff --check`：通过；SQLite 集成覆盖同租户幂等、租户/Customer 双唯一冲突与并发竞争；独立审查 PASS（task-F02-review.md，B1–B7 全过）；PostgreSQL 并发证据仍 blocked-env | 已 PASS，生成并派发 F03；COM-01/AC-01/02 的真实 PostgreSQL 证据待环境可用后补跑 |
| F03 | done | 2f4462ace578d5db341f7ce5284ba24bdefa63dc | RED：`go test ./internal/commercial -run 'Test(Prorate|Quote|Catalog)' -count=1` 因 undefined: Prorate/QuoteAmount/Segment 等失败；GREEN：同命令 12/12 通过；`go vet ./internal/commercial ./internal/application/repository/commercial`：通过；`go build` 同两包：通过；`git diff --check`：通过；仓库层（published 不可变、ConsumeQuote 单事务）仅经 vet/build 编译验证，SQLite 行为测试未包含于本任务文件清单故未运行，不标 pass；独立审查 PASS（task-F03-review.md，C1–C9 全过） | 已 PASS，生成并派发 F04 |
| F04 | done | af899cefb0bfb0f7d013e8ee057f40e7e7a07717 | RED：`go test ./internal/commercial ./internal/application/service/commercial -run 'Test(Monthly|Lifecycle)' -count=1` 因 undefined: MonthlyGrantKey/Subscription/BaseTier 等失败（service 包当时不存在，commercial 包 RED 为有效 RED）；GREEN：同命令 12/12 通过（5 域用例 + 7 SQLite 集成：同周期 8 worker 并发恰一 job、外部已发未确认 pending+external_ref 重放不换 key、年付逐月 Tick、提前续费不提前发未来月、过期保留 top-up 行并记录 base 档投影与超限原因、外部发放成功保存批次 ID、失败保 key）；`go vet` 三包通过；`go build` 三包通过；`git diff --check` 通过；`gofmt -l` 五个新 Go 文件收尾时修复 2 个 service 文件格式后为空；真实 PostgreSQL 迁移与外部发放方端到端证据 blocked-env 未运行，不标 pass；独立审查 PASS（task-F04-review.md，D1–D8 全过） | 已 PASS，生成并派发 F05 |
| F05 | done | 195cf6ec0fbffae49322c38a428704658a44c3b3 | GREEN（实现者中断后由 finisher 验证，RED 阶段未经 finisher 复核）：`go test ./internal/commercial ./internal/router -run "Test(Quota|Commercial)" -count=1`：11/11 通过（5 配额纯函数：超限拒绝增长/清理放行/零上限非无限/负计数拒绝/溢出防护；6 SQLite httptest：同用户 A/B 空间 usage/summary 不串、无 grant Admin POST 403 且 owner/billing-grant 放行、25 并发新增 CAS 不超限且计数=落库行数、充值 credits 不动资源配额、/health 关库后仍 200 不触计费、full-access key 403 且显式 commercial capability 放行）；brief 原命令 `-run "Test(Quota|CommercialScope)"` 6/6 通过；`go vet ./internal/commercial ./internal/router ./internal/container ./internal/handler ./internal/types`：通过；`go build` 同五包：通过；`gofmt -l` 8 个新增/改动文件修复 2 处对齐后为空；`git diff --check`：通过；`go test ./internal/router ./internal/handler ./internal/types ./internal/commercial ./internal/container -count=1` 全通过；finisher 收尾修复（均为最小修复）：handler commercialUserID 改用 types.UserIDFromContext（原 c.Get 读 gin Keys 取不到请求上下文 user，grant 判定恒 false）、types 新增 APIKeyCapabilityCommercial 常量+NormalizeAPIKeyCapability 分支（原 "commercial" 未注册被归一化剥离，HasCapability 恒 false，无法发放）、handler 常量改为别名 types.APIKeyCapabilityCommercial、gofmt 对齐 commercial.go/router.go；PostgreSQL 方言与外部计费端到端证据 blocked-env 未运行，不标 pass；独立审查 PASS（task-F05-review.md，E1–E7 全过） | 已 PASS；F 基础段完成，生成并派发 C01 |
| C01 | pending | 未实施 | 未运行 | 等待 F03, F05 |
| C02 | pending | 未实施 | 未运行 | 等待 C01 |
| C03 | pending | 未实施 | 未运行 | 等待 C02 |
| C04 | pending | 未实施 | 未运行 | 等待 C01, F04, V03 |
| C05 | pending | 未实施 | 未运行 | 等待 C02, C03, C04, U02, U03 |
| U01 | pending | 未实施 | 未运行 | 等待 F01, F03 |
| U02 | pending | 未实施 | 未运行 | 等待 U01, V03 |
| U03 | pending | 未实施 | 未运行 | 等待 U02, C04 |
| U04 | pending | 未实施 | 未运行 | 等待 U03, F02, F04 |
| U05 | pending | 未实施 | 未运行 | 等待 U01, U02, U03, U04 |
| A01 | pending | 未实施 | 未运行 | 等待 F02 |
| A02 | pending | 未实施 | 未运行 | 等待 A01 |
| A03 | pending | 未实施 | 未运行 | 等待 A02, U05 |
| A04 | pending | 未实施 | 未运行 | 等待 A03 |
| A05 | pending | 未实施 | 未运行 | 等待 A04 |
| A06 | pending | 未实施 | 未运行 | 等待 A04 |
| A07 | pending | 未实施 | 未运行 | 等待 A02, U05 |
| W01 | pending | 未实施 | 未运行 | 等待 C01, C05, F05 |
| W02 | pending | 未实施 | 未运行 | 等待 W01 |
| W03 | pending | 未实施 | 未运行 | 等待 W01, C05 |
| W04 | pending | 未实施 | 未运行 | 等待 A01, A02, A07 |
| W05 | pending | 未实施 | 未运行 | 等待 A03, U04, W04 |
| O01 | pending | 未实施 | 未运行 | 等待 C04, C05, U03, A03 |
| O02 | pending | 未实施 | 未运行 | 等待 F02, F04, U01, A07, O01 |
| O03 | pending | 未实施 | 未运行 | 等待 W02, W03, W04, W05, O01, O02, C02, C03, A05, A06, U05 |

本次只完成计划与文档自检，未执行实际实现测试、外部付款或Connector写入。
