# SaaS 00 官方接口验证 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 得到唯一、可复现且有证据的官方商业接入契约。

**Architecture:** 先建立只读盘点与隔离实验工具，再运行语义实验；结果失败阻止依赖能力，不用旧 fork 补洞。

**Tech Stack:** 当前仓库 Go 1.26、Gin、GORM、PostgreSQL／SQLite 版本化迁移；React 19、TypeScript 6、pnpm 10.28.2；官方 OpenMeter 固定版本；优先复用现有依赖。

**Spec:** [完整规格](../specs/2026-09-10-saas-billing-connectors-design.md)；[接口验证清单](../specs/2026-09-10-saas-billing-connectors-interface-verification.md)。执行者必须同时阅读本计划引用的规格章节与上下游契约。

## Global Constraints

- 每个空间始终独立购买、付款并承担费用；共享组织不改变归属，不共享余额或套餐。
- 费用预算批准和外部写入批准是独立条件，两者都满足才能执行。
- 首期不包含：跨空间合并付款或余额池、自动扣款、独立席位与存储加购包、嵌入式第三方 Web 应用、公开开发者上架与交易分成。
- 固定候选基线：`v1.0.0-beta.232`，提交 `887e0cac903ccd06e74d61ed23c651651d10c7a9`。
- 未在验证清单取得真实通过证据的外部能力，不得标记为生产可用；不使用旧本地 OpenMeter fork 接口。
- 保留已有 React、Craft、移动端和其他任务的工作；仅提交本任务明确拥有的文件，不使用 `git add .`。
- 下列内部接口是计划新增契约，不声称当前已存在；外部请求字段以 P00 的已验证契约为准。
- 时区、精度与分段报价采用完整规格的推荐默认值；外部模型无法无损表达时在 P00 停止依赖任务并修正规格，不能自行换商业语义。

---

## 依赖与执行边界

无代码实现依赖；当前本地端口曾 connection refused，先查明环境状态，不能把 schema 当联调。

本文件仅编写计划，不表示已实现或已测试。执行时先使用 using-git-worktrees 建立隔离工作区；所有命令从仓库根执行。每一步只做所列一个动作；较长代码修改按列出的文件／函数逐项执行，完成该任务的 RED→GREEN 与独立 review 后再进入下一任务。

## 文件结构与职责

`scripts/saas/` 负责可复现验证；`deploy/openmeter/contract-cases/` 保存脱敏案例与选模；`artifacts/` 存执行证据。脚本使用 Python 标准库，不安装依赖。

### Task V01: 固定版本接口盘点与只读健康探测

**Files:**
- Create: `scripts/saas/contract_inventory.py`、`scripts/saas/contract_inventory_test.py`
- Create: `scripts/saas/__init__.py`
- Modify: `docs/superpowers/specs/evidence/2026-09-10-saas-billing-interface-inventory.json`

**Interfaces:**
- Consumes: 固定提交的官方两份 schema；清单中的 local_path 和 SHA-256。
- Produces: Python `operation_paths(text: str, prefix: str) -> dict[str, str]`；CLI 输出版本、路径与探测结果，不输出凭据。

**行为与边界：** 缺 server 前缀、重复 operationId、schema hash 不符、连接拒绝分别测试。只读探测不能创建测试 Customer。

**验收映射：** OM-01；G0

- [ ] **Step 1: 在 `scripts/saas/contract_inventory_test.py` 写入以下失败断言。**

```python
import unittest
from scripts.saas.contract_inventory import operation_paths
class InventoryTests(unittest.TestCase):
    def test_v3_prefix_is_not_dropped(self):
        schema = "paths:\n  /openmeter/customers:\n    post:\n      operationId: create-customer\n"
        self.assertEqual(operation_paths(schema, "/api/v3"),
                         {"create-customer": "POST /api/v3/openmeter/customers"})
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
python3 -m unittest scripts.saas.contract_inventory_test -v
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

实现路径提取器；只处理固定 schema 的 paths 层级，未知结构直接拒绝而不输出猜测路径。

```python
import re

