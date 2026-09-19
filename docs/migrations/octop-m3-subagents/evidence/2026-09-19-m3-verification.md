# M3 Sub-agents — End-to-End Verification (2026-09-19)

Branch `octop-m3-subagents`, HEAD `d184ad9f` (feat(web): agent editor
subagents management tab) — the full M3 stack: catalog package + 271-slug
role library (`config/subagents/library`), `tenant_subagents` table +
repository + agent config field, budgeted tRPC sub-run executor with event
attribution + sanitization, `subagent_delegate` tool (conditional
registration, strict args, intersected whitelist), catalog/install APIs,
api-client module, editor tab. Verifier: Task 9 of the M3 sub-agents
milestone. Everything in §1–§6 ran against a real server process with a real
SQLite database; no mocks sit between curl and the handler. The chat LLM was
a scripted OpenAI-compatible mock (see Environment) — the model side is
scripted, the server side is real.

## Environment

| Item | Value |
|---|---|
| Server | prebuilt binary of HEAD: `go build -tags sqlite_fts5 ./cmd/server`, run from a scratch CWD with `config` and `migrations` symlinked to the repo (both unmodified) |
| Port | `127.0.0.1:18092` via `SERVER_PORT`/`SERVER_HOST` |
| DB | `DB_DRIVER=sqlite`, `DB_PATH=<scratch>/weknora.db`; migrations ran **0 → 84 cleanly on first boot** — the M2 duplicate-`000079` breakage is fixed on main (`5c4d990e` renumbered `000079_commercial_reservations_owner` → `000082`, and that fix is in this branch's base), so **no M2-style migrations workaround was needed**; `000084_tenant_subagents` is part of the clean run |
| Redis | none (Lite mode) |
| Retrieval | `RETRIEVE_DRIVER=sqlite` |
| Auth | self-serve registration; registered `subagents-e2e@local.dev`, `POST /api/v1/auth/login` → JWT (279 chars; short TTL — re-login between phases) |
| LLM | **scripted mock** `e2e-scripted-mock`: a python OpenAI-compatible server written for this E2E (scratch only, not committed) at `127.0.0.1:18093`, registered through `BUILTIN_MODELS_CONFIG=<scratch>/builtin_models.yaml` (scratch copy of the repo's entry + the scripted one; the repo's `config/` stayed untouched). This is what made **scripted tool-calling** feasible: the mock dispatches on each request's `tools`/message roles and emits either a `subagent_delegate` tool call (main run round 1), the role's final summary (sub-run), or a final answer quoting the tool result (main run round 2). It also serves `/resource` (a fixed page) for `web_fetch`. |
| SSRF | `SSRF_WHITELIST=127.0.0.1,192.168.3.30` (raw-IP mock base URLs) |
| LLM request dump | `WEKNORA_LLM_STREAM_RAW_DUMP_DIR=<scratch>/llm-dump` — one JSONL per streaming call; line 1 = the request wrapper (`data.messages`, `data.tools`), later lines = streamed chunks |
| Durable chat | `WEKNORA_AGENT_RECOVERY_ENABLED=true WEKNORA_AGENT_RECOVERY_ADMISSION_ENABLED=true` |
| Storage | `LOCAL_STORAGE_BASE_DIR=<scratch>/files` |

Scratch root (nothing committed): `.superpowers/sdd/2026-09-19-subagents-m3/e2e/`.

## 1. Catalog: `GET /api/v1/subagent-catalog` → 19 divisions, 271 entries

```
total: 271   entries: 271   divisions: 19 (divisions.json order)
  academic 6   design 9   engineering 42   finance 9   game-development 20
  gis 13   marketing 43   paid-media 7   product 5   project-management 7
  sales 9   security 10   spatial-computing 6   specialized 61   support 7
  testing 9   hr 2   legal 2   supply-chain 4        (sum = 271)
product-manager: {"slug":"product-manager","division":"product",
                  "name_zh":"产品经理","name_en":"Product Manager",
                  "emoji":"🧭","color":"blue","installed":false}
any installed = false (fresh tenant — no rows yet)
```

## 2. Detail: `GET /api/v1/subagent-catalog/product-manager`

```
name_zh: 产品经理 | name_en: Product Manager | emoji 🧭 | color blue | vibe 交付正确的东⻄…
body_zh: 10285 chars, starts "\n# 🧭 产品经理代理 … 你是 **Alex**，⼀位经验丰富的产品经理…"
body_en: 21956 chars (present-or-empty contract; both non-empty here)
tools_raw: "WebFetch, WebSearch, Read, Write, Edit"     ← contains WebFetch
installed: false → true after §3
```

## 3. Install on an agent → config + tenant row + catalog flag

Agent created with `POST /api/v1/agents` (minimal config:
`agent_mode: smart-reasoning`, `model_id: e2e-scripted-mock`,
`kb_selection_mode: none`) → id `29713130-8a46-4165-af66-3b215cdcfb23`,
`config.subagents` initially absent.

```
POST /agents/:id/subagents {"slug":"product-manager","locale":"zh-CN"}
  → 200 {"subagents":["product-manager"]}
GET  /agents/:id/subagents           → {"subagents":["product-manager"]}   (persisted)
GET  /agents/:id                     → config.subagents = ["product-manager"]
POST … {"slug":"product-manager"}    → 200 {"subagents":["product-manager"]} (idempotent, same list)
GET  /subagent-catalog/product-manager → installed = true
tenant_subagents row: slug=product-manager locale=zh division=product
                      source=builtin content=10548 bytes (role markdown verbatim)
```

## 4. Remove → config cleared, tenant row kept

```
DELETE /agents/:id/subagents/product-manager → 200 {"subagents":[]}
GET   /agents/:id/subagents                  → {"subagents":[]}
GET   /agents/:id                            → config.subagents absent
GET   /subagent-catalog/product-manager      → installed = true   (row kept)
sqlite tenant_subagents (deleted_at null)    → product-manager | zh
```

## 5. Error taxonomy (all on the wire)

| Case | Result |
|---|---|
| `POST /agents/:id/subagents {"slug":"no-such-role"}` | `404 {"code":1003,"message":"subagent not found"}` |
| `GET /subagent-catalog/no-such-role` | `404 {"code":1003,"message":"subagent not found"}` |
| `POST …/subagents {"slug":"product-manager","bogus_field":1}` | `400 invalid body: json: unknown field "bogus_field"` (strict decoding) |
| `POST …/subagents {}` | `400 slug required` |
| `GET /subagent-catalog` without token | `401 Unauthorized: missing authentication` |
| `POST …/subagents` without token | `401 Unauthorized: missing authentication` |

(product-manager was then re-installed for §6.)

## 6. Delegation live turn — the core

Scripted-mock dispatch made the model emit a real `subagent_delegate` tool
call, and the server executed the REAL delegate tool and the REAL budgeted
tRPC sub-run. Query: "Should we ship the onboarding checklist or the
referral program first? Please delegate this to a product specialist."
(tRPC session `20ff03c3…`, durable run `a664f8da`, status `succeeded`).

**Registration (conditional).** Server log:

```
subagent_delegate registered for session 20ff03c3… with specialists [product-manager]
Registered 1 tools
```

**Tools array in the dump (binding "cheap and strongly evidential" check).**
Dump 1 line 1 (`data.tools`) contains exactly one tool — `subagent_delegate`
— whose description embeds the run's slug list verbatim:

```
"Delegate a self-contained subtask to one of this agent's installed specialist
sub-agents. … Available specialists (use the slug exactly): product-manager. …"
```

**Round trip.** Dump 1 shows the streamed assistant chunk
`tool_calls[0].function = subagent_delegate({slug: "product-manager", goal:
"Translate this request into a product call: Should we ship …"})`; the server
journal then records `tool_dispatched` → `tool_result` for
`call_id=call_main_delegate`, and `agent_tool_calls` carries the full args +
result (`status=succeeded`, output `[no tools available]\n\n
E2E-SUBAGENT-SUMMARY-MARKER 产品经理 Alex 已完成评估…`).

**The sub-run is real.** The server log shows the delegate executing
(`[PIPELINE] stage=AgentTool action=execute_start args={"slug":
"product-manager","goal":…} tool="subagent_delegate"`), then a
**non-streaming LLM call from the executor** (`chatWithRawHTTP`,
endpoint `http://127.0.0.1:18093/v1/chat/completions`) whose request's
system message is the installed role's markdown body verbatim —
`"\n# 🧭 产品经理代理\n\n## 🧠 身份与记忆\n\n你是 **Alex**，⼀位经验丰富的产品经理…"`
— i.e. the tenant-installed copy is the sub-run system prompt, on a fresh
session id, single model round (toolless role → empty intersection is
first-class; the tool result carries the `[no tools available]` note).

**Summary returns to the model.** Dump 2 (main run round 2) contains
`messages` = [user, user, assistant(tool_calls), **tool**] where the tool
message content is the delegate's summary (`[no tools available]\n\n
E2E-SUBAGENT-SUMMARY-MARKER 产品经理 Alex …`), and the streamed final
answer quotes it: "Delegation complete. The specialist returned: <[no tools
available]\n\nE2E-SUBAGENT-SUMMARY-MARKER …> — adopted as the final plan."
The same text is persisted as the assistant message in the `messages` table,
and `agent_run_events` shows `run_started → tool_dispatched → tool_result →
run_completed`.

**Unknown slug (negative).** Same agent, query containing "UNKNOWN": the
scripted model delegated to slug `no-such-role`; `agent_tool_calls` records
`status=failed`,
`result={"success":false,"output":"","error":"subagent not installed:
no-such-role"}` (the model-facing sentinel), the run still completed
succeeded, and the round-2 dump shows the assistant/tool messages.
Honest caveat: on the durable engine the tool-message content rendered to
the model is `result.Result.Output` (`internal/agent/trpc/graph.go:254`) —
empty for a failed tool — so the *error text* is visible in the journal,
`agent_tool_calls.result.error` and the tool events, but not in the
tool-message content. This is pre-existing engine-wide rendering (any failed
tool renders the same), not M3 behavior; inside sub-runs the executor's
`renderSubagentToolResult` DOES fold errors into the model-visible output.

**Conditional-registration negative.** A second, identical agent WITHOUT
subagents installed: chat turn on a fresh tRPC session → log
`Registered 0 tools` (no `subagent_delegate registered` line at all) and the
dump's `data.tools` is `[]` — the tool is absent exactly because
`config.subagents` is empty. Same config shape as §6's agent minus
subagents; the only delta in the offered tool set is `subagent_delegate`.

**SUBSTITUTION (labeled): live `subagent:<slug>:<tool>` attributed events.**
The sub-run's attributed tool events could not be witnessed on this live
instance: the only role tools that can register without a sandbox are the
web tools, and enabling them crashes the durable run BEFORE any model call
with a pre-existing bug — `web_fetch`'s generated schema emits
`"type":["null","array"]` for `items`, which
`trpcagent.DeclarationTools` cannot unmarshal
(`durable run … tool declarations: tool web_fetch input schema: json:
cannot unmarshal array into Go struct field Schema.properties.type of type
string`; witnessed live in run `d7b33884` and reproduced programmatically
against the schema). Root cause is pre-existing: `web_fetch.go`,
`engine.go`, `utils/json.go` are untouched by all nine M3 commits (last
touched by main commits inside the branch base), and any web-tools agent on
the durable engine hits it with or without M3. Evidence supplied instead:
`internal/agent/subagents/executor_test.go` runs the REAL tRPC graph
(fake model, real tool instances): `TestExecuteToolLoopTwoRounds` (two-round
tool loop, attributed `subagent:<slug>:<tool>` events),
`TestExecuteAttachesToolExecContext`, `TestExecuteSanitizesEmittedEvents`,
`TestExecuteRoundOverrun`/`TestExecuteCharOverrun` (budget overrun is a
partial result, never an error) — all green in §8's gate run.

**SSE note.** On this durable setup the SSE stream delivered
`agent_query` + `session_title` but did not relay the run's completion
within the client's window; the durable transcript (`agent_run_events`,
`agent_tool_calls`, `messages`) plus the two raw dumps are the completion
witnesses. Two earlier runs failed for scratch-harness reasons only (my mock
initially emitted SSE without the `data:` prefix, then a double-prefixed
`[DONE]` frame) and are not evidence of product behavior.

## 7. Frontend

- `pnpm --filter @weknora/web build` (= `tsc -b && vite build`) → **exit 0**,
  `✓ built in 11.55s`; only the pre-existing Rollup chunk-size advisories.
  `AgentsPage-MLZZkm6p.js` present in `dist/assets/`.
- `apps/web/src/agents/agent-editor.test.tsx` (node test runner) → **19/19
  pass**, including the four Task-8 subagents subtests: "subagents section
  renders the installed list, catalog cards, division chips and zh names",
  "subagents nav item is gated by agent mode like tools/skills", "subagents
  empty installed list shows the delegation-off hint", "installing a catalog
  role patches config.subagents from the response and reaches the save
  payload". Manual browser smoke skipped (dev-env cost; jsdom suite pins the
  tab contract).
