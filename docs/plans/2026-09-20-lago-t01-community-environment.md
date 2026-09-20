# Lago T01 — Community Integration Environment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Provide one isolated, reproducible command to run pinned Lago Community `v1.53.0`, report API/worker/PostgreSQL/Redis health and release identity, and execute a real create-and-clean contract probe with reviewable evidence.

**Architecture:** Add a self-contained Lago deployment under `deploy/lago/`, independent from OpenMeter. A small operator CLI owns Compose lifecycle and health inventory; a separate contract probe uses a server-side API key and synthetic Customer identity, then always attempts cleanup. The repository does not expose a product-facing Lago endpoint in T01; the operator-visible health checker is the WeKnora-side evidence seam until Ticket #77 adds the Commercial Platform application seam.

**Tech Stack:** Docker Compose v2, Bash, Python 3 standard library, pinned Docker OCI index digests, Lago Community `v1.53.0`, unittest, HTTP JSON API.

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md`; parent and acceptance source: GitHub Issue #72 and Ticket #73.

## Global Constraints

- Lago is self-hosted Community; no Premium feature or license flag bypass is allowed.
- Pin Lago `v1.53.0` and immutable image digests. The API image OCI index digest is `sha256:4d1001772009beea6348b95b24938c3c289574516a312b1c191e83adbb546c8b`; frontend is `sha256:16646b2b1c834f110e57e3460d09cf24c7a487ced721f50f5886d65d4d6228af`; PostgreSQL image is `sha256:27a1e3a1138a69b5a0911ce125dfa613b9ced07604a2d781b9d31141f7f96036`; Redis image is `sha256:520775a41a63e77e06c73e35d2fd9cc15921a609516818796b4ecbb813078bc7`; PDF image `getlago/lago-gotenberg:7.8.2` is `sha256:e8eef6fb4a92a3440c70d06598b385e7eb2ec7fb2d9b908ca4aa028741a0b270`.
- Use PostgreSQL as the event store. Do not add ClickHouse, Kafka, or reuse OpenMeter database, Redis, volumes, or data.
- Bind host-accessible Lago ports to loopback only. Do not publish PostgreSQL or Redis ports to the host.
- Compose project, volume names, service names, and temporary external IDs must be Lago-specific. Do not set fixed `container_name` values that prevent isolated project names.
- Compose project name and host ports must be overrideable with `COMPOSE_PROJECT_NAME`, `LAGO_API_PORT`, and `LAGO_FRONT_PORT`, so each contract Ticket can run an isolated stack in its own worktree.
- Secrets are generated locally or injected from the caller environment; no usable key, password, or private material is committed, logged, or included in evidence.
- `LAGO_API_KEY` is server-side operator input for the synthetic contract probe; it is never sent to the browser, Agent context, or general application logs.
- T01 is an environment and evidence slice. Product-facing commercial behavior remains behind the provider-neutral Billing API and is delivered by later Tickets.
- Docker failures or missing credentials must be reported as blocked environment evidence, never as a passing contract.

## Review Focus

- Compose files and version lock disagree — test every service image reference against the lock and the recorded official tag/digest.
- Compose project isolation collides with existing OpenMeter or another Lago worktree — test unique project/volume identity and loopback-only host ports.
- A healthy API masks a dead worker, database, or Redis — health output must preserve each service's actual Compose health state.
- API readiness accepts only HTTP status without proving the pinned release — report both `/health` result and resolved immutable API image identity.
- Contract probe leaves a synthetic Customer behind after assertion or network failure — test cleanup on success and failure and surface cleanup failures distinctly.

---

### Task 1: Define and test the operator health snapshot

**Files:**
- Create: `deploy/lago/health.py`
- Create: `deploy/lago/test_health.py`
- Test: `deploy/lago/test_health.py`

**Interfaces:**
- Consumes: Docker Compose `ps --format json` service records and an HTTP GET to Lago API `/health`.
- Produces: `classify_service(row) -> {service, state, image, image_id}` and `overall_status(api_ok, services) -> ready | degraded | unavailable`; the CLI prints a JSON snapshot with locked release, API state, API worker and clock states, PostgreSQL state, Redis state, and overall result. A missing or unknown service is `unknown`, never silently `healthy`.

- [ ] **Step 1: Write failing behavior tests**

Add `unittest` cases with these observable inputs and outputs:

```python
def test_dead_worker_degrades_an_otherwise_healthy_api():
    services = {
        "api": {"Health": "healthy", "State": "running"},
        "api-worker": {"Health": "unhealthy", "State": "running"},
        "api-clock": {"State": "running"},
        "db": {"Health": "healthy", "State": "running"},
        "redis": {"Health": "healthy", "State": "running"},
    }
    assert overall_status(api_ok=True, services=services) == "degraded"
