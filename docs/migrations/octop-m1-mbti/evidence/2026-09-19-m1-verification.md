# M1 MBTI Persona — End-to-End Verification (2026-09-19)

Branch `octop-m1-mbti`, HEAD `70f6f4d6` (feat(web): 28-question MBTI test modal with apply-to-agent).
Verifier: Task 10 of the M1 MBTI persona milestone (spec §4 of
`docs/superpowers/specs/2026-09-19-octop-capabilities-migration-design.md`).

## Environment

| Item | Value |
|---|---|
| Server | source-run: `go run -tags sqlite_fts5 ./cmd/server` from the worktree root (later restarted from a prebuilt binary of the same commit after adding `SSRF_WHITELIST`, see below) |
| Port | `127.0.0.1:18089` via `SERVER_PORT=18089 SERVER_HOST=127.0.0.1` (viper env override of `server.port`; 8080/8082/8083 on this machine are occupied by other local environments) |
| Config | repo `config/config.yaml` (worktree copy, unmodified — carries no credentials) |
| DB | `DB_DRIVER=sqlite`, `DB_PATH=.superpowers/sdd/2026-09-19-mbti-persona-m1/e2e/weknora.db` (git-ignored scratch; migrations ran 0 → 78 on first boot) |
| Redis | none (`No REDIS_ADDR configured, Redis disabled (Lite mode)`) — in-process stream manager / task executor |
| Retrieval | `RETRIEVE_DRIVER=sqlite` (`Register sqlite retrieve engine success`) |
| Auth | self-serve registration was open (`GET /api/v1/auth/config` → `registration_mode: "self_serve"`); registered `mbti-e2e@local.dev`, then `POST /api/v1/auth/login` → JWT (272 chars). No admin bootstrap needed. |
| LLM | the repo's committed dev default `config/builtin_models.yaml` → `builtin-llm-mock` (LAN mock OpenAI server, already configured; no secrets added). Needed `SSRF_WHITELIST=<mock host>` because the mock's base URL is a raw IP. |
| LLM request dump | `WEKNORA_LLM_STREAM_RAW_DUMP_DIR=<scratch>/llm-dump` — writes one JSONL per stream, line 1 = the request wrapper (messages incl. system prompt) |

Scratch directory (all artifacts under it, not committed):
`.superpowers/sdd/2026-09-19-mbti-persona-m1/e2e/`.

## 1. Catalog: `GET /api/v1/mbti/types` (16 types)

```bash
TOKEN=$(cat e2e/out/token.txt)
curl -s http://127.0.0.1:18089/api/v1/mbti/types -H "Authorization: Bearer $TOKEN"
```

Observed (saved `e2e/out/03-types.json`):

```
count: 16
codes: ['INTJ', 'INTP', 'ENTJ', 'ENTP', 'INFJ', 'INFP', 'ENFJ', 'ENFP',
        'ISTJ', 'ISFJ', 'ESTJ', 'ESFJ', 'ISTP', 'ISFP', 'ESTP', 'ESFP']
first: INTJ 建筑师 / Architect color: #6366F1
```

Negative/auth posture, same session:

- `GET /api/v1/mbti/types` without token → `401 {"error":"Unauthorized: missing authentication"}`
- `GET /api/v1/mbti/types/ZZZZ` → `404 {"error":{"code":1003,"message":"mbti type not found"}}`
- `GET /api/v1/mbti/types/intj` (lowercase) → `200` with `code: INTJ`, `name_zh: 建筑师`, dimensions `{ei: I, sn: N, tf: T, jp: J}` — case normalization confirmed.

## 2. Test: questions + submit (28 × A → ESTJ)

`GET /api/v1/mbti/test/questions` → 28 items, ids 1–28, each carrying `dimension`, `a_pole`, `b_pole`, and both zh/en prompt variants (`e2e/out/05-questions.json`):

