# plan-t51 Ledger（执行历史）

本文件是 T21 #51 实施计划（plan-t51.md）的执行 ledger，记录计划执行期间的关键裁决与越权授权。

## 2026-09-26 — Task 3 修复轮 1

Task 3: Ruling — F1 排除集冻结 TOCTOU 授权越界下沉至 store CAS（Task 1 文件），依据 Spec 审批漂移不变量 + 纯实现层强化无架构决策（区别于 t59 延期案） — 代价：越权 Task 1 已 review 文件（强化不推翻）；repository/appconnector/plan.go 若与并行 #53/#54 分支冲突由集成修复员兜底（主控裁决 via escalation）

- **审查发现**: F1（important）——`internal/modules/appconnector/plan/plan.go` Approve 的排除集冻结检查是 read-then-act，`repository/appconnector/plan.go` ApprovePlan CAS WHERE 允许 `state IN {awaiting,authorized}` 不钉 excluded_json，两个并发 Approve 携不同 ExcludeSeqs 可双双通过、后写者覆写冻结决定并可批准先前被排除的项（「排除集首次批准后冻结」在并发下失守）。
- **裁决落点**: 授权实现员越界修改 Task 1 的 `internal/modules/appconnector/repository/appconnector/plan.go`，把冻结比对下沉进 CAS WHERE（`AND (state = 'awaiting_approval' OR excluded_json = ?)`，参数绑定），TDD 先行（store 层确定性 RED → GREEN）+ service 层并发不变量回归测试。
- **依据**: 排除集冻结是「排除项永不批准执行」审批完整性不变量的承载（#30 Spec 审批漂移）；实现弱于计划意图即修，无新架构决策成分。
- **F2（Task 0 未执行）**: 按审查定性为跨任务协调项，不在本修复轮修——记入 plan-t51.md-report.md 供主控/编排层收口（Task 6 AC3 全量迁移 e2e 前置）。