```

Also test `healthy`, `unhealthy`, `starting`, exited, no-healthcheck, and missing-service mapping; a running service without a healthcheck is reported as `running_unverified`, and a missing service is `unknown`. `api-worker` supplies the worker-health verdict; `api-clock` is reported separately because the pinned official Compose has no healthcheck for it. Test the reported Lago release/digests are loaded from `images.lock.json`, never inferred from user-controlled API response text.

- [ ] **Step 2: Run the tests and verify the expected RED**

Run: `python3 -m unittest deploy.lago.test_health -v`

Expected: tests fail because the health module and classification behavior do not exist.

- [ ] **Step 3: Implement the minimal health snapshot module**

Implement pure service classification and overall-status functions, plus `python3 deploy/lago/health.py --json`. The CLI runs `docker compose ps --format json` within the selected Lago project and GETs `http://127.0.0.1:${LAGO_API_PORT}/health`. It reports the API endpoint result; API worker and API clock as separate worker processes; PostgreSQL and Redis health; plus frontend and PDF container state. Where Compose defines no healthcheck, the report says `running_unverified` instead of claiming a health probe. Overall status is ready only when API `/health`, API worker, PostgreSQL, and Redis are healthy and `api-clock` is running; a missing/stopped clock is degraded, while a running clock remains explicitly unverified. Release/digests come from the lock file. Pure classifications remain testable without Docker or network access.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run: `python3 -m unittest deploy.lago.test_health -v`

Expected: all health classification and degraded-state tests pass.

- [ ] **Step 5: Commit the health snapshot slice**

Commit the health module and tests with a focused message. Do not include Compose changes from later tasks.

### Task 2: Add the isolated pinned Lago Community Compose environment

**Files:**
- Create: `deploy/lago/compose.yaml`
- Create: `deploy/lago/images.lock.json`
- Create: `deploy/lago/.env.example`
- Create: `deploy/lago/.gitignore`
- Create: `deploy/lago/lago.sh`
- Test: `deploy/lago/test_compose.py`

**Interfaces:**
- Consumes: the health snapshot contract from Task 1.
- Produces: `./deploy/lago/lago.sh init|up|down|status|config|contract-probe` as the supported local operator command.

- [ ] **Step 1: Write failing Compose contract tests**

Add tests for the manifest schema and Compose text contract. The manifest must identify release `v1.53.0` and five image roles (`api`, `front`, `db`, `redis`, `pdf`); each Compose image reference must equal its locked `repository@sha256:digest`. Assert Compose includes `api`, `api-worker`, `api-clock`, `db`, `redis`, `front`, and `pdf`; contains no `container_name`; publishes only loopback ports; does not publish database/Redis; uses Lago-only named volumes; and contains no ClickHouse/Kafka services.

The lock/Compose equality assertion should have this shape:

```python
def test_every_runtime_image_uses_its_locked_digest():
    lock = json.loads((DEPLOY_DIR / "images.lock.json").read_text())
    compose = (DEPLOY_DIR / "compose.yaml").read_text()
    for image in lock["images"].values():
        assert f'{image["repository"]}@{image["digest"]}' in compose
```

- [ ] **Step 2: Run the tests and verify the expected RED**

Run: `python3 -m unittest deploy.lago.test_compose -v`

Expected: the test fails because the Lago deployment files do not exist.

- [ ] **Step 3: Add the pinned Compose deployment and safe operator lifecycle**

Base the topology on the official Lago `v1.53.0` Compose definition, including API, API worker, API clock, migration, PostgreSQL, Redis, frontend, and Gotenberg PDF. Pin all five images to the exact OCI index/manifest digests in Global Constraints and record tag, repository, digest, supported platform, and verification date in `images.lock.json`. Use project-scoped volume names and no fixed container names. `lago.sh init` creates a private ignored `.env` with cryptographically random secrets and refuses to overwrite an existing file. `COMPOSE_PROJECT_NAME`, `LAGO_API_PORT`, and `LAGO_FRONT_PORT` can be set per worktree. `up` validates that required secret values differ from official sample defaults, then runs `docker compose up -d --wait`. API defaults to loopback port `48889`, frontend to loopback port `48890`; DB and Redis stay internal. `status` delegates service state to Task 1's health snapshot; `config` prints no secret values.

- [ ] **Step 4: Verify static and Compose behavior**

Run: `python3 -m unittest deploy.lago.test_compose -v`

Run: `./deploy/lago/lago.sh init`

Run: `./deploy/lago/lago.sh config`

Expected: unit checks pass and Docker Compose resolves the configuration without pulling images or creating containers.

- [ ] **Step 5: Commit the deployment slice**

Commit only the Compose, image lock, env template, ignore rules, operator lifecycle command, and tests.

### Task 3: Add the isolated real Lago Customer contract probe

**Files:**
- Create: `deploy/lago/contract_probe.py`
- Create: `deploy/lago/test_contract_probe.py`
- Test: `deploy/lago/test_contract_probe.py`

**Interfaces:**
- Consumes: API base URL, `LAGO_API_KEY`, release/image identity from the lock, and a unique probe run ID.
- Produces: a sanitized JSON result with request outcome, HTTP status, Lago release identity, synthetic Customer ID, cleanup result, timestamps, and `pass | fail | blocked-env` status.

- [ ] **Step 1: Write failing probe behavior tests**

