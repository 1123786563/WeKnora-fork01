# Happy 实时语音 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 保留 Happy 语音交互，通过产品授权的服务适配建立实时语音与可核对用量。

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

## 专项执行门槛与文件责任

本册的默认候选是复用 Happy 已有 ElevenLabs 原生 SDK 和交互，通过 Go 产品后端签发连接材料。实施 H21 前先完成该步骤的专项契约记录：实际供应商支持、会话 token 生命周期、工具回调、终止行为、用量核对。不能沿用上游 RevenueCat 项目、免费额度、硬编码 Agent ID。缺少运营配置只阻塞此分支，不阻塞核心聊天。

拟新增 POST `/api/v1/mobile/voice/sessions`（产品 session_id，agent_id，client_request_id）；DELETE `/api/v1/mobile/voice/sessions/:id`；GET `/api/v1/mobile/voice/sessions/:id`。服务端验证 session/Agent 所有权后选择配置中的语音 provider。移动端不提交供应商 API key，不自报可信计费用量。H21 拥有授权/连接，H22 拥有原生会话，H23 拥有终止/用量对账。

### H21：语音授权与 provider 适配

**依赖：** H12。**Files:** Create `docs/superpowers/specs/2026-09-10-happy-voice-service-design.md`、`internal/mobile/voice/service.go`、`service_test.go`、`provider.go`、`elevenlabs.go`、`elevenlabs_test.go`；Create `internal/handler/mobile_voice.go`、`mobile_voice_test.go`；Modify `internal/router/routes_mobile.go`；Create `packages/contracts/src/mobile/voice.ts`、`packages/api-client/src/mobile/voice.ts`。

**Interfaces:** Go `StartInput{UserID,SessionID,AgentID,RequestID string;TenantID uint64}`；`Authorizer.Check(ctx,StartInput) error`；`Provider.Start(ctx,StartInput)(Connection,error)`，Connection 仅含 `ID,Provider,Token string` 和 `ExpiresAt time.Time`；`Service.Start(ctx,input)` 先授权和幂等检查再调用 provider。client token 不进入一般响应日志。

- [ ] **Step 1：冻结专项契约并写越权不能触达 provider 的测试。** 阅读 Happy `apiVoice.ts`、`RealtimeVoiceSession.tsx`、server `voiceRoutes.ts` 与实施时供应商官方 SDK 文档；在新专项规格记录所有权、短期token、费用入口和数据出境范围。接口符合已确认信任边界后继续；若要求变更供应商或 E2EE 范围，先完成该分支设计决策。

```go
package voice
import("context";"errors";"testing")
func TestDeniedStartDoesNotContactProvider(t *testing.T){
 calls:=0
 s:=NewService(func(context.Context,StartInput)error{return errors.New("forbidden")},
  func(context.Context,StartInput)(Connection,error){calls++;return Connection{},nil})
 _,err:=s.Start(context.Background(),StartInput{UserID:"u",SessionID:"foreign"})
 if err==nil||calls!=0{t.Fatal("provider called without authorization")}
}
```

- [ ] **Step 2：RED。** `go test ./internal/mobile/voice -run TestDeniedStartDoesNotContactProvider -count=1`。
- [ ] **Step 3：实现授权优先的服务骨架和实际 provider。**

```go
package voice
import("context";"time")
type StartInput struct{UserID,SessionID,AgentID,RequestID string;TenantID uint64}
type Connection struct{ID,Provider,Token string;ExpiresAt time.Time}
type Service struct{check func(context.Context,StartInput)error;start func(context.Context,StartInput)(Connection,error)}
func NewService(c func(context.Context,StartInput)error,p func(context.Context,StartInput)(Connection,error))*Service{return &Service{check:c,start:p}}
func(s *Service)Start(ctx context.Context,i StartInput)(Connection,error){
 if err:=s.check(ctx,i);err!=nil{return Connection{},err};return s.start(ctx,i)
}
```

在真实接线时 `check` 调用现有 session/Agent授权与套餐能力入口，不让供应商 AgentID 替代产品 AgentID；provider 由服务端配置选择。requestID 以 user+tenant+session 作用域幂等，重复请求不会开第二条语音；持久关联与 H23 表一起交付再开放API。供应商超时返回明确未知/失败，不能返回 allowed=true。缓存token按实际有效期控制，不能伪造供应商没有提供的长有效期。

- [ ] **Step 4：GREEN。** 上述测试，加 httptest 假供应商403/429/500/超时/畸形token；handler401/跨空间拒绝、重复requestID同一连接。凭据未配置返回可展示的 unavailable，仍不能将语音行 accepted。
- [ ] **Step 5：提交。** stage 本任务文件与路由接线；`git commit -m "feat(mobile): authorize voice connections through product service"`。

### H22：原生语音状态与统一会话工具

**依赖：** H21/H10/H12。**Files:** Create `apps/mobile/sources/weknora/voice/controller.ts`、`controller.test.ts`、`VoiceProvider.tsx`；Modify `apps/mobile/sources/realtime/RealtimeSession.ts`、`RealtimeVoiceSession.tsx`、`sources/-session/SessionView.tsx` 的产品 wrapper。

**Interfaces:** `VoiceState='idle'|'connecting'|'active'|'stopping'|'failed'`；`VoicePort {start(sessionId:string):Promise<string>;stop():Promise<void>}`；`createVoiceController(port):{start(id):Promise<void>;stop():Promise<void>;state():VoiceState}`。底层 start 成功返回产品 voiceSessionId；Happy `startRealtimeSession(sessionId,initialContext?)` 原兼容调用只用于 remote wrapper。

