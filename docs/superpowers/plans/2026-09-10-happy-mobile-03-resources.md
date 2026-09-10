# Happy 资源与专业展示 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在统一会话中接通知识资源、原生文件、产物和专业 Agent 的受控扩展。

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

## 文件责任

H14 使用现有知识 SDK，H15 负责临时会话附件而非知识库自动入库，H16 负责受保护产物与文件展示，H17 负责专业表单/结果注册。对照 H01 的文件、代码、Diff、预览、分享逐项映射，不能只实现下载就勾选整族。

### H14：知识资源选择、检索与引用

**依赖：** H07/H09、原 T07。**Files:** Create `apps/mobile/sources/weknora/resources/knowledge-selection.ts`、`knowledge-selection.test.ts`、`KnowledgeScreen.tsx`、`ReferenceScreen.tsx`；Create `packages/api-client/src/knowledge/search.ts`、`search.test.ts`；Modify 原生资源路由。

**Interfaces:** `setKnowledgeSelection(authorizedIds:string[],requestedIds:string[]):string[]`；现有 knowledgeBases.list/get 的契约继续使用，搜索 `searchKnowledge(query:string,knowledgeBaseIds:string[]):Promise<unknown>` 在 facade 校验为实际 handler DTO 后导出类型，运行时输入不接受客户端任意下载 URL。

- [ ] **Step 1：写撤销资源后选择不残留、空选择合法的测试。**

```ts
import {expect,test} from 'vitest';
import {setKnowledgeSelection} from './knowledge-selection';
test('empty resources are valid and revoked resources are removed',()=>{
 expect(setKnowledgeSelection(['a'],[])).toEqual([]);
 expect(setKnowledgeSelection(['a'],['a','b','a'])).toEqual(['a']);
});
```

- [ ] **Step 2：RED。** `pnpm --filter @weknora/mobile exec vitest run sources/weknora/resources/knowledge-selection.test.ts`。
- [ ] **Step 3：实现选择与共享检索命令。**

```ts
export function setKnowledgeSelection(allowed:string[],requested:string[]):string[] {
 const set=new Set(allowed); return [...new Set(requested)].filter(id=>set.has(id));
}
// packages/api-client/src/knowledge/search.ts，使用 ClientRequest 类型
export function buildKnowledgeSearch(query:string,ids:string[]):ClientRequest {
 return {method:'POST',path:'/api/v1/knowledge-search',body:{query,knowledge_base_ids:ids}};
}
```

KnowledgeScreen 用原生虚拟列表与分页；选中文档/标签按实际 mentioned_items 传给 H10 扩展输入，不直接读 Vue store。ReferenceScreen 经 SDK 获取资源，403/404有明确信息；不信任模型返回的任意外链和未经授权的存储地址。知识问答使用 H10 mode knowledge，退出选择不影响通用 Agent 新建会话。

- [ ] **Step 4：GREEN 与真实引用。** 重跑测试；搜索 facade 验证空 query/403、分页/取消；用同一用户在 Web/移动检查一条引用的标题、片段、版本与权限。禁止用模型编造的引用充当文件验收。
- [ ] **Step 5：提交。** stage 本任务文件及路由；`git commit -m "feat(mobile): integrate knowledge resources without gating generic agents"`。

### H15：临时附件上传与原生文件生命周期

**依赖：** H14、H10。**Files:** Create `packages/contracts/src/mobile/attachment.ts`、`packages/api-client/src/sessions/attachments.ts`、`attachments.test.ts`；Create `apps/mobile/sources/weknora/resources/attachments.ts`、`attachments.test.ts`、`AttachmentStrip.tsx`。

**Interfaces:** `AttachmentRef {id:string;name:string;status:'uploading'|'processing'|'ready'|'failed'}` 为移动归一化状态；`canSendAttachments(refs:AttachmentRef[]):boolean`；SDK `buildAttachmentUpload(sessionId:string,body:unknown):ClientRequest`，body 由原生 multipart 端口生成；服务器响应 wire 状态按临时文档 handler 映射，不直接提交本地 URI。