- `packages/api-client` suite: **252/252 pass** (the subagents module has no
  dedicated test file; its wire contract is pinned by this document's live
  §1–§5 checks and consumed by the editor tests).

## 8. Go test gates

```
go test ./internal/agent/subagents/... ./internal/application/service/ \
        ./internal/handler/ ./internal/router/ ./internal/application/repository/
```

| Package | Result at HEAD `d184ad9f` | Pre-M3 base (temp worktree at `9e41df62` / plan commit `5936cb2c`) | Notes |
|---|---|---|---|
| `internal/agent/subagents` | **ok** | package is M3's own (scanner-only state green) | executor/tools/library/scan tests all green, incl. the integration tests cited in §6 |
| `internal/application/repository` | **ok** | **ok** | incl. `tenant_subagent` repository tests |
| `internal/handler` | FAIL (2) | identical FAIL (2) | `TestExecutionTargetRegistrationGinSQLiteLifecycle`, `TestExecutionRegistrationChallengeRequiresTenantAndOwner` — the execution-registration schema-drift reds M1/M2 already documented; untouched by M3. Subagent handler tests (catalog list/detail, install happy+idempotent, rejections, remove) all PASS. |
| `internal/router` | FAIL (1) | identical FAIL (1) | `TestExecutionRegistrationRoutesUseRealRegistrationTargetRevokeLifecycle` — same pre-existing family. `TestSubagentRoutesRegisterBesideAgentAndExpertRoutes` PASSES. |
| `internal/application/service` | FAIL (1) | identical FAIL (1) | `TestExecuteDurableRunPersistsBudgetExhaustionForNotification` — expects a `budget_exhausted` event, run emits `run_failed` with `task_budget_exhausted`. Fails identically before any M3 code; previously masked because M2's duplicate-migration breakage failed the whole package (88 tests) before this test could run — the migration fix (`5c4d990e`, in base) unmasked it. Deterministic (3/3 runs). All subagent service tests (`subagent_service_test.go`, `subagent_delegate_test.go` incl. registration conditions and locale fallback) PASS. |
| web build | **ok** | — | §7 |

