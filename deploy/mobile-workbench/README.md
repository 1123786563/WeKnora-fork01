# Managed mobile-workbench deployment (W34)

This directory is the W34 managed deployment surface of the mobile AI SaaS
workbench: hardening, capability switches, egress enforcement and the
observability contract, plus the W35 backup-restore runbook, event retention
policy and fault-drill entrypoints, plus the W36 native release /
compatibility window policy. The companion Go pieces are
`internal/execution/deployment_policy.go` (the managed-node policy and the
five health metrics), `internal/execution/restore_policy.go` (the W35
restore admission predicate), the event retention pass in
`internal/application/repository/agent_run_events.go`, and the capability
switches in `internal/config` (`workbench.*`).

## Topology

```
                 127.0.0.1:8080 (reverse proxy terminates TLS)
                        |
                   [ frontend net ]  172.31.0.0/24
                        |
        +------------ api ------------+     fixed private address 172.31.0.10 /
        |        (non-root, ro-rootfs, |    172.30.0.10 — mTLS peer anchor
        |         cpu/mem/pid limits)  |
        |                             |
   [ backend net — internal: true ] 172.30.0.0/24     <== NO outbound route
   postgres(.40) redis(.41) bridge(.20) open-connector(.50, profile)
        |                                             |
   api-workdir vol                       bridge-workspaces vol (isolated)
        |
   [ egress net ]  ---- egress-gateway (squid allowlist) ----> internet
```

Key properties (all verified with real containers where the environment
allowed — see `docs/evidence/mobile-workbench/W34-deployment.md`):

| Property | How it is enforced |
| --- | --- |
| Non-root execution | `user: "65534:65534"` on api/bridge/open-connector; postgres via its image's gosu drop to uid 999 |
| Immutable rootfs | `read_only: true` + noexec tmpfs `/tmp`; writable state only on dedicated volumes |
| Capability floor | `cap_drop: [ALL]` + `no-new-privileges:true` |
| Resource limits | per-service `cpus` / `mem_limit` / `pids_limit` |
| Isolated work volumes | `api-workdir` and `bridge-workspaces` are distinct volumes; a bridge node never sees another node's or the API's work volume; per-tenant subdirectories are runtime-allocated, never a shared writable home |
| No docker socket | nothing mounts `/var/run/docker.sock` (verified absent inside the container) |
| Secrets | mounted read-only under `/run/secrets` via the compose `secrets:` component — never environment variables, so `docker inspect` cannot leak a DSN or token |
| Egress enforcement | backend network is `internal: true` (no route at all); the only uplink is the squid allowlist gateway. Direct outbound fails, allowlisted domains pass, everything else (including internal IP literals via the proxy) is denied |
| Fixed private addresses | static IPs in `ipam`; API and bridge peers dial exactly these addresses (mTLS peer identity anchor) |
| Nothing internal published | only the API entry port is published (loopback-bound); the daemon admin port, postgres, redis and the open-connector console have NO host ports |

This compose file is a LOCAL, CONTROLLED verification of these orchestration
facts. Production substitutes its own egress gateway / host network policy
of at least equal strength — a config bool is a claim, not isolation
evidence (`internal/execution.ValidateManagedPolicy` encodes the same
contract in code).

## Provision secrets

```bash
mkdir -p deploy/mobile-workbench/secrets && cd deploy/mobile-workbench/secrets
umask 077
head -c 32 /dev/urandom | base64 > db_password
head -c 32 /dev/urandom | base64 > redis_password
head -c 32 /dev/urandom | base64 > bridge_signing_secret
```

The directory is git-ignored (`secrets/.gitignore`); rotate by writing a new
value and recreating the affected service (`docker compose up -d --force-recreate <svc>`).

## Validate the file

```bash
MW_SECRETS_DIR=./secrets docker compose -f deploy/mobile-workbench/compose.yaml config >/dev/null
```

