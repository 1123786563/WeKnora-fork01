# Profile Gates · 能力清单与门禁（MX-032）

**能力公式**：`capability = 部署开关 ∩ 服务健康 ∩ 用户权限 ∩ 验收证据`。证据缺失=unavailable（入口隐藏，不以置灰按钮冒充功能）。撤销后服务端拒绝新准入（RefreshCapability 不清除撤销——重新准入走发布门禁）。

## 各 profile 当前状态（代码 CapabilityService + deploy/mobile-workbench/profile-manifest.json）

| Profile | 部署 | 证据要求 | 当前裁决 | 依据 |
|---|---|---|---|---|
| core | ✓ | —（本轮 core 范围内验收） | **supported**（本轮范围内） | MX-001–015/017–021/030/031 accepted（登录/空间/任务/事件/审批/恢复/通知/清理）；发布验收在 MX-036 |
| oidc | ✓ | oidc:real-idp-e2e | **unavailable** | 真实 IdP 双平台回跳证据 blocked-env（无授权 IdP 实例） |
| resources | ✓ | resources:knowledge-list-wiring | **unavailable** | 知识列表 API 服务端接线未核验（MX-022 客户端就绪） |
| connectors | ✓ | connectors:connection-api-wiring | **unavailable** | 连接服务端 API 未核验（MX-023 客户端就绪） |
| remote | ✓ | remote:cancel-evidence + remote:paseo-live-probe | **unavailable（missing_cancel_evidence）** | MX-027 未实施（条件边未激活）；live Paseo blocked-env |
| personal_node | ✗（deploy 关闭） | personal-node:node-binding-evidence | **unavailable（deploy_disabled）** | 未实施 |
| dictation | ✓ | dictation:engine-wiring | **unavailable** | 识别引擎接线 blocked-env（MX-028 状态机就绪） |
| voice | ✓ | voice:media-provider-e2e | **unavailable** | 真实媒体/推送环境 blocked-env（MX-029 服务端+控制器就绪） |
| full_happy | ✗（deploy 关闭） | full-happy:original-matrix-complete | **unavailable（deploy_disabled）** | 原矩阵 H24–H33 完整实现未达成 |

## 门禁机制

- `CapabilityService.EvaluateCapability`：三态裁决（supported/unavailable+原因/forbidden+原因）——Go 测试钉住 frozen 场景（remote 缺取消证据）。
- `RevokeCapability`：core 不可撤销；撤销位独立于部署/健康（重新准入=发布门禁证据齐备后显式操作）。
- `profile-manifest.json`：部署侧开关与证据要求清单——probe 校验清单与代码要求一致（remote 必须要求 cancel 证据；personal_node/full_happy 必须 deploy 关闭）。
- 客户端消费：能力 unavailable → 入口不渲染（隐藏而非置灰）；forbidden → 显示无权限（MX-017/019 组件已按三态消费）。

## 待授权/待环境项（不得开放）

真实 IdP、推送供应商、语音媒体供应商、Paseo live 节点、真机 E2E（iOS+Android）、PostgreSQL 并发证据——全部 blocked-env/未授权，对应 profile 保持 unavailable。
