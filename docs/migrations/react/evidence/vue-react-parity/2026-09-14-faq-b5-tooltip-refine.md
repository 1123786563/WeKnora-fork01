# FAQ B5 tag tooltip refine — native title → FAQTagTooltip bubble

Date: 2026-09-14

## Change

The React FAQ card footer tag chip retired the d3a39b7b simplified form
(native `title` attribute) and now renders inside the committed
`FaqTagTooltip` bubble (`apps/web/src/faq/FAQPage.tsx`), matching the Vue
positioning-bubble component:

- Vue authority: `frontend/src/components/FAQTagTooltip.vue` — hover trigger
  (mouseenter/mouseleave), `Teleport` to `body`, `position: fixed` bubble
  driven by `updatePosition` (:63-101): 8px gap to the anchor, 8px viewport
  clamp on both axes, flip to `bottom` when the space above is under the
  padding, re-position on scroll (capture) and resize; max-width 320px,
  fade 0.15s, double-layer 5px caret (`::before` outline + `::after` fill).
- React parity: the bubble form was already committed for the card
  similar/negative/answer chips and the search-result chips (commit 8064522b).
  This slice extends it to the last native-title holdout — the footer
  `.faq-tag-chip` — by wrapping it in `FaqTagTooltip` with
  `content={tagName ?? t('knowledgeBase.untagged')}` (the exact text the old
  title carried: resolved tag name + 无标签 fallback). The wrapper's click
  toggle already stops propagation, preserving the Vue
  `faq-card-tag @click.stop` behavior.
- Kept divergence: `.tag-text` clipping (React 160px chip vs Vue 100px span)
  stays untouched, as scoped for this slice.

## Verification

- TDD red → green: new test `footer tag chip opens the hover bubble with the
  full tag name, untagged fallback otherwise` (interactive `FAQPageView`
  mount: hover → bubble carries 生产环境标签, leave → closes, untagged entry →
  无标签) failed before the wiring and passes after; the B5 wiring count test
  moved 3 → 4 wrappers (footer chip joins) and asserts the chip no longer
  carries a native `title`.
- `cd apps/web && npx tsx --test "src/faq/"*.test.tsx "src/faq/"*.test.ts`:
  79/79 passed (78 baseline + 1 new; the d3a39b7b-era assertion
  `class="faq-tag-chip" title="重要"` in FAQPage.test.tsx was rewritten to
  specify the bubble form — wrapper > chip > tag-text, no native title).
- `pnpm run typecheck:web`: passed (exit 0).

## Remaining divergences (not in this slice)

- Vue's footer chip is click-wrapped in a `t-dropdown` tag picker when the
  user can edit (`FAQEntryManager.vue:363-370`); React's per-card tag
  editing remains out of B5 scope. Vue also shows no tooltip at all on this
  chip — the bubble here is the coordinator-scoped upgrade of the d3a39b7b
  title form, reusing the shared Vue component shape.
- `.tag-text` clip width (160px vs Vue 100px) — pre-existing, untouched.
- Vue positions with root-zoom normalization (`getRootZoom` /
  `rectToCssPx`); React reads raw `getBoundingClientRect` under the
  assumption of no root `zoom` — inherited from the committed 8064522b
  component, unchanged here.
