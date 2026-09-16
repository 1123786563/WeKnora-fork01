# 2026-09-17 Round R440 — Settings phase 3, per-user preferences, wiki editor contract, embed bridge sweep (TDD, 5 parallel agents)

Round type: code-level TDD round (5 parallel agents — A1 settings editors phase 3, A2 per-user preference
namespacing G2, A3 wiki page sweep, A4 commercial/embed sweep, A5 verifier) plus an orchestrator closure of the
test-glob gap the verifier surfaced.

## A1 — Knowledge settings editors phase 3 (PASS)

multimodal / asr / faq migrated against KnowledgeBaseEditorModal.vue, completing the section set except `basic`:
- **multimodal**: toggle + conditional VLLM model select (required, non-active excluded), description language
  select (labels reuse `language.*`), custom instructions (maxlength 4000); `multimodalInvalid` blocks save and
  jumps to the section.
- **asr**: toggle + ASR model select (required); `language` round-trips without UI exactly as Vue.
- **faq**: two index-mode radio groups (question_only/question_answer, combined/separate) + entryGuide copy;
  `indexModeRequired` validation. Gating contract honored: faq section only for FAQ KBs; chunking/advanced stay
  `!isFAQ` gated (KBModelConfig is not).
Key contract finding: `faq_config` is NOT part of `KBModelConfigRequest` — it follows Vue's doSubmit as a base
update `PUT /api/v1/knowledge-bases/:id` (`{name, description, config:{faq_config}}`) issued BEFORE the config
PUT. Both vlm/asr implement Vue's disabled-clears-model_id semantics. 9 jsdom tests red 8 → green 9/9;
directory regression 59/59; zero new i18n keys. Remaining: `basic` section, document-KB
wiki/auto_tag/indexing_strategy base-update wiring (channel now in place).

## A2 — Per-user preference namespacing, G2 closed (PASS)

React preference storage moved from flat keys to Vue's `WeKnora_${userId}_{theme|font_sans|font_mono|font_size}`
(preferenceStorage.ts contract verified verbatim by the verifier): userId from `weknora_user` JSON `.id` else
`anon`; one-shot per-user migration latch with `anon > Vue legacy flat > React flat` priority, source keys always
removed, `resetMigrationLatch()` for account switches; `locale` stays a flat key on both sides. domain
local-preferences rewritten tests 16/16 (multi-user isolation, three legacy sources, anon adoption, latch
idempotency), GeneralPreferencesPanel 10/10, consumer regression 58/58.
Deferred: GeneralPreferencesPanel still reads font keys directly (out of domain); the platform layer has no
`weknora_user` writer yet, so runtime mostly lands in the anon namespace until it does.

## A3 — Wiki page sweep (PASS)

List/search/create gating/empty-permission states and revision history (list+diff+revert) verified aligned
against WikiBrowser.vue + WikiRevisionDrawer.vue. Two contract gaps fixed by TDD (editor 8/8, page 11/11):
1. Edit cancel — Vue `cancelEditPage` exits the editor discarding the draft; React had no way out.
2. 409 conflict overwrite — Vue offers 覆盖保存 (fetch latest page, re-save the local draft on latest.version)
   with 重载 injecting server content INTO the editor; React only showed the conflict text and reload exited the
   editor. `overwriteWikiPage` pure function + conflict-block button (existing 5-locale keys) + reload alignment.
Deferred (recorded): markdown+wiki-link rendering (React renders raw pre — large item, single-round candidate),
list meta shows v{version} vs Vue updated_at, issue dialog/queue status/backlink footer need missing api-client
endpoints.

## A4 — Commercial/embed sweep (PASS)

- commercial: **no Vue counterpart exists** (fork-owned feature; full frontend search found nothing) — zero
  changes, existing tests 8/8 recorded as the ledger conclusion.
- embed: origin pinning, host source validation, and the five-way error mapping already match; one real gap
  fixed by TDD (red → green 5/5): `post()` lacked Vue `postToParent`'s sensitive/handshake fallback — unknown
  origin must DROP sensitive payloads (conversation content) while non-sensitive handshake messages
  (bootstrap_request/ready) fall back to `'*'` for token handoff without a referrer.
Deferred (recorded as a standalone-round candidate): React has no embed page at all (`/embed/*` renders an
"enter via standalone entry" error; no embed.html — loading flow, theme injection, size adaptation all absent
with it).

## Orchestrator closure — shared test glob gap

A5 found the root `test:shared` script never globbed `packages/domain/src/settings/*.test.ts`, leaving the new
G2 regression tests outside gate protection (existing script gap). Added the glob; `pnpm test:shared` now runs
**590/590** (+16: A2's 14 new tests plus 2 previously-uncovered settings tests).

## Gates (final)

`pnpm test:shared` 590/590 (broadened glob), `pnpm test:web` 1437/1437 (A5 final run; +14 this round),
`pnpm typecheck:web` clean, `pnpm build:web` ✓ 4.11s. No Vue, mobile, or Go code modified.
Per-agent reports: .omc/state/r440/report-A{1..5}.md (session artifacts).
