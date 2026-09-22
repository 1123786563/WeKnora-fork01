# Backend Modularization Foundation + Pass A Implementation Plan

> **COMPLETED:** Accepted on main at `78f18915f` with evidence in `docs/architecture/evidence/pass-a-acceptance.md`. Pass B planning continues in `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md`.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reorganize the complete server backend into 16 navigable business modules without changing behavior, using parallel package moves and serialized integration barriers.

**Architecture:** Foundation records the current server, creates module/move manifests, adds composition contracts, and enforces ownership. Pass A moves only complete Go packages whose package-private dependencies remain intact; files trapped in horizontal packages are indexed under the owning module and exposed through a façade, then physically split in Pass B. Four module workers operate in isolated worktrees while one Integrator exclusively owns router/container/worker/migration integration.

**Tech Stack:** Go 1.26, Gin, GORM, dig, asynq, YAML v3, Testify, Git worktrees.

**Spec:** `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md`

## Global Constraints

- Scope is `cmd/server` and `internal`; never modify `cmd/desktop`, `docreader`, `client`, or standalone SDK/contract modules.
- Pass A changes paths, package/import references, and composition only; it does not change API, SQL, state machines, permissions, errors, retries, schema, or business rules.
- Move a complete Go package or do not move it. A subset of files from a directory may move only after a separate Pass B package-boundary task proves package-private dependencies are removed.
- Do not copy production implementations. Temporary aliases and façades contain no business logic and name their Pass B deletion task.
- Module workers never edit `internal/router/router.go`, `internal/container/container.go`, global worker/lifecycle registration, migration numbering, `go.mod`, `go.sum`, or cross-module contracts.
- The Integrator is the sole writer for shared composition files and integrates one module at a time.
- Every worker uses an isolated Git worktree and commits only files listed in its `OWNED_FILES` manifest.
- Existing failures are recorded at F0; new failures block integration. Skipped environment-dependent tests are `blocked-env`, never PASS.
- Pure move, compile/import repair, integration, and later boundary refactor are separate commits.
- No Integration Barrier passes without module tests, consumer tests, architecture guard, `go build ./...`, `go test ./internal/...`, and changed-range lint.

## Review Focus

- A manifest must reject partial-package moves that would sever package-private Go symbols.
- A module façade must not duplicate constructors, mutable registration, database writes, or business decisions.
- Router/worker integration must register every existing route/task exactly once in both Redis and Lite modes.
- Baseline comparison must distinguish a pre-existing failure from a new regression by test name and error signature.
- Pass A completion must leave no unowned server file while allowing explicitly indexed horizontal-package legacy files for Pass B.

---

## File Structure

| File | Responsibility |
|---|---|
| `docs/architecture/backend-modules.yaml` | Canonical 16-module ownership and target package map. |
| `docs/architecture/backend-baseline.md` | Base SHA, commands, results, existing failures, route/worker/package counts. |
| `docs/architecture/moves/<module>.yaml` | Exact `move_packages`, `legacy_files`, tests, owned files, integration points, and Pass B deletion obligations. |
| `docs/architecture/integration/<module>.md` | Providers, routes, workers, lifecycle hooks, configuration, and ordered integration commands. |
| `docs/architecture/evidence/<module>.md` | Before/after tests, rename evidence, review result, and integrated SHA. |
| `internal/modules/<module>/README.md` | Human navigation: responsibility, non-responsibility, current packages, legacy map, public entry, and owner. |
| `internal/modules/<module>/module.go` | Thin composition façade; delegates to existing constructors and contains no business logic. |
| `internal/modules/<module>/legacy/README.md` | Exact horizontal-package files still physically outside the module and the Pass B task that removes each entry. |
| `tools/modulemove/` | Strict manifest loader and verifier for full-package moves, ownership overlap, forbidden files, and test commands. |
| `tools/architectureguard/` | Server asset discovery, ownership coverage, forbidden imports, unique route/worker registration. |
| `internal/bootstrap/` | Duplicate-safe route/worker/lifecycle composition contracts. |

---

### Task F0 [SERIAL]: Capture the Server Baseline and Ownership Inventory

**Files:**
- Create: `docs/architecture/backend-baseline.md`
- Create: `docs/architecture/backend-modules.yaml`

**Interfaces:**
- Produces: recorded `base_sha`; module IDs `identity`, `knowledge`, `conversation`, `agentcatalog`, `agentruntime`, `workbench`, `craft`, `execution`, `datasource`, `appconnector`, `channels`, `airesource`, `commercial`, `insights`, `system`, `policy`; every server package/route/worker/hook/migration assigned to one owner.

