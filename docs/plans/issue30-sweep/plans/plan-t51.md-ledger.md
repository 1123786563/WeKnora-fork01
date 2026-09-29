# plan-t51 Ledger（执行历史）

本文件是 T21 #51 实施计划（plan-t51.md）的执行 ledger，记录计划执行期间的关键裁决与越权授权。

## 2026-09-26 — Task 3 修复轮 1

Task 3: Ruling — F1 排除集冻结 TOCTOU 授权越界下沉至 store CAS（Task 1 文件），依据 Spec 审批漂移不变量 + 纯实现层强化无架构决策（区别于 t59 延期案） — 代价：越权 Task 1 已 review 文件（强化不推翻）；repository/appconnector/plan.go 若与并行 #53/#54 分支冲突由集成修复员兜底（主控裁决 via escalation）

- **审查发现**: F1（important）——`internal/modules/appconnector/plan/plan.go` Approve 的排除集冻结检查是 read-then-act，`repository/appconnector/plan.go` ApprovePlan CAS WHERE 允许 `state IN {awaiting,authorized}` 不钉 excluded_json，两个并发 Approve 携不同 ExcludeSeqs 可双双通过、后写者覆写冻结决定并可批准先前被排除的项（「排除集首次批准后冻结」在并发下失守）。
- **裁决落点**: 授权实现员越界修改 Task 1 的 `internal/modules/appconnector/repository/appconnector/plan.go`，把冻结比对下沉进 CAS WHERE（`AND (state = 'awaiting_approval' OR excluded_json = ?)`，参数绑定），TDD 先行（store 层确定性 RED → GREEN）+ service 层并发不变量回归测试。
- **依据**: 排除集冻结是「排除项永不批准执行」审批完整性不变量的承载（#30 Spec 审批漂移）；实现弱于计划意图即修，无新架构决策成分。
- **F2（Task 0 未执行）**: 按审查定性为跨任务协调项，不在本修复轮修——记入 plan-t51.md-report.md 供主控/编排层收口（Task 6 AC3 全量迁移 e2e 前置）。

## 2026-09-26 — Task 5 修复轮 4

Task 5: Ruling — 在案『Task 0 需编排方授权』的授权由本裁决给出（终结 4 轮空转）；方案由计划原文的 marketplace→118 改写为 mobile_device_app→118/197（与其余分支统一，避免主动制造 000118 双占；免 migration.go 六处联动） — 代价：偏离 t51 计划原文（主控全局占用表优先）；该任务的同 finding 已消耗 4 轮 + Task 3 侧 F2 5 轮未收敛，流程教训（在案裁决若不附授权即派发将致空转）入最终报告（主控裁决 via escalation）

- **裁决落点**: 授权实现员（Task 5 接替者 R4）在本修复轮直接执行 Task 0，重编对象为 mobile_device_app：git mv 四文件（sqlite 000114→000118、versioned 000193→000197）+ 同步 5 个 mobile_device 系测试文件引用；**不动 migration.go（门控常量/探测串指向 marketplace 文件，原号不动即零联动）、不动 marketplace 文件、不动已落位的 000119/000198**。
- **执行结果**: 提交 `e760c9255`（9 files, +17/−17，与兄弟分支模板提交 `34849775b` 逐字节一致——`git diff --cached 34849775b -- <9 路径>` 空输出）；`go test ./internal/database/ -count=1` 由 FAIL（duplicate migration file）转 **ok**（Task 0 生效标志）；`go test ./internal/handler/ -run 'TestNotionPublish|TestAppPublicationsTableExists' -count=1` 3 个 #48 e2e 由 FAIL 转 **PASS**。
