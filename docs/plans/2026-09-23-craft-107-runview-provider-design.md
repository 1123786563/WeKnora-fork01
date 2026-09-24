# CRAFT-107 per-Run container provider: implementable seam and release gate

**Read-only design, 2026-09-23.** Integration HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`, moving worktree. Observed SHA-256: `internal/container/craft_runview_runtime.go` `c4d1196bb8e943f5ca5d9f7b8874d6df2b7df81caf1ff3d8f6e8ddbe7d524a34`, `craft_runtime.go` `a24213ce1a6a288dd58ab04f647dfdd66f52e490c4b17edbaab7d083856a8c46`, `docker/craft/runtime-config.json` `c211b994a7cdb88dac8b119e140b9eda29e9bd64e8ba59e63c5531484f0e7dde`, `opencode.lock.json` `3834e06533173c55dcbb1546229e0898a0540bb327b558ba505e7d34d268b37d`. Read approved Craft spec, `CONTEXT.md`, T01 per-run isolation design, RunView coordinator, OpenCode inventory research and pinned Dockerfile. Recheck before implementation; R1 coordinator review is still pending.

## Contract to implement

Implement `CraftRunViewRuntimeProvider` from `craft_runview_runtime.go` as one per-Run container adapter. `InspectOrCreateContainer(spec)` accepts only the coordinator's deterministic generation-derived `ContainerID`, `RuntimeID`, and in-container `Directory`; inspect an existing container by exact name plus immutable labels (generation hash/runtime ID/image digest), never adopt a same-name foreign container or replace a stopped/unknown one. On first creation, create once with deterministic name, label and mounts; on process restart, inspect and restart **that same container** with persisted HOME/data. Return `IdentityVerified`, `DedicatedForRun`, `DirectoryCanonical` only after actual engine inspection and mount/network/user/image checks. A missing container after an uncertain create is unresolved, not permission to create a new one unless the engine gives definitive absence before any create attempt.

`FindSessions(container)` talks only to this container's private OpenCode endpoint. Obtain full cursor pagination from pinned `GET /api/session` scoped to exact directory and project; independently validate every returned `Session.Info.location.directory`, `projectID` and ID, and use GET by ID if available. Set `Authoritative` and `Complete` only after all pages and container identity checks succeed. Zero after an uncertain CreateSession remains unresolved under the coordinator's one-shot intent. Multiple matches remain ambiguous. `CreateSession(container, directory)` sends one POST only after the coordinator's durable intent, then lets `FindSessions` prove the result; it never trusts a lost response or creates a second session. No model prompt is sent by this provider.

## Exact container layout and authority

Use a **built Linux image digest**, not the mutable `weknora-craft-runtime:1.18.4` tag. The Dockerfile already pins OpenCode 1.18.4 Linux binary hashes and runtime config; `opencode.lock.json.container_digest` is still `null`, so digest verification is a release gate. The adapter checks `opencode --version`, binary SHA for actual architecture, `/etc/craft/runtime-config.json` digest, image digest, UID/GID 10001, and configured command `opencode serve ... 4096` before marking the container usable. Do not treat `/doc` OpenAPI `info.version` as binary version.

One host-owned opaque root per generation, under a dedicated sandbox volume, maps only these paths into one container:

| Container path | Mount/access | Source rule |
| --- | --- | --- |
| `/workspace/<rv-hash>/inputs` | read-only, root-owned staged manifest files | T01 selected input refs/digests only; no prior Run files |
| `/workspace/<rv-hash>/knowledge` | read-only atomic T05 published package | exact accepted Run/digest, current ACL at retrieval; no prior Run package |
| `/workspace/<rv-hash>/output` | writable UID 10001 | verified baseline Version seed plus current draft only |
| `/home/craft/.local/share/opencode`, `/home/craft/.config/opencode` | private persistent volumes for this generation | no host home, no shared OpenCode DB/config/plugin state |
| `/tmp` | private bounded tmpfs | no shared temp data |

The coordinator currently chooses `Directory=path.Join(privateRootBase, "rv-"+hash)`; configure `privateRootBase=/workspace` so directory and mount path agree. Dockerfile defaults `WORKDIR /workspace/output`, so explicit directory selection on every API request is essential. Prefer a private container network with no published host port and only the platform's internal caller plus approved model gateway reachability; block DB/vector/storage/control-plane endpoints. No Docker socket, host source tree, platform secrets, tenant credentials or broad bind mounts enter the runtime. The platform container manager may need Docker/containerd control privileges, but those are outside the sandbox and must be held by a narrow server-side adapter, never the OpenCode process. Validate host symlinks and mount sources before creation and again on recovery; immutable mount layout is part of container identity.

## Protocol compatibility gate

Current Go `opencode.Client.CreateSession` uses legacy `POST /session`; checked-in lock describes that route. Source at pinned commit exposes v2 `POST /api/session`, `GET /api/session` and `GET /api/session/:id`, with cursor and `location.directory`/`projectID`. The inventory research does **not** prove that this particular built image exposes both route families or that a legacy-created session appears in v2 inventory. Before production wiring, run a live image fixture: create through the route the provider will use, enumerate through v2 across multiple pages, GET exact ID, verify directory/project equality, restart container and enumerate again. If `/api/session` is absent, do not mark `FindSessions` authoritative or enable R1; adapt to another verified complete inventory only with a new protocol test and review. The directory header is routing context, not OS isolation.

## Lifecycle, recovery and environmental blocker

Keep container and private HOME through unknown, waiting, stop reconciliation and retention windows. Cleanup only after durable terminal state, no unresolved CreateSession/prompt/charge-start intent, published Version retained elsewhere, and retention policy expiry. Stop/kill must be tied to persisted container ID and generation; never target by Task alone. A crashed adapter resumes by deterministic name + labels + mounts and inventories the **same** container. A container deletion or missing HOME after uncertain CreateSession cannot be healed by allocating a replacement for that generation; park the Run.

Implementable now: provider type, deterministic engine request/inspect validation, route client/decoder, fake engine and live fixture tests, fail-closed assembly toggle. True environment gate: verified Linux image digest, actual container engine access on target host, private network/model gateway topology, dedicated sandbox volume and a live route/restart test. The repository has a Dockerfile and lock but no per-Run container deployment assembly; those facts cannot be inferred from unit tests. Keep default-off until gate is met.

## Smallest nonoverlapping ownership

1. **Provider owner:** new `internal/container/craft_runview_container_provider.go` and `*_test.go`, optionally a private engine adapter file in the same package. Do not edit coordinator or `craft_runtime.go` while R1 review/runtime work is active. Define an injectable minimal engine interface for create/inspect/start/network/mount facts; production implementation may use a container engine API/CLI but must compare actual inspection fields, not trust request arguments.
2. **Protocol owner:** new `internal/modules/agentruntime/agent/opencode/inventory.go` and `inventory_test.go`; only modify `client.go` for a shared safe request helper if necessary. Use existing immutable `WithDirectory` and same-origin redirect policy. Confirm legacy/v2 route compatibility against built image.
3. **Assembly owner after prior checkpoints:** `internal/container/container.go`, `internal/container/craft_runtime.go`, deployment config and integration tests. Inject the provider/coordinator, replace shared serve default only behind explicit digest-checked config, then wire RunView handle to executor/material/collector. Avoid concurrent edits with T08/T19 and current runtime owner.

Acceptance is the real Run A→B exclusion and restart journey in `2026-09-23-craft-107-runview-runtime-adapter-plan.md`, plus inspect assertions for image digest, labels, mounts, user, HOME isolation, network and no host secrets. A successful provider unit test alone does not close R1.
