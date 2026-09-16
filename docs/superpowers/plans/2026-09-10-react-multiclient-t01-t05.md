# WeKnora React T01-T05 Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将已经存在的 T01–T05 React 基础切片从 `review` 推进到有证据的状态，并补齐契约审计、认证接入、作用域路由、平台注入和六语言数据层的可执行缺口。

**Architecture:** 保留 Go/Gin、Vue 发布源和 Wails 生命周期；React 仅通过 `packages/contracts`、`packages/api-client`、`packages/domain`、`packages/ui` 与 `apps/web` 接入。所有真实 API 行为以 `internal/router`、handler DTO/测试和当前 Swagger 2.0 对照为准，不能由页面或 mock 推断成功。

**Tech Stack:** TypeScript 6, React 19, Vite 7, pnpm 10, Vitest/tsx tests, Go/Gin, existing Vue i18n assets.

**Spec:** `docs/superpowers/specs/2026-09-10-react-multiclient-design.md`

## Global Constraints

- 保留现有 Go/Gin API、数据库、鉴权、RAG/Agent 和旧 Vue 构建输入；本计划不退役 Vue。
- `contracts`、`api-client`、`domain` 不依赖 React、DOM 或 Node runtime；共享 SDK 必须注入 transport 与 credential adapter。
- Bearer 与 Embed credential 不得互相 refresh；请求/缓存必须绑定 origin、user、tenant 和 generation。
- 对必填 ID、权限失败、mutation 响应、null/缺失/未知枚举严格解析；不得把失败转换成空成功。
- 所有实现必须有回归测试；mock、静态、真实后端和浏览器证据分别记录，不能互相冒充。
- Multica 只提供架构参考；不复制其业务源码、品牌资产或许可证不明的组件。

### Task 1: T01 contract and reuse audit completion

**Files:**
- Modify: `scripts/generate_react_migration_baseline.py`
- Modify: `docs/migrations/react/api-contract-matrix.csv`
- Modify: `docs/migrations/react/route-parity.csv`
- Modify: `docs/migrations/react/reuse-manifest.csv`
- Modify: `docs/migrations/react/version-matrix.md`
- Modify: `docs/migrations/react/runtime-baseline.md`
- Modify: `docs/superpowers/plans/react-migration-progress.md`
- Test: `scripts/test_generate_react_migration_baseline.py` (create if absent)

**Interfaces:** The script consumes current `frontend/src/router/index.ts`, `frontend/src/api`, `docs/swagger.json`, `internal/router`, and handler registration; it produces deterministic CSV/Markdown evidence with an explicit `source_status` (`swagger`, `registered-route`, `client-used`, `handler-reviewed`, or `special-route`).

- [ ] **Step 1: Write failing determinism and coverage tests** asserting current Swagger has 282 paths/361 operations, every operation has one matrix row, every registered `/api/v1` operation is classified, route parity has no empty task/role fields, and generated output is stable across two runs.
- [ ] **Step 2: Run the focused test** with `python3 -m unittest scripts/test_generate_react_migration_baseline.py -v`; expected initial failure for handler/permission and special-route assertions.
- [ ] **Step 3: Implement the smallest generator/audit changes** to extract registered Gin methods, classify `/files`, Embed, SSE, and terminal WS separately, and record DTO/permission review status rather than claiming Swagger is runtime truth.
- [ ] **Step 4: Regenerate and inspect diffs** with `python3 scripts/generate_react_migration_baseline.py`, `python3 -m unittest ...`, and `git diff --check`; verify no user-provided authority files are overwritten.
- [ ] **Step 5: Commit** only T01 evidence/script/test changes with message `docs: complete React migration contract audit`.

### Task 2: T02 shared client and real Web probe

**Files:**
- Modify: `packages/api-client/src/client.ts`
- Modify: `packages/api-client/src/ports.ts`
- Modify: `packages/contracts/src/index.ts`
- Modify: `apps/web/src/main.tsx`
- Modify: `apps/web/src/App.tsx`
- Create: `apps/web/src/platform/http.ts`
- Create: `apps/web/src/platform/http.test.ts`
- Test: `packages/api-client/src/client.test.ts`

**Interfaces:** Preserve `createWeKnoraClient(options: WeKnoraClientOptions)`, `HttpTransport.send`, and `scopedKey`; add an explicit browser transport adapter and a strict knowledge-base list response path. The page may show an error, but it must never treat missing auth or malformed mutation data as success.

- [ ] **Step 1: Add failing tests** for sub-path base URLs, `X-Tenant-ID`, `Accept-Language`, request cancellation, non-JSON errors, and malformed list responses.
- [ ] **Step 2: Run the focused package tests** with `pnpm test:shared -- --test-name-pattern='base|tenant|malformed|cancel'`; capture the failure.
- [ ] **Step 3: Implement the injected Web transport** without reading storage/env from shared packages; wire it to the existing React probe and expose request ID/error code in the diagnostic state.
- [ ] **Step 4: Verify mock/static evidence** with `pnpm test:shared`, `pnpm typecheck:shared`, `pnpm test:web`, `pnpm typecheck:web`, and `pnpm build:web`.
- [ ] **Step 5: If a running Go backend and test identity are available, run one real `GET /api/v1/knowledge-bases` probe** through the Vite app; otherwise record the exact missing credential/backend evidence in the ledger, not `accepted`.
- [ ] **Step 6: Commit** the client/transport slice with message `feat: wire React probe through scoped transport`.

### Task 3: T03 authentication endpoints and React login seam