## Notes and gaps

1. **Live `subagent:`-prefixed sub-run events substituted with executor
   integration tests** (§6, honestly labeled) — blocked live only by the
   pre-existing `web_fetch` schema-vs-`DeclarationTools` incompatibility,
   which deserves a dedicated fix task on main (it breaks ANY web-tools
   agent on the durable engine, independent of sub-agents).
2. **Failed-tool error text vs durable tool-message content** (§6 unknown
   slug caveat): pre-existing engine rendering; the delegate tool itself
   returns the model-facing sentinel exactly as designed.
3. **No migration workaround this round.** M2's scratch migrations copy is
   obsolete — main's `5c4d990e` fixed the duplicate, and 0 → 84 runs clean
   (verified twice: fresh boot here; the service gate now compiles and runs).
4. **Scripted mock is scaffolding.** The `e2e-scripted-mock` model entry and
   the python server lived entirely under the git-ignored scratch root and
   are deleted with it; the repo's committed `config/` was used unmodified
   (symlink). No secrets committed — the E2E password and all JWTs stayed in
   scratch and are deleted.
5. **SSE relay of durable completions** was not observed in this environment
   within the client window (§6 note); transcript tables + raw dumps carry
   the evidence. Flagged as an observation for the durable-run SSE relay,
   not investigated further here.
6. Scratch artifacts (DB, server log, dumps, mock, curl outputs) under
   `.superpowers/sdd/2026-09-19-subagents-m3/e2e/` were deleted after this
   document was written; nothing scratch is committed.
