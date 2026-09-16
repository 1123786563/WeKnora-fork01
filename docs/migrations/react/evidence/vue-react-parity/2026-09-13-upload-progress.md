# 上传进度共享层切片证据（transport XHR onProgress + 技能/文档上传接线）

日期：2026-09-13  
目标 worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`（分支 `codex/react-multiclient`）  
前置：复用 7a21611b（api-client Blob bridge + apps/web sendMultipartFile），不改动 `packages/api-client/src/client.ts`。

## 本轮范围

- 传输层新增字节级上传进度：`NativeMultipartFileRequest.onProgress`（ports.ts），JSON transport 新增 `sendMultipartFile`（浏览器 XHR 路径 `xhr.upload.onprogress`，非浏览器或未提供 onProgress 时回退 fetch）；fetch 回退保留原 headers/错误/revoked-objectURL 语义。
- web 平台层：`apps/web/src/platform/http.ts` 的 `sendMultipartFile` 改为委托 JSON transport（不再自建第二套请求实现），并新增 `observeUploadProgress(uri, cb)` 观察者注册表——client.ts 冻结不传 onProgress，UI 用自己 createObjectURL 的 blob: URI 作为键，把进度回调解桥到 transport。
- 技能 zip 上传（SkillSettingsPanel AddSkillWizard）：上传中渲染 `settings.sandbox.skillUploading`（共享 i18n，`Uploading {percent}%`，已验证 en-US/zh-CN 词条）+ 进度条；`registerSkillCatalogWithProgress` 造 NativeFileSource 并挂观察者。
- KB 文档上传（KnowledgeDocumentsPage + upload-pipeline）：pipeline 给 `upload` 回调第 4 参 `onProgress` 并记录 `UploadEntryState.progress`；页面 per-file 字节进度 → `knowledgeFileUploadProgress` 事件；拖放 mask（wk-dropzone）上方渲染 `UploadProgressMask`（`Uploading {n}%` + 填充进度条），批次百分比 = 各文件均值（Vue KnowledgeBaseList 聚合语义）。
- TDD：transport 5 用例（XHR 进度字节粒度、fetch 回退、无 XHR 回退、源读取失败、abort→AbortError）、http 3 用例（观察者→XHR 鉴权头/进度/一次性消费、无观察者走 fetch、stop() 释放）、panel 3 用例、pipeline 3 用例、dialog 2 用例；先红后绿。

## Vue 对齐证据（file:line）

| Vue 基线 | 语义 | React 对应 |
|---|---|---|
`frontend/src/api/system/index.ts:1117`、`frontend/src/api/skill/index.ts:64-66` | `Math.round((loaded*100)/total)` 百分比换算 | `uploadPercentFromProgress`（SkillSettingsPanel.tsx:619）、`uploadProgressPercent`（upload-pipeline.ts） |
`frontend/src/views/settings/SkillSettings.vue:208` + `:993-997` | 上传中 t-progress + onProgress 回调 | AddSkillWizard 上传中渲染 `SkillUploadProgress`（percent 文本 + bar） |
`frontend/src/components/SandboxSkillsPanel.vue:64-72` + `:1324-1326` | `settings.sandbox.skillUploading`（{percent}）文本 + 进度条；`uploadConfigSkill(...,(percent)=>…)` | `SkillUploadProgress` 用共享 i18n `settings.sandbox.skillUploading`；`registerSkillCatalogWithProgress` |
`frontend/src/components/upload-mask.vue:5-11`（platform/index.vue:7-9 ismask 全局遮罩） | 上传/拖放遮罩 | `UploadProgressMask` 覆盖 wk-dropzone（documents.css `.wk-upload-mask*`） |
`frontend/src/views/knowledge/KnowledgeBaseList.vue:67-68`（.progress-bar/.progress-bar-inner width%） | 进度条按百分比填充 | `.wk-upload-mask__bar/__fill` width% + `role=progressbar` aria |
`frontend/src/views/knowledge/KnowledgeBaseList.vue:1586-1595,1601` | 批次聚合=各任务进度均值并 clamp | `batchUploadProgress`（均值，done=100）+ `clampUploadPercent` |
`frontend/src/api/knowledge-base/index.ts:207-231` | uploadKnowledgeFile(form, onProgress) | 页面 upload 回调造 blob: 源 + observeUploadProgress → `client.knowledgeBases.documents.upload`（nativeFile 路径） |

## 已执行证据

| 层级 | 命令/结果 |
|---|---|
| 传输层新测试（红） | `pnpm --filter @weknora/api-client exec tsx --test src/transport/json.test.ts`：5 failed（sendMultipartFile is not a function）→ 实现后 5/5 passed |
| 平台层（红→绿） | `pnpm --filter @weknora/web exec tsx --test src/platform/http.test.ts`：新增 3 用例先红（缺导出）→ 14/14 passed（11 个既有用例无回归） |
| 设置面板（红→绿） | `SkillSettingsPanel.test.tsx`：新增 3 用例先红 → 31/31 passed |
| 文档上传（红→绿） | `upload-pipeline.test.ts` +3、`upload-confirm-dialog.test.tsx` +2 先红 → 分别 30/30、20/20 passed |
| documents 全量 | actions/list/preview/processing-timeline/upload-confirm-dialog/upload-pipeline 合计 **62/62 passed**（既有 57/57 基线保持全绿，+5 新用例） |
| api-client 回归 | `tsx --test src/client.test.ts src/configuration.test.ts src/transport/json.test.ts`：43/43 passed |
| 共享类型检查 | `pnpm run typecheck:shared`：passed（ports.ts/json.ts/index.ts 在编译清单内） |
| web 类型检查（我的文件） | 全项目 `tsc -p tsconfig.json --noEmit` 当前被他人 WIP 的 `src/settings/SettingsPage.tsx(354) TS1005` 语法错误阻断（非本切片文件）；以临时 scoped tsconfig（extends 项目配置 + vite-env.d.ts，仅含本切片 8 个文件）验证：**0 errors** |

## 传输层 API 形状

```ts
// packages/api-client/src/ports.ts
export interface UploadProgressEvent { loaded: number; total: number }
export interface NativeMultipartFileRequest {
  method: string; url: string; headers: Record<string, string>; file: NativeFileSource; fields: Record<string, string>;
  signal?: AbortSignal;
  onProgress?: (progress: UploadProgressEvent) => void; // XHR 路径开关
}
// createJsonTransport 新增：sendMultipartFile(request) —— 有 onProgress 且存在 XMLHttpRequest 时走 XHR
//（不手设 content-type，保留浏览器 multipart boundary；network error reject Error；abort→AbortError），否则 fetch 回退。

