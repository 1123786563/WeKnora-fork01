# Backend Modularization Pass B Plan Decomposition Framework

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the accepted Pass A module layout into strict module boundaries by decomposing 396 legacy files, removing 105 temporary import exceptions and all aliases, and ending with zero business code in horizontal host packages.

**Architecture:** Pass B is a plan set, not one implementation branch. B0 freezes shared ports and exact file ownership; B1–B4 run independent module/subdomain plans in parallel worktrees; IB1–IB4 serialize router/container/worker/migration integration; B5 removes compatibility debt only after every module boundary is green.

**Tech Stack:** Go 1.26, Gin, GORM, dig, asynq, YAML v3, Testify, SQLite/PostgreSQL compatibility tests, Git worktrees.

**Spec:** `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md`

## Verified Pass A Inputs

- Accepted mainline commit: `78f18915f` (`merge: backend modularization pass a`).
- Acceptance evidence: `docs/architecture/evidence/pass-a-acceptance.md`.
- Stable inventory: 633 routes, 23 Redis + 23 Lite task types, 58 lifecycle hooks, 537 migration files.
- Legacy inventory: 396 files across 16 manifests in `docs/architecture/moves/`.
- Temporary import exceptions: 105 exact paths, no wildcard exceptions.
- Existing Pass B briefs: Knowledge ×4, Conversation ×2, Agent Runtime ×4, Workbench, Craft, Insights.
- Known debt: 13 Agent Runtime lint findings, Commercial payment map-order test, Knowledge/System housekeeping ownership, module aliases listed in each manifest.

## Global Constraints

- The manifest is the source of file ownership; a child plan may not claim a file owned by another plan.
- Public contracts are frozen in B0. Later signature changes require an ADR/Spec amendment and a new serialized contract task.
- Module workers do not edit `internal/router/router.go`, `internal/container/container.go`, global Redis/Lite registration, migration numbering, `go.mod`, or `go.sum`.
- Integrator is the sole writer for shared composition files and merges one reviewed child branch at a time.
- Every moved legacy production file takes its associated `_test.go` files; host packages retain no forwarding business declaration.
- No dual write, duplicate route/worker/hook, broad architecture exception, or copy of another module's implementation.
- High-risk behavior uses old/new differential tests before legacy deletion.
- Each Integration Barrier must preserve 633 routes, 23+23 workers, 58 hooks, and 537 migration files unless a separately approved feature changes the contract.
- `cmd/desktop`, `docreader`, `client`, and standalone SDK/contract modules remain out of scope except mechanical import repair already accepted in Pass A.

## Review Focus

- A legacy file listed by two child plans must fail B0 ownership validation before either plan starts.
- A module must not solve a forbidden import by moving the dependency into `common`, adding a wildcard exception, or copying implementation code.
- Redis and Lite worker sets, route guards, and lifecycle start/stop ordering must remain equivalent after each integration.
- Tenant/RBAC, payment/usage, session stream, knowledge deletion/indexing, Agent Run recovery, and Craft Artifact behavior require differential evidence.
- B5 must prove zero remaining aliases/exceptions/legacy entries; “unused but harmless” compatibility packages do not count as complete.

---

## Plan Set and Dependency Graph

```text
B0 Contract/ownership freeze                                  [SERIAL]
  ↓
B1 Identity | AI Resource | Commercial | Execution           [PARALLEL]
  ↓ IB1                                                        [SERIAL]
B2 Knowledge program | Agent Catalog | Data Source | AppConn  [PARALLEL]
  ↓ IB2                                                        [SERIAL]
B3 Agent Runtime program | Conversation program | Channels | Insights
                                                               [PARALLEL WITH INTERNAL DAGs]
  ↓ IB3                                                        [SERIAL]
B4 Workbench | Craft | System+Policy                          [PARALLEL]
  ↓ IB4                                                        [SERIAL]
B5 aliases/exceptions/host-package cleanup + final review      [SERIAL]
```

## B0 — Contract and Ownership Freeze [SERIAL]

**Plan file:** `docs/plans/passb/00-contract-and-ownership-freeze.md`

**Files:**
- Create: `docs/architecture/passb/ownership-matrix.yaml`
- Create: `docs/architecture/passb/contracts.md`
- Create: `docs/architecture/passb/event-catalog.yaml`
- Create: `docs/architecture/passb/exception-ledger.yaml`
- Modify: existing `docs/architecture/passb/*.md` only to remove overlap and assign exact child-plan IDs
- Test: `tools/architectureguard` ownership/exception/contract checks

**Produces:** immutable Pass B file owner, public façade signatures, event versions, integration ownership, and exact exception-removal owner for every child plan.

