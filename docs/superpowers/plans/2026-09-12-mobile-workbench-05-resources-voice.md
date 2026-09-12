# 移动工作台 05：文件、产物与语音 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付可授权上传、版本化导出、安全预览、专业结果与受预算约束的语音交互。

**Architecture:** 复用现有附件/产物与知识权限入口，按需移植AWS示例的独立交互。语音控制走产品API，媒体走短期供应商会话；文字、审批、产物和费用都绑定产品Run。

**Tech Stack:** 现有文件服务/对象存储、Expo文件与音频端口、React Native WebView、Go/GORM；AWS示例只按固定来源移植。

**Spec:** [技术架构](../specs/2026-09-12-mobile-ai-saas-workbench-architecture.md) §6、§10–12；[总计划](2026-09-12-mobile-ai-saas-workbench.md)。执行者同时读取本册与规格。

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

文件传输、产物元数据、预览安全、渲染器和语音会话分开。W25/W28/W29可在平台链独立交付；远程导出W26依赖Paseo，不阻止已有平台附件上线。

### W25：会话附件上传、取消和校验

**依赖：** W07、W10。

**Files：**

- Create: `apps/mobile/sources/weknora/resources/upload.ts`、`apps/mobile/sources/weknora/resources/upload.test.ts`
- Modify: `internal/handler/session/temporary_document.go`、`packages/api-client/src/knowledge/documents.ts`
- Create: `internal/handler/session/mobile_upload_test.go`

**Interfaces：**

