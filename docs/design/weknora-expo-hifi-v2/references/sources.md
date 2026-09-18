# 来源与阅读范围

统一仓库SHA：`12737238aa7b9d6891e76b2397f6941024f45668`。检索日期：2026-09-17。

本文来源为固定版本的代码与用户指定设计文档，以及Expo官方文档。GitHub内容由连接器只读获取。以下“阅读范围”坦诚记录节选/截断，不将它们描述成全仓逐行审阅。未获取完整仓库归档、未执行生产测试。

## S01

当前代码基线与 nativeFile 接缝

- [`packages/api-client/src/knowledge/documents.test.ts`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/packages/api-client/src/knowledge/documents.test.ts) — main提交元数据及变更片段。

## S02

后端依赖与多端目录

- [`go.mod`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/go.mod) — 1–120；apps 目录由 GitHub tree读取。

## S03

产品领域边界

- [`CONTEXT.md`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/CONTEXT.md) — 全文。

## S04

现有 Expo 工程依赖与脚本

- [`apps/mobile/package.json`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/apps/mobile/package.json) — 全文。

## S05

共享包与React工程入口

- [`package.json`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/package.json) — 全文。
- [`apps/web/package.json`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/apps/web/package.json) — 全文。

## S06

React Web 业务与宿主耦合

- [`apps/web/src/App.tsx`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/apps/web/src/App.tsx) — 1–125；另读取src目录元数据。

## S07

Web共享视图与纯逻辑出口

- [`packages/views/src/index.ts`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/packages/views/src/index.ts) — 全文。

## S08

工作台路由及SSE/所有权入口

- [`internal/router/routes_workbench.go`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/internal/router/routes_workbench.go) — 全文。
- [`internal/handler/session/workbench_read.go`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/internal/handler/session/workbench_read.go) — 1–370，分段读取。

## S09

执行SDK当前wire形状

- [`packages/api-client/src/mobile/executions.ts`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/packages/api-client/src/mobile/executions.ts) — 全文。

## S10

身份与scope的已有使用

- [`apps/mobile/sources/weknora/auth/session.tsx`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/apps/mobile/sources/weknora/auth/session.tsx) — 全文；不代表product-session依赖已完整审计。

## S11

产品凭证和生命周期

- [`apps/mobile/sources/weknora/auth/session.tsx`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/apps/mobile/sources/weknora/auth/session.tsx) — 全文。

## S12

产品会话VM与渲染接缝

- [`apps/mobile/sources/weknora/conversations/view-model.ts`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/apps/mobile/sources/weknora/conversations/view-model.ts) — 1–210。
- [`apps/mobile/sources/weknora/conversations/ConversationScreen.tsx`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/apps/mobile/sources/weknora/conversations/ConversationScreen.tsx) — 全文。

## S13

原生OIDC共享SDK

- [`packages/api-client/src/auth/oidc.ts`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/packages/api-client/src/auth/oidc.ts) — 全文。

## S14

Happy保留渲染器

- [`apps/mobile/sources/-session/SessionView.tsx`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/apps/mobile/sources/-session/SessionView.tsx) — 1–155。

## S15

原生事件流parser

- [`apps/mobile/sources/weknora/platform/stream-transport.ts`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/apps/mobile/sources/weknora/platform/stream-transport.ts) — 全文。

## S16

原生持久化与加密端口

- [`apps/mobile/sources/weknora/platform/execution-storage.ts`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/apps/mobile/sources/weknora/platform/execution-storage.ts) — 1–180。

## S17

原移动架构与总计划

- [`docs/superpowers/specs/2026-09-12-mobile-ai-saas-workbench-architecture.md`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/docs/superpowers/specs/2026-09-12-mobile-ai-saas-workbench-architecture.md) — 架构开头至第4节附近；大文件返回截断，未声称全文阅读。
- [`docs/superpowers/plans/2026-09-12-mobile-ai-saas-workbench.md`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/docs/superpowers/plans/2026-09-12-mobile-ai-saas-workbench.md) — 1–180关键章节；返回存在截断。

## S18

W01–W06核心契约分册

- [`docs/superpowers/plans/2026-09-12-mobile-workbench-01-core.md`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/docs/superpowers/plans/2026-09-12-mobile-workbench-01-core.md) — 1–105。

## S19

产品会话与恢复分册