compose refuses to render while secret files are absent — fail closed.

## Capability switches (W34)

Each lane closes independently. Closing a lane rejects NEW work in that lane
only; already-admitted work finishes and read/cleanup paths stay available.
One switch never cuts query and cleanup at the same time.

| Config (`workbench:` section) | Env override | Unset default | Effect (wired entrypoints) |
| --- | --- | --- | --- |
| `read_enabled` | `WEKNORA_WORKBENCH_READ_ENABLED` | on (true) | workbench read endpoints (list/get/snapshot/events/lookup) answer 503 — **wired**: `workbenchReadGate` on the read route groups (`internal/router/routes_workbench.go`) |
| `platform_admission` | `WEKNORA_WORKBENCH_PLATFORM_ADMISSION` | on (true) | NEW platform-target workbench admissions rejected before budget/durable writes — **wired**: `workbench.NewWorkbenchCapabilityGate` installed on the `AdmissionCoordinator` (`internal/container/workbench.go`) |
| `paseo_admission` | `WEKNORA_WORKBENCH_PASEO_ADMISSION` | on (true) | NEW remote (Paseo) admissions — **wiring target W22–W24** (no production remote submit entrypoint exists yet; the gate factory and `container.WorkbenchPaseoAdmissionEnabled` are ready to install there) |
| `voice_admission` | `WEKNORA_WORKBENCH_VOICE_ADMISSION` | on (true) | NEW voice-lane executions — **wiring target W30/W31** (the voice API entrypoint does not exist yet) |
| `notifications_enabled` | `WEKNORA_WORKBENCH_NOTIFICATIONS_ENABLED` | on (true) | NEW notification deliveries — **wiring target W14/W15** (the outbox/delivery worker does not exist yet) |
| `worker_drain` | `WEKNORA_WORKBENCH_WORKER_DRAIN` | off (false) | NEW admissions refused in EVERY lane (drain; see below) — **wired**: the live tRPC admission entrypoint (`submitDurableAgentRun`), the workbench `AdmissionCoordinator`, and the container drain helpers |

Runtime consumption (as of the W34 fix round):

- `submitDurableAgentRun` (`internal/application/service/agent_run_graph.go`)
  refuses new tRPC admissions while draining — recovery admission and the
  drain check are separate gates, so drain never silently re-enables a
  disabled recovery lane or vice versa.
- `AdmissionCoordinator.Start` consults `workbench.NewWorkbenchCapabilityGate`
  BEFORE identity, budget reservation or any durable write: drain closes
  every target lane, `platform_admission` closes only the platform target.
- `workbenchReadGate` on the read route groups answers 503 while
  `read_enabled` is off; it never gates writes, admission or cleanup.
- `WorkbenchPlatformAdmissionEnabled(cfg)` / `WorkbenchPaseoAdmissionEnabled(cfg)`
  (`internal/container/agent_runtime.go`) remain the assembly-level
  predicates; `AgentRecoveryAdmissionEnabled(cfg)` is false while draining.

### Drain (rolling upgrade / incident stop)

`worker_drain: true` (or `WEKNORA_WORKBENCH_WORKER_DRAIN=true`) means: refuse
NEW admissions at every live entrypoint (tRPC graph admission and the
workbench admission coordinator), keep the durable worker running so
already-admitted runs execute to completion, and keep cleanup/reconciliation
available. Sequence for a rolling upgrade:

1. Set `WEKNORA_WORKBENCH_WORKER_DRAIN=true` on the OLD instance.
2. Wait for its worker to finish in-flight runs (watch
   `execution_stop_unconfirmed_total` — it stays 0 on a clean stop; a
   non-zero increment means the stop budget expired without confirmation).
3. Replace the instance (new version answers new admissions).
4. Clear the env on the new generation.

Version-incompatible peers are refused by the existing fenced
admission/dispatch layer (authorization version + payload hash), not by this
file; the two-bridge lease race is settled by the durable dispatch store's
fenced claim — both behaviours are covered by the W20–W24 tests.