- [ ] **Step 1：未就绪附件不能偷偷发送。**

```ts
import {expect,test} from 'vitest';
import {canSendAttachments} from './attachments';
test('processing and failed attachments block submission',()=>{
 expect(canSendAttachments([{id:'a',name:'x.csv',status:'processing'}])).toBe(false);
 expect(canSendAttachments([{id:'a',name:'x.csv',status:'ready'}])).toBe(true);
});
```

- [ ] **Step 2：RED。** `pnpm --filter @weknora/mobile exec vitest run sources/weknora/resources/attachments.test.ts`。
- [ ] **Step 3：实现附件状态门并连接 DocumentPicker。**

```ts
import type {AttachmentRef} from '@weknora/contracts';
export function canSendAttachments(items:AttachmentRef[]):boolean {
 return items.every(item=>item.status==='ready');
}
```

读取 `internal/handler/session/temporary_document.go`、`frontend/src/api/chat/temporary-attachments.ts` 的字段名和处理状态。SDK 使用 POST `/sessions/:id/attachments`、列表/详情/删除现有路由；原生 FormData 不手填 multipart boundary，上传 transport 不能再 JSON.stringify。图片若使用现有 base64 image API 单列大小上限，不把全部文档转 base64。取消选择清理本地临时拷贝，失败保留重新选择/重试入口；重试先核对是否已有同一临时文档，不重复入库。H10 发送体新增实际 attachment IDs/mentioned_items，不发送 `file://`。

- [ ] **Step 4：GREEN。** 增加413、上传后退出/切空间、取消、中文文件名、进程重启后原 URI 失效测试；真机相册图像和 CSV 各一条，后台恢复后状态可核对。大文件以实际后端限制为准，在读取字节前阻止超限。
- [ ] **Step 5：提交。** stage 本任务文件；`git commit -m "feat(mobile): manage native session attachments safely"`。

### H16：产物、文件、代码和 Diff 展示

**依赖：** H15、原 T13。**Files:** Create `packages/contracts/src/mobile/artifact.ts`、`packages/api-client/src/sessions/artifacts.ts`、`artifacts.test.ts`；Create `apps/mobile/sources/weknora/resources/artifacts.ts`、`ArtifactScreen.tsx`、`FileDiffScreen.tsx`；Read Happy `components/{FilesSidebar,FileViewPanel,AllFilesDiffView}.tsx`。

**Interfaces:** `ArtifactIdentity {sessionId:string;messageId:string;index:number}`；`artifactDownloadRequest(id):ClientRequest`；原生 `saveAuthorizedFile(request:HttpRequest):Promise<{uri:string;name:string}>` 在文件平台端口实现，向 SDK 外隐藏授权 header，分享只给本地 URI。

