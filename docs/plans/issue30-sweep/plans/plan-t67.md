# T37: 自托管盲推送与企业自签名模式（Issue #67）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让部署策略控制推送元数据暴露：标准客户端走无正文盲推送且可整体禁用（禁用后 App 前台仍向权威服务端同步）；企业自构建 App 以独立 App 身份注册设备并经独立 APNs/FCM Provider 投递——官方与企业 Token/设备注册在存储、投影、claim、撤销、Provider 路由全链路不混用。

**Architecture:** 在 `mobile_devices` 与 `mobile_notification_intents` 两表加入 `app_id` 身份维度（进主键与令牌排他索引，官方 `official` 永远合法、企业 `enterprise:<slug>` 必须命中部署声明的唯一允许清单 `MOBILE_ENTERPRISE_APP_ID`，注册 intent 的 HMAC 载荷绑定 App）。投递侧在 `workbenchservice` 增加 `AppRoutingNotificationProvider` 按 `Intent.AppID` 分派：official 走既有 expo/gateway 选择（新增 `MOBILE_NOTIFICATION_PAYLOAD=blind` 盲推模式与 `MOBILE_NOTIFICATION_PROVIDER=disabled` 禁用模式），enterprise 走新实现的 APNs（.p8 ES256 provider token）与 FCM（service account RS256 OAuth）真实 wire Provider（spec 认定 APNs/FCM 为 true external——Port + 可注入 endpoint/client 的真实现，本地以 httptest 在 Provider 边界取证，真机/真凭据验收单列 blocked-env）。客户端 `apps/mobile` 增加构建期 App 身份解析（`EXPO_PUBLIC_WEKNORA_APP_ID`，非法回落 official）与前台权威同步环（AppState active → 通知 Inbox 权威 `page()` 重投影，单飞合并、scope 每次事件重解析）——关闭网关时 App 不依赖推送仍可前台同步。

**Tech Stack:** Go 1.26（go.mod `github.com/Tencent/WeKnora`）、gin、GORM、golang-migrate（versioned/PostgreSQL + sqlite 两套独立序列）、`github.com/golang-jwt/jwt/v5`（go.mod 既有直接依赖，APNs ES256/FCM RS256 签名）、crypto/ecdsa + crypto/rsa + x509（stdlib）、httptest 端到端（真实迁移子集 sqlite + 真实 store/service/handler/worker + 真实 HTTP Provider 边界）；TypeScript（`packages/mobile-core`、`packages/api-client`、`apps/mobile` Expo RN 55）node:test + tsx。

**Spec:** `docs/plans/issue30-sweep/issues/issue-67.md`（验收标准原文）；领域术语 `CONTEXT.md`（「行动通知」87 行、「注册设备」96 行）；`docs/specs/2026-09-20-mobile-ai-office-design.md`（用户故事 65-67、Testing Decisions）；`docs/specs/2026-09-20-mobile-module-seams.md`（§5.3「推送只触发重新同步」、§12 true external 表：APNs/FCM = Push Port + mock/scripted Adapter）。

## Global Constraints

逐字引用批准需求与项目级约束（每个任务的要求都隐含本节）：

- 「官方与企业 Token/设备注册不混用。」（Issue #67 验收标准 1）
- 「关闭网关时应用仍可前台权威同步。」（Issue #67 验收标准 2）
- 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」（Issue #67 验收标准 3）
- 「标准客户端支持无正文盲推送且可禁用；企业自构建 App 使用独立标识和 APNs/FCM。」（Issue #67 What to build 原文）
- 「67. As a self-hosted administrator, I want to use a blind notification gateway, enterprise-signed app or no push, so that deployment policy controls metadata exposure.」（Spec 用户故事 67）
- 「65. As a member, I want push notifications only for required action, failure or unknown outcome, completion and important budget events, so that alerts remain useful.」「66. As a privacy-conscious member, I want push bodies to omit sensitive business content, so that lock-screen notifications do not leak data.」（Spec 用户故事 65-66——盲推模式是 66 在部署策略层的实现）
- 「推送只触发重新同步，不直接修改 Task 状态。」（module-seams §5.3 不变量）
- 「注册设备（Registered Device）：……用于推送寻址、本地密钥封装和企业设备策略；同一物理设备在不同部署实例中具有彼此隔离的注册，设备身份不取代用户身份，也不自动获得空间权限。」（CONTEXT.md:96）
- 「True external dependencies such as APNs, FCM, WebRTC, system audio and system share use mock or scripted Adapters at the Port and real-device acceptance separately.」（Spec Non-functional/Testing）
- 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」（module-seams §2 架构约束——`packages/mobile-core` 不出现 require/react-native）
- 服务端请求 URL 约束（Mimosa）：APNs/FCM/gateway Provider 仅允许 http/https，发请求前校验 host 并拒绝 localhost、环回、私有和保留地址（复用/新增 provider 的 endpoint 校验 `Configured()`； httptest 注入的 client 不触网）。
- 凭据约束（Mimosa）：APNs .p8、FCM service account JSON、gateway access token 一律只从环境变量/密钥文件路径读取；源码、示例和测试不写入可用凭据字面量（测试用即席生成的临时密钥文件）。
- SQL 约束（Mimosa）：仓储层所有新增 SQL 一律参数绑定（本计划只新增带 `?` 占位的 GORM 查询与迁移 DDL，无字符串拼接 SQL）。
- 测试归属：Task 1/2/3/4/6/7 的模块级单测钉各自实现；Task 5（真实迁移子集 sqlite + 真实 store/handler/projector/worker + httptest 真实 HTTP Provider 边界，`repository_test` 外部测试包全链）与 Task 7 的 opt-in 集成冒烟（真实 JSON transport + 真实 Runtime + 真实服务端）共同构成 AC3 证据；真机 APNs/FCM 投递与真实企业凭据验收列为 blocked-env（见差异记录第 6 条），不以 httptest 冒充真机证据。
- 任务结构：严格 RED → GREEN → REFACTOR；每任务以 commit 结束（若执行时由编排层接管提交，则 Commit 步骤改为「确认工作区仅含本任务文件」）。
- 迁移编号：versioned 取 **000193**、sqlite 取 **000114**（当前尾部 `migrations/versioned/000191_*` 双文件、`migrations/sqlite/000112_*` 双文件——#42 与 #59 并行同号；同批 B4 的 plan-t43 已声明取 000192/000113）。本计划与 B4 其余计划并行，**若集成时编号已被占用，整体顺延为下一个可用编号（内容不变）**，不得挤占他人编号。
- 共享文件最小改动：`packages/mobile-core/src/device/device-registry.ts`（#41 交付）只做向后兼容的可选参数扩展；`apps/mobile/src/composition.ts` 只加 App 身份与前台同步两处装配；Go 侧 `internal/handler/mobile_device.go`、`internal/container/container.go` 的修改逐函数列出。既有测试文件仅两处必要修改（`openMobileHandlerDB` 按序补 000059/000060/000114 三文件、`TestNotificationProjectEventDerivesDurableIdentity` 补 `app_id` 一列），其余测试全部落在新建文件。
- 本计划不修改 `mobile-runtime.ts`/`runtime/ports.ts` 任何既有行（与 #41 计划的并行冲突约定一致）。

## Review Focus

规格隐含、但任务测试未直接覆盖时最可能咬人的五类输入/失败模式（每条标注归属任务的测试）：

1. **企业 App 身份冒充**：任意已登录成员注册任意 `enterprise:<slug>`（或别人企业的 slug），借部署的企业 APNs/FCM 凭据向任意 token 投递。合理预期：服务端允许清单拒绝未声明的 App id（400），`MOBILE_ENTERPRISE_APP_ID` 未配置/非法时只有 official 通道（fail closed）。—— Task 4 `TestRegisterRejectsUndeclaredEnterpriseApp`（intent 签发与 register 两处 400）+ `internal/container/workbench.go` 非法声明回落零策略。
2. **跨 App token 接管**：企业注册用与官方相同的 token 抢注（或同 deviceId 注册互踩），把另一 App 的注册挤下线/覆盖。合理预期：令牌排他性按 `(environment, app_id, token_hash)` 划界，跨 App 永不互踩；同 App 内维持既有接管语义。—— Task 1 `TestTokenExclusivityIsPerApp` + `TestBindIsolatesOfficialAndEnterprise`；Task 5 e2e 再证。
3. **盲推半盲**：`MOBILE_NOTIFICATION_PAYLOAD=blind` 后 lock screen 仍出现 kind/标题（gateway body 带 `kind` 键、direct provider Title=kind、APNs alert 带 title）。合理预期：blind 时 direct payload Title/Body 为空、gateway JSON 无 `kind` 键、APNs 走 content-available 背景推送。—— Task 3 `TestPushPayloadPolicyBlindStripsKind` + `TestHTTPNotificationProviderBlindOmitsKind` + Task 2 `TestApnsProviderSendsAlertAndBackgroundPayloads`；Task 5 e2e 再证。
4. **禁用后 retry 风暴或静默丢行**：`disabled` 模式反复 claim→fail 刷告警，或直接把 intent 行丢弃。合理预期：复用既有 durable pause 语义——首次失败持久暂停、后续 RunOnce 在 claim 前跳过、行保持 pending、策略改回后可续投。—— Task 3 `TestDisabledNotificationProviderPausesDurablyWithoutRetryStorm`。
5. **撤销设备借另一 App 同 deviceId 行复活**：设备在企业 App 撤销后，官方 App 同 deviceId 的行让 Claim/Revalidate 的 EXISTS 误匹配（谓词缺 app 维度），企业 intent 照发。合理预期：Claim/Revalidate 的设备 EXISTS 显式 `d.app_id = intents.app_id` 对齐。—— Task 1 `TestClaimJoinsAppIDSoRevokedAppDoesNotResurrect` + Task 3 `TestAppRoutingProviderRevokesOnlyOwnAppRegistration`。

---

## 与调查结论的差异记录（以代码现状为准）

1. 调查称「apps/mobile 仅 15 个文件的 T01 登录闭环，无推送注册」。**已过时**：#41（B3）已交付设备注册全链——`apps/mobile/src/adapters/push-token.ts`（expo-notifications 惰性 require fail closed）、`composition.ts:215 registerActiveDeviceIfPossible`、`/inbox` 路由与 `InboxScreen`、`packages/mobile-core/src/device/device-registry.ts`、`packages/api-client/src/mobile/devices.ts`。#67 的客户端真实缺口收窄为：App 身份维度透传与「关闭网关时的前台权威同步」（当前 `/inbox` 只有手动刷新按钮，`composition.ts` 无 AppState 生命周期同步）。
2. 调查称「前置 #41、#66 均为 open 未完成」。**以代码现状为准**：两者的产出接口已在当前 HEAD（`fb5f6653a`）集成（上文 Consumes 列表与 `apps/mobile/src/composition.ts:24-38` 的 import 为证）。本计划直接消费。
3. 调查称「HTTP 网关 payload 仅含设备身份与 event/run/kind，无任务正文」。属实且保留：`notification_delivery.go:209` 的 gateway payload 无 title/body 正文；本计划在其上叠加部署级 blind 模式（进一步去掉 `kind` 元数据键），不改变「无任务正文」底线。
4. **迁移目录在当前 HEAD 因同号迁移而损坏（本计划作者实跑证实）**：`migrations/versioned/` 存在 `000191_agent_adoption_variants` 与 `000191_task_grants` 双文件、`migrations/sqlite/` 存在 `000112_*` 双文件（#42 与 #59 同批并行同号）。实跑 `go test ./internal/application/repository/ -run TestWorkbenchNotificationsTableExistsAfterMigrations -count=1` → **FAIL："duplicate migration file: 000112_task_grants.down.sql"**；`go test ./internal/application/repository/ -run TestTaskCollaborationEndToEndAC1 -count=1` 与 `go test ./internal/modules/workbench/service/workbench/ -run TestNotificationDeliveryRevalidatesBeforeProviderSend -count=1` 同因 **FAIL**（一切走 `migrator.Up()` 全目录的既有测试在 HEAD 均挂，属先在损坏，非本计划引入）。plan-t43 所称「仓库已接受同批并行同号先例」与实跑结果不符。**对策**：本计划全部新 Go 测试改用**聚焦迁移**模式——直接 `db.Exec` 指定 `.up.sql` 文件内容（既有先例 `internal/handler/mobile_device_test.go:20-27 openMobileHandlerDB` 只读 `000058_mobile_devices.up.sql`），不依赖全目录 `migrator.Up()`；本计划不修 #42/#59 的文件（归编排层集成处理），testCommand 只定向到不依赖全目录迁移的用例。
5. 调查称「推送 Provider 只有 Expo 与通用 HTTP 网关」。属实：`internal/modules/workbench/notification/` 仅 `expo.go`/`provider.go`；`container.go:1085 newMobileNotificationProvider` 单部署单选择（expo/gateway，未知 fail-closed）。本计划 Task 2/3 补 APNs/FCM 与按 App 路由。
6. **blocked-env 验收项（如实列出，不伪造）**：真实 APNs/FCM 服务端投递（需真实 .p8/service account 凭据与真机 token）无法在本地验证。本地替代证据：(a) APNs/FCM Provider 以 httptest 在真实 HTTP 边界取证（wire 形态、错误分类、盲推 payload、鉴权头）；(b) ES256/RS256 签名源以本地生成密钥逐位验证签名与声明；(c) Task 7 冒烟只消费服务端策略面（注册接受/拒绝），不宣称真机投递。真机验收与真实凭据留待部署方按 Spec「real-device acceptance separately」执行。
7. SQLite/持久化 TaskProjectionStore 存储选型 ADR 未决（B2-F23 延期项）与本计划无关：本计划不引入任何跨进程持久化缓存；前台权威同步环每次事件直接向服务端拉取，不落本地。

## 消费的前置接口（Consumes，前三批已集成于当前 HEAD）

- #41：`createDeviceRegistry(ports: DevicePorts): DeviceRegistry`（`packages/mobile-core/src/device/device-registry.ts`，两步注册/409 重取/token 接管/lease 围栏）；`createMobileDeviceRemote({ origin, request }): MobileDeviceRemote`（`packages/api-client/src/mobile/devices.ts`）；`createNotificationInbox`（`packages/mobile-core/src/inbox/notification-inbox.ts`，`page()` 即权威首页重投影）；`composition.ts` 的 `notificationInboxFor`/`registerActiveDeviceIfPossible`。
- #32/#66：`MobileRuntime.ports`（`packages/mobile-core/src/runtime/ports.ts` 的 `authorizedTransport`/`AppLifecyclePort` seam）、`RuntimeSnapshot.surface === 'authorized'` 判定、`process.env.EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN` 构建期环境变量先例（`composition.ts:58`）。
- 服务端：`repository.MobileDeviceStore`/`NotificationStore`/`NotificationProjector`/`NotificationDeliveryWorker`（`internal/application/repository/mobile_device.go`、`mobile_notification.go`、`internal/modules/workbench/service/workbench/notification.go`、`notification_delivery.go`）；`pushnotification.PushProvider`/`PushPayload`/`ProviderError`/`ClassifyPushFailure`（`internal/modules/workbench/notification/provider.go`）；`handler.MobileDeviceHandler` 与 `RegisterMobileDeviceRoutes`（`internal/handler/mobile_device.go`、`internal/router/routes_workbench.go:198-211`）。

---

## 文件结构总览

| 文件 | 职责 | 任务 |
|---|---|---|
| `migrations/versioned/000193_mobile_device_app.{up,down}.sql` | PostgreSQL：两表加 `app_id`（mobile_devices 进 PK + 令牌排他索引换列集；intents 换命名唯一约束） | 1 |
| `migrations/sqlite/000114_mobile_device_app.{up,down}.sql` | sqlite：两表重建（PK/UNIQUE 约束不可 ALTER） | 1 |
| `internal/application/repository/mobile_device.go` | AppID 校验/归一化、DeviceRegistration 与行结构、scoped/token 排他/ForApp 查询族 | 1 |
| `internal/application/repository/mobile_notification.go` | NotificationIntent.AppID、幂等键、Claim/Revalidate 的 app 对齐 EXISTS、投影按 App 扇出 | 1 |
| `internal/application/repository/mobile_device_app_test.go` | Task 1 测试（聚焦迁移） | 1 |
| `internal/modules/workbench/notification/apns.go` | APNs Provider（alert/background 双模式、receipt=apns-unique-id、错误分类）+ .p8 ES256 provider token 源 | 2 |
| `internal/modules/workbench/notification/apns_test.go` | Task 2 APNs 测试（httptest + 本地生成 EC 密钥） | 2 |
| `internal/modules/workbench/notification/fcm.go` | FCM HTTP v1 Provider（data-only/notification 双模式、receipt=name、错误分类）+ service account RS256 OAuth token 源 | 2 |
| `internal/modules/workbench/notification/fcm_test.go` | Task 2 FCM 测试（httptest + 本地生成 RSA 密钥） | 2 |
| `internal/modules/workbench/service/workbench/notification_delivery.go` | `PushPayloadPolicy`（盲推）、`NewHTTPNotificationProviderWithPolicy`、`DisabledNotificationProvider`、`AppRoutingNotificationProvider`、`NotificationDeviceRevoker` 升级为 `RevokeForApp` | 3 |
| `internal/modules/workbench/service/workbench/notification_app_policy_test.go` | Task 3 测试（新文件，聚焦迁移 helper） | 3 |
| `internal/config/config.go` | `MobileNotificationConfig.Payload` + `MOBILE_NOTIFICATION_PAYLOAD` env 覆盖 | 3 |
| `internal/container/container.go` | `newMobileNotificationProvider` 重写：blind/disabled/enterprise 路由 + APNs/FCM 装配 | 3 |
| `docs/architecture/integration/workbench.md` | §7 配置键清单补新键 | 3 |
| `internal/handler/mobile_device.go` | 注册/intent/presence wire 携带 `app_id`、HMAC intent 绑定 App、部署 App 策略 | 4 |
| `internal/handler/mobile_device_app_test.go` | Task 4 测试（新文件） | 4 |
| `internal/handler/mobile_device_test.go` | 仅改 `openMobileHandlerDB`：按序补执行 000059/000060/000114（000114 单独执行会因缺 intents 表失败） | 4 |
| `internal/application/repository/mobile_push_isolation_test.go` | Task 5 端到端（`package repository_test`） | 5 |
| `packages/api-client/src/mobile/devices.ts` | wire 携带 `app_id`、记录 `appId` | 6 |
| `packages/api-client/src/mobile/devices-app-id.test.ts` | Task 6 api-client 测试（新文件） | 6 |
| `packages/mobile-core/src/device/device-registry.ts` | `DeviceRemote.issueIntent/register` 可选 `appId`、`DeviceRegistry.register` 可选 `appId`、记录 `appId` | 6 |
| `packages/mobile-core/src/device/in-memory-device-remote.ts` | scenario Adapter 按 `appId:deviceId` 键控 | 6 |
| `packages/mobile-core/src/device/device-app-id.test.ts` | Task 6 mobile-core 测试（新文件） | 6 |
| `apps/mobile/src/app-id.ts` + `.test.ts` | 构建期 App 身份解析（official / enterprise:slug 白名单字符集） | 7 |
| `apps/mobile/src/foreground-sync.ts` + `.test.ts` | 前台权威同步环（单飞合并、scope 每事件重解析） | 7 |
| `apps/mobile/src/adapters/app-state.ts` | react-native AppState → 生命周期 Adapter（fail closed no-op） | 7 |
| `apps/mobile/src/composition.ts` | 注册携带 App 身份 + MobileApp 挂前台同步环 | 7 |
| `apps/mobile/src/blind-push-integration-smoke.ts` + `.test.ts` | opt-in 集成冒烟（AC1 客户端观察 + AC2 前台同步） | 7 |

不新增路由、不新增 API 路径（设备注册/通知投递端点集合不变，只扩请求字段）；`packages/contracts` 零改动（设备注册 wire 由 api-client 适配器内聚，contracts 无既有设备类型）。

---

### Task 1: 仓储与迁移——AppID 身份维度（隔离的地基）

**Files:**
- Create: `migrations/versioned/000193_mobile_device_app.up.sql`、`migrations/versioned/000193_mobile_device_app.down.sql`
- Create: `migrations/sqlite/000114_mobile_device_app.up.sql`、`migrations/sqlite/000114_mobile_device_app.down.sql`
- Modify: `internal/application/repository/mobile_device.go`
- Modify: `internal/application/repository/mobile_notification.go`
- Modify: `internal/application/repository/mobile_notification_test.go:115`（仅 `TestNotificationProjectEventDerivesDurableIdentity` 的最小表补 `app_id` 列）
- Modify: `internal/handler/mobile_device_test.go:20-27`（`openMobileHandlerDB` 按序补执行 000059/000060/000114——`mobileDeviceRow` 自带 AppID 字段后，旧 schema 库上的 INSERT 会缺列报错；000114 依赖 000059/000060 建出的 intents 表，必须三文件成组，与本任务同步）
- Test: `internal/application/repository/mobile_device_app_test.go`

**Interfaces:**
- Consumes: 既有 `MobileDeviceStore`/`NotificationStore` 全部方法（见 `mobile_device.go`/`mobile_notification.go`）。
- Produces（后续任务与生产代码依赖的精确签名）:
  - `const MobileAppIDOfficial = "official"`；`var ErrMobileDeviceInvalidApp = errors.New("invalid mobile app id")`
  - `func ValidateMobileAppID(appID string) error`（仅接受 `official` 或 `enterprise:<slug>`，slug `^[a-z0-9][a-z0-9-]{0,31}$`）
  - `func NormalizeMobileAppID(appID string) string`（空白 → `official`）
  - `repository.DeviceRegistration` 新增字段 `AppID string \`json:"app_id"\``
  - `repository.NotificationIntent` 新增字段 `AppID string`
  - `func (s *MobileDeviceStore) CurrentScopeGenerationForApp(ctx context.Context, tenant uint64, owner, device, appID string) (int64, error)`
  - `func (s *MobileDeviceStore) GetActiveForApp(ctx context.Context, tenant uint64, owner, device, appID string) (DeviceRegistration, error)`
  - `func (s *MobileDeviceStore) RevokeForApp(ctx context.Context, tenant uint64, owner, device, appID string, revision int64) error`
  - 既有 `CurrentScopeGeneration`/`GetActiveForTenant`/`RevokeForTenant`/`MarkPresence`/`GetPresence`/`SetPresence`/`DeletePresence` 保留并委托 `appID=MobileAppIDOfficial`（既有调用方零改动编译）。
  - 意图幂等 ID 格式从 `%d:%s:%s:%s:%s` 扩为 `%d:%s:%s:%s:%s:%s`（末段 AppID）。

- [ ] **Step 1: 写失败测试**

创建 `internal/application/repository/mobile_device_app_test.go`（`package repository`，聚焦迁移——不依赖损坏的全目录 `migrator.Up()`，见差异记录第 4 条）：

