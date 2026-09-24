# Craft #107 T01 RunView R5 production engine prerequisite plan

> **For Codex:** Execute through SDD RED → GREEN → REFACTOR, exact uncommitted checkpoints, independent Spec/quality review and backend validation. No production enablement or image publication.

**Goal:** Provide a real Docker `CraftRunViewContainerEngine` so the reviewed R1 provider can be assembled from server-owned pins and an actual private container/network. H3 central knowledge DI is gated on this production provider/resolver, as documented in `2026-09-24-craft-107-t05-r3-h3-task1-report.md`.

**Sources:** Approved Craft Spec #107/T01; R1 provider and H2 live mount reviews; T01 Linux image lock/protocol smoke; R4 current Workspace continuation plan; H3 DI gap report.

## Global constraints

- Integration Worktree, no commit/push, registry publication, production feature toggle or shared workDir fallback. Current lock has local Linux image ID but registry digest null; constructor must fail closed when no verified deployable digest/reference is configured.
- Real Docker inspect must report complete labels, mounts, network attachments, ports, privilege/capabilities/namespaces, resources, image ID/digest and runtime binary/config hashes. Never synthesize requested values as observed facts. Definitive 404 is `nil,nil`; all other uncertain failures propagate as errors.
- Container Create/Start outcome ambiguity stays unknown; do not retry create after an uncertain send or attach an existing container without the provider's durable marker and exact reinspection. Private network must be internal and generation-specific.

## Review focus

Actual Docker API projections versus requested config, complete isolation inspection, pinned image/runtime proof, ambiguity and cleanup, no implicit production enablement.

## Task 1 — Docker engine adapter

**Depends on:** R1 provider and H2 actual mount independent PASS. **Role:** backend_implementer; validator backend_validator. **Owned files:** new `internal/container/craft_runview_docker_engine.go/_test.go`, plus narrowly required observed-ImageID field/check in `internal/container/craft_runview_container_provider.go` and focused provider test; the existing H2 Docker test adapter in `internal/container/craft_runview_live_engine_h2_test.go` may project its actually inspected ImageID to match the new contract. These seams were added when real adapter tests showed the projection lacked actual ImageID; no competing owner has them. Not `container.go`, `craft_runtime.go`, R4 repository or T19 sandbox files. **Consumes:** `CraftRunViewContainerEngine` interface and pinned Docker SDK. **Produces:** real `EnsurePrivateNetwork`, `InspectContainer`, `CreateContainer`, `StartContainer`, and `ProbeRuntime` with exact observed projections.

1. RED fake-Docker tests: missing/wrong image digest, extra network, published port, writable knowledge/input, extra mount, host namespace/capability, wrong user/workdir/resource, changed labels or runtime hash must be observed and rejected by R1 provider; adapter cannot omit fields or copy request into response. Engine 404 versus permission/timeout distinguishes absence from unknown. Create response lost remains ambiguous; no second create is issued by adapter. Runtime probe computes actual in-container OpenCode 1.18.4 binary SHA and baked config SHA, not static doc version.
2. GREEN use Docker Engine SDK with server-owned exact create spec, internal private bridge, no external attachment, UID 10001, read-only root, exact tmpfs/binds/resources/caps/security options. Inspect complete actual state and all network endpoints; probe a running container through a bounded, read-only command and hash actual binary/config. Keep Docker client lifecycle and context deadlines explicit.
3. Run focused normal/race tests, then a disposable real Docker test on the reviewed local Linux/arm64 image: create one generation network/container through the provider, inspect exact isolation and runtime, read knowledge mount/write denial, then remove all test resources. Record Docker/image/OpenCode hashes, command exits, cleanup and exact file checkpoint. If the available local image cannot satisfy pinned config, report that deployment gap without weakening validation.

**Acceptance:** reviewed adapter proves real engine identity/isolation and can be injected into provider; it does not alone satisfy production DI or R4 A→B. **Failure:** keep default-off.

## Task 2 — Server-owned provider and material resolver assembly

**Depends on:** Task1 reviewed PASS; R4 frozen draft seam reviewed PASS; H2 reviewed PASS. **Role:** backend_implementer. **Owned files:** `internal/container/container.go` and new focused assembly tests, serial with H3 owner. **Consumes:** `NewCraftRunViewStore(db)`, Task1 engine, server-owned pinned provider config, durable admitted Run and H2 material handle. **Produces:** one fail-closed production provider/resolver registration available to H3. Config missing registry digest or invalid root/pins returns unavailable; no fallback to old workDir. Integrate H3 only after this Task2 review.

## Task 3 — Live R5 handoff

**Depends on:** Task2 and R4 Task1/2 plus H3/T14/T15 gates. Validate actual A→B private generations, OpenCode session by-ID reinspection, frozen current draft, exact read-only inputs/knowledge and output-only writes. Preserve full command and network evidence; no release claim before OCR and complete Ticket acceptance.
