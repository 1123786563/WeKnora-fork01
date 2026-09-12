### Task 3: O03: 按证据收口全部接口与产品联验

> **协调者补充（deviation note）**：(1) 66 个接口检查 ID 分布已核实：ALI-5、BUD-8、COM-7、CON-8、FS-4、NO-4、OM-10、OPS-4、SYNC-5、USE-6、WX-5，表格在 docs/superpowers/specs/2026-09-10-saas-billing-connectors-interface-verification.md（行格式 `| ID | 检查 | 通过标准 |`，状态以表格上方散文记录，多为 pending）；AC-01～AC-20 验收标准表在 docs/superpowers/specs/2026-09-10-saas-billing-connectors-design.md（~352 行附近）。(2) gate.py（V03 先例）展示了仅认 runtime 证据的准入门风格；release_gate.py 消费同风格 JSON。(3) 真实联验的诚实预期：当前环境无商户凭据/OpenMeter 服务/真实飞书 Notion 授权，provider/browser 级检查必然 blocked-env——CLI 对完整必需集必须退出 1，验收文档列出已完成（unit/integration）与未完成（provider/browser/blocked-env）范围，绝不宣称全链路成功；功能门槛可逐能力关闭（--withoutCapability 之类），但最终报告必须列出未完成范围。(4) 修改 interface-verification.md 时：只按真实证据更新状态记录（引用提交与测试命令），不得把 pending/blocked-env 改为 pass。

**Files:**
- Create: `scripts/saas/release_gate.py`、`scripts/saas/release_gate_test.py`
- Create: `docs/superpowers/plans/saas-billing-connectors-acceptance.md`
- Modify: `docs/superpowers/specs/2026-09-10-saas-billing-connectors-interface-verification.md`

**Interfaces:**
- Consumes: 所有任务产出的真实记录；规格AC-01～AC-20；66个接口检查ID。
- Produces: `require_pass(records:dict, required:set[str]) -> None`；验收报告明确unit/integration/provider/browser层级，不把skip当pass。

**行为与边界：** 先跑fake report证明gate会拒绝缺项，再完成真实联验；如果商户/外部目标未授权，保留对应blocked-env并禁止宣称全链路成功。整分支复核仅涉及本计划拥有文件，必要时用code-review/verification技能。

**验收映射：** AC-01 至 AC-20；G0 至 G5；全部66接口检查

- [ ] **Step 1: 在 `scripts/saas/release_gate_test.py` 写入以下失败断言。**

```python
import unittest
from scripts.saas.release_gate import require_pass
class ReleaseTests(unittest.TestCase):
    def test_mock_cannot_prove_provider_acceptance(self):
        with self.assertRaises(ValueError):
            require_pass({'WX-05':{'status':'pass','level':'mock','evidence':['x']}},{'WX-05'})
    def test_missing_case_is_failure(self):
        with self.assertRaises(ValueError): require_pass({}, {'AC-01'})
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
python3 -m unittest scripts.saas.release_gate_test -v
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```python
def require_pass(records: dict, required: set[str]) -> None:
    provider_cases = {"WX-05", "ALI-05", "FS-04", "NO-04"}
    for key in required:
        row = records.get(key, {})
        if row.get("status") != "pass" or not row.get("evidence"):
            raise ValueError(f"missing passing evidence: {key}")
        if key in provider_cases and row.get("level") != "provider":
            raise ValueError(f"real provider evidence required: {key}")
```

验收文件逐项引用AC-01～20、66接口ID及真实证据路径。按完整规格的输入和期望逐个执行跨空间、双渠道、月度、升级、退款、并发、父子Runtime、同步、两种写操作与回退场景。浏览器需要真实服务，保存对应页面状态/用户动作及后端关联，不只截图静态HTML。

CLI读取JSON记录 `--report artifacts/saas-acceptance/report.json`，从规格和检查清单解析全部必需ID；缺项、blocked-env、skip和mock替代真实提供方均退出1。功能门槛可逐能力关闭，但最终报告必须列出未完成范围。

- [ ] **Step 4: 加入本任务"行为与边界"列明的反例，并执行同一定向命令。**

```sh
python3 -m unittest scripts.saas.release_gate_test -v
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add scripts/saas/release_gate.py scripts/saas/release_gate_test.py docs/superpowers/plans/saas-billing-connectors-acceptance.md docs/superpowers/specs/2026-09-10-saas-billing-connectors-interface-verification.md docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "test: require complete commercial and connector acceptance evidence"
```
