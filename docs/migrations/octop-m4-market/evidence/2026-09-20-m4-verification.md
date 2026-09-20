# M4 Skill Market — End-to-End Verification (2026-09-20)

Branch `octop-m4-market`, HEAD `a32476a3` (feat(web): experts page remote and
tenant-market tabs) — the full M4 stack: SkillHub client (search/rankings/
download + exhaustive zip port + TTL cache, `internal/agent/skills/skillhub`),
skillset index + expert materialization (`internal/agent/experts`,
migrations `000173`/`000094`), market routes
(`internal/router/routes_skill_market.go`), tenant skill market
(`000174`/`000095`), tenant expert market (`000175`/`000096`), api-client
market module, `MarketPage` + experts-page market tabs. Verifier: Task 9 of
the M4 skill-market milestone. Everything in §1–§6 ran against a real server
process with a real SQLite database and — where the flow leaves the process —
a REAL SkillHub client talking to a local mock registry standing in for the
public one; no mocks sit between curl and the handler. Substitutions are
labeled where the environment blocked a live path.

## Environment

| Item | Value |
|---|---|
| Server | prebuilt binary of HEAD: `go build -tags sqlite_fts5 ./cmd/server`, run from a scratch CWD with `migrations` symlinked to the repo and a **scratch copy** of `config/` (repo `config/` untouched) |
| Port | `127.0.0.1:18095` via `SERVER_PORT`/`SERVER_HOST` |
| SkillHub host | **config-file override** (the T1 ledger records there is no env knob): the scratch `config/config.yaml` gained `skillhub_market: {host: http://127.0.0.1:18094, timeout_seconds: 10}` — exactly the operator-facing mechanism deployments would use |
| Mock registry | `mock_skillhub.py` (python scratch script, never committed) on `127.0.0.1:18094`, serving fixture JSON + zips from disk: `/api/v1/search`, `/api/v1/showcase/{hot,featured,newest,recommended,trending,paid}`, `/api/v1/download?slug=`, `/api/v1/skillsets[?page=&pageSize=]`, `/api/v1/skillsets/<slug>`. Every hit is logged with its User-Agent — the log shows **`weknora-skillhub-market/1.0`** (the port's rebranded UA) making 20+ of the 26 hits: the real Go client, not curl, consumed the mock. |
| DB | `DB_DRIVER=sqlite`, `DB_PATH=<scratch>/weknora.db`; migrations ran **0 → 96 cleanly on first boot** — `000094` (expert_installs), `000095` (published_skills), `000096` (published_experts) all part of the clean run (`schema_migrations` = 96, dirty 0) |
| Redis | scratch docker container `redis:7-alpine` on `127.0.0.1:16379` via `REDIS_ADDR` — the skill-install progress bus (publishProgress/SSE) is Redis-backed and degrades to a poll sentinel in Lite mode; Redis was added so the REAL SSE frames could be witnessed |
| Retrieval | `RETRIEVE_DRIVER=sqlite` |
| Storage | `LOCAL_STORAGE_BASE_DIR=<scratch>/files` (the expert-market and published-experts roots nest under it) |
| Sandbox | one docker sandbox config (`sandbox_type: docker`, image `wechatopenai/weknora-sandbox:main`) via `WEKNORA_SANDBOX_DOCKER_ENABLED=true`; docker daemon up on this host — the install runs created REAL sandbox containers |
| SSRF | `SSRF_WHITELIST=127.0.0.1` (mock registry + scratch redis are raw loopback IPs) |
| Auth | self-serve registration: `m4-e2e-admin@local.dev` (owner of tenant 1, JWT 277 chars, short TTL — re-login between phases) plus a second member `m4-e2e-member@local.dev` joined via the tenant invite-link flow (`POST /tenants/1/invite-links` role `contributor` → `POST /auth/register-by-invite`) |
| LLM | **down in this environment**: the repo's committed dev default `builtin-llm-mock` points at a LAN mock OpenAI server (`<llm-mock-host>:18090`, raw IP, redacted per the M3 lesson) that is not running today — probe returns connection failure. This bounds only the sandbox-install runs' final LLM step (§1, honestly labeled); no M4 market flow calls an LLM. |

Scratch root (nothing committed): `.superpowers/sdd/2026-09-20-skill-market-m4/e2e/`.

## 1. Remote market: search, rankings, install (the T1/T3 stack)

### 1.1 Search — real client against the mock

`GET /api/v1/skills/market/search?q=e2e&limit=20` → 200 (`stale: false`,
3 results). The three fixtures exercise the ported normalization chains:
`displayName`→`name`→`slug` (entry 3 has only `name`/`description` — the
fallback chain), `summary`→`description`, version passthrough:

```
e2e-skill-alpha | E2E Skill Alpha | 1.0.0 | Markdown report helper used by…
e2e-skill-beta  | E2E Skill Beta  | 2.3.1 | Data sanity checker used by…
e2e-qui-gon     | E2E Qui-Gon     |       | name/description fallback-chain…
```

### 1.2 Rankings

`GET /api/v1/skills/market/rankings/{kind}` for all six kinds
(hot/featured/newest/recommended/trending/paid) → 200, `stale: false`,
2 results each; `rankings/bogus` → `400 skillhub: unsupported ranking kind:
"bogus"`. `limit=0` on search → 400 (`limit must be a positive integer`).

### 1.3 Remote skill install — 202 → SSE progress → catalog row (real pipeline)

Sandbox config created via `POST /api/v1/sandbox-configs` (201). Then:

```
POST /api/v1/skills/market/install {"slug":"e2e-skill-beta","sandbox_config_ids":["<cfg>"]}
  → 202 {"data":{"catalog_id":"086eedb5-1c2c-45f6-8539-583774badc8f",
                 "install_ids":["6cbf3e3e-3c94-4c18-ae07-4a124ce7719f"]},"success":true}
```

**SSE progress** (`GET /api/v1/sandbox-configs/<cfg>/skills/<skillId>/install-events`,
captured live over the 25 s window):

```
event:message  data:{"percent":25,"stage":"sandbox_ready","done":false}
event:message  data:{"percent":28,"stage":"seeding","log":"seeding 3 files","done":false}
event:message  data:{"percent":35,"stage":"seeded","done":false}
event:message  data:{"percent":100,"stage":"failed","log":"baseURL SSRF check failed: …","status":"failed","done":true}
```

The run is real end-to-end through the sandbox stage: docker created the
container (`sandbox_ready`), seeded the validated package's 3 files
(SKILL.md, `assets/beta.txt`, LICENSE), and only the install agent's LLM
call failed — the committed dev mock LLM host is down in this environment
(Environment). **Labeled honestly**: the terminal state of both install rows
is `failed` at the model step (`error: baseURL SSRF check failed…` in
`tenant_skills`), not `installed`. Everything the M4 market milestone owns —
Download → ParsePackage → repack → RegisterCatalogFromArchive →
InstallCatalogToConfigs → install row + live progress — ran for real, and a
first install without Redis (Lite mode) additionally showed the designed
fallback frame (`live progress is unavailable; poll the skill for its status`).

**Catalog row** in the scratch SQLite DB (the binding check):

```
tenant_skill_catalog:
  ad1695aa… | e2e-skill-alpha | 1.0.0 | bundle_ref resource://jo0UlHAY… | sha c2b5d7dd…
  086eedb5… | e2e-skill-beta  | 2.3.1 | bundle_ref resource://R4blMDwz… | sha eddb6dcf…
  each with 1 tenant_skills install row pointing at it (catalog_id FK)
```

### 1.4 Stale / unreachable semantics (per-path)

Cache TTL is 300 s (singleflight + stale-fallback, `CachedClient`). Sequence
witnessed on the wire:

| Path | Mock up (primed) | TTL expired + mock stopped | Never cached + mock down |
|---|---|---|---|
| `search` | 200 `stale:false` | **200 `stale:true`** (3 results) | **503** `the skill market is unreachable…` |
| `rankings/hot` | 200 `stale:false` | **200 `stale:true`** (2 results) | 503 |
| skillset index `GET /experts/market` | 200 `stale:false` | **200 `stale:true`** (1 skillset, `installed` flags still live from the ledger) | 503 |
| skillset detail | 200 `stale:false` | **200 `stale:true`** | 503 |
| `install` (Download) | 202 | **503** `cannot download the skill market package…: dial tcp 127.0.0.1…` — Download bypasses the cache by design | 503 |

The no-cache 503s were witnessed first (server restarts had emptied the
in-memory cache), then the mock was restarted, listings primed, 5+ minutes of
other flows run (TTL expiry), mock stopped — and every listing path answered
its cached value with `stale:true`. No path ever returned a silent empty list.

### 1.5 Invalid packages (the zip port's guarantees, on the wire)

| Fixture served by the mock for `download?slug=` | Result |
|---|---|
| `e2e-zip-bomb` — SKILL.md + a 2 MiB zeros member compressing ~850:1 | **400** `the skill market package for e2e-zip-bomb is invalid: skillhub: package failed validation: exceeds a safety limit: zip compression ratio is too high: payload.bin` |
| `e2e-traversal` — member `../evil.txt` | **400** `…validation: unsafe zip path entry: "../evil.txt"` |
| unknown slug (`no-such-skill`) — upstream 404 | **503** `cannot download…: HTTP 404` — the registry does not distinguish 404 from outage on this wire (documented T2/T3 behavior) |

## 2. Skillset → expert (the T2/T3 stack)

### 2.1 Index and detail

`GET /api/v1/experts/market` → 200, 1 skillset (`e2e-kit-research`, zh/en
metadata, `skill_slugs: [e2e-skill-alpha, e2e-skill-beta]`, `installed:
false`); `GET /api/v1/experts/market/e2e-kit-research` → detail with
`name_en`/`description_en` picked from the raw `displayNameEn`/`summaryEn`
fields. Invalid slug chars route and fail validation → `400 invalid skillset
slug`; an upstream 404 for an unknown slug maps to 503 (the registry's 404 is
indistinguishable from an outage — known deferred T2-minor, honestly noted).

### 2.2 Install with agent_name → 201, materialized expert + agent

```
POST /api/v1/experts/market/e2e-kit-research/install {"agent_name":"研究套件助手-E2E"}
  → 201 {"agent":{"id":"3924bea5-…","name":"研究套件助手-E2E", …},
         "pending_skills":["e2e-skill-alpha","e2e-skill-beta"],
         "skill_install_ids":[],
         "expert_id":"skillhub-skillset-e2e-kit-research",
         "snapshot_sha256":"7aa65ea73de66168…"}
```

Agent `config.expert_source` = `{"expert_id":"skillhub-skillset-e2e-kit-research",
"source":"skillhub","slug":"e2e-kit-research"}` — the skillset slug carried in
provenance exactly as the brief binds. `skills_selection_mode: selected`,
`selected_skills` = the two SKILL.md names; no `sandbox_config_id` was passed,
so both stay `pending` (degrade-never-fail, the M2 contract).

**Materialized dir** on disk (`<scratch>/files/expert-market/1/e2e-kit-research/`):
`manifest.yaml` (id `skillhub-skillset-e2e-kit-research`, zh+en label/description,
`prompt_files: [SOUL.md]`, `skills: [e2e-skill-alpha, e2e-skill-beta]`,
`agent_config.agent_mode: smart-reasoning`) + `SOUL.md` + the two validated
skill payloads under `skills/<slug>/` (SKILL.md, LICENSE, and each package's
extra member). **expert_installs row**: slug `e2e-kit-research`, storage_ref
= the install dir, snapshot_sha256 identical to the API answer, created_by
the admin user.

**Catalog merge**: `GET /api/v1/experts` now lists 5 experts — the 4 builtins
plus `skillhub-skillset-e2e-kit-research` (label `E2E 研究套件`), and the
market index flips `installed: true` (ledger-driven, live even on the stale
listing).

## 3. Tenant skill market (T4)

```
POST /api/v1/skills/catalog/<alpha-catalog-id>/publish  {}   → 200 {catalog_id, published_by, published_at}
GET  /api/v1/market/tenant/skills                          → 200 [{catalog_id, name, description, version,
                                                                publisher_name: "m4e2eadmin", installed: true}]
POST /api/v1/market/tenant/skills/<catalogId>/install      → 202 {"installs":{"<cfg>":"289059cf-…"}}
```

**Zero content duplication — before/after evidence** (the brief's suggested
storage-ref/SHA comparison, from the scratch DB):

```
                       bundle_ref (tail)      bundle_sha256
PRE  e2e-skill-alpha   resource://jo0UlHAY…   c2b5d7dd…
POST e2e-skill-alpha   resource://jo0UlHAY…   c2b5d7dd…   (byte-identical)
catalog row count: 2 → 2          published_skills row: (id, catalog_id, published_by) — a
                                  visibility flag, no storage copy, no bundle bytes
```

The 202 returned the SAME skill id `289059cf…` as the original remote install
(same-bundle reinstall refreshes the shared row; `InstallCatalogToConfigs`
resolves the shared catalog row — the shared-row semantics per T4). The
publish body is strict: `{"x":1}` → 400 `publish takes no body`.

## 4. Tenant expert market (T5)

Admin created a probe agent whose config deliberately carries the fields the
whitelist must drop (`model_id`, `knowledge_bases` + `kb_selection_mode:
selected`, `sandbox_config_id`, `mcp_services`, `memory_enabled`,
`allowed_tools`, plus persona `INTJ` + style, `selected_skills:
[e2e-skill-alpha]`, `subagents: [product-manager]`).

```
POST /api/v1/agents/<id>/publish-expert {"name":"E2E 团队专家","description":"…"}
  → 201 {id: cbd87bf6-…, agent_id, snapshot_sha256: 6530a22e…, published_by, …}
```

**Exclusion assertions on the written bytes** — the snapshot tree
(`<scratch>/files/published-experts/1/tenant-expert-<agentID>/`,
manifest.yaml + SOUL.md) greps CLEAN for
`builtin-llm-mock|fake-kb|knowledge_bases|sandbox|memory_enabled|fake-mcp|mcp_services|allowed_tools`;
`manifest.yaml` shows `persona_mbti: ""` (the INTJ block is baked into
SOUL.md instead — one render, never doubled), `skills: [e2e-skill-alpha]`
with `skill_refs: true` (references, never payloads), `subagents:
[product-manager]`, `agent_config` = minimal smart-reasoning only.

**Cross-member install** (the second user, contributor role):

```
GET  /api/v1/market/tenant/experts          (member token) → 200, sees the expert,
                                            publisher_name "m4e2eadmin", installed: false
POST /api/v1/market/tenant/experts/<id>/install {"agent_name":"团队专家副本-Member"}
  → 201 agent 686888a2-… created for the MEMBER,
     expert_source {"expert_id":"tenant-expert-<agentID>","source":"tenant","slug":"<agentID>"},
     selected_skills [e2e-skill-alpha] (pending), subagents [product-manager],
     and the created agent carries model_id "" / kb_selection_mode "" / no sandbox —
     the exclusions flowed through the whole instantiate path.
```

## 5. Negatives (all on the wire)

| Case | Result |
|---|---|
| any market endpoint without token (search, skillset install, tenant lists, publish) | `401 Unauthorized: missing authentication` |
| contributor publishes a skill (`POST /skills/catalog/:id/publish`) | `403 Forbidden: insufficient workspace role` (Admin+ guard) |
| contributor installs a remote skill (`POST /skills/market/install`) | `403` (Admin+ — bakes remote code into images) |
| contributor publishes an agent expert | `403` (OwnedAgentOrAdmin — not their agent) |
| install body unknown field / trailing input / empty sandbox list / blank slug | `400` each (strict decode) |
| publish-expert unknown field | `400 invalid publish request: unknown fields…` |
| publish on unknown catalog id | `404 skill not found` |
| install a catalog skill that is NOT published | `404` (via the tenant skill market surface) |
| publish-expert on unknown agent | `404` |
| search `limit=0` | `400` |
| rankings unknown kind | `400 unsupported ranking kind` |
| skillset slug failing `[A-Za-z0-9_.-]+` | `400 invalid skillset slug` |
| unreachable host, cached listings | 200 `stale:true` (§1.4) |
| unreachable host, no cached value / install download | `503` with the underlying cause (§1.4) |
| zip bomb / traversal package | `400` with the member named (§1.5) |

## 6. Frontend

- `pnpm --filter @weknora/web build` (= `tsc -b && vite build`) → **exit 0**,
  `✓ built in 14.42s`; only the pre-existing Rollup chunk-size advisories.
  `MarketPage-3Fz8ToC4.js` and `ExpertsPage-6ejKwhlA.js` present in
  `dist/assets/`. Manual browser smoke skipped (jsdom suites pin the page
  contracts; same posture as M1–M3).
- `apps/web` node test runner: `src/market/MarketPage.test.tsx` → **7/7
  pass** (search/rankings render, install progress lifecycle incl. the
  202-partial-failure envelope, done drawer); `src/experts/ExpertsPage.test.tsx`
  → **21/21 pass** (remote skillset tab, tenant-market tabs, publish dialog,
  deep-link hydration).
- `packages/api-client` suite → **268/268 pass** (M4's market module added
  16 tests over M3's 252).

## 7. Live probe of the public registry (best-effort, not a gate)

`https://api.skillhub.cn` is **reachable today** (2026-09-20):

```
GET /api/v1/search?q=a&limit=5          → 200 {"results":[…5…]}   slugs: a, agent-browser, anti-fraud…
GET /api/v1/skillsets?page=1&pageSize=5 → 200 {"skillSets":[…5…], "total": 56}
                                          slugs: tech-test-automation, tech-bug-troubleshooting…
GET /api/v1/showcase/hot                → 200 {"section":"hot_downloads","skills":[…],"total":…}
```

The live `search` and `skillsets` documents carry exactly the shapes and
field names the port parses (`results` / `skillSets` with
`slug`/`displayName`/`summary`/`displayNameEn`…). One observation recorded
without gating: the live showcase document nests its list under `skills`
(plus `section`/`total`), while the port — following Octop's client — reads
`results`; a live rankings call would therefore normalize to an empty (200,
non-stale) list rather than error. Mock-based verification used the
`results` shape Octop/WeKnora code against.

## 8. Go test gates

```
go test ./internal/agent/skills/skillhub/ ./internal/agent/experts/ \
        ./internal/application/service/ ./internal/handler/ ./internal/router/ \
        ./internal/application/repository/
```

| Package | Result at HEAD `a32476a3` | Pre-M4 base (temp worktree at `8676ab0e`) | Notes |
|---|---|---|---|
| `internal/agent/skills/skillhub` | **ok** | — (M4's own package) | client/skillsets/zip/cache tests incl. adversarial zips |
| `internal/agent/experts` | **ok** | — (M4 extended it) | materialize/publish/scan tests green |
| `internal/application/repository` | FAIL (1) | **identical FAIL (1)** | `TestArtifactVersionsMigrationDownIsReversible` — `duplicate column name: backup_reconciled` in the artifact-versions migration's down/re-up cycle; joined the base from main after M3 (M3's run predates it); no M4 commit touches `artifact_version*`. The M4 repo tests (published_skill, published_expert, expert_install) all pass. |
| `internal/handler` | FAIL (2) | **identical FAIL (2)** | `TestExecutionTargetRegistrationGinSQLiteLifecycle`, `TestExecutionRegistrationChallengeRequiresTenantAndOwner` — the execution-registration schema-drift reds documented in M1/M2/M3. All market handler tests (skill market, tenant skill market, tenant expert market) pass. |
| `internal/router` | FAIL (1) | **identical FAIL (1)** | `TestExecutionRegistrationRoutesUseRealRegistrationTargetRevokeLifecycle` — same pre-existing family. The three market coexistence/route tests pass. |
| `internal/application/service` | FAIL (1) | **identical FAIL (1)** | `TestExecuteDurableRunPersistsBudgetExhaustionForNotification` — the M3-documented red (expects a `budget_exhausted` event; run emits `run_failed`/`task_budget_exhausted`). All M4 service tests (skill market, tenant skill market, tenant expert market) pass. |
| web build / page tests / api-client | **ok** (7/7, 21/21, 268/268) | — | §6 |

Five reds, every one reproduced byte-identically at the pre-M4 base `8676ab0e`
before any M4 commit exists — all pre-existing, none introduced or worsened
by the milestone.

## Notes and gaps

1. **Sandbox-install terminal state (labeled substitution).** Both remote
   installs end `failed` at the install agent's LLM call: the committed dev
   mock LLM (`builtin-llm-mock`, LAN host) is down in this environment today.
   Everything M4 owns in that pipeline — download, exhaustive zip
   validation, repack, catalog registration into the real SQLite DB, the 202
   + live SSE frames, docker sandbox creation and 3-file seeding — ran for
   real (§1.3). A scripted OpenAI-compatible mock (M3's `e2e-scripted-mock`
   pattern) would be needed to drive the model loop to `installed`; not
   attempted — out of the milestone's surface and the binding checks were
   all witnessed.
2. **Stale witness needed a restart cycle.** The in-memory listing cache is
   per-process; the no-cache 503s were witnessed after restarts, the
   stale:true 200s after a prime + TTL-expiry + mock-stop cycle. Both
   branches of every listing path are covered in §1.4's table.
3. **Unknown skillset upstream-404 → 503** (not 404): the registry does not
   distinguish a missing slug from an outage on the detail wire — the
   T2-ledger deferred minor, re-confirmed live here (§2.1).
4. **Live showcase shape observation** (§7): recorded for a future
   registry-contract task; the port is byte-faithful to Octop's client,
   which also reads `results`.
5. **Scratch is scaffolding.** The mock registry script, fixtures, scratch
   config copy, redis container, sqlite DB, server binary, logs and curl
   outputs lived under the git-ignored scratch root and are deleted after
   this document was written; the repo's committed `config/` was never
   modified (the host override lived only in the scratch copy). No secrets
   committed — E2E passwords and JWTs stayed in scratch; LAN IPs are
   redacted (`<llm-mock-host>`; the SkillHub mock ran on `127.0.0.1`).
