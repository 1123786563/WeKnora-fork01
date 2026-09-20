# Task 5 Report: Expose the Tenant catalog workflow through authenticated HTTP

Status: DONE  
Branch: `marketplace/t28-tenant-catalog-release`

## Implementation

- Added `AgentMarketplaceHandler` with strict JSON decoding (unknown fields, trailing values, null bodies and oversized request bodies rejected), explicit snake_case response DTOs, and tenant/actor identities sourced only from authenticated context.
- Registered Marketplace submission, review queue, review-and-publish, and catalog routes. Submission resolves the source Agent from the tenant-scoped frozen AgentVersion before applying the existing `OwnedAgentOrAdmin` guard. Review queue and review use `Admin`; catalog reads use `Viewer`. API-key policies require full access for reads and `manage_agents` plus full access for mutations.
- Registered the pre-existing AgentVersion routes in the production router and wired both service handlers/repositories in the container. Release files use a dedicated `tenant-releases` directory under the market data root.
- Added a real SQLite migration-backed HTTP lifecycle test covering freeze → submit → queue → approve → catalog, digest-verified bundle file materialization, and a subsequent Owner rejection that leaves the published pointer unchanged. The authorization matrix covers Viewer denial, Contributor own-Agent submission versus colleague-Agent denial, Admin approval, Owner queue/rejection, spoofed actor/tenant request fields, and cross-tenant source-version not-found behavior.

## TDD Evidence

- **RED:** `go test ./internal/router -run TestTenantAgentMarketplace -count=1` failed to compile because `NewAgentMarketplaceHandler` and `RegisterAgentMarketplaceRoutes` did not exist.
- **GREEN:** The same command passes after implementation (`ok`, 2.271s in the final uncached run).
- **Package suites:** `go test ./internal/handler ./internal/router ./internal/container -count=1` passes. The container package emits the existing linker warning `ignoring duplicate libraries: '-lc++'`; tests pass.
- **Diff hygiene:** `git diff --check` passes.

## Files Changed

- `internal/handler/agent_marketplace.go`
- `internal/router/routes_agent_marketplace.go`
- `internal/router/routes_agent_marketplace_test.go`
- `internal/router/router.go`
- `internal/container/container.go`
- `.superpowers/sdd/2026-09-20-t28-tenant-catalog-release/task-5-report.md`

## Assumptions and Remaining Risk

- The current codebase has no source that can resolve Skill/Subagent references into stable redistributable version, digest and license records. The production resolver therefore permits dependency-free Releases and fails closed with an explanatory error when the frozen Agent references Skills or Subagents; it does not create caller-controlled locks. A real immutable dependency resolver remains needed before such Agents can be submitted.
- PostgreSQL behavior was not exercised in this Task; the required end-to-end HTTP proof uses the repository's real SQLite migration stream.
