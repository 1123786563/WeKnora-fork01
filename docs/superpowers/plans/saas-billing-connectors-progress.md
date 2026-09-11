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
| F03 | done | 待提交（本次变更） | RED：`go test ./internal/commercial -run 'Test(Prorate|Quote|Catalog)' -count=1` 因 undefined: Prorate/QuoteAmount/Segment 等失败；GREEN：同命令 12/12 通过；`go vet ./internal/commercial ./internal/application/repository/commercial`：通过；`go build` 同两包：通过；`git diff --check`：通过；仓库层（published 不可变、ConsumeQuote 单事务）仅经 vet/build 编译验证，SQLite 行为测试未包含于本任务文件清单故未运行，不标 pass | 由协调者复核 |
| F04 | pending | 未实施 | 未运行 | 等待 F03 |
| F05 | pending | 未实施 | 未运行 | 等待 F02, F03 |
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
