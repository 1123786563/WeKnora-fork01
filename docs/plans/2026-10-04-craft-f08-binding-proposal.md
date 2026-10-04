# F08 T-3 — CraftCallBinding（固定 web build 活动）取值方案（bounded proposal）

日期：2026-10-04 ｜ 状态：**已签认（方案 A）** ｜ 分支：fix/craft-f08 ｜ 依据：`2026-10-03-craft-f08-dispatch-design.md` §2.2、`2026-09-28-craft-107-f08-execution-receipt-design.md` §bounded interface proposal

## 提议的 fail-closed 默认值

`CraftWebBuildCallBinding()`（internal/application/service/craft_budget.go，常量 + 构造器）：

| 字段 | 取值 | 类比先例 |
| --- | --- | --- |
| DelegationID | `"web-build"`（常量 `CraftWebBuildCallDelegationID`） | 空 facet 在 `AuthorizeCall` 泛型命名空间会撞 (tenant, run, '', '', '', 0) 唯一元组（craft_budget.go:862 注释）；命名 facet 与 `craft_model_gateway.go:506` 的成员委托一致 |
| ModelID | `"__craft_web_build__"`（常量 `CraftWebBuildCallModelID`，哨兵 facet） | `craftSandboxCallModelID = "__craft_sandbox__"`（craft_budget.go:919）：非模型物理动作的 server-owned 哨兵 facet，`AuthorizeSandbox` 每活动可插 |
| Funding | `commercial.FundingPlatform`（"platform"） | `PlatformAdmissionPolicy()`（workbench/admission.go:285）：platform Run 的 server-owned 默认，服务端绑定产出、非成员声明；过 `ValidateFunding` 白名单 |

## 逐项 rationale

- **DelegationID="web-build"**：durable run/task 身份已由 (tenant, run, TaskID/SessionID) + 每次尝试的 ActivityKey + RequestSHA256 完整承载（回执唯一键与 charge-start 行均含之），delegation facet 只需**命名固定活动族**。哨兵空串有两撞风险（上述唯一元组），故取稳定命名常量。
- **ModelID="__craft_web_build__"**：build exec 不消耗任何模型 token，把准入快照里的会话 model_id 记进来会把 sandbox 维度的物理动作误记为模型维度账目（`BillableModel` 语义下模型维度本就与 sandbox 维度独立，usage.go:36-38）。方案 A（本提案）：稳定哨兵，账目按活动族聚合；**方案 B（备选，供 owner 裁量）**：`__craft_web_build__/<toolchain-digest>`，按工具链版本分账，代价是 toolchain 换代后行碎片化、且 digest 需进入 DeriveCallID 输入。默认 A，owner 可改 B。
- **Funding="platform"**：固定 build 是平台提供的服务能力，无 BYOK 凭据参与；与 PlatformAdmissionPolicy 的 platform_gateway 同源。BYOK 即使上线也只豁免模型维度，不影响本选择。

## 不得自造的红线（设计文档约束）

- T-1 派发编排器**只消费** `CraftWebBuildCallBinding()`，不得内联字面量或接受调用方传入的绑定身份。
- 本常量未经 owner 签认前，T-1 以注入 seam 落地（构造器参数），生产装配在签认后才接线。

## Owner 签认

- [x] 预算 owner：controller per user's 持续推进 delegation ｜ 2026-10-05
- [x] 裁定方案 A（稳定哨兵 `__craft_web_build__`，Funding=platform）｜ controller ｜ 2026-10-05

> 签认后接线：生产派发触发点落在 craft_runtime（web-kind delegation 成功后、candidate staging 前），绑定仅经 `service.CraftWebBuildCallBinding()` 消费（红线一维持）。