- [ ] **Step 1：双击麦克风只有一个连接。**

```ts
import {expect,test} from 'vitest';
import {createVoiceController} from './controller';
test('double tap shares one voice connection',async()=>{
 let calls=0;const c=createVoiceController({start:async()=>{calls++;return 'v'},stop:async()=>{}});
 await Promise.all([c.start('s'),c.start('s')]);expect(calls).toBe(1);expect(c.state()).toBe('active');
});
```

- [ ] **Step 2：RED。** `pnpm --filter @weknora/mobile exec vitest run sources/weknora/voice/controller.test.ts`。
- [ ] **Step 3：实现单飞连接与状态机。**

```ts
export type VoiceState='idle'|'connecting'|'active'|'stopping'|'failed';
export interface VoicePort {start(id:string):Promise<string>;stop():Promise<void>}
export function createVoiceController(port:VoicePort){
 let state:VoiceState='idle',pending:Promise<void>|undefined,generation=0;
 return {state:()=>state,start(id:string){
  if(state==='stopping')return Promise.reject(new Error('VOICE_STOPPING'));
  if(pending)return pending;if(state==='active')return Promise.resolve();state='connecting';
  const started=generation;
  pending=port.start(id).then(()=>{if(started===generation)state='active'},e=>{if(started===generation)state='failed';throw e}).finally(()=>{pending=undefined});return pending;
 },async stop(){generation++;state='stopping';try{if(pending)await pending.catch(()=>{});await port.stop();}finally{state='idle';}}};
}
```

补“连接中stop/切空间”generation 测试，验证上面的取消门：start 完成不再把已取消状态改 active，停止流程关闭迟到连接。通过原有麦克风、语音状态条、静音/结束按钮接 controller；保留权限拒绝/音频焦点/耳机切换反馈。语音 client tools 只走 H10/H12 已授权命令；转写不能自己构造跨租户工具请求或第二套 Agent执行。录音权限在用户按麦克风时请求，退出必须释放原生音频资源。

- [ ] **Step 4：GREEN 与原生通话。** 测试connecting→取消、权限拒绝、连接失败、切空间、停止失败仍释放UI资源；iOS/Android 分别实测讲话、回复、静音、结束、来电打断/蓝牙切换。真实工具调用与文本会话显示同一产品会话归属。
- [ ] **Step 5：提交。** stage 本任务路径；`git commit -m "feat(mobile): retain Happy realtime voice interactions"`。

### H23：语音持久关联、结束与用量核对

**依赖：** H21/H22（上线授权端点前必须完成本任务持久化）。**Files:** Create `internal/mobile/voice/repository.go`、`usage.go`、`usage_test.go`、`reconcile.go`；Create `migrations/versioned/000095_mobile_voice_sessions.up.sql`、`.down.sql`；Create `apps/mobile/sources/weknora/voice/reconcile.ts`、`reconcile.test.ts`。

**Interfaces:** `Usage{ProviderSessionID string;DurationSeconds int64;Final bool}`；`UsageKey(provider,providerSessionID string) string`；repository 唯一键 `(user_id,tenant_id,session_id,client_request_id)` 和 `(provider,provider_session_id)`。原始用量通过当前后端计量入口记录，不在移动端计算最终扣款。

- [ ] **Step 1：重复供应商用量必须归并。**

```go
package voice
import "testing"
func TestUsageIdentity(t *testing.T){
 old:=Usage{ProviderSessionID:"v",DurationSeconds:10}
 final,err:=MergeUsage(old,Usage{ProviderSessionID:"v",DurationSeconds:12,Final:true})
 if err!=nil||final.DurationSeconds!=12||!final.Final{t.Fatal("cumulative usage was summed or lost")}
 replay,err:=MergeUsage(final,Usage{ProviderSessionID:"v",DurationSeconds:9})
 if err!=nil||replay!=final{t.Fatal("late event overwrote final usage")}
}
```

- [ ] **Step 2：RED。** `go test ./internal/mobile/voice -run TestUsageIdentity -count=1`；随后必须增加仓储级“同provider session最终用量重复写不产生第二条计量”的集成测试，不能仅依赖 key 单测。
- [ ] **Step 3：实现持久模型与 reconciler。**

```go
type Usage struct{ProviderSessionID string;DurationSeconds int64;Final bool}
func UsageKey(provider,id string)string{b,_:=json.Marshal([]string{provider,id});return string(b)}
func MergeUsage(old,next Usage)(Usage,error){
 if next.DurationSeconds<0 || (old.ProviderSessionID!=""&&old.ProviderSessionID!=next.ProviderSessionID){return old,errors.New("invalid usage")}
 if old.Final{return old,nil};if next.DurationSeconds<old.DurationSeconds{return old,errors.New("usage decreased")};return next,nil
}
```

usage.go 导入 encoding/json 和 errors。表保存产品/user/tenant/Agent关联、provider session、幂等requestID、连接状态、开始/结束时间、最后核对时间、原始用量/最终标记；不保存可复用供应商secret。DELETE验证归属，尽力终止provider并记录terminating；worker核对实际结束状态，重启后继续。用量接口失败保存unknown，不像上游示例回落0；供应商分页完整取证，最终记录幂等写入当前计量链路，不复制另一任务的计费实现。

- [ ] **Step 4：GREEN。** 隔离库验证重复start/重复用量、供应商500、客户端杀进程、终止超时、重启恢复；移动展示“用量待核对”与最终值，不假装免费。迁移up/down/up只在测试库运行。
- [ ] **Step 5：提交。** stage 本任务文件；`git commit -m "feat(mobile): reconcile voice lifecycle and authoritative usage"`。