// apps/web/src/platform/http.ts（UI 可达桥，client.ts 保持冻结）
export function observeUploadProgress(uri: string, listener: (p: UploadProgressEvent) => void): () => void;
export function uploadProgressListener(uri: string): ((p: UploadProgressEvent) => void) | undefined;
```

## 变更文件（本切片全部改动）

- `packages/api-client/src/ports.ts`（onProgress/UploadProgressEvent 类型）
- `packages/api-client/src/index.ts`（导出 UploadProgressEvent 类型；仅此一行）
- `packages/api-client/src/transport/json.ts`（sendMultipartFile：XHR + fetch 回退）
- `packages/api-client/src/transport/json.test.ts`（新增）
- `apps/web/src/platform/http.ts`（观察者注册表 + 委托 JSON transport）
- `apps/web/src/platform/http.test.ts`（+3 用例）
- `apps/web/src/settings/SkillSettingsPanel.tsx`（SkillUploadProgress、registerSkillCatalogWithProgress、uploadPercentFromProgress + 接线）
- `apps/web/src/settings/SkillSettingsPanel.test.tsx`（+3 用例）
- `apps/web/src/documents/KnowledgeDocumentsPage.tsx`（UploadProgressMask + upload 回调进度 + mask 渲染 + 事件真实百分比）
- `apps/web/src/documents/upload-pipeline.ts`（progress 状态 + uploadProgressPercent/clampUploadPercent/batchUploadProgress）
- `apps/web/src/documents/upload-pipeline.test.ts`（+3 用例）
- `apps/web/src/documents/upload-confirm-dialog.test.tsx`（+2 用例）
- `apps/web/src/documents/documents.css`（.wk-upload-mask* 样式）

未触碰：`packages/api-client/src/client.ts`、`packages/api-client/src/sandbox/**`、`packages/i18n/**`、`packages/views/**`、`packages/domain/**`、`apps/web/src/settings/SettingsPage.tsx`、`apps/web/src/chat/**`、`apps/mobile/**`。

## 剩余差距 / 边界

- UI 无法直接经 client.ts 传 onProgress（该文件冻结）：进度经 blob: URI 键的观察者注册表桥接；若未来 client.ts 放开 onProgress 透传，注册表可退役（mobile 可直接用 transport.onProgress）。
- 单文件粒度的行内百分比（UploadFilesPanel 行）未展示，仅 mask 批次百分比 + 事件流真实百分比（Vue 基线同类面板在 KnowledgeBaseList，另一切片已覆盖列表侧）。
- 无取消按钮变化：沿用既有 uploadController/uploadPipelineController abort（XHR abort→AbortError→CANCELLED 已由 client.ts 映射）。
- 真实浏览器 E2E（真实后端、大文件、迟到响应）与截图差异报告未执行，与本仓其他切片一致。