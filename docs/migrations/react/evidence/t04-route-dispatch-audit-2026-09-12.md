# T04 React route dispatch audit — 2026-09-12

## Scope

Read-only audit of `apps/web/src/routes.tsx` and `apps/web/src/main.tsx`, covering
`resolveRoute`, `guardRoute`, and the protected-page renderer. The audit excluded
the already fixed `/platform/knowledge-bases` and `/knowledgeBase/:id` regressions
except to verify that they remained covered.

## Findings and fixes

- `/knowledgeBase` was classified as a protected `knowledge-base` route and the
  guard allowed it, but the renderer only mounted a page when an ID was present;
  it therefore rendered `NotFoundPage`. `protectedPageForRoute` now maps the
  no-ID compatibility route to the knowledge-base list.
- `/platform/dev/markdown` was classified as a protected platform route, but the
  legacy route is public and there was no React renderer branch. The guard now
  allows it and the renderer mounts `DevMarkdownPage`, which keeps the fixture
  output escaped through React text nodes.
- `/creatChat/*` was an over-broad compatibility match: it normalized arbitrary
  suffixes to `/platform/creatChat/*`, then passed the guard and fell through to
  `NotFoundPage`. The alias now accepts only the registered legacy `/creatChat`
  path; unknown suffixes remain `not-found`.

## Regression evidence

`apps/web/src/routes.test.ts` asserts the `/knowledgeBase` list mapping, the
public and dispatchable development fixture, and existing canonical/deep-link
route behavior. The focused Web suite passed 99/99. Web typecheck and production
build passed; build output retained the existing non-fatal `>500 kB` warning.

The connected Chrome session then opened `http://127.0.0.1:5177/knowledgeBase`
and exposed the rendered `Knowledge bases` heading and live fixture rows. It
also opened `/platform/dev/markdown` and exposed the `Markdown rendering test`
heading, editor, reset action, and escaped `<script>` text in the rendered
fixture. No browser state was mutated beyond navigation.

## Remaining boundary

This audit proves route classification and renderer dispatch. It does not claim
full browser coverage for every route, Wails packaging, or a complete parity
implementation of the legacy Vue Markdown visual-regression catalog.