def operation_paths(text: str, prefix: str) -> dict[str, str]:
    result: dict[str, str] = {}
    path = method = None
    for line in text.splitlines():
        if re.match(r"^  /", line):
            path = line.strip()[:-1]
        match = re.match(r"^    (get|post|put|patch|delete):$", line)
        if match:
            method = match[1].upper()
        match = re.match(r"^      operationId: (.+)$", line)
        if match and path and method:
            if match[1] in result:
                raise ValueError("duplicate operationId")
            result[match[1]] = f"{method} {prefix}{path}"
    return result
```

CLI 使用 argparse，参数 `--schema PATH --prefix PREFIX --base-url URL --output PATH`；本地只读探测使用 `urllib.request.build_opener(ProxyHandler({}))`，超时 5 秒，只记录状态／异常类。读取 schema bytes 校验清单中的 SHA-256；不匹配返回退出码 2。GET 仅用清单四个路径，失败保留 blocked-env，不自动重启服务。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
python3 -m unittest scripts.saas.contract_inventory_test -v
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add scripts/saas/__init__.py scripts/saas/contract_inventory.py scripts/saas/contract_inventory_test.py docs/superpowers/specs/evidence/2026-09-10-saas-billing-interface-inventory.json docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "test: pin official commercial API inventory"
```

### Task V02: 可重复执行的业务契约实验驱动器

**Files:**
- Create: `scripts/saas/probe_case.py`、`scripts/saas/probe_case_test.py`
- Create: `deploy/openmeter/contract-cases/README.md`

**Interfaces:**
- Consumes: V01 的 operation_paths；验证清单 OM-02 至 OM-10。
- Produces: Python `assert_subset(actual: object, expected: object) -> None`；Case JSON 格式 `{id, operation_id, request, expected, captures, cleanup}`，expected 必须包含实际业务断言。

**行为与边界：** 嵌套余额不匹配必须失败；未授权写请求不得发送；未知 capture 和跨 namespace ID 拒绝。演示样例只能用于 runner 单测，不能生成 pass 业务证据。

**验收映射：** OM-02 至 OM-10 的实验基础

- [ ] **Step 1: 在 `scripts/saas/probe_case_test.py` 写入以下失败断言。**

```python
import unittest
from scripts.saas.probe_case import assert_subset
class ProbeTests(unittest.TestCase):
    def test_http_success_cannot_hide_wrong_balance(self):
        with self.assertRaises(AssertionError):
            assert_subset({"balance": "99"}, {"balance": "100"})
        assert_subset({"balance": "100", "id": "g1"}, {"balance": "100"})
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
python3 -m unittest scripts.saas.probe_case_test -v
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```python
def assert_subset(actual: object, expected: object) -> None:
    if isinstance(expected, dict):
        assert isinstance(actual, dict), "expected object"
        for key, value in expected.items():
            assert key in actual, f"missing {key}"
            assert_subset(actual[key], value)
    else:
        assert actual == expected, f"mismatch: {actual!r} != {expected!r}"
