# Craft 02 网页与 React 工作台 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用户可创建作品、上传数据、生成网页、预览、修改和下载指定版本。

**Architecture:** 后端发布不可变产物，隔离预览；共享前端层维护服务端快照和事件投影。assistant-ui只负责对话，作品视图与平台入口独立。

**Tech Stack:** Go/Gin/GORM、现有 Sandbox/对象存储、tRPC-Agent-Go/OpenCode、React/assistant-ui、TypeScript/pnpm；具体依赖沿专项锁定版本。

**Spec:** [产品方案](../specs/2026-09-10-onyx-craft-product-proposal.md)；必读 [总计划及共享契约](2026-09-10-craft-product-implementation.md)。

## Global Constraints

- “首版同一工作区串行修改。”
- “主 Run、ToolCall 和恢复调度复用现有 tRPC 恢复方案。”
- “关闭或刷新页面只影响订阅，不自动重新提交任务。”
- “历史版本引用保持原始内容，不能静默重定向到最新文件。”
- “基础权限与取消安全是每个可运行版本的前提”。
- “本方案不设定套餐、价格或充值规则，商业权威保持在现有专项方案。”
- 本文所有步骤为待执行计划。继承总计划全部门禁、错误类型、身份/版本约束，不把测试替身结果当成上线证据。

---

## 文件结构与执行约定

文件所有权在各任务 Files 中列出；接口定义只在对应责任模块维护。按任务依赖执行，修改共享入口前核对其他任务变更。Go 示例的测试文件使用被测包及 `testing`，生产代码按代码块使用的标准库导入；外部 craft 类型从 `github.com/Tencent/WeKnora/internal/craft` 导入。TypeScript 测试使用 `node:test` 和 `node:assert/strict`；UI 浏览器测试另用 Playwright。最小代码展示核心规则，后续集成动作和验收表同属必做内容，不能只实现纯函数就勾选整个任务。

### Task W01: 不可变产物与验证事实

**依赖与用户结果：** 依赖 R02/R05；用户可获取明确版本，不因后续修改改变旧下载。

**Files:**
- Create: `internal/craft/version.go`
- Test: `internal/craft/version_test.go`
- Create: `internal/application/service/craft_artifacts.go`
- Test: `internal/application/service/craft_artifacts_test.go`
- Create: `internal/application/repository/craft_version.go`
- Create: `migrations/versioned/000095_craft_versions.up.sql`
- Create: `migrations/versioned/000095_craft_versions.down.sql`
- Create: `migrations/sqlite/000016_craft_versions.up.sql`
- Create: `migrations/sqlite/000016_craft_versions.down.sql`

**Interfaces:**
- Consumes：`craft.Version/File/Check` 与现有 ArtifactCollector、resource存储/权限入口。
- Produces：`VersionKey(workspaceID,runID,digest string) string`；`VersionStore.Publish(ctx context.Context,scope Scope,v Version)(Version,error)`、`List(ctx context.Context,scope Scope)([]Version,error)`、`Get(ctx context.Context,scope Scope,id string)(Version,error)`；`CraftArtifactService.Collect(ctx context.Context,task craft.Task)(craft.Version,error)`。`VersionStore` 定义于craft/version.go；实现 `NewCraftVersionStore(*gorm.DB) craft.VersionStore`。

- [ ] **Step 1：先写失败测试。**