**Files:**
- Modify: `packages/api-client/src/client.ts`
- Modify: `packages/api-client/src/index.ts`
- Create: `packages/api-client/src/auth/endpoints.ts`
- Create: `packages/api-client/src/auth/endpoints.test.ts`
- Create: `apps/web/src/platform/credentials.ts`
- Create: `apps/web/src/auth/LoginPage.tsx`
- Create: `apps/web/src/auth/LoginPage.test.tsx`
- Modify: `apps/web/src/main.tsx`

**Interfaces:** Add strict `auth.login`, `auth.me`, `auth.refresh`, and `auth.logout` methods using the existing `CredentialAdapter` and `createRefreshCoordinator`; `LoginPage` must render explicit loading/error/success states and cannot infer success from HTTP 200 alone.

- [ ] **Step 1: Write failing tests** for invalid credentials, `success:false`, missing access token, concurrent refresh single-flight, Embed isolation, and stale refresh after logout.
- [ ] **Step 2: Run the auth tests** with `pnpm exec tsx --test packages/api-client/src/auth/*.test.ts apps/web/src/auth/LoginPage.test.tsx`; record the initial failures.
- [ ] **Step 3: Implement endpoint DTO parsing and the browser credential adapter** with a one-time compatibility read of existing `weknora_*` storage keys; do not log token values.
- [ ] **Step 4: Add the React login route/seam** while preserving old URL and OIDC callback ordering; register no Next/tRPC/BFF layer.
- [ ] **Step 5: Verify** with shared tests/typecheck and a browser test using an explicit mock server. Record live OIDC/backend evidence separately if unavailable.
- [ ] **Step 6: Commit** with message `feat: connect React authentication seam`.

### Task 4: T04 scoped router and capability guards

**Files:**
- Modify: `packages/domain/src/scope.ts`
- Modify: `packages/domain/src/query-key.ts`
- Create: `apps/web/src/routes.tsx`
- Create: `apps/web/src/platform/scope-runtime.ts`
- Create: `apps/web/src/platform/scope-runtime.test.ts`
- Create: `apps/web/src/routes.test.tsx`
- Modify: `apps/web/src/main.tsx`

**Interfaces:** `scope-runtime` owns the current `ScopeController`, abort signal, query-key prefix, and capability snapshot; route guards consume it but do not perform authorization decisions. Legacy paths `/platform/*`, `/knowledgeBase`, `/join`, and `creatChat` remain reachable or redirect compatibly.

- [ ] **Step 1: Write failing tests** for A→B late response rejection, logout invalidation, deep-link refresh, old URL redirects, missing capability, and Embed route isolation.
- [ ] **Step 2: Run focused tests** with `pnpm exec tsx --test packages/domain/src/scope.test.ts packages/domain/src/query-key.test.ts apps/web/src/platform/scope-runtime.test.ts apps/web/src/routes.test.tsx`.
- [ ] **Step 3: Implement route/runtime wiring** so tenant changes abort old requests, clear scoped transient state, and cause a new generation; retain hard navigation where current behavior requires it.
- [ ] **Step 4: Verify** shared/web tests, typecheck, build, and route extraction; do not call a browser page “Wails” evidence.
- [ ] **Step 5: Commit** with message `feat: add React scoped routing guards`.

### Task 5: T05 UI platform ports and six-language data layer

**Files:**
- Create: `packages/design-tokens/package.json`
- Create: `packages/design-tokens/src/tokens.ts`
- Create: `packages/i18n/package.json`
- Create: `packages/i18n/src/index.ts`
- Create: `packages/i18n/test/keys.test.ts`
- Modify: `packages/ui/src/index.tsx`
- Modify: `packages/ui/src/styles.css`
- Create: `apps/web/src/platform/adapters.ts`
- Create: `apps/web/src/platform/adapters.test.ts`
- Modify: `pnpm-workspace.yaml`

**Interfaces:** `packages/i18n` exports locale data and pure `formatMessage(locale,key,values)` for `zh-CN`, `en-US`, `ja-JP`, `ko-KR`, `ru-RU`, and the existing sixth locale discovered by the audit; UI components remain business-agnostic; platform adapters expose navigation, storage, file, and clipboard ports.

- [ ] **Step 1: Write failing key/placeholder and keyboard interaction tests** for all six locales, missing keys, interpolation, dialog focus restore, disabled/loading/error states, and no DOM import in design-tokens/i18n.
- [ ] **Step 2: Run the tests** with `pnpm exec tsx --test packages/i18n/test/keys.test.ts apps/web/src/platform/adapters.test.ts`; capture failures before implementation.
- [ ] **Step 3: Extract locale JSON/data without importing Vue runtime** and implement semantic tokens plus Button/Card/Status/Dialog primitives using existing WeKnora visual rules, not Multica brand assets.
- [ ] **Step 4: Add Web platform adapters** and inject them into the React entry; keep mobile and desktop implementations replaceable.
- [ ] **Step 5: Verify** boundary tests, `pnpm test:shared`, `pnpm test:web`, typechecks, build, and `git diff --check`; record that real native evidence remains pending.
- [ ] **Step 6: Commit** with message `feat: add shared React UI platform and i18n data`.

## Execution review

Before Task 1, compare the task file sets above with the current T01–T05 commits and record overlapping-file rulings in the progress ledger. Existing implementation is evidence to verify, not permission to skip tests. A task may move from `review` to `accepted` only when its own missing evidence is collected; otherwise retain `review` with a concrete next step.