```

CLI 参数示例：`python3 scripts/saas/probe_case.py --case deploy/openmeter/contract-cases/om-04.json --base-url http://127.0.0.1:48888 --namespace saas-om04 --allow-test-writes`：默认拒绝 POST/DELETE；只允许显式 namespace 的测试 Customer。根据 V01 operationId 找路径，captures 仅接受 JSON Pointer，后续请求以捕获 ID 绑定，不作任意表达式求值。每步保存脱敏请求、响应和断言到 `artifacts/saas-contract/NAME/`；非 2xx 或断言失败退出 1；缺服务退出 2。不得记录 Authorization、API key、完整付款人信息。清理只能处理 namespace 中实际创建且无未结算交易的对象。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
python3 -m unittest scripts.saas.probe_case_test -v
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add scripts/saas/probe_case.py scripts/saas/probe_case_test.py deploy/openmeter/contract-cases/README.md docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "test: add isolated commercial contract probe runner"
```

### Task V03: 执行两套官方模型实验并冻结唯一接入契约

**Files:**
- Create: `scripts/saas/gate.py`、`scripts/saas/gate_test.py`
- Create: `deploy/openmeter/contract-cases/selected-model.json`
- Modify: `docs/superpowers/specs/2026-09-10-saas-billing-connectors-interface-verification.md`

**Interfaces:**
- Consumes: V02 probe runner；完整规格 3.1、4.1、6、8；官方 schema 及当前运行配置。
- Produces: `validate_gate(report: dict) -> None`；选模 JSON 含 family、version、operations、passing_checks、evidence、precision、time_resolution、settlement_ack_strategy。

**行为与边界：** 这是有条件执行门槛而非空白实现：失败应产出具体不满足的断言及替代方案，不允许双钱包。对 HTTP 成功但业务不符和虚假 pass 缺证据均有负例；不因缺商户而伪造退款链路。

**验收映射：** OM-01 至 OM-10；G0/G1/G2 前置；规格 17

- [ ] **Step 1: 在 `scripts/saas/gate_test.py` 写入以下失败断言。**

```python
import unittest
from scripts.saas.gate import validate_gate
class GateTests(unittest.TestCase):
    def test_schema_evidence_is_not_runtime_pass(self):
        with self.assertRaises(ValueError):
            validate_gate({"family": "official_v3", "checks": {"OM-02": "schema"}})
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
python3 -m unittest scripts.saas.gate_test -v
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```python
def validate_gate(report: dict) -> None:
    required = {f"OM-{n:02d}" for n in range(1, 10)}
    if report.get("family") not in {"official_v1_v2", "official_v3"}:
        raise ValueError("one official family required")
    checks = report.get("checks", {})
    if any(checks.get(key) != "pass" for key in required):
        raise ValueError("commercial capability gate incomplete")
    if not report.get("evidence") or not report.get("settlement_ack_strategy"):
        raise ValueError("runtime evidence and acknowledgement strategy required")
```

按下面检查顺序，每完成一个实验再继续；每项记录输入、响应、期望／实际。

- [ ] **实验 OM-01：** 按接口清单该行执行单一场景集合，保留脱敏请求与实际最终状态。
- [ ] **实验 OM-02：** 按接口清单该行执行单一场景集合，保留脱敏请求与实际最终状态。
- [ ] **实验 OM-03：** 按接口清单该行执行单一场景集合，保留脱敏请求与实际最终状态。
- [ ] **实验 OM-04：** 按接口清单该行执行单一场景集合，保留脱敏请求与实际最终状态。
- [ ] **实验 OM-05：** 按接口清单该行执行单一场景集合，保留脱敏请求与实际最终状态。
- [ ] **实验 OM-06：** 按接口清单该行执行单一场景集合，保留脱敏请求与实际最终状态。
- [ ] **实验 OM-07：** 按接口清单该行执行单一场景集合，保留脱敏请求与实际最终状态。
- [ ] **实验 OM-08：** 按接口清单该行执行单一场景集合，保留脱敏请求与实际最终状态。
- [ ] **实验 OM-09：** 按接口清单该行执行单一场景集合，保留脱敏请求与实际最终状态。
- [ ] **实验 OM-10：** 记录拟采用的确认与协调策略及约束，完整并发通过状态由 U02/U03 交付后更新。

V2/V3 的创建、月度到期、消费、部分撤回各使用不同 namespace。实际 request JSON 从固定版本 schema 的 required/properties 和官方样例写入 `contract-cases`，由 runner 运行后保存，不能凭本计划虚构字段。若环境不可达，记录 blocked-env 并停止依赖计划；不得写一个假通过 selected-model。

OM-01～OM-09 完成外部契约实验后写选模文件；OM-10 在此记录候选协调策略与可实现性证据，生产并发证明由 U02/U03 完成后才标记 pass。保存所选 operationId、request schema、response JSON Pointer、幂等查询规则、精度／时间分辨率及消费确认策略。把 4.1 时间精度和升级公式与实际能力差异同步到规格。文件的 `checks` 值必须来自真实结果，gate CLI `--report selected-model.json` 验证后才允许 P01～P03 的外部适配上线。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
python3 -m unittest scripts.saas.gate_test -v
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add scripts/saas/gate.py scripts/saas/gate_test.py deploy/openmeter/contract-cases docs/superpowers/specs/2026-09-10-saas-billing-connectors-interface-verification.md docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "test: record validated official commercial model decision"
```