```
questions: 28
sample q1: {'id': 1, 'dimension': 'EI', 'a_pole': 'E', 'b_pole': 'I'}
zh: 在社交活动后，你通常感到： | en: After a social event, you usua…
```

`POST /api/v1/mbti/test/submit` with all 28 answers = `"A"` (`e2e/out/06-submit-allA.json`):

```
code: ESTJ | dims: {'ei': ('E', 85), 'jp': ('J', 85), 'sn': ('S', 85), 'tf': ('T', 85)}
profile name: 总经理 / Executive
```

All-first-poles `ESTJ` at the 85 % clamp — matches the Octop scoring contract (≥20 valid answers, 50–85 % band). Too-few-answers guard also exercised:

```
{"answers":{"1":"A","2":"B"}} → 400 {"message":"persona: too few answers (2/28), at least 20 required"}
```

## 3. Preview rendering (locale-aware)

`GET /api/v1/mbti/preview/INTJ` with `Accept-Language: zh-CN`:

```
# Persona: INTJ — 建筑师

你是 Agent，一名与 the user 协作的 AI 助手。

富有想象力的战略家，对一切都有计划.

特质：战略家、独立、深邃、追求完美。

## 行为风格

- **回答风格：**简洁有条理，喜欢深度而非广度
- **闲聊：**不爱闲聊，话题很快拐向观点和洞察
...
```

Same endpoint with `Accept-Language: en` renders the English variant (`# Persona: INTJ — Architect` / `You are Agent, an AI assistant…` / `## Behavior`).

## 4. Agent persona: PUT (valid / invalid / unknown)

Agent created with `POST /api/v1/agents` → id `63a69c7e-…` (`e2e/out/07-create-agent.json`), `config.persona_mbti` initially absent.

Valid apply (note lowercase `intj` on the wire, normalized server-side):

```
PUT /api/v1/agents/63a69c7e…/persona  {"code":"intj","style":"Answer with concise bullet points first."}
→ 200 {"data":{"persona_mbti":"INTJ","persona_style":"Answer with concise bullet points first."},"success":true}
```

Invalid code and unknown agent:

```
PUT …/persona {"code":"XXXX"}            → 400 {"message":"unknown mbti code: XXXX"}
PUT /api/v1/agents/00000000-0000-…/persona {"code":"INTJ"} → 404 {"message":"agent not found"}
```

Persistence cross-checked through the plain agent read API:

```
GET /api/v1/agents/63a69c7e…
→ persona_mbti: 'INTJ' | persona_style: 'Answer with concise bullet points first.'
```

## 5. Persona segment in the live system prompt (chat turn)

The chat path was exercised for real: session `POST /api/v1/sessions` (engine `builtin`), then
`POST /api/v1/agent-chat/:session_id` with `{"query":"Say hello in one short sentence.","agent_enabled":true,"agent_id":<the INTJ agent>}`.
The first attempt failed with `chat model is not configured: please set model_id on agent …` —
expected for a bare agent — so `model_id: builtin-llm-mock` was set via the ordinary
`PUT /api/v1/agents/:id` (persona fields intact). The second attempt failed on the SSRF guard
(`direct IP address access is not allowed`) because the committed mock model's base URL is a raw
LAN IP; resolved by restarting the instance with `SSRF_WHITELIST=<mock host IP>` (env-only, no
config change, no secrets).

With the whitelist in place the turn completed and `WEKNORA_LLM_STREAM_RAW_DUMP_DIR` captured the
outgoing LLM request. Two further routing facts had to be satisfied first (all discovered from the
SSE error envelopes, no code changes):

1. custom agents answer on the **tRPC engine**: `agent_mode` must be `smart-reasoning` and the
   session must be created with `engine_type: "trpc"` (otherwise the request funnels into the
   KnowledgeQA conversation pipeline, whose prompt correctly carries no persona segment);
2. `allowed_tools` defaulting to `knowledge_search` requires a rerank model — avoided by setting
   `kb_selection_mode: "none"` (no KBs on this agent);
