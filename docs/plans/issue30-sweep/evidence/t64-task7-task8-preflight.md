# T64 Task7/Task8 preflight — 2026-09-29

Audited read-only at coordination HEAD `615c015c0ea5b0b78878111677aa1facceed8470` before any T64 Task7 production implementation. Repository: `1123786563/WeKnora-fork01`.

## Dependency status reconciliation

Evidence pointers and rulings:

- #52 local delivery implementation Tasks 1–11 are integrated/reviewed; real GitHub-provider E2E is credentials-gated. Keep GitHub Issue open and mark local evidence verified with `blocked-env` qualifier.
- #54 provider-neutral adapter and local HTTP-emulator evidence are integrated/reviewed; real GitLab OAuth/repository E2E is environment-gated. Keep GitHub Issue open and mark local evidence verified with `blocked-env` qualifier.
- #59 adoption/variant lifecycle, contracts, and mobile projection are integrated with `plan-t59.md-report.md` evidence. Its declared predecessor #33 remains open; the graph must retain that edge and the root Issue cannot be considered complete.
- #60 public Marketplace review/publish CAS, privacy boundaries, cross-tenant adoption and HTTP evidence are integrated; Issue remains open.
- #61 Upgrade Proposal lifecycle, reconciliation, routes and HTTP/SQLite evidence are integrated; PostgreSQL was unavailable; Issue remains open.
- #63 Tasks 1–6 have review, validation, and integration evidence in `B6-execution-ledger.md`; the exact T63 Task2 reviewed/integrated patch digest match and later reports supersede historical blocked notes. Issue remains open.
- #64 Tasks 1–6 have review, validation, and integration evidence; Tasks 7–9 remain absent and are required for its three acceptance criteria. #65 is blocked until T7–T9 pass and integrate.

These are local code/evidence statuses. They do not alter GitHub Issue open/closed state or erase declared dependency edges.

## Task7 boundary

Task7 owns:

- Create `internal/handler/agent_security.go` and `internal/handler/agent_security_test.go`.
- Create `internal/router/routes_agent_security.go`.
- Create `internal/container/agent_security.go` and `internal/container/agent_security_wiring_test.go`.
- Modify `internal/router/router.go` and `internal/container/container.go` only.

It wires four admin/full-access revocation endpoints, strict request decoding and error mapping, a concrete `AgentSecurityService`, its handler, and the existing Adoption/Upgrade release gates. It does not own `internal/container/workbench.go`, session `Handler` gate setters, the workbench admission seam, or highest-interface E2E tests. Those belong to Tasks 8 and 9.

Consumed production interfaces are already present at the audited HEAD: `interfaces.AgentSecurityService`, revocation inputs/views, `service.NewAgentSecurityService(*repository.AgentSecurityStore, *repository.AgentRunStore)`, and Adoption/Upgrade `SetReleaseSecurityGate`. The old plan constructor signature taking `*gorm.DB` was stale and corrected. The chat gate Invoke is deferred until Task8 creates a real setter; the T7 container wiring test covers only Task7 providers and router registration.

Task7 does not overlap T55 delivery files. T63's router/container/lifecycle changes are already integrated and must be preserved. T65 Task4 later shares `container.go`/possibly `router.go`, but #65 stays blocked until #64 Tasks7–9; if future implementation scheduling brings them together, serialize those files.

## Task8 architectural findings (blocking that task only)

The original Task8 pseudocode in `plan-t64.md` is unsafe and must not be implemented literally:

1. Workbench replay is a public idempotency behavior. `AdmissionCoordinator.Start` computes the request hash and replays an already `admitted`, `dispatching`, or `rejected` request before the new-use gate. Security checks must not run ahead of that lookup.
2. `AgentRunStore.Admit` currently locks `sessions` before its request replay and lifecycle checks. Revocation's `withTenantSecurityGuard` locks `tenants` first. Adding a tenant lock after the session lock risks an inverse-order deadlock; the decisive new Run admission must acquire the tenant guard first and then session/request/run rows.
3. A service-level `VerdictForAgent` call outside `AgentRunStore.Admit` leaves a TOCTOU race. The tenant-guarded transaction must reuse `checkReleaseAdmissionTx` for release and exact dependency identity; existing lifecycle retirement checks remain intact.
4. Do not hold the tenant transaction across budget/network calls. If revocation wins before durable Run admission, the Run is denied and any pending intent/reservation is reconciled and released; if admission wins, its disposition is handled as in-flight work.
5. `AgentQA` calls `parseQARequest` before the old proposed gate. That parser saves image bytes and processes uploaded attachments. Its quick-answer custom-agent path also bypasses the old `agentModeEnabled` check, and message writes/live-run events happen before execution. A check alone is neither side-effect-free nor atomic.
6. Existing messages have no unique turn claim/admission state; the live marker is Redis or memory and only set for smart reasoning. Neither is a durable serialization seam covering quick-answer and smart-reasoning turns.
7. Production `NewWorkbenchAdmissionCoordinator` already receives `adoptions` and installs the #63 `SetAgentUseGate`. Extend it without dropping that argument or retirement gate.

An architect proposed a durable private chat-turn claim, tenant-guarded before uploads, but its revocation cancel/allow behavior, source-tenant mapping, notification/recovery, cleanup, and multi-tenant lock ordering require a dedicated detailed plan and review before Task8 dispatch. No new migration or chat semantics are authorized by this preflight alone.

## Task7 verification scope

Planned commands from the corrected Task7 plan:

```bash
go test ./internal/handler/ -run 'TestRevoke' -count=1
go test ./internal/container/ -run TestAgentSecurityWiringRegistered -count=1
go build ./...
git diff --check
```

This is an implementation preflight; it does not claim these Task7 commands have run or passed.