- [ ] Write `base_sha: <actual 40-character git SHA>` at the top of the baseline, then record outputs of `go build ./...`, `go test ./internal/... -count=1 -timeout=25m`, and `golangci-lint run --new-from-rev=HEAD~1 ./...`.
- [ ] Enumerate production packages with `go list ./...` and retain only `cmd/server` plus `internal/...`; enumerate route registrations, Redis/Lite worker registrations, `container.Invoke` lifecycle hooks, and migration files.
- [ ] Add each discovered asset to `backend-modules.yaml` with one owner and source path; reject duplicate or missing owners during manual review.
- [ ] Run the same baseline commands a second time to identify flaky failures; record differing signatures as unstable blockers.
- [ ] Commit: `docs: inventory backend module ownership and baseline`.

### Task F1 [SERIAL]: Create Exact Move Manifests and Module Skeletons

**Files:**
- Create: `docs/architecture/moves/{identity,knowledge,conversation,agentcatalog,agentruntime,workbench,craft,execution,datasource,appconnector,channels,airesource,commercial,insights,system,policy}.yaml`
- Create: `internal/modules/<module>/{README.md,module.go,legacy/README.md}` for all 16 modules

**Interfaces:**
- Consumes: F0 ownership map.
- Produces: exact `move_packages` and `legacy_files` lists consumed by every A-task; no worker independently chooses scope.

- [ ] For each module, classify every owned source as `move_package` only when the complete source directory has one owner; otherwise list each file under `legacy_files` with current path, reason, navigation label, and Pass B task ID.
- [ ] Add `owned_files`, `test_commands`, `integration_points`, and `forbidden_shared_files` to every manifest. `forbidden_shared_files` always includes router/container/global worker/migrations/go.mod/go.sum.
- [ ] Create module READMEs from the approved ownership matrix; create zero-logic `module.go` skeletons and legacy indexes.
- [ ] Cross-check that the union of all move/legacy entries equals the F0 server inventory and that intersections are empty.
- [ ] Commit: `docs: define pass-a module move manifests`.

### Task F2 [SERIAL]: Add Move Verification and Composition Guards

**Files:**
- Create: `tools/modulemove/{main.go,manifest.go,verify.go}`
- Test: `tools/modulemove/{manifest_test.go,verify_test.go}`
- Create: `tools/architectureguard/{main.go,discovery.go,check.go}`
- Test: `tools/architectureguard/{discovery_test.go,check_test.go}`
- Create: `internal/bootstrap/{routes.go,workers.go,lifecycle.go}`
- Test: `internal/bootstrap/{routes_test.go,workers_test.go,lifecycle_test.go}`
- Modify: `Makefile`

**Interfaces:**
- Produces: `go run ./tools/modulemove verify --module <id>`; `make check-backend-architecture`; duplicate-safe `RouteRegistry`, `WorkerRegistry`, and `LifecycleRegistry`.

- [ ] Write failing tests for unknown YAML fields, duplicate ownership, missing source, partial-package move, forbidden shared file, wildcard exception, duplicate route/worker/hook, and Redis/Lite worker mismatch.
- [ ] Run `go test ./tools/modulemove ./tools/architectureguard ./internal/bootstrap -count=1` and verify RED.
- [ ] Implement strict YAML loading with `KnownFields(true)`, Go-package directory verification via `go list -json`, AST import discovery, and sorted diagnostics.
- [ ] Implement registries that return `already registered` errors before mutating their underlying router/mux/hook set.
- [ ] Add Make targets `verify-module-moves` and `check-backend-architecture`; run both against all 16 manifests.
- [ ] Commit: `test: enforce pass-a package moves and module ownership`.

---

## Parallel Group A1 — Dispatch A1–A4 Together After F2

Each task runs in its own worktree, consumes its exact manifest, and must not edit shared integration files.

### Task A1 [PARALLEL]: Move App Connector Packages

**Files:** Manifest `docs/architecture/moves/appconnector.yaml`; target `internal/modules/appconnector`; evidence/integration documents for `appconnector`.

- [ ] Run manifest verification and every manifest `test_commands` entry; record pre-move output.
- [ ] `git mv` every complete package listed under `move_packages`; move its tests in the same operation.
- [ ] Repair only package/import paths, run `gofmt`, rerun target and direct-consumer tests, then `go build ./...`.
- [ ] Complete Integration Brief without editing router/container; run rename/function-body diff review.
- [ ] Commit move and compile repair separately; request review and return SHAs.

### Task A2 [PARALLEL]: Move Commercial Packages

**Files:** Manifest `docs/architecture/moves/commercial.yaml`; target `internal/modules/commercial`; evidence/integration documents for `commercial`.

- [ ] Verify manifest and run commercial/payment/usage baseline commands recorded in it.
- [ ] Move complete commercial, payment, usage, and existing commercial subpackages with their tests; leave mixed root-package files in legacy index.
- [ ] Repair imports only; run focused tests, direct consumers, build, and rename/function-body diff checks.
- [ ] Document billing/payment providers, routes, callbacks, workers, and lifecycle integration.
- [ ] Commit move and compile repair separately; request review and return SHAs.

