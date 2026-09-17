# 2026-09-17 Round R443 — Indexing lock, embed pagination/follow-ups/citations, shell badge+throttle, wiki source hydration (TDD, 5 parallel agents)

Round type: TDD round with 5 parallel agents (A1 isIndexingLocked, A2 embed residuals, A3 platform-shell
residuals, A4 wiki source-title hydration, A5 verifier). Verifier verdict: 4/4 PASS, zero regressions, no
rework items — the first round to close with no orchestrator fixes needed.

## A1 — isIndexingLocked (PASS)

Settings surface now probes `GET /api/v1/knowledge-bases/:id/knowledge?page=1&page_size=1` once on open
(edit mode, document type; FAQ/create paths skip it) and locks the indexing checkboxes with Vue's lockedTip
when `total > 0`; probe failure degrades to unlocked (deliberate divergence recorded: Vue closes the editor on
probe failure — React keeps the surface usable; product decision noted). All the requested-signal work landed:
15 request-count assertions updated individually across 7 existing test files + 4 new cases (locked render,
exact probe shape/singleton, total=0 unlocked, probe-failure fallback, committed strategy preserved on locked
save). knowledge-settings 76/76; i18n keys pre-existing (lockedTip) — zero new keys.

## A2 — Embed pagination, follow-ups, citations (PASS)

- History scroll pagination: cursor = previous batch's first `created_at` (`before_time` on the existing
  messages endpoint — no api-client change), top-edge trigger (scrollTop<=0, 500ms debounce, in-flight/hasMore
  guards), 20 per batch prepended with viewport preserved, four termination paths (empty/short batch/unchanged
  cursor/failure).
- Per-message follow-up suggestions: ensure on turn completion, GET backfill, generating poll (1s, max 120),
  ready-only render, click sends the question with `suggestion_attribution` attached to the next chat request;
  regenerate (allow_regenerate) + close controls. The host-analytics event reporting is intentionally not
  implemented (visible behavior only).
- Citation pills: `parseCitationSegments` mirrors Vue preprocessCitationTags/resolveCitationChunkId (doc title
  match, DOC-n/FAQ-n/positional digits, UUID passthrough, invalid tags dropped); web pills open externally, kb
  pills open the chunk via `embed.public.chunk`. Deviation recorded: plain-text parsing (no markdown pipeline on
  this face yet); `[[wiki]]` left as-is (no wiki route in embed).
15 new tests; embed suites 36/36; build ✓.

## A3 — Platform shell badge + throttle (PASS)

- Organizations pending-approval badge: sum of `pending_join_request_count` across orgs (mount-time fetch, no
  polling — matching Vue), rendered on the expanded sidebar item as an 18px amber pill with the raw count (no
  9+ cap), hidden on fetch failure. Defensive field read (api-client spread passthrough); explicit
  `Organization.pending_join_request_count` typing recorded as an api-client follow-up.
- Tenant submenu memberships refresh: Vue's ≥2000ms timestamp throttle on open, failure silently keeps the
  last-known list; `applyAuthMe` extracted and shared with the mount bootstrap (stale-client guarded).
6 new tests; platform 176/176.

## A4 — Wiki source-title hydration (PASS)

api-client needed ZERO new endpoints: the existing `knowledgeBases.documents.get(id)` is the same
`GET /api/v1/knowledge/{id}` Vue's getKnowledgeDetails uses. `createSourceRefTitleHydrator` ports the Vue
engine (dedupe, per-id cache, `title||file_name||fileName` resolution, failure keeps the truncated-id fallback
uncached, sequence guard retires stale hydration rounds); the reader footer consumes it via an optional
`sourceTitles` prop. 6 tests; wiki regression 44/44; typecheck:shared clean.

## Gates (final, merged state)

`pnpm test:web` 1530/1530 (+31, exactly the four agents' new cases), `pnpm test:shared` 596/596,
`pnpm typecheck:web` clean, `pnpm build:web` ✓ (A5 final run; orchestrator re-confirmed post-merge).
No Vue, mobile, or Go code modified; api-client and i18n untouched this round. Per-agent reports:
.omc/state/r443/report-A{1..4}.md + report-A5-review.md (session artifacts).
