# T37 #67 整计划最终修复批次报告（t67 final-fix round）

- 分支：`codex/issue30-t67`（worktree `.worktrees/issue30-sweep-t67`）
- 基线：`8fb630990`
- 范围：整计划最终审查的 5 项发现（2 important + 3 minor），一次批次全部处置
- 安全约束对齐：Mimosa 生成前约束「服务端请求 URL 时仅允许 http/https；发请求前校验 host，拒绝 localhost、环回、私有和保留地址」——本批次把该约束补齐到官方 gateway/expo 通道与 FCM token_uri 通道（此前仅企业 APNs/FCM endpoint 覆盖）。

## 发现 1（important）：official 通道 endpoint 无 host 校验 → 已修复

**改动**：`internal/container/container.go:1126-1149`。装配层在 official 通道分支前计算
`officialBlocked := endpoint != "" && workbenchservice.DisallowedPushEndpointHost(endpoint) != nil`，
命中时 official 落 `NewDisabledNotificationProvider()`（fail closed），expo 与 gateway/http 两分支对称覆盖。

**语义细分**（保住既有测试与运维可辨识性）：
- 空 URL：gateway 分支保留既有「未配置」fail-closed（错误码 `mobile_notification_provider_unconfigured`）；expo 空值回落公网默认 `https://exp.host/--/api/v2/push/send`，天然过 gate。
- 非空但指向 loopback/私有/保留主机：fail closed 为 `mobile_notification_provider_disabled`。
- 校验保持在装配层，provider 层维持 scheme-only（`validNotificationEndpoint` 未动）——Task 5 httptest e2e（`internal/application/repository/mobile_push_isolation_test.go` 直连 provider 构造器，不经装配层）实测不受影响（见回归证据）。

**回归测试**：`internal/container/mobile_notification_provider_wiring_test.go`
- `TestMobileProviderAssemblyOfficialGatewayHostGateFailsClosed`（`https://10.1.2.3/gateway` → Disabled）
- `TestMobileProviderAssemblyOfficialExpoLoopbackGateFailsClosed`（`http://127.0.0.1:2197` → Disabled）
- `TestMobileProviderAssemblyEmptyEndpointKeepsUnconfiguredSemantics`（空 URL 仍是 unconfigured，非 Disabled）
- `TestMobileProviderAssemblyHealthyPublicGatewayIsConfigured`（公网域名放行）

## 发现 2（important）：装配行为零测试 → 已补装配级测试

**改动**：新增 `internal/container/mobile_notification_provider_wiring_test.go`（package container，10 个测试用例），全走可观察行为（`Configured()` 可选接口 + `Send` 错误码），无真实网络（Disabled/空 endpoint 不出网；企业 lane 用 `devices=nil` 时 resolver 先行失败的 `InvalidRegistration` 码证明「已装配」）：

| 用例 | 钉住的行为 |
| --- | --- |
| OfficialGatewayHostGateFailsClosed | gateway + 私网 URL → Disabled |
| OfficialExpoLoopbackGateFailsClosed | expo + 环回 URL → Disabled |
| EmptyEndpointKeepsUnconfiguredSemantics | 空 URL ≠ 被拒，保留 unconfigured 码 |
| HealthyPublicGatewayIsConfigured | 公网 URL 放行，Configured()=true |
| UnknownProviderModeFallsBackToEmptyEndpoint | 未知模式忽略已配 URL → 空 endpoint |
| UndeclaredEnterpriseAppHasNoEnterpriseLane | 未声明企业 App → 无 AppRouting |
| IllegalEnterpriseAppFailsClosed（3 子用例） | `official`/`enterprise:Bad_App`/`not-an-app-id` 均不建企业 lane |
| EnterpriseApnsPrivateEndpointFailsClosed | APNs 私网 endpoint → 企业 lane Disabled、official lane 存活（OR 语义） |
| EnterpriseFcmTokenURILoopbackFailsClosed | 凭据文件 token_uri 指环回 → 企业 lane Disabled |
| EnterpriseFcmPublicTokenURIWiresLane | 公网 token_uri → lane 已装配 |

## 发现 3（minor）：FCM token_uri 无 host 校验 → 已补齐