```go
package repository

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openMobileAppDB 只执行本域迁移子集，顺序必须保持 000058（设备表）→ 000059/000060
// （意图表原形 + 补列）→ 000114（两表重建加 App 维度）：000114 的重建段对
// mobile_devices 与 mobile_notification_intents 做 INSERT...SELECT，二者必须已存在。
// 与 openMobileHandlerDB（internal/handler/mobile_device_test.go:20）同一聚焦模式：
// 当前 HEAD 的全目录 migrator.Up() 因 #42/#59 同号 000112 双文件损坏，不可依赖。
func openMobileAppDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "mobile-app.db")+"?_busy_timeout=5000"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	for _, file := range []string{
		"migrations/sqlite/000058_mobile_devices.up.sql",
		"migrations/sqlite/000059_mobile_notifications.up.sql",
		"migrations/sqlite/000060_mobile_notification_delivery.up.sql",
		"migrations/sqlite/000114_mobile_device_app.up.sql",
	} {
		up, err := readFileContents(filepath.Join(root, file))
		require.NoError(t, err, file)
		require.NoError(t, db.Exec(up).Error, file)
	}
	// Claim/Revalidate/投影谓词触及的最小 agent_runs/agent_run_events 形状（仅本测试域）。
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, owner_id TEXT NOT NULL,
		PRIMARY KEY (tenant_id, run_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE agent_run_events (
		tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, seq INTEGER NOT NULL,
		attempt_id TEXT NOT NULL DEFAULT '', event_type TEXT NOT NULL, payload TEXT NOT NULL DEFAULT '{}',
		PRIMARY KEY (tenant_id, run_id, seq))`).Error)
	require.NoError(t, db.Exec("INSERT INTO agent_runs (tenant_id, run_id, owner_id) VALUES (1, 'r1', 'u1')").Error)
	require.NoError(t, db.Exec("INSERT INTO agent_run_events (tenant_id, run_id, seq, event_type, payload) VALUES (1, 'r1', 9, 'run_completed', '{}')").Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func bindAppRegistration(t *testing.T, db *gorm.DB, device, token, appID string) DeviceRegistration {
	t.Helper()
	store := NewMobileDeviceStore(db, "dev")
	in := DeviceRegistration{
		TenantID: 1, OwnerID: "u1", DeviceID: device, Environment: "dev", Platform: "ios",
		TokenCiphertext: "cipher-" + token, TokenHash: DeviceTokenHash(token),
		Revision: 0, ScopeGeneration: 1, AppID: appID,
	}
	require.NoError(t, store.Bind(context.Background(), in))
	row, err := store.GetActiveForApp(context.Background(), 1, "u1", device, NormalizeMobileAppID(appID))
	require.NoError(t, err)
	return row
}

func TestValidateMobileAppID(t *testing.T) {
	require.NoError(t, ValidateMobileAppID("official"))
	require.NoError(t, ValidateMobileAppID("enterprise:acme"))
	require.NoError(t, ValidateMobileAppID("enterprise:a1-b2"))
	for _, invalid := range []string{"Official", "enterprise", "enterprise:", "enterprise:Acme", "enterprise:a_b", "enterprise:-ab", "enterprise:" + strings.Repeat("x", 33), "ios", "weknora"} {
		require.Error(t, ValidateMobileAppID(invalid), "%q must be invalid", invalid)
	}
	require.Equal(t, "official", NormalizeMobileAppID(""))
	require.Equal(t, "official", NormalizeMobileAppID("  "))
	require.Equal(t, "enterprise:acme", NormalizeMobileAppID(" enterprise:acme "))
}

func TestBindIsolatesOfficialAndEnterprise(t *testing.T) {
	db := openMobileAppDB(t)
	official := bindAppRegistration(t, db, "shared-device", "tok-official", "official")
	enterprise := bindAppRegistration(t, db, "shared-device", "tok-enterprise", "enterprise:acme")
	// 同一物理设备双 App：两行独立，revision 各自从 1 起，互不覆盖。
	require.Equal(t, "official", official.AppID)
	require.Equal(t, "enterprise:acme", enterprise.AppID)
	require.EqualValues(t, 1, official.Revision)
	require.EqualValues(t, 1, enterprise.Revision)
	// 官方行再次注册（token 轮换）只动官方行。
	rotated := bindAppRegistration(t, db, "shared-device", "tok-official-2", "official")
	require.EqualValues(t, 2, rotated.Revision)
	stillEnterprise, err := NewMobileDeviceStore(db, "dev").GetActiveForApp(context.Background(), 1, "u1", "shared-device", "enterprise:acme")
	require.NoError(t, err)
	require.EqualValues(t, 1, stillEnterprise.Revision, "official rebind must not touch the enterprise row")
}

func TestTokenExclusivityIsPerApp(t *testing.T) {
	db := openMobileAppDB(t)
	bindAppRegistration(t, db, "d-official", "same-token", "official")
	// 同一 token 注册到企业 App 的另一设备：跨 App 不互踢，两行都 active。
	bindAppRegistration(t, db, "d-enterprise", "same-token", "enterprise:acme")
	store := NewMobileDeviceStore(db, "dev")
	_, err := store.GetActiveForApp(context.Background(), 1, "u1", "d-official", "official")
	require.NoError(t, err, "cross-app registration must not revoke the official binding")
	// 同 App 内维持既有排他：官方 App 第二台设备绑定同 token，第一台被撤销。
	bindAppRegistration(t, db, "d-official-2", "same-token", "official")
	_, err = store.GetActiveForApp(context.Background(), 1, "u1", "d-official", "official")
	require.ErrorIs(t, err, ErrMobileDeviceNotFound, "same-app token takeover keeps exclusivity")
}

func TestNotificationIntentFanOutPerApp(t *testing.T) {
	db := openMobileAppDB(t)
	bindAppRegistration(t, db, "d1", "tok-a", "official")
	bindAppRegistration(t, db, "d2", "tok-b", "enterprise:acme")
	store := NewNotificationStore(db)
	require.NoError(t, store.ProjectEvent(context.Background(), RunNotificationEvent{
		TenantID: 1, OwnerID: "u1", RunID: "r1", Seq: 9, Type: "run_completed",
	}))
	var intents []struct {
		DeviceID string
		AppID    string
		State    string
	}
	require.NoError(t, db.Table("mobile_notification_intents").Select("device_id, app_id, state").Order("app_id").Find(&intents).Error)
	require.Len(t, intents, 2, "one intent per (device, app) registration")
	// Order("app_id")：'enterprise:acme' < 'official'（字典序）。
	require.Equal(t, "d2", intents[0].DeviceID)
	require.Equal(t, "enterprise:acme", intents[0].AppID)
	require.Equal(t, "d1", intents[1].DeviceID)
	require.Equal(t, "official", intents[1].AppID)
	deliveries, err := store.Claim(context.Background(), "worker-app", 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, deliveries, 2)
	apps := map[string]string{}
	for _, d := range deliveries {
		apps[d.Intent.DeviceID] = d.Intent.AppID
	}
	require.Equal(t, "official", apps["d1"])
	require.Equal(t, "enterprise:acme", apps["d2"])
}

func TestClaimJoinsAppIDSoRevokedAppDoesNotResurrect(t *testing.T) {
	db := openMobileAppDB(t)
	// 关键布局：官方与企业**同一 device_id** 各一行——若 Claim/Revalidate 的设备 EXISTS
	// 缺 app 对齐，未被撤销的官方行会让已撤销的企业投递复活。
	bindAppRegistration(t, db, "shared-x", "tok-official", "official")
	enterprise := bindAppRegistration(t, db, "shared-x", "tok-enterprise", "enterprise:acme")
	require.NoError(t, NewMobileDeviceStore(db, "dev").RevokeForApp(context.Background(), 1, "u1", "shared-x", "enterprise:acme", enterprise.Revision))
	store := NewNotificationStore(db)
	require.NoError(t, store.Enqueue(context.Background(), NotificationIntent{
		TenantID: 1, EventID: "1:r1:9", OwnerID: "u1", DeviceID: "shared-x", Environment: "dev",
		AppID: "enterprise:acme", Kind: "completed", RunID: "r1", ExpiresAt: time.Now().Add(time.Hour),
	}))
	deliveries, err := store.Claim(context.Background(), "worker-app", 10, time.Minute)
	require.NoError(t, err)
	require.Empty(t, deliveries, "a revoked enterprise registration must not claim even though an official row shares the device id")

	// RevalidateDelivery 的设备 EXISTS 同样按 app 对齐：把该行手工置为 in_flight 后，
	// 最终授权必须仍拒绝（官方行救不活企业投递）。
	var id string
	require.NoError(t, db.Raw("SELECT id FROM mobile_notification_intents LIMIT 1").Scan(&id).Error)
	require.NoError(t, db.Exec("UPDATE mobile_notification_intents SET state = 'in_flight', lease_owner = 'worker-app', fence = 1 WHERE id = ?", id).Error)
	require.False(t, store.RevalidateDelivery(context.Background(), NotificationDelivery{
		ID: id, Fence: 1,
		Intent: NotificationIntent{TenantID: 1, EventID: "1:r1:9", OwnerID: "u1", DeviceID: "shared-x", Environment: "dev", AppID: "enterprise:acme", Kind: "completed", RunID: "r1"},
	}, "worker-app"), "the final authorization seam must join on app_id, not just the device id")
}
```

同文件补一个小读取 helper（避免引入 `os` 重复样板）：

```go
func readFileContents(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/repository/ -run 'TestValidateMobileAppID|TestBindIsolates|TestTokenExclusivityIsPerApp|TestNotificationIntentFanOutPerApp|TestClaimJoinsAppID' -count=1`
Expected: FAIL——编译错误 `unknown field AppID` / `undefined: ValidateMobileAppID` / `GetActiveForApp`（实现尚未存在），以及迁移文件不存在导致的 `openMobileAppDB` 读文件失败。

- [ ] **Step 3: 最小实现（迁移 + 仓储）**

创建 `migrations/sqlite/000114_mobile_device_app.up.sql`：

```sql
-- T37 (#67): official 与 enterprise 自构建 App 的注册与令牌永不混用。app_id 进入
-- mobile_devices 主键（同一物理设备双 App 为两行独立）与令牌排他索引（跨 App 不互相
-- 接管）；mobile_notification_intents 的幂等身份含 app_id（投递按 App 路由 Provider）。
-- sqlite 无法 ALTER 主键/表级 UNIQUE，按 000055 同例整表重建。存量行归 official。
-- 注意：升级时仍 pending 的旧 5 段幂等 ID 行在重投影后会以 6 段新 ID 再入队一次
--（at-least-once 语义容忍，窗口仅升级瞬间的 pending 行）。
CREATE TABLE mobile_devices_rebuilt (
    tenant_id INTEGER NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    device_id VARCHAR(128) NOT NULL,
    environment VARCHAR(32) NOT NULL,
    app_id VARCHAR(64) NOT NULL DEFAULT 'official',
    space_id VARCHAR(128) NOT NULL DEFAULT '',
    platform VARCHAR(16) NOT NULL,
    token_ciphertext TEXT NOT NULL,
    token_hash VARCHAR(64) NOT NULL,
    revision INTEGER NOT NULL DEFAULT 1,
    scope_generation INTEGER NOT NULL DEFAULT 0,
    revoked_at DATETIME,
    last_seen_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, owner_id, device_id, environment, app_id),
    CHECK (platform IN ('ios', 'android')),
    CHECK (revision > 0),
    CHECK (scope_generation >= 0)
);
INSERT INTO mobile_devices_rebuilt
    (tenant_id, owner_id, device_id, environment, app_id, space_id, platform, token_ciphertext,
     token_hash, revision, scope_generation, revoked_at, last_seen_at, created_at, updated_at)
SELECT tenant_id, owner_id, device_id, environment, 'official', space_id, platform, token_ciphertext,
       token_hash, revision, scope_generation, revoked_at, last_seen_at, created_at, updated_at
FROM mobile_devices;
DROP TABLE mobile_devices;
ALTER TABLE mobile_devices_rebuilt RENAME TO mobile_devices;
CREATE UNIQUE INDEX uq_mobile_devices_active_token
    ON mobile_devices (environment, app_id, token_hash)
    WHERE revoked_at IS NULL;
CREATE INDEX idx_mobile_devices_owner
    ON mobile_devices (tenant_id, owner_id, environment, revoked_at, updated_at);

CREATE TABLE mobile_notification_intents_rebuilt (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    event_id TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    environment TEXT NOT NULL,
    app_id TEXT NOT NULL DEFAULT 'official',
    kind TEXT NOT NULL,
    run_id TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    state TEXT NOT NULL DEFAULT 'pending',
    attempt INTEGER NOT NULL DEFAULT 0,
    lease_owner TEXT NOT NULL DEFAULT '',
    lease_until DATETIME,
    next_attempt_at DATETIME,
    fence INTEGER NOT NULL DEFAULT 0,
    receipt_id VARCHAR(256) NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, event_id, owner_id, device_id, environment, app_id),
    CHECK (state IN ('pending', 'in_flight', 'sent', 'expired')),
    CHECK (attempt >= 0),
    CHECK (fence >= 0)
);
INSERT INTO mobile_notification_intents_rebuilt
    (id, tenant_id, event_id, owner_id, device_id, environment, app_id, kind, run_id, expires_at,
     state, attempt, lease_owner, lease_until, next_attempt_at, fence, receipt_id, last_error,
     created_at, updated_at)
SELECT id, tenant_id, event_id, owner_id, device_id, environment, 'official', kind, run_id, expires_at,
       state, attempt, lease_owner, lease_until, next_attempt_at, fence, receipt_id, last_error,
       created_at, updated_at
FROM mobile_notification_intents;
DROP TABLE mobile_notification_intents;
ALTER TABLE mobile_notification_intents_rebuilt RENAME TO mobile_notification_intents;
CREATE INDEX idx_mobile_notification_claim ON mobile_notification_intents (state, lease_until, expires_at, created_at);
CREATE INDEX idx_mobile_notification_owner ON mobile_notification_intents (tenant_id, owner_id, run_id, created_at);
CREATE INDEX idx_mobile_notification_next_attempt ON mobile_notification_intents (state, next_attempt_at, expires_at, created_at);
```

创建 `migrations/sqlite/000114_mobile_device_app.down.sql`：

```sql
-- 回滚：跨 App 数据无法双保留，确定性丢弃企业行后重建无 app_id 的原表（000058/000059+000060 形状）。
DELETE FROM mobile_notification_intents WHERE app_id <> 'official';
CREATE TABLE mobile_notification_intents_rebuilt (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    event_id TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    environment TEXT NOT NULL,
    kind TEXT NOT NULL,
    run_id TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    state TEXT NOT NULL DEFAULT 'pending',
    attempt INTEGER NOT NULL DEFAULT 0,
    lease_owner TEXT NOT NULL DEFAULT '',
    lease_until DATETIME,
    next_attempt_at DATETIME,
    fence INTEGER NOT NULL DEFAULT 0,
    receipt_id VARCHAR(256) NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, event_id, owner_id, device_id, environment),
    CHECK (state IN ('pending', 'in_flight', 'sent', 'expired')),
    CHECK (attempt >= 0),
    CHECK (fence >= 0)
);
INSERT INTO mobile_notification_intents_rebuilt
    (id, tenant_id, event_id, owner_id, device_id, environment, kind, run_id, expires_at,
     state, attempt, lease_owner, lease_until, next_attempt_at, fence, receipt_id, last_error,
     created_at, updated_at)
SELECT id, tenant_id, event_id, owner_id, device_id, environment, kind, run_id, expires_at,
       state, attempt, lease_owner, lease_until, next_attempt_at, fence, receipt_id, last_error,
       created_at, updated_at
FROM mobile_notification_intents;
DROP TABLE mobile_notification_intents;
ALTER TABLE mobile_notification_intents_rebuilt RENAME TO mobile_notification_intents;
CREATE INDEX idx_mobile_notification_claim ON mobile_notification_intents (state, lease_until, expires_at, created_at);
CREATE INDEX idx_mobile_notification_owner ON mobile_notification_intents (tenant_id, owner_id, run_id, created_at);
CREATE INDEX idx_mobile_notification_next_attempt ON mobile_notification_intents (state, next_attempt_at, expires_at, created_at);

DELETE FROM mobile_devices WHERE app_id <> 'official';
CREATE TABLE mobile_devices_rebuilt (
    tenant_id INTEGER NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    device_id VARCHAR(128) NOT NULL,
    environment VARCHAR(32) NOT NULL,
    space_id VARCHAR(128) NOT NULL DEFAULT '',
    platform VARCHAR(16) NOT NULL,
    token_ciphertext TEXT NOT NULL,
    token_hash VARCHAR(64) NOT NULL,
    revision INTEGER NOT NULL DEFAULT 1,
    scope_generation INTEGER NOT NULL DEFAULT 0,
    revoked_at DATETIME,
    last_seen_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, owner_id, device_id, environment),
    CHECK (platform IN ('ios', 'android')),
    CHECK (revision > 0),
    CHECK (scope_generation >= 0)
);
INSERT INTO mobile_devices_rebuilt
    (tenant_id, owner_id, device_id, environment, space_id, platform, token_ciphertext,
     token_hash, revision, scope_generation, revoked_at, last_seen_at, created_at, updated_at)
SELECT tenant_id, owner_id, device_id, environment, space_id, platform, token_ciphertext,
       token_hash, revision, scope_generation, revoked_at, last_seen_at, created_at, updated_at
FROM mobile_devices;
DROP TABLE mobile_devices;
ALTER TABLE mobile_devices_rebuilt RENAME TO mobile_devices;
CREATE UNIQUE INDEX uq_mobile_devices_active_token
    ON mobile_devices (environment, token_hash)
    WHERE revoked_at IS NULL;
CREATE INDEX idx_mobile_devices_owner
    ON mobile_devices (tenant_id, owner_id, environment, revoked_at, updated_at);
```

创建 `migrations/versioned/000193_mobile_device_app.up.sql`（PostgreSQL，命名约束可 ALTER，无需重建）：

```sql
-- T37 (#67): official 与 enterprise 自构建 App 的注册与令牌永不混用（同 sqlite 000114
-- 语义；PostgreSQL 具名约束直接 ALTER）。存量行归 official；升级瞬间仍 pending 的旧
-- 5 段幂等 ID 行在重投影后以 6 段新 ID 再入队一次（at-least-once 容忍）。
ALTER TABLE mobile_devices ADD COLUMN app_id VARCHAR(64) NOT NULL DEFAULT 'official';
ALTER TABLE mobile_devices DROP CONSTRAINT mobile_devices_pkey;
ALTER TABLE mobile_devices ADD PRIMARY KEY (tenant_id, owner_id, device_id, environment, app_id);
DROP INDEX IF EXISTS uq_mobile_devices_active_token;
CREATE UNIQUE INDEX uq_mobile_devices_active_token
    ON mobile_devices (environment, app_id, token_hash)
    WHERE revoked_at IS NULL;

ALTER TABLE mobile_notification_intents ADD COLUMN app_id VARCHAR(64) NOT NULL DEFAULT 'official';
ALTER TABLE mobile_notification_intents DROP CONSTRAINT uq_mobile_notification_identity;
ALTER TABLE mobile_notification_intents
    ADD CONSTRAINT uq_mobile_notification_identity
    UNIQUE (tenant_id, event_id, owner_id, device_id, environment, app_id);
```

创建 `migrations/versioned/000193_mobile_device_app.down.sql`：

```sql
ALTER TABLE mobile_notification_intents DROP CONSTRAINT uq_mobile_notification_identity;
ALTER TABLE mobile_notification_intents
    ADD CONSTRAINT uq_mobile_notification_identity
    UNIQUE (tenant_id, event_id, owner_id, device_id, environment);
ALTER TABLE mobile_notification_intents DROP COLUMN app_id;

DROP INDEX uq_mobile_devices_active_token;
ALTER TABLE mobile_devices DROP CONSTRAINT mobile_devices_pkey;
ALTER TABLE mobile_devices ADD PRIMARY KEY (tenant_id, owner_id, device_id, environment);
ALTER TABLE mobile_devices DROP COLUMN app_id;
CREATE UNIQUE INDEX uq_mobile_devices_active_token
    ON mobile_devices (environment, token_hash)
    WHERE revoked_at IS NULL;
```

修改 `internal/application/repository/mobile_device.go`：

(a) 顶部 var 块后新增 App 身份校验（放在 `ErrMobileDeviceInvalid` 之后）：

```go
// MobileAppIDOfficial is the standard client app identity. Enterprise
// self-built builds register under "enterprise:<slug>" declared by the
// deployment (MOBILE_ENTERPRISE_APP_ID); registrations under any other app
// id are rejected at the HTTP boundary.
const MobileAppIDOfficial = "official"

var ErrMobileDeviceInvalidApp = errors.New("invalid mobile app id")

// ValidateMobileAppID accepts exactly "official" or "enterprise:<slug>"
// where slug is 1-32 chars of [a-z0-9-] and must not start with '-'.
func ValidateMobileAppID(appID string) error {
	if appID == MobileAppIDOfficial {
		return nil
	}
	slug, ok := strings.CutPrefix(appID, "enterprise:")
	if !ok || len(slug) < 1 || len(slug) > 32 {
		return ErrMobileDeviceInvalidApp
	}
	for i := 0; i < len(slug); i++ {
		c := slug[i]
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			continue
		}
		if c == '-' && i > 0 {
			continue
		}
		return ErrMobileDeviceInvalidApp
	}
	return nil
}

// NormalizeMobileAppID maps absent input to the official app. It does NOT
// make invalid values valid: callers still run ValidateMobileAppID.
func NormalizeMobileAppID(appID string) string {
	trimmed := strings.TrimSpace(appID)
	if trimmed == "" {
		return MobileAppIDOfficial
	}
	return trimmed
}
```

(b) `DeviceRegistration`（`mobile_device.go:33-46`）`Environment` 字段后加：

```go
	AppID           string     `json:"app_id"`
```

(c) `mobileDeviceRow`（`mobile_device.go:48-63`）`Environment` 字段后加：

```go
	AppID           string     `gorm:"primaryKey;type:varchar(64)"`
```

`registration()`（`mobile_device.go:67-75`）映射加 `AppID: r.AppID`。

(d) `scoped()`（`mobile_device.go:121-127`）替换为带 App 维度：

```go
func (s *MobileDeviceStore) scoped(db *gorm.DB, tenantID uint64, ownerID, deviceID, appID string) *gorm.DB {
	q := db.Where("environment = ? AND owner_id = ? AND device_id = ? AND app_id = ?", s.environment, ownerID, deviceID, NormalizeMobileAppID(appID))
	if tenantID != 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	return q
}
```

(e) `validate()`（`mobile_device.go:108-119`）在 `in.Environment != s.environment` 判定后加一行 App 校验（放在 platform 校验前）：

```go
	if err := ValidateMobileAppID(NormalizeMobileAppID(in.AppID)); err != nil {
		return err
	}
```

(f) `Bind()`（`mobile_device.go:132-155`）开头（`TokenHash` 回填之后、`validate` 之前）加归一化：

```go
	in.AppID = NormalizeMobileAppID(in.AppID)
```

(g) `bindOnce()`（`mobile_device.go:157-245`）四处改动：
- `s.scoped(tx, in.TenantID, in.OwnerID, in.DeviceID)` → `s.scoped(tx, in.TenantID, in.OwnerID, in.DeviceID, in.AppID)`；
- 令牌排他查询（`mobile_device.go:199`）加 app 条件：

```go
		if err := tx.Where("environment = ? AND app_id = ? AND token_hash = ? AND revoked_at IS NULL", s.environment, in.AppID, in.TokenHash).
			Clauses(clause.Locking{Strength: "UPDATE"}).Find(&sameToken).Error; err != nil {
```

- 同 token 行撤销的 UPDATE WHERE（`mobile_device.go:208-209`）加 `AND app_id = ?`，参数表末尾补 `in.AppID`（行内参数顺序对齐 WHERE 列顺序：`row.TenantID, row.OwnerID, row.DeviceID, s.environment, row.AppID, row.Revision`——`mobileDeviceRow` 已含 AppID，`row.AppID` 直接可用）；
- 既有行 PK UPDATE WHERE（`mobile_device.go:220-221`）加 `AND app_id = ?` 与参数 `existing.AppID`；`Updates` 的 map 不含 `app_id`（PK 不变更）；
- 新建行（`mobile_device.go:239-242`）结构体字面量加 `AppID: in.AppID`。

(h) 既有读/撤销方法改走 app 维度并保留旧签名委托（`GetActiveForTenant`、`Revoke`、`RevokeForTenant`、`revoke`、`MarkPresence`、`GetPresence`、`SetPresence`、`DeletePresence`、`CurrentScopeGeneration` 逐个改写为：内部调用对应 ForApp 变体并传 `MobileAppIDOfficial`）。新增方法（放在既有方法旁）：

```go
func (s *MobileDeviceStore) CurrentScopeGenerationForApp(ctx context.Context, tenant uint64, owner, device, appID string) (int64, error) {
	if s == nil || s.db == nil || tenant == 0 || strings.TrimSpace(owner) == "" || strings.TrimSpace(device) == "" {
		return 0, ErrMobileDeviceInvalid
	}
	if err := ValidateMobileAppID(NormalizeMobileAppID(appID)); err != nil {
		return 0, err
	}
	var row mobileDeviceRow
	err := s.scoped(s.db.WithContext(ctx), tenant, owner, device, appID).Order("scope_generation DESC").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return row.ScopeGeneration, nil
}

func (s *MobileDeviceStore) GetActiveForApp(ctx context.Context, tenant uint64, owner, device, appID string) (DeviceRegistration, error) {
	if tenant == 0 || strings.TrimSpace(owner) == "" || strings.TrimSpace(device) == "" {
		return DeviceRegistration{}, ErrMobileDeviceInvalid
	}
	if err := ValidateMobileAppID(NormalizeMobileAppID(appID)); err != nil {
		return DeviceRegistration{}, err
	}
	var row mobileDeviceRow
	err := s.scoped(s.db.WithContext(ctx), tenant, owner, device, appID).Where("revoked_at IS NULL").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DeviceRegistration{}, ErrMobileDeviceNotFound
	}
	if err != nil {
		return DeviceRegistration{}, err
	}
	return row.registration(), nil
}