## Backup restore, event retention and fault drills (W35)

### Restore admission policy

Restoring a node's durable state from a backup snapshot closes NEW command
dispatch by default. It reopens only when the code-level predicate
`internal/execution.MayDispatchAfterRestore(reconciled, unknown)` holds:
every binding carried in the restored state has been reconciled against the
authoritative external systems — still-live external processes observed,
provider usage records matched against command receipts, projections
rebuilt — AND no dispatch outcome stayed unknown. A stale backup that still
shows a command as `queued` is never read as proof the command never
executed: the external process may have started and billed usage after the
snapshot was taken, so replaying it is exactly the duplicate-side-effect
window this policy closes. Node credentials are re-verified (epoch bumped)
as part of the restore drill before dispatch reopens.

### Event retention (snapshot-then-trim)

`AgentRunStore.ApplyEventRetention` (`internal/application/repository/agent_run_events.go`)
runs one bounded time-based retention pass over `agent_run_events`:

- Window: 30 days by default (`DefaultEventRetention`), overridable per pass
  via `EventRetentionOptions.Retention`; a negative window is rejected, not
  treated as trim-everything.
- Eligibility: only events older than the cutoff of runs already in a
  terminal status (`succeeded`/`failed`/`canceled`). Live runs are never
  touched.
- W33 coordination: runs whose session carries a live (non-purged)
  `execution_cleanup` tombstone are skipped entirely — their rows belong to
  the evidence-fenced W33 purge path, and deleting usage/replay evidence
  under that fence could deadlock settlement.
- Protected families, never deleted by this global pass (each follows its
  own retention/evidence policy): `usage.*` (commercial records),
  `approval.*` (live approval evidence), `run_completed` (the finalize
  idempotency receipt), `retention.trimmed` (the snapshots below).
- Snapshot-then-trim: for every trimmed run the pass appends a
  `retention.trimmed` summary event (trimmed count, first/last seq,
  per-type histogram, cutoff) and deletes the stale prefix in the SAME
  transaction — a crash can never leave events deleted without their
  snapshot, and old replay cursors over the trimmed prefix surface the
  explicit reload error (`ErrCursorExpired`) rather than silent data loss.

The pass is a store-level API invoked explicitly (operator job / worker
sweep); no background scheduler is wired to it in W35. The finalize-time
per-run watermark trim (`TrimEventsBefore`, last 1000 events) from earlier
weeks is unchanged and orthogonal.

### Fault drills

```bash
# Structure/validation/JSON-contract proof without a test deployment:
node scripts/mobile-workbench/fault-scenarios.mjs --harness [--repeat 3]

# Real injection into an explicit, allowlisted, non-production deployment:
node scripts/mobile-workbench/fault-scenarios.mjs \
  --target https://wb-test-1.internal:8443 \
  --allowlist "https://wb-test-1.internal" \
  --expect-version "$(git rev-parse --short HEAD)" \
  --scenario restore_snapshot --repeat 3
```

Guards, in order, before any injection: an explicit target (`--target` or
`WB_TEST_DEPLOYMENT`; otherwise immediate non-zero exit), origin membership
in `--allowlist` / `WB_TEST_ORIGIN_ALLOWLIST`, and the deployment control
endpoint answering `test_deployment: true` plus a version that matches
`--expect-version`. Each scenario (`start_ack_lost`, `bridge_restart`,
`daemon_restart`, `db_unavailable`, `restore_snapshot`) emits one JSON line
per drill — `scenario`, `baseline_sha`, `run_id`, `actual_process_count`,
`usage_count`, `result` — whose checks compare database run counts, external
process counts and commercial usage records, never just HTTP 200. After a
`restore_snapshot` drill the script also verifies dispatch stayed closed
until reconciliation finished and node credentials were re-verified. No
fixed RTO/RPO is promised; control-plane recovery time and data loss are
read off the drill output per run.