- [ ] Import all 396 manifest entries and 105 exceptions; reject duplicate/missing child-plan owners.
- [ ] Resolve Agent Runtime engine/protocol overlap: engine owns native persistence/service state; protocol owns wire packages and `native_archive` transport; no “first merged wins” wording remains.
- [ ] Resolve shared `internal/handler/session`, repository, service, and container integration ownership by exact path.
- [ ] Freeze Identity actor/tenant access, AI Resource capability resolution, Commercial admission/usage, Execution workspace, Knowledge retrieval, Conversation turn/session, and Task/Artifact contracts.
- [ ] Record event names/schema versions for Agent Run, Conversation, Knowledge processing, Craft, Usage, Workbench, and Insights projections.
- [ ] Add guard tests that fail on unowned legacy files, overlapping owners, changed frozen signatures, or expired exceptions.
- [ ] Commit and obtain architecture review before starting B1.

---

## B1 — Foundational Boundaries [FOUR PARALLEL PLANS]

| ID | Child plan | Legacy files | Primary boundary |
|---|---|---:|---|
| B1-ID | `docs/plans/passb/10-identity.md` | 27 | Actor/Tenant/RBAC/Audit public use cases |
| B1-AI | `docs/plans/passb/11-airesource.md` | 33 | Model/MCP/Search/Vector/Storage capability resolver |
| B1-CM | `docs/plans/passb/12-commercial.md` | 8 | Admission/Budget/Usage/Payment façade |
| B1-EX | `docs/plans/passb/13-execution.md` | 21 | Sandbox/Target/Workspace/Terminal/Browser façade |

Each child plan must define exact files, ports, characterization tests, differential cases, route/provider changes, alias deletion, exception deletion, and focused commands. The four plans use disjoint files and may run concurrently after B0.

### IB1 [SERIAL]

**Plan file:** `docs/plans/passb/19-foundation-integration.md`

- Integrate B1-ID → B1-AI → B1-CM → B1-EX one at a time.
- Switch shared composition only from reviewed Integration Briefs.
- Run tenant/RBAC, capability, payment/usage, sandbox/target differential suites.
- Freeze the concrete façades consumed by B2/B3/B4; update exception ledger.
- Run architecture guard, build, full internal tests, changed-range lint, and count parity.

---

## B2 — Core Capability Boundaries [PARALLEL PROGRAMS]

### Knowledge mini-program

**Coordinator plan:** `docs/plans/passb/20-knowledge-program.md`

```text
K0 knowledge ports/ownership freeze                         [SERIAL]
K1 ingest (9) | K2 retrieval (29) | K3 wiki+faq (18)       [PARALLEL]
K4 process/state machine (28), after K1+K2 façades          [SERIAL]
K5 knowledge integration + 18 workers + aliases             [SERIAL]
```

Child plans:

- `21-knowledge-ingest.md` from `knowledge-ingest.md`;
- `22-knowledge-retrieval.md` from `knowledge-retrieval.md`;
- `23-knowledge-wikifaq.md` from `knowledge-wikifaq.md`;
- `24-knowledge-process.md` from `knowledge-process.md`.

K1–K3 may run concurrently only after K0 assigns shared Chunk/KnowledgeBase/Tag/semantic types. K4 owns the 18 worker handlers and runs after ingest/retrieval contracts stabilize.

### Agent Catalog mini-program

**Coordinator plan:** `docs/plans/passb/25-agentcatalog-program.md` (55 legacy files).

- `25a-agent-definition-version.md`: custom agent, versions, persona, expert, subagent, favorites;
- `25b-skill-catalog-install.md`: tenant skill catalog/install/runtime verification/reaper;
- `25c-marketplace.md`: public and tenant agent/skill/expert marketplace handlers/services.

25a/25b may run in parallel after shared immutable-version contracts freeze; 25c follows both because it consumes their release/install façades.

### Independent B2 plans

- `docs/plans/passb/26-datasource.md` — 4 legacy files, sync scheduler/worker façade, alias removal;
- `docs/plans/passb/27-appconnector.md` — 7 legacy handlers, install/OAuth/action/sync routes, alias removal.

Data Source and App Connector can run concurrently with Knowledge/Agent Catalog because B1 already froze their external capability dependencies.

### IB2 [SERIAL]

**Plan file:** `docs/plans/passb/29-core-capability-integration.md`

Integrate Knowledge subplans in K-order, Agent Catalog in 25a→25b→25c dependency order, then Data Source and App Connector. Verify 18 Knowledge workers in Redis/Lite, catalog version/install invariants, sync/action behavior, guard/build/full tests/count parity, and remove only IB2-owned exceptions.

---

## B3 — Runtime and Interaction Boundaries [PARALLEL PROGRAMS]

### Agent Runtime mini-program

**Coordinator plan:** `docs/plans/passb/30-agentruntime-program.md` (45 legacy files plus 13 lint findings).

```text
R0 exact engine/protocol ownership from B0                  [SERIAL CHECK]
R1 memory/modelcontext | R2 tools                            [PARALLEL]
R3 engine/run/approval, after B1/B2 façades                  [SERIAL]
R4 native/tRPC/OpenCode protocol, after R3 state ownership   [SERIAL]
R5 runtime integration/recovery race/alias cleanup           [SERIAL]
```