func (s *MobileDeviceStore) RevokeForApp(ctx context.Context, tenant uint64, owner, device, appID string, revision int64) error {
	return s.revokeForApp(ctx, tenant, owner, device, appID, revision)
}

func (s *MobileDeviceStore) revokeForApp(ctx context.Context, tenant uint64, owner, device, appID string, revision int64) error {
	if s == nil || s.db == nil || s.environment == "" || strings.TrimSpace(owner) == "" || strings.TrimSpace(device) == "" {
		return ErrMobileDeviceInvalid
	}
	if err := ValidateMobileAppID(NormalizeMobileAppID(appID)); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row mobileDeviceRow
		q := s.scoped(tx, tenant, owner, device, appID).Clauses(clause.Locking{Strength: "UPDATE"})
		if err := q.Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMobileDeviceNotFound
			}
			return err
		}
		if row.RevokedAt != nil {
			if revision > 0 && revision < row.Revision {
				return ErrMobileDeviceRevision
			}
			return nil
		}
		if revision > 0 && revision != row.Revision {
			return ErrMobileDeviceRevision
		}
		now := time.Now().UTC()
		if updated := s.scoped(tx.Model(&mobileDeviceRow{}), tenant, owner, device, appID).Where("revision = ? AND revoked_at IS NULL", row.Revision).
			Updates(map[string]any{"revoked_at": now, "revision": row.Revision + 1, "scope_generation": row.ScopeGeneration + 1, "updated_at": now}); updated.Error != nil {
			return updated.Error
		} else if updated.RowsAffected != 1 {
			return ErrMobileDeviceRevision
		}
		return nil
	})
}
```

既有 `revoke(ctx, tenant, owner, device, revision)` 改为 `return s.revokeForApp(ctx, tenant, owner, device, MobileAppIDOfficial, revision)`；presence 四方法同理保留旧签名、内部以 `appID=MobileAppIDOfficial` 委托新的 ForApp 变体。以 `MarkPresenceForApp` 为例（其余 `GetPresenceForApp`/`SetPresenceForApp`/`DeletePresenceForApp` 与各自原实现逐行相同，仅 WHERE 追加 `AND app_id = ?`、参数 `NormalizeMobileAppID(appID)`，并在入口加 `ValidateMobileAppID` 校验）：

```go
func (s *MobileDeviceStore) MarkPresenceForApp(ctx context.Context, tenant uint64, owner, device, appID string, revision int64) error {
	if tenant == 0 || strings.TrimSpace(owner) == "" || strings.TrimSpace(device) == "" {
		return ErrMobileDeviceInvalid
	}
	if err := ValidateMobileAppID(NormalizeMobileAppID(appID)); err != nil {
		return err
	}
	appID = NormalizeMobileAppID(appID)
	q := s.db.WithContext(ctx).Table("mobile_devices").Where("tenant_id = ? AND environment = ? AND owner_id = ? AND device_id = ? AND app_id = ? AND revoked_at IS NULL", tenant, s.environment, owner, device, appID)
	if revision > 0 {
		var current mobileDeviceRow
		if err := q.Take(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMobileDeviceNotFound
			}
			return err
		}
		if current.Revision != revision {
			return ErrMobileDeviceRevision
		}
	}
	now := time.Now().UTC()
	result := s.db.WithContext(ctx).Exec("UPDATE mobile_devices SET last_seen_at = ?, updated_at = ? WHERE tenant_id = ? AND environment = ? AND owner_id = ? AND device_id = ? AND app_id = ? AND revoked_at IS NULL", now, now, tenant, s.environment, owner, device, appID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrMobileDeviceNotFound
	}
	return nil
}
```

`GetPresenceForApp`/`SetPresenceForApp`/`DeletePresenceForApp` 分别 = 原 `GetPresence`/`SetPresence`/`DeletePresence` 函数体，其中对 store 的调用换为对应 `...ForApp`/`GetActiveForApp` 并按上文加 app 参数；`RevokeBeforeScopeGeneration` **不加** app 过滤（scope epoch 属 owner：切账号/空间时该 owner 全部 App 的设备一并撤销——有意语义，函数注释补一句说明）。`ListActiveForTenant`/`list` 不变（返回行已携带 `AppID` 字段）。

修改 `internal/application/repository/mobile_notification.go`：

(a) `NotificationIntent`（`mobile_notification.go:20-29`）`Environment` 后加 `AppID string`；`mobileNotificationRow`（`mobile_notification.go:45-65`）`Environment` 后加 `AppID string \`gorm:"column:app_id"\``。

(b) `validateNotificationIntent`（`mobile_notification.go:163-174`）改为先归一化再校验：

```go
func validateNotificationIntent(in *NotificationIntent) error {
	in.AppID = NormalizeMobileAppID(in.AppID)
	if err := ValidateMobileAppID(in.AppID); err != nil {
		return err
	}
	if in.EventID == "" || in.OwnerID == "" || in.DeviceID == "" || in.Environment == "" ||
		in.Kind == "" || in.RunID == "" || in.ExpiresAt.IsZero() {
		return errors.New("invalid_notification_intent")
	}
	switch in.Kind {
	case "completed", "failed", "interaction_requested", "budget_exhausted":
	default:
		return errors.New("invalid_notification_kind")
	}
	return nil
}
```

(c) `notificationID`（`mobile_notification.go:176-180`）改为：

```go
func notificationID(in NotificationIntent) string {
	// The unique constraint is authoritative. A deterministic ID makes logs,
	// retries and support tooling refer to the same intent without token data.
	// AppID is part of the identity: one event fans out to one intent per app.
	return fmt.Sprintf("%d:%s:%s:%s:%s:%s", in.TenantID, in.EventID, in.OwnerID, in.DeviceID, in.Environment, in.AppID)
}
```

(d) `Enqueue`（`mobile_notification.go:184-210`）的 `row := mobileNotificationRow{...}` 字面量加 `AppID: in.AppID`（`validateNotificationIntent` 已归一化 `in`——注意 `validate` 接收的是值拷贝，归一化结果需带回：把 `validateNotificationIntent` 的调用改为 `in.AppID = NormalizeMobileAppID(in.AppID); if err := validateNotificationIntent(in); err != nil`，或直接在 Enqueue/enqueueNotificationTx 开头先 `in.AppID = NormalizeMobileAppID(in.AppID)`。采用后者，两处入口各加一行）。

(e) `Claim`（`mobile_notification.go:215-273`）：候选行 EXISTS 的设备子查询（`mobile_notification.go:228-233`）加 app 对齐：

```go
			EXISTS (SELECT 1 FROM mobile_devices d
				WHERE d.tenant_id = mobile_notification_intents.tenant_id
				  AND d.owner_id = mobile_notification_intents.owner_id
				  AND d.device_id = mobile_notification_intents.device_id
				  AND d.environment = mobile_notification_intents.environment
				  AND d.app_id = mobile_notification_intents.app_id
				  AND d.revoked_at IS NULL) AND
```

构造 `NotificationDelivery` 的 `Intent`（`mobile_notification.go:266`）加 `AppID: candidate.AppID`。

(f) `activeDeviceRevision`（`mobile_notification.go:275-297`）签名加 `appID string` 参数，WHERE 追加 `AND app_id = ?`（参数 `NormalizeMobileAppID(appID)`）；`Claim` 内调用点改为 `s.activeDeviceRevision(tx, candidate.TenantID, candidate.OwnerID, candidate.DeviceID, candidate.Environment, candidate.AppID)`。

(g) `RevalidateDelivery`（`mobile_notification.go:406-429`）的 `md` EXISTS 子查询同样追加 `AND md.app_id = mobile_notification_intents.app_id`。

(h) `projectEventTx`（`mobile_notification.go:515-560`）设备查询改选 app_id：

```go
	var devices []struct {
		DeviceID    string
		Environment string
		AppID       string
	}
	if err := tx.Table("mobile_devices").Select("device_id, environment, app_id").
		Where("tenant_id = ? AND owner_id = ? AND revoked_at IS NULL", evt.TenantID, run.OwnerID).Find(&devices).Error; err != nil {
		return err
	}
	for _, device := range devices {
		in := NotificationIntent{
			TenantID: evt.TenantID, EventID: eventIdentity(evt.TenantID, evt.RunID, evt.Seq),
			OwnerID: run.OwnerID, DeviceID: device.DeviceID, Environment: device.Environment,
			AppID: device.AppID, Kind: kind, RunID: evt.RunID, ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
		}
```

（`enqueueNotificationTx`（`mobile_notification.go:562-572`）开头加 `in.AppID = NormalizeMobileAppID(in.AppID)`，`row` 字面量加 `AppID: in.AppID`。）

(i) 修 `internal/handler/mobile_device_test.go:20-27`：`openMobileHandlerDB` 在执行 `000058_mobile_devices.up.sql` 之后**按序补齐 000059 → 000060 → 000114**（`mobileDeviceRow` 自带 AppID 字段后，只建旧 schema 的库会让 GORM INSERT 报 `no such column: app_id`；而 000114 的 intents 重建段 `INSERT...SELECT FROM mobile_notification_intents` 要求该表已存在——单追加 000114 会 `no such table`，故必须三文件成组、顺序固定）：

```go
	for _, file := range []string{
		"000059_mobile_notifications.up.sql",
		"000060_mobile_notification_delivery.up.sql",
		"000114_mobile_device_app.up.sql",
	} {
		upNext, err := os.ReadFile(filepath.Join("..", "..", "migrations", "sqlite", file))
		require.NoError(t, err)
		require.NoError(t, db.Exec(string(upNext)).Error)
	}
```

（该 helper 原有 000058 执行保持不变；意图表在此库中建出但 handler 测试不触它们，无副作用。）

（该文件已 import `os`/`path/filepath`（`mobile_device_test.go:9-11`），无新增 import。）

(j) 修 `internal/application/repository/mobile_notification_test.go:115`：`TestNotificationProjectEventDerivesDurableIdentity` 自建最小 `mobile_devices` 表的 CREATE TABLE 追加 `app_id TEXT NOT NULL DEFAULT 'official',`（`environment` 列之后）；INSERT 列表不变（默认值生效）。**如实注记**：该测试在 HEAD 本就 FAIL（全目录迁移因 #42/#59 同号 000112 损坏），且其自建最小 `mobile_devices` 表（:120）缺 `platform` 列而 ：122 的 INSERT 引用了 platform——编排层修复同号迁移后，该测试仍会因 `no such column: platform` 失败（**先在问题，非本计划引入**；platform 列修复属该测试自身维护，不由本计划顺带承担）。本计划此处只保证「app_id 列」这一维度不因本计划再挂；**该测试不得被用作本计划的回归证据**，回归证据以 Task 1 新建测试与本计划 testCommand 为准。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/application/repository/ -run 'TestValidateMobileAppID|TestBindIsolates|TestTokenExclusivityIsPerApp|TestNotificationIntentFanOutPerApp|TestClaimJoinsAppID' -count=1`
Expected: PASS（5 个测试）。
再跑既有聚焦迁移用例确认未回归：`go test ./internal/handler/ -run TestMobileDevice -count=1` → PASS（Step 3 (i) 已让该测试库补建 app_id 列）。

- [ ] **Step 5: Commit**

```bash
git add migrations/versioned/000193_mobile_device_app.up.sql migrations/versioned/000193_mobile_device_app.down.sql migrations/sqlite/000114_mobile_device_app.up.sql migrations/sqlite/000114_mobile_device_app.down.sql internal/application/repository/mobile_device.go internal/application/repository/mobile_notification.go internal/application/repository/mobile_device_app_test.go internal/application/repository/mobile_notification_test.go internal/handler/mobile_device_test.go
git commit -m "feat(mobile-push): app identity dimension on device registrations and push intents (T37 #67 task 1)"
```

---

### Task 2: notification 包——APNs 与 FCM Provider（企业自构建 App 的独立投递通道）

**Files:**
- Create: `internal/modules/workbench/notification/apns.go`
- Create: `internal/modules/workbench/notification/fcm.go`
- Test: `internal/modules/workbench/notification/apns_test.go`
- Test: `internal/modules/workbench/notification/fcm_test.go`

**Interfaces:**
- Consumes: `pushnotification.PushProvider`（`Send(context.Context, string, PushPayload) (PushReceipt, error)`）、`PushPayload{Title, Body, RunID, EventID}`、`ProviderError`、`ClassifyPushFailure`、`ParseRetryAfter`（`provider.go`/`expo.go`）。
- Produces（Task 3/5 依赖）:
  - `type ApnsTokenSource interface { Token(ctx context.Context) (string, error) }`
  - `func NewStaticApnsTokenSource(token string) ApnsTokenSource`
  - `func NewApnsP8TokenSource(keyPath, keyID, teamID string) (*ApnsP8TokenSource, error)`（`.Token(ctx)` 产 ES256 JWT，缓存 ≤50 分钟）
  - `func NewApnsProvider(endpoint, topic string, tokens ApnsTokenSource) *ApnsProvider`
  - `func NewApnsProviderWithClient(endpoint, topic string, tokens ApnsTokenSource, client *http.Client) *ApnsProvider`
  - `type FcmTokenSource interface { Token(ctx context.Context) (string, error) }`
  - `func NewStaticFcmTokenSource(token string) FcmTokenSource`
  - `func NewFcmServiceAccountTokenSource(credentialsPath, tokenURL string, client *http.Client) (*FcmServiceAccountTokenSource, error)`
  - `func NewFcmProvider(endpoint, project string, tokens FcmTokenSource) *FcmProvider`
  - `func NewFcmProviderWithClient(endpoint, project string, tokens FcmTokenSource, client *http.Client) *FcmProvider`
  - 两 Provider 均实现 `Send` + `Configured() bool`；盲推语义由 payload 携带：`PushPayload.Title == ""` 时 APNs 发 `apns-push-type: background` + `content-available`（无 alert 文案），FCM 发 data-only message（无 `notification` 段）。

- [ ] **Step 1: 写失败测试**

创建 `internal/modules/workbench/notification/apns_test.go`：

```go
package notification

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

type apnsCapture struct {
	method string
	path   string
	pushType string
	auth     string
	topic    string
	body    map[string]any
	aps     map[string]any
	status  int
	uniqueID string
}

func newApnsServer(t *testing.T, status int, uniqueID string) (*httptest.Server, *apnsCapture) {
	t.Helper()
	captured := &apnsCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method, captured.path = r.Method, r.URL.Path
		captured.pushType = r.Header.Get("apns-push-type")
		captured.auth = r.Header.Get("Authorization")
		captured.topic = r.Header.Get("apns-topic")
		_ = json.NewDecoder(r.Body).Decode(&captured.body)
		if aps, ok := captured.body["aps"].(map[string]any); ok {
			captured.aps = aps
		}
		if uniqueID != "" {
			w.Header().Set("apns-unique-id", uniqueID)
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return server, captured
}

func TestApnsProviderSendsAlertAndBackgroundPayloads(t *testing.T) {
	server, captured := newApnsServer(t, http.StatusOK, "receipt-1")
	provider := NewApnsProviderWithClient(server.URL, "bundle.acme", NewStaticApnsTokenSource("jwt-1"), server.Client())

	receipt, err := provider.Send(context.Background(), "aabb", PushPayload{Title: "completed", Body: "completed", RunID: "r1", EventID: "e1"})
	require.NoError(t, err)
	require.Equal(t, "receipt-1", receipt.ID)
	require.Equal(t, http.MethodPost, captured.method)
	require.Equal(t, "/3/device/aabb", captured.path)
	require.Equal(t, "alert", captured.pushType)
	require.Equal(t, "Bearer jwt-1", captured.auth)
	require.Equal(t, "bundle.acme", captured.topic)
	require.Equal(t, "completed", captured.aps["alert"].(map[string]any)["title"])
	require.Equal(t, "r1", captured.body["run_id"])

	// 盲推：Title 为空 → background content-available，无任何文案。
	_, err = provider.Send(context.Background(), "aabb", PushPayload{RunID: "r1", EventID: "e1"})
	require.NoError(t, err)
	require.Equal(t, "background", captured.pushType)
	require.Equal(t, float64(1), captured.aps["content-available"])
	require.NotContains(t, captured.aps, "alert", "blind push must not carry alert copy")
}

func TestApnsProviderMapsReceiptAndPermanentFailures(t *testing.T) {
	// 200 但无 apns-unique-id → MissingReceipt（可重试，不撤销）。
	server, _ := newApnsServer(t, http.StatusOK, "")
	provider := NewApnsProviderWithClient(server.URL, "bundle.acme", NewStaticApnsTokenSource("jwt"), server.Client())
	_, err := provider.Send(context.Background(), "aabb", PushPayload{Title: "completed"})
	var providerErr *ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "MissingReceipt", providerErr.Code)
	require.True(t, providerErr.Retry)
	require.False(t, providerErr.Revoke)

	// 410 → DeviceNotRegistered（撤销，不重试）。
	server410, _ := newApnsServer(t, http.StatusGone, "")
	provider = NewApnsProviderWithClient(server410.URL, "bundle.acme", NewStaticApnsTokenSource("jwt"), server410.Client())
	_, err = provider.Send(context.Background(), "aabb", PushPayload{Title: "completed"})
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "DeviceNotRegistered", providerErr.Code)
	require.True(t, providerErr.Revoke)
	require.False(t, providerErr.Retry)

	// 403 → InvalidProviderToken（配置类，不重试不撤销）。
	server403, _ := newApnsServer(t, http.StatusForbidden, "")
	provider = NewApnsProviderWithClient(server403.URL, "bundle.acme", NewStaticApnsTokenSource("jwt"), server403.Client())
	_, err = provider.Send(context.Background(), "aabb", PushPayload{Title: "completed"})
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "InvalidProviderToken", providerErr.Code)
}

func TestApnsProviderUnconfiguredFailClosed(t *testing.T) {
	require.False(t, (&ApnsProvider{}).Configured())
	require.False(t, NewApnsProvider("", "topic", NewStaticApnsTokenSource("jwt")).Configured())
	require.False(t, NewApnsProvider("https://apns.example/3/device", "", NewStaticApnsTokenSource("jwt")).Configured())
	require.False(t, NewApnsProvider("https://apns.example/3/device", "topic", nil).Configured())
	_, err := NewApnsProvider("", "topic", NewStaticApnsTokenSource("jwt")).Send(context.Background(), "aabb", PushPayload{})
	var providerErr *ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "InvalidProviderConfig", providerErr.Code)
}

func TestApnsP8TokenSourceSignsES256(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "AuthKey.p8")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600))

	source, err := NewApnsP8TokenSource(path, "KID123", "TEAM123")
	require.NoError(t, err)
	token, err := source.Token(context.Background())
	require.NoError(t, err)
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) { return &key.PublicKey, nil },
		jwt.WithValidMethods([]string{"ES256"}), jwt.WithTimeFunc(func() time.Time { return time.Now() }))
	require.NoError(t, err, "provider token must be a verifiable ES256 JWT")
	require.Equal(t, "KID123", parsed.Header["kid"])
	claims := parsed.Claims.(jwt.MapClaims)
	require.Equal(t, "TEAM123", claims["iss"])
	// 缓存：同源第二次调用返回同一 token（APNs provider token 生命期内不重签）。
	again, err := source.Token(context.Background())
	require.NoError(t, err)
	require.Equal(t, token, again)
}
```

创建 `internal/modules/workbench/notification/fcm_test.go`：

```go
package notification

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

type fcmCapture struct {
	path   string
	auth   string
	body   map[string]any
	status int
	name   string
}

func newFcmServer(t *testing.T, status int, name string) (*httptest.Server, *fcmCapture) {
	t.Helper()
	captured := &fcmCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.path, captured.auth = r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&captured.body)
		captured.status = status
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"name":"` + name + `"}`))
	}))
	t.Cleanup(server.Close)
	return server, captured
}

func TestFcmProviderSendsDataOnlyAndNotificationMessages(t *testing.T) {
	server, captured := newFcmServer(t, http.StatusOK, "projects/p/messages/1")
	provider := NewFcmProviderWithClient(server.URL, "proj-1", NewStaticFcmTokenSource("bearer-1"), server.Client())

	receipt, err := provider.Send(context.Background(), "fcm-token", PushPayload{Title: "completed", Body: "completed", RunID: "r1", EventID: "e1"})
	require.NoError(t, err)
	require.Equal(t, "projects/p/messages/1", receipt.ID)
	require.Equal(t, "/v1/projects/proj-1/messages:send", captured.path)
	require.Equal(t, "Bearer bearer-1", captured.auth)
	message := captured.body["message"].(map[string]any)
	require.Equal(t, "fcm-token", message["token"])
	require.Equal(t, "completed", message["notification"].(map[string]any)["title"])

	// 盲推：Title 为空 → data-only，无 notification 段。
	_, err = provider.Send(context.Background(), "fcm-token", PushPayload{RunID: "r1", EventID: "e1"})
	require.NoError(t, err)
	message = captured.body["message"].(map[string]any)
	require.NotContains(t, message, "notification", "blind push must not carry notification copy")
	require.Equal(t, "r1", message["data"].(map[string]any)["run_id"])
}

func TestFcmProviderMapsVendorErrors(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		response string
		code     string
		revoke   bool
		retry    bool
	}{
		{name: "unregistered", status: http.StatusNotFound, response: `{"error":{"status":"NOT_FOUND","code":404,"message":"Requested entity was not found."}}`, code: "DeviceNotRegistered", revoke: true},
		{name: "quota", status: http.StatusTooManyRequests, response: `{"error":{"status":"RESOURCE_EXHAUSTED","code":429}}`, code: "MessageRateExceeded", retry: true},
		{name: "unauthenticated", status: http.StatusUnauthorized, response: `{"error":{"status":"UNAUTHENTICATED","code":401}}`, code: "InvalidCredentials"},
		{name: "permission", status: http.StatusForbidden, response: `{"error":{"status":"PERMISSION_DENIED","code":403}}`, code: "InvalidProviderToken"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.response))
			}))
			defer server.Close()
			provider := NewFcmProviderWithClient(server.URL, "proj-1", NewStaticFcmTokenSource("bearer"), server.Client())
			_, err := provider.Send(context.Background(), "fcm-token", PushPayload{Title: "completed"})
			var providerErr *ProviderError
			require.ErrorAs(t, err, &providerErr, tc.name)
			require.Equal(t, tc.code, providerErr.Code, tc.name)
			require.Equal(t, tc.revoke, providerErr.Revoke, tc.name)
			require.Equal(t, tc.retry, providerErr.Retry, tc.name)
		})
	}
}

func TestFcmProviderUnconfiguredFailClosed(t *testing.T) {
	require.False(t, (&FcmProvider{}).Configured())
	require.False(t, NewFcmProvider("", "proj", NewStaticFcmTokenSource("t")).Configured())
	require.False(t, NewFcmProvider("https://fcm.example", "", NewStaticFcmTokenSource("t")).Configured())
	require.False(t, NewFcmProvider("https://fcm.example", "proj", nil).Configured())
}

