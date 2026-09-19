# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Before exploring, read these

- **`CONTEXT.md`** at the repo root.
- **`docs/adr/`** — read ADRs that touch the area you're about to work in.

If any of these files don't exist, **proceed silently**. Don't flag their absence; don't suggest creating them upfront. The `/domain-modeling` skill (reached via `/grill-with-docs` and `/improve-codebase-architecture`) creates them lazily when terms or decisions actually get resolved.

## File structure

This repository uses a single-context layout:

```text
/
├── CONTEXT.md
├── docs/adr/
│   ├── 0001-event-sourced-orders.md
│   └── 0002-postgres-for-write-model.md
└── src/
```

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal, a hypothesis, a test name), use the term as defined in `CONTEXT.md`. Don't drift to synonyms the glossary explicitly avoids.

If the concept you need isn't in the glossary yet, that's a signal — either you're inventing language the project doesn't use (reconsider) or there's a real gap (note it for `/domain-modeling`).

## Flag ADR conflicts

If your output contradicts an existing ADR, surface it explicitly rather than silently overriding:

> _Contradicts ADR-0007 (event-sourced orders) — but worth reopening because…_

## Persona (MBTI)

**Persona (MBTI)** is an optional per-agent personality layer, migrated from Octop: one of the 16 built-in MBTI profiles plus optional free-text style guidance, rendered as a leading segment of the agent's system prompt. It changes tone only — never retrieval, tools, or permissions.

- **Config fields** (`types.CustomAgentConfig`, persisted with the agent): `persona_mbti` — an uppercase MBTI code (`"INTJ"`) selecting a built-in profile from `internal/agent/persona` (16 profiles, 28-question test, bilingual zh/en rendering); `persona_style` — free text appended after the MBTI block. Both live under the agent's `config` and are returned by the agent read APIs.
- **Rendering point**: `renderPersonaSegment` in `internal/application/service/agent_capabilities.go`, called once inside `prepareAgentCapabilities` — the single capability-assembly point, so both execution engines (builtin ReAct and durable tRPC runs) get the persona segment prepended to the system prompt; on the builtin engine it rides in front of whichever prompt template applies (custom or the default scaffolding), never replacing it. Empty/unknown `persona_mbti` leaves the prompt untouched; `persona_style` only renders when a persona is set. Locale (zh/en) follows the request language context, falling back to the deployment default; durable (tRPC) runs currently always render the fallback because the request locale is not carried in the admission snapshot.
- **API endpoints** (`internal/router/routes_persona.go`, JWT or full-access API key): `GET /api/v1/mbti/types`, `GET /api/v1/mbti/types/:code`, `GET /api/v1/mbti/preview/:code`, `GET /api/v1/mbti/test/questions`, `POST /api/v1/mbti/test/submit`; agent-scoped mutations share the agent-update guard (creator or Admin+, `manage_agents` keys): `PUT /api/v1/agents/:id/persona` `{code, style?}`, `DELETE /api/v1/agents/:id/persona`.
- **Editor surface**: the React agent editor (`apps/web/src/agents/PersonaSection.tsx` + `MbtiTestModal.tsx`, via `packages/api-client/src/mbti.ts`) offers a 16-type grid with manual pick, an optional 28-question test modal that applies the scored code, and a style textarea; the section is offered only in smart-reasoning (agent) mode, since the quick-answer pipeline never renders a persona. The type list is fetched from the API — never hardcoded in the frontend.

When writing about it, keep the glossary discipline: "persona" is the MBTI capability on an agent, not a user profile or a custom system prompt.

## Experts (expert templates)

**Experts** are instantiable agent templates migrated from Octop: a shipped library of persona + starter-prompt + bundled-skill packages that map onto a tenant-owned custom agent in one action. In M2 the library is builtin-only (the shipped `config/experts/` templates); market install/publish are later milestones.

