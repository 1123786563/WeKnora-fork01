# 移动工作台 03：设备、可靠通知与深链 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 任务后台继续执行时，把待审批、完成、失败和预算不足通知可靠送达正确用户。

**Architecture:** 业务事件通过事务Outbox驱动通知投递；推送是可丢失、可重复的提示。所有深链恢复产品认证并重新检查资源权限，不能直接执行决定。

**Tech Stack:** Go/GORM、现有数据库、Expo Notifications/APNs/FCM端口、React Native。

**Spec:** [技术架构](../specs/2026-09-12-mobile-ai-saas-workbench-architecture.md) §9.2、§10、§13；[总计划](2026-09-12-mobile-ai-saas-workbench.md)。执行者同时读取本册与规格。

## Global Constraints

- “采用 **Happy 移动底座 + WeKnora 产品与执行控制 + Paseo 远程编码执行接入 + AWS 示例多模态交互复用**。”
- “分批交付不取消原 Happy 交互保留要求，未交付能力继续保留在清单中。”
- “任何 Run、事件、附件、审批都通过服务端认证上下文和持久所有权解析空间。”
- “同一请求 ID 与相同参数重放返回原 Run；同 ID 不同参数返回冲突。”
- “取消、失败和超时不抹掉实际用量，也不意味着外部副作用已回滚。”
- “预算授权、工具操作批准和连接授权是三件独立的事。”
- “子 Run 共享父任务预算树，不能复制一份可消费余额；新增子执行仍需授权和准入。”
- 保持 Expo 55 / React Native 0.83.1 / React 19.2.0 与当前 pnpm 10.28.2 工作区；执行前验证锁文件。Go 1.26.0；SQLite 与 PostgreSQL 分别验收。
- 本册所有新增接口和文件是实施目标；缺少环境记 `blocked-env`，不能把静态代码、fixture 或测试跳过写为真实运行验收。
- 实施前创建隔离工作区并带入未提交规格；保留并行任务修改。只提交本任务文件；不得 `git add .`、擅自发布或改写旧台账通过状态。

---

## 文件与责任边界

设备注册、通知记录和供应商投递分别建小模块。复用Run事件来源，不为通知创建新任务状态机；推送令牌只存后端加密字段。

### W13：设备注册与账号切换撤销

**依赖：** W07。

**Files：**

- Create: `internal/application/repository/mobile_device.go`、`internal/application/repository/mobile_device_test.go`
- Create: `internal/handler/mobile_device.go`、`internal/handler/mobile_device_test.go`
- Create: `apps/mobile/sources/weknora/notifications/registration.ts`
- Modify: `internal/router/routes_workbench.go`
- Create: `migrations/versioned/000124_mobile_devices.{up,down}.sql`、`migrations/sqlite/000044_mobile_devices.{up,down}.sql`

**Interfaces：**

Produces `DeviceRegistration{DeviceID,OwnerID,Environment,TokenCiphertext string; Revision int64; RevokedAt *time.Time}`，`NewMobileDeviceStore(db *gorm.DB,environment string)`；`Bind(ctx context.Context,row DeviceRegistration) error`、`Revoke(ctx,owner,device string,revision int64) error`、`ListActive(ctx,owner,environment string)([]DeviceRegistration,error)`。PUT/DELETE `/mobile/devices/:id`。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```go
package repository
import (
 "context"
 "testing"
 "github.com/stretchr/testify/require"
)
func TestMobileDeviceRevocationIsOwnerScoped(t *testing.T) {
 s:=NewMobileDeviceStore(openRunTestDB(t),"dev"); ctx:=context.Background()
 require.NoError(t,s.Bind(ctx,DeviceRegistration{DeviceID:"d",OwnerID:"u1",Environment:"dev",TokenCiphertext:"encrypted",Revision:1}))
 require.Error(t,s.Revoke(ctx,"other","d",1))
 require.NoError(t,s.Revoke(ctx,"u1","d",1))
 rows,err:=s.ListActive(ctx,"u1","dev");require.NoError(t,err);require.Empty(t,rows)
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/application/repository -run TestMobileDevice -count=1
```