```go
func TestVersionKeyChangesWithContent(t *testing.T) {
    a:=VersionKey("w","r","sha1")
    if a==VersionKey("w","r","sha2") {t.Fatal("mutable version")}
    if a!=VersionKey("w","r","sha1") {t.Fatal("publish retry changed key")}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/craft -run TestVersionKey -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
func VersionKey(workspaceID,runID,digest string) string {
    raw,_:=json.Marshal([]string{workspaceID,runID,digest})
    sum:=sha256.Sum256(raw)
    return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 4：** 扩展迁移craft_versions/版本文件表：tenant/workspace/run、kind、manifest_hash、resource_ref、file_hash/size、检查列表、created_at；唯一workspace/run/manifest_hash。使用本任务独立迁移，down按依赖顺序先删文件表再删版本表，不更改R02已发布迁移。编号冲突按总计划整体重编号。

- [ ] **Step 5：** Collect从绑定output读取规则内文件；按文件路径排序生成清单摘要，拒绝路径穿越、symlink、超限和含凭据文件。复用现有资源存储并将内容摘要纳入不可变对象身份，不能只用path+mtime判断同一版本。先上传完整对象再事务publish，失败对象通过O03延迟回收。

- [ ] **Step 6：** 为构建退出码、入口存在、页面预览检查分别写Check，未运行填not_run。只有发布完成的Version关联tool结果；输出文字成功但缺文件返回failed。W02未检查预览时不能写preview passed。

- [ ] **Step 7：** 服务测试真实存储替身写v1/v2后读v1，比较原始bytes；两次相同publish返回同ID；对象上传失败不生成可见版本；不同tenant不能Get。测试fixture内创建实际临时目录和不同内容/相同mtime文件，验证不会漏新版本。

- [ ] **Step 8：运行 GREEN 与验收。**

```bash
go test ./internal/craft -run TestVersionKey -count=1
```

预期 PASS。还必须逐项确认：

- `go test ./internal/application/service -run TestCraftArtifact -count=1` 与repository版本测试通过。
- down迁移同时删除版本相关表；历史resource语义与已有消息产物兼容。

- [ ] **Step 9：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/craft/version.go internal/craft/version_test.go internal/application/service/craft_artifacts.go internal/application/service/craft_artifacts_test.go internal/application/repository/craft_version.go migrations/versioned/000095_craft_versions.up.sql migrations/versioned/000095_craft_versions.down.sql migrations/sqlite/000016_craft_versions.up.sql migrations/sqlite/000016_craft_versions.down.sql
git diff --cached --check
git commit -m "feat(craft): w01 不可变产物与验证事实"
```

### Task W02: 独立 origin 受控预览

**依赖与用户结果：** 依赖 W01；用户预览确为当前版本文件，脚本无法读取主站会话。

**Files:**
- Create: `internal/craft/preview.go`
- Test: `internal/craft/preview_test.go`
- Create: `internal/handler/session/craft_preview.go`
- Create: `internal/application/service/craft_preview.go`
- Test: `internal/handler/session/craft_preview_test.go`
- Create: `deploy/craft/preview.conf`

**Interfaces:**
- Consumes：`VersionStore.Get`、现有资源授权读取。
- Produces：`PreviewOriginAllowed(appOrigin,previewOrigin string) bool`；`PreviewTicket { URL string; ExpiresAt time.Time; VersionID string }`；`CraftPreviewService.Issue(ctx context.Context,scope craft.Scope,versionID string)(craft.PreviewTicket,error)`；`Resolve(ctx context.Context,token,path string)(craft.File,error)`。ticket服务保存随机token摘要、version/允许路径、过期时间，不接受模型URL。

- [ ] **Step 1：先写失败测试。**

```go
func TestPreviewRejectsMainOrigin(t *testing.T) {
    if PreviewOriginAllowed("https://app.test","https://app.test") {t.Fatal("same origin")}
    if PreviewOriginAllowed("https://app.test","javascript:alert(1)") {t.Fatal("unsafe URL")}
    if !PreviewOriginAllowed("https://app.test","https://preview.test") {t.Fatal("isolated origin rejected")}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/craft -run TestPreviewRejects -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
func PreviewOriginAllowed(appOrigin,previewOrigin string) bool {
    a,e1:=url.Parse(appOrigin); p,e2:=url.Parse(previewOrigin)
    if e1!=nil || e2!=nil || a.Host=="" || p.Host=="" || p.User!=nil {return false}
    if p.Scheme!="https" || p.Path!="" || p.RawQuery!="" || p.Fragment!="" {return false}
    return !strings.EqualFold(a.Scheme+"://"+a.Host,p.Scheme+"://"+p.Host)
}
```

