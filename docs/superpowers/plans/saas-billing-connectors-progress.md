# SaaS 商业能力与 Connector 实施进度

更新：2026-09-11。总计划：[实施计划](2026-09-11-saas-billing-connectors-implementation.md)。状态：计划已整理，功能实现未开始。

状态值：pending / implementing / review / done / blocked。done必须包含实现提交、定向验证和对应门槛证据；没有真实环境时保留blocked并说明影响，不能用skip充当成功。

| 任务 | 状态 | 实现提交 | 验证结果 | 下一步 |
| --- | --- | --- | --- | --- |
| V01 | review | 待提交 | `python3 -m unittest scripts.saas.contract_inventory_test -v`：3/3 通过；外部只读探测未运行（blocked-env） | 在固定官方 schema 文件上运行 hash 校验与四个 GET 探测 |
| V02 | review | `待提交` | `python3 -m unittest scripts.saas.probe_case_test -v`：5/5 通过；`git diff --check` 通过；外部真实业务实验未运行（blocked-env） | 提交后由主协调器复核；需在可用 OpenMeter 环境执行 OM-02～OM-10 |
| V03 | pending | 未实施 | 未运行 | 等待 V02 |
| F01 | pending | 未实施 | 未运行 | 等待 V03 |
| F02 | pending | 未实施 | 未运行 | 等待 V03 |
| F03 | pending | 未实施 | 未运行 | 等待 F01, F02 |
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
