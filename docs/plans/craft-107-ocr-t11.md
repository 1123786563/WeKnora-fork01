# T11 OCR 报告（第 2 次运行）

范围说明：ask 给定线性范围 d84192a87^..fd371cfa7 为 T11 的 6 个连续提交。前 4 个第 1 轮已审（8 条 findings），已由 bb02a191d 全实修复（view() 非 restricted 不投影、digest/身份拒绝补审计、decodeCraftBody、去重窗、runShareAction 反馈、复用 contribution 组装、前端 restricted 守卫、declined 再决策入口），fd371cfa7 为豁免复核（无代码变更仅入册）。本轮审查第 1 轮之后的增量段 --from 926c99e23 --to fd371cfa7（含全部修复 diff，8 文件中 3 个非测试源码被审）。

## 本轮：--from 926c99e23 --to fd371cfa7

Review complete: 2 finding(s) across 3 selected item(s).

─── internal/application/service/craft_share.go:294-304 ───
[maintainability · medium] DecideShare 尾部的内联组装完整复制了 view()（第 209-216 行）的投影逻辑：Restricted 门控、Decision 投影、以及 state==Consented 时的 ShareDecisionTTL 到期计算。这两处是同一业务规则的平行实现：若 TTL、撤销语义或门控条件任一侧演进而另一侧未同步，DecideShare 的即时响应将与随后 GET /share 读到的持久化投影互相矛盾（例如过期时间计算漂移、门控不一致导致同一决策在两个接口呈现不同状态）。建议提取共享 helper（如 projectShareDecision(view, contribution, recorded, state)），view() 与本处统一调用；helper 内保留 DecisionBinds 检查对 DecideShare 刚写入的行同样成立，不影响正确性。

─── internal/application/service/craft_share.go:378-387 ───
[bug · low] 拒绝审计去重键 (tenant, actor, action, scope_id, target_id, outcome) 不包含拒绝原因：reason 只存在于 Details JSON 中，而同一 version 上共有三种不同的拒绝原因（attempted_decision / caller_identity_mismatch / evidence_digest_mismatch）共用同一 action='craft.share_denied' 与同一 target=versionID。60 秒窗口内，任意一种拒绝先落行后，同 actor 同 version 的其它原因拒绝会被静默吞掉——例如 evidence_digest_mismatch（replay 信号，包契约声明 "every proven refusal is audited"）可能被一条更早的 RequireTaskAccess 拒绝压制而不落审计行。被引用的姊妹实现 auditTaskDenial 特意将区分维度（task action）编码进 TargetID 列，使不同拒绝互不吞并；本处注释声称只去重 "replaying the same denial"，实际实现比声明更宽。建议将 reason 纳入去重键（如按 reason 派生不同的 action 值，或用与列类型匹配的 JSON 谓词匹配 reason），使不同原因的拒绝在窗口内各自独立去重。

第 1 轮 8 条 findings 的修复经重审确认无回归，新暴露 2 条修复引入的边界问题（投影平行实现、去重键缺 reason 维度）。