- [ ] **Step 4：** 首个网页闭环只服务不可变静态HTML/CSS/JS及相对资源；生成技能把数据打包，不能依赖任意本地开发服务。需要后端服务的内部应用必须返回“不支持此预览类型”，不得代理模型给出的任意端口/URL。

- [ ] **Step 5：** 为preview.conf配置独立origin和Host-only凭证策略；签发256bit随机短期ticket（默认5min），只保存摘要。主认证请求签发后由隔离origin一次兑换成只读、5min到期的随机路径能力 `/p/<opaque-capability>/index.html`，相对CSS/JS/图片继承该路径；每次读取都验证能力对应的版本和允许文件。该预览不依赖第三方cookie，初始兑换ticket与只读资源能力都不进持久事件/日志。禁止根绝对资源路径绕过版本前缀，清单构建时重写或拒绝；刷新过期预览重新经主站授权签发。

- [ ] **Step 6：** iframe sandbox默认allow-scripts，禁止same-origin/top-navigation/popups/forms；CSP限制connect-src none，资源只允许受控预览源，禁用ServiceWorker注册；Referrer-Policy no-referrer、nosniff。响应Content-Type按后端已验证MIME，拒绝请求路径二次解码绕过。预览源不能携带主站cookie/Authorization。

- [ ] **Step 7：** handler测试跨tenant/过期ticket/路径穿越/旧版本404，浏览器在W06验证脚本访问parent/cookie失败、外联被阻止、相对CSS/图片/JS可加载。若必须支持模块脚本，明确CORS仅开放预览所需资源，不能因此移除sandbox隔离。

- [ ] **Step 8：运行 GREEN 与验收。**

```bash
go test ./internal/craft -run TestPreviewRejects -count=1
```

预期 PASS。还必须逐项确认：

- 签发成功不等于页面可用；W06真实资源加载后才写preview检查通过。
- 生产origin未配置则功能不启用；本机HTTPS测试origin单独配置。