### Task A3 [PARALLEL]: Move Data Source Packages

**Files:** Manifest `docs/architecture/moves/datasource.yaml`; target `internal/modules/datasource`; evidence/integration documents for `datasource`.

- [ ] Verify manifest and run connector/scheduler/sync baseline commands.
- [ ] Move complete datasource and connector packages plus tests; retain mixed application/handler files in legacy index.
- [ ] Repair imports only; run all datasource connectors, direct consumers, build, and diff checks.
- [ ] Document registry, scheduler, worker, credential, purge, retry, and startup hooks.
- [ ] Commit move and compile repair separately; request review and return SHAs.

### Task A4 [PARALLEL]: Move Channels Packages

**Files:** Manifest `docs/architecture/moves/channels.yaml`; target `internal/modules/channels`; evidence/integration documents for `channels`.

- [ ] Verify manifest and run IM/Embed/Webhook baseline commands.
- [ ] Move complete IM adapter packages and tests; index mixed handlers/router files as legacy.
- [ ] Repair imports only; run every channel adapter test, direct consumers, build, and diff checks.
- [ ] Document public callbacks, authenticated routes, identity/session bindings, and outbound lifecycle.
- [ ] Commit move and compile repair separately; request review and return SHAs.

### Task IA1 [SERIAL INTEGRATION BARRIER]: Integrate A1–A4

**Files:** Shared router/container/worker/lifecycle files named by the four Integration Briefs; architecture inventory and batch evidence.

- [ ] Integrate reviewed module SHAs one at a time in order A1, A2, A3, A4; after each, apply only its Integration Brief and run its composition tests.
- [ ] Run all four module suites, their direct consumers, architecture guard, `go build ./...`, `go test ./internal/...`, and changed-range lint.
- [ ] Verify route/worker/hook counts equal F0 and Redis/Lite task sets are identical.
- [ ] Record integrated SHAs and commit: `refactor: integrate pass-a batch a1`.

---

## Parallel Group A2 — Dispatch A5–A8 Together After IA1

### Task A5 [PARALLEL]: Move Execution Packages

- [ ] Verify `execution.yaml` and run its sandbox/execution/browser baseline commands.
- [ ] Move complete `execution`, `sandbox`, and browser-skill packages with tests; index mixed terminal/env/resource handler files.
- [ ] Repair imports only; run target/direct-consumer tests, build, rename/function-body diff checks.
- [ ] Write target/registration/terminal/browser Integration Brief; commit move and compile repair separately; request review.

### Task A6 [PARALLEL]: Move AI Resource Packages

- [ ] Verify `airesource.yaml` and run model/MCP/search/vector/storage baseline commands.
- [ ] Move only complete exclusive-ownership packages with tests; index mixed service/handler/types files and leave secrets in their existing storage boundary.
- [ ] Repair imports only; run target/direct-consumer tests, build, rename/function-body diff checks.
- [ ] Write provider/credential Integration Brief; commit move and compile repair separately; request review.

### Task A7 [PARALLEL]: Move Agent Catalog Packages

- [ ] Verify `agentcatalog.yaml` and run catalog/market/install baseline commands.
- [ ] Move complete experts/persona/subagents/skills/catalog packages and tests; leave runtime-owned `internal/agent` packages to A11.
- [ ] Repair imports only; run target/direct-consumer tests, build, rename/function-body diff checks.
- [ ] Write version/market/skill Integration Brief; commit move and compile repair separately; request review.

### Task A8 [PARALLEL]: Move System and Policy Packages

- [ ] Verify `system.yaml` and `policy.yaml` in one worktree and run initialization/system/policy baseline commands.
- [ ] Move only exclusive packages; index config/middleware/router/container files without editing shared files.
- [ ] Repair imports only; run target/direct-consumer tests, build, rename/function-body diff checks.
- [ ] Write settings/housekeeping/limiter/gate Integration Brief; commit move and compile repair separately; request review.

### Task IA2 [SERIAL INTEGRATION BARRIER]: Integrate A5–A8 and Freeze Capability Entrypoints

- [ ] Integrate A5–A8 one at a time; sole-writer changes shared composition files.
- [ ] Freeze and document Execution, AI Resource, Agent Catalog, and Policy façade signatures used by A9–A14.
- [ ] Run batch module/consumer tests, guard, build, full internal tests, lint, and route/worker parity.
- [ ] Commit: `refactor: integrate pass-a batch a2`.

---

## Parallel Group A3 — Dispatch A9–A11 After IA2

### Task A9 [PARALLEL MOVE]: Move Knowledge Packages

