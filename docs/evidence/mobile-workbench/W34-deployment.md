# W34 Safe deployment, capability switches and observability

Date: 2026-09-17
Worktree: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`
Base: `711d459d` (worktree HEAD at task start was `ee748371`; other tasks'
commits had already landed on top of the dispatched BASE)

## Deliverables

- `internal/execution/deployment_policy.go` / `_test.go` —
  `DeploymentPolicy{Privileged, DockerSocket, SharedTenantHome, EnforcedEgress}`
  + `ValidateManagedPolicy` (sentinel `ErrManagedPolicyRejected`), plus the
  five W34 observability series (low-cardinality, label-free).
- `internal/config/config.go` + `internal/config/workbench_capability_test.go` —
  per-capability switches `workbench.read_enabled / platform_admission /
  paseo_admission / voice_admission / notifications_enabled / worker_drain`
  with `WEKNORA_WORKBENCH_*` env overrides.
- `internal/container/agent_runtime.go` +
  `internal/container/agent_runtime_test.go` — switch consumption with drain
  semantics (no NEW admissions anywhere; worker stays enabled so admitted
  runs finish; `Drain()` counts `stop_unconfirmed` when the stop budget
  expires unconfirmed).
- `deploy/mobile-workbench/compose.yaml` + `README.md` + `secrets/.gitignore`
  — hardened local verification topology with real egress enforcement.

## Pre-existing BASE breakage (fixed in-task, coordinator-authorized)

`go build ./internal/container/` at BASE failed with:

```text
internal/container/agent_runtime.go:223: invalid operation: r.Enabled && provider == nil (mismatched types *bool and untyped bool)
internal/container/agent_runtime.go:258/271/273/274: undefined: err
internal/container/container.go:57: workbenchservice ... imported and not used
```

Minimal fixes (no opportunistic refactoring):

- `agent_runtime.go:223` — `r.Enabled` → `r.RecoveryEnabled()` (the nil-safe
  accessor; `*bool` cannot be used as a bare bool).
- `agent_runtime.go` — `var err error` declared once before the two worker
  constructor branches.
- `container.go:57` — unused `workbenchservice` import removed. This file is
  concurrently owned by W11 (unstaged edits); only the single committed
  import line was staged for this task via a hand-built minimal patch
  (`git apply --cached`), so no W11 in-flight content entered this commit.

After the fixes: `go build ./internal/container/` → exit 0.

## RED (task behaviour, not environment failures)

```text
go test ./internal/execution -run TestManagedNode -count=1
```

Before implementation: FAIL (build) — `DeploymentPolicy`, `ValidateManagedPolicy`,
`ErrManagedPolicyRejected` undefined. Same pattern for the metrics tests
(`CountExecutionDispatchUnknown` etc. undefined), the config switch tests
(`AreWorkbenchReadsEnabled` etc. undefined, field `Workbench` unknown) and the
container consumption tests (`WorkbenchPlatformAdmissionEnabled` /
`WorkbenchPaseoAdmissionEnabled` undefined). Full transcripts:
`/tmp/w34-red.txt`, `/tmp/w34-red-metrics.txt`, `/tmp/w34-red-config.txt`,
`/tmp/w34-red-container.txt`.

## GREEN and regression

```text
go test ./internal/execution -count=1        # ok (6 deployment-policy/metrics tests + existing suite)
go test ./internal/config -count=1           # ok (4 new capability tests + existing suite)
go test ./internal/container -count=1        # W34 tests PASS; full suite result degraded later (see note)
go test ./internal/agent/recoverytest -count=1  # FAIL at fix time (see note)
go vet ./internal/execution ./internal/config ./internal/container  # exit 0
```

Note (review Minor-1): the container and recoverytest results above were
taken in the implementer's earlier run window and do NOT reflect the shared
worktree's final state: with the W33 migration landed, the full container
suite fails `TestWireCraftInteractionRegistrarRegistersPendingInteractions`
and recoverytest fails wholesale on the same root cause (`cannot start a
transaction within a transaction`, W33 sqlite NoTxWrap migration vs the
migrator's transaction wrap) — a pre-existing defect on BASE, exposed once
the BASE compile breakage was fixed, tracked in task-W34-report.md concern
2. The W34-scoped tests of all three packages pass.

## Container evidence (Docker 29.4.0, compose v5.1.2 — available in this env)

### File validation

```text
MW_SECRETS_DIR=/tmp/mw-secrets docker compose -f deploy/mobile-workbench/compose.yaml config
```

exit 0. Rendered config retains: `internal: true` on the backend network,
`read_only`, `cap_drop [ALL]`, `no-new-privileges`, users (65534 / 999),
`mem_limit`/`pids_limit`/`cpus` per service, zero docker-socket mounts, and
exactly one published port (API entry, loopback-bound 127.0.0.1:8080).

### Real stack (postgres, redis, egress-gateway, bridge up)

Egress enforcement — four-way probe from a container on the backend network:

| Probe | Command (abridged) | Result |
| --- | --- | --- |
| A. direct outbound, no proxy | `curl -m 6 https://openai.com` | FAIL — `Could not resolve host` (curl exit 6); no route off the internal network |
| B. via squid to allowlisted domain | `curl -x http://egress-gateway:3128 https://www.bigmodel.cn` | `http_code=200` — allowlisted egress works |
| C. via squid to non-allowlisted domain | `curl -x ... https://example.com` | `CONNECT tunnel failed, response 403` — fail closed |
| D. via squid to internal IP literal | `curl -x ... http://172.30.0.40:5432` | `http_code=403` — proxy cannot be used to reach the internal LAN |

