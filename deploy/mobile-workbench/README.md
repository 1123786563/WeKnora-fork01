# Managed mobile-workbench deployment (W34)

This directory is the W34 managed deployment surface of the mobile AI SaaS
workbench: hardening, capability switches, egress enforcement and the
observability contract. The companion Go pieces are
`internal/execution/deployment_policy.go` (the managed-node policy and the
five health metrics) and the capability switches in `internal/config`
(`workbench.*`).

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
