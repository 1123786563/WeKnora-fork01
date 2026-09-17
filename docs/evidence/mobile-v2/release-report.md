# Release Report · mobile-v2（MX-036）

**基线** main `6a70c35a` → 交付分支 `codex/expo-mobile-v2`（HEAD 见 acceptance-index.json）。未 push、未发布——发布门禁只准备材料。

## 1. 裁决

| Profile | 裁决 | 依据 |
|---|---|---|
| **core** | **releasable-with-conditions** | 29/36 任务 accepted（含 6 项收尾批复核待做）；核心链（身份/空间/任务/事件/审批/恢复/通知/用量）全绿；条件：native-e2e 设备证据（MX-034 blocked-env）+ 视觉/性能矩阵（MX-035 设备层）+ 收尾批复核 |
| remote | **blocked-env（能力关闭）** | MX-027 未激活（条件边：无 live Paseo 环境与授权）；MX-032 manifest 强制 unavailable |
| oidc/resources/connectors/dictation/voice | unavailable（证据门禁） | 实现面交付，环境证据 blocked-env |
| personal_node / full_happy | deploy-disabled | 未实施/原矩阵未达成 |
| **globalAllPassed** | **false** | 如实——不以 core 通过冒充全量 |

## 2. 变更范围

Go（契约/交互/SSE v2/聚合/inbox/voice/能力门禁 + 全链路由 DI）、共享 TS（contracts/api-client/domain 十余新模块）、Expo 产品端（weknora/ 产品域全量：UI 组件、18 页中的 17 页 Screen、平台层持久化/上传/深链/偏好）、证据体系（~230 文件锁、D-001–D-027 决策、19 份任务证据、故障矩阵、能力门禁清单）。

## 3. 回退方案

能力开关回退（CapabilityService.RevokeCapability + manifest deploy_enabled=false）：新版本先关新准入，旧运行保留清理/结算（服务端 Run 不中断）；无破坏性迁移。原生与 OTA 兼容窗口：SDK55 锁定组合（MX-002 官方矩阵）+ expo-updates 未启用（首版走商店包）。

## 4. 交接

- 复现：`pnpm run test:mobile-v2`（41/41）、`pnpm run test:shared`（590/590）、`go test ./internal/...`（100 包）、挂载套件 9/9、E2E 显式命令（blocked-env 正确红）。
- 责任人：协调者（本会话）；解除 blocked-env 需用户授权（受控账户/设备构建/IdP/供应商）。
- 风险：见 acceptance-index.json conditions + 各任务证据剩余项。

## 5. 单独授权项（未申请、未执行）

支付/商户、公开个人节点、E2EE、真实 Provider 写入、push、生产部署。