func TestFcmServiceAccountTokenSourceExchangesAssertion(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	credentials := map[string]any{
		"client_email": "push@proj-1.iam.gserviceaccount.test",
		"private_key":  string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})),
		"token_uri":    "",
	}
	raw, err := json.Marshal(credentials)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "service-account.json")
	require.NoError(t, os.WriteFile(path, raw, 0o600))

	var assertion string
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assertion = r.FormValue("assertion")
		require.Equal(t, "urn:ietf:params:oauth:grant-type:jwt-bearer", r.FormValue("grant_type"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"oauth-bearer","expires_in":3600}`))
	}))
	defer tokenServer.Close()

	source, err := NewFcmServiceAccountTokenSource(path, tokenServer.URL, tokenServer.Client())
	require.NoError(t, err)
	token, err := source.Token(context.Background())
	require.NoError(t, err)
	require.Equal(t, "oauth-bearer", token)

	parsed, err := jwt.Parse(assertion, func(t *jwt.Token) (any, error) { return &key.PublicKey, nil },
		jwt.WithValidMethods([]string{"RS256"}))
	require.NoError(t, err, "the assertion must be a verifiable RS256 JWT")
	claims := parsed.Claims.(jwt.MapClaims)
	require.Equal(t, "push@proj-1.iam.gserviceaccount.test", claims["iss"])
	require.Equal(t, "https://www.googleapis.com/auth/firebase.messaging", claims["scope"])

	again, err := source.Token(context.Background())
	require.NoError(t, err)
	require.Equal(t, "oauth-bearer", again, "cached token is reused until near expiry")
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/workbench/notification/ -count=1`
Expected: FAIL——`undefined: NewApnsProviderWithClient` 等编译错误。

- [ ] **Step 3: 最小实现**

创建 `internal/modules/workbench/notification/apns.go`：

```go
package notification

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ApnsTokenSource supplies the short-lived provider JWT APNs requires. The
// production source signs ES256 JWTs from a deployment-owned .p8 key; tests
// and externally-minted deployments inject a static source.
type ApnsTokenSource interface {
	Token(ctx context.Context) (string, error)
}

type staticApnsTokenSource struct{ token string }

func NewStaticApnsTokenSource(token string) ApnsTokenSource {
	return &staticApnsTokenSource{token: strings.TrimSpace(token)}
}

func (s *staticApnsTokenSource) Token(context.Context) (string, error) {
	if s.token == "" {
		return "", errors.New("apns provider token is not configured")
	}
	return s.token, nil
}

const apnsTokenTTL = 50 * time.Minute

type ApnsP8TokenSource struct {
	key    *ecdsa.PrivateKey
	keyID  string
	teamID string

	mu        sync.Mutex
	cached    string
	cachedExp time.Time
}

// NewApnsP8TokenSource parses a PKCS#8 .p8 provider key once. Credentials are
// read from the deployment's key file only; they never enter logs or env echo.
func NewApnsP8TokenSource(keyPath, keyID, teamID string) (*ApnsP8TokenSource, error) {
	raw, err := os.ReadFile(strings.TrimSpace(keyPath))
	if err != nil {
		return nil, fmt.Errorf("apns p8 key: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("apns p8 key is not PEM encoded")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("apns p8 key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("apns p8 key must hold an EC private key")
	}
	return &ApnsP8TokenSource{key: key, keyID: strings.TrimSpace(keyID), teamID: strings.TrimSpace(teamID)}, nil
}

func (s *ApnsP8TokenSource) Token(ctx context.Context) (string, error) {
	if s == nil || s.key == nil {
		return "", errors.New("apns p8 token source is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != "" && time.Now().Before(s.cachedExp) {
		return s.cached, nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": s.teamID, "iat": now.Unix(), "exp": now.Add(apnsTokenTTL).Unix(),
	})
	token.Header["kid"] = s.keyID
	signed, err := token.SignedString(s.key)
	if err != nil {
		return "", fmt.Errorf("apns provider token signing: %w", err)
	}
	s.cached, s.cachedExp = signed, now.Add(apnsTokenTTL-time.Minute)
	return signed, nil
}

// ApnsProvider speaks the APNs HTTP API: POST {endpoint}/3/device/{token}.
// A payload without Title is delivered as a background content-available
// push (blind mode): no lock-screen copy, the client re-syncs on wake.
type ApnsProvider struct {
	endpoint string
	topic    string
	tokens   ApnsTokenSource
	client   *http.Client
}

func NewApnsProvider(endpoint, topic string, tokens ApnsTokenSource) *ApnsProvider {
	return NewApnsProviderWithClient(endpoint, topic, tokens, nil)
}

func NewApnsProviderWithClient(endpoint, topic string, tokens ApnsTokenSource, client *http.Client) *ApnsProvider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &ApnsProvider{endpoint: strings.TrimSpace(endpoint), topic: strings.TrimSpace(topic), tokens: tokens, client: client}
}

func (p *ApnsProvider) Configured() bool {
	if p == nil || !validPushEndpoint(p.endpoint) || p.topic == "" || p.tokens == nil {
		return false
	}
	return true
}

func validPushEndpoint(endpoint string) bool {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	return err == nil && u.Scheme != "" && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https")
}

func (p *ApnsProvider) Send(ctx context.Context, token string, payload PushPayload) (PushReceipt, error) {
	if p == nil || !p.Configured() {
		return PushReceipt{}, &ProviderError{Code: "InvalidProviderConfig", Retry: false, Err: errors.New("apns provider is not configured")}
	}
	if strings.TrimSpace(token) == "" {
		return PushReceipt{}, &ProviderError{Code: "InvalidRegistration", Revoke: true, Retry: false, Err: errors.New("empty push token")}
	}
	bearer, err := p.tokens.Token(ctx)
	if err != nil {
		return PushReceipt{}, &ProviderError{Code: "InvalidProviderConfig", Retry: false, Err: err}
	}
	body := map[string]any{"run_id": payload.RunID, "event_id": payload.EventID}
	pushType := "background"
	if payload.Title != "" {
		pushType = "alert"
		body["aps"] = map[string]any{"alert": map[string]any{"title": payload.Title, "body": payload.Body}}
	} else {
		body["aps"] = map[string]any{"content-available": 1}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return PushReceipt{}, err
	}
	endpoint := strings.TrimRight(p.endpoint, "/") + "/3/device/" + url.PathEscape(strings.TrimSpace(token))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return PushReceipt{}, &ProviderError{Code: "InvalidProviderConfig", Retry: false, Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("apns-topic", p.topic)
	req.Header.Set("apns-push-type", pushType)
	resp, err := p.client.Do(req)
	if err != nil {
		return PushReceipt{}, &ProviderError{Code: "UnknownTransport", Retry: true, Err: err}
	}
	defer resp.Body.Close()
	var reason struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&reason)
	if resp.StatusCode == http.StatusTooManyRequests {
		return PushReceipt{}, &ProviderError{Code: "MessageRateExceeded", Retry: true, StatusCode: resp.StatusCode, RetryAfter: ParseRetryAfter(resp.Header.Get("Retry-After"))}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		code := apnsFailureCode(resp.StatusCode, reason.Reason)
		revoke, retry := ClassifyPushFailure(code)
		return PushReceipt{}, &ProviderError{Code: code, Revoke: revoke, Retry: retry, StatusCode: resp.StatusCode, Err: fmt.Errorf("apns status %d reason %s", resp.StatusCode, strings.TrimSpace(reason.Reason))}
	}
	receiptID := resp.Header.Get("apns-unique-id")
	if receiptID == "" {
		return PushReceipt{}, &ProviderError{Code: "MissingReceipt", Retry: true, StatusCode: resp.StatusCode, Err: ErrMissingReceiptID}
	}
	return PushReceipt{ID: receiptID, Status: "ok"}, nil
}

func apnsFailureCode(status int, reason string) string {
	switch strings.TrimSpace(reason) {
	case "Unregistered":
		return "DeviceNotRegistered"
	case "BadDeviceToken", "DeviceTokenNotForTopic":
		return "BadDeviceToken"
	case "InvalidProviderToken", "ExpiredProviderToken":
		return "InvalidProviderToken"
	case "MissingTopic", "TopicDisallowed":
		return "InvalidProviderConfig"
	}
	if status == http.StatusGone {
		return "DeviceNotRegistered"
	}
	if status == 400 || status == 413 {
		return "MessageTooBig"
	}
	if status == http.StatusForbidden {
		return "InvalidProviderToken"
	}
	if status >= 500 {
		return "UnknownTransport"
	}
	return "InvalidProviderToken"
}
```

创建 `internal/modules/workbench/notification/fcm.go`：

```go
package notification

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const fcmScope = "https://www.googleapis.com/auth/firebase.messaging"

// FcmTokenSource supplies OAuth2 access tokens for the FCM HTTP v1 API. The
// production source exchanges a service-account RS256 assertion; tests inject
// a static source.
type FcmTokenSource interface {
	Token(ctx context.Context) (string, error)
}

type staticFcmTokenSource struct{ token string }

func NewStaticFcmTokenSource(token string) FcmTokenSource {
	return &staticFcmTokenSource{token: strings.TrimSpace(token)}
}

func (s *staticFcmTokenSource) Token(context.Context) (string, error) {
	if s.token == "" {
		return "", errors.New("fcm access token is not configured")
	}
	return s.token, nil
}

type fcmCredentials struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

type FcmServiceAccountTokenSource struct {
	credentials fcmCredentials
	key         *rsa.PrivateKey
	tokenURL    string
	client      *http.Client

	mu        sync.Mutex
	cached    string
	cachedExp time.Time
}

// NewFcmServiceAccountTokenSource parses a service-account JSON key once and
// exchanges signed assertions for access tokens at tokenURL (the production
// default is https://oauth2.googleapis.com/token, set by the container).
func NewFcmServiceAccountTokenSource(credentialsPath, tokenURL string, client *http.Client) (*FcmServiceAccountTokenSource, error) {
	raw, err := os.ReadFile(strings.TrimSpace(credentialsPath))
	if err != nil {
		return nil, fmt.Errorf("fcm service account: %w", err)
	}
	var credentials fcmCredentials
	if err := json.Unmarshal(raw, &credentials); err != nil {
		return nil, fmt.Errorf("fcm service account: %w", err)
	}
	block, _ := pem.Decode([]byte(credentials.PrivateKey))
	if block == nil {
		return nil, errors.New("fcm service account private key is not PEM encoded")
	}
	parsed, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("fcm service account private key: %w", err)
	}
	tokenURL = strings.TrimSpace(tokenURL)
	if tokenURL == "" {
		tokenURL = credentials.TokenURI
	}
	if tokenURL == "" {
		tokenURL = "https://oauth2.googleapis.com/token"
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &FcmServiceAccountTokenSource{credentials: credentials, key: parsed, tokenURL: tokenURL, client: client}, nil
}

func (s *FcmServiceAccountTokenSource) Token(ctx context.Context) (string, error) {
	if s == nil || s.key == nil {
		return "", errors.New("fcm service account source is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != "" && time.Now().Before(s.cachedExp) {
		return s.cached, nil
	}
	now := time.Now()
	assertion := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":   s.credentials.ClientEmail,
		"scope": fcmScope,
		"aud":   s.tokenURL,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})
	signed, err := assertion.SignedString(s.key)
	if err != nil {
		return "", fmt.Errorf("fcm assertion signing: %w", err)
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {signed}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var token struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil || resp.StatusCode != http.StatusOK || token.AccessToken == "" {
		return "", fmt.Errorf("fcm token exchange status %d: %w", resp.StatusCode, err)
	}
	ttl := time.Duration(token.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	s.cached, s.cachedExp = token.AccessToken, now.Add(ttl-time.Minute)
	return token.AccessToken, nil
}

// FcmProvider speaks the FCM HTTP v1 API: POST {endpoint}/v1/projects/{project}/messages:send.
// A payload without Title is a data-only message (blind mode): no notification
// copy, the client re-syncs on wake.
type FcmProvider struct {
	endpoint string
	project  string
	tokens   FcmTokenSource
	client   *http.Client
}

func NewFcmProvider(endpoint, project string, tokens FcmTokenSource) *FcmProvider {
	return NewFcmProviderWithClient(endpoint, project, tokens, nil)
}

func NewFcmProviderWithClient(endpoint, project string, tokens FcmTokenSource, client *http.Client) *FcmProvider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &FcmProvider{endpoint: strings.TrimSpace(endpoint), project: strings.TrimSpace(project), tokens: tokens, client: client}
}

func (p *FcmProvider) Configured() bool {
	return p != nil && validPushEndpoint(p.endpoint) && p.project != "" && p.tokens != nil
}

func (p *FcmProvider) Send(ctx context.Context, token string, payload PushPayload) (PushReceipt, error) {
	if p == nil || !p.Configured() {
		return PushReceipt{}, &ProviderError{Code: "InvalidProviderConfig", Retry: false, Err: errors.New("fcm provider is not configured")}
	}
	if strings.TrimSpace(token) == "" {
		return PushReceipt{}, &ProviderError{Code: "InvalidRegistration", Revoke: true, Retry: false, Err: errors.New("empty push token")}
	}
	bearer, err := p.tokens.Token(ctx)
	if err != nil {
		return PushReceipt{}, &ProviderError{Code: "InvalidProviderConfig", Retry: false, Err: err}
	}
	message := map[string]any{"token": strings.TrimSpace(token), "data": map[string]string{"run_id": payload.RunID, "event_id": payload.EventID}}
	if payload.Title != "" {
		message["notification"] = map[string]any{"title": payload.Title, "body": payload.Body}
	}
	raw, err := json.Marshal(map[string]any{"message": message})
	if err != nil {
		return PushReceipt{}, err
	}
	endpoint := strings.TrimRight(p.endpoint, "/") + "/v1/projects/" + url.PathEscape(p.project) + "/messages:send"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return PushReceipt{}, &ProviderError{Code: "InvalidProviderConfig", Retry: false, Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)
	resp, err := p.client.Do(req)
	if err != nil {
		return PushReceipt{}, &ProviderError{Code: "UnknownTransport", Retry: true, Err: err}
	}
	defer resp.Body.Close()
	var body struct {
		Name  string `json:"name"`
		Error struct {
			Status string `json:"status"`
		} `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if resp.StatusCode == http.StatusTooManyRequests {
		return PushReceipt{}, &ProviderError{Code: "MessageRateExceeded", Retry: true, StatusCode: resp.StatusCode, RetryAfter: ParseRetryAfter(resp.Header.Get("Retry-After"))}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		code := fcmFailureCode(resp.StatusCode, body.Error.Status)
		revoke, retry := ClassifyPushFailure(code)
		return PushReceipt{}, &ProviderError{Code: code, Revoke: revoke, Retry: retry, StatusCode: resp.StatusCode, Err: fmt.Errorf("fcm status %d %s", resp.StatusCode, strings.TrimSpace(body.Error.Status))}
	}
	if body.Name == "" {
		return PushReceipt{}, &ProviderError{Code: "MissingReceipt", Retry: true, StatusCode: resp.StatusCode, Err: ErrMissingReceiptID}
	}
	return PushReceipt{ID: body.Name, Status: "ok"}, nil
}

func fcmFailureCode(status int, vendorStatus string) string {
	switch strings.TrimSpace(vendorStatus) {
	case "NOT_FOUND", "UNREGISTERED":
		return "DeviceNotRegistered"
	case "RESOURCE_EXHAUSTED":
		return "MessageRateExceeded"
	case "UNAUTHENTICATED":
		return "InvalidCredentials"
	case "PERMISSION_DENIED":
		return "InvalidProviderToken"
	case "INVALID_ARGUMENT":
		return "MessageTooBig"
	}
	if status == http.StatusUnauthorized {
		return "InvalidCredentials"
	}
	if status == http.StatusForbidden {
		return "InvalidProviderToken"
	}
	return "UnknownTransport"
}
```

注意：`validPushEndpoint` 与 `provider.go` 既有语义一致但属本包新私有函数（expo.go 用的是方法内联校验）；不改动既有文件。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/workbench/notification/ -count=1`
Expected: PASS（含既有 `expo_test.go`/`provider_test.go` 全部用例与新增 8 个用例）。

- [ ] **Step 5: Commit**

```bash
git add internal/modules/workbench/notification/apns.go internal/modules/workbench/notification/apns_test.go internal/modules/workbench/notification/fcm.go internal/modules/workbench/notification/fcm_test.go
git commit -m "feat(mobile-push): APNs and FCM providers with blind background payloads (T37 #67 task 2)"
```

---

### Task 3: workbenchservice——盲推/禁用/按 App 路由 + 容器装配

**Files:**
- Modify: `internal/modules/workbench/service/workbench/notification_delivery.go`
- Modify: `internal/config/config.go`（`MobileNotificationConfig` + env 覆盖）
- Modify: `internal/container/container.go:1081-1121`（`newMobileNotificationProvider` 重写）
- Modify: `docs/architecture/integration/workbench.md` §7 配置键
- Test: `internal/modules/workbench/service/workbench/notification_app_policy_test.go`

**Interfaces:**
- Consumes: Task 1 的 `repository.NormalizeMobileAppID`/`GetActiveForApp`/`RevokeForApp` 与 `NotificationIntent.AppID`；Task 2 的 `notification.NewApnsProvider`/`NewFcmProvider`/`NewApnsP8TokenSource`/`NewFcmServiceAccountTokenSource`。
- Produces（Task 5 与运维依赖）:
  - `type PushPayloadPolicy struct{ Blind bool }`
  - `func NewPushNotificationProviderWithOptions(provider pushnotification.PushProvider, resolve func(context.Context, repository.NotificationDelivery) (string, error), policy PushPayloadPolicy) *PushNotificationProvider`（既有 `NewPushNotificationProvider` 保留并委托 `PushPayloadPolicy{}`）
  - `func NewHTTPNotificationProviderWithPolicy(endpoint string, blind bool) *HTTPNotificationProvider`（既有 `NewHTTPNotificationProvider(endpoint)` 保留并委托 `blind=false`）
  - `func NewDisabledNotificationProvider() *DisabledNotificationProvider`；`DisabledNotificationProvider` 实现 `Send` + `Configured()`，错误码 `mobile_notification_provider_disabled`
  - `func NewAppRoutingNotificationProvider(fallback NotificationProvider, routes map[string]NotificationProvider) *AppRoutingNotificationProvider`（`Send`/`SendReceipt`/`Configured`）
  - `NotificationDeviceRevoker` 接口方法改为 `RevokeForApp(context.Context, uint64, string, string, string, int64) error`
  - 配置：`MobileNotificationConfig.Payload string`（yaml `payload`）+ env `MOBILE_NOTIFICATION_PAYLOAD`（`blind`|其他→默认）；容器 env 新键：`MOBILE_ENTERPRISE_APP_ID`、`MOBILE_ENTERPRISE_PUSH_PROVIDER`（apns|fcm）、`MOBILE_APNS_ENDPOINT`（默认 `https://api.push.apple.com/3/device`）、`MOBILE_APNS_TOPIC`、`MOBILE_APNS_KEY_PATH`、`MOBILE_APNS_KEY_ID`、`MOBILE_APNS_TEAM_ID`、`MOBILE_FCM_ENDPOINT`（默认 `https://fcm.googleapis.com`）、`MOBILE_FCM_PROJECT_ID`、`MOBILE_FCM_CREDENTIALS_PATH`；`MOBILE_NOTIFICATION_PROVIDER` 新增值 `disabled`/`none`。

- [ ] **Step 1: 写失败测试**

创建 `internal/modules/workbench/service/workbench/notification_app_policy_test.go`（`package workbench`；DB helper 为聚焦迁移——不依赖损坏的全目录 `migrator.Up()`）：

