# FAQ B5 tag-chip tooltip parity

Date: 2026-09-14

## Change

The React FAQ card footer tag chip (`apps/web/src/faq/FAQPage.tsx`, `.faq-tag-chip`)
now carries the full tag name as a native `title` attribute, aligning with the
Vue tag tooltip semantics:

- Vue authority: `frontend/src/components/FAQTagTooltip.vue` is a hover bubble
  (mouseenter/leave, fixed-position bubble with the full `content` text) used
  in `FAQEntryManager.vue` around truncated `question-tag` items. The footer
  `faq-tag-chip` truncates its `.tag-text` (Vue: `max-width: 100px`, ellipsis)
  so the full name is unreachable when clipped.
- React parity: the chip renders `title={tagName ?? t('knowledgeBase.untagged')}`
  — the same full text the Vue bubble would show, simplified to the native
  title attribute, which also serves as the accessible (aria) description of
  the chip. The body `question-tag` spans already carried `title={value}`.

## Verification

- `cd apps/web && npx tsx --test "src/faq/"*.test.tsx "src/faq/"*.test.ts`:
  59/59 passed (58 baseline + 1 new test `tag chip carries the FAQTagTooltip
  hover content as native tooltip semantics`, asserting `class="faq-tag-chip"
  title="重要"` and the `无标签` untagged fallback).
- `pnpm run typecheck:web`: passed (exit 0).

## Remaining divergences (not in this slice)

- Vue's bubble is a styled fixed-position popover (max-width 320px, arrow,
  fade transition, flip/boundary logic); React uses the native title tooltip.
  A richer popover can replace `title` later without API change.
- Vue `.tag-text` clips at `max-width: 100px`; the React chip clips at
  `.faq-tag-chip { max-width: 160px }` — pre-existing styling divergence,
  untouched here.
- Vue's footer chip is click-wrapped in a `t-dropdown` tag picker when the
  user can edit; React's per-card tag editing is not part of B5.
