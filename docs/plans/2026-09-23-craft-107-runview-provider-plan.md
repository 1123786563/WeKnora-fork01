# T01 Per-Run Provider and Inventory Plan

> **For Codex:** Execute with SDD RED → GREEN → REFACTOR, exact uncommitted checkpoints and independent Spec/quality Review. No commit.

**Goal:** Supply the reviewed RunView coordinator with a concrete per-Run container provider and complete pinned OpenCode session inventory, while keeping production dispatch disabled until live image/engine isolation is proven.

**Sources:** Spec #107/#120, RunView R1 review and report, `runview-provider-design.md`, `opencode-inventory-research.md`, T01 isolation design, current Dockerfile/lock and protocol review.

**Global Constraints:** Never treat a directory header as OS isolation; never replace a generation container after uncertain creation; never call OpenCode create a second time after committed intent. No client/model-supplied paths, host source/home/secrets/socket mounts or published ports. `opencode.lock.json.container_digest` is null: production enablement must fail closed until a real image digest, compatible routes and private runtime topology are verified. Distinct Task file ownership below permits concurrent implementation; shared live Docker/database resources are used serially.

**Review Focus:** actual inspected engine facts versus requested flags, deterministic generation labels/name/root, complete inventory pagination and directory/project equality, legacy `/session` vs v2 `/api/session` compatibility, unknown outcome, restart against same container, no unreviewed assembly toggle.

## Task 1 — pinned OpenCode inventory client

**Depends on:** R1 coordinator reviewed PASS and pinned source research. **Role:** backend_implementer. **Owned files:** new `internal/modules/agentruntime/agent/opencode/inventory.go`, `inventory_test.go`; `client.go` and its test only if a shared request helper is necessary and no other worker owns it. **Produces:** typed complete `GET /api/session` pagination and exact GET-by-ID metadata, with request-scoped directory/project and same-origin redirect policy.

1. RED: missing/looping cursor, truncated page, foreign directory/project, duplicate/malformed ID, 404/legacy-route mismatch and redirect/cross-origin cases fail closed; valid multiple pages and exact GET succeed.
2. GREEN: implement narrow typed method(s) against the pinned route schema, preserving WithDirectory and never asserting authoritative on partial results. Do not infer CreateSession idempotency from optional ID.
3. Focused protocol tests, diff check, full-content checkpoint and independent review. If live image exists, prove legacy create appears in v2 list; otherwise record this as production gate.

## Task 2 — deterministic container provider

**Depends on:** R1 coordinator reviewed PASS; stable coordinator/provider interface. **Role:** backend_implementer. **Owned files:** new `internal/container/craft_runview_container_provider.go`, `_test.go` and optional new private engine adapter file only; no `container.go` or `craft_runtime.go`. **Consumes:** generation container spec, engine inspect/create/start interface and Task 1 typed inventory once reviewed. **Produces:** `CraftRunViewRuntimeProvider` implementation that returns verified container identity and exact scoped session metadata.

1. RED: foreign same-name container, wrong image/label/mount/user/network/HOME, stopped/missing uncertain container, incomplete inventory, and changed generation all fail closed; valid restart reuses same container. Include first-create, crash, concurrent cases.
2. GREEN: deterministic request/inspect validation under a server-owned opaque root; read-only inputs/knowledge and private writable output/HOME; no host secrets/socket, ports or shared state. Production engine access is injectable, but response flags are set only from actual inspected facts. CreateSession requires the coordinator's one-shot intent and exact runtime directory; unknown outcome remains unresolved.
3. Focused tests/race/diff check, exact checkpoint and independent review. A fake engine is contract evidence only. Do not wire production by default without digest/route/network/live proof.

## Task 3 — assembly and Run A→B proof

**Depends on:** Tasks 1–2 reviewed/integrated plus image digest, live route/restart fixture, private network/volume configuration and R2–R4 material/executor/collector tasks. **Role:** backend_implementer then backend_validator and independent reviewer. **Owned files:** assigned after current `container.go`/`craft_runtime.go` ownership release. **Produces:** one real per-Run view across executor, selected input/knowledge and Version output.

Test live A→B exclusion, read-only mounts, no cross-Run HOME/history, current ACL, crash after CreateSession/prompt and immutable Version preservation. If any environmental prerequisite is absent, retain default-off and report exact blocker; no Ticket verification or OCR pass claim.