- [ ] **Step 1：版本与消息标识不能丢失。**

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import {artifactDownloadRequest} from './artifacts.ts';
test('download uses exact historical artifact identity',()=>{
 assert.equal(artifactDownloadRequest({sessionId:'s',messageId:'old',index:2}).path,
  '/api/v1/sessions/s/messages/old/artifacts/2/download');
 assert.throws(()=>artifactDownloadRequest({sessionId:'s',messageId:'m',index:-1}),/ARTIFACT/);
});
```

- [ ] **Step 2：RED。** `pnpm exec tsx --test packages/api-client/src/sessions/artifacts.test.ts`。
- [ ] **Step 3：实现精确下载与展示适配。**

```ts
import type {ClientRequest} from '../client.ts';
export function artifactDownloadRequest(i:ArtifactIdentity):ClientRequest {
 if(!Number.isInteger(i.index)||i.index<0)throw new Error('ARTIFACT_INDEX');
 return {method:'GET',path:`/api/v1/sessions/${encodeURIComponent(i.sessionId)}/messages/${encodeURIComponent(i.messageId)}/artifacts/${i.index}/download`};
}
```

列表用现有 artifact metadata；下载不缓存长期授权 URL。Happy 的机器文件树和产品产物使用不同 provider，文件 ID 不转换成任意绝对路径。保留代码横滚和 Diff 布局，差异必须比较用户选择的两个真实版本；后端无版本内容时显示能力缺口，不能与空串制造“diff”。HTML/Office 受控预览限制导航/JS桥/外域，不将 Bearer 放进 WebView URL；复杂预览失败仍可安全下载分享。

- [ ] **Step 4：GREEN。** 下载测试覆盖403/404/错误 MIME/Content-Disposition 路径穿越；原生历史产物、中文名、代码、Diff、保存/分享逐项截图。检查撤销权限后缓存内容与临时文件的清理策略。
- [ ] **Step 5：提交。** stage 本任务路径；`git commit -m "feat(mobile): preserve authorized artifact and file interactions"`。

### H17：专业 Agent 的受控表单和结果注册

**依赖：** H07/H16。**Files:** Create `packages/contracts/src/mobile/specialist.ts`；Create `apps/mobile/sources/weknora/resources/specialist-registry.ts`、`specialist-registry.test.ts`、`DataAnalysisResult.tsx`；Modify AgentPicker/产品结果 wrapper。

**Interfaces:** `SpecialistResult {kind:string;version:number;payload:unknown}`；`resolveRenderer(result):'data-analysis-v1'|'fallback'`；`AnalysisTable {columns:string[];rows:(string|number|null)[][]}`，parse 必须拒绝超出字段/大小规则的数据。注册是本地代码，不从服务端下载组件。

- [ ] **Step 1：未知类型与未知版本安全回退。**

```ts
import {expect,test} from 'vitest';
import {resolveRenderer} from './specialist-registry';
test('unknown schemas never execute remote presentation',()=>{
 expect(resolveRenderer({kind:'data-analysis',version:2,payload:{}})).toBe('fallback');
 expect(resolveRenderer({kind:'script',version:1,payload:'alert(1)'})).toBe('fallback');
});
```

- [ ] **Step 2：RED。** `pnpm --filter @weknora/mobile exec vitest run sources/weknora/resources/specialist-registry.test.ts`。
- [ ] **Step 3：实现注册分派并连接真实领域输出。**

```ts
import type {SpecialistResult} from '@weknora/contracts';
export function resolveRenderer(r:SpecialistResult):'data-analysis-v1'|'fallback' {
 if(r.kind!=='data-analysis'||r.version!==1||!r.payload||typeof r.payload!=='object')return 'fallback';
 const p=r.payload as {columns?:unknown;rows?:unknown};
 if(!Array.isArray(p.columns)||!p.columns.length||p.columns.length>50||!p.columns.every(c=>typeof c==='string'))return 'fallback';
 const width=p.columns.length;
 if(!Array.isArray(p.rows)||p.rows.length>200||!p.rows.every(row=>Array.isArray(row)&&row.length===width&&row.every(cell=>cell===null||(typeof cell==='string'&&cell.length<=10000)||(typeof cell==='number'&&Number.isFinite(cell)))))return 'fallback';
 return 'data-analysis-v1';
}
```

专业表单只收领域所需附件/参数，提交仍使用 H10，共享权限和审批；data-analysis 表格注册前校验 columns/rows 长度和单元格类型，原生分页，不信任 HTML。现有后端若仅返回 Markdown/文件，保持原样展示，不能由客户端猜成权威结构化结果；新增结构化输出契约在共享 DTO/工具结果 fixture 同时记录。展示数据分析结果与工具调用、产物关联，不另建聊天页或新 Runtime。

- [ ] **Step 4：GREEN 与专业场景。** 测试畸形表格、超大行数、未知 kind/version；实际 data-analysis Agent 上传小 CSV，输出解释与可下载文件。没有工具/model环境则明确阻塞真实验收，fixture 通过不替代。
- [ ] **Step 5：提交。** stage 本任务路径；`git commit -m "feat(mobile): add safe specialist presentation registry"`。