- **Builtin library location**: `config/experts/<id>/manifest.yaml` plus the persona documents it lists (`prompt_files`, e.g. `SOUL.md`/`IDENTITY.md`) and optional `skills/<slug>/SKILL.md` directories. `experts.LoadBuiltinExperts` (`internal/agent/experts`) scans `<ConfigDir()>/experts` lazily once per process and returns shared read-only experts; a missing directory is the normal "none shipped" case.
- **Provenance**: every instantiated agent carries `config.expert_source` — `{expert_id, source}` with `source: "builtin"` in M2 — set by `buildAgentFromExpert` (`internal/application/service/expert_service.go`) and visible through the ordinary agent read APIs; it survives later agent edits.
- **Instantiate mapping**: locale-resolved label → agent `name` (a non-blank `agent_name` override wins), description → `description`, `icon_name` → `avatar`; persona files joined with `"\n\n---\n\n"` (manifest order) plus the template's agent-config prompt → `config.system_prompt` (baked at instantiation locale, no per-turn re-resolution); quick prompts → `question_suggestions.starters` (curated mode); non-zero `agent_config` fields override the CreateAgent defaults; manifest `skills` → `skills_selection_mode: "selected"` + `selected_skills`.
- **API endpoints** (`internal/router/routes_expert.go`; JWT or full-access API key, Viewer+ reads / Contributor+ instantiate mirroring POST /agents): `GET /api/v1/experts` (locale-resolved summaries), `GET /api/v1/experts/:id` (detail incl. joined persona-markdown preview + quick prompts), `POST /api/v1/experts/:id/instantiate` with optional `{agent_name, sandbox_config_id}` (strict body: unknown fields → 400). The tenant comes from the auth context, never the body.
- **Skill pending semantics**: bundled skills resolve through `NewBundledSkillResolver` (`internal/application/service/expert_skills.go`) *before* the agent is persisted. `selected_skills` carries the SKILL.md frontmatter name (what the runtime's AllowedSkills matches — it may differ from the manifest slug); with a `sandbox_config_id` and the skill not yet installed there, the resolver zips the bundled directory and starts an async tenant-skill install, returning its id in `skill_install_ids`; `pending_skills` carries manifest slugs that are not usable yet (including ones whose install just started). Without a sandbox, or on any per-skill failure, the skill stays selected-and-pending — degrade, never fail: a missing optional skill must not abort the agent the user asked for.
- **Editor surface**: the React `/platform/experts` page (`apps/web/src/experts/ExpertsPage.tsx` via `packages/api-client`'s experts module) renders the catalog, a detail drawer with the persona preview, one-click instantiate with an optional custom name, a provenance badge on expert-created agents, and a pending-skill toast that deep-links the agent editor.

Glossary discipline: an "expert" is the template; the thing users chat with is the agent instantiated from it. Say "instantiate an expert", never "create an expert" for the agent-side action.

## Sub-agents (specialist roles)

**Sub-agents** are specialist roles an agent can delegate self-contained subtasks to, migrated from Octop's role library: the main agent calls one `subagent_delegate` tool, the chosen role runs its own short reasoning loop under its role prompt (it does not see the conversation), and its bounded summary comes back as the tool result. Unlike experts — which instantiate a whole new agent — a sub-agent is a capability installed *on* an existing agent.

- **Builtin library**: `config/subagents/library/{zh,en}/<division>/*.md` plus `zh/divisions.json` (the authoritative division set; 19 divisions, 271 slugs). A slug is the filename stem, paired across locales; each role file is YAML frontmatter (`name`, `description`, `color`, `emoji`, `vibe`, `tools` — a raw comma list) plus the markdown body that becomes the sub-run system prompt. `subagents.ScanSubagents` / `LoadBuiltinSubagents` (`internal/agent/subagents/scan.go`) scan `<ConfigDir()>/subagents` lazily once per process and return shared read-only values; `en/` is optional (en ships 217 of the 271 roles), and division directories not listed in divisions.json are an error.
- **Install model (two-step)**: `POST /api/v1/agents/:id/subagents {slug, locale?}` copies the catalog role verbatim into `tenant_subagents` — one row per (tenant, slug, locale), `source: "builtin"` — and appends the slug to the agent's `config.subagents` (config order is the delegation order). The *installed copy* is the source of truth at run time: shipping an updated library never changes a tenant's roles until a row is written. Install is idempotent (a reinstall refreshes the stored copy, 200 with the same list); `DELETE /api/v1/agents/:id/subagents/:slug` drops the slug from the agent config only — the tenant row is kept (copies are shared across agents; the catalog's `installed` flag stays true, and deleting rows is tenant-admin scope, not this endpoint). Locale resolution at install prefers the request locale, then en, then zh (en lacks 55 of the roles).
- **Delegation semantics**: `registerSubagentDelegateTool` (`internal/application/service/subagent_delegate.go`) registers `subagent_delegate` only when `config.subagents` is non-empty AND at least one configured slug has an installed tenant row — every other session keeps the tool off entirely, so the model never sees uninstalled specialists; the description lists exactly the config-order slugs. The model surface is strictly `{slug, goal, input_refs?}` (unknown fields, blank slug/goal and trailing input are rejected server-side by `ParseDelegateArgs`); session, user, model, tool whitelist and executor are assembled server-side. At delegation time the role's declared tools (`MapRoleTools`: Octop's `WebSearch/WebFetch/Read/Write/Edit` and their Chinese equivalents onto engine tool names) are intersected with the main agent's effective allowlist and the constructed registry — a sub-agent never gets a tool the main agent could not call itself. An empty intersection is first-class: most roles run toolless and the result carries a `[no tools available]` note so the model knows why nothing was looked up. The delegate returns the full summary, so serial relay (delegating again using the first answer) works; DAG orchestration is deliberately not built.
- **Budgets and the sub-run**: `subagents.Execute` (`internal/agent/subagents/executor.go`) runs the sub-run on a lightweight LLM/tools trpc graph with a fresh session id (`<main-session>:<slug>`, isolated history), the reused main-run model client, non-streaming generation, and two budgets: 8 model rounds and 32 KiB cumulative assistant text. Crossing either is not an error — the partial text returns with a `[budget exceeded]` note. The summary is capped at 8 KiB, tail-preserving (the conclusion lives at the end).
- **Events**: every sub-run tool call/result is attributed on the MAIN session's transcript as `subagent:<slug>:<tool>` (`agent.tool_call` / `agent.tool_result`), with the same sanitization the main engine applies to file writes and bulky tool data; the wrapper's `ToolExecContext` carries the main session id, the attributed call id and the delegating user id, so HITL gates and session-attributed tools behave exactly like main-run calls. The delegate tool's own result reports `{slug, rounds, overrun}` alongside the summary.
- **API endpoints** (`internal/router/routes_subagent.go`; catalog reads are Viewer+ with full-access API keys only, agent-scoped routes use the agentsRead/agentsWrite matrices — creator or Admin+): `GET /api/v1/subagent-catalog` (divisions in divisions.json order, slug-sorted entries with both locales' names and the tenant's installed flags), `GET /api/v1/subagent-catalog/:slug` (frontmatter scalars plus both locale bodies, present-or-empty), `GET|POST /api/v1/agents/:id/subagents`, `DELETE /api/v1/agents/:id/subagents/:slug`. The install body is strict (unknown field → 400, missing slug → 400); unknown slug → 404; the tenant comes from the auth context, never the body.
- **Editor surface**: the agent editor's Sub-agents section (`apps/web/src/agents/SubagentsSection.tsx` via `packages/api-client`'s subagents module) shows the agent's installed roles (config.subagents; delegation is off while the list is empty) with one-click removal, plus a catalog browser with division filter chips, client-side search and per-entry install; a detail peek renders the role body markdown. The section is offered only in smart-reasoning (agent) mode, like tools/skills. Install/remove patch `config.subagents` straight from the API response — the backend owns the persisted list, the form never composes it.

Glossary discipline: a "sub-agent" (or "specialist") is the installed role; "delegation" is the `subagent_delegate` tool call; "install" copies a library role into tenant storage and references it from the agent config. Say "delegate to a sub-agent", never "create a sub-agent" for the install action — nothing conversable is created.