- [ ] **Step 9：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/craft/preview.go internal/craft/preview_test.go internal/handler/session/craft_preview.go internal/application/service/craft_preview.go internal/handler/session/craft_preview_test.go deploy/craft/preview.conf
git diff --cached --check
git commit -m "feat(craft): w02 独立 origin 受控预览"
```

### Task W03: 作品会话与 HTTP 入口

**依赖与用户结果：** 依赖 R05/R06/W01；创建、上传关联、列表、继续修改和下载遵循现有权限。

**Files:**
- Create: `internal/craft/request.go`
- Test: `internal/craft/request_test.go`
- Create: `internal/handler/session/craft.go`
- Test: `internal/handler/session/craft_test.go`
- Create: `internal/application/service/craft_session.go`
- Test: `internal/application/service/craft_session_test.go`
- Modify: `internal/router/routes_chat.go`
- Modify: `internal/container/container.go`

**Interfaces:**
- Consumes：总计划API表、tRPC `Submit(ctx,runtime.Admission)(runtime.Run,error)`、R03 Stage、W01 VersionStore。
- Produces：`CreateRequest { RequestID,Title,Kind string }`；`ValidateCreate(CreateRequest) error`；`CraftSessionService.Create(ctx context.Context,scope craft.Scope,in craft.CreateRequest)(craft.Workspace,error)`、`Get(ctx context.Context,scope craft.Scope)(craft.Workspace,error)`。创建时Scope.SessionID允许空，由事务新建；后续所有入口必须非空。HTTP DTO固定snake_case字段。

- [ ] **Step 1：先写失败测试。**

```go
func TestCreateRequiresKnownKindAndKey(t *testing.T) {
    if ValidateCreate(CreateRequest{Title:"x",Kind:"web"})==nil {t.Fatal("no key")}
    if ValidateCreate(CreateRequest{RequestID:"a",Title:"x",Kind:"public-hosting"})==nil {t.Fatal("unsupported kind")}
    if err:=ValidateCreate(CreateRequest{RequestID:"a",Title:"x",Kind:"web"});err!=nil {t.Fatal(err)}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/craft -run TestCreateRequires -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
type CreateRequest struct { RequestID,Title,Kind string }
func ValidateCreate(r CreateRequest) error {
    if strings.TrimSpace(r.RequestID)=="" || strings.TrimSpace(r.Title)=="" || utf8.RuneCountInString(r.Title)>120 {return ErrInvalidInput}
    switch r.Kind {case "web","document","spreadsheet","slides":return nil}
    return ErrUnsupported
}
```

- [ ] **Step 4：** 按总计划完整注册API表；增加feature gate再次过滤kind，类型合法不意味着已开放。创建session固定engine_type=trpc；幂等键在tenant+user作用域事务保存request hash，第二请求返回同session，同键异参409。列表沿现有共享访问规则过滤，标题支持编辑但不更换引擎。

- [ ] **Step 5：** POST inputs只关联现有上传完成的resource；下载版本时验证session→workspace→version→file链。POST runs校验所有input_refs属于此工作区；新主Run通过既有Submit受理，不能HTTP直接启动goroutine。active Run含waiting_user阻止并发修改，409包含已有run ID。

- [ ] **Step 6：** 新增handler集成测试：owner创建/上传/两轮、viewer仅查看下载、跨tenant ID不可见、无权限编辑403、同键相同/不同参数、旧session builtin不能转Craft、依赖未就绪503。请求主体限制1MiB，prompt限制64KiB，服务端不接受client tenant/worker/沙箱身份。

- [ ] **Step 7：运行 GREEN 与验收。**

```bash
go test ./internal/craft -run TestCreateRequires -count=1
```

预期 PASS。还必须逐项确认：

- 既有chat/agent路由回归通过；不能为了Craft改默认engine。
- 运行 `go test ./internal/handler/session ./internal/application/service -run TestCraft -count=1`。

- [ ] **Step 8：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/craft/request.go internal/craft/request_test.go internal/handler/session/craft.go internal/handler/session/craft_test.go internal/application/service/craft_session.go internal/application/service/craft_session_test.go internal/router/routes_chat.go internal/container/container.go
git diff --cached --check
git commit -m "feat(craft): w03 作品会话与 HTTP 入口"
```

### Task W04: 共享契约与工作台状态控制

**依赖与用户结果：** 依赖G2、W03契约；刷新与重复事件不产生重复消息或跨空间串流。

**Files:**
- Create: `packages/contracts/src/craft/index.ts`
- Test: `packages/contracts/src/craft/index.test.ts`
- Modify: `packages/contracts/src/index.ts`
- Modify: `packages/api-client/src/index.ts`
- Modify: `packages/domain/package.json`
- Modify: `packages/core/package.json`
- Modify: `package.json`
- Create: `packages/api-client/src/craft/index.ts`
- Create: `packages/domain/src/craft/state.ts`
- Test: `packages/domain/src/craft/state.test.ts`
- Create: `packages/core/src/craft/controller.ts`
- Test: `packages/core/src/craft/controller.test.ts`

**Interfaces:**
- Consumes：总计划WorkspaceView/RunEvent；现有 `WeKnoraClient.request(input:ClientRequest):Promise<unknown>`、RequestScope。
- Produces：`CraftState { generation:number; runId:string|null; seq:number; mainStatus:string; delegationStatus:string; versionId:string|null }`；`CraftEvent { generation:number;runId:string;seq:number;kind:string;versionId?:string }`；`applyCraftEvent(s:CraftState,e:CraftEvent):CraftState`。
- API工厂 `createCraftApi(request:WeKnoraClient['request'])` 暴露 create/list/get/addInput/submit/versions/version/preview/download；参数与返回按总计划API表逐项定义，不以any代替schema。controller `load(sessionId:string):Promise<void>`、`submit(prompt:string):Promise<void>`、`dispose():void`，构造时注入api、scope、既有SSE transport。

- [ ] **Step 1：先写失败测试。**

```typescript
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { applyCraftEvent } from './state.ts';
test('child completion never finishes main run', () => {
  const s={generation:1,runId:'r',seq:0,mainStatus:'running',delegationStatus:'running',versionId:null};
  const n=applyCraftEvent(s,{generation:1,runId:'r',seq:1,kind:'delegation.finished'});
  assert.equal(n.mainStatus,'running');
  assert.equal(applyCraftEvent(n,{generation:1,runId:'r',seq:1,kind:'delegation.finished'}),n);
  assert.equal(applyCraftEvent(n,{generation:0,runId:'r',seq:2,kind:'delegation.finished'}),n);
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test packages/domain/src/craft/state.test.ts
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```typescript
export interface CraftState { generation:number; runId:string|null; seq:number; mainStatus:string; delegationStatus:string; versionId:string|null }
export interface CraftEvent { generation:number; runId:string; seq:number; kind:string; versionId?:string }
export function applyCraftEvent(s:CraftState,e:CraftEvent):CraftState {
  if(e.generation!==s.generation || e.runId!==s.runId || e.seq<=s.seq) return s;
  return {...s,seq:e.seq,
    delegationStatus:e.kind==='delegation.finished'?'finished':s.delegationStatus,
    versionId:e.kind==='artifact.published'?(e.versionId??s.versionId):s.versionId};
}
```

- [ ] **Step 4：** contracts为每API响应定义解析器：缺少ID/未知状态/不合法seq拒绝，不以类型断言吞错；JSON沿现有data envelope。下载走平台transport，不把文件bytes塞进领域状态。新增导出仅在G2相应package已存在时做，不自行另建React工程。

- [ ] **Step 5：** controller先get快照，再用last_seq订阅；按run+seq合并。网络错误只重连订阅，不再次submit；request_id一次用户发送生成一次，超时重试同键。generation变化立即销毁旧订阅并清空作品/附件选择。

- [ ] **Step 6：** 主终态只消费既有Run状态投影；子event函数不决定主状态。为seq缺口/过期cursor进入C03重载流程预留明确返回错误，不能跳过。控制器测试fake transport记录submit次数1、刷新重订阅次数增加但不提交。

- [ ] **Step 7：** 在根test/typecheck脚本加入精确craft路径，沿React现有工具执行；当前根test:shared不递归匹配craft目录，不能仅跑旧命令就声称覆盖。测试导出与NodeNext扩展名保持已建立约定。

- [ ] **Step 8：运行 GREEN 与验收。**

```bash
pnpm exec tsx --test packages/domain/src/craft/state.test.ts
```

预期 PASS。还必须逐项确认：

- controller generation隔离、API错误解码、同键重试与子终态不结束主消息均通过。
- 新增core/views是否存在由G2验证；依赖缺失时只完成契约与纯状态部分，不标W04完成。

- [ ] **Step 9：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add packages/contracts/src/craft/index.ts packages/contracts/src/craft/index.test.ts packages/contracts/src/index.ts packages/api-client/src/index.ts packages/domain/package.json packages/core/package.json package.json packages/api-client/src/craft/index.ts packages/domain/src/craft/state.ts packages/domain/src/craft/state.test.ts packages/core/src/craft/controller.ts packages/core/src/craft/controller.test.ts
git diff --cached --check
git commit -m "feat(craft): w04 共享契约与工作台状态控制"
```

### Task W05: assistant-ui 对话与作品面板

**依赖与用户结果：** 依赖W02/W04、G2；提供可实际操作的创作入口和左右工作台。

**Files:**
- Create: `packages/views/src/craft/home.tsx`
- Create: `packages/views/src/craft/workbench.tsx`
- Create: `packages/views/src/craft/preview.tsx`
- Create: `packages/views/src/craft/files.tsx`
- Create: `packages/views/src/craft/presentation.ts`
- Test: `packages/views/src/craft/presentation.test.ts`
- Create: `apps/web/src/features/craft/routes.tsx`

**Interfaces:**
- Consumes：W04 controller/state，G2对话运行适配；`Version` schema来自contracts。
- Produces：`CraftHome`、`CraftWorkbench`、`CraftPreview`、`CraftFiles` React组件；props只接数据/回调，不读全局localStorage。`statusLabel(main:string,child:string):string`，assistant-ui ExternalStoreRuntime从后端消息投影读取，不在组件内推导执行成功。

- [ ] **Step 1：先写失败测试。**

```typescript
import { test } from 'node:test';
import assert from 'node:assert/strict';
import {statusLabel} from './presentation.ts';
test('child finished still shows main verification',()=>{
  assert.equal(statusLabel('running','finished'),'主 Agent 正在检查结果');
  assert.equal(statusLabel('stopping','running'),'正在停止');
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test packages/views/src/craft/presentation.test.ts
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```typescript
export function statusLabel(main:string,child:string):string {
  if(main==='stopping') return '正在停止';
  if(main==='waiting_user') return '需要你的处理';
  if(main==='running' && child==='finished') return '主 Agent 正在检查结果';
  return ({queued:'等待执行',running:'正在生成',succeeded:'已完成',failed:'执行失败',canceled:'已停止'} as Record<string,string>)[main]??'状态待核对';
}
```

- [ ] **Step 4：** Home包含新建目标/附件/知识范围入口与最近作品分页。Workbench桌面左右分栏，右侧预览/文件/执行详情tab，顶栏标题/状态/当前版本/下载/版本历史；窄屏tab切换保留输入。按现有品牌tokens与i18n扩展中文/英文文案。

- [ ] **Step 5：** assistant-ui安装/版本和外部store adapter在G2统一锁定；Craft提供执行卡片、引用和R06问题/审批视图。只有main Run终态设置消息complete；卡片区分问题答复、批准范围与拒绝，不能给问题一个通用Approve按钮。

- [ ] **Step 6：** Preview加载/过期/失败/无产物四状态明确；切版本销毁原iframe与ticket，Files下载固定versionID；历史记录展示Run、生成时间和Check事实。仅在C05就绪后显示“从此版本继续编辑”。

- [ ] **Step 7：** 通过web现有路由装配登录/空间/写权限gate；调用W04 controller，不引入第二router。终端复用现有已授权Sandbox入口，置于执行详情的可选动作，默认收起；只读用户不显示可执行终端。

- [ ] **Step 8：** 为组件增加键盘焦点、面板resize、aria-live非逐token播报、空态/错误重试交互；重试预览不重试执行。浏览器断言在W06集中实现，避免只用label单测代替UI验收。

- [ ] **Step 9：运行 GREEN 与验收。**

```bash
pnpm exec tsx --test packages/views/src/craft/presentation.test.ts
```

预期 PASS。还必须逐项确认：

- 状态纯函数通过后仍须完成W06浏览器。
- 小屏输入可见、键盘可到达下载/版本按钮、iframe失败不遮住对话。

- [ ] **Step 10：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add packages/views/src/craft/home.tsx packages/views/src/craft/workbench.tsx packages/views/src/craft/preview.tsx packages/views/src/craft/files.tsx packages/views/src/craft/presentation.ts packages/views/src/craft/presentation.test.ts apps/web/src/features/craft/routes.tsx
git diff --cached --check
git commit -m "feat(craft): w05 assistant-ui 对话与作品面板"
```

### Task W06: 网页报告完整浏览器验收

**依赖与用户结果：** 依赖W01–W05、R07；首个对用户完整可用的B阶段交付。

**Files:**
- Modify: `apps/web/package.json`
- Modify: `pnpm-lock.yaml`
- Create: `apps/web/e2e/craft-report.spec.ts`
- Create: `apps/web/e2e/fixtures/craft-sales.csv`
- Create: `apps/web/playwright.craft.config.ts`
- Create: `docs/testing/craft/web-acceptance.md`

**Interfaces:**
- Consumes：W03真实HTTP、W05可访问路由 `/craft`、`/craft/:sessionId`，测试账号由环境注入。
- Produces：浏览器规范固定 `data-testid`：craft-goal、craft-create、craft-upload、craft-send、craft-prompt、craft-preview、craft-version、craft-download、craft-main-status；同名只用于相应唯一控件。Playwright config使用web已有认证storageState环境路径，不在仓库提交凭据。

- [ ] **Step 1：先写失败测试。**

```typescript
import {test,expect} from '@playwright/test';
test('sales report can be modified and old version downloaded',async({page})=>{
  await page.goto('/craft');
  await page.getByTestId('craft-goal').fill('按月分析销售数据，生成可筛选网页报告');
  await page.getByTestId('craft-create').click();
  await page.getByTestId('craft-upload').setInputFiles('e2e/fixtures/craft-sales.csv');
  await page.getByTestId('craft-send').click();
  await expect(page.getByTestId('craft-main-status')).toHaveText('已完成',{timeout:180000});
  const first=await page.getByTestId('craft-version').inputValue();
  await page.getByTestId('craft-prompt').fill('改为季度汇总，保留地区筛选');
  await page.getByTestId('craft-send').click();
  await expect(page.getByTestId('craft-version')).not.toHaveValue(first,{timeout:180000});
  await page.getByTestId('craft-version').selectOption(first);
  const download=page.waitForEvent('download');
  await page.getByTestId('craft-download').click();
  expect((await download).suggestedFilename()).not.toBe('');
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm --dir apps/web exec playwright test -c playwright.craft.config.ts e2e/craft-report.spec.ts
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```typescript
// playwright.craft.config.ts
import {defineConfig} from '@playwright/test';
export default defineConfig({
  testDir:'./e2e',testMatch:'craft-*.spec.ts',workers:1,retries:0,
  use:{acceptDownloads:true,baseURL:process.env.CRAFT_WEB_URL??'http://localhost:5173',
       storageState:process.env.CRAFT_AUTH_STATE,trace:'retain-on-failure'},
  timeout:240000
});
```

- [ ] **Step 4：** 将UI测试使用的testid加到W05组件；CSV固定两月两个地区总额300。首次下载保存bytes/SHA，再修改并再次下载旧版，断言SHA完全一致、新版不同；通过真实iframe读取标题/图表总额300和筛选结果。

- [ ] **Step 5：** 增加预览恶意fixture验证父站cookie/DOM访问失败和外部fetch阻止；跨tenant下载404/403；viewer编辑403；切空间旧stream不追加；刷新生成中页面POST runs仍1次。用后端日志/数据库计数校验，不能靠UI文案判断无重复。

- [ ] **Step 6：** 有条件安装项目已锁定Playwright依赖并维护lockfile，若尚无版本先在本任务固定后提交；不使用临时npx latest。启动真实Go/DB/存储/OpenCode/React及独立预览origin，记录依赖SHA、配置与截图；测试fixture模型和真实模型各跑一次。

- [ ] **Step 7：运行 GREEN 与验收。**

```bash
pnpm --dir apps/web exec playwright test -c playwright.craft.config.ts e2e/craft-report.spec.ts
```

预期 PASS。还必须逐项确认：

- B阶段验收八项：创建、上传、主检索/理解、子生成、预览、修改、旧版下载、再次打开。
- 文案“已完成”与主Run终态相同；无模型/环境不能运行时此任务保持未完成。

- [ ] **Step 8：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add apps/web/package.json pnpm-lock.yaml apps/web/e2e/craft-report.spec.ts apps/web/e2e/fixtures/craft-sales.csv apps/web/playwright.craft.config.ts docs/testing/craft/web-acceptance.md
git diff --cached --check
git commit -m "feat(craft): w06 网页报告完整浏览器验收"
```
