# Independent Task 4 review

**Scope:** `919df2289f9f89b6c59d9e15cba731f71fb867f5..db18dd85bd55aef4100442156a47832ca4c55664` in the `issue-140-fix-t4-module-seams` worktree. Static, read-only source review; no tests or OCR were run. Sources: approved backend module reorganization spec (`docs/specs/2026-09-21-backend-domain-module-reorganization-design.md`, especially §§3–4), mobile AI Office spec (developer delivery stories 43–48 and 53), ADR-0004, ADR-0008, `CONTEXT.md` (Task Owner, code delivery approval and recovery), and the assigned Task 4 brief.

## Finding

### T4-Q1 — Low — Production adapter behavior lacks direct coverage

- **Evidence / affected symbols:** `internal/container/code_delivery.go:153–236` implements the production action row, run, provider, connection, guard, dispatch, and lifecycle conversions. The new tests in `internal/modules/codedelivery/contracts_test.go:19–40` use `localSnapshot` and `fixtureRun`; `internal/modules/codedelivery/service_prepare_test.go:340–418` supplies separate fixture adapters that duplicate, rather than call, the production conversions. No `internal/container/*_test.go` references the `codeDelivery*` adapters.
- **Impact:** A future omission in production mapping of tenant/actor/connection/auth version, owner-scoped session, provider result, or the appconnector dispatch sentinels could pass the current codedelivery tests. Those values determine authorization, dispatch classification, and recovery state. This is a coverage gap, not an observed runtime failure in this commit.
- **Smallest correction:** Add focused tests in `internal/container` that exercise the production adapters with controlled sources and assert the mapped identity/payload/outcome plus `errors.Is` in both directions. Include owner/tenant rejection for `codeDeliveryRunReader` using its real store or a production-wiring test that reaches it.

## Spec compliance verdict

**Compliant for the changed scope; no blocking spec finding.** The codedelivery production package has no appconnector or agentruntime internal-package import after this commit. `ActionInput` and `ActionSnapshot` carry tenant, actor, connection, target, auth version and args (`internal/modules/codedelivery/contracts.go:13–18, 55–61`); the container maps them to the appconnector action and dispatcher (`internal/container/code_delivery.go:181–205`). `ConnectionIdentity` retains kind, owner, state, tenant, installation and auth version (`contracts.go:19–23`; container lines 228–230). The provider adapter returns the tenant-scoped installation's app ID (container lines 172–175). `RunReader` retains the owner-scoped lookup and session ID (container lines 163–165; source `internal/application/repository/agent_run.go:100–119`), preserving `taskId = sessionId`.

The delivery dispatch sentinels are distinct (`contracts.go:69–70`), and the container bridges them with `%w` in both directions (`code_delivery.go:181–220`), preserving `errors.Is` for appconnector settlement and delivery unknown handling. Agentruntime root sentinels are the same error objects used by the internal runtime (`internal/modules/agentruntime/module.go:23–27`; `agent/runtime/contracts.go:12–22`); queue restart now returns the root objects (`internal/modules/workbench/service/workbench/command_queue_next.go:57–99`).

The changed queue import is **the agentruntime root package**. The existing architectureguard exact-path exception at `tools/architectureguard/check.go:785–790` names `workbench/service/workbench/command_queue_next.go` importing **`agentruntime/agent/runtime`**. It is stale after this change and is assigned to Task 8; it is not evidence of an internal-package import in the changed queue file.

## Code quality verdict

**Acceptable with T4-Q1 follow-up.** The conversion code is straightforward and I found no concrete correctness, security, concurrency, data consistency, or state-machine regression in the diff. The production adapters remain untested directly, so confidence in the security-sensitive conversion boundary is limited to static inspection and fixture-level tests.