## Native release, compatibility window and upgrade policy (W36)

### Protocol compatibility window (actual behaviour)

The mobile app and the server negotiate through a protocol compatibility
window, implemented in `packages/domain/src/mobile/compatibility.ts` and
pinned by `packages/domain/src/mobile/compatibility.test.ts`:

- The server advertises `[protocol_minimum, protocol_maximum]` (this server
  release: `[2, 3]`; the current app build speaks protocol generation 3, so
  the window covers the PREVIOUS generation — generation-2 apps stay `full`).
- `protocolMode(client, minimum, maximum)` tri-state:
  - `full` — every surface usable, control commands (cancel/steer) included;
  - `upgrade_required` (app older than the window) — the app keeps the safe
    surface only: login plus the upgrade explanation; it sends NO control
    commands, and a server-side re-check refuses them anyway;
  - `server_upgrade_required` (app newer than the window, e.g. after a server
    rollback) — same safe-surface degradation.
- A malformed/unknown capability payload is treated as `unknown_schema`:
  control commands are refused rather than guessed; login stays.
- Malformed windows (`minimum > maximum`, non-integer or `< 1` generations)
  are rejected with `INVALID_PROTOCOL_RANGE`.
- Rollback interplay with the W34 switches: closing `platform_admission` or
  turning on `worker_drain` refuses NEW admissions while EXISTING runs stay
  queryable (read gate untouched) and cleanable — no admission switch ever
  cuts query and cleanup at the same time.

### Upgrade / rollback procedure

Rolling upgrade: follow the W34 drain sequence above (drain → wait for
in-flight runs → replace → clear). Rollback: close
`WEKNORA_WORKBENCH_PLATFORM_ADMISSION` (or set `WEKNORA_WORKBENCH_WORKER_DRAIN=true`)
on the rolled-back generation; apps newer than the rolled-back server window
degrade to the safe surface automatically (`server_upgrade_required`), and
old runs remain queryable and cleanable throughout.

### Release manifest (pin per release)

Fixed for this worktree state (verify before any store submission):

| Item | Value |
| --- | --- |
| App config actually resolved by Expo | `apps/mobile/app.config.ts` (version `0.0.0`, slug `weknora`; the sibling `app.config.js` dev variant is NOT what Expo resolves) |
| Bundle identifiers | iOS `com.weknora.mobile` / Android `com.weknora.mobile` |
| Expo SDK | 55 (resolved `expo@55.0.31`, spec `~55.0.8`) |
| React Native | `0.83.1` |
| React | `19.3.0` |
| JS update bundles | `npx expo export --platform ios|android` (scripts `export:ios` / `export:android` in `apps/mobile/package.json`); Hermes `.hbc` + `metadata.json` |

### Expand/contract and the native release rule

- **Expand** freely: widening the window (raising `protocol_maximum`) keeps
  every existing app inside `full`.
- **Contract deliberately**: dropping an old `protocol_minimum` is a release
  decision taken only after the old app generation's field compatibility has
  been validated; destructive cleanup of old-app data happens only after
  users have upgraded and the compatibility-carrying data has been
  re-validated.
- **Native changes need a native release**: `expo export` bundles ship JS and
  assets only (expo-updates). Adding or upgrading any native dependency
  (anything that changes `ios/`/`android/` native projects, Podfile entries,
  or the Expo/RN/plugins versions above) CANNOT be delivered as a JS update —
  it requires a new native build, signing and store review. A JS-only update
  must never be recorded as covering a native runtime change.

### Native payments

The native purchase entrypoint is NOT opened: the app carries
entitlement-sync code only (`apps/mobile/sources/sync/purchases.ts` parses
entitlement state; no store purchase flow is exposed). Web payment flows are
product surfaces, NOT store-acceptance evidence for the mobile lane. Do not
enable a purchase entry until the native payment lane is completed and
accepted.