- [`docs/superpowers/plans/2026-09-12-mobile-workbench-02-client.md`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/docs/superpowers/plans/2026-09-12-mobile-workbench-02-client.md) — 1–100，170–268。

## S20

设备通知与深链分册

- [`docs/superpowers/plans/2026-09-12-mobile-workbench-03-notifications.md`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/docs/superpowers/plans/2026-09-12-mobile-workbench-03-notifications.md) — 1–90。

## S21

Paseo分册

- [`docs/superpowers/plans/2026-09-12-mobile-workbench-04-paseo.md`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/docs/superpowers/plans/2026-09-12-mobile-workbench-04-paseo.md) — 1–95。

## S22

文件产物语音分册

- [`docs/superpowers/plans/2026-09-12-mobile-workbench-05-resources-voice.md`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/docs/superpowers/plans/2026-09-12-mobile-workbench-05-resources-voice.md) — 1–100。

## S23

交互保留与交付分册

- [`docs/superpowers/plans/2026-09-12-mobile-workbench-06-delivery.md`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/docs/superpowers/plans/2026-09-12-mobile-workbench-06-delivery.md) — 1–100。

## S24

W任务与验证状态参考

- [`docs/superpowers/plans/2026-09-12-mobile-workbench-code-agent-prompt.md`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/docs/superpowers/plans/2026-09-12-mobile-workbench-code-agent-prompt.md) — 1–105；返回尾部截断。
- [`docs/superpowers/plans/2026-09-12-mobile-workbench-execution-dag.md`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/docs/superpowers/plans/2026-09-12-mobile-workbench-execution-dag.md) — 完整节点/直接依赖/条件依赖与并行/热点部分；返回尾部截断。
- [`docs/superpowers/plans/2026-09-12-mobile-workbench-plan-review.md`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/docs/superpowers/plans/2026-09-12-mobile-workbench-plan-review.md) — 全文。
- [`docs/superpowers/plans/2026-09-12-mobile-workbench-task-index.json`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/docs/superpowers/plans/2026-09-12-mobile-workbench-task-index.json) — 1–160。
- [`docs/superpowers/plans/mobile-workbench-progress.md`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/docs/superpowers/plans/mobile-workbench-progress.md) — 全文。

## S25

OC集成设计执行边界

- [`docs/superpowers/plans/2026-09-12-open-connector-integration.md`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/docs/superpowers/plans/2026-09-12-open-connector-integration.md) — 1–135。
- [`docs/superpowers/plans/2026-09-12-open-connector-code-agent-prompt.md`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/docs/superpowers/plans/2026-09-12-open-connector-code-agent-prompt.md) — 1–85。

## S26

OC进度与不可发布子门禁

- [`docs/superpowers/plans/2026-09-12-open-connector-progress.md`](https://github.com/1123786563/WeKnora-fork01/blob/12737238aa7b9d6891e76b2397f6941024f45668/docs/superpowers/plans/2026-09-12-open-connector-progress.md) — 1–120请求返回至T18附近截断；补读28–65核对完整T18与证据开头。

## O01

[Expo SDK官方兼容矩阵](https://docs.expo.dev/versions/latest/) — 2026-09-17读取；55/RN0.83/React19.2.0；57/RN0.86/React19.2.3。

## O02

[Expo New Architecture](https://docs.expo.dev/guides/new-architecture/) — SDK55及以后仅New Architecture。

## O03

[Expo SDK56官方回归提示](https://expo.dev/changelog/sdk-56) — 2026-08-27增补说明；不以此代替本仓兼容验收。

## O04

[Expo OAuth/OpenID集成指南](https://docs.expo.dev/guides/authentication/) — Development Build与回调、客户端密钥边界。

## O05

[Expo SDK55 fetch/编码API](https://docs.expo.dev/versions/v55.0.0/sdk/expo/) — 显式expo/fetch读流与UTF-8原生编码。

## O06

[Expo SDK55 SQLite API](https://docs.expo.dev/versions/v55.0.0/sdk/sqlite/) — 事务返回Promise<void>；withTransactionAsync非独占；原生独占事务使用txn对象。

## O07

[Expo通知FAQ](https://docs.expo.dev/push-notifications/faq/) — 推送验收使用开发构建与正式应用凭据。

## O08

[Expo Update运行时版本](https://docs.expo.dev/eas-update/runtime-versions/) — runtimeVersion/fingerprint与分批更新。