预期：MobileDeviceStore/DeviceRegistration 未定义。

- [ ] **Step 3：实现最小行为。**

实现CAS更新，设备环境参与唯一键；原token加密更新与owner切换同事务。客户端不能在body指定有效owner。
```sql
UPDATE mobile_devices SET revoked_at = CURRENT_TIMESTAMP, revision = revision + 1
WHERE device_id = ? AND owner_id = ? AND environment = ? AND revision = ? AND revoked_at IS NULL;
```
NewMobileDeviceStore绑定environment，所有SQL自动带它；Bind拒绝row.Environment不匹配，ListActive拒绝调用参数environment不匹配。handler不接受客户端改写服务器环境。

- [ ] **Step 4：接通实际入口。**

registration.ts在系统通知权限已允许后取得token，登录成功绑定；token刷新幂等更新；logout先尝试后端撤销，再本地清理；离线退出保留最小待撤销标识，服务端短期活跃绑定/会话撤销联动。不会因为用户拒绝推送而阻止聊天。生产环境不接受development token。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 账号A→B后旧事件不能发给B；注销迟到更新不能复活绑定。
- 注册未认证401、其他owner删除404、过期revision409；真实正例token刷新。
- 原生通知拒绝、授权变化、卸载token失效；日志检索不含明文token。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add 'internal/application/repository/mobile_device.go' 'internal/application/repository/mobile_device_test.go' 'internal/handler/mobile_device.go' 'internal/handler/mobile_device_test.go' 'apps/mobile/sources/weknora/notifications/registration.ts' 'internal/router/routes_workbench.go' 'migrations/versioned/000124_mobile_devices.up.sql' 'migrations/versioned/000124_mobile_devices.down.sql' 'migrations/sqlite/000044_mobile_devices.up.sql' 'migrations/sqlite/000044_mobile_devices.down.sql'
git diff --cached --check
git diff --cached --stat
git commit -m "feat(notifications): register and revoke scoped devices"
```

### W14：事务事件到通知 Outbox

**依赖：** W03、W05、W13。

**Files：**

- Create: `internal/application/repository/mobile_notification.go`、`internal/application/repository/mobile_notification_test.go`
- Create: `internal/application/service/workbench/notification.go`
- Modify: `internal/application/repository/agent_run_events.go`
- Create: `migrations/versioned/000125_mobile_notifications.{up,down}.sql`、`migrations/sqlite/000045_mobile_notifications.{up,down}.sql`

**Interfaces：**

Produces `NotificationIntent{EventID,OwnerID,DeviceID,Environment,Kind,RunID string; ExpiresAt time.Time}`；`NewNotificationStore(db *gorm.DB)`；`Enqueue(ctx,intent) error`、`Claim(ctx,worker string,limit int,lease time.Duration)([]NotificationDelivery,error)`；Delivery包含ID/Intent/Attempt/Fence。唯一event/owner/device/environment。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```go
package repository
import (
 "context"
 "testing"
 "time"
 "github.com/stretchr/testify/require"
)
func TestNotificationOutboxDeduplicates(t *testing.T) {
 db:=openRunTestDB(t); ctx:=context.Background()
 _,err:=NewAgentRunStore(db).Admit(ctx,testAdmission());require.NoError(t,err)
 require.NoError(t,NewMobileDeviceStore(db,"dev").Bind(ctx,DeviceRegistration{DeviceID:"d",OwnerID:"u1",Environment:"dev",TokenCiphertext:"encrypted",Revision:1}))
 s:=NewNotificationStore(db)
 in:=NotificationIntent{EventID:"e",OwnerID:"u1",DeviceID:"d",Environment:"dev",Kind:"completed",RunID:"r1",ExpiresAt:time.Now().Add(time.Hour)}
 require.NoError(t,s.Enqueue(ctx,in));require.NoError(t,s.Enqueue(ctx,in))
 rows,err:=s.Claim(ctx,"worker",10,time.Minute);require.NoError(t,err);require.Len(t,rows,1)
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/application/repository -run TestNotificationOutbox -count=1
```

预期：NewNotificationStore 未定义。

- [ ] **Step 3：实现最小行为。**

消息状态与通知意图必须在同事务，或从持久事件按原子checkpoint消费；本计划选后者减少侵入。消费者在事务里读取seq后事件、插入唯一意图、推进消费cursor；重启后重复读由唯一键吸收。
```sql
INSERT INTO mobile_notification_intents(event_id, owner_id, device_id, environment, kind, run_id, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(event_id, owner_id, device_id, environment) DO NOTHING;
```
environment为设备所属应用环境；过期、撤销设备不claim，已在flight投递前再查。

- [ ] **Step 4：接通实际入口。**

仅terminal、interaction_requested、budget_exhausted触发；token增量不推送。投递前检查当前owner有权访问run；同一待处理卡被解决时原通知撤销/过期。新增worker使用现有容器生命周期，不在HTTP请求中等待供应商。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 事件落库后进程崩溃再消费不丢通知；通知插入后cursor未推进不重复。
- 两worker竞争只一个有效Fence；过期Fence不能ack。
- 审批已解决/成员已移除/设备撤销，实际Send调用次数为0；同一个测试包含有效通知发送成功组。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add 'internal/application/repository/mobile_notification.go' 'internal/application/repository/mobile_notification_test.go' 'internal/application/service/workbench/notification.go' 'internal/application/repository/agent_run_events.go' 'migrations/versioned/000125_mobile_notifications.up.sql' 'migrations/versioned/000125_mobile_notifications.down.sql' 'migrations/sqlite/000045_mobile_notifications.up.sql' 'migrations/sqlite/000045_mobile_notifications.down.sql'
git diff --cached --check
git diff --cached --stat
git commit -m "feat(notifications): derive durable intents from run events"
```

### W15：供应商投递、回执与重试

**依赖：** W14。

**Files：**

- Create: `internal/notification/provider.go`、`internal/notification/provider_test.go`、`internal/notification/expo.go`
- Create: `internal/application/service/workbench/notification_worker.go`
- Modify: `internal/container/container.go`、`internal/config/config.go`

**Interfaces：**

Produces `PushProvider.Send(ctx context.Context, token string,payload PushPayload)(PushReceipt,error)`；`PushPayload{Title,Body,RunID,EventID string}`；Receipt{ID,Status string}。错误分类永久失效、限流、可重试、未知。SDK/HTTP供应商选择用配置，生产token不传到mock provider。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```go
package notification
import "testing"
func TestPushClassification(t *testing.T) {
 cases:=[]struct{code string; revoke,retry bool}{{"DeviceNotRegistered",true,false},{"MessageRateExceeded",false,true},{"UnknownTransport",false,true}}
 for _,c:=range cases {r,b:=ClassifyPushFailure(c.code);if r!=c.revoke||b!=c.retry{t.Fatalf("%s",c.code)}}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/notification -run TestPush -count=1
```

预期：ClassifyPushFailure 未定义。

- [ ] **Step 3：实现最小行为。**

```go
func ClassifyPushFailure(code string)(revoke,retry bool) {
 switch code {
 case "DeviceNotRegistered": return true,false
 case "MessageRateExceeded","UnknownTransport": return false,true
 default: return false,false
 }
}
```
实际HTTP客户端验证响应、receipt ID和每条结果；200不代表最终送达。worker指数退避带抖动、上限和有效期，永久错误撤销设备，配置错误告警并暂停该provider。

- [ ] **Step 4：接通实际入口。**

容器注入有凭据的provider，缺配置只显示通知不可用；不阻塞原Run。APNs/FCM直连可换端口，首批Expo投递需固定运行环境与真实推送凭据。敏感参数不在锁屏payload；event_id供客户端幂等提示，无法保证供应商exactly-once时允许重复但不能重复业务操作。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- mock HTTP：429尊重retry-after，超时进入unknown/retry，invalid receipt不ack，批量部分失败只重试失败条。
- 一条真实iOS、一条真实Android推送和已撤销token负例；工具权限不允许发送时记blocked-env，不伪造送达。
- `go test ./internal/notification ./internal/application/service/workbench -run 'TestPush|TestNotificationWorker' -count=1`。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add 'internal/notification/provider.go' 'internal/notification/provider_test.go' 'internal/notification/expo.go' 'internal/application/service/workbench/notification_worker.go' 'internal/container/container.go' 'internal/config/config.go'
git diff --cached --check
git diff --cached --stat
git commit -m "feat(notifications): deliver pushes with receipts and bounded retries"
```

### W16：安全深链与待处理卡

**依赖：** W10、W11、W13–W15。

**Files：**

- Create: `apps/mobile/sources/weknora/notifications/deep-link.ts`、`apps/mobile/sources/weknora/notifications/deep-link.test.ts`
- Create: `apps/mobile/sources/weknora/notifications/NotificationRouter.tsx`
- Modify: `apps/mobile/sources/app/_layout.tsx`、`apps/mobile/sources/weknora/workbench/WorkbenchScreen.tsx`

**Interfaces：**

Produces `parseNotificationLink(raw:string):{tenantID:string;runID:string}`；只接受weknora://execution?tenant=...&run=...，不接受token/action/redirect/server。NotificationRouter保存导航意图后登录，从可信成员列表选择空间，查询W03所有权通过才进入。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { parseNotificationLink } from './deep-link.ts';
test('deep links cannot authorize commands or inject tokens',()=>{
 assert.deepEqual(parseNotificationLink('weknora://execution?tenant=t&run=r'),{tenantID:'t',runID:'r'});
 for(const suffix of ['&action=approve','&token=x','&server=https://evil.test']) {
  assert.throws(()=>parseNotificationLink('weknora://execution?tenant=t&run=r'+suffix),/INVALID_LINK/);
 }
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test apps/mobile/sources/weknora/notifications/deep-link.test.ts
```

预期：parseNotificationLink 未定义。

- [ ] **Step 3：实现最小行为。**

```ts
export function parseNotificationLink(raw:string) {
 const u=new URL(raw);
 if(u.protocol!=='weknora:'||u.hostname!=='execution'||u.pathname||u.hash||u.username||u.password)throw new Error('INVALID_LINK');
 const keys=[...u.searchParams.keys()];
 if(keys.length!==2||!keys.includes('tenant')||!keys.includes('run'))throw new Error('INVALID_LINK');
 const tenantID=u.searchParams.get('tenant')!,runID=u.searchParams.get('run')!;
 if(!tenantID||!runID)throw new Error('INVALID_LINK');
 return {tenantID,runID};
}
```
tenant仅提示导航，API身份由W07当前作用域，无法访问时清理意图并显示不可访问。

- [ ] **Step 4：接通实际入口。**

原生冷启动和通知点击事件进入同一去重router；未登录只保存内存/最小安全导航引用。读取最新PendingInteraction而非使用推送中的审批参数；点击批准再次走W05决策。iOS Live Activity/Android进度入口只调用同一路由。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 杀App点击、已登录不同空间、成员撤销、重复通知、同空间其他owner、过期审批。
- URI重复参数、大小写绕过、编码换行、外部scheme均拒绝。
- 双平台打开同一事件无自动approve/cancel网络请求；真实待审批卡能成功处理一次。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add 'apps/mobile/sources/weknora/notifications/deep-link.ts' 'apps/mobile/sources/weknora/notifications/deep-link.test.ts' 'apps/mobile/sources/weknora/notifications/NotificationRouter.tsx' 'apps/mobile/sources/app/_layout.tsx' 'apps/mobile/sources/weknora/workbench/WorkbenchScreen.tsx'
git diff --cached --check
git diff --cached --stat
git commit -m "feat(mobile): route notification links through product authorization"
```