Child plans: `31-agentruntime-memory.md`, `32-agentruntime-tools.md`, `33-agentruntime-engine.md`, `34-agentruntime-protocol.md`, directly grounded in the four existing briefs.

### Conversation mini-program

**Coordinator plan:** `docs/plans/passb/35-conversation-program.md` (44 legacy files).

- `35a-conversation-queryhistory.md` — 4 files; first vertical slice and differential gate;
- `35b-conversation-session.md` — 40 files; Session/Message/Feedback/share/temporary document/stream.

35a and 35b may implement application code in parallel after the Conversation façade freezes, but handler/session and module/router integration is serialized by the coordinator. Query History integrates first.

### Independent B3 plans

- `docs/plans/passb/36-channels.md` — 7 legacy files, IM/Embed identity/session ports, ten alias packages;
- `docs/plans/passb/37-insights.md` — 6 legacy files, read-only analytics/evaluation boundaries, metric alias cleanup.

Channels and Insights may run in parallel with R1/R2 and Conversation application work; Channels integration waits for the Conversation public session API.

### IB3 [SERIAL]

**Plan file:** `docs/plans/passb/39-runtime-interaction-integration.md`

Integrate Runtime R1→R2→R3→R4, Conversation Query History→Session, then Channels and Insights. Run Agent recovery race tests, session/stream/share/attachment differentials, IM/Embed delivery tests, analytics dialect tests, worker/route/hook parity, guard/build/full tests/lint.

---

## B4 — User Work and Governance Boundaries [THREE PARALLEL PLANS]

| ID | Child plan | Legacy files | Required frozen inputs |
|---|---|---:|---|
| B4-WB | `docs/plans/passb/40-workbench.md` | 19 | Task/Timeline/Artifact + Runtime/Commercial/Execution ports |
| B4-CR | `docs/plans/passb/41-craft.md` | 30 | Task/Artifact + Runtime/Execution/Knowledge/Commercial ports |
| B4-SP | `docs/plans/passb/42-system-policy.md` | 6 | Identity/Commercial decisions, settings/lifecycle ownership |

Workbench and Craft run concurrently only because B0/IB3 freeze shared Task/Artifact contracts; their handler/session file ownership is exact and disjoint. System+Policy owns housekeeping coordination and the remaining storage-allowlist/system files.

### IB4 [SERIAL]

**Plan file:** `docs/plans/passb/49-user-work-governance-integration.md`

Integrate Workbench → Craft → System+Policy, run Task/Artifact/notification/Craft lifecycle/schedule/system-setting/policy differentials, guard/build/full tests/lint, and freeze the final B5 cleanup ledger.

---

## B5 — Compatibility and Horizontal Host Cleanup [SERIAL]

**Plan file:** `docs/plans/passb/50-final-cleanup-and-acceptance.md`

- Remove all remaining alias packages only after `rg` proves zero non-test importers.
- Remove all 105 architecture exceptions and require zero forbidden imports.
- Require all 396 legacy entries to be marked migrated with destination package and commit SHA.
- Delete empty business content from horizontal `internal/application/service`, `internal/application/repository`, `internal/handler`, and `internal/handler/session`; retain only explicitly approved Platform/bootstrap primitives.
- Verify router/container only consume module façades and contain no business construction logic.
- Run `go test -race` for Agent Runtime/Execution/Commercial critical packages, PostgreSQL-tagged suites where environments exist, full build/tests/lint, migration checksum/count, route/worker/hook count parity, and final independent architecture/security review.
- Produce `docs/architecture/evidence/pass-b-acceptance.md` against Spec §17.2.

---

## Child Plan Authoring Order

1. Write and approve B0 detailed plan.
2. From B0 frozen contracts, write B1-ID/B1-AI/B1-CM/B1-EX and IB1 plans; dispatch only after all five are reviewed for signature consistency.
3. After IB1, write B2 child plans and IB2 from the actual integrated façades.
4. After IB2, write B3 child plans and IB3; do not pre-invent Agent Runtime/Conversation signatures.
5. After IB3, write B4 plans and IB4 from frozen Task/Artifact contracts.
6. Generate B5 from the live exception/alias/legacy ledger after IB4; its zero-debt assertions must use actual remaining paths.

## Plan Self-Review Record

- **Spec coverage:** B0–B5 and IB1–IB4 exactly match Spec §11–§17.2; all 16 modules and 396 legacy files have a plan owner.
- **Scope decomposition:** Large domains are split by existing Pass B briefs and file ownership rather than horizontal technical layers.
- **Parallel safety:** Only file-disjoint work with frozen contracts is parallel; all shared composition, contracts, migrations, and cleanup remain serialized.
- **No invented interfaces:** This framework names contract categories; exact Go signatures are produced and reviewed in B0 before child implementation plans are written.
- **Review Focus coverage:** ownership overlap, boundary workarounds, registration parity, high-risk differentials, and zero-debt cleanup each have a dedicated gate.

