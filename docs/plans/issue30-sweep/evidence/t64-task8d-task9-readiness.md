# Task 8D and Task9 Readiness Audit

Read-only audit date: 2026-09-29. Audited source commit: `3d1d1f94901590e6dd31e84f00fbfbcf1e66159b` (Task8B integrated; before Task8C implementation). Coordination checkout later advanced to documentation-only HEAD `98e6ddc6adc696db95198b4cd0c3bb182e87f7e8`; no audit read from an in-progress 8C worktree. No files changed and no tests were run during either audit.

## Task 8D interface and ownership

**Readiness:** not dispatch-ready until Task8C is independently reviewed, backend-validated at exact HEAD, and integrated. The approved serial implementation schedule is 8C → 8D → 8E → Task9.

**Owned files:** `internal/handler/session/qa.go`, `helpers.go`, `stream.go`; new `internal/handler/session/agent_security_claim.go` and focused tests; generated `docs/swagger.yaml`, `docs/swagger.json`, `docs/docs.go` from AgentQA annotations.

**Forbidden files:** `internal/handler/session/handler.go` (8B injection ownership), `internal/container/container.go`, repositories, services, migrations, Run-store/runtime files. Plan references: `docs/plans/issue30-sweep/plans/plan-t64-task8-atomic-admission.md:42-47,210-219`.

8B already injects `agentChatTurnClaimStore` and `agentVersionService` into `session.Handler` (`handler.go:81-82,101-110`). The claim API is `Admit`, `Renew`, `GetForHandler`, `Finish`, `CreateUserMessage`, `UpdateMessage`, and `CancelByOwner` (`agent_chat_turn_claim.go:31-53`); state uses `ReplayNew`, `ReplayActive`, and `ReplayTerminal` (`:15-20`). The claim persists source/session tenant, Agent, nullable Version and Release, assistant ID, generation, lease owner/expiry and state (`types/agent_chat_turn_claim.go:7-26`). Adopted work must execute the stored Version via `AgentVersionService.GetAgentVersion(ctx, sourceTenantID, versionID)`; confirmed non-Marketplace claims with Version and Release both NULL use the authorized current CustomAgent. Agentless QA (`customAgent == nil`, `agent_enabled=false`) bypasses claims and keeps the existing normal QA path.

The 8D acceptance boundary requires parsing/resolution before side effects, but claim admission before upload processing, messages, live-run allocation, or SSE. Existing QA parsing saves files/images before claim admission (`qa.go:232-275`, `919-925`), and existing turn construction creates and may delete user/assistant messages (`qa.go:976-1044`); the change must reuse the durable assistant placeholder and preserve failed/cancelled history. Every message mutation must be fenced by claim/source tenant/generation/lease owner/active/unexpired state in the same DB transaction. Stop must use authenticated owner and exact assistant message ID to `CancelByOwner` before emitting the stop event; watcher polls durable claim state every 100ms and fails closed on state-read error or lost lease. Blocked/conflict/replay map to 409, invalid RequestID to 400, and claim/version infrastructure failures to 500 before SSE headers; Swagger changes apply to AgentQA only.

Validation commands include focused AgentQA handler plus existing stream-stop/upload tests, the session package serially, `go build ./...`, `make docs`, and `git diff --check`. Do not overlap Task9 session-package tests or commands sharing Go cache/fixtures. These details are from plan `:83-89,223-230,240-251` and current code locations above.

## Task9 boundaries and gates

**Readiness:** not dispatch-ready until 8C, 8D, and 8E each pass independent Spec/Quality review and backend validation at exact committed HEAD and are integrated. Run Task9 session-package tests only after 8D validation has finished. Task9 owns two new test files only:

1. `internal/router/routes_agent_security_test.go` — File A for governance HTTP wiring, Workbench execution entry, history/tenant isolation/governance floor.
2. `internal/handler/session/agent_security_e2e_test.go` — File B, `package session` because Handler dependency fields are unexported; AgentQA HTTP/claim E2E.

Plan source: `docs/plans/issue30-sweep/plans/plan-t64.md:1686-1706`.

File A consumes `openTenantAgentMarketplaceHTTPTestDB`, `adoptionCall`, `publishAdoptionRelease`, the real-stack `newAgentAdoptionTestApp` assembly, `session.NewWorkbenchStartHandler`, `workbenchservice.NewAdmissionCoordinatorWithBinding`, and `middleware.ErrorHandler`. It must resolve published Version/Release through `AgentSecurityService.ResolvePublishedAgentVersion` and pass exact IDs via `TrustedAdmissionBinding`, never pins from JSON or the nil/default resolver.

File B consumes `openCraftHTTPDB` and `runGateSessions`, real migrated SQLite, security store/service, Run store, Adoption/Marketplace repositories, CustomAgent/AgentVersion services, durable claim store, persisted messages, Gin route, RequestID middleware, SSE, and revocation HTTP route. Only the external AgentQA/model executor may be a deterministic barrier adapter. Each test uses its own `t.TempDir()` DB; no fixed shared port. Workbench scenarios use separate `s-wb`, `s-wb-post`, `s-wb-dep-digest`, and `s-live` sessions; chat uses separate `s-chat`.

Seven required E2E tests:

- `TestAgentSecurityE2EReleaseRevocationBlocksNewTaskRunAndDisposesInFlight`
- `TestAgentSecurityE2EDependencyRevocationNeverSubstitutesByName`
- `TestAgentSecurityE2ERevocationKeepsHistoryReasonScopeReplacement`
- `TestAgentSecurityE2EGovernanceFloorAndTenantIsolation`
- `TestAgentSecurityE2EAgentChatTurnRefusedForRevokedRelease`
- `TestAgentSecurityE2EAgentChatInFlightCancelAndAllow`
- `TestAgentSecurityE2EAgentChatTurnRefusedForRevokedDependency`

Acceptance evidence must exercise real HTTP routes and storage: new use is refused after Release/exact dependency revocation; same-name alternate versions/digests are not substituted; history, reason, scope, replacement metadata, claims/messages, and terminal assistant state are preserved; `cancel` interrupts and terminalizes; `allow` completes; blocked AgentQA is HTTP 409 with zero SSE and no claim/message/upload effects. Do not assert exact error-body wording. Plan sources: `plan-t64.md:1724-1727,1738,1949,1991,2023,2059,2068-2069`.

No Task9 implementation file or active Task9 worktree existed at audited HEAD. The only inherited environment limit is PostgreSQL migration runtime not exercised; Task9 itself uses isolated SQLite according to the plan. Earlier package-suite failures from legacy Run fixtures lacking security pins remain assigned to 8C/8E admission/test integration and must not be treated as final accepted evidence.