Isolation inside the bridge container (`docker exec`):

| Check | Result |
| --- | --- |
| identity | `uid=65534(nobody) gid=65534(nogroup)` — non-root |
| rootfs | `touch /etc/probe` → `Read-only file system` (exit 1) |
| docker socket | `/var/run/docker.sock` → `No such file or directory` |
| secrets | `/run/secrets/bridge_signing_secret` readable (0444), append → `Read-only file system` |
| cross-volume | bridge mounts list contains ONLY `bridge-workspaces` (+ its own secret); `api-workdir` invisible |

Limits (docker inspect, bridge): `PidMax=128 Memory=1073741824
NanoCpus=1000000000 ReadonlyRootfs=true Privileged=false CapDrop=[ALL]`.
Postgres reached `healthy`; redis served with a password read from its
secret file. Stack torn down with `down -v` after evidence collection.

Two real-container defects were found and fixed during this verification
(both now encoded in the compose file / README):

1. postgres: overriding `user:` broke `initdb` (`could not change
   permissions ... Operation not permitted`) — the official image's
   entrypoint must start as root once and drops to uid 999 itself.
2. squid: `access_log stdio:/dev/stdout` is fatal for the unprivileged
   proxy user (`Cannot open '/dev/stdout' for writing`) — logs now go to
   the image's tailed log files.

## Acceptance checklist mapping (brief step 5)

- Cross-tenant volume / docker socket / un-gated egress / long-term secret
  reads fail — REAL-CONTAINER VERIFIED for socket, direct egress, secret
  scope (per-service secret mounts) and cross-volume visibility; per-tenant
  subdirectory enforcement inside one bridge node is a runtime (W19–W24)
  responsibility.
- Legitimate model/Action calls succeed — allowlisted egress verified
  end-to-end to a model provider domain (probe B); full Action round-trip
  belongs to the connector lanes.
- Two bridge instances lease race, rolling-upgrade drain, version
  incompatibility refused — CODE-LEVEL: fenced dispatch claim and
  authorization-version checks are the W20–W24 tested seams;
  `TestAgentRuntimeAssemblesUnderDrain` pins drain semantics. LIVE two-node
  race drill: blocked-env (bridge process entrypoint is the W19–W24
  placeholder).
- Backup keys, object storage access, credential rotation drills —
  blocked-env here; rotation procedure documented in the README; drills
  belong to W35.
- No env vars or DSNs in logs — compose passes no secret env vars at all
  (secrets are files); config parse-failure logging prints variable NAMES
  only.

## blocked-env summary

- mTLS interop (api↔bridge handshake) — certificates/procedure documented,
  not live-tested in this environment.
- Two-bridge live lease race and full api image build — bridge entrypoint
  is a placeholder; api build not exercised (long build, out of scope for
  the topology evidence).
- Backup/restore and credential-rotation drills — W35 scope.

No verification above was recorded as passing without its command and exit
status; every blocked item is labelled blocked-env rather than passed.
