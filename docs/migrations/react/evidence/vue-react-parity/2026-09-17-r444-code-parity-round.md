# 2026-09-17 Round R444 — Embedding lock warning, embed markdown pipeline, admin invite/pagination, shared-drawer browser evidence (TDD, 5 parallel agents)

Round type: TDD round with 5 parallel agents (A1 Embedding lock warning, A2 embed markdown pipeline,
A3 administration closeout + Organization typing, A4 browser fixture + shared-drawer evidence, A5 verifier).
A5 verdict: A1/A2/A3 PASS (zero orchestrator fixes needed); A4's report landed after A5's polling window but
before round closure — its findings are folded in here.

## A1 — Embedding locked warning (PASS)

Per Vue KBModelConfig.vue, the Embedding selector now locks when `ragEnabled && hasFiles` — the warning
(`knowledgeEditor.models.embeddingLocked`) renders under the Embedding row's description, driven by the SAME
R443 probe signal as isIndexingLocked. Contract nuance honored: with ALL indexing strategies off, Embedding
stays editable even though basic's checkbox lock still applies (Vue code is authoritative). The storage
migrate-hint became conditional on hasFiles (Vue `v-if="hasFiles"`). knowledge-settings 81/81 (+5); i18n key
pre-existing ×5 locale, zero new keys.
Deferred: storage select "editable when no files" needs the storage save pipeline first; Vue's
`ragEnabled !== false || wikiEnabled` row-visibility is a separate signal-driven diff, recorded.

## A2 — Embed face markdown pipeline (PASS)

The embed chat face now renders answers through the Vue-identical pipeline (EmbedBotMessage.vue →
chatMarkdownRenderer): citation tags are extracted to pill HTML placeholders BEFORE `marked.parse
({breaks:true, gfm:true})`, restored after parse, standalone citation paragraphs collapsed, then sanitized
through the same DOMPurify config. Reuses the R441 wiki markdown stack (same marked/dompurify versions);
wiki/markdown.ts only gained two exports (escapeHTML, allowed-URI regexp) — wiki consumption behavior
unchanged, wiki markdown tests 19/19. Two live findings fixed during TDD: DOMPurify hook-side
`element.remove()` aborts sanitization (FORBID_TAGS carries it instead, output-equivalent) and USE_PROFILES
strips `target` (same-version behavior as Vue). embed 44/44 (+8 incl. XSS payloads/attribute escaping/unclosed
streaming tags); citation pills now sit on rendered markdown, eliminating the R443 plain-text deviation.
Deferred: KaTeX (Vue has marked-katex-extension), mermaid/image safe-renderer, citation-icon svg asset.

## A3 — Administration closeout (PASS)

- `Organization.pending_join_request_count?: number` declared in api-client (R443's defensive read becomes
  typed; behavior unchanged).
- Invite two-step confirmation per Vue TenantMembers.vue: form submit only validates and previews (title/body
  with email/role interpolation, Vue `confirmInviteTitle/confirmInviteBody` copy), Back returns, the confirm
  step's main button sends, failure stays on confirm. 7 new keys ×5 locale (ru honestly mirrors the Vue ru-RU
  English source).
- Members pagination in the admin page: default 20, options [10,20,50,100], page window, prev/next, jump input
  with clamping, page-size change reloads server-side, server `total` counting.
administration 11/11, api-client identity 5/5, i18n guards green.
Deferred: barrel export for TenantMemberPage (structural type mirrors it), Vue's auto-accept skip and owner
invite role (React keeps the established ownerInviteForbidden semantics).

## A4 — Shared-drawer browser evidence with a fresh fixture (evidence only, zero code)

Created a reusable fixture (recorded for future rounds): account shared-fixture@local.dev (tenant 10003, new
KB 「共享夹具库」 `08e02d8b-…`, shared viewer to org 「Parity 共享空间」 which parity joined via invite code
`d2e59d97f312aaa9` as admin). Result: the R438 shared-KB drawer works on BOTH ends with the five fields and
buttons matching verbatim; 6 paired screenshots in `screenshots/r444-20260917/`. Entering the shared KB is
read-only on both ends.
Recorded diffs for future rounds: ① React viewer shared card exposes a 设置 button Vue does not have;
② React shows a read-only banner inside the shared KB while Vue shows none (and stale "支持拖拽上传" copy with
the upload entry actually hidden); ③ empty document count renders "-" on Vue vs "0" on React; ④ drawer close
is a "关闭设置" text button on Vue vs an × icon on React. Side observations: fixture tenant has no models so
the UI create wizard stalls (used the API for setup); Vue's share-management org search returns "no match" for
an org the user administers with no new network request — suspected Vue-side frontend filter bug, flagged for
upstream confirmation. The bonus pending-approval badge capture was not possible (the invite code joined
without approval; React's 待审核申请 tab confirmed present with an empty state).

## Gates (final)

`pnpm test:web` 1545/1545 (+19, all accounted: A1 +5, A2 +8, A3 web +2 / shared +4), `pnpm test:shared`
600/600, `pnpm typecheck:web` clean, `pnpm build:web` ✓ (A5 final run on the merged state; baseline
1530/596 all green). No Vue, mobile, or Go code modified. Per-agent reports:
.omc/state/r444/report-A{1..4}.md + report-A5-review.md (session artifacts).
