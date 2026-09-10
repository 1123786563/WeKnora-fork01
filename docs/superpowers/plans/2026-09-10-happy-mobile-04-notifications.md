# Happy 移动通知 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付产品事件到原生通知的可撤销、可重试链路与授权深链。

**Architecture:** Happy 原生交互通过移动适配层消费共享 contracts/api-client/domain；产品身份、会话与权限仍由 WeKnora 后端管理。本册只实现下列任务的责任范围，沿用总计划的文件归属和依赖。

**Tech Stack:** Expo 55、React Native 0.83.1、React 19.2.0、TypeScript、pnpm、Vitest/Node test，服务增量沿用 Go/Gin 与现有数据库。

**Spec:** [已确认方向与细化规格](../specs/2026-09-10-happy-agent-mobile-design.md)；[总计划、接口归属与执行规则](2026-09-10-happy-agent-mobile.md)。所有新增路径、类型和端点均是实施目标，不代表当前已经存在。

## Global Constraints

- 移动不导入 DOM `ui/views` 或 Web `core`。
- REST/SSE/文件端点只在共享 SDK 定义；原生 transport 承担 POST、header、增量读取和取消。
- 会话可跨轮切换 Agent。发送时记录实际 Agent；历史按消息的 Agent 标识展示，不能随当前选择回写旧消息。
- 前端能力声明用于展示，不授予权限。
- 未知状态先对账，禁止自动重复付费执行。
- 只有矩阵全部验收或用户明确变更范围，才能声称完整保留 Happy 移动能力。
- Happy 基线 `ac64b9b4677870f7b7a9eacfd0780959229717f1`，源文件从仓库归档引入；保留许可记录，不引用相邻工作目录。
- 全部全局约束还包括总计划 Global Constraints；执行者必须同时读取。每项运行证据区分 fixture、真实后端、原生运行。

---

## 新服务接口与落点

以下是拟新增产品 API，不是 Happy `/v1/push-tokens` 的直接代理：PUT `/api/v1/mobile/devices/:device_id` 登记当前用户设备；DELETE 同路径撤销；POST `/api/v1/mobile/devices/:device_id/presence` 上报前台会话。用户/租户均来自认证上下文，body 不能决定归属。默认使用 Expo push gateway（与 Happy 客户端技术栈一致），供应商密钥只放服务端配置。

H18 拥有设备持久化/路由，H19 拥有 durable outbox/发送 worker，H20 拥有原生登记与深链。推送 payload 不含聊天正文、凭证或可自由跳转的 URL；用户看到通用“任务完成/需要操作”，打开后重新授权读取内容。

### H18：设备登记、轮换和撤销

**依赖：** H08。**Files:** Create `internal/mobile/push/device.go`、`device_test.go`、`device_repository.go`；Create `internal/handler/mobile_devices.go`、`mobile_devices_test.go`、`internal/router/routes_mobile.go`；Modify `internal/router/router.go`；Create `migrations/versioned/000093_mobile_devices.up.sql`、`.down.sql`；Create `packages/api-client/src/mobile/devices.ts`。

**Interfaces:** Go `Device{ID,UserID,Token,Platform string; TenantID uint64}`；`AuthorizeDevice(actor string,d Device) error`；`DeviceRepository.Upsert(ctx,Device) error`、`Delete(ctx,userID,deviceID string) error`。路由注册通过现有 DI/RouterParams 注入，不能在 handler 创建全局 DB。

- [ ] **Step 1：跨用户设备撤销必须拒绝。**

```go
package push
import "testing"
func TestDeviceOwner(t *testing.T) {
 if err := AuthorizeDevice("other", Device{ID:"d",UserID:"owner"}); err == nil {t.Fatal("foreign device accepted")}
}
```

- [ ] **Step 2：RED。** `go test ./internal/mobile/push -run TestDeviceOwner -count=1`。
- [ ] **Step 3：实现归属检查、表和幂等 upsert。**

```go
package push
import "errors"
type Device struct { ID,UserID,Token,Platform string; TenantID uint64 }
func AuthorizeDevice(actor string,d Device) error {
 if actor=="" || actor!=d.UserID {return errors.New("device forbidden")}; return nil
}
```

```sql
CREATE TABLE mobile_devices (
 id VARCHAR(64) NOT NULL, user_id VARCHAR(512) NOT NULL,
 tenant_id BIGINT NOT NULL, token TEXT NOT NULL, platform VARCHAR(16) NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), revoked_at TIMESTAMPTZ,
 PRIMARY KEY (user_id,id)
);
CREATE UNIQUE INDEX mobile_devices_active_token ON mobile_devices(token) WHERE revoked_at IS NULL;
```

PUT 校验 iOS/Android、token 非空/长度上限、当前成员身份。token 轮换在事务中撤销旧绑定并更新；同 token 登录另一用户须通过该用户主动登记使旧用户绑定失效，不能查询泄露 token 所有人。DELETE 仅匹配当前 user+device，幂等204。日志不输出 token。down SQL `DROP TABLE mobile_devices;` 仅供隔离测试/明确回退使用，部署回退默认保留表。

- [ ] **Step 4：GREEN 与持久集成。** 同一命令；httptest 验证未认证401、跨用户404/403、有效重复PUT、轮换后旧token不可达、撤销后无推送。隔离库跑 migration up/down/up，重启服务设备仍存在；发现093编号冲突则调整整个文件名和引用，禁止覆盖。
- [ ] **Step 5：提交。** stage 本任务文件/DI接线/SDK；`git commit -m "feat(mobile): register revocable push devices"`。

### H19：产品事件 outbox、发送和回执

