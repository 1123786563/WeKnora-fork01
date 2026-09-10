# Happy 远程与高级交互 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 保留 Happy 原有远程机器、设备切换、目标和旁支会话能力，并隔离产品身份与远程协议。

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

## 专项方案与执行条件

Happy 的 machine/session RPC 是实际远程协议，不是 WeKnora 普通工具调用。本册默认候选保留独立 remote provider 与受信任的自托管 Happy 服务；产品保存可撤销的外部连接引用，移动端持有独立加密材料。不会把 WeKnora token 转发给 Happy，不以 CLI 机器路径取代产品资源ID。

H24 先冻结远程绑定证明/部署/加密边界专项规格，再开启协议接线。若无法验证远端归属或用户要求不同部署形态，本册保持 blocked 并给出具体决策，H01–H23 可继续。不得仅因能连到公共 Happy 地址就上传产品身份或连接私有机器。

### H24：可信远程绑定与凭证隔离

**依赖：** H08/H05。**Files:** Create `docs/superpowers/specs/2026-09-10-happy-remote-bridge-design.md`；Create `internal/mobile/remote/binding.go`、`binding_test.go`、`repository.go`；Create `internal/handler/mobile_remote.go`、Modify `routes_mobile.go`；Create `migrations/versioned/000096_mobile_remote_bindings.up.sql`、`.down.sql`；Create `apps/mobile/sources/weknora/remote/credentials.ts`、`binding.ts`、`binding.test.ts`；Create `integrations/happy-server/weknora-proof-route.ts`（按固定 Happy Server 的 Fastify 类型实现，部署到受控远端的最小扩展）。

**Interfaces:** `RemoteBinding{id:string;origin:string;remoteAccountId:string;status:'active'|'revoked'}`；Go `Proof{Audience,Nonce,RemoteAccountID string;ExpiresAt time.Time}`；`ValidateProof(p Proof,nonce string,now time.Time)error`。产品 POST `/api/v1/mobile/remote-bindings/challenge` 生成一次nonce；POST `/api/v1/mobile/remote-bindings` 接受签名proof；DELETE `/:id` 撤销。

- [ ] **Step 1：专项契约记录与失效证明测试。** 专项规格明确已配置 remote origin 白名单、签名公钥轮换、nonce一次性消费、过期、产品user/tenant绑定、远程密钥仅设备保存。远端最小扩展 POST `/v1/integrations/weknora/proofs` 只对自身已认证用户签发受众/nonce/account/expiry，不能接受body自报account。该新增部署方案完成评审后才执行接线。

```go
package remote
import("testing";"time")
func TestExpiredOrWrongProof(t *testing.T){
 now:=time.Unix(100,0)
 if ValidateProof(Proof{Audience:"weknora-mobile",Nonce:"n",RemoteAccountID:"u",ExpiresAt:now.Add(-time.Second)},"n",now)==nil{t.Fatal("expired proof")}
 if ValidateProof(Proof{Audience:"other",Nonce:"n",RemoteAccountID:"u",ExpiresAt:now.Add(time.Minute)},"n",now)==nil{t.Fatal("wrong audience")}
}
```

- [ ] **Step 2：RED。** `go test ./internal/mobile/remote -run TestExpiredOrWrongProof -count=1`。
- [ ] **Step 3：实现 proof claims 与签名验证接线。**

```go
package remote
import("errors";"time")
type Proof struct{Audience,Nonce,RemoteAccountID string;ExpiresAt time.Time}
func ValidateProof(p Proof,n string,now time.Time)error{
 if p.Audience!="weknora-mobile"||p.Nonce!=n||p.RemoteAccountID==""||!p.ExpiresAt.After(now){return errors.New("invalid proof")};return nil
}
```

claims校验之前必须用配置的远端公钥和固定算法验签，不能接受 `alg:none` 或由token控制jwks URL。nonce在产品DB事务中单次消费并写binding，带入当前认证user/tenant；同proof再次提交拒绝。移动SecureStore用独立remote命名空间保存原Happy secret/token，原加密实现保留。产品DB仅保留归属和外部引用，不保存Happy私钥；撤销关闭所有对应订阅并禁止新RPC。

- [ ] **Step 4：GREEN 与真实双端绑定。** 测试错误签名、过期/重放nonce、不同产品用户消费、远端account伪造、origin替换、轮换公钥和logout；使用受控Happy测试服务验证扫码/确认绑定→产品重启恢复引用→撤销失效。只验证claims函数不足以验收。
- [ ] **Step 5：提交。** stage 本任务文件；`git commit -m "feat(mobile): bind remote accounts with verified isolated credentials"`。远端扩展发布单独走其部署审批，不在客户端构建时自动发布。

### H25：远程机器与会话 RPC 接入

**依赖：** H24。**Files:** Create `apps/mobile/sources/weknora/remote/provider.ts`、`provider.test.ts`、`RemoteConnectionScreen.tsx`；Modify 引入的 `sources/sync/apiSocket.ts` 与 `ops.ts` 的实例化边界；Create `packages/contracts/src/mobile/remote.ts`。

**Interfaces:** `RemoteTarget{bindingId:string;machineId:string;sessionId?:string}`；`RemoteProvider {invoke(target:RemoteTarget,method:string,args:unknown):Promise<unknown>;close():void}`；RPC method只接受明确的operation映射，不允许页面直接传任意method。

- [ ] **Step 1：撤销绑定后的机器命令不能发送。**

