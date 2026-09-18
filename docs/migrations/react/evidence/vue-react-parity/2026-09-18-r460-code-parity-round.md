# 2026-09-18 Round R460 — AuthPages de-next completed, integrity self-check script, source-doc paired evidence; backend crash-loop discovered (4 parallel agents)

Round type: TDD round with 4 parallel agents (A1 AuthPages ?next cleanup + /register ruling, A2 pre-push
integrity script, A3 browser catch-up evidence, A4 verifier). Verdict: A1 PASS, A2 PASS, A3 partial
(environment-limited) — and A3 discovered the backend in a crash loop. Final gates: test:web 1665/1665,
test:shared 749/749, typecheck 0, build ✓ (code gates unaffected by the backend outage — tests are mocked).

## A1 — AuthPages de-next + /register ruling (PASS)

Vue contract verified across all six flows (login, accept-and-enter, register-by-invite, plain-register
switch, OIDC exchange, router guard): Vue consumes ZERO `?next` anywhere — login lands fixed
(/platform/knowledge-bases or /onboarding/workspace). React converged: the 3 residual next consumers
(login submit / register-by-invite / OIDC exchange) now use the constant `authNavigationTarget()`;
`nextPathAfterAuth` and the dead `inviteNavigationAfterAuth` removed. /register authenticated bounce ruled
ALREADY-EQUIVALENT (loginBeforeLoad is mounted on both /login and /register; Vue's no-tenant path lands kb
then requiresTenant bounces to onboarding — same end state, no code change). Scoped 85/85; red confirmed
(old implementation returned /platform/apps?tab=connections). Deferred: the legacy AuthPages stack is dead
code (no production references) — deletion queued as a separate task.

## A2 — Commit-integrity self-check script (PASS; audit batch 2)

`scripts/check-commit-integrity.mjs` (+ node:test 9/9) wired as `pnpm check:integrity`: P0 import
resolvability against the git index (catches the R442/R444/R451 missed-file shape — workspace-present but
un-added targets report P0), WARN test-glob coverage (322 test files checked against the expanded globs),
INFO dirty shared-package reminder. End-to-end incident replay on a /tmp mini repo: missed-file → 2 P0 exit 1;
add-fix-with-dangling-import → still P0; repaired → exit 0. Current tree: 0 P0, 1 WARN (a cross-package
bare-specifier deep link in packages/ui/interaction.test.tsx — recorded as a boundary note; the report's
"add react-dom devDependency" suggestion was factually wrong, the dep is declared). Pre-push hook wiring
suggested, not in domain.

## A3 — Browser catch-up (partial; two leftovers closed, one blocked)

- **Source-doc paired evidence COMPLETED**: the carrier page is `concept/source-doc-traceability` (the
  r459-source-doc-fixture page's own refs were emptied). Vue click → doc-main-drawer card-details drawer
  (URL unchanged); React click → real navigation to /knowledgeBase/…/documents/2dc2b763-… — the full
  R459/R457 paired comparison, each side per its established design.
- Embed mermaid: still untestable (embed-channels list empty, consistent since R446). The wiki-mermaid
  fallback attempt failed on a script bug AND exposed the backend crash (below).
- **BACKEND CRASH-LOOP DISCOVERED**: WeKnora-app restarts with a DI panic — BuildContainer missing
  `*workbench.RemoteUsageService` (container.go:849/507). This is from the external process's recent backend
  commits (their workbench/voice work); attributed to them, not caused by our fixture save (the PUT returned
  normally). Blocks browser verification until they fix the registration. Settings flicker regression (their
  59c9dd88) untested for the same reason.
- New parity gap candidate recorded: React's wiki edit button did not show (canContribute judgment) where
  Vue's did — queued for verification after backend recovery.

## Gates (final)

`pnpm test:web` 1665/1665, `pnpm test:shared` 749/749, `pnpm typecheck:web` 0, `pnpm build:web` ✓ (A4 final;
chat-domain external WIP landed mid-window and is noted for the next round's gates). No Vue, mobile, or Go
code modified by this round. Per-agent reports: .omc/state/r460/report-A{1,2,3}.md + report-A4-review.md
(session artifacts).
