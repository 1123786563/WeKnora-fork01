# R013 · API playground 分步抽屉（SSE）核实与实现（Vue→React 一致性）

- 日期：2026-09-14
- 工作区：`.worktrees/react-multiclient`（分支 `codex/react-multiclient`，未自行 commit，待协调者复核集成）
- 独占改动：`apps/web/src/integrations/` 下 API playground 新组件与测试（`packages/views/src/integrations/apiKeys.ts` 本切片无需改动，保持原样）
- 基准：`frontend/src/views/integrations/ApiIntegrationSettings.vue`、`frontend/src/views/integrations/apiPlaygroundSSE.ts`（+ 同名 .test.ts）

## 1. 核实结论：登记有效，Vue 基准存在

遗留登记（R013/N028 (3)「API playground 分步抽屉（SSE）」）**无误**，Vue 基准完整存在：

| 区块 | Vue 位置 | 内容 |
|---|---|---|
| 入口卡片 | ApiIntegrationSettings.vue L326-335 | `playground-entry`：标题/描述 + 「打开 Playground」outline 按钮 |
| 抽屉骨架 | L339-363 | SettingDrawer 640px：title/desc、confirm=运行（loading/disabled=!canRun）、footer-left 停止按钮 + 禁用原因、cancel 即停止并关闭 |
| 请求配置 | L365-399 | 测试智能体 select（`name · 内置` 后缀、ensurePlaygroundAgent 默认选 builtin-smart-reasoning）、外部用户 ID（tenant 模式禁用 + 分模式 hint）、测试问题 textarea（默认 'hello' / 'user_123'） |
| 请求预览 | L401-413 | buildPlaygroundHeaders(maskSecrets=true) 两步预览（POST /sessions + POST /agent-chat/<session_id>），密钥遮蔽 <API_KEY>/<JWT>，可复制 |
| 运行结果 | L415-458 | error alert、（signed_token）生成 token 步、步骤 1 Session（状态 tag + 响应 pre）、步骤 2 Agent Chat SSE（状态 tag + 原始流 pre）、提取出的回答、空态文案 |
| 运行状态机 | L1637-1735 | AbortController；signed_token 先铸 900s 测试 token；POST /sessions（success=false / data.id|ID 解析）；POST /agent-chat/:id（Accept: text/event-stream，body {query, agent_enabled, agent_id, channel:'api'}）；SSE 逐步回填 raw/answer；abort→stopped（仅 running 步）、异常→failed（仅 running 步）、成功→toast（耗时 ms） |
| SSE 消费器 | apiPlaygroundSSE.ts（9 条测试） | 帧缓冲/跨 chunk 重组/CRLF/多 data 行；终态还原：[DONE] 哨兵、response_type error/complete/answer+done；EOF 无终态 = failed(unexpected-eof)；终态即 cancel 不再等连接关闭 |

## 2. React 实现（本切片落点）

全部为 `apps/web/src/integrations/` 新文件，不触碰 page.tsx / IntegrationsRoutePage.tsx / styles.css：

| 文件 | 内容 |
|---|---|
| `apiPlaygroundSSE.ts` | Vue 语义 1:1 移植；**帧层复用 api-client 共享 SSE parser**（`createServerSentEventParser`，同 skill-install 模式）；终态 union `{status:'success'} | {status:'failed', reason:'terminal-error'|'unexpected-eof', error?}`；终态即 cancel；EOF 无终态=failed；**AbortError 向上传播**（支撑停止→stopped） |
| `apiPlaygroundModel.ts` | 纯逻辑：buildPlaygroundHeaders（遮蔽/direct/token 头）、playgroundRequestPreview、playgroundDisabledReason（running 短路→''，与 Vue L1039-1048 同序）、interpretSessionResponse（data.id|ID、success=false、error.message 映射）、compactText/formatJSON/formatResponseBody、ensurePlaygroundAgent、agentOptionLabel、buildChatRequestBody、externalUserHintKey、settlePlaygroundStatuses（Vue L1716-1726：仅 running 步翻转 stopped/failed，非 abort 的 running session 落 **failed** 不是 success）、PlaygroundStepStatus |
| `ApiPlaygroundDrawer.tsx` | 等价分步抽屉：请求配置（agent select 默认 builtin、tenant 禁用外部用户、分模式 hint、问题 textarea）→ 请求预览（遮蔽 + 复制）→ 运行结果（error alert、生成 token 步、Session/Chat 步状态 tag + pre、提取回答、空态）；footer：运行（disabled=running||禁用原因 + 原因文案）+ 运行中停止按钮；overlay 点击/Esc/关闭按钮 = 停止并关闭（Vue watch visible→stop）；卸载兜底 abort。运行状态机逐行对齐 Vue runPlayground（含 credentials:'omit'、URL 归一、耗时 ms） |
| `apiPlaygroundSSE.test.ts`（10） | Vue 9 条测试全量移植（同 fixture）+ 增量 progress 断言 |
| `apiPlaygroundModel.test.ts`（12） | 头构造/遮蔽、预览形状、禁用矩阵、会话解析、Vue helper、settle（含 L1723 锁定：非 abort 的 running session → failed） |
| `ApiPlaygroundDrawer.test.tsx`（9） | jsdom+createRoot 交互渲染：三节渲染与默认值、关闭、禁用门（tenant 锁定）、signed_token 预览遮蔽、direct_header 全流程（两次请求头/体深比对 + SSE 逐步回填 + 三步状态）、停止（会话 success 保持 / chat stopped / 测试已停止 / 运行钮恢复）、终态 error 事件、signed_token 铸 token + token 步 + 头携带、内置 agent 默认选中 |

