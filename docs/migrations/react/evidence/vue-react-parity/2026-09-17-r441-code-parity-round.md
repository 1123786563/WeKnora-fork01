# 2026-09-17 Round R441 — Settings basic section complete, wiki markdown, embed entry, per-user auth writes + paired-browser sweep

Round type: TDD round with 6 parallel agents (A1 settings finish, A2 wiki markdown rendering, A3 embed entry
page, A4 preference/auth writes, A5 browser-evidence-only, A6 verifier) plus an orchestrator closure of the
live-data chunking defect the browser sweep surfaced.

## A1 — Knowledge settings `basic` section + base-update wiring (PASS)

`BasicSettingsSection` ported from Vue: KB id (code + copy), type radio (disabled on edit), document-type
indexing-strategy checkboxes (RAG/Wiki with vector→keyword coupling, Vue `toggleVectorIndexing`), wiki
granularity + content/extraction instructions (maxlength 4000), name (required, 50), description (200).
`nameRequired` validation joins Vue `validateForm` order in `handleSave` (name → indexing atLeastOne →
multimodal → faq), failing with a section jump and zero requests. Document-KB base updates now carry
`{name, description, config:{wiki_config, auto_tag_config, indexing_strategy}}` via `buildKnowledgeSettingsBaseUpdate`
(Vue doSubmit L1399-1421 / loadKBData L918-941 field-for-field; FAQ sends only faq_config), and the base update
now ALWAYS precedes the config PUT (previously only FAQ did). A5's nav-order observation re-audited against
Vue L603-669: R438's implementation was already correct (五组顺序/组内键序/datasource `!isFAQ` 分支一致) — no
code change, recorded honestly. basic-section tests red 8/9 → green 9/9; directory 68/68; zero new i18n keys.
Deferred: `isIndexingLocked` (needs a file-count signal), wiki "NEW" badge, models required-field validation.

## A2 — Wiki markdown rendering (PASS)

The wiki reader upgraded from raw `<pre>` to Vue's chain: `[[slug|label]]` pre-processing → `marked.parse
({breaks:true})` → DOMPurify (same config semantics) → `.wiki-reader-body` with click-delegated navigation
(missing slugs stay put, as Vue). Reused marked + dompurify at the SAME versions as the Vue frontend
(marked ^17.0.5, dompurify ^3.4.11, zero new downloads) for byte-identical DOM. Notable verified finding:
DOMPurify 3.4.11 under USE_PROFILES drops explicit ALLOWED_ATTR, so Vue's real output carries only
`rel="noopener noreferrer"` (no target=_blank) — replicated; slugs are HTML-escaped before parse (Vue
AgentStreamDisplay convention), making malicious slugs inert text. Scoped 27/27.
Deferred: image preview dialog, index-view markdown reuse, reader footer backlinks/source line.

## A3 — Embed entry page (PASS)

`/embed/*` no longer renders the "standalone entry required" error: an isolated embed entry equivalent to Vue's
embed.html + embed-main.ts now mounts (main.tsx embed branch, /embed/ skips initTheme, index.html injects
theme-mode=light for /embed/ paths as the single-document equivalent of Vue's separate embed.html). The three
contracts land: bootstrap handshake (`bootstrap_request → provide_token → ready`, token also accepted via URL),
theme injection (channel `primary_color` → `--embed-primary` + color-mix badge), and full-viewport iframe fill
(Vue reports no size messages). Chat surface is a minimal composer + SSE stream reusing existing chat components,
posting through the R440 bridge (message_sent/received as sensitive). New host-protocol/host-context/messages
modules; 7 tests red → green; bridge regression 5/5; build emits the EmbedEntryPage chunk.
Deferred: history backfill, references/suggested questions, real-iframe e2e.

## A4 — Preference keys finish + platform user identity (PASS)

GeneralPreferencesPanel font_sans/font_mono reads/writes now go through the per-user path (domain
`readUserPreference`/`writeUserPreference`, flat keys added to the migration sources). The platform layer now
writes/clears `weknora_user` like Vue stores/auth.ts: persisted on the three /login success paths
(`persistLogin`), hydrated on /auth/me (`scope-runtime.hydrate`, covering the main.tsx login-jump bootstrap),
cleared on logout/401-refresh failure (`credentials.ts` anonymous branch + scope-runtime logout), each touching
the migration-latch reset. Full localStorage-sequence regression: u1 dark → logout → u2 default light (zero
bleed) → u2 system → u1 still dark. Scoped 71/71.
Note: pnpm `file:` deps copy-install — future domain edits need a `pnpm install` to propagate.

## A5 — Paired-browser sweep (PASS, read-only)

9 screenshots in `screenshots/r441-20260917/`. Settings grouped IA matches Vue across all five groups with the
models/chunking/advanced/multimodal/asr editors rendering (basic was still a placeholder at capture time —
closed by A1 in the same round). Datasource add-card form matches; Parity KB Demo has no datasource rows so
cron/pills were not observable. No shared-KB fixture exists, so the shared drawer pair was not capturable.
Recorded diffs for future rounds: separators control shape (listbox vs tag chips), missing 测试分块效果 entry /
Embedding-disabled warning / 200K badge, per-section 保存配置 vs Vue's single 取消/保存并关闭, English eyebrow +
CJK title duplication. One live-data defect found and fixed in-round (below).

## Orchestrator closure — chunking seed defect from live evidence

A5 observed overlap 0 on the Parity KB Demo in React where Vue showed 80. Root cause: Vue `loadKBData` seeds
`chunkOverlap: kb.chunking_config?.chunk_overlap || 80` (falsy fallback; comment aligns it with the backend
`chunker.DefaultChunkOverlap`), while React used a `typeof === 'number'` guard that let a stored 0 through.
Fixed to the Vue `||` semantics, with the same treatment for `separators` (Vue keeps an empty array — truthy —
as-is). Regression test pins both (stored `chunk_overlap: 0` → 80; `separators: []` → kept). Scoped 9/9.
The "512 字符 vs aria 500" observation could not be reproduced in code (slider text renders the same `value`
as aria-valuenow; sizeLabel copy is static "分块大小") — left as a note for the next browser pass.

## Gates (final)

Re-run on the final code (including the chunking closure): `pnpm test:web` 1473/1473 (+1 regression test),
`pnpm test:shared` 596/596, `pnpm typecheck:web` clean (the closure's first draft had a TS2322 wide-type error,
fixed with a `Number(...) || 80` numeric-domain equivalent of Vue's `||`), `pnpm build:web` ✓ 4.35s.
A6's pre-closure run: shared 596/596, web 1472/1472 (+41 this round), zero regressions. No Vue, mobile, or Go
code modified. Per-agent reports: .omc/state/r441/report-A{1..5}.md + report-A6-review.md (session artifacts).