```go
package workbench

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	pushnotification "github.com/Tencent/WeKnora/internal/modules/workbench/notification"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func execMigrationFiles(t *testing.T, db *gorm.DB, root string, files ...string) {
	t.Helper()
	for _, file := range files {
		raw, err := os.ReadFile(filepath.Join(root, file))
		require.NoError(t, err, file)
		require.NoError(t, db.Exec(string(raw)).Error, file)
	}
}

// openMobilePushPolicyDB 只执行本域迁移子集（差异记录第 4 条：全目录迁移在 HEAD 因
// 同号 000112 损坏），顺序必须保持 000058 → 000059 → 000060 → 000114（000114 重建段
// 依赖前两者建出的 mobile_devices 与 mobile_notification_intents），
// 再建 provider_state 表与最小 agent_runs/agent_run_events。
func openMobilePushPolicyDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../../../"))
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "push-policy.db")+"?_busy_timeout=5000"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	execMigrationFiles(t, db, root,
		"migrations/sqlite/000058_mobile_devices.up.sql",
		"migrations/sqlite/000059_mobile_notifications.up.sql",
		"migrations/sqlite/000060_mobile_notification_delivery.up.sql",
		"migrations/sqlite/000114_mobile_device_app.up.sql",
	)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS mobile_notification_provider_state (provider_key TEXT PRIMARY KEY, paused INTEGER NOT NULL DEFAULT 0, reason TEXT NOT NULL DEFAULT '', alert_count INTEGER NOT NULL DEFAULT 0, paused_at DATETIME, recovered_at DATETIME, updated_at DATETIME NOT NULL)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, owner_id TEXT NOT NULL, PRIMARY KEY (tenant_id, run_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE agent_run_events (tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, seq INTEGER NOT NULL, attempt_id TEXT NOT NULL DEFAULT '', event_type TEXT NOT NULL, payload TEXT NOT NULL DEFAULT '{}', PRIMARY KEY (tenant_id, run_id, seq))`).Error)
	require.NoError(t, db.Exec("INSERT INTO agent_runs (tenant_id, run_id, owner_id) VALUES (1, 'r1', 'u1')").Error)
	require.NoError(t, db.Exec("INSERT INTO agent_run_events (tenant_id, run_id, seq, event_type, payload) VALUES (1, 'r1', 9, 'run_completed', '{}')").Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

type capturingPushProvider struct {
	payloads []pushnotification.PushPayload
}

func (p *capturingPushProvider) Send(_ context.Context, _ string, payload pushnotification.PushPayload) (pushnotification.PushReceipt, error) {
	p.payloads = append(p.payloads, payload)
	return pushnotification.PushReceipt{ID: "apns-receipt", Status: "ok"}, nil
}

func appDelivery(id, appID string) repository.NotificationDelivery {
	return repository.NotificationDelivery{
		ID: id, Fence: 1,
		Intent: repository.NotificationIntent{
			TenantID: 1, EventID: "1:r1:9", OwnerID: "u1", DeviceID: "d-" + appID,
			Environment: "dev", AppID: appID, Kind: "completed", RunID: "r1",
		},
	}
}

func TestPushPayloadPolicyBlindStripsKind(t *testing.T) {
	inner := &capturingPushProvider{}
	resolve := func(context.Context, repository.NotificationDelivery) (string, error) { return "token", nil }
	visible := NewPushNotificationProvider(inner, resolve)
	_, err := visible.SendReceipt(context.Background(), appDelivery("id-1", "official"))
	require.NoError(t, err)
	require.Equal(t, "completed", inner.payloads[0].Title, "default policy keeps today's kind copy")
	require.Equal(t, "completed", inner.payloads[0].Body)

	blind := NewPushNotificationProviderWithOptions(inner, resolve, PushPayloadPolicy{Blind: true})
	inner.payloads = nil
	_, err = blind.SendReceipt(context.Background(), appDelivery("id-2", "official"))
	require.NoError(t, err)
	require.Equal(t, "", inner.payloads[0].Title, "blind policy must strip kind from the visible copy")
	require.Equal(t, "", inner.payloads[0].Body)
	require.Equal(t, "r1", inner.payloads[0].RunID, "opaque re-sync ids survive blinding")
	require.Equal(t, "1:r1:9", inner.payloads[0].EventID)
}

func TestHTTPNotificationProviderBlindOmitsKind(t *testing.T) {
	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"gw-1","status":"ok"}`))
	}))
	defer server.Close()

	visible := NewHTTPNotificationProvider(server.URL)
	_, err := visible.SendReceipt(context.Background(), appDelivery("id-1", "official"))
	require.NoError(t, err)
	require.Equal(t, "completed", bodies[0]["kind"])

	blind := NewHTTPNotificationProviderWithPolicy(server.URL, true)
	_, err = blind.SendReceipt(context.Background(), appDelivery("id-2", "official"))
	require.NoError(t, err)
	require.NotContains(t, bodies[1], "kind", "blind gateway payload must omit the kind metadata key")
	require.Equal(t, "r1", bodies[1]["run_id"], "opaque ids stay for the gateway to resolve the device")
}

func TestAppRoutingProviderDispatchesByApp(t *testing.T) {
	official := &capturingPushProvider{}
	enterprise := &capturingPushProvider{}
	routing := NewAppRoutingNotificationProvider(
		&notificationProviderSpyAdapter{inner: official},
		map[string]NotificationProvider{"enterprise:acme": &notificationProviderSpyAdapter{inner: enterprise}},
	)
	require.NoError(t, routing.Send(context.Background(), appDelivery("id-1", "official")))
	require.NoError(t, routing.Send(context.Background(), appDelivery("id-2", "enterprise:acme")))
	require.NoError(t, routing.Send(context.Background(), appDelivery("id-3", ""))) // 空 AppID 归一化为 official → fallback
	require.Len(t, official.payloads, 2)
	require.Len(t, enterprise.payloads, 1)
}

// notificationProviderSpyAdapter 把 capturingPushProvider 适配为 NotificationProvider。
type notificationProviderSpyAdapter struct{ inner *capturingPushProvider }

func (a *notificationProviderSpyAdapter) Send(ctx context.Context, d repository.NotificationDelivery) error {
	_, err := a.inner.Send(ctx, "", pushnotification.PushPayload{Title: d.Intent.Kind, Body: d.Intent.Kind, RunID: d.Intent.RunID, EventID: d.Intent.EventID})
	return err
}

func TestDisabledNotificationProviderPausesDurablyWithoutRetryStorm(t *testing.T) {
	db := openMobilePushPolicyDB(t)
	store := repository.NewNotificationStore(db)
	health := repository.NewNotificationProviderStateStore(db)
	require.NoError(t, store.Enqueue(context.Background(), repository.NotificationIntent{
		TenantID: 1, EventID: "1:r1:9", OwnerID: "u1", DeviceID: "d-official", Environment: "dev",
		AppID: "official", Kind: "completed", RunID: "r1", ExpiresAt: time.Now().Add(time.Hour),
	}))
	require.NoError(t, db.Exec(`INSERT INTO mobile_devices (tenant_id, owner_id, device_id, environment, app_id, platform, token_ciphertext, token_hash) VALUES (1, 'u1', 'd-official', 'dev', 'official', 'ios', 'cipher', 'hash')`).Error)
	worker := NewNotificationDeliveryWorkerWithHealth(store, NewDisabledNotificationProvider(), "disabled-worker", nil, health, "mobile")
	require.NoError(t, worker.RunOnce(context.Background(), 10))
	state, err := health.State(context.Background(), "mobile")
	require.NoError(t, err)
	require.True(t, state.Paused, "disabled mode must pause durably like any configuration error")
	// 第二轮：paused 且 provider 未配置 → claim 前即跳过，不刷告警。
	require.NoError(t, worker.RunOnce(context.Background(), 10))
	state, err = health.State(context.Background(), "mobile")
	require.NoError(t, err)
	require.EqualValues(t, 1, state.AlertCount, "no retry storm while paused+unconfigured")
	var row struct{ State string }
	require.NoError(t, db.Table("mobile_notification_intents").Select("state").Take(&row).Error)
	require.Equal(t, "pending", row.State, "disabling push never drops durable intents")
}

func TestAppRoutingProviderRevokesOnlyOwnAppRegistration(t *testing.T) {
	db := openMobilePushPolicyDB(t)
	ctx := context.Background()
	devices := repository.NewMobileDeviceStore(db, "dev")
	for _, app := range []string{"official", "enterprise:acme"} {
		require.NoError(t, devices.Bind(ctx, repository.DeviceRegistration{
			TenantID: 1, OwnerID: "u1", DeviceID: "shared-device", Environment: "dev", Platform: "ios",
			AppID: app, TokenCiphertext: "cipher-" + app, TokenHash: repository.DeviceTokenHash("tok-" + app),
			Revision: 0, ScopeGeneration: 1,
		}))
	}
	store := repository.NewNotificationStore(db)
	health := repository.NewNotificationProviderStateStore(db)
	failing := &revokingDirectProvider{}
	enterpriseProvider := NewPushNotificationProviderWithOptions(failing, func(context.Context, repository.NotificationDelivery) (string, error) {
		return "enterprise-token", nil
	}, PushPayloadPolicy{Blind: true})
	worker := NewNotificationDeliveryWorkerWithHealth(store, enterpriseProvider, "revoke-worker", devices, health, "mobile-revoke")
	require.NoError(t, store.Enqueue(ctx, repository.NotificationIntent{
		TenantID: 1, EventID: "1:r1:9", OwnerID: "u1", DeviceID: "shared-device", Environment: "dev",
		AppID: "enterprise:acme", Kind: "completed", RunID: "r1", ExpiresAt: time.Now().Add(time.Hour),
	}))
	require.NoError(t, worker.RunOnce(ctx, 10))
	_, err := devices.GetActiveForApp(ctx, 1, "u1", "shared-device", "enterprise:acme")
	require.ErrorIs(t, err, repository.ErrMobileDeviceNotFound, "the failing enterprise registration is revoked")
	official, err := devices.GetActiveForApp(ctx, 1, "u1", "shared-device", "official")
	require.NoError(t, err, "the official registration survives an enterprise provider failure")
	require.EqualValues(t, 1, official.Revision)
}

// revokingDirectProvider 模拟 APNs/FCM 的 DeviceNotRegistered 永久失败。
type revokingDirectProvider struct{}

func (p *revokingDirectProvider) Send(context.Context, string, pushnotification.PushPayload) (pushnotification.PushReceipt, error) {
	return pushnotification.PushReceipt{}, &pushnotification.ProviderError{Code: "DeviceNotRegistered", Revoke: true, Retry: false}
}
```

注意：worker 的 `deviceRevoker` 字段类型改为新接口（见 Step 3），`NewNotificationDeliveryWorkerWithHealth` 第 4 参在容器里传 `*repository.MobileDeviceStore`（Task 1 已实现 `RevokeForApp`），本测试同样传具体 store——签名不变、接口方法变。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/workbench/service/workbench/ -run 'TestPushPayloadPolicyBlind|TestHTTPNotificationProviderBlind|TestAppRouting|TestDisabledNotificationProviderPauses' -count=1`
Expected: FAIL——`undefined: NewPushNotificationProviderWithOptions` 等编译错误。

- [ ] **Step 3: 最小实现**

修改 `internal/modules/workbench/service/workbench/notification_delivery.go`：

(a) `NotificationDeviceRevoker`（`notification_delivery.go:62-64`）改为：

```go
// NotificationDeviceRevoker invalidates the device registration that produced
// a permanent provider failure. Implementations must scope the operation to
// the tenant, owner, and app carried by the durable intent.
type NotificationDeviceRevoker interface {
	RevokeForApp(context.Context, uint64, string, string, string, int64) error
}
```

`releaseDeliveryWithCause` 调用点（`notification_delivery.go:450`）改为：

```go
					if revokeErr := w.deviceRevoker.RevokeForApp(ctx, d.Intent.TenantID, d.Intent.OwnerID, d.Intent.DeviceID, repository.NormalizeMobileAppID(d.Intent.AppID), d.DeviceRevision); revokeErr != nil {
```

（包内已 import `repository`。）

(b) `PushNotificationProvider`（`notification_delivery.go:76-93`）加 policy 字段与新构造器（旧构造器保留）：

```go
type PushNotificationProvider struct {
	provider pushnotification.PushProvider
	resolve  func(context.Context, repository.NotificationDelivery) (string, error)
	policy   PushPayloadPolicy
}

// PushPayloadPolicy is the deployment metadata-exposure policy (story 67):
// blind strips every human-readable kind/title/body from the vendor payload;
// opaque re-sync ids (run_id/event_id) always survive.
type PushPayloadPolicy struct{ Blind bool }

func NewPushNotificationProvider(provider pushnotification.PushProvider, resolve func(context.Context, repository.NotificationDelivery) (string, error)) *PushNotificationProvider {
	return NewPushNotificationProviderWithOptions(provider, resolve, PushPayloadPolicy{})
}

func NewPushNotificationProviderWithOptions(provider pushnotification.PushProvider, resolve func(context.Context, repository.NotificationDelivery) (string, error), policy PushPayloadPolicy) *PushNotificationProvider {
	return &PushNotificationProvider{provider: provider, resolve: resolve, policy: policy}
}

func (p *PushNotificationProvider) payloadOf(d repository.NotificationDelivery) pushnotification.PushPayload {
	if p.policy.Blind {
		return pushnotification.PushPayload{RunID: d.Intent.RunID, EventID: d.Intent.EventID}
	}
	return pushnotification.PushPayload{Title: d.Intent.Kind, Body: d.Intent.Kind, RunID: d.Intent.RunID, EventID: d.Intent.EventID}
}
```

`SendReceipt`（`notification_delivery.go:100`，payload 字面量在 ：111）与 `SendBatch`（`notification_delivery.go:114`，payload 字面量在 ：150）里的 payload 字面量替换为 `p.payloadOf(d)` / `p.payloadOf(delivery)`（`SendBatch` 循环变量为 `d`）。

(c) `HTTPNotificationProvider`（`notification_delivery.go:175-186`）加 blind 字段：

```go
type HTTPNotificationProvider struct {
	endpoint string
	blind    bool
	client   *http.Client
}

func NewHTTPNotificationProvider(endpoint string) *HTTPNotificationProvider {
	return NewHTTPNotificationProviderWithPolicy(endpoint, false)
}

// NewHTTPNotificationProviderWithPolicy: blind=true omits the kind metadata
// key from the gateway payload; device identity and opaque ids stay so the
// gateway can still resolve the encrypted token server-side.
func NewHTTPNotificationProviderWithPolicy(endpoint string, blind bool) *HTTPNotificationProvider {
	return &HTTPNotificationProvider{endpoint: strings.TrimSpace(endpoint), blind: blind, client: &http.Client{Timeout: 10 * time.Second}}
}
```

`send()`（`notification_delivery.go:202`，payload 构造在 ：209）payload 构造改为：

```go
	payloadMap := map[string]any{"tenant_id": d.Intent.TenantID, "owner_id": d.Intent.OwnerID, "device_id": d.Intent.DeviceID, "environment": d.Intent.Environment, "app_id": repository.NormalizeMobileAppID(d.Intent.AppID), "event_id": d.Intent.EventID, "run_id": d.Intent.RunID, "attempt": d.Attempt}
	if !p.blind {
		payloadMap["kind"] = d.Intent.Kind
	}
	payload, err := json.Marshal(payloadMap)
```

(d) 文件尾部（`notification_delivery.go` 末尾）追加两个新 provider：

```go
// DisabledNotificationProvider is the explicit "no push" deployment policy
// (story 67). Sends fail closed with a configuration-class code so the
// durable worker pauses once and never storms; intents stay pending and are
// recoverable the moment the policy is switched back.
type DisabledNotificationProvider struct{}

func NewDisabledNotificationProvider() *DisabledNotificationProvider { return &DisabledNotificationProvider{} }

func (p *DisabledNotificationProvider) Configured() bool { return false }

func (p *DisabledNotificationProvider) Send(context.Context, repository.NotificationDelivery) error {
	return &pushnotification.ProviderError{Code: "mobile_notification_provider_disabled", Retry: false, Err: errors.New("mobile notification provider is disabled by deployment policy")}
}

// AppRoutingNotificationProvider dispatches each durable delivery to the
// provider registered for its app identity. An unknown or empty AppID
// normalizes to official and hits the fallback provider.
type AppRoutingNotificationProvider struct {
	fallback NotificationProvider
	routes   map[string]NotificationProvider
}

func NewAppRoutingNotificationProvider(fallback NotificationProvider, routes map[string]NotificationProvider) *AppRoutingNotificationProvider {
	return &AppRoutingNotificationProvider{fallback: fallback, routes: routes}
}

func (p *AppRoutingNotificationProvider) forApp(d repository.NotificationDelivery) NotificationProvider {
	if p == nil {
		return nil
	}
	app := repository.NormalizeMobileAppID(d.Intent.AppID)
	if routed, ok := p.routes[app]; ok && routed != nil {
		return routed
	}
	return p.fallback
}

func (p *AppRoutingNotificationProvider) Send(ctx context.Context, d repository.NotificationDelivery) error {
	provider := p.forApp(d)
	if provider == nil {
		return &pushnotification.ProviderError{Code: "mobile_notification_provider_unconfigured", Retry: false, Err: errors.New("no notification provider is configured for this app")}
	}
	return provider.Send(ctx, d)
}

func (p *AppRoutingNotificationProvider) SendReceipt(ctx context.Context, d repository.NotificationDelivery) (pushnotification.PushReceipt, error) {
	provider := p.forApp(d)
	if receiptProvider, ok := provider.(NotificationReceiptProvider); ok {
		return receiptProvider.SendReceipt(ctx, d)
	}
	err := p.Send(ctx, d)
	return pushnotification.PushReceipt{}, err
}

func (p *AppRoutingNotificationProvider) Configured() bool {
	if p == nil || p.fallback == nil {
		return false
	}
	if configured, ok := p.fallback.(NotificationProviderConfiguration); ok {
		return configured.Configured()
	}
	return true
}
```

`isProviderConfigurationError`（`notification_delivery.go:485-492`）case 列表加 `"mobile_notification_provider_disabled"`。

修改 `internal/config/config.go`：`MobileNotificationConfig`（`config.go:257-264`）加字段 `Payload string \`yaml:"payload" json:"payload"\``；`applyMobileNotificationEnvOverrides`（`config.go:1016` 起）加：

```go
	if value := strings.TrimSpace(os.Getenv("MOBILE_NOTIFICATION_PAYLOAD")); value != "" {
		cfg.MobileNotification.Payload = value
	}
```

修改 `internal/container/container.go:1081-1121`：`newMobileNotificationProvider` 整体替换为：

```go
// newMobileNotificationProvider keeps push delivery behind deployment policy
// (story 67): the standard client uses the single-deployment expo/gateway
// selection with an optional blind-payload mode and an explicit disabled
// mode; an enterprise-signed app (MOBILE_ENTERPRISE_APP_ID) gets its own
// APNs/FCM lane routed by intent app identity. Unknown modes and partial
// enterprise configuration fail closed rather than silently selecting a
// different vendor.
func newMobileNotificationProvider(cfg *config.Config, devices *repository.MobileDeviceStore) workbenchservice.NotificationProvider {
	endpoint := strings.TrimSpace(os.Getenv("MOBILE_NOTIFICATION_PROVIDER_URL"))
	providerKind := strings.ToLower(strings.TrimSpace(os.Getenv("MOBILE_NOTIFICATION_PROVIDER")))
	accessToken := strings.TrimSpace(os.Getenv("MOBILE_NOTIFICATION_ACCESS_TOKEN"))
	if cfg != nil && cfg.MobileNotification != nil {
		if endpoint == "" {
			endpoint = strings.TrimSpace(cfg.MobileNotification.ProviderURL)
		}
		if providerKind == "" {
			providerKind = strings.ToLower(strings.TrimSpace(cfg.MobileNotification.Provider))
		}
		if accessToken == "" {
			accessToken = strings.TrimSpace(cfg.MobileNotification.AccessToken)
		}
	}
	blind := false
	if cfg != nil && cfg.MobileNotification != nil && strings.EqualFold(strings.TrimSpace(cfg.MobileNotification.Payload), "blind") {
		blind = true
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("MOBILE_NOTIFICATION_PAYLOAD")), "blind") {
		blind = true
	}
	resolveFor := func(appID string) func(ctx context.Context, d repository.NotificationDelivery) (string, error) {
		return func(ctx context.Context, d repository.NotificationDelivery) (string, error) {
			if devices == nil {
				return "", errors.New("mobile_device_store_unavailable")
			}
			registration, err := devices.GetActiveForApp(ctx, d.Intent.TenantID, d.Intent.OwnerID, d.Intent.DeviceID, appID)
			if err != nil {
				return "", err
			}
			key := secutils.GetAESKey()
			if len(key) != 32 {
				return "", errors.New("mobile_device_token_decryption_not_configured")
			}
			return secutils.DecryptAESGCM(registration.TokenCiphertext, key)
		}
	}
	var official workbenchservice.NotificationProvider
	switch {
	case providerKind == "disabled" || providerKind == "none":
		official = workbenchservice.NewDisabledNotificationProvider()
	case strings.EqualFold(providerKind, "expo"):
		official = workbenchservice.NewPushNotificationProviderWithOptions(pushnotification.NewExpoProvider(endpoint, accessToken), resolveFor(repository.MobileAppIDOfficial), workbenchservice.PushPayloadPolicy{Blind: blind})
	case providerKind == "" || strings.EqualFold(providerKind, "gateway") || strings.EqualFold(providerKind, "http"):
		official = workbenchservice.NewHTTPNotificationProviderWithPolicy(endpoint, blind)
	default:
		// 未知模式 fail-closed：即使配置了 URL 也不投递（container.go:1114-1116 既有语义）。
		official = workbenchservice.NewHTTPNotificationProviderWithPolicy("", false)
	}
	enterpriseApp := strings.TrimSpace(os.Getenv("MOBILE_ENTERPRISE_APP_ID"))
	if enterpriseApp == "" || enterpriseApp == repository.MobileAppIDOfficial || repository.ValidateMobileAppID(enterpriseApp) != nil {
		// 未声明或非法的企业 App id：只有 official 通道（fail closed）。
		return official
	}
	var enterprise workbenchservice.NotificationProvider = workbenchservice.NewDisabledNotificationProvider()
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MOBILE_ENTERPRISE_PUSH_PROVIDER"))) {
	case "apns":
		endpoint := strings.TrimSpace(os.Getenv("MOBILE_APNS_ENDPOINT"))
		if endpoint == "" {
			endpoint = "https://api.push.apple.com/3/device"
		}
		if source, err := pushnotification.NewApnsP8TokenSource(os.Getenv("MOBILE_APNS_KEY_PATH"), os.Getenv("MOBILE_APNS_KEY_ID"), os.Getenv("MOBILE_APNS_TEAM_ID")); err == nil {
			enterprise = workbenchservice.NewPushNotificationProviderWithOptions(
				pushnotification.NewApnsProvider(endpoint, os.Getenv("MOBILE_APNS_TOPIC"), source),
				resolveFor(enterpriseApp), workbenchservice.PushPayloadPolicy{Blind: blind})
		}
	case "fcm":
		endpoint := strings.TrimSpace(os.Getenv("MOBILE_FCM_ENDPOINT"))
		if endpoint == "" {
			endpoint = "https://fcm.googleapis.com"
		}
		if source, err := pushnotification.NewFcmServiceAccountTokenSource(os.Getenv("MOBILE_FCM_CREDENTIALS_PATH"), "", nil); err == nil {
			enterprise = workbenchservice.NewPushNotificationProviderWithOptions(
				pushnotification.NewFcmProvider(endpoint, os.Getenv("MOBILE_FCM_PROJECT_ID"), source),
				resolveFor(enterpriseApp), workbenchservice.PushPayloadPolicy{Blind: blind})
		}
	}
	return workbenchservice.NewAppRoutingNotificationProvider(official, map[string]workbenchservice.NotificationProvider{enterpriseApp: enterprise})
}
```

（`container.go` 需新增 import `pushnotification "github.com/Tencent/WeKnora/internal/modules/workbench/notification"`；既有 `secutils` 别名沿用——`container.go:1099` 已用 `secutils`，确认 import 别名不变。）

修改 `docs/architecture/integration/workbench.md` §7 移动通知配置键 bullet，在 `MOBILE_NOTIFICATION_ACCESS_TOKEN` 之后补一行：

```markdown
  - 盲推/禁用/企业自签名（T37 #67）：`MOBILE_NOTIFICATION_PAYLOAD`（blind=无正文盲推送，
    去除 kind/title 元数据）、`MOBILE_NOTIFICATION_PROVIDER=disabled|none`（显式禁用，
    durable 暂停不丢行）、`MOBILE_ENTERPRISE_APP_ID`（企业自构建 App 唯一允许清单，
    `enterprise:<slug>`）、`MOBILE_ENTERPRISE_PUSH_PROVIDER`（apns|fcm）、
    `MOBILE_APNS_{ENDPOINT,TOPIC,KEY_PATH,KEY_ID,TEAM_ID}`、
    `MOBILE_FCM_{ENDPOINT,PROJECT_ID,CREDENTIALS_PATH}`（凭据只从密钥文件路径读取）。
```

同时检查既有 revoker 接口实现的测试替身：`grep -n "RevokeForTenant" internal/modules/workbench/service/workbench/*_test.go`——若有手写 fake 实现旧接口，同步把方法名改为 `RevokeForApp`（本仓库实查该 grep 在 `notification_delivery_test.go` 无手写 fake，`revoker` 一律传具体 `*repository.MobileDeviceStore`，预期零改动；执行时以 grep 结果为准，如出现 fake 则逐个改名）。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/workbench/service/workbench/ -run 'TestPushPayloadPolicyBlind|TestHTTPNotificationProviderBlind|TestAppRouting|TestDisabledNotificationProviderPauses' -count=1 && go build ./...`
Expected: PASS + 编译通过（`go build ./...` 覆盖 container 装配编译）。

- [ ] **Step 5: Commit**

```bash
git add internal/modules/workbench/service/workbench/notification_delivery.go internal/modules/workbench/service/workbench/notification_app_policy_test.go internal/config/config.go internal/container/container.go docs/architecture/integration/workbench.md
git commit -m "feat(mobile-push): blind/disabled payload policy and per-app provider routing (T37 #67 task 3)"
```

---

### Task 4: HTTP 边界——注册 wire 携带 app_id（intent HMAC 绑定 + 部署 App 允许清单）

**Files:**
- Modify: `internal/handler/mobile_device.go`
- Modify: `internal/container/workbench.go:123-125`（`NewMobileDeviceHandler` 挂策略）
- Test: `internal/handler/mobile_device_app_test.go`

**Interfaces:**
- Consumes: Task 1 的 `CurrentScopeGenerationForApp`/`GetActiveForApp`/`RevokeForApp`/`...ForApp` presence 族与 `ValidateMobileAppID`/`NormalizeMobileAppID`。
- Produces:
  - 请求体新可选字段 `app_id`（`POST /api/v1/mobile/devices/:id/registration-intent` body、`PUT /api/v1/mobile/devices/:id` body、`DELETE /api/v1/mobile/devices/:id?app_id=`、presence 三端点 `?app_id=` 查询参数）；缺省归 `official`。
  - 响应 data 新增 `app_id` 字段（register/list/presence）。
  - `type MobileAppPolicy struct{ EnterpriseAppID string }`；`func (h *MobileDeviceHandler) WithMobileAppPolicy(policy MobileAppPolicy) *MobileDeviceHandler`（默认零值=仅 official）。
  - 注册 intent 的 HMAC 载荷新增 `AppID` 字段（intent 签发与校验都归一化，official 跨界不可用、企业 intent 不能绑 official 行）。
  - 未声明/非法 `app_id` → 400；合法但与服务端允许清单不符 → 400。

- [ ] **Step 1: 写失败测试**

创建 `internal/handler/mobile_device_app_test.go`（复用既有 `openMobileHandlerDB`（`mobile_device_test.go:20-27`，Task 1 已补执行新迁移）；请求上下文用本文件自带的 recorder helper，不动既有 `mobileRequest`）：

```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func appDeviceHandler(t *testing.T, enterpriseAppID string) *MobileDeviceHandler {
	t.Helper()
	t.Setenv("SYSTEM_AES_KEY", "12345678901234567890123456789012")
	store := repository.NewMobileDeviceStore(openMobileHandlerDB(t), "dev")
	return NewMobileDeviceHandlerWithSealer(store, "dev", func(value string) (string, error) { return "enc:" + value, nil }).
		WithMobileAppPolicy(MobileAppPolicy{EnterpriseAppID: enterpriseAppID})
}

func appGinContext(method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request.WithContext(context.WithValue(context.WithValue(request.Context(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1"))
	return ctx, recorder
}

func issueAppIntent(t *testing.T, h *MobileDeviceHandler, device, appID string) string {
	t.Helper()
	ctx, recorder := appGinContext(http.MethodPost, "/api/v1/mobile/devices/"+device+"/registration-intent", `{"app_id":"`+appID+`"}`)
	ctx.Params = gin.Params{{Key: "id", Value: device}}
	h.IssueIntent(ctx)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response struct {
		Data struct {
			RegistrationIntent string `json:"registration_intent"`
			AppID              string `json:"app_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, appID, response.Data.AppID)
	return response.Data.RegistrationIntent
}

func registerAppDevice(t *testing.T, h *MobileDeviceHandler, device, appID, token, intent string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, recorder := appGinContext(http.MethodPut, "/api/v1/mobile/devices/"+device,
		`{"token":"`+token+`","platform":"ios","app_id":"`+appID+`","registration_intent":"`+intent+`"}`)
	ctx.Params = gin.Params{{Key: "id", Value: device}}
	h.Register(ctx)
	return recorder
}

func TestRegisterAppIDBindsIntentAndIsolates(t *testing.T) {
	h := appDeviceHandler(t, "enterprise:acme")
	officialIntent := issueAppIntent(t, h, "shared", "official")
	enterpriseIntent := issueAppIntent(t, h, "shared", "enterprise:acme")

	require.Equal(t, http.StatusOK, registerAppDevice(t, h, "shared", "official", "tok-official", officialIntent).Code)
	require.Equal(t, http.StatusOK, registerAppDevice(t, h, "shared", "enterprise:acme", "tok-enterprise", enterpriseIntent).Code)

	ctx, listRecorder := appGinContext(http.MethodGet, "/api/v1/mobile/devices", "")
	h.List(ctx)
	require.Equal(t, http.StatusOK, listRecorder.Code)
	var listed struct {
		Data []struct {
			DeviceID string `json:"device_id"`
			AppID    string `json:"app_id"`
			Revision int64  `json:"revision"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listRecorder.Body.Bytes(), &listed))
	require.Len(t, listed.Data, 2, "one row per app for the same physical device")
	apps := map[string]int64{}
	for _, row := range listed.Data {
		apps[row.AppID] = row.Revision
	}
	require.EqualValues(t, 1, apps["official"])
	require.EqualValues(t, 1, apps["enterprise:acme"], "the enterprise bind must not take over the official row")
}

func TestRegisterRejectsUndeclaredEnterpriseApp(t *testing.T) {
	h := appDeviceHandler(t, "") // 未声明企业 App：仅 official

	ctx, recorder := appGinContext(http.MethodPost, "/api/v1/mobile/devices/d/registration-intent", `{"app_id":"enterprise:acme"}`)
	ctx.Params = gin.Params{{Key: "id", Value: "d"}}
	h.IssueIntent(ctx)
	require.Equal(t, http.StatusBadRequest, recorder.Code, "undeclared enterprise intents fail closed")

	require.Equal(t, http.StatusBadRequest,
		registerAppDevice(t, h, "d", "enterprise:acme", "t", "x").Code,
		"undeclared enterprise registrations fail closed")
	require.Equal(t, http.StatusBadRequest,
		registerAppDevice(t, h, "d", "Enterprise:ACME", "t", "x").Code,
		"malformed app ids are rejected")
}

func TestOfficialIntentCannotBindEnterpriseRow(t *testing.T) {
	h := appDeviceHandler(t, "enterprise:acme")
	officialIntent := issueAppIntent(t, h, "shared", "official")
	require.Equal(t, http.StatusConflict,
		registerAppDevice(t, h, "shared", "enterprise:acme", "tok", officialIntent).Code,
		"an official intent must not bind an enterprise registration")
}

func TestPresenceScopedByApp(t *testing.T) {
	h := appDeviceHandler(t, "enterprise:acme")
	for _, appID := range []string{"official", "enterprise:acme"} {
		intent := issueAppIntent(t, h, "shared", appID)
		require.Equal(t, http.StatusOK, registerAppDevice(t, h, "shared", appID, "tok-"+appID, intent).Code)
	}
	putCtx, putRecorder := appGinContext(http.MethodPut, "/api/v1/mobile/devices/shared/presence?app_id=enterprise:acme", "")
	putCtx.Params = gin.Params{{Key: "id", Value: "shared"}}
	h.PutPresence(putCtx)
	require.Equal(t, http.StatusOK, putRecorder.Code, putRecorder.Body.String())

	// official 行不受企业 presence 影响，且仍可按 app 独立读取。
	getCtx, getRecorder := appGinContext(http.MethodGet, "/api/v1/mobile/devices/shared/presence?app_id=official", "")
	getCtx.Params = gin.Params{{Key: "id", Value: "shared"}}
	h.GetPresence(getCtx)
	require.Equal(t, http.StatusOK, getRecorder.Code, getRecorder.Body.String())
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/handler/ -run 'TestRegisterAppID|TestRegisterRejectsUndeclared|TestOfficialIntentCannotBind|TestPresenceScopedByApp' -count=1`
Expected: FAIL——`undefined: WithMobileAppPolicy` 等编译错误。

- [ ] **Step 3: 最小实现**

修改 `internal/handler/mobile_device.go`：

(a) `MobileDeviceStore` 接口（`mobile_device.go:31-41`）改为 app 维度（既有调用方只剩本 handler 与容器具体实现）：

```go
type MobileDeviceStore interface {
	Bind(context.Context, repository.DeviceRegistration) error
	CurrentScopeGenerationForApp(context.Context, uint64, string, string, string) (int64, error)
	RevokeForApp(context.Context, uint64, string, string, string, int64) error
	GetActiveForApp(context.Context, uint64, string, string, string) (repository.DeviceRegistration, error)
	ListActiveForTenant(context.Context, uint64, string, string) ([]repository.DeviceRegistration, error)
	MarkPresenceForApp(context.Context, uint64, string, string, string, int64) error
	GetPresenceForApp(context.Context, uint64, string, string, string, int64) (repository.DeviceRegistration, error)
	SetPresenceForApp(context.Context, uint64, string, string, string, int64) (repository.DeviceRegistration, error)
	DeletePresenceForApp(context.Context, uint64, string, string, string, int64) error
}
```

(b) `MobileDeviceHandler`（`mobile_device.go:43-47`）加策略字段与类型（`mobileRegistrationIntent` 之后）：

```go
type MobileDeviceHandler struct {
	store       MobileDeviceStore
	environment string
	seal        MobileTokenSealer
	appPolicy   MobileAppPolicy
}

// MobileAppPolicy is the deployment app allowlist (story 67): the official app
// is always registrable; exactly one enterprise app id may be declared. A
// zero value admits the official app only.
type MobileAppPolicy struct{ EnterpriseAppID string }

func (h *MobileDeviceHandler) WithMobileAppPolicy(policy MobileAppPolicy) *MobileDeviceHandler {
	h.appPolicy = policy
	return h
}

func (h *MobileDeviceHandler) appAllowed(appID string) bool {
	if appID == repository.MobileAppIDOfficial {
		return true
	}
	if repository.ValidateMobileAppID(appID) != nil {
		return false
	}
	return appID == strings.TrimSpace(h.appPolicy.EnterpriseAppID)
}
```

(c) `mobileRegistrationIntent`（`mobile_device.go:49-56`）加 `AppID string \`json:"app"\`` 字段；`encodeRegistrationIntent` 在 marshal 前归一化（保证既有测试零改动）：

```go
	raw, err := json.Marshal(in)
```
→ 在其前加：
```go
	if in.AppID == "" {
		in.AppID = repository.MobileAppIDOfficial
	}
```

`verifyRegistrationIntent`（`mobile_device.go:74-101`）签名加 `appID string` 参数（放在 `device` 之后），解析后校验 `in.AppID != appID` 一并纳入既有失败分支（返回 `ErrMobileDeviceRevision`）。

(d) `mobileDeviceRequest`（`mobile_device.go:117-124`）加 `AppID string \`json:"app_id,omitempty"\``；`Register`（`mobile_device.go:178-241`）：

- `req.Platform` 归一化之后加：
```go
	req.AppID = repository.NormalizeMobileAppID(req.AppID)
	if !h.appAllowed(req.AppID) {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
```
- `h.store.CurrentScopeGeneration(...)` → `h.store.CurrentScopeGenerationForApp(c.Request.Context(), tenant, owner, deviceID, req.AppID)`；
- `verifyRegistrationIntent(..., device, current)` → `verifyRegistrationIntent(..., device, req.AppID, current)`；
- `row := repository.DeviceRegistration{...}` 字面量加 `AppID: req.AppID`；
- 成功响应 `gin.H` 加 `"app_id": active.AppID`；`GetActiveForTenant` → `GetActiveForApp(..., req.AppID)`。

(e) `IssueIntent`（`mobile_device.go:129-160`）解析可选 body（空 body 合法）：

```go
	var body struct {
		AppID string `json:"app_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil && !errors.Is(err, io.EOF) {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	appID := repository.NormalizeMobileAppID(body.AppID)
	if !h.appAllowed(appID) {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
```

（import 加 `"io"`。）`CurrentScopeGeneration` 调用改 `CurrentScopeGenerationForApp(..., appID)`；`encodeRegistrationIntent(mobileRegistrationIntent{...})` 加 `AppID: appID`；响应 `gin.H` 加 `"app_id": appID`。

(f) `Revoke`（`mobile_device.go:243-278`）读 `c.Query("app_id")`（缺省 official、同样过 `appAllowed`，否则 400），`RevokeForTenant` → `RevokeForApp(..., appID, revision)`，`CurrentScopeGeneration` → `CurrentScopeGenerationForApp`。

(g) presence 四方法（`mobile_device.go:280-377`）：统一在方法开头读 `appID := repository.NormalizeMobileAppID(c.Query("app_id"))` + `appAllowed` 校验（不合法 400），store 调用改 `...ForApp(..., appID, ...)`；`presenceJSON`（`mobile_device.go:314-319`）的 `gin.H` 加 `"app_id": row.AppID`。

修改 `internal/container/workbench.go:123-125`：

```go
func NewMobileDeviceHandler(store *repository.MobileDeviceStore) *handler.MobileDeviceHandler {
	enterpriseApp := strings.TrimSpace(os.Getenv("MOBILE_ENTERPRISE_APP_ID"))
	if enterpriseApp != "" && repository.ValidateMobileAppID(enterpriseApp) != nil {
		enterpriseApp = "" // 非法声明 fail closed：仅 official 通道
	}
	return handler.NewMobileDeviceHandler(store, mobileEnvironment()).
		WithMobileAppPolicy(handler.MobileAppPolicy{EnterpriseAppID: enterpriseApp})
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/handler/ -run 'TestMobileDevice|TestRegisterAppID|TestRegisterRejectsUndeclared|TestOfficialIntentCannotBind|TestPresenceScopedByApp' -count=1 && go build ./...`
Expected: PASS（含 #41 既有 `TestMobileDeviceHandlerRequiresAuthAndScopesOwner`/`TestMobileDeviceHandlerRejectsGuessedFutureEpoch`——`encodeRegistrationIntent` 的归一化保证旧意图编码路径不变）+ 编译通过。

- [ ] **Step 5: Commit**

```bash
git add internal/handler/mobile_device.go internal/handler/mobile_device_app_test.go internal/container/workbench.go
git commit -m "feat(mobile-push): app_id on device registration wire with deployment allowlist (T37 #67 task 4)"
```

---

### Task 5: 端到端——真实迁移子集 + 真实 handler/projector/worker + httptest 真实 Provider 边界（AC1/AC3 服务端证据）

**Files:**
- Test: `internal/application/repository/mobile_push_isolation_test.go`（`package repository_test`，外部测试包——同 `task_collaboration_http_test.go:1` 先例，可 import handler 而 import 无环）

**Interfaces:**
- Consumes: Task 1 全部、Task 2 `NewApnsProviderWithClient`/`NewStaticApnsTokenSource`、Task 3 `NewAppRoutingNotificationProvider`/`NewHTTPNotificationProviderWithPolicy`/`NewPushNotificationProviderWithOptions`/`PushPayloadPolicy`、Task 4 `WithMobileAppPolicy`；`session`/`handler` 包不需要（设备面 handler 在 `handler` 包）。
- Produces: 无（纯证据测试）。

- [ ] **Step 1: 写测试（本任务为证据聚合，先写全再跑）**

创建 `internal/application/repository/mobile_push_isolation_test.go`：

```go
package repository_test

// T37 (#67) end-to-end evidence at the highest stable interface. Everything
// here is real: focused sqlite migration files (the full migration dir is
// broken at HEAD by duplicate 000112 from #42/#59 — see the plan's difference
// record), the real MobileDeviceHandler (two-step intent→register over gin),
// the real NotificationStore projector and delivery worker, and real HTTP
// provider boundaries (gateway + APNs shape) behind httptest. No mocked
// service, no hand-written projection.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/handler"
	workbenchservice "github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench"
	pushnotification "github.com/Tencent/WeKnora/internal/modules/workbench/notification"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type pushIsolationEnv struct {
	db        *gorm.DB
	engine    *gin.Engine
	gateway   *httptest.Server
	gwBodies  []map[string]any
	apns      *httptest.Server
	apnsSeen  []apnsRequest
}

type apnsRequest struct {
	path     string
	pushType string
	body     map[string]any
}

const enterpriseTokenHex = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"

func newPushIsolationEnv(t *testing.T) *pushIsolationEnv {
	t.Helper()
	t.Setenv("SYSTEM_AES_KEY", "12345678901234567890123456789012")
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "push-isolation.db") + "?_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	// 顺序固定：000058 → 000059 → 000060 → 000114（000114 的重建段 INSERT...SELECT
	// 依赖前序迁移建出的 mobile_devices 与 mobile_notification_intents）。
	for _, file := range []string{
		"migrations/sqlite/000058_mobile_devices.up.sql",
		"migrations/sqlite/000059_mobile_notifications.up.sql",
		"migrations/sqlite/000060_mobile_notification_delivery.up.sql",
		"migrations/sqlite/000114_mobile_device_app.up.sql",
	} {
		raw, err := os.ReadFile(filepath.Join(root, file))
		require.NoError(t, err, file)
		require.NoError(t, db.Exec(string(raw)).Error, file)
	}
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS mobile_notification_provider_state (provider_key TEXT PRIMARY KEY, paused INTEGER NOT NULL DEFAULT 0, reason TEXT NOT NULL DEFAULT '', alert_count INTEGER NOT NULL DEFAULT 0, paused_at DATETIME, recovered_at DATETIME, updated_at DATETIME NOT NULL)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, owner_id TEXT NOT NULL, PRIMARY KEY (tenant_id, run_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE agent_run_events (tenant_id INTEGER NOT NULL, run_id TEXT NOT NULL, seq INTEGER NOT NULL, attempt_id TEXT NOT NULL DEFAULT '', event_type TEXT NOT NULL, payload TEXT NOT NULL DEFAULT '{}', PRIMARY KEY (tenant_id, run_id, seq))`).Error)
	require.NoError(t, db.Exec("INSERT INTO agent_runs (tenant_id, run_id, owner_id) VALUES (1, 'r1', 'u1')").Error)
	require.NoError(t, db.Exec("INSERT INTO agent_run_events (tenant_id, run_id, seq, event_type, payload) VALUES (1, 'r1', 9, 'run_completed', '{}')").Error)

	devices := repository.NewMobileDeviceStore(db, "dev")
	deviceHandler := handler.NewMobileDeviceHandlerWithSealer(devices, "dev", func(token string) (string, error) {
		return utils.EncryptAESGCM(token, utils.GetAESKey())
	}).WithMobileAppPolicy(handler.MobileAppPolicy{EnterpriseAppID: "enterprise:acme"})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	v1 := engine.Group("/api/v1")
	devicesGroup := v1.Group("/mobile/devices")
	devicesGroup.POST("/:id/registration-intent", deviceHandler.IssueIntent)
	devicesGroup.PUT("/:id", deviceHandler.Register)
	devicesGroup.DELETE("/:id", deviceHandler.Revoke)
	devicesGroup.GET("", deviceHandler.List)

	env := &pushIsolationEnv{db: db, engine: engine}
	env.gateway = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		env.gwBodies = append(env.gwBodies, body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"gw-receipt","status":"ok"}`))
	}))
	env.apns = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		env.apnsSeen = append(env.apnsSeen, apnsRequest{path: r.URL.Path, pushType: r.Header.Get("apns-push-type"), body: body})
		w.Header().Set("apns-unique-id", "apns-receipt")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(func() {
		env.gateway.Close()
		env.apns.Close()
		conn, _ := db.DB()
		_ = conn.Close()
	})
	return env
}

func (e *pushIsolationEnv) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := req.Context()
	ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

func (e *pushIsolationEnv) registerApp(t *testing.T, device, appID, token string) {
	t.Helper()
	w := e.do(t, http.MethodPost, "/api/v1/mobile/devices/"+device+"/registration-intent", `{"app_id":"`+appID+`"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var intent struct {
		Data struct {
			RegistrationIntent string `json:"registration_intent"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &intent))
	w = e.do(t, http.MethodPut, "/api/v1/mobile/devices/"+device, `{"token":"`+token+`","platform":"ios","app_id":"`+appID+`","registration_intent":"`+intent.Data.RegistrationIntent+`"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestMobilePushIsolationOfficialAndEnterpriseNeverMix(t *testing.T) {
	env := newPushIsolationEnv(t)
	env.registerApp(t, "do1", "official", "ExponentPushToken[official]")
	env.registerApp(t, "do2", "enterprise:acme", enterpriseTokenHex)

	// 官方注册不被企业注册干扰（AC1：注册不混用）。
	w := env.do(t, http.MethodGet, "/api/v1/mobile/devices", "")
	require.Equal(t, http.StatusOK, w.Code)
	var listed struct {
		Data []struct {
			DeviceID string `json:"device_id"`
			AppID    string `json:"app_id"`
			Revision int64  `json:"revision"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listed))
	require.Len(t, listed.Data, 2)
	for _, row := range listed.Data {
		require.EqualValues(t, 1, row.Revision, "no cross-app takeover: both rows keep revision 1")
	}

	// 投影：一个事件按 App 扇出到两个 intent（真实 NotificationStore projector）。
	store := repository.NewNotificationStore(env.db)
	require.NoError(t, store.ProjectEvent(context.Background(), repository.RunNotificationEvent{
		TenantID: 1, OwnerID: "u1", RunID: "r1", Seq: 9, Type: "run_completed",
	}))

	// 投递：official → HTTP gateway（非盲推，含 kind）；enterprise → APNs（盲推，
	// background content-available，无 kind/title）。真实 worker + 真实 HTTP 边界。
	devices := repository.NewMobileDeviceStore(env.db, "dev")
	health := repository.NewNotificationProviderStateStore(env.db)
	enterpriseDirect := workbenchservice.NewPushNotificationProviderWithOptions(
		pushnotification.NewApnsProviderWithClient(env.apns.URL, "bundle.acme.enterprise", pushnotification.NewStaticApnsTokenSource("enterprise-jwt"), env.apns.Client()),
		func(ctx context.Context, d repository.NotificationDelivery) (string, error) {
			registration, err := devices.GetActiveForApp(ctx, d.Intent.TenantID, d.Intent.OwnerID, d.Intent.DeviceID, "enterprise:acme")
			if err != nil {
				return "", err
			}
			return utils.DecryptAESGCM(registration.TokenCiphertext, utils.GetAESKey())
		},
		workbenchservice.PushPayloadPolicy{Blind: true})
	provider := workbenchservice.NewAppRoutingNotificationProvider(
		workbenchservice.NewHTTPNotificationProvider(env.gateway.URL),
		map[string]workbenchservice.NotificationProvider{"enterprise:acme": enterpriseDirect},
	)
	worker := workbenchservice.NewNotificationDeliveryWorkerWithHealth(store, provider, "isolation-worker", devices, health, "mobile")
	require.NoError(t, worker.RunOnce(context.Background(), 10))

	require.Len(t, env.gwBodies, 1, "the official token reaches only the deployment gateway")
	require.Equal(t, "do1", env.gwBodies[0]["device_id"])
	require.Equal(t, "official", env.gwBodies[0]["app_id"])
	require.Equal(t, "completed", env.gwBodies[0]["kind"])
	require.NotContains(t, env.gwBodies[0], "token", "token material never enters the gateway payload")

	require.Len(t, env.apnsSeen, 1, "the enterprise token reaches only the enterprise APNs lane")
	require.Equal(t, "/3/device/"+enterpriseTokenHex, env.apnsSeen[0].path)
	require.Equal(t, "background", env.apnsSeen[0].pushType, "enterprise lane runs blind (no lock-screen copy)")
	aps := env.apnsSeen[0].body["aps"].(map[string]any)
	require.Equal(t, float64(1), aps["content-available"])
	require.NotContains(t, aps, "alert")
	require.NotContains(t, env.apnsSeen[0].body, "kind")

	var states []struct {
		AppID string
		State string
	}
	require.NoError(t, env.db.Table("mobile_notification_intents").Select("app_id, state").Order("app_id").Find(&states).Error)
	require.Len(t, states, 2)
	for _, row := range states {
		require.Equal(t, "sent", row.State)
	}
}

func TestMobilePushDisabledKeepsIntentsDurable(t *testing.T) {
	env := newPushIsolationEnv(t)
	env.registerApp(t, "do1", "official", "ExponentPushToken[official]")
	store := repository.NewNotificationStore(env.db)
	require.NoError(t, store.ProjectEvent(context.Background(), repository.RunNotificationEvent{
		TenantID: 1, OwnerID: "u1", RunID: "r1", Seq: 9, Type: "run_completed",
	}))
	health := repository.NewNotificationProviderStateStore(env.db)
	worker := workbenchservice.NewNotificationDeliveryWorkerWithHealth(store, workbenchservice.NewDisabledNotificationProvider(), "disabled-e2e-worker", repository.NewMobileDeviceStore(env.db, "dev"), health, "mobile")
	require.NoError(t, worker.RunOnce(context.Background(), 10))
	require.Empty(t, env.gwBodies, "disabled policy never reaches the gateway")
	var pending int64
	require.NoError(t, env.db.Table("mobile_notification_intents").Where("state = 'pending'").Count(&pending).Error)
	require.EqualValues(t, 1, pending, "disabling push keeps intents durable for later policy recovery")
}
```

（`utils.EncryptAESGCM/DecryptAESGCM/GetAESKey` 即容器内 `secutils` 同一包 `internal/utils`——`container.go:1107-1111` 先例。）

- [ ] **Step 2: 运行（本任务断言为 GREEN 目标；若失败按失败信息修 Task 1-4 实现而非测试）**

Run: `go test ./internal/application/repository/ -run 'TestMobilePushIsolation|TestMobilePushDisabled' -count=1 -v`
Expected: PASS（2 个测试，全部走真实 handler/projector/worker/HTTP）。

- [ ] **Step 3: Commit**

```bash
git add internal/application/repository/mobile_push_isolation_test.go
git commit -m "test(mobile-push): end-to-end app isolation, blind enterprise lane and disabled durability (T37 #67 task 5)"
```

---

### Task 6: TS wire/domain——设备注册携带 appId（mobile-core + api-client）

**Files:**
- Modify: `packages/mobile-core/src/device/device-registry.ts`
- Modify: `packages/mobile-core/src/device/in-memory-device-remote.ts`
- Modify: `packages/api-client/src/mobile/devices.ts`
- Test: `packages/mobile-core/src/device/device-app-id.test.ts`
- Test: `packages/api-client/src/mobile/devices-app-id.test.ts`

**Interfaces:**
- Consumes: #41 的 `createDeviceRegistry`/`DeviceRemote`/`DeviceRegistrationRecord`（`device-registry.ts`）、`createMobileDeviceRemote`（`devices.ts`）。
- Produces（Task 7 依赖）:
  - `DeviceRemote.issueIntent(deviceId: string, appId?: string): Promise<{ registrationIntent: string; scopeGeneration: number }>`
  - `DeviceRemote.register(input: { deviceId; token; platform; registrationIntent; appId?: string }): Promise<DeviceRegistrationRecord>`
  - `DeviceRegistry.register(input: { deviceId; token; platform; appId?: string }): Promise<DeviceRegistrationRecord>`（缺省 `official`；格式非法 → `DEVICE_INVALID_INPUT`，零 wire 调用）
  - `DeviceRegistrationRecord.appId: string`（与 api-client `MobileDeviceRegistration.appId` 结构逐字一致）
  - wire 字段 `app_id`（issueIntent POST body / register PUT body / 响应行；响应缺省回落 `official`）
  - `createScenarioDeviceRemote()` 的行键改为 `${appId}:${deviceId}`（同 device 双 App 两行互不覆盖）

- [ ] **Step 1: 写失败测试**

创建 `packages/mobile-core/src/device/device-app-id.test.ts`（lease 构造与既有 `device-registry.test.ts:7-13` 的 `leased()` 逐字同形——`RuntimeScopeLease` 从 `../runtime/scope-lease.ts` 具名导入；本文件不 import 既有测试文件）：

```ts
import assert from 'node:assert/strict';
import test from 'node:test';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import { createDeviceRegistry, DeviceError } from './device-registry.ts';
import { createScenarioDeviceRemote } from './in-memory-device-remote.ts';

function leased() {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: '7' });
  return { revocable, lease: revocable.asScopeLease() };
}