## Observability contract

Five low-cardinality series (NO labels; unbounded identities never enter
metrics — they belong to structured logs carrying correlation IDs only,
never tokens, prompt text or file contents):

| Metric | Meaning | Alarm |
| --- | --- | --- |
| `execution_dispatch_unknown_total` | dispatches whose outcome stayed unknown past the crash-window fence | any increase → § Dispatch unknown |
| `execution_stop_unconfirmed_total` | stops that ended without confirmation inside the drain budget | any increase → § Stop unconfirmed |
| `execution_observer_age_seconds` | age of the oldest unresolved remote observation | > 300s for 5m → § Observer stuck |
| `execution_settlement_backlog` | settlements waiting on terminal evidence | > 100 for 10m → § Settlement backlog |
| `execution_notification_backlog` | notifications pending delivery | > 1000 for 10m → § Notification backlog |

Recording seams: `internal/execution` counters/gauges
(`CountExecutionDispatchUnknown`, `CountExecutionStopUnconfirmed`,
`SetExecutionObserverAge`, `SetExecutionSettlementBacklog`,
`SetExecutionNotificationBacklog`); `AgentRuntime.Drain()` already records
`stop_unconfirmed` when its stop budget expires. The remaining recording
points land with the lanes that own the data (W35/W36 sweeps).

### Runbook (alert → action)

- **Dispatch unknown**: the outcome fence expired. Check the dispatch store
  for the affected run window; do NOT manually flip states — the reclaim
  path resolves unknowns; escalate only if `dispatch_unknown` keeps growing
  with no reclaim progress.
- **Stop unconfirmed**: a stop budget expired with the target still
  (possibly) running. Identify the instance from logs (correlation IDs),
  verify the target's own state, and re-issue the stop; never assume the
  run finished.
- **Observer stuck**: the remote observation loop made no progress. Check
  egress-gateway logs (the observer's calls traverse it) and the fixed
  peer address; restart the observer's instance if the age keeps rising.
- **Settlement backlog**: settlements are waiting on evidence. Purge stays
  blocked by design (`CleanupFacts.Settled`); verify the provider receipt
  lane and the settlement sweep's health instead of forcing purges.
- **Notification backlog**: delivery lane is unhealthy. Check the egress
  allowlist (new provider domains must be added to
  `egress_allowlist` and reloaded with `squid -k reconfigure`) and the
  notification worker.

## mTLS between API and bridge

Both peers live on the `internal: true` backend network with fixed
addresses (api 172.30.0.10, bridge 172.30.0.20). The application-layer
admission (HMAC service token + authorization version, see
`internal/execution/bridge.go`) already authenticates commands; transport
mTLS is layered on top by the deployment:

```bash
# CA + two peer certs (10y, SAN-less: peers are pinned by address)
openssl req -x509 -newkey rsa:3072 -nodes -keyout ca.key -out ca.crt -days 3650 -subj "/CN=mw-ca"
for p in api bridge; do
  openssl req -newkey rsa:3072 -nodes -keyout $p.key -out $p.csr -subj "/CN=$p.mobile-workbench.internal"
  openssl x509 -req -in $p.csr -CA ca.crt -CAkey ca.key -out $p.crt -days 820
done
```

Mount the peer cert/key and the CA as additional compose `secrets` and
terminate mTLS at the peers' fronting proxies (or the services themselves
where supported). Interop was NOT live-tested in this environment
(blocked-env; see the evidence doc).

## Image pinning

`ubuntu/squid:latest` is used for the local verification topology.
Production must pin a digest (`docker image inspect --format '{{.Id}}'`)
the same way `docker/compose.open-connector.yaml` pins its runtime.

## What is intentionally NOT here

- No daemon admin port, database port, redis port or open-connector console
  published to the host.
- No host paths mounted writable into api/bridge (workspace data lives on
  named volumes).
- No secrets in environment variables or in `environment:` blocks.
