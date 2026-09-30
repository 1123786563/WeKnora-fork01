# T08 Opportunity Web UI review fixes

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent duplicate opportunities from a repeated save and remove copied global CSS from the Opportunity stylesheet.

**Architecture:** Keep one request ID per unchanged import attempt. Once a receipt is saved, a second activation must not create a new attempt until the user explicitly starts a new draft (or reuses the saved request). Keep Opportunity styling local and rely on the existing app-global stylesheet imported separately.

**Tech Stack:** React/TypeScript UI tests, CSS, Web build.

**Sources:** T08 UI code commit `fbc3afac310a40e2e661722f49fd97318d068dd7`; independent reviewer findings in `apps/web/src/career/OpportunityPage.tsx:68-72,104` and `opportunity.css:73-538`; #146 and `docs/plans/2026-09-24-issue-140-t08-web.md`.

## Global Constraints

- Existing isolated T08 Web worktree only. Own Opportunity component/tests and its stylesheet; touch chat/route files only if a test proves necessary.
- Local code commit authorized; no push/merge/deploy. Preserve typed result link, explicit new attempt flow, same-ID unknown recovery, scope fencing, and exact inert text rendering.

## Review Focus

- Repeated click after success does not issue a second POST or create another opportunity. A clearly intentional new JD still can be saved with a fresh ID.
- `opportunity.css` contains only Opportunity-specific rules; no copied global tail or duplicate import changes the rest of the app.
- Focused and full Web tests, typecheck, build, and diff check pass.

---

### Task 1: One attempt per saved JD

**Depends:** initial T08 UI. **Owner/validator:** frontend_implementer / frontend_validator. **Files:** `apps/web/src/career/OpportunityPage.tsx`, `OpportunityPage.test.tsx`. **Consumes:** existing import state. **Produces:** explicit next-draft transition.

- [ ] RED: Test that a second save activation after success leaves POST count at one; test an explicit new draft can submit with a new ID.
- [ ] GREEN: Disable/redirect save in saved state until user explicitly starts a new draft. Keep unknown outcome lookup/retry unchanged.

### Task 2: Remove duplicate global stylesheet tail

**Depends:** initial T08 UI; same owner/worktree. **Owner/validator:** frontend_implementer / frontend_validator. **Files:** `apps/web/src/career/opportunity.css`. **Consumes:** current Opportunity rules and global `styles.css`. **Produces:** only local Opportunity CSS.

- [ ] PRECHECK: Diff `opportunity.css` lines 73 onward against `styles.css` and confirm the copied block has no Opportunity-only selector.
- [ ] GREEN: Remove the copied global block; retain the Opportunity media block and relevant local rules.
- [ ] VERIFY: Focused tests, `pnpm typecheck:web`, `pnpm test:web`, `pnpm build:web`, `git diff --check`; independent reviewer rechecks both medium findings and new regression risk.

## Shared-file preflight

No other implementation agent writes Web files during this repair. The backend and client contract are already reviewed and integrated; this fix must not change their wire.