/** 包一层捕获 issueIntent/register 的 app_id 入参（wire 透传观测）。 */
function capturingScenario() {
  const scenario = createScenarioDeviceRemote();
  const capturedApps: string[] = [];
  let lastRegisterInput: { appId?: string } | undefined;
  const remote = {
    issueIntent(deviceId: string, appId?: string) {
      capturedApps.push(appId ?? 'official');
      return scenario.remote.issueIntent(deviceId, appId);
    },
    register(input: { deviceId: string; token: string; platform: string; registrationIntent: string; appId?: string }) {
      lastRegisterInput = input;
      return scenario.remote.register(input);
    },
    revoke: scenario.remote.revoke.bind(scenario.remote),
    list: scenario.remote.list.bind(scenario.remote),
  };
  const leaseRef: { lease?: ReturnType<typeof leased>['lease'] } = { lease: leased().lease };
  const registry = createDeviceRegistry({ remote, lease: () => leaseRef.lease });
  return { scenario, registry, capturedApps: () => [...capturedApps], lastRegisterInput: () => lastRegisterInput };
}

test('register carries the app id on both wire calls and defaults to official', async () => {
  const { registry, capturedApps, lastRegisterInput } = capturingScenario();
  const record = await registry.register({ deviceId: 'd', token: 't', platform: 'ios' });
  assert.equal(record.appId, 'official');
  const enterprise = await registry.register({ deviceId: 'd2', token: 't2', platform: 'ios', appId: 'enterprise:acme' });
  assert.equal(enterprise.appId, 'enterprise:acme');
  assert.deepEqual(capturedApps(), ['official', 'enterprise:acme'], 'issueIntent receives the app id');
  assert.equal(lastRegisterInput()?.appId, 'enterprise:acme', 'register receives the app id');
});

