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
