# M2 Expert Templates — End-to-End Verification (2026-09-19)

Branch `octop-m2-experts`, HEAD `0f711ee7` (refactor(web): localized experts
instantiate-error toast copy) — the full M2 stack: experts scanner + 4 pilot
experts under `config/experts/`, `ExpertSource` provenance + instantiate
mapping, skill resolution with async install bridge, `/api/v1/experts` routes
(wired in container), api-client experts module, React `/platform/experts`
page + provenance badge. Verifier: Task 8 of the M2 expert-templates
milestone. Everything below ran against a real server process with a real
SQLite database; no mocks sit between curl and the handler.

## Environment

| Item | Value |
|---|---|
| Server | prebuilt binary of HEAD: `go build -tags sqlite_fts5 ./cmd/server`, run from a scratch CWD (see migrations note below) |
| Port | `127.0.0.1:18091` via `SERVER_PORT`/`SERVER_HOST` (18090 is occupied on this host by the mock LLM server itself) |
| DB | `DB_DRIVER=sqlite`, `DB_PATH=<scratch>/weknora.db` (git-ignored scratch; migrations ran 0 → 82 on first boot) |
| Migrations workaround | **known main breakage**: `migrations/sqlite` carries two `000079_*` pairs (`000079_commercial_reservations_owner.*` + `000079_message_feedback.*`, from main commit `c9310a09`), so golang-migrate refuses to open the source and **zero migrations run** — every sqlite table is then missing. Workaround used here (scratch only, no tracked file touched): ran the server from a scratch dir whose `migrations/` is a copy with `000079_message_feedback.*` renumbered to `000082_message_feedback.*`; `config` is a symlink to the repo's `config/` so experts/templates load verbatim. This same duplicate breaks 88 service-package tests (see Gates) and deserves a dedicated fix task on main. |
| Redis | none (Lite mode, in-process stream manager) |
| Retrieval | `RETRIEVE_DRIVER=sqlite` |
| Auth | self-serve registration (`registration_mode: "self_serve"`); registered `experts-e2e@local.dev`, `POST /api/v1/auth/login` → JWT (276 chars; short TTL — re-login between phases) |
| LLM | repo's committed dev default `config/builtin_models.yaml` → `builtin-llm-mock` (LAN mock OpenAI server at `192.168.3.30:18090`, already running on this host). Needed `SSRF_WHITELIST=192.168.3.30` (raw-IP base URL). |
| LLM request dump | `WEKNORA_LLM_STREAM_RAW_DUMP_DIR=<scratch>/llm-dump` — one JSONL per stream; line 1 = request wrapper (`data.messages`), later lines = streamed `chat.completion.chunk`s |
| Durable chat | `WEKNORA_AGENT_RECOVERY_ENABLED=true WEKNORA_AGENT_RECOVERY_ADMISSION_ENABLED=true` (tRPC admission opt-in, per M1 playbook) |
| Docker sandbox | one restart with `WEKNORA_SANDBOX_DOCKER_ENABLED=true` (docker 29.4.0 on host) + `LOCAL_STORAGE_BASE_DIR=<scratch>/files` (default `/data/files` is read-only on macOS; see §5) |

Scratch root (nothing committed): `.superpowers/sdd/2026-09-19-experts-m2/e2e/`.

## 1. Catalog: `GET /api/v1/experts` → 4 summaries, locale-resolved zh

```
count: 4 (ordered by directory name)
  general-assistant: 小通 · 通用助手           quick_prompts=12 skills=[]
  news-trend:        热点记者                  quick_prompts=6  skills=[daily-hot-news]
  ops-engineer:      运维工程师 Ops            quick_prompts=6  skills=[]
  stock-assistant:   老钱 · 证券观察员         quick_prompts=6  skills=[stock-info]
```