test('invalid app ids fail closed before any wire call', async () => {
  const { registry, capturedApps } = capturingScenario();
  for (const appId of ['Official', 'enterprise', 'enterprise:', 'enterprise:Acme', 'enterprise:a_b', 'enterprise:-ab', `enterprise:${'x'.repeat(33)}`, 'ios']) {
    await assert.rejects(
      registry.register({ deviceId: 'd', token: 't', platform: 'ios', appId }),
      (error: unknown) => error instanceof DeviceError && error.code === 'DEVICE_INVALID_INPUT',
      `${JSON.stringify(appId)} must be rejected client-side`,
    );
  }
  assert.deepEqual(capturedApps(), [], 'validation happens before any wire call');
});

test('the same device id under two apps keeps two independent records', async () => {
  const { scenario, registry } = ((): ReturnType<typeof capturingScenario> => {
    const inner = createScenarioDeviceRemote();
    const leaseRef: { lease?: ReturnType<typeof leased>['lease'] } = { lease: leased().lease };
    return { scenario: inner, registry: createDeviceRegistry({ remote: inner.remote, lease: () => leaseRef.lease }), capturedApps: () => [], lastRegisterInput: () => undefined };
  })();
  const official = await registry.register({ deviceId: 'shared', token: 't1', platform: 'ios' });
  const enterprise = await registry.register({ deviceId: 'shared', token: 't2', platform: 'ios', appId: 'enterprise:acme' });
  assert.equal(official.revision, 1);
  assert.equal(enterprise.revision, 1, 'the enterprise register must not take over the official record');
  assert.deepEqual(
    scenario.snapshot().active.map((row) => `${row.appId}:${row.deviceId}`).sort(),
    ['enterprise:acme:shared', 'official:shared'],
    'two independent records keyed by app',
  );
});
```

（第三个用例直接用未包装的 scenario remote——IIFE 内联组装，不依赖 capture。）

创建 `packages/api-client/src/mobile/devices-app-id.test.ts`：

```ts
import assert from 'node:assert/strict';
import test from 'node:test';
import { createMobileDeviceRemote } from './devices.ts';

type Captured = { method: string; path: string; body?: unknown };

function capturingRemote(responses: Array<Record<string, unknown>>): { remote: ReturnType<typeof createMobileDeviceRemote>; calls: Captured[] } {
  const calls: Captured[] = [];
  let index = 0;
  const request = async (input: { method: string; path: string; body?: unknown }): Promise<unknown> => {
    calls.push({ method: input.method, path: input.path, body: input.body });
    const response = responses[Math.min(index, responses.length - 1)];
    index += 1;
    return { success: true, data: response };
  };
  return { remote: createMobileDeviceRemote({ origin: 'https://deployment.example', request }), calls };
}

const intentRow = { registration_intent: 'intent-1', scope_generation: 2 };
const registerRow = { device_id: 'd', platform: 'ios', environment: 'dev', scope_generation: 2, revision: 1, app_id: 'enterprise:acme' };

test('issueIntent and register carry app_id on the wire', async () => {
  const { remote, calls } = capturingRemote([intentRow, registerRow]);
  await remote.issueIntent('d', 'enterprise:acme');
  assert.deepEqual(calls[0], { method: 'POST', path: '/api/v1/mobile/devices/d/registration-intent', body: { app_id: 'enterprise:acme' } });
  await remote.register({ deviceId: 'd', token: 't', platform: 'ios', registrationIntent: 'intent-1', appId: 'enterprise:acme' });
  assert.deepEqual(
    (calls[1]!.body as Record<string, unknown>),
    { token: 't', platform: 'ios', registration_intent: 'intent-1', app_id: 'enterprise:acme' },
  );
});

test('register omits app_id when defaulted and records official from the response', async () => {
  const { remote, calls } = capturingRemote([registerRow]);
  const record = await remote.register({ deviceId: 'd', token: 't', platform: 'ios', registrationIntent: 'intent-1' });
  assert.equal('app_id' in (calls[0]!.body as Record<string, unknown>), false, 'official default stays implicit on the wire (server normalizes)');
  assert.equal(record.appId, 'enterprise:acme');
});

test('list maps app_id and falls back to official for legacy rows', async () => {
  const { remote } = capturingRemote([[{ device_id: 'd', platform: 'ios', environment: 'dev', revision: 1, scope_generation: 1 }, { device_id: 'd2', platform: 'android', environment: 'dev', revision: 1, scope_generation: 1, app_id: 'enterprise:acme' }]]);
  const rows = await remote.list();
  assert.equal(rows[0]!.appId, 'official');
  assert.equal(rows[1]!.appId, 'enterprise:acme');
});
```

（关于 `calls[0]!` 等下标断言——**结构性不依赖 tsconfig 旗标**：本测试所有下标访问均带 `!` 非空断言，无论 `noUncheckedIndexedAccess` 开或关均可编译。已核实的弱证据（如实记录，未穷举全部配置）：`apps/mobile/tsconfig.json` 仅 `strict`/`noEmit`/`allowImportingTsExtensions` 且 extends `expo/tsconfig.base`，其中未 grep 到该 flag；`packages/mobile-core` 与 `packages/api-client` 无自有 tsconfig（实查 `ls` 报 No such file）；`expo/tsconfig.base` 本体未读取核实。typecheck 以 `pnpm --filter @weknora/mobile typecheck` 实跑为准。）

- [ ] **Step 2: 运行确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/device/device-app-id.test.ts packages/api-client/src/mobile/devices-app-id.test.ts`
Expected: FAIL——`appId` 不在 `DeviceRegistrationRecord`/`register` 入参上（TS 运行时按属性缺失断言失败），`capturedApps`/wire `app_id` 未实现。

- [ ] **Step 3: 最小实现**

修改 `packages/mobile-core/src/device/device-registry.ts`：

- `DeviceRegistrationRecord`（`device-registry.ts:17-25`）加 `appId: string;`（`deviceId` 之前）。
- `DeviceRemote`（`device-registry.ts:27-32`）：

```ts
export interface DeviceRemote {
  issueIntent(deviceId: string, appId?: string): Promise<{ registrationIntent: string; scopeGeneration: number }>;
  register(input: { deviceId: string; token: string; platform: string; registrationIntent: string; appId?: string }): Promise<DeviceRegistrationRecord>;
  revoke(input: { deviceId: string; revision?: number; appId?: string }): Promise<void>;
  list(): Promise<DeviceRegistrationRecord[]>;
}
```

- 文件头注释块后加 App id 校验（与服务端 `ValidateMobileAppID` 同字符集）：

```ts
const enterpriseAppIdPattern = /^enterprise:[a-z0-9][a-z0-9-]{0,31}$/;
/** 与服务端 repository.ValidateMobileAppID 同字符集；official 恒合法。 */
export function isValidDeviceAppId(appId: string): boolean {
  return appId === 'official' || enterpriseAppIdPattern.test(appId);
}
```

- `attemptRegister`（`device-registry.ts:91-102`）签名加 `appId: string`：`ports.remote.issueIntent(deviceId, appId)` 与 `ports.remote.register({ deviceId, token, platform, registrationIntent: intent.registrationIntent, appId })`。
- `register`（`device-registry.ts:104-128`）：入口归一化 `const appId = input.appId === undefined || input.appId.trim() === '' ? 'official' : input.appId.trim();`，非法（`!isValidDeviceAppId(appId)`）→ `DEVICE_INVALID_INPUT`（与 deviceId/token 校验同位置，零 wire 调用）；`attemptRegister(lease, deviceId, token, platform, appId)`。
- `revoke`（`device-registry.ts:130-147`）：`appId` 可选透传 `ports.remote.revoke({ deviceId, ...(input.revision === undefined ? {} : { revision: input.revision }), ...(appId === undefined ? {} : { appId }) })`。
- `assertSoundRecord` 不变（`appId` 由服务端行保证）。

修改 `packages/mobile-core/src/device/in-memory-device-remote.ts`：行键 `rows` 从 `Map<string, ...>` 键 `input.deviceId` 改为 `${appId}:${deviceId}`（`appId = input.appId ?? 'official'`，issueIntent 与 register/revoke 两端一致计算）；`issueIntent(deviceId, appId?)` 产 intent 串 `intent:${appId}:${deviceId}:${epoch}`；`register` 校验 intent 串前缀匹配（`intent:${appId}:${input.deviceId}:${epoch}`）、记录行携带 `appId`；`revoke` 按 `${appId}:${deviceId}` 查行（`input.appId ?? 'official'`）。观测（capturedApps 等）全部在测试文件的包装层完成，Adapter 不新增观测面。

修改 `packages/api-client/src/mobile/devices.ts`：

- `MobileDeviceRegistration`（`devices.ts:17-25`）加 `appId: string;`（注释保持「与 mobile-core DeviceRegistrationRecord 结构逐字一致」）。
- `issueIntent`（`devices.ts:66-75`）签名加 `appId?: string`，请求体 `body: appId === undefined ? undefined : { app_id: appId }`；响应解析不变。
- `register`（`devices.ts:76-96`）入参加 `appId?: string`；body 的 `app_id` 仅在提供时携带（`...(input.appId === undefined ? {} : { app_id: input.appId })`）；返回行 `appId: typeof data.app_id === 'string' && data.app_id.trim() !== '' ? data.app_id : 'official'`。
- `revoke`（`devices.ts:97-103`）入参加 `appId?: string`；query 改为分段拼接（`?` 前缀只在有任一参数时出现）：

```ts
    async revoke(input: { deviceId: string; revision?: number; appId?: string }): Promise<void> {
      if (input.revision !== undefined && (!Number.isSafeInteger(input.revision) || input.revision <= 0)) {
        throw new Error('device revision must be a positive integer');
      }
      const query = new URLSearchParams();
      if (input.revision !== undefined) query.set('revision', String(input.revision));
      if (input.appId !== undefined) query.set('app_id', input.appId);
      const suffix = query.size > 0 ? `?${query.toString()}` : '';
      await request({ method: 'DELETE', path: `${devicePath(input.deviceId)}${suffix}` }); // 204：无 body 可解
    },
```
- `list`（`devices.ts:104-125`）行映射加 `appId: typeof row.app_id === 'string' && row.app_id.trim() !== '' ? row.app_id : 'official'`。

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test packages/mobile-core/src/device/device-registry.test.ts packages/mobile-core/src/device/device-app-id.test.ts packages/api-client/src/mobile/devices.test.ts packages/api-client/src/mobile/devices-app-id.test.ts && pnpm --filter @weknora/mobile typecheck`
Expected: 全部 pass（含 #41 既有 device-registry/devices 用例零回归——可选参数向后兼容）+ typecheck 通过。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/device/device-registry.ts packages/mobile-core/src/device/in-memory-device-remote.ts packages/mobile-core/src/device/device-app-id.test.ts packages/api-client/src/mobile/devices.ts packages/api-client/src/mobile/devices-app-id.test.ts
git commit -m "feat(mobile-push): app id passthrough on device registration wire (T37 #67 task 6)"
```

---

### Task 7: apps/mobile——App 身份解析、前台权威同步环与 opt-in 集成冒烟（AC1 客户端半边 + AC2）

**Files:**
- Create: `apps/mobile/src/app-id.ts`
- Create: `apps/mobile/src/adapters/app-state.ts`
- Create: `apps/mobile/src/foreground-sync.ts`
- Modify: `apps/mobile/src/composition.ts`
- Create: `apps/mobile/src/blind-push-integration-smoke.ts`
- Test: `apps/mobile/src/app-id.test.ts`
- Test: `apps/mobile/src/foreground-sync.test.ts`
- Test: `apps/mobile/src/blind-push-integration-smoke.test.ts`

**Interfaces:**
- Consumes: Task 6 的 `DeviceRegistry.register({ ..., appId? })`；#41 的 `notificationInboxFor`/`createNotificationInbox`（`page()` 权威重投影）；#32/#66 的 `RuntimeSnapshot.surface === 'authorized'`、`snapshot.deployment.origin`、`snapshot.identity.activeTenantId`；`composition.ts:58` 的 `process.env.EXPO_PUBLIC_*` 构建期环境变量先例；`device-inbox-integration-smoke.ts` 的 `disallowedDeploymentHost` 主机防线与 opt-in 配置模式。
- Produces:
  - `export const OFFICIAL_WEKNORA_APP_ID = 'official'`；`isValidWeKnoraAppId(appId: string): boolean`；`resolveWeKnoraAppId(raw: string | undefined): string`（未配置/非法回落 official——fail closed 到官方通道）
  - `createNativeAppStateLifecycle(): { subscribe(listener: (state: 'active' | 'background') => void): () => void }`（无 AppState 时 no-op）
  - `createForegroundSyncLoop(ports: { lifecycle; resolveSync(): (() => Promise<void>) | undefined }): { start(): () => void }`
  - composition：`registerActiveDeviceIfPossible` 内部解析 `resolveWeKnoraAppId(process.env.EXPO_PUBLIC_WEKNORA_APP_ID)` 并随 register 透传；`MobileApp` 挂前台同步环
  - `blindPushIntegrationConfig(env): BlindPushIntegrationConfig`（复用 `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD` + 可选 `WEKNORA_MOBILE_TEST_ENTERPRISE_APP_ID`，主机防线复用 `disallowedDeploymentHost`）
  - `runBlindPushIntegration(config): Promise<BlindPushIntegrationEvidence>`、`emitBlindPushIntegrationEvidence(evidence, diagnostic)`；证据字段 `officialRegister: 'registered' | 'failed'`、`enterpriseRegister: 'registered' | 'rejected-by-policy' | 'failed'`、`isolation: 'isolated' | 'mixed' | 'unverified'`、`foregroundSync: 'synced' | 'failed'`、`inboxUnreadCount?: number`、`deploymentOrigin: string`、`commandTimestamp: string`

- [ ] **Step 1: 写失败测试**

创建 `apps/mobile/src/app-id.test.ts`：

```ts
import assert from 'node:assert/strict';
import test from 'node:test';
import { isValidWeKnoraAppId, resolveWeKnoraAppId, OFFICIAL_WEKNORA_APP_ID } from './app-id.ts';

test('valid app ids are official or enterprise slugs', () => {
  assert.equal(isValidWeKnoraAppId('official'), true);
  assert.equal(isValidWeKnoraAppId('enterprise:acme'), true);
  assert.equal(isValidWeKnoraAppId('enterprise:a1-b2'), true);
  for (const invalid of ['', 'Official', 'enterprise', 'enterprise:', 'enterprise:Acme', 'enterprise:a_b', `enterprise:${'x'.repeat(33)}`, 'ios']) {
    assert.equal(isValidWeKnoraAppId(invalid), false, JSON.stringify(invalid));
  }
});

test('resolveWeKnoraAppId falls back to official when unset or malformed', () => {
  assert.equal(resolveWeKnoraAppId(undefined), OFFICIAL_WEKNORA_APP_ID);
  assert.equal(resolveWeKnoraAppId('  '), OFFICIAL_WEKNORA_APP_ID);
  assert.equal(resolveWeKnoraAppId('not-an-app'), OFFICIAL_WEKNORA_APP_ID, 'a misconfigured build fails closed to the official lane');
  assert.equal(resolveWeKnoraAppId(' enterprise:acme '), 'enterprise:acme');
});
```

创建 `apps/mobile/src/foreground-sync.test.ts`：

```ts
import assert from 'node:assert/strict';
import test from 'node:test';
import { createForegroundSyncLoop } from './foreground-sync.ts';

function scriptedLifecycle() {
  const listeners = new Set<(state: 'active' | 'background') => void>();
  return {
    lifecycle: {
      subscribe(listener: (state: 'active' | 'background') => void): () => void {
        listeners.add(listener);
        return () => listeners.delete(listener);
      },
    },
    emit(state: 'active' | 'background') { for (const listener of [...listeners]) listener(state); },
  };
}

test('returning to the foreground triggers one authoritative sync', async () => {
  const script = scriptedLifecycle();
  const syncs: number[] = [];
  let gate: (() => void) | undefined;
  const loop = createForegroundSyncLoop({
    lifecycle: script.lifecycle,
    resolveSync: () => async () => { syncs.push(syncs.length); gate?.(); },
  });
  const stop = loop.start();
  script.emit('background');
  assert.equal(syncs.length, 0, 'background events never sync');
  script.emit('active');
  await new Promise<void>((resolve) => { gate = resolve; });
  assert.equal(syncs.length, 1);
  stop();
});

test('concurrent active events coalesce into one follow-up, never parallel syncs', async () => {
  const script = scriptedLifecycle();
  let inflight = 0;
  let maxInflight = 0;
  let release: (() => void) | undefined;
  const loop = createForegroundSyncLoop({
    lifecycle: script.lifecycle,
    resolveSync: () => async () => {
      inflight += 1;
      maxInflight = Math.max(maxInflight, inflight);
      await new Promise<void>((resolve) => { release = resolve; });
      inflight -= 1;
    },
  });
  loop.start();
  script.emit('active');
  script.emit('active');
  script.emit('active');
  await new Promise<void>((resolve) => setTimeout(resolve, 0));
  assert.equal(maxInflight, 1, 'syncs are serialized');
  release?.();
  await new Promise<void>((resolve) => setTimeout(resolve, 10));
  assert.ok(maxInflight <= 1, 'follow-up completed without parallel sync or unhandled rejection');
});

test('an unauthorized scope resolves undefined and never syncs', async () => {
  const script = scriptedLifecycle();
  let called = 0;
  const loop = createForegroundSyncLoop({
    lifecycle: script.lifecycle,
    resolveSync: () => { called += 1; return undefined; },
  });
  loop.start();
  script.emit('active');
  await new Promise<void>((resolve) => setTimeout(resolve, 0));
  assert.equal(called, 1, 'resolveSync is consulted per event');
});

test('sync failures are contained and the next foreground retries', async () => {
  const script = scriptedLifecycle();
  let attempts = 0;
  const loop = createForegroundSyncLoop({
    lifecycle: script.lifecycle,
    resolveSync: () => () => { attempts += 1; return attempts === 1 ? Promise.reject(new Error('offline')) : Promise.resolve(); },
  });
  loop.start();
  script.emit('active');
  await new Promise<void>((resolve) => setTimeout(resolve, 0));
  script.emit('active');
  await new Promise<void>((resolve) => setTimeout(resolve, 0));
  assert.equal(attempts, 2, 'the failed sync did not poison the loop');
});

test('stop unsubscribes the lifecycle listener', async () => {
  const script = scriptedLifecycle();
  let called = 0;
  const loop = createForegroundSyncLoop({ lifecycle: script.lifecycle, resolveSync: () => { called += 1; return () => Promise.resolve(); } });
  const stop = loop.start();
  stop();
  script.emit('active');
  await new Promise<void>((resolve) => setTimeout(resolve, 0));
  assert.equal(called, 0);
});
```

创建 `apps/mobile/src/blind-push-integration-smoke.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  blindPushIntegrationConfig,
  emitBlindPushIntegrationEvidence,
  runBlindPushIntegration,
} from './blind-push-integration-smoke.ts';

/**
 * Opt-in 真实 HTTP 检查（#67 AC1 客户端观察 + AC2——真实 JSON transport + 真实 Runtime +
 * 真实服务端注册面/inbox 读）。无回退凭据：
 *
 * WEKNORA_MOBILE_TEST_DEPLOYMENT_URL=https://deployment.example \
 * WEKNORA_MOBILE_TEST_EMAIL=mobile-test@example.test \
 * WEKNORA_MOBILE_TEST_PASSWORD=<short-lived-secret> \
 * [WEKNORA_MOBILE_TEST_ENTERPRISE_APP_ID=enterprise:acme] \
 * pnpm --filter @weknora/mobile test
 *
 * 占位 token 不是可用凭据（无 APNs/FCM 真实效力）；凭据一律来自环境变量。
 */
test('real HTTP official/enterprise registration isolation and foreground sync through the wire', async (t) => {
  const config = blindPushIntegrationConfig(process.env);
  if (!config.enabled) {
    if (config.disposition === 'skip') t.skip(`BLIND_PUSH_HTTP_SKIPPED: ${config.reason}`);
    else assert.fail(`BLIND_PUSH_HTTP_INVALID: ${config.reason}`);
    return;
  }
  const evidence = await runBlindPushIntegration(config);
  emitBlindPushIntegrationEvidence(evidence, (record) => t.diagnostic(record));
  assert.equal(evidence.officialRegister, 'registered', 'the official two-step registration must bind');
  assert.equal(evidence.foregroundSync, 'synced', 'foreground authoritative sync works with no push involvement');
  if (config.enterpriseAppId !== undefined) {
    assert.equal(evidence.enterpriseRegister, 'registered', 'the declared enterprise app must register under its own identity');
    assert.equal(evidence.isolation, 'isolated', 'official and enterprise registrations must not mix');
  } else {
    assert.ok(
      evidence.enterpriseRegister === 'rejected-by-policy' || evidence.enterpriseRegister === 'registered',
      'an undeclared probe is either policy-rejected (400) or registered on a deployment that allows it—both are honest observations',
    );
  }
});

test('blind-push config reuses the device-inbox credentials and host guard', () => {
  assert.deepEqual(
    blindPushIntegrationConfig({}),
    { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' },
  );
  const withCredentials = {
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://deployment.example',
    WEKNORA_MOBILE_TEST_EMAIL: 'mobile-test@example.test',
    WEKNORA_MOBILE_TEST_PASSWORD: 'pw',
  };
  for (const url of ['https://127.0.0.1', 'https://10.0.0.2', 'https://localhost']) {
    const config = blindPushIntegrationConfig({ ...withCredentials, WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: url });
    assert.equal(config.enabled, false, `${url} must not enable a real HTTP run`);
    assert.equal(config.disposition, 'invalid');
  }
  const enabled = blindPushIntegrationConfig(withCredentials);
  assert.equal(enabled.enabled, true);
  const enterprise = blindPushIntegrationConfig({ ...withCredentials, WEKNORA_MOBILE_TEST_ENTERPRISE_APP_ID: 'enterprise:acme' });
  assert.equal(enterprise.enabled, true);
  if (enterprise.enabled) assert.equal(enterprise.enterpriseAppId, 'enterprise:acme');
  const malformed = blindPushIntegrationConfig({ ...withCredentials, WEKNORA_MOBILE_TEST_ENTERPRISE_APP_ID: 'enterprise:Acme' });
  assert.equal(malformed.enabled, true, 'a malformed enterprise id degrades to official-only probing, still enabled');
});

test('emitted evidence carries no credential fields', () => {
  const emitted: string[] = [];
  emitBlindPushIntegrationEvidence({
    deploymentOrigin: 'https://deployment.example',
    officialRegister: 'registered',
    enterpriseRegister: 'rejected-by-policy',
    isolation: 'unverified',
    foregroundSync: 'synced',
    inboxUnreadCount: 0,
    commandTimestamp: '2026-09-24T00:00:00.000Z',
  }, (record) => emitted.push(record));
  assert.equal(emitted.length, 1);
  assert.doesNotMatch(emitted[0]!, /short-lived-secret|mobile-test@|password|Bearer\s/i, 'evidence must stay credential-free');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/app-id.test.ts apps/mobile/src/foreground-sync.test.ts apps/mobile/src/blind-push-integration-smoke.test.ts`
