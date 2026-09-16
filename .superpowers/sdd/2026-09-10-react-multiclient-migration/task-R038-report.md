# R038 Fix Round 1 Report

Date: 2026-09-13

## Scope

- Added React-owned member-list header, title/count badge, search/action, focus, and responsive styles using the existing `--wk-*` tokens.
- Reused `tenantMember.listTitle` and `tenantMember.searchPlaceholder` through the Web i18n path.
- Added an accessible, keyboard-focusable clear button. Clearing resets the controlled query and page, then calls the existing `load(1, '')` path so the API receives an unfiltered `q: undefined` request.
- Replaced the shallow class-string assertion with jsdom interaction coverage for initial loading, submitted search, stable header/input identity, manager controls, clearing, and visible unfiltered results.

## RED -> GREEN evidence

### RED

Command:

```text
cd apps/web && node --import tsx --test src/settings/TenantMembersPanel.test.tsx
```

Result before production changes: exit 1, 2 passed / 2 failed.

```text
FAIL member list header and localized search stay mounted during initial loading
  actual title: undefined; expected: Workspace members
FAIL search remains mounted and clearing reloads the unfiltered member list
  expected a search input/header interaction surface
```

The failures were caused by the missing localized title/search semantics and missing clearable interaction.

### GREEN

Command:

```text
cd apps/web && node --import tsx --test src/settings/TenantMembersPanel.test.tsx
```

Result: exit 0, 4 passed / 0 failed.

```text
PASS tenant members keeps viewer state read-only
PASS tenant members exposes manager search, invite and role controls
PASS member list header and localized search stay mounted during initial loading
PASS search remains mounted and clearing reloads the unfiltered member list
```

## Verification

### Web test suite

Command: `pnpm test:web`

Result: exit 1, 285 passed / 1 failed. R038's four tests passed. The unrelated existing failure is `src/auth/onboarding.test.ts`, which cannot resolve `@weknora/domain/auth/onboarding` from the installed workspace link.

### Web typecheck

Command: `pnpm typecheck:web`

Result: exit 1. No R038 type error remains. Existing unrelated errors:

```text
src/auth/onboarding.ts(1,128): TS2307 Cannot find module '@weknora/domain/auth/onboarding'
src/auth/WorkspaceOnboardingPage.tsx(91,18): TS7006 Parameter 'current' implicitly has an 'any' type
```

### Web build

Command: `pnpm build:web`

Result: exit 1 during `tsc -b`, with the same two unrelated onboarding errors above; Vite was not reached.

### React boundary check

Command: `node scripts/check-react-boundaries.mjs`

Result: exit 1 due to pre-existing repository-wide legacy-source reachability findings, including `packages/api-client/src/client.ts`, `packages/domain/src/auth/onboarding.ts`, generated/shared i18n files, and several existing Web settings/platform files. No reported finding names either R038-owned source file.

### Whitespace check

Command: `git diff --check`

Result after adding this report: exit 0.

## Remaining evidence gap

No browser visual or computed-style capture was performed in this fix round. The focused tests prove state and interaction behavior but do not constitute full Vue/React visual acceptance.