Use `http.server` as an in-process HTTP fixture. Assert that the probe sends Bearer auth only to the configured Lago origin, creates a unique Customer with `POST /api/v1/customers`, deletes that same `external_customer_id` with `DELETE /api/v1/customers/{external_customer_id}`, rejects non-2xx or malformed responses, and distinguishes cleanup failure from create failure. Force an exception after creation and prove cleanup still runs. Assert neither API key nor arbitrary response body is written to the report.

The failure-path test must prove the cleanup side effect directly:

```python
def test_probe_deletes_created_customer_after_later_failure(server):
    server.create_customer_response = {"customer": {"external_id": "weknora-t01-probe-a1"}}
    server.fail_after_create = True
    result = run_probe(server.url, api_key="secret-for-test-only")
    assert result.status == "fail"
    assert server.deleted_external_ids == ["weknora-t01-probe-a1"]
    assert "secret-for-test-only" not in result.serialized()
```

- [ ] **Step 2: Run the tests and verify the expected RED**

Run: `python3 -m unittest deploy.lago.test_contract_probe -v`

Expected: tests fail because the contract probe does not exist.

- [ ] **Step 3: Implement the minimal create-and-clean probe**

Before implementation, confirm the Customer create/delete operations and required `customer` request fields against the official API contract for the pinned release. Use `POST /api/v1/customers` and `DELETE /api/v1/customers/{external_customer_id}` only if that release contract confirms them; if it differs, record the actual operation in the ledger before coding. Generate a unique external ID `weknora-t01-probe-<uuid>` and a non-personal synthetic name. Always attempt deletion in `finally` after successful creation. Return a non-zero exit for failed health, create, or cleanup; classify missing API key as `blocked-env`. Save reports only to an explicit output path and redact secrets.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run: `python3 -m unittest deploy.lago.test_contract_probe -v`

Expected: success, API failure, malformed response, auth redaction, and cleanup behavior pass.

- [ ] **Step 5: Commit the real contract probe slice**

Commit only the probe and its tests.

### Task 4: Close the operator workflow with documentation and real runtime evidence

**Files:**
- Create: `deploy/lago/README.md`
- Create or update: `deploy/lago/evidence/README.md`
- Test: `deploy/lago/test_compose.py`
- Test: `deploy/lago/test_contract_probe.py`

**Interfaces:**
- Consumes: the lifecycle CLI, health snapshot, image lock, and real contract probe from Tasks 1–3.
- Produces: a single documented `init → up → status → contract-probe → down` workflow with sanitized, reviewable evidence.

- [ ] **Step 1: Add the operator walkthrough and evidence contract**

Document exact commands, required Docker Compose version, secret initialization, ports, reset behavior that preserves named volumes, image/tag/digest provenance, API/worker/database/Redis health meanings, Community/Premium boundary evidence, and how to report missing Docker/API credentials as `blocked-env`. State that Lago's AGPL review remains a production gate and T01 supplies evidence, not legal approval.

- [ ] **Step 2: Run the static checks**

Run: `python3 -m unittest deploy.lago.test_health deploy.lago.test_compose deploy.lago.test_contract_probe -v`

Run: `./deploy/lago/lago.sh config`

Expected: all tests pass and resolved Compose config is valid.

- [ ] **Step 3: Run the real local environment and contract probe**

Run: `./deploy/lago/lago.sh up`

Run: `./deploy/lago/lago.sh status --json`

Run: `./deploy/lago/lago.sh contract-probe --output deploy/lago/evidence/t01-contract.json`

Expected: API, API worker, API clock, PostgreSQL, and Redis meet their health/readiness rules; frontend and PDF are reported separately; a synthetic Customer is created and deleted; sanitized evidence records release/digests and cleanup. If Docker or an operator API key is unavailable, record `blocked-env` and do not claim the real contract passed.

- [ ] **Step 4: Stop services without deleting persisted volumes**

Run: `./deploy/lago/lago.sh down`

Expected: project containers stop; named Lago data volumes remain. No `down -v` is used.

- [ ] **Step 5: Re-run focused checks and commit the operator workflow**

Run the Task 2 and Task 3 commands again, review evidence for secrets, then commit only documentation and sanitized evidence.

## Plan Self-Review

- **Spec coverage:** T01 acceptance maps to isolated fixed-version Community runtime, digest lock for every official Compose image, operator-visible API/worker/clock/PostgreSQL/Redis state, real create/delete API probe, evidence, and OpenMeter isolation. API worker/DB/Redis health is actively probed; clock liveness is reported as running but unverified because official Compose has no worker healthcheck. No commercial behavior beyond T01 is included.
- **Execution detail scan:** every task names its files, command, and expected result. The real T01 run must confirm Customer create/delete behavior on the pinned `v1.53.0` runtime before contract evidence can pass.
- **Interface consistency:** Task 1 exports the health snapshot used by Task 2 and Task 4. Task 2 exports lifecycle and resolved Compose config used by Tasks 3 and 4. Task 3 exports a sanitized probe/report used by Task 4.
- **Review focus:** the five environment/data failure cases are listed above and assigned to the corresponding Compose, health, or probe tests.
- **TDD:** behavior tests precede the health mapping, Compose invariants, and real API probe implementation; external-runtime verification is explicitly separate from test-server evidence.