Expected: FAIL——模块不存在（`Cannot find module './app-id.ts'` 等）。

- [ ] **Step 3: 最小实现**

创建 `apps/mobile/src/app-id.ts`：

```ts
/** 构建期 App 身份（T37 #67）：官方标准客户端恒为 official；企业自构建构建以
 * EXPO_PUBLIC_WEKNORA_APP_ID 声明 enterprise:<slug>。非法值回落 official——
 * fail closed 到官方通道，企业注册只会在服务端策略允许时发生。 */
export const OFFICIAL_WEKNORA_APP_ID = 'official';
const ENTERPRISE_APP_ID_PATTERN = /^enterprise:[a-z0-9][a-z0-9-]{0,31}$/;

export function isValidWeKnoraAppId(appId: string): boolean {
  return appId === OFFICIAL_WEKNORA_APP_ID || ENTERPRISE_APP_ID_PATTERN.test(appId);
}

export function resolveWeKnoraAppId(raw: string | undefined): string {
  const trimmed = typeof raw === 'string' ? raw.trim() : '';
  return isValidWeKnoraAppId(trimmed) ? trimmed : OFFICIAL_WEKNORA_APP_ID;
}
```

创建 `apps/mobile/src/adapters/app-state.ts`：

```ts
/** 原生生命周期 Adapter：react-native AppState('change') 映射为 active/background。
 * Node 测试环境/无 AppState 时订阅为 no-op（fail closed：无自动前台同步，手动刷新仍在）。 */
export function createNativeAppStateLifecycle(): {
  subscribe(listener: (state: 'active' | 'background') => void): () => void;
} {
  return {
    subscribe(listener) {
      try {
        const reactNative = require('react-native') as {
          AppState?: { addEventListener?: (event: 'change', handler: (state: string) => void) => { remove(): void } };
        };
        if (typeof reactNative.AppState?.addEventListener !== 'function') return () => undefined;
        const subscription = reactNative.AppState.addEventListener('change', (state: string) => {
          listener(state === 'active' ? 'active' : 'background');
        });
        return () => subscription.remove();
      } catch {
        return () => undefined;
      }
    },
  };
}
```

创建 `apps/mobile/src/foreground-sync.ts`：

```ts
export interface ForegroundSyncLifecycle {
  subscribe(listener: (state: 'active' | 'background') => void): () => void;
}

export interface ForegroundSyncPorts {
  lifecycle: ForegroundSyncLifecycle;
  /** 每次前台事件时解析当前授权 scope 的权威同步动作；未授权/无 scope 返回 undefined（跳过）。 */
  resolveSync(): (() => Promise<void>) | undefined;
}

export interface ForegroundSyncLoop {
  start(): () => void;
}

/**
 * 前台权威同步环（#67 AC2）：推送网关关闭/无推送凭据时，回到前台即向权威服务端重新
 * 拉取（通知 Inbox page() 权威重投影——module-seams §5.3：推送只触发重新同步，权威
 * 状态永远在服务端）。
 * - 单飞合并：同步进行中收到新的 active 事件不并发，只记一次待执行；
 * - 失败包含：同步失败只吞掉（下次前台再试），绝不 unhandled rejection；
 * - scope 每次事件重新解析：切租户/换部署/登出后的旧闭包不再被引用。
 */
export function createForegroundSyncLoop(ports: ForegroundSyncPorts): ForegroundSyncLoop {
  let running = false;
  let followUpPending = false;
  let started = false;
  const runOnce = async (): Promise<void> => {
    running = true;
    try {
      const sync = ports.resolveSync();
      if (sync !== undefined) await sync();
    } catch {
      // 失败包含：下一次前台事件重试
    } finally {
      running = false;
      if (followUpPending) {
        followUpPending = false;
        void runOnce();
      }
    }
  };
  return {
    start() {
      if (started) return () => undefined;
      started = true;
      const unsubscribe = ports.lifecycle.subscribe((state) => {
        if (state !== 'active') return;
        if (running) {
          followUpPending = true;
          return;
        }
        void runOnce();
      });
      return () => { unsubscribe(); };
    },
  };
}
```

创建 `apps/mobile/src/blind-push-integration-smoke.ts`（结构与 `device-inbox-integration-smoke.ts` 同模式，自包含不 import 其配置解析；主机防线复用 `disallowedDeploymentHost`）：

```ts
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileDeviceRemote } from '@weknora/api-client/mobile/devices';
import { createMobileInboxRemote } from '@weknora/api-client/mobile/inbox';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createDeviceRegistry, createInMemoryCredentialStore, createMobileRuntime, createNotificationInbox, DeviceError } from '@weknora/mobile-core';
import { isValidWeKnoraAppId } from './app-id.ts';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';

export type BlindPushIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string; enterpriseAppId?: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface BlindPushIntegrationEvidence {
  deploymentOrigin: string;
  officialRegister: 'registered' | 'failed';
  enterpriseRegister: 'registered' | 'rejected-by-policy' | 'failed';
  isolation: 'isolated' | 'mixed' | 'unverified';
  foregroundSync: 'synced' | 'failed';
  inboxUnreadCount?: number;
  commandTimestamp: string;
}

export function blindPushIntegrationConfig(env: Record<string, string | undefined>): BlindPushIntegrationConfig {
  const deploymentOrigin = env.WEKNORA_MOBILE_TEST_DEPLOYMENT_URL?.trim();
  const email = env.WEKNORA_MOBILE_TEST_EMAIL?.trim();
  const password = env.WEKNORA_MOBILE_TEST_PASSWORD;
  if (!deploymentOrigin || !email || !password) {
    return { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' };
  }
  let parsed: URL;
  try {
    parsed = new URL(deploymentOrigin);
  } catch {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL is not an absolute URL' };
  }
  if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
  }
  const hostRejection = disallowedDeploymentHost(parsed.hostname, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL');
  if (hostRejection) return { enabled: false, disposition: 'invalid', reason: hostRejection };
  const rawEnterprise = env.WEKNORA_MOBILE_TEST_ENTERPRISE_APP_ID?.trim();
  // 非法企业 id 静默降级为「未声明」：冒烟只做官方通道 + 策略探测。
  const enterpriseAppId = rawEnterprise !== undefined && isValidWeKnoraAppId(rawEnterprise) && rawEnterprise !== 'official' ? rawEnterprise : undefined;
  return { enabled: true, deploymentOrigin: parsed.origin, email, password, ...(enterpriseAppId === undefined ? {} : { enterpriseAppId }) };
}

function wireStatus(error: unknown): number | undefined {
  if (typeof error !== 'object' || error === null) return undefined;
  if ((error as { name?: unknown }).name !== 'ApiError') return undefined;
  const status = (error as { status?: unknown }).status;
  return typeof status === 'number' ? status : undefined;
}

/** 真实 transport + Runtime + 两步注册（#67 AC1 客户端观察 + AC2 前台权威同步）。 */
export async function runBlindPushIntegration(config: Extract<BlindPushIntegrationConfig, { enabled: true }>): Promise<BlindPushIntegrationEvidence> {
  const evidence: BlindPushIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    officialRegister: 'failed',
    enterpriseRegister: 'failed',
    isolation: 'unverified',
    foregroundSync: 'failed',
    commandTimestamp: new Date().toISOString(),
  };
  const fetcher: FetchLike = (input, init) => fetch(input, init as RequestInit);
  const runtime = createMobileRuntime({
    credentialStore: createInMemoryCredentialStore(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    remoteFor(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return createMobileRuntimeRemote({ origin, request: client.request });
    },
    authorizedTransport(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return (input, accessToken) => client.request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
    },
  });
  const throughRuntime = (input: { method: string; path: string; headers?: Record<string, string>; body?: unknown }): Promise<unknown> =>
    runtime.authorizedRequest(input);
  const deviceId = 'blind-push-smoke-device';
  const registry = createDeviceRegistry({
    remote: createMobileDeviceRemote({ origin: config.deploymentOrigin, request: throughRuntime }),
    lease: () => runtime.scopeLease(),
  });
  try {
    const official = await registry.register({ deviceId, token: 'integration-placeholder-token', platform: 'ios' });
    evidence.officialRegister = 'registered';
    const officialRevision = official.revision;

    const enterpriseAppId = config.enterpriseAppId ?? 'enterprise:probe';
    try {
      const enterprise = await registry.register({ deviceId, token: 'integration-placeholder-token-enterprise', platform: 'ios', appId: enterpriseAppId });
      evidence.enterpriseRegister = 'registered';
      const rows = await registry.list();
      const mine = rows.filter((row) => row.deviceId === deviceId);
      const apps = new Set(mine.map((row) => row.appId));
      const officialRow = mine.find((row) => row.appId === 'official');
      evidence.isolation = mine.length >= 2 && apps.has('official') && apps.has(enterprise.appId) && officialRow?.revision === officialRevision
        ? 'isolated'
        : 'mixed';
    } catch (error) {
      // 部署未声明该企业 App（服务端允许清单拒绝 400）——隔离的另一半证据。
      const status = error instanceof DeviceError ? wireStatus((error as DeviceError & { cause?: unknown }).cause) : wireStatus(error);
      evidence.enterpriseRegister = status === 400 ? 'rejected-by-policy' : 'failed';
      const rows = await registry.list().catch(() => []);
      evidence.isolation = rows.some((row) => row.deviceId === deviceId && row.appId !== 'official') ? 'mixed' : 'unverified';
    }
  } catch {
    // 任一步失败保留枚举失败值
  }

  // AC2：与推送完全无关的前台权威同步——同一授权 scope 下 inbox page() 必须可用。
  try {
    const snapshot = runtime.snapshot();
    const origin = snapshot.deployment?.origin ?? config.deploymentOrigin;
    const inbox = createNotificationInbox({
      remote: createMobileInboxRemote({ origin, request: throughRuntime }),
      lease: () => runtime.scopeLease(),
    });
    const view = await inbox.page();
    evidence.foregroundSync = 'synced';
    evidence.inboxUnreadCount = view.unreadCount;
  } catch {
    evidence.foregroundSync = 'failed';
  }
  return evidence;
}

export function emitBlindPushIntegrationEvidence(evidence: BlindPushIntegrationEvidence, diagnostic: (record: string) => void): void {
  diagnostic(`BLIND_PUSH_EVIDENCE ${JSON.stringify(evidence)}`);
}
```

修改 `apps/mobile/src/composition.ts`：

(a) import 区加：

```ts
import { resolveWeKnoraAppId } from './app-id.ts';
import { createNativeAppStateLifecycle } from './adapters/app-state.ts';
import { createForegroundSyncLoop } from './foreground-sync.ts';
```

(b) `registerActiveDeviceIfPossible`（`composition.ts:215-237`）的 register 调用携带 App 身份（在 `try {` 内）：

```ts
    await deviceRegistryFor(activeRuntime, origin, snapshot.identity?.activeTenantId ?? '').register({
      deviceId,
      token,
      platform: nativeDevicePlatform(),
      appId: resolveWeKnoraAppId(typeof process !== 'undefined' ? process.env.EXPO_PUBLIC_WEKNORA_APP_ID : undefined),
    });
```

(c) `MobileApp`（`composition.ts:344-384`）设备注册 effect 之后加前台权威同步 effect：

```ts
  // T37（#67 AC2）：关闭推送网关/无推送时，回前台即向权威服务端同步（通知 Inbox
  // page() 权威重投影）。scope 每次事件重新解析；失败包含（下次前台再试）。
  useEffect(() => {
    const stop = createForegroundSyncLoop({
      lifecycle: createNativeAppStateLifecycle(),
      resolveSync: () => {
        const snap = activeRuntime.snapshot();
        if (snap.surface !== 'authorized' || !snap.deployment) return undefined;
        const origin = snap.deployment.origin;
        const tenantId = snap.identity?.activeTenantId ?? '';
        const inbox = notificationInboxFor(activeRuntime, origin, tenantId);
        return () => inbox.page().then(() => undefined, () => undefined);
      },
    }).start();
    return stop;
  }, [activeRuntime]);
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test apps/mobile/src/app-id.test.ts apps/mobile/src/foreground-sync.test.ts apps/mobile/src/blind-push-integration-smoke.test.ts apps/mobile/src/device-inbox-integration-smoke.test.ts && pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: 全 pass（既有 149 pass/5 skip 基线不回归——新 smoke 用例无凭据时 skip）+ typecheck 通过。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/app-id.ts apps/mobile/src/app-id.test.ts apps/mobile/src/adapters/app-state.ts apps/mobile/src/foreground-sync.ts apps/mobile/src/foreground-sync.test.ts apps/mobile/src/blind-push-integration-smoke.ts apps/mobile/src/blind-push-integration-smoke.test.ts apps/mobile/src/composition.ts
git commit -m "feat(mobile-push): build-time app identity and foreground authoritative sync loop (T37 #67 task 7)"
```

---

## 计划级验证命令（worktree 根执行）

**执行环境锚定（本计划作者实查）**：本计划的全部锚定——HEAD `fb5f6653a`、分支 `codex/issue30-mobile-office`、前置 #41/#66 的已集成文件（`packages/mobile-core/src/device/device-registry.ts`、`packages/api-client/src/mobile/devices.ts`、`apps/mobile/src/composition.ts:215 registerActiveDeviceIfPossible`）——仅在 worktree `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep` 成立。主仓 main（`e625e4960`）**不含** `packages/mobile-core/src/device/device-registry.ts` 与 `packages/api-client/src/mobile/devices.ts`（实查 `ls` 均报 No such file）。执行前必须确认 `git rev-parse --show-toplevel` 指向该 worktree 且 `git log --oneline -1` 含已合并的前三批提交，否则下列 TS 命令全部失败。

```bash
go test ./internal/application/repository/ -run 'TestValidateMobileAppID|TestBindIsolates|TestTokenExclusivityIsPerApp|TestNotificationIntentFanOutPerApp|TestClaimJoinsAppID|TestMobilePushIsolation|TestMobilePushDisabled' -count=1 && go test ./internal/modules/workbench/notification/ -count=1 && go test ./internal/modules/workbench/service/workbench/ -run 'TestPushPayloadPolicyBlind|TestHTTPNotificationProviderBlind|TestAppRouting|TestDisabledNotificationProviderPauses' -count=1 && go test ./internal/handler/ -run 'TestMobileDevice|TestRegisterAppID|TestRegisterRejectsUndeclared|TestOfficialIntentCannotBind|TestPresenceScopedByApp' -count=1 && go build ./... && pnpm exec tsx --test packages/mobile-core/src/device/device-registry.test.ts packages/mobile-core/src/device/device-app-id.test.ts packages/api-client/src/mobile/devices.test.ts packages/api-client/src/mobile/devices-app-id.test.ts apps/mobile/src/app-id.test.ts apps/mobile/src/foreground-sync.test.ts apps/mobile/src/blind-push-integration-smoke.test.ts apps/mobile/src/device-inbox-integration-smoke.test.ts && pnpm --filter @weknora/mobile typecheck
```

opt-in 真实集成（AC3 客户端半边，需短期凭据，运行时如实记录、不伪造）：

```bash
WEKNORA_MOBILE_TEST_DEPLOYMENT_URL=https://<public-host> \
WEKNORA_MOBILE_TEST_EMAIL=<test-account> \
WEKNORA_MOBILE_TEST_PASSWORD=<short-lived-secret> \
[WEKNORA_MOBILE_TEST_ENTERPRISE_APP_ID=enterprise:<slug>] \
pnpm --filter @weknora/mobile test
```

## 本计划产出的关键接口摘要（Produces，供后续计划 Consumes）

- Go：`repository.{MobileAppIDOfficial, ValidateMobileAppID, NormalizeMobileAppID, DeviceRegistration.AppID, NotificationIntent.AppID, MobileDeviceStore.GetActiveForApp/CurrentScopeGenerationForApp/RevokeForApp/…ForApp}`；`pushnotification.{ApnsProvider, FcmProvider, ApnsTokenSource, FcmTokenSource, NewApnsP8TokenSource, NewFcmServiceAccountTokenSource}`；`workbenchservice.{PushPayloadPolicy, NewPushNotificationProviderWithOptions, NewHTTPNotificationProviderWithPolicy, DisabledNotificationProvider, NewAppRoutingNotificationProvider, NotificationDeviceRevoker.RevokeForApp}`；`handler.{MobileAppPolicy, MobileDeviceHandler.WithMobileAppPolicy}`（wire 字段 `app_id`）；env 键 `MOBILE_NOTIFICATION_PAYLOAD`、`MOBILE_NOTIFICATION_PROVIDER=disabled|none`、`MOBILE_ENTERPRISE_APP_ID`、`MOBILE_ENTERPRISE_PUSH_PROVIDER`、`MOBILE_APNS_*`、`MOBILE_FCM_*`；迁移 `000193_mobile_device_app`（versioned）/`000114_mobile_device_app`（sqlite）。
- TS：`DeviceRegistrationRecord.appId`、`DeviceRegistry.register({ appId? })`、`DeviceRemote.issueIntent(deviceId, appId?)`、`MobileDeviceRegistration.appId`；apps/mobile `resolveWeKnoraAppId`/`isValidWeKnoraAppId`/`createForegroundSyncLoop`/`createNativeAppStateLifecycle`、构建期 `EXPO_PUBLIC_WEKNORA_APP_ID`、`BlindPushIntegrationEvidence`。

## 自我审查记录（writing-plans 四项检查）

**独立计划审查修复记录（第二轮，逐项已修）**：
- 【阻塞】三处聚焦迁移 helper 的文件顺序错误（000114 被排在 000059/000060 之前，其 intents 重建段会 `no such table: mobile_notification_intents`）：Task 1 `openMobileAppDB`、Task 3 `openMobilePushPolicyDB`、Task 5 `newPushIsolationEnv` 已全部改为 000058 → 000059 → 000060 → 000114 并加顺序注释；复核中发现同类第四处——Task 1(i) 给 `openMobileHandlerDB` 单追加 000114 也会因该库无 intents 表失败，已改为按序补 000059/000060/000114 三文件。
- 【行号】`SendReceipt`（:111→:100）、`SendBatch`（:150→:114）、`send()`（:209→:202）、`TestNotificationProjectEventDerivesDurableIdentity`（:124→:115）已按 worktree 实读修正（函数锚定 + 字面量行注记）。
- 【表述】Task 1(j) 已如实注明：`TestNotificationProjectEventDerivesDurableIdentity` 自建表（:120）缺 `platform` 列而 ：122 INSERT 引用 platform，编排层修复同号迁移后该测试仍会因 `no such column: platform` 失败——先在问题、非本计划引入，且该测试不得被用作本计划回归证据。
- 【环境锚定】已核实并写入验证命令段：worktree `codex/issue30-mobile-office` @ `fb5f6653a`；主仓 main @ `e625e4960` 无 `packages/mobile-core/src/device/device-registry.ts` 与 `packages/api-client/src/mobile/devices.ts`（实查 `ls`）；执行必须在该 worktree。
- 【脚注】Task 6 的 tsconfig 声明改为弱证据如实表述（`apps/mobile/tsconfig.json` 实读无该 flag、两包无自有 tsconfig、`expo/tsconfig.base` 未读取核实），并指出测试的 `!` 断言使编译不依赖该旗标。

1. **Spec 覆盖**：AC1（注册/令牌不混用）→ Task 1（存储隔离）+ Task 3（路由与按 App 撤销）+ Task 4（允许清单与 intent 绑定）+ Task 5/7（两端证据）；AC2（关网关前台权威同步）→ Task 7（前台同步环，`disabled` 服务端语义由 Task 3/5 保证投递面关闭且 durable）；AC3（最高稳定 Interface）→ Task 5（真实迁移子集 + 真实 handler/projector/worker + httptest 真实 HTTP Provider 边界）+ Task 7 opt-in 冒烟（真实 transport/Runtime/服务端）；What-to-build 三段（盲推送/可禁用/企业独立标识+APNs/FCM）分别落 Task 3/Task 3/Task 2+3+4+6+7。无遗漏。审查中修正：Review Focus #1 原引用了不存在的测试名，改为实际存在的 `TestRegisterRejectsUndeclaredEnterpriseApp`（intent+register 双 400）。
2. **占位符扫描**：全文无 TBD/TODO/「稍后实现」/「类似 Task N」。审查中清理了两处草稿残迹：Task 4 Step 1 初稿含跨闭包技巧与坏断言，已整体重写为最终平铺形态；Task 6 Step 1 初稿含三段试探性代码，已重写为单一最终形态并把 `leased()` helper 逐字内联（源：`device-registry.test.ts:7-13` 实读）。Task 3 Step 3 保留一条执行期 grep 指令（核对 `RevokeForTenant` 测试替身）——这是核实步骤而非占位：本计划作者已 grep（结果为零），指令为执行期防线。
3. **类型/签名一致性**：`appId` 在 `DeviceRemote`（mobile-core）与 `MobileDeviceRemote`（api-client）同为可选参数；`DeviceRegistrationRecord.appId: string` 与 `MobileDeviceRegistration.appId: string` 逐字一致；Go 侧 `RevokeForApp(ctx, tenant, owner, device, appID, revision)` 在接口（Task 3）与实现（Task 1）参数序一致；`NewPushNotificationProviderWithOptions`/`NewHTTPNotificationProviderWithPolicy` 在 Task 3 定义、Task 5 消费签名一致；`AppRoutingNotificationProvider` 构造签名与 Task 3/Task 5 用法一致。既有构造器（`NewPushNotificationProvider`/`NewHTTPNotificationProvider`）保留委托，既有测试零破坏（`notification_delivery_test.go` 的既有用法 grep 已核对）。审查中修正三处测试逻辑错误：Task 1 幂等 ID 改为从 DB 读回（不再手拼 6 段字面量）；`Order("app_id")` 断言序修正（enterprise < official）；`TestClaimJoinsAppIDSoRevokedAppDoesNotResurrect` 改为官方/企业**同 device_id** 布局（否则缺 app 对齐也碰不到该断言）。审查中另修正容器 default 分支：未知 provider 模式必须保持既有 fail-closed（空 endpoint），不因配置了 URL 而误投。
4. **Review Focus 落实**：5 条各已挂到指定任务的指定测试（见各条目），无空挂。