## 3. 测试证据（基线保持）

```
cd apps/web && npx tsx --test src/integrations/*.test.*
  tests 45 / pass 45 / fail 0
  （基线 tenant 1 + imWizardRender 5 + embedWizardRender 6 = 12；
    本切片新增 model 12 + SSE 10 + drawer 9 = 31，含子测试计数口径）

npx tsx --test packages/views/src/integrations/*.test.*
  tests 57 / pass 57 / fail 0（基线保持，本切片未改 packages/views）

pnpm run typecheck:web → 通过（exit 0）
```

## 4. 已知偏差（有意为之，逐条记录）

1. **抽屉壳**：Vue 为 SettingDrawer 浮层；React 为页面内 overlay 面板（等宽 640px、遮罩、Esc/遮罩点击/×关闭），与 embed 预览面板同一约定，待全局 Drawer 原语课题统一。
2. **成功提示**：Vue MessagePlugin.success toast → React 结果区内联 `wk-status` 状态行（`测试完成（{ms}ms）`），避免引入 toast 依赖。
3. **missing session id 边缘**：Vue 先置 session 'success' 再 throw missingSessionId；React interpretSessionResponse 归一为 ok:false（状态 tag 显示 failed，错误文案同键），信息不丢失。
4. **token 头名**：Vue 始终用常量 X-External-User-Token / X-External-User-ID（忽略服务端返回 header_name）；React 同常量（模型含 DEFAULT_* 导出），mint 端口仅取 token。
5. **agentsError**：Vue 有 agents 加载失败文案位；React 以 `agentsError` prop 呈现，宿主页可选注入。

## 5. 接线需求（共享文件，需协调者执行或授权；本切片未触碰）

抽屉组件已自包含，但入口在 page.tsx 内部（本切片无该文件写权限）。最小接线（约 10 行）：

- `packages/views/src/integrations/page.tsx` ApiIntegrationPanel：
  - 新增 prop `onOpenApiPlayground?: () => void`；
  - 用 Vue 式入口卡片替换现有临时 playground 表单（Session ID/Path/Body 手填区，L1241-1258）：
    ```tsx
    {onOpenApiPlayground ? <div className="wk-api-keys-header"><div className="wk-api-keys-title">
      <label>{t('integrations.api.playgroundTitle')}</label><p>{t('integrations.api.playgroundDesc')}</p>
    </div><button className="wk-button" type="button" onClick={onOpenApiPlayground}>{t('integrations.api.playgroundOpen')}</button></div> : null}
    ```
  - IntegrationsPage 增加 openApiPlayground state 并下发（或直接内嵌挂载 drawer，见下）。
- 挂载（二选一）：
  - page.tsx 内挂载（推荐，apiKey/principal/agents 状态齐备）：
    ```tsx
    {apiPlaygroundOpen ? <ApiPlaygroundDrawer open onClose={...} apiKey={apiKey}
      mode={principal?.mode ?? 'tenant'} agents={agents} apiBaseUrl={apiBaseUrl}
      mintToken={actions.onCreatePrincipalTestToken} t={t} /> : null}
    ```
    （`onCreatePrincipalTestToken` 返回 `IntegrationPrincipalToken{token,headerName,...}`，结构兼容 mint 端口；agents prop 已存在，仅需 api tab 时补一次 `void loadAgents()`。）
  - 或 IntegrationsRoutePage 挂载并经新 prop 下发（改动面更大，不推荐）。

## 6. 并发协作记录（向协调者报备）

本切片执行期间，另一 agent 在同一工作区并发写出了同主题的 `apiPlaygroundSSE.ts`/`apiPlaygroundModel.ts`/`ApiPlaygroundDrawer.tsx`（未与登记矩阵对齐，且 SSE 为手写帧解析、未复用 api-client parser；settle 曾把非 abort 的 running session 映射为 success）。经协调者裁决「以本切片版本为准收敛」：保留对方 model 中与 Vue 对齐的主体并修复 settle 偏差、移植其 playgroundDisabledReason 的 running 短路写法；SSE 按 slice 职责重构到 `createServerSentEventParser` + Vue union；drawer 以本切片测试为契约重写；期间对方两次引入的 force-stopped 覆写与重复 formatJSON 导出已清除。最终 6 文件均为收敛后状态，33/33 + 45/45 + 57/57 全绿。