**改动**：
- `internal/modules/workbench/notification/fcm.go`：新增只读访问器 `TokenURL()`（暴露解析后的交换 URL：显式覆盖 / 凭据文件 `token_uri` / Google 默认）。provider 层校验策略不变（构造器仍 scheme-only），httptest 单测 `fcm_test.go:128` 传显式 `tokenServer.URL` 不受影响。
- `internal/container/container.go` fcm 分支：`source` 构造成功后再对 `source.TokenURL()` 过 `DisallowedPushEndpointHost`，不通过则不装配（企业 lane 落 Disabled）。凭据文件 `token_uri` 是生产唯一路径（装配传空覆盖），至此与 gateway host 校验同一防线。

**回归测试**：`TestMobileProviderAssemblyEnterpriseFcmTokenURILoopbackFailsClosed` + `TestMobileProviderAssemblyEnterpriseFcmPublicTokenURIWiresLane`（见上表）。

## 发现 4（minor）：DisallowedPushEndpointHost 不解析 DNS → 已注明边界

**改动**：`internal/modules/workbench/service/workbench/notification_delivery.go:219-237` 文档注释补充：
- 覆盖面更新为「official gateway/expo、APNs/FCM、FCM 凭据文件 token_uri」全部 push URL；
- 明确已知边界：只校验字面 host（IP 字面量与 localhost 名），**不解析 DNS**——公网域名解析到私网可穿过；与移动端 `disallowedDeploymentHost` 先例对齐，管理面配置、厂商 endpoint 均为固定公网 DNS 名（exp.host / api.push.apple.com / fcm.googleapis.com / oauth2.googleapis.com），属可接受取舍；仅在 endpoint 变为运营方自定义域名时需重审。

## 发现 5（minor）：混合部署告警语义无文档 → 已入册

**改动**：`docs/architecture/integration/workbench.md` §7 新增两条：
- push endpoint host 防线条目：装配层对所有 push URL 运行 `DisallowedPushEndpointHost`、disabled 与 unconfigured 错误码区分、DNS 不解析边界；
- 混合部署运维语义（commit 48d109b98 有意取舍的入册）：official=disabled 且企业通道活跃时，每个到期 official intent 会 claim→config 类失败→暂停共享 `mobile` 健康键，`alert_count` 随退避尝试递增（指数退避兜底，非告警风暴）；企业 lane 不被饿死（聚合 `Configured()` OR 语义）；恢复 official 配置后共享键在下次投递成功时自动 recover。

## 回归证据（本会话实际执行的命令与结果）

1. `go build ./internal/container/ ./internal/modules/workbench/...` + `go vet`（同三包）→ **BUILD+VET OK**。
2. `go test ./internal/container/ -run 'TestMobileProviderAssembly' -v` → **10/10 PASS**（含 3 个子用例）。
3. `go test ./internal/modules/workbench/notification/ -v` → **17 PASS，包 ok**。
4. `go test ./internal/application/repository/ -run 'TestMobilePush' -v` → **Task 5 e2e 2/2 PASS**（`TestMobilePushIsolationOfficialAndEnterpriseNeverMix`、`TestMobilePushDisabledKeepsIntentsDurable`）——「不破坏 Task 5 httptest e2e」约束实测满足。
5. `go test ./internal/container/`（整包）→ 仅 1 个失败 `TestWireCraftInteractionRegistrarRegistersPendingInteractions`；经 `git stash -u` 后在 base `8fb630990` 上**逐字复现**（craft_interaction_wiring_test.go:70 同样报错）→ 预先存在，与本批次无关。
6. workbench service 包失败集合 base 对比：`go test ./internal/modules/workbench/service/workbench/ -v | grep '--- FAIL' | sort`，base 20 项 vs 改动后 20 项，**集合完全一致**（仅计时抖动）。
7. `internal/application/repository` 整包失败集合 base 对比：**276 项完全一致**（预先存在）。

### 预先存在的已知破损（非本批次范围，如实上报）

`migrations/sqlite/` 存在重复迁移文件 `000112_task_grants.{up,down}.sql` 与 `000112_agent_adoption_variants.{up,down}.sql`（#42/#59 遗留，`mobile_push_isolation_test.go:4-6` 注释已记载）。凡加载完整迁移目录的测试（workbench service 的 admission/notification delivery 系列、container 的 craft wiring、application/repository 大部分）在 base 上即失败。本批次所有受影响测试均能运行的证据来自：不经完整迁移目录的装配/单测（第 1、2、3 项）与 Task 5 的聚焦迁移 e2e（第 4 项）。

## 未做/不在范围

- 未修复重复迁移文件（属 #42/#59 范围，超出本修复批次授权）。
- 未给 `DisallowedPushEndpointHost` 增加 DNS 解析（发现 4 明示为可接受边界，仅要求注明）。