- [ ] Verify `knowledge.yaml` and run knowledge/retriever/ingestion baseline commands.
- [ ] Move only complete exclusive retriever/infrastructure/connector packages; index mixed files as ingest, process, wiki-faq, or retrieval legacy.
- [ ] Repair imports only; run knowledge/direct-consumer tests, build, rename/function-body diff checks.
- [ ] Deliver Integration Brief plus four Pass B briefs; commit separately and request review.

### Task A10 [PARALLEL MOVE]: Move Conversation Packages

- [ ] Verify `conversation.yaml` and run session/chat/feedback/query-history baseline commands.
- [ ] Move exclusive complete chat-pipeline/session packages; index mixed Session/Message/Feedback/Query History files.
- [ ] Repair imports only; run target/router consumers, build, stream/route characterization, and diff checks.
- [ ] Deliver Integration Brief and Pass B briefs; commit separately and request review.

### Task A11 [PARALLEL MOVE]: Move Agent Runtime Packages

- [ ] Verify `agentruntime.yaml` and run agent/runtime/tool/recovery baseline commands.
- [ ] Move complete runtime/tools/native/trpc/modelcontext/memory packages not owned by Agent Catalog; do not change frozen interfaces.
- [ ] Repair imports only; run target/direct-consumer/race tests named by the manifest, build, and diff checks.
- [ ] Deliver Integration Brief and Pass B briefs; commit separately and request review.

### Task IA3 [SERIAL INTEGRATION BARRIER]: Integrate Knowledge → Conversation → Agent Runtime

- [ ] Integrate A9 and verify; then A10 and verify; then A11 and verify. Do not reorder.
- [ ] Run high-risk characterization/differential tests for knowledge deletion/indexing, session/message streaming, tenant access, agent run/tool/approval/recovery, and Redis/Lite workers.
- [ ] Run guard, build, full internal tests, lint, and commit: `refactor: integrate pass-a core modules`.

---

## Parallel Group A4 — Dispatch A12–A14 After IA3

### Task A12 [PARALLEL]: Move Workbench Packages

- [ ] Verify `workbench.yaml` and run workbench/mobile/notification baseline commands.
- [ ] Move exclusive packages; index mixed session handlers; preserve frozen Task/Timeline/Artifact contracts.
- [ ] Repair imports only; run target/direct-consumer tests, build, and diff checks.
- [ ] Deliver Integration Brief and Pass B briefs; commit separately and request review.

### Task A13 [PARALLEL]: Move Craft Packages

- [ ] Verify `craft.yaml` and run workspace/run/interaction/artifact/schedule baseline commands.
- [ ] Move complete craft packages and tests; index mixed application/session/container files; preserve shared contracts.
- [ ] Repair imports only; run target/direct-consumer tests, build, and diff checks.
- [ ] Deliver Integration Brief and Pass B briefs; commit separately and request review.

### Task A14 [PARALLEL]: Move Insights Packages

- [ ] Verify `insights.yaml` and run analytics/evaluation/export baseline commands.
- [ ] Move exclusive analytics/evaluation/metric packages; index mixed handlers/repositories and preserve read-only ownership.
- [ ] Repair imports only; run target/direct-consumer tests, build, and diff checks.
- [ ] Deliver Integration Brief and Pass B briefs; commit separately and request review.

### Task IA4 [SERIAL PASS-A BARRIER]: Integrate A12–A14 and Accept Pass A

- [ ] Integrate A12, A13, A14 one at a time from reviewed SHAs.
- [ ] Run all module tests, direct consumers, architecture guard, `go build ./...`, `go test ./internal/... -count=1 -timeout=25m`, and changed-range lint.
- [ ] Compare routes/workers/hooks/migrations with F0; verify no duplicate registration, new failure, unowned file, or forbidden excluded-scope diff.
- [ ] Verify every legacy entry has a Pass B task ID and CI rejects new production files in old horizontal business directories.
- [ ] Produce `docs/architecture/evidence/pass-a-acceptance.md`; commit: `refactor: complete backend modularization pass a`.

---

## Plan Self-Review Record

- **Spec coverage:** F0–F2 implement Foundation; A1–A14 cover all 16 owners; IA1–IA4 serialize every shared integration point; Pass B is intentionally excluded until Pass A acceptance.
- **Placeholder scan:** Module scope is not left to workers; exact paths come from reviewed F1 manifests, which are a required interface produced before dispatch.
- **Type consistency:** Every module exposes the same zero-logic `module.go` façade contract from F2; A9–A14 consume only interfaces frozen at IA2/IA3.
- **Review Focus:** Partial packages, façade behavior, unique registration, baseline drift, and unowned legacy files each have an explicit guard and Integration Barrier check.
- **Execution method:** Use subagent-driven development with four implementation worktrees, one read-only reviewer, one verifier, and the root agent as sole Integrator.
