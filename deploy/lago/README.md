# Lago Community v1.53.0 — pinned local integration environment

This directory runs a pinned, isolated Lago Community `v1.53.0` stack for
WeKnora billing integration work (Ticket #73, Lago T01). It is the local
stand-in for the commercial billing authority described in
`docs/specs/2026-09-20-lago-billing-migration-design.md` and is fully
independent of the OpenMeter deployment (own Compose project, own volumes,
own ports; no shared database, Redis, or data).

Everything here is operator tooling. WeKnora's product-facing billing seam
arrives with the Commercial Platform tickets; this environment only proves
the runtime contract.

## Requirements

- Docker Engine with the Compose v2 plugin (`docker compose ...`).
  **Docker Compose v2.20 or newer** is required because the healthchecks use
  `start_interval`. Verified in anger with Docker 29.4.0 / Compose v5.1.2.
- `python3` (stdlib only) and `openssl` on the operator PATH — used by
  `lago.sh` for secret generation, the health snapshot, and the probe.
- Disk/time budget: the five pinned images total roughly 1.7 GB; a cold
  `up` is dominated by the `getlago/api` pull (~1 GB). First start also
  runs Rails migrations and seeds; several minutes is normal.
- Apple Silicon note: `getlago/lago-gotenberg` (the PDF service) publishes
  only `linux/amd64`. It runs under Rosetta/QEMU emulation, so its start is
  slower than the others. It is a non-core service (PDF rendering) and is
  reported separately by `status`.

## Operator workflow

One supported lifecycle, all commands run from the repository root:

```bash
./deploy/lago/lago.sh init     # 1. generate deploy/lago/.env (random secrets, mode 600)
# ... add first-run seed values to deploy/lago/.env (see below) ...
./deploy/lago/lago.sh up       # 2. validate secrets, pull pinned images, wait for health
./deploy/lago/lago.sh status   # 3. health snapshot JSON (delegates to health.py --json)
LAGO_API_KEY=<operator key> \
  ./deploy/lago/lago.sh contract-probe --output deploy/lago/evidence/t01-contract.json
                              # 4. real Customer create/delete probe against the pinned API
./deploy/lago/lago.sh down    # 5. stop the stack; named data volumes are PRESERVED
./deploy/lago/lago.sh config  #    anytime: resolved Compose config with secrets redacted
```

`up` runs `docker compose up -d --wait`, so it returns only after every
healthchecked service is actually healthy. With the seed values in `.env`
(see next section) the one-shot `migrate` service also creates the operator
account and API key during the first `up`.

### Secrets

`lago.sh init` writes `deploy/lago/.env` (git-ignored, mode 600) with
cryptographically random values for `POSTGRES_PASSWORD`, `SECRET_KEY_BASE`,
the three Lago encryption keys, and a generated RSA private key. It refuses
to overwrite an existing file — delete `.env` first to regenerate (this
orphans existing data volumes' encryption; see *Reset behavior*).

`up` re-validates that every required secret is present and **differs from
the official Lago sample defaults** before it touches Docker, and that
`LAGO_API_URL`/`LAGO_FRONT_URL` agree with the configured ports. `config`
prints the resolved Compose configuration with all secret values replaced
by `***REDACTED***`.

No usable secret is ever committed, logged, or written to evidence files.

### First-run operator account and API key

Lago v1.53.0 has **no REST onboarding endpoint** (verified against the
pinned source: neither `/api/v1/onboarding` nor `/api/v1/api_keys` exists in
the v1.53.0 `lago-api` routes/controllers). The supported on-premise path is
the seed task that the official v1.53.0 Compose wires into the `migrate`
one-shot (`scripts/migrate.sh` runs `rake signup:seed_organization` whenever
`LAGO_CREATE_ORG=true`, creating organization + admin user + API key).

To use it, append to `deploy/lago/.env` before the first `up` (values are
local throwaway operator material — never commit them):

```dotenv
LAGO_CREATE_ORG=true
LAGO_ORG_USER_EMAIL=ops-t01@weknora.local
LAGO_ORG_USER_PASSWORD=<choose a local throwaway password>
LAGO_ORG_NAME=WeKnora T01
LAGO_ORG_API_KEY=<random value, e.g. python3 -c "import secrets; print(secrets.token_urlsafe(32))">
```

- With `LAGO_ORG_API_KEY` set, the seed mints the organization API key with
  exactly that value, so you know it without reading the database.
- If the stack is already up, re-run the one-shot instead of recreating:
  `cd deploy/lago && docker compose run --rm migrate` (the seed is
  idempotent: `find_or_create_by` everywhere).
- Alternatives that also work on this release, for completeness: log into
  the frontend at http://127.0.0.1:48890 with the seeded user, or use
  GraphQL `registerUser` → `loginUser` → `createApiKey` on the API.

The API key is **server-side operator input only**. The contract probe
reads it exclusively from the caller's `LAGO_API_KEY` environment variable —
never from `.env`, never logged, never included in any report — and sends
it only to the loopback API origin.

### Ports

| Port  | Service  | Binding              |
|-------|----------|----------------------|
| 48889 | Lago API | `127.0.0.1` loopback |
| 48890 | Frontend | `127.0.0.1` loopback |

PostgreSQL and Redis are **not published** to the host; they live on the
internal Compose network only. Each worktree can run an isolated stack by
setting `COMPOSE_PROJECT_NAME`, `LAGO_API_PORT`, and `LAGO_FRONT_PORT` in
its own `.env` (edit the `*_URL` values accordingly; `up` enforces
port/URL consistency).

### What `status` reports

`lago.sh status` (→ `health.py --json`) prints one snapshot:

- `overall`: `ready` only when the API answers `GET /health`, `api`,
  `api-worker`, `db`, and `redis` are all Compose-`healthy`, and
  `api-clock` is running. Anything partially broken is `degraded`; no
  answer or unknown service is `unavailable`.
- Per-service `state` meanings:
  - `healthy` / `unhealthy` / `starting` — the Compose healthcheck verdict
    (API: `GET :3000/health`; worker: Sidekiq alive on `:8080`; db:
    `pg_isready`; redis: `redis-cli ping`).
  - `running_unverified` — the container runs but the pinned official
    Compose defines no healthcheck for it (`api-clock`, `front`, `pdf`).
    Liveness only; no health claim.
  - `exited`/`restarting`/... — the raw Compose state; `unknown` — the
    service is missing entirely (never silently healthy).
- `release` and `images` always come from `images.lock.json`, never from
  live API responses, so the reported identity is the pinned truth.

### Image provenance

`images.lock.json` records repository, tag, immutable OCI digest, supported
platforms, and verification date for all five images (api, front, db,
redis, pdf). `compose.yaml` references each image **only** by
`repository@sha256:digest`; the lock and Compose are cross-checked by
`test_compose.py`. Result: what you run is byte-identical to what T01
verified, regardless of tag movement upstream. The topology mirrors the
official `getlago/lago` `docker-compose.yml` at tag `v1.53.0` with WeKnora
isolation constraints (loopback-only published ports, no fixed container
names, Lago-scoped named volumes).

### Reset behavior

- `lago.sh down` stops and removes containers/network but **never deletes
  the named volumes** (`down -v` is not used anywhere). PostgreSQL data,
  Redis data, the storage volume — including the seeded operator account
  and API key — survive, so the next `up` resumes the same state without
  re-seeding.
- To wipe everything and start fresh: `docker compose -p weknora-lago down
  -v` (or `docker volume rm` the three `weknora-lago_lago_*` volumes),
  delete `deploy/lago/.env`, and re-run `init` + `up`. Deleting `.env`
  alone does not delete data; regenerating encryption keys with old data
  present can render stored ciphertext unreadable.

## Community / Premium boundary

This stack is **Lago Community v1.53.0 without any license flag**
(`LAGO_LICENSE` stays empty). It must not, and does not, unlock Premium
features. T01 evidence covers Community behavior only: Customer
create/delete via the public API, health/readiness of api/worker/clock/
db/redis, and the pinned image identity. Any capability that turns out to
be Premium-gated on this release is a finding for the billing migration
(spec: *Lago Premium features, including Premium Invoice Preview or
Subscription Overrides* are out of scope), never a reason to enable a
license here.

## Reporting a blocked environment (`blocked-env`)

The contract probe distinguishes "the pinned runtime failed the contract"
from "the environment cannot exercise the contract":

- No API key in the caller environment → probe reports
  `status: "blocked-env"` (exit 2) and makes **zero** API calls.
- Docker unavailable / stack never started → `status` reports
  `unavailable` and the probe fails its health step; record that output
  as the blocked-env evidence instead of claiming anything passed.

Per the plan rules: a `blocked-env` report is honest evidence of an
environment gap, **not** a substitute for the real contract. Never commit
a blocked run as if the contract passed; fix the environment (start the
stack, obtain an operator key via the seed path) and re-run.

## Compliance gate

Lago is AGPL-3.0. Self-hosting it for WeKnora integration work triggers
AGPL-3.0 obligations that **require legal review before any production
use**. T01 supplies runtime and contract *evidence only* — it is not legal
approval, and the AGPL review remains an open production gate owned by the
billing migration's completion criteria, not by this directory.

## Evidence

Sanitized, reviewable runtime evidence for this environment lives in
[`evidence/`](evidence/README.md). Nothing under `evidence/` may contain
secret values; the evidence README defines the contract.