3. durable tRPC admission is opt-in: restart with
   `WEKNORA_AGENT_RECOVERY_ENABLED=true WEKNORA_AGENT_RECOVERY_ADMISSION_ENABLED=true`.

### 5.1 With persona applied (INTJ)

`POST /api/v1/agent-chat/<trpc-session>` `{"query":"Say hello in one short sentence.","agent_enabled":true,"agent_id":<INTJ agent>}`
→ SSE `event:message … "response_type":"agent_query"`, durable run admitted, LLM streamed
(`[LLM Stream Raw Dump] writing packets to …llm_stream_mock-stream-model_20260919T175838….jsonl`).
Line 1 of that dump is the request wrapper; its `messages[0]` (role `system`) is, verbatim:

```
# Persona: INTJ — 建筑师

你是 the assistant，一名与 the user 协作的 AI 助手。

富有想象力的战略家，对一切都有计划.

特质：战略家、独立、深邃、追求完美。

## 行为风格

- **回答风格：**简洁有条理，喜欢深度而非广度
- **闲聊：**不爱闲聊，话题很快拐向观点和洞察
- **冲突：**冷静分析，直击问题根源
- **创造力：**系统级思考，构建优雅的长期方案
- **情绪：**简短回应情绪，然后给出务实建议
- **计划：**制定详尽的战略路线图，附带应急预案

Answer with concise bullet points first.
```

Checks: system message **starts with `# Persona: INTJ`**; the agent's `persona_style` free text is
appended; no `---` separator follows because this agent has an empty base `system_prompt` (the
"empty base prompt → segment alone" branch from Task 5). The dump then contains three streamed
`chat.completion.chunk` lines from the mock model — the turn genuinely round-tripped an LLM.
Locale note: the segment rendered in Chinese (zh-CN deployment default) because the durable worker
executes outside the HTTP request context, so `Accept-Language: en` is not propagated and
`LanguageFromContextOrDefault` falls back to `WEKNORA_LANGUAGE`/`zh-CN` — the renderer's designed
default fallback (see Notes and gaps).

### 5.2 After DELETE (segment gone)

`DELETE /api/v1/agents/:id/persona` → `200 {"data":{"persona_mbti":"","persona_style":""},"success":true}`;
`GET /api/v1/agents/:id` now shows both fields absent; a second DELETE is idempotent (`200`).
A second chat turn (`"query":"Say goodbye in one short sentence."`) on the same session produced
dump `llm_stream_mock-stream-model_20260919T180158….jsonl` whose request contains **no system
message at all** (first message is `user` history): no `# Persona`, no INTJ block, no style text —
with the persona removed and an empty base prompt the assembled system prompt is empty. Negative
check passes.

## 6. DELETE persona

Covered in §5.2 (same sequence): DELETE → 200 with cleared fields; GET shows `persona_mbti`/`persona_style`
gone; live chat turn no longer carries the persona segment in the outbound LLM request.

## 7. Frontend build

```bash
pnpm --filter @weknora/web build      # = tsc -b && vite build
```

Exit code 0 — `✓ built in 5.75s`; only Rollup chunk-size warnings (pre-existing advisory, largest
chunk `index-*.js` 2.7 MB / 692 kB gzip). The editor Personalization section
(`apps/web/src/agents/PersonaSection.tsx`) and test modal (`MbtiTestModal.tsx`) both compile and
bundle into `dist/assets/AgentsPage-*.js` (160 kB). Browser-level editor flow (grid pick → test →
apply → save → reopen) was not re-driven in this round: Task 8/9 review evidence and component
tests already pin that surface; this round's frontend gate is the production build.

## 8. Go test gates

```bash
go test ./internal/agent/persona/... ./internal/types/... ./internal/handler/...
pnpm --filter @weknora/web build        # = tsc -b && vite build → ✓ built in 5.75s, exit 0
```

Results (per package, machine under heavy unrelated load — see Environment):