Produces `UploadInput{sessionID,uri,name,mime:string;size:number}`；`validateUpload(input:UploadInput,limit:number):void`；`UploadPort.send(input,signal):Promise<{attachmentID:string}>`。复用现有session attachments路由，原生content://与file://由本地文件端口转成multipart，不发送设备本地路径给后端当文件。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { validateUpload } from './upload.ts';
test('oversized attachment is rejected before network access',()=>{
 const f={sessionID:'s',uri:'file:///tmp/a.pdf',name:'a.pdf',mime:'application/pdf',size:11};
 assert.throws(()=>validateUpload(f,10),/UPLOAD_TOO_LARGE/);
 assert.doesNotThrow(()=>validateUpload({...f,size:10},10));
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test apps/mobile/sources/weknora/resources/upload.test.ts
```

预期：validateUpload未定义。

- [ ] **Step 3：实现最小行为。**

```ts
export interface UploadInput {sessionID:string;uri:string;name:string;mime:string;size:number}
export function validateUpload(f:UploadInput,limit:number){
 if(!Number.isSafeInteger(f.size)||f.size<0||!Number.isSafeInteger(limit)||limit<=0)throw new Error('INVALID_UPLOAD');
 if(f.size>limit)throw new Error('UPLOAD_TOO_LARGE');
 if(!f.sessionID||!f.name||!f.uri)throw new Error('INVALID_UPLOAD');
}
```
服务端限制请求体、核对真实MIME/摘要/文件大小和会话权限；扫描未完成不得供Agent读取。保留原attachment ID契约，不另建无归属上传桶。

- [ ] **Step 4：接通实际入口。**

输入框选择/拍照/系统分享入口调用UploadPort；取消abort并清理未完成上传记录，失败不丢草稿。空间切换使迟到结果失效。允许类型与大小从部署能力得到，客户端检查仅体验层，后端再次检查。图片压缩保留原始关联，不能把缩略图误作原文件。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- Android content URI、iOS照片权限、文档选择、取消中上传、切空间、零字节、伪MIME、超限。
- 服务端另一owner附件读取/上传均拒绝；真实文件上传后Agent可按权限引用。
- 临时对象过期清理与已引用对象不误删；不复用AWS个人API Key和本地历史模式。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add apps/mobile/sources/weknora/resources/upload.ts apps/mobile/sources/weknora/resources/upload.test.ts internal/handler/session/temporary_document.go packages/api-client/src/knowledge/documents.ts internal/handler/session/mobile_upload_test.go
git diff --cached --check
git diff --cached --stat
git commit -m "feat(mobile): upload scoped attachments with cancellation"
```

### W26：远程文件导入与不可变产物版本

**依赖：** W18、W21、W25。

**Files：**

- Create: `internal/application/repository/artifact_version.go`、`internal/application/repository/artifact_version_test.go`
- Create: `services/paseo-adapter/src/artifact-export.ts`、`services/paseo-adapter/src/artifact-export.test.ts`
- Modify: `internal/handler/session/artifact_download.go`
- Create: `migrations/versioned/000130_artifact_versions.{up,down}.sql`、`migrations/sqlite/000050_artifact_versions.{up,down}.sql`

**Interfaces：**

Produces `ArtifactVersion{TenantID uint64;ID,RunID,SessionID,Digest,ObjectKey,MIME,ScanState string;Size int64}`；`NewArtifactVersionStore(db *gorm.DB)`、`Insert(ctx,row)error`；`validateArtifactMetadata(size:int64,digest:string)error`。Bridge export命令传workspace_ref和相对file_ref，返回流/摘要，不接受宿主绝对路径。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```go
package repository
import (
 "context"
 "testing"
 "github.com/stretchr/testify/require"
)
func TestArtifactVersionIsImmutable(t *testing.T) {
 s:=NewArtifactVersionStore(openRunTestDB(t));ctx:=context.Background()
 v:=ArtifactVersion{TenantID:1,ID:"v1",RunID:"r",SessionID:"s1",Digest:"a",ObjectKey:"k1",MIME:"text/plain",ScanState:"clean",Size:1}
 require.NoError(t,s.Insert(ctx,v))
 v.ObjectKey="k2"; require.Error(t,s.Insert(ctx,v))
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/application/repository -run TestArtifactVersion -count=1
```

预期：ArtifactVersionStore未定义。

- [ ] **Step 3：实现最小行为。**

Insert只插入，不upsert覆盖版本；同ID相同完整字段可幂等返回，不同内容冲突。对象key服务端生成，不由客户端提供。
```sql
CREATE UNIQUE INDEX artifact_version_identity ON artifact_versions(tenant_id, id);
CREATE INDEX artifact_version_run ON artifact_versions(tenant_id, run_id);
```
Bridge导出以已授权根目录描述符为边界逐段打开，拒绝..、绝对路径、符号链接逃逸；验证最终文件类型及size，读取过程中超限停止。不仅做字符串startsWith检查。

- [ ] **Step 4：接通实际入口。**

导入状态pending→uploaded→scanned→ready；扫描失败隔离不发布URL。导出时检查run/target/workspace授权和版本；上传能力令牌限定对象key、方法、大小和期限。handler复用既有artifact访问归属检查，旧handle读取保持兼容，新versionID显式区分。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 不可变版本、重复导入、上传成功但元数据失败后对账、未扫描不能读取。
- 路径编码、符号链接和读取期间替换文件、device/socket文件、大文件；安全负例实际触达节点读取端。
- 已取消Run可授权导出已生成文件，但不能借导出执行新工具；签名URL过期和跨空间下载。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add internal/application/repository/artifact_version.go internal/application/repository/artifact_version_test.go services/paseo-adapter/src/artifact-export.ts services/paseo-adapter/src/artifact-export.test.ts internal/handler/session/artifact_download.go migrations/versioned/000130_artifact_versions.up.sql migrations/versioned/000130_artifact_versions.down.sql migrations/sqlite/000050_artifact_versions.up.sql migrations/sqlite/000050_artifact_versions.down.sql
git diff --cached --check
git diff --cached --stat
git commit -m "feat(artifacts): import immutable scoped execution outputs"
```

### W27：隔离预览与 AWS 组件来源

**依赖：** W25、W26（远程版本场景）。

**Files：**

- Create: `apps/mobile/sources/weknora/resources/preview-policy.ts`、`apps/mobile/sources/weknora/resources/preview-policy.test.ts`、`apps/mobile/sources/weknora/resources/ArtifactPreview.tsx`
- Create: `internal/handler/artifact_preview.go`、`internal/handler/artifact_preview_test.go`
- Create: `docs/migrations/aws-mobile/source-manifest.json`

**Interfaces：**

Produces `previewHeaders():Record<string,string>`；ArtifactPreview props={previewURL:string;artifactVersionID:string}。独立preview origin无产品cookie，服务端按授权生成短期只读预览引用。来源清单含AWS固定commit、路径、license、移植理由与依赖。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { previewHeaders } from './preview-policy.ts';
test('preview cannot use product credentials or connect to APIs',()=>{
 const h=previewHeaders();
 assert.match(h['Content-Security-Policy'],/connect-src 'none'/);
 assert.match(h['Content-Security-Policy'],/sandbox allow-scripts/);
 assert.equal(h['Referrer-Policy'],'no-referrer');
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test apps/mobile/sources/weknora/resources/preview-policy.test.ts
```

预期：previewHeaders未定义。

- [ ] **Step 3：实现最小行为。**

```ts
export function previewHeaders(){return {
 'Content-Security-Policy':"default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; connect-src 'none'; form-action 'none'; base-uri 'none'; sandbox allow-scripts",
 'Referrer-Policy':'no-referrer',
 'X-Content-Type-Options':'nosniff',
 'Cache-Control':'private, no-store',
};}
```
上述允许静态产物内联脚本但不允许网络、表单和同源权限。服务端实际响应设置相同策略，客户端常量只是测试对照，不能只给WebView配置一份不生效的header。

- [ ] **Step 4：接通实际入口。**

独立origin仅接收artifact版本引用，后端解析对象；禁止任意URL代理、内部网络抓取和产品token注入。WebView关闭任意文件访问/桥，导航白名单仅预览，外链需明确交给系统浏览器。大型图表/代码延迟渲染；从AWS只选独立渲染组件，不导入其服务器配置。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 实际预览HTML尝试fetch产品API、读取cookie/localStorage、postMessage原生命令、顶层导航、表单提交和file URI，全部被阻止。
- 同时有合法静态图表/代码预览成功组；iOS与Android行为各验证。
- 资源权限撤销后新预览票据拒绝；已有票据期限限制明确。动态Web App外部能力调用不在本任务开放。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add apps/mobile/sources/weknora/resources/preview-policy.ts apps/mobile/sources/weknora/resources/preview-policy.test.ts apps/mobile/sources/weknora/resources/ArtifactPreview.tsx internal/handler/artifact_preview.go internal/handler/artifact_preview_test.go docs/migrations/aws-mobile/source-manifest.json
git diff --cached --check
git diff --cached --stat
git commit -m "feat(artifacts): isolate generated previews from product credentials"
```

### W28：知识引用与专业结果注册器

**依赖：** W10、W25、W27。

**Files：**

- Create: `apps/mobile/sources/weknora/renderers/registry.ts`、`apps/mobile/sources/weknora/renderers/registry.test.ts`、`apps/mobile/sources/weknora/renderers/KnowledgeCitation.tsx`、`apps/mobile/sources/weknora/renderers/DataAnalysisResult.tsx`
- Modify: `apps/mobile/sources/weknora/conversations/ConversationScreen.tsx`

**Interfaces：**

Produces `selectRenderer(kind:string):'citation'|'table'|'file'|'text'`；组件只消费经过contracts校验的数据，引用依赖有授权的knowledge/document/chunk ID，不直接执行source URL。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { selectRenderer } from './registry.ts';
test('unknown server component never becomes executable UI',()=>{
 assert.equal(selectRenderer('knowledge.citation'),'citation');
 assert.equal(selectRenderer('javascript:eval'),'text');
 assert.equal(selectRenderer('remote-component'),'text');
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test apps/mobile/sources/weknora/renderers/registry.test.ts
```

预期：selectRenderer未定义。

- [ ] **Step 3：实现最小行为。**

```ts
export function selectRenderer(kind:string):'citation'|'table'|'file'|'text'{
 switch(kind){case 'knowledge.citation':return 'citation';case 'analysis.table':return 'table';case 'artifact.file':return 'file';default:return 'text';}
}
```
结构化表格校验列数/行数和单元格类型，超出上限改下载；引用点击重新请求权限，不把渲染器当授权层。每轮Agent ID随消息保存。

- [ ] **Step 4：接通实际入口。**

将现有知识问答引用与data-analysis preset结果映射成上述类型；未知结果安全文本或文件回退。资源入口复用产品知识/附件接口，不强制所有会话绑定知识库。跨空间知识遵循现有共享授权和执行空间费用归属，链接跳转不自行切商业账户。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 真实知识问答引用可打开获准来源，撤销后拒绝；数据分析附件→表格/文件真实链路。
- 巨大表格、长中文、未知type、损坏payload、RTL/大字体和无障碍标签。
- 未接通的专业类型不得以placeholder卡片标完成；原Happy普通文本/工具回归。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add apps/mobile/sources/weknora/renderers/registry.ts apps/mobile/sources/weknora/renderers/registry.test.ts apps/mobile/sources/weknora/renderers/KnowledgeCitation.tsx apps/mobile/sources/weknora/renderers/DataAnalysisResult.tsx apps/mobile/sources/weknora/conversations/ConversationScreen.tsx
git diff --cached --check
git diff --cached --stat
git commit -m "feat(mobile): render authorized knowledge and specialist results"
```

### W29：按住说话、转写确认与文本提交

**依赖：** W10、W25。

**Files：**

- Create: `apps/mobile/sources/weknora/voice/dictation.ts`、`apps/mobile/sources/weknora/voice/dictation.test.ts`、`apps/mobile/sources/weknora/voice/DictationInput.tsx`
- Modify: `apps/mobile/sources/weknora/conversations/ConversationScreen.tsx`

**Interfaces：**

Produces `applyTranscript(text:string,setDraft:(text:string)=>void):void`；`DictationPort{start():Promise<void>;stop():Promise<{uri:string;durationMs:number}>;cancel():Promise<void>;transcribe(uri:string):Promise<string>}`。转写服务通过产品鉴权与预算，不直用长期模型key。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { applyTranscript } from './dictation.ts';
test('transcript updates editable draft without sending',()=>{
 let draft='';applyTranscript('删除这个文件',value=>{draft=value;});
 assert.equal(draft,'删除这个文件');
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test apps/mobile/sources/weknora/voice/dictation.test.ts
```

预期：applyTranscript未定义。

- [ ] **Step 3：实现最小行为。**

```ts
export function applyTranscript(text:string,setDraft:(text:string)=>void){setDraft(text.trim());}
```
录音状态idle/recording/transcribing/ready/error；权限拒绝回文本输入；松开停止、取消删除临时音频。默认每次最长60秒，能力配置可收紧。接口中不接收send函数，转写不能自动执行任务。

- [ ] **Step 4：接通实际入口。**

DictationInput调用原生音频端口；来电/后台/切空间触发cancel。转写完成只填原输入草稿，用户确认仍用W10提交和W04预算。图片/附件上下文不因转写替换丢失。实时会话不是本任务的替代验收。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 拒绝麦克风、录音中断、转写超时、空音频、超过时长、中文标点；失败保留原草稿。
- 挂载输入组件并spy SDK确认转写完成没有start执行调用；用户点击发送后只一次。
- iOS与Android真机音频路径和蓝牙切换；模拟器不能证明真实录音质量。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add apps/mobile/sources/weknora/voice/dictation.ts apps/mobile/sources/weknora/voice/dictation.test.ts apps/mobile/sources/weknora/voice/DictationInput.tsx apps/mobile/sources/weknora/conversations/ConversationScreen.tsx
git diff --cached --check
git diff --cached --stat
git commit -m "feat(mobile): add reviewable voice dictation"
```

### W30：语音会话授权、短期令牌和结算

**依赖：** W04、W24、W29。

**Files：**

- Create: `internal/voice/session.go`、`internal/voice/session_test.go`、`internal/voice/provider.go`
- Create: `internal/handler/mobile_voice.go`、`internal/handler/mobile_voice_test.go`
- Create: `internal/application/repository/voice_session.go`
- Create: `migrations/versioned/000131_voice_sessions.{up,down}.sql`、`migrations/sqlite/000051_voice_sessions.{up,down}.sql`
- Modify: `internal/router/routes_workbench.go`、`internal/container/container.go`

**Interfaces：**

Produces `Grant{Token string;ExpiresAt time.Time;MaxSeconds int}`；`AuthorizeVoice(now,deadline time.Time,budget int64)(time.Time,error)`；`VoiceProvider.Open(ctx,sessionRef string,maxSeconds int)(Grant,error)`、`Close(ctx,sessionRef string)error`。新增POST `/mobile/voice/sessions`、DELETE `/:id`、POST `/mobile/voice/transcriptions`；ownership/budget从产品上下文解析。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```go
package voice
import (
 "testing"
 "time"
)
func TestVoiceCannotOutliveBudget(t *testing.T) {
 now:=time.Now(); deadline:=now.Add(30*time.Second)
 expiry,err:=AuthorizeVoice(now,deadline,1)
 if err!=nil||expiry.After(deadline){t.Fatal("voice exceeds task deadline")}
 if _,err=AuthorizeVoice(now,deadline,0);err==nil{t.Fatal("unfunded voice granted")}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/voice -run TestVoice -count=1
```

预期：AuthorizeVoice未定义。

- [ ] **Step 3：实现最小行为。**

```go
func AuthorizeVoice(now,deadline time.Time,budget int64)(time.Time,error){
 if budget<=0||!deadline.After(now){return time.Time{},errors.New("voice_not_authorized")}
 expiry:=now.Add(time.Minute);if deadline.Before(expiry){expiry=deadline};return expiry,nil
}
```
实际签发前持久预算准入与provider session映射，签发超时进入unknown并核对，不能反复开收费会话。provider长期密钥只在服务端；无法提供短期token的供应商走服务端媒体代理，不能把长期key返回手机。

- [ ] **Step 4：接通实际入口。**

VoiceSession保存tenant/owner/session/run/providerRef/deadline/usage状态；供应商用量经过可信身份校验与幂等结算。新增audio/image计量维度必须同步existing commercial白名单、价格版本与OpenMeter映射，未配置拒绝收费语音，不借text维度扣费。转写接口在本任务提供真实后端，W29此前可fixture开发但未真实验收。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 跨owner、超预算、deadline过期拒绝且provider calls=0；有效正例短期token可用且不超权限。
- 签发后客户端断线、stop重复、供应商回调重放、转写失败仍结算实际费用；token不写日志。
- 一个选定供应商真实音频和用量证据；未提供服务/计价时blocked-env。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add internal/voice/session.go internal/voice/session_test.go internal/voice/provider.go internal/handler/mobile_voice.go internal/handler/mobile_voice_test.go internal/application/repository/voice_session.go migrations/versioned/000131_voice_sessions.up.sql migrations/versioned/000131_voice_sessions.down.sql migrations/sqlite/000051_voice_sessions.up.sql migrations/sqlite/000051_voice_sessions.down.sql internal/router/routes_workbench.go internal/container/container.go
git diff --cached --check
git diff --cached --stat
git commit -m "feat(voice): authorize short-lived budgeted sessions"
```

### W31：实时语音、打断与后台进度展示

**依赖：** W12、W15、W30。

**Files：**

- Create: `apps/mobile/sources/weknora/voice/realtime.ts`、`apps/mobile/sources/weknora/voice/realtime.test.ts`、`apps/mobile/sources/weknora/voice/VoicePanel.tsx`
- Create: `apps/mobile/sources/weknora/notifications/live-progress.ts`
- Modify: `apps/mobile/sources/weknora/conversations/ConversationScreen.tsx`

**Interfaces：**

Produces `interruptPlayback(stopAudio:()=>void):void`；RealtimeVoicePort.connect(grant: {token:string;expiresAt:string}):Promise<void>, close():Promise<void>, mute(value:boolean):void。与W30 wire统一ExpiresAt RFC3339，音频停止与产品cancel分离。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { interruptPlayback } from './realtime.ts';
test('interrupt stops audio without canceling execution',()=>{
 let stopped=0;interruptPlayback(()=>{stopped++;});assert.equal(stopped,1);
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test apps/mobile/sources/weknora/voice/realtime.test.ts
```

预期：interruptPlayback未定义。

- [ ] **Step 3：实现最小行为。**

```ts
export function interruptPlayback(stopAudio:()=>void){stopAudio();}
```
VoicePanel分别提供静音、停止播放、结束语音连接；取消Agent仍是明确的产品命令。语音触发高风险操作必须出现W05结构化审批卡，口头模糊回复不自动批准。

- [ ] **Step 4：接通实际入口。**

连接按W30 grant创建，过期续期重新准入；后台/来电断开后恢复查询，不自行重开收费session。Live Activity/Android进度只订阅Run状态提示，不持有worker，不以后台音频保活执行。若移植AWS展示能力记录具体来源与原生依赖，不整体复制其工程。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 播放打断不产生cancel API；结束语音只close provider；用户明确取消任务才调用W05。
- 双平台扬声器/耳机/蓝牙、来电、权限撤销、网络切换、额度耗尽；短期token过期无无限重连。
- Live Activity缺失时普通通知和前台恢复仍可用；后台进度不泄露内容。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add apps/mobile/sources/weknora/voice/realtime.ts apps/mobile/sources/weknora/voice/realtime.test.ts apps/mobile/sources/weknora/voice/VoicePanel.tsx apps/mobile/sources/weknora/notifications/live-progress.ts apps/mobile/sources/weknora/conversations/ConversationScreen.tsx
git diff --cached --check
git diff --cached --stat
git commit -m "feat(mobile): separate realtime voice controls from task execution"
```