**依赖：** H18。**Files:** Create `internal/mobile/push/outbox.go`、`outbox_test.go`、`worker.go`、`expo_sender.go`、`expo_sender_test.go`；Create `migrations/versioned/000094_mobile_push_outbox.up.sql`、`.down.sql`；Modify 实际产品消息完成/审批持久化出口（以 `internal/application/service/message.go` 与审批服务调用链定位）；Create `docs/migrations/happy/push-event-sources.md`。

**Interfaces:** `Event{ID,UserID,SessionID,MessageID,Kind string;TenantID uint64}`；`DeliveryKey(eventID,deviceID string) string`；`Sender.Send(ctx,token string,event Event)(ticketID string,err error)`；`Outbox.Enqueue(ctx,event Event) error` 和 worker claim/ack/retry 在数据库实现。事件种类 `completed|approval|question|failed`，只从可信服务事件构造。

- [ ] **Step 1：重放事件不能生成第二个设备投递键。**

```go
package push
import "testing"
func TestDeliveryIdentity(t *testing.T) {
 if DeliveryKey("e","d")==DeliveryKey("e","d2") {t.Fatal("devices collapsed")}
 if DeliveryKey("a:b","c")==DeliveryKey("a","b:c") {t.Fatal("ambiguous key")}
}
```

- [ ] **Step 2：RED。** `go test ./internal/mobile/push -run TestDeliveryIdentity -count=1`。
- [ ] **Step 3：实现稳定键、持久重试与网关。**

```go
func DeliveryKey(eventID,deviceID string) string {
 b,_:=json.Marshal([]string{eventID,deviceID}); return string(b)
}
```

在 `outbox.go` 导入 encoding/json。表以 `(event_id,device_id)` 为唯一约束，保存 payload、attempts、next_attempt_at、lease_until、ticket_id、receipt 状态。worker 事务 `FOR UPDATE SKIP LOCKED` 领取，5xx/429指数退避且有上限；进程崩溃租约过期重试。投递前再检查当前成员身份和设备撤销；有效前台同会话 presence 可抑制弹出，不能丢失产品待办。回执 DeviceNotRegistered 撤销 token，瞬时错误保留重试。

关键事件与 outbox 同事务写入；无法同事务的已有事件源必须用持久事件ID/水位补扫，不能只监听内存 event bus。明确交付语义为至少一次，网关成功后数据库 ack 丢失可能重复，H20 以 eventID 去重，禁止宣称端到端 exactly-once。Expo 请求使用配置 URL/超时/服务端 access token；凭证不写默认生产值。

- [ ] **Step 4：GREEN 与故障注入。** 同一包测试，httptest server 返回429→200、200后数据库失败、receipt无效token；重启worker后未投递事件补发。验证完成事件/审批事件各从真实产品路径进入 outbox，不仅直接调用 Enqueue。
- [ ] **Step 5：提交。** stage 明确的 source hook、outbox/provider/migration；`git commit -m "feat(mobile): deliver durable product push events"`。

### H20：原生通知授权与受控深链

**依赖：** H19/H05。**Files:** Create `packages/domain/src/mobile/notification-target.ts`、`notification-target.test.ts`；Create `apps/mobile/sources/weknora/notifications/register.ts`、`open.ts`、`open.test.ts`；Modify 原生 `_layout.tsx` 和通知设置页。

**Interfaces:** `NotificationTarget{tenantId:string;sessionId:string;eventId:string}`；`parseNotificationTarget(value:unknown):NotificationTarget`；`openNotification(target,loadSession:(id:string)=>Promise<void>):Promise<void>` 要在 scope 切换授权后执行。deviceId 是本安装稳定随机值，不是硬件追踪标识。

- [ ] **Step 1：拒绝任意 URL 与缺失身份。**

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import {parseNotificationTarget} from './notification-target.ts';
test('notification cannot navigate to untrusted URLs',()=>{
 assert.throws(()=>parseNotificationTarget({url:'https://evil.test'}),/NOTIFICATION/);
 assert.deepEqual(parseNotificationTarget({tenantId:'t',sessionId:'s',eventId:'e'}),{tenantId:'t',sessionId:'s',eventId:'e'});
});
```

- [ ] **Step 2：RED。** `pnpm exec tsx --test packages/domain/src/mobile/notification-target.test.ts`。
- [ ] **Step 3：实现解析和授权导航。**

```ts
export function parseNotificationTarget(v:unknown):NotificationTarget {
 if(!v||typeof v!=='object')throw new Error('NOTIFICATION');
 const x=v as Record<string,unknown>;
 if(['tenantId','sessionId','eventId'].some(k=>typeof x[k]!=='string'||!x[k]))throw new Error('NOTIFICATION');
 return {tenantId:x.tenantId as string,sessionId:x.sessionId as string,eventId:x.eventId as string};
}
```

用户主动开启通知时才请求系统权限并登记 token；拒绝不循环弹窗。冷启动先等待登录，验证 membership 后切空间，再 getSession 授权，最后进入会话；失败留在当前页解释。eventId 去重覆盖冷启动初始 notification 与 listener 双到；退出先调用撤销（离线记录撤销重试且本地忽略该身份通知），不能假设清本地token已完成服务器撤销。

- [ ] **Step 4：GREEN 与真实设备推送。** 同一测试；原生前台/后台/杀进程、两账号切换、权限撤销、旧会话删除、重复event各跑一次。使用项目 development build 和真实推送凭证；Expo Go/模拟器不作为完整推送验收。
- [ ] **Step 5：提交。** stage 本任务文件；`git commit -m "feat(mobile): open push events through authorized navigation"`。
