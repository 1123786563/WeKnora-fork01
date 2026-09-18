# Mobile Workbench Release Runbook (MX-036)

> 状态：**材料就绪，未发布**。发布需单独授权（不在本轮执行）。

## 发布前置（按序核验）

1. `pnpm run test:mobile-v2` 42/42；`pnpm run test:shared` 590/590；`go test ./internal/...` 全绿。
2. 收尾批复核闭环（acceptance-index.json 中 review-pending 项）。
3. MX-034 设备证据采集（native-e2e.md 配方）+ MX-035 视觉/性能矩阵回填——或按裁决保留 conditions 发布 core。
4. 能力门禁核对：deploy/mobile-workbench/profile-manifest.json 与 CapabilityService 一致（probe 已钉）。

## 构建

- 锁定组合（MX-002）：expo ~55.0.31 / react 19.2.0 / RN 0.83.10（官方 SDK55 矩阵）。
- `cd apps/mobile && pnpm exec expo prebuild -p ios` / `-p android` → xcodebuild / gradlew（dev client 或 store build 按渠道）。
- 首版走商店包（expo-updates 未启用；OTA 窗口随后续版本评估）。

## 发布顺序（灰度）

1. 服务端先发（Go：契约/SSE v2/inbox/voice/能力端点——向后兼容 v1）。
2. 客户端后发（version=2 SSE 在客户端升级后自动协商）。
3. 灰度开 core；其余 profile 保持 manifest 关闭直到证据齐备。

## 回退

- 客户端：商店版本回退（无 OTA）。
- 服务端能力：`CapabilityService.RevokeCapability(profile)`（新准入立即拒绝）+ manifest deploy_enabled=false——旧运行保留清理与结算（Run 不中断）。
- 数据：无破坏性迁移；回退不改服务端 Run/结算。

## 禁止事项

- 未授权不得：push、合并共享分支、真实 Provider 写入、支付通道、生产部署、公开个人节点、E2EE。