| Package | Result | Notes |
|---|---|---|
| `internal/agent/persona` | **ok** | all M1 data/scoring/render tests green |
| `internal/types` | FAIL (1) | `TestEveryContextKeyDeclaresACloneDecision` — known pre-existing red on main (`CredentialVersion`, lands before the branch base `e42eea88`); documented in the SDD ledger |
| `internal/handler` | FAIL (2) | `TestExecutionTargetRegistrationGinSQLiteLifecycle`, `TestExecutionRegistrationChallengeRequiresTenantAndOwner` — stale test SQLite schema (`execution_targets` has no column `usage_binding_json`, INSERT returns rows:0); code last touched by `d7657d26` which is **in the base**; no branch commit touches these files |
| `internal/handler/dto` | ok | |
| `internal/handler/session` | HANG | `TestWorkbenchDecisionHTTPResumesDurableApprovalAndScopesIdentity` deadlocks at `id := <-pending` (workbench_commands_test.go:134) because the test's local `workbenchserviceInteractionRow` schema lacks `external_pending_id`/`credential_version` that production `interaction.go` (last changed `d2e18040`, **in the base**) now writes — the interaction INSERT returns rows:0 so the approval event never fires. Deterministic (reproduces in isolation with `-run`, 0 % CPU). The hang stalls every parallel test behind it and trips the default 10 m package timeout. |
| web build | **ok** | exit 0 |

M1-relevant coverage within `internal/handler` (persona routes/handler tests, including the
wildcard-coexistence regression) all passed — the only failures listed above are pre-existing
schema-drift reds inherited from the base commit; per the task brief they are excluded and not
fixed here.

## Notes and gaps

1. **Durable-path locale.** The persona segment rendered zh-CN in the live dump despite the curl
   sending `Accept-Language: en`: the durable tRPC worker executes outside the HTTP request
   context, so the request locale is not in the admission snapshot and
   `LanguageFromContextOrDefault` falls back to the deployment default (`WEKNORA_LANGUAGE` unset →
   `zh-CN`). This is the renderer's designed default fallback, not a regression introduced by M1 —
   but if en-locale persona rendering on the durable engine matters operationally, the admission
   snapshot should capture the request language (follow-up candidate; handler-level
   `GET /mbti/preview` honors `Accept-Language` and was verified for both locales in §3).
2. **Agent-name slot.** The persona segment's `You are the assistant` uses the assembly fallback
   (`personaAgentNameFallback`): the custom agent's display name is not threaded into capability
   assembly (known M1 deferral, recorded in Task 5's report).
3. **No substitution needed.** The brief allowed substituting the live chat turn with service-level
   test evidence if no LLM was configured; the repo's committed dev mock model made the real path
   possible, so the primary evidence is the actual outbound LLM request dump.
4. Scratch artifacts (DB, server logs, curl outputs, both JSONL dumps) lived under
   `.superpowers/sdd/2026-09-19-mbti-persona-m1/e2e/` and were deleted after this document was
   written; nothing scratch is committed.
5. **Pre-existing red gates (excluded per brief).** Three, all present at the branch base
   `e42eea88` and untouched by every M1 commit: (a) types `TestEveryContextKeyDeclaresACloneDecision`
   (known-red, SDD ledger); (b) handler execution-registration tests — stale test schema missing
   `execution_targets.usage_binding_json`; (c) session
   `TestWorkbenchDecisionHTTPResumesDurableApprovalAndScopesIdentity` — same stale-schema pattern
   for `workbench_interactions.external_pending_id`, which deadlocks the whole session package.
   (b) and (c) look like one underlying drift: W22–W24 model columns added without updating the
   tests' hand-rolled `AutoMigrate` row structs. Worth a dedicated fix task on main.
6. **Machine load.** This verification ran on a host with load average >200 from unrelated
   processes; the Go link step alone took ~10 minutes. This affected wall-clock only — every
   pass/fail above is deterministic and load-independent (failures are schema assertions and a
   0 %-CPU deadlock, not timing flakes).