```ts
import {expect,test} from 'vitest';
import {createRemoteProvider} from './provider';
test('revoked connection never contacts a machine',async()=>{
 let calls=0;
 const p=createRemoteProvider(()=>false,async()=>{calls++;return {}});
 await expect(p.invoke({bindingId:'b',machineId:'m'},'spawn',{})).rejects.toThrow('REMOTE_REVOKED');
 expect(calls).toBe(0);
});
```

- [ ] **Step 2：RED。** `pnpm --filter @weknora/mobile exec vitest run sources/weknora/remote/provider.test.ts`。
- [ ] **Step 3：实现provider门和原协议绑定。**

```ts
export interface RemoteTarget{bindingId:string;machineId:string;sessionId?:string}
export function createRemoteProvider(active:(id:string)=>boolean,rpc:(t:RemoteTarget,m:string,a:unknown)=>Promise<unknown>){
 let closed=false;
 const methods=new Set(['spawn','resume','stop','switch','readFile','writeFile','listDirectory','bash','goal','fork','sideChat','archive','rewind','duplicate']);
 return {async invoke(t:RemoteTarget,m:string,a:unknown){
  if(closed||!active(t.bindingId))throw new Error('REMOTE_REVOKED');
  if(!methods.has(m))throw new Error('REMOTE_METHOD');
  return rpc(t,m,a);
 },close(){closed=true;}};
}
```

rpc 实际按固定 `ops.ts` 的 machineSpawnNewSession/machineResumeSession/sessionSwitch/sessionReadFile/sessionWriteFile 等签名映射；保留 Happy encryption 与远端能力协商，不重新实现密码学。将全局 singleton 变为每binding连接对象，closed时真正disconnect。旧版本CLI不支持某命令时明确提示升级/不支持并保留未验收记录。bash/写文件沿Happy现有显式用户操作与权限提示，产品能力开关不能绕过远程工具审批。

- [ ] **Step 4：GREEN 与受控机器验证。** RPC测试覆盖超时不重发命令、断网重连、会话/机器不匹配、切binding迟到结果；真实测试目录完成spawn→观察→停止→恢复→local/remote切换、读文件、写测试文件、列目录。禁止对用户生产目录运行演示命令。
- [ ] **Step 5：提交。** stage本任务路径；`git commit -m "feat(mobile): preserve remote machine interactions behind provider boundary"`。

### H26：目标、旁支、fork/archive 与能力路由

**依赖：** H25/H09。**Files:** Create `packages/domain/src/mobile/action-route.ts`、`action-route.test.ts`；Create `apps/mobile/sources/weknora/remote/advanced-actions.ts`、`advanced-actions.test.ts`；Modify `AgentGoalBar`、`AgentQuestionBanner`、SessionView/会话菜单的适配接线；Create `docs/migrations/happy/advanced-command-matrix.json`。

**Interfaces:** `BackendKind='product'|'remote'`；`AdvancedAction='goal'|'fork'|'sideChat'|'archive'|'rewind'|'duplicate'`；`routeAdvanced(kind,action,supported:Set<AdvancedAction>):'remote'|'product'|'unsupported'`。remote使用实际ops签名；product只调用后端已实现且验收的命令，不把删除当archive。

- [ ] **Step 1：不支持的产品fork绝不转发远程或伪造成功。**

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import {routeAdvanced} from './action-route.ts';
test('advanced actions stay with their backend',()=>{
 assert.equal(routeAdvanced('product','fork',new Set()),'unsupported');
 assert.equal(routeAdvanced('remote','fork',new Set(['fork'])),'remote');
});
```

- [ ] **Step 2：RED。** `pnpm exec tsx --test packages/domain/src/mobile/action-route.test.ts`。
- [ ] **Step 3：实现能力路由与真实操作映射。**

```ts
export type BackendKind='product'|'remote';
export type AdvancedAction='goal'|'fork'|'sideChat'|'archive'|'rewind'|'duplicate';
export function routeAdvanced(kind:BackendKind,action:AdvancedAction,supported:Set<AdvancedAction>){
 return supported.has(action)?kind:'unsupported';
}
```

通过每binding实例的 `RemoteProvider.invoke` 分派，H25的method白名单包含本任务操作；禁止直接调用全局singleton。逐项连接 `sessionGoalAction`、`forkAndSpawn`、`spawnSideChat`、`sessionArchive`、Claude/Codex rewind/duplicate；保持原协议的source message UUID/item ID，不能以产品MessageID代替。目标变化从后端确认更新，RPC未知结果先查询再允许重试。旁支返回新会话ID并带父来源，原会话仍保留，archive与delete区别保留。

产品对应能力若不存在，矩阵记录准确缺失：目标需持久目标状态/更新命令；fork需授权历史边界与文件引用策略；archive需持久归档过滤。不能在本任务静默引入一套新领域模型。先完成相应产品专项规格和独立后端切片再将该格标accepted；remote协议原能力应独立真实验收。完整“所有产品Agent都有这些能力”不属于已确认规格，未来扩展不能被写成既有支持。

- [ ] **Step 4：GREEN与交互矩阵。** 验证受控remote会话的目标、fork、sideChat、archive、rewind/duplicate所有支持动作；同UI的product模式明确展示支持/不支持且不触发远程RPC。若矩阵要求的Happy原能力在选定CLI版本无法实现，记录版本阻塞，不能删除该行。
- [ ] **Step 5：提交。** stage本任务路径；`git commit -m "feat(mobile): route advanced session actions to verified capabilities"`。