Each summary carries `icon_name`/`color`/`persona_mbti` (empty for the pilot
experts — only Octop's MBTI-tagged templates set it). Negative/auth posture:

- without token → `401 {"error":"Unauthorized: missing authentication"}`

## 2. Detail: `GET /api/v1/experts/ops-engineer`

```
label: 运维工程师 Ops | icon: terminal | color: #16a34a
persona_markdown: 3461 chars, starts "You are **Ops**, a senior systems reliability engineer and shell command expert."
  - contains the verbatim SOUL.md first line ("You are **Ops**…")
  - SOUL.md + IDENTITY.md joined with "\n\n---\n\n" (manifest prompt_files order)
quick_prompts: 6 (locale-resolved zh):
  K8s Pod 排障 / Docker 容器诊断 / 磁盘与内存排查 / 服务与端口检查 / 网络连通诊断 / 写运维脚本
  sample prompt: "请给出排查 Kubernetes Pod 异常的 kubectl 命令，包括查看状态、事件、日志和进入容器调试。"
```

`Accept-Language: en` on the same endpoint resolves `label: Ops · SRE
Engineer` and English quick-prompt titles (locale rule: exact → prefix → en →
first key). Unknown id → `404 {"error":{"code":1003,"message":"expert not found"}}`.

## 3. Instantiate `ops-engineer` → 201 + persisted agent

`POST /api/v1/experts/ops-engineer/instantiate {}` → **201**:

```
agent: 00855c9a-47d7-454f-9c50-950265083833  name: 运维工程师 Ops (locale label)
pending_skills: []   skill_install_ids: []   (no bundled skills)
```

`GET /api/v1/agents/00855c9a…` cross-checks the persisted record:

```
config.expert_source: {"expert_id":"ops-engineer","source":"builtin"}
config.system_prompt: 3461 chars — byte-identical join of SOUL.md + IDENTITY.md
                      (same separator as the §2 preview; ends with the IDENTITY
                      autopilot paragraph)
config.agent_mode: smart-reasoning | kb_selection_mode: all | web_search_enabled: true
                    (manifest agent_config overrides applied)
config.question_suggestions.starters: mode=curated, enabled=true, count=6,
                    6 zh items — the locale-resolved quick-prompt prompt texts
avatar: terminal (manifest icon_name)
```

`agent_name` override: `{"agent_name":"My Ops Agent"}` → 201 with
`name: My Ops Agent` (label skipped). A subsequent ordinary
`PUT /api/v1/agents/:id` (setting `config.model_id`) keeps `expert_source`,
the starters and the system prompt intact — provenance survives agent edits.

## 4. Persona in the live LLM request (chat turn)

tRPC session `POST /api/v1/sessions {"engine_type":"trpc"}` → 201. First chat
attempt hit the expected mock-env gap (`rerank model is not configured` —
`kb_selection_mode: all` + knowledge_search needs a reranker); resolved the
way M1 did, by setting `kb_selection_mode: "none"` on the agent via the
ordinary update API (no code change; documented as a mock-env adaptation).

`POST /api/v1/agent-chat/<session>` `{"query":"Say hello in one short
sentence.","agent_enabled":true,"agent_id":00855c9a…}` → SSE 200, one
`"response_type":"agent_query"` completion, and the raw dump
`llm_stream_mock-stream-model_20260919T223205.727659000.jsonl`:

- **line 1 request `data.messages[0]` role=system, 3461 chars — byte-identical
  to the agent's `config.system_prompt`** (verified programmatically), i.e.
  the expert persona (SOUL + IDENTITY) rides as the system message verbatim;
- lines 2–4 are three streamed `chat.completion.chunk` objects from the mock
  model — the turn genuinely round-tripped an LLM.

## 5. Skill resolution: stock-assistant

### 5.1 Without `sandbox_config_id`

`POST /api/v1/experts/stock-assistant/instantiate {}` → **201**:

```
pending_skills:   ["stock-info"]     (manifest slug — operator-facing reference)
skill_install_ids: []                (no sandbox → nothing to install onto)
agent config.selected_skills: ["akshare-data"]   ← SKILL.md frontmatter name,
                                                   NOT the slug
agent config.skills_selection_mode: "selected"
config.expert_source: {"expert_id":"stock-assistant","source":"builtin"}
```

The frontmatter-vs-slug split is exactly the resolver contract: `Selected`
carries the install name the runtime matches `AllowedSkills` against,
`Pending` carries the manifest slug.

### 5.2 With `sandbox_config_id` (real async install bridge)

`POST /api/v1/sandbox-configs {"name":"e2e-docker","config":{"sandbox_type":"docker","docker":{"image":"wechatopenai/weknora-sandbox:main"}}}` →
201 (`17f9010f-…`) after one restart with `WEKNORA_SANDBOX_DOCKER_ENABLED=true`
(creating a docker config is refused while the backend is opt-out — by design).

Two earlier instantiate attempts exercised the **degrade-never-fail contract
live**: the install bridge was genuinely invoked (zip → `InstallSkill` → row
created) but the bundle store failed on this host's read-only `/data`
(`mkdir /data: read-only file system`, the dev default `LOCAL_STORAGE_BASE_DIR`);
the resolver logged `[expert] install bundled skill "stock-info" … failed`,
kept the skill pending, and the agent still came back **201**. After pointing
`LOCAL_STORAGE_BASE_DIR` at scratch, the happy path:

```
POST /api/v1/experts/stock-assistant/instantiate {"sandbox_config_id":"17f9010f…"} → 201
pending_skills:    ["stock-info"]
skill_install_ids: ["3122a10b-4ab0-4edf-9a74-35c659810246"]   ← real async install started
selected_skills:   ["akshare-data"]
config.sandbox_config_id: "17f9010f…" stamped on the agent
```

Corroboration: server log `Instantiated expert "stock-assistant" … (skills: 1
selected, 1 install(s) started, 1 pending)`; the zipped bundled skill was
stored under the file service (`files/1/exports/<uuid>….zip`); and
`GET /api/v1/sandbox-configs/17f9010f…/skills` lists the tenant skill row
`akshare-data` (id `3122a10b…`) with `status: installing`, `enabled: true` —
the async install was queued through the tenant skill service. (The server was
stopped immediately after capture so the background docker install could not
pull the multi-GB sandbox image; install completion is the sandbox
subsystem's contract, not the experts bridge's.)

## 6. Error taxonomy (all observed on the wire)

| Case | Result |
|---|---|
| `GET /experts` unauthenticated | `401 Unauthorized: missing authentication` |
| `GET /experts/no-such-expert` | `404 {"code":1003,"message":"expert not found"}` |
| `POST /experts/ops-engineer/instantiate {"bogus_field":1}` | `400 invalid body: json: unknown field "bogus_field"` (strict decoding) |
| `agent_name` > 255 chars | `400 agent_name exceeds 255 characters` (handler cap; unit-tested) |
| Instantiate on unknown expert | 404, same sentinel as the GET |

## 7. Agents list shows instantiated agents

`GET /api/v1/agents?page=1&page_size=20` → 4 builtin agents + 6
expert-instantiated custom agents, each carrying
`config.expert_source.expert_id` (`ops-engineer` ×2, `stock-assistant` ×4 —
one per phase above) and, where given, `config.sandbox_config_id`.

## 8. Frontend

- `pnpm --filter @weknora/web build` (= `tsc -b && vite build`) → **exit 0**,
  `✓ built in 11.78s`; only the pre-existing Rollup chunk-size advisories
  (largest `index-*.js` 2.7 MB / 697 kB gzip). `ExpertsPage-D-qFFcHP.js` is in
  `dist/assets/` (lazy route chunk).
- `apps/web/src/experts/ExpertsPage.test.tsx` → **6/6 pass** (cards + skill
  chips + mbti badge, error/empty states, detail drawer persona markdown +
  quick prompts, instantiate toast with pending-skill count + editor
  deep-link, custom agent name, failure toast). Manual browser smoke skipped
  (dev-env cost); the jsdom suite pins the page contract.
- Carried fix from Task 7 review: `apps/web/src/routes.tsx` `resolveRoute`
  platform allowlist now includes `'/platform/experts'` (one line, matching
  sibling entries) + one assertion in `routes.test.ts`;
  `routes.test.ts`/`router.test.tsx` (which already pinned the TanStack route
  tree entry) green. Web unit suite has 38 pre-existing failures in
  `src/documents/KnowledgeDocumentDetailPage.test.tsx` — verified identical at
  base `0f711ee7` via a stash run; unrelated to this change.

## 9. Go test gates

```
go test ./internal/agent/experts/... ./internal/application/service/ ./internal/handler/ ./internal/router/
```

| Package | Result | Notes |
|---|---|---|
| `internal/agent/experts` | **ok** | scanner/zip/pilot-import tests all green |
| `internal/application/service` | FAIL (88) | every failure is the **pre-existing duplicate sqlite migration 000079** (`failed to open source … duplicate migration file: 000079_message_feedback.down.sql`) breaking the shared test helpers (`agent_run_graph_test.go:51`, `craft_snapshot_test.go:210`); same breakage as the runtime workaround in Environment. Verified pre-existing at base via a stash run of representative tests (`TestCancelReleasesSessionSlotOnRealStore`, `TestInteractionStoreRegisterIsIdempotentPerArguments` — identical failure, same root cause). |
| `internal/handler` | FAIL (2) | `TestExecutionTargetRegistrationGinSQLiteLifecycle`, `TestExecutionRegistrationChallengeRequiresTenantAndOwner` — the two execution-registration reds M1's evidence already documented (schema drift, base commit); both re-verified failing at base via stash. |
| `internal/router` | FAIL (1) | `TestExecutionRegistrationRoutesUseRealRegistrationTargetRevokeLifecycle` (201-vs-404, same execution-registration family); verified failing at base via stash. |
| web build | **ok** | exit 0 (§8) |

Expert-relevant coverage is green everywhere it exists:

- `internal/handler`: `TestExpertListSummaries`, `TestExpertListLocaleResolution`,
  `TestExpertDetail`, `TestExpertInstantiateHappy`, `TestExpertInstantiateValidation`,
  `TestExpertInstantiateServiceError` — all PASS.
- `internal/router`: `TestExpertRoutesRegisterBesideAgentAndPersonaRoutes` — PASS.
- `internal/application/service`: `TestBuildAgentFromExpert` (+8 subtests incl.
  no-mutation of shared experts), `TestInstantiateExpert` (+4 subtests),
  `TestExpertServiceCatalog`, and the full `TestResolveExpertSkills*` family
  (installed-only, install-kicked-off, no-sandbox, installer-error degrade,
  ready/enabled filter, missing-dir fallback, lister-error degrade, no-skills) —
  all PASS.

## Notes and gaps

1. **Duplicate sqlite migration 000079 (main breakage).** Both the runtime E2E
   (needs migrations to create tables) and 88 service tests are blocked by it.
   This verification worked around it in scratch (renumber to 000082); the fix
   belongs on main (renumber one pair properly). Not fixed here per the brief's
   pre-existing-red policy.
2. **Skill install completion not awaited.** The E2E proves the bridge starts
   a real async install (install id returned, row `installing`, bundle stored);
   it does not wait for the sandboxed install to finish (would pull a multi-GB
   image and drive a root-shell install session). Install-completion semantics
   are covered by the tenant-skill subsystem's own tests.
3. **Mock-env adaptations (documented, no code changed):** `SSRF_WHITELIST`
   for the raw-IP mock base URL; `kb_selection_mode: "none"` on the chat agent
   (no rerank model in the mock env); `LOCAL_STORAGE_BASE_DIR` + docker enable
   env for §5.2; scratch migrations copy for §Environment.
4. **Locale of the durable path.** As in M1, the persona renders from the
   persisted `config.system_prompt` (already locale-resolved at instantiate
   time, zh for this run) rather than re-resolving per turn — so no
   durable-locale divergence applies to experts: the prompt is baked at
   instantiation locale by design.
5. **No secrets committed.** JWTs, passwords and scratch artifacts lived only
   under the git-ignored scratch root and were deleted after this document was
   written; the committed `config/config.yaml`/`builtin_models.yaml` were used
   unmodified via symlink.
